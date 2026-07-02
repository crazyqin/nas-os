// Package alerting 告警增强系统集成
// 提供模板、路由、聚合的统一入口
package alerting

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Manager 告警增强管理器 - 整合模板、路由、聚合.
type Manager struct {
	engine     *TemplateEngine
	router     *Router
	aggregator *Aggregator

	// 配置
	config *ManagerConfig

	// 回调
	onSend      func(channelID string, alert *AlertVars) error
	onAggregate func(aggregated []*AggregatedAlert)

	mu     sync.RWMutex
	stopCh chan struct{}
}

// ManagerConfig 管理器配置.
type ManagerConfig struct {
	EnableAggregation bool          `json:"enableAggregation"`
	AggregationWindow time.Duration `json:"aggregationWindow"`
	EnableRouting     bool          `json:"enableRouting"`
	EnableTemplates   bool          `json:"enableTemplates"`
}

// DefaultManagerConfig 默认配置.
func DefaultManagerConfig() ManagerConfig {
	return ManagerConfig{
		EnableAggregation: true,
		AggregationWindow: 5 * time.Minute,
		EnableRouting:     true,
		EnableTemplates:   true,
	}
}

// NewManager 创建增强告警管理器.
func NewManager(config ManagerConfig) *Manager {
	// 创建组件
	engine := NewTemplateEngine()

	aggregatorConfig := DefaultAggregationConfig()
	aggregatorConfig.Window = config.AggregationWindow

	m := &Manager{
		engine:     engine,
		router:     NewRouter(engine),
		aggregator: NewAggregator(aggregatorConfig),
		config:     &config,
		stopCh:     make(chan struct{}),
	}

	return m
}

// Start 启动管理器.
func (m *Manager) Start() error {
	if m.config.EnableAggregation {
		m.aggregator.SetFlushCallback(func(aggregated []*AggregatedAlert) {
			m.handleAggregatedAlerts(aggregated)
		})
		go m.aggregator.StartFlushLoop(m.stopCh)
	}
	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() {
	close(m.stopCh)
}

// GetEngine 获取模板引擎.
func (m *Manager) GetEngine() *TemplateEngine {
	return m.engine
}

// GetRouter 获取路由器.
func (m *Manager) GetRouter() *Router {
	return m.router
}

// GetAggregator 获取聚合器.
func (m *Manager) GetAggregator() *Aggregator {
	return m.aggregator
}

// ProcessAlert 处理单条告警.
func (m *Manager) ProcessAlert(ctx context.Context, vars *AlertVars) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 如果启用了聚合
	if m.config.EnableAggregation {
		m.aggregator.Add(
			vars.AlertID,
			vars.AlertName,
			vars.Message,
			vars.Source,
			vars.ServiceType,
			vars.Level,
			vars.Value,
			vars.Timestamp,
		)
		return nil
	}

	// 如果启用了路由
	if m.config.EnableRouting {
		_, err := m.router.Route(ctx, vars.Level, vars.AlertName, vars.ServiceType, vars.Tags, vars)
		return err
	}

	return nil
}

// ProcessAggregatedAlert 处理聚合告警.
func (m *Manager) ProcessAggregatedAlert(ctx context.Context, aggregated *AggregatedAlert) error {
	// 构建聚合告警的变量
	vars := &AlertVars{
		AlertID:     aggregated.Key,
		AlertName:   aggregated.AlertName,
		Level:       aggregated.Level,
		ServiceType: aggregated.ServiceType,
		Message:     aggregated.Summary,
		Timestamp:   aggregated.LastSeen,
		Tags:        make(map[string]string),
		Extra: map[string]interface{}{
			"count":     aggregated.Count,
			"firstSeen": aggregated.FirstSeen,
			"lastSeen":  aggregated.LastSeen,
			"children":  aggregated.Children,
		},
	}

	// 添加汇总信息
	vars.Extra["summary"] = aggregated.Summary

	// 路由到对应渠道
	if m.config.EnableRouting {
		_, err := m.router.Route(ctx, vars.Level, vars.AlertName, vars.ServiceType, vars.Tags, vars)
		return err
	}

	return nil
}

// handleAggregatedAlerts 处理聚合后的告警.
func (m *Manager) handleAggregatedAlerts(aggregated []*AggregatedAlert) {
	if m.onAggregate != nil {
		m.onAggregate(aggregated)
	}

	// 发送到回调
	for _, agg := range aggregated {
		// 构建告警变量并发送
		vars := &AlertVars{
			AlertID:     agg.Key,
			AlertName:   agg.AlertName,
			Level:       agg.Level,
			ServiceType: agg.ServiceType,
			Message:     agg.Summary,
			Timestamp:   agg.LastSeen,
			Tags:        make(map[string]string),
			Extra: map[string]interface{}{
				"count":     agg.Count,
				"firstSeen": agg.FirstSeen,
				"lastSeen":  agg.LastSeen,
			},
		}

		// 调用发送回调
		if m.onSend != nil {
			_ = m.onSend("aggregated", vars)
		}
	}
}

// SetOnSend 设置发送回调.
func (m *Manager) SetOnSend(fn func(channelID string, alert *AlertVars) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onSend = fn
}

// SetOnAggregate 设置聚合回调.
func (m *Manager) SetOnAggregate(fn func(aggregated []*AggregatedAlert)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.onAggregate = fn
}

// GetStatus 获取状态.
func (m *Manager) GetStatus() map[string]interface{} {
	return map[string]interface{}{
		"config":     m.config,
		"templates":  len(m.engine.ListTemplates()),
		"channels":   len(m.router.ListChannels()),
		"rules":      len(m.router.GetRules()),
		"pending":    m.aggregator.GetPending(),
		"suppressed": m.router.GetSuppressionCount(),
	}
}

// QuickSend 快速发送告警（直接渲染并发送）.
func (m *Manager) QuickSend(ctx context.Context, chType ChannelType, target, templateID string, vars *AlertVars) error {
	_ = &Channel{
		ID:       fmt.Sprintf("quick-%d", time.Now().UnixNano()),
		Type:     chType,
		Target:   target,
		Template: templateID,
		Enabled:  true,
	}

	switch chType {
	case ChannelEmail:
		subject, body, err := m.engine.Render(templateID, vars)
		if err != nil {
			return err
		}
		tmpl, _ := m.engine.GetTemplate(templateID)
		isHTML := tmpl != nil && tmpl.IsHTML
		return SendEmail(ctx, target, subject, body, isHTML)

	case ChannelTelegram, ChannelDingTalk, ChannelWeChat:
		_, body, err := m.engine.Render(templateID, vars)
		if err != nil {
			return err
		}
		payload := map[string]interface{}{
			"msgtype":  "markdown",
			"markdown": map[string]string{"content": body},
		}
		return SendWebhook(ctx, target, payload)

	default:
		payload, err := m.engine.RenderToJSON(templateID, vars)
		if err != nil {
			return err
		}
		return SendWebhook(ctx, target, payload)
	}
}
