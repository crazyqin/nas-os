// Package systemclone - 系统克隆管理器 v2
// 新增：RAID1 镜像保护、自动故障转移、健康监控、在线扩容迁移
package systemclone

import (
	"fmt"
	"log"
	"math/rand"
	"sync"
	"time"
)

// Manager 系统克隆管理器
type Manager struct {
	mu              sync.RWMutex
	tasks           map[string]*DiskCloneTask
	images          map[string]*BackupImage
	restores        map[string]*RestoreTask
	pxe             map[string]*PXEDeployConfig
	stats           *CloneStats

	// 新增：系统盘镜像
	mirrors         map[string]*SystemMirror
	diskHealth      map[string]*DiskHealthInfo
	healthConfig    HealthMonitorConfig
	failoverEvents  map[string]*FailoverEvent
	migrationTasks  map[string]*MigrationTask
	expandTasks     map[string]*ExpandTask
	mirrorStats     *MirrorStats
	healthStopCh    chan struct{}
	healthRunning   bool
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		tasks:          make(map[string]*DiskCloneTask),
		images:         make(map[string]*BackupImage),
		restores:       make(map[string]*RestoreTask),
		pxe:            make(map[string]*PXEDeployConfig),
		stats:          &CloneStats{},
		mirrors:        make(map[string]*SystemMirror),
		diskHealth:     make(map[string]*DiskHealthInfo),
		failoverEvents: make(map[string]*FailoverEvent),
		migrationTasks: make(map[string]*MigrationTask),
		expandTasks:    make(map[string]*ExpandTask),
		mirrorStats:    &MirrorStats{},
		healthConfig: HealthMonitorConfig{
			Enabled:              true,
			CheckIntervalSec:     300, // 5 分钟
			TemperatureThreshold: 60,
			HealthScoreThreshold: 70,
			MaxReallocatedSect:   100,
			MaxPendingSect:       50,
			AutoFailover:         true,
			FailoverDelaySec:     30,
		},
	}
}

// ============================================================
// 原有功能
// ============================================================

// CreateCloneTask 创建克隆任务
func (m *Manager) CreateCloneTask(task *DiskCloneTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task.ID == "" {
		task.ID = fmt.Sprintf("clone-%d", time.Now().UnixNano())
	}
	task.Status = CloneStatusPending
	task.CreatedAt = time.Now()
	m.tasks[task.ID] = task
	m.stats.TotalClones++
	return nil
}

// StartClone 启动克隆任务
func (m *Manager) StartClone(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Status = CloneStatusRunning
	now := time.Now()
	task.StartedAt = &now

	go func() {
		time.Sleep(3 * time.Second)
		m.mu.Lock()
		defer m.mu.Unlock()
		task.Status = CloneStatusCompleted
		task.Progress = 100
		task.BytesCopied = task.BytesTotal
		completed := time.Now()
		task.CompletedAt = &completed
		m.stats.SuccessfulClones++
	}()

	return nil
}

// GetCloneTask 获取克隆任务
func (m *Manager) GetCloneTask(id string) (*DiskCloneTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, fmt.Errorf("task %s not found", id)
	}
	return task, nil
}

// ListCloneTasks 列出克隆任务
func (m *Manager) ListCloneTasks() []*DiskCloneTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*DiskCloneTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// CreateImage 创建备份镜像
func (m *Manager) CreateImage(image *BackupImage) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if image.ID == "" {
		image.ID = fmt.Sprintf("img-%d", time.Now().UnixNano())
	}
	image.CreatedAt = time.Now()
	m.images[image.ID] = image
	m.stats.TotalImages++
	m.stats.TotalImageSize += image.SizeBytes
	return nil
}

// ListImages 列出镜像
func (m *Manager) ListImages() []*BackupImage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	images := make([]*BackupImage, 0, len(m.images))
	for _, img := range m.images {
		images = append(images, img)
	}
	return images
}

// RestoreFromImage 从镜像恢复
func (m *Manager) RestoreFromImage(imageID, targetDisk string) (*RestoreTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.images[imageID]; !ok {
		return nil, fmt.Errorf("image %s not found", imageID)
	}

	task := &RestoreTask{
		ID:         fmt.Sprintf("restore-%d", time.Now().UnixNano()),
		ImageID:    imageID,
		TargetDisk: targetDisk,
		Status:     CloneStatusRunning,
		Progress:   0,
		CreatedAt:  time.Now(),
	}
	m.restores[task.ID] = task
	m.stats.TotalRestores++

	go func() {
		time.Sleep(5 * time.Second)
		m.mu.Lock()
		defer m.mu.Unlock()
		task.Status = CloneStatusCompleted
		task.Progress = 100
		completed := time.Now()
		task.CompletedAt = &completed
	}()

	return task, nil
}

// ConfigurePXE 配置 PXE 部署
func (m *Manager) ConfigurePXE(config *PXEDeployConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.ID == "" {
		config.ID = fmt.Sprintf("pxe-%d", time.Now().UnixNano())
	}
	m.pxe[config.ID] = config
	return nil
}

// GetStats 获取统计
func (m *Manager) GetStats() *CloneStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.stats
}

// ============================================================
// 新增功能 - RAID1 系统盘镜像
// ============================================================

// CreateMirror 创建系统盘 RAID1 镜像
func (m *Manager) CreateMirror(primaryDisk, secondaryDisk string, spareDisks []string) (*SystemMirror, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if primaryDisk == "" || secondaryDisk == "" {
		return nil, fmt.Errorf("主盘和镜像盘不能为空")
	}
	if primaryDisk == secondaryDisk {
		return nil, fmt.Errorf("主盘和镜像盘不能相同")
	}

	// 检查磁盘是否已被使用
	for _, mirror := range m.mirrors {
		if mirror.PrimaryDisk == primaryDisk || mirror.SecondaryDisk == primaryDisk {
			return nil, fmt.Errorf("磁盘 %s 已被镜像 %s 使用", primaryDisk, mirror.ID)
		}
		if mirror.PrimaryDisk == secondaryDisk || mirror.SecondaryDisk == secondaryDisk {
			return nil, fmt.Errorf("磁盘 %s 已被镜像 %s 使用", secondaryDisk, mirror.ID)
		}
	}

	now := time.Now()
	mirror := &SystemMirror{
		ID:             fmt.Sprintf("mirror-%d", now.UnixNano()),
		Name:           fmt.Sprintf("system-mirror-%s", now.Format("20060102")),
		PrimaryDisk:    primaryDisk,
		SecondaryDisk:  secondaryDisk,
		SpareDisks:     spareDisks,
		Status:         MirrorStatusSyncing,
		BootDisk:       primaryDisk,
		TotalSizeBytes: 256 * 1024 * 1024 * 1024, // 模拟 256GB
		SyncProgress:   0,
		CreatedAt:      now,
		UpdatedAt:      now,
	}
	m.mirrors[mirror.ID] = mirror
	m.mirrorStats.TotalMirrors++

	// 模拟同步完成
	go m.simulateMirrorSync(mirror.ID)

	log.Printf("[systemclone] 创建 RAID1 镜像: %s (主盘: %s, 镜像盘: %s)", mirror.ID, primaryDisk, secondaryDisk)
	return mirror, nil
}

// simulateMirrorSync 模拟镜像同步过程
func (m *Manager) simulateMirrorSync(mirrorID string) {
	for i := 0; i <= 100; i += 5 {
		time.Sleep(500 * time.Millisecond)
		m.mu.Lock()
		mirror, ok := m.mirrors[mirrorID]
		if !ok {
			m.mu.Unlock()
			return
		}
		mirror.SyncProgress = i
		mirror.UpdatedAt = time.Now()
		m.mu.Unlock()
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	mirror, ok := m.mirrors[mirrorID]
	if !ok {
		return
	}
	mirror.Status = MirrorStatusHealthy
	mirror.SyncProgress = 100
	now := time.Now()
	mirror.LastSyncTime = &now
	mirror.UpdatedAt = now
	m.mirrorStats.HealthyMirrors++
	log.Printf("[systemclone] RAID1 镜像 %s 同步完成，状态: healthy", mirrorID)
}

// GetMirror 获取镜像信息
func (m *Manager) GetMirror(id string) (*SystemMirror, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mirror, ok := m.mirrors[id]
	if !ok {
		return nil, fmt.Errorf("镜像 %s 不存在", id)
	}
	return mirror, nil
}

// ListMirrors 列出所有镜像
func (m *Manager) ListMirrors() []*SystemMirror {
	m.mu.RLock()
	defer m.mu.RUnlock()

	mirrors := make([]*SystemMirror, 0, len(m.mirrors))
	for _, mirror := range m.mirrors {
		mirrors = append(mirrors, mirror)
	}
	return mirrors
}

// DeleteMirror 删除镜像
func (m *Manager) DeleteMirror(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	mirror, ok := m.mirrors[id]
	if !ok {
		return fmt.Errorf("镜像 %s 不存在", id)
	}
	if mirror.Status == MirrorStatusRebuilding {
		return fmt.Errorf("镜像正在重建中，无法删除")
	}

	delete(m.mirrors, id)
	m.mirrorStats.TotalMirrors--
	if mirror.Status == MirrorStatusHealthy {
		m.mirrorStats.HealthyMirrors--
	} else if mirror.Status == MirrorStatusDegraded {
		m.mirrorStats.DegradedMirrors--
	}

	log.Printf("[systemclone] 删除镜像: %s", id)
	return nil
}

// ============================================================
// 新增功能 - 健康监控
// ============================================================

// StartHealthMonitor 启动健康监控
func (m *Manager) StartHealthMonitor() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.healthRunning {
		return fmt.Errorf("健康监控已在运行")
	}

	m.healthStopCh = make(chan struct{})
	m.healthRunning = true

	go m.healthMonitorLoop()

	log.Println("[systemclone] 健康监控已启动")
	return nil
}

// StopHealthMonitor 停止健康监控
func (m *Manager) StopHealthMonitor() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.healthRunning {
		return
	}

	close(m.healthStopCh)
	m.healthRunning = false
	log.Println("[systemclone] 健康监控已停止")
}

// healthMonitorLoop 健康监控循环
func (m *Manager) healthMonitorLoop() {
	interval := time.Duration(m.healthConfig.CheckIntervalSec) * time.Second
	if interval < 10*time.Second {
		interval = 10 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-m.healthStopCh:
			return
		case <-ticker.C:
			m.performHealthCheck()
		}
	}
}

// performHealthCheck 执行健康检查
func (m *Manager) performHealthCheck() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	for _, mirror := range m.mirrors {
		// 检查主盘健康
		primaryHealth := m.checkDiskHealth(mirror.PrimaryDisk, DiskRolePrimary)
		m.diskHealth[mirror.PrimaryDisk] = primaryHealth

		// 检查镜像盘健康
		secondaryHealth := m.checkDiskHealth(mirror.SecondaryDisk, DiskRoleSecondary)
		m.diskHealth[mirror.SecondaryDisk] = secondaryHealth

		// 检查热备盘健康
		for _, spare := range mirror.SpareDisks {
			spareHealth := m.checkDiskHealth(spare, DiskRoleSpare)
			m.diskHealth[spare] = spareHealth
		}

		mirror.LastCheckTime = &now
		mirror.UpdatedAt = now

		// 更新镜像状态
		m.updateMirrorStatus(mirror, primaryHealth, secondaryHealth)

		// 自动故障转移检查
		if m.healthConfig.AutoFailover {
			m.checkAutoFailover(mirror, primaryHealth, secondaryHealth)
		}
	}
}

// checkDiskHealth 检查单个磁盘健康（模拟 SMART 数据读取）
func (m *Manager) checkDiskHealth(device string, role DiskRole) *DiskHealthInfo {
	health, exists := m.diskHealth[device]
	if exists {
		// 更新已有记录
		health.LastCheckTime = time.Now()
		// 模拟轻微波动
		health.Temperature = 35 + rand.Intn(15)
		health.HealthScore = health.HealthScore + (rand.Float64()*2 - 1)
		if health.HealthScore > 100 {
			health.HealthScore = 100
		}
		if health.HealthScore < 0 {
			health.HealthScore = 0
		}
		health.HealthStatus = m.scoreToStatus(health.HealthScore)
		return health
	}

	// 创建新记录
	score := 85.0 + rand.Float64()*15 // 模拟 85-100 的初始健康分
	return &DiskHealthInfo{
		Device:            device,
		Role:              role,
		HealthStatus:      m.scoreToStatus(score),
		Temperature:       35 + rand.Intn(15),
		PowerOnHours:      1000 + rand.Int63n(10000),
		ReallocatedSect:   rand.Int63n(10),
		PendingSect:       rand.Int63n(5),
		UncorrectableSect: 0,
		HealthScore:       score,
		LastCheckTime:     time.Now(),
	}
}

// scoreToStatus 健康评分转状态
func (m *Manager) scoreToStatus(score float64) HealthStatus {
	switch {
	case score >= 80:
		return HealthStatusGood
	case score >= 60:
		return HealthStatusWarning
	case score >= 40:
		return HealthStatusCritical
	default:
		return HealthStatusFailed
	}
}

// updateMirrorStatus 更新镜像状态
func (m *Manager) updateMirrorStatus(mirror *SystemMirror, primary, secondary *DiskHealthInfo) {
	oldStatus := mirror.Status

	if primary.HealthStatus == HealthStatusFailed && secondary.HealthStatus == HealthStatusFailed {
		mirror.Status = MirrorStatusFailed
	} else if primary.HealthStatus == HealthStatusFailed || secondary.HealthStatus == HealthStatusFailed {
		mirror.Status = MirrorStatusDegraded
	} else if primary.HealthStatus == HealthStatusCritical || secondary.HealthStatus == HealthStatusCritical {
		mirror.Status = MirrorStatusDegraded
	} else if mirror.Status != MirrorStatusSyncing && mirror.Status != MirrorStatusRebuilding {
		mirror.Status = MirrorStatusHealthy
	}

	if oldStatus != mirror.Status {
		log.Printf("[systemclone] 镜像 %s 状态变更: %s -> %s", mirror.ID, oldStatus, mirror.Status)
	}
}

// checkAutoFailover 检查自动故障转移
func (m *Manager) checkAutoFailover(mirror *SystemMirror, primary, secondary *DiskHealthInfo) {
	// 主盘故障，自动切换到镜像盘
	if primary.HealthStatus == HealthStatusFailed && secondary.HealthStatus != HealthStatusFailed {
		event := &FailoverEvent{
			ID:          fmt.Sprintf("failover-%d", time.Now().UnixNano()),
			MirrorID:    mirror.ID,
			TriggerType: FailoverTriggerAuto,
			FailedDisk:  primary.Device,
			FailedRole:  DiskRolePrimary,
			Reason:      fmt.Sprintf("主盘健康状态: %s, 评分: %.1f", primary.HealthStatus, primary.HealthScore),
			NewBootDisk: secondary.Device,
			Status:      "pending",
			CreatedAt:   time.Now(),
		}
		m.failoverEvents[event.ID] = event
		m.mirrorStats.TotalFailovers++
		m.mirrorStats.AutoFailovers++

		// 执行故障转移
		go m.executeFailover(event.ID, mirror.ID, secondary.Device)
		log.Printf("[systemclone] 触发自动故障转移: 主盘 %s 故障，切换到 %s", primary.Device, secondary.Device)
	}
}

// executeFailover 执行故障转移
func (m *Manager) executeFailover(eventID, mirrorID, newBootDisk string) {
	// 模拟故障转移延迟
	delay := time.Duration(m.healthConfig.FailoverDelaySec) * time.Second
	if delay < 1*time.Second {
		delay = 1 * time.Second
	}
	time.Sleep(delay)

	m.mu.Lock()
	defer m.mu.Unlock()

	event, ok := m.failoverEvents[eventID]
	if !ok {
		return
	}

	mirror, ok := m.mirrors[mirrorID]
	if !ok {
		event.Status = "failed"
		event.ErrorMessage = "镜像不存在"
		return
	}

	// 切换启动盘
	mirror.BootDisk = newBootDisk
	mirror.UpdatedAt = time.Now()

	event.Status = "completed"
	completed := time.Now()
	event.CompletedAt = &completed

	log.Printf("[systemclone] 故障转移完成: %s -> %s", event.FailedDisk, newBootDisk)
}

// ManualFailover 手动故障转移
func (m *Manager) ManualFailover(mirrorID, targetDisk string) (*FailoverEvent, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mirror, ok := m.mirrors[mirrorID]
	if !ok {
		return nil, fmt.Errorf("镜像 %s 不存在", mirrorID)
	}

	if targetDisk != mirror.PrimaryDisk && targetDisk != mirror.SecondaryDisk {
		return nil, fmt.Errorf("目标磁盘 %s 不属于镜像 %s", targetDisk, mirrorID)
	}

	if targetDisk == mirror.BootDisk {
		return nil, fmt.Errorf("目标磁盘 %s 已是当前启动盘", targetDisk)
	}

	// 确定故障盘
	var failedDisk string
	var failedRole DiskRole
	if targetDisk == mirror.PrimaryDisk {
		failedDisk = mirror.SecondaryDisk
		failedRole = DiskRoleSecondary
	} else {
		failedDisk = mirror.PrimaryDisk
		failedRole = DiskRolePrimary
	}

	event := &FailoverEvent{
		ID:          fmt.Sprintf("failover-%d", time.Now().UnixNano()),
		MirrorID:    mirrorID,
		TriggerType: FailoverTriggerManual,
		FailedDisk:  failedDisk,
		FailedRole:  failedRole,
		Reason:      "手动切换",
		NewBootDisk: targetDisk,
		Status:      "completed",
		CreatedAt:   time.Now(),
	}
	completed := time.Now()
	event.CompletedAt = &completed

	m.failoverEvents[event.ID] = event
	m.mirrorStats.TotalFailovers++
	m.mirrorStats.ManualFailovers++

	mirror.BootDisk = targetDisk
	mirror.UpdatedAt = time.Now()

	log.Printf("[systemclone] 手动故障转移: 启动盘切换到 %s", targetDisk)
	return event, nil
}

// ============================================================
// 新增功能 - 在线扩容和迁移
// ============================================================

// StartMigration 启动在线磁盘迁移
func (m *Manager) StartMigration(mirrorID, sourceDisk, targetDisk string) (*MigrationTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mirror, ok := m.mirrors[mirrorID]
	if !ok {
		return nil, fmt.Errorf("镜像 %s 不存在", mirrorID)
	}

	// 验证源盘属于镜像
	if sourceDisk != mirror.PrimaryDisk && sourceDisk != mirror.SecondaryDisk {
		return nil, fmt.Errorf("源磁盘 %s 不属于镜像 %s", sourceDisk, mirrorID)
	}

	// 验证目标盘未被使用
	if targetDisk == mirror.PrimaryDisk || targetDisk == mirror.SecondaryDisk {
		return nil, fmt.Errorf("目标磁盘 %s 已在镜像中使用", targetDisk)
	}

	now := time.Now()
	task := &MigrationTask{
		ID:          fmt.Sprintf("migrate-%d", now.UnixNano()),
		MirrorID:    mirrorID,
		SourceDisk:  sourceDisk,
		TargetDisk:  targetDisk,
		Phase:       "sync",
		Status:      MigrationStatusRunning,
		Progress:    0,
		BytesTotal:  mirror.TotalSizeBytes,
		BytesCopied: 0,
		SpeedMBps:   120.0, // 模拟速度
		CreatedAt:   now,
		StartedAt:   &now,
	}
	m.migrationTasks[task.ID] = task
	m.mirrorStats.TotalMigrations++

	// 标记镜像为重建中
	mirror.Status = MirrorStatusRebuilding
	mirror.UpdatedAt = now

	// 模拟迁移过程
	go m.simulateMigration(task.ID, mirrorID, sourceDisk, targetDisk)

	log.Printf("[systemclone] 启动磁盘迁移: %s -> %s (镜像: %s)", sourceDisk, targetDisk, mirrorID)
	return task, nil
}

// simulateMigration 模拟迁移过程
func (m *Manager) simulateMigration(taskID, mirrorID, sourceDisk, targetDisk string) {
	// Phase 1: 同步
	for i := 0; i <= 100; i += 10 {
		time.Sleep(800 * time.Millisecond)
		m.mu.Lock()
		task, ok := m.migrationTasks[taskID]
		if !ok {
			m.mu.Unlock()
			return
		}
		task.Progress = i
		task.BytesCopied = int64(float64(task.BytesTotal) * float64(i) / 100.0)
		m.mu.Unlock()
	}

	// Phase 2: 验证
	m.mu.Lock()
	task, ok := m.migrationTasks[taskID]
	if !ok {
		m.mu.Unlock()
		return
	}
	task.Phase = "verify"
	task.Progress = 0
	m.mu.Unlock()

	time.Sleep(2 * time.Second)

	// Phase 3: 切换
	m.mu.Lock()
	task, ok = m.migrationTasks[taskID]
	if !ok {
		m.mu.Unlock()
		return
	}
	task.Phase = "switch"
	task.Progress = 100
	task.Status = MigrationStatusCompleted
	completed := time.Now()
	task.CompletedAt = &completed
	task.SpeedMBps = 125.0

	// 更新镜像
	mirror, ok := m.mirrors[mirrorID]
	if ok {
		if sourceDisk == mirror.PrimaryDisk {
			mirror.PrimaryDisk = targetDisk
		} else {
			mirror.SecondaryDisk = targetDisk
		}
		mirror.Status = MirrorStatusHealthy
		now := time.Now()
		mirror.LastSyncTime = &now
		mirror.UpdatedAt = now
		m.mirrorStats.SuccessfulMigrations++
	}
	m.mu.Unlock()

	log.Printf("[systemclone] 磁盘迁移完成: %s -> %s", sourceDisk, targetDisk)
}

// GetMigrationTask 获取迁移任务
func (m *Manager) GetMigrationTask(id string) (*MigrationTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.migrationTasks[id]
	if !ok {
		return nil, fmt.Errorf("迁移任务 %s 不存在", id)
	}
	return task, nil
}

// ListMigrationTasks 列出迁移任务
func (m *Manager) ListMigrationTasks() []*MigrationTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*MigrationTask, 0, len(m.migrationTasks))
	for _, t := range m.migrationTasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// StartExpand 启动在线扩容（替换为更大容量的磁盘）
func (m *Manager) StartExpand(mirrorID, oldDisk, newDisk string) (*ExpandTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	mirror, ok := m.mirrors[mirrorID]
	if !ok {
		return nil, fmt.Errorf("镜像 %s 不存在", mirrorID)
	}

	if oldDisk != mirror.PrimaryDisk && oldDisk != mirror.SecondaryDisk {
		return nil, fmt.Errorf("磁盘 %s 不属于镜像 %s", oldDisk, mirrorID)
	}

	if newDisk == mirror.PrimaryDisk || newDisk == mirror.SecondaryDisk {
		return nil, fmt.Errorf("新磁盘 %s 已在镜像中使用", newDisk)
	}

	now := time.Now()
	task := &ExpandTask{
		ID:       fmt.Sprintf("expand-%d", now.UnixNano()),
		MirrorID: mirrorID,
		NewDisk:  newDisk,
		OldDisk:  oldDisk,
		Phase:    "add",
		Status:   MigrationStatusRunning,
		Progress: 0,
		CreatedAt: now,
	}
	m.expandTasks[task.ID] = task
	m.mirrorStats.TotalExpansions++

	// 标记镜像为重建中
	mirror.Status = MirrorStatusRebuilding
	mirror.UpdatedAt = now

	// 模拟扩容过程
	go m.simulateExpand(task.ID, mirrorID, oldDisk, newDisk)

	log.Printf("[systemclone] 启动在线扩容: 替换 %s -> %s (镜像: %s)", oldDisk, newDisk, mirrorID)
	return task, nil
}

// simulateExpand 模拟扩容过程
func (m *Manager) simulateExpand(taskID, mirrorID, oldDisk, newDisk string) {
	phases := []string{"add", "sync", "verify", "replace"}

	for _, phase := range phases {
		m.mu.Lock()
		task, ok := m.expandTasks[taskID]
		if !ok {
			m.mu.Unlock()
			return
		}
		task.Phase = phase
		task.Progress = 0
		m.mu.Unlock()

		// 模拟每个阶段的进度
		for i := 0; i <= 100; i += 20 {
			time.Sleep(500 * time.Millisecond)
			m.mu.Lock()
			task, ok := m.expandTasks[taskID]
			if !ok {
				m.mu.Unlock()
				return
			}
			task.Progress = i
			m.mu.Unlock()
		}
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.expandTasks[taskID]
	if !ok {
		return
	}
	task.Status = MigrationStatusCompleted
	task.Progress = 100
	completed := time.Now()
	task.CompletedAt = &completed

	// 更新镜像
	mirror, ok := m.mirrors[mirrorID]
	if ok {
		if oldDisk == mirror.PrimaryDisk {
			mirror.PrimaryDisk = newDisk
		} else {
			mirror.SecondaryDisk = newDisk
		}
		mirror.Status = MirrorStatusHealthy
		// 模拟扩容后的容量增加
		mirror.TotalSizeBytes = mirror.TotalSizeBytes * 2
		now := time.Now()
		mirror.LastSyncTime = &now
		mirror.UpdatedAt = now
		m.mirrorStats.SuccessfulExpansions++
	}

	log.Printf("[systemclone] 在线扩容完成: %s -> %s", oldDisk, newDisk)
}

// GetExpandTask 获取扩容任务
func (m *Manager) GetExpandTask(id string) (*ExpandTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.expandTasks[id]
	if !ok {
		return nil, fmt.Errorf("扩容任务 %s 不存在", id)
	}
	return task, nil
}

// ListExpandTasks 列出扩容任务
func (m *Manager) ListExpandTasks() []*ExpandTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*ExpandTask, 0, len(m.expandTasks))
	for _, t := range m.expandTasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// ============================================================
// 辅助功能
// ============================================================

// GetDiskHealth 获取磁盘健康信息
func (m *Manager) GetDiskHealth(device string) (*DiskHealthInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	health, ok := m.diskHealth[device]
	if !ok {
		return nil, fmt.Errorf("磁盘 %s 的健康信息不存在", device)
	}
	return health, nil
}

// ListDiskHealth 列出所有磁盘健康信息
func (m *Manager) ListDiskHealth() []*DiskHealthInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	healthList := make([]*DiskHealthInfo, 0, len(m.diskHealth))
	for _, h := range m.diskHealth {
		healthList = append(healthList, h)
	}
	return healthList
}

// GetHealthConfig 获取健康监控配置
func (m *Manager) GetHealthConfig() HealthMonitorConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.healthConfig
}

// UpdateHealthConfig 更新健康监控配置
func (m *Manager) UpdateHealthConfig(config HealthMonitorConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.healthConfig = config
}

// ListFailoverEvents 列出故障转移事件
func (m *Manager) ListFailoverEvents() []*FailoverEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	events := make([]*FailoverEvent, 0, len(m.failoverEvents))
	for _, e := range m.failoverEvents {
		events = append(events, e)
	}
	return events
}

// GetMirrorStats 获取镜像统计
func (m *Manager) GetMirrorStats() *MirrorStats {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.mirrorStats
}
