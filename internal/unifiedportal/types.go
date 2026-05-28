// Package unifiedportal 提供统一搜索门户功能
// 支持跨模块全局搜索、快捷操作、智能推荐
package unifiedportal

import (
	"fmt"
	"sync"
	"time"
)

// SearchResultType 搜索结果类型
type SearchResultType string

const (
	ResultTypeFile       SearchResultType = "file"
	ResultTypeApp        SearchResultType = "app"
	ResultTypeSetting    SearchResultType = "setting"
	ResultTypeUser       SearchResultType = "user"
	ResultTypeShare      SearchResultType = "share"
	ResultTypeContainer  SearchResultType = "container"
	ResultTypeVM         SearchResultType = "vm"
	ResultTypeSnapshot   SearchResultType = "snapshot"
)

// SearchResult 搜索结果
type SearchResult struct {
	ID          string           `json:"id"`
	Type        SearchResultType `json:"type"`
	Title       string           `json:"title"`
	Description string           `json:"description,omitempty"`
	Path        string           `json:"path,omitempty"`
	Icon        string           `json:"icon,omitempty"`
	Score       float64          `json:"score"`       // 相关性评分
	Action      string           `json:"action"`      // 快捷操作
	ActionURL   string           `json:"actionUrl"`   // 操作链接
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// SearchRequest 搜索请求
type SearchRequest struct {
	Query      string   `json:"query"`
	Types      []string `json:"types,omitempty"`      // 过滤类型
	Limit      int      `json:"limit,omitempty"`
	Offset     int      `json:"offset,omitempty"`
	Category   string   `json:"category,omitempty"`   // 搜索分类
}

// SearchResponse 搜索响应
type SearchResponse struct {
	Query       string          `json:"query"`
	TotalCount  int             `json:"totalCount"`
	Results     []*SearchResult `json:"results"`
	Suggestions []string        `json:"suggestions,omitempty"`
	Duration    time.Duration   `json:"duration"`
}

// QuickAction 快捷操作
type QuickAction struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Icon        string `json:"icon,omitempty"`
	Category    string `json:"category"`
	Action      string `json:"action"`
	Params      map[string]string `json:"params,omitempty"`
	Enabled     bool   `json:"enabled"`
}

// PortalConfig 门户配置
type PortalConfig struct {
	MaxResults       int      `json:"maxResults"`
	EnableSuggestions bool   `json:"enableSuggestions"`
	EnableQuickActions bool  `json:"enableQuickActions"`
	SearchCategories []string `json:"searchCategories"`
	ExcludeTypes     []string `json:"excludeTypes"`
}

// SearchProvider 搜索提供者接口
type SearchProvider interface {
	Name() string
	Search(query string, limit int) ([]*SearchResult, error)
}

// Manager 统一门户管理器
type Manager struct {
	mu          sync.RWMutex
	providers   map[string]SearchProvider
	actions     map[string]*QuickAction
	config      *PortalConfig
	recentSearches []string
}

// NewManager 创建门户管理器
func NewManager(config *PortalConfig) *Manager {
	if config == nil {
		config = &PortalConfig{
			MaxResults:        50,
			EnableSuggestions: true,
			EnableQuickActions: true,
		}
	}
	return &Manager{
		providers:      make(map[string]SearchProvider),
		actions:        make(map[string]*QuickAction),
		config:         config,
		recentSearches: make([]string, 0),
	}
}

// RegisterProvider 注册搜索提供者
func (m *Manager) RegisterProvider(provider SearchProvider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[provider.Name()] = provider
}

// Search 全局搜索
func (m *Manager) Search(req *SearchRequest) *SearchResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	start := time.Now()
	results := make([]*SearchResult, 0)

	if req.Limit <= 0 {
		req.Limit = m.config.MaxResults
	}

	// 搜索所有注册的提供者
	for _, provider := range m.providers {
		if len(req.Types) > 0 {
			skip := true
			for _, t := range req.Types {
				if t == provider.Name() {
					skip = false
					break
				}
			}
			if skip {
				continue
			}
		}

		providerResults, err := provider.Search(req.Query, req.Limit)
		if err != nil {
			continue
		}
		results = append(results, providerResults...)
	}

	// 按评分排序（简化实现）
	if len(results) > req.Limit {
		results = results[:req.Limit]
	}

	// 记录搜索历史
	m.recordSearch(req.Query)

	return &SearchResponse{
		Query:       req.Query,
		TotalCount:  len(results),
		Results:     results,
		Suggestions: m.getSuggestions(req.Query),
		Duration:    time.Since(start),
	}
}

// RegisterAction 注册快捷操作
func (m *Manager) RegisterAction(action *QuickAction) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.actions[action.ID] = action
}

// GetActions 获取所有快捷操作
func (m *Manager) GetActions() []*QuickAction {
	m.mu.RLock()
	defer m.mu.RUnlock()

	actions := make([]*QuickAction, 0, len(m.actions))
	for _, a := range m.actions {
		if a.Enabled {
			actions = append(actions, a)
		}
	}
	return actions
}

// GetRecentSearches 获取最近搜索
func (m *Manager) GetRecentSearches() []string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.recentSearches
}

func (m *Manager) recordSearch(query string) {
	if len(m.recentSearches) >= 20 {
		m.recentSearches = m.recentSearches[1:]
	}
	m.recentSearches = append(m.recentSearches, query)
}

func (m *Manager) getSuggestions(query string) []string {
	// 简单建议逻辑
	suggestions := []string{}
	for _, recent := range m.recentSearches {
		if len(recent) > len(query) && recent[:len(query)] == query {
			suggestions = append(suggestions, recent)
		}
	}
	if len(suggestions) > 5 {
		suggestions = suggestions[:5]
	}
	_ = fmt.Sprintf("search: %s", query)
	return suggestions
}
