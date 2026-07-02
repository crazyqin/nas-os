// Package drivemigration 提供硬盘迁移功能
// 支持在不中断服务的情况下将数据从旧盘迁移到新盘
// 参考群晖的硬盘迁移和TrueNAS的磁盘替换功能
package drivemigration

import (
	"encoding/json"
	"errors"
	"fmt"
	"sync"
	"time"
)

// 迁移类型.
const (
	MigrationTypeReplace = "replace" // 替换：旧盘换新盘
	MigrationTypeExpand  = "expand"  // 扩容：添加新盘扩展存储池
	MigrationTypeMigrate = "migrate" // 迁移：跨存储池迁移
	MigrationTypeShrink  = "shrink"  // 缩容：移除磁盘
)

// 迁移状态.
const (
	StatusPending    = "pending"    // 等待中
	StatusPreparing  = "preparing"  // 准备中
	StatusSyncing    = "syncing"    // 同步中
	StatusVerifying  = "verifying"  // 验证中
	StatusCompleting = "completing" // 完成中
	StatusCompleted  = "completed"  // 已完成
	StatusFailed     = "failed"     // 失败
	StatusCancelled  = "cancelled"  // 已取消
)

// RAID类型.
const (
	RAIDTypeBasic  = "basic"  // Basic/单盘
	RAIDTypeRAID0  = "raid0"  // RAID 0
	RAIDTypeRAID1  = "raid1"  // RAID 1
	RAIDTypeRAID5  = "raid5"  // RAID 5
	RAIDTypeRAID6  = "raid6"  // RAID 6
	RAIDTypeRAID10 = "raid10" // RAID 10
	RAIDTypeSHR    = "shr"    // Synology Hybrid RAID
	RAIDTypeRAIDZ1 = "raidz1" // RAIDZ1
	RAIDTypeRAIDZ2 = "raidz2" // RAIDZ2
	RAIDTypeRAIDZ3 = "raidz3" // RAIDZ3
)

var (
	ErrDiskNotFound      = errors.New("磁盘未找到")
	ErrPoolNotFound      = errors.New("存储池未找到")
	ErrMigrationExists   = errors.New("迁移任务已存在")
	ErrDiskInUse         = errors.New("磁盘正在使用中")
	ErrInsufficientSpace = errors.New("空间不足")
	ErrInvalidRAIDType   = errors.New("不支持的RAID类型")
	ErrMigrationRunning  = errors.New("迁移正在进行中")
)

// Disk 磁盘信息.
type Disk struct {
	ID          string    `json:"id"`          // 磁盘ID
	Device      string    `json:"device"`      // 设备路径（如/dev/sda）
	Model       string    `json:"model"`       // 型号
	Serial      string    `json:"serial"`      // 序列号
	Size        int64     `json:"size"`        // 容量（字节）
	UsedSize    int64     `json:"used_size"`   // 已使用容量
	Health      string    `json:"health"`      // 健康状态
	Temperature int       `json:"temperature"` // 温度
	RPM         int       `json:"rpm"`         // 转速（SSD为0）
	Interface   string    `json:"interface"`   // 接口（SATA/SAS/NVMe）
	PoolID      string    `json:"pool_id"`     // 所属存储池
	SlotIndex   int       `json:"slot_index"`  // 槽位
	IsSpare     bool      `json:"is_spare"`    // 是否热备盘
	IsSSD       bool      `json:"is_ssd"`      // 是否SSD
	WearLevel   float64   `json:"wear_level"`  // 磨损程度（SSD）
	CreatedAt   time.Time `json:"created_at"`
}

// StoragePool 存储池.
type StoragePool struct {
	ID          string    `json:"id"`           // 池ID
	Name        string    `json:"name"`         // 池名称
	RAIDType    string    `json:"raid_type"`    // RAID类型
	TotalSize   int64     `json:"total_size"`   // 总容量
	UsedSize    int64     `json:"used_size"`    // 已用容量
	AvailSize   int64     `json:"avail_size"`   // 可用容量
	Disks       []string  `json:"disks"`        // 磁盘ID列表
	Status      string    `json:"status"`       // 状态
	Degraded    bool      `json:"degraded"`     // 是否降级
	Rebuilding  bool      `json:"rebuilding"`   // 是否重建中
	ScrubStatus string    `json:"scrub_status"` // Scrub状态
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// MigrationTask 迁移任务.
type MigrationTask struct {
	ID             string    `json:"id"`              // 任务ID
	Type           string    `json:"type"`            // 迁移类型
	Status         string    `json:"status"`          // 任务状态
	SourcePoolID   string    `json:"source_pool_id"`  // 源存储池
	TargetPoolID   string    `json:"target_pool_id"`  // 目标存储池
	SourceDiskID   string    `json:"source_disk_id"`  // 源磁盘
	TargetDiskID   string    `json:"target_disk_id"`  // 目标磁盘
	Progress       float64   `json:"progress"`        // 进度百分比
	BytesTotal     int64     `json:"bytes_total"`     // 总字节数
	BytesCopied    int64     `json:"bytes_copied"`    // 已复制字节数
	SpeedMBps      float64   `json:"speed_mbps"`      // 当前速度（MB/s）
	ETASeconds     int64     `json:"eta_seconds"`     // 预计剩余秒数
	StartedAt      time.Time `json:"started_at"`      // 开始时间
	CompletedAt    time.Time `json:"completed_at"`    // 完成时间
	ErrorMessage   string    `json:"error_message"`   // 错误信息
	VerificationOK bool      `json:"verification_ok"` // 验证通过
	CreatedAt      time.Time `json:"created_at"`
}

// MigrationManager 磁盘迁移管理器.
type MigrationManager struct {
	mu          sync.RWMutex
	disks       map[string]*Disk
	pools       map[string]*StoragePool
	tasks       map[string]*MigrationTask
	taskCounter int64
}

// NewMigrationManager 创建迁移管理器.
func NewMigrationManager() *MigrationManager {
	return &MigrationManager{
		disks: make(map[string]*Disk),
		pools: make(map[string]*StoragePool),
		tasks: make(map[string]*MigrationTask),
	}
}

// RegisterDisk 注册磁盘.
func (m *MigrationManager) RegisterDisk(disk *Disk) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if disk.ID == "" {
		disk.ID = fmt.Sprintf("disk-%s", disk.Serial)
	}
	disk.CreatedAt = time.Now()
	m.disks[disk.ID] = disk
	return nil
}

// CreatePool 创建存储池.
func (m *MigrationManager) CreatePool(pool *StoragePool) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if pool.ID == "" {
		pool.ID = fmt.Sprintf("pool-%s", pool.Name)
	}
	pool.CreatedAt = time.Now()
	pool.UpdatedAt = time.Now()
	pool.Status = "healthy"
	m.pools[pool.ID] = pool
	return nil
}

// StartMigration 启动迁移任务.
func (m *MigrationManager) StartMigration(task *MigrationTask) (*MigrationTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查磁盘是否存在
	if task.SourceDiskID != "" {
		if _, ok := m.disks[task.SourceDiskID]; !ok {
			return nil, ErrDiskNotFound
		}
	}
	if task.TargetDiskID != "" {
		if _, ok := m.disks[task.TargetDiskID]; !ok {
			return nil, ErrDiskNotFound
		}
	}

	// 检查存储池
	if task.SourcePoolID != "" {
		pool, ok := m.pools[task.SourcePoolID]
		if !ok {
			return nil, ErrPoolNotFound
		}
		// 检查是否已有迁移任务
		for _, t := range m.tasks {
			if t.SourcePoolID == task.SourcePoolID && (t.Status == StatusPending || t.Status == StatusSyncing) {
				return nil, ErrMigrationRunning
			}
		}
		_ = pool
	}

	m.taskCounter++
	if task.ID == "" {
		task.ID = fmt.Sprintf("mig-%d", m.taskCounter)
	}
	task.Status = StatusPending
	task.CreatedAt = time.Now()
	m.tasks[task.ID] = task
	return task, nil
}

// UpdateProgress 更新迁移进度.
func (m *MigrationManager) UpdateProgress(taskID string, progress float64, bytesCopied int64, speedMBps float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return ErrPoolNotFound // reuse error
	}

	task.Progress = progress
	task.BytesCopied = bytesCopied
	task.SpeedMBps = speedMBps
	if progress >= 100 {
		task.Status = StatusCompleted
		task.CompletedAt = time.Now()
	} else if progress > 0 {
		task.Status = StatusSyncing
		if task.StartedAt.IsZero() {
			task.StartedAt = time.Now()
		}
	}
	if speedMBps > 0 && task.BytesTotal > 0 {
		remaining := float64(task.BytesTotal-bytesCopied) / (speedMBps * 1024 * 1024)
		task.ETASeconds = int64(remaining)
	}
	return nil
}

// GetMigration 获取迁移任务详情.
func (m *MigrationManager) GetMigration(taskID string) (*MigrationTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("task %s not found", taskID)
	}
	return task, nil
}

// ListMigrations 列出所有迁移任务.
func (m *MigrationManager) ListMigrations() []*MigrationTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*MigrationTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		list = append(list, t)
	}
	return list
}

// ListDisks 列出所有磁盘.
func (m *MigrationManager) ListDisks() []*Disk {
	m.mu.RLock()
	defer m.mu.RUnlock()
	list := make([]*Disk, 0, len(m.disks))
	for _, d := range m.disks {
		list = append(list, d)
	}
	return list
}

// GetPool 获取存储池信息.
func (m *MigrationManager) GetPool(poolID string) (*StoragePool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	pool, ok := m.pools[poolID]
	if !ok {
		return nil, ErrPoolNotFound
	}
	return pool, nil
}

// ExportReport 导出迁移报告.
func (m *MigrationManager) ExportReport() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	report := map[string]interface{}{
		"disks":       m.disks,
		"pools":       m.pools,
		"migrations":  m.tasks,
		"exported_at": time.Now(),
	}
	return json.MarshalIndent(report, "", "  ")
}
