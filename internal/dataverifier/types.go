// Package dataverifier 提供数据完整性校验引擎
package dataverifier

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrJobNotFound 校验任务不存在.
	ErrJobNotFound = errors.New("校验任务不存在")
	// ErrJobAlreadyExists 校验任务已存在.
	ErrJobAlreadyExists = errors.New("校验任务已存在")
	// ErrInvalidPath 无效路径.
	ErrInvalidPath = errors.New("无效路径")
	// ErrJobRunning 任务正在运行中.
	ErrJobRunning = errors.New("任务正在运行中")
	// ErrChecksumMismatch 校验和不匹配.
	ErrChecksumMismatch = errors.New("校验和不匹配")
)

// ========== 核心类型 ==========

// VerifyAlgorithm 校验算法.
type VerifyAlgorithm string

const (
	// AlgorithmCRC32 CRC32算法.
	AlgorithmCRC32 VerifyAlgorithm = "crc32"
	// AlgorithmSHA256 SHA256算法.
	AlgorithmSHA256 VerifyAlgorithm = "sha256"
	// AlgorithmSHA512 SHA512算法.
	AlgorithmSHA512 VerifyAlgorithm = "sha512"
	// AlgorithmXXHASH XXHash算法（高速）.
	AlgorithmXXHASH VerifyAlgorithm = "xxhash"
	// AlgorithmBLAKE3 BLAKE3算法（高速安全）.
	AlgorithmBLAKE3 VerifyAlgorithm = "blake3"
)

// JobStatus 任务状态.
type JobStatus string

const (
	// JobStatusPending 等待执行.
	JobStatusPending JobStatus = "pending"
	// JobStatusRunning 运行中.
	JobStatusRunning JobStatus = "running"
	// JobStatusCompleted 已完成.
	JobStatusCompleted JobStatus = "completed"
	// JobStatusFailed 失败.
	JobStatusFailed JobStatus = "failed"
	// JobStatusCancelled 已取消.
	JobStatusCancelled JobStatus = "cancelled"
)

// ScheduleType 调度类型.
type ScheduleType string

const (
	// ScheduleManual 手动触发.
	ScheduleManual ScheduleType = "manual"
	// ScheduleInterval 定时间隔.
	ScheduleInterval ScheduleType = "interval"
	// ScheduleCron Cron表达式.
	ScheduleCron ScheduleType = "cron"
)

// ========== 数据结构 ==========

// VerifyJob 校验任务.
type VerifyJob struct {
	ID         string          `json:"id"`          // 任务ID
	Name       string          `json:"name"`        // 任务名称
	Paths      []string        `json:"paths"`       // 校验路径列表
	Algorithm  VerifyAlgorithm `json:"algorithm"`   // 校验算法
	Schedule   ScheduleConfig  `json:"schedule"`    // 调度配置
	Status     JobStatus       `json:"status"`      // 当前状态
	LastRun    *time.Time      `json:"last_run"`    // 上次运行时间
	NextRun    *time.Time      `json:"next_run"`    // 下次运行时间
	FileCount  int64           `json:"file_count"`  // 文件总数
	ErrorCount int64           `json:"error_count"` // 错误数
	CreatedAt  time.Time       `json:"created_at"`  // 创建时间
	UpdatedAt  time.Time       `json:"updated_at"`  // 更新时间
}

// ScheduleConfig 调度配置.
type ScheduleConfig struct {
	Type     ScheduleType  `json:"type"`                // 调度类型
	Interval time.Duration `json:"interval,omitempty"`  // 间隔时长
	CronExpr string        `json:"cron_expr,omitempty"` // Cron表达式
}

// VerifyResult 校验结果.
type VerifyResult struct {
	JobID        string        `json:"job_id"`        // 任务ID
	TotalFiles   int64         `json:"total_files"`   // 总文件数
	CheckedFiles int64         `json:"checked_files"` // 已检查文件数
	PassedFiles  int64         `json:"passed_files"`  // 通过文件数
	FailedFiles  int64         `json:"failed_files"`  // 失败文件数
	SkipFiles    int64         `json:"skip_files"`    // 跳过文件数
	StartTime    time.Time     `json:"start_time"`    // 开始时间
	EndTime      time.Time     `json:"end_time"`      // 结束时间
	Duration     time.Duration `json:"duration"`      // 耗时
	Errors       []FileError   `json:"errors"`        // 错误列表
}

// FileError 文件错误.
type FileError struct {
	Path     string `json:"path"`     // 文件路径
	Error    string `json:"error"`    // 错误信息
	Expected string `json:"expected"` // 期望校验和
	Actual   string `json:"actual"`   // 实际校验和
}

// ChecksumEntry 校验和记录.
type ChecksumEntry struct {
	Path      string          `json:"path"`       // 文件路径
	Algorithm VerifyAlgorithm `json:"algorithm"`  // 算法
	Hash      string          `json:"hash"`       // 校验和
	Size      int64           `json:"size"`       // 文件大小
	ModTime   time.Time       `json:"mod_time"`   // 修改时间
	UpdatedAt time.Time       `json:"updated_at"` // 更新时间
}

// VerifyStats 校验统计.
type VerifyStats struct {
	TotalJobs      int64         `json:"total_jobs"`       // 总任务数
	RunningJobs    int64         `json:"running_jobs"`     // 运行中任务数
	TotalFiles     int64         `json:"total_files"`      // 总文件数
	TotalErrors    int64         `json:"total_errors"`     // 总错误数
	LastVerifyTime *time.Time    `json:"last_verify_time"` // 最后校验时间
	AvgDuration    time.Duration `json:"avg_duration"`     // 平均耗时
}

// CreateJobRequest 创建任务请求.
type CreateJobRequest struct {
	Name      string          `json:"name" binding:"required"`
	Paths     []string        `json:"paths" binding:"required,min=1"`
	Algorithm VerifyAlgorithm `json:"algorithm"`
	Schedule  ScheduleConfig  `json:"schedule"`
}

// UpdateJobRequest 更新任务请求.
type UpdateJobRequest struct {
	Name      *string          `json:"name,omitempty"`
	Paths     []string         `json:"paths,omitempty"`
	Algorithm *VerifyAlgorithm `json:"algorithm,omitempty"`
	Schedule  *ScheduleConfig  `json:"schedule,omitempty"`
}
