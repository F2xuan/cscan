package worker

import (
	"bufio"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"os/exec"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// ==================== Terminal Configuration ====================

// TerminalConfig 终端配置
type TerminalConfig struct {
	DefaultTimeout time.Duration // 默认命令执行超时
	MaxSessions    int           // 最大会话数
	Blacklist      []string      // 命令黑名单（正则表达式）
	WorkingDir     string        // 默认工作目录
}

// DefaultTerminalConfig 默认终端配置
func DefaultTerminalConfig() *TerminalConfig {
	// 默认命令黑名单 - 危险命令
	defaultBlacklist := []string{
		`^rm\s+(-rf?|--recursive)\s+/\s*$`,   // rm -rf /
		`^rm\s+(-rf?|--recursive)\s+/\*`,     // rm -rf /*
		`^rm\s+(-rf?|--recursive)\s+~`,       // rm -rf ~
		`^shutdown`,                          // shutdown
		`^reboot`,                            // reboot
		`^halt`,                              // halt
		`^poweroff`,                          // poweroff
		`^init\s+0`,                          // init 0
		`^init\s+6`,                          // init 6
		`^mkfs`,                              // mkfs (format disk)
		`^dd\s+.*of=/dev/`,                   // dd to device
		`^:\(\)\{.*\}`,                       // fork bomb
		`>\s*/dev/sd[a-z]`,                   // write to disk device
		`>\s*/dev/nvme`,                      // write to nvme device
		`^chmod\s+(-R\s+)?777\s+/\s*$`,       // chmod 777 /
		`^chown\s+(-R\s+)?.*\s+/\s*$`,        // chown /
		`^format\s+[a-zA-Z]:`,                // Windows format drive
		`^del\s+/[sS]\s+/[qQ]\s+[a-zA-Z]:\\`, // Windows del /s /q C:\
		`^rd\s+/[sS]\s+/[qQ]\s+[a-zA-Z]:\\`,  // Windows rd /s /q C:\
	}

	workingDir, _ := os.Getwd()

	return &TerminalConfig{
		DefaultTimeout: 60 * time.Second,
		MaxSessions:    3,
		Blacklist:      defaultBlacklist,
		WorkingDir:     workingDir,
	}
}

// ==================== Terminal Session ====================

// TerminalSession 终端会话
type TerminalSession struct {
	ID         string
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	cancel     context.CancelFunc
	outputChan chan []byte
	closeChan  chan struct{}
	closeOnce  sync.Once
	mu         sync.Mutex
	isRunning  bool
	createdAt  time.Time
	lastActive time.Time
	cols       int
	rows       int
}

// NewTerminalSession 创建新的终端会话
func NewTerminalSession(id string) *TerminalSession {
	return &TerminalSession{
		ID:         id,
		outputChan: make(chan []byte, 256),
		closeChan:  make(chan struct{}),
		createdAt:  time.Now(),
		lastActive: time.Now(),
		cols:       80,
		rows:       24,
	}
}

// Close 关闭会话
func (s *TerminalSession) Close() {
	s.closeOnce.Do(func() {
		close(s.closeChan)
		s.mu.Lock()
		defer s.mu.Unlock()

		if s.cancel != nil {
			s.cancel()
		}
		if s.stdin != nil {
			s.stdin.Close()
		}
		if s.stdout != nil {
			s.stdout.Close()
		}
		if s.stderr != nil {
			s.stderr.Close()
		}
		if s.cmd != nil && s.cmd.Process != nil {
			s.cmd.Process.Kill()
		}
		s.isRunning = false
	})
}

// IsRunning 检查会话是否正在运行
func (s *TerminalSession) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.isRunning
}

// UpdateLastActive 更新最后活动时间
func (s *TerminalSession) UpdateLastActive() {
	s.mu.Lock()
	s.lastActive = time.Now()
	s.mu.Unlock()
}

// GetLastActive 获取最后活动时间
func (s *TerminalSession) GetLastActive() time.Time {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastActive
}

// ==================== Terminal Handler ====================

// TerminalHandler 终端处理器
type TerminalHandler struct {
	config           *TerminalConfig
	sessions         sync.Map // sessionId -> *TerminalSession
	sessionCount     int32
	mu               sync.Mutex
	createMu         sync.Mutex // 保护 CreateSession/CloseSession 的 check-and-store 原子性
	blacklistRegexps []*regexp.Regexp
	onOutput         func(sessionId string, data []byte) // 输出回调
}

// NewTerminalHandler 创建终端处理器
func NewTerminalHandler(config *TerminalConfig) *TerminalHandler {
	if config == nil {
		config = DefaultTerminalConfig()
	}

	h := &TerminalHandler{
		config: config,
	}

	// 编译黑名单正则表达式
	h.compileBlacklist()

	return h
}

// compileBlacklist 编译黑名单正则表达式
func (h *TerminalHandler) compileBlacklist() {
	h.blacklistRegexps = make([]*regexp.Regexp, 0, len(h.config.Blacklist))
	for _, pattern := range h.config.Blacklist {
		if re, err := regexp.Compile(pattern); err == nil {
			h.blacklistRegexps = append(h.blacklistRegexps, re)
		}
	}
}

// SetBlacklist 设置命令黑名单
func (h *TerminalHandler) SetBlacklist(patterns []string) {
	h.mu.Lock()
	h.config.Blacklist = patterns
	h.mu.Unlock()
	h.compileBlacklist()
}

// GetBlacklist 获取命令黑名单
func (h *TerminalHandler) GetBlacklist() []string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return append([]string{}, h.config.Blacklist...)
}

// SetDefaultTimeout 设置默认超时
func (h *TerminalHandler) SetDefaultTimeout(timeout time.Duration) {
	h.mu.Lock()
	h.config.DefaultTimeout = timeout
	h.mu.Unlock()
}

// GetDefaultTimeout 获取默认超时
func (h *TerminalHandler) GetDefaultTimeout() time.Duration {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.config.DefaultTimeout
}

// SetOutputHandler 设置输出回调
func (h *TerminalHandler) SetOutputHandler(handler func(sessionId string, data []byte)) {
	h.onOutput = handler
}

// IsCommandBlacklisted 检查命令是否在黑名单中
func (h *TerminalHandler) IsCommandBlacklisted(command string) bool {
	// 清理命令（去除首尾空格，转小写用于匹配）
	cmd := strings.TrimSpace(command)
	cmdLower := strings.ToLower(cmd)

	for _, re := range h.blacklistRegexps {
		if re.MatchString(cmd) || re.MatchString(cmdLower) {
			return true
		}
	}
	return false
}

// GetSessionCount 获取当前会话数
func (h *TerminalHandler) GetSessionCount() int {
	return int(atomic.LoadInt32(&h.sessionCount))
}

// GetSession 获取会话
func (h *TerminalHandler) GetSession(sessionId string) (*TerminalSession, bool) {
	if session, ok := h.sessions.Load(sessionId); ok {
		return session.(*TerminalSession), true
	}
	return nil, false
}

// ==================== Session Management ====================

// CreateSession 创建新会话
// 修复 C-12：原实现 check-count 与 store 非原子，并发调用可绕过 MaxSessions 限制
// 现使用 createMu 将"检查数量+检查存在+存储+计数"合并为临界区
func (h *TerminalHandler) CreateSession(sessionId string) (*TerminalSession, error) {
	h.createMu.Lock()
	defer h.createMu.Unlock()

	// 检查会话数限制
	if int(atomic.LoadInt32(&h.sessionCount)) >= h.config.MaxSessions {
		return nil, &TerminalError{Code: ErrCodeSessionLimit, Message: "maximum session limit reached"}
	}

	// 检查会话是否已存在
	if _, exists := h.sessions.Load(sessionId); exists {
		return nil, &TerminalError{Code: ErrCodeSessionExists, Message: "session already exists"}
	}

	session := NewTerminalSession(sessionId)
	h.sessions.Store(sessionId, session)
	atomic.AddInt32(&h.sessionCount, 1)

	return session, nil
}

// CloseSession 关闭会话
func (h *TerminalHandler) CloseSession(sessionId string) error {
	h.createMu.Lock()
	session, ok := h.sessions.Load(sessionId)
	if ok {
		h.sessions.Delete(sessionId)
		atomic.AddInt32(&h.sessionCount, -1)
	}
	h.createMu.Unlock()

	if !ok {
		return &TerminalError{Code: ErrCodeSessionNotFound, Message: "session not found"}
	}

	s := session.(*TerminalSession)
	s.Close()

	return nil
}

// CloseAllSessions 关闭所有会话
func (h *TerminalHandler) CloseAllSessions() {
	h.createMu.Lock()
	defer h.createMu.Unlock()
	h.sessions.Range(func(key, value interface{}) bool {
		if session, ok := value.(*TerminalSession); ok {
			session.Close()
		}
		h.sessions.Delete(key)
		return true
	})
	atomic.StoreInt32(&h.sessionCount, 0)
}

// ==================== Command Execution ====================

// ExecuteCommand 执行命令（单次执行，非交互式）
func (h *TerminalHandler) ExecuteCommand(ctx context.Context, sessionId, command string) error {
	// 检查黑名单
	if h.IsCommandBlacklisted(command) {
		return &TerminalError{Code: ErrCodeBlacklisted, Message: "command is blacklisted: " + command}
	}

	// 获取或创建会话
	session, ok := h.GetSession(sessionId)
	if !ok {
		var err error
		session, err = h.CreateSession(sessionId)
		if err != nil {
			return err
		}
	}

	session.UpdateLastActive()

	// 创建带超时的上下文
	timeout := h.config.DefaultTimeout
	execCtx, cancel := context.WithTimeout(ctx, timeout)
	session.cancel = cancel

	// 根据操作系统选择shell
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		// Windows: 使用 cmd /C 执行命令，先设置 UTF-8 代码页
		cmd = exec.CommandContext(execCtx, "cmd", "/C", "chcp 65001 >nul && "+command)
	} else {
		cmd = exec.CommandContext(execCtx, "sh", "-c", command)
	}

	// 设置环境变量，确保 UTF-8 输出
	cmd.Env = append(os.Environ(), "PYTHONIOENCODING=utf-8")

	// 设置工作目录
	if h.config.WorkingDir != "" {
		cmd.Dir = h.config.WorkingDir
	}

	// 获取输出管道
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cancel()
		return &TerminalError{Code: ErrCodeExecFailed, Message: "failed to create stdout pipe: " + err.Error()}
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cancel()
		return &TerminalError{Code: ErrCodeExecFailed, Message: "failed to create stderr pipe: " + err.Error()}
	}

	session.mu.Lock()
	session.cmd = cmd
	session.stdout = stdout
	session.stderr = stderr
	session.isRunning = true
	session.mu.Unlock()

	// 启动命令
	if err := cmd.Start(); err != nil {
		cancel()
		session.mu.Lock()
		session.isRunning = false
		session.mu.Unlock()
		return &TerminalError{Code: ErrCodeExecFailed, Message: "failed to start command: " + err.Error()}
	}

	// 启动输出读取协程
	go h.readOutput(session, stdout, "stdout")
	go h.readOutput(session, stderr, "stderr")

	// 等待命令完成
	go func() {
		err := cmd.Wait()
		cancel()

		session.mu.Lock()
		session.isRunning = false
		session.mu.Unlock()

		// 发送命令完成信号
		if h.onOutput != nil {
			var exitMsg string
			if err != nil {
				if execCtx.Err() == context.DeadlineExceeded {
					exitMsg = fmt.Sprintf("\r\n[Command timed out after %v]\r\n", timeout)
				} else {
					exitMsg = fmt.Sprintf("\r\n[Command exited with error: %v]\r\n", err)
				}
			} else {
				exitMsg = "\r\n[Command completed]\r\n"
			}
			h.onOutput(sessionId, []byte(exitMsg))
		}
	}()

	return nil
}

// readOutput 读取输出
func (h *TerminalHandler) readOutput(session *TerminalSession, reader io.Reader, streamType string) {
	scanner := bufio.NewScanner(reader)
	// 增加缓冲区大小以处理长行
	buf := make([]byte, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	for scanner.Scan() {
		select {
		case <-session.closeChan:
			return
		default:
			line := scanner.Bytes()
			// 添加换行符
			output := append(line, '\r', '\n')

			// 发送到输出通道
			select {
			case session.outputChan <- output:
			default:
				// 通道满了，丢弃
			}

			// 调用输出回调
			if h.onOutput != nil {
				h.onOutput(session.ID, output)
			}
		}
	}
}

// SendInput 发送输入到会话
func (h *TerminalHandler) SendInput(sessionId string, data []byte) error {
	session, ok := h.GetSession(sessionId)
	if !ok {
		return &TerminalError{Code: ErrCodeSessionNotFound, Message: "session not found"}
	}

	session.UpdateLastActive()

	session.mu.Lock()
	stdin := session.stdin
	session.mu.Unlock()

	if stdin == nil {
		return &TerminalError{Code: ErrCodeNoStdin, Message: "no stdin available"}
	}

	_, err := stdin.Write(data)
	if err != nil {
		return &TerminalError{Code: ErrCodeWriteFailed, Message: "failed to write to stdin: " + err.Error()}
	}

	return nil
}

// ResizeTerminal 调整终端大小
func (h *TerminalHandler) ResizeTerminal(sessionId string, cols, rows int) error {
	session, ok := h.GetSession(sessionId)
	if !ok {
		return &TerminalError{Code: ErrCodeSessionNotFound, Message: "session not found"}
	}

	session.mu.Lock()
	session.cols = cols
	session.rows = rows
	session.mu.Unlock()

	// 注意：对于非PTY的命令执行，resize不会有实际效果
	// 如果需要真正的终端大小调整，需要使用PTY

	return nil
}

// InterruptCommand 中断当前命令（Ctrl+C）
func (h *TerminalHandler) InterruptCommand(sessionId string) error {
	session, ok := h.GetSession(sessionId)
	if !ok {
		return &TerminalError{Code: ErrCodeSessionNotFound, Message: "session not found"}
	}

	session.mu.Lock()
	cmd := session.cmd
	cancel := session.cancel
	session.mu.Unlock()

	if cancel != nil {
		cancel()
	}

	if cmd != nil && cmd.Process != nil {
		// 发送中断信号
		if runtime.GOOS == "windows" {
			cmd.Process.Kill()
		} else {
			cmd.Process.Signal(os.Interrupt)
		}
	}

	return nil
}

// ==================== Error Types ====================

// TerminalError 终端错误
type TerminalError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *TerminalError) Error() string {
	return e.Message
}

// 终端错误码
const (
	ErrCodeSessionLimit    = 429 // 会话数达到上限
	ErrCodeSessionExists   = 409 // 会话已存在
	ErrCodeSessionNotFound = 404 // 会话不存在
	ErrCodeBlacklisted     = 403 // 命令被黑名单拦截
	ErrCodeExecFailed      = 500 // 命令执行失败
	ErrCodeTimeout         = 408 // 命令执行超时
	ErrCodeNoStdin         = 400 // 无stdin可用
	ErrCodeWriteFailed     = 500 // 写入失败
)

// IsBlacklistedError 检查是否是黑名单错误
func IsBlacklistedError(err error) bool {
	if te, ok := err.(*TerminalError); ok {
		return te.Code == ErrCodeBlacklisted
	}
	return false
}

// IsSessionLimitError 检查是否是会话限制错误
func IsSessionLimitError(err error) bool {
	if te, ok := err.(*TerminalError); ok {
		return te.Code == ErrCodeSessionLimit
	}
	return false
}

// ==================== WebSocket Message Payloads ====================

// TerminalOpenRequest 打开终端请求
type TerminalOpenRequest struct {
	RequestId string `json:"requestId"`
	SessionId string `json:"sessionId"`
	Cols      int    `json:"cols,omitempty"`
	Rows      int    `json:"rows,omitempty"`
}

// TerminalOpenResponse 打开终端响应
type TerminalOpenResponse struct {
	RequestId string `json:"requestId"`
	SessionId string `json:"sessionId"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// TerminalCloseRequest 关闭终端请求
type TerminalCloseRequest struct {
	RequestId string `json:"requestId"`
	SessionId string `json:"sessionId"`
}

// TerminalCloseResponse 关闭终端响应
type TerminalCloseResponse struct {
	RequestId string `json:"requestId"`
	SessionId string `json:"sessionId"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// TerminalInputRequest 终端输入请求
type TerminalInputRequest struct {
	RequestId string `json:"requestId"`
	SessionId string `json:"sessionId"`
	Data      string `json:"data"`              // Base64编码的输入数据
	Command   string `json:"command,omitempty"` // 可选：直接执行的命令
}

// TerminalInputResponse 终端输入响应
type TerminalInputResponse struct {
	RequestId string `json:"requestId"`
	SessionId string `json:"sessionId"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// TerminalOutputPayload 终端输出载荷
type TerminalOutputPayload struct {
	SessionId string `json:"sessionId"`
	Data      string `json:"data"` // Base64编码的输出数据
}

// TerminalResizeRequest 终端大小调整请求
type TerminalResizeRequest struct {
	RequestId string `json:"requestId"`
	SessionId string `json:"sessionId"`
	Cols      int    `json:"cols"`
	Rows      int    `json:"rows"`
}

// TerminalResizeResponse 终端大小调整响应
type TerminalResizeResponse struct {
	RequestId string `json:"requestId"`
	SessionId string `json:"sessionId"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// ==================== Helper Functions ====================

// EncodeTerminalOutput 编码终端输出为Base64
func EncodeTerminalOutput(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// DecodeTerminalInput 解码Base64终端输入
func DecodeTerminalInput(data string) ([]byte, error) {
	return base64.StdEncoding.DecodeString(data)
}
