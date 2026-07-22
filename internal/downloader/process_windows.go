//go:build windows

package downloader

import (
	"os/exec"
	"strconv"

	"video-downloader-go/internal/procutil"
)

// killTree mirrors utils.terminate_process_tree's Windows branch: taskkill
// /T reaches the whole process tree (yt-dlp + any ffmpeg it spawned), /F
// forces a hard kill since there is no reliable graceful signal to send a
// --windowed process on Windows.
func killTree(pid int) {
	killCmd := exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(pid))
	procutil.SetProcAttrs(killCmd)
	_ = killCmd.Run()
}
