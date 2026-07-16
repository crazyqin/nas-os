// Package bitrotheal 提供数据完整性校验与自愈功能
// 检测位翻转（bitrot）并从冗余副本自动修复受损数据。
package bitrotheal

import (
	"errors"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrPathRequired 必须指定路径.
	ErrPathRequired = errors.New("必须指定路径")
	// ErrFileNotFound 文件不存在.
	ErrFileNotFound = errors.New("文件不存在")
	// ErrChecksumNotFound 未找到校验和记录.
	ErrChecksumNotFound = errors.New("未找到校验和记录")
	// ErrChecksumMismatch 校验和不匹配，数据可能已损坏.
	ErrChecksumMismatch = errors.New("校验和不匹配")
	// ErrRepairFailed 修复失败.
	ErrRepairFailed = errors.New("修复失败")
	// ErrNoRedundancy 没有可用的冗余副本.
	ErrNoRedundancy = errors.New("没有可用的冗余副本")
	// ErrInvalidAlgorithm 不支持的校验算法.
	ErrInvalidAlgorithm = errors.New("不支持的校验算法")
)

// ========== 枚举类型 ==========

// CheckAlgorithm 校验算法.
type CheckAlgorithm string

const (
	// AlgorithmCRC32 CRC-32 算法（快速，适合大量小文件）.
	AlgorithmCRC32 CheckAlgorithm = "crc32"
	// AlgorithmSHA256 SHA-256 算法（默认，安全性高）.
	AlgorithmSHA256 CheckAlgorithm = "sha256"
)

// RepairStrategy 修复策略.
type RepairStrategy string

const (
	// RepairFromReplica 从冗余副本修复.
	RepairFromReplica RepairStrategy = "replica"
	// RepairFromBackup 从备份恢复.
	RepairFromBackup RepairStrategy = "backup"
	// RepairFromRAID 从 RAID 冗余恢复.
	RepairFromRAID RepairStrategy = "raid"
	// RepairManual 需要人工干预.
	RepairManual RepairStrategy = "manual"
)

// ========== 配置 ==========

// HealConfig 自愈引擎配置.
type HealConfig struct {
	// Algorithm 校验算法（默认 SHA-256）.
	Algorithm CheckAlgorithm
	// ReplicaPaths 冗余副本搜索路径.
	ReplicaPaths []string
	// BackupRoot 备份根目录.
	BackupRoot string
	// ScanInterval 定期扫描间隔.
	ScanInterval time.Duration
	// AutoRepair 是否自动修复.
	AutoRepair bool
	// MaxConcurrent 最大并发扫描数.
	MaxConcurrent int
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *HealConfig {
	return &HealConfig{
		Algorithm:     AlgorithmSHA256,
		ScanInterval:  24 * time.Hour,
		AutoRepair:    true,
		MaxConcurrent: 4,
	}
}

// ========== 核心类型 ==========

// ChecksumEntry 校验和记录.
type ChecksumEntry struct {
	// Path 文件路径.
	Path string `json:"path"`
	// Algorithm 校验算法.
	Algorithm CheckAlgorithm `json:"algorithm"`
	// Checksum 校验和值（十六进制）.
	Checksum string `json:"checksum"`
	// LastVerified 最后校验时间.
	LastVerified time.Time `json:"last_verified"`
	// RepairCount 修复次数.
	RepairCount int `json:"repair_count"`
	// FileSize 文件大小（字节）.
	FileSize int64 `json:"file_size"`
}

// IntegrityReport 完整性扫描报告.
type IntegrityReport struct {
	// ScannedFiles 扫描文件总数.
	ScannedFiles int `json:"scanned_files"`
	// CorruptedFiles 发现损坏文件数.
	CorruptedFiles int `json:"corrupted_files"`
	// RepairedFiles 已修复文件数.
	RepairedFiles int `json:"repaired_files"`
	// UnrepairableFiles 无法修复文件数.
	UnrepairableFiles int `json:"unrepairable_files"`
	// ScanDuration 扫描耗时.
	ScanDuration time.Duration `json:"scan_duration"`
	// CorruptedPaths 损坏文件路径列表.
	CorruptedPaths []string `json:"corrupted_paths,omitempty"`
	// UnrepairablePaths 无法修复文件路径列表.
	UnrepairablePaths []string `json:"unrepairable_paths,omitempty"`
	// StartTime 扫描开始时间.
	StartTime time.Time `json:"start_time"`
}

// RepairResult 修复结果.
type RepairResult struct {
	// Path 文件路径.
	Path string `json:"path"`
	// Success 是否修复成功.
	Success bool `json:"success"`
	// Strategy 使用的修复策略.
	Strategy RepairStrategy `json:"strategy"`
	// SourcePath 修复来源路径.
	SourcePath string `json:"source_path,omitempty"`
	// Error 修复失败原因.
	Error string `json:"error,omitempty"`
	// Duration 修复耗时.
	Duration time.Duration `json:"duration"`
}

// ========== 引擎结构 ==========

// HealEngine 自愈引擎.
type HealEngine struct {
	mu       sync.RWMutex
	config   *HealConfig
	checksum map[string]*ChecksumEntry // path -> entry
	scanning bool
	stopCh   chan struct{}
}
