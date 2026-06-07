package downloadstation

import (
	"container/heap"
	"sync"
	"time"
)

// DownloadQueue 下载队列.
type DownloadQueue struct {
	mu         sync.RWMutex
	items      *priorityQueue
	active     map[string]*DownloadTask
	config     QueueConfig
	speedStats []SpeedStats
	maxStats   int
	totalSpeed int64
	stopCh     chan struct{}
}

// NewDownloadQueue 创建下载队列.
func NewDownloadQueue(config QueueConfig) *DownloadQueue {
	pq := &priorityQueue{}
	heap.Init(pq)

	q := &DownloadQueue{
		items:    pq,
		active:   make(map[string]*DownloadTask),
		config:   config,
		maxStats: 3600, // 保留 1 小时的速度统计
		stopCh:   make(chan struct{}),
	}

	return q
}

// Push 添加任务到队列.
func (q *DownloadQueue) Push(task *DownloadTask) {
	q.mu.Lock()
	defer q.mu.Unlock()

	task.Status = TaskStatusQueued
	heap.Push(q.items, task)
}

// Pop 从队列取出最高优先级的任务.
func (q *DownloadQueue) Pop() *DownloadTask {
	q.mu.Lock()
	defer q.mu.Unlock()

	if q.items.Len() == 0 {
		return nil
	}

	item := heap.Pop(q.items).(*DownloadTask)
	return item
}

// Peek 查看队首任务（不移除）.
func (q *DownloadQueue) Peek() *DownloadTask {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if q.items.Len() == 0 {
		return nil
	}

	return (*q.items)[0]
}

// Len 返回队列长度.
func (q *DownloadQueue) Len() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.items.Len()
}

// ActiveCount 返回活跃任务数.
func (q *DownloadQueue) ActiveCount() int {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.active)
}

// CanStart 检查是否可以开始新任务.
func (q *DownloadQueue) CanStart() bool {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return len(q.active) < q.config.MaxConcurrent
}

// StartTask 将任务标记为活跃.
func (q *DownloadQueue) StartTask(task *DownloadTask) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.active[task.ID] = task
}

// CompleteTask 将任务标记为完成.
func (q *DownloadQueue) CompleteTask(taskID string) {
	q.mu.Lock()
	defer q.mu.Unlock()

	delete(q.active, taskID)
}

// GetActive 获取所有活跃任务.
func (q *DownloadQueue) GetActive() []*DownloadTask {
	q.mu.RLock()
	defer q.mu.RUnlock()

	tasks := make([]*DownloadTask, 0, len(q.active))
	for _, task := range q.active {
		tasks = append(tasks, task)
	}
	return tasks
}

// GetQueued 获取所有排队任务.
func (q *DownloadQueue) GetQueued() []*DownloadTask {
	q.mu.RLock()
	defer q.mu.RUnlock()

	tasks := make([]*DownloadTask, q.items.Len())
	copy(tasks, *q.items)
	return tasks
}

// UpdateConfig 更新队列配置.
func (q *DownloadQueue) UpdateConfig(config QueueConfig) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.config = config
}

// GetConfig 获取队列配置.
func (q *DownloadQueue) GetConfig() QueueConfig {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.config
}

// UpdateTotalSpeed 更新总速度.
func (q *DownloadQueue) UpdateTotalSpeed(speed int64) {
	q.mu.Lock()
	defer q.mu.Unlock()

	q.totalSpeed = speed
	q.recordSpeedStats(speed)
}

// GetTotalSpeed 获取当前总速度.
func (q *DownloadQueue) GetTotalSpeed() int64 {
	q.mu.RLock()
	defer q.mu.RUnlock()
	return q.totalSpeed
}

// recordSpeedStats 记录速度统计.
func (q *DownloadQueue) recordSpeedStats(speed int64) {
	stat := SpeedStats{
		Timestamp:   time.Now(),
		Speed:       speed,
		ActiveTasks: len(q.active),
		TotalSpeed:  speed,
	}

	q.speedStats = append(q.speedStats, stat)

	// 限制统计数量
	if len(q.speedStats) > q.maxStats {
		q.speedStats = q.speedStats[len(q.speedStats)-q.maxStats:]
	}
}

// GetSpeedStats 获取速度统计.
func (q *DownloadQueue) GetSpeedStats(limit int) []SpeedStats {
	q.mu.RLock()
	defer q.mu.RUnlock()

	if limit <= 0 || limit > len(q.speedStats) {
		limit = len(q.speedStats)
	}

	start := len(q.speedStats) - limit
	if start < 0 {
		start = 0
	}

	result := make([]SpeedStats, limit)
	copy(result, q.speedStats[start:])
	return result
}

// Remove 从队列中移除任务.
func (q *DownloadQueue) Remove(taskID string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()

	// 从活跃任务中移除
	if _, ok := q.active[taskID]; ok {
		delete(q.active, taskID)
		return true
	}

	// 从队列中移除
	for i, item := range *q.items {
		if item.ID == taskID {
			heap.Remove(q.items, i)
			return true
		}
	}

	return false
}

// Stop 停止队列处理.
func (q *DownloadQueue) Stop() {
	close(q.stopCh)
}

// Stopped 检查队列是否已停止.
func (q *DownloadQueue) Stopped() bool {
	select {
	case <-q.stopCh:
		return true
	default:
		return false
	}
}

// priorityQueue 优先级队列实现（heap.Interface）.
type priorityQueue []*DownloadTask

func (pq priorityQueue) Len() int { return len(pq) }

func (pq priorityQueue) Less(i, j int) bool {
	// 优先级高的排在前面
	if pq[i].Priority != pq[j].Priority {
		return pq[i].Priority > pq[j].Priority
	}
	// 同优先级按创建时间排序（先创建的先下载）
	return pq[i].CreatedAt.Before(pq[j].CreatedAt)
}

func (pq priorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *priorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*DownloadTask))
}

func (pq *priorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil // 避免内存泄漏
	*pq = old[:n-1]
	return item
}

// SpeedLimiter 速度限制器.
type SpeedLimiter struct {
	mu           sync.Mutex
	totalLimit   int64 // 总速度限制
	perTaskLimit int64 // 单任务速度限制
	bucket       int64 // 当前可用字节数
	lastTime     time.Time
}

// NewSpeedLimiter 创建速度限制器.
func NewSpeedLimiter(totalLimit, perTaskLimit int64) *SpeedLimiter {
	return &SpeedLimiter{
		totalLimit:   totalLimit,
		perTaskLimit: perTaskLimit,
		bucket:       totalLimit,
		lastTime:     time.Now(),
	}
}

// Allow 检查是否允许下载指定字节数.
func (l *SpeedLimiter) Allow(bytes int64) bool {
	if l.totalLimit <= 0 {
		return true // 无限制
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastTime)
	l.lastTime = now

	// 补充令牌桶
	l.bucket += int64(elapsed.Seconds()) * l.totalLimit
	if l.bucket > l.totalLimit {
		l.bucket = l.totalLimit
	}

	if l.bucket >= bytes {
		l.bucket -= bytes
		return true
	}

	return false
}

// Wait 等待直到允许下载指定字节数.
func (l *SpeedLimiter) Wait(bytes int64) time.Duration {
	if l.totalLimit <= 0 {
		return 0
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	now := time.Now()
	elapsed := now.Sub(l.lastTime)
	l.lastTime = now

	// 补充令牌桶
	l.bucket += int64(elapsed.Seconds()) * l.totalLimit
	if l.bucket > l.totalLimit {
		l.bucket = l.totalLimit
	}

	if l.bucket >= bytes {
		l.bucket -= bytes
		return 0
	}

	// 计算需要等待的时间
	deficit := bytes - l.bucket
	waitTime := time.Duration(float64(deficit) / float64(l.totalLimit) * float64(time.Second))
	return waitTime
}

// UpdateLimits 更新速度限制.
func (l *SpeedLimiter) UpdateLimits(totalLimit, perTaskLimit int64) {
	l.mu.Lock()
	defer l.mu.Unlock()

	l.totalLimit = totalLimit
	l.perTaskLimit = perTaskLimit
	l.bucket = totalLimit
}

// GetPerTaskLimit 获取单任务速度限制.
func (l *SpeedLimiter) GetPerTaskLimit() int64 {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.perTaskLimit
}
