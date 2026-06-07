package aiconsole

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
)

// Redactor 隐私数据脱敏引擎.
type Redactor struct {
	mu      sync.RWMutex
	regexps map[string]*compiledRule
	rules   []*RedactRule
}

// compiledRule 编译后的规则.
type compiledRule struct {
	rule *RedactRule
	re   *regexp.Regexp
}

// NewRedactor 创建脱敏引擎.
func NewRedactor() *Redactor {
	return &Redactor{
		regexps: make(map[string]*compiledRule),
	}
}

// LoadRules 从规则列表加载（通常从数据库获取）.
func (rd *Redactor) LoadRules(rules []*RedactRule) error {
	rd.mu.Lock()
	defer rd.mu.Unlock()

	rd.regexps = make(map[string]*compiledRule)
	rd.rules = rules

	// 按优先级排序，高优先级先处理
	sort.Slice(rd.rules, func(i, j int) bool {
		return rd.rules[i].Priority > rd.rules[j].Priority
	})

	for _, r := range rd.rules {
		if !r.Enabled {
			continue
		}
		re, err := regexp.Compile(r.Pattern)
		if err != nil {
			return fmt.Errorf("规则 %s 正则编译失败: %w", r.ID, err)
		}
		rd.regexps[r.ID] = &compiledRule{rule: r, re: re}
	}
	return nil
}

// SetDefaultRules 加载默认内置规则（中国常见 PII 类型）.
func (rd *Redactor) SetDefaultRules() {
	defaults := []*RedactRule{
		{
			ID:        "builtin_id_card",
			Name:      "中国身份证号",
			PIIType:   PIIIDCard,
			Pattern:   `\b[1-9]\d{5}(?:18|19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`,
			Strategy:  StrategyPartial,
			ShowFirst: 3, ShowLast: 4,
			MaskChar: "*",
			Enabled:  true,
			Priority: 100,
		},
		{
			ID:        "builtin_bank_card",
			Name:      "银行卡号",
			PIIType:   PIIBankCard,
			Pattern:   `\b(?:\d{4}[-\s]?){3}\d{4}\b`,
			Strategy:  StrategyPartial,
			ShowFirst: 4, ShowLast: 4,
			MaskChar: "*",
			Enabled:  true,
			Priority: 95,
		},
		{
			ID:        "builtin_phone",
			Name:      "手机号码",
			PIIType:   PIIPhone,
			Pattern:   `\b1[3-9]\d{9}\b`,
			Strategy:  StrategyPartial,
			ShowFirst: 3, ShowLast: 4,
			MaskChar: "*",
			Enabled:  true,
			Priority: 90,
		},
		{
			ID:        "builtin_email",
			Name:      "电子邮箱",
			PIIType:   PIIEmail,
			Pattern:   `\b[\w.\-]+@[\w.\-]+\.\w{2,}\b`,
			Strategy:  StrategyPartial,
			ShowFirst: 2, ShowLast: 0,
			MaskChar: "*",
			Enabled:  true,
			Priority: 85,
		},
		{
			ID:       "builtin_ip",
			Name:     "IP地址",
			PIIType:  PIIIPAddress,
			Pattern:  `\b(?:\d{1,3}\.){3}\d{1,3}\b`,
			Strategy: StrategyMask,
			MaskChar: "*",
			Enabled:  true,
			Priority: 80,
		},
		{
			ID:        "builtin_passport",
			Name:      "护照号",
			PIIType:   PIIPassport,
			Pattern:   `\b[EGeg]\d{8}\b`,
			Strategy:  StrategyPartial,
			ShowFirst: 1, ShowLast: 2,
			MaskChar: "*",
			Enabled:  true,
			Priority: 75,
		},
	}
	rd.LoadRules(defaults) //nolint:errcheck // 默认规则都是合法正则
}

// Process 对文本执行脱敏.
func (rd *Redactor) Process(text string) *RedactResult {
	rd.mu.RLock()
	defer rd.mu.RUnlock()

	result := &RedactResult{
		Processed:  text,
		Redactions: make([]RedactDetail, 0),
	}

	for _, cr := range rd.regexps {
		matches := cr.re.FindAllStringIndex(result.Processed, -1)
		// 反向处理以保持索引正确
		for i := len(matches) - 1; i >= 0; i-- {
			start, end := matches[i][0], matches[i][1]
			original := result.Processed[start:end]

			replacement := rd.applyStrategy(original, cr.rule)

			result.Redactions = append(result.Redactions, RedactDetail{
				PIIType:  cr.rule.PIIType,
				Start:    start,
				End:      end,
				Original: original,
				Replaced: replacement,
				Strategy: cr.rule.Strategy,
				RuleID:   cr.rule.ID,
				RuleName: cr.rule.Name,
			})

			result.Processed = result.Processed[:start] + replacement + result.Processed[end:]
			result.RedactCount++
		}
	}

	result.HasRedaction = result.RedactCount > 0
	return result
}

// applyStrategy 应用脱敏策略.
func (rd *Redactor) applyStrategy(value string, rule *RedactRule) string {
	switch rule.Strategy {
	case StrategyMask:
		if rule.MaskChar == "" {
			return strings.Repeat("*", len(value))
		}
		return strings.Repeat(rule.MaskChar, len(value))

	case StrategyPartial:
		maskChar := rule.MaskChar
		if maskChar == "" {
			maskChar = "*"
		}

		runes := []rune(value)
		totalLen := len(runes)
		showFirst := rule.ShowFirst
		showLast := rule.ShowLast

		if showFirst+showLast >= totalLen {
			return value // 太短了，全部显示
		}

		maskLen := totalLen - showFirst - showLast
		prefix := string(runes[:showFirst])
		suffix := ""
		if showLast > 0 {
			suffix = string(runes[totalLen-showLast:])
		}
		return prefix + strings.Repeat(maskChar, maskLen) + suffix

	case StrategyHash:
		h := sha256.Sum256([]byte(value))
		return "[HASH:" + hex.EncodeToString(h[:8]) + "]"

	case StrategyRemove:
		if rule.Replacement != "" {
			return rule.Replacement
		}
		return "[REDACTED]"

	default:
		return strings.Repeat("*", len(value))
	}
}

// HasPII 检测文本是否包含 PII 信息.
func (rd *Redactor) HasPII(text string) bool {
	rd.mu.RLock()
	defer rd.mu.RUnlock()

	for _, cr := range rd.regexps {
		if cr.re.MatchString(text) {
			return true
		}
	}
	return false
}

// GetRules 获取当前规则列表.
func (rd *Redactor) GetRules() []*RedactRule {
	rd.mu.RLock()
	defer rd.mu.RUnlock()
	cp := make([]*RedactRule, len(rd.rules))
	copy(cp, rd.rules)
	return cp
}
