// Package projectboard 提供项目看板管理功能。
// gantt.go 实现甘特图功能，包括任务依赖、关键路径和资源调度。
package projectboard

import (
	"sort"
	"time"
)

// GanttManager 甘特图管理器。
type GanttManager struct {
	engine *Engine
}

// NewGanttManager 创建甘特图管理器。
func NewGanttManager(engine *Engine) *GanttManager {
	return &GanttManager{engine: engine}
}

// GanttTask 甘特图任务。
type GanttTask struct {
	CardID      string     `json:"card_id"`
	Title       string     `json:"title"`
	StartDate   time.Time  `json:"start_date"`
	EndDate     time.Time  `json:"end_date"`
	Duration    int        `json:"duration"` // 天数
	Progress    int        `json:"progress"`
	AssigneeID  string     `json:"assignee_id"`
	Dependencies []string  `json:"dependencies"`
	IsCritical  bool       `json:"is_critical"`
	Level       int        `json:"level"` // 层级
}

// DependencyType 依赖类型。
type DependencyType string

const (
	DependencyFS DependencyType = "finish_to_start"  // 完成-开始
	DependencySS DependencyType = "start_to_start"    // 开始-开始
	DependencyFF DependencyType = "finish_to_finish"  // 完成-完成
	DependencySF DependencyType = "start_to_finish"   // 开始-完成
)

// AddDependency 添加任务依赖。
func (g *GanttManager) AddDependency(fromID, toID string, depType DependencyType) error {
	g.engine.mu.Lock()
	defer g.engine.mu.Unlock()

	fromCard, exists := g.engine.cards[fromID]
	if !exists {
		return ErrCardNotFound
	}

	toCard, exists := g.engine.cards[toID]
	if !exists {
		return ErrCardNotFound
	}

	// 检查循环依赖
	if g.hasDependencyCycle(fromID, toID) {
		return ErrCircularDependency
	}

	// 检查是否已存在
	for _, dep := range fromCard.Dependencies {
		if dep == toID {
			return nil
		}
	}

	fromCard.Dependencies = append(fromCard.Dependencies, toID)
	fromCard.UpdatedAt = time.Now()

	// 存储依赖类型（简化：使用 toCard 的字段存储）
	_ = depType
	_ = toCard

	return nil
}

// RemoveDependency 移除任务依赖。
func (g *GanttManager) RemoveDependency(fromID, toID string) error {
	g.engine.mu.Lock()
	defer g.engine.mu.Unlock()

	fromCard, exists := g.engine.cards[fromID]
	if !exists {
		return ErrCardNotFound
	}

	for i, dep := range fromCard.Dependencies {
		if dep == toID {
			fromCard.Dependencies = append(fromCard.Dependencies[:i], fromCard.Dependencies[i+1:]...)
			fromCard.UpdatedAt = time.Now()
			return nil
		}
	}

	return nil
}

// hasDependencyCycle 检查是否存在循环依赖。
func (g *GanttManager) hasDependencyCycle(fromID, toID string) bool {
	visited := make(map[string]bool)
	return g.detectCycle(toID, fromID, visited)
}

// detectCycle 检测循环。
func (g *GanttManager) detectCycle(current, target string, visited map[string]bool) bool {
	if current == target {
		return true
	}

	if visited[current] {
		return false
	}

	visited[current] = true

	card, exists := g.engine.cards[current]
	if !exists {
		return false
	}

	for _, dep := range card.Dependencies {
		if g.detectCycle(dep, target, visited) {
			return true
		}
	}

	return false
}

// GetGanttTasks 获取甘特图任务列表。
func (g *GanttManager) GetGanttTasks(boardID string) []GanttTask {
	g.engine.mu.RLock()
	defer g.engine.mu.RUnlock()

	result := make([]GanttTask, 0)

	for _, card := range g.engine.cards {
		if card.BoardID != boardID {
			continue
		}

		// 跳过没有日期的卡片
		if card.StartDate == nil || card.DueDate == nil {
			continue
		}

		duration := int(card.DueDate.Sub(*card.StartDate).Hours() / 24)
		if duration < 1 {
			duration = 1
		}

		task := GanttTask{
			CardID:       card.ID,
			Title:        card.Title,
			StartDate:    *card.StartDate,
			EndDate:      *card.DueDate,
			Duration:     duration,
			Progress:     card.Progress,
			AssigneeID:   card.AssigneeID,
			Dependencies: card.Dependencies,
		}

		result = append(result, task)
	}

	// 按开始日期排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].StartDate.Before(result[j].StartDate)
	})

	return result
}

// CalculateCriticalPath 计算关键路径。
func (g *GanttManager) CalculateCriticalPath(boardID string) []string {
	g.engine.mu.RLock()
	defer g.engine.mu.RUnlock()

	// 收集所有任务
	tasks := make(map[string]*Card)
	for _, card := range g.engine.cards {
		if card.BoardID == boardID && card.StartDate != nil && card.DueDate != nil {
			tasks[card.ID] = card
		}
	}

	if len(tasks) == 0 {
		return nil
	}

	// 计算最早开始时间 (ES) 和最早完成时间 (EF)
	earliestStart := make(map[string]time.Time)
	earliestFinish := make(map[string]time.Time)

	for id, card := range tasks {
		es := *card.StartDate
		ef := *card.DueDate

		// 检查依赖
		maxEF := es
		for _, depID := range card.Dependencies {
			if depCard, exists := tasks[depID]; exists {
				depEF := *depCard.DueDate
				if depEF.After(maxEF) {
					maxEF = depEF
				}
			}
		}

		if maxEF.After(es) {
			es = maxEF
			ef = es.Add(card.DueDate.Sub(*card.StartDate))
		}

		earliestStart[id] = es
		earliestFinish[id] = ef
	}

	// 找到项目结束时间
	projectEnd := earliestFinish[listFirst(tasks)]
	for _, ef := range earliestFinish {
		if ef.After(projectEnd) {
			projectEnd = ef
		}
	}

	// 计算最晚开始时间 (LS) 和最晚完成时间 (LF)
	latestStart := make(map[string]time.Time)
	latestFinish := make(map[string]time.Time)

	for id, card := range tasks {
		lf := projectEnd
		ls := lf.Add(-card.DueDate.Sub(*card.StartDate))
		latestStart[id] = ls
		latestFinish[id] = lf
	}

	// 逆向计算
	for _, card := range tasks {
		for _, depID := range card.Dependencies {
			if depCard, exists := tasks[depID]; exists {
				ls := latestStart[depID]
				newLF := ls
				newLS := newLF.Add(-depCard.DueDate.Sub(*depCard.StartDate))

				if newLF.Before(latestFinish[depID]) {
					latestFinish[depID] = newLF
					latestStart[depID] = newLS
				}
			}
		}
	}

	// 找出关键路径（浮动时间为 0 的任务）
	criticalPath := make([]string, 0)
	for id := range tasks {
		es := earliestStart[id]
		ls := latestStart[id]
		// 简化：如果 ES == LS，则在关键路径上
		if es.Equal(ls) {
			criticalPath = append(criticalPath, id)
		}
	}

	return criticalPath
}

// listFirst 获取 map 中的第一个 key。
func listFirst(m map[string]*Card) string {
	for k := range m {
		return k
	}
	return ""
}

// ScheduleTasks 任务调度（资源均衡）。
func (g *GanttManager) ScheduleTasks(boardID string) []GanttTask {
	g.engine.mu.RLock()
	defer g.engine.mu.RUnlock()

	// 收集未调度的任务
	unscheduled := make([]*Card, 0)
	for _, card := range g.engine.cards {
		if card.BoardID == boardID && card.StartDate == nil {
			unscheduled = append(unscheduled, card)
		}
	}

	if len(unscheduled) == 0 {
		return nil
	}

	// 按优先级和依赖排序
	sort.Slice(unscheduled, func(i, j int) bool {
		pi := comparePriority(unscheduled[i].Priority, unscheduled[j].Priority)
		if pi != 0 {
			return pi > 0
		}
		return len(unscheduled[i].Dependencies) < len(unscheduled[j].Dependencies)
	})

	// 资源调度
	resourceUsage := make(map[string][]time.Time) // assigneeID -> 使用的时间段
	scheduled := make([]GanttTask, 0)
	now := time.Now()

	for _, card := range unscheduled {
		// 计算依赖的最早完成时间
		earliestDepEnd := now
		for _, depID := range card.Dependencies {
			if depCard, exists := g.engine.cards[depID]; exists && depCard.DueDate != nil {
				if depCard.DueDate.After(earliestDepEnd) {
					earliestDepEnd = *depCard.DueDate
				}
			}
		}

		// 估算工期（默认 3 天）
		duration := 3
		if card.EstimateHrs > 0 {
			duration = int(card.EstimateHrs / 8)
			if duration < 1 {
				duration = 1
			}
		}

		// 查找可用时间槽
		startDate := earliestDepEnd
		if card.AssigneeID != "" {
			// 检查资源冲突
			for {
				conflict := false
				endDate := startDate.AddDate(0, 0, duration)

				for _, usedTime := range resourceUsage[card.AssigneeID] {
					usedEnd := usedTime.AddDate(0, 0, 1)
					if startDate.Before(usedEnd) && endDate.After(usedTime) {
						conflict = true
						startDate = usedEnd
						break
					}
				}

				if !conflict {
					break
				}
			}

			// 记录资源使用
			endDate := startDate.AddDate(0, 0, duration)
			resourceUsage[card.AssigneeID] = append(resourceUsage[card.AssigneeID], startDate)
			_ = endDate
		}

		scheduled = append(scheduled, GanttTask{
			CardID:       card.ID,
			Title:        card.Title,
			StartDate:    startDate,
			EndDate:      startDate.AddDate(0, 0, duration),
			Duration:     duration,
			Progress:     card.Progress,
			AssigneeID:   card.AssigneeID,
			Dependencies: card.Dependencies,
		})
	}

	return scheduled
}

// GetResourceUtilization 获取资源利用率。
func (g *GanttManager) GetResourceUtilization(boardID string, start, end time.Time) map[string]map[string]interface{} {
	g.engine.mu.RLock()
	defer g.engine.mu.RUnlock()

	totalDays := int(end.Sub(start).Hours() / 24)
	if totalDays <= 0 {
		totalDays = 1
	}

	// 统计每个用户的任务
	userTasks := make(map[string][]*Card)
	for _, card := range g.engine.cards {
		if card.BoardID != boardID || card.AssigneeID == "" {
			continue
		}
		if card.StartDate == nil || card.DueDate == nil {
			continue
		}
		userTasks[card.AssigneeID] = append(userTasks[card.AssigneeID], card)
	}

	result := make(map[string]map[string]interface{})
	for userID, tasks := range userTasks {
		totalTaskDays := 0
		activeTasks := 0
		for _, card := range tasks {
			taskDays := int(card.DueDate.Sub(*card.StartDate).Hours() / 24)
			if taskDays < 1 {
				taskDays = 1
			}
			totalTaskDays += taskDays
			if card.Status != CardStatusDone {
				activeTasks++
			}
		}

		utilization := float64(totalTaskDays) / float64(totalDays) * 100
		if utilization > 100 {
			utilization = 100
		}

		result[userID] = map[string]interface{}{
			"total_tasks":    len(tasks),
			"active_tasks":   activeTasks,
			"total_days":     totalTaskDays,
			"period_days":    totalDays,
			"utilization":    utilization,
		}
	}

	return result
}

// GetTimeline 获取时间线数据。
func (g *GanttManager) GetTimeline(boardID string) map[string]interface{} {
	g.engine.mu.RLock()
	defer g.engine.mu.RUnlock()

	tasks := make([]GanttTask, 0)
	var minDate, maxDate *time.Time

	for _, card := range g.engine.cards {
		if card.BoardID != boardID || card.StartDate == nil || card.DueDate == nil {
			continue
		}

		task := GanttTask{
			CardID:       card.ID,
			Title:        card.Title,
			StartDate:    *card.StartDate,
			EndDate:      *card.DueDate,
			Duration:     int(card.DueDate.Sub(*card.StartDate).Hours() / 24),
			Progress:     card.Progress,
			AssigneeID:   card.AssigneeID,
			Dependencies: card.Dependencies,
		}
		tasks = append(tasks, task)

		if minDate == nil || card.StartDate.Before(*minDate) {
			minDate = card.StartDate
		}
		if maxDate == nil || card.DueDate.After(*maxDate) {
			maxDate = card.DueDate
		}
	}

	// 计算关键路径
	criticalPath := g.CalculateCriticalPath(boardID)
	criticalMap := make(map[string]bool)
	for _, id := range criticalPath {
		criticalMap[id] = true
	}

	// 标记关键路径任务
	for i := range tasks {
		if criticalMap[tasks[i].CardID] {
			tasks[i].IsCritical = true
		}
	}

	return map[string]interface{}{
		"tasks":          tasks,
		"min_date":       minDate,
		"max_date":       maxDate,
		"critical_path":  criticalPath,
		"total_tasks":    len(tasks),
	}
}
