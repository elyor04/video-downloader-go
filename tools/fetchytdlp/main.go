// Command fetchytdlp downloads the latest available yt-dlp release for the
// current platform into bin/ if it isn't already there. bin/ is gitignored
// (an 18MB+ binary doesn't belong in the repo), so this replaces committing
// it: it's wired into wails.json's frontend:install, which runs before
// every `wails dev`/`wails build`, so a fresh clone fetches it
// automatically on first build.
//
// There's no pinned version to bump here -- this always asks GitHub what's
// currently latest, same as the app's own runtime auto-updater
// (internal/manager/updates.go). To pick up a newer release on an existing
// checkout, run: go run ./tools/fetchytdlp -force.
//
// The actual fetch mechanics live in internal/updater, shared with that
// runtime auto-updater; this file just owns the CLI's -force/skip-if-present
// behavior.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"video-downloader-go/internal/updater"
	"video-downloader-go/internal/utils"
)

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

	ctx := context.Background()
	fmt.Println("fetchytdlp: resolving latest yt-dlp release...")
	version, err := updater.LatestYtdlpVersion(ctx)
	if err != nil {
		fail(err)
	}

	fmt.Printf("fetchytdlp: downloading yt-dlp %s -> %s\n", version, dest)
	if err := updater.DownloadYtdlp(ctx, version, dest); err != nil {
		fail(err)
	}
	fmt.Println("fetchytdlp: done")
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

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fetchytdlp:", err)
	os.Exit(1)
}
