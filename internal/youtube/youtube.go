package youtube

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

const channelURL = "https://www.youtube.com/@DIELINKE/streams"

type VideoMeta struct {
	ID           string
	Title        string
	Description  string
	PublishedAt  time.Time
	DurationSecs int
	WebpageURL   string
}

// ytFlatEntry is the JSON shape returned by yt-dlp --flat-playlist --dump-single-json
type ytFlatPlaylist struct {
	Entries []struct {
		ID          string `json:"id"`
		Title       string `json:"title"`
		Description string `json:"description"`
		UploadDate  string `json:"upload_date"` // YYYYMMDD, may be absent in flat mode
		Duration    float64 `json:"duration"`
		URL         string `json:"url"`
	} `json:"entries"`
}

// ListStreams returns up to limit recent livestreams from the channel, newest first.
func ListStreams(limit int) ([]VideoMeta, error) {
	playlistItems := fmt.Sprintf("1-%d", limit)
	cmd := exec.Command(
		"yt-dlp",
		"--flat-playlist",
		"--dump-single-json",
		"--playlist-items", playlistItems,
		"--no-warnings",
		"--quiet",
		channelURL,
	)

	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp list: %w", err)
	}

	var playlist ytFlatPlaylist
	if err := json.Unmarshal(out, &playlist); err != nil {
		return nil, fmt.Errorf("parse yt-dlp output: %w", err)
	}

	videos := make([]VideoMeta, 0, len(playlist.Entries))
	for _, e := range playlist.Entries {
		meta := VideoMeta{
			ID:           e.ID,
			Title:        e.Title,
			Description:  e.Description,
			DurationSecs: int(e.Duration),
			WebpageURL:   fmt.Sprintf("https://www.youtube.com/watch?v=%s", e.ID),
		}
		if e.UploadDate != "" {
			meta.PublishedAt = parseUploadDate(e.UploadDate)
		}
		videos = append(videos, meta)
	}
	return videos, nil
}

// ytVideoMeta is the JSON shape returned by yt-dlp --dump-json for a single video.
type ytVideoMeta struct {
	ID          string  `json:"id"`
	Title       string  `json:"title"`
	Description string  `json:"description"`
	UploadDate  string  `json:"upload_date"` // YYYYMMDD
	Duration    float64 `json:"duration"`
}

// FetchVideoMeta fetches full metadata for a single video, including the correct
// locale-aware title and description.
func FetchVideoMeta(ctx context.Context, videoID string) (VideoMeta, error) {
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	cmd := exec.CommandContext(ctx,
		"yt-dlp",
		"--dump-json",
		"--no-playlist",
		"--no-warnings",
		"--quiet",
		videoURL,
	)

	out, err := cmd.Output()
	if err != nil {
		return VideoMeta{}, fmt.Errorf("yt-dlp fetch meta %s: %w", videoID, err)
	}

	var v ytVideoMeta
	if err := json.Unmarshal(out, &v); err != nil {
		return VideoMeta{}, fmt.Errorf("parse yt-dlp meta %s: %w", videoID, err)
	}

	meta := VideoMeta{
		ID:           v.ID,
		Title:        v.Title,
		Description:  v.Description,
		DurationSecs: int(v.Duration),
		WebpageURL:   videoURL,
	}
	if v.UploadDate != "" {
		meta.PublishedAt = parseUploadDate(v.UploadDate)
	}
	return meta, nil
}

// Download downloads the best audio for a video ID into destDir.
// Returns the path of the downloaded file (extension varies).
func Download(videoID, destDir string) (string, error) {
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	outputTemplate := filepath.Join(destDir, videoID+".%(ext)s")

	cmd := exec.Command(
		"yt-dlp",
		"--format", "bestaudio",
		"--no-playlist",
		"--output", outputTemplate,
		"--no-warnings",
		videoURL,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("yt-dlp download %s: %w\n%s", videoID, err, out)
	}

	// Find the downloaded file — yt-dlp may use any extension
	matches, err := filepath.Glob(filepath.Join(destDir, videoID+".*"))
	if err != nil || len(matches) == 0 {
		return "", fmt.Errorf("downloaded file not found for video %s", videoID)
	}
	// Filter out .part files (incomplete downloads)
	for _, m := range matches {
		if !strings.HasSuffix(m, ".part") {
			return m, nil
		}
	}
	return "", fmt.Errorf("no complete download found for video %s", videoID)
}

func parseUploadDate(s string) time.Time {
	// YYYYMMDD
	if len(s) != 8 {
		return time.Time{}
	}
	t, err := time.Parse("20060102", s)
	if err != nil {
		return time.Time{}
	}
	return t.UTC()
}
