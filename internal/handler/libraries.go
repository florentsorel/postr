package handler

import (
	"errors"
	"net/http"

	postrdb "github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/plex"
	"github.com/labstack/echo/v5"
)

type libraryItem struct {
	Key     string `json:"key"`
	Title   string `json:"title"`
	Type    string `json:"type"`
	Enabled bool   `json:"enabled"`
}

type getLibrariesResponse struct {
	Configured bool          `json:"configured"`
	Reachable  bool          `json:"reachable"`
	Error      string        `json:"error,omitempty"`
	Libraries  []libraryItem `json:"libraries,omitempty"`
}

func (h *Handler) GetLibraries(c *echo.Context) error {
	if h.plex == nil {
		return c.JSON(http.StatusOK, getLibrariesResponse{Configured: false})
	}

	sections, err := h.plex.Sections(c.Request().Context())
	if err != nil {
		msg := "Unable to reach Plex server."
		if errors.Is(err, plex.ErrUnauthorized) {
			msg = "Invalid Plex token."
		}
		return c.JSON(http.StatusOK, getLibrariesResponse{Configured: true, Reachable: false, Error: msg})
	}

	// Load saved enabled states from DB
	saved, err := h.db.ListLibrarySettings(c.Request().Context())
	if err != nil {
		return jsonInternalError(c)
	}
	enabledByKey := make(map[string]bool, len(saved))
	for _, s := range saved {
		enabledByKey[s.SectionKey] = s.Enabled != 0
	}

	var libraries []libraryItem
	for _, s := range sections {
		if s.Type != "movie" && s.Type != "show" {
			continue
		}
		enabled := true
		if v, ok := enabledByKey[s.Key]; ok {
			enabled = v
		}
		libraries = append(libraries, libraryItem{Key: s.Key, Title: s.Title, Type: s.Type, Enabled: enabled})
	}

	return c.JSON(http.StatusOK, getLibrariesResponse{
		Configured: true,
		Reachable:  true,
		Libraries:  libraries,
	})
}

type saveLibraryItem struct {
	Key     string `json:"key"`
	Enabled bool   `json:"enabled"`
}

type saveLibrariesRequest struct {
	Libraries []saveLibraryItem `json:"libraries"`
}

func (h *Handler) SaveLibraries(c *echo.Context) error {
	var req saveLibrariesRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}

	ctx := c.Request().Context()
	for _, lib := range req.Libraries {
		var enabled int64
		if lib.Enabled {
			enabled = 1
		}
		if err := h.db.UpsertLibrarySetting(ctx, postrdb.UpsertLibrarySettingParams{SectionKey: lib.Key, Enabled: enabled}); err != nil {
			return jsonInternalError(c)
		}
	}

	return c.NoContent(http.StatusNoContent)
}
