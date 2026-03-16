package handler_test

import (
	"bytes"
	"net/http"
	"strings"
	"testing"

	"github.com/labstack/echo/v5"
)

func TestGetMedia(t *testing.T) {
	t.Run("empty DB returns empty array", func(t *testing.T) {
		setup := newTestSetup(t, nil)
		rec, c := newCtx(t, http.MethodGet, "/api/media", "")

		if err := setup.handler.GetMedia(c); err != nil {
			t.Fatalf("GetMedia: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status: want %d, got %d", http.StatusOK, rec.Code)
		}
		items := decodeJSON[[]any](t, rec.Body.Bytes())
		if len(items) != 0 {
			t.Errorf("want 0 items, got %d", len(items))
		}
	})

	t.Run("returns items with thumb URL after import", func(t *testing.T) {
		setup := newTestSetup(t, defaultMock())
		runImport(t, setup.handler, importBody)

		rec, c := newCtx(t, http.MethodGet, "/api/media", "")
		if err := setup.handler.GetMedia(c); err != nil {
			t.Fatalf("GetMedia: %v", err)
		}

		type mediaItem struct {
			Title string `json:"title"`
			Thumb string `json:"thumb"`
		}
		items := decodeJSON[[]mediaItem](t, rec.Body.Bytes())
		if len(items) != 2 {
			t.Fatalf("want 2 items, got %d", len(items))
		}
		for _, item := range items {
			if !strings.HasPrefix(item.Thumb, "/api/media/") {
				t.Errorf("item %q: thumb URL want prefix /api/media/, got %q", item.Title, item.Thumb)
			}
			if !strings.HasSuffix(item.Thumb, "/thumb") {
				t.Errorf("item %q: thumb URL want suffix /thumb, got %q", item.Title, item.Thumb)
			}
		}
	})
}

func TestGetMediaThumb(t *testing.T) {
	t.Run("unknown ratingKey returns 404", func(t *testing.T) {
		setup := newTestSetup(t, nil)
		rec, c := newCtx(t, http.MethodGet, "/api/media/999/thumb", "")
		c.SetPathValues(echo.PathValues{{Name: "ratingKey", Value: "999"}})

		if err := setup.handler.GetMediaThumb(c); err != nil {
			t.Fatalf("GetMediaThumb: %v", err)
		}
		if rec.Code != http.StatusNotFound {
			t.Errorf("status: want %d, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("known ratingKey serves poster file", func(t *testing.T) {
		setup := newTestSetup(t, defaultMock())
		runImport(t, setup.handler, importBody)

		rec, c := newCtx(t, http.MethodGet, "/api/media/101/thumb", "")
		c.SetPathValues(echo.PathValues{{Name: "ratingKey", Value: "101"}})

		if err := setup.handler.GetMediaThumb(c); err != nil {
			t.Fatalf("GetMediaThumb: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status: want %d, got %d", http.StatusOK, rec.Code)
		}
		if !bytes.Equal(rec.Body.Bytes(), []byte("fake-poster")) {
			t.Errorf("body: want 'fake-poster', got %q", rec.Body.String())
		}
	})
}
