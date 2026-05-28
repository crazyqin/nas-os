package webapphost

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// Router 路由管理器
type Router struct {
	mu        sync.RWMutex
	routes    map[string]*RouteRule
	domains   map[string]*DomainConfig
	domainApp map[string]string // domain -> appID
}

// NewRouter 创建路由管理器
func NewRouter() *Router {
	return &Router{
		routes:    make(map[string]*RouteRule),
		domains:   make(map[string]*DomainConfig),
		domainApp: make(map[string]string),
	}
}

// AddRoute 添加路由规则
func (r *Router) AddRoute(rule *RouteRule) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if rule.ID == "" {
		rule.ID = GenerateID("route")
	}

	if rule.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if rule.Path == "" {
		rule.Path = "/"
	}
	if rule.AppID == "" {
		return fmt.Errorf("app ID is required")
	}

	// 检查域名+路径唯一性
	for _, existing := range r.routes {
		if existing.Domain == rule.Domain && existing.Path == rule.Path && existing.ID != rule.ID {
			return fmt.Errorf("route already exists for %s%s", rule.Domain, rule.Path)
		}
	}

	rule.CreatedAt = time.Now()
	r.routes[rule.ID] = rule

	log.Printf("Route added: %s%s -> app %s", rule.Domain, rule.Path, rule.AppID)
	return nil
}

// RemoveRoute 移除路由规则
func (r *Router) RemoveRoute(id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.routes[id]; !exists {
		return fmt.Errorf("route not found: %s", id)
	}

	delete(r.routes, id)
	log.Printf("Route removed: %s", id)
	return nil
}

// UpdateRoute 更新路由规则
func (r *Router) UpdateRoute(id string, updates *RouteRule) (*RouteRule, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	route, exists := r.routes[id]
	if !exists {
		return nil, fmt.Errorf("route not found: %s", id)
	}

	if updates.Domain != "" {
		route.Domain = updates.Domain
	}
	if updates.Path != "" {
		route.Path = updates.Path
	}
	if updates.AppID != "" {
		route.AppID = updates.AppID
	}
	if updates.Priority != 0 {
		route.Priority = updates.Priority
	}
	if updates.StripPath != route.StripPath {
		route.StripPath = updates.StripPath
	}
	if updates.Headers != nil {
		route.Headers = updates.Headers
	}

	return route, nil
}

// GetRoute 获取路由规则
func (r *Router) GetRoute(id string) (*RouteRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	route, exists := r.routes[id]
	if !exists {
		return nil, fmt.Errorf("route not found: %s", id)
	}
	return route, nil
}

// ListRoutes 列出所有路由规则
func (r *Router) ListRoutes() []*RouteRule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make([]*RouteRule, 0, len(r.routes))
	for _, route := range r.routes {
		routes = append(routes, route)
	}

	sort.Slice(routes, func(i, j int) bool {
		if routes[i].Domain != routes[j].Domain {
			return routes[i].Domain < routes[j].Domain
		}
		return routes[i].Priority > routes[j].Priority
	})

	return routes
}

// ListRoutesByApp 列出应用的路由规则
func (r *Router) ListRoutesByApp(appID string) []*RouteRule {
	r.mu.RLock()
	defer r.mu.RUnlock()

	routes := make([]*RouteRule, 0)
	for _, route := range r.routes {
		if route.AppID == appID {
			routes = append(routes, route)
		}
	}

	sort.Slice(routes, func(i, j int) bool {
		return routes[i].Priority > routes[j].Priority
	})

	return routes
}

// MatchRoute 匹配路由
func (r *Router) MatchRoute(domain, path string) (*RouteRule, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 按优先级排序匹配
	var matches []*RouteRule
	for _, route := range r.routes {
		if route.Domain == domain && matchPath(route.Path, path) {
			matches = append(matches, route)
		}
	}

	if len(matches) == 0 {
		return nil, fmt.Errorf("no route found for %s%s", domain, path)
	}

	// 按优先级排序
	sort.Slice(matches, func(i, j int) bool {
		return matches[i].Priority > matches[j].Priority
	})

	return matches[0], nil
}

// matchPath 匹配路径
func matchPath(routePath, requestPath string) bool {
	if routePath == "/" {
		return true
	}
	if routePath == requestPath {
		return true
	}
	// 前缀匹配
	if len(requestPath) >= len(routePath) && requestPath[:len(routePath)] == routePath {
		return true
	}
	return false
}

// AddDomain 添加域名配置
func (r *Router) AddDomain(config *DomainConfig) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if config.Domain == "" {
		return fmt.Errorf("domain is required")
	}
	if config.AppID == "" {
		return fmt.Errorf("app ID is required")
	}

	if _, exists := r.domains[config.Domain]; exists {
		return fmt.Errorf("domain already exists: %s", config.Domain)
	}

	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()
	r.domains[config.Domain] = config
	r.domainApp[config.Domain] = config.AppID

	log.Printf("Domain added: %s -> app %s", config.Domain, config.AppID)
	return nil
}

// RemoveDomain 移除域名配置
func (r *Router) RemoveDomain(domain string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.domains[domain]; !exists {
		return fmt.Errorf("domain not found: %s", domain)
	}

	// 移除相关路由
	for id, route := range r.routes {
		if route.Domain == domain {
			delete(r.routes, id)
		}
	}

	delete(r.domains, domain)
	delete(r.domainApp, domain)

	log.Printf("Domain removed: %s", domain)
	return nil
}

// UpdateDomain 更新域名配置
func (r *Router) UpdateDomain(domain string, updates *DomainConfig) (*DomainConfig, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	config, exists := r.domains[domain]
	if !exists {
		return nil, fmt.Errorf("domain not found: %s", domain)
	}

	if updates.SSLEnabled != config.SSLEnabled {
		config.SSLEnabled = updates.SSLEnabled
	}
	if updates.CertID != "" {
		config.CertID = updates.CertID
	}
	if updates.RedirectHTTPS != config.RedirectHTTPS {
		config.RedirectHTTPS = updates.RedirectHTTPS
	}
	if updates.Headers != nil {
		config.Headers = updates.Headers
	}

	config.UpdatedAt = time.Now()
	return config, nil
}

// GetDomain 获取域名配置
func (r *Router) GetDomain(domain string) (*DomainConfig, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	config, exists := r.domains[domain]
	if !exists {
		return nil, fmt.Errorf("domain not found: %s", domain)
	}
	return config, nil
}

// ListDomains 列出所有域名配置
func (r *Router) ListDomains() []*DomainConfig {
	r.mu.RLock()
	defer r.mu.RUnlock()

	domains := make([]*DomainConfig, 0, len(r.domains))
	for _, domain := range r.domains {
		domains = append(domains, domain)
	}

	sort.Slice(domains, func(i, j int) bool {
		return domains[i].Domain < domains[j].Domain
	})

	return domains
}

// GetAppByDomain 根据域名获取应用 ID
func (r *Router) GetAppByDomain(domain string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	appID, exists := r.domainApp[domain]
	if !exists {
		return "", fmt.Errorf("no app found for domain: %s", domain)
	}
	return appID, nil
}

// RemoveAppRoutes 移除应用的所有路由
func (r *Router) RemoveAppRoutes(appID string) {
	r.mu.Lock()
	defer r.mu.Unlock()

	for id, route := range r.routes {
		if route.AppID == appID {
			delete(r.routes, id)
		}
	}

	for domain, aid := range r.domainApp {
		if aid == appID {
			delete(r.domains, domain)
			delete(r.domainApp, domain)
		}
	}
}

// GetRouteCount 获取路由数量
func (r *Router) GetRouteCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.routes)
}

// GetDomainCount 获取域名数量
func (r *Router) GetDomainCount() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.domains)
}
