package audio

import (
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
)

// TranscodeToMP3 converts inputPath to a 128kbps MP3 in destDir.
// Returns the path of the output MP3.
func TranscodeToMP3(videoID, inputPath, destDir string) (string, error) {
	outputPath := filepath.Join(destDir, videoID+".mp3")

	cmd := exec.Command(
		"ffmpeg",
		"-i", inputPath,
		"-vn",                // no video stream
		"-acodec", "libmp3lame",
		"-ab", "128k",
		"-y",                 // overwrite without prompt
		outputPath,
	)

	if out, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("ffmpeg transcode %s: %w\n%s", videoID, err, out)
	}

	return outputPath, nil
}

// ffprobeOutput is the relevant subset of ffprobe's JSON output.
type ffprobeOutput struct {
	Streams []struct {
		CodecType string `json:"codec_type"`
		Duration  string `json:"duration"`
	} `json:"streams"`
	Format struct {
		Duration string `json:"duration"`
	} `json:"format"`
}

// ProbeDuration returns the duration of an audio file in seconds.
func ProbeDuration(filePath string) (int, error) {
	cmd := exec.Command(
		"ffprobe",
		"-v", "quiet",
		"-print_format", "json",
		"-show_streams",
		"-show_format",
		filePath,
	)

	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe %s: %w", filePath, err)
	}

	var probe ffprobeOutput
	if err := json.Unmarshal(out, &probe); err != nil {
		return 0, fmt.Errorf("parse ffprobe output: %w", err)
	}

	// Prefer duration from the first audio stream, fall back to format duration
	durStr := probe.Format.Duration
	for _, s := range probe.Streams {
		if strings.EqualFold(s.CodecType, "audio") && s.Duration != "" {
			durStr = s.Duration
			break
		}
	}

	if durStr == "" {
		return 0, fmt.Errorf("no duration found in %s", filePath)
	}

	f, err := strconv.ParseFloat(durStr, 64)
	if err != nil {
		return 0, fmt.Errorf("parse duration %q: %w", durStr, err)
	}

	return int(f), nil
}
