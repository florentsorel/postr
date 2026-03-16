package handler

import (
	"errors"
	"net/http"

	"github.com/florentsorel/postr/internal/plex"
	"github.com/labstack/echo/v5"
)

type plexStatusResponse struct {
	Configured bool `json:"configured"`
}

type plexPingResponse struct {
	Reachable bool   `json:"reachable"`
	Error     string `json:"error,omitempty"`
}

func (h *Handler) GetPlexStatus(c *echo.Context) error {
	return c.JSON(http.StatusOK, plexStatusResponse{
		Configured: h.config.PlexURL != "" && h.config.PlexToken != "",
	})
}

func (h *Handler) PingPlex(c *echo.Context) error {
	if h.plex == nil {
		return c.JSON(http.StatusOK, plexPingResponse{
			Reachable: false,
			Error:     "Plex is not configured.",
		})
	}

	_, err := h.plex.Sections(c.Request().Context())
	if err == nil {
		return c.JSON(http.StatusOK, plexPingResponse{Reachable: true})
	}

	msg := "Unable to reach Plex server."
	if errors.Is(err, plex.ErrUnauthorized) {
		msg = "Invalid Plex token."
	}
	return c.JSON(http.StatusOK, plexPingResponse{Reachable: false, Error: msg})
}
