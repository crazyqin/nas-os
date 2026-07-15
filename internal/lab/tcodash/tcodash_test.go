package tcodash

import (
	"testing"
	"time"
)

func TestNewDashboard(t *testing.T) {
	d := NewDashboard(0.8, "CNY")
	if d == nil {
		t.Fatal("NewDashboard 返回 nil")
	}
	if d.currency != "CNY" {
		t.Error("货币应为 CNY")
	}
}

func TestAddCostEntry(t *testing.T) {
	d := NewDashboard(0.8, "CNY")
	d.AddCostEntry(&CostEntry{
		ID: "cost-1", Category: CostHardware, Description: "硬盘",
		Amount: 2000, Currency: "CNY", Timestamp: time.Now(),
	})

	report := d.GenerateReport()
	if report.TotalCost != 2000 {
		t.Errorf("总成本应为 2000, 实际 %.2f", report.TotalCost)
	}
}

func TestAddAsset(t *testing.T) {
	d := NewDashboard(0.8, "CNY")
	d.AddAsset(&StorageAsset{
		ID: "disk-1", Name: "WD Red 8TB", Type: "HDD",
		CapacityTB: 8, UsedTB: 5, PowerWatts: 6,
	})

	report := d.GenerateReport()
	if report.TotalCapacityTB != 8 {
		t.Errorf("总容量应为 8TB, 实际 %.1f", report.TotalCapacityTB)
	}
	if report.TotalUsedTB != 5 {
		t.Errorf("已用应为 5TB, 实际 %.1f", report.TotalUsedTB)
	}
	if report.UtilizationPct != 62.5 {
		t.Errorf("利用率应为 62.5%%, 实际 %.1f%%", report.UtilizationPct)
	}
}

func TestMonthlyPowerCost(t *testing.T) {
	d := NewDashboard(0.8, "CNY")
	d.AddAsset(&StorageAsset{
		ID: "disk-1", Name: "WD Red 8TB", CapacityTB: 8, PowerWatts: 10,
	})

	report := d.GenerateReport()
	expected := 10 * 24 * 30 * 0.8 / 1000 // 5.76
	if report.MonthlyPowerCost < expected-0.01 || report.MonthlyPowerCost > expected+0.01 {
		t.Errorf("月度电费应为 %.2f, 实际 %.2f", expected, report.MonthlyPowerCost)
	}
}

func TestCostPerTB(t *testing.T) {
	d := NewDashboard(0.8, "CNY")
	d.AddCostEntry(&CostEntry{
		ID: "cost-1", Category: CostStorage, Amount: 4000, Currency: "CNY",
		Timestamp: time.Now(),
	})
	d.AddAsset(&StorageAsset{
		ID: "disk-1", CapacityTB: 10, UsedTB: 5,
	})

	report := d.GenerateReport()
	expected := 4000.0 / 5.0
	if report.CostPerTB < expected-0.01 || report.CostPerTB > expected+0.01 {
		t.Errorf("每TB成本应为 %.2f, 实际 %.2f", expected, report.CostPerTB)
	}
}

func TestCostBreakdown(t *testing.T) {
	d := NewDashboard(0.8, "CNY")
	d.AddCostEntry(&CostEntry{ID: "1", Category: CostHardware, Amount: 3000, Currency: "CNY", Timestamp: time.Now()})
	d.AddCostEntry(&CostEntry{ID: "2", Category: CostPower, Amount: 1000, Currency: "CNY", Timestamp: time.Now()})
	d.AddCostEntry(&CostEntry{ID: "3", Category: CostHardware, Amount: 2000, Currency: "CNY", Timestamp: time.Now()})

	breakdown := d.GetCostBreakdown()
	if len(breakdown) != 2 {
		t.Errorf("应有 2 个分类, 实际 %d", len(breakdown))
	}
	if breakdown[0].Category != CostHardware {
		t.Error("最高分类应为 hardware")
	}
	if breakdown[0].Amount != 5000 {
		t.Errorf("hardware 金额应为 5000, 实际 %.2f", breakdown[0].Amount)
	}
}

func TestRecordUsage(t *testing.T) {
	d := NewDashboard(0.8, "CNY")
	for i := 0; i < 50; i++ {
		d.RecordUsage(&UsageMetric{
			Timestamp: time.Now().Add(time.Duration(-i) * time.Hour),
			Volume:    "pool1", UsedTB: float64(i), TotalTB: 100,
		})
	}
	report := d.GenerateReport()
	if len(report.Trend) > 50 {
		t.Errorf("趋势数据不应超过 50 条, 实际 %d", len(report.Trend))
	}
}

func TestFormatReport(t *testing.T) {
	d := NewDashboard(0.8, "CNY")
	d.AddAsset(&StorageAsset{ID: "disk-1", CapacityTB: 10, UsedTB: 5, PowerWatts: 10})
	d.AddCostEntry(&CostEntry{ID: "1", Category: CostHardware, Amount: 5000, Currency: "CNY", Timestamp: time.Now()})

	report := d.GenerateReport()
	output := d.FormatReport(report)
	if output == "" {
		t.Error("格式化报告不应为空")
	}
}
