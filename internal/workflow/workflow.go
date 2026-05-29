// Package workflow 智能工作流引擎 - 核心实现
package workflow

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"time"
)

// NewManager 创建工作流管理器
func NewManager(dataFile string) *Manager {
	return &Manager{
		workflows:  make(map[string]*Workflow),
		executions: make(map[string]*Execution),
		versions:   make(map[string][]*Version),
		templates:  make(map[string]*Template),
		config: &Config{
			MaxWorkflows:  100,
			MaxExecutions: 10000,
			ExecRetention: 30,
			MaxVersions:   20,
		},
		dataFile: dataFile,
	}
}

// Initialize 初始化管理器
func (m *Manager) Initialize() error {
	m.loadDefaultTemplates()
	return m.load()
}

// CreateWorkflow 创建工作流
func (m *Manager) CreateWorkflow(wf *Workflow) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.workflows[wf.ID]; exists {
		return ErrWorkflowExists
	}
	if len(m.workflows) >= m.config.MaxWorkflows {
		return ErrMaxWorkflows
	}
	if err := validateDAG(wf.Nodes, wf.Edges); err != nil {
		return err
	}

	now := time.Now()
	wf.Status = WfDraft
	wf.Version = 1
	wf.CreatedAt = now
	wf.UpdatedAt = now
	m.workflows[wf.ID] = wf

	// 保存初始版本
	m.versions[wf.ID] = []*Version{{
		Version:     1,
		Description: "初始版本",
		Nodes:       wf.Nodes,
		Edges:       wf.Edges,
		Triggers:    wf.Triggers,
		CreatedAt:   now,
	}}

	return m.save()
}

// UpdateWorkflow 更新工作流
func (m *Manager) UpdateWorkflow(wf *Workflow) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.workflows[wf.ID]
	if !ok {
		return ErrWorkflowNotFound
	}
	if err := validateDAG(wf.Nodes, wf.Edges); err != nil {
		return err
	}

	// 版本号递增
	wf.Version = existing.Version + 1
	wf.CreatedAt = existing.CreatedAt
	wf.UpdatedAt = time.Now()
	m.workflows[wf.ID] = wf

	// 保存版本历史
	versions := m.versions[wf.ID]
	if len(versions) >= m.config.MaxVersions {
		versions = versions[1:] // 移除最旧版本
	}
	m.versions[wf.ID] = append(versions, &Version{
		Version:     wf.Version,
		Description: fmt.Sprintf("版本 %d", wf.Version),
		Nodes:       wf.Nodes,
		Edges:       wf.Edges,
		Triggers:    wf.Triggers,
		CreatedAt:   time.Now(),
	})

	return m.save()
}

// GetWorkflow 获取工作流
func (m *Manager) GetWorkflow(id string) (*Workflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	wf, ok := m.workflows[id]
	if !ok {
		return nil, ErrWorkflowNotFound
	}
	return wf, nil
}

// ListWorkflows 列出工作流
func (m *Manager) ListWorkflows(status WorkflowStatus, tags []string) []*Workflow {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Workflow
	for _, wf := range m.workflows {
		if status != "" && wf.Status != status {
			continue
		}
		if len(tags) > 0 && !hasAnyTag(wf.Tags, tags) {
			continue
		}
		result = append(result, wf)
	}
	return result
}

// DeleteWorkflow 删除工作流
func (m *Manager) DeleteWorkflow(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.workflows[id]; !ok {
		return ErrWorkflowNotFound
	}
	delete(m.workflows, id)
	delete(m.versions, id)
	return m.save()
}

// EnableWorkflow 启用工作流
func (m *Manager) EnableWorkflow(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wf, ok := m.workflows[id]
	if !ok {
		return ErrWorkflowNotFound
	}
	wf.Status = WfActive
	wf.UpdatedAt = time.Now()
	return m.save()
}

// DisableWorkflow 禁用工作流
func (m *Manager) DisableWorkflow(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wf, ok := m.workflows[id]
	if !ok {
		return ErrWorkflowNotFound
	}
	wf.Status = WfDisabled
	wf.UpdatedAt = time.Now()
	return m.save()
}

// ExecuteWorkflow 执行工作流
func (m *Manager) ExecuteWorkflow(workflowID string, triggerType TriggerType, input string) (*Execution, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	wf, ok := m.workflows[workflowID]
	if !ok {
		return nil, ErrWorkflowNotFound
	}
	if wf.Status == WfDisabled {
		return nil, ErrWorkflowDisabled
	}

	exec := &Execution{
		ID:          fmt.Sprintf("exec-%d", time.Now().UnixNano()),
		WorkflowID:  workflowID,
		Version:     wf.Version,
		Status:      ExecRunning,
		TriggerType: triggerType,
		Input:       input,
		StartedAt:   time.Now(),
	}

	// 按DAG拓扑序执行节点
	order, err := topologicalSort(wf.Nodes, wf.Edges)
	if err != nil {
		exec.Status = ExecFailed
		exec.Error = err.Error()
		exec.EndedAt = time.Now()
		exec.Duration = exec.EndedAt.Sub(exec.StartedAt).Milliseconds()
		m.executions[exec.ID] = exec
		return exec, m.save()
	}

	for _, nodeID := range order {
		node := findNode(wf.Nodes, nodeID)
		if node == nil {
			continue
		}

		nodeExec := NodeExecution{
			NodeID:    nodeID,
			Status:    ExecRunning,
			StartedAt: time.Now(),
		}

		// 模拟执行
		output, err := m.executeNode(node, input)
		if err != nil {
			nodeExec.Status = ExecFailed
			nodeExec.Error = err.Error()
		} else {
			nodeExec.Status = ExecSuccess
			nodeExec.Output = output
			input = output // 链式传递
		}
		nodeExec.EndedAt = time.Now()
		nodeExec.Duration = nodeExec.EndedAt.Sub(nodeExec.StartedAt).Milliseconds()
		exec.Nodes = append(exec.Nodes, nodeExec)

		if nodeExec.Status == ExecFailed {
			exec.Status = ExecFailed
			exec.Error = fmt.Sprintf("节点 %s 执行失败: %s", node.Name, err.Error())
			break
		}
	}

	if exec.Status == ExecRunning {
		exec.Status = ExecSuccess
		exec.Output = input
	}
	exec.EndedAt = time.Now()
	exec.Duration = exec.EndedAt.Sub(exec.StartedAt).Milliseconds()
	m.executions[exec.ID] = exec

	return exec, m.save()
}

// executeNode 执行单个节点
func (m *Manager) executeNode(node *Node, input string) (string, error) {
	switch node.Type {
	case NodeFileOp:
		return executeFileOp(node, input)
	case NodeScript:
		return executeScript(node, input)
	case NodeHTTP:
		return executeHTTP(node, input)
	case NodeNotify:
		return executeNotify(node, input)
	case NodeAI:
		return executeAI(node, input)
	case NodeCondition:
		return executeCondition(node, input)
	case NodeDelay:
		return executeDelay(node)
	case NodeTransform:
		return executeTransform(node, input)
	case NodeStart:
		return input, nil
	case NodeEnd:
		return input, nil
	default:
		return "", ErrInvalidNodeType
	}
}

// executeFileOp 执行文件操作
func executeFileOp(node *Node, input string) (string, error) {
	operation := node.Config["operation"]
	switch operation {
	case "copy":
		return fmt.Sprintf("文件复制: %s -> %s", node.Config["source"], node.Config["dest"]), nil
	case "move":
		return fmt.Sprintf("文件移动: %s -> %s", node.Config["source"], node.Config["dest"]), nil
	case "delete":
		return fmt.Sprintf("文件删除: %s", node.Config["path"]), nil
	case "mkdir":
		return fmt.Sprintf("创建目录: %s", node.Config["path"]), nil
	default:
		return "", fmt.Errorf("未知文件操作: %s", operation)
	}
}

// executeScript 执行脚本
func executeScript(node *Node, input string) (string, error) {
	script := node.Config["script"]
	if script == "" {
		return "", fmt.Errorf("脚本内容为空")
	}
	// 模拟脚本执行
	return fmt.Sprintf("脚本执行完成: %s", script[:min(50, len(script))]), nil
}

// executeHTTP 执行HTTP调用
func executeHTTP(node *Node, input string) (string, error) {
	url := node.Config["url"]
	method := node.Config["method"]
	if url == "" {
		return "", fmt.Errorf("HTTP URL为空")
	}
	return fmt.Sprintf("HTTP %s %s 完成", method, url), nil
}

// executeNotify 发送通知
func executeNotify(node *Node, input string) (string, error) {
	title := node.Config["title"]
	channel := node.Config["channel"]
	return fmt.Sprintf("通知已发送到 %s: %s", channel, title), nil
}

// executeAI 执行AI推理
func executeAI(node *Node, input string) (string, error) {
	prompt := node.Config["prompt"]
	if prompt == "" {
		return "", fmt.Errorf("AI prompt为空")
	}
	return fmt.Sprintf("AI推理完成，基于输入: %s", input[:min(50, len(input))]), nil
}

// executeCondition 条件判断
func executeCondition(node *Node, input string) (string, error) {
	condition := node.Config["condition"]
	value := node.Config["value"]
	if condition == "" {
		return "true", nil
	}
	if strings.Contains(input, value) {
		return "true", nil
	}
	return "false", nil
}

// executeDelay 延时
func executeDelay(node *Node) (string, error) {
	return "延时完成", nil
}

// executeTransform 数据转换
func executeTransform(node *Node, input string) (string, error) {
	transformType := node.Config["type"]
	switch transformType {
	case "json_extract":
		return fmt.Sprintf("JSON提取完成: %s", input), nil
	case "regex":
		return fmt.Sprintf("正则匹配完成: %s", input), nil
	case "template":
		return fmt.Sprintf("模板渲染完成: %s", input), nil
	default:
		return input, nil
	}
}

// GetExecution 获取执行记录
func (m *Manager) GetExecution(execID string) (*Execution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	exec, ok := m.executions[execID]
	if !ok {
		return nil, ErrExecutionNotFound
	}
	return exec, nil
}

// ListExecutions 列出执行记录
func (m *Manager) ListExecutions(workflowID string, status ExecutionStatus, limit int) []*Execution {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Execution
	for _, exec := range m.executions {
		if workflowID != "" && exec.WorkflowID != workflowID {
			continue
		}
		if status != "" && exec.Status != status {
			continue
		}
		result = append(result, exec)
	}

	// 按时间倒序
	sortExecutions(result)

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// GetVersions 获取工作流版本历史
func (m *Manager) GetVersions(workflowID string) ([]*Version, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.workflows[workflowID]; !ok {
		return nil, ErrWorkflowNotFound
	}
	return m.versions[workflowID], nil
}

// RollbackVersion 回滚到指定版本
func (m *Manager) RollbackVersion(workflowID string, version int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	wf, ok := m.workflows[workflowID]
	if !ok {
		return ErrWorkflowNotFound
	}

	versions, ok := m.versions[workflowID]
	if !ok {
		return fmt.Errorf("版本历史不存在")
	}

	var target *Version
	for _, v := range versions {
		if v.Version == version {
			target = v
			break
		}
	}
	if target == nil {
		return fmt.Errorf("版本 %d 不存在", version)
	}

	wf.Nodes = target.Nodes
	wf.Edges = target.Edges
	wf.Triggers = target.Triggers
	wf.Version++
	wf.UpdatedAt = time.Now()

	m.versions[workflowID] = append(versions, &Version{
		Version:     wf.Version,
		Description: fmt.Sprintf("回滚到版本 %d", version),
		Nodes:       wf.Nodes,
		Edges:       wf.Edges,
		Triggers:    wf.Triggers,
		CreatedAt:   time.Now(),
	})

	return m.save()
}

// ListTemplates 列出模板
func (m *Manager) ListTemplates(category string) []*Template {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Template
	for _, tpl := range m.templates {
		if category != "" && tpl.Category != category {
			continue
		}
		result = append(result, tpl)
	}
	return result
}

// CreateFromTemplate 从模板创建工作流
func (m *Manager) CreateFromTemplate(templateID string, name string) (*Workflow, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tpl, ok := m.templates[templateID]
	if !ok {
		return nil, ErrTemplateNotFound
	}

	wf := tpl.Workflow
	wf.ID = fmt.Sprintf("wf-%d", time.Now().UnixNano())
	wf.Name = name
	wf.Status = WfDraft
	wf.Version = 1
	now := time.Now()
	wf.CreatedAt = now
	wf.UpdatedAt = now

	m.workflows[wf.ID] = &wf
	m.versions[wf.ID] = []*Version{{
		Version:     1,
		Description: "从模板创建",
		Nodes:       wf.Nodes,
		Edges:       wf.Edges,
		Triggers:    wf.Triggers,
		CreatedAt:   now,
	}}

	tpl.Downloads++
	return &wf, m.save()
}

// GetStats 获取统计信息
func (m *Manager) GetStats() *Stats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &Stats{
		TotalWorkflows: len(m.workflows),
		TotalTemplates: len(m.templates),
	}

	for _, wf := range m.workflows {
		if wf.Status == WfActive {
			stats.ActiveWorkflows++
		}
	}

	stats.TotalExecutions = len(m.executions)
	success := 0
	var totalDuration int64
	for _, exec := range m.executions {
		if exec.Status == ExecSuccess {
			success++
		}
		totalDuration += exec.Duration
	}
	if stats.TotalExecutions > 0 {
		stats.SuccessRate = float64(success) / float64(stats.TotalExecutions) * 100
		stats.AvgDuration = float64(totalDuration) / float64(stats.TotalExecutions)
	}

	return stats
}

// loadDefaultTemplates 加载默认模板
func (m *Manager) loadDefaultTemplates() {
	templates := []Template{
		{
			ID:          "backup-to-cloud",
			Name:        "自动备份到云端",
			Description: "定期将NAS文件备份到云存储",
			Category:    "备份",
			Tags:        []string{"备份", "云存储", "自动"},
			Downloads:   5000,
			Rating:      4.5,
			Author:      "nas-os",
			Workflow: Workflow{
				Name:        "自动备份到云端",
				Description: "定期将指定目录备份到云存储",
				Nodes: []Node{
					{ID: "start", Name: "开始", Type: NodeStart},
					{ID: "check", Name: "检查文件变化", Type: NodeFileOp, Config: map[string]string{"operation": "list", "path": "/data"}},
					{ID: "compress", Name: "压缩文件", Type: NodeScript, Config: map[string]string{"script": "tar -czf backup.tar.gz /data"}},
					{ID: "upload", Name: "上传到云端", Type: NodeHTTP, Config: map[string]string{"url": "https://api.cloud.com/upload", "method": "POST"}},
					{ID: "notify", Name: "发送通知", Type: NodeNotify, Config: map[string]string{"title": "备份完成", "channel": "telegram"}},
					{ID: "end", Name: "结束", Type: NodeEnd},
				},
				Edges: []Edge{
					{From: "start", To: "check"},
					{From: "check", To: "compress"},
					{From: "compress", To: "upload"},
					{From: "upload", To: "notify"},
					{From: "notify", To: "end"},
				},
				Triggers: []Trigger{
					{Type: TriggerCron, Config: map[string]string{"schedule": "0 2 * * *"}, Enabled: true},
				},
			},
		},
		{
			ID:          "media-organize",
			Name:        "媒体文件整理",
			Description: "自动整理下载的媒体文件",
			Category:    "媒体",
			Tags:        []string{"媒体", "整理", "自动"},
			Downloads:   3500,
			Rating:      4.3,
			Author:      "nas-os",
			Workflow: Workflow{
				Name:        "媒体文件整理",
				Description: "自动分类和整理媒体文件",
				Nodes: []Node{
					{ID: "start", Name: "开始", Type: NodeStart},
					{ID: "scan", Name: "扫描目录", Type: NodeFileOp, Config: map[string]string{"operation": "list", "path": "/downloads"}},
					{ID: "classify", Name: "AI分类", Type: NodeAI, Config: map[string]string{"prompt": "识别媒体类型"}},
					{ID: "move", Name: "移动文件", Type: NodeFileOp, Config: map[string]string{"operation": "move"}},
					{ID: "end", Name: "结束", Type: NodeEnd},
				},
				Edges: []Edge{
					{From: "start", To: "scan"},
					{From: "scan", To: "classify"},
					{From: "classify", To: "move"},
					{From: "move", To: "end"},
				},
				Triggers: []Trigger{
					{Type: TriggerFile, Config: map[string]string{"path": "/downloads", "pattern": "*"}, Enabled: true},
				},
			},
		},
		{
			ID:          "disk-monitor",
			Name:        "磁盘空间监控",
			Description: "监控磁盘空间并自动清理",
			Category:    "监控",
			Tags:        []string{"监控", "磁盘", "清理"},
			Downloads:   4200,
			Rating:      4.6,
			Author:      "nas-os",
			Workflow: Workflow{
				Name:        "磁盘空间监控",
				Description: "监控磁盘使用率，超过阈值自动清理",
				Nodes: []Node{
					{ID: "start", Name: "开始", Type: NodeStart},
					{ID: "check", Name: "检查空间", Type: NodeScript, Config: map[string]string{"script": "df -h"}},
					{ID: "condition", Name: "空间不足?", Type: NodeCondition, Config: map[string]string{"condition": "usage > 80", "value": "80%"}},
					{ID: "clean", Name: "清理临时文件", Type: NodeScript, Config: map[string]string{"script": "find /tmp -mtime +7 -delete"}},
					{ID: "alert", Name: "发送告警", Type: NodeNotify, Config: map[string]string{"title": "磁盘空间不足", "channel": "telegram"}},
					{ID: "end", Name: "结束", Type: NodeEnd},
				},
				Edges: []Edge{
					{From: "start", To: "check"},
					{From: "check", To: "condition"},
					{From: "condition", To: "clean", Condition: "true"},
					{From: "condition", To: "end", Condition: "false"},
					{From: "clean", To: "alert"},
					{From: "alert", To: "end"},
				},
				Triggers: []Trigger{
					{Type: TriggerCron, Config: map[string]string{"schedule": "0 */6 * * *"}, Enabled: true},
				},
			},
		},
	}
	for i := range templates {
		m.templates[templates[i].ID] = &templates[i]
	}
}

// validateDAG 验证DAG有效性
func validateDAG(nodes []Node, edges []Edge) error {
	if len(nodes) == 0 {
		return ErrInvalidDAG
	}

	hasStart := false
	hasEnd := false
	nodeIDs := make(map[string]bool)

	for _, n := range nodes {
		if n.Type == NodeStart {
			hasStart = true
		}
		if n.Type == NodeEnd {
			hasEnd = true
		}
		nodeIDs[n.ID] = true
	}
	if !hasStart {
		return ErrNoStartNode
	}
	if !hasEnd {
		return ErrNoEndNode
	}

	// 检查边引用的节点是否存在
	for _, e := range edges {
		if !nodeIDs[e.From] || !nodeIDs[e.To] {
			return fmt.Errorf("边引用了不存在的节点: %s -> %s", e.From, e.To)
		}
	}

	// 检测环
	if hasCycle(nodes, edges) {
		return ErrInvalidDAG
	}

	return nil
}

// hasCycle 使用DFS检测环
func hasCycle(nodes []Node, edges []Edge) bool {
	adj := make(map[string][]string)
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e.To)
	}

	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(string) bool
	dfs = func(node string) bool {
		visited[node] = true
		recStack[node] = true

		for _, neighbor := range adj[node] {
			if !visited[neighbor] {
				if dfs(neighbor) {
					return true
				}
			} else if recStack[neighbor] {
				return true
			}
		}

		recStack[node] = false
		return false
	}

	for _, n := range nodes {
		if !visited[n.ID] {
			if dfs(n.ID) {
				return true
			}
		}
	}
	return false
}

// topologicalSort 拓扑排序
func topologicalSort(nodes []Node, edges []Edge) ([]string, error) {
	if hasCycle(nodes, edges) {
		return nil, ErrInvalidDAG
	}

	inDegree := make(map[string]int)
	adj := make(map[string][]string)
	for _, n := range nodes {
		inDegree[n.ID] = 0
	}
	for _, e := range edges {
		adj[e.From] = append(adj[e.From], e.To)
		inDegree[e.To]++
	}

	var queue []string
	for id, deg := range inDegree {
		if deg == 0 {
			queue = append(queue, id)
		}
	}

	var order []string
	for len(queue) > 0 {
		node := queue[0]
		queue = queue[1:]
		order = append(order, node)

		for _, neighbor := range adj[node] {
			inDegree[neighbor]--
			if inDegree[neighbor] == 0 {
				queue = append(queue, neighbor)
			}
		}
	}

	if len(order) != len(nodes) {
		return nil, ErrInvalidDAG
	}
	return order, nil
}

// findNode 查找节点
func findNode(nodes []Node, id string) *Node {
	for i := range nodes {
		if nodes[i].ID == id {
			return &nodes[i]
		}
	}
	return nil
}

// hasAnyTag 检查是否有任意标签匹配
func hasAnyTag(tags []string, filter []string) bool {
	tagSet := make(map[string]bool)
	for _, t := range tags {
		tagSet[t] = true
	}
	for _, f := range filter {
		if tagSet[f] {
			return true
		}
	}
	return false
}

// sortExecutions 按时间倒序排序
func sortExecutions(execs []*Execution) {
	for i := 0; i < len(execs)-1; i++ {
		for j := i + 1; j < len(execs); j++ {
			if execs[j].StartedAt.After(execs[i].StartedAt) {
				execs[i], execs[j] = execs[j], execs[i]
			}
		}
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// load 加载数据
func (m *Manager) load() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := os.ReadFile(m.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var stored struct {
		Workflows  map[string]*Workflow   `json:"workflows"`
		Executions map[string]*Execution  `json:"executions"`
		Versions   map[string][]*Version  `json:"versions"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	if stored.Workflows != nil {
		m.workflows = stored.Workflows
	}
	if stored.Executions != nil {
		m.executions = stored.Executions
	}
	if stored.Versions != nil {
		m.versions = stored.Versions
	}
	return nil
}

// save 保存数据
func (m *Manager) save() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := json.MarshalIndent(struct {
		Workflows  map[string]*Workflow   `json:"workflows"`
		Executions map[string]*Execution  `json:"executions"`
		Versions   map[string][]*Version  `json:"versions"`
	}{m.workflows, m.executions, m.versions}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.dataFile, data, 0644)
}
