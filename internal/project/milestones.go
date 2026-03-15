// Package project provides milestone management functionality
package project

import (
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// MilestoneStatus 里程碑状态
type MilestoneStatus string

const (
	MilestoneStatusPlanned   MilestoneStatus = "planned"
	MilestoneStatusActive    MilestoneStatus = "active"
	MilestoneStatusCompleted MilestoneStatus = "completed"
	MilestoneStatusCancelled MilestoneStatus = "cancelled"
)

// MilestoneManager 里程碑管理器
type MilestoneManager struct {
	mu      sync.RWMutex
	manager *Manager
}

// NewMilestoneManager 创建里程碑管理器
func NewMilestoneManager(mgr *Manager) *MilestoneManager {
	return &MilestoneManager{
		manager: mgr,
	}
}

// CreateMilestoneWithTasks 创建里程碑并关联任务
func (mm *MilestoneManager) CreateMilestoneWithTasks(name, description, projectID, createdBy string, dueDate *time.Time, taskIDs []string) (*Milestone, error) {
	milestone, err := mm.manager.CreateMilestone(name, description, projectID, createdBy, dueDate)
	if err != nil {
		return nil, err
	}

	// 关联任务
	for _, taskID := range taskIDs {
		updates := map[string]interface{}{
			"milestone_id": milestone.ID,
		}
		_, err := mm.manager.UpdateTask(taskID, createdBy, updates)
		if err != nil {
			// 继续处理其他任务
			continue
		}
	}

	return milestone, nil
}

// StartMilestone 启动里程碑
func (mm *MilestoneManager) StartMilestone(milestoneID, userID string) (*Milestone, error) {
	updates := map[string]interface{}{
		"status":     MilestoneStatusActive,
		"start_date": time.Now(),
	}
	return mm.manager.UpdateMilestone(milestoneID, updates)
}

// CompleteMilestone 完成里程碑
func (mm *MilestoneManager) CompleteMilestone(milestoneID, userID string) (*Milestone, error) {
	// 检查是否所有任务都已完成
	stats := mm.GetMilestoneStats(milestoneID)
	if stats.IncompleteTasks > 0 {
		return nil, errors.New("还有未完成的任务，无法完成里程碑")
	}

	now := time.Now()
	updates := map[string]interface{}{
		"status":       MilestoneStatusCompleted,
		"completed_at": now,
	}
	return mm.manager.UpdateMilestone(milestoneID, updates)
}

// CancelMilestone 取消里程碑
func (mm *MilestoneManager) CancelMilestone(milestoneID, userID string) (*Milestone, error) {
	updates := map[string]interface{}{
		"status": MilestoneStatusCancelled,
	}
	return mm.manager.UpdateMilestone(milestoneID, updates)
}

// MilestoneStats 里程碑统计
type MilestoneStats struct {
	TotalTasks      int            `json:"total_tasks"`
	CompletedTasks  int            `json:"completed_tasks"`
	IncompleteTasks int            `json:"incomplete_tasks"`
	ByStatus        map[string]int `json:"by_status"`
	ByPriority      map[string]int `json:"by_priority"`
	Progress        int            `json:"progress"` // 0-100
	OverdueTasks    int            `json:"overdue_tasks"`
	OnTrack         bool           `json:"on_track"` // 是否按计划进行
}

// GetMilestoneStats 获取里程碑统计
func (mm *MilestoneManager) GetMilestoneStats(milestoneID string) MilestoneStats {
	mm.mu.RLock()
	defer mm.mu.RUnlock()

	stats := MilestoneStats{
		ByStatus:   make(map[string]int),
		ByPriority: make(map[string]int),
	}

	filter := TaskFilter{
		MilestoneID: milestoneID,
		Limit:       10000,
	}
	tasks := mm.manager.ListTasks(filter)

	now := time.Now()
	for _, task := range tasks {
		stats.TotalTasks++
		stats.ByStatus[string(task.Status)]++
		stats.ByPriority[string(task.Priority)]++

		if task.Status == TaskStatusDone {
			stats.CompletedTasks++
		} else {
			stats.IncompleteTasks++
		}

		// 过期任务
		if task.DueDate != nil && task.DueDate.Before(now) && task.Status != TaskStatusDone {
			stats.OverdueTasks++
		}
	}

	// 计算进度
	if stats.TotalTasks > 0 {
		stats.Progress = (stats.CompletedTasks * 100) / stats.TotalTasks
	}

	// 判断是否按计划进行
	milestone, err := mm.manager.GetMilestone(milestoneID)
	if err == nil && milestone.DueDate != nil {
		// 如果距离截止日期不到一半时间，但完成度不到一半，则不在轨道上
		totalDuration := milestone.DueDate.Sub(milestone.CreatedAt)
		elapsed := now.Sub(milestone.CreatedAt)
		expectedProgress := int((elapsed.Seconds() / totalDuration.Seconds()) * 100)
		stats.OnTrack = stats.Progress >= expectedProgress-10 // 允许10%的偏差
	} else {
		stats.OnTrack = true
	}

	return stats
}

// GetMilestoneTasks 获取里程碑任务
func (mm *MilestoneManager) GetMilestoneTasks(milestoneID string, limit, offset int) []*Task {
	filter := TaskFilter{
		MilestoneID: milestoneID,
		Limit:       limit,
		Offset:      offset,
		OrderBy:     "priority",
		OrderDesc:   true,
	}
	return mm.manager.ListTasks(filter)
}

// ProgressRecord 里程碑进度记录
type ProgressRecord struct {
	ID          string    `json:"id"`
	MilestoneID string    `json:"milestone_id"`
	Date        time.Time `json:"date"`
	Progress    int       `json:"progress"`
	TaskCount   int       `json:"task_count"`
	DoneCount   int       `json:"done_count"`
	RecordedBy  string    `json:"recorded_by"`
	Notes       string    `json:"notes,omitempty"`
}

// MilestoneProgressTracker 进度追踪器
type MilestoneProgressTracker struct {
	mu       sync.RWMutex
	progress map[string][]*ProgressRecord // milestoneID -> progress records
	manager  *Manager
}

// NewMilestoneProgressTracker 创建进度追踪器
func NewMilestoneProgressTracker(mgr *Manager) *MilestoneProgressTracker {
	return &MilestoneProgressTracker{
		progress: make(map[string][]*ProgressRecord),
		manager:  mgr,
	}
}

// RecordProgress 记录进度
func (mpt *MilestoneProgressTracker) RecordProgress(milestoneID, recordedBy, notes string) (*ProgressRecord, error) {
	mpt.mu.Lock()
	defer mpt.mu.Unlock()

	_, err := mpt.manager.GetMilestone(milestoneID)
	if err != nil {
		return nil, err
	}

	// 获取任务统计
	mm := NewMilestoneManager(mpt.manager)
	stats := mm.GetMilestoneStats(milestoneID)

	progress := &ProgressRecord{
		ID:          uuid.New().String(),
		MilestoneID: milestoneID,
		Date:        time.Now(),
		Progress:    stats.Progress,
		TaskCount:   stats.TotalTasks,
		DoneCount:   stats.CompletedTasks,
		RecordedBy:  recordedBy,
		Notes:       notes,
	}

	mpt.progress[milestoneID] = append(mpt.progress[milestoneID], progress)
	return progress, nil
}

// GetProgressHistory 获取进度历史
func (mpt *MilestoneProgressTracker) GetProgressHistory(milestoneID string) []*ProgressRecord {
	mpt.mu.RLock()
	defer mpt.mu.RUnlock()
	return append([]*ProgressRecord{}, mpt.progress[milestoneID]...)
}

// GetLatestProgress 获取最新进度
func (mpt *MilestoneProgressTracker) GetLatestProgress(milestoneID string) *ProgressRecord {
	mpt.mu.RLock()
	defer mpt.mu.RUnlock()

	records := mpt.progress[milestoneID]
	if len(records) == 0 {
		return nil
	}
	return records[len(records)-1]
}

// MilestoneBurndown 燃尽图数据
type MilestoneBurndown struct {
	MilestoneID string               `json:"milestone_id"`
	DataPoints  []BurndownDataPoint  `json:"data_points"`
	IdealLine   []BurndownIdealPoint `json:"ideal_line"`
}

// BurndownDataPoint 燃尽图数据点
type BurndownDataPoint struct {
	Date         time.Time `json:"date"`
	Remaining    int       `json:"remaining"`     // 剩余任务数
	RemainingPts int       `json:"remaining_pts"` // 剩余故事点（可选）
	Completed    int       `json:"completed"`     // 已完成任务数
	CompletedPts int       `json:"completed_pts"` // 已完成故事点（可选）
}

// BurndownIdealPoint 理想燃尽线数据点
type BurndownIdealPoint struct {
	Date      time.Time `json:"date"`
	Remaining int       `json:"remaining"`
}

// CalculateBurndown 计算燃尽图
func (mpt *MilestoneProgressTracker) CalculateBurndown(milestoneID string) (*MilestoneBurndown, error) {
	mpt.mu.RLock()
	defer mpt.mu.RUnlock()

	milestone, err := mpt.manager.GetMilestone(milestoneID)
	if err != nil {
		return nil, err
	}

	records := mpt.progress[milestoneID]
	burndown := &MilestoneBurndown{
		MilestoneID: milestoneID,
		DataPoints:  make([]BurndownDataPoint, 0),
		IdealLine:   make([]BurndownIdealPoint, 0),
	}

	if len(records) == 0 {
		return burndown, nil
	}

	// 计算实际数据点
	for _, record := range records {
		burndown.DataPoints = append(burndown.DataPoints, BurndownDataPoint{
			Date:      record.Date,
			Remaining: record.TaskCount - record.DoneCount,
			Completed: record.DoneCount,
		})
	}

	// 计算理想燃尽线
	if milestone != nil && milestone.DueDate != nil {
		totalTasks := records[0].TaskCount
		startDate := milestone.CreatedAt
		endDate := *milestone.DueDate
		duration := endDate.Sub(startDate).Hours() / 24 // 天数

		if duration > 0 {
			dailyBurn := float64(totalTasks) / duration
			for i := 0; i <= int(duration); i++ {
				date := startDate.AddDate(0, 0, i)
				remaining := totalTasks - int(float64(i)*dailyBurn)
				if remaining < 0 {
					remaining = 0
				}
				burndown.IdealLine = append(burndown.IdealLine, BurndownIdealPoint{
					Date:      date,
					Remaining: remaining,
				})
			}
		}
	}

	return burndown, nil
}

// MilestoneDependency 里程碑依赖
type MilestoneDependency struct {
	ID          string    `json:"id"`
	MilestoneID string    `json:"milestone_id"`
	DependsOnID string    `json:"depends_on_id"`
	Type        string    `json:"type"` // finish_to_start, start_to_start, finish_to_finish, start_to_finish
	CreatedAt   time.Time `json:"created_at"`
	CreatedBy   string    `json:"created_by"`
}

// DependencyManager 依赖管理器
type DependencyManager struct {
	mu           sync.RWMutex
	dependencies map[string][]*MilestoneDependency // milestoneID -> dependencies
}

// NewDependencyManager 创建依赖管理器
func NewDependencyManager() *DependencyManager {
	return &DependencyManager{
		dependencies: make(map[string][]*MilestoneDependency),
	}
}

// AddDependency 添加依赖
func (dm *DependencyManager) AddDependency(milestoneID, dependsOnID, depType, createdBy string) (*MilestoneDependency, error) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	// 检查循环依赖
	if dm.hasCycle(milestoneID, dependsOnID) {
		return nil, errors.New("检测到循环依赖")
	}

	dep := &MilestoneDependency{
		ID:          uuid.New().String(),
		MilestoneID: milestoneID,
		DependsOnID: dependsOnID,
		Type:        depType,
		CreatedAt:   time.Now(),
		CreatedBy:   createdBy,
	}

	dm.dependencies[milestoneID] = append(dm.dependencies[milestoneID], dep)
	return dep, nil
}

// RemoveDependency 移除依赖
func (dm *DependencyManager) RemoveDependency(milestoneID, dependsOnID string) {
	dm.mu.Lock()
	defer dm.mu.Unlock()

	deps := dm.dependencies[milestoneID]
	newDeps := make([]*MilestoneDependency, 0)
	for _, d := range deps {
		if d.DependsOnID != dependsOnID {
			newDeps = append(newDeps, d)
		}
	}
	dm.dependencies[milestoneID] = newDeps
}

// GetDependencies 获取里程碑依赖
func (dm *DependencyManager) GetDependencies(milestoneID string) []*MilestoneDependency {
	dm.mu.RLock()
	defer dm.mu.RUnlock()
	return append([]*MilestoneDependency{}, dm.dependencies[milestoneID]...)
}

// GetDependents 获取依赖此里程碑的其他里程碑
func (dm *DependencyManager) GetDependents(milestoneID string) []*MilestoneDependency {
	dm.mu.RLock()
	defer dm.mu.RUnlock()

	result := make([]*MilestoneDependency, 0)
	for _, deps := range dm.dependencies {
		for _, dep := range deps {
			if dep.DependsOnID == milestoneID {
				result = append(result, dep)
			}
		}
	}
	return result
}

// hasCycle 检查是否存在循环依赖
func (dm *DependencyManager) hasCycle(startID, targetID string) bool {
	visited := make(map[string]bool)
	return dm.detectCycle(targetID, startID, visited)
}

// detectCycle 深度优先检测循环
func (dm *DependencyManager) detectCycle(currentID, targetID string, visited map[string]bool) bool {
	if currentID == targetID {
		return true
	}
	if visited[currentID] {
		return false
	}
	visited[currentID] = true

	for _, dep := range dm.dependencies[currentID] {
		if dm.detectCycle(dep.DependsOnID, targetID, visited) {
			return true
		}
	}
	return false
}
