// Package chaosengineer 提供 NAS 系统的混沌工程模块
// 支持故障注入测试、韧性评估、自动恢复验证、实验计划调度和安全边界控制
package chaosengineer

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

// ==================== 错误定义 ====================

var (
	ErrExperimentNotFound = errors.New("experiment not found")
	ErrExperimentRunning  = errors.New("experiment is already running")
	ErrExperimentNotRun   = errors.New("experiment is not running")
	ErrSafetyViolation    = errors.New("safety boundary violation")
	ErrInvalidFaultType   = errors.New("invalid fault type")
	ErrInvalidSeverity    = errors.New("invalid severity level")
	ErrNoTargetSpecified  = errors.New("no target specified")
	ErrRecoveryFailed     = errors.New("auto recovery failed")
	ErrExperimentConflict = errors.New("experiment conflict: another experiment targeting the same resource is running")
)

// ==================== 枚举类型 ====================

// FaultType 故障注入类型.
type FaultType string

const (
	FaultDiskFull       FaultType = "disk_full"       // 磁盘满
	FaultNetworkLatency FaultType = "network_latency" // 网络延迟
	FaultNetworkLoss    FaultType = "network_loss"    // 网络丢包
	FaultCPUStress      FaultType = "cpu_stress"      // CPU 压力
	FaultMemoryStress   FaultType = "memory_stress"   // 内存压力
	FaultIOStress       FaultType = "io_stress"       // IO 压力
	FaultProcessKill    FaultType = "process_kill"    // 进程终止
	FaultDiskIO         FaultType = "disk_io"         // 磁盘IO延迟
)

// Severity 严重程度.
type Severity string

const (
	SeverityLow      Severity = "low"      // 低
	SeverityMedium   Severity = "medium"   // 中
	SeverityHigh     Severity = "high"     // 高
	SeverityCritical Severity = "critical" // 严重
)

// ExperimentStatus 实验状态.
type ExperimentStatus string

const (
	StatusCreated   ExperimentStatus = "created"   // 已创建
	StatusRunning   ExperimentStatus = "running"   // 运行中
	StatusCompleted ExperimentStatus = "completed" // 已完成
	StatusFailed    ExperimentStatus = "failed"    // 失败
	StatusCancelled ExperimentStatus = "cancelled" // 已取消
	StatusScheduled ExperimentStatus = "scheduled" // 已调度
)

// RecoveryStatus 恢复状态.
type RecoveryStatus string

const (
	RecoveryPending RecoveryStatus = "pending" // 待恢复
	RecoveryRunning RecoveryStatus = "running" // 恢复中
	RecoverySuccess RecoveryStatus = "success" // 恢复成功
	RecoveryFailed  RecoveryStatus = "failed"  // 恢复失败
)

// TargetType 目标类型.
type TargetType string

const (
	TargetDisk    TargetType = "disk"    // 磁盘
	TargetNetwork TargetType = "network" // 网络
	TargetCPU     TargetType = "cpu"     // CPU
	TargetMemory  TargetType = "memory"  // 内存
	TargetProcess TargetType = "process" // 进程
	TargetService TargetType = "service" // 服务
)

// ScheduleType 调度类型.
type ScheduleType string

const (
	ScheduleImmediate ScheduleType = "immediate" // 立即执行
	ScheduleOnce      ScheduleType = "once"      // 定时一次
	ScheduleCron      ScheduleType = "cron"      // Cron 表达式
	ScheduleInterval  ScheduleType = "interval"  // 固定间隔
)

// ==================== 核心类型 ====================

// FaultConfig 故障注入配置.
type FaultConfig struct {
	Type       FaultType      `json:"type"`        // 故障类型
	Target     string         `json:"target"`      // 目标（磁盘路径、网卡名等）
	TargetType TargetType     `json:"target_type"` // 目标类型
	Severity   Severity       `json:"severity"`    // 严重程度
	Parameters map[string]any `json:"parameters"`  // 故障参数
	Duration   time.Duration  `json:"duration"`    // 持续时间
}

// SafetyBoundary 安全边界配置.
type SafetyBoundary struct {
	MaxDuration       time.Duration `json:"max_duration"`       // 最大持续时间
	MaxCPUUsage       float64       `json:"max_cpu_usage"`      // 最大 CPU 使用率 (0-100)
	MaxMemoryUsage    float64       `json:"max_memory_usage"`   // 最大内存使用率 (0-100)
	MaxDiskUsage      float64       `json:"max_disk_usage"`     // 最大磁盘使用率 (0-100)
	ProtectedPaths    []string      `json:"protected_paths"`    // 受保护路径
	ProtectedServices []string      `json:"protected_services"` // 受保护服务
	AutoRecover       bool          `json:"auto_recover"`       // 自动恢复
	RequireConfirm    bool          `json:"require_confirm"`    // 需要确认
}

// Schedule 实验调度配置.
type Schedule struct {
	Type      ScheduleType  `json:"type"`                 // 调度类型
	CronExpr  string        `json:"cron_expr,omitempty"`  // Cron 表达式
	Interval  time.Duration `json:"interval,omitempty"`   // 间隔时间
	StartTime *time.Time    `json:"start_time,omitempty"` // 开始时间
}

// Hypothesis 实验假设.
type Hypothesis struct {
	Description string `json:"description"` // 假设描述
	Expected    string `json:"expected"`    // 预期结果
}

// MetricPoint 指标数据点.
type MetricPoint struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
	Unit      string    `json:"unit"`
}

// ResilienceScore 韧性评分.
type ResilienceScore struct {
	Overall      float64            `json:"overall"`      // 总分 (0-100)
	Recovery     float64            `json:"recovery"`     // 恢复能力
	Stability    float64            `json:"stability"`    // 稳定性
	Availability float64            `json:"availability"` // 可用性
	Breakdown    map[string]float64 `json:"breakdown"`    // 分项评分
}

// RecoveryResult 恢复结果.
type RecoveryResult struct {
	Status    RecoveryStatus `json:"status"`     // 恢复状态
	StartTime time.Time      `json:"start_time"` // 开始时间
	EndTime   time.Time      `json:"end_time"`   // 结束时间
	Duration  time.Duration  `json:"duration"`   // 持续时间
	Steps     []RecoveryStep `json:"steps"`      // 恢复步骤
	Error     string         `json:"error"`      // 错误信息
}

// RecoveryStep 恢复步骤.
type RecoveryStep struct {
	Name      string    `json:"name"`       // 步骤名称
	Status    string    `json:"status"`     // 状态
	StartTime time.Time `json:"start_time"` // 开始时间
	EndTime   time.Time `json:"end_time"`   // 结束时间
	Error     string    `json:"error"`      // 错误信息
}

// Observation 实验观察记录.
type Observation struct {
	Timestamp time.Time          `json:"timestamp"`
	Phase     string             `json:"phase"` // before/during/after
	Metrics   map[string]float64 `json:"metrics"`
	Notes     string             `json:"notes"`
}

// Experiment 实验定义.
type Experiment struct {
	ID          string           `json:"id"`
	Name        string           `json:"name"`
	Description string           `json:"description"`
	Status      ExperimentStatus `json:"status"`
	Fault       FaultConfig      `json:"fault"`
	Safety      SafetyBoundary   `json:"safety"`
	Schedule    *Schedule        `json:"schedule,omitempty"`
	Hypothesis  *Hypothesis      `json:"hypothesis,omitempty"`
	Tags        []string         `json:"tags,omitempty"`

	// 运行时状态
	StartTime    *time.Time       `json:"start_time,omitempty"`
	EndTime      *time.Time       `json:"end_time,omitempty"`
	Recovery     *RecoveryResult  `json:"recovery,omitempty"`
	Resilience   *ResilienceScore `json:"resilience_score,omitempty"`
	Observations []Observation    `json:"observations,omitempty"`
	ErrorMsg     string           `json:"error_message,omitempty"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// ResilienceReport 韧性评估报告.
type ResilienceReport struct {
	ID                string           `json:"id"`
	GeneratedAt       time.Time        `json:"generated_at"`
	TotalExperiments  int              `json:"total_experiments"`
	PassedExperiments int              `json:"passed_experiments"`
	FailedExperiments int              `json:"failed_experiments"`
	OverallScore      float64          `json:"overall_score"`
	Score             *ResilienceScore `json:"score"`
	Recommendations   []string         `json:"recommendations"`
	ExperimentIDs     []string         `json:"experiment_ids"`
}

// Dashboard 仪表盘数据.
type Dashboard struct {
	TotalExperiments     int               `json:"total_experiments"`
	RunningExperiments   int               `json:"running_experiments"`
	CompletedExperiments int               `json:"completed_experiments"`
	FailedExperiments    int               `json:"failed_experiments"`
	OverallResilience    float64           `json:"overall_resilience"`
	RecentExperiments    []*Experiment     `json:"recent_experiments"`
	FaultDistribution    map[FaultType]int `json:"fault_distribution"`
	UpdatedAt            time.Time         `json:"updated_at"`
}

// Manager 混沌工程管理器.
type Manager struct {
	mu          sync.RWMutex
	experiments map[string]*Experiment
	reports     map[string]*ResilienceReport
	config      *Config
	running     map[string]context.CancelFunc // 运行中的实验取消函数
	ctx         context.Context
	cancel      context.CancelFunc
}

// Config 管理器配置.
type Config struct {
	Enabled         bool           `json:"enabled"`
	DefaultSafety   SafetyBoundary `json:"default_safety"`
	MaxConcurrent   int            `json:"max_concurrent"`
	MetricsInterval time.Duration  `json:"metrics_interval"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		Enabled: true,
		DefaultSafety: SafetyBoundary{
			MaxDuration:       30 * time.Minute,
			MaxCPUUsage:       80.0,
			MaxMemoryUsage:    80.0,
			MaxDiskUsage:      90.0,
			ProtectedPaths:    []string{"/", "/boot", "/etc", "/usr"},
			ProtectedServices: []string{"sshd", "systemd-journald"},
			AutoRecover:       true,
			RequireConfirm:    true,
		},
		MaxConcurrent:   3,
		MetricsInterval: 5 * time.Second,
	}
}

// ValidateFaultType 验证故障类型.
func ValidateFaultType(ft FaultType) error {
	switch ft {
	case FaultDiskFull, FaultNetworkLatency, FaultNetworkLoss,
		FaultCPUStress, FaultMemoryStress, FaultIOStress,
		FaultProcessKill, FaultDiskIO:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidFaultType, ft)
	}
}

// ValidateSeverity 验证严重程度.
func ValidateSeverity(s Severity) error {
	switch s {
	case SeverityLow, SeverityMedium, SeverityHigh, SeverityCritical:
		return nil
	default:
		return fmt.Errorf("%w: %s", ErrInvalidSeverity, s)
	}
}
