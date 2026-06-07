// Package scenesmart 智能家居场景引擎
// 对标飞牛fnOS智能家居自动化功能
package scenesmart

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

// SceneType 场景类型
type SceneType string

const (
	SceneTypeManual    SceneType = "manual"    // 手动场景
	SceneTypeAuto      SceneType = "auto"      // 自动场景
	SceneTypeScheduled SceneType = "scheduled" // 定时场景
	SceneTypeTrigger   SceneType = "trigger"   // 触发场景
)

// SceneStatus 场景状态
type SceneStatus string

const (
	SceneStatusActive   SceneStatus = "active"
	SceneStatusInactive SceneStatus = "inactive"
	SceneStatusRunning  SceneStatus = "running"
)

// Action 动作定义
type Action struct {
	DeviceID  string                 `json:"device_id"`
	Command   string                 `json:"command"`
	Params    map[string]interface{} `json:"params,omitempty"`
	Delay     time.Duration          `json:"delay,omitempty"`
	Condition *Condition             `json:"condition,omitempty"`
}

// Condition 触发条件
type Condition struct {
	Type      string      `json:"type"`     // time, device, sensor, expression
	Operator  string      `json:"operator"` // eq, neq, gt, lt, gte, lte, between
	Value     interface{} `json:"value"`
	DeviceID  string      `json:"device_id,omitempty"`
	SensorID  string      `json:"sensor_id,omitempty"`
	StartTime *time.Time  `json:"start_time,omitempty"`
	EndTime   *time.Time  `json:"end_time,omitempty"`
	Days      []int       `json:"days,omitempty"` // 0=Sunday, 1=Monday, ...
}

// Trigger 触发器
type Trigger struct {
	Type      string     `json:"type"` // manual, time, device, sensor, webhook
	Condition *Condition `json:"condition,omitempty"`
	Schedule  string     `json:"schedule,omitempty"` // cron expression
	WebhookID string     `json:"webhook_id,omitempty"`
}

// Scene 智能场景定义
type Scene struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Type        SceneType   `json:"type"`
	Status      SceneStatus `json:"status"`
	Triggers    []Trigger   `json:"triggers"`
	Actions     []Action    `json:"actions"`
	Enabled     bool        `json:"enabled"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	LastRun     *time.Time  `json:"last_run,omitempty"`
	RunCount    int64       `json:"run_count"`
	Tags        []string    `json:"tags,omitempty"`
}

// SceneExecution 场景执行记录
type SceneExecution struct {
	ID        string            `json:"id"`
	SceneID   string            `json:"scene_id"`
	Status    string            `json:"status"` // success, failed, partial
	StartedAt time.Time         `json:"started_at"`
	EndedAt   time.Time         `json:"ended_at,omitempty"`
	Error     string            `json:"error,omitempty"`
	Actions   []ActionExecution `json:"actions"`
}

// ActionExecution 动作执行记录
type ActionExecution struct {
	DeviceID  string    `json:"device_id"`
	Command   string    `json:"command"`
	Status    string    `json:"status"`
	Error     string    `json:"error,omitempty"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
}

// Manager 场景管理器
type Manager struct {
	mu         sync.RWMutex
	scenes     map[string]*Scene
	executions map[string][]*SceneExecution
	deviceMgr  DeviceManager
	sensorMgr  SensorManager
	logger     Logger
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// DeviceManager 设备管理器接口
type DeviceManager interface {
	SendCommand(ctx context.Context, deviceID, command string, params map[string]interface{}) error
	GetDeviceStatus(ctx context.Context, deviceID string) (map[string]interface{}, error)
}

// SensorManager 传感器管理器接口
type SensorManager interface {
	GetSensorValue(ctx context.Context, sensorID string) (interface{}, error)
	Subscribe(ctx context.Context, sensorID string, callback func(interface{})) error
}

// Logger 日志接口
type Logger interface {
	Info(msg string, args ...interface{})
	Error(msg string, args ...interface{})
	Debug(msg string, args ...interface{})
}

// NewManager 创建场景管理器
func NewManager(deviceMgr DeviceManager, sensorMgr SensorManager, logger Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	m := &Manager{
		scenes:     make(map[string]*Scene),
		executions: make(map[string][]*SceneExecution),
		deviceMgr:  deviceMgr,
		sensorMgr:  sensorMgr,
		logger:     logger,
		ctx:        ctx,
		cancel:     cancel,
	}

	// 启动定时场景调度器
	m.wg.Add(1)
	go m.scheduleLoop()

	return m
}

// CreateScene 创建场景
func (m *Manager) CreateScene(scene *Scene) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if scene.ID == "" {
		scene.ID = generateID()
	}
	scene.CreatedAt = time.Now()
	scene.UpdatedAt = time.Now()
	scene.Status = SceneStatusActive

	m.scenes[scene.ID] = scene
	m.logger.Info("场景创建成功: %s (%s)", scene.Name, scene.ID)
	return nil
}

// UpdateScene 更新场景
func (m *Manager) UpdateScene(scene *Scene) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.scenes[scene.ID]
	if !ok {
		return fmt.Errorf("场景不存在: %s", scene.ID)
	}

	scene.CreatedAt = existing.CreatedAt
	scene.UpdatedAt = time.Now()
	m.scenes[scene.ID] = scene
	m.logger.Info("场景更新成功: %s (%s)", scene.Name, scene.ID)
	return nil
}

// DeleteScene 删除场景
func (m *Manager) DeleteScene(sceneID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.scenes[sceneID]; !ok {
		return fmt.Errorf("场景不存在: %s", sceneID)
	}

	delete(m.scenes, sceneID)
	delete(m.executions, sceneID)
	m.logger.Info("场景删除成功: %s", sceneID)
	return nil
}

// GetScene 获取场景
func (m *Manager) GetScene(sceneID string) (*Scene, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scene, ok := m.scenes[sceneID]
	if !ok {
		return nil, fmt.Errorf("场景不存在: %s", sceneID)
	}
	return scene, nil
}

// ListScenes 列出所有场景
func (m *Manager) ListScenes() []*Scene {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scenes := make([]*Scene, 0, len(m.scenes))
	for _, scene := range m.scenes {
		scenes = append(scenes, scene)
	}
	return scenes
}

// ExecuteScene 执行场景
func (m *Manager) ExecuteScene(ctx context.Context, sceneID string) (*SceneExecution, error) {
	m.mu.RLock()
	scene, ok := m.scenes[sceneID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("场景不存在: %s", sceneID)
	}

	if !scene.Enabled {
		return nil, fmt.Errorf("场景已禁用: %s", sceneID)
	}

	execution := &SceneExecution{
		ID:        generateID(),
		SceneID:   sceneID,
		StartedAt: time.Now(),
		Actions:   make([]ActionExecution, 0, len(scene.Actions)),
	}

	// 执行所有动作
	allSuccess := true
	for _, action := range scene.Actions {
		actionExec := m.executeAction(ctx, action)
		execution.Actions = append(execution.Actions, actionExec)
		if actionExec.Status != "success" {
			allSuccess = false
		}
	}

	execution.EndedAt = time.Now()
	if allSuccess {
		execution.Status = "success"
	} else {
		execution.Status = "partial"
	}

	// 更新场景统计
	m.mu.Lock()
	scene.LastRun = &execution.EndedAt
	scene.RunCount++
	m.executions[sceneID] = append(m.executions[sceneID], execution)
	m.mu.Unlock()

	m.logger.Info("场景执行完成: %s, 状态: %s", scene.Name, execution.Status)
	return execution, nil
}

// executeAction 执行单个动作
func (m *Manager) executeAction(ctx context.Context, action Action) ActionExecution {
	exec := ActionExecution{
		DeviceID:  action.DeviceID,
		Command:   action.Command,
		StartedAt: time.Now(),
	}

	// 延迟执行
	if action.Delay > 0 {
		select {
		case <-time.After(action.Delay):
		case <-ctx.Done():
			exec.Status = "cancelled"
			exec.Error = "context cancelled"
			exec.EndedAt = time.Now()
			return exec
		}
	}

	// 检查条件
	if action.Condition != nil {
		conditionMet, err := m.evaluateCondition(ctx, action.Condition)
		if err != nil {
			exec.Status = "failed"
			exec.Error = fmt.Sprintf("条件评估失败: %v", err)
			exec.EndedAt = time.Now()
			return exec
		}
		if !conditionMet {
			exec.Status = "skipped"
			exec.Error = "条件不满足"
			exec.EndedAt = time.Now()
			return exec
		}
	}

	// 发送设备命令
	err := m.deviceMgr.SendCommand(ctx, action.DeviceID, action.Command, action.Params)
	if err != nil {
		exec.Status = "failed"
		exec.Error = err.Error()
	} else {
		exec.Status = "success"
	}
	exec.EndedAt = time.Now()
	return exec
}

// evaluateCondition 评估条件
func (m *Manager) evaluateCondition(ctx context.Context, cond *Condition) (bool, error) {
	switch cond.Type {
	case "time":
		return m.evaluateTimeCondition(cond), nil
	case "device":
		return m.evaluateDeviceCondition(ctx, cond)
	case "sensor":
		return m.evaluateSensorCondition(ctx, cond)
	default:
		return false, fmt.Errorf("未知条件类型: %s", cond.Type)
	}
}

// evaluateTimeCondition 评估时间条件
func (m *Manager) evaluateTimeCondition(cond *Condition) bool {
	now := time.Now()

	// 检查日期
	if len(cond.Days) > 0 {
		weekday := int(now.Weekday())
		found := false
		for _, day := range cond.Days {
			if day == weekday {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 检查时间范围
	if cond.StartTime != nil && cond.EndTime != nil {
		current := now.Hour()*60 + now.Minute()
		start := cond.StartTime.Hour()*60 + cond.StartTime.Minute()
		end := cond.EndTime.Hour()*60 + cond.EndTime.Minute()

		if start <= end {
			return current >= start && current <= end
		}
		// 跨午夜
		return current >= start || current <= end
	}

	return true
}

// evaluateDeviceCondition 评估设备条件
func (m *Manager) evaluateDeviceCondition(ctx context.Context, cond *Condition) (bool, error) {
	status, err := m.deviceMgr.GetDeviceStatus(ctx, cond.DeviceID)
	if err != nil {
		return false, err
	}

	value, ok := status["status"]
	if !ok {
		return false, fmt.Errorf("设备状态字段不存在")
	}

	return compareValues(value, cond.Value, cond.Operator), nil
}

// evaluateSensorCondition 评估传感器条件
func (m *Manager) evaluateSensorCondition(ctx context.Context, cond *Condition) (bool, error) {
	value, err := m.sensorMgr.GetSensorValue(ctx, cond.SensorID)
	if err != nil {
		return false, err
	}

	return compareValues(value, cond.Value, cond.Operator), nil
}

// compareValues 比较值
func compareValues(actual, expected interface{}, operator string) bool {
	switch operator {
	case "eq":
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", expected)
	case "neq":
		return fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", expected)
	default:
		return false
	}
}

// scheduleLoop 定时调度循环
func (m *Manager) scheduleLoop() {
	defer m.wg.Done()
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkScheduledScenes()
		}
	}
}

// checkScheduledScenes 检查定时场景
func (m *Manager) checkScheduledScenes() {
	m.mu.RLock()
	scenes := make([]*Scene, 0)
	for _, scene := range m.scenes {
		if scene.Type == SceneTypeScheduled && scene.Enabled {
			scenes = append(scenes, scene)
		}
	}
	m.mu.RUnlock()

	for _, scene := range scenes {
		for _, trigger := range scene.Triggers {
			if trigger.Type == "time" && trigger.Schedule != "" {
				// 检查是否到了执行时间
				if m.shouldExecuteSchedule(trigger.Schedule) {
					go m.ExecuteScene(context.Background(), scene.ID)
				}
			}
		}
	}
}

// shouldExecuteSchedule 检查是否应该执行定时任务
func (m *Manager) shouldExecuteSchedule(schedule string) bool {
	// 简化实现：每分钟检查一次
	// 实际应该解析 cron 表达式
	return true
}

// GetSceneExecutions 获取场景执行记录
func (m *Manager) GetSceneExecutions(sceneID string) []*SceneExecution {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.executions[sceneID]
}

// Stop 停止管理器
func (m *Manager) Stop() {
	m.cancel()
	m.wg.Wait()
}

func generateID() string {
	return fmt.Sprintf("scene_%d", time.Now().UnixNano())
}

// RegisterHandlers 注册 HTTP 处理器
func (m *Manager) RegisterHandlers(mux *http.ServeMux) {
	mux.HandleFunc("/api/scenes", m.handleScenes)
	mux.HandleFunc("/api/scenes/execute", m.handleExecuteScene)
	mux.HandleFunc("/api/scenes/executions", m.handleGetExecutions)
}

func (m *Manager) handleScenes(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		scenes := m.ListScenes()
		writeJSON(w, scenes)
	case http.MethodPost:
		var scene Scene
		if err := json.NewDecoder(r.Body).Decode(&scene); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if err := m.CreateScene(&scene); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, scene)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (m *Manager) handleExecuteScene(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SceneID string `json:"scene_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	execution, err := m.ExecuteScene(r.Context(), req.SceneID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, execution)
}

func (m *Manager) handleGetExecutions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sceneID := r.URL.Query().Get("scene_id")
	if sceneID == "" {
		http.Error(w, "scene_id is required", http.StatusBadRequest)
		return
	}

	executions := m.GetSceneExecutions(sceneID)
	writeJSON(w, executions)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
