// Package storageanomaly - 单元测试
package storageanomaly

import (
	"testing"
	"time"
)

func TestDefaultAnomalyConfig(t *testing.T) {
	config := DefaultAnomalyConfig()

	if config.CapacityGrowthThreshold != 10.0 {
		t.Errorf("CapacityGrowthThreshold 期望 10.0, 实际 %f", config.CapacityGrowthThreshold)
	}

	if config.IOPSSpikeThreshold != 3.0 {
		t.Errorf("IOPSSpikeThreshold 期望 3.0, 实际 %f", config.IOPSSpikeThreshold)
	}

	if config.LatencySpikeThreshold != 100.0 {
		t.Errorf("LatencySpikeThreshold 期望 100.0, 实际 %f", config.LatencySpikeThreshold)
	}

	if config.SampleInterval != 5*time.Minute {
		t.Errorf("SampleInterval 期望 5m, 实际 %v", config.SampleInterval)
	}

	if config.HistoryWindow != 24*time.Hour {
		t.Errorf("HistoryWindow 期望 24h, 实际 %v", config.HistoryWindow)
	}
}

func TestNewAnomalyManager(t *testing.T) {
	manager := NewAnomalyManager(nil)
	if manager == nil {
		t.Fatal("NewAnomalyManager 返回 nil")
	}

	if manager.rules == nil {
		t.Fatal("rules map 为 nil")
	}

	if manager.metrics == nil {
		t.Fatal("metrics map 为 nil")
	}

	// 验证默认规则已初始化
	rules := manager.ListRules()
	if len(rules) < 4 {
		t.Errorf("期望至少 4 个默认规则, 实际 %d", len(rules))
	}
}

func TestCollectMetrics(t *testing.T) {
	manager := NewAnomalyManager(nil)

	metrics := &StorageMetrics{
		DeviceID:     "test-device",
		MountPoint:   "/data",
		TotalSpace:   1024 * 1024 * 1024 * 100, // 100GB
		UsedSpace:    1024 * 1024 * 1024 * 60,  // 60GB
		FreeSpace:    1024 * 1024 * 1024 * 40,  // 40GB
		UsagePercent: 60.0,
		ReadIOPS:     100,
		WriteIOPS:    50,
		ReadLatency:  10,
		WriteLatency: 15,
	}

	err := manager.CollectMetrics(metrics)
	if err != nil {
		t.Fatalf("CollectMetrics 失败: %v", err)
	}

	// 验证指标已保存
	manager.mu.RLock()
	history, exists := manager.metrics["test-device"]
	manager.mu.RUnlock()

	if !exists {
		t.Fatal("指标未保存")
	}

	if len(history) != 1 {
		t.Errorf("期望 1 个指标, 实际 %d", len(history))
	}
}

func TestCollectMetricsNoDeviceID(t *testing.T) {
	manager := NewAnomalyManager(nil)

	metrics := &StorageMetrics{
		DeviceID: "",
	}

	err := manager.CollectMetrics(metrics)
	if err == nil {
		t.Error("期望返回错误")
	}
}

func TestDetectAnomaliesNoData(t *testing.T) {
	manager := NewAnomalyManager(nil)

	result, err := manager.DetectAnomalies("non-existent")
	if err != nil {
		t.Fatalf("DetectAnomalies 失败: %v", err)
	}

	if result.HasAnomaly {
		t.Error("无数据时不应有异常")
	}
}

func TestDetectAnomaliesInsufficientSamples(t *testing.T) {
	manager := NewAnomalyManager(nil)

	// 添加少量样本
	for i := 0; i < 5; i++ {
		manager.CollectMetrics(&StorageMetrics{
			DeviceID:     "test-device",
			UsagePercent: 50.0,
			CollectedAt:  time.Now(),
		})
	}

	result, err := manager.DetectAnomalies("test-device")
	if err != nil {
		t.Fatalf("DetectAnomalies 失败: %v", err)
	}

	if result.HasAnomaly {
		t.Error("样本不足时不应有异常")
	}
}

func TestDetectCapacityAnomaly(t *testing.T) {
	config := DefaultAnomalyConfig()
	config.MinSamples = 5
	manager := NewAnomalyManager(&config)

	// 添加高使用率指标
	for i := 0; i < 10; i++ {
		manager.CollectMetrics(&StorageMetrics{
			DeviceID:     "test-device",
			MountPoint:   "/data",
			UsagePercent: 95.0,
			CollectedAt:  time.Now().Add(time.Duration(-10+i) * time.Minute),
		})
	}

	result, err := manager.DetectAnomalies("test-device")
	if err != nil {
		t.Fatalf("DetectAnomalies 失败: %v", err)
	}

	if !result.HasAnomaly {
		t.Error("高使用率应触发异常")
	}

	// 检查是否有容量异常事件
	hasCapacityEvent := false
	for _, event := range result.Events {
		if event.Type == AnomalyTypeCapacityGrowth {
			hasCapacityEvent = true
			break
		}
	}

	if !hasCapacityEvent {
		t.Error("应包含容量异常事件")
	}
}

func TestDetectIOPSAnomaly(t *testing.T) {
	config := DefaultAnomalyConfig()
	config.MinSamples = 5
	manager := NewAnomalyManager(&config)

	// 添加正常IOPS指标
	for i := 0; i < 8; i++ {
		manager.CollectMetrics(&StorageMetrics{
			DeviceID:    "test-device",
			MountPoint:  "/data",
			ReadIOPS:    100,
			WriteIOPS:   50,
			CollectedAt: time.Now().Add(time.Duration(-8+i) * time.Minute),
		})
	}

	// 添加高IOPS指标
	manager.CollectMetrics(&StorageMetrics{
		DeviceID:    "test-device",
		MountPoint:  "/data",
		ReadIOPS:    500, // 5倍于平均值
		WriteIOPS:   250,
		CollectedAt: time.Now(),
	})

	result, err := manager.DetectAnomalies("test-device")
	if err != nil {
		t.Fatalf("DetectAnomalies 失败: %v", err)
	}

	if !result.HasAnomaly {
		t.Error("高IOPS应触发异常")
	}
}

func TestDetectLatencyAnomaly(t *testing.T) {
	config := DefaultAnomalyConfig()
	config.MinSamples = 5
	manager := NewAnomalyManager(&config)

	// 添加高延迟指标
	for i := 0; i < 10; i++ {
		manager.CollectMetrics(&StorageMetrics{
			DeviceID:     "test-device",
			MountPoint:   "/data",
			ReadLatency:  200, // 超过100ms阈值
			WriteLatency: 150,
			CollectedAt:  time.Now().Add(time.Duration(-10+i) * time.Minute),
		})
	}

	result, err := manager.DetectAnomalies("test-device")
	if err != nil {
		t.Fatalf("DetectAnomalies 失败: %v", err)
	}

	if !result.HasAnomaly {
		t.Error("高延迟应触发异常")
	}
}

func TestDetectDataCorruption(t *testing.T) {
	config := DefaultAnomalyConfig()
	config.MinSamples = 5
	manager := NewAnomalyManager(&config)

	// 添加有数据损坏的指标
	for i := 0; i < 10; i++ {
		manager.CollectMetrics(&StorageMetrics{
			DeviceID:       "test-device",
			MountPoint:     "/data",
			CorruptedFiles: 5,
			ErrorCount:     10,
			CollectedAt:    time.Now().Add(time.Duration(-10+i) * time.Minute),
		})
	}

	result, err := manager.DetectAnomalies("test-device")
	if err != nil {
		t.Fatalf("DetectAnomalies 失败: %v", err)
	}

	if !result.HasAnomaly {
		t.Error("数据损坏应触发异常")
	}
}

func TestCreateRule(t *testing.T) {
	manager := NewAnomalyManager(nil)

	req := &CreateRuleRequest{
		Name:        "自定义规则",
		Description: "测试规则",
		Type:        AnomalyTypeIOPSSpike,
		Severity:    SeverityWarning,
		Conditions: []Condition{
			{Field: "iops", Operator: "gt", Value: 1000},
		},
	}

	rule, err := manager.CreateRule(req)
	if err != nil {
		t.Fatalf("CreateRule 失败: %v", err)
	}

	if rule.ID == "" {
		t.Error("规则ID为空")
	}

	if rule.Name != "自定义规则" {
		t.Errorf("规则名称错误: %s", rule.Name)
	}
}

func TestGetRule(t *testing.T) {
	manager := NewAnomalyManager(nil)

	// 获取默认规则
	rule, err := manager.GetRule("capacity_growth")
	if err != nil {
		t.Fatalf("GetRule 失败: %v", err)
	}

	if rule.ID != "capacity_growth" {
		t.Errorf("规则ID错误: %s", rule.ID)
	}

	// 获取不存在的规则
	_, err = manager.GetRule("non-existent")
	if err == nil {
		t.Error("期望返回错误")
	}
}

func TestListRules(t *testing.T) {
	manager := NewAnomalyManager(nil)

	rules := manager.ListRules()
	if len(rules) < 4 {
		t.Errorf("期望至少 4 个规则, 实际 %d", len(rules))
	}
}

func TestUpdateRule(t *testing.T) {
	manager := NewAnomalyManager(nil)

	enabled := false
	req := &UpdateRuleRequest{
		ID:      "capacity_growth",
		Enabled: &enabled,
	}

	err := manager.UpdateRule(req)
	if err != nil {
		t.Fatalf("UpdateRule 失败: %v", err)
	}

	rule, _ := manager.GetRule("capacity_growth")
	if rule.Enabled {
		t.Error("规则应被禁用")
	}
}

func TestDeleteRule(t *testing.T) {
	manager := NewAnomalyManager(nil)

	// 创建规则
	req := &CreateRuleRequest{
		Name:     "待删除规则",
		Type:     AnomalyTypeIOPSSpike,
		Severity: SeverityWarning,
	}
	rule, _ := manager.CreateRule(req)

	// 删除规则
	err := manager.DeleteRule(rule.ID)
	if err != nil {
		t.Fatalf("DeleteRule 失败: %v", err)
	}

	// 验证已删除
	_, err = manager.GetRule(rule.ID)
	if err == nil {
		t.Error("规则应已删除")
	}
}

func TestListEvents(t *testing.T) {
	config := DefaultAnomalyConfig()
	config.MinSamples = 5 // 降低最小样本数要求
	manager := NewAnomalyManager(&config)

	// 添加一些指标并检测
	for i := 0; i < 15; i++ {
		manager.CollectMetrics(&StorageMetrics{
			DeviceID:     "test-device",
			MountPoint:   "/data",
			UsagePercent: 95.0,
			CollectedAt:  time.Now().Add(time.Duration(-15+i) * time.Minute),
		})
	}

	// 触发检测
	manager.DetectAnomalies("test-device")

	// 列出事件
	events := manager.ListEvents("test-device", "", 10)
	if len(events) == 0 {
		t.Error("应有事件")
	}
}

func TestAckEvent(t *testing.T) {
	config := DefaultAnomalyConfig()
	config.MinSamples = 5
	manager := NewAnomalyManager(&config)

	// 添加指标并检测
	for i := 0; i < 15; i++ {
		manager.CollectMetrics(&StorageMetrics{
			DeviceID:     "test-device",
			MountPoint:   "/data",
			UsagePercent: 95.0,
			CollectedAt:  time.Now().Add(time.Duration(-15+i) * time.Minute),
		})
	}

	manager.DetectAnomalies("test-device")
	events := manager.ListEvents("test-device", "", 1)

	if len(events) > 0 {
		err := manager.AckEvent(events[0].ID, "admin")
		if err != nil {
			t.Fatalf("AckEvent 失败: %v", err)
		}

		event, _ := manager.GetEvent(events[0].ID)
		if event.AckedAt == nil {
			t.Error("事件应已确认")
		}
		if event.AckedBy != "admin" {
			t.Errorf("确认人应为 admin, 实际 %s", event.AckedBy)
		}
	}
}

func TestResolveEvent(t *testing.T) {
	config := DefaultAnomalyConfig()
	config.MinSamples = 5
	manager := NewAnomalyManager(&config)

	// 添加指标并检测
	for i := 0; i < 15; i++ {
		manager.CollectMetrics(&StorageMetrics{
			DeviceID:     "test-device",
			MountPoint:   "/data",
			UsagePercent: 95.0,
			CollectedAt:  time.Now().Add(time.Duration(-15+i) * time.Minute),
		})
	}

	manager.DetectAnomalies("test-device")
	events := manager.ListEvents("test-device", "", 1)

	if len(events) > 0 {
		err := manager.ResolveEvent(events[0].ID)
		if err != nil {
			t.Fatalf("ResolveEvent 失败: %v", err)
		}

		event, _ := manager.GetEvent(events[0].ID)
		if !event.Resolved {
			t.Error("事件应已解决")
		}
		if event.ResolvedAt == nil {
			t.Error("ResolvedAt 不应为 nil")
		}
	}
}

func TestGetStats(t *testing.T) {
	manager := NewAnomalyManager(nil)

	stats := manager.GetStats()

	if stats.ActiveRules < 4 {
		t.Errorf("ActiveRules 期望至少 4, 实际 %d", stats.ActiveRules)
	}
}

func TestAnomalyTypes(t *testing.T) {
	types := []AnomalyType{
		AnomalyTypeAccessPattern,
		AnomalyTypeDataCorruption,
		AnomalyTypeCapacityGrowth,
		AnomalyTypeIOPSSpike,
		AnomalyTypeLatencySpike,
		AnomalyTypeDiskFailure,
	}

	for _, at := range types {
		if at == "" {
			t.Error("AnomalyType 不应为空")
		}
	}
}

func TestAnomalySeverity(t *testing.T) {
	severities := []AnomalySeverity{
		SeverityInfo,
		SeverityWarning,
		SeverityCritical,
		SeverityFatal,
	}

	for _, s := range severities {
		if s == "" {
			t.Error("AnomalySeverity 不应为空")
		}
	}
}

func TestCalculateMean(t *testing.T) {
	tests := []struct {
		values   []float64
		expected float64
	}{
		{[]float64{1, 2, 3, 4, 5}, 3.0},
		{[]float64{10, 20, 30}, 20.0},
		{[]float64{}, 0},
	}

	for _, test := range tests {
		result := calculateMean(test.values)
		if result != test.expected {
			t.Errorf("calculateMean(%v) 期望 %f, 实际 %f", test.values, test.expected, result)
		}
	}
}

func TestCalculateStdDev(t *testing.T) {
	tests := []struct {
		values   []float64
		expected float64
	}{
		{[]float64{2, 4, 4, 4, 5, 5, 7, 9}, 2.0},
		{[]float64{}, 0},
	}

	for _, test := range tests {
		result := calculateStdDev(test.values)
		// 允许小误差
		if result-test.expected > 0.01 || test.expected-result > 0.01 {
			t.Errorf("calculateStdDev(%v) 期望 %f, 实际 %f", test.values, test.expected, result)
		}
	}
}

func TestResetAlertCounts(t *testing.T) {
	manager := NewAnomalyManager(nil)

	manager.alertCounts["test"] = 10
	manager.ResetAlertCounts()

	if len(manager.alertCounts) != 0 {
		t.Error("告警计数应已重置")
	}
}
