package containermigrator

import (
	"fmt"
	"sync"
	"time"
)

// Container 容器信息
type Container struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Image       string            `json:"image"`
	State       string            `json:"state"`
	Status      string            `json:"status"`
	HostID      string            `json:"host_id"`
	HostName    string            `json:"host_name"`
	Ports       []PortMapping     `json:"ports"`
	Volumes     []VolumeMount     `json:"volumes"`
	Env         map[string]string `json:"env"`
	Labels      map[string]string `json:"labels"`
	CreatedAt   time.Time         `json:"created_at"`
	StartedAt   *time.Time        `json:"started_at,omitempty"`
	CPUPercent  float64           `json:"cpu_percent"`
	MemoryMB    int64             `json:"memory_mb"`
	MemoryLimit int64             `json:"memory_limit_mb"`
	NetworkRx   int64             `json:"network_rx_bytes"`
	NetworkTx   int64             `json:"network_tx_bytes"`
}

// PortMapping 端口映射
type PortMapping struct {
	HostPort      int    `json:"host_port"`
	ContainerPort int    `json:"container_port"`
	Protocol      string `json:"protocol"`
}

// VolumeMount 挂载卷
type VolumeMount struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	ReadOnly    bool   `json:"read_only"`
}

// Host 主机信息
type Host struct {
	ID            string            `json:"id"`
	Name          string            `json:"name"`
	Address       string            `json:"address"`
	OS            string            `json:"os"`
	CPUCores      int               `json:"cpu_cores"`
	TotalMemory   int64             `json:"total_memory_mb"`
	UsedMemory    int64             `json:"used_memory_mb"`
	Containers    int               `json:"containers"`
	MaxContainers int               `json:"max_containers"`
	Status        string            `json:"status"`
	Labels        map[string]string `json:"labels"`
	LastPing      time.Time         `json:"last_ping"`
}

// Snapshot 容器快照
type Snapshot struct {
	ID          string     `json:"id"`
	ContainerID string     `json:"container_id"`
	HostID      string     `json:"host_id"`
	Size        int64      `json:"size_bytes"`
	State       string     `json:"state"`
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
}

// MigrationTask 迁移任务
type MigrationTask struct {
	ID            string        `json:"id"`
	ContainerID   string        `json:"container_id"`
	ContainerName string        `json:"container_name"`
	SourceHostID  string        `json:"source_host_id"`
	TargetHostID  string        `json:"target_host_id"`
	Status        string        `json:"status"`
	Phase         string        `json:"phase"`
	Progress      float64       `json:"progress"`
	BytesTotal    int64         `json:"bytes_total"`
	BytesSynced   int64         `json:"bytes_synced"`
	SnapshotID    string        `json:"snapshot_id,omitempty"`
	StartedAt     time.Time     `json:"started_at"`
	CompletedAt   *time.Time    `json:"completed_at,omitempty"`
	Elapsed       time.Duration `json:"elapsed"`
	Error         string        `json:"error,omitempty"`
	RollbackPoint string        `json:"rollback_point,omitempty"`
}

// MigrationConfig 迁移配置
type MigrationConfig struct {
	SyncIntervalSec   int  `json:"sync_interval_sec"`
	MaxDowntimeMs     int  `json:"max_downtime_ms"`
	EnableCompression bool `json:"enable_compression"`
	RetriesOnFailure  int  `json:"retries_on_failure"`
	AutoRollback      bool `json:"auto_rollback"`
	SnapshotRetention int  `json:"snapshot_retention_hours"`
}

// MigrationStats 迁移统计
type MigrationStats struct {
	TotalMigrations  int     `json:"total_migrations"`
	Successful       int     `json:"successful"`
	Failed           int     `json:"failed"`
	InProgress       int     `json:"in_progress"`
	AvgDurationSec   float64 `json:"avg_duration_sec"`
	TotalBytesSynced int64   `json:"total_bytes_synced"`
	TotalContainers  int     `json:"total_containers"`
	TotalHosts       int     `json:"total_hosts"`
}

// ContainerMigrator 容器热迁移管理器
type ContainerMigrator struct {
	mu         sync.RWMutex
	containers map[string]*Container
	hosts      map[string]*Host
	snapshots  map[string]*Snapshot
	tasks      map[string]*MigrationTask
	config     *MigrationConfig
	dataPath   string
}

// NewManager 创建容器迁移管理器
func NewManager(dataPath string) *ContainerMigrator {
	m := &ContainerMigrator{
		containers: make(map[string]*Container),
		hosts:      make(map[string]*Host),
		snapshots:  make(map[string]*Snapshot),
		tasks:      make(map[string]*MigrationTask),
		config: &MigrationConfig{
			SyncIntervalSec:   5,
			MaxDowntimeMs:     5000,
			EnableCompression: true,
			RetriesOnFailure:  3,
			AutoRollback:      true,
			SnapshotRetention: 24,
		},
		dataPath: dataPath,
	}
	return m
}

// RegisterHost 注册主机
func (m *ContainerMigrator) RegisterHost(host *Host) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if host.ID == "" {
		host.ID = fmt.Sprintf("host_%d", time.Now().UnixNano())
	}
	if host.Address == "" {
		return fmt.Errorf("主机地址不能为空")
	}
	host.LastPing = time.Now()
	host.Status = "online"
	m.hosts[host.ID] = host
	return nil
}

// UnregisterHost 注销主机
func (m *ContainerMigrator) UnregisterHost(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.hosts[id]; !exists {
		return fmt.Errorf("主机不存在: %s", id)
	}
	delete(m.hosts, id)
	return nil
}

// GetHost 获取主机
func (m *ContainerMigrator) GetHost(id string) (*Host, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	host, exists := m.hosts[id]
	if !exists {
		return nil, fmt.Errorf("主机不存在: %s", id)
	}
	return host, nil
}

// ListHosts 列出主机
func (m *ContainerMigrator) ListHosts() []*Host {
	m.mu.RLock()
	defer m.mu.RUnlock()
	hosts := make([]*Host, 0, len(m.hosts))
	for _, h := range m.hosts {
		hosts = append(hosts, h)
	}
	return hosts
}

// RegisterContainer 注册容器
func (m *ContainerMigrator) RegisterContainer(c *Container) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if c.ID == "" {
		return fmt.Errorf("容器ID不能为空")
	}
	if _, exists := m.hosts[c.HostID]; !exists {
		return fmt.Errorf("主机不存在: %s", c.HostID)
	}
	m.containers[c.ID] = c
	return nil
}

// UnregisterContainer 注销容器
func (m *ContainerMigrator) UnregisterContainer(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.containers[id]; !exists {
		return fmt.Errorf("容器不存在: %s", id)
	}
	delete(m.containers, id)
	return nil
}

// GetContainer 获取容器
func (m *ContainerMigrator) GetContainer(id string) (*Container, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, exists := m.containers[id]
	if !exists {
		return nil, fmt.Errorf("容器不存在: %s", id)
	}
	return c, nil
}

// ListContainers 列出容器
func (m *ContainerMigrator) ListContainers() []*Container {
	m.mu.RLock()
	defer m.mu.RUnlock()
	containers := make([]*Container, 0, len(m.containers))
	for _, c := range m.containers {
		containers = append(containers, c)
	}
	return containers
}

// CreateSnapshot 创建容器快照
func (m *ContainerMigrator) CreateSnapshot(containerID string) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, exists := m.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("容器不存在: %s", containerID)
	}

	snap := &Snapshot{
		ID:          fmt.Sprintf("snap_%d", time.Now().UnixNano()),
		ContainerID: containerID,
		HostID:      c.HostID,
		State:       "created",
		CreatedAt:   time.Now(),
	}
	if m.config.SnapshotRetention > 0 {
		exp := time.Now().Add(time.Duration(m.config.SnapshotRetention) * time.Hour)
		snap.ExpiresAt = &exp
	}
	m.snapshots[snap.ID] = snap
	return snap, nil
}

// ListSnapshots 列出快照
func (m *ContainerMigrator) ListSnapshots(containerID string) []*Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()
	snaps := make([]*Snapshot, 0)
	for _, s := range m.snapshots {
		if containerID == "" || s.ContainerID == containerID {
			snaps = append(snaps, s)
		}
	}
	return snaps
}

// DeleteSnapshot 删除快照
func (m *ContainerMigrator) DeleteSnapshot(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.snapshots[id]; !exists {
		return fmt.Errorf("快照不存在: %s", id)
	}
	delete(m.snapshots, id)
	return nil
}

// StartMigration 启动迁移
func (m *ContainerMigrator) StartMigration(containerID, targetHostID string) (*MigrationTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	c, exists := m.containers[containerID]
	if !exists {
		return nil, fmt.Errorf("容器不存在: %s", containerID)
	}
	if _, exists := m.hosts[targetHostID]; !exists {
		return nil, fmt.Errorf("目标主机不存在: %s", targetHostID)
	}
	if c.HostID == targetHostID {
		return nil, fmt.Errorf("源和目标主机相同")
	}

	// 检查是否有进行中的迁移
	for _, t := range m.tasks {
		if t.ContainerID == containerID && (t.Status == "running" || t.Status == "pending") {
			return nil, fmt.Errorf("容器 %s 已有进行中的迁移任务", containerID)
		}
	}

	// 创建快照
	snap := &Snapshot{
		ID:          fmt.Sprintf("snap_%d", time.Now().UnixNano()),
		ContainerID: containerID,
		HostID:      c.HostID,
		State:       "created",
		CreatedAt:   time.Now(),
	}
	m.snapshots[snap.ID] = snap

	task := &MigrationTask{
		ID:            fmt.Sprintf("mig_%d", time.Now().UnixNano()),
		ContainerID:   containerID,
		ContainerName: c.Name,
		SourceHostID:  c.HostID,
		TargetHostID:  targetHostID,
		Status:        "pending",
		Phase:         "snapshot",
		SnapshotID:    snap.ID,
		StartedAt:     time.Now(),
		RollbackPoint: snap.ID,
	}
	m.tasks[task.ID] = task
	return task, nil
}

// GetMigration 获取迁移任务
func (m *ContainerMigrator) GetMigration(id string) (*MigrationTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, exists := m.tasks[id]
	if !exists {
		return nil, fmt.Errorf("迁移任务不存在: %s", id)
	}
	return task, nil
}

// ListMigrations 列出迁移任务
func (m *ContainerMigrator) ListMigrations() []*MigrationTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tasks := make([]*MigrationTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// UpdateMigrationProgress 更新迁移进度
func (m *ContainerMigrator) UpdateMigrationProgress(id string, progress float64, phase string, bytesSynced int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("迁移任务不存在: %s", id)
	}
	task.Progress = progress
	task.Phase = phase
	task.BytesSynced = bytesSynced
	task.Elapsed = time.Since(task.StartedAt)
	if progress >= 100 {
		task.Status = "completed"
		now := time.Now()
		task.CompletedAt = &now
		// 更新容器主机
		if c, ok := m.containers[task.ContainerID]; ok {
			c.HostID = task.TargetHostID
			if h, ok := m.hosts[task.TargetHostID]; ok {
				c.HostName = h.Name
			}
		}
	}
	return nil
}

// RollbackMigration 回滚迁移
func (m *ContainerMigrator) RollbackMigration(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, exists := m.tasks[id]
	if !exists {
		return fmt.Errorf("迁移任务不存在: %s", id)
	}
	if task.Status == "completed" {
		return fmt.Errorf("已完成的迁移无法回滚")
	}

	task.Status = "rolled_back"
	task.Error = "手动回滚"

	// 恢复容器到源主机
	if c, ok := m.containers[task.ContainerID]; ok {
		c.HostID = task.SourceHostID
		if h, ok := m.hosts[task.SourceHostID]; ok {
			c.HostName = h.Name
		}
	}
	return nil
}

// GetStats 获取统计
func (m *ContainerMigrator) GetStats() *MigrationStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &MigrationStats{
		TotalContainers: len(m.containers),
		TotalHosts:      len(m.hosts),
	}

	totalDuration := 0.0
	durationCount := 0
	for _, t := range m.tasks {
		stats.TotalMigrations++
		stats.TotalBytesSynced += t.BytesSynced
		switch t.Status {
		case "completed":
			stats.Successful++
			if !t.CompletedAt.IsZero() {
				totalDuration += t.CompletedAt.Sub(t.StartedAt).Seconds()
				durationCount++
			}
		case "failed":
			stats.Failed++
		case "pending", "running":
			stats.InProgress++
		}
	}
	if durationCount > 0 {
		stats.AvgDurationSec = totalDuration / float64(durationCount)
	}
	return stats
}

// UpdateConfig 更新配置
func (m *ContainerMigrator) UpdateConfig(cfg *MigrationConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
}

// GetConfig 获取配置
func (m *ContainerMigrator) GetConfig() *MigrationConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}
