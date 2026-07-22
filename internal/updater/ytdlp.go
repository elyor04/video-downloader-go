// Package updater holds the fetch/extract mechanics shared by the build-time
// tools/fetchytdlp and tools/fetchffmpeg CLIs and the running app's own
// startup auto-updater (internal/manager/updates.go). The CLIs own their
// pinned version constants and flag/printing behavior; this package only
// knows how to ask "what's the latest yt-dlp version" and how to fetch a
// given version/build into a destination path.
package updater

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"runtime"
	"strings"
)

// YtdlpAssetName maps the current platform to the standalone yt-dlp binary
// published under that name in yt-dlp's GitHub release assets. Only the
// platforms this app is packaged for (via build/darwin, build/windows) are
// covered; anything else fails loudly rather than silently grabbing the
// wrong executable.
func YtdlpAssetName() (string, error) {
	return ytdlpAssetNameFor(runtime.GOOS, runtime.GOARCH)
}

// ytdlpAssetNameFor is YtdlpAssetName's pure implementation, taking GOOS/GOARCH
// as parameters so it's table-testable for platforms other than the one
// running the test.
func ytdlpAssetNameFor(goos, goarch string) (string, error) {
	switch goos {
	case "windows":
		switch goarch {
		case "arm64":
			return "yt-dlp_arm64.exe", nil
		case "386":
			return "yt-dlp_x86.exe", nil
		default:
			return "yt-dlp.exe", nil
		}
	case "darwin":
		return "yt-dlp_macos", nil // universal2: Intel + Apple Silicon
	case "linux":
		if goarch == "arm64" {
			return "yt-dlp_linux_aarch64", nil
		}
		return "yt-dlp_linux", nil
	default:
		return "", fmt.Errorf("updater: unsupported platform %s/%s", goos, goarch)
	}
}

// latestYtdlpRelease mirrors just the field of GitHub's release API response
// LatestYtdlpVersion needs.
type latestYtdlpRelease struct {
	TagName string `json:"tag_name"`
}

// ytdlpLatestReleaseURL is a var (not a const) so tests can point it at an
// httptest.Server instead of the real GitHub API.
var ytdlpLatestReleaseURL = "https://api.github.com/repos/yt-dlp/yt-dlp/releases/latest"

// LatestYtdlpVersion asks GitHub for yt-dlp's latest published release tag
// (yt-dlp's version string doubles as its release tag, e.g. "2026.07.20").
func LatestYtdlpVersion(ctx context.Context) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, ytdlpLatestReleaseURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "video-downloader-go")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("updater: unexpected status from GitHub: %s", resp.Status)
	}

	var release latestYtdlpRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}
	return release.TagName, nil
}

// InstalledYtdlpVersion runs the bundled yt-dlp binary's --version flag to
// find out what's actually on disk, rather than trusting any pinned constant
// -- the user (or a previous update) may have replaced it independently.
func InstalledYtdlpVersion(ctx context.Context, ytdlpPath string) (string, error) {
	cmd := exec.CommandContext(ctx, ytdlpPath, "--version")
	setProcAttrs(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// DownloadYtdlp fetches the given yt-dlp release version's asset for the
// current platform into dest, mirroring tools/fetchytdlp's download step
// (temp file + rename so an interrupted download never leaves a corrupt
// binary in place).
func DownloadYtdlp(ctx context.Context, version, dest string) error {
	asset, err := YtdlpAssetName()
	if err != nil {
		return err
	}
	url := fmt.Sprintf("https://github.com/yt-dlp/yt-dlp/releases/download/%s/%s", version, asset)
	return downloadAtomic(ctx, url, dest)
}

// downloadAtomic writes url's body to dest via a temp file + rename, so an
// interrupted download never leaves a corrupt binary behind, then marks it
// executable on non-Windows (GitHub release assets download without the
// execute bit; exec.Command refuses to run a non-executable file).
func downloadAtomic(ctx context.Context, url, dest string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("updater: unexpected status: %s", resp.Status)
	}

	tmp := dest + ".download"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(tmp)
		return err
	}
	if err := f.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmp, 0o755); err != nil {
			os.Remove(tmp)
			return err
		}
	}
	return os.Rename(tmp, dest)
}
