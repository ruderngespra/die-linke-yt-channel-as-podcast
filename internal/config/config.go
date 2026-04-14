package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	// GitHub
	GitHubToken string
	GitHubOwner string
	GitHubRepo  string
	FeedBaseURL string // GitHub Pages base URL

	// Feed metadata
	FeedTitle       string
	FeedDescription string
	FeedLanguage    string
	FeedLink        string

	// Runtime
	DryRun          bool
	YTPlaylistLimit int
	TempDir         string
}

func LoadFromEnv() (*Config, error) {
	cfg := &Config{
		FeedTitle:       "DIE LINKE Pressekonferenzen",
		FeedDescription: "Livestreams und Pressekonferenzen von DIE LINKE und der Linksfraktion im Bundestag, als Podcast.",
		FeedLanguage:    "de",
		FeedLink:        "https://www.youtube.com/@DIELINKE/streams",
		YTPlaylistLimit: 5,
		TempDir:         os.TempDir(),
	}

	required := map[string]*string{
		"GITHUB_TOKEN":   &cfg.GitHubToken,
		"GITHUB_OWNER":   &cfg.GitHubOwner,
		"GITHUB_REPO":    &cfg.GitHubRepo,
		"FEED_BASE_URL":  &cfg.FeedBaseURL,
	}

	for key, dest := range required {
		val := os.Getenv(key)
		if val == "" {
			return nil, fmt.Errorf("required env var %s is not set", key)
		}
		*dest = val
	}

	if v := os.Getenv("DRY_RUN"); v != "" {
		cfg.DryRun, _ = strconv.ParseBool(v)
	}

	if v := os.Getenv("YT_PLAYLIST_LIMIT"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil {
			return nil, fmt.Errorf("invalid YT_PLAYLIST_LIMIT: %w", err)
		}
		cfg.YTPlaylistLimit = n
	}

	if v := os.Getenv("TEMP_DIR"); v != "" {
		cfg.TempDir = v
	}

	return cfg, nil
}
