package webapphost

import (
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// Marketplace 应用市场.
type Marketplace struct {
	mu         sync.RWMutex
	templates  *TemplateManager
	manager    *WebAppManager
	installed  map[string]*InstalledApp
	updates    map[string]*UpdateInfo
	categories map[string]*MarketCategory
}

// InstalledApp 已安装应用.
type InstalledApp struct {
	TemplateID  string    `json:"template_id"`
	AppID       string    `json:"app_id"`
	Version     string    `json:"version"`
	InstalledAt time.Time `json:"installed_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	AutoUpdate  bool      `json:"auto_update"`
}

// UpdateInfo 更新信息.
type UpdateInfo struct {
	TemplateID     string `json:"template_id"`
	CurrentVersion string `json:"current_version"`
	NewVersion     string `json:"new_version"`
	ReleaseNotes   string `json:"release_notes"`
	SizeMB         int64  `json:"size_mb"`
	Critical       bool   `json:"critical"`
}

// MarketSearchParams 市场搜索参数.
type MarketSearchParams struct {
	Query    string `json:"query"`
	Category string `json:"category"`
	Type     string `json:"type"`
	Official bool   `json:"official"`
	Featured bool   `json:"featured"`
	SortBy   string `json:"sort_by"` // name, downloads, rating, updated
	SortDesc bool   `json:"sort_desc"`
	Limit    int    `json:"limit"`
	Offset   int    `json:"offset"`
}

// NewMarketplace 创建应用市场.
func NewMarketplace(templates *TemplateManager, manager *WebAppManager) *Marketplace {
	return &Marketplace{
		templates:  templates,
		manager:    manager,
		installed:  make(map[string]*InstalledApp),
		updates:    make(map[string]*UpdateInfo),
		categories: make(map[string]*MarketCategory),
	}
}

// BrowseApps 浏览应用.
func (mp *Marketplace) BrowseApps(params *MarketSearchParams) []*MarketApp {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	// 获取所有模板
	listOpts := &TemplateListOptions{
		Category: params.Category,
		Type:     params.Type,
		Official: params.Official,
		Featured: params.Featured,
		Search:   params.Query,
		SortBy:   params.SortBy,
		SortDesc: params.SortDesc,
		Limit:    params.Limit,
		Offset:   params.Offset,
	}

	templates := mp.templates.ListTemplates(listOpts)

	// 转换为市场应用
	apps := make([]*MarketApp, 0, len(templates))
	for _, tmpl := range templates {
		app := &MarketApp{
			ID:          tmpl.ID,
			Name:        tmpl.Name,
			DisplayName: tmpl.DisplayName,
			Description: tmpl.Description,
			Category:    tmpl.Category,
			Icon:        tmpl.Icon,
			Version:     tmpl.Version,
			Author:      tmpl.Author,
			Type:        tmpl.Type,
			Tags:        tmpl.Tags,
			Rating:      tmpl.Rating,
			Downloads:   tmpl.Downloads,
			Official:    tmpl.Official,
			Featured:    tmpl.Featured,
			SizeMB:      tmpl.MinDisk,
		}

		// 检查是否已安装
		if installed, exists := mp.installed[tmpl.ID]; exists {
			app.Installed = true
			// 检查是否有更新
			if update, exists := mp.updates[tmpl.ID]; exists {
				app.UpdateAvail = update.NewVersion != installed.Version
			}
		}

		apps = append(apps, app)
	}

	return apps
}

// GetApp 获取应用详情.
func (mp *Marketplace) GetApp(templateID string) (*MarketApp, error) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	tmpl, err := mp.templates.GetTemplate(templateID)
	if err != nil {
		return nil, err
	}

	app := &MarketApp{
		ID:          tmpl.ID,
		Name:        tmpl.Name,
		DisplayName: tmpl.DisplayName,
		Description: tmpl.Description,
		Category:    tmpl.Category,
		Icon:        tmpl.Icon,
		Version:     tmpl.Version,
		Author:      tmpl.Author,
		Type:        tmpl.Type,
		Tags:        tmpl.Tags,
		Rating:      tmpl.Rating,
		Downloads:   tmpl.Downloads,
		Official:    tmpl.Official,
		Featured:    tmpl.Featured,
		SizeMB:      tmpl.MinDisk,
	}

	if installed, exists := mp.installed[tmpl.ID]; exists {
		app.Installed = true
		if update, exists := mp.updates[tmpl.ID]; exists {
			app.UpdateAvail = update.NewVersion != installed.Version
		}
	}

	return app, nil
}

// GetCategories 获取分类列表.
func (mp *Marketplace) GetCategories() []*MarketCategory {
	return mp.templates.ListCategories()
}

// InstallApp 安装应用.
func (mp *Marketplace) InstallApp(templateID string, config *DeployConfig) (*WebApp, error) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	// 检查是否已安装
	if _, exists := mp.installed[templateID]; exists {
		return nil, fmt.Errorf("app already installed: %s", templateID)
	}

	// 获取模板
	tmpl, err := mp.templates.GetTemplate(templateID)
	if err != nil {
		return nil, err
	}

	// 使用模板配置
	if config == nil {
		config = &DeployConfig{
			AppName:    tmpl.Name,
			TemplateID: templateID,
			Type:       tmpl.Type,
			Image:      tmpl.Image,
			Version:    tmpl.Version,
		}
	} else {
		config.TemplateID = templateID
		if config.Type == "" {
			config.Type = tmpl.Type
		}
		if config.Image == "" {
			config.Image = tmpl.Image
		}
		if config.Version == "" {
			config.Version = tmpl.Version
		}
	}

	// 创建部署器
	deployer := NewDeployer(mp.manager)

	// 部署应用
	app, err := deployer.Deploy(config)
	if err != nil {
		return nil, fmt.Errorf("failed to install app: %w", err)
	}

	// 记录安装信息
	mp.installed[templateID] = &InstalledApp{
		TemplateID:  templateID,
		AppID:       app.ID,
		Version:     tmpl.Version,
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
		AutoUpdate:  true,
	}

	log.Printf("App installed: %s (template: %s)", app.Name, templateID)
	return app, nil
}

// UninstallApp 卸载应用.
func (mp *Marketplace) UninstallApp(templateID string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	installed, exists := mp.installed[templateID]
	if !exists {
		return fmt.Errorf("app not installed: %s", templateID)
	}

	// 创建部署器
	deployer := NewDeployer(mp.manager)

	// 卸载应用
	if err := deployer.Undeploy(installed.AppID); err != nil {
		return fmt.Errorf("failed to uninstall app: %w", err)
	}

	// 删除安装记录
	delete(mp.installed, templateID)
	delete(mp.updates, templateID)

	log.Printf("App uninstalled: %s", templateID)
	return nil
}

// UpdateApp 更新应用.
func (mp *Marketplace) UpdateApp(templateID string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	installed, exists := mp.installed[templateID]
	if !exists {
		return fmt.Errorf("app not installed: %s", templateID)
	}

	update, exists := mp.updates[templateID]
	if !exists {
		return fmt.Errorf("no update available for: %s", templateID)
	}

	// 创建部署器
	deployer := NewDeployer(mp.manager)

	// 更新应用
	if err := deployer.UpdateApp(installed.AppID, update.NewVersion); err != nil {
		return fmt.Errorf("failed to update app: %w", err)
	}

	// 更新安装记录
	installed.Version = update.NewVersion
	installed.UpdatedAt = time.Now()

	// 删除更新记录
	delete(mp.updates, templateID)

	log.Printf("App updated: %s to version %s", templateID, update.NewVersion)
	return nil
}

// CheckUpdates 检查更新.
func (mp *Marketplace) CheckUpdates() []*UpdateInfo {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	updates := make([]*UpdateInfo, 0)

	for templateID, installed := range mp.installed {
		tmpl, err := mp.templates.GetTemplate(templateID)
		if err != nil {
			continue
		}

		// 检查是否有新版本（模拟）
		if tmpl.Version != installed.Version {
			update := &UpdateInfo{
				TemplateID:     templateID,
				CurrentVersion: installed.Version,
				NewVersion:     tmpl.Version,
				ReleaseNotes:   "Bug fixes and improvements",
				SizeMB:         10,
				Critical:       false,
			}
			mp.updates[templateID] = update
			updates = append(updates, update)
		}
	}

	return updates
}

// GetInstalledApps 获取已安装应用列表.
func (mp *Marketplace) GetInstalledApps() []*InstalledApp {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	apps := make([]*InstalledApp, 0, len(mp.installed))
	for _, app := range mp.installed {
		apps = append(apps, app)
	}

	sort.Slice(apps, func(i, j int) bool {
		return apps[i].InstalledAt.After(apps[j].InstalledAt)
	})

	return apps
}

// IsInstalled 检查应用是否已安装.
func (mp *Marketplace) IsInstalled(templateID string) bool {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	_, exists := mp.installed[templateID]
	return exists
}

// SetAutoUpdate 设置自动更新.
func (mp *Marketplace) SetAutoUpdate(templateID string, enabled bool) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	installed, exists := mp.installed[templateID]
	if !exists {
		return fmt.Errorf("app not installed: %s", templateID)
	}

	installed.AutoUpdate = enabled
	return nil
}

// GetMarketStats 获取市场统计.
func (mp *Marketplace) GetMarketStats() *MarketStats {
	mp.mu.RLock()
	defer mp.mu.RUnlock()

	stats := &MarketStats{
		TotalTemplates: mp.templates.GetTemplateCount(),
		InstalledApps:  len(mp.installed),
		PendingUpdates: len(mp.updates),
	}

	// 统计分类
	categories := mp.templates.ListCategories()
	for _, cat := range categories {
		stats.Categories = append(stats.Categories, cat.Name)
	}

	return stats
}

// MarketStats 市场统计.
type MarketStats struct {
	TotalTemplates int      `json:"total_templates"`
	InstalledApps  int      `json:"installed_apps"`
	PendingUpdates int      `json:"pending_updates"`
	Categories     []string `json:"categories"`
}

// GetFeaturedApps 获取推荐应用.
func (mp *Marketplace) GetFeaturedApps(limit int) []*MarketApp {
	return mp.BrowseApps(&MarketSearchParams{
		Featured: true,
		SortBy:   "downloads",
		SortDesc: true,
		Limit:    limit,
	})
}

// GetPopularApps 获取热门应用.
func (mp *Marketplace) GetPopularApps(limit int) []*MarketApp {
	return mp.BrowseApps(&MarketSearchParams{
		SortBy:   "downloads",
		SortDesc: true,
		Limit:    limit,
	})
}

// GetTopRatedApps 获取高评分应用.
func (mp *Marketplace) GetTopRatedApps(limit int) []*MarketApp {
	return mp.BrowseApps(&MarketSearchParams{
		SortBy:   "rating",
		SortDesc: true,
		Limit:    limit,
	})
}
