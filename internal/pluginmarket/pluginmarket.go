package pluginmarket

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// PluginMarket 插件市场
type PluginMarket struct {
	mu          sync.RWMutex
	plugins     map[string]*Plugin
	installed   map[string]*Installation
	categories  map[string]*Category
	reviews     map[string][]*Review
	config      *Config
}

// Plugin 插件
type Plugin struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Version     string    `json:"version"`
	Author      string    `json:"author"`
	License     string    `json:"license"`
	Homepage    string    `json:"homepage"`
	Repository  string    `json:"repository"`
	Category    string    `json:"category"`
	Tags        []string  `json:"tags"`
	Icon        string    `json:"icon"`
	Screenshots []string  `json:"screenshots"`
	Downloads   int64     `json:"downloads"`
	Rating      float64   `json:"rating"`
	Size        int64     `json:"size"`
	MinVersion  string    `json:"min_version"`
	MaxVersion  string    `json:"max_version"`
	IsVerified  bool      `json:"is_verified"`
	IsFeatured  bool      `json:"is_featured"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Installation 安装记录
type Installation struct {
	PluginID    string    `json:"plugin_id"`
	Version     string    `json:"version"`
	Config      map[string]interface{} `json:"config"`
	IsEnabled   bool      `json:"is_enabled"`
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// Category 分类
type Category struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Icon        string    `json:"icon"`
	Count       int       `json:"count"`
}

// Review 评价
type Review struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	PluginID  string    `json:"plugin_id"`
	Rating    int       `json:"rating"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	CreatedAt time.Time `json:"created_at"`
}

// Config 配置
type Config struct {
	RepositoryURL   string `json:"repository_url"`
	AutoUpdate      bool   `json:"auto_update"`
	UpdateInterval  time.Duration `json:"update_interval"`
	MaxDownloads    int    `json:"max_downloads"`
	VerifyPlugins   bool   `json:"verify_plugins"`
	AllowBeta       bool   `json:"allow_beta"`
}

// NewPluginMarket 创建插件市场
func NewPluginMarket(config *Config) *PluginMarket {
	return &PluginMarket{
		plugins:    make(map[string]*Plugin),
		installed:  make(map[string]*Installation),
		categories: make(map[string]*Category),
		reviews:    make(map[string][]*Review),
		config:     config,
	}
}

// RegisterPlugin 注册插件
func (pm *PluginMarket) RegisterPlugin(ctx context.Context, plugin *Plugin) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	plugin.CreatedAt = time.Now()
	plugin.UpdatedAt = time.Now()
	pm.plugins[plugin.ID] = plugin
	return nil
}

// GetPlugin 获取插件
func (pm *PluginMarket) GetPlugin(ctx context.Context, id string) (*Plugin, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	plugin, exists := pm.plugins[id]
	if !exists {
		return nil, fmt.Errorf("plugin not found: %s", id)
	}
	return plugin, nil
}

// ListPlugins 列出插件
func (pm *PluginMarket) ListPlugins(ctx context.Context, category string, limit int) []*Plugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	var plugins []*Plugin
	for _, plugin := range pm.plugins {
		if category == "" || plugin.Category == category {
			plugins = append(plugins, plugin)
		}
	}
	
	// 按评分排序
	sortPluginsByRating(plugins)
	
	if len(plugins) > limit {
		return plugins[:limit]
	}
	return plugins
}

// SearchPlugins 搜索插件
func (pm *PluginMarket) SearchPlugins(ctx context.Context, query string, limit int) []*Plugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	var results []*Plugin
	for _, plugin := range pm.plugins {
		if matchPlugin(plugin, query) {
			results = append(results, plugin)
		}
	}
	
	// 按评分排序
	sortPluginsByRating(results)
	
	if len(results) > limit {
		return results[:limit]
	}
	return results
}

// InstallPlugin 安装插件
func (pm *PluginMarket) InstallPlugin(ctx context.Context, pluginID string, config map[string]interface{}) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	plugin, exists := pm.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}
	
	// 检查是否已安装
	if _, exists := pm.installed[pluginID]; exists {
		return fmt.Errorf("plugin already installed: %s", pluginID)
	}
	
	installation := &Installation{
		PluginID:    pluginID,
		Version:     plugin.Version,
		Config:      config,
		IsEnabled:   true,
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}
	
	pm.installed[pluginID] = installation
	
	// 更新下载计数
	plugin.Downloads++
	
	return nil
}

// UninstallPlugin 卸载插件
func (pm *PluginMarket) UninstallPlugin(ctx context.Context, pluginID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	if _, exists := pm.installed[pluginID]; !exists {
		return fmt.Errorf("plugin not installed: %s", pluginID)
	}
	
	delete(pm.installed, pluginID)
	return nil
}

// EnablePlugin 启用插件
func (pm *PluginMarket) EnablePlugin(ctx context.Context, pluginID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	installation, exists := pm.installed[pluginID]
	if !exists {
		return fmt.Errorf("plugin not installed: %s", pluginID)
	}
	
	installation.IsEnabled = true
	installation.UpdatedAt = time.Now()
	return nil
}

// DisablePlugin 禁用插件
func (pm *PluginMarket) DisablePlugin(ctx context.Context, pluginID string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	installation, exists := pm.installed[pluginID]
	if !exists {
		return fmt.Errorf("plugin not installed: %s", pluginID)
	}
	
	installation.IsEnabled = false
	installation.UpdatedAt = time.Now()
	return nil
}

// GetInstallation 获取安装信息
func (pm *PluginMarket) GetInstallation(ctx context.Context, pluginID string) (*Installation, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	installation, exists := pm.installed[pluginID]
	if !exists {
		return nil, fmt.Errorf("plugin not installed: %s", pluginID)
	}
	return installation, nil
}

// ListInstalled 列出已安装插件
func (pm *PluginMarket) ListInstalled(ctx context.Context) []*Installation {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	var installations []*Installation
	for _, installation := range pm.installed {
		installations = append(installations, installation)
	}
	return installations
}

// AddReview 添加评价
func (pm *PluginMarket) AddReview(ctx context.Context, review *Review) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	review.CreatedAt = time.Now()
	pm.reviews[review.PluginID] = append(pm.reviews[review.PluginID], review)
	
	// 更新插件评分
	pm.updatePluginRating(review.PluginID)
	
	return nil
}

// GetReviews 获取评价
func (pm *PluginMarket) GetReviews(ctx context.Context, pluginID string, limit int) []*Review {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	reviews := pm.reviews[pluginID]
	if len(reviews) > limit {
		return reviews[:limit]
	}
	return reviews
}

// AddCategory 添加分类
func (pm *PluginMarket) AddCategory(ctx context.Context, category *Category) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	pm.categories[category.ID] = category
	return nil
}

// ListCategories 列出分类
func (pm *PluginMarket) ListCategories(ctx context.Context) []*Category {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	var categories []*Category
	for _, category := range pm.categories {
		categories = append(categories, category)
	}
	return categories
}

// GetFeatured 获取推荐插件
func (pm *PluginMarket) GetFeatured(ctx context.Context, limit int) []*Plugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	var featured []*Plugin
	for _, plugin := range pm.plugins {
		if plugin.IsFeatured {
			featured = append(featured, plugin)
		}
	}
	
	if len(featured) > limit {
		return featured[:limit]
	}
	return featured
}

// GetPopular 获取热门插件
func (pm *PluginMarket) GetPopular(ctx context.Context, limit int) []*Plugin {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	var plugins []*Plugin
	for _, plugin := range pm.plugins {
		plugins = append(plugins, plugin)
	}
	
	// 按下载量排序
	sortPluginsByDownloads(plugins)
	
	if len(plugins) > limit {
		return plugins[:limit]
	}
	return plugins
}

// UpdatePlugin 更新插件
func (pm *PluginMarket) UpdatePlugin(ctx context.Context, pluginID, newVersion string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	
	plugin, exists := pm.plugins[pluginID]
	if !exists {
		return fmt.Errorf("plugin not found: %s", pluginID)
	}
	
	plugin.Version = newVersion
	plugin.UpdatedAt = time.Now()
	
	// 更新安装信息
	if installation, exists := pm.installed[pluginID]; exists {
		installation.Version = newVersion
		installation.UpdatedAt = time.Now()
	}
	
	return nil
}

// CheckUpdates 检查更新
func (pm *PluginMarket) CheckUpdates(ctx context.Context) map[string]string {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	updates := make(map[string]string)
	for pluginID, installation := range pm.installed {
		plugin, exists := pm.plugins[pluginID]
		if exists && plugin.Version != installation.Version {
			updates[pluginID] = plugin.Version
		}
	}
	return updates
}

// GetStats 获取统计信息
func (pm *PluginMarket) GetStats(ctx context.Context) map[string]interface{} {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	
	totalDownloads := int64(0)
	for _, plugin := range pm.plugins {
		totalDownloads += plugin.Downloads
	}
	
	return map[string]interface{}{
		"total_plugins":     len(pm.plugins),
		"installed_plugins": len(pm.installed),
		"total_categories":  len(pm.categories),
		"total_downloads":   totalDownloads,
	}
}

// 内部方法
func (pm *PluginMarket) updatePluginRating(pluginID string) {
	reviews := pm.reviews[pluginID]
	if len(reviews) == 0 {
		return
	}
	
	total := 0
	for _, review := range reviews {
		total += review.Rating
	}
	
	avgRating := float64(total) / float64(len(reviews))
	
	if plugin, exists := pm.plugins[pluginID]; exists {
		plugin.Rating = avgRating
	}
}

// 辅助函数
func matchPlugin(plugin *Plugin, query string) bool {
	// 检查名称
	if contains(plugin.Name, query) {
		return true
	}
	
	// 检查描述
	if contains(plugin.Description, query) {
		return true
	}
	
	// 检查标签
	for _, tag := range plugin.Tags {
		if contains(tag, query) {
			return true
		}
	}
	
	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0)
}

func sortPluginsByRating(plugins []*Plugin) {
	for i := 0; i < len(plugins); i++ {
		for j := i + 1; j < len(plugins); j++ {
			if plugins[i].Rating < plugins[j].Rating {
				plugins[i], plugins[j] = plugins[j], plugins[i]
			}
		}
	}
}

func sortPluginsByDownloads(plugins []*Plugin) {
	for i := 0; i < len(plugins); i++ {
		for j := i + 1; j < len(plugins); j++ {
			if plugins[i].Downloads < plugins[j].Downloads {
				plugins[i], plugins[j] = plugins[j], plugins[i]
			}
		}
	}
}
