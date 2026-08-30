package handler

import (
	"log/slog"
	"net/http"

	postrdb "github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/mediaserver"
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
	if h.server == nil {
		return c.JSON(http.StatusOK, getLibrariesResponse{Configured: false})
	}

	ctx := c.Request().Context()
	libs, err := h.server.Libraries(ctx)
	if err != nil {
		return c.JSON(http.StatusOK, getLibrariesResponse{
			Configured: true,
			Reachable:  false,
			Error:      h.unreachableMessage(err),
		})
	}

	// Load saved enabled states from DB
	saved, err := h.db.ListLibrarySettings(ctx, h.provider())
	if err != nil {
		return jsonInternalError(c, err)
	}
	enabledByKey := make(map[string]bool, len(saved))
	for _, s := range saved {
		enabledByKey[s.SectionKey] = s.Enabled != 0
	}

	var libraries []libraryItem
	for _, l := range libs {
		// Collection libraries (Jellyfin box sets) are not user-selectable: they
		// are imported alongside the movie libraries they belong to.
		if l.Type != mediaserver.TypeMovie && l.Type != mediaserver.TypeShow {
			continue
		}
		enabled := true
		if v, ok := enabledByKey[l.Key]; ok {
			enabled = v
		}
		libraries = append(libraries, libraryItem{Key: l.Key, Title: l.Title, Type: l.Type, Enabled: enabled})
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
		if err := h.db.UpsertLibrarySetting(ctx, postrdb.UpsertLibrarySettingParams{
			Provider:   h.provider(),
			SectionKey: lib.Key,
			Enabled:    enabled,
		}); err != nil {
			return jsonInternalError(c, err)
		}
		slog.Info("library setting saved", "key", lib.Key, "enabled", lib.Enabled)
	}

	return c.NoContent(http.StatusNoContent)
}
