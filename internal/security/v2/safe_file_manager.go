// Package securityv2 提供安全模块 v2 版本
package securityv2

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// SafeFileManager 安全文件管理器
// 解决路径遍历漏洞和竞态条件问题
type SafeFileManager struct {
	baseDir    string
	allowedExt map[string]bool
	mu         sync.RWMutex
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
	}
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
	safePath, err := m.SafePath(userPath)
	if err != nil {
		return nil, err
	}

	// 检查文件扩展名
	ext := strings.ToLower(filepath.Ext(safePath))
	m.mu.RLock()
	allowed := m.allowedExt[ext]
	m.mu.RUnlock()

	if !allowed {
		return nil, fmt.Errorf("file extension %s not allowed", ext)
	}

	// 使用 Lstat 而非 Stat，不跟随符号链接
	info, err := os.Lstat(safePath)
	if err != nil {
		return nil, fmt.Errorf("cannot stat file: %w", err)
	}

	// 检查是否为常规文件
	if !info.Mode().IsRegular() {
		return nil, errors.New("not a regular file")
	}

	// 检查是否为符号链接
	if info.Mode()&os.ModeSymlink != 0 {
		return nil, errors.New("symbolic links not allowed")
	}

	return os.ReadFile(safePath)
}

// SafeWrite 安全写入文件
func (m *SafeFileManager) SafeWrite(userPath string, data []byte) error {
	safePath, err := m.SafePath(userPath)
	if err != nil {
		return err
	}

	// 检查文件扩展名
	ext := strings.ToLower(filepath.Ext(safePath))
	m.mu.RLock()
	allowed := m.allowedExt[ext]
	m.mu.RUnlock()

	if !allowed {
		return fmt.Errorf("file extension %s not allowed", ext)
	}

	// 创建临时文件
	tmpPath := safePath + ".tmp." + randomString(8)

	// 写入临时文件
	if err := os.WriteFile(tmpPath, data, 0644); err != nil {
		return fmt.Errorf("cannot write temp file: %w", err)
	}

	// 原子重命名
	if err := os.Rename(tmpPath, safePath); err != nil {
		os.Remove(tmpPath) // 清理临时文件
		return fmt.Errorf("cannot rename file: %w", err)
	}

	return nil
}

// SafeDelete 安全删除文件
func (m *SafeFileManager) SafeDelete(userPath string) error {
	safePath, err := m.SafePath(userPath)
	if err != nil {
		return err
	}

	// 检查文件是否存在
	info, err := os.Lstat(safePath)
	if err != nil {
		return fmt.Errorf("cannot stat file: %w", err)
	}

	// 只允许删除常规文件
	if !info.Mode().IsRegular() {
		return errors.New("can only delete regular files")
	}

	return os.Remove(safePath)
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

// ========== 安全审计 ==========

// AuditResult 审计结果
type AuditResult struct {
	Path      string
	Issues    []string
	Severity  string
	CheckedAt time.Time
}

// SecurityAuditor 安全审计器
type SecurityAuditor struct {
	manager *SafeFileManager
}

// NewSecurityAuditor 创建安全审计器
func NewSecurityAuditor(manager *SafeFileManager) *SecurityAuditor {
	return &SecurityAuditor{manager: manager}
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
	_, err := a.manager.SafePath(userPath)
	if err != nil {
		result.Issues = append(result.Issues, err.Error())
		result.Severity = "high"
		return result, nil
	}

	// 检查敏感文件名
	if strings.Contains(strings.ToLower(userPath), "password") ||
		strings.Contains(strings.ToLower(userPath), "secret") ||
		strings.Contains(strings.ToLower(userPath), "key") {
		result.Issues = append(result.Issues, "sensitive filename detected")
		result.Severity = "medium"
	}

	return result, nil
}
