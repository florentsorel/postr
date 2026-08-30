package handler

import (
	"log/slog"
	"net/http"

	"github.com/florentsorel/postr/internal/config"
	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/florentsorel/postr/internal/posters"
	"github.com/labstack/echo/v5"
)

type Handler struct {
	db     *db.Queries
	config *config.Config
	// server is the active media server client, or nil when none is configured.
	server mediaserver.Client
	// source is the *inactive* server's client, built only when its credentials
	// are also set. It exists solely so posters imported from a previous server
	// can be carried over; nothing else in the app reads from it.
	source   mediaserver.Client
	sessions *sessionStore
}

func New(queries *db.Queries, cfg *config.Config, client mediaserver.Client) *Handler {
	return &Handler{db: queries, config: cfg, server: client, sessions: newSessionStore()}
}

// WithSource attaches the inactive server's client, enabling poster migration.
func (h *Handler) WithSource(client mediaserver.Client) *Handler {
	h.source = client
	return h
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

// posterDir returns the directory holding posters of one media type for the
// active provider.
func (h *Handler) posterDir(mediaType string) string {
	return posters.Dir(h.config.DataPath, h.provider(), mediaType)
}

// posterPath returns the path of a single locally stored poster.
func (h *Handler) posterPath(mediaType, itemID, ext string) string {
	return posters.Path(h.config.DataPath, h.provider(), mediaType, itemID, ext)
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
