package appstore

import (
	"fmt"
	"sync"
	"time"
)

// App represents an application in the store
type App struct {
	ID            string    `json:"id"`
	Name          string    `json:"name"`
	Description   string    `json:"description"`
	Version       string    `json:"version"`
	Author        string    `json:"author"`
	Category      string    `json:"category"`
	Icon          string    `json:"icon"`
	Screenshots   []string  `json:"screenshots"`
	DownloadURL   string    `json:"download_url"`
	Homepage      string    `json:"homepage"`
	License       string    `json:"license"`
	Size          int64     `json:"size"`
	Downloads     int       `json:"downloads"`
	Rating        float64   `json:"rating"`
	RatingCount   int       `json:"rating_count"`
	Tags          []string  `json:"tags"`
	Requirements  []string  `json:"requirements"`
	IsInstalled   bool      `json:"is_installed"`
	IsOfficial    bool      `json:"is_official"`
	IsVerified    bool      `json:"is_verified"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

// InstalledApp represents an installed application
type InstalledApp struct {
	AppID        string    `json:"app_id"`
	Name         string    `json:"name"`
	Version      string    `json:"version"`
	InstalledAt  time.Time `json:"installed_at"`
	UpdatedAt    time.Time `json:"updated_at"`
	Status       string    `json:"status"`
	ConfigPath   string    `json:"config_path"`
	DataPath     string    `json:"data_path"`
	Port         int       `json:"port"`
	IsRunning    bool      `json:"is_running"`
	AutoStart    bool      `json:"auto_start"`
}

// AppSearchRequest represents an app search request
type AppSearchRequest struct {
	Query    string   `json:"query"`
	Category string   `json:"category"`
	Tags     []string `json:"tags"`
	SortBy   string   `json:"sort_by"`
	Page     int      `json:"page"`
	PageSize int      `json:"pageSize"`
}

// AppSearchResult represents app search results
type AppSearchResult struct {
	Apps     []App `json:"apps"`
	Total    int   `json:"total"`
	Page     int   `json:"page"`
	PageSize int   `json:"pageSize"`
	HasMore  bool  `json:"has_more"`
}

// InstallRequest represents an app install request
type InstallRequest struct {
	AppID       string            `json:"app_id"`
	Version     string            `json:"version"`
	Config      map[string]string `json:"config"`
	AutoStart   bool              `json:"auto_start"`
}

// InstallStatus represents app installation status
type InstallStatus struct {
	ID          string    `json:"id"`
	AppID       string    `json:"app_id"`
	Status      string    `json:"status"`
	Progress    float64   `json:"progress"`
	StartedAt   time.Time `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Error       string    `json:"error,omitempty"`
}

// AppStats represents app store statistics
type AppStats struct {
	TotalApps      int     `json:"total_apps"`
	InstalledApps  int     `json:"installed_apps"`
	AvailableApps  int     `json:"available_apps"`
	TotalDownloads int     `json:"total_downloads"`
	AvgRating      float64 `json:"avg_rating"`
}

// Manager manages the app store
type Manager struct {
	mu         sync.RWMutex
	apps       map[string]*App
	installed  map[string]*InstalledApp
	installs   map[string]*InstallStatus
	categories []string
}

// NewManager creates a new app store manager
func NewManager() *Manager {
	return &Manager{
		apps:      make(map[string]*App),
		installed: make(map[string]*InstalledApp),
		installs:  make(map[string]*InstallStatus),
		categories: []string{
			"productivity", "media", "network", "storage",
			"security", "development", "utilities", "games",
		},
	}
}

// SearchApps searches for apps
func (m *Manager) SearchApps(req *AppSearchRequest) *AppSearchResult {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var results []App
	for _, app := range m.apps {
		if m.matchesSearch(app, req) {
			results = append(results, *app)
		}
	}

	total := len(results)
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	return &AppSearchResult{
		Apps:     results[start:end],
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
		HasMore:  end < total,
	}
}

func (m *Manager) matchesSearch(app *App, req *AppSearchRequest) bool {
	if req.Query != "" {
		query := req.Query
		if !contains(app.Name, query) && !contains(app.Description, query) {
			return false
		}
	}

	if req.Category != "" && app.Category != req.Category {
		return false
	}

	return true
}

func contains(s, substr string) bool {
	return len(substr) == 0 || len(s) >= len(substr) && (s == substr || len(s) > 0 && (s[0] == substr[0] && contains(s[1:], substr[1:])))
}

// GetApp returns an app by ID
func (m *Manager) GetApp(id string) (*App, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	app, ok := m.apps[id]
	return app, ok
}

// InstallApp installs an app
func (m *Manager) InstallApp(req *InstallRequest) (*InstallStatus, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	app, ok := m.apps[req.AppID]
	if !ok {
		return nil, fmt.Errorf("app not found: %s", req.AppID)
	}

	if _, installed := m.installed[req.AppID]; installed {
		return nil, fmt.Errorf("app already installed: %s", req.AppID)
	}

	now := time.Now()
	status := &InstallStatus{
		ID:        fmt.Sprintf("install-%d", now.UnixNano()),
		AppID:     req.AppID,
		Status:    "installing",
		StartedAt: now,
	}
	m.installs[status.ID] = status

	go m.processInstall(status, app, req)

	return status, nil
}

func (m *Manager) processInstall(status *InstallStatus, app *App, req *InstallRequest) {
	time.Sleep(2 * time.Second)

	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	status.Status = "completed"
	status.Progress = 100
	status.CompletedAt = &now

	m.installed[req.AppID] = &InstalledApp{
		AppID:       req.AppID,
		Name:        app.Name,
		Version:     app.Version,
		InstalledAt: now,
		UpdatedAt:   now,
		Status:      "running",
		AutoStart:   req.AutoStart,
	}
}

// UninstallApp uninstalls an app
func (m *Manager) UninstallApp(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.installed[id]; !ok {
		return fmt.Errorf("app not installed: %s", id)
	}

	delete(m.installed, id)
	return nil
}

// GetInstalledApps returns all installed apps
func (m *Manager) GetInstalledApps() []*InstalledApp {
	m.mu.RLock()
	defer m.mu.RUnlock()

	apps := make([]*InstalledApp, 0, len(m.installed))
	for _, app := range m.installed {
		apps = append(apps, app)
	}
	return apps
}

// GetInstallStatus returns installation status
func (m *Manager) GetInstallStatus(id string) (*InstallStatus, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	status, ok := m.installs[id]
	return status, ok
}

// GetCategories returns app categories
func (m *Manager) GetCategories() []string {
	return m.categories
}

// GetStats returns app store statistics
func (m *Manager) GetStats() *AppStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &AppStats{
		TotalApps:     len(m.apps),
		InstalledApps: len(m.installed),
	}

	for _, app := range m.apps {
		stats.TotalDownloads += app.Downloads
	}

	return stats
}
