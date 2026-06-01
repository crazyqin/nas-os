package smarthealthpredict

import (
	"context"
	"testing"
	"time"
)

func TestNewManager(t *testing.T) {
	t.Run("创建成功", func(t *testing.T) {
		manager, err := NewManager("/tmp/test-smart")
		if err != nil {
			t.Fatalf("创建管理器失败: %v", err)
		}
		if manager == nil {
			t.Fatal("管理器为 nil")
		}
	})

	t.Run("路径为空", func(t *testing.T) {
		_, err := NewManager("")
		if err != ErrPathRequired {
			t.Fatalf("期望 ErrPathRequired，得到 %v", err)
		}
	})

	t.Run("自定义选项", func(t *testing.T) {
		manager, err := NewManager("/tmp/test-smart",
			WithModel(ModelMLBased),
			WithRetentionDays(30),
			WithPredictInterval(12*time.Hour),
		)
		if err != nil {
			t.Fatalf("创建管理器失败: %v", err)
		}
		if manager.model != ModelMLBased {
			t.Errorf("模型不匹配: got %v, want %v", manager.model, ModelMLBased)
		}
		if manager.retentionDays != 30 {
			t.Errorf("保留天数不匹配: got %d, want 30", manager.retentionDays)
		}
	})
}

func TestScanDisk(t *testing.T) {
	manager, err := NewManager("/tmp/test-smart-scan")
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	report, err := manager.ScanDisk(context.Background(), "/dev/sda")
	if err != nil {
		t.Fatalf("扫描磁盘失败: %v", err)
	}

	if report == nil {
		t.Fatal("报告为 nil")
	}

	if report.Score < 0 || report.Score > 100 {
		t.Errorf("健康评分超出范围: %d", report.Score)
	}

	if report.Status == "" {
		t.Error("健康状态为空")
	}

	if report.Timestamp.IsZero() {
		t.Error("时间戳为零值")
	}
}

func TestHealthStatus(t *testing.T) {
	manager, err := NewManager("/tmp/test-smart-status")
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	tests := []struct {
		score    int
		expected HealthStatus
	}{
		{95, StatusExcellent},
		{80, StatusGood},
		{60, StatusFair},
		{40, StatusPoor},
		{20, StatusCritical},
	}

	for _, tt := range tests {
		status := manager.getHealthStatus(tt.score)
		if status != tt.expected {
			t.Errorf("score=%d: got %v, want %v", tt.score, status, tt.expected)
		}
	}
}

func TestAlertRules(t *testing.T) {
	manager, err := NewManager("/tmp/test-smart-alerts")
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 测试温度告警
	info := DiskInfo{
		Device:      "/dev/sda",
		Temperature: 65,
		PowerOn:     1000,
		SMARTPassed: true,
	}

	attrs := []SMARTAttribute{
		{Name: "Temperature_Celsius", RawValue: 65},
	}

	alerts := manager.checkAlerts(info, attrs, 80)
	if len(alerts) == 0 {
		t.Error("期望有温度告警，但没有")
	}

	// 测试健康分告警
	info.Temperature = 30
	alerts = manager.checkAlerts(info, attrs, 25)
	foundHealthAlert := false
	for _, a := range alerts {
		if a.Type == "health" {
			foundHealthAlert = true
			break
		}
	}
	if !foundHealthAlert {
		t.Error("期望有健康分告警，但没有")
	}
}

func TestTrendAnalysis(t *testing.T) {
	manager, err := NewManager("/tmp/test-smart-trend")
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	device := "/dev/sda"

	// 添加历史数据
	for i := 0; i < 10; i++ {
		manager.history[device] = append(manager.history[device], HealthSnapshot{
			Timestamp:   time.Now().AddDate(0, 0, -10+i),
			Score:       90 - i*2,
			Temperature: 40 + i,
			PowerOn:     8000 + int64(i*100),
		})
	}

	trend := manager.analyzeTrend(device)
	if trend == nil {
		t.Fatal("趋势分析为 nil")
	}

	if trend.HealthTrend != "degrading" {
		t.Errorf("期望 degrading，得到 %v", trend.HealthTrend)
	}
}

func TestPredictions(t *testing.T) {
	manager, err := NewManager("/tmp/test-smart-predict")
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	info := DiskInfo{
		Device: "/dev/sda",
		Type:   DiskTypeHDD,
		Health: 80,
	}

	attrs := []SMARTAttribute{
		{Name: "Reallocated_Sector_Ct", RawValue: 150, Critical: true},
	}

	trend := &TrendAnalysis{
		HealthTrend: "stable",
		DeclineRate: 0,
	}

	predictions := manager.predictFailures("/dev/sda", info, attrs, trend)
	if len(predictions) == 0 {
		t.Error("期望有故障预测，但没有")
	}
}

func TestGetDiskList(t *testing.T) {
	manager, err := NewManager("/tmp/test-smart-list")
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 扫描两个磁盘
	manager.ScanDisk(context.Background(), "/dev/sda")
	manager.ScanDisk(context.Background(), "/dev/sdb")

	disks := manager.GetDiskList()
	if len(disks) != 2 {
		t.Errorf("期望 2 个磁盘，得到 %d", len(disks))
	}
}

func TestGetDiskHistory(t *testing.T) {
	manager, err := NewManager("/tmp/test-smart-history")
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	device := "/dev/sda"

	// 扫描磁盘生成历史
	manager.ScanDisk(context.Background(), device)

	history := manager.GetDiskHistory(device, 30)
	if len(history) == 0 {
		t.Error("期望有历史数据，但没有")
	}
}

func TestGetAlerts(t *testing.T) {
	manager, err := NewManager("/tmp/test-smart-get-alerts")
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	// 扫描磁盘
	manager.ScanDisk(context.Background(), "/dev/sda")

	alerts := manager.GetAlerts()
	// 告警数量取决于模拟数据
	t.Logf("获取到 %d 个告警", len(alerts))
}

func TestHandler(t *testing.T) {
	manager, err := NewManager("/tmp/test-smart-handler")
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	handler := NewHandler(manager)
	if handler == nil {
		t.Fatal("处理器为 nil")
	}
}
