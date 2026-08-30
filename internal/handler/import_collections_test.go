package handler_test

import (
	"context"
	"testing"

	"github.com/florentsorel/postr/internal/mediaserver"
)

const importCollectionsBody = `{"targets":[{"type":"collection","sectionKeys":["lib1","lib2"]}]}`

// collectionMock serves two movie libraries plus, optionally, a server-wide
// collection library. Every library returns the same two collections, which is
// what a Jellyfin server does when queried per movie library.
func collectionMock(global bool) (*mockServer, *[]string) {
	var queried []string
	libs := []mediaserver.Library{
		{Key: "lib1", Type: mediaserver.TypeMovie, Title: "Movies"},
		{Key: "lib2", Type: mediaserver.TypeMovie, Title: "4K Movies"},
	}
	if global {
		libs = append(libs, mediaserver.Library{Key: "lib-box", Type: mediaserver.TypeCollection, Title: "Collections"})
	}

	return &mockServer{
		globalCollections: global,
		librariesFunc: func(ctx context.Context) ([]mediaserver.Library, error) {
			return libs, nil
		},
		itemsFunc: func(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
			queried = append(queried, libraryKey)
			return []mediaserver.Item{
				{ID: "c1", Title: "Marvel", HasPoster: true},
				{ID: "c2", Title: "Star Wars", HasPoster: true},
			}, nil
		},
	}, &queried
}

// TestImport_GlobalCollectionsImportedOnce covers Jellyfin: box sets live in one
// server-wide folder, so importing them once per selected movie library would
// import the same collections twice and orphan them on the next run.
func TestImport_GlobalCollectionsImportedOnce(t *testing.T) {
	mock, queried := collectionMock(true)
	setup := newTestSetup(t, mock)

	result := runImport(t, setup.handler, importCollectionsBody)

	if len(*queried) != 1 || (*queried)[0] != "lib-box" {
		t.Errorf("libraries queried: want [lib-box] once, got %v", *queried)
	}
	if result.Added != 2 {
		t.Errorf("Added: want 2, got %d", result.Added)
	}

	// A second run must be a clean no-op, not a wave of orphans.
	second := runImport(t, setup.handler, importCollectionsBody)
	if second.Added != 0 || second.Deleted != 0 {
		t.Errorf("second import: want Added=0 Deleted=0, got Added=%d Deleted=%d", second.Added, second.Deleted)
	}
}

// TestImport_ScopedCollectionsQueryEachLibrary covers Plex, where collections
// belong to the section that holds them.
func TestImport_ScopedCollectionsQueryEachLibrary(t *testing.T) {
	mock, queried := collectionMock(false)
	setup := newTestSetup(t, mock)

	runImport(t, setup.handler, importCollectionsBody)

	if len(*queried) != 2 {
		t.Errorf("libraries queried: want lib1 and lib2, got %v", *queried)
	}
}

// TestImport_GlobalCollectionsSkippedWhenServerHasNone verifies a server with no
// collection folder simply skips the target instead of failing the import.
func TestImport_GlobalCollectionsSkippedWhenServerHasNone(t *testing.T) {
	mock := &mockServer{
		globalCollections: true,
		librariesFunc: func(ctx context.Context) ([]mediaserver.Library, error) {
			return []mediaserver.Library{{Key: "lib1", Type: mediaserver.TypeMovie, Title: "Movies"}}, nil
		},
		itemsFunc: func(ctx context.Context, libraryKey, mediaType string) ([]mediaserver.Item, error) {
			t.Errorf("Items must not be called when the server has no collection library (got %q)", libraryKey)
			return nil, nil
		},
	}
	setup := newTestSetup(t, mock)

	result := runImport(t, setup.handler, importCollectionsBody)
	if result.Added != 0 {
		t.Errorf("Added: want 0, got %d", result.Added)
	}
}
