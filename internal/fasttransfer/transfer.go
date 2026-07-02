package fasttransfer

import (
	"fmt"
	"sync"
	"time"
)

// TransferManager 高速传输管理器
// 对标群晖 Presto File Server / Synology High-Speed Transfer.
type TransferManager struct {
	mu        sync.RWMutex
	config    *Config
	transfers map[string]*Transfer
}

// Config 传输配置.
type Config struct {
	MaxConcurrent int  `json:"max_concurrent"`
	CompressLevel int  `json:"compress_level"` // 0-9, 0=不压缩
	EncryptAES    bool `json:"encrypt_aes"`
	ChunkSizeMB   int  `json:"chunk_size_mb"`
	BandwidthMBps int  `json:"bandwidth_mbps"` // 0=不限速
}

// Transfer 传输任务.
type Transfer struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	SourcePath  string        `json:"source_path"`
	DestPath    string        `json:"dest_path"`
	Status      string        `json:"status"` // pending/running/completed/failed/paused
	TotalBytes  int64         `json:"total_bytes"`
	Transferred int64         `json:"transferred"`
	SpeedMBps   float64       `json:"speed_mbps"`
	Compressed  bool          `json:"compressed"`
	Encrypted   bool          `json:"encrypted"`
	StartedAt   time.Time     `json:"started_at"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Elapsed     time.Duration `json:"elapsed"`
	ErrorMsg    string        `json:"error_msg,omitempty"`
}

// NewTransferManager 创建管理器.
func NewTransferManager(cfg *Config) *TransferManager {
	if cfg == nil {
		cfg = &Config{
			MaxConcurrent: 4,
			CompressLevel: 6,
			EncryptAES:    true,
			ChunkSizeMB:   64,
		}
	}
	return &TransferManager{
		config:    cfg,
		transfers: make(map[string]*Transfer),
	}
}

// CreateTransfer 创建传输任务.
func (m *TransferManager) CreateTransfer(name, src, dst string) (*Transfer, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查并发数
	running := 0
	for _, t := range m.transfers {
		if t.Status == "running" {
			running++
		}
	}
	if running >= m.config.MaxConcurrent {
		return nil, fmt.Errorf("已达最大并发数 %d", m.config.MaxConcurrent)
	}

	t := &Transfer{
		ID:         fmt.Sprintf("xfer-%d", time.Now().UnixNano()),
		Name:       name,
		SourcePath: src,
		DestPath:   dst,
		Status:     "pending",
		Compressed: m.config.CompressLevel > 0,
		Encrypted:  m.config.EncryptAES,
		StartedAt:  time.Now(),
	}

	m.transfers[t.ID] = t
	return t, nil
}

// GetTransfer 获取传输详情.
func (m *TransferManager) GetTransfer(id string) (*Transfer, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	t, ok := m.transfers[id]
	if !ok {
		return nil, fmt.Errorf("传输任务 %s 不存在", id)
	}
	return t, nil
}

// ListTransfers 列出所有传输.
func (m *TransferManager) ListTransfers() []*Transfer {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Transfer, 0, len(m.transfers))
	for _, t := range m.transfers {
		result = append(result, t)
	}
	return result
}

// CancelTransfer 取消传输.
func (m *TransferManager) CancelTransfer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	t, ok := m.transfers[id]
	if !ok {
		return fmt.Errorf("传输任务 %s 不存在", id)
	}
	if t.Status != "running" && t.Status != "pending" {
		return fmt.Errorf("任务状态 %s 无法取消", t.Status)
	}
	t.Status = "cancelled"
	return nil
}

// GetStats 获取传输统计.
func (m *TransferManager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	total := len(m.transfers)
	completed, failed, running := 0, 0, 0
	var totalBytes int64
	for _, t := range m.transfers {
		switch t.Status {
		case "completed":
			completed++
		case "failed":
			failed++
		case "running":
			running++
		}
		totalBytes += t.Transferred
	}
	return map[string]interface{}{
		"total_transfers": total,
		"completed":       completed,
		"failed":          failed,
		"running":         running,
		"total_bytes":     totalBytes,
	}
}
