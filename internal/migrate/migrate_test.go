package migrate_test

import (
	"testing"

	"github.com/florentsorel/postr/internal/mediaserver"
	"github.com/florentsorel/postr/internal/migrate"
)

func ids(m map[string]string) map[string]string { return m }

func TestBuildPlan_ExternalIDWinsOverDivergentTitles(t *testing.T) {
	// The exact case in the user's library: Plex stores "Invincible (2021)"
	// where Jellyfin stores "Invincible".
	source := []mediaserver.Item{
		{ID: "10", Title: "Invincible (2021)", Year: 2021, ExternalIDs: ids(map[string]string{"tmdb": "95557"})},
	}
	target := []mediaserver.Item{
		{ID: "guid-a", Title: "Invincible", Year: 2021, ExternalIDs: ids(map[string]string{"tmdb": "95557"})},
	}

	plan := migrate.BuildPlan(mediaserver.TypeShow, source, target)

	if len(plan.Matches) != 1 {
		t.Fatalf("matches: want 1, got %d (%+v)", len(plan.Matches), plan.Unmatched)
	}
	if plan.Matches[0].TargetID != "guid-a" || plan.Matches[0].By != migrate.ByExternalID {
		t.Errorf("match: got %+v", plan.Matches[0])
	}
}

// TestBuildPlan_MatchesOnAnySharedSource is the case the old test only claimed
// to cover: the two servers store different sets of identifiers and overlap on
// just one. Matching on a single preferred source would miss this pair.
func TestBuildPlan_MatchesOnAnySharedSource(t *testing.T) {
	cases := []struct {
		name           string
		source, target map[string]string
		wantMatch      bool
	}{
		{
			name:   "plex knows imdb, jellyfin knows tmdb and imdb",
			source: map[string]string{"imdb": "tt1375666"},
			target: map[string]string{"tmdb": "27205", "imdb": "tt1375666"},
			// tmdb is tried first on the target side; the shared source is imdb.
			wantMatch: true,
		},
		{
			name:      "plex knows tvdb, jellyfin knows tmdb and tvdb",
			source:    map[string]string{"tvdb": "81189"},
			target:    map[string]string{"tmdb": "1396", "tvdb": "81189"},
			wantMatch: true,
		},
		{
			name:      "no source in common",
			source:    map[string]string{"imdb": "tt1375666"},
			target:    map[string]string{"tmdb": "27205"},
			wantMatch: false,
		},
		{
			name:      "same source, different ids",
			source:    map[string]string{"tmdb": "27205"},
			target:    map[string]string{"tmdb": "99999"},
			wantMatch: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			// Titles differ so only identifiers can pair them.
			source := []mediaserver.Item{{ID: "10", Title: "Inception", Year: 2010, ExternalIDs: tc.source}}
			target := []mediaserver.Item{{ID: "g1", Title: "Origine", Year: 2010, ExternalIDs: tc.target}}

			plan := migrate.BuildPlan(mediaserver.TypeMovie, source, target)

			if tc.wantMatch {
				if len(plan.Matches) != 1 || plan.Matches[0].By != migrate.ByExternalID {
					t.Fatalf("want one external-id match, got %+v / %+v", plan.Matches, plan.Unmatched)
				}
				return
			}
			if len(plan.Matches) != 0 {
				t.Fatalf("want no match, got %+v", plan.Matches)
			}
		})
	}
}

func TestBuildPlan_TitleFallbackForCollections(t *testing.T) {
	// No external database tracks collections, so title is all there is.
	source := []mediaserver.Item{{ID: "9", Title: "MCU"}}
	target := []mediaserver.Item{{ID: "box-1", Title: "mcu"}}

	plan := migrate.BuildPlan(mediaserver.TypeCollection, source, target)

	if len(plan.Matches) != 1 {
		t.Fatalf("matches: want 1, got %d (%+v)", len(plan.Matches), plan.Unmatched)
	}
	if plan.Matches[0].By != migrate.ByTitle {
		t.Errorf("By: want %q, got %q", migrate.ByTitle, plan.Matches[0].By)
	}
}

func TestBuildPlan_YearDisambiguatesTitleFallback(t *testing.T) {
	source := []mediaserver.Item{
		{ID: "1", Title: "Dune", Year: 1984},
		{ID: "2", Title: "Dune", Year: 2021},
	}
	target := []mediaserver.Item{
		{ID: "g1", Title: "Dune", Year: 1984},
		{ID: "g2", Title: "Dune", Year: 2021},
	}

	plan := migrate.BuildPlan(mediaserver.TypeMovie, source, target)

	if len(plan.Matches) != 2 {
		t.Fatalf("matches: want 2, got %d (%+v)", len(plan.Matches), plan.Unmatched)
	}
	got := map[string]string{}
	for _, m := range plan.Matches {
		got[m.SourceID] = m.TargetID
	}
	if got["1"] != "g1" || got["2"] != "g2" {
		t.Errorf("wrong pairing: %v", got)
	}
}

// TestBuildPlan_AmbiguousTitlesAreRefused is the safety property that matters
// most: a wrong pair silently overwrites artwork on the target server.
func TestBuildPlan_AmbiguousTitlesAreRefused(t *testing.T) {
	source := []mediaserver.Item{{ID: "1", Title: "Obsession"}}
	target := []mediaserver.Item{
		{ID: "g1", Title: "Obsession"},
		{ID: "g2", Title: "obsession"},
	}

	plan := migrate.BuildPlan(mediaserver.TypeMovie, source, target)

	if len(plan.Matches) != 0 {
		t.Fatalf("want no match on an ambiguous title, got %+v", plan.Matches)
	}
	if len(plan.Unmatched) != 1 || plan.Unmatched[0].Reason != migrate.ReasonAmbiguous {
		t.Errorf("want one ambiguous report, got %+v", plan.Unmatched)
	}
}

func TestBuildPlan_DuplicateSourceTitlesAreRefused(t *testing.T) {
	// Two local items answer to the same title: neither can claim the target.
	source := []mediaserver.Item{
		{ID: "1", Title: "Obsession"},
		{ID: "2", Title: "Obsession"},
	}
	target := []mediaserver.Item{{ID: "g1", Title: "Obsession"}}

	plan := migrate.BuildPlan(mediaserver.TypeMovie, source, target)

	if len(plan.Matches) != 0 {
		t.Fatalf("want no match, got %+v", plan.Matches)
	}
	if len(plan.Unmatched) != 2 {
		t.Fatalf("want both reported, got %+v", plan.Unmatched)
	}
	for _, u := range plan.Unmatched {
		if u.Reason != migrate.ReasonAmbiguous {
			t.Errorf("item %s: reason = %q", u.SourceID, u.Reason)
		}
	}
}

func TestBuildPlan_SeasonsMatchOnSeriesPlusNumber(t *testing.T) {
	seriesIDs := ids(map[string]string{"tvdb": "81189"})
	source := []mediaserver.Item{
		{ID: "201", Title: "Breaking Bad", SeasonNumber: 1, ExternalIDs: seriesIDs},
		{ID: "202", Title: "Breaking Bad", SeasonNumber: 2, ExternalIDs: seriesIDs},
	}
	target := []mediaserver.Item{
		{ID: "gs2", Title: "Breaking Bad", SeasonNumber: 2, ExternalIDs: seriesIDs},
		{ID: "gs1", Title: "Breaking Bad", SeasonNumber: 1, ExternalIDs: seriesIDs},
	}

	plan := migrate.BuildPlan(mediaserver.TypeSeason, source, target)

	if len(plan.Matches) != 2 {
		t.Fatalf("matches: want 2, got %d (%+v)", len(plan.Matches), plan.Unmatched)
	}
	got := map[string]string{}
	for _, m := range plan.Matches {
		got[m.SourceID] = m.TargetID
	}
	if got["201"] != "gs1" || got["202"] != "gs2" {
		t.Errorf("seasons crossed over: %v", got)
	}
}

func TestBuildPlan_NoCounterpartIsReported(t *testing.T) {
	source := []mediaserver.Item{
		{ID: "1", Title: "Only In Plex", Year: 1999, ExternalIDs: ids(map[string]string{"tmdb": "1"})},
	}
	target := []mediaserver.Item{
		{ID: "g1", Title: "Something Else", Year: 2001, ExternalIDs: ids(map[string]string{"tmdb": "2"})},
	}

	plan := migrate.BuildPlan(mediaserver.TypeMovie, source, target)

	if len(plan.Matches) != 0 {
		t.Fatalf("want no match, got %+v", plan.Matches)
	}
	if len(plan.Unmatched) != 1 || plan.Unmatched[0].Reason != migrate.ReasonNoCounterpart {
		t.Errorf("unmatched: got %+v", plan.Unmatched)
	}
}

// TestBuildPlan_OneTargetIsClaimedOnce stops a single target item from
// receiving artwork from two different source items.
func TestBuildPlan_OneTargetIsClaimedOnce(t *testing.T) {
	source := []mediaserver.Item{
		{ID: "1", Title: "Inception", Year: 2010, ExternalIDs: ids(map[string]string{"tmdb": "27205"})},
		{ID: "2", Title: "Inception", Year: 2010},
	}
	target := []mediaserver.Item{
		{ID: "g1", Title: "Inception", Year: 2010, ExternalIDs: ids(map[string]string{"tmdb": "27205"})},
	}

	plan := migrate.BuildPlan(mediaserver.TypeMovie, source, target)

	if len(plan.Matches) != 1 {
		t.Fatalf("matches: want 1, got %d", len(plan.Matches))
	}
	if plan.Matches[0].SourceID != "1" || plan.Matches[0].By != migrate.ByExternalID {
		t.Errorf("the external-id match should win: %+v", plan.Matches[0])
	}
	if len(plan.Unmatched) != 1 || plan.Unmatched[0].SourceID != "2" {
		t.Errorf("unmatched: got %+v", plan.Unmatched)
	}
}

func TestNormalizeTitle(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Invincible (2021)", "invincible"},
		{"The Matrix", "the matrix"},
		{"Bob l'éponge, le film : Un pour tous", "bob l éponge le film un pour tous"},
		{"  Spaced   Out  ", "spaced out"},
		{"MCU", "mcu"},
		{"Blade Runner 2049", "blade runner 2049"}, // a year in the middle is part of the title
		{"", ""},
	}
	for _, c := range cases {
		if got := migrate.NormalizeTitle(c.in); got != c.want {
			t.Errorf("NormalizeTitle(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}
