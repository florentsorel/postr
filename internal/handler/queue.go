package handler

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/labstack/echo/v5"
	"golang.org/x/sync/semaphore"
)

type queueItemResponse struct {
	RatingKey    string `json:"ratingKey"`
	Title        string `json:"title"`
	Type         string `json:"type"`
	SeasonNumber *int64 `json:"seasonNumber,omitempty"`
	Thumb        string `json:"thumb"`
}

func (h *Handler) GetQueue(c *echo.Context) error {
	rows, err := h.db.ListPosterQueueWithMedia(c.Request().Context(), h.provider())
	if err != nil {
		return jsonInternalError(c, err)
	}

	items := make([]queueItemResponse, 0, len(rows))
	for _, r := range rows {
		item := queueItemResponse{
			RatingKey: r.RatingKey,
			Title:     r.Title,
			Type:      r.Type,
			Thumb:     "/api/media/" + r.RatingKey + "/thumb",
		}
		if r.SeasonNumber.Valid {
			item.SeasonNumber = &r.SeasonNumber.Int64
		}
		items = append(items, item)
	}

	return c.JSON(http.StatusOK, items)
}

type removeQueueResponse struct {
	Thumb    string `json:"thumb"`
	Warning  string `json:"warning,omitempty"`
	Orphaned bool   `json:"orphaned,omitempty"`
}

func (h *Handler) RemoveFromQueue(c *echo.Context) error {
	ratingKey := c.Param("ratingKey")
	ctx := c.Request().Context()

	// Ping first so config errors leave the queue intact.
	if h.server != nil {
		if pingErr := h.server.Ping(ctx); pingErr != nil {
			return jsonError(c, http.StatusBadGateway,
				h.connectionErrorMessage("Could not restore the "+h.serverName()+" poster.", pingErr))
		}
	}

	if err := h.db.DeletePosterQueueByRatingKey(ctx, ratingKey); err != nil {
		return jsonInternalError(c, err)
	}

	resp := removeQueueResponse{Thumb: "/api/media/" + ratingKey + "/thumb"}

	if h.server != nil {
		m, err := h.db.GetMediaByRatingKey(ctx, ratingKey)
		if err == nil {
			now := time.Now().Unix()
			ext, saveErr := h.savePoster(ctx, m.Type, ratingKey)
			if saveErr != nil && !errors.Is(saveErr, errPosterUnchanged) {
				slog.Warn("failed to restore poster", "title", m.Title, "ratingKey", ratingKey, "error", saveErr)
				resp.Warning = "Could not restore the " + h.serverName() + " poster. The media may no longer exist in " + h.serverName() + "."
				if errors.Is(saveErr, mediaserver.ErrNotFound) {
					resp.Orphaned = true
					_ = h.db.MarkOrphan(ctx, db.MarkOrphanParams{
						RatingKey: ratingKey,
						UpdatedAt: now,
					})
					slog.Info("marked as orphan during restore", "title", m.Title, "ratingKey", ratingKey)
				} else {
					_ = h.db.SetLocallyModified(ctx, db.SetLocallyModifiedParams{
						LocallyModified: 0,
						UpdatedAt:       now,
						RatingKey:       ratingKey,
					})
				}
			} else if saveErr == nil {
				_ = h.db.UpdateMediaThumb(ctx, db.UpdateMediaThumbParams{
					Thumb:     sql.NullString{String: ext, Valid: true},
					UpdatedAt: now,
					RatingKey: ratingKey,
				})
				_ = h.db.SetLocallyModified(ctx, db.SetLocallyModifiedParams{
					LocallyModified: 0,
					UpdatedAt:       now,
					RatingKey:       ratingKey,
				})
			}
		}
	}

	return c.JSON(http.StatusOK, resp)
}

func (h *Handler) PushPoster(c *echo.Context) error {
	if h.server == nil {
		return jsonError(c, http.StatusBadRequest, h.serverName()+" is not configured")
	}
	ratingKey := c.Param("ratingKey")
	ctx := c.Request().Context()

	m, err := h.db.GetMediaByRatingKey(ctx, ratingKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return jsonError(c, http.StatusNotFound, "media not found")
		}
		return jsonInternalError(c, err)
	}

	ext := "jpg"
	if m.Thumb.Valid && m.Thumb.String != "" {
		ext = m.Thumb.String
	}

	path := h.posterPath(m.Type, ratingKey, ext)
	data, err := os.ReadFile(path)
	if err != nil {
		return jsonError(c, http.StatusNotFound, "poster file not found")
	}

	if pingErr := h.server.Ping(ctx); pingErr != nil {
		return jsonError(c, http.StatusBadGateway,
			h.connectionErrorMessage("Failed to push poster to "+h.serverName()+".", pingErr))
	}

	slog.Info("pushing poster", "provider", h.provider(), "type", m.Type, "title", m.Title, "ratingKey", ratingKey)
	if err := h.server.UploadPoster(ctx, ratingKey, data, mediaserver.ContentTypeFromExt(ext)); err != nil {
		slog.Error("failed to push poster", "title", m.Title, "ratingKey", ratingKey, "error", err)
		if errors.Is(err, mediaserver.ErrNotFound) {
			now := time.Now().Unix()
			_ = h.db.MarkOrphan(ctx, db.MarkOrphanParams{RatingKey: ratingKey, UpdatedAt: now})
			_ = h.db.DeletePosterQueueByRatingKey(ctx, ratingKey)
			return c.JSON(http.StatusGone, map[string]any{
				"error":    "The media no longer exists in " + h.serverName() + ".",
				"orphaned": true,
			})
		}
		return jsonError(c, http.StatusBadGateway, "Failed to push poster to "+h.serverName()+". The media may no longer exist.")
	}
	slog.Info("poster pushed", "provider", h.provider(), "type", m.Type, "title", m.Title)

	if err := h.db.DeletePosterQueueByRatingKey(ctx, ratingKey); err != nil {
		return jsonInternalError(c, err)
	}

	now := time.Now().Unix()
	if err := h.db.SetLocallyModified(ctx, db.SetLocallyModifiedParams{
		LocallyModified: 0,
		UpdatedAt:       now,
		RatingKey:       ratingKey,
	}); err != nil {
		return jsonInternalError(c, err)
	}

	h.resyncLocalPoster(ctx, m.Type, ratingKey, now)

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) PushAllPosters(c *echo.Context) error {
	if h.server == nil {
		return jsonError(c, http.StatusBadRequest, h.serverName()+" is not configured")
	}
	ctx := c.Request().Context()

	rows, err := h.db.ListPosterQueueWithMedia(ctx, h.provider())
	if err != nil {
		return jsonInternalError(c, err)
	}

	type result struct {
		RatingKey string `json:"ratingKey"`
		Error     string `json:"error,omitempty"`
	}

	results := make([]result, len(rows))
	sem := semaphore.NewWeighted(4)
	var mu sync.Mutex
	var wg sync.WaitGroup

	slog.Info("push all started", "count", len(rows))
	for i, r := range rows {
		wg.Add(1)
		go func(i int, ratingKey, rType, thumbStr string) {
			defer wg.Done()
			if err := sem.Acquire(ctx, 1); err != nil {
				mu.Lock()
				results[i] = result{RatingKey: ratingKey, Error: err.Error()}
				mu.Unlock()
				return
			}
			defer sem.Release(1)

			path := h.posterPath(rType, ratingKey, thumbStr)
			data, err := os.ReadFile(path)
			if err != nil {
				mu.Lock()
				results[i] = result{RatingKey: ratingKey, Error: "file not found"}
				mu.Unlock()
				return
			}

			if err := h.server.UploadPoster(ctx, ratingKey, data, mediaserver.ContentTypeFromExt(thumbStr)); err != nil {
				slog.Error("push all: failed to push poster", "ratingKey", ratingKey, "error", err)
				mu.Lock()
				results[i] = result{RatingKey: ratingKey, Error: err.Error()}
				mu.Unlock()
				return
			}

			if err := h.db.DeletePosterQueueByRatingKey(ctx, ratingKey); err != nil {
				mu.Lock()
				results[i] = result{RatingKey: ratingKey, Error: "push succeeded but failed to remove from queue: " + err.Error()}
				mu.Unlock()
				return
			}

			now := time.Now().Unix()
			// Leaving this flag set would show the item as still differing from
			// the server and hide it from the next sync, even though the push
			// went through — so it is reported rather than swallowed.
			if err := h.db.SetLocallyModified(ctx, db.SetLocallyModifiedParams{
				LocallyModified: 0,
				UpdatedAt:       now,
				RatingKey:       ratingKey,
			}); err != nil {
				slog.Error("push all: poster pushed but failed to clear locally_modified", "ratingKey", ratingKey, "error", err)
				mu.Lock()
				results[i] = result{RatingKey: ratingKey, Error: "push succeeded but the item is still marked as locally modified: " + err.Error()}
				mu.Unlock()
				return
			}

			h.resyncLocalPoster(ctx, rType, ratingKey, now)

			mu.Lock()
			results[i] = result{RatingKey: ratingKey}
			mu.Unlock()
		}(i, r.RatingKey, r.Type, func() string {
			if r.Thumb.Valid && r.Thumb.String != "" {
				return r.Thumb.String
			}
			return "jpg"
		}())
	}

	wg.Wait()
	slog.Info("push all done", "total", len(rows))
	return c.JSON(http.StatusOK, results)
}

// resyncLocalPoster re-downloads the poster from the media server after a
// successful push so the local copy matches what the server actually stores
// (both Plex and Jellyfin may re-encode the image). This prevents false
// "changed" detections on the next sync.
func (h *Handler) resyncLocalPoster(ctx context.Context, mediaType, ratingKey string, updatedAt int64) {
	newExt, err := h.savePoster(ctx, mediaType, ratingKey)
	if err != nil && !errors.Is(err, errPosterUnchanged) {
		slog.Warn("resyncLocalPoster: failed to re-download poster — next sync may report a false change", "ratingKey", ratingKey, "error", err)
		return
	}
	if err := h.db.UpdateMediaThumb(ctx, db.UpdateMediaThumbParams{
		Thumb:     sql.NullString{String: newExt, Valid: true},
		UpdatedAt: updatedAt,
		RatingKey: ratingKey,
	}); err != nil {
		slog.Warn("resyncLocalPoster: failed to update thumb in DB", "ratingKey", ratingKey, "error", err)
	}
}
