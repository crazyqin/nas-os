// Package projectboard 提供项目看板管理功能。
// agile.go 实现敏捷管理功能，包括Sprint规划、燃尽图和速度统计。
package projectboard

import (
	"sort"
	"time"
)

// AgileManager 敏捷管理器。
type AgileManager struct {
	engine *Engine
}

// NewAgileManager 创建敏捷管理器。
func NewAgileManager(engine *Engine) *AgileManager {
	return &AgileManager{engine: engine}
}

// CreateSprint 创建迭代。
func (a *AgileManager) CreateSprint(projectID, name, goal string, startDate, endDate time.Time) (*Sprint, error) {
	a.engine.mu.Lock()
	defer a.engine.mu.Unlock()

	if _, exists := a.engine.projects[projectID]; !exists {
		return nil, ErrProjectNotFound
	}

	now := time.Now()
	sprint := &Sprint{
		ID:        generateID(),
		ProjectID: projectID,
		Name:      name,
		Goal:      goal,
		StartDate: startDate,
		EndDate:   endDate,
		Status:    "planned",
		CreatedAt: now,
		UpdatedAt: now,
	}

	a.engine.sprints[sprint.ID] = sprint
	return sprint, nil
}

// GetSprint 获取迭代。
func (a *AgileManager) GetSprint(id string) (*Sprint, error) {
	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()

	sprint, exists := a.engine.sprints[id]
	if !exists {
		return nil, ErrSprintNotFound
	}
	return sprint, nil
}

// ListSprints 列出迭代。
func (a *AgileManager) ListSprints(projectID string) []*Sprint {
	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()

	result := make([]*Sprint, 0)
	for _, sprint := range a.engine.sprints {
		if sprint.ProjectID == projectID {
			result = append(result, sprint)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].StartDate.Before(result[j].StartDate)
	})

	return result
}

// StartSprint 启动迭代。
func (a *AgileManager) StartSprint(id string) (*Sprint, error) {
	a.engine.mu.Lock()
	defer a.engine.mu.Unlock()

	sprint, exists := a.engine.sprints[id]
	if !exists {
		return nil, ErrSprintNotFound
	}

	if sprint.Status != "planned" {
		return nil, ErrInvalidTransition
	}

	sprint.Status = "active"
	sprint.UpdatedAt = time.Now()
	return sprint, nil
}

// CompleteSprint 完成迭代。
func (a *AgileManager) CompleteSprint(id string) (*Sprint, error) {
	a.engine.mu.Lock()
	defer a.engine.mu.Unlock()

	sprint, exists := a.engine.sprints[id]
	if !exists {
		return nil, ErrSprintNotFound
	}

	if sprint.Status != "active" {
		return nil, ErrInvalidTransition
	}

	sprint.Status = "completed"
	sprint.UpdatedAt = time.Now()

	// 计算完成的 Velocity
	completedPoints := 0
	for _, cardID := range sprint.CardIDs {
		if card, exists := a.engine.cards[cardID]; exists {
			if card.Status == CardStatusDone {
				completedPoints += card.StoryPoints
			}
		}
	}
	sprint.Velocity = completedPoints

	return sprint, nil
}

// AddCardToSprint 添加卡片到迭代。
func (a *AgileManager) AddCardToSprint(sprintID, cardID string) error {
	a.engine.mu.Lock()
	defer a.engine.mu.Unlock()

	sprint, exists := a.engine.sprints[sprintID]
	if !exists {
		return ErrSprintNotFound
	}

	card, exists := a.engine.cards[cardID]
	if !exists {
		return ErrCardNotFound
	}

	// 检查是否已存在
	for _, id := range sprint.CardIDs {
		if id == cardID {
			return nil
		}
	}

	sprint.CardIDs = append(sprint.CardIDs, cardID)
	card.SprintID = sprintID
	card.UpdatedAt = time.Now()
	sprint.UpdatedAt = time.Now()

	return nil
}

// RemoveCardFromSprint 从迭代移除卡片。
func (a *AgileManager) RemoveCardFromSprint(sprintID, cardID string) error {
	a.engine.mu.Lock()
	defer a.engine.mu.Unlock()

	sprint, exists := a.engine.sprints[sprintID]
	if !exists {
		return ErrSprintNotFound
	}

	card, exists := a.engine.cards[cardID]
	if !exists {
		return ErrCardNotFound
	}

	for i, id := range sprint.CardIDs {
		if id == cardID {
			sprint.CardIDs = append(sprint.CardIDs[:i], sprint.CardIDs[i+1:]...)
			card.SprintID = ""
			card.UpdatedAt = time.Now()
			sprint.UpdatedAt = time.Now()
			return nil
		}
	}

	return ErrCardNotInSprint
}

// GetSprintCards 获取迭代的卡片。
func (a *AgileManager) GetSprintCards(sprintID string) ([]*Card, error) {
	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()

	sprint, exists := a.engine.sprints[sprintID]
	if !exists {
		return nil, ErrSprintNotFound
	}

	result := make([]*Card, 0)
	for _, cardID := range sprint.CardIDs {
		if card, exists := a.engine.cards[cardID]; exists {
			result = append(result, card)
		}
	}

	return result, nil
}

// GetBurndownData 获取燃尽图数据。
func (a *AgileManager) GetBurndownData(sprintID string) ([]BurndownPoint, error) {
	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()

	sprint, exists := a.engine.sprints[sprintID]
	if !exists {
		return nil, ErrSprintNotFound
	}

	// 计算总点数
	totalPoints := 0
	for _, cardID := range sprint.CardIDs {
		if card, exists := a.engine.cards[cardID]; exists {
			totalPoints += card.StoryPoints
		}
	}

	// 生成燃尽图数据点
	points := make([]BurndownPoint, 0)
	currentDate := sprint.StartDate
	duration := sprint.EndDate.Sub(sprint.StartDate)
	days := int(duration.Hours() / 24)

	if days <= 0 {
		days = 1
	}

	idealSlope := float64(totalPoints) / float64(days)

	for i := 0; i <= days; i++ {
		date := currentDate.AddDate(0, 0, i)
		if date.After(sprint.EndDate) {
			date = sprint.EndDate
		}

		ideal := totalPoints - int(float64(i)*idealSlope)
		if ideal < 0 {
			ideal = 0
		}

		// 计算已完成点数（简化：按当前状态统计）
		completed := 0
		remaining := totalPoints
		for _, cardID := range sprint.CardIDs {
			if card, exists := a.engine.cards[cardID]; exists {
				if card.Status == CardStatusDone {
					completed += card.StoryPoints
				}
			}
		}
		remaining = totalPoints - completed

		points = append(points, BurndownPoint{
			Date:      date,
			Remaining: remaining,
			Ideal:     ideal,
			Completed: completed,
		})

		if date.Equal(sprint.EndDate) {
			break
		}
	}

	return points, nil
}

// GetVelocityData 获取速度统计数据。
func (a *AgileManager) GetVelocityData(projectID string) ([]VelocityData, error) {
	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()

	if _, exists := a.engine.projects[projectID]; !exists {
		return nil, ErrProjectNotFound
	}

	result := make([]VelocityData, 0)

	// 获取已完成的迭代
	sprints := make([]*Sprint, 0)
	for _, sprint := range a.engine.sprints {
		if sprint.ProjectID == projectID && sprint.Status == "completed" {
			sprints = append(sprints, sprint)
		}
	}

	// 按时间排序
	sort.Slice(sprints, func(i, j int) bool {
		return sprints[i].EndDate.Before(sprints[j].EndDate)
	})

	for _, sprint := range sprints {
		committed := 0
		completed := 0

		for _, cardID := range sprint.CardIDs {
			if card, exists := a.engine.cards[cardID]; exists {
				committed += card.StoryPoints
				if card.Status == CardStatusDone {
					completed += card.StoryPoints
				}
			}
		}

		completionRate := 0.0
		if committed > 0 {
			completionRate = float64(completed) / float64(committed) * 100
		}

		result = append(result, VelocityData{
			SprintID:       sprint.ID,
			SprintName:     sprint.Name,
			Committed:      committed,
			Completed:      completed,
			CompletionRate: completionRate,
		})
	}

	return result, nil
}

// GetAverageVelocity 获取平均速度。
func (a *AgileManager) GetAverageVelocity(projectID string, lastN int) (float64, error) {
	velocityData, err := a.GetVelocityData(projectID)
	if err != nil {
		return 0, err
	}

	if len(velocityData) == 0 {
		return 0, nil
	}

	// 取最后 N 个迭代
	start := 0
	if lastN > 0 && lastN < len(velocityData) {
		start = len(velocityData) - lastN
	}

	totalCompleted := 0
	count := 0
	for _, data := range velocityData[start:] {
		totalCompleted += data.Completed
		count++
	}

	if count == 0 {
		return 0, nil
	}

	return float64(totalCompleted) / float64(count), nil
}

// GetSprintProgress 获取迭代进度。
func (a *AgileManager) GetSprintProgress(sprintID string) (map[string]interface{}, error) {
	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()

	sprint, exists := a.engine.sprints[sprintID]
	if !exists {
		return nil, ErrSprintNotFound
	}

	totalCards := len(sprint.CardIDs)
	totalPoints := 0
	completedCards := 0
	completedPoints := 0
	inProgressCards := 0

	for _, cardID := range sprint.CardIDs {
		if card, exists := a.engine.cards[cardID]; exists {
			totalPoints += card.StoryPoints
			if card.Status == CardStatusDone {
				completedCards++
				completedPoints += card.StoryPoints
			} else if card.Status == CardStatusInProgress || card.Status == CardStatusReview {
				inProgressCards++
			}
		}
	}

	progress := 0.0
	if totalPoints > 0 {
		progress = float64(completedPoints) / float64(totalPoints) * 100
	}

	return map[string]interface{}{
		"sprint_id":         sprint.ID,
		"sprint_name":       sprint.Name,
		"status":            sprint.Status,
		"total_cards":       totalCards,
		"completed_cards":   completedCards,
		"in_progress_cards": inProgressCards,
		"total_points":      totalPoints,
		"completed_points":  completedPoints,
		"progress":          progress,
		"start_date":        sprint.StartDate,
		"end_date":          sprint.EndDate,
	}, nil
}

// EstimateSprintCapacity 估算迭代容量。
func (a *AgileManager) EstimateSprintCapacity(projectID string, startDate, endDate time.Time) (map[string]interface{}, error) {
	a.engine.mu.RLock()
	defer a.engine.mu.RUnlock()

	if _, exists := a.engine.projects[projectID]; !exists {
		return nil, ErrProjectNotFound
	}

	// 获取历史平均速度
	avgVelocity := 0.0
	completedSprints := 0

	for _, sprint := range a.engine.sprints {
		if sprint.ProjectID == projectID && sprint.Status == "completed" {
			avgVelocity += float64(sprint.Velocity)
			completedSprints++
		}
	}

	if completedSprints > 0 {
		avgVelocity = avgVelocity / float64(completedSprints)
	}

	// 计算天数
	days := int(endDate.Sub(startDate).Hours() / 24)
	if days <= 0 {
		days = 1
	}

	// 估算每日容量
	dailyCapacity := 0.0
	if avgVelocity > 0 {
		// 假设平均迭代周期为 14 天
		dailyCapacity = avgVelocity / 14.0
	}

	estimatedCapacity := dailyCapacity * float64(days)

	return map[string]interface{}{
		"project_id":         projectID,
		"start_date":         startDate,
		"end_date":           endDate,
		"days":               days,
		"avg_velocity":       avgVelocity,
		"daily_capacity":     dailyCapacity,
		"estimated_capacity": estimatedCapacity,
		"completed_sprints":  completedSprints,
	}, nil
}
