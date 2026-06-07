package aidatamasking

import (
	"crypto/sha256"
	"fmt"
	"regexp"
	"strings"
	"time"
)

// sensitiveMatch 内部匹配记录
type sensitiveMatch struct {
	start    int
	end      int
	text     string
	rule     *MaskingRule
	dataType SensitiveDataType
}

// Engine 脱敏引擎
type Engine struct {
	config   *MaskingEngineConfig
	patterns []*SensitivePattern
	rules    map[string]*MaskingRule
}

// NewEngine 创建脱敏引擎
func NewEngine(config *MaskingEngineConfig) *Engine {
	if config == nil {
		config = DefaultMaskingEngineConfig()
	}

	e := &Engine{
		config:   config,
		patterns: GetDefaultPatterns(),
		rules:    make(map[string]*MaskingRule),
	}

	// 初始化默认规则
	e.initDefaultRules()
	return e
}

// initDefaultRules 初始化默认脱敏规则
func (e *Engine) initDefaultRules() {
	defaultRules := []*MaskingRule{
		{
			ID:          "rule-id-card",
			Name:        "身份证号脱敏",
			DataType:    DataTypeIDCard,
			Strategy:    StrategyMask,
			Enabled:     true,
			KeepPrefix:  6,
			KeepSuffix:  4,
			MaskChar:    "*",
			Description: "保留前6位和后4位，中间用*替换",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "rule-phone",
			Name:        "手机号脱敏",
			DataType:    DataTypePhone,
			Strategy:    StrategyMask,
			Enabled:     true,
			KeepPrefix:  3,
			KeepSuffix:  4,
			MaskChar:    "*",
			Description: "保留前3位和后4位，中间用*替换",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "rule-bank-card",
			Name:        "银行卡号脱敏",
			DataType:    DataTypeBankCard,
			Strategy:    StrategyMask,
			Enabled:     true,
			KeepPrefix:  4,
			KeepSuffix:  4,
			MaskChar:    "*",
			Description: "保留前4位和后4位，中间用*替换",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "rule-email",
			Name:        "邮箱脱敏",
			DataType:    DataTypeEmail,
			Strategy:    StrategyMask,
			Enabled:     true,
			KeepPrefix:  3,
			KeepSuffix:  0,
			MaskChar:    "*",
			Description: "保留前3个字符，@后面的部分保留域名",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "rule-ip-address",
			Name:        "IP地址脱敏",
			DataType:    DataTypeIPAddress,
			Strategy:    StrategyMask,
			Enabled:     true,
			KeepPrefix:  0,
			KeepSuffix:  0,
			MaskChar:    "*",
			Description: "完全脱敏IP地址",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "rule-license-plate",
			Name:        "车牌号脱敏",
			DataType:    DataTypeLicensePlate,
			Strategy:    StrategyMask,
			Enabled:     true,
			KeepPrefix:  2,
			KeepSuffix:  2,
			MaskChar:    "*",
			Description: "保留前2位和后2位，中间用*替换",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "rule-passport",
			Name:        "护照号脱敏",
			DataType:    DataTypePassport,
			Strategy:    StrategyMask,
			Enabled:     true,
			KeepPrefix:  1,
			KeepSuffix:  3,
			MaskChar:    "*",
			Description: "保留首字母和后3位，中间用*替换",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for _, rule := range defaultRules {
		e.rules[rule.ID] = rule
	}
}

// MaskText 对文本进行脱敏处理
func (e *Engine) MaskText(req *MaskingRequest) (*MaskingResponse, error) {
	if !e.config.Enabled {
		return nil, fmt.Errorf("masking engine is disabled")
	}

	if len(req.Text) > e.config.MaxTextLength {
		return nil, fmt.Errorf("text length exceeds maximum: %d > %d", len(req.Text), e.config.MaxTextLength)
	}

	start := time.Now()
	text := req.Text
	var results []*MaskingResult

	// 确定使用的规则
	rules := e.rules
	if len(req.Rules) > 0 {
		rules = make(map[string]*MaskingRule)
		for _, r := range req.Rules {
			rules[r.ID] = r
		}
	}

	// 收集所有匹配
	var matches []sensitiveMatch

	// 使用模式匹配
	for _, pattern := range e.patterns {
		// 查找匹配的规则
		rule := e.findRuleForType(pattern.DataType, rules)
		if rule == nil || !rule.Enabled {
			continue
		}

		// 如果规则有自定义模式，使用规则的模式
		var re *regexp.Regexp
		if rule.Pattern != "" {
			var err error
			re, err = regexp.Compile(rule.Pattern)
			if err != nil {
				continue
			}
		} else {
			re = pattern.Pattern
		}

		// 查找所有匹配
		for _, loc := range re.FindAllStringIndex(text, -1) {
			matches = append(matches, sensitiveMatch{
				start:    loc[0],
				end:      loc[1],
				text:     text[loc[0]:loc[1]],
				rule:     rule,
				dataType: pattern.DataType,
			})
		}
	}

	// 按位置排序并处理重叠（优先保留较长的匹配）
	matches = e.resolveOverlaps(matches)

	// 应用脱敏
	maskedText := text
	offset := 0

	for _, m := range matches {
		masked := e.applyMasking(m.text, m.rule)

		// 记录结果
		if req.TestMode {
			results = append(results, &MaskingResult{
				Original: m.text,
				Masked:   masked,
				DataType: m.dataType,
				Strategy: m.rule.Strategy,
				StartPos: m.start,
				EndPos:   m.end,
				RuleID:   m.rule.ID,
			})
		}

		// 替换文本
		before := maskedText[:m.start+offset]
		after := maskedText[m.end+offset:]
		maskedText = before + masked + after
		offset += len(masked) - (m.end - m.start)
	}

	// 生成摘要
	summary := e.generateSummary(matches)

	return &MaskingResponse{
		MaskedText: maskedText,
		Results:    results,
		Summary:    summary,
		CreatedAt:  time.Now(),
		Duration:   time.Since(start),
	}, nil
}

// resolveOverlaps 解决重叠匹配，优先保留较长的匹配
func (e *Engine) resolveOverlaps(matches []sensitiveMatch) []sensitiveMatch {
	if len(matches) == 0 {
		return matches
	}

	// 按起始位置排序
	for i := 0; i < len(matches)-1; i++ {
		for j := i + 1; j < len(matches); j++ {
			if matches[i].start > matches[j].start {
				matches[i], matches[j] = matches[j], matches[i]
			}
		}
	}

	var result []sensitiveMatch
	lastEnd := -1

	for _, m := range matches {
		if m.start >= lastEnd {
			result = append(result, m)
			lastEnd = m.end
		}
	}

	return result
}

// findRuleForType 查找指定类型的规则
func (e *Engine) findRuleForType(dataType SensitiveDataType, rules map[string]*MaskingRule) *MaskingRule {
	for _, rule := range rules {
		if rule.DataType == dataType {
			return rule
		}
	}
	return nil
}

// applyMasking 应用脱敏策略
func (e *Engine) applyMasking(text string, rule *MaskingRule) string {
	maskChar := rule.MaskChar
	if maskChar == "" {
		maskChar = "*"
	}

	switch rule.Strategy {
	case StrategyMask:
		return e.applyMaskStrategy(text, rule.KeepPrefix, rule.KeepSuffix, maskChar)
	case StrategyReplace:
		if rule.Replacement != "" {
			return rule.Replacement
		}
		return "[REDACTED]"
	case StrategyHash:
		return e.applyHashStrategy(text)
	case StrategyTruncate:
		return e.applyTruncateStrategy(text, rule.KeepPrefix, rule.KeepSuffix)
	case StrategyRedact:
		return ""
	default:
		return e.applyMaskStrategy(text, rule.KeepPrefix, rule.KeepSuffix, maskChar)
	}
}

// applyMaskStrategy 应用掩码策略
func (e *Engine) applyMaskStrategy(text string, keepPrefix, keepSuffix int, maskChar string) string {
	runes := []rune(text)
	length := len(runes)

	if keepPrefix+keepSuffix >= length {
		return text
	}

	prefix := string(runes[:keepPrefix])
	suffix := ""
	if keepSuffix > 0 {
		suffix = string(runes[length-keepSuffix:])
	}

	maskLength := length - keepPrefix - keepSuffix
	mask := strings.Repeat(maskChar, maskLength)

	return prefix + mask + suffix
}

// applyHashStrategy 应用哈希策略
func (e *Engine) applyHashStrategy(text string) string {
	hash := sha256.Sum256([]byte(text))
	return fmt.Sprintf("%x", hash[:8]) // 取前8字节
}

// applyTruncateStrategy 应用截断策略
func (e *Engine) applyTruncateStrategy(text string, keepPrefix, keepSuffix int) string {
	runes := []rune(text)
	length := len(runes)

	if keepPrefix+keepSuffix >= length {
		return text
	}

	prefix := string(runes[:keepPrefix])
	suffix := ""
	if keepSuffix > 0 {
		suffix = string(runes[length-keepSuffix:])
	}

	return prefix + "..." + suffix
}

// generateSummary 生成脱敏摘要
func (e *Engine) generateSummary(matches []sensitiveMatch) *MaskingSummary {
	summary := &MaskingSummary{
		ByType:     make(map[SensitiveDataType]int),
		ByStrategy: make(map[MaskingStrategy]int),
	}

	summary.TotalMatches = len(matches)
	for _, m := range matches {
		summary.ByType[m.dataType]++
		summary.ByStrategy[m.rule.Strategy]++
	}

	return summary
}

// AddRule 添加脱敏规则
func (e *Engine) AddRule(rule *MaskingRule) error {
	if !IsValidDataType(rule.DataType) {
		return fmt.Errorf("invalid data type: %s", rule.DataType)
	}
	if !IsValidStrategy(rule.Strategy) {
		return fmt.Errorf("invalid strategy: %s", rule.Strategy)
	}

	if rule.ID == "" {
		rule.ID = fmt.Sprintf("rule-%d", time.Now().UnixNano())
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	e.rules[rule.ID] = rule
	return nil
}

// UpdateRule 更新脱敏规则
func (e *Engine) UpdateRule(id string, rule *MaskingRule) error {
	existing, ok := e.rules[id]
	if !ok {
		return fmt.Errorf("rule not found: %s", id)
	}

	rule.ID = id
	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now()
	e.rules[id] = rule
	return nil
}

// DeleteRule 删除脱敏规则
func (e *Engine) DeleteRule(id string) error {
	if _, ok := e.rules[id]; !ok {
		return fmt.Errorf("rule not found: %s", id)
	}
	delete(e.rules, id)
	return nil
}

// GetRule 获取脱敏规则
func (e *Engine) GetRule(id string) (*MaskingRule, error) {
	rule, ok := e.rules[id]
	if !ok {
		return nil, fmt.Errorf("rule not found: %s", id)
	}
	return rule, nil
}

// ListRules 列出所有脱敏规则
func (e *Engine) ListRules() []*MaskingRule {
	rules := make([]*MaskingRule, 0, len(e.rules))
	for _, r := range e.rules {
		rules = append(rules, r)
	}
	return rules
}

// HasSensitiveData 检查文本是否包含敏感数据
func (e *Engine) HasSensitiveData(text string) (bool, []SensitiveDataType) {
	var detected []SensitiveDataType

	for _, pattern := range e.patterns {
		if pattern.Pattern.MatchString(text) {
			detected = append(detected, pattern.DataType)
		}
	}

	return len(detected) > 0, detected
}

// GetConfig 获取配置
func (e *Engine) GetConfig() *MaskingEngineConfig {
	return e.config
}

// UpdateConfig 更新配置
func (e *Engine) UpdateConfig(config *MaskingEngineConfig) {
	if config != nil {
		e.config = config
	}
}
