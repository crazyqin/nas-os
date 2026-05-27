package reverseproxy

import (
	"fmt"
	"net/http"
	"net/http/httputil"
	"net/url"
	"sync"
	"time"
)

// ReverseProxyManager 反向代理管理器
type ReverseProxyManager struct {
	mu      sync.RWMutex
	proxies map[string]*ProxyRule
	config  *ProxyConfig
}

// ProxyConfig 代理配置
type ProxyConfig struct {
	MaxProxies     int           `json:"max_proxies"`
	DefaultTimeout time.Duration `json:"default_timeout"`
	EnableHTTPS    bool          `json:"enable_https"`
}

// ProxyRule 代理规则
type ProxyRule struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	Domain       string            `json:"domain"`
	TargetURL    string            `json:"target_url"`
	Path         string            `json:"path"`
	Enabled      bool              `json:"enabled"`
	SSLEnabled   bool              `json:"ssl_enabled"`
	SSLCert      string            `json:"ssl_cert,omitempty"`
	SSLKey       string            `json:"ssl_key,omitempty"`
	Headers      map[string]string `json:"headers,omitempty"`
	LoadBalancing string           `json:"load_balancing,omitempty"`
	RateLimit    int               `json:"rate_limit,omitempty"`
	CreatedAt    time.Time         `json:"created_at"`
	UpdatedAt    time.Time         `json:"updated_at"`
	RequestCount int64             `json:"request_count"`
	LastAccess   time.Time         `json:"last_access,omitempty"`
}

// ProxyStats 代理统计
type ProxyStats struct {
	TotalRequests   int64   `json:"total_requests"`
	ActiveProxies   int     `json:"active_proxies"`
	TotalProxies    int     `json:"total_proxies"`
	AvgResponseTime float64 `json:"avg_response_time"`
	ErrorRate       float64 `json:"error_rate"`
}

// ReverseProxy 反向代理实体
type ReverseProxy struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Domain     string            `json:"domain"`
	TargetURL  string            `json:"target_url"`
	SSLEnabled bool              `json:"ssl_enabled"`
	CertID     string            `json:"cert_id,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
	Status     string            `json:"status"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// CreateProxyRequest 创建代理请求
type CreateProxyRequest struct {
	Name       string            `json:"name"`
	Domain     string            `json:"domain"`
	TargetURL  string            `json:"target_url"`
	SSLEnabled bool              `json:"ssl_enabled"`
	CertID     string            `json:"cert_id,omitempty"`
	Headers    map[string]string `json:"headers,omitempty"`
}

// UpdateProxyRequest 更新代理请求
type UpdateProxyRequest struct {
	Name       *string            `json:"name,omitempty"`
	Domain     *string            `json:"domain,omitempty"`
	TargetURL  *string            `json:"target_url,omitempty"`
	SSLEnabled *bool              `json:"ssl_enabled,omitempty"`
	CertID     *string            `json:"cert_id,omitempty"`
	Headers    *map[string]string `json:"headers,omitempty"`
	Status     *string            `json:"status,omitempty"`
}

// AddRuleRequest 添加规则请求
type AddRuleRequest struct {
	Path          string `json:"path"`
	TargetURL     string `json:"target_url"`
	LoadBalancing string `json:"load_balancing,omitempty"`
	RateLimit     int    `json:"rate_limit,omitempty"`
}

// SuccessResponse 成功响应
type SuccessResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message,omitempty"`
}

// NewReverseProxyManager 创建反向代理管理器
func NewReverseProxyManager(config *ProxyConfig) *ReverseProxyManager {
	if config == nil {
		config = &ProxyConfig{
			MaxProxies:     100,
			DefaultTimeout: 30 * time.Second,
			EnableHTTPS:    true,
		}
	}
	return &ReverseProxyManager{
		proxies: make(map[string]*ProxyRule),
		config:  config,
	}
}

// AddProxy 添加代理规则
func (m *ReverseProxyManager) AddProxy(rule *ProxyRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		rule.ID = fmt.Sprintf("proxy_%d", time.Now().UnixNano())
	}

	if rule.Domain == "" {
		return fmt.Errorf("domain is required")
	}

	if rule.TargetURL == "" {
		return fmt.Errorf("target URL is required")
	}

	// 验证目标URL
	_, err := url.Parse(rule.TargetURL)
	if err != nil {
		return fmt.Errorf("invalid target URL: %w", err)
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	rule.Enabled = true

	m.proxies[rule.ID] = rule

	return nil
}

// RemoveProxy 移除代理规则
func (m *ReverseProxyManager) RemoveProxy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.proxies[id]; !exists {
		return fmt.Errorf("proxy not found: %s", id)
	}

	delete(m.proxies, id)
	return nil
}

// UpdateProxy 更新代理规则
func (m *ReverseProxyManager) UpdateProxy(id string, rule *ProxyRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.proxies[id]
	if !exists {
		return fmt.Errorf("proxy not found: %s", id)
	}

	if rule.Domain != "" {
		existing.Domain = rule.Domain
	}
	if rule.TargetURL != "" {
		existing.TargetURL = rule.TargetURL
	}
	if rule.Path != "" {
		existing.Path = rule.Path
	}
	if rule.SSLCert != "" {
		existing.SSLCert = rule.SSLCert
	}
	if rule.SSLKey != "" {
		existing.SSLKey = rule.SSLKey
	}
	existing.UpdatedAt = time.Now()

	return nil
}

// GetProxy 获取代理规则
func (m *ReverseProxyManager) GetProxy(id string) (*ProxyRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	proxy, exists := m.proxies[id]
	if !exists {
		return nil, fmt.Errorf("proxy not found: %s", id)
	}
	return proxy, nil
}

// ListProxies 列出所有代理规则
func (m *ReverseProxyManager) ListProxies() []*ProxyRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	proxies := make([]*ProxyRule, 0, len(m.proxies))
	for _, p := range m.proxies {
		proxies = append(proxies, p)
	}
	return proxies
}

// EnableProxy 启用代理
func (m *ReverseProxyManager) EnableProxy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proxy, exists := m.proxies[id]
	if !exists {
		return fmt.Errorf("proxy not found: %s", id)
	}

	proxy.Enabled = true
	proxy.UpdatedAt = time.Now()
	return nil
}

// DisableProxy 禁用代理
func (m *ReverseProxyManager) DisableProxy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proxy, exists := m.proxies[id]
	if !exists {
		return fmt.Errorf("proxy not found: %s", id)
	}

	proxy.Enabled = false
	proxy.UpdatedAt = time.Now()
	return nil
}

// CreateReverseProxy 创建反向代理
func (m *ReverseProxyManager) CreateReverseProxy(rule *ProxyRule) (*httputil.ReverseProxy, error) {
	target, err := url.Parse(rule.TargetURL)
	if err != nil {
		return nil, fmt.Errorf("invalid target URL: %w", err)
	}

	proxy := httputil.NewSingleHostReverseProxy(target)

	// 自定义Director
	originalDirector := proxy.Director
	proxy.Director = func(req *http.Request) {
		originalDirector(req)
		req.Host = target.Host

		// 添加自定义头部
		for key, value := range rule.Headers {
			req.Header.Set(key, value)
		}
	}

	// 错误处理
	proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		w.WriteHeader(http.StatusBadGateway)
		fmt.Fprintf(w, "Proxy error: %v", err)
	}

	return proxy, nil
}

// GetStats 获取统计信息
func (m *ReverseProxyManager) GetStats() *ProxyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &ProxyStats{
		TotalProxies: len(m.proxies),
	}

	for _, proxy := range m.proxies {
		if proxy.Enabled {
			stats.ActiveProxies++
		}
		stats.TotalRequests += proxy.RequestCount
	}

	return stats
}

// FindProxyByDomain 根据域名查找代理
func (m *ReverseProxyManager) FindProxyByDomain(domain string) (*ProxyRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, proxy := range m.proxies {
		if proxy.Domain == domain && proxy.Enabled {
			return proxy, nil
		}
	}

	return nil, fmt.Errorf("no proxy found for domain: %s", domain)
}

// UpdateRequestCount 更新请求计数
func (m *ReverseProxyManager) UpdateRequestCount(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if proxy, exists := m.proxies[id]; exists {
		proxy.RequestCount++
		proxy.LastAccess = time.Now()
	}
}
