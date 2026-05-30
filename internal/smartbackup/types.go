package smartbackup

import (
	"fmt"
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
	BackupStatusCanceled BackupStatus = "canceled"
)

// TargetType 存储目标类型
type TargetType string

const (
	TargetTypeLocal    TargetType = "local"     // 本地存储
	TargetTypeRemoteNAS TargetType = "remote_nas" // 远程NAS
	TargetTypeS3       TargetType = "s3"        // S3云存储
)

// BackupTarget 备份目标
type BackupTarget struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        TargetType `json:"type"`
	Endpoint    string     `json:"endpoint,omitempty"`    // S3 endpoint 或 NAS 地址
	Bucket      string     `json:"bucket,omitempty"`      // S3 bucket 或 NAS 共享路径
	AccessKey   string     `json:"access_key,omitempty"`  // S3 access key
	SecretKey   string     `json:"secret_key,omitempty"`  // S3 secret key
	LocalPath   string     `json:"local_path,omitempty"`  // 本地路径
	Region      string     `json:"region,omitempty"`      // S3 region
	Status      string     `json:"status"`
	TotalSpace  int64      `json:"total_space"`  // 总空间(字节)
	UsedSpace   int64      `json:"used_space"`   // 已用空间(字节)
	CreatedAt   time.Time  `json:"created_at"`
	UpdatedAt   time.Time  `json:"updated_at"`
}

// BackupPolicy 备份策略（3-2-1规则支持）
type BackupPolicy struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	SourcePaths []string     `json:"source_paths"`
	TargetIDs   []string     `json:"target_ids"`
	BackupType  BackupType   `json:"backup_type"`
	Schedule    string       `json:"schedule"`
	RPO         *RPORequirements `json:"rpo,omitempty"`
	RTO         *RTORequirements `json:"rto,omitempty"`
	Retention   *RetentionPolicy `json:"retention,omitempty"`
	Status      BackupStatus `json:"status"`
	HealthScore float64      `json:"health_score"` // 健康评分 0-100
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// RetentionPolicy 保留策略
type RetentionPolicy struct {
	DailyKeep   int `json:"daily_keep"`   // 保留最近N天的每日备份
	WeeklyKeep  int `json:"weekly_keep"`  // 保留最近N周的每周备份
	MonthlyKeep int `json:"monthly_keep"` // 保留最近N月的每月备份
	MaxVersions int `json:"max_versions"` // 最大版本数
}

// BackupExecution 备份执行记录
type BackupExecution struct {
	ID          string       `json:"id"`
	PolicyID    string       `json:"policy_id"`
	BackupType  BackupType   `json:"backup_type"`
	TargetID    string       `json:"target_id"`
	Status      BackupStatus `json:"status"`
	StartTime   time.Time    `json:"start_time"`
	EndTime     time.Time    `json:"end_time"`
	SizeBytes   int64        `json:"size_bytes"`
	ChainID     string       `json:"chain_id,omitempty"`     // 备份链路ID
	ParentID    string       `json:"parent_id,omitempty"`    // 父备份ID（增量/差异）
	FilesTotal  int          `json:"files_total"`
	FilesCopied int          `json:"files_copied"`
	FilesFailed int          `json:"files_failed"`
	Error       string       `json:"error,omitempty"`
}

// BackupChain 备份链路
type BackupChain struct {
	ID          string           `json:"id"`
	PolicyID    string           `json:"policy_id"`
	TargetID    string           `json:"target_id"`
	FullBackup  *BackupExecution `json:"full_backup"`
	Incremental []*BackupExecution `json:"incremental"`
	TotalSize   int64            `json:"total_size"`
	ChainLength int              `json:"chain_length"`
	HealthScore float64          `json:"health_score"` // 链路健康评分
	CreatedAt   time.Time        `json:"created_at"`
	UpdatedAt   time.Time        `json:"updated_at"`
}

// VerificationResult 验证结果
type VerificationResult struct {
	ID          string     `json:"id"`
	ExecutionID string     `json:"execution_id"`
	Status      string     `json:"status"` // passed, failed, partial
	CheckedAt   time.Time  `json:"checked_at"`
	FilesChecked int       `json:"files_checked"`
	FilesPassed  int       `json:"files_passed"`
	FilesFailed  int       `json:"files_failed"`
	Errors       []string  `json:"errors,omitempty"`
	RecoveryTest *RecoveryTestResult `json:"recovery_test,omitempty"`
}

// RecoveryTestResult 恢复测试结果
type RecoveryTestResult struct {
	ID          string    `json:"id"`
	Status      string    `json:"status"` // passed, failed
	TestPath    string    `json:"test_path"`
	RestoreTime float64   `json:"restore_time_seconds"`
	TestedAt    time.Time `json:"tested_at"`
	Error       string    `json:"error,omitempty"`
}

// LoadMetrics 系统负载指标
type LoadMetrics struct {
	CPUPercent    float64   `json:"cpu_percent"`
	MemoryPercent float64   `json:"memory_percent"`
	DiskIOPercent float64   `json:"disk_io_percent"`
	NetworkIO     int64     `json:"network_io_bytes"`
	Timestamp     time.Time `json:"timestamp"`
}

// ScheduleOptimization 调度优化建议
type ScheduleOptimization struct {
	RecommendedTime  time.Time `json:"recommended_time"`
	LoadScore        float64   `json:"load_score"`   // 负载评分 0-100
	Reason           string    `json:"reason"`
	WaitMinutes      int       `json:"wait_minutes"` // 建议等待分钟数
}

// RPORequirements RPO 要求
type RPORequirements struct {
	MaxDataLoss time.Duration `json:"max_data_loss"`
}

// RTORequirements RTO 要求
type RTORequirements struct {
	MaxRecoveryTime time.Duration `json:"max_recovery_time"`
}

// StrategyAnalysis 策略分析
type StrategyAnalysis struct {
	DataType        string           `json:"data_type"`
	DataSizeGB      float64          `json:"data_size_gb"`
	ChangeFrequency *ChangeFrequency `json:"change_frequency,omitempty"`
	Requirements    *RPORequirements `json:"requirements,omitempty"`
	RTORequirements *RTORequirements `json:"rto_requirements,omitempty"`
	TargetCount     int              `json:"target_count"` // 目标数量（3-2-1规则）
}

// ChangeFrequency 数据变化频率
type ChangeFrequency struct {
	ChangeRate   float64 `json:"change_rate"`
	DailyChanges int     `json:"daily_changes"`
}

// BackupStrategy 备份策略推荐
type BackupStrategy struct {
	RecommendedType BackupType `json:"recommended_type"`
	Reason          string     `json:"reason"`
	EstimatedSize   float64    `json:"estimated_size"`
	EstimatedTime   float64    `json:"estimated_time"`
	RPOFeasible     bool       `json:"rpo_feasible"`
	RTOFeasible     bool       `json:"rto_feasible"`
	ThreeTwoOne     *ThreeTwoOneCompliance `json:"three_two_one,omitempty"`
	Recommendations []string   `json:"recommendations"`
	Confidence      float64    `json:"confidence"`
}

// ThreeTwoOneCompliance 3-2-1规则合规性
type ThreeTwoOneCompliance struct {
	TotalCopies   int  `json:"total_copies"`    // 总副本数（目标3）
	MediaTypes    int  `json:"media_types"`     // 介质类型数（目标2）
	OffsiteCopies int  `json:"offsite_copies"`  // 异地副本数（目标1）
	Compliant     bool `json:"compliant"`       // 是否合规
}

// BackupStats 备份统计
type BackupStats struct {
	TotalPolicies    int     `json:"total_policies"`
	ActivePolicies   int     `json:"active_policies"`
	TotalExecutions  int     `json:"total_executions"`
	SuccessfulBackups int    `json:"successful_backups"`
	FailedBackups    int     `json:"failed_backups"`
	TotalSizeBytes   int64   `json:"total_size_bytes"`
	AvgHealthScore   float64 `json:"avg_health_score"`
	ComplianceRate   float64 `json:"compliance_rate"` // 3-2-1合规率
}

// WindowOptimization 备份窗口优化
type WindowOptimization struct {
	RecommendedStart int      `json:"recommended_start"`
	RecommendedEnd   int      `json:"recommended_end"`
	PeakHours        []int    `json:"peak_hours"`
	OffPeakHours     []int    `json:"off_peak_hours"`
	Reason           string   `json:"reason"`
	Suggestions      []string `json:"suggestions"`
}

// PolicyEvaluation 策略评估
type PolicyEvaluation struct {
	PolicyID         string   `json:"policy_id"`
	TotalExecutions  int      `json:"total_executions"`
	FailedExecutions int      `json:"failed_executions"`
	SuccessRate      float64  `json:"success_rate"`
	AvgDuration      string   `json:"avg_duration"`
	RPOCompliance    bool     `json:"rpo_compliance"`
	RTOCompliance    bool     `json:"rto_compliance"`
	Score            float64  `json:"score"`
	Recommendations  []string `json:"recommendations"`
}

// NewBackupPolicy 创建新的备份策略
func NewBackupPolicy(name string, sourcePaths []string, targetIDs []string) *BackupPolicy {
	now := time.Now()
	return &BackupPolicy{
		ID:          fmt.Sprintf("policy-%d", now.UnixNano()),
		Name:        name,
		SourcePaths: sourcePaths,
		TargetIDs:   targetIDs,
		BackupType:  BackupTypeFull,
		Status:      BackupStatusPending,
		HealthScore: 100,
		Retention: &RetentionPolicy{
			DailyKeep:   7,
			WeeklyKeep:  4,
			MonthlyKeep: 12,
			MaxVersions: 100,
		},
		CreatedAt: now,
		UpdatedAt: now,
	}
}

// NewBackupTarget 创建新的备份目标
func NewBackupTarget(name string, targetType TargetType) *BackupTarget {
	now := time.Now()
	return &BackupTarget{
		ID:        fmt.Sprintf("target-%d", now.UnixNano()),
		Name:      name,
		Type:      targetType,
		Status:    "active",
		CreatedAt: now,
		UpdatedAt: now,
	}
}
