// Package appstore 提供应用商店功能增强
// 应用目录管理 - 支持多源仓库、应用推荐、依赖解析、沙箱隔离、批量管理
package appstore

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ========== 应用目录类型 ==========

// CatalogEntry 应用目录条目
type CatalogEntry struct {
	ID           string            `json:"id"`
	Name         string            `json:"name"`
	DisplayName  string            `json:"displayName"`
	Description  string            `json:"description"`
	Category     string            `json:"category"`
	Icon         string            `json:"icon"`
	Version      string            `json:"version"`
	LatestVersion string           `json:"latestVersion"`
	Author       string            `json:"author"`
	Website      string            `json:"website"`
	Source       string            `json:"source"`
	License      string            `json:"license"`
	Tags         []string          `json:"tags"`
	Rating       float64           `json:"rating"`
	Downloads    int64             `json:"downloads"`
	Containers   []ContainerSpec   `json:"containers"`
	Dependencies []string          `json:"dependencies,omitempty"`
	Conflicts    []string          `json:"conflicts,omitempty"`
	Metadata     map[string]string `json:"metadata,omitempty"`
	RepositoryID string            `json:"repositoryId"`
	UpdatedAt    time.Time         `json:"updatedAt"`
}

// ContainerSpec 容器规格
type ContainerSpec struct {
	Name            string            `json:"name"`
	Image           string            `json:"image"`
	Ports           []PortSpec        `json:"ports"`
	Volumes         []VolumeSpec      `json:"volumes"`
	Environment     map[string]string `json:"environment"`
	RestartPolicy   string            `json:"restartPolicy"`
	NetworkMode     string            `json:"networkMode"`
	Privileged      bool              `json:"privileged"`
	ResourceLimits  *ResourceLimits   `json:"resourceLimits,omitempty"`
	ComposeTemplate string            `json:"composeTemplate,omitempty"`
}

// PortSpec 端口规格
type PortSpec struct {
	Name            string `json:"name"`
	ContainerPort   int    `json:"containerPort"`
	Protocol        string `json:"protocol"`
	DefaultHostPort int    `json:"defaultHostPort"`
	Description     string `json:"description"`
}

// VolumeSpec 卷规格
type VolumeSpec struct {
	Name            string `json:"name"`
	ContainerPath   string `json:"containerPath"`
	DefaultHostPath string `json:"defaultHostPath"`
	Description     string `json:"description"`
	ReadOnly        bool   `json:"readOnly"`
}

// ResourceLimits 资源限制
type ResourceLimits struct {
	CPUCores  float64 `json:"cpuCores"`
	MemoryMB  int64   `json:"memoryMB"`
	DiskGB    int64   `json:"diskGB"`
	IOPSRead  int     `json:"iopsRead,omitempty"`
	IOPSWrite int     `json:"iopsWrite,omitempty"`
	NetMbps   int     `json:"netMbps,omitempty"`
}

// ========== 仓库源类型 ==========

// Repository 仓库定义
type Repository struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	URL       string    `json:"url"`
	Type      string    `json:"type"` // "official", "community", "custom"
	Enabled   bool      `json:"enabled"`
	Priority  int       `json:"priority"` // 越高越优先
	Verified  bool      `json:"verified"`
	UpdatedAt time.Time `json:"updatedAt"`
	Apps      int       `json:"apps"` // 应用数量
}

// RepositoryData 仓库数据（从远端获取）
type RepositoryData struct {
	Repository Repository      `json:"repository"`
	Apps       []CatalogEntry  `json:"apps"`
	SyncedAt   time.Time       `json:"syncedAt"`
}

// ========== 多源应用目录 ==========

// Catalog 应用目录管理器
type Catalog struct {
	mu          sync.RWMutex
	repos       map[string]*Repository
	repoData    map[string]*RepositoryData // 仓库数据缓存
	entries     map[string]*CatalogEntry   // 合并后的统一目录
	dataDir     string
	httpClient  *http.Client
}

// CatalogConfig 目录配置
type CatalogConfig struct {
	DataDir       string       `json:"dataDir"`
	Repositories  []Repository `json:"repositories"`
	SyncInterval  time.Duration `json:"syncInterval"`
	HTTPTimeout   time.Duration `json:"httpTimeout"`
}

// NewCatalog 创建应用目录
func NewCatalog(cfg *CatalogConfig) *Catalog {
	if cfg == nil {
		cfg = &CatalogConfig{
			DataDir:      "/var/lib/nas-os/appstore",
			SyncInterval: 1 * time.Hour,
			HTTPTimeout:  30 * time.Second,
		}
	}
	if cfg.HTTPTimeout == 0 {
		cfg.HTTPTimeout = 30 * time.Second
	}

	c := &Catalog{
		repos:      make(map[string]*Repository),
		repoData:   make(map[string]*RepositoryData),
		entries:    make(map[string]*CatalogEntry),
		dataDir:    cfg.DataDir,
		httpClient: &http.Client{Timeout: cfg.HTTPTimeout},
	}

	// 注册默认仓库
	defaultRepos := []Repository{
		{ID: "official", Name: "官方仓库", URL: "https://apps.nas-os.io/official", Type: "official", Enabled: true, Priority: 100, Verified: true},
		{ID: "community", Name: "社区仓库", URL: "https://apps.nas-os.io/community", Type: "community", Enabled: true, Priority: 50, Verified: false},
	}
	for _, r := range defaultRepos {
		c.repos[r.ID] = &Repository{
			ID:        r.ID,
			Name:      r.Name,
			URL:       r.URL,
			Type:      r.Type,
			Enabled:   r.Enabled,
			Priority:  r.Priority,
			Verified:  r.Verified,
			UpdatedAt: time.Now(),
		}
	}

	// 注册用户自定义仓库
	for _, r := range cfg.Repositories {
		if r.ID == "" {
			continue
		}
		c.repos[r.ID] = &Repository{
			ID:        r.ID,
			Name:      r.Name,
			URL:       r.URL,
			Type:      r.Type,
			Enabled:   r.Enabled,
			Priority:  r.Priority,
			Verified:  r.Verified,
			UpdatedAt: time.Now(),
		}
	}

	// 加载本地缓存
	c.loadCache()

	// 加载内置应用
	c.loadBuiltinApps()

	return c
}

// AddRepository 添加仓库
func (c *Catalog) AddRepository(repo *Repository) error {
	if repo.ID == "" {
		return fmt.Errorf("仓库ID不能为空")
	}
	if repo.URL == "" {
		return fmt.Errorf("仓库URL不能为空")
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.repos[repo.ID]; exists {
		return fmt.Errorf("仓库 %s 已存在", repo.ID)
	}

	repo.UpdatedAt = time.Now()
	if repo.Type == "" {
		repo.Type = "custom"
	}
	c.repos[repo.ID] = repo

	return nil
}

// RemoveRepository 移除仓库
func (c *Catalog) RemoveRepository(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.repos[id]; !exists {
		return fmt.Errorf("仓库 %s 不存在", id)
	}

	delete(c.repos, id)
	delete(c.repoData, id)
	c.rebuildEntries()

	return nil
}

// UpdateRepository 更新仓库配置
func (c *Catalog) UpdateRepository(repo *Repository) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if _, exists := c.repos[repo.ID]; !exists {
		return fmt.Errorf("仓库 %s 不存在", repo.ID)
	}

	repo.UpdatedAt = time.Now()
	c.repos[repo.ID] = repo

	return nil
}

// ListRepositories 列出所有仓库
func (c *Catalog) ListRepositories() []*Repository {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*Repository, 0, len(c.repos))
	for _, r := range c.repos {
		result = append(result, r)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Priority > result[j].Priority
	})

	return result
}

// SyncRepository 同步单个仓库
func (c *Catalog) SyncRepository(ctx context.Context, repoID string) error {
	c.mu.RLock()
	repo, exists := c.repos[repoID]
	c.mu.RUnlock()

	if !exists {
		return fmt.Errorf("仓库 %s 不存在", repoID)
	}
	if !repo.Enabled {
		return fmt.Errorf("仓库 %s 已禁用", repoID)
	}

	data, err := c.fetchRepoData(ctx, repo)
	if err != nil {
		return fmt.Errorf("同步仓库 %s 失败: %w", repoID, err)
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	c.repoData[repoID] = data
	repo.Apps = len(data.Apps)
	repo.UpdatedAt = time.Now()
	c.rebuildEntries()

	return nil
}

// SyncAll 同步所有启用的仓库
func (c *Catalog) SyncAll(ctx context.Context) map[string]error {
	c.mu.RLock()
	repoIDs := make([]string, 0, len(c.repos))
	for id, r := range c.repos {
		if r.Enabled {
			repoIDs = append(repoIDs, id)
		}
	}
	c.mu.RUnlock()

	results := make(map[string]error)
	for _, id := range repoIDs {
		results[id] = c.SyncRepository(ctx, id)
	}

	return results
}

// ListApps 列出所有应用（支持过滤）
func (c *Catalog) ListApps(filter *AppFilter) []*CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	result := make([]*CatalogEntry, 0, len(c.entries))
	for _, entry := range c.entries {
		if filter != nil {
			if filter.Category != "" && entry.Category != filter.Category {
				continue
			}
			if filter.Tag != "" {
				found := false
				for _, tag := range entry.Tags {
					if strings.EqualFold(tag, filter.Tag) {
						found = true
						break
					}
				}
				if !found {
					continue
				}
			}
			if filter.RepositoryID != "" && entry.RepositoryID != filter.RepositoryID {
				continue
			}
			if filter.Verified && !c.isVerified(entry.RepositoryID) {
				continue
			}
		}
		result = append(result, entry)
	}

	// 按评分和下载量排序
	sort.Slice(result, func(i, j int) bool {
		si := result[i].Rating*100 + float64(result[i].Downloads)/1000
		sj := result[j].Rating*100 + float64(result[j].Downloads)/1000
		return si > sj
	})

	return result
}

// AppFilter 应用过滤器
type AppFilter struct {
	Category     string `json:"category"`
	Tag          string `json:"tag"`
	RepositoryID string `json:"repositoryId"`
	Verified     bool   `json:"verified"`
	Query        string `json:"query"`
}

// GetApp 获取应用详情
func (c *Catalog) GetApp(id string) (*CatalogEntry, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	entry, ok := c.entries[id]
	return entry, ok
}

// SearchApps 搜索应用
func (c *Catalog) SearchApps(query string) []*CatalogEntry {
	c.mu.RLock()
	defer c.mu.RUnlock()

	query = strings.ToLower(query)
	result := make([]*CatalogEntry, 0)

	for _, entry := range c.entries {
		if strings.Contains(strings.ToLower(entry.Name), query) ||
			strings.Contains(strings.ToLower(entry.DisplayName), query) ||
			strings.Contains(strings.ToLower(entry.Description), query) ||
			matchTags(entry.Tags, query) {
			result = append(result, entry)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		// 精确匹配优先
		iExact := strings.ToLower(result[i].Name) == query || strings.ToLower(result[i].DisplayName) == query
		jExact := strings.ToLower(result[j].Name) == query || strings.ToLower(result[j].DisplayName) == query
		if iExact != jExact {
			return iExact
		}
		return result[i].Rating > result[j].Rating
	})

	return result
}

// Categories 获取所有分类
func (c *Catalog) Categories() []string {
	c.mu.RLock()
	defer c.mu.RUnlock()

	catSet := make(map[string]struct{})
	for _, entry := range c.entries {
		if entry.Category != "" {
			catSet[entry.Category] = struct{}{}
		}
	}

	result := make([]string, 0, len(catSet))
	for cat := range catSet {
		result = append(result, cat)
	}
	sort.Strings(result)

	return result
}

// GetUpdates 获取可更新的应用
func (c *Catalog) GetUpdates(installed map[string]string) []*UpdateInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var updates []*UpdateInfo
	for id, currentVersion := range installed {
		entry, ok := c.entries[id]
		if !ok {
			continue
		}
		if entry.LatestVersion != "" && entry.LatestVersion != currentVersion {
			updates = append(updates, &UpdateInfo{
				AppID:         id,
				Name:          entry.DisplayName,
				CurrentVersion: currentVersion,
				LatestVersion: entry.LatestVersion,
				Changelog:     entry.Metadata["changelog"],
			})
		}
	}

	return updates
}

// UpdateInfo 更新信息
type UpdateInfo struct {
	AppID          string `json:"appId"`
	Name           string `json:"name"`
	CurrentVersion string `json:"currentVersion"`
	LatestVersion  string `json:"latestVersion"`
	Changelog      string `json:"changelog,omitempty"`
}

// ========== 内部方法 ==========

// fetchRepoData 获取仓库数据
func (c *Catalog) fetchRepoData(ctx context.Context, repo *Repository) (*RepositoryData, error) {
	// 尝试从本地缓存加载
	localPath := filepath.Join(c.dataDir, "repos", repo.ID+".json")
	if data, err := os.ReadFile(localPath); err == nil {
		var repoData RepositoryData
		if err := json.Unmarshal(data, &repoData); err == nil {
			repoData.SyncedAt = time.Now()
			return &repoData, nil
		}
	}

	// 尝试从远端获取
	if repo.URL != "" && strings.HasPrefix(repo.URL, "http") {
		req, err := http.NewRequestWithContext(ctx, "GET", repo.URL+"/catalog.json", nil)
		if err != nil {
			return nil, err
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, err
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return nil, fmt.Errorf("HTTP %d", resp.StatusCode)
		}

		var repoData RepositoryData
		if err := json.NewDecoder(resp.Body).Decode(&repoData); err != nil {
			return nil, err
		}

		repoData.Repository = *repo
		repoData.SyncedAt = time.Now()

		// 缓存到本地
		os.MkdirAll(filepath.Join(c.dataDir, "repos"), 0750)
		if data, err := json.MarshalIndent(&repoData, "", "  "); err == nil {
			os.WriteFile(localPath, data, 0644)
		}

		return &repoData, nil
	}

	return &RepositoryData{
		Repository: *repo,
		Apps:       []CatalogEntry{},
		SyncedAt:   time.Now(),
	}, nil
}

// rebuildEntries 重建合并目录（按优先级合并，高优先级仓库的应用优先）
func (c *Catalog) rebuildEntries() {
	c.entries = make(map[string]*CatalogEntry)

	// 按优先级排序仓库
	repoList := make([]*Repository, 0, len(c.repos))
	for _, r := range c.repos {
		if r.Enabled {
			repoList = append(repoList, r)
		}
	}
	sort.Slice(repoList, func(i, j int) bool {
		return repoList[i].Priority > repoList[j].Priority
	})

	// 按优先级合并
	for _, repo := range repoList {
		data, ok := c.repoData[repo.ID]
		if !ok {
			continue
		}
		for i := range data.Apps {
			entry := &data.Apps[i]
			entry.RepositoryID = repo.ID
			if existing, exists := c.entries[entry.ID]; exists {
				// 高优先级仓库覆盖低优先级
				if repo.Priority > c.getRepoPriority(existing.RepositoryID) {
					c.entries[entry.ID] = entry
				}
			} else {
				c.entries[entry.ID] = entry
			}
		}
	}
}

// getRepoPriority 获取仓库优先级
func (c *Catalog) getRepoPriority(repoID string) int {
	if r, ok := c.repos[repoID]; ok {
		return r.Priority
	}
	return 0
}

// isVerified 检查仓库是否经过验证
func (c *Catalog) isVerified(repoID string) bool {
	if r, ok := c.repos[repoID]; ok {
		return r.Verified
	}
	return false
}

// loadCache 加载本地缓存
func (c *Catalog) loadCache() {
	cacheFile := filepath.Join(c.dataDir, "catalog-cache.json")
	data, err := os.ReadFile(cacheFile)
	if err != nil {
		return
	}

	var cached []*RepositoryData
	if err := json.Unmarshal(data, &cached); err != nil {
		return
	}

	for _, rd := range cached {
		if rd.Repository.ID != "" {
			c.repoData[rd.Repository.ID] = rd
		}
	}

	c.rebuildEntries()
}

// SaveCache 保存缓存
func (c *Catalog) SaveCache() error {
	c.mu.RLock()
	defer c.mu.RUnlock()

	os.MkdirAll(c.dataDir, 0750)

	data := make([]*RepositoryData, 0, len(c.repoData))
	for _, rd := range c.repoData {
		data = append(data, rd)
	}

	cacheFile := filepath.Join(c.dataDir, "catalog-cache.json")
	cacheData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cacheFile, cacheData, 0644)
}

// loadBuiltinApps 加载内置应用
func (c *Catalog) loadBuiltinApps() {
	builtin := []CatalogEntry{
		{
			ID: "jellyfin", Name: "jellyfin", DisplayName: "Jellyfin",
			Description: "开源媒体服务器，支持电影、电视剧、音乐播放",
			Category: "Media", Icon: "🎬", Version: "latest", LatestVersion: "latest",
			Author: "Jellyfin Team", Website: "https://jellyfin.org",
			License: "GPL-2.0", Tags: []string{"media", "streaming", "video"},
			Rating: 4.8, Downloads: 15000,
			Containers: []ContainerSpec{
				{Name: "jellyfin", Image: "jellyfin/jellyfin:latest",
					Ports: []PortSpec{{Name: "web", ContainerPort: 8096, DefaultHostPort: 8096, Protocol: "tcp"}},
					Volumes: []VolumeSpec{
						{Name: "config", ContainerPath: "/config", DefaultHostPath: "/opt/nas/apps/jellyfin/config"},
						{Name: "media", ContainerPath: "/media", DefaultHostPath: "/opt/nas/media", ReadOnly: true},
					},
					RestartPolicy: "unless-stopped",
				},
			},
			RepositoryID: "official", UpdatedAt: time.Now(),
		},
		{
			ID: "nextcloud", Name: "nextcloud", DisplayName: "Nextcloud",
			Description: "私有云存储和协作平台",
			Category: "Productivity", Icon: "☁️", Version: "latest", LatestVersion: "latest",
			Author: "Nextcloud GmbH", Website: "https://nextcloud.com",
			License: "AGPL-3.0", Tags: []string{"cloud", "storage", "sync"},
			Rating: 4.6, Downloads: 12000,
			Containers: []ContainerSpec{
				{Name: "nextcloud", Image: "nextcloud:latest",
					Ports: []PortSpec{{Name: "web", ContainerPort: 80, DefaultHostPort: 8080, Protocol: "tcp"}},
					Volumes: []VolumeSpec{
						{Name: "data", ContainerPath: "/var/www/html", DefaultHostPath: "/opt/nas/apps/nextcloud/data"},
					},
					RestartPolicy: "unless-stopped",
				},
			},
			Dependencies: []string{"postgres", "redis"},
			RepositoryID: "official", UpdatedAt: time.Now(),
		},
		{
			ID: "homeassistant", Name: "homeassistant", DisplayName: "Home Assistant",
			Description: "开源智能家居平台，支持数千种设备集成",
			Category: "Smart Home", Icon: "🏠", Version: "stable", LatestVersion: "stable",
			Author: "Home Assistant", Website: "https://www.home-assistant.io",
			License: "Apache-2.0", Tags: []string{"iot", "smart-home", "automation"},
			Rating: 4.9, Downloads: 20000,
			Containers: []ContainerSpec{
				{Name: "homeassistant", Image: "homeassistant/home-assistant:stable",
					Privileged: true, NetworkMode: "host",
					Volumes: []VolumeSpec{
						{Name: "config", ContainerPath: "/config", DefaultHostPath: "/opt/nas/apps/homeassistant/config"},
					},
					RestartPolicy: "unless-stopped",
				},
			},
			RepositoryID: "official", UpdatedAt: time.Now(),
		},
		{
			ID: "postgres", Name: "postgres", DisplayName: "PostgreSQL",
			Description: "强大的开源关系型数据库",
			Category: "Database", Icon: "🗄️", Version: "15", LatestVersion: "16",
			Author: "PostgreSQL", Website: "https://www.postgresql.org",
			License: "PostgreSQL", Tags: []string{"database", "sql", "rdbms"},
			Rating: 4.7, Downloads: 18000,
			Containers: []ContainerSpec{
				{Name: "postgres", Image: "postgres:15",
					Ports: []PortSpec{{Name: "db", ContainerPort: 5432, DefaultHostPort: 5432, Protocol: "tcp"}},
					Volumes: []VolumeSpec{
						{Name: "data", ContainerPath: "/var/lib/postgresql/data", DefaultHostPath: "/opt/nas/apps/postgres/data"},
					},
					RestartPolicy: "unless-stopped",
				},
			},
			RepositoryID: "official", UpdatedAt: time.Now(),
		},
		{
			ID: "redis", Name: "redis", DisplayName: "Redis",
			Description: "高性能内存数据库和缓存服务器",
			Category: "Database", Icon: "⚡", Version: "latest", LatestVersion: "latest",
			Author: "Redis Ltd", Website: "https://redis.io",
			License: "BSD-3-Clause", Tags: []string{"database", "cache", "nosql"},
			Rating: 4.8, Downloads: 16000,
			Containers: []ContainerSpec{
				{Name: "redis", Image: "redis:latest",
					Ports: []PortSpec{{Name: "db", ContainerPort: 6379, DefaultHostPort: 6379, Protocol: "tcp"}},
					Volumes: []VolumeSpec{
						{Name: "data", ContainerPath: "/data", DefaultHostPath: "/opt/nas/apps/redis/data"},
					},
					RestartPolicy: "unless-stopped",
				},
			},
			RepositoryID: "official", UpdatedAt: time.Now(),
		},
		{
			ID: "plex", Name: "plex", DisplayName: "Plex",
			Description: "流行媒体服务器，支持电影、电视剧、音乐播放",
			Category: "Media", Icon: "🎥", Version: "latest", LatestVersion: "latest",
			Author: "Plex Inc", Website: "https://www.plex.tv",
			License: "Proprietary", Tags: []string{"media", "streaming"},
			Rating: 4.5, Downloads: 14000,
			Containers: []ContainerSpec{
				{Name: "plex", Image: "plexinc/pms-docker:latest",
					Ports: []PortSpec{{Name: "web", ContainerPort: 32400, DefaultHostPort: 32400, Protocol: "tcp"}},
					Volumes: []VolumeSpec{
						{Name: "config", ContainerPath: "/config", DefaultHostPath: "/opt/nas/apps/plex/config"},
						{Name: "media", ContainerPath: "/data", DefaultHostPath: "/opt/nas/media", ReadOnly: true},
					},
					RestartPolicy: "unless-stopped",
				},
			},
			Conflicts:     []string{"jellyfin"},
			RepositoryID:  "official", UpdatedAt: time.Now(),
		},
		{
			ID: "transmission", Name: "transmission", DisplayName: "Transmission",
			Description: "轻量级 BitTorrent 客户端",
			Category: "Download", Icon: "📥", Version: "latest", LatestVersion: "latest",
			Author: "Transmission", Website: "https://transmissionbt.com",
			License: "GPL-2.0", Tags: []string{"download", "torrent", "bt"},
			Rating: 4.4, Downloads: 10000,
			Containers: []ContainerSpec{
				{Name: "transmission", Image: "linuxserver/transmission:latest",
					Ports: []PortSpec{
						{Name: "web", ContainerPort: 9091, DefaultHostPort: 9091, Protocol: "tcp"},
						{Name: "bt", ContainerPort: 51413, DefaultHostPort: 51413, Protocol: "tcp"},
					},
					Volumes: []VolumeSpec{
						{Name: "config", ContainerPath: "/config", DefaultHostPath: "/opt/nas/apps/transmission/config"},
						{Name: "downloads", ContainerPath: "/downloads", DefaultHostPath: "/opt/nas/downloads"},
					},
					RestartPolicy: "unless-stopped",
				},
			},
			RepositoryID: "official", UpdatedAt: time.Now(),
		},
		{
			ID: "pihole", Name: "pihole", DisplayName: "Pi-hole",
			Description: "网络级广告拦截器，DNS服务器",
			Category: "Network", Icon: "🛡️", Version: "latest", LatestVersion: "latest",
			Author: "Pi-hole LLC", Website: "https://pi-hole.net",
			License: "EUPL-1.2", Tags: []string{"network", "dns", "adblock"},
			Rating: 4.7, Downloads: 11000,
			Containers: []ContainerSpec{
				{Name: "pihole", Image: "pihole/pihole:latest",
					Ports: []PortSpec{
						{Name: "dns-tcp", ContainerPort: 53, DefaultHostPort: 53, Protocol: "tcp"},
						{Name: "dns-udp", ContainerPort: 53, DefaultHostPort: 53, Protocol: "udp"},
						{Name: "web", ContainerPort: 80, DefaultHostPort: 8081, Protocol: "tcp"},
					},
					Volumes: []VolumeSpec{
						{Name: "config", ContainerPath: "/etc/pihole", DefaultHostPath: "/opt/nas/apps/pihole/etc"},
					},
					RestartPolicy: "unless-stopped",
				},
			},
			Conflicts:    []string{"adguard"},
			RepositoryID: "official", UpdatedAt: time.Now(),
		},
		{
			ID: "nginx", Name: "nginx", DisplayName: "Nginx",
			Description: "高性能Web服务器和反向代理",
			Category: "Network", Icon: "🌐", Version: "latest", LatestVersion: "latest",
			Author: "NGINX", Website: "https://nginx.org",
			License: "BSD-2-Clause", Tags: []string{"network", "web", "proxy"},
			Rating: 4.8, Downloads: 17000,
			Containers: []ContainerSpec{
				{Name: "nginx", Image: "nginx:latest",
					Ports: []PortSpec{
						{Name: "http", ContainerPort: 80, DefaultHostPort: 8888, Protocol: "tcp"},
						{Name: "https", ContainerPort: 443, DefaultHostPort: 8443, Protocol: "tcp"},
					},
					Volumes: []VolumeSpec{
						{Name: "config", ContainerPath: "/etc/nginx", DefaultHostPath: "/opt/nas/apps/nginx/config"},
						{Name: "html", ContainerPath: "/usr/share/nginx/html", DefaultHostPath: "/opt/nas/apps/nginx/html"},
					},
					RestartPolicy: "unless-stopped",
				},
			},
			RepositoryID: "official", UpdatedAt: time.Now(),
		},
		{
			ID: "syncthing", Name: "syncthing", DisplayName: "Syncthing",
			Description: "开源文件同步工具，支持多设备间实时同步",
			Category: "Productivity", Icon: "🔄", Version: "latest", LatestVersion: "latest",
			Author: "Syncthing Foundation", Website: "https://syncthing.net",
			License: "MPL-2.0", Tags: []string{"sync", "file", "p2p"},
			Rating: 4.6, Downloads: 9000,
			Containers: []ContainerSpec{
				{Name: "syncthing", Image: "syncthing/syncthing:latest",
					Ports: []PortSpec{
						{Name: "web", ContainerPort: 8384, DefaultHostPort: 8384, Protocol: "tcp"},
						{Name: "sync", ContainerPort: 22000, DefaultHostPort: 22000, Protocol: "tcp"},
					},
					Volumes: []VolumeSpec{
						{Name: "data", ContainerPath: "/var/syncthing", DefaultHostPath: "/opt/nas/apps/syncthing/data"},
					},
					RestartPolicy: "unless-stopped",
				},
			},
			RepositoryID: "official", UpdatedAt: time.Now(),
		},
		{
			ID: "qdrant", Name: "qdrant", DisplayName: "Qdrant",
			Description: "高性能向量数据库，用于AI应用",
			Category: "AI", Icon: "🧠", Version: "latest", LatestVersion: "latest",
			Author: "Qdrant", Website: "https://qdrant.tech",
			License: "Apache-2.0", Tags: []string{"ai", "vector", "database"},
			Rating: 4.5, Downloads: 5000,
			Containers: []ContainerSpec{
				{Name: "qdrant", Image: "qdrant/qdrant:latest",
					Ports: []PortSpec{
						{Name: "web", ContainerPort: 6333, DefaultHostPort: 6333, Protocol: "tcp"},
						{Name: "grpc", ContainerPort: 6334, DefaultHostPort: 6334, Protocol: "tcp"},
					},
					Volumes: []VolumeSpec{
						{Name: "data", ContainerPath: "/qdrant/storage", DefaultHostPath: "/opt/nas/apps/qdrant/data"},
					},
					RestartPolicy: "unless-stopped",
				},
			},
			RepositoryID: "official", UpdatedAt: time.Now(),
		},
	}

	c.mu.Lock()
	defer c.mu.Unlock()

	for i := range builtin {
		c.entries[builtin[i].ID] = &builtin[i]
	}
}

// matchTags 匹配标签
func matchTags(tags []string, query string) bool {
	for _, tag := range tags {
		if strings.Contains(strings.ToLower(tag), query) {
			return true
		}
	}
	return false
}
