package handler

import (
	"net/http"

	"github.com/florentsorel/postr/db"
	"github.com/florentsorel/postr/internal/config"
	"github.com/florentsorel/postr/internal/plex"
	"github.com/labstack/echo/v5"
)

type Handler struct {
	db     *db.Queries
	config *config.Config
	plex   *plex.Client
}

func New(queries *db.Queries, cfg *config.Config, plexClient *plex.Client) *Handler {
	return &Handler{db: queries, config: cfg, plex: plexClient}
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
