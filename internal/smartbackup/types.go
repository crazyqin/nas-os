package smartbackup

import (
	"time"
)

// BackupType 备份类型
type BackupType string

const (
	BackupTypeFull         BackupType = "full"         // 全量备份
	BackupTypeIncremental  BackupType = "incremental"  // 增量备份
	BackupTypeDifferential BackupType = "differential" // 差异备份
)

// BackupStatus 备份状态
type BackupStatus string

const (
	BackupStatusPending  BackupStatus = "pending"
	BackupStatusRunning  BackupStatus = "running"
	BackupStatusSuccess  BackupStatus = "success"
	BackupStatusFailed   BackupStatus = "failed"
	BackupStatusExpired  BackupStatus = "expired"
)

// Priority 数据优先级
type Priority string

const (
	PriorityLow      Priority = "low"
	PriorityMedium   Priority = "medium"
	PriorityHigh     Priority = "high"
	PriorityCritical Priority = "critical"
)

// RPOConfig RPO 配置
type RPOConfig struct {
	MaxDataLoss   time.Duration `json:"max_data_loss"`   // 最大可接受数据丢失时间
	CheckInterval time.Duration `json:"check_interval"`  // 检查间隔
}

// RTOConfig RTO 配置
type RTOConfig struct {
	MaxRecoveryTime time.Duration `json:"max_recovery_time"` // 最大可接受恢复时间
	AutoFailover    bool          `json:"auto_failover"`     // 是否自动故障转移
}

// ChangeFrequency 数据变化频率
type ChangeFrequency struct {
	DailyChanges   int     `json:"daily_changes"`   // 每日变化量(MB)
	WeeklyChanges  int     `json:"weekly_changes"`  // 每周变化量(MB)
	MonthlyChanges int     `json:"monthly_changes"` // 每月变化量(MB)
	ChangeRate     float64 `json:"change_rate"`     // 变化率(0-1)
}

// BackupPolicy 备份策略
type BackupPolicy struct {
	ID              string            `json:"id"`
	Name            string            `json:"name"`
	Description     string            `json:"description,omitempty"`
	BackupType      BackupType        `json:"backup_type"`
	SourcePaths     []string          `json:"source_paths"`
	TargetIDs       []string          `json:"target_ids"`
	Schedule        *Schedule         `json:"schedule,omitempty"`
	Retention       *RetentionPolicy  `json:"retention,omitempty"`
	RPO             *RPOConfig        `json:"rpo,omitempty"`
	RTO             *RTOConfig        `json:"rto,omitempty"`
	Priority        Priority          `json:"priority"`
	Enabled         bool              `json:"enabled"`
	Tags            map[string]string `json:"tags,omitempty"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

// Schedule 调度配置
type Schedule struct {
	CronExpr    string    `json:"cron_expr"`
	Interval    string    `json:"interval,omitempty"`
	StartTime   time.Time `json:"start_time,omitempty"`
	EndTime     time.Time `json:"end_time,omitempty"`
	MaxDuration string    `json:"max_duration,omitempty"`
}

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	KeepLast    int `json:"keep_last"`
	KeepDaily   int `json:"keep_daily"`
	KeepWeekly  int `json:"keep_weekly"`
	KeepMonthly int `json:"keep_monthly"`
	KeepYearly  int `json:"keep_yearly"`
}

// BackupStrategy 备份策略推荐
type BackupStrategy struct {
	RecommendedType BackupType `json:"recommended_type"`
	Reason          string     `json:"reason"`
	Confidence      float64    `json:"confidence"` // 0-1
	EstimatedSize   float64    `json:"estimated_size_mb"`
	EstimatedTime   float64    `json:"estimated_time_minutes"`
	RPOFeasible     bool       `json:"rpo_feasible"`
	RTOFeasible     bool       `json:"rto_feasible"`
	Recommendations []string   `json:"recommendations"`
}

// BackupExecution 备份执行记录
type BackupExecution struct {
	ID           string       `json:"id"`
	PolicyID     string       `json:"policy_id"`
	BackupType   BackupType   `json:"backup_type"`
	Status       BackupStatus `json:"status"`
	StartTime    time.Time    `json:"start_time"`
	EndTime      time.Time    `json:"end_time,omitempty"`
	Duration     string       `json:"duration,omitempty"`
	SizeBytes    int64        `json:"size_bytes"`
	FilesCount   int          `json:"files_count"`
	ErrorMessage string       `json:"error_message,omitempty"`
	TargetID     string       `json:"target_id"`
	Checksum     string       `json:"checksum,omitempty"`
}

// StrategyAnalysis 策略分析请求
type StrategyAnalysis struct {
	DataType        string          `json:"data_type"`
	DataSizeGB      float64         `json:"data_size_gb"`
	ChangeFrequency *ChangeFrequency `json:"change_frequency,omitempty"`
	CurrentBackup   *BackupPolicy   `json:"current_backup,omitempty"`
	Requirements    *RPOConfig      `json:"requirements,omitempty"`
	RTORequirements *RTOConfig      `json:"rto_requirements,omitempty"`
}

// WindowOptimization 备份窗口优化结果
type WindowOptimization struct {
	RecommendedStart int      `json:"recommended_start_hour"` // 推荐开始时间(小时)
	RecommendedEnd   int      `json:"recommended_end_hour"`   // 推荐结束时间(小时)
	Reason           string   `json:"reason"`
	PeakHours        []int    `json:"peak_hours"`
	OffPeakHours     []int    `json:"off_peak_hours"`
	Suggestions      []string `json:"suggestions"`
}

// PolicyEvaluation 策略评估结果
type PolicyEvaluation struct {
	PolicyID         string   `json:"policy_id"`
	RPOCompliance    bool     `json:"rpo_compliance"`
	RTOCompliance    bool     `json:"rto_compliance"`
	SuccessRate      float64  `json:"success_rate"`
	AvgDuration      string   `json:"avg_duration"`
	TotalExecutions  int      `json:"total_executions"`
	FailedExecutions int      `json:"failed_executions"`
	Recommendations  []string `json:"recommendations"`
	Score            float64  `json:"score"` // 0-100
}
