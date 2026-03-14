-- +goose Up
CREATE TABLE settings (
    id       INTEGER PRIMARY KEY AUTOINCREMENT,
    type     TEXT    NOT NULL,
    key      TEXT    NOT NULL,
    value    TEXT,
    position INTEGER,
    UNIQUE (type, key)
);

INSERT INTO settings (type, key, value, position) VALUES
    ('poster_source', 'tmdb',   'true', 1),
    ('poster_source', 'tvdb',   'true', 2),
    ('poster_source', 'fanart', 'true', 3),
    ('option',        'auto_resize', 'true', NULL);

-- +goose Down
DROP TABLE settings;
