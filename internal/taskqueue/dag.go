package taskqueue

import (
	"fmt"
	"sync"
)

// ========== DAG 工作流 ==========

// DAGWorkflow DAG 工作流定义.
type DAGWorkflow struct {
	mu       sync.RWMutex
	ID       string              `json:"id"`
	Name     string              `json:"name"`
	Tasks    map[string]*DAGNode `json:"tasks"`
	Entry    []string            `json:"entry"` // 入口节点（无依赖的任务）
	manager  *Manager
}

// DAGNode DAG 节点.
type DAGNode struct {
	TaskID       string   `json:"task_id"`
	Dependencies []string `json:"dependencies"` // 前置依赖
	Dependents   []string `json:"dependents"`   // 后续依赖（自动构建）
}

// NewDAGWorkflow 创建 DAG 工作流.
func NewDAGWorkflow(id, name string, mgr *Manager) *DAGWorkflow {
	return &DAGWorkflow{
		ID:      id,
		Name:    name,
		Tasks:   make(map[string]*DAGNode),
		Entry:   make([]string, 0),
		manager: mgr,
	}
}

// AddTask 添加任务节点.
func (dag *DAGWorkflow) AddTask(taskID string, dependencies []string) error {
	dag.mu.Lock()
	defer dag.mu.Unlock()

	// 验证依赖任务必须已添加
	for _, dep := range dependencies {
		if _, exists := dag.Tasks[dep]; !exists {
			return fmt.Errorf("依赖任务 %s 不存在", dep)
		}
	}

	// 检测循环
	if dag.wouldCycle(taskID, dependencies) {
		return ErrCycleDetected
	}

	node := &DAGNode{
		TaskID:       taskID,
		Dependencies: dependencies,
		Dependents:   make([]string, 0),
	}

	dag.Tasks[taskID] = node

	// 更新依赖关系
	for _, dep := range dependencies {
		if depNode, exists := dag.Tasks[dep]; exists {
			depNode.Dependents = append(depNode.Dependents, taskID)
		}
	}

	// 入口节点（无依赖）
	if len(dependencies) == 0 {
		dag.Entry = append(dag.Entry, taskID)
	}

	return nil
}

// wouldCycle 检测添加后是否有环.
func (dag *DAGWorkflow) wouldCycle(taskID string, dependencies []string) bool {
	// BFS: 从 taskID 的依赖反向查找，看能否到达 taskID
	graph := make(map[string][]string)
	for _, node := range dag.Tasks {
		graph[node.TaskID] = node.Dependents
	}
	graph[taskID] = dependencies // 临时添加

	// 检查从 taskID 出发能否回到 taskID
	visited := make(map[string]bool)
	queue := make([]string, 0, len(dependencies))
	queue = append(queue, dependencies...)

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]

		if current == taskID {
			return true
		}

		if visited[current] {
			continue
		}
		visited[current] = true

		if node, exists := dag.Tasks[current]; exists {
			queue = append(queue, node.Dependencies...)
		}
	}

	return false
}

// GetTopologicalOrder 拓扑排序.
func (dag *DAGWorkflow) GetTopologicalOrder() ([]string, error) {
	dag.mu.RLock()
	defer dag.mu.RUnlock()

	// 计算入度
	inDegree := make(map[string]int)
	for _, node := range dag.Tasks {
		inDegree[node.TaskID] = len(node.Dependencies)
	}

	// 从入度为 0 的节点开始
	queue := make([]string, 0)
	for _, entry := range dag.Entry {
		queue = append(queue, entry)
	}

	result := make([]string, 0, len(dag.Tasks))

	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		result = append(result, current)

		node, exists := dag.Tasks[current]
		if !exists {
			continue
		}

		for _, dependent := range node.Dependents {
			inDegree[dependent]--
			if inDegree[dependent] == 0 {
				queue = append(queue, dependent)
			}
		}
	}

	if len(result) != len(dag.Tasks) {
		return nil, ErrCycleDetected
	}

	return result, nil
}

// GetParallelLevels 获取可并行执行的层级.
func (dag *DAGWorkflow) GetParallelLevels() ([][]string, error) {
	dag.mu.RLock()
	defer dag.mu.RUnlock()

	// 计算入度
	inDegree := make(map[string]int)
	for _, node := range dag.Tasks {
		inDegree[node.TaskID] = len(node.Dependencies)
	}

	levels := make([][]string, 0)
	remaining := len(dag.Tasks)

	// 初始层级（入度为 0）
	currentLevel := make([]string, 0)
	for id, deg := range inDegree {
		if deg == 0 {
			currentLevel = append(currentLevel, id)
		}
	}

	for len(currentLevel) > 0 {
		levels = append(levels, currentLevel)
		remaining -= len(currentLevel)

		nextLevel := make([]string, 0)
		for _, current := range currentLevel {
			node, exists := dag.Tasks[current]
			if !exists {
				continue
			}
			for _, dependent := range node.Dependents {
				inDegree[dependent]--
				if inDegree[dependent] == 0 {
					nextLevel = append(nextLevel, dependent)
				}
			}
		}
		currentLevel = nextLevel
	}

	if remaining != 0 {
		return nil, ErrCycleDetected
	}

	return levels, nil
}

// Execute 执行 DAG 工作流.
func (dag *DAGWorkflow) Execute(payloads map[string]map[string]interface{}) (map[string]error, error) {
	dag.mu.RLock()
	_, err := dag.GetTopologicalOrder()
	dag.mu.RUnlock()

	if err != nil {
		return nil, err
	}

	results := make(map[string]error)
	var mu sync.Mutex

	// 获取并行层级
	dag.mu.RLock()
	levels, err := dag.GetParallelLevels()
	dag.mu.RUnlock()

	if err != nil {
		return nil, err
	}

	for _, level := range levels {
		// 同一层级并行提交
		var wg sync.WaitGroup
		for _, taskID := range level {
			wg.Add(1)
			go func(id string) {
				defer wg.Done()

				// 获取任务
				dag.manager.mu.RLock()
				task, exists := dag.manager.tasks[id]
				dag.manager.mu.RUnlock()

				if !exists {
					mu.Lock()
					results[id] = fmt.Errorf("任务 %s 不存在", id)
					mu.Unlock()
					return
				}

				// 等待任务完成
				for {
					dag.manager.mu.RLock()
					status := task.Status
					dag.manager.mu.RUnlock()

					switch status {
					case StatusSuccess:
						mu.Lock()
						results[id] = nil
						mu.Unlock()
						return
					case StatusFailed, StatusDead, StatusCancelled, StatusTimeout:
						mu.Lock()
						results[id] = fmt.Errorf("任务失败: %s", task.Error)
						mu.Unlock()
						return
					default:
						// 等待
						select {
						case <-task.cancel:
							mu.Lock()
							results[id] = fmt.Errorf("任务被取消")
							mu.Unlock()
							return
						}
					}
				}
			}(taskID)
		}
		wg.Wait()

		// 检查当前层级是否有失败
		for _, taskID := range level {
			mu.Lock()
			if results[taskID] != nil {
				mu.Unlock()
				return results, fmt.Errorf("任务 %s 失败，终止工作流", taskID)
			}
			mu.Unlock()
		}
	}

	return results, nil
}

// Validate 验证 DAG 结构.
func (dag *DAGWorkflow) Validate() error {
	dag.mu.RLock()
	defer dag.mu.RUnlock()

	// 检查所有依赖是否存在
	for _, node := range dag.Tasks {
		for _, dep := range node.Dependencies {
			if _, exists := dag.Tasks[dep]; !exists {
				return fmt.Errorf("任务 %s 依赖的任务 %s 不存在", node.TaskID, dep)
			}
		}
	}

	// 检查是否有环
	_, err := dag.GetTopologicalOrder()
	return err
}

// GetDependents 获取任务的所有下游依赖（递归）.
func (dag *DAGWorkflow) GetDependents(taskID string) []string {
	dag.mu.RLock()
	defer dag.mu.RUnlock()

	result := make([]string, 0)
	visited := make(map[string]bool)

	var traverse func(id string)
	traverse = func(id string) {
		node, exists := dag.Tasks[id]
		if !exists {
			return
		}
		for _, dep := range node.Dependents {
			if !visited[dep] {
				visited[dep] = true
				result = append(result, dep)
				traverse(dep)
			}
		}
	}

	traverse(taskID)
	return result
}

// GetDependencies 获取任务的所有上游依赖（递归）.
func (dag *DAGWorkflow) GetDependencies(taskID string) []string {
	dag.mu.RLock()
	defer dag.mu.RUnlock()

	result := make([]string, 0)
	visited := make(map[string]bool)

	var traverse func(id string)
	traverse = func(id string) {
		node, exists := dag.Tasks[id]
		if !exists {
			return
		}
		for _, dep := range node.Dependencies {
			if !visited[dep] {
				visited[dep] = true
				result = append(result, dep)
				traverse(dep)
			}
		}
	}

	traverse(taskID)
	return result
}
