package handler_test

import (
	"bytes"
	"database/sql"
	"image"
	"image/color"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/florentsorel/postr/internal/posters"
	"github.com/labstack/echo/v5"
)

// uploadFromURL drives POST /api/media/:ratingKey/upload-url and returns the
// status and decoded body.
func uploadFromURL(t *testing.T, setup *testSetup, ratingKey, url string) (int, map[string]string) {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/media/"+ratingKey+"/upload-url",
		strings.NewReader(`{"url":"`+url+`"}`))
	req.Header.Set(echo.HeaderContentType, echo.MIMEApplicationJSON)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "ratingKey", Value: ratingKey}})

	if err := setup.handler.UploadPosterFromURL(c); err != nil {
		t.Fatalf("UploadPosterFromURL: %v", err)
	}
	return rec.Code, decodeJSON[map[string]string](t, rec.Body.Bytes())
}

// importedSetup gives a handler with one imported movie to attach posters to.
func importedSetup(t *testing.T) *testSetup {
	t.Helper()
	setup := newTestSetup(t, defaultMock())
	runImport(t, setup.handler, importBody)
	return setup
}

// jpegBytes builds a payload whose first bytes make http.DetectContentType
// report image/jpeg, for the cases where the server declares nothing useful.
func jpegBytes(size int) []byte {
	data := make([]byte, size)
	copy(data, []byte{0xFF, 0xD8, 0xFF, 0xE0})
	return data
}

// TestUploadPosterFromURL_SendsAnIdentifyingUserAgent covers the reason
// ThePosterDB answered 403: Go's default agent is rejected by the bot
// protection in front of several poster sites.
func TestUploadPosterFromURL_SendsAnIdentifyingUserAgent(t *testing.T) {
	var gotUA string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotUA = r.Header.Get("User-Agent")
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(jpegBytes(64))
	}))
	defer srv.Close()

	setup := importedSetup(t)
	if code, _ := uploadFromURL(t, setup, "101", srv.URL+"/asset/1"); code != http.StatusOK {
		t.Fatalf("status: want 200, got %d", code)
	}

	if gotUA == "" || strings.HasPrefix(gotUA, "Go-http-client") {
		t.Errorf("User-Agent = %q; the Go default is what gets blocked", gotUA)
	}
	if !strings.Contains(gotUA, "Postr") {
		t.Errorf("User-Agent should identify Postr, got %q", gotUA)
	}
}

// TestUploadPosterFromURL_RefusesOversizedInsteadOfTruncating is the bug that
// would have bitten silently: a poster over the cap used to be cut short and
// stored as a corrupt image.
func TestUploadPosterFromURL_RefusesOversizedInsteadOfTruncating(t *testing.T) {
	huge := jpegBytes(33 << 20) // just past the 32 MB ceiling
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(huge)
	}))
	defer srv.Close()

	setup := importedSetup(t)
	code, body := uploadFromURL(t, setup, "101", srv.URL+"/huge.jpg")

	if code == http.StatusOK {
		t.Fatal("an oversized image must be refused, not silently truncated")
	}
	if !strings.Contains(body["error"], "larger than") {
		t.Errorf("the error should say the image is too large, got %q", body["error"])
	}

	// Nothing may have been written to disk.
	path := posters.Path(setup.dataPath, mediaserver.ProviderPlex, "movie", "101", "jpg")
	if data, err := os.ReadFile(path); err == nil && len(data) >= 32<<20 {
		t.Error("a truncated poster was stored anyway")
	}
}

// TestUploadPosterFromURL_RefusesNonImages stops an HTML error page served
// under a 200 from being stored and later pushed to the media server.
func TestUploadPosterFromURL_RefusesNonImages(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=UTF-8")
		_, _ = w.Write([]byte("<html><body>Attention Required! | Cloudflare</body></html>"))
	}))
	defer srv.Close()

	setup := importedSetup(t)
	// The path even ends in .jpg — only the content can be trusted.
	code, body := uploadFromURL(t, setup, "101", srv.URL+"/poster.jpg")

	if code == http.StatusOK {
		t.Fatal("an HTML page must not be accepted as a poster")
	}
	if !strings.Contains(body["error"], "JPEG, PNG or WEBP") {
		t.Errorf("error should name the accepted formats, got %q", body["error"])
	}
}

// TestUploadPosterFromURL_ExtensionFromContentDisposition covers the shape of a
// ThePosterDB asset URL: no extension in the path, the filename in a header.
func TestUploadPosterFromURL_ExtensionFromContentDisposition(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		w.Header().Set("Content-Disposition", `inline; filename="The Super Mario Bros. Movie (2023).png"`)
		_, _ = w.Write(jpegBytes(64))
	}))
	defer srv.Close()

	setup := importedSetup(t)
	code, body := uploadFromURL(t, setup, "101", srv.URL+"/api/assets/272535")

	if code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%v)", code, body)
	}
	if body["ext"] != "png" {
		t.Errorf("ext = %q, want png from the Content-Disposition filename", body["ext"])
	}
}

// TestUploadPosterFromURL_SniffsWhenServerDeclaresNothing falls back to the
// bytes themselves when neither header helps.
func TestUploadPosterFromURL_SniffsWhenServerDeclaresNothing(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/octet-stream")
		_, _ = w.Write(jpegBytes(64))
	}))
	defer srv.Close()

	setup := importedSetup(t)
	code, body := uploadFromURL(t, setup, "101", srv.URL+"/api/assets/272535")

	if code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%v)", code, body)
	}
	if body["ext"] != "jpg" {
		t.Errorf("ext = %q, want jpg sniffed from the payload", body["ext"])
	}
}

func TestUploadPosterFromURL_StoresAndQueuesThePoster(t *testing.T) {
	const payload = "poster-bytes-from-the-web"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(append(jpegBytes(4), []byte(payload)...))
	}))
	defer srv.Close()

	setup := importedSetup(t)
	if code, body := uploadFromURL(t, setup, "101", srv.URL+"/p.jpg"); code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%v)", code, body)
	}

	stored, err := os.ReadFile(posters.Path(setup.dataPath, mediaserver.ProviderPlex, "movie", "101", "jpg"))
	if err != nil {
		t.Fatalf("poster not stored: %v", err)
	}
	if !strings.Contains(string(stored), payload) {
		t.Error("the stored file does not hold what the URL served")
	}

	queued, err := setup.queries.ListPosterQueueWithMedia(t.Context(), mediaserver.ProviderPlex)
	if err != nil {
		t.Fatalf("ListPosterQueueWithMedia: %v", err)
	}
	if len(queued) != 1 || queued[0].RatingKey != "101" {
		t.Errorf("the poster should be queued for push, got %+v", queued)
	}
}

func TestUploadPosterFromURL_ReportsUpstreamStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	setup := importedSetup(t)
	code, body := uploadFromURL(t, setup, "101", srv.URL+"/blocked.jpg")

	if code != http.StatusBadGateway {
		t.Errorf("status: want 502, got %d", code)
	}
	if !strings.Contains(body["error"], "403") {
		t.Errorf("the error should carry the upstream status, got %q", body["error"])
	}
}

// bigJPEG builds a poster-shaped image wider than any sane display, standing in
// for what poster sites actually serve.
func bigJPEG(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x % 256), G: uint8(y % 256), B: 90, A: 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 95}); err != nil {
		t.Fatalf("encode: %v", err)
	}
	return buf.Bytes()
}

func enableAutoResize(t *testing.T, setup *testSetup, width string) {
	t.Helper()
	ctx := t.Context()
	for _, s := range []struct{ key, value string }{{"auto_resize", "true"}, {"resize_width", width}} {
		if err := setup.queries.UpdateSetting(ctx, db.UpdateSettingParams{
			Value: sql.NullString{String: s.value, Valid: true}, Type: "option", Key: s.key,
		}); err != nil {
			t.Fatalf("set %s: %v", s.key, err)
		}
	}
}

func servePoster(t *testing.T, body []byte) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/jpeg")
		_, _ = w.Write(body)
	}))
	t.Cleanup(srv.Close)
	return srv
}

// TestUploadPosterFromURL_ResizesWhenEnabled is the point of the whole feature:
// a poster site serves multi-megabyte scans, and a URL upload used to bypass the
// resizing that file uploads got in the browser.
func TestUploadPosterFromURL_ResizesWhenEnabled(t *testing.T) {
	original := bigJPEG(t, 3158, 4737) // the dimensions ThePosterDB serves
	srv := servePoster(t, original)

	setup := importedSetup(t)
	enableAutoResize(t, setup, "1000")

	if code, body := uploadFromURL(t, setup, "101", srv.URL+"/api/assets/272535"); code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%v)", code, body)
	}

	stored, err := os.ReadFile(posters.Path(setup.dataPath, mediaserver.ProviderPlex, "movie", "101", "jpg"))
	if err != nil {
		t.Fatalf("poster not stored: %v", err)
	}
	cfg, _, err := image.DecodeConfig(bytes.NewReader(stored))
	if err != nil {
		t.Fatalf("stored poster is not a valid image: %v", err)
	}
	if cfg.Width != 1000 {
		t.Errorf("stored width = %d, want 1000", cfg.Width)
	}
	if cfg.Height != 1500 {
		t.Errorf("stored height = %d, want 1500 (aspect ratio preserved)", cfg.Height)
	}
	if len(stored) >= len(original) {
		t.Errorf("stored poster is not smaller: %d -> %d bytes", len(original), len(stored))
	}
}

// TestUploadPosterFromURL_KeepsFullSizeWhenDisabled pins the default: the
// setting ships off, and nothing should silently re-encode a user's artwork.
func TestUploadPosterFromURL_KeepsFullSizeWhenDisabled(t *testing.T) {
	original := bigJPEG(t, 2000, 3000)
	srv := servePoster(t, original)

	setup := importedSetup(t)

	if code, body := uploadFromURL(t, setup, "101", srv.URL+"/p.jpg"); code != http.StatusOK {
		t.Fatalf("status: want 200, got %d (%v)", code, body)
	}

	stored, err := os.ReadFile(posters.Path(setup.dataPath, mediaserver.ProviderPlex, "movie", "101", "jpg"))
	if err != nil {
		t.Fatalf("poster not stored: %v", err)
	}
	if !bytes.Equal(stored, original) {
		t.Error("with auto_resize off the poster must be stored byte-for-byte")
	}
}
