// Package tierrules 提供智能存储分层规则引擎功能。
// 参考群晖 DSM 7.3 的 "Smarter Tiering" 特性，支持基于文件属性的自动分层策略。
package tierrules

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrRuleNameEmpty 规则名称为空.
	ErrRuleNameEmpty = errors.New("规则名称不能为空")
	// ErrRuleNameDuplicate 规则名称重复.
	ErrRuleNameDuplicate = errors.New("规则名称已存在")
	// ErrRuleNotFound 规则不存在.
	ErrRuleNotFound = errors.New("规则不存在")
	// ErrInvalidTier 无效的存储层级.
	ErrInvalidTier = errors.New("无效的存储层级")
	// ErrSameTier 源层级和目标层级相同.
	ErrSameTier = errors.New("源层级和目标层级不能相同")
	// ErrFileNotFound 文件不存在.
	ErrFileNotFound = errors.New("文件不存在")
	// ErrNoMatchingRule 没有匹配的规则.
	ErrNoMatchingRule = errors.New("没有匹配的规则")
)

// ========== 存储层级常量 ==========

// StorageTier 存储层级类型.
type StorageTier string

const (
	// TierNVMe NVMe 高速层（最高性能）.
	TierNVMe StorageTier = "nvme"
	// TierSSD SSD 固态层.
	TierSSD StorageTier = "ssd"
	// TierHDD HDD 机械硬盘层.
	TierHDD StorageTier = "hdd"
	// TierCloud 云存储层.
	TierCloud StorageTier = "cloud"
	// TierArchive 归档层（最低成本）.
	TierArchive StorageTier = "archive"
)

// ValidTiers 所有合法的存储层级.
var ValidTiers = map[StorageTier]bool{
	TierNVMe:    true,
	TierSSD:     true,
	TierHDD:     true,
	TierCloud:   true,
	TierArchive: true,
}

// TierOrder 层级优先级顺序（数值越小越快）.
var TierOrder = map[StorageTier]int{
	TierNVMe:    0,
	TierSSD:     1,
	TierHDD:     2,
	TierCloud:   3,
	TierArchive: 4,
}

// ========== 核心数据结构 ==========

// TierConditions 分层条件定义.
type TierConditions struct {
	// MaxAccessDays 最大未访问天数（超过此天数未访问则匹配）.
	MaxAccessDays int `json:"maxAccessDays,omitempty"`
	// MinAgeDays 最小文件年龄天数（创建超过此天数才匹配）.
	MinAgeDays int `json:"minAgeDays,omitempty"`
	// FilePatterns 文件名匹配模式（支持通配符，如 "*.log", "*.tmp"）.
	FilePatterns []string `json:"filePatterns,omitempty"`
	// MinSizeBytes 最小文件大小（字节）.
	MinSizeBytes int64 `json:"minSizeBytes,omitempty"`
	// MaxSizeBytes 最大文件大小（字节）.
	MaxSizeBytes int64 `json:"maxSizeBytes,omitempty"`
}

// TierRule 分层规则.
type TierRule struct {
	// Name 规则名称（唯一标识）.
	Name string `json:"name"`
	// Description 规则描述.
	Description string `json:"description,omitempty"`
	// SourceTier 源存储层级.
	SourceTier StorageTier `json:"sourceTier"`
	// TargetTier 目标存储层级.
	TargetTier StorageTier `json:"targetTier"`
	// Conditions 匹配条件.
	Conditions TierConditions `json:"conditions"`
	// Priority 优先级（数值越大越优先）.
	Priority int `json:"priority"`
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
	// CreatedAt 创建时间.
	CreatedAt time.Time `json:"createdAt"`
	// UpdatedAt 更新时间.
	UpdatedAt time.Time `json:"updatedAt"`
}

// TierStats 分层迁移统计.
type TierStats struct {
	// TotalMoved 总迁移文件数.
	TotalMoved int64 `json:"totalMoved"`
	// TotalBytes 总迁移字节数.
	TotalBytes int64 `json:"totalBytes"`
	// LastRunTime 最近一次运行时间.
	LastRunTime time.Time `json:"lastRunTime"`
	// ErrorCount 累计错误次数.
	ErrorCount int64 `json:"errorCount"`
}

// FileInfo 文件信息（用于规则评估）.
type FileInfo struct {
	// Path 文件路径.
	Path string `json:"path"`
	// Name 文件名.
	Name string `json:"name"`
	// Size 文件大小（字节）.
	Size int64 `json:"size"`
	// ModTime 最后修改时间.
	ModTime time.Time `json:"modTime"`
	// AccessTime 最后访问时间.
	AccessTime time.Time `json:"accessTime"`
	// CurrentTier 当前所在存储层级.
	CurrentTier StorageTier `json:"currentTier"`
	// IsDir 是否为目录.
	IsDir bool `json:"isDir"`
}

// EvaluateRequest 评估请求.
type EvaluateRequest struct {
	// File 待评估的文件信息.
	File FileInfo `json:"file"`
}

// EvaluateResponse 评估响应.
type EvaluateResponse struct {
	// File 文件路径.
	File string `json:"file"`
	// CurrentTier 当前层级.
	CurrentTier StorageTier `json:"currentTier"`
	// RecommendedTier 推荐目标层级.
	RecommendedTier StorageTier `json:"recommendedTier"`
	// MatchedRule 匹配的规则名称.
	MatchedRule string `json:"matchedRule"`
	// ShouldMigrate 是否需要迁移.
	ShouldMigrate bool `json:"shouldMigrate"`
}

// RunRequest 批量运行请求.
type RunRequest struct {
	// DryRun 试运行模式（不实际迁移）.
	DryRun bool `json:"dryRun,omitempty"`
}

// RunResponse 批量运行响应.
type RunResponse struct {
	// Stats 运行统计.
	Stats TierStats `json:"stats"`
	// DryRun 是否为试运行.
	DryRun bool `json:"dryRun"`
}

// CreateRuleRequest 创建规则请求.
type CreateRuleRequest struct {
	// Name 规则名称.
	Name string `json:"name" binding:"required"`
	// Description 规则描述.
	Description string `json:"description,omitempty"`
	// SourceTier 源存储层级.
	SourceTier StorageTier `json:"sourceTier" binding:"required"`
	// TargetTier 目标存储层级.
	TargetTier StorageTier `json:"targetTier" binding:"required"`
	// Conditions 匹配条件.
	Conditions TierConditions `json:"conditions"`
	// Priority 优先级.
	Priority int `json:"priority"`
	// Enabled 是否启用.
	Enabled bool `json:"enabled"`
}
