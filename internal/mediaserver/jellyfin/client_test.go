package jellyfin_test

import (
	"context"
	"encoding/base64"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/florentsorel/postr/internal/mediaserver/jellyfin"
)

const apiKey = "key123"

// newServer starts a fake Jellyfin server that enforces API-key auth and routes
// exact paths to JSON bodies.
func newServer(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.Header.Get("Authorization"), `Token="`+apiKey+`"`) {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		body, ok := routes[r.URL.Path]
		if !ok {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(body))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestLibraries_MapsCollectionTypes(t *testing.T) {
	srv := newServer(t, map[string]string{
		"/Library/VirtualFolders": `[
			{"Name":"Movies","ItemId":"lib-movies","CollectionType":"movies"},
			{"Name":"Shows","ItemId":"lib-shows","CollectionType":"tvshows"},
			{"Name":"Collections","ItemId":"lib-box","CollectionType":"boxsets"},
			{"Name":"Music","ItemId":"lib-music","CollectionType":"music"}
		]`,
	})

	libs, err := jellyfin.NewClient(srv.URL, apiKey).Libraries(context.Background())
	if err != nil {
		t.Fatalf("Libraries: %v", err)
	}

	want := []mediaserver.Library{
		{Key: "lib-movies", Title: "Movies", Type: mediaserver.TypeMovie},
		{Key: "lib-shows", Title: "Shows", Type: mediaserver.TypeShow},
		{Key: "lib-box", Title: "Collections", Type: mediaserver.TypeCollection},
	}
	if len(libs) != len(want) {
		t.Fatalf("want %d libraries (music dropped), got %d: %+v", len(want), len(libs), libs)
	}
	for i := range want {
		if libs[i] != want[i] {
			t.Errorf("library %d:\n want %+v\n  got %+v", i, want[i], libs[i])
		}
	}
}

func TestItems_MoviesMapIDTitleYearAndPoster(t *testing.T) {
	var gotQuery url.Values
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotQuery = r.URL.Query()
		_, _ = w.Write([]byte(`{"Items":[
			{"Id":"abc","Name":"Inception","ProductionYear":2010,"DateCreated":"2023-05-01T12:00:00.0000000Z","ImageTags":{"Primary":"hash"}},
			{"Id":"def","Name":"No Poster","ProductionYear":1999,"ImageTags":{}}
		]}`))
	}))
	defer srv.Close()

	items, err := jellyfin.NewClient(srv.URL, apiKey).Items(context.Background(), "lib-movies", mediaserver.TypeMovie)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}

	if got := gotQuery.Get("IncludeItemTypes"); got != "Movie" {
		t.Errorf("IncludeItemTypes: want Movie, got %q", got)
	}
	if got := gotQuery.Get("ParentId"); got != "lib-movies" {
		t.Errorf("ParentId: want lib-movies, got %q", got)
	}
	if got := gotQuery.Get("Recursive"); got != "true" {
		t.Errorf("Recursive: want true, got %q", got)
	}

	if items[0].ID != "abc" || items[0].Title != "Inception" || items[0].Year != 2010 {
		t.Errorf("item 0: got %+v", items[0])
	}
	if !items[0].HasPoster {
		t.Error("item 0: want HasPoster=true when ImageTags.Primary is set")
	}
	if items[0].AddedAt == 0 {
		t.Error("item 0: want DateCreated parsed into AddedAt")
	}
	if items[1].HasPoster {
		t.Error("item 1: want HasPoster=false when ImageTags is empty")
	}
}

func TestItems_SeasonsUseSeriesNameAndIndex(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`{"Items":[
			{"Id":"s1","Name":"Season 1","SeriesName":"Breaking Bad","IndexNumber":1,"PremiereDate":"2008-01-20T00:00:00.0000000Z","ImageTags":{"Primary":"h"}}
		]}`))
	}))
	defer srv.Close()

	items, err := jellyfin.NewClient(srv.URL, apiKey).Items(context.Background(), "lib", mediaserver.TypeSeason)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	want := mediaserver.Item{ID: "s1", Title: "Breaking Bad", Year: 2008, SeasonNumber: 1, HasPoster: true}
	if items[0] != want {
		t.Errorf("season:\n want %+v\n  got %+v", want, items[0])
	}
}

// TestUploadPoster_SendsBase64 pins the Jellyfin quirk that image uploads carry
// a base64-encoded body rather than raw bytes.
func TestUploadPoster_SendsBase64(t *testing.T) {
	var gotPath, gotContentType string
	var gotBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(http.StatusNoContent)
	}))
	defer srv.Close()

	raw := []byte("poster-bytes")
	if err := jellyfin.NewClient(srv.URL, apiKey).UploadPoster(context.Background(), "abc", raw, "image/jpeg"); err != nil {
		t.Fatalf("UploadPoster: %v", err)
	}
	if gotPath != "/Items/abc/Images/Primary" {
		t.Errorf("path: got %q", gotPath)
	}
	if gotContentType != "image/jpeg" {
		t.Errorf("content-type: got %q", gotContentType)
	}
	decoded, err := base64.StdEncoding.DecodeString(string(gotBody))
	if err != nil {
		t.Fatalf("body is not base64: %v (%q)", err, gotBody)
	}
	if string(decoded) != string(raw) {
		t.Errorf("decoded body: want %q, got %q", raw, decoded)
	}
}

func TestDownloadPoster(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/Items/abc/Images/Primary" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "image/webp")
		_, _ = w.Write([]byte("webp-bytes"))
	}))
	defer srv.Close()
	c := jellyfin.NewClient(srv.URL, apiKey)

	data, ext, err := c.DownloadPoster(context.Background(), "abc")
	if err != nil {
		t.Fatalf("DownloadPoster: %v", err)
	}
	if string(data) != "webp-bytes" || ext != "webp" {
		t.Errorf("want webp-bytes/webp, got %q/%q", data, ext)
	}

	if _, _, err := c.DownloadPoster(context.Background(), "gone"); !errors.Is(err, mediaserver.ErrNotFound) {
		t.Errorf("missing item: want ErrNotFound, got %v", err)
	}
}

func TestPing_InvalidAPIKeyIsUnauthorized(t *testing.T) {
	srv := newServer(t, map[string]string{"/System/Info": `{"Version":"10.10.0"}`})

	if err := jellyfin.NewClient(srv.URL, "wrong").Ping(context.Background()); !errors.Is(err, mediaserver.ErrUnauthorized) {
		t.Errorf("want ErrUnauthorized, got %v", err)
	}
	if err := jellyfin.NewClient(srv.URL, apiKey).Ping(context.Background()); err != nil {
		t.Errorf("valid key: want nil, got %v", err)
	}
}

func TestGlobalCollectionsIsTrue(t *testing.T) {
	if !jellyfin.NewClient("http://x", apiKey).GlobalCollections() {
		t.Error("Jellyfin box sets live in their own folder; want true")
	}
}
