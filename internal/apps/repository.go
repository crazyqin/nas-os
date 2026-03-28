// Package apps 应用仓库管理
// 管理本地和远程应用模板仓库
package apps

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"nas-os/pkg/app"
)

// Repository 应用仓库管理器
type Repository struct {
	mu sync.RWMutex

	dataDir   string             // 数据目录
	config    RepositoryConfig   // 仓库配置文件
	sources   map[string]*Source // 仓库源缓存
}

// Source 仓库源
type Source struct {
	Name      string            `json:"name"`
	URL       string            `json:"url"`
	Type      string            `json:"type"`      // local/remote
	Enabled   bool              `json:"enabled"`
	Priority  int               `json:"priority"`  // 优先级（越高越优先）
	UpdatedAt time.Time         `json:"updatedAt"`
	Cache     []*app.Template   `json:"cache"`     // 缓存的模板
}

// RepositoryConfig 仓库配置
type RepositoryConfig struct {
	Sources []RepositorySourceConfig `json:"sources"`
}

// RepositorySourceConfig 仓库源配置
type RepositorySourceConfig struct {
	Name     string `json:"name"`
	URL      string `json:"url"`
	Type     string `json:"type"`
	Enabled  bool   `json:"enabled"`
	Priority int    `json:"priority"`
}

// NewRepository 创建应用仓库管理器
func NewRepository(dataDir string) (*Repository, error) {
	configFile := filepath.Join(dataDir, "repository-config.json")

	repo := &Repository{
		dataDir: dataDir,
		config:  RepositoryConfig{Sources: []RepositorySourceConfig{}},
		sources: make(map[string]*Source),
	}

	// 加载配置
	if err := repo.loadConfig(configFile); err != nil {
		fmt.Printf("加载仓库配置失败: %v\n", err)
	}

	// 初始化本地仓库源
	if len(repo.config.Sources) == 0 {
		// 添加默认本地仓库
		repo.config.Sources = append(repo.config.Sources, RepositorySourceConfig{
			Name:     "local",
			URL:      filepath.Join(dataDir, "local-repo"),
			Type:     "local",
			Enabled:  true,
			Priority: 100,
		})
		repo.saveConfig(configFile)
	}

	// 初始化源缓存
	for _, srcCfg := range repo.config.Sources {
		repo.sources[srcCfg.Name] = &Source{
			Name:      srcCfg.Name,
			URL:       srcCfg.URL,
			Type:      srcCfg.Type,
			Enabled:   srcCfg.Enabled,
			Priority:  srcCfg.Priority,
			UpdatedAt: time.Now(),
			Cache:     []*app.Template{},
		}
	}

	// 创建本地仓库目录
	localRepoPath := filepath.Join(dataDir, "local-repo")
	if err := os.MkdirAll(localRepoPath, 0750); err != nil {
		fmt.Printf("创建本地仓库目录失败: %v\n", err)
	}

	return repo, nil
}

// loadConfig 加载仓库配置
func (r *Repository) loadConfig(configFile string) error {
	data, err := os.ReadFile(configFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	return json.Unmarshal(data, &r.config)
}

// saveConfig 保存仓库配置
func (r *Repository) saveConfig(configFile string) error {
	data, err := json.MarshalIndent(r.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	return os.WriteFile(configFile, data, 0644)
}

// ========== 仓库源管理 ==========

// AddSource 添加仓库源
func (r *Repository) AddSource(url string, name string) error {
	if name == "" {
		name = r.generateSourceName(url)
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// 检查是否已存在
	if _, exists := r.sources[name]; exists {
		return fmt.Errorf("仓库源 %s 已存在", name)
	}

	// 判断类型
	sourceType := "remote"
	if strings.HasPrefix(url, "/") || strings.HasPrefix(url, "file://") {
		sourceType = "local"
	}

	// 创建源
	source := &Source{
		Name:      name,
		URL:       url,
		Type:      sourceType,
		Enabled:   true,
		Priority:  50,
		UpdatedAt: time.Now(),
		Cache:     []*app.Template{},
	}

	r.sources[name] = source

	// 更新配置
	r.config.Sources = append(r.config.Sources, RepositorySourceConfig{
		Name:     name,
		URL:      url,
		Type:     sourceType,
		Enabled:  true,
		Priority: 50,
	})

	configFile := filepath.Join(r.dataDir, "repository-config.json")
	return r.saveConfig(configFile)
}

// RemoveSource 移除仓库源
func (r *Repository) RemoveSource(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.sources[name]; !exists {
		return fmt.Errorf("仓库源 %s 不存在", name)
	}

	// 不允许移除本地仓库
	if name == "local" {
		return fmt.Errorf("不能移除本地仓库")
	}

	delete(r.sources, name)

	// 更新配置
	newSources := []RepositorySourceConfig{}
	for _, src := range r.config.Sources {
		if src.Name != name {
			newSources = append(newSources, src)
		}
	}
	r.config.Sources = newSources

	configFile := filepath.Join(r.dataDir, "repository-config.json")
	return r.saveConfig(configFile)
}

// ListSources 列出仓库源
func (r *Repository) ListSources() []app.RepositorySource {
	r.mu.RLock()
	defer r.mu.RUnlock()

	result := []app.RepositorySource{}
	for _, src := range r.sources {
		result = append(result, app.RepositorySource{
			Name:      src.Name,
			URL:       src.URL,
			Type:      src.Type,
			Enabled:   src.Enabled,
			Priority:  src.Priority,
			UpdatedAt: src.UpdatedAt.Format(time.RFC3339),
		})
	}

	// 按优先级排序
	sortSources(result)

	return result
}

// EnableSource 启用仓库源
func (r *Repository) EnableSource(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	source, exists := r.sources[name]
	if !exists {
		return fmt.Errorf("仓库源 %s 不存在", name)
	}

	source.Enabled = true

	// 更新配置
	for i, src := range r.config.Sources {
		if src.Name == name {
			r.config.Sources[i].Enabled = true
			break
		}
	}

	configFile := filepath.Join(r.dataDir, "repository-config.json")
	return r.saveConfig(configFile)
}

// DisableSource 禁用仓库源
func (r *Repository) DisableSource(name string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	source, exists := r.sources[name]
	if !exists {
		return fmt.Errorf("仓库源 %s 不存在", name)
	}

	// 不允许禁用本地仓库
	if name == "local" {
		return fmt.Errorf("不能禁用本地仓库")
	}

	source.Enabled = false

	// 更新配置
	for i, src := range r.config.Sources {
		if src.Name == name {
			r.config.Sources[i].Enabled = false
			break
		}
	}

	configFile := filepath.Join(r.dataDir, "repository-config.json")
	return r.saveConfig(configFile)
}

// ========== 模板同步 ==========

// Refresh 刷新仓库（从远程源同步模板到本地Catalog）
func (r *Repository) Refresh(ctx context.Context, catalog *Catalog) error {
	r.mu.RLock()
	sources := r.sources
	r.mu.RUnlock()

	// 按优先级收集所有模板
	allTemplates := make(map[string]*app.Template)

	for _, source := range sources {
		if !source.Enabled {
			continue
		}

		templates, err := r.fetchTemplates(ctx, source)
		if err != nil {
			fmt.Printf("从仓库 %s 获取模板失败: %v\n", source.Name, err)
			continue
		}

		// 添加模板（按优先级，高优先级覆盖低优先级）
		for _, tmpl := range templates {
			if existing, ok := allTemplates[tmpl.ID]; ok {
				// 检查优先级
				if source.Priority > r.getSourcePriority(existing.ID) {
					allTemplates[tmpl.ID] = tmpl
				}
			} else {
				allTemplates[tmpl.ID] = tmpl
			}
		}

		// 更新缓存
		source.Cache = templates
	}

	// 将模板添加到Catalog
	for _, tmpl := range allTemplates {
		if err := catalog.Add(tmpl); err != nil {
			// 如果已存在，尝试更新
			if err := catalog.Update(tmpl); err != nil {
				fmt.Printf("添加/更新模板 %s 失败: %v\n", tmpl.ID, err)
			}
		}
	}

	return nil
}

// fetchTemplates 从仓库源获取模板
func (r *Repository) fetchTemplates(ctx context.Context, source *Source) ([]*app.Template, error) {
	switch source.Type {
	case "local":
		return r.fetchFromLocal(source.URL)
	case "remote":
		return r.fetchFromRemote(ctx, source.URL)
	default:
		return nil, fmt.Errorf("未知仓库类型: %s", source.Type)
	}
}

// fetchFromLocal 从本地目录获取模板
func (r *Repository) fetchFromLocal(path string) ([]*app.Template, error) {
	// 处理 file:// 协议
	if strings.HasPrefix(path, "file://") {
		path = strings.TrimPrefix(path, "file://")
	}

	files, err := os.ReadDir(path)
	if err != nil {
		return nil, fmt.Errorf("读取本地仓库失败: %w", err)
	}

	templates := []*app.Template{}
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), ".json") {
			filePath := filepath.Join(path, file.Name())
			data, err := os.ReadFile(filePath)
			if err != nil {
				fmt.Printf("读取模板文件 %s 失败: %v\n", file.Name(), err)
				continue
			}

			tmpl := &app.Template{}
			if err := json.Unmarshal(data, tmpl); err != nil {
				fmt.Printf("解析模板文件 %s 失败: %v\n", file.Name(), err)
				continue
			}

			templates = append(templates, tmpl)
		}
	}

	return templates, nil
}

// fetchFromRemote 从远程URL获取模板
func (r *Repository) fetchFromRemote(ctx context.Context, url string) ([]*app.Template, error) {
	// 构建请求
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("创建请求失败: %w", err)
	}

	// 发送请求
	client := &http.Client{Timeout: 30 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP错误: %d", resp.StatusCode)
	}

	// 读取响应
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("读取响应失败: %w", err)
	}

	// 解析模板列表
	var templates []*app.Template
	if err := json.Unmarshal(data, &templates); err != nil {
		// 可能是单个模板
		tmpl := &app.Template{}
		if err := json.Unmarshal(data, tmpl); err != nil {
			return nil, fmt.Errorf("解析响应失败: %w", err)
		}
		templates = []*app.Template{tmpl}
	}

	return templates, nil
}

// ========== 本地模板管理 ==========

// AddLocalTemplate 添加本地模板
func (r *Repository) AddLocalTemplate(tmpl *app.Template) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	localPath := filepath.Join(r.dataDir, "local-repo")
	tmplPath := filepath.Join(localPath, tmpl.ID+".json")

	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化模板失败: %w", err)
	}

	return os.WriteFile(tmplPath, data, 0644)
}

// RemoveLocalTemplate 移除本地模板
func (r *Repository) RemoveLocalTemplate(templateID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	localPath := filepath.Join(r.dataDir, "local-repo")
	tmplPath := filepath.Join(localPath, templateID+".json")

	return os.Remove(tmplPath)
}

// ========== 辅助方法 ==========

// generateSourceName 生成源名称
func (r *Repository) generateSourceName(url string) string {
	// 从URL提取名称
	if strings.Contains(url, "://") {
		parts := strings.Split(url, "/")
		if len(parts) >= 3 {
			return parts[2]
		}
	}
	return "unknown"
}

// getSourcePriority 获取模板来源的优先级
func (r *Repository) getSourcePriority(templateID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, source := range r.sources {
		for _, tmpl := range source.Cache {
			if tmpl.ID == templateID {
				return source.Priority
			}
		}
	}
	return 0
}

// sortSources 按优先级排序仓库源
func sortSources(sources []app.RepositorySource) {
	for i := 0; i < len(sources)-1; i++ {
		for j := i + 1; j < len(sources); j++ {
			if sources[i].Priority < sources[j].Priority {
				sources[i], sources[j] = sources[j], sources[i]
			}
		}
	}
}

// ========== 仓库索引 ==========

// GetIndex 获取仓库索引（所有可用模板列表）
func (r *Repository) GetIndex(ctx context.Context) ([]*app.Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 收集所有缓存中的模板
	templates := []*app.Template{}
	seen := make(map[string]bool)

	for _, source := range r.sources {
		if !source.Enabled {
			continue
		}

		for _, tmpl := range source.Cache {
			if !seen[tmpl.ID] {
				templates = append(templates, tmpl)
				seen[tmpl.ID] = true
			}
		}
	}

	return templates, nil
}

// GetTemplate 从仓库获取特定模板
func (r *Repository) GetTemplate(ctx context.Context, templateID string) (*app.Template, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	// 按优先级查找
	for _, source := range r.sources {
		if !source.Enabled {
			continue
		}

		for _, tmpl := range source.Cache {
			if tmpl.ID == templateID {
				return tmpl, nil
			}
		}
	}

	// 尝试从各源重新获取
	for _, source := range r.sources {
		if !source.Enabled {
			continue
		}

		templates, err := r.fetchTemplates(ctx, source)
		if err != nil {
			continue
		}

		for _, tmpl := range templates {
			if tmpl.ID == templateID {
				return tmpl, nil
			}
		}
	}

	return nil, fmt.Errorf("模板 %s 不存在", templateID)
}

// ========== 同步状态 ==========

// SyncStatus 同步状态
type SyncStatus struct {
	SourceName  string    `json:"sourceName"`
	URL         string    `json:"url"`
	LastSync    time.Time `json:"lastSync"`
	TemplateNum int       `json:"templateNum"`
	Status      string    `json:"status"` // success/failed/pending
	Error       string    `json:"error"`
}

// GetSyncStatus 获取同步状态
func (r *Repository) GetSyncStatus() []SyncStatus {
	r.mu.RLock()
	defer r.mu.RUnlock()

	statuses := []SyncStatus{}
	for _, source := range r.sources {
		status := SyncStatus{
			SourceName:  source.Name,
			URL:         source.URL,
			LastSync:    source.UpdatedAt,
			TemplateNum: len(source.Cache),
			Status:      "success",
		}
		statuses = append(statuses, status)
	}

	return statuses
}