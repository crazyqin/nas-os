// Package quota 提供存储配额管理和告警功能
package quota

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrRuleNotFound 配额规则不存在
	ErrRuleNotFound = errors.New("配额规则不存在")
	// ErrRuleExists 配额规则已存在
	ErrRuleExists = errors.New("配额规则已存在")
	// ErrQuotaExceeded 超出配额限制
	ErrQuotaExceeded = errors.New("超出配额限制")
	// ErrInvalidTarget 无效目标
	ErrInvalidTarget = errors.New("无效的配额目标")
	// ErrInvalidMaxBytes 无效容量限制
	ErrInvalidMaxBytes = errors.New("无效的容量限制")
)

// ========== 存储接口 ==========

// StorageProvider 存储信息提供者接口
type StorageProvider interface {
	// GetVolumeUsage 获取卷使用情况
	GetVolumeUsage(volumeName string) (total, used, free int64, err error)
	// GetUserUsage 获取用户存储使用量
	GetUserUsage(username, volumeName string) (used int64, err error)
	// GetGroupUsage 获取组存储使用量
	GetGroupUsage(groupName, volumeName string) (used int64, err error)
}

// UserProvider 用户信息提供者接口
type UserProvider interface {
	UserExists(username string) bool
	GroupExists(groupName string) bool
}

// Notifier 通知发送接口
type Notifier interface {
	SendAlert(alert *Alert, config *NotificationConfig) error
}

// ========== Manager 配额管理器 ==========

// Manager 配额管理器
type Manager struct {
	mu            sync.RWMutex
	rules         map[string]*QuotaRule // ruleID -> QuotaRule
	alerts        map[string]*Alert     // alertID -> Alert
	alertHistory  []*Alert
	configPath    string
	storageProv   StorageProvider
	userProv      UserProvider
	notifier      Notifier
	notifyConfig  NotificationConfig
	coolDownTrack map[string]time.Time // targetID -> last alert time
}

// NewManager 创建配额管理器
func NewManager(configPath string, storage StorageProvider, user UserProvider) (*Manager, error) {
	m := &Manager{
		rules:         make(map[string]*QuotaRule),
		alerts:        make(map[string]*Alert),
		alertHistory:  make([]*Alert, 0),
		configPath:    configPath,
		storageProv:   storage,
		userProv:      user,
		notifyConfig:  DefaultNotificationConfig(),
		coolDownTrack: make(map[string]time.Time),
	}

	if configPath != "" {
		if err := m.loadConfig(); err != nil {
			return nil, fmt.Errorf("加载配额配置失败: %w", err)
		}
	}

	return m, nil
}

// SetNotifier 设置通知器
func (m *Manager) SetNotifier(n Notifier) {
	m.mu.Lock()
	m.notifier = n
	m.mu.Unlock()
}

// SetNotifyConfig 设置通知配置
func (m *Manager) SetNotifyConfig(config NotificationConfig) {
	m.mu.Lock()
	m.notifyConfig = config
	m.mu.Unlock()
}

// ========== 配额规则管理 ==========

// CreateRule 创建配额规则
func (m *Manager) CreateRule(input QuotaRuleInput) (*QuotaRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证目标类型
	targetType := TargetType(input.TargetType)
	if targetType != TargetTypeUser && targetType != TargetTypeGroup && targetType != TargetTypeVolume {
		return nil, ErrInvalidTarget
	}

	// 验证目标存在
	if m.userProv != nil {
		if targetType == TargetTypeUser && !m.userProv.UserExists(input.TargetID) {
			return nil, ErrInvalidTarget
		}
		if targetType == TargetTypeGroup && !m.userProv.GroupExists(input.TargetID) {
			return nil, ErrInvalidTarget
		}
	}

	// 验证容量
	if input.MaxBytes <= 0 {
		return nil, ErrInvalidMaxBytes
	}

	// 检查重复
	for _, rule := range m.rules {
		if rule.TargetType == input.TargetType && rule.TargetID == input.TargetID {
			return nil, ErrRuleExists
		}
	}

	// 设置默认值
	warnPercent := input.WarnPercent
	if warnPercent <= 0 || warnPercent > 100 {
		warnPercent = 80 // 默认80%告警
	}

	action := input.Action
	if action == "" {
		action = string(ActionNotify)
	}

	rule := &QuotaRule{
		ID:          generateID(),
		TargetType:  input.TargetType,
		TargetID:    input.TargetID,
		MaxBytes:    input.MaxBytes,
		WarnPercent: warnPercent,
		Action:      action,
		Enabled:     input.Enabled,
		CreatedAt:   time.Now(),
	}

	m.rules[rule.ID] = rule
	_ = m.saveConfig()

	return rule, nil
}

// GetRule 获取配额规则
func (m *Manager) GetRule(id string) (*QuotaRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, ErrRuleNotFound
	}
	return rule, nil
}

// ListRules 列出所有配额规则
func (m *Manager) ListRules() []*QuotaRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*QuotaRule, 0, len(m.rules))
	for _, r := range m.rules {
		result = append(result, r)
	}
	return result
}

// UpdateRule 更新配额规则
func (m *Manager) UpdateRule(id string, input QuotaRuleInput) (*QuotaRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, ErrRuleNotFound
	}

	// 验证容量
	if input.MaxBytes <= 0 {
		return nil, ErrInvalidMaxBytes
	}

	rule.MaxBytes = input.MaxBytes
	if input.WarnPercent > 0 && input.WarnPercent <= 100 {
		rule.WarnPercent = input.WarnPercent
	}
	if input.Action != "" {
		rule.Action = input.Action
	}
	rule.Enabled = input.Enabled
	rule.UpdatedAt = time.Now()

	_ = m.saveConfig()
	return rule, nil
}

// DeleteRule 删除配额规则
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[id]; !exists {
		return ErrRuleNotFound
	}

	delete(m.rules, id)
	_ = m.saveConfig()
	return nil
}

// ========== 配额使用查询 ==========

// GetUsage 获取配额使用情况
func (m *Manager) GetUsage(ruleID string) (*QuotaUsage, error) {
	m.mu.RLock()
	rule, exists := m.rules[ruleID]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrRuleNotFound
	}

	return m.calculateUsage(rule)
}

// GetAllUsage 获取所有配额使用情况
func (m *Manager) GetAllUsage() []*QuotaUsage {
	m.mu.RLock()
	rules := make([]*QuotaRule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	m.mu.RUnlock()

	result := make([]*QuotaUsage, 0, len(rules))
	for _, r := range rules {
		usage, err := m.calculateUsage(r)
		if err != nil {
			continue
		}
		result = append(result, usage)
	}
	return result
}

// calculateUsage 计算配额使用情况
func (m *Manager) calculateUsage(rule *QuotaRule) (*QuotaUsage, error) {
	usage := &QuotaUsage{
		RuleID:   rule.ID,
		TargetID: rule.TargetID,
		MaxBytes: rule.MaxBytes,
	}

	// 获取实际使用量
	used, err := m.getTargetUsage(rule)
	if err != nil {
		used = 0
	}
	usage.UsedBytes = used

	// 计算百分比
	if rule.MaxBytes > 0 {
		usage.Percent = float64(used) / float64(rule.MaxBytes) * 100
	}

	// 判断状态
	if usage.Percent >= 100 {
		usage.Status = string(StatusExceeded)
	} else if usage.Percent >= float64(rule.WarnPercent) {
		usage.Status = string(StatusWarning)
	} else {
		usage.Status = string(StatusNormal)
	}

	return usage, nil
}

// getTargetUsage 获取目标使用量
func (m *Manager) getTargetUsage(rule *QuotaRule) (int64, error) {
	if m.storageProv == nil {
		return 0, errors.New("存储提供者未设置")
	}

	switch TargetType(rule.TargetType) {
	case TargetTypeUser:
		return m.storageProv.GetUserUsage(rule.TargetID, "")
	case TargetTypeGroup:
		return m.storageProv.GetGroupUsage(rule.TargetID, "")
	case TargetTypeVolume:
		_, used, _, err := m.storageProv.GetVolumeUsage(rule.TargetID)
		return used, err
	default:
		return 0, ErrInvalidTarget
	}
}

// ========== 告警管理 ==========

// CheckAndAlert 检查并生成告警
func (m *Manager) CheckAndAlert() []*Alert {
	m.mu.RLock()
	rules := make([]*QuotaRule, 0, len(m.rules))
	for _, r := range m.rules {
		if r.Enabled {
			rules = append(rules, r)
		}
	}
	m.mu.RUnlock()

	newAlerts := make([]*Alert, 0)
	for _, rule := range rules {
		usage, err := m.calculateUsage(rule)
		if err != nil {
			continue
		}

		alert := m.evaluateUsage(rule, usage)
		if alert != nil {
			newAlerts = append(newAlerts, alert)
			m.addAlert(alert)
		}
	}

	return newAlerts
}

// evaluateUsage 评估使用情况并生成告警
func (m *Manager) evaluateUsage(rule *QuotaRule, usage *QuotaUsage) *Alert {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查冷却时间
	if lastAlert, ok := m.coolDownTrack[rule.TargetID]; ok {
		coolDown := time.Duration(m.notifyConfig.CoolDownMin) * time.Minute
		if time.Since(lastAlert) < coolDown {
			return nil // 冷却期内不重复告警
		}
	}

	var alert *Alert

	if usage.Percent >= 100 {
		// 超限告警
		alert = &Alert{
			ID:        generateID(),
			Type:      string(AlertTypeExceeded),
			Target:    rule.TargetID,
			Percent:   usage.Percent,
			Message:   fmt.Sprintf("配额已超限：%s 使用量 %.1f%%，已超过限制", rule.TargetID, usage.Percent),
			CreatedAt: time.Now(),
		}

		// 执行action
		if rule.Action == string(ActionBlock) {
			// TODO: 实际阻止写入逻辑
		}
	} else if usage.Percent >= float64(rule.WarnPercent) {
		// 警告告警
		alert = &Alert{
			ID:        generateID(),
			Type:      string(AlertTypeWarning),
			Target:    rule.TargetID,
			Percent:   usage.Percent,
			Message:   fmt.Sprintf("配额警告：%s 使用量 %.1f%%，已达到 %d%% 告警阈值", rule.TargetID, usage.Percent, rule.WarnPercent),
			CreatedAt: time.Now(),
		}
	}

	if alert != nil {
		m.coolDownTrack[rule.TargetID] = time.Now()
	}

	return alert
}

// addAlert 添加告警
func (m *Manager) addAlert(alert *Alert) {
	m.mu.Lock()
	m.alerts[alert.ID] = alert
	m.mu.Unlock()

	// 发送通知
	if m.notifier != nil && m.notifyConfig.Enabled {
		go m.notifier.SendAlert(alert, &m.notifyConfig)
	}
}

// GetAlerts 获取活跃告警
func (m *Manager) GetAlerts() []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Alert, 0, len(m.alerts))
	for _, a := range m.alerts {
		if !a.Resolved {
			result = append(result, a)
		}
	}
	return result
}

// GetAlertHistory 获取告警历史
func (m *Manager) GetAlertHistory(limit int) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.alertHistory) {
		limit = len(m.alertHistory)
	}

	result := make([]*Alert, limit)
	copy(result, m.alertHistory[len(m.alertHistory)-limit:])
	return result
}

// ResolveAlert 解决告警
func (m *Manager) ResolveAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[alertID]
	if !exists {
		return errors.New("告警不存在")
	}

	alert.Resolved = true
	m.alertHistory = append(m.alertHistory, alert)
	delete(m.alerts, alertID)

	_ = m.saveConfig()
	return nil
}

// ========== 配额检查 ==========

// CheckQuota 检查是否允许写入
func (m *Manager) CheckQuota(targetType, targetID string, additionalBytes int64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 查找匹配的规则
	var matchedRule *QuotaRule
	for _, rule := range m.rules {
		if rule.TargetType == targetType && rule.TargetID == targetID && rule.Enabled {
			matchedRule = rule
			break
		}
	}

	if matchedRule == nil {
		return nil // 无配额限制
	}

	// 计算当前使用量
	usage, err := m.calculateUsage(matchedRule)
	if err != nil {
		return err
	}

	// 检查是否会超限
	if usage.UsedBytes+additionalBytes > matchedRule.MaxBytes {
		return ErrQuotaExceeded
	}

	return nil
}

// ========== 持久化 ==========

type persistentConfig struct {
	Rules        []*QuotaRule     `json:"rules"`
	Alerts       []*Alert         `json:"alerts"`
	AlertHistory []*Alert         `json:"alert_history"`
	NotifyConfig NotificationConfig `json:"notify_config"`
}

func (m *Manager) loadConfig() error {
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

	var pc persistentConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	for _, r := range pc.Rules {
		m.rules[r.ID] = r
	}
	for _, a := range pc.Alerts {
		m.alerts[a.ID] = a
	}
	m.alertHistory = pc.AlertHistory
	m.notifyConfig = pc.NotifyConfig

	return nil
}

func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	pc := persistentConfig{
		Rules:        make([]*QuotaRule, 0, len(m.rules)),
		Alerts:       make([]*Alert, 0, len(m.alerts)),
		AlertHistory: m.alertHistory,
		NotifyConfig: m.notifyConfig,
	}

	for _, r := range m.rules {
		pc.Rules = append(pc.Rules, r)
	}
	for _, a := range m.alerts {
		pc.Alerts = append(pc.Alerts, a)
	}

	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0600)
}

// ========== 工具函数 ==========

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		// 使用时间戳作为后备
		b = []byte(fmt.Sprintf("%d%d", time.Now().UnixNano(), time.Now().Nanosecond()))
	}
	return hex.EncodeToString(b)[:16]
}

// GetDirSize 获取目录大小（辅助函数）
func GetDirSize(path string) (int64, error) {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return 0, nil
	}

	cmd := exec.Command("du", "-sb", path)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("计算目录大小失败: %w", err)
	}

	var size int64
	_, _ = fmt.Sscanf(string(output), "%d", &size)
	return size, nil
}