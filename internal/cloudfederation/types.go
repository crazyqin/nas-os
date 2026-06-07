// Package cloudfederation 多云联邦管理 - 统一跨云存储管理
package cloudfederation

import (
	"errors"
	"time"
)

// CloudProvider 云提供商类型
type CloudProvider string

const (
	ProviderAWS     CloudProvider = "aws"
	ProviderAzure   CloudProvider = "azure"
	ProviderGCS     CloudProvider = "gcs"
	ProviderAliyun  CloudProvider = "aliyun"
	ProviderTencent CloudProvider = "tencent"
	ProviderHuawei  CloudProvider = "huawei"
	ProviderMinIO   CloudProvider = "minio"
)

// ProviderStatus 云提供商连接状态
type ProviderStatus string

const (
	ProviderStatusOnline  ProviderStatus = "online"
	ProviderStatusOffline ProviderStatus = "offline"
	ProviderStatusError   ProviderStatus = "error"
)

// PlacementStrategy 数据放置策略
type PlacementStrategy string

const (
	StrategyCostOptimized    PlacementStrategy = "cost_optimized"
	StrategyLatencyOptimized PlacementStrategy = "latency_optimized"
	StrategyCompliance       PlacementStrategy = "compliance"
	StrategyGeoLocation      PlacementStrategy = "geo_location"
	StrategyBalanced         PlacementStrategy = "balanced"
)

// SyncStatus 同步状态
type SyncStatus string

const (
	SyncStatusPending    SyncStatus = "pending"
	SyncStatusInProgress SyncStatus = "in_progress"
	SyncStatusCompleted  SyncStatus = "completed"
	SyncStatusFailed     SyncStatus = "failed"
)

// MigrationStatus 迁移状态
type MigrationStatus string

const (
	MigrationStatusPending    MigrationStatus = "pending"
	MigrationStatusInProgress MigrationStatus = "in_progress"
	MigrationStatusCompleted  MigrationStatus = "completed"
	MigrationStatusFailed     MigrationStatus = "failed"
	MigrationStatusCancelled  MigrationStatus = "cancelled"
)

// CloudProviderConfig 云提供商配置
type CloudProviderConfig struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Provider  CloudProvider     `json:"provider"`
	Region    string            `json:"region"`
	Endpoint  string            `json:"endpoint,omitempty"`
	AccessKey string            `json:"access_key"`
	SecretKey string            `json:"secret_key"`
	Bucket    string            `json:"bucket,omitempty"`
	Status    ProviderStatus    `json:"status"`
	IsDefault bool              `json:"is_default"`
	Labels    map[string]string `json:"labels,omitempty"`
	LastCheck time.Time         `json:"last_check"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// Namespace 统一存储命名空间
type Namespace struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Providers   []string          `json:"providers"`
	Strategy    PlacementStrategy `json:"strategy"`
	Compliance  []string          `json:"compliance,omitempty"`
	Tags        map[string]string `json:"tags,omitempty"`
	ObjectCount int64             `json:"object_count"`
	TotalSize   int64             `json:"total_size"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// StorageObject 统一存储对象
type StorageObject struct {
	ID        string            `json:"id"`
	Namespace string            `json:"namespace"`
	Key       string            `json:"key"`
	Size      int64             `json:"size"`
	ETag      string            `json:"etag,omitempty"`
	Provider  string            `json:"provider"`
	Bucket    string            `json:"bucket"`
	Location  string            `json:"location,omitempty"`
	Tags      map[string]string `json:"tags,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
}

// SyncTask 同步任务
type SyncTask struct {
	ID             string     `json:"id"`
	Namespace      string     `json:"namespace"`
	SourceProvider string     `json:"source_provider"`
	TargetProvider string     `json:"target_provider"`
	SourcePrefix   string     `json:"source_prefix,omitempty"`
	TargetPrefix   string     `json:"target_prefix,omitempty"`
	Status         SyncStatus `json:"status"`
	TotalObjects   int64      `json:"total_objects"`
	SyncedObjects  int64      `json:"synced_objects"`
	FailedObjects  int64      `json:"failed_objects"`
	TotalBytes     int64      `json:"total_bytes"`
	SyncedBytes    int64      `json:"synced_bytes"`
	ErrorMessage   string     `json:"error_message,omitempty"`
	StartedAt      *time.Time `json:"started_at,omitempty"`
	CompletedAt    *time.Time `json:"completed_at,omitempty"`
	CreatedAt      time.Time  `json:"created_at"`
}

// MigrationTask 迁移任务
type MigrationTask struct {
	ID              string          `json:"id"`
	Namespace       string          `json:"namespace"`
	SourceProvider  string          `json:"source_provider"`
	TargetProvider  string          `json:"target_provider"`
	SourcePrefix    string          `json:"source_prefix,omitempty"`
	TargetPrefix    string          `json:"target_prefix,omitempty"`
	DeleteSource    bool            `json:"delete_source"`
	Status          MigrationStatus `json:"status"`
	TotalObjects    int64           `json:"total_objects"`
	MigratedObjects int64           `json:"migrated_objects"`
	FailedObjects   int64           `json:"failed_objects"`
	TotalBytes      int64           `json:"total_bytes"`
	MigratedBytes   int64           `json:"migrated_bytes"`
	ErrorMessage    string          `json:"error_message,omitempty"`
	StartedAt       *time.Time      `json:"started_at,omitempty"`
	CompletedAt     *time.Time      `json:"completed_at,omitempty"`
	CreatedAt       time.Time       `json:"created_at"`
}

// CostReport 成本报告
type CostReport struct {
	Provider     string    `json:"provider"`
	StorageCost  float64   `json:"storage_cost"`
	TransferCost float64   `json:"transfer_cost"`
	RequestCost  float64   `json:"request_cost"`
	TotalCost    float64   `json:"total_cost"`
	StorageGB    float64   `json:"storage_gb"`
	TransferGB   float64   `json:"transfer_gb"`
	RequestCount int64     `json:"request_count"`
	Period       string    `json:"period"`
	GeneratedAt  time.Time `json:"generated_at"`
}

// CostAnalysis 多云成本分析
type CostAnalysis struct {
	Period        string        `json:"period"`
	Reports       []*CostReport `json:"reports"`
	TotalCost     float64       `json:"total_cost"`
	Optimizations []string      `json:"optimizations,omitempty"`
	GeneratedAt   time.Time     `json:"generated_at"`
}

// FederationConfig 联邦配置
type FederationConfig struct {
	DefaultStrategy    PlacementStrategy `json:"default_strategy"`
	AutoSync           bool              `json:"auto_sync"`
	SyncInterval       int               `json:"sync_interval"`
	MaxConcurrentSyncs int               `json:"max_concurrent_syncs"`
	MaxConcurrentMigs  int               `json:"max_concurrent_migrations"`
	CostAlertThreshold float64           `json:"cost_alert_threshold"`
}

// 错误定义
var (
	ErrProviderNotFound  = errors.New("cloud provider not found")
	ErrProviderExists    = errors.New("cloud provider already exists")
	ErrNamespaceNotFound = errors.New("namespace not found")
	ErrNamespaceExists   = errors.New("namespace already exists")
	ErrObjectNotFound    = errors.New("object not found")
	ErrSyncTaskNotFound  = errors.New("sync task not found")
	ErrMigrationNotFound = errors.New("migration task not found")
	ErrInvalidProvider   = errors.New("invalid cloud provider")
	ErrInvalidStrategy   = errors.New("invalid placement strategy")
	ErrProviderOffline   = errors.New("cloud provider is offline")
	ErrSameProvider      = errors.New("source and target provider cannot be the same")
	ErrTaskInProgress    = errors.New("task already in progress")
	ErrMaxTasks          = errors.New("maximum concurrent tasks reached")
)
