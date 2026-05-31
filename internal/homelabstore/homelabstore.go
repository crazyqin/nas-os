// Package homelabstore 提供家庭实验室应用商店
// 精选自托管应用目录、一键安装/卸载、自动更新管理、社区评分
package homelabstore

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"sync"
	"time"
)

// ========== 常量 ==========

const (
	Version = "1.0.0"
)

// ========== 应用状态 ==========

// AppStatus 应用状态
type AppStatus string

const (
	AppStatusAvailable AppStatus = "available"
	AppStatusInstalling AppStatus = "installing"
	AppStatusInstalled  AppStatus = "installed"
	AppStatusUpdating   AppStatus = "updating"
	AppStatusRemoving   AppStatus = "removing"
	AppStatusError      AppStatus = "error"
)

// ========== 应用类别 ==========

// AppCategory 应用类别
type AppCategory string

const (
	CategoryMedia      AppCategory = "media"
	CategoryProductivity AppCategory = "productivity"
	CategoryDevelopment AppCategory = "development"
	CategoryNetworking  AppCategory = "networking"
	CategorySecurity    AppCategory = "security"
	CategoryMonitoring  AppCategory = "monitoring"
	CategoryDatabase    AppCategory = "database"
	CategoryStorage     AppCategory = "storage"
	CategoryCommunication AppCategory = "communication"
	CategoryAutomation  AppCategory = "automation"
	CategoryAI          AppCategory = "ai"
	CategoryGaming      AppCategory = "gaming"
	CategoryOther       AppCategory = "other"
)

// ========== 数据结构 ==========

// App 应用信息
type App struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	DisplayName string      `json:"display_name"`
	Description string      `json:"description"`
	Category    AppCategory `json:"category"`
	Version     string      `json:"version"`
	LatestVersion string    `json:"latest_version"`
	Author      string      `json:"author"`
	Website     string      `json:"website"`
	Repository  string      `json:"repository"`
	License     string      `json:"license"`
	Icon        string      `json:"icon"`
	Screenshots []string    `json:"screenshots"`
	Tags        []string    `json:"tags"`
	Status      AppStatus   `json:"status"`
	Installed   bool        `json:"installed"`
	InstalledAt *time.Time  `json:"installed_at,omitempty"`
	UpdatedAt   *time.Time  `json:"updated_at,omitempty"`
	Size        int64       `json:"size"`
	Downloads   int64       `json:"downloads"`
	Rating      float64     `json:"rating"`
	RatingCount int         `json:"rating_count"`
	Featured    bool        `json:"featured"`
	Verified    bool        `json:"verified"`
	MinCPU      int         `json:"min_cpu"`
	MinMemory   int64       `json:"min_memory"`
	MinDisk     int64       `json:"min_disk"`
	Ports       []int       `json:"ports"`
	Volumes     []string    `json:"volumes"`
	EnvVars     []EnvVar    `json:"env_vars"`
	Compose     string      `json:"compose"`
	HealthCheck string      `json:"health_check"`
}

// EnvVar 环境变量
type EnvVar struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Default     string `json:"default"`
	Required    bool   `json:"required"`
	Secret      bool   `json:"secret"`
}

// Review 用户评价
type Review struct {
	ID        string    `json:"id"`
	AppID     string    `json:"app_id"`
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Rating    int       `json:"rating"` // 1-5
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	Helpful   int       `json:"helpful"`
	CreatedAt time.Time `json:"created_at"`
}

// InstallTask 安装任务
type InstallTask struct {
	ID        string    `json:"id"`
	AppID     string    `json:"app_id"`
	Action    string    `json:"action"` // "install", "update", "remove"
	Status    string    `json:"status"` // "pending", "running", "completed", "failed"
	Progress  int       `json:"progress"` // 0-100
	Message   string    `json:"message"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   *time.Time `json:"ended_at,omitempty"`
	Error     string    `json:"error,omitempty"`
}

// StoreStats 商店统计
type StoreStats struct {
	TotalApps     int            `json:"total_apps"`
	InstalledApps int            `json:"installed_apps"`
	Categories    map[string]int `json:"categories"`
	TotalDownloads int64         `json:"total_downloads"`
	FeaturedApps  int            `json:"featured_apps"`
	LastUpdated   time.Time      `json:"last_updated"`
}

// ========== 管理器 ==========

// AppStore 应用商店
type AppStore struct {
	mu       sync.RWMutex
	apps     map[string]*App
	reviews  map[string][]*Review
	tasks    map[string]*InstallTask
	catalog  []*App // 预置目录
}

// NewAppStore 创建应用商店
func NewAppStore() *AppStore {
	store := &AppStore{
		apps:    make(map[string]*App),
		reviews: make(map[string][]*Review),
		tasks:   make(map[string]*InstallTask),
	}
	store.initCatalog()
	return store
}

// initCatalog 初始化应用目录
func (s *AppStore) initCatalog() {
	s.catalog = []*App{
		{
			ID: "nextcloud", Name: "nextcloud", DisplayName: "Nextcloud",
			Description: "私有云存储和协作平台", Category: CategoryStorage,
			Version: "28.0.0", Author: "Nextcloud GmbH", License: "AGPL-3.0",
			Tags: []string{"cloud", "sync", "sharing"}, Featured: true, Verified: true,
			Rating: 4.5, Downloads: 1000000, Size: 500*1024*1024,
			Ports: []int{8080}, MinCPU: 2, MinMemory: 2*1024*1024*1024,
		},
		{
			ID: "jellyfin", Name: "jellyfin", DisplayName: "Jellyfin",
			Description: "免费开源媒体服务器", Category: CategoryMedia,
			Version: "10.8.0", Author: "Jellyfin Project", License: "GPL-2.0",
			Tags: []string{"media", "streaming", "video"}, Featured: true, Verified: true,
			Rating: 4.7, Downloads: 500000, Size: 300*1024*1024,
			Ports: []int{8096}, MinCPU: 2, MinMemory: 2*1024*1024*1024,
		},
		{
			ID: "homeassistant", Name: "homeassistant", DisplayName: "Home Assistant",
			Description: "开源智能家居自动化平台", Category: CategoryAutomation,
			Version: "2024.1.0", Author: "Home Assistant", License: "Apache-2.0",
			Tags: []string{"iot", "smart-home", "automation"}, Featured: true, Verified: true,
			Rating: 4.8, Downloads: 800000, Size: 200*1024*1024,
			Ports: []int{8123}, MinCPU: 1, MinMemory: 1*1024*1024*1024,
		},
		{
			ID: "pihole", Name: "pihole", DisplayName: "Pi-hole",
			Description: "网络广告拦截器", Category: CategoryNetworking,
			Version: "5.17.0", Author: "Pi-hole", License: "EUPL-1.2",
			Tags: []string{"dns", "ad-block", "privacy"}, Featured: true, Verified: true,
			Rating: 4.6, Downloads: 600000, Size: 50*1024*1024,
			Ports: []int{80, 53}, MinCPU: 1, MinMemory: 512*1024*1024,
		},
		{
			ID: "portainer", Name: "portainer", DisplayName: "Portainer",
			Description: "容器管理界面", Category: CategoryDevelopment,
			Version: "2.19.0", Author: "Portainer.io", License: "Zlib",
			Tags: []string{"docker", "containers", "management"}, Featured: true, Verified: true,
			Rating: 4.4, Downloads: 700000, Size: 100*1024*1024,
			Ports: []int{9000}, MinCPU: 1, MinMemory: 256*1024*1024,
		},
		{
			ID: "vaultwarden", Name: "vaultwarden", DisplayName: "Vaultwarden",
			Description: "Bitwarden兼容密码管理器", Category: CategorySecurity,
			Version: "1.30.0", Author: "Daniel García", License: "AGPL-3.0",
			Tags: []string{"password", "security", "vault"}, Featured: true, Verified: true,
			Rating: 4.9, Downloads: 400000, Size: 50*1024*1024,
			Ports: []int{8080}, MinCPU: 1, MinMemory: 256*1024*1024,
		},
		{
			ID: "grafana", Name: "grafana", DisplayName: "Grafana",
			Description: "数据可视化和监控平台", Category: CategoryMonitoring,
			Version: "10.2.0", Author: "Grafana Labs", License: "AGPL-3.0",
			Tags: []string{"monitoring", "dashboard", "visualization"}, Featured: true, Verified: true,
			Rating: 4.6, Downloads: 900000, Size: 150*1024*1024,
			Ports: []int{3000}, MinCPU: 1, MinMemory: 512*1024*1024,
		},
		{
			ID: "n8n", Name: "n8n", DisplayName: "n8n",
			Description: "工作流自动化平台", Category: CategoryAutomation,
			Version: "1.22.0", Author: "n8n GmbH", License: "Sustainable Use",
			Tags: []string{"automation", "workflow", "integration"}, Featured: true, Verified: true,
			Rating: 4.5, Downloads: 300000, Size: 200*1024*1024,
			Ports: []int{5678}, MinCPU: 2, MinMemory: 1*1024*1024*1024,
		},
		{
			ID: "ollama", Name: "ollama", DisplayName: "Ollama",
			Description: "本地大语言模型运行", Category: CategoryAI,
			Version: "0.1.20", Author: "Ollama", License: "MIT",
			Tags: []string{"ai", "llm", "local"}, Featured: true, Verified: true,
			Rating: 4.8, Downloads: 200000, Size: 500*1024*1024,
			Ports: []int{11434}, MinCPU: 4, MinMemory: 8*1024*1024*1024,
		},
		{
			ID: "immich", Name: "immich", DisplayName: "Immich",
			Description: "自托管照片和视频管理", Category: CategoryMedia,
			Version: "1.91.0", Author: "Immich", License: "AGPL-3.0",
			Tags: []string{"photos", "videos", "backup"}, Featured: true, Verified: true,
			Rating: 4.7, Downloads: 250000, Size: 300*1024*1024,
			Ports: []int{2283}, MinCPU: 2, MinMemory: 2*1024*1024*1024,
		},
	}

	// 初始化应用映射
	for _, app := range s.catalog {
		s.apps[app.ID] = app
	}
}

// GetCatalog 获取应用目录
func (s *AppStore) GetCatalog() []*App {
	s.mu.RLock()
	defer s.mu.RUnlock()

	apps := make([]*App, 0, len(s.apps))
	for _, app := range s.apps {
		apps = append(apps, app)
	}
	sort.Slice(apps, func(i, j int) bool {
		return apps[i].Downloads > apps[j].Downloads
	})
	return apps
}

// GetApp 获取应用详情
func (s *AppStore) GetApp(appID string) (*App, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	app, ok := s.apps[appID]
	return app, ok
}

// GetFeaturedApps 获取精选应用
func (s *AppStore) GetFeaturedApps() []*App {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var featured []*App
	for _, app := range s.apps {
		if app.Featured {
			featured = append(featured, app)
		}
	}
	sort.Slice(featured, func(i, j int) bool {
		return featured[i].Rating > featured[j].Rating
	})
	return featured
}

// GetAppsByCategory 按类别获取应用
func (s *AppStore) GetAppsByCategory(category AppCategory) []*App {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var apps []*App
	for _, app := range s.apps {
		if app.Category == category {
			apps = append(apps, app)
		}
	}
	sort.Slice(apps, func(i, j int) bool {
		return apps[i].Rating > apps[j].Rating
	})
	return apps
}

// SearchApps 搜索应用
func (s *AppStore) SearchApps(query string) []*App {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var results []*App
	query = lowercase(query)
	for _, app := range s.apps {
		if contains(app.Name, query) || contains(app.DisplayName, query) || contains(app.Description, query) {
			results = append(results, app)
		}
	}
	sort.Slice(results, func(i, j int) bool {
		return results[i].Rating > results[j].Rating
	})
	return results
}

// InstallApp 安装应用
func (s *AppStore) InstallApp(appID string) (*InstallTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, exists := s.apps[appID]
	if !exists {
		return nil, fmt.Errorf("app not found: %s", appID)
	}
	if app.Installed {
		return nil, fmt.Errorf("app already installed: %s", appID)
	}

	task := &InstallTask{
		ID:        fmt.Sprintf("task-%d", time.Now().UnixNano()),
		AppID:     appID,
		Action:    "install",
		Status:    "pending",
		Progress:  0,
		StartedAt: time.Now(),
	}
	s.tasks[task.ID] = task

	// 模拟安装过程
	go s.executeInstall(task)

	return task, nil
}

// executeInstall 执行安装
func (s *AppStore) executeInstall(task *InstallTask) {
	s.mu.Lock()
	task.Status = "running"
	task.Progress = 10
	s.mu.Unlock()

	time.Sleep(time.Second * 2)

	s.mu.Lock()
	task.Progress = 50
	s.mu.Unlock()

	time.Sleep(time.Second * 2)

	s.mu.Lock()
	task.Progress = 90
	s.mu.Unlock()

	time.Sleep(time.Second * 1)

	s.mu.Lock()
	defer s.mu.Unlock()

	task.Status = "completed"
	task.Progress = 100
	now := time.Now()
	task.EndedAt = &now

	if app, exists := s.apps[task.AppID]; exists {
		app.Installed = true
		app.Status = AppStatusInstalled
		app.InstalledAt = &now
	}
}

// UpdateApp 更新应用
func (s *AppStore) UpdateApp(appID string) (*InstallTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, exists := s.apps[appID]
	if !exists {
		return nil, fmt.Errorf("app not found: %s", appID)
	}
	if !app.Installed {
		return nil, fmt.Errorf("app not installed: %s", appID)
	}

	task := &InstallTask{
		ID:        fmt.Sprintf("task-%d", time.Now().UnixNano()),
		AppID:     appID,
		Action:    "update",
		Status:    "pending",
		Progress:  0,
		StartedAt: time.Now(),
	}
	s.tasks[task.ID] = task

	go s.executeUpdate(task)

	return task, nil
}

// executeUpdate 执行更新
func (s *AppStore) executeUpdate(task *InstallTask) {
	s.mu.Lock()
	task.Status = "running"
	task.Progress = 30
	s.mu.Unlock()

	time.Sleep(time.Second * 3)

	s.mu.Lock()
	task.Progress = 70
	s.mu.Unlock()

	time.Sleep(time.Second * 2)

	s.mu.Lock()
	defer s.mu.Unlock()

	task.Status = "completed"
	task.Progress = 100
	now := time.Now()
	task.EndedAt = &now

	if app, exists := s.apps[task.AppID]; exists {
		app.Version = app.LatestVersion
		app.UpdatedAt = &now
	}
}

// RemoveApp 移除应用
func (s *AppStore) RemoveApp(appID string) (*InstallTask, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	app, exists := s.apps[appID]
	if !exists {
		return nil, fmt.Errorf("app not found: %s", appID)
	}
	if !app.Installed {
		return nil, fmt.Errorf("app not installed: %s", appID)
	}

	task := &InstallTask{
		ID:        fmt.Sprintf("task-%d", time.Now().UnixNano()),
		AppID:     appID,
		Action:    "remove",
		Status:    "pending",
		Progress:  0,
		StartedAt: time.Now(),
	}
	s.tasks[task.ID] = task

	go s.executeRemove(task)

	return task, nil
}

// executeRemove 执行移除
func (s *AppStore) executeRemove(task *InstallTask) {
	s.mu.Lock()
	task.Status = "running"
	task.Progress = 50
	s.mu.Unlock()

	time.Sleep(time.Second * 2)

	s.mu.Lock()
	defer s.mu.Unlock()

	task.Status = "completed"
	task.Progress = 100
	now := time.Now()
	task.EndedAt = &now

	if app, exists := s.apps[task.AppID]; exists {
		app.Installed = false
		app.Status = AppStatusAvailable
		app.InstalledAt = nil
	}
}

// AddReview 添加评价
func (s *AppStore) AddReview(review *Review) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.apps[review.AppID]; !exists {
		return fmt.Errorf("app not found: %s", review.AppID)
	}

	if review.ID == "" {
		review.ID = fmt.Sprintf("review-%d", time.Now().UnixNano())
	}
	review.CreatedAt = time.Now()

	s.reviews[review.AppID] = append(s.reviews[review.AppID], review)

	// 更新评分
	app := s.apps[review.AppID]
	totalRating := 0.0
	count := 0
	for _, r := range s.reviews[review.AppID] {
		totalRating += float64(r.Rating)
		count++
	}
	app.Rating = totalRating / float64(count)
	app.RatingCount = count

	return nil
}

// GetReviews 获取应用评价
func (s *AppStore) GetReviews(appID string) []*Review {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.reviews[appID]
}

// GetInstalledApps 获取已安装应用
func (s *AppStore) GetInstalledApps() []*App {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var installed []*App
	for _, app := range s.apps {
		if app.Installed {
			installed = append(installed, app)
		}
	}
	return installed
}

// GetStats 获取商店统计
func (s *AppStore) GetStats() *StoreStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &StoreStats{
		TotalApps:  len(s.apps),
		Categories: make(map[string]int),
		LastUpdated: time.Now(),
	}

	for _, app := range s.apps {
		stats.Categories[string(app.Category)]++
		stats.TotalDownloads += app.Downloads
		if app.Installed {
			stats.InstalledApps++
		}
		if app.Featured {
			stats.FeaturedApps++
		}
	}

	return stats
}

// GetTask 获取任务状态
func (s *AppStore) GetTask(taskID string) (*InstallTask, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	task, ok := s.tasks[taskID]
	return task, ok
}

// 辅助函数
func lowercase(s string) string {
	result := make([]byte, len(s))
	for i, c := range s {
		if c >= 'A' && c <= 'Z' {
			result[i] = byte(c + 32)
		} else {
			result[i] = byte(c)
		}
	}
	return string(result)
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || searchString(s, substr))
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ========== HTTP API ==========

// Handler HTTP API处理器
type Handler struct {
	store *AppStore
}

// NewHandler 创建处理器
func NewHandler(store *AppStore) *Handler {
	return &Handler{store: store}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/catalog", h.handleCatalog)
	mux.HandleFunc(prefix+"/featured", h.handleFeatured)
	mux.HandleFunc(prefix+"/search", h.handleSearch)
	mux.HandleFunc(prefix+"/app", h.handleApp)
	mux.HandleFunc(prefix+"/install", h.handleInstall)
	mux.HandleFunc(prefix+"/update", h.handleUpdate)
	mux.HandleFunc(prefix+"/remove", h.handleRemove)
	mux.HandleFunc(prefix+"/reviews", h.handleReviews)
	mux.HandleFunc(prefix+"/installed", h.handleInstalled)
	mux.HandleFunc(prefix+"/stats", h.handleStats)
	mux.HandleFunc(prefix+"/task", h.handleTask)
}

func (h *Handler) handleCatalog(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.store.GetCatalog())
}

func (h *Handler) handleFeatured(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.store.GetFeaturedApps())
}

func (h *Handler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	query := r.URL.Query().Get("q")
	json.NewEncoder(w).Encode(h.store.SearchApps(query))
}

func (h *Handler) handleApp(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	appID := r.URL.Query().Get("id")
	app, ok := h.store.GetApp(appID)
	if !ok {
		http.Error(w, "app not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(app)
}

func (h *Handler) handleInstall(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		AppID string `json:"app_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	task, err := h.store.InstallApp(req.AppID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(task)
}

func (h *Handler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		AppID string `json:"app_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	task, err := h.store.UpdateApp(req.AppID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(task)
}

func (h *Handler) handleRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		AppID string `json:"app_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	task, err := h.store.RemoveApp(req.AppID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(task)
}

func (h *Handler) handleReviews(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		appID := r.URL.Query().Get("app_id")
		json.NewEncoder(w).Encode(h.store.GetReviews(appID))
	case http.MethodPost:
		var review Review
		if err := json.NewDecoder(r.Body).Decode(&review); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := h.store.AddReview(&review); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(review)
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleInstalled(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.store.GetInstalledApps())
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.store.GetStats())
}

func (h *Handler) handleTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	taskID := r.URL.Query().Get("id")
	task, ok := h.store.GetTask(taskID)
	if !ok {
		http.Error(w, "task not found", http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(task)
}
