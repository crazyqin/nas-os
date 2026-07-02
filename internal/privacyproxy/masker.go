package privacyproxy

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"
)

// =========================================================================
// 一、内置脱敏规则
// =========================================================================

// builtinRules 内置规则定义（编译后的正则表达式在 init 中生成）.
var builtinRuleDefs = []struct {
	id      string
	name    string
	pattern string
	action  MaskAction
	keepPre int
	keepSuf int
	desc    string
}{
	{"builtin-id-card", "身份证号", `[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]`, ActionMask, 6, 2, "中国大陆居民身份证号（18位）"},
	{"builtin-phone", "手机号", `1[3-9]\d{9}`, ActionMask, 3, 2, "中国大陆手机号（11位）"},
	{"builtin-email", "邮箱", `[a-zA-Z0-9._%+\-]+@[a-zA-Z0-9.\-]+\.[a-zA-Z]{2,}`, ActionMask, 1, 5, "电子邮箱地址"},
	{"builtin-bank-card", "银行卡号", `[1-9]\d{15,18}`, ActionMask, 4, 4, "银行卡号（16-19位）"},
	{"builtin-ipv4", "IPv4地址", `(?:(?:25[0-5]|2[0-4]\d|[01]?\d\d?)\.){3}(?:25[0-5]|2[0-4]\d|[01]?\d\d?)`, ActionMask, 0, 0, "IPv4 地址"},
	{"builtin-api-key", "API Key", `(?i)sk-[a-zA-Z0-9]{20,}|AKIA[a-zA-Z0-9]{16}|AIza[a-zA-Z0-9_\-]{35}|Bearer\s+[a-zA-Z0-9._\-]{20,}`, ActionRedact, 0, 0, "常见 AI API Key（OpenAI/AWS/Google/Bearer Token）"},
	{"builtin-passport", "护照号", `[A-Z]\d{8}`, ActionMask, 1, 0, "中国护照号码"},
	{"builtin-license-plate", "车牌号", `[京津沪渝冀豫云辽黑湘皖鲁新苏浙赣鄂桂甘晋蒙陕吉闽贵粤川青藏琼宁][A-HJ-NP-Z][A-HJ-NP-Z0-9]{4,5}[A-HJ-NP-Z0-9挂学警港澳]`, ActionMask, 2, 1, "中国车牌号"},
}

// =========================================================================
// 二、脱敏引擎
// =========================================================================

// compiledRule 编译后的规则（正则已预编译，避免每次重新编译）.
type compiledRule struct {
	rule   *MaskRule
	regexp *regexp.Regexp
}

// Masker 脱敏引擎.
type Masker struct {
	mu        sync.RWMutex
	rules     []*compiledRule // 按优先级排序后的规则列表
	config    *MaskConfig
	ruleIndex map[string]*compiledRule // 按规则 ID 索引
}

// NewMasker 创建脱敏引擎实例.
func NewMasker(cfg *MaskConfig) *Masker {
	if cfg == nil {
		cfg = DefaultMaskConfig()
	}
	m := &Masker{
		config:    cfg,
		ruleIndex: make(map[string]*compiledRule),
	}
	m.loadBuiltinRules()
	return m
}

// loadBuiltinRules 加载内置规则.
func (m *Masker) loadBuiltinRules() {
	for _, def := range builtinRuleDefs {
		re, err := regexp.Compile(def.pattern)
		if err != nil {
			continue
		}
		rule := &MaskRule{
			ID:          def.id,
			Name:        def.name,
			Type:        RuleTypeBuiltin,
			Pattern:     def.pattern,
			Action:      def.action,
			KeepPrefix:  def.keepPre,
			KeepSuffix:  def.keepSuf,
			Enabled:     true,
			Priority:    0, // 内置规则优先级为 0
			Description: def.desc,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		cr := &compiledRule{rule: rule, regexp: re}
		m.rules = append(m.rules, cr)
		m.ruleIndex[rule.ID] = cr
	}
}

// AddRule 添加自定义脱敏规则.
func (m *Masker) AddRule(rule *MaskRule) error {
	if rule == nil {
		return fmt.Errorf("规则不能为空")
	}
	if rule.ID == "" {
		return fmt.Errorf("规则 ID 不能为空")
	}
	if rule.Pattern == "" {
		return fmt.Errorf("规则正则表达式不能为空")
	}
	if rule.Type == "" {
		rule.Type = RuleTypeCustom
	}

	re, err := regexp.Compile(rule.Pattern)
	if err != nil {
		return fmt.Errorf("正则表达式编译失败: %w", err)
	}

	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}
	rule.UpdatedAt = time.Now()

	m.mu.Lock()
	defer m.mu.Unlock()

	// 如果 ID 已存在，替换
	if existing, ok := m.ruleIndex[rule.ID]; ok {
		existing.rule = rule
		existing.regexp = re
	} else {
		cr := &compiledRule{rule: rule, regexp: re}
		m.rules = append(m.rules, cr)
		m.ruleIndex[rule.ID] = cr
	}

	m.sortRulesLocked()
	return nil
}

// RemoveRule 移除规则（内置规则不可移除）.
func (m *Masker) RemoveRule(ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cr, ok := m.ruleIndex[ruleID]
	if !ok {
		return fmt.Errorf("规则 %s 不存在", ruleID)
	}
	if cr.rule.Type == RuleTypeBuiltin {
		return fmt.Errorf("内置规则 %s 不可删除", ruleID)
	}

	for i, r := range m.rules {
		if r.rule.ID == ruleID {
			m.rules = append(m.rules[:i], m.rules[i+1:]...)
			break
		}
	}
	delete(m.ruleIndex, ruleID)
	return nil
}

// EnableRule 启用/禁用规则.
func (m *Masker) EnableRule(ruleID string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cr, ok := m.ruleIndex[ruleID]
	if !ok {
		return fmt.Errorf("规则 %s 不存在", ruleID)
	}
	cr.rule.Enabled = enabled
	cr.rule.UpdatedAt = time.Now()
	return nil
}

// ListRules 返回所有规则的副本.
func (m *Masker) ListRules() []*MaskRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MaskRule, len(m.rules))
	for i, cr := range m.rules {
		cp := *cr.rule
		result[i] = &cp
	}
	return result
}

// GetRule 按 ID 获取规则.
func (m *Masker) GetRule(ruleID string) (*MaskRule, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cr, ok := m.ruleIndex[ruleID]
	if !ok {
		return nil, false
	}
	cp := *cr.rule
	return &cp, true
}

// sortRulesLocked 按优先级排序（调用方需持有写锁）.
func (m *Masker) sortRulesLocked() {
	sort.SliceStable(m.rules, func(i, j int) bool {
		return m.rules[i].rule.Priority < m.rules[j].rule.Priority
	})
}

// =========================================================================
// 三、脱敏核心逻辑
// =========================================================================

// MaskResult 单次脱敏匹配结果.
type MaskResult struct {
	RuleID   string `json:"rule_id"`
	RuleName string `json:"rule_name"`
	Original string `json:"original"`
	Masked   string `json:"masked"`
	Start    int    `json:"start"`
	End      int    `json:"end"`
}

// MaskOutcome 脱敏整体结果.
type MaskOutcome struct {
	MaskedText string       `json:"masked_text"`
	Matches    []MaskResult `json:"matches"`
	TotalHits  int          `json:"total_hits"`
}

// Mask 对文本执行脱敏处理
// 依次应用所有已启用的规则，记录每次匹配和替换。
func (m *Masker) Mask(text string) *MaskOutcome {
	outcome := &MaskOutcome{
		MaskedText: text,
		Matches:    []MaskResult{},
	}

	if text == "" {
		return outcome
	}

	m.mu.RLock()
	defer m.mu.RUnlock()

	// 逐条规则应用到当前文本
	// 注意：每条规则处理的是上一条规则处理后的结果
	for _, cr := range m.rules {
		if !cr.rule.Enabled {
			continue
		}
		result := m.applyRule(outcome.MaskedText, cr)
		if len(result.matches) > 0 {
			outcome.MaskedText = result.text
			outcome.Matches = append(outcome.Matches, result.matches...)
		}
	}
	outcome.TotalHits = len(outcome.Matches)
	return outcome
}

// applyRuleResult 规则应用中间结果.
type applyRuleResult struct {
	text    string
	matches []MaskResult
}

// applyRule 对文本应用单条规则.
func (m *Masker) applyRule(text string, cr *compiledRule) applyRuleResult {
	matches := cr.regexp.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return applyRuleResult{text: text}
	}

	result := text
	offset := 0 // 替换后偏移量变化
	var matchResults []MaskResult

	for _, loc := range matches {
		start := loc[0] + offset
		end := loc[1] + offset
		if start > len(result) || end > len(result) {
			break
		}
		original := result[start:end]
		masked := m.applyAction(original, cr.rule)

		// 替换
		result = result[:start] + masked + result[end:]
		offset += len(masked) - (end - start)

		matchResults = append(matchResults, MaskResult{
			RuleID:   cr.rule.ID,
			RuleName: cr.rule.Name,
			Original: truncate(original, 200),
			Masked:   truncate(masked, 200),
			Start:    loc[0],
			End:      loc[1],
		})
	}

	return applyRuleResult{text: result, matches: matchResults}
}

// applyAction 按规则动作对单个匹配项执行脱敏.
func (m *Masker) applyAction(text string, rule *MaskRule) string {
	switch rule.Action {
	case ActionReplace:
		if rule.Replacement != "" {
			return rule.Replacement
		}
		return "[REPLACED]"

	case ActionHash:
		h := sha256.Sum256([]byte(text))
		return "hash:" + hex.EncodeToString(h[:8]) // 取前 8 字节，16 个十六进制字符

	case ActionRedact:
		return "[REDACTED]"

	case ActionMask:
		return maskString(text, rule.KeepPrefix, rule.KeepSuffix)

	default:
		// 未知动作，使用默认掩码
		return maskString(text, 0, 0)
	}
}

// maskString 对字符串执行掩码：保留前 keepPrefix 和后 keepSuffix 字符，中间用 * 代替
// 如果字符串长度不足以保留首尾，则全部掩码.
func maskString(s string, keepPrefix, keepSuffix int) string {
	runes := []rune(s)
	n := len(runes)
	if n == 0 {
		return s
	}
	// 确保保留数不超过字符���长度
	if keepPrefix+keepSuffix >= n {
		keepPrefix = n / 3
		keepSuffix = n / 3
	}
	if keepPrefix < 0 {
		keepPrefix = 0
	}
	if keepSuffix < 0 {
		keepSuffix = 0
	}
	if keepPrefix+keepSuffix >= n {
		return strings.Repeat("*", n)
	}
	maskLen := n - keepPrefix - keepSuffix
	if maskLen <= 0 {
		return s
	}
	return string(runes[:keepPrefix]) + strings.Repeat("*", maskLen) + string(runes[n-keepSuffix:])
}

// truncate 截断字符串到最大长度.
func truncate(s string, maxLen int) string {
	runes := []rune(s)
	if len(runes) <= maxLen {
		return s
	}
	return string(runes[:maxLen]) + "..."
}
