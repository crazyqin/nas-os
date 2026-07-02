// Package cms 提供状态聚合器
package cms

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"go.uber.org/zap"
)

// StatusAggregator 状态聚合器.
type StatusAggregator struct {
	config      FleetConfig
	nodeStatus  map[string]*NodeDetailedStatus
	fleetStatus *FleetStatus
	logger      *zap.Logger
	mu          sync.RWMutex
	ctx         context.Context
	cancel      context.CancelFunc
	dataFile    string
}

// NodeDetailedStatus 节点详细状态.
type NodeDetailedStatus struct {
	DeviceID        string                 `json:"deviceId"`
	Name            string                 `json:"name"`
	Status          string                 `json:"status"`
	HealthScore     int                    `json:"healthScore"` // 0-100
	Metrics         map[string]interface{} `json:"metrics"`
	Services        []ServiceHealth        `json:"services"`
	Alerts          []AlertInfo            `json:"alerts"`
	LastHeartbeat   time.Time              `json:"lastHeartbeat"`
	Uptime          time.Duration          `json:"uptime"`
	CPUUsage        float64                `json:"cpuUsage"`
	MemoryUsage     float64                `json:"memoryUsage"`
	DiskUsage       float64                `json:"diskUsage"`
	NetworkInBytes  uint64                 `json:"networkInBytes"`
	NetworkOutBytes uint64                 `json:"networkOutBytes"`
	Temperature     float64                `json:"temperature"` // 设备温度
}

// ServiceHealth 服务健康状态.
type ServiceHealth struct {
	Name   string        `json:"name"`
	Status string        `json:"status"` // running, stopped, error
	Port   int           `json:"port"`
	Uptime time.Duration `json:"uptime"`
}

// AlertInfo 告警信息.
type AlertInfo struct {
	AlertID   string    `json:"alertId"`
	Type      string    `json:"type"`  // cpu, memory, disk, network, temperature
	Level     string    `json:"level"` // info, warning, critical
	Message   string    `json:"message"`
	Timestamp time.Time `json:"timestamp"`
	Resolved  bool      `json:"resolved"`
}

// FleetStatus 舰队整体状态.
type FleetStatus struct {
	FleetID          string            `json:"fleetId"`
	ClusterName      string            `json:"clusterName"`
	TotalNodes       int               `json:"totalNodes"`
	ActiveNodes      int               `json:"activeNodes"`
	OfflineNodes     int               `json:"offlineNodes"`
	WarningNodes     int               `json:"warningNodes"`
	HealthyNodes     int               `json:"healthyNodes"`
	TotalCPUUsage    float64           `json:"totalCpuUsage"`
	TotalMemoryUsage float64           `json:"totalMemoryUsage"`
	TotalDiskUsage   float64           `json:"totalDiskUsage"`
	OverallHealth    int               `json:"overallHealth"` // 0-100
	ActiveAlerts     int               `json:"activeAlerts"`
	LastUpdated      time.Time         `json:"lastUpdated"`
	NodeSummary      map[string]string `json:"nodeSummary"` // nodeID -> status
}

// NewStatusAggregator 创建状态聚合器.
func NewStatusAggregator(config FleetConfig, logger *zap.Logger) (*StatusAggregator, error) {
	ctx, cancel := context.WithCancel(context.Background())

	sa := &StatusAggregator{
		config:      config,
		nodeStatus:  make(map[string]*NodeDetailedStatus),
		fleetStatus: &FleetStatus{},
		logger:      logger,
		ctx:         ctx,
		cancel:      cancel,
		dataFile:    filepath.Join(config.DataDir, "status.json"),
	}

	// 加载持久化数据
	if err := sa.loadState(); err != nil {
		logger.Warn("加载状态数据失败", zap.Error(err))
	}

	return sa, nil
}

// Start 启动状态聚合器.
func (sa *StatusAggregator) Start() {
	sa.logger.Info("状态聚合器启动")
}

// Stop 停止状态聚合器.
func (sa *StatusAggregator) Stop() {
	sa.cancel()
	sa.saveState()
	sa.logger.Info("状态聚合器停止")
}

// GetNodeStatus 获取节点详细状态.
func (sa *StatusAggregator) GetNodeStatus(deviceID string) (*NodeDetailedStatus, error) {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	status, ok := sa.nodeStatus[deviceID]
	if !ok {
		return nil, fmt.Errorf("节点状态不存在: %s", deviceID)
	}

	return status, nil
}

// GetFleetStatus 获取舰队整体状态.
func (sa *StatusAggregator) GetFleetStatus() *FleetStatus {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	return sa.fleetStatus
}

// UpdateNodeMetrics 更新节点指标.
func (sa *StatusAggregator) UpdateNodeMetrics(deviceID string, metrics map[string]interface{}) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	status, ok := sa.nodeStatus[deviceID]
	if !ok {
		status = &NodeDetailedStatus{
			DeviceID:      deviceID,
			Name:          deviceID,
			Status:        "unknown",
			HealthScore:   100,
			Metrics:       make(map[string]interface{}),
			Services:      []ServiceHealth{},
			Alerts:        []AlertInfo{},
			LastHeartbeat: time.Now(),
		}
		sa.nodeStatus[deviceID] = status
	}

	// 更新指标
	for k, v := range metrics {
		status.Metrics[k] = v
	}

	// 更新具体字段
	if cpu, ok := metrics["cpuUsage"].(float64); ok {
		status.CPUUsage = cpu
	}
	if mem, ok := metrics["memoryUsage"].(float64); ok {
		status.MemoryUsage = mem
	}
	if disk, ok := metrics["diskUsage"].(float64); ok {
		status.DiskUsage = disk
	}
	if temp, ok := metrics["temperature"].(float64); ok {
		status.Temperature = temp
	}

	status.LastHeartbeat = time.Now()
	status.Status = "active"

	// 计算健康分数
	status.HealthScore = sa.calculateHealthScore(status)

	// 更新舰队状态
	sa.updateFleetStatus()
}

// SetNodeStatus 设置节点状态.
func (sa *StatusAggregator) SetNodeStatus(deviceID string, status *NodeDetailedStatus) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	sa.nodeStatus[deviceID] = status
	sa.updateFleetStatus()
}

// RemoveNode 移除节点状态.
func (sa *StatusAggregator) RemoveNode(deviceID string) {
	sa.mu.Lock()
	defer sa.mu.Unlock()

	delete(sa.nodeStatus, deviceID)
	sa.updateFleetStatus()
}

// calculateHealthScore 计算健康分数.
func (sa *StatusAggregator) calculateHealthScore(status *NodeDetailedStatus) int {
	score := 100

	// CPU 使用率扣分
	if status.CPUUsage > 80 {
		score -= 20
	} else if status.CPUUsage > 60 {
		score -= 10
	}

	// 内存使用率扣分
	if status.MemoryUsage > 90 {
		score -= 15
	} else if status.MemoryUsage > 75 {
		score -= 8
	}

	// 磁盘使用率扣分
	if status.DiskUsage > 90 {
		score -= 20
	} else if status.DiskUsage > 80 {
		score -= 10
	}

	// 温度扣分
	if status.Temperature > 70 {
		score -= 15
	} else if status.Temperature > 60 {
		score -= 5
	}

	// 告警扣分
	for _, alert := range status.Alerts {
		if !alert.Resolved {
			switch alert.Level {
			case "critical":
				score -= 20
			case "warning":
				score -= 10
			}
		}
	}

	// 心跳超时扣分
	heartbeatAge := time.Since(status.LastHeartbeat)
	if heartbeatAge > sa.config.HeartbeatTimeout {
		score -= 30
		status.Status = "offline"
	}

	if score < 0 {
		score = 0
	}

	return score
}

// updateFleetStatus 更新舰队状态.
func (sa *StatusAggregator) updateFleetStatus() {
	sa.fleetStatus.TotalNodes = len(sa.nodeStatus)
	sa.fleetStatus.ActiveNodes = 0
	sa.fleetStatus.OfflineNodes = 0
	sa.fleetStatus.WarningNodes = 0
	sa.fleetStatus.HealthyNodes = 0
	sa.fleetStatus.TotalCPUUsage = 0
	sa.fleetStatus.TotalMemoryUsage = 0
	sa.fleetStatus.TotalDiskUsage = 0
	sa.fleetStatus.ActiveAlerts = 0
	sa.fleetStatus.NodeSummary = make(map[string]string)

	for _, status := range sa.nodeStatus {
		sa.fleetStatus.NodeSummary[status.DeviceID] = status.Status

		switch status.Status {
		case "active":
			if status.HealthScore >= 80 {
				sa.fleetStatus.HealthyNodes++
			} else {
				sa.fleetStatus.WarningNodes++
			}
			sa.fleetStatus.ActiveNodes++
			sa.fleetStatus.TotalCPUUsage += status.CPUUsage
			sa.fleetStatus.TotalMemoryUsage += status.MemoryUsage
			sa.fleetStatus.TotalDiskUsage += status.DiskUsage
		case "offline":
			sa.fleetStatus.OfflineNodes++
		}

		for _, alert := range status.Alerts {
			if !alert.Resolved {
				sa.fleetStatus.ActiveAlerts++
			}
		}
	}

	// 计算平均值
	if sa.fleetStatus.ActiveNodes > 0 {
		sa.fleetStatus.TotalCPUUsage /= float64(sa.fleetStatus.ActiveNodes)
		sa.fleetStatus.TotalMemoryUsage /= float64(sa.fleetStatus.ActiveNodes)
		sa.fleetStatus.TotalDiskUsage /= float64(sa.fleetStatus.ActiveNodes)
	}

	// 计算整体健康分数
	sa.fleetStatus.OverallHealth = sa.calculateFleetHealth()
	sa.fleetStatus.LastUpdated = time.Now()
}

// calculateFleetHealth 计算舰队整体健康分数.
func (sa *StatusAggregator) calculateFleetHealth() int {
	if len(sa.nodeStatus) == 0 {
		return 100
	}

	totalScore := 0
	for _, status := range sa.nodeStatus {
		totalScore += status.HealthScore
	}

	return totalScore / len(sa.nodeStatus)
}

// loadState 加载持久化状态.
func (sa *StatusAggregator) loadState() error {
	data, err := os.ReadFile(sa.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &sa.nodeStatus)
}

// saveState 保存持久化状态.
func (sa *StatusAggregator) saveState() error {
	sa.mu.RLock()
	defer sa.mu.RUnlock()

	data, err := json.Marshal(sa.nodeStatus)
	if err != nil {
		return err
	}

	return os.WriteFile(sa.dataFile, data, 0640)
}
