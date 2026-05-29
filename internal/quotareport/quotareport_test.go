package quotareport

import (
	"testing"
	"time"
)

func TestNewQuotaManager(t *testing.T) {
	mgr := NewQuotaManager(nil)
	if mgr == nil {
		t.Fatal("expected non-nil manager")
	}

	config := mgr.GetConfig()
	if !config.Enabled {
		t.Error("expected manager to be enabled by default")
	}
}

func TestAddAndGetQuota(t *testing.T) {
	mgr := NewQuotaManager(nil)

	quota := &QuotaEntry{
		ID:          "quota1",
		Type:        QuotaTypeUser,
		Name:        "user1",
		TargetID:    "1000",
		HardLimit:   1024 * 1024 * 1024, // 1GB
		SoftLimit:   800 * 1024 * 1024,  // 800MB
		GracePeriod: 7,
	}

	if err := mgr.AddQuota(quota); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, exists := mgr.GetQuota("quota1")
	if !exists {
		t.Fatal("expected quota to exist")
	}
	if got.Name != "user1" {
		t.Errorf("expected name 'user1', got %q", got.Name)
	}
}

func TestUpdateUsage(t *testing.T) {
	mgr := NewQuotaManager(nil)

	quota := &QuotaEntry{
		ID:        "quota1",
		Type:      QuotaTypeUser,
		Name:      "user1",
		HardLimit: 1024 * 1024 * 1024, // 1GB
	}

	mgr.AddQuota(quota)

	// 更新使用量
	if err := mgr.UpdateUsage("quota1", 500*1024*1024, 100); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	got, _ := mgr.GetQuota("quota1")
	if got.CurrentUsage != 500*1024*1024 {
		t.Errorf("expected usage 500MB, got %d", got.CurrentUsage)
	}
	if got.Status != QuotaStatusNormal {
		t.Errorf("expected normal status, got %v", got.Status)
	}
}

func TestQuotaWarningStatus(t *testing.T) {
	mgr := NewQuotaManager(nil)

	quota := &QuotaEntry{
		ID:        "quota1",
		Type:      QuotaTypeUser,
		Name:      "user1",
		HardLimit: 1024 * 1024 * 1024, // 1GB
	}

	mgr.AddQuota(quota)

	// 设置使用量超过警告阈值 (80%)
	mgr.UpdateUsage("quota1", 850*1024*1024, 100)

	got, _ := mgr.GetQuota("quota1")
	if got.Status != QuotaStatusWarning {
		t.Errorf("expected warning status, got %v", got.Status)
	}
}

func TestQuotaExceededStatus(t *testing.T) {
	mgr := NewQuotaManager(nil)

	quota := &QuotaEntry{
		ID:        "quota1",
		Type:      QuotaTypeUser,
		Name:      "user1",
		HardLimit: 1024 * 1024 * 1024, // 1GB
	}

	mgr.AddQuota(quota)

	// 设置使用量超过限制
	mgr.UpdateUsage("quota1", 1100*1024*1024, 100)

	got, _ := mgr.GetQuota("quota1")
	if got.Status != QuotaStatusExceeded {
		t.Errorf("expected exceeded status, got %v", got.Status)
	}
}

func TestDeleteQuota(t *testing.T) {
	mgr := NewQuotaManager(nil)

	quota := &QuotaEntry{
		ID:   "quota1",
		Name: "user1",
	}

	mgr.AddQuota(quota)

	if err := mgr.DeleteQuota("quota1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, exists := mgr.GetQuota("quota1")
	if exists {
		t.Error("expected quota to be deleted")
	}
}

func TestListQuotas(t *testing.T) {
	mgr := NewQuotaManager(nil)

	mgr.AddQuota(&QuotaEntry{
		ID:   "quota1",
		Type: QuotaTypeUser,
		Name: "user1",
	})

	mgr.AddQuota(&QuotaEntry{
		ID:   "quota2",
		Type: QuotaTypeGroup,
		Name: "group1",
	})

	// 列出所有
	all := mgr.ListQuotas(nil)
	if len(all) != 2 {
		t.Errorf("expected 2 quotas, got %d", len(all))
	}

	// 按类型过滤
	userType := QuotaTypeUser
	users := mgr.ListQuotas(&userType)
	if len(users) != 1 {
		t.Errorf("expected 1 user quota, got %d", len(users))
	}
}

func TestGenerateReport(t *testing.T) {
	mgr := NewQuotaManager(nil)

	mgr.AddQuota(&QuotaEntry{
		ID:          "quota1",
		Type:        QuotaTypeUser,
		Name:        "user1",
		HardLimit:   1024 * 1024 * 1024,
		CurrentUsage: 500 * 1024 * 1024,
		Status:      QuotaStatusNormal,
	})

	period := ReportPeriod{
		Start: time.Now().Add(-24 * time.Hour),
		End:   time.Now(),
		Type:  "daily",
	}

	report := mgr.GenerateReport(period)
	if report == nil {
		t.Fatal("expected non-nil report")
	}
	if report.TotalQuotas != 1 {
		t.Errorf("expected 1 quota, got %d", report.TotalQuotas)
	}
}

func TestGetAlerts(t *testing.T) {
	mgr := NewQuotaManager(nil)

	quota := &QuotaEntry{
		ID:        "quota1",
		Type:      QuotaTypeUser,
		Name:      "user1",
		HardLimit: 1024 * 1024 * 1024,
	}

	mgr.AddQuota(quota)

	// 触发警告
	mgr.UpdateUsage("quota1", 850*1024*1024, 100)

	alerts := mgr.GetAlerts(10)
	if len(alerts) == 0 {
		t.Error("expected alerts to be generated")
	}
}

func TestGetStats(t *testing.T) {
	mgr := NewQuotaManager(nil)

	mgr.AddQuota(&QuotaEntry{
		ID:   "quota1",
		Type: QuotaTypeUser,
		Name: "user1",
	})

	mgr.AddQuota(&QuotaEntry{
		ID:   "quota2",
		Type: QuotaTypeGroup,
		Name: "group1",
	})

	stats := mgr.GetStats()
	totalQuotas := stats["total_quotas"].(int)
	if totalQuotas != 2 {
		t.Errorf("expected 2 quotas, got %d", totalQuotas)
	}
}
