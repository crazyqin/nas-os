// Package containerha 提供容器高可用故障转移功能
package containerha

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"sync"
	"time"
)

// Start 启动健康检查器
func (hc *HealthChecker) Start(ctx context.Context) {
	log.Printf("[ContainerHA] 启动健康检查器，间隔: %v", hc.checkInterval)

	ticker := time.NewTicker(hc.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-hc.stopCh:
			return
		case <-ticker.C:
			hc.runChecks()
		}
	}
}

// Stop 停止健康检查器
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

// runChecks 执行健康检查
func (hc *HealthChecker) runChecks() {
	nodes := hc.manager.GetNodes()

	var wg sync.WaitGroup
	for _, node := range nodes {
		if node.ID == hc.manager.GetLocalNodeID() {
			continue // 跳过自己
		}

		wg.Add(1)
		go func(n ContainerHANode) {
			defer wg.Done()
			hc.checkNode(&n)
		}(node)
	}
	wg.Wait()
}

// checkNode 检查节点健康状态
func (hc *HealthChecker) checkNode(node *ContainerHANode) {
	startTime := time.Now()

	result := &HealthCheckResult{
		NodeID:    node.ID,
		CheckTime: startTime,
	}

	// 执行节点级健康检查
	err := hc.pingNode(node)
	responseTime := time.Since(startTime).Milliseconds()

	result.ResponseTime = responseTime

	if err != nil {
		result.Healthy = false
		result.ErrorMessage = err.Error()
		result.Failures = hc.incrementFailureCount(node.ID)
		log.Printf("[ContainerHA] 节点 %s 健康检查失败: %v (连续失败: %d)", node.ID, err, result.Failures)
	} else {
		result.Healthy = true
		result.Failures = 0
		hc.resetFailureCount(node.ID)
	}

	// 执行容器级健康检查
	containerResults := hc.checkContainersOnNode(node.ID)
	result.ContainerHealth = containerResults

	// 保存检查结果
	hc.resultsMu.Lock()
	hc.results[node.ID] = result
	hc.resultsMu.Unlock()

	// 处理检查结果
	hc.processCheckResult(result)
}

// pingNode ping节点
func (hc *HealthChecker) pingNode(node *ContainerHANode) error {
	// 尝试HTTP健康检查
	addr := net.JoinHostPort(node.Address, fmt.Sprintf("%d", node.Port))
	healthURL := fmt.Sprintf("http://%s/health", addr)
	client := &http.Client{
		Timeout: hc.timeout,
	}

	resp, err := client.Get(healthURL)
	if err != nil {
		// 尝试直接连接
		conn, err := net.DialTimeout("tcp", addr, hc.timeout)
		if err != nil {
			return fmt.Errorf("无法连接到节点: %v", err)
		}
		conn.Close()
		return nil
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("健康检查返回非200状态码: %d", resp.StatusCode)
	}

	return nil
}

// checkContainersOnNode 检查节点上的容器
func (hc *HealthChecker) checkContainersOnNode(nodeID string) []ContainerHealthResult {
	containers := hc.manager.getContainersOnNode(nodeID)
	results := make([]ContainerHealthResult, 0, len(containers))

	for _, containerID := range containers {
		result := hc.checkContainer(containerID, nodeID)
		results = append(results, result)
	}

	return results
}

// checkContainer 检查单个容器
func (hc *HealthChecker) checkContainer(containerID, nodeID string) ContainerHealthResult {
	startTime := time.Now()

	result := ContainerHealthResult{
		ContainerID: containerID,
	}

	// 实际实现中，这里应该：
	// 1. 检查容器是否在运行
	// 2. 检查容器资源使用情况
	// 3. 如果配置了HTTP健康检查，执行HTTP检查
	// 4. 检查容器日志是否有错误

	// 示例：假设容器健康
	result.Healthy = true
	result.ResponseTime = time.Since(startTime).Milliseconds()

	return result
}

// incrementFailureCount 增加失败计数
func (hc *HealthChecker) incrementFailureCount(nodeID string) int {
	hc.resultsMu.Lock()
	defer hc.resultsMu.Unlock()

	if result, exists := hc.results[nodeID]; exists {
		return result.Failures + 1
	}
	return 1
}

// resetFailureCount 重置失败计数
func (hc *HealthChecker) resetFailureCount(nodeID string) {
	hc.resultsMu.Lock()
	defer hc.resultsMu.Unlock()

	if result, exists := hc.results[nodeID]; exists {
		result.Failures = 0
	}
}

// processCheckResult 处理检查结果
func (hc *HealthChecker) processCheckResult(result *HealthCheckResult) {
	if !result.Healthy && result.Failures >= hc.manager.config.FailureThreshold {
		// 触发故障转移
		log.Printf("[ContainerHA] 节点 %s 连续失败 %d 次，触发故障转移", result.NodeID, result.Failures)

		// 获取该节点上的容器
		containers := hc.manager.getContainersOnNode(result.NodeID)
		if len(containers) > 0 {
			request := &FailoverRequest{
				Containers: containers,
				Reason:     fmt.Sprintf("健康检查失败，连续失败 %d 次", result.Failures),
			}

			_, err := hc.manager.ExecuteFailover(request)
			if err != nil {
				log.Printf("[ContainerHA] 故障转移失败: %v", err)
			}
		}
	}
}

// GetCheckResult 获取检查结果
func (hc *HealthChecker) GetCheckResult(nodeID string) *HealthCheckResult {
	hc.resultsMu.RLock()
	defer hc.resultsMu.RUnlock()

	if result, exists := hc.results[nodeID]; exists {
		return result
	}

	return nil
}

// GetAllCheckResults 获取所有检查结果
func (hc *HealthChecker) GetAllCheckResults() map[string]*HealthCheckResult {
	hc.resultsMu.RLock()
	defer hc.resultsMu.RUnlock()

	results := make(map[string]*HealthCheckResult)
	for k, v := range hc.results {
		results[k] = v
	}

	return results
}

// Start 启动同步管理器
func (sm *SyncManager) Start(ctx context.Context) {
	log.Printf("[ContainerHA] 启动同步管理器，模式: %s，间隔: %v", sm.mode, sm.syncInterval)

	ticker := time.NewTicker(sm.syncInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-sm.stopCh:
			return
		case <-ticker.C:
			sm.runSync()
		}
	}
}

// Stop 停止同步管理器
func (sm *SyncManager) Stop() {
	close(sm.stopCh)
}

// runSync 执行同步
func (sm *SyncManager) runSync() {
	sm.statusMu.Lock()
	sm.status.State = "syncing"
	sm.statusMu.Unlock()

	// 获取所有受保护容器
	containers := sm.manager.GetAllProtectedContainers()

	totalContainers := len(containers)
	syncedContainers := 0
	pendingContainers := 0
	failedSyncs := 0

	for _, container := range containers {
		if container.Status != "running" {
			continue
		}

		err := sm.syncContainer(container)
		if err != nil {
			log.Printf("[ContainerHA] 同步容器 %s 失败: %v", container.ContainerID, err)
			failedSyncs++
		} else {
			syncedContainers++
		}
		pendingContainers = totalContainers - syncedContainers - failedSyncs
	}

	// 更新同步状态
	sm.statusMu.Lock()
	now := time.Now()
	sm.status.State = "idle"
	sm.status.LastSyncTime = &now
	sm.status.SyncedContainers = syncedContainers
	sm.status.PendingContainers = pendingContainers
	sm.status.FailedSyncs = failedSyncs
	sm.status.Progress = float64(syncedContainers) / float64(totalContainers) * 100
	sm.statusMu.Unlock()

	log.Printf("[ContainerHA] 同步完成: 成功 %d, 失败 %d, 待同步 %d", syncedContainers, failedSyncs, pendingContainers)
}

// syncContainer 同步单个容器
func (sm *SyncManager) syncContainer(container *ProtectedContainer) error {
	switch sm.mode {
	case "checkpoint":
		return sm.syncWithCheckpoint(container)
	case "realtime":
		return sm.syncRealtime(container)
	default:
		return fmt.Errorf("未知的同步模式: %s", sm.mode)
	}
}

// syncWithCheckpoint 使用检查点同步
func (sm *SyncManager) syncWithCheckpoint(container *ProtectedContainer) error {
	log.Printf("[ContainerHA] 使用检查点同步容器 %s", container.ContainerID)

	// 1. 在当前节点创建检查点
	checkpoint, err := sm.createContainerCheckpoint(container)
	if err != nil {
		return fmt.Errorf("创建检查点失败: %v", err)
	}

	// 2. 同步检查点到备份节点
	err = sm.syncCheckpointToBackup(checkpoint, container)
	if err != nil {
		return fmt.Errorf("同步检查点失败: %v", err)
	}

	// 3. 更新容器的同步时间
	sm.manager.containerMu.Lock()
	if c, exists := sm.manager.containers[container.ContainerID]; exists {
		now := time.Now()
		c.LastSyncTime = &now
		c.CheckpointPath = checkpoint.Path
	}
	sm.manager.containerMu.Unlock()

	return nil
}

// createContainerCheckpoint 创建容器检查点
func (sm *SyncManager) createContainerCheckpoint(container *ProtectedContainer) (*Checkpoint, error) {
	checkpoint := &Checkpoint{
		ContainerID: container.ContainerID,
		NodeID:      container.CurrentNode,
		Path:        fmt.Sprintf("/var/lib/containerha/checkpoints/%s/%s", container.ContainerID, time.Now().Format("20060102150405")),
		Timestamp:   time.Now(),
		Status:      "creating",
	}

	// 实际实现中，这里应该调用容器运行时的检查点API
	// 对于 LXC: lxc-checkpoint
	// 对于 Docker: docker checkpoint create

	// 模拟检查点创建
	log.Printf("[ContainerHA] 为容器 %s 创建检查点: %s", container.ContainerID, checkpoint.Path)
	checkpoint.Status = "ready"

	return checkpoint, nil
}

// syncCheckpointToBackup 同步检查点到备份节点
func (sm *SyncManager) syncCheckpointToBackup(checkpoint *Checkpoint, container *ProtectedContainer) error {
	// 获取备份节点
	backupNodeID := sm.getBackupNode(container.CurrentNode)
	if backupNodeID == "" {
		return fmt.Errorf("没有可用的备份节点")
	}

	// 实际实现中，这里应该使用 rsync 或其他工具同步检查点文件
	// 例如: rsync -avz /path/to/checkpoint user@backup-node:/path/to/backup

	log.Printf("[ContainerHA] 同步检查点从 %s 到 %s", container.CurrentNode, backupNodeID)

	return nil
}

// getBackupNode 获取备份节点
func (sm *SyncManager) getBackupNode(currentNodeID string) string {
	sm.manager.nodeMu.RLock()
	defer sm.manager.nodeMu.RUnlock()

	for _, node := range sm.manager.nodes {
		if node.ID != currentNodeID && node.Status == "online" {
			return node.ID
		}
	}

	return ""
}

// syncRealtime 实时同步
func (sm *SyncManager) syncRealtime(container *ProtectedContainer) error {
	log.Printf("[ContainerHA] 实时同步容器 %s", container.ContainerID)

	// 实际实现中，这里应该：
	// 1. 使用 inotify 监控容器文件系统变化
	// 2. 实时同步变化的文件到备份节点
	// 3. 或者使用分布式文件系统（如 GlusterFS）

	// 示例：使用简单的文件同步
	sourcePath := fmt.Sprintf("/var/lib/%s/%s", container.Type, container.ContainerID)
	backupNodeID := sm.getBackupNode(container.CurrentNode)

	if backupNodeID == "" {
		return fmt.Errorf("没有可用的备份节点")
	}

	// 实际实现中，这里应该调用 rsync 或类似工具
	log.Printf("[ContainerHA] 同步路径 %s 到节点 %s", sourcePath, backupNodeID)

	return nil
}

// GetAllProtectedContainers 获取所有受保护容器
func (m *FailoverManager) GetAllProtectedContainers() []*ProtectedContainer {
	m.containerMu.RLock()
	defer m.containerMu.RUnlock()

	containers := make([]*ProtectedContainer, 0, len(m.containers))
	for _, container := range m.containers {
		containers = append(containers, container)
	}

	return containers
}

// GetProtectedContainer 获取指定受保护容器
func (m *FailoverManager) GetProtectedContainer(containerID string) (*ProtectedContainer, error) {
	m.containerMu.RLock()
	defer m.containerMu.RUnlock()

	container, exists := m.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("容器不存在: %s", containerID)
	}

	return container, nil
}

// SyncNow 立即执行同步
func (m *FailoverManager) SyncNow() error {
	go m.syncManager.runSync()
	return nil
}

// GetSyncStatus 获取同步状态
func (m *FailoverManager) GetSyncStatus() SyncStatus {
	m.syncManager.statusMu.RLock()
	defer m.syncManager.statusMu.RUnlock()

	return m.syncManager.status
}

// RestoreFromCheckpoint 从检查点恢复容器
func (m *FailoverManager) RestoreFromCheckpoint(containerID, checkpointPath, targetNode string) error {
	log.Printf("[ContainerHA] 从检查点恢复容器 %s 到节点 %s", containerID, targetNode)

	// 1. 验证检查点
	checkpoint, err := m.validateCheckpoint(checkpointPath)
	if err != nil {
		return fmt.Errorf("检查点验证失败: %v", err)
	}

	// 2. 传输检查点到目标节点
	err = m.transferCheckpoint(checkpoint, targetNode)
	if err != nil {
		return fmt.Errorf("检查点传输失败: %v", err)
	}

	// 3. 从检查点恢复容器
	err = m.restoreContainer(containerID, checkpointPath, targetNode)
	if err != nil {
		return fmt.Errorf("容器恢复失败: %v", err)
	}

	// 4. 更新容器状态
	m.containerMu.Lock()
	if container, exists := m.containers[containerID]; exists {
		container.CurrentNode = targetNode
		container.Status = "running"
		container.HealthStatus = "healthy"
	}
	m.containerMu.Unlock()

	return nil
}

// validateCheckpoint 验证检查点
func (m *FailoverManager) validateCheckpoint(checkpointPath string) (*Checkpoint, error) {
	// 实际实现中，这里应该：
	// 1. 检查检查点文件是否存在
	// 2. 验证检查点完整性（校验和）
	// 3. 检查检查点状态

	checkpoint := &Checkpoint{
		Path:   checkpointPath,
		Status: "ready",
	}

	return checkpoint, nil
}

// transferCheckpoint 传输检查点
func (m *FailoverManager) transferCheckpoint(checkpoint *Checkpoint, targetNode string) error {
	// 实际实现中，这里应该使用 SCP/rsync 传输检查点文件
	log.Printf("[ContainerHA] 传输检查点 %s 到节点 %s", checkpoint.Path, targetNode)

	return nil
}

// restoreContainer 恢复容器
func (m *FailoverManager) restoreContainer(containerID, checkpointPath, targetNode string) error {
	// 实际实现中，这里应该调用容器运行时的恢复API
	// 对于 LXC: lxc-checkpoint -r
	// 对于 Docker: docker start --checkpoint

	log.Printf("[ContainerHA] 从检查点 %s 恢复容器 %s", checkpointPath, containerID)

	return nil
}

// ListCheckpoints 列出容器的检查点
func (m *FailoverManager) ListCheckpoints(containerID string) ([]Checkpoint, error) {
	// 实际实现中，这里应该列出容器的所有检查点
	// 示例返回
	checkpoints := []Checkpoint{
		{
			ContainerID: containerID,
			Path:        fmt.Sprintf("/var/lib/containerha/checkpoints/%s/latest", containerID),
			Timestamp:   time.Now().Add(-1 * time.Hour),
			Status:      "ready",
		},
	}

	return checkpoints, nil
}

// DeleteCheckpoint 删除检查点
func (m *FailoverManager) DeleteCheckpoint(checkpointPath string) error {
	// 实际实现中，这里应该删除检查点文件
	log.Printf("[ContainerHA] 删除检查点: %s", checkpointPath)

	return nil
}
