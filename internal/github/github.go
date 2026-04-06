package github

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

const apiBase = "https://api.github.com"

// Episode holds all metadata for a podcast episode, stored in the GitHub Release body.
type Episode struct {
	VideoID       string    `json:"video_id"`
	Title         string    `json:"title"`
	Description   string    `json:"description"`
	PublishedAt   time.Time `json:"published_at"`
	DurationSecs  int       `json:"duration_secs"`
	FileSizeBytes int64     `json:"file_size_bytes"`
	MP3URL        string    `json:"mp3_url"`
}

// ListProcessedVideoIDs queries the Releases API and returns all video IDs
// that already have a release (tag format: "video-{videoID}").
func ListProcessedVideoIDs(ctx context.Context, token, owner, repo string) (map[string]bool, error) {
	releases, err := listReleases(ctx, token, owner, repo)
	if err != nil {
		return nil, err
	}

	ids := make(map[string]bool, len(releases))
	for _, r := range releases {
		if videoID, ok := videoIDFromTag(r.TagName); ok {
			ids[videoID] = true
		}
	}
	return ids, nil
}

// GetAllEpisodes returns full episode metadata from all releases, newest first.
func GetAllEpisodes(ctx context.Context, token, owner, repo string) ([]Episode, error) {
	releases, err := listReleases(ctx, token, owner, repo)
	if err != nil {
		return nil, err
	}

	episodes := make([]Episode, 0, len(releases))
	for _, r := range releases {
		if _, ok := videoIDFromTag(r.TagName); !ok {
			continue
		}
		var ep Episode
		if err := json.Unmarshal([]byte(r.Body), &ep); err != nil {
			// Skip releases with unparseable bodies — don't fail the whole run
			continue
		}
		episodes = append(episodes, ep)
	}
	return episodes, nil
}

// CreateRelease creates a GitHub Release tagged "video-{videoID}", uploads the MP3
// as a release asset, and returns the public download URL of the asset.
func CreateRelease(ctx context.Context, token, owner, repo string, ep Episode, mp3Path string) (string, error) {
	tag := "video-" + ep.VideoID

	// Get file size before creating the release
	info, err := os.Stat(mp3Path)
	if err != nil {
		return "", fmt.Errorf("stat mp3: %w", err)
	}
	ep.FileSizeBytes = info.Size()

	// Derive the public asset URL before upload so we can store it in the release body.
	// GitHub asset download URLs are deterministic:
	// https://github.com/{owner}/{repo}/releases/download/{tag}/{filename}
	assetFilename := ep.VideoID + ".mp3"
	ep.MP3URL = fmt.Sprintf("https://github.com/%s/%s/releases/download/%s/%s",
		owner, repo, tag, assetFilename)

	body, err := json.Marshal(ep)
	if err != nil {
		return "", fmt.Errorf("marshal episode: %w", err)
	}

	type createReleaseRequest struct {
		TagName string `json:"tag_name"`
		Name    string `json:"name"`
		Body    string `json:"body"`
		Draft   bool   `json:"draft"`
	}

	reqBody, _ := json.Marshal(createReleaseRequest{
		TagName: tag,
		Name:    ep.Title,
		Body:    string(body),
		Draft:   false,
	})

	var release struct {
		ID        int64  `json:"id"`
		UploadURL string `json:"upload_url"`
	}
	if err := doRequest(ctx, token, http.MethodPost,
		fmt.Sprintf("%s/repos/%s/%s/releases", apiBase, owner, repo),
		reqBody, &release); err != nil {
		return "", fmt.Errorf("create release: %w", err)
	}

	// Upload the MP3 asset
	if _, err := uploadAsset(ctx, token, release.UploadURL, assetFilename, mp3Path); err != nil {
		return "", fmt.Errorf("upload asset: %w", err)
	}

	return ep.MP3URL, nil
}

// PublishFeed commits feed.xml to the gh-pages branch.
func PublishFeed(ctx context.Context, token, owner, repo string, feedXML []byte) error {
	const branch = "gh-pages"
	const path = "feed.xml"

	// Get current file SHA (needed for update; absent on first push)
	sha, err := getFileSHA(ctx, token, owner, repo, branch, path)
	if err != nil {
		return fmt.Errorf("get file SHA: %w", err)
	}

	type updateFileRequest struct {
		Message string `json:"message"`
		Content string `json:"content"`
		SHA     string `json:"sha,omitempty"`
		Branch  string `json:"branch"`
	}

	req := updateFileRequest{
		Message: "Update podcast feed",
		Content: base64.StdEncoding.EncodeToString(feedXML),
		Branch:  branch,
	}
	if sha != "" {
		req.SHA = sha
	}

	reqBody, _ := json.Marshal(req)
	if err := doRequest(ctx, token, http.MethodPut,
		fmt.Sprintf("%s/repos/%s/%s/contents/%s", apiBase, owner, repo, path),
		reqBody, nil); err != nil {
		return fmt.Errorf("publish feed: %w", err)
	}

	return nil
}

// --- internal helpers ---

type ghRelease struct {
	ID      int64  `json:"id"`
	TagName string `json:"tag_name"`
	Name    string `json:"name"`
	Body    string `json:"body"`
}

func listReleases(ctx context.Context, token, owner, repo string) ([]ghRelease, error) {
	var all []ghRelease
	page := 1
	for {
		url := fmt.Sprintf("%s/repos/%s/%s/releases?per_page=100&page=%d", apiBase, owner, repo, page)
		var page_releases []ghRelease
		if err := doRequest(ctx, token, http.MethodGet, url, nil, &page_releases); err != nil {
			return nil, fmt.Errorf("list releases page %d: %w", page, err)
		}
		all = append(all, page_releases...)
		if len(page_releases) < 100 {
			break
		}
		page++
	}
	return all, nil
}

func videoIDFromTag(tag string) (string, bool) {
	after, found := strings.CutPrefix(tag, "video-")
	if !found || after == "" {
		return "", false
	}
	return after, true
}

func getFileSHA(ctx context.Context, token, owner, repo, branch, path string) (string, error) {
	url := fmt.Sprintf("%s/repos/%s/%s/contents/%s?ref=%s", apiBase, owner, repo, path, branch)
	var result struct {
		SHA string `json:"sha"`
	}
	err := doRequest(ctx, token, http.MethodGet, url, nil, &result)
	if err != nil {
		// 404 means file doesn't exist yet — that's fine
		if strings.Contains(err.Error(), "404") {
			return "", nil
		}
		return "", err
	}
	return result.SHA, nil
}

func uploadAsset(ctx context.Context, token, uploadURL, filename, filePath string) (string, error) {
	// uploadURL from GitHub API looks like: https://uploads.github.com/repos/.../releases/{id}/assets{?name,label}
	// Strip the template part and add our filename
	baseURL := strings.Split(uploadURL, "{")[0]
	url := baseURL + "?name=" + filename

	f, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("open %s: %w", filePath, err)
	}
	defer f.Close()

	info, err := f.Stat()
	if err != nil {
		return "", fmt.Errorf("stat %s: %w", filePath, err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, f)
	if err != nil {
		return "", err
	}
	req.ContentLength = info.Size()
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "audio/mpeg")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("upload asset: %w", err)
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("upload asset HTTP %d: %s", resp.StatusCode, respBody)
	}

	var result struct {
		BrowserDownloadURL string `json:"browser_download_url"`
	}
	if err := json.Unmarshal(respBody, &result); err != nil {
		return "", fmt.Errorf("parse upload response: %w", err)
	}
	return result.BrowserDownloadURL, nil
}

func doRequest(ctx context.Context, token, method, url string, body []byte, out any) error {
	var bodyReader io.Reader
	if body != nil {
		bodyReader = bytes.NewReader(body)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("http %s %s: %w", method, url, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("%d: %s", resp.StatusCode, respBody)
	}

	if out != nil && len(respBody) > 0 {
		if err := json.Unmarshal(respBody, out); err != nil {
			return fmt.Errorf("parse response: %w", err)
		}
	}
	return nil
}
