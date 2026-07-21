//go:build windows

package downloader

import (
	"os/exec"
	"strconv"
	"syscall"
)

// setProcAttrs hides the console window yt-dlp.exe would otherwise flash
// (it's a console app; this is a GUI app) and creates the process in its
// own process group so a cancel can take ffmpeg down with it.
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{
		HideWindow:    true,
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}
}

// killTree mirrors utils.terminate_process_tree's Windows branch: taskkill
// /T reaches the whole process tree (yt-dlp + any ffmpeg it spawned), /F
// forces a hard kill since there is no reliable graceful signal to send a
// --windowed process on Windows.
func killTree(pid int) {
	killCmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	setProcAttrs(killCmd)
	_ = killCmd.Run()
}
