// Package securityv2 提供安全模块 v2 版本
package securityv2

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// SafeFileManager 安全文件管理器
// 解决路径遍历漏洞和竞态条件问题
type SafeFileManager struct {
	baseDir           string
	allowedExt        map[string]bool
	mu                sync.RWMutex
	accessLogger      *AccessLogger
	permChecker       *PermissionChecker
	sensitiveDetector *SensitiveFileDetector
}

// NewSafeFileManager 创建安全文件管理器
func NewSafeFileManager(baseDir string) *SafeFileManager {
	// 规范化基目录路径
	baseDir = filepath.Clean(baseDir)

	return &SafeFileManager{
		baseDir: baseDir,
		allowedExt: map[string]bool{
			".txt":  true,
			".pdf":  true,
			".doc":  true,
			".docx": true,
			".xls":  true,
			".xlsx": true,
			".jpg":  true,
			".png":  true,
			".mp4":  true,
			".zip":  true,
		},
		accessLogger:      NewAccessLogger("", 1000),
		permChecker:       NewPermissionChecker(),
		sensitiveDetector: NewSensitiveFileDetector(),
	}
}

// SetAccessLogger 设置访问日志记录器
func (m *SafeFileManager) SetAccessLogger(logger *AccessLogger) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.accessLogger = logger
}

// GetAccessLogger 获取访问日志记录器
func (m *SafeFileManager) GetAccessLogger() *AccessLogger {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.accessLogger
}

// SafePath 验证并返回安全路径
// 防止路径遍历攻击
func (m *SafeFileManager) SafePath(userPath string) (string, error) {
	// 清理路径
	cleanPath := filepath.Clean(userPath)

	// 检查是否为绝对路径（绝对路径直接拒绝，除非是 baseDir 本身）
	if filepath.IsAbs(cleanPath) {
		// 获取规范化的基目录路径
		absBase, err := filepath.Abs(m.baseDir)
		if err != nil {
			return "", fmt.Errorf("cannot resolve base directory: %w", err)
		}

		// 只有当路径是 baseDir 或其子路径时才允许
		if !strings.HasPrefix(cleanPath+string(os.PathSeparator), absBase+string(os.PathSeparator)) && cleanPath != absBase {
			return "", errors.New("absolute path not allowed")
		}
	}

	// 移除开头的斜杠
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	cleanPath = strings.TrimPrefix(cleanPath, "\\")

	// 构建完整路径
	fullPath := filepath.Join(m.baseDir, cleanPath)

	// 获取规范化的基目录路径
	absBase, err := filepath.Abs(m.baseDir)
	if err != nil {
		return "", fmt.Errorf("cannot resolve base directory: %w", err)
	}

	// 获取规范化的完整路径
	absFull, err := filepath.Abs(fullPath)
	if err != nil {
		return "", fmt.Errorf("cannot resolve path: %w", err)
	}

	// 检查路径是否在基目录内（对于不存在的文件也有效）
	// 确保路径以基目录开头
	relPath, err := filepath.Rel(absBase, absFull)
	if err != nil {
		return "", fmt.Errorf("cannot compute relative path: %w", err)
	}

	// 如果相对路径以 .. 开头，说明在基目录外
	if strings.HasPrefix(relPath, "..") || strings.HasPrefix(relPath, "../") {
		return "", errors.New("path traversal detected")
	}

	// 对于已存在的路径，检查符号链接
	resolved, err := filepath.EvalSymlinks(absFull)
	if err == nil {
		// 文件存在，验证符号链接目标
		if !strings.HasPrefix(resolved+string(os.PathSeparator), absBase+string(os.PathSeparator)) && resolved != absBase {
			return "", errors.New("symlink points outside base directory")
		}
	}

	return absFull, nil
}

// SafeRead 安全读取文件
func (m *SafeFileManager) SafeRead(userPath string) ([]byte, error) {
	startTime := time.Now()
	logEntry := AccessLog{
		Operation: "read",
		Path:      userPath,
	}

	defer func() {
		logEntry.Duration = time.Since(startTime).Nanoseconds()
		if m.accessLogger != nil {
			m.accessLogger.Log(logEntry)
		}
	}()

	safePath, err := m.SafePath(userPath)
	if err != nil {
		logEntry.Success = false
		logEntry.Error = err.Error()
		return nil, err
	}

	// 检查敏感文件
	if sensitiveResult, _ := m.sensitiveDetector.Detect(safePath); sensitiveResult != nil {
		logEntry.SecurityRisk = fmt.Sprintf("敏感文件访问: %s", sensitiveResult.FileNameMatch)
	}

	// 检查文件扩展名
	ext := strings.ToLower(filepath.Ext(safePath))
	m.mu.RLock()
	allowed := m.allowedExt[ext]
	m.mu.RUnlock()

	if !allowed {
		err := fmt.Errorf("file extension %s not allowed", ext)
		logEntry.Success = false
		logEntry.Error = err.Error()
		return nil, err
	}

	// 使用 Lstat 而非 Stat，不跟随符号链接
	info, err := os.Lstat(safePath)
	if err != nil {
		logEntry.Success = false
		logEntry.Error = fmt.Sprintf("cannot stat file: %v", err)
		return nil, fmt.Errorf("cannot stat file: %w", err)
	}

	// 检查是否为常规文件
	if !info.Mode().IsRegular() {
		err := errors.New("not a regular file")
		logEntry.Success = false
		logEntry.Error = err.Error()
		return nil, err
	}

	// 检查是否为符号链接
	if info.Mode()&os.ModeSymlink != 0 {
		err := errors.New("symbolic links not allowed")
		logEntry.Success = false
		logEntry.Error = err.Error()
		return nil, err
	}

	// 检查文件权限
	if permIssue, _ := m.permChecker.CheckFilePermission(safePath); permIssue != nil {
		logEntry.SecurityRisk = permIssue.Message
	}

	data, err := os.ReadFile(safePath)
	if err != nil {
		logEntry.Success = false
		logEntry.Error = err.Error()
		return nil, err
	}

	logEntry.Success = true
	logEntry.FileSize = int64(len(data))
	return data, nil
}

// SafeWrite 安全写入文件
func (m *SafeFileManager) SafeWrite(userPath string, data []byte) error {
	startTime := time.Now()
	logEntry := AccessLog{
		Operation: "write",
		Path:      userPath,
	}

	defer func() {
		logEntry.Duration = time.Since(startTime).Nanoseconds()
		logEntry.FileSize = int64(len(data))
		if m.accessLogger != nil {
			m.accessLogger.Log(logEntry)
		}
	}()

	safePath, err := m.SafePath(userPath)
	if err != nil {
		logEntry.Success = false
		logEntry.Error = err.Error()
		return err
	}

	// 检查文件扩展名
	ext := strings.ToLower(filepath.Ext(safePath))
	m.mu.RLock()
	allowed := m.allowedExt[ext]
	m.mu.RUnlock()

	if !allowed {
		err := fmt.Errorf("file extension %s not allowed", ext)
		logEntry.Success = false
		logEntry.Error = err.Error()
		return err
	}

	// 创建临时文件
	tmpPath := safePath + ".tmp." + randomString(8)

	// 写入临时文件（使用安全的权限）
	if err := os.WriteFile(tmpPath, data, 0600); err != nil {
		logEntry.Success = false
		logEntry.Error = err.Error()
		return fmt.Errorf("cannot write temp file: %w", err)
	}

	// 原子重命名
	if err := os.Rename(tmpPath, safePath); err != nil {
		os.Remove(tmpPath) // 清理临时文件
		logEntry.Success = false
		logEntry.Error = err.Error()
		return fmt.Errorf("cannot rename file: %w", err)
	}

	// 设置最终文件权限
	if err := os.Chmod(safePath, 0644); err != nil {
		logEntry.Success = false
		logEntry.Error = err.Error()
		return fmt.Errorf("cannot set file permissions: %w", err)
	}

	logEntry.Success = true
	return nil
}

// SafeDelete 安全删除文件
func (m *SafeFileManager) SafeDelete(userPath string) error {
	startTime := time.Now()
	logEntry := AccessLog{
		Operation: "delete",
		Path:      userPath,
	}

	defer func() {
		logEntry.Duration = time.Since(startTime).Nanoseconds()
		if m.accessLogger != nil {
			m.accessLogger.Log(logEntry)
		}
	}()

	safePath, err := m.SafePath(userPath)
	if err != nil {
		logEntry.Success = false
		logEntry.Error = err.Error()
		return err
	}

	// 检查文件是否存在
	info, err := os.Lstat(safePath)
	if err != nil {
		logEntry.Success = false
		logEntry.Error = fmt.Sprintf("cannot stat file: %v", err)
		return fmt.Errorf("cannot stat file: %w", err)
	}

	// 只允许删除常规文件
	if !info.Mode().IsRegular() {
		err := errors.New("can only delete regular files")
		logEntry.Success = false
		logEntry.Error = err.Error()
		return err
	}

	// 检查敏感文件
	if sensitiveResult, _ := m.sensitiveDetector.Detect(safePath); sensitiveResult != nil {
		logEntry.SecurityRisk = fmt.Sprintf("删除敏感文件: %s", sensitiveResult.FileNameMatch)
	}

	if err := os.Remove(safePath); err != nil {
		logEntry.Success = false
		logEntry.Error = err.Error()
		return err
	}

	logEntry.Success = true
	return nil
}

// SafeWalk 安全遍历目录
func (m *SafeFileManager) SafeWalk(userPath string, walkFn func(path string, info fs.FileInfo) error) error {
	safePath, err := m.SafePath(userPath)
	if err != nil {
		return err
	}

	return filepath.Walk(safePath, func(path string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 再次验证路径（防止在遍历过程中目录被修改）
		if !strings.HasPrefix(filepath.Clean(path)+string(os.PathSeparator), m.baseDir+string(os.PathSeparator)) {
			return fs.SkipDir
		}

		return walkFn(path, info)
	})
}

// AddAllowedExtension 添加允许的文件扩展名
func (m *SafeFileManager) AddAllowedExtension(ext string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.allowedExt[strings.ToLower(ext)] = true
}

// RemoveAllowedExtension 移除允许的文件扩展名
func (m *SafeFileManager) RemoveAllowedExtension(ext string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.allowedExt, strings.ToLower(ext))
}

// ========== 辅助函数 ==========

func randomString(length int) string {
	b := make([]byte, length)
	rand.Read(b)
	return hex.EncodeToString(b)[:length]
}

// ========== 文件哈希校验 ==========

// FileHash 文件哈希
type FileHash struct {
	Path      string
	Hash      string
	Size      int64
	Timestamp time.Time
}

// HashCalculator 哈希计算器
type HashCalculator struct{}

// CalculateFileHash 计算文件哈希
func (c *HashCalculator) CalculateFileHash(path string) (*FileHash, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	hash := sha256.Sum256(data)

	return &FileHash{
		Path:      path,
		Hash:      hex.EncodeToString(hash[:]),
		Size:      info.Size(),
		Timestamp: time.Now(),
	}, nil
}

// VerifyFileIntegrity 验证文件完整性
func (c *HashCalculator) VerifyFileIntegrity(path, expectedHash string) (bool, error) {
	fileHash, err := c.CalculateFileHash(path)
	if err != nil {
		return false, err
	}

	return fileHash.Hash == expectedHash, nil
}

// ========== 文件权限检查 ==========

// PermissionChecker 权限检查器
type PermissionChecker struct {
	maxFileMode os.FileMode // 最大允许的文件权限
	maxDirMode  os.FileMode // 最大允许的目录权限
}

// NewPermissionChecker 创建权限检查器
func NewPermissionChecker() *PermissionChecker {
	return &PermissionChecker{
		maxFileMode: 0644, // 文件最大权限：rw-r--r--
		maxDirMode:  0755, // 目录最大权限：rwxr-xr-x
	}
}

// PermissionIssue 权限问题
type PermissionIssue struct {
	Path        string
	CurrentMode os.FileMode
	MaxMode     os.FileMode
	Severity    string
	Message     string
}

// CheckFilePermission 检查文件权限
func (c *PermissionChecker) CheckFilePermission(path string) (*PermissionIssue, error) {
	info, err := os.Stat(path)
	if err != nil {
		return nil, err
	}

	mode := info.Mode().Perm()
	issue := &PermissionIssue{
		Path:        path,
		CurrentMode: mode,
		MaxMode:     c.maxFileMode,
		Severity:    "low",
	}

	if info.IsDir() {
		// 目录权限检查
		if mode > c.maxDirMode {
			issue.Severity = "high"
			issue.Message = fmt.Sprintf("目录权限过于开放: %o > %o", mode, c.maxDirMode)
		} else if mode&0002 != 0 { // others write
			issue.Severity = "medium"
			issue.Message = "目录允许其他用户写入"
		}
	} else {
		// 文件权限检查
		if mode > c.maxFileMode {
			issue.Severity = "high"
			issue.Message = fmt.Sprintf("文件权限过于开放: %o > %o", mode, c.maxFileMode)
		} else if mode&0002 != 0 { // others write
			issue.Severity = "medium"
			issue.Message = "文件允许其他用户写入"
		} else if mode&0022 != 0 { // group/others write
			issue.Severity = "low"
			issue.Message = "文件允许组或其他用户写入"
		}
	}

	if issue.Message == "" {
		return nil, nil // 无问题
	}

	return issue, nil
}

// FixPermission 修复文件权限
func (c *PermissionChecker) FixPermission(path string) error {
	info, err := os.Stat(path)
	if err != nil {
		return err
	}

	var targetMode os.FileMode
	if info.IsDir() {
		targetMode = c.maxDirMode
	} else {
		targetMode = c.maxFileMode
	}

	return os.Chmod(path, targetMode)
}

// ========== 敏感文件检测 ==========

// SensitiveFileDetector 敏感文件检测器
type SensitiveFileDetector struct {
	sensitivePatterns []*regexp.Regexp
	sensitiveContent  []*regexp.Regexp
}

// NewSensitiveFileDetector 创建敏感文件检测器
func NewSensitiveFileDetector() *SensitiveFileDetector {
	return &SensitiveFileDetector{
		sensitivePatterns: []*regexp.Regexp{
			// 敏感文件名模式
			regexp.MustCompile(`(?i)password`),
			regexp.MustCompile(`(?i)secret`),
			regexp.MustCompile(`(?i)private[_-]?key`),
			regexp.MustCompile(`(?i)\.pem$`),
			regexp.MustCompile(`(?i)\.key$`),
			regexp.MustCompile(`(?i)\.p12$`),
			regexp.MustCompile(`(?i)\.pfx$`),
			regexp.MustCompile(`(?i)id_rsa`),
			regexp.MustCompile(`(?i)id_ed25519`),
			regexp.MustCompile(`(?i)\.ssh/`),
			regexp.MustCompile(`(?i)\.gnupg/`),
			regexp.MustCompile(`(?i)credentials`),
			regexp.MustCompile(`(?i)api[_-]?key`),
			regexp.MustCompile(`(?i)access[_-]?token`),
			regexp.MustCompile(`(?i)\.env$`),
			regexp.MustCompile(`(?i)\.htpasswd`),
			regexp.MustCompile(`(?i)wp-config\.php`),
			regexp.MustCompile(`(?i)config\.php`),
			regexp.MustCompile(`(?i)settings\.py`),
			regexp.MustCompile(`(?i)\.aws/`),
			regexp.MustCompile(`(?i)\.docker/config\.json`),
		},
		sensitiveContent: []*regexp.Regexp{
			// 敏感内容模式
			regexp.MustCompile(`(?i)password\s*=\s*['"][^'"]+['"]`),
			regexp.MustCompile(`(?i)api[_-]?key\s*=\s*['"][^'"]+['"]`),
			regexp.MustCompile(`(?i)secret[_-]?key\s*=\s*['"][^'"]+['"]`),
			regexp.MustCompile(`(?i)token\s*=\s*['"][^'"]+['"]`),
			regexp.MustCompile(`-----BEGIN\s+(RSA\s+)?PRIVATE\s+KEY-----`),
			regexp.MustCompile(`-----BEGIN\s+OPENSSH\s+PRIVATE\s+KEY-----`),
			regexp.MustCompile(`(?i)mysql://[^:]+:[^@]+@`),
			regexp.MustCompile(`(?i)postgresql://[^:]+:[^@]+@`),
			regexp.MustCompile(`(?i)mongodb://[^:]+:[^@]+@`),
		},
	}
}

// SensitiveFileResult 敏感文件检测结果
type SensitiveFileResult struct {
	Path          string
	FileNameMatch string   // 匹配的文件名模式
	ContentMatch  []string // 匹配的内容模式
	Severity      string
}

// Detect 检测敏感文件
func (d *SensitiveFileDetector) Detect(path string) (*SensitiveFileResult, error) {
	result := &SensitiveFileResult{
		Path:     path,
		Severity: "low",
	}

	// 检查文件名
	basename := filepath.Base(path)
	for _, pattern := range d.sensitivePatterns {
		if pattern.MatchString(path) || pattern.MatchString(basename) {
			result.FileNameMatch = pattern.String()
			result.Severity = "high"
			break
		}
	}

	// 检查文件内容（仅对文本文件）
	if isTextFile(path) {
		data, err := os.ReadFile(path)
		if err == nil && len(data) < 10*1024*1024 { // 限制10MB
			content := string(data)
			for _, pattern := range d.sensitiveContent {
				if pattern.MatchString(content) {
					result.ContentMatch = append(result.ContentMatch, pattern.String())
					if result.Severity != "high" {
						result.Severity = "critical"
					}
				}
			}
		}
	}

	if result.FileNameMatch == "" && len(result.ContentMatch) == 0 {
		return nil, nil // 无敏感信息
	}

	return result, nil
}

// isTextFile 判断是否为文本文件
func isTextFile(path string) bool {
	textExts := map[string]bool{
		".txt": true, ".json": true, ".xml": true, ".yaml": true, ".yml": true,
		".ini": true, ".cfg": true, ".conf": true, ".config": true,
		".env": true, ".properties": true, ".toml": true,
		".py": true, ".js": true, ".ts": true, ".go": true, ".java": true,
		".php": true, ".rb": true, ".sh": true, ".bash": true,
		".md": true, ".rst": true, ".log": true,
	}
	ext := strings.ToLower(filepath.Ext(path))
	return textExts[ext]
}

// ========== 访问日志记录 ==========

// AccessLog 访问日志
type AccessLog struct {
	Timestamp    time.Time `json:"timestamp"`
	Operation    string    `json:"operation"` // read, write, delete, walk
	Path         string    `json:"path"`
	User         string    `json:"user"`      // 可选：用户标识
	ClientIP     string    `json:"client_ip"` // 可选：客户端IP
	Success      bool      `json:"success"`
	Error        string    `json:"error,omitempty"`
	FileSize     int64     `json:"file_size,omitempty"`
	Duration     int64     `json:"duration_ns,omitempty"`
	SecurityRisk string    `json:"security_risk,omitempty"` // 安全风险标记
}

// AccessLogger 访问日志记录器
type AccessLogger struct {
	logs    []AccessLog
	mu      sync.RWMutex
	maxLogs int
	logFile string
	enabled bool
}

// NewAccessLogger 创建访问日志记录器
func NewAccessLogger(logFile string, maxLogs int) *AccessLogger {
	return &AccessLogger{
		logs:    make([]AccessLog, 0),
		maxLogs: maxLogs,
		logFile: logFile,
		enabled: true,
	}
}

// Log 记录访问日志
func (l *AccessLogger) Log(log AccessLog) {
	if !l.enabled {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	// 设置时间戳
	if log.Timestamp.IsZero() {
		log.Timestamp = time.Now()
	}

	// 添加到内存日志
	l.logs = append(l.logs, log)

	// 限制日志数量
	if len(l.logs) > l.maxLogs {
		l.logs = l.logs[len(l.logs)-l.maxLogs:]
	}

	// 写入文件
	if l.logFile != "" {
		l.writeToFile(log)
	}
}

// writeToFile 写入日志文件
func (l *AccessLogger) writeToFile(log AccessLog) {
	data, err := json.Marshal(log)
	if err != nil {
		return
	}

	f, err := os.OpenFile(l.logFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	if err != nil {
		return
	}
	defer f.Close()

	f.Write(data)
	f.Write([]byte("\n"))
}

// GetLogs 获取日志
func (l *AccessLogger) GetLogs(limit int) []AccessLog {
	l.mu.RLock()
	defer l.mu.RUnlock()

	if limit <= 0 || limit > len(l.logs) {
		return append([]AccessLog{}, l.logs...)
	}

	start := len(l.logs) - limit
	return append([]AccessLog{}, l.logs[start:]...)
}

// GetRecentFailures 获取最近的失败操作
func (l *AccessLogger) GetRecentFailures(count int) []AccessLog {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var failures []AccessLog
	for i := len(l.logs) - 1; i >= 0 && len(failures) < count; i-- {
		if !l.logs[i].Success {
			failures = append(failures, l.logs[i])
		}
	}
	return failures
}

// GetSecurityRisks 获取有安全风险的日志
func (l *AccessLogger) GetSecurityRisks() []AccessLog {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var risks []AccessLog
	for _, log := range l.logs {
		if log.SecurityRisk != "" {
			risks = append(risks, log)
		}
	}
	return risks
}

// Clear 清空日志
func (l *AccessLogger) Clear() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.logs = make([]AccessLog, 0)
}

// Enable 启用日志
func (l *AccessLogger) Enable() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = true
}

// Disable 禁用日志
func (l *AccessLogger) Disable() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.enabled = false
}

// ========== 增强的安全审计 ==========

// AuditResult 审计结果
type AuditResult struct {
	Path              string
	Issues            []string
	Severity          string
	CheckedAt         time.Time
	PermissionIssue   *PermissionIssue
	SensitiveFileInfo *SensitiveFileResult
}

// SecurityAuditor 安全审计器
type SecurityAuditor struct {
	manager           *SafeFileManager
	permChecker       *PermissionChecker
	sensitiveDetector *SensitiveFileDetector
}

// NewSecurityAuditor 创建安全审计器
func NewSecurityAuditor(manager *SafeFileManager) *SecurityAuditor {
	return &SecurityAuditor{
		manager:           manager,
		permChecker:       NewPermissionChecker(),
		sensitiveDetector: NewSensitiveFileDetector(),
	}
}

// AuditPath 审计路径安全性
func (a *SecurityAuditor) AuditPath(ctx context.Context, userPath string) (*AuditResult, error) {
	result := &AuditResult{
		Path:      userPath,
		Issues:    []string{},
		CheckedAt: time.Now(),
		Severity:  "low",
	}

	// 测试路径是否安全
	safePath, err := a.manager.SafePath(userPath)
	if err != nil {
		result.Issues = append(result.Issues, err.Error())
		result.Severity = "high"
		return result, nil
	}

	// 检查敏感文件名和内容
	sensitiveResult, err := a.sensitiveDetector.Detect(safePath)
	if err == nil && sensitiveResult != nil {
		result.SensitiveFileInfo = sensitiveResult
		result.Issues = append(result.Issues, fmt.Sprintf("敏感文件检测: %s", sensitiveResult.FileNameMatch))
		if sensitiveResult.Severity == "critical" {
			result.Severity = "critical"
		} else if result.Severity != "critical" && sensitiveResult.Severity == "high" {
			result.Severity = "high"
		}
	}

	// 如果文件存在，检查权限
	if info, err := os.Stat(safePath); err == nil {
		permIssue, err := a.permChecker.CheckFilePermission(safePath)
		if err == nil && permIssue != nil {
			result.PermissionIssue = permIssue
			result.Issues = append(result.Issues, permIssue.Message)
			if result.Severity != "critical" && permIssue.Severity == "high" {
				result.Severity = "high"
			} else if result.Severity == "low" && permIssue.Severity == "medium" {
				result.Severity = "medium"
			}
		}

		// 检查是否为符号链接
		if info.Mode()&os.ModeSymlink != 0 {
			result.Issues = append(result.Issues, "文件是符号链接")
			if result.Severity == "low" {
				result.Severity = "medium"
			}
		}
	}

	return result, nil
}

// AuditDirectory 审计目录安全性
func (a *SecurityAuditor) AuditDirectory(ctx context.Context, dirPath string) ([]*AuditResult, error) {
	safePath, err := a.manager.SafePath(dirPath)
	if err != nil {
		return nil, err
	}

	var results []*AuditResult

	err = filepath.Walk(safePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过错误
		}

		relPath, err := filepath.Rel(a.manager.baseDir, path)
		if err != nil {
			return nil
		}

		result, err := a.AuditPath(ctx, relPath)
		if err != nil {
			return nil
		}

		if len(result.Issues) > 0 {
			results = append(results, result)
		}

		return nil
	})

	return results, err
}

// FixPermissions 修复目录下所有文件权限
func (a *SecurityAuditor) FixPermissions(ctx context.Context, dirPath string) (int, []error) {
	safePath, err := a.manager.SafePath(dirPath)
	if err != nil {
		return 0, []error{err}
	}

	fixed := 0
	var errs []error

	_ = filepath.Walk(safePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		permIssue, err := a.permChecker.CheckFilePermission(path)
		if err != nil {
			return nil
		}

		if permIssue != nil {
			if err := a.permChecker.FixPermission(path); err != nil {
				errs = append(errs, fmt.Errorf("修复权限失败 %s: %w", path, err))
			} else {
				fixed++
			}
		}

		return nil
	})

	return fixed, errs
}
