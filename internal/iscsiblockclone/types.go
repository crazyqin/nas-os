// Package iscsiblockclone iSCSI块克隆加速
// 通过ZFS块克隆技术实现VM/容器的秒级克隆，对标TrueNAS iSCSI XCOPY。
// 支持10倍加速的VM克隆和快照操作。
package iscsiblockclone

import (
	"errors"
	"sync"
	"time"
)

// CloneType 克隆类型
type CloneType string

const (
	CloneFull    CloneType = "full"    // 完整克隆
	CloneThin    CloneType = "thin"    // 精简克隆
	CloneLinked  CloneType = "linked"  // 链接克隆（COW）
)

// CloneStatus 克隆状态
type CloneStatus string

const (
	StatusPending    CloneStatus = "pending"
	StatusInProgress CloneStatus = "in_progress"
	StatusCompleted  CloneStatus = "completed"
	StatusFailed     CloneStatus = "failed"
)

// BlockCloneTask 块克隆任务
type BlockCloneTask struct {
	ID          string      `json:"id"`
	SourceLUN   string      `json:"source_lun"`
	TargetLUN   string      `json:"target_lun"`
	Type        CloneType   `json:"type"`
	Status      CloneStatus `json:"status"`
	Progress    float64     `json:"progress"`
	SizeBytes   int64       `json:"size_bytes"`
	ClonedBytes int64       `json:"cloned_bytes"`
	SpeedMBps   float64     `json:"speed_mbps"`
	StartedAt   time.Time   `json:"started_at"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	ErrorMsg     string     `json:"error_msg,omitempty"`
}

// LUNInfo LUN信息
type LUNInfo struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	SizeBytes  int64     `json:"size_bytes"`
	BlockSize  int       `json:"block_size"`
	Protocol   string    `json:"protocol"`
	TargetIQN  string    `json:"target_iqn"`
	ReadOnly   bool      `json:"read_only"`
	CreatedAt  time.Time `json:"created_at"`
}

// CloneStats 克隆统计
type CloneStats struct {
	TotalClones    int64   `json:"total_clones"`
	SuccessfulClones int64 `json:"successful_clones"`
	FailedClones   int64   `json:"failed_clones"`
	TotalBytesCloned int64 `json:"total_bytes_cloned"`
	AverageSpeedMBps float64 `json:"average_speed_mbps"`
}

// BlockCloneManager 块克隆管理器
type BlockCloneManager struct {
	mu      sync.RWMutex
	config  ManagerConfig
	luns    map[string]*LUNInfo
	tasks   map[string]*BlockCloneTask
	stats   CloneStats
}

// ManagerConfig 管理器配置
type ManagerConfig struct {
	MaxConcurrentClones int    `json:"max_concurrent_clones"`
	DefaultCloneType    CloneType `json:"default_clone_type"`
	BlockSizeKB         int    `json:"block_size_kb"`
	EnableCompression   bool   `json:"enable_compression"`
	EnableDedup         bool   `json:"enable_dedup"`
	TargetIOPS          int    `json:"target_iops"`
}

// DefaultManagerConfig 默认配置
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		MaxConcurrentClones: 4,
		DefaultCloneType:    CloneLinked,
		BlockSizeKB:         128,
		EnableCompression:   true,
		EnableDedup:         true,
		TargetIOPS:          100000,
	}
}

// 预定义错误
var (
	ErrLUNNotFound      = errors.New("LUN not found")
	ErrLUNExists        = errors.New("LUN already exists")
	ErrTaskNotFound     = errors.New("clone task not found")
	ErrMaxConcurrent    = errors.New("max concurrent clones reached")
	ErrSourceReadOnly   = errors.New("source LUN is read-only")
	ErrSizeMismatch     = errors.New("source and target size mismatch")
	ErrInvalidCloneType = errors.New("invalid clone type")
)
