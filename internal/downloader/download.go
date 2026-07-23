package downloader

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"video-downloader-go/internal/procutil"
	"video-downloader-go/internal/utils"
)

var ErrCancelled = errors.New("cancelled")

// ErrShutdown is the cancellation cause internal/manager.Shutdown passes to
// a job's context (via context.CancelCauseFunc) to mean "the whole app is
// exiting", as opposed to an ordinary user cancel. runAttempt checks
// context.Cause(ctx) for this and skips straight to killTree instead of
// cancelTree's graceful wait: once the app is gone there's no one left to
// observe a multi-second grace period, and waiting one out here would risk
// an orphaned yt-dlp/ffmpeg process outliving the window entirely.
var ErrShutdown = errors.New("app shutdown")

// cancelGracePeriod is how long a user-initiated cancel (CancelJob/
// CancelAll) waits for yt-dlp/ffmpeg to exit on their own after being asked
// nicely -- see cancelTree in process_unix.go/process_windows.go -- before
// escalating to killTree's unconditional hard kill.
const cancelGracePeriod = 2 * time.Second

// signinRe mirrors worker_process.py's _SIGNIN_RE.
var signinRe = regexp.MustCompile(`\b[Ss]ign in\b|--username`)

type ProgressEvent struct {
	Status          string
	DownloadedBytes int64 // -1 = unknown
	TotalBytes      int64 // -1 = unknown
	Speed           float64
	ETA             float64
	PlaylistIndex   int // 0 = not applicable
	NEntries        int // 0 = unknown
}

// Params mirrors the `params` dict built in download_manager.py's
// _start_download and consumed by worker_process.run_download.
type Params struct {
	URL              string
	Mode             string // "video" | "audio"
	Resolution       int
	ConvertTo        string
	OutputDir        string
	FileName         string
	IsPlaylist       bool
	DownloadPlaylist bool
}

// Callbacks lets the manager react to what's happening inside the
// subprocess without the downloader package knowing anything about jobs,
// Wails events, or the UI. RequestLogin/RequestPassword must block until the
// user responds AND must return promptly (ok=false) once ctx is done —
// otherwise a cancel during an auth prompt would leak the goroutine.
type Callbacks struct {
	OnProgress      func(ProgressEvent)
	OnStage         func(stage string)
	RequestLogin    func(ctx context.Context) (username, password string, ok bool)
	RequestPassword func(ctx context.Context) (password string, ok bool)
}

type credentials struct {
	username      string
	password      string
	videoPassword string
}

// authState persists across retry attempts within one Download call,
// mirroring worker_process.py: a single _AuthInterceptingLogger instance is
// reused across the `while True` retry loop, so only one credential prompt
// is ever offered per job — a second "sign in" failure after bad
// credentials just errors out rather than prompting again.
type authState struct {
	triggered bool
	creds     credentials
}

// Download mirrors worker_process.run_download: builds and runs yt-dlp,
// retrying once with credentials if the site demands a login or a video
// password. Returns the output directory on success, ErrCancelled if ctx
// was cancelled, or an error describing what went wrong.
func Download(ctx context.Context, ytdlpPath string, params Params, cb Callbacks) (string, error) {
	if utils.FFmpegLocation() == "" {
		return "", errors.New(utils.FFmpegMissingMessage())
	}

	auth := &authState{}
	for {
		retry, err := runAttempt(ctx, ytdlpPath, params, auth, cb)
		if err != nil {
			return "", err
		}
		if retry {
			continue
		}
		return params.OutputDir, nil
	}
}

func runAttempt(ctx context.Context, ytdlpPath string, params Params, auth *authState, cb Callbacks) (retry bool, err error) {
	args := buildArgs(params, auth.creds)

	// destFile collects yt-dlp's own byte-accurate report of each video's
	// destination filename (see cleanupTracker's doc comment for why this
	// can't just be parsed from stdout for titles with characters illegal
	// in a Windows filename). --print-to-file implies --simulate unless
	// told otherwise, hence --no-simulate right alongside it.
	destFile, err := os.CreateTemp("", "video-downloader-dest-*.txt")
	if err != nil {
		return false, err
	}
	destFile.Close()
	defer os.Remove(destFile.Name())
	args = append(args, "--print-to-file", "before_dl:%(filename)s", destFile.Name(), "--no-simulate")

	cmd := exec.Command(ytdlpPath, args...)
	procutil.SetProcAttrs(cmd)

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return false, err
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return false, err
	}
	if err := cmd.Start(); err != nil {
		return false, err
	}

	var stderrMu sync.Mutex
	var stderrLines []string
	retrySignal := false

	var leftovers cleanupTracker
	stdoutDone := make(chan struct{})
	go func() {
		defer close(stdoutDone)
		scanner := bufio.NewScanner(stdoutPipe)
		scanner.Buffer(make([]byte, 64*1024), 1<<20)
		for scanner.Scan() {
			line := scanner.Text()
			handleStdoutLine(line, cb)
			trackCleanupCandidate(&leftovers, line)
		}
	}()

	stderrDone := make(chan struct{})
	go func() {
		defer close(stderrDone)
		scanner := bufio.NewScanner(stderrPipe)
		scanner.Buffer(make([]byte, 64*1024), 1<<20)
		for scanner.Scan() {
			line := scanner.Text()
			stderrMu.Lock()
			stderrLines = append(stderrLines, line)
			stderrMu.Unlock()

			if auth.triggered {
				continue
			}
			isVideoPassword := strings.Contains(line, "--video-password")
			isSignin := !isVideoPassword && signinRe.MatchString(line)
			if !isVideoPassword && !isSignin {
				continue
			}
			auth.triggered = true

			if isVideoPassword {
				password, ok := cb.RequestPassword(ctx)
				if ok && password != "" {
					auth.creds.videoPassword = password
					retrySignal = true
					killTree(cmd.Process.Pid)
				}
			} else {
				username, password, ok := cb.RequestLogin(ctx)
				if ok && (username != "" || password != "") {
					auth.creds.username, auth.creds.password = username, password
					retrySignal = true
					killTree(cmd.Process.Pid)
				}
			}
		}
	}()

	waitErr := make(chan error, 1)
	go func() { waitErr <- cmd.Wait() }()

	var runErr error
	select {
	case <-ctx.Done():
		if errors.Is(context.Cause(ctx), ErrShutdown) {
			killTree(cmd.Process.Pid)
			<-waitErr
		} else {
			cancelTree(cmd, waitErr, cancelGracePeriod)
		}
		<-stdoutDone
		<-stderrDone
		// A small/fast job can still exit 0 despite the cancel signal --
		// cancelTree's soft-kill step only asks it to stop, it doesn't
		// force an abort, so a job that was seconds from finishing anyway
		// can complete normally before the signal has any effect. When
		// that happens, trust yt-dlp's own success and leave its output
		// alone rather than deleting a finished download just because the
		// cancel and the natural finish landed at nearly the same moment.
		if cmd.ProcessState == nil || !cmd.ProcessState.Success() {
			leftovers.cleanup(destFile.Name(), params.ConvertTo)
		}
		return false, ErrCancelled
	case runErr = <-waitErr:
		<-stdoutDone
		<-stderrDone
	}

	if retrySignal {
		return true, nil
	}
	if runErr != nil {
		stderrMu.Lock()
		msg := lastErrorLine(strings.Join(stderrLines, "\n"))
		stderrMu.Unlock()
		return false, fmt.Errorf("%s", msg)
	}
	return false, nil
}

func buildOuttmpl(params Params) string {
	name := params.FileName
	if name == "" {
		name = "%(title)s"
	}
	if params.IsPlaylist && params.DownloadPlaylist {
		return filepath.Join(params.OutputDir, name+" - %(playlist_index)s.%(ext)s")
	}
	return filepath.Join(params.OutputDir, name+".%(ext)s")
}

// buildArgs mirrors the ydl_opts construction in worker_process.run_download.
func buildArgs(params Params, creds credentials) []string {
	args := []string{
		"--newline",
		"--no-warnings",
		"--socket-timeout", "30",
		"--retries", "10",
		"--fragment-retries", "10",
		"-o", buildOuttmpl(params),
		// One JSON object per progress tick; `|0` keeps playlist_index/
		// n_entries valid JSON when absent (they otherwise render as the
		// bare, non-JSON token `NA` and break parsing of the whole line).
		"--progress-template", `download:{"progress":%(progress)j,"playlist_index":%(info.playlist_index|0)d,"n_entries":%(info.n_entries|0)d}`,
		"--progress-template", "postprocess:CONVERTING",
	}
	if ffmpeg := utils.FFmpegLocation(); ffmpeg != "" {
		args = append(args, "--ffmpeg-location", ffmpeg)
	}

	if params.Mode == "audio" {
		args = append(args, "-f", "bestaudio/best")
	} else {
		height := params.Resolution
		if height <= 0 {
			height = utils.MaxResolution
		}
		args = append(args, "--format-sort", fmt.Sprintf("res~%d", height))
	}

	if params.ConvertTo != "" && params.ConvertTo != "original" {
		if params.Mode == "audio" {
			args = append(args, "-x", "--audio-format", params.ConvertTo)
		} else {
			args = append(args, "--recode-video", params.ConvertTo)
		}
	}

	if !(params.IsPlaylist && params.DownloadPlaylist) {
		args = append(args, "--no-playlist")
	}

	if creds.username != "" || creds.password != "" {
		args = append(args, "--username", creds.username, "--password", creds.password)
	}
	if creds.videoPassword != "" {
		args = append(args, "--video-password", creds.videoPassword)
	}

	args = append(args, params.URL)
	return args
}

type progressLine struct {
	Progress struct {
		Status             string   `json:"status"`
		DownloadedBytes    *int64   `json:"downloaded_bytes"`
		TotalBytes         *int64   `json:"total_bytes"`
		TotalBytesEstimate *int64   `json:"total_bytes_estimate"`
		Speed              *float64 `json:"speed"`
		ETA                *float64 `json:"eta"`
	} `json:"progress"`
	PlaylistIndex int `json:"playlist_index"`
	NEntries      int `json:"n_entries"`
}

func (p progressLine) toEvent() ProgressEvent {
	ev := ProgressEvent{
		Status:          p.Progress.Status,
		DownloadedBytes: -1,
		TotalBytes:      -1,
		Speed:           -1,
		ETA:             -1,
		PlaylistIndex:   p.PlaylistIndex,
		NEntries:        p.NEntries,
	}
	if p.Progress.DownloadedBytes != nil {
		ev.DownloadedBytes = *p.Progress.DownloadedBytes
	}
	if p.Progress.TotalBytes != nil {
		ev.TotalBytes = *p.Progress.TotalBytes
	} else if p.Progress.TotalBytesEstimate != nil {
		ev.TotalBytes = *p.Progress.TotalBytesEstimate
	}
	if p.Progress.Speed != nil {
		ev.Speed = *p.Progress.Speed
	}
	if p.Progress.ETA != nil {
		ev.ETA = *p.Progress.ETA
	}
	return ev
}

// handleStdoutLine mirrors worker_process.py's on_progress/on_pp hooks.
// yt-dlp's --progress-template only prints the *template body*, not the
// "download:"/"postprocess:" type prefix used to select it, so lines are
// told apart by content: a bare "CONVERTING" marker vs. a JSON object.
func handleStdoutLine(line string, cb Callbacks) {
	line = strings.TrimSpace(line)
	if line == "" {
		return
	}
	if line == "CONVERTING" {
		if cb.OnStage != nil {
			cb.OnStage("converting")
		}
		return
	}
	if !strings.HasPrefix(line, "{") {
		return
	}
	var pl progressLine
	if err := json.Unmarshal([]byte(line), &pl); err != nil {
		return
	}
	if pl.Progress.Status != "downloading" && pl.Progress.Status != "finished" {
		return
	}
	if cb.OnProgress != nil {
		cb.OnProgress(pl.toEvent())
	}
}
