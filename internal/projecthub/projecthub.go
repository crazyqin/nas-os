package projecthub

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusTodo       TaskStatus = "todo"
	TaskStatusInProgress TaskStatus = "in_progress"
	TaskStatusReview     TaskStatus = "review"
	TaskStatusDone       TaskStatus = "done"
	TaskStatusBlocked    TaskStatus = "blocked"
)

// TaskPriority 任务优先级
type TaskPriority string

const (
	TaskPriorityLow      TaskPriority = "low"
	TaskPriorityMedium   TaskPriority = "medium"
	TaskPriorityHigh     TaskPriority = "high"
	TaskPriorityCritical TaskPriority = "critical"
)

// Project 项目
type Project struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Status      string    `json:"status"` // active, archived, completed
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
	Tasks       []string  `json:"tasks"`       // task IDs
	Milestones  []string  `json:"milestones"`  // milestone IDs
	Members     []string  `json:"members"`     // member IDs
}

// Task 任务
type Task struct {
	ID          string       `json:"id"`
	ProjectID   string       `json:"project_id"`
	Title       string       `json:"title"`
	Description string       `json:"description"`
	Priority    TaskPriority `json:"priority"`
	Status      TaskStatus   `json:"status"`
	AssigneeID  string       `json:"assignee_id"`  // 负责人
	Deadline    *time.Time   `json:"deadline"`      // 截止日期
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
	CompletedAt *time.Time   `json:"completed_at"`
}

// Milestone 里程碑
type Milestone struct {
	ID          string    `json:"id"`
	ProjectID   string    `json:"project_id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	DueDate     *time.Time `json:"due_date"`
	Completed   bool      `json:"completed"`
	CreatedAt   time.Time `json:"created_at"`
}

// TimeEntry 工时记录
type TimeEntry struct {
	ID          string    `json:"id"`
	TaskID      string    `json:"task_id"`
	MemberID    string    `json:"member_id"`
	Hours       float64   `json:"hours"`
	Description string    `json:"description"`
	Date        time.Time `json:"date"`
	CreatedAt   time.Time `json:"created_at"`
}

// TeamMember 团队成员
type TeamMember struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Email     string    `json:"email"`
	Role      string    `json:"role"` // developer, designer, pm, etc.
	CreatedAt time.Time `json:"created_at"`
}

// ProjectStats 项目统计
type ProjectStats struct {
	TotalTasks     int     `json:"total_tasks"`
	CompletedTasks int     `json:"completed_tasks"`
	OverdueTasks   int     `json:"overdue_tasks"`
	TotalHours     float64 `json:"total_hours"`
	Progress       float64 `json:"progress"` // 0-100
}

// MemberWorkload 成员工作量
type MemberWorkload struct {
	MemberID      string  `json:"member_id"`
	ActiveTasks   int     `json:"active_tasks"`
	TotalHours    float64 `json:"total_hours"`
	OverdueTasks  int     `json:"overdue_tasks"`
}

// Config 配置
type Config struct {
	DataPath string `json:"data_path"` // 数据存储路径
}

// ProjectHub 项目管理中心
type ProjectHub struct {
	mu         sync.RWMutex
	config     Config
	projects   map[string]*Project
	tasks      map[string]*Task
	milestones map[string]*Milestone
	timeEntries map[string]*TimeEntry
	members    map[string]*TeamMember
	started    bool
	stopCh     chan struct{}
}

// New 创建项目管理中心
func New(config Config) *ProjectHub {
	return &ProjectHub{
		config:      config,
		projects:    make(map[string]*Project),
		tasks:       make(map[string]*Task),
		milestones:  make(map[string]*Milestone),
		timeEntries: make(map[string]*TimeEntry),
		members:     make(map[string]*TeamMember),
		stopCh:      make(chan struct{}),
	}
}

// Start 启动项目管理中心
func (ph *ProjectHub) Start() error {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	if ph.started {
		return fmt.Errorf("projecthub already started")
	}

	ph.started = true
	log.Println("项目管理中心已启动")
	return nil
}

// Stop 停止项目管理中心
func (ph *ProjectHub) Stop() error {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	if !ph.started {
		return fmt.Errorf("projecthub not started")
	}

	close(ph.stopCh)
	ph.started = false
	log.Println("项目管理中心已停止")
	return nil
}

// CreateProject 创建项目
func (ph *ProjectHub) CreateProject(project Project) (*Project, error) {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	if project.ID == "" {
		return nil, fmt.Errorf("project ID is required")
	}
	if project.Name == "" {
		return nil, fmt.Errorf("project name is required")
	}

	if _, exists := ph.projects[project.ID]; exists {
		return nil, fmt.Errorf("project already exists: %s", project.ID)
	}

	now := time.Now()
	project.CreatedAt = now
	project.UpdatedAt = now
	if project.Status == "" {
		project.Status = "active"
	}
	if project.Tasks == nil {
		project.Tasks = []string{}
	}
	if project.Milestones == nil {
		project.Milestones = []string{}
	}
	if project.Members == nil {
		project.Members = []string{}
	}

	ph.projects[project.ID] = &project
	log.Printf("项目已创建: %s (%s)", project.Name, project.ID)
	return &project, nil
}

// GetProject 获取项目
func (ph *ProjectHub) GetProject(id string) (*Project, error) {
	ph.mu.RLock()
	defer ph.mu.RUnlock()

	project, exists := ph.projects[id]
	if !exists {
		return nil, fmt.Errorf("project not found: %s", id)
	}
	return project, nil
}

// ListProjects 列出项目
func (ph *ProjectHub) ListProjects() []Project {
	ph.mu.RLock()
	defer ph.mu.RUnlock()

	projects := make([]Project, 0, len(ph.projects))
	for _, p := range ph.projects {
		projects = append(projects, *p)
	}
	return projects
}

// CreateTask 创建任务
func (ph *ProjectHub) CreateTask(projectID string, task Task) (*Task, error) {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	if task.ID == "" {
		return nil, fmt.Errorf("task ID is required")
	}
	if task.Title == "" {
		return nil, fmt.Errorf("task title is required")
	}

	project, exists := ph.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	if _, exists := ph.tasks[task.ID]; exists {
		return nil, fmt.Errorf("task already exists: %s", task.ID)
	}

	now := time.Now()
	task.ProjectID = projectID
	task.CreatedAt = now
	task.UpdatedAt = now
	if task.Status == "" {
		task.Status = TaskStatusTodo
	}
	if task.Priority == "" {
		task.Priority = TaskPriorityMedium
	}

	ph.tasks[task.ID] = &task
	project.Tasks = append(project.Tasks, task.ID)
	project.UpdatedAt = now

	log.Printf("任务已创建: %s (项目: %s)", task.Title, projectID)
	return &task, nil
}

// UpdateTask 更新任务
func (ph *ProjectHub) UpdateTask(taskID string, updates map[string]interface{}) (*Task, error) {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	task, exists := ph.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	if title, ok := updates["title"].(string); ok {
		task.Title = title
	}
	if desc, ok := updates["description"].(string); ok {
		task.Description = desc
	}
	if priority, ok := updates["priority"].(TaskPriority); ok {
		task.Priority = priority
	}
	if status, ok := updates["status"].(TaskStatus); ok {
		task.Status = status
		if status == TaskStatusDone {
			now := time.Now()
			task.CompletedAt = &now
		}
	}
	if deadline, ok := updates["deadline"].(*time.Time); ok {
		task.Deadline = deadline
	}

	task.UpdatedAt = time.Now()
	log.Printf("任务已更新: %s", taskID)
	return task, nil
}

// AssignTask 分配任务
func (ph *ProjectHub) AssignTask(taskID, memberID string) error {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	task, exists := ph.tasks[taskID]
	if !exists {
		return fmt.Errorf("task not found: %s", taskID)
	}

	if _, exists := ph.members[memberID]; !exists {
		return fmt.Errorf("member not found: %s", memberID)
	}

	task.AssigneeID = memberID
	task.UpdatedAt = time.Now()
	log.Printf("任务 %s 已分配给 %s", taskID, memberID)
	return nil
}

// AddMilestone 添加里程碑
func (ph *ProjectHub) AddMilestone(projectID string, milestone Milestone) (*Milestone, error) {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	if milestone.ID == "" {
		return nil, fmt.Errorf("milestone ID is required")
	}
	if milestone.Name == "" {
		return nil, fmt.Errorf("milestone name is required")
	}

	project, exists := ph.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	if _, exists := ph.milestones[milestone.ID]; exists {
		return nil, fmt.Errorf("milestone already exists: %s", milestone.ID)
	}

	milestone.ProjectID = projectID
	milestone.CreatedAt = time.Now()

	ph.milestones[milestone.ID] = &milestone
	project.Milestones = append(project.Milestones, milestone.ID)
	project.UpdatedAt = time.Now()

	log.Printf("里程碑已添加: %s (项目: %s)", milestone.Name, projectID)
	return &milestone, nil
}

// LogTime 记录工时
func (ph *ProjectHub) LogTime(taskID string, entry TimeEntry) (*TimeEntry, error) {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	if entry.ID == "" {
		return nil, fmt.Errorf("time entry ID is required")
	}

	task, exists := ph.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("task not found: %s", taskID)
	}

	if entry.MemberID == "" {
		return nil, fmt.Errorf("member ID is required")
	}
	if entry.Hours <= 0 {
		return nil, fmt.Errorf("hours must be positive")
	}

	entry.TaskID = taskID
	entry.CreatedAt = time.Now()
	if entry.Date.IsZero() {
		entry.Date = time.Now()
	}

	ph.timeEntries[entry.ID] = &entry
	task.UpdatedAt = time.Now()

	log.Printf("工时已记录: %.1f 小时 (任务: %s)", entry.Hours, taskID)
	return &entry, nil
}

// GetProjectStats 获取项目统计
func (ph *ProjectHub) GetProjectStats(projectID string) (*ProjectStats, error) {
	ph.mu.RLock()
	defer ph.mu.RUnlock()

	project, exists := ph.projects[projectID]
	if !exists {
		return nil, fmt.Errorf("project not found: %s", projectID)
	}

	stats := &ProjectStats{}
	now := time.Now()

	for _, taskID := range project.Tasks {
		task, exists := ph.tasks[taskID]
		if !exists {
			continue
		}

		stats.TotalTasks++
		if task.Status == TaskStatusDone {
			stats.CompletedTasks++
		}
		if task.Deadline != nil && task.Deadline.Before(now) && task.Status != TaskStatusDone {
			stats.OverdueTasks++
		}
	}

	// 计算总工时
	for _, entry := range ph.timeEntries {
		task, exists := ph.tasks[entry.TaskID]
		if exists && task.ProjectID == projectID {
			stats.TotalHours += entry.Hours
		}
	}

	// 计算进度
	if stats.TotalTasks > 0 {
		stats.Progress = float64(stats.CompletedTasks) / float64(stats.TotalTasks) * 100
	}

	return stats, nil
}

// GetMemberWorkload 获取成员工作量
func (ph *ProjectHub) GetMemberWorkload(memberID string) (*MemberWorkload, error) {
	ph.mu.RLock()
	defer ph.mu.RUnlock()

	if _, exists := ph.members[memberID]; !exists {
		return nil, fmt.Errorf("member not found: %s", memberID)
	}

	workload := &MemberWorkload{
		MemberID: memberID,
	}
	now := time.Now()

	// 统计活跃任务
	for _, task := range ph.tasks {
		if task.AssigneeID != memberID {
			continue
		}
		if task.Status != TaskStatusDone {
			workload.ActiveTasks++
		}
		if task.Deadline != nil && task.Deadline.Before(now) && task.Status != TaskStatusDone {
			workload.OverdueTasks++
		}
	}

	// 统计总工时
	for _, entry := range ph.timeEntries {
		if entry.MemberID == memberID {
			workload.TotalHours += entry.Hours
		}
	}

	return workload, nil
}

// GetUpcomingDeadlines 获取即将到期的任务
func (ph *ProjectHub) GetUpcomingDeadlines(days int) []Task {
	ph.mu.RLock()
	defer ph.mu.RUnlock()

	now := time.Now()
	deadline := now.AddDate(0, 0, days)

	var upcoming []Task
	for _, task := range ph.tasks {
		if task.Deadline == nil {
			continue
		}
		if task.Deadline.After(now) && task.Deadline.Before(deadline) && task.Status != TaskStatusDone {
			upcoming = append(upcoming, *task)
		}
	}
	return upcoming
}

// AddTeamMember 添加团队成员
func (ph *ProjectHub) AddTeamMember(member TeamMember) (*TeamMember, error) {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	if member.ID == "" {
		return nil, fmt.Errorf("member ID is required")
	}
	if member.Name == "" {
		return nil, fmt.Errorf("member name is required")
	}

	if _, exists := ph.members[member.ID]; exists {
		return nil, fmt.Errorf("member already exists: %s", member.ID)
	}

	member.CreatedAt = time.Now()
	ph.members[member.ID] = &member
	log.Printf("团队成员已添加: %s (%s)", member.Name, member.ID)
	return &member, nil
}

// GetTeamMember 获取团队成员
func (ph *ProjectHub) GetTeamMember(id string) (*TeamMember, error) {
	ph.mu.RLock()
	defer ph.mu.RUnlock()

	member, exists := ph.members[id]
	if !exists {
		return nil, fmt.Errorf("member not found: %s", id)
	}
	return member, nil
}

// AddProjectMember 将成员添加到项目
func (ph *ProjectHub) AddProjectMember(projectID, memberID string) error {
	ph.mu.Lock()
	defer ph.mu.Unlock()

	project, exists := ph.projects[projectID]
	if !exists {
		return fmt.Errorf("project not found: %s", projectID)
	}

	if _, exists := ph.members[memberID]; !exists {
		return fmt.Errorf("member not found: %s", memberID)
	}

	// 检查是否已在项目中
	for _, id := range project.Members {
		if id == memberID {
			return fmt.Errorf("member already in project: %s", memberID)
		}
	}

	project.Members = append(project.Members, memberID)
	project.UpdatedAt = time.Now()
	log.Printf("成员 %s 已添加到项目 %s", memberID, projectID)
	return nil
}
