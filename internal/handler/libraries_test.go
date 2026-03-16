package handler_test

import (
	"context"
	"errors"
	"net/http"
	"testing"

	"github.com/florentsorel/postr/internal/handler"
	"github.com/florentsorel/postr/internal/plex"
)

func TestGetLibraries(t *testing.T) {
	type libItem struct {
		Key     string `json:"key"`
		Type    string `json:"type"`
		Enabled bool   `json:"enabled"`
	}
	type libsResp struct {
		Configured bool      `json:"configured"`
		Reachable  bool      `json:"reachable"`
		Error      string    `json:"error"`
		Libraries  []libItem `json:"libraries"`
	}

	t.Run("plex not configured", func(t *testing.T) {
		setup := newTestSetup(t, nil)
		rec, c := newCtx(t, http.MethodGet, "/api/libraries", "")

		if err := setup.handler.GetLibraries(c); err != nil {
			t.Fatalf("GetLibraries: %v", err)
		}
		resp := decodeJSON[libsResp](t, rec.Body.Bytes())
		if resp.Configured {
			t.Error("configured: want false")
		}
	})

	t.Run("plex unreachable", func(t *testing.T) {
		mock := &mockPlex{
			sectionsFunc: func(ctx context.Context) ([]plex.Section, error) {
				return nil, errors.New("connection timeout")
			},
		}
		setup := newTestSetup(t, mock)
		rec, c := newCtx(t, http.MethodGet, "/api/libraries", "")

		if err := setup.handler.GetLibraries(c); err != nil {
			t.Fatalf("GetLibraries: %v", err)
		}
		resp := decodeJSON[libsResp](t, rec.Body.Bytes())
		if !resp.Configured {
			t.Error("configured: want true")
		}
		if resp.Reachable {
			t.Error("reachable: want false")
		}
		if resp.Error != "Unable to reach Plex server." {
			t.Errorf("error: want 'Unable to reach Plex server.', got %q", resp.Error)
		}
	})

	t.Run("invalid token", func(t *testing.T) {
		mock := &mockPlex{
			sectionsFunc: func(ctx context.Context) ([]plex.Section, error) {
				return nil, plex.ErrUnauthorized
			},
		}
		setup := newTestSetup(t, mock)
		rec, c := newCtx(t, http.MethodGet, "/api/libraries", "")

		if err := setup.handler.GetLibraries(c); err != nil {
			t.Fatalf("GetLibraries: %v", err)
		}
		resp := decodeJSON[libsResp](t, rec.Body.Bytes())
		if resp.Error != "Invalid Plex token." {
			t.Errorf("error: want 'Invalid Plex token.', got %q", resp.Error)
		}
	})

	t.Run("enabled by default when no DB state exists", func(t *testing.T) {
		mock := &mockPlex{
			sectionsFunc: func(ctx context.Context) ([]plex.Section, error) {
				return []plex.Section{
					{Key: "1", Type: "movie", Title: "Movies"},
					{Key: "2", Type: "show", Title: "TV Series"},
				}, nil
			},
		}
		setup := newTestSetup(t, mock)
		rec, c := newCtx(t, http.MethodGet, "/api/libraries", "")

		if err := setup.handler.GetLibraries(c); err != nil {
			t.Fatalf("GetLibraries: %v", err)
		}
		resp := decodeJSON[libsResp](t, rec.Body.Bytes())
		if len(resp.Libraries) != 2 {
			t.Fatalf("libraries: want 2, got %d", len(resp.Libraries))
		}
		for _, lib := range resp.Libraries {
			if !lib.Enabled {
				t.Errorf("library %q: want enabled=true by default, got false", lib.Key)
			}
		}
	})

	t.Run("filters out non-movie/show types", func(t *testing.T) {
		mock := &mockPlex{
			sectionsFunc: func(ctx context.Context) ([]plex.Section, error) {
				return []plex.Section{
					{Key: "1", Type: "movie", Title: "Movies"},
					{Key: "2", Type: "music", Title: "Music"},
					{Key: "3", Type: "photo", Title: "Photos"},
				}, nil
			},
		}
		setup := newTestSetup(t, mock)
		rec, c := newCtx(t, http.MethodGet, "/api/libraries", "")

		if err := setup.handler.GetLibraries(c); err != nil {
			t.Fatalf("GetLibraries: %v", err)
		}
		resp := decodeJSON[libsResp](t, rec.Body.Bytes())
		if len(resp.Libraries) != 1 {
			t.Errorf("libraries: want 1 (movie only), got %d", len(resp.Libraries))
		}
		if len(resp.Libraries) == 1 && resp.Libraries[0].Type != "movie" {
			t.Errorf("library type: want movie, got %q", resp.Libraries[0].Type)
		}
	})

	t.Run("respects saved disabled state", func(t *testing.T) {
		mock := &mockPlex{
			sectionsFunc: func(ctx context.Context) ([]plex.Section, error) {
				return []plex.Section{
					{Key: "1", Type: "movie", Title: "Movies"},
				}, nil
			},
		}
		setup := newTestSetup(t, mock)

		// Save library as disabled.
		_, saveCtx := newCtx(t, http.MethodPost, "/api/libraries", `{"libraries":[{"key":"1","enabled":false}]}`)
		if err := setup.handler.SaveLibraries(saveCtx); err != nil {
			t.Fatalf("SaveLibraries: %v", err)
		}

		rec, c := newCtx(t, http.MethodGet, "/api/libraries", "")
		if err := setup.handler.GetLibraries(c); err != nil {
			t.Fatalf("GetLibraries: %v", err)
		}
		resp := decodeJSON[libsResp](t, rec.Body.Bytes())
		if len(resp.Libraries) != 1 {
			t.Fatalf("libraries: want 1, got %d", len(resp.Libraries))
		}
		if resp.Libraries[0].Enabled {
			t.Error("enabled: want false after saving disabled state")
		}
	})
}

func TestSaveLibraries(t *testing.T) {
	t.Run("round-trip enabled state", func(t *testing.T) {
		mock := &mockPlex{
			sectionsFunc: func(ctx context.Context) ([]plex.Section, error) {
				return []plex.Section{
					{Key: "1", Type: "movie", Title: "Movies"},
					{Key: "2", Type: "show", Title: "TV"},
				}, nil
			},
		}
		setup := newTestSetup(t, mock)

		body := `{"libraries":[{"key":"1","enabled":true},{"key":"2","enabled":false}]}`
		_, saveCtx := newCtx(t, http.MethodPost, "/api/libraries", body)
		if err := setup.handler.SaveLibraries(saveCtx); err != nil {
			t.Fatalf("SaveLibraries: %v", err)
		}

		rec, c := newCtx(t, http.MethodGet, "/api/libraries", "")
		if err := setup.handler.GetLibraries(c); err != nil {
			t.Fatalf("GetLibraries: %v", err)
		}

		type libItem struct {
			Key     string `json:"key"`
			Enabled bool   `json:"enabled"`
		}
		type libsResp struct {
			Libraries []libItem `json:"libraries"`
		}
		resp := decodeJSON[libsResp](t, rec.Body.Bytes())

		byKey := make(map[string]bool, len(resp.Libraries))
		for _, l := range resp.Libraries {
			byKey[l.Key] = l.Enabled
		}
		if !byKey["1"] {
			t.Error("key 1: want enabled=true")
		}
		if byKey["2"] {
			t.Error("key 2: want enabled=false")
		}
	})

	t.Run("invalid body returns 400", func(t *testing.T) {
		setup := newTestSetup(t, nil)
		rec, c := newCtx(t, http.MethodPost, "/api/libraries", `not json`)

		if err := setup.handler.SaveLibraries(c); err != nil {
			t.Fatalf("SaveLibraries: %v", err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status: want %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})
}

// Ensure *mockPlex satisfies handler.PlexClient (compile-time check).
var _ handler.PlexClient = (*mockPlex)(nil)
