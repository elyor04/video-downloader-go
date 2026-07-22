package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

// TestMain lets tests re-exec the test binary itself as a fake yt-dlp/ffmpeg
// process (the standard os/exec testing idiom): when UPDATER_TEST_HELPER is
// set, print canned output and exit immediately instead of running tests.
func TestMain(m *testing.M) {
	switch os.Getenv("UPDATER_TEST_HELPER") {
	case "ytdlp":
		fmt.Print("2026.07.20")
		os.Exit(0)
	case "ffmpeg":
		fmt.Print("ffmpeg version 8.1-full_build-www.gyan.dev\nbuilt with gcc\n")
		os.Exit(0)
	}
	os.Exit(m.Run())
}

func TestYtdlpAssetNameFor(t *testing.T) {
	cases := []struct {
		goos, goarch string
		want         string
		wantErr      bool
	}{
		{"windows", "amd64", "yt-dlp.exe", false},
		{"windows", "arm64", "yt-dlp_arm64.exe", false},
		{"windows", "386", "yt-dlp_x86.exe", false},
		{"darwin", "amd64", "yt-dlp_macos", false},
		{"darwin", "arm64", "yt-dlp_macos", false},
		{"linux", "amd64", "yt-dlp_linux", false},
		{"linux", "arm64", "yt-dlp_linux_aarch64", false},
		{"plan9", "amd64", "", true},
	}
	for _, c := range cases {
		got, err := ytdlpAssetNameFor(c.goos, c.goarch)
		if c.wantErr {
			if err == nil {
				t.Errorf("ytdlpAssetNameFor(%q, %q) = %q, want error", c.goos, c.goarch, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("ytdlpAssetNameFor(%q, %q) unexpected error: %v", c.goos, c.goarch, err)
			continue
		}
		if got != c.want {
			t.Errorf("ytdlpAssetNameFor(%q, %q) = %q, want %q", c.goos, c.goarch, got, c.want)
		}
	}
}

func TestLatestYtdlpVersion(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(latestYtdlpRelease{TagName: "2026.07.20"})
	}))
	defer srv.Close()

	orig := ytdlpLatestReleaseURL
	ytdlpLatestReleaseURL = srv.URL
	defer func() { ytdlpLatestReleaseURL = orig }()

	got, err := LatestYtdlpVersion(context.Background())
	if err != nil {
		t.Fatalf("LatestYtdlpVersion() error = %v", err)
	}
	if got != "2026.07.20" {
		t.Errorf("LatestYtdlpVersion() = %q, want %q", got, "2026.07.20")
	}
}

func TestLatestYtdlpVersionNonOKStatus(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	defer srv.Close()

	orig := ytdlpLatestReleaseURL
	ytdlpLatestReleaseURL = srv.URL
	defer func() { ytdlpLatestReleaseURL = orig }()

	if _, err := LatestYtdlpVersion(context.Background()); err == nil {
		t.Error("LatestYtdlpVersion() error = nil, want non-nil for a 403 response")
	}
}

func TestInstalledYtdlpVersion(t *testing.T) {
	t.Setenv("UPDATER_TEST_HELPER", "ytdlp")
	got, err := InstalledYtdlpVersion(context.Background(), os.Args[0])
	if err != nil {
		t.Fatalf("InstalledYtdlpVersion() error = %v", err)
	}
	if got != "2026.07.20" {
		t.Errorf("InstalledYtdlpVersion() = %q, want %q", got, "2026.07.20")
	}
}

// TestDownloadAtomicCreatesMissingDestDir guards against a regression of the
// bug where a fresh install's bin/ directory doesn't exist yet (nothing has
// ever downloaded into it before): downloadAtomic must create dest's parent
// directory itself rather than assuming the caller already did, since
// internal/manager/updates.go's missing-binary flow points dest at
// utils.PreferredBinDir(), which explicitly documents that it never creates
// anything.
func TestDownloadAtomicCreatesMissingDestDir(t *testing.T) {
	const body = "fake yt-dlp binary contents"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(body))
	}))
	defer srv.Close()

	dest := filepath.Join(t.TempDir(), "nested", "bin", "yt-dlp") // parent dirs don't exist yet
	if err := downloadAtomic(context.Background(), srv.URL, dest); err != nil {
		t.Fatalf("downloadAtomic() error = %v, want nil even when dest's parent directory doesn't exist yet", err)
	}

	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatalf("ReadFile(%s) error = %v", dest, err)
	}
	if string(got) != body {
		t.Errorf("downloaded content = %q, want %q", got, body)
	}
}
