-- name: UpsertLibrary :one
INSERT INTO libraries (section_key, title, type, imported_at)
VALUES (?, ?, ?, ?)
ON CONFLICT (section_key) DO UPDATE SET
    title       = excluded.title,
    type        = excluded.type,
    imported_at = excluded.imported_at
RETURNING *;

-- name: UpsertMedia :exec
INSERT INTO media (library_id, rating_key, title, type, year, season_number, thumb, added_at, created_at, updated_at)
VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
ON CONFLICT (rating_key) DO UPDATE SET
    title         = excluded.title,
    type          = excluded.type,
    year          = excluded.year,
    season_number = excluded.season_number,
    thumb         = excluded.thumb,
    added_at      = excluded.added_at,
    updated_at    = excluded.updated_at;

-- name: ListMedia :many
SELECT id, library_id, rating_key, title, type, year, season_number, thumb, added_at, created_at
FROM media
ORDER BY added_at DESC NULLS LAST;

-- name: GetMediaByRatingKey :one
SELECT id, library_id, rating_key, title, type, year, season_number, thumb, added_at, created_at, updated_at
FROM media
WHERE rating_key = ?;

-- name: ListRatingKeysByLibraryIDAndType :many
SELECT rating_key FROM media WHERE library_id = ? AND type = ?;

-- name: DeleteMediaByRatingKey :exec
DELETE FROM media WHERE rating_key = ?;

-- name: DeleteMediaByLibrary :exec
DELETE FROM media WHERE library_id = ?;
