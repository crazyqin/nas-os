package aiconsole

import (
	"sync"
	"time"
)

// Dashboard 使用统计仪表盘.
type Dashboard struct {
	mu        sync.RWMutex
	store     *Store
	startTime time.Time
}

// NewDashboard 创建仪表盘实例.
func NewDashboard(store *Store) *Dashboard {
	return &Dashboard{
		store:     store,
		startTime: time.Now(),
	}
}

// OverviewStats 总览统计.
type OverviewStats struct {
	TotalRequests      int64         `json:"totalRequests"`
	SuccessRequests    int64         `json:"successRequests"`
	FailedRequests     int64         `json:"failedRequests"`
	TotalTokens        int64         `json:"totalTokens"`
	PromptTokens       int64         `json:"promptTokens"`
	CompletionTokens   int64         `json:"completionTokens"`
	AvgDurationMs      float64       `json:"avgDurationMs"`
	ActiveModels       int           `json:"activeModels"`
	TotalUsers         int           `json:"totalUsers"`
	Uptime             time.Duration `json:"uptime"`
	RedactedRequests   int64         `json:"redactedRequests"`
	TotalRedactCount   int64         `json:"totalRedactCount"`
}

// ModelUsageStats 模型使用统计.
type ModelUsageStats struct {
	ModelID            string  `json:"modelId"`
	ModelName          string  `json:"modelName"`
	Provider           string  `json:"provider"`
	RequestCount       int64   `json:"requestCount"`
	SuccessCount       int64   `json:"successCount"`
	FailureCount       int64   `json:"failureCount"`
	TotalTokens        int64   `json:"totalTokens"`
	AvgDurationMs      float64 `json:"avgDurationMs"`
	LastUsedAt         time.Time `json:"lastUsedAt"`
}

// UserUsageStats 用户使用统计.
type UserUsageStats struct {
	UserID             string  `json:"userId"`
	Username           string  `json:"username"`
	RequestCount       int64   `json:"requestCount"`
	TotalTokens        int64   `json:"totalTokens"`
	PromptTokens       int64   `json:"promptTokens"`
	CompletionTokens   int64   `json:"completionTokens"`
	LastUsedAt         time.Time `json:"lastUsedAt"`
}

// UsageTrend 使用趋势.
type UsageTrend struct {
	Date             string `json:"date"`
	RequestCount     int64  `json:"requestCount"`
	TotalTokens      int64  `json:"totalTokens"`
	SuccessCount     int64  `json:"successCount"`
	FailedCount      int64  `json:"failedCount"`
}

// GetOverview 获取总览统计.
func (d *Dashboard) GetOverview() (*OverviewStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	stats := &OverviewStats{
		Uptime: time.Since(d.startTime),
	}

	// 查询审计日志统计
	rows, err := d.store.db.Query(`
		SELECT 
			COUNT(*) as total,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as success,
			SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END) as failed,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) as completion_tokens,
			COALESCE(AVG(duration_ms), 0) as avg_duration,
			COALESCE(SUM(CASE WHEN redacted = 1 THEN 1 ELSE 0 END), 0) as redacted,
			COALESCE(SUM(redact_count), 0) as redact_count
		FROM ai_audit_logs
	`)
	if err != nil {
		return stats, nil
	}
	defer rows.Close()

	if rows.Next() {
		_ = rows.Scan(
			&stats.TotalRequests,
			&stats.SuccessRequests,
			&stats.FailedRequests,
			&stats.TotalTokens,
			&stats.PromptTokens,
			&stats.CompletionTokens,
			&stats.AvgDurationMs,
			&stats.RedactedRequests,
			&stats.TotalRedactCount,
		)
	}

	// 查询活跃模型数
	var modelCount int
	_ = d.store.db.QueryRow("SELECT COUNT(*) FROM ai_models WHERE enabled = 1 AND status = 'active'").Scan(&modelCount)
	stats.ActiveModels = modelCount

	// 查询用户数
	var userCount int
	_ = d.store.db.QueryRow("SELECT COUNT(DISTINCT user_id) FROM ai_audit_logs WHERE user_id != ''").Scan(&userCount)
	stats.TotalUsers = userCount

	return stats, nil
}

// GetModelStats 获取模型使用统计.
func (d *Dashboard) GetModelStats() ([]*ModelUsageStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.store.db.Query(`
		SELECT 
			a.model_id,
			COALESCE(m.name, a.model_name) as model_name,
			COALESCE(m.provider, '') as provider,
			COUNT(*) as request_count,
			SUM(CASE WHEN a.success = 1 THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN a.success = 0 THEN 1 ELSE 0 END) as failure_count,
			COALESCE(SUM(a.total_tokens), 0) as total_tokens,
			COALESCE(AVG(a.duration_ms), 0) as avg_duration,
			MAX(a.timestamp) as last_used
		FROM ai_audit_logs a
		LEFT JOIN ai_models m ON a.model_id = m.id
		GROUP BY a.model_id
		ORDER BY request_count DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []*ModelUsageStats
	for rows.Next() {
		s := &ModelUsageStats{}
		var lastUsed *time.Time
		if err := rows.Scan(
			&s.ModelID, &s.ModelName, &s.Provider,
			&s.RequestCount, &s.SuccessCount, &s.FailureCount,
			&s.TotalTokens, &s.AvgDurationMs, &lastUsed,
		); err != nil {
			continue
		}
		if lastUsed != nil {
			s.LastUsedAt = *lastUsed
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// GetUserStats 获取用户使用统计.
func (d *Dashboard) GetUserStats(limit int) ([]*UserUsageStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if limit <= 0 {
		limit = 20
	}

	rows, err := d.store.db.Query(`
		SELECT 
			user_id,
			MAX(username) as username,
			COUNT(*) as request_count,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			COALESCE(SUM(prompt_tokens), 0) as prompt_tokens,
			COALESCE(SUM(completion_tokens), 0) as completion_tokens,
			MAX(timestamp) as last_used
		FROM ai_audit_logs
		WHERE user_id != ''
		GROUP BY user_id
		ORDER BY total_tokens DESC
		LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []*UserUsageStats
	for rows.Next() {
		s := &UserUsageStats{}
		var lastUsed *time.Time
		if err := rows.Scan(
			&s.UserID, &s.Username, &s.RequestCount,
			&s.TotalTokens, &s.PromptTokens, &s.CompletionTokens,
			&lastUsed,
		); err != nil {
			continue
		}
		if lastUsed != nil {
			s.LastUsedAt = *lastUsed
		}
		stats = append(stats, s)
	}
	return stats, nil
}

// GetUsageTrend 获取使用趋势（按天）.
func (d *Dashboard) GetUsageTrend(days int) ([]*UsageTrend, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if days <= 0 {
		days = 7
	}

	// 计算起始日期
	startDate := time.Now().AddDate(0, 0, -days).Format("2006-01-02")

	rows, err := d.store.db.Query(`
		SELECT 
			DATE(timestamp) as date,
			COUNT(*) as request_count,
			COALESCE(SUM(total_tokens), 0) as total_tokens,
			SUM(CASE WHEN success = 1 THEN 1 ELSE 0 END) as success_count,
			SUM(CASE WHEN success = 0 THEN 1 ELSE 0 END) as failed_count
		FROM ai_audit_logs
		WHERE DATE(timestamp) >= ?
		GROUP BY DATE(timestamp)
		ORDER BY date
	`, startDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var trends []*UsageTrend
	for rows.Next() {
		t := &UsageTrend{}
		if err := rows.Scan(&t.Date, &t.RequestCount, &t.TotalTokens, &t.SuccessCount, &t.FailedCount); err != nil {
			continue
		}
		trends = append(trends, t)
	}
	return trends, nil
}

// GetProviderStats 获取提供者统计.
func (d *Dashboard) GetProviderStats() (map[string]*ProviderStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	rows, err := d.store.db.Query(`
		SELECT 
			COALESCE(m.provider, 'unknown') as provider,
			COUNT(*) as request_count,
			COALESCE(SUM(a.total_tokens), 0) as total_tokens,
			COALESCE(AVG(a.duration_ms), 0) as avg_duration
		FROM ai_audit_logs a
		LEFT JOIN ai_models m ON a.model_id = m.id
		GROUP BY m.provider
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	stats := make(map[string]*ProviderStats)
	for rows.Next() {
		s := &ProviderStats{}
		var provider string
		if err := rows.Scan(&provider, &s.RequestCount, &s.TotalTokens, &s.AvgDurationMs); err != nil {
			continue
		}
		stats[provider] = s
	}
	return stats, nil
}

// ProviderStats 提供者统计.
type ProviderStats struct {
	RequestCount  int64   `json:"requestCount"`
	TotalTokens   int64   `json:"totalTokens"`
	AvgDurationMs float64 `json:"avgDurationMs"`
}
