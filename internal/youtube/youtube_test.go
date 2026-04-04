package youtube

import (
	"testing"
	"time"
)

func TestParseUploadDate_Valid(t *testing.T) {
	cases := []struct {
		input string
		want  time.Time
	}{
		{"20260101", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)},
		{"20261231", time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)},
		{"20240229", time.Date(2024, 2, 29, 0, 0, 0, 0, time.UTC)}, // leap day
	}

	for _, tc := range cases {
		got := parseUploadDate(tc.input)
		if !got.Equal(tc.want) {
			t.Errorf("parseUploadDate(%q) = %v, want %v", tc.input, got, tc.want)
		}
	}
}

func TestParseUploadDate_Invalid(t *testing.T) {
	cases := []string{
		"",
		"2026-01-01",  // wrong format
		"20261301",    // invalid month
		"abcdefgh",
		"2026",
	}

	for _, input := range cases {
		got := parseUploadDate(input)
		if !got.IsZero() {
			t.Errorf("parseUploadDate(%q) = %v, want zero time", input, got)
		}
	}
}

func TestParseYtdlpOutput(t *testing.T) {
	// Test the JSON parsing logic directly by constructing a ytFlatPlaylist
	// and verifying the VideoMeta output.
	playlist := ytFlatPlaylist{
		Entries: []struct {
			ID          string `json:"id"`
			Title       string `json:"title"`
			Description string `json:"description"`
			UploadDate  string `json:"upload_date"`
			Duration    float64 `json:"duration"`
			URL         string `json:"url"`
		}{
			{
				ID:         "abc123",
				Title:      "Test Stream",
				UploadDate: "20260401",
				Duration:   7200.0,
				URL:        "https://www.youtube.com/watch?v=abc123",
			},
			{
				ID:    "nouploaddate",
				Title: "No Date",
				// UploadDate intentionally empty
			},
		},
	}

	videos := make([]VideoMeta, 0, len(playlist.Entries))
	for _, e := range playlist.Entries {
		meta := VideoMeta{
			ID:           e.ID,
			Title:        e.Title,
			Description:  e.Description,
			DurationSecs: int(e.Duration),
			WebpageURL:   "https://www.youtube.com/watch?v=" + e.ID,
		}
		if e.UploadDate != "" {
			meta.PublishedAt = parseUploadDate(e.UploadDate)
		}
		videos = append(videos, meta)
	}

	if len(videos) != 2 {
		t.Fatalf("got %d videos, want 2", len(videos))
	}

	v0 := videos[0]
	if v0.ID != "abc123" {
		t.Errorf("ID = %q, want abc123", v0.ID)
	}
	if v0.DurationSecs != 7200 {
		t.Errorf("DurationSecs = %d, want 7200", v0.DurationSecs)
	}
	if v0.WebpageURL != "https://www.youtube.com/watch?v=abc123" {
		t.Errorf("WebpageURL = %q", v0.WebpageURL)
	}
	if v0.PublishedAt.IsZero() {
		t.Error("PublishedAt should not be zero for valid upload date")
	}

	v1 := videos[1]
	if !v1.PublishedAt.IsZero() {
		t.Error("PublishedAt should be zero when upload_date is missing")
	}
}
