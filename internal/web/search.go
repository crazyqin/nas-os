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
	ID          string        `json:"id"`
	Title       string        `json:"title"`
	Description string        `json:"description"`
	Category    SearchCategory `json:"category"`
	Path        string        `json:"path"`          // Navigation path
	Icon        string        `json:"icon"`
	Score       float64       `json:"score"`         // Relevance score
	Tags        []string      `json:"tags"`
	LastUpdated time.Time     `json:"last_updated"`
}

// SearchHistory represents a user's search history entry.
type SearchHistoryEntry struct {
	Query     string    `json:"query"`
	Timestamp time.Time `json:"timestamp"`
	Count     int       `json:"count"`          // Number of results
}

// QuickNavEntry represents a quick navigation shortcut.
type QuickNavEntry struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Path        string    `json:"path"`
	Icon        string    `json:"icon"`
	Shortcuts   []string  `json:"shortcuts"`     // Keyboard shortcuts
	Category    string    `json:"category"`
	UsageCount  int       `json:"usage_count"`
	CreatedAt   time.Time `json:"created_at"`
}

// GlobalSearchConfig holds search configuration.
type GlobalSearchConfig struct {
	MaxResults       int       `json:"max_results"`
	EnableHistory    bool      `json:"enable_history"`
	HistoryMaxItems  int       `json:"history_max_items"`
	IndexPath        string    `json:"index_path"`
	SearchTimeoutMs  int       `json:"search_timeout_ms"`
}

// GlobalSearchService provides global search functionality.
type GlobalSearchService struct {
	mu           sync.RWMutex
	config       *GlobalSearchConfig
	index        map[SearchCategory][]SearchItem
	history      []SearchHistoryEntry
	quickNav     []QuickNavEntry
	logger       *zap.Logger
	configPath   string
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

// Search performs global search across all categories.
func (s *GlobalSearchService) Search(ctx context.Context, query string, categories []SearchCategory) ([]SearchItem, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if query == "" {
		return []SearchItem{}, nil
	}

	query = strings.ToLower(query)
	results := []SearchItem{}

	// Search in specified categories (or all if empty)
	searchCategories := categories
	if len(searchCategories) == 0 {
		searchCategories = []SearchCategory{CategorySettings, CategoryFiles, CategoryApps, CategoryUsers, CategoryShares}
	}

	for _, cat := range searchCategories {
		items := s.index[cat]
		for _, item := range items {
			// Simple matching: title or description contains query
			titleMatch := strings.Contains(strings.ToLower(item.Title), query)
			descMatch := strings.Contains(strings.ToLower(item.Description), query)
			tagMatch := false
			for _, tag := range item.Tags {
				if strings.Contains(strings.ToLower(tag), query) {
					tagMatch = true
					break
				}
			}

			if titleMatch || descMatch || tagMatch {
				// Calculate relevance score
				score := 0.5
				if titleMatch {
					score += 0.3
				}
				if tagMatch {
					score += 0.2
				}
				item.Score = score
				results = append(results, item)
			}
		}
	}

	// Sort by score (descending) and limit results
	if len(results) > s.config.MaxResults {
		results = results[:s.config.MaxResults]
	}

	// Record search history
	if s.config.EnableHistory {
		s.recordHistory(query, len(results))
	}

	s.logger.Info("Global search completed",
		zap.String("query", query),
		zap.Int("results", len(results)))

	return results, nil
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
		ID:        uuid.New().String(),
		Name:      name,
		Path:      path,
		Icon:      icon,
		Shortcuts: shortcuts,
		Category:  "custom",
		UsageCount: 0,
		CreatedAt: time.Now(),
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

	// TODO: Rebuild from actual system data
	// This would scan settings, files, apps, etc.

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
		Config  *GlobalSearchConfig `json:"config"`
		History []SearchHistoryEntry `json:"history"`
		QuickNav []QuickNavEntry     `json:"quick_nav"`
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
		Config  *GlobalSearchConfig `json:"config"`
		History []SearchHistoryEntry `json:"history"`
		QuickNav []QuickNavEntry     `json:"quick_nav"`
	}{
		Config:  s.config,
		History: s.history,
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