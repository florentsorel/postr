package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/plex"
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

func (h *Handler) ImportFromPlex(c *echo.Context) error {
	var req importRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}
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

	log := c.Logger()
	ctx := c.Request().Context()

	// Phase 1: fetch all sections and items upfront to know the total.
	sections, err := h.plex.Sections(ctx)
	if err != nil {
		send(sseErrorEvent{Type: "error", Message: "failed to fetch Plex sections: " + err.Error()})
		return nil
	}
	sectionByKey := make(map[string]plex.Section, len(sections))
	for _, s := range sections {
		sectionByKey[s.Key] = s
	}

	type workBatch struct {
		target     importTarget
		sectionKey string
		section    plex.Section
		items      []plex.Item
	}

	var batches []workBatch
	for _, target := range req.Targets {
		for _, sectionKey := range target.SectionKeys {
			sec, ok := sectionByKey[sectionKey]
			if !ok {
				log.Warn("section not found in Plex", "key", sectionKey)
				continue
			}

			var items []plex.Item
			switch target.Type {
			case "movie", "show":
				items, err = h.plex.AllItems(ctx, sectionKey)
			case "season":
				var shows []plex.Item
				shows, err = h.plex.AllItems(ctx, sectionKey)
				if err == nil {
					for _, show := range shows {
						var seasons []plex.Item
						seasons, err = h.plex.Children(ctx, show.RatingKey)
						if err != nil {
							break
						}
						for i := range seasons {
							seasons[i].Title = show.Title
							episodes, err := h.plex.Children(ctx, seasons[i].RatingKey)
							if err == nil && len(episodes) > 0 {
								seasons[i].Year = episodes[0].SeasonYear()
							} else {
								seasons[i].Year = show.Year
							}
						}
						items = append(items, seasons...)
					}
				}
			case "collection":
				items, err = h.plex.Collections(ctx, sectionKey)
			}

			if err != nil {
				send(sseErrorEvent{Type: "error", Message: "failed to fetch items for section " + sec.Title + ": " + err.Error()})
				return nil
			}

			batches = append(batches, workBatch{target, sectionKey, sec, items})
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
		library, err := h.db.UpsertLibrary(ctx, db.UpsertLibraryParams{
			SectionKey: batch.sectionKey,
			Title:      batch.section.Title,
			Type:       batch.section.Type,
			ImportedAt: time.Now().Unix(),
		})
		if err != nil {
			log.Error("failed to upsert library", "section", batch.sectionKey, "error", err)
			send(sseErrorEvent{Type: "error", Message: "database error for library " + batch.section.Title})
			return nil
		}

		// Build set of existing rating_keys for this library+type to detect new vs existing items.
		existingKeys, err := h.db.ListRatingKeysByLibraryIDAndType(ctx, db.ListRatingKeysByLibraryIDAndTypeParams{
			LibraryID: library.ID,
			Type:      batch.target.Type,
		})
		if err != nil {
			log.Error("failed to list existing keys", "error", err)
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
			_, isExisting := existingSet[item.RatingKey]

			var thumbExt string
			if item.Thumb != "" {
				var saveErr error
				thumbExt, saveErr = h.saveThumb(ctx, batch.target.Type, item.RatingKey, item.Thumb)
				if errors.Is(saveErr, errThumbUnchanged) && isExisting {
					skipped++
					processedSet[item.RatingKey] = struct{}{}
					send(sseSkipEvent{Type: "skip", Title: item.Title, Message: "unchanged"})
					send(sseProgressEvent{Type: "progress", Current: current, Total: total})
					continue
				}
				if saveErr != nil && !errors.Is(saveErr, errThumbUnchanged) {
					log.Warn("failed to save thumb", "ratingKey", item.RatingKey, "error", saveErr)
					send(sseSkipEvent{Type: "skip", Title: item.Title, Message: "thumbnail download failed: " + saveErr.Error()})
				}
			}

			params := db.UpsertMediaParams{
				LibraryID:    library.ID,
				RatingKey:    item.RatingKey,
				Title:        item.Title,
				Type:         batch.target.Type,
				Year:         sql.NullInt64{Int64: int64(item.Year), Valid: item.Year != 0},
				SeasonNumber: sql.NullInt64{Int64: int64(item.Index), Valid: batch.target.Type == "season" && item.Index != 0},
				Thumb:        sql.NullString{String: thumbExt, Valid: thumbExt != ""},
				AddedAt:      sql.NullInt64{Int64: item.AddedAt, Valid: item.AddedAt != 0},
				CreatedAt:    now,
				UpdatedAt:    now,
			}
			if err := h.db.UpsertMedia(ctx, params); err != nil {
				log.Error("failed to upsert media", "ratingKey", item.RatingKey, "error", err)
			} else {
				if !isExisting {
					added++
				}
				processedSet[item.RatingKey] = struct{}{}
				// The poster was just re-downloaded from Plex, so any pending
				// local push is now stale — remove it from the queue.
				if err := h.db.DeletePosterQueueByRatingKey(ctx, item.RatingKey); err != nil {
					log.Warn("failed to remove stale queue entry", "ratingKey", item.RatingKey, "error", err)
				}
			}

			send(sseProgressEvent{Type: "progress", Current: current, Total: total})
		}

		// Delete items that are no longer in Plex for this library+type.
		for key := range existingSet {
			if _, processed := processedSet[key]; !processed {
				if err := h.db.DeleteMediaByRatingKey(ctx, key); err != nil {
					log.Error("failed to delete stale media", "ratingKey", key, "error", err)
				} else {
					deleted++
				}
			}
		}
	}

	send(sseDoneEvent{Type: "done", Added: added, Skipped: skipped, Deleted: deleted})
	return nil
}

var errThumbUnchanged = errors.New("unchanged")

// saveThumb downloads a poster from Plex and writes it to disk, skipping the
// write if the file already exists with identical content.
// It returns the file extension (e.g. "jpg", "png", "webp") and any error.
func (h *Handler) saveThumb(ctx context.Context, mediaType, ratingKey, thumbPath string) (string, error) {
	data, ext, err := h.plex.DownloadThumb(ctx, thumbPath)
	if err != nil {
		return "", err
	}

	dir := filepath.Join(h.config.DataPath, "posters", mediaType)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	dest := filepath.Join(dir, ratingKey+"."+ext)

	if existing, err := os.ReadFile(dest); err == nil && bytes.Equal(existing, data) {
		return ext, errThumbUnchanged
	}

	return ext, os.WriteFile(dest, data, 0o644)
}
