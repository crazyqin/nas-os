// Package sysdiag 提供系统诊断功能
package sysdiag

import (
	"time"
)

// DiagStatus 诊断状态.
type DiagStatus string

const (
	DiagStatusPass     DiagStatus = "pass"     // 通过
	DiagStatusWarn     DiagStatus = "warn"     // 警告
	DiagStatusFail     DiagStatus = "fail"     // 失败
	DiagStatusRunning  DiagStatus = "running"  // 运行中
	DiagStatusPending  DiagStatus = "pending"  // 等待中
)

// DiagCategory 诊断类别.
type DiagCategory string

const (
	CategoryHardware    DiagCategory = "hardware"    // 硬件
	CategoryStorage     DiagCategory = "storage"     // 存储
	CategoryFilesystem  DiagCategory = "filesystem"  // 文件系统
	CategoryNetwork     DiagCategory = "network"     // 网络
	CategoryService     DiagCategory = "service"     // 服务
	CategoryPerformance DiagCategory = "performance" // 性能
)

// Severity 问题严重程度.
type Severity string

const (
	SeverityLow      Severity = "low"
	SeverityMedium   Severity = "medium"
	SeverityHigh     Severity = "high"
	SeverityCritical Severity = "critical"
)

// DiagTask 诊断任务.
type DiagTask struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Category  DiagCategory `json:"category"`
	Status    DiagStatus   `json:"status"`
	StartTime time.Time    `json:"start_time"`
	EndTime   time.Time    `json:"end_time,omitempty"`
	Results   []*DiagResult `json:"results,omitempty"`
}

// DiagResult 单项诊断结果.
type DiagResult struct {
	Name     string            `json:"name"`
	Category DiagCategory      `json:"category"`
	Status   DiagStatus        `json:"status"`
	Message  string            `json:"message"`
	Details  map[string]interface{} `json:"details,omitempty"`
	Duration time.Duration     `json:"duration"`
}

// HealthCheckItem 健康检查项.
type HealthCheckItem struct {
	Name        string       `json:"name"`
	Category    DiagCategory `json:"category"`
	Status      DiagStatus   `json:"status"`
	Value       interface{}  `json:"value"`
	Threshold   interface{}  `json:"threshold,omitempty"`
	Message     string       `json:"message"`
	LastChecked time.Time    `json:"last_checked"`
}

// RepairSuggestion 修复建议.
type RepairSuggestion struct {
	ID          string   `json:"id"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Severity    Severity `json:"severity"`
	Steps       []string `json:"steps"`
	AutoFixable bool     `json:"auto_fixable"`
}

// DiagReport 诊断报告.
type DiagReport struct {
	ID          string              `json:"id"`
	TaskID      string              `json:"task_id"`
	GeneratedAt time.Time           `json:"generated_at"`
	Summary     *DiagSummary        `json:"summary"`
	Results     []*DiagResult       `json:"results"`
	HealthItems []*HealthCheckItem  `json:"health_items"`
	Suggestions []*RepairSuggestion `json:"suggestions"`
}

// DiagSummary 诊断摘要.
type DiagSummary struct {
	TotalChecks int `json:"total_checks"`
	Passed      int `json:"passed"`
	Warnings    int `json:"warnings"`
	Failures    int `json:"failures"`
	Duration    time.Duration `json:"duration"`
}

// HardwareInfo 硬件信息.
type HardwareInfo struct {
	CPUModel     string  `json:"cpu_model"`
	CPUCores     int     `json:"cpu_cores"`
	CPUTemp      float64 `json:"cpu_temp"`
	MemTotalGB   float64 `json:"mem_total_gb"`
	MemUsedGB    float64 `json:"mem_used_gb"`
	MemUsagePct  float64 `json:"mem_usage_pct"`
	DiskTotalGB  float64 `json:"disk_total_gb"`
	DiskUsedGB   float64 `json:"disk_used_gb"`
	DiskUsagePct float64 `json:"disk_usage_pct"`
}

// StorageArrayStatus 存储阵列状态.
type StorageArrayStatus struct {
	Name       string   `json:"name"`
	Level      string   `json:"level"`       // RAID 级别
	State      string   `json:"state"`       // 状态
	Devices    []string `json:"devices"`     // 设备列表
	Active     int      `json:"active"`      // 活跃设备数
	Degraded   int      `json:"degraded"`    // 降级设备数
	Failed     int      `json:"failed"`      // 失败设备数
	Spare      int      `json:"spare"`       // 备用设备数
	TotalSize  string   `json:"total_size"`
	UsedSize   string   `json:"used_size"`
}

// NetworkTestResult 网络测试结果.
type NetworkTestResult struct {
	Target     string        `json:"target"`
	Reachable  bool          `json:"reachable"`
	Latency    time.Duration `json:"latency"`
	PacketLoss float64       `json:"packet_loss"`
	Error      string        `json:"error,omitempty"`
}

// ServiceStatus 服务状态.
type ServiceStatus struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`    // active, inactive, failed
	SubStatus string    `json:"sub_status"`
	PID       int       `json:"pid"`
	Memory    int64     `json:"memory"`
	Uptime    string    `json:"uptime"`
	Since     time.Time `json:"since"`
}

// ========== 系统诊断增强类型 ==========

// NetworkDiag 网络诊断结果
type NetworkDiag struct {
	Interfaces     []NetworkInterface `json:"interfaces"`
	Connectivity   ConnectivityTest   `json:"connectivity"`
	DNSResolution  DNSDiagResult      `json:"dns_resolution"`
	Bandwidth      BandwidthTest      `json:"bandwidth"`
	Latency        []LatencyTest      `json:"latency"`
	Issues         []DiagIssue        `json:"issues,omitempty"`
}

// NetworkInterface 网络接口信息
type NetworkInterface struct {
	Name      string `json:"name"`
	Status    string `json:"status"` // up, down
	IP        string `json:"ip"`
	MAC       string `json:"mac"`
	Speed     string `json:"speed"`
	MTU       int    `json:"mtu"`
}

// ConnectivityTest 连通性测试
type ConnectivityTest struct {
	GatewayReachable bool          `json:"gateway_reachable"`
	InternetReachable bool         `json:"internet_reachable"`
	GatewayLatency   time.Duration `json:"gateway_latency"`
	InternetLatency  time.Duration `json:"internet_latency"`
}

// DNSDiagResult DNS 诊断结果
type DNSDiagResult struct {
	Resolver   string        `json:"resolver"`
	Working    bool          `json:"working"`
	Latency    time.Duration `json:"latency"`
}

// BandwidthTest 带宽测试
type BandwidthTest struct {
	UploadMbps   float64 `json:"upload_mbps"`
	DownloadMbps float64 `json:"download_mbps"`
}

// LatencyTest 延迟测试
type LatencyTest struct {
	Target  string        `json:"target"`
	Avg     time.Duration `json:"avg"`
	Min     time.Duration `json:"min"`
	Max     time.Duration `json:"max"`
	Loss    float64       `json:"loss_percent"`
}

// StorageDiag 存储诊断结果
type StorageDiag struct {
	Arrays      []StorageArrayDiag  `json:"arrays"`
	Disks       []DiskDiag          `json:"disks"`
	Filesystems []FilesystemDiag    `json:"filesystems"`
	SMART       []SMARTDiag         `json:"smart"`
	Issues      []DiagIssue         `json:"issues,omitempty"`
}

// StorageArrayDiag 存储阵列诊断
type StorageArrayDiag struct {
	Name     string `json:"name"`
	Level    string `json:"level"`
	State    string `json:"state"`
	Healthy  bool   `json:"healthy"`
	Degraded bool   `json:"degraded"`
}

// DiskDiag 磁盘诊断
type DiskDiag struct {
	Device   string  `json:"device"`
	Model    string  `json:"model"`
	SizeGB   float64 `json:"size_gb"`
	Temp     float64 `json:"temp_celsius"`
	Healthy  bool    `json:"healthy"`
	Warnings []string `json:"warnings,omitempty"`
}

// FilesystemDiag 文件系统诊断
type FilesystemDiag struct {
	Mount    string  `json:"mount"`
	Type     string  `json:"type"`
	SizeGB   float64 `json:"size_gb"`
	UsedGB   float64 `json:"used_gb"`
	UsagePct float64 `json:"usage_pct"`
	Clean    bool    `json:"clean"`
}

// SMARTDiag SMART 诊断
type SMARTDiag struct {
	Device    string `json:"device"`
	Health    string `json:"health"`
	Temp      int    `json:"temp"`
	PowerOn   int    `json:"power_on_hours"`
	Reallocated int  `json:"reallocated_sectors"`
}

// PerfBottleneck 性能瓶颈分析
type PerfBottleneck struct {
	Component   string  `json:"component"` // cpu, memory, disk, network
	Severity    string  `json:"severity"`  // low, medium, high, critical
	Current     float64 `json:"current_value"`
	Threshold   float64 `json:"threshold"`
	Unit        string  `json:"unit"`
	Description string  `json:"description"`
	Recommendation string `json:"recommendation"`
}

// AutoFix 自动修复
type AutoFix struct {
	ID          string    `json:"id"`
	IssueType   string    `json:"issue_type"`
	Component   string    `json:"component"`
	Action      string    `json:"action"`
	Status      string    `json:"status"` // pending, running, success, failed
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Result      string    `json:"result,omitempty"`
}

// DiagIssue 诊断问题
type DiagIssue struct {
	Component string `json:"component"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
}

// AutoFixRequest 自动修复请求
type AutoFixRequest struct {
	IssueType string `json:"issue_type" binding:"required"`
	Component string `json:"component" binding:"required"`
}
