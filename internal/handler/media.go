package handler

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/florentsorel/postr/internal/db"
	"github.com/labstack/echo/v5"
)

type mediaResponse struct {
	ID              int64  `json:"id"`
	RatingKey       string `json:"ratingKey"`
	Title           string `json:"title"`
	Type            string `json:"type"`
	Year            *int64 `json:"year,omitempty"`
	SeasonNumber    *int64 `json:"seasonNumber,omitempty"`
	Thumb           string `json:"thumb,omitempty"`
	LocallyModified bool   `json:"locallyModified"`
	IsOrphan        bool   `json:"isOrphan"`
	AddedAt         *int64 `json:"addedAt,omitempty"`
}

func (h *Handler) GetMediaThumb(c *echo.Context) error {
	ratingKey := c.Param("ratingKey")

	m, err := h.db.GetMediaByRatingKey(c.Request().Context(), ratingKey)
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
	path := filepath.Join(h.config.DataPath, "posters", m.Type, ratingKey+"."+ext)
	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(c.Response(), c.Request(), path)
	return nil
}

// extFromFilename returns the file extension to use for storing a poster,
// based on the uploaded filename.
func extFromFilename(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".png":
		return "png"
	case ".webp":
		return "webp"
	default:
		return "jpg"
	}
}

// storePoster writes poster data to disk, updates the DB, and enqueues the item.
func (h *Handler) storePoster(ctx context.Context, m db.GetMediaByRatingKeyRow, ratingKey, ext string, data []byte) error {
	dir := filepath.Join(h.config.DataPath, "posters", m.Type)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	for _, oldExt := range []string{"jpg", "jpeg", "png", "webp"} {
		if oldExt != ext {
			_ = os.Remove(filepath.Join(dir, ratingKey+"."+oldExt))
		}
	}
	if err := os.WriteFile(filepath.Join(dir, ratingKey+"."+ext), data, 0o644); err != nil {
		return err
	}
	now := time.Now().Unix()
	if err := h.db.UpdateMediaThumb(ctx, db.UpdateMediaThumbParams{
		Thumb:     sql.NullString{String: ext, Valid: true},
		UpdatedAt: now,
		RatingKey: ratingKey,
	}); err != nil {
		return err
	}
	if err := h.db.SetLocallyModified(ctx, db.SetLocallyModifiedParams{
		LocallyModified: 1,
		UpdatedAt:       now,
		RatingKey:       ratingKey,
	}); err != nil {
		return err
	}
	return h.db.UpsertPosterQueue(ctx, db.UpsertPosterQueueParams{
		MediaID:   m.ID,
		CreatedAt: now,
	})
}

func (h *Handler) UploadMediaPoster(c *echo.Context) error {
	ratingKey := c.Param("ratingKey")
	ctx := c.Request().Context()

	m, err := h.db.GetMediaByRatingKey(ctx, ratingKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return jsonError(c, http.StatusNotFound, "media not found")
		}
		return jsonInternalError(c, err)
	}

	file, err := c.FormFile("file")
	if err != nil {
		return jsonError(c, http.StatusBadRequest, "file required")
	}

	src, err := file.Open()
	if err != nil {
		return jsonInternalError(c, err)
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return jsonInternalError(c, err)
	}

	ext := extFromFilename(file.Filename)
	if err := h.storePoster(ctx, m, ratingKey, ext, data); err != nil {
		return jsonInternalError(c, err)
	}

	slog.Info("poster uploaded", "type", m.Type, "title", m.Title, "ratingKey", ratingKey, "source", "file")
	return c.JSON(http.StatusOK, map[string]string{"ext": ext, "thumb": "/api/media/" + ratingKey + "/thumb"})
}

func extFromContentType(ct string) string {
	switch {
	case strings.HasPrefix(ct, "image/png"):
		return "png"
	case strings.HasPrefix(ct, "image/webp"):
		return "webp"
	case strings.HasPrefix(ct, "image/jpeg"):
		return "jpg"
	default:
		return ""
	}
}

func (h *Handler) UploadPosterFromURL(c *echo.Context) error {
	ratingKey := c.Param("ratingKey")
	ctx := c.Request().Context()

	m, err := h.db.GetMediaByRatingKey(ctx, ratingKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return jsonError(c, http.StatusNotFound, "media not found")
		}
		return jsonInternalError(c, err)
	}

	var body struct {
		URL string `json:"url"`
	}
	if err := c.Bind(&body); err != nil || strings.TrimSpace(body.URL) == "" {
		return jsonError(c, http.StatusBadRequest, "url required")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, body.URL, nil)
	if err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid url")
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return jsonError(c, http.StatusBadGateway, "failed to fetch URL")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return jsonError(c, http.StatusBadGateway, "URL returned status "+strconv.Itoa(resp.StatusCode))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, 10<<20))
	if err != nil {
		return jsonInternalError(c, err)
	}

	ext := extFromContentType(resp.Header.Get("Content-Type"))
	if ext == "" {
		ext = extFromFilename(body.URL)
	}

	if err := h.storePoster(ctx, m, ratingKey, ext, data); err != nil {
		return jsonInternalError(c, err)
	}

	slog.Info("poster uploaded", "type", m.Type, "title", m.Title, "ratingKey", ratingKey, "source", "url")
	return c.JSON(http.StatusOK, map[string]string{"ext": ext, "thumb": "/api/media/" + ratingKey + "/thumb"})
}

func (h *Handler) GetMedia(c *echo.Context) error {
	rows, err := h.db.ListMedia(c.Request().Context())
	if err != nil {
		return jsonInternalError(c, err)
	}

	items := make([]mediaResponse, 0, len(rows))
	for _, m := range rows {
		item := mediaResponse{
			ID:        m.ID,
			RatingKey: m.RatingKey,
			Title:     m.Title,
			Type:      m.Type,
		}
		if m.Year.Valid {
			item.Year = &m.Year.Int64
		}
		if m.Thumb.Valid {
			v := strconv.FormatInt(m.UpdatedAt, 10)
			item.Thumb = "/api/media/" + m.RatingKey + "/thumb?v=" + v
		}
		if m.SeasonNumber.Valid {
			item.SeasonNumber = &m.SeasonNumber.Int64
		}
		if m.AddedAt.Valid {
			item.AddedAt = &m.AddedAt.Int64
		}
		item.LocallyModified = m.LocallyModified != 0
		item.IsOrphan = m.IsOrphan != 0
		items = append(items, item)
	}

	return c.JSON(http.StatusOK, items)
}

func (h *Handler) DeleteOrphan(c *echo.Context) error {
	ratingKey := c.Param("ratingKey")
	ctx := c.Request().Context()

	m, err := h.db.GetMediaByRatingKey(ctx, ratingKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return jsonError(c, http.StatusNotFound, "media not found")
		}
		return jsonInternalError(c, err)
	}

	if err := h.db.DeleteMediaByRatingKey(ctx, ratingKey); err != nil {
		return jsonInternalError(c, err)
	}

	for _, ext := range []string{"jpg", "png", "webp"} {
		_ = os.Remove(filepath.Join(h.config.DataPath, "posters", m.Type, ratingKey+"."+ext))
	}

	slog.Info("orphan deleted", "type", m.Type, "title", m.Title, "ratingKey", ratingKey)
	return c.NoContent(http.StatusNoContent)
}
