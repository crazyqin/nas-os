// Package migration 提供系统迁移管理功能
// 对标群晖 Migration Assistant
package migration

import (
	"time"
)

// ========== 迁移类型 ==========

// MigrationSourceType 源系统类型.
type MigrationSourceType string

const (
	SourceSynology   MigrationSourceType = "synology"
	SourceQNAP       MigrationSourceType = "qnap"
	SourceTrueNAS    MigrationSourceType = "truenas"
	SourceUnraid     MigrationSourceType = "unraid"
	SourceGenericNAS MigrationSourceType = "generic_nas"
	SourceLinux      MigrationSourceType = "linux"
	SourceWindows    MigrationSourceType = "windows"
)

// MigrationStatus 迁移任务状态.
type MigrationStatus string

const (
	MigrationStatusPending    MigrationStatus = "pending"
	MigrationStatusPlanning   MigrationStatus = "planning"
	MigrationStatusReady      MigrationStatus = "ready"
	MigrationStatusRunning    MigrationStatus = "running"
	MigrationStatusPaused     MigrationStatus = "paused"
	MigrationStatusCompleted  MigrationStatus = "completed"
	MigrationStatusFailed     MigrationStatus = "failed"
	MigrationStatusCancelled  MigrationStatus = "cancelled"
	MigrationStatusRolledBack MigrationStatus = "rolled_back"
)

// DataCategory 数据类别.
type DataCategory string

const (
	CategorySystem     DataCategory = "system"
	CategoryUsers      DataCategory = "users"
	CategoryShares     DataCategory = "shares"
	CategoryApps       DataCategory = "apps"
	CategorySettings   DataCategory = "settings"
	CategoryCerts      DataCategory = "certificates"
	CategoryDocker     DataCategory = "docker"
	CategoryVMs        DataCategory = "virtual_machines"
	CategoryBackup     DataCategory = "backup_tasks"
	CategoryScheduled  DataCategory = "scheduled_tasks"
)

// ========== 核心类型 ==========

// MigrationTask 迁移任务.
type MigrationTask struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	SourceType  MigrationSourceType `json:"sourceType"`
	SourceHost  string          `json:"sourceHost"`
	SourcePort  int             `json:"sourcePort"`
	SourceUser  string          `json:"sourceUser"`
	TargetPath  string          `json:"targetPath"`
	Status      MigrationStatus `json:"status"`
	PlanID      string          `json:"planId,omitempty"`
	Progress    *ProgressInfo   `json:"progress,omitempty"`
	Error       string          `json:"error,omitempty"`
	CreatedAt   time.Time       `json:"createdAt"`
	StartedAt   time.Time       `json:"startedAt,omitempty"`
	FinishedAt  time.Time       `json:"finishedAt,omitempty"`
	Tags        []string        `json:"tags,omitempty"`
}

// MigrationPlan 迁移计划.
type MigrationPlan struct {
	ID              string          `json:"id"`
	TaskID          string          `json:"taskId"`
	SourceType      MigrationSourceType `json:"sourceType"`
	SourceVersion   string          `json:"sourceVersion"`
	SourceHost      string          `json:"sourceHost"`
	TotalDataSize   int64           `json:"totalDataSize"`
	TotalItems      int             `json:"totalItems"`
	Mappings        []DataMapping   `json:"mappings"`
	Warnings        []PlanWarning   `json:"warnings,omitempty"`
	EstimatedTime   time.Duration   `json:"estimatedTime"`
	Compatible      bool            `json:"compatible"`
	CompatibilityNotes []string     `json:"compatibilityNotes,omitempty"`
	CreatedAt       time.Time       `json:"createdAt"`
}

// DataMapping 数据映射关系.
type DataMapping struct {
	ID          string        `json:"id"`
	Category    DataCategory  `json:"category"`
	SourcePath  string        `json:"sourcePath"`
	TargetPath  string        `json:"targetPath"`
	ItemCount   int           `json:"itemCount"`
	TotalSize   int64         `json:"totalSize"`
	Selected    bool          `json:"selected"`
	Convertible bool          `json:"convertible"` // 是否需要格式转换
	ConvertNote string        `json:"convertNote,omitempty"`
	Order       int           `json:"order"` // 执行顺序
}

// ProgressInfo 迁移进度信息.
type ProgressInfo struct {
	OverallPercent  int              `json:"overallPercent"`  // 总体进度 0-100
	CurrentCategory DataCategory     `json:"currentCategory"` // 当前处理的数据类别
	CategoryPercent int              `json:"categoryPercent"` // 当前类别进度 0-100
	TransferredBytes int64           `json:"transferredBytes"`
	TotalBytes      int64            `json:"totalBytes"`
	TransferredItems int             `json:"transferredItems"`
	TotalItems      int              `json:"totalItems"`
	Speed           int64            `json:"speed"`           // bytes/sec
	RemainingSec    int64            `json:"remainingSec"`
	Phase           string           `json:"phase"`           // 当前阶段描述
	CategoryProgress map[string]int  `json:"categoryProgress"` // 各类别进度
}

// MigrationResult 迁移结果.
type MigrationResult struct {
	TaskID          string            `json:"taskId"`
	Status          MigrationStatus   `json:"status"`
	TotalMigrated   int               `json:"totalMigrated"`
	TotalFailed     int               `json:"totalFailed"`
	TotalSkipped    int               `json:"totalSkipped"`
	BytesMigrated   int64             `json:"bytesMigrated"`
	Duration        time.Duration     `json:"duration"`
	CategoryResults []CategoryResult  `json:"categoryResults"`
	Errors          []MigrationErrorDetail `json:"errors,omitempty"`
	Warnings        []string          `json:"warnings,omitempty"`
	RollbackID      string            `json:"rollbackId,omitempty"`
	CompletedAt     time.Time         `json:"completedAt"`
}

// CategoryResult 单类别迁移结果.
type CategoryResult struct {
	Category    DataCategory `json:"category"`
	Status      string       `json:"status"` // success, partial, failed
	Migrated    int          `json:"migrated"`
	Failed      int          `json:"failed"`
	Skipped     int          `json:"skipped"`
	SizeBytes   int64        `json:"sizeBytes"`
	Duration    time.Duration `json:"duration"`
	ErrorDetail string       `json:"errorDetail,omitempty"`
}

// PlanWarning 迁移计划警告.
type PlanWarning struct {
	Level   string `json:"level"` // info, warning, error
	Category string `json:"category"`
	Message string `json:"message"`
}

// MigrationErrorDetail 迁移错误详情.
type MigrationErrorDetail struct {
	Category DataCategory `json:"category"`
	Item     string       `json:"item"`
	Error    string       `json:"error"`
	Code     string       `json:"code,omitempty"`
}

// SourceSystemInfo 源系统信息.
type SourceSystemInfo struct {
	Type          MigrationSourceType `json:"type"`
	Version       string              `json:"version"`
	Hostname      string              `json:"hostname"`
	TotalStorage  int64               `json:"totalStorage"`
	UsedStorage   int64               `json:"usedStorage"`
	TotalUsers    int                 `json:"totalUsers"`
	TotalShares   int                 `json:"totalShares"`
	TotalApps     int                 `json:"totalApps"`
	IPAddresses   []string            `json:"ipAddresses"`
	SystemModel   string              `json:"systemModel,omitempty"`
	SerialNumber  string              `json:"serialNumber,omitempty"`
}

// Checkpoint 断点续传检查点.
type Checkpoint struct {
	TaskID          string       `json:"taskId"`
	CategoryIndex   int          `json:"categoryIndex"`
	ItemIndex       int          `json:"itemIndex"`
	BytesTransferred int64       `json:"bytesTransferred"`
	Timestamp       time.Time    `json:"timestamp"`
}

// ========== 请求/响应类型 ==========

// CreateMigrationRequest 创建迁移任务请求.
type CreateMigrationRequest struct {
	Name        string              `json:"name" validate:"required"`
	Description string              `json:"description,omitempty"`
	SourceType  MigrationSourceType `json:"sourceType" validate:"required"`
	SourceHost  string              `json:"sourceHost" validate:"required"`
	SourcePort  int                 `json:"sourcePort"`
	SourceUser  string              `json:"sourceUser" validate:"required"`
	SourcePass  string              `json:"sourcePass"`
	TargetPath  string              `json:"targetPath" validate:"required"`
	Tags        []string            `json:"tags,omitempty"`
}

// UpdateMappingRequest 更新数据映射请求.
type UpdateMappingRequest struct {
	Selected   bool   `json:"selected"`
	TargetPath string `json:"targetPath,omitempty"`
}

// ResumeRequest 断点续传请求.
type ResumeRequest struct {
	TaskID string `json:"taskId" validate:"required"`
}
