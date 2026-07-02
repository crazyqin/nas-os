package contentworkflow

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ContentType represents content types.
type ContentType string

const (
	TypeDocument ContentType = "document"
	TypeImage    ContentType = "image"
	TypeVideo    ContentType = "video"
	TypeAudio    ContentType = "audio"
	TypePost     ContentType = "post"
)

// WorkflowStatus represents workflow status.
type WorkflowStatus string

const (
	StatusDraft     WorkflowStatus = "draft"
	StatusReview    WorkflowStatus = "review"
	StatusApproved  WorkflowStatus = "approved"
	StatusPublished WorkflowStatus = "published"
	StatusArchived  WorkflowStatus = "archived"
)

// Content represents a content item.
type Content struct {
	ID          string            `json:"id"`
	Title       string            `json:"title"`
	Type        ContentType       `json:"type"`
	Body        string            `json:"body"`
	Tags        []string          `json:"tags"`
	Author      string            `json:"author"`
	Status      WorkflowStatus    `json:"status"`
	Version     int               `json:"version"`
	WorkflowID  string            `json:"workflow_id,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	PublishedAt *time.Time        `json:"published_at,omitempty"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Workflow represents a content workflow.
type Workflow struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	ContentType ContentType `json:"content_type"`
	Stages      []Stage     `json:"stages"`
	Enabled     bool        `json:"enabled"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

// Stage represents a workflow stage.
type Stage struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Status      WorkflowStatus `json:"status"`
	Assignees   []string       `json:"assignees"`
	Actions     []Action       `json:"actions"`
	AutoAdvance bool           `json:"auto_advance"`
}

// Action represents a stage action.
type Action struct {
	Type   ActionType             `json:"type"`
	Config map[string]interface{} `json:"config,omitempty"`
}

// ActionType represents action types.
type ActionType string

const (
	ActionNotify    ActionType = "notify"
	ActionAIReview  ActionType = "ai_review"
	ActionPublish   ActionType = "publish"
	ActionArchive   ActionType = "archive"
	ActionTransform ActionType = "transform"
)

// Approval represents an approval record.
type Approval struct {
	ID        string    `json:"id"`
	ContentID string    `json:"content_id"`
	StageID   string    `json:"stage_id"`
	Approver  string    `json:"approver"`
	Decision  string    `json:"decision"`
	Comment   string    `json:"comment"`
	CreatedAt time.Time `json:"created_at"`
}

// Manager manages content workflows.
type Manager struct {
	mu        sync.RWMutex
	contents  map[string]*Content
	workflows map[string]*Workflow
	approvals []*Approval
	templates map[string]*Content
}

// NewManager creates a new content workflow manager.
func NewManager() *Manager {
	return &Manager{
		contents:  make(map[string]*Content),
		workflows: make(map[string]*Workflow),
		templates: make(map[string]*Content),
	}
}

// CreateContent creates new content.
func (m *Manager) CreateContent(content *Content) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if content.ID == "" {
		content.ID = fmt.Sprintf("content-%d", time.Now().UnixNano())
	}

	content.Status = StatusDraft
	content.Version = 1
	content.CreatedAt = time.Now()
	content.UpdatedAt = time.Now()

	m.contents[content.ID] = content
	return nil
}

// UpdateContent updates content.
func (m *Manager) UpdateContent(content *Content) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.contents[content.ID]
	if !exists {
		return fmt.Errorf("content %s not found", content.ID)
	}

	content.Version = existing.Version + 1
	content.UpdatedAt = time.Now()
	m.contents[content.ID] = content
	return nil
}

// DeleteContent deletes content.
func (m *Manager) DeleteContent(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.contents[id]; !exists {
		return fmt.Errorf("content %s not found", id)
	}

	delete(m.contents, id)
	return nil
}

// GetContent gets content by ID.
func (m *Manager) GetContent(id string) (*Content, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	content, exists := m.contents[id]
	if !exists {
		return nil, fmt.Errorf("content %s not found", id)
	}

	return content, nil
}

// ListContents lists all contents.
func (m *Manager) ListContents(contentType ContentType, status WorkflowStatus) []*Content {
	m.mu.RLock()
	defer m.mu.RUnlock()

	contents := make([]*Content, 0)
	for _, content := range m.contents {
		if contentType != "" && content.Type != contentType {
			continue
		}
		if status != "" && content.Status != status {
			continue
		}
		contents = append(contents, content)
	}
	return contents
}

// CreateWorkflow creates a new workflow.
func (m *Manager) CreateWorkflow(workflow *Workflow) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if workflow.ID == "" {
		workflow.ID = fmt.Sprintf("workflow-%d", time.Now().UnixNano())
	}

	workflow.CreatedAt = time.Now()
	workflow.UpdatedAt = time.Now()

	m.workflows[workflow.ID] = workflow
	return nil
}

// GetWorkflow gets workflow by ID.
func (m *Manager) GetWorkflow(id string) (*Workflow, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workflow, exists := m.workflows[id]
	if !exists {
		return nil, fmt.Errorf("workflow %s not found", id)
	}

	return workflow, nil
}

// ListWorkflows lists all workflows.
func (m *Manager) ListWorkflows() []*Workflow {
	m.mu.RLock()
	defer m.mu.RUnlock()

	workflows := make([]*Workflow, 0, len(m.workflows))
	for _, w := range m.workflows {
		workflows = append(workflows, w)
	}
	return workflows
}

// SubmitForReview submits content for review.
func (m *Manager) SubmitForReview(contentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	content, exists := m.contents[contentID]
	if !exists {
		return fmt.Errorf("content %s not found", contentID)
	}

	if content.Status != StatusDraft {
		return fmt.Errorf("content must be in draft status to submit for review")
	}

	content.Status = StatusReview
	content.UpdatedAt = time.Now()
	return nil
}

// ApproveContent approves content.
func (m *Manager) ApproveContent(contentID, approver, comment string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	content, exists := m.contents[contentID]
	if !exists {
		return fmt.Errorf("content %s not found", contentID)
	}

	if content.Status != StatusReview {
		return fmt.Errorf("content must be in review status to approve")
	}

	approval := &Approval{
		ID:        fmt.Sprintf("approval-%d", time.Now().UnixNano()),
		ContentID: contentID,
		Approver:  approver,
		Decision:  "approved",
		Comment:   comment,
		CreatedAt: time.Now(),
	}

	m.approvals = append(m.approvals, approval)
	content.Status = StatusApproved
	content.UpdatedAt = time.Now()

	return nil
}

// PublishContent publishes content.
func (m *Manager) PublishContent(contentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	content, exists := m.contents[contentID]
	if !exists {
		return fmt.Errorf("content %s not found", contentID)
	}

	if content.Status != StatusApproved {
		return fmt.Errorf("content must be approved before publishing")
	}

	now := time.Now()
	content.Status = StatusPublished
	content.PublishedAt = &now
	content.UpdatedAt = now

	return nil
}

// ArchiveContent archives content.
func (m *Manager) ArchiveContent(contentID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	content, exists := m.contents[contentID]
	if !exists {
		return fmt.Errorf("content %s not found", contentID)
	}

	content.Status = StatusArchived
	content.UpdatedAt = time.Now()
	return nil
}

// SaveTemplate saves content as template.
func (m *Manager) SaveTemplate(contentID, templateName string) error {
	m.mu.RLock()
	content, exists := m.contents[contentID]
	m.mu.RUnlock()

	if !exists {
		return fmt.Errorf("content %s not found", contentID)
	}

	template := &Content{
		Title: templateName,
		Type:  content.Type,
		Body:  content.Body,
		Tags:  content.Tags,
	}

	m.mu.Lock()
	m.templates[templateName] = template
	m.mu.Unlock()

	return nil
}

// CreateFromTemplate creates content from template.
func (m *Manager) CreateFromTemplate(templateName, title string) (*Content, error) {
	m.mu.RLock()
	template, exists := m.templates[templateName]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("template %s not found", templateName)
	}

	content := &Content{
		Title: title,
		Type:  template.Type,
		Body:  template.Body,
		Tags:  template.Tags,
	}

	if err := m.CreateContent(content); err != nil {
		return nil, err
	}

	return content, nil
}

// AIReviewContent uses AI to review content.
func (m *Manager) AIReviewContent(ctx context.Context, contentID string) (map[string]interface{}, error) {
	m.mu.RLock()
	content, exists := m.contents[contentID]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("content %s not found", contentID)
	}

	// AI review would go here
	result := map[string]interface{}{
		"content_id": contentID,
		"score":      85,
		"suggestions": []string{
			"Consider adding more details",
			"Check for grammar errors",
		},
		"readability": "good",
		"seo_score":   90,
		"_note":       "AI review for: " + content.Title,
	}

	return result, nil
}

// GetApprovalHistory gets approval history for content.
func (m *Manager) GetApprovalHistory(contentID string) []*Approval {
	m.mu.RLock()
	defer m.mu.RUnlock()

	approvals := make([]*Approval, 0)
	for _, a := range m.approvals {
		if a.ContentID == contentID {
			approvals = append(approvals, a)
		}
	}
	return approvals
}

// HandleHTTP registers HTTP handlers.
func (m *Manager) HandleHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/content/items", m.handleContents)
	mux.HandleFunc("/api/v1/content/item/", m.handleContent)
	mux.HandleFunc("/api/v1/content/workflows", m.handleWorkflows)
	mux.HandleFunc("/api/v1/content/submit", m.handleSubmit)
	mux.HandleFunc("/api/v1/content/approve", m.handleApprove)
	mux.HandleFunc("/api/v1/content/publish", m.handlePublish)
	mux.HandleFunc("/api/v1/content/ai-review", m.handleAIReview)
}

func (m *Manager) handleContents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		contents := m.ListContents("", "")
		json.NewEncoder(w).Encode(contents)
	case http.MethodPost:
		var content Content
		if err := json.NewDecoder(r.Body).Decode(&content); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateContent(&content); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(content)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleContent(w http.ResponseWriter, r *http.Request) {
	// Extract ID from path
	id := r.URL.Path[len("/api/v1/content/item/"):]
	if id == "" {
		http.Error(w, "ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		content, err := m.GetContent(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(content)
	case http.MethodPut:
		var content Content
		if err := json.NewDecoder(r.Body).Decode(&content); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		content.ID = id
		if err := m.UpdateContent(&content); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(content)
	case http.MethodDelete:
		if err := m.DeleteContent(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleWorkflows(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	workflows := m.ListWorkflows()
	json.NewEncoder(w).Encode(workflows)
}

func (m *Manager) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ContentID string `json:"content_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.SubmitForReview(req.ContentID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "submitted"})
}

func (m *Manager) handleApprove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ContentID string `json:"content_id"`
		Approver  string `json:"approver"`
		Comment   string `json:"comment"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.ApproveContent(req.ContentID, req.Approver, req.Comment); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "approved"})
}

func (m *Manager) handlePublish(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ContentID string `json:"content_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	if err := m.PublishContent(req.ContentID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(map[string]string{"status": "published"})
}

func (m *Manager) handleAIReview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ContentID string `json:"content_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	result, err := m.AIReviewContent(r.Context(), req.ContentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(result)
}
