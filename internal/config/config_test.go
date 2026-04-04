package config

import (
	"testing"
)

func TestLoadFromEnv_Success(t *testing.T) {
	setRequiredEnv(t)

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if cfg.GitHubToken != "test-token" {
		t.Errorf("GitHubToken = %q, want test-token", cfg.GitHubToken)
	}
	if cfg.YTPlaylistLimit != 50 {
		t.Errorf("YTPlaylistLimit = %d, want 50 (default)", cfg.YTPlaylistLimit)
	}
	if cfg.FeedTitle != "DIE LINKE Pressekonferenzen" {
		t.Errorf("FeedTitle = %q, want default", cfg.FeedTitle)
	}
	if cfg.DryRun {
		t.Error("DryRun should default to false")
	}
}

func TestLoadFromEnv_MissingRequired(t *testing.T) {
	required := []string{
		"GITHUB_TOKEN",
		"GITHUB_OWNER",
		"GITHUB_REPO",
		"FEED_BASE_URL",
	}

	for _, key := range required {
		t.Run(key, func(t *testing.T) {
			setRequiredEnv(t)
			t.Setenv(key, "")

			_, err := LoadFromEnv()
			if err == nil {
				t.Errorf("expected error when %s is missing", key)
			}
		})
	}
}

func TestLoadFromEnv_DryRun(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("DRY_RUN", "true")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !cfg.DryRun {
		t.Error("DryRun should be true")
	}
}

func TestLoadFromEnv_PlaylistLimit(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("YT_PLAYLIST_LIMIT", "10")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.YTPlaylistLimit != 10 {
		t.Errorf("YTPlaylistLimit = %d, want 10", cfg.YTPlaylistLimit)
	}
}

func TestLoadFromEnv_InvalidPlaylistLimit(t *testing.T) {
	setRequiredEnv(t)
	t.Setenv("YT_PLAYLIST_LIMIT", "notanumber")

	_, err := LoadFromEnv()
	if err == nil {
		t.Error("expected error for invalid YT_PLAYLIST_LIMIT")
	}
}

func setRequiredEnv(t *testing.T) {
	t.Helper()
	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("GITHUB_OWNER", "ruderngespra")
	t.Setenv("GITHUB_REPO", "die-linke-yt-channel-as-podcast")
	t.Setenv("FEED_BASE_URL", "https://ruderngespra.github.io/die-linke-yt-channel-as-podcast")
}
