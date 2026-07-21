//go:build !windows

package downloader

import (
	"os/exec"
	"syscall"
)

// setProcAttrs makes the child the leader of its own process group so
// killTree can take ffmpeg down with it (ffmpeg is spawned by yt-dlp's
// postprocessors as a grandchild).
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killTree mirrors utils.terminate_process_tree's POSIX branch: SIGKILL the
// whole process group. Always a hard kill, matching the Windows side and
// the original's rationale (no reliable graceful step on the platform this
// is primarily shipped for, and cancellation should be immediate).
func killTree(pid int) {
	pgid, err := syscall.Getpgid(pid)
	if err != nil {
		return
	}
	_ = syscall.Kill(-pgid, syscall.SIGKILL)
}
