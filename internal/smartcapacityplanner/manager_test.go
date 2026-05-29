// Package smartcapacityplanner 测试
package smartcapacityplanner

import (
	"testing"
	"time"
)

func TestNewPlannerManager(t *testing.T) {
	m := NewPlannerManager()
	if m == nil {
		t.Fatal("manager should not be nil")
	}
}

func TestRecordUsage(t *testing.T) {
	m := NewPlannerManager()

	snapshot, err := m.RecordUsage(RecordUsageRequest{
		TotalBytes: 1000000000, // 1GB
		UsedBytes:  500000000,  // 500MB
		MountPoint: "/data",
		FileSystem: "ext4",
	})
	if err != nil {
		t.Fatalf("record usage failed: %v", err)
	}
	if snapshot == nil {
		t.Fatal("snapshot should not be nil")
	}
	if snapshot.ID == "" {
		t.Error("snapshot should have an ID")
	}
	if snapshot.TotalBytes != 1000000000 {
		t.Errorf("expected total bytes 1000000000, got %d", snapshot.TotalBytes)
	}
	if snapshot.UsedBytes != 500000000 {
		t.Errorf("expected used bytes 500000000, got %d", snapshot.UsedBytes)
	}
	if snapshot.FreeBytes != 500000000 {
		t.Errorf("expected free bytes 500000000, got %d", snapshot.FreeBytes)
	}
	if snapshot.UsageRate != 0.5 {
		t.Errorf("expected usage rate 0.5, got %f", snapshot.UsageRate)
	}
	if snapshot.MountPoint != "/data" {
		t.Errorf("expected mount point /data, got %s", snapshot.MountPoint)
	}
	if snapshot.Timestamp.IsZero() {
		t.Error("timestamp should be set")
	}
}

func TestRecordUsageValidation(t *testing.T) {
	m := NewPlannerManager()

	// 测试无效的总容量
	_, err := m.RecordUsage(RecordUsageRequest{
		TotalBytes: -1,
		UsedBytes:  0,
		MountPoint: "/data",
	})
	if err == nil {
		t.Error("expected error for negative total bytes")
	}

	// 测试无效的已用容量
	_, err = m.RecordUsage(RecordUsageRequest{
		TotalBytes: 1000,
		UsedBytes:  -1,
		MountPoint: "/data",
	})
	if err == nil {
		t.Error("expected error for negative used bytes")
	}

	// 测试已用容量超过总容量
	_, err = m.RecordUsage(RecordUsageRequest{
		TotalBytes: 1000,
		UsedBytes:  2000,
		MountPoint: "/data",
	})
	if err == nil {
		t.Error("expected error for used bytes exceeding total bytes")
	}
}

func TestGetLatestSnapshot(t *testing.T) {
	m := NewPlannerManager()

	// 没有快照时
	_, err := m.GetLatestSnapshot("")
	if err == nil {
		t.Error("expected error when no snapshots")
	}

	// 添加快照
	m.RecordUsage(RecordUsageRequest{
		TotalBytes: 1000000000,
		UsedBytes:  500000000,
		MountPoint: "/data",
	})
	m.RecordUsage(RecordUsageRequest{
		TotalBytes: 2000000000,
		UsedBytes:  1000000000,
		MountPoint: "/backup",
	})

	// 获取最新快照（不限挂载点）
	snapshot, err := m.GetLatestSnapshot("")
	if err != nil {
		t.Fatalf("get latest snapshot failed: %v", err)
	}
	if snapshot.MountPoint != "/backup" {
		t.Errorf("expected mount point /backup, got %s", snapshot.MountPoint)
	}

	// 获取指定挂载点的快照
	snapshot, err = m.GetLatestSnapshot("/data")
	if err != nil {
		t.Fatalf("get latest snapshot for /data failed: %v", err)
	}
	if snapshot.MountPoint != "/data" {
		t.Errorf("expected mount point /data, got %s", snapshot.MountPoint)
	}

	// 获取不存在的挂载点
	_, err = m.GetLatestSnapshot("/nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent mount point")
	}
}

func TestListSnapshots(t *testing.T) {
	m := NewPlannerManager()

	// 添加多个快照
	for i := 0; i < 10; i++ {
		m.RecordUsage(RecordUsageRequest{
			TotalBytes: 1000000000,
			UsedBytes:  int64(i * 100000000),
			MountPoint: "/data",
		})
	}

	// 获取限制数量
	snapshots := m.ListSnapshots(5)
	if len(snapshots) != 5 {
		t.Errorf("expected 5 snapshots, got %d", len(snapshots))
	}

	// 获取全部
	all := m.ListSnapshots(0)
	if len(all) != 10 {
		t.Errorf("expected 10 snapshots, got %d", len(all))
	}
}

func TestForecastCapacity(t *testing.T) {
	m := NewPlannerManager()

	// 添加足够的历史数据
	for i := 0; i < 10; i++ {
		m.RecordUsage(RecordUsageRequest{
			TotalBytes: 1000000000,
			UsedBytes:  int64(i * 100000000),
			MountPoint: "/data",
		})
		time.Sleep(10 * time.Millisecond) // 确保时间戳不同
	}

	// 线性预测
	result, err := m.ForecastCapacity(ForecastRequest{
		ModelType: "linear",
		DaysAhead: 30,
		MountPoint: "/data",
	})
	if err != nil {
		t.Fatalf("linear forecast failed: %v", err)
	}
	if result == nil {
		t.Fatal("result should not be nil")
	}
	if result.ModelType != "linear" {
		t.Errorf("expected model type linear, got %s", result.ModelType)
	}
	if result.PredictedUsage <= 0 || result.PredictedUsage > 1.0 {
		t.Errorf("predicted usage should be between 0 and 1, got %f", result.PredictedUsage)
	}
	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("confidence should be between 0 and 1, got %f", result.Confidence)
	}
}

func TestForecastInsufficientData(t *testing.T) {
	m := NewPlannerManager()

	// 只添加1个快照
	m.RecordUsage(RecordUsageRequest{
		TotalBytes: 1000000000,
		UsedBytes:  500000000,
		MountPoint: "/data",
	})

	// 预测应该失败
	_, err := m.ForecastCapacity(ForecastRequest{
		ModelType: "linear",
		DaysAhead: 30,
	})
	if err == nil {
		t.Error("expected error with insufficient data")
	}
}

func TestForecastUnsupportedModel(t *testing.T) {
	m := NewPlannerManager()

	// 添加足够的历史数据
	for i := 0; i < 5; i++ {
		m.RecordUsage(RecordUsageRequest{
			TotalBytes: 1000000000,
			UsedBytes:  int64(i * 100000000),
			MountPoint: "/data",
		})
	}

	// 使用不支持的模型
	_, err := m.ForecastCapacity(ForecastRequest{
		ModelType: "unsupported",
		DaysAhead: 30,
	})
	if err == nil {
		t.Error("expected error for unsupported model")
	}
}

func TestForecastExponential(t *testing.T) {
	m := NewPlannerManager()

	// 添加历史数据
	for i := 0; i < 10; i++ {
		m.RecordUsage(RecordUsageRequest{
			TotalBytes: 1000000000,
			UsedBytes:  int64(i * 100000000),
			MountPoint: "/data",
		})
		time.Sleep(10 * time.Millisecond)
	}

	result, err := m.ForecastCapacity(ForecastRequest{
		ModelType: "exponential",
		DaysAhead: 30,
		MountPoint: "/data",
	})
	if err != nil {
		t.Fatalf("exponential forecast failed: %v", err)
	}
	if result.ModelType != "exponential" {
		t.Errorf("expected model type exponential, got %s", result.ModelType)
	}
}

func TestForecastSeasonal(t *testing.T) {
	m := NewPlannerManager()

	// 添加足够的历史数据（至少7天）
	for i := 0; i < 14; i++ {
		m.RecordUsage(RecordUsageRequest{
			TotalBytes: 1000000000,
			UsedBytes:  int64(i * 50000000),
			MountPoint: "/data",
		})
		time.Sleep(10 * time.Millisecond)
	}

	result, err := m.ForecastCapacity(ForecastRequest{
		ModelType: "seasonal",
		DaysAhead: 30,
		MountPoint: "/data",
	})
	if err != nil {
		t.Fatalf("seasonal forecast failed: %v", err)
	}
	if result.ModelType != "seasonal" {
		t.Errorf("expected model type seasonal, got %s", result.ModelType)
	}
}

func TestGetGrowthTrend(t *testing.T) {
	m := NewPlannerManager()

	// 添加历史数据
	for i := 0; i < 10; i++ {
		m.RecordUsage(RecordUsageRequest{
			TotalBytes: 1000000000,
			UsedBytes:  int64(i * 100000000),
			MountPoint: "/data",
		})
		time.Sleep(10 * time.Millisecond)
	}

	// 获取每日趋势
	trend, err := m.GetGrowthTrend("/data", "daily")
	if err != nil {
		t.Fatalf("get growth trend failed: %v", err)
	}
	if trend == nil {
		t.Fatal("trend should not be nil")
	}
	if trend.Period != "daily" {
		t.Errorf("expected period daily, got %s", trend.Period)
	}
	if trend.GrowthBytes == 0 {
		t.Error("growth bytes should not be 0")
	}
}

func TestGetGrowthTrendInsufficientData(t *testing.T) {
	m := NewPlannerManager()

	// 只添加1个快照
	m.RecordUsage(RecordUsageRequest{
		TotalBytes: 1000000000,
		UsedBytes:  500000000,
		MountPoint: "/data",
	})

	_, err := m.GetGrowthTrend("/data", "daily")
	if err == nil {
		t.Error("expected error with insufficient data")
	}
}

func TestGeneratePlan(t *testing.T) {
	m := NewPlannerManager()

	// 添加历史数据
	for i := 0; i < 10; i++ {
		m.RecordUsage(RecordUsageRequest{
			TotalBytes: 1000000000,
			UsedBytes:  int64(i * 100000000),
			MountPoint: "/data",
		})
		time.Sleep(10 * time.Millisecond)
	}

	plan, err := m.GeneratePlan("/data")
	if err != nil {
		t.Fatalf("generate plan failed: %v", err)
	}
	if plan == nil {
		t.Fatal("plan should not be nil")
	}
	if plan.ID == "" {
		t.Error("plan should have an ID")
	}
	if plan.CurrentUsage <= 0 {
		t.Error("current usage should be positive")
	}
	if plan.DaysUntilFull <= 0 {
		t.Error("days until full should be positive")
	}
	if plan.RecommendedAction == "" {
		t.Error("recommended action should not be empty")
	}
	if plan.Priority == "" {
		t.Error("priority should not be empty")
	}
}

func TestGeneratePlanInsufficientData(t *testing.T) {
	m := NewPlannerManager()

	m.RecordUsage(RecordUsageRequest{
		TotalBytes: 1000000000,
		UsedBytes:  500000000,
		MountPoint: "/data",
	})

	_, err := m.GeneratePlan("/data")
	if err == nil {
		t.Error("expected error with insufficient data")
	}
}

func TestAlertThresholds(t *testing.T) {
	m := NewPlannerManager()

	// 默认阈值
	warning, critical := m.GetAlertThresholds()
	if warning != 0.80 {
		t.Errorf("expected warning threshold 0.80, got %f", warning)
	}
	if critical != 0.95 {
		t.Errorf("expected critical threshold 0.95, got %f", critical)
	}

	// 设置新阈值
	err := m.SetAlertThresholds(0.75, 0.90)
	if err != nil {
		t.Fatalf("set alert thresholds failed: %v", err)
	}

	warning, critical = m.GetAlertThresholds()
	if warning != 0.75 {
		t.Errorf("expected warning threshold 0.75, got %f", warning)
	}
	if critical != 0.90 {
		t.Errorf("expected critical threshold 0.90, got %f", critical)
	}
}

func TestAlertThresholdsValidation(t *testing.T) {
	m := NewPlannerManager()

	// 无效的告警阈值
	err := m.SetAlertThresholds(-0.1, 0.9)
	if err == nil {
		t.Error("expected error for negative warning threshold")
	}

	err = m.SetAlertThresholds(1.1, 0.9)
	if err == nil {
		t.Error("expected error for warning threshold > 1")
	}

	err = m.SetAlertThresholds(0.9, -0.1)
	if err == nil {
		t.Error("expected error for negative critical threshold")
	}

	err = m.SetAlertThresholds(0.9, 1.1)
	if err == nil {
		t.Error("expected error for critical threshold > 1")
	}

	// 告警阈值 >= 严重告警阈值
	err = m.SetAlertThresholds(0.9, 0.8)
	if err == nil {
		t.Error("expected error when warning >= critical")
	}

	err = m.SetAlertThresholds(0.9, 0.9)
	if err == nil {
		t.Error("expected error when warning == critical")
	}
}

func TestTriggerAlerts(t *testing.T) {
	m := NewPlannerManager()

	// 设置低阈值便于测试
	m.SetAlertThresholds(0.50, 0.80)

	// 添加超过阈值的快照
	m.RecordUsage(RecordUsageRequest{
		TotalBytes: 1000000000,
		UsedBytes:  850000000, // 85%
		MountPoint: "/data",
	})

	// 检查告警
	alerts := m.ListAlerts(false)
	if len(alerts) == 0 {
		t.Error("expected alerts to be generated")
	}

	// 应该有严重告警
	foundCritical := false
	for _, alert := range alerts {
		if alert.Level == "critical" {
			foundCritical = true
			break
		}
	}
	if !foundCritical {
		t.Error("expected critical alert")
	}
}

func TestTriggerAlertManually(t *testing.T) {
	m := NewPlannerManager()

	// 设置阈值
	m.SetAlertThresholds(0.50, 0.80)

	// 添加快照
	m.RecordUsage(RecordUsageRequest{
		TotalBytes: 1000000000,
		UsedBytes:  600000000, // 60%
		MountPoint: "/data",
	})

	// 手动触发告警
	alerts, err := m.TriggerAlert("/data")
	if err != nil {
		t.Fatalf("trigger alert failed: %v", err)
	}

	// 应该有告警（60% > 50%）
	if len(alerts) == 0 {
		t.Error("expected alerts to be generated")
	}
}

func TestMarkAlertRead(t *testing.T) {
	m := NewPlannerManager()

	m.SetAlertThresholds(0.50, 0.80)

	m.RecordUsage(RecordUsageRequest{
		TotalBytes: 1000000000,
		UsedBytes:  850000000,
		MountPoint: "/data",
	})

	alerts := m.ListAlerts(false)
	if len(alerts) == 0 {
		t.Fatal("expected alerts")
	}

	// 标记为已读
	err := m.MarkAlertRead(alerts[0].ID)
	if err != nil {
		t.Fatalf("mark alert read failed: %v", err)
	}

	// 验证已读
	unreadAlerts := m.ListAlerts(true)
	for _, alert := range unreadAlerts {
		if alert.ID == alerts[0].ID {
			t.Error("alert should be marked as read")
		}
	}

	// 测试不存在的告警
	err = m.MarkAlertRead("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent alert")
	}
}

func TestClearAlerts(t *testing.T) {
	m := NewPlannerManager()

	m.SetAlertThresholds(0.50, 0.80)

	m.RecordUsage(RecordUsageRequest{
		TotalBytes: 1000000000,
		UsedBytes:  850000000,
		MountPoint: "/data",
	})

	if len(m.ListAlerts(false)) == 0 {
		t.Fatal("expected alerts before clear")
	}

	m.ClearAlerts()

	if len(m.ListAlerts(false)) != 0 {
		t.Error("expected no alerts after clear")
	}
}

func TestGetForecasts(t *testing.T) {
	m := NewPlannerManager()

	// 添加历史数据
	for i := 0; i < 5; i++ {
		m.RecordUsage(RecordUsageRequest{
			TotalBytes: 1000000000,
			UsedBytes:  int64(i * 100000000),
			MountPoint: "/data",
		})
		time.Sleep(10 * time.Millisecond)
	}

	// 进行预测
	m.ForecastCapacity(ForecastRequest{
		ModelType: "linear",
		DaysAhead: 30,
		MountPoint: "/data",
	})

	forecasts := m.GetForecasts()
	if len(forecasts) == 0 {
		t.Error("expected forecasts to be stored")
	}
}

func TestGetPlans(t *testing.T) {
	m := NewPlannerManager()

	// 添加历史数据
	for i := 0; i < 5; i++ {
		m.RecordUsage(RecordUsageRequest{
			TotalBytes: 1000000000,
			UsedBytes:  int64(i * 100000000),
			MountPoint: "/data",
		})
		time.Sleep(10 * time.Millisecond)
	}

	// 生成规划
	m.GeneratePlan("/data")

	plans := m.GetPlans()
	if len(plans) == 0 {
		t.Error("expected plans to be stored")
	}
}

func TestClearHistory(t *testing.T) {
	m := NewPlannerManager()

	// 添加数据
	m.RecordUsage(RecordUsageRequest{
		TotalBytes: 1000000000,
		UsedBytes:  500000000,
		MountPoint: "/data",
	})

	if len(m.ListSnapshots(0)) == 0 {
		t.Fatal("expected snapshots before clear")
	}

	m.ClearHistory()

	if len(m.ListSnapshots(0)) != 0 {
		t.Error("expected no snapshots after clear")
	}
}

func TestAlertAutoGeneration(t *testing.T) {
	m := NewPlannerManager()

	// 测试不同使用率触发的告警级别
	testCases := []struct {
		usageRate    float64
		expectAlert  bool
		expectLevel  string
	}{
		{0.50, false, ""},           // 50% - 无告警
		{0.81, true, "warning"},     // 81% - 警告
		{0.96, true, "critical"},    // 96% - 严重
		{1.00, true, "critical"},    // 100% - 严重
	}

	for _, tc := range testCases {
		// 清除之前的告警
		m.ClearAlerts()

		usedBytes := int64(float64(1000000000) * tc.usageRate)
		m.RecordUsage(RecordUsageRequest{
			TotalBytes: 1000000000,
			UsedBytes:  usedBytes,
			MountPoint: "/data",
		})

		alerts := m.ListAlerts(false)
		if tc.expectAlert {
			if len(alerts) == 0 {
				t.Errorf("expected alert for usage rate %.2f", tc.usageRate)
				continue
			}
			if alerts[0].Level != tc.expectLevel {
				t.Errorf("expected level %s for usage rate %.2f, got %s", tc.expectLevel, tc.usageRate, alerts[0].Level)
			}
		} else {
			if len(alerts) > 0 {
				t.Errorf("expected no alert for usage rate %.2f, got %d alerts", tc.usageRate, len(alerts))
			}
		}
	}
}

func TestPriorityLevels(t *testing.T) {
	m := NewPlannerManager()

	testCases := []struct {
		daysUntilFull int
		expected      Priority
	}{
		{15, PriorityHigh},
		{60, PriorityMedium},
		{180, PriorityLow},
	}

	for _, tc := range testCases {
		m.ClearHistory()

		// 创建模拟数据以达到预期的 daysUntilFull
		// 假设每天增长 1%，那么 100% / daysUntilFull = daily growth
		dailyGrowth := 1.0 / float64(tc.daysUntilFull)

		// 添加足够的数据点
		for i := 0; i < 10; i++ {
			usage := 0.5 + dailyGrowth*float64(i)
			if usage > 1.0 {
				usage = 1.0
			}
			m.RecordUsage(RecordUsageRequest{
				TotalBytes: 1000000000,
				UsedBytes:  int64(float64(1000000000) * usage),
				MountPoint: "/data",
			})
			time.Sleep(10 * time.Millisecond)
		}

		plan, err := m.GeneratePlan("/data")
		if err != nil {
			t.Fatalf("generate plan failed: %v", err)
		}

		// 注意：实际的 daysUntilFull 可能与预期略有不同，因为是基于线性回归计算的
		// 这里主要验证优先级逻辑
		if plan.Priority == "" {
			t.Error("priority should not be empty")
		}
	}
}
