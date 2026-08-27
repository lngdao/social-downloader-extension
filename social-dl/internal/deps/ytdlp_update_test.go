package deps

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestShouldCheckYtDlp(t *testing.T) {
	now := time.Date(2026, time.August, 27, 12, 0, 0, 0, time.UTC)
	marker := filepath.Join(t.TempDir(), "last-update-check")

	if !shouldCheckYtDlp(marker, now) {
		t.Fatal("expected a missing marker to trigger an update check")
	}

	markYtDlpChecked(marker, now)
	if shouldCheckYtDlp(marker, now.Add(23*time.Hour)) {
		t.Fatal("did not expect an update check within the 24-hour interval")
	}
	if !shouldCheckYtDlp(marker, now.Add(24*time.Hour)) {
		t.Fatal("expected an update check after the 24-hour interval")
	}
}

func TestShouldCheckYtDlp_InvalidMarker(t *testing.T) {
	marker := filepath.Join(t.TempDir(), "last-update-check")
	if err := os.WriteFile(marker, []byte("not-a-timestamp"), 0644); err != nil {
		t.Fatal(err)
	}

	if !shouldCheckYtDlp(marker, time.Now()) {
		t.Fatal("expected an invalid marker to trigger an update check")
	}
}
