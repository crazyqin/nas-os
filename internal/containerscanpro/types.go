// Package containerscanpro 提供容器安全扫描增强功能，包括 CVE 漏洞检测、
// 合规策略检查、运行时异常检测、自动修复建议、扫描报告生成等。
package containerscanpro

import (
	"time"
)

// ============================================================
// 常量和枚举
// ============================================================

// 严重级别
const (
	SeverityCritical = "CRITICAL"
	SeverityHigh     = "HIGH"
	SeverityMedium   = "MEDIUM"
	SeverityLow      = "LOW"
	SeverityInfo     = "INFO"
)

// 扫描状态
type ScanStatus string

const (
	StatusQueued    ScanStatus = "queued"
	StatusScanning  ScanStatus = "scanning"
	StatusCompleted ScanStatus = "completed"
	StatusFailed    ScanStatus = "failed"
)

// 修复方式
type FixAction string

const (
	FixActionUpgrade FixAction = "upgrade"  // 升级依赖
	FixActionReplace FixAction = "replace"  // 替换镜像
	FixActionIgnore  FixAction = "ignore"   // 忽略
)

// 合规标准
type ComplianceStandard string

const (
	StandardCIS      ComplianceStandard = "CIS"      // CIS Benchmark
	StandardCustom   ComplianceStandard = "CUSTOM"   // 自定义规则
	StandardNIST     ComplianceStandard = "NIST"     // NIST 800-190
	StandardOWASP    ComplianceStandard = "OWASP"    // OWASP
)

// 运行时异常类型
type AnomalyType string

const (
	AnomalyPrivilegedContainer AnomalyType = "privileged_container" // 特权容器
	AnomalyAbnormalProcess     AnomalyType = "abnormal_process"     // 异常进程
	AnomalyAbnormalNetwork     AnomalyType = "abnormal_network"     // 异常网络
	AnomalyResourceAbuse       AnomalyType = "resource_abuse"       // 资源异常
	AnomalySensitiveMount      AnomalyType = "sensitive_mount"      // 敏感挂载
)

// ============================================================
// 核心数据结构
// ============================================================

// ScanPolicy 扫描策略配置
type ScanPolicy struct {
	ID              string    `json:"id"`                          // 策略 ID
	Name            string    `json:"name"`                        // 策略名称
	Description     string    `json:"description"`                 // 描述
	SeverityThreshold string  `json:"severity_threshold"`          // 严重级别阈值（低于此级别的忽略）
	AutoFixEnabled  bool      `json:"autofix_enabled"`             // 是否启用自动修复
	ComplianceStandards []ComplianceStandard `json:"compliance_standards"` // 合规标准列表
	ExcludePackages []string  `json:"exclude_packages,omitempty"`  // 排除的包
	ScheduleCron    string    `json:"schedule_cron,omitempty"`     // 定时扫描 cron 表达式
	Enabled         bool      `json:"enabled"`                     // 是否启用
	CreatedAt       time.Time `json:"created_at"`                  // 创建时间
	UpdatedAt       time.Time `json:"updated_at"`                  // 更新时间
}

// VulnerabilityCVE CVE 漏洞条目
type VulnerabilityCVE struct {
	CVEID       string    `json:"cve_id"`       // CVE ID，如 CVE-2024-1234
	Severity    string    `json:"severity"`     // 严重级别
	Title       string    `json:"title"`        // 标题
	Description string    `json:"description"`  // 描述
	Package     string    `json:"package"`      // 影响的包名
	Version     string    `json:"version"`      // 当前版本
	FixVersion  string    `json:"fix_version"`  // 修复版本
	CVSS        float64   `json:"cvss"`         // CVSS 评分
	PublishedAt time.Time `json:"published_at"` // 发布时间
	References  []string  `json:"references,omitempty"` // 参考链接
}

// ScanResult 扫描结果
type ScanResult struct {
	ID              string             `json:"id"`               // 扫描 ID
	ImageID         string             `json:"image_id"`         // 镜像 ID
	ImageName       string             `json:"image_name"`       // 镜像名称
	ScanTime        time.Time          `json:"scan_time"`        // 扫描时间
	Duration        time.Duration      `json:"duration"`         // 扫描耗时
	Status          ScanStatus         `json:"status"`           // 扫描状态
	Vulnerabilities []VulnerabilityCVE `json:"vulnerabilities"`  // 漏洞列表
	ComplianceStatus []ComplianceResult `json:"compliance_status"` // 合规状态
	VulnSummary     VulnSummary        `json:"vuln_summary"`     // 漏洞统计
	Compliant       bool               `json:"compliant"`        // 是否合规
	FixSuggestions  []AutoFixAction    `json:"fix_suggestions"`  // 修复建议
	PolicyID        string             `json:"policy_id"`        // 使用的策略 ID
}

// VulnSummary 漏洞统计
type VulnSummary struct {
	Total    int `json:"total"`
	Critical int `json:"critical"`
	High     int `json:"high"`
	Medium   int `json:"medium"`
	Low      int `json:"low"`
	Info     int `json:"info"`
}

// ComplianceRule 合规规则
type ComplianceRule struct {
	ID          string            `json:"id"`           // 规则 ID
	Name        string            `json:"name"`         // 规则名称
	Description string            `json:"description"`  // 描述
	Standard    ComplianceStandard `json:"standard"`     // 所属标准
	CheckLogic  string            `json:"check_logic"`  // 检查逻辑描述
	Severity    string            `json:"severity"`     // 违反时的严重级别
	Enabled     bool              `json:"enabled"`      // 是否启用
}

// ComplianceResult 合规检查结果
type ComplianceResult struct {
	RuleID    string `json:"rule_id"`    // 规则 ID
	RuleName  string `json:"rule_name"`  // 规则名称
	Passed    bool   `json:"passed"`     // 是否通过
	Message   string `json:"message"`    // 检查结果消息
	Severity  string `json:"severity"`   // 严重级别
}

// RuntimeMonitor 运行时监控状态
type RuntimeMonitor struct {
	ContainerID   string        `json:"container_id"`   // 容器 ID
	ContainerName string        `json:"container_name"` // 容器名称
	ImageName     string        `json:"image_name"`     // 镜像名称
	IsPrivileged  bool          `json:"is_privileged"`  // 是否特权容器
	Anomalies     []Anomaly     `json:"anomalies"`      // 异常列表
	LastCheckTime time.Time     `json:"last_check_time"` // 最后检查时间
	Status        string        `json:"status"`         // 状态：normal/warning/critical
}

// Anomaly 异常行为
type Anomaly struct {
	Type      AnomalyType `json:"type"`       // 异常类型
	Message   string      `json:"message"`    // 描述
	Severity  string      `json:"severity"`   // 严重级别
	DetectedAt time.Time  `json:"detected_at"` // 检测时间
	Details   string      `json:"details"`    // 详细信息
}

// AutoFixAction 自动修复动作
type AutoFixAction struct {
	VulnID      string   `json:"vuln_id"`      // 漏洞 ID
	Package     string   `json:"package"`      // 包名
	CurrentVer  string   `json:"current_ver"`  // 当前版本
	FixVer      string   `json:"fix_ver"`      // 修复版本
	Action      FixAction `json:"action"`      // 修复方式
	Command     string   `json:"command"`      // 修复命令
	Description string   `json:"description"`  // 描述
	Applied     bool     `json:"applied"`      // 是否已执行
}

// ListEntry 黑名单/白名单条目
type ListEntry struct {
	ImageName string    `json:"image_name"` // 镜像名称
	Reason    string    `json:"reason"`     // 原因
	AddedAt   time.Time `json:"added_at"`   // 添加时间
	AddedBy   string    `json:"added_by"`   // 添加者
}

// ============================================================
// API 请求/响应结构
// ============================================================

// ScanRequest 扫描请求
type ScanRequest struct {
	ImageName string `json:"image_name" binding:"required"` // 镜像名称
	PolicyID  string `json:"policy_id"`                     // 策略 ID（可选）
	ForceRescan bool `json:"force_rescan"`                  // 强制重新扫描
}

// PolicyRequest 策略创建请求
type PolicyRequest struct {
	Name               string             `json:"name" binding:"required"`
	Description        string             `json:"description"`
	SeverityThreshold  string             `json:"severity_threshold"`
	AutoFixEnabled     bool               `json:"autofix_enabled"`
	ComplianceStandards []ComplianceStandard `json:"compliance_standards"`
	ExcludePackages    []string           `json:"exclude_packages"`
	ScheduleCron       string             `json:"schedule_cron"`
	Enabled            *bool              `json:"enabled"`
}

// AutoFixRequest 自动修复请求
type AutoFixRequest struct {
	ScanID    string   `json:"scan_id" binding:"required"`    // 扫描 ID
	VulnIDs   []string `json:"vuln_ids"`                      // 指定漏洞 ID 列表（空则修复全部）
	FixAction FixAction `json:"fix_action"`                   // 修复方式
}

// APIResponse 统一 API 响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}
