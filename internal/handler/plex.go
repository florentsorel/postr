package handler

import (
	"net/http"
	"time"

	"github.com/florentsorel/postr/plex"
	"github.com/labstack/echo/v5"
)

type plexStatusResponse struct {
	Configured bool `json:"configured"`
}

type plexPingResponse struct {
	Reachable bool   `json:"reachable"`
	Error     string `json:"error,omitempty"`
}

type plexSectionsResponse struct {
	Sections []plexSection `json:"sections"`
}

type plexSection struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

func (h *Handler) GetPlexSections(c *echo.Context) error {
	if h.config.PlexURL == "" || h.config.PlexToken == "" {
		return c.JSON(http.StatusOK, plexSectionsResponse{Sections: []plexSection{}})
	}
	client := plex.NewClient(h.config.PlexURL, h.config.PlexToken)
	sections, err := client.Sections(c.Request().Context())
	if err != nil {
		return c.JSON(http.StatusBadGateway, map[string]string{"error": err.Error()})
	}
	out := make([]plexSection, len(sections))
	for i, s := range sections {
		out[i] = plexSection{Key: s.Key, Type: s.Type, Title: s.Title}
	}
	return c.JSON(http.StatusOK, plexSectionsResponse{Sections: out})
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
