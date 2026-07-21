// Command fetchytdlp downloads the pinned yt-dlp release for the current
// platform into bin/ if it isn't already there. bin/ is gitignored (an
// 18MB+ binary doesn't belong in the repo), so this replaces committing it:
// it's wired into wails.json's frontend:install, which runs before every
// `wails dev`/`wails build`, so a fresh clone fetches it automatically on
// first build.
//
// To bump the pinned version, update ytdlpVersion below and delete the
// stale bin/ contents (or run: go run ./tools/fetchytdlp -force).
package main

import (
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"

	"video-downloader-go/internal/utils"
)

const ytdlpVersion = "2026.07.04"

func main() {
	force := flag.Bool("force", false, "re-download even if the bin/ copy already exists")
	flag.Parse()

	repoRoot, err := repoRootDir()
	if err != nil {
		fail(err)
	}
	dest := filepath.Join(repoRoot, "bin", utils.YtdlpBinaryName())

	if !*force {
		if _, err := os.Stat(dest); err == nil {
			fmt.Printf("fetchytdlp: %s already present, skipping (use -force to re-download)\n", dest)
			return
		}
	}

	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		fail(err)
	}

	asset, err := releaseAsset()
	if err != nil {
		fail(err)
	}

	url := fmt.Sprintf("https://github.com/yt-dlp/yt-dlp/releases/download/%s/%s", ytdlpVersion, asset)
	fmt.Printf("fetchytdlp: downloading %s -> %s\n", url, dest)
	if err := download(url, dest); err != nil {
		fail(err)
	}
	fmt.Println("fetchytdlp: done")
}

// releaseAsset maps the current platform to the standalone yt-dlp binary
// published under that name in yt-dlp's GitHub release assets. Only the
// platforms this app is packaged for (via build/darwin, build/windows) are
// covered; anything else fails loudly rather than silently grabbing the
// wrong executable.
func releaseAsset() (string, error) {
	switch runtime.GOOS {
	case "windows":
		switch runtime.GOARCH {
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
		if runtime.GOARCH == "arm64" {
			return "yt-dlp_linux_aarch64", nil
		}
		return "yt-dlp_linux", nil
	default:
		return "", fmt.Errorf("fetchytdlp: unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

// repoRootDir locates the module root from this source file's own location
// (tools/fetchytdlp/main.go), rather than the process's current working
// directory — which varies depending on whether this is invoked directly
// (`go run ./tools/fetchytdlp` from the repo root) or via wails.json's
// frontend:install (cwd is frontend/).
func repoRootDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("fetchytdlp: could not determine source location")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile))), nil
}

// download writes to a temp file and renames into place, so an interrupted
// download never leaves a corrupt yt-dlp binary behind.
func download(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
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
	// GitHub release assets download without the execute bit; exec.Command
	// on darwin/linux refuses to run a non-executable file.
	if runtime.GOOS != "windows" {
		if err := os.Chmod(tmp, 0o755); err != nil {
			os.Remove(tmp)
			return err
		}
	}
	return os.Rename(tmp, dest)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fetchytdlp:", err)
	os.Exit(1)
}
