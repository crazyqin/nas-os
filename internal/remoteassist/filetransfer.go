// filetransfer.go - 安全文件传输
package remoteassist

import (
	"crypto/sha256"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FileTransferManager 文件传输管理器.
type FileTransferManager struct {
	transfers map[string]*FileTransfer
	files     map[string]*FileInfo
	mu        sync.RWMutex
}

// NewFileTransferManager 创建文件传输管理器.
func NewFileTransferManager() *FileTransferManager {
	return &FileTransferManager{
		transfers: make(map[string]*FileTransfer),
		files:     make(map[string]*FileInfo),
	}
}

// StartUpload 开始上传.
func (m *FileTransferManager) StartUpload(sessionID string, fileName string, fileSize int64) (*FileTransfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	transfer := &FileTransfer{
		ID:          uuid.New().String(),
		SessionID:   sessionID,
		Direction:   "upload",
		FileName:    fileName,
		FileSize:    fileSize,
		Transferred: 0,
		Progress:    0,
		Status:      "pending",
		StartedAt:   time.Now(),
	}

	m.transfers[transfer.ID] = transfer

	log.Printf("📤 开始上传: %s, 文件: %s, 大小: %d",
		transfer.ID, fileName, fileSize)

	return transfer, nil
}

// StartDownload 开始下载.
func (m *FileTransferManager) StartDownload(sessionID string, filePath string) (*FileTransfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查文件是否存在
	info, err := os.Stat(filePath)
	if err != nil {
		return nil, fmt.Errorf("文件不存在: %w", err)
	}

	transfer := &FileTransfer{
		ID:          uuid.New().String(),
		SessionID:   sessionID,
		Direction:   "download",
		FileName:    info.Name(),
		FilePath:    filePath,
		FileSize:    info.Size(),
		Transferred: 0,
		Progress:    0,
		Status:      "pending",
		StartedAt:   time.Now(),
	}

	m.transfers[transfer.ID] = transfer

	log.Printf("📥 开始下载: %s, 文件: %s, 大小: %d",
		transfer.ID, info.Name(), info.Size())

	return transfer, nil
}

// UpdateProgress 更新传输进度.
func (m *FileTransferManager) UpdateProgress(transferID string, transferred int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	transfer, exists := m.transfers[transferID]
	if !exists {
		return fmt.Errorf("传输不存在: %s", transferID)
	}

	transfer.Transferred = transferred
	transfer.Progress = float64(transferred) / float64(transfer.FileSize) * 100

	// 计算传输速度
	elapsed := time.Since(transfer.StartedAt).Seconds()
	if elapsed > 0 {
		transfer.Speed = int64(float64(transferred) / elapsed)
	}

	if transferred >= transfer.FileSize {
		transfer.Status = "completed"
		now := time.Now()
		transfer.CompletedAt = &now
	} else {
		transfer.Status = "transferring"
	}

	return nil
}

// CompleteTransfer 完成传输.
func (m *FileTransferManager) CompleteTransfer(transferID string, hash string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	transfer, exists := m.transfers[transferID]
	if !exists {
		return fmt.Errorf("传输不存在: %s", transferID)
	}

	transfer.Status = "completed"
	transfer.Progress = 100
	transfer.Transferred = transfer.FileSize
	transfer.Hash = hash
	now := time.Now()
	transfer.CompletedAt = &now

	log.Printf("✅ 传输完成: %s, 耗时: %.2f秒",
		transferID, now.Sub(transfer.StartedAt).Seconds())

	return nil
}

// FailTransfer 传输失败.
func (m *FileTransferManager) FailTransfer(transferID string, err error) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	transfer, exists := m.transfers[transferID]
	if !exists {
		return fmt.Errorf("传输不存在: %s", transferID)
	}

	transfer.Status = "failed"
	transfer.Error = err.Error()

	log.Printf("❌ 传输失败: %s, 错误: %v", transferID, err)
	return nil
}

// CancelTransfer 取消传输.
func (m *FileTransferManager) CancelTransfer(transferID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	transfer, exists := m.transfers[transferID]
	if !exists {
		return fmt.Errorf("传输不存在: %s", transferID)
	}

	if transfer.Status == "completed" {
		return fmt.Errorf("传输已完成，无法取消")
	}

	transfer.Status = "cancelled"

	log.Printf("🚫 取消传输: %s", transferID)
	return nil
}

// GetTransfer 获取传输信息.
func (m *FileTransferManager) GetTransfer(transferID string) (*FileTransfer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	transfer, exists := m.transfers[transferID]
	if !exists {
		return nil, fmt.Errorf("传输不存在: %s", transferID)
	}
	return transfer, nil
}

// ListTransfers 列出传输.
func (m *FileTransferManager) ListTransfers(sessionID string) []*FileTransfer {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*FileTransfer, 0)
	for _, transfer := range m.transfers {
		if sessionID != "" && transfer.SessionID != sessionID {
			continue
		}
		result = append(result, transfer)
	}
	return result
}

// ListFiles 列出文件.
func (m *FileTransferManager) ListFiles(dirPath string) ([]*FileInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	entries, err := os.ReadDir(dirPath)
	if err != nil {
		return nil, fmt.Errorf("读取目录失败: %w", err)
	}

	result := make([]*FileInfo, 0, len(entries))
	for _, entry := range entries {
		info, err := entry.Info()
		if err != nil {
			continue
		}

		result = append(result, &FileInfo{
			Name:     entry.Name(),
			Path:     filepath.Join(dirPath, entry.Name()),
			Size:     info.Size(),
			IsDir:    entry.IsDir(),
			ModTime:  info.ModTime(),
			Mode:     info.Mode().String(),
			MimeType: getMimeType(entry.Name()),
		})
	}

	return result, nil
}

// getMimeType 获取MIME类型.
func getMimeType(filename string) string {
	ext := filepath.Ext(filename)
	mimeTypes := map[string]string{
		".txt":  "text/plain",
		".html": "text/html",
		".css":  "text/css",
		".js":   "application/javascript",
		".json": "application/json",
		".pdf":  "application/pdf",
		".jpg":  "image/jpeg",
		".jpeg": "image/jpeg",
		".png":  "image/png",
		".gif":  "image/gif",
		".mp4":  "video/mp4",
		".mp3":  "audio/mpeg",
		".zip":  "application/zip",
		".tar":  "application/x-tar",
		".gz":   "application/gzip",
	}

	if mime, exists := mimeTypes[ext]; exists {
		return mime
	}
	return "application/octet-stream"
}

// CalculateFileHash 计算文件哈希.
func (m *FileTransferManager) CalculateFileHash(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", fmt.Errorf("计算哈希失败: %w", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

// VerifyFile 验证文件完整性.
func (m *FileTransferManager) VerifyFile(filePath string, expectedHash string) (bool, error) {
	hash, err := m.CalculateFileHash(filePath)
	if err != nil {
		return false, err
	}

	return hash == expectedHash, nil
}

// GetTransferStats 获取传输统计.
func (m *FileTransferManager) GetTransferStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := map[string]interface{}{
		"total_transfers":  0,
		"active_transfers": 0,
		"completed":        0,
		"failed":           0,
		"total_bytes":      int64(0),
		"avg_speed":        int64(0),
	}

	totalSpeed := int64(0)
	speedCount := 0

	for _, transfer := range m.transfers {
		stats["total_transfers"] = stats["total_transfers"].(int) + 1

		switch transfer.Status {
		case "transferring", "pending":
			stats["active_transfers"] = stats["active_transfers"].(int) + 1
		case "completed":
			stats["completed"] = stats["completed"].(int) + 1
			stats["total_bytes"] = stats["total_bytes"].(int64) + transfer.Transferred
		case "failed":
			stats["failed"] = stats["failed"].(int) + 1
		}

		if transfer.Speed > 0 {
			totalSpeed += transfer.Speed
			speedCount++
		}
	}

	if speedCount > 0 {
		stats["avg_speed"] = totalSpeed / int64(speedCount)
	}

	return stats
}

// CleanupTransfers 清理旧传输记录.
func (m *FileTransferManager) CleanupTransfers(olderThan time.Duration) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-olderThan)
	cleaned := 0

	for id, transfer := range m.transfers {
		if transfer.Status == "completed" || transfer.Status == "failed" {
			if transfer.StartedAt.Before(cutoff) {
				delete(m.transfers, id)
				cleaned++
			}
		}
	}

	if cleaned > 0 {
		log.Printf("🧹 清理传输记录: %d 条", cleaned)
	}

	return cleaned
}
