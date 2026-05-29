package projectkanban

import (
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 看板管理器
type Manager struct {
	boards map[string]*Board
	mu     sync.RWMutex
}

// NewManager 创建管理器
func NewManager() *Manager {
	return &Manager{
		boards: make(map[string]*Board),
	}
}

// ListBoards 列出所有看板
func (m *Manager) ListBoards() []*Board {
	m.mu.RLock()
	defer m.mu.RUnlock()

	boards := make([]*Board, 0, len(m.boards))
	for _, b := range m.boards {
		if !b.Archived {
			boards = append(boards, b)
		}
	}
	return boards
}

// CreateBoard 创建看板
func (m *Manager) CreateBoard(req *CreateBoardRequest) *Board {
	m.mu.Lock()
	defer m.mu.Unlock()

	board := &Board{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Members:     req.Members,
		CreatedAt:   time.Now(),
		Columns: []Column{
			{ID: uuid.New().String(), Name: "待办", Order: 1, Cards: []Card{}},
			{ID: uuid.New().String(), Name: "进行中", Order: 2, Cards: []Card{}},
			{ID: uuid.New().String(), Name: "已完成", Order: 3, Cards: []Card{}},
		},
	}

	m.boards[board.ID] = board
	return board
}

// GetBoard 获取看板详情
func (m *Manager) GetBoard(id string) (*Board, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, ok := m.boards[id]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", id)
	}
	return board, nil
}

// ArchiveBoard 归档看板
func (m *Manager) ArchiveBoard(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[id]
	if !ok {
		return fmt.Errorf("board not found: %s", id)
	}
	board.Archived = true
	return nil
}

// CreateCard 创建卡片
func (m *Manager) CreateCard(boardID string, req *CreateCardRequest) (*Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	// 查找目标列
	var targetCol *Column
	for i := range board.Columns {
		if board.Columns[i].ID == req.ColumnID {
			targetCol = &board.Columns[i]
			break
		}
	}
	if targetCol == nil {
		return nil, fmt.Errorf("column not found: %s", req.ColumnID)
	}

	// 验证优先级
	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}
	if !isValidPriority(priority) {
		return nil, fmt.Errorf("invalid priority: %s", priority)
	}

	card := Card{
		ID:          uuid.New().String(),
		Title:       req.Title,
		Description: req.Description,
		Priority:    priority,
		Labels:      req.Labels,
		Assignee:    req.Assignee,
		DueDate:     req.DueDate,
		Attachments: req.Attachments,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	targetCol.Cards = append(targetCol.Cards, card)
	return &card, nil
}

// UpdateCard 更新卡片
func (m *Manager) UpdateCard(boardID, cardID string, req *UpdateCardRequest) (*Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	// 查找卡片所在的列
	for colIdx := range board.Columns {
		for cardIdx := range board.Columns[colIdx].Cards {
			if board.Columns[colIdx].Cards[cardIdx].ID == cardID {
				card := &board.Columns[colIdx].Cards[cardIdx]

				// 更新字段
				if req.Title != "" {
					card.Title = req.Title
				}
				if req.Description != "" {
					card.Description = req.Description
				}
				if req.Priority != "" {
					if !isValidPriority(req.Priority) {
						return nil, fmt.Errorf("invalid priority: %s", req.Priority)
					}
					card.Priority = req.Priority
				}
				if req.Labels != nil {
					card.Labels = req.Labels
				}
				if req.Assignee != "" {
					card.Assignee = req.Assignee
				}
				if req.DueDate != nil {
					card.DueDate = req.DueDate
				}
				if req.Attachments != nil {
					card.Attachments = req.Attachments
				}

				// 处理列移动
				if req.ColumnID != "" && req.ColumnID != board.Columns[colIdx].ID {
					// 从当前列移除
					board.Columns[colIdx].Cards = append(
						board.Columns[colIdx].Cards[:cardIdx],
						board.Columns[colIdx].Cards[cardIdx+1:]...,
					)

					// 找到目标列
					var targetCol *Column
					for i := range board.Columns {
						if board.Columns[i].ID == req.ColumnID {
							targetCol = &board.Columns[i]
							break
						}
					}
					if targetCol == nil {
						return nil, fmt.Errorf("target column not found: %s", req.ColumnID)
					}
					targetCol.Cards = append(targetCol.Cards, *card)
				}

				card.UpdatedAt = time.Now()
				return card, nil
			}
		}
	}

	return nil, fmt.Errorf("card not found: %s", cardID)
}

// MoveCard 移动卡片到另一列
func (m *Manager) MoveCard(boardID, cardID string, req *MoveCardRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, ok := m.boards[boardID]
	if !ok {
		return fmt.Errorf("board not found: %s", boardID)
	}

	// 查找并移除卡片
	var card Card
	found := false
	for colIdx := range board.Columns {
		for cardIdx := range board.Columns[colIdx].Cards {
			if board.Columns[colIdx].Cards[cardIdx].ID == cardID {
				card = board.Columns[colIdx].Cards[cardIdx]
				board.Columns[colIdx].Cards = append(
					board.Columns[colIdx].Cards[:cardIdx],
					board.Columns[colIdx].Cards[cardIdx+1:]...,
				)
				found = true
				break
			}
		}
		if found {
			break
		}
	}
	if !found {
		return fmt.Errorf("card not found: %s", cardID)
	}

	// 找到目标列
	var targetCol *Column
	for i := range board.Columns {
		if board.Columns[i].ID == req.TargetColumnID {
			targetCol = &board.Columns[i]
			break
		}
	}
	if targetCol == nil {
		return fmt.Errorf("target column not found: %s", req.TargetColumnID)
	}

	// 插入到指定位置
	if req.Position < 0 || req.Position >= len(targetCol.Cards) {
		targetCol.Cards = append(targetCol.Cards, card)
	} else {
		targetCol.Cards = append(targetCol.Cards[:req.Position+1], targetCol.Cards[req.Position:]...)
		targetCol.Cards[req.Position] = card
	}

	card.UpdatedAt = time.Now()
	return nil
}

// GetBoardStats 获取看板统计
func (m *Manager) GetBoardStats(boardID string) (*BoardStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	stats := &BoardStats{}
	now := time.Now()

	// 获取最后一列作为"已完成"列
	completedColumnID := ""
	if len(board.Columns) > 0 {
		completedColumnID = board.Columns[len(board.Columns)-1].ID
	}

	for _, col := range board.Columns {
		stats.TotalCards += len(col.Cards)
		if col.ID == completedColumnID {
			stats.CompletedCards += len(col.Cards)
		}

		// 统计延期任务
		for _, card := range col.Cards {
			if card.DueDate != nil && card.DueDate.Before(now) && col.ID != completedColumnID {
				stats.OverdueCards++
			}
		}
	}

	if stats.TotalCards > 0 {
		stats.CompletionRate = float64(stats.CompletedCards) / float64(stats.TotalCards) * 100
	}

	return stats, nil
}

// GetColumnCards 获取指定列的卡片
func (m *Manager) GetColumnCards(boardID, columnID string) ([]Card, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, ok := m.boards[boardID]
	if !ok {
		return nil, fmt.Errorf("board not found: %s", boardID)
	}

	for _, col := range board.Columns {
		if col.ID == columnID {
			return col.Cards, nil
		}
	}

	return nil, fmt.Errorf("column not found: %s", columnID)
}

// isValidPriority 验证优先级
func isValidPriority(priority string) bool {
	switch priority {
	case "low", "medium", "high", "urgent":
		return true
	default:
		return false
	}
}
