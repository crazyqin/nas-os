// Package quota 提供存储配额管理和告警功能
package quota

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 告警规则错误 ==========

var (
	// ErrAlertRuleNotFound 告警规则不存在
	ErrAlertRuleNotFound = errors.New("告警规则不存在")
	// ErrAlertRuleExists 告警规则已存在
	ErrAlertRuleExists = errors.New("告警规则已存在")
	// ErrInvalidThreshold 无效阈值
	ErrInvalidThreshold = errors.New("无效的阈值")
)

// AlertRuleManager 告警规则管理器
type AlertRuleManager struct {
	mu         sync.RWMutex
	rules      map[string]*AlertRule
	alertState map[string]*alertState // ruleID + targetID -> state
	configPath string
}

type alertState struct {
	LastAlertTime time.Time
	LastThreshold int
	AlertCount    int
}

// NewAlertRuleManager 创建告警规则管理器
func NewAlertRuleManager(configPath string) (*AlertRuleManager, error) {
	m := &AlertRuleManager{
		rules:      make(map[string]*AlertRule),
		alertState: make(map[string]*alertState),
		configPath: configPath,
	}

	if configPath != "" {
		if err := m.loadConfig(); err != nil {
			return nil, fmt.Errorf("加载告警规则配置失败: %w", err)
		}
	}

	return m, nil
}

// CreateRule 创建告警规则
func (m *AlertRuleManager) CreateRule(input AlertRuleInput) (*AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证阈值
	if len(input.Thresholds) == 0 {
		return nil, ErrInvalidThreshold
	}
	for _, t := range input.Thresholds {
		if t <= 0 || t > 100 {
			return nil, ErrInvalidThreshold
		}
	}

	// 设置默认值
	if input.TargetType == "" {
		input.TargetType = "*"
	}
	if input.TargetID == "" {
		input.TargetID = "*"
	}
	if len(input.Channels) == 0 {
		input.Channels = []string{"email"}
	}

	rule := &AlertRule{
		ID:            generateAlertRuleID(),
		Name:          input.Name,
		TargetType:    input.TargetType,
		TargetID:      input.TargetID,
		Thresholds:    input.Thresholds,
		Channels:      input.Channels,
		Enabled:       input.Enabled,
		ScheduleStart: input.ScheduleStart,
		ScheduleEnd:   input.ScheduleEnd,
		RepeatEnabled: input.RepeatEnabled,
		RepeatHours:   input.RepeatHours,
		CreatedAt:     time.Now(),
	}

	m.rules[rule.ID] = rule
	_ = m.saveConfig()

	return rule, nil
}

// GetRule 获取告警规则
func (m *AlertRuleManager) GetRule(id string) (*AlertRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, ErrAlertRuleNotFound
	}
	return rule, nil
}

// ListRules 列出所有告警规则
func (m *AlertRuleManager) ListRules() []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AlertRule, 0, len(m.rules))
	for _, r := range m.rules {
		result = append(result, r)
	}
	return result
}

// UpdateRule 更新告警规则
func (m *AlertRuleManager) UpdateRule(id string, input AlertRuleInput) (*AlertRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, ErrAlertRuleNotFound
	}

	// 验证阈值
	if len(input.Thresholds) > 0 {
		for _, t := range input.Thresholds {
			if t <= 0 || t > 100 {
				return nil, ErrInvalidThreshold
			}
		}
		rule.Thresholds = input.Thresholds
	}

	if input.Name != "" {
		rule.Name = input.Name
	}
	if input.TargetType != "" {
		rule.TargetType = input.TargetType
	}
	if input.TargetID != "" {
		rule.TargetID = input.TargetID
	}
	if len(input.Channels) > 0 {
		rule.Channels = input.Channels
	}
	rule.Enabled = input.Enabled
	rule.ScheduleStart = input.ScheduleStart
	rule.ScheduleEnd = input.ScheduleEnd
	rule.RepeatEnabled = input.RepeatEnabled
	rule.RepeatHours = input.RepeatHours

	_ = m.saveConfig()
	return rule, nil
}

// DeleteRule 删除告警规则
func (m *AlertRuleManager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[id]; !exists {
		return ErrAlertRuleNotFound
	}

	delete(m.rules, id)
	_ = m.saveConfig()
	return nil
}

// ShouldAlert 检查是否应该触发告警
func (m *AlertRuleManager) ShouldAlert(targetType, targetID string, currentPercent float64) []*AlertRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var matchedRules []*AlertRule

	for _, rule := range m.rules {
		if !rule.Enabled {
			continue
		}

		// 检查目标匹配
		if rule.TargetType != "*" && rule.TargetType != targetType {
			continue
		}
		if rule.TargetID != "*" && rule.TargetID != targetID {
			continue
		}

		// 检查时间窗口
		if !m.isInScheduleWindow(rule) {
			continue
		}

		// 检查是否达到阈值
		thresholdCrossed := false
		for _, threshold := range rule.Thresholds {
			if currentPercent >= float64(threshold) {
				thresholdCrossed = true
				break
			}
		}

		if !thresholdCrossed {
			continue
		}

		// 检查重复告警设置
		stateKey := rule.ID + "_" + targetID
		state, exists := m.alertState[stateKey]

		if exists && !rule.RepeatEnabled {
			// 不重复告警，检查是否已告警过
			continue
		}

		if exists && rule.RepeatEnabled && rule.RepeatHours > 0 {
			// 检查重复间隔
			nextAlertTime := state.LastAlertTime.Add(time.Duration(rule.RepeatHours) * time.Hour)
			if time.Now().Before(nextAlertTime) {
				continue
			}
		}

		matchedRules = append(matchedRules, rule)
	}

	return matchedRules
}

// RecordAlert 记录告警已发送
func (m *AlertRuleManager) RecordAlert(ruleID, targetID string, threshold int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stateKey := ruleID + "_" + targetID
	state, exists := m.alertState[stateKey]
	if !exists {
		state = &alertState{}
	}

	state.LastAlertTime = time.Now()
	state.LastThreshold = threshold
	state.AlertCount++
	m.alertState[stateKey] = state
}

// isInScheduleWindow 检查是否在告警时间窗口内
func (m *AlertRuleManager) isInScheduleWindow(rule *AlertRule) bool {
	if rule.ScheduleStart == "" || rule.ScheduleEnd == "" {
		return true // 未设置时间窗口，始终允许
	}

	now := time.Now()
	currentTime := now.Format("15:04")

	startTime := rule.ScheduleStart
	endTime := rule.ScheduleEnd

	// 处理跨天情况（如 22:00 - 06:00）
	if startTime > endTime {
		// 跨天
		return currentTime >= startTime || currentTime <= endTime
	}

	return currentTime >= startTime && currentTime <= endTime
}

// GetAlertStats 获取告警统计
func (m *AlertRuleManager) GetAlertStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalAlerts := 0
	for _, state := range m.alertState {
		totalAlerts += state.AlertCount
	}

	return map[string]interface{}{
		"rule_count":     len(m.rules),
		"enabled_rules":  m.countEnabledRules(),
		"total_alerts":   totalAlerts,
		"tracked_states": len(m.alertState),
	}
}

func (m *AlertRuleManager) countEnabledRules() int {
	count := 0
	for _, rule := range m.rules {
		if rule.Enabled {
			count++
		}
	}
	return count
}

// ========== 持久化 ==========

type alertRuleConfig struct {
	Rules []*AlertRule `json:"rules"`
}

func (m *AlertRuleManager) loadConfig() error {
	if m.configPath == "" {
		return nil
	}

	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var cfg alertRuleConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	for _, r := range cfg.Rules {
		m.rules[r.ID] = r
	}

	return nil
}

func (m *AlertRuleManager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	cfg := alertRuleConfig{
		Rules: make([]*AlertRule, 0, len(m.rules)),
	}

	for _, r := range m.rules {
		cfg.Rules = append(cfg.Rules, r)
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0600)
}

func generateAlertRuleID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("ar_%d", time.Now().UnixNano())
	}
	return "ar_" + hex.EncodeToString(b)
}

// ========== 预设告警规则 ==========

// DefaultAlertRules 默认告警规则
func DefaultAlertRules() []*AlertRuleInput {
	return []*AlertRuleInput{
		{
			Name:       "低容量警告",
			TargetType: "*",
			TargetID:   "*",
			Thresholds: []int{60},
			Channels:   []string{"email"},
			Enabled:    true,
		},
		{
			Name:       "中等容量警告",
			TargetType: "*",
			TargetID:   "*",
			Thresholds: []int{80},
			Channels:   []string{"email", "webhook"},
			Enabled:    true,
		},
		{
			Name:       "高容量警告",
			TargetType: "*",
			TargetID:   "*",
			Thresholds: []int{90},
			Channels:   []string{"email", "webhook"},
			Enabled:    true,
		},
		{
			Name:          "紧急容量警告",
			TargetType:    "*",
			TargetID:      "*",
			Thresholds:    []int{95},
			Channels:      []string{"email", "webhook"},
			Enabled:       true,
			RepeatEnabled: true,
			RepeatHours:   24,
		},
	}
}

// InitDefaultRules 初始化默认告警规则
func (m *AlertRuleManager) InitDefaultRules() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已有规则
	if len(m.rules) > 0 {
		return nil
	}

	for _, input := range DefaultAlertRules() {
		rule := &AlertRule{
			ID:            generateAlertRuleID(),
			Name:          input.Name,
			TargetType:    input.TargetType,
			TargetID:      input.TargetID,
			Thresholds:    input.Thresholds,
			Channels:      input.Channels,
			Enabled:       input.Enabled,
			ScheduleStart: input.ScheduleStart,
			ScheduleEnd:   input.ScheduleEnd,
			RepeatEnabled: input.RepeatEnabled,
			RepeatHours:   input.RepeatHours,
			CreatedAt:     time.Now(),
		}
		m.rules[rule.ID] = rule
	}

	return m.saveConfig()
}