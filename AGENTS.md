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
- Imported media is stored locally in SQLite and displayed in a responsive grid layout.

### 2. Poster Management

Each media item supports two ways to change its poster:

**a) Local Upload**
- User uploads an image file directly.
- Optional setting to auto-resize the image to Plex-compatible dimensions.

**b) Fetch from External Sources**
- Supported poster sources:
  - [TMDB](https://www.themoviedb.org/)
  - [TVDB](https://thetvdb.com/)
  - [Fanart.tv](https://fanart.tv/)
  - [Mediux.pro](https://mediux.pro/)
  - [ThePosterDB](https://theposterdb.com/)
- In Settings, the user selects which sources are active (multiple allowed).
- When clicking "Change Poster" on a media item, the fetch only queries the selected sources.

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

- Media library is displayed in a **grid layout** after import.
- Each card shows the current poster thumbnail, title, and a "Change Poster" action button.
- The interface should feel clean and media-focused — take visual inspiration from media manager UIs (e.g., poster grids similar to Plex/Jellyfin dashboards).

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
