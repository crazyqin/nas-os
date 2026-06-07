package alerting

import (
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// AggregatedAlert 聚合后的告警
type AggregatedAlert struct {
	Key         string      `json:"key"`
	AlertName   string      `json:"alertName"`
	Level       AlertLevel  `json:"level"`
	ServiceType string      `json:"serviceType"`
	Count       int         `json:"count"`
	FirstSeen   time.Time   `json:"firstSeen"`
	LastSeen    time.Time   `json:"lastSeen"`
	Children    []AlertItem `json:"children"`
	Summary     string      `json:"summary"`
}

// AlertItem 单条告警项
type AlertItem struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Message   string    `json:"message"`
	Source    string    `json:"source"`
	Value     float64   `json:"value"`
	Timestamp time.Time `json:"timestamp"`
}

// AggregationConfig 聚合配置
type AggregationConfig struct {
	Window        time.Duration `json:"window"`        // 聚合窗口，默认5分钟
	GroupBy       []string      `json:"groupBy"`       // 聚合维度：level, serviceType, name
	MaxGroupSize  int           `json:"maxGroupSize"`  // 每组最大数量，超过则裁剪
	FlushInterval time.Duration `json:"flushInterval"` // 刷新间隔
}

// DefaultAggregationConfig 默认聚合配置
func DefaultAggregationConfig() AggregationConfig {
	return AggregationConfig{
		Window:        5 * time.Minute,
		GroupBy:       []string{"level", "serviceType"},
		MaxGroupSize:  100,
		FlushInterval: 30 * time.Second,
	}
}

// aggregationGroup 聚合组
type aggregationGroup struct {
	key         string
	alertName   string
	level       AlertLevel
	serviceType string
	items       []AlertItem
	firstSeen   time.Time
	lastSeen    time.Time
}

// Aggregator 告警聚合器
type Aggregator struct {
	mu     sync.RWMutex
	config AggregationConfig
	groups map[string]*aggregationGroup
	alerts []AlertItem // 未聚合的原始告警

	// 回调函数
	onFlush func(aggregated []*AggregatedAlert)
}

// NewAggregator 创建告警聚合器
func NewAggregator(config AggregationConfig) *Aggregator {
	if config.Window <= 0 {
		config.Window = 5 * time.Minute
	}
	if config.MaxGroupSize <= 0 {
		config.MaxGroupSize = 100
	}
	if config.FlushInterval <= 0 {
		config.FlushInterval = 30 * time.Second
	}

	return &Aggregator{
		config: config,
		groups: make(map[string]*aggregationGroup),
		alerts: make([]AlertItem, 0),
	}
}

// SetFlushCallback 设置刷新回调
func (a *Aggregator) SetFlushCallback(fn func(aggregated []*AggregatedAlert)) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.onFlush = fn
}

// Add 添加告警到聚合器
func (a *Aggregator) Add(id, name, message, source, serviceType string, level AlertLevel, value float64, timestamp time.Time) {
	a.mu.Lock()
	defer a.mu.Unlock()

	item := AlertItem{
		ID:        id,
		Name:      name,
		Message:   message,
		Source:    source,
		Value:     value,
		Timestamp: timestamp,
	}

	// 保存原始告警
	a.alerts = append(a.alerts, item)

	// 计算聚合key
	key := a.computeGroupKey(name, level, serviceType)

	group, exists := a.groups[key]
	if !exists {
		group = &aggregationGroup{
			key:         key,
			alertName:   name,
			level:       level,
			serviceType: serviceType,
			items:       make([]AlertItem, 0),
			firstSeen:   timestamp,
			lastSeen:    timestamp,
		}
		a.groups[key] = group
	}

	group.items = append(group.items, item)
	group.lastSeen = timestamp

	// 限制组大小
	if len(group.items) > a.config.MaxGroupSize {
		group.items = group.items[len(group.items)-a.config.MaxGroupSize:]
	}
}

// Flush 刷新并返回聚合结果
func (a *Aggregator) Flush() []*AggregatedAlert {
	a.mu.Lock()
	defer a.mu.Unlock()

	result := make([]*AggregatedAlert, 0, len(a.groups))

	for key, group := range a.groups {
		// 检查窗口是否到期
		if time.Since(group.firstSeen) < a.config.Window && time.Since(group.lastSeen) < a.config.Window {
			// 还没到窗口时间，但如果是强制刷新也可以返回
			continue
		}

		aggregated := a.buildAggregatedAlert(group)
		result = append(result, aggregated)

		// 清理已刷新的组
		delete(a.groups, key)
	}

	// 清理过期的原始告警
	a.cleanExpiredAlerts()

	return result
}

// FlushAll 强制刷新所有聚合组
func (a *Aggregator) FlushAll() []*AggregatedAlert {
	a.mu.Lock()
	defer a.mu.Unlock()

	result := make([]*AggregatedAlert, 0, len(a.groups))

	for key, group := range a.groups {
		aggregated := a.buildAggregatedAlert(group)
		result = append(result, aggregated)
		delete(a.groups, key)
	}

	a.alerts = make([]AlertItem, 0)
	return result
}

// GetPending 获取待刷新的聚合组数量
func (a *Aggregator) GetPending() int {
	a.mu.RLock()
	defer a.mu.RUnlock()
	return len(a.groups)
}

// GetPendingDetails 获取待刷新的聚合组详情
func (a *Aggregator) GetPendingDetails() []*AggregatedAlert {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]*AggregatedAlert, 0, len(a.groups))
	for _, group := range a.groups {
		aggregated := a.buildAggregatedAlert(group)
		result = append(result, aggregated)
	}
	return result
}

// GetStats 获取聚合器统计
func (a *Aggregator) GetStats() map[string]interface{} {
	a.mu.RLock()
	defer a.mu.RUnlock()

	totalItems := 0
	for _, group := range a.groups {
		totalItems += len(group.items)
	}

	return map[string]interface{}{
		"groupCount":    len(a.groups),
		"totalItems":    totalItems,
		"rawAlertCount": len(a.alerts),
		"window":        a.config.Window.String(),
		"flushInterval": a.config.FlushInterval.String(),
	}
}

// buildAggregatedAlert 构建聚合告警
func (a *Aggregator) buildAggregatedAlert(group *aggregationGroup) *AggregatedAlert {
	children := make([]AlertItem, len(group.items))
	copy(children, group.items)

	// 按时间排序
	sort.Slice(children, func(i, j int) bool {
		return children[i].Timestamp.Before(children[j].Timestamp)
	})

	summary := a.generateSummary(group, children)

	return &AggregatedAlert{
		Key:         group.key,
		AlertName:   group.alertName,
		Level:       group.level,
		ServiceType: group.serviceType,
		Count:       len(children),
		FirstSeen:   group.firstSeen,
		LastSeen:    group.lastSeen,
		Children:    children,
		Summary:     summary,
	}
}

// generateSummary 生成告警汇总
func (a *Aggregator) generateSummary(group *aggregationGroup, children []AlertItem) string {
	switch {
	case len(children) == 1:
		item := children[0]
		return fmt.Sprintf("%s: %s (来源: %s)", group.alertName, item.Message, item.Source)

	case len(children) <= 3:
		var parts []string
		for _, item := range children {
			parts = append(parts, fmt.Sprintf("%s (来源: %s)", item.Message, item.Source))
		}
		return fmt.Sprintf("%s [%d条]: %s", group.alertName, len(children), strings.Join(parts, "; "))

	default:
		// 超过3条时，显示汇总
		sources := make(map[string]int)
		for _, item := range children {
			sources[item.Source]++
		}

		var sourceParts []string
		for src, count := range sources {
			sourceParts = append(sourceParts, fmt.Sprintf("%s×%d", src, count))
		}

		return fmt.Sprintf("%s [%d条告警, 来自: %s, 时间: %s ~ %s]",
			group.alertName,
			len(children),
			strings.Join(sourceParts, ", "),
			children[0].Timestamp.Format("15:04:05"),
			children[len(children)-1].Timestamp.Format("15:04:05"),
		)
	}
}

// computeGroupKey 计算聚合组key
func (a *Aggregator) computeGroupKey(name string, level AlertLevel, serviceType string) string {
	var parts []string

	for _, dim := range a.config.GroupBy {
		switch dim {
		case "level":
			parts = append(parts, string(level))
		case "serviceType":
			parts = append(parts, serviceType)
		case "name":
			parts = append(parts, name)
		}
	}

	if len(parts) == 0 {
		// 默认按 name+level 聚合
		return fmt.Sprintf("%s|%s", name, level)
	}

	return strings.Join(parts, "|")
}

// cleanExpiredAlerts 清理过期的原始告警
func (a *Aggregator) cleanExpiredAlerts() {
	window := a.config.Window
	cutoff := time.Now().Add(-window)

	newAlerts := make([]AlertItem, 0, len(a.alerts))
	for _, alert := range a.alerts {
		if alert.Timestamp.After(cutoff) {
			newAlerts = append(newAlerts, alert)
		}
	}
	a.alerts = newAlerts
}

// StartFlushLoop 启动定时刷新循环
func (a *Aggregator) StartFlushLoop(stopCh <-chan struct{}) {
	ticker := time.NewTicker(a.config.FlushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			aggregated := a.Flush()
			if len(aggregated) > 0 && a.onFlush != nil {
				a.onFlush(aggregated)
			}
		case <-stopCh:
			// 停止前刷新剩余
			aggregated := a.FlushAll()
			if len(aggregated) > 0 && a.onFlush != nil {
				a.onFlush(aggregated)
			}
			return
		}
	}
}
