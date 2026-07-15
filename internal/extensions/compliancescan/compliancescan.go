// Package compliancescan 提供合规自检扫描器功能
// 对标 TrueNAS STIG/FIPS 合规扫描和 Synology 安全审计
package compliancescan

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Standard 合规标准
type Standard string

const (
	StandardSTIG     Standard = "STIG"
	StandardFIPS     Standard = "FIPS"
	StandardGDPR     Standard = "GDPR"
	StandardHIPAA    Standard = "HIPAA"
	StandardPCI      Standard = "PCI-DSS"
	StandardISO27001 Standard = "ISO27001"
	StandardSOC2     Standard = "SOC2"
	StandardNIST     Standard = "NIST"
)

// Severity 严重级别
type Severity string

const (
	SevCritical Severity = "critical"
	SevHigh     Severity = "high"
	SevMedium   Severity = "medium"
	SevLow      Severity = "low"
	SevInfo     Severity = "info"
)

// CheckStatus 检查状态
type CheckStatus string

const (
	StatusPass   CheckStatus = "pass"
	StatusFail   CheckStatus = "fail"
	StatusWarn   CheckStatus = "warn"
	StatusSkip   CheckStatus = "skip"
	StatusManual CheckStatus = "manual"
)

// Rule 合规规则
type Rule struct {
	ID          string    `json:"id"`
	Standard    Standard  `json:"standard"`
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Severity    Severity  `json:"severity"`
	Category    string    `json:"category"`
	Check       CheckFunc `json:"-"`
	Remediation string    `json:"remediation"`
	References  []string  `json:"references,omitempty"`
}

// CheckFunc 检查函数
type CheckFunc func(ctx *ScanContext) *CheckResult

// CheckResult 检查结果
type CheckResult struct {
	Status  CheckStatus `json:"status"`
	Message string      `json:"message"`
	Detail  string      `json:"detail,omitempty"`
}

// ScanContext 扫描上下文
type ScanContext struct {
	EncryptionEnabled bool
	MFAEnabled        bool
	AuditEnabled      bool
	PasswordMinLen    int
	FirewallEnabled   bool
	SSHEnabled        bool
	TLSEnabled        bool
	WORMEnabled       bool
	BackupEnabled     bool
	SnapshotEnabled   bool
	RBACEnabled       bool
}

// ScanReport 扫描报告
type ScanReport struct {
	Standard    Standard                    `json:"standard"`
	Timestamp   time.Time                   `json:"timestamp"`
	TotalRules  int                         `json:"total_rules"`
	Passed      int                         `json:"passed"`
	Failed      int                         `json:"failed"`
	Warnings    int                         `json:"warnings"`
	Skipped     int                         `json:"skipped"`
	Score       int                         `json:"score"`
	Results     []*ScanResult               `json:"results"`
	Categories  map[string]*CategorySummary `json:"categories"`
	GeneratedAt time.Time                   `json:"generated_at"`
}

// ScanResult 扫描结果
type ScanResult struct {
	Rule        *Rule        `json:"rule"`
	Result      *CheckResult `json:"result"`
	EvaluatedAt time.Time    `json:"evaluated_at"`
}

// CategorySummary 分类汇总
type CategorySummary struct {
	Total   int `json:"total"`
	Passed  int `json:"passed"`
	Failed  int `json:"failed"`
	Warning int `json:"warning"`
}

// Scanner 扫描器
type Scanner struct {
	mu      sync.RWMutex
	rules   map[Standard][]*Rule
	context *ScanContext
}

// NewScanner 创建扫描器
func NewScanner() *Scanner {
	s := &Scanner{
		rules: make(map[Standard][]*Rule),
		context: &ScanContext{
			EncryptionEnabled: true,
			MFAEnabled:        true,
			AuditEnabled:      true,
			PasswordMinLen:    12,
			FirewallEnabled:   true,
			SSHEnabled:        true,
			TLSEnabled:        true,
		},
	}
	s.initDefaultRules()
	return s
}

// SetContext 设置扫描上下文
func (s *Scanner) SetContext(ctx *ScanContext) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.context = ctx
}

// initDefaultRules 初始化默认规则
func (s *Scanner) initDefaultRules() {
	s.rules[StandardSTIG] = []*Rule{
		{
			ID: "STIG-001", Standard: StandardSTIG, Title: "加密静态数据",
			Description: "确保所有存储卷已加密", Severity: SevCritical,
			Category: "加密", Remediation: "启用 ZFS 加密或 LUKS",
			Check: func(ctx *ScanContext) *CheckResult {
				if !ctx.EncryptionEnabled {
					return &CheckResult{Status: StatusFail, Message: "存储加密未启用"}
				}
				return &CheckResult{Status: StatusPass, Message: "存储加密已启用"}
			},
		},
		{
			ID: "STIG-002", Standard: StandardSTIG, Title: "多因素认证",
			Description: "确保 MFA 已启用", Severity: SevHigh,
			Category: "认证", Remediation: "启用 TOTP 或 FIDO2 认证",
			Check: func(ctx *ScanContext) *CheckResult {
				if !ctx.MFAEnabled {
					return &CheckResult{Status: StatusFail, Message: "MFA 未启用"}
				}
				return &CheckResult{Status: StatusPass, Message: "MFA 已启用"}
			},
		},
		{
			ID: "STIG-003", Standard: StandardSTIG, Title: "审计日志",
			Description: "确保审计日志已启用并记录关键操作", Severity: SevHigh,
			Category: "审计", Remediation: "启用系统审计日志",
			Check: func(ctx *ScanContext) *CheckResult {
				if !ctx.AuditEnabled {
					return &CheckResult{Status: StatusFail, Message: "审计日志未启用"}
				}
				return &CheckResult{Status: StatusPass, Message: "审计日志已启用"}
			},
		},
		{
			ID: "STIG-004", Standard: StandardSTIG, Title: "密码策略",
			Description: "密码最小长度不小于 14 字符", Severity: SevMedium,
			Category: "认证", Remediation: "设置密码策略 minlen=14",
			Check: func(ctx *ScanContext) *CheckResult {
				if ctx.PasswordMinLen < 14 {
					return &CheckResult{Status: StatusFail, Message: fmt.Sprintf("密码最小长度 %d, 应为 14+", ctx.PasswordMinLen)}
				}
				return &CheckResult{Status: StatusPass, Message: "密码策略符合要求"}
			},
		},
		{
			ID: "STIG-005", Standard: StandardSTIG, Title: "防火墙",
			Description: "确保防火墙已启用", Severity: SevHigh,
			Category: "网络安全", Remediation: "启用 firewall",
			Check: func(ctx *ScanContext) *CheckResult {
				if !ctx.FirewallEnabled {
					return &CheckResult{Status: StatusFail, Message: "防火墙未启用"}
				}
				return &CheckResult{Status: StatusPass, Message: "防火墙已启用"}
			},
		},
	}

	s.rules[StandardFIPS] = []*Rule{
		{
			ID: "FIPS-001", Standard: StandardFIPS, Title: "FIPS 加密模块",
			Description: "确保使用 FIPS 认证的加密模块", Severity: SevCritical,
			Category: "加密", Remediation: "安装并启用 FIPS 认证的加密库",
			Check: func(ctx *ScanContext) *CheckResult {
				if !ctx.EncryptionEnabled {
					return &CheckResult{Status: StatusFail, Message: "加密未启用"}
				}
				return &CheckResult{Status: StatusPass, Message: "加密模块已启用"}
			},
		},
		{
			ID: "FIPS-002", Standard: StandardFIPS, Title: "TLS 配置",
			Description: "确保 TLS 1.2+ 已启用", Severity: SevHigh,
			Category: "网络安全", Remediation: "禁用 TLS 1.0/1.1",
			Check: func(ctx *ScanContext) *CheckResult {
				if !ctx.TLSEnabled {
					return &CheckResult{Status: StatusFail, Message: "TLS 未启用"}
				}
				return &CheckResult{Status: StatusPass, Message: "TLS 已启用"}
			},
		},
	}

	s.rules[StandardGDPR] = []*Rule{
		{
			ID: "GDPR-001", Standard: StandardGDPR, Title: "数据加密",
			Description: "确保个人数据已加密存储", Severity: SevHigh,
			Category: "数据保护", Remediation: "启用存储加密",
			Check: func(ctx *ScanContext) *CheckResult {
				if !ctx.EncryptionEnabled {
					return &CheckResult{Status: StatusFail, Message: "数据加密未启用"}
				}
				return &CheckResult{Status: StatusPass, Message: "数据加密已启用"}
			},
		},
		{
			ID: "GDPR-002", Standard: StandardGDPR, Title: "访问控制",
			Description: "确保 RBAC 已启用", Severity: SevHigh,
			Category: "访问控制", Remediation: "启用 RBAC",
			Check: func(ctx *ScanContext) *CheckResult {
				if !ctx.RBACEnabled {
					return &CheckResult{Status: StatusWarn, Message: "RBAC 未确认启用"}
				}
				return &CheckResult{Status: StatusPass, Message: "RBAC 已启用"}
			},
		},
		{
			ID: "GDPR-003", Standard: StandardGDPR, Title: "备份策略",
			Description: "确保定期备份已配置", Severity: SevMedium,
			Category: "数据保护", Remediation: "配置定期备份任务",
			Check: func(ctx *ScanContext) *CheckResult {
				if !ctx.BackupEnabled {
					return &CheckResult{Status: StatusWarn, Message: "备份未确认"}
				}
				return &CheckResult{Status: StatusPass, Message: "备份已配置"}
			},
		},
	}
}

// RunScan 运行扫描
func (s *Scanner) RunScan(standard Standard) *ScanReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rules, exists := s.rules[standard]
	if !exists {
		return &ScanReport{
			Standard:   standard,
			Timestamp:  time.Now(),
			TotalRules: 0,
			Results:    make([]*ScanResult, 0),
			Categories: make(map[string]*CategorySummary),
		}
	}

	report := &ScanReport{
		Standard:    standard,
		Timestamp:   time.Now(),
		GeneratedAt: time.Now(),
		TotalRules:  len(rules),
		Results:     make([]*ScanResult, 0, len(rules)),
		Categories:  make(map[string]*CategorySummary),
	}

	for _, rule := range rules {
		result := rule.Check(s.context)
		sr := &ScanResult{
			Rule:        rule,
			Result:      result,
			EvaluatedAt: time.Now(),
		}
		report.Results = append(report.Results, sr)

		cat := rule.Category
		if report.Categories[cat] == nil {
			report.Categories[cat] = &CategorySummary{}
		}
		report.Categories[cat].Total++
		switch result.Status {
		case StatusPass:
			report.Passed++
			report.Categories[cat].Passed++
		case StatusFail:
			report.Failed++
			report.Categories[cat].Failed++
		case StatusWarn:
			report.Warnings++
			report.Categories[cat].Warning++
		case StatusSkip:
			report.Skipped++
		}
	}

	// 计算评分
	total := report.Passed + report.Failed + report.Warnings
	if total > 0 {
		report.Score = (report.Passed * 100) / total
	}

	return report
}

// RunAllScans 运行所有标准扫描
func (s *Scanner) RunAllScans() map[Standard]*ScanReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	results := make(map[Standard]*ScanReport)
	for std := range s.rules {
		results[std] = s.RunScan(std)
	}
	return results
}

// ListStandards 列出可用标准
func (s *Scanner) ListStandards() []Standard {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var standards []Standard
	for std := range s.rules {
		standards = append(standards, std)
	}
	sort.Slice(standards, func(i, j int) bool {
		return string(standards[i]) < string(standards[j])
	})
	return standards
}

// FormatReport 格式化报告
func (s *Scanner) FormatReport(report *ScanReport) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("合规扫描报告 [%s]:\n", report.Standard))
	sb.WriteString(strings.Repeat("═", 50) + "\n")
	sb.WriteString(fmt.Sprintf("评分: %d/100\n", report.Score))
	sb.WriteString(fmt.Sprintf("通过: %d / 失败: %d / 警告: %d / 跳过: %d\n\n",
		report.Passed, report.Failed, report.Warnings, report.Skipped))

	// 按分类汇总
	sb.WriteString("分类汇总:\n")
	for cat, summary := range report.Categories {
		sb.WriteString(fmt.Sprintf("  %-15s 通过 %d/%d\n", cat, summary.Passed, summary.Total))
	}

	// 详细结果
	sb.WriteString("\n详细结果:\n")
	for _, result := range report.Results {
		icon := "✅"
		switch result.Result.Status {
		case StatusFail:
			icon = "❌"
		case StatusWarn:
			icon = "⚠️"
		case StatusSkip:
			icon = "⏭️"
		}
		sb.WriteString(fmt.Sprintf("  %s [%s] %s: %s\n",
			icon, result.Rule.ID, result.Rule.Title, result.Result.Message))
		if result.Rule.Remediation != "" && result.Result.Status == StatusFail {
			sb.WriteString(fmt.Sprintf("     修复建议: %s\n", result.Rule.Remediation))
		}
	}

	return sb.String()
}
