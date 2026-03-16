package scheduler

import (
	"fmt"
	"sync"
	"time"
)

// DependencyManager 依赖管理器
type DependencyManager struct {
	tasks        map[string]*Task
	dependents   map[string][]string // 被依赖映射: taskID -> 依赖它的任务列表
	dependencies map[string][]string // 依赖映射: taskID -> 它依赖的任务列表
	completed    map[string]bool
	mu           sync.RWMutex
}

// NewDependencyManager 创建依赖管理器
func NewDependencyManager() *DependencyManager {
	return &DependencyManager{
		tasks:        make(map[string]*Task),
		dependents:   make(map[string][]string),
		dependencies: make(map[string][]string),
		completed:    make(map[string]bool),
	}
}

// RegisterTask 注册任务
func (dm *DependencyManager) RegisterTask(task *Task) error {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if task.ID == "" {
		return fmt.Errorf("任务 ID 不能为空")
	}

	dm.tasks[task.ID] = task

	// 设置依赖关系
	if len(task.Dependencies) > 0 {
		dm.dependencies[task.ID] = task.Dependencies

		for _, depID := range task.Dependencies {
			dm.dependents[depID] = append(dm.dependents[depID], task.ID)
		}
	}

	return nil
}

// UnregisterTask 注销任务
func (dm *DependencyManager) UnregisterTask(taskID string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 移除依赖关系
	if deps, exists := dm.dependencies[taskID]; exists {
		for _, depID := range deps {
			dm.removeDependent(depID, taskID)
		}
		delete(dm.dependencies, taskID)
	}

	// 移除被依赖关系
	if dependents, exists := dm.dependents[taskID]; exists {
		for _, depID := range dependents {
			dm.removeDependency(depID, taskID)
		}
		delete(dm.dependents, taskID)
	}

	delete(dm.tasks, taskID)
	delete(dm.completed, taskID)
}

func (dm *DependencyManager) removeDependent(taskID, dependentID string) {
	dependents := dm.dependents[taskID]
	for i, id := range dependents {
		if id == dependentID {
			dm.dependents[taskID] = append(dependents[:i], dependents[i+1:]...)
			break
		}
	}
}

func (dm *DependencyManager) removeDependency(taskID, depID string) {
	deps := dm.dependencies[taskID]
	for i, id := range deps {
		if id == depID {
			dm.dependencies[taskID] = append(deps[:i], deps[i+1:]...)
			break
		}
	}
}

// CanRun 检查任务是否可以运行
func (dm *DependencyManager) CanRun(taskID string) (bool, string) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	task, exists := dm.tasks[taskID]
	if !exists {
		return false, "任务不存在"
	}

	if len(task.Dependencies) == 0 {
		return true, ""
	}

	condition := task.DependCondition
	if condition == "" {
		condition = "all" // 默认全部依赖完成
	}

	completedCount := 0
	failedCount := 0
	pendingCount := 0

	for _, depID := range task.Dependencies {
		depTask, exists := dm.tasks[depID]
		if !exists {
			return false, fmt.Sprintf("依赖任务 %s 不存在", depID)
		}

		if dm.completed[depID] {
			completedCount++
		} else if depTask.Status == TaskStatusFailed {
			failedCount++
		} else {
			pendingCount++
		}
	}

	switch condition {
	case "all":
		if completedCount == len(task.Dependencies) {
			return true, ""
		}
		if failedCount > 0 {
			return false, fmt.Sprintf("%d 个依赖任务失败", failedCount)
		}
		return false, fmt.Sprintf("还有 %d 个依赖任务未完成", pendingCount)

	case "any":
		if completedCount > 0 {
			return true, ""
		}
		if failedCount == len(task.Dependencies) {
			return false, "所有依赖任务都失败"
		}
		return false, "没有已完成的依赖任务"

	default:
		return false, fmt.Sprintf("未知的依赖条件: %s", condition)
	}
}

// MarkCompleted 标记任务完成
func (dm *DependencyManager) MarkCompleted(taskID string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.completed[taskID] = true

	// 更新任务状态
	if task, exists := dm.tasks[taskID]; exists {
		task.Status = TaskStatusCompleted
		now := time.Now()
		task.LastRunAt = &now
		task.SuccessCount++
	}
}

// MarkFailed 标记任务失败
func (dm *DependencyManager) MarkFailed(taskID string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	if task, exists := dm.tasks[taskID]; exists {
		task.Status = TaskStatusFailed
		task.FailCount++
	}
}

// GetDependents 获取依赖于指定任务的任务列表
func (dm *DependencyManager) GetDependents(taskID string) []string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	dependents := dm.dependents[taskID]
	result := make([]string, len(dependents))
	copy(result, dependents)
	return result
}

// GetDependencies 获取指定任务的依赖列表
func (dm *DependencyManager) GetDependencies(taskID string) []string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	deps := dm.dependencies[taskID]
	result := make([]string, len(deps))
	copy(result, deps)
	return result
}

// GetReadyTasks 获取可以执行的任务列表
func (dm *DependencyManager) GetReadyTasks() []string {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	ready := make([]string, 0)

	for taskID, task := range dm.tasks {
		if task.Status != TaskStatusPending && task.Status != TaskStatusPaused {
			continue
		}

		if canRun, _ := dm.canRunInternal(taskID); canRun {
			ready = append(ready, taskID)
		}
	}

	return ready
}

func (dm *DependencyManager) canRunInternal(taskID string) (bool, string) {
	task, exists := dm.tasks[taskID]
	if !exists {
		return false, "任务不存在"
	}

	if len(task.Dependencies) == 0 {
		return true, ""
	}

	condition := task.DependCondition
	if condition == "" {
		condition = "all"
	}

	completedCount := 0
	for _, depID := range task.Dependencies {
		if dm.completed[depID] {
			completedCount++
		}
	}

	switch condition {
	case "all":
		return completedCount == len(task.Dependencies), ""
	case "any":
		return completedCount > 0, ""
	default:
		return false, ""
	}
}

// HasCycle 检查是否存在循环依赖
func (dm *DependencyManager) HasCycle() bool {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	for taskID := range dm.tasks {
		if dm.hasCycleDFS(taskID, visited, recStack) {
			return true
		}
	}

	return false
}

func (dm *DependencyManager) hasCycleDFS(taskID string, visited, recStack map[string]bool) bool {
	if recStack[taskID] {
		return true
	}

	if visited[taskID] {
		return false
	}

	visited[taskID] = true
	recStack[taskID] = true

	for _, depID := range dm.dependencies[taskID] {
		if dm.hasCycleDFS(depID, visited, recStack) {
			return true
		}
	}

	recStack[taskID] = false
	return false
}

// GetExecutionOrder 获取执行顺序（拓扑排序）
func (dm *DependencyManager) GetExecutionOrder() ([]string, error) {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	if dm.HasCycle() {
		return nil, fmt.Errorf("存在循环依赖")
	}

	// Kahn's 算法
	inDegree := make(map[string]int)
	for taskID := range dm.tasks {
		inDegree[taskID] = 0
	}

	// 计算入度
	for taskID, deps := range dm.dependencies {
		for _, depID := range deps {
			if _, exists := dm.tasks[depID]; exists {
				inDegree[taskID]++
			}
		}
	}

	// 找出入度为 0 的节点
	queue := make([]string, 0)
	for taskID, degree := range inDegree {
		if degree == 0 {
			queue = append(queue, taskID)
		}
	}

	result := make([]string, 0)

	for len(queue) > 0 {
		taskID := queue[0]
		queue = queue[1:]
		result = append(result, taskID)

		// 减少依赖此任务的任务的入度
		for _, dependentID := range dm.dependents[taskID] {
			inDegree[dependentID]--
			if inDegree[dependentID] == 0 {
				queue = append(queue, dependentID)
			}
		}
	}

	return result, nil
}

// Reset 重置所有完成状态
func (dm *DependencyManager) Reset() {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	dm.completed = make(map[string]bool)
	for _, task := range dm.tasks {
		if task.Status == TaskStatusCompleted {
			task.Status = TaskStatusPending
		}
	}
}

// ResetTask 重置指定任务的完成状态
func (dm *DependencyManager) ResetTask(taskID string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	delete(dm.completed, taskID)
	if task, exists := dm.tasks[taskID]; exists {
		if task.Status == TaskStatusCompleted {
			task.Status = TaskStatusPending
		}
	}
}

// DependencyGraph 依赖图
type DependencyGraph struct {
	Nodes []*GraphNode `json:"nodes"`
	Edges []*GraphEdge `json:"edges"`
}

// GraphNode 图节点
type GraphNode struct {
	ID     string     `json:"id"`
	Name   string     `json:"name"`
	Status TaskStatus `json:"status"`
}

// GraphEdge 图边
type GraphEdge struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// GetGraph 获取依赖图
func (dm *DependencyManager) GetGraph() *DependencyGraph {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	graph := &DependencyGraph{
		Nodes: make([]*GraphNode, 0),
		Edges: make([]*GraphEdge, 0),
	}

	for taskID, task := range dm.tasks {
		graph.Nodes = append(graph.Nodes, &GraphNode{
			ID:     taskID,
			Name:   task.Name,
			Status: task.Status,
		})

		for _, depID := range dm.dependencies[taskID] {
			graph.Edges = append(graph.Edges, &GraphEdge{
				From: depID,
				To:   taskID,
			})
		}
	}

	return graph
}
