// Package appstore 提供应用商店功能（参考 Unraid Community Apps 设计）
package appstore

import (
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ========== 应用模板定义 ==========

// AppTemplate 应用模板
type AppTemplate struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    string            `json:"category"`
	Icon        string            `json:"icon"`
	Version     string            `json:"version"`
	DockerImage string            `json:"docker_image"`
	Ports       []PortMapping     `json:"ports"`
	Volumes     []VolumeMapping   `json:"volumes"`
	Environment map[string]string `json:"environment"`
	Labels      map[string]string `json:"labels"`
	Network     string            `json:"network"`
	Restart     string            `json:"restart"`
	Author      string            `json:"author"`
	Source      string            `json:"source"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// PortMapping 端口映射
type PortMapping struct {
	ContainerPort int    `json:"container_port"`
	HostPort      int    `json:"host_port"`
	Protocol      string `json:"protocol"`
	Description   string `json:"description"`
}

// VolumeMapping 卷映射
type VolumeMapping struct {
	ContainerPath string `json:"container_path"`
	HostPath      string `json:"host_path"`
	ReadOnly      bool   `json:"read_only"`
	Description   string `json:"description"`
}

// ========== 预置应用模板 ==========

// DefaultTemplates 默认应用模板列表
var DefaultTemplates = []AppTemplate{
	{
		ID:          "plex",
		Name:        "Plex Media Server",
		Description: "媒体服务器，支持电影、音乐、照片串流",
		Category:    "media",
		Icon:        "plex.png",
		Version:     "latest",
		DockerImage: "plexinc/pms-docker:latest",
		Ports: []PortMapping{
			{ContainerPort: 32400, HostPort: 32400, Protocol: "tcp", Description: "Web界面"},
		},
		Volumes: []VolumeMapping{
			{ContainerPath: "/config", HostPath: "/apps/plex/config", Description: "配置"},
			{ContainerPath: "/data", HostPath: "/media", ReadOnly: true, Description: "媒体库"},
		},
		Environment: map[string]string{"PLEX_CLAIM": ""},
		Network:     "host",
		Restart:     "unless-stopped",
		Author:      "Plex Inc.",
		Source:      "https://hub.docker.com/r/plexinc/pms-docker",
	},
	{
		ID:          "jellyfin",
		Name:        "Jellyfin",
		Description: "开源媒体服务器",
		Category:    "media",
		Icon:        "jellyfin.png",
		Version:     "latest",
		DockerImage: "jellyfin/jellyfin:latest",
		Ports: []PortMapping{
			{ContainerPort: 8096, HostPort: 8096, Protocol: "tcp", Description: "Web界面"},
		},
		Volumes: []VolumeMapping{
			{ContainerPath: "/config", HostPath: "/apps/jellyfin/config", Description: "配置"},
			{ContainerPath: "/cache", HostPath: "/apps/jellyfin/cache", Description: "缓存"},
			{ContainerPath: "/media", HostPath: "/media", ReadOnly: true, Description: "媒体库"},
		},
		Network: "bridge",
		Restart: "unless-stopped",
		Author:  "Jellyfin Project",
		Source:  "https://hub.docker.com/r/jellyfin/jellyfin",
	},
	{
		ID:          "nextcloud",
		Name:        "Nextcloud",
		Description: "私有云存储和协作平台",
		Category:    "productivity",
		Icon:        "nextcloud.png",
		Version:     "latest",
		DockerImage: "nextcloud:latest",
		Ports: []PortMapping{
			{ContainerPort: 80, HostPort: 8080, Protocol: "tcp", Description: "Web界面"},
		},
		Volumes: []VolumeMapping{
			{ContainerPath: "/var/www/html", HostPath: "/apps/nextcloud/data", Description: "数据"},
		},
		Network: "bridge",
		Restart: "unless-stopped",
		Author:  "Nextcloud GmbH",
		Source:  "https://hub.docker.com/_/nextcloud",
	},
	{
		ID:          "homeassistant",
		Name:        "Home Assistant",
		Description: "智能家居自动化平台",
		Category:    "automation",
		Icon:        "homeassistant.png",
		Version:     "stable",
		DockerImage: "homeassistant/home-assistant:stable",
		Ports: []PortMapping{
			{ContainerPort: 8123, HostPort: 8123, Protocol: "tcp", Description: "Web界面"},
		},
		Volumes: []VolumeMapping{
			{ContainerPath: "/config", HostPath: "/apps/homeassistant/config", Description: "配置"},
		},
		Network: "host",
		Restart: "unless-stopped",
		Author:  "Home Assistant",
		Source:  "https://hub.docker.com/r/homeassistant/home-assistant",
	},
}

// ========== 应用商店管理 ==========

// Store 商店管理器
type Store struct {
	mu        sync.RWMutex
	templates map[string]AppTemplate
	installed map[string]*InstalledApp
}

// InstalledApp 已安装应用
type InstalledApp struct {
	Template    AppTemplate `json:"template"`
	ContainerID string      `json:"container_id"`
	Status      string      `json:"status"`
	InstalledAt time.Time   `json:"installed_at"`
	CustomPorts []PortMapping `json:"custom_ports,omitempty"`
}

// NewStore 创建商店
func NewStore() *Store {
	s := &Store{
		templates: make(map[string]AppTemplate),
		installed: make(map[string]*InstalledApp),
	}
	for _, t := range DefaultTemplates {
		s.templates[t.ID] = t
	}
	return s
}

// ListTemplates 列出模板
func (s *Store) ListTemplates(category string) []AppTemplate {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []AppTemplate
	for _, t := range s.templates {
		if category == "" || t.Category == category {
			result = append(result, t)
		}
	}
	return result
}

// GetTemplate 获取模板
func (s *Store) GetTemplate(id string) (AppTemplate, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	t, ok := s.templates[id]
	return t, ok
}

// Install 安装应用
func (s *Store) Install(ctx context.Context, templateID string, customPorts []PortMapping) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	tmpl, ok := s.templates[templateID]
	if !ok {
		return fmt.Errorf("模板不存在: %s", templateID)
	}

	if _, exists := s.installed[templateID]; exists {
		return fmt.Errorf("应用已安装: %s", templateID)
	}

	s.installed[templateID] = &InstalledApp{
		Template:    tmpl,
		Status:      "running",
		InstalledAt: time.Now(),
		CustomPorts: customPorts,
	}

	return nil
}

// Uninstall 卸载应用
func (s *Store) Uninstall(templateID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, ok := s.installed[templateID]; !ok {
		return fmt.Errorf("应用未安装: %s", templateID)
	}

	delete(s.installed, templateID)
	return nil
}

// ListInstalled 列出已安装应用
func (s *Store) ListInstalled() []*InstalledApp {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*InstalledApp, 0, len(s.installed))
	for _, app := range s.installed {
		result = append(result, app)
	}
	return result
}

// Export 导出配置
func (s *Store) Export() ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	data := map[string]interface{}{
		"templates": s.templates,
		"installed": s.installed,
	}
	return json.MarshalIndent(data, "", "  ")
}