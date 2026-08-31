package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/handler"
	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/florentsorel/postr/internal/posters"
	"github.com/labstack/echo/v5"
)

func runPushPoster(t *testing.T, h interface{ PushPoster(*echo.Context) error }, ratingKey string) int {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/media/"+ratingKey+"/push", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "ratingKey", Value: ratingKey}})
	if err := h.PushPoster(c); err != nil {
		t.Fatalf("PushPoster: %v", err)
	}
	return rec.Code
}

// simulateLocalChange writes a different poster file to disk and adds the item
// to the queue with locally_modified=1, mimicking what happens after a user
// uploads a new poster via the UI.
func simulateLocalChange(t *testing.T, setup *testSetup, ratingKey, mediaType string, content []byte) {
	t.Helper()
	ctx := context.Background()

	dir := posters.Dir(setup.dataPath, mediaserver.ProviderPlex, mediaType)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, ratingKey+".jpg"), content, 0o644); err != nil {
		t.Fatalf("write poster: %v", err)
	}

	m, err := setup.queries.GetMediaByRatingKey(ctx, ratingKey)
	if err != nil {
		t.Fatalf("GetMediaByRatingKey: %v", err)
	}
	if err := setup.queries.UpsertPosterQueue(ctx, db.UpsertPosterQueueParams{
		MediaID:   m.ID,
		CreatedAt: time.Now().Unix(),
	}); err != nil {
		t.Fatalf("UpsertPosterQueue: %v", err)
	}
	if err := setup.queries.SetLocallyModified(ctx, db.SetLocallyModifiedParams{
		LocallyModified: 1,
		UpdatedAt:       time.Now().Unix(),
		RatingKey:       ratingKey,
	}); err != nil {
		t.Fatalf("SetLocallyModified: %v", err)
	}
}

// TestPushPoster_NoFalseSyncAfterReencode reproduces the bug where syncing
// after a push detected spurious changes because the media server re-encodes uploaded
// images. After a successful push, the local copy must be updated with the
// bytes the server actually stores so the next sync sees no difference.
func TestPushPoster_NoFalseSyncAfterReencode(t *testing.T) {
	mock := defaultMock()
	setup := newTestSetup(t, mock)

	// Import: local files saved as "original-poster".
	runImport(t, setup.handler, importBody)

	// Simulate user uploading a new poster for item 101.
	simulateLocalChange(t, setup, "101", "movie", []byte("user-uploaded-poster"))

	// The server re-encodes the image on its end — what it serves back differs from
	// what we uploaded. Other items keep their original bytes.
	serverStoredVersion := []byte("plex-reencoded-poster")
	mock.downloadFunc = func(_ context.Context, itemID string) ([]byte, string, error) {
		if itemID == "101" {
			return serverStoredVersion, "jpg", nil
		}
		return []byte("fake-poster"), "jpg", nil
	}

	// Push: uploads the local file, then resyncLocalPoster downloads the
	// server's version and updates the local copy.
	code := runPushPoster(t, setup.handler, "101")
	if code != http.StatusNoContent {
		t.Fatalf("PushPoster: want 204, got %d", code)
	}

	// Sync: the server still returns the same re-encoded bytes — local copy must
	// already match, so zero changes should be reported.
	result := runSync(t, setup.handler)

	if result.Changed != 0 {
		t.Errorf("Changed: want 0 after push+resync, got %d (false positives from server-side re-encoding)", result.Changed)
	}
}

func runRemoveFromQueue(t *testing.T, h *handler.Handler, ratingKey string) removeQueueResult {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodDelete, "/api/queue/"+ratingKey, nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	c.SetPathValues(echo.PathValues{{Name: "ratingKey", Value: ratingKey}})
	if err := h.RemoveFromQueue(c); err != nil {
		t.Fatalf("RemoveFromQueue: %v", err)
	}
	if rec.Code != http.StatusOK {
		t.Fatalf("RemoveFromQueue: want 200, got %d (%s)", rec.Code, rec.Body.String())
	}
	return decodeJSON[removeQueueResult](t, rec.Body.Bytes())
}

type removeQueueResult struct {
	Thumb    string `json:"thumb"`
	Warning  string `json:"warning"`
	Orphaned bool   `json:"orphaned"`
}

// TestRemoveFromQueue_ClearsLocallyModifiedWhenPosterUnchanged covers the case
// where the server already serves the exact bytes the local file holds:
// savePoster reports errPosterUnchanged, which is a successful restore and must
// still clear locally_modified — otherwise the item stays flagged as differing
// from the server forever, showing "Get from {server}" on a card that matches.
func TestRemoveFromQueue_ClearsLocallyModifiedWhenPosterUnchanged(t *testing.T) {
	mock := defaultMock()
	setup := newTestSetup(t, mock)
	ctx := context.Background()

	runImport(t, setup.handler, importBody)

	localPoster := []byte("user-uploaded-poster")
	simulateLocalChange(t, setup, "101", "movie", localPoster)

	// The server serves back exactly what sits on disk — savePoster short-circuits
	// with errPosterUnchanged instead of writing the file.
	mock.downloadFunc = func(_ context.Context, itemID string) ([]byte, string, error) {
		if itemID == "101" {
			return localPoster, "jpg", nil
		}
		return []byte("fake-poster"), "jpg", nil
	}

	resp := runRemoveFromQueue(t, setup.handler, "101")
	if resp.Warning != "" {
		t.Fatalf("Warning: want empty, got %q", resp.Warning)
	}

	m, err := setup.queries.GetMediaByRatingKey(ctx, "101")
	if err != nil {
		t.Fatalf("GetMediaByRatingKey: %v", err)
	}
	if m.LocallyModified != 0 {
		t.Errorf("locally_modified: want 0 after restore, got %d", m.LocallyModified)
	}
	if !m.Thumb.Valid || m.Thumb.String != "jpg" {
		t.Errorf("thumb: want %q, got %v", "jpg", m.Thumb)
	}
}
