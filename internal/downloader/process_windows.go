//go:build windows

package downloader

import (
	"os"
	"os/exec"
	"strconv"
	"time"

	"video-downloader-go/internal/procutil"
)

// killTree mirrors utils.terminate_process_tree's Windows branch: taskkill
// /T reaches the whole process tree (yt-dlp + any ffmpeg it spawned), /F
// forces a hard kill immediately, no grace period. Used for app shutdown
// (see ErrShutdown in download.go) and the credentials-retry path, where an
// immediate restart matters more than a graceful wind-down of the attempt
// being discarded.
func killTree(pid int) {
	killCmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	procutil.SetProcAttrs(killCmd)
	_ = killCmd.Run()
}

// cancelTree is the user-cancel path. yt-dlp/ffmpeg are spawned in their
// own console process group (internal/procutil.SetProcAttrs sets
// CREATE_NEW_PROCESS_GROUP), so Process.Signal(os.Interrupt) here delivers
// a CTRL_BREAK_EVENT scoped to just that group -- Go's os/exec maps
// os.Interrupt on Windows to GenerateConsoleCtrlEvent(CTRL_BREAK_EVENT,
// pid), which only reaches the target group, not our own (console-less)
// process. yt-dlp's Python runtime turns that into a KeyboardInterrupt like
// it does for POSIX SIGINT, so it still gets a chance to abort more
// cleanly than a hard kill.
//
// Unlike on POSIX, this is NOT confirmed to make the bundled ffmpeg finish
// writing a valid output file: a live test against bin/ffmpeg.exe
// (mid-encode, sent CTRL_BREAK, several presets tried) had it exit within
// ~300ms every time but still leave a "moov atom not found" file, same as
// an immediate killTree. So on Windows this step mainly benefits yt-dlp's
// own shutdown, not file integrity -- it's kept anyway since it's cheap
// (real exit times were a few hundred ms, nowhere near gracePeriod) and
// harmless, with killTree still guaranteed as the backstop.
//
// If the tree hasn't exited on its own within gracePeriod -- or the signal
// couldn't be delivered at all -- it's escalated to killTree's forceful
// taskkill /F /T.
//
// waitErr must be the same channel the caller's
// `go func() { waitErr <- cmd.Wait() }()` sends to -- cancelTree always
// drains exactly one value from it before returning, so the caller never
// has to.
func cancelTree(cmd *exec.Cmd, waitErr <-chan error, gracePeriod time.Duration) {
	if err := cmd.Process.Signal(os.Interrupt); err == nil {
		select {
		case <-waitErr:
			return
		case <-time.After(gracePeriod):
		}
	}
	killTree(cmd.Process.Pid)
	<-waitErr
}
