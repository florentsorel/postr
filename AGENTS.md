# Postr

A self-hosted web application for managing and updating poster artwork in Plex Media Server. Inspired by [Posteria](https://github.com/jeremehancock/Posteria), designed for homelab deployment via Docker.

---

## Purpose

Postr allows users to browse their Plex library and replace poster images for movies, TV series, seasons, and collections — either by uploading local files or fetching artwork from direct image URLs.

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

## Core Features (V1)

### 1. Plex Library Import

- A button triggers a sync with the connected Plex server.
- The user can choose which media types to import:
  - Movies
  - TV Series
  - Season posters
  - Collections
- Imported media metadata is stored locally in SQLite (title, type, year, `added_at` timestamp from Plex).
- The import streams real-time progress via SSE (`text/event-stream`). The frontend reads the stream and displays a progress bar + final recap.
- Import stats: **Added** (new items), **Skipped** (existing items whose poster is byte-identical — DB is not touched), **Deleted** (items removed from Plex). Thumbnail download failures appear in a separate errors accordion.
- During import, the current Plex poster for each item is **downloaded and stored locally** at `/data/posters/{type}/{ratingKey}.jpg`. The filename is the Plex `ratingKey` (string). Thumbnails are never served directly from Plex URLs (which require auth) — they are served by the Go backend at `/api/media/{ratingKey}/thumb`.
- Smart comparison: for existing items, the poster is downloaded and compared byte-for-byte before any DB write. If identical, the item is counted as skipped and the DB upsert is skipped entirely.

### 2. Sync from Plex

- Checks whether posters have been updated directly in Plex since the last import.
- Only checks items that have **not** been locally modified (`locally_modified = 0`) and are **not orphans** (`is_orphan = 0`).
- Compares each local poster byte-for-byte with the current Plex poster. Updates any that have changed.
- Streams real-time progress via SSE. Displays a progress bar while checking, then shows a recap on completion:
  - **Updated** items listed with a badge.
  - **Failed** items listed separately with the reason (e.g. "No longer exists in Plex").
- Items that return 404 from Plex are automatically **marked as orphans** (see Orphaned Items below).
- "All posters are up to date" message only shown when there are zero changes and zero failures.
- Plex connectivity is checked (ping) on modal open — sync button disabled if unreachable or token invalid.
- Does not add or remove items — only updates existing posters.
- Button only visible when at least one item has been imported.

### 3. Poster Management

Each media card exposes actions on hover:

**a) Change Poster**
- Opens a modal with two tabs:
  - **Upload** — user uploads an image file directly (drag & drop or browse). Auto-resize to Plex-compatible dimensions (configurable).
  - **From URL** — user pastes a direct image URL (JPG, PNG, WEBP). The server fetches the image server-side to avoid CORS issues.
- Once confirmed, the new poster is saved locally and queued for push to Plex.

**b) Send to Plex**
- Pushes the locally modified poster directly to Plex.
- Only visible on cards that have a pending change (item is in the queue).
- Pings Plex first: config errors (bad URL/token) show a specific toast and keep the item in the queue. If Plex returns 404, the item is marked as orphan.

**c) Get from Plex**
- Re-downloads the poster currently set in Plex and overwrites the local copy.
- Only visible on cards where the local poster has been locally modified (differs from Plex).
- Pings Plex first: config errors return an error toast and keep the item in the queue. If Plex returns 404, the item is marked as orphan.

### 4. Queue

- Lists all posters modified locally that are pending push to Plex.
- Push one at a time or all at once with "Push all to Plex".
- Removing an item from the queue restores the original Plex poster (pings Plex first — config errors keep the item in the queue).
- Button only visible when there are pending items.

### 5. Orphaned Items

- An item becomes an **orphan** (`is_orphan = 1`) when it is no longer found in Plex (HTTP 404) during: import, sync, Send to Plex, or Get from Plex.
- Orphans are **not** created for connectivity/token errors — only confirmed 404s.
- Orphaned items appear in a dedicated **Orphaned** tab (only visible when at least one orphan exists).
- The tab auto-disappears and the view switches back to "All" when the last orphan is deleted.
- A toast is shown immediately when an item becomes orphan after a user action.
- Orphaned items can be permanently deleted via a trash icon on the card.
- On re-import, if an orphaned item reappears in Plex (same `ratingKey`), `is_orphan` is reset to `0` automatically by the upsert.
- Note: Plex assigns new `ratingKey`s when an item is deleted and re-added — the old orphan record will remain until manually deleted.

### 6. Settings

Two categories of settings:

**Editable (stored in SQLite):**
- Option to enable/disable automatic image resizing on upload, and target width
- Per-library enable/disable toggle (which Plex libraries are included in imports)

**Read-only (from environment variables, displayed in UI but not editable):**
- Plex server URL and token — set via `PLEX_URL` / `PLEX_TOKEN` (token shown as set/not set only, never exposed)
- Auth status, username — set via `AUTH_ENABLED` / `AUTH_USER` / `AUTH_PASS`

The backend exposes `GET /api/settings` which returns both env-based config (read-only) and DB-stored settings. Only DB-stored settings are accepted on `POST /api/settings`.

`PLEX_URL` is normalized at startup: scheme defaults to `http://` if omitted, trailing slashes and paths are stripped. Invalid schemes (non http/https) cause a startup error.

### 7. Authentication (Optional)

- A login form protects the app for users who expose it to the public internet.
- All auth credentials are configured exclusively via environment variables — no database storage.
- Authentication can be disabled for purely local/homelab use by setting `AUTH_ENABLED=false`.

---

## UI / UX

- Media library is displayed in a **responsive grid layout** after import (2→3→4→5→6 columns).
- Each card shows the locally stored poster thumbnail, title, type badge, and year.
- On hover: **Change Poster**, **Send to Plex** (if queued), and **Get from Plex** (if locally modified) action buttons appear.
- Tabs filter by type: All / Movies / TV Series / Seasons / Collections / Orphaned (conditional).
- Sort options: Title (A–Z), Year, Recently Added (`addedAt` from Plex, stored in SQLite). Sort is hidden on the Orphaned tab.
- Search bar filters by title in real time across **all items** (not scoped to the current page), including on the Orphaned tab.
- Tab, sort, and page are reflected in the URL as query params (`?tab=movie&sort=year&page=2`). The search is local-only (not in the URL).
- Keyboard shortcuts: `?` toggles the help modal, `⌘K` / `Ctrl+K` focuses the search bar.
- Help modal documents all features with per-button visibility rules.
- Header buttons are conditionally visible: Import/Sync require Plex configured, Sync requires items imported, Queue requires pending items.
- Import and Sync modals ping Plex on open to show connectivity errors before the user can proceed.
- Error layout (502) shown when backend is unreachable.
- `KeepAlive` on RouterView avoids skeleton flash when navigating back from Settings.
- The interface feels clean and media-focused — dark theme with Plex yellow (`#E5A00D`) as primary color.

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

| Variable       | Description                                  |
| -------------- | -------------------------------------------- |
| `PLEX_URL`     | Base URL of the Plex Media Server            |
| `PLEX_TOKEN`   | Plex authentication token                    |
| `AUTH_ENABLED` | Enable login form (`true` / `false`)         |
| `AUTH_USER`    | Username for login (if auth enabled)         |
| `AUTH_PASS`    | Password for login (if auth enabled)         |
| `DB_PATH`      | Path to SQLite database file                 |
| `DATA_PATH`    | Path to local poster storage directory       |

---

## Docker

The application is packaged as a single Docker image containing both the Go backend and the built Vue frontend (served as static files by the Go server).

---

## API Endpoints

| Method   | Path                               | Description                                        |
| -------- | ---------------------------------- | -------------------------------------------------- |
| `GET`    | `/api/settings`                    | Get all settings (env vars + DB)                   |
| `POST`   | `/api/settings`                    | Save editable settings (options)                   |
| `GET`    | `/api/libraries`                   | List Plex libraries with enabled state from DB     |
| `POST`   | `/api/libraries`                   | Save per-library enabled/disabled state            |
| `GET`    | `/api/media`                       | List imported media items                          |
| `DELETE` | `/api/media/:ratingKey`            | Delete an orphaned media item                      |
| `GET`    | `/api/media/:ratingKey/thumb`      | Serve locally stored poster for a media item       |
| `POST`   | `/api/media/:ratingKey/upload`     | Upload a poster file (multipart)                   |
| `POST`   | `/api/media/:ratingKey/upload-url` | Fetch and store a poster from a URL (server-side)  |
| `POST`   | `/api/media/:ratingKey/push`       | Push local poster to Plex                          |
| `GET`    | `/api/queue`                       | List pending poster changes                        |
| `DELETE` | `/api/queue/:ratingKey`            | Remove item from queue (restores Plex poster)      |
| `POST`   | `/api/queue/push-all`              | Push all queued posters to Plex                    |
| `GET`    | `/api/plex/status`                 | Check if Plex is configured (URL + token set)      |
| `GET`    | `/api/plex/ping`                   | Test Plex connectivity and token validity          |
| `POST`   | `/api/plex/import`                 | Import media from Plex (SSE stream)                |
| `POST`   | `/api/plex/sync`                   | Sync poster changes from Plex (SSE stream)         |

---

## Project Structure

```
postr/
├── cmd/
│   └── api/
│       └── main.go        # Application entrypoint
├── db/
│   ├── migrations/        # Goose SQL migrations
│   ├── queries/           # sqlc query definitions
│   └── *.sql.go           # Generated sqlc code
├── internal/
│   ├── config/            # Env var config (caarlos0/env)
│   ├── handler/           # HTTP handlers (Echo v5)
│   └── plex/              # Plex HTTP client (*plex.Client constructed once in main.go)
├── web/                   # Vue 3 + Vite frontend
│   └── src/
│       ├── components/
│       ├── composables/
│       ├── pages/
│       └── stores/
├── Dockerfile
└── AGENTS.md
```
