package handler

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/florentsorel/postr/db"
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
	AddedAt         *int64 `json:"addedAt,omitempty"`
}

func (h *Handler) GetMediaThumb(c *echo.Context) error {
	ratingKey := c.Param("ratingKey")

	m, err := h.db.GetMediaByRatingKey(c.Request().Context(), ratingKey)
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
		return jsonInternalError(c)
	}

	file, err := c.FormFile("file")
	if err != nil {
		return jsonError(c, http.StatusBadRequest, "file required")
	}

	src, err := file.Open()
	if err != nil {
		return jsonInternalError(c)
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return jsonInternalError(c)
	}

	ext := extFromFilename(file.Filename)
	if err := h.storePoster(ctx, m, ratingKey, ext, data); err != nil {
		return jsonInternalError(c)
	}

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
		return jsonInternalError(c)
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
		return jsonInternalError(c)
	}

	ext := extFromContentType(resp.Header.Get("Content-Type"))
	if ext == "" {
		ext = extFromFilename(body.URL)
	}

	if err := h.storePoster(ctx, m, ratingKey, ext, data); err != nil {
		return jsonInternalError(c)
	}

	return c.JSON(http.StatusOK, map[string]string{"ext": ext, "thumb": "/api/media/" + ratingKey + "/thumb"})
}

func (h *Handler) GetMedia(c *echo.Context) error {
	rows, err := h.db.ListMedia(c.Request().Context())
	if err != nil {
		return jsonInternalError(c)
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
		items = append(items, item)
	}

	return c.JSON(http.StatusOK, items)
}
