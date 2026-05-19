// Package containerorch - 健康检查器
package containerorch

import (
	"context"
	"sync"
	"time"
)

// HealthChecker 容器健康检查器.
type HealthChecker struct {
	mu      sync.RWMutex
	manager *Manager
	checks  map[string]*HealthCheckResult
	stopCh  chan struct{}
}

// HealthCheckResult 健康检查结果.
type HealthCheckResult struct {
	ContainerID string    `json:"containerId"`
	Status      string    `json:"status"` // healthy, unhealthy, starting
	Message     string    `json:"message,omitempty"`
	LastCheck   time.Time `json:"lastCheck"`
	Consecutive int       `json:"consecutive"` // 连续成功/失败次数
}

// NewHealthChecker 创建健康检查器.
func NewHealthChecker(manager *Manager) *HealthChecker {
	return &HealthChecker{
		manager: manager,
		checks:  make(map[string]*HealthCheckResult),
		stopCh:  make(chan struct{}),
	}
}

// Start 启动健康检查器.
func (hc *HealthChecker) Start(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			hc.checkAll()
		case <-ctx.Done():
			return
		case <-hc.stopCh:
			return
		}
	}
}

// checkAll 检查所有容器健康状态.
func (hc *HealthChecker) checkAll() {
	hc.manager.mu.RLock()
	containers := make(map[string]*Container)
	for k, v := range hc.manager.containers {
		containers[k] = v
	}
	hc.manager.mu.RUnlock()

	for id, container := range containers {
		if container.Status == "running" {
			hc.checkContainer(id)
		}
	}
}

// checkContainer 检查单个容器.
func (hc *HealthChecker) checkContainer(containerID string) {
	hc.mu.Lock()
	defer hc.mu.Unlock()

	result, exists := hc.checks[containerID]
	if !exists {
		result = &HealthCheckResult{
			ContainerID: containerID,
		}
		hc.checks[containerID] = result
	}

	// 默认健康（实际应执行容器健康检查命令）
	result.Status = "healthy"
	result.LastCheck = time.Now()
	result.Consecutive++
}

// GetStatus 获取健康检查器状态.
func (hc *HealthChecker) GetStatus() map[string]*HealthCheckResult {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	status := make(map[string]*HealthCheckResult)
	for k, v := range hc.checks {
		status[k] = v
	}
	return status
}

// Stop 停止健康检查器.
func (hc *HealthChecker) Stop() {
	close(hc.stopCh)
}

// IsHealthy 检查容器是否健康.
func (hc *HealthChecker) IsHealthy(containerID string) bool {
	hc.mu.RLock()
	defer hc.mu.RUnlock()
	result, exists := hc.checks[containerID]
	if !exists {
		return false
	}
	return result.Status == "healthy"
}

// RunHealthCheck 执行一次健康检查（用于 API 调用）.
func (hc *HealthChecker) RunHealthCheck(ctx context.Context) error {
	hc.checkAll()
	return nil
}
