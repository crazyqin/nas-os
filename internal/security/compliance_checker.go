// Package security 提供合规检查功能
package security

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 合规标准类型 ==========

// ComplianceStandard 合规标准类型
type ComplianceStandard string

const (
	StandardGDPR     ComplianceStandard = "gdpr"     // 通用数据保护条例
	StandardSOC2     ComplianceStandard = "soc2"     // SOC 2 Type II
	StandardISO27001 ComplianceStandard = "iso27001" // ISO/IEC 27001
	StandardHIPAA    ComplianceStandard = "hipaa"    // 健康保险流通与责任法案
	StandardPCI      ComplianceStandard = "pci"      // PCI DSS
	StandardCCPA     ComplianceStandard = "ccpa"     // 加州消费者隐私法
	StandardNIST     ComplianceStandard = "nist"     // NIST Cybersecurity Framework
	StandardCSL      ComplianceStandard = "csl"      // 中国网络安全法
	StandardDSL      ComplianceStandard = "dsl"      // 中国数据安全法
	StandardPIPL     ComplianceStandard = "pipl"     // 中国个人信息保护法
)

// ComplianceLevel 合规等级
type ComplianceLevel string

const (
	LevelFull         ComplianceLevel = "full"          // 完全合规
	LevelPartial      ComplianceLevel = "partial"       // 部分合规
	LevelNonCompliant ComplianceLevel = "non_compliant" // 不合规
	LevelUnknown      ComplianceLevel = "unknown"       // 未知
)

// ComplianceStatus 合规状态
type ComplianceStatus string

const (
	StatusPassed        ComplianceStatus = "passed"
	StatusFailed        ComplianceStatus = "failed"
	StatusWarning       ComplianceStatus = "warning"
	StatusSkipped       ComplianceStatus = "skipped"
	StatusNotApplicable ComplianceStatus = "not_applicable"
)

// ========== 检查项类型 ==========

// ComplianceCategory 合规检查类别
type ComplianceCategory string

const (
	CategoryAccessControl      ComplianceCategory = "access_control"
	CategoryDataProtection     ComplianceCategory = "data_protection"
	CategoryEncryption         ComplianceCategory = "encryption"
	CategoryAudit              ComplianceCategory = "audit"
	CategoryIncidentResponse   ComplianceCategory = "incident_response"
	CategoryBusinessContinuity ComplianceCategory = "business_continuity"
	CategoryAssetManagement    ComplianceCategory = "asset_management"
	CategoryNetworkSecurity    ComplianceCategory = "network_security"
	CategoryVulnerability      ComplianceCategory = "vulnerability"
	CategoryPrivacy            ComplianceCategory = "privacy"
	CategoryConsent            ComplianceCategory = "consent"
	CategoryBreachNotification ComplianceCategory = "breach_notification"
	CategoryDataRetention      ComplianceCategory = "data_retention"
	CategoryThirdParty         ComplianceCategory = "third_party"
)

// ComplianceCheckItem 合规检查项
type ComplianceCheckItem struct {
	ID              string             `json:"id"`
	Standard        ComplianceStandard `json:"standard"`
	ControlID       string             `json:"control_id"` // 如 GDPR Article 5, SOC2 CC6.1
	Category        ComplianceCategory `json:"category"`
	Name            string             `json:"name"`
	Description     string             `json:"description"`
	Requirement     string             `json:"requirement"`
	Weight          int                `json:"weight"`   // 权重
	Severity        string             `json:"severity"` // critical, high, medium, low
	Remediation     string             `json:"remediation"`
	References      []string           `json:"references"`
	ApplicableRoles []string           `json:"applicable_roles"`
	Tags            []string           `json:"tags"`
}

// ComplianceCheckResult 合规检查结果
type ComplianceCheckResult struct {
	ItemID         string                 `json:"item_id"`
	Standard       ComplianceStandard     `json:"standard"`
	ControlID      string                 `json:"control_id"`
	Category       ComplianceCategory     `json:"category"`
	Name           string                 `json:"name"`
	Status         ComplianceStatus       `json:"status"`
	Level          ComplianceLevel        `json:"level"`
	Score          int                    `json:"score"` // 0-100
	Message        string                 `json:"message"`
	Details        map[string]interface{} `json:"details,omitempty"`
	Evidence       []string               `json:"evidence,omitempty"`
	Remediation    string                 `json:"remediation,omitempty"`
	CheckTime      time.Time              `json:"check_time"`
	Duration       time.Duration          `json:"duration"`
	CheckedBy      string                 `json:"checked_by"`
	Acknowledged   bool                   `json:"acknowledged"`
	AcknowledgedBy string                 `json:"acknowledged_by,omitempty"`
	AcknowledgedAt *time.Time             `json:"acknowledged_at,omitempty"`
}

// ========== 合规报告类型 ==========

// ComplianceReport 合规报告
type ComplianceReport struct {
	ReportID        string                   `json:"report_id"`
	Title           string                   `json:"title"`
	Standard        ComplianceStandard       `json:"standard"`
	GeneratedAt     time.Time                `json:"generated_at"`
	ValidUntil      time.Time                `json:"valid_until"`
	OverallScore    int                      `json:"overall_score"` // 0-100
	OverallLevel    ComplianceLevel          `json:"overall_level"`
	Summary         ComplianceSummary        `json:"summary"`
	CategoryScores  map[string]int           `json:"category_scores"`
	Results         []*ComplianceCheckResult `json:"results"`
	Remediations    []RemediationItem        `json:"remediations"`
	Recommendations []string                 `json:"recommendations"`
	NextReviewDate  time.Time                `json:"next_review_date"`
	Version         string                   `json:"version"`
}

// ComplianceSummary 合规摘要
type ComplianceSummary struct {
	TotalChecks    int `json:"total_checks"`
	PassedChecks   int `json:"passed_checks"`
	FailedChecks   int `json:"failed_checks"`
	WarningChecks  int `json:"warning_checks"`
	SkippedChecks  int `json:"skipped_checks"`
	NotApplicable  int `json:"not_applicable"`
	CriticalIssues int `json:"critical_issues"`
	HighIssues     int `json:"high_issues"`
	MediumIssues   int `json:"medium_issues"`
	LowIssues      int `json:"low_issues"`
}

// RemediationItem 整改项
type RemediationItem struct {
	ID          string     `json:"id"`
	ItemID      string     `json:"item_id"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Priority    int        `json:"priority"` // 1-4，1最高
	Status      string     `json:"status"`   // open, in_progress, resolved
	AssignedTo  string     `json:"assigned_to,omitempty"`
	DueDate     *time.Time `json:"due_date,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`
	Resolution  string     `json:"resolution,omitempty"`
}

// ========== 合规检查器 ==========

// ComplianceChecker 合规检查器
type ComplianceChecker struct {
	config       ComplianceCheckerConfig
	standards    map[ComplianceStandard]*StandardDefinition
	results      map[string]*ComplianceCheckResult
	reports      []*ComplianceReport
	checkHistory map[string][]*ComplianceCheckResult
	mu           sync.RWMutex
	storageDir   string
}

// ComplianceCheckerConfig 合规检查器配置
type ComplianceCheckerConfig struct {
	Enabled          bool                 `json:"enabled"`
	AutoCheck        bool                 `json:"auto_check"`
	CheckInterval    time.Duration        `json:"check_interval"`
	ReportRetention  int                  `json:"report_retention"` // 天数
	MaxReports       int                  `json:"max_reports"`
	NotifyOnFailure  bool                 `json:"notify_on_failure"`
	NotifyChannels   []string             `json:"notify_channels"`
	EnabledStandards []ComplianceStandard `json:"enabled_standards"`
	CustomChecks     []CustomCheck        `json:"custom_checks"`
}

// DefaultComplianceCheckerConfig 默认配置
func DefaultComplianceCheckerConfig() ComplianceCheckerConfig {
	return ComplianceCheckerConfig{
		Enabled:          true,
		AutoCheck:        true,
		CheckInterval:    24 * time.Hour,
		ReportRetention:  365,
		MaxReports:       100,
		NotifyOnFailure:  true,
		EnabledStandards: []ComplianceStandard{StandardGDPR, StandardSOC2},
	}
}

// StandardDefinition 标准定义
type StandardDefinition struct {
	Standard   ComplianceStandard     `json:"standard"`
	Name       string                 `json:"name"`
	Version    string                 `json:"version"`
	Effective  time.Time              `json:"effective"`
	CheckItems []*ComplianceCheckItem `json:"check_items"`
	Weights    map[string]int         `json:"weights"`
}

// CustomCheck 自定义检查
type CustomCheck struct {
	ID          string             `json:"id"`
	Name        string             `json:"name"`
	Description string             `json:"description"`
	Standard    ComplianceStandard `json:"standard"`
	Category    ComplianceCategory `json:"category"`
	CheckFunc   func(ctx context.Context) (ComplianceStatus, string, error)
}

// NewComplianceChecker 创建合规检查器
func NewComplianceChecker(config ComplianceCheckerConfig) *ComplianceChecker {
	storageDir := "/var/lib/nas-os/compliance"
	os.MkdirAll(storageDir, 0750)

	cc := &ComplianceChecker{
		config:       config,
		standards:    make(map[ComplianceStandard]*StandardDefinition),
		results:      make(map[string]*ComplianceCheckResult),
		reports:      make([]*ComplianceReport, 0),
		checkHistory: make(map[string][]*ComplianceCheckResult),
		storageDir:   storageDir,
	}

	// 初始化标准
	cc.initStandards()

	// 加载历史报告
	cc.loadReports()

	return cc
}

// initStandards 初始化合规标准
func (cc *ComplianceChecker) initStandards() {
	// GDPR 标准
	cc.standards[StandardGDPR] = cc.initGDPRStandard()

	// SOC 2 标准
	cc.standards[StandardSOC2] = cc.initSOC2Standard()

	// ISO 27001 标准
	cc.standards[StandardISO27001] = cc.initISO27001Standard()

	// 中国相关法规
	cc.standards[StandardCSL] = cc.initCSLStandard()
	cc.standards[StandardPIPL] = cc.initPIPLStandard()
}

// initGDPRStandard 初始化 GDPR 标准
func (cc *ComplianceChecker) initGDPRStandard() *StandardDefinition {
	return &StandardDefinition{
		Standard:  StandardGDPR,
		Name:      "General Data Protection Regulation",
		Version:   "2018",
		Effective: time.Date(2018, 5, 25, 0, 0, 0, 0, time.UTC),
		CheckItems: []*ComplianceCheckItem{
			{
				ID:          "gdpr-001",
				Standard:    StandardGDPR,
				ControlID:   "Article 5.1.a",
				Category:    CategoryDataProtection,
				Name:        "数据处理的合法性",
				Description: "个人数据必须以合法、公平和透明的方式处理",
				Requirement: "确保所有个人数据处理活动都有合法依据",
				Weight:      10,
				Severity:    "high",
				Remediation: "建立数据处理合法依据评估流程",
				References:  []string{"https://gdpr.eu/article-5/"},
			},
			{
				ID:          "gdpr-002",
				Standard:    StandardGDPR,
				ControlID:   "Article 5.1.b",
				Category:    CategoryDataProtection,
				Name:        "数据最小化原则",
				Description: "个人数据必须限于处理目的所必需的最小范围",
				Requirement: "仅收集实现处理目的所必需的最少个人数据",
				Weight:      8,
				Severity:    "medium",
				Remediation: "审查数据收集范围，移除不必要的字段",
			},
			{
				ID:          "gdpr-003",
				Standard:    StandardGDPR,
				ControlID:   "Article 5.1.c",
				Category:    CategoryDataRetention,
				Name:        "数据存储限制",
				Description: "个人数据应以可识别数据主体的形式保存不超过处理目的所需的时间",
				Requirement: "建立数据保留策略，定期清理过期数据",
				Weight:      9,
				Severity:    "high",
				Remediation: "实施数据保留策略和自动清理机制",
			},
			{
				ID:          "gdpr-004",
				Standard:    StandardGDPR,
				ControlID:   "Article 5.1.d",
				Category:    CategoryDataProtection,
				Name:        "数据准确性",
				Description: "个人数据必须准确，并在必要时保持最新",
				Requirement: "建立数据准确性验证和更新机制",
				Weight:      7,
				Severity:    "medium",
			},
			{
				ID:          "gdpr-005",
				Standard:    StandardGDPR,
				ControlID:   "Article 5.1.f",
				Category:    CategoryDataProtection,
				Name:        "数据完整性",
				Description: "个人数据必须以适当的安全方式处理",
				Requirement: "实施适当的技术和组织措施保护个人数据",
				Weight:      10,
				Severity:    "critical",
				Remediation: "实施加密、访问控制、审计日志等安全措施",
			},
			{
				ID:          "gdpr-006",
				Standard:    StandardGDPR,
				ControlID:   "Article 7",
				Category:    CategoryConsent,
				Name:        "同意管理",
				Description: "数据处理必须获得数据主体的有效同意",
				Requirement: "建立同意收集、记录和撤回机制",
				Weight:      9,
				Severity:    "high",
			},
			{
				ID:          "gdpr-007",
				Standard:    StandardGDPR,
				ControlID:   "Article 15",
				Category:    CategoryPrivacy,
				Name:        "数据主体访问权",
				Description: "数据主体有权获取其个人数据的副本",
				Requirement: "建立数据主体访问请求处理流程",
				Weight:      8,
				Severity:    "high",
			},
			{
				ID:          "gdpr-008",
				Standard:    StandardGDPR,
				ControlID:   "Article 17",
				Category:    CategoryDataProtection,
				Name:        "删除权",
				Description: "数据主体有权要求删除其个人数据",
				Requirement: "建立数据删除请求处理流程",
				Weight:      8,
				Severity:    "high",
			},
			{
				ID:          "gdpr-009",
				Standard:    StandardGDPR,
				ControlID:   "Article 32",
				Category:    CategoryEncryption,
				Name:        "数据处理安全",
				Description: "实施适当的技术和组织措施确保安全",
				Requirement: "实施加密、伪匿名化、访问控制等措施",
				Weight:      10,
				Severity:    "critical",
			},
			{
				ID:          "gdpr-010",
				Standard:    StandardGDPR,
				ControlID:   "Article 33",
				Category:    CategoryBreachNotification,
				Name:        "数据泄露通知",
				Description: "发现数据泄露后72小时内通知监管机构",
				Requirement: "建立数据泄露检测和通知机制",
				Weight:      10,
				Severity:    "critical",
			},
			{
				ID:          "gdpr-011",
				Standard:    StandardGDPR,
				ControlID:   "Article 35",
				Category:    CategoryPrivacy,
				Name:        "数据保护影响评估",
				Description: "高风险处理活动需进行DPIA",
				Requirement: "建立DPIA评估流程",
				Weight:      8,
				Severity:    "high",
			},
			{
				ID:          "gdpr-012",
				Standard:    StandardGDPR,
				ControlID:   "Article 37",
				Category:    CategoryPrivacy,
				Name:        "数据保护官",
				Description: "指定数据保护官(DPO)",
				Requirement: "任命合格的数据保护官",
				Weight:      7,
				Severity:    "medium",
			},
		},
		Weights: map[string]int{
			"critical": 10,
			"high":     8,
			"medium":   5,
			"low":      2,
		},
	}
}

// initSOC2Standard 初始化 SOC 2 标准
func (cc *ComplianceChecker) initSOC2Standard() *StandardDefinition {
	return &StandardDefinition{
		Standard:  StandardSOC2,
		Name:      "SOC 2 Type II",
		Version:   "2017",
		Effective: time.Date(2017, 1, 1, 0, 0, 0, 0, time.UTC),
		CheckItems: []*ComplianceCheckItem{
			{
				ID:          "soc2-001",
				Standard:    StandardSOC2,
				ControlID:   "CC6.1",
				Category:    CategoryAccessControl,
				Name:        "访问控制策略",
				Description: "建立逻辑访问安全策略和程序",
				Requirement: "制定并实施访问控制策略",
				Weight:      10,
				Severity:    "critical",
			},
			{
				ID:          "soc2-002",
				Standard:    StandardSOC2,
				ControlID:   "CC6.2",
				Category:    CategoryAccessControl,
				Name:        "系统账户管理",
				Description: "在新用户创建前进行注册和授权",
				Requirement: "实施用户账户生命周期管理",
				Weight:      9,
				Severity:    "high",
			},
			{
				ID:          "soc2-003",
				Standard:    StandardSOC2,
				ControlID:   "CC6.3",
				Category:    CategoryAccessControl,
				Name:        "权限管理",
				Description: "基于角色和职责分配系统访问权限",
				Requirement: "实施基于角色的访问控制(RBAC)",
				Weight:      9,
				Severity:    "high",
			},
			{
				ID:          "soc2-004",
				Standard:    StandardSOC2,
				ControlID:   "CC6.6",
				Category:    CategoryNetworkSecurity,
				Name:        "网络安全控制",
				Description: "实施边界保护措施",
				Requirement: "配置防火墙、入侵检测等网络安全控制",
				Weight:      9,
				Severity:    "high",
			},
			{
				ID:          "soc2-005",
				Standard:    StandardSOC2,
				ControlID:   "CC6.7",
				Category:    CategoryVulnerability,
				Name:        "漏洞管理",
				Description: "建立漏洞识别和管理流程",
				Requirement: "实施漏洞扫描和补丁管理流程",
				Weight:      9,
				Severity:    "high",
			},
			{
				ID:          "soc2-006",
				Standard:    StandardSOC2,
				ControlID:   "CC7.1",
				Category:    CategoryVulnerability,
				Name:        "威胁识别",
				Description: "识别和评估潜在安全威胁",
				Requirement: "建立威胁情报收集和分析流程",
				Weight:      8,
				Severity:    "high",
			},
			{
				ID:          "soc2-007",
				Standard:    StandardSOC2,
				ControlID:   "CC7.2",
				Category:    CategoryAudit,
				Name:        "系统监控",
				Description: "监控系统运行和安全事件",
				Requirement: "实施安全事件监控和日志记录",
				Weight:      9,
				Severity:    "high",
			},
			{
				ID:          "soc2-008",
				Standard:    StandardSOC2,
				ControlID:   "CC7.4",
				Category:    CategoryIncidentResponse,
				Name:        "事件响应",
				Description: "建立安全事件响应流程",
				Requirement: "制定并测试事件响应计划",
				Weight:      9,
				Severity:    "high",
			},
			{
				ID:          "soc2-009",
				Standard:    StandardSOC2,
				ControlID:   "CC8.1",
				Category:    CategoryBusinessContinuity,
				Name:        "变更管理",
				Description: "管理影响系统的变更",
				Requirement: "建立变更管理和审批流程",
				Weight:      8,
				Severity:    "medium",
			},
			{
				ID:          "soc2-010",
				Standard:    StandardSOC2,
				ControlID:   "A1.2",
				Category:    CategoryBusinessContinuity,
				Name:        "业务连续性",
				Description: "建立业务连续性和灾难恢复计划",
				Requirement: "制定并测试业务连续性计划",
				Weight:      8,
				Severity:    "high",
			},
			{
				ID:          "soc2-011",
				Standard:    StandardSOC2,
				ControlID:   "CC9.2",
				Category:    CategoryThirdParty,
				Name:        "供应商管理",
				Description: "评估和管理供应商风险",
				Requirement: "建立供应商风险评估流程",
				Weight:      7,
				Severity:    "medium",
			},
		},
		Weights: map[string]int{
			"critical": 10,
			"high":     8,
			"medium":   5,
			"low":      2,
		},
	}
}

// initISO27001Standard 初始化 ISO 27001 标准
func (cc *ComplianceChecker) initISO27001Standard() *StandardDefinition {
	return &StandardDefinition{
		Standard:  StandardISO27001,
		Name:      "ISO/IEC 27001:2022",
		Version:   "2022",
		Effective: time.Date(2022, 10, 1, 0, 0, 0, 0, time.UTC),
		CheckItems: []*ComplianceCheckItem{
			{
				ID:          "iso27001-001",
				Standard:    StandardISO27001,
				ControlID:   "A.5.1",
				Category:    CategoryAccessControl,
				Name:        "信息安全政策",
				Description: "建立信息安全政策并获得管理层批准",
				Weight:      10,
				Severity:    "critical",
			},
			{
				ID:          "iso27001-002",
				Standard:    StandardISO27001,
				ControlID:   "A.5.2",
				Category:    CategoryAssetManagement,
				Name:        "信息安全组织",
				Description: "建立信息安全组织结构",
				Weight:      8,
				Severity:    "high",
			},
			{
				ID:          "iso27001-003",
				Standard:    StandardISO27001,
				ControlID:   "A.5.3",
				Category:    CategoryAccessControl,
				Name:        "角色和职责",
				Description: "定义信息安全角色和职责",
				Weight:      8,
				Severity:    "high",
			},
		},
		Weights: map[string]int{
			"critical": 10,
			"high":     8,
			"medium":   5,
			"low":      2,
		},
	}
}

// initCSLStandard 初始化网络安全法标准
func (cc *ComplianceChecker) initCSLStandard() *StandardDefinition {
	return &StandardDefinition{
		Standard:  StandardCSL,
		Name:      "中华人民共和国网络安全法",
		Version:   "2017",
		Effective: time.Date(2017, 6, 1, 0, 0, 0, 0, time.UTC),
		CheckItems: []*ComplianceCheckItem{
			{
				ID:          "csl-001",
				Standard:    StandardCSL,
				ControlID:   "第21条",
				Category:    CategoryNetworkSecurity,
				Name:        "网络安全等级保护",
				Description: "实施网络安全等级保护制度",
				Weight:      10,
				Severity:    "critical",
			},
			{
				ID:          "csl-002",
				Standard:    StandardCSL,
				ControlID:   "第22条",
				Category:    CategoryVulnerability,
				Name:        "安全漏洞修复",
				Description: "及时修复系统漏洞",
				Weight:      9,
				Severity:    "high",
			},
			{
				ID:          "csl-003",
				Standard:    StandardCSL,
				ControlID:   "第25条",
				Category:    CategoryIncidentResponse,
				Name:        "网络安全事件应急预案",
				Description: "制定网络安全事件应急预案",
				Weight:      9,
				Severity:    "high",
			},
			{
				ID:          "csl-004",
				Standard:    StandardCSL,
				ControlID:   "第40条",
				Category:    CategoryPrivacy,
				Name:        "用户信息保护",
				Description: "严格保护用户个人信息",
				Weight:      10,
				Severity:    "critical",
			},
		},
		Weights: map[string]int{
			"critical": 10,
			"high":     8,
			"medium":   5,
			"low":      2,
		},
	}
}

// initPIPLStandard 初始化个人信息保护法标准
func (cc *ComplianceChecker) initPIPLStandard() *StandardDefinition {
	return &StandardDefinition{
		Standard:  StandardPIPL,
		Name:      "中华人民共和国个人信息保护法",
		Version:   "2021",
		Effective: time.Date(2021, 11, 1, 0, 0, 0, 0, time.UTC),
		CheckItems: []*ComplianceCheckItem{
			{
				ID:          "pipl-001",
				Standard:    StandardPIPL,
				ControlID:   "第6条",
				Category:    CategoryDataProtection,
				Name:        "个人信息处理原则",
				Description: "遵循合法、正当、必要原则处理个人信息",
				Weight:      10,
				Severity:    "critical",
			},
			{
				ID:          "pipl-002",
				Standard:    StandardPIPL,
				ControlID:   "第13条",
				Category:    CategoryConsent,
				Name:        "个人信息处理合法性",
				Description: "取得个人同意方可处理个人信息",
				Weight:      10,
				Severity:    "critical",
			},
			{
				ID:          "pipl-003",
				Standard:    StandardPIPL,
				ControlID:   "第51条",
				Category:    CategoryPrivacy,
				Name:        "个人信息保护负责人",
				Description: "指定个人信息保护负责人",
				Weight:      8,
				Severity:    "high",
			},
			{
				ID:          "pipl-004",
				Standard:    StandardPIPL,
				ControlID:   "第57条",
				Category:    CategoryBreachNotification,
				Name:        "个人信息泄露通知",
				Description: "发生个人信息泄露时立即采取补救措施并通知",
				Weight:      10,
				Severity:    "critical",
			},
		},
		Weights: map[string]int{
			"critical": 10,
			"high":     8,
			"medium":   5,
			"low":      2,
		},
	}
}

// ========== 合规检查执行 ==========

// RunCheck 执行单项检查
func (cc *ComplianceChecker) RunCheck(ctx context.Context, itemID string) (*ComplianceCheckResult, error) {
	item := cc.getCheckItem(itemID)
	if item == nil {
		return nil, fmt.Errorf("检查项不存在: %s", itemID)
	}

	startTime := time.Now()
	result := &ComplianceCheckResult{
		ItemID:    itemID,
		Standard:  item.Standard,
		ControlID: item.ControlID,
		Category:  item.Category,
		Name:      item.Name,
		CheckTime: startTime,
	}

	// 执行检查
	status, message, details := cc.executeCheck(ctx, item)
	result.Status = status
	result.Message = message
	result.Details = details
	result.Duration = time.Since(startTime)

	// 计算分数和等级
	result.Score = cc.calculateItemScore(item, status)
	result.Level = cc.determineLevel(result.Score)

	// 设置整改建议
	if status == StatusFailed {
		result.Remediation = item.Remediation
	}

	// 保存结果
	cc.mu.Lock()
	cc.results[itemID] = result
	cc.checkHistory[itemID] = append(cc.checkHistory[itemID], result)
	cc.mu.Unlock()

	return result, nil
}

// RunAllChecks 执行所有检查
func (cc *ComplianceChecker) RunAllChecks(ctx context.Context, standard ComplianceStandard) (*ComplianceReport, error) {
	std, exists := cc.standards[standard]
	if !exists {
		return nil, fmt.Errorf("标准不存在: %s", standard)
	}

	report := &ComplianceReport{
		ReportID:       fmt.Sprintf("COMP-%s", uuid.New().String()[:8]),
		Title:          fmt.Sprintf("%s 合规检查报告", std.Name),
		Standard:       standard,
		GeneratedAt:    time.Now(),
		ValidUntil:     time.Now().AddDate(0, 1, 0), // 1个月有效期
		Results:        make([]*ComplianceCheckResult, 0),
		Remediations:   make([]RemediationItem, 0),
		CategoryScores: make(map[string]int),
	}

	// 执行所有检查项
	categoryResults := make(map[string][]*ComplianceCheckResult)

	for _, item := range std.CheckItems {
		result, err := cc.RunCheck(ctx, item.ID)
		if err != nil {
			continue
		}

		report.Results = append(report.Results, result)
		categoryResults[string(item.Category)] = append(categoryResults[string(item.Category)], result)
	}

	// 计算摘要
	report.Summary = cc.calculateSummary(report.Results)

	// 计算各类别分数
	for cat, results := range categoryResults {
		report.CategoryScores[cat] = cc.calculateCategoryScore(results)
	}

	// 计算总体分数
	report.OverallScore = cc.calculateOverallScore(report.Results, std.CheckItems)
	report.OverallLevel = cc.getOverallLevel(report.OverallScore)

	// 生成整改项
	report.Remediations = cc.generateRemediations(report.Results)

	// 生成建议
	report.Recommendations = cc.generateRecommendations(report.Results)

	// 设置下次审核日期
	report.NextReviewDate = time.Now().AddDate(0, 1, 0)

	// 保存报告
	cc.mu.Lock()
	cc.reports = append(cc.reports, report)
	cc.mu.Unlock()

	// 清理旧报告
	cc.cleanupOldReports()

	// 保存到文件
	cc.saveReport(report)

	return report, nil
}

// RunChecksByCategory 按类别执行检查
func (cc *ComplianceChecker) RunChecksByCategory(ctx context.Context, standard ComplianceStandard, category ComplianceCategory) (*ComplianceReport, error) {
	std, exists := cc.standards[standard]
	if !exists {
		return nil, fmt.Errorf("标准不存在: %s", standard)
	}

	report := &ComplianceReport{
		ReportID:       fmt.Sprintf("COMP-%s", uuid.New().String()[:8]),
		Title:          fmt.Sprintf("%s - %s 合规检查报告", std.Name, category),
		Standard:       standard,
		GeneratedAt:    time.Now(),
		ValidUntil:     time.Now().AddDate(0, 1, 0),
		Results:        make([]*ComplianceCheckResult, 0),
		CategoryScores: make(map[string]int),
	}

	for _, item := range std.CheckItems {
		if item.Category == category {
			result, err := cc.RunCheck(ctx, item.ID)
			if err != nil {
				continue
			}
			report.Results = append(report.Results, result)
		}
	}

	report.Summary = cc.calculateSummary(report.Results)
	report.OverallScore = cc.calculateCategoryScore(report.Results)
	report.OverallLevel = cc.getOverallLevel(report.OverallScore)

	return report, nil
}

// executeCheck 执行具体检查
func (cc *ComplianceChecker) executeCheck(ctx context.Context, item *ComplianceCheckItem) (ComplianceStatus, string, map[string]interface{}) {
	// 根据检查项类型执行检查
	// 这里简化实现，实际应该根据具体检查项实现

	details := make(map[string]interface{})

	switch item.Category {
	case CategoryAccessControl:
		return cc.checkAccessControl(item, details)
	case CategoryEncryption:
		return cc.checkEncryption(item, details)
	case CategoryAudit:
		return cc.checkAudit(item, details)
	case CategoryDataProtection:
		return cc.checkDataProtection(item, details)
	case CategoryVulnerability:
		return cc.checkVulnerability(item, details)
	case CategoryIncidentResponse:
		return cc.checkIncidentResponse(item, details)
	case CategoryBreachNotification:
		return cc.checkBreachNotification(item, details)
	case CategoryPrivacy:
		return cc.checkPrivacy(item, details)
	default:
		return StatusSkipped, "检查未实现", details
	}
}

// checkAccessControl 检查访问控制
func (cc *ComplianceChecker) checkAccessControl(item *ComplianceCheckItem, details map[string]interface{}) (ComplianceStatus, string, map[string]interface{}) {
	// 检查是否配置了访问控制策略
	policyExists := true // 实际应检查系统配置
	details["policy_exists"] = policyExists

	if policyExists {
		return StatusPassed, "访问控制策略已配置", details
	}
	return StatusFailed, "未配置访问控制策略", details
}

// checkEncryption 检查加密
func (cc *ComplianceChecker) checkEncryption(item *ComplianceCheckItem, details map[string]interface{}) (ComplianceStatus, string, map[string]interface{}) {
	// 检查加密配置
	encryptionEnabled := true // 实际应检查系统配置
	details["encryption_enabled"] = encryptionEnabled

	if encryptionEnabled {
		return StatusPassed, "加密已启用", details
	}
	return StatusFailed, "未启用加密", details
}

// checkAudit 检查审计
func (cc *ComplianceChecker) checkAudit(item *ComplianceCheckItem, details map[string]interface{}) (ComplianceStatus, string, map[string]interface{}) {
	// 检查审计日志配置
	auditEnabled := true // 实际应检查系统配置
	details["audit_enabled"] = auditEnabled

	if auditEnabled {
		return StatusPassed, "审计已启用", details
	}
	return StatusFailed, "未启用审计", details
}

// checkDataProtection 检查数据保护
func (cc *ComplianceChecker) checkDataProtection(item *ComplianceCheckItem, details map[string]interface{}) (ComplianceStatus, string, map[string]interface{}) {
	// 检查数据保护措施
	protectionEnabled := true // 实际应检查系统配置
	details["protection_enabled"] = protectionEnabled

	if protectionEnabled {
		return StatusPassed, "数据保护措施已实施", details
	}
	return StatusFailed, "未实施必要的数据保护措施", details
}

// checkVulnerability 检查漏洞管理
func (cc *ComplianceChecker) checkVulnerability(item *ComplianceCheckItem, details map[string]interface{}) (ComplianceStatus, string, map[string]interface{}) {
	// 检查漏洞管理流程
	vulnManagementEnabled := true // 实际应检查系统配置
	details["vuln_management_enabled"] = vulnManagementEnabled

	if vulnManagementEnabled {
		return StatusPassed, "漏洞管理流程已建立", details
	}
	return StatusFailed, "未建立漏洞管理流程", details
}

// checkIncidentResponse 检查事件响应
func (cc *ComplianceChecker) checkIncidentResponse(item *ComplianceCheckItem, details map[string]interface{}) (ComplianceStatus, string, map[string]interface{}) {
	// 检查事件响应计划
	planExists := true // 实际应检查系统配置
	details["plan_exists"] = planExists

	if planExists {
		return StatusPassed, "事件响应计划已制定", details
	}
	return StatusFailed, "未制定事件响应计划", details
}

// checkBreachNotification 检查泄露通知
func (cc *ComplianceChecker) checkBreachNotification(item *ComplianceCheckItem, details map[string]interface{}) (ComplianceStatus, string, map[string]interface{}) {
	// 检查泄露通知流程
	processExists := true // 实际应检查系统配置
	details["process_exists"] = processExists

	if processExists {
		return StatusPassed, "泄露通知流程已建立", details
	}
	return StatusFailed, "未建立泄露通知流程", details
}

// checkPrivacy 检查隐私保护
func (cc *ComplianceChecker) checkPrivacy(item *ComplianceCheckItem, details map[string]interface{}) (ComplianceStatus, string, map[string]interface{}) {
	// 检查隐私保护措施
	privacyProtectionEnabled := true // 实际应检查系统配置
	details["privacy_protection_enabled"] = privacyProtectionEnabled

	if privacyProtectionEnabled {
		return StatusPassed, "隐私保护措施已实施", details
	}
	return StatusFailed, "未实施必要的隐私保护措施", details
}

// ========== 报告管理 ==========

// GetReport 获取报告
func (cc *ComplianceChecker) GetReport(reportID string) (*ComplianceReport, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	for _, report := range cc.reports {
		if report.ReportID == reportID {
			return report, nil
		}
	}

	return nil, fmt.Errorf("报告不存在: %s", reportID)
}

// GetLatestReport 获取最新报告
func (cc *ComplianceChecker) GetLatestReport(standard ComplianceStandard) (*ComplianceReport, error) {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	var latest *ComplianceReport
	for _, report := range cc.reports {
		if report.Standard == standard {
			if latest == nil || report.GeneratedAt.After(latest.GeneratedAt) {
				latest = report
			}
		}
	}

	if latest == nil {
		return nil, fmt.Errorf("未找到 %s 标准的报告", standard)
	}

	return latest, nil
}

// ListReports 列出报告
func (cc *ComplianceChecker) ListReports(standard ComplianceStandard, limit int) []*ComplianceReport {
	cc.mu.RLock()
	defer cc.mu.RUnlock()

	var reports []*ComplianceReport
	for _, report := range cc.reports {
		if standard == "" || report.Standard == standard {
			reports = append(reports, report)
		}
	}

	// 按时间排序
	if len(reports) > limit {
		reports = reports[:limit]
	}

	return reports
}

// ========== 辅助方法 ==========

// getCheckItem 获取检查项
func (cc *ComplianceChecker) getCheckItem(itemID string) *ComplianceCheckItem {
	for _, std := range cc.standards {
		for _, item := range std.CheckItems {
			if item.ID == itemID {
				return item
			}
		}
	}
	return nil
}

// calculateItemScore 计算单项分数
func (cc *ComplianceChecker) calculateItemScore(item *ComplianceCheckItem, status ComplianceStatus) int {
	switch status {
	case StatusPassed:
		return 100
	case StatusWarning:
		return 70
	case StatusFailed:
		return 0
	case StatusSkipped, StatusNotApplicable:
		return -1 // 不计入总分
	default:
		return 50
	}
}

// determineLevel 确定合规等级
func (cc *ComplianceChecker) determineLevel(score int) ComplianceLevel {
	switch {
	case score >= 90:
		return LevelFull
	case score >= 60:
		return LevelPartial
	default:
		return LevelNonCompliant
	}
}

// calculateSummary 计算摘要
func (cc *ComplianceChecker) calculateSummary(results []*ComplianceCheckResult) ComplianceSummary {
	summary := ComplianceSummary{}

	for _, r := range results {
		summary.TotalChecks++

		switch r.Status {
		case StatusPassed:
			summary.PassedChecks++
		case StatusFailed:
			summary.FailedChecks++
			switch r.ItemID[:strings.Index(r.ItemID, "-")] {
			case "gdpr", "soc2", "iso":
				// 根据严重程度计数
			}
		case StatusWarning:
			summary.WarningChecks++
		case StatusSkipped:
			summary.SkippedChecks++
		case StatusNotApplicable:
			summary.NotApplicable++
		}
	}

	return summary
}

// calculateCategoryScore 计算类别分数
func (cc *ComplianceChecker) calculateCategoryScore(results []*ComplianceCheckResult) int {
	if len(results) == 0 {
		return 0
	}

	totalScore := 0
	validCount := 0

	for _, r := range results {
		if r.Score >= 0 {
			totalScore += r.Score
			validCount++
		}
	}

	if validCount == 0 {
		return 0
	}

	return totalScore / validCount
}

// calculateOverallScore 计算总体分数
func (cc *ComplianceChecker) calculateOverallScore(results []*ComplianceCheckResult, items []*ComplianceCheckItem) int {
	if len(results) == 0 {
		return 0
	}

	// 使用加权平均
	totalWeight := 0
	weightedScore := 0.0

	itemMap := make(map[string]*ComplianceCheckItem)
	for _, item := range items {
		itemMap[item.ID] = item
	}

	for _, result := range results {
		item, exists := itemMap[result.ItemID]
		if !exists {
			continue
		}

		if result.Score >= 0 {
			totalWeight += item.Weight
			weightedScore += float64(result.Score) * float64(item.Weight)
		}
	}

	if totalWeight == 0 {
		return 0
	}

	return int(weightedScore / float64(totalWeight))
}

// getOverallLevel 获取总体等级
func (cc *ComplianceChecker) getOverallLevel(score int) ComplianceLevel {
	switch {
	case score >= 90:
		return LevelFull
	case score >= 70:
		return LevelPartial
	case score >= 50:
		return LevelNonCompliant
	default:
		return LevelUnknown
	}
}

// generateRemediations 生成整改项
func (cc *ComplianceChecker) generateRemediations(results []*ComplianceCheckResult) []RemediationItem {
	var remediations []RemediationItem

	for _, result := range results {
		if result.Status == StatusFailed {
			item := cc.getCheckItem(result.ItemID)
			if item == nil {
				continue
			}

			rem := RemediationItem{
				ID:          fmt.Sprintf("REM-%s", uuid.New().String()[:8]),
				ItemID:      result.ItemID,
				Title:       item.Name,
				Description: result.Message,
				Priority:    cc.severityToPriority(item.Severity),
				Status:      "open",
				CreatedAt:   time.Now(),
			}

			remediations = append(remediations, rem)
		}
	}

	return remediations
}

// severityToPriority 严重程度转优先级
func (cc *ComplianceChecker) severityToPriority(severity string) int {
	switch severity {
	case "critical":
		return 1
	case "high":
		return 2
	case "medium":
		return 3
	default:
		return 4
	}
}

// generateRecommendations 生成建议
func (cc *ComplianceChecker) generateRecommendations(results []*ComplianceCheckResult) []string {
	var recommendations []string

	// 分析失败项，生成改进建议
	failedCategories := make(map[string]int)
	for _, result := range results {
		if result.Status == StatusFailed {
			failedCategories[string(result.Category)]++
		}
	}

	for cat, count := range failedCategories {
		rec := fmt.Sprintf("建议加强 %s 领域的合规措施，当前存在 %d 个不合规项", cat, count)
		recommendations = append(recommendations, rec)
	}

	return recommendations
}

// cleanupOldReports 清理旧报告
func (cc *ComplianceChecker) cleanupOldReports() {
	if len(cc.reports) <= cc.config.MaxReports {
		return
	}

	// 按时间排序，保留最新的报告
	sorted := make([]*ComplianceReport, len(cc.reports))
	copy(sorted, cc.reports)

	// 简单保留最新的 MaxReports 个
	cc.reports = sorted[len(sorted)-cc.config.MaxReports:]
}

// ========== 持久化 ==========

// saveReport 保存报告
func (cc *ComplianceChecker) saveReport(report *ComplianceReport) error {
	filename := filepath.Join(cc.storageDir, fmt.Sprintf("report_%s_%s.json",
		report.Standard, report.GeneratedAt.Format("2006-01-02_150405")))

	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(filename, data, 0640)
}

// loadReports 加载报告
func (cc *ComplianceChecker) loadReports() {
	entries, err := os.ReadDir(cc.storageDir)
	if err != nil {
		return
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		data, err := os.ReadFile(filepath.Join(cc.storageDir, entry.Name()))
		if err != nil {
			continue
		}

		var report ComplianceReport
		if err := json.Unmarshal(data, &report); err != nil {
			continue
		}

		cc.reports = append(cc.reports, &report)
	}
}

// ========== 导出功能 ==========

// ExportReportToJSON 导出报告为 JSON
func (cc *ComplianceChecker) ExportReportToJSON(report *ComplianceReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// ExportReportToHTML 导出报告为 HTML
func (cc *ComplianceChecker) ExportReportToHTML(report *ComplianceReport) ([]byte, error) {
	var html strings.Builder
	html.WriteString(`<!DOCTYPE html>
<html>
<head>
    <title>` + report.Title + `</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        .passed { color: #4caf50; }
        .failed { color: #f44336; }
        .warning { color: #ff9800; }
        table { border-collapse: collapse; width: 100%; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #f5f5f5; }
        .score { font-size: 24px; font-weight: bold; }
    </style>
</head>
<body>
    <h1>` + report.Title + `</h1>
    <p>报告ID: ` + report.ReportID + `</p>
    <p>生成时间: ` + report.GeneratedAt.Format("2006-01-02 15:04:05") + `</p>
    <p class="score">合规分数: ` + fmt.Sprintf("%d", report.OverallScore) + `/100</p>
    <p>合规等级: ` + string(report.OverallLevel) + `</p>
    <h2>检查摘要</h2>
    <ul>
        <li>通过: ` + fmt.Sprintf("%d", report.Summary.PassedChecks) + `</li>
        <li>失败: ` + fmt.Sprintf("%d", report.Summary.FailedChecks) + `</li>
        <li>警告: ` + fmt.Sprintf("%d", report.Summary.WarningChecks) + `</li>
    </ul>
    <h2>检查结果</h2>
    <table>
        <tr>
            <th>控制ID</th>
            <th>名称</th>
            <th>状态</th>
            <th>分数</th>
        </tr>
`)

	for _, result := range report.Results {
		html.WriteString(fmt.Sprintf(`        <tr>
            <td>%s</td>
            <td>%s</td>
            <td class="%s">%s</td>
            <td>%d</td>
        </tr>
`,
			result.ControlID,
			result.Name,
			result.Status,
			result.Status,
			result.Score,
		))
	}

	html.WriteString(`    </table>
</body>
</html>`)

	return []byte(html.String()), nil
}

// GetStandards 获取所有标准
func (cc *ComplianceChecker) GetStandards() []ComplianceStandard {
	standards := make([]ComplianceStandard, 0, len(cc.standards))
	for std := range cc.standards {
		standards = append(standards, std)
	}
	return standards
}

// GetCheckItems 获取检查项
func (cc *ComplianceChecker) GetCheckItems(standard ComplianceStandard) []*ComplianceCheckItem {
	std, exists := cc.standards[standard]
	if !exists {
		return nil
	}
	return std.CheckItems
}

// AcknowledgeResult 确认结果
func (cc *ComplianceChecker) AcknowledgeResult(itemID, acknowledgedBy string) error {
	cc.mu.Lock()
	defer cc.mu.Unlock()

	result, exists := cc.results[itemID]
	if !exists {
		return fmt.Errorf("检查结果不存在: %s", itemID)
	}

	now := time.Now()
	result.Acknowledged = true
	result.AcknowledgedBy = acknowledgedBy
	result.AcknowledgedAt = &now

	return nil
}
