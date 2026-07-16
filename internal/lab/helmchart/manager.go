// Package helmchart 提供 Helm Chart 应用商店功能
// 支持 Helm Chart 仓库管理、Chart 搜索、安装和卸载
package helmchart

import (
	"fmt"
	"sync"
	"time"
)

// Repository 表示 Helm Chart 仓库.
type Repository struct {
	Name        string `json:"name"`         // 仓库名称
	URL         string `json:"url"`          // 仓库 URL
	Description string `json:"description"`  // 描述
	IsBuiltin   bool   `json:"is_builtin"`   // 是否内置仓库
	Status      string `json:"status"`       // active/error/syncing
	ChartCount  int    `json:"chart_count"`  // Chart 数量
	LastSyncAt  int64  `json:"last_sync_at"` // 最后同步时间
	CreatedAt   int64  `json:"created_at"`
}

// Chart 表示一个 Helm Chart.
type Chart struct {
	Name        string   `json:"name"`        // Chart 名称
	Version     string   `json:"version"`     // 版本
	AppVersion  string   `json:"app_version"` // 应用版本
	Repository  string   `json:"repository"`  // 所属仓库
	Description string   `json:"description"` // 描述
	Keywords    []string `json:"keywords"`    // 关键词
	Home        string   `json:"home"`        // 主页
	Icon        string   `json:"icon"`        // 图标 URL
	Deprecated  bool     `json:"deprecated"`  // 是否已弃用
	CreatedAt   int64    `json:"created_at"`
}

// InstalledChart 表示已安装的 Chart.
type InstalledChart struct {
	Name        string            `json:"name"`      // 安装名称
	Chart       string            `json:"chart"`     // Chart 名称
	Version     string            `json:"version"`   // 安装的版本
	Namespace   string            `json:"namespace"` // K8s 命名空间
	Status      string            `json:"status"`    // deployed/failed/pending
	Values      map[string]string `json:"values"`    // 自定义值
	Notes       string            `json:"notes"`     // 安装说明
	InstalledAt int64             `json:"installed_at"`
	UpdatedAt   int64             `json:"updated_at"`
}

// Manager 管理 Helm Chart 应用商店.
type Manager struct {
	mu        sync.RWMutex
	repos     map[string]*Repository
	charts    map[string]*Chart // key: repo/name:version
	installed map[string]*InstalledChart
}

// NewManager 创建 Helm Chart 管理器.
func NewManager() *Manager {
	m := &Manager{
		repos:     make(map[string]*Repository),
		charts:    make(map[string]*Chart),
		installed: make(map[string]*InstalledChart),
	}

	// 添加内置仓库
	m.addBuiltinRepos()

	return m
}

func (m *Manager) addBuiltinRepos() {
	builtin := []struct {
		name, url, desc string
	}{
		{"stable", "https://charts.helm.sh/stable", "Helm 官方稳定版仓库"},
		{"bitnami", "https://charts.bitnami.com/bitnami", "Bitnami 应用仓库"},
		{"nas-apps", "https://charts.nas-os.io/stable", "NAS-OS 官方应用仓库"},
	}

	for _, r := range builtin {
		m.repos[r.name] = &Repository{
			Name:      r.name,
			URL:       r.url,
			IsBuiltin: true,
			Status:    "active",
			CreatedAt: time.Now().Unix(),
		}
	}
}

// AddRepo 添加 Chart 仓库.
func (m *Manager) AddRepo(name, url, description string) (*Repository, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "" {
		return nil, fmt.Errorf("仓库名称不能为空")
	}

	if url == "" {
		return nil, fmt.Errorf("仓库 URL 不能为空")
	}

	if _, exists := m.repos[name]; exists {
		return nil, fmt.Errorf("仓库 %s 已存在", name)
	}

	repo := &Repository{
		Name:        name,
		URL:         url,
		Description: description,
		Status:      "active",
		CreatedAt:   time.Now().Unix(),
	}

	m.repos[name] = repo

	return repo, nil
}

// RemoveRepo 删除仓库.
func (m *Manager) RemoveRepo(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	repo, exists := m.repos[name]
	if !exists {
		return fmt.Errorf("仓库 %s 不存在", name)
	}

	if repo.IsBuiltin {
		return fmt.Errorf("不能删除内置仓库 %s", name)
	}

	// 删除仓库中的所有 Chart
	for key, chart := range m.charts {
		if chart.Repository == name {
			delete(m.charts, key)
		}
	}

	delete(m.repos, name)

	return nil
}

// ListRepos 列出所有仓库.
func (m *Manager) ListRepos() []*Repository {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Repository, 0, len(m.repos))
	for _, repo := range m.repos {
		result = append(result, repo)
	}
	return result
}

// SearchChart 搜索 Chart.
func (m *Manager) SearchChart(keyword string) []*Chart {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Chart
	for _, chart := range m.charts {
		if matchChart(chart, keyword) {
			result = append(result, chart)
		}
	}
	return result
}

func matchChart(chart *Chart, keyword string) bool {
	if keyword == "" {
		return true
	}

	// 检查名称
	if contains(chart.Name, keyword) {
		return true
	}

	// 检查描述
	if contains(chart.Description, keyword) {
		return true
	}

	// 检查关键词
	for _, kw := range chart.Keywords {
		if contains(kw, keyword) {
			return true
		}
	}

	return false
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0)
}

// InstallChart 安装 Chart.
func (m *Manager) InstallChart(name, chart, version, namespace string, values map[string]string) (*InstalledChart, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if name == "" {
		return nil, fmt.Errorf("安装名称不能为空")
	}

	if chart == "" {
		return nil, fmt.Errorf("Chart 名称不能为空")
	}

	if _, exists := m.installed[name]; exists {
		return nil, fmt.Errorf("已存在同名安装: %s", name)
	}

	if namespace == "" {
		namespace = "default"
	}

	installed := &InstalledChart{
		Name:        name,
		Chart:       chart,
		Version:     version,
		Namespace:   namespace,
		Status:      "deployed",
		Values:      values,
		InstalledAt: time.Now().Unix(),
		UpdatedAt:   time.Now().Unix(),
	}

	m.installed[name] = installed

	return installed, nil
}

// UninstallChart 卸载 Chart.
func (m *Manager) UninstallChart(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.installed[name]; !exists {
		return fmt.Errorf("未找到安装: %s", name)
	}

	delete(m.installed, name)

	return nil
}

// ListInstalled 列出已安装的 Chart.
func (m *Manager) ListInstalled() []*InstalledChart {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*InstalledChart, 0, len(m.installed))
	for _, chart := range m.installed {
		result = append(result, chart)
	}
	return result
}

// GetInstalled 获取已安装的 Chart 详情.
func (m *Manager) GetInstalled(name string) (*InstalledChart, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	chart, exists := m.installed[name]
	if !exists {
		return nil, fmt.Errorf("未找到安装: %s", name)
	}

	return chart, nil
}

// UpgradeChart 升级已安装的 Chart.
func (m *Manager) UpgradeChart(name, newVersion string, values map[string]string) (*InstalledChart, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	chart, exists := m.installed[name]
	if !exists {
		return nil, fmt.Errorf("未找到安装: %s", name)
	}

	if newVersion != "" {
		chart.Version = newVersion
	}

	if values != nil {
		chart.Values = values
	}

	chart.UpdatedAt = time.Now().Unix()

	return chart, nil
}

// SyncRepo 同步仓库.
func (m *Manager) SyncRepo(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	repo, exists := m.repos[name]
	if !exists {
		return fmt.Errorf("仓库 %s 不存在", name)
	}

	repo.Status = "syncing"

	// 模拟同步过程
	go func() {
		time.Sleep(2 * time.Second)
		m.mu.Lock()
		repo.Status = "active"
		repo.LastSyncAt = time.Now().Unix()
		repo.ChartCount = len(m.charts)
		m.mu.Unlock()
	}()

	return nil
}

// GetStats 获取应用商店统计信息.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return map[string]interface{}{
		"repository_count": len(m.repos),
		"chart_count":      len(m.charts),
		"installed_count":  len(m.installed),
	}
}
