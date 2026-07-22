//go:build windows

package updater

import (
	"os/exec"
	"syscall"
)

// setProcAttrs hides the console window a spawned console app (yt-dlp,
// ffmpeg, tar) would otherwise flash on startup (they're console apps; this
// is a GUI app). Mirrors internal/downloader's process_windows.go, which
// exists for the exact same reason on the download path.
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}
