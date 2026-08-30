package handler

import (
	"errors"
	"net/http"

	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/labstack/echo/v5"
)

type serverStatusResponse struct {
	Configured bool   `json:"configured"`
	Provider   string `json:"provider"`
	Name       string `json:"name"`
}

type serverPingResponse struct {
	Reachable bool   `json:"reachable"`
	Error     string `json:"error,omitempty"`
}

func (h *Handler) GetServerStatus(c *echo.Context) error {
	return c.JSON(http.StatusOK, serverStatusResponse{
		Configured: h.config.ServerConfigured(),
		Provider:   h.provider(),
		Name:       h.serverName(),
	})
}

func (h *Handler) PingServer(c *echo.Context) error {
	if h.server == nil {
		return c.JSON(http.StatusOK, serverPingResponse{
			Reachable: false,
			Error:     h.serverName() + " is not configured.",
		})
	}

	if err := h.server.Ping(c.Request().Context()); err != nil {
		return c.JSON(http.StatusOK, serverPingResponse{Reachable: false, Error: h.unreachableMessage(err)})
	}
	return c.JSON(http.StatusOK, serverPingResponse{Reachable: true})
}

// invalidCredentialMessage names the rejected credential of the active server,
// without trailing punctuation so callers can compose it.
func (h *Handler) invalidCredentialMessage() string {
	if h.provider() == mediaserver.ProviderJellyfin {
		return "Invalid Jellyfin API key"
	}
	return "Invalid Plex token"
}

// unreachableMessage turns a connection error into a standalone sentence.
func (h *Handler) unreachableMessage(err error) string {
	if errors.Is(err, mediaserver.ErrUnauthorized) {
		return h.invalidCredentialMessage() + "."
	}
	return "Unable to reach " + h.serverName() + " server."
}

// credentialEnvVar returns the environment variable holding the credential of
// the active server, for use in error messages.
func (h *Handler) credentialEnvVar() string {
	if h.provider() == mediaserver.ProviderJellyfin {
		return "JELLYFIN_API_KEY"
	}
	return "PLEX_TOKEN"
}

// urlEnvVar returns the environment variable holding the URL of the active
// server, for use in error messages.
func (h *Handler) urlEnvVar() string {
	if h.provider() == mediaserver.ProviderJellyfin {
		return "JELLYFIN_URL"
	}
	return "PLEX_URL"
}

// connectionErrorMessage builds the toast shown when an action fails before it
// could reach the server, e.g. "Failed to push poster to Jellyfin. Invalid
// Jellyfin API key — check your JELLYFIN_API_KEY setting."
func (h *Handler) connectionErrorMessage(prefix string, err error) string {
	if errors.Is(err, mediaserver.ErrUnauthorized) {
		return prefix + " " + h.invalidCredentialMessage() + " — check your " + h.credentialEnvVar() + " setting."
	}
	return prefix + " Unable to reach " + h.serverName() + " — check your " + h.urlEnvVar() + " setting."
}
