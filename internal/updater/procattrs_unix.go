//go:build !windows

package updater

import (
	"os/exec"
	"syscall"
)

// setProcAttrs makes the child the leader of its own process group.
// Mirrors internal/downloader's process_unix.go; there's no console-flash
// concern on non-Windows, but the same call keeps this package's exec.Command
// calls consistent across platforms.
func setProcAttrs(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}
