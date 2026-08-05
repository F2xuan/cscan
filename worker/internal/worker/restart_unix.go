//go:build !windows

package worker

import (
	"os"
	"syscall"
)

// platformRestart handles Unix-specific process restart
func platformRestart(executable string, args []string, logger Logger) {
	// Unix: 使用 syscall.Exec 替换当前进程
	env := os.Environ()
	if err := syscall.Exec(executable, args, env); err != nil {
		logger.Error("Failed to exec: %v", err)
		os.Exit(1)
	}
}
