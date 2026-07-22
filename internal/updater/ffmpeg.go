package updater

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"

	"video-downloader-go/internal/utils"
)

// RefreshFfmpeg always resolves and downloads whatever's currently the
// latest available ffmpeg + ffprobe build for the current platform into
// binDir, overwriting whatever is already there, and returns the resolved
// version string. There's no pinned/hardcoded version anywhere -- both
// tools/fetchffmpeg's dev/build-time fetch and the app's own runtime
// time-gated refresh (internal/manager/updates.go) always ask upstream
// what's newest.
//
// FFmpeg itself doesn't publish official prebuilt binaries, so this pulls
// from two well-established third-party build providers:
//
//   - Windows/Linux: BtbN/FFmpeg-Builds (github.com/BtbN/FFmpeg-Builds), GPL
//     variant (includes libx264/libx265, matching what winget/brew/apt
//     ship). Served from the repo's rolling "latest" release tag, which
//     carries several concurrently maintained stable-branch version lines
//     at once (e.g. both n7.1 and n8.1) under a hash-free filename that
//     always tracks that line's current build -- LatestFfmpegVersionLine
//     picks the highest version line published there. There's no immutable
//     per-build tag like yt-dlp's version releases, so even re-resolving
//     "the latest line" on every call can still land a newer build within
//     the same line than what's already on disk.
//   - macOS: evermeet.cx, a long-running static-build service exposing a
//     small JSON "info" API (see fetchEvermeetInfo) that directly reports
//     the current version and download URL -- no version needs to be
//     guessed or pinned. Only the Intel (x86_64) build is fetched; it runs
//     fine on Apple Silicon under Rosetta 2.
func RefreshFfmpeg(ctx context.Context, binDir string) (string, error) {
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", err
	}
	ffmpegDest := filepath.Join(binDir, utils.FfmpegBinaryName())
	ffprobeDest := filepath.Join(binDir, utils.FfprobeBinaryName())

	switch runtime.GOOS {
	case "windows", "linux":
		return fetchBtbN(ctx, binDir, ffmpegDest, ffprobeDest)
	case "darwin":
		return fetchEvermeet(ctx, ffmpegDest, ffprobeDest)
	default:
		return "", fmt.Errorf("updater: unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

// InstalledFfmpegVersionLine runs the bundled ffmpeg binary's -version flag
// and returns its first line, for display purposes only -- BtbN's
// git-describe-style version strings for a running binary aren't directly
// comparable to LatestFfmpegVersionLine's "8.1"-style version line, so this
// is never used to decide whether a refresh is needed (see
// internal/manager/updates.go's time-gated ffmpegRefreshInterval instead).
func InstalledFfmpegVersionLine(ctx context.Context, ffmpegPath string) (string, error) {
	cmd := exec.CommandContext(ctx, ffmpegPath, "-version")
	setProcAttrs(cmd)
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	lines := strings.SplitN(string(out), "\n", 2)
	return strings.TrimSpace(lines[0]), nil
}

// btbnPlatform maps GOARCH to BtbN's platform slug for the current GOOS.
func btbnPlatform() (string, error) {
	return btbnPlatformFor(runtime.GOOS, runtime.GOARCH)
}

// btbnPlatformFor is btbnPlatform's pure implementation, taking GOOS/GOARCH
// as parameters so it's table-testable for platforms other than the one
// running the test.
func btbnPlatformFor(goos, goarch string) (string, error) {
	var amd64Name, arm64Name string
	if goos == "windows" {
		amd64Name, arm64Name = "win64", "winarm64"
	} else {
		amd64Name, arm64Name = "linux64", "linuxarm64"
	}
	switch goarch {
	case "amd64":
		return amd64Name, nil
	case "arm64":
		return arm64Name, nil
	default:
		return "", fmt.Errorf("updater: BtbN only publishes amd64/arm64 builds, got %s", goarch)
	}
}

// btbnLatestReleaseURL is a var (not a const) so tests can point it at an
// httptest.Server instead of the real GitHub API.
var btbnLatestReleaseURL = "https://api.github.com/repos/BtbN/FFmpeg-Builds/releases/tags/latest"

// btbnAssetPattern matches a full (non-shared) GPL build's release asset
// name, e.g. "ffmpeg-n8.1-latest-win64-gpl-8.1.zip" or
// "...-linux64-gpl-8.1.tar.xz". Deliberately does NOT match the "-gpl-
// shared-" or "-lgpl-" variants BtbN also publishes: the literal "-gpl-"
// segment must be immediately followed by digits, which fails to match
// "-gpl-shared-...", and the platform capture group excludes "-", which
// stops it from absorbing "l" out of "-lgpl-" to fake an alignment.
var btbnAssetPattern = regexp.MustCompile(`^ffmpeg-n(\d+)\.(\d+)-latest-([a-z0-9]+)-gpl-\d+\.\d+\.(?:zip|tar\.xz)$`)

// LatestFfmpegVersionLine asks BtbN's rolling "latest" GitHub release for
// the highest stable ffmpeg version line it currently publishes a full GPL
// build of for this platform. BtbN's "latest" tag carries several
// concurrently maintained version lines at once (e.g. both n7.1 and n8.1),
// so "latest" here means the newest version *line*, not the newest file.
func LatestFfmpegVersionLine(ctx context.Context) (string, error) {
	platform, err := btbnPlatform()
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, btbnLatestReleaseURL, nil)
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

	var release struct {
		Assets []struct {
			Name string `json:"name"`
		} `json:"assets"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", err
	}

	bestMajor, bestMinor := -1, -1
	for _, asset := range release.Assets {
		m := btbnAssetPattern.FindStringSubmatch(asset.Name)
		if m == nil || m[3] != platform {
			continue
		}
		major, _ := strconv.Atoi(m[1])
		minor, _ := strconv.Atoi(m[2])
		if major > bestMajor || (major == bestMajor && minor > bestMinor) {
			bestMajor, bestMinor = major, minor
		}
	}
	if bestMajor < 0 {
		return "", fmt.Errorf("updater: no ffmpeg GPL build found for %s in BtbN's latest release", platform)
	}
	return fmt.Sprintf("%d.%d", bestMajor, bestMinor), nil
}

// fetchBtbN resolves the current latest version line, downloads its GPL
// archive for this platform, and extracts just bin/ffmpeg(.exe) and
// bin/ffprobe(.exe) from it.
func fetchBtbN(ctx context.Context, binDir, ffmpegDest, ffprobeDest string) (string, error) {
	platform, err := btbnPlatform()
	if err != nil {
		return "", err
	}
	version, err := LatestFfmpegVersionLine(ctx)
	if err != nil {
		return "", err
	}
	base := fmt.Sprintf("ffmpeg-n%s-latest-%s-gpl-%s", version, platform, version)
	ext := ".zip"
	if runtime.GOOS == "linux" {
		ext = ".tar.xz"
	}
	asset := base + ext
	url := fmt.Sprintf("https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/%s", asset)

	archivePath := filepath.Join(binDir, asset+".download")
	defer os.Remove(archivePath)
	if err := downloadFile(ctx, url, archivePath); err != nil {
		return "", err
	}

	want := map[string]string{
		base + "/bin/" + utils.FfmpegBinaryName():  ffmpegDest,
		base + "/bin/" + utils.FfprobeBinaryName(): ffprobeDest,
	}
	if runtime.GOOS == "linux" {
		if err := extractFromTarXz(archivePath, want); err != nil {
			return "", err
		}
	} else if err := extractFromZip(archivePath, want); err != nil {
		return "", err
	}
	return version, nil
}

// evermeetInfoBaseURL is a var (not a const) so tests can point it at an
// httptest.Server instead of the real evermeet.cx API.
var evermeetInfoBaseURL = "https://evermeet.cx/ffmpeg/info"

// evermeetRelease mirrors just the fields fetchEvermeetInfo needs out of
// evermeet.cx's JSON "info" API response.
type evermeetRelease struct {
	Version  string `json:"version"`
	Download struct {
		Zip struct {
			URL string `json:"url"`
		} `json:"zip"`
	} `json:"download"`
}

// fetchEvermeetInfo asks evermeet.cx which version of `name` ("ffmpeg" or
// "ffprobe") is current and where to download it -- no version needs to be
// guessed or pinned, unlike BtbN's rolling release.
func fetchEvermeetInfo(ctx context.Context, name string) (evermeetRelease, error) {
	url := evermeetInfoBaseURL + "/" + name + "/release"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return evermeetRelease{}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return evermeetRelease{}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return evermeetRelease{}, fmt.Errorf("updater: unexpected status from evermeet.cx: %s", resp.Status)
	}

	var release evermeetRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return evermeetRelease{}, err
	}
	if release.Download.Zip.URL == "" {
		return evermeetRelease{}, fmt.Errorf("updater: evermeet.cx info for %s has no zip download URL", name)
	}
	return release, nil
}

// fetchEvermeet downloads evermeet.cx's separate ffmpeg and ffprobe
// archives, each a flat zip containing a single executable at its root.
func fetchEvermeet(ctx context.Context, ffmpegDest, ffprobeDest string) (string, error) {
	version, err := fetchEvermeetOne(ctx, "ffmpeg", ffmpegDest)
	if err != nil {
		return "", err
	}
	if _, err := fetchEvermeetOne(ctx, "ffprobe", ffprobeDest); err != nil {
		return "", err
	}
	return version, nil
}

func fetchEvermeetOne(ctx context.Context, name, dest string) (string, error) {
	info, err := fetchEvermeetInfo(ctx, name)
	if err != nil {
		return "", err
	}
	archivePath := dest + ".zip.download"
	defer os.Remove(archivePath)
	if err := downloadFile(ctx, info.Download.Zip.URL, archivePath); err != nil {
		return "", err
	}
	if err := extractFromZip(archivePath, map[string]string{name: dest}); err != nil {
		return "", err
	}
	return info.Version, nil
}

// extractFromZip pulls each entry named in `want` (archive path -> local
// destination) out of a zip file, erroring if any are missing.
func extractFromZip(archivePath string, want map[string]string) error {
	r, err := zip.OpenReader(archivePath)
	if err != nil {
		return err
	}
	defer r.Close()

	found := make(map[string]bool, len(want))
	for _, f := range r.File {
		dest, ok := want[f.Name]
		if !ok {
			continue
		}
		if err := extractZipEntry(f, dest); err != nil {
			return err
		}
		found[f.Name] = true
	}
	for name := range want {
		if !found[name] {
			return fmt.Errorf("updater: %s not found in %s", name, archivePath)
		}
	}
	return nil
}

func extractZipEntry(f *zip.File, dest string) error {
	rc, err := f.Open()
	if err != nil {
		return err
	}
	defer rc.Close()

	tmp := dest + ".download"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, rc); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

// extractFromTarXz shells out to the system `tar` (universal on Linux, and
// the only stdlib-free way to decode .tar.xz without adding an xz
// dependency) to unpack into a temp dir, then copies just the wanted files
// out of it.
func extractFromTarXz(archivePath string, want map[string]string) error {
	tmpDir, err := os.MkdirTemp("", "updater-ffmpeg-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("tar", "-xJf", archivePath, "-C", tmpDir)
	setProcAttrs(cmd)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("updater: tar extraction failed (requires system `tar` with xz support): %w", err)
	}

	for src, dest := range want {
		srcPath := filepath.Join(tmpDir, filepath.FromSlash(src))
		if err := copyToDest(srcPath, dest); err != nil {
			return err
		}
	}
	return nil
}

func copyToDest(src, dest string) error {
	in, err := os.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	tmp := dest + ".download"
	out, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
	if err != nil {
		return err
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		os.Remove(tmp)
		return err
	}
	if err := out.Close(); err != nil {
		os.Remove(tmp)
		return err
	}
	return os.Rename(tmp, dest)
}

// downloadFile is a plain (non-atomic) fetch: its destination is always a
// throwaway ".download" archive that the caller removes once extraction
// finishes, so there's no partial-file risk to guard against the way
// extractZipEntry/copyToDest do for the final binaries.
func downloadFile(ctx context.Context, url, dest string) error {
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

	f, err := os.Create(dest)
	if err != nil {
		return err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		f.Close()
		os.Remove(dest)
		return err
	}
	return f.Close()
}
