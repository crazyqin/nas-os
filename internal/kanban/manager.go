// Package kanban provides Kanban board management functionality.
package kanban

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager manages Kanban boards.
type Manager struct {
	mu          sync.RWMutex
	boards      map[string]*Board
	templates   map[string]*BoardTemplate
	configPath  string
}

// NewManager creates a new Kanban manager.
func NewManager(configPath string) *Manager {
	m := &Manager{
		boards:     make(map[string]*Board),
		templates:  make(map[string]*BoardTemplate),
		configPath: configPath,
	}

	// Load existing config
	if err := m.loadConfig(); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to load kanban config: %v", err)
	}

	// Initialize default templates if empty
	if len(m.templates) == 0 {
		m.initDefaultTemplates()
	}

	return m
}

// initDefaultTemplates initializes default board templates.
func (m *Manager) initDefaultTemplates() {
	m.templates["basic"] = &BoardTemplate{
		ID:          "basic",
		Name:        "基础看板",
		Description: "基础的三列看板模板",
		Columns:     []string{"待办", "进行中", "已完成"},
		IsDefault:   true,
		CreatedAt:   time.Now(),
	}

	m.templates["scrum"] = &BoardTemplate{
		ID:          "scrum",
		Name:        "Scrum看板",
		Description: "Scrum敏捷开发看板模板",
		Columns:     []string{"待办", "开发中", "测试中", "已完成"},
		CreatedAt:   time.Now(),
	}

	m.templates["bug"] = &BoardTemplate{
		ID:          "bug",
		Name:        "Bug追踪",
		Description: "Bug追踪看板模板",
		Columns:     []string{"新Bug", "确认中", "修复中", "验证中", "已关闭"},
		CreatedAt:   time.Now(),
	}

	log.Printf("Initialized %d default kanban templates", len(m.templates))
}

// CreateBoard creates a new Kanban board.
func (m *Manager) CreateBoard(req CreateBoardRequest, ownerID string) (*Board, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	boardID := uuid.New().String()
	now := time.Now()

	board := &Board{
		ID:          boardID,
		Name:        req.Name,
		Description: req.Description,
		Columns:     make([]*Column, 0),
		Tags:        make([]*Tag, 0),
		Members:     make([]*Member, 0),
		OwnerID:     ownerID,
		IsPublic:    req.IsPublic,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	// Add owner as member
	board.Members = append(board.Members, &Member{
		UserID:   ownerID,
		Role:     "owner",
		JoinedAt: now,
	})

	// Create columns from template or default
	if req.TemplateID != "" {
		template, exists := m.templates[req.TemplateID]
		if !exists {
			return nil, fmt.Errorf("template %s not found", req.TemplateID)
		}
		for i, colName := range template.Columns {
			board.Columns = append(board.Columns, &Column{
				ID:        uuid.New().String(),
				BoardID:   boardID,
				Name:      colName,
				Position:  i,
				Cards:     make([]*Card, 0),
				CreatedAt: now,
			})
		}
		// Copy template tags
		for _, tag := range template.Tags {
			board.Tags = append(board.Tags, &Tag{
				ID:      uuid.New().String(),
				BoardID: boardID,
				Name:    tag.Name,
				Color:   tag.Color,
			})
		}
	} else {
		// Default columns: 待办, 进行中, 已完成
		defaultColumns := []string{"待办", "进行中", "已完成"}
		for i, colName := range defaultColumns {
			board.Columns = append(board.Columns, &Column{
				ID:        uuid.New().String(),
				BoardID:   boardID,
				Name:      colName,
				Position:  i,
				Cards:     make([]*Card, 0),
				CreatedAt: now,
			})
		}
	}

	m.boards[boardID] = board

	log.Printf("Created kanban board: %s (%s)", board.Name, boardID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save kanban config: %v", err)
	}

	return board, nil
}

// GetBoard returns a board by ID.
func (m *Manager) GetBoard(boardID string) (*Board, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	board, exists := m.boards[boardID]
	if !exists {
		return nil, fmt.Errorf("board %s not found", boardID)
	}

	return board, nil
}

// ListBoards returns all boards.
func (m *Manager) ListBoards() []*Board {
	m.mu.RLock()
	defer m.mu.RUnlock()

	boards := make([]*Board, 0, len(m.boards))
	for _, board := range m.boards {
		boards = append(boards, board)
	}

	// Sort by creation time
	sort.Slice(boards, func(i, j int) bool {
		return boards[i].CreatedAt.After(boards[j].CreatedAt)
	})

	return boards
}

// UpdateBoard updates a board.
func (m *Manager) UpdateBoard(boardID string, req UpdateBoardRequest) (*Board, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, exists := m.boards[boardID]
	if !exists {
		return nil, fmt.Errorf("board %s not found", boardID)
	}

	if req.Name != nil {
		board.Name = *req.Name
	}
	if req.Description != nil {
		board.Description = *req.Description
	}
	if req.IsPublic != nil {
		board.IsPublic = *req.IsPublic
	}

	board.UpdatedAt = time.Now()

	log.Printf("Updated kanban board: %s", boardID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save kanban config: %v", err)
	}

	return board, nil
}

// DeleteBoard deletes a board.
func (m *Manager) DeleteBoard(boardID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.boards[boardID]; !exists {
		return fmt.Errorf("board %s not found", boardID)
	}

	delete(m.boards, boardID)

	log.Printf("Deleted kanban board: %s", boardID)

	return m.saveConfig()
}

// CreateCard creates a new card in a board.
func (m *Manager) CreateCard(boardID string, req CreateCardRequest, createdBy string) (*Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, exists := m.boards[boardID]
	if !exists {
		return nil, fmt.Errorf("board %s not found", boardID)
	}

	// Find target column
	var targetColumn *Column
	for _, col := range board.Columns {
		if col.ID == req.ColumnID {
			targetColumn = col
			break
		}
	}

	if targetColumn == nil {
		return nil, fmt.Errorf("column %s not found in board %s", req.ColumnID, boardID)
	}

	// Check WIP limit
	if targetColumn.WIPLimit > 0 && len(targetColumn.Cards) >= targetColumn.WIPLimit {
		return nil, fmt.Errorf("column %s has reached WIP limit (%d)", targetColumn.Name, targetColumn.WIPLimit)
	}

	now := time.Now()
	card := &Card{
		ID:          uuid.New().String(),
		ColumnID:    req.ColumnID,
		BoardID:     boardID,
		Title:       req.Title,
		Description: req.Description,
		Position:    len(targetColumn.Cards),
		Priority:    req.Priority,
		AssigneeID:  req.AssigneeID,
		Tags:        req.Tags,
		Comments:    make([]*Comment, 0),
		Attachments: make([]*Attachment, 0),
		DueDate:     req.DueDate,
		CreatedBy:   createdBy,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if card.Priority == "" {
		card.Priority = "medium"
	}

	targetColumn.Cards = append(targetColumn.Cards, card)
	board.UpdatedAt = now

	log.Printf("Created card %s in board %s, column %s", card.ID, boardID, req.ColumnID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save kanban config: %v", err)
	}

	return card, nil
}

// MoveCard moves a card to another column or position.
func (m *Manager) MoveCard(boardID, cardID string, req MoveCardRequest) (*Card, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, exists := m.boards[boardID]
	if !exists {
		return nil, fmt.Errorf("board %s not found", boardID)
	}

	// Find and remove card from source column
	var card *Card
	var sourceColumn *Column
	for _, col := range board.Columns {
		for i, c := range col.Cards {
			if c.ID == cardID {
				card = c
				sourceColumn = col
				// Remove from source
				col.Cards = append(col.Cards[:i], col.Cards[i+1:]...)
				break
			}
		}
		if card != nil {
			break
		}
	}

	if card == nil {
		return nil, fmt.Errorf("card %s not found in board %s", cardID, boardID)
	}

	// Find target column
	var targetColumn *Column
	for _, col := range board.Columns {
		if col.ID == req.TargetColumnID {
			targetColumn = col
			break
		}
	}

	if targetColumn == nil {
		return nil, fmt.Errorf("target column %s not found", req.TargetColumnID)
	}

	// Check WIP limit
	if targetColumn.WIPLimit > 0 && len(targetColumn.Cards) >= targetColumn.WIPLimit {
		// Put card back in source column
		sourceColumn.Cards = append(sourceColumn.Cards, card)
		return nil, fmt.Errorf("target column %s has reached WIP limit (%d)", targetColumn.Name, targetColumn.WIPLimit)
	}

	// Update card
	card.ColumnID = req.TargetColumnID
	card.UpdatedAt = time.Now()

	// Insert at position
	position := req.Position
	if position < 0 || position > len(targetColumn.Cards) {
		position = len(targetColumn.Cards)
	}

	// Insert card at position
	targetColumn.Cards = append(targetColumn.Cards, nil)
	copy(targetColumn.Cards[position+1:], targetColumn.Cards[position:])
	targetColumn.Cards[position] = card

	// Update positions
	for i, c := range targetColumn.Cards {
		c.Position = i
	}

	board.UpdatedAt = time.Now()

	log.Printf("Moved card %s to column %s at position %d", cardID, req.TargetColumnID, position)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save kanban config: %v", err)
	}

	return card, nil
}

// AddComment adds a comment to a card.
func (m *Manager) AddComment(boardID, cardID, userID, username, content string) (*Comment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, exists := m.boards[boardID]
	if !exists {
		return nil, fmt.Errorf("board %s not found", boardID)
	}

	// Find card
	var card *Card
	for _, col := range board.Columns {
		for _, c := range col.Cards {
			if c.ID == cardID {
				card = c
				break
			}
		}
		if card != nil {
			break
		}
	}

	if card == nil {
		return nil, fmt.Errorf("card %s not found", cardID)
	}

	now := time.Now()
	comment := &Comment{
		ID:        uuid.New().String(),
		CardID:    cardID,
		UserID:    userID,
		Username:  username,
		Content:   content,
		CreatedAt: now,
		UpdatedAt: now,
	}

	card.Comments = append(card.Comments, comment)
	card.UpdatedAt = now
	board.UpdatedAt = now

	log.Printf("Added comment to card %s", cardID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save kanban config: %v", err)
	}

	return comment, nil
}

// AddTag adds a tag to a board.
func (m *Manager) AddTag(boardID, name, color string) (*Tag, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, exists := m.boards[boardID]
	if !exists {
		return nil, fmt.Errorf("board %s not found", boardID)
	}

	// Check for duplicate tag name
	for _, tag := range board.Tags {
		if tag.Name == name {
			return nil, fmt.Errorf("tag %s already exists", name)
		}
	}

	tag := &Tag{
		ID:      uuid.New().String(),
		BoardID: boardID,
		Name:    name,
		Color:   color,
	}

	board.Tags = append(board.Tags, tag)
	board.UpdatedAt = time.Now()

	log.Printf("Added tag %s to board %s", name, boardID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save kanban config: %v", err)
	}

	return tag, nil
}

// AddMember adds a member to a board.
func (m *Manager) AddMember(boardID, userID, username, role string) (*Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	board, exists := m.boards[boardID]
	if !exists {
		return nil, fmt.Errorf("board %s not found", boardID)
	}

	// Check if already a member
	for _, member := range board.Members {
		if member.UserID == userID {
			return nil, fmt.Errorf("user %s is already a member", userID)
		}
	}

	member := &Member{
		UserID:   userID,
		Username: username,
		Role:     role,
		JoinedAt: time.Now(),
	}

	board.Members = append(board.Members, member)
	board.UpdatedAt = time.Now()

	log.Printf("Added member %s to board %s with role %s", userID, boardID, role)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save kanban config: %v", err)
	}

	return member, nil
}

// GetTemplates returns all board templates.
func (m *Manager) GetTemplates() []*BoardTemplate {
	m.mu.RLock()
	defer m.mu.RUnlock()

	templates := make([]*BoardTemplate, 0, len(m.templates))
	for _, t := range m.templates {
		templates = append(templates, t)
	}

	return templates
}

// saveConfig saves configuration to disk.
func (m *Manager) saveConfig() error {
	cfg := struct {
		Boards    map[string]*Board         `json:"boards"`
		Templates map[string]*BoardTemplate `json:"templates"`
	}{
		Boards:    m.boards,
		Templates: m.templates,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Ensure directory exists
	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0644)
}

// loadConfig loads configuration from disk.
func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	var cfg struct {
		Boards    map[string]*Board         `json:"boards"`
		Templates map[string]*BoardTemplate `json:"templates"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Boards != nil {
		m.boards = cfg.Boards
	}
	if cfg.Templates != nil {
		m.templates = cfg.Templates
	}

	return nil
}
