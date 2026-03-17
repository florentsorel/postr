-- +goose Up
CREATE TABLE library_settings (
    section_key TEXT    PRIMARY KEY,
    enabled     INTEGER NOT NULL DEFAULT 1
);

-- +goose Down
DROP TABLE library_settings;
