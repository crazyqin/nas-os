package smartbilling

import (
	"fmt"
	"math"
	"testing"
	"time"
)

// roundTo2ForTest 测试用的四舍五入辅助函数
func roundTo2ForTest(v float64) float64 {
	return math.Round(v*100) / 100
}

// setupBilling 创建测试用的计费系统实例
func setupBilling() *SmartBilling {
	sb := NewSmartBilling()

	// 设置阶梯定价策略
	sb.SetPricingStrategy(&PricingStrategy{
		ResourceType: ResourceStorage,
		Unit:         "GB",
		Tiers: []TierConfig{
			{Limit: 100, Price: 0.1},  // 0-100GB: 0.1元/GB
			{Limit: 400, Price: 0.08}, // 100-500GB: 0.08元/GB
			{Limit: 0, Price: 0.05},   // 500GB以上: 0.05元/GB
		},
	})

	sb.SetPricingStrategy(&PricingStrategy{
		ResourceType: ResourceBandwidth,
		Unit:         "GB",
		Tiers: []TierConfig{
			{Limit: 1000, Price: 0.05}, // 0-1000GB: 0.05元/GB
			{Limit: 0, Price: 0.03},    // 1000GB以上: 0.03元/GB
		},
	})

	sb.SetPricingStrategy(&PricingStrategy{
		ResourceType: ResourceCPU,
		Unit:         "核·小时",
		Tiers: []TierConfig{
			{Limit: 0, Price: 0.5}, // 0.5元/核·小时
		},
	})

	sb.SetPricingStrategy(&PricingStrategy{
		ResourceType: ResourceMemory,
		Unit:         "GB·小时",
		Tiers: []TierConfig{
			{Limit: 0, Price: 0.2}, // 0.2元/GB·小时
		},
	})

	return sb
}

func TestNewSmartBilling(t *testing.T) {
	sb := NewSmartBilling()
	if sb == nil {
		t.Fatal("NewSmartBilling 返回 nil")
	}
	if sb.accounts == nil {
		t.Error("accounts map 未初始化")
	}
	if sb.pricingStrategy == nil {
		t.Error("pricingStrategy map 未初始化")
	}
}

func TestAddAccount(t *testing.T) {
	sb := setupBilling()

	// 正常添加账户
	account, err := sb.AddAccount("user1", "张三", BudgetConfig{
		Limit:   1000,
		Period:  "monthly",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("添加账户失败: %v", err)
	}
	if account.ID != "user1" {
		t.Errorf("期望ID为user1，实际为%s", account.ID)
	}
	if account.Name != "张三" {
		t.Errorf("期望Name为张三，实际为%s", account.Name)
	}
	if account.TotalCost != 0 {
		t.Errorf("期望TotalCost为0，实际为%f", account.TotalCost)
	}

	// 重复添加账户
	_, err = sb.AddAccount("user1", "张三2", BudgetConfig{})
	if err == nil {
		t.Error("重复添加账户应返回错误")
	}
}

func TestGetAccount(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{Limit: 500, Period: "monthly", Enabled: true})

	// 正常获取
	account, err := sb.GetAccount("user1")
	if err != nil {
		t.Fatalf("获取账户失败: %v", err)
	}
	if account.Name != "张三" {
		t.Errorf("期望Name为张三，实际为%s", account.Name)
	}
	if account.Budget.Limit != 500 {
		t.Errorf("期望预算上限为500，实际为%f", account.Budget.Limit)
	}

	// 获取不存在的账户
	_, err = sb.GetAccount("nonexistent")
	if err == nil {
		t.Error("获取不存在的账户应返回错误")
	}
}

func TestListAccounts(t *testing.T) {
	sb := setupBilling()

	// 空列表
	accounts := sb.ListAccounts()
	if len(accounts) != 0 {
		t.Errorf("期望空列表，实际有 %d 个账户", len(accounts))
	}

	// 添加多个账户
	sb.AddAccount("user2", "李四", BudgetConfig{})
	sb.AddAccount("user1", "张三", BudgetConfig{})
	sb.AddAccount("user3", "王五", BudgetConfig{})

	accounts = sb.ListAccounts()
	if len(accounts) != 3 {
		t.Errorf("期望3个账户，实际有 %d 个", len(accounts))
	}

	// 验证按ID排序
	if accounts[0].ID != "user1" || accounts[1].ID != "user2" || accounts[2].ID != "user3" {
		t.Error("账户未按ID排序")
	}
}

func TestSetPricingStrategy(t *testing.T) {
	sb := NewSmartBilling()

	strategy := &PricingStrategy{
		ResourceType: ResourceStorage,
		Unit:         "GB",
		Tiers: []TierConfig{
			{Limit: 100, Price: 0.1},
		},
	}
	sb.SetPricingStrategy(strategy)

	// 验证策略已设置
	retrieved, err := sb.GetPricingStrategy(ResourceStorage)
	if err != nil {
		t.Fatalf("获取定价策略失败: %v", err)
	}
	if retrieved.Unit != "GB" {
		t.Errorf("期望单位为GB，实际为%s", retrieved.Unit)
	}
	if len(retrieved.Tiers) != 1 {
		t.Errorf("期望1个阶梯，实际有 %d 个", len(retrieved.Tiers))
	}
}

func TestGetPricingStrategy(t *testing.T) {
	sb := NewSmartBilling()

	// 获取未设置的策略
	_, err := sb.GetPricingStrategy(ResourceStorage)
	if err == nil {
		t.Error("获取未设置的策略应返回错误")
	}

	// 设置后获取
	sb.SetPricingStrategy(&PricingStrategy{
		ResourceType: ResourceStorage,
		Unit:         "GB",
		Tiers:        []TierConfig{{Limit: 100, Price: 0.1}},
	})
	strategy, err := sb.GetPricingStrategy(ResourceStorage)
	if err != nil {
		t.Fatalf("获取策略失败: %v", err)
	}
	if strategy.ResourceType != ResourceStorage {
		t.Errorf("期望资源类型为storage，实际为%s", strategy.ResourceType)
	}
}

func TestRecordUsage(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{})

	// 正常记录
	record, err := sb.RecordUsage("user1", ResourceStorage, 50)
	if err != nil {
		t.Fatalf("记录使用失败: %v", err)
	}
	if record.AccountID != "user1" {
		t.Errorf("期望账户ID为user1，实际为%s", record.AccountID)
	}
	if record.ResourceType != ResourceStorage {
		t.Errorf("期望资源类型为storage，实际为%s", record.ResourceType)
	}
	if record.Amount != 50 {
		t.Errorf("期望使用量为50，实际为%f", record.Amount)
	}
	// 50GB * 0.1元/GB = 5元
	if record.Cost != 5.0 {
		t.Errorf("期望费用为5.0，实际为%f", record.Cost)
	}

	// 账户不存在
	_, err = sb.RecordUsage("nonexistent", ResourceStorage, 10)
	if err == nil {
		t.Error("不存在的账户应返回错误")
	}

	// 负数使用量
	_, err = sb.RecordUsage("user1", ResourceStorage, -10)
	if err == nil {
		t.Error("负数使用量应返回错误")
	}

	// 未设置定价策略的资源
	_, err = sb.RecordUsage("user1", "unknown_resource", 10)
	if err == nil {
		t.Error("未设置定价策略应返回错误")
	}
}

func TestRecordUsageWithSuspendedAccount(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{})

	// 暂停账户
	sb.SuspendAccount("user1")

	// 尝试记录使用
	_, err := sb.RecordUsage("user1", ResourceStorage, 10)
	if err == nil {
		t.Error("暂停账户应返回错误")
	}

	// 激活后再记录
	sb.ActivateAccount("user1")
	record, err := sb.RecordUsage("user1", ResourceStorage, 10)
	if err != nil {
		t.Fatalf("激活后记录使用失败: %v", err)
	}
	if record.Cost != 1.0 {
		t.Errorf("期望费用为1.0，实际为%f", record.Cost)
	}
}

func TestCalculateCostWithTiers(t *testing.T) {
	sb := setupBilling()

	// 测试阶梯定价
	strategy := &PricingStrategy{
		ResourceType: ResourceStorage,
		Unit:         "GB",
		Tiers: []TierConfig{
			{Limit: 100, Price: 0.1},  // 0-100GB: 0.1元/GB
			{Limit: 400, Price: 0.08}, // 100-500GB: 0.08元/GB
			{Limit: 0, Price: 0.05},   // 500GB以上: 0.05元/GB
		},
	}

	// 测试1: 50GB (第一阶梯)
	cost := sb.calculateCost(strategy, 50)
	if cost != 5.0 {
		t.Errorf("50GB: 期望费用5.0，实际%f", cost)
	}

	// 测试2: 150GB (跨两个阶梯)
	// 100GB * 0.1 + 50GB * 0.08 = 10 + 4 = 14
	cost = sb.calculateCost(strategy, 150)
	if cost != 14.0 {
		t.Errorf("150GB: 期望费用14.0，实际%f", cost)
	}

	// 测试3: 600GB (跨三个阶梯)
	// 100GB * 0.1 + 400GB * 0.08 + 100GB * 0.05 = 10 + 32 + 5 = 47
	cost = sb.calculateCost(strategy, 600)
	if cost != 47.0 {
		t.Errorf("600GB: 期望费用47.0，实际%f", cost)
	}

	// 测试4: 0GB
	cost = sb.calculateCost(strategy, 0)
	if cost != 0 {
		t.Errorf("0GB: 期望费用0，实际%f", cost)
	}
}

func TestCalculateBill(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{})

	now := time.Now()
	start := now.Add(-24 * time.Hour)
	end := now.Add(24 * time.Hour)

	// 记录使用
	sb.RecordUsage("user1", ResourceStorage, 150)
	sb.RecordUsage("user1", ResourceBandwidth, 500)
	sb.RecordUsage("user1", ResourceCPU, 10)

	// 计算账单
	items, total, err := sb.CalculateBill("user1", start, end)
	if err != nil {
		t.Fatalf("计算账单失败: %v", err)
	}

	// 验证存储费用: 100*0.1 + 50*0.08 = 14.0
	storageItem, exists := items[ResourceStorage]
	if !exists {
		t.Error("缺少存储费用明细")
	} else {
		if storageItem.Cost != 14.0 {
			t.Errorf("期望存储费用14.0，实际%f", storageItem.Cost)
		}
		if storageItem.Usage != 150 {
			t.Errorf("期望存储使用量150，实际%f", storageItem.Usage)
		}
	}

	// 验证带宽费用: 500 * 0.05 = 25.0
	bandwidthItem, exists := items[ResourceBandwidth]
	if !exists {
		t.Error("缺少带宽费用明细")
	} else {
		if bandwidthItem.Cost != 25.0 {
			t.Errorf("期望带宽费用25.0，实际%f", bandwidthItem.Cost)
		}
	}

	// 验证CPU费用: 10 * 0.5 = 5.0
	cpuItem, exists := items[ResourceCPU]
	if !exists {
		t.Error("缺少CPU费用明细")
	} else {
		if cpuItem.Cost != 5.0 {
			t.Errorf("期望CPU费用5.0，实际%f", cpuItem.Cost)
		}
	}

	// 验证总费用: 14.0 + 25.0 + 5.0 = 44.0
	if total != 44.0 {
		t.Errorf("期望总费用44.0，实际%f", total)
	}

	// 不存在的账户
	_, _, err = sb.CalculateBill("nonexistent", start, end)
	if err == nil {
		t.Error("不存在的账户应返回错误")
	}
}

func TestSetBudget(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{})

	// 设置预算
	err := sb.SetBudget("user1", BudgetConfig{
		Limit:   500,
		Period:  "monthly",
		Enabled: true,
	})
	if err != nil {
		t.Fatalf("设置预算失败: %v", err)
	}

	// 验证预算已设置
	account, _ := sb.GetAccount("user1")
	if account.Budget.Limit != 500 {
		t.Errorf("期望预算上限500，实际%f", account.Budget.Limit)
	}
	if account.Budget.Period != "monthly" {
		t.Errorf("期望周期为monthly，实际%s", account.Budget.Period)
	}

	// 不存在的账户
	err = sb.SetBudget("nonexistent", BudgetConfig{})
	if err == nil {
		t.Error("不存在的账户应返回错误")
	}
}

func TestCheckBudget(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{
		Limit:   100,
		Period:  "monthly",
		Enabled: true,
	})

	// 初始状态 - 在预算内
	within, used, limit, exceeded, err := sb.CheckBudget("user1")
	if err != nil {
		t.Fatalf("检查预算失败: %v", err)
	}
	if !within {
		t.Error("初始状态应在预算内")
	}
	if used != 0 {
		t.Errorf("期望已用0，实际%f", used)
	}
	if limit != 100 {
		t.Errorf("期望上限100，实际%f", limit)
	}
	if exceeded {
		t.Error("初始状态不应超额")
	}

	// 记录使用后检查
	sb.RecordUsage("user1", ResourceStorage, 50) // 50*0.1 = 5元
	sb.RecordUsage("user1", ResourceStorage, 50) // 50*0.1 = 5元，累计10元

	within, used, limit, exceeded, err = sb.CheckBudget("user1")
	if err != nil {
		t.Fatalf("检查预算失败: %v", err)
	}
	if !within {
		t.Error("10元应在100元预算内")
	}
	if used != 10.0 {
		t.Errorf("期望已用10.0，实际%f", used)
	}

	// 超出预算
	sb.RecordUsage("user1", ResourceStorage, 1000) // 100*0.1 + 400*0.08 + 500*0.05 = 10+32+25 = 67元，累计77元
	sb.RecordUsage("user1", ResourceStorage, 1000) // 再67元，累计144元

	within, used, _, exceeded, err = sb.CheckBudget("user1")
	if err != nil {
		t.Fatalf("检查预算失败: %v", err)
	}
	if within {
		t.Error("144元应超出100元预算")
	}
	if !exceeded {
		t.Error("应该超额")
	}
	if used != 144.0 {
		t.Errorf("期望已用144.0，实际%f", used)
	}

	// 禁用预算检查
	sb.SetBudget("user1", BudgetConfig{Enabled: false})
	within, _, _, exceeded, err = sb.CheckBudget("user1")
	if err != nil {
		t.Fatalf("检查预算失败: %v", err)
	}
	if !within {
		t.Error("禁用预算时应在预算内")
	}
	if exceeded {
		t.Error("禁用预算时不应超额")
	}

	// 不存在的账户
	_, _, _, _, err = sb.CheckBudget("nonexistent")
	if err == nil {
		t.Error("不存在的账户应返回错误")
	}
}

func TestGenerateInvoice(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{})

	now := time.Now()
	start := now.Add(-24 * time.Hour)
	end := now.Add(24 * time.Hour)

	// 记录使用
	sb.RecordUsage("user1", ResourceStorage, 200)
	sb.RecordUsage("user1", ResourceBandwidth, 800)
	sb.RecordUsage("user1", ResourceMemory, 50)

	// 生成账单
	invoice, err := sb.GenerateInvoice("user1", "2024-01", start, end)
	if err != nil {
		t.Fatalf("生成账单失败: %v", err)
	}

	if invoice.AccountID != "user1" {
		t.Errorf("期望账户ID为user1，实际为%s", invoice.AccountID)
	}
	if invoice.Period != "2024-01" {
		t.Errorf("期望周期为2024-01，实际为%s", invoice.Period)
	}
	if len(invoice.Items) != 3 {
		t.Errorf("期望3个明细项，实际有 %d 个", len(invoice.Items))
	}

	// 验证各项费用
	// 存储: 100*0.1 + 100*0.08 = 18.0
	// 带宽: 800*0.05 = 40.0
	// 内存: 50*0.2 = 10.0
	expectedTotal := 18.0 + 40.0 + 10.0
	if invoice.Total != roundTo2ForTest(expectedTotal) {
		t.Errorf("期望总费用%f，实际%f", expectedTotal, invoice.Total)
	}

	// 不存在的账户
	_, err = sb.GenerateInvoice("nonexistent", "2024-01", start, end)
	if err == nil {
		t.Error("不存在的账户应返回错误")
	}
}

func TestGetStats(t *testing.T) {
	sb := setupBilling()

	// 空系统统计
	stats := sb.GetStats()
	if stats.AccountCount != 0 {
		t.Errorf("期望账户数0，实际%d", stats.AccountCount)
	}
	if stats.RecordCount != 0 {
		t.Errorf("期望记录数0，实际%d", stats.RecordCount)
	}
	if stats.TotalRevenue != 0 {
		t.Errorf("期望总收入0，实际%f", stats.TotalRevenue)
	}

	// 添加账户和记录
	sb.AddAccount("user1", "张三", BudgetConfig{})
	sb.AddAccount("user2", "李四", BudgetConfig{})

	sb.RecordUsage("user1", ResourceStorage, 100)   // 10元
	sb.RecordUsage("user1", ResourceBandwidth, 200) // 10元
	sb.RecordUsage("user2", ResourceCPU, 10)        // 5元

	stats = sb.GetStats()

	if stats.AccountCount != 2 {
		t.Errorf("期望账户数2，实际%d", stats.AccountCount)
	}
	if stats.RecordCount != 3 {
		t.Errorf("期望记录数3，实际%d", stats.RecordCount)
	}
	if stats.TotalRevenue != 25.0 {
		t.Errorf("期望总收入25.0，实际%f", stats.TotalRevenue)
	}
	if stats.ByResource[ResourceStorage] != 10.0 {
		t.Errorf("期望存储费用10.0，实际%f", stats.ByResource[ResourceStorage])
	}
	if stats.ByResource[ResourceBandwidth] != 10.0 {
		t.Errorf("期望带宽费用10.0，实际%f", stats.ByResource[ResourceBandwidth])
	}
	if stats.ByResource[ResourceCPU] != 5.0 {
		t.Errorf("期望CPU费用5.0，实际%f", stats.ByResource[ResourceCPU])
	}
	if stats.ByAccount["user1"] != 20.0 {
		t.Errorf("期望user1费用20.0，实际%f", stats.ByAccount["user1"])
	}
	if stats.ByAccount["user2"] != 5.0 {
		t.Errorf("期望user2费用5.0，实际%f", stats.ByAccount["user2"])
	}
	if stats.AvgCostPerUser != 12.5 {
		t.Errorf("期望平均费用12.5，实际%f", stats.AvgCostPerUser)
	}
}

func TestSuspendAndActivateAccount(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{})

	// 暂停
	err := sb.SuspendAccount("user1")
	if err != nil {
		t.Fatalf("暂停账户失败: %v", err)
	}

	account, _ := sb.GetAccount("user1")
	if !account.IsSuspended {
		t.Error("账户应已暂停")
	}

	// 激活
	err = sb.ActivateAccount("user1")
	if err != nil {
		t.Fatalf("激活账户失败: %v", err)
	}

	account, _ = sb.GetAccount("user1")
	if account.IsSuspended {
		t.Error("账户不应暂停")
	}

	// 不存在的账户
	err = sb.SuspendAccount("nonexistent")
	if err == nil {
		t.Error("不存在的账户应返回错误")
	}

	err = sb.ActivateAccount("nonexistent")
	if err == nil {
		t.Error("不存在的账户应返回错误")
	}
}

func TestGetUsageRecords(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{})

	now := time.Now()
	start := now.Add(-48 * time.Hour)
	end := now.Add(24 * time.Hour)

	// 记录使用
	sb.RecordUsage("user1", ResourceStorage, 100)
	sb.RecordUsage("user1", ResourceBandwidth, 200)
	sb.RecordUsage("user1", ResourceCPU, 10)

	// 获取记录
	records := sb.GetUsageRecords("user1", start, end)
	if len(records) != 3 {
		t.Errorf("期望3条记录，实际%d条", len(records))
	}

	// 验证记录按时间排序
	for i := 1; i < len(records); i++ {
		if records[i].Timestamp.Before(records[i-1].Timestamp) {
			t.Error("记录应按时间升序排序")
			break
		}
	}

	// 空时间段
	emptyStart := now.Add(-100 * time.Hour)
	emptyEnd := now.Add(-50 * time.Hour)
	records = sb.GetUsageRecords("user1", emptyStart, emptyEnd)
	if len(records) != 0 {
		t.Errorf("期望空记录，实际%d条", len(records))
	}

	// 不存在的账户
	records = sb.GetUsageRecords("nonexistent", start, end)
	if len(records) != 0 {
		t.Errorf("不存在的账户应返回空记录，实际%d条", len(records))
	}
}

func TestRecordUsageUpdatesTotalCost(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{})

	// 记录多次使用
	sb.RecordUsage("user1", ResourceStorage, 100)   // 10元
	sb.RecordUsage("user1", ResourceBandwidth, 200) // 10元
	sb.RecordUsage("user1", ResourceCPU, 10)        // 5元

	account, _ := sb.GetAccount("user1")
	expectedTotal := 25.0
	if account.TotalCost != expectedTotal {
		t.Errorf("期望累计费用%f，实际%f", expectedTotal, account.TotalCost)
	}
}

func TestGenerateInvoiceSortedByResourceType(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{})

	now := time.Now()
	start := now.Add(-24 * time.Hour)
	end := now.Add(24 * time.Hour)

	// 添加各种资源类型
	sb.RecordUsage("user1", ResourceCPU, 10)
	sb.RecordUsage("user1", ResourceStorage, 100)
	sb.RecordUsage("user1", ResourceMemory, 50)
	sb.RecordUsage("user1", ResourceBandwidth, 200)

	invoice, err := sb.GenerateInvoice("user1", "2024-01", start, end)
	if err != nil {
		t.Fatalf("生成账单失败: %v", err)
	}

	// 验证明细项按资源类型排序
	if len(invoice.Items) != 4 {
		t.Errorf("期望4个明细项，实际%d个", len(invoice.Items))
	}

	// 验证排序顺序: bandwidth < cpu < memory < storage
	expectedOrder := []ResourceType{ResourceBandwidth, ResourceCPU, ResourceMemory, ResourceStorage}
	for i, item := range invoice.Items {
		if i < len(expectedOrder) && item.ResourceType != expectedOrder[i] {
			t.Errorf("第%d项期望资源类型%s，实际%s", i, expectedOrder[i], item.ResourceType)
		}
	}
}

func TestRecordIDIncrement(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{})

	record1, _ := sb.RecordUsage("user1", ResourceStorage, 100)
	record2, _ := sb.RecordUsage("user1", ResourceStorage, 100)
	record3, _ := sb.RecordUsage("user1", ResourceStorage, 100)

	if record1.ID != "UR-1" {
		t.Errorf("期望记录ID为UR-1，实际为%s", record1.ID)
	}
	if record2.ID != "UR-2" {
		t.Errorf("期望记录ID为UR-2，实际为%s", record2.ID)
	}
	if record3.ID != "UR-3" {
		t.Errorf("期望记录ID为UR-3，实际为%s", record3.ID)
	}
}

func TestInvoiceIDIncrement(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{})

	now := time.Now()
	start := now.Add(-24 * time.Hour)
	end := now.Add(24 * time.Hour)

	sb.RecordUsage("user1", ResourceStorage, 100)

	invoice1, _ := sb.GenerateInvoice("user1", "2024-01", start, end)
	invoice2, _ := sb.GenerateInvoice("user1", "2024-02", start, end)

	if invoice1.ID != "INV-1" {
		t.Errorf("期望账单ID为INV-1，实际为%s", invoice1.ID)
	}
	if invoice2.ID != "INV-2" {
		t.Errorf("期望账单ID为INV-2，实际为%s", invoice2.ID)
	}
}

func TestBudgetWithDifferentPeriods(t *testing.T) {
	sb := setupBilling()

	// 测试不同预算周期
	periods := []string{"daily", "weekly", "monthly"}
	for i, period := range periods {
		id := fmt.Sprintf("user%d", i+1)
		sb.AddAccount(id, fmt.Sprintf("用户%d", i+1), BudgetConfig{
			Limit:   100,
			Period:  period,
			Enabled: true,
		})

		account, _ := sb.GetAccount(id)
		if account.Budget.Period != period {
			t.Errorf("用户%s期望周期%s，实际%s", id, period, account.Budget.Period)
		}
	}
}

func TestZeroAmountUsage(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{})

	// 零使用量
	record, err := sb.RecordUsage("user1", ResourceStorage, 0)
	if err != nil {
		t.Fatalf("零使用量不应报错: %v", err)
	}
	if record.Cost != 0 {
		t.Errorf("零使用量费用应为0，实际%f", record.Cost)
	}
}

func TestLargeAmountUsage(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{})

	// 大量使用
	record, err := sb.RecordUsage("user1", ResourceStorage, 10000)
	if err != nil {
		t.Fatalf("大量使用不应报错: %v", err)
	}

	// 100*0.1 + 400*0.08 + 9500*0.05 = 10 + 32 + 475 = 517
	expectedCost := 517.0
	if record.Cost != expectedCost {
		t.Errorf("期望费用%f，实际%f", expectedCost, record.Cost)
	}
}

func TestMultipleAccountsIsolation(t *testing.T) {
	sb := setupBilling()
	sb.AddAccount("user1", "张三", BudgetConfig{})
	sb.AddAccount("user2", "李四", BudgetConfig{})

	sb.RecordUsage("user1", ResourceStorage, 100) // 10元
	sb.RecordUsage("user2", ResourceStorage, 200) // 10+8=18元

	account1, _ := sb.GetAccount("user1")
	account2, _ := sb.GetAccount("user2")

	if account1.TotalCost != 10.0 {
		t.Errorf("用户1期望累计费用10.0，实际%f", account1.TotalCost)
	}
	if account2.TotalCost != 18.0 {
		t.Errorf("用户2期望累计费用18.0，实际%f", account2.TotalCost)
	}

	// 验证记录隔离
	now := time.Now()
	start := now.Add(-24 * time.Hour)
	end := now.Add(24 * time.Hour)

	records1 := sb.GetUsageRecords("user1", start, end)
	records2 := sb.GetUsageRecords("user2", start, end)

	if len(records1) != 1 {
		t.Errorf("用户1期望1条记录，实际%d条", len(records1))
	}
	if len(records2) != 1 {
		t.Errorf("用户2期望1条记录，实际%d条", len(records2))
	}
}
