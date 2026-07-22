package manager

import (
	"context"
	"errors"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"video-downloader-go/internal/job"
	"video-downloader-go/internal/utils"
)

// testYtdlpPath is a synthetic bundled yt-dlp path (has a directory
// component, unlike the bare "yt-dlp" PATH-fallback case) built with
// filepath.Join so tests behave the same on every OS.
var testYtdlpPath = filepath.Join("opt", "app", "bin", "yt-dlp")

// sandboxSettingsDir redirects settings.Load/Save's target directory to a
// temp dir for the duration of the test, so tests that stamp a refresh
// timestamp (which calls Settings.Save) never touch the real user's config
// directory. os.UserConfigDir() reads one of these env vars depending on OS.
func sandboxSettingsDir(t *testing.T) {
	t.Helper()
	dir := t.TempDir()
	t.Setenv("AppData", dir)         // windows
	t.Setenv("XDG_CONFIG_HOME", dir) // linux
	t.Setenv("HOME", dir)            // darwin fallback
}

var errNotOnPath = errors.New("not found on PATH")

// fakeUpdateChecker fakes every network/filesystem/PATH interaction
// updateChecker abstracts, so tests never hit GitHub, the real filesystem,
// or the real PATH.
type fakeUpdateChecker struct {
	installedYtdlp    string
	installedErr      error
	installedCalled   bool
	latestYtdlp       string
	latestErr         error
	downloadErr       error
	downloadedVersion string
	downloadedDest    string
	refreshErr        error
	refreshCalled     bool
	refreshBinDir     string

	bundledPaths    map[string]string // binary name -> resolved path; absent/empty means not found
	lookPathResults map[string]string // binary name -> resolved PATH location; absent means not on PATH
	preferredBinDir string
}

func (f *fakeUpdateChecker) InstalledYtdlpVersion(ctx context.Context, ytdlpPath string) (string, error) {
	f.installedCalled = true
	return f.installedYtdlp, f.installedErr
}

func (f *fakeUpdateChecker) LatestYtdlpVersion(ctx context.Context) (string, error) {
	return f.latestYtdlp, f.latestErr
}

func (f *fakeUpdateChecker) DownloadYtdlp(ctx context.Context, version, dest string) error {
	f.downloadedVersion = version
	f.downloadedDest = dest
	return f.downloadErr
}

func (f *fakeUpdateChecker) RefreshFfmpeg(ctx context.Context, binDir string) (string, error) {
	f.refreshCalled = true
	f.refreshBinDir = binDir
	return "8.1", f.refreshErr
}

func (f *fakeUpdateChecker) ResolveBundledPath(name string) string {
	return f.bundledPaths[name]
}

func (f *fakeUpdateChecker) LookPath(name string) (string, error) {
	if path, ok := f.lookPathResults[name]; ok {
		return path, nil
	}
	return "", errNotOnPath
}

func (f *fakeUpdateChecker) PreferredBinDir() string {
	return f.preferredBinDir
}

// capturedEmit records every event/payload m.emit is called with.
type capturedEmit struct {
	events []string
	last   interface{}
}

func (c *capturedEmit) fn() func(event string, data ...interface{}) {
	return func(event string, data ...interface{}) {
		c.events = append(c.events, event)
		if len(data) > 0 {
			c.last = data[0]
		}
	}
}

// -- checkYtdlpUpdate: bundled copy present --

func TestCheckYtdlpUpdateEmitsWhenVersionsDiffer(t *testing.T) {
	m := New(testYtdlpPath)
	checker := &fakeUpdateChecker{
		bundledPaths:   map[string]string{utils.YtdlpBinaryName(): testYtdlpPath},
		installedYtdlp: "2026.07.04",
		latestYtdlp:    "2026.07.20",
	}
	m.updateChecker = checker
	emit := &capturedEmit{}
	m.emit = emit.fn()

	m.checkYtdlpUpdate()

	if len(emit.events) != 1 || emit.events[0] != "update:ytdlp-available" {
		t.Fatalf("emitted events = %v, want exactly [update:ytdlp-available]", emit.events)
	}
	prompt, ok := emit.last.(UpdatePrompt)
	if !ok {
		t.Fatalf("emitted payload = %#v, want UpdatePrompt", emit.last)
	}
	if prompt.Missing {
		t.Error("prompt.Missing = true, want false (bundled copy is present, just outdated)")
	}
	if prompt.CurrentVersion != "2026.07.04" || prompt.LatestVersion != "2026.07.20" {
		t.Fatalf("unexpected prompt: %+v", prompt)
	}
	if m.ytdlpUpdate == nil {
		t.Fatal("expected m.ytdlpUpdate to be set")
	}
}

func TestCheckYtdlpUpdateNoEmitWhenUpToDate(t *testing.T) {
	m := New(testYtdlpPath)
	m.updateChecker = &fakeUpdateChecker{
		bundledPaths:   map[string]string{utils.YtdlpBinaryName(): testYtdlpPath},
		installedYtdlp: "2026.07.20",
		latestYtdlp:    "2026.07.20",
	}
	emit := &capturedEmit{}
	m.emit = emit.fn()

	m.checkYtdlpUpdate()

	if len(emit.events) != 0 {
		t.Fatalf("emitted events = %v, want none when already up to date", emit.events)
	}
	if m.ytdlpUpdate != nil {
		t.Fatalf("m.ytdlpUpdate = %+v, want nil", m.ytdlpUpdate)
	}
}

func TestCheckYtdlpUpdateNoEmitOnCheckerError(t *testing.T) {
	m := New(testYtdlpPath)
	m.updateChecker = &fakeUpdateChecker{
		bundledPaths:   map[string]string{utils.YtdlpBinaryName(): testYtdlpPath},
		installedYtdlp: "2026.07.04",
		latestErr:      errors.New("offline"),
	}
	emit := &capturedEmit{}
	m.emit = emit.fn()

	m.checkYtdlpUpdate()

	if len(emit.events) != 0 {
		t.Fatalf("emitted events = %v, want none on checker error (fail silently)", emit.events)
	}
}

// -- checkYtdlpUpdate: no bundled copy --

func TestCheckYtdlpUpdateSkipsWhenPathCopyWorks(t *testing.T) {
	m := New(testYtdlpPath)
	checker := &fakeUpdateChecker{
		lookPathResults: map[string]string{utils.YtdlpBinaryName(): filepath.Join("usr", "bin", "yt-dlp")},
		installedYtdlp:  "2026.07.04",
		latestYtdlp:     "2026.07.20",
	}
	m.updateChecker = checker

	m.checkYtdlpUpdate()

	if checker.installedCalled {
		t.Error("expected InstalledYtdlpVersion not to be called when a working PATH copy exists")
	}
	if m.ytdlpUpdate != nil {
		t.Fatalf("m.ytdlpUpdate = %+v, want nil", m.ytdlpUpdate)
	}
}

func TestCheckYtdlpUpdatePromptsWhenMissing(t *testing.T) {
	m := New(testYtdlpPath)
	binDir := filepath.Join("opt", "app", "bin")
	checker := &fakeUpdateChecker{preferredBinDir: binDir, latestYtdlp: "2026.07.20"}
	m.updateChecker = checker
	emit := &capturedEmit{}
	m.emit = emit.fn()

	m.checkYtdlpUpdate()

	if len(emit.events) != 1 || emit.events[0] != "update:ytdlp-available" {
		t.Fatalf("emitted events = %v, want exactly [update:ytdlp-available]", emit.events)
	}
	prompt, ok := emit.last.(UpdatePrompt)
	if !ok {
		t.Fatalf("emitted payload = %#v, want UpdatePrompt", emit.last)
	}
	if !prompt.Missing {
		t.Error("prompt.Missing = false, want true")
	}
	if prompt.LatestVersion != "2026.07.20" {
		t.Errorf("prompt.LatestVersion = %q, want %q", prompt.LatestVersion, "2026.07.20")
	}
	wantPath := filepath.Join(binDir, utils.YtdlpBinaryName())
	if m.ytdlpPath != wantPath {
		t.Errorf("m.ytdlpPath = %q, want %q (so a later ConfirmUpdate downloads to the right place)", m.ytdlpPath, wantPath)
	}
}

func TestCheckYtdlpUpdateMissingNoEmitWhenNoBinDirKnown(t *testing.T) {
	m := New(testYtdlpPath)
	checker := &fakeUpdateChecker{preferredBinDir: "", latestYtdlp: "2026.07.20"}
	m.updateChecker = checker
	emit := &capturedEmit{}
	m.emit = emit.fn()

	m.checkYtdlpUpdate()

	if len(emit.events) != 0 {
		t.Fatalf("emitted events = %v, want none when no bin dir is known", emit.events)
	}
}

func TestCheckYtdlpUpdateMissingNoEmitOnCheckerError(t *testing.T) {
	m := New(testYtdlpPath)
	checker := &fakeUpdateChecker{
		preferredBinDir: filepath.Join("opt", "app", "bin"),
		latestErr:       errors.New("offline"),
	}
	m.updateChecker = checker
	emit := &capturedEmit{}
	m.emit = emit.fn()

	m.checkYtdlpUpdate()

	if len(emit.events) != 0 {
		t.Fatalf("emitted events = %v, want none on checker error (fail silently)", emit.events)
	}
}

// -- checkFfmpegRefresh: both binaries present --

func ffmpegBundledPaths(binDir string) map[string]string {
	return map[string]string{
		utils.FfmpegBinaryName():  filepath.Join(binDir, utils.FfmpegBinaryName()),
		utils.FfprobeBinaryName(): filepath.Join(binDir, utils.FfprobeBinaryName()),
	}
}

func TestCheckFfmpegRefreshRecentNoPrompt(t *testing.T) {
	m := New(testYtdlpPath)
	m.updateChecker = &fakeUpdateChecker{bundledPaths: ffmpegBundledPaths(filepath.Join("opt", "app", "bin"))}
	m.settingsData.LastFfmpegRefresh = time.Now().Add(-24 * time.Hour).UTC().Format(time.RFC3339)
	emit := &capturedEmit{}
	m.emit = emit.fn()

	m.checkFfmpegRefresh()

	if len(emit.events) != 0 {
		t.Fatalf("emitted events = %v, want none when refreshed recently", emit.events)
	}
	if m.ffmpegUpdate != nil {
		t.Fatalf("m.ffmpegUpdate = %+v, want nil", m.ffmpegUpdate)
	}
}

func TestCheckFfmpegRefreshOverduePrompts(t *testing.T) {
	m := New(testYtdlpPath)
	m.updateChecker = &fakeUpdateChecker{bundledPaths: ffmpegBundledPaths(filepath.Join("opt", "app", "bin"))}
	m.settingsData.LastFfmpegRefresh = time.Now().Add(-31 * 24 * time.Hour).UTC().Format(time.RFC3339)
	emit := &capturedEmit{}
	m.emit = emit.fn()

	m.checkFfmpegRefresh()

	if len(emit.events) != 1 || emit.events[0] != "update:ffmpeg-available" {
		t.Fatalf("emitted events = %v, want exactly [update:ffmpeg-available]", emit.events)
	}
	prompt, ok := emit.last.(UpdatePrompt)
	if !ok {
		t.Fatalf("emitted payload = %#v, want UpdatePrompt", emit.last)
	}
	if prompt.Missing {
		t.Error("prompt.Missing = true, want false (both binaries are present, just overdue)")
	}
	if m.ffmpegUpdate == nil {
		t.Fatal("expected m.ffmpegUpdate to be set")
	}
}

func TestCheckFfmpegRefreshFirstObservationStampsWithoutPrompt(t *testing.T) {
	sandboxSettingsDir(t)
	m := New(testYtdlpPath)
	m.updateChecker = &fakeUpdateChecker{bundledPaths: ffmpegBundledPaths(filepath.Join("opt", "app", "bin"))}
	m.settingsData.LastFfmpegRefresh = ""
	emit := &capturedEmit{}
	m.emit = emit.fn()

	m.checkFfmpegRefresh()

	if len(emit.events) != 0 {
		t.Fatalf("emitted events = %v, want none on first-ever observation", emit.events)
	}
	if m.settingsData.LastFfmpegRefresh == "" {
		t.Fatal("expected LastFfmpegRefresh to be stamped")
	}
	if _, err := time.Parse(time.RFC3339, m.settingsData.LastFfmpegRefresh); err != nil {
		t.Fatalf("LastFfmpegRefresh = %q is not a valid RFC3339 timestamp: %v", m.settingsData.LastFfmpegRefresh, err)
	}
}

// -- checkFfmpegRefresh: missing --

func TestCheckFfmpegRefreshSkipsWhenPathCopyWorks(t *testing.T) {
	m := New(testYtdlpPath)
	checker := &fakeUpdateChecker{
		lookPathResults: map[string]string{"ffmpeg": filepath.Join("usr", "bin", "ffmpeg")},
	}
	m.updateChecker = checker
	emit := &capturedEmit{}
	m.emit = emit.fn()

	m.checkFfmpegRefresh()

	if len(emit.events) != 0 {
		t.Fatalf("emitted events = %v, want none when a working PATH copy exists", emit.events)
	}
	if m.ffmpegUpdate != nil {
		t.Fatalf("m.ffmpegUpdate = %+v, want nil", m.ffmpegUpdate)
	}
}

func TestCheckFfmpegRefreshPromptsWhenMissing(t *testing.T) {
	m := New(testYtdlpPath)
	checker := &fakeUpdateChecker{}
	m.updateChecker = checker
	emit := &capturedEmit{}
	m.emit = emit.fn()

	m.checkFfmpegRefresh()

	if len(emit.events) != 1 || emit.events[0] != "update:ffmpeg-available" {
		t.Fatalf("emitted events = %v, want exactly [update:ffmpeg-available]", emit.events)
	}
	prompt, ok := emit.last.(UpdatePrompt)
	if !ok {
		t.Fatalf("emitted payload = %#v, want UpdatePrompt", emit.last)
	}
	if !prompt.Missing {
		t.Error("prompt.Missing = false, want true")
	}
}

func TestCheckFfmpegRefreshPromptsWhenOnlyFfprobeMissing(t *testing.T) {
	m := New(testYtdlpPath)
	checker := &fakeUpdateChecker{
		bundledPaths: map[string]string{utils.FfmpegBinaryName(): filepath.Join("opt", "app", "bin", utils.FfmpegBinaryName())},
	}
	m.updateChecker = checker
	emit := &capturedEmit{}
	m.emit = emit.fn()

	m.checkFfmpegRefresh()

	if len(emit.events) != 1 || emit.events[0] != "update:ffmpeg-available" {
		t.Fatalf("emitted events = %v, want exactly [update:ffmpeg-available] when only ffprobe is missing", emit.events)
	}
}

// -- ConfirmUpdate: ytdlp --

func TestConfirmUpdateDeclineDoesNotDownload(t *testing.T) {
	m := New(testYtdlpPath)
	checker := &fakeUpdateChecker{}
	m.updateChecker = checker
	m.ytdlpUpdate = &UpdatePrompt{Kind: "ytdlp", CurrentVersion: "2026.07.04", LatestVersion: "2026.07.20"}

	if err := m.ConfirmUpdate("ytdlp", false); err != nil {
		t.Fatalf("ConfirmUpdate(decline) error = %v", err)
	}
	if checker.downloadedVersion != "" {
		t.Errorf("DownloadYtdlp was called with version %q, want no call", checker.downloadedVersion)
	}
	if m.ytdlpUpdate != nil {
		t.Fatal("expected m.ytdlpUpdate to be cleared even on decline")
	}
}

func TestConfirmUpdateYtdlpAcceptDownloadsCorrectVersionAndDest(t *testing.T) {
	m := New(testYtdlpPath)
	checker := &fakeUpdateChecker{}
	m.updateChecker = checker
	m.ytdlpUpdate = &UpdatePrompt{Kind: "ytdlp", CurrentVersion: "2026.07.04", LatestVersion: "2026.07.20"}

	if err := m.ConfirmUpdate("ytdlp", true); err != nil {
		t.Fatalf("ConfirmUpdate(accept) error = %v", err)
	}
	if checker.downloadedVersion != "2026.07.20" {
		t.Errorf("downloaded version = %q, want %q", checker.downloadedVersion, "2026.07.20")
	}
	if checker.downloadedDest != testYtdlpPath {
		t.Errorf("downloaded dest = %q, want %q", checker.downloadedDest, testYtdlpPath)
	}
	if m.ytdlpUpdate != nil {
		t.Fatal("expected m.ytdlpUpdate to be cleared after confirming")
	}
}

func TestConfirmUpdateYtdlpPropagatesDownloadError(t *testing.T) {
	m := New(testYtdlpPath)
	wantErr := errors.New("disk full")
	m.updateChecker = &fakeUpdateChecker{downloadErr: wantErr}
	m.ytdlpUpdate = &UpdatePrompt{Kind: "ytdlp", LatestVersion: "2026.07.20"}

	err := m.ConfirmUpdate("ytdlp", true)
	if !errors.Is(err, wantErr) {
		t.Fatalf("ConfirmUpdate() error = %v, want %v", err, wantErr)
	}
}

// TestConfirmUpdateYtdlpMissingDownloadsToPreferredBinDir exercises the full
// missing-binary flow end to end: promptYtdlpMissing repoints m.ytdlpPath at
// the preferred bin/ dir, and ConfirmUpdate then downloads to exactly that
// path with no further plumbing needed.
func TestConfirmUpdateYtdlpMissingDownloadsToPreferredBinDir(t *testing.T) {
	m := New("yt-dlp") // starting value is irrelevant; promptYtdlpMissing overwrites it
	binDir := filepath.Join("opt", "app", "bin")
	checker := &fakeUpdateChecker{preferredBinDir: binDir, latestYtdlp: "2026.07.20"}
	m.updateChecker = checker

	m.promptYtdlpMissing(checker, utils.YtdlpBinaryName())

	wantDest := filepath.Join(binDir, utils.YtdlpBinaryName())
	if m.ytdlpPath != wantDest {
		t.Fatalf("m.ytdlpPath = %q, want %q", m.ytdlpPath, wantDest)
	}

	if err := m.ConfirmUpdate("ytdlp", true); err != nil {
		t.Fatalf("ConfirmUpdate(accept) error = %v", err)
	}
	if checker.downloadedDest != wantDest {
		t.Errorf("downloaded dest = %q, want %q", checker.downloadedDest, wantDest)
	}
	if checker.downloadedVersion != "2026.07.20" {
		t.Errorf("downloaded version = %q, want %q", checker.downloadedVersion, "2026.07.20")
	}
}

// -- ConfirmUpdate: ffmpeg --

func TestConfirmUpdateFfmpegAcceptRefreshesAndStamps(t *testing.T) {
	sandboxSettingsDir(t)
	m := New(testYtdlpPath)
	binDir := filepath.Join("opt", "app", "bin")
	checker := &fakeUpdateChecker{preferredBinDir: binDir}
	m.updateChecker = checker
	m.ffmpegUpdate = &UpdatePrompt{Kind: "ffmpeg"}

	if err := m.ConfirmUpdate("ffmpeg", true); err != nil {
		t.Fatalf("ConfirmUpdate(accept) error = %v", err)
	}
	if !checker.refreshCalled {
		t.Fatal("expected RefreshFfmpeg to be called")
	}
	if checker.refreshBinDir != binDir {
		t.Errorf("refresh binDir = %q, want %q", checker.refreshBinDir, binDir)
	}
	if m.settingsData.LastFfmpegRefresh == "" {
		t.Error("expected LastFfmpegRefresh to be stamped after a successful refresh")
	}
	if m.ffmpegUpdate != nil {
		t.Fatal("expected m.ffmpegUpdate to be cleared after confirming")
	}
}

func TestConfirmUpdateFfmpegSkipsStampOnRefreshError(t *testing.T) {
	sandboxSettingsDir(t)
	m := New(testYtdlpPath)
	m.updateChecker = &fakeUpdateChecker{
		preferredBinDir: filepath.Join("opt", "app", "bin"),
		refreshErr:      errors.New("network error"),
	}
	m.ffmpegUpdate = &UpdatePrompt{Kind: "ffmpeg"}

	if err := m.ConfirmUpdate("ffmpeg", true); err == nil {
		t.Fatal("ConfirmUpdate() error = nil, want the refresh error")
	}
	if m.settingsData.LastFfmpegRefresh != "" {
		t.Error("expected LastFfmpegRefresh not to be stamped when the refresh failed")
	}
}

func TestConfirmUpdateFfmpegErrorsWhenNoBinDirKnown(t *testing.T) {
	m := New(testYtdlpPath)
	checker := &fakeUpdateChecker{preferredBinDir: ""}
	m.updateChecker = checker
	m.ffmpegUpdate = &UpdatePrompt{Kind: "ffmpeg"}

	if err := m.ConfirmUpdate("ffmpeg", true); err == nil {
		t.Fatal("ConfirmUpdate() error = nil, want an error when no bundled bin dir is known")
	}
	if checker.refreshCalled {
		t.Error("expected RefreshFfmpeg not to be called when no bundled bin dir is known")
	}
}

// -- concurrency --

// TestNoDataRaceBetweenJobStartAndMissingBinaryUpdate guards against
// reintroducing the race jobs.go's startFetch/startDownload/RequestPreview
// used to have: m.ytdlpPath was write-once before promptYtdlpMissing started
// mutating it at runtime, so their old pattern of reading m.ytdlpPath from
// inside a goroutine spawned after m.mu.Unlock() (safe only while the field
// was immutable) became a real race. Run with -race: a job starting
// concurrently with a missing-binary prompt being resolved must never touch
// m.ytdlpPath outside the lock.
func TestNoDataRaceBetweenJobStartAndMissingBinaryUpdate(t *testing.T) {
	m := New(testYtdlpPath)
	checker := &fakeUpdateChecker{preferredBinDir: filepath.Join("opt", "app", "bin"), latestYtdlp: "2026.07.20"}
	m.updateChecker = checker

	j := job.New("https://example.com/video", job.ModeVideo, 1080, "original", t.TempDir(), "%(title)s")
	m.mu.Lock()
	m.jobs = append(m.jobs, j)
	m.mu.Unlock()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		m.startFetch(j)
	}()
	go func() {
		defer wg.Done()
		m.promptYtdlpMissing(checker, utils.YtdlpBinaryName())
	}()
	wg.Wait()
}
