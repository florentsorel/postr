package handler

import (
	"database/sql"
	"net/http"

	"github.com/florentsorel/postr/db"
	"github.com/labstack/echo/v5"
)

type sourceResponse struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Description string `json:"description"`
	Enabled     bool   `json:"enabled"`
	Position    int64  `json:"position"`
}

type settingsResponse struct {
	PlexURL     string           `json:"plex_url"`
	PlexToken   string           `json:"plex_token"`
	AuthEnabled bool             `json:"auth_enabled"`
	AuthUser    string           `json:"auth_user"`
	AuthPassSet bool             `json:"auth_pass_set"`
	AutoResize  bool             `json:"auto_resize"`
	Sources     []sourceResponse `json:"sources"`
}

var sourceMeta = map[string]struct{ label, description string }{
	"tmdb":   {"TMDB", "The Movie Database"},
	"tvdb":   {"TVDB", "The TV Database"},
	"fanart": {"Fanart.tv", "Community artwork"},
}

func (h *Handler) GetSettings(c *echo.Context) error {
	settings, err := h.db.ListSettings(c.Request().Context())
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to fetch settings")
	}

	resp := settingsResponse{
		PlexURL:     h.config.PlexURL,
		PlexToken:   h.config.PlexToken,
		AuthEnabled: h.config.AuthEnabled,
		AuthUser:    h.config.AuthUser,
		AuthPassSet: h.config.AuthPass != "",
		AutoResize:  true,
	}

	for _, s := range settings {
		switch s.Type {
		case "poster_source":
			meta, ok := sourceMeta[s.Key]
			if !ok {
				continue
			}
			resp.Sources = append(resp.Sources, sourceResponse{
				ID:          s.Key,
				Label:       meta.label,
				Description: meta.description,
				Enabled:     s.Value.String == "true",
				Position:    s.Position.Int64,
			})
		case "option":
			if s.Key == "auto_resize" {
				resp.AutoResize = s.Value.String == "true"
			}
		}
	}

	return c.JSON(http.StatusOK, resp)
}

type saveSourceRequest struct {
	ID      string `json:"id"`
	Enabled bool   `json:"enabled"`
}

type saveSettingsRequest struct {
	Sources []saveSourceRequest `json:"sources"`
	Options struct {
		AutoResize bool `json:"autoResize"`
	} `json:"options"`
}

func (h *Handler) SaveSettings(c *echo.Context) error {
	var req saveSettingsRequest
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request body")
	}

	ctx := c.Request().Context()

	for i, s := range req.Sources {
		if _, ok := sourceMeta[s.ID]; !ok {
			continue
		}
		value := "false"
		if s.Enabled {
			value = "true"
		}
		if err := h.db.UpdatePosterSource(ctx, db.UpdatePosterSourceParams{
			Value:    sql.NullString{String: value, Valid: true},
			Position: sql.NullInt64{Int64: int64(i), Valid: true},
			Key:      s.ID,
		}); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, "failed to save source")
		}
	}

	autoResize := "false"
	if req.Options.AutoResize {
		autoResize = "true"
	}
	if err := h.db.UpdateSetting(ctx, db.UpdateSettingParams{
		Value: sql.NullString{String: autoResize, Valid: true},
		Type:  "option",
		Key:   "auto_resize",
	}); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to save options")
	}

	return c.NoContent(http.StatusNoContent)
}
