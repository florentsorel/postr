package handler

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/plex"
	"github.com/labstack/echo/v5"
)

type sseSyncChangedEvent struct {
	Type         string `json:"type"`
	RatingKey    string `json:"ratingKey"`
	Title        string `json:"title"`
	MediaType    string `json:"mediaType"`
	SeasonNumber *int64 `json:"seasonNumber,omitempty"`
	UpdatedAt    int64  `json:"updatedAt"`
}

type sseSyncDoneEvent struct {
	Type    string `json:"type"`
	Changed int    `json:"changed"`
	Checked int    `json:"checked"`
	Failed  int    `json:"failed"`
}

type sseSyncFailedEvent struct {
	Type      string `json:"type"`
	RatingKey string `json:"ratingKey"`
	Title     string `json:"title"`
	Reason    string `json:"reason"`
	Orphaned  bool   `json:"orphaned,omitempty"`
}

func (h *Handler) SyncFromPlex(c *echo.Context) error {
	if h.plex == nil {
		return jsonError(c, http.StatusBadRequest, "Plex is not configured")
	}

	resp := c.Response()
	resp.Header().Set("Content-Type", "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(resp)
	send := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(resp, "data: %s\n\n", b)
		_ = rc.Flush()
	}

	ctx := c.Request().Context()
	media, err := h.db.ListMedia(ctx)
	if err != nil {
		send(sseErrorEvent{Type: "error", Message: "failed to list media"})
		return nil
	}

	// Only check items that haven't been locally modified and are not orphans.
	var toCheck []db.ListMediaRow
	for _, m := range media {
		if m.LocallyModified == 0 && m.IsOrphan == 0 {
			toCheck = append(toCheck, m)
		}
	}

	slog.Info("sync started", "checking", len(toCheck))
	send(sseStartEvent{Type: "start", Total: len(toCheck)})

	var changed, failed int
	for i, m := range toCheck {
		thumbPath := "/library/metadata/" + m.RatingKey + "/thumb"
		ext, saveErr := h.saveThumb(ctx, m.Type, m.RatingKey, thumbPath)
		if errors.Is(saveErr, errThumbUnchanged) {
			send(sseProgressEvent{Type: "progress", Current: i + 1, Total: len(toCheck)})
			continue
		}
		if saveErr != nil {
			slog.Warn("sync: failed to download thumb", "title", m.Title, "ratingKey", m.RatingKey, "error", saveErr)
			reason := "Failed to download poster"
			if errors.Is(saveErr, plex.ErrNotFound) {
				reason = "No longer exists in Plex"
				_ = h.db.MarkOrphan(ctx, db.MarkOrphanParams{RatingKey: m.RatingKey, UpdatedAt: time.Now().Unix()})
			}
			failed++
			send(sseSyncFailedEvent{Type: "failed", RatingKey: m.RatingKey, Title: m.Title, Reason: reason, Orphaned: errors.Is(saveErr, plex.ErrNotFound)})
			send(sseProgressEvent{Type: "progress", Current: i + 1, Total: len(toCheck)})
			continue
		}

		now := time.Now().Unix()
		if err := h.db.UpdateMediaThumb(ctx, db.UpdateMediaThumbParams{
			Thumb:     sql.NullString{String: ext, Valid: true},
			UpdatedAt: now,
			RatingKey: m.RatingKey,
		}); err != nil {
			slog.Warn("sync: failed to update thumb in DB", "title", m.Title, "ratingKey", m.RatingKey, "error", err)
		}

		slog.Info("sync: poster updated", "type", m.Type, "title", m.Title)
		changed++
		event := sseSyncChangedEvent{
			Type:      "changed",
			RatingKey: m.RatingKey,
			Title:     m.Title,
			MediaType: m.Type,
			UpdatedAt: now,
		}
		if m.SeasonNumber.Valid {
			event.SeasonNumber = &m.SeasonNumber.Int64
		}
		send(event)
		send(sseProgressEvent{Type: "progress", Current: i + 1, Total: len(toCheck)})
	}

	slog.Info("sync done", "changed", changed, "failed", failed, "checked", len(toCheck))
	send(sseSyncDoneEvent{Type: "done", Changed: changed, Checked: len(toCheck), Failed: failed})
	return nil
}
