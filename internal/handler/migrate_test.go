package handler_test

import (
	"bufio"
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/florentsorel/postr/internal/config"
	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/handler"
	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/florentsorel/postr/internal/posters"
	"github.com/labstack/echo/v5"
)

type migrateResult struct {
	Migrated  int
	ByID      int
	ByTitle   int
	Unchanged int
	Unmatched int
	Skipped   int
	Reports   []string // "<type>|<title>|<reason>" for unmatched and skipped items
}

func runMigrate(t *testing.T, h *handler.Handler) migrateResult {
	t.Helper()
	e := echo.New()
	req := httptest.NewRequest(http.MethodPost, "/api/server/migrate", nil)
	rec := httptest.NewRecorder()
	c := e.NewContext(req, rec)
	if err := h.MigratePosters(c); err != nil {
		t.Fatalf("MigratePosters: %v", err)
	}

	var result migrateResult
	scanner := bufio.NewScanner(bytes.NewReader(rec.Body.Bytes()))
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		var ev map[string]any
		if err := json.Unmarshal([]byte(strings.TrimPrefix(line, "data: ")), &ev); err != nil {
			continue
		}
		switch ev["type"] {
		case "error":
			t.Fatalf("migration reported an error: %v", ev["message"])
		case "done":
			result.Migrated = int(ev["migrated"].(float64))
			result.ByID = int(ev["byId"].(float64))
			result.ByTitle = int(ev["byTitle"].(float64))
			result.Unchanged = int(ev["unchanged"].(float64))
			result.Unmatched = int(ev["unmatched"].(float64))
			result.Skipped = int(ev["skipped"].(float64))
		case "unmatched", "skipped":
			result.Reports = append(result.Reports,
				ev["mediaType"].(string)+"|"+ev["title"].(string)+"|"+ev["reason"].(string))
		}
	}
	return result
}

// seedSourcePoster inserts a media row for the inactive provider and writes its
// poster to disk, mimicking a library imported before the switch.
func seedSourcePoster(t *testing.T, setup *testSetup, libraryID int64, ratingKey, title, mediaType string, year int64, content string) {
	t.Helper()
	ctx := context.Background()
	now := time.Now().Unix()
	err := setup.queries.UpsertMedia(ctx, db.UpsertMediaParams{
		Provider:  mediaserver.ProviderPlex,
		LibraryID: libraryID,
		RatingKey: ratingKey,
		Title:     title,
		Type:      mediaType,
		Year:      sql.NullInt64{Int64: year, Valid: year != 0},
		Thumb:     sql.NullString{String: "jpg", Valid: true},
		CreatedAt: now,
		UpdatedAt: now,
	})
	if err != nil {
		t.Fatalf("seed media: %v", err)
	}

	dir := posters.Dir(setup.dataPath, mediaserver.ProviderPlex, mediaType)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	path := posters.Path(setup.dataPath, mediaserver.ProviderPlex, mediaType, ratingKey, "jpg")
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write poster: %v", err)
	}
}

func seedSourceLibrary(t *testing.T, setup *testSetup) int64 {
	t.Helper()
	lib, err := setup.queries.UpsertLibrary(context.Background(), db.UpsertLibraryParams{
		Provider:   mediaserver.ProviderPlex,
		SectionKey: "1",
		Title:      "Movies",
		Type:       mediaserver.TypeMovie,
		ImportedAt: time.Now().Unix(),
	})
	if err != nil {
		t.Fatalf("seed library: %v", err)
	}
	return lib.ID
}

// jellyfinSetup builds a handler running on Jellyfin with Plex attached as the
// migration source, and imports the Jellyfin library so target rows exist.
func jellyfinSetup(t *testing.T, target, source *mockServer) *testSetup {
	t.Helper()
	cfg := &config.Config{
		MediaServer: mediaserver.ProviderJellyfin,
		JellyfinURL: "http://jellyfin:8096", JellyfinAPIKey: "key",
		PlexURL: "http://plex:32400", PlexToken: "tok",
	}
	setup := newTestSetupWithCfg(t, cfg, target)
	setup.handler.WithSource(source)
	return setup
}

func TestMigratePosters_CarriesArtworkAcrossServers(t *testing.T) {
	target := &mockServer{
		provider:          mediaserver.ProviderJellyfin,
		globalCollections: true,
		librariesFunc: func(ctx context.Context) ([]mediaserver.Library, error) {
			return []mediaserver.Library{{Key: "jf-movies", Type: mediaserver.TypeMovie, Title: "Movies"}}, nil
		},
		itemsFunc: func(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
			if mediaType != mediaserver.TypeMovie {
				return nil, nil
			}
			return []mediaserver.Item{
				// Jellyfin names it differently — only the TMDB id ties them together.
				{ID: "jf-abc", Title: "Inception", Year: 2010, HasPoster: true,
					ExternalIDs: map[string]string{"tmdb": "27205"}},
			}, nil
		},
	}
	source := &mockServer{
		provider: mediaserver.ProviderPlex,
		librariesFunc: func(ctx context.Context) ([]mediaserver.Library, error) {
			return []mediaserver.Library{{Key: "1", Type: mediaserver.TypeMovie, Title: "Movies"}}, nil
		},
		itemsFunc: func(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
			if mediaType != mediaserver.TypeMovie {
				return nil, nil
			}
			return []mediaserver.Item{
				{ID: "2519", Title: "Inception (2010)", Year: 2010,
					ExternalIDs: map[string]string{"tmdb": "27205"}},
			}, nil
		},
	}

	setup := jellyfinSetup(t, target, source)
	runImport(t, setup.handler, `{"targets":[{"type":"movie","sectionKeys":["jf-movies"]}]}`)

	libID := seedSourceLibrary(t, setup)
	seedSourcePoster(t, setup, libID, "2519", "Inception (2010)", mediaserver.TypeMovie, 2010, "my-custom-plex-poster")

	result := runMigrate(t, setup.handler)

	if result.Migrated != 1 || result.ByID != 1 {
		t.Fatalf("want 1 migration by external id, got %+v", result)
	}

	// The artwork now sits under the Jellyfin item's own id.
	got, err := os.ReadFile(posters.Path(setup.dataPath, mediaserver.ProviderJellyfin, mediaserver.TypeMovie, "jf-abc", "jpg"))
	if err != nil {
		t.Fatalf("migrated poster not found: %v", err)
	}
	if string(got) != "my-custom-plex-poster" {
		t.Errorf("content = %q, want the Plex poster", got)
	}

	// The Plex original is left untouched, so a rollback is always possible.
	if _, err := os.Stat(posters.Path(setup.dataPath, mediaserver.ProviderPlex, mediaserver.TypeMovie, "2519", "jpg")); err != nil {
		t.Errorf("source poster should survive the migration: %v", err)
	}

	// And it waits in the queue rather than reaching Jellyfin on its own.
	queued, err := setup.queries.ListPosterQueueWithMedia(context.Background(), mediaserver.ProviderJellyfin)
	if err != nil {
		t.Fatalf("ListPosterQueueWithMedia: %v", err)
	}
	if len(queued) != 1 || queued[0].RatingKey != "jf-abc" {
		t.Fatalf("queue: want jf-abc, got %+v", queued)
	}

	m, err := setup.queries.GetMediaByRatingKey(context.Background(), "jf-abc")
	if err != nil {
		t.Fatalf("GetMediaByRatingKey: %v", err)
	}
	if m.LocallyModified != 1 {
		t.Error("target item should be flagged as locally modified")
	}
}

// TestMigratePosters_NeverPushesToTheServer pins the review-first contract.
func TestMigratePosters_NeverPushesToTheServer(t *testing.T) {
	var uploads int
	target := &mockServer{
		provider: mediaserver.ProviderJellyfin,
		librariesFunc: func(ctx context.Context) ([]mediaserver.Library, error) {
			return []mediaserver.Library{{Key: "jf", Type: mediaserver.TypeMovie, Title: "Movies"}}, nil
		},
		itemsFunc: func(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
			if mediaType != mediaserver.TypeMovie {
				return nil, nil
			}
			return []mediaserver.Item{{ID: "jf-1", Title: "Inception", Year: 2010, HasPoster: true,
				ExternalIDs: map[string]string{"tmdb": "27205"}}}, nil
		},
		uploadFunc: func(ctx context.Context, itemID string, data []byte, contentType string) error {
			uploads++
			return nil
		},
	}
	source := &mockServer{
		provider: mediaserver.ProviderPlex,
		librariesFunc: func(ctx context.Context) ([]mediaserver.Library, error) {
			return []mediaserver.Library{{Key: "1", Type: mediaserver.TypeMovie, Title: "Movies"}}, nil
		},
		itemsFunc: func(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
			if mediaType != mediaserver.TypeMovie {
				return nil, nil
			}
			return []mediaserver.Item{{ID: "1", Title: "Inception", Year: 2010,
				ExternalIDs: map[string]string{"tmdb": "27205"}}}, nil
		},
	}

	setup := jellyfinSetup(t, target, source)
	runImport(t, setup.handler, `{"targets":[{"type":"movie","sectionKeys":["jf"]}]}`)
	libID := seedSourceLibrary(t, setup)
	seedSourcePoster(t, setup, libID, "1", "Inception", mediaserver.TypeMovie, 2010, "poster")

	if got := runMigrate(t, setup.handler); got.Migrated != 1 {
		t.Fatalf("want 1 migrated, got %+v", got)
	}
	if uploads != 0 {
		t.Errorf("migration uploaded %d posters; it must only fill the queue", uploads)
	}
}

func TestMigratePosters_ReportsItemsWithNoCounterpart(t *testing.T) {
	target := &mockServer{
		provider: mediaserver.ProviderJellyfin,
		librariesFunc: func(ctx context.Context) ([]mediaserver.Library, error) {
			return []mediaserver.Library{{Key: "jf", Type: mediaserver.TypeMovie, Title: "Movies"}}, nil
		},
		itemsFunc: func(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
			if mediaType != mediaserver.TypeMovie {
				return nil, nil
			}
			return []mediaserver.Item{{ID: "jf-1", Title: "Something Else", Year: 2001, HasPoster: true,
				ExternalIDs: map[string]string{"tmdb": "1"}}}, nil
		},
	}
	source := &mockServer{
		provider: mediaserver.ProviderPlex,
		librariesFunc: func(ctx context.Context) ([]mediaserver.Library, error) {
			return []mediaserver.Library{{Key: "1", Type: mediaserver.TypeMovie, Title: "Movies"}}, nil
		},
		itemsFunc: func(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
			return nil, nil
		},
	}

	setup := jellyfinSetup(t, target, source)
	runImport(t, setup.handler, `{"targets":[{"type":"movie","sectionKeys":["jf"]}]}`)
	libID := seedSourceLibrary(t, setup)
	seedSourcePoster(t, setup, libID, "999", "Gone From Jellyfin", mediaserver.TypeMovie, 1999, "poster")

	result := runMigrate(t, setup.handler)

	if result.Migrated != 0 {
		t.Errorf("migrated: want 0, got %d", result.Migrated)
	}
	if result.Unmatched != 1 {
		t.Fatalf("unmatched: want 1, got %d (%v)", result.Unmatched, result.Reports)
	}
	if len(result.Reports) != 1 || !strings.Contains(result.Reports[0], "Gone From Jellyfin") {
		t.Errorf("report should name the item: %v", result.Reports)
	}
}

func TestMigratePosters_RefusesWhenTargetLibraryNotImported(t *testing.T) {
	setup := jellyfinSetup(t,
		&mockServer{provider: mediaserver.ProviderJellyfin},
		&mockServer{provider: mediaserver.ProviderPlex},
	)
	libID := seedSourceLibrary(t, setup)
	seedSourcePoster(t, setup, libID, "1", "Inception", mediaserver.TypeMovie, 2010, "poster")

	e := echo.New()
	rec := httptest.NewRecorder()
	c := e.NewContext(httptest.NewRequest(http.MethodPost, "/api/server/migrate", nil), rec)
	if err := setup.handler.MigratePosters(c); err != nil {
		t.Fatalf("MigratePosters: %v", err)
	}

	if !strings.Contains(rec.Body.String(), "Import your Jellyfin library") {
		t.Errorf("expected a clear precondition error, got: %s", rec.Body.String())
	}
}

func TestGetMigrateStatus_UnavailableWithoutASecondServer(t *testing.T) {
	setup := newTestSetup(t, &mockServer{})
	rec, c := newCtx(t, http.MethodGet, "/api/server/migrate/status", "")
	if err := setup.handler.GetMigrateStatus(c); err != nil {
		t.Fatalf("GetMigrateStatus: %v", err)
	}

	resp := decodeJSON[struct {
		Available bool   `json:"available"`
		Reason    string `json:"reason"`
	}](t, rec.Body.Bytes())
	if resp.Available {
		t.Error("available: want false with no second server configured")
	}
	if resp.Reason == "" {
		t.Error("a reason should be given")
	}
}

// TestMigratePosters_SecondRunIsANoOp guards the cost of re-running: artwork
// already carried over must not be copied and queued again, or the user pushes
// bytes the server already holds.
func TestMigratePosters_SecondRunIsANoOp(t *testing.T) {
	target := &mockServer{
		provider: mediaserver.ProviderJellyfin,
		librariesFunc: func(ctx context.Context) ([]mediaserver.Library, error) {
			return []mediaserver.Library{{Key: "jf", Type: mediaserver.TypeMovie, Title: "Movies"}}, nil
		},
		itemsFunc: func(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
			if mediaType != mediaserver.TypeMovie {
				return nil, nil
			}
			return []mediaserver.Item{{ID: "jf-1", Title: "Inception", Year: 2010, HasPoster: true,
				ExternalIDs: map[string]string{"tmdb": "27205"}}}, nil
		},
	}
	source := &mockServer{
		provider: mediaserver.ProviderPlex,
		librariesFunc: func(ctx context.Context) ([]mediaserver.Library, error) {
			return []mediaserver.Library{{Key: "1", Type: mediaserver.TypeMovie, Title: "Movies"}}, nil
		},
		itemsFunc: func(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
			if mediaType != mediaserver.TypeMovie {
				return nil, nil
			}
			return []mediaserver.Item{{ID: "1", Title: "Inception", Year: 2010,
				ExternalIDs: map[string]string{"tmdb": "27205"}}}, nil
		},
	}

	setup := jellyfinSetup(t, target, source)
	runImport(t, setup.handler, `{"targets":[{"type":"movie","sectionKeys":["jf"]}]}`)
	libID := seedSourceLibrary(t, setup)
	seedSourcePoster(t, setup, libID, "1", "Inception", mediaserver.TypeMovie, 2010, "poster")

	if first := runMigrate(t, setup.handler); first.Migrated != 1 || first.Unchanged != 0 {
		t.Fatalf("first run: want 1 migrated / 0 unchanged, got %+v", first)
	}

	// Clear the queue the way pushing to the server would.
	ctx := context.Background()
	if err := setup.queries.DeletePosterQueueByRatingKey(ctx, "jf-1"); err != nil {
		t.Fatalf("clear queue: %v", err)
	}

	second := runMigrate(t, setup.handler)

	if second.Migrated != 0 {
		t.Errorf("second run: want 0 migrated, got %d", second.Migrated)
	}
	if second.Unchanged != 1 {
		t.Errorf("second run: want 1 unchanged, got %d", second.Unchanged)
	}

	queued, err := setup.queries.ListPosterQueueWithMedia(ctx, mediaserver.ProviderJellyfin)
	if err != nil {
		t.Fatalf("ListPosterQueueWithMedia: %v", err)
	}
	if len(queued) != 0 {
		t.Errorf("second run re-queued %d item(s); it should have left the queue empty", len(queued))
	}
}

// TestMigratePosters_ChangedSourcePosterIsCarriedAgain is the other half of the
// contract: idempotence must not mean "never update".
func TestMigratePosters_ChangedSourcePosterIsCarriedAgain(t *testing.T) {
	target := &mockServer{
		provider: mediaserver.ProviderJellyfin,
		librariesFunc: func(ctx context.Context) ([]mediaserver.Library, error) {
			return []mediaserver.Library{{Key: "jf", Type: mediaserver.TypeMovie, Title: "Movies"}}, nil
		},
		itemsFunc: func(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
			if mediaType != mediaserver.TypeMovie {
				return nil, nil
			}
			return []mediaserver.Item{{ID: "jf-1", Title: "Inception", Year: 2010, HasPoster: true,
				ExternalIDs: map[string]string{"tmdb": "27205"}}}, nil
		},
	}
	source := &mockServer{
		provider: mediaserver.ProviderPlex,
		librariesFunc: func(ctx context.Context) ([]mediaserver.Library, error) {
			return []mediaserver.Library{{Key: "1", Type: mediaserver.TypeMovie, Title: "Movies"}}, nil
		},
		itemsFunc: func(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
			if mediaType != mediaserver.TypeMovie {
				return nil, nil
			}
			return []mediaserver.Item{{ID: "1", Title: "Inception", Year: 2010,
				ExternalIDs: map[string]string{"tmdb": "27205"}}}, nil
		},
	}

	setup := jellyfinSetup(t, target, source)
	runImport(t, setup.handler, `{"targets":[{"type":"movie","sectionKeys":["jf"]}]}`)
	libID := seedSourceLibrary(t, setup)
	seedSourcePoster(t, setup, libID, "1", "Inception", mediaserver.TypeMovie, 2010, "poster")
	runMigrate(t, setup.handler)

	ctx := context.Background()
	if err := setup.queries.DeletePosterQueueByRatingKey(ctx, "jf-1"); err != nil {
		t.Fatalf("clear queue: %v", err)
	}
	// The user picks a new poster on the source side.
	seedSourcePoster(t, setup, libID, "1", "Inception", mediaserver.TypeMovie, 2010, "a-different-poster")

	second := runMigrate(t, setup.handler)

	if second.Migrated != 1 || second.Unchanged != 0 {
		t.Fatalf("want the new poster carried over, got %+v", second)
	}
	got, err := os.ReadFile(posters.Path(setup.dataPath, mediaserver.ProviderJellyfin, mediaserver.TypeMovie, "jf-1", "jpg"))
	if err != nil {
		t.Fatalf("read migrated poster: %v", err)
	}
	if string(got) != "a-different-poster" {
		t.Errorf("content = %q, want the updated poster", got)
	}
}
