package stigcompliance

import (
	"fmt"
	"time"
)

// NewSTIGComplianceChecker 创建STIG合规检查器
func NewSTIGComplianceChecker(cfg CheckerConfig) *STIGComplianceChecker {
	checker := &STIGComplianceChecker{
		rules:   make(map[string]*STIGRule),
		config:  cfg,
	}
	checker.loadDefaultRules()
	return checker
}

// loadDefaultRules 加载默认STIG规则
func (c *STIGComplianceChecker) loadDefaultRules() {
	defaults := []*STIGRule{
		{ID: "V-250001", Title: "密码复杂度要求", Description: "系统必须启用密码复杂度策略", Severity: SeverityCat1, Category: "账户管理", Enabled: true},
		{ID: "V-250002", Title: "SSH协议版本", Description: "必须使用SSHv2", Severity: SeverityCat1, Category: "网络安全", Enabled: true},
		{ID: "V-250003", Title: "审计日志启用", Description: "必须启用系统审计日志", Severity: SeverityCat1, Category: "审计", Enabled: true},
		{ID: "V-250004", Title: "文件权限检查", Description: "关键文件权限必须符合标准", Severity: SeverityCat2, Category: "文件系统", Enabled: true},
		{ID: "V-250005", Title: "网络服务最小化", Description: "禁用不必要的网络服务", Severity: SeverityCat2, Category: "网络安全", Enabled: true},
		{ID: "V-250006", Title: "加密传输要求", Description: "数据传输必须使用TLS加密", Severity: SeverityCat1, Category: "加密", Enabled: true},
		{ID: "V-250007", Title: "访问控制列表", Description: "必须配置适当的ACL", Severity: SeverityCat2, Category: "访问控制", Enabled: true},
		{ID: "V-250008", Title: "系统更新检查", Description: "系统必须保持最新安全补丁", Severity: SeverityCat2, Category: "补丁管理", Enabled: true},
		{ID: "V-250009", Title: "备份验证", Description: "必须定期验证备份完整性", Severity: SeverityCat3, Category: "备份", Enabled: true},
		{ID: "V-250010", Title: "用户会话超时", Description: "空闲会话必须自动超时", Severity: SeverityCat3, Category: "会话管理", Enabled: true},
	}
	for _, rule := range defaults {
		c.rules[rule.ID] = rule
	}
}

// AddRule 添加规则
func (c *STIGComplianceChecker) AddRule(rule *STIGRule) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.rules[rule.ID]; exists {
		return ErrRuleExists
	}
	c.rules[rule.ID] = rule
	return nil
}

// RemoveRule 移除规则
func (c *STIGComplianceChecker) RemoveRule(ruleID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, exists := c.rules[ruleID]; !exists {
		return ErrRuleNotFound
	}
	delete(c.rules, ruleID)
	return nil
}

// GetRule 获取规则
func (c *STIGComplianceChecker) GetRule(ruleID string) (*STIGRule, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	rule, exists := c.rules[ruleID]
	if !exists {
		return nil, ErrRuleNotFound
	}
	return rule, nil
}

// ListRules 列出所有规则
func (c *STIGComplianceChecker) ListRules() []*STIGRule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]*STIGRule, 0, len(c.rules))
	for _, r := range c.rules {
		result = append(result, r)
	}
	return result
}

// RunAudit 执行审计
func (c *STIGComplianceChecker) RunAudit() *AuditReport {
	start := time.Now()

	c.mu.RLock()
	rules := make([]*STIGRule, 0, len(c.rules))
	for _, r := range c.rules {
		if r.Enabled {
			rules = append(rules, r)
		}
	}
	c.mu.RUnlock()

	results := make([]CheckResult, 0, len(rules))
	passed, failed := 0, 0

	for _, rule := range rules {
		result := c.checkRule(rule)
		results = append(results, result)
		if result.Status == CheckPass {
			passed++
		} else if result.Status == CheckFail {
			failed++
		}
	}

	score := float64(0)
	if len(rules) > 0 {
		score = float64(passed) / float64(len(rules)) * 100
	}

	report := &AuditReport{
		ID:          fmt.Sprintf("audit-%d", time.Now().UnixNano()),
		TotalRules:  len(rules),
		Passed:      passed,
		Failed:      failed,
		Score:       score,
		Results:     results,
		GeneratedAt: time.Now(),
		Duration:    time.Since(start).Milliseconds(),
	}

	c.mu.Lock()
	c.reports = append(c.reports, report)
	c.mu.Unlock()

	return report
}

func (c *STIGComplianceChecker) checkRule(rule *STIGRule) CheckResult {
	// 模拟检查逻辑
	return CheckResult{
		RuleID:    rule.ID,
		RuleTitle: rule.Title,
		Status:    CheckPass,
		Severity:  rule.Severity,
		Message:   fmt.Sprintf("规则 %s 检查通过", rule.ID),
		CheckedAt: time.Now(),
	}
}

// GetLatestReport 获取最新报告
func (c *STIGComplianceChecker) GetLatestReport() *AuditReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	if len(c.reports) == 0 {
		return nil
	}
	return c.reports[len(c.reports)-1]
}

// GetReportHistory 获取报告历史
func (c *STIGComplianceChecker) GetReportHistory() []*AuditReport {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.reports
}

// GetRuleCount 获取规则数量
func (c *STIGComplianceChecker) GetRuleCount() int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return len(c.rules)
}
