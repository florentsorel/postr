package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/florentsorel/postr/internal/db"
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

	dir := filepath.Join(setup.dataPath, "posters", mediaType)
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

// TestPushPoster_NoFalseSyncAfterPlexReencode reproduces the bug where syncing
// after a push detected spurious changes because Plex re-encodes uploaded
// images. After a successful push, the local copy must be updated with the
// bytes Plex actually stores so the next sync sees no difference.
func TestPushPoster_NoFalseSyncAfterPlexReencode(t *testing.T) {
	mock := defaultMock()
	setup := newTestSetup(t, mock)

	// Import: local files saved as "original-poster".
	runImport(t, setup.handler, importBody)

	// Simulate user uploading a new poster for item 101.
	simulateLocalChange(t, setup, "101", "movie", []byte("user-uploaded-poster"))

	// Plex re-encodes the image on its end — what it serves back differs from
	// what we uploaded. Other items keep their original bytes.
	plexStoredVersion := []byte("plex-reencoded-poster")
	mock.downloadThumbFunc = func(_ context.Context, thumbPath string) ([]byte, string, error) {
		if strings.Contains(thumbPath, "101") {
			return plexStoredVersion, "jpg", nil
		}
		return []byte("fake-poster"), "jpg", nil
	}

	// Push: uploads the local file, then resyncLocalThumb downloads Plex's
	// version and updates the local copy.
	code := runPushPoster(t, setup.handler, "101")
	if code != http.StatusNoContent {
		t.Fatalf("PushPoster: want 204, got %d", code)
	}

	// Sync: Plex still returns the same re-encoded bytes — local copy must
	// already match, so zero changes should be reported.
	result := runSync(t, setup.handler)

	if result.Changed != 0 {
		t.Errorf("Changed: want 0 after push+resync, got %d (false positives from Plex re-encoding)", result.Changed)
	}
}
