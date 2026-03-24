package search

import (
	"strings"
	"sync"
)

// AppItem 应用项
type AppItem struct {
	ID          string   `json:"id"`          // 唯一标识
	Name        string   `json:"name"`        // 应用名称
	DisplayName string   `json:"displayName"` // 显示名称
	Description string   `json:"description"` // 描述
	Category    string   `json:"category"`    // 分类
	Version     string   `json:"version"`     // 版本
	Status      string   `json:"status"`      // 状态: running, stopped, installing, error
	Icon        string   `json:"icon"`        // 图标URL或名称
	Path        string   `json:"path"`        // 访问路径
	Port        int      `json:"port"`        // 服务端口
	Keywords    []string `json:"keywords"`    // 搜索关键词
	Type        string   `json:"type"`        // 类型: app, container, service
}

// ContainerItem 容器项
type ContainerItem struct {
	ID          string   `json:"id"`          // 容器ID
	Name        string   `json:"name"`        // 容器名称
	Image       string   `json:"image"`       // 镜像名称
	Status      string   `json:"status"`      // 状态: running, stopped, paused
	State       string   `json:"state"`       // 状态详情
	Ports       []string `json:"ports"`       // 端口映射
	Networks    []string `json:"networks"`    // 网络列表
	Keywords    []string `json:"keywords"`    // 搜索关键词
	Labels      map[string]string `json:"labels"` // 容器标签
}

// AppRegistry 应用注册表
type AppRegistry struct {
	apps       []AppItem
	containers []ContainerItem
	mu         sync.RWMutex
}

// NewAppRegistry 创建应用注册表
func NewAppRegistry() *AppRegistry {
	return &AppRegistry{
		apps:       make([]AppItem, 0),
		containers: make([]ContainerItem, 0),
	}
}

// RegisterApp 注册应用
func (r *AppRegistry) RegisterApp(apps ...AppItem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.apps = append(r.apps, apps...)
}

// RegisterContainer 注册容器
func (r *AppRegistry) RegisterContainer(containers ...ContainerItem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.containers = append(r.containers, containers...)
}

// ClearApps 清空应用列表
func (r *AppRegistry) ClearApps() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.apps = make([]AppItem, 0)
}

// ClearContainers 清空容器列表
func (r *AppRegistry) ClearContainers() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.containers = make([]ContainerItem, 0)
}

// UpdateApps 更新应用列表
func (r *AppRegistry) UpdateApps(apps []AppItem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.apps = apps
}

// UpdateContainers 更新容器列表
func (r *AppRegistry) UpdateContainers(containers []ContainerItem) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.containers = containers
}

// GetAllApps 获取所有应用
func (r *AppRegistry) GetAllApps() []AppItem {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]AppItem, len(r.apps))
	copy(result, r.apps)
	return result
}

// GetAllContainers 获取所有容器
func (r *AppRegistry) GetAllContainers() []ContainerItem {
	r.mu.RLock()
	defer r.mu.RUnlock()
	result := make([]ContainerItem, len(r.containers))
	copy(result, r.containers)
	return result
}

// AppSearchResult 应用搜索结果
type AppSearchResult struct {
	Item       interface{} `json:"item"`       // AppItem 或 ContainerItem
	Score      float64     `json:"score"`      // 相关性分数
	MatchType  string      `json:"matchType"`  // name, keyword, image, status
	MatchField string      `json:"matchField"` // 匹配字段
	Type       string      `json:"type"`       // app 或 container
}

// SearchApps 搜索应用
func (r *AppRegistry) SearchApps(query string, limit int) []AppSearchResult {
	if query == "" {
		return nil
	}

	r.mu.RLock()
	defer r.mu.RUnlock()

	query = strings.ToLower(query)
	results := make([]AppSearchResult, 0)

	// 搜索应用
	for _, app := range r.apps {
		score := 0.0
		matchType := ""
		matchField := ""

		// 检查名称匹配
		if strings.Contains(strings.ToLower(app.Name), query) ||
			strings.Contains(strings.ToLower(app.DisplayName), query) {
			score = 1.0
			matchType = "name"
			matchField = "name"
		}

		// 检查关键词匹配
		if score == 0 {
			for _, kw := range app.Keywords {
				if strings.Contains(strings.ToLower(kw), query) {
					score = 0.8
					matchType = "keyword"
					matchField = "keyword"
					break
				}
			}
		}

		// 检查描述匹配
		if score == 0 && strings.Contains(strings.ToLower(app.Description), query) {
			score = 0.6
			matchType = "description"
			matchField = "description"
		}

		// 检查分类匹配
		if score == 0 && strings.Contains(strings.ToLower(app.Category), query) {
			score = 0.5
			matchType = "category"
			matchField = "category"
		}

		// 检查状态匹配
		if score == 0 && strings.Contains(strings.ToLower(app.Status), query) {
			score = 0.4
			matchType = "status"
			matchField = "status"
		}

		if score > 0 {
			results = append(results, AppSearchResult{
				Item:       app,
				Score:      score,
				MatchType:  matchType,
				MatchField: matchField,
				Type:       "app",
			})
		}
	}

	// 搜索容器
	for _, container := range r.containers {
		score := 0.0
		matchType := ""
		matchField := ""

		// 检查名称匹配
		if strings.Contains(strings.ToLower(container.Name), query) {
			score = 1.0
			matchType = "name"
			matchField = "name"
		}

		// 检查镜像匹配
		if score == 0 && strings.Contains(strings.ToLower(container.Image), query) {
			score = 0.9
			matchType = "image"
			matchField = "image"
		}

		// 检查ID匹配
		if score == 0 && strings.Contains(strings.ToLower(container.ID), query) {
			score = 0.7
			matchType = "id"
			matchField = "id"
		}

		// 检查关键词匹配
		if score == 0 {
			for _, kw := range container.Keywords {
				if strings.Contains(strings.ToLower(kw), query) {
					score = 0.8
					matchType = "keyword"
					matchField = "keyword"
					break
				}
			}
		}

		// 检查状态匹配
		if score == 0 && strings.Contains(strings.ToLower(container.Status), query) {
			score = 0.5
			matchType = "status"
			matchField = "status"
		}

		// 检查标签匹配
		if score == 0 {
			for k, v := range container.Labels {
				if strings.Contains(strings.ToLower(k), query) ||
					strings.Contains(strings.ToLower(v), query) {
					score = 0.6
					matchType = "label"
					matchField = "label"
					break
				}
			}
		}

		if score > 0 {
			results = append(results, AppSearchResult{
				Item:       container,
				Score:      score,
				MatchType:  matchType,
				MatchField: matchField,
				Type:       "container",
			})
		}
	}

	// 按分数排序
	sortAppSearchResults(results)

	// 限制数量
	if limit > 0 && len(results) > limit {
		results = results[:limit]
	}

	return results
}

// GetRunningApps 获取运行中的应用
func (r *AppRegistry) GetRunningApps() []AppItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []AppItem
	for _, app := range r.apps {
		if app.Status == "running" {
			results = append(results, app)
		}
	}
	return results
}

// GetRunningContainers 获取运行中的容器
func (r *AppRegistry) GetRunningContainers() []ContainerItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []ContainerItem
	for _, c := range r.containers {
		if c.Status == "running" {
			results = append(results, c)
		}
	}
	return results
}

// GetAppCategories 获取应用分类
func (r *AppRegistry) GetAppCategories() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	categoryMap := make(map[string]bool)
	for _, app := range r.apps {
		if app.Category != "" {
			categoryMap[app.Category] = true
		}
	}

	categories := make([]string, 0, len(categoryMap))
	for cat := range categoryMap {
		categories = append(categories, cat)
	}
	return categories
}

// sortAppSearchResults 排序应用搜索结果
func sortAppSearchResults(results []AppSearchResult) {
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[i].Score < results[j].Score {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}

// SearchAppsByStatus 按状态搜索应用
func (r *AppRegistry) SearchAppsByStatus(status string) []AppItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []AppItem
	status = strings.ToLower(status)
	for _, app := range r.apps {
		if strings.ToLower(app.Status) == status {
			results = append(results, app)
		}
	}
	return results
}

// SearchContainersByStatus 按状态搜索容器
func (r *AppRegistry) SearchContainersByStatus(status string) []ContainerItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []ContainerItem
	status = strings.ToLower(status)
	for _, c := range r.containers {
		if strings.ToLower(c.Status) == status {
			results = append(results, c)
		}
	}
	return results
}

// SearchContainersByImage 按镜像搜索容器
func (r *AppRegistry) SearchContainersByImage(image string) []ContainerItem {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var results []ContainerItem
	image = strings.ToLower(image)
	for _, c := range r.containers {
		if strings.Contains(strings.ToLower(c.Image), image) {
			results = append(results, c)
		}
	}
	return results
}

// GetAppStats 获取应用统计
func (r *AppRegistry) GetAppStats() map[string]interface{} {
	r.mu.RLock()
	defer r.mu.RUnlock()

	runningApps := 0
	stoppedApps := 0
	runningContainers := 0
	stoppedContainers := 0

	for _, app := range r.apps {
		if app.Status == "running" {
			runningApps++
		} else {
			stoppedApps++
		}
	}

	for _, c := range r.containers {
		if c.Status == "running" {
			runningContainers++
		} else {
			stoppedContainers++
		}
	}

	return map[string]interface{}{
		"apps": map[string]int{
			"total":   len(r.apps),
			"running": runningApps,
			"stopped": stoppedApps,
		},
		"containers": map[string]int{
			"total":   len(r.containers),
			"running": runningContainers,
			"stopped": stoppedContainers,
		},
	}
}