package aifilearchive

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// ArchiveRule defines file archiving rules
type ArchiveRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Conditions  []Condition       `json:"conditions"`
	Action      ArchiveAction     `json:"action"`
	Priority    int               `json:"priority"`
	Enabled     bool              `json:"enabled"`
	Tags        []string          `json:"tags"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
	Metadata    map[string]string `json:"metadata,omitempty"`
}

// Condition represents a rule condition
type Condition struct {
	Type     ConditionType `json:"type"`
	Operator string        `json:"operator"`
	Value    interface{}   `json:"value"`
}

// ConditionType defines condition types
type ConditionType string

const (
	ConditionFileType    ConditionType = "file_type"
	ConditionFileSize    ConditionType = "file_size"
	ConditionAge         ConditionType = "age"
	ConditionAccessFreq  ConditionType = "access_frequency"
	ConditionTags        ConditionType = "tags"
	ConditionPath        ConditionType = "path"
	ConditionAIAnalysis  ConditionType = "ai_analysis"
)

// ArchiveAction defines what happens when rule matches
type ArchiveAction struct {
	Type         ActionType `json:"type"`
	TargetPath   string     `json:"target_path,omitempty"`
	Compress     bool       `json:"compress"`
	Encrypt      bool       `json:"encrypt"`
	Dedup        bool       `json:"dedup"`
	Notify       bool       `json:"notify"`
	RetentionDays int       `json:"retention_days,omitempty"`
}

// ActionType defines action types
type ActionType string

const (
	ActionArchive  ActionType = "archive"
	ActionMove     ActionType = "move"
	ActionCompress ActionType = "compress"
	ActionDelete   ActionType = "delete"
	ActionTag      ActionType = "tag"
)

// ArchiveJob represents an archiving job
type ArchiveJob struct {
	ID          string        `json:"id"`
	RuleID      string        `json:"rule_id"`
	Status      JobStatus     `json:"status"`
	FilesTotal  int           `json:"files_total"`
	FilesDone   int           `json:"files_done"`
	BytesTotal  int64         `json:"bytes_total"`
	BytesDone   int64         `json:"bytes_done"`
	Errors      []string      `json:"errors,omitempty"`
	StartedAt   *time.Time    `json:"started_at,omitempty"`
	CompletedAt *time.Time    `json:"completed_at,omitempty"`
	Duration    time.Duration `json:"duration,omitempty"`
}

// JobStatus defines job statuses
type JobStatus string

const (
	JobPending   JobStatus = "pending"
	JobRunning   JobStatus = "running"
	JobCompleted JobStatus = "completed"
	JobFailed    JobStatus = "failed"
	JobCancelled JobStatus = "cancelled"
)

// AIClassification represents AI file classification result
type AIClassification struct {
	Category    string   `json:"category"`
	Confidence  float64  `json:"confidence"`
	Tags        []string `json:"tags"`
	Suggestions []string `json:"suggestions"`
}

// Manager manages AI file archiving
type Manager struct {
	mu          sync.RWMutex
	rules       map[string]*ArchiveRule
	jobs        map[string]*ArchiveJob
	classifiers []Classifier
	stats       *ArchiveStats
}

// Classifier interface for AI classification
type Classifier interface {
	Classify(ctx context.Context, filePath string) (*AIClassification, error)
	Name() string
}

// ArchiveStats tracks archiving statistics
type ArchiveStats struct {
	TotalArchived   int64     `json:"total_archived"`
	TotalSize       int64     `json:"total_size"`
	SpaceSaved      int64     `json:"space_saved"`
	RulesApplied    int64     `json:"rules_applied"`
	LastArchiveTime time.Time `json:"last_archive_time"`
}

// NewManager creates a new archive manager
func NewManager() *Manager {
	return &Manager{
		rules: make(map[string]*ArchiveRule),
		jobs:  make(map[string]*ArchiveJob),
		stats: &ArchiveStats{},
	}
}

// AddRule adds an archiving rule
func (m *Manager) AddRule(rule *ArchiveRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if rule.ID == "" {
		return fmt.Errorf("rule ID is required")
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule
	return nil
}

// UpdateRule updates an existing rule
func (m *Manager) UpdateRule(rule *ArchiveRule) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[rule.ID]; !exists {
		return fmt.Errorf("rule %s not found", rule.ID)
	}

	rule.UpdatedAt = time.Now()
	m.rules[rule.ID] = rule
	return nil
}

// DeleteRule deletes a rule
func (m *Manager) DeleteRule(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.rules[id]; !exists {
		return fmt.Errorf("rule %s not found", id)
	}

	delete(m.rules, id)
	return nil
}

// GetRule gets a rule by ID
func (m *Manager) GetRule(id string) (*ArchiveRule, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rule, exists := m.rules[id]
	if !exists {
		return nil, fmt.Errorf("rule %s not found", id)
	}

	return rule, nil
}

// ListRules lists all rules
func (m *Manager) ListRules() []*ArchiveRule {
	m.mu.RLock()
	defer m.mu.RUnlock()

	rules := make([]*ArchiveRule, 0, len(m.rules))
	for _, rule := range m.rules {
		rules = append(rules, rule)
	}
	return rules
}

// RunArchive runs archiving job
func (m *Manager) RunArchive(ctx context.Context, ruleID string) (*ArchiveJob, error) {
	m.mu.Lock()

	rule, exists := m.rules[ruleID]
	if !exists {
		m.mu.Unlock()
		return nil, fmt.Errorf("rule %s not found", ruleID)
	}

	if !rule.Enabled {
		m.mu.Unlock()
		return nil, fmt.Errorf("rule %s is disabled", ruleID)
	}

	job := &ArchiveJob{
		ID:     fmt.Sprintf("job-%d", time.Now().UnixNano()),
		RuleID: ruleID,
		Status: JobPending,
	}
	m.jobs[job.ID] = job
	m.mu.Unlock()

	// Run job asynchronously
	go m.executeJob(ctx, job, rule)

	return job, nil
}

// executeJob executes an archive job
func (m *Manager) executeJob(ctx context.Context, job *ArchiveJob, rule *ArchiveRule) {
	m.mu.Lock()
	job.Status = JobRunning
	now := time.Now()
	job.StartedAt = &now
	m.mu.Unlock()

	// Simulate archiving work
	defer func() {
		m.mu.Lock()
		completed := time.Now()
		job.CompletedAt = &completed
		job.Status = JobCompleted
		job.Duration = completed.Sub(*job.StartedAt)
		m.stats.TotalArchived += int64(job.FilesDone)
		m.stats.LastArchiveTime = completed
		m.mu.Unlock()
	}()

	// Process files based on rule conditions
	select {
	case <-ctx.Done():
		job.Status = JobCancelled
		return
	default:
		// Archive processing logic would go here
		job.FilesTotal = 100 // Placeholder
		job.FilesDone = 100
		job.BytesTotal = 1024 * 1024 * 100
		job.BytesDone = job.BytesTotal
	}
}

// GetJob gets a job by ID
func (m *Manager) GetJob(id string) (*ArchiveJob, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	job, exists := m.jobs[id]
	if !exists {
		return nil, fmt.Errorf("job %s not found", id)
	}

	return job, nil
}

// ListJobs lists all jobs
func (m *Manager) ListJobs() []*ArchiveJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*ArchiveJob, 0, len(m.jobs))
	for _, job := range m.jobs {
		jobs = append(jobs, job)
	}
	return jobs
}

// GetStats gets archive statistics
func (m *Manager) GetStats() *ArchiveStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats
}

// RegisterClassifier registers an AI classifier
func (m *Manager) RegisterClassifier(c Classifier) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.classifiers = append(m.classifiers, c)
}

// ClassifyFile classifies a file using AI
func (m *Manager) ClassifyFile(ctx context.Context, filePath string) ([]*AIClassification, error) {
	m.mu.RLock()
	classifiers := make([]Classifier, len(m.classifiers))
	copy(classifiers, m.classifiers)
	m.mu.RUnlock()

	results := make([]*AIClassification, 0, len(classifiers))
	for _, c := range classifiers {
		result, err := c.Classify(ctx, filePath)
		if err != nil {
			continue
		}
		results = append(results, result)
	}

	return results, nil
}

// HandleHTTP registers HTTP handlers
func (m *Manager) HandleHTTP(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/archive/rules", m.handleRules)
	mux.HandleFunc("/api/v1/archive/jobs", m.handleJobs)
	mux.HandleFunc("/api/v1/archive/stats", m.handleStats)
	mux.HandleFunc("/api/v1/archive/classify", m.handleClassify)
}

func (m *Manager) handleRules(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		rules := m.ListRules()
		json.NewEncoder(w).Encode(rules)
	case http.MethodPost:
		var rule ArchiveRule
		if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.AddRule(&rule); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(rule)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleJobs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	jobs := m.ListJobs()
	json.NewEncoder(w).Encode(jobs)
}

func (m *Manager) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := m.GetStats()
	json.NewEncoder(w).Encode(stats)
}

func (m *Manager) handleClassify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FilePath string `json:"file_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	results, err := m.ClassifyFile(r.Context(), req.FilePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	json.NewEncoder(w).Encode(results)
}
