package handler_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/florentsorel/postr/db"
	"github.com/florentsorel/postr/internal/config"
	"github.com/florentsorel/postr/internal/handler"
	"github.com/florentsorel/postr/internal/plex"
	"github.com/labstack/echo/v5"
)

// testSetup holds a handler and its dependencies for use in tests.
type testSetup struct {
	handler  *handler.Handler
	queries  *db.Queries
	dataPath string
}

func newTestSetup(t *testing.T, plexClient handler.PlexClient) *testSetup {
	t.Helper()
	return newTestSetupWithCfg(t, &config.Config{}, plexClient)
}

func newTestSetupWithCfg(t *testing.T, cfg *config.Config, plexClient handler.PlexClient) *testSetup {
	t.Helper()
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	t.Cleanup(func() { conn.Close() })
	cfg.DataPath = t.TempDir()
	queries := db.New(conn)
	return &testSetup{
		handler:  handler.New(queries, cfg, plexClient),
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

// mockPlex is a configurable Plex client for tests.
type mockPlex struct {
	sectionsFunc      func(ctx context.Context) ([]plex.Section, error)
	allItemsFunc      func(ctx context.Context, sectionKey string) ([]plex.Item, error)
	childrenFunc      func(ctx context.Context, ratingKey string) ([]plex.Item, error)
	collectionsFunc   func(ctx context.Context, sectionKey string) ([]plex.Item, error)
	downloadThumbFunc func(ctx context.Context, thumbPath string) ([]byte, error)
}

func (m *mockPlex) Sections(ctx context.Context) ([]plex.Section, error) {
	return m.sectionsFunc(ctx)
}
func (m *mockPlex) AllItems(ctx context.Context, sectionKey string) ([]plex.Item, error) {
	return m.allItemsFunc(ctx, sectionKey)
}
func (m *mockPlex) Children(ctx context.Context, ratingKey string) ([]plex.Item, error) {
	if m.childrenFunc != nil {
		return m.childrenFunc(ctx, ratingKey)
	}
	return nil, nil
}
func (m *mockPlex) Collections(ctx context.Context, sectionKey string) ([]plex.Item, error) {
	if m.collectionsFunc != nil {
		return m.collectionsFunc(ctx, sectionKey)
	}
	return nil, nil
}
func (m *mockPlex) DownloadThumb(ctx context.Context, thumbPath string) ([]byte, error) {
	if m.downloadThumbFunc != nil {
		return m.downloadThumbFunc(ctx, thumbPath)
	}
	return []byte("fake-poster"), nil
}
