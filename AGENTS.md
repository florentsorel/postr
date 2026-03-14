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
- Opens a modal to pick a new poster via upload or external source fetch.
- Once confirmed, the new poster is saved locally **and automatically pushed to Plex** in one step.
- Upload: user uploads an image file directly. Optional auto-resize to Plex-compatible dimensions.
- Fetch: queries the enabled poster sources (TMDB, TVDB, Fanart.tv, Mediux, ThePosterDB) and displays results to pick from.

**b) Send to Plex**
- Pushes the locally stored poster to Plex without picking a new one.
- Useful to restore a poster if Plex lost or overwrote it outside of Postr.

**c) Get from Plex**
- Re-downloads the poster currently set in Plex for that item and overwrites the local copy.
- Useful for resyncing when a poster was changed directly in Plex outside of Postr, or as a manual backup.

### 3. Settings

Two categories of settings:

**Editable (stored in SQLite):**
- Toggle which poster sources are enabled (TMDB, TVDB, Fanart.tv, Mediux, ThePosterDB)
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
├── sources/          # Poster source fetchers (TMDB, TVDB, etc.)
├── web/              # Vue + Vite app
│   ├── src/
│   │   ├── components/
│   │   ├── pages/
│   │   └── hooks/
│   └── public/
├── Dockerfile
├── docker-compose.yml
└── AGENTS.md
```
