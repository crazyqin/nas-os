// Package rdgateway 提供文件传输通道功能
package rdgateway

import (
	"fmt"
	"sync"
	"time"
)

// FileTransfer 文件传输管理器.
type FileTransfer struct {
	mu        sync.RWMutex
	transfers map[string]*TransferJob // transferID -> job
}

// TransferJob 文件传输任务.
type TransferJob struct {
	ID          string     `json:"id"`
	SessionID   string     `json:"session_id"`
	Filename    string     `json:"filename"`
	Size        int64      `json:"size"`
	Transferred int64      `json:"transferred"`
	Direction   string     `json:"direction"` // upload, download
	Status      string     `json:"status"`    // pending, transferring, completed, failed
	Checksum    string     `json:"checksum,omitempty"`
	Error       string     `json:"error,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// TransferStatus 传输状态常量.
const (
	TransferStatusPending      = "pending"
	TransferStatusTransferring = "transferring"
	TransferStatusCompleted    = "completed"
	TransferStatusFailed       = "failed"
)

// NewFileTransfer 创建文件传输管理器.
func NewFileTransfer() *FileTransfer {
	return &FileTransfer{
		transfers: make(map[string]*TransferJob),
	}
}

// StartTransfer 开始文件传输.
func (ft *FileTransfer) StartTransfer(sessionID, filename string, size int64, direction string) (*TransferJob, error) {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	if direction != "upload" && direction != "download" {
		return nil, fmt.Errorf("invalid direction: %s", direction)
	}

	job := &TransferJob{
		ID:        generateID(),
		SessionID: sessionID,
		Filename:  filename,
		Size:      size,
		Direction: direction,
		Status:    TransferStatusPending,
		StartedAt: time.Now(),
	}

	ft.transfers[job.ID] = job
	return job, nil
}

// GetTransfer 获取传输任务.
func (ft *FileTransfer) GetTransfer(id string) (*TransferJob, error) {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	job, ok := ft.transfers[id]
	if !ok {
		return nil, fmt.Errorf("transfer %q not found", id)
	}
	return job, nil
}

// UpdateProgress 更新传输进度.
func (ft *FileTransfer) UpdateProgress(id string, transferred int64) error {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	job, ok := ft.transfers[id]
	if !ok {
		return fmt.Errorf("transfer %q not found", id)
	}

	job.Transferred = transferred
	if job.Status == TransferStatusPending {
		job.Status = TransferStatusTransferring
	}
	if transferred >= job.Size {
		job.Status = TransferStatusCompleted
		now := time.Now()
		job.CompletedAt = &now
	}

	return nil
}

// FailTransfer 标记传输失败.
func (ft *FileTransfer) FailTransfer(id string, reason string) error {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	job, ok := ft.transfers[id]
	if !ok {
		return fmt.Errorf("transfer %q not found", id)
	}

	job.Status = TransferStatusFailed
	job.Error = reason
	now := time.Now()
	job.CompletedAt = &now

	return nil
}

// ListTransfers 列出传输任务.
func (ft *FileTransfer) ListTransfers(sessionID string) []*TransferJob {
	ft.mu.RLock()
	defer ft.mu.RUnlock()

	var result []*TransferJob
	for _, job := range ft.transfers {
		if sessionID == "" || job.SessionID == sessionID {
			result = append(result, job)
		}
	}
	return result
}

// DeleteTransfer 删除传输任务.
func (ft *FileTransfer) DeleteTransfer(id string) error {
	ft.mu.Lock()
	defer ft.mu.Unlock()

	if _, ok := ft.transfers[id]; !ok {
		return fmt.Errorf("transfer %q not found", id)
	}
	delete(ft.transfers, id)
	return nil
}

// TransferCount 返回传输任务数.
func (ft *FileTransfer) TransferCount() int {
	ft.mu.RLock()
	defer ft.mu.RUnlock()
	return len(ft.transfers)
}
