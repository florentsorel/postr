# Postr

A self-hosted web application for managing and updating poster artwork in Plex Media Server. Inspired by [Posteria](https://github.com/jeremehancock/Posteria), designed for homelab deployment via Docker.

---

## Purpose

Postr allows users to browse their Plex library and replace poster images for movies, TV series, seasons, and collections — either by uploading local files or fetching artwork from external poster sources.

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

## Core Features

### 1. Plex Library Import

- A button triggers a sync with the connected Plex server.
- The user can choose which media types to import:
  - Movies
  - TV Series
  - Season posters
  - Collections
- Imported media metadata is stored locally in SQLite (title, type, year, `added_at` timestamp from Plex).
- During import, the current Plex poster for each item is **downloaded and stored locally** at `/data/posters/{type}/{id}.jpg`. Thumbnails are never served directly from Plex URLs (which require auth) — they are served by the Go backend at `/api/media/{id}/thumb`.
- Smart comparison: a poster is only written to disk if its content differs from the existing local file.

### 2. Poster Management

Each media card exposes two actions on hover:

**a) Change Poster**
- Opens a modal with three tabs:
  - **Upload** — user uploads an image file directly. Optional auto-resize to Plex-compatible dimensions.
  - **Library** — browse images from previously uploaded ZIP packs (see § Poster Library). Displayed as a scrollable grid; user picks one image.
  - **Find online** — queries enabled poster sources and displays results in a scrollable grid (infinite scroll). Sources are fetched in the order defined in Settings.
  - **From URL** — user pastes a direct image URL or a Mediux YAML set URL. If a Mediux YAML URL is detected, it is fetched and parsed to display the posters it contains.
- Once confirmed, the new poster is saved locally **and automatically pushed to Plex** in one step.

**Poster sources:**

| Source | API | Notes |
| ------------ | --- | ------------------------------------ |
| TMDB | ✅ | Official REST API, requires API key |
| TVDB | ✅ | Official REST API v4, requires API key |
| Fanart.tv | ✅ | Official REST API, requires API key |
| Mediux | ❌ | No public API — supported via YAML set URLs pasted in "From URL" |
| ThePosterDB | ❌ | No public API — not integrated programmatically |

TMDB, TVDB, and Fanart.tv API keys are configurable in Settings. Mediux is supported through its YAML set format only (no search).

**b) Send to Plex**
- Pushes the locally stored poster to Plex without picking a new one.
- Useful to restore a poster if Plex lost or overwrote it outside of Postr.

**c) Get from Plex**
- Re-downloads the poster currently set in Plex for that item and overwrites the local copy.
- Useful for resyncing when a poster was changed directly in Plex outside of Postr, or as a manual backup.

### 3. Poster Library (ZIP Import)

A dedicated page (`/library`) for managing locally stored poster packs.

**Purpose:** Sites like ThePosterDB let users download a ZIP containing posters for an entire collection (e.g., all X-Men films + the collection art). Rather than re-uploading the same ZIP every time the "Change Poster" modal is opened, users upload once to the Library and reuse images from there.

**Upload flow:**
- User uploads a `.zip` file, gives it a friendly display name (e.g., "X-Men Collection" instead of "xmen-pack-by-toto-2024"), and optionally adds **tags** — free-form strings matching the media titles or franchises covered (e.g., `x-men`, `wolverine`, `logan`). A single pack can cover multiple franchises.
- The backend extracts the ZIP and stores each image at `/data/library/{pack_id}/{filename}`
- SQLite schema:
  - `library_packs`: `id`, `name`, `created_at`
  - `tags`: `id`, `name` (shared vocabulary)
  - `library_pack_tags`: `pack_id`, `tag_id` (pack ↔ tag association)
  - `media_tags`: `media_id`, `tag_id` (optional — reserved for future explicit media tagging)

**In the Change Poster modal — "Library" tab:**
- When the tab opens, packs are **automatically filtered**: if the media item has entries in `media_tags`, matching is done via tag intersection (`pack.tags ∩ media.tags ≠ ∅`); otherwise it falls back to a case-insensitive substring match of the media title against tag names. No tagging required from the user.
- A "Show all packs" toggle clears the filter to browse the full library — useful when auto-filter misses a match.
- Images are displayed in a scrollable grid (same UI as "Find online"). User picks one → works like any other poster selection.

**Library page (`/library`):**
- Lists all packs with name, tag list, image count, and upload date.
- Upload form: ZIP file input + name field + tag input (pill input, autocomplete from existing tags).
- Per-pack actions: view images, edit name/tags, delete (removes files + DB record).

**Backend API:**
- `GET /api/library` — list all packs (id, name, tags, image count, created_at)
- `POST /api/library` — upload a ZIP + name + tags, returns the created pack
- `GET /api/library/:id` — list images in a pack
- `PATCH /api/library/:id` — update name and/or tags
- `DELETE /api/library/:id` — delete a pack and its files
- Images served at `/api/library/:id/:filename`

**Storage:** `/data/library/{pack_id}/` — sibling of `/data/posters/`, covered by the same `DATA_PATH` env var.

### 4. Settings

Two categories of settings:

**Editable (stored in SQLite):**
- Toggle which poster sources are enabled (TMDB, TVDB, Fanart.tv) and their order (drag to reorder — first enabled source is used by default)
- API keys for TMDB, TVDB, Fanart.tv
- Option to enable/disable automatic image resizing on upload

**Read-only (from environment variables, displayed in UI but not editable):**
- Plex server URL and token — set via `PLEX_URL` / `PLEX_TOKEN`
- Auth status, username — set via `AUTH_ENABLED` / `AUTH_USER` / `AUTH_PASS`

The backend exposes `GET /api/settings` which returns both env-based config (read-only) and DB-stored settings. Only DB-stored settings are accepted on `POST /api/settings`.

### 4. Authentication (Optional)

- A login form protects the app for users who expose it to the public internet.
- All auth credentials are configured exclusively via environment variables — no database storage.
- Authentication can be disabled for purely local/homelab use by setting `AUTH_ENABLED=false`.

---

## UI / UX

- Media library is displayed in a **responsive grid layout** after import (2→3→4→5→6 columns).
- Each card shows the locally stored poster thumbnail, title, type badge, and year.
- On hover: **Change Poster**, **Send to Plex**, and **Get from Plex** action buttons appear.
- Tabs filter by type: All / Movies / TV Series / Seasons / Collections.
- Sort options: Title (A–Z), Type, Year, Recently Added (`addedAt` from Plex, stored in SQLite).
- Search bar filters by title in real time across **all items** (not scoped to the current page).
- Tab, sort, and page are reflected in the URL as query params (`?tab=movie&sort=year&page=2`). The search is local-only (not in the URL). Page is preserved across tab/sort changes and clamped when search reduces the result count.
- Empty state shown when no media has been imported yet, with a CTA to trigger the import.
- The interface should feel clean and media-focused — dark theme with Plex yellow (`#E5A00D`) as primary color.

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

## Project Structure (Planned)

```
postr/
├── cmd/
│   └── api/
│       └── main.go   # Application entrypoint
├── plex/             # Plex API client
├── db/               # SQLite models & queries
├── handlers/         # HTTP handlers
├── library/          # ZIP extraction and poster library management
├── sources/          # Poster source fetchers (TMDB, TVDB, etc.)
├── web/              # Vue + Vite app
│   ├── src/
│   │   ├── components/
│   │   ├── pages/         # LibraryPage, SettingsPage, PosterLibraryPage
│   │   └── hooks/
│   └── public/
├── Dockerfile
├── docker-compose.yml
└── AGENTS.md
```
