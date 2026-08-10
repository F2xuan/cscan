package scanner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"github.com/zeromicro/go-zero/core/logx"
)

// CmdExecutor 统一子进程执行器
type CmdExecutor struct {
	binaryPath     string
	memoryLimitMB  int64
	defaultTimeout time.Duration
}

func NewCmdExecutor(binaryPath string, memoryLimitMB int64, defaultTimeout time.Duration) *CmdExecutor {
	return &CmdExecutor{
		binaryPath:     binaryPath,
		memoryLimitMB:  memoryLimitMB,
		defaultTimeout: defaultTimeout,
	}
}

func (e *CmdExecutor) Execute(ctx context.Context, args []string, opts ExecuteOpts) (*ExecuteResult, error) {
	result := &ExecuteResult{}

	timeout := e.defaultTimeout
	if opts.Timeout > 0 {
		timeout = opts.Timeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, e.binaryPath, args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}
	if len(opts.Env) > 0 {
		cmd.Env = opts.Env
	}

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	startTime := time.Now()
	err := cmd.Start()
	if err != nil {
		result.Stderr = fmt.Sprintf("failed to start: %v", err)
		return result, err
	}

	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()

	select {
	case <-execCtx.Done():
		if cmd.Process != nil {
			_ = cmd.Process.Kill()
		}
		<-done
		result.Duration = time.Since(startTime)
		result.Stderr = stderrBuf.String()
		return result, fmt.Errorf("%s: timeout after %v", e.binaryPath, timeout)
	case err := <-done:
		result.Stdout = stdoutBuf.String()
		result.Stderr = stderrBuf.String()
		result.ExitCode = 0
		if err != nil {
			if exitErr, ok := err.(*exec.ExitError); ok {
				result.ExitCode = exitErr.ExitCode()
			}
		}
		result.Duration = time.Since(startTime)
		return result, err
	}
}

func (e *CmdExecutor) StreamLines(ctx context.Context, args []string, handler func(line string) (bool, error), opts ExecuteOpts) error {
	timeout := e.defaultTimeout
	if opts.Timeout > 0 {
		timeout = opts.Timeout
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	cmd := exec.CommandContext(execCtx, e.binaryPath, args...)
	if opts.WorkingDir != "" {
		cmd.Dir = opts.WorkingDir
	}
	if len(opts.Env) > 0 {
		cmd.Env = opts.Env
	}

	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		stdoutPipe.Close()
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		stdoutPipe.Close()
		stderrPipe.Close()
		return fmt.Errorf("start: %w", err)
	}

	done := make(chan error, 1)
	go func() {
		var stderrBuf bytes.Buffer
		_, _ = io.Copy(&stderrBuf, stderrPipe)
		stderrPipe.Close()
		err := cmd.Wait()
		if err != nil && stderrBuf.Len() > 0 {
			logx.Debugf("[%s] stderr: %s", e.binaryPath, stderrBuf.String())
		}
		done <- err
	}()

	scanner := bufio.NewScanner(stdoutPipe)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)
	for scanner.Scan() {
		line := scanner.Text()
		if line == "" {
			continue
		}
		cont, err := handler(line)
		if err != nil {
			_ = cmd.Process.Kill()
			<-done
			stdoutPipe.Close()
			return fmt.Errorf("handler error: %w", err)
		}
		if !cont {
			_ = cmd.Process.Kill()
			break
		}
	}

	stdoutPipe.Close()
	<-done
	return nil
}

func (e *CmdExecutor) CheckHealth() ToolHealth {
	health := ToolHealth{Name: e.binaryPath}

	path, err := exec.LookPath(e.binaryPath)
	if err != nil {
		health.Available = false
		health.Error = fmt.Sprintf("binary not found: %v", err)
		return health
	}

	health.Available = true
	health.Path = path

	cmd := exec.Command(e.binaryPath, "-version")
	output, err := cmd.CombinedOutput()
	if err != nil {
		health.Version = "unknown"
		return health
	}
	health.Version = parseVersion(string(output))
	return health
}

func parseVersion(output string) string {
	fields := strings.Fields(output)
	for _, f := range fields {
		if looksLikeVersion(f) {
			return f
		}
	}
	return "unknown"
}

func looksLikeVersion(s string) bool {
	if len(s) < 2 || len(s) > 20 {
		return false
	}
	hasDigit := false
	hasDot := false
	for _, c := range s {
		if c >= '0' && c <= '9' {
			hasDigit = true
		}
		if c == '.' {
			hasDot = true
		}
	}
	return hasDigit && hasDot
}

// ExecuteOpts 执行选项
type ExecuteOpts struct {
	Timeout       time.Duration
	MemoryLimitMB int64
	WorkingDir    string
	Env           []string
}

// ExecuteResult 执行结果
type ExecuteResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Duration time.Duration
}

// ToolHealth 工具健康检查结果
type ToolHealth struct {
	Name      string
	Available bool
	Path      string
	Version   string
	Error     string
}

// ScanLineResult 单行扫描结果
type ScanLineResult struct {
	Line     string
	Parsed   interface{}
	Error    error
	Continue bool
}

// JSONLineParser JSON 行解析辅助函数
func JSONLineParser(line string) (map[string]interface{}, error) {
	var result map[string]interface{}
	if err := json.Unmarshal([]byte(line), &result); err != nil {
		return nil, err
	}
	return result, nil
}
