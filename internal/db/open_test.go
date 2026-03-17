package db_test

import (
	"context"
	"testing"

	"github.com/florentsorel/postr/internal/db"
)

func TestOpen_RunsMigrations(t *testing.T) {
	conn, err := db.Open(":memory:")
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	defer conn.Close()

	q := db.New(conn)
	settings, err := q.ListSettings(context.Background())
	if err != nil {
		t.Fatalf("ListSettings: %v", err)
	}

	want := map[string]string{
		"tmdb":         "false",
		"tvdb":         "false",
		"fanart":       "false",
		"auto_resize":  "true",
		"resize_width": "1000",
	}

	if len(settings) != len(want) {
		t.Fatalf("got %d settings, want %d", len(settings), len(want))
	}

	for _, s := range settings {
		expected, ok := want[s.Key]
		if !ok {
			t.Errorf("unexpected setting key %q", s.Key)
			continue
		}
		if !s.Value.Valid || s.Value.String != expected {
			t.Errorf("setting %q: got value %v, want %q", s.Key, s.Value, expected)
		}
	}
}
