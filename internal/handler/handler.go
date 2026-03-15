package handler

import (
	"github.com/florentsorel/postr/db"
	"github.com/florentsorel/postr/internal/config"
)

type Handler struct {
	db     *db.Queries
	config *config.Config
}

func New(queries *db.Queries, cfg *config.Config) *Handler {
	return &Handler{db: queries, config: cfg}
}
