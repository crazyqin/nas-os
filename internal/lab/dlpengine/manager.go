// Package dlpengine 提供数据防泄漏引擎核心管理逻辑
package dlpengine

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager DLP引擎管理器.
type Manager struct {
	mu         sync.RWMutex
	logger     *zap.Logger
	config     *DLPConfig
	policies   map[string]*DLPPolicy
	patterns   map[string]*SensitivePattern
	violations []*Violation
	stats      *ScanStats
}

// NewManager 创建DLP引擎管理器.
func NewManager(logger *zap.Logger, config *DLPConfig) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultDLPConfig()
	}

	m := &Manager{
		logger:     logger,
		config:     config,
		policies:   make(map[string]*DLPPolicy),
		patterns:   make(map[string]*SensitivePattern),
		violations: make([]*Violation, 0),
		stats: &ScanStats{
			ByLevel:     make(map[SensitivityLevel]int64),
			ByChannel:   make(map[TransferProtocol]int64),
			ByAction:    make(map[PolicyAction]int64),
			TopPatterns: make([]PatternStat, 0),
			TopUsers:    make([]UserStat, 0),
		},
	}

	// 初始化默认模式
	m.initDefaultPatterns()

	return m
}

// generateID 生成唯一 ID.
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// initDefaultPatterns 初始化默认敏感数据模式.
func (m *Manager) initDefaultPatterns() {
	defaultPatterns := []*SensitivePattern{
		{
			ID:          "pattern-ssn",
			Name:        "社会安全号码",
			Description: "美国社会安全号码 (SSN)",
			Type:        PatternRegex,
			Level:       SensitivityCritical,
			Pattern:     `\b\d{3}-\d{2}-\d{4}\b`,
			IsEnabled:   true,
			Category:    "PII",
			Confidence:  0.95,
		},
		{
			ID:          "pattern-credit-card",
			Name:        "信用卡号",
			Description: "信用卡号码 (Visa, MasterCard, etc.)",
			Type:        PatternRegex,
			Level:       SensitivityCritical,
			Pattern:     `\b(?:\d{4}[- ]?){3}\d{4}\b`,
			IsEnabled:   true,
			Category:    "PCI",
			Confidence:  0.9,
		},
		{
			ID:          "pattern-email",
			Name:        "邮箱地址",
			Description: "电子邮箱地址",
			Type:        PatternRegex,
			Level:       SensitivityMedium,
			Pattern:     `\b[A-Za-z0-9._%+-]+@[A-Za-z0-9.-]+\.[A-Z|a-z]{2,}\b`,
			IsEnabled:   true,
			Category:    "PII",
			Confidence:  0.99,
		},
		{
			ID:          "pattern-phone",
			Name:        "电话号码",
			Description: "国际电话号码",
			Type:        PatternRegex,
			Level:       SensitivityMedium,
			Pattern:     `\b(?:\+?1[-.]?)?\(?[0-9]{3}\)?[-.]?[0-9]{3}[-.]?[0-9]{4}\b`,
			IsEnabled:   true,
			Category:    "PII",
			Confidence:  0.85,
		},
		{
			ID:          "pattern-china-id",
			Name:        "身份证号",
			Description: "中国居民身份证号码",
			Type:        PatternRegex,
			Level:       SensitivityCritical,
			Pattern:     `\b[1-9]\d{5}(?:19|20)\d{2}(?:0[1-9]|1[0-2])(?:0[1-9]|[12]\d|3[01])\d{3}[\dXx]\b`,
			IsEnabled:   true,
			Category:    "PII",
			Confidence:  0.98,
		},
		{
			ID:          "pattern-api-key",
			Name:        "API密钥",
			Description: "常见API密钥格式",
			Type:        PatternRegex,
			Level:       SensitivityHigh,
			Pattern:     `(?:api[_-]?key|apikey|access[_-]?key|secret[_-]?key)[=:]\s*['"]?([A-Za-z0-9_\-]{20,})['"]?`,
			IsEnabled:   true,
			Category:    "Secrets",
			Confidence:  0.85,
		},
		{
			ID:          "pattern-password",
			Name:        "密码",
			Description: "密码字段",
			Type:        PatternRegex,
			Level:       SensitivityHigh,
			Pattern:     `(?:password|passwd|pwd)[=:]\s*['"]?([^\s'"]{8,})['"]?`,
			IsEnabled:   true,
			Category:    "Secrets",
			Confidence:  0.8,
		},
		{
			ID:          "pattern-private-key",
			Name:        "私钥",
			Description: "PEM格式私钥",
			Type:        PatternRegex,
			Level:       SensitivityCritical,
			Pattern:     `-----BEGIN\s+(?:RSA\s+)?PRIVATE\s+KEY-----`,
			IsEnabled:   true,
			Category:    "Secrets",
			Confidence:  0.99,
		},
		{
			ID:          "pattern-ip-address",
			Name:        "IP地址",
			Description: "IPv4地址",
			Type:        PatternRegex,
			Level:       SensitivityLow,
			Pattern:     `\b(?:\d{1,3}\.){3}\d{1,3}\b`,
			IsEnabled:   true,
			Category:    "Network",
			Confidence:  0.9,
		},
		{
			ID:          "pattern-medical-record",
			Name:        "医疗记录号",
			Description: "医疗记录号码",
			Type:        PatternRegex,
			Level:       SensitivityHigh,
			Pattern:     `\b(?:MRN|MR|Medical Record)[\s:#]*([A-Z0-9]{6,12})\b`,
			IsEnabled:   true,
			Category:    "PHI",
			Confidence:  0.75,
		},
	}

	for _, p := range defaultPatterns {
		p.CreatedAt = time.Now()
		p.LastUpdated = time.Now()
		m.patterns[p.ID] = p
	}
}

// ScanContent 扫描内容.
func (m *Manager) ScanContent(req *ScanRequest) (*ScanResult, error) {
	if !m.config.Enabled {
		return &ScanResult{
			ID:        generateID(),
			ScanID:    req.ID,
			Resource:  req.Resource,
			Timestamp: time.Now(),
		}, nil
	}

	start := time.Now()

	if req.ID == "" {
		req.ID = generateID()
	}

	// 检查内容大小
	if int64(len(req.Content)) > m.config.MaxContentSize {
		return nil, fmt.Errorf("content size exceeds maximum allowed (%d bytes)", m.config.MaxContentSize)
	}

	result := &ScanResult{
		ID:         generateID(),
		ScanID:     req.ID,
		Resource:   req.Resource,
		Size:       int64(len(req.Content)),
		Timestamp:  start,
		Violations: make([]*Violation, 0),
	}

	content := string(req.Content)
	patternsMatched := 0
	maxLevel := SensitivityNone
	violations := make([]*Violation, 0)

	// 遍历所有模式进行匹配
	m.mu.RLock()
	for _, pattern := range m.patterns {
		if !pattern.IsEnabled {
			continue
		}

		matches := m.matchPattern(pattern, content)
		if len(matches) > 0 {
			patternsMatched++

			// 更新最大敏感级别
			if m.sensitivityValue(pattern.Level) > m.sensitivityValue(maxLevel) {
				maxLevel = pattern.Level
			}

			// 匹配策略并创建违规记录
			violation := m.createViolation(req, pattern, matches)
			if violation != nil {
				violations = append(violations, violation)
			}
		}
	}
	m.mu.RUnlock()

	result.PatternsMatched = patternsMatched
	result.ViolationCount = len(violations)
	result.HasViolation = len(violations) > 0
	result.MaxLevel = maxLevel
	result.Violations = violations
	result.ScanDuration = time.Since(start)

	// 决定动作
	if result.HasViolation {
		action := m.determineAction(violations)
		result.Action = action
		result.Blocked = action == ActionBlock || action == ActionQuarantine

		// 保存违规记录
		m.mu.Lock()
		m.violations = append(m.violations, violations...)

		// 更新统计
		m.stats.TotalScans++
		m.stats.ViolationsFound += int64(len(violations))
		if result.Blocked {
			m.stats.BlockedTransfers++
		}
		m.stats.ByLevel[maxLevel]++
		if req.Channel != "" {
			m.stats.ByChannel[req.Channel]++
		}
		m.stats.ByAction[action]++
		m.mu.Unlock()

		// 限制违规记录数量
		m.trimViolations()

		m.logger.Warn("DLP violation detected",
			zap.String("resource", req.Resource),
			zap.Int("violations", len(violations)),
			zap.String("max_level", string(maxLevel)),
			zap.Bool("blocked", result.Blocked))
	} else {
		m.mu.Lock()
		m.stats.TotalScans++
		m.mu.Unlock()
	}

	return result, nil
}

// SetPolicy 设置策略.
func (m *Manager) SetPolicy(policy *DLPPolicy) (*DLPPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.ID == "" {
		policy.ID = generateID()
	}
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	// 验证模式是否存在
	for _, patternID := range policy.Patterns {
		if _, ok := m.patterns[patternID]; !ok {
			return nil, fmt.Errorf("pattern not found: %s", patternID)
		}
	}

	m.policies[policy.ID] = policy

	m.logger.Info("DLP policy set",
		zap.String("policy_id", policy.ID),
		zap.String("name", policy.Name),
		zap.String("action", string(policy.Action)))

	return policy, nil
}

// GetViolations 获取违规记录.
func (m *Manager) GetViolations(limit int, level SensitivityLevel, userID string) []*Violation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Violation, 0)

	for i := len(m.violations) - 1; i >= 0; i-- {
		v := m.violations[i]

		if level != "" && v.Level != level {
			continue
		}
		if userID != "" && v.UserID != userID {
			continue
		}

		result = append(result, v)
		if limit > 0 && len(result) >= limit {
			break
		}
	}

	return result
}

// BlockTransfer 阻断传输.
func (m *Manager) BlockTransfer(resource string, userID string, reason string) error {
	if !m.config.AutoBlock {
		return fmt.Errorf("auto-block is disabled")
	}

	violation := &Violation{
		ID:        generateID(),
		Level:     SensitivityHigh,
		Status:    ViolationStatusNew,
		UserID:    userID,
		Resource:  resource,
		Blocked:   true,
		Action:    ActionBlock,
		Timestamp: time.Now(),
		Context:   reason,
	}

	m.mu.Lock()
	m.violations = append(m.violations, violation)
	m.stats.BlockedTransfers++
	m.mu.Unlock()

	m.logger.Warn("transfer blocked",
		zap.String("resource", resource),
		zap.String("user_id", userID),
		zap.String("reason", reason))

	return nil
}

// matchPattern 匹配模式.
func (m *Manager) matchPattern(pattern *SensitivePattern, content string) []MatchedContent {
	matches := make([]MatchedContent, 0)

	switch pattern.Type {
	case PatternRegex:
		re, err := regexp.Compile(pattern.Pattern)
		if err != nil {
			m.logger.Error("invalid regex pattern",
				zap.String("pattern_id", pattern.ID),
				zap.Error(err))
			return matches
		}

		indexes := re.FindAllStringIndex(content, -1)
		for _, idx := range indexes {
			if len(idx) >= 2 {
				match := content[idx[0]:idx[1]]
				// 过滤低置信度匹配
				if m.isHighConfidenceMatch(pattern, match) {
					matches = append(matches, MatchedContent{
						PatternName: pattern.Name,
						Match:       match,
						StartPos:    idx[0],
						EndPos:      idx[1],
						Confidence:  pattern.Confidence,
					})
				}
			}
		}

	case PatternKeyword:
		lowerContent := strings.ToLower(content)
		lowerPattern := strings.ToLower(pattern.Pattern)
		idx := 0
		for {
			pos := strings.Index(lowerContent[idx:], lowerPattern)
			if pos == -1 {
				break
			}
			actualPos := idx + pos
			matches = append(matches, MatchedContent{
				PatternName: pattern.Name,
				Match:       content[actualPos : actualPos+len(pattern.Pattern)],
				StartPos:    actualPos,
				EndPos:      actualPos + len(pattern.Pattern),
				Confidence:  pattern.Confidence,
			})
			idx = actualPos + len(pattern.Pattern)
		}

	case PatternFingerprint:
		// 指纹匹配需要预计算的内容哈希
		// 简化实现：假设 pattern 存储的是内容哈希
		if strings.Contains(content, pattern.Pattern) {
			matches = append(matches, MatchedContent{
				PatternName: pattern.Name,
				Match:       "[fingerprint match]",
				Confidence:  pattern.Confidence,
			})
		}
	}

	// 限制匹配数量
	maxMatches := 100
	if len(matches) > maxMatches {
		matches = matches[:maxMatches]
	}

	return matches
}

// isHighConfidenceMatch 检查是否为高置信度匹配.
func (m *Manager) isHighConfidenceMatch(pattern *SensitivePattern, match string) bool {
	if pattern.Confidence >= m.config.MinConfidence {
		return true
	}

	// 根据模式类型进行额外验证
	switch pattern.Category {
	case "PII":
		// 对PII进行额外格式验证
		return len(match) >= 6
	case "PCI":
		// 对信用卡进行Luhn校验
		return m.luhnCheck(match)
	case "Secrets":
		// 对密钥进行长度验证
		return len(match) >= 16
	}

	return true
}

// luhnCheck Luhn校验算法.
func (m *Manager) luhnCheck(number string) bool {
	// 移除分隔符
	cleaned := strings.ReplaceAll(strings.ReplaceAll(number, "-", ""), " ", "")

	sum := 0
	alternate := false

	for i := len(cleaned) - 1; i >= 0; i-- {
		n := int(cleaned[i] - '0')
		if n < 0 || n > 9 {
			return false
		}
		if alternate {
			n *= 2
			if n > 9 {
				n -= 9
			}
		}
		sum += n
		alternate = !alternate
	}

	return sum%10 == 0
}

// createViolation 创建违规记录.
func (m *Manager) createViolation(req *ScanRequest, pattern *SensitivePattern, matches []MatchedContent) *Violation {
	// 匹配策略
	policy := m.findMatchingPolicy(pattern, req)
	if policy == nil {
		// 没有匹配的策略，创建默认违规记录
		return &Violation{
			ID:          generateID(),
			PatternID:   pattern.ID,
			Level:       pattern.Level,
			Status:      ViolationStatusNew,
			UserID:      req.UserID,
			Resource:    req.Resource,
			Channel:     req.Channel,
			SourceIP:    req.SourceIP,
			Destination: req.Destination,
			MatchCount:  len(matches),
			MatchedData: matches,
			Action:      m.configToAction(),
			Blocked:     m.config.AutoBlock,
			Timestamp:   time.Now(),
		}
	}

	return &Violation{
		ID:          generateID(),
		PolicyID:    policy.ID,
		PolicyName:  policy.Name,
		PatternID:   pattern.ID,
		Level:       pattern.Level,
		Status:      ViolationStatusNew,
		UserID:      req.UserID,
		Resource:    req.Resource,
		Channel:     req.Channel,
		SourceIP:    req.SourceIP,
		Destination: req.Destination,
		MatchCount:  len(matches),
		MatchedData: matches,
		Action:      policy.Action,
		Blocked:     policy.Action == ActionBlock || policy.Action == ActionQuarantine,
		Timestamp:   time.Now(),
	}
}

// findMatchingPolicy 查找匹配的策略.
func (m *Manager) findMatchingPolicy(pattern *SensitivePattern, req *ScanRequest) *DLPPolicy {
	var matchedPolicy *DLPPolicy

	for _, policy := range m.policies {
		if !policy.Enabled {
			continue
		}

		// 检查模式是否包含在策略中
		patternMatched := false
		for _, pid := range policy.Patterns {
			if pid == pattern.ID {
				patternMatched = true
				break
			}
		}
		if !patternMatched {
			continue
		}

		// 检查用户匹配
		if len(policy.Users) > 0 && !containsStr(policy.Users, req.UserID) {
			continue
		}

		// 检查通道匹配
		if len(policy.Channels) > 0 && req.Channel != "" {
			channelMatched := false
			for _, ch := range policy.Channels {
				if ch == req.Channel {
					channelMatched = true
					break
				}
			}
			if !channelMatched {
				continue
			}
		}

		// 选择优先级最高的策略
		if matchedPolicy == nil || policy.Priority > matchedPolicy.Priority {
			matchedPolicy = policy
		}
	}

	return matchedPolicy
}

// determineAction 确定动作.
func (m *Manager) determineAction(violations []*Violation) PolicyAction {
	if len(violations) == 0 {
		return ActionLog
	}

	// 使用最严格的动作
	actionPriority := map[PolicyAction]int{
		ActionBlock:      5,
		ActionQuarantine: 4,
		ActionEncrypt:    3,
		ActionRedact:     2,
		ActionWarn:       1,
		ActionNotify:     1,
		ActionLog:        0,
	}

	maxAction := ActionLog
	maxPriority := 0

	for _, v := range violations {
		priority := actionPriority[v.Action]
		if priority > maxPriority {
			maxPriority = priority
			maxAction = v.Action
		}
	}

	return maxAction
}

// configToAction 根据配置确定动作.
func (m *Manager) configToAction() PolicyAction {
	if m.config.AutoBlock {
		return ActionBlock
	}
	return ActionWarn
}

// sensitivityValue 敏感级别数值.
func (m *Manager) sensitivityValue(level SensitivityLevel) int {
	switch level {
	case SensitivityCritical:
		return 5
	case SensitivityHigh:
		return 4
	case SensitivityMedium:
		return 3
	case SensitivityLow:
		return 2
	case SensitivityNone:
		return 1
	default:
		return 0
	}
}

// trimViolations 裁剪违规记录.
func (m *Manager) trimViolations() {
	m.mu.Lock()
	defer m.mu.Unlock()

	maxRecords := 10000
	if len(m.violations) > maxRecords {
		m.violations = m.violations[len(m.violations)-maxRecords:]
	}
}

// containsStr 检查字符串切片是否包含元素.
func containsStr(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// CreatePattern 创建敏感数据模式.
func (m *Manager) CreatePattern(pattern *SensitivePattern) (*SensitivePattern, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pattern.ID == "" {
		pattern.ID = generateID()
	}
	pattern.CreatedAt = time.Now()
	pattern.LastUpdated = time.Now()

	// 验证正则表达式
	if pattern.Type == PatternRegex {
		if _, err := regexp.Compile(pattern.Pattern); err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	m.patterns[pattern.ID] = pattern
	return pattern, nil
}

// GetPattern 获取模式.
func (m *Manager) GetPattern(id string) (*SensitivePattern, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pattern, ok := m.patterns[id]
	if !ok {
		return nil, fmt.Errorf("pattern not found: %s", id)
	}
	return pattern, nil
}

// ListPatterns 列出所有模式.
func (m *Manager) ListPatterns() []*SensitivePattern {
	m.mu.RLock()
	defer m.mu.RUnlock()

	patterns := make([]*SensitivePattern, 0, len(m.patterns))
	for _, p := range m.patterns {
		patterns = append(patterns, p)
	}
	return patterns
}

// UpdatePattern 更新模式.
func (m *Manager) UpdatePattern(id string, pattern *SensitivePattern) (*SensitivePattern, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.patterns[id]
	if !ok {
		return nil, fmt.Errorf("pattern not found: %s", id)
	}

	if pattern.Type == PatternRegex {
		if _, err := regexp.Compile(pattern.Pattern); err != nil {
			return nil, fmt.Errorf("invalid regex pattern: %w", err)
		}
	}

	existing.Name = pattern.Name
	existing.Description = pattern.Description
	existing.Type = pattern.Type
	existing.Level = pattern.Level
	existing.Pattern = pattern.Pattern
	existing.IsEnabled = pattern.IsEnabled
	existing.Category = pattern.Category
	existing.Tags = pattern.Tags
	existing.Confidence = pattern.Confidence
	existing.LastUpdated = time.Now()

	return existing, nil
}

// DeletePattern 删除模式.
func (m *Manager) DeletePattern(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.patterns[id]; !ok {
		return fmt.Errorf("pattern not found: %s", id)
	}
	delete(m.patterns, id)
	return nil
}

// GetPolicy 获取策略.
func (m *Manager) GetPolicy(id string) (*DLPPolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy not found: %s", id)
	}
	return policy, nil
}

// ListPolicies 列出所有策略.
func (m *Manager) ListPolicies() []*DLPPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policies := make([]*DLPPolicy, 0, len(m.policies))
	for _, p := range m.policies {
		policies = append(policies, p)
	}
	return policies
}

// UpdatePolicy 更新策略.
func (m *Manager) UpdatePolicy(id string, policy *DLPPolicy) (*DLPPolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy not found: %s", id)
	}

	existing.Name = policy.Name
	existing.Description = policy.Description
	existing.Enabled = policy.Enabled
	existing.Priority = policy.Priority
	existing.Action = policy.Action
	existing.Level = policy.Level
	existing.Patterns = policy.Patterns
	existing.Channels = policy.Channels
	existing.Users = policy.Users
	existing.Groups = policy.Groups
	existing.Exceptions = policy.Exceptions
	existing.NotifyEmail = policy.NotifyEmail
	existing.MaxMatches = policy.MaxMatches
	existing.UpdatedAt = time.Now()

	return existing, nil
}

// DeletePolicy 删除策略.
func (m *Manager) DeletePolicy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.policies[id]; !ok {
		return fmt.Errorf("policy not found: %s", id)
	}
	delete(m.policies, id)
	return nil
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() *ScanStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	return &stats
}

// GetConfig 获取配置.
func (m *Manager) GetConfig() *DLPConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置.
func (m *Manager) UpdateConfig(cfg *DLPConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}
