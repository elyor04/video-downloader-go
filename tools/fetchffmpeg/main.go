// Command fetchffmpeg downloads static ffmpeg + ffprobe binaries for the
// current platform into bin/ if they aren't already there, mirroring
// tools/fetchytdlp. bin/ is gitignored, and this is wired into wails.json's
// frontend:install so a fresh clone fetches everything automatically on
// first build.
//
// Unlike yt-dlp, FFmpeg itself doesn't publish official prebuilt binaries,
// so this pulls from two well-established third-party build providers used
// by many projects for exactly this purpose:
//
//   - Windows/Linux: BtbN/FFmpeg-Builds (github.com/BtbN/FFmpeg-Builds),
//     the "n8.1" stable-branch build, GPL variant (includes libx264/
//     libx265, matching what winget/brew/apt ship — an LGPL build would
//     silently produce worse mp4/mkv recodes). Served from the repo's
//     rolling "latest" release tag under a hash-free filename that always
//     tracks the current n8.1 build; there's no immutable per-build tag
//     like yt-dlp's version releases, so this is pinned to the ffmpeg
//     version line, not an exact immutable artifact.
//   - macOS: evermeet.cx, a long-running static-build service. Its URLs
//     are versioned and immutable, so evermeetBuild pins an exact build.
//     Only the Intel (x86_64) build is fetched; it runs fine on Apple
//     Silicon under Rosetta 2.
//
// Both sources distribute ffmpeg and ffprobe as part of a larger archive
// (BtbN also includes ffplay, headers, and libs; evermeet ships each tool
// as its own archive), so this downloads to a temp file, extracts just the
// two binaries this app needs into bin/, and discards the rest.
package main

import (
	"archive/zip"
	"flag"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"video-downloader-go/internal/utils"
)

// ffmpegVersion pins the BtbN stable-branch build (Windows/Linux).
const ffmpegVersion = "8.1"

// evermeetBuild pins the exact evermeet.cx build (macOS). Check
// https://evermeet.cx/ffmpeg/info/ffmpeg/release for the current version
// when bumping this.
const evermeetBuild = "8.1.2"

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

	if err := os.MkdirAll(binDir, 0o755); err != nil {
		fail(err)
	}

	if err := fetch(binDir, ffmpegDest, ffprobeDest); err != nil {
		fail(err)
	}
	fmt.Println("fetchffmpeg: done")
}

func fetch(binDir, ffmpegDest, ffprobeDest string) error {
	switch runtime.GOOS {
	case "windows", "linux":
		return fetchBtbN(binDir, ffmpegDest, ffprobeDest)
	case "darwin":
		return fetchEvermeet(ffmpegDest, ffprobeDest)
	default:
		return fmt.Errorf("fetchffmpeg: unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

// btbnPlatform maps GOARCH to BtbN's platform slug for the current GOOS.
func btbnPlatform() (string, error) {
	var amd64Name, arm64Name string
	if runtime.GOOS == "windows" {
		amd64Name, arm64Name = "win64", "winarm64"
	} else {
		amd64Name, arm64Name = "linux64", "linuxarm64"
	}
	switch runtime.GOARCH {
	case "amd64":
		return amd64Name, nil
	case "arm64":
		return arm64Name, nil
	default:
		return "", fmt.Errorf("fetchffmpeg: BtbN only publishes amd64/arm64 builds, got %s", runtime.GOARCH)
	}
}

// fetchBtbN downloads the BtbN GPL archive for this platform and extracts
// just bin/ffmpeg(.exe) and bin/ffprobe(.exe) from it.
func fetchBtbN(binDir, ffmpegDest, ffprobeDest string) error {
	platform, err := btbnPlatform()
	if err != nil {
		return err
	}
	base := fmt.Sprintf("ffmpeg-n%s-latest-%s-gpl-%s", ffmpegVersion, platform, ffmpegVersion)
	ext := ".zip"
	if runtime.GOOS == "linux" {
		ext = ".tar.xz"
	}
	asset := base + ext
	url := fmt.Sprintf("https://github.com/BtbN/FFmpeg-Builds/releases/download/latest/%s", asset)

	archivePath := filepath.Join(binDir, asset+".download")
	defer os.Remove(archivePath)
	fmt.Printf("fetchffmpeg: downloading %s\n", url)
	if err := downloadFile(url, archivePath); err != nil {
		return err
	}

	want := map[string]string{
		base + "/bin/" + utils.FfmpegBinaryName():  ffmpegDest,
		base + "/bin/" + utils.FfprobeBinaryName(): ffprobeDest,
	}
	if runtime.GOOS == "linux" {
		return extractFromTarXz(archivePath, want)
	}
	return extractFromZip(archivePath, want)
}

// fetchEvermeet downloads evermeet.cx's separate ffmpeg and ffprobe
// archives, each a flat zip containing a single executable at its root.
func fetchEvermeet(ffmpegDest, ffprobeDest string) error {
	if err := fetchEvermeetOne("ffmpeg", ffmpegDest); err != nil {
		return err
	}
	return fetchEvermeetOne("ffprobe", ffprobeDest)
}

func fetchEvermeetOne(name, dest string) error {
	url := fmt.Sprintf("https://evermeet.cx/ffmpeg/%s-%s.zip", name, evermeetBuild)
	archivePath := dest + ".zip.download"
	defer os.Remove(archivePath)
	fmt.Printf("fetchffmpeg: downloading %s\n", url)
	if err := downloadFile(url, archivePath); err != nil {
		return err
	}
	return extractFromZip(archivePath, map[string]string{name: dest})
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
			return fmt.Errorf("fetchffmpeg: %s not found in %s", name, archivePath)
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
	tmpDir, err := os.MkdirTemp("", "fetchffmpeg-*")
	if err != nil {
		return err
	}
	defer os.RemoveAll(tmpDir)

	cmd := exec.Command("tar", "-xJf", archivePath, "-C", tmpDir)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("fetchffmpeg: tar extraction failed (requires system `tar` with xz support): %w", err)
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

// downloadFile is a plain (non-atomic) fetch: its destination is always a
// throwaway ".download" archive that the caller removes once extraction
// finishes, so there's no partial-file risk to guard against here the way
// extractZipEntry/copyToDest do for the final binaries.
func downloadFile(url, dest string) error {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status: %s", resp.Status)
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

func fail(err error) {
	fmt.Fprintln(os.Stderr, "fetchffmpeg:", err)
	os.Exit(1)
}
