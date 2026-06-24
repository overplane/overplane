//go:build !windows

package container

import (
	"os/exec"
	"syscall"
)

// setProcessGroup detaches the engine subprocess into its own process group
// so cancellation can terminate the whole tree.
func setProcessGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// terminateProcessGroup SIGTERMs the subprocess group; exec.Cmd.WaitDelay
// escalates to SIGKILL if it lingers.
func terminateProcessGroup(pid int) error {
	return syscall.Kill(-pid, syscall.SIGTERM)
}
