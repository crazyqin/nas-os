package smartmigrate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SmartMigrateManager 智能数据迁移管理器
// 对标群晖 Data Migration 和 TrueNAS 数据迁移功能
// 支持跨存储池、跨设备、跨节点的数据迁移.
type SmartMigrateManager struct {
	mu      sync.RWMutex
	config  *MigrateConfig
	tasks   map[string]*MigrateTask
	history []MigrateRecord
	ctx     context.Context
	cancel  context.CancelFunc
	wg      sync.WaitGroup
}

// MigrateConfig 迁移配置.
type MigrateConfig struct {
	Enabled          bool `json:"enabled"`
	MaxConcurrent    int  `json:"max_concurrent"`
	ChunkSizeMB      int  `json:"chunk_size_mb"`
	VerifyChecksum   bool `json:"verify_checksum"`
	PreservePerms    bool `json:"preserve_permissions"`
	BandwidthLimitMB int  `json:"bandwidth_limit_mb"` // 0=无限
	RetryCount       int  `json:"retry_count"`
	RetryDelaySec    int  `json:"retry_delay_sec"`
}

// MigrateTask 迁移任务.
type MigrateTask struct {
	ID              string          `json:"id"`
	Name            string          `json:"name"`
	SourcePath      string          `json:"source_path"`
	DestPath        string          `json:"dest_path"`
	Type            MigrateType     `json:"type"`
	Status          MigrateStatus   `json:"status"`
	TotalBytes      int64           `json:"total_bytes"`
	TransferedBytes int64           `json:"transferred_bytes"`
	TotalFiles      int             `json:"total_files"`
	TransferedFiles int             `json:"transferred_files"`
	SpeedMBps       float64         `json:"speed_mbps"`
	ETA             time.Duration   `json:"eta"`
	StartTime       time.Time       `json:"start_time"`
	EndTime         *time.Time      `json:"end_time,omitempty"`
	ErrorMsg        string          `json:"error_msg,omitempty"`
	Options         *MigrateOptions `json:"options,omitempty"`
	ChecksumOK      bool            `json:"checksum_ok"`
}

// MigrateType 迁移类型.
type MigrateType string

const (
	TypeCopy      MigrateType = "copy"      // 复制
	TypeMove      MigrateType = "move"      // 移动
	TypeSync      MigrateType = "sync"      // 同步
	TypeReplicate MigrateType = "replicate" // 复制（含校验）
)

// MigrateStatus 迁移状态.
type MigrateStatus string

const (
	MigrateStatusPending   MigrateStatus = "pending"
	MigrateStatusRunning   MigrateStatus = "running"
	MigrateStatusPaused    MigrateStatus = "paused"
	MigrateStatusCompleted MigrateStatus = "completed"
	MigrateStatusFailed    MigrateStatus = "failed"
	MigrateStatusCancelled MigrateStatus = "cancelled"
)

// MigrateOptions 迁移选项.
type MigrateOptions struct {
	ExcludePatterns []string `json:"exclude_patterns"`
	IncludePatterns []string `json:"include_patterns"`
	DryRun          bool     `json:"dry_run"`
	SyncDelete      bool     `json:"sync_delete"` // 同步删除目标端多余文件
	Compress        bool     `json:"compress"`
	Encrypt         bool     `json:"encrypt"`
}

// MigrateRecord 迁移历史记录.
type MigrateRecord struct {
	TaskID      string        `json:"task_id"`
	SourcePath  string        `json:"source_path"`
	DestPath    string        `json:"dest_path"`
	TotalBytes  int64         `json:"total_bytes"`
	Duration    time.Duration `json:"duration"`
	Success     bool          `json:"success"`
	ErrorMsg    string        `json:"error_msg,omitempty"`
	CompletedAt time.Time     `json:"completed_at"`
}

// NewSmartMigrateManager 创建智能迁移管理器.
func NewSmartMigrateManager(cfg *MigrateConfig) *SmartMigrateManager {
	if cfg == nil {
		cfg = &MigrateConfig{
			Enabled:        true,
			MaxConcurrent:  3,
			ChunkSizeMB:    64,
			VerifyChecksum: true,
			PreservePerms:  true,
			RetryCount:     3,
			RetryDelaySec:  5,
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	return &SmartMigrateManager{
		config:  cfg,
		tasks:   make(map[string]*MigrateTask),
		history: make([]MigrateRecord, 0),
		ctx:     ctx,
		cancel:  cancel,
	}
}

// Start 启动管理器.
func (m *SmartMigrateManager) Start() error { return nil }

// Stop 停止管理器.
func (m *SmartMigrateManager) Stop() error {
	m.cancel()
	m.wg.Wait()
	return nil
}

// CreateTask 创建迁移任务.
func (m *SmartMigrateManager) CreateTask(name, src, dst string, mtype MigrateType, opts *MigrateOptions) (*MigrateTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证源路径
	srcInfo, err := os.Stat(src)
	if err != nil {
		return nil, fmt.Errorf("源路径无效: %w", err)
	}

	// 计算总大小
	totalBytes, totalFiles := int64(0), 0
	if srcInfo.IsDir() {
		filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
			if err == nil && !info.IsDir() {
				totalBytes += info.Size()
				totalFiles++
			}
			return nil
		})
	} else {
		totalBytes = srcInfo.Size()
		totalFiles = 1
	}

	task := &MigrateTask{
		ID:         fmt.Sprintf("mig-%d", time.Now().UnixNano()),
		Name:       name,
		SourcePath: src,
		DestPath:   dst,
		Type:       mtype,
		Status:     MigrateStatusPending,
		TotalBytes: totalBytes,
		TotalFiles: totalFiles,
		StartTime:  time.Now(),
		Options:    opts,
	}

	m.tasks[task.ID] = task
	return task, nil
}

// StartTask 启动迁移任务.
func (m *SmartMigrateManager) StartTask(taskID string) error {
	m.mu.RLock()
	task, exists := m.tasks[taskID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}
	if task.Status == MigrateStatusRunning {
		return fmt.Errorf("任务 %s 已在运行中", taskID)
	}

	task.Status = MigrateStatusRunning
	m.wg.Add(1)
	go m.executeTask(task)
	return nil
}

func (m *SmartMigrateManager) executeTask(task *MigrateTask) {
	defer m.wg.Done()
	startTime := time.Now()

	defer func() {
		now := time.Now()
		task.EndTime = &now
		duration := now.Sub(startTime)
		m.mu.Lock()
		m.history = append(m.history, MigrateRecord{
			TaskID:      task.ID,
			SourcePath:  task.SourcePath,
			DestPath:    task.DestPath,
			TotalBytes:  task.TotalBytes,
			Duration:    duration,
			Success:     task.Status == MigrateStatusCompleted,
			ErrorMsg:    task.ErrorMsg,
			CompletedAt: now,
		})
		m.mu.Unlock()
	}()

	srcInfo, err := os.Stat(task.SourcePath)
	if err != nil {
		task.Status = MigrateStatusFailed
		task.ErrorMsg = err.Error()
		return
	}

	if srcInfo.IsDir() {
		err = m.migrateDirectory(task)
	} else {
		err = m.migrateFile(task, task.SourcePath, task.DestPath)
	}

	if err != nil {
		task.Status = MigrateStatusFailed
		task.ErrorMsg = err.Error()
		return
	}

	// 校验
	if m.config.VerifyChecksum && task.Options != nil && !task.Options.DryRun {
		ok, err := m.verifyChecksum(task)
		if err != nil || !ok {
			task.Status = MigrateStatusFailed
			task.ErrorMsg = "校验失败"
			task.ChecksumOK = false
			return
		}
		task.ChecksumOK = true
	}

	task.Status = MigrateStatusCompleted
	task.SpeedMBps = float64(task.TotalBytes) / 1024 / 1024 / time.Since(task.StartTime).Seconds()
}

func (m *SmartMigrateManager) migrateDirectory(task *MigrateTask) error {
	return filepath.Walk(task.SourcePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(task.SourcePath, path)
		destPath := filepath.Join(task.DestPath, relPath)

		if info.IsDir() {
			return os.MkdirAll(destPath, info.Mode())
		}
		return m.migrateFile(task, path, destPath)
	})
}

func (m *SmartMigrateManager) migrateFile(task *MigrateTask, src, dst string) error {
	if task.Options != nil && task.Options.DryRun {
		task.TransferedBytes += mustFileSize(src)
		task.TransferedFiles++
		return nil
	}

	if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
		return err
	}

	for retry := 0; retry <= m.config.RetryCount; retry++ {
		err := m.copyFile(task, src, dst)
		if err == nil {
			task.TransferedFiles++
			return nil
		}
		if retry < m.config.RetryCount {
			time.Sleep(time.Duration(m.config.RetryDelaySec) * time.Second)
		}
		return err
	}
	return nil
}

func (m *SmartMigrateManager) copyFile(task *MigrateTask, src, dst string) error {
	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	buf := make([]byte, m.config.ChunkSizeMB*1024*1024)
	for {
		n, err := srcFile.Read(buf)
		if n > 0 {
			written, werr := dstFile.Write(buf[:n])
			if werr != nil {
				return werr
			}
			task.TransferedBytes += int64(written)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
	}

	// 保留权限
	if m.config.PreservePerms {
		info, _ := os.Stat(src)
		if info != nil {
			os.Chmod(dst, info.Mode())
		}
	}

	return nil
}

func (m *SmartMigrateManager) verifyChecksum(task *MigrateTask) (bool, error) {
	srcHash, err := hashFile(task.SourcePath)
	if err != nil {
		return false, err
	}
	dstHash, err := hashFile(task.DestPath)
	if err != nil {
		return false, err
	}
	return srcHash == dstHash, nil
}

func hashFile(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func mustFileSize(path string) int64 {
	info, err := os.Stat(path)
	if err != nil {
		return 0
	}
	return info.Size()
}

// GetTask 获取任务详情.
func (m *SmartMigrateManager) GetTask(taskID string) (*MigrateTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}
	return task, nil
}

// ListTasks 列出所有任务.
func (m *SmartMigrateManager) ListTasks() []*MigrateTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*MigrateTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		result = append(result, t)
	}
	return result
}

// PauseTask 暂停任务.
func (m *SmartMigrateManager) PauseTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}
	if task.Status != MigrateStatusRunning {
		return fmt.Errorf("任务 %s 不在运行中", taskID)
	}
	task.Status = MigrateStatusPaused
	return nil
}

// CancelTask 取消任务.
func (m *SmartMigrateManager) CancelTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	task, exists := m.tasks[taskID]
	if !exists {
		return fmt.Errorf("任务 %s 不存在", taskID)
	}
	task.Status = MigrateStatusCancelled
	return nil
}

// GetHistory 获取迁移历史.
func (m *SmartMigrateManager) GetHistory() []MigrateRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]MigrateRecord, len(m.history))
	copy(result, m.history)
	return result
}
