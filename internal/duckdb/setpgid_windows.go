//go:build windows

package duckdb

import (
	"os/exec"
	"syscall"
)

// applyProcessGroup is the Windows equivalent of the Unix setpgid path:
// it asks the kernel to put the duckdb subprocess in a new process group
// so a Ctrl-Break can target it cleanly. Note: Windows duckdb is best-
// effort in v1 — the named clients run macOS/Linux.
func applyProcessGroup(cmd *exec.Cmd) {
	if cmd.SysProcAttr == nil {
		cmd.SysProcAttr = &syscall.SysProcAttr{}
	}
	cmd.SysProcAttr.CreationFlags |= 0x00000200 // CREATE_NEW_PROCESS_GROUP
}

// killProcessGroup is a no-op on Windows in v1. exec.CommandContext's
// default cancellation already calls Process.Kill on the subprocess; we
// rely on that for now and revisit if anyone reports orphans.
func killProcessGroup(pid int) {}
