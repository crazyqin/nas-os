package smartapi

import (
	"sync"
	"time"

	"go.uber.org/zap"
)

// APIKey API密钥.
type APIKey struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Key         string    `json:"key"`
	Secret      string    `json:"secret"`
	Scopes      []string  `json:"scopes"`
	RateLimit   int       `json:"rate_limit"` // requests per minute
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	ExpiresAt   time.Time `json:"expires_at"`
	LastUsedAt  time.Time `json:"last_used_at"`
	Metadata    map[string]string `json:"metadata"`
}

// RateLimitConfig 限流配置.
type RateLimitConfig struct {
	ID         string `json:"id"`
	APIKeyID   string `json:"api_key_id"`
	WindowSec  int    `json:"window_sec"`
	MaxRequests int   `json:"max_requests"`
	BurstSize  int    `json:"burst_size"`
}

// APIUsageStats API使用统计.
type APIUsageStats struct {
	APIKeyID     string  `json:"api_key_id"`
	TotalCalls   int64   `json:"total_calls"`
	SuccessCalls int64   `json:"success_calls"`
	ErrorCalls   int64   `json:"error_calls"`
	AvgLatencyMs float64 `json:"avg_latency_ms"`
	P95LatencyMs float64 `json:"p95_latency_ms"`
	Period       string  `json:"period"`
}

// APIEndpoint API端点.
type APIEndpoint struct {
	Path        string   `json:"path"`
	Method      string   `json:"method"`
	Description string   `json:"description"`
	Scopes      []string `json:"scopes"`
	RateLimit   int      `json:"rate_limit"`
	Deprecated  bool     `json:"deprecated"`
	Version     string   `json:"version"`
}

// RequestLog 请求日志.
type RequestLog struct {
	ID          string    `json:"id"`
	APIKeyID    string    `json:"api_key_id"`
	Path        string    `json:"path"`
	Method      string    `json:"method"`
	StatusCode  int       `json:"status_code"`
	LatencyMs   float64   `json:"latency_ms"`
	IP          string    `json:"ip"`
	UserAgent   string    `json:"user_agent"`
	Timestamp   time.Time `json:"timestamp"`
}

// Engine API网关增强引擎.
type Engine struct {
	apiKeys    map[string]*APIKey
	rateLimits map[string]*RateLimitConfig
	stats      map[string]*APIUsageStats
	endpoints  map[string]*APIEndpoint
	logs       []*RequestLog
	logger     *zap.Logger
	mu         sync.RWMutex
}

// NewEngine 创建API网关增强引擎.
func NewEngine(logger *zap.Logger) *Engine {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Engine{
		apiKeys:    make(map[string]*APIKey),
		rateLimits: make(map[string]*RateLimitConfig),
		stats:      make(map[string]*APIUsageStats),
		endpoints:  make(map[string]*APIEndpoint),
		logger:     logger,
	}
}

// CreateAPIKey 创建API密钥.
func (e *Engine) CreateAPIKey(key *APIKey) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if key.ID == "" {
		return ErrInvalidKeyID
	}
	key.CreatedAt = time.Now()
	if key.Metadata == nil {
		key.Metadata = make(map[string]string)
	}
	e.apiKeys[key.ID] = key
	e.logger.Info("API密钥已创建", zap.String("id", key.ID), zap.String("name", key.Name))
	return nil
}

// GetAPIKey 获取API密钥.
func (e *Engine) GetAPIKey(id string) (*APIKey, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	key, ok := e.apiKeys[id]
	return key, ok
}

// ValidateAPIKey 验证API密钥.
func (e *Engine) ValidateAPIKey(keyStr string) (*APIKey, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	for _, key := range e.apiKeys {
		if key.Key == keyStr && key.Enabled {
			if !key.ExpiresAt.IsZero() && time.Now().After(key.ExpiresAt) {
				return nil, ErrKeyExpired
			}
			return key, nil
		}
	}
	return nil, ErrKeyNotFound
}

// ListAPIKeys 列出API密钥.
func (e *Engine) ListAPIKeys() []*APIKey {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	keys := make([]*APIKey, 0, len(e.apiKeys))
	for _, k := range e.apiKeys {
		keys = append(keys, k)
	}
	return keys
}

// RevokeAPIKey 撤销API密钥.
func (e *Engine) RevokeAPIKey(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	key, ok := e.apiKeys[id]
	if !ok {
		return ErrKeyNotFound
	}
	key.Enabled = false
	e.logger.Info("API密钥已撤销", zap.String("id", id))
	return nil
}

// SetRateLimit 设置限流.
func (e *Engine) SetRateLimit(config *RateLimitConfig) error {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	if config.ID == "" {
		return ErrInvalidConfigID
	}
	e.rateLimits[config.APIKeyID] = config
	return nil
}

// RegisterEndpoint 注册端点.
func (e *Engine) RegisterEndpoint(endpoint *APIEndpoint) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	key := endpoint.Method + ":" + endpoint.Path
	e.endpoints[key] = endpoint
}

// LogRequest 记录请求.
func (e *Engine) LogRequest(log *RequestLog) {
	e.mu.Lock()
	defer e.mu.Unlock()
	
	log.Timestamp = time.Now()
	e.logs = append(e.logs, log)
	
	// 更新统计
	if stats, ok := e.stats[log.APIKeyID]; ok {
		stats.TotalCalls++
		if log.StatusCode >= 200 && log.StatusCode < 400 {
			stats.SuccessCalls++
		} else {
			stats.ErrorCalls++
		}
		stats.AvgLatencyMs = (stats.AvgLatencyMs*float64(stats.TotalCalls-1) + log.LatencyMs) / float64(stats.TotalCalls)
	}
}

// GetUsageStats 获取使用统计.
func (e *Engine) GetUsageStats(apiKeyID string) *APIUsageStats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	if stats, ok := e.stats[apiKeyID]; ok {
		return stats
	}
	return &APIUsageStats{APIKeyID: apiKeyID}
}

// GetGatewayStats 获取网关统计.
func (e *Engine) GetGatewayStats() map[string]interface{} {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	activeKeys := 0
	for _, k := range e.apiKeys {
		if k.Enabled {
			activeKeys++
		}
	}
	
	return map[string]interface{}{
		"total_keys":    len(e.apiKeys),
		"active_keys":   activeKeys,
		"total_endpoints": len(e.endpoints),
		"total_requests": len(e.logs),
		"rate_limits":   len(e.rateLimits),
	}
}

// GetRecentLogs 获取最近日志.
func (e *Engine) GetRecentLogs(limit int) []*RequestLog {
	e.mu.RLock()
	defer e.mu.RUnlock()
	
	if limit > len(e.logs) {
		limit = len(e.logs)
	}
	
	start := len(e.logs) - limit
	if start < 0 {
		start = 0
	}
	
	logs := make([]*RequestLog, limit)
	copy(logs, e.logs[start:])
	return logs
}
