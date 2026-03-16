package handler

import (
	"net/http"
	"time"

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
	if h.config.PlexURL == "" || h.config.PlexToken == "" {
		return c.JSON(http.StatusOK, plexPingResponse{
			Reachable: false,
			Error:     "Plex is not configured.",
		})
	}

	client := &http.Client{Timeout: 5 * time.Second}
	req, err := http.NewRequestWithContext(
		c.Request().Context(),
		http.MethodGet,
		h.config.PlexURL+"/library/sections",
		nil,
	)
	if err != nil {
		return c.JSON(http.StatusOK, plexPingResponse{Reachable: false, Error: err.Error()})
	}
	req.Header.Set("X-Plex-Token", h.config.PlexToken)

	resp, err := client.Do(req)
	if err != nil {
		return c.JSON(http.StatusOK, plexPingResponse{Reachable: false, Error: "Unable to reach Plex server."})
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return c.JSON(http.StatusOK, plexPingResponse{Reachable: false, Error: "Invalid Plex token."})
	}
	if resp.StatusCode != http.StatusOK {
		return c.JSON(http.StatusOK, plexPingResponse{Reachable: false, Error: "Plex returned an unexpected response."})
	}

	return c.JSON(http.StatusOK, plexPingResponse{Reachable: true})
}
