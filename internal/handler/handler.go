package handler

import (
	"github.com/florentsorel/postr/db"
	"github.com/florentsorel/postr/internal/config"
)

type Handler struct {
	queries *db.Queries
	config  *config.Config
}

func New(queries *db.Queries, cfg *config.Config) *Handler {
	return &Handler{queries: queries, config: cfg}
}
