package plex_test

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"reflect"
	"testing"

	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/florentsorel/postr/internal/mediaserver/plex"
)

// newServer starts a fake Plex server routing exact paths to JSON bodies.
func newServer(t *testing.T, routes map[string]string) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Plex-Token") != "tok" {
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

func TestLibraries_KeepsOnlyMovieAndShow(t *testing.T) {
	srv := newServer(t, map[string]string{
		"/library/sections": `{"MediaContainer":{"Directory":[
			{"key":"1","type":"movie","title":"Movies"},
			{"key":"2","type":"show","title":"TV"},
			{"key":"3","type":"artist","title":"Music"}
		]}}`,
	})

	libs, err := plex.NewClient(srv.URL, "tok").Libraries(context.Background())
	if err != nil {
		t.Fatalf("Libraries: %v", err)
	}
	if len(libs) != 2 {
		t.Fatalf("want 2 libraries, got %d: %+v", len(libs), libs)
	}
	if libs[0].Type != mediaserver.TypeMovie || libs[1].Type != mediaserver.TypeShow {
		t.Errorf("types: got %q and %q", libs[0].Type, libs[1].Type)
	}
}

// seasonRoutes describes one show ("Breaking Bad", 2008) with two seasons whose
// first episodes air in 2008 and 2009.
func seasonRoutes() map[string]string {
	return map[string]string{
		"/library/sections": `{"MediaContainer":{"Directory":[{"key":"1","type":"show","title":"TV"}]}}`,
		"/library/sections/1/all": `{"MediaContainer":{"Metadata":[
			{"ratingKey":"10","guid":"plex://show/5d9c081","title":"Breaking Bad","year":2008,"thumb":"/thumb/10",
			 "Guid":[{"id":"imdb://tt0903747"},{"id":"tvdb://81189"}]}
		]}}`,
		"/library/metadata/10/children": `{"MediaContainer":{"Metadata":[
			{"ratingKey":"201","guid":"plex://season/5d9c082","title":"Season 1","index":1,"thumb":"/thumb/201"},
			{"ratingKey":"202","guid":"plex://season/5d9c083","title":"Season 2","index":2,"thumb":"/thumb/202"}
		]}}`,
		"/library/metadata/201/children": `{"MediaContainer":{"Metadata":[{"ratingKey":"301","originallyAvailableAt":"2008-01-20"}]}}`,
		"/library/metadata/202/children": `{"MediaContainer":{"Metadata":[{"ratingKey":"401","originallyAvailableAt":"2009-03-08"}]}}`,
	}
}

func TestItems_SeasonsCarryShowTitleNumberAndAirYear(t *testing.T) {
	srv := newServer(t, seasonRoutes())

	items, err := plex.NewClient(srv.URL, "tok").Items(context.Background(), "1", mediaserver.TypeSeason)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if len(items) != 2 {
		t.Fatalf("want 2 seasons, got %d", len(items))
	}

	showIDs := map[string]string{"imdb": "tt0903747", "tvdb": "81189"}
	want := []mediaserver.Item{
		{ID: "201", Title: "Breaking Bad", Year: 2008, SeasonNumber: 1, HasPoster: true, ExternalIDs: showIDs},
		{ID: "202", Title: "Breaking Bad", Year: 2009, SeasonNumber: 2, HasPoster: true, ExternalIDs: showIDs},
	}
	for i, w := range want {
		if !reflect.DeepEqual(items[i], w) {
			t.Errorf("season %d:\n want %+v\n  got %+v", i, w, items[i])
		}
	}
}

func TestItems_SeasonYearFallsBackToShowWhenNoEpisodes(t *testing.T) {
	routes := seasonRoutes()
	// Season 2 has not aired yet.
	routes["/library/metadata/202/children"] = `{"MediaContainer":{"Metadata":[]}}`
	srv := newServer(t, routes)

	items, err := plex.NewClient(srv.URL, "tok").Items(context.Background(), "1", mediaserver.TypeSeason)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if items[1].Year != 2008 {
		t.Errorf("year: want 2008 (show fallback), got %d", items[1].Year)
	}
}

func TestItems_MoviesMapHasPosterFromThumb(t *testing.T) {
	srv := newServer(t, map[string]string{
		"/library/sections/1/all": `{"MediaContainer":{"Metadata":[
			{"ratingKey":"101","guid":"plex://movie/5d776831","title":"Inception","year":2010,"addedAt":1600000000,"thumb":"/thumb/101",
			 "Guid":[{"id":"imdb://tt1375666"},{"id":"tmdb://27205"}]},
			{"ratingKey":"102","guid":"plex://movie/5d776832","title":"No Poster","year":1999}
		]}}`,
	})

	items, err := plex.NewClient(srv.URL, "tok").Items(context.Background(), "1", mediaserver.TypeMovie)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if !items[0].HasPoster {
		t.Error("item 101: want HasPoster=true")
	}
	if items[0].AddedAt != 1600000000 {
		t.Errorf("item 101 addedAt: want 1600000000, got %d", items[0].AddedAt)
	}
	if items[1].HasPoster {
		t.Error("item 102: want HasPoster=false when thumb is absent")
	}
}

func TestDownloadPoster(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/library/metadata/101/thumb":
			w.Header().Set("Content-Type", "image/png")
			_, _ = w.Write([]byte("png-bytes"))
		default:
			w.WriteHeader(http.StatusNotFound)
		}
	}))
	defer srv.Close()
	c := plex.NewClient(srv.URL, "tok")

	data, ext, err := c.DownloadPoster(context.Background(), "101")
	if err != nil {
		t.Fatalf("DownloadPoster: %v", err)
	}
	if string(data) != "png-bytes" || ext != "png" {
		t.Errorf("want png-bytes/png, got %q/%q", data, ext)
	}

	if _, _, err := c.DownloadPoster(context.Background(), "999"); !errors.Is(err, mediaserver.ErrNotFound) {
		t.Errorf("missing item: want ErrNotFound, got %v", err)
	}
}

func TestUploadPoster_SendsRawBytes(t *testing.T) {
	var gotBody []byte
	var gotPath, gotContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotContentType = r.Header.Get("Content-Type")
		gotBody = make([]byte, r.ContentLength)
		_, _ = r.Body.Read(gotBody)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	if err := plex.NewClient(srv.URL, "tok").UploadPoster(context.Background(), "101", []byte("raw"), "image/jpeg"); err != nil {
		t.Fatalf("UploadPoster: %v", err)
	}
	if gotPath != "/library/metadata/101/posters" {
		t.Errorf("path: got %q", gotPath)
	}
	if gotContentType != "image/jpeg" {
		t.Errorf("content-type: got %q", gotContentType)
	}
	if string(gotBody) != "raw" {
		t.Errorf("body: want raw bytes, got %q", gotBody)
	}
}

func TestPing_InvalidTokenIsUnauthorized(t *testing.T) {
	srv := newServer(t, map[string]string{"/library/sections": `{"MediaContainer":{"Directory":[]}}`})

	if err := plex.NewClient(srv.URL, "wrong").Ping(context.Background()); !errors.Is(err, mediaserver.ErrUnauthorized) {
		t.Errorf("want ErrUnauthorized, got %v", err)
	}
	if err := plex.NewClient(srv.URL, "tok").Ping(context.Background()); err != nil {
		t.Errorf("valid token: want nil, got %v", err)
	}
}

func TestGlobalCollectionsIsFalse(t *testing.T) {
	if plex.NewClient("http://x", "tok").GlobalCollections() {
		t.Error("Plex collections live inside sections; want false")
	}
}

// TestItems_ParsesExternalGuids covers the payload Plex really sends: a
// lowercase "guid" string next to the capitalised "Guid" array. Because
// encoding/json matches keys case-insensitively as a fallback, a struct that
// only declares "Guid" fails the whole decode on the string — which is exactly
// how the migration broke.
func TestItems_ParsesExternalGuids(t *testing.T) {
	srv := newServer(t, map[string]string{
		"/library/sections/1/all": `{"MediaContainer":{"Metadata":[
			{"ratingKey":"101","guid":"plex://movie/5d776831","title":"Inception","year":2010,"thumb":"/t",
			 "Guid":[{"id":"imdb://tt1375666"},{"id":"tmdb://27205"}]}
		]}}`,
	})

	items, err := plex.NewClient(srv.URL, "tok").Items(context.Background(), "1", mediaserver.TypeMovie)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}

	want := map[string]string{"imdb": "tt1375666", "tmdb": "27205"}
	if !reflect.DeepEqual(items[0].ExternalIDs, want) {
		t.Errorf("ExternalIDs = %v, want %v", items[0].ExternalIDs, want)
	}
	// Plex's own reference means nothing to another server and must be dropped.
	if _, ok := items[0].ExternalIDs["plex"]; ok {
		t.Error("the plex:// reference should not be kept as an external id")
	}
	// Both sources must be offered, so the item can meet a counterpart that
	// only knows one of them.
	wantKeys := []string{"movie|tmdb:27205", "movie|imdb:tt1375666"}
	if got := items[0].MatchKeys(mediaserver.TypeMovie); !reflect.DeepEqual(got, wantKeys) {
		t.Errorf("MatchKeys = %v, want %v", got, wantKeys)
	}
}

// TestItems_NoGuidsIsNotAnError covers a server that answers without the array
// at all — matching then falls back to titles rather than failing.
func TestItems_NoGuidsIsNotAnError(t *testing.T) {
	srv := newServer(t, map[string]string{
		"/library/sections/1/all": `{"MediaContainer":{"Metadata":[
			{"ratingKey":"101","guid":"plex://movie/5d776831","title":"Inception","year":2010,"thumb":"/t"}
		]}}`,
	})

	items, err := plex.NewClient(srv.URL, "tok").Items(context.Background(), "1", mediaserver.TypeMovie)
	if err != nil {
		t.Fatalf("Items: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("want 1 item, got %d", len(items))
	}
	if items[0].ExternalIDs != nil {
		t.Errorf("ExternalIDs = %v, want nil", items[0].ExternalIDs)
	}
	if items[0].Title != "Inception" {
		t.Errorf("the item should still decode: %+v", items[0])
	}
}
