package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/florentsorel/postr/internal/config"
	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/handler"
	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/labstack/echo/v5"
)

// testSetup holds a handler and its dependencies for use in tests.
type testSetup struct {
	handler  *handler.Handler
	queries  *db.Queries
	dataPath string
}

func newTestSetup(t *testing.T, client mediaserver.Client) *testSetup {
	t.Helper()
	return newTestSetupWithCfg(t, &config.Config{}, client)
}

func newTestSetupWithCfg(t *testing.T, cfg *config.Config, client mediaserver.Client) *testSetup {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	cfg.DataPath = t.TempDir()
	// config.Load resolves this; tests build Config literals directly.
	if cfg.MediaServer == "" {
		cfg.MediaServer = mediaserver.ProviderPlex
	}
	queries := db.New(conn)
	return &testSetup{
		handler:  handler.New(queries, cfg, client),
		queries:  queries,
		dataPath: cfg.DataPath,
	}
}

// newCtx creates an Echo context backed by a response recorder.
func newCtx(t *testing.T, method, path, body string) (*httptest.ResponseRecorder, *echo.Context) {
	t.Helper()
	e := echo.New()
	var r io.Reader
	if body != "" {
		r = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, path, r)
	if body != "" {
		req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	}
	rec := httptest.NewRecorder()
	return rec, e.NewContext(req, rec)
}

func decodeJSON[T any](t *testing.T, body []byte) T {
	t.Helper()
	var v T
	if err := json.Unmarshal(body, &v); err != nil {
		t.Fatalf("decodeJSON: %v\nbody: %s", err, body)
	}
	return v
}

// mockServer is a configurable mediaserver.Client for tests. Unset funcs fall
// back to benign defaults so each test only wires up what it exercises.
type mockServer struct {
	provider          string
	globalCollections bool
	pingFunc          func(ctx context.Context) error
	librariesFunc     func(ctx context.Context) ([]mediaserver.Library, error)
	itemsFunc         func(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error)
	downloadFunc      func(ctx context.Context, itemID string) ([]byte, string, error)
	uploadFunc        func(ctx context.Context, itemID string, data []byte, contentType string) error
}

func (m *mockServer) Provider() string {
	if m.provider != "" {
		return m.provider
	}
	return mediaserver.ProviderPlex
}

func (m *mockServer) Name() string {
	if m.Provider() == mediaserver.ProviderJellyfin {
		return "Jellyfin"
	}
	return "Plex"
}

func (m *mockServer) GlobalCollections() bool { return m.globalCollections }

// Ping defaults to whatever Libraries reports, mirroring the real clients where
// a failing listing means an unreachable server.
func (m *mockServer) Ping(ctx context.Context) error {
	if m.pingFunc != nil {
		return m.pingFunc(ctx)
	}
	if m.librariesFunc != nil {
		_, err := m.librariesFunc(ctx)
		return err
	}
	return nil
}

func (m *mockServer) Libraries(ctx context.Context) ([]mediaserver.Library, error) {
	if m.librariesFunc != nil {
		return m.librariesFunc(ctx)
	}
	return nil, nil
}

func (m *mockServer) Items(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
	if m.itemsFunc != nil {
		return m.itemsFunc(ctx, libraryKey, mediaType)
	}
	return nil, nil
}

func (m *mockServer) DownloadPoster(ctx context.Context, itemID string) ([]byte, string, error) {
	if m.downloadFunc != nil {
		return m.downloadFunc(ctx, itemID)
	}
	return []byte("fake-poster"), "jpg", nil
}

func (m *mockServer) UploadPoster(ctx context.Context, itemID string, data []byte, contentType string) error {
	if m.uploadFunc != nil {
		return m.uploadFunc(ctx, itemID, data, contentType)
	}
	return nil
}

// Ensure *mockServer satisfies mediaserver.Client (compile-time check).
var _ mediaserver.Client = (*mockServer)(nil)
