package manager

import (
	"context"
	"fmt"
	"os/exec"
	"path/filepath"
	"time"

	"video-downloader-go/internal/updater"
	"video-downloader-go/internal/utils"
)

// ffmpegRefreshInterval is how long bin/ffmpeg + bin/ffprobe are left alone
// before checkFfmpegRefresh offers to re-pull them. Even though RefreshFfmpeg
// always resolves the latest version line (see internal/updater.RefreshFfmpeg's
// doc), BtbN's rolling release can still mutate a line's build without its
// version string changing, so there's no reliable way to diff versions for
// these -- this is a time gate rather than a version check.
const ffmpegRefreshInterval = 30 * 24 * time.Hour

// updateCheckTimeout bounds the network/subprocess calls CheckForUpdates
// makes, so a hung connection can never delay startup or block forever.
const updateCheckTimeout = 10 * time.Second

// updateDownloadTimeout bounds ConfirmUpdate's actual download -- longer,
// since the ffmpeg archive is 100MB+.
const updateDownloadTimeout = 5 * time.Minute

// UpdatePrompt is emitted as update:ytdlp-available / update:ffmpeg-available
// and answered via ConfirmUpdate.
type UpdatePrompt struct {
	Kind           string `json:"kind"`    // "ytdlp" | "ffmpeg"
	Missing        bool   `json:"missing"` // true if not found anywhere (vs. present but outdated)
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"` // "" for the ffmpeg time-gated/missing cases
}

// updateChecker abstracts the internal/updater calls and filesystem/PATH
// lookups CheckForUpdates and ConfirmUpdate make, so tests can substitute
// fakes instead of hitting GitHub, the real filesystem, or PATH.
type updateChecker interface {
	InstalledYtdlpVersion(ctx context.Context, ytdlpPath string) (string, error)
	LatestYtdlpVersion(ctx context.Context) (string, error)
	DownloadYtdlp(ctx context.Context, version, dest string) error
	RefreshFfmpeg(ctx context.Context, binDir string) (string, error)
	// ResolveBundledPath, LookPath and PreferredBinDir mirror the
	// internal/utils functions of the same name/shape (LookPath mirrors
	// os/exec.LookPath).
	ResolveBundledPath(name string) string
	LookPath(name string) (string, error)
	PreferredBinDir() string
}

type realUpdateChecker struct{}

func (realUpdateChecker) InstalledYtdlpVersion(ctx context.Context, ytdlpPath string) (string, error) {
	return updater.InstalledYtdlpVersion(ctx, ytdlpPath)
}

func (realUpdateChecker) LatestYtdlpVersion(ctx context.Context) (string, error) {
	return updater.LatestYtdlpVersion(ctx)
}

func (realUpdateChecker) DownloadYtdlp(ctx context.Context, version, dest string) error {
	return updater.DownloadYtdlp(ctx, version, dest)
}

func (realUpdateChecker) RefreshFfmpeg(ctx context.Context, binDir string) (string, error) {
	return updater.RefreshFfmpeg(ctx, binDir)
}

func (realUpdateChecker) ResolveBundledPath(name string) string {
	return utils.ResolveBundledPath(name)
}

func (realUpdateChecker) LookPath(name string) (string, error) {
	return exec.LookPath(name)
}

func (realUpdateChecker) PreferredBinDir() string {
	return utils.PreferredBinDir()
}

// CheckForUpdates kicks off the yt-dlp and ffmpeg checks in the background.
// Called once from main.go's OnStartup; must never block app startup, so
// both checks run in their own goroutines and fail silently on any
// network/subprocess error -- for an outdated-but-present binary this is a
// background nicety, not a user-facing operation. A binary that can't be
// found anywhere is different (the app can't function at all without one),
// but still can't block startup, so it's surfaced the same way: an
// update:*-available prompt the user answers via ConfirmUpdate.
func (m *Manager) CheckForUpdates() {
	go m.checkYtdlpUpdate()
	go m.checkFfmpegRefresh()
}

// checkYtdlpUpdate handles both yt-dlp cases under one roof: if a bundled
// copy exists, check whether GitHub has a newer release; if not, and there's
// no working PATH copy either, offer to fetch one fresh.
func (m *Manager) checkYtdlpUpdate() {
	m.mu.Lock()
	checker := m.updateChecker
	m.mu.Unlock()

	name := utils.YtdlpBinaryName()
	if bundled := checker.ResolveBundledPath(name); bundled != "" {
		m.checkYtdlpNewerVersion(checker, bundled)
		return
	}
	if _, err := checker.LookPath(name); err == nil {
		return // user's own PATH copy already works; nothing to manage
	}
	m.promptYtdlpMissing(checker, name)
}

// checkYtdlpNewerVersion compares an already-present bundled yt-dlp's own
// --version output against GitHub's latest release tag every startup, per
// the "every time it should check" requirement -- there's no persisted
// state to gate this one.
func (m *Manager) checkYtdlpNewerVersion(checker updateChecker, ytdlpPath string) {
	ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
	defer cancel()

	current, err := checker.InstalledYtdlpVersion(ctx, ytdlpPath)
	if err != nil {
		return
	}
	latest, err := checker.LatestYtdlpVersion(ctx)
	if err != nil || latest == "" || latest == current {
		return
	}

	prompt := UpdatePrompt{Kind: "ytdlp", CurrentVersion: current, LatestVersion: latest}
	m.mu.Lock()
	m.ytdlpUpdate = &prompt
	m.mu.Unlock()
	m.emit("update:ytdlp-available", prompt)
}

// promptYtdlpMissing offers to fetch yt-dlp fresh into bin/ when neither a
// bundled copy nor a PATH copy can be found. Points m.ytdlpPath at the
// target the download would land at (the same path ConfirmUpdate's ytdlp
// branch then downloads to), so a successful download is immediately usable
// by subsequent jobs with no further resolution step.
func (m *Manager) promptYtdlpMissing(checker updateChecker, name string) {
	binDir := checker.PreferredBinDir()
	if binDir == "" {
		return
	}
	dest := filepath.Join(binDir, name)

	ctx, cancel := context.WithTimeout(context.Background(), updateCheckTimeout)
	defer cancel()
	latest, err := checker.LatestYtdlpVersion(ctx)
	if err != nil || latest == "" {
		return
	}

	prompt := UpdatePrompt{Kind: "ytdlp", Missing: true, LatestVersion: latest}
	m.mu.Lock()
	m.ytdlpPath = dest
	m.ytdlpUpdate = &prompt
	m.mu.Unlock()
	m.emit("update:ytdlp-available", prompt)
}

// checkFfmpegRefresh handles both ffmpeg cases under one roof: if ffmpeg or
// ffprobe can't be found at all (and there's no working PATH copy either),
// offer to fetch them fresh; otherwise fall back to the existing time-gated
// refresh once both are confirmed present.
func (m *Manager) checkFfmpegRefresh() {
	m.mu.Lock()
	checker := m.updateChecker
	m.mu.Unlock()

	if checker.ResolveBundledPath(utils.FfmpegBinaryName()) == "" || checker.ResolveBundledPath(utils.FfprobeBinaryName()) == "" {
		if _, err := checker.LookPath("ffmpeg"); err == nil {
			return // utils.FFmpegLocation() already falls back to PATH; app already works
		}
		m.promptFfmpegMissing()
		return
	}

	m.mu.Lock()
	last := m.settingsData.LastFfmpegRefresh
	m.mu.Unlock()

	if last == "" {
		m.stampFfmpegRefreshed()
		return
	}
	if t, err := time.Parse(time.RFC3339, last); err == nil && time.Since(t) < ffmpegRefreshInterval {
		return
	}

	prompt := UpdatePrompt{Kind: "ffmpeg"}
	m.mu.Lock()
	m.ffmpegUpdate = &prompt
	m.mu.Unlock()
	m.emit("update:ffmpeg-available", prompt)
}

// promptFfmpegMissing offers to fetch ffmpeg/ffprobe fresh into bin/ when
// they can't be found anywhere.
func (m *Manager) promptFfmpegMissing() {
	prompt := UpdatePrompt{Kind: "ffmpeg", Missing: true}
	m.mu.Lock()
	m.ffmpegUpdate = &prompt
	m.mu.Unlock()
	m.emit("update:ffmpeg-available", prompt)
}

// stampFfmpegRefreshed records "now" as the last-refreshed time without
// prompting -- used both for the first-ever observation in
// checkFfmpegRefresh and after ConfirmUpdate successfully refreshes ffmpeg.
func (m *Manager) stampFfmpegRefreshed() {
	m.mu.Lock()
	m.settingsData.LastFfmpegRefresh = time.Now().UTC().Format(time.RFC3339)
	s := m.settingsData
	m.mu.Unlock()
	_ = s.Save()
}

// ConfirmUpdate answers a pending update:ytdlp-available/update:ffmpeg-available
// prompt -- whether it's an outdated-version offer or a missing-binary offer
// makes no difference here, both download the same way. It runs
// synchronously (like BrowseOutputDir) so the frontend can await it directly
// and show a spinner for the download's duration; Wails turns a non-nil
// error return into a rejected JS promise, so callers don't need a separate
// completion/failure event.
func (m *Manager) ConfirmUpdate(kind string, accept bool) error {
	m.mu.Lock()
	var prompt *UpdatePrompt
	switch kind {
	case "ytdlp":
		prompt = m.ytdlpUpdate
		m.ytdlpUpdate = nil
	case "ffmpeg":
		prompt = m.ffmpegUpdate
		m.ffmpegUpdate = nil
	}
	ytdlpPath := m.ytdlpPath
	checker := m.updateChecker
	m.mu.Unlock()

	if !accept || prompt == nil {
		return nil
	}

	ctx, cancel := context.WithTimeout(context.Background(), updateDownloadTimeout)
	defer cancel()

	switch kind {
	case "ytdlp":
		return checker.DownloadYtdlp(ctx, prompt.LatestVersion, ytdlpPath)
	case "ffmpeg":
		binDir := checker.PreferredBinDir()
		if binDir == "" {
			return fmt.Errorf("manager: no bundled binaries directory known to refresh ffmpeg into")
		}
		if _, err := checker.RefreshFfmpeg(ctx, binDir); err != nil {
			return err
		}
		m.stampFfmpegRefreshed()
		return nil
	default:
		return fmt.Errorf("manager: unknown update kind %q", kind)
	}
}
