package licensescan

import (
	"log"
	"sync"
	"time"
)

// Scheduler 定期扫描调度器.
type Scheduler struct {
	mu       sync.Mutex
	manager  *Manager
	tasks    []ScheduledTask
	stopCh   chan struct{}
	running  bool
}

// ScheduledTask 定期扫描任务.
type ScheduledTask struct {
	ID         string        `json:"id"`          // 任务ID
	Name       string        `json:"name"`        // 任务名称
	ScanType   ScanType      `json:"scan_type"`   // 扫描类型
	Targets    []string      `json:"targets"`     // 扫描目标列表
	PolicyID   string        `json:"policy_id"`   // 使用的策略ID
	Interval   time.Duration `json:"interval"`    // 扫描间隔
	Enabled    bool          `json:"enabled"`     // 是否启用
	LastRun    time.Time     `json:"last_run"`    // 上次运行时间
	NextRun    time.Time     `json:"next_run"`    // 下次运行时间
}

// NewScheduler 创建定期扫描调度器.
func NewScheduler(manager *Manager) *Scheduler {
	return &Scheduler{
		manager: manager,
		tasks:   make([]ScheduledTask, 0),
		stopCh:  make(chan struct{}),
	}
}

// AddTask 添加定期扫描任务.
func (s *Scheduler) AddTask(task ScheduledTask) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if task.ID == "" {
		task.ID = "task-" + time.Now().Format("20060102150405")
	}
	if task.Interval <= 0 {
		task.Interval = 24 * time.Hour // 默认每天一次
	}
	task.NextRun = time.Now().Add(task.Interval)
	s.tasks = append(s.tasks, task)
}

// RemoveTask 移除定期扫描任务.
func (s *Scheduler) RemoveTask(id string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, task := range s.tasks {
		if task.ID == id {
			s.tasks = append(s.tasks[:i], s.tasks[i+1:]...)
			return true
		}
	}
	return false
}

// ListTasks 列出所有定期扫描任务.
func (s *Scheduler) ListTasks() []ScheduledTask {
	s.mu.Lock()
	defer s.mu.Unlock()

	tasks := make([]ScheduledTask, len(s.tasks))
	copy(tasks, s.tasks)
	return tasks
}

// Start 启动调度器.
func (s *Scheduler) Start() {
	s.mu.Lock()
	if s.running {
		s.mu.Unlock()
		return
	}
	s.running = true
	s.mu.Unlock()

	go s.run()
}

// Stop 停止调度器.
func (s *Scheduler) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}
	s.running = false
	close(s.stopCh)
	s.stopCh = make(chan struct{})
}

// run 调度器主循环.
func (s *Scheduler) run() {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopCh:
			return
		case now := <-ticker.C:
			s.runDueTasks(now)
		}
	}
}

// runDueTasks 运行到期的扫描任务.
func (s *Scheduler) runDueTasks(now time.Time) {
	s.mu.Lock()
	var dueTasks []int
	for i, task := range s.tasks {
		if task.Enabled && !task.NextRun.IsZero() && now.After(task.NextRun) {
			dueTasks = append(dueTasks, i)
		}
	}
	s.mu.Unlock()

	for _, idx := range dueTasks {
		s.mu.Lock()
		if idx >= len(s.tasks) {
			s.mu.Unlock()
			continue
		}
		task := s.tasks[idx]
		s.tasks[idx].LastRun = now
		s.tasks[idx].NextRun = now.Add(task.Interval)
		s.mu.Unlock()

		go s.executeTask(task)
	}
}

// executeTask 执行扫描任务.
func (s *Scheduler) executeTask(task ScheduledTask) {
	log.Printf("[licensescan] 执行定期扫描任务: %s", task.Name)

	switch task.ScanType {
	case ScanTypeDocker:
		for _, target := range task.Targets {
			result, err := s.manager.RunDockerScan(target, task.PolicyID)
			if err != nil {
				log.Printf("[licensescan] Docker扫描失败 %s: %v", target, err)
			} else if len(result.Violations) > 0 {
				log.Printf("[licensescan] Docker扫描发现 %d 个违规: %s", len(result.Violations), target)
			}
		}
	case ScanTypeGoMod:
		for _, target := range task.Targets {
			result, err := s.manager.RunGoModScan(target, task.PolicyID)
			if err != nil {
				log.Printf("[licensescan] Go模块扫描失败 %s: %v", target, err)
			} else if len(result.Violations) > 0 {
				log.Printf("[licensescan] Go模块扫描发现 %d 个违规: %s", len(result.Violations), target)
			}
		}
	case ScanTypeFull:
		// 区分Docker和Go mod目标
		var dockerTargets, goModTargets []string
		for _, target := range task.Targets {
			if len(target) > 4 && target[len(target)-4:] == ".mod" {
				goModTargets = append(goModTargets, target)
			} else {
				dockerTargets = append(dockerTargets, target)
			}
		}
		for _, img := range dockerTargets {
			if _, err := s.manager.RunDockerScan(img, task.PolicyID); err != nil {
				log.Printf("[licensescan] Docker扫描失败 %s: %v", img, err)
			}
		}
		for _, mod := range goModTargets {
			if _, err := s.manager.RunGoModScan(mod, task.PolicyID); err != nil {
				log.Printf("[licensescan] Go模块扫描失败 %s: %v", mod, err)
			}
		}
	}
}
