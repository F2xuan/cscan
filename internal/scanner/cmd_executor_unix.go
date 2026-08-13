//go:build !windows

package scanner

import (
	"os/exec"
	"syscall"
)

// setSysProcAttr 设置 Unix 平台进程属性，创建独立进程组以便后续 kill 整棵树
func setSysProcAttr(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
}

// killProcessTree 杀掉进程及其所有子进程（通过负 PID 向整个进程组发 SIGKILL）
func killProcessTree(cmd *exec.Cmd) {
	if cmd.Process == nil {
		return
	}
	_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
}
