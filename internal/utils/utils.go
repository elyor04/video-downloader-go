// Package utils holds small platform/formatting helpers shared across the backend.
package utils

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
)

const MaxResolution = 65535

const MaxConcurrentDownloads = 5

// ResolutionOption is one entry in the resolution ladder. Label is an
// i18next key the frontend resolves to a translated string; numeric labels
// like "1080p" are rendered directly from Value and need no translation.
type ResolutionOption struct {
	Value int    `json:"value"`
	Label string `json:"label"`
}

var ResolutionLadder = []ResolutionOption{
	{MaxResolution, "resolution.best"},
	{4320, "4320p (8K)"},
	{2160, "2160p (4K)"},
	{1440, "1440p"},
	{1080, "1080p"},
	{720, "720p"},
	{480, "480p"},
	{360, "360p"},
	{240, "240p"},
	{144, "144p"},
}

var VideoConvertOptions = []string{"original", "mp4", "mkv", "webm"}
var AudioConvertOptions = []string{"original", "mp3", "m4a", "wav"}

// YtdlpBinaryName returns the name of the yt-dlp executable this platform's
// bin/ directory holds, matching the asset tools/fetchytdlp downloads for it.
func YtdlpBinaryName() string {
	if runtime.GOOS == "windows" {
		return "yt-dlp.exe"
	}
	return "yt-dlp"
}

// FfmpegBinaryName and FfprobeBinaryName return the names of the ffmpeg/
// ffprobe executables this platform's bin/ directory holds, matching the
// assets tools/fetchffmpeg downloads for it.
func FfmpegBinaryName() string {
	if runtime.GOOS == "windows" {
		return "ffmpeg.exe"
	}
	return "ffmpeg"
}

func FfprobeBinaryName() string {
	if runtime.GOOS == "windows" {
		return "ffprobe.exe"
	}
	return "ffprobe"
}

// ResolveBundledPath looks for `name` in, in order: bin/ next to the
// running executable (the packaged-build layout -- see
// build/windows/installer/project.nsi, which installs yt-dlp/ffmpeg/ffprobe
// into $INSTDIR\bin -- resolved via os.Executable() rather than the
// process's working directory, so it's reliable regardless of how the app
// was launched); directly next to the executable (flat-layout fallback);
// ./bin relative to the working directory (the dev-mode layout
// tools/fetchytdlp and tools/fetchffmpeg populate, where cwd is the project
// root); directly under the working directory (flat-layout fallback). No
// current build of this project produces a flat layout, but the fallbacks
// are cheap insurance for layouts this function doesn't yet know about
// (e.g. a future macOS package). Returns "" if it's in none of those
// places.
func ResolveBundledPath(name string) string {
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		if candidate := filepath.Join(exeDir, "bin", name); fileExists(candidate) {
			return candidate
		}
		if candidate := filepath.Join(exeDir, name); fileExists(candidate) {
			return candidate
		}
	}
	if wd, err := os.Getwd(); err == nil {
		if candidate := filepath.Join(wd, "bin", name); fileExists(candidate) {
			return candidate
		}
		if candidate := filepath.Join(wd, name); fileExists(candidate) {
			return candidate
		}
	}
	return ""
}

func fileExists(path string) bool {
	_, err := os.Stat(path)
	return err == nil
}

// PreferredBinDir returns the bin/ directory a missing bundled binary
// should be downloaded into: next to the running executable in a packaged
// build, or under the working directory in dev mode. Mirrors
// ResolveBundledPath's first two search roots, but doesn't require anything
// to already exist there -- it's the write-side counterpart used by
// internal/manager/updates.go when yt-dlp/ffmpeg/ffprobe can't be found
// anywhere. Returns "" if neither os.Executable() nor os.Getwd() succeed.
func PreferredBinDir() string {
	if exe, err := os.Executable(); err == nil {
		return filepath.Join(filepath.Dir(exe), "bin")
	}
	if wd, err := os.Getwd(); err == nil {
		return filepath.Join(wd, "bin")
	}
	return ""
}

// FFmpegLocation mirrors utils.ffmpeg_location(): shutil.which("ffmpeg"),
// but first prefers the copy tools/fetchffmpeg bundled, so the app doesn't
// depend on the user having installed ffmpeg themselves.
func FFmpegLocation() string {
	if path := ResolveBundledPath(FfmpegBinaryName()); path != "" {
		return path
	}
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return ""
	}
	return path
}

// FFmpegMissingMessage mirrors utils.ffmpeg_missing_message() verbatim,
// including staying untranslated — the Python original never ran this
// through tr() either, so job.error_message carries this exact English text
// on both sides.
func FFmpegMissingMessage() string {
	message := "FFmpeg is not installed. "
	switch runtime.GOOS {
	case "windows":
		message += "Please run this command: winget install ffmpeg"
	case "darwin":
		message += "Please run this command: brew install ffmpeg"
	case "linux":
		message += "Please run this command: sudo apt install ffmpeg"
	default:
		message += "Please refer to this link: https://ffmpeg.org/download.html"
	}
	return message
}

// CheckDownloadDir mirrors utils.check_download_dir: returns "" if path is a
// writable directory, else a stable message key.
func CheckDownloadDir(path string, create bool) string {
	if create {
		info, err := os.Stat(path)
		if err == nil {
			if !info.IsDir() {
				return "error.notADirectory"
			}
		} else if os.IsNotExist(err) {
			if mkErr := os.MkdirAll(path, 0o755); mkErr != nil {
				return "error.couldNotCreateDirectory"
			}
		} else {
			return "error.couldNotCreateDirectory"
		}
	} else {
		info, err := os.Stat(path)
		if err != nil || !info.IsDir() {
			return "error.notADirectory"
		}
	}

	f, err := os.CreateTemp(path, ".write-test-*")
	if err != nil {
		return "error.permissionDenied"
	}
	name := f.Name()
	f.Close()
	os.Remove(name)
	return ""
}

// OpenInFileManager mirrors utils.open_in_file_manager for each platform.
func OpenInFileManager(path string) error {
	switch runtime.GOOS {
	case "windows":
		return exec.Command("explorer", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}

// FormatBytes mirrors utils.format_bytes. n < 0 means unknown.
func FormatBytes(n float64) string {
	if n < 0 {
		return "?"
	}
	value := n
	for _, unit := range []string{"B", "KB", "MB", "GB", "TB"} {
		if value < 1024 {
			return fmt.Sprintf("%.1f %s", value, unit)
		}
		value /= 1024
	}
	return fmt.Sprintf("%.1f PB", value)
}

// FormatSpeed mirrors utils.format_speed.
func FormatSpeed(bytesPerSec float64) string {
	if bytesPerSec < 0 {
		return "?"
	}
	return FormatBytes(bytesPerSec) + "/s"
}

// FormatEta mirrors utils.format_eta.
func FormatEta(seconds float64) string {
	if seconds < 0 {
		return "?"
	}
	total := int64(seconds)
	hours := total / 3600
	rem := total % 3600
	minutes := rem / 60
	secs := rem % 60
	if hours > 0 {
		return fmt.Sprintf("%d:%02d:%02d", hours, minutes, secs)
	}
	return fmt.Sprintf("%d:%02d", minutes, secs)
}
