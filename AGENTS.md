# Postr

A self-hosted web application for managing and updating poster artwork in a Plex or Jellyfin media server. Inspired by [Posteria](https://github.com/jeremehancock/Posteria), designed for homelab deployment via Docker.

---

## Purpose

Postr allows users to browse their media library and replace poster images for movies, TV series, seasons, and collections — either by uploading local files or fetching artwork from direct image URLs.

---

## Tech Stack

| Layer      | Technology                                         |
| ---------- | -------------------------------------------------- |
| Backend    | Go with Echo v5                                    |
| Frontend   | Vue 3 + Vite, TypeScript 5.9+                      |
| UI Library | Nuxt UI v4 (standalone, includes Tailwind CSS v4)  |
| Routing    | Vue Router                                         |
| State      | Pinia                                              |
| Database   | SQLite                                             |
| Deploy     | Docker image                                       |

---

## References

- Nuxt UI LLM docs: https://ui.nuxt.com/llms.txt

---

## Media Servers

Postr supports **Plex** and **Jellyfin**, one at a time. The active server is chosen at startup from `MEDIA_SERVER` (`plex` | `jellyfin`); when unset it is inferred — `jellyfin` if only `JELLYFIN_URL` is set, `plex` otherwise.

- `internal/mediaserver` declares the provider-neutral contract: `Client`, `Library`, `Item`, and the `ErrUnauthorized` / `ErrNotFound` sentinels. Handlers only ever see these types.
- `internal/mediaserver/plex` and `internal/mediaserver/jellyfin` implement `Client`. Every provider-specific detail lives there — URL shapes, auth headers, and the fact that Jellyfin expects **base64-encoded** image upload bodies while Plex takes raw bytes.
- The active client is built once in `main.go` and injected into the handler; it is `nil` when the server is not configured.

Provider differences the abstraction absorbs:

| | Plex | Jellyfin |
|---|---|---|
| Auth | `X-Plex-Token` header | `Authorization: MediaBrowser Token="…"` (+ `X-Emby-Token`) |
| Libraries | `/library/sections` | `/Library/VirtualFolders` |
| Items | `/library/sections/{key}/all` | `/Items?ParentId=…&IncludeItemTypes=…&Recursive=true` |
| Seasons | walk shows → children, year from the first episode | `IncludeItemTypes=Season`, year from `ProductionYear`/`PremiereDate` |
| Collections | per section, `/library/sections/{key}/collections` | server-wide `BoxSet` folder (`GlobalCollections() == true`) |
| Poster GET | `/library/metadata/{id}/thumb` | `/Items/{id}/Images/Primary` |
| Poster POST | raw bytes | base64-encoded body |

Because Jellyfin keeps every collection in one top-level folder, `GlobalCollections()` tells the import to anchor collections to that folder once instead of importing them per selected movie library.

### Provider isolation on disk

`internal/posters` owns the layout: `{DATA_PATH}/posters/{provider}/{type}/{itemID}.{ext}`. Every path in the handlers goes through `posterDir` / `posterPath`, never `filepath.Join` on its own.

Postr predates multi-server support and used to write `posters/{type}/` with no provider segment. `posters.MigrateLegacyLayout` relocates those files into `posters/plex/` at startup — Plex being the only server that layout could have come from, the same assumption migration `00005` makes. It is idempotent, and it refuses to overwrite an existing destination file, reporting a `SkippedError` instead.

### Provider isolation in the database

`libraries`, `media` and `library_settings` all carry a `provider` column (default `'plex'`, backfilled by migration `00005`). Reads are scoped to the active provider, so switching `MEDIA_SERVER` hides the other server's data instead of deleting it. `rating_key` stays globally unique — Plex ratingKeys and Jellyfin GUIDs never collide in practice, and keeping the constraint simple avoids threading the provider through every lookup.

---

## Poster Migration

When both servers are configured, posters imported from the inactive one can be carried over to the active one. `main.go` builds a second `mediaserver.Client` for the inactive provider and hands it to the handler via `WithSource`; nothing else in the app reads from it.

- `internal/migrate` holds the pairing logic as a pure function — no network, no filesystem — so every failure mode is reproducible in a test.
- Items are matched on the TMDB / IMDB / TVDB identifiers both servers expose (`mediaserver.Item.ExternalIDs`, filled by `includeGuids=1` on Plex and `Fields=ProviderIds` on Jellyfin). Seasons carry their **series'** identifiers plus a season number, because neither server assigns external IDs to a season. Collections carry none and can only be matched by title.
- Titles are the fallback. `NormalizeTitle` strips case, punctuation and a trailing `(2021)`, which is how a Plex `Invincible (2021)` meets a Jellyfin `Invincible`.
- **A pairing that is not unique on either side is refused, never guessed.** Writing the wrong artwork over a title is worse than leaving it to the user, so ambiguous candidates are reported instead.
- The universe of items comes from the database (that is what says which posters are actually on disk); the servers are only queried for identifiers.
- Matched posters are copied to the target's directory and **queued** — the migration never uploads. The source posters are left in place, so a migration can be repeated or abandoned freely.
- The copy skips a destination that already holds identical bytes, so re-running is a quiet no-op instead of re-queueing everything. This holds only while the server stores uploads verbatim; one that re-encodes them will look changed on the next run.

### Collections are the weak spot

Measured on a real 517-item Plex library migrated to Jellyfin: movies 93%, shows 90%, seasons 92%, **collections 0%**.

Plex exposes no external identifier for a collection, and Jellyfin's are commonly auto-created from TMDB under localized names (`Sing` on one side, `Tous en scène - Saga` on the other). With neither a shared id nor a comparable title, every collection is reported unmatched. That is the designed refusal-to-guess working, but the outcome is useless, so the confirm screen warns about it up front rather than leaving the user to discover it in the report.

Matching collections by their **members** — translating a collection's movies through the movie matches already established — would fix this. It is not implemented.

---

## Core Features (V1)

### 1. Library Import

- A button triggers a sync with the connected media server.
- The user can choose which media types to import:
  - Movies
  - TV Series
  - Season posters
  - Collections
- Imported media metadata is stored locally in SQLite (title, type, year, `added_at` timestamp from the server), tagged with the active `provider`.
- The import streams real-time progress via SSE (`text/event-stream`). The frontend reads the stream and displays a progress bar + final recap.
- Import stats: **Added** (new items), **Skipped** (existing items whose poster is byte-identical — DB is not touched), **Deleted** (items removed from the server). Thumbnail download failures appear in a separate errors accordion.
- During import, the current server-side poster for each item is **downloaded and stored locally** at `/data/posters/{provider}/{type}/{itemID}.jpg`. The filename is the server's own item ID (Plex `ratingKey` or Jellyfin GUID). Thumbnails are never served directly from server URLs (which require auth) — they are served by the Go backend at `/api/media/{ratingKey}/thumb`.
- Smart comparison: for existing items, the poster is downloaded and compared byte-for-byte before any DB write. If identical, the item is counted as skipped and the DB upsert is skipped entirely.
- Collections on Jellyfin are imported once from the server-wide box set folder, not once per selected movie library.

### 2. Sync from the Media Server

- Checks whether posters have been updated directly on the server since the last import.
- Only checks items that have **not** been locally modified (`locally_modified = 0`) and are **not orphans** (`is_orphan = 0`).
- Compares each local poster byte-for-byte with the current server-side poster. Updates any that have changed.
- Streams real-time progress via SSE. Displays a progress bar while checking, then shows a recap on completion:
  - **Updated** items listed with a badge.
  - **Failed** items listed separately with the reason (e.g. "No longer exists in Jellyfin").
- Items that return 404 from the server are automatically **marked as orphans** (see Orphaned Items below).
- "All posters are up to date" message only shown when there are zero changes and zero failures.
- Server connectivity is checked (ping) on modal open — sync button disabled if unreachable or the credential is invalid.
- Does not add or remove items — only updates existing posters.
- Button only visible when at least one item has been imported.

### 3. Poster Management

Each media card exposes actions on hover:

**a) Change Poster**
- Opens a modal with two tabs:
  - **Upload** — user uploads an image file directly (drag & drop or browse). Auto-resize to poster-friendly dimensions (configurable).
  - **From URL** — user pastes a direct image URL (JPG, PNG, WEBP). The server fetches the image server-side to avoid CORS issues.
- Once confirmed, the new poster is saved locally and queued for push to the server.

**b) Send to {server}**
- Pushes the locally modified poster directly to the server. The button label uses the active server's name.
- Only visible on cards that have a pending change (item is in the queue).
- Pings the server first: config errors (bad URL/credential) show a toast naming the exact env var to fix and keep the item in the queue. On a 404, the item is marked as orphan.

**c) Get from {server}**
- Re-downloads the poster currently set on the server and overwrites the local copy.
- Only visible on cards where the local poster has been locally modified (differs from the server).
- Pings the server first: config errors return an error toast and keep the item in the queue. On a 404, the item is marked as orphan.

### 4. Queue

- Lists all posters modified locally that are pending push to the server.
- Push one at a time or all at once with "Push all to {server}".
- Removing an item from the queue restores the original server-side poster (pings first — config errors keep the item in the queue).
- Button only visible when there are pending items.

### 5. Orphaned Items

- An item becomes an **orphan** (`is_orphan = 1`) when it is no longer found on the server (HTTP 404) during: import, sync, Send to server, or Get from server.
- Orphans are **not** created for connectivity/token errors — only confirmed 404s.
- Orphaned items appear in a dedicated **Orphaned** tab (only visible when at least one orphan exists).
- The tab auto-disappears and the view switches back to "All" when the last orphan is deleted.
- A toast is shown immediately when an item becomes orphan after a user action.
- Orphaned items can be permanently deleted via a trash icon on the card.
- On re-import, if an orphaned item reappears on the server (same `ratingKey`), `is_orphan` is reset to `0` automatically by the upsert.
- Note: Plex assigns new `ratingKey`s when an item is deleted and re-added — the old orphan record will remain until manually deleted.

### 6. Settings

Two categories of settings:

**Editable (stored in SQLite):**
- Option to enable/disable automatic image resizing on upload, and target width
- Per-library enable/disable toggle (which server libraries are included in imports), scoped to the active provider

**Read-only (from environment variables, displayed in UI but not editable):**
- Active server URL and credential — set via `PLEX_URL` / `PLEX_TOKEN` or `JELLYFIN_URL` / `JELLYFIN_API_KEY` (the credential is shown as set/not set only, never exposed). The section title, the credential label ("Token" vs "API key") and the env var names in the hints all follow the active provider.
- Auth status, username — set via `AUTH_ENABLED` / `AUTH_USER` / `AUTH_PASS`

The backend exposes `GET /api/settings` which returns both env-based config (read-only) and DB-stored settings. Only DB-stored settings are accepted on `POST /api/settings`.

`PLEX_URL` and `JELLYFIN_URL` are normalized at startup: scheme defaults to `http://` if omitted, trailing slashes and paths are stripped. Invalid schemes (non http/https) cause a startup error, as does an unknown `MEDIA_SERVER` value.

### 7. Authentication (Optional)

- A login form protects the app for users who expose it to the public internet.
- All auth credentials are configured exclusively via environment variables — no database storage.
- Authentication can be disabled for purely local/homelab use by setting `AUTH_ENABLED=false`.

---

## UI / UX

- Media library is displayed in a **responsive grid layout** after import (2→3→4→5→6 columns).
- Each card shows the locally stored poster thumbnail, title, type badge, and year.
- On hover: **Change Poster**, **Send to {server}** (if queued), and **Get from {server}** (if locally modified) action buttons appear.
- Tabs filter by type: All / Movies / TV Series / Seasons / Collections / Orphaned (conditional).
- Sort options: Title (A–Z), Year, Recently Added (`addedAt` from the server, stored in SQLite). Sort is hidden on the Orphaned tab.
- Search bar filters by title in real time across **all items** (not scoped to the current page), including on the Orphaned tab.
- Tab, sort, and page are reflected in the URL as query params (`?tab=movie&sort=year&page=2`). The search is local-only (not in the URL).
- Keyboard shortcuts: `?` toggles the help modal, `⌘K` / `Ctrl+K` focuses the search bar.
- Help modal documents all features with per-button visibility rules.
- Header buttons are conditionally visible: Import/Sync require a configured server, Sync requires items imported, Queue requires pending items.
- Import and Sync modals ping the server on open to show connectivity errors before the user can proceed.
- Error layout (502) shown when backend is unreachable.
- `KeepAlive` on RouterView avoids skeleton flash when navigating back from Settings.
- The interface feels clean and media-focused — dark theme with amber (`#E5A00D`) as primary color, whichever server is configured.
- Every server-specific label comes from `useServerStore` (`/api/server/status`), loaded by the router guard before the first render. Never hardcode "Plex" or "Jellyfin" in a component.

---

## Deferred for Future Versions

The following features are **not implemented in V1**. Code stubs and detailed specs are preserved in `DEFERRED.md` (not versioned).

### Poster Sources (TMDB / TVDB / Fanart.tv)
- "Find online" tab in the Change Poster modal — search external databases and pick from a scrollable grid with infinite scroll.
- API keys and source ordering configurable in Settings.
- Backend already has the DB schema and `settings.go` handler for sources — only the API integration and frontend tab need to be wired up.

### Poster Library (ZIP packs)
- Upload ZIP files from sites like ThePosterDB, name them, tag them, and browse images in a "Library" tab in the Change Poster modal.
- Dedicated `/library` page to manage packs (view, edit, delete).
- Auto-filter by tag intersection or title match when opening the Library tab for a media item.

---

## Environment Variables

| Variable           | Description                                                          |
| ------------------ | -------------------------------------------------------------------- |
| `MEDIA_SERVER`     | Active server: `plex` or `jellyfin`. Inferred when unset (see above)  |
|                    | Configuring *both* servers enables poster migration between them      |
| `PLEX_URL`         | Base URL of the Plex Media Server                                    |
| `PLEX_TOKEN`       | Plex authentication token                                            |
| `JELLYFIN_URL`     | Base URL of the Jellyfin server                                      |
| `JELLYFIN_API_KEY` | Jellyfin API key (Dashboard → Advanced → API Keys)                   |
| `AUTH_ENABLED`     | Enable login form (`true` / `false`)                                 |
| `AUTH_USER`        | Username for login (if auth enabled)                                 |
| `AUTH_PASS`        | Password for login (if auth enabled)                                 |
| `DB_PATH`          | Path to SQLite database file                                         |
| `DATA_PATH`        | Path to local poster storage directory                               |

---

## Docker

The application is packaged as a single Docker image containing both the Go backend and the built Vue frontend (served as static files by the Go server).

---

## API Endpoints

| Method   | Path                               | Description                                        |
| -------- | ---------------------------------- | -------------------------------------------------- |
| `GET`    | `/api/settings`                    | Get all settings (env vars + DB)                   |
| `POST`   | `/api/settings`                    | Save editable settings (options)                   |
| `GET`    | `/api/libraries`                   | List server libraries with enabled state from DB   |
| `POST`   | `/api/libraries`                   | Save per-library enabled/disabled state            |
| `GET`    | `/api/media`                       | List imported media items                          |
| `DELETE` | `/api/media/:ratingKey`            | Delete an orphaned media item                      |
| `GET`    | `/api/media/:ratingKey/thumb`      | Serve locally stored poster for a media item       |
| `POST`   | `/api/media/:ratingKey/upload`     | Upload a poster file (multipart)                   |
| `POST`   | `/api/media/:ratingKey/upload-url` | Fetch and store a poster from a URL (server-side)  |
| `POST`   | `/api/media/:ratingKey/push`       | Push local poster to the media server              |
| `GET`    | `/api/queue`                       | List pending poster changes                        |
| `DELETE` | `/api/queue/:ratingKey`            | Remove item from queue (restores server poster)    |
| `POST`   | `/api/queue/push-all`              | Push all queued posters to the media server        |
| `GET`    | `/api/server/status`               | Active provider, display name, and configured flag |
| `GET`    | `/api/server/ping`                 | Test connectivity and credential validity          |
| `POST`   | `/api/server/import`               | Import media from the server (SSE stream)          |
| `POST`   | `/api/server/sync`                 | Sync poster changes from the server (SSE stream)   |
| `GET`    | `/api/server/migrate/status`       | Whether posters can be carried over, and how many  |
| `POST`   | `/api/server/migrate`              | Carry posters to the active server (SSE stream)    |

---

## Project Structure

```
postr/
├── cmd/
│   └── postr/
│       └── main.go        # Application entrypoint; picks the media server client
├── internal/
│   ├── config/            # Env var config (caarlos0/env), provider resolution
│   ├── db/
│   │   ├── migrations/    # Goose SQL migrations
│   │   ├── queries/       # sqlc query definitions
│   │   └── *.sql.go       # Generated sqlc code
│   ├── handler/           # HTTP handlers (Echo v5), provider-agnostic
│   ├── mediaserver/       # Provider-neutral Client contract + shared types
│   │   ├── plex/          # Plex implementation
│   │   └── jellyfin/      # Jellyfin implementation
│   ├── migrate/           # Pure cross-server item pairing (no I/O)
│   └── posters/           # On-disk poster layout + legacy relocation
├── web/                   # Vue 3 + Vite frontend
│   └── src/
│       ├── components/
│       ├── composables/
│       ├── pages/
│       └── stores/        # incl. useServerStore (active provider + display name)
├── Dockerfile
└── AGENTS.md
```
