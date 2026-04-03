// Package alerting 提供告警系统核心功能
// 包括自定义规则、静默配置、告警聚合等
package alerting

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ========== 常量和类型定义 ==========

// RuleGroup 规则分组
type RuleGroup string

const (
	GroupStorage  RuleGroup = "storage"  // 存储相关（磁盘、容量、IO）
	GroupNetwork  RuleGroup = "network"  // 网络相关（带宽、延迟、连接）
	GroupSystem   RuleGroup = "system"   // 系统相关（CPU、内存、进程）
	GroupSecurity RuleGroup = "security" // 安全相关（入侵、异常访问）
)

// MetricType 指标类型
type MetricType string

const (
	MetricCPUUsage       MetricType = "cpu_usage_percent"
	MetricMemoryUsage    MetricType = "memory_usage_percent"
	MetricDiskUsage      MetricType = "disk_usage_percent"
	MetricDiskIO         MetricType = "disk_io_latency_ms"
	MetricDiskRead       MetricType = "disk_read_bytes"
	MetricDiskWrite      MetricType = "disk_write_bytes"
	MetricNetworkIn      MetricType = "network_in_bytes"
	MetricNetworkOut     MetricType = "network_out_bytes"
	MetricNetworkLatency MetricType = "network_latency_ms"
	MetricTemperature    MetricType = "temperature_celsius"
	MetricServiceStatus  MetricType = "service_status"
)

// Operator 比较操作符
type Operator string

const (
	OpGreaterThan  Operator = ">"
	OpGreaterEqual Operator = ">="
	OpLessThan     Operator = "<"
	OpLessEqual    Operator = "<="
	OpEqual        Operator = "=="
	OpNotEqual     Operator = "!="
)

// ========== 错误定义 ==========

var (
	ErrRuleNotFound      = errors.New("rule not found")
	ErrRuleDisabled      = errors.New("rule is disabled")
	ErrInvalidThreshold  = errors.New("invalid threshold value")
	ErrInvalidDuration   = errors.New("invalid duration")
	ErrRuleNameDuplicate = errors.New("rule name already exists")
)

// ========== 自定义告警规则 ==========

// CustomAlertRule 自定义告警规则（用户可配置）
type CustomAlertRule struct {
	// 基础信息
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description,omitempty"`
	Group       RuleGroup `json:"group"`
	Enabled     bool      `json:"enabled"`

	// 指标配置
	Metric    MetricType `json:"metric"`
	Operator  Operator   `json:"operator"`
	Threshold float64    `json:"threshold"`
	Duration  int        `json:"duration"` // 持续时间（秒），0表示立即触发

	// 告警级别
	Level AlertLevel `json:"level"`

	// 标签和注解
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`

	// 作用范围
	Scope       RuleScope `json:"scope,omitempty"`
	Targets     []string  `json:"targets,omitempty"`     // 具体目标（如磁盘名、服务名）
	Exclude     []string  `json:"exclude,omitempty"`     // 排除目标

	// 抑制配置
	InhibitBy   []string `json:"inhibit_by,omitempty"`   // 被哪些规则抑制
	InhibitFrom []string `json:"inhibit_from,omitempty"` // 抑制哪些规则

	// 告警动作
	Actions []AlertAction `json:"actions,omitempty"`

	// 时间信息
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`

	// 用户信息
	CreatedBy string `json:"created_by,omitempty"`

	// 内部状态（不持久化）
	pendingSince time.Time
	fireCount    int
	lastEvaluated time.Time
	mu           sync.RWMutex
}

// RuleScope 规则作用范围
type RuleScope struct {
	Devices    []string `json:"devices,omitempty"`    // 设备范围
	Services   []string `json:"services,omitempty"`   // 服务范围
	Users      []string `json:"users,omitempty"`      // 用户范围
	Networks   []string `json:"networks,omitempty"`   // 网络范围
	Containers []string `json:"containers,omitempty"` // 容器范围
}

// AlertAction 告警动作
type AlertAction struct {
	Type       string            `json:"type"` // notify, webhook, script, email
	Target     string            `json:"target,omitempty"`
	Params     map[string]string `json:"params,omitempty"`
	Enabled    bool              `json:"enabled"`
	Delay      int               `json:"delay,omitempty"` // 延迟执行（秒）
	Repeat     int               `json:"repeat,omitempty"` // 重复次数
	RepeatInterval int           `json:"repeat_interval,omitempty"` // 重复间隔（秒）
}

// NewCustomAlertRule 创建新规则
func NewCustomAlertRule(name string, group RuleGroup, metric MetricType) *CustomAlertRule {
	return &CustomAlertRule{
		ID:        generateRuleID(),
		Name:      name,
		Group:     group,
		Metric:    metric,
		Enabled:   true,
		Level:     AlertLevelWarning,
		Operator:  OpGreaterThan,
		Duration:  60, // 默认持续1分钟
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
		Labels:    make(map[string]string),
		Annotations: make(map[string]string),
	}
}

// Evaluate 评估规则（检查阈值是否触发）
func (r *CustomAlertRule) Evaluate(value float64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	r.lastEvaluated = time.Now()

	triggered := r.evaluateThreshold(value)

	if triggered {
		if r.Duration > 0 {
			// 需要持续一定时间才触发
			if r.pendingSince.IsZero() {
				r.pendingSince = time.Now()
				r.fireCount = 1
				return false // 还未达到持续时间
			}

		 elapsed := time.Since(r.pendingSince).Seconds()
			if elapsed < float64(r.Duration) {
				r.fireCount++
				return false
			}
			// 达到持续时间
			return true
		}
		// 立即触发
		return true
	}

	// 未触发，重置pending状态
	r.pendingSince = time.Time{}
	r.fireCount = 0
	return false
}

// evaluateThreshold 评估阈值
func (r *CustomAlertRule) evaluateThreshold(value float64) bool {
	switch r.Operator {
	case OpGreaterThan:
		return value > r.Threshold
	case OpGreaterEqual:
		return value >= r.Threshold
	case OpLessThan:
		return value < r.Threshold
	case OpLessEqual:
		return value <= r.Threshold
	case OpEqual:
		return value == r.Threshold
	case OpNotEqual:
		return value != r.Threshold
	default:
		return false
	}
}

// ResetPending 重置pending状态
func (r *CustomAlertRule) ResetPending() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.pendingSince = time.Time{}
	r.fireCount = 0
}

// GetState 获取规则状态
func (r *CustomAlertRule) GetState() RuleState {
	r.mu.RLock()
	defer r.mu.RUnlock()

	return RuleState{
		ID:            r.ID,
		Name:          r.Name,
		Enabled:       r.Enabled,
		LastEvaluated: r.lastEvaluated,
		PendingSince:  r.pendingSince,
		FireCount:     r.fireCount,
		IsPending:     !r.pendingSince.IsZero() && time.Since(r.pendingSince).Seconds() < float64(r.Duration),
	}
}

// Update 更新规则配置
func (r *CustomAlertRule) Update(updates RuleUpdate) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if updates.Name != nil {
		r.Name = *updates.Name
	}
	if updates.Description != nil {
		r.Description = *updates.Description
	}
	if updates.Threshold != nil {
		if *updates.Threshold < 0 {
			return ErrInvalidThreshold
		}
		r.Threshold = *updates.Threshold
	}
	if updates.Operator != nil {
		r.Operator = *updates.Operator
	}
	if updates.Duration != nil {
		if *updates.Duration < 0 {
			return ErrInvalidDuration
		}
		r.Duration = *updates.Duration
	}
	if updates.Level != nil {
		r.Level = *updates.Level
	}
	if updates.Enabled != nil {
		r.Enabled = *updates.Enabled
	}
	if updates.Group != nil {
		r.Group = *updates.Group
	}
	if updates.Labels != nil {
		r.Labels = updates.Labels
	}
	if updates.Annotations != nil {
		r.Annotations = updates.Annotations
	}
	if updates.Actions != nil {
		r.Actions = updates.Actions
	}
	if updates.Scope != nil {
		r.Scope = *updates.Scope
	}
	if updates.Targets != nil {
		r.Targets = updates.Targets
	}
	if updates.Exclude != nil {
		r.Exclude = updates.Exclude
	}

	r.UpdatedAt = time.Now()
	return nil
}

// RuleUpdate 规则更新请求
type RuleUpdate struct {
	Name        *string           `json:"name,omitempty"`
	Description *string           `json:"description,omitempty"`
	Threshold   *float64          `json:"threshold,omitempty"`
	Operator    *Operator         `json:"operator,omitempty"`
	Duration    *int              `json:"duration,omitempty"`
	Level       *AlertLevel       `json:"level,omitempty"`
	Enabled     *bool             `json:"enabled,omitempty"`
	Group       *RuleGroup        `json:"group,omitempty"`
	Labels      map[string]string `json:"labels,omitempty"`
	Annotations map[string]string `json:"annotations,omitempty"`
	Actions     []AlertAction     `json:"actions,omitempty"`
	Scope       *RuleScope        `json:"scope,omitempty"`
	Targets     []string          `json:"targets,omitempty"`
	Exclude     []string          `json:"exclude,omitempty"`
}

// RuleState 规则运行状态
type RuleState struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Enabled       bool      `json:"enabled"`
	LastEvaluated time.Time `json:"last_evaluated"`
	PendingSince  time.Time `json:"pending_since,omitempty"`
	FireCount     int       `json:"fire_count"`
	IsPending     bool      `json:"is_pending"`
}

// ========== 自定义规则管理器 ==========

// CustomRuleManager 自定义规则管理器
type CustomRuleManager struct {
	logger *zap.Logger
	mu     sync.RWMutex

	rules    map[string]*CustomAlertRule    // 按ID索引
	ruleList []*CustomAlertRule             // 规则列表
	groups   map[RuleGroup][]*CustomAlertRule // 按分组索引

	// 数据库持久化
	db DatabaseStore

	// 告警触发回调
	onAlert func(alert *CustomAlert)

	// 指标采集器接口
	collectors map[MetricType]MetricCollector

	// 配置
	config RuleManagerConfig
}

// RuleManagerConfig 规则管理器配置
type RuleManagerConfig struct {
	MaxRules        int           `json:"max_rules"`         // 最大规则数量
	CheckInterval   time.Duration `json:"check_interval"`    // 检查间隔
	DefaultDuration int           `json:"default_duration"`  // 默认持续时间（秒）
}

// DefaultRuleManagerConfig 默认配置
func DefaultRuleManagerConfig() RuleManagerConfig {
	return RuleManagerConfig{
		MaxRules:        100,
		CheckInterval:   30 * time.Second,
		DefaultDuration: 60,
	}
}

// MetricCollector 指标采集器接口
type MetricCollector interface {
	Collect(ctx context.Context) (map[string]float64, error)
}

// DatabaseStore 数据库存储接口
type DatabaseStore interface {
	SaveRule(rule *CustomAlertRule) error
	LoadRules() ([]*CustomAlertRule, error)
	DeleteRule(id string) error
	UpdateRule(rule *CustomAlertRule) error
}

// NewCustomRuleManager 创建自定义规则管理器
func NewCustomRuleManager(logger *zap.Logger, db DatabaseStore) *CustomRuleManager {
	mgr := &CustomRuleManager{
		logger:     logger,
		rules:      make(map[string]*CustomAlertRule),
		ruleList:   make([]*CustomAlertRule, 0),
		groups:     make(map[RuleGroup][]*CustomAlertRule),
		db:         db,
		collectors: make(map[MetricType]MetricCollector),
		config:     DefaultRuleManagerConfig(),
	}

	// 从数据库加载规则
	if db != nil {
		mgr.loadFromDatabase()
	}

	// 添加默认规则模板
	mgr.addDefaultTemplates()

	return mgr
}

// loadFromDatabase 从数据库加载规则
func (mgr *CustomRuleManager) loadFromDatabase() {
	if mgr.db == nil {
		return
	}

	rules, err := mgr.db.LoadRules()
	if err != nil {
		mgr.logger.Warn("加载规则失败", zap.Error(err))
		return
	}

	for _, rule := range rules {
		mgr.rules[rule.ID] = rule
		mgr.ruleList = append(mgr.ruleList, rule)
		mgr.groups[rule.Group] = append(mgr.groups[rule.Group], rule)
	}

	mgr.logger.Info("从数据库加载规则", zap.Int("count", len(rules)))
}

// addDefaultTemplates 添加默认规则模板
func (mgr *CustomRuleManager) addDefaultTemplates() {
	// 这些是模板，不会被持久化，用户可以基于模板创建自定义规则
	defaultTemplates := []*CustomAlertRule{
		// CPU规则
		{
			ID:        "template-cpu-high",
			Name:      "CPU使用率过高",
			Group:     GroupSystem,
			Metric:    MetricCPUUsage,
			Operator:  OpGreaterThan,
			Threshold: 80,
			Duration:  300, // 5分钟
			Level:     AlertLevelWarning,
			Enabled:   false, // 模板默认禁用
			Labels:    map[string]string{"template": "true"},
		},
		{
			ID:        "template-cpu-critical",
			Name:      "CPU使用率严重",
			Group:     GroupSystem,
			Metric:    MetricCPUUsage,
			Operator:  OpGreaterThan,
			Threshold: 95,
			Duration:  60,
			Level:     AlertLevelCritical,
			Enabled:   false,
			Labels:    map[string]string{"template": "true"},
		},

		// 内存规则
		{
			ID:        "template-memory-high",
			Name:      "内存使用率过高",
			Group:     GroupSystem,
			Metric:    MetricMemoryUsage,
			Operator:  OpGreaterThan,
			Threshold: 85,
			Duration:  300,
			Level:     AlertLevelWarning,
			Enabled:   false,
			Labels:    map[string]string{"template": "true"},
		},

		// 磁盘规则
		{
			ID:        "template-disk-space",
			Name:      "磁盘空间不足",
			Group:     GroupStorage,
			Metric:    MetricDiskUsage,
			Operator:  OpGreaterThan,
			Threshold: 85,
			Duration:  0, // 立即告警
			Level:     AlertLevelWarning,
			Enabled:   false,
			Labels:    map[string]string{"template": "true"},
		},
		{
			ID:        "template-disk-critical",
			Name:      "磁盘空间严重不足",
			Group:     GroupStorage,
			Metric:    MetricDiskUsage,
			Operator:  OpGreaterThan,
			Threshold: 95,
			Duration:  0,
			Level:     AlertLevelCritical,
			Enabled:   false,
			Labels:    map[string]string{"template": "true"},
		},

		// 网络规则
		{
			ID:        "template-network-latency",
			Name:      "网络延迟过高",
			Group:     GroupNetwork,
			Metric:    MetricNetworkLatency,
			Operator:  OpGreaterThan,
			Threshold: 100, // 100ms
			Duration:  180,
			Level:     AlertLevelWarning,
			Enabled:   false,
			Labels:    map[string]string{"template": "true"},
		},

		// 温度规则
		{
			ID:        "template-temperature",
			Name:      "设备温度过高",
			Group:     GroupSystem,
			Metric:    MetricTemperature,
			Operator:  OpGreaterThan,
			Threshold: 70, // 70度
			Duration:  120,
			Level:     AlertLevelWarning,
			Enabled:   false,
			Labels:    map[string]string{"template": "true"},
		},
	}

	for _, tmpl := range defaultTemplates {
		mgr.rules[tmpl.ID] = tmpl
		mgr.ruleList = append(mgr.ruleList, tmpl)
		mgr.groups[tmpl.Group] = append(mgr.groups[tmpl.Group], tmpl)
	}
}

// RegisterCollector 注册指标采集器
func (mgr *CustomRuleManager) RegisterCollector(metric MetricType, collector MetricCollector) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.collectors[metric] = collector
}

// SetOnAlert 设置告警回调
func (mgr *CustomRuleManager) SetOnAlert(callback func(alert *CustomAlert)) {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()
	mgr.onAlert = callback
}

// AddRule 添加自定义规则
func (mgr *CustomRuleManager) AddRule(rule *CustomAlertRule) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	// 检查数量限制
	if len(mgr.ruleList) >= mgr.config.MaxRules {
		return errors.New("exceeded maximum rules limit")
	}

	// 检查名称重复
	for _, existing := range mgr.ruleList {
		if existing.Name == rule.Name && existing.ID != rule.ID {
			return ErrRuleNameDuplicate
		}
	}

	// 生成ID（如果没有）
	if rule.ID == "" {
		rule.ID = generateRuleID()
	}
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	// 添加到管理器
	mgr.rules[rule.ID] = rule
	mgr.ruleList = append(mgr.ruleList, rule)
	mgr.groups[rule.Group] = append(mgr.groups[rule.Group], rule)

	// 持久化到数据库
	if mgr.db != nil {
		if err := mgr.db.SaveRule(rule); err != nil {
			mgr.logger.Error("保存规则失败", zap.Error(err), zap.String("rule_id", rule.ID))
			// 不回滚内存中的添加，因为用户可能想继续使用
		}
	}

	mgr.logger.Info("添加规则",
		zap.String("id", rule.ID),
		zap.String("name", rule.Name),
		zap.String("group", string(rule.Group)),
	)

	return nil
}

// UpdateRule 更新规则
func (mgr *CustomRuleManager) UpdateRule(id string, updates RuleUpdate) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	rule, ok := mgr.rules[id]
	if !ok {
		return ErrRuleNotFound
	}

	// 检查名称重复（如果更新了名称）
	if updates.Name != nil {
		for _, existing := range mgr.ruleList {
			if existing.Name == *updates.Name && existing.ID != id {
				return ErrRuleNameDuplicate
			}
		}
	}

	// 更新规则
	if err := rule.Update(updates); err != nil {
		return err
	}

	// 持久化
	if mgr.db != nil {
		if err := mgr.db.UpdateRule(rule); err != nil {
			mgr.logger.Error("更新规则失败", zap.Error(err), zap.String("rule_id", id))
		}
	}

	mgr.logger.Info("更新规则", zap.String("id", id))
	return nil
}

// DeleteRule 删除规则
func (mgr *CustomRuleManager) DeleteRule(id string) error {
	mgr.mu.Lock()
	defer mgr.mu.Unlock()

	rule, ok := mgr.rules[id]
	if !ok {
		return ErrRuleNotFound
	}

	// 从索引中删除
	delete(mgr.rules, id)

	// 从列表中删除
	for i, r := range mgr.ruleList {
		if r.ID == id {
			mgr.ruleList = append(mgr.ruleList[:i], mgr.ruleList[i+1:]...)
			break
		}
	}

	// 从分组中删除
	for i, r := range mgr.groups[rule.Group] {
		if r.ID == id {
			mgr.groups[rule.Group] = append(mgr.groups[rule.Group][:i], mgr.groups[rule.Group][i+1:]...)
			break
		}
	}

	// 从数据库删除
	if mgr.db != nil {
		if err := mgr.db.DeleteRule(id); err != nil {
			mgr.logger.Error("删除规则失败", zap.Error(err), zap.String("rule_id", id))
		}
	}

	mgr.logger.Info("删除规则", zap.String("id", id))
	return nil
}

// GetRule 获取规则
func (mgr *CustomRuleManager) GetRule(id string) (*CustomAlertRule, error) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	rule, ok := mgr.rules[id]
	if !ok {
		return nil, ErrRuleNotFound
	}
	return rule, nil
}

// GetRules 获取所有规则
func (mgr *CustomRuleManager) GetRules() []*CustomAlertRule {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	result := make([]*CustomAlertRule, len(mgr.ruleList))
	copy(result, mgr.ruleList)
	return result
}

// GetRulesByGroup 按分组获取规则
func (mgr *CustomRuleManager) GetRulesByGroup(group RuleGroup) []*CustomAlertRule {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	rules := mgr.groups[group]
	result := make([]*CustomAlertRule, len(rules))
	copy(result, rules)
	return result
}

// GetEnabledRules 获取启用的规则
func (mgr *CustomRuleManager) GetEnabledRules() []*CustomAlertRule {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	result := make([]*CustomAlertRule, 0)
	for _, rule := range mgr.ruleList {
		if rule.Enabled {
			result = append(result, rule)
		}
	}
	return result
}

// GetTemplates 获取模板规则
func (mgr *CustomRuleManager) GetTemplates() []*CustomAlertRule {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	result := make([]*CustomAlertRule, 0)
	for _, rule := range mgr.ruleList {
		if rule.Labels != nil && rule.Labels["template"] == "true" {
			result = append(result, rule)
		}
	}
	return result
}

// CreateFromTemplate 从模板创建规则
func (mgr *CustomRuleManager) CreateFromTemplate(templateID string, name string, customizations RuleUpdate) (*CustomAlertRule, error) {
	mgr.mu.RLock()
	tmpl, ok := mgr.rules[templateID]
	mgr.mu.RUnlock()

	if !ok {
		return nil, ErrRuleNotFound
	}

	// 复制模板
	newRule := &CustomAlertRule{
		ID:          generateRuleID(),
		Name:        name,
		Description: tmpl.Description,
		Group:       tmpl.Group,
		Metric:      tmpl.Metric,
		Operator:    tmpl.Operator,
		Threshold:   tmpl.Threshold,
		Duration:    tmpl.Duration,
		Level:       tmpl.Level,
		Enabled:     true, // 从模板创建的规则默认启用
		Labels:      make(map[string]string),
		Annotations: make(map[string]string),
		Actions:     make([]AlertAction, 0),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 应用自定义配置
	_ = newRule.Update(customizations)

	// 添加到管理器
	if err := mgr.AddRule(newRule); err != nil {
		return nil, err
	}

	return newRule, nil
}

// EvaluateAll 评估所有规则
func (mgr *CustomRuleManager) EvaluateAll(ctx context.Context) ([]*CustomAlert, error) {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	alerts := make([]*CustomAlert, 0)

	for metric, collector := range mgr.collectors {
		values, err := collector.Collect(ctx)
		if err != nil {
			mgr.logger.Warn("采集指标失败",
				zap.String("metric", string(metric)),
				zap.Error(err),
			)
			continue
		}

		// 评估使用此指标的规则
		for _, rule := range mgr.ruleList {
			if !rule.Enabled || rule.Metric != metric {
				continue
			}

			// 获取对应的指标值
			var value float64
			for target, v := range values {
				// 检查目标是否匹配
				if len(rule.Targets) > 0 {
					if !contains(rule.Targets, target) {
						continue
					}
				}
				if contains(rule.Exclude, target) {
					continue
				}

				value = v
				if rule.Evaluate(value) {
					alert := mgr.createAlert(rule, value, target)
					alerts = append(alerts, alert)
				}
			}
		}
	}

	return alerts, nil
}

// createAlert 创建告警
func (mgr *CustomRuleManager) createAlert(rule *CustomAlertRule, value float64, target string) *CustomAlert {
	return &CustomAlert{
		ID:           generateAlertID(),
		RuleID:       rule.ID,
		RuleName:     rule.Name,
		Group:        rule.Group,
		Level:        rule.Level,
		Metric:       rule.Metric,
		CurrentValue: value,
		Threshold:    rule.Threshold,
		Operator:     rule.Operator,
		Target:       target,
		Message:      fmt.Sprintf("%s: %.2f (阈值: %.2f)", rule.Name, value, rule.Threshold),
		Labels:       rule.Labels,
		Annotations:  rule.Annotations,
		Status:       AlertStatusFiring,
		StartsAt:     time.Now(),
		Actions:      rule.Actions,
	}
}

// Start 启动规则检查循环
func (mgr *CustomRuleManager) Start(ctx context.Context) {
	ticker := time.NewTicker(mgr.config.CheckInterval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				alerts, err := mgr.EvaluateAll(ctx)
				if err != nil {
					mgr.logger.Warn("规则评估失败", zap.Error(err))
					continue
				}

				// 触发告警回调
				if mgr.onAlert != nil {
					for _, alert := range alerts {
						go mgr.onAlert(alert)
					}
				}
			}
		}
	}()

	mgr.logger.Info("规则检查已启动",
		zap.Duration("interval", mgr.config.CheckInterval),
	)
}

// GetStats 获取统计信息
func (mgr *CustomRuleManager) GetStats() RuleManagerStats {
	mgr.mu.RLock()
	defer mgr.mu.RUnlock()

	stats := RuleManagerStats{
		TotalRules:  len(mgr.ruleList),
		EnabledRules: 0,
		Templates:   0,
		ByGroup:     make(map[RuleGroup]int),
		ByMetric:    make(map[MetricType]int),
		ByLevel:     make(map[AlertLevel]int),
	}

	for _, rule := range mgr.ruleList {
		if rule.Enabled {
			stats.EnabledRules++
		}
		if rule.Labels != nil && rule.Labels["template"] == "true" {
			stats.Templates++
		}
		stats.ByGroup[rule.Group]++
		stats.ByMetric[rule.Metric]++
		stats.ByLevel[rule.Level]++
	}

	return stats
}

// RuleManagerStats 规则管理器统计
type RuleManagerStats struct {
	TotalRules   int                    `json:"total_rules"`
	EnabledRules int                    `json:"enabled_rules"`
	Templates    int                    `json:"templates"`
	ByGroup      map[RuleGroup]int      `json:"by_group"`
	ByMetric     map[MetricType]int     `json:"by_metric"`
	ByLevel      map[AlertLevel]int     `json:"by_level"`
}

// ========== 告警对象 ==========

// CustomAlert 自定义告警
type CustomAlert struct {
	ID           string            `json:"id"`
	RuleID       string            `json:"rule_id"`
	RuleName     string            `json:"rule_name"`
	Group        RuleGroup         `json:"group"`
	Level        AlertLevel        `json:"level"`
	Metric       MetricType        `json:"metric"`
	CurrentValue float64           `json:"current_value"`
	Threshold    float64           `json:"threshold"`
	Operator     Operator          `json:"operator"`
	Target       string            `json:"target,omitempty"`
	Message      string            `json:"message"`
	Labels       map[string]string `json:"labels,omitempty"`
	Annotations  map[string]string `json:"annotations,omitempty"`
	Status       AlertStatus       `json:"status"`
	StartsAt     time.Time         `json:"starts_at"`
	EndsAt       *time.Time        `json:"ends_at,omitempty"`
	Duration     int               `json:"duration,omitempty"` // 告警持续时间（秒）
	Actions      []AlertAction     `json:"actions,omitempty"`

	// 告警处理状态
	Acknowledged   bool      `json:"acknowledged"`
	AcknowledgedBy string    `json:"acknowledged_by,omitempty"`
	AcknowledgedAt time.Time `json:"acknowledged_at,omitempty"`
	Silenced       bool      `json:"silenced"`
	SilenceID      string    `json:"silence_id,omitempty"`
}

// AlertStatus 告警状态
type AlertStatus string

const (
	AlertStatusPending AlertStatus = "pending" // 等待中（未达到持续时间）
	AlertStatusFiring  AlertStatus = "firing"  // 触发中
	AlertStatusResolved AlertStatus = "resolved" // 已恢复
	AlertStatusSilenced AlertStatus = "silenced" // 已静默
)

// ========== 辅助函数 ==========

func generateRuleID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("rule-%s", hex.EncodeToString(b))
}

func generateAlertID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return fmt.Sprintf("alert-%d-%s", time.Now().Unix(), hex.EncodeToString(b))
}

func contains(list []string, item string) bool {
	for _, s := range list {
		if s == item {
			return true
		}
	}
	return false
}