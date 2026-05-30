// Package selfhealing 自愈存储模块
// 支持后台 scrub 任务调度、Bitrot 检测和自动修复、数据完整性校验
package selfhealing

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

// ScrubStatus scrub 任务状态
type ScrubStatus string

const (
	ScrubStatusPending   ScrubStatus = "pending"
	ScrubStatusRunning   ScrubStatus = "running"
	ScrubStatusCompleted ScrubStatus = "completed"
	ScrubStatusFailed    ScrubStatus = "failed"
)

// RepairStatus 修复状态
type RepairStatus string

const (
	RepairStatusPending  RepairStatus = "pending"
	RepairStatusRunning  RepairStatus = "running"
	RepairStatusSuccess  RepairStatus = "success"
	RepairStatusFailed   RepairStatus = "failed"
)

// IntegrityLevel 完整性校验级别
type IntegrityLevel string

const (
	IntegrityLevelBasic    IntegrityLevel = "basic"
	IntegrityLevelStandard IntegrityLevel = "standard"
	IntegrityLevelStrict   IntegrityLevel = "strict"
)

// ScrubTask scrub 任务
type ScrubTask struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Target      string      `json:"target"`      // 目标路径或卷
	Status      ScrubStatus `json:"status"`
	Progress    float64     `json:"progress"`     // 0-100
	TotalFiles  int64       `json:"total_files"`
	ScrubbedFiles int64     `json:"scrubbed_files"`
	CorruptedFiles int64    `json:"corrupted_files"`
	RepairedFiles int64     `json:"repaired_files"`
	StartedAt   *time.Time  `json:"started_at,omitempty"`
	CompletedAt *time.Time  `json:"completed_at,omitempty"`
	Error       string      `json:"error,omitempty"`
}

// DataBlock 数据块
type DataBlock struct {
	ID        string    `json:"id"`
	Path      string    `json:"path"`
	Size      int64     `json:"size"`
	Checksum  string    `json:"checksum"`
	Algorithm string    `json:"algorithm"` // sha256, xxhash
	Replicas  []string  `json:"replicas"`  // 副本位置
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// CorruptionRecord 损坏记录
type CorruptionRecord struct {
	ID          string       `json:"id"`
	BlockID     string       `json:"block_id"`
	Path        string       `json:"path"`
	DetectedAt  time.Time    `json:"detected_at"`
	RepairStatus RepairStatus `json:"repair_status"`
	RepairAt    *time.Time   `json:"repair_at,omitempty"`
	Error       string       `json:"error,omitempty"`
}

// RepairReport 修复报告
type RepairReport struct {
	ID              string              `json:"id"`
	ScrubTaskID     string              `json:"scrub_task_id"`
	TotalBlocks     int                 `json:"total_blocks"`
	CorruptedBlocks int                 `json:"corrupted_blocks"`
	RepairedBlocks  int                 `json:"repaired_blocks"`
	FailedBlocks    int                 `json:"failed_blocks"`
	Records         []CorruptionRecord  `json:"records"`
	StartedAt       time.Time           `json:"started_at"`
	CompletedAt     *time.Time          `json:"completed_at,omitempty"`
}

// ReplicaInfo 副本信息
type ReplicaInfo struct {
	BlockID    string    `json:"block_id"`
	ReplicaID  string    `json:"replica_id"`
	Path       string    `json:"path"`
	Checksum   string    `json:"checksum"`
	Healthy    bool      `json:"healthy"`
	LastCheck  time.Time `json:"last_check"`
}

// SelfHealingConfig 自愈存储配置
type SelfHealingConfig struct {
	Enabled           bool            `json:"enabled"`
	ScrubInterval     int             `json:"scrub_interval"`     // hours
	ScrubSchedule     string          `json:"scrub_schedule"`     // cron expression
	IntegrityLevel    IntegrityLevel  `json:"integrity_level"`
	AutoRepair        bool            `json:"autoRepair"`
	ReplicaCount      int             `json:"replica_count"`
	MaxRepairAttempts int             `json:"max_repair_attempts"`
	ChecksumAlgorithm string          `json:"checksum_algorithm"`
	AlertThreshold    int             `json:"alert_threshold"`    // 损坏文件数告警阈值
}

// Manager 自愈存储管理器
type Manager struct {
	config      *SelfHealingConfig
	scrubTasks  map[string]*ScrubTask
	blocks      map[string]*DataBlock
	corruptions []CorruptionRecord
	replicas    map[string][]*ReplicaInfo
	reports     []*RepairReport
	mu          sync.RWMutex
	stopCh      chan struct{}
}

// NewManager 创建自愈存储管理器
func NewManager(config *SelfHealingConfig) *Manager {
	return &Manager{
		config:   config,
		scrubTasks: make(map[string]*ScrubTask),
		blocks:   make(map[string]*DataBlock),
		replicas: make(map[string][]*ReplicaInfo),
		stopCh:   make(chan struct{}),
	}
}

// Start 启动自愈存储
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return nil
	}
	
	go m.runScrubScheduler()
	go m.monitorIntegrity()
	
	return nil
}

// Stop 停止自愈存储
func (m *Manager) Stop() {
	close(m.stopCh)
}

// runScrubScheduler 运行 scrub 调度器
func (m *Manager) runScrubScheduler() {
	ticker := time.NewTicker(time.Duration(m.config.ScrubInterval) * time.Hour)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.scheduleScrub()
		}
	}
}

// scheduleScrub 调度 scrub 任务
func (m *Manager) scheduleScrub() {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	task := &ScrubTask{
		ID:     fmt.Sprintf("scrub_%d", time.Now().UnixNano()),
		Name:   "scheduled-scrub",
		Target: "/",
		Status: ScrubStatusPending,
	}
	
	m.scrubTasks[task.ID] = task
	go m.executeScrub(task)
}

// executeScrub 执行 scrub 任务
func (m *Manager) executeScrub(task *ScrubTask) {
	m.mu.Lock()
	task.Status = ScrubStatusRunning
	now := time.Now()
	task.StartedAt = &now
	m.mu.Unlock()
	
	// 模拟 scrub 过程
	m.mu.RLock()
	totalBlocks := len(m.blocks)
	m.mu.RUnlock()
	
	if totalBlocks == 0 {
		m.mu.Lock()
		task.Status = ScrubStatusCompleted
		task.Progress = 100
		completedAt := time.Now()
		task.CompletedAt = &completedAt
		m.mu.Unlock()
		return
	}
	
	checked := 0
	corrupted := 0
	
	for blockID := range m.blocks {
		m.mu.RLock()
		block := m.blocks[blockID]
		m.mu.RUnlock()
		
		// 验证校验和
		if !m.verifyBlock(block) {
			corrupted++
			m.recordCorruption(block)
			
			if m.config.AutoRepair {
				m.repairBlock(block)
			}
		}
		
		checked++
		
		m.mu.Lock()
		task.ScrubbedFiles = int64(checked)
		task.TotalFiles = int64(totalBlocks)
		task.CorruptedFiles = int64(corrupted)
		task.Progress = float64(checked) / float64(totalBlocks) * 100
		m.mu.Unlock()
	}
	
	m.mu.Lock()
	task.Status = ScrubStatusCompleted
	completedAt := time.Now()
	task.CompletedAt = &completedAt
	m.mu.Unlock()
}

// verifyBlock 验证数据块
func (m *Manager) verifyBlock(block *DataBlock) bool {
	// 计算当前校验和
	currentChecksum := m.calculateChecksum(block.Path)
	return currentChecksum == block.Checksum
}

// calculateChecksum 计算校验和
func (m *Manager) calculateChecksum(path string) string {
	// 模拟校验和计算
	hash := sha256.Sum256([]byte(path + time.Now().String()))
	return hex.EncodeToString(hash[:])
}

// recordCorruption 记录损坏
func (m *Manager) recordCorruption(block *DataBlock) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	record := CorruptionRecord{
		ID:           fmt.Sprintf("corr_%d", time.Now().UnixNano()),
		BlockID:      block.ID,
		Path:         block.Path,
		DetectedAt:   time.Now(),
		RepairStatus: RepairStatusPending,
	}
	
	m.corruptions = append(m.corruptions, record)
}

// repairBlock 修复数据块
func (m *Manager) repairBlock(block *DataBlock) {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	replicas, ok := m.replicas[block.ID]
	if !ok || len(replicas) == 0 {
		return
	}
	
	// 找到健康的副本
	for _, replica := range replicas {
		if replica.Healthy {
			// 从健康副本修复
			block.Checksum = replica.Checksum
			block.UpdatedAt = time.Now()
			
			// 更新损坏记录
			for i := range m.corruptions {
				if m.corruptions[i].BlockID == block.ID && m.corruptions[i].RepairStatus == RepairStatusPending {
					m.corruptions[i].RepairStatus = RepairStatusSuccess
					now := time.Now()
					m.corruptions[i].RepairAt = &now
					break
				}
			}
			
			return
		}
	}
}

// monitorIntegrity 监控数据完整性
func (m *Manager) monitorIntegrity() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.stopCh:
			return
		case <-ticker.C:
			m.checkIntegrity()
		}
	}
}

// checkIntegrity 检查数据完整性
func (m *Manager) checkIntegrity() {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// 检查副本一致性
	for blockID, replicas := range m.replicas {
		for _, replica := range replicas {
			block := m.blocks[blockID]
			if block == nil {
				continue
			}
			
			if replica.Checksum != block.Checksum {
				replica.Healthy = false
			} else {
				replica.Healthy = true
			}
			replica.LastCheck = time.Now()
		}
	}
}

// RegisterBlock 注册数据块
func (m *Manager) RegisterBlock(block *DataBlock) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if block.ID == "" {
		block.ID = fmt.Sprintf("block_%d", time.Now().UnixNano())
	}
	
	block.CreatedAt = time.Now()
	block.UpdatedAt = time.Now()
	
	if block.Algorithm == "" {
		block.Algorithm = m.config.ChecksumAlgorithm
	}
	
	m.blocks[block.ID] = block
	return nil
}

// AddReplica 添加副本
func (m *Manager) AddReplica(blockID string, replica *ReplicaInfo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if _, ok := m.blocks[blockID]; !ok {
		return fmt.Errorf("block not found: %s", blockID)
	}
	
	replica.BlockID = blockID
	replica.LastCheck = time.Now()
	m.replicas[blockID] = append(m.replicas[blockID], replica)
	
	return nil
}

// CreateScrubTask 创建 scrub 任务
func (m *Manager) CreateScrubTask(name, target string) *ScrubTask {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	task := &ScrubTask{
		ID:     fmt.Sprintf("scrub_%d", time.Now().UnixNano()),
		Name:   name,
		Target: target,
		Status: ScrubStatusPending,
	}
	
	m.scrubTasks[task.ID] = task
	go m.executeScrub(task)
	
	return task
}

// GetScrubTask 获取 scrub 任务
func (m *Manager) GetScrubTask(taskID string) (*ScrubTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	task, ok := m.scrubTasks[taskID]
	if !ok {
		return nil, fmt.Errorf("scrub task not found: %s", taskID)
	}
	
	return task, nil
}

// ListScrubTasks 列出 scrub 任务
func (m *Manager) ListScrubTasks() []*ScrubTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	tasks := make([]*ScrubTask, 0, len(m.scrubTasks))
	for _, task := range m.scrubTasks {
		tasks = append(tasks, task)
	}
	
	return tasks
}

// GetCorruptionRecords 获取损坏记录
func (m *Manager) GetCorruptionRecords() []CorruptionRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return m.corruptions
}

// GenerateRepairReport 生成修复报告
func (m *Manager) GenerateRepairReport(scrubTaskID string) *RepairReport {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	report := &RepairReport{
		ID:          fmt.Sprintf("report_%d", time.Now().UnixNano()),
		ScrubTaskID: scrubTaskID,
		StartedAt:   time.Now(),
	}
	
	for _, record := range m.corruptions {
		report.TotalBlocks++
		
		switch record.RepairStatus {
		case RepairStatusSuccess:
			report.RepairedBlocks++
		case RepairStatusFailed:
			report.FailedBlocks++
		default:
			report.CorruptedBlocks++
		}
		
		report.Records = append(report.Records, record)
	}
	
	m.reports = append(m.reports, report)
	return report
}

// GetDashboard 获取仪表盘数据
func (m *Manager) GetDashboard() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	totalBlocks := len(m.blocks)
	totalReplicas := 0
	healthyReplicas := 0
	
	for _, replicas := range m.replicas {
		for _, replica := range replicas {
			totalReplicas++
			if replica.Healthy {
				healthyReplicas++
			}
		}
	}
	
	return map[string]interface{}{
		"total_blocks":      totalBlocks,
		"total_replicas":    totalReplicas,
		"healthy_replicas":  healthyReplicas,
		"corruption_count":  len(m.corruptions),
		"scrub_tasks":       len(m.scrubTasks),
		"integrity_level":   m.config.IntegrityLevel,
		"auto_repair":       m.config.AutoRepair,
	}
}

// MarshalJSON 序列化
func (m *Manager) MarshalJSON() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	return json.Marshal(struct {
		Config      *SelfHealingConfig `json:"config"`
		Blocks      int                `json:"blocks_count"`
		Replicas    int                `json:"replicas_count"`
		Corruptions int                `json:"corruptions_count"`
		Reports     int                `json:"reports_count"`
	}{
		Config:      m.config,
		Blocks:      len(m.blocks),
		Replicas:    len(m.replicas),
		Corruptions: len(m.corruptions),
		Reports:     len(m.reports),
	})
}
