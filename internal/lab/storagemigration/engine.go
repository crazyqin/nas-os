package storagemigration

import (
	"fmt"
	"sync"
	"time"
)

// Engine 存储迁移引擎。
type Engine struct {
	mu    sync.RWMutex
	tasks map[string]*MigrationTask
}

// New 创建迁移引擎。
func New() *Engine {
	return &Engine{
		tasks: make(map[string]*MigrationTask),
	}
}

// ValidateConfig 验证迁移配置。
func (e *Engine) ValidateConfig(cfg MigrationConfig) error {
	valid := false
	for _, s := range AllSources() {
		if cfg.Source == s {
			valid = true
			break
		}
	}
	if !valid {
		return ErrInvalidSource
	}
	if cfg.SourceHost == "" {
		return fmt.Errorf("源主机不能为空")
	}
	if cfg.SourcePath == "" {
		return fmt.Errorf("源路径不能为空")
	}
	if cfg.DestPath == "" {
		return fmt.Errorf("目标路径不能为空")
	}
	return nil
}

// Start 启动迁移任务。
func (e *Engine) Start(cfg MigrationConfig) (*MigrationTask, error) {
	if err := e.ValidateConfig(cfg); err != nil {
		return nil, err
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	task := &MigrationTask{
		ID:        fmt.Sprintf("migrate-%d", time.Now().UnixNano()),
		Config:    cfg,
		Status:    StatusPending,
		StartedAt: time.Now(),
		Log:       []string{},
	}

	e.tasks[task.ID] = task

	// 模拟异步执行
	go e.runMigration(task)

	return task, nil
}

// GetTask 获取迁移任务。
func (e *Engine) GetTask(taskID string) (*MigrationTask, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	task, ok := e.tasks[taskID]
	if !ok {
		return nil, ErrNoMigration
	}
	return task, nil
}

// ListTasks 列出所有迁移任务。
func (e *Engine) ListTasks() []*MigrationTask {
	e.mu.RLock()
	defer e.mu.RUnlock()

	tasks := make([]*MigrationTask, 0, len(e.tasks))
	for _, t := range e.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// Cancel 取消迁移任务。
func (e *Engine) Cancel(taskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	task, ok := e.tasks[taskID]
	if !ok {
		return ErrNoMigration
	}
	if task.Status != StatusMigrating && task.Status != StatusScanning {
		return fmt.Errorf("任务状态 %s 无法取消", task.Status)
	}

	task.Status = StatusCancelled
	now := time.Now()
	task.EndedAt = &now
	task.Log = append(task.Log, fmt.Sprintf("[%s] 迁移已取消", now.Format(time.RFC3339)))
	return nil
}

// GetReport 获取迁移报告。
func (e *Engine) GetReport(taskID string) (*MigrationReport, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	task, ok := e.tasks[taskID]
	if !ok {
		return nil, ErrNoMigration
	}
	if task.Status != StatusCompleted && task.Status != StatusFailed {
		return nil, fmt.Errorf("迁移尚未完成，当前状态: %s", task.Status)
	}

	report := &MigrationReport{
		TaskID:       task.ID,
		Source:       task.Config.Source,
		SourceHost:   task.Config.SourceHost,
		TotalFiles:   task.TotalFiles,
		TotalBytes:   task.TotalBytes,
		SuccessFiles: task.DoneFiles,
		StartedAt:    task.StartedAt,
	}

	if task.EndedAt != nil {
		report.EndedAt = *task.EndedAt
		report.Duration = task.EndedAt.Sub(task.StartedAt)
	}

	if report.Duration.Seconds() > 0 {
		report.AvgSpeed = float64(task.DoneBytes) / 1024 / 1024 / report.Duration.Seconds()
	}

	return report, nil
}

// runMigration 执行迁移（模拟）。
func (e *Engine) runMigration(task *MigrationTask) {
	// 扫描阶段
	e.mu.Lock()
	task.Status = StatusScanning
	task.Log = append(task.Log, fmt.Sprintf("[%s] 开始扫描源数据...", time.Now().Format(time.RFC3339)))
	e.mu.Unlock()

	time.Sleep(100 * time.Millisecond)

	// 模拟扫描结果
	e.mu.Lock()
	task.TotalFiles = 1234
	task.TotalBytes = 50 * 1024 * 1024 * 1024 // 50GB
	task.Status = StatusMigrating
	task.Log = append(task.Log, fmt.Sprintf("[%s] 扫描完成: %d 文件, %d 字节", time.Now().Format(time.RFC3339), task.TotalFiles, task.TotalBytes))
	e.mu.Unlock()

	// 迁移阶段（模拟进度）
	steps := 10
	for i := 0; i < steps; i++ {
		e.mu.Lock()
		if task.Status == StatusCancelled {
			return
		}
		task.DoneFiles = int64(float64(task.TotalFiles) * float64(i+1) / float64(steps))
		task.DoneBytes = int64(float64(task.TotalBytes) * float64(i+1) / float64(steps))
		task.Progress = float64(i+1) / float64(steps) * 100
		task.Speed = 120.5
		task.ETA = time.Duration(steps-i-1) * 100 * time.Millisecond
		e.mu.Unlock()
		time.Sleep(50 * time.Millisecond)
	}

	// 验证阶段
	e.mu.Lock()
	task.Status = StatusVerifying
	task.Log = append(task.Log, fmt.Sprintf("[%s] 开始验证数据完整性...", time.Now().Format(time.RFC3339)))
	e.mu.Unlock()

	time.Sleep(100 * time.Millisecond)

	// 完成
	e.mu.Lock()
	task.Status = StatusCompleted
	task.Progress = 100
	task.DoneFiles = task.TotalFiles
	task.DoneBytes = task.TotalBytes
	task.Speed = 0
	task.ETA = 0
	now := time.Now()
	task.EndedAt = &now
	task.Log = append(task.Log, fmt.Sprintf("[%s] 迁移完成", now.Format(time.RFC3339)))
	e.mu.Unlock()
}
