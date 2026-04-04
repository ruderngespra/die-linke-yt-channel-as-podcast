package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/ruderngespra/die-linke-yt-channel-as-podcast/internal/audio"
	"github.com/ruderngespra/die-linke-yt-channel-as-podcast/internal/config"
	"github.com/ruderngespra/die-linke-yt-channel-as-podcast/internal/feed"
	gh "github.com/ruderngespra/die-linke-yt-channel-as-podcast/internal/github"
	"github.com/ruderngespra/die-linke-yt-channel-as-podcast/internal/youtube"
)

func main() {
	if err := run(); err != nil {
		log.Fatalf("error: %v", err)
	}
}

func run() error {
	ctx := context.Background()

	// 1. Load config
	cfg, err := config.LoadFromEnv()
	if err != nil {
		return fmt.Errorf("config: %w", err)
	}
	log.Printf("starting sync (dry_run=%v, playlist_limit=%d)", cfg.DryRun, cfg.YTPlaylistLimit)

	// 2. List already-processed video IDs from GitHub Releases
	processed, err := gh.ListProcessedVideoIDs(ctx, cfg.GitHubToken, cfg.GitHubOwner, cfg.GitHubRepo)
	if err != nil {
		return fmt.Errorf("list processed IDs: %w", err)
	}
	log.Printf("%d videos already processed", len(processed))

	// 3. List recent streams from YouTube
	videos, err := youtube.ListStreams(cfg.YTPlaylistLimit)
	if err != nil {
		return fmt.Errorf("list streams: %w", err)
	}
	log.Printf("found %d videos from YouTube", len(videos))

	// 4. Set up temp dir
	tmpDir := filepath.Join(cfg.TempDir, "yt-to-podcast")
	if err := os.MkdirAll(tmpDir, 0o755); err != nil {
		return fmt.Errorf("create temp dir: %w", err)
	}
	defer func() {
		if err := os.RemoveAll(tmpDir); err != nil {
			log.Printf("warning: cleanup temp dir: %v", err)
		}
	}()

	// 5. Process new videos
	newCount := 0
	for _, video := range videos {
		if processed[video.ID] {
			log.Printf("skipping %s (already processed)", video.ID)
			continue
		}

		log.Printf("processing %s: %s", video.ID, video.Title)

		if err := processVideo(ctx, cfg, video, tmpDir); err != nil {
			log.Printf("error processing %s: %v — skipping", video.ID, err)
			continue
		}
		newCount++
	}
	log.Printf("processed %d new videos", newCount)

	// 6. Get all episodes from GitHub Releases (for feed regeneration)
	episodes, err := gh.GetAllEpisodes(ctx, cfg.GitHubToken, cfg.GitHubOwner, cfg.GitHubRepo)
	if err != nil {
		return fmt.Errorf("get all episodes: %w", err)
	}

	// 7. Generate RSS feed
	feedMeta := feed.FeedMeta{
		Title:       cfg.FeedTitle,
		Description: cfg.FeedDescription,
		Language:    cfg.FeedLanguage,
		Link:        cfg.FeedLink,
	}

	// Convert gh.Episode to feed.Episode
	feedEpisodes := make([]feed.Episode, 0, len(episodes))
	for _, ep := range episodes {
		feedEpisodes = append(feedEpisodes, feed.Episode{
			VideoID:       ep.VideoID,
			Title:         ep.Title,
			Description:   ep.Description,
			PublishedAt:   ep.PublishedAt,
			DurationSecs:  ep.DurationSecs,
			FileSizeBytes: ep.FileSizeBytes,
			MP3URL:        ep.MP3URL,
		})
	}

	rssXML, err := feed.Generate(feedMeta, feedEpisodes)
	if err != nil {
		return fmt.Errorf("generate feed: %w", err)
	}

	if cfg.DryRun {
		dryRunPath := filepath.Join(tmpDir, "feed.xml")
		if err := os.WriteFile(dryRunPath, rssXML, 0o644); err != nil {
			return fmt.Errorf("write dry-run feed: %w", err)
		}
		log.Printf("dry-run: feed written to %s", dryRunPath)
		return nil
	}

	// 8. Publish feed to GitHub Pages (gh-pages branch)
	if err := gh.PublishFeed(ctx, cfg.GitHubToken, cfg.GitHubOwner, cfg.GitHubRepo, rssXML); err != nil {
		return fmt.Errorf("publish feed: %w", err)
	}

	feedURL := strings.TrimRight(cfg.FeedBaseURL, "/") + "/feed.xml"
	log.Printf("feed published: %s", feedURL)

	return nil
}

func processVideo(ctx context.Context, cfg *config.Config, video youtube.VideoMeta, tmpDir string) error {
	// Download audio
	downloadedPath, err := youtube.Download(video.ID, tmpDir)
	if err != nil {
		return fmt.Errorf("download: %w", err)
	}

	// Transcode to MP3
	mp3Path, err := audio.TranscodeToMP3(video.ID, downloadedPath, tmpDir)
	if err != nil {
		return fmt.Errorf("transcode: %w", err)
	}
	_ = os.Remove(downloadedPath)

	// Probe duration
	durationSecs, err := audio.ProbeDuration(mp3Path)
	if err != nil {
		log.Printf("warning: could not probe duration for %s: %v", video.ID, err)
	}

	ep := gh.Episode{
		VideoID:      video.ID,
		Title:        video.Title,
		Description:  video.Description,
		PublishedAt:  video.PublishedAt,
		DurationSecs: durationSecs,
	}

	if cfg.DryRun {
		log.Printf("dry-run: would create release for %s", video.ID)
		return nil
	}

	// Create GitHub Release and upload MP3
	mp3URL, err := gh.CreateRelease(ctx, cfg.GitHubToken, cfg.GitHubOwner, cfg.GitHubRepo, ep, mp3Path)
	if err != nil {
		return fmt.Errorf("create release: %w", err)
	}

	log.Printf("uploaded %s: %s", video.ID, mp3URL)
	return nil
}
