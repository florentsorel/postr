-- +goose Up
INSERT INTO settings (type, key, value, position) VALUES ('option', 'resize_width', '1000', NULL);

CREATE TABLE poster_queue (
    id         INTEGER PRIMARY KEY AUTOINCREMENT,
    media_id   INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    created_at INTEGER NOT NULL,
    UNIQUE (media_id)
);

-- +goose Down
DELETE FROM settings WHERE type = 'option' AND key = 'resize_width';
DROP TABLE poster_queue;
