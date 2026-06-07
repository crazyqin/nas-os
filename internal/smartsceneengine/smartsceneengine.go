// Package smartsceneengine 实现智能场景自动化引擎。
// 支持事件驱动的自动化规则，条件触发，多动作执行，场景联动。
package smartsceneengine

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// EventType 事件类型
type EventType string

const (
	EventFileCreated    EventType = "file.created"
	EventFileModified   EventType = "file.modified"
	EventFileDeleted    EventType = "file.deleted"
	EventDiskLow        EventType = "disk.low"
	EventDiskFull       EventType = "disk.full"
	EventCPUHigh        EventType = "cpu.high"
	EventMemoryHigh     EventType = "memory.high"
	EventServiceDown    EventType = "service.down"
	EventBackupComplete EventType = "backup.complete"
	EventBackupFailed   EventType = "backup.failed"
	EventLoginSuccess   EventType = "login.success"
	EventLoginFailed    EventType = "login.failed"
	EventNetworkChange  EventType = "network.change"
	EventUPSOnBattery   EventType = "ups.battery"
	EventScheduled      EventType = "scheduled"
	EventCustom         EventType = "custom"
)

// ConditionOperator 条件操作符
type ConditionOperator string

const (
	OpEquals      ConditionOperator = "eq"
	OpNotEquals   ConditionOperator = "neq"
	OpContains    ConditionOperator = "contains"
	OpGreaterThan ConditionOperator = "gt"
	OpLessThan    ConditionOperator = "lt"
	OpIn          ConditionOperator = "in"
	OpRegex       ConditionOperator = "regex"
)

// ActionType 动作类型
type ActionType string

const (
	ActionNotify     ActionType = "notify"     // 发送通知
	ActionWebhook    ActionType = "webhook"    // 调用webhook
	ActionScript     ActionType = "script"     // 执行脚本
	ActionBackup     ActionType = "backup"     // 触发备份
	ActionCleanup    ActionType = "cleanup"    // 清理文件
	ActionRestart    ActionType = "restart"    // 重启服务
	ActionSnapshot   ActionType = "snapshot"   // 创建快照
	ActionRecordLog  ActionType = "log"        // 记录日志
	ActionThrottle   ActionType = "throttle"   // 限流
	ActionQuarantine ActionType = "quarantine" // 隔离文件
	ActionAlert      ActionType = "alert"      // 升级告警
	ActionDelay      ActionType = "delay"      // 延迟执行
)

// Condition 条件
type Condition struct {
	Field    string            `json:"field"`    // 事件字段
	Operator ConditionOperator `json:"operator"` // 操作符
	Value    interface{}       `json:"value"`    // 比较值
}

// Action 动作
type Action struct {
	Type       ActionType             `json:"type"`
	Params     map[string]interface{} `json:"params"`
	DelayMs    int                    `json:"delayMs,omitempty"`    // 延迟执行
	Timeout    int                    `json:"timeout,omitempty"`    // 超时（秒）
	RetryCount int                    `json:"retryCount,omitempty"` // 重试次数
}

// Trigger 触发器
type Trigger struct {
	Type     EventType         `json:"type"`               // 事件类型
	Schedule string            `json:"schedule,omitempty"` // cron表达式（定时触发）
	Filters  map[string]string `json:"filters,omitempty"`  // 事件过滤
}

// Scene 场景定义
type Scene struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Enabled     bool        `json:"enabled"`
	Priority    int         `json:"priority"` // 优先级 (1-10)
	Trigger     Trigger     `json:"trigger"`
	Conditions  []Condition `json:"conditions"`
	Actions     []Action    `json:"actions"`
	CooldownSec int         `json:"cooldownSec"` // 冷却时间（秒）
	Tags        []string    `json:"tags"`
	CreatedAt   time.Time   `json:"createdAt"`
	UpdatedAt   time.Time   `json:"updatedAt"`
	LastRunAt   time.Time   `json:"lastRunAt,omitempty"`
	RunCount    int         `json:"runCount"`
	LastError   string      `json:"lastError,omitempty"`
}

// SceneEvent 场景事件
type SceneEvent struct {
	ID        string                 `json:"id"`
	Type      EventType              `json:"type"`
	Source    string                 `json:"source"`
	Data      map[string]interface{} `json:"data"`
	Timestamp time.Time              `json:"timestamp"`
}

// ExecutionLog 执行日志
type ExecutionLog struct {
	ID        string      `json:"id"`
	SceneID   string      `json:"sceneId"`
	SceneName string      `json:"sceneName"`
	EventID   string      `json:"eventId"`
	Triggered bool        `json:"triggered"`
	Actions   []ActionLog `json:"actions"`
	StartedAt time.Time   `json:"startedAt"`
	Duration  int64       `json:"durationMs"`
	Error     string      `json:"error,omitempty"`
}

// ActionLog 动作执行日志
type ActionLog struct {
	Type     ActionType `json:"type"`
	Status   string     `json:"status"`
	Duration int64      `json:"durationMs"`
	Error    string     `json:"error,omitempty"`
}

// Engine 场景引擎
type Engine struct {
	mu        sync.RWMutex
	scenes    map[string]*Scene
	logs      []ExecutionLog
	lastRun   map[string]time.Time // sceneID -> last run time
	eventChan chan SceneEvent
	quit      chan struct{}
	running   bool
}

// NewEngine 创建场景引擎
func NewEngine() *Engine {
	return &Engine{
		scenes:    make(map[string]*Scene),
		logs:      make([]ExecutionLog, 0),
		lastRun:   make(map[string]time.Time),
		eventChan: make(chan SceneEvent, 1000),
		quit:      make(chan struct{}),
	}
}

// AddScene 添加场景
func (e *Engine) AddScene(scene Scene) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if scene.ID == "" {
		return fmt.Errorf("场景ID不能为空")
	}
	scene.CreatedAt = time.Now()
	scene.UpdatedAt = time.Now()
	scene.Enabled = true
	e.scenes[scene.ID] = &scene
	return nil
}

// UpdateScene 更新场景
func (e *Engine) UpdateScene(scene Scene) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	existing, ok := e.scenes[scene.ID]
	if !ok {
		return fmt.Errorf("场景 %s 不存在", scene.ID)
	}
	scene.CreatedAt = existing.CreatedAt
	scene.UpdatedAt = time.Now()
	scene.RunCount = existing.RunCount
	scene.LastRunAt = existing.LastRunAt
	e.scenes[scene.ID] = &scene
	return nil
}

// RemoveScene 删除场景
func (e *Engine) RemoveScene(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, ok := e.scenes[id]; !ok {
		return fmt.Errorf("场景 %s 不存在", id)
	}
	delete(e.scenes, id)
	return nil
}

// EnableScene 启用场景
func (e *Engine) EnableScene(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	scene, ok := e.scenes[id]
	if !ok {
		return fmt.Errorf("场景 %s 不存在", id)
	}
	scene.Enabled = true
	return nil
}

// DisableScene 禁用场景
func (e *Engine) DisableScene(id string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	scene, ok := e.scenes[id]
	if !ok {
		return fmt.Errorf("场景 %s 不存在", id)
	}
	scene.Enabled = false
	return nil
}

// GetScene 获取场景
func (e *Engine) GetScene(id string) (*Scene, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	scene, ok := e.scenes[id]
	if !ok {
		return nil, fmt.Errorf("场景 %s 不存在", id)
	}
	return scene, nil
}

// ListScenes 列出所有场景
func (e *Engine) ListScenes(enabledOnly bool) []Scene {
	e.mu.RLock()
	defer e.mu.RUnlock()

	scenes := make([]Scene, 0, len(e.scenes))
	for _, s := range e.scenes {
		if !enabledOnly || s.Enabled {
			scenes = append(scenes, *s)
		}
	}
	return scenes
}

// EmitEvent 发送事件
func (e *Engine) EmitEvent(event SceneEvent) {
	if event.ID == "" {
		event.ID = fmt.Sprintf("evt_%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	select {
	case e.eventChan <- event:
	default:
		// 通道满，丢弃事件
	}
}

// Start 启动引擎
func (e *Engine) Start() {
	e.mu.Lock()
	if e.running {
		e.mu.Unlock()
		return
	}
	e.running = true
	e.mu.Unlock()

	go e.eventLoop()
}

// Stop 停止引擎
func (e *Engine) Stop() {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.running {
		return
	}
	e.running = false
	close(e.quit)
}

// eventLoop 事件处理循环
func (e *Engine) eventLoop() {
	for {
		select {
		case event := <-e.eventChan:
			e.processEvent(event)
		case <-e.quit:
			return
		}
	}
}

// processEvent 处理事件
func (e *Engine) processEvent(event SceneEvent) {
	e.mu.RLock()
	scenes := make([]*Scene, 0)
	for _, s := range e.scenes {
		if s.Enabled && s.Trigger.Type == event.Type {
			scenes = append(scenes, s)
		}
	}
	e.mu.RUnlock()

	for _, scene := range scenes {
		// 检查冷却时间
		e.mu.RLock()
		lastRun, exists := e.lastRun[scene.ID]
		e.mu.RUnlock()

		if exists && time.Since(lastRun) < time.Duration(scene.CooldownSec)*time.Second {
			continue
		}

		// 检查条件
		if !e.checkConditions(scene.Conditions, event.Data) {
			continue
		}

		// 执行动作
		go e.executeScene(scene, event)
	}
}

// checkConditions 检查条件
func (e *Engine) checkConditions(conditions []Condition, data map[string]interface{}) bool {
	for _, cond := range conditions {
		val, ok := data[cond.Field]
		if !ok {
			return false
		}
		if !e.evaluateCondition(cond, val) {
			return false
		}
	}
	return true
}

// evaluateCondition 评估单个条件
func (e *Engine) evaluateCondition(cond Condition, actual interface{}) bool {
	switch cond.Operator {
	case OpEquals:
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", cond.Value)
	case OpNotEquals:
		return fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", cond.Value)
	case OpContains:
		actualStr := fmt.Sprintf("%v", actual)
		valueStr := fmt.Sprintf("%v", cond.Value)
		return contains(actualStr, valueStr)
	case OpGreaterThan:
		return toFloat64(actual) > toFloat64(cond.Value)
	case OpLessThan:
		return toFloat64(actual) < toFloat64(cond.Value)
	default:
		return false
	}
}

// executeScene 执行场景
func (e *Engine) executeScene(scene *Scene, event SceneEvent) {
	start := time.Now()
	log := ExecutionLog{
		ID:        fmt.Sprintf("log_%d", start.UnixNano()),
		SceneID:   scene.ID,
		SceneName: scene.Name,
		EventID:   event.ID,
		StartedAt: start,
		Actions:   make([]ActionLog, 0),
	}

	for _, action := range scene.Actions {
		actionStart := time.Now()
		actionLog := ActionLog{
			Type: action.Type,
		}

		// 延迟执行
		if action.DelayMs > 0 {
			time.Sleep(time.Duration(action.DelayMs) * time.Millisecond)
		}

		// 执行动作（简化版，实际应根据Type分发）
		actionLog.Status = "executed"
		actionLog.Duration = time.Since(actionStart).Milliseconds()
		log.Actions = append(log.Actions, actionLog)
	}

	log.Duration = time.Since(start).Milliseconds()
	log.Triggered = true

	// 更新场景状态
	e.mu.Lock()
	scene.RunCount++
	scene.LastRunAt = time.Now()
	scene.LastError = ""
	e.lastRun[scene.ID] = time.Now()
	e.logs = append(e.logs, log)
	// 保留最近1000条日志
	if len(e.logs) > 1000 {
		e.logs = e.logs[len(e.logs)-1000:]
	}
	e.mu.Unlock()
}

// GetLogs 获取执行日志
func (e *Engine) GetLogs(sceneID string, limit int) []ExecutionLog {
	e.mu.RLock()
	defer e.mu.RUnlock()

	logs := make([]ExecutionLog, 0)
	for i := len(e.logs) - 1; i >= 0 && len(logs) < limit; i-- {
		if sceneID == "" || e.logs[i].SceneID == sceneID {
			logs = append(logs, e.logs[i])
		}
	}
	return logs
}

// GetStats 获取引擎统计
type EngineStats struct {
	TotalScenes  int `json:"totalScenes"`
	ActiveScenes int `json:"activeScenes"`
	TotalEvents  int `json:"totalEvents"`
	TotalRuns    int `json:"totalRuns"`
}

func (e *Engine) GetStats() EngineStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := EngineStats{
		TotalScenes: len(e.scenes),
	}
	for _, s := range e.scenes {
		if s.Enabled {
			stats.ActiveScenes++
		}
		stats.TotalRuns += s.RunCount
	}
	stats.TotalEvents = len(e.logs)
	return stats
}

// RegisterRoutes 注册 HTTP 路由
func (e *Engine) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/scenes", e.handleScenes)
	mux.HandleFunc("/api/v1/scenes/logs", e.handleLogs)
	mux.HandleFunc("/api/v1/scenes/stats", e.handleStats)
	mux.HandleFunc("/api/v1/scenes/emit", e.handleEmit)
}

func (e *Engine) handleScenes(w http.ResponseWriter, r *http.Request) {
	scenes := e.ListScenes(false)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(scenes)
}

func (e *Engine) handleLogs(w http.ResponseWriter, r *http.Request) {
	sceneID := r.URL.Query().Get("sceneId")
	logs := e.GetLogs(sceneID, 50)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(logs)
}

func (e *Engine) handleStats(w http.ResponseWriter, r *http.Request) {
	stats := e.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

func (e *Engine) handleEmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var event SceneEvent
	if err := json.NewDecoder(r.Body).Decode(&event); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	e.EmitEvent(event)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "emitted"})
}

func contains(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(s) > 0 && len(sub) > 0 && containsStr(s, sub))
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	default:
		return 0
	}
}
