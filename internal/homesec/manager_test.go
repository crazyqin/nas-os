// Package homesec 提供家庭安防系统单元测试
package homesec

import (
	"fmt"
	"strings"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	manager := NewManager()

	if manager == nil {
		t.Fatal("NewManager 返回 nil")
	}

	// 检查默认规则
	if len(manager.rules) != 3 {
		t.Errorf("期望 3 个默认规则，实际 %d", len(manager.rules))
	}

	// 检查默认安防码
	if manager.armCode != "1234" {
		t.Errorf("期望默认安防码 1234，实际 %s", manager.armCode)
	}

	// 检查面板状态
	if manager.panel.Status != PanelDisarmed {
		t.Errorf("期望面板状态 disarmed，实际 %s", manager.panel.Status)
	}
}

func TestAddDevice(t *testing.T) {
	manager := NewManager()

	device := &Device{
		Name:     "前门传感器",
		Type:     DeviceDoorWindow,
		Location: "前门",
		Battery:  100,
		Enabled:  true,
	}

	result, err := manager.AddDevice(device)
	if err != nil {
		t.Fatalf("AddDevice 失败: %v", err)
	}

	if result.ID == "" {
		t.Error("设备 ID 为空")
	}

	if result.Name != "前门传感器" {
		t.Errorf("期望设备名 前门传感器，实际 %s", result.Name)
	}

	// 测试重复添加
	_, err = manager.AddDevice(result)
	if err == nil {
		t.Error("重复添加设备应该失败")
	}
}

func TestUpdateDevice(t *testing.T) {
	manager := NewManager()

	device := &Device{
		Name:     "前门传感器",
		Type:     DeviceDoorWindow,
		Location: "前门",
		Battery:  100,
		Enabled:  true,
	}

	added, _ := manager.AddDevice(device)

	// 更新设备
	updated := &Device{
		Name:     "前门传感器-更新",
		Type:     DeviceDoorWindow,
		Location: "前门",
		Battery:  90,
		Enabled:  true,
	}

	result, err := manager.UpdateDevice(added.ID, updated)
	if err != nil {
		t.Fatalf("UpdateDevice 失败: %v", err)
	}

	if result.Name != "前门传感器-更新" {
		t.Errorf("期望设备名 前门传感器-更新，实际 %s", result.Name)
	}

	// 测试更新不存在的设备
	_, err = manager.UpdateDevice("nonexistent", updated)
	if err == nil {
		t.Error("更新不存在的设备应该失败")
	}
}

func TestDeleteDevice(t *testing.T) {
	manager := NewManager()

	device := &Device{
		Name:     "前门传感器",
		Type:     DeviceDoorWindow,
		Location: "前门",
		Battery:  100,
		Enabled:  true,
	}

	added, _ := manager.AddDevice(device)

	// 删除设备
	err := manager.DeleteDevice(added.ID)
	if err != nil {
		t.Fatalf("DeleteDevice 失败: %v", err)
	}

	// 确认已删除
	_, err = manager.GetDevice(added.ID)
	if err == nil {
		t.Error("获取已删除的设备应该失败")
	}

	// 测试删除不存在的设备
	err = manager.DeleteDevice("nonexistent")
	if err == nil {
		t.Error("删除不存在的设备应该失败")
	}
}

func TestGetDevice(t *testing.T) {
	manager := NewManager()

	device := &Device{
		Name:     "前门传感器",
		Type:     DeviceDoorWindow,
		Location: "前门",
		Battery:  100,
		Enabled:  true,
	}

	added, _ := manager.AddDevice(device)

	// 获取设备
	result, err := manager.GetDevice(added.ID)
	if err != nil {
		t.Fatalf("GetDevice 失败: %v", err)
	}

	if result.Name != "前门传感器" {
		t.Errorf("期望设备名 前门传感器，实际 %s", result.Name)
	}

	// 测试获取不存在的设备
	_, err = manager.GetDevice("nonexistent")
	if err == nil {
		t.Error("获取不存在的设备应该失败")
	}
}

func TestListDevices(t *testing.T) {
	manager := NewManager()

	// 添加多个设备
	manager.AddDevice(&Device{Name: "设备1", Type: DeviceDoorWindow, Enabled: true})
	manager.AddDevice(&Device{Name: "设备2", Type: DeviceMotion, Enabled: true})
	manager.AddDevice(&Device{Name: "设备3", Type: DeviceSmoke, Enabled: true})

	devices, err := manager.ListDevices()
	if err != nil {
		t.Fatalf("ListDevices 失败: %v", err)
	}

	if len(devices) != 3 {
		t.Errorf("期望 3 个设备，实际 %d", len(devices))
	}
}

func TestCreateZone(t *testing.T) {
	manager := NewManager()

	zone := &Zone{
		Name:      "一楼区域",
		Type:      ZonePerimeter,
		DeviceIDs: []string{},
	}

	result, err := manager.CreateZone(zone)
	if err != nil {
		t.Fatalf("CreateZone 失败: %v", err)
	}

	if result.ID == "" {
		t.Error("区域 ID 为空")
	}

	if result.Name != "一楼区域" {
		t.Errorf("期望区域名 一楼区域，实际 %s", result.Name)
	}

	// 检查面板是否更新
	if len(manager.panel.ZoneIDs) != 1 {
		t.Errorf("期望面板有 1 个区域，实际 %d", len(manager.panel.ZoneIDs))
	}
}

func TestUpdateZone(t *testing.T) {
	manager := NewManager()

	zone := &Zone{
		Name: "一楼区域",
		Type: ZonePerimeter,
	}

	added, _ := manager.CreateZone(zone)

	updated := &Zone{
		Name: "一楼区域-更新",
		Type: ZoneInterior,
	}

	result, err := manager.UpdateZone(added.ID, updated)
	if err != nil {
		t.Fatalf("UpdateZone 失败: %v", err)
	}

	if result.Name != "一楼区域-更新" {
		t.Errorf("期望区域名 一楼区域-更新，实际 %s", result.Name)
	}
}

func TestDeleteZone(t *testing.T) {
	manager := NewManager()

	zone := &Zone{
		Name: "一楼区域",
		Type: ZonePerimeter,
	}

	added, _ := manager.CreateZone(zone)

	// 删除区域
	err := manager.DeleteZone(added.ID)
	if err != nil {
		t.Fatalf("DeleteZone 失败: %v", err)
	}

	// 检查面板是否更新
	if len(manager.panel.ZoneIDs) != 0 {
		t.Errorf("期望面板有 0 个区域，实际 %d", len(manager.panel.ZoneIDs))
	}
}

func TestArmZone(t *testing.T) {
	manager := NewManager()

	zone := &Zone{
		Name: "一楼区域",
		Type: ZonePerimeter,
	}

	added, _ := manager.CreateZone(zone)

	// 布防区域
	err := manager.ArmZone(added.ID, ArmAway)
	if err != nil {
		t.Fatalf("ArmZone 失败: %v", err)
	}

	// 验证区域状态
	manager.mu.RLock()
	zone, _ = manager.zones[added.ID]
	manager.mu.RUnlock()

	if !zone.Armed {
		t.Error("区域应该处于布防状态")
	}

	// 检查事件
	events, _ := manager.GetEvents("", time.Time{}, time.Time{}, 100)
	if len(events) == 0 {
		t.Error("应该有布防事件")
	}
}

func TestDisarmZone(t *testing.T) {
	manager := NewManager()

	zone := &Zone{
		Name: "一楼区域",
		Type: ZonePerimeter,
	}

	added, _ := manager.CreateZone(zone)

	// 先布防
	manager.ArmZone(added.ID, ArmAway)

	// 撤防区域 - 正确的安防码
	err := manager.DisarmZone(added.ID, "1234")
	if err != nil {
		t.Fatalf("DisarmZone 失败: %v", err)
	}

	// 验证区域状态
	manager.mu.RLock()
	zone, _ = manager.zones[added.ID]
	manager.mu.RUnlock()

	if zone.Armed {
		t.Error("区域应该处于撤防状态")
	}

	// 测试错误的安防码
	manager.ArmZone(added.ID, ArmAway)
	err = manager.DisarmZone(added.ID, "wrong")
	if err == nil {
		t.Error("错误的安防码应该失败")
	}
}

func TestBypassZone(t *testing.T) {
	manager := NewManager()

	zone := &Zone{
		Name: "一楼区域",
		Type: ZonePerimeter,
	}

	added, _ := manager.CreateZone(zone)

	// 绕过区域
	err := manager.BypassZone(added.ID)
	if err != nil {
		t.Fatalf("BypassZone 失败: %v", err)
	}

	// 验证区域状态
	manager.mu.RLock()
	zone, _ = manager.zones[added.ID]
	manager.mu.RUnlock()

	if !zone.Bypass {
		t.Error("区域应该处于绕过状态")
	}
}

func TestTriggerAlarm(t *testing.T) {
	manager := NewManager()

	device := &Device{
		Name:    "前门传感器",
		Type:    DeviceDoorWindow,
		Enabled: true,
	}

	added, _ := manager.AddDevice(device)

	// 触发报警
	event, err := manager.TriggerAlarm(added.ID, EventTrigger)
	if err != nil {
		t.Fatalf("TriggerAlarm 失败: %v", err)
	}

	if event.ID == "" {
		t.Error("事件 ID 为空")
	}

	if event.Severity != SeverityCritical {
		t.Errorf("期望严重程度 critical，实际 %s", event.Severity)
	}

	// 验证设备状态
	manager.mu.RLock()
	device, _ = manager.devices[added.ID]
	manager.mu.RUnlock()

	if device.Status != StatusTriggered {
		t.Errorf("期望设备状态 triggered，实际 %s", device.Status)
	}
}

func TestGetEvents(t *testing.T) {
	manager := NewManager()

	// 添加设备并触发报警
	device1 := &Device{Name: "传感器1", Type: DeviceMotion, Enabled: true}
	device2 := &Device{Name: "传感器2", Type: DeviceDoorWindow, Enabled: true}
	added1, _ := manager.AddDevice(device1)
	added2, _ := manager.AddDevice(device2)

	manager.TriggerAlarm(added1.ID, EventTrigger)
	manager.TriggerAlarm(added2.ID, EventTrigger)

	// 获取所有事件
	events, err := manager.GetEvents("", time.Time{}, time.Time{}, 100)
	if err != nil {
		t.Fatalf("GetEvents 失败: %v", err)
	}

	if len(events) < 2 {
		t.Errorf("期望至少 2 个事件，实际 %d", len(events))
	}

	// 测试限制
	events, _ = manager.GetEvents("", time.Time{}, time.Time{}, 1)
	if len(events) != 1 {
		t.Errorf("期望 1 个事件，实际 %d", len(events))
	}
}

func TestAcknowledgeEvent(t *testing.T) {
	manager := NewManager()

	// 添加设备并触发报警
	device := &Device{Name: "传感器", Type: DeviceMotion, Enabled: true}
	added, _ := manager.AddDevice(device)
	event, _ := manager.TriggerAlarm(added.ID, EventTrigger)

	// 确认事件
	err := manager.AcknowledgeEvent(event.ID)
	if err != nil {
		t.Fatalf("AcknowledgeEvent 失败: %v", err)
	}

	// 验证事件状态
	events, _ := manager.GetEvents("", time.Time{}, time.Time{}, 100)
	for _, e := range events {
		if e.ID == event.ID && !e.Acked {
			t.Error("事件应该已确认")
		}
	}
}

func TestCreateAlarmRule(t *testing.T) {
	manager := NewManager()

	rule := &AlarmRule{
		Name: "自定义规则",
		Conditions: []Condition{
			{DeviceType: DeviceMotion, Status: StatusTriggered},
		},
		Actions: []Action{
			{Type: ActionNotify, Target: "admin"},
		},
		Enabled:  true,
		Priority: 5,
	}

	result, err := manager.CreateAlarmRule(rule)
	if err != nil {
		t.Fatalf("CreateAlarmRule 失败: %v", err)
	}

	if result.ID == "" {
		t.Error("规则 ID 为空")
	}

	if result.Name != "自定义规则" {
		t.Errorf("期望规则名 自定义规则，实际 %s", result.Name)
	}
}

func TestUpdateAlarmRule(t *testing.T) {
	manager := NewManager()

	rule := &AlarmRule{
		Name:     "自定义规则",
		Enabled:  true,
		Priority: 5,
	}

	added, _ := manager.CreateAlarmRule(rule)

	updated := &AlarmRule{
		Name:     "自定义规则-更新",
		Enabled:  false,
		Priority: 8,
	}

	result, err := manager.UpdateAlarmRule(added.ID, updated)
	if err != nil {
		t.Fatalf("UpdateAlarmRule 失败: %v", err)
	}

	if result.Name != "自定义规则-更新" {
		t.Errorf("期望规则名 自定义规则-更新，实际 %s", result.Name)
	}
}

func TestDeleteAlarmRule(t *testing.T) {
	manager := NewManager()

	rule := &AlarmRule{
		Name:    "自定义规则",
		Enabled: true,
	}

	added, _ := manager.CreateAlarmRule(rule)

	// 删除规则
	err := manager.DeleteAlarmRule(added.ID)
	if err != nil {
		t.Fatalf("DeleteAlarmRule 失败: %v", err)
	}

	// 确认已删除
	manager.mu.RLock()
	_, exists := manager.rules[added.ID]
	manager.mu.RUnlock()

	if exists {
		t.Error("规则应该已删除")
	}
}

func TestEvaluateRules(t *testing.T) {
	manager := NewManager()

	// 添加烟雾传感器
	device := &Device{
		Name:    "烟雾传感器",
		Type:    DeviceSmoke,
		Enabled: true,
	}
	added, _ := manager.AddDevice(device)

	// 触发报警
	event, _ := manager.TriggerAlarm(added.ID, EventTrigger)

	// 评估规则
	actions, err := manager.EvaluateRules(event)
	if err != nil {
		t.Fatalf("EvaluateRules 失败: %v", err)
	}

	// 应该匹配火灾规则
	if len(actions) == 0 {
		t.Error("应该有匹配的动作")
	}
}

func TestExecuteAction(t *testing.T) {
	manager := NewManager()

	action := Action{
		Type:   ActionNotify,
		Target: "admin",
	}

	err := manager.ExecuteAction(action)
	if err != nil {
		t.Fatalf("ExecuteAction 失败: %v", err)
	}
}

func TestCreateSchedule(t *testing.T) {
	manager := NewManager()

	schedule := &Schedule{
		Name:       "工作日布防",
		ZoneIDs:    []string{},
		ArmTime:    "22:00",
		DisarmTime: "07:00",
		Days:       []string{"monday", "tuesday", "wednesday", "thursday", "friday"},
		Enabled:    true,
	}

	result, err := manager.CreateSchedule(schedule)
	if err != nil {
		t.Fatalf("CreateSchedule 失败: %v", err)
	}

	if result.ID == "" {
		t.Error("计划 ID 为空")
	}

	if result.Name != "工作日布防" {
		t.Errorf("期望计划名 工作日布防，实际 %s", result.Name)
	}
}

func TestUpdateSchedule(t *testing.T) {
	manager := NewManager()

	schedule := &Schedule{
		Name:    "工作日布防",
		Enabled: true,
	}

	added, _ := manager.CreateSchedule(schedule)

	updated := &Schedule{
		Name:    "工作日布防-更新",
		Enabled: false,
	}

	result, err := manager.UpdateSchedule(added.ID, updated)
	if err != nil {
		t.Fatalf("UpdateSchedule 失败: %v", err)
	}

	if result.Name != "工作日布防-更新" {
		t.Errorf("期望计划名 工作日布防-更新，实际 %s", result.Name)
	}
}

func TestDeleteSchedule(t *testing.T) {
	manager := NewManager()

	schedule := &Schedule{
		Name:    "工作日布防",
		Enabled: true,
	}

	added, _ := manager.CreateSchedule(schedule)

	// 删除计划
	err := manager.DeleteSchedule(added.ID)
	if err != nil {
		t.Fatalf("DeleteSchedule 失败: %v", err)
	}

	// 确认已删除
	manager.mu.RLock()
	_, exists := manager.schedules[added.ID]
	manager.mu.RUnlock()

	if exists {
		t.Error("计划应该已删除")
	}
}

func TestCheckSchedules(t *testing.T) {
	manager := NewManager()

	// 创建一个区域
	zone := &Zone{
		Name: "测试区域",
		Type: ZonePerimeter,
	}
	addedZone, _ := manager.CreateZone(zone)

	// 创建一个计划 - 使用当前时间
	now := time.Now()
	currentTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
	currentDay := strings.ToLower(now.Weekday().String())

	schedule := &Schedule{
		Name:       "测试计划",
		ZoneIDs:    []string{addedZone.ID},
		ArmTime:    currentTime,
		DisarmTime: "23:59",
		Days:       []string{currentDay},
		Enabled:    true,
	}
	manager.CreateSchedule(schedule)

	// 执行计划检查
	err := manager.CheckSchedules()
	if err != nil {
		t.Fatalf("CheckSchedules 失败: %v", err)
	}

	// 验证区域已布防
	manager.mu.RLock()
	zone, _ = manager.zones[addedZone.ID]
	manager.mu.RUnlock()

	if !zone.Armed {
		t.Error("区域应该按计划布防")
	}
}

func TestGetPanelStatus(t *testing.T) {
	manager := NewManager()

	panel, err := manager.GetPanelStatus()
	if err != nil {
		t.Fatalf("GetPanelStatus 失败: %v", err)
	}

	if panel.Status != PanelDisarmed {
		t.Errorf("期望面板状态 disarmed，实际 %s", panel.Status)
	}
}

func TestArmAll(t *testing.T) {
	manager := NewManager()

	// 创建多个区域
	manager.CreateZone(&Zone{Name: "区域1", Type: ZonePerimeter})
	manager.CreateZone(&Zone{Name: "区域2", Type: ZoneInterior})

	// 全部布防
	err := manager.ArmAll(ArmAway)
	if err != nil {
		t.Fatalf("ArmAll 失败: %v", err)
	}

	// 验证所有区域已布防
	manager.mu.RLock()
	for _, zone := range manager.zones {
		if !zone.Armed {
			t.Errorf("区域 %s 应该已布防", zone.Name)
		}
	}
	manager.mu.RUnlock()

	// 检查面板状态
	if manager.panel.Status != PanelArmedAway {
		t.Errorf("期望面板状态 armed_away，实际 %s", manager.panel.Status)
	}
}

func TestDisarmAll(t *testing.T) {
	manager := NewManager()

	// 创建多个区域
	manager.CreateZone(&Zone{Name: "区域1", Type: ZonePerimeter})
	manager.CreateZone(&Zone{Name: "区域2", Type: ZoneInterior})

	// 先布防
	manager.ArmAll(ArmAway)

	// 全部撤防 - 正确的安防码
	err := manager.DisarmAll("1234")
	if err != nil {
		t.Fatalf("DisarmAll 失败: %v", err)
	}

	// 验证所有区域已撤防
	manager.mu.RLock()
	for _, zone := range manager.zones {
		if zone.Armed {
			t.Errorf("区域 %s 应该已撤防", zone.Name)
		}
	}
	manager.mu.RUnlock()

	// 测试错误的安防码
	manager.ArmAll(ArmAway)
	err = manager.DisarmAll("wrong")
	if err == nil {
		t.Error("错误的安防码应该失败")
	}
}

func TestGetSecurityScore(t *testing.T) {
	manager := NewManager()

	// 添加一些设备和区域
	manager.AddDevice(&Device{Name: "设备1", Type: DeviceDoorWindow, Enabled: true, Battery: 100})
	manager.AddDevice(&Device{Name: "设备2", Type: DeviceMotion, Enabled: true, Battery: 50})
	manager.CreateZone(&Zone{Name: "区域1", Type: ZonePerimeter})

	// 获取评分
	score, details, err := manager.GetSecurityScore()
	if err != nil {
		t.Fatalf("GetSecurityScore 失败: %v", err)
	}

	if score < 0 || score > 100 {
		t.Errorf("评分应该在 0-100 之间，实际 %d", score)
	}

	if details == nil {
		t.Error("评分明细不应该为空")
	}

	// 检查明细字段
	if _, ok := details["total_devices"]; !ok {
		t.Error("评分明细应该包含 total_devices")
	}

	if _, ok := details["total_zones"]; !ok {
		t.Error("评分明细应该包含 total_zones")
	}
}

func TestConcurrentAccess(t *testing.T) {
	manager := NewManager()

	// 并发添加设备
	done := make(chan bool, 10)
	for i := 0; i < 10; i++ {
		go func(index int) {
			device := &Device{
				Name:    fmt.Sprintf("设备%d", index),
				Type:    DeviceDoorWindow,
				Enabled: true,
			}
			manager.AddDevice(device)
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证所有设备都已添加
	devices, _ := manager.ListDevices()
	if len(devices) != 10 {
		t.Errorf("期望 10 个设备，实际 %d", len(devices))
	}
}
