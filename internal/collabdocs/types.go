// Package collabdocs provides collaborative document editing for NAS-OS
// Features: Real-time collaboration, markdown/WYSIWYG, version history
// Competitor benchmark: 对标群晖Office, 超越飞牛文档
package collabdocs

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// DocumentType represents the type of document
type DocumentType string

const (
	DocTypeMarkdown DocumentType = "markdown"
	DocTypeRichText DocumentType = "richtext"
	DocTypeSheet    DocumentType = "sheet"
	DocTypeSlides   DocumentType = "slides"
	DocTypeDiagram  DocumentType = "diagram"
)

// Permission represents document permission
type Permission string

const (
	PermOwner   Permission = "owner"
	PermEdit    Permission = "edit"
	PermComment Permission = "comment"
	PermView    Permission = "view"
)

// Document represents a collaborative document
type Document struct {
	ID            string         `json:"id"`
	Title         string         `json:"title"`
	Type          DocumentType   `json:"type"`
	Content       string         `json:"content"`
	Owner         string         `json:"owner"`
	Collaborators []Collaborator `json:"collaborators"`
	Version       int            `json:"version"`
	IsPublic      bool           `json:"is_public"`
	Tags          []string       `json:"tags"`
	Size          int64          `json:"size"`
	CreatedAt     time.Time      `json:"created_at"`
	UpdatedAt     time.Time      `json:"updated_at"`
	UpdatedBy     string         `json:"updated_by"`
}

// Collaborator represents a document collaborator
type Collaborator struct {
	UserID     string     `json:"user_id"`
	Permission Permission `json:"permission"`
	AddedAt    time.Time  `json:"added_at"`
	AddedBy    string     `json:"added_by"`
}

// Comment represents a document comment
type Comment struct {
	ID        string    `json:"id"`
	DocID     string    `json:"doc_id"`
	UserID    string    `json:"user_id"`
	Content   string    `json:"content"`
	ParentID  string    `json:"parent_id,omitempty"`
	Resolved  bool      `json:"resolved"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Version represents a document version
type Version struct {
	ID        string    `json:"id"`
	DocID     string    `json:"doc_id"`
	Version   int       `json:"version"`
	Content   string    `json:"content"`
	UserID    string    `json:"user_id"`
	Message   string    `json:"message"`
	Size      int64     `json:"size"`
	CreatedAt time.Time `json:"created_at"`
}

// Change represents a real-time change
type Change struct {
	ID        string    `json:"id"`
	DocID     string    `json:"doc_id"`
	UserID    string    `json:"user_id"`
	Op        string    `json:"op"` // insert, delete, format
	Position  int       `json:"position"`
	Content   string    `json:"content"`
	Length    int       `json:"length"`
	Timestamp time.Time `json:"timestamp"`
}

// Cursor represents a user's cursor position
type Cursor struct {
	UserID    string     `json:"user_id"`
	DocID     string     `json:"doc_id"`
	Position  int        `json:"position"`
	Selection *Selection `json:"selection,omitempty"`
	UpdatedAt time.Time  `json:"updated_at"`
}

// Selection represents a text selection
type Selection struct {
	Start int `json:"start"`
	End   int `json:"end"`
}

// Session represents an editing session
type Session struct {
	ID        string    `json:"id"`
	DocID     string    `json:"doc_id"`
	UserID    string    `json:"user_id"`
	Cursor    *Cursor   `json:"cursor,omitempty"`
	Connected bool      `json:"connected"`
	StartedAt time.Time `json:"started_at"`
	LastPing  time.Time `json:"last_ping"`
}

// Template represents a document template
type Template struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Type        DocumentType `json:"type"`
	Content     string       `json:"content"`
	Category    string       `json:"category"`
	Icon        string       `json:"icon"`
	CreatedAt   time.Time    `json:"created_at"`
}

// Config represents collaborative docs configuration
type Config struct {
	Enabled              bool `json:"enabled"`
	MaxDocuments         int  `json:"max_documents"`
	MaxVersions          int  `json:"max_versions"`
	AutoSaveInterval     int  `json:"auto_save_interval"` // seconds
	CollaborationEnabled bool `json:"collaboration_enabled"`
	PublicSharing        bool `json:"public_sharing"`
	MaxFileSize          int  `json:"max_file_size"` // bytes
}

// Manager manages collaborative documents
type Manager struct {
	config     *Config
	documents  map[string]*Document
	comments   map[string][]*Comment
	versions   map[string][]*Version
	sessions   map[string]*Session
	templates  []*Template
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
	changeChan chan *Change
}

// NewManager creates a new collaborative docs manager
func NewManager(config *Config) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		config:     config,
		documents:  make(map[string]*Document),
		comments:   make(map[string][]*Comment),
		versions:   make(map[string][]*Version),
		sessions:   make(map[string]*Session),
		templates:  make([]*Template, 0),
		ctx:        ctx,
		cancel:     cancel,
		changeChan: make(chan *Change, 1000),
	}
}

// Start starts the collaborative docs manager
func (m *Manager) Start() error {
	if !m.config.Enabled {
		return nil
	}

	// Initialize default templates
	m.initDefaultTemplates()

	// Start auto-save
	go m.autoSave()

	// Start session cleanup
	go m.cleanupSessions()

	return nil
}

// Stop stops the collaborative docs manager
func (m *Manager) Stop() {
	m.cancel()
}

// initDefaultTemplates initializes default templates
func (m *Manager) initDefaultTemplates() {
	m.templates = []*Template{
		{
			ID:          "tpl_meeting",
			Name:        "会议纪要",
			Description: "标准会议纪要模板",
			Type:        DocTypeMarkdown,
			Category:    "business",
			Icon:        "📋",
		},
		{
			ID:          "tpl_report",
			Name:        "工作报告",
			Description: "周报/月报模板",
			Type:        DocTypeMarkdown,
			Category:    "business",
			Icon:        "📊",
		},
		{
			ID:          "tpl_readme",
			Name:        "README",
			Description: "项目README模板",
			Type:        DocTypeMarkdown,
			Category:    "dev",
			Icon:        "📖",
		},
		{
			ID:          "tpl_notes",
			Name:        "笔记",
			Description: "通用笔记模板",
			Type:        DocTypeMarkdown,
			Category:    "personal",
			Icon:        "📝",
		},
	}
}

// autoSave auto-saves documents
func (m *Manager) autoSave() {
	ticker := time.NewTicker(time.Duration(m.config.AutoSaveInterval) * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.saveAllDocuments()
		}
	}
}

// saveAllDocuments saves all modified documents
func (m *Manager) saveAllDocuments() {
	// Auto-save implementation
}

// cleanupSessions cleans up inactive sessions
func (m *Manager) cleanupSessions() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.removeInactiveSessions()
		}
	}
}

// removeInactiveSessions removes inactive sessions
func (m *Manager) removeInactiveSessions() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().Add(-10 * time.Minute)
	for id, session := range m.sessions {
		if session.LastPing.Before(cutoff) {
			delete(m.sessions, id)
		}
	}
}

// CreateDocument creates a new document
func (m *Manager) CreateDocument(doc *Document) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if doc.ID == "" {
		doc.ID = fmt.Sprintf("doc_%d", time.Now().UnixNano())
	}
	doc.Version = 1
	doc.CreatedAt = time.Now()
	doc.UpdatedAt = time.Now()

	m.documents[doc.ID] = doc

	// Create initial version
	m.versions[doc.ID] = []*Version{
		{
			ID:        fmt.Sprintf("ver_%d", time.Now().UnixNano()),
			DocID:     doc.ID,
			Version:   1,
			Content:   doc.Content,
			UserID:    doc.Owner,
			Message:   "Initial version",
			CreatedAt: time.Now(),
		},
	}

	return nil
}

// GetDocument returns a document by ID
func (m *Manager) GetDocument(docID string) (*Document, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	doc, ok := m.documents[docID]
	if !ok {
		return nil, fmt.Errorf("document not found: %s", docID)
	}
	return doc, nil
}

// UpdateDocument updates a document
func (m *Manager) UpdateDocument(docID string, content string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, ok := m.documents[docID]
	if !ok {
		return fmt.Errorf("document not found: %s", docID)
	}

	doc.Content = content
	doc.Version++
	doc.UpdatedAt = time.Now()
	doc.UpdatedBy = userID
	doc.Size = int64(len(content))

	// Save version
	if len(m.versions[docID]) >= m.config.MaxVersions {
		m.versions[docID] = m.versions[docID][1:]
	}
	m.versions[docID] = append(m.versions[docID], &Version{
		ID:        fmt.Sprintf("ver_%d", time.Now().UnixNano()),
		DocID:     docID,
		Version:   doc.Version,
		Content:   content,
		UserID:    userID,
		CreatedAt: time.Now(),
	})

	return nil
}

// DeleteDocument deletes a document
func (m *Manager) DeleteDocument(docID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.documents[docID]; !ok {
		return fmt.Errorf("document not found: %s", docID)
	}

	delete(m.documents, docID)
	delete(m.comments, docID)
	delete(m.versions, docID)

	return nil
}

// ListDocuments lists documents for a user
func (m *Manager) ListDocuments(userID string) []*Document {
	m.mu.RLock()
	defer m.mu.RUnlock()

	docs := make([]*Document, 0)
	for _, doc := range m.documents {
		if doc.Owner == userID || doc.IsPublic {
			docs = append(docs, doc)
		} else {
			for _, collab := range doc.Collaborators {
				if collab.UserID == userID {
					docs = append(docs, doc)
					break
				}
			}
		}
	}
	return docs
}

// AddCollaborator adds a collaborator to a document
func (m *Manager) AddCollaborator(docID string, collab Collaborator) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, ok := m.documents[docID]
	if !ok {
		return fmt.Errorf("document not found: %s", docID)
	}

	collab.AddedAt = time.Now()
	doc.Collaborators = append(doc.Collaborators, collab)

	return nil
}

// RemoveCollaborator removes a collaborator
func (m *Manager) RemoveCollaborator(docID string, userID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	doc, ok := m.documents[docID]
	if !ok {
		return fmt.Errorf("document not found: %s", docID)
	}

	for i, collab := range doc.Collaborators {
		if collab.UserID == userID {
			doc.Collaborators = append(doc.Collaborators[:i], doc.Collaborators[i+1:]...)
			break
		}
	}

	return nil
}

// AddComment adds a comment to a document
func (m *Manager) AddComment(comment *Comment) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.documents[comment.DocID]; !ok {
		return fmt.Errorf("document not found: %s", comment.DocID)
	}

	if comment.ID == "" {
		comment.ID = fmt.Sprintf("cmt_%d", time.Now().UnixNano())
	}
	comment.CreatedAt = time.Now()
	comment.UpdatedAt = time.Now()

	m.comments[comment.DocID] = append(m.comments[comment.DocID], comment)
	return nil
}

// GetComments returns comments for a document
func (m *Manager) GetComments(docID string) []*Comment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.comments[docID]
}

// GetVersions returns versions for a document
func (m *Manager) GetVersions(docID string) []*Version {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.versions[docID]
}

// JoinSession joins an editing session
func (m *Manager) JoinSession(docID string, userID string) *Session {
	m.mu.Lock()
	defer m.mu.Unlock()

	session := &Session{
		ID:        fmt.Sprintf("sess_%d", time.Now().UnixNano()),
		DocID:     docID,
		UserID:    userID,
		Connected: true,
		StartedAt: time.Now(),
		LastPing:  time.Now(),
	}

	m.sessions[session.ID] = session
	return session
}

// LeaveSession leaves an editing session
func (m *Manager) LeaveSession(sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if session, ok := m.sessions[sessionID]; ok {
		session.Connected = false
		delete(m.sessions, sessionID)
	}
}

// ApplyChange applies a real-time change
func (m *Manager) ApplyChange(change *Change) error {
	m.changeChan <- change
	return nil
}

// GetTemplates returns document templates
func (m *Manager) GetTemplates() []*Template {
	return m.templates
}

// GetStats returns document statistics
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalComments := 0
	for _, comments := range m.comments {
		totalComments += len(comments)
	}

	totalVersions := 0
	for _, versions := range m.versions {
		totalVersions += len(versions)
	}

	activeSessions := 0
	for _, session := range m.sessions {
		if session.Connected {
			activeSessions++
		}
	}

	return map[string]interface{}{
		"total_documents":       len(m.documents),
		"total_comments":        totalComments,
		"total_versions":        totalVersions,
		"active_sessions":       activeSessions,
		"total_templates":       len(m.templates),
		"collaboration_enabled": m.config.CollaborationEnabled,
	}
}
