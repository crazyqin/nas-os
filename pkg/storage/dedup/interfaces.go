// Package dedup 定义高性能去重接口
package dedup

import (
	"context"
	"io"
)

// DedupEngine 去重引擎接口
// 定义去重系统的核心操作
type DedupEngine interface {
	// Lookup 查找块哈希对应的条目
	Lookup(hash ChunkHash) (*DDTEntry, error)

	// Insert 插入新条目或增加引用计数
	Insert(hash ChunkHash, size uint32) (*DDTEntry, bool, error)

	// Remove 移除条目
	Remove(hash ChunkHash) error

	// GetStats 获取统计信息
	GetStats() DDTStats

	// Close 关闭引擎
	Close() error
}

// DedupWriter 去重写入器接口
type DedupWriter interface {
	// Write 写入数据并执行去重
	Write(ctx context.Context, data []byte) (*DDTEntry, bool, error)

	// WriteStream 写入流并执行去重
	WriteStream(ctx context.Context, r io.Reader) ([]*DDTEntry, uint64, error)
}

// DedupReader 去重读取器接口
type DedupReader interface {
	// Read 读取块数据
	Read(ctx context.Context, hash ChunkHash) ([]byte, error)

	// Verify 验证块数据完整性
	Verify(ctx context.Context, hash ChunkHash) (bool, error)
}

// DedupManager 去重管理器接口
// 提供完整的去重生命周期管理
type DedupManager interface {
	DedupEngine
	DedupWriter
	DedupReader

	// CreateSnapshot 创建快照
	CreateSnapshot(ctx context.Context, name string) error

	// RestoreSnapshot 恢复快照
	RestoreSnapshot(ctx context.Context, name string) error

	// ListSnapshots 列出所有快照
	ListSnapshots() ([]string, error)

	// Compact 压缩表
	Compact(ctx context.Context) error

	// Export 导出表
	Export(ctx context.Context, w io.Writer) error

	// Import 导入表
	Import(ctx context.Context, r io.Reader) error
}

// ChunkProcessor 块处理器接口
// 用于自定义块处理逻辑
type ChunkProcessor interface {
	// Process 处理块
	Process(ctx context.Context, chunk []byte, hash ChunkHash) error

	// ShouldDedup 判断是否应该去重
	ShouldDedup(chunk []byte) bool

	// PreProcess 预处理块（在计算哈希之前）
	PreProcess(chunk []byte) ([]byte, error)

	// PostProcess 后处理块（在存储之后）
	PostProcess(chunk []byte, entry *DDTEntry) error
}

// HashProvider 哈希提供者接口
type HashProvider interface {
	// Compute 计算哈希
	Compute(data []byte) ChunkHash

	// Name 返回算法名称
	Name() string

	// Size 返回哈希大小（字节）
	Size() int
}

// StorageBackend 存储后端接口
// 用于抽象底层存储实现
type StorageBackend interface {
	// Write 写入块数据
	Write(ctx context.Context, block uint64, data []byte) error

	// Read 读取块数据
	Read(ctx context.Context, block uint64) ([]byte, error)

	// Delete 删除块数据
	Delete(ctx context.Context, block uint64) error

	// Exists 检查块是否存在
	Exists(ctx context.Context, block uint64) bool

	// Allocate 分配新块
	Allocate(ctx context.Context, size uint32) (uint64, error)

	// Free 释放块
	Free(ctx context.Context, block uint64) error

	// Stats 返回存储统计
	Stats() StorageStats
}

// StorageStats 存储统计
type StorageStats struct {
	TotalBlocks   uint64  `json:"totalBlocks"`
	UsedBlocks    uint64  `json:"usedBlocks"`
	FreeBlocks    uint64  `json:"freeBlocks"`
	TotalBytes    uint64  `json:"totalBytes"`
	UsedBytes     uint64  `json:"usedBytes"`
	Fragmentation float64 `json:"fragmentation"`
}

// DedupPolicy 去重策略接口
type DedupPolicy interface {
	// ShouldDedup 判断是否应该对数据进行去重
	ShouldDedup(data []byte, info ChunkInfo) bool

	// GetChunkSize 获取块大小
	GetChunkSize(data []byte) int

	// GetRetention 获取保留策略
	GetRetention(hash ChunkHash) RetentionAction

	// OnDuplicate 当发现重复时的处理
	OnDuplicate(hash ChunkHash, existing *DDTEntry) DuplicateAction
}

// ChunkInfo 块信息
type ChunkInfo struct {
	Path     string
	Size     int64
	Offset   int64
	Modified bool
	UserData map[string]interface{}
}

// RetentionAction 保留动作
type RetentionAction int

const (
	// RetentionKeep indicates keeping the snapshot
	RetentionKeep RetentionAction = iota
	// RetentionDelete indicates deleting the snapshot
	RetentionDelete
	// RetentionArchive indicates archiving the snapshot
	RetentionArchive
)

// DuplicateAction 重复动作
type DuplicateAction int

const (
	// DuplicateSkip indicates skipping duplicate
	DuplicateSkip DuplicateAction = iota
	// DuplicateUpdate indicates updating duplicate
	DuplicateUpdate
	// DuplicateReplace indicates replacing duplicate
	DuplicateReplace
)

// DedupEventListener 去重事件监听器
type DedupEventListener interface {
	// OnInsert 当插入新条目时调用
	OnInsert(entry *DDTEntry, isNew bool)

	// OnHit 当命中缓存时调用
	OnHit(hash ChunkHash, entry *DDTEntry)

	// OnMiss 当未命中时调用
	OnMiss(hash ChunkHash)

	// OnRemove 当移除条目时调用
	OnRemove(hash ChunkHash, entry *DDTEntry)

	// OnError 当发生错误时调用
	OnError(err error, operation string, hash ChunkHash)
}

// DedupMetrics 去重度量接口
type DedupMetrics interface {
	// RecordHit 记录命中
	RecordHit()

	// RecordMiss 记录未命中
	RecordMiss()

	// RecordWrite 记录写入
	RecordWrite(bytes int64)

	// RecordDedup 记录去重
	RecordDedup(bytes int64)

	// RecordError 记录错误
	RecordError(operation string)

	// GetHitRate 获取命中率
	GetHitRate() float64

	// GetDedupRate 获取去重率
	GetDedupRate() float64
}

// DedupConfigProvider 配置提供者接口
type DedupConfigProvider interface {
	// GetConfig 获取当前配置
	GetConfig() *DedupConfig

	// UpdateConfig 更新配置
	UpdateConfig(config *DedupConfig) error

	// WatchConfig 监听配置变更
	WatchConfig(callback func(*DedupConfig)) error
}

// DedupSerializer 序列化接口
type DedupSerializer interface {
	// SerializeEntry 序列化条目
	SerializeEntry(entry *DDTEntry) ([]byte, error)

	// DeserializeEntry 反序列化条目
	DeserializeEntry(data []byte) (*DDTEntry, error)

	// SerializeTable 序列化整个表
	SerializeTable(entries map[ChunkHash]*DDTEntry) ([]byte, error)

	// DeserializeTable 反序列化整个表
	DeserializeTable(data []byte) (map[ChunkHash]*DDTEntry, error)
}

// DedupIndexer 索引器接口
// 用于加速查找
type DedupIndexer interface {
	// Index 建立索引
	Index(hash ChunkHash, entry *DDTEntry) error

	// Lookup 查找
	Lookup(hash ChunkHash) (*DDTEntry, error)

	// Delete 删除索引
	Delete(hash ChunkHash) error

	// Rebuild 重建索引
	Rebuild(entries map[ChunkHash]*DDTEntry) error

	// Size 返回索引大小
	Size() int
}
