//go:build !windows

package downloader

import "syscall"

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
