package main

import (
	"os"

	"github.com/florentsorel/postr/db"
	"github.com/florentsorel/postr/internal/config"
	"github.com/florentsorel/postr/internal/handler"
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

	h := handler.New(db.New(conn), cfg)

	api := e.Group("/api")
	api.GET("/settings", h.GetSettings)
	api.POST("/settings", h.SaveSettings)

	if err := e.Start(":8080"); err != nil {
		e.Logger.Error("failed to start server", "error", err)
	}
}
