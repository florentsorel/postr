package handler

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/plex"
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
	rows, err := h.db.ListPosterQueueWithMedia(c.Request().Context())
	if err != nil {
		return jsonInternalError(c)
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
	if h.plex != nil {
		if _, pingErr := h.plex.Sections(ctx); pingErr != nil {
			if errors.Is(pingErr, plex.ErrUnauthorized) {
				return jsonError(c, http.StatusBadGateway, "Could not restore the Plex poster. Invalid Plex token — check your PLEX_TOKEN setting.")
			}
			return jsonError(c, http.StatusBadGateway, "Could not restore the Plex poster. Unable to reach Plex — check your PLEX_URL setting.")
		}
	}

	if err := h.db.DeletePosterQueueByRatingKey(ctx, ratingKey); err != nil {
		return jsonInternalError(c)
	}

	resp := removeQueueResponse{Thumb: "/api/media/" + ratingKey + "/thumb"}

	if h.plex != nil {
		m, err := h.db.GetMediaByRatingKey(ctx, ratingKey)
		if err == nil {
			now := time.Now().Unix()
			thumbPath := "/library/metadata/" + ratingKey + "/thumb"
			ext, saveErr := h.saveThumb(ctx, m.Type, ratingKey, thumbPath)
			if saveErr != nil && !errors.Is(saveErr, errThumbUnchanged) {
				slog.Warn("failed to restore Plex poster", "title", m.Title, "ratingKey", ratingKey, "error", saveErr)
				resp.Warning = "Could not restore the Plex poster. The media may no longer exist in Plex."
				if errors.Is(saveErr, plex.ErrNotFound) {
					resp.Orphaned = true
					_ = h.db.MarkOrphan(ctx, db.MarkOrphanParams{
						RatingKey: ratingKey,
						UpdatedAt: now,
					})
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
	if h.plex == nil {
		return jsonError(c, http.StatusBadRequest, "Plex is not configured")
	}
	ratingKey := c.Param("ratingKey")
	ctx := c.Request().Context()

	m, err := h.db.GetMediaByRatingKey(ctx, ratingKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return jsonError(c, http.StatusNotFound, "media not found")
		}
		return jsonInternalError(c)
	}

	ext := "jpg"
	if m.Thumb.Valid && m.Thumb.String != "" {
		ext = m.Thumb.String
	}

	path := filepath.Join(h.config.DataPath, "posters", m.Type, ratingKey+"."+ext)
	data, err := os.ReadFile(path)
	if err != nil {
		return jsonError(c, http.StatusNotFound, "poster file not found")
	}

	if _, pingErr := h.plex.Sections(ctx); pingErr != nil {
		if errors.Is(pingErr, plex.ErrUnauthorized) {
			return jsonError(c, http.StatusBadGateway, "Failed to push poster to Plex. Invalid Plex token — check your PLEX_TOKEN setting.")
		}
		return jsonError(c, http.StatusBadGateway, "Failed to push poster to Plex. Unable to reach Plex — check your PLEX_URL setting.")
	}

	slog.Info("pushing poster to Plex", "type", m.Type, "title", m.Title, "ratingKey", ratingKey)
	if err := h.plex.UploadPoster(ctx, ratingKey, data, plex.ContentTypeFromExt(ext)); err != nil {
		slog.Error("failed to push poster to Plex", "title", m.Title, "ratingKey", ratingKey, "error", err)
		if errors.Is(err, plex.ErrNotFound) {
			now := time.Now().Unix()
			_ = h.db.MarkOrphan(ctx, db.MarkOrphanParams{RatingKey: ratingKey, UpdatedAt: now})
			_ = h.db.DeletePosterQueueByRatingKey(ctx, ratingKey)
			return c.JSON(http.StatusGone, map[string]any{
				"error":    "The media no longer exists in Plex.",
				"orphaned": true,
			})
		}
		return jsonError(c, http.StatusBadGateway, "Failed to push poster to Plex. The media may no longer exist.")
	}
	slog.Info("poster pushed to Plex", "type", m.Type, "title", m.Title)

	if err := h.db.DeletePosterQueueByRatingKey(ctx, ratingKey); err != nil {
		return jsonInternalError(c)
	}

	now := time.Now().Unix()
	if err := h.db.SetLocallyModified(ctx, db.SetLocallyModifiedParams{
		LocallyModified: 0,
		UpdatedAt:       now,
		RatingKey:       ratingKey,
	}); err != nil {
		return jsonInternalError(c)
	}

	return c.NoContent(http.StatusNoContent)
}

func (h *Handler) PushAllPosters(c *echo.Context) error {
	if h.plex == nil {
		return jsonError(c, http.StatusBadRequest, "Plex is not configured")
	}
	ctx := c.Request().Context()

	rows, err := h.db.ListPosterQueueWithMedia(ctx)
	if err != nil {
		return jsonInternalError(c)
	}

	type result struct {
		RatingKey string `json:"ratingKey"`
		Error     string `json:"error,omitempty"`
	}

	results := make([]result, len(rows))
	sem := semaphore.NewWeighted(4)
	var mu sync.Mutex
	var wg sync.WaitGroup

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

			path := filepath.Join(h.config.DataPath, "posters", rType, ratingKey+"."+thumbStr)
			data, err := os.ReadFile(path)
			if err != nil {
				mu.Lock()
				results[i] = result{RatingKey: ratingKey, Error: "file not found"}
				mu.Unlock()
				return
			}

			if err := h.plex.UploadPoster(ctx, ratingKey, data, plex.ContentTypeFromExt(thumbStr)); err != nil {
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
			_ = h.db.SetLocallyModified(ctx, db.SetLocallyModifiedParams{
				LocallyModified: 0,
				UpdatedAt:       now,
				RatingKey:       ratingKey,
			})

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
	return c.JSON(http.StatusOK, results)
}
