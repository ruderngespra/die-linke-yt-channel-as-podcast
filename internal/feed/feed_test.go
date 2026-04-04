package feed

import (
	"encoding/xml"
	"strings"
	"testing"
	"time"
)

var testMeta = FeedMeta{
	Title:       "Test Podcast",
	Description: "Test description",
	Language:    "de",
	Link:        "https://example.com",
}

func TestGenerate_ValidXML(t *testing.T) {
	episodes := []Episode{
		{
			VideoID:       "abc123",
			Title:         "Episode 1",
			Description:   "First episode",
			PublishedAt:   time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
			DurationSecs:  3661,
			FileSizeBytes: 1024,
			MP3URL:        "https://example.com/audio/abc123.mp3",
		},
	}

	data, err := Generate(testMeta, episodes)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	// Must parse as valid XML
	var result rss
	if err := xml.Unmarshal(data, &result); err != nil {
		t.Fatalf("output is not valid XML: %v\n%s", err, data)
	}
}

func TestGenerate_FeedMetadata(t *testing.T) {
	data, err := Generate(testMeta, nil)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	s := string(data)
	for _, want := range []string{
		"<title>Test Podcast</title>",
		"<description>Test description</description>",
		"<language>de</language>",
		"http://www.itunes.com/dtds/podcast-1.0.dtd",
		`version="2.0"`,
	} {
		if !strings.Contains(s, want) {
			t.Errorf("output missing %q", want)
		}
	}
}

func TestGenerate_EpisodeEnclosure(t *testing.T) {
	episodes := []Episode{
		{
			VideoID:       "xyz789",
			Title:         "Test Episode",
			FileSizeBytes: 2048,
			MP3URL:        "https://example.com/audio/xyz789.mp3",
			DurationSecs:  120,
		},
	}

	data, err := Generate(testMeta, episodes)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	s := string(data)
	if !strings.Contains(s, `url="https://example.com/audio/xyz789.mp3"`) {
		t.Error("enclosure URL not found in output")
	}
	if !strings.Contains(s, `length="2048"`) {
		t.Error("enclosure length not found in output")
	}
	if !strings.Contains(s, `type="audio/mpeg"`) {
		t.Error("enclosure type not found in output")
	}
	if !strings.Contains(s, "<guid") {
		t.Error("guid element not found in output")
	}
}

func TestGenerate_XMLHeader(t *testing.T) {
	data, err := Generate(testMeta, nil)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if !strings.HasPrefix(string(data), "<?xml") {
		t.Error("output should start with XML declaration")
	}
}

func TestGenerate_EmptyEpisodes(t *testing.T) {
	data, err := Generate(testMeta, []Episode{})
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("output should not be empty even with no episodes")
	}
}

func TestFormatDuration(t *testing.T) {
	cases := []struct {
		secs int
		want string
	}{
		{0, "0:00"},
		{59, "0:59"},
		{60, "1:00"},
		{90, "1:30"},
		{3600, "1:00:00"},
		{3661, "1:01:01"},
		{7322, "2:02:02"},
	}

	for _, tc := range cases {
		got := formatDuration(tc.secs)
		if got != tc.want {
			t.Errorf("formatDuration(%d) = %q, want %q", tc.secs, got, tc.want)
		}
	}
}
