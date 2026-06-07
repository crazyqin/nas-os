package sysdiagnostic

import (
	"time"
)

// DiagnosticStatus 诊断状态.
type DiagnosticStatus string

const (
	// DiagnosticStatusHealthy 健康.
	DiagnosticStatusHealthy DiagnosticStatus = "healthy"
	// DiagnosticStatusWarning 警告.
	DiagnosticStatusWarning DiagnosticStatus = "warning"
	// DiagnosticStatusCritical 严重.
	DiagnosticStatusCritical DiagnosticStatus = "critical"
	// DiagnosticStatusUnknown 未知.
	DiagnosticStatusUnknown DiagnosticStatus = "unknown"
)

// CheckCategory 检查类别.
type CheckCategory string

const (
	// CheckCategoryCPU CPU 检查.
	CheckCategoryCPU CheckCategory = "cpu"
	// CheckCategoryMemory 内存检查.
	CheckCategoryMemory CheckCategory = "memory"
	// CheckCategoryDisk 磁盘检查.
	CheckCategoryDisk CheckCategory = "disk"
	// CheckCategoryNetwork 网络检查.
	CheckCategoryNetwork CheckCategory = "network"
	// CheckCategoryService 服务检查.
	CheckCategoryService CheckCategory = "service"
	// CheckCategorySystem 系统检查.
	CheckCategorySystem CheckCategory = "system"
)

// IssueSeverity 问题严重程度.
type IssueSeverity string

const (
	// IssueSeverityLow 低.
	IssueSeverityLow IssueSeverity = "low"
	// IssueSeverityMedium 中.
	IssueSeverityMedium IssueSeverity = "medium"
	// IssueSeverityHigh 高.
	IssueSeverityHigh IssueSeverity = "high"
	// IssueSeverityCritical 严重.
	IssueSeverityCritical IssueSeverity = "critical"
)

// IssueStatus 问题状态.
type IssueStatus string

const (
	// IssueStatusOpen 打开.
	IssueStatusOpen IssueStatus = "open"
	// IssueStatusInProgress 处理中.
	IssueStatusInProgress IssueStatus = "in_progress"
	// IssueStatusResolved 已解决.
	IssueStatusResolved IssueStatus = "resolved"
	// IssueStatusIgnored 已忽略.
	IssueStatusIgnored IssueStatus = "ignored"
)

// DiagnosticReport 诊断报告.
type DiagnosticReport struct {
	ID          string           `json:"id"`
	Status      DiagnosticStatus `json:"status"`
	Score       int              `json:"score"` // 0-100
	GeneratedAt time.Time        `json:"generatedAt"`
	Duration    time.Duration    `json:"duration"`

	// 检查结果
	Checks []SystemCheck `json:"checks"`

	// 发现的问题
	Issues []Issue `json:"issues"`

	// 系统概览
	SystemOverview *SystemOverview `json:"systemOverview"`

	// 建议
	Recommendations []Recommendation `json:"recommendations,omitempty"`
}

// SystemCheck 系统检查.
type SystemCheck struct {
	ID       string        `json:"id"`
	Category CheckCategory `json:"category"`
	Name     string        `json:"name"`
	Status   string        `json:"status"` // pass/warn/fail
	Message  string        `json:"message"`
	Details  interface{}   `json:"details,omitempty"`
	Duration time.Duration `json:"duration"`
}

// SystemOverview 系统概览.
type SystemOverview struct {
	// CPU
	CPUUsage  float64 `json:"cpuUsage"`  // CPU 使用率 (%)
	CPUCores  int     `json:"cpuCores"`  // CPU 核心数
	CPUModel  string  `json:"cpuModel"`  // CPU 型号
	CPUTemp   float64 `json:"cpuTemp"`   // CPU 温度 (°C)
	LoadAvg1  float64 `json:"loadAvg1"`  // 1 分钟负载
	LoadAvg5  float64 `json:"loadAvg5"`  // 5 分钟负载
	LoadAvg15 float64 `json:"loadAvg15"` // 15 分钟负载

	// 内存
	MemoryTotal    int64   `json:"memoryTotal"`    // 总内存 (bytes)
	MemoryUsed     int64   `json:"memoryUsed"`     // 已用内存 (bytes)
	MemoryFree     int64   `json:"memoryFree"`     // 空闲内存 (bytes)
	MemoryUsagePct float64 `json:"memoryUsagePct"` // 内存使用率 (%)
	SwapTotal      int64   `json:"swapTotal"`      // 总 Swap (bytes)
	SwapUsed       int64   `json:"swapUsed"`       // 已用 Swap (bytes)

	// 磁盘
	Disks []DiskOverview `json:"disks"`

	// 网络
	NetworkInterfaces []NetworkInterface `json:"networkInterfaces"`
	NetworkIO         *NetworkIO         `json:"networkIO"`

	// 服务
	Services []ServiceStatus `json:"services"`

	// 系统信息
	Hostname    string    `json:"hostname"`
	OS          string    `json:"os"`
	Kernel      string    `json:"kernel"`
	Uptime      int64     `json:"uptime"`      // 秒
	UptimeHuman string    `json:"uptimeHuman"` // 可读格式
	BootTime    time.Time `json:"bootTime"`
}

// DiskOverview 磁盘概览.
type DiskOverview struct {
	Device     string  `json:"device"`
	MountPoint string  `json:"mountPoint"`
	FileSystem string  `json:"fileSystem"`
	Total      int64   `json:"total"`
	Used       int64   `json:"used"`
	Available  int64   `json:"available"`
	UsagePct   float64 `json:"usagePct"`
	Health     string  `json:"health"`
}

// NetworkInterface 网络接口.
type NetworkInterface struct {
	Name      string   `json:"name"`
	IP        []string `json:"ip"`
	MAC       string   `json:"mac"`
	Speed     string   `json:"speed"`  // 1Gbps, 10Gbps 等
	Status    string   `json:"status"` // up/down
	MTU       int      `json:"mtu"`
	RxBytes   int64    `json:"rxBytes"`
	TxBytes   int64    `json:"txBytes"`
	RxPackets int64    `json:"rxPackets"`
	TxPackets int64    `json:"txPackets"`
	Errors    int64    `json:"errors"`
	Drops     int64    `json:"drops"`
}

// NetworkIO 网络 IO.
type NetworkIO struct {
	TotalRxBytes   int64 `json:"totalRxBytes"`
	TotalTxBytes   int64 `json:"totalTxBytes"`
	TotalRxPackets int64 `json:"totalRxPackets"`
	TotalTxPackets int64 `json:"totalTxPackets"`
	RxRate         int64 `json:"rxRate"` // bytes/sec
	TxRate         int64 `json:"txRate"` // bytes/sec
}

// ServiceStatus 服务状态.
type ServiceStatus struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"` // running/stopped/error
	PID       int       `json:"pid,omitempty"`
	Uptime    string    `json:"uptime,omitempty"`
	CPU       float64   `json:"cpu,omitempty"`    // CPU 使用率 (%)
	Memory    int64     `json:"memory,omitempty"` // 内存使用 (bytes)
	StartedAt time.Time `json:"startedAt,omitempty"`
}

// Issue 问题.
type Issue struct {
	ID          string        `json:"id"`
	Category    CheckCategory `json:"category"`
	Severity    IssueSeverity `json:"severity"`
	Status      IssueStatus   `json:"status"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Impact      string        `json:"impact"`
	RootCause   string        `json:"rootCause"`
	DetectedAt  time.Time     `json:"detectedAt"`
	ResolvedAt  time.Time     `json:"resolvedAt,omitempty"`

	// 修复指南
	RepairGuide *RepairGuide `json:"repairGuide,omitempty"`

	// 相关检查
	CheckIDs []string `json:"checkIds,omitempty"`
}

// RepairGuide 修复指南.
type RepairGuide struct {
	ID            string       `json:"id"`
	IssueID       string       `json:"issueId"`
	Title         string       `json:"title"`
	Description   string       `json:"description"`
	Difficulty    string       `json:"difficulty"` // easy/medium/hard
	EstimatedTime string       `json:"estimatedTime"`
	Prerequisites []string     `json:"prerequisites,omitempty"`
	Steps         []RepairStep `json:"steps"`
	Warnings      []string     `json:"warnings,omitempty"`
	References    []string     `json:"references,omitempty"`
}

// RepairStep 修复步骤.
type RepairStep struct {
	StepNumber  int    `json:"stepNumber"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	Expected    string `json:"expected,omitempty"`
	Notes       string `json:"notes,omitempty"`
}

// Recommendation 建议.
type Recommendation struct {
	ID          string        `json:"id"`
	Category    CheckCategory `json:"category"`
	Priority    int           `json:"priority"` // 1-5
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Action      string        `json:"action"`
	Impact      string        `json:"impact"`
}

// DiagnosticTrend 诊断趋势.
type DiagnosticTrend struct {
	Timestamp time.Time        `json:"timestamp"`
	Score     int              `json:"score"`
	Status    DiagnosticStatus `json:"status"`
	Issues    int              `json:"issues"`
	CPUUsage  float64          `json:"cpuUsage"`
	MemUsage  float64          `json:"memUsage"`
	DiskUsage float64          `json:"diskUsage"`
}

// QuickHealthCheckResult 快速健康检查结果.
type QuickHealthCheckResult struct {
	Status    DiagnosticStatus `json:"status"`
	Score     int              `json:"score"`
	CheckedAt time.Time        `json:"checkedAt"`
	Duration  time.Duration    `json:"duration"`

	// 快速概览
	CPUStatus     string `json:"cpuStatus"`
	MemoryStatus  string `json:"memoryStatus"`
	DiskStatus    string `json:"diskStatus"`
	NetworkStatus string `json:"networkStatus"`
	ServiceStatus string `json:"serviceStatus"`

	// 摘要
	TotalIssues    int `json:"totalIssues"`
	CriticalIssues int `json:"criticalIssues"`
	WarningIssues  int `json:"warningIssues"`
}

// DiagnosticRequest 诊断请求.
type DiagnosticRequest struct {
	Categories     []CheckCategory `json:"categories,omitempty"` // 为空则检查所有
	IncludeDetails bool            `json:"includeDetails"`
	Force          bool            `json:"force"`
}

// Response API 响应.
type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ErrorResponse 错误响应.
type ErrorResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}
