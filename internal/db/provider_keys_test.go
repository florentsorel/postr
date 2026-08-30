package db_test

import (
	"context"
	"testing"

	"github.com/florentsorel/postr/internal/db"
)

// TestLibraryKeysAreScopedByProvider proves the isolation the provider column
// exists for. Before migration 00006 the keys were global, so two servers using
// the same section key would overwrite each other's row and flip its provider.
func TestLibraryKeysAreScopedByProvider(t *testing.T) {
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("db.Open: %v", err)
	}
	defer conn.Close()

	q := db.New(conn)
	ctx := context.Background()

	// The same key on both servers — contrived for Plex vs Jellyfin, but the
	// schema must not depend on their id formats never colliding.
	const sharedKey = "1"

	plexLib, err := q.UpsertLibrary(ctx, db.UpsertLibraryParams{
		Provider: "plex", SectionKey: sharedKey, Title: "Plex Movies", Type: "movie", ImportedAt: 1,
	})
	if err != nil {
		t.Fatalf("upsert plex library: %v", err)
	}
	jellyLib, err := q.UpsertLibrary(ctx, db.UpsertLibraryParams{
		Provider: "jellyfin", SectionKey: sharedKey, Title: "Jellyfin Movies", Type: "movie", ImportedAt: 2,
	})
	if err != nil {
		t.Fatalf("upsert jellyfin library: %v", err)
	}

	if plexLib.ID == jellyLib.ID {
		t.Fatal("the two servers' libraries collapsed into one row")
	}
	if plexLib.Provider != "plex" || jellyLib.Provider != "jellyfin" {
		t.Errorf("providers got mixed up: %q and %q", plexLib.Provider, jellyLib.Provider)
	}
	if plexLib.Title != "Plex Movies" {
		t.Errorf("the Plex row was overwritten: title = %q", plexLib.Title)
	}

	// Re-upserting one server's library must update only that row.
	if _, err := q.UpsertLibrary(ctx, db.UpsertLibraryParams{
		Provider: "jellyfin", SectionKey: sharedKey, Title: "Renamed", Type: "movie", ImportedAt: 3,
	}); err != nil {
		t.Fatalf("re-upsert: %v", err)
	}

	// Settings are keyed the same way.
	for _, provider := range []string{"plex", "jellyfin"} {
		if err := q.UpsertLibrarySetting(ctx, db.UpsertLibrarySettingParams{
			Provider: provider, SectionKey: sharedKey, Enabled: map[string]int64{"plex": 1, "jellyfin": 0}[provider],
		}); err != nil {
			t.Fatalf("upsert %s setting: %v", provider, err)
		}
	}

	plexSettings, err := q.ListLibrarySettings(ctx, "plex")
	if err != nil {
		t.Fatalf("list plex settings: %v", err)
	}
	jellySettings, err := q.ListLibrarySettings(ctx, "jellyfin")
	if err != nil {
		t.Fatalf("list jellyfin settings: %v", err)
	}
	if len(plexSettings) != 1 || len(jellySettings) != 1 {
		t.Fatalf("each provider should keep its own setting, got %d and %d", len(plexSettings), len(jellySettings))
	}
	if plexSettings[0].Enabled != 1 || jellySettings[0].Enabled != 0 {
		t.Errorf("settings crossed over: plex=%d jellyfin=%d", plexSettings[0].Enabled, jellySettings[0].Enabled)
	}
}
