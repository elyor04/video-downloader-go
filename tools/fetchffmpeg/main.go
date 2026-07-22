// Command fetchffmpeg downloads the latest available static ffmpeg +
// ffprobe binaries for the current platform into bin/ if they aren't
// already there, mirroring tools/fetchytdlp. bin/ is gitignored, and this
// is wired into wails.json's frontend:install so a fresh clone fetches
// everything automatically on first build.
//
// There's no pinned version to bump here -- this always asks upstream
// what's currently latest, same as the app's own runtime auto-updater
// (internal/manager/updates.go). To pick up a newer build on an existing
// checkout, run: go run ./tools/fetchffmpeg -force.
//
// The actual fetch/extract mechanics (including which upstream build
// provider is used per platform, and why) live in internal/updater, shared
// with that runtime auto-updater; this file just owns the CLI's
// -force/skip-if-present behavior.
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
	force := flag.Bool("force", false, "re-download even if the bin/ copies already exist")
	flag.Parse()

	repoRoot, err := repoRootDir()
	if err != nil {
		fail(err)
	}
	binDir := filepath.Join(repoRoot, "bin")
	ffmpegDest := filepath.Join(binDir, utils.FfmpegBinaryName())
	ffprobeDest := filepath.Join(binDir, utils.FfprobeBinaryName())

	if !*force {
		_, ffmpegErr := os.Stat(ffmpegDest)
		_, ffprobeErr := os.Stat(ffprobeDest)
		if ffmpegErr == nil && ffprobeErr == nil {
			fmt.Printf("fetchffmpeg: %s and %s already present, skipping (use -force to re-download)\n", ffmpegDest, ffprobeDest)
			return
		}
	}

	fmt.Println("fetchffmpeg: resolving latest ffmpeg build...")
	version, err := updater.RefreshFfmpeg(context.Background(), binDir)
	if err != nil {
		fail(err)
	}
	fmt.Printf("fetchffmpeg: fetched ffmpeg %s -> %s\n", version, binDir)
}

// repoRootDir locates the module root from this source file's own location,
// mirroring tools/fetchytdlp's repoRootDir (see there for why: cwd varies
// depending on how this is invoked).
func repoRootDir() (string, error) {
	_, thisFile, _, ok := runtime.Caller(0)
	if !ok {
		return "", fmt.Errorf("fetchffmpeg: could not determine source location")
	}
	return filepath.Dir(filepath.Dir(filepath.Dir(thisFile))), nil
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fetchffmpeg:", err)
	os.Exit(1)
}
