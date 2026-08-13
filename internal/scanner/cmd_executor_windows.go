//go:build windows

package scanner

import (
	"os/exec"
	"strconv"
)

// setSysProcAttr Windows 平台无需 Setpgid，taskkill /T 负责递归终止子进程
func setSysProcAttr(cmd *exec.Cmd) {
	// no-op
}

// killProcessTree 使用 taskkill /F /T /PID 递归终止进程树，
// 避免 Chrome/nmap NSE 子进程成为孤儿
func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = exec.Command("taskkill", "/F", "/T", "/PID", strconv.Itoa(cmd.Process.Pid)).Run()
}
