package main

import (
	"io"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/florentsorel/postr/internal/config"
	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/handler"
	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/florentsorel/postr/internal/mediaserver/jellyfin"
	"github.com/florentsorel/postr/internal/mediaserver/plex"
	"github.com/florentsorel/postr/internal/web"
	"github.com/labstack/echo/v5"
	"github.com/labstack/echo/v5/middleware"
	"github.com/lmittmann/tint"
)

func main() {
	slog.SetDefault(slog.New(tint.NewTextHandler(os.Stdout, nil)))

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

	// Postr talks to one media server at a time; which one is decided by
	// MEDIA_SERVER, or inferred from whichever server's URL is set.
	var serverClient mediaserver.Client
	if cfg.ServerConfigured() {
		switch cfg.MediaServer {
		case mediaserver.ProviderJellyfin:
			serverClient = jellyfin.NewClient(cfg.JellyfinURL, cfg.JellyfinAPIKey)
		default:
			serverClient = plex.NewClient(cfg.PlexURL, cfg.PlexToken)
		}
	}
	slog.Info("media server", "provider", cfg.MediaServer, "configured", cfg.ServerConfigured())

	h := handler.New(db.New(conn), cfg, serverClient)

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

	api.GET("/server/status", h.GetServerStatus)
	api.GET("/server/ping", h.PingServer)
	api.POST("/server/import", h.ImportFromServer)
	api.POST("/server/sync", h.SyncFromServer)

	// SPA fallback — serve embedded frontend for all non-API routes
	e.GET("/*", echo.WrapHandler(web.Handler()))

	slog.Info("server starting", "addr", ":8080")
	if err := e.Start(":8080"); err != nil {
		slog.Error("server stopped", "error", err)
	}
}
