// Package smartdiskmigrat 提供智能磁盘迁移功能
// NAS迁移、磁盘搬迁、数据完整性验证、RAID迁移
package smartdiskmigrat

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// MigrationType 迁移类型
type MigrationType string

const (
	MigrationTypeDiskSwap    MigrationType = "disk_swap"    // 同机换盘
	MigrationTypePoolMigrate MigrationType = "pool_migrate" // 存储池迁移
	MigrationTypeRAIDMigrate MigrationType = "raid_migrate" // RAID 级别迁移
	MigrationTypeNasToNas    MigrationType = "nas_to_nas"   // NAS 到 NAS
)

// MigrationStatus 迁移状态
type MigrationStatus string

const (
	StatusPending    MigrationStatus = "pending"     // 待执行
	StatusRunning    MigrationStatus = "running"     // 运行中
	StatusPaused     MigrationStatus = "paused"      // 已暂停
	StatusCompleted  MigrationStatus = "completed"   // 已完成
	StatusFailed     MigrationStatus = "failed"      // 失败
	StatusCancelled  MigrationStatus = "cancelled"   // 已取消
	StatusRolledBack MigrationStatus = "rolled_back" // 已回滚
)

// StepStatus 步骤状态
type StepStatus string

const (
	StepPending   StepStatus = "pending"
	StepRunning   StepStatus = "running"
	StepCompleted StepStatus = "completed"
	StepFailed    StepStatus = "failed"
	StepSkipped   StepStatus = "skipped"
)

// HotSpareType 热备类型
type HotSpareType string

const (
	HotSpareTypeHot  HotSpareType = "hot"  // 热备
	HotSpareTypeWarm HotSpareType = "warm" // 暖备
)

// DiskInfo 磁盘信息
type DiskInfo struct {
	DeviceName string  `json:"deviceName"` // /dev/sda
	Model      string  `json:"model"`
	SizeBytes  int64   `json:"sizeBytes"`
	Health     float64 `json:"health"` // 0-100%
	TempC      int     `json:"tempC"`
	SmartOK    bool    `json:"smartOk"`
	Serial     string  `json:"serial"`
}

// MigrationPlan 迁移计划
type MigrationPlan struct {
	ID           string          `json:"id"`
	Name         string          `json:"name"`
	SourceDevice string          `json:"sourceDevice"`
	TargetDevice string          `json:"targetDevice"`
	Type         MigrationType   `json:"type"`
	Status       MigrationStatus `json:"status"`
	Steps        []MigrationStep `json:"steps"`
	Warnings     []string        `json:"warnings,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
	UpdatedAt    time.Time       `json:"updatedAt"`
}

// MigrationStep 迁移步骤
type MigrationStep struct {
	Sequence    int           `json:"sequence"`
	Description string        `json:"description"`
	Status      StepStatus    `json:"status"`
	Progress    float64       `json:"progress"` // 0-100%
	Duration    time.Duration `json:"duration"`
	Error       string        `json:"error,omitempty"`
}

// MigrationJob 迁移任务
type MigrationJob struct {
	ID          string          `json:"id"`
	PlanID      string          `json:"planId"`
	Status      MigrationStatus `json:"status"`
	CurrentStep int             `json:"currentStep"`
	TotalSteps  int             `json:"totalSteps"`
	Progress    float64         `json:"progress"` // 0-100%
	StartedAt   time.Time       `json:"startedAt"`
	FinishedAt  *time.Time      `json:"finishedAt,omitempty"`
	Error       string          `json:"error,omitempty"`
}

// RAIDMigrateInfo RAID 迁移信息
type RAIDMigrateInfo struct {
	SourceLevel string          `json:"sourceLevel"` // raid1, raid5, raid6, etc.
	TargetLevel string          `json:"targetLevel"`
	Status      MigrationStatus `json:"status"`
	Progress    float64         `json:"progress"`
}

// HotSpare 热备盘信息
type HotSpare struct {
	DiskName string       `json:"diskName"`
	Type     HotSpareType `json:"type"` // hot/warm
	PoolID   string       `json:"poolId"`
	Status   string       `json:"status"` // active/standby/in_use
}

// IntegrityCheck 完整性检查结果
type IntegrityCheck struct {
	TotalFiles   int           `json:"totalFiles"`
	PassedFiles  int           `json:"passedFiles"`
	FailedFiles  int           `json:"failedFiles"`
	Duration     time.Duration `json:"duration"`
	CheckedBytes int64         `json:"checkedBytes"`
	Errors       []string      `json:"errors,omitempty"`
}

// TimeEstimate 时间预估
type TimeEstimate struct {
	DiskSizeBytes int64         `json:"diskSizeBytes"`
	EstimatedTime time.Duration `json:"estimatedTime"`
	TransferRate  int64         `json:"transferRate"` // bytes/sec
	Bottleneck    string        `json:"bottleneck"`   // disk, network, cpu
}

// ========== Manager ==========

// Manager 智能磁盘迁移管理器
type Manager struct {
	mu         sync.RWMutex
	plans      map[string]*MigrationPlan
	jobs       map[string]*MigrationJob
	hotSpares  map[string]*HotSpare
	disks      []DiskInfo
	nextPlanID int
	nextJobID  int
}

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		plans:      make(map[string]*MigrationPlan),
		jobs:       make(map[string]*MigrationJob),
		hotSpares:  make(map[string]*HotSpare),
		nextPlanID: 1,
		nextJobID:  1,
	}
	m.initDefaults()
	return m
}

// initDefaults 初始化默认数据
func (m *Manager) initDefaults() {
	// 模拟磁盘
	m.disks = []DiskInfo{
		{DeviceName: "/dev/sda", Model: "WDC WD40EFRX", SizeBytes: 4000787030016, Health: 98.0, TempC: 35, SmartOK: true, Serial: "WD-WCC4E0123456"},
		{DeviceName: "/dev/sdb", Model: "WDC WD40EFRX", SizeBytes: 4000787030016, Health: 95.0, TempC: 37, SmartOK: true, Serial: "WD-WCC4E0123457"},
		{DeviceName: "/dev/sdc", Model: "Seagate ST4000VN008", SizeBytes: 4000787030016, Health: 92.0, TempC: 38, SmartOK: true, Serial: "ZA123456"},
		{DeviceName: "/dev/sdd", Model: "Seagate ST4000VN008", SizeBytes: 4000787030016, Health: 97.0, TempC: 36, SmartOK: true, Serial: "ZA123457"},
		{DeviceName: "/dev/sde", Model: "Toshiba MG04ACA400N", SizeBytes: 4000787030016, Health: 100.0, TempC: 33, SmartOK: true, Serial: "87ABCDEF"},
	}

	// 热备盘
	m.hotSpares["/dev/sde"] = &HotSpare{
		DiskName: "/dev/sde", Type: HotSpareTypeHot, PoolID: "pool-1", Status: "standby",
	}

	// 示例计划
	m.plans["plan-1"] = &MigrationPlan{
		ID: "plan-1", Name: "替换 sda", SourceDevice: "/dev/sda", TargetDevice: "/dev/sde",
		Type: MigrationTypeDiskSwap, Status: StatusPending, CreatedAt: time.Now().Add(-1 * time.Hour), UpdatedAt: time.Now().Add(-1 * time.Hour),
		Steps: []MigrationStep{
			{Sequence: 1, Description: "检查目标磁盘", Status: StepPending},
			{Sequence: 2, Description: "同步数据", Status: StepPending},
			{Sequence: 3, Description: "验证数据完整性", Status: StepPending},
			{Sequence: 4, Description: "切换磁盘", Status: StepPending},
		},
	}
	m.nextPlanID = 2
}

// ========== 磁盘扫描 ==========

// ScanDisks 扫描所有磁盘
func (m *Manager) ScanDisks() ([]DiskInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 在实际实现中会扫描 /dev/sd*
	return m.disks, nil
}

// ========== 计划管理 ==========

// CreatePlan 创建迁移计划
func (m *Manager) CreatePlan(plan *MigrationPlan) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if plan.ID == "" {
		plan.ID = fmt.Sprintf("plan-%d", m.nextPlanID)
		m.nextPlanID++
	}
	plan.Status = StatusPending
	plan.CreatedAt = time.Now()
	plan.UpdatedAt = time.Now()

	m.plans[plan.ID] = plan
	log.Printf("[磁盘迁移] 创建计划: %s (%s)", plan.Name, plan.ID)
	return nil
}

// GetPlan 获取迁移计划
func (m *Manager) GetPlan(id string) *MigrationPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[id]
	if !ok {
		return nil
	}
	return plan
}

// ListPlans 列出所有迁移计划
func (m *Manager) ListPlans() []MigrationPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plans := make([]MigrationPlan, 0, len(m.plans))
	for _, p := range m.plans {
		plans = append(plans, *p)
	}
	return plans
}

// ValidatePlan 验证迁移计划
func (m *Manager) ValidatePlan(id string) ([]string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[id]
	if !ok {
		return nil, fmt.Errorf("plan %s not found", id)
	}

	var warnings []string

	// 检查源设备
	sourceFound := false
	for _, disk := range m.disks {
		if disk.DeviceName == plan.SourceDevice {
			sourceFound = true
			if !disk.SmartOK {
				warnings = append(warnings, fmt.Sprintf("源磁盘 %s SMART 状态异常", plan.SourceDevice))
			}
			if disk.Health < 80 {
				warnings = append(warnings, fmt.Sprintf("源磁盘 %s 健康度低: %.1f%%", plan.SourceDevice, disk.Health))
			}
			break
		}
	}
	if !sourceFound {
		warnings = append(warnings, fmt.Sprintf("源磁盘 %s 未找到", plan.SourceDevice))
	}

	// 检查目标设备
	targetFound := false
	for _, disk := range m.disks {
		if disk.DeviceName == plan.TargetDevice {
			targetFound = true
			if !disk.SmartOK {
				warnings = append(warnings, fmt.Sprintf("目标磁盘 %s SMART 状态异常", plan.TargetDevice))
			}
			if disk.Health < 90 {
				warnings = append(warnings, fmt.Sprintf("目标磁盘 %s 健康度: %.1f%%", plan.TargetDevice, disk.Health))
			}
			break
		}
	}
	if !targetFound {
		warnings = append(warnings, fmt.Sprintf("目标磁盘 %s 未找到", plan.TargetDevice))
	}

	// 检查容量
	if sourceFound && targetFound {
		var sourceSize, targetSize int64
		for _, disk := range m.disks {
			if disk.DeviceName == plan.SourceDevice {
				sourceSize = disk.SizeBytes
			}
			if disk.DeviceName == plan.TargetDevice {
				targetSize = disk.SizeBytes
			}
		}
		if targetSize < sourceSize {
			warnings = append(warnings, "目标磁盘容量小于源磁盘")
		}
	}

	return warnings, nil
}

// ========== 任务执行 ==========

// ExecutePlan 执行迁移计划
func (m *Manager) ExecutePlan(id string) (*MigrationJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[id]
	if !ok {
		return nil, fmt.Errorf("plan %s not found", id)
	}

	if plan.Status == StatusRunning {
		return nil, fmt.Errorf("plan %s is already running", id)
	}

	// 创建任务
	job := &MigrationJob{
		ID:         fmt.Sprintf("job-%d", m.nextJobID),
		PlanID:     id,
		Status:     StatusRunning,
		TotalSteps: len(plan.Steps),
		Progress:   0,
		StartedAt:  time.Now(),
	}
	m.nextJobID++

	// 更新计划状态
	plan.Status = StatusRunning
	plan.UpdatedAt = time.Now()

	// 初始化步骤状态
	for i := range plan.Steps {
		plan.Steps[i].Status = StepPending
		plan.Steps[i].Progress = 0
	}

	m.jobs[job.ID] = job
	log.Printf("[磁盘迁移] 执行计划: %s, 任务: %s", id, job.ID)
	return job, nil
}

// GetJobStatus 获取任务状态
func (m *Manager) GetJobStatus(jobID string) *MigrationJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return nil
	}
	return job
}

// PauseJob 暂停任务
func (m *Manager) PauseJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}

	if job.Status != StatusRunning {
		return fmt.Errorf("job %s is not running", jobID)
	}

	job.Status = StatusPaused

	// 更新计划状态
	if plan, ok := m.plans[job.PlanID]; ok {
		plan.Status = StatusPaused
		plan.UpdatedAt = time.Now()
	}

	log.Printf("[磁盘迁移] 暂停任务: %s", jobID)
	return nil
}

// ResumeJob 恢复任务
func (m *Manager) ResumeJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}

	if job.Status != StatusPaused {
		return fmt.Errorf("job %s is not paused", jobID)
	}

	job.Status = StatusRunning

	// 更新计划状态
	if plan, ok := m.plans[job.PlanID]; ok {
		plan.Status = StatusRunning
		plan.UpdatedAt = time.Now()
	}

	log.Printf("[磁盘迁移] 恢复任务: %s", jobID)
	return nil
}

// CancelJob 取消任务
func (m *Manager) CancelJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}

	if job.Status == StatusCompleted || job.Status == StatusCancelled {
		return fmt.Errorf("job %s is already %s", jobID, job.Status)
	}

	job.Status = StatusCancelled
	now := time.Now()
	job.FinishedAt = &now

	// 更新计划状态
	if plan, ok := m.plans[job.PlanID]; ok {
		plan.Status = StatusCancelled
		plan.UpdatedAt = time.Now()
	}

	log.Printf("[磁盘迁移] 取消任务: %s", jobID)
	return nil
}

// RollbackJob 回滚任务
func (m *Manager) RollbackJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}

	if job.Status != StatusFailed && job.Status != StatusCancelled {
		return fmt.Errorf("job %s cannot be rolled back (status: %s)", jobID, job.Status)
	}

	job.Status = StatusRolledBack

	// 更新计划状态
	if plan, ok := m.plans[job.PlanID]; ok {
		plan.Status = StatusRolledBack
		plan.UpdatedAt = time.Now()
	}

	log.Printf("[磁盘迁移] 回滚任务: %s", jobID)
	return nil
}

// ========== 热备盘管理 ==========

// AddHotSpare 添加热备盘
func (m *Manager) AddHotSpare(disk string, poolID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查磁盘是否存在
	found := false
	for _, d := range m.disks {
		if d.DeviceName == disk {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("disk %s not found", disk)
	}

	// 检查是否已经是热备盘
	if _, ok := m.hotSpares[disk]; ok {
		return fmt.Errorf("disk %s is already a hot spare", disk)
	}

	m.hotSpares[disk] = &HotSpare{
		DiskName: disk, Type: HotSpareTypeHot, PoolID: poolID, Status: "standby",
	}
	log.Printf("[磁盘迁移] 添加热备盘: %s -> 池 %s", disk, poolID)
	return nil
}

// RemoveHotSpare 移除热备盘
func (m *Manager) RemoveHotSpare(disk string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	hs, ok := m.hotSpares[disk]
	if !ok {
		return fmt.Errorf("disk %s is not a hot spare", disk)
	}

	if hs.Status == "in_use" {
		return fmt.Errorf("disk %s is currently in use", disk)
	}

	delete(m.hotSpares, disk)
	log.Printf("[磁盘迁移] 移除热备盘: %s", disk)
	return nil
}

// ListHotSpares 列出热备盘
func (m *Manager) ListHotSpares() []HotSpare {
	m.mu.RLock()
	defer m.mu.RUnlock()

	spares := make([]HotSpare, 0, len(m.hotSpares))
	for _, hs := range m.hotSpares {
		spares = append(spares, *hs)
	}
	return spares
}

// ========== 时间预估 ==========

// EstimateTime 预估迁移时间
func (m *Manager) EstimateTime(diskSize int64, migrateType MigrationType) (*TimeEstimate, error) {
	if diskSize <= 0 {
		return nil, fmt.Errorf("invalid disk size: %d", diskSize)
	}

	var transferRate int64
	var bottleneck string

	switch migrateType {
	case MigrationTypeDiskSwap:
		transferRate = 150 * 1024 * 1024 // 150 MB/s (SATA)
		bottleneck = "disk"
	case MigrationTypePoolMigrate:
		transferRate = 200 * 1024 * 1024 // 200 MB/s (SATA 优化)
		bottleneck = "disk"
	case MigrationTypeRAIDMigrate:
		transferRate = 100 * 1024 * 1024 // 100 MB/s (RAID 重建)
		bottleneck = "disk"
	case MigrationTypeNasToNas:
		transferRate = 100 * 1024 * 1024 // 100 MB/s (1GbE)
		bottleneck = "network"
	default:
		return nil, fmt.Errorf("unknown migration type: %s", migrateType)
	}

	estimatedSec := diskSize / transferRate
	estimatedTime := time.Duration(estimatedSec) * time.Second

	return &TimeEstimate{
		DiskSizeBytes: diskSize,
		EstimatedTime: estimatedTime,
		TransferRate:  transferRate,
		Bottleneck:    bottleneck,
	}, nil
}

// ========== 完整性验证 ==========

// VerifyIntegrity 验证数据完整性
func (m *Manager) VerifyIntegrity(jobID string) (*IntegrityCheck, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	_, ok := m.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job %s not found", jobID)
	}

	// 模拟完整性检查
	check := &IntegrityCheck{
		TotalFiles:   1000,
		PassedFiles:  998,
		FailedFiles:  2,
		Duration:     5 * time.Minute,
		CheckedBytes: 500 * 1024 * 1024 * 1024, // 500GB
		Errors: []string{
			"/data/photos/2023/IMG_001.jpg: checksum mismatch",
			"/data/documents/report.pdf: read error",
		},
	}
	return check, nil
}
