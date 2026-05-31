package appstore

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// AppStatus 应用状态
type AppStatus string

const (
	AppStatusInstalled   AppStatus = "installed"
	AppStatusAvailable  AppStatus = "available"
	AppStatusUpdating   AppStatus = "updating"
	AppStatusError      AppStatus = "error"
)

// AppCategory 应用分类
type AppCategory string

const (
	CategoryMedia     AppCategory = "media"
	CategoryProductivity AppCategory = "productivity"
	CategoryDevelopment AppCategory = "development"
	CategoryNetwork   AppCategory = "network"
	CategorySecurity  AppCategory = "security"
	CategoryStorage   AppCategory = "storage"
	CategoryUtility   AppCategory = "utility"
)

// App 应用
type App struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Version     string      `json:"version"`
	Category    AppCategory `json:"category"`
	Icon        string      `json:"icon,omitempty"`
	Author      string      `json:"author"`
	License     string      `json:"license,omitempty"`
	Homepage    string      `json:"homepage,omitempty"`
	Repository  string      `json:"repository,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	Status      AppStatus   `json:"status"`
	Installed   bool        `json:"installed"`
	InstalledAt *time.Time  `json:"installed_at,omitempty"`
	Config      map[string]any `json:"config,omitempty"`
}

// AppStore 应用商店
type AppStore struct {
	mu      sync.RWMutex
	apps    map[string]*App
	updates map[string]string // appID -> latestVersion
}

// NewAppStore 创建应用商店
func NewAppStore() *AppStore {
	store := &AppStore{
		apps:    make(map[string]*App),
		updates: make(map[string]string),
	}
	
	store.initDefaultApps()
	return store
}

func (s *AppStore) initDefaultApps() {
	defaultApps := []*App{
		{
			ID:          "plex",
			Name:        "Plex Media Server",
			Description: "流媒体服务器，支持多种格式",
			Version:     "1.32.0",
			Category:    CategoryMedia,
			Author:      "Plex",
		},
		{
			ID:          "nextcloud",
			Name:        "Nextcloud",
			Description: "私有云存储和协作平台",
			Version:     "28.0.0",
			Category:    CategoryProductivity,
			Author:      "Nextcloud GmbH",
		},
		{
			ID:          "homeassistant",
			Name:        "Home Assistant",
			Description: "智能家居自动化平台",
			Version:     "2024.1.0",
			Category:    CategoryUtility,
			Author:      "Home Assistant",
		},
		{
			ID:          "pihole",
			Name:        "Pi-hole",
			Description: "DNS广告拦截",
			Version:     "5.17.0",
			Category:    CategoryNetwork,
			Author:      "Pi-hole",
		},
		{
			ID:          "gitea",
			Name:        "Gitea",
			Description: "轻量级Git服务",
			Version:     "1.21.0",
			Category:    CategoryDevelopment,
			Author:      "Gitea",
		},
	}
	
	for _, app := range defaultApps {
		app.Status = AppStatusAvailable
		s.apps[app.ID] = app
	}
}

// ListApps 列出应用
func (s *AppStore) ListApps(ctx context.Context, category AppCategory) []*App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []*App
	for _, app := range s.apps {
		if category != "" && app.Category != category {
			continue
		}
		result = append(result, app)
	}
	return result
}

// GetApp 获取应用
func (s *AppStore) GetApp(ctx context.Context, id string) (*App, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	app, ok := s.apps[id]
	if !ok {
		return nil, fmt.Errorf("app %s not found", id)
	}
	return app, nil
}

// InstallApp 安装应用
func (s *AppStore) InstallApp(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	app, ok := s.apps[id]
	if !ok {
		return fmt.Errorf("app %s not found", id)
	}
	
	app.Installed = true
	app.Status = AppStatusInstalled
	now := time.Now()
	app.InstalledAt = &now
	
	return nil
}

// UninstallApp 卸载应用
func (s *AppStore) UninstallApp(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	app, ok := s.apps[id]
	if !ok {
		return fmt.Errorf("app %s not found", id)
	}
	
	app.Installed = false
	app.Status = AppStatusAvailable
	app.InstalledAt = nil
	
	return nil
}

// UpdateApp 更新应用
func (s *AppStore) UpdateApp(ctx context.Context, id, version string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	app, ok := s.apps[id]
	if !ok {
		return fmt.Errorf("app %s not found", id)
	}
	
	app.Version = version
	app.Status = AppStatusInstalled
	
	return nil
}

// SearchApps 搜索应用
func (s *AppStore) SearchApps(ctx context.Context, query string) []*App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []*App
	for _, app := range s.apps {
		if contains(app.Name, query) || contains(app.Description, query) {
			result = append(result, app)
		}
	}
	return result
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && len(substr) > 0)
}
