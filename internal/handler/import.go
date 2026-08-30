package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"

	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/labstack/echo/v5"
)

type importTarget struct {
	Type        string   `json:"type"`
	SectionKeys []string `json:"sectionKeys"`
}

type importRequest struct {
	Targets []importTarget `json:"targets"`
}

type sseStartEvent struct {
	Type  string `json:"type"`
	Total int    `json:"total"`
}

type sseProgressEvent struct {
	Type    string `json:"type"`
	Current int    `json:"current"`
	Total   int    `json:"total"`
}

type sseDoneEvent struct {
	Type    string `json:"type"`
	Added   int    `json:"added"`
	Skipped int    `json:"skipped"`
	Deleted int    `json:"deleted"`
}

type sseSkipEvent struct {
	Type    string `json:"type"`
	Title   string `json:"title"`
	Message string `json:"message"`
}

type sseErrorEvent struct {
	Type    string `json:"type"`
	Message string `json:"message"`
}

func (h *Handler) ImportFromServer(c *echo.Context) error {
	var req importRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}
	if h.server == nil {
		return jsonError(c, http.StatusBadRequest, h.serverName()+" is not configured")
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

	slog.Info("import started", "provider", h.provider())

	// Phase 1: fetch all libraries and items upfront to know the total.
	libs, err := h.server.Libraries(ctx)
	if err != nil {
		send(sseErrorEvent{Type: "error", Message: "failed to fetch " + h.serverName() + " libraries: " + err.Error()})
		return nil
	}
	libraryByKey := make(map[string]mediaserver.Library, len(libs))
	for _, l := range libs {
		libraryByKey[l.Key] = l
	}

	type workBatch struct {
		mediaType  string
		libraryKey string
		library    mediaserver.Library
		items      []mediaserver.Item
	}

	var batches []workBatch
	for _, target := range req.Targets {
		sectionKeys := target.SectionKeys

		// Jellyfin keeps every collection in one server-wide folder, so importing
		// collections once per selected movie library would import them several
		// times over. Anchor them to that folder instead.
		if target.Type == mediaserver.TypeCollection && h.server.GlobalCollections() {
			key, ok := collectionLibraryKey(libs)
			if !ok {
				slog.Info("no collection library on server, skipping collections")
				continue
			}
			sectionKeys = []string{key}
		}

		for _, sectionKey := range sectionKeys {
			lib, ok := libraryByKey[sectionKey]
			if !ok {
				slog.Warn("library not found on server", "key", sectionKey)
				continue
			}

			items, err := h.server.Items(ctx, sectionKey, target.Type)
			if err != nil {
				send(sseErrorEvent{Type: "error", Message: "failed to fetch items for library " + lib.Title + ": " + err.Error()})
				return nil
			}

			slog.Info("fetching items", "library", lib.Title, "type", target.Type, "count", len(items))
			batches = append(batches, workBatch{target.Type, sectionKey, lib, items})
		}
	}

	total := 0
	for _, b := range batches {
		total += len(b.items)
	}
	send(sseStartEvent{Type: "start", Total: total})

	// Phase 2: process each item, stream progress.
	var added, skipped, deleted int
	current := 0

	for _, batch := range batches {
		slog.Info("importing library", "library", batch.library.Title, "type", batch.mediaType, "total", len(batch.items))

		library, err := h.db.UpsertLibrary(ctx, db.UpsertLibraryParams{
			Provider:   h.provider(),
			SectionKey: batch.libraryKey,
			Title:      batch.library.Title,
			Type:       batch.library.Type,
			ImportedAt: time.Now().Unix(),
		})
		if err != nil {
			slog.Error("failed to upsert library", "section", batch.libraryKey, "error", err)
			send(sseErrorEvent{Type: "error", Message: "database error for library " + batch.library.Title})
			return nil
		}

		// Build set of existing rating_keys for this library+type to detect new vs existing items.
		existingKeys, err := h.db.ListRatingKeysByLibraryIDAndType(ctx, db.ListRatingKeysByLibraryIDAndTypeParams{
			LibraryID: library.ID,
			Type:      batch.mediaType,
		})
		if err != nil {
			slog.Error("failed to list existing keys", "error", err)
			send(sseErrorEvent{Type: "error", Message: "database error"})
			return nil
		}
		existingSet := make(map[string]struct{}, len(existingKeys))
		for _, k := range existingKeys {
			existingSet[k] = struct{}{}
		}

		processedSet := make(map[string]struct{}, len(batch.items))

		now := time.Now().Unix()
		for _, item := range batch.items {
			current++
			_, isExisting := existingSet[item.ID]

			var thumbExt string
			if item.HasPoster {
				var saveErr error
				thumbExt, saveErr = h.savePoster(ctx, batch.mediaType, item.ID)
				if errors.Is(saveErr, errPosterUnchanged) && isExisting {
					skipped++
					processedSet[item.ID] = struct{}{}
					send(sseSkipEvent{Type: "skip", Title: item.Title, Message: "unchanged"})
					send(sseProgressEvent{Type: "progress", Current: current, Total: total})
					continue
				}
				if saveErr != nil && !errors.Is(saveErr, errPosterUnchanged) {
					slog.Warn("failed to save poster", "title", item.Title, "id", item.ID, "error", saveErr)
					send(sseSkipEvent{Type: "skip", Title: item.Title, Message: "thumbnail download failed: " + saveErr.Error()})
				}
			}

			params := db.UpsertMediaParams{
				Provider:     h.provider(),
				LibraryID:    library.ID,
				RatingKey:    item.ID,
				Title:        item.Title,
				Type:         batch.mediaType,
				Year:         sql.NullInt64{Int64: int64(item.Year), Valid: item.Year != 0},
				SeasonNumber: sql.NullInt64{Int64: int64(item.SeasonNumber), Valid: batch.mediaType == mediaserver.TypeSeason && item.SeasonNumber != 0},
				Thumb:        sql.NullString{String: thumbExt, Valid: thumbExt != ""},
				AddedAt:      sql.NullInt64{Int64: item.AddedAt, Valid: item.AddedAt != 0},
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if err := h.db.UpsertMedia(ctx, params); err != nil {
				slog.Error("failed to upsert media", "title", item.Title, "id", item.ID, "error", err)
			} else {
				if !isExisting {
					slog.Info("imported", "type", batch.mediaType, "title", item.Title)
					added++
				}
				processedSet[item.ID] = struct{}{}
				// The poster was just re-downloaded from the server, so any pending
				// local push is now stale — remove it from the queue.
				if err := h.db.DeletePosterQueueByRatingKey(ctx, item.ID); err != nil {
					slog.Warn("failed to remove stale queue entry", "id", item.ID, "error", err)
				}
			}

			send(sseProgressEvent{Type: "progress", Current: current, Total: total})
		}

		// Mark items no longer on the server as orphans.
		for key := range existingSet {
			if _, processed := processedSet[key]; !processed {
				if err := h.db.MarkOrphan(ctx, db.MarkOrphanParams{
					RatingKey: key,
					UpdatedAt: now,
				}); err != nil {
					slog.Error("failed to mark orphan", "id", key, "error", err)
				} else {
					slog.Info("marked as orphan", "type", batch.mediaType, "id", key)
					deleted++
				}
			}
		}
	}

	slog.Info("import done", "added", added, "skipped", skipped, "deleted", deleted)
	send(sseDoneEvent{Type: "done", Added: added, Skipped: skipped, Deleted: deleted})
	return nil
}

// collectionLibraryKey returns the key of the server-wide collection library,
// if the server exposes one.
func collectionLibraryKey(libs []mediaserver.Library) (string, bool) {
	for _, l := range libs {
		if l.Type == mediaserver.TypeCollection {
			return l.Key, true
		}
	}
	return "", false
}

var errPosterUnchanged = errors.New("unchanged")

// savePoster downloads the poster currently set on the media server and writes
// it to disk, skipping the write if the file already exists with identical
// content. It returns the file extension (e.g. "jpg", "png", "webp") and any
// error, or errPosterUnchanged when the local copy is already up to date.
func (h *Handler) savePoster(ctx context.Context, mediaType, itemID string) (string, error) {
	data, ext, err := h.server.DownloadPoster(ctx, itemID)
	if err != nil {
		return "", err
	}

	if err := os.MkdirAll(h.posterDir(mediaType), 0o755); err != nil {
		return "", err
	}

	dest := h.posterPath(mediaType, itemID, ext)

	if existing, err := os.ReadFile(dest); err == nil && bytes.Equal(existing, data) {
		return ext, errPosterUnchanged
	}

	return ext, os.WriteFile(dest, data, 0o644)
}
