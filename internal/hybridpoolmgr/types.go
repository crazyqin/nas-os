// Package hybridpoolmgr 提供混合存储池管理功能
// 对标 OpenZFS 2.4 混合存储池，支持 NVMe + SSD + HDD 多层智能分层
package hybridpoolmgr

import (
	"sync"
	"time"
)

// DeviceTier 设备层级.
type DeviceTier string

const (
	// TierNVMe NVMe 层级，最高性能.
	TierNVMe DeviceTier = "nvme"
	// TierSSD SSD 层级，高性能.
	TierSSD DeviceTier = "ssd"
	// TierHDD HDD 层级，大容量.
	TierHDD DeviceTier = "hdd"
)

// PoolStatus 池状态.
type PoolStatus string

const (
	// PoolStatusOnline 在线.
	PoolStatusOnline PoolStatus = "online"
	// PoolStatusDegraded 降级.
	PoolStatusDegraded PoolStatus = "degraded"
	// PoolStatusFaulted 故障.
	PoolStatusFaulted PoolStatus = "faulted"
	// PoolStatusOffline 离线.
	PoolStatusOffline PoolStatus = "offline"
)

// AlertLevel 告警级别.
type AlertLevel string

const (
	// AlertLevelInfo 信息.
	AlertLevelInfo AlertLevel = "info"
	// AlertLevelWarning 警告.
	AlertLevelWarning AlertLevel = "warning"
	// AlertLevelCritical 严重.
	AlertLevelCritical AlertLevel = "critical"
)

// ==================== 存储设备 ====================

// StorageDevice 存储设备信息.
type StorageDevice struct {
	Path       string     `json:"path"`       // 设备路径，如 /dev/nvme0n1
	Name       string     `json:"name"`       // 设备名称
	Tier       DeviceTier `json:"tier"`       // 设备层级
	Model      string     `json:"model"`      // 设备型号
	Serial     string     `json:"serial"`     // 序列号
	TotalBytes uint64     `json:"totalBytes"` // 总容量（字节）
	UsedBytes  uint64     `json:"usedBytes"`  // 已使用（字节）
	FreeBytes  uint64     `json:"freeBytes"`  // 可用空间（字节）
	Healthy    bool       `json:"healthy"`    // 健康状态
	Temperature int       `json:"temperature"` // 温度（℃）
	WearLevel  int        `json:"wearLevel"`  // 磨损等级（0-100，仅SSD/NVMe）
	AddedAt    time.Time  `json:"addedAt"`    // 添加时间
}

// ==================== 混合池 ====================

// HybridPool 混合存储池.
type HybridPool struct {
	mu sync.RWMutex `json:"-"`

	// 基本信息
	Name        string    `json:"name"`        // 池名称
	UUID        string    `json:"uuid"`        // 唯一标识符
	Description string    `json:"description"` // 描述
	CreatedAt   time.Time `json:"createdAt"`   // 创建时间

	// 设备列表（按层级）
	NVMEDevices []*StorageDevice `json:"nvmeDevices"` // NVMe 设备列表
	SSDDevices  []*StorageDevice `json:"ssdDevices"`  // SSD 设备列表
	HDDDevices  []*StorageDevice `json:"hddDevices"`  // HDD 设备列表

	// 池容量
	TotalBytes uint64 `json:"totalBytes"` // 总容量
	UsedBytes  uint64 `json:"usedBytes"`  // 已使用
	FreeBytes  uint64 `json:"freeBytes"`  // 可用

	// 分层配置
	TieringConfig TieringConfig `json:"tieringConfig"`

	// 重平衡策略
	RebalancePolicy RebalancePolicy `json:"rebalancePolicy"`

	// 状态
	Status   PoolStatus `json:"status"`   // 池状态
	Healthy  bool       `json:"healthy"`  // 是否健康
	MountPoint string   `json:"mountPoint"` // 挂载点

	// 统计
	IOStats *PoolIOStats `json:"ioStats"` // IO 统计
}

// TieringConfig 数据分层配置.
type TieringConfig struct {
	Enabled         bool    `json:"enabled"`         // 是否启用自动分层
	HotThreshold    float64 `json:"hotThreshold"`    // 热数据 IO 阈值（IOPS）
	WarmThreshold   float64 `json:"warmThreshold"`   // 温数据 IO 阈值
	ColdAgeDays     int     `json:"coldAgeDays"`     // 冷数据天数阈值
	PromotePolicy   string  `json:"promotePolicy"`   // 提升策略: "aggressive", "moderate", "conservative"
	DemotePolicy    string  `json:"demotePolicy"`    // 降级策略: "aggressive", "moderate", "conservative"
	MaxPromoteMBps  int     `json:"maxPromoteMBps"`  // 最大提升速率（MB/s）
	MaxDemoteMBps   int     `json:"maxDemoteMBps"`   // 最大降级速率（MB/s）
	TieringWindow   string  `json:"tieringWindow"`   // 分层执行窗口，如 "02:00-06:00"
	ScanIntervalMin int     `json:"scanIntervalMin"` // 扫描间隔（分钟）
}

// DefaultTieringConfig 默认分层配置.
var DefaultTieringConfig = TieringConfig{
	Enabled:         true,
	HotThreshold:    1000,  // 1000 IOPS
	WarmThreshold:   100,   // 100 IOPS
	ColdAgeDays:     30,
	PromotePolicy:   "moderate",
	DemotePolicy:    "conservative",
	MaxPromoteMBps:  500,
	MaxDemoteMBps:   200,
	TieringWindow:   "02:00-06:00",
	ScanIntervalMin: 60,
}

// RebalancePolicy 重平衡策略.
type RebalancePolicy struct {
	Enabled          bool    `json:"enabled"`          // 是否启用自动重平衡
	ThresholdPercent float64 `json:"thresholdPercent"` // 不均衡阈值（百分比）
	MaxMigrateMBps   int     `json:"maxMigrateMBps"`   // 最大迁移速率（MB/s）
	MinFreePercent   float64 `json:"minFreePercent"`   // 最小空闲百分比
	ScheduleCron     string  `json:"scheduleCron"`     // 定时调度
	Running          bool    `json:"running"`          // 是否正在运行
	Progress         float64 `json:"progress"`         // 进度（0-100）
}

// DefaultRebalancePolicy 默认重平衡策略.
var DefaultRebalancePolicy = RebalancePolicy{
	Enabled:          true,
	ThresholdPercent: 15.0,
	MaxMigrateMBps:   300,
	MinFreePercent:   10.0,
	ScheduleCron:     "0 3 * * 0", // 每周日凌晨3点
}

// ==================== IO 统计与热度分析 ====================

// PoolIOStats 池级别 IO 统计.
type PoolIOStats struct {
	mu sync.RWMutex `json:"-"`

	TotalReadOps    uint64  `json:"totalReadOps"`    // 总读操作数
	TotalWriteOps   uint64  `json:"totalWriteOps"`   // 总写操作数
	TotalReadBytes  uint64  `json:"totalReadBytes"`  // 总读字节数
	TotalWriteBytes uint64  `json:"totalWriteBytes"` // 总写字节数
	ReadIOPS        float64 `json:"readIops"`        // 当前读 IOPS
	WriteIOPS       float64 `json:"writeIops"`       // 当前写 IOPS
	ReadBandwidth   float64 `json:"readBandwidth"`   // 读带宽（MB/s）
	WriteBandwidth  float64 `json:"writeBandwidth"`  // 写带宽（MB/s）
	AvgReadLatency  float64 `json:"avgReadLatency"`  // 平均读延迟（微秒）
	AvgWriteLatency float64 `json:"avgWriteLatency"` // 平均写延迟（微秒）
	UpdatedAt       time.Time `json:"updatedAt"`     // 更新时间

	// 按层级统计
	NVMeStats *TierIOStats `json:"nvmeStats"` // NVMe 层统计
	SSDStats  *TierIOStats `json:"ssdStats"`  // SSD 层统计
	HDDStats  *TierIOStats `json:"hddStats"`  // HDD 层统计
}

// TierIOStats 层级 IO 统计.
type TierIOStats struct {
	Tier         DeviceTier `json:"tier"`         // 层级
	ReadOps      uint64     `json:"readOps"`      // 读操作数
	WriteOps     uint64     `json:"writeOps"`     // 写操作数
	ReadBytes    uint64     `json:"readBytes"`    // 读字节数
	WriteBytes   uint64     `json:"writeBytes"`   // 写字节数
	ReadIOPS     float64    `json:"readIops"`     // 读 IOPS
	WriteIOPS    float64    `json:"writeIops"`    // 写 IOPS
	AvgLatency   float64    `json:"avgLatency"`   // 平均延迟（微秒）
	UpdatedAt    time.Time  `json:"updatedAt"`    // 更新时间
}

// BlockHeat 块级热度信息.
type BlockHeat struct {
	BlockID      string    `json:"blockId"`      // 块标识
	Path         string    `json:"path"`         // 文件路径
	Tier         DeviceTier `json:"tier"`         // 当前所在层级
	Size         uint64    `json:"size"`         // 块大小（字节）
	ReadCount    int64     `json:"readCount"`    // 读次数
	WriteCount   int64     `json:"writeCount"`   // 写次数
	LastAccess   time.Time `json:"lastAccess"`   // 最后访问时间
	HeatScore    float64   `json:"heatScore"`    // 热度评分（0-100）
	AccessWindow int       `json:"accessWindow"` // 访问窗口（小时）
}

// HeatAnalysisResult 热度分析结果.
type HeatAnalysisResult struct {
	PoolName     string       `json:"poolName"`     // 池名称
	TotalBlocks  int          `json:"totalBlocks"`  // 总块数
	HotBlocks    int          `json:"hotBlocks"`    // 热块数
	WarmBlocks   int          `json:"warmBlocks"`   // 温块数
	ColdBlocks   int          `json:"coldBlocks"`   // 冷块数
	AnalysisTime time.Time    `json:"analysisTime"` // 分析时间
	TopHotBlocks []*BlockHeat `json:"topHotBlocks"` // 热度最高的块
	TopColdBlocks []*BlockHeat `json:"topColdBlocks"` // 冷度最高的块
}

// ==================== 池健康 ====================

// PoolHealth 池健康状态.
type PoolHealth struct {
	PoolName        string         `json:"poolName"`        // 池名称
	Status          PoolStatus     `json:"status"`          // 池状态
	Healthy         bool           `json:"healthy"`         // 是否健康
	DeviceHealth    []*DeviceHealth `json:"deviceHealth"`   // 设备健康状态
	Alerts          []*PoolAlert   `json:"alerts"`          // 告警列表
	TierBalance     *TierBalance   `json:"tierBalance"`     // 层级均衡状态
	LastCheckTime   time.Time      `json:"lastCheckTime"`   // 最后检查时间
	UptimeSeconds   int64          `json:"uptimeSeconds"`   // 运行时间（秒）
}

// DeviceHealth 设备健康状态.
type DeviceHealth struct {
	Device      string     `json:"device"`      // 设备路径
	Tier        DeviceTier `json:"tier"`        // 所属层级
	Healthy     bool       `json:"healthy"`     // 是否健康
	Temperature int        `json:"temperature"` // 温度
	WearLevel   int        `json:"wearLevel"`   // 磨损等级
	ErrorCode   int        `json:"errorCode"`   // 错误码
	Message     string     `json:"message"`     // 状态消息
}

// PoolAlert 池告警.
type PoolAlert struct {
	ID        string     `json:"id"`        // 告警 ID
	PoolName  string     `json:"poolName"`  // 所属池
	Level     AlertLevel `json:"level"`     // 告警级别
	Device    string     `json:"device"`    // 关联设备
	Message   string     `json:"message"`   // 告警消息
	CreatedAt time.Time  `json:"createdAt"` // 创建时间
	Resolved  bool       `json:"resolved"`  // 是否已解决
}

// TierBalance 层级均衡状态.
type TierBalance struct {
	NVMeUsedPercent float64 `json:"nvmeUsedPercent"` // NVMe 使用率
	SSDUsedPercent  float64 `json:"ssdUsedPercent"`  // SSD 使用率
	HDDUsedPercent  float64 `json:"hddUsedPercent"`  // HDD 使用率
	Balanced        bool    `json:"balanced"`        // 是否均衡
	Recommendation  string  `json:"recommendation"`  // 建议
}

// ==================== 创建/更新请求 ====================

// CreatePoolRequest 创建混合池请求.
type CreatePoolRequest struct {
	Name        string            `json:"name" binding:"required"`  // 池名称
	Description string            `json:"description"`              // 描述
	NVMEDevices []string          `json:"nvmeDevices"`              // NVMe 设备列表
	SSDDevices  []string          `json:"ssdDevices"`               // SSD 设备列表
	HDDDevices  []string          `json:"hddDevices" binding:"required,min=1"` // HDD 设备列表
	Tiering     *TieringConfig    `json:"tiering"`                  // 分层配置
	Rebalance   *RebalancePolicy  `json:"rebalance"`               // 重平衡策略
}

// AddDeviceRequest 添加设备请求.
type AddDeviceRequest struct {
	DevicePath string     `json:"devicePath" binding:"required"` // 设备路径
	Tier       DeviceTier `json:"tier" binding:"required"`       // 设备层级
}

// UpdateTieringRequest 更新分层配置请求.
type UpdateTieringRequest struct {
	TieringConfig `json:",inline"` // 内联分层配置
}

// UpdateRebalanceRequest 更新重平衡策略请求.
type UpdateRebalanceRequest struct {
	RebalancePolicy `json:",inline"` // 内联重平衡策略
}
