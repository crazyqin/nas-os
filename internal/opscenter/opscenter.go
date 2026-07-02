// Package opscenter 运维中心核心实现
package opscenter

import (
	"fmt"
	"sync"
	"time"
)

// OpsCenter 运维中心.
type OpsCenter struct {
	mu     sync.RWMutex
	config Config
	nodes  map[string]*NASNode
	alerts []*Alert
	checks map[string]*HealthCheck
}

// New 创建运维中心.
func New(config Config) *OpsCenter {
	return &OpsCenter{
		config: config,
		nodes:  make(map[string]*NASNode),
		alerts: make([]*Alert, 0),
		checks: make(map[string]*HealthCheck),
	}
}

// RegisterNode 注册NAS节点.
func (oc *OpsCenter) RegisterNode(node *NASNode) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	if len(oc.nodes) >= oc.config.MaxNodes {
		return fmt.Errorf("maximum nodes (%d) reached", oc.config.MaxNodes)
	}

	node.RegisteredAt = time.Now()
	node.LastSeen = time.Now()
	oc.nodes[node.ID] = node
	return nil
}

// UpdateNodeStatus 更新节点状态.
func (oc *OpsCenter) UpdateNodeStatus(id string, cpu, mem, temp float64) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	node, ok := oc.nodes[id]
	if !ok {
		return fmt.Errorf("node %s not found", id)
	}

	node.CPUPercent = cpu
	node.MemPercent = mem
	node.Temperature = temp
	node.LastSeen = time.Now()

	// 自动告警
	if cpu > 90 {
		oc.addAlert(id, SeverityWarning, "CPU 过高", fmt.Sprintf("CPU 使用率 %.1f%%", cpu))
	}
	if mem > 90 {
		oc.addAlert(id, SeverityWarning, "内存不足", fmt.Sprintf("内存使用率 %.1f%%", mem))
	}
	if temp > 80 {
		oc.addAlert(id, SeverityCritical, "温度过高", fmt.Sprintf("温度 %.1f°C", temp))
	}

	return nil
}

// Heartbeat 节点心跳.
func (oc *OpsCenter) Heartbeat(nodeID string) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	node, ok := oc.nodes[nodeID]
	if !ok {
		return fmt.Errorf("node %s not found", nodeID)
	}

	node.LastSeen = time.Now()
	if node.Status == NodeOffline {
		node.Status = NodeOnline
		oc.addAlert(nodeID, SeverityInfo, "节点恢复", fmt.Sprintf("节点 %s 已恢复在线", nodeID))
	}

	return nil
}

// CheckOfflineNodes 检查离线节点.
func (oc *OpsCenter) CheckOfflineNodes() {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	threshold := time.Duration(oc.config.HeartbeatSec*3) * time.Second
	for _, node := range oc.nodes {
		if time.Since(node.LastSeen) > threshold && node.Status != NodeOffline {
			node.Status = NodeOffline
			oc.addAlert(node.ID, SeverityCritical, "节点离线", fmt.Sprintf("节点 %s 已离线", node.Hostname))
		}
	}
}

// GetDashboard 获取运维仪表盘数据.
func (oc *OpsCenter) GetDashboard() *Dashboard {
	oc.mu.RLock()
	defer oc.mu.RUnlock()

	dash := &Dashboard{}
	dash.TotalNodes = len(oc.nodes)

	var totalCPU, totalMem float64
	for _, node := range oc.nodes {
		switch node.Status {
		case NodeOnline:
			dash.OnlineNodes++
		case NodeOffline:
			dash.OfflineNodes++
		}
		totalCPU += node.CPUPercent
		totalMem += node.MemPercent
		dash.TotalStorageTB += node.DiskTotalTB
		dash.UsedStorageTB += node.DiskUsedTB
		dash.Nodes = append(dash.Nodes, node)
	}

	if dash.TotalNodes > 0 {
		dash.AvgCPU = totalCPU / float64(dash.TotalNodes)
		dash.AvgMemory = totalMem / float64(dash.TotalNodes)
	}

	// 告警统计
	for _, alert := range oc.alerts {
		if !alert.Resolved {
			dash.TotalAlerts++
			if alert.Severity == SeverityCritical || alert.Severity == SeverityFatal {
				dash.CriticalAlerts++
			}
		}
	}

	// 最近10条告警
	start := 0
	if len(oc.alerts) > 10 {
		start = len(oc.alerts) - 10
	}
	dash.RecentAlerts = oc.alerts[start:]

	return dash
}

// AcknowledgeAlert 确认告警.
func (oc *OpsCenter) AcknowledgeAlert(alertID, ackedBy string) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	for _, alert := range oc.alerts {
		if alert.ID == alertID {
			alert.Acked = true
			alert.AckedBy = ackedBy
			return nil
		}
	}
	return fmt.Errorf("alert %s not found", alertID)
}

// ResolveAlert 解决告警.
func (oc *OpsCenter) ResolveAlert(alertID string) error {
	oc.mu.Lock()
	defer oc.mu.Unlock()

	for _, alert := range oc.alerts {
		if alert.ID == alertID {
			now := time.Now()
			alert.Resolved = true
			alert.ResolvedAt = &now
			return nil
		}
	}
	return fmt.Errorf("alert %s not found", alertID)
}

// GetNodes 获取所有节点.
func (oc *OpsCenter) GetNodes() []*NASNode {
	oc.mu.RLock()
	defer oc.mu.RUnlock()

	nodes := make([]*NASNode, 0, len(oc.nodes))
	for _, n := range oc.nodes {
		nodes = append(nodes, n)
	}
	return nodes
}

// GetAlerts 获取告警列表.
func (oc *OpsCenter) GetAlerts(severity Severity, unresolvedOnly bool) []*Alert {
	oc.mu.RLock()
	defer oc.mu.RUnlock()

	var result []*Alert
	for _, a := range oc.alerts {
		if severity != "" && a.Severity != severity {
			continue
		}
		if unresolvedOnly && a.Resolved {
			continue
		}
		result = append(result, a)
	}
	return result
}

func (oc *OpsCenter) addAlert(nodeID string, severity Severity, title, message string) {
	alert := &Alert{
		ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		NodeID:    nodeID,
		Severity:  severity,
		Title:     title,
		Message:   message,
		Source:    "opscenter",
		CreatedAt: time.Now(),
	}
	oc.alerts = append(oc.alerts, alert)
}
