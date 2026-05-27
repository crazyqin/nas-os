// Package photodedup 提供照片重复检测核心业务逻辑
package photodedup

import (
	"fmt"
	"math/bits"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 照片去重管理器.
type Manager struct {
	tasks     map[string]*ScanTask
	groups    map[string]map[string]*DuplicateGroup // taskID -> groupID -> group
	photos    map[string]map[string]*PhotoInfo       // taskID -> photoID -> photo
	schedule  *ScheduleConfig
	mu        sync.RWMutex
}

// NewManager 创建照片去重管理器.
func NewManager() *Manager {
	return &Manager{
		tasks:  make(map[string]*ScanTask),
		groups: make(map[string]map[string]*DuplicateGroup),
		photos: make(map[string]map[string]*PhotoInfo),
	}
}

// ========== 任务管理 ==========

// StartScan 启动扫描任务.
func (m *Manager) StartScan(req StartScanRequest) (*ScanTask, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 参数校验
	if req.Threshold == 0 {
		req.Threshold = 90
	}
	if req.Threshold < 0 || req.Threshold > 100 {
		return nil, ErrInvalidThreshold
	}
	if req.Algorithm == "" {
		req.Algorithm = HashPHash
	}
	if !isValidAlgorithm(req.Algorithm) {
		return nil, ErrInvalidHashAlgorithm
	}

	now := time.Now()
	task := &ScanTask{
		ID:          uuid.New().String(),
		Status:      StatusRunning,
		ScanDirs:    req.ScanDirs,
		ExcludeDirs: req.ExcludeDirs,
		Threshold:   req.Threshold,
		Algorithm:   req.Algorithm,
		CreatedAt:   now,
		StartedAt:   &now,
		Progress:    0,
	}

	m.tasks[task.ID] = task
	m.groups[task.ID] = make(map[string]*DuplicateGroup)
	m.photos[task.ID] = make(map[string]*PhotoInfo)

	// 模拟后台扫描（实际实现需遍历文件系统）
	go m.runScan(task)

	return task, nil
}

// GetTask 获取扫描任务.
func (m *Manager) GetTask(taskID string) (*ScanTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListTasks 列出所有扫描任务.
func (m *Manager) ListTasks() []*ScanTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*ScanTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}

	sort.Slice(tasks, func(i, j int) bool {
		return tasks[i].CreatedAt.After(tasks[j].CreatedAt)
	})

	return tasks
}

// PauseTask 暂停扫描任务.
func (m *Manager) PauseTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if task.Status != StatusRunning {
		return ErrTaskNotRunning
	}

	task.Status = StatusPaused
	return nil
}

// ResumeTask 恢复扫描任务.
func (m *Manager) ResumeTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if task.Status != StatusPaused {
		return ErrTaskNotRunning
	}

	task.Status = StatusRunning
	go m.runScan(task)
	return nil
}

// CancelTask 取消扫描任务.
func (m *Manager) CancelTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return ErrTaskNotFound
	}
	if task.Status != StatusRunning && task.Status != StatusPaused {
		return ErrTaskNotRunning
	}

	now := time.Now()
	task.Status = StatusCancelled
	task.FinishedAt = &now
	return nil
}

// ========== 重复组查询 ==========

// GetDuplicateGroups 获取任务的重复组列表.
func (m *Manager) GetDuplicateGroups(taskID string) ([]*DuplicateGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.tasks[taskID]; !ok {
		return nil, ErrTaskNotFound
	}

	taskGroups, ok := m.groups[taskID]
	if !ok {
		return []*DuplicateGroup{}, nil
	}

	groups := make([]*DuplicateGroup, 0, len(taskGroups))
	for _, g := range taskGroups {
		groups = append(groups, g)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].WastedSize > groups[j].WastedSize
	})

	return groups, nil
}

// GetDuplicateGroup 获取单个重复组详情.
func (m *Manager) GetDuplicateGroup(taskID, groupID string) (*DuplicateGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	taskGroups, ok := m.groups[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}

	group, ok := taskGroups[groupID]
	if !ok {
		return nil, ErrGroupNotFound
	}

	return group, nil
}

// ========== 保留策略 ==========

// SetRetain 设置组内保留的照片.
func (m *Manager) SetRetain(taskID, groupID, photoID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	taskGroups, ok := m.groups[taskID]
	if !ok {
		return ErrTaskNotFound
	}

	group, ok := taskGroups[groupID]
	if !ok {
		return ErrGroupNotFound
	}

	// 验证 photoID 存在于该组
	found := false
	for _, p := range group.Photos {
		if p.ID == photoID {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("photo %q not in group %q", photoID, groupID)
	}

	group.RetainID = photoID
	return nil
}

// ========== 批量清理 ==========

// PreviewCleanup 预览批量清理结果.
func (m *Manager) PreviewCleanup(taskID string, req BatchCleanupRequest) (*CleanupPreview, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.tasks[taskID]; !ok {
		return nil, ErrTaskNotFound
	}

	if !isValidRetainPolicy(req.RetainPolicy) {
		return nil, ErrInvalidRetainPolicy
	}

	taskGroups := m.groups[taskID]
	preview := &CleanupPreview{}

	for _, groupID := range req.GroupIDs {
		group, ok := taskGroups[groupID]
		if !ok {
			continue
		}

		preview.GroupCount++

		// 根据保留策略选出保留照片
		retainPhoto := m.selectByPolicy(group, req.RetainPolicy)
		for _, p := range group.Photos {
			if p.ID != retainPhoto.ID {
				preview.DeleteCount++
				preview.ReclaimedSize += p.FileSize
			}
		}
	}

	return preview, nil
}

// ExecuteCleanup 执行批量清理.
func (m *Manager) ExecuteCleanup(taskID string, req BatchCleanupRequest) (*CleanupResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !req.Confirmed {
		return nil, ErrBatchNotConfirmed
	}

	if _, ok := m.tasks[taskID]; !ok {
		return nil, ErrTaskNotFound
	}

	if !isValidRetainPolicy(req.RetainPolicy) {
		return nil, ErrInvalidRetainPolicy
	}

	action := req.Action
	if action == "" {
		action = ActionTrash
	}

	taskGroups := m.groups[taskID]
	result := &CleanupResult{}

	for _, groupID := range req.GroupIDs {
		group, ok := taskGroups[groupID]
		if !ok {
			continue
		}

		retainPhoto := m.selectByPolicy(group, req.RetainPolicy)
		var remaining []*PhotoInfo

		for _, p := range group.Photos {
			if p.ID == retainPhoto.ID {
				remaining = append(remaining, p)
				continue
			}

			// 实际删除/移动操作（此处为内存模拟）
			result.DeletedCount++
			result.ReclaimedSize += p.FileSize
		}

		// 更新组：仅保留选中的照片
		if len(remaining) > 0 {
			group.Photos = remaining
			group.RetainID = remaining[0].ID
			group.WastedSize = 0
		} else {
			delete(taskGroups, groupID)
		}
	}

	return result, nil
}

// ========== 统计 ==========

// GetScanStats 获取扫描结果统计.
func (m *Manager) GetScanStats(taskID string) (*ScanStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, ErrTaskNotFound
	}

	taskGroups := m.groups[taskID]
	taskPhotos := m.photos[taskID]

	stats := &ScanStats{
		TotalScanned: len(taskPhotos),
		TotalGroups:  len(taskGroups),
	}

	for _, group := range taskGroups {
		stats.TotalDuplicates += len(group.Photos) - 1 // 每组去掉保留的那张
		stats.TotalWasted += group.WastedSize
	}

	// 也从 task 结构同步
	if task.TotalGroups > stats.TotalGroups {
		stats.TotalGroups = task.TotalGroups
	}
	if task.TotalWasted > stats.TotalWasted {
		stats.TotalWasted = task.TotalWasted
	}

	return stats, nil
}

// ========== 定时任务 ==========

// GetSchedule 获取定时扫描配置.
func (m *Manager) GetSchedule() *ScheduleConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.schedule == nil {
		return &ScheduleConfig{Enabled: false}
	}
	return m.schedule
}

// SetSchedule 设置定时扫描配置.
func (m *Manager) SetSchedule(cfg ScheduleConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if cfg.Threshold < 0 || cfg.Threshold > 100 {
		return ErrInvalidThreshold
	}
	if cfg.Threshold == 0 {
		cfg.Threshold = 90
	}
	if cfg.Algorithm == "" {
		cfg.Algorithm = HashPHash
	}
	if !isValidAlgorithm(cfg.Algorithm) {
		return ErrInvalidHashAlgorithm
	}

	m.schedule = &cfg
	return nil
}

// ========== 内部方法 ==========

// runScan 模拟执行扫描任务（后台协程）.
func (m *Manager) runScan(task *ScanTask) {
	// 模拟扫描过程：分 10 步推进进度
	totalSteps := 10
	for i := 0; i < totalSteps; i++ {
		time.Sleep(100 * time.Millisecond) // 模拟耗时

		m.mu.Lock()
		// 检查是否被暂停或取消
		if task.Status == StatusCancelled {
			m.mu.Unlock()
			return
		}
		if task.Status == StatusPaused {
			m.mu.Unlock()
			// 等待恢复
			for {
				time.Sleep(200 * time.Millisecond)
				m.mu.RLock()
				if task.Status == StatusRunning {
					m.mu.RUnlock()
					break
				}
				if task.Status == StatusCancelled {
					m.mu.RUnlock()
					return
				}
				m.mu.RUnlock()
			}
			m.mu.Lock()
		}

		task.Progress = float64(i+1) / float64(totalSteps) * 100
		task.TotalFiles = (i + 1) * 100 // 模拟每步扫描 100 个文件
		m.mu.Unlock()
	}

	// 模拟生成重复组结果
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	task.Status = StatusCompleted
	task.FinishedAt = &now
	task.Progress = 100

	// 模拟生成一些测试数据
	m.generateMockResults(task.ID)
}

// generateMockResults 生成模拟扫描结果（测试用）.
func (m *Manager) generateMockResults(taskID string) {
	taskPhotos := m.photos[taskID]
	taskGroups := m.groups[taskID]

	// 生成模拟照片
	photos := make([]*PhotoInfo, 5)
	for i := 0; i < 5; i++ {
		p := &PhotoInfo{
			ID:       uuid.New().String(),
			FilePath: fmt.Sprintf("/photos/img_%d.jpg", i),
			FileName: fmt.Sprintf("img_%d.jpg", i),
			FileSize: int64((i + 1) * 1024 * 1024), // 1MB, 2MB, 3MB, 4MB, 5MB
			Width:    1920,
			Height:   1080,
			ModTime:  time.Now().Add(-time.Duration(i) * 24 * time.Hour),
			HashValue: uint64(0xFF00FF00FF00FF00),
			BlurScore: float64(100 + i*50),
			ThumbnailURL: fmt.Sprintf("/api/v1/photo-dedup/thumbnails/%s", uuid.New().String()),
		}
		photos[i] = p
		taskPhotos[p.ID] = p
	}

	// 生成一个重复组（前 3 张照片为一组）
	group := &DuplicateGroup{
		ID:         uuid.New().String(),
		Similarity: 95.5,
		Photos:     photos[:3],
		RetainID:   photos[2].ID, // 保留最大的
		TotalSize:  photos[0].FileSize + photos[1].FileSize + photos[2].FileSize,
		WastedSize: photos[0].FileSize + photos[1].FileSize,
	}
	taskGroups[group.ID] = group

	// 更新任务统计
	task := m.tasks[taskID]
	task.TotalGroups = len(taskGroups)
	task.TotalWasted = group.WastedSize
}

// ========== 辅助函数 ==========

// selectByPolicy 根据保留策略选择保留照片.
func (m *Manager) selectByPolicy(group *DuplicateGroup, policy RetainPolicy) *PhotoInfo {
	if len(group.Photos) == 0 {
		return nil
	}

	switch policy {
	case RetainLargest:
		return selectMax(group.Photos, func(p *PhotoInfo) int64 { return p.FileSize })
	case RetainSmallest:
		return selectMin(group.Photos, func(p *PhotoInfo) int64 { return p.FileSize })
	case RetainNewest:
		return selectMax(group.Photos, func(p *PhotoInfo) int64 { return p.ModTime.Unix() })
	case RetainOldest:
		return selectMin(group.Photos, func(p *PhotoInfo) int64 { return p.ModTime.Unix() })
	case RetainSharpest:
		return selectMaxFloat(group.Photos, func(p *PhotoInfo) float64 { return p.BlurScore })
	case RetainManual:
		// 手动模式：优先使用组的 RetainID，否则默认选第一张
		if group.RetainID != "" {
			for _, p := range group.Photos {
				if p.ID == group.RetainID {
					return p
				}
			}
		}
		return group.Photos[0]
	default:
		return group.Photos[0]
	}
}

// selectMax 选择属性最大的照片.
func selectMax(photos []*PhotoInfo, fn func(*PhotoInfo) int64) *PhotoInfo {
	best := photos[0]
	bestVal := fn(best)
	for _, p := range photos[1:] {
		if v := fn(p); v > bestVal {
			best = p
			bestVal = v
		}
	}
	return best
}

// selectMin 选择属性最小的照片.
func selectMin(photos []*PhotoInfo, fn func(*PhotoInfo) int64) *PhotoInfo {
	best := photos[0]
	bestVal := fn(best)
	for _, p := range photos[1:] {
		if v := fn(p); v < bestVal {
			best = p
			bestVal = v
		}
	}
	return best
}

// selectMaxFloat 选择浮点属性最大的照片.
func selectMaxFloat(photos []*PhotoInfo, fn func(*PhotoInfo) float64) *PhotoInfo {
	best := photos[0]
	bestVal := fn(best)
	for _, p := range photos[1:] {
		if v := fn(p); v > bestVal {
			best = p
			bestVal = v
		}
	}
	return best
}

// isValidAlgorithm 校验哈希算法是否有效.
func isValidAlgorithm(alg HashAlgorithm) bool {
	switch alg {
	case HashPHash, HashDHash, HashAHash:
		return true
	default:
		return false
	}
}

// isValidRetainPolicy 校验保留策略是否有效.
func isValidRetainPolicy(policy RetainPolicy) bool {
	switch policy {
	case RetainLargest, RetainSmallest, RetainNewest, RetainOldest, RetainSharpest, RetainManual:
		return true
	default:
		return false
	}
}

// HammingDistance 计算两个哈希值的汉明距离.
func HammingDistance(a, b uint64) int {
	return bits.OnesCount64(a ^ b)
}

// SimilarityFromHamming 根据汉明距离计算相似度百分比（基于 64 位哈希）.
func SimilarityFromHamming(distance int) float64 {
	return float64(64-distance) / 64.0 * 100.0
}

// IsSupportedImage 检查文件扩展名是否为支持的图片格式.
func IsSupportedImage(filename string) bool {
	supported := []string{".jpg", ".jpeg", ".png", ".gif", ".bmp", ".tiff", ".tif", ".webp", ".heic", ".heif"}
	lower := strings.ToLower(filename)
	for _, ext := range supported {
		if strings.HasSuffix(lower, ext) {
			return true
		}
	}
	return false
}
