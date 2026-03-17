package handler

import (
	"database/sql"
	"net/http"
	"strconv"

	"github.com/florentsorel/postr/internal/db"
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
	PlexURL      string           `json:"plex_url"`
	PlexTokenSet bool             `json:"plex_token_set"`
	AuthEnabled  bool             `json:"auth_enabled"`
	AuthUser     string           `json:"auth_user"`
	AuthPassSet  bool             `json:"auth_pass_set"`
	AutoResize   bool             `json:"auto_resize"`
	ResizeWidth  int              `json:"resize_width"`
	Sources      []sourceResponse `json:"sources"`
}

var sourceMeta = map[string]struct{ label, description string }{
	"tmdb":   {"TMDB", "The Movie Database"},
	"tvdb":   {"TVDB", "The TV Database"},
	"fanart": {"Fanart.tv", "Community artwork"},
}

func (h *Handler) GetSettings(c *echo.Context) error {
	settings, err := h.db.ListSettings(c.Request().Context())
	if err != nil {
		return jsonInternalError(c)
	}

	resp := settingsResponse{
		PlexURL:      h.config.PlexURL,
		PlexTokenSet: h.config.PlexToken != "",
		AuthEnabled:  h.config.AuthEnabled,
		AuthUser:     h.config.AuthUser,
		AuthPassSet:  h.config.AuthPass != "",
		AutoResize:   true,
		ResizeWidth:  1000,
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
				Enabled:     s.Value.Valid && s.Value.String == "true",
				Position:    s.Position.Int64,
			})
		case "option":
			switch s.Key {
			case "auto_resize":
				resp.AutoResize = s.Value.Valid && s.Value.String == "true"
			case "resize_width":
				if s.Value.Valid && s.Value.String != "" {
					if w, err := strconv.Atoi(s.Value.String); err == nil && w > 0 {
						resp.ResizeWidth = w
					}
				}
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
		AutoResize  bool `json:"autoResize"`
		ResizeWidth int  `json:"resizeWidth"`
	} `json:"options"`
}

func (h *Handler) SaveSettings(c *echo.Context) error {
	var req saveSettingsRequest
	if err := c.Bind(&req); err != nil {
		return jsonError(c, http.StatusBadRequest, "invalid request body")
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
			return jsonInternalError(c)
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
		return jsonInternalError(c)
	}

	resizeWidth := req.Options.ResizeWidth
	if req.Options.AutoResize && resizeWidth < 500 {
		return jsonError(c, http.StatusUnprocessableEntity, "Target width must be at least 500px")
	}
	if err := h.db.UpdateSetting(ctx, db.UpdateSettingParams{
		Value: sql.NullString{String: strconv.Itoa(resizeWidth), Valid: true},
		Type:  "option",
		Key:   "resize_width",
	}); err != nil {
		return jsonInternalError(c)
	}

	return c.NoContent(http.StatusNoContent)
}
