package aiguardrails

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 错误定义 ==========

var (
	// ErrPolicyNotFound 策略未找到.
	ErrPolicyNotFound = errors.New("护栏策略未找到")
	// ErrPolicyAlreadyExists 策略已存在.
	ErrPolicyAlreadyExists = errors.New("护栏策略已存在")
	// ErrInvalidPolicyType 无效策略类型.
	ErrInvalidPolicyType = errors.New("无效的护栏策略类型")
	// ErrInvalidRuleType 无效规则类型.
	ErrInvalidRuleType = errors.New("无效的规则类型")
	// ErrGuardrailDisabled 护栏未启用.
	ErrGuardrailDisabled = errors.New("AI 护栏未启用")
	// ErrInputBlocked 输入被阻止.
	ErrInputBlocked = errors.New("输入被安全护栏阻止")
	// ErrOutputBlocked 输出被阻止.
	ErrOutputBlocked = errors.New("输出被安全护栏阻止")
	// ErrInputTooLong 输入过长.
	ErrInputTooLong = errors.New("输入超出最大长度限制")
	// ErrOutputTooLong 输出过长.
	ErrOutputTooLong = errors.New("输出超出最大长度限制")
	// ErrModelBlocked 模型被禁止.
	ErrModelBlocked = errors.New("目标模型被禁止使用")
)

// ========== 预置 PII 正则 ==========

var (
	// piiEmailPattern 电子邮箱.
	piiEmailPattern = `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Za-z]{2,}\b`
	// piiPhonePattern 手机号（中国）.
	piiPhonePattern = `\b1[3-9]\d{9}\b`
	// piiIDCardPattern 身份证号.
	piiIDCardPattern = `\b\d{17}[\dXx]\b`
	// piiBankCardPattern 银行卡号.
	piiBankCardPattern = `\b\d{16,19}\b`
	// piiIPPattern IP 地址.
	piiIPPattern = `\b\d{1,3}\.\d{1,3}\.\d{1,3}\.\d{1,3}\b`
)

// ========== Prompt Injection 特征 ==========

var promptInjectionPatterns = []string{
	`(?i)ignore\s+(all\s+)?(previous|prior|above)\s+instructions`,
	`(?i)disregard\s+(all\s+)?(previous|prior|above)\s+instructions`,
	`(?i)forget\s+(all\s+)?(previous|prior|above)\s+instructions`,
	`(?i)you\s+are\s+now\s+(a|an)\s+`,
	`(?i)act\s+as\s+(a|an)\s+`,
	`(?i)pretend\s+(you\s+are|to\s+be)\s+`,
	`(?i)reveal\s+(your|the)\s+(system\s+)?prompt`,
	`(?i)show\s+(me\s+)?(your|the)\s+(system\s+)?prompt`,
	`(?i)>?\s*system\s*:\s*`,
	`(?i)jailbreak`,
	`(?i)DAN\s+mode`,
}

// ========== 预置敏感关键词 ==========

var sensitiveKeywords = []string{
	"密码", "password", "passwd", "secret", "token", "api_key",
	"private_key", "私钥", "密钥", "凭证", "credential",
}

// ========== 服务定义 ==========

// Service AI 安全护栏服务.
type Service struct {
	mu        sync.RWMutex
	policies  map[string]*GuardrailPolicy // policyID -> Policy
	config    AIGuardrailConfig           // 全局配置
	auditLogs []AuditLogEntry             // 审计日志
}

// NewService 创建 AI 安全护栏服务.
func NewService() *Service {
	svc := &Service{
		policies: make(map[string]*GuardrailPolicy),
		config: AIGuardrailConfig{
			Enabled:              true,
			MaxInputLength:       32768,
			MaxOutputLength:      32768,
			RedactPII:            true,
			BlockPromptInjection: true,
			LogAllRequests:       true,
			RetentionDays:        90,
		},
	}
	svc.initDefaultPolicies()
	return svc
}

// initDefaultPolicies 初始化默认策略.
func (s *Service) initDefaultPolicies() {
	now := time.Now()

	// PII 检测策略
	piiPolicy := &GuardrailPolicy{
		ID:        uuid.New().String(),
		Name:      "默认 PII 检测策略",
		Type:      PolicyPII,
		Status:    StatusEnabled,
		Priority:  10,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: "system",
		Rules: []GuardrailRule{
			{ID: "pii-email", Name: "邮箱检测", Type: RuleRegex, Pattern: piiEmailPattern, Severity: SeverityMedium, Action: ActionRedact, Enabled: true},
			{ID: "pii-phone", Name: "手机号检测", Type: RuleRegex, Pattern: piiPhonePattern, Severity: SeverityMedium, Action: ActionRedact, Enabled: true},
			{ID: "pii-idcard", Name: "身份证号检测", Type: RuleRegex, Pattern: piiIDCardPattern, Severity: SeverityHigh, Action: ActionRedact, Enabled: true},
			{ID: "pii-bankcard", Name: "银行卡号检测", Type: RuleRegex, Pattern: piiBankCardPattern, Severity: SeverityHigh, Action: ActionRedact, Enabled: true},
			{ID: "pii-ip", Name: "IP 地址检测", Type: RuleRegex, Pattern: piiIPPattern, Severity: SeverityLow, Action: ActionWarn, Enabled: true},
		},
	}

	// Prompt Injection 防护策略
	injectionPolicy := &GuardrailPolicy{
		ID:        uuid.New().String(),
		Name:      "Prompt Injection 防护策略",
		Type:      PolicyPromptInjection,
		Status:    StatusEnabled,
		Priority:  5,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: "system",
		Rules:     buildInjectionRules(),
	}

	// 敏感数据检测策略
	sensitivePolicy := &GuardrailPolicy{
		ID:        uuid.New().String(),
		Name:      "敏感数据检测策略",
		Type:      PolicySensitiveData,
		Status:    StatusEnabled,
		Priority:  15,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: "system",
		Rules:     buildSensitiveRules(),
	}

	// 输入长度限制策略
	inputPolicy := &GuardrailPolicy{
		ID:        uuid.New().String(),
		Name:      "输入长度限制策略",
		Type:      PolicyInputFilter,
		Status:    StatusEnabled,
		Priority:  1,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: "system",
		Rules: []GuardrailRule{
			{ID: "input-len", Name: "最大输入长度", Type: RuleLength, Pattern: "32768", Severity: SeverityMedium, Action: ActionBlock, Enabled: true},
		},
	}

	// 输出长度限制策略
	outputPolicy := &GuardrailPolicy{
		ID:        uuid.New().String(),
		Name:      "输出长度限制策略",
		Type:      PolicyOutputFilter,
		Status:    StatusEnabled,
		Priority:  1,
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: "system",
		Rules: []GuardrailRule{
			{ID: "output-len", Name: "最大输出长度", Type: RuleLength, Pattern: "32768", Severity: SeverityMedium, Action: ActionBlock, Enabled: true},
		},
	}

	s.mu.Lock()
	s.policies[piiPolicy.ID] = piiPolicy
	s.policies[injectionPolicy.ID] = injectionPolicy
	s.policies[sensitivePolicy.ID] = sensitivePolicy
	s.policies[inputPolicy.ID] = inputPolicy
	s.policies[outputPolicy.ID] = outputPolicy
	s.mu.Unlock()
}

// buildInjectionRules 构建 Prompt Injection 规则.
func buildInjectionRules() []GuardrailRule {
	rules := make([]GuardrailRule, 0, len(promptInjectionPatterns))
	for i, pat := range promptInjectionPatterns {
		rules = append(rules, GuardrailRule{
			ID:       fmt.Sprintf("injection-%d", i+1),
			Name:     fmt.Sprintf("Prompt Injection 模式 %d", i+1),
			Type:     RuleRegex,
			Pattern:  pat,
			Severity: SeverityCritical,
			Action:   ActionBlock,
			Enabled:  true,
		})
	}
	return rules
}

// buildSensitiveRules 构建敏感数据规则.
func buildSensitiveRules() []GuardrailRule {
	rules := make([]GuardrailRule, 0, len(sensitiveKeywords))
	for i, kw := range sensitiveKeywords {
		rules = append(rules, GuardrailRule{
			ID:       fmt.Sprintf("sensitive-%d", i+1),
			Name:     fmt.Sprintf("敏感关键词: %s", kw),
			Type:     RuleKeyword,
			Pattern:  kw,
			Severity: SeverityHigh,
			Action:   ActionWarn,
			Enabled:  true,
		})
	}
	return rules
}

// ========== 配置管理 ==========

// GetConfig 获取全局配置.
func (s *Service) GetConfig() AIGuardrailConfig {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// UpdateConfig 更新全局配置.
func (s *Service) UpdateConfig(req ConfigRequest) AIGuardrailConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.config = AIGuardrailConfig(req)
	if s.config.MaxInputLength == 0 {
		s.config.MaxInputLength = 32768
	}
	if s.config.MaxOutputLength == 0 {
		s.config.MaxOutputLength = 32768
	}
	if s.config.RetentionDays == 0 {
		s.config.RetentionDays = 90
	}
	return s.config
}

// ========== 策略管理 ==========

// CreatePolicy 创建护栏策略.
func (s *Service) CreatePolicy(req PolicyRequest) (*GuardrailPolicy, error) {
	if !isValidPolicyType(req.Type) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidPolicyType, req.Type)
	}
	for _, rule := range req.Rules {
		if !isValidRuleType(rule.Type) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRuleType, rule.Type)
		}
	}

	now := time.Now()
	policy := &GuardrailPolicy{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Rules:       req.Rules,
		Status:      StatusEnabled,
		Priority:    req.Priority,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   req.CreatedBy,
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	s.policies[policy.ID] = policy
	return policy, nil
}

// GetPolicy 获取策略.
func (s *Service) GetPolicy(policyID string) (*GuardrailPolicy, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	policy, ok := s.policies[policyID]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	return policy, nil
}

// ListPolicies 列出所有策略.
func (s *Service) ListPolicies() []*GuardrailPolicy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	policies := make([]*GuardrailPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		policies = append(policies, p)
	}
	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority < policies[j].Priority
	})
	return policies
}

// UpdatePolicy 更新策略.
func (s *Service) UpdatePolicy(policyID string, req PolicyRequest) (*GuardrailPolicy, error) {
	if !isValidPolicyType(req.Type) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidPolicyType, req.Type)
	}
	for _, rule := range req.Rules {
		if !isValidRuleType(rule.Type) {
			return nil, fmt.Errorf("%w: %s", ErrInvalidRuleType, rule.Type)
		}
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	policy, ok := s.policies[policyID]
	if !ok {
		return nil, ErrPolicyNotFound
	}
	policy.Name = req.Name
	policy.Type = req.Type
	policy.Description = req.Description
	policy.Rules = req.Rules
	policy.Priority = req.Priority
	policy.UpdatedAt = time.Now()
	return policy, nil
}

// DeletePolicy 删除策略.
func (s *Service) DeletePolicy(policyID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.policies[policyID]; !ok {
		return ErrPolicyNotFound
	}
	delete(s.policies, policyID)
	return nil
}

// TogglePolicy 启用/禁用策略.
func (s *Service) TogglePolicy(policyID string, enabled bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	policy, ok := s.policies[policyID]
	if !ok {
		return ErrPolicyNotFound
	}
	if enabled {
		policy.Status = StatusEnabled
	} else {
		policy.Status = StatusDisabled
	}
	policy.UpdatedAt = time.Now()
	return nil
}

// ========== 过滤核心 ==========

// FilterInput 过滤输入文本.
func (s *Service) FilterInput(req FilterRequest) (*FilterResponse, error) {
	cfg := s.GetConfig()
	if !cfg.Enabled {
		return &FilterResponse{Allowed: true, CleanText: req.Text, Action: ActionAllow}, nil
	}

	// 检查模型黑名单
	if req.Model != "" {
		if isModelBlocked(req.Model, cfg.WhitelistModels, cfg.BlacklistModels) {
			return &FilterResponse{Allowed: false, Action: ActionBlock, Reason: "目标模型被禁止"}, ErrModelBlocked
		}
	}

	// 检查输入长度
	if cfg.MaxInputLength > 0 && len(req.Text) > cfg.MaxInputLength {
		return &FilterResponse{Allowed: false, Action: ActionBlock, Reason: fmt.Sprintf("输入长度 %d 超出限制 %d", len(req.Text), cfg.MaxInputLength)}, ErrInputTooLong
	}

	// 执行策略检测
	results, redactedText := s.runPolicies(req.Text, "input")

	// 处理结果
	cleanText := redactedText
	blocked := false
	var blockReasons []string

	for _, r := range results {
		if r.Action == ActionBlock {
			blocked = true
			blockReasons = append(blockReasons, r.Message)
		}
	}

	resp := &FilterResponse{
		Allowed:   !blocked,
		Results:   results,
		CleanText: cleanText,
		Action:    ActionAllow,
		Reason:    strings.Join(blockReasons, "; "),
	}
	if blocked {
		resp.Action = ActionBlock
	} else if len(results) > 0 {
		// 如果有警告等非阻止结果，也标记
		for _, r := range results {
			if r.Action == ActionWarn {
				resp.Action = ActionWarn
				break
			}
			if r.Action == ActionRedact {
				resp.Action = ActionRedact
			}
		}
	}

	// 记录审计日志
	if cfg.LogAllRequests || blocked {
		s.addAuditLog(AuditLogEntry{
			ID:        uuid.New().String(),
			Timestamp: time.Now(),
			User:      req.User,
			ClientIP:  req.ClientIP,
			Direction: "input",
			Model:     req.Model,
			InputText: truncateText(req.Text, 500),
			Action:    resp.Action,
			Results:   results,
			Reason:    resp.Reason,
		})
	}

	if blocked {
		return resp, ErrInputBlocked
	}
	return resp, nil
}

// FilterOutput 过滤输出文本.
func (s *Service) FilterOutput(req FilterRequest) (*FilterResponse, error) {
	cfg := s.GetConfig()
	if !cfg.Enabled {
		return &FilterResponse{Allowed: true, CleanText: req.Text, Action: ActionAllow}, nil
	}

	// 检查输出长度
	if cfg.MaxOutputLength > 0 && len(req.Text) > cfg.MaxOutputLength {
		return &FilterResponse{Allowed: false, Action: ActionBlock, Reason: fmt.Sprintf("输出长度 %d 超出限制 %d", len(req.Text), cfg.MaxOutputLength)}, ErrOutputTooLong
	}

	// 执行策略检测（输出不检测 Prompt Injection）
	results, redactedText := s.runPoliciesForOutput(req.Text)

	cleanText := redactedText
	blocked := false
	var blockReasons []string

	for _, r := range results {
		if r.Action == ActionBlock {
			blocked = true
			blockReasons = append(blockReasons, r.Message)
		}
	}

	resp := &FilterResponse{
		Allowed:   !blocked,
		Results:   results,
		CleanText: cleanText,
		Action:    ActionAllow,
		Reason:    strings.Join(blockReasons, "; "),
	}
	if blocked {
		resp.Action = ActionBlock
	} else if len(results) > 0 {
		for _, r := range results {
			if r.Action == ActionWarn {
				resp.Action = ActionWarn
				break
			}
			if r.Action == ActionRedact {
				resp.Action = ActionRedact
			}
		}
	}

	// 记录审计日志
	if cfg.LogAllRequests || blocked {
		s.addAuditLog(AuditLogEntry{
			ID:         uuid.New().String(),
			Timestamp:  time.Now(),
			User:       req.User,
			ClientIP:   req.ClientIP,
			Direction:  "output",
			Model:      req.Model,
			OutputText: truncateText(req.Text, 500),
			Action:     resp.Action,
			Results:    results,
			Reason:     resp.Reason,
		})
	}

	if blocked {
		return resp, ErrOutputBlocked
	}
	return resp, nil
}

// ========== 内部检测方法 ==========

// runPolicies 对输入文本执行所有启用的策略，返回检测结果和累积脱敏后的文本.
func (s *Service) runPolicies(text string, direction string) ([]DetectionResult, string) {
	s.mu.RLock()
	policies := make([]*GuardrailPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		if p.Status == StatusEnabled {
			policies = append(policies, p)
		}
	}
	s.mu.RUnlock()

	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority < policies[j].Priority
	})

	cleanText := text
	var results []DetectionResult
	for _, policy := range policies {
		for _, rule := range policy.Rules {
			if !rule.Enabled {
				continue
			}
			result := s.applyRule(cleanText, policy, rule)
			if result.Hit {
				results = append(results, result)
				if result.RedactedText != "" {
					cleanText = result.RedactedText
				}
			}
		}
	}
	return results, cleanText
}

// runPoliciesForOutput 对输出文本执行策略（跳过 Prompt Injection），返回检测结果和累积脱敏后的文本.
func (s *Service) runPoliciesForOutput(text string) ([]DetectionResult, string) {
	s.mu.RLock()
	policies := make([]*GuardrailPolicy, 0, len(s.policies))
	for _, p := range s.policies {
		if p.Status == StatusEnabled && p.Type != PolicyPromptInjection && p.Type != PolicyInputFilter {
			policies = append(policies, p)
		}
	}
	s.mu.RUnlock()

	sort.Slice(policies, func(i, j int) bool {
		return policies[i].Priority < policies[j].Priority
	})

	cleanText := text
	var results []DetectionResult
	for _, policy := range policies {
		for _, rule := range policy.Rules {
			if !rule.Enabled {
				continue
			}
			result := s.applyRule(cleanText, policy, rule)
			if result.Hit {
				results = append(results, result)
				if result.RedactedText != "" {
					cleanText = result.RedactedText
				}
			}
		}
	}
	return results, cleanText
}

// applyRule 应用单条规则.
func (s *Service) applyRule(text string, policy *GuardrailPolicy, rule GuardrailRule) DetectionResult {
	result := DetectionResult{
		Action:     rule.Action,
		Severity:   rule.Severity,
		PolicyType: policy.Type,
		RuleID:     rule.ID,
		RuleName:   rule.Name,
	}

	switch rule.Type {
	case RuleRegex:
		re, err := regexp.Compile(rule.Pattern)
		if err != nil {
			result.Message = fmt.Sprintf("规则 %s 正则编译失败", rule.Name)
			return result
		}
		matches := re.FindStringSubmatch(text)
		if len(matches) > 0 {
			result.Hit = true
			result.MatchedText = truncateText(matches[0], 100)
			result.Message = fmt.Sprintf("命中规则 %s: 检测到匹配内容", rule.Name)
			if rule.Action == ActionRedact {
				result.RedactedText = re.ReplaceAllString(text, "[REDACTED]")
			}
		}

	case RuleKeyword:
		if strings.Contains(strings.ToLower(text), strings.ToLower(rule.Pattern)) {
			result.Hit = true
			result.MatchedText = rule.Pattern
			result.Message = fmt.Sprintf("命中规则 %s: 检测到敏感关键词 '%s'", rule.Name, rule.Pattern)
			if rule.Action == ActionRedact {
				result.RedactedText = strings.ReplaceAll(text, rule.Pattern, "[REDACTED]")
			}
		}

	case RuleLength:
		maxLen := 0
		fmt.Sscanf(rule.Pattern, "%d", &maxLen)
		if maxLen > 0 && len(text) > maxLen {
			result.Hit = true
			result.MatchedText = fmt.Sprintf("长度 %d > 限制 %d", len(text), maxLen)
			result.Message = fmt.Sprintf("命中规则 %s: 文本长度 %d 超出限制 %d", rule.Name, len(text), maxLen)
		}

	case RulePattern:
		// 简单模式匹配（通配符风格）
		if strings.Contains(text, rule.Pattern) {
			result.Hit = true
			result.MatchedText = rule.Pattern
			result.Message = fmt.Sprintf("命中规则 %s: 检测到匹配模式", rule.Name)
		}

	case RuleSemantic:
		// 语义匹配在此简化为关键词包含
		if strings.Contains(strings.ToLower(text), strings.ToLower(rule.Pattern)) {
			result.Hit = true
			result.MatchedText = rule.Pattern
			result.Message = fmt.Sprintf("命中规则 %s: 语义匹配成功", rule.Name)
		}
	}

	return result
}

// ========== 审计日志 ==========

// addAuditLog 添加审计日志.
func (s *Service) addAuditLog(entry AuditLogEntry) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 清理过期日志
	cfg := s.config
	if cfg.RetentionDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -cfg.RetentionDays)
		filtered := s.auditLogs[:0]
		for _, log := range s.auditLogs {
			if log.Timestamp.After(cutoff) {
				filtered = append(filtered, log)
			}
		}
		s.auditLogs = filtered
	}

	s.auditLogs = append(s.auditLogs, entry)
}

// QueryAudit 查询审计日志.
func (s *Service) QueryAudit(query AuditQuery) []AuditLogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var entries []AuditLogEntry
	for _, log := range s.auditLogs {
		if query.Direction != "" && log.Direction != query.Direction {
			continue
		}
		if query.User != "" && log.User != query.User {
			continue
		}
		if query.Action != "" && log.Action != query.Action {
			continue
		}
		if query.StartTime != nil && log.Timestamp.Before(*query.StartTime) {
			continue
		}
		if query.EndTime != nil && log.Timestamp.After(*query.EndTime) {
			continue
		}
		entries = append(entries, log)
	}

	if query.Limit > 0 && len(entries) > query.Limit {
		entries = entries[len(entries)-query.Limit:]
	}
	return entries
}

// GetAuditLogs 获取所有审计日志.
func (s *Service) GetAuditLogs() []AuditLogEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]AuditLogEntry, len(s.auditLogs))
	copy(out, s.auditLogs)
	return out
}

// ========== 辅助函数 ==========

// isValidPolicyType 检查是否为有效策略类型.
func isValidPolicyType(t PolicyType) bool {
	switch t {
	case PolicyInputFilter, PolicyOutputFilter, PolicySensitiveData,
		PolicyPII, PolicyPromptInjection, PolicyContentSafety:
		return true
	}
	return false
}

// isValidRuleType 检查是否为有效规则类型.
func isValidRuleType(t RuleType) bool {
	switch t {
	case RuleRegex, RuleKeyword, RuleSemantic, RuleLength, RulePattern:
		return true
	}
	return false
}

// isModelBlocked 检查模型是否被禁止.
func isModelBlocked(model string, whitelist, blacklist []string) bool {
	// 黑名单优先
	for _, m := range blacklist {
		if m == model {
			return true
		}
	}
	// 如果有白名单，不在白名单内的都被阻止
	if len(whitelist) > 0 {
		found := false
		for _, m := range whitelist {
			if m == model {
				found = true
				break
			}
		}
		if !found {
			return true
		}
	}
	return false
}

// truncateText 截断文本到指定长度.
func truncateText(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}
	return text[:maxLen] + "...[truncated]"
}
