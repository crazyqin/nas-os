// Package apilevelmeter 提供 API 使用量监控和配额管理功能，
// 对标 TrueNAS 25.04 User-linked API Keys 管控。
// 追踪每个 API Key 的调用频率、带宽消耗、错误率，并支持配额限制。
// 户部开发。
package apilevelmeter

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// APIKey API Key 定义.
type APIKey struct {
	ID          string    `json:"id"`
	Label       string    `json:"label"`
	UserID      string    `json:"user_id"`
	KeyPrefix   string    `json:"key_prefix"` // 只存前8位用于显示
	Scopes      []string  `json:"scopes"`
	CreatedAt   time.Time `json:"created_at"`
	LastUsedAt  *time.Time `json:"last_used_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Disabled    bool      `json:"disabled"`
}

// UsageRecord 使用量记录.
type UsageRecord struct {
	KeyID          string    `json:"key_id"`
	Endpoint       string    `json:"endpoint"`
	Method         string    `json:"method"`
	StatusCode     int       `json:"status_code"`
	ResponseTimeMs int64     `json:"response_time_ms"`
	BytesIn        int64    `json:"bytes_in"`
	BytesOut       int64    `json:"bytes_out"`
	Timestamp      time.Time `json:"timestamp"`
}

// UsageSummary 使用量汇总.
type UsageSummary struct {
	KeyID           string  `json:"key_id"`
	TotalRequests   int64   `json:"total_requests"`
	ErrorCount      int64   `json:"error_count"`
	ErrorRate       float64 `json:"error_rate"`
	TotalBytesIn     int64   `json:"total_bytes_in"`
	TotalBytesOut   int64   `json:"total_bytes_out"`
	AvgResponseMs   float64 `json:"avg_response_ms"`
	P99ResponseMs   float64 `json:"p99_response_ms"`
	RequestsPerMin  float64 `json:"requests_per_min"`
	UniqueEndpoints  int     `json:"unique_endpoints"`
	WindowMinutes   int     `json:"window_minutes"`
}

// Quota API Key 配额.
type Quota struct {
	KeyID             string `json:"key_id"`
	MaxRequestsPerMin int    `json:"max_requests_per_min"`
	MaxBytesPerHour   int64  `json:"max_bytes_per_hour"`
	MaxErrorsPerHour  int    `json:"max_errors_per_hour"`
	CurrentReqPerMin  int    `json:"current_req_per_min"`
	CurrentBytesHour  int64  `json:"current_bytes_hour"`
	CurrentErrorsHour int    `json:"current_errors_hour"`
	Throttled         bool   `json:"throttled"`
}

// Alert 配额告警.
type Alert struct {
	KeyID     string    `json:"key_id"`
	Type      string    `json:"type"` // rate_limit, bandwidth, error_rate, expired
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
}

// Manager API 使用量管理器.
type Manager struct {
	mu       sync.RWMutex
	keys     map[string]*APIKey
	records  map[string][]UsageRecord // keyID -> records
	quotas   map[string]*Quota
	alerts   []Alert
	maxRecordsPerKey int
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		keys:             make(map[string]*APIKey),
		records:          make(map[string][]UsageRecord),
		quotas:           make(map[string]*Quota),
		maxRecordsPerKey: 10000,
	}
}

// RegisterKey 注册 API Key.
func (m *Manager) RegisterKey(key *APIKey) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if key.ID == "" {
		return fmt.Errorf("key ID cannot be empty")
	}
	if key.Label == "" {
		return fmt.Errorf("key label cannot be empty")
	}
	key.CreatedAt = time.Now()
	m.keys[key.ID] = key
	return nil
}

// DisableKey 禁用 Key.
func (m *Manager) DisableKey(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	k, ok := m.keys[id]
	if !ok {
		return fmt.Errorf("key %s not found", id)
	}
	k.Disabled = true
	return nil
}

// ListKeys 列出 API Keys.
func (m *Manager) ListKeys() []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*APIKey, 0, len(m.keys))
	for _, k := range m.keys {
		result = append(result, k)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.Before(result[j].CreatedAt) })
	return result
}

// RecordUsage 记录 API 使用.
func (m *Manager) RecordUsage(rec UsageRecord) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.keys[rec.KeyID]; !ok {
		return fmt.Errorf("key %s not found", rec.KeyID)
	}
	if _, ok := m.records[rec.KeyID]; !ok {
		m.records[rec.KeyID] = make([]UsageRecord, 0, m.maxRecordsPerKey)
	}
	records := m.records[rec.KeyID]
	records = append(records, rec)
	if len(records) > m.maxRecordsPerKey {
		records = records[len(records)-m.maxRecordsPerKey:]
	}
	m.records[rec.KeyID] = records
	// 更新 last used
	now := time.Now()
	m.keys[rec.KeyID].LastUsedAt = &now
	return nil
}

// GetSummary 获取使用量汇总.
func (m *Manager) GetSummary(keyID string, windowMinutes int) (*UsageSummary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	records, ok := m.records[keyID]
	if !ok {
		return &UsageSummary{KeyID: keyID}, nil
	}
	if windowMinutes <= 0 {
		windowMinutes = 60
	}
	cutoff := time.Now().Add(-time.Duration(windowMinutes) * time.Minute)
	endpoints := make(map[string]bool)
	var totalReq, errCount int64
	var totalIn, totalOut int64
	var totalRespMs float64
	var respTimes []float64
	for _, r := range records {
		if r.Timestamp.Before(cutoff) {
			continue
		}
		totalReq++
		endpoints[r.Endpoint] = true
		if r.StatusCode >= 400 {
			errCount++
		}
		totalIn += r.BytesIn
		totalOut += r.BytesOut
		totalRespMs += float64(r.ResponseTimeMs)
		respTimes = append(respTimes, float64(r.ResponseTimeMs))
	}
	summary := &UsageSummary{
		KeyID:          keyID,
		TotalRequests:  totalReq,
		ErrorCount:     errCount,
		UniqueEndpoints: len(endpoints),
		WindowMinutes:  windowMinutes,
	}
	if totalReq > 0 {
		summary.ErrorRate = float64(errCount) / float64(totalReq)
		summary.AvgResponseMs = totalRespMs / float64(totalReq)
		summary.RequestsPerMin = float64(totalReq) / float64(windowMinutes)
		summary.TotalBytesIn = totalIn
		summary.TotalBytesOut = totalOut
	}
	if len(respTimes) > 0 {
		sort.Float64s(respTimes)
		p99Idx := int(float64(len(respTimes)) * 0.99)
		if p99Idx >= len(respTimes) {
			p99Idx = len(respTimes) - 1
		}
		summary.P99ResponseMs = respTimes[p99Idx]
	}
	return summary, nil
}

// SetQuota 设置配额.
func (m *Manager) SetQuota(q *Quota) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if q.KeyID == "" {
		return fmt.Errorf("key ID required")
	}
	m.quotas[q.KeyID] = q
	return nil
}

// CheckQuota 检查配额.
func (m *Manager) CheckQuota(keyID string) (*Quota, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	q, ok := m.quotas[keyID]
	if !ok {
		return &Quota{KeyID: keyID}, nil
	}
	// 检查是否超限
	if q.MaxRequestsPerMin > 0 && q.CurrentReqPerMin >= q.MaxRequestsPerMin {
		q.Throttled = true
		m.alerts = append(m.alerts, Alert{
			KeyID: keyID, Type: "rate_limit",
			Message: fmt.Sprintf("Rate limit exceeded: %d/%d req/min", q.CurrentReqPerMin, q.MaxRequestsPerMin),
			Timestamp: time.Now(),
		})
	}
	if q.MaxBytesPerHour > 0 && q.CurrentBytesHour >= q.MaxBytesPerHour {
		q.Throttled = true
		m.alerts = append(m.alerts, Alert{
			KeyID: keyID, Type: "bandwidth",
			Message: fmt.Sprintf("Bandwidth limit exceeded: %d/%d bytes/hr", q.CurrentBytesHour, q.MaxBytesPerHour),
			Timestamp: time.Now(),
		})
	}
	if q.MaxErrorsPerHour > 0 && q.CurrentErrorsHour >= q.MaxErrorsPerHour {
		m.alerts = append(m.alerts, Alert{
			KeyID: keyID, Type: "error_rate",
			Message: fmt.Sprintf("Error limit exceeded: %d/%d errors/hr", q.CurrentErrorsHour, q.MaxErrorsPerHour),
			Timestamp: time.Now(),
		})
	}
	return q, nil
}

// ListAlerts 列出告警.
func (m *Manager) ListAlerts() []Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Alert, len(m.alerts))
	copy(result, m.alerts)
	return result
}

// ClearAlerts 清除告警.
func (m *Manager) ClearAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = m.alerts[:0]
}