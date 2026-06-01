package privacycomputing

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// NewMaskingManager 创建数据脱敏管理器
func NewMaskingManager() *MaskingManager {
	mm := &MaskingManager{
		rules: make(map[string]*DataMaskRule),
	}
	// 添加默认规则
	mm.addDefaultRules()
	return mm
}

// addDefaultRules 添加默认脱敏规则
func (mm *MaskingManager) addDefaultRules() {
	defaultRules := []DataMaskRule{
		{
			ID:       "rule-phone",
			Name:     "手机号脱敏",
			Type:     "regex",
			Pattern:  `1[3-9]\d{9}`,
			Strategy: "partial",
			Config: map[string]interface{}{
				"prefix_keep": 3,
				"suffix_keep": 4,
				"mask_char":   "*",
			},
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			ID:       "rule-idcard",
			Name:     "身份证脱敏",
			Type:     "regex",
			Pattern:  `[1-9]\d{5}(19|20)\d{2}(0[1-9]|1[0-2])(0[1-9]|[12]\d|3[01])\d{3}[\dXx]`,
			Strategy: "partial",
			Config: map[string]interface{}{
				"prefix_keep": 3,
				"suffix_keep": 4,
				"mask_char":   "*",
			},
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			ID:       "rule-email",
			Name:     "邮箱脱敏",
			Type:     "regex",
			Pattern:  `[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`,
			Strategy: "partial",
			Config: map[string]interface{}{
				"prefix_keep": 2,
				"suffix_keep": 0,
				"mask_char":   "*",
			},
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			ID:       "rule-bankcard",
			Name:     "银行卡脱敏",
			Type:     "regex",
			Pattern:  `[1-9]\d{12,18}`,
			Strategy: "partial",
			Config: map[string]interface{}{
				"prefix_keep": 4,
				"suffix_keep": 4,
				"mask_char":   "*",
			},
			Enabled:   true,
			CreatedAt: time.Now(),
		},
		{
			Name:     "IP地址脱敏",
			ID:       "rule-ip",
			Type:     "regex",
			Pattern:  `\b((25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(25[0-5]|2[0-4]\d|[01]?\d\d?)\b`,
			Strategy: "mask",
			Config: map[string]interface{}{
				"mask_char": "*",
			},
			Enabled:   true,
			CreatedAt: time.Now(),
		},
	}

	for _, rule := range defaultRules {
		mm.rules[rule.ID] = &rule
	}
}

// CreateRule 创建脱敏规则
func (mm *MaskingManager) CreateRule(req CreateMaskRuleRequest) (*DataMaskRule, error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("规则名称不能为空")
	}

	validTypes := map[string]bool{
		"regex":  true,
		"column": true,
		"row":    true,
		"cell":   true,
	}
	if !validTypes[req.Type] {
		return nil, fmt.Errorf("不支持的规则类型: %s", req.Type)
	}

	validStrategies := map[string]bool{
		"mask":           true,
		"partial":        true,
		"hash":           true,
		"tokenize":       true,
		"pseudonymize":   true,
	}
	if !validStrategies[req.Strategy] {
		return nil, fmt.Errorf("不支持的脱敏策略: %s", req.Strategy)
	}

	rule := &DataMaskRule{
		ID:        uuid.New().String(),
		Name:      req.Name,
		Type:      req.Type,
		Pattern:   req.Pattern,
		Strategy:  req.Strategy,
		Config:    req.Config,
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	mm.rules[rule.ID] = rule
	return rule, nil
}

// GetRule 获取脱敏规则
func (mm *MaskingManager) GetRule(ruleID string) (*DataMaskRule, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	rule, exists := mm.rules[ruleID]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", ruleID)
	}
	return rule, nil
}

// ListRules 列出所有脱敏规则
func (mm *MaskingManager) ListRules() []*DataMaskRule {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	rules := make([]*DataMaskRule, 0, len(mm.rules))
	for _, rule := range mm.rules {
		rules = append(rules, rule)
	}
	return rules
}

// UpdateRule 更新脱敏规则
func (mm *MaskingManager) UpdateRule(ruleID string, req CreateMaskRuleRequest) (*DataMaskRule, error) {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	rule, exists := mm.rules[ruleID]
	if !exists {
		return nil, fmt.Errorf("规则不存在: %s", ruleID)
	}

	if req.Name != "" {
		rule.Name = req.Name
	}
	if req.Type != "" {
		rule.Type = req.Type
	}
	if req.Pattern != "" {
		rule.Pattern = req.Pattern
	}
	if req.Strategy != "" {
		rule.Strategy = req.Strategy
	}
	if req.Config != nil {
		rule.Config = req.Config
	}

	return rule, nil
}

// DeleteRule 删除脱敏规则
func (mm *MaskingManager) DeleteRule(ruleID string) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	if _, exists := mm.rules[ruleID]; !exists {
		return fmt.Errorf("规则不存在: %s", ruleID)
	}

	delete(mm.rules, ruleID)
	return nil
}

// ApplyMask 应用脱敏规则
func (mm *MaskingManager) ApplyMask(req ApplyMaskRequest) (*MaskResult, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	if req.Content == "" {
		return nil, fmt.Errorf("内容不能为空")
	}

	// 确定要使用的规则
	rulesToApply := make([]*DataMaskRule, 0)
	if len(req.RuleIDs) > 0 {
		for _, ruleID := range req.RuleIDs {
			if rule, exists := mm.rules[ruleID]; exists && rule.Enabled {
				rulesToApply = append(rulesToApply, rule)
			}
		}
	} else {
		// 使用所有启用的规则
		for _, rule := range mm.rules {
			if rule.Enabled && rule.Type == "regex" {
				rulesToApply = append(rulesToApply, rule)
			}
		}
	}

	maskedContent := req.Content
	details := make([]MaskDetail, 0)
	rulesApplied := make([]string, 0)

	for _, rule := range rulesToApply {
		if rule.Type != "regex" || rule.Pattern == "" {
			continue
		}

		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			continue
		}

		matches := re.FindAllString(maskedContent, -1)
		if len(matches) == 0 {
			continue
		}

		rulesApplied = append(rulesApplied, rule.Name)

		for _, match := range matches {
			masked := applyMaskStrategy(match, rule.Strategy, rule.Config)
			details = append(details, MaskDetail{
				Field:    rule.Name,
				Rule:     rule.Strategy,
				Original: match,
				Masked:   masked,
			})
		}

		maskedContent = re.ReplaceAllStringFunc(maskedContent, func(match string) string {
			return applyMaskStrategy(match, rule.Strategy, rule.Config)
		})
	}

	return &MaskResult{
		OriginalCount: len(req.Content),
		MaskedCount:   len(maskedContent),
		RulesApplied:  rulesApplied,
		Details:       details,
		ProcessedAt:   time.Now(),
	}, nil
}

// ApplyTableMask 应用表格数据脱敏
func (mm *MaskingManager) ApplyTableMask(req ApplyTableMaskRequest) ([]map[string]interface{}, error) {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	if len(req.Data) == 0 {
		return nil, fmt.Errorf("数据不能为空")
	}

	result := make([]map[string]interface{}, len(req.Data))

	for i, row := range req.Data {
		maskedRow := make(map[string]interface{})
		for col, value := range row {
			ruleID, hasRule := req.Rules[col]
			if !hasRule {
				maskedRow[col] = value
				continue
			}

			rule, exists := mm.rules[ruleID]
			if !exists || !rule.Enabled {
				maskedRow[col] = value
				continue
			}

			strValue := fmt.Sprintf("%v", value)
			maskedRow[col] = applyMaskStrategy(strValue, rule.Strategy, rule.Config)
		}
		result[i] = maskedRow
	}

	return result, nil
}

// applyMaskStrategy 应用脱敏策略
func applyMaskStrategy(value, strategy string, config map[string]interface{}) string {
	runes := []rune(value)
	length := len(runes)

	switch strategy {
	case "mask":
		maskChar := "*"
		if mc, ok := config["mask_char"].(string); ok {
			maskChar = mc
		}
		return strings.Repeat(maskChar, length)

	case "partial":
		prefixKeep := 3
		suffixKeep := 4
		maskChar := "*"

		if pk, ok := config["prefix_keep"].(int); ok {
			prefixKeep = pk
		}
		if sk, ok := config["suffix_keep"].(int); ok {
			suffixKeep = sk
		}
		if mc, ok := config["mask_char"].(string); ok {
			maskChar = mc
		}

		if length <= prefixKeep+suffixKeep {
			return strings.Repeat(maskChar, length)
		}

		prefix := string(runes[:prefixKeep])
		suffix := ""
		if suffixKeep > 0 {
			suffix = string(runes[length-suffixKeep:])
		}
		maskLen := length - prefixKeep - suffixKeep
		return prefix + strings.Repeat(maskChar, maskLen) + suffix

	case "hash":
		hash := sha256.Sum256([]byte(value))
		return fmt.Sprintf("%x", hash[:8])

	case "tokenize":
		// 简单的token化
		return fmt.Sprintf("TOKEN_%x", sha256.Sum256([]byte(value))[:4])

	case "pseudonymize":
		// 伪匿名化
		return fmt.Sprintf("PSEUDO_%x", sha256.Sum256([]byte(value))[:4])

	default:
		return value
	}
}

// GetEnabledRules 获取所有启用的规则
func (mm *MaskingManager) GetEnabledRules() []*DataMaskRule {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	rules := make([]*DataMaskRule, 0)
	for _, rule := range mm.rules {
		if rule.Enabled {
			rules = append(rules, rule)
		}
	}
	return rules
}

// ToggleRule 启用/禁用规则
func (mm *MaskingManager) ToggleRule(ruleID string, enabled bool) error {
	mm.mu.Lock()
	defer mm.mu.Unlock()

	rule, exists := mm.rules[ruleID]
	if !exists {
		return fmt.Errorf("规则不存在: %s", ruleID)
	}

	rule.Enabled = enabled
	return nil
}
