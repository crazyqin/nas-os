// Package systemhealthwizard 提供系统健康检查向导功能，引导用户逐步完成全面的系统诊断。
package systemhealthwizard

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrWizardRunning 表示向导正在运行中。
	ErrWizardRunning = errors.New("健康检查向导正在运行中")
	// ErrNoWizardSession 表示没有活跃的向导会话。
	ErrNoWizardSession = errors.New("没有活跃的向导会话")
	// ErrStepSkipped 表示该步骤已被跳过。
	ErrStepSkipped = errors.New("该步骤已被跳过")
)

// ========== 检查步骤 ==========

// CheckStep 检查步骤类型。
type CheckStep string

const (
	// StepDiskHealth 磁盘健康检查。
	StepDiskHealth CheckStep = "disk_health"
	// StepRAIDStatus RAID 状态检查。
	StepRAIDStatus CheckStep = "raid_status"
	// StepMemoryTest 内存测试。
	StepMemoryTest CheckStep = "memory_test"
	// StepCPUBurn CPU 压力测试。
	StepCPUBurn CheckStep = "cpu_burn"
	// StepNetworkConnectivity 网络连通性检查。
	StepNetworkConnectivity CheckStep = "network_connectivity"
	// StepDiskSpace 磁盘空间检查。
	StepDiskSpace CheckStep = "disk_space"
	// StepServiceStatus 服务状态检查。
	StepServiceStatus CheckStep = "service_status"
	// StepSecurityScan 安全扫描。
	StepSecurityScan CheckStep = "security_scan"
	// StepBackupIntegrity 备份完整性检查。
	StepBackupIntegrity CheckStep = "backup_integrity"
	// StepPerformanceBaseline 性能基线测试。
	StepPerformanceBaseline CheckStep = "performance_baseline"
)

// AllSteps 返回所有检查步骤。
func AllSteps() []CheckStep {
	return []CheckStep{
		StepDiskHealth,
		StepRAIDStatus,
		StepMemoryTest,
		StepCPUBurn,
		StepNetworkConnectivity,
		StepDiskSpace,
		StepServiceStatus,
		StepSecurityScan,
		StepBackupIntegrity,
		StepPerformanceBaseline,
	}
}

// StepInfo 步骤信息。
type StepInfo struct {
	Step        CheckStep `json:"step"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Critical    bool      `json:"critical"`
	AutoFix     bool      `json:"auto_fix"`
}

// DefaultSteps 返回默认步骤信息列表。
func DefaultSteps() []StepInfo {
	return []StepInfo{
		{Step: StepDiskHealth, Name: "磁盘健康", Description: "检查 S.M.A.R.T. 状态和磁盘错误", Critical: true, AutoFix: false},
		{Step: StepRAIDStatus, Name: "RAID 状态", Description: "检查 RAID 阵列完整性和降级状态", Critical: true, AutoFix: false},
		{Step: StepMemoryTest, Name: "内存测试", Description: "检测内存错误和 ECC 状态", Critical: true, AutoFix: false},
		{Step: StepCPUBurn, Name: "CPU 压力测试", Description: "短时 CPU 负载测试检测过热和稳定性", Critical: false, AutoFix: false},
		{Step: StepNetworkConnectivity, Name: "网络连通性", Description: "检查网络接口、DNS 和外部连通性", Critical: false, AutoFix: true},
		{Step: StepDiskSpace, Name: "磁盘空间", Description: "检查各分区使用率和大文件", Critical: false, AutoFix: true},
		{Step: StepServiceStatus, Name: "服务状态", Description: "检查核心服务运行状态", Critical: false, AutoFix: true},
		{Step: StepSecurityScan, Name: "安全扫描", Description: "检查开放端口、弱密码和安全配置", Critical: false, AutoFix: false},
		{Step: StepBackupIntegrity, Name: "备份完整性", Description: "验证最近备份的完整性和可恢复性", Critical: true, AutoFix: false},
		{Step: StepPerformanceBaseline, Name: "性能基线", Description: "磁盘 I/O、网络吞吐量基线测试", Critical: false, AutoFix: false},
	}
}

// ========== 检查结果 ==========

// ResultStatus 检查结果状态。
type ResultStatus string

const (
	// StatusPass 通过。
	StatusPass ResultStatus = "pass"
	// StatusWarn 警告。
	StatusWarn ResultStatus = "warn"
	// StatusFail 失败。
	StatusFail ResultStatus = "fail"
	// StatusSkipped 跳过。
	StatusSkipped ResultStatus = "skipped"
	// StatusRunning 运行中。
	StatusRunning ResultStatus = "running"
)

// StepResult 单步检查结果。
type StepResult struct {
	Step      CheckStep     `json:"step"`
	Status    ResultStatus  `json:"status"`
	Message   string        `json:"message"`
	Details   []string      `json:"details,omitempty"`
	FixAction string        `json:"fix_action,omitempty"`
	Duration  time.Duration `json:"duration"`
	Timestamp time.Time     `json:"timestamp"`
}

// WizardSession 向导会话。
type WizardSession struct {
	ID        string                    `json:"id"`
	Steps     []CheckStep               `json:"steps"`
	Results   map[CheckStep]*StepResult `json:"results"`
	Current   int                       `json:"current"`
	Status    string                    `json:"status"`
	StartedAt time.Time                 `json:"started_at"`
	EndedAt   *time.Time                `json:"ended_at,omitempty"`
	Score     float64                   `json:"score"`
}

// WizardReport 向导报告。
type WizardReport struct {
	SessionID       string        `json:"session_id"`
	TotalSteps      int           `json:"total_steps"`
	PassedSteps     int           `json:"passed_steps"`
	WarnSteps       int           `json:"warn_steps"`
	FailedSteps     int           `json:"failed_steps"`
	SkippedSteps    int           `json:"skipped_steps"`
	OverallScore    float64       `json:"overall_score"`
	Results         []*StepResult `json:"results"`
	Recommendations []string      `json:"recommendations"`
	StartedAt       time.Time     `json:"started_at"`
	EndedAt         time.Time     `json:"ended_at"`
	Duration        time.Duration `json:"duration"`
}
