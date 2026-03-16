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
