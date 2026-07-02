package alertguided

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Aggregator 告警聚合器
// 将相同类型告警合并，避免告警风暴.
type Aggregator struct {
	groups   map[string]*AlertGroup // aggregationKey -> group
	mu       sync.RWMutex
	logger   *zap.Logger
	dedupTTL time.Duration // 去重窗口
}

// NewAggregator 创建聚合器.
func NewAggregator(logger *zap.Logger, dedupTTL time.Duration) *Aggregator {
	if logger == nil {
		logger = zap.NewNop()
	}
	if dedupTTL == 0 {
		dedupTTL = 5 * time.Minute
	}
	return &Aggregator{
		groups:   make(map[string]*AlertGroup),
		logger:   logger,
		dedupTTL: dedupTTL,
	}
}

// AlertGroup 告警分组.
type AlertGroup struct {
	Key         string    `json:"key"`
	Category    Category  `json:"category"`
	Severity    Severity  `json:"severity"`
	PrimaryID   string    `json:"primaryId"` // 主告警ID
	AlertIDs    []string  `json:"alertIds"`  // 所有关联告警ID
	Count       int       `json:"count"`
	FirstSeen   time.Time `json:"firstSeen"`
	LastSeen    time.Time `json:"lastSeen"`
	LastMessage string    `json:"lastMessage"`
	IsSilenced  bool      `json:"isSilenced"`
}

// Aggregate 聚合告警
// 返回: (group, isNew)
// 如果是同组已有未解决告警，追加到该组并返回 false
// 否则创建新组返回 true.
func (a *Aggregator) Aggregate(alert *GuidedAlert) (*AlertGroup, bool) {
	a.mu.Lock()
	defer a.mu.Unlock()

	key := alert.AggregationKey
	if key == "" {
		key = fmt.Sprintf("%s:%s", alert.Category, alert.Title)
	}

	now := time.Now()

	group, exists := a.groups[key]
	if exists && !a.isExpired(group) && group.PrimaryID != "" {
		// 追加到已有分组
		group.AlertIDs = append(group.AlertIDs, alert.ID)
		group.Count++
		group.LastSeen = now
		group.LastMessage = alert.Message
		a.logger.Info("alert aggregated into existing group",
			zap.String("key", key),
			zap.Int("count", group.Count),
		)
		return group, false
	}

	// 创建新分组
	group = &AlertGroup{
		Key:         key,
		Category:    alert.Category,
		Severity:    alert.Severity,
		PrimaryID:   alert.ID,
		AlertIDs:    []string{alert.ID},
		Count:       1,
		FirstSeen:   now,
		LastSeen:    now,
		LastMessage: alert.Message,
	}
	a.groups[key] = group
	a.logger.Info("new alert group created", zap.String("key", key))
	return group, true
}

// Resolve 分组解决，清除聚合.
func (a *Aggregator) Resolve(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	delete(a.groups, key)
	a.logger.Info("alert group resolved", zap.String("key", key))
}

// ResolveByAlertID 通过告警ID解决分组.
func (a *Aggregator) ResolveByAlertID(alertID string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	for key, group := range a.groups {
		for _, id := range group.AlertIDs {
			if id == alertID {
				delete(a.groups, key)
				a.logger.Info("alert group resolved by alert id",
					zap.String("key", key),
					zap.String("alertId", alertID),
				)
				return
			}
		}
	}
}

// GetGroup 获取分组.
func (a *Aggregator) GetGroup(key string) (*AlertGroup, bool) {
	a.mu.RLock()
	defer a.mu.RUnlock()
	g, ok := a.groups[key]
	return g, ok
}

// ListGroups 列出所有活跃分组.
func (a *Aggregator) ListGroups() []*AlertGroup {
	a.mu.RLock()
	defer a.mu.RUnlock()
	result := make([]*AlertGroup, 0, len(a.groups))
	for _, g := range a.groups {
		if !a.isExpired(g) {
			result = append(result, g)
		}
	}
	return result
}

// SilenceGroup 静音分组.
func (a *Aggregator) SilenceGroup(key string) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if g, ok := a.groups[key]; ok {
		g.IsSilenced = true
	}
}

// Cleanup 清理过期分组.
func (a *Aggregator) Cleanup() int {
	a.mu.Lock()
	defer a.mu.Unlock()
	count := 0
	for key, g := range a.groups {
		if a.isExpired(g) {
			delete(a.groups, key)
			count++
		}
	}
	if count > 0 {
		a.logger.Info("cleaned up expired groups", zap.Int("count", count))
	}
	return count
}

// Summary 聚合汇总.
func (a *Aggregator) Summary() *AggregationSummary {
	a.mu.RLock()
	defer a.mu.RUnlock()
	summary := &AggregationSummary{
		Groups:       make(map[string]int),
		ByCategory:   make(map[Category]int),
		TotalDeduped: 0,
	}
	for _, g := range a.groups {
		if a.isExpired(g) {
			continue
		}
		summary.ActiveGroups++
		summary.TotalAlerts += g.Count
		summary.ByCategory[g.Category] += g.Count
		summary.Groups[g.Key] = g.Count
		if g.Count > 1 {
			summary.TotalDeduped += g.Count - 1
		}
	}
	return summary
}

func (a *Aggregator) isExpired(g *AlertGroup) bool {
	return time.Since(g.LastSeen) > a.dedupTTL
}

// AggregationSummary 聚合汇总.
type AggregationSummary struct {
	ActiveGroups int              `json:"activeGroups"`
	TotalAlerts  int              `json:"totalAlerts"`
	TotalDeduped int              `json:"totalDeduped"`
	ByCategory   map[Category]int `json:"byCategory"`
	Groups       map[string]int   `json:"groups"`
}
