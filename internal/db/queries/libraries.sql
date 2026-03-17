-- name: ListLibrarySettings :many
SELECT section_key, enabled FROM library_settings;

-- name: UpsertLibrarySetting :exec
INSERT INTO library_settings (section_key, enabled)
VALUES (?, ?)
ON CONFLICT (section_key) DO UPDATE SET enabled = excluded.enabled;
