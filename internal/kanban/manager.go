// Package kanban 提供看板核心管理逻辑
package kanban

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"time"
)

// Manager 看板管理器
type Manager struct {
	mu       sync.RWMutex
	boards   map[string]*Board
	activity []*Activity
}

// NewManager 创建看板管理器
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

// ListBoards 列出所有看板
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

// CreateBoard 创建看板
func (m *Manager) CreateBoard(req *CreateBoardRequest) (*Board, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	board := &Board{
		ID:          generateID(),
		Name:        req.Name,
		Description: req.Description,
		Columns: []*Column{
			{ID: generateID(), Name: "待办", Position: 0, Cards: make([]*Card, 0), CreatedAt: now},
			{ID: generateID(), Name: "进行中", Position: 1, Cards: make([]*Card, 0), CreatedAt: now},
			{ID: generateID(), Name: "已完成", Position: 2, Cards: make([]*Card, 0), CreatedAt: now},
		},
		Labels: make([]*Label, 0),
		Members: []*Member{{
			UserID:   req.OwnerID,
			Role:     "owner",
			JoinedAt: now,
		}},
		OwnerID:    req.OwnerID,
		IsArchived: false,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	// 回填 boardID
	for _, col := range board.Columns {
		col.BoardID = board.ID
	}

	m.boards[board.ID] = board
	m.addActivity(board.ID, "", req.OwnerID, "board_created", fmt.Sprintf("创建看板: %s", req.Name))

	return board, nil
}

// GetBoard 获取看板
func (m *Manager) GetBoard(id string) (*Board, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, ok := m.boards[id]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", id)
	}
	return board, nil
}

// DeleteBoard 删除看板
func (m *Manager) DeleteBoard(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.boards[id]; !ok {
		return fmt.Errorf("board not found: %s", id)
	}
	delete(m.boards, id)
	return nil
}

// AddCard 添加卡片
func (m *Manager) AddCard(boardID string, req *AddCardRequest) (*Card, error) {
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

	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}

	now := time.Now()
	card := &Card{
		ID:          generateID(),
		ColumnID:    req.ColumnID,
		BoardID:     boardID,
		Title:       req.Title,
		Description: req.Description,
		Position:    len(targetCol.Cards),
		Priority:    priority,
		AssigneeID:  req.AssigneeID,
		LabelIDs:    req.LabelIDs,
		DueDate:     req.DueDate,
		CreatedBy:   req.CreatedBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	targetCol.Cards = append(targetCol.Cards, card)
	board.UpdatedAt = now

	m.addActivity(boardID, card.ID, req.CreatedBy, "card_created", fmt.Sprintf("添加卡片: %s", req.Title))

	return card, nil
}

// MoveCard 移动卡片（拖拽）
func (m *Manager) MoveCard(boardID, cardID string, req *MoveCardRequest) (*Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	// 查找卡片所在列
	var card *Card
	var srcCol *Column
	for _, col := range board.Columns {
		for i, c := range col.Cards {
			if c.ID == cardID {
				card = c
				srcCol = col
				// 从源列移除
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

	// 插入到目标列
	pos := req.Position
	if pos < 0 || pos > len(dstCol.Cards) {
		pos = len(dstCol.Cards)
	}
	dstCol.Cards = append(dstCol.Cards[:pos], append([]*Card{card}, dstCol.Cards[pos:]...)...)

	// 更新卡片信息
	card.ColumnID = req.TargetColumnID
	card.Position = pos
	card.UpdatedAt = time.Now()

	// 重排位置
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

// AddLabel 添加标签
func (m *Manager) AddLabel(boardID string, req *AddLabelRequest) (*Label, error) {
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

// DeleteLabel 删除标签
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
			board.UpdatedAt = time.Now()
			return nil
		}
	}
	return fmt.Errorf("label not found: %s", labelID)
}

// AssignMember 分配成员
func (m *Manager) AssignMember(boardID string, req *AssignMemberRequest) (*Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	// 检查是否已存在
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

// RemoveMember 移除成员
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

// GetActivity 获取活动记录
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

// GetBoardProgress 获取看板进度
func (m *Manager) GetBoardProgress(boardID string) (map[string]int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	progress := make(map[string]int)
	total := 0
	for _, col := range board.Columns {
		count := len(col.Cards)
		progress[col.Name] = count
		total += count
	}
	progress["total"] = total

	return progress, nil
}
