package handler_test

import (
	"context"
	"testing"

	"github.com/florentsorel/postr/internal/plex"
)

const importSeasonBody = `{"targets":[{"type":"season","sectionKeys":["1"]}]}`

// seasonMock returns a mock where:
//   - section "1" is a show library
//   - one show "Breaking Bad" (ratingKey "10", year 2008)
//   - two seasons: S1 (ratingKey "201") and S2 (ratingKey "202")
//   - each season has one episode with originallyAvailableAt set
func seasonMock() *mockPlex {
	return &mockPlex{
		sectionsFunc: func(ctx context.Context) ([]plex.Section, error) {
			return []plex.Section{{Key: "1", Type: "show", Title: "Shows"}}, nil
		},
		allItemsFunc: func(ctx context.Context, sectionKey string) ([]plex.Item, error) {
			return []plex.Item{
				{RatingKey: "10", Title: "Breaking Bad", Year: 2008, Thumb: "/thumb/10"},
			}, nil
		},
		childrenFunc: func(ctx context.Context, ratingKey string) ([]plex.Item, error) {
			switch ratingKey {
			case "10": // show → seasons
				return []plex.Item{
					{RatingKey: "201", Title: "Season 1", Index: 1, Thumb: "/thumb/201"},
					{RatingKey: "202", Title: "Season 2", Index: 2, Thumb: "/thumb/202"},
				}, nil
			case "201": // season 1 → episodes
				return []plex.Item{
					{RatingKey: "301", OriginallyAvailableAt: "2008-01-20"},
				}, nil
			case "202": // season 2 → episodes
				return []plex.Item{
					{RatingKey: "401", OriginallyAvailableAt: "2009-03-08"},
				}, nil
			}
			return nil, nil
		},
	}
}

func TestImport_Season_TitleIsShowName(t *testing.T) {
	setup := newTestSetup(t, seasonMock())
	result := runImport(t, setup.handler, importSeasonBody)

	if result.Added != 2 {
		t.Fatalf("Added: want 2, got %d", result.Added)
	}

	for _, ratingKey := range []string{"201", "202"} {
		m, err := setup.queries.GetMediaByRatingKey(context.Background(), ratingKey)
		if err != nil {
			t.Fatalf("GetMediaByRatingKey(%q): %v", ratingKey, err)
		}
		if m.Title != "Breaking Bad" {
			t.Errorf("ratingKey %q: title want %q, got %q", ratingKey, "Breaking Bad", m.Title)
		}
	}
}

func TestImport_Season_SeasonNumberStored(t *testing.T) {
	setup := newTestSetup(t, seasonMock())
	runImport(t, setup.handler, importSeasonBody)

	cases := []struct {
		ratingKey string
		wantN     int64
	}{
		{"201", 1},
		{"202", 2},
	}
	for _, tc := range cases {
		m, err := setup.queries.GetMediaByRatingKey(context.Background(), tc.ratingKey)
		if err != nil {
			t.Fatalf("GetMediaByRatingKey(%q): %v", tc.ratingKey, err)
		}
		if !m.SeasonNumber.Valid || m.SeasonNumber.Int64 != tc.wantN {
			t.Errorf("ratingKey %q: season_number want %d, got %v", tc.ratingKey, tc.wantN, m.SeasonNumber)
		}
	}
}

func TestImport_Season_YearFromFirstEpisode(t *testing.T) {
	setup := newTestSetup(t, seasonMock())
	runImport(t, setup.handler, importSeasonBody)

	cases := []struct {
		ratingKey string
		wantYear  int64
	}{
		{"201", 2008},
		{"202", 2009},
	}
	for _, tc := range cases {
		m, err := setup.queries.GetMediaByRatingKey(context.Background(), tc.ratingKey)
		if err != nil {
			t.Fatalf("GetMediaByRatingKey(%q): %v", tc.ratingKey, err)
		}
		if !m.Year.Valid || m.Year.Int64 != tc.wantYear {
			t.Errorf("ratingKey %q: year want %d, got %v", tc.ratingKey, tc.wantYear, m.Year)
		}
	}
}

func TestImport_Season_YearFallbackToShowWhenNoEpisodes(t *testing.T) {
	mock := seasonMock()
	// Season 202 has no episodes yet.
	originalChildren := mock.childrenFunc
	mock.childrenFunc = func(ctx context.Context, ratingKey string) ([]plex.Item, error) {
		if ratingKey == "202" {
			return nil, nil
		}
		return originalChildren(ctx, ratingKey)
	}

	setup := newTestSetup(t, mock)
	runImport(t, setup.handler, importSeasonBody)

	m, err := setup.queries.GetMediaByRatingKey(context.Background(), "202")
	if err != nil {
		t.Fatalf("GetMediaByRatingKey: %v", err)
	}
	if !m.Year.Valid || m.Year.Int64 != 2008 {
		t.Errorf("year: want 2008 (show fallback), got %v", m.Year)
	}
}
