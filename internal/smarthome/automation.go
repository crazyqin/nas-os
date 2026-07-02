package smarthome

import (
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
)

// ============================================================
// 自动化错误
// ============================================================

var (
	ErrSceneNotFound  = errors.New("scene not found")
	ErrSceneExists    = errors.New("scene already exists")
	ErrTaskNotFound   = errors.New("task not found")
	ErrTaskExists     = errors.New("task already exists")
	ErrSceneDisabled  = errors.New("scene is disabled")
	ErrInvalidTrigger = errors.New("invalid trigger configuration")
	ErrInvalidAction  = errors.New("invalid action configuration")
)

// ============================================================
// 场景管理
// ============================================================

// AddScene 添加自动化场景.
func (m *Manager) AddScene(scene *Scene) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if scene.ID == "" {
		scene.ID = uuid.New().String()
	}

	if _, exists := m.scenes[scene.ID]; exists {
		return ErrSceneExists
	}

	// 验证触发器
	if err := validateTrigger(&scene.Trigger); err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidTrigger, err)
	}

	// 验证动作
	for i, action := range scene.Actions {
		if err := validateAction(&action); err != nil {
			return fmt.Errorf("action[%d]: %w: %v", i, ErrInvalidAction, err)
		}
	}

	now := time.Now()
	scene.CreatedAt = now
	scene.UpdatedAt = now
	if !scene.Enabled {
		scene.Enabled = true // 默认启用
	}

	m.scenes[scene.ID] = scene
	return nil
}

// GetScene 获取场景.
func (m *Manager) GetScene(id string) (*Scene, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scene, ok := m.scenes[id]
	if !ok {
		return nil, ErrSceneNotFound
	}
	return scene, nil
}

// ListScenes 列出所有场景.
func (m *Manager) ListScenes() []*Scene {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scenes := make([]*Scene, 0, len(m.scenes))
	for _, s := range m.scenes {
		scenes = append(scenes, s)
	}
	return scenes
}

// UpdateScene 更新场景.
func (m *Manager) UpdateScene(id string, update *Scene) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	scene, ok := m.scenes[id]
	if !ok {
		return ErrSceneNotFound
	}

	if update.Name != "" {
		scene.Name = update.Name
	}
	if update.Description != "" {
		scene.Description = update.Description
	}

	// 更新触发器
	if update.Trigger.Type != "" {
		if err := validateTrigger(&update.Trigger); err != nil {
			return fmt.Errorf("%w: %v", ErrInvalidTrigger, err)
		}
		scene.Trigger = update.Trigger
	}

	// 更新条件
	if update.Conditions != nil {
		scene.Conditions = update.Conditions
	}

	// 更新动作
	if update.Actions != nil {
		for i, action := range update.Actions {
			if err := validateAction(&action); err != nil {
				return fmt.Errorf("action[%d]: %w: %v", i, ErrInvalidAction, err)
			}
		}
		scene.Actions = update.Actions
	}

	scene.Enabled = update.Enabled
	scene.UpdatedAt = time.Now()

	return nil
}

// DeleteScene 删除场景.
func (m *Manager) DeleteScene(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.scenes[id]; !ok {
		return ErrSceneNotFound
	}

	// 关联的定时任务
	for _, task := range m.tasks {
		if task.SceneID == id {
			delete(m.tasks, task.ID)
		}
	}

	delete(m.scenes, id)
	return nil
}

// EnableScene 启用场景.
func (m *Manager) EnableScene(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	scene, ok := m.scenes[id]
	if !ok {
		return ErrSceneNotFound
	}

	scene.Enabled = true
	scene.UpdatedAt = time.Now()
	return nil
}

// DisableScene 禁用场景.
func (m *Manager) DisableScene(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	scene, ok := m.scenes[id]
	if !ok {
		return ErrSceneNotFound
	}

	scene.Enabled = false
	scene.UpdatedAt = time.Now()
	return nil
}

// ExecuteScene 执行场景.
func (m *Manager) ExecuteScene(id string) error {
	m.mu.Lock()
	scene, ok := m.scenes[id]
	if !ok {
		m.mu.Unlock()
		return ErrSceneNotFound
	}

	if !scene.Enabled {
		m.mu.Unlock()
		return ErrSceneDisabled
	}

	// 评估条件
	if !m.evaluateConditions(scene.Conditions) {
		m.mu.Unlock()
		return nil // 条件不满足，静默跳过
	}

	now := time.Now()
	scene.LastRun = &now
	scene.RunCount++
	m.mu.Unlock()

	// 执行动作
	return m.executeActions(scene.Actions)
}

// ============================================================
// 条件评估
// ============================================================

// evaluateConditions 评估所有条件（AND逻辑）.
func (m *Manager) evaluateConditions(conditions []Condition) bool {
	for _, cond := range conditions {
		if !m.evaluateCondition(cond) {
			return false
		}
	}
	return true
}

// evaluateCondition 评估单个条件.
func (m *Manager) evaluateCondition(cond Condition) bool {
	device, ok := m.devices[cond.DeviceID]
	if !ok {
		return false
	}

	currentValue, ok := device.State[cond.Field]
	if !ok {
		return false
	}

	return compareValues(currentValue, cond.Value, cond.Operator)
}

// compareValues 比较两个值.
func compareValues(current, expected any, op ComparisonOperator) bool {
	switch op {
	case OpEqual:
		return fmt.Sprintf("%v", current) == fmt.Sprintf("%v", expected)
	case OpNotEqual:
		return fmt.Sprintf("%v", current) != fmt.Sprintf("%v", expected)
	case OpGreaterThan:
		return toFloat64(current) > toFloat64(expected)
	case OpLessThan:
		return toFloat64(current) < toFloat64(expected)
	case OpGreaterEqual:
		return toFloat64(current) >= toFloat64(expected)
	case OpLessEqual:
		return toFloat64(current) <= toFloat64(expected)
	case OpContains:
		return contains(fmt.Sprintf("%v", current), fmt.Sprintf("%v", expected))
	default:
		return false
	}
}

// toFloat64 将值转换为float64.
func toFloat64(v any) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case uint:
		return float64(val)
	case uint64:
		return float64(val)
	default:
		return 0
	}
}

// contains 检查字符串是否包含子串.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 || findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ============================================================
// 动作执行
// ============================================================

// executeActions 执行一组动作.
func (m *Manager) executeActions(actions []Action) error {
	for _, action := range actions {
		if err := m.executeAction(action); err != nil {
			return err
		}
	}
	return nil
}

// executeAction 执行单个动作.
func (m *Manager) executeAction(action Action) error {
	switch action.Type {
	case ActionTypeDeviceControl:
		return m.executeDeviceControl(action)
	case ActionTypeNotification:
		return m.executeNotification(action)
	case ActionTypeDelay:
		return m.executeDelay(action)
	case ActionTypeScene:
		return m.ExecuteScene(action.SceneID)
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
}

// executeDeviceControl 执行设备控制动作.
func (m *Manager) executeDeviceControl(action Action) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	device, ok := m.devices[action.DeviceID]
	if !ok {
		return ErrDeviceNotFound
	}

	// 应用属性到设备状态
	for k, v := range action.Properties {
		device.State[k] = v
	}
	device.UpdatedAt = time.Now()
	device.LastSeen = time.Now()

	m.addEvent(DeviceEvent{
		DeviceID:   device.ID,
		DeviceName: device.Name,
		Type:       "scene_action",
		State:      action.Properties,
		Timestamp:  time.Now(),
	})

	return nil
}

// executeNotification 执行通知动作.
func (m *Manager) executeNotification(action Action) error {
	// 通知功能可以扩展为推送、邮件、短信等
	// 这里只记录事件
	m.mu.Lock()
	defer m.mu.Unlock()

	m.addEvent(DeviceEvent{
		DeviceID:  "system",
		Type:      "notification",
		State:     map[string]any{"message": action.Message},
		Timestamp: time.Now(),
	})

	return nil
}

// executeDelay 执行延时动作.
func (m *Manager) executeDelay(action Action) error {
	if action.DelayMs > 0 {
		time.Sleep(time.Duration(action.DelayMs) * time.Millisecond)
	}
	return nil
}

// ============================================================
// 定时任务管理
// ============================================================

// AddScheduledTask 添加定时任务.
func (m *Manager) AddScheduledTask(task *ScheduledTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if task.ID == "" {
		task.ID = uuid.New().String()
	}

	if _, exists := m.tasks[task.ID]; exists {
		return ErrTaskExists
	}

	// 验证场景存在
	if _, ok := m.scenes[task.SceneID]; !ok {
		return ErrSceneNotFound
	}

	// 验证cron表达式
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	if _, err := parser.Parse(task.CronExpr); err != nil {
		return fmt.Errorf("invalid cron expression: %w", err)
	}

	now := time.Now()
	task.CreatedAt = now
	task.UpdatedAt = now
	if !task.Enabled {
		task.Enabled = true
	}

	m.tasks[task.ID] = task
	return nil
}

// GetScheduledTask 获取定时任务.
func (m *Manager) GetScheduledTask(id string) (*ScheduledTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[id]
	if !ok {
		return nil, ErrTaskNotFound
	}
	return task, nil
}

// ListScheduledTasks 列出所有定时任务.
func (m *Manager) ListScheduledTasks() []*ScheduledTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	tasks := make([]*ScheduledTask, 0, len(m.tasks))
	for _, t := range m.tasks {
		tasks = append(tasks, t)
	}
	return tasks
}

// UpdateScheduledTask 更新定时任务.
func (m *Manager) UpdateScheduledTask(id string, update *ScheduledTask) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[id]
	if !ok {
		return ErrTaskNotFound
	}

	if update.Name != "" {
		task.Name = update.Name
	}
	if update.CronExpr != "" {
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		if _, err := parser.Parse(update.CronExpr); err != nil {
			return fmt.Errorf("invalid cron expression: %w", err)
		}
		task.CronExpr = update.CronExpr
	}
	if update.SceneID != "" {
		if _, ok := m.scenes[update.SceneID]; !ok {
			return ErrSceneNotFound
		}
		task.SceneID = update.SceneID
	}

	task.Enabled = update.Enabled
	task.UpdatedAt = time.Now()

	return nil
}

// DeleteScheduledTask 删除定时任务.
func (m *Manager) DeleteScheduledTask(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.tasks[id]; !ok {
		return ErrTaskNotFound
	}

	delete(m.tasks, id)
	return nil
}

// ExecuteScheduledTasks 执行到期的定时任务.
func (m *Manager) ExecuteScheduledTasks() {
	m.mu.Lock()
	tasks := make([]*ScheduledTask, 0)
	for _, task := range m.tasks {
		if task.Enabled {
			tasks = append(tasks, task)
		}
	}
	m.mu.Unlock()

	for _, task := range tasks {
		if m.shouldRunTask(task) {
			m.ExecuteScene(task.SceneID)

			m.mu.Lock()
			now := time.Now()
			task.LastRun = &now
			task.RunCount++
			task.UpdatedAt = now
			m.mu.Unlock()
		}
	}
}

// shouldRunTask 检查任务是否应该执行.
func (m *Manager) shouldRunTask(task *ScheduledTask) bool {
	parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	schedule, err := parser.Parse(task.CronExpr)
	if err != nil {
		return false
	}

	now := time.Now()
	var lastRun time.Time
	if task.LastRun != nil {
		lastRun = *task.LastRun
	} else {
		lastRun = task.CreatedAt
	}

	nextRun := schedule.Next(lastRun)
	return now.After(nextRun) || now.Equal(nextRun)
}

// ============================================================
// 验证函数
// ============================================================

// validateTrigger 验证触发器.
func validateTrigger(trigger *Trigger) error {
	switch trigger.Type {
	case TriggerTypeDevice:
		if trigger.DeviceID == "" {
			return fmt.Errorf("device_id is required for device trigger")
		}
		if trigger.Field == "" {
			return fmt.Errorf("field is required for device trigger")
		}
	case TriggerTypeTime:
		if trigger.CronExpr == "" && trigger.TimeStr == "" {
			return fmt.Errorf("cron_expr or time_str is required for time trigger")
		}
	case TriggerTypeSunrise, TriggerTypeSunset:
		// 日出日落触发不需要额外参数
	case TriggerTypeManual:
		// 手动触发不需要额外参数
	default:
		return fmt.Errorf("unknown trigger type: %s", trigger.Type)
	}
	return nil
}

// validateAction 验证动作.
func validateAction(action *Action) error {
	switch action.Type {
	case ActionTypeDeviceControl:
		if action.DeviceID == "" {
			return fmt.Errorf("device_id is required for device_control action")
		}
		if len(action.Properties) == 0 {
			return fmt.Errorf("properties are required for device_control action")
		}
	case ActionTypeNotification:
		if action.Message == "" {
			return fmt.Errorf("message is required for notification action")
		}
	case ActionTypeDelay:
		if action.DelayMs <= 0 {
			return fmt.Errorf("delay_ms must be positive for delay action")
		}
	case ActionTypeScene:
		if action.SceneID == "" {
			return fmt.Errorf("scene_id is required for scene action")
		}
	default:
		return fmt.Errorf("unknown action type: %s", action.Type)
	}
	return nil
}
