-- name: UpsertPosterQueue :exec
INSERT INTO poster_queue (media_id, created_at)
VALUES (?, ?)
ON CONFLICT (media_id) DO UPDATE SET created_at = excluded.created_at;

-- name: DeletePosterQueueByRatingKey :exec
DELETE FROM poster_queue
WHERE media_id = (SELECT id FROM media WHERE rating_key = ?);

-- name: ListPosterQueue :many
SELECT pq.id, m.rating_key, m.title, m.type, m.thumb
FROM poster_queue pq
JOIN media m ON m.id = pq.media_id
ORDER BY pq.created_at DESC;

-- name: CountPosterQueue :one
SELECT COUNT(*) FROM poster_queue;

-- name: ListPosterQueueWithMedia :many
SELECT pq.id, m.id AS media_id, m.rating_key, m.title, m.type, m.season_number, m.thumb
FROM poster_queue pq
JOIN media m ON m.id = pq.media_id
ORDER BY pq.created_at DESC;
