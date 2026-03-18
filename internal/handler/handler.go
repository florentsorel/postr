package handler

import (
	"context"
	"net/http"

	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/config"
	"github.com/florentsorel/postr/internal/plex"
	"github.com/labstack/echo/v5"
)

// PlexClient is the subset of plex.Client operations used by the handler.
type PlexClient interface {
	Sections(ctx context.Context) ([]plex.Section, error)
	AllItems(ctx context.Context, sectionKey string) ([]plex.Item, error)
	Children(ctx context.Context, ratingKey string) ([]plex.Item, error)
	Collections(ctx context.Context, sectionKey string) ([]plex.Item, error)
	DownloadThumb(ctx context.Context, thumbPath string) ([]byte, string, error)
	UploadPoster(ctx context.Context, ratingKey string, data []byte, contentType string) error
}

type Handler struct {
	db       *db.Queries
	config   *config.Config
	plex     PlexClient
	sessions *sessionStore
}

func New(queries *db.Queries, cfg *config.Config, plexClient PlexClient) *Handler {
	return &Handler{db: queries, config: cfg, plex: plexClient, sessions: newSessionStore()}
}

type errorResponse struct {
	Error string `json:"error"`
}

func jsonError(c *echo.Context, status int, msg string) error {
	return c.JSON(status, errorResponse{Error: msg})
}

func jsonInternalError(c *echo.Context) error {
	return jsonError(c, http.StatusInternalServerError, "internal server error")
}
