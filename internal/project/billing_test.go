package project

import (
	"testing"
	"time"
)

func TestBillingManager_SetQuota(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	// 先创建项目
	project, err := mgr.CreateProject("Test Project", "TP", "Test Description", "user1", "user1")
	if err != nil {
		t.Fatalf("Failed to create project: %v", err)
	}

	// 设置CPU配额
	quota, err := billingMgr.SetQuota(project.ID, ResourceTypeCPU, 100, 80, 0.5, "CNY")
	if err != nil {
		t.Fatalf("Failed to set quota: %v", err)
	}

	if quota.HardLimit != 100 {
		t.Errorf("Expected hard limit 100, got %d", quota.HardLimit)
	}
	if quota.SoftLimit != 80 {
		t.Errorf("Expected soft limit 80, got %d", quota.SoftLimit)
	}
	if quota.UnitPrice != 0.5 {
		t.Errorf("Expected unit price 0.5, got %f", quota.UnitPrice)
	}
	if quota.Currency != "CNY" {
		t.Errorf("Expected currency CNY, got %s", quota.Currency)
	}
}

func TestBillingManager_GetQuota(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")
	billingMgr.SetQuota(project.ID, ResourceTypeCPU, 100, 80, 0.5, "CNY")

	// 获取配额
	quota, err := billingMgr.GetQuota(project.ID, ResourceTypeCPU)
	if err != nil {
		t.Fatalf("Failed to get quota: %v", err)
	}

	if quota.ResourceType != ResourceTypeCPU {
		t.Errorf("Expected resource type cpu, got %s", quota.ResourceType)
	}

	// 获取不存在的配额
	_, err = billingMgr.GetQuota(project.ID, ResourceTypeMemory)
	if err != ErrQuotaNotFound {
		t.Errorf("Expected ErrQuotaNotFound, got %v", err)
	}
}

func TestBillingManager_ListQuotas(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")
	billingMgr.SetQuota(project.ID, ResourceTypeCPU, 100, 80, 0.5, "CNY")
	billingMgr.SetQuota(project.ID, ResourceTypeMemory, 1024, 800, 0.1, "CNY")

	quotas := billingMgr.ListQuotas(project.ID)
	if len(quotas) != 2 {
		t.Errorf("Expected 2 quotas, got %d", len(quotas))
	}
}

func TestBillingManager_AllocateResource(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")
	billingMgr.SetQuota(project.ID, ResourceTypeCPU, 100, 80, 0.5, "CNY")

	// 分配资源
	err := billingMgr.AllocateResource(project.ID, ResourceTypeCPU, 50, "user1", "Test allocation")
	if err != nil {
		t.Fatalf("Failed to allocate resource: %v", err)
	}

	// 检查使用量
	quota, _ := billingMgr.GetQuota(project.ID, ResourceTypeCPU)
	if quota.Used != 50 {
		t.Errorf("Expected used 50, got %d", quota.Used)
	}
}

func TestBillingManager_AllocateResource_ExceedQuota(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")
	billingMgr.SetQuota(project.ID, ResourceTypeCPU, 100, 80, 0.5, "CNY")

	// 尝试分配超过配额的资源
	err := billingMgr.AllocateResource(project.ID, ResourceTypeCPU, 150, "user1", "Test allocation")
	if err != ErrQuotaExceeded {
		t.Errorf("Expected ErrQuotaExceeded, got %v", err)
	}
}

func TestBillingManager_ReleaseResource(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")
	billingMgr.SetQuota(project.ID, ResourceTypeCPU, 100, 80, 0.5, "CNY")
	billingMgr.AllocateResource(project.ID, ResourceTypeCPU, 50, "user1", "Test allocation")

	// 释放资源
	err := billingMgr.ReleaseResource(project.ID, ResourceTypeCPU, 30, "user1", "Test release")
	if err != nil {
		t.Fatalf("Failed to release resource: %v", err)
	}

	quota, _ := billingMgr.GetQuota(project.ID, ResourceTypeCPU)
	if quota.Used != 20 {
		t.Errorf("Expected used 20, got %d", quota.Used)
	}
}

func TestBillingManager_GetQuotaUsage(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")
	billingMgr.SetQuota(project.ID, ResourceTypeCPU, 100, 80, 0.5, "CNY")
	billingMgr.AllocateResource(project.ID, ResourceTypeCPU, 50, "user1", "Allocation 1")
	billingMgr.AllocateResource(project.ID, ResourceTypeCPU, 20, "user1", "Allocation 2")

	usage := billingMgr.GetQuotaUsage(project.ID, 10)
	if len(usage) != 2 {
		t.Errorf("Expected 2 usage records, got %d", len(usage))
	}
}

func TestBillingManager_CreateBillingRecord(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")

	now := time.Now()
	periodStart := now.AddDate(0, 0, -30)
	periodEnd := now

	resourceCosts := []ResourceCost{
		{
			ResourceType: ResourceTypeCPU,
			Quantity:     100,
			UnitPrice:    0.5,
			UsageHours:   720,
			Subtotal:     360,
		},
		{
			ResourceType: ResourceTypeMemory,
			Quantity:     1024,
			UnitPrice:    0.1,
			UsageHours:   720,
			Subtotal:     72,
		},
	}

	record, err := billingMgr.CreateBillingRecord(project.ID, periodStart, periodEnd, resourceCosts, 0.1, 0.1)
	if err != nil {
		t.Fatalf("Failed to create billing record: %v", err)
	}

	if record.TotalCost != 432 {
		t.Errorf("Expected total cost 432, got %f", record.TotalCost)
	}
	if record.Status != "draft" {
		t.Errorf("Expected status draft, got %s", record.Status)
	}
}

func TestBillingManager_GetBillingRecord(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")

	now := time.Now()
	record, _ := billingMgr.CreateBillingRecord(
		project.ID,
		now.AddDate(0, 0, -30),
		now,
		[]ResourceCost{
			{ResourceType: ResourceTypeCPU, Quantity: 100, UnitPrice: 0.5, UsageHours: 720, Subtotal: 360},
		},
		0, 0,
	)

	// 获取记录
	fetched, err := billingMgr.GetBillingRecord(record.ID)
	if err != nil {
		t.Fatalf("Failed to get billing record: %v", err)
	}

	if fetched.ID != record.ID {
		t.Errorf("Expected ID %s, got %s", record.ID, fetched.ID)
	}
}

func TestBillingManager_ListBillingRecords(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")

	now := time.Now()
	for i := 0; i < 3; i++ {
		billingMgr.CreateBillingRecord(
			project.ID,
			now.AddDate(0, -i-1, 0),
			now.AddDate(0, -i, 0),
			[]ResourceCost{
				{ResourceType: ResourceTypeCPU, Quantity: 100, UnitPrice: 0.5, UsageHours: 720, Subtotal: 360},
			},
			0, 0,
		)
	}

	records := billingMgr.ListBillingRecords(project.ID, 10, 0)
	if len(records) != 3 {
		t.Errorf("Expected 3 records, got %d", len(records))
	}
}

func TestBillingManager_UpdateBillingStatus(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")

	now := time.Now()
	record, _ := billingMgr.CreateBillingRecord(
		project.ID,
		now.AddDate(0, 0, -30),
		now,
		[]ResourceCost{
			{ResourceType: ResourceTypeCPU, Quantity: 100, UnitPrice: 0.5, UsageHours: 720, Subtotal: 360},
		},
		0, 0,
	)

	// 更新状态
	paidAt := time.Now()
	err := billingMgr.UpdateBillingStatus(record.ID, "paid", &paidAt)
	if err != nil {
		t.Fatalf("Failed to update billing status: %v", err)
	}

	fetched, _ := billingMgr.GetBillingRecord(record.ID)
	if fetched.Status != "paid" {
		t.Errorf("Expected status paid, got %s", fetched.Status)
	}
	if fetched.PaidAt == nil {
		t.Error("Expected paid_at to be set")
	}
}

func TestBillingManager_GetCostAnalysis(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")

	now := time.Now()
	periodStart := now.AddDate(0, 0, -30)
	periodEnd := now

	// 创建一些计费记录
	billingMgr.CreateBillingRecord(
		project.ID,
		periodStart,
		periodEnd,
		[]ResourceCost{
			{ResourceType: ResourceTypeCPU, Quantity: 100, UnitPrice: 0.5, UsageHours: 720, Subtotal: 360},
			{ResourceType: ResourceTypeMemory, Quantity: 1024, UnitPrice: 0.1, UsageHours: 720, Subtotal: 72},
		},
		0, 0,
	)

	// 获取成本分析
	analysis, err := billingMgr.GetCostAnalysis(project.ID, periodStart, periodEnd)
	if err != nil {
		t.Fatalf("Failed to get cost analysis: %v", err)
	}

	if analysis.TotalCost != 432 {
		t.Errorf("Expected total cost 432, got %f", analysis.TotalCost)
	}
	if analysis.ProjectID != project.ID {
		t.Errorf("Expected project ID %s, got %s", project.ID, analysis.ProjectID)
	}
}

func TestBillingManager_GetResourceUsageReport(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")

	// 设置配额
	billingMgr.SetQuota(project.ID, ResourceTypeCPU, 100, 80, 0.5, "CNY")
	billingMgr.SetQuota(project.ID, ResourceTypeMemory, 1024, 800, 0.1, "CNY")

	// 分配资源
	billingMgr.AllocateResource(project.ID, ResourceTypeCPU, 60, "user1", "Test allocation")
	billingMgr.AllocateResource(project.ID, ResourceTypeMemory, 512, "user1", "Test allocation")

	now := time.Now()
	report, err := billingMgr.GetResourceUsageReport(project.ID, now.AddDate(0, 0, -7), now)
	if err != nil {
		t.Fatalf("Failed to get resource usage report: %v", err)
	}

	if report.ProjectID != project.ID {
		t.Errorf("Expected project ID %s, got %s", project.ID, report.ProjectID)
	}
	if len(report.Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(report.Resources))
	}
	if len(report.QuotaUsage) != 2 {
		t.Errorf("Expected 2 quota usages, got %d", len(report.QuotaUsage))
	}
}

func TestBillingManager_DeleteQuota(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")
	billingMgr.SetQuota(project.ID, ResourceTypeCPU, 100, 80, 0.5, "CNY")

	// 删除配额
	err := billingMgr.DeleteQuota(project.ID, ResourceTypeCPU)
	if err != nil {
		t.Fatalf("Failed to delete quota: %v", err)
	}

	// 确认删除
	_, err = billingMgr.GetQuota(project.ID, ResourceTypeCPU)
	if err != ErrQuotaNotFound {
		t.Errorf("Expected ErrQuotaNotFound, got %v", err)
	}
}

func TestBillingManager_InvalidProject(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	// 使用不存在的项目
	_, err := billingMgr.SetQuota("nonexistent", ResourceTypeCPU, 100, 80, 0.5, "CNY")
	if err != ErrProjectNotFound {
		t.Errorf("Expected ErrProjectNotFound, got %v", err)
	}

	err = billingMgr.AllocateResource("nonexistent", ResourceTypeCPU, 50, "user1", "Test")
	if err != ErrQuotaNotFound {
		t.Errorf("Expected ErrQuotaNotFound, got %v", err)
	}

	_, err = billingMgr.CreateBillingRecord("nonexistent", time.Now(), time.Now(), nil, 0, 0)
	if err != ErrProjectNotFound {
		t.Errorf("Expected ErrProjectNotFound, got %v", err)
	}
}

func TestBillingManager_InvalidQuotaValue(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")

	// 无效的配额值
	_, err := billingMgr.SetQuota(project.ID, ResourceTypeCPU, 0, 0, 0.5, "CNY")
	if err != ErrInvalidQuotaValue {
		t.Errorf("Expected ErrInvalidQuotaValue, got %v", err)
	}

	_, err = billingMgr.SetQuota(project.ID, ResourceTypeCPU, -10, 0, 0.5, "CNY")
	if err != ErrInvalidQuotaValue {
		t.Errorf("Expected ErrInvalidQuotaValue, got %v", err)
	}
}

func TestBillingManager_SoftLimitAdjustment(t *testing.T) {
	mgr := NewManager()
	billingMgr := NewBillingManager(mgr)

	project, _ := mgr.CreateProject("Test Project", "TP", "Test", "user1", "user1")

	// 设置软限制大于硬限制，应该自动调整
	quota, err := billingMgr.SetQuota(project.ID, ResourceTypeCPU, 100, 150, 0.5, "CNY")
	if err != nil {
		t.Fatalf("Failed to set quota: %v", err)
	}

	// 软限制应该被调整为硬限制的值
	if quota.SoftLimit > quota.HardLimit {
		t.Errorf("Soft limit should not exceed hard limit")
	}
}
