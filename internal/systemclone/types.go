// Package systemclone 提供系统克隆与镜像备份功能
// 支持全盘克隆、增量镜像、系统恢复、PXE 网络部署
package systemclone

import (
	"time"
)

// CloneType 克隆类型
type CloneType string

const (
	CloneTypeFull        CloneType = "full"        // 全盘克隆
	CloneTypeIncremental CloneType = "incremental"  // 增量克隆
	CloneTypeDifferential CloneType = "differential" // 差异克隆
)

// CloneStatus 克隆状态
type CloneStatus string

const (
	CloneStatusPending    CloneStatus = "pending"
	CloneStatusRunning    CloneStatus = "running"
	CloneStatusCompleted  CloneStatus = "completed"
	CloneStatusFailed     CloneStatus = "failed"
	CloneStatusCancelled  CloneStatus = "cancelled"
)

// ImageFormat 镜像格式
type ImageFormat string

const (
	FormatRaw  ImageFormat = "raw"  // 原始镜像
	FormatQCOW2 ImageFormat = "qcow2" // QEMU 镜像
	FormatVMDK ImageFormat = "vmdk" // VMware 镜像
	FormatISO  ImageFormat = "iso"  // ISO 镜像
)

// DiskCloneTask 克隆任务
type DiskCloneTask struct {
	ID            string      `json:"id"`
	Name          string      `json:"name"`
	Type          CloneType   `json:"type"`
	SourceDisk    string      `json:"sourceDisk"`    // /dev/sda
	TargetDisk    string      `json:"targetDisk"`    // /dev/sdb 或镜像路径
	Format        ImageFormat `json:"format,omitempty"`
	Status        CloneStatus `json:"status"`
	Progress      int         `json:"progress"`       // 0-100
	BytesTotal    int64       `json:"bytesTotal"`
	BytesCopied   int64       `json:"bytesCopied"`
	SpeedMBps     float64     `json:"speedMBps"`     // 速度 MB/s
	ETA           string      `json:"eta,omitempty"`  // 预计剩余时间
	ErrorMessage  string      `json:"errorMessage,omitempty"`
	CreatedAt     time.Time   `json:"createdAt"`
	StartedAt     *time.Time  `json:"startedAt,omitempty"`
	CompletedAt   *time.Time  `json:"completedAt,omitempty"`
}

// RestoreTask 恢复任务
type RestoreTask struct {
	ID           string      `json:"id"`
	ImageID      string      `json:"imageId"`      // 镜像 ID
	TargetDisk   string      `json:"targetDisk"`
	Status       CloneStatus `json:"status"`
	Progress     int         `json:"progress"`
	ErrorMessage string      `json:"errorMessage,omitempty"`
	CreatedAt    time.Time   `json:"createdAt"`
	CompletedAt  *time.Time  `json:"completedAt,omitempty"`
}

// BackupImage 备份镜像
type BackupImage struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	SourceDisk  string      `json:"sourceDisk"`
	Format      ImageFormat `json:"format"`
	SizeBytes   int64       `json:"sizeBytes"`
	Compressed  bool        `json:"compressed"`
	Encrypted   bool        `json:"encrypted"`
	Checksum    string      `json:"checksum"`
	Tags        []string    `json:"tags,omitempty"`
	CreatedAt   time.Time   `json:"createdAt"`
	ExpiresAt   *time.Time  `json:"expiresAt,omitempty"`
}

// PXEDeployConfig PXE 网络部署配置
type PXEDeployConfig struct {
	ID          string `json:"id"`
	ImageID     string `json:"imageId"`
	NetworkCIDR string `json:"networkCidr"` // 192.168.1.0/24
	TFTPServer  string `json:"tftpServer"`
	BootFile    string `json:"bootFile"`
	AutoInstall bool   `json:"autoInstall"`
	Enabled     bool   `json:"enabled"`
}

// CloneStats 克隆统计
type CloneStats struct {
	TotalClones      int     `json:"totalClones"`
	SuccessfulClones int     `json:"successfulClones"`
	FailedClones     int     `json:"failedClones"`
	TotalImages      int     `json:"totalImages"`
	TotalImageSize   int64   `json:"totalImageSize"`
	TotalRestores    int     `json:"totalRestores"`
	AvgSpeedMBps     float64 `json:"avgSpeedMBps"`
	AvgCloneTime     string  `json:"avgCloneTime"`
}
