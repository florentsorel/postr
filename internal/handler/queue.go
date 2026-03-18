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

func (h *Handler) RemoveFromQueue(c *echo.Context) error {
	ratingKey := c.Param("ratingKey")
	ctx := c.Request().Context()

	if err := h.db.DeletePosterQueueByRatingKey(ctx, ratingKey); err != nil {
		return jsonInternalError(c)
	}

	// Best-effort: re-download the current Plex poster to restore local copy.
	// If Plex is not configured or the pull fails, we still return success.
	if h.plex != nil {
		m, err := h.db.GetMediaByRatingKey(ctx, ratingKey)
		if err == nil {
			thumbPath := "/library/metadata/" + ratingKey + "/thumb"
			if ext, saveErr := h.saveThumb(ctx, m.Type, ratingKey, thumbPath); saveErr == nil {
				now := time.Now().Unix()
				if err := h.db.UpdateMediaThumb(ctx, db.UpdateMediaThumbParams{
					Thumb:     sql.NullString{String: ext, Valid: true},
					UpdatedAt: now,
					RatingKey: ratingKey,
				}); err != nil {
					c.Logger().Warn("failed to update thumb after pull")
				}
				if err := h.db.SetLocallyModified(ctx, db.SetLocallyModifiedParams{
					LocallyModified: 0,
					UpdatedAt:       now,
					RatingKey:       ratingKey,
				}); err != nil {
					c.Logger().Warn("failed to clear locally_modified after pull")
				}
			}
		}
	}

	return c.JSON(http.StatusOK, map[string]string{"thumb": "/api/media/" + ratingKey + "/thumb"})
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

	slog.Info("pushing poster to Plex", "type", m.Type, "title", m.Title, "ratingKey", ratingKey)
	if err := h.plex.UploadPoster(ctx, ratingKey, data, plex.ContentTypeFromExt(ext)); err != nil {
		slog.Error("failed to push poster to Plex", "title", m.Title, "ratingKey", ratingKey, "error", err)
		return jsonError(c, http.StatusBadGateway, "failed to push to Plex: "+err.Error())
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
