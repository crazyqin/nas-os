// Package datapipeline 管道管理器核心实现
package datapipeline

import (
	"encoding/json"
	"os"
	"sync"
	"time"
)

// Manager 管道管理器
type Manager struct {
	mu         sync.RWMutex
	pipelines  map[string]*Pipeline
	executions map[string][]*Execution
	dlq        []*DLQEntry
	config     *ManagerConfig
	dataFile   string
	stopCh     map[string]chan struct{}
	wg         sync.WaitGroup // 追踪后台 goroutine
}

// NewManager 创建管理器
func NewManager(dataFile string) *Manager {
	return &Manager{
		pipelines:  make(map[string]*Pipeline),
		executions: make(map[string][]*Execution),
		dlq:        make([]*DLQEntry, 0),
		stopCh:     make(map[string]chan struct{}),
		config: &ManagerConfig{
			MaxPipelines:  100,
			MaxExecutions: 1000,
			MaxDLQSize:    5000,
			WorkerCount:   4,
		},
		dataFile: dataFile,
	}
}

// Initialize 初始化管理器
func (m *Manager) Initialize() error {
	return m.load()
}

// CreatePipeline 创建管道
func (m *Manager) CreatePipeline(p *Pipeline) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.pipelines[p.ID]; exists {
		return ErrPipelineExists
	}
	if len(m.pipelines) >= m.config.MaxPipelines {
		return ErrMaxPipelines
	}
	if !isValidSourceType(p.Source.Type) {
		return ErrInvalidSource
	}
	if !isValidScheduleType(p.Schedule.Type) {
		return ErrInvalidSchedule
	}
	for _, t := range p.Transforms {
		if !isValidTransformType(t.Type) {
			return ErrInvalidTransform
		}
	}

	p.Status = PipelineIdle
	p.CreatedAt = time.Now()
	p.UpdatedAt = time.Now()
	if p.Retry.MaxRetries == 0 {
		p.Retry = RetryPolicy{MaxRetries: 3, InitialDelay: time.Second, MaxDelay: 5 * time.Minute, BackoffFactor: 2.0}
	}
	if p.DeadLetter.MaxSize == 0 {
		p.DeadLetter = DeadLetterConfig{Enabled: true, MaxSize: 1000, Retention: "7d"}
	}
	m.pipelines[p.ID] = p
	return m.save()
}

// DeletePipeline 删除管道
func (m *Manager) DeletePipeline(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.pipelines[id]
	if !ok {
		return ErrPipelineNotFound
	}
	if p.Status == PipelineRunning {
		return ErrPipelineRunning
	}

	delete(m.pipelines, id)
	delete(m.executions, id)
	if ch, ok := m.stopCh[id]; ok {
		close(ch)
		delete(m.stopCh, id)
	}
	return m.save()
}

// GetPipeline 获取管道
func (m *Manager) GetPipeline(id string) (*Pipeline, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.pipelines[id]
	if !ok {
		return nil, ErrPipelineNotFound
	}
	return p, nil
}

// ListPipelines 列出管道
func (m *Manager) ListPipelines(status PipelineStatus, tag string) []*Pipeline {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Pipeline
	for _, p := range m.pipelines {
		if status != "" && p.Status != status {
			continue
		}
		if tag != "" && !containsTag(p.Tags, tag) {
			continue
		}
		result = append(result, p)
	}
	return result
}

// UpdatePipeline 更新管道配置
func (m *Manager) UpdatePipeline(id string, update *Pipeline) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.pipelines[id]
	if !ok {
		return ErrPipelineNotFound
	}
	if p.Status == PipelineRunning {
		return ErrPipelineRunning
	}

	if update.Name != "" {
		p.Name = update.Name
	}
	if update.Description != "" {
		p.Description = update.Description
	}
	if update.Source.Type != "" {
		p.Source = update.Source
	}
	if len(update.Transforms) > 0 {
		p.Transforms = update.Transforms
	}
	if update.Sink.Type != "" {
		p.Sink = update.Sink
	}
	if update.Schedule.Type != "" {
		p.Schedule = update.Schedule
	}
	if update.Tags != nil {
		p.Tags = update.Tags
	}
	p.UpdatedAt = time.Now()
	return m.save()
}

// StartPipeline 启动管道
func (m *Manager) StartPipeline(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.pipelines[id]
	if !ok {
		return ErrPipelineNotFound
	}
	if p.Status == PipelineRunning {
		return ErrPipelineRunning
	}

	p.Status = PipelineRunning
	p.UpdatedAt = time.Now()

	switch p.Schedule.Type {
	case ScheduleCron, ScheduleRealtime:
		stopCh := make(chan struct{})
		m.stopCh[id] = stopCh
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.runScheduled(id, stopCh)
		}()
	case ScheduleManual, ScheduleOnce:
		m.wg.Add(1)
		go func() {
			defer m.wg.Done()
			m.executePipeline(id, "manual")
		}()
	}

	return m.save()
}

// StopPipeline 停止管道
func (m *Manager) StopPipeline(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	p, ok := m.pipelines[id]
	if !ok {
		return ErrPipelineNotFound
	}
	if p.Status != PipelineRunning {
		return ErrPipelineNotRunning
	}

	p.Status = PipelinePaused
	p.UpdatedAt = time.Now()

	if ch, ok := m.stopCh[id]; ok {
		close(ch)
		delete(m.stopCh, id)
	}
	return m.save()
}

// TriggerExecution 手动触发执行
func (m *Manager) TriggerExecution(id string) (*Execution, error) {
	m.mu.RLock()
	p, ok := m.pipelines[id]
	m.mu.RUnlock()
	if !ok {
		return nil, ErrPipelineNotFound
	}
	if p.Status != PipelineRunning && p.Status != PipelineIdle {
		return nil, ErrPipelineNotRunning
	}

	exec := m.executePipeline(id, "manual")
	return exec, nil
}

// GetExecutions 获取执行历史
func (m *Manager) GetExecutions(pipelineID string, limit int) []*Execution {
	m.mu.RLock()
	defer m.mu.RUnlock()

	execs, ok := m.executions[pipelineID]
	if !ok {
		return nil
	}
	if limit <= 0 || limit > len(execs) {
		limit = len(execs)
	}
	start := len(execs) - limit
	if start < 0 {
		start = 0
	}
	return execs[start:]
}

// GetExecution 获取单个执行记录
func (m *Manager) GetExecution(pipelineID, execID string) (*Execution, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	execs, ok := m.executions[pipelineID]
	if !ok {
		return nil, ErrExecutionNotFound
	}
	for _, e := range execs {
		if e.ID == execID {
			return e, nil
		}
	}
	return nil, ErrExecutionNotFound
}

// GetDLQ 获取死信队列
func (m *Manager) GetDLQ(pipelineID string, limit int) []*DLQEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*DLQEntry
	for _, entry := range m.dlq {
		if pipelineID != "" && entry.PipelineID != pipelineID {
			continue
		}
		result = append(result, entry)
		if limit > 0 && len(result) >= limit {
			break
		}
	}
	return result
}

// RetryDLQEntry 重试死信队列条目
func (m *Manager) RetryDLQEntry(entryID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, entry := range m.dlq {
		if entry.ID == entryID {
			m.dlq = append(m.dlq[:i], m.dlq[i+1:]...)
			m.wg.Add(1)
			go func() {
				defer m.wg.Done()
				m.executePipeline(entry.PipelineID, "dlq_retry")
			}()
			return m.save()
		}
	}
	return ErrExecutionNotFound
}

// Wait 等待所有后台 goroutine 完成
func (m *Manager) Wait() {
	m.wg.Wait()
}

// ClearDLQ 清空死信队列
func (m *Manager) ClearDLQ(pipelineID string) int {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pipelineID == "" {
		count := len(m.dlq)
		m.dlq = make([]*DLQEntry, 0)
		m.save()
		return count
	}

	var kept []*DLQEntry
	removed := 0
	for _, entry := range m.dlq {
		if entry.PipelineID == pipelineID {
			removed++
		} else {
			kept = append(kept, entry)
		}
	}
	m.dlq = kept
	m.save()
	return removed
}

// GetStats 获取统计信息
func (m *Manager) GetStats() *PipelineStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &PipelineStats{
		TotalPipelines: len(m.pipelines),
		DLQCount:       len(m.dlq),
	}

	for _, p := range m.pipelines {
		if p.Status == PipelineRunning {
			stats.RunningPipelines++
		}
		stats.TotalExecutions += p.RunCount
	}

	for _, execs := range m.executions {
		for _, e := range execs {
			switch e.Status {
			case ExecSuccess:
				stats.SuccessCount++
			case ExecFailed, ExecDead:
				stats.FailedCount++
			}
		}
	}
	return stats
}

// 获取所有执行历史（用于监控面板）
func (m *Manager) ListAllExecutions(limit int) []*Execution {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var all []*Execution
	for _, execs := range m.executions {
		all = append(all, execs...)
	}

	// 按时间倒序
	for i := 0; i < len(all)-1; i++ {
		for j := i + 1; j < len(all); j++ {
			if all[j].StartTime.After(all[i].StartTime) {
				all[i], all[j] = all[j], all[i]
			}
		}
	}

	if limit > 0 && len(all) > limit {
		all = all[:limit]
	}
	return all
}

// GetPipelineHealth 获取管道健康状态
func (m *Manager) GetPipelineHealth(id string) (map[string]interface{}, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	p, ok := m.pipelines[id]
	if !ok {
		return nil, ErrPipelineNotFound
	}

	execs := m.executions[id]
	successRate := 0.0
	avgDuration := int64(0)
	if len(execs) > 0 {
		success := 0
		totalDuration := int64(0)
		for _, e := range execs {
			if e.Status == ExecSuccess {
				success++
			}
			totalDuration += e.Duration
		}
		successRate = float64(success) / float64(len(execs)) * 100
		avgDuration = totalDuration / int64(len(execs))
	}

	// 计算DLQ条目数
	dlqCount := 0
	for _, entry := range m.dlq {
		if entry.PipelineID == id {
			dlqCount++
		}
	}

	return map[string]interface{}{
		"pipeline_id":     id,
		"name":            p.Name,
		"status":          p.Status,
		"success_rate":    successRate,
		"avg_duration_ms": avgDuration,
		"total_runs":      p.RunCount,
		"dlq_entries":     dlqCount,
		"last_success":    p.LastSuccess,
		"last_error":      p.LastError,
	}, nil
}

// 内部方法

func (m *Manager) runScheduled(pipelineID string, stopCh chan struct{}) {
	m.mu.RLock()
	p, ok := m.pipelines[pipelineID]
	m.mu.RUnlock()
	if !ok {
		return
	}

	interval := parseDuration(p.Schedule.Cron)
	if interval == 0 {
		interval = 30 * time.Second
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-stopCh:
			return
		case <-ticker.C:
			m.mu.RLock()
			p, ok := m.pipelines[pipelineID]
			m.mu.RUnlock()
			if !ok || p.Status != PipelineRunning {
				return
			}
			m.executePipeline(pipelineID, "scheduled")
		}
	}
}

func (m *Manager) executePipeline(pipelineID, trigger string) *Execution {
	exec := &Execution{
		ID:         generateID(),
		PipelineID: pipelineID,
		Status:     ExecRunning,
		StartTime:  time.Now(),
		Trigger:    trigger,
	}

	m.mu.Lock()
	m.executions[pipelineID] = append(m.executions[pipelineID], exec)
	if len(m.executions[pipelineID]) > m.config.MaxExecutions {
		m.executions[pipelineID] = m.executions[pipelineID][len(m.executions[pipelineID])-m.config.MaxExecutions:]
	}
	m.mu.Unlock()

	success := m.simulateExecution(pipelineID)

	endTime := time.Now()
	exec.EndTime = &endTime
	exec.Duration = endTime.Sub(exec.StartTime).Milliseconds()

	m.mu.Lock()
	p, ok := m.pipelines[pipelineID]
	if ok {
		p.RunCount++
		p.UpdatedAt = time.Now()
	}
	m.mu.Unlock()

	if success {
		exec.Status = ExecSuccess
		exec.RecordsIn = 100
		exec.RecordsOut = 95
		m.mu.Lock()
		if p, ok := m.pipelines[pipelineID]; ok {
			p.LastSuccess = &endTime
			p.LastError = ""
		}
		m.mu.Unlock()
	} else {
		exec.Status = ExecFailed
		exec.ErrorMsg = "simulated processing error"

		m.mu.RLock()
		p, _ := m.pipelines[pipelineID]
		maxRetries := 3
		dlqEnabled := true
		if p != nil {
			maxRetries = p.Retry.MaxRetries
			dlqEnabled = p.DeadLetter.Enabled
		}
		m.mu.RUnlock()

		if exec.RetryCount < maxRetries {
			exec.RetryCount++
			exec.Status = ExecRetrying
			m.mu.Lock()
			if p, ok := m.pipelines[pipelineID]; ok {
				p.LastError = exec.ErrorMsg
			}
			m.mu.Unlock()
		} else if dlqEnabled {
			exec.Status = ExecDead
			m.addToDLQ(pipelineID, exec.ID, "", exec.ErrorMsg, exec.RetryCount)
		}
	}

	m.save()
	return exec
}

func (m *Manager) simulateExecution(pipelineID string) bool {
	hash := 0
	for _, c := range pipelineID {
		hash += int(c)
	}
	return hash%5 != 0
}

func (m *Manager) addToDLQ(pipelineID, execID, data, errMsg string, retryCount int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(m.dlq) >= m.config.MaxDLQSize {
		m.dlq = m.dlq[1:]
	}

	entry := &DLQEntry{
		ID:          generateID(),
		PipelineID:  pipelineID,
		ExecutionID: execID,
		Data:        data,
		Error:       errMsg,
		RetryCount:  retryCount,
		CreatedAt:   time.Now(),
	}
	m.dlq = append(m.dlq, entry)
}

func (m *Manager) load() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := os.ReadFile(m.dataFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var stored struct {
		Pipelines  map[string]*Pipeline    `json:"pipelines"`
		Executions map[string][]*Execution `json:"executions"`
		DLQ        []*DLQEntry             `json:"dlq"`
	}
	if err := json.Unmarshal(data, &stored); err != nil {
		return err
	}
	if stored.Pipelines != nil {
		m.pipelines = stored.Pipelines
	}
	if stored.Executions != nil {
		m.executions = stored.Executions
	}
	if stored.DLQ != nil {
		m.dlq = stored.DLQ
	}
	return nil
}

func (m *Manager) save() error {
	if m.dataFile == "" {
		return nil
	}
	data, err := json.MarshalIndent(struct {
		Pipelines  map[string]*Pipeline    `json:"pipelines"`
		Executions map[string][]*Execution `json:"executions"`
		DLQ        []*DLQEntry             `json:"dlq"`
	}{m.pipelines, m.executions, m.dlq}, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(m.dataFile, data, 0644)
}

// 验证函数

func isValidSourceType(t DataSourceType) bool {
	switch t {
	case SourceFile, SourceDB, SourceAPI, SourceS3, SourceKafka, SourceStdin:
		return true
	}
	return false
}

func isValidTransformType(t TransformType) bool {
	switch t {
	case TransformFilter, TransformMap, TransformAggregate, TransformJoin, TransformWindow, TransformFlatten, TransformDedup:
		return true
	}
	return false
}

func isValidScheduleType(t ScheduleType) bool {
	switch t {
	case ScheduleCron, ScheduleRealtime, ScheduleManual, ScheduleOnce:
		return true
	}
	return false
}

func containsTag(tags []string, tag string) bool {
	for _, t := range tags {
		if t == tag {
			return true
		}
	}
	return false
}

func parseDuration(s string) time.Duration {
	if s == "" {
		return 0
	}
	d, err := time.ParseDuration(s)
	if err != nil {
		return 0
	}
	return d
}

func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomHex(8)
}

func randomHex(n int) string {
	const hex = "0123456789abcdef"
	b := make([]byte, n)
	for i := range b {
		b[i] = hex[time.Now().UnixNano()%16]
	}
	return string(b)
}
