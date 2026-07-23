//go:build !windows

package downloader

import (
	"os/exec"
	"syscall"
	"time"
)

// killTree mirrors utils.terminate_process_tree's POSIX branch: SIGKILL the
// whole process group immediately, no grace period. Used for app shutdown
// (see ErrShutdown in download.go) and the credentials-retry path, where an
// immediate restart matters more than a graceful wind-down of the attempt
// being discarded.
func killTree(pid int) {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}

// cancelTree is the user-cancel path: SIGINT the whole process group first.
// yt-dlp's Python runtime treats SIGINT as KeyboardInterrupt and aborts
// cleanly, and ffmpeg -- sharing the group as yt-dlp's child -- treats
// SIGINT as its own graceful-stop signal, finalizing whatever output file
// it was writing instead of leaving a truncated one. If the tree hasn't
// exited on its own within gracePeriod, it's escalated to killTree.
//
// waitErr must be the same channel the caller's
// `go func() { waitErr <- cmd.Wait() }()` sends to -- cancelTree always
// drains exactly one value from it before returning, so the caller never
// has to.
func cancelTree(cmd *exec.Cmd, waitErr <-chan error, gracePeriod time.Duration) {
	if pgid, err := syscall.Getpgid(cmd.Process.Pid); err == nil {
		_ = syscall.Kill(-pgid, syscall.SIGINT)
		select {
		case <-waitErr:
			return
		case <-time.After(gracePeriod):
		}
	}
	killTree(cmd.Process.Pid)
	<-waitErr
}
