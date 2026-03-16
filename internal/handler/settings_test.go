package handler_test

import (
	"net/http"
	"testing"

	"github.com/florentsorel/postr/internal/config"
)

func TestGetSettings(t *testing.T) {
	t.Run("reflects env-based config values", func(t *testing.T) {
		cfg := &config.Config{
			PlexURL:     "http://plex:32400",
			PlexToken:   "secret",
			AuthEnabled: true,
			AuthUser:    "admin",
		}
		setup := newTestSetupWithCfg(t, cfg, nil)
		rec, c := newCtx(t, http.MethodGet, "/api/settings", "")

		if err := setup.handler.GetSettings(c); err != nil {
			t.Fatalf("GetSettings: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status: want %d, got %d", http.StatusOK, rec.Code)
		}

		type settingsResp struct {
			PlexURL      string `json:"plex_url"`
			PlexTokenSet bool   `json:"plex_token_set"`
			AuthEnabled  bool   `json:"auth_enabled"`
			AuthUser     string `json:"auth_user"`
			AuthPassSet  bool   `json:"auth_pass_set"`
		}
		resp := decodeJSON[settingsResp](t, rec.Body.Bytes())

		if resp.PlexURL != "http://plex:32400" {
			t.Errorf("plex_url: want %q, got %q", "http://plex:32400", resp.PlexURL)
		}
		if !resp.PlexTokenSet {
			t.Error("plex_token_set: want true")
		}
		if !resp.AuthEnabled {
			t.Error("auth_enabled: want true")
		}
		if resp.AuthUser != "admin" {
			t.Errorf("auth_user: want 'admin', got %q", resp.AuthUser)
		}
		if resp.AuthPassSet {
			t.Error("auth_pass_set: want false (no password set)")
		}
	})

	t.Run("auto_resize defaults to true", func(t *testing.T) {
		setup := newTestSetup(t, nil)
		rec, c := newCtx(t, http.MethodGet, "/api/settings", "")

		if err := setup.handler.GetSettings(c); err != nil {
			t.Fatalf("GetSettings: %v", err)
		}
		type settingsResp struct {
			AutoResize bool `json:"auto_resize"`
		}
		resp := decodeJSON[settingsResp](t, rec.Body.Bytes())
		if !resp.AutoResize {
			t.Error("auto_resize: want true (default)")
		}
	})
}

func TestSaveSettings(t *testing.T) {
	t.Run("round-trip auto_resize", func(t *testing.T) {
		setup := newTestSetup(t, nil)

		_, saveCtx := newCtx(t, http.MethodPost, "/api/settings", `{"sources":[],"options":{"autoResize":false}}`)
		if err := setup.handler.SaveSettings(saveCtx); err != nil {
			t.Fatalf("SaveSettings: %v", err)
		}

		rec, c := newCtx(t, http.MethodGet, "/api/settings", "")
		if err := setup.handler.GetSettings(c); err != nil {
			t.Fatalf("GetSettings: %v", err)
		}
		type settingsResp struct {
			AutoResize bool `json:"auto_resize"`
		}
		resp := decodeJSON[settingsResp](t, rec.Body.Bytes())
		if resp.AutoResize {
			t.Error("auto_resize: want false after saving false")
		}
	})

	t.Run("invalid body returns 400", func(t *testing.T) {
		setup := newTestSetup(t, nil)
		rec, c := newCtx(t, http.MethodPost, "/api/settings", `not json`)

		if err := setup.handler.SaveSettings(c); err != nil {
			t.Fatalf("SaveSettings: %v", err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status: want %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}
