package plex

import "testing"

func TestSeasonYear(t *testing.T) {
	tests := []struct {
		name                  string
		originallyAvailableAt string
		year                  int
		want                  int
	}{
		{
			name:                  "extracts year from full date",
			originallyAvailableAt: "2014-01-13",
			year:                  0,
			want:                  2014,
		},
		{
			name:                  "extracts year from year-only string",
			originallyAvailableAt: "2009",
			year:                  0,
			want:                  2009,
		},
		{
			name:                  "falls back to Year when field is empty",
			originallyAvailableAt: "",
			year:                  2008,
			want:                  2008,
		},
		{
			name:                  "falls back to Year when field is too short",
			originallyAvailableAt: "200",
			year:                  2008,
			want:                  2008,
		},
		{
			name:                  "falls back to Year when field is non-numeric",
			originallyAvailableAt: "abcd-01-01",
			year:                  2008,
			want:                  2008,
		},
		{
			name:                  "returns 0 when both fields are empty",
			originallyAvailableAt: "",
			year:                  0,
			want:                  0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			item := Item{
				OriginallyAvailableAt: tt.originallyAvailableAt,
				Year:                  tt.year,
			}
			if got := item.SeasonYear(); got != tt.want {
				t.Errorf("SeasonYear() = %d, want %d", got, tt.want)
			}
		})
	}
}
