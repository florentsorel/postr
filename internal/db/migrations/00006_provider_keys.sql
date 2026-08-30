-- +goose Up
-- Migration 00005 added the provider column but left the keys global, so a
-- section_key shared by two servers would overwrite the other's row and flip
-- its provider — defeating the isolation the column exists for. Rebuild both
-- tables with the provider in the key.
--
-- Safe to drop and recreate: foreign_keys is not enabled on this connection,
-- and libraries.id values are carried over verbatim so media.library_id keeps
-- pointing at the right rows.

CREATE TABLE library_settings_new (
    provider    TEXT    NOT NULL DEFAULT 'plex',
    section_key TEXT    NOT NULL,
    enabled     INTEGER NOT NULL DEFAULT 1,
    PRIMARY KEY (provider, section_key)
);
INSERT INTO library_settings_new (provider, section_key, enabled)
    SELECT provider, section_key, enabled FROM library_settings;
DROP TABLE library_settings;
ALTER TABLE library_settings_new RENAME TO library_settings;

CREATE TABLE libraries_new (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    provider    TEXT    NOT NULL DEFAULT 'plex',
    section_key TEXT    NOT NULL,
    title       TEXT    NOT NULL,
    type        TEXT    NOT NULL,
    imported_at INTEGER NOT NULL,
    UNIQUE (provider, section_key)
);
INSERT INTO libraries_new (id, provider, section_key, title, type, imported_at)
    SELECT id, provider, section_key, title, type, imported_at FROM libraries;
DROP TABLE libraries;
ALTER TABLE libraries_new RENAME TO libraries;

-- +goose Down
CREATE TABLE library_settings_old (
    section_key TEXT    PRIMARY KEY,
    enabled     INTEGER NOT NULL DEFAULT 1,
    provider    TEXT    NOT NULL DEFAULT 'plex'
);
INSERT OR IGNORE INTO library_settings_old (section_key, enabled, provider)
    SELECT section_key, enabled, provider FROM library_settings;
DROP TABLE library_settings;
ALTER TABLE library_settings_old RENAME TO library_settings;

CREATE TABLE libraries_old (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    section_key TEXT    NOT NULL UNIQUE,
    title       TEXT    NOT NULL,
    type        TEXT    NOT NULL,
    imported_at INTEGER NOT NULL,
    provider    TEXT    NOT NULL DEFAULT 'plex'
);
INSERT OR IGNORE INTO libraries_old (id, section_key, title, type, imported_at, provider)
    SELECT id, section_key, title, type, imported_at, provider FROM libraries;
DROP TABLE libraries;
ALTER TABLE libraries_old RENAME TO libraries;
