package config_test

import (
	"strings"
	"testing"

	"github.com/florentsorel/postr/internal/config"
)

func TestLoad_PlexURLNormalize(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr string
	}{
		{
			name:  "empty stays empty",
			input: "",
			want:  "",
		},
		{
			name:  "bare host:port gets http scheme",
			input: "192.168.1.120:32400",
			want:  "http://192.168.1.120:32400",
		},
		{
			name:  "http scheme kept as-is",
			input: "http://192.168.1.120:32400",
			want:  "http://192.168.1.120:32400",
		},
		{
			name:  "https scheme kept as-is",
			input: "https://plex.example.com",
			want:  "https://plex.example.com",
		},
		{
			name:  "trailing slash stripped",
			input: "http://192.168.1.120:32400/",
			want:  "http://192.168.1.120:32400",
		},
		{
			name:  "path and query stripped",
			input: "http://192.168.1.120:32400/web?foo=bar",
			want:  "http://192.168.1.120:32400",
		},
		{
			name:    "invalid scheme rejected",
			input:   "ftp://192.168.1.120:32400",
			wantErr: "scheme must be http or https",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.input != "" {
				t.Setenv("PLEX_URL", tt.input)
			}
			cfg, err := config.Load()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not mention %q", err.Error(), tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.PlexURL != tt.want {
				t.Errorf("PlexURL = %q, want %q", cfg.PlexURL, tt.want)
			}
		})
	}
}

func TestLoad_AuthValidation(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr string
	}{
		{
			name:    "auth enabled without user",
			env:     map[string]string{"AUTH_ENABLED": "true", "AUTH_PASS": "secret"},
			wantErr: "AUTH_USER",
		},
		{
			name:    "auth enabled without pass",
			env:     map[string]string{"AUTH_ENABLED": "true", "AUTH_USER": "admin"},
			wantErr: "AUTH_PASS",
		},
		{
			name: "auth enabled with both set",
			env:  map[string]string{"AUTH_ENABLED": "true", "AUTH_USER": "admin", "AUTH_PASS": "secret"},
		},
		{
			name: "auth disabled, credentials not required",
			env:  map[string]string{"AUTH_ENABLED": "false"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			_, err := config.Load()
			if tt.wantErr != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("error %q does not mention %q", err.Error(), tt.wantErr)
				}
			} else if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
		})
	}
}

func TestLoad_MediaServerResolution(t *testing.T) {
	tests := []struct {
		name         string
		env          map[string]string
		want         string
		wantURL      string
		wantToken    string
		wantConfig   bool
		wantErrParts string
	}{
		{
			name:       "defaults to plex when nothing is set",
			env:        map[string]string{},
			want:       "plex",
			wantConfig: false,
		},
		{
			name:       "existing Plex deployment keeps working untouched",
			env:        map[string]string{"PLEX_URL": "plex:32400", "PLEX_TOKEN": "tok"},
			want:       "plex",
			wantURL:    "http://plex:32400",
			wantToken:  "tok",
			wantConfig: true,
		},
		{
			name:       "inferred from JELLYFIN_URL when no Plex URL is set",
			env:        map[string]string{"JELLYFIN_URL": "jelly:8096", "JELLYFIN_API_KEY": "key"},
			want:       "jellyfin",
			wantURL:    "http://jelly:8096",
			wantToken:  "key",
			wantConfig: true,
		},
		{
			name: "explicit MEDIA_SERVER wins over a leftover Plex config",
			env: map[string]string{
				"MEDIA_SERVER": "jellyfin", "JELLYFIN_URL": "https://jelly.example.com/", "JELLYFIN_API_KEY": "key",
				"PLEX_URL": "http://plex:32400", "PLEX_TOKEN": "old",
			},
			want:       "jellyfin",
			wantURL:    "https://jelly.example.com",
			wantToken:  "key",
			wantConfig: true,
		},
		{
			name:       "both URLs set without MEDIA_SERVER stays on Plex",
			env:        map[string]string{"PLEX_URL": "plex:32400", "PLEX_TOKEN": "tok", "JELLYFIN_URL": "jelly:8096"},
			want:       "plex",
			wantURL:    "http://plex:32400",
			wantToken:  "tok",
			wantConfig: true,
		},
		{
			name:         "unknown MEDIA_SERVER is rejected",
			env:          map[string]string{"MEDIA_SERVER": "emby"},
			wantErrParts: "MEDIA_SERVER",
		},
		{
			name:         "invalid JELLYFIN_URL scheme is rejected",
			env:          map[string]string{"JELLYFIN_URL": "ftp://jelly:8096"},
			wantErrParts: "JELLYFIN_URL",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			cfg, err := config.Load()
			if tt.wantErrParts != "" {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				if !strings.Contains(err.Error(), tt.wantErrParts) {
					t.Errorf("error %q does not mention %q", err.Error(), tt.wantErrParts)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if cfg.MediaServer != tt.want {
				t.Errorf("MediaServer = %q, want %q", cfg.MediaServer, tt.want)
			}
			if cfg.ServerURL() != tt.wantURL {
				t.Errorf("ServerURL() = %q, want %q", cfg.ServerURL(), tt.wantURL)
			}
			if cfg.ServerToken() != tt.wantToken {
				t.Errorf("ServerToken() = %q, want %q", cfg.ServerToken(), tt.wantToken)
			}
			if cfg.ServerConfigured() != tt.wantConfig {
				t.Errorf("ServerConfigured() = %v, want %v", cfg.ServerConfigured(), tt.wantConfig)
			}
		})
	}
}
