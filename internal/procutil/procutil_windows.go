//go:build windows

// Package procutil holds the per-OS exec.Cmd attribute setup shared by
// internal/downloader and internal/updater, both of which spawn console
// subprocesses (yt-dlp, ffmpeg, tar, taskkill) the same way.
package procutil

import (
	"os/exec"
	"syscall"
)

// SetProcAttrs hides the console window a spawned console app (yt-dlp,
// ffmpeg, tar, taskkill) would otherwise flash on startup (they're console
// apps; this is a GUI app), and creates the process in its own process
// group so a killTree call (internal/downloader) can take a spawned ffmpeg
// down along with its yt-dlp parent.
func SetProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
