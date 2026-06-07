package monitor

import (
	"testing"
	"time"
)

func TestNewActivityMonitor(t *testing.T) {
	monitor := NewActivityMonitor(nil, nil)
	if monitor == nil {
		t.Fatal("ActivityMonitor should not be nil")
	}
	if monitor.config == nil {
		t.Fatal("Config should not be nil")
	}
	if monitor.tracker == nil {
		t.Fatal("Tracker should not be nil")
	}
	if monitor.detector == nil {
		t.Fatal("Detector should not be nil")
	}
}

func TestRecordActivity(t *testing.T) {
	monitor := NewActivityMonitor(nil, nil)

	activity := &FileActivity{
		ID:        "test-1",
		Path:      "/test/file.txt",
		EventType: ActivityCreate,
		User:      "testuser",
		Size:      1024,
		Timestamp: time.Now(),
	}

	err := monitor.RecordActivity(activity)
	if err != nil {
		t.Fatalf("RecordActivity failed: %v", err)
	}

	// 验证记录已保存
	history := monitor.GetActivityHistory(&ActivityFilter{Limit: 10})
	if len(history) != 1 {
		t.Errorf("Expected 1 record, got %d", len(history))
	}
}

func TestActivityStatistics(t *testing.T) {
	monitor := NewActivityMonitor(nil, nil)

	// 记录多个活动
	for i := 0; i < 5; i++ {
		activity := &FileActivity{
			ID:        "test-" + string(rune('0'+i)),
			Path:      "/test/file.txt",
			EventType: ActivityModify,
			User:      "testuser",
			Size:      1024,
			Timestamp: time.Now(),
		}
		monitor.RecordActivity(activity)
	}

	stats := monitor.GetStatistics()
	if stats.TotalActivities != 5 {
		t.Errorf("Expected 5 total activities, got %d", stats.TotalActivities)
	}

	if stats.EventCounts["modify"] != 5 {
		t.Errorf("Expected 5 modify events, got %d", stats.EventCounts["modify"])
	}
}

func TestAnomalyDetection(t *testing.T) {
	config := DefaultActivityConfig()
	config.EnableAnomalyDetection = true
	monitor := NewActivityMonitor(config, nil)

	// 大量删除（应该触发异常）
	activity := &FileActivity{
		ID:        "mass-delete",
		Path:      "/test/files",
		EventType: ActivityDelete,
		User:      "suspicious",
		Size:      500, // 超过阈值
		Timestamp: time.Now(),
	}

	monitor.RecordActivity(activity)

	// 检查是否检测到异常
	history := monitor.GetActivityHistory(&ActivityFilter{IsAnomaly: true})
	if len(history) == 0 {
		t.Error("Expected anomaly to be detected for mass delete")
	}
}

func TestSuspiciousFileDetection(t *testing.T) {
	config := DefaultActivityConfig()
	config.EnableAnomalyDetection = true
	monitor := NewActivityMonitor(config, nil)

	// 创建可疑文件类型
	activity := &FileActivity{
		ID:        "suspicious-exe",
		Path:      "/test/malware.exe",
		EventType: ActivityCreate,
		User:      "testuser",
		FileType:  ".exe",
		Timestamp: time.Now(),
	}

	monitor.RecordActivity(activity)

	// 应该检测为异常
	if !activity.IsAnomaly {
		t.Error("Expected .exe file creation to be flagged as anomaly")
	}
}

func TestUnusualTimeDetection(t *testing.T) {
	config := DefaultActivityConfig()
	config.EnableAnomalyDetection = true
	monitor := NewActivityMonitor(config, nil)

	// 在非工作时间（2:00 AM）的操作
	unusualTime := time.Date(2026, 4, 5, 2, 0, 0, 0, time.Local)
	activity := &FileActivity{
		ID:        "night-activity",
		Path:      "/test/sensitive.txt",
		EventType: ActivityModify,
		User:      "testuser",
		Size:      1024,
		Timestamp: unusualTime,
	}

	monitor.RecordActivity(activity)

	// 应该检测为异常时间
	if !activity.IsAnomaly {
		t.Error("Expected activity at 2AM to be flagged as unusual time")
	}
}

func TestActivityFilter(t *testing.T) {
	monitor := NewActivityMonitor(nil, nil)

	// 记录不同类型的活动
	activities := []*FileActivity{
		{ID: "1", Path: "/path1", EventType: ActivityCreate, User: "user1"},
		{ID: "2", Path: "/path2", EventType: ActivityDelete, User: "user1"},
		{ID: "3", Path: "/path1", EventType: ActivityModify, User: "user2"},
	}

	for _, a := range activities {
		monitor.RecordActivity(a)
	}

	// 按路径过滤
	filteredByPath := monitor.GetActivityHistory(&ActivityFilter{Path: "/path1"})
	if len(filteredByPath) != 2 {
		t.Errorf("Expected 2 activities for /path1, got %d", len(filteredByPath))
	}

	// 按用户过滤
	filteredByUser := monitor.GetActivityHistory(&ActivityFilter{User: "user1"})
	if len(filteredByUser) != 2 {
		t.Errorf("Expected 2 activities for user1, got %d", len(filteredByUser))
	}

	// 按事件类型过滤
	filteredByType := monitor.GetActivityHistory(&ActivityFilter{EventTypes: []ActivityType{ActivityDelete}})
	if len(filteredByType) != 1 {
		t.Errorf("Expected 1 delete activity, got %d", len(filteredByType))
	}
}

func TestAlertManagement(t *testing.T) {
	monitor := NewActivityMonitor(nil, nil)

	// 创建一个会触发告警的活动
	activity := &FileActivity{
		ID:        "alert-test",
		Path:      "/test/encrypted.locked",
		EventType: ActivityModify,
		User:      "suspicious",
		FileType:  ".locked", // 勒索软件特征
		Timestamp: time.Now(),
	}

	monitor.RecordActivity(activity)

	// 检查告警是否创建
	alerts := monitor.GetAnomalyAlerts("", 10)
	if len(alerts) == 0 {
		t.Error("Expected alert to be created for ransomware-like activity")
	}

	// 清除告警
	if len(alerts) > 0 {
		err := monitor.ClearAlert(alerts[0].ID)
		if err != nil {
			t.Errorf("ClearAlert failed: %v", err)
		}
	}
}

func TestActivityTracker(t *testing.T) {
	tracker := NewActivityTracker(100)

	// 记录活动
	for i := 0; i < 50; i++ {
		activity := &FileActivity{
			ID:        "test-" + string(rune('0'+i%10)),
			Path:      "/test/file.txt",
			EventType: ActivityModify,
			User:      "testuser",
			Timestamp: time.Now(),
		}
		tracker.Record(activity)
	}

	stats := tracker.GetStatistics()
	if stats.TotalActivities != 50 {
		t.Errorf("Expected 50 activities, got %d", stats.TotalActivities)
	}
}

func TestAnomalyDetector(t *testing.T) {
	config := DefaultActivityConfig()
	detector := NewAnomalyDetector(config)

	// 测试各种异常检测
	tests := []struct {
		name     string
		activity *FileActivity
		expect   bool
	}{
		{
			name: "normal activity",
			activity: &FileActivity{
				EventType: ActivityRead,
				User:      "normal",
				// 使用工作时间（10:00）避免非工作时间误判
				Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			expect: false,
		},
		{
			name: "suspicious file",
			activity: &FileActivity{
				EventType: ActivityCreate,
				User:      "suspicious",
				FileType:  ".exe",
				// 使用工作时间避免非工作时间误判
				Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			expect: true,
		},
		{
			name: "ransomware indicator",
			activity: &FileActivity{
				EventType: ActivityModify,
				User:      "malware",
				FileType:  ".encrypted",
				// 使用工作时间避免非工作时间误判
				Timestamp: time.Date(2024, 1, 15, 10, 0, 0, 0, time.UTC),
			},
			expect: true,
		},
	}

	for _, tt := range tests {
		result := detector.DetectAnomaly(tt.activity)
		if result.IsAnomaly != tt.expect {
			t.Errorf("%s: expected IsAnomaly=%v, got %v", tt.name, tt.expect, result.IsAnomaly)
		}
	}
}

func TestAlertManager(t *testing.T) {
	am := NewAlertManager()

	// 创建告警
	alert := &AnomalyAlert{
		ID:        "alert-1",
		Type:      AnomalyMassDelete,
		Severity:  SeverityHigh,
		Path:      "/test",
		User:      "test",
		Timestamp: time.Now(),
	}

	am.CreateAlert(alert)

	// 获取告警
	alerts := am.GetAlerts("", 10)
	if len(alerts) != 1 {
		t.Errorf("Expected 1 alert, got %d", len(alerts))
	}

	// 清除告警
	am.ClearAlert("alert-1")

	alerts = am.GetAlerts("", 10)
	if len(alerts) != 1 {
		t.Errorf("Alert should still exist after clearing")
	}
}

func TestCleanup(t *testing.T) {
	monitor := NewActivityMonitor(nil, nil)

	// 记录一个旧活动
	oldActivity := &FileActivity{
		ID:        "old",
		Path:      "/test/old.txt",
		EventType: ActivityCreate,
		User:      "test",
		Timestamp: time.Now().AddDate(0, 0, -60), // 60天前
	}
	monitor.RecordActivity(oldActivity)

	// 记录一个新活动
	newActivity := &FileActivity{
		ID:        "new",
		Path:      "/test/new.txt",
		EventType: ActivityCreate,
		User:      "test",
		Timestamp: time.Now(),
	}
	monitor.RecordActivity(newActivity)

	// 执行清理
	monitor.cleanupOldRecords()

	// 检查结果
	history := monitor.GetActivityHistory(&ActivityFilter{})
	if len(history) != 1 {
		t.Errorf("Expected 1 activity after cleanup, got %d", len(history))
	}
}

func BenchmarkRecordActivity(b *testing.B) {
	monitor := NewActivityMonitor(nil, nil)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		activity := &FileActivity{
			ID:        "bench-" + string(rune(i)),
			Path:      "/test/file.txt",
			EventType: ActivityModify,
			User:      "benchmark",
			Size:      1024,
			Timestamp: time.Now(),
		}
		monitor.RecordActivity(activity)
	}
}

func BenchmarkAnomalyDetection(b *testing.B) {
	config := DefaultActivityConfig()
	config.EnableAnomalyDetection = true
	monitor := NewActivityMonitor(config, nil)

	activity := &FileActivity{
		ID:        "bench",
		Path:      "/test/suspicious.exe",
		EventType: ActivityCreate,
		User:      "benchmark",
		FileType:  ".exe",
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		monitor.RecordActivity(activity)
	}
}
