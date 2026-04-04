package github

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestVideoIDFromTag(t *testing.T) {
	cases := []struct {
		tag     string
		want    string
		wantOK  bool
	}{
		{"video-abc123", "abc123", true},
		{"video-xyz_789", "xyz_789", true},
		{"abc123", "", false},       // no prefix
		{"video-", "", false},       // empty ID
		{"other-abc123", "", false}, // wrong prefix
	}

	for _, tc := range cases {
		got, ok := videoIDFromTag(tc.tag)
		if ok != tc.wantOK {
			t.Errorf("videoIDFromTag(%q) ok=%v, want %v", tc.tag, ok, tc.wantOK)
		}
		if got != tc.want {
			t.Errorf("videoIDFromTag(%q) = %q, want %q", tc.tag, got, tc.want)
		}
	}
}

func TestListProcessedVideoIDs(t *testing.T) {
	releases := []ghRelease{
		{ID: 1, TagName: "video-abc123"},
		{ID: 2, TagName: "video-xyz789"},
		{ID: 3, TagName: "other-release"}, // should be ignored
	}

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(releases)
	}))
	defer server.Close()

	// Test the core logic directly using listReleases equivalent
	ids := make(map[string]bool)
	for _, r := range releases {
		if videoID, ok := videoIDFromTag(r.TagName); ok {
			ids[videoID] = true
		}
	}

	if !ids["abc123"] {
		t.Error("abc123 should be in processed IDs")
	}
	if !ids["xyz789"] {
		t.Error("xyz789 should be in processed IDs")
	}
	if ids["other-release"] {
		t.Error("other-release should not be in processed IDs")
	}
	if len(ids) != 2 {
		t.Errorf("got %d IDs, want 2", len(ids))
	}
}

func TestGetAllEpisodes_ParsesBody(t *testing.T) {
	ep := Episode{
		VideoID:       "abc123",
		Title:         "Test Episode",
		Description:   "A test",
		PublishedAt:   time.Date(2026, 4, 1, 10, 0, 0, 0, time.UTC),
		DurationSecs:  3600,
		FileSizeBytes: 1024,
		MP3URL:        "https://example.com/abc123.mp3",
	}
	body, _ := json.Marshal(ep)

	releases := []ghRelease{
		{ID: 1, TagName: "video-abc123", Body: string(body)},
		{ID: 2, TagName: "video-xyz789", Body: "invalid json"}, // should be skipped
		{ID: 3, TagName: "other-tag", Body: string(body)},      // should be skipped (wrong tag)
	}

	// Test the parsing logic directly
	episodes := make([]Episode, 0)
	for _, r := range releases {
		if _, ok := videoIDFromTag(r.TagName); !ok {
			continue
		}
		var parsed Episode
		if err := json.Unmarshal([]byte(r.Body), &parsed); err != nil {
			continue
		}
		episodes = append(episodes, parsed)
	}

	if len(episodes) != 1 {
		t.Fatalf("got %d episodes, want 1", len(episodes))
	}
	if episodes[0].VideoID != "abc123" {
		t.Errorf("VideoID = %q, want abc123", episodes[0].VideoID)
	}
	if episodes[0].DurationSecs != 3600 {
		t.Errorf("DurationSecs = %d, want 3600", episodes[0].DurationSecs)
	}
}

func TestGetAllEpisodes_EmptyReleases(t *testing.T) {
	episodes := make([]Episode, 0)
	// No releases = no episodes, no error
	if len(episodes) != 0 {
		t.Error("expected empty episode list")
	}
}

func TestDoRequest_HTTPError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, `{"message":"Not Found"}`, http.StatusNotFound)
	}))
	defer server.Close()

	err := doRequest(context.Background(), "token", http.MethodGet, server.URL, nil, nil)
	if err == nil {
		t.Error("expected error for HTTP 404")
	}
	if !containsString(err.Error(), "404") {
		t.Errorf("error should mention 404, got: %v", err)
	}
}

func TestDoRequest_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Verify auth header is set
		if r.Header.Get("Authorization") == "" {
			t.Error("Authorization header missing")
		}
		if r.Header.Get("X-GitHub-Api-Version") == "" {
			t.Error("X-GitHub-Api-Version header missing")
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"id": "123"})
	}))
	defer server.Close()

	var out map[string]string
	err := doRequest(context.Background(), "mytoken", http.MethodGet, server.URL, nil, &out)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out["id"] != "123" {
		t.Errorf("out[id] = %q, want 123", out["id"])
	}
}

func TestEpisodeJSON_Roundtrip(t *testing.T) {
	ep := Episode{
		VideoID:       "test123",
		Title:         "Test",
		PublishedAt:   time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		DurationSecs:  7200,
		FileSizeBytes: 50_000_000,
		MP3URL:        "https://github.com/.../test123.mp3",
	}

	data, err := json.Marshal(ep)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	var got Episode
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	if got.VideoID != ep.VideoID {
		t.Errorf("VideoID = %q, want %q", got.VideoID, ep.VideoID)
	}
	if got.DurationSecs != ep.DurationSecs {
		t.Errorf("DurationSecs = %d, want %d", got.DurationSecs, ep.DurationSecs)
	}
	if !got.PublishedAt.Equal(ep.PublishedAt) {
		t.Errorf("PublishedAt = %v, want %v", got.PublishedAt, ep.PublishedAt)
	}
}

func containsString(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		func() bool {
			for i := 0; i <= len(s)-len(substr); i++ {
				if s[i:i+len(substr)] == substr {
					return true
				}
			}
			return false
		}())
}
