package handler_test

import (
	"bytes"
	"net/http"
	"net/http/httptest"
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
			if !strings.Contains(item.Thumb, "/thumb") {
				t.Errorf("item %q: thumb URL want /thumb, got %q", item.Title, item.Thumb)
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

func TestUploadPosterFromURL(t *testing.T) {
	t.Run("unknown ratingKey returns 404", func(t *testing.T) {
		setup := newTestSetup(t, nil)
		rec, c := newCtx(t, http.MethodPost, "/api/media/999/upload-url", `{"url":"http://example.com/poster.jpg"}`)
		c.SetPathValues(echo.PathValues{{Name: "ratingKey", Value: "999"}})

		if err := setup.handler.UploadPosterFromURL(c); err != nil {
			t.Fatalf("UploadPosterFromURL: %v", err)
		}
		if rec.Code != http.StatusNotFound {
			t.Errorf("status: want %d, got %d", http.StatusNotFound, rec.Code)
		}
	})

	t.Run("missing url returns 400", func(t *testing.T) {
		setup := newTestSetup(t, defaultMock())
		runImport(t, setup.handler, importBody)

		rec, c := newCtx(t, http.MethodPost, "/api/media/101/upload-url", `{"url":""}`)
		c.SetPathValues(echo.PathValues{{Name: "ratingKey", Value: "101"}})

		if err := setup.handler.UploadPosterFromURL(c); err != nil {
			t.Fatalf("UploadPosterFromURL: %v", err)
		}
		if rec.Code != http.StatusBadRequest {
			t.Errorf("status: want %d, got %d", http.StatusBadRequest, rec.Code)
		}
	})

	t.Run("remote server returns non-200 gives 502", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, "not found", http.StatusNotFound)
		}))
		defer srv.Close()

		setup := newTestSetup(t, defaultMock())
		runImport(t, setup.handler, importBody)

		rec, c := newCtx(t, http.MethodPost, "/api/media/101/upload-url", `{"url":"`+srv.URL+`/poster.jpg"}`)
		c.SetPathValues(echo.PathValues{{Name: "ratingKey", Value: "101"}})

		if err := setup.handler.UploadPosterFromURL(c); err != nil {
			t.Fatalf("UploadPosterFromURL: %v", err)
		}
		if rec.Code != http.StatusBadGateway {
			t.Errorf("status: want %d, got %d", http.StatusBadGateway, rec.Code)
		}
	})

	t.Run("valid jpeg URL saves poster and returns thumb", func(t *testing.T) {
		posterData := []byte("fake-jpeg-data")
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/jpeg")
			_, _ = w.Write(posterData)
		}))
		defer srv.Close()

		setup := newTestSetup(t, defaultMock())
		runImport(t, setup.handler, importBody)

		rec, c := newCtx(t, http.MethodPost, "/api/media/101/upload-url", `{"url":"`+srv.URL+`/poster.jpg"}`)
		c.SetPathValues(echo.PathValues{{Name: "ratingKey", Value: "101"}})

		if err := setup.handler.UploadPosterFromURL(c); err != nil {
			t.Fatalf("UploadPosterFromURL: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status: want %d, got %d", http.StatusOK, rec.Code)
		}
		type resp struct {
			Ext   string `json:"ext"`
			Thumb string `json:"thumb"`
		}
		body := decodeJSON[resp](t, rec.Body.Bytes())
		if body.Ext != "jpg" {
			t.Errorf("ext: want jpg, got %q", body.Ext)
		}
		if !strings.HasPrefix(body.Thumb, "/api/media/101/thumb") {
			t.Errorf("thumb: want prefix /api/media/101/thumb, got %q", body.Thumb)
		}
	})

	t.Run("valid png URL saves poster with png extension", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("fake-png-data"))
		}))
		defer srv.Close()

		setup := newTestSetup(t, defaultMock())
		runImport(t, setup.handler, importBody)

		rec, c := newCtx(t, http.MethodPost, "/api/media/101/upload-url", `{"url":"`+srv.URL+`/poster.png"}`)
		c.SetPathValues(echo.PathValues{{Name: "ratingKey", Value: "101"}})

		if err := setup.handler.UploadPosterFromURL(c); err != nil {
			t.Fatalf("UploadPosterFromURL: %v", err)
		}
		if rec.Code != http.StatusOK {
			t.Errorf("status: want %d, got %d", http.StatusOK, rec.Code)
		}
		type resp struct {
			Ext string `json:"ext"`
		}
		body := decodeJSON[resp](t, rec.Body.Bytes())
		if body.Ext != "png" {
			t.Errorf("ext: want png, got %q", body.Ext)
		}
	})
}
