package smarthomeautomation

import (
	"testing"
	"time"
)

func TestNewSmartHomeAutomation(t *testing.T) {
	sha := NewSmartHomeAutomation()
	if sha == nil {
		t.Fatal("NewSmartHomeAutomation returned nil")
	}
}

// ==================== 设备管理测试 ====================

func TestAddDevice(t *testing.T) {
	sha := NewSmartHomeAutomation()

	device := &Device{
		ID:         "light1",
		Name:       "客厅灯",
		Type:       DeviceLight,
		Brand:      "Philips",
		Model:      "Hue",
		Properties: map[string]string{"brightness": "100", "color": "warm"},
	}

	err := sha.AddDevice(device)
	if err != nil {
		t.Fatalf("AddDevice failed: %v", err)
	}

	if device.Status != DeviceOnline {
		t.Errorf("expected online, got %v", device.Status)
	}

	// 测试重复添加
	err = sha.AddDevice(device)
	if err == nil {
		t.Error("expected error for duplicate device")
	}
}

func TestRemoveDevice(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.AddDevice(&Device{
		ID:         "light1",
		Name:       "客厅灯",
		Type:       DeviceLight,
		Properties: make(map[string]string),
	})

	err := sha.RemoveDevice("light1")
	if err != nil {
		t.Fatalf("RemoveDevice failed: %v", err)
	}

	_, err = sha.GetDevice("light1")
	if err == nil {
		t.Error("expected error for removed device")
	}
}

func TestGetDevice(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.AddDevice(&Device{
		ID:         "light1",
		Name:       "客厅灯",
		Type:       DeviceLight,
		Properties: make(map[string]string),
	})

	device, err := sha.GetDevice("light1")
	if err != nil {
		t.Fatalf("GetDevice failed: %v", err)
	}
	if device.Name != "客厅灯" {
		t.Errorf("expected 客厅灯, got %s", device.Name)
	}
}

func TestListDevices(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.AddDevice(&Device{ID: "light1", Name: "灯1", Type: DeviceLight, Brand: "Philips", Properties: make(map[string]string)})
	sha.AddDevice(&Device{ID: "light2", Name: "灯2", Type: DeviceLight, Brand: "Yeelight", Properties: make(map[string]string)})
	sha.AddDevice(&Device{ID: "sensor1", Name: "传感器1", Type: DeviceSensor, Brand: "Aqara", Properties: make(map[string]string)})

	// 按类型筛选
	lights := sha.ListDevices(DeviceLight, "")
	if len(lights) != 2 {
		t.Errorf("expected 2 lights, got %d", len(lights))
	}

	// 按品牌筛选
	philips := sha.ListDevices("", "Philips")
	if len(philips) != 1 {
		t.Errorf("expected 1 Philips device, got %d", len(philips))
	}
}

func TestUpdateDeviceStatus(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.AddDevice(&Device{ID: "light1", Name: "灯1", Type: DeviceLight, Properties: make(map[string]string)})

	err := sha.UpdateDeviceStatus("light1", DeviceOffline)
	if err != nil {
		t.Fatalf("UpdateDeviceStatus failed: %v", err)
	}

	device, _ := sha.GetDevice("light1")
	if device.Status != DeviceOffline {
		t.Errorf("expected offline, got %v", device.Status)
	}
}

func TestUpdateDeviceProperties(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.AddDevice(&Device{ID: "light1", Name: "灯1", Type: DeviceLight, Properties: make(map[string]string)})

	err := sha.UpdateDeviceProperties("light1", map[string]string{"brightness": "50"})
	if err != nil {
		t.Fatalf("UpdateDeviceProperties failed: %v", err)
	}

	device, _ := sha.GetDevice("light1")
	if device.Properties["brightness"] != "50" {
		t.Errorf("expected 50, got %s", device.Properties["brightness"])
	}

	// 检查状态历史
	states := sha.GetDeviceStateHistory("light1", 10)
	if len(states) != 1 {
		t.Errorf("expected 1 state record, got %d", len(states))
	}
}

// ==================== 规则引擎测试 ====================

func TestCreateRule(t *testing.T) {
	sha := NewSmartHomeAutomation()

	rule := &AutomationRule{
		ID:          "rule1",
		Name:        "回家自动开灯",
		Description: "当门锁打开时，自动打开客厅灯",
		Trigger:     Trigger{Type: TriggerEvent, DeviceID: "lock1", EventName: "unlock"},
		Conditions: ConditionGroup{
			Logic: LogicAnd,
			Conditions: []Condition{
				{DeviceID: "sensor1", Property: "brightness", Operator: OpLess, Value: "50"},
			},
		},
		Actions: []Action{
			{DeviceID: "light1", Command: "on", Parameters: map[string]string{"brightness": "100"}},
		},
		CreatedBy: "user1",
	}

	err := sha.CreateRule(rule)
	if err != nil {
		t.Fatalf("CreateRule failed: %v", err)
	}

	if rule.Status != RuleActive {
		t.Errorf("expected active, got %v", rule.Status)
	}

	// 测试重复创建
	err = sha.CreateRule(rule)
	if err == nil {
		t.Error("expected error for duplicate rule")
	}
}

func TestUpdateRule(t *testing.T) {
	sha := NewSmartHomeAutomation()

	rule := &AutomationRule{
		ID:   "rule1",
		Name: "规则1",
		Trigger: Trigger{Type: TriggerManual},
		Actions: []Action{},
		CreatedBy: "user1",
	}

	sha.CreateRule(rule)

	rule.Name = "更新后的规则"
	err := sha.UpdateRule(rule)
	if err != nil {
		t.Fatalf("UpdateRule failed: %v", err)
	}

	rule, _ = sha.GetRule("rule1")
	if rule.Name != "更新后的规则" {
		t.Errorf("expected 更新后的规则, got %s", rule.Name)
	}
}

func TestDeleteRule(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.CreateRule(&AutomationRule{
		ID:   "rule1",
		Name: "规则1",
		Trigger: Trigger{Type: TriggerManual},
		Actions: []Action{},
	})

	err := sha.DeleteRule("rule1")
	if err != nil {
		t.Fatalf("DeleteRule failed: %v", err)
	}

	_, err = sha.GetRule("rule1")
	if err == nil {
		t.Error("expected error for deleted rule")
	}
}

func TestListRules(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.CreateRule(&AutomationRule{ID: "rule1", Name: "规则1", Status: RuleActive, Trigger: Trigger{Type: TriggerTimer}, Actions: []Action{}})
	sha.CreateRule(&AutomationRule{ID: "rule2", Name: "规则2", Status: RuleInactive, Trigger: Trigger{Type: TriggerSensor}, Actions: []Action{}})
	sha.CreateRule(&AutomationRule{ID: "rule3", Name: "规则3", Status: RuleActive, Trigger: Trigger{Type: TriggerTimer}, Actions: []Action{}})

	// 按状态筛选
	activeRules := sha.ListRules(RuleActive, "")
	if len(activeRules) != 2 {
		t.Errorf("expected 2 active rules, got %d", len(activeRules))
	}

	// 按触发方式筛选
	timerRules := sha.ListRules("", TriggerTimer)
	if len(timerRules) != 2 {
		t.Errorf("expected 2 timer rules, got %d", len(timerRules))
	}
}

func TestEnableDisableRule(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.CreateRule(&AutomationRule{ID: "rule1", Name: "规则1", Trigger: Trigger{Type: TriggerManual}, Actions: []Action{}})

	// 禁用
	err := sha.DisableRule("rule1")
	if err != nil {
		t.Fatalf("DisableRule failed: %v", err)
	}

	rule, _ := sha.GetRule("rule1")
	if rule.Status != RuleInactive {
		t.Errorf("expected inactive, got %v", rule.Status)
	}

	// 启用
	err = sha.EnableRule("rule1")
	if err != nil {
		t.Fatalf("EnableRule failed: %v", err)
	}

	rule, _ = sha.GetRule("rule1")
	if rule.Status != RuleActive {
		t.Errorf("expected active, got %v", rule.Status)
	}
}

func TestEvaluateConditions(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.AddDevice(&Device{
		ID:         "sensor1",
		Name:       "光照传感器",
		Type:       DeviceSensor,
		Properties: map[string]string{"brightness": "30"},
	})

	// 条件满足
	group := ConditionGroup{
		Logic: LogicAnd,
		Conditions: []Condition{
			{DeviceID: "sensor1", Property: "brightness", Operator: OpLess, Value: "50"},
		},
	}

	if !sha.EvaluateConditions(group) {
		t.Error("expected condition to be true")
	}

	// 条件不满足
	group.Conditions[0].Operator = OpGreater
	if sha.EvaluateConditions(group) {
		t.Error("expected condition to be false")
	}
}

func TestExecuteRule(t *testing.T) {
	sha := NewSmartHomeAutomation()

	// 添加设备
	sha.AddDevice(&Device{
		ID:         "light1",
		Name:       "客厅灯",
		Type:       DeviceLight,
		Properties: make(map[string]string),
	})

	sha.AddDevice(&Device{
		ID:         "sensor1",
		Name:       "光照传感器",
		Type:       DeviceSensor,
		Properties: map[string]string{"brightness": "30"},
	})

	// 创建规则
	rule := &AutomationRule{
		ID:   "rule1",
		Name: "光线暗自动开灯",
		Trigger: Trigger{Type: TriggerSensor, DeviceID: "sensor1"},
		Conditions: ConditionGroup{
			Logic: LogicAnd,
			Conditions: []Condition{
				{DeviceID: "sensor1", Property: "brightness", Operator: OpLess, Value: "50"},
			},
		},
		Actions: []Action{
			{DeviceID: "light1", Command: "on", Parameters: map[string]string{"brightness": "100"}},
		},
		CreatedBy: "user1",
	}

	sha.CreateRule(rule)

	// 执行规则
	log, err := sha.ExecuteRule("rule1")
	if err != nil {
		t.Fatalf("ExecuteRule failed: %v", err)
	}

	if log.Status != "success" {
		t.Errorf("expected success, got %s", log.Status)
	}

	// 检查设备状态
	light, _ := sha.GetDevice("light1")
	if light.Properties["brightness"] != "100" {
		t.Errorf("expected brightness 100, got %s", light.Properties["brightness"])
	}

	// 检查规则统计
	rule, _ = sha.GetRule("rule1")
	if rule.RunCount != 1 {
		t.Errorf("expected run count 1, got %d", rule.RunCount)
	}
}

func TestExecuteRuleWithElse(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.AddDevice(&Device{
		ID:         "light1",
		Name:       "客厅灯",
		Type:       DeviceLight,
		Properties: map[string]string{"brightness": "100"},
	})

	sha.AddDevice(&Device{
		ID:         "sensor1",
		Name:       "光照传感器",
		Type:       DeviceSensor,
		Properties: map[string]string{"brightness": "80"},
	})

	rule := &AutomationRule{
		ID:   "rule1",
		Name: "光线调节",
		Trigger: Trigger{Type: TriggerSensor, DeviceID: "sensor1"},
		Conditions: ConditionGroup{
			Logic: LogicAnd,
			Conditions: []Condition{
				{DeviceID: "sensor1", Property: "brightness", Operator: OpLess, Value: "50"},
			},
		},
		Actions: []Action{
			{DeviceID: "light1", Command: "on", Parameters: map[string]string{"brightness": "100"}},
		},
		ElseActions: []Action{
			{DeviceID: "light1", Command: "off", Parameters: map[string]string{"brightness": "0"}},
		},
		CreatedBy: "user1",
	}

	sha.CreateRule(rule)

	log, err := sha.ExecuteRule("rule1")
	if err != nil {
		t.Fatalf("ExecuteRule failed: %v", err)
	}

	if log.Status != "success" {
		t.Errorf("expected success, got %s", log.Status)
	}

	light, _ := sha.GetDevice("light1")
	if light.Properties["brightness"] != "0" {
		t.Errorf("expected brightness 0, got %s", light.Properties["brightness"])
	}
}

func TestExecuteRuleCooldown(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.AddDevice(&Device{
		ID:         "light1",
		Name:       "灯",
		Type:       DeviceLight,
		Properties: make(map[string]string),
	})

	rule := &AutomationRule{
		ID:       "rule1",
		Name:     "冷却测试",
		Trigger:  Trigger{Type: TriggerManual},
		Actions:  []Action{{DeviceID: "light1", Command: "on"}},
		Cooldown: 5 * time.Second,
		CreatedBy: "user1",
	}

	sha.CreateRule(rule)

	// 第一次执行
	_, err := sha.ExecuteRule("rule1")
	if err != nil {
		t.Fatalf("first execute failed: %v", err)
	}

	// 第二次执行应该失败（冷却期内）
	_, err = sha.ExecuteRule("rule1")
	if err == nil {
		t.Error("expected error for cooldown")
	}
}

// ==================== 场景管理测试 ====================

func TestCreateScene(t *testing.T) {
	sha := NewSmartHomeAutomation()

	scene := &Scene{
		ID:          "scene1",
		Name:        "回家模式",
		Description: "一键开启回家模式",
		Actions: []Action{
			{DeviceID: "light1", Command: "on", Parameters: map[string]string{"brightness": "100"}},
			{DeviceID: "ac1", Command: "on", Parameters: map[string]string{"temperature": "25"}},
		},
		CreatedBy: "user1",
	}

	err := sha.CreateScene(scene)
	if err != nil {
		t.Fatalf("CreateScene failed: %v", err)
	}

	if scene.Status != SceneActive {
		t.Errorf("expected active, got %v", scene.Status)
	}
}

func TestUpdateScene(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.CreateScene(&Scene{ID: "scene1", Name: "场景1", Actions: []Action{}})

	scene, _ := sha.GetScene("scene1")
	scene.Name = "更新后的场景"
	err := sha.UpdateScene(scene)
	if err != nil {
		t.Fatalf("UpdateScene failed: %v", err)
	}

	scene, _ = sha.GetScene("scene1")
	if scene.Name != "更新后的场景" {
		t.Errorf("expected 更新后的场景, got %s", scene.Name)
	}
}

func TestDeleteScene(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.CreateScene(&Scene{ID: "scene1", Name: "场景1", Actions: []Action{}})

	err := sha.DeleteScene("scene1")
	if err != nil {
		t.Fatalf("DeleteScene failed: %v", err)
	}

	_, err = sha.GetScene("scene1")
	if err == nil {
		t.Error("expected error for deleted scene")
	}
}

func TestListScenes(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.CreateScene(&Scene{ID: "scene1", Name: "场景1", Order: 2, Actions: []Action{}})
	sha.CreateScene(&Scene{ID: "scene2", Name: "场景2", Order: 1, Actions: []Action{}})
	sha.CreateScene(&Scene{ID: "scene3", Name: "场景3", Order: 3, Status: SceneInactive, Actions: []Action{}})

	activeScenes := sha.ListScenes(SceneActive)
	if len(activeScenes) != 2 {
		t.Errorf("expected 2 active scenes, got %d", len(activeScenes))
	}

	// 检查排序
	if activeScenes[0].ID != "scene2" {
		t.Errorf("expected scene2 first, got %s", activeScenes[0].ID)
	}
}

func TestActivateScene(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.AddDevice(&Device{ID: "light1", Name: "灯1", Type: DeviceLight, Properties: make(map[string]string)})
	sha.AddDevice(&Device{ID: "ac1", Name: "空调1", Type: DeviceAirCon, Properties: make(map[string]string)})

	scene := &Scene{
		ID:   "scene1",
		Name: "回家模式",
		Actions: []Action{
			{DeviceID: "light1", Command: "on", Parameters: map[string]string{"brightness": "100"}},
			{DeviceID: "ac1", Command: "on", Parameters: map[string]string{"temperature": "25"}},
		},
	}

	sha.CreateScene(scene)

	log, err := sha.ActivateScene("scene1")
	if err != nil {
		t.Fatalf("ActivateScene failed: %v", err)
	}

	if log.Status != "success" {
		t.Errorf("expected success, got %s", log.Status)
	}

	if len(log.Actions) != 2 {
		t.Errorf("expected 2 actions, got %d", len(log.Actions))
	}

	light, _ := sha.GetDevice("light1")
	if light.Properties["brightness"] != "100" {
		t.Errorf("expected brightness 100, got %s", light.Properties["brightness"])
	}
}

// ==================== 日志测试 ====================

func TestGetLogs(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.AddDevice(&Device{ID: "light1", Name: "灯1", Type: DeviceLight, Properties: make(map[string]string)})

	sha.CreateRule(&AutomationRule{
		ID:       "rule1",
		Name:     "规则1",
		Trigger:  Trigger{Type: TriggerManual},
		Actions:  []Action{{DeviceID: "light1", Command: "on"}},
		CreatedBy: "user1",
	})

	// 执行几次
	sha.ExecuteRule("rule1")
	sha.ExecuteRule("rule1")

	logs := sha.GetLogs("", 10)
	if len(logs) != 2 {
		t.Errorf("expected 2 logs, got %d", len(logs))
	}

	// 按规则ID筛选
	logs = sha.GetLogs("rule1", 10)
	if len(logs) != 2 {
		t.Errorf("expected 2 logs for rule1, got %d", len(logs))
	}
}

func TestClearLogs(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.AddDevice(&Device{ID: "light1", Name: "灯1", Type: DeviceLight, Properties: make(map[string]string)})
	sha.CreateRule(&AutomationRule{
		ID:       "rule1",
		Name:     "规则1",
		Trigger:  Trigger{Type: TriggerManual},
		Actions:  []Action{{DeviceID: "light1", Command: "on"}},
		CreatedBy: "user1",
	})

	sha.ExecuteRule("rule1")

	count := sha.ClearLogs(time.Now().Add(1 * time.Hour))
	if count != 1 {
		t.Errorf("expected 1 cleared, got %d", count)
	}

	logs := sha.GetLogs("", 10)
	if len(logs) != 0 {
		t.Errorf("expected 0 logs, got %d", len(logs))
	}
}

// ==================== 使用模式与推荐测试 ====================

func TestTrackUsage(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.TrackUsage("light1", "on")
	sha.TrackUsage("light1", "on")
	sha.TrackUsage("light1", "on")

	// 检查使用模式
	sha.GenerateRecommendations()
	recs := sha.GetRecommendations(10)

	if len(recs) == 0 {
		t.Error("expected at least 1 recommendation")
	}
}

func TestGenerateRecommendations(t *testing.T) {
	sha := NewSmartHomeAutomation()

	// 模拟多次使用
	for i := 0; i < 5; i++ {
		sha.TrackUsage("light1", "on")
	}

	sha.GenerateRecommendations()
	recs := sha.GetRecommendations(10)

	if len(recs) != 1 {
		t.Errorf("expected 1 recommendation, got %d", len(recs))
	}

	if recs[0].Confidence <= 0 {
		t.Error("expected positive confidence")
	}
}

// ==================== 统计测试 ====================

func TestGetStats(t *testing.T) {
	sha := NewSmartHomeAutomation()

	// 添加设备
	sha.AddDevice(&Device{ID: "light1", Name: "灯1", Type: DeviceLight, Properties: make(map[string]string)})
	sha.AddDevice(&Device{ID: "light2", Name: "灯2", Type: DeviceLight, Properties: make(map[string]string)})
	sha.AddDevice(&Device{ID: "sensor1", Name: "传感器1", Type: DeviceSensor, Properties: make(map[string]string)})
	sha.UpdateDeviceStatus("light2", DeviceOffline)

	// 添加规则
	sha.CreateRule(&AutomationRule{
		ID:   "rule1",
		Name: "规则1",
		Trigger: Trigger{Type: TriggerManual},
		Actions: []Action{{DeviceID: "light1", Command: "on"}},
	})
	sha.CreateRule(&AutomationRule{
		ID:   "rule2",
		Name: "规则2",
		Status: RuleInactive,
		Trigger: Trigger{Type: TriggerManual},
		Actions: []Action{},
	})

	// 添加场景
	sha.CreateScene(&Scene{ID: "scene1", Name: "场景1", Actions: []Action{}})

	// 执行规则
	sha.ExecuteRule("rule1")

	stats := sha.GetStats()

	if stats.TotalDevices != 3 {
		t.Errorf("expected 3 devices, got %d", stats.TotalDevices)
	}
	if stats.OnlineDevices != 2 {
		t.Errorf("expected 2 online devices, got %d", stats.OnlineDevices)
	}
	if stats.TotalRules != 2 {
		t.Errorf("expected 2 rules, got %d", stats.TotalRules)
	}
	if stats.ActiveRules != 1 {
		t.Errorf("expected 1 active rule, got %d", stats.ActiveRules)
	}
	if stats.TotalScenes != 1 {
		t.Errorf("expected 1 scene, got %d", stats.TotalScenes)
	}
	if stats.ExecutionsToday != 1 {
		t.Errorf("expected 1 execution today, got %d", stats.ExecutionsToday)
	}
	if stats.SuccessRate != 100 {
		t.Errorf("expected 100%% success rate, got %.1f%%", stats.SuccessRate)
	}
}

// ==================== 边界测试 ====================

func TestGetDeviceNotFound(t *testing.T) {
	sha := NewSmartHomeAutomation()

	_, err := sha.GetDevice("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent device")
	}
}

func TestGetRuleNotFound(t *testing.T) {
	sha := NewSmartHomeAutomation()

	_, err := sha.GetRule("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent rule")
	}
}

func TestGetSceneNotFound(t *testing.T) {
	sha := NewSmartHomeAutomation()

	_, err := sha.GetScene("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent scene")
	}
}

func TestExecuteInactiveRule(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.CreateRule(&AutomationRule{
		ID:       "rule1",
		Name:     "规则1",
		Status:   RuleInactive,
		Trigger:  Trigger{Type: TriggerManual},
		Actions:  []Action{},
	})

	_, err := sha.ExecuteRule("rule1")
	if err == nil {
		t.Error("expected error for inactive rule")
	}
}

func TestActivateInactiveScene(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.CreateScene(&Scene{ID: "scene1", Name: "场景1", Status: SceneInactive, Actions: []Action{}})

	_, err := sha.ActivateScene("scene1")
	if err == nil {
		t.Error("expected error for inactive scene")
	}
}

func TestGetDeviceStateHistoryLimit(t *testing.T) {
	sha := NewSmartHomeAutomation()

	sha.AddDevice(&Device{ID: "light1", Name: "灯1", Type: DeviceLight, Properties: make(map[string]string)})

	for i := 0; i < 10; i++ {
		sha.UpdateDeviceProperties("light1", map[string]string{"brightness": string(rune('0' + i))})
	}

	states := sha.GetDeviceStateHistory("light1", 5)
	if len(states) != 5 {
		t.Errorf("expected 5 states, got %d", len(states))
	}
}

func TestConditionGroupEmpty(t *testing.T) {
	sha := NewSmartHomeAutomation()

	group := ConditionGroup{
		Logic:      LogicAnd,
		Conditions: []Condition{},
		Groups:     []ConditionGroup{},
	}

	if !sha.EvaluateConditions(group) {
		t.Error("expected empty group to be true")
	}
}
