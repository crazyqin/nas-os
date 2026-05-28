// Package personalportal 提供个人门户仪表盘功能。
// feed.go 实现信息流聚合功能，包括 RSS、邮件、通知、日程统一视图。
package personalportal

import (
	"sort"
	"time"
)

// FeedAggregator 信息流聚合器。
type FeedAggregator struct {
	engine *PortalEngine
}

// NewFeedAggregator 创建信息流聚合器。
func NewFeedAggregator(engine *PortalEngine) *FeedAggregator {
	return &FeedAggregator{engine: engine}
}

// AddFeedConfig 添加信息流配置。
func (fa *FeedAggregator) AddFeedConfig(userID, name string, source FeedSource, url string, refreshRate, maxItems int) (*FeedConfig, error) {
	fa.engine.mu.Lock()
	defer fa.engine.mu.Unlock()

	now := time.Now()
	config := &FeedConfig{
		ID:          generateID(),
		UserID:      userID,
		Name:        name,
		Source:      source,
		URL:         url,
		RefreshRate: refreshRate,
		Enabled:     true,
		MaxItems:    maxItems,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	fa.engine.feedConfigs[config.ID] = config
	return config, nil
}

// GetFeedConfig 获取信息流配置。
func (fa *FeedAggregator) GetFeedConfig(id string) (*FeedConfig, error) {
	fa.engine.mu.RLock()
	defer fa.engine.mu.RUnlock()

	config, exists := fa.engine.feedConfigs[id]
	if !exists {
		return nil, ErrFeedConfigNotFound
	}
	return config, nil
}

// ListFeedConfigs 列出用户的信息流配置。
func (fa *FeedAggregator) ListFeedConfigs(userID string) []*FeedConfig {
	fa.engine.mu.RLock()
	defer fa.engine.mu.RUnlock()

	result := make([]*FeedConfig, 0)
	for _, config := range fa.engine.feedConfigs {
		if config.UserID == userID {
			result = append(result, config)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// UpdateFeedConfig 更新信息流配置。
func (fa *FeedAggregator) UpdateFeedConfig(id string, updates map[string]interface{}) (*FeedConfig, error) {
	fa.engine.mu.Lock()
	defer fa.engine.mu.Unlock()

	config, exists := fa.engine.feedConfigs[id]
	if !exists {
		return nil, ErrFeedConfigNotFound
	}

	if name, ok := updates["name"].(string); ok {
		config.Name = name
	}
	if url, ok := updates["url"].(string); ok {
		config.URL = url
	}
	if refreshRate, ok := updates["refresh_rate"].(int); ok {
		config.RefreshRate = refreshRate
	}
	if enabled, ok := updates["enabled"].(bool); ok {
		config.Enabled = enabled
	}
	if maxItems, ok := updates["max_items"].(int); ok {
		config.MaxItems = maxItems
	}

	config.UpdatedAt = time.Now()
	return config, nil
}

// DeleteFeedConfig 删除信息流配置。
func (fa *FeedAggregator) DeleteFeedConfig(id string) error {
	fa.engine.mu.Lock()
	defer fa.engine.mu.Unlock()

	if _, exists := fa.engine.feedConfigs[id]; !exists {
		return ErrFeedConfigNotFound
	}

	delete(fa.engine.feedConfigs, id)
	return nil
}

// AddFeedItem 添加信息流项目。
func (fa *FeedAggregator) AddFeedItem(userID string, item *FeedItem) {
	fa.engine.mu.Lock()
	defer fa.engine.mu.Unlock()

	item.ID = generateID()
	item.Timestamp = time.Now()
	item.Read = false

	fa.engine.feedItems[userID] = append(fa.engine.feedItems[userID], item)
}

// GetAggregatedFeed 获取聚合信息流。
func (fa *FeedAggregator) GetAggregatedFeed(userID string, limit int) []*FeedItem {
	fa.engine.mu.RLock()
	defer fa.engine.mu.RUnlock()

	allItems := make([]*FeedItem, 0)
	for _, items := range fa.engine.feedItems {
		for _, item := range items {
			allItems = append(allItems, item)
		}
	}

	// 按时间倒序排序
	sort.Slice(allItems, func(i, j int) bool {
		return allItems[i].Timestamp.After(allItems[j].Timestamp)
	})

	// 限制数量
	if limit > 0 && limit < len(allItems) {
		allItems = allItems[:limit]
	}

	return allItems
}

// GetFeedBySource 按来源获取信息流。
func (fa *FeedAggregator) GetFeedBySource(userID string, source FeedSource, limit int) []*FeedItem {
	fa.engine.mu.RLock()
	defer fa.engine.mu.RUnlock()

	result := make([]*FeedItem, 0)
	for _, items := range fa.engine.feedItems {
		for _, item := range items {
			if item.Source == source {
				result = append(result, item)
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}

	return result
}

// MarkFeedItemRead 标记信息流项目已读。
func (fa *FeedAggregator) MarkFeedItemRead(userID, itemID string) error {
	fa.engine.mu.Lock()
	defer fa.engine.mu.Unlock()

	for _, items := range fa.engine.feedItems {
		for _, item := range items {
			if item.ID == itemID {
				item.Read = true
				return nil
			}
		}
	}

	return nil
}

// MarkAllFeedItemsRead 标记所有信息流项目已读。
func (fa *FeedAggregator) MarkAllFeedItemsRead(userID string, source *FeedSource) {
	fa.engine.mu.Lock()
	defer fa.engine.mu.Unlock()

	for _, items := range fa.engine.feedItems {
		for _, item := range items {
			if source == nil || item.Source == *source {
				item.Read = true
			}
		}
	}
}

// GetUnreadFeedCount 获取未读信息流数量。
func (fa *FeedAggregator) GetUnreadFeedCount(userID string) map[FeedSource]int {
	fa.engine.mu.RLock()
	defer fa.engine.mu.RUnlock()

	counts := make(map[FeedSource]int)
	for _, items := range fa.engine.feedItems {
		for _, item := range items {
			if !item.Read {
				counts[item.Source]++
			}
		}
	}

	return counts
}

// SearchFeedItems 搜索信息流项目。
func (fa *FeedAggregator) SearchFeedItems(userID, query string, limit int) []*FeedItem {
	fa.engine.mu.RLock()
	defer fa.engine.mu.RUnlock()

	result := make([]*FeedItem, 0)

	for _, items := range fa.engine.feedItems {
		for _, item := range items {
			if containsIgnoreCase(item.Title, query) || containsIgnoreCase(item.Summary, query) {
				result = append(result, item)
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	if limit > 0 && limit < len(result) {
		result = result[:limit]
	}

	return result
}

// DeleteFeedItem 删除信息流项目。
func (fa *FeedAggregator) DeleteFeedItem(userID, itemID string) {
	fa.engine.mu.Lock()
	defer fa.engine.mu.Unlock()

	items := fa.engine.feedItems[userID]
	for i, item := range items {
		if item.ID == itemID {
			fa.engine.feedItems[userID] = append(items[:i], items[i+1:]...)
			return
		}
	}
}

// CleanupOldFeedItems 清理旧的信息流项目。
func (fa *FeedAggregator) CleanupOldFeedItems(userID string, maxAge time.Duration) int {
	fa.engine.mu.Lock()
	defer fa.engine.mu.Unlock()

	cutoff := time.Now().Add(-maxAge)
	removed := 0

	items := fa.engine.feedItems[userID]
	newItems := make([]*FeedItem, 0, len(items))

	for _, item := range items {
		if item.Timestamp.After(cutoff) {
			newItems = append(newItems, item)
		} else {
			removed++
		}
	}

	fa.engine.feedItems[userID] = newItems
	return removed
}

// GetFeedStats 获取信息流统计。
func (fa *FeedAggregator) GetFeedStats(userID string) map[string]interface{} {
	fa.engine.mu.RLock()
	defer fa.engine.mu.RUnlock()

	total := 0
	unread := 0
	bySource := make(map[FeedSource]int)

	for _, items := range fa.engine.feedItems {
		for _, item := range items {
			total++
			if !item.Read {
				unread++
			}
			bySource[item.Source]++
		}
	}

	return map[string]interface{}{
		"total":     total,
		"unread":    unread,
		"read":      total - unread,
		"by_source": bySource,
	}
}

// AddMockFeedData 添加模拟信息流数据。
func (fa *FeedAggregator) AddMockFeedData(userID string) {
	now := time.Now()

	// RSS 订阅
	fa.AddFeedItem(userID, &FeedItem{
		Source:    FeedSourceRSS,
		Title:     "Go 1.22 发布：新特性一览",
		Summary:   "Go 1.22 带来了多项性能改进和新特性...",
		Link:      "https://example.com/go122",
		Timestamp: now.Add(-1 * time.Hour),
		Category:  "技术",
	})

	fa.AddFeedItem(userID, &FeedItem{
		Source:    FeedSourceRSS,
		Title:     "Kubernetes 最佳实践",
		Summary:   "本文介绍了 K8s 集群管理的最佳实践...",
		Link:      "https://example.com/k8s",
		Timestamp: now.Add(-3 * time.Hour),
		Category:  "技术",
	})

	// 邮件
	fa.AddFeedItem(userID, &FeedItem{
		Source:    FeedSourceEmail,
		Title:     "项目周报 - 2024年第5周",
		Summary:   "本周完成了用户认证模块的开发...",
		Timestamp: now.Add(-2 * time.Hour),
		Category:  "工作",
	})

	// 日程
	fa.AddFeedItem(userID, &FeedItem{
		Source:    FeedSourceCalendar,
		Title:     "明天 10:00 - 团队周会",
		Summary:   "讨论本周进度和下周计划",
		Timestamp: now.Add(-30 * time.Minute),
		Category:  "日程",
	})

	// 通知
	fa.AddFeedItem(userID, &FeedItem{
		Source:    FeedSourceNotification,
		Title:     "系统更新完成",
		Summary:   "NAS 系统已更新到最新版本",
		Timestamp: now.Add(-5 * time.Hour),
		Category:  "系统",
	})
}

// containsIgnoreCase 不区分大小写检查包含。
func containsIgnoreCase(s, substr string) bool {
	if len(substr) == 0 {
		return true
	}
	if len(s) == 0 {
		return false
	}

	sLower := toLower(s)
	substrLower := toLower(substr)

	for i := 0; i <= len(sLower)-len(substrLower); i++ {
		if sLower[i:i+len(substrLower)] == substrLower {
			return true
		}
	}

	return false
}

// toLower 简单的转小写。
func toLower(s string) string {
	result := make([]byte, len(s))
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		result[i] = c
	}
	return string(result)
}
