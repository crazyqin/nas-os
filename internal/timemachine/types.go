// Package timemachine 提供 macOS Time Machine 备份服务器功能
// 共享管理、配额控制、设备注册、备份监控、历史管理、清理策略
package timemachine

import (
	"time"
)

// ========== Time Machine 共享 ==========

// TimeMachineShare Time Machine 共享配置
type TimeMachineShare struct {
	ID        string    `json:"id"`        // 唯一标识
	Name      string    `json:"name"`      // 共享名称
	Path      string    `json:"path"`      // 共享路径
	Protocol  Protocol  `json:"protocol"`  // 协议 (AFP/SMB)
	Enabled   bool      `json:"enabled"`   // 是否启用
	DeviceNum int       `json:"deviceNum"` // 关联设备数
	CreatedAt time.Time `json:"createdAt"` // 创建时间
	UpdatedAt time.Time `json:"updatedAt"` // 更新时间
}

// Protocol 共享协议
type Protocol string

const (
	ProtocolAFP Protocol = "afp" // Apple Filing Protocol
	ProtocolSMB Protocol = "smb" // Server Message Block
)

// ========== 备份配额 ==========

// BackupQuota 设备备份配额
type BackupQuota struct {
	DeviceID   string `json:"deviceId"`   // 设备 ID
	QuotaBytes int64  `json:"quotaBytes"` // 配额大小 (字节)
	UsedBytes  int64  `json:"usedBytes"`  // 已用大小 (字节)
	FreeBytes  int64  `json:"freeBytes"`  // 剩余大小 (字节)
}

// ========== 备份设备 ==========

// BackupDevice 备份设备
type BackupDevice struct {
	ID         string    `json:"id"`         // 唯一标识
	Hostname   string    `json:"hostname"`   // 主机名
	MACAddress string    `json:"macAddress"` // MAC 地址
	IPAddress  string    `json:"ipAddress"`  // IP 地址
	OSVersion  string    `json:"osVersion"`  // macOS 版本
	Registered time.Time `json:"registered"` // 注册时间
	Online     bool      `json:"online"`     // 是否在线
	Approved   bool      `json:"approved"`   // 是否已批准
}

// ========== 备份任务 ==========

// BackupJob 备份任务
type BackupJob struct {
	ID         string      `json:"id"`         // 唯一标识
	DeviceID   string      `json:"deviceId"`   // 设备 ID
	Status     BackupStatus `json:"status"`     // 任务状态
	StartTime  time.Time   `json:"startTime"`  // 开始时间
	EndTime    *time.Time  `json:"endTime"`    // 结束时间
	Size       int64       `json:"size"`       // 备份大小 (字节)
	Duration   int64       `json:"duration"`   // 持续时间 (秒)
}

// BackupStatus 备份状态
type BackupStatus string

const (
	BackupStatusPending   BackupStatus = "pending"   // 等待中
	BackupStatusRunning   BackupStatus = "running"   // 进行中
	BackupStatusCompleted BackupStatus = "completed" // 已完成
	BackupStatusFailed    BackupStatus = "failed"    // 失败
)

// ========== 备份快照 ==========

// BackupSnapshot 备份快照
type BackupSnapshot struct {
	ID        string    `json:"id"`        // 唯一标识
	DeviceID  string    `json:"deviceId"`  // 设备 ID
	JobID     string    `json:"jobId"`     // 任务 ID
	Timestamp time.Time `json:"timestamp"` // 快照时间
	Size      int64     `json:"size"`      // 快照大小 (字节)
	Consistent bool     `json:"consistent"` // 一致性状态
}

// ========== 保留策略 ==========

// RetentionPolicy 备份保留策略
type RetentionPolicy struct {
	RetentionDays int  `json:"retentionDays"` // 保留天数
	MinKeep       int  `json:"minKeep"`       // 最小保留数
	MaxBackups    int  `json:"maxBackups"`    // 最大备份数
	AutoCleanup   bool `json:"autoCleanup"`   // 自动清理
}

// ========== 流量限制 ==========

// TrafficLimit 流量限制配置
type TrafficLimit struct {
	BandwidthKBps int    `json:"bandwidthKBps"` // 带宽限制 (KB/s)
	TimeWindow    string `json:"timeWindow"`    // 时间窗口 (如 "08:00-22:00")
	Enabled       bool   `json:"enabled"`       // 是否启用
}

// ========== 广播配置 ==========

// BroadcastConfig mDNS 广播配置
type BroadcastConfig struct {
	ServiceName string            `json:"serviceName"` // 服务名称
	Port        int               `json:"port"`        // 端口
	Enabled     bool              `json:"enabled"`     // 是否启用
	TXTRecords  map[string]string `json:"txtRecords"`  // TXT 记录
}

// ========== 统计数据 ==========

// TimeMachineStats Time Machine 统计
type TimeMachineStats struct {
	TotalDevices   int   `json:"totalDevices"`   // 总设备数
	ActiveDevices  int   `json:"activeDevices"`  // 活跃设备数
	TotalBackupSize int64 `json:"totalBackupSize"` // 总备份大小 (字节)
	TodayBackups   int   `json:"todayBackups"`   // 今日备份数
}

// ========== 服务状态 ==========

// ServiceStatus 服务状态
type ServiceStatus struct {
	Running   bool      `json:"running"`   // 是否运行
	Shares    int       `json:"shares"`    // 共享数
	Devices   int       `json:"devices"`   // 设备数
	StartTime time.Time `json:"startTime"` // 启动时间
}
