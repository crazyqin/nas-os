// Package gpuds 提供 GPU Direct Storage 功能
// 绕过 CPU 直接在 GPU 和 NVMe 之间传输数据
package gpuds

import (
	"time"
)

// GPUDevice GPU 设备信息
type GPUDevice struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	UUID         string            `json:"uuid"`
	PCIAddress   string            `json:"pci_address"`
	MemoryTotal  int64             `json:"memory_total"`  // bytes
	MemoryUsed   int64             `json:"memory_used"`   // bytes
	MemoryFree   int64             `json:"memory_free"`   // bytes
	ComputeCap   string            `json:"compute_cap"`   // e.g. "8.6"
	Driver       string            `json:"driver"`
	State        GPUDeviceState    `json:"state"`
	Temperature  int               `json:"temperature"`   // Celsius
	PowerUsage   int               `json:"power_usage"`   // Watts
	GPDSupported bool              `json:"gds_supported"` // GPU Direct Storage support
	NVMeAffinity []string          `json:"nvme_affinity"` // Affinity NVMe devices
	Metadata     map[string]string `json:"metadata,omitempty"`
	LastSeen     time.Time         `json:"last_seen"`
	CreatedAt    time.Time         `json:"created_at"`
}

// GPUDeviceState GPU 设备状态
type GPUDeviceState string

const (
	GPUStateActive    GPUDeviceState = "active"
	GPUStateIdle      GPUDeviceState = "idle"
	GPUStateError     GPUDeviceState = "error"
	GPUStateDisabled  GPUDeviceState = "disabled"
)

// DirectBuffer 直接传输缓冲区
type DirectBuffer struct {
	ID          string       `json:"id"`
	GPUDeviceID string       `json:"gpu_device_id"`
	Size        int64        `json:"size"`        // bytes
	Offset      int64        `json:"offset"`
	State       BufferState  `json:"state"`
	CreatedAt   time.Time    `json:"created_at"`
	LastUsed    time.Time    `json:"last_used"`
}

// BufferState 缓冲区状态
type BufferState string

const (
	BufferStateAllocated BufferState = "allocated"
	BufferStateInUse     BufferState = "in_use"
	BufferStateFree      BufferState = "free"
	BufferStateError     BufferState = "error"
)

// TransferJob 传输任务
type TransferJob struct {
	ID           string          `json:"id"`
	Source       TransferEndpoint `json:"source"`
	Destination  TransferEndpoint `json:"destination"`
	Size         int64           `json:"size"`          // bytes
	Transferred  int64           `json:"transferred"`   // bytes
	State        TransferState   `json:"state"`
	Bandwidth    float64         `json:"bandwidth"`     // MB/s
	StartTime    time.Time       `json:"start_time"`
	EndTime      *time.Time      `json:"end_time,omitempty"`
	Duration     int64           `json:"duration_ms"`
	ErrorMessage string          `json:"error_message,omitempty"`
}

// TransferEndpoint 传输端点
type TransferEndpoint struct {
	Type       string `json:"type"` // "gpu", "nvme", "host"
	DeviceID   string `json:"device_id"`
	BufferID   string `json:"buffer_id,omitempty"`
	Path       string `json:"path,omitempty"`     // NVMe path
	Offset     int64  `json:"offset,omitempty"`
}

// TransferState 传输状态
type TransferState string

const (
	TransferStatePending   TransferState = "pending"
	TransferStateRunning   TransferState = "running"
	TransferStateCompleted TransferState = "completed"
	TransferStateFailed    TransferState = "failed"
	TransferStateCancelled TransferState = "cancelled"
)

// GPUDSConfig GPU Direct Storage 配置
type GPUDSConfig struct {
	Enabled         bool    `json:"enabled"`
	MaxBuffers      int     `json:"max_buffers"`
	DefaultBufSize  int64   `json:"default_buf_size"`  // bytes
	MaxTransferSize int64   `json:"max_transfer_size"` // bytes
	IOQueueDepth    int     `json:"io_queue_depth"`
	NumaAware       bool    `json:"numa_aware"`
	CompressEnabled bool    `json:"compress_enabled"`
	EncryptEnabled  bool    `json:"encrypt_enabled"`
	BandwidthLimit  float64 `json:"bandwidth_limit"` // MB/s, 0 = unlimited
}

// TransferStats 传输统计
type TransferStats struct {
	TotalTransfers    int64   `json:"total_transfers"`
	SuccessfulTransfers int64 `json:"successful_transfers"`
	FailedTransfers   int64   `json:"failed_transfers"`
	TotalBytes        int64   `json:"total_bytes"`
	AvgBandwidth      float64 `json:"avg_bandwidth"`    // MB/s
	MaxBandwidth      float64 `json:"max_bandwidth"`    // MB/s
	ActiveJobs        int     `json:"active_jobs"`
	AllocatedBuffers  int     `json:"allocated_buffers"`
}

// BandwidthStats 带宽统计
type BandwidthStats struct {
	DeviceID        string    `json:"device_id"`
	CurrentBandwidth float64  `json:"current_bandwidth"` // MB/s
	AvgBandwidth    float64   `json:"avg_bandwidth"`     // MB/s
	MaxBandwidth    float64   `json:"max_bandwidth"`     // MB/s
	MinBandwidth    float64   `json:"min_bandwidth"`     // MB/s
	SampleCount     int       `json:"sample_count"`
	LastMeasured    time.Time `json:"last_measured"`
}
