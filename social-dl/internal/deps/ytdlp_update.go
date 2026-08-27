package deps

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const ytdlpUpdateInterval = 24 * time.Hour

// MaybeUpdateYtDlp runs yt-dlp's built-in updater at most once per day.
// Update failures are returned to the caller so they can be logged, but the
// existing binary remains usable and is never removed by this function.
func MaybeUpdateYtDlp(ctx context.Context, ytdlpPath string) (string, error) {
	if ytdlpPath == "" || !fileExists(ytdlpPath) {
		return "", fmt.Errorf("yt-dlp binary not found: %s", ytdlpPath)
	}

	markerPath := ytdlpUpdateMarkerPath(ytdlpPath)
	if !shouldCheckYtDlp(markerPath, time.Now()) {
		return "", nil
	}

	cmd := exec.CommandContext(ctx, ytdlpPath, "-U")
	output, err := cmd.CombinedOutput()
	if err != nil {
		message := strings.TrimSpace(string(output))
		if message != "" {
			return message, fmt.Errorf("yt-dlp self-update: %w: %s", err, message)
		}
		return "", fmt.Errorf("yt-dlp self-update: %w", err)
	}

	// A successful -U means the binary is either updated or already current.
	// Record the check only after that success; transient network failures will
	// therefore be retried on the next launch.
	markYtDlpChecked(markerPath, time.Now())
	return strings.TrimSpace(string(output)), nil
}

func ytdlpUpdateMarkerPath(ytdlpPath string) string {
	return ytdlpPath + ".last-update-check"
}

func shouldCheckYtDlp(markerPath string, now time.Time) bool {
	data, err := os.ReadFile(markerPath)
	if err != nil {
		return true
	}

	checkedAt, err := time.Parse(time.RFC3339Nano, strings.TrimSpace(string(data)))
	if err != nil || now.Before(checkedAt) {
		return true
	}
	return now.Sub(checkedAt) >= ytdlpUpdateInterval
}

func markYtDlpChecked(markerPath string, now time.Time) {
	_ = os.WriteFile(markerPath, []byte(now.UTC().Format(time.RFC3339Nano)), 0644)
}
