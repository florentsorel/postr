// Package plex implements mediaserver.Client for Plex Media Server.
package plex

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/florentsorel/postr/internal/mediaserver"
)

// Client talks to a Plex Media Server using an X-Plex-Token.
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// NewClient returns a Plex client for the given base URL and token.
func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Provider() string { return mediaserver.ProviderPlex }

func (c *Client) Name() string { return "Plex" }

// GlobalCollections is false: Plex collections belong to the section that
// contains them.
func (c *Client) GlobalCollections() bool { return false }

func (c *Client) Ping(ctx context.Context) error {
	var r sectionsResponse
	return c.get(ctx, "/library/sections", &r)
}

// section is a Plex library section.
type section struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

// item is a Plex metadata entry (movie, show, season, collection).
type item struct {
	RatingKey             string `json:"ratingKey"`
	Title                 string `json:"title"`
	Type                  string `json:"type"`
	Year                  int    `json:"year"`
	Thumb                 string `json:"thumb"`
	AddedAt               int64  `json:"addedAt"`
	Index                 int    `json:"index"`                 // season number
	OriginallyAvailableAt string `json:"originallyAvailableAt"` // e.g. "2014-01-13"
	// GuidString is Plex's own opaque reference, e.g. "plex://movie/5d7768...".
	// It is useless to another server, but it MUST be declared: Plex sends both
	// "guid" (this string) and "Guid" (the array below), and encoding/json falls
	// back to case-insensitive key matching. Without an exact match for the
	// lowercase key it would land on Guid and fail the whole decode with
	// "cannot unmarshal string into ... []struct".
	GuidString string `json:"guid"`
	// Guid holds external database references such as "tmdb://27205". Plex only
	// fills it when the request asks with includeGuids=1.
	Guid []struct {
		ID string `json:"id"`
	} `json:"Guid"`
}

// externalIDs turns Plex's "source://id" references into a source-keyed map.
func (i item) externalIDs() map[string]string {
	if len(i.Guid) == 0 {
		return nil
	}
	ids := make(map[string]string, len(i.Guid))
	for _, g := range i.Guid {
		source, id, ok := strings.Cut(g.ID, "://")
		if !ok || id == "" {
			continue
		}
		// Plex's own "plex://movie/5d77..." reference means nothing to another
		// server, so it is not worth keeping.
		if source = strings.ToLower(source); source == "plex" {
			continue
		}
		ids[source] = id
	}
	if len(ids) == 0 {
		return nil
	}
	return ids
}

// seasonYear returns the year from OriginallyAvailableAt if set, otherwise
// falls back to Year.
func (i item) seasonYear() int {
	if len(i.OriginallyAvailableAt) >= 4 {
		y := 0
		for _, ch := range i.OriginallyAvailableAt[:4] {
			if ch < '0' || ch > '9' {
				return i.Year
			}
			y = y*10 + int(ch-'0')
		}
		return y
	}
	return i.Year
}

type sectionsResponse struct {
	MediaContainer struct {
		Directory []section `json:"Directory"`
	} `json:"MediaContainer"`
}

type itemsResponse struct {
	MediaContainer struct {
		Metadata []item `json:"Metadata"`
	} `json:"MediaContainer"`
}

func (c *Client) Libraries(ctx context.Context) ([]mediaserver.Library, error) {
	var r sectionsResponse
	if err := c.get(ctx, "/library/sections", &r); err != nil {
		return nil, err
	}

	libraries := make([]mediaserver.Library, 0, len(r.MediaContainer.Directory))
	for _, s := range r.MediaContainer.Directory {
		if s.Type != mediaserver.TypeMovie && s.Type != mediaserver.TypeShow {
			continue
		}
		libraries = append(libraries, mediaserver.Library{Key: s.Key, Title: s.Title, Type: s.Type})
	}
	return libraries, nil
}

func (c *Client) Items(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
	switch mediaType {
	case mediaserver.TypeMovie, mediaserver.TypeShow:
		raw, err := c.sectionItems(ctx, "/library/sections/"+libraryKey+"/all?includeGuids=1")
		if err != nil {
			return nil, err
		}
		return convert(raw), nil

	case mediaserver.TypeCollection:
		raw, err := c.sectionItems(ctx, "/library/sections/"+libraryKey+"/collections")
		// Plex tracks no external database reference for a collection, so these
		// items are matched by title alone.
		if err != nil {
			return nil, err
		}
		return convert(raw), nil

	case mediaserver.TypeSeason:
		return c.seasons(ctx, libraryKey)

	default:
		return nil, fmt.Errorf("plex: unsupported media type %q", mediaType)
	}
}

// seasons walks every show in a section and collects its seasons. Plex does not
// expose the season air date on the season itself, so the year is taken from
// the first episode and falls back to the show's year.
func (c *Client) seasons(ctx context.Context, libraryKey string) ([]mediaserver.Item, error) {
	shows, err := c.sectionItems(ctx, "/library/sections/"+libraryKey+"/all?includeGuids=1")
	if err != nil {
		return nil, err
	}

	var out []mediaserver.Item
	for _, show := range shows {
		seasons, err := c.sectionItems(ctx, "/library/metadata/"+show.RatingKey+"/children")
		if err != nil {
			return nil, err
		}
		for _, s := range seasons {
			year := show.Year
			if episodes, err := c.sectionItems(ctx, "/library/metadata/"+s.RatingKey+"/children"); err == nil && len(episodes) > 0 {
				year = episodes[0].seasonYear()
			}
			out = append(out, mediaserver.Item{
				ID:           s.RatingKey,
				Title:        show.Title,
				Year:         year,
				SeasonNumber: s.Index,
				AddedAt:      s.AddedAt,
				HasPoster:    s.Thumb != "",
				// Plex does not give a season its own external reference, so a
				// season is identified by its show plus its season number.
				ExternalIDs: show.externalIDs(),
			})
		}
	}
	return out, nil
}

func (c *Client) sectionItems(ctx context.Context, path string) ([]item, error) {
	var r itemsResponse
	if err := c.get(ctx, path, &r); err != nil {
		return nil, err
	}
	return r.MediaContainer.Metadata, nil
}

func convert(raw []item) []mediaserver.Item {
	out := make([]mediaserver.Item, 0, len(raw))
	for _, i := range raw {
		out = append(out, mediaserver.Item{
			ID:          i.RatingKey,
			Title:       i.Title,
			Year:        i.Year,
			AddedAt:     i.AddedAt,
			HasPoster:   i.Thumb != "",
			ExternalIDs: i.externalIDs(),
		})
	}
	return out
}

// DownloadPoster fetches the poster currently set in Plex for an item. The
// unversioned /thumb path always resolves to the selected poster.
func (c *Client) DownloadPoster(ctx context.Context, itemID string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+"/library/metadata/"+itemID+"/thumb", nil)
	if err != nil {
		return nil, "", err
	}
	req.Header.Set("X-Plex-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if err := statusError(resp.StatusCode, "poster for "+itemID); err != nil {
		return nil, "", err
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}
	return data, mediaserver.ExtFromContentType(resp.Header.Get("Content-Type")), nil
}

// UploadPoster uploads an image to Plex as the poster for the given item.
func (c *Client) UploadPoster(ctx context.Context, itemID string, data []byte, contentType string) error {
	url := c.baseURL + "/library/metadata/" + itemID + "/posters"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(data))
	if err != nil {
		return err
	}
	req.Header.Set("X-Plex-Token", c.token)
	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return statusError(resp.StatusCode, "poster upload for "+itemID)
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("X-Plex-Token", c.token)
	req.Header.Set("Accept", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if err := statusError(resp.StatusCode, path); err != nil {
		return err
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// statusError maps an HTTP status to the provider-neutral sentinel errors.
func statusError(status int, what string) error {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return mediaserver.ErrUnauthorized
	case status == http.StatusNotFound:
		return mediaserver.ErrNotFound
	case status < 200 || status >= 300:
		return fmt.Errorf("plex returned %d for %s", status, what)
	}
	return nil
}
