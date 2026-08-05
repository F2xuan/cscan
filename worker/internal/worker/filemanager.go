package worker

import (
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
)

// FileInfo 文件信息
type FileInfo struct {
	Name    string `json:"name"`
	Size    int64  `json:"size"`
	Mode    string `json:"mode"`
	ModTime int64  `json:"modTime"`
	IsDir   bool   `json:"isDir"`
}

// FileManagerConfig 文件管理器配置
type FileManagerConfig struct {
	AllowedPaths  []string // 允许访问的目录列表
	MaxUploadSize int64    // 最大上传大小（字节）
}

// DefaultFileManagerConfig 默认配置
func DefaultFileManagerConfig() *FileManagerConfig {
	// 默认允许访问的目录
	allowedPaths := []string{}

	// 获取当前工作目录
	if cwd, err := os.Getwd(); err == nil {
		allowedPaths = append(allowedPaths, cwd)
	}

	// 获取用户主目录
	if home, err := os.UserHomeDir(); err == nil {
		allowedPaths = append(allowedPaths, home)
	}

	// 添加临时目录
	allowedPaths = append(allowedPaths, os.TempDir())

	return &FileManagerConfig{
		AllowedPaths:  allowedPaths,
		MaxUploadSize: 100 * 1024 * 1024, // 100MB
	}
}

// FileManager 文件管理器
type FileManager struct {
	config *FileManagerConfig
	mu     sync.RWMutex
}

// NewFileManager 创建文件管理器
func NewFileManager(config *FileManagerConfig) *FileManager {
	if config == nil {
		config = DefaultFileManagerConfig()
	}
	return &FileManager{
		config: config,
	}
}

// SetAllowedPaths 设置允许访问的目录
func (m *FileManager) SetAllowedPaths(paths []string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.AllowedPaths = paths
}

// GetAllowedPaths 获取允许访问的目录
func (m *FileManager) GetAllowedPaths() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return append([]string{}, m.config.AllowedPaths...)
}

// SetMaxUploadSize 设置最大上传大小
func (m *FileManager) SetMaxUploadSize(size int64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config.MaxUploadSize = size
}

// GetMaxUploadSize 获取最大上传大小
func (m *FileManager) GetMaxUploadSize() int64 {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config.MaxUploadSize
}

// IsPathAllowed 检查路径是否在允许的目录范围内（安全沙箱）
func (m *FileManager) IsPathAllowed(path string) bool {
	m.mu.RLock()
	allowedPaths := m.config.AllowedPaths
	m.mu.RUnlock()

	// 获取绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}

	// 清理路径，防止路径遍历攻击
	cleanPath := filepath.Clean(absPath)

	// 检查是否包含路径遍历尝试
	if strings.Contains(path, "..") {
		// 如果原始路径包含 ".."，需要验证清理后的路径仍在允许范围内
		// 这已经在下面的检查中处理
	}

	// 检查是否在允许的目录范围内
	for _, allowedPath := range allowedPaths {
		allowedAbs, err := filepath.Abs(allowedPath)
		if err != nil {
			continue
		}
		allowedClean := filepath.Clean(allowedAbs)

		// 检查cleanPath是否以allowedClean开头
		// 需要确保是目录边界，而不是前缀匹配
		if isSubPath(cleanPath, allowedClean) {
			return true
		}
	}

	return false
}

// isSubPath 检查path是否是basePath的子路径
func isSubPath(path, basePath string) bool {
	// 确保路径使用统一的分隔符
	path = filepath.Clean(path)
	basePath = filepath.Clean(basePath)

	// 如果路径相等，也算是子路径
	if path == basePath {
		return true
	}

	// 确保basePath以分隔符结尾进行前缀匹配
	if !strings.HasSuffix(basePath, string(filepath.Separator)) {
		basePath = basePath + string(filepath.Separator)
	}

	return strings.HasPrefix(path, basePath)
}

// normalizePath 规范化路径
func (m *FileManager) normalizePath(path string) (string, error) {
	// 处理空路径
	if path == "" {
		// 返回第一个允许的路径
		m.mu.RLock()
		allowedPaths := m.config.AllowedPaths
		m.mu.RUnlock()

		if len(allowedPaths) > 0 {
			return filepath.Abs(allowedPaths[0])
		}
		return "", fmt.Errorf("no allowed paths configured")
	}

	// 处理 ~ 开头的路径（用户主目录）
	if strings.HasPrefix(path, "~") {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("cannot resolve home directory: %w", err)
		}
		path = filepath.Join(home, path[1:])
	}

	// 获取绝对路径
	absPath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path: %w", err)
	}

	return filepath.Clean(absPath), nil
}

// ListDir 列出目录内容
func (m *FileManager) ListDir(path string) ([]FileInfo, error) {
	// 规范化路径
	normalizedPath, err := m.normalizePath(path)
	if err != nil {
		return nil, err
	}

	// 安全检查
	if !m.IsPathAllowed(normalizedPath) {
		return nil, &FileError{Code: ErrCodeForbidden, Message: "path not allowed: " + path}
	}

	// 检查路径是否存在
	info, err := os.Stat(normalizedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &FileError{Code: ErrCodeNotFound, Message: "path not found: " + path}
		}
		return nil, &FileError{Code: ErrCodePermission, Message: "cannot access path: " + err.Error()}
	}

	// 如果是文件，返回单个文件信息
	if !info.IsDir() {
		return []FileInfo{fileInfoFromOS(info)}, nil
	}

	// 读取目录内容
	entries, err := os.ReadDir(normalizedPath)
	if err != nil {
		return nil, &FileError{Code: ErrCodePermission, Message: "cannot read directory: " + err.Error()}
	}

	files := make([]FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue // 跳过无法获取信息的文件
		}
		files = append(files, fileInfoFromOS(info))
	}

	return files, nil
}

// fileInfoFromOS 从os.FileInfo转换为FileInfo
func fileInfoFromOS(info os.FileInfo) FileInfo {
	return FileInfo{
		Name:    info.Name(),
		Size:    info.Size(),
		Mode:    info.Mode().String(),
		ModTime: info.ModTime().UnixMilli(),
		IsDir:   info.IsDir(),
	}
}

// ReadFile 读取文件内容
func (m *FileManager) ReadFile(path string) ([]byte, error) {
	// 规范化路径
	normalizedPath, err := m.normalizePath(path)
	if err != nil {
		return nil, err
	}

	// 安全检查
	if !m.IsPathAllowed(normalizedPath) {
		return nil, &FileError{Code: ErrCodeForbidden, Message: "path not allowed: " + path}
	}

	// 检查文件是否存在
	info, err := os.Stat(normalizedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &FileError{Code: ErrCodeNotFound, Message: "file not found: " + path}
		}
		return nil, &FileError{Code: ErrCodePermission, Message: "cannot access file: " + err.Error()}
	}

	// 检查是否是目录
	if info.IsDir() {
		return nil, &FileError{Code: ErrCodeInvalid, Message: "path is a directory: " + path}
	}

	// 读取文件
	data, err := os.ReadFile(normalizedPath)
	if err != nil {
		return nil, &FileError{Code: ErrCodePermission, Message: "cannot read file: " + err.Error()}
	}

	return data, nil
}

// ReadFileBase64 读取文件内容并返回Base64编码
func (m *FileManager) ReadFileBase64(path string) (string, error) {
	data, err := m.ReadFile(path)
	if err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(data), nil
}

// WriteFile 写入文件内容
func (m *FileManager) WriteFile(path string, data []byte) error {
	// 规范化路径
	normalizedPath, err := m.normalizePath(path)
	if err != nil {
		return err
	}

	// 安全检查
	if !m.IsPathAllowed(normalizedPath) {
		return &FileError{Code: ErrCodeForbidden, Message: "path not allowed: " + path}
	}

	// 检查文件大小限制
	m.mu.RLock()
	maxSize := m.config.MaxUploadSize
	m.mu.RUnlock()

	if int64(len(data)) > maxSize {
		return &FileError{Code: ErrCodeTooLarge, Message: fmt.Sprintf("file too large: %d bytes (max: %d)", len(data), maxSize)}
	}

	// 确保父目录存在
	dir := filepath.Dir(normalizedPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return &FileError{Code: ErrCodePermission, Message: "cannot create directory: " + err.Error()}
	}

	// 写入文件
	if err := os.WriteFile(normalizedPath, data, 0644); err != nil {
		return &FileError{Code: ErrCodePermission, Message: "cannot write file: " + err.Error()}
	}

	return nil
}

// WriteFileBase64 写入Base64编码的文件内容
func (m *FileManager) WriteFileBase64(path string, base64Data string) error {
	data, err := base64.StdEncoding.DecodeString(base64Data)
	if err != nil {
		return &FileError{Code: ErrCodeInvalid, Message: "invalid base64 data: " + err.Error()}
	}
	return m.WriteFile(path, data)
}

// DeleteFile 删除文件或目录
func (m *FileManager) DeleteFile(path string) error {
	// 规范化路径
	normalizedPath, err := m.normalizePath(path)
	if err != nil {
		return err
	}

	// 安全检查
	if !m.IsPathAllowed(normalizedPath) {
		return &FileError{Code: ErrCodeForbidden, Message: "path not allowed: " + path}
	}

	// 检查是否是允许的根目录（不允许删除根目录）
	m.mu.RLock()
	allowedPaths := m.config.AllowedPaths
	m.mu.RUnlock()

	for _, allowedPath := range allowedPaths {
		allowedAbs, _ := filepath.Abs(allowedPath)
		if filepath.Clean(normalizedPath) == filepath.Clean(allowedAbs) {
			return &FileError{Code: ErrCodeForbidden, Message: "cannot delete root allowed path"}
		}
	}

	// 检查文件是否存在
	info, err := os.Stat(normalizedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileError{Code: ErrCodeNotFound, Message: "file not found: " + path}
		}
		return &FileError{Code: ErrCodePermission, Message: "cannot access file: " + err.Error()}
	}

	// 删除文件或目录
	if info.IsDir() {
		if err := os.RemoveAll(normalizedPath); err != nil {
			return &FileError{Code: ErrCodePermission, Message: "cannot delete directory: " + err.Error()}
		}
	} else {
		if err := os.Remove(normalizedPath); err != nil {
			return &FileError{Code: ErrCodePermission, Message: "cannot delete file: " + err.Error()}
		}
	}

	return nil
}

// MakeDir 创建目录
func (m *FileManager) MakeDir(path string) error {
	// 规范化路径
	normalizedPath, err := m.normalizePath(path)
	if err != nil {
		return err
	}

	// 安全检查
	if !m.IsPathAllowed(normalizedPath) {
		return &FileError{Code: ErrCodeForbidden, Message: "path not allowed: " + path}
	}

	// 检查是否已存在
	if info, err := os.Stat(normalizedPath); err == nil {
		if info.IsDir() {
			return nil // 目录已存在，不报错
		}
		return &FileError{Code: ErrCodeExists, Message: "path exists and is not a directory: " + path}
	}

	// 创建目录
	if err := os.MkdirAll(normalizedPath, 0755); err != nil {
		return &FileError{Code: ErrCodePermission, Message: "cannot create directory: " + err.Error()}
	}

	return nil
}

// CopyFile 复制文件
func (m *FileManager) CopyFile(src, dst string) error {
	// 规范化路径
	srcPath, err := m.normalizePath(src)
	if err != nil {
		return err
	}
	dstPath, err := m.normalizePath(dst)
	if err != nil {
		return err
	}

	// 安全检查
	if !m.IsPathAllowed(srcPath) {
		return &FileError{Code: ErrCodeForbidden, Message: "source path not allowed: " + src}
	}
	if !m.IsPathAllowed(dstPath) {
		return &FileError{Code: ErrCodeForbidden, Message: "destination path not allowed: " + dst}
	}

	// 打开源文件
	srcFile, err := os.Open(srcPath)
	if err != nil {
		if os.IsNotExist(err) {
			return &FileError{Code: ErrCodeNotFound, Message: "source file not found: " + src}
		}
		return &FileError{Code: ErrCodePermission, Message: "cannot open source file: " + err.Error()}
	}
	defer srcFile.Close()

	// 获取源文件信息
	srcInfo, err := srcFile.Stat()
	if err != nil {
		return &FileError{Code: ErrCodePermission, Message: "cannot stat source file: " + err.Error()}
	}

	if srcInfo.IsDir() {
		return &FileError{Code: ErrCodeInvalid, Message: "source is a directory: " + src}
	}

	// 确保目标目录存在
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return &FileError{Code: ErrCodePermission, Message: "cannot create destination directory: " + err.Error()}
	}

	// 创建目标文件
	dstFile, err := os.Create(dstPath)
	if err != nil {
		return &FileError{Code: ErrCodePermission, Message: "cannot create destination file: " + err.Error()}
	}
	defer dstFile.Close()

	// 复制内容
	if _, err := io.Copy(dstFile, srcFile); err != nil {
		return &FileError{Code: ErrCodePermission, Message: "cannot copy file: " + err.Error()}
	}

	// 设置权限
	if err := os.Chmod(dstPath, srcInfo.Mode()); err != nil {
		// 权限设置失败不是致命错误，只记录警告
	}

	return nil
}

// MoveFile 移动文件
func (m *FileManager) MoveFile(src, dst string) error {
	// 规范化路径
	srcPath, err := m.normalizePath(src)
	if err != nil {
		return err
	}
	dstPath, err := m.normalizePath(dst)
	if err != nil {
		return err
	}

	// 安全检查
	if !m.IsPathAllowed(srcPath) {
		return &FileError{Code: ErrCodeForbidden, Message: "source path not allowed: " + src}
	}
	if !m.IsPathAllowed(dstPath) {
		return &FileError{Code: ErrCodeForbidden, Message: "destination path not allowed: " + dst}
	}

	// 检查源文件是否存在
	if _, err := os.Stat(srcPath); err != nil {
		if os.IsNotExist(err) {
			return &FileError{Code: ErrCodeNotFound, Message: "source file not found: " + src}
		}
		return &FileError{Code: ErrCodePermission, Message: "cannot access source file: " + err.Error()}
	}

	// 确保目标目录存在
	dstDir := filepath.Dir(dstPath)
	if err := os.MkdirAll(dstDir, 0755); err != nil {
		return &FileError{Code: ErrCodePermission, Message: "cannot create destination directory: " + err.Error()}
	}

	// 尝试重命名（同一文件系统内的移动）
	if err := os.Rename(srcPath, dstPath); err != nil {
		// 如果重命名失败（可能跨文件系统），尝试复制后删除
		if err := m.CopyFile(src, dst); err != nil {
			return err
		}
		if err := m.DeleteFile(src); err != nil {
			return err
		}
	}

	return nil
}

// GetFileInfo 获取文件信息
func (m *FileManager) GetFileInfo(path string) (*FileInfo, error) {
	// 规范化路径
	normalizedPath, err := m.normalizePath(path)
	if err != nil {
		return nil, err
	}

	// 安全检查
	if !m.IsPathAllowed(normalizedPath) {
		return nil, &FileError{Code: ErrCodeForbidden, Message: "path not allowed: " + path}
	}

	// 获取文件信息
	info, err := os.Stat(normalizedPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, &FileError{Code: ErrCodeNotFound, Message: "file not found: " + path}
		}
		return nil, &FileError{Code: ErrCodePermission, Message: "cannot access file: " + err.Error()}
	}

	fileInfo := fileInfoFromOS(info)
	return &fileInfo, nil
}

// GetWorkingDirectory 获取当前工作目录
func (m *FileManager) GetWorkingDirectory() string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}
	return cwd
}

// GetHomeDirectory 获取用户主目录
func (m *FileManager) GetHomeDirectory() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return home
}

// GetTempDirectory 获取临时目录
func (m *FileManager) GetTempDirectory() string {
	return os.TempDir()
}

// GetSystemRoot 获取系统根目录
func (m *FileManager) GetSystemRoot() string {
	if runtime.GOOS == "windows" {
		return "C:\\"
	}
	return "/"
}

// ==================== Error Types ====================

// FileError 文件操作错误
type FileError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

func (e *FileError) Error() string {
	return e.Message
}

// 错误码定义
const (
	ErrCodeForbidden  = 403 // 路径不允许访问
	ErrCodeNotFound   = 404 // 文件不存在
	ErrCodePermission = 403 // 权限不足
	ErrCodeExists     = 409 // 文件已存在
	ErrCodeInvalid    = 400 // 无效操作
	ErrCodeTooLarge   = 413 // 文件太大
	ErrCodeDiskFull   = 507 // 磁盘空间不足
)

// IsNotFoundError 检查是否是文件不存在错误
func IsNotFoundError(err error) bool {
	if fe, ok := err.(*FileError); ok {
		return fe.Code == ErrCodeNotFound
	}
	return false
}

// IsForbiddenError 检查是否是禁止访问错误
func IsForbiddenError(err error) bool {
	if fe, ok := err.(*FileError); ok {
		return fe.Code == ErrCodeForbidden
	}
	return false
}

// IsPermissionError 检查是否是权限错误
func IsPermissionError(err error) bool {
	if fe, ok := err.(*FileError); ok {
		return fe.Code == ErrCodePermission
	}
	return false
}

// ==================== WebSocket Message Payloads ====================

// FileListRequest 文件列表请求
type FileListRequest struct {
	RequestId string `json:"requestId"`
	Path      string `json:"path"`
}

// FileListResponse 文件列表响应
type FileListResponse struct {
	RequestId string     `json:"requestId"`
	Path      string     `json:"path"`
	Files     []FileInfo `json:"files,omitempty"`
	Error     string     `json:"error,omitempty"`
}

// FileUploadRequest 文件上传请求
type FileUploadRequest struct {
	RequestId string `json:"requestId"`
	Path      string `json:"path"`
	Data      string `json:"data"` // Base64编码的文件内容
}

// FileUploadResponse 文件上传响应
type FileUploadResponse struct {
	RequestId string `json:"requestId"`
	Path      string `json:"path"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// FileDownloadRequest 文件下载请求
type FileDownloadRequest struct {
	RequestId string `json:"requestId"`
	Path      string `json:"path"`
}

// FileDownloadResponse 文件下载响应
type FileDownloadResponse struct {
	RequestId string `json:"requestId"`
	Path      string `json:"path"`
	Data      string `json:"data,omitempty"` // Base64编码的文件内容
	Size      int64  `json:"size,omitempty"`
	Error     string `json:"error,omitempty"`
}

// FileDeleteRequest 文件删除请求
type FileDeleteRequest struct {
	RequestId string `json:"requestId"`
	Path      string `json:"path"`
}

// FileDeleteResponse 文件删除响应
type FileDeleteResponse struct {
	RequestId string `json:"requestId"`
	Path      string `json:"path"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}

// FileMkdirRequest 创建目录请求
type FileMkdirRequest struct {
	RequestId string `json:"requestId"`
	Path      string `json:"path"`
}

// FileMkdirResponse 创建目录响应
type FileMkdirResponse struct {
	RequestId string `json:"requestId"`
	Path      string `json:"path"`
	Success   bool   `json:"success"`
	Error     string `json:"error,omitempty"`
}
