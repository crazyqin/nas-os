// Package storageanomaly 测试
package storageanomaly

import (
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("manager should not be nil")
	}

	config := m.GetConfig()
	if !config.Enabled {
		t.Error("default config should be enabled")
	}
	if config.DeviationFactor != 3.0 {
		t.Errorf("expected deviation factor 3.0, got %f", config.DeviationFactor)
	}
}

func TestDefaultRules(t *testing.T) {
	m := NewManager()

	rules := m.ListRules()
	if len(rules) != 6 {
		t.Errorf("expected 6 default rules, got %d", len(rules))
	}

	// 检查规则类型覆盖
	types := make(map[string]bool)
	for _, r := range rules {
		types[r.EventType] = true
	}
	expected := []string{"write_spike", "size_anomaly", "access_pattern", "data_leak", "hw_failure", "malware"}
	for _, et := range expected {
		if !types[et] {
			t.Errorf("missing default rule for type %s", et)
		}
	}
}

func TestIngestSample(t *testing.T) {
	m := NewManager()

	req := IngestSampleRequest{
		Path:       "/data/test",
		WriteBytes: 1024,
		ReadBytes:  512,
		FileCount:  10,
		AccessOps:  20,
	}

	m.IngestSample(req)

	count := m.GetSampleCount("/data/test")
	if count != 1 {
		t.Errorf("expected 1 sample, got %d", count)
	}

	// 再导入一个
	m.IngestSample(req)
	count = m.GetSampleCount("/data/test")
	if count != 2 {
		t.Errorf("expected 2 samples, got %d", count)
	}

	// 不同路径
	m.IngestSample(IngestSampleRequest{Path: "/data/other", WriteBytes: 2048})
	if m.GetSampleCount("/data/other") != 1 {
		t.Error("should have 1 sample for /data/other")
	}
}

func TestBuildBaseline(t *testing.T) {
	m := NewManager()

	// 无样本应报错
	_, err := m.BuildBaseline("/empty")
	if err == nil {
		t.Error("expected error for empty path")
	}

	// 导入多个样本
	for i := 0; i < 20; i++ {
		m.IngestSample(IngestSampleRequest{
			Path:       "/data/test",
			WriteBytes: int64(1000 + i*10),
			ReadBytes:  int64(500 + i*5),
			FileCount:  10 + i,
			AccessOps:  20 + i,
		})
	}

	baseline, err := m.BuildBaseline("/data/test")
	if err != nil {
		t.Fatalf("build baseline failed: %v", err)
	}

	if baseline.Path != "/data/test" {
		t.Errorf("expected path /data/test, got %s", baseline.Path)
	}
	if baseline.SampleCount != 20 {
		t.Errorf("expected 20 samples, got %d", baseline.SampleCount)
	}
	if baseline.AvgWriteBytes <= 0 {
		t.Error("avg write rate should be positive")
	}
	if baseline.AvgReadBytes <= 0 {
		t.Error("avg read rate should be positive")
	}
	if baseline.LastUpdated.IsZero() {
		t.Error("last updated should be set")
	}
}

func TestGetBaseline(t *testing.T) {
	m := NewManager()

	// 不存在应报错
	_, err := m.GetBaseline("/nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent baseline")
	}

	// 创建基线
	m.IngestSample(IngestSampleRequest{Path: "/data/test", WriteBytes: 1000, ReadBytes: 500, FileCount: 10, AccessOps: 20})
	m.BuildBaseline("/data/test")

	baseline, err := m.GetBaseline("/data/test")
	if err != nil {
		t.Fatalf("get baseline failed: %v", err)
	}
	if baseline.Path != "/data/test" {
		t.Errorf("expected path /data/test, got %s", baseline.Path)
	}
}

func TestListBaselines(t *testing.T) {
	m := NewManager()

	if len(m.ListBaselines()) != 0 {
		t.Error("should have no baselines initially")
	}

	// 创建两个基线
	m.IngestSample(IngestSampleRequest{Path: "/a", WriteBytes: 100})
	m.BuildBaseline("/a")
	m.IngestSample(IngestSampleRequest{Path: "/b", WriteBytes: 200})
	m.BuildBaseline("/b")

	baselines := m.ListBaselines()
	if len(baselines) != 2 {
		t.Errorf("expected 2 baselines, got %d", len(baselines))
	}
}

func TestDetectAnomaly(t *testing.T) {
	m := NewManager()

	// 建立基线：稳定写入 1000 bytes
	for i := 0; i < 30; i++ {
		m.IngestSample(IngestSampleRequest{
			Path:       "/data/stable",
			WriteBytes: 1000,
			ReadBytes:  500,
			FileCount:  10,
			AccessOps:  20,
		})
	}
	bl, err := m.BuildBaseline("/data/stable")
	if err != nil {
		t.Fatalf("build baseline failed: %v", err)
	}
	_ = bl

	// 正常数据不应触发异常
	events, err := m.DetectAnomaly("/data/stable", SampleDataPoint{
		Timestamp:  time.Now(),
		WriteBytes: 1000,
		ReadBytes:  500,
		FileCount:  10,
		AccessOps:  20,
	})
	if err != nil {
		t.Fatalf("detect failed: %v", err)
	}
	if len(events) != 0 {
		t.Errorf("expected 0 anomalies for normal data, got %d", len(events))
	}

	// 极端异常数据应触发
	events, err = m.DetectAnomaly("/data/stable", SampleDataPoint{
		Timestamp:  time.Now(),
		WriteBytes: 100000, // 100x 正常值
		ReadBytes:  50000,
		FileCount:  1000,
		AccessOps:  2000,
	})
	if err != nil {
		t.Fatalf("detect failed: %v", err)
	}
	if len(events) == 0 {
		t.Error("expected anomalies for extreme data")
	}

	// 验证事件类型
	hasWriteSpike := false
	for _, evt := range events {
		if evt.EventType == "write_spike" {
			hasWriteSpike = true
		}
		if evt.ID == "" {
			t.Error("event should have ID")
		}
		if evt.Timestamp.IsZero() {
			t.Error("event should have timestamp")
		}
		if evt.Deviation <= 0 {
			t.Error("deviation should be positive")
		}
	}
	if !hasWriteSpike {
		t.Error("should detect write spike")
	}
}

func TestDetectNoBaseline(t *testing.T) {
	m := NewManager()

	_, err := m.DetectAnomaly("/no/baseline", SampleDataPoint{
		WriteBytes: 1000,
	})
	if err == nil {
		t.Error("expected error when no baseline exists")
	}
}

func TestDetectDisabled(t *testing.T) {
	m := NewManager()

	// 建立基线
	m.IngestSample(IngestSampleRequest{Path: "/data", WriteBytes: 100})
	m.BuildBaseline("/data")

	// 禁用检测
	enabled := false
	m.UpdateConfig(UpdateConfigRequest{Enabled: &enabled})

	events, err := m.DetectAnomaly("/data", SampleDataPoint{WriteBytes: 999999})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if events != nil {
		t.Error("should return nil when disabled")
	}
}

func TestClassifyEvent(t *testing.T) {
	m := NewManager()

	evt := &AnomalyEvent{
		EventType: "write_spike",
		Deviation: 3.0,
	}
	severity := m.ClassifyEvent(evt)
	if severity != "medium" {
		t.Errorf("expected medium for deviation 3.0, got %s", severity)
	}

	evt.Deviation = 7.0
	severity = m.ClassifyEvent(evt)
	if severity != "critical" {
		t.Errorf("expected critical for deviation 7.0, got %s", severity)
	}

	evt.Deviation = 5.0
	severity = m.ClassifyEvent(evt)
	if severity != "high" {
		t.Errorf("expected high for deviation 5.0, got %s", severity)
	}
}

func TestAutoRespond(t *testing.T) {
	m := NewManager()

	evt := &AnomalyEvent{
		EventType: "write_spike",
		Severity:  "critical",
		Deviation: 10.0,
	}

	resp := m.AutoRespond(evt)
	if resp == "" {
		t.Error("response should not be empty")
	}
	if evt.Response == "" {
		t.Error("event response should be set")
	}

	// 测试不同事件类型
	evt.EventType = "data_leak"
	evt.Severity = "high"
	resp = m.AutoRespond(evt)
	if resp == "" {
		t.Error("response should not be empty for data_leak")
	}

	evt.EventType = "malware"
	evt.Severity = "critical"
	resp = m.AutoRespond(evt)
	if resp == "" {
		t.Error("response should not be empty for malware")
	}
}

func TestGetEvent(t *testing.T) {
	m := NewManager()

	// 构建基线并触发异常
	for i := 0; i < 20; i++ {
		m.IngestSample(IngestSampleRequest{Path: "/data", WriteBytes: 100, ReadBytes: 50, FileCount: 5, AccessOps: 10})
	}
	m.BuildBaseline("/data")

	events, _ := m.DetectAnomaly("/data", SampleDataPoint{
		WriteBytes: 99999,
		ReadBytes:  50000,
		FileCount:  500,
		AccessOps:  1000,
	})

	if len(events) > 0 {
		evt, err := m.GetEvent(events[0].ID)
		if err != nil {
			t.Fatalf("get event failed: %v", err)
		}
		if evt.ID != events[0].ID {
			t.Error("event ID mismatch")
		}
	}
}

func TestGetEventNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetEvent("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent event")
	}
}

func TestListEvents(t *testing.T) {
	m := NewManager()

	// 构建基线
	for i := 0; i < 20; i++ {
		m.IngestSample(IngestSampleRequest{Path: "/data", WriteBytes: 100, ReadBytes: 50, FileCount: 5, AccessOps: 10})
	}
	m.BuildBaseline("/data")

	// 触发多次异常
	for i := 0; i < 5; i++ {
		m.DetectAnomaly("/data", SampleDataPoint{
			WriteBytes: 99999,
			ReadBytes:  50000,
			FileCount:  500,
			AccessOps:  1000,
		})
	}

	// 列出全部
	all := m.ListEvents(0, "", "")
	if len(all) == 0 {
		t.Error("should have events")
	}

	// 带限制
	limited := m.ListEvents(2, "", "")
	if len(limited) > 2 {
		t.Errorf("expected at most 2, got %d", len(limited))
	}

	// 按类型过滤
	writeEvents := m.ListEvents(0, "", "write_spike")
	for _, evt := range writeEvents {
		if evt.EventType != "write_spike" {
			t.Error("filtered events should only contain write_spike")
		}
	}
}

func TestResolveEvent(t *testing.T) {
	m := NewManager()

	// 构建基线
	for i := 0; i < 20; i++ {
		m.IngestSample(IngestSampleRequest{Path: "/data", WriteBytes: 100, ReadBytes: 50, FileCount: 5, AccessOps: 10})
	}
	m.BuildBaseline("/data")

	events, _ := m.DetectAnomaly("/data", SampleDataPoint{
		WriteBytes: 99999,
		ReadBytes:  50000,
		FileCount:  500,
		AccessOps:  1000,
	})

	if len(events) > 0 {
		err := m.ResolveEvent(events[0].ID)
		if err != nil {
			t.Fatalf("resolve event failed: %v", err)
		}

		evt, _ := m.GetEvent(events[0].ID)
		if !evt.Resolved {
			t.Error("event should be resolved")
		}
		if evt.ResolvedAt == nil {
			t.Error("resolved_at should be set")
		}
	}
}

func TestResolveEventNotFound(t *testing.T) {
	m := NewManager()

	err := m.ResolveEvent("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent event")
	}
}

func TestGetStats(t *testing.T) {
	m := NewManager()

	stats := m.GetStats()
	if stats.TotalEvents != 0 {
		t.Error("should have 0 events initially")
	}
	if stats.Unresolved != 0 {
		t.Error("should have 0 unresolved initially")
	}

	// 构建基线并触发异常
	for i := 0; i < 20; i++ {
		m.IngestSample(IngestSampleRequest{Path: "/data", WriteBytes: 100, ReadBytes: 50, FileCount: 5, AccessOps: 10})
	}
	m.BuildBaseline("/data")

	m.DetectAnomaly("/data", SampleDataPoint{
		WriteBytes: 99999,
		ReadBytes:  50000,
		FileCount:  500,
		AccessOps:  1000,
	})

	stats = m.GetStats()
	if stats.TotalEvents == 0 {
		t.Error("should have events after detection")
	}
	if stats.BySeverity == nil {
		t.Error("by_severity should not be nil")
	}
	if stats.ByType == nil {
		t.Error("by_type should not be nil")
	}
	if stats.LastEventTime == nil {
		t.Error("last event time should be set")
	}
}

func TestAddRule(t *testing.T) {
	m := NewManager()

	rule := m.AddRule(AddRuleRequest{
		Name:        "自定义规则",
		EventType:   "custom_type",
		Threshold:   5.0,
		MinSamples:  10,
		Description: "测试规则",
	})

	if rule.ID == "" {
		t.Error("rule should have ID")
	}
	if rule.Name != "自定义规则" {
		t.Errorf("expected name 自定义规则, got %s", rule.Name)
	}
	if !rule.Enabled {
		t.Error("rule should be enabled by default")
	}
	if rule.Threshold != 5.0 {
		t.Errorf("expected threshold 5.0, got %f", rule.Threshold)
	}

	// 默认值
	rule2 := m.AddRule(AddRuleRequest{
		Name:      "默认值规则",
		EventType: "test",
	})
	if rule2.Threshold != 3.0 {
		t.Errorf("expected default threshold 3.0, got %f", rule2.Threshold)
	}
	if rule2.MinSamples != 5 {
		t.Errorf("expected default min_samples 5, got %d", rule2.MinSamples)
	}
}

func TestToggleRule(t *testing.T) {
	m := NewManager()

	rules := m.ListRules()
	ruleID := rules[0].ID

	err := m.ToggleRule(ruleID, false)
	if err != nil {
		t.Fatalf("toggle rule failed: %v", err)
	}

	// 验证
	for _, r := range m.ListRules() {
		if r.ID == ruleID && r.Enabled {
			t.Error("rule should be disabled")
		}
	}

	// 重新启用
	m.ToggleRule(ruleID, true)
	for _, r := range m.ListRules() {
		if r.ID == ruleID && !r.Enabled {
			t.Error("rule should be enabled")
		}
	}
}

func TestToggleRuleNotFound(t *testing.T) {
	m := NewManager()

	err := m.ToggleRule("nonexistent", true)
	if err == nil {
		t.Error("expected error for nonexistent rule")
	}
}

func TestRemoveRule(t *testing.T) {
	m := NewManager()

	rule := m.AddRule(AddRuleRequest{
		Name:      "to remove",
		EventType: "test",
	})
	initialCount := len(m.ListRules())

	err := m.RemoveRule(rule.ID)
	if err != nil {
		t.Fatalf("remove rule failed: %v", err)
	}

	if len(m.ListRules()) != initialCount-1 {
		t.Error("rule count should decrease by 1")
	}
}

func TestRemoveRuleNotFound(t *testing.T) {
	m := NewManager()

	err := m.RemoveRule("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent rule")
	}
}

func TestUpdateConfig(t *testing.T) {
	m := NewManager()

	enabled := false
	interval := 600
	factor := 2.5
	age := 48
	autoRespond := false
	threshold := "high"
	maxEvents := 50

	m.UpdateConfig(UpdateConfigRequest{
		Enabled:          &enabled,
		ScanInterval:     &interval,
		DeviationFactor:  &factor,
		MinBaselineAge:   &age,
		AutoRespond:      &autoRespond,
		AlertThreshold:   &threshold,
		MaxEventsPerHour: &maxEvents,
	})

	config := m.GetConfig()
	if config.Enabled {
		t.Error("should be disabled")
	}
	if config.ScanInterval != 600 {
		t.Errorf("expected 600, got %d", config.ScanInterval)
	}
	if config.DeviationFactor != 2.5 {
		t.Errorf("expected 2.5, got %f", config.DeviationFactor)
	}
	if config.MinBaselineAge != 48 {
		t.Errorf("expected 48, got %d", config.MinBaselineAge)
	}
	if config.AutoRespond {
		t.Error("auto respond should be false")
	}
	if config.AlertThreshold != "high" {
		t.Errorf("expected high, got %s", config.AlertThreshold)
	}
	if config.MaxEventsPerHour != 50 {
		t.Errorf("expected 50, got %d", config.MaxEventsPerHour)
	}
}

func TestUpdateConfigPartial(t *testing.T) {
	m := NewManager()

	original := m.GetConfig()

	interval := 120
	m.UpdateConfig(UpdateConfigRequest{ScanInterval: &interval})

	updated := m.GetConfig()
	if updated.ScanInterval != 120 {
		t.Errorf("expected 120, got %d", updated.ScanInterval)
	}
	// 其他字段应不变
	if updated.Enabled != original.Enabled {
		t.Error("enabled should not change")
	}
	if updated.DeviationFactor != original.DeviationFactor {
		t.Error("deviation factor should not change")
	}
}

func TestSeverityClassification(t *testing.T) {
	tests := []struct {
		deviation float64
		expected  string
	}{
		{1.0, "low"},
		{2.5, "low"},
		{3.0, "medium"},
		{4.0, "medium"},
		{4.5, "high"},
		{5.5, "high"},
		{6.0, "critical"},
		{10.0, "critical"},
	}

	for _, tt := range tests {
		result := classifySeverity(tt.deviation)
		if result != tt.expected {
			t.Errorf("classifySeverity(%f) = %s, want %s", tt.deviation, result, tt.expected)
		}
	}
}

func TestComputeStdDev(t *testing.T) {
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	mean := 5.0

	std := computeStdDev(values, mean)
	// 标准差应约为 2.0
	if std < 1.9 || std > 2.1 {
		t.Errorf("expected std dev ~2.0, got %f", std)
	}

	// 空数组
	if computeStdDev(nil, 0) != 0 {
		t.Error("empty array should return 0")
	}
}

func TestComputeStdDevFromValues(t *testing.T) {
	values := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	std := computeStdDevFromValues(values)
	if std < 1.9 || std > 2.1 {
		t.Errorf("expected std dev ~2.0, got %f", std)
	}

	if computeStdDevFromValues(nil) != 0 {
		t.Error("nil should return 0")
	}
}

func TestConcurrentAccess(t *testing.T) {
	m := NewManager()

	// 导入样本
	for i := 0; i < 30; i++ {
		m.IngestSample(IngestSampleRequest{Path: "/data", WriteBytes: 1000, ReadBytes: 500, FileCount: 10, AccessOps: 20})
	}
	m.BuildBaseline("/data")

	done := make(chan bool, 10)

	// 并发检测
	for i := 0; i < 5; i++ {
		go func() {
			m.DetectAnomaly("/data", SampleDataPoint{
				WriteBytes: 99999,
				ReadBytes:  50000,
				FileCount:  500,
				AccessOps:  1000,
			})
			done <- true
		}()
	}

	// 并发读取
	for i := 0; i < 5; i++ {
		go func() {
			m.GetStats()
			m.ListEvents(10, "", "")
			m.ListRules()
			done <- true
		}()
	}

	for i := 0; i < 10; i++ {
		<-done
	}
}
