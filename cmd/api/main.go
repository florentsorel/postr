package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/florentsorel/postr/internal/config"
	"github.com/lmittmann/tint"
	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/handler"
	"github.com/florentsorel/postr/internal/plex"
	"github.com/florentsorel/postr/internal/web"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
)

func main() {
	slog.SetDefault(slog.New(tint.NewHandler(os.Stdout, nil)))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	logsDir := filepath.Join(cfg.DataPath, "logs")
	if err := os.MkdirAll(logsDir, 0755); err != nil {
		slog.Error("failed to create logs directory", "error", err)
		os.Exit(1)
	}

	accessLog, err := os.OpenFile(
		filepath.Join(logsDir, "access.log"),
		os.O_CREATE|os.O_WRONLY|os.O_APPEND,
		0644,
	)
	if err != nil {
		slog.Error("failed to open access log", "error", err)
		os.Exit(1)
	}
	defer accessLog.Close()

	accessLogger := slog.New(slog.NewJSONHandler(accessLog, nil))

	e := echo.New()
	e.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	e.Use(middleware.RequestLoggerWithConfig(middleware.RequestLoggerConfig{
		LogMethod:   true,
		LogURI:      true,
		LogStatus:   true,
		LogLatency:  true,
		LogRemoteIP: true,
		LogValuesFunc: func(c *echo.Context, v middleware.RequestLoggerValues) error {
			accessLogger.Info("REQUEST",
				"method", v.Method,
				"uri", v.URI,
				"status", v.Status,
				"latency_ms", v.Latency.Milliseconds(),
				"remote_ip", v.RemoteIP,
			)
			return nil
		},
	}))

	conn, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("failed to open database", "error", err)
		os.Exit(1)
	}
	defer conn.Close()

	var plexClient handler.PlexClient
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
	api.DELETE("/media/:ratingKey", h.DeleteOrphan)
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

	// SPA fallback — serve embedded frontend for all non-API routes
	e.GET("/*", echo.WrapHandler(web.Handler()))

	slog.Info("server starting", "addr", ":8080")
	if err := e.Start(":8080"); err != nil {
		slog.Error("server stopped", "error", err)
	}
}
