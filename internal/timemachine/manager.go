// Package timemachine 提供 macOS Time Machine 备份服务器功能
// 共享管理、配额控制、设备注册、备份监控、历史管理、清理策略
package timemachine

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager Time Machine 管理器.
type Manager struct {
	mu            sync.RWMutex
	shares        map[string]*TimeMachineShare
	devices       map[string]*BackupDevice
	jobs          map[string]*BackupJob
	snapshots     map[string]*BackupSnapshot
	quotas        map[string]*BackupQuota
	retention     RetentionPolicy
	traffic       TrafficLimit
	broadcast     BroadcastConfig
	stats         TimeMachineStats
	logger        *zap.Logger
	stopCh        chan struct{}
	running       bool
	startTime     time.Time
	cleanupTicker *time.Ticker
}

// NewManager 创建 Time Machine 管理器.
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		shares:    make(map[string]*TimeMachineShare),
		devices:   make(map[string]*BackupDevice),
		jobs:      make(map[string]*BackupJob),
		snapshots: make(map[string]*BackupSnapshot),
		quotas:    make(map[string]*BackupQuota),
		retention: RetentionPolicy{
			RetentionDays: 30,
			MinKeep:       3,
			MaxBackups:    100,
			AutoCleanup:   true,
		},
		traffic: TrafficLimit{
			BandwidthKBps: 0,
			TimeWindow:    "",
			Enabled:       false,
		},
		broadcast: BroadcastConfig{
			ServiceName: "TimeMachine NAS",
			Port:        548,
			Enabled:     true,
			TXTRecords: map[string]string{
				"dk0": "adVN=TimeMachine,adVF=0x82",
			},
		},
		logger:  logger,
		stopCh:  make(chan struct{}),
		running: false,
	}

	// 初始化默认共享
	m.initDefaults()

	return m
}

// initDefaults 初始化默认配置.
func (m *Manager) initDefaults() {
	defaultShare := &TimeMachineShare{
		ID:        "default",
		Name:      "TimeMachine",
		Path:      "/srv/timemachine",
		Protocol:  ProtocolSMB,
		Enabled:   true,
		DeviceNum: 0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.shares["default"] = defaultShare
}

// Start 启动 Time Machine 服务.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("服务已在运行")
	}

	m.running = true
	m.startTime = time.Now()

	// 启动自动清理定时器
	if m.retention.AutoCleanup {
		m.cleanupTicker = time.NewTicker(24 * time.Hour)
		go func() {
			for {
				select {
				case <-m.cleanupTicker.C:
					m.autoCleanup()
				case <-m.stopCh:
					return
				}
			}
		}()
	}

	m.logger.Info("[Time Machine] 服务已启动")
	return nil
}

// Stop 停止 Time Machine 服务.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return fmt.Errorf("服务未运行")
	}

	m.running = false
	close(m.stopCh)

	if m.cleanupTicker != nil {
		m.cleanupTicker.Stop()
	}

	m.logger.Info("[Time Machine] 服务已停止")
	return nil
}

// ========== 共享管理 ==========

// ListShares 列出所有共享.
func (m *Manager) ListShares() []TimeMachineShare {
	m.mu.RLock()
	defer m.mu.RUnlock()

	shares := make([]TimeMachineShare, 0, len(m.shares))
	for _, share := range m.shares {
		shares = append(shares, *share)
	}
	return shares
}

// CreateShare 创建共享.
func (m *Manager) CreateShare(name, path string, protocol Protocol) (*TimeMachineShare, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查名称重复
	for _, share := range m.shares {
		if share.Name == name {
			return nil, fmt.Errorf("共享名称已存在: %s", name)
		}
	}

	// 校验协议
	if protocol != ProtocolAFP && protocol != ProtocolSMB {
		return nil, fmt.Errorf("不支持的协议: %s", protocol)
	}

	id := fmt.Sprintf("share_%d", time.Now().UnixNano())
	share := &TimeMachineShare{
		ID:        id,
		Name:      name,
		Path:      path,
		Protocol:  protocol,
		Enabled:   true,
		DeviceNum: 0,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	m.shares[id] = share
	m.updateStats()

	m.logger.Info("[Time Machine] 创建共享",
		zap.String("id", id),
		zap.String("name", name),
		zap.String("path", path))

	return share, nil
}

// UpdateShare 更新共享.
func (m *Manager) UpdateShare(id, name, path string, protocol Protocol, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	share, ok := m.shares[id]
	if !ok {
		return fmt.Errorf("共享不存在: %s", id)
	}

	if name != "" {
		share.Name = name
	}
	if path != "" {
		share.Path = path
	}
	if protocol != "" {
		if protocol != ProtocolAFP && protocol != ProtocolSMB {
			return fmt.Errorf("不支持的协议: %s", protocol)
		}
		share.Protocol = protocol
	}
	share.Enabled = enabled
	share.UpdatedAt = time.Now()

	m.logger.Info("[Time Machine] 更新共享", zap.String("id", id))
	return nil
}

// DeleteShare 删除共享.
func (m *Manager) DeleteShare(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	share, ok := m.shares[id]
	if !ok {
		return fmt.Errorf("共享不存在: %s", id)
	}

	if share.DeviceNum > 0 {
		return fmt.Errorf("共享 %s 仍有 %d 个设备关联，无法删除", share.Name, share.DeviceNum)
	}

	delete(m.shares, id)
	m.updateStats()

	m.logger.Info("[Time Machine] 删除共享", zap.String("id", id))
	return nil
}

// ========== 设备管理 ==========

// ListDevices 列出所有设备.
func (m *Manager) ListDevices() []BackupDevice {
	m.mu.RLock()
	defer m.mu.RUnlock()

	devices := make([]BackupDevice, 0, len(m.devices))
	for _, device := range m.devices {
		devices = append(devices, *device)
	}
	return devices
}

// RegisterDevice 注册设备.
func (m *Manager) RegisterDevice(hostname, macAddress, ipAddress, osVersion string) (*BackupDevice, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已注册
	for _, device := range m.devices {
		if device.MACAddress == macAddress {
			return nil, fmt.Errorf("设备已注册: %s", macAddress)
		}
	}

	id := fmt.Sprintf("device_%d", time.Now().UnixNano())
	device := &BackupDevice{
		ID:         id,
		Hostname:   hostname,
		MACAddress: macAddress,
		IPAddress:  ipAddress,
		OSVersion:  osVersion,
		Registered: time.Now(),
		Online:     true,
		Approved:   false,
	}

	m.devices[id] = device
	m.updateStats()

	m.logger.Info("[Time Machine] 注册设备",
		zap.String("id", id),
		zap.String("hostname", hostname),
		zap.String("mac", macAddress))

	return device, nil
}

// ApproveDevice 批准设备.
func (m *Manager) ApproveDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[id]
	if !ok {
		return fmt.Errorf("设备不存在: %s", id)
	}

	if device.Approved {
		return fmt.Errorf("设备已批准: %s", id)
	}

	device.Approved = true
	m.updateStats()

	m.logger.Info("[Time Machine] 批准设备", zap.String("id", id))
	return nil
}

// RemoveDevice 移除设备.
func (m *Manager) RemoveDevice(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[id]
	if !ok {
		return fmt.Errorf("设备不存在: %s", id)
	}

	// 删除设备的所有备份任务
	for jobID, job := range m.jobs {
		if job.DeviceID == id {
			delete(m.jobs, jobID)
		}
	}

	// 删除设备的所有快照
	for snapID, snap := range m.snapshots {
		if snap.DeviceID == id {
			delete(m.snapshots, snapID)
		}
	}

	// 删除配额
	delete(m.quotas, id)

	delete(m.devices, id)
	m.updateStats()

	m.logger.Info("[Time Machine] 移除设备",
		zap.String("id", id),
		zap.String("hostname", device.Hostname))

	return nil
}

// GetDeviceBackups 获取设备备份列表.
func (m *Manager) GetDeviceBackups(deviceID string) ([]BackupJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	backups := make([]BackupJob, 0)
	for _, job := range m.jobs {
		if job.DeviceID == deviceID {
			backups = append(backups, *job)
		}
	}
	return backups, nil
}

// ========== 备份任务管理 ==========

// ListBackups 列出所有备份任务.
func (m *Manager) ListBackups() []BackupJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]BackupJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, *job)
	}
	return jobs
}

// GetBackup 获取备份详情.
func (m *Manager) GetBackup(id string) (*BackupJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[id]
	if !ok {
		return nil, fmt.Errorf("备份任务不存在: %s", id)
	}
	return job, nil
}

// CreateBackup 创建备份任务.
func (m *Manager) CreateBackup(deviceID string) (*BackupJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查设备
	device, ok := m.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	if !device.Approved {
		return nil, fmt.Errorf("设备未批准: %s", deviceID)
	}

	// 检查配额
	quota, hasQuota := m.quotas[deviceID]
	if hasQuota && quota.UsedBytes >= quota.QuotaBytes {
		return nil, fmt.Errorf("设备配额已满: %s", deviceID)
	}

	id := fmt.Sprintf("job_%d", time.Now().UnixNano())
	now := time.Now()
	job := &BackupJob{
		ID:        id,
		DeviceID:  deviceID,
		Status:    BackupStatusRunning,
		StartTime: now,
		Size:      0,
		Duration:  0,
	}

	m.jobs[id] = job
	m.updateStats()

	m.logger.Info("[Time Machine] 创建备份任务",
		zap.String("id", id),
		zap.String("device", deviceID))

	return job, nil
}

// CompleteBackup 完成备份任务.
func (m *Manager) CompleteBackup(id string, size int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("备份任务不存在: %s", id)
	}

	if job.Status != BackupStatusRunning {
		return fmt.Errorf("任务状态不正确: %s", job.Status)
	}

	now := time.Now()
	job.Status = BackupStatusCompleted
	job.EndTime = &now
	job.Size = size
	job.Duration = int64(now.Sub(job.StartTime).Seconds())

	// 创建快照
	snapID := fmt.Sprintf("snap_%d", time.Now().UnixNano())
	snapshot := &BackupSnapshot{
		ID:         snapID,
		DeviceID:   job.DeviceID,
		JobID:      id,
		Timestamp:  now,
		Size:       size,
		Consistent: true,
	}
	m.snapshots[snapID] = snapshot

	// 更新配额使用量
	if quota, ok := m.quotas[job.DeviceID]; ok {
		quota.UsedBytes += size
		quota.FreeBytes = quota.QuotaBytes - quota.UsedBytes
	}

	m.updateStats()

	m.logger.Info("[Time Machine] 备份完成",
		zap.String("job", id),
		zap.Int64("size", size))

	return nil
}

// FailBackup 标记备份失败.
func (m *Manager) FailBackup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("备份任务不存在: %s", id)
	}

	if job.Status != BackupStatusRunning {
		return fmt.Errorf("任务状态不正确: %s", job.Status)
	}

	now := time.Now()
	job.Status = BackupStatusFailed
	job.EndTime = &now
	job.Duration = int64(now.Sub(job.StartTime).Seconds())

	m.logger.Info("[Time Machine] 备份失败", zap.String("job", id))
	return nil
}

// DeleteBackup 删除备份.
func (m *Manager) DeleteBackup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[id]
	if !ok {
		return fmt.Errorf("备份任务不存在: %s", id)
	}

	// 删除关联快照
	for snapID, snap := range m.snapshots {
		if snap.JobID == id {
			// 更新配额
			if quota, ok := m.quotas[job.DeviceID]; ok {
				quota.UsedBytes -= snap.Size
				if quota.UsedBytes < 0 {
					quota.UsedBytes = 0
				}
				quota.FreeBytes = quota.QuotaBytes - quota.UsedBytes
			}
			delete(m.snapshots, snapID)
		}
	}

	delete(m.jobs, id)
	m.updateStats()

	m.logger.Info("[Time Machine] 删除备份", zap.String("id", id))
	return nil
}

// ========== 配额管理 ==========

// GetQuota 获取设备配额.
func (m *Manager) GetQuota(deviceID string) (*BackupQuota, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.devices[deviceID]
	if !ok {
		return nil, fmt.Errorf("设备不存在: %s", deviceID)
	}

	quota, ok := m.quotas[deviceID]
	if !ok {
		// 返回默认配额
		return &BackupQuota{
			DeviceID:   deviceID,
			QuotaBytes: 0,
			UsedBytes:  0,
			FreeBytes:  0,
		}, nil
	}
	return quota, nil
}

// SetQuota 设置设备配额.
func (m *Manager) SetQuota(deviceID string, quotaBytes int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, ok := m.devices[deviceID]
	if !ok {
		return fmt.Errorf("设备不存在: %s", deviceID)
	}

	if quotaBytes < 0 {
		return fmt.Errorf("配额不能为负数")
	}

	quota, ok := m.quotas[deviceID]
	if !ok {
		quota = &BackupQuota{
			DeviceID: deviceID,
		}
		m.quotas[deviceID] = quota
	}

	quota.QuotaBytes = quotaBytes
	quota.FreeBytes = quotaBytes - quota.UsedBytes

	m.logger.Info("[Time Machine] 设置配额",
		zap.String("device", deviceID),
		zap.Int64("quota", quotaBytes))

	return nil
}

// ========== 保留策略 ==========

// GetRetentionPolicy 获取保留策略.
func (m *Manager) GetRetentionPolicy() RetentionPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.retention
}

// UpdateRetentionPolicy 更新保留策略.
func (m *Manager) UpdateRetentionPolicy(policy RetentionPolicy) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if policy.RetentionDays < 1 {
		return fmt.Errorf("保留天数不能小于 1")
	}
	if policy.MinKeep < 1 {
		return fmt.Errorf("最小保留数不能小于 1")
	}
	if policy.MaxBackups < policy.MinKeep {
		return fmt.Errorf("最大备份数不能小于最小保留数")
	}

	// 如果启用了自动清理，启动清理定时器
	if policy.AutoCleanup && !m.retention.AutoCleanup {
		m.cleanupTicker = time.NewTicker(24 * time.Hour)
		go func() {
			for {
				select {
				case <-m.cleanupTicker.C:
					m.autoCleanup()
				case <-m.stopCh:
					return
				}
			}
		}()
	}

	// 如果禁用了自动清理，停止定时器
	if !policy.AutoCleanup && m.retention.AutoCleanup {
		if m.cleanupTicker != nil {
			m.cleanupTicker.Stop()
			m.cleanupTicker = nil
		}
	}

	m.retention = policy

	m.logger.Info("[Time Machine] 更新保留策略",
		zap.Int("retentionDays", policy.RetentionDays),
		zap.Int("maxBackups", policy.MaxBackups))

	return nil
}

// CleanupExpiredBackups 手动触发清理.
func (m *Manager) CleanupExpiredBackups() (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	return m.cleanupExpired()
}

// autoCleanup 自动清理过期备份.
func (m *Manager) autoCleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	count, err := m.cleanupExpired()
	if err != nil {
		m.logger.Error("[Time Machine] 自动清理失败", zap.Error(err))
		return
	}

	if count > 0 {
		m.logger.Info("[Time Machine] 自动清理完成", zap.Int("count", count))
	}
}

// cleanupExpired 清理过期备份（内部调用，需要已加锁）.
func (m *Manager) cleanupExpired() (int, error) {
	count := 0
	cutoff := time.Now().AddDate(0, 0, -m.retention.RetentionDays)

	// 按设备分组备份
	deviceBackups := make(map[string][]*BackupJob)
	for _, job := range m.jobs {
		if job.Status == BackupStatusCompleted {
			deviceBackups[job.DeviceID] = append(deviceBackups[job.DeviceID], job)
		}
	}

	for deviceID, backups := range deviceBackups {
		// 按时间排序（新的在前）
		sortBackups(backups)

		// 检查每个备份
		for i, job := range backups {
			// 保留最小数量
			if i < m.retention.MinKeep {
				continue
			}

			// 检查是否过期
			if job.EndTime != nil && job.EndTime.Before(cutoff) {
				// 删除过期备份
				m.deleteJobInternal(job.ID)
				count++
			}
		}

		// 检查最大备份数
		if len(backups) > m.retention.MaxBackups {
			excess := len(backups) - m.retention.MaxBackups
			for i := len(backups) - 1; i >= 0 && excess > 0; i-- {
				// 保留最小数量
				if i < m.retention.MinKeep {
					break
				}
				m.deleteJobInternal(backups[i].ID)
				excess--
				count++
			}
		}

		// 更新设备关联的共享计数
		m.updateDeviceShareCount(deviceID)
	}

	return count, nil
}

// deleteJobInternal 删除备份任务（内部调用，需要已加锁）.
func (m *Manager) deleteJobInternal(id string) {
	job, ok := m.jobs[id]
	if !ok {
		return
	}

	// 删除关联快照
	for snapID, snap := range m.snapshots {
		if snap.JobID == id {
			// 更新配额
			if quota, ok := m.quotas[job.DeviceID]; ok {
				quota.UsedBytes -= snap.Size
				if quota.UsedBytes < 0 {
					quota.UsedBytes = 0
				}
				quota.FreeBytes = quota.QuotaBytes - quota.UsedBytes
			}
			delete(m.snapshots, snapID)
		}
	}

	delete(m.jobs, id)
}

// sortBackups 按时间排序备份（新的在前）.
func sortBackups(backups []*BackupJob) {
	for i := 1; i < len(backups); i++ {
		for j := i; j > 0; j-- {
			if backups[j].StartTime.After(backups[j-1].StartTime) {
				backups[j], backups[j-1] = backups[j-1], backups[j]
			} else {
				break
			}
		}
	}
}

// ========== 流量限制 ==========

// GetTrafficLimit 获取流量限制.
func (m *Manager) GetTrafficLimit() TrafficLimit {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.traffic
}

// UpdateTrafficLimit 更新流量限制.
func (m *Manager) UpdateTrafficLimit(limit TrafficLimit) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if limit.BandwidthKBps < 0 {
		return fmt.Errorf("带宽限制不能为负数")
	}

	m.traffic = limit

	m.logger.Info("[Time Machine] 更新流量限制",
		zap.Int("bandwidthKBps", limit.BandwidthKBps),
		zap.Bool("enabled", limit.Enabled))

	return nil
}

// ========== 广播配置 ==========

// GetBroadcastConfig 获取广播配置.
func (m *Manager) GetBroadcastConfig() BroadcastConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.broadcast
}

// UpdateBroadcastConfig 更新广播配置.
func (m *Manager) UpdateBroadcastConfig(config BroadcastConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.Port < 1 || config.Port > 65535 {
		return fmt.Errorf("端口范围无效: %d", config.Port)
	}

	m.broadcast = config

	m.logger.Info("[Time Machine] 更新广播配置",
		zap.String("serviceName", config.ServiceName),
		zap.Int("port", config.Port))

	return nil
}

// ========== 统计 ==========

// GetStats 获取统计信息.
func (m *Manager) GetStats() TimeMachineStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// updateStats 更新统计信息（内部调用，需要已加锁）.
func (m *Manager) updateStats() {
	stats := TimeMachineStats{}

	for _, device := range m.devices {
		stats.TotalDevices++
		if device.Online && device.Approved {
			stats.ActiveDevices++
		}
	}

	for _, snap := range m.snapshots {
		stats.TotalBackupSize += snap.Size
	}

	today := time.Now().Truncate(24 * time.Hour)
	for _, job := range m.jobs {
		if job.StartTime.After(today) {
			stats.TodayBackups++
		}
	}

	m.stats = stats
}

// updateDeviceShareCount 更新设备关联的共享计数.
func (m *Manager) updateDeviceShareCount(deviceID string) {
	// 这里简化处理，直接统计所有设备数
	// 实际应该统计每个共享关联的设备数
	for _, share := range m.shares {
		share.DeviceNum = len(m.devices)
	}
}

// GetServiceStatus 获取服务状态.
func (m *Manager) GetServiceStatus() ServiceStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return ServiceStatus{
		Running:   m.running,
		Shares:    len(m.shares),
		Devices:   len(m.devices),
		StartTime: m.startTime,
	}
}
