//go:build windows

package worker

import (
	"os"
	"os/exec"
	"syscall"
)

// platformRestart handles Windows-specific process restart
func platformRestart(executable string, args []string, logger Logger) {
	// Windows: 使用 CREATE_NEW_PROCESS_GROUP 标志启动新进程
	cmd := exec.Command(executable, args[1:]...)
	cmd.SysProcAttr = &syscall.SysProcAttr{
		CreationFlags: syscall.CREATE_NEW_PROCESS_GROUP,
	}

	if err := cmd.Start(); err != nil {
		logger.Error("Failed to restart worker: %v", err)
		os.Exit(1)
	}

	logger.Info("New worker process started (PID: %d), exiting current process...", cmd.Process.Pid)
	os.Exit(0)
}
