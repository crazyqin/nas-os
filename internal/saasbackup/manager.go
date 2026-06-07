package saasbackup

import (
	"crypto/sha256"
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 错误定义 ==========

var (
	// ErrTenantNotFound 租户未找到.
	ErrTenantNotFound = errors.New("租户未找到")
	// ErrJobNotFound 任务未找到.
	ErrJobNotFound = errors.New("备份任务未找到")
	// ErrItemNotFound 备份项未找到.
	ErrItemNotFound = errors.New("备份项未找到")
	// ErrTenantNotConnected 租户未连接.
	ErrTenantNotConnected = errors.New("租户未连接")
	// ErrJobAlreadyRunning 任务已在运行中.
	ErrJobAlreadyRunning = errors.New("任务已在运行中")
	// ErrInvalidRestoreMode 无效的恢复模式.
	ErrInvalidRestoreMode = errors.New("无效的恢复模式")
	// ErrCrossUserRequiresTarget 跨用户恢复需要指定目标用户.
	ErrCrossUserRequiresTarget = errors.New("跨用户恢复需要指定目标用户")
)

// ========== 管理器 ==========

// Manager SaaS 备份核心管理器，管理租户、备份任务和备份项.
type Manager struct {
	mu      sync.RWMutex
	tenants map[string]*SaaSTenant // tenantID -> SaaSTenant
	jobs    map[string]*BackupJob  // jobID -> BackupJob
	items   map[string]*BackupItem // itemID -> BackupItem
}

// NewManager 创建 SaaS 备份管理器.
func NewManager() *Manager {
	return &Manager{
		tenants: make(map[string]*SaaSTenant),
		jobs:    make(map[string]*BackupJob),
		items:   make(map[string]*BackupItem),
	}
}

// ========== 租户管理 ==========

// ConnectTenant 连接 SaaS 租户.
func (m *Manager) ConnectTenant(req ConnectTenantRequest) (*SaaSTenant, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenant := &SaaSTenant{
		ID:          uuid.New().String(),
		Provider:    req.Provider,
		Domain:      req.Domain,
		AdminEmail:  req.AdminEmail,
		ConnectedAt: time.Now(),
		Status:      TenantStatusConnected,
	}

	m.tenants[tenant.ID] = tenant
	return tenant, nil
}

// ListTenants 列出所有租户.
func (m *Manager) ListTenants() []*SaaSTenant {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tenants := make([]*SaaSTenant, 0, len(m.tenants))
	for _, t := range m.tenants {
		tenants = append(tenants, t)
	}
	return tenants
}

// DisconnectTenant 断开 SaaS 租户连接.
func (m *Manager) DisconnectTenant(tenantID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	tenant, ok := m.tenants[tenantID]
	if !ok {
		return ErrTenantNotFound
	}

	tenant.Status = TenantStatusDisconnected
	return nil
}

// ========== 备份任务管理 ==========

// CreateJob 创建备份任务.
func (m *Manager) CreateJob(req CreateJobRequest) (*BackupJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证租户存在且已连接
	tenant, ok := m.tenants[req.TenantID]
	if !ok {
		return nil, ErrTenantNotFound
	}
	if tenant.Status != TenantStatusConnected {
		return nil, ErrTenantNotConnected
	}

	// 设置默认保留天数
	retentionDays := req.RetentionDays
	if retentionDays <= 0 {
		retentionDays = 30
	}

	job := &BackupJob{
		ID:            uuid.New().String(),
		Provider:      tenant.Provider,
		TenantID:      req.TenantID,
		UserID:        req.UserID,
		ResourceType:  req.ResourceType,
		Schedule:      req.Schedule,
		Status:        JobStatusIdle,
		RetentionDays: retentionDays,
	}

	m.jobs[job.ID] = job
	return job, nil
}

// ListJobs 列出所有备份任务.
func (m *Manager) ListJobs() []*BackupJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*BackupJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobs = append(jobs, j)
	}
	return jobs
}

// ExecuteBackup 执行备份任务.
func (m *Manager) ExecuteBackup(jobID string) (*BackupJob, error) {
	m.mu.Lock()

	job, ok := m.jobs[jobID]
	if !ok {
		m.mu.Unlock()
		return nil, ErrJobNotFound
	}

	if job.Status == JobStatusRunning {
		m.mu.Unlock()
		return nil, ErrJobAlreadyRunning
	}

	// 验证租户仍然连接
	tenant, ok := m.tenants[job.TenantID]
	if !ok {
		m.mu.Unlock()
		return nil, ErrTenantNotFound
	}
	if tenant.Status != TenantStatusConnected {
		m.mu.Unlock()
		return nil, ErrTenantNotConnected
	}

	// 标记为运行中
	job.Status = JobStatusRunning
	m.mu.Unlock()

	// 模拟备份过程：生成模拟数据
	items := m.simulateBackup(job)

	m.mu.Lock()
	defer m.mu.Unlock()

	// 更新任务状态
	now := time.Now()
	job.LastRun = &now
	job.Status = JobStatusCompleted

	// 统计备份项
	totalSize := int64(0)
	for _, item := range items {
		m.items[item.ID] = item
		totalSize += item.SizeBytes
	}
	job.ItemCount += len(items)
	job.SizeBytes += totalSize

	return job, nil
}

// simulateBackup 模拟备份过程，生成模拟备份项.
func (m *Manager) simulateBackup(job *BackupJob) []*BackupItem {
	var items []*BackupItem

	// 根据资源类型生成不同数量的模拟项
	count := 0
	switch job.ResourceType {
	case ResourceMail:
		count = 5
	case ResourceDrive:
		count = 3
	case ResourceContacts:
		count = 10
	case ResourceCalendar:
		count = 2
	}

	for i := 0; i < count; i++ {
		itemID := uuid.New().String()
		sourcePath := fmt.Sprintf("/%s/%s/%s/item_%d", job.Provider, job.TenantID, job.ResourceType, i)
		backupPath := fmt.Sprintf("/backups/%s/%s", job.ID, itemID)
		sizeBytes := int64(1024 * (i + 1)) // 模拟不同大小

		// 生成校验和
		checksum := sha256.Sum256([]byte(fmt.Sprintf("%s-%s-%d", job.ID, itemID, i)))

		item := &BackupItem{
			ID:         itemID,
			JobID:      job.ID,
			SourcePath: sourcePath,
			BackupPath: backupPath,
			SizeBytes:  sizeBytes,
			Checksum:   fmt.Sprintf("%x", checksum),
			CreatedAt:  time.Now(),
			ItemType:   string(job.ResourceType),
		}

		items = append(items, item)
	}

	return items
}

// ========== 数据恢复 ==========

// RestoreData 恢复备份数据.
func (m *Manager) RestoreData(req RestoreRequest) ([]*BackupItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 验证任务存在
	job, ok := m.jobs[req.JobID]
	if !ok {
		return nil, ErrJobNotFound
	}

	// 验证恢复模式
	switch req.RestoreMode {
	case RestoreModeOriginal:
		// 原始位置恢复，使用任务的 UserID
	case RestoreModeCrossUser:
		// 跨用户恢复需要指定目标用户
		if req.TargetUserID == "" {
			return nil, ErrCrossUserRequiresTarget
		}
	default:
		return nil, ErrInvalidRestoreMode
	}

	// 收集要恢复的项
	var restoredItems []*BackupItem
	for _, itemID := range req.ItemIDs {
		item, ok := m.items[itemID]
		if !ok {
			continue
		}
		// 验证项属于该任务
		if item.JobID != req.JobID {
			continue
		}
		restoredItems = append(restoredItems, item)
	}

	if len(restoredItems) == 0 {
		return nil, ErrItemNotFound
	}

	// 确定恢复目标用户
	targetUser := job.UserID
	if req.RestoreMode == RestoreModeCrossUser {
		targetUser = req.TargetUserID
	}

	// 模拟恢复：在实际实现中这里会调用 SaaS API
	_ = targetUser

	return restoredItems, nil
}

// ========== 统计和查询 ==========

// GetStats 获取备份统计信息.
func (m *Manager) GetStats() *BackupStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &BackupStats{
		TotalJobs: len(m.jobs),
	}

	var totalSize int64
	successCount := 0
	var lastBackup *time.Time

	for _, job := range m.jobs {
		stats.TotalItems += job.ItemCount
		totalSize += job.SizeBytes

		if job.Status == JobStatusCompleted {
			successCount++
		}

		if job.LastRun != nil {
			if lastBackup == nil || job.LastRun.After(*lastBackup) {
				lastBackup = job.LastRun
			}
		}
	}

	stats.TotalSize = totalSize
	stats.LastBackupTime = lastBackup

	// 计算成功率
	if stats.TotalJobs > 0 {
		stats.SuccessRate = float64(successCount) / float64(stats.TotalJobs) * 100
	}

	return stats
}

// ListItems 列出指定任务的所有备份项.
func (m *Manager) ListItems(jobID string) ([]*BackupItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 验证任务存在
	if _, ok := m.jobs[jobID]; !ok {
		return nil, ErrJobNotFound
	}

	var items []*BackupItem
	for _, item := range m.items {
		if item.JobID == jobID {
			items = append(items, item)
		}
	}
	return items, nil
}
