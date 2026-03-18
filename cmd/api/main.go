package main

import (
	"os"

	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/config"
	"github.com/florentsorel/postr/internal/handler"
	"github.com/florentsorel/postr/internal/plex"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func main() {
	e := echo.New()
	e.Use(middleware.RequestLogger())

	cfg, err := config.Load()
	if err != nil {
		e.Logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		e.Logger.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	var plexClient *plex.Client
	if cfg.PlexURL != "" && cfg.PlexToken != "" {
		plexClient = plex.NewClient(cfg.PlexURL, cfg.PlexToken)
	}

	h := handler.New(db.New(conn), cfg, plexClient)

	// Public auth routes
	e.POST("/api/auth/login", h.Login)
	e.POST("/api/auth/logout", h.Logout)
	e.GET("/api/auth/check", h.AuthCheck)

	// Protected routes
	api := e.Group("/api", h.RequireAuth)
	api.GET("/settings", h.GetSettings)
	api.POST("/settings", h.SaveSettings)

	api.GET("/libraries", h.GetLibraries)
	api.POST("/libraries", h.SaveLibraries)

	api.GET("/media", h.GetMedia)
	api.GET("/media/:ratingKey/thumb", h.GetMediaThumb)
	api.POST("/media/:ratingKey/upload", h.UploadMediaPoster)
	api.POST("/media/:ratingKey/upload-url", h.UploadPosterFromURL)
	api.POST("/media/:ratingKey/push", h.PushPoster)

	api.GET("/queue", h.GetQueue)
	api.DELETE("/queue/:ratingKey", h.RemoveFromQueue)
	api.POST("/queue/push-all", h.PushAllPosters)

	api.GET("/plex/status", h.GetPlexStatus)
	api.GET("/plex/ping", h.PingPlex)
	api.POST("/plex/import", h.ImportFromPlex)
	api.POST("/plex/sync", h.SyncFromPlex)

	if err := e.Start(":8080"); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
}
