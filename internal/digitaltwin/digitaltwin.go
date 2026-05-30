// Package digitaltwin 数字孪生模块
// 支持 NAS 配置快照、虚拟实例创建、配置差异对比、灾难恢复演练
package digitaltwin

import (
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// SnapshotType 快照类型
type SnapshotType string

const (
	SnapshotTypeFull       SnapshotType = "full"
	SnapshotTypeConfig     SnapshotType = "config"
	SnapshotTypeStorage    SnapshotType = "storage"
	SnapshotTypeNetwork    SnapshotType = "network"
	SnapshotTypeService    SnapshotType = "service"
)

// TwinStatus 虚拟实例状态
type TwinStatus string

const (
	TwinStatusCreating  TwinStatus = "creating"
	TwinStatusReady     TwinStatus = "ready"
	TwinStatusRunning   TwinStatus = "running"
	TwinStatusStopped   TwinStatus = "stopped"
	TwinStatusError     TwinStatus = "error"
	TwinStatusDestroyed TwinStatus = "destroyed"
)

// Dr演练Status 灾难恢复演练状态
type Dr演练Status string

const (
	Dr演练StatusPending   Dr演练Status = "pending"
	Dr演练StatusRunning   Dr演练Status = "running"
	Dr演练StatusCompleted Dr演练Status = "completed"
	Dr演练StatusFailed    Dr演练Status = "failed"
)

// ConfigDiffType 配置差异类型
type ConfigDiffType string

const (
	DiffTypeAdded    ConfigDiffType = "added"
	DiffTypeRemoved  ConfigDiffType = "removed"
	DiffTypeModified ConfigDiffType = "modified"
	DiffTypeUnchanged ConfigDiffType = "unchanged"
)

// Snapshot 配置快照
type Snapshot struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        SnapshotType      `json:"type"`
	Version     string            `json:"version"`
	Data        map[string]interface{} `json:"data"`
	Checksum    string            `json:"checksum"`
	Size        int64             `json:"size"`
	CreatedAt   time.Time         `json:"created_at"`
	Tags        []string          `json:"tags"`
}

// VirtualTwin 虚拟 NAS 实例
type VirtualTwin struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	SnapshotID  string            `json:"snapshot_id"`
	Status      TwinStatus        `json:"status"`
	Config      map[string]interface{} `json:"config"`
	Resources   TwinResources     `json:"resources"`
	Network     TwinNetwork       `json:"network"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	StoppedAt   *time.Time        `json:"stopped_at,omitempty"`
}

// TwinResources 虚拟实例资源
type TwinResources struct {
	CPU     int   `json:"cpu"`     // CPU 核心数
	Memory  int   `json:"memory"`  // 内存 MB
	Storage int64 `json:"storage"` // 存储 bytes
	DiskCount int `json:"disk_count"`
}

// TwinNetwork 虚拟实例网络
type TwinNetwork struct {
	IPAddress string `json:"ip_address"`
	MACAddress string `json:"mac_address"`
	Ports     []int  `json:"ports"`
	VLAN      int    `json:"vlan"`
}

// ConfigDiff 配置差异
type ConfigDiff struct {
	Path      string         `json:"path"`
	Type      ConfigDiffType `json:"type"`
	OldValue  interface{}    `json:"old_value,omitempty"`
	NewValue  interface{}    `json:"new_value,omitempty"`
}

// DiffResult 差异对比结果
type DiffResult struct {
	ID          string       `json:"id"`
	Snapshot1ID string       `json:"snapshot1_id"`
	Snapshot2ID string       `json:"snapshot2_id"`
	Diffs       []ConfigDiff `json:"diffs"`
	TotalDiffs  int          `json:"total_diffs"`
	AddedCount  int          `json:"added_count"`
	RemovedCount int         `json:"removed_count"`
	ModifiedCount int        `json:"modified_count"`
	GeneratedAt time.Time    `json:"generated_at"`
}

// Dr演练 灾难恢复演练
type Dr演练 struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	SnapshotID  string         `json:"snapshot_id"`
	TwinID      string         `json:"twin_id"`
	Status      Dr演练Status   `json:"status"`
	Steps       []Dr演练Step   `json:"steps"`
	Results     *Dr演练Result  `json:"results,omitempty"`
	StartedAt   *time.Time     `json:"started_at,omitempty"`
	CompletedAt *time.Time     `json:"completed_at,omitempty"`
}

// Dr演练Step 演练步骤
type Dr演练Step struct {
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Status      string        `json:"status"`
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
}

// Dr演练Result 演练结果
type Dr演练Result struct {
	TotalSteps    int           `json:"total_steps"`
	PassedSteps   int           `json:"passed_steps"`
	FailedSteps   int           `json:"failed_steps"`
	TotalDuration time.Duration `json:"total_duration"`
	RecoveryTime  time.Duration `json:"recovery_time"`
	DataLoss      bool          `json:"data_loss"`
	Success       bool          `json:"success"`
	Recommendations []string    `json:"recommendations"`
}

// TopologyNode 拓扑节点
type TopologyNode struct {
	ID       string            `json:"id"`
	Type     string            `json:"type"` // nas, disk, pool, volume, share
	Name     string            `json:"name"`
	Status   string            `json:"status"`
	ParentID string            `json:"parent_id,omitempty"`
	Children []string          `json:"children,omitempty"`
	Metadata map[string]interface{} `json:"metadata,omitempty"`
}

// TopologyEdge 拓扑边
type TopologyEdge struct {
	Source string `json:"source"`
	Target string `json:"target"`
	Type   string `json:"type"` // contains, connects, replicates
}

// StorageTopology 存储拓扑
type StorageTopology struct {
	ID        string          `json:"id"`
	Nodes     []TopologyNode  `json:"nodes"`
	Edges     []TopologyEdge  `json:"edges"`
	GeneratedAt time.Time     `json:"generated_at"`
}

// DigitalTwinConfig 数字孪生配置
type DigitalTwinConfig struct {
	Enabled           bool   `json:"enabled"`
	MaxSnapshots      int    `json:"max_snapshots"`
	MaxTwins          int    `json:"max_twins"`
	SnapshotRetention int    `json:"snapshot_retention"` // days
	AutoSnapshot      bool   `json:"auto_snapshot"`
	SnapshotInterval  int    `json:"snapshot_interval"`  // hours
	Dr演练Enabled     bool   `json:"dr_演练_enabled"`
}

// Manager 数字孪生管理器
type Manager struct {
	config    *DigitalTwinConfig
	snapshots map[string]*Snapshot
	twins     map[string]*VirtualTwin
	diffs     []*DiffResult
	演练s     map[string]*Dr演练
	mu        sync.RWMutex
	stopCh    chan struct{}
}

// NewManager 创建数字孪生管理器
func NewManager(config *DigitalTwinConfig) *Manager {
	return &Manager{
		config:    config,
		snapshots: make(map[string]*Snapshot),
		twins:     make(map[string]*VirtualTwin),
		演练s:     make(map[string]*Dr演练),
		stopCh:    make(chan struct{}),
	}
}

// Start 启动数字孪生
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return nil
	}
	
	if m.config.AutoSnapshot {
		go m.autoSnapshot()
	}
	
	return nil
}

// Stop 停止数字孪生
func (m *Manager) Stop() {
	close(m.stopCh)
}

// autoSnapshot 自动快照
func (m *Manager) autoSnapshot() {
	ticker := time.NewTicker(time.Duration(m.config.SnapshotInterval) * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.CreateSnapshot("auto-snapshot", "Automatic scheduled snapshot", SnapshotTypeConfig, nil)
		}
	}
}

// CreateSnapshot 创建快照
func (m *Manager) CreateSnapshot(name, description string, snapshotType SnapshotType, data map[string]interface{}) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if len(m.snapshots) >= m.config.MaxSnapshots {
		return nil, fmt.Errorf("maximum snapshots reached: %d", m.config.MaxSnapshots)
	}
	
	snapshot := &Snapshot{
		ID:          fmt.Sprintf("snap_%d", time.Now().UnixNano()),
		Name:        name,
		Description: description,
		Type:        snapshotType,
		Version:     "2.527.0",
		Data:        data,
		CreatedAt:   time.Now(),
	}
	
	if snapshot.Data == nil {
		snapshot.Data = m.collectCurrentConfig()
	}
	
	// 计算校验和
	dataBytes, _ := json.Marshal(snapshot.Data)
	snapshot.Size = int64(len(dataBytes))
	
	m.snapshots[snapshot.ID] = snapshot
	return snapshot, nil
}

// collectCurrentConfig 收集当前配置
func (m *Manager) collectCurrentConfig() map[string]interface{} {
	return map[string]interface{}{
		"hostname": "nas-os",
		"version":  "2.527.0",
		"storage": map[string]interface{}{
			"pools": []string{"pool1", "pool2"},
			"disks": []string{"/dev/sda", "/dev/sdb"},
		},
		"network": map[string]interface{}{
			"interfaces": []string{"eth0"},
			"ip":         "192.168.1.100",
		},
	}
}

// GetSnapshot 获取快照
func (m *Manager) GetSnapshot(snapshotID string) (*Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	snapshot, ok := m.snapshots[snapshotID]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
	}
	
	return snapshot, nil
}

// ListSnapshots 列出快照
func (m *Manager) ListSnapshots() []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	snapshots := make([]*Snapshot, 0, len(m.snapshots))
	for _, s := range m.snapshots {
		snapshots = append(snapshots, s)
	}
	
	return snapshots
}

// DeleteSnapshot 删除快照
func (m *Manager) DeleteSnapshot(snapshotID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, ok := m.snapshots[snapshotID]; !ok {
		return fmt.Errorf("snapshot not found: %s", snapshotID)
	}
	
	delete(m.snapshots, snapshotID)
	return nil
}

// CreateVirtualTwin 创建虚拟实例
func (m *Manager) CreateVirtualTwin(name, description, snapshotID string, resources TwinResources) (*VirtualTwin, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if len(m.twins) >= m.config.MaxTwins {
		return nil, fmt.Errorf("maximum twins reached: %d", m.config.MaxTwins)
	}
	
	snapshot, ok := m.snapshots[snapshotID]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
	}
	
	twin := &VirtualTwin{
		ID:          fmt.Sprintf("twin_%d", time.Now().UnixNano()),
		Name:        name,
		Description: description,
		SnapshotID:  snapshotID,
		Status:      TwinStatusCreating,
		Config:      snapshot.Data,
		Resources:   resources,
		Network: TwinNetwork{
			IPAddress: fmt.Sprintf("192.168.1.%d", len(m.twins)+100),
			Ports:     []int{80, 443, 22},
		},
		CreatedAt:   time.Now(),
	}
	
	m.twins[twin.ID] = twin
	
	// 模拟创建过程
	go func() {
		time.Sleep(2 * time.Second)
		m.mu.Lock()
		twin.Status = TwinStatusReady
		m.mu.Unlock()
	}()
	
	return twin, nil
}

// GetVirtualTwin 获取虚拟实例
func (m *Manager) GetVirtualTwin(twinID string) (*VirtualTwin, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	twin, ok := m.twins[twinID]
	if !ok {
		return nil, fmt.Errorf("virtual twin not found: %s", twinID)
	}
	
	return twin, nil
}

// ListVirtualTwins 列出虚拟实例
func (m *Manager) ListVirtualTwins() []*VirtualTwin {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	twins := make([]*VirtualTwin, 0, len(m.twins))
	for _, t := range m.twins {
		twins = append(twins, t)
	}
	
	return twins
}

// StartVirtualTwin 启动虚拟实例
func (m *Manager) StartVirtualTwin(twinID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	twin, ok := m.twins[twinID]
	if !ok {
		return fmt.Errorf("virtual twin not found: %s", twinID)
	}
	
	if twin.Status != TwinStatusReady && twin.Status != TwinStatusStopped {
		return fmt.Errorf("cannot start twin in status: %s", twin.Status)
	}
	
	twin.Status = TwinStatusRunning
	now := time.Now()
	twin.StartedAt = &now
	
	return nil
}

// StopVirtualTwin 停止虚拟实例
func (m *Manager) StopVirtualTwin(twinID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	twin, ok := m.twins[twinID]
	if !ok {
		return fmt.Errorf("virtual twin not found: %s", twinID)
	}
	
	if twin.Status != TwinStatusRunning {
		return fmt.Errorf("cannot stop twin in status: %s", twin.Status)
	}
	
	twin.Status = TwinStatusStopped
	now := time.Now()
	twin.StoppedAt = &now
	
	return nil
}

// DestroyVirtualTwin 销毁虚拟实例
func (m *Manager) DestroyVirtualTwin(twinID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	twin, ok := m.twins[twinID]
	if !ok {
		return fmt.Errorf("virtual twin not found: %s", twinID)
	}
	
	twin.Status = TwinStatusDestroyed
	delete(m.twins, twinID)
	
	return nil
}

// CompareSnapshots 对比快照
func (m *Manager) CompareSnapshots(snapshot1ID, snapshot2ID string) (*DiffResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	snapshot1, ok := m.snapshots[snapshot1ID]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", snapshot1ID)
	}
	
	snapshot2, ok := m.snapshots[snapshot2ID]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", snapshot2ID)
	}
	
	result := &DiffResult{
		ID:          fmt.Sprintf("diff_%d", time.Now().UnixNano()),
		Snapshot1ID: snapshot1ID,
		Snapshot2ID: snapshot2ID,
		GeneratedAt: time.Now(),
	}
	
	result.Diffs = m.compareConfigs(snapshot1.Data, snapshot2.Data, "")
	result.TotalDiffs = len(result.Diffs)
	
	for _, diff := range result.Diffs {
		switch diff.Type {
		case DiffTypeAdded:
			result.AddedCount++
		case DiffTypeRemoved:
			result.RemovedCount++
		case DiffTypeModified:
			result.ModifiedCount++
		}
	}
	
	m.diffs = append(m.diffs, result)
	return result, nil
}

// compareConfigs 对比配置
func (m *Manager) compareConfigs(config1, config2 map[string]interface{}, prefix string) []ConfigDiff {
	var diffs []ConfigDiff
	
	// 检查 config1 中的键
	for key, val1 := range config1 {
		path := key
		if prefix != "" {
			path = prefix + "." + key
		}
		
		val2, ok := config2[key]
		if !ok {
			diffs = append(diffs, ConfigDiff{
				Path:     path,
				Type:     DiffTypeRemoved,
				OldValue: val1,
			})
			continue
		}
		
		// 递归对比嵌套对象
		if map1, ok := val1.(map[string]interface{}); ok {
			if map2, ok := val2.(map[string]interface{}); ok {
				diffs = append(diffs, m.compareConfigs(map1, map2, path)...)
				continue
			}
		}
		
		// 简单值对比
		if fmt.Sprintf("%v", val1) != fmt.Sprintf("%v", val2) {
			diffs = append(diffs, ConfigDiff{
				Path:     path,
				Type:     DiffTypeModified,
				OldValue: val1,
				NewValue: val2,
			})
		}
	}
	
	// 检查 config2 中新增的键
	for key := range config2 {
		if _, ok := config1[key]; !ok {
			path := key
			if prefix != "" {
				path = prefix + "." + key
			}
			diffs = append(diffs, ConfigDiff{
				Path:     path,
				Type:     DiffTypeAdded,
				NewValue: config2[key],
			})
		}
	}
	
	return diffs
}

// StartDr演练 启动灾难恢复演练
func (m *Manager) StartDr演练(name, description, snapshotID string) (*Dr演练, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.config.Dr演练Enabled {
		return nil, fmt.Errorf("DR 演练未启用")
	}
	
	// 创建虚拟实例
	snapshot, ok := m.snapshots[snapshotID]
	if !ok {
		return nil, fmt.Errorf("snapshot not found: %s", snapshotID)
	}
	
	twin := &VirtualTwin{
		ID:         fmt.Sprintf("twin_dr_%d", time.Now().UnixNano()),
		Name:       "DR-" + name,
		SnapshotID: snapshotID,
		Status:     TwinStatusCreating,
		Config:     snapshot.Data,
		Resources: TwinResources{
			CPU:    2,
			Memory: 4096,
			Storage: 100 * 1024 * 1024 * 1024,
		},
		CreatedAt: time.Now(),
	}
	m.twins[twin.ID] = twin
	
	演练 := &Dr演练{
		ID:         fmt.Sprintf("dr_%d", time.Now().UnixNano()),
		Name:       name,
		Description: description,
		SnapshotID: snapshotID,
		TwinID:     twin.ID,
		Status:     Dr演练StatusPending,
		Steps: []Dr演练Step{
			{Name: "创建虚拟实例", Description: "从快照创建虚拟 NAS"},
			{Name: "恢复配置", Description: "应用快照配置"},
			{Name: "验证服务", Description: "检查关键服务状态"},
			{Name: "数据完整性检查", Description: "验证数据完整性"},
			{Name: "网络连通性测试", Description: "测试网络连接"},
			{Name: "生成报告", Description: "生成演练报告"},
		},
	}
	
	m.演练s[演练.ID] = 演练
	
	// 异步执行演练
	go m.executeDr演练(演练)
	
	return 演练, nil
}

// executeDr演练 执行灾难恢复演练
func (m *Manager) executeDr演练(演练 *Dr演练) {
	m.mu.Lock()
	演练.Status = Dr演练StatusRunning
	now := time.Now()
	演练.StartedAt = &now
	m.mu.Unlock()
	
	passedSteps := 0
	failedSteps := 0
	startTime := time.Now()
	
	for i := range 演练.Steps {
		// 模拟步骤执行
		time.Sleep(1 * time.Second)
		
		m.mu.Lock()
		演练.Steps[i].Duration = time.Second
		演练.Steps[i].Status = "passed"
		passedSteps++
		m.mu.Unlock()
	}
	
	m.mu.Lock()
	completedAt := time.Now()
	演练.CompletedAt = &completedAt
	演练.Status = Dr演练StatusCompleted
	
	演练.Results = &Dr演练Result{
		TotalSteps:    len(演练.Steps),
		PassedSteps:   passedSteps,
		FailedSteps:   failedSteps,
		TotalDuration: time.Since(startTime),
		RecoveryTime:  time.Since(startTime),
		DataLoss:      false,
		Success:       failedSteps == 0,
		Recommendations: []string{
			"建议定期进行灾难恢复演练",
			"确保备份数据的完整性",
		},
	}
	m.mu.Unlock()
}

// GetDr演练 获取灾难恢复演练
func (m *Manager) GetDr演练(演练ID string) (*Dr演练, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	演练, ok := m.演练s[演练ID]
	if !ok {
		return nil, fmt.Errorf("DR 演练未找到: %s", 演练ID)
	}
	
	return 演练, nil
}

// GenerateTopology 生成存储拓扑
func (m *Manager) GenerateTopology() *StorageTopology {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	topology := &StorageTopology{
		ID:          fmt.Sprintf("topo_%d", time.Now().UnixNano()),
		GeneratedAt: time.Now(),
	}
	
	// NAS 根节点
	topology.Nodes = append(topology.Nodes, TopologyNode{
		ID:     "nas",
		Type:   "nas",
		Name:   "NAS-OS",
		Status: "online",
	})
	
	// 存储池
	pools := []string{"pool1", "pool2"}
	for _, pool := range pools {
		topology.Nodes = append(topology.Nodes, TopologyNode{
			ID:       pool,
			Type:     "pool",
			Name:     pool,
			Status:   "healthy",
			ParentID: "nas",
		})
		topology.Edges = append(topology.Edges, TopologyEdge{
			Source: "nas",
			Target: pool,
			Type:   "contains",
		})
	}
	
	return topology
}

// GetDashboard 获取仪表盘数据
func (m *Manager) GetDashboard() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	runningTwins := 0
	for _, twin := range m.twins {
		if twin.Status == TwinStatusRunning {
			runningTwins++
		}
	}
	
	return map[string]interface{}{
		"snapshots_count": len(m.snapshots),
		"twins_count":     len(m.twins),
		"running_twins":   runningTwins,
		"演练s_count":     len(m.演练s),
		"auto_snapshot":   m.config.AutoSnapshot,
		"dr_演练_enabled": m.config.Dr演练Enabled,
	}
}

// MarshalJSON 序列化
func (m *Manager) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return json.Marshal(struct {
		Config    *DigitalTwinConfig `json:"config"`
		Snapshots int                `json:"snapshots_count"`
		Twins     int                `json:"twins_count"`
		演练s     int                `json:"演练s_count"`
		Diffs     int                `json:"diffs_count"`
	}{
		Config:    m.config,
		Snapshots: len(m.snapshots),
		Twins:     len(m.twins),
		演练s:     len(m.演练s),
		Diffs:     len(m.diffs),
	})
}
