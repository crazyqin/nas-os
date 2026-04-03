// Package cloudsync provides cloud storage synchronization
// This file implements resumable upload with checkpoint support
package cloudsync

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ResumableUpload 断点续传管理器
// 用于大文件上传的进度保存和恢复.
type ResumableUpload struct {
	mu sync.RWMutex

	// 进度文件存储路径
	progressPath string

	// 上传进度记录
	uploadProgress map[string]*UploadProgress

	// 分片大小（默认 4MB）
	chunkSize int64
}

// UploadProgress 上传进度信息.
type UploadProgress struct {
	// 文件标识
	FileID      string    `json:"fileId"`      // 文件唯一标识（任务ID + 文件路径哈希）
	LocalPath   string    `json:"localPath"`   // 本地文件路径
	RemotePath  string    `json:"remotePath"`  // 远程文件路径
	ProviderID  string    `json:"providerId"`  // 提供商ID
	TaskID      string    `json:"taskId"`      // 同步任务ID

	// 文件信息
	FileSize    int64     `json:"fileSize"`    // 文件总大小
	FileHash    string    `json:"fileHash"`    // 文件哈希（用于验证）

	// 上传状态
	Status      UploadStatus `json:"status"`    // 上传状态
	StartTime   time.Time    `json:"startTime"` // 开始时间
	UpdateTime  time.Time    `json:"updateTime"` // 最后更新时间

	// 上传进度
	UploadedSize  int64 `json:"uploadedSize"`  // 已上传大小
	UploadedChunks int  `json:"uploadedChunks"` // 已上传分片数
	TotalChunks    int  `json:"totalChunks"`    // 总分片数

	// 会话信息（用于断点续传）
	SessionID    string `json:"sessionId,omitempty"`    // 上传会话ID
	SessionData  string `json:"sessionData,omitempty"`  // 会话数据（不同提供商格式不同）

	// 错误信息
	LastError   string    `json:"lastError,omitempty"`   // 最后错误
	RetryCount  int       `json:"retryCount"`            // 重试次数
	MaxRetries  int       `json:"maxRetries"`            // 最大重试次数
}

// UploadStatus 上传状态.
type UploadStatus string

const (
	UploadStatusPending    UploadStatus = "pending"    // 待上传
	UploadStatusInProgress UploadStatus = "in_progress" // 上传中
	UploadStatusPaused     UploadStatus = "paused"     // 已暂停
	UploadStatusCompleted  UploadStatus = "completed"  // 已完成
	UploadStatusFailed     UploadStatus = "failed"     // 失败
	UploadStatusCancelled  UploadStatus = "cancelled"  // 已取消
)

// NewResumableUpload 创建断点续传管理器.
func NewResumableUpload(progressPath string) *ResumableUpload {
	return &ResumableUpload{
		progressPath:   progressPath,
		uploadProgress: make(map[string]*UploadProgress),
		chunkSize:      4 * 1024 * 1024, // 4MB
	}
}

// Initialize 初始化断点续传管理器.
func (r *ResumableUpload) Initialize() error {
	// 加载已有进度
	return r.loadProgress()
}

// loadProgress 加载进度文件.
func (r *ResumableUpload) loadProgress() error {
	data, err := os.ReadFile(r.progressPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // 文件不存在是正常的
		}
		return fmt.Errorf("读取进度文件失败: %w", err)
	}

	var progress map[string]*UploadProgress
	if err := json.Unmarshal(data, &progress); err != nil {
		return fmt.Errorf("解析进度文件失败: %w", err)
	}

	r.mu.Lock()
	r.uploadProgress = progress
	r.mu.Unlock()

	return nil
}

// saveProgress 保存进度文件.
func (r *ResumableUpload) saveProgress() error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := json.MarshalIndent(r.uploadProgress, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化进度失败: %w", err)
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(r.progressPath), 0750); err != nil {
		return fmt.Errorf("创建目录失败: %w", err)
	}

	return os.WriteFile(r.progressPath, data, 0640)
}

// CreateProgress 创建新的上传进度.
func (r *ResumableUpload) CreateProgress(
	taskID, providerID, localPath, remotePath string,
	fileSize int64, fileHash string,
) *UploadProgress {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 生成唯一ID
	fileID := generateProgressID(taskID, localPath)

	// 计算分片数
	totalChunks := int((fileSize + r.chunkSize - 1) / r.chunkSize)

	progress := &UploadProgress{
		FileID:      fileID,
		LocalPath:   localPath,
		RemotePath:  remotePath,
		ProviderID:  providerID,
		TaskID:      taskID,
		FileSize:    fileSize,
		FileHash:    fileHash,
		Status:      UploadStatusPending,
		StartTime:   time.Now(),
		UpdateTime:  time.Now(),
		TotalChunks: totalChunks,
		MaxRetries:  3,
	}

	r.uploadProgress[fileID] = progress
	_ = r.saveProgress()

	return progress
}

// GetProgress 获取上传进度.
func (r *ResumableUpload) GetProgress(fileID string) (*UploadProgress, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	progress, ok := r.uploadProgress[fileID]
	if !ok {
		return nil, fmt.Errorf("进度不存在: %s", fileID)
	}

	return progress, nil
}

// GetProgressByFile 通过文件路径获取进度.
func (r *ResumableUpload) GetProgressByFile(taskID, localPath string) (*UploadProgress, error) {
	fileID := generateProgressID(taskID, localPath)
	return r.GetProgress(fileID)
}

// UpdateProgress 更新上传进度.
func (r *ResumableUpload) UpdateProgress(fileID string, uploadedSize int64, uploadedChunks int, sessionID, sessionData string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	progress, ok := r.uploadProgress[fileID]
	if !ok {
		return fmt.Errorf("进度不存在: %s", fileID)
	}

	progress.UploadedSize = uploadedSize
	progress.UploadedChunks = uploadedChunks
	progress.SessionID = sessionID
	progress.SessionData = sessionData
	progress.UpdateTime = time.Now()

	if progress.Status == UploadStatusPending {
		progress.Status = UploadStatusInProgress
	}

	return r.saveProgress()
}

// SetSessionID 设置会话ID.
func (r *ResumableUpload) SetSessionID(fileID, sessionID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	progress, ok := r.uploadProgress[fileID]
	if !ok {
		return fmt.Errorf("进度不存在: %s", fileID)
	}

	progress.SessionID = sessionID
	progress.UpdateTime = time.Now()

	return r.saveProgress()
}

// MarkCompleted 标记上传完成.
func (r *ResumableUpload) MarkCompleted(fileID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	progress, ok := r.uploadProgress[fileID]
	if !ok {
		return fmt.Errorf("进度不存在: %s", fileID)
	}

	progress.Status = UploadStatusCompleted
	progress.UploadedSize = progress.FileSize
	progress.UploadedChunks = progress.TotalChunks
	progress.UpdateTime = time.Now()

	// 完成后删除进度记录
	delete(r.uploadProgress, fileID)

	return r.saveProgress()
}

// MarkFailed 标记上传失败.
func (r *ResumableUpload) MarkFailed(fileID, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	progress, ok := r.uploadProgress[fileID]
	if !ok {
		return fmt.Errorf("进度不存在: %s", fileID)
	}

	progress.Status = UploadStatusFailed
	progress.LastError = errMsg
	progress.RetryCount++
	progress.UpdateTime = time.Now()

	return r.saveProgress()
}

// MarkPaused 标记上传暂停.
func (r *ResumableUpload) MarkPaused(fileID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	progress, ok := r.uploadProgress[fileID]
	if !ok {
		return fmt.Errorf("进度不存在: %s", fileID)
	}

	progress.Status = UploadStatusPaused
	progress.UpdateTime = time.Now()

	return r.saveProgress()
}

// MarkCancelled 标记上传取消.
func (r *ResumableUpload) MarkCancelled(fileID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	progress, ok := r.uploadProgress[fileID]
	if !ok {
		return fmt.Errorf("进度不存在: %s", fileID)
	}

	progress.Status = UploadStatusCancelled
	progress.UpdateTime = time.Now()

	// 取消后删除进度记录
	delete(r.uploadProgress, fileID)

	return r.saveProgress()
}

// CanResume 检查是否可以恢复上传.
func (r *ResumableUpload) CanResume(fileID string) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	progress, ok := r.uploadProgress[fileID]
	if !ok {
		return false
	}

	// 只有暂停或失败的进度可以恢复
	if progress.Status != UploadStatusPaused && progress.Status != UploadStatusFailed {
		return false
	}

	// 检查是否有会话信息
	if progress.SessionID == "" {
		return false
	}

	// 检查重试次数
	if progress.RetryCount >= progress.MaxRetries {
		return false
	}

	// 检查文件是否还存在
	if _, err := os.Stat(progress.LocalPath); os.IsNotExist(err) {
		return false
	}

	return true
}

// GetResumableUploads 获取可恢复的上传列表.
func (r *ResumableUpload) GetResumableUploads() []*UploadProgress {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var resumable []*UploadProgress
	for _, progress := range r.uploadProgress {
		if r.CanResume(progress.FileID) {
			resumable = append(resumable, progress)
		}
	}
	return resumable
}

// GetPendingUploads 获取待处理的上传列表.
func (r *ResumableUpload) GetPendingUploads() []*UploadProgress {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var pending []*UploadProgress
	for _, progress := range r.uploadProgress {
		if progress.Status == UploadStatusPending || progress.Status == UploadStatusPaused {
			pending = append(pending, progress)
		}
	}
	return pending
}

// CleanupCompleted 清理已完成的进度.
func (r *ResumableUpload) CleanupCompleted() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	for fileID, progress := range r.uploadProgress {
		if progress.Status == UploadStatusCompleted || progress.Status == UploadStatusCancelled {
			delete(r.uploadProgress, fileID)
		}
	}

	return r.saveProgress()
}

// CleanupExpired 清理过期进度（超过7天未更新）.
func (r *ResumableUpload) CleanupExpired() error {
	r.mu.Lock()
	defer r.mu.Unlock()

	expiredThreshold := time.Now().Add(-7 * 24 * time.Hour)

	for fileID, progress := range r.uploadProgress {
		if progress.UpdateTime.Before(expiredThreshold) {
			delete(r.uploadProgress, fileID)
		}
	}

	return r.saveProgress()
}

// GetChunkInfo 获取分片信息.
func (r *ResumableUpload) GetChunkInfo(fileID string) (int64, int, int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	progress, ok := r.uploadProgress[fileID]
	if !ok {
		return 0, 0, 0, fmt.Errorf("进度不存在: %s", fileID)
	}

	return r.chunkSize, progress.UploadedChunks, progress.TotalChunks, nil
}

// GetNextChunkOffset 获取下一个分片的偏移量.
func (r *ResumableUpload) GetNextChunkOffset(fileID string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	progress, ok := r.uploadProgress[fileID]
	if !ok {
		return 0, fmt.Errorf("进度不存在: %s", fileID)
	}

	return int64(progress.UploadedChunks) * r.chunkSize, nil
}

// GetProgressPercent 获取进度百分比.
func (r *ResumableUpload) GetProgressPercent(fileID string) (float64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	progress, ok := r.uploadProgress[fileID]
	if !ok {
		return 0, fmt.Errorf("进度不存在: %s", fileID)
	}

	if progress.FileSize == 0 {
		return 0, nil
	}

	return float64(progress.UploadedSize) / float64(progress.FileSize) * 100, nil
}

// Stats 统计信息.
type ResumableUploadStats struct {
	TotalUploads     int   `json:"totalUploads"`
	PendingUploads   int   `json:"pendingUploads"`
	InProgressUploads int  `json:"inProgressUploads"`
	PausedUploads    int   `json:"pausedUploads"`
	CompletedUploads int   `json:"completedUploads"`
	FailedUploads    int   `json:"failedUploads"`
	TotalBytes       int64 `json:"totalBytes"`
	UploadedBytes    int64 `json:"uploadedBytes"`
}

// GetStats 获取统计信息.
func (r *ResumableUpload) GetStats() *ResumableUploadStats {
	r.mu.RLock()
	defer r.mu.RUnlock()

	stats := &ResumableUploadStats{
		TotalUploads: len(r.uploadProgress),
	}

	for _, progress := range r.uploadProgress {
		stats.TotalBytes += progress.FileSize
		stats.UploadedBytes += progress.UploadedSize

		switch progress.Status {
		case UploadStatusPending:
			stats.PendingUploads++
		case UploadStatusInProgress:
			stats.InProgressUploads++
		case UploadStatusPaused:
			stats.PausedUploads++
		case UploadStatusCompleted:
			stats.CompletedUploads++
		case UploadStatusFailed:
			stats.FailedUploads++
		}
	}

	return stats
}

// generateProgressID 生成进度ID.
func generateProgressID(taskID, localPath string) string {
	// 使用任务ID和文件路径生成唯一标识
	return fmt.Sprintf("%s_%s", taskID, filepath.Base(localPath))
}