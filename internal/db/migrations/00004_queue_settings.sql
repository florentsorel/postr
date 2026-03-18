-- +goose Up
CREATE TABLE poster_queue (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id   INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    UNIQUE (media_id)
);

-- +goose Down
DROP TABLE poster_queue;
