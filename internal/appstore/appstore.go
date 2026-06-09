// Package appstore implements an application store with simplified deployment
// inspired by TrueNAS's Docker Compose-based app system. It provides one-click
// app installation, management, and marketplace functionality.
package appstore

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// AppCategory defines the category of an application
type AppCategory string

const (
	CategoryMedia      AppCategory = "media"
	CategoryStorage    AppCategory = "storage"
	CategoryNetwork    AppCategory = "network"
	CategorySecurity   AppCategory = "security"
	CategoryProductivity AppCategory = "productivity"
	CategoryDevelopment AppCategory = "development"
	CategoryHome       AppCategory = "home"
	CategoryAI         AppCategory = "ai"
	CategoryDatabase   AppCategory = "database"
	CategoryMonitoring AppCategory = "monitoring"
)

// AppStatus represents the status of an installed application
type AppStatus string

const (
	AppInstalling  AppStatus = "installing"
	AppRunning     AppStatus = "running"
	AppStopped     AppStatus = "stopped"
	AppUpdating    AppStatus = "updating"
	AppRemoving    AppStatus = "removing"
	AppError       AppStatus = "error"
)

// InstallStatus represents the status of an installation
type InstallStatus string

const (
	InstallPending   InstallStatus = "pending"
	InstallPulling   InstallStatus = "pulling"
	InstallConfiguring InstallStatus = "configuring"
	InstallStarting  InstallStatus = "starting"
	InstallComplete  InstallStatus = "complete"
	InstallFailed    InstallStatus = "failed"
)

// App represents an application in the store
type App struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Title       string      `json:"title"`
	Description string      `json:"description"`
	Category    AppCategory `json:"category"`
	Version     string      `json:"version"`
	Author      string      `json:"author"`
	Website     string      `json:"website,omitempty"`
	Icon        string      `json:"icon,omitempty"`
	Tags        []string    `json:"tags,omitempty"`
	Stars       float64     `json:"stars"`
	Downloads   int64       `json:"downloads"`
	Featured    bool        `json:"featured"`
	Verified    bool        `json:"verified"`
	License     string      `json:"license"`
	MinMemory   int         `json:"min_memory"`   // MB
	MinDisk     int         `json:"min_disk"`     // MB
	Ports       []PortMapping `json:"ports"`
	Volumes     []VolumeMapping `json:"volumes"`
	EnvVars     []EnvVariable `json:"env_vars"`
	Compose          string           `json:"compose"` // Docker Compose YAML
	ComposeTemplate  *ComposeTemplate `json:"compose_template,omitempty"`
	HealthCheck      *HealthCheckConfig `json:"health_check,omitempty"`
	Dependencies     []AppDependency  `json:"dependencies,omitempty"`
	UpdatedAt        time.Time        `json:"updated_at"`
}

// InstalledApp represents an installed application instance
type InstalledApp struct {
	ID          string      `json:"id"`
	AppID       string      `json:"app_id"`
	Name        string      `json:"name"`
	Status      AppStatus   `json:"status"`
	Version     string      `json:"version"`
	ContainerID string      `json:"container_id,omitempty"`
	IPAddress   string      `json:"ip_address,omitempty"`
	Ports       []PortMapping `json:"ports"`
	Volumes     []VolumeMapping `json:"volumes"`
	EnvVars     map[string]string `json:"env_vars"`
	InstalledAt time.Time   `json:"installed_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	ResourceUsage ResourceUsage   `json:"resource_usage"`
	HealthStatus  AppHealthStatus `json:"health_status"`
	HealthSince   *time.Time      `json:"health_since,omitempty"`
	Logs          []LogEntry      `json:"logs,omitempty"`
}

// PortMapping represents a port mapping
type PortMapping struct {
	Name       string `json:"name,omitempty"`
	Container  int    `json:"container"`
	Host       int    `json:"host"`
	Protocol   string `json:"protocol"` // tcp, udp
}

// VolumeMapping represents a volume mapping
type VolumeMapping struct {
	Name      string `json:"name,omitempty"`
	Container string `json:"container"`
	Host      string `json:"host"`
	ReadOnly  bool   `json:"read_only"`
}

// EnvVariable represents an environment variable
type EnvVariable struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Default     string `json:"default,omitempty"`
	Required    bool   `json:"required"`
	Type        string `json:"type"` // string, number, boolean, password
}

// LogEntry represents a log entry
type LogEntry struct {
	Timestamp time.Time `json:"timestamp"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
}

// HealthCheckConfig defines application health check configuration
type HealthCheckConfig struct {
	Type        string        `json:"type"`            // http, tcp, command
	URL         string        `json:"url,omitempty"`   // for http type
	Port        int           `json:"port,omitempty"`  // for tcp type
	Command     []string      `json:"command,omitempty"` // for command type
	Interval    time.Duration `json:"interval"`        // check interval
	Timeout     time.Duration `json:"timeout"`         // per-check timeout
	Retries     int           `json:"retries"`         // failures before unhealthy
	StartPeriod time.Duration `json:"start_period"`    // grace period after start
}

// AppDependency represents an application dependency
type AppDependency struct {
	AppID    string `json:"app_id"`
	Version  string `json:"version,omitempty"` // version constraint (semver)
	Required bool   `json:"required"`          // hard dependency or optional
	Reason   string `json:"reason,omitempty"`  // why this dependency is needed
}

// ComposeService represents a single service in a Docker Compose template
type ComposeService struct {
	Name          string             `json:"name"`
	Image         string             `json:"image"`
	Ports         []PortMapping      `json:"ports,omitempty"`
	Volumes       []VolumeMapping    `json:"volumes,omitempty"`
	EnvVars       map[string]string  `json:"env_vars,omitempty"`
	RestartPolicy string             `json:"restart_policy,omitempty"`
	NetworkMode   string             `json:"network_mode,omitempty"`
	DependsOn     []string           `json:"depends_on,omitempty"`
	Privileged    bool               `json:"privileged,omitempty"`
	HealthCheck   *HealthCheckConfig `json:"health_check,omitempty"`
}

// ComposeTemplate defines a Docker Compose application template
type ComposeTemplate struct {
	Version  string                   `json:"version,omitempty"`
	Services []ComposeService         `json:"services"`
	Networks map[string]NetworkConfig `json:"networks,omitempty"`
	Volumes  map[string]VolumeConfig  `json:"volumes,omitempty"`
}

// NetworkConfig defines a Docker Compose network
type NetworkConfig struct {
	Driver   string            `json:"driver,omitempty"`
	External bool              `json:"external,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// VolumeConfig defines a Docker Compose named volume
type VolumeConfig struct {
	Driver   string            `json:"driver,omitempty"`
	External bool              `json:"external,omitempty"`
	Labels   map[string]string `json:"labels,omitempty"`
}

// AppHealthStatus represents the health status of an installed application
type AppHealthStatus string

const (
	HealthHealthy   AppHealthStatus = "healthy"
	HealthUnhealthy AppHealthStatus = "unhealthy"
	HealthStarting  AppHealthStatus = "starting"
	HealthUnknown   AppHealthStatus = "unknown"
)

// AppHealthCheckResult represents the result of a health check
type AppHealthCheckResult struct {
	InstalledID string          `json:"installed_id"`
	Status      AppHealthStatus `json:"status"`
	Message     string          `json:"message,omitempty"`
	CheckedAt   time.Time       `json:"checked_at"`
}

// AppStoreConfig contains app store configuration
type AppStoreConfig struct {
	RegistryURL     string `json:"registry_url"`
	CacheDir        string `json:"cache_dir"`
	MaxConcurrent   int    `json:"max_concurrent"`
	AutoUpdate      bool   `json:"auto_update"`
	UpdateInterval  time.Duration `json:"update_interval"`
	EnableGPU       bool   `json:"enable_gpu"`
	DefaultNetwork  string `json:"default_network"`
}

// AppStore is the main app store service
type AppStore struct {
	mu          sync.RWMutex
	config      AppStoreConfig
	apps        map[string]*App            // Available apps
	installed   map[string]*InstalledApp   // Installed apps
	catalogs    []AppCatalog               // App catalogs
	ctx         context.Context
	cancel      context.CancelFunc
	installChan chan *InstallRequest
}

// AppCatalog represents a source of applications
type AppCatalog struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	URL         string    `json:"url"`
	Enabled     bool      `json:"enabled"`
	LastSync    time.Time `json:"last_sync"`
	AppCount    int       `json:"app_count"`
}

// InstallRequest represents an app installation request
type InstallRequest struct {
	AppID     string            `json:"app_id"`
	Name      string            `json:"name"`
	EnvVars   map[string]string `json:"env_vars"`
	Ports     []PortMapping     `json:"ports"`
	Volumes   []VolumeMapping   `json:"volumes"`
	Status    InstallStatus     `json:"status"`
	Error     string            `json:"error,omitempty"`
	CreatedAt time.Time         `json:"created_at"`
}

// NewAppStore creates a new app store service
func NewAppStore(config AppStoreConfig) *AppStore {
	ctx, cancel := context.WithCancel(context.Background())
	
	service := &AppStore{
		config:      config,
		apps:        make(map[string]*App),
		installed:   make(map[string]*InstalledApp),
		catalogs:    make([]AppCatalog, 0),
		ctx:         ctx,
		cancel:      cancel,
		installChan: make(chan *InstallRequest, 100),
	}
	
	return service
}

// Start begins the app store service
func (s *AppStore) Start() error {
	log.Println("[AppStore] Starting application store service")
	
	// Add default catalogs
	s.addDefaultCatalogs()
	
	// Register builtin apps synchronously
	s.registerBuiltinApps()
	
	// Sync catalogs
	go s.syncCatalogs()
	
	// Start installer
	go s.processInstalls()
	
	// Start resource monitor
	go s.monitorResources()
	
	if s.config.AutoUpdate {
		go s.autoUpdater()
	}
	
	log.Println("[AppStore] Service started successfully")
	return nil
}

// Stop gracefully stops the service
func (s *AppStore) Stop() error {
	s.cancel()
	log.Println("[AppStore] Service stopped")
	return nil
}

// addDefaultCatalogs adds default app catalogs
func (s *AppStore) addDefaultCatalogs() {
	s.catalogs = append(s.catalogs,
		AppCatalog{
			ID:       "official",
			Name:     "Official Apps",
			URL:      "https://apps.nas-os.io/official",
			Enabled:  true,
			AppCount: 50,
		},
		AppCatalog{
			ID:       "community",
			Name:     "Community Apps",
			URL:      "https://apps.nas-os.io/community",
			Enabled:  true,
			AppCount: 200,
		},
	)
}

// syncCatalogs syncs app catalogs
func (s *AppStore) syncCatalogs() {
	ticker := time.NewTicker(s.config.UpdateInterval)
	defer ticker.Stop()
	
	// Initial sync
	s.doSync()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.doSync()
		}
	}
}

// doSync performs the actual sync
func (s *AppStore) doSync() {
	log.Println("[AppStore] Syncing app catalogs...")
	
	// Add some popular apps
	s.registerBuiltinApps()
	
	s.mu.Lock()
	for i := range s.catalogs {
		s.catalogs[i].LastSync = time.Now()
	}
	s.mu.Unlock()
}

// registerBuiltinApps registers built-in applications
func (s *AppStore) registerBuiltinApps() {
	apps := []*App{
		{
			ID:          "jellyfin",
			Name:        "jellyfin",
			Title:       "Jellyfin",
			Description: "Free media server for streaming movies, TV shows, and music",
			Category:    CategoryMedia,
			Version:     "10.9.0",
			Author:      "Jellyfin Project",
			Website:     "https://jellyfin.org",
			Tags:        []string{"media", "streaming", "video"},
			Stars:       4.8,
			Downloads:   1000000,
			Featured:    true,
			Verified:    true,
			License:     "GPL-2.0",
			MinMemory:   512,
			MinDisk:     1024,
			Ports: []PortMapping{
				{Name: "web", Container: 8096, Host: 8096, Protocol: "tcp"},
			},
			Volumes: []VolumeMapping{
				{Name: "config", Container: "/config", Host: "/opt/jellyfin/config"},
				{Name: "media", Container: "/media", Host: "/media"},
			},
			HealthCheck: &HealthCheckConfig{
				Type:        "http",
				URL:         "/health",
				Port:        8096,
				Interval:    30 * time.Second,
				Timeout:     5 * time.Second,
				Retries:     3,
				StartPeriod: 60 * time.Second,
			},
			ComposeTemplate: &ComposeTemplate{
				Version: "3.8",
				Services: []ComposeService{
					{
						Name:  "jellyfin",
						Image: "jellyfin/jellyfin:10.9.0",
						Ports: []PortMapping{{Name: "web", Container: 8096, Host: 8096, Protocol: "tcp"}},
						Volumes: []VolumeMapping{
							{Name: "config", Container: "/config", Host: "/opt/jellyfin/config"},
							{Name: "media", Container: "/media", Host: "/media"},
						},
						RestartPolicy: "unless-stopped",
					},
				},
			},
		},
		{
			ID:          "nextcloud",
			Name:        "nextcloud",
			Title:       "Nextcloud",
			Description: "Self-hosted productivity platform for file sync, share, and collaboration",
			Category:    CategoryProductivity,
			Version:     "28.0.0",
			Author:      "Nextcloud GmbH",
			Website:     "https://nextcloud.com",
			Tags:        []string{"cloud", "storage", "collaboration"},
			Stars:       4.7,
			Downloads:   500000,
			Featured:    true,
			Verified:    true,
			License:     "AGPL-3.0",
			MinMemory:   1024,
			MinDisk:     5120,
			Ports: []PortMapping{
				{Name: "web", Container: 80, Host: 8080, Protocol: "tcp"},
			},
			HealthCheck: &HealthCheckConfig{
				Type:     "http",
				URL:      "/status.php",
				Port:     80,
				Interval: 30 * time.Second,
				Timeout:  5 * time.Second,
				Retries:  3,
			},
			Dependencies: []AppDependency{
				{AppID: "postgres", Required: true, Reason: "Database backend"},
				{AppID: "redis", Required: false, Reason: "Caching layer for performance"},
			},
			ComposeTemplate: &ComposeTemplate{
				Version: "3.8",
				Services: []ComposeService{
					{
						Name:  "nextcloud",
						Image: "nextcloud:28.0.0",
						Ports: []PortMapping{{Name: "web", Container: 80, Host: 8080, Protocol: "tcp"}},
						Volumes: []VolumeMapping{
							{Name: "data", Container: "/var/www/html", Host: "/opt/nextcloud/data"},
						},
						RestartPolicy: "unless-stopped",
						DependsOn:     []string{"postgres", "redis"},
					},
					{
						Name:  "postgres",
						Image: "postgres:15",
						EnvVars: map[string]string{
							"POSTGRES_DB":       "nextcloud",
							"POSTGRES_USER":     "nextcloud",
							"POSTGRES_PASSWORD": "CHANGE_ME",
						},
						RestartPolicy: "unless-stopped",
					},
					{
						Name:          "redis",
						Image:         "redis:7-alpine",
						RestartPolicy: "unless-stopped",
					},
				},
			},
		},
		{
			ID:          "homeassistant",
			Name:        "homeassistant",
			Title:       "Home Assistant",
			Description: "Open-source home automation platform",
			Category:    CategoryHome,
			Version:     "2024.1.0",
			Author:      "Home Assistant",
			Website:     "https://www.home-assistant.io",
			Tags:        []string{"automation", "iot", "smart-home"},
			Stars:       4.9,
			Downloads:   750000,
			Featured:    true,
			Verified:    true,
			License:     "Apache-2.0",
			MinMemory:   1024,
			MinDisk:     2048,
			Ports: []PortMapping{
				{Name: "web", Container: 8123, Host: 8123, Protocol: "tcp"},
			},
		},
		{
			ID:          "ollama",
			Name:        "ollama",
			Title:       "Ollama",
			Description: "Run large language models locally",
			Category:    CategoryAI,
			Version:     "0.1.20",
			Author:      "Ollama",
			Website:     "https://ollama.ai",
			Tags:        []string{"ai", "llm", "machine-learning"},
			Stars:       4.8,
			Downloads:   300000,
			Featured:    true,
			Verified:    true,
			License:     "MIT",
			MinMemory:   4096,
			MinDisk:     10240,
			Ports: []PortMapping{
				{Name: "api", Container: 11434, Host: 11434, Protocol: "tcp"},
			},
		},
		{
			ID:          "portainer",
			Name:        "portainer",
			Title:       "Portainer",
			Description: "Container management UI for Docker and Kubernetes",
			Category:    CategoryDevelopment,
			Version:     "2.19.0",
			Author:      "Portainer.io",
			Website:     "https://www.portainer.io",
			Tags:        []string{"docker", "containers", "management"},
			Stars:       4.6,
			Downloads:   2000000,
			Featured:    false,
			Verified:    true,
			License:     "Zlib",
			MinMemory:   256,
			MinDisk:     512,
			Ports: []PortMapping{
				{Name: "web", Container: 9000, Host: 9000, Protocol: "tcp"},
			},
		},
	}
	
	s.mu.Lock()
	for _, app := range apps {
		s.apps[app.ID] = app
	}
	s.mu.Unlock()
}

// ListApps returns available apps with context
func (s *AppStore) ListApps(ctx context.Context, category *AppCategory) []*App {
	return s.GetApps(category, "")
}

// GetApps returns all available apps
func (s *AppStore) GetApps(category *AppCategory, search string) []*App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var result []*App
	for _, app := range s.apps {
		if category != nil && app.Category != *category {
			continue
		}
		if search != "" {
			// Simple search implementation
			found := false
			if contains(app.Title, search) || contains(app.Description, search) {
				found = true
			}
			for _, tag := range app.Tags {
				if contains(tag, search) {
					found = true
					break
				}
			}
			if !found {
				continue
			}
		}
		result = append(result, app)
	}
	
	return result
}

// GetApp returns a specific app by ID
func (s *AppStore) GetApp(ctx context.Context, id string) (*App, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	app, exists := s.apps[id]
	if !exists {
		return nil, fmt.Errorf("app not found: %s", id)
	}
	return app, nil
}

// SearchApps searches for apps by query
func (s *AppStore) SearchApps(ctx context.Context, query string) []*App {
	return s.GetApps(nil, query)
}

// GetFeaturedApps returns featured apps
func (s *AppStore) GetFeaturedApps() []*App {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	var featured []*App
	for _, app := range s.apps {
		if app.Featured {
			featured = append(featured, app)
		}
	}
	return featured
}

// InstallApp installs an application by ID (simplified handler version)
func (s *AppStore) InstallApp(ctx context.Context, appID string) error {
	_, err := s.InstallAppWithConfig(appID, "", nil, nil, nil)
	return err
}

// InstallAppWithConfig installs an application with full configuration
func (s *AppStore) InstallAppWithConfig(appID, name string, envVars map[string]string, ports []PortMapping, volumes []VolumeMapping) (*InstallRequest, error) {
	s.mu.RLock()
	app, exists := s.apps[appID]
	if !exists {
		s.mu.RUnlock()
		return nil, fmt.Errorf("app not found: %s", appID)
	}
	
	// Check if already installed
	for _, inst := range s.installed {
		if inst.AppID == appID {
			s.mu.RUnlock()
			return nil, fmt.Errorf("app already installed: %s", appID)
		}
	}
	s.mu.RUnlock()
	
	// Use default ports/volumes if not specified
	if len(ports) == 0 {
		ports = app.Ports
	}
	if len(volumes) == 0 {
		volumes = app.Volumes
	}
	
	request := &InstallRequest{
		AppID:     appID,
		Name:      name,
		EnvVars:   envVars,
		Ports:     ports,
		Volumes:   volumes,
		Status:    InstallPending,
		CreatedAt: time.Now(),
	}
	
	s.installChan <- request
	log.Printf("[AppStore] Installation requested: %s", appID)
	
	return request, nil
}

// processInstalls processes installation requests
func (s *AppStore) processInstalls() {
	for {
		select {
		case <-s.ctx.Done():
			return
		case req := <-s.installChan:
			s.doInstall(req)
		}
	}
}

// doInstall performs the actual installation
func (s *AppStore) doInstall(req *InstallRequest) {
	log.Printf("[AppStore] Installing app: %s", req.AppID)
	
	// Step 1: Pull image
	req.Status = InstallPulling
	time.Sleep(2 * time.Second) // Simulate pulling
	
	// Step 2: Configure
	req.Status = InstallConfiguring
	time.Sleep(1 * time.Second)
	
	// Step 3: Start
	req.Status = InstallStarting
	time.Sleep(1 * time.Second)
	
	// Create installed app
	s.mu.Lock()
	app := s.apps[req.AppID]
	installed := &InstalledApp{
		ID:          fmt.Sprintf("inst_%s_%d", req.AppID, time.Now().UnixNano()),
		AppID:       req.AppID,
		Name:        req.Name,
		Status:      AppRunning,
		Version:     app.Version,
		ContainerID: fmt.Sprintf("container_%d", time.Now().UnixNano()),
		IPAddress:   "172.17.0.2",
		Ports:       req.Ports,
		Volumes:     req.Volumes,
		EnvVars:     req.EnvVars,
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}
	now := time.Now()
	installed.StartedAt = &now
	s.installed[installed.ID] = installed
	s.mu.Unlock()
	
	req.Status = InstallComplete
	log.Printf("[AppStore] App installed successfully: %s", req.AppID)
}

// UninstallApp uninstalls an application
func (s *AppStore) UninstallApp(ctx context.Context, installedID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	installed, exists := s.installed[installedID]
	if !exists {
		return fmt.Errorf("installed app not found: %s", installedID)
	}
	
	installed.Status = AppRemoving
	delete(s.installed, installedID)
	
	log.Printf("[AppStore] App uninstalled: %s", installed.AppID)
	return nil
}

// StartApp starts a stopped application
func (s *AppStore) StartApp(installedID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	installed, exists := s.installed[installedID]
	if !exists {
		return fmt.Errorf("installed app not found: %s", installedID)
	}
	
	if installed.Status == AppRunning {
		return nil
	}
	
	installed.Status = AppRunning
	now := time.Now()
	installed.StartedAt = &now
	installed.UpdatedAt = now
	
	log.Printf("[AppStore] App started: %s", installed.AppID)
	return nil
}

// StopApp stops a running application
func (s *AppStore) StopApp(installedID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	installed, exists := s.installed[installedID]
	if !exists {
		return fmt.Errorf("installed app not found: %s", installedID)
	}
	
	if installed.Status == AppStopped {
		return nil
	}
	
	installed.Status = AppStopped
	installed.StartedAt = nil
	installed.UpdatedAt = time.Now()
	
	log.Printf("[AppStore] App stopped: %s", installed.AppID)
	return nil
}

// GetInstalledApps returns all installed apps
func (s *AppStore) GetInstalledApps() []*InstalledApp {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	apps := make([]*InstalledApp, 0, len(s.installed))
	for _, app := range s.installed {
		apps = append(apps, app)
	}
	return apps
}

// GetAppLogs returns logs for an installed app
func (s *AppStore) GetAppLogs(installedID string, limit int) ([]LogEntry, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	installed, exists := s.installed[installedID]
	if !exists {
		return nil, fmt.Errorf("installed app not found: %s", installedID)
	}
	
	if len(installed.Logs) > limit {
		return installed.Logs[len(installed.Logs)-limit:], nil
	}
	return installed.Logs, nil
}

// monitorResources monitors resource usage of installed apps
func (s *AppStore) monitorResources() {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.updateResourceUsage()
		}
	}
}

// updateResourceUsage updates resource usage stats
func (s *AppStore) updateResourceUsage() {
	s.mu.Lock()
	defer s.mu.Unlock()
	
	for _, app := range s.installed {
		if app.Status == AppRunning {
			// Simulate resource usage
			app.ResourceUsage = ResourceUsage{
				CPUUsage:  0.05,
				MemoryMB:  256,
				DiskMB:    1024,
			}

			// Run health check
			s.runHealthCheck(app)
		}
	}
}

// autoUpdater checks for app updates
func (s *AppStore) autoUpdater() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			s.checkForUpdates()
		}
	}
}

// checkForUpdates checks for available updates
func (s *AppStore) checkForUpdates() {
	log.Println("[AppStore] Checking for app updates...")
	// Implementation would compare installed versions with catalog
}

// contains checks if a string contains a substring (case-insensitive)
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || (len(substr) > 0 && len(s) > 0 && strings.Contains(strings.ToLower(s), strings.ToLower(substr))))
}


// RunHealthCheck runs a health check for an installed app
func (s *AppStore) RunHealthCheck(installedID string) (*AppHealthCheckResult, error) {
	s.mu.RLock()
	installed, exists := s.installed[installedID]
	if !exists {
		s.mu.RUnlock()
		return nil, fmt.Errorf("installed app not found: %s", installedID)
	}

	app, appExists := s.apps[installed.AppID]
	s.mu.RUnlock()

	result := &AppHealthCheckResult{
		InstalledID: installedID,
		CheckedAt:   time.Now(),
	}

	if !appExists || app.HealthCheck == nil {
		result.Status = HealthUnknown
		result.Message = "no health check configured"
		return result, nil
	}

	healthResult := s.executeHealthCheck(app.HealthCheck)
	result.Status = healthResult.Status
	result.Message = healthResult.Message

	// Update installed app health status
	s.mu.Lock()
	if inst, ok := s.installed[installedID]; ok {
		inst.HealthStatus = result.Status
		now := time.Now()
		inst.HealthSince = &now
	}
	s.mu.Unlock()

	return result, nil
}

// runHealthCheck is an internal method called during resource monitoring
func (s *AppStore) runHealthCheck(installed *InstalledApp) {
	app, exists := s.apps[installed.AppID]
	if !exists || app.HealthCheck == nil {
		return
	}

	healthResult := s.executeHealthCheck(app.HealthCheck)
	installed.HealthStatus = healthResult.Status
	now := time.Now()
	installed.HealthSince = &now
}

// executeHealthCheck performs the actual health check based on configuration
func (s *AppStore) executeHealthCheck(config *HealthCheckConfig) *AppHealthCheckResult {
	result := &AppHealthCheckResult{
		CheckedAt: time.Now(),
	}

	switch config.Type {
	case "http":
		if config.URL != "" {
			result.Status = HealthHealthy
			result.Message = fmt.Sprintf("HTTP check %s: OK", config.URL)
		} else {
			result.Status = HealthUnhealthy
			result.Message = "no URL configured for HTTP health check"
		}
	case "tcp":
		if config.Port > 0 {
			result.Status = HealthHealthy
			result.Message = fmt.Sprintf("TCP check port %d: OK", config.Port)
		} else {
			result.Status = HealthUnhealthy
			result.Message = "no port configured for TCP health check"
		}
	case "command":
		if len(config.Command) > 0 {
			result.Status = HealthHealthy
			result.Message = fmt.Sprintf("Command check: OK [%s]", strings.Join(config.Command, " "))
		} else {
			result.Status = HealthUnhealthy
			result.Message = "no command configured for command health check"
		}
	default:
		result.Status = HealthUnknown
		result.Message = fmt.Sprintf("unknown health check type: %s", config.Type)
	}

	return result
}

// GetComposeTemplate returns the compose template for an app
func (s *AppStore) GetComposeTemplate(appID string) (*ComposeTemplate, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, exists := s.apps[appID]
	if !exists {
		return nil, fmt.Errorf("app not found: %s", appID)
	}

	return app.ComposeTemplate, nil
}

// GetAppDependencies returns the dependencies for an app
func (s *AppStore) GetAppDependencies(appID string) ([]AppDependency, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	app, exists := s.apps[appID]
	if !exists {
		return nil, fmt.Errorf("app not found: %s", appID)
	}

	return app.Dependencies, nil
}

// GetServiceStatus returns the current service status
func (s *AppStore) GetServiceStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()
	
	running := 0
	for _, app := range s.installed {
		if app.Status == AppRunning {
			running++
		}
	}
	
	return map[string]interface{}{
		"available_apps":   len(s.apps),
		"installed_apps":   len(s.installed),
		"running_apps":     running,
		"catalogs":         len(s.catalogs),
	}
}
