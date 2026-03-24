// Package zfs 定义 ZFS 不可变快照接口
package zfs

import (
	"context"
	"time"
)

// ========== 核心接口定义 ==========

// PoolManager 池管理接口
type PoolManager interface {
	// ListPools 列出所有池
	ListPools(ctx context.Context) ([]PoolInfo, error)

	// GetPool 获取池信息
	GetPool(ctx context.Context, name string) (*PoolInfo, error)

	// CreatePool 创建池
	CreatePool(ctx context.Context, name string, config PoolCreateConfig) (*PoolInfo, error)

	// DestroyPool 销毁池
	DestroyPool(ctx context.Context, name string, force bool) error

	// ScrubPool 扫描池
	ScrubPool(ctx context.Context, name string) error

	// GetPoolStatus 获取池状态
	GetPoolStatus(ctx context.Context, name string) (*PoolStatus, error)
}

// PoolCreateConfig 池创建配置
type PoolCreateConfig struct {
	Vdevs      []VdevSpec      `json:"vdevs"`
	Properties map[string]string `json:"properties"`
	Features   []string        `json:"features"`
	Force      bool            `json:"force"`
}

// VdevSpec 虚拟设备规格
type VdevSpec struct {
	Type string   `json:"type"` // disk, file, mirror, raidz, raidz2, raidz3
	Paths []string `json:"paths"`
}

// PoolStatus 池状态
type PoolStatus struct {
	Name       string       `json:"name"`
	State      string       `json:"state"`
	Errors     []PoolError  `json:"errors"`
	Scan       ScanInfo     `json:"scan"`
	Config     PoolConfig   `json:"config"`
}

// PoolError 池错误
type PoolError struct {
	Type    string `json:"type"`
	Message string `json:"message"`
	Vdev    string `json:"vdev,omitempty"`
}

// PoolConfig 池配置
type PoolConfig struct {
	Name      string       `json:"name"`
	Vdevs     []VdevInfo   `json:"vdevs"`
}

// DatasetManager 数据集管理接口
type DatasetManager interface {
	// ListDatasets 列出数据集
	ListDatasets(ctx context.Context, pool string) ([]DatasetInfo, error)

	// GetDataset 获取数据集
	GetDataset(ctx context.Context, name string) (*DatasetInfo, error)

	// CreateDataset 创建数据集
	CreateDataset(ctx context.Context, name string, props map[string]string) (*DatasetInfo, error)

	// DestroyDataset 销毁数据集
	DestroyDataset(ctx context.Context, name string, recursive bool) error

	// SetProperty 设置属性
	SetProperty(ctx context.Context, dataset, prop, value string) error

	// GetProperty 获取属性
	GetProperty(ctx context.Context, dataset, prop string) (string, error)

	// ResizeDataset 调整数据集大小
	ResizeDataset(ctx context.Context, name string, size uint64) error
}

// SnapshotManager 快照管理接口
type SnapshotManager interface {
	// ListSnapshots 列出快照
	ListSnapshots(ctx context.Context, dataset string) ([]SnapshotInfo, error)

	// CreateSnapshot 创建快照
	CreateSnapshot(ctx context.Context, dataset string, opts SnapshotCreateOptions) (*SnapshotInfo, error)

	// DeleteSnapshot 删除快照
	DeleteSnapshot(ctx context.Context, fullName string, force bool) error

	// RollbackSnapshot 回滚快照
	RollbackSnapshot(ctx context.Context, fullName string, opts SnapshotRestoreOptions) error

	// CloneSnapshot 克隆快照
	CloneSnapshot(ctx context.Context, fullName string, opts CloneOptions) (*DatasetInfo, error)

	// GetSnapshot 获取快照信息
	GetSnapshot(ctx context.Context, fullName string) (*SnapshotInfo, error)

	// RenameSnapshot 重命名快照
	RenameSnapshot(ctx context.Context, oldName, newName string) error
}

// ImmutableSnapshotManager 不可变快照管理接口
type ImmutableSnapshotManager interface {
	SnapshotManager

	// SetImmutable 设置不可变
	SetImmutable(ctx context.Context, fullName string, lockType LockType, expiryTime *time.Time) error

	// ReleaseImmutable 释放不可变
	ReleaseImmutable(ctx context.Context, fullName string, approver string) error

	// ListImmutableSnapshots 列出不可变快照
	ListImmutableSnapshots() []SnapshotInfo

	// VerifySnapshot 验证快照
	VerifySnapshot(ctx context.Context, fullName string) error

	// GetVerificationStatus 获取验证状态
	GetVerificationStatus(fullName string) (verified bool, verifiedAt time.Time, checksum string)
}

// HoldManager 保留管理接口
type HoldManager interface {
	// SetHold 设置保留
	SetHold(ctx context.Context, snapshot, tag string) error

	// ReleaseHold 释放保留
	ReleaseHold(ctx context.Context, snapshot, tag string) error

	// ListHolds 列出保留
	ListHolds(ctx context.Context, snapshot string) ([]HoldInfo, error)
}

// BookmarkManager 书签管理接口
type BookmarkManager interface {
	// CreateBookmark 创建书签
	CreateBookmark(ctx context.Context, snapshot, bookmark string) error

	// DeleteBookmark 删除书签
	DeleteBookmark(ctx context.Context, bookmark string) error

	// ListBookmarks 列出书签
	ListBookmarks(ctx context.Context, dataset string) ([]BookmarkInfo, error)
}

// BookmarkInfo 书签信息
type BookmarkInfo struct {
	Name         string    `json:"name"`
	FullName     string    `json:"fullName"`
	Dataset      string    `json:"dataset"`
	CreationTime time.Time `json:"creationTime"`
	Source       string    `json:"source"` // 源快照
}

// SendReceiveManager 发送/接收管理接口
type SendReceiveManager interface {
	// SendSnapshot 发送快照
	SendSnapshot(ctx context.Context, snapshot string, opts SendOptions) error

	// ReceiveSnapshot 接收快照
	ReceiveSnapshot(ctx context.Context, dataset string, opts ReceiveOptions) error

	// EstimateSendSize 估算发送大小
	EstimateSendSize(ctx context.Context, snapshot string) (uint64, error)
}

// SendOptions 发送选项
type SendOptions struct {
	Incremental   string `json:"incremental,omitempty"` // 增量发送的基准快照
	Compress      bool   `json:"compress"`
	Raw           bool   `json:"raw"`      // 发送加密数据
	Properties    bool   `json:"properties"` // 包含属性
	Verbose       bool   `json:"verbose"`
	OutputFile    string `json:"outputFile,omitempty"`
	BufferSize    int    `json:"bufferSize"`
}

// ReceiveOptions 接收选项
type ReceiveOptions struct {
	Force         bool   `json:"force"`
	Resumable     bool   `json:"resumable"`
	Properties    map[string]string `json:"properties,omitempty"`
	Origin        string `json:"origin,omitempty"` // 用于克隆
	InputFile     string `json:"inputFile,omitempty"`
	BufferSize    int    `json:"bufferSize"`
}

// EncryptionManager 加密管理接口
type EncryptionManager interface {
	// CreateEncryptedDataset 创建加密数据集
	CreateEncryptedDataset(ctx context.Context, name string, opts EncryptionOptions) (*DatasetInfo, error)

	// LoadKey 加载密钥
	LoadKey(ctx context.Context, dataset string, key string) error

	// UnloadKey 卸载密钥
	UnloadKey(ctx context.Context, dataset string) error

	// ChangeKey 更改密钥
	ChangeKey(ctx context.Context, dataset string, oldKey, newKey string) error

	// IsEncrypted 检查是否加密
	IsEncrypted(ctx context.Context, dataset string) (bool, error)
}

// EncryptionOptions 加密选项
type EncryptionOptions struct {
	Algorithm   string `json:"algorithm"`   // aes-256-gcm, aes-128-gcm
	KeyLocation string `json:"keyLocation"` // prompt, file, passphrase
	Key         string `json:"key,omitempty"`
	Passphrase  string `json:"passphrase,omitempty"`
}

// QuotaManager 配额管理接口
type QuotaManager interface {
	// SetQuota 设置配额
	SetQuota(ctx context.Context, dataset string, quota uint64) error

	// SetRefQuota 设置引用配额
	SetRefQuota(ctx context.Context, dataset string, quota uint64) error

	// SetReservation 设置预留
	SetReservation(ctx context.Context, dataset string, reservation uint64) error

	// SetRefReservation 设置引用预留
	SetRefReservation(ctx context.Context, dataset string, reservation uint64) error

	// GetUsage 获取使用情况
	GetUsage(ctx context.Context, dataset string) (*UsageInfo, error)
}

// UsageInfo 使用信息
type UsageInfo struct {
	Used          uint64 `json:"used"`
	Avail         uint64 `json:"avail"`
	Referenced    uint64 `json:"referenced"`
	Quota         uint64 `json:"quota"`
	RefQuota      uint64 `json:"refQuota"`
	Reservation   uint64 `json:"reservation"`
	RefReservation uint64 `json:"refReservation"`
}

// ReplicationManager 复制管理接口
type ReplicationManager interface {
	// CreateReplicationTask 创建复制任务
	CreateReplicationTask(ctx context.Context, config ReplicationConfig) (*ReplicationTask, error)

	// RunReplicationTask 运行复制任务
	RunReplicationTask(ctx context.Context, taskID string) error

	// CancelReplicationTask 取消复制任务
	CancelReplicationTask(ctx context.Context, taskID string) error

	// GetReplicationStatus 获取复制状态
	GetReplicationStatus(ctx context.Context, taskID string) (*ReplicationStatus, error)

	// ListReplicationTasks 列出复制任务
	ListReplicationTasks(ctx context.Context) ([]ReplicationTask, error)
}

// ReplicationConfig 复制配置
type ReplicationConfig struct {
	Name           string        `json:"name"`
	SourceDataset  string        `json:"sourceDataset"`
	TargetHost     string        `json:"targetHost"`
	TargetDataset  string        `json:"targetDataset"`
	Recursive      bool          `json:"recursive"`
	Compress       bool          `json:"compress"`
	Encryption     bool          `json:"encryption"`
	Schedule       string        `json:"schedule"` // cron 表达式
	Retention      RetentionSpec `json:"retention"`
}

// RetentionSpec 保留规格
type RetentionSpec struct {
	KeepLast   int  `json:"keepLast"`
	KeepHourly int  `json:"keepHourly"`
	KeepDaily  int  `json:"keepDaily"`
	KeepWeekly int  `json:"keepWeekly"`
	KeepMonthly int  `json:"keepMonthly"`
	KeepYearly int  `json:"keepYearly"`
}

// ReplicationTask 复制任务
type ReplicationTask struct {
	ID           string            `json:"id"`
	Config       ReplicationConfig `json:"config"`
	State        string            `json:"state"` // pending, running, completed, failed
	LastRun      time.Time         `json:"lastRun"`
	NextRun      time.Time         `json:"nextRun"`
	BytesSent    uint64            `json:"bytesSent"`
	BytesReceived uint64           `json:"bytesReceived"`
	SnapshotsSent int              `json:"snapshotsSent"`
	Errors       []string          `json:"errors,omitempty"`
}

// ReplicationStatus 复制状态
type ReplicationStatus struct {
	TaskID         string    `json:"taskId"`
	State          string    `json:"state"`
	Progress       float64   `json:"progress"`
	CurrentSnapshot string   `json:"currentSnapshot,omitempty"`
	BytesTransferred uint64  `json:"bytesTransferred"`
	TotalBytes     uint64    `json:"totalBytes"`
	StartTime      time.Time `json:"startTime"`
	ETA            time.Time `json:"eta,omitempty"`
	Speed          float64   `json:"speed"` // MB/s
	Errors         int       `json:"errors"`
}

// ZFSProvider 完整的 ZFS 提供者接口
type ZFSProvider interface {
	PoolManager
	DatasetManager
	ImmutableSnapshotManager
	HoldManager
	BookmarkManager
	SendReceiveManager
	EncryptionManager
	QuotaManager
	ReplicationManager

	// IsAvailable 检查是否可用
	IsAvailable() bool

	// Close 关闭
	Close() error
}