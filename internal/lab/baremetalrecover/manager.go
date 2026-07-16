// Package baremetalrecover 提供裸机恢复功能
// 系统备份镜像创建、启动恢复介质创建、恢复计划管理、备份验证、
// 增量备份支持、恢复执行和进度跟踪、备份加密、备份存储位置管理、自动备份调度
package baremetalrecover

import (
	"crypto/sha256"
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// BackupType 备份类型.
type BackupType string

const (
	BackupTypeFull        BackupType = "full"        // 全量备份
	BackupTypeIncremental BackupType = "incremental" // 增量备份
)

// MediaType 恢复介质类型.
type MediaType string

const (
	MediaTypeUSB MediaType = "usb" // USB 介质
	MediaTypeISO MediaType = "iso" // ISO 镜像
)

// LocationType 存储位置类型.
type LocationType string

const (
	LocationTypeLocal LocationType = "local" // 本地存储
	LocationTypeNAS   LocationType = "nas"   // 网络存储
	LocationTypeCloud LocationType = "cloud" // 云存储
)

// JobStatus 任务状态.
type JobStatus string

const (
	JobStatusPending   JobStatus = "pending"   // 等待中
	JobStatusRunning   JobStatus = "running"   // 运行中
	JobStatusCompleted JobStatus = "completed" // 已完成
	JobStatusFailed    JobStatus = "failed"    // 失败
	JobStatusCancelled JobStatus = "cancelled" // 已取消
)

// StepStatus 步骤状态.
type StepStatus string

const (
	StepStatusPending   StepStatus = "pending"   // 等待中
	StepStatusRunning   StepStatus = "running"   // 运行中
	StepStatusCompleted StepStatus = "completed" // 已完成
	StepStatusFailed    StepStatus = "failed"    // 失败
	StepStatusSkipped   StepStatus = "skipped"   // 已跳过
)

// BackupOptions 备份选项.
type BackupOptions struct {
	Type          BackupType `json:"type"`          // 备份类型
	ParentImageID string     `json:"parentImageId"` // 增量备份的父镜像ID
	Compress      bool       `json:"compress"`      // 是否压缩
	Encrypt       bool       `json:"encrypt"`       // 是否加密
	EncryptionKey string     `json:"encryptionKey"` // 加密密钥
	ExcludePaths  []string   `json:"excludePaths"`  // 排除路径
	LocationID    string     `json:"locationId"`    // 存储位置ID
	Verify        bool       `json:"verify"`        // 备份后验证
}

// BackupImage 备份镜像.
type BackupImage struct {
	ID            string     `json:"id"`
	Name          string     `json:"name"`
	Size          int64      `json:"size"`          // 字节
	Type          BackupType `json:"type"`          // 全量/增量
	ParentImageID string     `json:"parentImageId"` // 增量备份的父镜像ID
	SourceDevice  string     `json:"sourceDevice"`  // 源设备
	LocationID    string     `json:"locationId"`    // 存储位置ID
	Path          string     `json:"path"`          // 镜像路径
	Checksum      string     `json:"checksum"`      // SHA256校验和
	Encrypted     bool       `json:"encrypted"`     // 是否加密
	Verified      bool       `json:"verified"`      // 是否已验证
	CreatedAt     time.Time  `json:"createdAt"`
}

// RecoveryPlan 恢复计划.
type RecoveryPlan struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Description  string         `json:"description"`
	ImageIDs     []string       `json:"imageIds"`     // 镜像ID列表
	TargetDevice string         `json:"targetDevice"` // 目标设备
	Steps        []RecoveryStep `json:"steps"`        // 恢复步骤
	Status       JobStatus      `json:"status"`
	CreatedAt    time.Time      `json:"createdAt"`
}

// RecoveryStep 恢复步骤.
type RecoveryStep struct {
	Sequence    int        `json:"sequence"`    // 序号
	Type        string     `json:"type"`        // 步骤类型
	Description string     `json:"description"` // 描述
	Status      StepStatus `json:"status"`      // 状态
	Progress    int        `json:"progress"`    // 进度 (0-100)
	ErrorMsg    string     `json:"errorMsg"`    // 错误信息
}

// RecoveryMedia 恢复介质.
type RecoveryMedia struct {
	ID        string    `json:"id"`
	Type      MediaType `json:"type"`  // USB/ISO
	Path      string    `json:"path"`  // 介质路径
	Label     string    `json:"label"` // 介质标签
	Size      int64     `json:"size"`  // 字节
	CreatedAt time.Time `json:"createdAt"`
}

// BackupSchedule 备份调度.
type BackupSchedule struct {
	ID          string    `json:"id"`
	PlanID      string    `json:"planId"` // 关联计划ID
	Name        string    `json:"name"`
	Frequency   string    `json:"frequency"`   // 调度频率: daily, weekly, monthly
	RetainCount int       `json:"retainCount"` // 保留份数
	NextRunTime time.Time `json:"nextRunTime"` // 下次执行时间
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"createdAt"`
}

// BackupLocation 备份存储位置.
type BackupLocation struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Type      LocationType `json:"type"`      // local/nas/cloud
	Path      string       `json:"path"`      // 存储路径
	Capacity  int64        `json:"capacity"`  // 总容量（字节）
	Used      int64        `json:"used"`      // 已用容量（字节）
	Available int64        `json:"available"` // 可用容量（字节）
	Enabled   bool         `json:"enabled"`
	CreatedAt time.Time    `json:"createdAt"`
}

// RestoreJob 恢复任务.
type RestoreJob struct {
	ID          string     `json:"id"`
	PlanID      string     `json:"planId"`      // 关联计划ID
	Status      JobStatus  `json:"status"`      // 任务状态
	Progress    int        `json:"progress"`    // 总进度 (0-100)
	CurrentStep int        `json:"currentStep"` // 当前步骤序号
	ErrorMsg    string     `json:"errorMsg"`    // 错误信息
	StartTime   time.Time  `json:"startTime"`   // 开始时间
	EndTime     *time.Time `json:"endTime"`     // 结束时间
}

// ========== Manager ==========

// Manager 裸机恢复管理器.
type Manager struct {
	mu        sync.RWMutex
	images    map[string]*BackupImage
	plans     map[string]*RecoveryPlan
	media     map[string]*RecoveryMedia
	schedules map[string]*BackupSchedule
	locations map[string]*BackupLocation
	jobs      map[string]*RestoreJob
	imageSeq  int
	mediaSeq  int
	jobSeq    int
	locSeq    int
	schedSeq  int
	planSeq   int
}

// NewManager 创建管理器.
func NewManager() *Manager {
	return &Manager{
		images:    make(map[string]*BackupImage),
		plans:     make(map[string]*RecoveryPlan),
		media:     make(map[string]*RecoveryMedia),
		schedules: make(map[string]*BackupSchedule),
		locations: make(map[string]*BackupLocation),
		jobs:      make(map[string]*RestoreJob),
	}
}

// ========== 镜像管理 ==========

// CreateImage 创建备份镜像.
func (m *Manager) CreateImage(device string, name string, opts *BackupOptions) (*BackupImage, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if device == "" {
		return nil, fmt.Errorf("device is required")
	}
	if name == "" {
		return nil, fmt.Errorf("name is required")
	}

	m.imageSeq++
	id := fmt.Sprintf("img-%d", m.imageSeq)

	backupType := BackupTypeFull
	parentID := ""
	encrypted := false
	encryptionKey := ""
	compress := false
	locationID := ""

	if opts != nil {
		backupType = opts.Type
		parentID = opts.ParentImageID
		encrypted = opts.Encrypt
		encryptionKey = opts.EncryptionKey
		compress = opts.Compress
		locationID = opts.LocationID

		// 增量备份需要父镜像
		if backupType == BackupTypeIncremental && parentID == "" {
			return nil, fmt.Errorf("parent image ID required for incremental backup")
		}
		if backupType == BackupTypeIncremental {
			if _, ok := m.images[parentID]; !ok {
				return nil, fmt.Errorf("parent image %s not found", parentID)
			}
		}
	}

	// 模拟镜像大小
	size := int64(1024 * 1024 * 512) // 512MB 模拟
	if compress {
		size = size * 70 / 100 // 压缩后约70%
	}

	// 生成校验和
	checksumData := fmt.Sprintf("%s-%s-%d-%s", id, name, size, device)
	checksum := fmt.Sprintf("%x", sha256.Sum256([]byte(checksumData)))

	// 加密标记
	if encrypted && encryptionKey != "" {
		size += 1024 * 16 // 加密头额外16KB
	}

	img := &BackupImage{
		ID:            id,
		Name:          name,
		Size:          size,
		Type:          backupType,
		ParentImageID: parentID,
		SourceDevice:  device,
		LocationID:    locationID,
		Path:          fmt.Sprintf("/backup/images/%s.img", id),
		Checksum:      checksum,
		Encrypted:     encrypted,
		Verified:      false,
		CreatedAt:     time.Now(),
	}

	m.images[id] = img
	log.Printf("[裸机恢复] 创建镜像: %s (%s)", id, name)
	return img, nil
}

// ListImages 列出所有镜像.
func (m *Manager) ListImages() []BackupImage {
	m.mu.RLock()
	defer m.mu.RUnlock()

	images := make([]BackupImage, 0, len(m.images))
	for _, img := range m.images {
		images = append(images, *img)
	}
	return images
}

// DeleteImage 删除镜像.
func (m *Manager) DeleteImage(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	img, ok := m.images[id]
	if !ok {
		return fmt.Errorf("image %s not found", id)
	}

	// 检查是否有其他镜像依赖此镜像（作为父镜像）
	for _, other := range m.images {
		if other.ParentImageID == id {
			return fmt.Errorf("image %s is parent of %s, cannot delete", id, other.ID)
		}
	}

	delete(m.images, id)
	log.Printf("[裸机恢复] 删除镜像: %s (%s)", id, img.Name)
	return nil
}

// VerifyImage 验证镜像.
func (m *Manager) VerifyImage(id string) (bool, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	img, ok := m.images[id]
	if !ok {
		return false, fmt.Errorf("image %s not found", id)
	}

	// 模拟验证：重新计算校验和
	checksumData := fmt.Sprintf("%s-%s-%d-%s", img.ID, img.Name, img.Size, img.SourceDevice)
	expectedChecksum := fmt.Sprintf("%x", sha256.Sum256([]byte(checksumData)))

	valid := img.Checksum == expectedChecksum
	img.Verified = valid

	if valid {
		log.Printf("[裸机恢复] 镜像验证成功: %s", id)
	} else {
		log.Printf("[裸机恢复] 镜像验证失败: %s", id)
	}

	return valid, nil
}

// ========== 恢复介质 ==========

// CreateRecoveryMedia 创建恢复介质.
func (m *Manager) CreateRecoveryMedia(mediaType string, path string) (*RecoveryMedia, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if path == "" {
		return nil, fmt.Errorf("path is required")
	}

	mt := MediaType(mediaType)
	if mt != MediaTypeUSB && mt != MediaTypeISO {
		return nil, fmt.Errorf("unsupported media type: %s (use 'usb' or 'iso')", mediaType)
	}

	m.mediaSeq++
	id := fmt.Sprintf("media-%d", m.mediaSeq)

	label := "USB恢复盘"
	size := int64(1024 * 1024 * 200) // 200MB
	if mt == MediaTypeISO {
		label = "恢复光盘"
		size = int64(1024 * 1024 * 150) // 150MB
	}

	media := &RecoveryMedia{
		ID:        id,
		Type:      mt,
		Path:      path,
		Label:     label,
		Size:      size,
		CreatedAt: time.Now(),
	}

	m.media[id] = media
	log.Printf("[裸机恢复] 创建恢复介质: %s (%s)", id, label)
	return media, nil
}

// ========== 恢复计划 ==========

// CreatePlan 创建恢复计划.
func (m *Manager) CreatePlan(plan *RecoveryPlan) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if plan == nil {
		return fmt.Errorf("plan is required")
	}
	if plan.Name == "" {
		return fmt.Errorf("plan name is required")
	}

	m.planSeq++
	if plan.ID == "" {
		plan.ID = fmt.Sprintf("plan-%d", m.planSeq)
	}

	// 验证镜像存在
	for _, imgID := range plan.ImageIDs {
		if _, ok := m.images[imgID]; !ok {
			return fmt.Errorf("image %s not found", imgID)
		}
	}

	plan.Status = JobStatusPending
	plan.CreatedAt = time.Now()

	m.plans[plan.ID] = plan
	log.Printf("[裸机恢复] 创建恢复计划: %s (%s)", plan.ID, plan.Name)
	return nil
}

// ExecutePlan 执行恢复计划.
func (m *Manager) ExecutePlan(planID string, targetDevice string) (*RestoreJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan %s not found", planID)
	}

	if targetDevice == "" {
		return nil, fmt.Errorf("target device is required")
	}

	m.jobSeq++
	jobID := fmt.Sprintf("job-%d", m.jobSeq)

	// 初始化步骤状态
	for i := range plan.Steps {
		plan.Steps[i].Status = StepStatusPending
		plan.Steps[i].Progress = 0
	}

	job := &RestoreJob{
		ID:          jobID,
		PlanID:      planID,
		Status:      JobStatusRunning,
		Progress:    0,
		CurrentStep: 0,
		StartTime:   time.Now(),
	}

	plan.Status = JobStatusRunning
	plan.TargetDevice = targetDevice

	m.jobs[jobID] = job
	log.Printf("[裸机恢复] 执行恢复计划: %s -> %s (任务: %s)", planID, targetDevice, jobID)
	return job, nil
}

// GetJobStatus 获取任务状态.
func (m *Manager) GetJobStatus(jobID string) *RestoreJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return nil
	}
	return job
}

// ListJobs 列出所有任务.
func (m *Manager) ListJobs() []RestoreJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]RestoreJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, *j)
	}
	return jobs
}

// CancelJob 取消任务.
func (m *Manager) CancelJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("job %s not found", jobID)
	}

	if job.Status != JobStatusRunning && job.Status != JobStatusPending {
		return fmt.Errorf("job %s cannot be cancelled (status: %s)", jobID, job.Status)
	}

	job.Status = JobStatusCancelled
	now := time.Now()
	job.EndTime = &now

	// 更新关联计划状态
	if plan, ok := m.plans[job.PlanID]; ok {
		plan.Status = JobStatusCancelled
	}

	log.Printf("[裸机恢复] 取消任务: %s", jobID)
	return nil
}

// ========== 存储位置管理 ==========

// AddLocation 添加存储位置.
func (m *Manager) AddLocation(loc *BackupLocation) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if loc == nil {
		return fmt.Errorf("location is required")
	}
	if loc.Name == "" {
		return fmt.Errorf("location name is required")
	}
	if loc.Path == "" {
		return fmt.Errorf("location path is required")
	}

	lt := loc.Type
	if lt != LocationTypeLocal && lt != LocationTypeNAS && lt != LocationTypeCloud {
		return fmt.Errorf("unsupported location type: %s", lt)
	}

	m.locSeq++
	if loc.ID == "" {
		loc.ID = fmt.Sprintf("loc-%d", m.locSeq)
	}

	if loc.Capacity > 0 {
		loc.Available = loc.Capacity - loc.Used
	}

	loc.Enabled = true
	loc.CreatedAt = time.Now()

	m.locations[loc.ID] = loc
	log.Printf("[裸机恢复] 添加存储位置: %s (%s)", loc.ID, loc.Name)
	return nil
}

// ListLocations 列出所有存储位置.
func (m *Manager) ListLocations() []BackupLocation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	locs := make([]BackupLocation, 0, len(m.locations))
	for _, loc := range m.locations {
		locs = append(locs, *loc)
	}
	return locs
}

// ========== 调度管理 ==========

// SetSchedule 设置备份调度.
func (m *Manager) SetSchedule(planID string, sched *BackupSchedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.plans[planID]; !ok {
		return fmt.Errorf("plan %s not found", planID)
	}
	if sched == nil {
		return fmt.Errorf("schedule is required")
	}

	m.schedSeq++
	if sched.ID == "" {
		sched.ID = fmt.Sprintf("sched-%d", m.schedSeq)
	}

	sched.PlanID = planID
	sched.Enabled = true
	sched.CreatedAt = time.Now()

	if sched.NextRunTime.IsZero() {
		sched.NextRunTime = m.calculateNextRun(sched.Frequency)
	}

	m.schedules[sched.ID] = sched
	log.Printf("[裸机恢复] 设置备份调度: %s (计划: %s, 频率: %s)", sched.ID, planID, sched.Frequency)
	return nil
}

// GetSchedule 获取计划的调度.
func (m *Manager) GetSchedule(planID string) *BackupSchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	for _, sched := range m.schedules {
		if sched.PlanID == planID {
			return sched
		}
	}
	return nil
}

// calculateNextRun 计算下次执行时间.
func (m *Manager) calculateNextRun(frequency string) time.Time {
	now := time.Now()
	switch frequency {
	case "daily":
		return now.Add(24 * time.Hour)
	case "weekly":
		return now.Add(7 * 24 * time.Hour)
	case "monthly":
		return now.AddDate(0, 1, 0)
	default:
		return now.Add(24 * time.Hour)
	}
}
