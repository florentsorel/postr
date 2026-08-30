-- name: ListLibrarySettings :many
SELECT section_key, enabled FROM library_settings WHERE provider = ?;

-- name: UpsertLibrarySetting :exec
INSERT INTO library_settings (provider, section_key, enabled)
VALUES (?, ?, ?)
ON CONFLICT (provider, section_key) DO UPDATE SET
    enabled = excluded.enabled;
