package drdrill

import (
	"time"
)

// ─────────────────────── 演练类型 ───────────────────────

// DrillType 演练类型.
type DrillType string

const (
	DrillTypeDiskFault    DrillType = "disk_fault"    // 磁盘故障模拟
	DrillTypeNetworkDown  DrillType = "network_down"  // 网络中断模拟
	DrillTypePoolDegrade  DrillType = "pool_degrade"  // 存储池降级
	DrillTypeServiceDown  DrillType = "service_down"  // 服务宕机恢复
	DrillTypeDataRecovery DrillType = "data_recovery" // 数据恢复验证
)

// ─────────────────────── 演练范围 ───────────────────────

// DrillScope 演练范围.
type DrillScope string

const (
	ScopeSystem  DrillScope = "system"  // 全系统
	ScopePool    DrillScope = "pool"    // 指定存储池
	ScopeService DrillScope = "service" // 指定服务
)

// ─────────────────────── 演练模式 ───────────────────────

// DrillMode 演练模式.
type DrillMode string

const (
	ModeDryRun DrillMode = "dry_run" // 模拟模式（不实际执行破坏操作）
	ModeReal   DrillMode = "real"    // 实战模式
)

// ─────────────────────── 调度频率 ───────────────────────

// ScheduleFrequency 调度频率.
type ScheduleFrequency string

const (
	FreqMonthly   ScheduleFrequency = "monthly"   // 月度
	FreqQuarterly ScheduleFrequency = "quarterly" // 季度
)

// ─────────────────────── 步骤状态 ───────────────────────

// StepStatus 步骤执行状态.
type StepStatus string

const (
	StepPending    StepStatus = "pending"
	StepRunning    StepStatus = "running"
	StepSuccess    StepStatus = "success"
	StepFailed     StepStatus = "failed"
	StepSkipped    StepStatus = "skipped"
	StepRolledBack StepStatus = "rolled_back"
)

// ─────────────────────── 执行状态 ───────────────────────

// ExecutionStatus 演练执行状态.
type ExecutionStatus string

const (
	ExecPending ExecutionStatus = "pending"
	ExecRunning ExecutionStatus = "running"
	ExecSuccess ExecutionStatus = "success"
	ExecFailed  ExecutionStatus = "failed"
	ExecAborted ExecutionStatus = "aborted"
)

// ─────────────────────── 核心类型 ───────────────────────

// StepDef 演练步骤定义.
type StepDef struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Timeout     time.Duration `json:"timeout"`
	MaxRetries  int           `json:"max_retries"`
	Rollback    string        `json:"rollback,omitempty"` // 回滚动作描述
}

// DrillPlan 容灾演练计划.
type DrillPlan struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	Type        DrillType       `json:"type"`
	Scope       DrillScope      `json:"scope"`
	ScopeTarget string          `json:"scope_target,omitempty"` // 存储池名称或服务名称
	Mode        DrillMode       `json:"mode"`
	Steps       []StepDef       `json:"steps"`
	Schedule    *ScheduleConfig `json:"schedule,omitempty"`
	Enabled     bool            `json:"enabled"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}

// ScheduleConfig 调度配置.
type ScheduleConfig struct {
	Frequency      ScheduleFrequency `json:"frequency"`
	DayOfWeek      int               `json:"day_of_week,omitempty"`      // 0=Sunday
	DayOfMonth     int               `json:"day_of_month,omitempty"`     // 1-31
	MonthOfQuarter int               `json:"month_of_quarter,omitempty"` // 1-3
	Hour           int               `json:"hour"`                       // 0-23
	Minute         int               `json:"minute"`                     // 0-59
	ReminderDays   int               `json:"reminder_days,omitempty"`    // 提前几天提醒
}

// StepResult 单步执行结果.
type StepResult struct {
	Name       string        `json:"name"`
	Status     StepStatus    `json:"status"`
	StartTime  time.Time     `json:"start_time"`
	EndTime    time.Time     `json:"end_time"`
	Duration   time.Duration `json:"duration"`
	Retried    int           `json:"retried"`
	Error      string        `json:"error,omitempty"`
	RolledBack bool          `json:"rolled_back"`
}

// DrillExecution 演练执行记录.
type DrillExecution struct {
	ID            string          `json:"id"`
	PlanID        string          `json:"plan_id"`
	PlanName      string          `json:"plan_name"`
	Mode          DrillMode       `json:"mode"`
	Status        ExecutionStatus `json:"status"`
	SnapshotID    string          `json:"snapshot_id,omitempty"` // 保护点快照ID
	StartTime     time.Time       `json:"start_time"`
	EndTime       time.Time       `json:"end_time"`
	TotalDuration time.Duration   `json:"total_duration"`
	StepResults   []StepResult    `json:"step_results"`
	ErrorMessage  string          `json:"error_message,omitempty"`
}

// DrillReport 演练报告.
type DrillReport struct {
	ExecutionID   string          `json:"execution_id"`
	PlanID        string          `json:"plan_id"`
	PlanName      string          `json:"plan_name"`
	Mode          DrillMode       `json:"mode"`
	Status        ExecutionStatus `json:"status"`
	StartTime     time.Time       `json:"start_time"`
	EndTime       time.Time       `json:"end_time"`
	TotalDuration time.Duration   `json:"total_duration"`
	StepResults   []StepResult    `json:"step_results"`
	RTOActual     time.Duration   `json:"rto_actual"`           // RTO 实测值
	RPOActual     time.Duration   `json:"rpo_actual"`           // RPO 实测值
	RTOTarget     time.Duration   `json:"rto_target,omitempty"` // RTO 目标值
	RPOTarget     time.Duration   `json:"rpo_target,omitempty"` // RPO 目标值
	Issues        []string        `json:"issues"`               // 发现的问题
	Suggestions   []string        `json:"suggestions"`          // 改进建议
	Trend         *TrendData      `json:"trend,omitempty"`      // 历史对比趋势
}

// TrendData 历史趋势数据.
type TrendData struct {
	TotalDrills     int           `json:"total_drills"`
	SuccessRate     float64       `json:"success_rate"`
	AvgRTO          time.Duration `json:"avg_rto"`
	AvgRPO          time.Duration `json:"avg_rpo"`
	BestRTO         time.Duration `json:"best_rto"`
	WorstRTO        time.Duration `json:"worst_rto"`
	BestRPO         time.Duration `json:"best_rpo"`
	WorstRPO        time.Duration `json:"worst_rpo"`
	ImprovementRate float64       `json:"improvement_rate"` // 相比上次改善百分比
}

// DRMetrics RTO/RPO 指标统计.
type DRMetrics struct {
	TotalPlans    int           `json:"total_plans"`
	TotalExecs    int           `json:"total_execs"`
	SuccessRate   float64       `json:"success_rate"`
	AvgRTO        time.Duration `json:"avg_rto"`
	AvgRPO        time.Duration `json:"avg_rpo"`
	BestRTO       time.Duration `json:"best_rto"`
	WorstRTO      time.Duration `json:"worst_rto"`
	BestRPO       time.Duration `json:"best_rpo"`
	WorstRPO      time.Duration `json:"worst_rpo"`
	LastDrillTime time.Time     `json:"last_drill_time,omitempty"`
}

// ─────────────────────── API 请求/响应 ───────────────────────

// CreatePlanRequest 创建演练计划请求.
type CreatePlanRequest struct {
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description"`
	Type        string          `json:"type" binding:"required"`
	Scope       string          `json:"scope" binding:"required"`
	ScopeTarget string          `json:"scope_target"`
	Mode        string          `json:"mode" binding:"required"`
	Steps       []StepDef       `json:"steps" binding:"required,min=1"`
	Schedule    *ScheduleConfig `json:"schedule"`
}

// APIResponse 通用API响应.
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func successResp(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

func errResp(code int, msg string) APIResponse {
	return APIResponse{Code: code, Message: msg}
}
