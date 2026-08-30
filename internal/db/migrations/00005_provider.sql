-- +goose Up
-- Tag existing rows as Plex: they were all imported before Jellyfin support
-- existed. Only rows matching the configured provider are read back, so data
-- from a previous server is kept but never mixed in.
ALTER TABLE libraries        ADD COLUMN provider TEXT NOT NULL DEFAULT 'plex';
ALTER TABLE media            ADD COLUMN provider TEXT NOT NULL DEFAULT 'plex';
ALTER TABLE library_settings ADD COLUMN provider TEXT NOT NULL DEFAULT 'plex';

-- +goose Down
ALTER TABLE library_settings DROP COLUMN provider;
ALTER TABLE media            DROP COLUMN provider;
ALTER TABLE libraries        DROP COLUMN provider;
