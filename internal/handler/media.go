package handler

import (
	"database/sql"
	"errors"
	"net/http"
	"path/filepath"

	"github.com/labstack/echo/v5"
)

type mediaResponse struct {
	ID       int64  `json:"id"`
	Title    string `json:"title"`
	Type     string `json:"type"`
	Year     *int64 `json:"year,omitempty"`
	Thumb    string `json:"thumb,omitempty"`
	AddedAt  *int64 `json:"addedAt,omitempty"`
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

	path := filepath.Join(h.config.DataPath, "posters", m.Type, ratingKey+".jpg")
	return c.File(path)
}

func (h *Handler) GetMedia(c *echo.Context) error {
	rows, err := h.db.ListMedia(c.Request().Context())
	if err != nil {
		return jsonInternalError(c)
	}

	items := make([]mediaResponse, 0, len(rows))
	for _, m := range rows {
		item := mediaResponse{
			ID:    m.ID,
			Title: m.Title,
			Type:  m.Type,
		}
		if m.Year.Valid {
			item.Year = &m.Year.Int64
		}
		if m.Thumb.Valid {
			item.Thumb = "/api/media/" + m.RatingKey + "/thumb"
		}
		if m.AddedAt.Valid {
			item.AddedAt = &m.AddedAt.Int64
		}
		items = append(items, item)
	}

	return c.JSON(http.StatusOK, items)
}
