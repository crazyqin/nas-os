package aitokenmeter

import (
	"sync"
	"time"
)

// Meter Token 计量器，管理滑动窗口限流和用量记录.
type Meter struct {
	mu       sync.RWMutex
	windows  map[string]*slidingWindow // key: userID 或 userID:provider
	usage    []TokenUsage
	maxUsage int // 最大保留用量记录数
}

// NewMeter 创建计量器.
// maxUsage: 最大保留历史用量记录数.
func NewMeter(maxUsage int) *Meter {
	if maxUsage <= 0 {
		maxUsage = 10000
	}
	return &Meter{
		windows:  make(map[string]*slidingWindow),
		usage:    make([]TokenUsage, 0, 256),
		maxUsage: maxUsage,
	}
}

// newSlidingWindow 创建滑动窗口.
func newSlidingWindow(window time.Duration, maxTokens, maxRequests int) *slidingWindow {
	return &slidingWindow{
		window:      window,
		maxTokens:   maxTokens,
		maxRequests: maxRequests,
		events:      make([]windowEvent, 0, 64),
	}
}

// check 检查是否允许指定 Token 数通过 (并发安全).
// 返回 (allowed bool, retryAfter time.Duration).
func (sw *slidingWindow) check(tokens int) (bool, time.Duration) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-sw.window)

	// 清理过期事件
	validIdx := 0
	for i, e := range sw.events {
		if e.timestamp.After(cutoff) {
			validIdx = i
			break
		}
		if i == len(sw.events)-1 {
			validIdx = len(sw.events)
		}
	}
	if validIdx > 0 {
		sw.events = sw.events[validIdx:]
	}

	// 计算窗口内总 Token 和请求数
	totalTokens := 0
	for _, e := range sw.events {
		totalTokens += e.tokens
	}
	requestCount := len(sw.events)

	// 检查请求数限制
	if sw.maxRequests > 0 && requestCount >= sw.maxRequests {
		retry := sw.events[0].timestamp.Add(sw.window).Sub(now)
		if retry < 0 {
			retry = 0
		}
		return false, retry
	}

	// 检查 Token 限制
	if sw.maxTokens > 0 && totalTokens+tokens > sw.maxTokens {
		retry := sw.events[0].timestamp.Add(sw.window).Sub(now)
		if retry < 0 {
			retry = 0
		}
		return false, retry
	}

	return true, 0
}

// add 记录 Token 使用.
func (sw *slidingWindow) add(tokens int) {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	sw.events = append(sw.events, windowEvent{
		timestamp: time.Now(),
		tokens:    tokens,
	})
}

// currentUsage 获取窗口内当前 Token 用量.
func (sw *slidingWindow) currentUsage() int {
	sw.mu.Lock()
	defer sw.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-sw.window)
	total := 0
	for _, e := range sw.events {
		if e.timestamp.After(cutoff) {
			total += e.tokens
		}
	}
	return total
}

// ========== Meter 公开方法 ==========

// getWindowKey 生成窗口 key.
func windowKey(userID string, provider Provider) string {
	if provider == "" {
		return userID
	}
	return userID + ":" + string(provider)
}

// CheckAndRecord 检查限流并记录用量 (原子操作).
// 返回 ErrRateLimited 如果触发限流.
func (m *Meter) CheckAndRecord(usage TokenUsage, limits []RateLimit) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查所有限流规则
	for _, lim := range limits {
		key := windowKey(usage.UserID, lim.Provider)
		sw, exists := m.windows[key]
		if !exists {
			sw = newSlidingWindow(lim.Window, lim.MaxTokens, lim.MaxRequests)
			m.windows[key] = sw
		}
		allowed, _ := sw.check(usage.TotalTokens)
		if !allowed {
			return ErrRateLimited
		}
	}

	// 全部通过，记录到所有相关窗口
	for _, lim := range limits {
		key := windowKey(usage.UserID, lim.Provider)
		sw := m.windows[key]
		sw.add(usage.TotalTokens)
	}

	// 保存用量记录
	m.usage = append(m.usage, usage)
	if len(m.usage) > m.maxUsage {
		m.usage = m.usage[len(m.usage)-m.maxUsage:]
	}

	return nil
}

// GetUserUsage 获取用户在指定时间范围内的总用量.
func (m *Meter) GetUserUsage(userID string, since time.Time) (tokens int, cost float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, u := range m.usage {
		if u.UserID == userID && u.Timestamp.After(since) {
			tokens += u.TotalTokens
			cost += u.Cost
		}
	}
	return
}

// GetProviderUsage 获取指定提供商在时间范围内的总用量.
func (m *Meter) GetProviderUsage(provider Provider, since time.Time) (tokens int, cost float64) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, u := range m.usage {
		if u.Provider == provider && u.Timestamp.After(since) {
			tokens += u.TotalTokens
			cost += u.Cost
		}
	}
	return
}

// GetWindowUsage 获取用户在指定窗口内的当前用量.
func (m *Meter) GetWindowUsage(userID string, provider Provider) int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key := windowKey(userID, provider)
	sw, exists := m.windows[key]
	if !exists {
		return 0
	}
	return sw.currentUsage()
}

// RecentUsage 获取最近 N 条用量记录.
func (m *Meter) RecentUsage(n int) []TokenUsage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if n <= 0 || n > len(m.usage) {
		n = len(m.usage)
	}
	start := len(m.usage) - n
	result := make([]TokenUsage, n)
	copy(result, m.usage[start:])
	return result
}

// UsageCount 总记录数.
func (m *Meter) UsageCount() int {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return len(m.usage)
}
