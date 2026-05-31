// Package backupverify 备份完整性验证
// 对标群晖Active Backup验证 + TrueNAS Scrub
// 定期校验备份文件完整性，支持测试恢复、审计追踪
package backupverify

import (
	"crypto/sha256"
	"fmt"
	"sync"
	"time"
)

// VerifyStatus 验证状态
type VerifyStatus string

const (
	VerifyPending  VerifyStatus = "pending"
	VerifyRunning  VerifyStatus = "running"
	VerifyPassed   VerifyStatus = "passed"
	VerifyFailed   VerifyStatus = "failed"
	VerifySkipped  VerifyStatus = "skipped"
)

// VerifyMode 验证模式
type VerifyMode string

const (
	ModeChecksum VerifyMode = "checksum"  // 校验和验证
	ModeDeep     VerifyMode = "deep"      // 深度验证（读取+校验）
	ModeRestore  VerifyMode = "restore"   // 测试恢复验证
)

// VerifyTask 验证任务
type VerifyTask struct {
	ID          string       `json:"id"`
	BackupID    string       `json:"backup_id"`
	BackupPath  string       `json:"backup_path"`
	Mode        VerifyMode   `json:"mode"`
	Status      VerifyStatus `json:"status"`
	TotalFiles  int          `json:"total_files"`
	CheckedFiles int         `json:"checked_files"`
	FailedFiles int          `json:"failed_files"`
	TotalSize   int64        `json:"total_size"`
	Duration    string       `json:"duration"`
	Errors      []VerifyError `json:"errors"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt *time.Time   `json:"completed_at"`
}

// VerifyError 验证错误
type VerifyError struct {
	File    string `json:"file"`
	Error   string `json:"error"`
	Expected string `json:"expected,omitempty"`
	Actual   string `json:"actual,omitempty"`
}

// VerifySchedule 验证计划
type VerifySchedule struct {
	ID         string     `json:"id"`
	BackupID   string     `json:"backup_id"`
	Mode       VerifyMode `json:"mode"`
	Cron       string     `json:"cron"`
	Enabled    bool       `json:"enabled"`
	LastRun    *time.Time `json:"last_run"`
	NextRun    *time.Time `json:"next_run"`
	CreatedAt  time.Time  `json:"created_at"`
}

// IntegrityReport 完整性报告
type IntegrityReport struct {
	BackupID      string    `json:"backup_id"`
	TotalChecks   int       `json:"total_checks"`
	PassedChecks  int       `json:"passed_checks"`
	FailedChecks  int       `json:"failed_checks"`
	LastCheck     time.Time `json:"last_check"`
	OverallStatus string    `json:"overall_status"`
	HealthScore   int       `json:"health_score"` // 0-100
}

// Manager 备份验证管理器
type Manager struct {
	mu        sync.RWMutex
	tasks     map[string]*VerifyTask
	schedules map[string]*VerifySchedule
	reports   map[string]*IntegrityReport
}

// NewManager 创建备份验证管理器
func NewManager() *Manager {
	return &Manager{
		tasks:     make(map[string]*VerifyTask),
		schedules: make(map[string]*VerifySchedule),
		reports:   make(map[string]*IntegrityReport),
	}
}

// RunVerification 运行验证
func (m *Manager) RunVerification(backupID, backupPath string, mode VerifyMode) (*VerifyTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if backupID == "" {
		return nil, fmt.Errorf("备份ID不能为空")
	}

	taskID := fmt.Sprintf("verify_%d", time.Now().UnixNano())
	task := &VerifyTask{
		ID:         taskID,
		BackupID:   backupID,
		BackupPath: backupPath,
		Mode:       mode,
		Status:     VerifyRunning,
		StartedAt:  time.Now(),
		Errors:     make([]VerifyError, 0),
	}
	m.tasks[taskID] = task

	go m.runVerify(task)
	return task, nil
}

func (m *Manager) runVerify(task *VerifyTask) {
	// 模拟验证过程
	task.TotalFiles = 1500
	task.TotalSize = 50 * 1024 * 1024 * 1024

	switch task.Mode {
	case ModeChecksum:
		task.CheckedFiles = 1500
		task.FailedFiles = 0
	case ModeDeep:
		task.CheckedFiles = 1500
		task.FailedFiles = 2
		task.Errors = []VerifyError{
			{File: "/backup/data/file1.dat", Error: "校验和不匹配", Expected: "abc123", Actual: "def456"},
			{File: "/backup/data/file2.dat", Error: "文件损坏"},
		}
	case ModeRestore:
		task.CheckedFiles = 1500
		task.FailedFiles = 0
	}

	now := time.Now()
	task.CompletedAt = &now
	task.Duration = now.Sub(task.StartedAt).String()
	if task.FailedFiles > 0 {
		task.Status = VerifyFailed
	} else {
		task.Status = VerifyPassed
	}

	// 更新报告
	m.mu.Lock()
	m.reports[task.BackupID] = &IntegrityReport{
		BackupID:      task.BackupID,
		TotalChecks:   task.TotalFiles,
		PassedChecks:  task.CheckedFiles - task.FailedFiles,
		FailedChecks:  task.FailedFiles,
		LastCheck:     now,
		OverallStatus: string(task.Status),
		HealthScore:   (task.CheckedFiles - task.FailedFiles) * 100 / task.TotalFiles,
	}
	m.mu.Unlock()
}

// GetTask 获取验证任务
func (m *Manager) GetTask(taskID string) (*VerifyTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}
	return task, nil
}

// ListTasks 列出验证任务
func (m *Manager) ListTasks() []*VerifyTask {
	m.mu.RLock()
	defer m.mu.RUnlock()
	tasks := make([]*VerifyTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// AddSchedule 添加验证计划
func (m *Manager) AddSchedule(schedule *VerifySchedule) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if schedule.ID == "" {
		schedule.ID = fmt.Sprintf("sched_%d", time.Now().UnixNano())
	}
	schedule.CreatedAt = time.Now()
	m.schedules[schedule.ID] = schedule
	return nil
}

// GetSchedules 获取验证计划
func (m *Manager) GetSchedules() []*VerifySchedule {
	m.mu.RLock()
	defer m.mu.RUnlock()
	schedules := make([]*VerifySchedule, 0, len(m.schedules))
	for _, s := range m.schedules {
		schedules = append(schedules, s)
	}
	return schedules
}

// GetReport 获取完整性报告
func (m *Manager) GetReport(backupID string) (*IntegrityReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	report, ok := m.reports[backupID]
	if !ok {
		return nil, fmt.Errorf("报告不存在: %s", backupID)
	}
	return report, nil
}

// GenerateChecksum 生成校验和
func GenerateChecksum(data []byte) string {
	hash := sha256.Sum256(data)
	return fmt.Sprintf("%x", hash)
}
