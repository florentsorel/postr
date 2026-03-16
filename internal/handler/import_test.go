package handler_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/florentsorel/postr/internal/handler"
	"github.com/florentsorel/postr/internal/plex"
	"github.com/labstack/echo/v5"
)

type importResult struct {
	Added   int
	Skipped int
	Deleted int
}

func runImport(t *testing.T, h *handler.Handler, body string) importResult {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/plex/import", strings.NewReader(body))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.ImportFromPlex(c); err != nil {
		t.Fatalf("ImportFromPlex: %v", err)
	}
	return parseSSEDone(t, rec.Body.Bytes())
}

func parseSSEDone(t *testing.T, body []byte) importResult {
	t.Helper()
	scanner := bufio.NewScanner(bytes.NewReader(body))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var event map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &event); err != nil {
			continue
		}
		if event["type"] == "done" {
			return importResult{
				Added:   int(event["added"].(float64)),
				Skipped: int(event["skipped"].(float64)),
				Deleted: int(event["deleted"].(float64)),
			}
		}
	}
	t.Fatal("no 'done' SSE event found in response")
	return importResult{}
}

var (
	testSection = plex.Section{Key: "1", Type: "movie", Title: "Movies"}
	testItems   = []plex.Item{
		{RatingKey: "101", Title: "Inception", Thumb: "/thumb/101"},
		{RatingKey: "102", Title: "The Matrix", Thumb: "/thumb/102"},
	}
)

func defaultMock() *mockPlex {
	return &mockPlex{
		sectionsFunc: func(ctx context.Context) ([]plex.Section, error) {
			return []plex.Section{testSection}, nil
		},
		allItemsFunc: func(ctx context.Context, sectionKey string) ([]plex.Item, error) {
			return testItems, nil
		},
	}
}

const importBody = `{"targets":[{"type":"movie","sectionKeys":["1"]}]}`

func TestImport_Added(t *testing.T) {
	setup := newTestSetup(t, defaultMock())
	result := runImport(t, setup.handler, importBody)

	if result.Added != 2 {
		t.Errorf("Added: want 2, got %d", result.Added)
	}
	if result.Skipped != 0 {
		t.Errorf("Skipped: want 0, got %d", result.Skipped)
	}
	if result.Deleted != 0 {
		t.Errorf("Deleted: want 0, got %d", result.Deleted)
	}
}

func TestImport_Skipped(t *testing.T) {
	setup := newTestSetup(t, defaultMock())

	first := runImport(t, setup.handler, importBody)
	if first.Added != 2 {
		t.Fatalf("setup: want Added=2 on first import, got %d", first.Added)
	}

	second := runImport(t, setup.handler, importBody)

	if second.Added != 0 {
		t.Errorf("Added: want 0, got %d", second.Added)
	}
	if second.Skipped != 2 {
		t.Errorf("Skipped: want 2, got %d", second.Skipped)
	}
	if second.Deleted != 0 {
		t.Errorf("Deleted: want 0, got %d", second.Deleted)
	}
}

func TestImport_Deleted(t *testing.T) {
	mock := defaultMock()
	setup := newTestSetup(t, mock)

	first := runImport(t, setup.handler, importBody)
	if first.Added != 2 {
		t.Fatalf("setup: want Added=2 on first import, got %d", first.Added)
	}

	mock.allItemsFunc = func(ctx context.Context, sectionKey string) ([]plex.Item, error) {
		return testItems[:1], nil
	}
	mock.downloadThumbFunc = func(ctx context.Context, thumbPath string) ([]byte, error) {
		return []byte("updated-poster"), nil
	}

	second := runImport(t, setup.handler, importBody)

	if second.Added != 0 {
		t.Errorf("Added: want 0, got %d", second.Added)
	}
	if second.Deleted != 1 {
		t.Errorf("Deleted: want 1, got %d", second.Deleted)
	}
}
