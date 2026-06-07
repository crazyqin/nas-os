// Package notifyrouter 提供智能通知路由核心管理逻辑
package notifyrouter

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 通知路由管理器
type Manager struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	rules         map[string]*NotifyRule
	channels      map[Channel]*ChannelConfig
	preferences   map[string]*UserPreference
	deliveries    map[string]*[]*Delivery // notifyID -> deliveries
	notifyIndex   map[string]*Notification
	channelStats  map[Channel]*ChannelStats
	throttleCount map[string]*throttleCounter
}

// throttleCounter 限流计数器
type throttleCounter struct {
	MinuteCount int
	HourCount   int
	DayCount    int
	LastReset   time.Time
}

// NewManager 创建通知路由管理器
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		logger:        logger,
		rules:         make(map[string]*NotifyRule),
		channels:      make(map[Channel]*ChannelConfig),
		preferences:   make(map[string]*UserPreference),
		deliveries:    make(map[string]*[]*Delivery),
		notifyIndex:   make(map[string]*Notification),
		channelStats:  make(map[Channel]*ChannelStats),
		throttleCount: make(map[string]*throttleCounter),
	}

	// 初始化默认渠道配置
	m.initDefaultChannels()

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// initDefaultChannels 初始化默认渠道配置
func (m *Manager) initDefaultChannels() {
	defaultChannels := []*ChannelConfig{
		{Channel: ChannelEmail, IsEnabled: true, MaxRetries: 3, TimeoutSecs: 30},
		{Channel: ChannelSMS, IsEnabled: true, MaxRetries: 2, TimeoutSecs: 15},
		{Channel: ChannelPush, IsEnabled: true, MaxRetries: 3, TimeoutSecs: 10},
		{Channel: ChannelSlack, IsEnabled: true, MaxRetries: 3, TimeoutSecs: 15},
		{Channel: ChannelWeChat, IsEnabled: true, MaxRetries: 3, TimeoutSecs: 15},
		{Channel: ChannelDingTalk, IsEnabled: true, MaxRetries: 3, TimeoutSecs: 15},
		{Channel: ChannelWebhook, IsEnabled: true, MaxRetries: 3, TimeoutSecs: 30},
		{Channel: ChannelVoice, IsEnabled: false, MaxRetries: 1, TimeoutSecs: 60},
	}

	for _, ch := range defaultChannels {
		m.channels[ch.Channel] = ch
		m.channelStats[ch.Channel] = &ChannelStats{
			Channel: ch.Channel,
		}
	}
}

// RouteNotification 路由通知到最佳渠道
func (m *Manager) RouteNotification(req *RouteNotificationRequest) (*RouteResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	notify := req.Notification
	if notify.ID == "" {
		notify.ID = generateID()
	}
	if notify.CreatedAt.IsZero() {
		notify.CreatedAt = time.Now()
	}

	// 存储通知
	m.notifyIndex[notify.ID] = notify

	// 如果指定了强制渠道
	if req.ForceChannel != nil {
		return m.forceRoute(notify, *req.ForceChannel)
	}

	// 检查用户偏好
	preferred := m.getUserPreferredChannels(notify.UserID)

	// 匹配规则
	selectedChannels := m.matchRules(notify, preferred)

	if len(selectedChannels) == 0 {
		// 使用默认渠道
		selectedChannels = []Channel{ChannelEmail, ChannelPush}
	}

	// 检查限流
	filteredChannels := m.filterByThrottle(notify.UserID, selectedChannels)
	if len(filteredChannels) == 0 {
		filteredChannels = selectedChannels[:1] // 至少保留一个渠道
	}

	// 选择最佳渠道
	bestChannel := m.selectBestChannel(filteredChannels)

	// 创建投递记录
	deliveries := make([]*Delivery, 0)
	for _, ch := range filteredChannels {
		status := StatusQueued
		if ch != bestChannel {
			status = StatusPending
		}

		delivery := &Delivery{
			ID:        generateID(),
			NotifyID:  notify.ID,
			Channel:   ch,
			UserID:    notify.UserID,
			Status:    status,
			CreatedAt: time.Now(),
			UpdatedAt: time.Now(),
		}
		deliveries = append(deliveries, delivery)
	}

	m.deliveries[notify.ID] = &deliveries

	result := &RouteResult{
		NotifyID:        notify.ID,
		SelectedChannel: bestChannel,
		AllChannels:     filteredChannels,
		Deliveries:      deliveries,
		Reason:          fmt.Sprintf("selected based on user preference and rule matching"),
		RoutedAt:        time.Now(),
	}

	m.logger.Info("notification routed",
		zap.String("notify_id", notify.ID),
		zap.String("channel", string(bestChannel)),
		zap.String("priority", string(notify.Priority)))

	return result, nil
}

// forceRoute 强制路由到指定渠道
func (m *Manager) forceRoute(notify *Notification, channel Channel) (*RouteResult, error) {
	if _, ok := m.channels[channel]; !ok {
		return nil, fmt.Errorf("channel not configured: %s", channel)
	}

	delivery := &Delivery{
		ID:        generateID(),
		NotifyID:  notify.ID,
		Channel:   channel,
		UserID:    notify.UserID,
		Status:    StatusQueued,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.deliveries[notify.ID] = &[]*Delivery{delivery}

	return &RouteResult{
		NotifyID:        notify.ID,
		SelectedChannel: channel,
		AllChannels:     []Channel{channel},
		Deliveries:      []*Delivery{delivery},
		Reason:          "forced channel selection",
		RoutedAt:        time.Now(),
	}, nil
}

// getUserPreferredChannels 获取用户偏好渠道
func (m *Manager) getUserPreferredChannels(userID string) []Channel {
	pref, ok := m.preferences[userID]
	if !ok || len(pref.PreferredChannels) == 0 {
		return nil
	}

	// 检查是否在免打扰时间
	if pref.QuietHours != nil && m.isInQuietHours(pref.QuietHours) {
		// 在免打扰时间内，只使用非打扰渠道
		quiet := make([]Channel, 0)
		for _, ch := range pref.PreferredChannels {
			if ch == ChannelPush || ch == ChannelEmail {
				quiet = append(quiet, ch)
			}
		}
		return quiet
	}

	return pref.PreferredChannels
}

// isInQuietHours 检查是否在免打扰时间
func (m *Manager) isInQuietHours(quietHours *TimeWindow) bool {
	now := time.Now()
	hour := now.Hour()
	day := int(now.Weekday())

	if len(quietHours.DaysOfWeek) > 0 {
		found := false
		for _, d := range quietHours.DaysOfWeek {
			if d == day {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if quietHours.StartHour <= quietHours.EndHour {
		return hour >= quietHours.StartHour && hour < quietHours.EndHour
	}
	// 跨午夜的情况
	return hour >= quietHours.StartHour || hour < quietHours.EndHour
}

// matchRules 匹配路由规则
func (m *Manager) matchRules(notify *Notification, preferred []Channel) []Channel {
	// 按优先级顺序排序规则
	orderedRules := make([]*NotifyRule, 0)
	for _, rule := range m.rules {
		if rule.IsActive {
			orderedRules = append(orderedRules, rule)
		}
	}

	for _, rule := range orderedRules {
		if m.matchesCondition(notify, rule) {
			channels := rule.Channels

			// 与用户偏好取交集
			if len(preferred) > 0 {
				intersected := m.intersectChannels(channels, preferred)
				if len(intersected) > 0 {
					return intersected
				}
			}

			return channels
		}
	}

	return preferred
}

// matchesCondition 检查通知是否匹配规则条件
func (m *Manager) matchesCondition(notify *Notification, rule *NotifyRule) bool {
	if rule.Conditions == nil {
		return true
	}

	cond := rule.Conditions

	// 检查优先级
	if len(cond.Priorities) > 0 {
		found := false
		for _, p := range cond.Priorities {
			if p == notify.Priority {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查用户
	if len(cond.UserIDs) > 0 {
		found := false
		for _, uid := range cond.UserIDs {
			if uid == notify.UserID {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查标签
	if len(cond.Tags) > 0 {
		hasTag := false
		for _, tag := range cond.Tags {
			for _, notifyTag := range notify.Tags {
				if tag == notifyTag {
					hasTag = true
					break
				}
			}
			if hasTag {
				break
			}
		}
		if !hasTag {
			return false
		}
	}

	// 检查关键词
	if len(cond.Keywords) > 0 {
		hasKeyword := false
		for _, kw := range cond.Keywords {
			if strings.Contains(notify.Title, kw) || strings.Contains(notify.Body, kw) {
				hasKeyword = true
				break
			}
		}
		if !hasKeyword {
			return false
		}
	}

	// 检查时间窗口
	if cond.TimeWindow != nil {
		if !m.isInTimeWindow(cond.TimeWindow) {
			return false
		}
	}

	return true
}

// isInTimeWindow 检查是否在时间窗口内
func (m *Manager) isInTimeWindow(tw *TimeWindow) bool {
	now := time.Now()
	hour := now.Hour()
	day := int(now.Weekday())

	if len(tw.DaysOfWeek) > 0 {
		found := false
		for _, d := range tw.DaysOfWeek {
			if d == day {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if tw.StartHour <= tw.EndHour {
		return hour >= tw.StartHour && hour < tw.EndHour
	}
	return hour >= tw.StartHour || hour < tw.EndHour
}

// intersectChannels 取两个渠道列表的交集
func (m *Manager) intersectChannels(a, b []Channel) []Channel {
	set := make(map[Channel]bool)
	for _, ch := range a {
		set[ch] = true
	}

	result := make([]Channel, 0)
	for _, ch := range b {
		if set[ch] {
			result = append(result, ch)
		}
	}
	return result
}

// filterByThrottle 按限流过滤渠道
func (m *Manager) filterByThrottle(userID string, channels []Channel) []Channel {
	filtered := make([]Channel, 0)

	for _, ch := range channels {
		key := fmt.Sprintf("%s:%s", userID, ch)
		counter, ok := m.throttleCount[key]
		if !ok {
			filtered = append(filtered, ch)
			continue
		}

		// 重置过期计数
		if time.Since(counter.LastReset) > 24*time.Hour {
			counter.DayCount = 0
			counter.HourCount = 0
			counter.MinuteCount = 0
			counter.LastReset = time.Now()
		}

		// 检查限流
		config := m.channels[ch]
		if config == nil {
			filtered = append(filtered, ch)
			continue
		}

		rule := m.findThrottleRule(userID)
		if rule != nil && rule.Throttle != nil {
			if rule.Throttle.MaxPerMinute > 0 && counter.MinuteCount >= rule.Throttle.MaxPerMinute {
				continue
			}
			if rule.Throttle.MaxPerHour > 0 && counter.HourCount >= rule.Throttle.MaxPerHour {
				continue
			}
			if rule.Throttle.MaxPerDay > 0 && counter.DayCount >= rule.Throttle.MaxPerDay {
				continue
			}
		}

		filtered = append(filtered, ch)
	}

	return filtered
}

// findThrottleRule 查找限流规则
func (m *Manager) findThrottleRule(userID string) *NotifyRule {
	for _, rule := range m.rules {
		if rule.IsActive && rule.Throttle != nil {
			if rule.Conditions == nil || len(rule.Conditions.UserIDs) == 0 {
				return rule
			}
			for _, uid := range rule.Conditions.UserIDs {
				if uid == userID {
					return rule
				}
			}
		}
	}
	return nil
}

// selectBestChannel 选择最佳渠道
func (m *Manager) selectBestChannel(channels []Channel) Channel {
	if len(channels) == 0 {
		return ChannelEmail
	}

	// 按投递率排序
	best := channels[0]
	bestRate := float64(-1)

	for _, ch := range channels {
		stats, ok := m.channelStats[ch]
		if !ok || stats.TotalSent == 0 {
			// 新渠道，给予机会
			return ch
		}

		if stats.DeliveryRate > bestRate {
			bestRate = stats.DeliveryRate
			best = ch
		}
	}

	return best
}

// SetRules 设置路由规则
func (m *Manager) SetRules(req *SetRuleRequest) (*NotifyRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule := &NotifyRule{
		ID:          generateID(),
		Name:        req.Name,
		Description: req.Description,
		Type:        req.Type,
		IsActive:    true,
		Priority:    req.Priority,
		Channels:    req.Channels,
		Conditions:  req.Conditions,
		Fallback:    req.Fallback,
		Throttle:    req.Throttle,
		Priority_:   req.Priority_,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.rules[rule.ID] = rule

	m.logger.Info("rule created",
		zap.String("rule_id", rule.ID),
		zap.String("name", rule.Name))

	return rule, nil
}

// GetRules 获取所有规则
func (m *Manager) GetRules() []*NotifyRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*NotifyRule, 0, len(m.rules))
	for _, r := range m.rules {
		rules = append(rules, r)
	}
	return rules
}

// GetRule 获取规则
func (m *Manager) GetRule(id string) (*NotifyRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, ok := m.rules[id]
	if !ok {
		return nil, fmt.Errorf("rule not found: %s", id)
	}
	return rule, nil
}

// UpdateRule 更新规则
func (m *Manager) UpdateRule(id string, req *SetRuleRequest) (*NotifyRule, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	rule, ok := m.rules[id]
	if !ok {
		return nil, fmt.Errorf("rule not found: %s", id)
	}

	rule.Name = req.Name
	rule.Description = req.Description
	rule.Type = req.Type
	rule.Priority = req.Priority
	rule.Channels = req.Channels
	rule.Conditions = req.Conditions
	rule.Fallback = req.Fallback
	rule.Throttle = req.Throttle
	rule.Priority_ = req.Priority_
	rule.UpdatedAt = time.Now()

	return rule, nil
}

// DeleteRule 删除规则
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.rules[id]; !ok {
		return fmt.Errorf("rule not found: %s", id)
	}

	delete(m.rules, id)
	return nil
}

// GetDeliveryStatus 获取投递状态
func (m *Manager) GetDeliveryStatus(notifyID, deliveryID string) ([]*Delivery, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if deliveryID != "" {
		// 查找特定投递
		for _, deliveries := range m.deliveries {
			for _, d := range *deliveries {
				if d.ID == deliveryID {
					return []*Delivery{d}, nil
				}
			}
		}
		return nil, fmt.Errorf("delivery not found: %s", deliveryID)
	}

	// 查找通知的所有投递
	if notifyID != "" {
		deliveries, ok := m.deliveries[notifyID]
		if !ok {
			return nil, fmt.Errorf("notification not found: %s", notifyID)
		}
		return *deliveries, nil
	}

	// 返回所有投递
	all := make([]*Delivery, 0)
	for _, deliveries := range m.deliveries {
		all = append(all, *deliveries...)
	}
	return all, nil
}

// UpdateDeliveryStatus 更新投递状态
func (m *Manager) UpdateDeliveryStatus(deliveryID string, status DeliveryStatus, errMsg string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, deliveries := range m.deliveries {
		for _, d := range *deliveries {
			if d.ID == deliveryID {
				d.Status = status
				d.UpdatedAt = time.Now()

				if errMsg != "" {
					d.LastError = errMsg
				}

				if status == StatusSent {
					now := time.Now()
					d.SentAt = &now
				}

				if status == StatusDelivered {
					now := time.Now()
					d.DeliveredAt = &now
					m.updateChannelStats(d.Channel, true)
				}

				if status == StatusFailed {
					m.updateChannelStats(d.Channel, false)
					d.RetryCount++
				}

				return nil
			}
		}
	}

	return fmt.Errorf("delivery not found: %s", deliveryID)
}

// updateChannelStats 更新渠道统计
func (m *Manager) updateChannelStats(channel Channel, success bool) {
	stats, ok := m.channelStats[channel]
	if !ok {
		stats = &ChannelStats{Channel: channel}
		m.channelStats[channel] = stats
	}

	stats.TotalSent++
	if success {
		stats.TotalDelivered++
	} else {
		stats.TotalFailed++
	}

	if stats.TotalSent > 0 {
		stats.DeliveryRate = float64(stats.TotalDelivered) / float64(stats.TotalSent) * 100
	}

	stats.LastUsed = time.Now()
}

// SetUserPreference 设置用户偏好
func (m *Manager) SetUserPreference(pref *UserPreference) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pref.UserID == "" {
		return fmt.Errorf("user_id is required")
	}

	pref.UpdatedAt = time.Now()
	m.preferences[pref.UserID] = pref

	m.logger.Info("user preference updated",
		zap.String("user_id", pref.UserID))

	return nil
}

// GetUserPreference 获取用户偏好
func (m *Manager) GetUserPreference(userID string) (*UserPreference, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	pref, ok := m.preferences[userID]
	if !ok {
		return nil, fmt.Errorf("user preference not found: %s", userID)
	}
	return pref, nil
}

// OptimizeChannels 优化渠道配置
func (m *Manager) OptimizeChannels() *OptimizeResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	recommendations := make([]Recommendation, 0)

	// 分析每个渠道的表现
	for channel, stats := range m.channelStats {
		if stats.TotalSent == 0 {
			continue
		}

		// 低投递率
		if stats.DeliveryRate < 80 {
			recommendations = append(recommendations, Recommendation{
				Channel:  channel,
				Action:   "review_and_fix",
				Reason:   fmt.Sprintf("投递率仅 %.1f%%，低于 80%% 基准", stats.DeliveryRate),
				Impact:   "提高通知送达率",
				Priority: 1,
			})
		}

		// 高失败率
		failRate := float64(stats.TotalFailed) / float64(stats.TotalSent) * 100
		if failRate > 20 {
			recommendations = append(recommendations, Recommendation{
				Channel:  channel,
				Action:   "check_configuration",
				Reason:   fmt.Sprintf("失败率 %.1f%%，超过 20%% 阈值", failRate),
				Impact:   "减少通知失败",
				Priority: 2,
			})
		}

		// 长时间未使用
		if !stats.LastUsed.IsZero() && time.Since(stats.LastUsed) > 30*24*time.Hour {
			recommendations = append(recommendations, Recommendation{
				Channel:  channel,
				Action:   "consider_disable",
				Reason:   "超过 30 天未使用",
				Impact:   "简化配置",
				Priority: 3,
			})
		}
	}

	// 检查未配置的渠道
	allChannels := []Channel{ChannelEmail, ChannelSMS, ChannelPush, ChannelSlack, ChannelWeChat, ChannelDingTalk, ChannelWebhook}
	for _, ch := range allChannels {
		if _, ok := m.channelStats[ch]; !ok || m.channelStats[ch].TotalSent == 0 {
			recommendations = append(recommendations, Recommendation{
				Channel:  ch,
				Action:   "consider_enable",
				Reason:   "渠道未启用或从未使用",
				Impact:   "提供备用通知渠道",
				Priority: 4,
			})
		}
	}

	return &OptimizeResult{
		CurrentConfig:   m.channelStats,
		Recommendations: recommendations,
		GeneratedAt:     time.Now(),
	}
}

// GetChannelStats 获取渠道统计
func (m *Manager) GetChannelStats() map[Channel]*ChannelStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := make(map[Channel]*ChannelStats)
	for k, v := range m.channelStats {
		stats[k] = v
	}
	return stats
}
