package handler_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/florentsorel/postr/internal/config"
	"github.com/florentsorel/postr/internal/handler"
	"github.com/florentsorel/postr/internal/plex"
)

func TestGetPlexStatus(t *testing.T) {
	type statusResp struct {
		Configured bool `json:"configured"`
	}

	t.Run("not configured when URL and token are empty", func(t *testing.T) {
		setup := newTestSetup(t, nil)
		rec, c := newCtx(t, http.MethodGet, "/api/plex/status", "")

		if err := setup.handler.GetPlexStatus(c); err != nil {
			t.Fatalf("GetPlexStatus: %v", err)
		}
		resp := decodeJSON[statusResp](t, rec.Body.Bytes())
		if resp.Configured {
			t.Error("configured: want false")
		}
	})

	t.Run("configured when URL and token are set", func(t *testing.T) {
		cfg := &config.Config{PlexURL: "http://plex:32400", PlexToken: "secret"}
		setup := newTestSetupWithCfg(t, cfg, nil)
		rec, c := newCtx(t, http.MethodGet, "/api/plex/status", "")

		if err := setup.handler.GetPlexStatus(c); err != nil {
			t.Fatalf("GetPlexStatus: %v", err)
		}
		resp := decodeJSON[statusResp](t, rec.Body.Bytes())
		if !resp.Configured {
			t.Error("configured: want true")
		}
	})
}

func TestPingPlex(t *testing.T) {
	type pingResp struct {
		Reachable bool   `json:"reachable"`
		Error     string `json:"error"`
	}

	tests := []struct {
		name          string
		plex          handler.PlexClient
		wantReachable bool
		wantError     string
	}{
		{
			name:          "plex not configured",
			plex:          nil,
			wantReachable: false,
			wantError:     "Plex is not configured.",
		},
		{
			name: "reachable",
			plex: &mockPlex{
				sectionsFunc: func(ctx context.Context) ([]plex.Section, error) {
					return []plex.Section{{Key: "1", Type: "movie", Title: "Movies"}}, nil
				},
			},
			wantReachable: true,
		},
		{
			name: "invalid token",
			plex: &mockPlex{
				sectionsFunc: func(ctx context.Context) ([]plex.Section, error) {
					return nil, plex.ErrUnauthorized
				},
			},
			wantReachable: false,
			wantError:     "Invalid Plex token.",
		},
		{
			name: "network error",
			plex: &mockPlex{
				sectionsFunc: func(ctx context.Context) ([]plex.Section, error) {
					return nil, errors.New("connection refused")
				},
			},
			wantReachable: false,
			wantError:     "Unable to reach Plex server.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setup := newTestSetup(t, tt.plex)
			rec, c := newCtx(t, http.MethodGet, "/api/plex/ping", "")

			if err := setup.handler.PingPlex(c); err != nil {
				t.Fatalf("PingPlex: %v", err)
			}
			if rec.Code != http.StatusOK {
				t.Errorf("status: want %d, got %d", http.StatusOK, rec.Code)
			}
			resp := decodeJSON[pingResp](t, rec.Body.Bytes())
			if resp.Reachable != tt.wantReachable {
				t.Errorf("reachable: want %v, got %v", tt.wantReachable, resp.Reachable)
			}
			if resp.Error != tt.wantError {
				t.Errorf("error: want %q, got %q", tt.wantError, resp.Error)
			}
		})
	}
}
