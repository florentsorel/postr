package posters_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/florentsorel/postr/internal/posters"
)

// writeLegacy creates a poster in the pre-provider layout: posters/{type}/{name}.
func writeLegacy(t *testing.T, dataPath, mediaType, name, content string) {
	t.Helper()
	dir := filepath.Join(dataPath, "posters", mediaType)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(b)
}

func TestPath(t *testing.T) {
	got := posters.Path("/data", "jellyfin", "movie", "abc123", "jpg")
	want := filepath.Join("/data", "posters", "jellyfin", "movie", "abc123.jpg")
	if got != want {
		t.Errorf("Path = %q, want %q", got, want)
	}
}

func TestMigrateLegacyLayout_MovesEveryTypeAndClearsLegacyDirs(t *testing.T) {
	dataPath := t.TempDir()
	writeLegacy(t, dataPath, "movie", "2519.jpg", "movie-poster")
	writeLegacy(t, dataPath, "show", "77.png", "show-poster")
	writeLegacy(t, dataPath, "season", "201.jpg", "season-poster")
	writeLegacy(t, dataPath, "collection", "9.webp", "collection-poster")

	moved, err := posters.MigrateLegacyLayout(dataPath, "plex")
	if err != nil {
		t.Fatalf("MigrateLegacyLayout: %v", err)
	}
	if moved != 4 {
		t.Errorf("moved: want 4, got %d", moved)
	}

	cases := []struct{ mediaType, name, content string }{
		{"movie", "2519.jpg", "movie-poster"},
		{"show", "77.png", "show-poster"},
		{"season", "201.jpg", "season-poster"},
		{"collection", "9.webp", "collection-poster"},
	}
	for _, c := range cases {
		dest := filepath.Join(posters.Dir(dataPath, "plex", c.mediaType), c.name)
		if got := readFile(t, dest); got != c.content {
			t.Errorf("%s: content = %q, want %q", dest, got, c.content)
		}
		// The legacy directory must be gone, not merely emptied.
		if _, err := os.Stat(filepath.Join(dataPath, "posters", c.mediaType)); !errors.Is(err, os.ErrNotExist) {
			t.Errorf("legacy dir posters/%s still exists", c.mediaType)
		}
	}
}

func TestMigrateLegacyLayout_IsIdempotent(t *testing.T) {
	dataPath := t.TempDir()
	writeLegacy(t, dataPath, "movie", "2519.jpg", "poster")

	if _, err := posters.MigrateLegacyLayout(dataPath, "plex"); err != nil {
		t.Fatalf("first run: %v", err)
	}

	// Every subsequent startup must be a no-op.
	for i := range 3 {
		moved, err := posters.MigrateLegacyLayout(dataPath, "plex")
		if err != nil {
			t.Fatalf("run %d: %v", i+2, err)
		}
		if moved != 0 {
			t.Errorf("run %d: moved = %d, want 0", i+2, moved)
		}
	}

	if got := readFile(t, posters.Path(dataPath, "plex", "movie", "2519", "jpg")); got != "poster" {
		t.Errorf("content = %q, want %q", got, "poster")
	}
}

func TestMigrateLegacyLayout_EmptyDataDirIsANoOp(t *testing.T) {
	moved, err := posters.MigrateLegacyLayout(t.TempDir(), "plex")
	if err != nil {
		t.Fatalf("MigrateLegacyLayout: %v", err)
	}
	if moved != 0 {
		t.Errorf("moved: want 0, got %d", moved)
	}
}

// TestMigrateLegacyLayout_NeverOverwritesDestination guards the one case where
// data could be lost: a half-finished migration where the new layout already
// holds a file of the same name.
func TestMigrateLegacyLayout_NeverOverwritesDestination(t *testing.T) {
	dataPath := t.TempDir()
	writeLegacy(t, dataPath, "movie", "2519.jpg", "legacy-version")
	writeLegacy(t, dataPath, "movie", "2520.jpg", "moves-fine")

	destDir := posters.Dir(dataPath, "plex", "movie")
	if err := os.MkdirAll(destDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(destDir, "2519.jpg"), []byte("newer-version"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	moved, err := posters.MigrateLegacyLayout(dataPath, "plex")

	var skipped *posters.SkippedError
	if !errors.As(err, &skipped) {
		t.Fatalf("want SkippedError, got %v", err)
	}
	if len(skipped.Files) != 1 || skipped.Files[0] != "2519.jpg" {
		t.Errorf("skipped files: want [2519.jpg], got %v", skipped.Files)
	}
	if moved != 1 {
		t.Errorf("moved: want 1 (the unblocked file), got %d", moved)
	}

	// The newer copy survives untouched...
	if got := readFile(t, filepath.Join(destDir, "2519.jpg")); got != "newer-version" {
		t.Errorf("destination was overwritten: %q", got)
	}
	// ...and the legacy file is left behind rather than silently dropped.
	if got := readFile(t, filepath.Join(dataPath, "posters", "movie", "2519.jpg")); got != "legacy-version" {
		t.Errorf("legacy file lost: %q", got)
	}
	if got := readFile(t, filepath.Join(destDir, "2520.jpg")); got != "moves-fine" {
		t.Errorf("unblocked file: %q", got)
	}
}

// TestMigrateLegacyLayout_IgnoresProviderDirs makes sure the already-migrated
// layout is never mistaken for legacy content and re-nested.
func TestMigrateLegacyLayout_IgnoresProviderDirs(t *testing.T) {
	dataPath := t.TempDir()
	dir := posters.Dir(dataPath, "jellyfin", "movie")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(dir, "abc.jpg"), []byte("jellyfin"), 0o644); err != nil {
		t.Fatalf("write: %v", err)
	}

	moved, err := posters.MigrateLegacyLayout(dataPath, "plex")
	if err != nil {
		t.Fatalf("MigrateLegacyLayout: %v", err)
	}
	if moved != 0 {
		t.Errorf("moved: want 0, got %d", moved)
	}
	if got := readFile(t, filepath.Join(dir, "abc.jpg")); got != "jellyfin" {
		t.Errorf("jellyfin poster disturbed: %q", got)
	}
	if _, err := os.Stat(posters.Dir(dataPath, "plex", "jellyfin")); !errors.Is(err, os.ErrNotExist) {
		t.Error("provider dir was re-nested under plex/")
	}
}
