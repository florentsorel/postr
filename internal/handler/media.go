package handler

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"mime"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/florentsorel/postr/internal/db"
	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/florentsorel/postr/internal/posters"
	"github.com/labstack/echo/v5"
)

type mediaResponse struct {
	ID              int64  `json:"id"`
	RatingKey       string `json:"ratingKey"`
	Title           string `json:"title"`
	Type            string `json:"type"`
	Year            *int64 `json:"year,omitempty"`
	SeasonNumber    *int64 `json:"seasonNumber,omitempty"`
	Thumb           string `json:"thumb,omitempty"`
	LocallyModified bool   `json:"locallyModified"`
	IsOrphan        bool   `json:"isOrphan"`
	AddedAt         *int64 `json:"addedAt,omitempty"`
}

func (h *Handler) GetMediaThumb(c *echo.Context) error {
	ratingKey := c.Param("ratingKey")

	m, err := h.db.GetMediaByRatingKey(c.Request().Context(), ratingKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return jsonError(c, http.StatusNotFound, "media not found")
		}
		return jsonInternalError(c, err)
	}

	ext := "jpg"
	if m.Thumb.Valid && m.Thumb.String != "" {
		ext = m.Thumb.String
	}
	path := h.posterPath(m.Type, ratingKey, ext)
	c.Response().Header().Set("Cache-Control", "public, max-age=31536000, immutable")
	http.ServeFile(c.Response(), c.Request(), path)
	return nil
}

// extFromFilename returns the file extension to use for storing a poster,
// based on the uploaded filename.
func extFromFilename(filename string) string {
	switch strings.ToLower(filepath.Ext(filename)) {
	case ".png":
		return "png"
	case ".webp":
		return "webp"
	default:
		return "jpg"
	}
}

// resizeTarget returns the width user-supplied posters should be scaled down
// to, or 0 when the option is off. Failing to read the setting must not block an
// upload, so it falls back to the same defaults GetSettings reports.
func (h *Handler) resizeTarget(ctx context.Context) int {
	const defaultWidth = 1000

	enabled, err := h.db.GetSetting(ctx, db.GetSettingParams{Type: "option", Key: "auto_resize"})
	if err != nil {
		slog.Warn("could not read the auto_resize setting, leaving posters untouched", "error", err)
		return 0
	}
	if !enabled.Value.Valid || enabled.Value.String != "true" {
		return 0
	}

	width, err := h.db.GetSetting(ctx, db.GetSettingParams{Type: "option", Key: "resize_width"})
	if err != nil || !width.Value.Valid {
		return defaultWidth
	}
	w, err := strconv.Atoi(width.Value.String)
	if err != nil || w <= 0 {
		return defaultWidth
	}
	return w
}

// storePoster writes poster data to disk, updates the DB, and enqueues the item.
// It returns the extension the poster was finally stored under, which can differ
// from the one passed in when resizing re-encodes the image.
//
// Only user-supplied artwork goes through here — import and sync write what the
// media server gave them, untouched.
func (h *Handler) storePoster(ctx context.Context, m db.GetMediaByRatingKeyRow, ratingKey, ext string, data []byte) (string, error) {
	if width := h.resizeTarget(ctx); width > 0 {
		resized, newExt, err := posters.Resize(data, ext, width)
		if err != nil {
			// Storing the original beats rejecting an upload over a format the
			// decoder did not recognise.
			slog.Warn("could not resize poster, storing it as-is", "ratingKey", ratingKey, "error", err)
		} else {
			if len(resized) != len(data) {
				slog.Info("poster resized", "ratingKey", ratingKey, "width", width,
					"from", len(data), "to", len(resized))
			}
			data, ext = resized, newExt
		}
	}

	if err := os.MkdirAll(h.posterDir(m.Type), 0o755); err != nil {
		return "", err
	}
	for _, oldExt := range []string{"jpg", "jpeg", "png", "webp"} {
		if oldExt != ext {
			_ = os.Remove(h.posterPath(m.Type, ratingKey, oldExt))
		}
	}
	if err := os.WriteFile(h.posterPath(m.Type, ratingKey, ext), data, 0o644); err != nil {
		return "", err
	}
	now := time.Now().Unix()
	if err := h.db.UpdateMediaThumb(ctx, db.UpdateMediaThumbParams{
		Thumb:     sql.NullString{String: ext, Valid: true},
		UpdatedAt: now,
		RatingKey: ratingKey,
	}); err != nil {
		return "", err
	}
	if err := h.db.SetLocallyModified(ctx, db.SetLocallyModifiedParams{
		LocallyModified: 1,
		UpdatedAt:       now,
		RatingKey:       ratingKey,
	}); err != nil {
		return "", err
	}
	return ext, h.db.UpsertPosterQueue(ctx, db.UpsertPosterQueueParams{
		MediaID:   m.ID,
		CreatedAt: now,
	})
}

func (h *Handler) UploadMediaPoster(c *echo.Context) error {
	ratingKey := c.Param("ratingKey")
	ctx := c.Request().Context()

	m, err := h.db.GetMediaByRatingKey(ctx, ratingKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return jsonError(c, http.StatusNotFound, "media not found")
		}
		return jsonInternalError(c, err)
	}

	file, err := c.FormFile("file")
	if err != nil {
		return jsonError(c, http.StatusBadRequest, "file required")
	}

	src, err := file.Open()
	if err != nil {
		return jsonInternalError(c, err)
	}
	defer src.Close()

	data, err := io.ReadAll(src)
	if err != nil {
		return jsonInternalError(c, err)
	}

	ext, err := h.storePoster(ctx, m, ratingKey, extFromFilename(file.Filename), data)
	if err != nil {
		return jsonInternalError(c, err)
	}

	slog.Info("poster uploaded", "type", m.Type, "title", m.Title, "ratingKey", ratingKey, "source", "file")
	return c.JSON(http.StatusOK, map[string]string{"ext": ext, "thumb": "/api/media/" + ratingKey + "/thumb"})
}

func extFromContentType(ct string) string {
	switch {
	case strings.HasPrefix(ct, "image/png"):
		return "png"
	case strings.HasPrefix(ct, "image/webp"):
		return "webp"
	case strings.HasPrefix(ct, "image/jpeg"):
		return "jpg"
	default:
		return ""
	}
}

// maxPosterBytes caps what Postr will pull from a remote URL. Poster sites
// routinely serve 10-15 MB scans — the previous 10 MB ceiling silently cut them
// in half and stored the truncated result.
const maxPosterBytes = 32 << 20

// posterFetchUserAgent identifies Postr honestly. It exists because Go's default
// "Go-http-client/1.1" is rejected outright by the bot protection in front of
// several poster sites (ThePosterDB answers 403); any real name gets through, so
// there is no reason to impersonate a browser.
const posterFetchUserAgent = "Postr (+https://github.com/florentsorel/postr)"

// posterFetchClient has the timeout http.DefaultClient lacks: without one, a
// remote host that accepts the connection and then stalls would hang the
// request until the user gives up.
var posterFetchClient = &http.Client{Timeout: 60 * time.Second}

// fetchRemotePoster downloads an image by URL and returns its bytes and file
// extension. It refuses anything that is not an image, and refuses — rather
// than truncates — anything past maxPosterBytes.
func fetchRemotePoster(ctx context.Context, rawURL string) (data []byte, ext string, status int, err error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, "", http.StatusBadRequest, errors.New("invalid url")
	}
	req.Header.Set("User-Agent", posterFetchUserAgent)
	req.Header.Set("Accept", "image/jpeg,image/png,image/webp,image/*;q=0.8")

	resp, err := posterFetchClient.Do(req)
	if err != nil {
		return nil, "", http.StatusBadGateway, errors.New("failed to fetch URL")
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", http.StatusBadGateway, errors.New("URL returned status " + strconv.Itoa(resp.StatusCode))
	}

	// Read one byte past the ceiling: if it arrives, the image is over the limit
	// and the rest would have been dropped without anyone noticing.
	data, err = io.ReadAll(io.LimitReader(resp.Body, maxPosterBytes+1))
	if err != nil {
		return nil, "", http.StatusBadGateway, errors.New("failed to read the image")
	}
	if len(data) > maxPosterBytes {
		return nil, "", http.StatusRequestEntityTooLarge,
			errors.New("image is larger than " + strconv.Itoa(maxPosterBytes>>20) + " MB")
	}
	if len(data) == 0 {
		return nil, "", http.StatusBadGateway, errors.New("the URL returned an empty response")
	}

	ext = posterExt(resp, rawURL, data)
	if ext == "" {
		// A site answering with an HTML error page under a 200 would otherwise
		// be stored as a poster and pushed to the media server.
		return nil, "", http.StatusUnsupportedMediaType,
			errors.New("the URL did not return a JPEG, PNG or WEBP image")
	}
	return data, ext, http.StatusOK, nil
}

// posterExt determines the file extension, preferring what the server declares
// and falling back to what the bytes actually are. Poster sites often serve
// extension-less URLs, naming the file in Content-Disposition instead.
func posterExt(resp *http.Response, rawURL string, data []byte) string {
	if ext := extFromContentType(resp.Header.Get("Content-Type")); ext != "" {
		return ext
	}
	if cd := resp.Header.Get("Content-Disposition"); cd != "" {
		if _, params, err := mime.ParseMediaType(cd); err == nil {
			if ext := extFromFilename(params["filename"]); params["filename"] != "" && ext != "" {
				return ext
			}
		}
	}
	// Sniffing beats the URL: a path can end in .jpg and serve anything.
	if ext := extFromContentType(http.DetectContentType(data)); ext != "" {
		return ext
	}
	return ""
}

// previewWidth is enough for the 160pt preview box on a 3x display, and keeps
// the round-trip small: poster sites serve 10 MB scans for a thumbnail nobody
// looks at full size.
const previewWidth = 480

// GetPosterPreview fetches a remote image through the backend and returns a
// small version of it.
//
// The browser cannot load these URLs itself: it sends a Referer carrying
// Postr's own origin, and the bot protection in front of poster sites answers
// 403 to it (ThePosterDB rejects a localhost Referer while accepting a public
// one). The server sends no Referer at all, which is why the same URL works
// from here.
//
// This reaches arbitrary URLs on behalf of the user, but adds no capability
// that POST /api/media/:ratingKey/upload-url did not already have, and sits
// behind the same authentication.
func (h *Handler) GetPosterPreview(c *echo.Context) error {
	rawURL := strings.TrimSpace(c.QueryParam("url"))
	if rawURL == "" {
		return jsonError(c, http.StatusBadRequest, "url required")
	}

	data, ext, status, err := fetchRemotePoster(c.Request().Context(), rawURL)
	if err != nil {
		return jsonError(c, status, err.Error())
	}

	// A failure here is not worth surfacing: the full-size image is still a
	// valid preview, just heavier.
	if small, smallExt, resizeErr := posters.Resize(data, ext, previewWidth); resizeErr == nil {
		data, ext = small, smallExt
	} else {
		slog.Warn("could not shrink poster preview", "url", rawURL, "error", resizeErr)
	}

	// The URL is user input that changes as they type; caching it would only
	// serve stale bytes.
	c.Response().Header().Set("Cache-Control", "no-store")
	return c.Blob(http.StatusOK, mediaserver.ContentTypeFromExt(ext), data)
}

func (h *Handler) UploadPosterFromURL(c *echo.Context) error {
	ratingKey := c.Param("ratingKey")
	ctx := c.Request().Context()

	m, err := h.db.GetMediaByRatingKey(ctx, ratingKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return jsonError(c, http.StatusNotFound, "media not found")
		}
		return jsonInternalError(c, err)
	}

	var body struct {
		URL string `json:"url"`
	}
	if err := c.Bind(&body); err != nil || strings.TrimSpace(body.URL) == "" {
		return jsonError(c, http.StatusBadRequest, "url required")
	}

	data, ext, status, fetchErr := fetchRemotePoster(ctx, strings.TrimSpace(body.URL))
	if fetchErr != nil {
		slog.Warn("failed to fetch poster from URL", "url", body.URL, "error", fetchErr)
		return jsonError(c, status, fetchErr.Error())
	}

	ext, err = h.storePoster(ctx, m, ratingKey, ext, data)
	if err != nil {
		return jsonInternalError(c, err)
	}

	slog.Info("poster uploaded", "type", m.Type, "title", m.Title, "ratingKey", ratingKey, "source", "url")
	return c.JSON(http.StatusOK, map[string]string{"ext": ext, "thumb": "/api/media/" + ratingKey + "/thumb"})
}

func (h *Handler) GetMedia(c *echo.Context) error {
	rows, err := h.db.ListMedia(c.Request().Context(), h.provider())
	if err != nil {
		return jsonInternalError(c, err)
	}

	items := make([]mediaResponse, 0, len(rows))
	for _, m := range rows {
		item := mediaResponse{
			ID:        m.ID,
			RatingKey: m.RatingKey,
			Title:     m.Title,
			Type:      m.Type,
		}
		if m.Year.Valid {
			item.Year = &m.Year.Int64
		}
		if m.Thumb.Valid {
			v := strconv.FormatInt(m.UpdatedAt, 10)
			item.Thumb = "/api/media/" + m.RatingKey + "/thumb?v=" + v
		}
		if m.SeasonNumber.Valid {
			item.SeasonNumber = &m.SeasonNumber.Int64
		}
		if m.AddedAt.Valid {
			item.AddedAt = &m.AddedAt.Int64
		}
		item.LocallyModified = m.LocallyModified != 0
		item.IsOrphan = m.IsOrphan != 0
		items = append(items, item)
	}

	return c.JSON(http.StatusOK, items)
}

func (h *Handler) DeleteOrphan(c *echo.Context) error {
	ratingKey := c.Param("ratingKey")
	ctx := c.Request().Context()

	m, err := h.db.GetMediaByRatingKey(ctx, ratingKey)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return jsonError(c, http.StatusNotFound, "media not found")
		}
		return jsonInternalError(c, err)
	}

	if err := h.db.DeleteMediaByRatingKey(ctx, ratingKey); err != nil {
		return jsonInternalError(c, err)
	}

	for _, ext := range []string{"jpg", "png", "webp"} {
		_ = os.Remove(h.posterPath(m.Type, ratingKey, ext))
	}

	slog.Info("orphan deleted", "type", m.Type, "title", m.Title, "ratingKey", ratingKey)
	return c.NoContent(http.StatusNoContent)
}
