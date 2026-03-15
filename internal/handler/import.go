package handler

import (
	"log/slog"
	"net/http"

	"github.com/florentsorel/postr/plex"
	"github.com/labstack/echo/v5"
)

type importTarget struct {
	Type        string   `json:"type"`
	SectionKeys []string `json:"sectionKeys"`
}

type importRequest struct {
	Targets []importTarget `json:"targets"`
}

func (h *Handler) ImportFromPlex(c *echo.Context) error {
	var req importRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
	}

	if h.config.PlexURL == "" || h.config.PlexToken == "" {
		return jsonError(c, http.StatusBadRequest, "Plex is not configured")
	}

	client := plex.NewClient(h.config.PlexURL, h.config.PlexToken)
	ctx := c.Request().Context()

	var errs []string

	for _, target := range req.Targets {
		slog.Info("importing", "type", target.Type, "sections", target.SectionKeys)

		for _, sectionKey := range target.SectionKeys {
			switch target.Type {
			case "movie":
				items, err := client.AllItems(ctx, sectionKey)
				if err != nil {
					slog.Error("failed to fetch movies", "section", sectionKey, "error", err)
					errs = append(errs, err.Error())
					continue
				}
				slog.Info("movies", "section", sectionKey, "count", len(items))
				for _, m := range items {
					slog.Info("movie", "id", m.RatingKey, "title", m.Title, "year", m.Year, "thumb", m.Thumb, "addedAt", m.AddedAt)
				}

			case "show":
				shows, err := client.AllItems(ctx, sectionKey)
				if err != nil {
					slog.Error("failed to fetch shows", "section", sectionKey, "error", err)
					errs = append(errs, err.Error())
					continue
				}
				slog.Info("shows", "section", sectionKey, "count", len(shows))
				for _, s := range shows {
					slog.Info("show", "id", s.RatingKey, "title", s.Title, "year", s.Year, "thumb", s.Thumb, "addedAt", s.AddedAt)
				}

			case "season":
				shows, err := client.AllItems(ctx, sectionKey)
				if err != nil {
					slog.Error("failed to fetch shows for seasons", "section", sectionKey, "error", err)
					errs = append(errs, err.Error())
					continue
				}
				for _, show := range shows {
					seasons, err := client.Children(ctx, show.RatingKey)
					if err != nil {
						slog.Error("failed to fetch seasons", "show", show.Title, "error", err)
						errs = append(errs, err.Error())
						continue
					}
					slog.Info("seasons", "show", show.Title, "count", len(seasons))
					for _, s := range seasons {
						slog.Info("season", "id", s.RatingKey, "show", show.Title, "index", s.Index, "title", s.Title, "thumb", s.Thumb, "addedAt", s.AddedAt)
					}
				}

			case "collection":
				cols, err := client.Collections(ctx, sectionKey)
				if err != nil {
					slog.Error("failed to fetch collections", "section", sectionKey, "error", err)
					errs = append(errs, err.Error())
					continue
				}
				slog.Info("collections", "section", sectionKey, "count", len(cols))
				for _, col := range cols {
					slog.Info("collection", "id", col.RatingKey, "title", col.Title, "thumb", col.Thumb)
				}
			}
		}
	}

	if len(errs) > 0 {
		return jsonError(c, http.StatusBadGateway, errs[0])
	}
	return c.NoContent(http.StatusNoContent)
}
