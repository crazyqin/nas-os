// Package containersched 提供智能容器调度核心业务逻辑
package containersched

import (
	"container/heap"
	"context"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 常量 ==========

const (
	// DefaultMaxRetries 默认最大重试次数
	DefaultMaxRetries = 3
	// DefaultCooldown 默认扩缩容冷却时间
	DefaultCooldown = 5 * time.Minute
	// DefaultPowerSaveThreshold 默认节能阈值
	DefaultPowerSaveThreshold = 0.3
	// DefaultMinActiveNodes 默认最小活跃节点数
	DefaultMinActiveNodes = 1
	// ScoreWeights 权重总和
	ScoreWeights = 100
)

// ========== 错误类型 ==========

// NotFoundError 资源未找到错误
type NotFoundError struct {
	Resource string
	ID       string
}

func (e *NotFoundError) Error() string {
	return fmt.Sprintf("%s %q not found", e.Resource, e.ID)
}

// ValidationError 验证错误
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation error on %s: %s", e.Field, e.Message)
}

// ScheduleError 调度错误
type ScheduleError struct {
	ContainerID string
	Message     string
}

func (e *ScheduleError) Error() string {
	return fmt.Sprintf("schedule error for container %s: %s", e.ContainerID, e.Message)
}

// ========== 优先级队列 ==========

// PriorityQueue 优先级队列
type PriorityQueue []*QueueItem

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// 优先级高的排在前面
	return pq[i].Request.Priority > pq[j].Request.Priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *PriorityQueue) Push(x interface{}) {
	item := x.(*QueueItem)
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	*pq = old[0 : n-1]
	return item
}

// ========== Manager ==========

// Manager 智能容器调度管理器
type Manager struct {
	nodes         map[string]*Node
	placements    map[string]*Placement
	queue         PriorityQueue
	autoScale     map[string]*AutoScalePolicy
	powerSave     *PowerSaveConfig
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	stats         *ScheduleStats
}

// NewManager 创建调度管理器
func NewManager() *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	pq := make(PriorityQueue, 0)
	heap.Init(&pq)

	return &Manager{
		nodes:      make(map[string]*Node),
		placements: make(map[string]*Placement),
		queue:      pq,
		autoScale:  make(map[string]*AutoScalePolicy),
		powerSave: &PowerSaveConfig{
			Enabled:           false,
			Threshold:         DefaultPowerSaveThreshold,
			MinActiveNodes:    DefaultMinActiveNodes,
			ConsolidationTime: "02:00-06:00",
		},
		ctx:    ctx,
		cancel: cancel,
		stats:  &ScheduleStats{},
	}
}

// ========== 节点管理 ==========

// CreateNode 创建节点
func (m *Manager) CreateNode(req CreateNodeRequest) (*Node, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, &ValidationError{Field: "name", Message: "name is required"}
	}

	node := &Node{
		ID:     uuid.New().String(),
		Name:   req.Name,
		Host:   req.Host,
		Role:   req.Role,
		Status: NodeStatusReady,
		Resources: &NodeResources{
			CPU: CPUResource{
				TotalCores:   4,
				UsedCores:    0,
				FreeCores:    4,
				UsagePercent: 0,
			},
			Memory: MemoryResource{
				TotalBytes:   8 * 1024 * 1024 * 1024,
				UsedBytes:    0,
				FreeBytes:    8 * 1024 * 1024 * 1024,
				UsagePercent: 0,
			},
			DiskIO: DiskIOResource{
				ReadBPS:      0,
				WriteBPS:     0,
				ReadIOPS:     0,
				WriteIOPS:    0,
				UsagePercent: 0,
			},
			Network: NetworkResource{
				BandwidthBPS: 1000 * 1024 * 1024,
				UsedBPS:      0,
				UsagePercent: 0,
			},
			UpdatedAt: time.Now(),
		},
		Labels:        req.Labels,
		Taints:        req.Taints,
		LastHeartbeat: time.Now(),
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
	}

	m.nodes[node.ID] = node
	return node, nil
}

// GetNode 获取节点
func (m *Manager) GetNode(id string) (*Node, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	node, ok := m.nodes[id]
	if !ok {
		return nil, &NotFoundError{Resource: "node", ID: id}
	}
	return node, nil
}

// ListNodes 列出节点
func (m *Manager) ListNodes() []*Node {
	m.mu.RLock()
	defer m.mu.RUnlock()

	nodes := make([]*Node, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, node)
	}
	sort.Slice(nodes, func(i, j int) bool {
		return nodes[i].CreatedAt.Before(nodes[j].CreatedAt)
	})
	return nodes
}

// UpdateNode 更新节点
func (m *Manager) UpdateNode(id string, req UpdateNodeRequest) (*Node, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[id]
	if !ok {
		return nil, &NotFoundError{Resource: "node", ID: id}
	}

	if req.Name != nil {
		node.Name = *req.Name
	}
	if req.Role != nil {
		node.Role = *req.Role
	}
	if req.Labels != nil {
		node.Labels = req.Labels
	}
	if req.Taints != nil {
		node.Taints = req.Taints
	}
	node.UpdatedAt = time.Now()

	return node, nil
}

// DeleteNode 删除节点
func (m *Manager) DeleteNode(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.nodes[id]; !ok {
		return &NotFoundError{Resource: "node", ID: id}
	}

	// 检查是否有容器在该节点上
	for _, p := range m.placements {
		if p.NodeID == id {
			return &ScheduleError{
				ContainerID: p.ContainerID,
				Message:     "cannot delete node with running containers",
			}
		}
	}

	delete(m.nodes, id)
	return nil
}

// UpdateNodeResources 更新节点资源
func (m *Manager) UpdateNodeResources(id string, resources *NodeResources) (*Node, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	node, ok := m.nodes[id]
	if !ok {
		return nil, &NotFoundError{Resource: "node", ID: id}
	}

	resources.UpdatedAt = time.Now()
	node.Resources = resources
	node.UpdatedAt = time.Now()

	return node, nil
}

// ========== 调度核心逻辑 ==========

// Schedule 调度容器到最优节点
func (m *Manager) Schedule(req *ScheduleRequest) (*ScheduleResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证请求
	if req.ContainerID == "" {
		return nil, &ValidationError{Field: "container_id", Message: "container_id is required"}
	}
	if req.Image == "" {
		return nil, &ValidationError{Field: "image", Message: "image is required"}
	}

	// 获取可用节点
	availableNodes := m.getAvailableNodes(req.Constraints)
	if len(availableNodes) == 0 {
		return nil, &ScheduleError{
			ContainerID: req.ContainerID,
			Message:     "no available nodes",
		}
	}

	// 计算每个节点的分数
	type nodeScore struct {
		node  *Node
		score int
	}

	scores := make([]nodeScore, 0, len(availableNodes))
	for _, node := range availableNodes {
		score := m.calculateScore(node, req)
		scores = append(scores, nodeScore{node: node, score: score})
	}

	// 按分数排序
	sort.Slice(scores, func(i, j int) bool {
		return scores[i].score > scores[j].score
	})

	// 选择最优节点
	best := scores[0]
	if best.score <= 0 {
		return nil, &ScheduleError{
			ContainerID: req.ContainerID,
			Message:     "no suitable node found (score too low)",
		}
	}

	// 记录放置
	placement := &Placement{
		ContainerID:   req.ContainerID,
		ContainerName: req.ContainerName,
		NodeID:        best.node.ID,
		NodeName:      best.node.Name,
		Resources:     req.Resources,
		ScheduledAt:   time.Now(),
		Priority:      req.Priority,
	}
	m.placements[req.ContainerID] = placement

	// 更新统计
	m.stats.TotalScheduled++
	m.stats.TotalContainers++
	now := time.Now()
	m.stats.LastScheduledAt = &now

	result := &ScheduleResult{
		ContainerID: req.ContainerID,
		NodeID:      best.node.ID,
		NodeName:    best.node.Name,
		Score:       best.score,
		Reason:      fmt.Sprintf("selected node %s with score %d", best.node.Name, best.score),
		ScheduledAt: time.Now(),
		Success:     true,
	}

	log.Printf("Scheduled container %s to node %s (score: %d)", req.ContainerID, best.node.Name, best.score)
	return result, nil
}

// getAvailableNodes 获取可用节点
func (m *Manager) getAvailableNodes(constraints *ScheduleConstraints) []*Node {
	available := make([]*Node, 0)

	for _, node := range m.nodes {
		// 检查节点状态
		if node.Status != NodeStatusReady && node.Status != NodeStatusScheduling {
			continue
		}

		// 检查排除节点
		if constraints != nil {
			excluded := false
			for _, id := range constraints.ExcludedNodes {
				if node.ID == id {
					excluded = true
					break
				}
			}
			if excluded {
				continue
			}

			// 检查节点选择器
			if !matchLabels(node.Labels, constraints.NodeSelector) {
				continue
			}
		}

		// 检查污点容忍（始终检查）
		tolerations := []Toleration{}
		if constraints != nil {
			tolerations = constraints.Tolerations
		}
		if !m.toleratesTaints(node.Taints, tolerations) {
			continue
		}

		available = append(available, node)
	}

	return available
}

// calculateScore 计算节点分数
func (m *Manager) calculateScore(node *Node, req *ScheduleRequest) int {
	score := 0

	// 1. 资源匹配度 (40分)
	resourceScore := m.calculateResourceScore(node, req.Resources)
	score += resourceScore

	// 2. 亲和性 (20分)
	affinityScore := m.calculateAffinityScore(node, req.Constraints)
	score += affinityScore

	// 3. 反亲和性 (20分)
	antiAffinityScore := m.calculateAntiAffinityScore(node, req.Constraints)
	score += antiAffinityScore

	// 4. 负载均衡 (10分)
	loadBalanceScore := m.calculateLoadBalanceScore(node)
	score += loadBalanceScore

	// 5. 优先节点 (10分)
	preferredScore := m.calculatePreferredScore(node, req.Constraints)
	score += preferredScore

	// 6. 优先级加分 (最高10分)
	priorityBonus := int(req.Priority) // 1-10分
	score += priorityBonus

	return score
}

// calculateResourceScore 计算资源匹配分数
func (m *Manager) calculateResourceScore(node *Node, req *ResourceRequest) int {
	if req == nil {
		return 20 // 默认分数
	}

	res := node.Resources
	score := 0

	// CPU 分数 (0-10)
	if req.CPUCores > 0 {
		if res.CPU.FreeCores >= req.CPUCores {
			cpuRatio := res.CPU.FreeCores / float64(res.CPU.TotalCores)
			score += int(10 * cpuRatio)
		} else {
			return 0 // 资源不足
		}
	} else {
		score += 5
	}

	// 内存分数 (0-10)
	if req.MemoryBytes > 0 {
		if res.Memory.FreeBytes >= req.MemoryBytes {
			memRatio := float64(res.Memory.FreeBytes) / float64(res.Memory.TotalBytes)
			score += int(10 * memRatio)
		} else {
			return 0 // 资源不足
		}
	} else {
		score += 5
	}

	// 磁盘 IO 分数 (0-10)
	if req.DiskIOBPS > 0 {
		freeIO := res.DiskIO.ReadBPS + res.DiskIO.WriteBPS
		if freeIO >= req.DiskIOBPS {
			score += 10 - int(res.DiskIO.UsagePercent/10)
		} else {
			score += 3
		}
	} else {
		score += 5
	}

	// 网络分数 (0-10)
	if req.NetworkBPS > 0 {
		freeNet := res.Network.BandwidthBPS - res.Network.UsedBPS
		if freeNet >= req.NetworkBPS {
			score += 10 - int(res.Network.UsagePercent/10)
		} else {
			score += 3
		}
	} else {
		score += 5
	}

	return score
}

// calculateAffinityScore 计算亲和性分数
func (m *Manager) calculateAffinityScore(node *Node, constraints *ScheduleConstraints) int {
	if constraints == nil || len(constraints.Affinity) == 0 {
		return 10 // 默认中等分数
	}

	score := 0
	for _, rule := range constraints.Affinity {
		// 检查目标容器是否在同一节点
		for _, p := range m.placements {
			if p.ContainerName == rule.TargetContainer && p.NodeID == node.ID {
				score += rule.Weight
			}
		}
		// 检查标签匹配
		if matchLabels(node.Labels, rule.Labels) {
			score += rule.Weight / 2
		}
	}

	if score > 20 {
		score = 20
	}
	return score
}

// calculateAntiAffinityScore 计算反亲和性分数
func (m *Manager) calculateAntiAffinityScore(node *Node, constraints *ScheduleConstraints) int {
	if constraints == nil || len(constraints.AntiAffinity) == 0 {
		return 10 // 默认中等分数
	}

	score := 20 // 满分
	for _, rule := range constraints.AntiAffinity {
		// 检查目标容器是否在同一节点
		for _, p := range m.placements {
			if p.ContainerName == rule.TargetContainer && p.NodeID == node.ID {
				score -= rule.Weight
			}
		}
	}

	if score < 0 {
		score = 0
	}
	return score
}

// calculateLoadBalanceScore 计算负载均衡分数
func (m *Manager) calculateLoadBalanceScore(node *Node) int {
	// 负载越低分数越高
	cpuScore := int(5 * (1 - node.Resources.CPU.UsagePercent/100))
	memScore := int(5 * (1 - node.Resources.Memory.UsagePercent/100))
	return cpuScore + memScore
}

// calculatePreferredScore 计算优先节点分数
func (m *Manager) calculatePreferredScore(node *Node, constraints *ScheduleConstraints) int {
	if constraints == nil || len(constraints.PreferredNodes) == 0 {
		return 5 // 默认分数
	}

	for _, id := range constraints.PreferredNodes {
		if node.ID == id {
			return 10
		}
	}
	return 5
}

// matchLabels 匹配标签
func matchLabels(nodeLabels, required map[string]string) bool {
	if len(required) == 0 {
		return true
	}
	for k, v := range required {
		if nodeLabels[k] != v {
			return false
		}
	}
	return true
}

// toleratesTaints 检查是否容忍污点
func (m *Manager) toleratesTaints(taints []Taint, tolerations []Toleration) bool {
	if len(taints) == 0 {
		return true
	}
	// 如果有污点但没有容忍规则，不能调度
	if len(tolerations) == 0 {
		return false
	}

	for _, taint := range taints {
		tolerated := false
		for _, toleration := range tolerations {
			if toleration.Effect == taint.Effect || toleration.Effect == "" {
				if toleration.Operator == "Exists" {
					tolerated = true
					break
				}
				if toleration.Operator == "Equal" && toleration.Value == taint.Value {
					tolerated = true
					break
				}
			}
		}
		if !tolerated && taint.Effect == TaintEffectNoSchedule {
			return false
		}
	}
	return true
}

// ========== 调度队列 ==========

// Enqueue 将调度请求加入队列
func (m *Manager) Enqueue(req *ScheduleRequest) (*QueueItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.ContainerID == "" {
		return nil, &ValidationError{Field: "container_id", Message: "container_id is required"}
	}

	if req.RequestedAt.IsZero() {
		req.RequestedAt = time.Now()
	}

	item := &QueueItem{
		Request:  req,
		Status:   QueueItemStatusPending,
		QueuedAt: time.Now(),
	}

	heap.Push(&m.queue, item)
	m.stats.PendingInQueue++

	return item, nil
}

// Dequeue 从队列中取出最高优先级的请求
func (m *Manager) Dequeue() *QueueItem {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.queue.Len() == 0 {
		return nil
	}

	item := heap.Pop(&m.queue).(*QueueItem)
	item.Status = QueueItemStatusProcessing
	now := time.Now()
	item.StartedAt = &now
	m.stats.PendingInQueue--

	return item
}

// GetQueueStatus 获取队列状态
func (m *Manager) GetQueueStatus() []*QueueItem {
	m.mu.RLock()
	defer m.mu.RUnlock()

	items := make([]*QueueItem, len(m.queue))
	copy(items, m.queue)
	return items
}

// ========== 自动扩缩容 ==========

// CreateAutoScalePolicy 创建自动扩缩容策略
func (m *Manager) CreateAutoScalePolicy(containerName string, policy *AutoScalePolicy) (*AutoScalePolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if containerName == "" {
		return nil, &ValidationError{Field: "container_name", Message: "container_name is required"}
	}

	policy.ID = uuid.New().String()
	policy.ContainerName = containerName
	if policy.Cooldown == 0 {
		policy.Cooldown = DefaultCooldown
	}

	m.autoScale[containerName] = policy
	return policy, nil
}

// GetAutoScalePolicy 获取自动扩缩容策略
func (m *Manager) GetAutoScalePolicy(containerName string) (*AutoScalePolicy, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.autoScale[containerName]
	if !ok {
		return nil, &NotFoundError{Resource: "auto_scale_policy", ID: containerName}
	}
	return policy, nil
}

// UpdateAutoScalePolicy 更新自动扩缩容策略
func (m *Manager) UpdateAutoScalePolicy(containerName string, req UpdateAutoScaleRequest) (*AutoScalePolicy, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	policy, ok := m.autoScale[containerName]
	if !ok {
		return nil, &NotFoundError{Resource: "auto_scale_policy", ID: containerName}
	}

	if req.Enabled != nil {
		policy.Enabled = *req.Enabled
	}
	if req.MinReplicas != nil {
		policy.MinReplicas = *req.MinReplicas
	}
	if req.MaxReplicas != nil {
		policy.MaxReplicas = *req.MaxReplicas
	}
	if req.Metrics != nil {
		policy.Metrics = req.Metrics
	}
	if req.Cooldown != nil {
		policy.Cooldown = *req.Cooldown
	}
	if req.ScaleUpStep != nil {
		policy.ScaleUpStep = *req.ScaleUpStep
	}
	if req.ScaleDownStep != nil {
		policy.ScaleDownStep = *req.ScaleDownStep
	}

	return policy, nil
}

// EvaluateAutoScale 评估是否需要扩缩容
func (m *Manager) EvaluateAutoScale(containerName string) (string, int, string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	policy, ok := m.autoScale[containerName]
	if !ok {
		return "", 0, "", &NotFoundError{Resource: "auto_scale_policy", ID: containerName}
	}

	if !policy.Enabled {
		return "none", 0, "auto scale disabled", nil
	}

	// 检查冷却时间
	if policy.LastScaleAt != nil {
		if time.Since(*policy.LastScaleAt) < policy.Cooldown {
			return "none", 0, "in cooldown period", nil
		}
	}

	// 评估指标
	shouldScaleUp := false
	shouldScaleDown := true
	scaleReason := ""

	for _, metric := range policy.Metrics {
		switch metric.Type {
		case MetricTypeCPU:
			if metric.Current > metric.Target {
				shouldScaleUp = true
				scaleReason = fmt.Sprintf("CPU usage %.1f%% exceeds target %.1f%%", metric.Current, metric.Target)
			}
			if metric.Current > metric.Target*0.5 {
				shouldScaleDown = false
			}
		case MetricTypeMemory:
			if metric.Current > metric.Target {
				shouldScaleUp = true
				scaleReason = fmt.Sprintf("Memory usage %.1f%% exceeds target %.1f%%", metric.Current, metric.Target)
			}
			if metric.Current > metric.Target*0.5 {
				shouldScaleDown = false
			}
		}
	}

	// 计算当前副本数（通过放置记录统计）
	currentReplicas := 0
	for _, p := range m.placements {
		if p.ContainerName == containerName {
			currentReplicas++
		}
	}

	if shouldScaleUp && currentReplicas < policy.MaxReplicas {
		newReplicas := currentReplicas + policy.ScaleUpStep
		if newReplicas > policy.MaxReplicas {
			newReplicas = policy.MaxReplicas
		}
		return "scale_up", newReplicas, scaleReason, nil
	}

	if shouldScaleDown && currentReplicas > policy.MinReplicas {
		newReplicas := currentReplicas - policy.ScaleDownStep
		if newReplicas < policy.MinReplicas {
			newReplicas = policy.MinReplicas
		}
		return "scale_down", newReplicas, "low resource usage", nil
	}

	return "none", currentReplicas, "no scaling needed", nil
}

// ========== 节能模式 ==========

// GetPowerSaveConfig 获取节能模式配置
func (m *Manager) GetPowerSaveConfig() *PowerSaveConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.powerSave
}

// UpdatePowerSaveConfig 更新节能模式配置
func (m *Manager) UpdatePowerSaveConfig(req UpdatePowerSaveRequest) *PowerSaveConfig {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Enabled != nil {
		m.powerSave.Enabled = *req.Enabled
	}
	if req.Threshold != nil {
		m.powerSave.Threshold = *req.Threshold
	}
	if req.MinActiveNodes != nil {
		m.powerSave.MinActiveNodes = *req.MinActiveNodes
	}
	if req.ConsolidationTime != nil {
		m.powerSave.ConsolidationTime = *req.ConsolidationTime
	}

	return m.powerSave
}

// EvaluatePowerSave 评估节能模式
func (m *Manager) EvaluatePowerSave() ([]string, []string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if !m.powerSave.Enabled {
		return nil, nil, nil
	}

	// 计算集群整体负载
	totalCPUUsage := 0.0
	totalMemoryUsage := 0.0
	nodeCount := 0

	for _, node := range m.nodes {
		if node.Status == NodeStatusReady || node.Status == NodeStatusScheduling {
			totalCPUUsage += node.Resources.CPU.UsagePercent
			totalMemoryUsage += node.Resources.Memory.UsagePercent
			nodeCount++
		}
	}

	if nodeCount <= m.powerSave.MinActiveNodes {
		return nil, nil, nil
	}

	avgCPUUsage := totalCPUUsage / float64(nodeCount)
	avgMemoryUsage := totalMemoryUsage / float64(nodeCount)
	avgUsage := (avgCPUUsage + avgMemoryUsage) / 2

	// 如果负载低于阈值，建议将容器整合到少数节点
	if avgUsage < m.powerSave.Threshold*100 {
		// 找出负载最低的节点，建议将其容器迁移到其他节点
		nodesToDrain := make([]string, 0)
		nodesToKeep := make([]string, 0)

		// 按负载排序
		type nodeLoad struct {
			id    string
			usage float64
		}
		loads := make([]nodeLoad, 0)
		for _, node := range m.nodes {
			if node.Status == NodeStatusReady {
				usage := (node.Resources.CPU.UsagePercent + node.Resources.Memory.UsagePercent) / 2
				loads = append(loads, nodeLoad{id: node.ID, usage: usage})
			}
		}
		sort.Slice(loads, func(i, j int) bool {
			return loads[i].usage < loads[j].usage
		})

		// 保留负载高的节点，排空负载低的节点
		for i, load := range loads {
			if i < len(loads)-m.powerSave.MinActiveNodes {
				nodesToDrain = append(nodesToDrain, load.id)
			} else {
				nodesToKeep = append(nodesToKeep, load.id)
			}
		}

		return nodesToDrain, nodesToKeep, nil
	}

	return nil, nil, nil
}

// ========== 容器放置管理 ==========

// GetPlacement 获取容器放置信息
func (m *Manager) GetPlacement(containerID string) (*Placement, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	placement, ok := m.placements[containerID]
	if !ok {
		return nil, &NotFoundError{Resource: "placement", ID: containerID}
	}
	return placement, nil
}

// ListPlacements 列出所有容器放置
func (m *Manager) ListPlacements() []*Placement {
	m.mu.RLock()
	defer m.mu.RUnlock()

	placements := make([]*Placement, 0, len(m.placements))
	for _, p := range m.placements {
		placements = append(placements, p)
	}
	return placements
}

// RemovePlacement 移除容器放置
func (m *Manager) RemovePlacement(containerID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.placements[containerID]; !ok {
		return &NotFoundError{Resource: "placement", ID: containerID}
	}

	delete(m.placements, containerID)
	m.stats.TotalContainers--
	return nil
}

// ========== 统计 ==========

// GetStats 获取调度统计
func (m *Manager) GetStats() *ScheduleStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := *m.stats
	stats.ActiveNodes = 0
	for _, node := range m.nodes {
		if node.Status == NodeStatusReady || node.Status == NodeStatusScheduling {
			stats.ActiveNodes++
		}
	}
	stats.PendingInQueue = m.queue.Len()

	return &stats
}
