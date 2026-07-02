// Package systemclone 提供系统克隆与镜像备份功能
// 支持全盘克隆、增量镜像、系统恢复、PXE 网络部署
// v2: 增加系统盘 RAID1 镜像保护、自动故障转移、健康监控、在线扩容迁移
package systemclone

import (
	"time"
)

// CloneType 克隆类型.
type CloneType string

const (
	CloneTypeFull         CloneType = "full"         // 全盘克隆
	CloneTypeIncremental  CloneType = "incremental"  // 增量克隆
	CloneTypeDifferential CloneType = "differential" // 差异克隆
)

// CloneStatus 克隆状态.
type CloneStatus string

const (
	CloneStatusPending   CloneStatus = "pending"
	CloneStatusRunning   CloneStatus = "running"
	CloneStatusCompleted CloneStatus = "completed"
	CloneStatusFailed    CloneStatus = "failed"
	CloneStatusCancelled CloneStatus = "cancelled"
)

// ImageFormat 镜像格式.
type ImageFormat string

const (
	FormatRaw   ImageFormat = "raw"   // 原始镜像
	FormatQCOW2 ImageFormat = "qcow2" // QEMU 镜像
	FormatVMDK  ImageFormat = "vmdk"  // VMware 镜像
	FormatISO   ImageFormat = "iso"   // ISO 镜像
)

// MirrorStatus 镜像状态.
type MirrorStatus string

const (
	MirrorStatusSyncing    MirrorStatus = "syncing"    // 同步中
	MirrorStatusDegraded   MirrorStatus = "degraded"   // 降级（单盘运行）
	MirrorStatusHealthy    MirrorStatus = "healthy"    // 健康（双盘同步）
	MirrorStatusFailed     MirrorStatus = "failed"     // 故障
	MirrorStatusRebuilding MirrorStatus = "rebuilding" // 重建中
)

// DiskRole 磁盘角色.
type DiskRole string

const (
	DiskRolePrimary   DiskRole = "primary"   // 主盘
	DiskRoleSecondary DiskRole = "secondary" // 镜像盘
	DiskRoleSpare     DiskRole = "spare"     // 热备盘
)

// HealthStatus 健康状态.
type HealthStatus string

const (
	HealthStatusGood     HealthStatus = "good"
	HealthStatusWarning  HealthStatus = "warning"
	HealthStatusCritical HealthStatus = "critical"
	HealthStatusFailed   HealthStatus = "failed"
)

// FailoverTrigger 故障转移触发方式.
type FailoverTrigger string

const (
	FailoverTriggerAuto   FailoverTrigger = "auto"   // 自动
	FailoverTriggerManual FailoverTrigger = "manual" // 手动
)

// MigrationStatus 迁移状态.
type MigrationStatus string

const (
	MigrationStatusPending   MigrationStatus = "pending"
	MigrationStatusRunning   MigrationStatus = "running"
	MigrationStatusCompleted MigrationStatus = "completed"
	MigrationStatusFailed    MigrationStatus = "failed"
	MigrationStatusCancelled MigrationStatus = "cancelled"
)

// ============================================================
// 原有类型
// ============================================================

// DiskCloneTask 克隆任务.
type DiskCloneTask struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`
	Type         CloneType   `json:"type"`
	SourceDisk   string      `json:"sourceDisk"` // /dev/sda
	TargetDisk   string      `json:"targetDisk"` // /dev/sdb 或镜像路径
	Format       ImageFormat `json:"format,omitempty"`
	Status       CloneStatus `json:"status"`
	Progress     int         `json:"progress"` // 0-100
	BytesTotal   int64       `json:"bytesTotal"`
	BytesCopied  int64       `json:"bytesCopied"`
	SpeedMBps    float64     `json:"speedMBps"`     // 速度 MB/s
	ETA          string      `json:"eta,omitempty"` // 预计剩余时间
	ErrorMessage string      `json:"errorMessage,omitempty"`
	CreatedAt    time.Time   `json:"createdAt"`
	StartedAt    *time.Time  `json:"startedAt,omitempty"`
	CompletedAt  *time.Time  `json:"completedAt,omitempty"`
}

// RestoreTask 恢复任务.
type RestoreTask struct {
	ID           string      `json:"id"`
	ImageID      string      `json:"imageId"` // 镜像 ID
	TargetDisk   string      `json:"targetDisk"`
	Status       CloneStatus `json:"status"`
	Progress     int         `json:"progress"`
	ErrorMessage string      `json:"errorMessage,omitempty"`
	CreatedAt    time.Time   `json:"createdAt"`
	CompletedAt  *time.Time  `json:"completedAt,omitempty"`
}

// BackupImage 备份镜像.
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

// PXEDeployConfig PXE 网络部署配置.
type PXEDeployConfig struct {
	ID          string `json:"id"`
	ImageID     string `json:"imageId"`
	NetworkCIDR string `json:"networkCidr"` // 192.168.1.0/24
	TFTPServer  string `json:"tftpServer"`
	BootFile    string `json:"bootFile"`
	AutoInstall bool   `json:"autoInstall"`
	Enabled     bool   `json:"enabled"`
}

// CloneStats 克隆统计.
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

// ============================================================
// 新增类型 - 系统盘 RAID1 镜像保护
// ============================================================

// SystemMirror 系统盘 RAID1 镜像配置.
type SystemMirror struct {
	ID             string       `json:"id"`
	Name           string       `json:"name"`
	PrimaryDisk    string       `json:"primaryDisk"`   // 主盘 /dev/sda
	SecondaryDisk  string       `json:"secondaryDisk"` // 镜像盘 /dev/sdb
	SpareDisks     []string     `json:"spareDisks"`    // 热备盘列表
	Status         MirrorStatus `json:"status"`
	BootDisk       string       `json:"bootDisk"` // 当前启动盘
	LastSyncTime   *time.Time   `json:"lastSyncTime,omitempty"`
	LastCheckTime  *time.Time   `json:"lastCheckTime,omitempty"`
	SyncProgress   int          `json:"syncProgress"` // 0-100
	TotalSizeBytes int64        `json:"totalSizeBytes"`
	UsedSizeBytes  int64        `json:"usedSizeBytes"`
	SyncSpeedMBps  float64      `json:"syncSpeedMBps"`
	CreatedAt      time.Time    `json:"createdAt"`
	UpdatedAt      time.Time    `json:"updatedAt"`
}

// DiskHealthInfo 磁盘健康信息.
type DiskHealthInfo struct {
	Device            string       `json:"device"`            // /dev/sda
	Role              DiskRole     `json:"role"`              // primary/secondary/spare
	HealthStatus      HealthStatus `json:"healthStatus"`      // good/warning/critical/failed
	Temperature       int          `json:"temperature"`       // 温度 °C
	PowerOnHours      int64        `json:"powerOnHours"`      // 通电时间
	ReallocatedSect   int64        `json:"reallocatedSect"`   // 重映射扇区数
	PendingSect       int64        `json:"pendingSect"`       // 待映射扇区数
	UncorrectableSect int64        `json:"uncorrectableSect"` // 不可修复扇区数
	HealthScore       float64      `json:"healthScore"`       // 0-100 健康评分
	LastError         string       `json:"lastError,omitempty"`
	LastCheckTime     time.Time    `json:"lastCheckTime"`
}

// HealthMonitorConfig 健康监控配置.
type HealthMonitorConfig struct {
	Enabled              bool    `json:"enabled"`
	CheckIntervalSec     int     `json:"checkIntervalSec"`     // 检查间隔（秒）
	TemperatureThreshold int     `json:"temperatureThreshold"` // 温度告警阈值
	HealthScoreThreshold float64 `json:"healthScoreThreshold"` // 健康评分告警阈值
	MaxReallocatedSect   int64   `json:"maxReallocatedSect"`   // 最大重映射扇区数
	MaxPendingSect       int64   `json:"maxPendingSect"`       // 最大待映射扇区数
	AutoFailover         bool    `json:"autoFailover"`         // 自动故障转移
	FailoverDelaySec     int     `json:"failoverDelaySec"`     // 故障转移延迟（秒）
	AlertWebhookURL      string  `json:"alertWebhookUrl"`      // 告警 Webhook URL
}

// FailoverEvent 故障转移事件.
type FailoverEvent struct {
	ID           string          `json:"id"`
	MirrorID     string          `json:"mirrorID"`
	TriggerType  FailoverTrigger `json:"triggerType"` // auto/manual
	FailedDisk   string          `json:"failedDisk"`
	FailedRole   DiskRole        `json:"failedRole"`
	Reason       string          `json:"reason"`
	NewBootDisk  string          `json:"newBootDisk"`
	Status       string          `json:"status"` // pending/completed/failed
	ErrorMessage string          `json:"errorMessage,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	CompletedAt  *time.Time      `json:"completedAt,omitempty"`
}

// MigrationTask 在线迁移任务.
type MigrationTask struct {
	ID           string          `json:"id"`
	MirrorID     string          `json:"mirrorID"`
	SourceDisk   string          `json:"sourceDisk"` // 被替换的盘
	TargetDisk   string          `json:"targetDisk"` // 新盘
	Phase        string          `json:"phase"`      // sync/verify/switch
	Status       MigrationStatus `json:"status"`
	Progress     int             `json:"progress"` // 0-100
	BytesTotal   int64           `json:"bytesTotal"`
	BytesCopied  int64           `json:"bytesCopied"`
	SpeedMBps    float64         `json:"speedMBps"`
	ETA          string          `json:"eta,omitempty"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	StartedAt    *time.Time      `json:"startedAt,omitempty"`
	CompletedAt  *time.Time      `json:"completedAt,omitempty"`
}

// ExpandTask 扩容任务.
type ExpandTask struct {
	ID           string          `json:"id"`
	MirrorID     string          `json:"mirrorID"`
	NewDisk      string          `json:"newDisk"` // 新增的盘
	OldDisk      string          `json:"oldDisk"` // 被替换的盘
	Phase        string          `json:"phase"`   // add/sync/verify/replace
	Status       MigrationStatus `json:"status"`
	Progress     int             `json:"progress"`
	ErrorMessage string          `json:"errorMessage,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	CompletedAt  *time.Time      `json:"completedAt,omitempty"`
}

// MirrorStats 镜像统计.
type MirrorStats struct {
	TotalMirrors         int     `json:"totalMirrors"`
	HealthyMirrors       int     `json:"healthyMirrors"`
	DegradedMirrors      int     `json:"degradedMirrors"`
	FailedMirrors        int     `json:"failedMirrors"`
	TotalFailovers       int     `json:"totalFailovers"`
	AutoFailovers        int     `json:"autoFailovers"`
	ManualFailovers      int     `json:"manualFailovers"`
	TotalMigrations      int     `json:"totalMigrations"`
	SuccessfulMigrations int     `json:"successfulMigrations"`
	TotalExpansions      int     `json:"totalExpansions"`
	SuccessfulExpansions int     `json:"successfulExpansions"`
	AvgSyncSpeedMBps     float64 `json:"avgSyncSpeedMBps"`
}
