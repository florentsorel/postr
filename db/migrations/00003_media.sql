-- +goose Up
CREATE TABLE libraries (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    section_key TEXT    NOT NULL UNIQUE,
    title       TEXT    NOT NULL,
    type        TEXT    NOT NULL,
    imported_at INTEGER NOT NULL
);

CREATE TABLE media (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    library_id    INTEGER NOT NULL REFERENCES libraries(id) ON DELETE CASCADE,
    rating_key    TEXT    NOT NULL UNIQUE,
    title         TEXT    NOT NULL,
    type          TEXT    NOT NULL,
    season_number INTEGER,
    year          INTEGER,
    thumb         TEXT,
    added_at      INTEGER,
    created_at    INTEGER NOT NULL,
    updated_at    INTEGER NOT NULL
);

-- +goose Down
DROP TABLE media;
DROP TABLE libraries;
