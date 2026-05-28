// Package projectboard 提供项目看板管理功能，支持看板/列表/甘特图多视图。
package projectboard

import (
	"errors"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// 错误定义。
var (
	ErrProjectNotFound    = errors.New("项目不存在")
	ErrBoardNotFound      = errors.New("看板不存在")
	ErrCardNotFound       = errors.New("卡片不存在")
	ErrColumnNotFound     = errors.New("列不存在")
	ErrLabelNotFound      = errors.New("标签不存在")
	ErrLabelExists        = errors.New("标签已存在")
	ErrSprintNotFound     = errors.New("迭代不存在")
	ErrMilestoneNotFound  = errors.New("里程碑不存在")
	ErrWorkflowNotFound   = errors.New("工作流不存在")
	ErrInvalidTransition  = errors.New("无效的状态转换")
	ErrInvalidProgress    = errors.New("无效的进度值")
	ErrWIPLimitExceeded   = errors.New("WIP 限制已超出")
	ErrCircularDependency = errors.New("循环依赖")
	ErrTimeEntryNotFound  = errors.New("时间记录不存在")
	ErrCardNotInSprint    = errors.New("卡片不在迭代中")
)

// Engine 项目看板引擎。
type Engine struct {
	mu          sync.RWMutex
	projects    map[string]*Project
	boards      map[string]*Board
	cards       map[string]*Card
	labels      map[string]*Label
	sprints     map[string]*Sprint
	milestones  map[string]*Milestone
	workflows   map[string]*Workflow
	timeEntries map[string]*TimeEntry
}

// generateID 生成唯一 ID。
func generateID() string {
	return uuid.New().String()
}

// NewEngine 创建引擎。
func NewEngine() *Engine {
	return &Engine{
		projects:    make(map[string]*Project),
		boards:      make(map[string]*Board),
		cards:       make(map[string]*Card),
		labels:      make(map[string]*Label),
		sprints:     make(map[string]*Sprint),
		milestones:  make(map[string]*Milestone),
		workflows:   make(map[string]*Workflow),
		timeEntries: make(map[string]*TimeEntry),
	}
}

// ========== 项目管理 ==========

// CreateProject 创建项目。
func (e *Engine) CreateProject(name, description, ownerID, createdBy string) (*Project, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	project := &Project{
		ID:        uuid.New().String(),
		Name:      name,
		Description: description,
		Status:    ProjectStatusActive,
		OwnerID:   ownerID,
		MemberIDs: []string{ownerID},
		CreatedAt: now,
		UpdatedAt: now,
		CreatedBy: createdBy,
	}

	e.projects[project.ID] = project
	return project, nil
}

// GetProject 获取项目。
func (e *Engine) GetProject(id string) (*Project, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	project, exists := e.projects[id]
	if !exists {
		return nil, ErrProjectNotFound
	}
	return project, nil
}

// UpdateProject 更新项目。
func (e *Engine) UpdateProject(id string, updates map[string]interface{}) (*Project, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	project, exists := e.projects[id]
	if !exists {
		return nil, ErrProjectNotFound
	}

	if name, ok := updates["name"].(string); ok {
		project.Name = name
	}
	if desc, ok := updates["description"].(string); ok {
		project.Description = desc
	}
	if status, ok := updates["status"].(ProjectStatus); ok {
		project.Status = status
	}

	project.UpdatedAt = time.Now()
	return project, nil
}

// DeleteProject 删除项目。
func (e *Engine) DeleteProject(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.projects[id]; !exists {
		return ErrProjectNotFound
	}

	// 删除关联数据
	for _, sprint := range e.sprints {
		if sprint.ProjectID == id {
			delete(e.sprints, sprint.ID)
		}
	}
	for _, ms := range e.milestones {
		if ms.ProjectID == id {
			delete(e.milestones, ms.ID)
		}
	}
	for _, label := range e.labels {
		if label.ProjectID == id {
			delete(e.labels, label.ID)
		}
	}
	for _, board := range e.boards {
		if board.ProjectID == id {
			// 删除看板下的卡片
			for _, card := range e.cards {
				if card.BoardID == board.ID {
					delete(e.cards, card.ID)
				}
			}
			delete(e.boards, board.ID)
		}
	}

	delete(e.projects, id)
	return nil
}

// ListProjects 列出项目。
func (e *Engine) ListProjects(ownerID string) []*Project {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Project, 0)
	for _, project := range e.projects {
		if ownerID == "" || project.OwnerID == ownerID {
			result = append(result, project)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// AddProjectMember 添加项目成员。
func (e *Engine) AddProjectMember(projectID, memberID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	project, exists := e.projects[projectID]
	if !exists {
		return ErrProjectNotFound
	}

	for _, id := range project.MemberIDs {
		if id == memberID {
			return nil
		}
	}

	project.MemberIDs = append(project.MemberIDs, memberID)
	project.UpdatedAt = time.Now()
	return nil
}

// RemoveProjectMember 移除项目成员。
func (e *Engine) RemoveProjectMember(projectID, memberID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	project, exists := e.projects[projectID]
	if !exists {
		return ErrProjectNotFound
	}

	for i, id := range project.MemberIDs {
		if id == memberID {
			project.MemberIDs = append(project.MemberIDs[:i], project.MemberIDs[i+1:]...)
			project.UpdatedAt = time.Now()
			return nil
		}
	}

	return nil
}

// ========== 看板管理 ==========

// CreateBoard 创建看板。
func (e *Engine) CreateBoard(projectID, name, description string) (*Board, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.projects[projectID]; !exists {
		return nil, ErrProjectNotFound
	}

	now := time.Now()
	board := &Board{
		ID:          uuid.New().String(),
		ProjectID:   projectID,
		Name:        name,
		Description: description,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// 创建默认列
	board.Columns = []Column{
		{ID: uuid.New().String(), BoardID: board.ID, Name: "待办", Status: CardStatusTodo, Position: 0},
		{ID: uuid.New().String(), BoardID: board.ID, Name: "进行中", Status: CardStatusInProgress, Position: 1, WIPLimit: 5},
		{ID: uuid.New().String(), BoardID: board.ID, Name: "评审", Status: CardStatusReview, Position: 2, WIPLimit: 3},
		{ID: uuid.New().String(), BoardID: board.ID, Name: "完成", Status: CardStatusDone, Position: 3},
	}

	e.boards[board.ID] = board

	// 更新项目看板计数
	if project, exists := e.projects[projectID]; exists {
		project.BoardCount++
		project.UpdatedAt = now
	}

	return board, nil
}

// GetBoard 获取看板。
func (e *Engine) GetBoard(id string) (*Board, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	board, exists := e.boards[id]
	if !exists {
		return nil, ErrBoardNotFound
	}
	return board, nil
}

// DeleteBoard 删除看板。
func (e *Engine) DeleteBoard(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	board, exists := e.boards[id]
	if !exists {
		return ErrBoardNotFound
	}

	// 删除关联卡片
	for _, card := range e.cards {
		if card.BoardID == id {
			delete(e.cards, card.ID)
		}
	}

	// 更新项目看板计数
	if project, exists := e.projects[board.ProjectID]; exists {
		project.BoardCount--
		project.UpdatedAt = time.Now()
	}

	delete(e.boards, id)
	return nil
}

// ListBoards 列出看板。
func (e *Engine) ListBoards(projectID string) []*Board {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Board, 0)
	for _, board := range e.boards {
		if board.ProjectID == projectID {
			result = append(result, board)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// ========== 卡片管理 ==========

// CreateCard 创建卡片。
func (e *Engine) CreateCard(boardID, title, description, reporterID string, priority CardPriority) (*Card, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	board, exists := e.boards[boardID]
	if !exists {
		return nil, ErrBoardNotFound
	}

	// 找到第一个列（待办列）
	var columnID string
	var columnStatus CardStatus
	for _, col := range board.Columns {
		if col.Position == 0 {
			// 检查 WIP 限制
			if col.WIPLimit > 0 && col.CardCount >= col.WIPLimit {
				return nil, ErrWIPLimitExceeded
			}
			columnID = col.ID
			columnStatus = col.Status
			break
		}
	}

	now := time.Now()
	card := &Card{
		ID:          uuid.New().String(),
		BoardID:     boardID,
		ColumnID:    columnID,
		Title:       title,
		Description: description,
		Status:      columnStatus,
		Priority:    priority,
		ReporterID:  reporterID,
		CreatedAt:   now,
		UpdatedAt:   now,
		CreatedBy:   reporterID,
	}

	e.cards[card.ID] = card
	board.CardCount++

	// 更新列卡片计数
	for i, col := range board.Columns {
		if col.ID == columnID {
			board.Columns[i].CardCount++
			break
		}
	}

	// 更新项目卡片计数
	if project, exists := e.projects[board.ProjectID]; exists {
		project.CardCount++
		project.UpdatedAt = now
	}

	return card, nil
}

// GetCard 获取卡片。
func (e *Engine) GetCard(id string) (*Card, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	card, exists := e.cards[id]
	if !exists {
		return nil, ErrCardNotFound
	}
	return card, nil
}

// UpdateCard 更新卡片。
func (e *Engine) UpdateCard(id string, updates map[string]interface{}) (*Card, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	card, exists := e.cards[id]
	if !exists {
		return nil, ErrCardNotFound
	}

	now := time.Now()
	card.UpdatedAt = now

	if title, ok := updates["title"].(string); ok {
		card.Title = title
	}
	if desc, ok := updates["description"].(string); ok {
		card.Description = desc
	}
	if priority, ok := updates["priority"].(CardPriority); ok {
		card.Priority = priority
	}
	if assigneeID, ok := updates["assignee_id"].(string); ok {
		card.AssigneeID = assigneeID
	}
	if dueDate, ok := updates["due_date"].(*time.Time); ok {
		card.DueDate = dueDate
	}
	if startDate, ok := updates["start_date"].(*time.Time); ok {
		card.StartDate = startDate
	}
	// 也支持 time.Time 值类型
	if dueDateVal, ok := updates["due_date"].(time.Time); ok {
		card.DueDate = &dueDateVal
	}
	if startDateVal, ok := updates["start_date"].(time.Time); ok {
		card.StartDate = &startDateVal
	}
	if labels, ok := updates["labels"].([]string); ok {
		card.Labels = labels
	}
	if points, ok := updates["story_points"].(int); ok {
		card.StoryPoints = points
	}
	if estimate, ok := updates["estimate_hrs"].(float64); ok {
		card.EstimateHrs = estimate
	}
	if estimateInt, ok := updates["estimate_hrs"].(int); ok {
		card.EstimateHrs = float64(estimateInt)
	}

	return card, nil
}

// DeleteCard 删除卡片。
func (e *Engine) DeleteCard(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	card, exists := e.cards[id]
	if !exists {
		return ErrCardNotFound
	}

	// 更新列卡片计数
	if board, exists := e.boards[card.BoardID]; exists {
		board.CardCount--
		for i, col := range board.Columns {
			if col.ID == card.ColumnID {
				board.Columns[i].CardCount--
				break
			}
		}

		// 更新项目卡片计数
		if project, exists := e.projects[board.ProjectID]; exists {
			project.CardCount--
			project.UpdatedAt = time.Now()
		}
	}

	// 移除子任务引用
	if card.ParentID != "" {
		if parent, exists := e.cards[card.ParentID]; exists {
			for i, subID := range parent.SubtaskIDs {
				if subID == id {
					parent.SubtaskIDs = append(parent.SubtaskIDs[:i], parent.SubtaskIDs[i+1:]...)
					break
				}
			}
		}
	}

	delete(e.cards, id)
	return nil
}

// ListCards 列出卡片。
func (e *Engine) ListCards(boardID string, filter CardFilter) []*Card {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Card, 0)
	for _, card := range e.cards {
		if boardID != "" && card.BoardID != boardID {
			continue
		}
		if !matchCardFilter(card, filter) {
			continue
		}
		result = append(result, card)
	}

	sortCards(result, filter.OrderBy, filter.OrderDesc)

	offset := filter.Offset
	if offset > len(result) {
		offset = len(result)
	}
	end := offset + filter.Limit
	if filter.Limit <= 0 || end > len(result) {
		end = len(result)
	}

	return result[offset:end]
}

// MoveCard 移动卡片到新列。
func (e *Engine) MoveCard(cardID, toColumnID string) (*Card, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	card, exists := e.cards[cardID]
	if !exists {
		return nil, ErrCardNotFound
	}

	board, exists := e.boards[card.BoardID]
	if !exists {
		return nil, ErrBoardNotFound
	}

	var toColumn *Column
	for i, col := range board.Columns {
		if col.ID == toColumnID {
			// 检查 WIP 限制
			if col.WIPLimit > 0 && col.CardCount >= col.WIPLimit && card.ColumnID != toColumnID {
				return nil, ErrWIPLimitExceeded
			}
			toColumn = &board.Columns[i]
			break
		}
	}

	if toColumn == nil {
		return nil, ErrColumnNotFound
	}

	// 更新旧列计数
	for i, col := range board.Columns {
		if col.ID == card.ColumnID {
			board.Columns[i].CardCount--
			break
		}
	}

	// 更新新列计数
	toColumn.CardCount++

	card.ColumnID = toColumnID
	card.Status = toColumn.Status
	card.UpdatedAt = time.Now()

	// 自动更新进度
	switch toColumn.Status {
	case CardStatusDone:
		card.Progress = 100
	case CardStatusBacklog:
		card.Progress = 0
	}

	return card, nil
}

// AssignCard 分配卡片。
func (e *Engine) AssignCard(cardID, assigneeID string) (*Card, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	card, exists := e.cards[cardID]
	if !exists {
		return nil, ErrCardNotFound
	}

	card.AssigneeID = assigneeID
	card.UpdatedAt = time.Now()
	return card, nil
}

// UpdateCardProgress 更新卡片进度。
func (e *Engine) UpdateCardProgress(cardID string, progress int) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if progress < 0 || progress > 100 {
		return ErrInvalidProgress
	}

	card, exists := e.cards[cardID]
	if !exists {
		return ErrCardNotFound
	}

	card.Progress = progress
	card.UpdatedAt = time.Now()
	return nil
}

// AddSubtask 添加子任务。
func (e *Engine) AddSubtask(parentID, subtaskID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	parent, exists := e.cards[parentID]
	if !exists {
		return ErrCardNotFound
	}

	subtask, exists := e.cards[subtaskID]
	if !exists {
		return ErrCardNotFound
	}

	subtask.ParentID = parentID
	parent.SubtaskIDs = append(parent.SubtaskIDs, subtaskID)
	parent.UpdatedAt = time.Now()
	return nil
}

// ========== 标签管理 ==========

// CreateLabel 创建标签。
func (e *Engine) CreateLabel(projectID, name, color string) (*Label, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.projects[projectID]; !exists {
		return nil, ErrProjectNotFound
	}

	for _, label := range e.labels {
		if label.ProjectID == projectID && label.Name == name {
			return nil, ErrLabelExists
		}
	}

	label := &Label{
		ID:        uuid.New().String(),
		ProjectID: projectID,
		Name:      name,
		Color:     color,
		CreatedAt: time.Now(),
	}

	e.labels[label.ID] = label
	return label, nil
}

// DeleteLabel 删除标签。
func (e *Engine) DeleteLabel(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	label, exists := e.labels[id]
	if !exists {
		return ErrLabelNotFound
	}

	// 从卡片中移除标签
	for _, card := range e.cards {
		board, exists := e.boards[card.BoardID]
		if exists && board.ProjectID == label.ProjectID {
			card.Labels = removeString(card.Labels, label.Name)
		}
	}

	delete(e.labels, id)
	return nil
}

// ListLabels 列出标签。
func (e *Engine) ListLabels(projectID string) []*Label {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Label, 0)
	for _, label := range e.labels {
		if label.ProjectID == projectID {
			result = append(result, label)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Name < result[j].Name
	})

	return result
}

// AddLabelToCard 给卡片添加标签。
func (e *Engine) AddLabelToCard(cardID, labelID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	card, exists := e.cards[cardID]
	if !exists {
		return ErrCardNotFound
	}

	label, exists := e.labels[labelID]
	if !exists {
		return ErrLabelNotFound
	}

	for _, l := range card.Labels {
		if l == label.Name {
			return nil
		}
	}

	card.Labels = append(card.Labels, label.Name)
	card.UpdatedAt = time.Now()
	return nil
}

// RemoveLabelFromCard 从卡片移除标签。
func (e *Engine) RemoveLabelFromCard(cardID, labelID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	card, exists := e.cards[cardID]
	if !exists {
		return ErrCardNotFound
	}

	label, exists := e.labels[labelID]
	if !exists {
		return ErrLabelNotFound
	}

	card.Labels = removeString(card.Labels, label.Name)
	card.UpdatedAt = time.Now()
	return nil
}

// ========== 时间追踪 ==========

// LogTime 记录时间。
func (e *Engine) LogTime(cardID, userID string, hours float64, note string, date time.Time) (*TimeEntry, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	card, exists := e.cards[cardID]
	if !exists {
		return nil, ErrCardNotFound
	}

	entry := &TimeEntry{
		ID:        uuid.New().String(),
		CardID:    cardID,
		UserID:    userID,
		Hours:     hours,
		Note:      note,
		Date:      date,
		CreatedAt: time.Now(),
	}

	e.timeEntries[entry.ID] = entry
	card.SpentHrs += hours
	card.UpdatedAt = time.Now()

	return entry, nil
}

// ListTimeEntries 列出时间记录。
func (e *Engine) ListTimeEntries(cardID string) []*TimeEntry {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*TimeEntry, 0)
	for _, entry := range e.timeEntries {
		if cardID == "" || entry.CardID == cardID {
			result = append(result, entry)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Date.After(result[j].Date)
	})

	return result
}

// GetProjectStats 获取项目统计。
func (e *Engine) GetProjectStats(projectID string) (*ProjectStats, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if _, exists := e.projects[projectID]; !exists {
		return nil, ErrProjectNotFound
	}

	stats := &ProjectStats{
		ByStatus:   make(map[string]int),
		ByPriority: make(map[string]int),
	}

	now := time.Now()

	for _, card := range e.cards {
		board, exists := e.boards[card.BoardID]
		if !exists || board.ProjectID != projectID {
			continue
		}

		stats.TotalCards++
		stats.ByStatus[string(card.Status)]++
		stats.ByPriority[string(card.Priority)]++
		stats.TotalPoints += card.StoryPoints
		stats.TotalHours += card.EstimateHrs
		stats.SpentHours += card.SpentHrs

		if card.Status == CardStatusDone {
			stats.CompletedPoints += card.StoryPoints
		}

		if card.DueDate != nil && card.DueDate.Before(now) && card.Status != CardStatusDone {
			stats.OverdueCards++
		}
	}

	return stats, nil
}

// ========== 辅助函数 ==========

func matchCardFilter(card *Card, filter CardFilter) bool {
	if len(filter.Status) > 0 {
		found := false
		for _, s := range filter.Status {
			if s == card.Status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(filter.Priority) > 0 {
		found := false
		for _, p := range filter.Priority {
			if p == card.Priority {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if filter.AssigneeID != "" && card.AssigneeID != filter.AssigneeID {
		return false
	}

	if filter.SprintID != "" && card.SprintID != filter.SprintID {
		return false
	}

	if filter.MilestoneID != "" && card.MilestoneID != filter.MilestoneID {
		return false
	}

	if len(filter.Labels) > 0 {
		for _, fl := range filter.Labels {
			found := false
			for _, cl := range card.Labels {
				if cl == fl {
					found = true
					break
				}
			}
			if !found {
				return false
			}
		}
	}

	if filter.Search != "" {
		search := strings.ToLower(filter.Search)
		title := strings.ToLower(card.Title)
		desc := strings.ToLower(card.Description)
		if !strings.Contains(title, search) && !strings.Contains(desc, search) {
			return false
		}
	}

	return true
}

func sortCards(cards []*Card, orderBy string, desc bool) {
	sort.Slice(cards, func(i, j int) bool {
		var less bool
		switch orderBy {
		case "priority":
			less = comparePriority(cards[i].Priority, cards[j].Priority) < 0
		case "due_date":
			if cards[i].DueDate == nil && cards[j].DueDate == nil {
				less = false
			} else if cards[i].DueDate == nil {
				less = true
			} else if cards[j].DueDate == nil {
				less = false
			} else {
				less = cards[i].DueDate.Before(*cards[j].DueDate)
			}
		case "status":
			less = compareCardStatus(cards[i].Status, cards[j].Status) < 0
		case "story_points":
			less = cards[i].StoryPoints < cards[j].StoryPoints
		default:
			less = cards[i].CreatedAt.Before(cards[j].CreatedAt)
		}
		if desc {
			return !less
		}
		return less
	})
}

func comparePriority(a, b CardPriority) int {
	priorityOrder := map[CardPriority]int{
		PriorityUrgent: 4,
		PriorityHigh:   3,
		PriorityMedium: 2,
		PriorityLow:    1,
	}
	return priorityOrder[a] - priorityOrder[b]
}

func compareCardStatus(a, b CardStatus) int {
	statusOrder := map[CardStatus]int{
		CardStatusBacklog:    0,
		CardStatusTodo:       1,
		CardStatusInProgress: 2,
		CardStatusReview:     3,
		CardStatusDone:       4,
	}
	return statusOrder[a] - statusOrder[b]
}

func removeString(slice []string, s string) []string {
	result := make([]string, 0, len(slice))
	for _, v := range slice {
		if v != s {
			result = append(result, v)
		}
	}
	return result
}
