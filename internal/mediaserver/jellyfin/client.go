// Package jellyfin implements mediaserver.Client for Jellyfin.
package jellyfin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/florentsorel/postr/internal/mediaserver"
)

// Jellyfin item types, as expected by the IncludeItemTypes query parameter.
const (
	itemTypeMovie      = "Movie"
	itemTypeSeries     = "Series"
	itemTypeSeason     = "Season"
	itemTypeBoxSet     = "BoxSet"
	collectionTypeBox  = "boxsets"
	collectionTypeMovi = "movies"
	collectionTypeShow = "tvshows"
)

// Client talks to a Jellyfin server using an API key created in the dashboard.
type Client struct {
	baseURL    string
	apiKey     string
	httpClient *http.Client
}

// NewClient returns a Jellyfin client for the given base URL and API key.
func NewClient(baseURL, apiKey string) *Client {
	return &Client{
		baseURL: baseURL,
		apiKey:  apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

func (c *Client) Provider() string { return mediaserver.ProviderJellyfin }

func (c *Client) Name() string { return "Jellyfin" }

// GlobalCollections is true: Jellyfin stores box sets in a single top-level
// "Collections" folder rather than inside each movie library.
func (c *Client) GlobalCollections() bool { return true }

func (c *Client) Ping(ctx context.Context) error {
	var info struct {
		Version string `json:"Version"`
	}
	return c.get(ctx, "/System/Info", &info)
}

// virtualFolder is an entry of /Library/VirtualFolders.
type virtualFolder struct {
	Name           string `json:"Name"`
	ItemID         string `json:"ItemId"`
	CollectionType string `json:"CollectionType"`
}

// item is a Jellyfin item as returned by /Items.
type item struct {
	ID             string            `json:"Id"`
	Name           string            `json:"Name"`
	SeriesName     string            `json:"SeriesName"`
	ProductionYear int               `json:"ProductionYear"`
	PremiereDate   string            `json:"PremiereDate"`
	DateCreated    string            `json:"DateCreated"`
	IndexNumber    int               `json:"IndexNumber"`
	ImageTags      map[string]string `json:"ImageTags"`
}

type itemsResponse struct {
	Items []item `json:"Items"`
}

func (c *Client) Libraries(ctx context.Context) ([]mediaserver.Library, error) {
	var folders []virtualFolder
	if err := c.get(ctx, "/Library/VirtualFolders", &folders); err != nil {
		return nil, err
	}

	libraries := make([]mediaserver.Library, 0, len(folders))
	for _, f := range folders {
		var libType string
		switch strings.ToLower(f.CollectionType) {
		case collectionTypeMovi:
			libType = mediaserver.TypeMovie
		case collectionTypeShow:
			libType = mediaserver.TypeShow
		case collectionTypeBox:
			libType = mediaserver.TypeCollection
		default:
			continue
		}
		libraries = append(libraries, mediaserver.Library{Key: f.ItemID, Title: f.Name, Type: libType})
	}
	return libraries, nil
}

func (c *Client) Items(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
	var includeType string
	switch mediaType {
	case mediaserver.TypeMovie:
		includeType = itemTypeMovie
	case mediaserver.TypeShow:
		includeType = itemTypeSeries
	case mediaserver.TypeSeason:
		includeType = itemTypeSeason
	case mediaserver.TypeCollection:
		includeType = itemTypeBoxSet
	default:
		return nil, fmt.Errorf("jellyfin: unsupported media type %q", mediaType)
	}

	q := url.Values{}
	q.Set("ParentId", libraryKey)
	q.Set("IncludeItemTypes", includeType)
	q.Set("Recursive", "true")
	q.Set("Fields", "DateCreated,ProductionYear,PremiereDate")
	q.Set("EnableImageTypes", "Primary")

	var r itemsResponse
	if err := c.get(ctx, "/Items?"+q.Encode(), &r); err != nil {
		return nil, err
	}

	out := make([]mediaserver.Item, 0, len(r.Items))
	for _, i := range r.Items {
		converted := mediaserver.Item{
			ID:        i.ID,
			Title:     i.Name,
			Year:      i.year(),
			AddedAt:   parseTime(i.DateCreated),
			HasPoster: i.ImageTags["Primary"] != "",
		}
		// Season cards are labelled with the series title, matching Plex.
		if mediaType == mediaserver.TypeSeason {
			converted.SeasonNumber = i.IndexNumber
			if i.SeriesName != "" {
				converted.Title = i.SeriesName
			}
		}
		out = append(out, converted)
	}
	return out, nil
}

// year prefers ProductionYear and falls back to the year of PremiereDate, which
// is the only date Jellyfin sets on many seasons.
func (i item) year() int {
	if i.ProductionYear != 0 {
		return i.ProductionYear
	}
	if t := parseTime(i.PremiereDate); t != 0 {
		return time.Unix(t, 0).UTC().Year()
	}
	return 0
}

// parseTime converts a Jellyfin ISO-8601 timestamp to a Unix timestamp,
// returning 0 when the field is absent or malformed.
func parseTime(v string) int64 {
	if v == "" {
		return 0
	}
	t, err := time.Parse(time.RFC3339, v)
	if err != nil {
		return 0
	}
	return t.Unix()
}

// DownloadPoster fetches the primary image currently set in Jellyfin.
func (c *Client) DownloadPoster(ctx context.Context, itemID string) ([]byte, string, error) {
	req, err := c.newRequest(ctx, http.MethodGet, "/Items/"+itemID+"/Images/Primary", nil)
	if err != nil {
		return nil, "", err
	}

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

// UploadPoster replaces the primary image of an item. Jellyfin expects the
// image body to be base64-encoded text rather than raw bytes.
func (c *Client) UploadPoster(ctx context.Context, itemID string, data []byte, contentType string) error {
	encoded := base64.StdEncoding.EncodeToString(data)

	req, err := c.newRequest(ctx, http.MethodPost, "/Items/"+itemID+"/Images/Primary", strings.NewReader(encoded))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", contentType)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	return statusError(resp.StatusCode, "poster upload for "+itemID)
}

func (c *Client) get(ctx context.Context, path string, out any) error {
	req, err := c.newRequest(ctx, http.MethodGet, path, nil)
	if err != nil {
		return err
	}
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

func (c *Client) newRequest(ctx context.Context, method, path string, body io.Reader) (*http.Request, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, body)
	if err != nil {
		return nil, err
	}
	// Jellyfin reads the API key from either header; sending both keeps older
	// and newer server versions happy.
	req.Header.Set("Authorization", `MediaBrowser Client="Postr", Device="Postr", DeviceId="postr", Version="1.0.0", Token="`+c.apiKey+`"`)
	req.Header.Set("X-Emby-Token", c.apiKey)
	return req, nil
}

// statusError maps an HTTP status to the provider-neutral sentinel errors.
func statusError(status int, what string) error {
	switch {
	case status == http.StatusUnauthorized || status == http.StatusForbidden:
		return mediaserver.ErrUnauthorized
	case status == http.StatusNotFound:
		return mediaserver.ErrNotFound
	case status < 200 || status >= 300:
		return fmt.Errorf("jellyfin returned %d for %s", status, what)
	}
	return nil
}
