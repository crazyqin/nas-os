// Package containerorch 提供 K3s 轻量级容器编排功能
package containerorch

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"
)

// Scheduler 调度器.
type Scheduler struct {
	mu       sync.RWMutex
	manager  *Manager
	nodes    map[string]*Node         // 节点列表
	strategy ScheduleStrategy         // 调度策略
	stopCh   chan struct{}
}

// Node 节点信息.
type Node struct {
	mu sync.RWMutex `json:"-"`

	// 基本信息
	ID         string            `json:"id"`         // 节点唯一标识
	Name       string            `json:"name"`       // 节点名称
	IP         string            `json:"ip"`         // 节点 IP
	Hostname   string            `json:"hostname"`   // 主机名

	// 状态
	Ready      bool              `json:"ready"`      // 是否就绪
	Schedulable bool             `json:"schedulable"` // 是否可调度

	// 资源
	TotalCPU    int64             `json:"totalCpu"`    // 总 CPU (毫核)
	UsedCPU     int64             `json:"usedCpu"`     // 已用 CPU (毫核)
	TotalMemory int64             `json:"totalMemory"` // 总内存 (字节)
	UsedMemory  int64             `json:"usedMemory"`  // 已用内存 (字节)

	// 容器/Pod 数量
	RunningPods int               `json:"runningPods"` // 运行中 Pod 数

	// 标签
	Labels      map[string]string `json:"labels"`      // 节点标签

	// 时间
	RegisteredAt time.Time        `json:"registeredAt"` // 注册时间
	LastHeartbeat time.Time       `json:"lastHeartbeat"` // 最后心跳
}

// GetAvailableCPU 获取可用 CPU (毫核).
func (n *Node) GetAvailableCPU() int64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.TotalCPU - n.UsedCPU
}

// GetAvailableMemory 获取可用内存 (字节).
func (n *Node) GetAvailableMemory() int64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	return n.TotalMemory - n.UsedMemory
}

// GetCPUUsage 获取 CPU 使用率.
func (n *Node) GetCPUUsage() float64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.TotalCPU == 0 {
		return 0
	}
	return float64(n.UsedCPU) / float64(n.TotalCPU)
}

// GetMemoryUsage 获取内存使用率.
func (n *Node) GetMemoryUsage() float64 {
	n.mu.RLock()
	defer n.mu.RUnlock()
	if n.TotalMemory == 0 {
		return 0
	}
	return float64(n.UsedMemory) / float64(n.TotalMemory)
}

// ScheduleStrategy 调度策略.
type ScheduleStrategy string

const (
	StrategyRoundRobin    ScheduleStrategy = "roundRobin"    // 轮询调度
	StrategyLeastLoaded   ScheduleStrategy = "leastLoaded"   // 最小负载
	StrategyBinPacking    ScheduleStrategy = "binPacking"     // 装箱策略
	StrategySpreadEven    ScheduleStrategy = "spreadEven"     // 均匀分布
	StrategyNodeAffinity  ScheduleStrategy = "nodeAffinity"   // 节点亲和性
)

// NewScheduler 创建调度器.
func NewScheduler(manager *Manager) *Scheduler {
	return &Scheduler{
		manager:  manager,
		nodes:    make(map[string]*Node),
		strategy: StrategyLeastLoaded,
		stopCh:   make(chan struct{}),
	}
}

// Start 启动调度器.
func (s *Scheduler) Start(ctx context.Context) {
	log.Printf("[Scheduler] 启动调度器，策略: %s", s.strategy)

	// 添加默认节点（当前节点）
	s.RegisterNode(&Node{
		ID:         s.manager.nodeID,
		Name:       "node-1",
		IP:         "192.168.1.100",
		Hostname:   "nas-node-1",
		Ready:      true,
		Schedulable: true,
		TotalCPU:    4000,             // 4 核 = 4000 毫核
		TotalMemory: 8 * 1024 * 1024 * 1024, // 8GB
		Labels: map[string]string{
			"kubernetes.io/hostname": "nas-node-1",
			"node.kubernetes.io/instance-type": "nas",
		},
		RegisteredAt:  time.Now(),
		LastHeartbeat: time.Now(),
	})

	// 启动心跳检查
	go s.heartbeatCheck(ctx)
}

// Stop 停止调度器.
func (s *Scheduler) Stop() {
	close(s.stopCh)
}

// RegisterNode 注册节点.
func (s *Scheduler) RegisterNode(node *Node) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.nodes[node.ID] = node
	log.Printf("[Scheduler] 节点已注册: %s (%s)", node.Name, node.ID)
}

// UnregisterNode 注销节点.
func (s *Scheduler) UnregisterNode(nodeID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.nodes, nodeID)
	log.Printf("[Scheduler] 节点已注销: %s", nodeID)
}

// GetNode 获取节点.
func (s *Scheduler) GetNode(nodeID string) (*Node, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	node, ok := s.nodes[nodeID]
	return node, ok
}

// ListNodes 列出所有节点.
func (s *Scheduler) ListNodes() []*Node {
	s.mu.RLock()
	defer s.mu.RUnlock()
	nodes := make([]*Node, 0, len(s.nodes))
	for _, n := range s.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// SchedulePod 调度 Pod 到节点.
func (s *Scheduler) SchedulePod(pod *Pod) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 获取可用节点
	availableNodes := s.getAvailableNodes()
	if len(availableNodes) == 0 {
		return "", fmt.Errorf("no available nodes")
	}

	// 根据策略选择节点
	var selectedNode *Node
	var err error

	switch s.strategy {
	case StrategyRoundRobin:
		selectedNode, err = s.scheduleRoundRobin(availableNodes)
	case StrategyLeastLoaded:
		selectedNode, err = s.scheduleLeastLoaded(availableNodes)
	case StrategyBinPacking:
		selectedNode, err = s.scheduleBinPacking(availableNodes, pod)
	case StrategySpreadEven:
		selectedNode, err = s.scheduleSpreadEven(availableNodes)
	case StrategyNodeAffinity:
		selectedNode, err = s.scheduleNodeAffinity(availableNodes, pod)
	default:
		selectedNode, err = s.scheduleLeastLoaded(availableNodes)
	}

	if err != nil {
		return "", err
	}

	// 更新节点资源
	selectedNode.mu.Lock()
	selectedNode.UsedCPU += s.estimateCPU(pod)
	selectedNode.UsedMemory += s.estimateMemory(pod)
	selectedNode.RunningPods++
	selectedNode.LastHeartbeat = time.Now()
	selectedNode.mu.Unlock()

	log.Printf("[Scheduler] Pod %s 已调度到节点 %s", pod.ID, selectedNode.ID)
	return selectedNode.ID, nil
}

// ReleaseNodeResources 释放节点资源.
func (s *Scheduler) ReleaseNodeResources(nodeID string, pod *Pod) {
	s.mu.RLock()
	node, ok := s.nodes[nodeID]
	s.mu.RUnlock()

	if !ok {
		return
	}

	node.mu.Lock()
	node.UsedCPU -= s.estimateCPU(pod)
	node.UsedMemory -= s.estimateMemory(pod)
	node.RunningPods--
	if node.UsedCPU < 0 {
		node.UsedCPU = 0
	}
	if node.UsedMemory < 0 {
		node.UsedMemory = 0
	}
	if node.RunningPods < 0 {
		node.RunningPods = 0
	}
	node.mu.Unlock()
}

// SetStrategy 设置调度策略.
func (s *Scheduler) SetStrategy(strategy ScheduleStrategy) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.strategy = strategy
	log.Printf("[Scheduler] 调度策略已更新: %s", strategy)
}

// GetStrategy 获取调度策略.
func (s *Scheduler) GetStrategy() ScheduleStrategy {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.strategy
}

// ==================== 调度算法 ====================

// getAvailableNodes 获取可用节点.
func (s *Scheduler) getAvailableNodes() []*Node {
	nodes := make([]*Node, 0)
	for _, n := range s.nodes {
		if n.Ready && n.Schedulable {
			nodes = append(nodes, n)
		}
	}
	return nodes
}

// scheduleRoundRobin 轮询调度.
func (s *Scheduler) scheduleRoundRobin(nodes []*Node) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no available nodes")
	}

	// 找到运行 Pod 最少的节点
	minPods := nodes[0]
	for _, n := range nodes[1:] {
		if n.RunningPods < minPods.RunningPods {
			minPods = n
		}
	}
	return minPods, nil
}

// scheduleLeastLoaded 最小负载调度.
func (s *Scheduler) scheduleLeastLoaded(nodes []*Node) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no available nodes")
	}

	var bestNode *Node
	bestScore := -1.0

	for _, n := range nodes {
		// 计算负载分数（越低越好）
		cpuUsage := n.GetCPUUsage()
		memUsage := n.GetMemoryUsage()
		score := 1.0 - (cpuUsage+memUsage)/2.0 // 可用资源比例

		if bestNode == nil || score > bestScore {
			bestNode = n
			bestScore = score
		}
	}

	return bestNode, nil
}

// scheduleBinPacking 装箱策略（尽量填满节点）.
func (s *Scheduler) scheduleBinPacking(nodes []*Node, pod *Pod) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no available nodes")
	}

	neededCPU := s.estimateCPU(pod)
	neededMem := s.estimateMemory(pod)

	// 找到资源最紧张但仍然能容纳的节点
	var bestNode *Node
	bestUsage := -1.0

	for _, n := range nodes {
		availCPU := n.GetAvailableCPU()
		availMem := n.GetAvailableMemory()

		// 检查资源是否足够
		if availCPU < neededCPU || availMem < neededMem {
			continue
		}

		// 计算使用率（越高越优先，实现装箱效果）
		usage := (n.GetCPUUsage() + n.GetMemoryUsage()) / 2.0
		if bestNode == nil || usage > bestUsage {
			bestNode = n
			bestUsage = usage
		}
	}

	if bestNode == nil {
		return nil, fmt.Errorf("no node has enough resources")
	}

	return bestNode, nil
}

// scheduleSpreadEven 均匀分布调度.
func (s *Scheduler) scheduleSpreadEven(nodes []*Node) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no available nodes")
	}

	// 找到 Pod 最少的节点
	var bestNode *Node
	minPods := -1

	for _, n := range nodes {
		if bestNode == nil || n.RunningPods < minPods {
			bestNode = n
			minPods = n.RunningPods
		}
	}

	return bestNode, nil
}

// scheduleNodeAffinity 节点亲和性调度.
func (s *Scheduler) scheduleNodeAffinity(nodes []*Node, pod *Pod) (*Node, error) {
	if len(nodes) == 0 {
		return nil, fmt.Errorf("no available nodes")
	}

	// 检查 Pod 的节点选择器
	if len(pod.Spec.NodeSelector) == 0 {
		// 没有节点选择器，使用最小负载策略
		return s.scheduleLeastLoaded(nodes)
	}

	// 查找匹配的节点
	matchedNodes := make([]*Node, 0)
	for _, n := range nodes {
		if s.matchNodeSelector(n, pod.Spec.NodeSelector) {
			matchedNodes = append(matchedNodes, n)
		}
	}

	if len(matchedNodes) == 0 {
		return nil, fmt.Errorf("no node matches the node selector")
	}

	// 在匹配的节点中选择负载最小的
	return s.scheduleLeastLoaded(matchedNodes)
}

// matchNodeSelector 匹配节点选择器.
func (s *Scheduler) matchNodeSelector(node *Node, selector map[string]string) bool {
	for key, value := range selector {
		nodeValue, ok := node.Labels[key]
		if !ok || nodeValue != value {
			return false
		}
	}
	return true
}

// ==================== 资源估算 ====================

// estimateCPU 估算 Pod 所需 CPU (毫核).
func (s *Scheduler) estimateCPU(pod *Pod) int64 {
	var totalCPU int64
	for _, container := range pod.Containers {
		// 解析 CPU 请求
		if container.Resources.Requests.CPU != "" {
			totalCPU += parseCPU(container.Resources.Requests.CPU)
		} else {
			totalCPU += 100 // 默认 100 毫核
		}
	}
	return totalCPU
}

// estimateMemory 估算 Pod 所需内存 (字节).
func (s *Scheduler) estimateMemory(pod *Pod) int64 {
	var totalMem int64
	for _, container := range pod.Containers {
		// 解析内存请求
		if container.Resources.Requests.Memory != "" {
			totalMem += parseMemory(container.Resources.Requests.Memory)
		} else {
			totalMem += 128 * 1024 * 1024 // 默认 128MB
		}
	}
	return totalMem
}

// parseCPU 解析 CPU 资源字符串.
func parseCPU(cpu string) int64 {
	// 简化实现：支持 "100m" 和 "1" 格式
	if len(cpu) == 0 {
		return 100
	}

	// 如果以 "m" 结尾，表示毫核
	if cpu[len(cpu)-1] == 'm' {
		// 解析数字部分
		var val int64
		for _, c := range cpu[:len(cpu)-1] {
			if c >= '0' && c <= '9' {
				val = val*10 + int64(c-'0')
			}
		}
		return val
	}

	// 否则表示核数
	var val int64
	for _, c := range cpu {
		if c >= '0' && c <= '9' {
			val = val*10 + int64(c-'0')
		}
	}
	return val * 1000
}

// parseMemory 解析内存资源字符串.
func parseMemory(mem string) int64 {
	// 简化实现：支持 "128Mi", "1Gi" 格式
	if len(mem) == 0 {
		return 128 * 1024 * 1024
	}

	// 解析数字和单位
	var val int64
	var unit string
	for i, c := range mem {
		if c >= '0' && c <= '9' {
			val = val*10 + int64(c-'0')
		} else {
			unit = mem[i:]
			break
		}
	}

	switch unit {
	case "Ki":
		return val * 1024
	case "Mi":
		return val * 1024 * 1024
	case "Gi":
		return val * 1024 * 1024 * 1024
	case "Ti":
		return val * 1024 * 1024 * 1024 * 1024
	default:
		return val
	}
}

// ==================== 心跳检查 ====================

// heartbeatCheck 心跳检查.
func (s *Scheduler) heartbeatCheck(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-s.stopCh:
			return
		case <-ticker.C:
			s.checkNodeHeartbeats()
		}
	}
}

// checkNodeHeartbeats 检查节点心跳.
func (s *Scheduler) checkNodeHeartbeats() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for _, node := range s.nodes {
		// 如果超过 2 分钟没有心跳，标记为未就绪
		if now.Sub(node.LastHeartbeat) > 2*time.Minute {
			if node.Ready {
				node.Ready = false
				log.Printf("[Scheduler] 节点未就绪: %s (心跳超时)", node.ID)
			}
		}
	}
}

// Heartbeat 更新节点心跳.
func (s *Scheduler) Heartbeat(nodeID string) error {
	s.mu.RLock()
	node, ok := s.nodes[nodeID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	node.mu.Lock()
	node.LastHeartbeat = time.Now()
	node.Ready = true
	node.mu.Unlock()

	return nil
}

// UpdateNodeResources 更新节点资源.
func (s *Scheduler) UpdateNodeResources(nodeID string, usedCPU, usedMemory int64) error {
	s.mu.RLock()
	node, ok := s.nodes[nodeID]
	s.mu.RUnlock()

	if !ok {
		return fmt.Errorf("node not found: %s", nodeID)
	}

	node.mu.Lock()
	node.UsedCPU = usedCPU
	node.UsedMemory = usedMemory
	node.mu.Unlock()

	return nil
}
