// Package migration 提供设备间数据迁移管理功能
package migration

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 常量定义 ==========

// 默认超时时间.
const (
	DefaultScanTimeout    = 10 * time.Minute
	DefaultMigrateTimeout = 24 * time.Hour
)

// ========== 状态定义 ==========

// Status 迁移任务状态.
type Status string

const (
	StatusPending    Status = "pending"
	StatusScanning   Status = "scanning"
	StatusMigrating  Status = "migrating"
	StatusVerifying  Status = "verifying"
	StatusCompleted  Status = "completed"
	StatusFailed     Status = "failed"
	StatusCancelled  Status = "cancelled"
	StatusRolledBack Status = "rolled_back"
)

// Mode 迁移模式.
type Mode string

const (
	ModeFull        Mode = "full"
	ModeIncremental Mode = "incremental"
)

// ========== 数据结构 ==========

// Task 迁移任务.
type Task struct {
	ID           string     `json:"id"`
	Name         string     `json:"name"`
	SourceDevice string     `json:"sourceDevice"`
	TargetDevice string     `json:"targetDevice"`
	SourcePath   string     `json:"sourcePath"`
	TargetPath   string     `json:"targetPath"`
	Mode         Mode       `json:"mode"`
	Status       Status     `json:"status"`
	Progress     int        `json:"progress"`  // 百分比 0-100
	Speed        int64      `json:"speed"`     // bytes/sec
	RemainingSec int64      `json:"remainingSec"`
	TotalSize    int64      `json:"totalSize"`
	Transferred  int64      `json:"transferred"`
	TotalFiles   int64      `json:"totalFiles"`
	FilesDone    int64      `json:"filesDone"`
	Error        string     `json:"error,omitempty"`
	CreatedAt    time.Time  `json:"createdAt"`
	StartedAt    time.Time  `json:"startedAt,omitempty"`
	FinishedAt   time.Time  `json:"finishedAt,omitempty"`
	SnapshotID   string     `json:"snapshotId,omitempty"` // 回滚快照
}

// ScanResult 预扫描结果.
type ScanResult struct {
	TotalSize    int64  `json:"totalSize"`
	TotalFiles   int64  `json:"totalFiles"`
	EstimatedSec int64  `json:"estimatedSec"`
	Incremental  bool   `json:"incremental"`
	ChangedFiles int64  `json:"changedFiles,omitempty"`
	ChangedSize  int64  `json:"changedSize,omitempty"`
}

// VerifyResult 验证结果.
type VerifyResult struct {
	Valid       bool   `json:"valid"`
	CheckedFiles int64 `json:"checkedFiles"`
	Mismatches  int64  `json:"mismatches"`
	Duration    int64  `json:"durationMs"`
}

// MigrationError 迁移业务错误.
type MigrationError struct {
	Code    int
	Message string
	Err     error
}

func (e *MigrationError) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %v", e.Message, e.Err)
	}
	return e.Message
}

func (e *MigrationError) Unwrap() error { return e.Err }
func (e *MigrationError) NotFound() bool { return e.Code == 404 }
func (e *MigrationError) BadRequest() bool { return e.Code == 400 }
func (e *MigrationError) Conflict() bool { return e.Code == 409 }

// 预定义错误.
var (
	ErrTaskNotFound     = &MigrationError{Code: 404, Message: "迁移任务不存在"}
	ErrTaskNotRunning   = &MigrationError{Code: 400, Message: "迁移任务未在运行"}
	ErrTaskNotCompleted = &MigrationError{Code: 400, Message: "迁移任务未完成，无法验证"}
	ErrInvalidDevice    = &MigrationError{Code: 400, Message: "设备ID无效"}
	ErrAlreadyRunning   = &MigrationError{Code: 409, Message: "已有迁移任务正在运行"}
	ErrNoSnapshot       = &MigrationError{Code: 400, Message: "无可用快照，无法回滚"}
)

// ========== Manager ==========

// Manager 迁移管理器.
type Manager struct {
	mu      sync.RWMutex
	tasks   map[string]*Task
	cancels map[string]context.CancelFunc

	// 文件传输接口（可替换为 mock）
	transferFn TransferFunc
	scanFn     ScanFunc
	verifyFn   VerifyFunc
}

// TransferFunc 文件传输函数签名.
type TransferFunc func(ctx context.Context, src, dst string, progress func(int64)) error

// ScanFunc 预扫描函数签名.
type ScanFunc func(ctx context.Context, src string) (*ScanResult, error)

// VerifyFunc 数据验证函数签名.
type VerifyFunc func(ctx context.Context, src, dst string) (*VerifyResult, error)

// NewManager 创建迁移管理器.
func NewManager() *Manager {
	m := &Manager{
		tasks:   make(map[string]*Task),
		cancels: make(map[string]context.CancelFunc),
	}
	m.transferFn = defaultTransfer
	m.scanFn = defaultScan
	m.verifyFn = defaultVerify
	return m
}

// ========== 创建任务 ==========

// CreateRequest 创建迁移任务请求.
type CreateRequest struct {
	Name         string `json:"name" validate:"required"`
	SourceDevice string `json:"sourceDevice" validate:"required"`
	TargetDevice string `json:"targetDevice" validate:"required"`
	SourcePath   string `json:"sourcePath" validate:"required"`
	TargetPath   string `json:"targetPath" validate:"required"`
	Mode         Mode   `json:"mode"`
}

// CreateTask 创建迁移任务.
func (m *Manager) CreateTask(req *CreateRequest) (*Task, error) {
	if req.SourceDevice == "" || req.TargetDevice == "" {
		return nil, &MigrationError{Code: 400, Message: "源设备和目标设备不能为空"}
	}
	if req.SourcePath == "" || req.TargetPath == "" {
		return nil, &MigrationError{Code: 400, Message: "源路径和目标路径不能为空"}
	}
	if req.Mode == "" {
		req.Mode = ModeFull
	}

	task := &Task{
		ID:           uuid.New().String(),
		Name:         req.Name,
		SourceDevice: req.SourceDevice,
		TargetDevice: req.TargetDevice,
		SourcePath:   req.SourcePath,
		TargetPath:   req.TargetPath,
		Mode:         req.Mode,
		Status:       StatusPending,
		CreatedAt:    time.Now(),
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	slog.Info("迁移任务已创建", "id", task.ID, "source", req.SourceDevice, "target", req.TargetDevice)
	return task, nil
}

// ========== 预扫描 ==========

// Scan 预扫描评估数据量.
func (m *Manager) Scan(ctx context.Context, taskID string) (*ScanResult, error) {
	m.mu.RLock()
	task, ok := m.tasks[taskID]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrTaskNotFound
	}

	task.Status = StatusScanning
	result, err := m.scanFn(ctx, task.SourcePath)
	if err != nil {
		task.Status = StatusFailed
		task.Error = fmt.Sprintf("扫描失败: %v", err)
		return nil, err
	}

	task.TotalSize = result.TotalSize
	task.TotalFiles = result.TotalFiles
	task.Status = StatusPending

	return result, nil
}

// ========== 执行迁移 ==========

// Start 启动迁移任务.
func (m *Manager) Start(taskID string) error {
	m.mu.RLock()
	task, ok := m.tasks[taskID]
	m.mu.RUnlock()
	if !ok {
		return ErrTaskNotFound
	}

	if task.Status == StatusMigrating {
		return ErrAlreadyRunning
	}

	ctx, cancel := context.WithCancel(context.Background())
	m.mu.Lock()
	m.cancels[taskID] = cancel
	task.Status = StatusMigrating
	m.mu.Unlock()

	go m.runMigration(ctx, task)
	return nil
}

func (m *Manager) runMigration(ctx context.Context, task *Task) {
	task.StartedAt = time.Now()

	var transferred int64
	progressFn := func(bytes int64) {
		transferred += bytes
		task.Transferred = transferred
		if task.TotalSize > 0 {
			task.Progress = int(transferred * 100 / task.TotalSize)
		}
		elapsed := time.Since(task.StartedAt).Seconds()
		if elapsed > 0 {
			task.Speed = int64(float64(transferred) / elapsed)
			if task.Speed > 0 && task.TotalSize > transferred {
				task.RemainingSec = (task.TotalSize - transferred) / task.Speed
			}
		}
	}

	err := m.transferFn(ctx, task.SourcePath, task.TargetPath, progressFn)

	m.mu.Lock()
	delete(m.cancels, task.ID)
	m.mu.Unlock()

	if err != nil {
		if ctx.Err() != nil {
			task.Status = StatusCancelled
		} else {
			task.Status = StatusFailed
			task.Error = err.Error()
		}
	} else {
		task.Progress = 100
		task.Status = StatusCompleted
		task.FinishedAt = time.Now()
		task.SnapshotID = uuid.New().String() // 生成回滚快照 ID
		slog.Info("迁移完成", "id", task.ID, "duration", task.FinishedAt.Sub(task.StartedAt))
	}
}

// ========== 取消迁移 ==========

// Cancel 取消迁移任务.
func (m *Manager) Cancel(taskID string) error {
	m.mu.RLock()
	cancel, ok := m.cancels[taskID]
	m.mu.RUnlock()
	if !ok {
		return ErrTaskNotRunning
	}
	cancel()
	return nil
}

// ========== 回滚 ==========

// Rollback 回滚迁移（恢复到迁移前状态）.
func (m *Manager) Rollback(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if task.SnapshotID == "" {
		return ErrNoSnapshot
	}

	// 模拟回滚：删除目标数据，恢复快照
	task.Status = StatusRolledBack
	task.Progress = 0
	task.Transferred = 0
	task.Error = ""

	slog.Info("迁移已回滚", "id", taskID, "snapshot", task.SnapshotID)
	return nil
}

// ========== 验证 ==========

// Verify 验证迁移数据完整性.
func (m *Manager) Verify(ctx context.Context, taskID string) (*VerifyResult, error) {
	m.mu.RLock()
	task, ok := m.tasks[taskID]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrTaskNotFound
	}

	if task.Status != StatusCompleted {
		return nil, ErrTaskNotCompleted
	}

	task.Status = StatusVerifying
	result, err := m.verifyFn(ctx, task.SourcePath, task.TargetPath)
	if err != nil {
		task.Status = StatusFailed
		task.Error = fmt.Sprintf("验证失败: %v", err)
		return nil, err
	}

	if result.Valid {
		task.Status = StatusCompleted
	} else {
		task.Status = StatusFailed
		task.Error = fmt.Sprintf("验证发现 %d 处不一致", result.Mismatches)
	}

	return result, nil
}

// ========== 查询 ==========

// GetTask 获取任务详情.
func (m *Manager) GetTask(taskID string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListTasks 列出所有任务.
func (m *Manager) ListTasks() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tasks := make([]*Task, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// DeleteTask 删除任务记录.
func (m *Manager) DeleteTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.tasks[taskID]; !ok {
		return ErrTaskNotFound
	}
	if cancel, exists := m.cancels[taskID]; exists {
		cancel()
		delete(m.cancels, taskID)
	}
	delete(m.tasks, taskID)
	return nil
}

// ========== 默认实现 ==========

func defaultTransfer(ctx context.Context, src, dst string, progress func(int64)) error {
	// 默认实现：模拟传输
	const chunkSize = 1024 * 1024 // 1MB
	for i := 0; i < 10; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}
		time.Sleep(50 * time.Millisecond)
		progress(chunkSize)
	}
	return nil
}

func defaultScan(_ context.Context, _ string) (*ScanResult, error) {
	return &ScanResult{
		TotalSize:  10 * 1024 * 1024, // 10MB
		TotalFiles: 100,
	}, nil
}

func defaultVerify(_ context.Context, src, dst string) (*VerifyResult, error) {
	return &VerifyResult{
		Valid:        true,
		CheckedFiles: 100,
		Mismatches:   0,
		Duration:     100,
	}, nil
}

// ComputeChecksum 计算文件校验和（用于数据完整性校验）.
func ComputeChecksum(r io.Reader) (string, error) {
	h := sha256.New()
	if _, err := io.Copy(h, r); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// ========== 接口注入（测试用） ==========

// SetTransferFunc 设置传输函数.
func (m *Manager) SetTransferFunc(fn TransferFunc) { m.transferFn = fn }

// SetScanFunc 设置扫描函数.
func (m *Manager) SetScanFunc(fn ScanFunc) { m.scanFn = fn }

// SetVerifyFunc 设置验证函数.
func (m *Manager) SetVerifyFunc(fn VerifyFunc) { m.verifyFn = fn }

// ensure errors is used (avoid unused import in some build modes).
var _ = errors.New
