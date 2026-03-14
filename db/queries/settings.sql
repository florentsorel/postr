-- name: ListSettings :many
SELECT * FROM settings ORDER BY type, position, key;

-- name: ListSettingsByType :many
SELECT * FROM settings WHERE type = ? ORDER BY position, key;

-- name: GetSetting :one
SELECT * FROM settings WHERE type = ? AND key = ?;

-- name: UpdateSetting :exec
UPDATE settings SET value = ? WHERE type = ? AND key = ?;

-- name: UpdatePosterSourcePosition :exec
UPDATE settings SET position = ? WHERE type = 'poster_source' AND key = ?;
