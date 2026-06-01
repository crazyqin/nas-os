package smartbandwidthpredict

import (
	"container/heap"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Scheduler 智能调度器
type Scheduler struct {
	mu     sync.RWMutex
	config *Config
	logger *zap.Logger
	plans  []*SchedulePlan
}

// NewScheduler 创建智能调度器
func NewScheduler(config *Config, logger *zap.Logger) *Scheduler {
	if config == nil {
		config = DefaultConfig()
	}
	if logger == nil {
		logger = zap.NewNop()
	}

	return &Scheduler{
		config: config,
		logger: logger,
		plans:  make([]*SchedulePlan, 0),
	}
}

// CreatePlan 创建调度计划
func (s *Scheduler) CreatePlan(tasks []*ScheduleTask, prediction *BandwidthPrediction, policies map[string]*QoSPolicy) (*SchedulePlan, error) {
	if len(tasks) == 0 {
		return nil, fmt.Errorf("任务列表不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 创建优先级队列
	pq := make(PriorityQueue, len(tasks))
	for i, task := range tasks {
		pq[i] = &TaskItem{
			task:     task,
			priority: task.Priority,
			index:    i,
		}
	}
	heap.Init(&pq)

	// 计算可用带宽
	availableMbps := s.calculateAvailableBandwidth(prediction, policies)

	// 调度任务
	scheduledTasks := make([]*ScheduleTask, 0)
	usedMbps := 0.0
	currentTime := time.Now()

	for pq.Len() > 0 {
		item := heap.Pop(&pq).(*TaskItem)
		task := item.task

		// 检查带宽是否足够
		if usedMbps+task.RequiredMbps > availableMbps {
			s.logger.Warn("带宽不足，跳过任务",
				zap.String("task_id", task.ID),
				zap.Float64("required", task.RequiredMbps),
				zap.Float64("available", availableMbps-usedMbps),
			)
			continue
		}

		// 设置调度时间
		task.ScheduledAt = currentTime
		task.Deadline = currentTime.Add(task.Duration)

		scheduledTasks = append(scheduledTasks, task)
		usedMbps += task.RequiredMbps

		// 更新当前时间（模拟串行调度）
		currentTime = task.Deadline
	}

	plan := &SchedulePlan{
		ID:        fmt.Sprintf("plan_%d", time.Now().UnixNano()),
		Tasks:     scheduledTasks,
		StartTime: time.Now(),
		EndTime:   currentTime,
		TotalMbps: usedMbps,
		CreatedAt: time.Now(),
	}

	s.plans = append(s.plans, plan)

	s.logger.Info("调度计划创建成功",
		zap.String("plan_id", plan.ID),
		zap.Int("scheduled_tasks", len(scheduledTasks)),
		zap.Int("total_tasks", len(tasks)),
		zap.Float64("used_mbps", usedMbps),
		zap.Float64("available_mbps", availableMbps),
	)

	return plan, nil
}

// calculateAvailableBandwidth 计算可用带宽
func (s *Scheduler) calculateAvailableBandwidth(prediction *BandwidthPrediction, policies map[string]*QoSPolicy) float64 {
	// 基础可用带宽
	available := prediction.PredictedMbps

	// 预留带宽（基于置信度）
	reservedRatio := 0.1 * (1 - prediction.Confidence)
	available *= (1 - reservedRatio)

	// 考虑趋势
	switch prediction.Trend {
	case TrendRising:
		available *= 0.9 // 上升趋势时保守分配
	case TrendFalling:
		available *= 1.1 // 下降趋势时可以多分配
	}

	// 应用QoS策略的最小带宽保证
	totalMinMbps := 0.0
	for _, policy := range policies {
		if policy.Enabled {
			totalMinMbps += policy.MinMbps
		}
	}

	// 确保不超过总带宽限制
	if available > s.config.TotalBandwidthMbps {
		available = s.config.TotalBandwidthMbps
	}

	// 减去QoS保证的带宽
	available -= totalMinMbps
	if available < 0 {
		available = 0
	}

	return available
}

// AdjustPlan 动态调整调度计划
func (s *Scheduler) AdjustPlan(planID string, newPrediction *BandwidthPrediction) (*SchedulePlan, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 查找计划
	var plan *SchedulePlan
	for _, p := range s.plans {
		if p.ID == planID {
			plan = p
			break
		}
	}

	if plan == nil {
		return nil, fmt.Errorf("调度计划不存在: %s", planID)
	}

	// 重新计算带宽分配
	availableMbps := newPrediction.PredictedMbps * 0.8

	// 重新调度任务
	rescheduledTasks := make([]*ScheduleTask, 0)
	usedMbps := 0.0
	currentTime := time.Now()

	// 按优先级重新排序
	pq := make(PriorityQueue, len(plan.Tasks))
	for i, task := range plan.Tasks {
		pq[i] = &TaskItem{
			task:     task,
			priority: task.Priority,
			index:    i,
		}
	}
	heap.Init(&pq)

	for pq.Len() > 0 {
		item := heap.Pop(&pq).(*TaskItem)
		task := item.task

		if usedMbps+task.RequiredMbps <= availableMbps {
			task.ScheduledAt = currentTime
			task.Deadline = currentTime.Add(task.Duration)
			rescheduledTasks = append(rescheduledTasks, task)
			usedMbps += task.RequiredMbps
			currentTime = task.Deadline
		}
	}

	// 更新计划
	plan.Tasks = rescheduledTasks
	plan.TotalMbps = usedMbps
	plan.EndTime = currentTime

	s.logger.Info("调度计划调整完成",
		zap.String("plan_id", planID),
		zap.Int("rescheduled_tasks", len(rescheduledTasks)),
		zap.Float64("new_available_mbps", availableMbps),
	)

	return plan, nil
}

// GetPlan 获取调度计划
func (s *Scheduler) GetPlan(planID string) (*SchedulePlan, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, plan := range s.plans {
		if plan.ID == planID {
			return plan, nil
		}
	}

	return nil, fmt.Errorf("调度计划不存在: %s", planID)
}

// GetAllPlans 获取所有调度计划
func (s *Scheduler) GetAllPlans() []*SchedulePlan {
	s.mu.RLock()
	defer s.mu.RUnlock()

	result := make([]*SchedulePlan, len(s.plans))
	copy(result, s.plans)
	return result
}

// DeletePlan 删除调度计划
func (s *Scheduler) DeletePlan(planID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for i, plan := range s.plans {
		if plan.ID == planID {
			s.plans = append(s.plans[:i], s.plans[i+1:]...)
			return nil
		}
	}

	return fmt.Errorf("调度计划不存在: %s", planID)
}

// GetPlanStats 获取计划统计
func (s *Scheduler) GetPlanStats() *PlanStats {
	s.mu.RLock()
	defer s.mu.RUnlock()

	stats := &PlanStats{
		TotalPlans: len(s.plans),
	}

	for _, plan := range s.plans {
		stats.TotalTasks += len(plan.Tasks)
		stats.TotalMbps += plan.TotalMbps
	}

	if stats.TotalPlans > 0 {
		stats.AvgTasksPerPlan = float64(stats.TotalTasks) / float64(stats.TotalPlans)
		stats.AvgMbpsPerPlan = stats.TotalMbps / float64(stats.TotalPlans)
	}

	return stats
}

// OptimizeForPeakHours 优化高峰时段调度
func (s *Scheduler) OptimizeForPeakHours(tasks []*ScheduleTask, peakHours []int, prediction *BandwidthPrediction) []*ScheduleTask {
	if len(tasks) == 0 || len(peakHours) == 0 {
		return tasks
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 高峰时段降低带宽分配
	peakReduction := 0.7 // 高峰时段只分配70%带宽

	optimized := make([]*ScheduleTask, 0, len(tasks))
	currentHour := time.Now().Hour()

	isPeakHour := false
	for _, hour := range peakHours {
		if currentHour == hour {
			isPeakHour = true
			break
		}
	}

	for _, task := range tasks {
		newTask := *task

		if isPeakHour {
			// 高峰时段降低带宽需求
			newTask.RequiredMbps *= peakReduction
		}

		optimized = append(optimized, &newTask)
	}

	return optimized
}

// EstimateCompletionTime 估算完成时间
func (s *Scheduler) EstimateCompletionTime(tasks []*ScheduleTask, availableMbps float64) time.Duration {
	if len(tasks) == 0 || availableMbps <= 0 {
		return 0
	}

	s.mu.RLock()
	defer s.mu.RUnlock()

	// 按优先级排序
	pq := make(PriorityQueue, len(tasks))
	for i, task := range tasks {
		pq[i] = &TaskItem{
			task:     task,
			priority: task.Priority,
			index:    i,
		}
	}
	heap.Init(&pq)

	totalDuration := time.Duration(0)
	usedMbps := 0.0

	for pq.Len() > 0 {
		item := heap.Pop(&pq).(*TaskItem)
		task := item.task

		if usedMbps+task.RequiredMbps <= availableMbps {
			usedMbps += task.RequiredMbps
			totalDuration += task.Duration
		}
	}

	return totalDuration
}

// UpdateConfig 更新配置
func (s *Scheduler) UpdateConfig(config *Config) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if config != nil {
		s.config = config
	}
}

// GetConfig 获取配置
func (s *Scheduler) GetConfig() *Config {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.config
}

// PlanStats 计划统计
type PlanStats struct {
	TotalPlans     int     `json:"total_plans"`
	TotalTasks     int     `json:"total_tasks"`
	TotalMbps      float64 `json:"total_mbps"`
	AvgTasksPerPlan float64 `json:"avg_tasks_per_plan"`
	AvgMbpsPerPlan  float64 `json:"avg_mbps_per_plan"`
}

// TaskItem 任务项（用于优先级队列）
type TaskItem struct {
	task     *ScheduleTask
	priority int
	index    int
}

// PriorityQueue 优先级队列
type PriorityQueue []*TaskItem

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	// 优先级高的排在前面
	return pq[i].priority > pq[j].priority
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*TaskItem)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[0 : n-1]
	return item
}
