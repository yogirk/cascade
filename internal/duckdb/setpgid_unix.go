//go:build !windows

package duckdb

import (
	"os/exec"
	"syscall"
)

// applyProcessGroup makes cmd the leader of a new process group so that
// killing it via context cancellation also kills any child processes
// duckdb might spawn (e.g. for extension downloads). Without this,
// Ctrl-C can leave orphans behind.
func applyProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.Setpgid = true
}

// killProcessGroup sends SIGKILL to the process group rooted at pid.
// Best-effort: errors are ignored because the process may already be
// gone by the time we get here.
func killProcessGroup(pid int) {
	_ = syscall.Kill(-pid, syscall.SIGKILL)
}
