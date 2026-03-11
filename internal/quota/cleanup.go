// Package quota 提供存储配额管理功能
package quota

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"time"

	"golang.org/x/sys/unix"
)

// CleanupManager 清理策略管理器
type CleanupManager struct {
	quotaMgr *Manager
	tasks    map[string]*CleanupTask
}

// NewCleanupManager 创建清理管理器
func NewCleanupManager(quotaMgr *Manager) *CleanupManager {
	return &CleanupManager{
		quotaMgr: quotaMgr,
		tasks:    make(map[string]*CleanupTask),
	}
}

// ========== 清理策略管理 ==========

// CreatePolicy 创建清理策略
func (m *CleanupManager) CreatePolicy(input CleanupPolicyInput) (*CleanupPolicy, error) {
	m.quotaMgr.mu.Lock()
	defer m.quotaMgr.mu.Unlock()

	// 验证卷存在
	if m.quotaMgr.storageMgr != nil {
		vol := m.quotaMgr.storageMgr.GetVolume(input.VolumeName)
		if vol == nil {
			return nil, ErrVolumeNotFound
		}
	}

	// 验证策略参数
	if err := m.validatePolicyInput(&input); err != nil {
		return nil, err
	}

	policy := &CleanupPolicy{
		ID:           generateID(),
		Name:         input.Name,
		VolumeName:   input.VolumeName,
		Path:         input.Path,
		Type:         input.Type,
		Action:       input.Action,
		Enabled:      input.Enabled,
		MaxAge:       input.MaxAge,
		MinSize:      input.MinSize,
		Patterns:     input.Patterns,
		QuotaPercent: input.QuotaPercent,
		MaxAccessAge: input.MaxAccessAge,
		ArchivePath:  input.ArchivePath,
		MovePath:     input.MovePath,
		Schedule:     input.Schedule,
		RetentionDays: input.RetentionDays,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	m.quotaMgr.policies[policy.ID] = policy
	m.quotaMgr.saveConfig()

	return policy, nil
}

// GetPolicy 获取清理策略
func (m *CleanupManager) GetPolicy(id string) (*CleanupPolicy, error) {
	m.quotaMgr.mu.RLock()
	defer m.quotaMgr.mu.RUnlock()

	policy, exists := m.quotaMgr.policies[id]
	if !exists {
		return nil, ErrCleanupPolicyNotFound
	}
	return policy, nil
}

// ListPolicies 列出所有清理策略
func (m *CleanupManager) ListPolicies() []*CleanupPolicy {
	m.quotaMgr.mu.RLock()
	defer m.quotaMgr.mu.RUnlock()

	result := make([]*CleanupPolicy, 0, len(m.quotaMgr.policies))
	for _, p := range m.quotaMgr.policies {
		result = append(result, p)
	}
	return result
}

// UpdatePolicy 更新清理策略
func (m *CleanupManager) UpdatePolicy(id string, input CleanupPolicyInput) (*CleanupPolicy, error) {
	m.quotaMgr.mu.Lock()
	defer m.quotaMgr.mu.Unlock()

	policy, exists := m.quotaMgr.policies[id]
	if !exists {
		return nil, ErrCleanupPolicyNotFound
	}

	if err := m.validatePolicyInput(&input); err != nil {
		return nil, err
	}

	policy.Name = input.Name
	policy.Path = input.Path
	policy.Type = input.Type
	policy.Action = input.Action
	policy.Enabled = input.Enabled
	policy.MaxAge = input.MaxAge
	policy.MinSize = input.MinSize
	policy.Patterns = input.Patterns
	policy.QuotaPercent = input.QuotaPercent
	policy.MaxAccessAge = input.MaxAccessAge
	policy.ArchivePath = input.ArchivePath
	policy.MovePath = input.MovePath
	policy.Schedule = input.Schedule
	policy.RetentionDays = input.RetentionDays
	policy.UpdatedAt = time.Now()

	m.quotaMgr.saveConfig()
	return policy, nil
}

// DeletePolicy 删除清理策略
func (m *CleanupManager) DeletePolicy(id string) error {
	m.quotaMgr.mu.Lock()
	defer m.quotaMgr.mu.Unlock()

	if _, exists := m.quotaMgr.policies[id]; !exists {
		return ErrCleanupPolicyNotFound
	}

	delete(m.quotaMgr.policies, id)
	m.quotaMgr.saveConfig()
	return nil
}

// EnablePolicy 启用/禁用策略
func (m *CleanupManager) EnablePolicy(id string, enabled bool) error {
	m.quotaMgr.mu.Lock()
	defer m.quotaMgr.mu.Unlock()

	policy, exists := m.quotaMgr.policies[id]
	if !exists {
		return ErrCleanupPolicyNotFound
	}

	policy.Enabled = enabled
	policy.UpdatedAt = time.Now()
	m.quotaMgr.saveConfig()
	return nil
}

// validatePolicyInput 验证策略输入
func (m *CleanupManager) validatePolicyInput(input *CleanupPolicyInput) error {
	switch input.Type {
	case CleanupPolicyAge:
		if input.MaxAge <= 0 {
			return fmt.Errorf("年龄策略需要指定 MaxAge")
		}
	case CleanupPolicySize:
		if input.MinSize <= 0 {
			return fmt.Errorf("大小策略需要指定 MinSize")
		}
	case CleanupPolicyPattern:
		if len(input.Patterns) == 0 {
			return fmt.Errorf("模式策略需要指定 Patterns")
		}
	case CleanupPolicyQuota:
		if input.QuotaPercent <= 0 || input.QuotaPercent > 100 {
			return fmt.Errorf("配额策略需要指定有效的 QuotaPercent (0-100)")
		}
	case CleanupPolicyAccess:
		if input.MaxAccessAge <= 0 {
			return fmt.Errorf("访问时间策略需要指定 MaxAccessAge")
		}
	}

	// 验证动作目标
	if input.Action == CleanupActionArchive && input.ArchivePath == "" {
		return fmt.Errorf("归档动作需要指定 ArchivePath")
	}
	if input.Action == CleanupActionMove && input.MovePath == "" {
		return fmt.Errorf("移动动作需要指定 MovePath")
	}

	return nil
}

// ========== 清理执行 ==========

// RunPolicy 执行清理策略
func (m *CleanupManager) RunPolicy(policyID string) (*CleanupTask, error) {
	m.quotaMgr.mu.RLock()
	policy, exists := m.quotaMgr.policies[policyID]
	m.quotaMgr.mu.RUnlock()

	if !exists {
		return nil, ErrCleanupPolicyNotFound
	}

	return m.executePolicy(policy)
}

// executePolicy 执行清理策略
func (m *CleanupManager) executePolicy(policy *CleanupPolicy) (*CleanupTask, error) {
	// 创建任务记录
	task := &CleanupTask{
		ID:        generateID(),
		PolicyID:  policy.ID,
		PolicyName: policy.Name,
		VolumeName: policy.VolumeName,
		Path:      policy.Path,
		Status:    CleanupTaskRunning,
		StartedAt: time.Now(),
		Errors:    make([]string, 0),
	}

	m.tasks[task.ID] = task

	// 获取目标路径
	targetPath := policy.Path
	if targetPath == "" && m.quotaMgr.storageMgr != nil {
		vol := m.quotaMgr.storageMgr.GetVolume(policy.VolumeName)
		if vol != nil {
			targetPath = vol.MountPoint
		}
	}

	if targetPath == "" {
		task.Status = CleanupTaskFailed
		task.Errors = append(task.Errors, "无法确定目标路径")
		now := time.Now()
		task.CompletedAt = &now
		return task, fmt.Errorf("无法确定目标路径")
	}

	// 查找符合条件的文件
	files, err := m.findFiles(targetPath, policy)
	if err != nil {
		task.Status = CleanupTaskFailed
		task.Errors = append(task.Errors, err.Error())
		now := time.Now()
		task.CompletedAt = &now
		return task, err
	}

	// 执行清理动作
	for _, file := range files {
		if err := m.processFile(file, policy); err != nil {
			task.Errors = append(task.Errors, fmt.Sprintf("%s: %v", file, err))
			continue
		}
		task.FilesProcessed++
		// 获取文件大小
		if info, err := os.Stat(file); err == nil {
			task.BytesFreed += uint64(info.Size())
		}
	}

	// 更新任务状态
	task.Status = CleanupTaskCompleted
	now := time.Now()
	task.CompletedAt = &now

	return task, nil
}

// findFiles 查找符合条件的文件
func (m *CleanupManager) findFiles(rootPath string, policy *CleanupPolicy) ([]string, error) {
	var files []string
	now := time.Now()

	err := filepath.WalkDir(rootPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过错误
		}

		if d.IsDir() {
			return nil
		}

		info, err := d.Info()
		if err != nil {
			return nil
		}

		match := false
		switch policy.Type {
		case CleanupPolicyAge:
			// 按文件修改时间
			fileAge := now.Sub(info.ModTime()).Hours() / 24
			if int(fileAge) > policy.MaxAge {
				match = true
			}

		case CleanupPolicySize:
			// 按文件大小
			if uint64(info.Size()) >= policy.MinSize {
				match = true
			}

		case CleanupPolicyPattern:
			// 按文件名模式
			for _, pattern := range policy.Patterns {
				matched, _ := filepath.Match(pattern, d.Name())
				if matched {
					match = true
					break
				}
				// 也支持正则表达式
				if reg, err := regexp.Compile(pattern); err == nil {
					if reg.MatchString(d.Name()) {
						match = true
						break
					}
				}
			}

		case CleanupPolicyQuota:
			// 按配额比例触发，需要额外处理
			// 这里简化为查找最大的文件
			match = true

		case CleanupPolicyAccess:
			// 按访问时间
			if stat, ok := info.Sys().(*unix.Stat_t); ok {
				atime := time.Unix(int64(stat.Atim.Sec), int64(stat.Atim.Nsec))
				accessAge := now.Sub(atime).Hours() / 24
				if int(accessAge) > policy.MaxAccessAge {
					match = true
				}
			}
		}

		if match {
			files = append(files, path)
		}

		return nil
	})

	return files, err
}

// processFile 处理单个文件
func (m *CleanupManager) processFile(filePath string, policy *CleanupPolicy) error {
	switch policy.Action {
	case CleanupActionDelete:
		return os.Remove(filePath)

	case CleanupActionArchive:
		// 创建归档目录
		if err := os.MkdirAll(policy.ArchivePath, 0755); err != nil {
			return err
		}
		// 移动文件
		destPath := filepath.Join(policy.ArchivePath, filepath.Base(filePath))
		return os.Rename(filePath, destPath)

	case CleanupActionMove:
		// 创建目标目录
		if err := os.MkdirAll(policy.MovePath, 0755); err != nil {
			return err
		}
		// 移动文件
		destPath := filepath.Join(policy.MovePath, filepath.Base(filePath))
		return os.Rename(filePath, destPath)
	}

	return fmt.Errorf("未知清理动作: %s", policy.Action)
}

// RunAutoCleanup 运行自动清理（检查配额并执行清理）
func (m *CleanupManager) RunAutoCleanup() ([]*CleanupTask, error) {
	var tasks []*CleanupTask

	// 获取所有配额使用情况
	usages, err := m.quotaMgr.GetAllUsage()
	if err != nil {
		return nil, err
	}

	// 检查是否有超过阈值的配额
	for _, usage := range usages {
		if !usage.IsOverSoft {
			continue
		}

		// 查找相关的清理策略
		m.quotaMgr.mu.RLock()
		var relatedPolicies []*CleanupPolicy
		for _, p := range m.quotaMgr.policies {
			if p.Enabled && p.Type == CleanupPolicyQuota && 
			   p.VolumeName == usage.VolumeName {
				relatedPolicies = append(relatedPolicies, p)
			}
		}
		m.quotaMgr.mu.RUnlock()

		// 执行清理策略
		for _, policy := range relatedPolicies {
			if usage.UsagePercent >= policy.QuotaPercent {
				task, err := m.executePolicy(policy)
				if err != nil {
					continue
				}
				tasks = append(tasks, task)
			}
		}
	}

	return tasks, nil
}

// GetTask 获取清理任务
func (m *CleanupManager) GetTask(taskID string) (*CleanupTask, error) {
	task, exists := m.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务不存在")
	}
	return task, nil
}

// ListTasks 列出清理任务
func (m *CleanupManager) ListTasks(limit int) []*CleanupTask {
	tasks := make([]*CleanupTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}

	// 按时间排序
	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].StartedAt.After(tasks[j].StartedAt)
	})

	if limit > 0 && len(tasks) > limit {
		tasks = tasks[:limit]
	}

	return tasks
}

// CleanupOldTasks 清理旧任务记录
func (m *CleanupManager) CleanupOldTasks(maxAge time.Duration) int {
	cutoff := time.Now().Add(-maxAge)
	count := 0

	for id, task := range m.tasks {
		if task.CompletedAt != nil && task.CompletedAt.Before(cutoff) {
			delete(m.tasks, id)
			count++
		}
	}

	return count
}

// GetCleanupStats 获取清理统计
func (m *CleanupManager) GetCleanupStats() map[string]interface{} {
	m.quotaMgr.mu.RLock()
	policyCount := len(m.quotaMgr.policies)
	enabledCount := 0
	for _, p := range m.quotaMgr.policies {
		if p.Enabled {
			enabledCount++
		}
	}
	m.quotaMgr.mu.RUnlock()

	var totalFiles int
	var totalBytes uint64
	for _, task := range m.tasks {
		totalFiles += task.FilesProcessed
		totalBytes += task.BytesFreed
	}

	return map[string]interface{}{
		"total_policies":   policyCount,
		"enabled_policies": enabledCount,
		"total_tasks":      len(m.tasks),
		"total_files_cleaned": totalFiles,
		"total_bytes_freed":   totalBytes,
	}
}