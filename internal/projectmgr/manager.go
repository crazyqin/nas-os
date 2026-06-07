// Package projectmgr provides project management functionality.
package projectmgr

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

// Manager manages projects.
type Manager struct {
	mu         sync.RWMutex
	projects   map[string]*Project
	configPath string
}

// NewManager creates a new project manager.
func NewManager(configPath string) *Manager {
	m := &Manager{
		projects:   make(map[string]*Project),
		configPath: configPath,
	}

	// Load existing config
	if err := m.loadConfig(); err != nil && !os.IsNotExist(err) {
		log.Printf("Failed to load project config: %v", err)
	}

	return m
}

// CreateProject creates a new project.
func (m *Manager) CreateProject(req CreateProjectRequest, ownerID string) (*Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	projectID := uuid.New().String()
	now := time.Now()

	// Set default priority
	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}

	project := &Project{
		ID:          projectID,
		Name:        req.Name,
		Description: req.Description,
		Status:      "planning",
		Priority:    priority,
		OwnerID:     ownerID,
		StartDate:   req.StartDate,
		EndDate:     req.EndDate,
		Budget:      req.Budget,
		Spent:       0,
		Tags:        req.Tags,
		Members: []*Member{
			{
				UserID:   ownerID,
				Role:     "owner",
				JoinedAt: now,
			},
		},
		Milestones: make([]*Milestone, 0),
		Tasks:      make([]*Task, 0),
		TasksTotal: 0,
		TasksDone:  0,
		Progress:   0,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	m.projects[projectID] = project

	log.Printf("Created project: %s (%s)", project.Name, projectID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save project config: %v", err)
	}

	return project, nil
}

// GetProject returns a project by ID.
func (m *Manager) GetProject(projectID string) (*Project, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project %s not found", projectID)
	}

	return project, nil
}

// ListProjects returns all projects.
func (m *Manager) ListProjects() []*Project {
	m.mu.RLock()
	defer m.mu.RUnlock()

	projects := make([]*Project, 0, len(m.projects))
	for _, project := range m.projects {
		projects = append(projects, project)
	}

	// Sort by creation time
	sort.Slice(projects, func(i, j int) bool {
		return projects[i].CreatedAt.After(projects[j].CreatedAt)
	})

	return projects
}

// UpdateProject updates a project.
func (m *Manager) UpdateProject(projectID string, req UpdateProjectRequest) (*Project, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project %s not found", projectID)
	}

	if req.Name != nil {
		project.Name = *req.Name
	}
	if req.Description != nil {
		project.Description = *req.Description
	}
	if req.Status != nil {
		project.Status = *req.Status
	}
	if req.Priority != nil {
		project.Priority = *req.Priority
	}
	if req.StartDate != nil {
		project.StartDate = req.StartDate
	}
	if req.EndDate != nil {
		project.EndDate = req.EndDate
	}
	if req.Budget != nil {
		project.Budget = *req.Budget
	}
	if req.Tags != nil {
		project.Tags = req.Tags
	}

	project.UpdatedAt = time.Now()

	log.Printf("Updated project: %s", projectID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save project config: %v", err)
	}

	return project, nil
}

// DeleteProject deletes a project.
func (m *Manager) DeleteProject(projectID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.projects[projectID]; !exists {
		return fmt.Errorf("project %s not found", projectID)
	}

	delete(m.projects, projectID)

	log.Printf("Deleted project: %s", projectID)

	return m.saveConfig()
}

// AddMember adds a member to a project.
func (m *Manager) AddMember(projectID, userID, username, role string, hourlyRate float64) (*Member, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project %s not found", projectID)
	}

	// Check if already a member
	for _, member := range project.Members {
		if member.UserID == userID {
			return nil, fmt.Errorf("user %s is already a member", userID)
		}
	}

	member := &Member{
		UserID:     userID,
		Username:   username,
		Role:       role,
		HourlyRate: hourlyRate,
		JoinedAt:   time.Now(),
	}

	project.Members = append(project.Members, member)
	project.UpdatedAt = time.Now()

	log.Printf("Added member %s to project %s with role %s", userID, projectID, role)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save project config: %v", err)
	}

	return member, nil
}

// CreateMilestone creates a new milestone in a project.
func (m *Manager) CreateMilestone(projectID string, req CreateMilestoneRequest) (*Milestone, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project %s not found", projectID)
	}

	milestone := &Milestone{
		ID:          uuid.New().String(),
		ProjectID:   projectID,
		Name:        req.Name,
		Description: req.Description,
		DueDate:     req.DueDate,
		Status:      "pending",
		Progress:    0,
		Tasks:       make([]string, 0),
		CreatedAt:   time.Now(),
	}

	project.Milestones = append(project.Milestones, milestone)
	project.UpdatedAt = time.Now()

	log.Printf("Created milestone %s in project %s", milestone.Name, projectID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save project config: %v", err)
	}

	return milestone, nil
}

// CreateTask creates a new task in a project.
func (m *Manager) CreateTask(projectID string, req CreateTaskRequest, createdBy, createdByName string) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project %s not found", projectID)
	}

	// Validate milestone if specified
	if req.MilestoneID != "" {
		found := false
		for _, ms := range project.Milestones {
			if ms.ID == req.MilestoneID {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("milestone %s not found in project", req.MilestoneID)
		}
	}

	// Validate parent task if specified
	if req.ParentID != "" {
		found := false
		for _, task := range project.Tasks {
			if task.ID == req.ParentID {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("parent task %s not found in project", req.ParentID)
		}
	}

	// Set default priority
	priority := req.Priority
	if priority == "" {
		priority = "medium"
	}

	// Get assignee name
	assigneeName := ""
	if req.AssigneeID != "" {
		for _, member := range project.Members {
			if member.UserID == req.AssigneeID {
				assigneeName = member.Username
				break
			}
		}
	}

	now := time.Now()
	task := &Task{
		ID:             uuid.New().String(),
		ProjectID:      projectID,
		MilestoneID:    req.MilestoneID,
		ParentID:       req.ParentID,
		Title:          req.Title,
		Description:    req.Description,
		Status:         "todo",
		Priority:       priority,
		AssigneeID:     req.AssigneeID,
		AssigneeName:   assigneeName,
		Tags:           req.Tags,
		Dependencies:   req.Dependencies,
		StartDate:      req.StartDate,
		DueDate:        req.DueDate,
		EstimatedHours: req.EstimatedHours,
		ActualHours:    0,
		SubTasks:       make([]*Task, 0),
		Timesheets:     make([]*Timesheet, 0),
		Comments:       make([]*Comment, 0),
		CreatedBy:      createdBy,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	project.Tasks = append(project.Tasks, task)
	project.TasksTotal++
	project.UpdatedAt = now

	// Update milestone task list
	if req.MilestoneID != "" {
		for _, ms := range project.Milestones {
			if ms.ID == req.MilestoneID {
				ms.Tasks = append(ms.Tasks, task.ID)
				break
			}
		}
	}

	log.Printf("Created task %s in project %s", task.Title, projectID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save project config: %v", err)
	}

	return task, nil
}

// GetTasks returns all tasks in a project.
func (m *Manager) GetTasks(projectID string) ([]*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project %s not found", projectID)
	}

	return project.Tasks, nil
}

// UpdateTask updates a task.
func (m *Manager) UpdateTask(projectID, taskID string, req UpdateTaskRequest) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project %s not found", projectID)
	}

	// Find task
	var task *Task
	for _, t := range project.Tasks {
		if t.ID == taskID {
			task = t
			break
		}
	}

	if task == nil {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	if req.Title != nil {
		task.Title = *req.Title
	}
	if req.Description != nil {
		task.Description = *req.Description
	}
	if req.Status != nil {
		oldStatus := task.Status
		task.Status = *req.Status

		// Update project stats
		if oldStatus != "done" && *req.Status == "done" {
			project.TasksDone++
			now := time.Now()
			task.CompletedAt = &now
		} else if oldStatus == "done" && *req.Status != "done" {
			project.TasksDone--
			task.CompletedAt = nil
		}

		// Update progress
		if project.TasksTotal > 0 {
			project.Progress = float64(project.TasksDone) / float64(project.TasksTotal) * 100
		}
	}
	if req.Priority != nil {
		task.Priority = *req.Priority
	}
	if req.AssigneeID != nil {
		task.AssigneeID = *req.AssigneeID
		// Update assignee name
		for _, member := range project.Members {
			if member.UserID == *req.AssigneeID {
				task.AssigneeName = member.Username
				break
			}
		}
	}
	if req.Tags != nil {
		task.Tags = req.Tags
	}
	if req.Dependencies != nil {
		task.Dependencies = req.Dependencies
	}
	if req.StartDate != nil {
		task.StartDate = req.StartDate
	}
	if req.DueDate != nil {
		task.DueDate = req.DueDate
	}
	if req.EstimatedHours != nil {
		task.EstimatedHours = *req.EstimatedHours
	}

	task.UpdatedAt = time.Now()
	project.UpdatedAt = time.Now()

	log.Printf("Updated task %s in project %s", taskID, projectID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save project config: %v", err)
	}

	return task, nil
}

// LogTime logs time for a task.
func (m *Manager) LogTime(projectID string, req LogTimeRequest, userID, username string) (*Timesheet, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project %s not found", projectID)
	}

	// Find task
	var task *Task
	for _, t := range project.Tasks {
		if t.ID == req.TaskID {
			task = t
			break
		}
	}

	if task == nil {
		return nil, fmt.Errorf("task %s not found", req.TaskID)
	}

	timesheet := &Timesheet{
		ID:          uuid.New().String(),
		TaskID:      req.TaskID,
		UserID:      userID,
		Username:    username,
		Date:        req.Date,
		Hours:       req.Hours,
		Description: req.Description,
		CreatedAt:   time.Now(),
	}

	task.Timesheets = append(task.Timesheets, timesheet)
	task.ActualHours += req.Hours
	task.UpdatedAt = time.Now()

	// Update project spent based on member hourly rate
	for _, member := range project.Members {
		if member.UserID == userID {
			project.Spent += req.Hours * member.HourlyRate
			break
		}
	}

	project.UpdatedAt = time.Now()

	log.Printf("Logged %.1f hours for task %s by user %s", req.Hours, req.TaskID, userID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save project config: %v", err)
	}

	return timesheet, nil
}

// AddComment adds a comment to a task.
func (m *Manager) AddComment(projectID, taskID, userID, username, content string) (*Comment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project %s not found", projectID)
	}

	// Find task
	var task *Task
	for _, t := range project.Tasks {
		if t.ID == taskID {
			task = t
			break
		}
	}

	if task == nil {
		return nil, fmt.Errorf("task %s not found", taskID)
	}

	comment := &Comment{
		ID:        uuid.New().String(),
		TaskID:    taskID,
		UserID:    userID,
		Username:  username,
		Content:   content,
		CreatedAt: time.Now(),
	}

	task.Comments = append(task.Comments, comment)
	task.UpdatedAt = time.Now()
	project.UpdatedAt = time.Now()

	log.Printf("Added comment to task %s", taskID)

	if err := m.saveConfig(); err != nil {
		log.Printf("Failed to save project config: %v", err)
	}

	return comment, nil
}

// GetGanttData returns gantt chart data for a project.
func (m *Manager) GetGanttData(projectID string) ([]*GanttTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project %s not found", projectID)
	}

	ganttTasks := make([]*GanttTask, 0)

	// Build task map for level calculation
	taskMap := make(map[string]*Task)
	for _, task := range project.Tasks {
		taskMap[task.ID] = task
	}

	// Calculate level for each task
	var calculateLevel func(taskID string) int
	calculateLevel = func(taskID string) int {
		task, exists := taskMap[taskID]
		if !exists || task.ParentID == "" {
			return 0
		}
		return 1 + calculateLevel(task.ParentID)
	}

	for _, task := range project.Tasks {
		progress := 0.0
		if task.Status == "done" {
			progress = 100
		} else if task.Status == "in_progress" {
			progress = 50
		} else if task.Status == "review" {
			progress = 80
		}

		ganttTask := &GanttTask{
			ID:           task.ID,
			Title:        task.Title,
			StartDate:    task.StartDate,
			EndDate:      task.DueDate,
			Progress:     progress,
			Dependencies: task.Dependencies,
			Assignee:     task.AssigneeName,
			Status:       task.Status,
			ParentID:     task.ParentID,
			Level:        calculateLevel(task.ID),
		}

		ganttTasks = append(ganttTasks, ganttTask)
	}

	// Sort by start date
	sort.Slice(ganttTasks, func(i, j int) bool {
		if ganttTasks[i].StartDate == nil {
			return false
		}
		if ganttTasks[j].StartDate == nil {
			return true
		}
		return ganttTasks[i].StartDate.Before(*ganttTasks[j].StartDate)
	})

	return ganttTasks, nil
}

// GetProjectReport generates a project report.
func (m *Manager) GetProjectReport(projectID string) (*ProjectReport, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	project, exists := m.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project %s not found", projectID)
	}

	report := &ProjectReport{
		ProjectID:       projectID,
		ProjectName:     project.Name,
		TotalTasks:      project.TasksTotal,
		Progress:        project.Progress,
		BudgetUsed:      project.Spent,
		BudgetRemaining: project.Budget - project.Spent,
		GeneratedAt:     time.Now(),
	}

	// Calculate task stats
	now := time.Now()
	for _, task := range project.Tasks {
		if task.Status == "done" {
			report.CompletedTasks++
		}
		if task.DueDate != nil && task.DueDate.Before(now) && task.Status != "done" {
			report.OverdueTasks++
		}
		report.TotalHoursLogged += task.ActualHours
	}

	// Calculate member stats
	memberStats := make(map[string]*MemberStat)
	for _, member := range project.Members {
		memberStats[member.UserID] = &MemberStat{
			UserID:   member.UserID,
			Username: member.Username,
		}
	}

	for _, task := range project.Tasks {
		if stat, exists := memberStats[task.AssigneeID]; exists {
			stat.TasksAssigned++
			if task.Status == "done" {
				stat.TasksCompleted++
			}
			stat.HoursLogged += task.ActualHours
		}
	}

	report.MemberStats = make([]*MemberStat, 0)
	for _, stat := range memberStats {
		report.MemberStats = append(report.MemberStats, stat)
	}

	// Calculate timeline stats (simplified - by day for last 7 days)
	report.TimelineStats = make([]*TimelineStat, 0)
	for i := 6; i >= 0; i-- {
		date := now.AddDate(0, 0, -i)
		dateStr := date.Format("2006-01-02")

		stat := &TimelineStat{
			Date: dateStr,
		}

		for _, task := range project.Tasks {
			// Check tasks created on this day
			if task.CreatedAt.Format("2006-01-02") == dateStr {
				stat.TasksCreated++
			}
			// Check tasks completed on this day
			if task.CompletedAt != nil && task.CompletedAt.Format("2006-01-02") == dateStr {
				stat.TasksDone++
			}
			// Check timesheets on this day
			for _, ts := range task.Timesheets {
				if ts.Date.Format("2006-01-02") == dateStr {
					stat.HoursLogged += ts.Hours
				}
			}
		}

		report.TimelineStats = append(report.TimelineStats, stat)
	}

	return report, nil
}

// saveConfig saves configuration to disk.
func (m *Manager) saveConfig() error {
	cfg := struct {
		Projects map[string]*Project `json:"projects"`
	}{
		Projects: m.projects,
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
		Projects map[string]*Project `json:"projects"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	if cfg.Projects != nil {
		m.projects = cfg.Projects
	}

	return nil
}
