package reverseproxy

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager manages reverse proxy configurations (REST API oriented)
type Manager struct {
	mu      sync.RWMutex
	proxies map[string]*ReverseProxy
	rules   map[string][]ProxyRule // proxyID -> rules
	stats   ProxyStats
}

// NewManager creates a new reverse proxy manager with mock data
func NewManager() *Manager {
	m := &Manager{
		proxies: make(map[string]*ReverseProxy),
		rules:   make(map[string][]ProxyRule),
	}

	m.addMockProxies()
	return m
}

func (m *Manager) addMockProxies() {
	mockProxies := []struct {
		name      string
		domain    string
		targetURL string
	}{
		{"web-app", "app.example.com", "http://localhost:3000"},
		{"api-service", "api.example.com", "http://localhost:8080"},
		{"admin-panel", "admin.example.com", "http://localhost:9090"},
	}

	for _, mp := range mockProxies {
		proxy := &ReverseProxy{
			ID:         uuid.New().String(),
			Name:       mp.name,
			Domain:     mp.domain,
			TargetURL:  mp.targetURL,
			SSLEnabled: true,
			Status:     "active",
			Headers:    map[string]string{"X-Forwarded-For": "true"},
			CreatedAt:  time.Now().Add(-time.Duration(len(m.proxies)) * 24 * time.Hour),
			UpdatedAt:  time.Now(),
		}
		m.proxies[proxy.ID] = proxy
		m.rules[proxy.ID] = []ProxyRule{
			{
				ID:        uuid.New().String(),
				Path:      "/",
				TargetURL: mp.targetURL,
			},
		}
	}

	m.stats = ProxyStats{
		TotalProxies:    len(m.proxies),
		ActiveProxies:   len(m.proxies),
		TotalRequests:   125000,
		AvgResponseTime: 45.5,
		ErrorRate:       0.02,
	}
}

// CreateProxy creates a new reverse proxy
func (m *Manager) CreateProxy(req CreateProxyRequest) (*ReverseProxy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, p := range m.proxies {
		if p.Domain == req.Domain {
			return nil, fmt.Errorf("proxy with domain '%s' already exists", req.Domain)
		}
	}

	proxy := &ReverseProxy{
		ID:         uuid.New().String(),
		Name:       req.Name,
		Domain:     req.Domain,
		TargetURL:  req.TargetURL,
		SSLEnabled: req.SSLEnabled,
		CertID:     req.CertID,
		Headers:    req.Headers,
		Status:     "active",
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	if proxy.Headers == nil {
		proxy.Headers = make(map[string]string)
	}

	m.proxies[proxy.ID] = proxy
	m.rules[proxy.ID] = []ProxyRule{}
	m.updateStats()

	return proxy, nil
}

// UpdateProxy updates an existing reverse proxy
func (m *Manager) UpdateProxy(id string, req UpdateProxyRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	proxy, ok := m.proxies[id]
	if !ok {
		return fmt.Errorf("proxy not found: %s", id)
	}

	if req.Name != nil {
		proxy.Name = *req.Name
	}
	if req.Domain != nil {
		for pid, p := range m.proxies {
			if pid != id && p.Domain == *req.Domain {
				return fmt.Errorf("proxy with domain '%s' already exists", *req.Domain)
			}
		}
		proxy.Domain = *req.Domain
	}
	if req.TargetURL != nil {
		proxy.TargetURL = *req.TargetURL
	}
	if req.SSLEnabled != nil {
		proxy.SSLEnabled = *req.SSLEnabled
	}
	if req.CertID != nil {
		proxy.CertID = *req.CertID
	}
	if req.Headers != nil {
		proxy.Headers = *req.Headers
	}
	if req.Status != nil {
		proxy.Status = *req.Status
	}

	proxy.UpdatedAt = time.Now()
	m.updateStats()

	return nil
}

// DeleteProxy deletes a reverse proxy
func (m *Manager) DeleteProxy(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.proxies[id]; !ok {
		return fmt.Errorf("proxy not found: %s", id)
	}

	delete(m.proxies, id)
	delete(m.rules, id)
	m.updateStats()

	return nil
}

// GetProxy returns a specific proxy by ID
func (m *Manager) GetProxy(id string) (*ReverseProxy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	proxy, ok := m.proxies[id]
	if !ok {
		return nil, fmt.Errorf("proxy not found: %s", id)
	}
	return proxy, nil
}

// ListProxies returns all configured proxies
func (m *Manager) ListProxies() []ReverseProxy {
	m.mu.RLock()
	defer m.mu.RUnlock()

	proxies := make([]ReverseProxy, 0, len(m.proxies))
	for _, p := range m.proxies {
		proxies = append(proxies, *p)
	}
	return proxies
}

// ReloadConfig reloads the proxy configuration
func (m *Manager) ReloadConfig() error {
	return nil
}

// GetStats returns aggregated proxy statistics
func (m *Manager) GetStats() ProxyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats
}

// AddRule adds a routing rule to a proxy
func (m *Manager) AddRule(proxyID string, rule ProxyRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.proxies[proxyID]; !ok {
		return fmt.Errorf("proxy not found: %s", proxyID)
	}

	rule.ID = uuid.New().String()
	m.rules[proxyID] = append(m.rules[proxyID], rule)

	return nil
}

// GetRules returns all rules for a proxy
func (m *Manager) GetRules(proxyID string) ([]ProxyRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.proxies[proxyID]; !ok {
		return nil, fmt.Errorf("proxy not found: %s", proxyID)
	}

	rules := m.rules[proxyID]
	if rules == nil {
		rules = []ProxyRule{}
	}
	return rules, nil
}

func (m *Manager) updateStats() {
	active := 0
	for _, p := range m.proxies {
		if p.Status == "active" {
			active++
		}
	}

	m.stats = ProxyStats{
		TotalProxies:    len(m.proxies),
		ActiveProxies:   active,
		TotalRequests:   m.stats.TotalRequests,
		AvgResponseTime: m.stats.AvgResponseTime,
		ErrorRate:       m.stats.ErrorRate,
	}
}
