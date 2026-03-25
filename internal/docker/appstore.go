package docker

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// AppTemplate 应用模板
type AppTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Icon        string            `json:"icon"`
	Version     string            `json:"version"`
	Image       string            `json:"image"`
	Ports       []PortConfig      `json:"ports"`
	Volumes     []VolumeConfig    `json:"volumes"`
	Environment map[string]string `json:"environment"`
	Compose     string            `json:"compose,omitempty"` // Docker Compose 模板
	Notes       string            `json:"notes"`
	Website     string            `json:"website"`
	Source      string            `json:"source"`
	// 版本管理相关字段
	VersionHistory   []TemplateVersion `json:"versionHistory,omitempty"`
	CurrentVersion   string            `json:"currentVersion"`
	MinNASVersion    string            `json:"minNasVersion,omitempty"`
	Changelog        string            `json:"changelog,omitempty"`
	UpdateURL        string            `json:"updateUrl,omitempty"`
	AutoUpdate       bool              `json:"autoUpdate"`
	LastCheckedAt    time.Time         `json:"lastCheckedAt,omitempty"`
	AvailableUpdate  string            `json:"availableUpdate,omitempty"`
	SupportedArchs   []string          `json:"supportedArchs,omitempty"`
	Requirements     *AppRequirements  `json:"requirements,omitempty"`
}

// TemplateVersion 模板版本信息
type TemplateVersion struct {
	Version      string    `json:"version"`
	ImageTag     string    `json:"imageTag"`
	ReleaseNotes string    `json:"releaseNotes"`
	PublishedAt  time.Time `json:"publishedAt"`
	Digest       string    `json:"digest"`
	Deprecated   bool      `json:"deprecated"`
	MinVersion   string    `json:"minVersion,omitempty"` // 最低兼容版本
}

// AppRequirements 应用系统要求
type AppRequirements struct {
	MinCPU     int    `json:"minCpu"`     // 最小 CPU 核心数
	MinMemory  int64  `json:"minMemory"`  // 最小内存 (MB)
	MinStorage int64  `json:"minStorage"` // 最小存储 (MB)
	GPU        bool   `json:"gpu"`        // 是否需要 GPU
	Ports      []int  `json:"ports"`      // 需要开放的端口
}

// PortConfig 端口配置
type PortConfig struct {
	Port        int    `json:"port"`
	Protocol    string `json:"protocol"`
	Description string `json:"description"`
	Default     int    `json:"default"` // 默认主机端口
}

// VolumeConfig 卷配置
type VolumeConfig struct {
	ContainerPath string `json:"containerPath"`
	Description   string `json:"description"`
	Default       string `json:"default"` // 默认主机路径
}

// InstalledApp 已安装应用
type InstalledApp struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	DisplayName string            `json:"displayName"`
	TemplateID  string            `json:"templateId"`
	Version     string            `json:"version"`
	Status      string            `json:"status"`
	InstallTime time.Time         `json:"installTime"`
	Ports       map[int]int       `json:"ports"`   // 容器端口 -> 主机端口
	Volumes     map[string]string `json:"volumes"` // 容器路径 -> 主机路径
	Environment map[string]string `json:"environment"`
	ContainerID string            `json:"containerId"`
	ComposePath string            `json:"composePath"`
	// 版本管理
	InstalledVersion string    `json:"installedVersion"`
	LastUpdateTime   time.Time `json:"lastUpdateTime"`
	AutoUpdate       bool      `json:"autoUpdate"`
	// 备份信息
	LastBackupTime time.Time `json:"lastBackupTime,omitempty"`
	BackupCount     int       `json:"backupCount"`
}

// AppBackup 应用备份
type AppBackup struct {
	ID           string            `json:"id"`
	AppID        string            `json:"appId"`
	AppName      string            `json:"appName"`
	TemplateName string            `json:"templateName"`
	Version      string            `json:"version"`
	CreatedAt    time.Time         `json:"createdAt"`
	Size         int64             `json:"size"`
	Path         string            `json:"path"`
	Type         string            `json:"type"` // "full", "config", "data"
	Description  string            `json:"description"`
	Includes     []string          `json:"includes"`
	Config       map[string]string `json:"config,omitempty"`   // 配置快照
	VolumePaths  map[string]string `json:"volumePaths"`        // 卷路径映射
	ComposeFile  string            `json:"composeFile"`        // compose 文件内容
	Checksum     string            `json:"checksum"`           // 校验和
	Compressed   bool              `json:"compressed"`         // 是否压缩
}

// BackupManager 备份管理器
type BackupManager struct {
	store      *AppStore
	backupDir  string
	backups    map[string]*AppBackup
	dataFile   string
	maxBackups int // 最大备份数量
}

// NewBackupManager 创建备份管理器
func NewBackupManager(store *AppStore, dataDir string) (*BackupManager, error) {
	backupDir := filepath.Join(dataDir, "app-backups")
	dataFile := filepath.Join(dataDir, "backups.json")

	bm := &BackupManager{
		store:      store,
		backupDir:  backupDir,
		dataFile:   dataFile,
		backups:    make(map[string]*AppBackup),
		maxBackups: 10, // 默认保留最近10个备份
	}

	if err := os.MkdirAll(backupDir, 0750); err != nil {
		return nil, err
	}

	if err := bm.loadBackups(); err != nil {
		fmt.Printf("加载备份列表失败: %v\n", err)
	}

	return bm, nil
}

// UpdateCheckResult 更新检测结果
type UpdateCheckResult struct {
	AppID           string    `json:"appId"`
	AppName         string    `json:"appName"`
	CurrentVersion  string    `json:"currentVersion"`
	LatestVersion   string    `json:"latestVersion"`
	HasUpdate       bool      `json:"hasUpdate"`
	UpdateAvailable bool      `json:"updateAvailable"`
	ReleaseNotes    string    `json:"releaseNotes"`
	CheckedAt       time.Time `json:"checkedAt"`
	ImageDigest     string    `json:"imageDigest"`
	Size            int64     `json:"size"`
	Critical        bool      `json:"critical"` // 是否为关键更新
}

// UpdateAPI 更新检测 API 响应
type UpdateAPI struct {
	Store         *AppStore
	backupManager *BackupManager
}

// AppStore 应用商店
type AppStore struct {
	manager     *Manager
	templateDir string
	installDir  string
	dataFile    string
	templates   map[string]*AppTemplate
	installed   map[string]*InstalledApp
}

// NewAppStore 创建应用商店
func NewAppStore(mgr *Manager, dataDir string) (*AppStore, error) {
	templateDir := filepath.Join(dataDir, "app-templates")
	installDir := filepath.Join(dataDir, "apps")
	dataFile := filepath.Join(dataDir, "installed-apps.json")

	store := &AppStore{
		manager:     mgr,
		templateDir: templateDir,
		installDir:  installDir,
		dataFile:    dataFile,
		templates:   make(map[string]*AppTemplate),
		installed:   make(map[string]*InstalledApp),
	}

	// 创建目录
	if err := os.MkdirAll(templateDir, 0750); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(installDir, 0750); err != nil {
		return nil, err
	}

	// 加载内置模板
	store.loadBuiltinTemplates()

	// 加载已安装应用
	if err := store.loadInstalled(); err != nil {
		// 文件不存在不影响启动
		fmt.Printf("加载已安装应用列表失败: %v\n", err)
	}

	return store, nil
}

// loadBuiltinTemplates 加载内置模板
func (s *AppStore) loadBuiltinTemplates() {
	templates := []*AppTemplate{
		{
			ID:          "nextcloud",
			Name:        "nextcloud",
			DisplayName: "Nextcloud",
			Description: "私有云存储服务，支持文件同步、分享、在线办公",
			Category:    "Productivity",
			Icon:        "☁️",
			Version:     "latest",
			Image:       "nextcloud:latest",
			Ports: []PortConfig{
				{Port: 80, Protocol: "tcp", Description: "Web 界面", Default: 8080},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/var/www/html", Description: "数据目录", Default: "/opt/nas/apps/nextcloud/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  nextcloud:
    image: nextcloud:latest
    container_name: nextcloud
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:80"
    volumes:
      - {{.DataDir}}:/var/www/html
    environment:
      - NEXTCLOUD_TRUSTED_DOMAINS={{.TrustedDomains}}
`,
			Notes:   "首次访问需要创建管理员账户",
			Website: "https://nextcloud.com",
			Source:  "https://github.com/nextcloud",
		},
		{
			ID:          "jellyfin",
			Name:        "jellyfin",
			DisplayName: "Jellyfin",
			Description: "开源媒体服务器，支持电影、电视剧、音乐播放",
			Category:    "Media",
			Icon:        "🎬",
			Version:     "latest",
			Image:       "jellyfin/jellyfin:latest",
			Ports: []PortConfig{
				{Port: 8096, Protocol: "tcp", Description: "Web 界面", Default: 8096},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/config", Description: "配置目录", Default: "/opt/nas/apps/jellyfin/config"},
				{ContainerPath: "/cache", Description: "缓存目录", Default: "/opt/nas/apps/jellyfin/cache"},
				{ContainerPath: "/media", Description: "媒体目录", Default: "/opt/nas/media"},
			},
			Environment: map[string]string{
				"PUID": "1000",
				"PGID": "1000",
			},
			Compose: `version: '3'
services:
  jellyfin:
    image: jellyfin/jellyfin:latest
    container_name: jellyfin
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:8096"
    volumes:
      - {{.ConfigDir}}:/config
      - {{.CacheDir}}:/cache
      - {{.MediaDir}}:/media
    environment:
      - PUID=1000
      - PGID=1000
`,
			Notes:   "首次启动需要设置媒体库",
			Website: "https://jellyfin.org",
			Source:  "https://github.com/jellyfin/jellyfin",
		},
		{
			ID:          "homeassistant",
			Name:        "homeassistant",
			DisplayName: "Home Assistant",
			Description: "开源智能家居平台，支持数千种设备集成",
			Category:    "Smart Home",
			Icon:        "🏠",
			Version:     "stable",
			Image:       "homeassistant/home-assistant:stable",
			Ports: []PortConfig{
				{Port: 8123, Protocol: "tcp", Description: "Web 界面", Default: 8123},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/config", Description: "配置目录", Default: "/opt/nas/apps/homeassistant/config"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  homeassistant:
    image: homeassistant/home-assistant:stable
    container_name: homeassistant
    restart: unless-stopped
    privileged: true
    network_mode: host
    volumes:
      - {{.ConfigDir}}:/config
      - /etc/localtime:/etc/localtime:ro
`,
			Notes:   "使用 host 网络模式以支持设备发现",
			Website: "https://www.home-assistant.io",
			Source:  "https://github.com/home-assistant/core",
		},
		{
			ID:          "pihole",
			Name:        "pihole",
			DisplayName: "Pi-hole",
			Description: "网络级广告拦截器，DNS 服务器",
			Category:    "Network",
			Icon:        "🛡️",
			Version:     "latest",
			Image:       "pihole/pihole:latest",
			Ports: []PortConfig{
				{Port: 53, Protocol: "tcp", Description: "DNS (TCP)", Default: 53},
				{Port: 53, Protocol: "udp", Description: "DNS (UDP)", Default: 53},
				{Port: 80, Protocol: "tcp", Description: "Web 界面", Default: 8081},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/etc/pihole", Description: "配置目录", Default: "/opt/nas/apps/pihole/etc"},
				{ContainerPath: "/etc/dnsmasq.d", Description: "DNS 配置", Default: "/opt/nas/apps/pihole/dnsmasq"},
			},
			Environment: map[string]string{
				"TZ": "Asia/Shanghai",
			},
			Compose: `version: '3'
services:
  pihole:
    image: pihole/pihole:latest
    container_name: pihole
    restart: unless-stopped
    ports:
      - "{{.DNSPort}}:53/tcp"
      - "{{.DNSPort}}:53/udp"
      - "{{.WebPort}}:80"
    volumes:
      - {{.ConfigDir}}:/etc/pihole
      - {{.DnsmasqDir}}:/etc/dnsmasq.d
    environment:
      - TZ=Asia/Shanghai
      - WEBPASSWORD={{.WebPassword}}
`,
			Notes:   "设置路由器 DNS 指向 Pi-hole 以全局拦截广告",
			Website: "https://pi-hole.net",
			Source:  "https://github.com/pi-hole/pi-hole",
		},
		{
			ID:          "transmission",
			Name:        "transmission",
			DisplayName: "Transmission",
			Description: "轻量级 BitTorrent 客户端",
			Category:    "Download",
			Icon:        "📥",
			Version:     "latest",
			Image:       "linuxserver/transmission:latest",
			Ports: []PortConfig{
				{Port: 9091, Protocol: "tcp", Description: "Web 界面", Default: 9091},
				{Port: 51413, Protocol: "tcp", Description: "BT 端口", Default: 51413},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/config", Description: "配置目录", Default: "/opt/nas/apps/transmission/config"},
				{ContainerPath: "/downloads", Description: "下载目录", Default: "/opt/nas/downloads"},
			},
			Environment: map[string]string{
				"PUID": "1000",
				"PGID": "1000",
				"TZ":   "Asia/Shanghai",
			},
			Compose: `version: '3'
services:
  transmission:
    image: linuxserver/transmission:latest
    container_name: transmission
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:9091"
      - "{{.BTPort}}:51413"
      - "{{.BTPort}}:51413/udp"
    volumes:
      - {{.ConfigDir}}:/config
      - {{.DownloadDir}}:/downloads
    environment:
      - PUID=1000
      - PGID=1000
      - TZ=Asia/Shanghai
`,
			Notes:   "默认用户名/密码: admin/admin",
			Website: "https://transmissionbt.com",
			Source:  "https://github.com/transmission/transmission",
		},
		{
			ID:          "syncthing",
			Name:        "syncthing",
			DisplayName: "Syncthing",
			Description: "开源文件同步工具，支持多设备同步",
			Category:    "Productivity",
			Icon:        "🔄",
			Version:     "latest",
			Image:       "syncthing/syncthing:latest",
			Ports: []PortConfig{
				{Port: 8384, Protocol: "tcp", Description: "Web 界面", Default: 8384},
				{Port: 22000, Protocol: "tcp", Description: "同步端口", Default: 22000},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/var/syncthing", Description: "数据目录", Default: "/opt/nas/apps/syncthing/data"},
			},
			Environment: map[string]string{
				"PUID": "1000",
				"PGID": "1000",
			},
			Compose: `version: '3'
services:
  syncthing:
    image: syncthing/syncthing:latest
    container_name: syncthing
    restart: unless-stopped
    hostname: nas-syncthing
    ports:
      - "{{.WebPort}}:8384"
      - "{{.SyncPort}}:22000/tcp"
      - "{{.SyncPort}}:22000/udp"
    volumes:
      - {{.DataDir}}:/var/syncthing
    environment:
      - PUID=1000
      - PGID=1000
`,
			Notes:   "首次访问需要设置用户名密码",
			Website: "https://syncthing.net",
			Source:  "https://github.com/syncthing/syncthing",
		},
		{
			ID:          "gitea",
			Name:        "gitea",
			DisplayName: "Gitea",
			Description: "轻量级 Git 服务，自建代码仓库",
			Category:    "Development",
			Icon:        "🐙",
			Version:     "latest",
			Image:       "gitea/gitea:latest",
			Ports: []PortConfig{
				{Port: 3000, Protocol: "tcp", Description: "Web 界面", Default: 3000},
				{Port: 22, Protocol: "tcp", Description: "SSH", Default: 2222},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/data", Description: "数据目录", Default: "/opt/nas/apps/gitea/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  gitea:
    image: gitea/gitea:latest
    container_name: gitea
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:3000"
      - "{{.SSHPort}}:22"
    volumes:
      - {{.DataDir}}:/data
      - /etc/timezone:/etc/timezone:ro
      - /etc/localtime:/etc/localtime:ro
`,
			Notes:   "首次访问需要完成安装向导",
			Website: "https://gitea.io",
			Source:  "https://github.com/go-gitea/gitea",
		},
		{
			ID:          "vaultwarden",
			Name:        "vaultwarden",
			DisplayName: "Vaultwarden",
			Description: "Bitwarden 密码管理器服务端，管理所有密码",
			Category:    "Security",
			Icon:        "🔐",
			Version:     "latest",
			Image:       "vaultwarden/server:latest",
			Ports: []PortConfig{
				{Port: 80, Protocol: "tcp", Description: "Web 界面", Default: 8082},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/data", Description: "数据目录", Default: "/opt/nas/apps/vaultwarden/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  vaultwarden:
    image: vaultwarden/server:latest
    container_name: vaultwarden
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:80"
    volumes:
      - {{.DataDir}}:/data
    environment:
      - SIGNUPS_ALLOWED=true
`,
			Notes:   "建议配合 HTTPS 使用，可搭配 Bitwarden 客户端",
			Website: "https://github.com/dani-garcia/vaultwarden",
			Source:  "https://github.com/dani-garcia/vaultwarden",
		},
		{
			ID:          "immich",
			Name:        "immich",
			DisplayName: "Immich",
			Description: "自托管照片和视频备份方案，类似 Google Photos",
			Category:    "Media",
			Icon:        "📸",
			Version:     "latest",
			Image:       "ghcr.io/immich-app/immich-server:latest",
			Ports: []PortConfig{
				{Port: 2283, Protocol: "tcp", Description: "Web 界面", Default: 2283},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/usr/src/app/upload", Description: "上传目录", Default: "/opt/nas/apps/immich/upload"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  immich-server:
    image: ghcr.io/immich-app/immich-server:latest
    container_name: immich-server
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:3001"
    volumes:
      - {{.UploadDir}}:/usr/src/app/upload
      - /etc/localtime:/etc/localtime:ro
    environment:
      - DB_HOSTNAME=immich-db
      - DB_USERNAME=postgres
      - DB_PASSWORD=postgres
      - DB_DATABASE_NAME=immich
      - REDIS_HOSTNAME=immich-redis
    depends_on:
      - immich-db
      - immich-redis

  immich-db:
    image: tensorchord/pgvecto-rs:pg14-v0.2.0
    container_name: immich-db
    restart: unless-stopped
    volumes:
      - /opt/nas/apps/immich/db:/var/lib/postgresql/data
    environment:
      - POSTGRES_USER=postgres
      - POSTGRES_PASSWORD=postgres
      - POSTGRES_DB=immich

  immich-redis:
    image: redis:6.2-alpine
    container_name: immich-redis
    restart: unless-stopped
`,
			Notes:   "首次访问需要创建管理员账户，需要较多资源",
			Website: "https://immich.app",
			Source:  "https://github.com/immich-app/immich",
		},
		{
			ID:          "nginxproxymanager",
			Name:        "nginxproxymanager",
			DisplayName: "Nginx Proxy Manager",
			Description: "反向代理管理界面，SSL 证书自动申请",
			Category:    "Network",
			Icon:        "🌐",
			Version:     "latest",
			Image:       "jc21/nginx-proxy-manager:latest",
			Ports: []PortConfig{
				{Port: 80, Protocol: "tcp", Description: "HTTP", Default: 80},
				{Port: 443, Protocol: "tcp", Description: "HTTPS", Default: 443},
				{Port: 81, Protocol: "tcp", Description: "管理界面", Default: 8181},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/data", Description: "数据目录", Default: "/opt/nas/apps/npm/data"},
				{ContainerPath: "/etc/letsencrypt", Description: "证书目录", Default: "/opt/nas/apps/npm/letsencrypt"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  nginx-proxy-manager:
    image: jc21/nginx-proxy-manager:latest
    container_name: nginx-proxy-manager
    restart: unless-stopped
    ports:
      - "{{.HTTPPort}}:80"
      - "{{.HTTPSPort}}:443"
      - "{{.WebPort}}:81"
    volumes:
      - {{.DataDir}}:/data
      - {{.CertsDir}}:/etc/letsencrypt
`,
			Notes:   "默认登录: admin@example.com / changeme",
			Website: "https://nginxproxymanager.com",
			Source:  "https://github.com/NginxProxyManager/nginx-proxy-manager",
		},
		{
			ID:          "portainer",
			Name:        "portainer",
			DisplayName: "Portainer",
			Description: "Docker 容器管理界面，可视化管理容器",
			Category:    "Development",
			Icon:        "🐳",
			Version:     "latest",
			Image:       "portainer/portainer-ce:latest",
			Ports: []PortConfig{
				{Port: 9443, Protocol: "tcp", Description: "Web 界面", Default: 9443},
			},
			Volumes: []VolumeConfig{
				{ContainerPath: "/data", Description: "数据目录", Default: "/opt/nas/apps/portainer/data"},
			},
			Environment: map[string]string{},
			Compose: `version: '3'
services:
  portainer:
    image: portainer/portainer-ce:latest
    container_name: portainer
    restart: unless-stopped
    ports:
      - "{{.WebPort}}:9443"
    volumes:
      - {{.DataDir}}:/data
      - /var/run/docker.sock:/var/run/docker.sock
`,
			Notes:   "首次访问需要设置管理员密码",
			Website: "https://www.portainer.io",
			Source:  "https://github.com/portainer/portainer",
		},
	}

	for _, t := range templates {
		s.templates[t.ID] = t
	}
}

// loadInstalled 加载已安装应用
func (s *AppStore) loadInstalled() error {
	data, err := os.ReadFile(s.dataFile)
	if err != nil {
		return err
	}

	var apps []*InstalledApp
	if err := json.Unmarshal(data, &apps); err != nil {
		return err
	}

	for _, app := range apps {
		s.installed[app.ID] = app
	}
	return nil
}

// saveInstalled 保存已安装应用
func (s *AppStore) saveInstalled() error {
	apps := make([]*InstalledApp, 0, len(s.installed))
	for _, app := range s.installed {
		apps = append(apps, app)
	}

	data, err := json.MarshalIndent(apps, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(s.dataFile, data, 0640)
}

// ListTemplates 列出所有模板
func (s *AppStore) ListTemplates() []*AppTemplate {
	result := make([]*AppTemplate, 0, len(s.templates))
	for _, t := range s.templates {
		result = append(result, t)
	}
	return result
}

// GetTemplate 获取模板
func (s *AppStore) GetTemplate(id string) *AppTemplate {
	return s.templates[id]
}

// ListInstalled 列出已安装应用
func (s *AppStore) ListInstalled() []*InstalledApp {
	result := make([]*InstalledApp, 0, len(s.installed))
	for _, app := range s.installed {
		// 更新状态
		if container, err := s.manager.GetContainer(app.Name); err == nil {
			app.Status = container.State
		} else {
			app.Status = "stopped"
		}
		result = append(result, app)
	}
	return result
}

// GetInstalled 获取已安装应用
func (s *AppStore) GetInstalled(id string) *InstalledApp {
	app, ok := s.installed[id]
	if !ok {
		return nil
	}
	// 更新状态
	if container, err := s.manager.GetContainer(app.Name); err == nil {
		app.Status = container.State
	}
	return app
}

// InstallApp 安装应用
func (s *AppStore) InstallApp(templateID string, config map[string]interface{}) (*InstalledApp, error) {
	template, ok := s.templates[templateID]
	if !ok {
		return nil, fmt.Errorf("模板不存在: %s", templateID)
	}

	// 检查是否已安装
	if _, exists := s.installed[templateID]; exists {
		return nil, fmt.Errorf("应用已安装: %s", template.DisplayName)
	}

	appDir := filepath.Join(s.installDir, template.Name)
	if err := os.MkdirAll(appDir, 0750); err != nil {
		return nil, err
	}

	// 准备配置
	ports := make(map[int]int)
	volumes := make(map[string]string)
	env := make(map[string]string)

	// 从配置中提取端口映射
	for _, port := range template.Ports {
		key := fmt.Sprintf("port_%d", port.Port)
		if val, ok := config[key].(float64); ok {
			ports[port.Port] = int(val)
		} else {
			ports[port.Port] = port.Default
		}
	}

	// 从配置中提取卷映射
	for _, vol := range template.Volumes {
		key := fmt.Sprintf("vol_%s", strings.ReplaceAll(vol.ContainerPath, "/", "_"))
		if val, ok := config[key].(string); ok && val != "" {
			volumes[vol.ContainerPath] = val
		} else {
			volumes[vol.ContainerPath] = vol.Default
		}
	}

	// 复制环境变量
	for k, v := range template.Environment {
		env[k] = v
	}

	// 从配置中提取自定义环境变量
	if customEnv, ok := config["environment"].(map[string]interface{}); ok {
		for k, v := range customEnv {
			if str, ok := v.(string); ok {
				env[k] = str
			}
		}
	}

	// 生成 Docker Compose 文件
	composeContent := s.renderCompose(template, config)
	composePath := filepath.Join(appDir, "docker-compose.yml")
	if err := os.WriteFile(composePath, []byte(composeContent), 0640); err != nil {
		return nil, err
	}

	// 使用 docker-compose 启动
	cmd := exec.Command("docker-compose", "-f", composePath, "up", "-d")
	cmd.Dir = appDir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("启动失败: %w, %s", err, string(output))
	}

	// 获取容器 ID
	containers, _ := s.manager.ListContainers(false)
	var containerID string
	for _, c := range containers {
		if c.Name == template.Name {
			containerID = c.ID
			break
		}
	}

	// 记录安装信息
	app := &InstalledApp{
		ID:          templateID,
		Name:        template.Name,
		DisplayName: template.DisplayName,
		TemplateID:  templateID,
		Version:     template.Version,
		Status:      "running",
		InstallTime: time.Now(),
		Ports:       ports,
		Volumes:     volumes,
		Environment: env,
		ContainerID: containerID,
		ComposePath: composePath,
	}

	s.installed[templateID] = app
	if err := s.saveInstalled(); err != nil {
		fmt.Printf("保存安装信息失败: %v\n", err)
	}

	return app, nil
}

// renderCompose 渲染 Docker Compose 模板
func (s *AppStore) renderCompose(template *AppTemplate, config map[string]interface{}) string {
	compose := template.Compose
	if compose == "" {
		// 生成默认 compose
		compose = s.generateDefaultCompose(template, config)
	}

	// 替换变量
	for key, val := range config {
		placeholder := fmt.Sprintf("{{.%s}}", key)
		compose = strings.ReplaceAll(compose, placeholder, fmt.Sprintf("%v", val))
	}

	// 替换端口
	for _, port := range template.Ports {
		key := fmt.Sprintf("port_%d", port.Port)
		hostPort := port.Default
		if val, ok := config[key].(float64); ok {
			hostPort = int(val)
		}
		compose = strings.ReplaceAll(compose, "{{.WebPort}}", fmt.Sprintf("%d", hostPort))
	}

	// 替换路径
	for _, vol := range template.Volumes {
		key := fmt.Sprintf("vol_%s", strings.ReplaceAll(vol.ContainerPath, "/", "_"))
		hostPath := vol.Default
		if val, ok := config[key].(string); ok && val != "" {
			hostPath = val
		}
		// 根据 key 生成占位符名
		name := strings.TrimPrefix(vol.ContainerPath, "/")
		name = strings.ReplaceAll(name, "/", "")
		compose = strings.ReplaceAll(compose, fmt.Sprintf("{{.%sDir}}", name), hostPath)
	}

	return compose
}

// generateDefaultCompose 生成默认 compose
func (s *AppStore) generateDefaultCompose(template *AppTemplate, config map[string]interface{}) string {
	var ports []string
	for _, port := range template.Ports {
		hostPort := port.Default
		if val, ok := config[fmt.Sprintf("port_%d", port.Port)].(float64); ok {
			hostPort = int(val)
		}
		ports = append(ports, fmt.Sprintf("      - \"%d:%d\"", hostPort, port.Port))
	}

	var volumes []string
	for _, vol := range template.Volumes {
		hostPath := vol.Default
		if val, ok := config[fmt.Sprintf("vol_%s", strings.ReplaceAll(vol.ContainerPath, "/", "_"))].(string); ok && val != "" {
			hostPath = val
		}
		volumes = append(volumes, fmt.Sprintf("      - %s:%s", hostPath, vol.ContainerPath))
	}

	var env []string
	for k, v := range template.Environment {
		env = append(env, fmt.Sprintf("      - %s=%s", k, v))
	}

	return fmt.Sprintf(`version: '3'
services:
  %s:
    image: %s
    container_name: %s
    restart: unless-stopped
    ports:
%s
    volumes:
%s
    environment:
%s
`, template.Name, template.Image, template.Name, strings.Join(ports, "\n"), strings.Join(volumes, "\n"), strings.Join(env, "\n"))
}

// UninstallApp 卸载应用
func (s *AppStore) UninstallApp(id string, removeData bool) error {
	app, ok := s.installed[id]
	if !ok {
		return fmt.Errorf("应用未安装: %s", id)
	}

	// 停止并删除容器
	if app.ComposePath != "" {
		cmd := exec.Command("docker-compose", "-f", app.ComposePath, "down")
		if err := cmd.Run(); err != nil {
			log.Printf("docker-compose down 失败: %v", err)
		}
	} else if app.ContainerID != "" {
		if err := s.manager.RemoveContainer(app.ContainerID, true); err != nil {
			log.Printf("移除容器失败: %v", err)
		}
	}

	// 删除数据
	if removeData {
		appDir := filepath.Join(s.installDir, app.Name)
		_ = os.RemoveAll(appDir)
	}

	// 从记录中删除
	delete(s.installed, id)
	if err := s.saveInstalled(); err != nil {
		return err
	}

	return nil
}

// StartApp 启动应用
func (s *AppStore) StartApp(id string) error {
	app, ok := s.installed[id]
	if !ok {
		return fmt.Errorf("应用未安装: %s", id)
	}

	if app.ComposePath != "" {
		cmd := exec.Command("docker-compose", "-f", app.ComposePath, "start")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("启动失败: %w, %s", err, string(output))
		}
	} else {
		return s.manager.StartContainer(app.ContainerID)
	}

	app.Status = "running"
	if err := s.saveInstalled(); err != nil {
		log.Printf("保存安装状态失败: %v", err)
	}
	return nil
}

// StopApp 停止应用
func (s *AppStore) StopApp(id string) error {
	app, ok := s.installed[id]
	if !ok {
		return fmt.Errorf("应用未安装: %s", id)
	}

	if app.ComposePath != "" {
		cmd := exec.Command("docker-compose", "-f", app.ComposePath, "stop")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("停止失败: %w, %s", err, string(output))
		}
	} else {
		return s.manager.StopContainer(app.ContainerID, 10)
	}

	app.Status = "stopped"
	if err := s.saveInstalled(); err != nil {
		log.Printf("保存安装状态失败: %v", err)
	}
	return nil
}

// RestartApp 重启应用
func (s *AppStore) RestartApp(id string) error {
	app, ok := s.installed[id]
	if !ok {
		return fmt.Errorf("应用未安装: %s", id)
	}

	if app.ComposePath != "" {
		cmd := exec.Command("docker-compose", "-f", app.ComposePath, "restart")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("重启失败: %w, %s", err, string(output))
		}
	} else {
		return s.manager.RestartContainer(app.ContainerID, 10)
	}

	return nil
}

// UpdateApp 更新应用
func (s *AppStore) UpdateApp(id string) error {
	app, ok := s.installed[id]
	if !ok {
		return fmt.Errorf("应用未安装: %s", id)
	}

	template, ok := s.templates[id]
	if !ok {
		return fmt.Errorf("模板不存在: %s", id)
	}

	// 拉取最新镜像
	if err := s.manager.PullImage(template.Image); err != nil {
		return fmt.Errorf("拉取镜像失败: %w", err)
	}

	// 重新创建容器
	if app.ComposePath != "" {
		cmd := exec.Command("docker-compose", "-f", app.ComposePath, "up", "-d")
		output, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("更新失败: %w, %s", err, string(output))
		}
	}

	return nil
}

// GetAppStats 获取应用统计
func (s *AppStore) GetAppStats(id string) (map[string]interface{}, error) {
	app, ok := s.installed[id]
	if !ok {
		return nil, fmt.Errorf("应用未安装: %s", id)
	}

	if app.ContainerID == "" {
		return nil, fmt.Errorf("容器 ID 为空")
	}

	stats, err := s.manager.GetContainerStats(app.ContainerID)
	if err != nil {
		return nil, err
	}

	return map[string]interface{}{
		"cpuUsage":   stats.CPUUsage,
		"memUsage":   stats.MemUsage,
		"memLimit":   stats.MemLimit,
		"netRx":      stats.NetRX,
		"netTx":      stats.NetTX,
		"blockRead":  stats.BlockRead,
		"blockWrite": stats.BlockWrite,
	}, nil
}

// === 模板版本管理 ===

// GetTemplateVersionHistory 获取模板版本历史
func (s *AppStore) GetTemplateVersionHistory(templateID string) ([]TemplateVersion, error) {
	template := s.templates[templateID]
	if template == nil {
		return nil, fmt.Errorf("模板不存在: %s", templateID)
	}
	return template.VersionHistory, nil
}

// AddTemplateVersion 添加模板版本
func (s *AppStore) AddTemplateVersion(templateID string, version TemplateVersion) error {
	template := s.templates[templateID]
	if template == nil {
		return fmt.Errorf("模板不存在: %s", templateID)
	}

	// 检查版本是否已存在
	for _, v := range template.VersionHistory {
		if v.Version == version.Version {
			return fmt.Errorf("版本已存在: %s", version.Version)
		}
	}

	template.VersionHistory = append(template.VersionHistory, version)
	template.CurrentVersion = version.Version
	template.Version = version.Version
	template.LastCheckedAt = time.Now()

	return s.saveTemplate(template)
}

// SetTemplateAutoUpdate 设置模板自动更新
func (s *AppStore) SetTemplateAutoUpdate(templateID string, autoUpdate bool) error {
	template := s.templates[templateID]
	if template == nil {
		return fmt.Errorf("模板不存在: %s", templateID)
	}
	template.AutoUpdate = autoUpdate
	return s.saveTemplate(template)
}

// saveTemplate 保存模板到文件
func (s *AppStore) saveTemplate(template *AppTemplate) error {
	templatePath := filepath.Join(s.templateDir, template.ID+".json")
	data, err := json.MarshalIndent(template, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(templatePath, data, 0640)
}

// CheckTemplateUpdate 检查模板更新
func (s *AppStore) CheckTemplateUpdate(templateID string) (*UpdateCheckResult, error) {
	template := s.templates[templateID]
	if template == nil {
		return nil, fmt.Errorf("模板不存在: %s", templateID)
	}

	// 如果有更新URL，从远程检查
	if template.UpdateURL != "" {
		// 这里可以实现远程检查逻辑
		// 目前先返回基本信息
	}

	// 更新检查时间
	template.LastCheckedAt = time.Now()

	return &UpdateCheckResult{
		AppID:          templateID,
		AppName:        template.DisplayName,
		CurrentVersion: template.Version,
		LatestVersion:  template.AvailableUpdate,
		HasUpdate:      template.AvailableUpdate != "" && template.AvailableUpdate != template.Version,
		CheckedAt:      time.Now(),
	}, nil
}

// CheckAllTemplateUpdates 检查所有模板更新
func (s *AppStore) CheckAllTemplateUpdates() ([]*UpdateCheckResult, error) {
	var results []*UpdateCheckResult

	for id := range s.templates {
		result, err := s.CheckTemplateUpdate(id)
		if err != nil {
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// GetAppRequirements 获取应用系统要求
func (s *AppStore) GetAppRequirements(templateID string) (*AppRequirements, error) {
	template := s.templates[templateID]
	if template == nil {
		return nil, fmt.Errorf("模板不存在: %s", templateID)
	}
	return template.Requirements, nil
}

// === 备份管理器方法 ===

// loadBackups 加载备份列表
func (bm *BackupManager) loadBackups() error {
	data, err := os.ReadFile(bm.dataFile)
	if err != nil {
		return err
	}

	var backups []*AppBackup
	if err := json.Unmarshal(data, &backups); err != nil {
		return err
	}

	for _, b := range backups {
		bm.backups[b.ID] = b
	}
	return nil
}

// saveBackups 保存备份列表
func (bm *BackupManager) saveBackups() error {
	var backups []*AppBackup
	for _, b := range bm.backups {
		backups = append(backups, b)
	}

	data, err := json.MarshalIndent(backups, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(bm.dataFile, data, 0640)
}

// CreateBackup 创建应用备份
func (bm *BackupManager) CreateBackup(appID string, backupType string, description string) (*AppBackup, error) {
	app := bm.store.GetInstalled(appID)
	if app == nil {
		return nil, fmt.Errorf("应用未安装: %s", appID)
	}

	backupID := fmt.Sprintf("%s-%d", appID, time.Now().Unix())
	backupPath := filepath.Join(bm.backupDir, backupID)

	if err := os.MkdirAll(backupPath, 0750); err != nil {
		return nil, err
	}

	backup := &AppBackup{
		ID:           backupID,
		AppID:        appID,
		AppName:      app.DisplayName,
		TemplateName: app.TemplateID,
		Version:      app.Version,
		CreatedAt:    time.Now(),
		Path:         backupPath,
		Type:         backupType,
		Description:  description,
		VolumePaths:  app.Volumes,
		Compressed:   true,
	}

	// 保存配置
	backup.Config = app.Environment

	// 保存 Compose 文件
	if app.ComposePath != "" {
		composeData, err := os.ReadFile(app.ComposePath)
		if err == nil {
			backup.ComposeFile = string(composeData)
			// 同时保存到备份目录
			_ = os.WriteFile(filepath.Join(backupPath, "docker-compose.yml"), composeData, 0640)
		}
	}

	// 保存配置快照
	configPath := filepath.Join(backupPath, "config.json")
	configData, _ := json.MarshalIndent(map[string]interface{}{
		"appId":       app.ID,
		"name":        app.Name,
		"displayName": app.DisplayName,
		"templateId":  app.TemplateID,
		"version":     app.Version,
		"ports":       app.Ports,
		"volumes":     app.Volumes,
		"environment": app.Environment,
	}, "", "  ")
	_ = os.WriteFile(configPath, configData, 0640)

	// 备份卷数据
	if backupType == "full" || backupType == "data" {
		for containerPath, hostPath := range app.Volumes {
			if _, err := os.Stat(hostPath); err == nil {
				volBackupPath := filepath.Join(backupPath, "volumes", strings.ReplaceAll(containerPath, "/", "_"))
				_ = os.MkdirAll(filepath.Dir(volBackupPath), 0750)
				// 使用 tar 打包卷数据
				cmd := exec.Command("tar", "-czf", volBackupPath+".tar.gz", "-C", hostPath, ".")
				_ = cmd.Run()
				backup.Includes = append(backup.Includes, containerPath)
			}
		}
	}

	// 计算大小
	var totalSize int64
	filepath.Walk(backupPath, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			totalSize += info.Size()
		}
		return nil
	})
	backup.Size = totalSize

	// 计算校验和
	backup.Checksum = bm.calculateChecksum(backupPath)

	bm.backups[backupID] = backup

	// 更新应用的备份信息
	app.LastBackupTime = backup.CreatedAt
	app.BackupCount++
	_ = bm.store.saveInstalled()

	// 清理旧备份
	bm.cleanupOldBackups(appID)

	if err := bm.saveBackups(); err != nil {
		return nil, err
	}

	return backup, nil
}

// RestoreBackup 恢复应用备份
func (bm *BackupManager) RestoreBackup(backupID string) error {
	backup, ok := bm.backups[backupID]
	if !ok {
		return fmt.Errorf("备份不存在: %s", backupID)
	}

	// 检查应用是否存在
	app := bm.store.GetInstalled(backup.AppID)
	if app == nil {
		return fmt.Errorf("应用未安装: %s", backup.AppID)
	}

	// 停止应用
	_ = bm.store.StopApp(backup.AppID)

	// 恢复配置
	if backup.Config != nil {
		app.Environment = backup.Config
	}

	// 恢复 Compose 文件
	if backup.ComposeFile != "" && app.ComposePath != "" {
		_ = os.WriteFile(app.ComposePath, []byte(backup.ComposeFile), 0640)
	}

	// 恢复卷数据
	for _, containerPath := range backup.Includes {
		volBackupFile := filepath.Join(backup.Path, "volumes", strings.ReplaceAll(containerPath, "/", "_")+".tar.gz")
		hostPath := backup.VolumePaths[containerPath]

		if _, err := os.Stat(volBackupFile); err == nil && hostPath != "" {
			// 解压到目标目录
			_ = os.MkdirAll(hostPath, 0750)
			cmd := exec.Command("tar", "-xzf", volBackupFile, "-C", hostPath)
			_ = cmd.Run()
		}
	}

	// 保存配置
	_ = bm.store.saveInstalled()

	// 启动应用
	return bm.store.StartApp(backup.AppID)
}

// ListBackups 列出应用备份
func (bm *BackupManager) ListBackups(appID string) []*AppBackup {
	var backups []*AppBackup
	for _, b := range bm.backups {
		if appID == "" || b.AppID == appID {
			backups = append(backups, b)
		}
	}

	// 按时间排序（最新的在前）
	for i := 0; i < len(backups); i++ {
		for j := i + 1; j < len(backups); j++ {
			if backups[i].CreatedAt.Before(backups[j].CreatedAt) {
				backups[i], backups[j] = backups[j], backups[i]
			}
		}
	}

	return backups
}

// GetBackup 获取备份详情
func (bm *BackupManager) GetBackup(backupID string) (*AppBackup, error) {
	backup, ok := bm.backups[backupID]
	if !ok {
		return nil, fmt.Errorf("备份不存在: %s", backupID)
	}
	return backup, nil
}

// DeleteBackup 删除备份
func (bm *BackupManager) DeleteBackup(backupID string) error {
	backup, ok := bm.backups[backupID]
	if !ok {
		return fmt.Errorf("备份不存在: %s", backupID)
	}

	// 删除备份文件
	if err := os.RemoveAll(backup.Path); err != nil {
		return err
	}

	// 更新应用的备份计数
	app := bm.store.GetInstalled(backup.AppID)
	if app != nil && app.BackupCount > 0 {
		app.BackupCount--
		_ = bm.store.saveInstalled()
	}

	delete(bm.backups, backupID)
	return bm.saveBackups()
}

// cleanupOldBackups 清理旧备份
func (bm *BackupManager) cleanupOldBackups(appID string) {
	backups := bm.ListBackups(appID)
	if len(backups) <= bm.maxBackups {
		return
	}

	// 删除最旧的备份
	for i := bm.maxBackups; i < len(backups); i++ {
		_ = bm.DeleteBackup(backups[i].ID)
	}
}

// calculateChecksum 计算校验和
func (bm *BackupManager) calculateChecksum(path string) string {
	// 简单的校验和计算
	var hash uint32
	filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			hash += uint32(info.Size())
		}
		return nil
	})
	return fmt.Sprintf("%x", hash)
}

// ScheduleBackup 定时备份
func (bm *BackupManager) ScheduleBackup(appID string, interval time.Duration) {
	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for range ticker.C {
			_, err := bm.CreateBackup(appID, "config", "定时备份")
			if err != nil {
				fmt.Printf("定时备份失败 [%s]: %v\n", appID, err)
			}
		}
	}()
}

// ExportBackup 导出备份到指定路径
func (bm *BackupManager) ExportBackup(backupID string, destPath string) error {
	_, ok := bm.backups[backupID]
	if !ok {
		return fmt.Errorf("备份不存在: %s", backupID)
	}

	// 打包备份
	outputFile := filepath.Join(destPath, backupID+".tar.gz")
	cmd := exec.Command("tar", "-czf", outputFile, "-C", bm.backupDir, backupID)
	return cmd.Run()
}

// ImportBackup 从外部导入备份
func (bm *BackupManager) ImportBackup(backupFile string) (*AppBackup, error) {
	// 解压备份文件
	cmd := exec.Command("tar", "-xzf", backupFile, "-C", bm.backupDir)
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	// 获取备份 ID
	backupID := strings.TrimSuffix(filepath.Base(backupFile), ".tar.gz")

	// 加载新备份
	backupPath := filepath.Join(bm.backupDir, backupID, "config.json")
	configData, err := os.ReadFile(backupPath)
	if err != nil {
		return nil, err
	}

	var backup AppBackup
	if err := json.Unmarshal(configData, &backup); err != nil {
		return nil, err
	}

	backup.Path = filepath.Join(bm.backupDir, backupID)
	bm.backups[backup.ID] = &backup

	if err := bm.saveBackups(); err != nil {
		return nil, err
	}

	return &backup, nil
}

// === 更新检测 API ===

// CheckAppUpdate 检查应用更新
func (s *AppStore) CheckAppUpdate(appID string) (*UpdateCheckResult, error) {
	app := s.installed[appID]
	if app == nil {
		return nil, fmt.Errorf("应用未安装: %s", appID)
	}

	template := s.templates[app.TemplateID]
	if template == nil {
		return nil, fmt.Errorf("模板不存在: %s", app.TemplateID)
	}

	// 检查 Docker Hub 是否有新镜像
	latestTag, digest, size, err := s.fetchLatestImageInfo(template.Image)
	if err != nil {
		return nil, err
	}

	hasUpdate := latestTag != "" && latestTag != app.Version

	return &UpdateCheckResult{
		AppID:          appID,
		AppName:        app.DisplayName,
		CurrentVersion: app.Version,
		LatestVersion:  latestTag,
		HasUpdate:      hasUpdate,
		UpdateAvailable: hasUpdate,
		CheckedAt:      time.Now(),
		ImageDigest:    digest,
		Size:           size,
	}, nil
}

// fetchLatestImageInfo 获取最新镜像信息
func (s *AppStore) fetchLatestImageInfo(image string) (tag string, digest string, size int64, err error) {
	// 解析镜像名称
	parts := strings.Split(image, ":")
	imageName := parts[0]

	var namespace, name string
	if strings.Contains(imageName, "/") {
		nsParts := strings.SplitN(imageName, "/", 2)
		namespace = nsParts[0]
		name = nsParts[1]
	} else {
		namespace = "library"
		name = imageName
	}

	// 查询 Docker Hub API
	url := fmt.Sprintf("https://hub.docker.com/v2/repositories/%s/%s/tags/?page_size=5", namespace, name)

	// 使用 http 客户端请求
	resp, err := httpDefaultClient.Get(url)
	if err != nil {
		return "", "", 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return "", "", 0, fmt.Errorf("Docker Hub API 返回 %d", resp.StatusCode)
	}

	var result struct {
		Results []struct {
			Name        string `json:"name"`
			FullSize    int64  `json:"full_size"`
			LastUpdated string `json:"last_updated"`
			Images      []struct {
				Digest string `json:"digest"`
			} `json:"images"`
		} `json:"results"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", "", 0, err
	}

	// 返回最新的稳定版本标签（跳过 latest）
	for _, t := range result.Results {
		if t.Name != "latest" && !strings.Contains(t.Name, "-") {
			var d string
			if len(t.Images) > 0 {
				d = t.Images[0].Digest
			}
			return t.Name, d, t.FullSize, nil
		}
	}

	// 如果没有稳定版本，返回第一个
	if len(result.Results) > 0 {
		t := result.Results[0]
		var d string
		if len(t.Images) > 0 {
			d = t.Images[0].Digest
		}
		return t.Name, d, t.FullSize, nil
	}

	return "", "", 0, fmt.Errorf("未找到镜像标签")
}

// httpDefaultClient 默认 HTTP 客户端
var httpDefaultClient = &struct {
	Get func(url string) (*httpResponse, error)
}{
	Get: func(url string) (*httpResponse, error) {
		resp, err := http.Get(url)
		if err != nil {
			return nil, err
		}
		return &httpResponse{Response: resp}, nil
	},
}

type httpResponse struct {
	*http.Response
}

// CheckAllAppUpdates 检查所有应用更新
func (s *AppStore) CheckAllAppUpdates() ([]*UpdateCheckResult, error) {
	var results []*UpdateCheckResult

	for id := range s.installed {
		result, err := s.CheckAppUpdate(id)
		if err != nil {
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// GetAvailableUpdates 获取可用更新列表
func (s *AppStore) GetAvailableUpdates() []*UpdateCheckResult {
	results, _ := s.CheckAllAppUpdates()
	var updates []*UpdateCheckResult
	for _, r := range results {
		if r.HasUpdate {
			updates = append(updates, r)
		}
	}
	return updates
}

// SetAppAutoUpdate 设置应用自动更新
func (s *AppStore) SetAppAutoUpdate(appID string, autoUpdate bool) error {
	app := s.installed[appID]
	if app == nil {
		return fmt.Errorf("应用未安装: %s", appID)
	}
	app.AutoUpdate = autoUpdate
	return s.saveInstalled()
}

// PerformAutoUpdates 执行自动更新
func (s *AppStore) PerformAutoUpdates() ([]string, error) {
	var updated []string

	for id, app := range s.installed {
		if !app.AutoUpdate {
			continue
		}

		result, err := s.CheckAppUpdate(id)
		if err != nil || !result.HasUpdate {
			continue
		}

		if err := s.UpdateApp(id); err != nil {
			continue
		}

		app.Version = result.LatestVersion
		app.LastUpdateTime = time.Now()
		updated = append(updated, id)
	}

	if len(updated) > 0 {
		_ = s.saveInstalled()
	}

	return updated, nil
}
