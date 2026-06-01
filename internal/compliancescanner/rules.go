// Package compliancescanner 提供安全合规扫描功能
package compliancescanner

import (
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// RuleManager 规则管理器.
type RuleManager struct {
	mu          sync.RWMutex
	rules       map[string]*ScanRule
	rulesByStd  map[ComplianceStandard][]*ScanRule
	rulesByCat  map[ScanCategory][]*ScanRule
	logger      *zap.Logger
	version     string
	updatedAt   time.Time
}

// NewRuleManager 创建规则管理器.
func NewRuleManager(logger *zap.Logger) *RuleManager {
	if logger == nil {
		logger = zap.NewNop()
	}

	rm := &RuleManager{
		rules:      make(map[string]*ScanRule),
		rulesByStd: make(map[ComplianceStandard][]*ScanRule),
		rulesByCat: make(map[ScanCategory][]*ScanRule),
		logger:     logger,
		version:    "1.0.0",
		updatedAt:  time.Now(),
	}

	// 加载内置规则
	rm.loadBuiltinRules()

	return rm
}

// loadBuiltinRules 加载内置规则.
func (rm *RuleManager) loadBuiltinRules() {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	now := time.Now()

	// CIS 基准规则
	cisRules := []*ScanRule{
		// 系统配置
		{
			ID:                "CIS-1.1.1",
			Name:              "文件系统挂载检查",
			Description:       "检查 /tmp 分区是否独立挂载",
			Standard:          StandardCIS,
			Category:          CategorySystemConfig,
			Severity:          SeverityHigh,
			CheckFunc:         "checkTmpMount",
			RemediationAdvice: "配置 /tmp 为独立分区",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "CIS-1.1.2",
			Name:              "文件系统权限检查",
			Description:       "检查 /tmp 目录权限是否为 1777",
			Standard:          StandardCIS,
			Category:          CategorySystemConfig,
			Severity:          SeverityMedium,
			CheckFunc:         "checkTmpPermissions",
			RemediationAdvice: "设置 /tmp 权限为 1777",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "CIS-1.3.1",
			Name:              "引导加载器配置检查",
			Description:       "检查引导加载器配置文件权限",
			Standard:          StandardCIS,
			Category:          CategorySystemConfig,
			Severity:          SeverityCritical,
			CheckFunc:         "checkBootloaderConfig",
			RemediationAdvice: "设置 GRUB 配置文件权限为 600",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		// 文件权限
		{
			ID:                "CIS-6.1.1",
			Name:              "关键文件权限检查",
			Description:       "检查 /etc/passwd 文件权限",
			Standard:          StandardCIS,
			Category:          CategoryFilePermission,
			Severity:          SeverityCritical,
			CheckFunc:         "checkPasswdPermissions",
			RemediationFunc:   "fixPasswdPermissions",
			RemediationAdvice: "设置 /etc/passwd 权限为 644",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "CIS-6.1.2",
			Name:              "密码文件权限检查",
			Description:       "检查 /etc/shadow 文件权限",
			Standard:          StandardCIS,
			Category:          CategoryFilePermission,
			Severity:          SeverityCritical,
			CheckFunc:         "checkShadowPermissions",
			RemediationFunc:   "fixShadowPermissions",
			RemediationAdvice: "设置 /etc/shadow 权限为 640",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "CIS-6.1.3",
			Name:              "SUID文件检查",
			Description:       "检查系统中的SUID文件",
			Standard:          StandardCIS,
			Category:          CategoryFilePermission,
			Severity:          SeverityHigh,
			CheckFunc:         "checkSUIDFiles",
			RemediationAdvice: "移除不必要的SUID权限",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "CIS-6.1.4",
			Name:              "SGID文件检查",
			Description:       "检查系统中的SGID文件",
			Standard:          StandardCIS,
			Category:          CategoryFilePermission,
			Severity:          SeverityHigh,
			CheckFunc:         "checkSGIDFiles",
			RemediationAdvice: "移除不必要的SGID权限",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		// 网络安全
		{
			ID:                "CIS-3.1.1",
			Name:              "IP转发检查",
			Description:       "检查IP转发是否禁用",
			Standard:          StandardCIS,
			Category:          CategoryNetworkSecurity,
			Severity:          SeverityHigh,
			CheckFunc:         "checkIPForward",
			RemediationFunc:   "disableIPForward",
			RemediationAdvice: "在 /etc/sysctl.conf 中设置 net.ipv4.ip_forward = 0",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "CIS-3.2.1",
			Name:              "防火墙配置检查",
			Description:       "检查防火墙是否启用",
			Standard:          StandardCIS,
			Category:          CategoryNetworkSecurity,
			Severity:          SeverityCritical,
			CheckFunc:         "checkFirewall",
			RemediationAdvice: "启用并配置防火墙",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "CIS-3.3.1",
			Name:              "开放端口检查",
			Description:       "检查不必要的开放端口",
			Standard:          StandardCIS,
			Category:          CategoryNetworkSecurity,
			Severity:          SeverityMedium,
			CheckFunc:         "checkOpenPorts",
			RemediationAdvice: "关闭不必要的网络端口",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		// 服务安全
		{
			ID:                "CIS-2.1.1",
			Name:              "不必要服务检查",
			Description:       "检查不必要的服务是否禁用",
			Standard:          StandardCIS,
			Category:          CategoryServiceSecurity,
			Severity:          SeverityMedium,
			CheckFunc:         "checkUnnecessaryServices",
			RemediationAdvice: "禁用不必要的服务",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "CIS-2.2.1",
			Name:              "SSH配置检查",
			Description:       "检查SSH服务安全配置",
			Standard:          StandardCIS,
			Category:          CategoryServiceSecurity,
			Severity:          SeverityCritical,
			CheckFunc:         "checkSSHConfig",
			RemediationAdvice: "配置SSH安全参数",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		// 加密合规
		{
			ID:                "CIS-4.1.1",
			Name:              "TLS版本检查",
			Description:       "检查系统TLS版本是否安全",
			Standard:          StandardCIS,
			Category:          CategoryCryptoCompliance,
			Severity:          SeverityCritical,
			CheckFunc:         "checkTLSVersion",
			RemediationAdvice: "禁用TLS 1.0和1.1，启用TLS 1.2+",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "CIS-4.1.2",
			Name:              "密码套件检查",
			Description:       "检查使用的密码套件是否安全",
			Standard:          StandardCIS,
			Category:          CategoryCryptoCompliance,
			Severity:          SeverityHigh,
			CheckFunc:         "checkCipherSuites",
			RemediationAdvice: "禁用弱密码套件",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "CIS-4.2.1",
			Name:              "SSH密钥算法检查",
			Description:       "检查SSH使用的密钥算法",
			Standard:          StandardCIS,
			Category:          CategoryCryptoCompliance,
			Severity:          SeverityHigh,
			CheckFunc:         "checkSSHKeyAlgorithms",
			RemediationAdvice: "使用安全的SSH密钥算法",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}

	// 等保2.0规则
	mlps2Rules := []*ScanRule{
		// 系统配置
		{
			ID:                "MLPS2-8.1.1.1",
			Name:              "身份鉴别检查",
			Description:       "检查系统身份鉴别机制",
			Standard:          StandardMLPS2,
			Category:          CategorySystemConfig,
			Severity:          SeverityCritical,
			CheckFunc:         "checkIdentityAuth",
			RemediationAdvice: "配置多因素身份鉴别",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "MLPS2-8.1.1.2",
			Name:              "登录失败处理检查",
			Description:       "检查登录失败处理机制",
			Standard:          StandardMLPS2,
			Category:          CategorySystemConfig,
			Severity:          SeverityHigh,
			CheckFunc:         "checkLoginFailure",
			RemediationAdvice: "配置登录失败锁定策略",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "MLPS2-8.1.2.1",
			Name:              "访问控制检查",
			Description:       "检查访问控制策略",
			Standard:          StandardMLPS2,
			Category:          CategorySystemConfig,
			Severity:          SeverityCritical,
			CheckFunc:         "checkAccessControl",
			RemediationAdvice: "配置最小权限访问控制",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "MLPS2-8.1.3.1",
			Name:              "安全审计检查",
			Description:       "检查安全审计策略",
			Standard:          StandardMLPS2,
			Category:          CategorySystemConfig,
			Severity:          SeverityCritical,
			CheckFunc:         "checkAuditPolicy",
			RemediationAdvice: "配置完善的安全审计策略",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		// 文件权限
		{
			ID:                "MLPS2-8.1.4.1",
			Name:              "重要文件保护检查",
			Description:       "检查重要文件是否受到保护",
			Standard:          StandardMLPS2,
			Category:          CategoryFilePermission,
			Severity:          SeverityCritical,
			CheckFunc:         "checkImportantFiles",
			RemediationAdvice: "设置重要文件的访问权限",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "MLPS2-8.1.4.2",
			Name:              "用户数据保护检查",
			Description:       "检查用户数据保护措施",
			Standard:          StandardMLPS2,
			Category:          CategoryFilePermission,
			Severity:          SeverityHigh,
			CheckFunc:         "checkUserDataProtection",
			RemediationAdvice: "实施用户数据加密和访问控制",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		// 网络安全
		{
			ID:                "MLPS2-8.2.1.1",
			Name:              "网络安全架构检查",
			Description:       "检查网络安全架构设计",
			Standard:          StandardMLPS2,
			Category:          CategoryNetworkSecurity,
			Severity:          SeverityCritical,
			CheckFunc:         "checkNetworkArchitecture",
			RemediationAdvice: "实施网络安全域划分",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "MLPS2-8.2.2.1",
			Name:              "边界防护检查",
			Description:       "检查网络边界防护措施",
			Standard:          StandardMLPS2,
			Category:          CategoryNetworkSecurity,
			Severity:          SeverityCritical,
			CheckFunc:         "checkBoundaryProtection",
			RemediationAdvice: "配置网络边界防护设备",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		// 服务安全
		{
			ID:                "MLPS2-8.3.1.1",
			Name:              "恶意代码防范检查",
			Description:       "检查恶意代码防范措施",
			Standard:          StandardMLPS2,
			Category:          CategoryServiceSecurity,
			Severity:          SeverityCritical,
			CheckFunc:         "checkMalwareProtection",
			RemediationAdvice: "部署恶意代码防范系统",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		{
			ID:                "MLPS2-8.3.2.1",
			Name:              "数据完整性检查",
			Description:       "检查数据完整性保护措施",
			Standard:          StandardMLPS2,
			Category:          CategoryServiceSecurity,
			Severity:          SeverityHigh,
			CheckFunc:         "checkDataIntegrity",
			RemediationAdvice: "实施数据完整性校验机制",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
		// 加密合规
		{
			ID:                "MLPS2-8.4.1.1",
			Name:              "密码使用合规检查",
			Description:       "检查密码使用是否符合要求",
			Standard:          StandardMLPS2,
			Category:          CategoryCryptoCompliance,
			Severity:          SeverityCritical,
			CheckFunc:         "checkCryptoCompliance",
			RemediationAdvice: "使用符合国密标准的密码算法",
			Enabled:           true,
			CreatedAt:         now,
			UpdatedAt:         now,
		},
	}

	// 加载所有规则
	allRules := append(cisRules, mlps2Rules...)
	for _, rule := range allRules {
		rm.rules[rule.ID] = rule
		rm.rulesByStd[rule.Standard] = append(rulesByStd(rule.Standard, rm.rulesByStd), rule)
		rm.rulesByCat[rule.Category] = append(rulesByCat(rule.Category, rm.rulesByCat), rule)
	}

	rm.logger.Info("加载内置规则完成",
		zap.Int("cis_rules", len(cisRules)),
		zap.Int("mlps2_rules", len(mlps2Rules)),
		zap.Int("total_rules", len(allRules)),
	)
}

// rulesByStd 辅助函数：获取指定标准的规则切片.
func rulesByStd(std ComplianceStandard, m map[ComplianceStandard][]*ScanRule) []*ScanRule {
	if v, ok := m[std]; ok {
		return v
	}
	return make([]*ScanRule, 0)
}

// rulesByCat 辅助函数：获取指定类别的规则切片.
func rulesByCat(cat ScanCategory, m map[ScanCategory][]*ScanRule) []*ScanRule {
	if v, ok := m[cat]; ok {
		return v
	}
	return make([]*ScanRule, 0)
}

// GetRule 获取规则.
func (rm *RuleManager) GetRule(id string) (*ScanRule, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rule, exists := rm.rules[id]
	if !exists {
		return nil, ErrRuleNotFound
	}
	return rule, nil
}

// GetAllRules 获取所有规则.
func (rm *RuleManager) GetAllRules() []*ScanRule {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rules := make([]*ScanRule, 0, len(rm.rules))
	for _, rule := range rm.rules {
		rules = append(rules, rule)
	}
	return rules
}

// GetRulesByStandard 获取指定标准的规则.
func (rm *RuleManager) GetRulesByStandard(std ComplianceStandard) []*ScanRule {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.rulesByStd[std]
}

// GetRulesByCategory 获取指定类别的规则.
func (rm *RuleManager) GetRulesByCategory(cat ScanCategory) []*ScanRule {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.rulesByCat[cat]
}

// GetEnabledRules 获取启用的规则.
func (rm *RuleManager) GetEnabledRules() []*ScanRule {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	rules := make([]*ScanRule, 0)
	for _, rule := range rm.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	return rules
}

// AddRule 添加规则.
func (rm *RuleManager) AddRule(rule *ScanRule) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rule.ID == "" {
		return ErrInvalidConfig
	}

	if _, exists := rm.rules[rule.ID]; exists {
		return ErrInvalidConfig
	}

	now := time.Now()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	rm.rules[rule.ID] = rule
	rm.rulesByStd[rule.Standard] = append(rm.rulesByStd[rule.Standard], rule)
	rm.rulesByCat[rule.Category] = append(rm.rulesByCat[rule.Category], rule)

	rm.logger.Info("添加规则", zap.String("rule_id", rule.ID), zap.String("rule_name", rule.Name))
	return nil
}

// UpdateRule 更新规则.
func (rm *RuleManager) UpdateRule(rule *ScanRule) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if rule.ID == "" {
		return ErrInvalidConfig
	}

	existing, exists := rm.rules[rule.ID]
	if !exists {
		return ErrRuleNotFound
	}

	// 更新索引
	rm.removeFromIndex(existing)

	rule.UpdatedAt = time.Now()
	rm.rules[rule.ID] = rule
	rm.rulesByStd[rule.Standard] = append(rm.rulesByStd[rule.Standard], rule)
	rm.rulesByCat[rule.Category] = append(rm.rulesByCat[rule.Category], rule)

	rm.logger.Info("更新规则", zap.String("rule_id", rule.ID))
	return nil
}

// DeleteRule 删除规则.
func (rm *RuleManager) DeleteRule(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rule, exists := rm.rules[id]
	if !exists {
		return ErrRuleNotFound
	}

	rm.removeFromIndex(rule)
	delete(rm.rules, id)

	rm.logger.Info("删除规则", zap.String("rule_id", id))
	return nil
}

// EnableRule 启用规则.
func (rm *RuleManager) EnableRule(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rule, exists := rm.rules[id]
	if !exists {
		return ErrRuleNotFound
	}

	rule.Enabled = true
	rule.UpdatedAt = time.Now()

	rm.logger.Info("启用规则", zap.String("rule_id", id))
	return nil
}

// DisableRule 禁用规则.
func (rm *RuleManager) DisableRule(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rule, exists := rm.rules[id]
	if !exists {
		return ErrRuleNotFound
	}

	rule.Enabled = false
	rule.UpdatedAt = time.Now()

	rm.logger.Info("禁用规则", zap.String("rule_id", id))
	return nil
}

// removeFromIndex 从索引中移除规则.
func (rm *RuleManager) removeFromIndex(rule *ScanRule) {
	// 从标准索引移除
	if rules, ok := rm.rulesByStd[rule.Standard]; ok {
		for i, r := range rules {
			if r.ID == rule.ID {
				rm.rulesByStd[rule.Standard] = append(rules[:i], rules[i+1:]...)
				break
			}
		}
	}

	// 从类别索引移除
	if rules, ok := rm.rulesByCat[rule.Category]; ok {
		for i, r := range rules {
			if r.ID == rule.ID {
				rm.rulesByCat[rule.Category] = append(rules[:i], rules[i+1:]...)
				break
			}
		}
	}
}

// GetRuleStats 获取规则统计.
func (rm *RuleManager) GetRuleStats() map[string]int {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	stats := map[string]int{
		"total":          len(rm.rules),
		"enabled":        0,
		"disabled":       0,
		"cis":            0,
		"mlps2":          0,
		"critical":       0,
		"high":           0,
		"medium":         0,
		"low":            0,
		"system_config":  0,
		"file_permission": 0,
		"network_security": 0,
		"service_security": 0,
		"crypto_compliance": 0,
	}

	for _, rule := range rm.rules {
		if rule.Enabled {
			stats["enabled"]++
		} else {
			stats["disabled"]++
		}

		switch rule.Standard {
		case StandardCIS:
			stats["cis"]++
		case StandardMLPS2:
			stats["mlps2"]++
		}

		switch rule.Severity {
		case SeverityCritical:
			stats["critical"]++
		case SeverityHigh:
			stats["high"]++
		case SeverityMedium:
			stats["medium"]++
		case SeverityLow:
			stats["low"]++
		}

		switch rule.Category {
		case CategorySystemConfig:
			stats["system_config"]++
		case CategoryFilePermission:
			stats["file_permission"]++
		case CategoryNetworkSecurity:
			stats["network_security"]++
		case CategoryServiceSecurity:
			stats["service_security"]++
		case CategoryCryptoCompliance:
			stats["crypto_compliance"]++
		}
	}

	return stats
}

// UpdateRuleVersion 更新规则版本.
func (rm *RuleManager) UpdateRuleVersion(version string) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	rm.version = version
	rm.updatedAt = time.Now()
}

// GetVersion 获取规则版本.
func (rm *RuleManager) GetVersion() string {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	return rm.version
}

// SearchRules 搜索规则.
func (rm *RuleManager) SearchRules(keyword string) []*ScanRule {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	results := make([]*ScanRule, 0)
	keyword = strings.ToLower(keyword)

	for _, rule := range rm.rules {
		if strings.Contains(strings.ToLower(rule.Name), keyword) ||
			strings.Contains(strings.ToLower(rule.Description), keyword) ||
			strings.Contains(strings.ToLower(rule.ID), keyword) {
			results = append(results, rule)
		}
	}

	return results
}

// strings 需要导入.
