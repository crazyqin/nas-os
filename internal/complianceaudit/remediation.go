// Package complianceaudit 提供自动修复建议功能
package complianceaudit

import (
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Remediator 自动修复建议器
type Remediator struct {
	logger     *zap.Logger
	strategies map[CheckCategory]*RemediationStrategy
}

// RemediationStrategy 修复策略
type RemediationStrategy struct {
	Category    CheckCategory     `json:"category"`
	AutoFixable bool              `json:"auto_fixable"`
	Steps       []RemediationStep `json:"steps"`
}

// RemediationStep 修复步骤
type RemediationStep struct {
	Order       int    `json:"order"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Command     string `json:"command,omitempty"`
	RiskLevel   string `json:"risk_level"` // low, medium, high
}

// NewRemediator 创建修复建议器
func NewRemediator(logger *zap.Logger) *Remediator {
	if logger == nil {
		logger = zap.NewNop()
	}

	rm := &Remediator{
		logger:     logger,
		strategies: make(map[CheckCategory]*RemediationStrategy),
	}

	rm.registerStrategies()
	return rm
}

// registerStrategies 注册修复策略
func (rm *Remediator) registerStrategies() {
	rm.strategies[CategoryPasswordPolicy] = &RemediationStrategy{
		Category:    CategoryPasswordPolicy,
		AutoFixable: false,
		Steps: []RemediationStep{
			{Order: 1, Title: "配置密码复杂度要求", Description: "编辑 /etc/pam.d/common-password，添加密码复杂度规则", RiskLevel: "low"},
			{Order: 2, Title: "设置密码有效期", Description: "编辑 /etc/login.defs，设置 PASS_MAX_DAYS=90", RiskLevel: "low"},
			{Order: 3, Title: "配置密码历史", Description: "设置 remember=5 防止重复使用最近5个密码", RiskLevel: "low"},
			{Order: 4, Title: "启用账户锁定", Description: "配置连续失败登录后的账户锁定策略", RiskLevel: "medium"},
		},
	}

	rm.strategies[CategoryAccessControl] = &RemediationStrategy{
		Category:    CategoryAccessControl,
		AutoFixable: false,
		Steps: []RemediationStep{
			{Order: 1, Title: "修复敏感目录权限", Description: "设置 /etc, /root 等目录的正确权限", RiskLevel: "low"},
			{Order: 2, Title: "禁用不必要的 SUID 文件", Description: "移除非标准 SUID 文件的特殊权限位", RiskLevel: "medium"},
			{Order: 3, Title: "加强 SSH 配置", Description: "禁用 root 直接登录，使用密钥认证", RiskLevel: "low"},
			{Order: 4, Title: "清理多余用户账户", Description: "禁用或删除不使用的系统账户", RiskLevel: "medium"},
		},
	}

	rm.strategies[CategoryEncryption] = &RemediationStrategy{
		Category:    CategoryEncryption,
		AutoFixable: false,
		Steps: []RemediationStep{
			{Order: 1, Title: "启用磁盘加密", Description: "使用 LUKS 加密敏感分区", RiskLevel: "high"},
			{Order: 2, Title: "配置 TLS 1.2+", Description: "禁用 SSLv3 和 TLS 1.0/1.1", RiskLevel: "low"},
			{Order: 3, Title: "更新 SSH 加密算法", Description: "使用强加密算法，移除弱密码套件", RiskLevel: "low"},
			{Order: 4, Title: "配置 HTTPS 重定向", Description: "确保所有 Web 服务使用 HTTPS", RiskLevel: "low"},
		},
	}

	rm.strategies[CategoryNetworkSecurity] = &RemediationStrategy{
		Category:    CategoryNetworkSecurity,
		AutoFixable: false,
		Steps: []RemediationStep{
			{Order: 1, Title: "配置防火墙规则", Description: "使用 iptables/nftables 配置入站规则", RiskLevel: "low"},
			{Order: 2, Title: "关闭不必要端口", Description: "停止未使用的服务，关闭对外暴露的端口", RiskLevel: "medium"},
			{Order: 3, Title: "配置网络分段", Description: "将敏感服务放在独立网络段", RiskLevel: "high"},
			{Order: 4, Title: "启用入侵检测", Description: "配置 IDS/IPS 监控异常流量", RiskLevel: "medium"},
		},
	}

	rm.strategies[CategoryAuditLog] = &RemediationStrategy{
		Category:    CategoryAuditLog,
		AutoFixable: false,
		Steps: []RemediationStep{
			{Order: 1, Title: "启用 auditd", Description: "启动并配置 Linux 审计守护进程", RiskLevel: "low"},
			{Order: 2, Title: "配置审计规则", Description: "添加关键文件和系统调用的审计规则", RiskLevel: "low"},
			{Order: 3, Title: "配置远程日志", Description: "设置日志转发到远程日志服务器", RiskLevel: "low"},
			{Order: 4, Title: "设置日志保留", Description: "配置日志轮转和保留至少 6 个月", RiskLevel: "low"},
		},
	}

	rm.strategies[CategoryDataProtection] = &RemediationStrategy{
		Category:    CategoryDataProtection,
		AutoFixable: false,
		Steps: []RemediationStep{
			{Order: 1, Title: "数据分类", Description: "对数据进行分类标记（公开/内部/机密/绝密）", RiskLevel: "medium"},
			{Order: 2, Title: "实施数据保留策略", Description: "定义数据生命周期和自动清理规则", RiskLevel: "medium"},
			{Order: 3, Title: "配置数据备份", Description: "实施 3-2-1 备份策略", RiskLevel: "medium"},
			{Order: 4, Title: "启用访问审计", Description: "记录敏感数据的访问和修改操作", RiskLevel: "low"},
		},
	}

	rm.strategies[CategoryIncidentResponse] = &RemediationStrategy{
		Category:    CategoryIncidentResponse,
		AutoFixable: false,
		Steps: []RemediationStep{
			{Order: 1, Title: "制定应急预案", Description: "编写安全事件响应流程文档", RiskLevel: "high"},
			{Order: 2, Title: "配置告警通知", Description: "设置安全事件的自动告警和通知机制", RiskLevel: "medium"},
			{Order: 3, Title: "建立响应团队", Description: "指定安全事件响应责任人和团队", RiskLevel: "high"},
			{Order: 4, Title: "定期演练", Description: "每季度进行安全事件响应演练", RiskLevel: "medium"},
		},
	}
}

// GetRemediation 获取检查结果的修复建议
func (rm *Remediator) GetRemediation(result *CheckResult) *Remediation {
	if result.Status != StatusFail {
		return nil
	}

	strategy, ok := rm.strategies[result.Category]
	if !ok {
		return &Remediation{
			Title:       fmt.Sprintf("修复 %s", result.Name),
			Description: result.Message,
			Steps:       []string{"请根据检查结果手动修复"},
			Priority:    riskToPriority(result.RiskLevel),
			Deadline:    riskToDeadline(result.RiskLevel),
		}
	}

	steps := make([]string, 0, len(strategy.Steps))
	for _, s := range strategy.Steps {
		steps = append(steps, fmt.Sprintf("%s: %s", s.Title, s.Description))
	}

	return &Remediation{
		Title:       fmt.Sprintf("修复 %s 问题", result.Category),
		Description: fmt.Sprintf("按照以下步骤修复 %s 类别的安全问题", result.Category),
		Steps:       steps,
		Priority:    riskToPriority(result.RiskLevel),
		Deadline:    riskToDeadline(result.RiskLevel),
	}
}

// GenerateRemediations 批量生成修复建议
func (rm *Remediator) GenerateRemediations(findings []*Finding) []*RemediationItem {
	items := make([]*RemediationItem, 0)

	for _, finding := range findings {
		if finding.Status != StatusFail {
			continue
		}

		item := &RemediationItem{
			FindingID:   finding.ID,
			Title:       fmt.Sprintf("修复: %s", finding.Title),
			Description: finding.Description,
			Status:      "pending",
			Priority:    riskToPriority(finding.RiskLevel),
			Deadline:    time.Now().AddDate(0, 0, riskToDeadline(finding.RiskLevel)),
		}

		// 获取具体步骤
		if strategy, ok := rm.strategies[finding.Category]; ok {
			steps := make([]string, 0, len(strategy.Steps))
			for _, s := range strategy.Steps {
				steps = append(steps, fmt.Sprintf("[%d] %s: %s", s.Order, s.Title, s.Description))
			}
			item.Steps = steps
		} else {
			item.Steps = []string{"请根据检查结果手动修复"}
		}

		items = append(items, item)
	}

	// 按优先级排序
	sortRemediations(items)

	return items
}

// GetAutoFixable 获取可自动修复的建议
func (rm *Remediator) GetAutoFixable(category CheckCategory) []RemediationStep {
	strategy, ok := rm.strategies[category]
	if !ok {
		return nil
	}

	if strategy.AutoFixable {
		return strategy.Steps
	}

	return nil
}

// GetStrategies 获取所有修复策略
func (rm *Remediator) GetStrategies() map[CheckCategory]*RemediationStrategy {
	return rm.strategies
}

// AddStrategy 添加自定义修复策略
func (rm *Remediator) AddStrategy(strategy *RemediationStrategy) {
	rm.strategies[strategy.Category] = strategy
	rm.logger.Info("added remediation strategy", zap.String("category", string(strategy.Category)))
}

// riskToPriority 风险等级转优先级
func riskToPriority(level RiskLevel) int {
	switch level {
	case RiskCritical:
		return 5
	case RiskHigh:
		return 4
	case RiskMedium:
		return 3
	case RiskLow:
		return 2
	default:
		return 1
	}
}

// riskToDeadline 风险等级转建议完成天数
func riskToDeadline(level RiskLevel) int {
	switch level {
	case RiskCritical:
		return 3
	case RiskHigh:
		return 7
	case RiskMedium:
		return 30
	case RiskLow:
		return 90
	default:
		return 30
	}
}

// sortRemediations 按优先级排序修复建议
func sortRemediations(items []*RemediationItem) {
	for i := 1; i < len(items); i++ {
		key := items[i]
		j := i - 1
		for j >= 0 && items[j].Priority < key.Priority {
			items[j+1] = items[j]
			j--
		}
		items[j+1] = key
	}
}
