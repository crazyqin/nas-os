// Package privacyvault - shredder.go 实现安全擦除功能，包括多次覆写删除、
// SSD TRIM 安全擦除和临时文件清理，确保数据不可恢复。
package privacyvault

import (
	"crypto/rand"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 擦除模式常量
// ============================================================

// ShredMode 擦除模式
type ShredMode string

const (
	// ShredModeStandard 标准覆写（3次：随机、补零、随机）
	ShredModeStandard ShredMode = "standard"
	// ShredModeDoD5220 美国国防部 DoD 5220.22-M 标准（7次覆写）
	ShredModeDoD5220 ShredMode = "dod5220"
	// ShredModeGutmann Gutmann 方法（35次覆写）
	ShredModeGutmann ShredMode = "gutmann"
	// ShredModeRandom 纯随机覆写
	ShredModeRandom ShredMode = "random"
)

// ============================================================
// 数据结构
// ============================================================

// ShredResult 擦除操作结果
type ShredResult struct {
	// FilePath 文件路径
	FilePath string `json:"file_path"`
	// Mode 擦除模式
	Mode ShredMode `json:"mode"`
	// Passes 实际覆写次数
	Passes int `json:"passes"`
	// BytesWritten 总写入字节数
	BytesWritten int64 `json:"bytes_written"`
	// Duration 操作耗时
	Duration time.Duration `json:"duration"`
	// Success 是否成功
	Success bool `json:"success"`
	// Error 错误信息
	Error string `json:"error,omitempty"`
	// Timestamp 操作时间
	Timestamp time.Time `json:"timestamp"`
}

// ShredConfig 擦除配置
type ShredConfig struct {
	// Mode 擦除模式
	Mode ShredMode `json:"mode"`
	// Passes 自定义覆写次数（仅 random 模式生效）
	Passes int `json:"passes"`
	// SyncAfterWrite 每次覆写后是否同步到磁盘
	SyncAfterWrite bool `json:"sync_after_write"`
	// DeleteAfterWipe 覆写后是否删除文件
	DeleteAfterWipe bool `json:"delete_after_wipe"`
	// TempDirPatterns 临时文件目录模式
	TempDirPatterns []string `json:"temp_dir_patterns"`
	// MaxFileSize 最大可擦除文件大小（字节）
	MaxFileSize int64 `json:"max_file_size"`
}

// DefaultShredConfig 默认擦除配置
func DefaultShredConfig() *ShredConfig {
	return &ShredConfig{
		Mode:            ShredModeStandard,
		Passes:          3,
		SyncAfterWrite:  true,
		DeleteAfterWipe: true,
		TempDirPatterns: []string{"/tmp/*", "/var/tmp/*", os.TempDir()},
		MaxFileSize:     10 * 1024 * 1024 * 1024, // 10GB
	}
}

// Shredder 安全擦除器
type Shredder struct {
	config *ShredConfig
	mu     sync.Mutex
}

// NewShredder 创建安全擦除器
func NewShredder(config *ShredConfig) *Shredder {
	if config == nil {
		config = DefaultShredConfig()
	}
	return &Shredder{config: config}
}

// ============================================================
// 公共方法
// ============================================================

// ShredFile 安全擦除单个文件
func (s *Shredder) ShredFile(filePath string) (*ShredResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	startTime := time.Now()
	result := &ShredResult{
		FilePath:  filePath,
		Mode:      s.config.Mode,
		Timestamp: startTime,
	}

	// 获取文件信息
	info, err := os.Stat(filePath)
	if err != nil {
		result.Error = err.Error()
		return result, NewPrivacyVaultError("FILE_NOT_FOUND", "文件不存在", err)
	}

	if info.IsDir() {
		result.Error = "目标是目录，不是文件"
		return result, NewPrivacyVaultError("IS_DIRECTORY", "目标是目录", nil)
	}

	if info.Size() > s.config.MaxFileSize {
		result.Error = fmt.Sprintf("文件大小超出限制（%d > %d）", info.Size(), s.config.MaxFileSize)
		return result, NewPrivacyVaultError("FILE_TOO_LARGE", "文件大小超出限制", nil)
	}

	// 执行覆写
	passes := s.getPassesForMode()
	totalWritten, err := s.overwriteFile(filePath, info.Size(), passes)
	if err != nil {
		result.Error = err.Error()
		return result, err
	}

	result.Passes = passes
	result.BytesWritten = totalWritten
	result.Success = true

	// 删除文件
	if s.config.DeleteAfterWipe {
		if err := os.Remove(filePath); err != nil {
			result.Success = false
			result.Error = fmt.Sprintf("覆写成功但删除失败: %v", err)
		}
	}

	result.Duration = time.Since(startTime)
	return result, nil
}

// ShredData 安全擦除内存中的数据（置零）
func (s *Shredder) ShredData(data []byte) {
	for i := range data {
		data[i] = 0
	}
}

// ShredDirectory 递归擦除目录中的所有文件
func (s *Shredder) ShredDirectory(dirPath string) ([]*ShredResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	var results []*ShredResult
	var firstErr error

	err := filepath.Walk(dirPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}

		result := &ShredResult{
			FilePath:  path,
			Mode:      s.config.Mode,
			Timestamp: time.Now(),
		}

		passes := s.getPassesForMode()
		totalWritten, err := s.overwriteFile(path, info.Size(), passes)
		if err != nil {
			result.Error = err.Error()
			if firstErr == nil {
				firstErr = err
			}
		} else {
			result.Passes = passes
			result.BytesWritten = totalWritten
			result.Success = true

			if s.config.DeleteAfterWipe {
				os.Remove(path)
			}
		}

		result.Duration = time.Since(result.Timestamp)
		results = append(results, result)
		return nil
	})

	if err != nil && firstErr == nil {
		firstErr = err
	}

	return results, firstErr
}

// CleanupTempFiles 清理临时文件
func (s *Shredder) CleanupTempFiles() ([]*ShredResult, error) {
	var results []*ShredResult
	var firstErr error

	for _, pattern := range s.config.TempDirPatterns {
		matches, err := filepath.Glob(pattern)
		if err != nil {
			continue
		}

		for _, match := range matches {
			info, err := os.Stat(match)
			if err != nil {
				continue
			}

			if info.IsDir() {
				dirResults, err := s.ShredDirectory(match)
				if err != nil && firstErr == nil {
					firstErr = err
				}
				results = append(results, dirResults...)
			} else if !info.IsDir() {
				result, err := s.ShredFile(match)
				if err != nil && firstErr == nil {
					firstErr = err
				}
				if result != nil {
					results = append(results, result)
				}
			}
		}
	}

	return results, firstErr
}

// SSDTrimFlush 执行 SSD TRIM 刷新（模拟）
// 实际实现需要调用 fstrim 或设备特定命令
func (s *Shredder) SSDTrimFlush(mountPoint string) error {
	// 检查挂载点
	if mountPoint == "" {
		return NewPrivacyVaultError("INVALID_MOUNT_POINT", "挂载点为空", nil)
	}

	// 实际实现中应执行 fstrim 命令
	// cmd := exec.Command("fstrim", "-v", mountPoint)
	// return cmd.Run()
	return nil
}

// GetStats 获取擦除统计信息
func (s *Shredder) GetStats(results []*ShredResult) *ShredStats {
	stats := &ShredStats{}
	for _, r := range results {
		stats.TotalFiles++
		stats.TotalBytes += r.BytesWritten
		stats.TotalPasses += r.Passes
		stats.TotalDuration += r.Duration
		if r.Success {
			stats.SuccessCount++
		} else {
			stats.FailCount++
		}
	}
	return stats
}

// ============================================================
// 内部方法
// ============================================================

func (s *Shredder) getPassesForMode() int {
	switch s.config.Mode {
	case ShredModeStandard:
		return 3
	case ShredModeDoD5220:
		return 7
	case ShredModeGutmann:
		return 35
	case ShredModeRandom:
		return s.config.Passes
	default:
		return 3
	}
}

func (s *Shredder) overwriteFile(filePath string, fileSize int64, passes int) (int64, error) {
	f, err := os.OpenFile(filePath, os.O_WRONLY, 0)
	if err != nil {
		return 0, NewPrivacyVaultError("FILE_OPEN_FAILED", "打开文件失败", err)
	}
	defer f.Close()

	var totalWritten int64
	bufSize := 4096
	buf := make([]byte, bufSize)

	for pass := 0; pass < passes; pass++ {
		if _, err := f.Seek(0, 0); err != nil {
			return totalWritten, NewPrivacyVaultError("SEEK_FAILED", "文件定位失败", err)
		}

		remaining := fileSize
		for remaining > 0 {
			toWrite := int64(bufSize)
			if toWrite > remaining {
				toWrite = remaining
			}

			// 生成随机数据或特定模式
			if err := s.fillBuffer(buf[:toWrite], pass); err != nil {
				return totalWritten, err
			}

			n, err := f.Write(buf[:toWrite])
			if err != nil {
				return totalWritten, NewPrivacyVaultError("WRITE_FAILED", "写入失败", err)
			}
			totalWritten += int64(n)
			remaining -= int64(n)
		}

		// 同步到磁盘
		if s.config.SyncAfterWrite {
			if err := f.Sync(); err != nil {
				return totalWritten, NewPrivacyVaultError("SYNC_FAILED", "同步磁盘失败", err)
			}
		}
	}

	return totalWritten, nil
}

func (s *Shredder) fillBuffer(buf []byte, pass int) error {
	switch s.config.Mode {
	case ShredModeStandard:
		switch pass % 3 {
		case 0, 2:
			if _, err := io.ReadFull(rand.Reader, buf); err != nil {
				return err
			}
		case 1:
			for i := range buf {
				buf[i] = 0
			}
		}
	case ShredModeDoD5220:
		switch pass % 7 {
		case 0:
			for i := range buf {
				buf[i] = 0xFF
			}
		case 1:
			for i := range buf {
				buf[i] = 0x00
			}
		case 2, 4, 6:
			if _, err := io.ReadFull(rand.Reader, buf); err != nil {
				return err
			}
		case 3:
			for i := range buf {
				buf[i] = 0xAA
			}
		case 5:
			for i := range buf {
				buf[i] = 0x55
			}
		}
	default:
		if _, err := io.ReadFull(rand.Reader, buf); err != nil {
			return err
		}
	}
	return nil
}

// ============================================================
// 统计结构
// ============================================================

// ShredStats 擦除统计信息
type ShredStats struct {
	// TotalFiles 处理文件总数
	TotalFiles int `json:"total_files"`
	// SuccessCount 成功数
	SuccessCount int `json:"success_count"`
	// FailCount 失败数
	FailCount int `json:"fail_count"`
	// TotalBytes 总擦除字节数
	TotalBytes int64 `json:"total_bytes"`
	// TotalPasses 总覆写次数
	TotalPasses int `json:"total_passes"`
	// TotalDuration 总耗时
	TotalDuration time.Duration `json:"total_duration"`
}

// IsTempFile 判断文件是否为临时文件
func IsTempFile(path string) bool {
	tmpDir := os.TempDir()
	if strings.HasPrefix(path, tmpDir) {
		return true
	}
	base := filepath.Base(path)
	return strings.HasPrefix(base, ".") || strings.HasSuffix(base, ".tmp") ||
		strings.HasSuffix(base, ".temp") || strings.HasSuffix(base, ".swp")
}
