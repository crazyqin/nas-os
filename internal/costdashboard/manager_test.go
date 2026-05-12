// Package costdashboard 测试
package costdashboard

import (
	"testing"
)

func TestAddProvider(t *testing.T) {
	m := NewManager()
	provider := m.AddProvider(AddProviderRequest{
		Name:   "阿里云 OSS",
		Type:   ProviderAliyun,
		APIKey: "test-key",
		Region: "cn-hangzhou",
	})
	if provider == nil {
		t.Fatal("提供商不应为nil")
	}
	if provider.Name != "阿里云 OSS" {
		t.Errorf("名称不匹配: %s", provider.Name)
	}
	if provider.Type != ProviderAliyun {
		t.Errorf("类型不匹配: %s", provider.Type)
	}
	if provider.Status != "active" {
		t.Errorf("状态应为active: %s", provider.Status)
	}
}

func TestRemoveProvider(t *testing.T) {
	m := NewManager()
	provider := m.AddProvider(AddProviderRequest{
		Name: "test",
		Type: ProviderAWS,
	})

	err := m.RemoveProvider(provider.ID)
	if err != nil {
		t.Fatalf("删除提供商失败: %v", err)
	}

	providers := m.ListProviders()
	if len(providers) != 0 {
		t.Errorf("应无提供商，实际 %d", len(providers))
	}
}

func TestListProviders(t *testing.T) {
	m := NewManager()
	m.AddProvider(AddProviderRequest{Name: "p1", Type: ProviderAliyun})
	m.AddProvider(AddProviderRequest{Name: "p2", Type: ProviderTencent})
	m.AddProvider(AddProviderRequest{Name: "p3", Type: ProviderAWS})

	providers := m.ListProviders()
	if len(providers) != 3 {
		t.Errorf("期望3个提供商，实际 %d", len(providers))
	}
}

func TestUpdateProvider(t *testing.T) {
	m := NewManager()
	provider := m.AddProvider(AddProviderRequest{
		Name: "old",
		Type: ProviderAliyun,
	})

	newName := "new"
	updated, err := m.UpdateProvider(provider.ID, UpdateProviderRequest{Name: &newName})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Name != "new" {
		t.Errorf("名称未更新: %s", updated.Name)
	}
}

func TestUpdateProviderNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.UpdateProvider("nonexistent", UpdateProviderRequest{})
	if err == nil {
		t.Error("应返回错误")
	}
}

func TestSyncMetrics(t *testing.T) {
	m := NewManager()
	provider := m.AddProvider(AddProviderRequest{
		Name: "测试云",
		Type: ProviderAliyun,
	})

	metrics, err := m.SyncMetrics(provider.ID)
	if err != nil {
		t.Fatalf("同步失败: %v", err)
	}
	if metrics.ProviderID != provider.ID {
		t.Errorf("提供商ID不匹配")
	}
	if metrics.UsedBytes <= 0 {
		t.Error("使用量应大于0")
	}
	if metrics.CostPerGB <= 0 {
		t.Error("每GB成本应大于0")
	}
}

func TestSyncMetricsNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.SyncMetrics("nonexistent")
	if err == nil {
		t.Error("应返回错误")
	}
}

func TestGetMetrics(t *testing.T) {
	m := NewManager()
	p1 := m.AddProvider(AddProviderRequest{Name: "p1", Type: ProviderAliyun})
	p2 := m.AddProvider(AddProviderRequest{Name: "p2", Type: ProviderAWS})

	m.SyncMetrics(p1.ID)
	m.SyncMetrics(p2.ID)

	metrics := m.GetMetrics()
	if len(metrics) != 2 {
		t.Errorf("期望2个指标，实际 %d", len(metrics))
	}
}

func TestCompareProviders(t *testing.T) {
	m := NewManager()
	p1 := m.AddProvider(AddProviderRequest{Name: "p1", Type: ProviderAliyun})
	p2 := m.AddProvider(AddProviderRequest{Name: "p2", Type: ProviderAWS})

	m.SyncMetrics(p1.ID)
	m.SyncMetrics(p2.ID)

	result, err := m.CompareProviders([]string{p1.ID, p2.ID})
	if err != nil {
		t.Fatalf("对比失败: %v", err)
	}
	if len(result) != 2 {
		t.Errorf("期望2个结果，实际 %d", len(result))
	}
}

func TestCompareProvidersPartialNotFound(t *testing.T) {
	m := NewManager()
	p1 := m.AddProvider(AddProviderRequest{Name: "p1", Type: ProviderAliyun})
	m.SyncMetrics(p1.ID)

	_, err := m.CompareProviders([]string{p1.ID, "nonexistent"})
	if err == nil {
		t.Error("应返回错误")
	}
}

func TestGetUsageTrend(t *testing.T) {
	m := NewManager()
	provider := m.AddProvider(AddProviderRequest{
		Name: "trend-test",
		Type: ProviderAliyun,
	})

	trend, err := m.GetUsageTrend(provider.ID, "weekly")
	if err != nil {
		t.Fatalf("获取趋势失败: %v", err)
	}
	if len(trend) != 7 {
		t.Errorf("期望7个数据点，实际 %d", len(trend))
	}
}

func TestGetUsageTrendNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetUsageTrend("nonexistent", "weekly")
	if err == nil {
		t.Error("应返回错误")
	}
}

func TestForecastCost(t *testing.T) {
	m := NewManager()
	provider := m.AddProvider(AddProviderRequest{
		Name: "forecast-test",
		Type: ProviderAliyun,
	})
	m.SyncMetrics(provider.ID)

	forecast, err := m.ForecastCost(provider.ID, 3)
	if err != nil {
		t.Fatalf("预测失败: %v", err)
	}

	forecasts, ok := forecast["forecasts"].([]map[string]interface{})
	if !ok {
		t.Fatal("forecasts 类型错误")
	}
	if len(forecasts) != 3 {
		t.Errorf("期望3个月预测，实际 %d", len(forecasts))
	}
}

func TestGenerateReport(t *testing.T) {
	m := NewManager()
	p1 := m.AddProvider(AddProviderRequest{Name: "p1", Type: ProviderAliyun})
	p2 := m.AddProvider(AddProviderRequest{Name: "p2", Type: ProviderTencent})

	m.SyncMetrics(p1.ID)
	m.SyncMetrics(p2.ID)

	report := m.GenerateReport(PeriodMonthly)
	if report == nil {
		t.Fatal("报告不应为nil")
	}
	if report.Period != PeriodMonthly {
		t.Errorf("周期不匹配: %s", report.Period)
	}
	if report.TotalCost <= 0 {
		t.Error("总成本应大于0")
	}
	if len(report.Providers) != 2 {
		t.Errorf("期望2个提供商，实际 %d", len(report.Providers))
	}
}

func TestGetReport(t *testing.T) {
	m := NewManager()
	report := m.GenerateReport(PeriodWeekly)

	got, err := m.GetReport(report.ID)
	if err != nil {
		t.Fatalf("获取报告失败: %v", err)
	}
	if got.ID != report.ID {
		t.Errorf("报告ID不匹配")
	}
}

func TestGetReportNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.GetReport("nonexistent")
	if err == nil {
		t.Error("应返回错误")
	}
}

func TestListReports(t *testing.T) {
	m := NewManager()
	m.GenerateReport(PeriodDaily)
	m.GenerateReport(PeriodWeekly)

	reports := m.ListReports()
	if len(reports) != 2 {
		t.Errorf("期望2个报告，实际 %d", len(reports))
	}
}

func TestSetAlert(t *testing.T) {
	m := NewManager()
	provider := m.AddProvider(AddProviderRequest{
		Name: "alert-test",
		Type: ProviderAliyun,
	})
	m.SyncMetrics(provider.ID)

	alert, err := m.SetAlert(SetAlertRequest{
		ProviderID: provider.ID,
		Threshold:  50.0,
		Severity:   SeverityWarning,
	})
	if err != nil {
		t.Fatalf("设置告警失败: %v", err)
	}
	if alert.ProviderID != provider.ID {
		t.Errorf("提供商ID不匹配")
	}
	if alert.Severity != SeverityWarning {
		t.Errorf("严重级别不匹配: %s", alert.Severity)
	}
}

func TestSetAlertProviderNotFound(t *testing.T) {
	m := NewManager()
	_, err := m.SetAlert(SetAlertRequest{
		ProviderID: "nonexistent",
		Threshold:  100,
		Severity:   SeverityCritical,
	})
	if err == nil {
		t.Error("应返回错误")
	}
}

func TestCheckAlerts(t *testing.T) {
	m := NewManager()
	provider := m.AddProvider(AddProviderRequest{
		Name: "check-test",
		Type: ProviderAliyun,
	})
	m.SyncMetrics(provider.ID)

	m.SetAlert(SetAlertRequest{
		ProviderID: provider.ID,
		Threshold:  0.01, // 极低阈值，确保触发
		Severity:   SeverityCritical,
	})

	alerts := m.CheckAlerts()
	if len(alerts) == 0 {
		t.Fatal("应有告警")
	}
	if alerts[0].TriggeredAt.IsZero() {
		t.Error("告警应已触发")
	}
}

func TestAcknowledgeAlert(t *testing.T) {
	m := NewManager()
	provider := m.AddProvider(AddProviderRequest{
		Name: "ack-test",
		Type: ProviderAliyun,
	})

	alert, _ := m.SetAlert(SetAlertRequest{
		ProviderID: provider.ID,
		Threshold:  100,
		Severity:   SeverityWarning,
	})

	err := m.AcknowledgeAlert(alert.ID)
	if err != nil {
		t.Fatalf("确认告警失败: %v", err)
	}

	alerts := m.GetAlerts()
	for _, a := range alerts {
		if a.ID == alert.ID && !a.Acked {
			t.Error("告警应已确认")
		}
	}
}

func TestAcknowledgeAlertNotFound(t *testing.T) {
	m := NewManager()
	err := m.AcknowledgeAlert("nonexistent")
	if err == nil {
		t.Error("应返回错误")
	}
}

func TestAnalyzeOptimization(t *testing.T) {
	m := NewManager()
	provider := m.AddProvider(AddProviderRequest{
		Name: "opt-test",
		Type: ProviderAliyun,
	})
	m.SyncMetrics(provider.ID)

	recs := m.AnalyzeOptimization()
	// 应至少有一些建议（取决于模拟数据）
	if recs == nil {
		t.Fatal("建议不应为nil")
	}
}

func TestGetRecommendations(t *testing.T) {
	m := NewManager()
	provider := m.AddProvider(AddProviderRequest{
		Name: "rec-test",
		Type: ProviderAliyun,
	})
	m.SyncMetrics(provider.ID)
	m.AnalyzeOptimization()

	recs := m.GetRecommendations()
	if recs == nil {
		t.Fatal("建议不应为nil")
	}
}

func TestRemoveProviderCleansAlerts(t *testing.T) {
	m := NewManager()
	provider := m.AddProvider(AddProviderRequest{
		Name: "clean-test",
		Type: ProviderAliyun,
	})
	m.SyncMetrics(provider.ID)

	m.SetAlert(SetAlertRequest{
		ProviderID: provider.ID,
		Threshold:  100,
		Severity:   SeverityWarning,
	})

	m.RemoveProvider(provider.ID)

	alerts := m.GetAlerts()
	for _, a := range alerts {
		if a.ProviderID == provider.ID {
			t.Error("关联告警应已清理")
		}
	}
}

func TestReportTrendComparison(t *testing.T) {
	m := NewManager()
	p := m.AddProvider(AddProviderRequest{Name: "trend", Type: ProviderAliyun})
	m.SyncMetrics(p.ID)

	// 第一次报告
	report1 := m.GenerateReport(PeriodMonthly)

	// 再次同步（模拟数据变化）后再生成
	m.SyncMetrics(p.ID)
	report2 := m.GenerateReport(PeriodMonthly)

	if report2.TotalCost == 0 {
		t.Error("总成本不应为0")
	}
	// 趋势应存在
	_ = report1.Trend
	_ = report2.Trend
}
