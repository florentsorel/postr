package handler_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/florentsorel/postr/internal/config"
	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/labstack/echo/v5"
)

func TestGetServerStatus(t *testing.T) {
	type statusResp struct {
		Configured bool   `json:"configured"`
		Provider   string `json:"provider"`
		Name       string `json:"name"`
	}

	t.Run("not configured when URL and token are empty", func(t *testing.T) {
		setup := newTestSetup(t, nil)
		rec, c := newCtx(t, http.MethodGet, "/api/server/status", "")

		if err := setup.handler.GetServerStatus(c); err != nil {
			t.Fatalf("GetServerStatus: %v", err)
		}
		resp := decodeJSON[statusResp](t, rec.Body.Bytes())
		if resp.Configured {
			t.Error("configured: want false")
		}
	})

	t.Run("configured when Plex URL and token are set", func(t *testing.T) {
		cfg := &config.Config{PlexURL: "http://plex:32400", PlexToken: "secret"}
		setup := newTestSetupWithCfg(t, cfg, nil)
		rec, c := newCtx(t, http.MethodGet, "/api/server/status", "")

		if err := setup.handler.GetServerStatus(c); err != nil {
			t.Fatalf("GetServerStatus: %v", err)
		}
		resp := decodeJSON[statusResp](t, rec.Body.Bytes())
		if !resp.Configured {
			t.Error("configured: want true")
		}
		if resp.Provider != mediaserver.ProviderPlex || resp.Name != "Plex" {
			t.Errorf("provider/name: want plex/Plex, got %s/%s", resp.Provider, resp.Name)
		}
	})

	t.Run("reports Jellyfin when it is the active server", func(t *testing.T) {
		cfg := &config.Config{
			MediaServer:    mediaserver.ProviderJellyfin,
			JellyfinURL:    "http://jellyfin:8096",
			JellyfinAPIKey: "secret",
			// A leftover Plex config must not leak into the response.
			PlexURL:   "http://plex:32400",
			PlexToken: "old",
		}
		setup := newTestSetupWithCfg(t, cfg, nil)
		rec, c := newCtx(t, http.MethodGet, "/api/server/status", "")

		if err := setup.handler.GetServerStatus(c); err != nil {
			t.Fatalf("GetServerStatus: %v", err)
		}
		resp := decodeJSON[statusResp](t, rec.Body.Bytes())
		if !resp.Configured {
			t.Error("configured: want true")
		}
		if resp.Provider != mediaserver.ProviderJellyfin || resp.Name != "Jellyfin" {
			t.Errorf("provider/name: want jellyfin/Jellyfin, got %s/%s", resp.Provider, resp.Name)
		}
	})
}

func TestPingServer(t *testing.T) {
	type pingResp struct {
		Reachable bool   `json:"reachable"`
		Error     string `json:"error"`
	}

	tests := []struct {
		name          string
		cfg           *config.Config
		client        mediaserver.Client
		wantReachable bool
		wantError     string
	}{
		{
			name:          "server not configured",
			client:        nil,
			wantReachable: false,
			wantError:     "Plex is not configured.",
		},
		{
			name: "reachable",
			client: &mockServer{
				pingFunc: func(ctx context.Context) error { return nil },
			},
			wantReachable: true,
		},
		{
			name: "invalid token",
			client: &mockServer{
				pingFunc: func(ctx context.Context) error { return mediaserver.ErrUnauthorized },
			},
			wantReachable: false,
			wantError:     "Invalid Plex token.",
		},
		{
			name: "network error",
			client: &mockServer{
				pingFunc: func(ctx context.Context) error { return errors.New("connection refused") },
			},
			wantReachable: false,
			wantError:     "Unable to reach Plex server.",
		},
		{
			name: "invalid Jellyfin API key is named as such",
			cfg:  &config.Config{MediaServer: mediaserver.ProviderJellyfin},
			client: &mockServer{
				provider: mediaserver.ProviderJellyfin,
				pingFunc: func(ctx context.Context) error { return mediaserver.ErrUnauthorized },
			},
			wantReachable: false,
			wantError:     "Invalid Jellyfin API key.",
		},
		{
			name:          "Jellyfin not configured",
			cfg:           &config.Config{MediaServer: mediaserver.ProviderJellyfin},
			client:        nil,
			wantReachable: false,
			wantError:     "Jellyfin is not configured.",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := tt.cfg
			if cfg == nil {
				cfg = &config.Config{}
			}
			setup := newTestSetupWithCfg(t, cfg, tt.client)
			rec, c := newCtx(t, http.MethodGet, "/api/server/ping", "")

			if err := setup.handler.PingServer(c); err != nil {
				t.Fatalf("PingServer: %v", err)
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

// TestPushPoster_JellyfinErrorMessagesNameTheRightSetting guards against the
// error copy regressing to Plex wording once Jellyfin is the active server.
func TestPushPoster_JellyfinErrorMessagesNameTheRightSetting(t *testing.T) {
	cfg := &config.Config{MediaServer: mediaserver.ProviderJellyfin}
	mock := &mockServer{
		provider:          mediaserver.ProviderJellyfin,
		globalCollections: true,
		librariesFunc: func(ctx context.Context) ([]mediaserver.Library, error) {
			return []mediaserver.Library{{Key: "lib1", Type: mediaserver.TypeMovie, Title: "Movies"}}, nil
		},
		itemsFunc: func(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
			return []mediaserver.Item{{ID: "abc123", Title: "Inception", HasPoster: true}}, nil
		},
	}
	setup := newTestSetupWithCfg(t, cfg, mock)
	runImport(t, setup.handler, `{"targets":[{"type":"movie","sectionKeys":["lib1"]}]}`)
	simulateLocalChange(t, setup, "abc123", "movie", []byte("new-poster"))

	// The API key is rejected: the push must fail before touching Jellyfin.
	mock.pingFunc = func(ctx context.Context) error { return mediaserver.ErrUnauthorized }

	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/media/abc123/push", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "ratingKey", Value: "abc123"}})
	if err := setup.handler.PushPoster(c); err != nil {
		t.Fatalf("PushPoster: %v", err)
	}

	resp := decodeJSON[struct {
		Error string `json:"error"`
	}](t, rec.Body.Bytes())
	want := "Failed to push poster to Jellyfin. Invalid Jellyfin API key — check your JELLYFIN_API_KEY setting."
	if resp.Error != want {
		t.Errorf("error:\n want %q\n  got %q", want, resp.Error)
	}
}
