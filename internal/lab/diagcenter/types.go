// Package diagcenter 提供智能诊断中心功能。
// 整合系统告警、自愈建议和故障排查指导，类似 TrueNAS 的 Guided Alerts。
package diagcenter

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrNoDiagData 尚未执行过诊断.
	ErrNoDiagData = errors.New("尚未执行诊断，请先调用 POST /api/v1/diag/run")
	// ErrDiagInProgress 诊断任务正在进行中.
	ErrDiagInProgress = errors.New("诊断任务正在执行中，请稍后再试")
)

// ========== 诊断严重级别 ==========

// Severity 严重级别.
type Severity string

const (
	SeverityInfo     Severity = "info"     // 信息
	SeverityWarning  Severity = "warning"  // 警告
	SeverityCritical Severity = "critical" // 严重
	SeverityFatal    Severity = "fatal"    // 致命
)

// ========== 诊断状态 ==========

// DiagStatus 诊断状态.
type DiagStatus string

const (
	StatusHealthy  DiagStatus = "healthy"  // 健康
	StatusDegraded DiagStatus = "degraded" // 降级
	StatusCritical DiagStatus = "critical" // 严重故障
	StatusFatal    DiagStatus = "fatal"    // 致命故障
)

// ClassifyStatus 根据严重级别返回状态.
func ClassifyStatus(severity Severity) DiagStatus {
	switch severity {
	case SeverityFatal:
		return StatusFatal
	case SeverityCritical:
		return StatusCritical
	case SeverityWarning:
		return StatusDegraded
	default:
		return StatusHealthy
	}
}

// ========== 检查项 ==========

// CheckCategory 检查类别.
type CheckCategory string

const (
	CategoryDisk    CheckCategory = "disk"    // 磁盘检查
	CategoryMemory  CheckCategory = "memory"  // 内存检查
	CategoryCPU     CheckCategory = "cpu"     // CPU 检查
	CategoryService CheckCategory = "service" // 服务检查
	CategoryNetwork CheckCategory = "network" // 网络检查
	CategoryRAID    CheckCategory = "raid"    // RAID 检查
)

// CheckItem 单项检查结果.
type CheckItem struct {
	Category    CheckCategory `json:"category"`    // 检查类别
	Name        string        `json:"name"`        // 检查名称
	Status      DiagStatus    `json:"status"`      // 检查状态
	Severity    Severity      `json:"severity"`    // 严重级别
	Message     string        `json:"message"`     // 检查结果消息
	Value       interface{}   `json:"value"`       // 当前值
	Threshold   interface{}   `json:"threshold"`   // 阈值
	Remediation *Remediation  `json:"remediation"` // 修复建议
}

// ========== 修复建议 ==========

// Remediation 引导式修复建议.
type Remediation struct {
	Title       string   `json:"title"`       // 建议标题
	Description string   `json:"description"` // 详细描述
	Steps       []string `json:"steps"`       // 操作步骤
	QuickFix    string   `json:"quick_fix"`   // 快速修复链接/命令
	DocURL      string   `json:"doc_url"`     // 文档链接
}

// ========== 诊断结果 ==========

// DiagResult 诊断结果.
type DiagResult struct {
	ID        string        `json:"id"`        // 诊断 ID
	Timestamp time.Time     `json:"timestamp"` // 诊断时间
	Status    DiagStatus    `json:"status"`    // 整体状态
	Checks    []CheckItem   `json:"checks"`    // 检查项列表
	Alerts    []Alert       `json:"alerts"`    // 告警列表
	Summary   string        `json:"summary"`   // 诊断摘要
	Duration  time.Duration `json:"duration"`  // 诊断耗时
}

// ========== 告警 ==========

// Alert 系统告警.
type Alert struct {
	ID           string       `json:"id"`           // 告警 ID
	Category     string       `json:"category"`     // 所属类别
	Severity     Severity     `json:"severity"`     // 严重级别
	Title        string       `json:"title"`        // 告警标题
	Description  string       `json:"description"`  // 详细描述
	Timestamp    time.Time    `json:"timestamp"`    // 告警时间
	Remediation  *Remediation `json:"remediation"`  // 修复建议
	Acknowledged bool         `json:"acknowledged"` // 是否已确认
}

// ========== 根因分析 ==========

// RootCause 根因分析.
type RootCause struct {
	Symptom           string   `json:"symptom"`            // 症状描述
	PossibleCauses    []string `json:"possible_causes"`    // 可能原因
	RecommendedAction string   `json:"recommended_action"` // 推荐操作
	Confidence        float64  `json:"confidence"`         // 置信度 0-1
}

// ========== 诊断历史 ==========

// HistoryQuery 历史查询参数.
type HistoryQuery struct {
	Days  int `form:"days"`  // 查询最近N天，默认30
	Limit int `form:"limit"` // 最大条数，默认100
}

// HistoryResponse 历史响应.
type HistoryResponse struct {
	Results    []DiagResult `json:"results"`
	TotalCount int          `json:"total_count"`
}

// ========== 诊断配置 ==========

// Config 诊断中心配置.
type Config struct {
	// 磁盘检查
	DiskWarnTempC  int `json:"disk_warn_temp_c"` // 磁盘温度警告阈值，默认55
	DiskCritTempC  int `json:"disk_crit_temp_c"` // 磁盘温度严重阈值，默认65
	DiskMaxRealloc int `json:"disk_max_realloc"` // 最大重分配扇区数，默认100

	// 内存检查
	MemWarnPercent float64 `json:"mem_warn_percent"` // 内存警告阈值，默认80
	MemCritPercent float64 `json:"mem_crit_percent"` // 内存严重阈值，默认95

	// CPU 检查
	CPUWarnPercent float64 `json:"cpu_warn_percent"` // CPU 警告阈值，默认80
	CPUCritPercent float64 `json:"cpu_crit_percent"` // CPU 严重阈值，默认95

	// 网络检查
	NetworkTargets []string `json:"network_targets"` // 网络检查目标

	// 服务检查
	RequiredServices []string `json:"required_services"` // 必须运行的服务
}

// DefaultConfig 默认配置.
func DefaultConfig() *Config {
	return &Config{
		DiskWarnTempC:  55,
		DiskCritTempC:  65,
		DiskMaxRealloc: 100,
		MemWarnPercent: 80,
		MemCritPercent: 95,
		CPUWarnPercent: 80,
		CPUCritPercent: 95,
		NetworkTargets: []string{
			"8.8.8.8",         // Google DNS
			"114.114.114.114", // 国内 DNS
		},
		RequiredServices: []string{
			"docker",
			"smbd",
			"nginx",
		},
	}
}

// ========== API 请求/响应 ==========

// RunDiagRequest 运行诊断请求.
type RunDiagRequest struct {
	Categories []CheckCategory `json:"categories"` // 指定检查类别，空则全部检查
}

// RunDiagResponse 运行诊断响应.
type RunDiagResponse struct {
	Result  *DiagResult `json:"result"`
	Message string      `json:"message"`
}
