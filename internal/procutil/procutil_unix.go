//go:build !windows

// Package procutil holds the per-OS exec.Cmd attribute setup shared by
// internal/downloader and internal/updater, both of which spawn console
// subprocesses (yt-dlp, ffmpeg, tar, taskkill) the same way.
package procutil

import (
	"os/exec"
	"syscall"
)

// SetProcAttrs makes the child the leader of its own process group, so a
// killTree call (internal/downloader) can take a spawned ffmpeg down along
// with its yt-dlp parent. There's no console-flash concern on non-Windows,
// but internal/updater's exec.Command calls use this too for consistency.
func SetProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
