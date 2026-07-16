package aideidentification

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// DeidentificationManager 脱敏管理器.
type DeidentificationManager struct {
	mu          sync.RWMutex
	config      DeidentificationConfig
	rules       map[string]*DeidentificationRule // 规则列表
	patterns    map[PIIType]*regexp.Regexp       // 编译后的正则
	stats       *DeidentificationStats           // 统计
	auditLog    []AuditEntry                     // 审计日志
	ruleCounter int                              // 规则计数器
}

// NewDeidentificationManager 创建脱敏管理器.
func NewDeidentificationManager(config *DeidentificationConfig) *DeidentificationManager {
	cfg := DefaultDeidentificationConfig()
	if config != nil {
		cfg = *config
	}

	m := &DeidentificationManager{
		config:   cfg,
		rules:    make(map[string]*DeidentificationRule),
		patterns: make(map[PIIType]*regexp.Regexp),
		stats: &DeidentificationStats{
			ByPIIType: make(map[PIIType]int),
			ByPolicy:  make(map[string]int),
		},
		auditLog: make([]AuditEntry, 0),
	}

	// 初始化内置规则
	m.initBuiltinRules()

	return m
}

// initBuiltinRules 初始化内置 PII 检测规则.
func (m *DeidentificationManager) initBuiltinRules() {
	builtinRules := []struct {
		name    string
		piiType PIIType
		pattern string
		policy  RedactionPolicy
	}{
		{
			name:    "中国手机号",
			piiType: PIITypePhone,
			pattern: `1[3-9]\d{9}`,
			policy:  PolicyPartial,
		},
		{
			name:    "电子邮箱",
			piiType: PIITypeEmail,
			pattern: `[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`,
			policy:  PolicyPartial,
		},
		{
			name:    "身份证号",
			piiType: PIITypeIDCard,
			pattern: `[1-9]\d{5}(18|19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]`,
			policy:  PolicyPartial,
		},
		{
			name:    "银行卡号",
			piiType: PIITypeBankCard,
			pattern: `[1-9]\d{15,18}`,
			policy:  PolicyPartial,
		},
		{
			name:    "IPv4地址",
			piiType: PIITypeIP,
			pattern: `((25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(25[0-5]|2[0-4]\d|[01]?\d\d?)`,
			policy:  PolicyMask,
		},
		{
			name:    "中国车牌号",
			piiType: PIITypeLicensePlate,
			pattern: `[京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤川青藏琼宁][A-HJ-NP-Z][A-HJ-NP-Z0-9]{4,5}[A-HJ-NP-Z0-9挂学警港澳]`,
			policy:  PolicyMask,
		},
		{
			name:    "护照号",
			piiType: PIITypePassport,
			pattern: `[A-Z]\d{8}`,
			policy:  PolicyMask,
		},
	}

	for i, rule := range builtinRules {
		m.ruleCounter++
		r := &DeidentificationRule{
			ID:          fmt.Sprintf("builtin_%d", i+1),
			Name:        rule.name,
			Description: "内置规则: " + rule.name,
			Enabled:     true,
			PIIType:     rule.piiType,
			Policy:      rule.policy,
			Pattern:     rule.pattern,
			Priority:    100 - i, // 内置规则优先级递减
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		m.rules[r.ID] = r

		// 编译正则
		if re, err := regexp.Compile(rule.pattern); err == nil {
			m.patterns[rule.piiType] = re
		}
	}
}

// CreateRule 创建脱敏规则.
func (m *DeidentificationManager) CreateRule(req *CreateRuleRequest) (*DeidentificationRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证正则表达式
	if _, err := regexp.Compile(req.Pattern); err != nil {
		return nil, fmt.Errorf("无效的正则表达式: %w", err)
	}

	m.ruleCounter++
	rule := &DeidentificationRule{
		ID:          fmt.Sprintf("custom_%d", m.ruleCounter),
		Name:        req.Name,
		Description: req.Description,
		Enabled:     true,
		PIIType:     req.PIIType,
		Policy:      req.Policy,
		Pattern:     req.Pattern,
		Placeholder: req.Placeholder,
		Priority:    req.Priority,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.rules[rule.ID] = rule

	// 编译正则
	if re, err := regexp.Compile(req.Pattern); err == nil {
		m.patterns[req.PIIType] = re
	}

	// 审计日志
	m.addAuditEntry("rule_create", rule.ID, rule.PIIType, 0, "api")

	return rule, nil
}

// UpdateRule 更新脱敏规则.
func (m *DeidentificationManager) UpdateRule(req *UpdateRuleRequest) (*DeidentificationRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[req.ID]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", req.ID)
	}

	// 更新字段
	if req.Name != "" {
		rule.Name = req.Name
	}
	if req.Description != "" {
		rule.Description = req.Description
	}
	if req.Enabled != nil {
		rule.Enabled = *req.Enabled
	}
	if req.Policy != "" {
		rule.Policy = req.Policy
	}
	if req.Pattern != "" {
		// 验证正则
		if _, err := regexp.Compile(req.Pattern); err != nil {
			return nil, fmt.Errorf("无效的正则表达式: %w", err)
		}
		rule.Pattern = req.Pattern
	}
	if req.Placeholder != "" {
		rule.Placeholder = req.Placeholder
	}
	if req.Priority != nil {
		rule.Priority = *req.Priority
	}

	rule.UpdatedAt = time.Now()

	// 重新编译正则
	if re, err := regexp.Compile(rule.Pattern); err == nil {
		m.patterns[rule.PIIType] = re
	}

	// 审计日志
	m.addAuditEntry("rule_update", rule.ID, rule.PIIType, 0, "api")

	return rule, nil
}

// DeleteRule 删除脱敏规则.
func (m *DeidentificationManager) DeleteRule(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[ruleID]
	if !exists {
		return fmt.Errorf("规则不存在: %s", ruleID)
	}

	// 内置规则不允许删除
	if strings.HasPrefix(ruleID, "builtin_") {
		return fmt.Errorf("内置规则不允许删除")
	}

	// 审计日志
	m.addAuditEntry("rule_delete", rule.ID, rule.PIIType, 0, "api")

	delete(m.rules, ruleID)
	return nil
}

// GetRule 获取规则.
func (m *DeidentificationManager) GetRule(ruleID string) (*DeidentificationRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[ruleID]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", ruleID)
	}

	return rule, nil
}

// ListRules 列出所有规则.
func (m *DeidentificationManager) ListRules() []DeidentificationRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]DeidentificationRule, 0, len(m.rules))
	for _, rule := range m.rules {
		rules = append(rules, *rule)
	}

	return rules
}

// Deidentify 执行脱敏.
func (m *DeidentificationManager) Deidentify(text string, ruleID string) (*DeidentificationResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := &DeidentificationResult{
		OriginalText: text,
		RedactedText: text,
		Redactions:   make([]RedactionResult, 0),
		ProcessedAt:  time.Now(),
	}

	// 获取要应用的规则
	rules := m.getEnabledRules(ruleID)
	if len(rules) == 0 {
		return result, nil
	}

	// 按优先级排序规则
	m.sortRulesByPriority(rules)

	// 应用每条规则
	redactedText := text
	offset := 0

	for _, rule := range rules {
		re, exists := m.patterns[rule.PIIType]
		if !exists {
			continue
		}

		// 查找所有匹配
		matches := re.FindAllStringIndex(redactedText, -1)
		if len(matches) == 0 {
			continue
		}

		// 从后向前替换，避免偏移问题
		for i := len(matches) - 1; i >= 0; i-- {
			start := matches[i][0]
			end := matches[i][1]
			original := redactedText[start:end]

			// 执行脱敏
			redacted := m.applyPolicy(original, rule)

			// 替换文本
			redactedText = redactedText[:start] + redacted + redactedText[end:]

			// 记录脱敏结果
			redaction := RedactionResult{
				RuleID:      rule.ID,
				PIIType:     rule.PIIType,
				Original:    original,
				Redacted:    redacted,
				Policy:      rule.Policy,
				StartOffset: start + offset,
				EndOffset:   end + offset,
			}
			result.Redactions = append(result.Redactions, redaction)
		}

		offset += len(redactedText) - len(text)
	}

	result.RedactedText = redactedText
	result.TotalRedacted = len(result.Redactions)

	// 更新统计
	m.updateStats(result)

	// 审计日志
	if m.config.AuditLog {
		m.addAuditEntry("deidentify", ruleID, "", result.TotalRedacted, "api")
	}

	return result, nil
}

// DeidentifyBatch 批量脱敏.
func (m *DeidentificationManager) DeidentifyBatch(req *BatchDeidentificationRequest) (*BatchDeidentificationResult, error) {
	results := make([]DeidentificationResult, 0, len(req.Texts))
	totalRedacted := 0

	for _, text := range req.Texts {
		result, err := m.Deidentify(text, req.RuleID)
		if err != nil {
			return nil, fmt.Errorf("批量脱敏失败: %w", err)
		}
		results = append(results, *result)
		totalRedacted += result.TotalRedacted
	}

	avgRedactions := 0.0
	if len(results) > 0 {
		avgRedactions = float64(totalRedacted) / float64(len(results))
	}

	return &BatchDeidentificationResult{
		Results: results,
		Summary: BatchSummary{
			TotalTexts:    len(req.Texts),
			TotalRedacted: totalRedacted,
			AvgRedactions: avgRedactions,
		},
	}, nil
}

// GetStats 获取统计信息.
func (m *DeidentificationManager) GetStats() *DeidentificationStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats
}

// GetAuditLog 获取审计日志.
func (m *DeidentificationManager) GetAuditLog(limit int) []AuditEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.auditLog) {
		limit = len(m.auditLog)
	}

	// 返回最新的日志
	start := len(m.auditLog) - limit
	if start < 0 {
		start = 0
	}

	return m.auditLog[start:]
}

// ============================================================
// 内部方法
// ============================================================

// getEnabledRules 获取启用的规则.
func (m *DeidentificationManager) getEnabledRules(ruleID string) []*DeidentificationRule {
	if ruleID != "" {
		// 指定规则
		if rule, exists := m.rules[ruleID]; exists && rule.Enabled {
			return []*DeidentificationRule{rule}
		}
		return nil
	}

	// 所有启用的规则
	rules := make([]*DeidentificationRule, 0)
	for _, rule := range m.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}

	return rules
}

// sortRulesByPriority 按优先级排序（高优先级在前）.
func (m *DeidentificationManager) sortRulesByPriority(rules []*DeidentificationRule) {
	// 简单冒泡排序
	for i := 0; i < len(rules); i++ {
		for j := i + 1; j < len(rules); j++ {
			if rules[j].Priority > rules[i].Priority {
				rules[i], rules[j] = rules[j], rules[i]
			}
		}
	}
}

// applyPolicy 应用脱敏策略.
func (m *DeidentificationManager) applyPolicy(text string, rule *DeidentificationRule) string {
	switch rule.Policy {
	case PolicyMask:
		// 使用规则的占位符或默认占位符
		placeholder := rule.Placeholder
		if placeholder == "" {
			placeholder = m.config.Placeholder
		}
		return placeholder

	case PolicyHash:
		// SHA256 哈希
		hash := sha256.Sum256([]byte(text))
		return fmt.Sprintf("[HASH:%x]", hash[:8])

	case PolicyReplace:
		// 使用词典替换
		if replacement, ok := m.config.ReplaceDict[string(rule.PIIType)]; ok {
			return replacement
		}
		return m.config.Placeholder

	case PolicyRemove:
		// 完全移除
		return ""

	case PolicyPartial:
		// 部分脱敏
		return m.partialMask(text)

	default:
		return m.config.Placeholder
	}
}

// partialMask 部分脱敏.
func (m *DeidentificationManager) partialMask(text string) string {
	runes := []rune(text)
	totalLen := len(runes)

	if totalLen <= m.config.PrefixLen+m.config.SuffixLen {
		// 太短，全部脱敏
		return strings.Repeat("*", totalLen)
	}

	prefix := string(runes[:m.config.PrefixLen])
	suffix := string(runes[totalLen-m.config.SuffixLen:])
	maskLen := totalLen - m.config.PrefixLen - m.config.SuffixLen

	return prefix + strings.Repeat("*", maskLen) + suffix
}

// updateStats 更新统计信息.
func (m *DeidentificationManager) updateStats(result *DeidentificationResult) {
	m.stats.TotalProcessed++
	m.stats.TotalRedacted += result.TotalRedacted
	m.stats.LastProcessedAt = &result.ProcessedAt

	for _, redaction := range result.Redactions {
		m.stats.ByPIIType[redaction.PIIType]++
		m.stats.ByPolicy[string(redaction.Policy)]++
	}
}

// addAuditEntry 添加审计日志.
func (m *DeidentificationManager) addAuditEntry(action string, ruleID string, piiType PIIType, redactedLen int, source string) {
	entry := AuditEntry{
		ID:          fmt.Sprintf("audit_%d", time.Now().UnixNano()),
		Action:      action,
		RuleID:      ruleID,
		PIIType:     piiType,
		RedactedLen: redactedLen,
		Timestamp:   time.Now(),
		Source:      source,
	}

	m.auditLog = append(m.auditLog, entry)

	// 限制日志数量（保留最近10000条）
	if len(m.auditLog) > 10000 {
		m.auditLog = m.auditLog[len(m.auditLog)-10000:]
	}
}
