// Package storage 提供硬盘按需唤醒触发器
// 实现飞牛fnOS风格的智能唤醒机制：访问请求触发、批量唤醒、延迟策略
package storage

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

// WakeTriggerConfig 唤醒触发器配置
type WakeTriggerConfig struct {
	// 唤醒延迟策略
	StaggerDelayMs int `json:"stagger_delay_ms"` // 磁盘间唤醒间隔(ms)，避免电源浪涌

	// 最大并发唤醒数
	MaxConcurrentWakeups int `json:"max_concurrent_wakeups"`

	// 批量唤醒窗口
	BatchWindowMs int `json:"batch_window_ms"` // 等待更多请求的时间窗口

	// 预唤醒策略
	EnablePreWake bool `json:"enable_pre_wake"` // 根据访问模式预唤醒

	// 预唤醒阈值（访问频率）
	PreWakeAccessThreshold int `json:"pre_wake_access_threshold"` // 每小时访问次数

	// 唤醒超时
	WakeTimeoutMs int `json:"wake_timeout_ms"`

	// 唤醒失败重试次数
	WakeRetryCount int `json:"wake_retry_count"`

	// 唤醒失败重试间隔
	WakeRetryDelayMs int `json:"wake_retry_delay_ms"`
}

// DefaultWakeTriggerConfig 默认唤醒配置
func DefaultWakeTriggerConfig() *WakeTriggerConfig {
	return &WakeTriggerConfig{
		StaggerDelayMs:        500,   // 500ms间隔
		MaxConcurrentWakeups:  3,     // 最多3个同时唤醒
		BatchWindowMs:         2000,  // 2秒批量窗口
		EnablePreWake:         true,  // 启用预唤醒
		PreWakeAccessThreshold: 5,    // 每小时5次访问
		WakeTimeoutMs:         30000, // 30秒超时
		WakeRetryCount:        3,     // 重试3次
		WakeRetryDelayMs:      1000,  // 1秒重试间隔
	}
}

// WakeRequestType 唤醒请求类型
type WakeRequestType string

const (
	WakeRequestImmediate WakeRequestType = "immediate" // 立即唤醒（用户访问）
	WakeRequestBatched   WakeRequestType = "batched"   // 批量唤醒（系统调度）
	WakeRequestPreWake   WakeRequestType = "pre_wake"  // 预唤醒（智能预测）
	WakeRequestScheduled WakeRequestType = "scheduled" // 定时唤醒（维护任务）
)

// WakeRequest 唤醒请求
type WakeRequest struct {
	ID          string          `json:"id"`
	DiskPath    string          `json:"disk_path"`
	Type        WakeRequestType `json:"type"`
	Priority    WakePriority    `json:"priority"`
	Reason      string          `json:"reason"`
	CreatedAt   time.Time       `json:"created_at"`
	TriggeredBy string          `json:"triggered_by"` // 触发来源（API/服务/用户）
	Deadline    time.Time       `json:"deadline"`     // 截止时间（超时）
	index       int             // 优先级队列内部索引
}

// WakeResult 唤醒结果
type WakeResult struct {
	RequestID   string        `json:"request_id"`
	DiskPath    string        `json:"disk_path"`
	Success     bool          `json:"success"`
	DurationMs  int           `json:"duration_ms"`
	Error       string        `json:"error,omitempty"`
	RetryCount  int           `json:"retry_count"`
	WokeAt      time.Time      `json:"woke_at"`
	FromState   DiskPowerState `json:"from_state"`
}

// DiskWakeTrigger 磁盘唤醒触发器
type DiskWakeTrigger struct {
	config     *WakeTriggerConfig
	powerMgr   *DiskPowerManager
	priorityQ  *WakePriorityQueue

	// 请求管理
	pendingRequests map[string]*WakeRequest
	resultsHistory  []WakeResult
	activeWakeups   int32 // 当前活跃唤醒数

	// 批量唤醒窗口
	batchWindowTimer *time.Timer
	batchRequests    []*WakeRequest

	// 访问模式追踪（用于预唤醒）
	accessPatterns map[string]*DiskAccessPattern

	// 回调
	onWakeSuccess func(result WakeResult)
	onWakeFailure func(result WakeResult)

	mu    sync.RWMutex
	ctx   context.Context
	cancel context.CancelFunc
}

// DiskAccessPattern 磁盘访问模式
type DiskAccessPattern struct {
	DiskPath       string    `json:"disk_path"`
	AccessCount    int       `json:"access_count"`
	LastAccess     time.Time  `json:"last_access"`
	HourlyAccess   map[int]int `json:"hourly_access"` // 每小时访问次数 (0-23)
	PeakHours      []int     `json:"peak_hours"`     // 高峰时段
	AvgAccessHour  float64   `json:"avg_access_hour"` // 平均每小时访问
	UpdatedAt      time.Time  `json:"updated_at"`
}

// NewDiskWakeTrigger 创建磁盘唤醒触发器
func NewDiskWakeTrigger(ctx context.Context, config *WakeTriggerConfig, powerMgr *DiskPowerManager) *DiskWakeTrigger {
	if config == nil {
		config = DefaultWakeTriggerConfig()
	}

	childCtx, cancel := context.WithCancel(ctx)

	trigger := &DiskWakeTrigger{
		config:          config,
		powerMgr:        powerMgr,
		priorityQ:       NewWakePriorityQueue(),
		pendingRequests: make(map[string]*WakeRequest),
		resultsHistory:  make([]WakeResult, 0),
		accessPatterns:  make(map[string]*DiskAccessPattern),
		batchRequests:   make([]*WakeRequest, 0),
		ctx:             childCtx,
		cancel:          cancel,
	}

	return trigger
}

// Start 启动唤醒触发器
func (t *DiskWakeTrigger) Start() {
	go t.processQueue()
	go t.preWakeMonitor()
	go t.batchWindowProcessor()
}

// Stop 停止触发器
func (t *DiskWakeTrigger) Stop() {
	t.cancel()
	if t.batchWindowTimer != nil {
		t.batchWindowTimer.Stop()
	}
}

// RequestWakeUp 请求唤醒磁盘
func (t *DiskWakeTrigger) RequestWakeUp(req *WakeRequest) (*WakeResult, error) {
	if req.DiskPath == "" {
		return nil, fmt.Errorf("disk path is required")
	}

	// 检查磁盘是否已激活
	state, err := t.powerMgr.GetState(req.DiskPath)
	if err != nil {
		return nil, fmt.Errorf("disk not registered: %s", req.DiskPath)
	}

	if state == DiskPowerActive {
		// 已激活，无需唤醒
		return &WakeResult{
			RequestID: req.ID,
			DiskPath:  req.DiskPath,
			Success:   true,
			DurationMs: 0,
			WokeAt:    time.Now(),
			FromState: state,
		}, nil
	}

	// 设置默认值
	if req.ID == "" {
		req.ID = fmt.Sprintf("wake-%s-%d", req.DiskPath, time.Now().UnixNano())
	}
	if req.CreatedAt.IsZero() {
		req.CreatedAt = time.Now()
	}
	if req.Deadline.IsZero() {
		req.Deadline = time.Now().Add(time.Duration(t.config.WakeTimeoutMs) * time.Millisecond)
	}
	if req.Priority == 0 {
		req.Priority = WakePriorityNormal
	}

	// 立即请求直接处理
	if req.Type == WakeRequestImmediate {
		return t.executeWakeUp(req)
	}

	// 其他请求加入队列或批量窗口
	t.mu.Lock()

	if req.Type == WakeRequestBatched && t.config.BatchWindowMs > 0 {
		// 加入批量窗口
		t.batchRequests = append(t.batchRequests, req)
		t.scheduleBatchWindow()
	} else {
		// 加入优先级队列
		t.priorityQ.AddRequest(req)
		t.pendingRequests[req.ID] = req
	}

	t.mu.Unlock()

	// 等待结果（异步）
	return t.waitForResult(req.ID)
}

// executeWakeUp 执行唤醒操作
func (t *DiskWakeTrigger) executeWakeUp(req *WakeRequest) (*WakeResult, error) {
	startTime := time.Now()
	result := &WakeResult{
		RequestID: req.ID,
		DiskPath:  req.DiskPath,
		FromState: DiskPowerSleeping,
	}

	// 获取当前状态
	state, _ := t.powerMgr.GetState(req.DiskPath)
	result.FromState = state

	// 检查并发限制
	for {
		current := atomic.LoadInt32(&t.activeWakeups)
		if current < int32(t.config.MaxConcurrentWakeups) {
			atomic.AddInt32(&t.activeWakeups, 1)
			break
		}
		// 等待其他唤醒完成
		time.Sleep(time.Duration(t.config.StaggerDelayMs) * time.Millisecond)
	}

	defer atomic.AddInt32(&t.activeWakeups, -1)

	// 执行唤醒（带重试）
	var wakeErr error
	for retry := 0; retry <= t.config.WakeRetryCount; retry++ {
		result.RetryCount = retry

		// 记录访问
		t.recordAccess(req.DiskPath)

		// 调用电源管理器唤醒
		wakeErr = t.powerMgr.WakeUpDisk(req.DiskPath)
		if wakeErr == nil {
			result.Success = true
			break
		}

		// 重试延迟
		if retry < t.config.WakeRetryCount {
			time.Sleep(time.Duration(t.config.WakeRetryDelayMs) * time.Millisecond)
		}
	}

	result.DurationMs = int(time.Since(startTime).Milliseconds())
	result.WokeAt = time.Now()

	if wakeErr != nil {
		result.Success = false
		result.Error = wakeErr.Error()
	}

	// 记录历史
	t.mu.Lock()
	t.resultsHistory = append(t.resultsHistory, *result)
	if len(t.resultsHistory) > 1000 {
		t.resultsHistory = t.resultsHistory[len(t.resultsHistory)-1000:]
	}
	t.mu.Unlock()

	// 回调
	if result.Success && t.onWakeSuccess != nil {
		t.onWakeSuccess(*result)
	} else if !result.Success && t.onWakeFailure != nil {
		t.onWakeFailure(*result)
	}

	return result, wakeErr
}

// scheduleBatchWindow 调度批量唤醒窗口
func (t *DiskWakeTrigger) scheduleBatchWindow() {
	if t.batchWindowTimer != nil {
		t.batchWindowTimer.Stop()
	}
	t.batchWindowTimer = time.AfterFunc(
		time.Duration(t.config.BatchWindowMs)*time.Millisecond,
		func() {
			t.processBatchRequests()
		},
	)
}

// processBatchRequests 处理批量唤醒请求
func (t *DiskWakeTrigger) processBatchRequests() {
	t.mu.Lock()
	requests := t.batchRequests
	t.batchRequests = make([]*WakeRequest, 0)
	t.mu.Unlock()

	if len(requests) == 0 {
		return
	}

	// 按磁盘分组，避免重复唤醒
	diskSet := make(map[string]bool)
	for _, req := range requests {
		diskSet[req.DiskPath] = true
	}

	// 带延迟的批量唤醒
	delay := 0
	for diskPath := range diskSet {
		// 创建批量唤醒请求
		batchReq := &WakeRequest{
			ID:        fmt.Sprintf("batch-%s-%d", diskPath, time.Now().UnixNano()),
			DiskPath:  diskPath,
			Type:      WakeRequestBatched,
			Priority:  WakePriorityNormal,
			Reason:    "batch_window",
			CreatedAt: time.Now(),
		}

		// 延迟唤醒
		go func(delayMs int) {
			if delayMs > 0 {
				time.Sleep(time.Duration(delayMs) * time.Millisecond)
			}
			t.executeWakeUp(batchReq)
		}(delay)

		delay += t.config.StaggerDelayMs
	}
}

// processQueue 处理优先级队列
func (t *DiskWakeTrigger) processQueue() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			req := t.priorityQ.PopRequest()
			if req != nil {
				t.executeWakeUp(req)
				t.mu.Lock()
				delete(t.pendingRequests, req.ID)
				t.mu.Unlock()
			}
		}
	}
}

// preWakeMonitor 预唤醒监控
func (t *DiskWakeTrigger) preWakeMonitor() {
	if !t.config.EnablePreWake {
		return
	}

	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-t.ctx.Done():
			return
		case <-ticker.C:
			t.checkPreWakeCandidates()
		}
	}
}

// checkPreWakeCandidates 检查预唤醒候选
func (t *DiskWakeTrigger) checkPreWakeCandidates() {
	t.mu.RLock()
	defer t.mu.RUnlock()

	now := time.Now()
	currentHour := now.Hour()

	for _, pattern := range t.accessPatterns {
		// 检查是否是高峰时段
		isPeakHour := false
		for _, h := range pattern.PeakHours {
			if h == currentHour {
				isPeakHour = true
				break
			}
		}

		// 高峰时段且磁盘休眠，预唤醒
		if isPeakHour && pattern.AvgAccessHour >= float64(t.config.PreWakeAccessThreshold) {
			state, _ := t.powerMgr.GetState(pattern.DiskPath)
			if state == DiskPowerSleeping || state == DiskPowerStandby {
				// 创建预唤醒请求
				preWakeReq := &WakeRequest{
					ID:        fmt.Sprintf("prewake-%s-%d", pattern.DiskPath, now.UnixNano()),
					DiskPath:  pattern.DiskPath,
					Type:      WakeRequestPreWake,
					Priority:  WakePriorityLow,
					Reason:    "peak_hour_prediction",
					CreatedAt: now,
					TriggeredBy: "prewake_monitor",
				}

				go t.executeWakeUp(preWakeReq)
			}
		}
	}
}

// batchWindowProcessor 批量窗口处理器
func (t *DiskWakeTrigger) batchWindowProcessor() {
	// 已经通过 scheduleBatchWindow 处理
}

// recordAccess 记录访问模式
func (t *DiskWakeTrigger) recordAccess(diskPath string) {
	t.mu.Lock()
	defer t.mu.Unlock()

	now := time.Now()
	hour := now.Hour()

	pattern, exists := t.accessPatterns[diskPath]
	if !exists {
		pattern = &DiskAccessPattern{
			DiskPath:     diskPath,
			HourlyAccess: make(map[int]int),
		}
		t.accessPatterns[diskPath] = pattern
	} else if pattern.HourlyAccess == nil {
		// 确保 map 已初始化
		pattern.HourlyAccess = make(map[int]int)
	}

	pattern.AccessCount++
	pattern.LastAccess = now
	pattern.HourlyAccess[hour]++
	pattern.UpdatedAt = now

	// 计算平均每小时访问
	totalHours := len(pattern.HourlyAccess)
	if totalHours > 0 {
		total := 0
		for _, count := range pattern.HourlyAccess {
			total += count
		}
		pattern.AvgAccessHour = float64(total) / float64(totalHours)
	}

	// 更新高峰时段（访问次数最多的3小时）
	pattern.PeakHours = t.calculatePeakHours(pattern.HourlyAccess)
}

// calculatePeakHours 计算高峰时段
func (t *DiskWakeTrigger) calculatePeakHours(hourlyAccess map[int]int) []int {
	// 找出访问最多的时段
	type hourCount struct {
		hour  int
		count int
	}

	hours := make([]hourCount, 0, len(hourlyAccess))
	for h, c := range hourlyAccess {
		hours = append(hours, hourCount{hour: h, count: c})
	}

	// 排序取前3
	// 简单实现
	peakHours := make([]int, 0, 3)
	maxCount := 0
	maxHour := -1
	for _, hc := range hours {
		if hc.count > maxCount {
			maxCount = hc.count
			maxHour = hc.hour
		}
	}
	if maxHour >= 0 {
		peakHours = append(peakHours, maxHour)
	}

	return peakHours
}

// waitForResult 等待唤醒结果
func (t *DiskWakeTrigger) waitForResult(requestID string) (*WakeResult, error) {
	timeout := time.Duration(t.config.WakeTimeoutMs) * time.Millisecond
	start := time.Now()

	for {
		t.mu.RLock()
		for _, result := range t.resultsHistory {
			if result.RequestID == requestID {
				t.mu.RUnlock()
				if result.Success {
					return &result, nil
				}
				return &result, errors.New(result.Error)
			}
		}
		t.mu.RUnlock()

		if time.Since(start) > timeout {
			return nil, fmt.Errorf("wake timeout for request %s", requestID)
		}

		time.Sleep(50 * time.Millisecond)
	}
}

// GetPendingRequests 获取待处理请求
func (t *DiskWakeTrigger) GetPendingRequests() []*WakeRequest {
	t.mu.RLock()
	defer t.mu.RUnlock()

	requests := make([]*WakeRequest, 0, len(t.pendingRequests))
	for _, req := range t.pendingRequests {
		requests = append(requests, req)
	}
	return requests
}

// GetAccessPatterns 获取访问模式
func (t *DiskWakeTrigger) GetAccessPatterns() map[string]*DiskAccessPattern {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.accessPatterns
}

// GetWakeHistory 获取唤醒历史
func (t *DiskWakeTrigger) GetWakeHistory(limit int) []WakeResult {
	t.mu.RLock()
	defer t.mu.RUnlock()

	if limit <= 0 || limit > len(t.resultsHistory) {
		limit = len(t.resultsHistory)
	}

	start := len(t.resultsHistory) - limit
	if start < 0 {
		start = 0
	}

	return t.resultsHistory[start:]
}

// GetStats 获取唤醒统计
func (t *DiskWakeTrigger) GetStats() *WakeTriggerStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := &WakeTriggerStats{
		PendingRequests: len(t.pendingRequests),
		QueueLength:     t.priorityQ.Len(),
		ActiveWakeups:   int(atomic.LoadInt32(&t.activeWakeups)),
		TotalWakeups:    len(t.resultsHistory),
	}

	successCount := 0
	failureCount := 0
	totalDuration := 0
	for _, result := range t.resultsHistory {
		if result.Success {
			successCount++
		} else {
			failureCount++
		}
		totalDuration += result.DurationMs
	}

	stats.SuccessCount = successCount
	stats.FailureCount = failureCount
	if len(t.resultsHistory) > 0 {
		stats.AvgDurationMs = totalDuration / len(t.resultsHistory)
	}

	return stats
}

// WakeTriggerStats 唤醒统计
type WakeTriggerStats struct {
	PendingRequests int `json:"pending_requests"`
	QueueLength     int `json:"queue_length"`
	ActiveWakeups   int `json:"active_wakeups"`
	TotalWakeups    int `json:"total_wakeups"`
	SuccessCount    int `json:"success_count"`
	FailureCount    int `json:"failure_count"`
	AvgDurationMs   int `json:"avg_duration_ms"`
}

// SetCallbacks 设置回调
func (t *DiskWakeTrigger) SetCallbacks(
	onSuccess func(result WakeResult),
	onFailure func(result WakeResult),
) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.onWakeSuccess = onSuccess
	t.onWakeFailure = onFailure
}

// CancelRequest 取消唤醒请求
func (t *DiskWakeTrigger) CancelRequest(requestID string) bool {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, exists := t.pendingRequests[requestID]; exists {
		delete(t.pendingRequests, requestID)
		t.priorityQ.RemoveByID(requestID)
		return true
	}

	// 检查批量请求
	for i, req := range t.batchRequests {
		if req.ID == requestID {
			t.batchRequests = append(t.batchRequests[:i], t.batchRequests[i+1:]...)
			return true
		}
	}

	return false
}

// UpdateConfig 更新配置
func (t *DiskWakeTrigger) UpdateConfig(config *WakeTriggerConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}
	t.mu.Lock()
	defer t.mu.Unlock()
	t.config = config
	return nil
}

// GetConfig 获取配置
func (t *DiskWakeTrigger) GetConfig() *WakeTriggerConfig {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.config
}