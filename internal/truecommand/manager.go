package truecommand

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager TrueCommand 多 NAS 管理器.
type Manager struct {
	mu         sync.RWMutex
	config     TrueCommandConfig
	systems    map[string]*NASSystem
	clusters   map[string]*Cluster
	alerts     []*Alert
	dashboards map[string]*Dashboard
	running    bool
	stopCh     chan struct{}
}

// NewManager 创建管理器.
func NewManager(cfg TrueCommandConfig) *Manager {
	if cfg.PollInterval == 0 {
		cfg.PollInterval = 60 * time.Second
	}
	if cfg.AlertLimit == 0 {
		cfg.AlertLimit = 5000
	}
	if cfg.MaxSystems == 0 {
		cfg.MaxSystems = 100
	}
	return &Manager{
		config:     cfg,
		systems:    make(map[string]*NASSystem),
		clusters:   make(map[string]*Cluster),
		alerts:     make([]*Alert, 0),
		dashboards: make(map[string]*Dashboard),
		stopCh:     make(chan struct{}),
	}
}

// Start 启动管理器.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.running {
		return nil
	}
	m.running = true
	m.stopCh = make(chan struct{})
	go m.monitorLoop()
	log.Println("[TrueCommand] 多 NAS 管理器已启动")
	return nil
}

// Stop 停止.
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.running {
		return
	}
	m.running = false
	close(m.stopCh)
	log.Println("[TrueCommand] 多 NAS 管理器已停止")
}

// ========== 系统管理 ==========

// RegisterSystem 注册 NAS 系统.
func (m *Manager) RegisterSystem(system *NASSystem) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if system.ID == "" {
		return fmt.Errorf("系统 ID 不能为空")
	}
	if len(m.systems) >= m.config.MaxSystems {
		return fmt.Errorf("已达到最大系统数量限制 (%d)", m.config.MaxSystems)
	}
	if _, exists := m.systems[system.ID]; exists {
		return fmt.Errorf("系统 %s 已存在", system.ID)
	}
	system.Status = SystemStatusOnline
	system.LastSeen = time.Now()
	system.RegisteredAt = time.Now()
	m.systems[system.ID] = system
	log.Printf("[TrueCommand] NAS 系统已注册: %s (%s)", system.Name, system.Host)
	return nil
}

// UnregisterSystem 注销系统.
func (m *Manager) UnregisterSystem(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.systems[id]; !exists {
		return fmt.Errorf("系统 %s 不存在", id)
	}
	delete(m.systems, id)
	log.Printf("[TrueCommand] NAS 系统已注销: %s", id)
	return nil
}

// GetSystem 获取系统.
func (m *Manager) GetSystem(id string) (*NASSystem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	system, exists := m.systems[id]
	if !exists {
		return nil, fmt.Errorf("系统 %s 不存在", id)
	}
	return system, nil
}

// ListSystems 列出所有系统.
func (m *Manager) ListSystems() []*NASSystem {
	m.mu.RLock()
	defer m.mu.RUnlock()
	systems := make([]*NASSystem, 0, len(m.systems))
	for _, s := range m.systems {
		systems = append(systems, s)
	}
	return systems
}

// ========== 集群管理 ==========

// CreateCluster 创建集群.
func (m *Manager) CreateCluster(cluster *Cluster) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cluster.ID == "" {
		return fmt.Errorf("集群 ID 不能为空")
	}
	if _, exists := m.clusters[cluster.ID]; exists {
		return fmt.Errorf("集群 %s 已存在", cluster.ID)
	}
	cluster.Status = ClusterStatusActive
	cluster.CreatedAt = time.Now()
	m.clusters[cluster.ID] = cluster
	log.Printf("[TrueCommand] 集群已创建: %s (%s)", cluster.Name, cluster.Type)
	return nil
}

// DeleteCluster 删除集群.
func (m *Manager) DeleteCluster(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.clusters[id]; !exists {
		return fmt.Errorf("集群 %s 不存在", id)
	}
	delete(m.clusters, id)
	log.Printf("[TrueCommand] 集群已删除: %s", id)
	return nil
}

// GetCluster 获取集群.
func (m *Manager) GetCluster(id string) (*Cluster, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cluster, exists := m.clusters[id]
	if !exists {
		return nil, fmt.Errorf("集群 %s 不存在", id)
	}
	return cluster, nil
}

// ListClusters 列出所有集群.
func (m *Manager) ListClusters() []*Cluster {
	m.mu.RLock()
	defer m.mu.RUnlock()
	clusters := make([]*Cluster, 0, len(m.clusters))
	for _, c := range m.clusters {
		clusters = append(clusters, c)
	}
	return clusters
}

// AddSystemToCluster 将系统添加到集群.
func (m *Manager) AddSystemToCluster(systemID, clusterID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	system, exists := m.systems[systemID]
	if !exists {
		return fmt.Errorf("系统 %s 不存在", systemID)
	}
	cluster, exists := m.clusters[clusterID]
	if !exists {
		return fmt.Errorf("集群 %s 不存在", clusterID)
	}
	system.ClusterID = clusterID
	cluster.Members = append(cluster.Members, systemID)
	log.Printf("[TrueCommand] 系统 %s 已添加到集群 %s", systemID, clusterID)
	return nil
}

// ========== 告警管理 ==========

// GetAlerts 获取告警.
func (m *Manager) GetAlerts(systemID string, limit int) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	alerts := make([]*Alert, 0)
	for i := len(m.alerts) - 1; i >= 0; i-- {
		if systemID == "" || m.alerts[i].SystemID == systemID {
			alerts = append(alerts, m.alerts[i])
			if limit > 0 && len(alerts) >= limit {
				break
			}
		}
	}
	return alerts
}

// AcknowledgeAlert 确认告警.
func (m *Manager) AcknowledgeAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, alert := range m.alerts {
		if alert.ID == alertID {
			alert.Acknowledged = true
			alert.AckedAt = time.Now()
			log.Printf("[TrueCommand] 告警已确认: %s", alertID)
			return nil
		}
	}
	return fmt.Errorf("告警 %s 不存在", alertID)
}

// ========== 仪表板 ==========

// CreateDashboard 创建仪表板.
func (m *Manager) CreateDashboard(dashboard *Dashboard) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if dashboard.ID == "" {
		return fmt.Errorf("仪表板 ID 不能为空")
	}
	dashboard.CreatedAt = time.Now()
	m.dashboards[dashboard.ID] = dashboard
	log.Printf("[TrueCommand] 仪表板已创建: %s", dashboard.Name)
	return nil
}

// GetDashboard 获取仪表板.
func (m *Manager) GetDashboard(id string) (*Dashboard, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	dashboard, exists := m.dashboards[id]
	if !exists {
		return nil, fmt.Errorf("仪表板 %s 不存在", id)
	}
	return dashboard, nil
}

// ListDashboards 列出所有仪表板.
func (m *Manager) ListDashboards() []*Dashboard {
	m.mu.RLock()
	defer m.mu.RUnlock()
	dashboards := make([]*Dashboard, 0, len(m.dashboards))
	for _, d := range m.dashboards {
		dashboards = append(dashboards, d)
	}
	return dashboards
}

// ========== 统计 ==========

// GetStats 获取统计信息.
func (m *Manager) GetStats() *TrueCommandStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	online := 0
	totalCPU := 0.0
	totalMemory := int64(0)
	totalStorage := int64(0)
	for _, s := range m.systems {
		if s.Status == SystemStatusOnline {
			online++
		}
		totalCPU += s.CPUUsage
		totalMemory += s.MemoryTotal
		totalStorage += s.StorageTotal
	}
	unacked := 0
	for _, a := range m.alerts {
		if !a.Acknowledged {
			unacked++
		}
	}
	return &TrueCommandStats{
		TotalSystems:    len(m.systems),
		OnlineSystems:   online,
		TotalClusters:   len(m.clusters),
		TotalAlerts:     len(m.alerts),
		UnackedAlerts:   unacked,
		TotalDashboards: len(m.dashboards),
		AvgCPUUsage:     totalCPU / float64(len(m.systems)),
		TotalMemory:     totalMemory,
		TotalStorage:    totalStorage,
	}
}

// ========== 内部方法 ==========

// monitorLoop 监控循环.
func (m *Manager) monitorLoop() {
	ticker := time.NewTicker(m.config.PollInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.pollSystems()
		}
	}
}

// pollSystems 轮询所有系统.
func (m *Manager) pollSystems() {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, system := range m.systems {
		system.LastSeen = time.Now()
	}
}

// addAlert 添加告警.
func (m *Manager) addAlert(systemID, alertType, message, severity string) {
	alert := &Alert{
		ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
		SystemID:  systemID,
		Type:      alertType,
		Message:   message,
		Severity:  severity,
		Timestamp: time.Now(),
	}
	m.alerts = append(m.alerts, alert)
	if len(m.alerts) > m.config.AlertLimit {
		m.alerts = m.alerts[1:]
	}
}
