package handler

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/florentsorel/postr/internal/migrate"
	"github.com/florentsorel/postr/internal/posters"
	"github.com/labstack/echo/v5"
)

// migratableTypes are the media types carried over, in the order they are
// processed.
var migratableTypes = []string{
	mediaserver.TypeMovie,
	mediaserver.TypeShow,
	mediaserver.TypeSeason,
	mediaserver.TypeCollection,
}

type migrateStatusResponse struct {
	// Available reports whether a migration can be attempted at all: both
	// servers configured, and posters sitting under the inactive one.
	Available bool `json:"available"`
	// SourceName is the server the posters would come from, e.g. "Plex".
	SourceName string `json:"sourceName"`
	// SourceProvider is its machine identifier.
	SourceProvider string `json:"sourceProvider"`
	// PosterCount is how many locally stored posters the source holds.
	PosterCount int64 `json:"posterCount"`
	// TargetImported is how many items have been imported from the active
	// server. The migration needs them: artwork is attached to those rows.
	TargetImported int64  `json:"targetImported"`
	Reason         string `json:"reason,omitempty"`
}

// GetMigrateStatus tells the UI whether offering a poster migration makes sense.
func (h *Handler) GetMigrateStatus(c *echo.Context) error {
	ctx := c.Request().Context()
	resp := migrateStatusResponse{}

	if h.source == nil {
		resp.Reason = "No second media server is configured."
		return c.JSON(http.StatusOK, resp)
	}
	resp.SourceProvider = h.source.Provider()
	resp.SourceName = h.source.Name()

	sourceRows, err := h.db.ListMedia(ctx, resp.SourceProvider)
	if err != nil {
		return jsonInternalError(c, err)
	}
	targetRows, err := h.db.ListMedia(ctx, h.provider())
	if err != nil {
		return jsonInternalError(c, err)
	}
	resp.PosterCount = int64(len(sourceRows))
	resp.TargetImported = int64(len(targetRows))

	switch {
	case len(sourceRows) == 0:
		resp.Reason = "No posters were imported from " + resp.SourceName + "."
	case len(targetRows) == 0:
		resp.Reason = "Import your " + h.serverName() + " library first — posters are attached to imported items."
	default:
		resp.Available = true
	}
	return c.JSON(http.StatusOK, resp)
}

type sseMigrateDoneEvent struct {
	Type     string `json:"type"`
	Migrated int    `json:"migrated"`
	ByID     int    `json:"byId"`
	ByTitle  int    `json:"byTitle"`
	// Unchanged counts items whose artwork was already carried over by an
	// earlier run, so a repeated migration is a quiet no-op.
	Unchanged int `json:"unchanged"`
	Unmatched int `json:"unmatched"`
	Skipped   int `json:"skipped"`
}

type sseMigrateItemEvent struct {
	Type      string `json:"type"`
	Title     string `json:"title"`
	MediaType string `json:"mediaType"`
	Reason    string `json:"reason"`
}

// MigratePosters carries locally stored artwork from the inactive media server
// over to the active one, queueing every carried poster for review instead of
// pushing it straight to the server.
//
// The set of items to migrate comes from the database — that is what says which
// posters are actually held on disk — while both servers are queried live for
// the external identifiers that let their items be recognised as one another.
func (h *Handler) MigratePosters(c *echo.Context) error {
	if h.server == nil {
		return jsonError(c, http.StatusBadRequest, h.serverName()+" is not configured")
	}
	if h.source == nil {
		return jsonError(c, http.StatusBadRequest, "No second media server is configured to migrate from")
	}

	resp := c.Response()
	resp.Header().Set("Content-Type", "text/event-stream")
	resp.Header().Set("Cache-Control", "no-cache")
	resp.Header().Set("Connection", "keep-alive")
	resp.WriteHeader(http.StatusOK)

	rc := http.NewResponseController(resp)
	send := func(v any) {
		b, _ := json.Marshal(v)
		fmt.Fprintf(resp, "data: %s\n\n", b)
		_ = rc.Flush()
	}

	ctx := c.Request().Context()
	sourceProvider := h.source.Provider()

	slog.Info("poster migration started", "from", sourceProvider, "to", h.provider())

	sourceRows, err := h.db.ListMedia(ctx, sourceProvider)
	if err != nil {
		send(sseErrorEvent{Type: "error", Message: "failed to list " + h.source.Name() + " items"})
		return nil
	}
	targetRows, err := h.db.ListMedia(ctx, h.provider())
	if err != nil {
		send(sseErrorEvent{Type: "error", Message: "failed to list " + h.serverName() + " items"})
		return nil
	}
	if len(targetRows) == 0 {
		send(sseErrorEvent{Type: "error", Message: "Import your " + h.serverName() + " library before migrating posters."})
		return nil
	}

	// Identifiers are only on the servers, never in our database.
	sourceIDs, err := externalIDsByItem(ctx, h.source)
	if err != nil {
		send(sseErrorEvent{Type: "error", Message: "failed to read " + h.source.Name() + " metadata: " + err.Error()})
		return nil
	}
	targetIDs, err := externalIDsByItem(ctx, h.server)
	if err != nil {
		send(sseErrorEvent{Type: "error", Message: "failed to read " + h.serverName() + " metadata: " + err.Error()})
		return nil
	}

	sourceByType := itemsByType(sourceRows, sourceIDs)
	targetByType := itemsByType(targetRows, targetIDs)

	total := 0
	for _, t := range migratableTypes {
		total += len(sourceByType[t])
	}
	send(sseStartEvent{Type: "start", Total: total})

	targetRowByKey := make(map[string]db.ListMediaRow, len(targetRows))
	for _, r := range targetRows {
		targetRowByKey[r.RatingKey] = r
	}
	sourceExtByKey := make(map[string]string, len(sourceRows))
	for _, r := range sourceRows {
		ext := "jpg"
		if r.Thumb.Valid && r.Thumb.String != "" {
			ext = r.Thumb.String
		}
		sourceExtByKey[r.RatingKey] = ext
	}

	var migrated, byID, byTitle, unchanged, skipped, unmatchedCount, current int

	for _, mediaType := range migratableTypes {
		plan := migrate.BuildPlan(mediaType, sourceByType[mediaType], targetByType[mediaType])

		for _, m := range plan.Matches {
			current++
			targetRow, ok := targetRowByKey[m.TargetID]
			if !ok {
				// Live on the server but never imported: nothing to attach to.
				skipped++
				send(sseMigrateItemEvent{Type: "skipped", Title: m.Title, MediaType: mediaType, Reason: "not imported yet"})
				send(sseProgressEvent{Type: "progress", Current: current, Total: total})
				continue
			}

			ext := sourceExtByKey[m.SourceID]
			src := posters.Path(h.config.DataPath, sourceProvider, mediaType, m.SourceID, ext)
			dst := posters.Path(h.config.DataPath, h.provider(), mediaType, m.TargetID, ext)

			copied, err := copyPoster(src, dst)
			if err != nil {
				skipped++
				reason := "poster file missing"
				if !errors.Is(err, os.ErrNotExist) {
					reason = err.Error()
				}
				send(sseMigrateItemEvent{Type: "skipped", Title: m.Title, MediaType: mediaType, Reason: reason})
				send(sseProgressEvent{Type: "progress", Current: current, Total: total})
				continue
			}
			if !copied {
				// A previous run already carried this one over. Re-queueing it
				// would push bytes the server already holds.
				unchanged++
				send(sseProgressEvent{Type: "progress", Current: current, Total: total})
				continue
			}

			if err := h.queuePoster(ctx, targetRow.ID, m.TargetID, ext); err != nil {
				slog.Error("migration: failed to queue poster", "title", m.Title, "error", err)
				skipped++
				send(sseMigrateItemEvent{Type: "skipped", Title: m.Title, MediaType: mediaType, Reason: "database error"})
				send(sseProgressEvent{Type: "progress", Current: current, Total: total})
				continue
			}

			migrated++
			if m.By == migrate.ByExternalID {
				byID++
			} else {
				byTitle++
			}
			send(sseProgressEvent{Type: "progress", Current: current, Total: total})
		}

		for _, u := range plan.Unmatched {
			current++
			unmatchedCount++
			send(sseMigrateItemEvent{Type: "unmatched", Title: u.Title, MediaType: u.MediaType, Reason: u.Reason})
			send(sseProgressEvent{Type: "progress", Current: current, Total: total})
		}
	}

	slog.Info("poster migration done", "migrated", migrated, "byId", byID, "byTitle", byTitle,
		"unchanged", unchanged, "unmatched", unmatchedCount, "skipped", skipped)
	send(sseMigrateDoneEvent{
		Type: "done", Migrated: migrated, ByID: byID, ByTitle: byTitle,
		Unchanged: unchanged, Unmatched: unmatchedCount, Skipped: skipped,
	})
	return nil
}

// queuePoster marks a target item as locally modified and queues it, so the
// migrated artwork waits for the user's review instead of reaching the server.
func (h *Handler) queuePoster(ctx context.Context, mediaID int64, ratingKey, ext string) error {
	now := time.Now().Unix()
	if err := h.db.UpdateMediaThumb(ctx, db.UpdateMediaThumbParams{
		Thumb:     sql.NullString{String: ext, Valid: true},
		UpdatedAt: now,
		RatingKey: ratingKey,
	}); err != nil {
		return err
	}
	if err := h.db.SetLocallyModified(ctx, db.SetLocallyModifiedParams{
		LocallyModified: 1,
		UpdatedAt:       now,
		RatingKey:       ratingKey,
	}); err != nil {
		return err
	}
	return h.db.UpsertPosterQueue(ctx, db.UpsertPosterQueueParams{MediaID: mediaID, CreatedAt: now})
}

// externalIDsByItem asks a server for every item it holds and keeps only their
// external identifiers, keyed by the server's own item id.
func externalIDsByItem(ctx context.Context, client mediaserver.Client) (map[string]map[string]string, error) {
	libraries, err := client.Libraries(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[string]map[string]string)
	for _, lib := range libraries {
		for _, mediaType := range migratableTypes {
			// Collections only live in the collection library on servers that
			// keep them apart, and in movie libraries on those that do not.
			if !libraryHoldsType(client, lib, mediaType) {
				continue
			}
			items, err := client.Items(ctx, lib.Key, mediaType)
			if err != nil {
				return nil, err
			}
			for _, item := range items {
				if len(item.ExternalIDs) > 0 {
					out[item.ID] = item.ExternalIDs
				}
			}
		}
	}
	return out, nil
}

// libraryHoldsType reports whether asking a library for a media type can return
// anything, so the migration does not fire pointless requests.
func libraryHoldsType(client mediaserver.Client, lib mediaserver.Library, mediaType string) bool {
	switch mediaType {
	case mediaserver.TypeMovie:
		return lib.Type == mediaserver.TypeMovie
	case mediaserver.TypeShow, mediaserver.TypeSeason:
		return lib.Type == mediaserver.TypeShow
	case mediaserver.TypeCollection:
		if client.GlobalCollections() {
			return lib.Type == mediaserver.TypeCollection
		}
		return lib.Type == mediaserver.TypeMovie
	}
	return false
}

// itemsByType turns database rows into matchable items, attaching the external
// identifiers just read from the server.
func itemsByType(rows []db.ListMediaRow, externalIDs map[string]map[string]string) map[string][]mediaserver.Item {
	out := make(map[string][]mediaserver.Item)
	for _, r := range rows {
		if r.IsOrphan != 0 {
			continue
		}
		item := mediaserver.Item{
			ID:          r.RatingKey,
			Title:       r.Title,
			ExternalIDs: externalIDs[r.RatingKey],
		}
		if r.Year.Valid {
			item.Year = int(r.Year.Int64)
		}
		if r.SeasonNumber.Valid {
			item.SeasonNumber = int(r.SeasonNumber.Int64)
		}
		out[r.Type] = append(out[r.Type], item)
	}
	return out
}

// copyPoster copies a poster file, creating the destination directory, and
// reports whether anything was written. A destination already holding identical
// bytes is left alone, which is what makes a repeated migration a no-op instead
// of re-queueing artwork the server already has.
//
// The comparison is on bytes, so it only holds while the server stores what it
// was given. A server that re-encodes uploads will look changed on the next run.
//
// The write goes through a temporary file so a failure never leaves a truncated
// poster behind.
func copyPoster(src, dst string) (bool, error) {
	data, err := os.ReadFile(src)
	if err != nil {
		return false, err
	}

	if existing, err := os.ReadFile(dst); err == nil && bytes.Equal(existing, data) {
		return false, nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return false, err
	}

	tmp := dst + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return false, err
	}
	if err := os.Rename(tmp, dst); err != nil {
		_ = os.Remove(tmp)
		return false, err
	}
	return true, nil
}
