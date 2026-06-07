package filesynchub

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// SyncTask represents a file synchronization task
type SyncTask struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Source      string        `json:"source"`
	Destination string        `json:"destination"`
	Mode        string        `json:"mode"` // mirror, backup, sync
	Status      string        `json:"status"`
	LastSync    time.Time     `json:"last_sync"`
	NextSync    time.Time     `json:"next_sync"`
	Interval    time.Duration `json:"interval"`
	FileCount   int64         `json:"file_count"`
	TotalSize   int64         `json:"total_size"`
}

// SyncResult represents the result of a sync operation
type SyncResult struct {
	TaskID       string    `json:"task_id"`
	StartTime    time.Time `json:"start_time"`
	EndTime      time.Time `json:"end_time"`
	FilesSynced  int       `json:"files_synced"`
	FilesSkipped int       `json:"files_skipped"`
	BytesTotal   int64     `json:"bytes_total"`
	Errors       []string  `json:"errors,omitempty"`
}

// ConflictResolution defines how to handle conflicts
type ConflictResolution struct {
	Strategy string `json:"strategy"` // source, destination, newest, largest, manual
}

// FileSyncHub provides file synchronization across devices
// Inspired by Synology Drive
type FileSyncHub struct {
	mu          sync.RWMutex
	tasks       map[string]*SyncTask
	running     bool
	stopCh      chan struct{}
	conflictRes ConflictResolution
}

// NewFileSyncHub creates a new FileSyncHub instance
func NewFileSyncHub() *FileSyncHub {
	return &FileSyncHub{
		tasks:       make(map[string]*SyncTask),
		stopCh:      make(chan struct{}),
		conflictRes: ConflictResolution{Strategy: "newest"},
	}
}

// AddTask adds a synchronization task
func (fsh *FileSyncHub) AddTask(task SyncTask) error {
	fsh.mu.Lock()
	defer fsh.mu.Unlock()

	if task.ID == "" {
		return fmt.Errorf("task ID is required")
	}
	if task.Source == "" || task.Destination == "" {
		return fmt.Errorf("source and destination are required")
	}
	if task.Interval == 0 {
		task.Interval = 1 * time.Hour
	}
	task.Status = "pending"
	fsh.tasks[task.ID] = &task
	return nil
}

// RemoveTask removes a synchronization task
func (fsh *FileSyncHub) RemoveTask(taskID string) {
	fsh.mu.Lock()
	defer fsh.mu.Unlock()
	delete(fsh.tasks, taskID)
}

// RunSync executes a sync task immediately
func (fsh *FileSyncHub) RunSync(ctx context.Context, taskID string) (*SyncResult, error) {
	fsh.mu.RLock()
	task, exists := fsh.tasks[taskID]
	fsh.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	result := &SyncResult{
		TaskID:    taskID,
		StartTime: time.Now(),
	}

	// Walk source directory
	err := filepath.Walk(task.Source, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		relPath, _ := filepath.Rel(task.Source, path)
		destPath := filepath.Join(task.Destination, relPath)

		if info.IsDir() {
			os.MkdirAll(destPath, info.Mode())
			return nil
		}

		// Check if file needs sync
		needsSync, err := fsh.needsSync(path, destPath)
		if err != nil {
			result.Errors = append(result.Errors, err.Error())
			return nil
		}

		if needsSync {
			err = fsh.copyFile(path, destPath)
			if err != nil {
				result.Errors = append(result.Errors, err.Error())
			} else {
				result.FilesSynced++
				result.BytesTotal += info.Size()
			}
		} else {
			result.FilesSkipped++
		}

		return nil
	})

	result.EndTime = time.Now()

	fsh.mu.Lock()
	task.Status = "completed"
	task.LastSync = time.Now()
	task.FileCount = int64(result.FilesSynced)
	fsh.mu.Unlock()

	if err != nil {
		return result, err
	}
	return result, nil
}

// GetTask returns a sync task
func (fsh *FileSyncHub) GetTask(taskID string) (*SyncTask, error) {
	fsh.mu.RLock()
	defer fsh.mu.RUnlock()

	task, exists := fsh.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}
	return task, nil
}

// ListTasks returns all sync tasks
func (fsh *FileSyncHub) ListTasks() []*SyncTask {
	fsh.mu.RLock()
	defer fsh.mu.RUnlock()

	tasks := make([]*SyncTask, 0, len(fsh.tasks))
	for _, t := range fsh.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// SetConflictResolution sets the conflict resolution strategy
func (fsh *FileSyncHub) SetConflictResolution(resolution ConflictResolution) {
	fsh.mu.Lock()
	defer fsh.mu.Unlock()
	fsh.conflictRes = resolution
}

// Start begins automatic synchronization
func (fsh *FileSyncHub) Start(ctx context.Context) error {
	fsh.mu.Lock()
	if fsh.running {
		fsh.mu.Unlock()
		return fmt.Errorf("already running")
	}
	fsh.running = true
	fsh.mu.Unlock()

	go fsh.syncLoop(ctx)
	return nil
}

// Stop stops automatic synchronization
func (fsh *FileSyncHub) Stop() {
	fsh.mu.Lock()
	defer fsh.mu.Unlock()
	if fsh.running {
		close(fsh.stopCh)
		fsh.running = false
	}
}

func (fsh *FileSyncHub) syncLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-fsh.stopCh:
			return
		case <-ticker.C:
			fsh.runPendingTasks(ctx)
		}
	}
}

func (fsh *FileSyncHub) runPendingTasks(ctx context.Context) {
	fsh.mu.RLock()
	var pendingTasks []string
	for id, task := range fsh.tasks {
		if task.NextSync.Before(time.Now()) {
			pendingTasks = append(pendingTasks, id)
		}
	}
	fsh.mu.RUnlock()

	for _, taskID := range pendingTasks {
		fsh.RunSync(ctx, taskID)
	}
}

func (fsh *FileSyncHub) needsSync(src, dst string) (bool, error) {
	srcInfo, err := os.Stat(src)
	if err != nil {
		return false, err
	}

	dstInfo, err := os.Stat(dst)
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}

	// Compare modification time
	if srcInfo.ModTime().After(dstInfo.ModTime()) {
		return true, nil
	}

	// Compare size
	if srcInfo.Size() != dstInfo.Size() {
		return true, nil
	}

	return false, nil
}

func (fsh *FileSyncHub) copyFile(src, dst string) error {
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

	_, err = io.Copy(dstFile, srcFile)
	return err
}

// ComputeFileHash computes SHA256 hash of a file
func ComputeFileHash(path string) (string, error) {
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
