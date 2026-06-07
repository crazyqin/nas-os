// Package aifilecleaner 提供AI智能文件清理功能
package aifilecleaner

import (
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// CleanTask 清理任务（manager内部使用，与types.go的CleanupTask区分）
type CleanTask struct {
	ID          string     `json:"id"`
	Status      TaskStatus `json:"status"`
	Mode        DeleteMode `json:"mode"`
	Files       []string   `json:"files"`
	TotalSize   int64      `json:"total_size"`
	FreedSize   int64      `json:"freed_size"`
	Processed   int        `json:"processed"`
	Failed      int        `json:"failed"`
	Errors      []string   `json:"errors,omitempty"`
	StartedAt   time.Time  `json:"started_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
	Progress    float64    `json:"progress"`
}

// Manager AI文件清理管理器
type Manager struct {
	mu         sync.RWMutex
	config     *ScanConfig
	results    *ScanResult
	tasks      map[string]*CleanTask
	duplicates map[string][]*DuplicateGroup
}

// NewManager 创建管理器
func NewManager(config *ScanConfig) *Manager {
	if config == nil {
		config = &ScanConfig{
			RootPath:             "/",
			LargeFileThresholdMB: 100,
			StaleDays:            90,
			MaxDepth:             10,
		}
	}
	return &Manager{
		config:     config,
		tasks:      make(map[string]*CleanTask),
		duplicates: make(map[string][]*DuplicateGroup),
	}
}

// Scan 扫描文件系统
func (m *Manager) Scan() (*ScanResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	startTime := time.Now()
	result := &ScanResult{
		ScanStartedAt: startTime,
	}

	err := filepath.Walk(m.config.RootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 跳过无法访问的文件
		}

		// 检查深度
		depth := strings.Count(path, string(os.PathSeparator)) - strings.Count(m.config.RootPath, string(os.PathSeparator))
		if depth > m.config.MaxDepth {
			if info.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}

		result.TotalFiles++
		result.TotalSizeBytes += info.Size()

		fileInfo := &FileInfo{
			Path:      path,
			Name:      info.Name(),
			Size:      info.Size(),
			ModTime:   info.ModTime(),
			IsDir:     info.IsDir(),
			Extension: filepath.Ext(info.Name()),
		}

		// 检查大文件
		if !info.IsDir() && info.Size() > int64(m.config.LargeFileThresholdMB)*1024*1024 {
			result.LargeFiles = append(result.LargeFiles, *fileInfo)
		}

		// 检查陈旧文件
		if !info.IsDir() {
			daysSinceUse := int(time.Since(info.ModTime()).Hours() / 24)
			if daysSinceUse >= m.config.StaleDays {
				fileInfo.DaysSinceUse = daysSinceUse
				result.StaleFiles = append(result.StaleFiles, *fileInfo)
			}
		}

		return nil
	})

	result.DurationSeconds = time.Since(startTime).Seconds()
	result.ScanFinishedAt = time.Now()
	m.results = result

	return result, err
}

// FindDuplicates 查找重复文件
func (m *Manager) FindDuplicates(paths []string) (map[string][]*DuplicateGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	hashMap := make(map[string][]*FileInfo)

	for _, rootPath := range paths {
		err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
			if err != nil || info.IsDir() {
				return nil
			}

			// 只处理小于100MB的文件
			if info.Size() > 100*1024*1024 {
				return nil
			}

			hash, err := m.fileHash(path)
			if err != nil {
				return nil
			}

			fileInfo := &FileInfo{
				Path:    path,
				Name:    info.Name(),
				Size:    info.Size(),
				ModTime: info.ModTime(),
				Hash:    hash,
			}

			hashMap[hash] = append(hashMap[hash], fileInfo)
			return nil
		})

		if err != nil {
			return nil, err
		}
	}

	// 过滤出有重复的组
	duplicates := make(map[string][]*DuplicateGroup)
	for hash, files := range hashMap {
		if len(files) > 1 {
			group := &DuplicateGroup{
				Hash:        hash,
				Size:        files[0].Size,
				Count:       len(files),
				Files:       files,
				WastedBytes: int64(len(files)-1) * files[0].Size,
			}
			duplicates[hash] = append(duplicates[hash], group)
		}
	}

	m.duplicates = duplicates
	return duplicates, nil
}

// CreateCleanTask 创建清理任务
func (m *Manager) CreateCleanTask(files []string, mode DeleteMode) (*CleanTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(files) == 0 {
		return nil, ErrNoScanData
	}

	// 验证文件存在
	var totalSize int64
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			return nil, fmt.Errorf("%w: %s", ErrFileNotFound, f)
		}
		totalSize += info.Size()
	}

	task := &CleanTask{
		ID:        fmt.Sprintf("task_%d", time.Now().UnixNano()),
		Status:    TaskStatusPending,
		Mode:      mode,
		Files:     files,
		TotalSize: totalSize,
		StartedAt: time.Now(),
	}

	m.tasks[task.ID] = task
	return task, nil
}

// RunCleanTask 执行清理任务
func (m *Manager) RunCleanTask(taskID string) error {
	m.mu.Lock()
	task, exists := m.tasks[taskID]
	if !exists {
		m.mu.Unlock()
		return ErrTaskNotFound
	}
	if task.Status == TaskStatusRunning {
		m.mu.Unlock()
		return ErrTaskAlreadyRunning
	}
	task.Status = TaskStatusRunning
	m.mu.Unlock()

	go m.executeCleanTask(task)
	return nil
}

// executeCleanTask 执行清理任务
func (m *Manager) executeCleanTask(task *CleanTask) {
	for i, filePath := range task.Files {
		m.mu.Lock()
		if task.Status == TaskStatusCancelled {
			m.mu.Unlock()
			return
		}
		m.mu.Unlock()

		var err error
		if task.Mode == DeleteModeRecycle {
			// 移动到回收站
			err = m.moveToRecycle(filePath)
		} else {
			// 永久删除
			err = os.Remove(filePath)
		}

		m.mu.Lock()
		if err != nil {
			task.Failed++
			task.Errors = append(task.Errors, fmt.Sprintf("%s: %v", filePath, err))
		} else {
			task.Processed++
			info, _ := os.Stat(filePath)
			if info != nil {
				task.FreedSize += info.Size()
			}
		}
		task.Progress = float64(i+1) / float64(len(task.Files)) * 100
		m.mu.Unlock()
	}

	m.mu.Lock()
	now := time.Now()
	task.CompletedAt = &now
	if task.Failed > 0 && task.Processed == 0 {
		task.Status = TaskStatusFailed
	} else {
		task.Status = TaskStatusCompleted
	}
	m.mu.Unlock()
}

// GetTask 获取任务
func (m *Manager) GetTask(taskID string) (*CleanTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListTasks 列出所有任务
func (m *Manager) ListTasks() []*CleanTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*CleanTask, 0, len(m.tasks))
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// CancelTask 取消任务
func (m *Manager) CancelTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, exists := m.tasks[taskID]
	if !exists {
		return ErrTaskNotFound
	}
	if task.Status != TaskStatusRunning {
		return ErrTaskNotRunning
	}

	task.Status = TaskStatusCancelled
	return nil
}

// GetScanResult 获取扫描结果
func (m *Manager) GetScanResult() (*ScanResult, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.results == nil {
		return nil, ErrNoScanData
	}
	return m.results, nil
}

// fileHash 计算文件哈希
func (m *Manager) fileHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", h.Sum(nil)), nil
}

// moveToRecycle 移动到回收站
func (m *Manager) moveToRecycle(path string) error {
	recycleDir := filepath.Join(filepath.Dir(path), ".recycle")
	if err := os.MkdirAll(recycleDir, 0755); err != nil {
		return err
	}

	baseName := filepath.Base(path)
	destPath := filepath.Join(recycleDir, fmt.Sprintf("%s_%d", baseName, time.Now().UnixNano()))

	return os.Rename(path, destPath)
}
