package smarthomeautomation

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// TriggerType 触发方式
type TriggerType string

const (
	TriggerTimer    TriggerType = "timer"
	TriggerSensor   TriggerType = "sensor"
	TriggerEvent    TriggerType = "event"
	TriggerManual   TriggerType = "manual"
	TriggerSchedule TriggerType = "schedule"
)

// ComparisonOperator 比较运算符
type ComparisonOperator string

const (
	OpEqual        ComparisonOperator = "eq"
	OpNotEqual     ComparisonOperator = "ne"
	OpGreater      ComparisonOperator = "gt"
	OpLess         ComparisonOperator = "lt"
	OpGreaterEqual ComparisonOperator = "gte"
	OpLessEqual    ComparisonOperator = "lte"
	OpContains     ComparisonOperator = "contains"
	OpIn           ComparisonOperator = "in"
)

// LogicalOperator 逻辑运算符
type LogicalOperator string

const (
	LogicAnd LogicalOperator = "and"
	LogicOr  LogicalOperator = "or"
)

// DeviceType 设备类型
type DeviceType string

const (
	DeviceLight       DeviceType = "light"
	DeviceSwitch      DeviceType = "switch"
	DeviceThermostat  DeviceType = "thermostat"
	DeviceLock        DeviceType = "lock"
	DeviceCamera      DeviceType = "camera"
	DeviceSensor      DeviceType = "sensor"
	DevicePlug        DeviceType = "plug"
	DeviceFan         DeviceType = "fan"
	DeviceCurtain     DeviceType = "curtain"
	DeviceSpeaker     DeviceType = "speaker"
	DeviceTV          DeviceType = "tv"
	DeviceAirCon      DeviceType = "aircon"
	DeviceHumidifier  DeviceType = "humidifier"
	DeviceUnknown     DeviceType = "unknown"
)

// DeviceStatus 设备状态
type DeviceStatus string

const (
	DeviceOnline  DeviceStatus = "online"
	DeviceOffline DeviceStatus = "offline"
	DeviceError   DeviceStatus = "error"
	DeviceBusy    DeviceStatus = "busy"
)

// RuleStatus 规则状态
type RuleStatus string

const (
	RuleActive   RuleStatus = "active"
	RuleInactive RuleStatus = "inactive"
	RulePaused   RuleStatus = "paused"
)

// SceneStatus 场景状态
type SceneStatus string

const (
	SceneActive   SceneStatus = "active"
	SceneInactive SceneStatus = "inactive"
)

// Device 设备
type Device struct {
	ID         string            `json:"id"`
	Name       string            `json:"name"`
	Type       DeviceType        `json:"type"`
	Brand      string            `json:"brand"`
	Model      string            `json:"model"`
	Status     DeviceStatus      `json:"status"`
	Properties map[string]string `json:"properties"`
	LastSeen   time.Time         `json:"last_seen"`
	CreatedAt  time.Time         `json:"created_at"`
	UpdatedAt  time.Time         `json:"updated_at"`
}

// DeviceState 设备状态记录
type DeviceState struct {
	DeviceID  string            `json:"device_id"`
	State     map[string]string `json:"state"`
	Timestamp time.Time         `json:"timestamp"`
	Source    string            `json:"source"`
}

// Condition 条件
type Condition struct {
	DeviceID string            `json:"device_id"`
	Property string            `json:"property"`
	Operator ComparisonOperator `json:"operator"`
	Value    string            `json:"value"`
}

// ConditionGroup 条件组
type ConditionGroup struct {
	Logic      LogicalOperator `json:"logic"`
	Conditions []Condition     `json:"conditions"`
	Groups     []ConditionGroup `json:"groups,omitempty"`
}

// Action 动作
type Action struct {
	DeviceID   string            `json:"device_id"`
	Command    string            `json:"command"`
	Parameters map[string]string `json:"parameters,omitempty"`
	Delay      time.Duration     `json:"delay,omitempty"`
}

// Trigger 触发器
type Trigger struct {
	Type       TriggerType      `json:"type"`
	DeviceID   string           `json:"device_id,omitempty"`
	Property   string           `json:"property,omitempty"`
	Schedule   string           `json:"schedule,omitempty"` // cron expression
	EventName  string           `json:"event_name,omitempty"`
	TimeWindow *TimeWindow      `json:"time_window,omitempty"`
}

// TimeWindow 时间窗口
type TimeWindow struct {
	Start string `json:"start"` // HH:MM
	End   string `json:"end"`   // HH:MM
}

// AutomationRule 自动化规则
type AutomationRule struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Status      RuleStatus      `json:"status"`
	Trigger     Trigger         `json:"trigger"`
	Conditions  ConditionGroup  `json:"conditions"`
	Actions     []Action        `json:"actions"`
	ElseActions []Action        `json:"else_actions,omitempty"`
	MaxRetries  int             `json:"max_retries,omitempty"`
	Cooldown    time.Duration   `json:"cooldown,omitempty"`
	LastRun     *time.Time      `json:"last_run,omitempty"`
	RunCount    int             `json:"run_count"`
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
	CreatedBy   string          `json:"created_by"`
}

// Scene 场景
type Scene struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Description string        `json:"description"`
	Status      SceneStatus   `json:"status"`
	Actions     []Action      `json:"actions"`
	Icon        string        `json:"icon,omitempty"`
	Order       int           `json:"order"`
	CreatedAt   time.Time     `json:"created_at"`
	UpdatedAt   time.Time     `json:"updated_at"`
	CreatedBy   string        `json:"created_by"`
}

// AutomationLog 自动化日志
type AutomationLog struct {
	ID        string    `json:"id"`
	RuleID    string    `json:"rule_id"`
	RuleName  string    `json:"rule_name"`
	Trigger   string    `json:"trigger"`
	Actions   []string  `json:"actions"`
	Status    string    `json:"status"` // success, failed, partial
	Error     string    `json:"error,omitempty"`
	Duration  int64     `json:"duration_ms"`
	Timestamp time.Time `json:"timestamp"`
}

// UsagePattern 使用模式
type UsagePattern struct {
	DeviceID   string    `json:"device_id"`
	Action     string    `json:"action"`
	DayOfWeek  int       `json:"day_of_week"`
	Hour       int       `json:"hour"`
	Frequency  int       `json:"frequency"`
	LastUsed   time.Time `json:"last_used"`
}

// Recommendation 智能推荐
type Recommendation struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"` // rule, scene, device
	Title       string    `json:"title"`
	Description string    `json:"description"`
	Confidence  float64   `json:"confidence"`
	Action      Action    `json:"action"`
	CreatedAt   time.Time `json:"created_at"`
}

// AutomationStats 自动化统计
type AutomationStats struct {
	TotalRules       int            `json:"total_rules"`
	ActiveRules      int            `json:"active_rules"`
	TotalScenes      int            `json:"total_scenes"`
	TotalDevices     int            `json:"total_devices"`
	OnlineDevices    int            `json:"online_devices"`
	ExecutionsToday  int            `json:"executions_today"`
	SuccessRate      float64        `json:"success_rate"`
	TopRules         []RuleStat     `json:"top_rules"`
	DeviceTypeCounts map[string]int `json:"device_type_counts"`
}

// RuleStat 规则统计
type RuleStat struct {
	RuleID    string `json:"rule_id"`
	RuleName  string `json:"rule_name"`
	RunCount  int    `json:"run_count"`
	LastRun   *time.Time `json:"last_run"`
}

// SmartHomeAutomation 智能家居自动化引擎
type SmartHomeAutomation struct {
	mu             sync.RWMutex
	devices        map[string]*Device
	deviceStates   map[string][]*DeviceState
	rules          map[string]*AutomationRule
	scenes         map[string]*Scene
	logs           []*AutomationLog
	usagePatterns  map[string]*UsagePattern
	recommendations map[string]*Recommendation
}

// NewSmartHomeAutomation 创建智能家居自动化引擎
func NewSmartHomeAutomation() *SmartHomeAutomation {
	return &SmartHomeAutomation{
		devices:        make(map[string]*Device),
		deviceStates:   make(map[string][]*DeviceState),
		rules:          make(map[string]*AutomationRule),
		scenes:         make(map[string]*Scene),
		logs:           make([]*AutomationLog, 0),
		usagePatterns:  make(map[string]*UsagePattern),
		recommendations: make(map[string]*Recommendation),
	}
}

// ==================== 设备管理 ====================

// AddDevice 添加设备
func (sha *SmartHomeAutomation) AddDevice(device *Device) error {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	if _, exists := sha.devices[device.ID]; exists {
		return fmt.Errorf("设备 %s 已存在", device.ID)
	}

	device.Status = DeviceOnline
	device.LastSeen = time.Now()
	device.CreatedAt = time.Now()
	device.UpdatedAt = time.Now()

	sha.devices[device.ID] = device
	return nil
}

// RemoveDevice 移除设备
func (sha *SmartHomeAutomation) RemoveDevice(deviceID string) error {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	if _, exists := sha.devices[deviceID]; !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	delete(sha.devices, deviceID)
	delete(sha.deviceStates, deviceID)
	return nil
}

// GetDevice 获取设备
func (sha *SmartHomeAutomation) GetDevice(deviceID string) (*Device, error) {
	sha.mu.RLock()
	defer sha.mu.RUnlock()

	device, exists := sha.devices[deviceID]
	if !exists {
		return nil, fmt.Errorf("设备 %s 不存在", deviceID)
	}
	return device, nil
}

// ListDevices 列出设备
func (sha *SmartHomeAutomation) ListDevices(deviceType DeviceType, brand string) []*Device {
	sha.mu.RLock()
	defer sha.mu.RUnlock()

	devices := make([]*Device, 0)
	for _, device := range sha.devices {
		if deviceType != "" && device.Type != deviceType {
			continue
		}
		if brand != "" && device.Brand != brand {
			continue
		}
		devices = append(devices, device)
	}
	return devices
}

// UpdateDeviceStatus 更新设备状态
func (sha *SmartHomeAutomation) UpdateDeviceStatus(deviceID string, status DeviceStatus) error {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	device, exists := sha.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	device.Status = status
	device.LastSeen = time.Now()
	device.UpdatedAt = time.Now()
	return nil
}

// UpdateDeviceProperties 更新设备属性
func (sha *SmartHomeAutomation) UpdateDeviceProperties(deviceID string, properties map[string]string) error {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	device, exists := sha.devices[deviceID]
	if !exists {
		return fmt.Errorf("设备 %s 不存在", deviceID)
	}

	for k, v := range properties {
		device.Properties[k] = v
	}

	device.LastSeen = time.Now()
	device.UpdatedAt = time.Now()

	// 记录状态历史
	state := &DeviceState{
		DeviceID:  deviceID,
		State:     properties,
		Timestamp: time.Now(),
		Source:    "update",
	}
	sha.deviceStates[deviceID] = append(sha.deviceStates[deviceID], state)

	return nil
}

// GetDeviceStateHistory 获取设备状态历史
func (sha *SmartHomeAutomation) GetDeviceStateHistory(deviceID string, limit int) []*DeviceState {
	sha.mu.RLock()
	defer sha.mu.RUnlock()

	states := sha.deviceStates[deviceID]
	if limit <= 0 || limit > len(states) {
		limit = len(states)
	}

	start := len(states) - limit
	if start < 0 {
		start = 0
	}
	return states[start:]
}

// ==================== 规则引擎 ====================

// CreateRule 创建自动化规则
func (sha *SmartHomeAutomation) CreateRule(rule *AutomationRule) error {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	if _, exists := sha.rules[rule.ID]; exists {
		return fmt.Errorf("规则 %s 已存在", rule.ID)
	}

	if rule.Status == "" {
		rule.Status = RuleActive
	}
	rule.RunCount = 0
	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	sha.rules[rule.ID] = rule
	return nil
}

// UpdateRule 更新自动化规则
func (sha *SmartHomeAutomation) UpdateRule(rule *AutomationRule) error {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	if _, exists := sha.rules[rule.ID]; !exists {
		return fmt.Errorf("规则 %s 不存在", rule.ID)
	}

	rule.UpdatedAt = time.Now()
	sha.rules[rule.ID] = rule
	return nil
}

// DeleteRule 删除自动化规则
func (sha *SmartHomeAutomation) DeleteRule(ruleID string) error {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	if _, exists := sha.rules[ruleID]; !exists {
		return fmt.Errorf("规则 %s 不存在", ruleID)
	}

	delete(sha.rules, ruleID)
	return nil
}

// GetRule 获取规则
func (sha *SmartHomeAutomation) GetRule(ruleID string) (*AutomationRule, error) {
	sha.mu.RLock()
	defer sha.mu.RUnlock()

	rule, exists := sha.rules[ruleID]
	if !exists {
		return nil, fmt.Errorf("规则 %s 不存在", ruleID)
	}
	return rule, nil
}

// ListRules 列出规则
func (sha *SmartHomeAutomation) ListRules(status RuleStatus, triggerType TriggerType) []*AutomationRule {
	sha.mu.RLock()
	defer sha.mu.RUnlock()

	rules := make([]*AutomationRule, 0)
	for _, rule := range sha.rules {
		if status != "" && rule.Status != status {
			continue
		}
		if triggerType != "" && rule.Trigger.Type != triggerType {
			continue
		}
		rules = append(rules, rule)
	}
	return rules
}

// EnableRule 启用规则
func (sha *SmartHomeAutomation) EnableRule(ruleID string) error {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	rule, exists := sha.rules[ruleID]
	if !exists {
		return fmt.Errorf("规则 %s 不存在", ruleID)
	}

	rule.Status = RuleActive
	rule.UpdatedAt = time.Now()
	return nil
}

// DisableRule 禁用规则
func (sha *SmartHomeAutomation) DisableRule(ruleID string) error {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	rule, exists := sha.rules[ruleID]
	if !exists {
		return fmt.Errorf("规则 %s 不存在", ruleID)
	}

	rule.Status = RuleInactive
	rule.UpdatedAt = time.Now()
	return nil
}

// EvaluateConditions 评估条件
func (sha *SmartHomeAutomation) EvaluateConditions(group ConditionGroup) bool {
	sha.mu.RLock()
	defer sha.mu.RUnlock()

	return sha.evaluateConditionGroup(group)
}

func (sha *SmartHomeAutomation) evaluateConditionGroup(group ConditionGroup) bool {
	if len(group.Conditions) == 0 && len(group.Groups) == 0 {
		return true
	}

	results := make([]bool, 0)

	// 评估直接条件
	for _, cond := range group.Conditions {
		results = append(results, sha.evaluateCondition(cond))
	}

	// 评估子条件组
	for _, subGroup := range group.Groups {
		results = append(results, sha.evaluateConditionGroup(subGroup))
	}

	if len(results) == 0 {
		return true
	}

	// 应用逻辑运算符
	switch group.Logic {
	case LogicOr:
		for _, r := range results {
			if r {
				return true
			}
		}
		return false
	default: // LogicAnd
		for _, r := range results {
			if !r {
				return false
			}
		}
		return true
	}
}

func (sha *SmartHomeAutomation) evaluateCondition(cond Condition) bool {
	device, exists := sha.devices[cond.DeviceID]
	if !exists {
		return false
	}

	value, ok := device.Properties[cond.Property]
	if !ok {
		return false
	}

	switch cond.Operator {
	case OpEqual:
		return value == cond.Value
	case OpNotEqual:
		return value != cond.Value
	case OpGreater:
		return compareNumeric(value, cond.Value) > 0
	case OpLess:
		return compareNumeric(value, cond.Value) < 0
	case OpGreaterEqual:
		return compareNumeric(value, cond.Value) >= 0
	case OpLessEqual:
		return compareNumeric(value, cond.Value) <= 0
	case OpContains:
		return containsString(value, cond.Value)
	default:
		return false
	}
}

// ExecuteRule 执行规则
func (sha *SmartHomeAutomation) ExecuteRule(ruleID string) (*AutomationLog, error) {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	rule, exists := sha.rules[ruleID]
	if !exists {
		return nil, fmt.Errorf("规则 %s 不存在", ruleID)
	}

	if rule.Status != RuleActive {
		return nil, fmt.Errorf("规则 %s 未激活", ruleID)
	}

	startTime := time.Now()

	// 检查冷却期
	if rule.LastRun != nil && rule.Cooldown > 0 {
		if time.Since(*rule.LastRun) < rule.Cooldown {
			return nil, fmt.Errorf("规则 %s 在冷却期内", ruleID)
		}
	}

	// 评估条件
	conditionMet := sha.evaluateConditionGroup(rule.Conditions)

	// 执行动作
	actions := rule.Actions
	if !conditionMet && len(rule.ElseActions) > 0 {
		actions = rule.ElseActions
	}

	executedActions := make([]string, 0)
	var execErr error

	for _, action := range actions {
		if err := sha.executeAction(action); err != nil {
			execErr = err
			break
		}
		executedActions = append(executedActions, fmt.Sprintf("%s:%s", action.DeviceID, action.Command))
	}

	// 更新规则统计
	now := time.Now()
	rule.LastRun = &now
	rule.RunCount++
	rule.UpdatedAt = now

	// 创建日志
	log := &AutomationLog{
		ID:        fmt.Sprintf("log_%d", time.Now().UnixNano()),
		RuleID:    ruleID,
		RuleName:  rule.Name,
		Trigger:   string(rule.Trigger.Type),
		Actions:   executedActions,
		Duration:  time.Since(startTime).Milliseconds(),
		Timestamp: now,
	}

	if execErr != nil {
		log.Status = "failed"
		log.Error = execErr.Error()
	} else if conditionMet {
		log.Status = "success"
	} else {
		log.Status = "success"
	}

	sha.logs = append(sha.logs, log)
	return log, nil
}

func (sha *SmartHomeAutomation) executeAction(action Action) error {
	device, exists := sha.devices[action.DeviceID]
	if !exists {
		return fmt.Errorf("设备 %s 不存在", action.DeviceID)
	}

	if device.Status != DeviceOnline {
		return fmt.Errorf("设备 %s 离线", action.DeviceID)
	}

	// 模拟执行动作
	if action.Parameters != nil {
		for k, v := range action.Parameters {
			device.Properties[k] = v
		}
	}

	device.UpdatedAt = time.Now()
	return nil
}

// ==================== 场景管理 ====================

// CreateScene 创建场景
func (sha *SmartHomeAutomation) CreateScene(scene *Scene) error {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	if _, exists := sha.scenes[scene.ID]; exists {
		return fmt.Errorf("场景 %s 已存在", scene.ID)
	}

	if scene.Status == "" {
		scene.Status = SceneActive
	}
	scene.CreatedAt = time.Now()
	scene.UpdatedAt = time.Now()

	sha.scenes[scene.ID] = scene
	return nil
}

// UpdateScene 更新场景
func (sha *SmartHomeAutomation) UpdateScene(scene *Scene) error {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	if _, exists := sha.scenes[scene.ID]; !exists {
		return fmt.Errorf("场景 %s 不存在", scene.ID)
	}

	scene.UpdatedAt = time.Now()
	sha.scenes[scene.ID] = scene
	return nil
}

// DeleteScene 删除场景
func (sha *SmartHomeAutomation) DeleteScene(sceneID string) error {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	if _, exists := sha.scenes[sceneID]; !exists {
		return fmt.Errorf("场景 %s 不存在", sceneID)
	}

	delete(sha.scenes, sceneID)
	return nil
}

// GetScene 获取场景
func (sha *SmartHomeAutomation) GetScene(sceneID string) (*Scene, error) {
	sha.mu.RLock()
	defer sha.mu.RUnlock()

	scene, exists := sha.scenes[sceneID]
	if !exists {
		return nil, fmt.Errorf("场景 %s 不存在", sceneID)
	}
	return scene, nil
}

// ListScenes 列出场景
func (sha *SmartHomeAutomation) ListScenes(status SceneStatus) []*Scene {
	sha.mu.RLock()
	defer sha.mu.RUnlock()

	scenes := make([]*Scene, 0)
	for _, scene := range sha.scenes {
		if status != "" && scene.Status != status {
			continue
		}
		scenes = append(scenes, scene)
	}

	sort.Slice(scenes, func(i, j int) bool {
		return scenes[i].Order < scenes[j].Order
	})

	return scenes
}

// ActivateScene 激活场景
func (sha *SmartHomeAutomation) ActivateScene(sceneID string) (*AutomationLog, error) {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	scene, exists := sha.scenes[sceneID]
	if !exists {
		return nil, fmt.Errorf("场景 %s 不存在", sceneID)
	}

	if scene.Status != SceneActive {
		return nil, fmt.Errorf("场景 %s 未激活", sceneID)
	}

	startTime := time.Now()
	executedActions := make([]string, 0)
	var execErr error

	for _, action := range scene.Actions {
		if err := sha.executeAction(action); err != nil {
			execErr = err
			break
		}
		executedActions = append(executedActions, fmt.Sprintf("%s:%s", action.DeviceID, action.Command))
	}

	log := &AutomationLog{
		ID:        fmt.Sprintf("scene_log_%d", time.Now().UnixNano()),
		RuleID:    sceneID,
		RuleName:  scene.Name,
		Trigger:   "scene",
		Actions:   executedActions,
		Duration:  time.Since(startTime).Milliseconds(),
		Timestamp: time.Now(),
	}

	if execErr != nil {
		log.Status = "failed"
		log.Error = execErr.Error()
	} else {
		log.Status = "success"
	}

	sha.logs = append(sha.logs, log)
	return log, nil
}

// ==================== 日志管理 ====================

// GetLogs 获取日志
func (sha *SmartHomeAutomation) GetLogs(ruleID string, limit int) []*AutomationLog {
	sha.mu.RLock()
	defer sha.mu.RUnlock()

	logs := make([]*AutomationLog, 0)
	for i := len(sha.logs) - 1; i >= 0; i-- {
		if ruleID != "" && sha.logs[i].RuleID != ruleID {
			continue
		}
		logs = append(logs, sha.logs[i])
		if limit > 0 && len(logs) >= limit {
			break
		}
	}
	return logs
}

// ClearLogs 清除日志
func (sha *SmartHomeAutomation) ClearLogs(before time.Time) int {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	count := 0
	newLogs := make([]*AutomationLog, 0)
	for _, log := range sha.logs {
		if log.Timestamp.Before(before) {
			count++
		} else {
			newLogs = append(newLogs, log)
		}
	}
	sha.logs = newLogs
	return count
}

// ==================== 使用模式与推荐 ====================

// TrackUsage 追踪使用模式
func (sha *SmartHomeAutomation) TrackUsage(deviceID, action string) {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	now := time.Now()
	key := fmt.Sprintf("%s:%s:%d:%d", deviceID, action, now.Weekday(), now.Hour())

	pattern, exists := sha.usagePatterns[key]
	if !exists {
		pattern = &UsagePattern{
			DeviceID:  deviceID,
			Action:    action,
			DayOfWeek: int(now.Weekday()),
			Hour:      now.Hour(),
			Frequency: 0,
		}
		sha.usagePatterns[key] = pattern
	}

	pattern.Frequency++
	pattern.LastUsed = now
}

// GetRecommendations 获取智能推荐
func (sha *SmartHomeAutomation) GetRecommendations(limit int) []*Recommendation {
	sha.mu.RLock()
	defer sha.mu.RUnlock()

	recs := make([]*Recommendation, 0)
	for _, rec := range sha.recommendations {
		recs = append(recs, rec)
	}

	sort.Slice(recs, func(i, j int) bool {
		return recs[i].Confidence > recs[j].Confidence
	})

	if limit > 0 && limit < len(recs) {
		recs = recs[:limit]
	}
	return recs
}

// GenerateRecommendations 生成推荐
func (sha *SmartHomeAutomation) GenerateRecommendations() {
	sha.mu.Lock()
	defer sha.mu.Unlock()

	// 清除旧推荐
	sha.recommendations = make(map[string]*Recommendation)

	// 基于使用模式生成推荐
	for _, pattern := range sha.usagePatterns {
		if pattern.Frequency >= 3 {
			rec := &Recommendation{
				ID:          fmt.Sprintf("rec_%s_%s", pattern.DeviceID, pattern.Action),
				Type:        "rule",
				Title:       fmt.Sprintf("自动执行 %s 的 %s", pattern.DeviceID, pattern.Action),
				Description: fmt.Sprintf("您在 %s %d:00 经常执行此操作", weekdayName(pattern.DayOfWeek), pattern.Hour),
				Confidence:  float64(pattern.Frequency) / 10.0,
				Action: Action{
					DeviceID: pattern.DeviceID,
					Command:  pattern.Action,
				},
				CreatedAt: time.Now(),
			}
			if rec.Confidence > 1.0 {
				rec.Confidence = 1.0
			}
			sha.recommendations[rec.ID] = rec
		}
	}
}

// ==================== 统计 ====================

// GetStats 获取统计信息
func (sha *SmartHomeAutomation) GetStats() *AutomationStats {
	sha.mu.RLock()
	defer sha.mu.RUnlock()

	stats := &AutomationStats{
		DeviceTypeCounts: make(map[string]int),
		TopRules:         make([]RuleStat, 0),
	}

	stats.TotalDevices = len(sha.devices)
	stats.TotalScenes = len(sha.scenes)
	stats.TotalRules = len(sha.rules)

	for _, device := range sha.devices {
		stats.DeviceTypeCounts[string(device.Type)]++
		if device.Status == DeviceOnline {
			stats.OnlineDevices++
		}
	}

	for _, rule := range sha.rules {
		if rule.Status == RuleActive {
			stats.ActiveRules++
		}
		stats.TopRules = append(stats.TopRules, RuleStat{
			RuleID:   rule.ID,
			RuleName: rule.Name,
			RunCount: rule.RunCount,
			LastRun:  rule.LastRun,
		})
	}

	// 统计今日执行
	today := time.Now().Truncate(24 * time.Hour)
	successCount := 0
	for _, log := range sha.logs {
		if log.Timestamp.After(today) {
			stats.ExecutionsToday++
			if log.Status == "success" {
				successCount++
			}
		}
	}

	if stats.ExecutionsToday > 0 {
		stats.SuccessRate = float64(successCount) / float64(stats.ExecutionsToday) * 100
	}

	// 排序 top rules
	sort.Slice(stats.TopRules, func(i, j int) bool {
		return stats.TopRules[i].RunCount > stats.TopRules[j].RunCount
	})

	if len(stats.TopRules) > 10 {
		stats.TopRules = stats.TopRules[:10]
	}

	return stats
}

// ==================== 辅助函数 ====================

func containsString(s, substr string) bool {
	return len(substr) > 0 && len(s) >= len(substr) && (s == substr || len(s) > 0 && containsSubstr(s, substr))
}

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func weekdayName(day int) string {
	names := []string{"周日", "周一", "周二", "周三", "周四", "周五", "周六"}
	if day >= 0 && day < len(names) {
		return names[day]
	}
	return "未知"
}

func compareNumeric(a, b string) int {
	numA, errA := parseNumber(a)
	numB, errB := parseNumber(b)
	if errA != nil || errB != nil {
		// 字符串比较
		if a < b {
			return -1
		}
		if a > b {
			return 1
		}
		return 0
	}
	if numA < numB {
		return -1
	}
	if numA > numB {
		return 1
	}
	return 0
}

func parseNumber(s string) (float64, error) {
	var n float64
	_, err := fmt.Sscanf(s, "%f", &n)
	return n, err
}
