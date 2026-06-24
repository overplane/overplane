//go:build windows

package container

import (
	"os"
	"os/exec"
)

// setProcessGroup is a no-op on Windows; cancellation kills the direct child
// only.
func setProcessGroup(_ *exec.Cmd) {}

// terminateProcessGroup kills the engine subprocess.
func terminateProcessGroup(pid int) error {
	p, err := os.FindProcess(pid)
	if err != nil {
		return err
	}
	return p.Kill()
}
