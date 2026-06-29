// Package filepreview 提供加密 PDF 预览解锁功能。
// 支持检测加密 PDF、密码验证、解锁后生成预览缩略图，参考飞牛 fnOS v1.2。
package filepreview

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// EncryptedPDFManager 加密 PDF 管理器
type EncryptedPDFManager struct {
	mu sync.RWMutex

	// 密码尝试限制
	maxAttempts    int
	blockDuration  time.Duration
	attempts       map[string]int    // fileHash → 尝试次数
	blocked        map[string]time.Time // fileHash → 封锁截止时间
}

// NewEncryptedPDFManager 创建加密 PDF 管理器
func NewEncryptedPDFManager() *EncryptedPDFManager {
	return &EncryptedPDFManager{
		maxAttempts:   5,
		blockDuration: 15 * time.Minute,
		attempts:      make(map[string]int),
		blocked:       make(map[string]time.Time),
	}
}

// PDFOwnerPassword PDF 所有者密码错误
var ErrPDFOwnerPassword = fmt.Errorf("所有者密码错误")

// PDFPasswordRequired 需要密码
var ErrPDFPasswordRequired = fmt.Errorf("PDF 文件已加密，需要密码")

// PDFPasswordIncorrect 密码错误
var ErrPDFPasswordIncorrect = fmt.Errorf("密码错误")

// PDFAttemptsExceeded 尝试次数过多
var ErrPDFAttemptsExceeded = fmt.Errorf("密码尝试次数过多，已封锁")

// PDFNotEncrypted PDF 未加密
var ErrPDFNotEncrypted = fmt.Errorf("PDF 文件未加密")

// IsEncryptedPDF 检测 PDF 文件是否加密
func (m *EncryptedPDFManager) IsEncryptedPDF(ctx context.Context, filePath string) (bool, error) {
	// 读取 PDF 文件头部
	data, err := os.ReadFile(filePath)
	if err != nil {
		return false, fmt.Errorf("读取文件失败: %w", err)
	}

	return IsEncryptedPDFBytes(data), nil
}

// IsEncryptedPDFBytes 检测 PDF 字节流是否加密
func IsEncryptedPDFBytes(data []byte) bool {
	content := string(data)
	// 检查 /Encrypt 字典标记
	return strings.Contains(content, "/Encrypt")
}

// UnlockPDF 用密码解锁加密 PDF 并生成预览
func (m *EncryptedPDFManager) UnlockPDF(ctx context.Context, filePath, password string) (string, error) {
	// 获取文件哈希用于追踪
	fileHash, err := getFileHash(filePath)
	if err != nil {
		return "", fmt.Errorf("计算文件哈希失败: %w", err)
	}

	// 检查是否被封锁
	if m.isBlocked(fileHash) {
		return "", ErrPDFAttemptsExceeded
	}

	// 检查是否加密
	encrypted, err := m.IsEncryptedPDF(ctx, filePath)
	if err != nil {
		return "", err
	}
	if !encrypted {
		return "", ErrPDFNotEncrypted
	}

	// 使用 qpdf 验证密码并解密
	decryptedPath := fmt.Sprintf("%s.decrypted.pdf", filePath)

	cmd := exec.CommandContext(ctx, "qpdf",
		"--password", password,
		"--decrypt",
		filePath,
		decryptedPath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		// 密码错误
		if strings.Contains(stderrStr, "invalid password") || strings.Contains(stderrStr, "Password incorrect") {
			m.recordFailedAttempt(fileHash)
			remaining := m.maxAttempts - m.attempts[fileHash]
			if remaining <= 0 {
				return "", ErrPDFAttemptsExceeded
			}
			return "", fmt.Errorf("%w: 剩余尝试次数 %d", ErrPDFPasswordIncorrect, remaining)
		}
		return "", fmt.Errorf("解密 PDF 失败: %s: %w", stderrStr, err)
	}

	// 成功解锁，重置尝试次数
	m.resetAttempts(fileHash)

	return decryptedPath, nil
}

// GenerateEncryptedPDFPreview 生成加密 PDF 的预览
func (m *EncryptedPDFManager) GenerateEncryptedPDFPreview(ctx context.Context, filePath, password string, pageNum int) (*PreviewResult, error) {
	if pageNum <= 0 {
		pageNum = 1
	}

	// 解锁 PDF
	decryptedPath, err := m.UnlockPDF(ctx, filePath, password)
	if err != nil {
		return nil, err
	}
	defer os.Remove(decryptedPath) // 清理解密文件

	// 生成预览图
	outputPath := fmt.Sprintf("%s_enc_p%d.jpg", filePath, pageNum)

	cmd := exec.CommandContext(ctx, "pdftoppm",
		"-f", fmt.Sprintf("%d", pageNum),
		"-l", fmt.Sprintf("%d", pageNum),
		"-jpeg",
		"-r", "150",
		"-scale-to", "1200",
		decryptedPath,
		strings.TrimSuffix(outputPath, filepath.Ext(outputPath)),
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("生成预览失败: %s: %w", stderr.String(), err)
	}

	// 检查生成的文件
	actualPath := fmt.Sprintf("%s_enc_p%d-1.jpg", filePath, pageNum)
	if _, err := os.Stat(actualPath); err != nil {
		return nil, fmt.Errorf("%w: 预览生成失败", ErrGenerationFailed)
	}

	// 重命名到目标路径
	if err := os.Rename(actualPath, outputPath); err != nil {
		return nil, err
	}

	stat, _ := os.Stat(outputPath)
	return &PreviewResult{
		FilePath:    filePath,
		FileType:    FileTypeDocument,
		PreviewPath: outputPath,
		ContentType: "image/jpeg",
		Width:       1200,
		FileSize:    stat.Size(),
		GeneratedAt: stat.ModTime(),
		PageNumber:  pageNum,
		Metadata: map[string]string{
			"encrypted": "true",
			"unlocked":  "true",
		},
	}, nil
}

// VerifyPassword 验证密码是否正确（不生成预览）
func (m *EncryptedPDFManager) VerifyPassword(ctx context.Context, filePath, password string) error {
	// 先检查是否加密
	encrypted, err := m.IsEncryptedPDF(ctx, filePath)
	if err != nil {
		return fmt.Errorf("检查加密状态失败: %w", err)
	}
	if !encrypted {
		return ErrPDFNotEncrypted
	}

	fileHash, err := getFileHash(filePath)
	if err != nil {
		return fmt.Errorf("计算文件哈希失败: %w", err)
	}

	if m.isBlocked(fileHash) {
		return ErrPDFAttemptsExceeded
	}

	// 用 qpdf 检查密码
	cmd := exec.CommandContext(ctx, "qpdf",
		"--password", password,
		"--check",
		filePath,
	)
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		stderrStr := stderr.String()
		if strings.Contains(stderrStr, "invalid password") || strings.Contains(stderrStr, "Password incorrect") {
			m.recordFailedAttempt(fileHash)
			remaining := m.maxAttempts - m.attempts[fileHash]
			if remaining <= 0 {
				return ErrPDFAttemptsExceeded
			}
			return fmt.Errorf("%w: 剩余尝试次数 %d", ErrPDFPasswordIncorrect, remaining)
		}
		return fmt.Errorf("验证密码失败: %w", err)
	}

	m.resetAttempts(fileHash)
	return nil
}

// SetMaxAttempts 设置最大尝试次数和封锁时长
func (m *EncryptedPDFManager) SetMaxAttempts(max int, blockDuration time.Duration) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.maxAttempts = max
	m.blockDuration = blockDuration
}

// ClearExpiredBlocks 清理过期的封锁记录
func (m *EncryptedPDFManager) ClearExpiredBlocks() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for hash, until := range m.blocked {
		if now.After(until) {
			delete(m.blocked, hash)
			delete(m.attempts, hash)
		}
	}
}

// isBlocked 检查文件是否被封锁
func (m *EncryptedPDFManager) isBlocked(fileHash string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	until, ok := m.blocked[fileHash]
	if !ok {
		return false
	}
	return time.Now().Before(until)
}

// recordFailedAttempt 记录失败尝试
func (m *EncryptedPDFManager) recordFailedAttempt(fileHash string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.attempts[fileHash]++
	if m.attempts[fileHash] >= m.maxAttempts {
		m.blocked[fileHash] = time.Now().Add(m.blockDuration)
	}
}

// resetAttempts 重置尝试次数
func (m *EncryptedPDFManager) resetAttempts(fileHash string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.attempts, fileHash)
	delete(m.blocked, fileHash)
}

// getFileHash 计算文件内容的 SHA256 哈希
func getFileHash(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:]), nil
}
