// Package kanbanboard 核心管理逻辑
package kanbanboard

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Manager 看板管理器.
type Manager struct {
	mu       sync.RWMutex
	boards   map[string]*Board
	activity []*Activity
}

// NewManager 创建看板管理器.
func NewManager() *Manager {
	return &Manager{
		boards:   make(map[string]*Board),
		activity: make([]*Activity, 0),
	}
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

func (m *Manager) addActivity(boardID, cardID, userID, action, detail string) {
	m.activity = append(m.activity, &Activity{
		ID:        generateID(),
		BoardID:   boardID,
		CardID:    cardID,
		UserID:    userID,
		Action:    action,
		Detail:    detail,
		CreatedAt: time.Now(),
	})
}

// ============================================================
// 看板管理
// ============================================================

// ListBoards 列出所有看板.
func (m *Manager) ListBoards() []*Board {
	m.mu.RLock()
	defer m.mu.RUnlock()

	boards := make([]*Board, 0, len(m.boards))
	for _, b := range m.boards {
		boards = append(boards, b)
	}
	sort.Slice(boards, func(i, j int) bool {
		return boards[i].CreatedAt.After(boards[j].CreatedAt)
	})
	return boards
}

// CreateBoard 创建看板.
func (m *Manager) CreateBoard(req *CreateBoardRequest) (*Board, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("board name is required")
	}

	now := time.Now()
	board := &Board{
		ID:          generateID(),
		Name:        req.Name,
		Description: req.Description,
		Status:      BoardStatusActive,
		Columns: []*Column{
			{ID: generateID(), Name: "待办", Position: 0, WIPLimit: 0, Cards: make([]*Card, 0), CreatedAt: now},
			{ID: generateID(), Name: "进行中", Position: 1, WIPLimit: 5, Cards: make([]*Card, 0), CreatedAt: now},
			{ID: generateID(), Name: "已完成", Position: 2, WIPLimit: 0, Cards: make([]*Card, 0), CreatedAt: now},
		},
		Labels:    make([]*Label, 0),
		Members:   []*Member{{UserID: req.OwnerID, Role: "owner", JoinedAt: now}},
		OwnerID:   req.OwnerID,
		CreatedAt: now,
		UpdatedAt: now,
	}

	for _, col := range board.Columns {
		col.BoardID = board.ID
	}

	m.boards[board.ID] = board
	m.addActivity(board.ID, "", req.OwnerID, "board_created", fmt.Sprintf("创建看板: %s", req.Name))

	return board, nil
}

// GetBoard 获取看板.
func (m *Manager) GetBoard(id string) (*Board, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, ok := m.boards[id]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", id)
	}
	return board, nil
}

// UpdateBoard 更新看板.
func (m *Manager) UpdateBoard(id string, req *UpdateBoardRequest) (*Board, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[id]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", id)
	}

	if req.Name != nil {
		board.Name = *req.Name
	}
	if req.Description != nil {
		board.Description = *req.Description
	}
	board.UpdatedAt = time.Now()

	m.addActivity(id, "", "", "board_updated", fmt.Sprintf("更新看板: %s", board.Name))
	return board, nil
}

// DeleteBoard 删除看板.
func (m *Manager) DeleteBoard(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.boards[id]; !ok {
		return fmt.Errorf("board not found: %s", id)
	}
	delete(m.boards, id)
	return nil
}

// ArchiveBoard 归档看板.
func (m *Manager) ArchiveBoard(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[id]
	if !ok {
		return fmt.Errorf("board not found: %s", id)
	}

	board.Status = BoardStatusArchived
	board.UpdatedAt = time.Now()
	m.addActivity(id, "", "", "board_archived", fmt.Sprintf("归档看板: %s", board.Name))
	return nil
}

// ============================================================
// 列管理
// ============================================================

// AddColumn 添加列.
func (m *Manager) AddColumn(boardID string, req *CreateColumnRequest) (*Column, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	// 调整位置
	pos := req.Position
	if pos < 0 || pos > len(board.Columns) {
		pos = len(board.Columns)
	}

	column := &Column{
		ID:        generateID(),
		BoardID:   boardID,
		Name:      req.Name,
		Position:  pos,
		WIPLimit:  req.WIPLimit,
		Cards:     make([]*Card, 0),
		CreatedAt: time.Now(),
	}

	// 插入并重排
	board.Columns = append(board.Columns[:pos], append([]*Column{column}, board.Columns[pos:]...)...)
	for i, col := range board.Columns {
		col.Position = i
	}

	board.UpdatedAt = time.Now()
	m.addActivity(boardID, "", "", "column_added", fmt.Sprintf("添加列: %s", req.Name))
	return column, nil
}

// UpdateColumn 更新列.
func (m *Manager) UpdateColumn(boardID, columnID string, req *UpdateColumnRequest) (*Column, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	var col *Column
	for _, c := range board.Columns {
		if c.ID == columnID {
			col = c
			break
		}
	}
	if col == nil {
		return nil, fmt.Errorf("column not found: %s", columnID)
	}

	if req.Name != nil {
		col.Name = *req.Name
	}
	if req.WIPLimit != nil {
		col.WIPLimit = *req.WIPLimit
	}
	if req.Position != nil {
		newPos := *req.Position
		if newPos >= 0 && newPos < len(board.Columns) {
			// 移除旧位置
			for i, c := range board.Columns {
				if c.ID == columnID {
					board.Columns = append(board.Columns[:i], board.Columns[i+1:]...)
					break
				}
			}
			// 插入新位置
			board.Columns = append(board.Columns[:newPos], append([]*Column{col}, board.Columns[newPos:]...)...)
			for i, c := range board.Columns {
				c.Position = i
			}
		}
	}

	board.UpdatedAt = time.Now()
	return col, nil
}

// DeleteColumn 删除列.
func (m *Manager) DeleteColumn(boardID, columnID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return fmt.Errorf("board not found: %s", boardID)
	}

	for i, col := range board.Columns {
		if col.ID == columnID {
			if len(col.Cards) > 0 {
				return fmt.Errorf("cannot delete column with cards, move cards first")
			}
			board.Columns = append(board.Columns[:i], board.Columns[i+1:]...)
			for j, c := range board.Columns {
				c.Position = j
			}
			board.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("column not found: %s", columnID)
}

// ============================================================
// 卡片管理
// ============================================================

// AddCard 添加卡片.
func (m *Manager) AddCard(boardID string, req *CreateCardRequest) (*Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	var targetCol *Column
	for _, col := range board.Columns {
		if col.ID == req.ColumnID {
			targetCol = col
			break
		}
	}
	if targetCol == nil {
		return nil, fmt.Errorf("column not found: %s", req.ColumnID)
	}

	// 检查 WIP 限制
	if targetCol.WIPLimit > 0 && len(targetCol.Cards) >= targetCol.WIPLimit {
		return nil, fmt.Errorf("column %s reached WIP limit (%d)", targetCol.Name, targetCol.WIPLimit)
	}

	priority := req.Priority
	if priority == "" {
		priority = PriorityMedium
	}

	now := time.Now()
	card := &Card{
		ID:          generateID(),
		ColumnID:    req.ColumnID,
		BoardID:     boardID,
		Title:       req.Title,
		Description: req.Description,
		Status:      CardStatusTodo,
		Priority:    priority,
		AssigneeID:  req.AssigneeID,
		LabelIDs:    req.LabelIDs,
		DueDate:     req.DueDate,
		Position:    len(targetCol.Cards),
		CreatedBy:   req.CreatedBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	targetCol.Cards = append(targetCol.Cards, card)
	board.UpdatedAt = now
	m.addActivity(boardID, card.ID, req.CreatedBy, "card_created", fmt.Sprintf("添加卡片: %s", req.Title))

	return card, nil
}

// GetCard 获取卡片.
func (m *Manager) GetCard(boardID, cardID string) (*Card, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	for _, col := range board.Columns {
		for _, card := range col.Cards {
			if card.ID == cardID {
				return card, nil
			}
		}
	}
	return nil, fmt.Errorf("card not found: %s", cardID)
}

// UpdateCard 更新卡片.
func (m *Manager) UpdateCard(boardID, cardID string, req *UpdateCardRequest) (*Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	card := m.findCard(board, cardID)
	if card == nil {
		return nil, fmt.Errorf("card not found: %s", cardID)
	}

	if req.Title != nil {
		card.Title = *req.Title
	}
	if req.Description != nil {
		card.Description = *req.Description
	}
	if req.Priority != nil {
		card.Priority = *req.Priority
	}
	if req.AssigneeID != nil {
		card.AssigneeID = *req.AssigneeID
	}
	if req.DueDate != nil {
		card.DueDate = req.DueDate
	}
	if req.Status != nil {
		card.Status = *req.Status
		if *req.Status == CardStatusDone && card.CompletedAt == nil {
			now := time.Now()
			card.CompletedAt = &now
		}
	}

	card.UpdatedAt = time.Now()
	board.UpdatedAt = time.Now()
	m.addActivity(boardID, cardID, "", "card_updated", fmt.Sprintf("更新卡片: %s", card.Title))

	return card, nil
}

// DeleteCard 删除卡片.
func (m *Manager) DeleteCard(boardID, cardID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return fmt.Errorf("board not found: %s", boardID)
	}

	for _, col := range board.Columns {
		for i, card := range col.Cards {
			if card.ID == cardID {
				col.Cards = append(col.Cards[:i], col.Cards[i+1:]...)
				for j, c := range col.Cards {
					c.Position = j
				}
				board.UpdatedAt = time.Now()
				m.addActivity(boardID, cardID, "", "card_deleted", fmt.Sprintf("删除卡片: %s", card.Title))
				return nil
			}
		}
	}
	return fmt.Errorf("card not found: %s", cardID)
}

// MoveCard 移动卡片.
func (m *Manager) MoveCard(boardID, cardID string, req *MoveCardRequest) (*Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	// 查找卡片
	var card *Card
	var srcCol *Column
	for _, col := range board.Columns {
		for i, c := range col.Cards {
			if c.ID == cardID {
				card = c
				srcCol = col
				srcCol.Cards = append(srcCol.Cards[:i], srcCol.Cards[i+1:]...)
				break
			}
		}
		if card != nil {
			break
		}
	}
	if card == nil {
		return nil, fmt.Errorf("card not found: %s", cardID)
	}

	// 找目标列
	var dstCol *Column
	for _, col := range board.Columns {
		if col.ID == req.TargetColumnID {
			dstCol = col
			break
		}
	}
	if dstCol == nil {
		return nil, fmt.Errorf("target column not found: %s", req.TargetColumnID)
	}

	// 检查 WIP 限制
	if dstCol.WIPLimit > 0 && len(dstCol.Cards) >= dstCol.WIPLimit {
		// 回滚
		srcCol.Cards = append(srcCol.Cards, card)
		return nil, fmt.Errorf("target column %s reached WIP limit (%d)", dstCol.Name, dstCol.WIPLimit)
	}

	// 插入
	pos := req.Position
	if pos < 0 || pos > len(dstCol.Cards) {
		pos = len(dstCol.Cards)
	}
	dstCol.Cards = append(dstCol.Cards[:pos], append([]*Card{card}, dstCol.Cards[pos:]...)...)

	card.ColumnID = req.TargetColumnID
	card.Position = pos
	card.UpdatedAt = time.Now()

	// 重排
	for i, c := range dstCol.Cards {
		c.Position = i
	}
	for i, c := range srcCol.Cards {
		c.Position = i
	}

	board.UpdatedAt = time.Now()
	m.addActivity(boardID, cardID, "", "card_moved",
		fmt.Sprintf("卡片 %s 从 %s 移动到 %s", card.Title, srcCol.Name, dstCol.Name))

	return card, nil
}

// AssignCard 分配卡片给成员.
func (m *Manager) AssignCard(boardID, cardID, assigneeID string) (*Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	card := m.findCard(board, cardID)
	if card == nil {
		return nil, fmt.Errorf("card not found: %s", cardID)
	}

	oldAssignee := card.AssigneeID
	card.AssigneeID = assigneeID
	card.UpdatedAt = time.Now()
	board.UpdatedAt = time.Now()

	m.addActivity(boardID, cardID, assigneeID, "card_assigned",
		fmt.Sprintf("分配卡片 %s: %s -> %s", card.Title, oldAssignee, assigneeID))

	return card, nil
}

// ============================================================
// 标签管理
// ============================================================

// AddLabel 添加标签.
func (m *Manager) AddLabel(boardID string, req *CreateLabelRequest) (*Label, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	label := &Label{
		ID:      generateID(),
		BoardID: boardID,
		Name:    req.Name,
		Color:   req.Color,
	}

	board.Labels = append(board.Labels, label)
	board.UpdatedAt = time.Now()
	m.addActivity(boardID, "", "", "label_added", fmt.Sprintf("添加标签: %s", req.Name))

	return label, nil
}

// UpdateLabel 更新标签.
func (m *Manager) UpdateLabel(boardID, labelID string, req *UpdateLabelRequest) (*Label, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	for _, l := range board.Labels {
		if l.ID == labelID {
			if req.Name != nil {
				l.Name = *req.Name
			}
			if req.Color != nil {
				l.Color = *req.Color
			}
			board.UpdatedAt = time.Now()
			return l, nil
		}
	}
	return nil, fmt.Errorf("label not found: %s", labelID)
}

// DeleteLabel 删除标签.
func (m *Manager) DeleteLabel(boardID, labelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return fmt.Errorf("board not found: %s", boardID)
	}

	for i, l := range board.Labels {
		if l.ID == labelID {
			board.Labels = append(board.Labels[:i], board.Labels[i+1:]...)
			// 从所有卡片中移除该标签
			for _, col := range board.Columns {
				for _, card := range col.Cards {
					card.LabelIDs = removeString(card.LabelIDs, labelID)
				}
			}
			board.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("label not found: %s", labelID)
}

// ApplyLabel 应用标签到卡片.
func (m *Manager) ApplyLabel(boardID, cardID, labelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return fmt.Errorf("board not found: %s", boardID)
	}

	// 验证标签存在
	labelExists := false
	for _, l := range board.Labels {
		if l.ID == labelID {
			labelExists = true
			break
		}
	}
	if !labelExists {
		return fmt.Errorf("label not found: %s", labelID)
	}

	card := m.findCard(board, cardID)
	if card == nil {
		return fmt.Errorf("card not found: %s", cardID)
	}

	// 避免重复
	for _, id := range card.LabelIDs {
		if id == labelID {
			return nil
		}
	}

	card.LabelIDs = append(card.LabelIDs, labelID)
	card.UpdatedAt = time.Now()
	board.UpdatedAt = time.Now()
	return nil
}

// RemoveLabel 移除卡片标签.
func (m *Manager) RemoveLabel(boardID, cardID, labelID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return fmt.Errorf("board not found: %s", boardID)
	}

	card := m.findCard(board, cardID)
	if card == nil {
		return fmt.Errorf("card not found: %s", cardID)
	}

	card.LabelIDs = removeString(card.LabelIDs, labelID)
	card.UpdatedAt = time.Now()
	board.UpdatedAt = time.Now()
	return nil
}

// ============================================================
// 搜索与过滤
// ============================================================

// SearchCards 搜索卡片.
func (m *Manager) SearchCards(filter *CardFilter) []*Card {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Card, 0)

	for _, board := range m.boards {
		if filter.BoardID != "" && board.ID != filter.BoardID {
			continue
		}

		for _, col := range board.Columns {
			for _, card := range col.Cards {
				if !matchFilter(card, filter) {
					continue
				}
				result = append(result, card)
			}
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].UpdatedAt.After(result[j].UpdatedAt)
	})
	return result
}

// ============================================================
// 统计报表
// ============================================================

// GetBoardStats 获取看板统计.
func (m *Manager) GetBoardStats(boardID string) (*BoardStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	stats := &BoardStats{}

	for _, col := range board.Columns {
		for _, card := range col.Cards {
			stats.TotalCards++
			switch card.Status {
			case CardStatusDone:
				stats.CompletedCards++
			case CardStatusInProgress:
				stats.InProgressCards++
			case CardStatusBlocked:
				stats.BlockedCards++
			default:
				stats.TodoCards++
			}
		}
	}

	return stats, nil
}

// GetBurndownData 获取燃尽图数据.
func (m *Manager) GetBurndownData(boardID string, days int) ([]*BurndownPoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	if days <= 0 {
		days = 14
	}

	// 统计总卡片数
	totalCards := 0
	for _, col := range board.Columns {
		totalCards += len(col.Cards)
	}

	points := make([]*BurndownPoint, 0, days)
	now := time.Now()

	// 模拟理想燃尽线
	for i := 0; i < days; i++ {
		date := now.AddDate(0, 0, -days+i+1)
		idealRemaining := totalCards - (totalCards*i)/days
		if idealRemaining < 0 {
			idealRemaining = 0
		}

		// 计算实际完成数（基于 CompletedAt）
		completed := 0
		for _, col := range board.Columns {
			for _, card := range col.Cards {
				if card.CompletedAt != nil && !card.CompletedAt.After(date) {
					completed++
				}
			}
		}

		points = append(points, &BurndownPoint{
			Date:      date,
			Remaining: totalCards - completed,
			Completed: completed,
		})
	}

	return points, nil
}

// GetVelocityData 获取速度图数据.
func (m *Manager) GetVelocityData(boardID string, sprints int) ([]*VelocityPoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	if sprints <= 0 {
		sprints = 4
	}

	// 简化实现：按周统计
	points := make([]*VelocityPoint, 0, sprints)
	now := time.Now()

	for i := sprints - 1; i >= 0; i-- {
		weekStart := now.AddDate(0, 0, -7*(i+1))
		weekEnd := now.AddDate(0, 0, -7*i)

		planned := 0
		completed := 0
		for _, col := range board.Columns {
			for _, card := range col.Cards {
				if card.CreatedAt.After(weekStart) && card.CreatedAt.Before(weekEnd) {
					planned++
				}
				if card.CompletedAt != nil && card.CompletedAt.After(weekStart) && card.CompletedAt.Before(weekEnd) {
					completed++
				}
			}
		}

		points = append(points, &VelocityPoint{
			SprintName: fmt.Sprintf("Sprint %d", sprints-i),
			Planned:    planned,
			Completed:  completed,
		})
	}

	return points, nil
}

// GetCumulativeFlowData 获取累积流图数据.
func (m *Manager) GetCumulativeFlowData(boardID string, days int) ([]*CumulativeFlowPoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	if days <= 0 {
		days = 14
	}

	points := make([]*CumulativeFlowPoint, 0, days)
	now := time.Now()

	for i := 0; i < days; i++ {
		date := now.AddDate(0, 0, -days+i+1)
		counts := make(map[string]int)

		for _, col := range board.Columns {
			count := 0
			for _, card := range col.Cards {
				if card.CreatedAt.Before(date) || card.CreatedAt.Equal(date) {
					count++
				}
			}
			counts[col.Name] = count
		}

		points = append(points, &CumulativeFlowPoint{
			Date:   date,
			Counts: counts,
		})
	}

	return points, nil
}

// ============================================================
// 成员管理
// ============================================================

// AssignMember 分配成员.
func (m *Manager) AssignMember(boardID string, req *AssignMemberRequest) (*Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	// 检查重复
	for _, mem := range board.Members {
		if mem.UserID == req.UserID {
			mem.Role = req.Role
			board.UpdatedAt = time.Now()
			m.addActivity(boardID, "", req.UserID, "member_updated",
				fmt.Sprintf("更新成员 %s 角色为 %s", req.Username, req.Role))
			return mem, nil
		}
	}

	member := &Member{
		UserID:   req.UserID,
		Username: req.Username,
		Role:     req.Role,
		JoinedAt: time.Now(),
	}

	board.Members = append(board.Members, member)
	board.UpdatedAt = time.Now()
	m.addActivity(boardID, "", req.UserID, "member_assigned",
		fmt.Sprintf("添加成员: %s (%s)", req.Username, req.Role))

	return member, nil
}

// RemoveMember 移除成员.
func (m *Manager) RemoveMember(boardID, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return fmt.Errorf("board not found: %s", boardID)
	}

	for i, mem := range board.Members {
		if mem.UserID == userID {
			board.Members = append(board.Members[:i], board.Members[i+1:]...)
			board.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("member not found: %s", userID)
}

// ============================================================
// 活动记录
// ============================================================

// GetActivity 获取活动记录.
func (m *Manager) GetActivity(boardID string, limit int) []*Activity {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	result := make([]*Activity, 0)
	for i := len(m.activity) - 1; i >= 0 && len(result) < limit; i-- {
		if boardID == "" || m.activity[i].BoardID == boardID {
			result = append(result, m.activity[i])
		}
	}
	return result
}

// ============================================================
// 辅助函数
// ============================================================

func (m *Manager) findCard(board *Board, cardID string) *Card {
	for _, col := range board.Columns {
		for _, card := range col.Cards {
			if card.ID == cardID {
				return card
			}
		}
	}
	return nil
}

func matchFilter(card *Card, filter *CardFilter) bool {
	if filter.AssigneeID != "" && card.AssigneeID != filter.AssigneeID {
		return false
	}
	if filter.Status != "" && card.Status != filter.Status {
		return false
	}
	if filter.Priority != "" && card.Priority != filter.Priority {
		return false
	}
	if len(filter.LabelIDs) > 0 {
		hasLabel := false
		for _, filterLabel := range filter.LabelIDs {
			for _, cardLabel := range card.LabelIDs {
				if filterLabel == cardLabel {
					hasLabel = true
					break
				}
			}
			if hasLabel {
				break
			}
		}
		if !hasLabel {
			return false
		}
	}
	if filter.Keyword != "" {
		keyword := strings.ToLower(filter.Keyword)
		if !strings.Contains(strings.ToLower(card.Title), keyword) &&
			!strings.Contains(strings.ToLower(card.Description), keyword) {
			return false
		}
	}
	return true
}

func removeString(slice []string, target string) []string {
	result := make([]string, 0, len(slice))
	for _, s := range slice {
		if s != target {
			result = append(result, s)
		}
	}
	return result
}
