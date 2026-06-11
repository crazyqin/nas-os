// Package containerha 提供容器高可用故障转移功能
package containerha

import (
	"context"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/google/uuid"
)

// NewFailoverManager 创建新的故障转移管理器
func NewFailoverManager(config *ContainerHAConfig, localNodeID string) *FailoverManager {
	manager := &FailoverManager{
		config:      config,
		nodes:       make(map[string]*ContainerHANode),
		containers:  make(map[string]*ProtectedContainer),
		localNodeID: localNodeID,
		startTime:   time.Now(),
		stopCh:      make(chan struct{}),
		eventCh:     make(chan ClusterEvent, 100),
		failoverHistory: make([]FailoverEvent, 0),
	}

	// 初始化健康检查器
	manager.healthChecker = &HealthChecker{
		manager:       manager,
		checkInterval: time.Duration(config.HealthCheckInterval) * time.Second,
		timeout:       time.Duration(config.HeartbeatTimeout) * time.Second,
		stopCh:        make(chan struct{}),
		results:       make(map[string]*HealthCheckResult),
	}

	// 初始化同步管理器
	manager.syncManager = &SyncManager{
		manager:      manager,
		mode:         config.SyncMode,
		syncInterval: time.Duration(config.SyncInterval) * time.Second,
		stopCh:       make(chan struct{}),
		status: SyncStatus{
			Mode:  config.SyncMode,
			State: "idle",
		},
	}

	// 初始化节点
	manager.initializeNodes()

	// 初始化受保护容器
	manager.initializeContainers()

	return manager
}

// initializeNodes 初始化节点
func (m *FailoverManager) initializeNodes() {
	m.nodeMu.Lock()
	defer m.nodeMu.Unlock()

	// 添加主节点
	primaryNode := &ContainerHANode{
		ID:           m.config.PrimaryNode.ID,
		Address:      m.config.PrimaryNode.Address,
		Port:         m.config.PrimaryNode.Port,
		Role:         "master",
		Status:       "offline",
		LastHeartbeat: time.Time{},
		Weight:       m.config.PrimaryNode.Weight,
		JoinedAt:     time.Now(),
	}
	m.nodes[primaryNode.ID] = primaryNode

	// 添加从节点
	for _, secondary := range m.config.SecondaryNodes {
		node := &ContainerHANode{
			ID:           secondary.ID,
			Address:      secondary.Address,
			Port:         secondary.Port,
			Role:         "slave",
			Status:       "offline",
			LastHeartbeat: time.Time{},
			Weight:       secondary.Weight,
			JoinedAt:     time.Now(),
		}
		m.nodes[node.ID] = node
	}

	// 确定本地节点角色
	if localNode, exists := m.nodes[m.localNodeID]; exists {
		m.isMaster = localNode.Role == "master"
		localNode.Status = "online"
		localNode.LastHeartbeat = time.Now()
	}
}

// initializeContainers 初始化受保护容器
func (m *FailoverManager) initializeContainers() {
	m.containerMu.Lock()
	defer m.containerMu.Unlock()

	for _, containerConfig := range m.config.ProtectedContainers {
		container := &ProtectedContainer{
			ContainerID:  containerConfig.ContainerID,
			Type:         containerConfig.Type,
			Name:         containerConfig.ContainerID, // 使用ID作为默认名称
			Status:       "unknown",
			CurrentNode:  m.localNodeID,
			OriginalNode: m.localNodeID,
			StaticIP:     containerConfig.StaticIP,
			HealthStatus: "unknown",
			Priority:     containerConfig.Priority,
		}
		m.containers[containerConfig.ContainerID] = container
	}
}

// Start 启动故障转移管理器
func (m *FailoverManager) Start(ctx context.Context) error {
	log.Printf("[ContainerHA] 启动故障转移管理器，节点ID: %s", m.localNodeID)

	// 更新状态
	m.statusMu.Lock()
	m.status = &ContainerHAStatus{
		ClusterName:    m.config.ClusterName,
		ClusterStatus:  "initializing",
		ActiveMaster:   m.getMasterNodeID(),
		StartTime:      time.Now(),
	}
	m.statusMu.Unlock()

	// 启动健康检查器
	go m.healthChecker.Start(ctx)

	// 启动同步管理器
	go m.syncManager.Start(ctx)

	// 启动心跳监听
	go m.startHeartbeatListener(ctx)

	// 启动故障检测循环
	go m.startFailureDetection(ctx)

	// 如果是主节点，启动自动回切检查
	if m.isMaster && m.config.AutoFailback {
		go m.startAutoFailbackCheck(ctx)
	}

	// 更新状态为健康
	m.statusMu.Lock()
	m.status.ClusterStatus = "healthy"
	m.statusMu.Unlock()

	log.Printf("[ContainerHA] 故障转移管理器启动完成")

	return nil
}

// Stop 停止故障转移管理器
func (m *FailoverManager) Stop() {
	log.Printf("[ContainerHA] 停止故障转移管理器")

	close(m.stopCh)
	m.healthChecker.Stop()
	m.syncManager.Stop()

	log.Printf("[ContainerHA] 故障转移管理器已停止")
}

// getMasterNodeID 获取主节点ID
func (m *FailoverManager) getMasterNodeID() string {
	m.nodeMu.RLock()
	defer m.nodeMu.RUnlock()

	for _, node := range m.nodes {
		if node.Role == "master" && node.Status == "online" {
			return node.ID
		}
	}
	return ""
}

// startHeartbeatListener 启动心跳监听
func (m *FailoverManager) startHeartbeatListener(ctx context.Context) {
	log.Printf("[ContainerHA] 启动心跳监听")

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		default:
			// 接收并处理心跳消息
			// 实际实现中这里应该监听UDP/TCP端口
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// ProcessHeartbeat 处理心跳消息
func (m *FailoverManager) ProcessHeartbeat(heartbeat *HeartbeatMessage) error {
	m.nodeMu.Lock()
	defer m.nodeMu.Unlock()

	node, exists := m.nodes[heartbeat.NodeID]
	if !exists {
		return fmt.Errorf("未知节点: %s", heartbeat.NodeID)
	}

	// 更新节点状态
	node.LastHeartbeat = heartbeat.Timestamp
	node.Status = heartbeat.Status
	node.ResourceUsage = heartbeat.ResourceUsage
	node.HealthScore = m.calculateHealthScore(heartbeat.ResourceUsage)

	// 更新容器状态
	m.updateContainerStates(heartbeat.NodeID, heartbeat.ContainerStates)

	return nil
}

// calculateHealthScore 计算健康分数
func (m *FailoverManager) calculateHealthScore(usage ResourceUsage) int {
	score := 100

	// CPU使用率影响
	if usage.CPUUsage > 90 {
		score -= 30
	} else if usage.CPUUsage > 70 {
		score -= 15
	}

	// 内存使用率影响
	if usage.MemoryUsage > 90 {
		score -= 30
	} else if usage.MemoryUsage > 70 {
		score -= 15
	}

	// 磁盘使用率影响
	if usage.DiskUsage > 95 {
		score -= 20
	} else if usage.DiskUsage > 80 {
		score -= 10
	}

	if score < 0 {
		score = 0
	}

	return score
}

// updateContainerStates 更新容器状态
func (m *FailoverManager) updateContainerStates(nodeID string, states []ContainerState) {
	m.containerMu.Lock()
	defer m.containerMu.Unlock()

	for _, state := range states {
		if container, exists := m.containers[state.ContainerID]; exists {
			container.Status = state.Status
			container.CurrentNode = nodeID

			// 更新健康状态
			if state.Status == "running" {
				container.HealthStatus = "healthy"
			} else {
				container.HealthStatus = "unhealthy"
			}
		}
	}
}

// startFailureDetection 启动故障检测循环
func (m *FailoverManager) startFailureDetection(ctx context.Context) {
	log.Printf("[ContainerHA] 启动故障检测")

	ticker := time.NewTicker(time.Duration(m.config.HealthCheckInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.detectFailures()
		}
	}
}

// detectFailures 检测故障
func (m *FailoverManager) detectFailures() {
	m.nodeMu.RLock()
	nodesToCheck := make([]*ContainerHANode, 0)
	for _, node := range m.nodes {
		if node.ID != m.localNodeID {
			nodesToCheck = append(nodesToCheck, node)
		}
	}
	m.nodeMu.RUnlock()

	for _, node := range nodesToCheck {
		if m.isNodeFailed(node) {
			log.Printf("[ContainerHA] 检测到节点故障: %s", node.ID)
			m.handleNodeFailure(node)
		}
	}
}

// isNodeFailed 判断节点是否故障
func (m *FailoverManager) isNodeFailed(node *ContainerHANode) bool {
	// 检查心跳超时
	heartbeatTimeout := time.Duration(m.config.HeartbeatTimeout) * time.Second
	if time.Since(node.LastHeartbeat) > heartbeatTimeout {
		log.Printf("[ContainerHA] 节点 %s 心跳超时", node.ID)
		return true
	}

	// 检查健康分数
	if node.HealthScore < 20 {
		log.Printf("[ContainerHA] 节点 %s 健康分数过低: %d", node.ID, node.HealthScore)
		return true
	}

	// 检查资源阈值
	if m.config.EnableResourceCheck {
		if m.isResourceExhausted(node.ResourceUsage) {
			log.Printf("[ContainerHA] 节点 %s 资源耗尽", node.ID)
			return true
		}
	}

	return false
}

// isResourceExhausted 判断资源是否耗尽
func (m *FailoverManager) isResourceExhausted(usage ResourceUsage) bool {
	thresholds := m.config.ResourceThresholds

	if thresholds.CPUThreshold > 0 && usage.CPUUsage > thresholds.CPUThreshold {
		return true
	}
	if thresholds.MemoryThreshold > 0 && usage.MemoryUsage > thresholds.MemoryThreshold {
		return true
	}
	if thresholds.DiskThreshold > 0 && usage.DiskUsage > thresholds.DiskThreshold {
		return true
	}

	return false
}

// handleNodeFailure 处理节点故障
func (m *FailoverManager) handleNodeFailure(failedNode *ContainerHANode) {
	// 更新节点状态
	m.nodeMu.Lock()
	failedNode.Status = "offline"
	m.nodeMu.Unlock()

	// 获取该节点上运行的受保护容器
	affectedContainers := m.getContainersOnNode(failedNode.ID)

	if len(affectedContainers) == 0 {
		log.Printf("[ContainerHA] 节点 %s 上没有受保护容器", failedNode.ID)
		return
	}

	// 执行故障转移
	request := &FailoverRequest{
		Containers: affectedContainers,
		Reason:     fmt.Sprintf("节点故障: %s", failedNode.ID),
		Force:      true,
	}

	_, err := m.ExecuteFailover(request)
	if err != nil {
		log.Printf("[ContainerHA] 故障转移失败: %v", err)
	}
}

// getContainersOnNode 获取节点上运行的容器
func (m *FailoverManager) getContainersOnNode(nodeID string) []string {
	m.containerMu.RLock()
	defer m.containerMu.RUnlock()

	containers := make([]string, 0)
	for _, container := range m.containers {
		if container.CurrentNode == nodeID && container.Status == "running" {
			containers = append(containers, container.ContainerID)
		}
	}

	return containers
}

// ExecuteFailover 执行故障转移
func (m *FailoverManager) ExecuteFailover(request *FailoverRequest) (*FailoverResponse, error) {
	log.Printf("[ContainerHA] 开始故障转移，原因: %s", request.Reason)

	startTime := time.Now()
	eventID := uuid.New().String()

	// 确定目标节点
	targetNodeID := request.TargetNode
	if targetNodeID == "" {
		var err error
		targetNodeID, err = m.selectTargetNode(request.Containers)
		if err != nil {
			return nil, fmt.Errorf("选择目标节点失败: %v", err)
		}
	}

	// 验证目标节点
	if !m.isNodeAvailable(targetNodeID) {
		return nil, fmt.Errorf("目标节点不可用: %s", targetNodeID)
	}

	// 执行容器迁移
	affectedContainers := make([]string, 0)
	warnings := make([]string, 0)

	for _, containerID := range request.Containers {
		err := m.migrateContainer(containerID, targetNodeID, request.Force)
		if err != nil {
			warnings = append(warnings, fmt.Sprintf("容器 %s 迁移失败: %v", containerID, err))
			continue
		}
		affectedContainers = append(affectedContainers, containerID)
	}

	// 记录故障转移事件
	event := FailoverEvent{
		EventID:            eventID,
		Timestamp:          startTime,
		Type:               "failover",
		TargetNode:         targetNodeID,
		AffectedContainers: affectedContainers,
		Reason:             request.Reason,
		Duration:           time.Since(startTime),
	}

	if len(warnings) > 0 {
		event.Status = "partial"
		event.ErrorMessage = fmt.Sprintf("部分容器迁移失败: %v", warnings)
	} else {
		event.Status = "success"
	}

	m.recordFailoverEvent(event)

	// 处理静态IP故障转移
	if m.config.EnableStaticIP {
		m.handleStaticIPFailover(affectedContainers, targetNodeID)
	}

	// 更新状态
	m.updateStatus()

	log.Printf("[ContainerHA] 故障转移完成，耗时: %v", event.Duration)

	return &FailoverResponse{
		Success:            len(affectedContainers) > 0,
		EventID:            eventID,
		Message:            fmt.Sprintf("故障转移完成，%d 个容器已迁移", len(affectedContainers)),
		AffectedContainers: affectedContainers,
		TargetNode:         targetNodeID,
		Warnings:           warnings,
	}, nil
}

// selectTargetNode 选择目标节点
func (m *FailoverManager) selectTargetNode(containers []string) (string, error) {
	m.nodeMu.RLock()
	defer m.nodeMu.RUnlock()

	bestNode := ""
	bestScore := -1

	for _, node := range m.nodes {
		// 跳过当前运行容器的节点
		if m.hasContainersOnNode(node.ID, containers) {
			continue
		}

		// 跳过离线节点
		if node.Status != "online" {
			continue
		}

		// 选择健康分数最高的节点
		if node.HealthScore > bestScore {
			bestScore = node.HealthScore
			bestNode = node.ID
		}
	}

	if bestNode == "" {
		return "", fmt.Errorf("没有可用的目标节点")
	}

	return bestNode, nil
}

// hasContainersOnNode 检查节点上是否有指定容器
func (m *FailoverManager) hasContainersOnNode(nodeID string, containers []string) bool {
	m.containerMu.RLock()
	defer m.containerMu.RUnlock()

	for _, containerID := range containers {
		if container, exists := m.containers[containerID]; exists {
			if container.CurrentNode == nodeID {
				return true
			}
		}
	}
	return false
}

// isNodeAvailable 检查节点是否可用
func (m *FailoverManager) isNodeAvailable(nodeID string) bool {
	m.nodeMu.RLock()
	defer m.nodeMu.RUnlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return false
	}

	return node.Status == "online" && node.HealthScore > 50
}

// migrateContainer 迁移容器
func (m *FailoverManager) migrateContainer(containerID, targetNodeID string, force bool) error {
	m.containerMu.Lock()
	container, exists := m.containers[containerID]
	if !exists {
		m.containerMu.Unlock()
		return fmt.Errorf("容器不存在: %s", containerID)
	}

	sourceNodeID := container.CurrentNode
	m.containerMu.Unlock()

	log.Printf("[ContainerHA] 迁移容器 %s 从 %s 到 %s", containerID, sourceNodeID, targetNodeID)

	// 1. 在目标节点创建检查点（如果不是强制迁移）
	if !force {
		err := m.createCheckpoint(containerID, sourceNodeID)
		if err != nil {
			log.Printf("[ContainerHA] 创建检查点失败: %v", err)
			// 继续尝试直接迁移
		}
	}

	// 2. 停止源节点上的容器
	err := m.stopContainer(containerID, sourceNodeID)
	if err != nil {
		return fmt.Errorf("停止容器失败: %v", err)
	}

	// 3. 同步容器数据到目标节点
	err = m.syncContainerData(containerID, sourceNodeID, targetNodeID)
	if err != nil {
		return fmt.Errorf("同步容器数据失败: %v", err)
	}

	// 4. 在目标节点启动容器
	err = m.startContainer(containerID, targetNodeID)
	if err != nil {
		return fmt.Errorf("启动容器失败: %v", err)
	}

	// 5. 更新容器状态
	m.containerMu.Lock()
	container.CurrentNode = targetNodeID
	container.FailoverCount++
	m.containerMu.Unlock()

	return nil
}

// createCheckpoint 创建检查点
func (m *FailoverManager) createCheckpoint(containerID, nodeID string) error {
	log.Printf("[ContainerHA] 为容器 %s 创建检查点", containerID)

	// 实际实现中，这里应该调用 LXC/Docker 的检查点 API
	// 例如：lxc checkpoint 或 docker checkpoint

	checkpoint := &Checkpoint{
		ContainerID: containerID,
		NodeID:      nodeID,
		Path:        fmt.Sprintf("/var/lib/containerha/checkpoints/%s/%s", containerID, time.Now().Format("20060102150405")),
		Timestamp:   time.Now(),
		Status:      "ready",
	}

	// 保存检查点信息
	m.saveCheckpoint(checkpoint)

	return nil
}

// saveCheckpoint 保存检查点信息
func (m *FailoverManager) saveCheckpoint(checkpoint *Checkpoint) {
	// 实际实现中，这里应该将检查点信息持久化存储
	log.Printf("[ContainerHA] 保存检查点: %s", checkpoint.Path)
}

// stopContainer 停止容器
func (m *FailoverManager) stopContainer(containerID, nodeID string) error {
	log.Printf("[ContainerHA] 在节点 %s 停止容器 %s", nodeID, containerID)

	// 实际实现中，这里应该调用容器运行时 API
	// 例如：docker stop 或 lxc-stop

	return nil
}

// syncContainerData 同步容器数据
func (m *FailoverManager) syncContainerData(containerID, sourceNode, targetNode string) error {
	log.Printf("[ContainerHA] 同步容器 %s 数据从 %s 到 %s", containerID, sourceNode, targetNode)

	// 实际实现中，这里应该使用 rsync 或其他工具同步容器文件系统
	// 包括容器配置、数据卷等

	return nil
}

// startContainer 启动容器
func (m *FailoverManager) startContainer(containerID, nodeID string) error {
	log.Printf("[ContainerHA] 在节点 %s 启动容器 %s", nodeID, containerID)

	// 实际实现中，这里应该调用容器运行时 API
	// 例如：docker start 或 lxc-start

	return nil
}

// handleStaticIPFailover 处理静态IP故障转移
func (m *FailoverManager) handleStaticIPFailover(containers []string, targetNode string) {
	for _, containerID := range containers {
		m.containerMu.RLock()
		container, exists := m.containers[containerID]
		if !exists || container.StaticIP == "" {
			m.containerMu.RUnlock()
			continue
		}
		staticIP := container.StaticIP
		m.containerMu.RUnlock()

		// 迁移虚拟IP
		for _, vip := range m.config.VirtualIPs {
			if vip.IP == staticIP {
				err := m.migrateVirtualIP(vip, targetNode)
				if err != nil {
					log.Printf("[ContainerHA] 迁移虚拟IP失败: %v", err)
				}
			}
		}
	}
}

// migrateVirtualIP 迁移虚拟IP
func (m *FailoverManager) migrateVirtualIP(vip VirtualIPConfig, targetNode string) error {
	log.Printf("[ContainerHA] 迁移虚拟IP %s 到节点 %s", vip.IP, targetNode)

	// 获取目标节点地址
	m.nodeMu.RLock()
	node, exists := m.nodes[targetNode]
	if !exists {
		m.nodeMu.RUnlock()
		return fmt.Errorf("目标节点不存在: %s", targetNode)
	}
	targetAddr := node.Address
	m.nodeMu.RUnlock()

	// 实际实现中，这里应该：
	// 1. 在源节点移除虚拟IP
	// 2. 在目标节点添加虚拟IP
	// 3. 更新ARP表

	// 这里使用简单的网络操作示例
	conn, err := net.DialTimeout("tcp", fmt.Sprintf("%s:22", targetAddr), 5*time.Second)
	if err != nil {
		return fmt.Errorf("无法连接到目标节点: %v", err)
	}
	conn.Close()

	return nil
}

// recordFailoverEvent 记录故障转移事件
func (m *FailoverManager) recordFailoverEvent(event FailoverEvent) {
	m.historyMu.Lock()
	defer m.historyMu.Unlock()

	m.failoverHistory = append(m.failoverHistory, event)

	// 保留最近100个事件
	if len(m.failoverHistory) > 100 {
		m.failoverHistory = m.failoverHistory[len(m.failoverHistory)-100:]
	}

	// 更新最后故障转移时间
	m.statusMu.Lock()
	if m.status != nil {
		now := time.Now()
		m.status.LastFailoverTime = &now
		m.status.LastFailoverReason = event.Reason
		m.status.FailoverHistory = m.failoverHistory
	}
	m.statusMu.Unlock()
}

// updateStatus 更新状态
func (m *FailoverManager) updateStatus() {
	m.statusMu.Lock()
	defer m.statusMu.Unlock()

	if m.status == nil {
		return
	}

	// 更新节点信息
	m.nodeMu.RLock()
	nodes := make([]ContainerHANode, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, *node)
	}
	m.nodeMu.RUnlock()
	m.status.Nodes = nodes

	// 更新运行中的容器
	m.containerMu.RLock()
	containers := make([]ProtectedContainer, 0, len(m.containers))
	for _, container := range m.containers {
		if container.Status == "running" {
			containers = append(containers, *container)
		}
	}
	m.containerMu.RUnlock()
	m.status.RunningContainers = containers

	// 更新集群状态
	m.status.ClusterStatus = m.calculateClusterStatus()
	m.status.ActiveMaster = m.getMasterNodeID()
	m.status.Uptime = time.Since(m.startTime)

	// 更新同步状态
	m.syncManager.statusMu.RLock()
	m.status.SyncStatus = m.syncManager.status
	m.syncManager.statusMu.RUnlock()
}

// calculateClusterStatus 计算集群状态
func (m *FailoverManager) calculateClusterStatus() string {
	m.nodeMu.RLock()
	defer m.nodeMu.RUnlock()

	onlineCount := 0
	totalCount := len(m.nodes)

	for _, node := range m.nodes {
		if node.Status == "online" {
			onlineCount++
		}
	}

	// 所有节点在线
	if onlineCount == totalCount {
		return "healthy"
	}

	// 超过一半节点在线
	if onlineCount > totalCount/2 {
		return "degraded"
	}

	// 不到一半节点在线
	return "critical"
}

// startAutoFailbackCheck 启动自动回切检查
func (m *FailoverManager) startAutoFailbackCheck(ctx context.Context) {
	log.Printf("[ContainerHA] 启动自动回切检查")

	failbackDelay := time.Duration(m.config.FailbackDelay) * time.Second
	ticker := time.NewTicker(failbackDelay)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkAndFailback()
		}
	}
}

// checkAndFailback 检查并执行回切
func (m *FailoverManager) checkAndFailback() {
	// 获取原始主节点
	originalMasterID := m.config.PrimaryNode.ID

	// 检查原始主节点是否在线
	m.nodeMu.RLock()
	originalMaster, exists := m.nodes[originalMasterID]
	m.nodeMu.RUnlock()

	if !exists || originalMaster.Status != "online" {
		return
	}

	// 检查是否需要回切
	m.containerMu.RLock()
	needFailback := false
	containersToFailback := make([]string, 0)

	for _, container := range m.containers {
		if container.OriginalNode == originalMasterID && container.CurrentNode != originalMasterID {
			needFailback = true
			containersToFailback = append(containersToFailback, container.ContainerID)
		}
	}
	m.containerMu.RUnlock()

	if !needFailback {
		return
	}

	log.Printf("[ContainerHA] 执行自动回切到主节点 %s", originalMasterID)

	// 执行回切
	request := &FailoverRequest{
		TargetNode: originalMasterID,
		Containers: containersToFailback,
		Reason:     "自动回切到主节点",
		Planned:    true,
	}

	_, err := m.ExecuteFailover(request)
	if err != nil {
		log.Printf("[ContainerHA] 自动回切失败: %v", err)
	}
}

// GetStatus 获取当前状态
func (m *FailoverManager) GetStatus() *ContainerHAStatus {
	m.updateStatus()

	m.statusMu.RLock()
	defer m.statusMu.RUnlock()

	return m.status
}

// GetNodes 获取所有节点信息
func (m *FailoverManager) GetNodes() []ContainerHANode {
	m.nodeMu.RLock()
	defer m.nodeMu.RUnlock()

	nodes := make([]ContainerHANode, 0, len(m.nodes))
	for _, node := range m.nodes {
		nodes = append(nodes, *node)
	}

	return nodes
}

// GetNode 获取指定节点信息
func (m *FailoverManager) GetNode(nodeID string) (*ContainerHANode, error) {
	m.nodeMu.RLock()
	defer m.nodeMu.RUnlock()

	node, exists := m.nodes[nodeID]
	if !exists {
		return nil, fmt.Errorf("节点不存在: %s", nodeID)
	}

	return node, nil
}

// GetConfig 获取配置
func (m *FailoverManager) GetConfig() *ContainerHAConfig {
	return m.config
}

// UpdateConfig 更新配置
func (m *FailoverManager) UpdateConfig(config *ContainerHAConfig) error {
	// 验证配置
	if err := m.validateConfig(config); err != nil {
		return fmt.Errorf("配置验证失败: %v", err)
	}

	m.config = config

	// 更新健康检查器配置
	m.healthChecker.checkInterval = time.Duration(config.HealthCheckInterval) * time.Second
	m.healthChecker.timeout = time.Duration(config.HeartbeatTimeout) * time.Second

	// 更新同步管理器配置
	m.syncManager.mode = config.SyncMode
	m.syncManager.syncInterval = time.Duration(config.SyncInterval) * time.Second

	log.Printf("[ContainerHA] 配置已更新")

	return nil
}

// validateConfig 验证配置
func (m *FailoverManager) validateConfig(config *ContainerHAConfig) error {
	if config.ClusterName == "" {
		return fmt.Errorf("集群名称不能为空")
	}

	if config.PrimaryNode.ID == "" {
		return fmt.Errorf("主节点ID不能为空")
	}

	if config.HealthCheckInterval <= 0 {
		return fmt.Errorf("健康检查间隔必须大于0")
	}

	if config.FailureThreshold <= 0 {
		return fmt.Errorf("故障阈值必须大于0")
	}

	if config.HeartbeatTimeout <= 0 {
		return fmt.Errorf("心跳超时时间必须大于0")
	}

	return nil
}

// GetFailoverHistory 获取故障转移历史
func (m *FailoverManager) GetFailoverHistory() []FailoverEvent {
	m.historyMu.RLock()
	defer m.historyMu.RUnlock()

	history := make([]FailoverEvent, len(m.failoverHistory))
	copy(history, m.failoverHistory)

	return history
}

// IsMaster 是否为主节点
func (m *FailoverManager) IsMaster() bool {
	return m.isMaster
}

// GetLocalNodeID 获取本地节点ID
func (m *FailoverManager) GetLocalNodeID() string {
	return m.localNodeID
}
