// Package fleet provides multi-node fleet management and task scheduling.
package fleet

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Node represents a node in the fleet.
type Node struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Address      string    `json:"address"`
	Port         int       `json:"port"`
	Status       string    `json:"status"`        // "online", "offline", "maintenance"
	Priority     int       `json:"priority"`      // 0-100, higher = more priority
	Capacity     int64     `json:"capacity"`      // Available capacity
	Load         float64   `json:"load"`          // Current load 0-1
	Tags         []string  `json:"tags"`
	LastSeen     time.Time `json:"last_seen"`
	CreatedAt    time.Time `json:"created_at"`
}

// Task represents a scheduled task in the fleet.
type Task struct {
	ID           string    `json:"id"`
	Name         string    `json:"name"`
	Type         string    `json:"type"`          // "backup", "sync", "replication", "custom"
	NodeID       string    `json:"node_id"`       // Assigned node
	Status       string    `json:"status"`        // "pending", "running", "completed", "failed"
	Priority     int       `json:"priority"`      // Task priority 0-100
	Progress     float64   `json:"progress"`      // 0-1
	RetryCount   int       `json:"retry_count"`
	MaxRetries   int       `json:"max_retries"`
	Schedule     string    `json:"schedule"`      // Cron schedule
	LastRun      time.Time `json:"last_run"`
	NextRun      time.Time `json:"next_run"`
	CreatedAt    time.Time `json:"created_at"`
	CompletedAt  time.Time `json:"completed_at,omitempty"`
	Error        string    `json:"error,omitempty"`
}

// TaskReport represents aggregated task status report.
type TaskReport struct {
	GeneratedAt     time.Time `json:"generated_at"`
	TotalTasks      int       `json:"total_tasks"`
	PendingTasks    int       `json:"pending_tasks"`
	RunningTasks    int       `json:"running_tasks"`
	CompletedTasks  int       `json:"completed_tasks"`
	FailedTasks     int       `json:"failed_tasks"`
	NodeStats       map[string]*NodeTaskStats `json:"node_stats"`
	RecentErrors    []string  `json:"recent_errors"`
	SuccessRate     float64   `json:"success_rate"`
	AvgExecutionTime float64  `json:"avg_execution_time_seconds"`
}

// NodeTaskStats represents task statistics for a node.
type NodeTaskStats struct {
	NodeID        string  `json:"node_id"`
	NodeName      string  `json:"node_name"`
	TotalTasks    int     `json:"total_tasks"`
	RunningTasks  int     `json:"running_tasks"`
	CompletedTasks int    `json:"completed_tasks"`
	FailedTasks   int     `json:"failed_tasks"`
	AvgLoad       float64 `json:"avg_load"`
}

// SchedulerConfig holds scheduler configuration.
type SchedulerConfig struct {
	DefaultPriority    int     `json:"default_priority"`
	MaxRetries         int     `json:"max_retries"`
	LoadThreshold      float64 `json:"load_threshold"`     // Max load before rejecting tasks
	BalanceInterval    int     `json:"balance_interval"`   // Seconds between load balance checks
	FailoverEnabled    bool    `json:"failover_enabled"`
	HealthCheckInterval int    `json:"health_check_interval"` // Seconds
}

// Scheduler manages task distribution across fleet nodes.
type Scheduler struct {
	mu      sync.RWMutex
	nodes   map[string]*Node
	tasks   map[string]*Task
	config  *SchedulerConfig
	logger  *zap.Logger
	configPath string
}

// NewScheduler creates a new fleet scheduler.
func NewScheduler(configPath string, logger *zap.Logger) (*Scheduler, error) {
	if logger == nil {
		logger = zap.NewNop()
	}

	config := &SchedulerConfig{
		DefaultPriority:    50,
		MaxRetries:         3,
		LoadThreshold:      0.8,
		BalanceInterval:    60,
		FailoverEnabled:    true,
		HealthCheckInterval: 30,
	}

	s := &Scheduler{
		nodes:      make(map[string]*Node),
		tasks:      make(map[string]*Task),
		config:     config,
		logger:     logger,
		configPath: configPath,
	}

	if err := s.loadConfig(); err != nil && !os.IsNotExist(err) {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}

	return s, nil
}

// RegisterNode registers a new node in the fleet.
func (s *Scheduler) RegisterNode(ctx context.Context, name, address string, port int, priority int) (*Node, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	nodeID := uuid.New().String()

	now := time.Now()

	node := &Node{
		ID:        nodeID,
		Name:      name,
		Address:   address,
		Port:      port,
		Status:    "online",
		Priority:  priority,
		Capacity:  0,
		Load:      0,
		Tags:      []string{},
		LastSeen:  now,
		CreatedAt: now,
	}

	s.nodes[nodeID] = node
	s.logger.Info("Registered fleet node",
		zap.String("node_id", nodeID),
		zap.String("name", name),
		zap.Int("priority", priority))

	return node, s.saveConfig()
}

// RemoveNode removes a node from the fleet.
func (s *Scheduler) RemoveNode(ctx context.Context, nodeID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, exists := s.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	// Check for running tasks on this node
	for _, task := range s.tasks {
		if task.NodeID == nodeID && task.Status == "running" {
			// Mark task for failover
			task.Status = "pending"
			task.NodeID = ""
			s.logger.Warn("Node removed, task pending reassignment",
				zap.String("task_id", task.ID),
				zap.String("node_id", nodeID))
		}
	}

	delete(s.nodes, nodeID)
	s.logger.Info("Removed fleet node", zap.String("node_id", nodeID))

	return s.saveConfig()
}

// SetNodePriority sets the priority of a node.
func (s *Scheduler) SetNodePriority(ctx context.Context, nodeID string, priority int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	node, exists := s.nodes[nodeID]
	if !exists {
		return fmt.Errorf("node %s not found", nodeID)
	}

	if priority < 0 || priority > 100 {
		return fmt.Errorf("priority must be 0-100")
	}

	node.Priority = priority
	s.logger.Info("Set node priority",
		zap.String("node_id", nodeID),
		zap.Int("priority", priority))

	return s.saveConfig()
}

// SelectNode selects the best node for a task based on priority and load.
func (s *Scheduler) SelectNode(ctx context.Context, task *Task) (*Node, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(s.nodes) == 0 {
		return nil, fmt.Errorf("no nodes available in fleet")
	}

	// Filter online nodes with acceptable load
	candidates := make([]*Node, 0)
	for _, node := range s.nodes {
		if node.Status == "online" && node.Load < s.config.LoadThreshold {
			candidates = append(candidates, node)
		}
	}

	if len(candidates) == 0 {
		return nil, fmt.Errorf("no nodes with acceptable load")
	}

	// Sort by priority (descending) and load (ascending)
	// Higher priority + lower load = best candidate
	bestNode := candidates[0]
	bestScore := float64(bestNode.Priority) - bestNode.Load*10

	for _, node := range candidates[1:] {
		score := float64(node.Priority) - node.Load*10
		if score > bestScore {
			bestScore = score
			bestNode = node
		}
	}

	return bestNode, nil
}

// SubmitTask submits a new task to the scheduler.
func (s *Scheduler) SubmitTask(ctx context.Context, name, taskType string, priority int, schedule string) (*Task, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	taskID := uuid.New().String()

	if priority == 0 {
		priority = s.config.DefaultPriority
	}

	task := &Task{
		ID:         taskID,
		Name:       name,
		Type:       taskType,
		NodeID:     "",
		Status:     "pending",
		Priority:   priority,
		Progress:   0,
		RetryCount: 0,
		MaxRetries: s.config.MaxRetries,
		Schedule:   schedule,
		CreatedAt:  time.Now(),
	}

	// Calculate next run if schedule is set
	if schedule != "" {
		task.NextRun = time.Now().Add(1 * time.Minute) // Simplified
	}

	s.tasks[taskID] = task
	s.logger.Info("Submitted fleet task",
		zap.String("task_id", taskID),
		zap.String("name", name),
		zap.String("type", taskType))

	// Try to assign to a node
	node, err := s.SelectNode(ctx, task)
	if err == nil {
		task.NodeID = node.ID
		task.Status = "running"
		node.Load += 0.1 // Increase load estimate
		s.logger.Info("Assigned task to node",
			zap.String("task_id", taskID),
			zap.String("node_id", node.ID))
	}

	return task, s.saveConfig()
}

// UpdateTaskProgress updates task progress.
func (s *Scheduler) UpdateTaskProgress(ctx context.Context, taskID string, progress float64) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Progress = progress
	if progress >= 1.0 {
		task.Status = "completed"
		task.CompletedAt = time.Now()
		// Reduce node load
		if node, exists := s.nodes[task.NodeID]; exists {
			node.Load -= 0.1
			if node.Load < 0 {
				node.Load = 0
			}
		}
		s.logger.Info("Task completed", zap.String("task_id", taskID))
	}

	return s.saveConfig()
}

// FailTask marks a task as failed.
func (s *Scheduler) FailTask(ctx context.Context, taskID string, errMsg string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	task, exists := s.tasks[taskID]
	if !exists {
		return fmt.Errorf("task %s not found", taskID)
	}

	task.Error = errMsg
	task.RetryCount++

	// Reduce node load
	if node, exists := s.nodes[task.NodeID]; exists {
		node.Load -= 0.1
		if node.Load < 0 {
			node.Load = 0
		}
	}

	// Check retry limit
	if task.RetryCount < task.MaxRetries && s.config.FailoverEnabled {
		task.Status = "pending"
		task.NodeID = ""
		task.Progress = 0
		s.logger.Warn("Task failed, pending retry",
			zap.String("task_id", taskID),
			zap.Int("retry_count", task.RetryCount))
	} else {
		task.Status = "failed"
		s.logger.Error("Task failed permanently",
			zap.String("task_id", taskID),
			zap.String("error", errMsg))
	}

	return s.saveConfig()
}

// GetTaskReport generates an aggregated task status report.
func (s *Scheduler) GetTaskReport(ctx context.Context) *TaskReport {
	s.mu.RLock()
	defer s.mu.RUnlock()

	report := &TaskReport{
		GeneratedAt:  time.Now(),
		NodeStats:    make(map[string]*NodeTaskStats),
		RecentErrors: []string{},
	}

	// Count tasks by status
	for _, task := range s.tasks {
		report.TotalTasks++
		switch task.Status {
		case "pending":
			report.PendingTasks++
		case "running":
			report.RunningTasks++
		case "completed":
			report.CompletedTasks++
		case "failed":
			report.FailedTasks++
			if len(report.RecentErrors) < 10 {
				report.RecentErrors = append(report.RecentErrors, task.Error)
			}
		}

		// Aggregate node stats
		if task.NodeID != "" {
			stats, exists := report.NodeStats[task.NodeID]
			if !exists {
				node := s.nodes[task.NodeID]
				nodeName := "unknown"
				if node != nil {
					nodeName = node.Name
				}
				stats = &NodeTaskStats{
					NodeID:   task.NodeID,
					NodeName: nodeName,
				}
				report.NodeStats[task.NodeID] = stats
			}
			stats.TotalTasks++
			switch task.Status {
			case "running":
				stats.RunningTasks++
			case "completed":
				stats.CompletedTasks++
			case "failed":
				stats.FailedTasks++
			}
		}
	}

	// Calculate success rate
	if report.TotalTasks > 0 {
		report.SuccessRate = float64(report.CompletedTasks) / float64(report.TotalTasks)
	}

	return report
}

// ListNodes returns all fleet nodes.
func (s *Scheduler) ListNodes() []*Node {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Node, 0, len(s.nodes))
	for _, node := range s.nodes {
		result = append(result, node)
	}
	return result
}

// ListTasks returns all tasks.
func (s *Scheduler) ListTasks() []*Task {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*Task, 0, len(s.tasks))
	for _, task := range s.tasks {
		result = append(result, task)
	}
	return result
}

// loadConfig loads scheduler configuration.
func (s *Scheduler) loadConfig() error {
	data, err := os.ReadFile(s.configPath)
	if err != nil {
		return err
	}

	var cfg struct {
		Nodes  map[string]*Node `json:"nodes"`
		Tasks  map[string]*Task `json:"tasks"`
		Config *SchedulerConfig `json:"config"`
	}

	if err := json.Unmarshal(data, &cfg); err != nil {
		return fmt.Errorf("failed to parse config: %w", err)
	}

	s.nodes = cfg.Nodes
	s.tasks = cfg.Tasks
	if cfg.Config != nil {
		s.config = cfg.Config
	}

	return nil
}

// saveConfig saves scheduler configuration.
func (s *Scheduler) saveConfig() error {
	cfg := struct {
		Nodes  map[string]*Node `json:"nodes"`
		Tasks  map[string]*Task `json:"tasks"`
		Config *SchedulerConfig `json:"config"`
	}{
		Nodes:  s.nodes,
		Tasks:  s.tasks,
		Config: s.config,
	}

	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	dir := filepath.Dir(s.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config dir: %w", err)
	}

	return os.WriteFile(s.configPath, data, 0644)
}