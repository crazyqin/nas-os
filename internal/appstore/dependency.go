// Package appstore 应用依赖自动解析和冲突检测
package appstore

import (
	"fmt"
	"strings"
)

// ========== 依赖解析器 ==========

// DependencyResolver 依赖解析器
type DependencyResolver struct {
	catalog *Catalog
}

// NewDependencyResolver 创建依赖解析器
func NewDependencyResolver(catalog *Catalog) *DependencyResolver {
	return &DependencyResolver{catalog: catalog}
}

// ResolveResult 解析结果
type ResolveResult struct {
	Resolved     []string   `json:"resolved"`     // 需要安装的依赖（拓扑排序）
	Conflicts    []Conflict `json:"conflicts"`    // 冲突列表
	Warnings     []string   `json:"warnings"`     // 警告信息
	TotalApps    int        `json:"totalApps"`    // 总需安装数量（含目标应用）
	InstallOrder []string   `json:"installOrder"` // 推荐安装顺序
}

// Conflict 冲突定义
type Conflict struct {
	AppA     string `json:"appA"`
	AppB     string `json:"appB"`
	Reason   string `json:"reason"`
	Severity string `json:"severity"` // "error", "warning"
}

// Resolve 解析应用依赖
// 输入目标应用ID和已安装应用列表，返回需要额外安装的依赖和冲突
func (dr *DependencyResolver) Resolve(appID string, installed map[string]bool) (*ResolveResult, error) {
	app, ok := dr.catalog.GetApp(appID)
	if !ok {
		return nil, fmt.Errorf("应用 %s 不存在", appID)
	}

	result := &ResolveResult{
		Conflicts: make([]Conflict, 0),
		Warnings:  make([]string, 0),
	}

	// 1. 检查直接冲突
	for _, conflictID := range app.Conflicts {
		if installed[conflictID] {
			conflictApp, _ := dr.catalog.GetApp(conflictID)
			name := conflictID
			if conflictApp != nil {
				name = conflictApp.DisplayName
			}
			result.Conflicts = append(result.Conflicts, Conflict{
				AppA:     appID,
				AppB:     conflictID,
				Reason:   fmt.Sprintf("%s 与 %s 存在冲突，不能同时安装", app.DisplayName, name),
				Severity: "error",
			})
		}
	}

	// 2. 检查反向冲突（已安装的应用声明与目标冲突）
	for installedID := range installed {
		installedApp, ok := dr.catalog.GetApp(installedID)
		if !ok {
			continue
		}
		for _, conflictID := range installedApp.Conflicts {
			if conflictID == appID {
				result.Conflicts = append(result.Conflicts, Conflict{
					AppA:     installedID,
					AppB:     appID,
					Reason:   fmt.Sprintf("已安装的 %s 声明与 %s 冲突", installedApp.DisplayName, app.DisplayName),
					Severity: "error",
				})
			}
		}
	}

	// 3. 递归解析依赖（不包含目标应用自身）
	visited := make(map[string]bool)
	resolved := make([]string, 0)
	for _, dep := range app.Dependencies {
		if err := dr.resolveDeps(dep, installed, visited, &resolved, result); err != nil {
			return nil, err
		}
	}

	// 去重并生成安装顺序
	seen := make(map[string]bool)
	var unique []string
	for _, id := range resolved {
		if !seen[id] && !installed[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}

	result.Resolved = unique
	result.TotalApps = len(unique) + 1 // +1 for the target app itself
	result.InstallOrder = dr.topologicalSort(unique, installed)

	// 4. 检查端口冲突
	dr.checkPortConflicts(appID, installed, result)

	return result, nil
}

// resolveDeps 递归解析依赖
func (dr *DependencyResolver) resolveDeps(appID string, installed map[string]bool, visited map[string]bool, resolved *[]string, result *ResolveResult) error {
	if visited[appID] {
		// 循环依赖检测
		result.Warnings = append(result.Warnings, fmt.Sprintf("检测到循环依赖: %s", appID))
		return nil
	}

	// 已安装则跳过
	if installed[appID] {
		return nil
	}

	app, ok := dr.catalog.GetApp(appID)
	if !ok {
		// 依赖不存在，给出警告但不阻断
		result.Warnings = append(result.Warnings, fmt.Sprintf("依赖 %s 不在目录中，可能需要手动安装", appID))
		return nil
	}

	visited[appID] = true

	// 先解析子依赖
	for _, dep := range app.Dependencies {
		if err := dr.resolveDeps(dep, installed, visited, resolved, result); err != nil {
			return err
		}
	}

	*resolved = append(*resolved, appID)
	return nil
}

// topologicalSort 拓扑排序生成安装顺序
func (dr *DependencyResolver) topologicalSort(apps []string, installed map[string]bool) []string {
	// 构建依赖图
	inDegree := make(map[string]int)
	graph := make(map[string][]string)

	for _, appID := range apps {
		if _, ok := inDegree[appID]; !ok {
			inDegree[appID] = 0
		}
		app, ok := dr.catalog.GetApp(appID)
		if !ok {
			continue
		}
		for _, dep := range app.Dependencies {
			if !installed[dep] {
				graph[dep] = append(graph[dep], appID)
				inDegree[appID]++
			}
		}
	}

	// Kahn 算法
	var queue []string
	for _, appID := range apps {
		if inDegree[appID] == 0 {
			queue = append(queue, appID)
		}
	}

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		for _, neighbor := range graph[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	// 如果有环，追加剩余节点
	if len(order) < len(apps) {
		seen := make(map[string]bool)
		for _, id := range order {
			seen[id] = true
		}
		for _, id := range apps {
			if !seen[id] {
				order = append(order, id)
			}
		}
	}

	return order
}

// checkPortConflicts 检查端口冲突
func (dr *DependencyResolver) checkPortConflicts(appID string, installed map[string]bool, result *ResolveResult) {
	app, ok := dr.catalog.GetApp(appID)
	if !ok {
		return
	}

	// 收集目标应用使用的端口
	targetPorts := make(map[int]string)
	for _, container := range app.Containers {
		for _, port := range container.Ports {
			if port.DefaultHostPort > 0 {
				if existing, exists := targetPorts[port.DefaultHostPort]; exists {
					result.Warnings = append(result.Warnings,
						fmt.Sprintf("应用 %s 内部端口冲突: %d 被 %s 和 %s 同时使用", appID, port.DefaultHostPort, existing, port.Name))
				}
				targetPorts[port.DefaultHostPort] = port.Name
			}
		}
	}

	// 与已安装应用检查端口冲突
	for installedID := range installed {
		installedApp, ok := dr.catalog.GetApp(installedID)
		if !ok {
			continue
		}
		for _, container := range installedApp.Containers {
			for _, port := range container.Ports {
				if port.DefaultHostPort > 0 {
					if _, exists := targetPorts[port.DefaultHostPort]; exists {
						result.Warnings = append(result.Warnings,
							fmt.Sprintf("端口冲突: %s 和 %s 都使用端口 %d",
								app.DisplayName, installedApp.DisplayName, port.DefaultHostPort))
					}
				}
			}
		}
	}
}

// BatchResolve 批量解析依赖（批量安装多个应用时）
func (dr *DependencyResolver) BatchResolve(appIDs []string, installed map[string]bool) (*ResolveResult, error) {
	result := &ResolveResult{
		Conflicts: make([]Conflict, 0),
		Warnings:  make([]string, 0),
	}

	// 检查批量应用之间的互相冲突
	for i := 0; i < len(appIDs); i++ {
		for j := i + 1; j < len(appIDs); j++ {
			appA, okA := dr.catalog.GetApp(appIDs[i])
			appB, okB := dr.catalog.GetApp(appIDs[j])
			if !okA || !okB {
				continue
			}

			// 检查 A 声明与 B 冲突
			for _, conflictID := range appA.Conflicts {
				if conflictID == appIDs[j] {
					result.Conflicts = append(result.Conflicts, Conflict{
						AppA:     appIDs[i],
						AppB:     appIDs[j],
						Reason:   fmt.Sprintf("%s 与 %s 存在冲突", appA.DisplayName, appB.DisplayName),
						Severity: "error",
					})
				}
			}
		}
	}

	if len(result.Conflicts) > 0 {
		return result, nil
	}

	// 合并解析所有依赖
	visited := make(map[string]bool)
	resolved := make([]string, 0)
	for _, appID := range appIDs {
		if err := dr.resolveDeps(appID, installed, visited, &resolved, result); err != nil {
			return nil, err
		}
	}

	// 去重
	seen := make(map[string]bool)
	var unique []string
	for _, id := range resolved {
		if !seen[id] && !installed[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}

	result.Resolved = unique
	result.TotalApps = len(unique) + len(appIDs)
	result.InstallOrder = dr.topologicalSort(unique, installed)

	return result, nil
}

// ValidateUninstall 验证卸载（检查是否有其他应用依赖此应用）
func (dr *DependencyResolver) ValidateUninstall(appID string, installed map[string]bool) []string {
	var dependents []string

	for installedID := range installed {
		if installedID == appID {
			continue
		}
		app, ok := dr.catalog.GetApp(installedID)
		if !ok {
			continue
		}
		for _, dep := range app.Dependencies {
			if dep == appID {
				dependents = append(dependents, installedID)
				break
			}
		}
	}

	return dependents
}

// GetDependencyGraph 获取应用的依赖关系图
func (dr *DependencyResolver) GetDependencyGraph(appID string) *DependencyGraph {
	_, ok := dr.catalog.GetApp(appID)
	if !ok {
		return nil
	}

	graph := &DependencyGraph{
		Root:    appID,
		Deps:    make(map[string][]string),
		Visited: make(map[string]bool),
	}

	dr.buildGraph(appID, graph)
	return graph
}

// DependencyGraph 依赖关系图
type DependencyGraph struct {
	Root    string              `json:"root"`
	Deps    map[string][]string `json:"deps"` // appID -> [depIDs]
	Visited map[string]bool     `json:"-"`
}

func (dr *DependencyResolver) buildGraph(appID string, graph *DependencyGraph) {
	if graph.Visited[appID] {
		return
	}
	graph.Visited[appID] = true

	app, ok := dr.catalog.GetApp(appID)
	if !ok {
		return
	}

	graph.Deps[appID] = app.Dependencies
	for _, dep := range app.Dependencies {
		dr.buildGraph(dep, graph)
	}
}

// FormatDependencyTree 格式化依赖树为可读字符串
func (dr *DependencyResolver) FormatDependencyTree(appID string, installed map[string]bool) string {
	graph := dr.GetDependencyGraph(appID)
	if graph == nil {
		return ""
	}

	var sb strings.Builder
	dr.formatTree(appID, graph, installed, &sb, "")
	return sb.String()
}

func (dr *DependencyResolver) formatTree(appID string, graph *DependencyGraph, installed map[string]bool, sb *strings.Builder, prefix string) {
	app, _ := dr.catalog.GetApp(appID)
	name := appID
	if app != nil {
		name = app.DisplayName
	}

	status := "⬜"
	if installed[appID] {
		status = "✅"
	}

	sb.WriteString(fmt.Sprintf("%s%s %s\n", prefix, status, name))

	deps := graph.Deps[appID]
	for i, dep := range deps {
		isLast := i == len(deps)-1
		newPrefix := prefix
		if isLast {
			newPrefix += "  └─ "
		} else {
			newPrefix += "  ├─ "
		}
		dr.formatTree(dep, graph, installed, sb, newPrefix)
	}
}
