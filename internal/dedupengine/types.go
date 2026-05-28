// Package dedupengine 提供基于内容寻址的块级重复数据删除功能
package dedupengine

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrBlockNotFound 内容块不存在.
	ErrBlockNotFound = errors.New("内容块不存在")
	// ErrInvalidBlockSize 无效块大小.
	ErrInvalidBlockSize = errors.New("无效块大小")
	// ErrInvalidData 无效数据.
	ErrInvalidData = errors.New("无效数据")
	// ErrBlockInUse 内容块正在被引用，无法删除.
	ErrBlockInUse = errors.New("内容块正在被引用，无法删除")
	// ErrHashMismatch 哈希校验失败.
	ErrHashMismatch = errors.New("哈希校验失败")
)

// ========== 块大小 ==========

// BlockSize 块大小枚举.
type BlockSize int

const (
	// BlockSize4K 4KB 块.
	BlockSize4K BlockSize = 4 * 1024
	// BlockSize64K 64KB 块.
	BlockSize64K BlockSize = 64 * 1024
	// BlockSize1M 1MB 块.
	BlockSize1M BlockSize = 1024 * 1024
)

// String 返回块大小的可读表示.
func (bs BlockSize) String() string {
	switch bs {
	case BlockSize4K:
		return "4KB"
	case BlockSize64K:
		return "64KB"
	case BlockSize1M:
		return "1MB"
	default:
		return "unknown"
	}
}

// ========== 核心类型 ==========

// ContentBlock 内容块.
type ContentBlock struct {
	Hash      string    `json:"hash"`       // SHA-256 哈希值
	Data      []byte    `json:"-"`          // 原始数据（JSON 序列化时忽略）
	RefCount  int       `json:"ref_count"`  // 引用计数
	Size      int       `json:"size"`       // 数据大小（字节）
	CreatedAt time.Time `json:"created_at"` // 创建时间
}

// DedupStats 去重统计信息.
type DedupStats struct {
	TotalBlocks    int   `json:"total_blocks"`    // 总块数（含重复引用）
	UniqueBlocks   int   `json:"unique_blocks"`   // 唯一块数
	DuplicateBlocks int  `json:"duplicate_blocks"` // 重复块数
	TotalSize      int64 `json:"total_size"`      // 总数据大小（字节）
	UniqueSize     int64 `json:"unique_size"`     // 去重后存储大小（字节）
	SpaceSaved     int64 `json:"space_saved"`     // 节省空间（字节）
}

// DedupRatio 返回去重率（0-1 之间，越高表示去重效果越好）.
func (s DedupStats) DedupRatio() float64 {
	if s.TotalBlocks == 0 {
		return 0
	}
	return float64(s.DuplicateBlocks) / float64(s.TotalBlocks)
}

// SpaceSavingsPercent 返回节省空间的百分比.
func (s DedupStats) SpaceSavingsPercent() float64 {
	if s.TotalSize == 0 {
		return 0
	}
	return float64(s.SpaceSaved) / float64(s.TotalSize) * 100
}

// StoreResult 存储结果.
type StoreResult struct {
	Hash      string `json:"hash"`       // 内容哈希
	IsNew     bool   `json:"is_new"`     // 是否为新块
	RefCount  int    `json:"ref_count"`  // 当前引用计数
	Size      int    `json:"size"`       // 数据大小
}
