// Package web provides global search and quick navigation API.
package web

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// SearchCategory represents search result category.
type SearchCategory string

const (
	CategorySettings SearchCategory = "settings"
	CategoryFiles    SearchCategory = "files"
	CategoryApps     SearchCategory = "apps"
	CategoryUsers    SearchCategory = "users"
	CategoryShares   SearchCategory = "shares"
	CategoryLogs     SearchCategory = "logs"
)

// SearchItem represents a single search result.
type SearchItem struct {
	ID          string         `json:"id"`
	Title       string         `json:"title"`
	Description string         `json:"description"`
	Category    SearchCategory `json:"category"`
	Path        string         `json:"path"` // Navigation path
	Icon        string         `json:"icon"`
	Score       float64        `json:"score"` // Relevance score
	Tags        []string       `json:"tags"`
	LastUpdated time.Time      `json:"last_updated"`
}

// SearchHistory represents a user's search history entry.
type SearchHistoryEntry struct {
	Query     string    `json:"query"`
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count"` // Number of results
}

// QuickNavEntry represents a quick navigation shortcut.
type QuickNavEntry struct {
	ID         string    `json:"id"`
	Name       string    `json:"name"`
	Path       string    `json:"path"`
	Icon       string    `json:"icon"`
	Shortcuts  []string  `json:"shortcuts"` // Keyboard shortcuts
	Category   string    `json:"category"`
	UsageCount int       `json:"usage_count"`
	CreatedAt  time.Time `json:"created_at"`
}

// GlobalSearchConfig holds search configuration.
type GlobalSearchConfig struct {
	MaxResults      int    `json:"max_results"`
	EnableHistory   bool   `json:"enable_history"`
	HistoryMaxItems int    `json:"history_max_items"`
	IndexPath       string `json:"index_path"`
	SearchTimeoutMs int    `json:"search_timeout_ms"`
	DefaultLimit    int    `json:"default_limit"`    // 默认每页数量
	MaxLimit        int    `json:"max_limit"`        // 最大每页数量
	CacheResults    bool   `json:"cache_results"`    // 缓存搜索结果
	CacheExpirySec  int    `json:"cache_expiry_sec"` // 缓存过期时间(秒)
}

// GlobalSearchService provides global search functionality.
type GlobalSearchService struct {
	mu         sync.RWMutex
	config     *GlobalSearchConfig
	index      map[SearchCategory][]SearchItem
	history    []SearchHistoryEntry
	quickNav   []QuickNavEntry
	logger     *zap.Logger
	configPath string
}

// NewGlobalSearchService creates a new global search service.
func NewGlobalSearchService(configPath string, logger *zap.Logger) (*GlobalSearchService, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	config := &GlobalSearchConfig{
		MaxResults:      50,
		EnableHistory:   true,
		HistoryMaxItems: 100,
		IndexPath:       "/var/lib/nas-os/search-index",
		SearchTimeoutMs: 500,
		DefaultLimit:    20,
		MaxLimit:        100,
		CacheResults:    true,
		CacheExpirySec:  60,
	}

	s := &GlobalSearchService{
		config:     config,
		index:      make(map[SearchCategory][]SearchItem),
		history:    []SearchHistoryEntry{},
		quickNav:   []QuickNavEntry{},
		logger:     logger,
		configPath: configPath,
	}

	// Initialize default quick nav entries
	s.initDefaultQuickNav()

	if err := s.loadConfig(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return s, nil
}

// initDefaultQuickNav initializes default quick navigation entries.
func (s *GlobalSearchService) initDefaultQuickNav() {
	s.quickNav = []QuickNavEntry{
		{ID: "storage", Name: "存储管理", Path: "/storage", Icon: "storage", Shortcuts: []string{"S"}, Category: "main", UsageCount: 0},
		{ID: "shares", Name: "共享管理", Path: "/shares", Icon: "share", Shortcuts: []string{"H"}, Category: "main", UsageCount: 0},
		{ID: "users", Name: "用户管理", Path: "/users", Icon: "users", Shortcuts: []string{"U"}, Category: "main", UsageCount: 0},
		{ID: "network", Name: "网络设置", Path: "/network", Icon: "network", Shortcuts: []string{"N"}, Category: "main", UsageCount: 0},
		{ID: "apps", Name: "应用中心", Path: "/apps", Icon: "apps", Shortcuts: []string{"A"}, Category: "main", UsageCount: 0},
		{ID: "monitor", Name: "系统监控", Path: "/monitor", Icon: "monitor", Shortcuts: []string{"M"}, Category: "main", UsageCount: 0},
		{ID: "backup", Name: "备份管理", Path: "/backup", Icon: "backup", Shortcuts: []string{"B"}, Category: "main", UsageCount: 0},
		{ID: "security", Name: "安全设置", Path: "/security", Icon: "security", Shortcuts: []string{"Ctrl+S"}, Category: "main", UsageCount: 0},
	}
}

// SearchRequest 增强搜索请求（支持分页、排序）
type SearchRequest struct {
	Query      string           `json:"query"`      // 搜索关键词
	Categories []SearchCategory `json:"categories"` // 搜索类别
	Offset     int              `json:"offset"`     // 分页偏移
	Limit      int              `json:"limit"`      // 每页数量
	SortBy     string           `json:"sortBy"`     // 排序字段 (score, title, last_updated)
	SortDesc   bool             `json:"sortDesc"`   // 降序排序
	Fuzzy      bool             `json:"fuzzy"`      // 模糊搜索
	ExactMatch bool             `json:"exactMatch"` // 精确匹配
	Tags       []string         `json:"tags"`       // 标签过滤
}

// SearchResponse 增强搜索响应（支持分页统计）
type SearchResponse struct {
	Query       string         `json:"query"`
	Results     []SearchItem   `json:"results"`
	Total       int            `json:"total"`       // 总结果数
	Offset      int            `json:"offset"`      // 当前偏移
	Limit       int            `json:"limit"`       // 每页数量
	Truncated   bool           `json:"truncated"`   // 是否截断
	Took        int64          `json:"took"`        // 查询耗时(ms)
	Facets      map[string]int `json:"facets"`      // 分类统计
	Suggestions []string       `json:"suggestions"` // 搜索建议
}

// Search performs global search across all categories.
func (s *GlobalSearchService) Search(ctx context.Context, query string, categories []SearchCategory) ([]SearchItem, error) {
	req := SearchRequest{
		Query:      query,
		Categories: categories,
		Offset:     0,
		Limit:      s.config.MaxResults,
		SortBy:     "score",
		SortDesc:   true,
	}
	resp, err := s.SearchAdvanced(ctx, req)
	if err != nil {
		return nil, err
	}
	return resp.Results, nil
}

// SearchAdvanced 高级搜索（支持分页、排序、过滤）
func (s *GlobalSearchService) SearchAdvanced(ctx context.Context, req SearchRequest) (*SearchResponse, error) {
	startTime := time.Now()

	s.mu.RLock()
	defer s.mu.RUnlock()

	response := &SearchResponse{
		Query:       req.Query,
		Facets:      make(map[string]int),
		Suggestions: make([]string, 0),
	}

	if req.Query == "" {
		response.Results = []SearchItem{}
		return response, nil
	}

	// 设置默认值
	if req.Limit <= 0 {
		req.Limit = s.config.MaxResults
	}
	if req.Limit > 100 {
		req.Limit = 100 // 最大100条/页
	}
	if req.Offset < 0 {
		req.Offset = 0
	}

	query := strings.ToLower(req.Query)
	allResults := []SearchItem{}

	// Search in specified categories (or all if empty)
	searchCategories := req.Categories
	if len(searchCategories) == 0 {
		searchCategories = []SearchCategory{CategorySettings, CategoryFiles, CategoryApps, CategoryUsers, CategoryShares, CategoryLogs}
	}

	for _, cat := range searchCategories {
		items := s.index[cat]
		for _, item := range items {
			// 匹配计算
			var titleMatch, descMatch, tagMatch bool

			if req.ExactMatch {
				// 精确匹配
				titleMatch = strings.EqualFold(item.Title, req.Query)
				descMatch = strings.Contains(strings.ToLower(item.Description), query)
			} else if req.Fuzzy {
				// 模糊匹配 - 允许部分匹配
				titleMatch = strings.Contains(strings.ToLower(item.Title), query) ||
					len(query) >= 3 && strings.Contains(query, strings.ToLower(item.Title))
				descMatch = strings.Contains(strings.ToLower(item.Description), query)
			} else {
				// 默认匹配
				titleMatch = strings.Contains(strings.ToLower(item.Title), query)
				descMatch = strings.Contains(strings.ToLower(item.Description), query)
			}

			// 标签匹配
			for _, tag := range item.Tags {
				if strings.Contains(strings.ToLower(tag), query) {
					tagMatch = true
					break
				}
			}

			// 标签过滤
			if len(req.Tags) > 0 {
				tagFilterMatch := false
				for _, filterTag := range req.Tags {
					for _, itemTag := range item.Tags {
						if strings.EqualFold(itemTag, filterTag) {
							tagFilterMatch = true
							break
						}
					}
					if tagFilterMatch {
						break
					}
				}
				if !tagFilterMatch && (titleMatch || descMatch || tagMatch) {
					continue // 不符合标签过滤
				}
			}

			if titleMatch || descMatch || tagMatch {
				// Calculate relevance score
				score := 0.5
				if titleMatch {
					score += 0.3
					if strings.EqualFold(item.Title, req.Query) {
						score += 0.2 // 精确匹配加分
					}
				}
				if tagMatch {
					score += 0.2
				}
				if descMatch {
					score += 0.1
				}
				item.Score = score
				allResults = append(allResults, item)

				// 分类统计
				response.Facets[string(cat)]++
			}
		}
	}

	// 排序
	s.sortSearchItems(allResults, req.SortBy, req.SortDesc)

	response.Total = len(allResults)

	// 分页
	start := req.Offset
	end := req.Offset + req.Limit

	if start >= len(allResults) {
		response.Results = []SearchItem{}
		response.Truncated = false
	} else {
		if end > len(allResults) {
			end = len(allResults)
			response.Truncated = false
		} else {
			response.Truncated = end < len(allResults)
		}
		response.Results = allResults[start:end]
	}

	response.Offset = req.Offset
	response.Limit = req.Limit
	response.Took = time.Since(startTime).Milliseconds()

	// 生成搜索建议
	response.Suggestions = s.generateSuggestions(query, allResults)

	// Record search history
	if s.config.EnableHistory {
		s.recordHistory(req.Query, response.Total)
	}

	s.logger.Info("Global search completed",
		zap.String("query", req.Query),
		zap.Int("total", response.Total),
		zap.Int("returned", len(response.Results)),
		zap.Int64("tookMs", response.Took))

	return response, nil
}

// sortSearchItems 排序搜索结果
func (s *GlobalSearchService) sortSearchItems(items []SearchItem, sortBy string, desc bool) {
	if sortBy == "" {
		sortBy = "score"
	}

	// 使用冒泡排序（简化实现）
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			var less bool

			switch sortBy {
			case "score":
				less = items[i].Score < items[j].Score
			case "title":
				less = strings.ToLower(items[i].Title) < strings.ToLower(items[j].Title)
			case "last_updated":
				less = items[i].LastUpdated.Before(items[j].LastUpdated)
			default:
				less = items[i].Score < items[j].Score
			}

			// 降序时反转比较
			if desc {
				less = !less
			}

			if less {
				items[i], items[j] = items[j], items[i]
			}
		}
	}
}

// generateSuggestions 生成搜索建议
func (s *GlobalSearchService) generateSuggestions(query string, results []SearchItem) []string {
	suggestions := make([]string, 0)

	// 从结果标题提取建议
	for _, item := range results {
		titleLower := strings.ToLower(item.Title)
		if strings.HasPrefix(titleLower, query) && titleLower != query {
			suggestions = append(suggestions, item.Title)
		}
		if len(suggestions) >= 5 {
			break
		}
	}

	// 从历史记录提取建议
	s.mu.RLock()
	for _, entry := range s.history {
		if strings.HasPrefix(strings.ToLower(entry.Query), query) && entry.Query != query {
			suggestions = append(suggestions, entry.Query)
		}
		if len(suggestions) >= 10 {
			break
		}
	}
	s.mu.RUnlock()

	return suggestions
}

// recordHistory records a search query in history.
func (s *GlobalSearchService) recordHistory(query string, count int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := SearchHistoryEntry{
		Query:     query,
		Timestamp: time.Now(),
		Count:     count,
	}

	// Add to history
	s.history = append([]SearchHistoryEntry{entry}, s.history...)

	// Limit history size
	if len(s.history) > s.config.HistoryMaxItems {
		s.history = s.history[:s.config.HistoryMaxItems]
	}
}

// GetSearchHistory returns recent search history.
func (s *GlobalSearchService) GetSearchHistory(ctx context.Context, limit int) []SearchHistoryEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}
	return s.history[:limit]
}

// ClearSearchHistory clears search history.
func (s *GlobalSearchService) ClearSearchHistory(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.history = []SearchHistoryEntry{}
	s.logger.Info("Search history cleared")
	return s.saveConfig()
}

// GetQuickNav returns quick navigation entries.
func (s *GlobalSearchService) GetQuickNav(ctx context.Context) []QuickNavEntry {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.quickNav
}

// AddQuickNav adds a custom quick navigation entry.
func (s *GlobalSearchService) AddQuickNav(ctx context.Context, name, path, icon string, shortcuts []string) (*QuickNavEntry, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry := QuickNavEntry{
		ID:         uuid.New().String(),
		Name:       name,
		Path:       path,
		Icon:       icon,
		Shortcuts:  shortcuts,
		Category:   "custom",
		UsageCount: 0,
		CreatedAt:  time.Now(),
	}

	s.quickNav = append(s.quickNav, entry)
	s.logger.Info("Added quick navigation entry", zap.String("name", name))

	return &entry, s.saveConfig()
}

// RemoveQuickNav removes a quick navigation entry.
func (s *GlobalSearchService) RemoveQuickNav(ctx context.Context, entryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, entry := range s.quickNav {
		if entry.ID == entryID {
			s.quickNav = append(s.quickNav[:i], s.quickNav[i+1:]...)
			s.logger.Info("Removed quick navigation entry", zap.String("id", entryID))
			return s.saveConfig()
		}
	}

	return fmt.Errorf("entry %s not found", entryID)
}

// IncrementQuickNavUsage increments usage count for a quick nav entry.
func (s *GlobalSearchService) IncrementQuickNavUsage(ctx context.Context, entryID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i := range s.quickNav {
		if s.quickNav[i].ID == entryID {
			s.quickNav[i].UsageCount++
			return s.saveConfig()
		}
	}

	return fmt.Errorf("entry %s not found", entryID)
}

// RegisterIndexItem registers an item in the search index.
func (s *GlobalSearchService) RegisterIndexItem(ctx context.Context, category SearchCategory, item SearchItem) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.index[category] = append(s.index[category], item)
	s.logger.Debug("Registered search index item",
		zap.String("category", string(category)),
		zap.String("id", item.ID))

	return nil
}

// ClearIndex clears the search index for a category.
func (s *GlobalSearchService) ClearIndex(ctx context.Context, category SearchCategory) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.index[category] = []SearchItem{}
	s.logger.Info("Cleared search index", zap.String("category", string(category)))
	return nil
}

// RebuildIndex rebuilds the search index from system data.
func (s *GlobalSearchService) RebuildIndex(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear all indexes
	for cat := range s.index {
		s.index[cat] = []SearchItem{}
	}

	now := time.Now()
	s.index[CategorySettings] = []SearchItem{
		{ID: "settings-storage", Title: "存储设置", Description: "管理存储池、磁盘和配额", Category: CategorySettings, Path: "/storage", Icon: "storage", Tags: []string{"storage", "disk", "quota"}, LastUpdated: now},
		{ID: "settings-network", Title: "网络设置", Description: "管理网络接口和共享服务", Category: CategorySettings, Path: "/network", Icon: "network", Tags: []string{"network", "smb", "nfs"}, LastUpdated: now},
	}
	s.index[CategoryApps] = []SearchItem{
		{ID: "app-backup", Title: "备份", Description: "快照、复制和云同步", Category: CategoryApps, Path: "/backup", Icon: "backup", Tags: []string{"backup", "snapshot"}, LastUpdated: now},
		{ID: "app-monitor", Title: "监控", Description: "系统资源与活动监控", Category: CategoryApps, Path: "/monitor", Icon: "monitor", Tags: []string{"monitor", "metrics"}, LastUpdated: now},
	}

	s.logger.Info("Search index rebuilt")
	return s.saveConfig()
}

// GetSearchStats returns search statistics.
func (s *GlobalSearchService) GetSearchStats(ctx context.Context) map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	totalItems := 0
	categoryCounts := make(map[string]int)
	for cat, items := range s.index {
		totalItems += len(items)
		categoryCounts[string(cat)] = len(items)
	}

	return map[string]interface{}{
		"total_indexed_items": totalItems,
		"categories":          categoryCounts,
		"history_entries":     len(s.history),
		"quick_nav_entries":   len(s.quickNav),
	}
}

// loadConfig loads search service configuration.
func (s *GlobalSearchService) loadConfig() error {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return err
	}

	var cfg struct {
		Config   *GlobalSearchConfig  `json:"config"`
		History  []SearchHistoryEntry `json:"history"`
		QuickNav []QuickNavEntry      `json:"quick_nav"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Config != nil {
		s.config = cfg.Config
	}
	s.history = cfg.History
	if len(cfg.QuickNav) > 0 {
		s.quickNav = cfg.QuickNav
	}

	return nil
}

// saveConfig saves search service configuration.
func (s *GlobalSearchService) saveConfig() error {
	cfg := struct {
		Config   *GlobalSearchConfig  `json:"config"`
		History  []SearchHistoryEntry `json:"history"`
		QuickNav []QuickNavEntry      `json:"quick_nav"`
	}{
		Config:   s.config,
		History:  s.history,
		QuickNav: s.quickNav,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(s.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	return os.WriteFile(s.configPath, data, 0644)
}
