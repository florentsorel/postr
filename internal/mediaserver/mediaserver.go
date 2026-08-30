// Package mediaserver defines the provider-neutral contract Postr uses to talk
// to a media server. Each supported server (Plex, Jellyfin) implements Client
// in its own subpackage; handlers only ever see the types declared here.
package mediaserver

import (
	"context"
	"errors"
	"strings"
)

// Provider identifiers, also stored in the `provider` column of the database so
// data imported from one server is never mixed with another's.
const (
	ProviderPlex     = "plex"
	ProviderJellyfin = "jellyfin"
)

// Media types Postr can import. They double as the values of `media.type`.
const (
	TypeMovie      = "movie"
	TypeShow       = "show"
	TypeSeason     = "season"
	TypeCollection = "collection"
)

var (
	// ErrUnauthorized means the server rejected the configured credentials.
	ErrUnauthorized = errors.New("invalid media server credentials")
	// ErrNotFound means the item no longer exists on the server (HTTP 404).
	// Only this error turns an item into an orphan.
	ErrNotFound = errors.New("media not found on server")
)

// Library is a top-level container on the server: a Plex section or a Jellyfin
// virtual folder. Type is one of TypeMovie, TypeShow or TypeCollection.
type Library struct {
	Key   string
	Title string
	Type  string
}

// Item is a single importable entry. ID is the server's own identifier — the
// Plex ratingKey or the Jellyfin item GUID — and is used as the poster filename
// on disk.
type Item struct {
	ID string
	// Title of the item. For seasons this is the series title, matching how
	// Postr labels season cards.
	Title string
	Year  int
	// SeasonNumber is only set for TypeSeason items.
	SeasonNumber int
	// AddedAt is a Unix timestamp, 0 when the server does not report one.
	AddedAt int64
	// HasPoster reports whether the server currently has a poster for the item;
	// when false there is nothing to download.
	HasPoster bool
}

// Client is the set of operations Postr needs from a media server.
type Client interface {
	// Provider returns the machine identifier: ProviderPlex or ProviderJellyfin.
	Provider() string
	// Name returns the display name shown in the UI, e.g. "Jellyfin".
	Name() string
	// Ping verifies the server is reachable and the credentials are valid.
	// It returns ErrUnauthorized when the token or API key is rejected.
	Ping(ctx context.Context) error
	// Libraries returns the movie and show libraries on the server. Servers with
	// GlobalCollections also return their collection library.
	Libraries(ctx context.Context) ([]Library, error)
	// Items returns every item of the given media type in a library.
	Items(ctx context.Context, libraryKey, mediaType string) ([]Item, error)
	// DownloadPoster fetches the poster currently set on the server, returning
	// the raw bytes and the file extension (jpg, png, webp).
	DownloadPoster(ctx context.Context, itemID string) ([]byte, string, error)
	// UploadPoster replaces the poster of an item on the server.
	UploadPoster(ctx context.Context, itemID string, data []byte, contentType string) error
	// GlobalCollections reports whether collections live in a single server-wide
	// library (Jellyfin box sets) rather than inside each movie library (Plex).
	// When true, Postr imports collections once instead of once per library.
	GlobalCollections() bool
}

// ExtFromContentType maps a Content-Type header value to a file extension.
func ExtFromContentType(ct string) string {
	ct = strings.ToLower(strings.TrimSpace(ct))
	if idx := strings.Index(ct, ";"); idx >= 0 {
		ct = strings.TrimSpace(ct[:idx])
	}
	switch ct {
	case "image/png":
		return "png"
	case "image/webp":
		return "webp"
	default:
		return "jpg"
	}
}

// ContentTypeFromExt maps a file extension to a MIME content type.
func ContentTypeFromExt(ext string) string {
	switch strings.ToLower(ext) {
	case "png":
		return "image/png"
	case "webp":
		return "image/webp"
	default:
		return "image/jpeg"
	}
}
