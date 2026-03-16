package plex

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"
)

var ErrUnauthorized = errors.New("invalid Plex token")

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

func NewClient(baseURL, token string) *Client {
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// DownloadThumb fetches a raw image from a Plex thumb path (e.g. /library/metadata/123/thumb/...).
func (c *Client) DownloadThumb(ctx context.Context, thumbPath string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+thumbPath, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Plex-Token", c.token)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("plex returned %d for thumb %s", resp.StatusCode, thumbPath)
	}

	return io.ReadAll(resp.Body)
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

	if resp.StatusCode == http.StatusUnauthorized {
		return ErrUnauthorized
	}
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("plex returned %d for %s", resp.StatusCode, path)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

// Section represents a Plex library section.
type Section struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	Title string `json:"title"`
}

// Item represents a media item (movie, show, season, collection).
type Item struct {
	RatingKey string `json:"ratingKey"`
	Title     string `json:"title"`
	Type      string `json:"type"`
	Year      int    `json:"year"`
	Thumb     string `json:"thumb"`
	AddedAt   int64  `json:"addedAt"`
	Index     int    `json:"index"` // season number
}

type sectionsResponse struct {
	MediaContainer struct {
		Directory []Section `json:"Directory"`
	} `json:"MediaContainer"`
}

type itemsResponse struct {
	MediaContainer struct {
		Metadata []Item `json:"Metadata"`
	} `json:"MediaContainer"`
}

// Sections returns all library sections.
func (c *Client) Sections(ctx context.Context) ([]Section, error) {
	var r sectionsResponse
	if err := c.get(ctx, "/library/sections", &r); err != nil {
		return nil, err
	}
	return r.MediaContainer.Directory, nil
}

// AllItems returns all items in a section.
func (c *Client) AllItems(ctx context.Context, sectionKey string) ([]Item, error) {
	var r itemsResponse
	if err := c.get(ctx, "/library/sections/"+sectionKey+"/all", &r); err != nil {
		return nil, err
	}
	return r.MediaContainer.Metadata, nil
}

// Collections returns all collections in a section.
func (c *Client) Collections(ctx context.Context, sectionKey string) ([]Item, error) {
	var r itemsResponse
	if err := c.get(ctx, "/library/sections/"+sectionKey+"/collections", &r); err != nil {
		return nil, err
	}
	return r.MediaContainer.Metadata, nil
}

// Children returns the children of an item (e.g. seasons of a show).
func (c *Client) Children(ctx context.Context, ratingKey string) ([]Item, error) {
	var r itemsResponse
	if err := c.get(ctx, "/library/metadata/"+ratingKey+"/children", &r); err != nil {
		return nil, err
	}
	return r.MediaContainer.Metadata, nil
}
