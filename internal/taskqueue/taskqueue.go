package taskqueue

import (
	"container/heap"
	"context"
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ========== 优先级堆 ==========

// priorityItem 堆中的任务项.
type priorityItem struct {
	task     *Task
	index    int // 在堆中的索引
	sequence int // 入队顺序（同优先级 FIFO）
}

// priorityQueue 优先级队列（最大堆，优先级高的先出）.
type priorityQueue []*priorityItem

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	if pq[i].task.Priority != pq[j].task.Priority {
		return pq[i].task.Priority > pq[j].task.Priority // 高优先级优先
	}
	return pq[i].sequence < pq[j].sequence // 同优先级 FIFO
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *priorityQueue) Push(x interface{}) {
	item := x.(*priorityItem)
	item.index = len(*pq)
	*pq = append(*pq, item)
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil
	item.index = -1
	*pq = old[:n-1]
	return item
}

// ========== Worker ==========

// worker Worker定义.
type worker struct {
	id      int
	manager *Manager
	ctx     context.Context
	cancel  context.CancelFunc
}

// start 启动Worker.
func (w *worker) start() {
	go func() {
		for {
			select {
			case <-w.ctx.Done():
				return
			case item := <-w.manager.workCh:
				w.processItem(item)
			}
		}
	}()
}

// processItem 处理任务.
func (w *worker) processItem(item *priorityItem) {
	task := item.task
	mgr := w.manager

	// 标记开始
	now := time.Now()
	task.mu.Lock()
	task.Status = StatusRunning
	task.StartedAt = &now
	task.cancel = make(chan struct{})
	task.mu.Unlock()

	// 通知进度回调
	mgr.notifyProgress(task.ID, 0)

	// 创建任务上下文
	taskCtx := &TaskContext{
		TaskID:   task.ID,
		Name:     task.Name,
		Payload:  task.Payload,
		Progress: 0,
		cancel:   task.cancel,
	}

	// 执行任务（带超时）
	var execErr error
	done := make(chan error, 1)

	go func() {
		done <- task.handler(taskCtx)
	}()

	if task.Timeout > 0 {
		select {
		case execErr = <-done:
			// 正常完成
		case <-time.After(task.Timeout):
			execErr = fmt.Errorf("任务超时: %s", task.Timeout)
			task.mu.Lock()
			task.Status = StatusTimeout
			task.mu.Unlock()
			// 触发取消
			task.cancelOnce.Do(func() { close(task.cancel) })
			<-done // 等待handler退出
		case <-task.cancel:
			execErr = fmt.Errorf("任务被取消")
			task.mu.Lock()
			task.Status = StatusCancelled
			task.mu.Unlock()
			<-done
		}
	} else {
		select {
		case execErr = <-done:
			// 正常完成
		case <-task.cancel:
			execErr = fmt.Errorf("任务被取消")
			task.mu.Lock()
			task.Status = StatusCancelled
			task.mu.Unlock()
			<-done
		}
	}

	// 更新进度
	task.mu.Lock()
	if execErr == nil {
		task.Progress = 1.0
		task.Status = StatusSuccess
	} else if task.Status == StatusRunning {
		// 只有还在running状态的才判断重试
		task.Error = execErr.Error()
		task.Status = StatusFailed
	}
	task.mu.Unlock()

	// 处理重试
	if task.Status == StatusFailed {
		mgr.handleRetry(task, execErr)
		return
	}

	// 完成处理
	completedAt := time.Now()
	task.mu.Lock()
	task.CompletedAt = &completedAt
	task.mu.Unlock()

	// 从运行集移除
	mgr.mu.Lock()
	delete(mgr.running, task.ID)
	mgr.mu.Unlock()

	// 通知完成回调
	mgr.notifyComplete(task.ID, task.Status, execErr)

	// 尝试触发依赖任务
	if task.Status == StatusSuccess {
		mgr.triggerDependents(task.ID)
	}
}

// handleRetry 处理重试逻辑.
func (m *Manager) handleRetry(task *Task, execErr error) {
	m.mu.Lock()
	task.RetryCount++

	if task.RetryCount >= task.MaxRetries {
		// 重试耗尽，进入死信队列
		task.Status = StatusDead
		completedAt := time.Now()
		task.CompletedAt = &completedAt
		delete(m.running, task.ID)

		// 加入死信队列
		m.deadLetter = append(m.deadLetter, task)
		if m.config.DeadLetterLimit > 0 && len(m.deadLetter) > m.config.DeadLetterLimit {
			m.deadLetter = m.deadLetter[1:]
		}
		m.mu.Unlock()

		m.notifyComplete(task.ID, StatusDead, execErr)
		return
	}

	// 计算退避延迟
	delay := task.RetryDelay
	if task.BackoffFactor > 0 && task.RetryCount > 1 {
		for i := 1; i < task.RetryCount; i++ {
			delay = time.Duration(float64(delay) * task.BackoffFactor)
		}
	}
	task.Status = StatusRetrying
	delete(m.running, task.ID)
	m.mu.Unlock()

	// 延迟后重新入队
	go func() {
		time.Sleep(delay)
		m.mu.Lock()
		if m.stopped {
			m.mu.Unlock()
			return
		}
		task.Status = StatusReady
		item := &priorityItem{
			task:     task,
			sequence: m.nextSeq,
		}
		m.nextSeq++
		heap.Push(&m.queue, item)
		m.mu.Unlock()

		// 通知调度
		select {
		case m.notifyCh <- struct{}{}:
		default:
		}
	}()
}

// ========== Manager ==========

// Manager 任务队列管理器.
type Manager struct {
	mu      sync.RWMutex
	config  *Config
	started bool
	stopped bool

	// 优先级队列
	queue   priorityQueue
	nextSeq int

	// 运行中的任务
	running map[string]*Task

	// 所有任务索引
	tasks map[string]*Task

	// 死信队列
	deadLetter []*Task

	// Worker池
	workers []*worker
	workCh  chan *priorityItem

	// 调度通知
	notifyCh chan struct{}

	// 回调
	progressCallbacks []ProgressCallback
	completeCallbacks []CompletionCallback
}

// NewManager 创建管理器.
func NewManager(cfg *Config) *Manager {
	if cfg == nil {
		cfg = DefaultConfig()
	}
	if cfg.MaxWorkers <= 0 {
		cfg.MaxWorkers = 4
	}
	if cfg.PollInterval <= 0 {
		cfg.PollInterval = 100 * time.Millisecond
	}
	if cfg.DeadLetterLimit <= 0 {
		cfg.DeadLetterLimit = 1000
	}

	return &Manager{
		config:   cfg,
		tasks:    make(map[string]*Task),
		running:  make(map[string]*Task),
		workCh:   make(chan *priorityItem, cfg.MaxWorkers*2),
		notifyCh: make(chan struct{}, 1),
	}
}

// Start 启动管理器.
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.started {
		return nil
	}

	m.stopped = false
	m.started = true

	// 初始化堆
	heap.Init(&m.queue)

	// 启动Worker
	m.workers = make([]*worker, m.config.MaxWorkers)
	for i := 0; i < m.config.MaxWorkers; i++ {
		ctx, cancel := context.WithCancel(context.Background())
		w := &worker{
			id:      i,
			manager: m,
			ctx:     ctx,
			cancel:  cancel,
		}
		m.workers[i] = w
		w.start()
	}

	// 启动调度循环
	go m.scheduleLoop()

	return nil
}

// Stop 停止管理器.
func (m *Manager) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.started {
		return nil
	}

	m.stopped = true
	m.started = false

	// 停止所有Worker
	for _, w := range m.workers {
		w.cancel()
	}

	return nil
}

// IsRunning 运行状态.
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.started
}

// ========== 任务提交 ==========

// Submit 提交任务.
func (m *Manager) Submit(opts *TaskOptions, handler TaskHandler) (*Task, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.stopped {
		return nil, ErrQueueStopped
	}

	if m.config.MaxQueueSize > 0 && len(m.queue) >= m.config.MaxQueueSize {
		return nil, ErrQueueFull
	}

	if opts == nil {
		opts = &TaskOptions{}
	}

	task := &Task{
		ID:            uuid.New().String(),
		Name:          opts.Name,
		Description:   opts.Description,
		Status:        StatusPending,
		Priority:      opts.Priority,
		Payload:       opts.Payload,
		MaxRetries:    opts.MaxRetries,
		RetryDelay:    opts.RetryDelay,
		BackoffFactor: opts.BackoffFactor,
		Timeout:       opts.Timeout,
		Dependencies:  opts.Dependencies,
		CreatedAt:     time.Now(),
		handler:       handler,
		onProgress:    opts.OnProgress,
		onComplete:    opts.OnComplete,
	}

	// 应用默认值
	if task.MaxRetries == 0 {
		task.MaxRetries = m.config.DefaultRetry
	}
	if task.RetryDelay == 0 {
		task.RetryDelay = m.config.DefaultDelay
	}
	if task.BackoffFactor == 0 {
		task.BackoffFactor = 2.0
	}
	if task.Timeout == 0 {
		task.Timeout = m.config.DefaultTimeout
	}

	// 检查依赖
	if len(task.Dependencies) > 0 {
		if err := m.validateDependencies(task); err != nil {
			return nil, err
		}
		task.Status = StatusPending
	} else {
		// 无依赖，直接入队
		task.Status = StatusReady
		item := &priorityItem{
			task:     task,
			sequence: m.nextSeq,
		}
		m.nextSeq++
		heap.Push(&m.queue, item)
	}

	m.tasks[task.ID] = task

	return task, nil
}

// TaskOptions 任务提交选项.
type TaskOptions struct {
	Name          string                 `json:"name"`
	Description   string                 `json:"description,omitempty"`
	Priority      TaskPriority           `json:"priority"`
	Payload       map[string]interface{} `json:"payload,omitempty"`
	MaxRetries    int                    `json:"max_retries"`
	RetryDelay    time.Duration          `json:"retry_delay"`
	BackoffFactor float64                `json:"backoff_factor"`
	Timeout       time.Duration          `json:"timeout"`
	Dependencies  []string               `json:"dependencies,omitempty"`
	OnProgress    ProgressCallback       `json:"-"`
	OnComplete    CompletionCallback     `json:"-"`
}

// ========== 任务操作 ==========

// GetTask 获取任务.
func (m *Manager) GetTask(id string) (*Task, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, exists := m.tasks[id]
	if !exists {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// CancelTask 取消任务.
func (m *Manager) CancelTask(id string) error {
	m.mu.RLock()
	task, exists := m.tasks[id]
	m.mu.RUnlock()

	if !exists {
		return ErrTaskNotFound
	}

	task.mu.Lock()
	defer task.mu.Unlock()

	switch task.Status {
	case StatusPending, StatusReady, StatusRetrying:
		// 从队列中移除（简化处理：标记取消，调度时跳过）
		task.Status = StatusCancelled
		completedAt := time.Now()
		task.CompletedAt = &completedAt
		return nil
	case StatusRunning:
		// 发送取消信号
		task.cancelOnce.Do(func() { close(task.cancel) })
		return nil
	default:
		return ErrTaskNotCancellable
	}
}

// ListTasks 列出任务.
func (m *Manager) ListTasks(filter TaskFilter) []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Task, 0)
	for _, task := range m.tasks {
		if !matchesTaskFilter(task, filter) {
			continue
		}
		result = append(result, task)
	}

	// 排序
	sort.Slice(result, func(i, j int) bool {
		if result[i].Priority != result[j].Priority {
			return result[i].Priority > result[j].Priority
		}
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})

	// 分页
	if filter.Offset > len(result) {
		return []*Task{}
	}
	end := filter.Offset + filter.Limit
	if filter.Limit <= 0 || end > len(result) {
		end = len(result)
	}
	return result[filter.Offset:end]
}

// GetStats 获取队列统计.
func (m *Manager) GetStats() *QueueStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &QueueStats{
		ByStatus:       make(map[string]int),
		ByPriority:     make(map[string]int),
		RunningWorkers: len(m.running),
		MaxWorkers:     m.config.MaxWorkers,
		QueueSize:      m.queue.Len(),
		DeadLetterSize: len(m.deadLetter),
	}

	var totalWait, totalExec time.Duration
	var waitCount, execCount int

	for _, task := range m.tasks {
		stats.TotalTasks++
		stats.ByStatus[string(task.Status)]++
		stats.ByPriority[task.Priority.String()]++

		if task.StartedAt != nil {
			wait := task.StartedAt.Sub(task.CreatedAt)
			totalWait += wait
			waitCount++
		}
		if task.CompletedAt != nil && task.StartedAt != nil {
			exec := task.CompletedAt.Sub(*task.StartedAt)
			totalExec += exec
			execCount++
		}
	}

	if waitCount > 0 {
		stats.AvgWaitTime = float64(totalWait.Milliseconds()) / float64(waitCount)
	}
	if execCount > 0 {
		stats.AvgExecTime = float64(totalExec.Milliseconds()) / float64(execCount)
	}

	return stats
}

// GetDeadLetter 获取死信队列.
func (m *Manager) GetDeadLetter() []*Task {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Task, len(m.deadLetter))
	copy(result, m.deadLetter)
	return result
}

// RetryDeadLetter 重试死信任务.
func (m *Manager) RetryDeadLetter(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, task := range m.deadLetter {
		if task.ID == taskID {
			// 从死信队列移除
			m.deadLetter = append(m.deadLetter[:i], m.deadLetter[i+1:]...)

			// 重置状态
			task.Status = StatusReady
			task.RetryCount = 0
			task.Error = ""
			task.CompletedAt = nil

			// 重新入队
			item := &priorityItem{
				task:     task,
				sequence: m.nextSeq,
			}
			m.nextSeq++
			heap.Push(&m.queue, item)

			// 通知调度
			select {
			case m.notifyCh <- struct{}{}:
			default:
			}

			return nil
		}
	}

	return ErrTaskNotFound
}

// ========== 回调注册 ==========

// OnProgress 注册进度回调.
func (m *Manager) OnProgress(cb ProgressCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.progressCallbacks = append(m.progressCallbacks, cb)
}

// OnComplete 注册完成回调.
func (m *Manager) OnComplete(cb CompletionCallback) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.completeCallbacks = append(m.completeCallbacks, cb)
}

// ========== 内部方法 ==========

// scheduleLoop 调度循环.
func (m *Manager) scheduleLoop() {
	ticker := time.NewTicker(m.config.PollInterval)
	defer ticker.Stop()

	for {
		m.mu.RLock()
		if m.stopped {
			m.mu.RUnlock()
			return
		}
		m.mu.RUnlock()

		select {
		case <-ticker.C:
			m.dispatch()
		case <-m.notifyCh:
			m.dispatch()
		}
	}
}

// dispatch 分发任务到Worker.
func (m *Manager) dispatch() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查依赖任务
	m.checkPendingDependencies()

	for m.queue.Len() > 0 {
		// 检查Worker容量
		if len(m.running) >= m.config.MaxWorkers {
			return
		}

		item := heap.Pop(&m.queue).(*priorityItem)
		task := item.task

		// 跳过已取消的任务
		if task.Status == StatusCancelled {
			continue
		}

		m.running[task.ID] = task
		m.workCh <- item
	}
}

// checkPendingDependencies 检查待处理的依赖任务.
func (m *Manager) checkPendingDependencies() {
	for _, task := range m.tasks {
		if task.Status != StatusPending {
			continue
		}

		// 检查所有依赖是否完成
		allDone := true
		for _, depID := range task.Dependencies {
			dep, exists := m.tasks[depID]
			if !exists || dep.Status != StatusSuccess {
				allDone = false
				break
			}
		}

		if allDone {
			task.Status = StatusReady
			item := &priorityItem{
				task:     task,
				sequence: m.nextSeq,
			}
			m.nextSeq++
			heap.Push(&m.queue, item)
		}
	}
}

// triggerDependents 触发依赖任务.
func (m *Manager) triggerDependents(completedTaskID string) {
	m.checkPendingDependencies()

	// 通知调度
	select {
	case m.notifyCh <- struct{}{}:
	default:
	}
}

// validateDependencies 验证依赖关系.
func (m *Manager) validateDependencies(task *Task) error {
	depSet := make(map[string]bool)
	for _, depID := range task.Dependencies {
		if depID == task.ID {
			return ErrSelfDependency
		}
		if depSet[depID] {
			return ErrDuplicateDep
		}
		depSet[depID] = true
	}

	// 检查循环依赖（BFS）
	if m.hasCycle(task.ID, task.Dependencies) {
		return ErrCycleDetected
	}

	return nil
}

// hasCycle 检测是否有循环依赖.
func (m *Manager) hasCycle(taskID string, deps []string) bool {
	// 构建临时图
	graph := make(map[string][]string)
	for _, t := range m.tasks {
		graph[t.ID] = t.Dependencies
	}
	graph[taskID] = deps

	// DFS检测环
	visited := make(map[string]bool)
	recStack := make(map[string]bool)

	var dfs func(id string) bool
	dfs = func(id string) bool {
		visited[id] = true
		recStack[id] = true

		for _, dep := range graph[id] {
			if !visited[dep] {
				if dfs(dep) {
					return true
				}
			} else if recStack[dep] {
				return true
			}
		}

		recStack[id] = false
		return false
	}

	return dfs(taskID)
}

// notifyProgress 通知进度.
func (m *Manager) notifyProgress(taskID string, progress float64) {
	m.mu.RLock()
	cbs := make([]ProgressCallback, len(m.progressCallbacks))
	copy(cbs, m.progressCallbacks)
	m.mu.RUnlock()

	// 同时通知任务自身回调
	m.mu.RLock()
	task, exists := m.tasks[taskID]
	m.mu.RUnlock()

	if exists {
		task.mu.RLock()
		if task.onProgress != nil {
			task.onProgress(taskID, progress)
		}
		task.mu.RUnlock()
	}

	for _, cb := range cbs {
		cb(taskID, progress)
	}
}

// notifyComplete 通知完成.
func (m *Manager) notifyComplete(taskID string, status TaskStatus, err error) {
	m.mu.RLock()
	cbs := make([]CompletionCallback, len(m.completeCallbacks))
	copy(cbs, m.completeCallbacks)
	m.mu.RUnlock()

	m.mu.RLock()
	task, exists := m.tasks[taskID]
	m.mu.RUnlock()

	if exists {
		task.mu.RLock()
		if task.onComplete != nil {
			task.onComplete(taskID, status, err)
		}
		task.mu.RUnlock()
	}

	for _, cb := range cbs {
		cb(taskID, status, err)
	}
}

// ========== 辅助函数 ==========

func matchesTaskFilter(task *Task, filter TaskFilter) bool {
	if len(filter.Status) > 0 {
		found := false
		for _, s := range filter.Status {
			if s == task.Status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(filter.Priority) > 0 {
		found := false
		for _, p := range filter.Priority {
			if p == task.Priority {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if filter.Name != "" {
		if task.Name != filter.Name {
			return false
		}
	}

	return true
}
