// Package collabwhiteboard 提供协作白板功能
package collabwhiteboard

import (
	"errors"
	"sync"
	"time"
)

// Engine 白板引擎.
type Engine struct {
	mu            sync.RWMutex
	boards        map[string]*Board
	versions      map[string][]*Version
	operations    map[string][]*Operation
	collaborators map[string][]*Collaborator
	idCounter     int64
}

// NewEngine 创建白板引擎.
func NewEngine() *Engine {
	return &Engine{
		boards:        make(map[string]*Board),
		versions:      make(map[string][]*Version),
		operations:    make(map[string][]*Operation),
		collaborators: make(map[string][]*Collaborator),
	}
}

// generateID 生成唯一ID.
func (e *Engine) generateID(prefix string) string {
	e.idCounter++
	return prefix + "_" + time.Now().Format("20060102150405") + "_" + string(rune('A'+e.idCounter%26))
}

// CreateBoard 创建白板.
func (e *Engine) CreateBoard(req CreateBoardRequest) (*Board, error) {
	if req.Title == "" {
		return nil, errors.New("标题不能为空")
	}
	e.mu.Lock()
	defer e.mu.Unlock()

	width := req.Width
	if width <= 0 {
		width = 1920
	}
	height := req.Height
	if height <= 0 {
		height = 1080
	}

	board := &Board{
		ID:          e.generateID("board"),
		Title:       req.Title,
		Description: req.Description,
		Owner:       req.Owner,
		Width:       width,
		Height:      height,
		Elements:    make([]Element, 0),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	e.boards[board.ID] = board
	e.versions[board.ID] = make([]*Version, 0)
	e.operations[board.ID] = make([]*Operation, 0)
	e.collaborators[board.ID] = make([]*Collaborator, 0)

	// 添加所有者为协作者
	e.collaborators[board.ID] = append(e.collaborators[board.ID], &Collaborator{
		UserID:   req.Owner,
		Username: req.Owner,
		Role:     "owner",
		Color:    "#000000",
		JoinedAt: time.Now(),
	})

	return board, nil
}

// GetBoard 获取白板.
func (e *Engine) GetBoard(id string) (*Board, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	board, ok := e.boards[id]
	if !ok {
		return nil, errors.New("白板不存在")
	}
	return board, nil
}

// UpdateBoard 更新白板.
func (e *Engine) UpdateBoard(id string, req UpdateBoardRequest) (*Board, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	board, ok := e.boards[id]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	if req.Title != nil {
		board.Title = *req.Title
	}
	if req.Description != nil {
		board.Description = *req.Description
	}
	if req.Width != nil {
		board.Width = *req.Width
	}
	if req.Height != nil {
		board.Height = *req.Height
	}
	board.UpdatedAt = time.Now()

	return board, nil
}

// DeleteBoard 删除白板.
func (e *Engine) DeleteBoard(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.boards[id]; !ok {
		return errors.New("白板不存在")
	}

	delete(e.boards, id)
	delete(e.versions, id)
	delete(e.operations, id)
	delete(e.collaborators, id)

	return nil
}

// ListBoards 列出白板.
func (e *Engine) ListBoards(owner string) []*Board {
	e.mu.RLock()
	defer e.mu.RUnlock()

	result := make([]*Board, 0)
	for _, board := range e.boards {
		if owner == "" || board.Owner == owner {
			result = append(result, board)
		}
	}
	return result
}

// AddElement 添加元素.
func (e *Engine) AddElement(boardID string, req AddElementRequest, userID string) (*Element, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	board, ok := e.boards[boardID]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	element := Element{
		ID:        e.generateID("elem"),
		Type:      req.Type,
		X:         req.X,
		Y:         req.Y,
		Width:     req.Width,
		Height:    req.Height,
		Layer:     req.Layer,
		Visible:   true,
		Locked:    false,
		Style:     req.Style,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	board.Elements = append(board.Elements, element)
	board.UpdatedAt = time.Now()

	// 记录操作
	op := &Operation{
		ID:        e.generateID("op"),
		Type:      "add",
		ElementID: element.ID,
		Data:      element,
		UserID:    userID,
		Timestamp: time.Now(),
	}
	e.operations[boardID] = append(e.operations[boardID], op)

	return &element, nil
}

// UpdateElement 更新元素.
func (e *Engine) UpdateElement(boardID, elementID string, req UpdateElementRequest, userID string) (*Element, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	board, ok := e.boards[boardID]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	for i, elem := range board.Elements {
		if elem.ID == elementID {
			if req.X != nil {
				board.Elements[i].X = *req.X
			}
			if req.Y != nil {
				board.Elements[i].Y = *req.Y
			}
			if req.Width != nil {
				board.Elements[i].Width = *req.Width
			}
			if req.Height != nil {
				board.Elements[i].Height = *req.Height
			}
			if req.Rotation != nil {
				board.Elements[i].Rotation = *req.Rotation
			}
			if req.Layer != nil {
				board.Elements[i].Layer = *req.Layer
			}
			if req.Visible != nil {
				board.Elements[i].Visible = *req.Visible
			}
			if req.Locked != nil {
				board.Elements[i].Locked = *req.Locked
			}
			if req.Style != nil {
				board.Elements[i].Style = *req.Style
			}
			board.Elements[i].UpdatedAt = time.Now()
			board.UpdatedAt = time.Now()

			// 记录操作
			op := &Operation{
				ID:        e.generateID("op"),
				Type:      "update",
				ElementID: elementID,
				Data:      board.Elements[i],
				UserID:    userID,
				Timestamp: time.Now(),
			}
			e.operations[boardID] = append(e.operations[boardID], op)

			return &board.Elements[i], nil
		}
	}

	return nil, errors.New("元素不存在")
}

// DeleteElement 删除元素.
func (e *Engine) DeleteElement(boardID, elementID, userID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	board, ok := e.boards[boardID]
	if !ok {
		return errors.New("白板不存在")
	}

	for i, elem := range board.Elements {
		if elem.ID == elementID {
			board.Elements = append(board.Elements[:i], board.Elements[i+1:]...)
			board.UpdatedAt = time.Now()

			// 记录操作
			op := &Operation{
				ID:        e.generateID("op"),
				Type:      "delete",
				ElementID: elementID,
				Data:      elem,
				UserID:    userID,
				Timestamp: time.Now(),
			}
			e.operations[boardID] = append(e.operations[boardID], op)

			return nil
		}
	}

	return errors.New("元素不存在")
}

// MoveElement 移动元素.
func (e *Engine) MoveElement(boardID, elementID string, req MoveElementRequest, userID string) (*Element, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	board, ok := e.boards[boardID]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	for i, elem := range board.Elements {
		if elem.ID == elementID {
			board.Elements[i].X = req.X
			board.Elements[i].Y = req.Y
			board.Elements[i].UpdatedAt = time.Now()
			board.UpdatedAt = time.Now()

			// 记录操作
			op := &Operation{
				ID:        e.generateID("op"),
				Type:      "move",
				ElementID: elementID,
				Data:      board.Elements[i],
				UserID:    userID,
				Timestamp: time.Now(),
			}
			e.operations[boardID] = append(e.operations[boardID], op)

			return &board.Elements[i], nil
		}
	}

	return nil, errors.New("元素不存在")
}

// ResizeElement 调整元素大小.
func (e *Engine) ResizeElement(boardID, elementID string, req ResizeElementRequest, userID string) (*Element, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	board, ok := e.boards[boardID]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	for i, elem := range board.Elements {
		if elem.ID == elementID {
			board.Elements[i].Width = req.Width
			board.Elements[i].Height = req.Height
			board.Elements[i].UpdatedAt = time.Now()
			board.UpdatedAt = time.Now()

			// 记录操作
			op := &Operation{
				ID:        e.generateID("op"),
				Type:      "resize",
				ElementID: elementID,
				Data:      board.Elements[i],
				UserID:    userID,
				Timestamp: time.Now(),
			}
			e.operations[boardID] = append(e.operations[boardID], op)

			return &board.Elements[i], nil
		}
	}

	return nil, errors.New("元素不存在")
}

// GetElement 获取元素.
func (e *Engine) GetElement(boardID, elementID string) (*Element, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	board, ok := e.boards[boardID]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	for _, elem := range board.Elements {
		if elem.ID == elementID {
			return &elem, nil
		}
	}

	return nil, errors.New("元素不存在")
}

// GetElements 获取所有元素.
func (e *Engine) GetElements(boardID string) ([]Element, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	board, ok := e.boards[boardID]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	return board.Elements, nil
}

// SaveVersion 保存版本.
func (e *Engine) SaveVersion(boardID, userID, comment string) (*Version, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	board, ok := e.boards[boardID]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	elementsCopy := make([]Element, len(board.Elements))
	copy(elementsCopy, board.Elements)

	version := &Version{
		ID:        e.generateID("ver"),
		BoardID:   boardID,
		UserID:    userID,
		Elements:  elementsCopy,
		Comment:   comment,
		CreatedAt: time.Now(),
	}

	e.versions[boardID] = append(e.versions[boardID], version)
	return version, nil
}

// GetVersions 获取版本历史.
func (e *Engine) GetVersions(boardID string) ([]*Version, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	versions, ok := e.versions[boardID]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	return versions, nil
}

// RestoreVersion 恢复版本.
func (e *Engine) RestoreVersion(boardID, versionID, userID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	board, ok := e.boards[boardID]
	if !ok {
		return errors.New("白板不存在")
	}

	versions := e.versions[boardID]
	for _, ver := range versions {
		if ver.ID == versionID {
			board.Elements = make([]Element, len(ver.Elements))
			copy(board.Elements, ver.Elements)
			board.UpdatedAt = time.Now()

			// 记录操作
			op := &Operation{
				ID:        e.generateID("op"),
				Type:      "restore",
				ElementID: "",
				Data:      Element{},
				UserID:    userID,
				Timestamp: time.Now(),
			}
			e.operations[boardID] = append(e.operations[boardID], op)

			return nil
		}
	}

	return errors.New("版本不存在")
}

// GetOperations 获取操作历史.
func (e *Engine) GetOperations(boardID string, limit int) ([]*Operation, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	operations, ok := e.operations[boardID]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	if limit > 0 && limit < len(operations) {
		return operations[len(operations)-limit:], nil
	}
	return operations, nil
}

// AddCollaborator 添加协作者.
func (e *Engine) AddCollaborator(boardID string, req AddCollaboratorRequest) (*Collaborator, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.boards[boardID]; !ok {
		return nil, errors.New("白板不存在")
	}

	// 检查是否已存在
	for _, collab := range e.collaborators[boardID] {
		if collab.UserID == req.UserID {
			return nil, errors.New("协作者已存在")
		}
	}

	collab := &Collaborator{
		UserID:   req.UserID,
		Username: req.Username,
		Role:     req.Role,
		Color:    req.Color,
		JoinedAt: time.Now(),
	}

	e.collaborators[boardID] = append(e.collaborators[boardID], collab)
	return collab, nil
}

// RemoveCollaborator 移除协作者.
func (e *Engine) RemoveCollaborator(boardID, userID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	collabs, ok := e.collaborators[boardID]
	if !ok {
		return errors.New("白板不存在")
	}

	for i, collab := range collabs {
		if collab.UserID == userID {
			if collab.Role == "owner" {
				return errors.New("不能移除所有者")
			}
			e.collaborators[boardID] = append(collabs[:i], collabs[i+1:]...)
			return nil
		}
	}

	return errors.New("协作者不存在")
}

// GetCollaborators 获取协作者列表.
func (e *Engine) GetCollaborators(boardID string) ([]*Collaborator, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	collabs, ok := e.collaborators[boardID]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	return collabs, nil
}

// UpdateCollaboratorRole 更新协作者角色.
func (e *Engine) UpdateCollaboratorRole(boardID, userID, role string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	collabs, ok := e.collaborators[boardID]
	if !ok {
		return errors.New("白板不存在")
	}

	for _, collab := range collabs {
		if collab.UserID == userID {
			if collab.Role == "owner" {
				return errors.New("不能修改所有者角色")
			}
			collab.Role = role
			return nil
		}
	}

	return errors.New("协作者不存在")
}

// ClearBoard 清空白板.
func (e *Engine) ClearBoard(boardID, userID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	board, ok := e.boards[boardID]
	if !ok {
		return errors.New("白板不存在")
	}

	board.Elements = make([]Element, 0)
	board.UpdatedAt = time.Now()

	// 记录操作
	op := &Operation{
		ID:        e.generateID("op"),
		Type:      "clear",
		ElementID: "",
		Data:      Element{},
		UserID:    userID,
		Timestamp: time.Now(),
	}
	e.operations[boardID] = append(e.operations[boardID], op)

	return nil
}

// DuplicateElement 复制元素.
func (e *Engine) DuplicateElement(boardID, elementID, userID string) (*Element, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	board, ok := e.boards[boardID]
	if !ok {
		return nil, errors.New("白板不存在")
	}

	for _, elem := range board.Elements {
		if elem.ID == elementID {
			newElem := elem
			newElem.ID = e.generateID("elem")
			newElem.X += 20
			newElem.Y += 20
			newElem.CreatedAt = time.Now()
			newElem.UpdatedAt = time.Now()

			board.Elements = append(board.Elements, newElem)
			board.UpdatedAt = time.Now()

			// 记录操作
			op := &Operation{
				ID:        e.generateID("op"),
				Type:      "add",
				ElementID: newElem.ID,
				Data:      newElem,
				UserID:    userID,
				Timestamp: time.Now(),
			}
			e.operations[boardID] = append(e.operations[boardID], op)

			return &newElem, nil
		}
	}

	return nil, errors.New("元素不存在")
}
