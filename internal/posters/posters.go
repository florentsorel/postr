// Package posters owns the on-disk layout of locally stored poster images.
//
// Posters live at {DATA_PATH}/posters/{provider}/{type}/{itemID}.{ext}. The
// provider segment keeps artwork imported from one media server from ever
// colliding with, or being mistaken for, another's — the on-disk counterpart of
// the `provider` column in the database.
package posters

import (
	"errors"
	"os"
	"path/filepath"
)

// root is the directory under DATA_PATH holding every poster.
const root = "posters"

// mediaTypes are the per-type directories Postr writes into. They double as the
// set of directories the legacy layout could contain.
var mediaTypes = []string{"movie", "show", "season", "collection"}

// Dir returns the directory holding the posters of one media type for a provider.
func Dir(dataPath, provider, mediaType string) string {
	return filepath.Join(dataPath, root, provider, mediaType)
}

// Path returns the full path of a single poster file.
func Path(dataPath, provider, mediaType, itemID, ext string) string {
	return filepath.Join(Dir(dataPath, provider, mediaType), itemID+"."+ext)
}

// MigrateLegacyLayout moves posters written by versions that predate multi-server
// support — which stored them at posters/{type}/ with no provider segment — into
// posters/{provider}/{type}/.
//
// Callers must pass the provider those files belong to, which is always "plex":
// the legacy layout only ever existed while Plex was the sole supported server,
// matching how database migration 00005 backfills the provider column.
//
// It is idempotent and safe to run on every startup: once the legacy directories
// are gone it does nothing. A file whose destination already exists is left in
// place rather than overwriting the newer copy, and reported in the error.
func MigrateLegacyLayout(dataPath, provider string) (moved int, err error) {
	var skipped []string

	for _, mediaType := range mediaTypes {
		legacyDir := filepath.Join(dataPath, root, mediaType)
		entries, readErr := os.ReadDir(legacyDir)
		if readErr != nil {
			if errors.Is(readErr, os.ErrNotExist) {
				continue // already migrated, or this type was never imported
			}
			return moved, readErr
		}

		destDir := Dir(dataPath, provider, mediaType)
		if err := os.MkdirAll(destDir, 0o755); err != nil {
			return moved, err
		}

		for _, entry := range entries {
			if entry.IsDir() {
				continue
			}
			dest := filepath.Join(destDir, entry.Name())
			if _, statErr := os.Stat(dest); statErr == nil {
				skipped = append(skipped, entry.Name())
				continue
			}
			if err := os.Rename(filepath.Join(legacyDir, entry.Name()), dest); err != nil {
				return moved, err
			}
			moved++
		}

		// Only succeeds once the directory is empty, which is what we want:
		// anything left behind is a signal worth preserving.
		_ = os.Remove(legacyDir)
	}

	if len(skipped) > 0 {
		return moved, &SkippedError{Files: skipped}
	}
	return moved, nil
}

// SkippedError reports legacy posters left in place because a file already
// existed at their destination. The migration itself succeeded for every other
// file; this is a warning, not a failure.
type SkippedError struct {
	Files []string
}

func (e *SkippedError) Error() string {
	return "some legacy posters were left in place because the destination already exists"
}
