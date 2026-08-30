package handler

import (
	"log/slog"
	"net/http"

	"github.com/florentsorel/postr/internal/config"
	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/labstack/echo/v5"
)

type Handler struct {
	db     *db.Queries
	config *config.Config
	// server is the active media server client, or nil when none is configured.
	server   mediaserver.Client
	sessions *sessionStore
}

func New(queries *db.Queries, cfg *config.Config, client mediaserver.Client) *Handler {
	return &Handler{db: queries, config: cfg, server: client, sessions: newSessionStore()}
}

// provider returns the identifier of the active media server, used to scope
// every database read to the data imported from that server.
func (h *Handler) provider() string {
	return h.config.MediaServer
}

// serverName returns the display name of the active media server, for use in
// user-facing messages.
func (h *Handler) serverName() string {
	return h.config.ServerName()
}

type errorResponse struct {
	Error string `json:"error"`
}

func jsonError(c *echo.Context, status int, msg string) error {
	return c.JSON(status, errorResponse{Error: msg})
}

func jsonInternalError(c *echo.Context, err error) error {
	slog.Error("internal server error", "error", err, "path", c.Request().URL.Path)
	return jsonError(c, http.StatusInternalServerError, "internal server error")
}
