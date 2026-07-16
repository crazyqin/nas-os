// Package smartlifecycle 提供智能文件生命周期管理功能
// 支持文件老化分析、自动归档、智能清理、存储回收
package smartlifecycle

import (
	"time"
)

// PolicyType 策略类型.
type PolicyType string

const (
	PolicyTypeArchive  PolicyType = "archive"  // 归档策略
	PolicyTypeDelete   PolicyType = "delete"   // 删除策略
	PolicyTypeCompress PolicyType = "compress" // 压缩策略
	PolicyTypeMove     PolicyType = "move"     // 迁移策略
)

// FileStatus 文件生命周期状态.
type FileStatus string

const (
	FileStatusActive   FileStatus = "active"   // 活跃
	FileStatusWarm     FileStatus = "warm"     // 温数据
	FileStatusCold     FileStatus = "cold"     // 冷数据
	FileStatusArchived FileStatus = "archived" // 已归档
	FileStatusExpired  FileStatus = "expired"  // 已过期
)

// LifecyclePolicy 生命周期策略.
type LifecyclePolicy struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Description string     `json:"description,omitempty"`
	Type        PolicyType `json:"type"`
	Enabled     bool       `json:"enabled"`
	// 触发条件
	MinAgeDays   int      `json:"minAgeDays"`   // 最小天数
	MaxSizeBytes int64    `json:"maxSizeBytes"` // 最大大小
	Extensions   []string `json:"extensions"`   // 文件扩展名过滤
	PathPattern  string   `json:"pathPattern"`  // 路径匹配模式
	ExcludePaths []string `json:"excludePaths"` // 排除路径
	// 动作参数
	TargetPath      string `json:"targetPath,omitempty"`      // 目标路径（move/archive）
	CompressLevel   int    `json:"compressLevel,omitempty"`   // 压缩级别 1-9
	DeleteAfterDays int    `json:"deleteAfterDays,omitempty"` // 归档后多少天删除
	// 元数据
	CreatedAt time.Time  `json:"createdAt"`
	UpdatedAt time.Time  `json:"updatedAt"`
	LastRunAt *time.Time `json:"lastRunAt,omitempty"`
}

// FileRecord 文件记录.
type FileRecord struct {
	ID           string     `json:"id"`
	Path         string     `json:"path"`
	Size         int64      `json:"size"`
	Status       FileStatus `json:"status"`
	LastAccessAt time.Time  `json:"lastAccessAt"`
	CreatedAt    time.Time  `json:"createdAt"`
	ModifiedAt   time.Time  `json:"modifiedAt"`
	Extension    string     `json:"extension"`
	Owner        string     `json:"owner,omitempty"`
	Tags         []string   `json:"tags,omitempty"`
}

// ScanResult 扫描结果.
type ScanResult struct {
	ID          string        `json:"id"`
	ScanTime    time.Time     `json:"scanTime"`
	TotalFiles  int           `json:"totalFiles"`
	TotalSize   int64         `json:"totalSize"`
	ActiveFiles int           `json:"activeFiles"`
	WarmFiles   int           `json:"warmFiles"`
	ColdFiles   int           `json:"coldFiles"`
	Candidates  []*FileRecord `json:"candidates"` // 可清理的文件
	SavedBytes  int64         `json:"savedBytes"` // 预计可回收空间
	Duration    time.Duration `json:"duration"`
}

// ExecutionResult 执行结果.
type ExecutionResult struct {
	ID         string    `json:"id"`
	PolicyID   string    `json:"policyId"`
	PolicyName string    `json:"policyName"`
	StartTime  time.Time `json:"startTime"`
	EndTime    time.Time `json:"endTime"`
	Processed  int       `json:"processed"`  // 处理文件数
	Success    int       `json:"success"`    // 成功数
	Failed     int       `json:"failed"`     // 失败数
	FreedBytes int64     `json:"freedBytes"` // 释放空间
	Errors     []string  `json:"errors,omitempty"`
}

// LifecycleStats 生命周期统计.
type LifecycleStats struct {
	TotalPolicies    int            `json:"totalPolicies"`
	ActivePolicies   int            `json:"activePolicies"`
	TotalScans       int            `json:"totalScans"`
	TotalExecutions  int            `json:"totalExecutions"`
	TotalFreedBytes  int64          `json:"totalFreedBytes"`
	StatusBreakdown  map[string]int `json:"statusBreakdown"`
	LastScanTime     *time.Time     `json:"lastScanTime,omitempty"`
	NextScheduledRun *time.Time     `json:"nextScheduledRun,omitempty"`
}
