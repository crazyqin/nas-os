// Package billing 提供计费管理功能测试
package billing

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewBillingManager(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "billing-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	config := DefaultBillingConfig()
	bm, err := NewBillingManager(config, tmpDir)
	require.NoError(t, err)
	require.NotNil(t, bm)
	assert.Equal(t, tmpDir, bm.dataDir)
	assert.True(t, bm.config.Enabled)
}

func TestNewBillingManagerNilConfig(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "billing-test")
	require.NoError(t, err)
	defer func() { _ = os.RemoveAll(tmpDir) }()

	bm, err := NewBillingManager(nil, tmpDir)
	require.NoError(t, err)
	require.NotNil(t, bm)
	assert.NotNil(t, bm.config)
}

func TestDefaultBillingConfig(t *testing.T) {
	config := DefaultBillingConfig()
	assert.True(t, config.Enabled)
	assert.Equal(t, "CNY", config.DefaultCurrency)
	assert.Equal(t, BillingCycleMonthly, config.BillingCycle)
	assert.Equal(t, 0.1, config.StoragePricing.BasePricePerGB)
	assert.Equal(t, 0.5, config.BandwidthPricing.TrafficPricePerGB)
}

func TestRecordUsage(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	ctx := context.Background()
	input := &UsageRecordInput{
		UserID:            "user1",
		UserName:          "测试用户",
		PoolID:            "pool1",
		PoolName:          "测试存储池",
		PeriodStart:       time.Now().AddDate(0, 0, -30),
		PeriodEnd:         time.Now(),
		StorageUsedBytes:  100 * 1024 * 1024 * 1024, // 100 GB
		StoragePeakBytes:  120 * 1024 * 1024 * 1024, // 120 GB
		BandwidthInBytes:  50 * 1024 * 1024 * 1024,  // 50 GB
		BandwidthOutBytes: 80 * 1024 * 1024 * 1024,  // 80 GB
		PeakBandwidthMbps: 100,
		APIRequests:       10000,
		FileOperations:    5000,
	}

	record, err := bm.RecordUsage(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, record)
	assert.NotEmpty(t, record.ID)
	assert.Equal(t, "user1", record.UserID)
	assert.Equal(t, 100.0, record.StorageUsedGB)
	assert.Equal(t, 130.0, record.BandwidthTotalGB)
}

func TestGetUsageRecord(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	ctx := context.Background()
	created, err := bm.RecordUsage(ctx, &UsageRecordInput{
		UserID:           "user1",
		PeriodStart:      time.Now().AddDate(0, 0, -30),
		PeriodEnd:        time.Now(),
		StorageUsedBytes: 100 * 1024 * 1024 * 1024,
	})
	require.NoError(t, err)

	retrieved, err := bm.GetUsageRecord(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)

	_, err = bm.GetUsageRecord("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrUsageRecordNotFound, err)
}

func TestListUsageRecords(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	ctx := context.Background()

	// 创建多个用量记录
	for i := 0; i < 5; i++ {
		_, err := bm.RecordUsage(ctx, &UsageRecordInput{
			UserID:           "user1",
			PoolID:           "pool1",
			PeriodStart:      time.Now().AddDate(0, 0, -30+i),
			PeriodEnd:        time.Now().AddDate(0, 0, -25+i),
			StorageUsedBytes: uint64((i + 1) * 10 * 1024 * 1024 * 1024),
		})
		require.NoError(t, err)
	}

	// 测试列出所有记录
	records, err := bm.ListUsageRecords("", "", time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Len(t, records, 5)

	// 测试按用户过滤
	records, err = bm.ListUsageRecords("user1", "", time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Len(t, records, 5)

	// 测试按存储池过滤
	records, err = bm.ListUsageRecords("", "pool1", time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Len(t, records, 5)

	// 测试按时间过滤
	start := time.Now().AddDate(0, 0, -28)
	end := time.Now().AddDate(0, 0, -26)
	records, err = bm.ListUsageRecords("", "", start, end)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(records), 1)
}

func TestGetUserUsageSummary(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	ctx := context.Background()

	// 创建多个用量记录
	for i := 0; i < 3; i++ {
		_, err := bm.RecordUsage(ctx, &UsageRecordInput{
			UserID:            "user1",
			UserName:          "测试用户",
			PoolID:            "pool1",
			PoolName:          "存储池1",
			PeriodStart:       time.Now().AddDate(0, 0, -30),
			PeriodEnd:         time.Now(),
			StorageUsedBytes:  uint64((i + 1) * 100 * 1024 * 1024 * 1024),
			BandwidthInBytes:  uint64((i + 1) * 50 * 1024 * 1024 * 1024),
			BandwidthOutBytes: uint64((i + 1) * 30 * 1024 * 1024 * 1024),
			APIRequests:       int64((i + 1) * 1000),
		})
		require.NoError(t, err)
	}

	summary, err := bm.GetUserUsageSummary("user1", time.Now().AddDate(0, 0, -60), time.Now())
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, "user1", summary.UserID)
	assert.Equal(t, "测试用户", summary.UserName)
	assert.Equal(t, 600.0, summary.TotalStorageUsedGB) // 100 + 200 + 300
	assert.Equal(t, 480.0, summary.TotalBandwidthGB)   // 80 + 160 + 240
	assert.Equal(t, int64(6000), summary.TotalAPIRequests)
	assert.NotNil(t, summary.PoolSummaries["pool1"])
}

func TestCreateInvoice(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	ctx := context.Background()
	now := time.Now()
	start := now.AddDate(0, -1, 0)

	input := &InvoiceInput{
		UserID:      "user1",
		UserName:    "测试用户",
		PeriodStart: start,
		PeriodEnd:   now,
		LineItems: []InvoiceLineItemInput{
			{
				Description: "存储费用",
				Quantity:    100,
				Unit:        "GB",
				UnitPrice:   0.1,
				PoolID:      "pool1",
				PoolName:    "存储池1",
			},
			{
				Description: "带宽费用",
				Quantity:    50,
				Unit:        "GB",
				UnitPrice:   0.5,
			},
		},
		Notes: "月度账单",
	}

	invoice, err := bm.CreateInvoice(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, invoice)
	assert.NotEmpty(t, invoice.ID)
	assert.Contains(t, invoice.InvoiceNumber, "INV")
	assert.Equal(t, InvoiceStatusDraft, invoice.Status)
	assert.Equal(t, 35.0, invoice.Subtotal) // 100*0.1 + 50*0.5 = 10 + 25 = 35
	assert.Equal(t, 10.0, invoice.StorageAmount)
	assert.Equal(t, 25.0, invoice.BandwidthAmount)
}

func TestCreateInvoiceWithDiscount(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	ctx := context.Background()

	input := &InvoiceInput{
		UserID:      "user1",
		UserName:    "测试用户",
		PeriodStart: time.Now().AddDate(0, -1, 0),
		PeriodEnd:   time.Now(),
		LineItems: []InvoiceLineItemInput{
			{
				Description:     "存储费用",
				Quantity:        100,
				UnitPrice:       0.1,
				DiscountPercent: 10, // 10% 折扣
			},
		},
		DiscountAmount: 5,
		DiscountReason: "老客户优惠",
	}

	invoice, err := bm.CreateInvoice(ctx, input)
	require.NoError(t, err)
	// 100 * 0.1 = 10, 10%折扣 = 9, 减5 = 4
	assert.Equal(t, 9.0, invoice.Subtotal)
	assert.Equal(t, 4.0, invoice.TotalAmount)
	assert.Equal(t, "老客户优惠", invoice.DiscountReason)
}

func TestCreateInvoiceWithTax(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "billing-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := DefaultBillingConfig()
	config.TaxRate = 0.13 // 13% 税率
	bm, err := NewBillingManager(config, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()

	input := &InvoiceInput{
		UserID:      "user1",
		PeriodStart: time.Now().AddDate(0, -1, 0),
		PeriodEnd:   time.Now(),
		LineItems: []InvoiceLineItemInput{
			{
				Description: "服务费",
				Quantity:    100,
				UnitPrice:   1.0,
			},
		},
	}

	invoice, err := bm.CreateInvoice(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, 100.0, invoice.Subtotal)
	assert.Equal(t, 13.0, invoice.TaxAmount) // 100 * 0.13
	assert.Equal(t, 113.0, invoice.TotalAmount)
}

func TestIssueInvoice(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	invoice := createTestInvoice(t, bm)

	issued, err := bm.IssueInvoice(invoice.ID)
	require.NoError(t, err)
	assert.Equal(t, InvoiceStatusIssued, issued.Status)

	// 不能重复开具
	_, err = bm.IssueInvoice(invoice.ID)
	assert.Error(t, err)
}

func TestMarkInvoicePaid(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	invoice := createTestInvoice(t, bm)

	// 先开具
	_, err := bm.IssueInvoice(invoice.ID)
	require.NoError(t, err)

	// 标记已支付
	paid, err := bm.MarkInvoicePaid(invoice.ID, "alipay", "PAY123")
	require.NoError(t, err)
	assert.Equal(t, InvoiceStatusPaid, paid.Status)
	assert.NotNil(t, paid.PaidAt)
	assert.Equal(t, "alipay", paid.PaymentMethod)

	// 不能重复标记
	_, err = bm.MarkInvoicePaid(invoice.ID, "wechat", "PAY456")
	assert.Error(t, err)
	assert.Equal(t, ErrInvoiceAlreadyPaid, err)
}

func TestVoidInvoice(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	invoice := createTestInvoice(t, bm)

	voided, err := bm.VoidInvoice(invoice.ID)
	require.NoError(t, err)
	assert.Equal(t, InvoiceStatusVoid, voided.Status)

	// 不能重复作废
	_, err = bm.VoidInvoice(invoice.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrInvoiceAlreadyVoid, err)
}

func TestGetInvoice(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	created := createTestInvoice(t, bm)

	retrieved, err := bm.GetInvoice(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)

	byNumber, err := bm.GetInvoiceByNumber(created.InvoiceNumber)
	require.NoError(t, err)
	assert.Equal(t, created.ID, byNumber.ID)

	_, err = bm.GetInvoice("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrInvoiceNotFound, err)
}

func TestListInvoices(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	ctx := context.Background()

	// 创建多个发票
	for i := 0; i < 3; i++ {
		_, err := bm.CreateInvoice(ctx, &InvoiceInput{
			UserID:      "user1",
			PeriodStart: time.Now().AddDate(0, -1, 0),
			PeriodEnd:   time.Now(),
			LineItems: []InvoiceLineItemInput{
				{Description: "费用", Quantity: float64(i+1) * 100, UnitPrice: 0.1},
			},
		})
		require.NoError(t, err)
	}

	// 列出所有发票
	invoices, err := bm.ListInvoices("", "", time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Len(t, invoices, 3)

	// 按用户过滤
	invoices, err = bm.ListInvoices("user1", "", time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Len(t, invoices, 3)
}

func TestGenerateInvoiceFromUsage(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	ctx := context.Background()

	// 先记录用量
	_, err := bm.RecordUsage(ctx, &UsageRecordInput{
		UserID:            "user1",
		UserName:          "测试用户",
		PoolID:            "pool1",
		PoolName:          "存储池1",
		PeriodStart:       time.Now().AddDate(0, 0, -30),
		PeriodEnd:         time.Now(),
		StorageUsedBytes:  100 * 1024 * 1024 * 1024, // 100 GB
		BandwidthInBytes:  50 * 1024 * 1024 * 1024,
		BandwidthOutBytes: 30 * 1024 * 1024 * 1024,
	})
	require.NoError(t, err)

	// 根据用量生成发票
	invoice, err := bm.GenerateInvoiceFromUsage(ctx, "user1",
		time.Now().AddDate(0, 0, -60), time.Now())
	require.NoError(t, err)
	require.NotNil(t, invoice)
	assert.Equal(t, "user1", invoice.UserID)
	assert.Greater(t, invoice.TotalAmount, 0.0)
}

func TestGetBillingStats(t *testing.T) {
	bm := createTestBillingManager(t)
	defer func() { _ = os.RemoveAll(bm.dataDir) }()

	ctx := context.Background()

	// 创建用量记录和发票
	_, err := bm.RecordUsage(ctx, &UsageRecordInput{
		UserID:           "user1",
		PoolID:           "pool1",
		PeriodStart:      time.Now().AddDate(0, 0, -30),
		PeriodEnd:        time.Now(),
		StorageUsedBytes: 100 * 1024 * 1024 * 1024,
	})
	require.NoError(t, err)

	invoice := createTestInvoice(t, bm)
	_, err = bm.IssueInvoice(invoice.ID)
	require.NoError(t, err)
	_, err = bm.MarkInvoicePaid(invoice.ID, "alipay", "PAY123")
	require.NoError(t, err)

	stats, err := bm.GetBillingStats(time.Now().AddDate(0, -1, 0), time.Now())
	require.NoError(t, err)
	require.NotNil(t, stats)
	assert.GreaterOrEqual(t, stats.TotalInvoices, 1)
	assert.GreaterOrEqual(t, stats.PaidInvoices, 1)
}

func TestGetUserBillingStats(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	ctx := context.Background()

	_, err := bm.RecordUsage(ctx, &UsageRecordInput{
		UserID:           "user1",
		PeriodStart:      time.Now().AddDate(0, 0, -30),
		PeriodEnd:        time.Now(),
		StorageUsedBytes: 100 * 1024 * 1024 * 1024,
	})
	require.NoError(t, err)

	stats, err := bm.GetUserBillingStats("user1", time.Now().AddDate(0, -1, 0), time.Now())
	require.NoError(t, err)
	assert.Equal(t, "user1", stats.UserID)
	assert.Equal(t, 100.0, stats.StorageUsedGB)
}

func TestGetInvoiceSummary(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	ctx := context.Background()

	// 创建多个发票
	for i := 0; i < 3; i++ {
		inv, err := bm.CreateInvoice(ctx, &InvoiceInput{
			UserID:      "user1",
			UserName:    "测试用户",
			PeriodStart: time.Now().AddDate(0, -1, 0),
			PeriodEnd:   time.Now(),
			LineItems: []InvoiceLineItemInput{
				{Description: "费用", Quantity: 100, UnitPrice: 0.1},
			},
		})
		require.NoError(t, err)
		if i == 0 {
			_, err = bm.IssueInvoice(inv.ID)
			require.NoError(t, err)
		}
	}

	summary, err := bm.GetInvoiceSummary("user1", time.Now().AddDate(0, -2, 0), time.Now())
	require.NoError(t, err)
	assert.Equal(t, "user1", summary.UserID)
	assert.Equal(t, 3, summary.TotalInvoices)
	assert.Equal(t, 2, summary.DraftInvoices)
	assert.Equal(t, 1, summary.IssuedInvoices)
}

func TestCalculateStorageCost(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	// 测试免费额度内的计算
	cost := bm.calculateStorageCost(5) // 5GB，免费额度 10GB
	assert.Equal(t, 0.0, cost)

	// 测试超出免费额度的计算
	cost = bm.calculateStorageCost(50) // 50GB
	// (50 - 10) * 0.1 = 4
	assert.Equal(t, 4.0, cost)

	// 测试阶梯定价
	cost = bm.calculateStorageCost(500) // 500GB
	// 第一阶梯 0-100: 100 * 0.1 = 10
	// 第二阶梯 100-500: 400 * 0.08 = 32
	// 总计 42（减去免费额度 10GB）
	// 实际计算：(500 - 10) 按阶梯计费
	assert.Greater(t, cost, 0.0)
}

func TestCalculateBandwidthCost(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	// 测试免费额度内的计算
	cost := bm.calculateBandwidthCost(50) // 50GB，免费额度 100GB
	assert.Equal(t, 0.0, cost)

	// 测试超出免费额度的计算
	cost = bm.calculateBandwidthCost(200) // 200GB
	// (200 - 100) * 0.5 = 50
	assert.Equal(t, 50.0, cost)
}

func TestCleanupOldData(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	ctx := context.Background()

	// 创建一些数据
	_, err := bm.RecordUsage(ctx, &UsageRecordInput{
		UserID:           "user1",
		PeriodStart:      time.Now().AddDate(0, 0, -400), // 很旧的记录
		PeriodEnd:        time.Now().AddDate(0, 0, -395),
		StorageUsedBytes: 100 * 1024 * 1024 * 1024,
	})
	require.NoError(t, err)

	// 清理过期数据
	err = bm.CleanupOldData()
	require.NoError(t, err)
}

func TestUpdateConfig(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	newConfig := DefaultBillingConfig()
	newConfig.DefaultCurrency = "USD"
	newConfig.StoragePricing.BasePricePerGB = 0.2

	err := bm.UpdateConfig(newConfig)
	require.NoError(t, err)

	retrieved := bm.GetConfig()
	assert.Equal(t, "USD", retrieved.DefaultCurrency)
	assert.Equal(t, 0.2, retrieved.StoragePricing.BasePricePerGB)
}

func TestBytesToGB(t *testing.T) {
	tests := []struct {
		bytes    uint64
		expected float64
	}{
		{0, 0},
		{1024 * 1024 * 1024, 1.0},         // 1 GB
		{100 * 1024 * 1024 * 1024, 100.0}, // 100 GB
		{1024 * 1024, 0.0009765625},       // 1 MB
	}

	for _, tt := range tests {
		result := bytesToGB(tt.bytes)
		assert.InDelta(t, tt.expected, result, 0.0001)
	}
}

func TestTieredPricing(t *testing.T) {
	tiers := []StorageTier{
		{MinGB: 0, MaxGB: 100, PricePerGB: 0.1},
		{MinGB: 100, MaxGB: 1000, PricePerGB: 0.08},
		{MinGB: 1000, MaxGB: -1, PricePerGB: 0.05},
	}

	tests := []struct {
		amount   float64
		expected float64
	}{
		{50, 5.0},     // 50 * 0.1 = 5
		{150, 14.0},   // 100 * 0.1 + 50 * 0.08 = 10 + 4 = 14
		{2000, 132.0}, // 100 * 0.1 + 900 * 0.08 + 1000 * 0.05 = 10 + 72 + 50 = 132
	}

	for _, tt := range tests {
		result := calculateTieredCost(tt.amount, tiers)
		assert.InDelta(t, tt.expected, result, 0.01)
	}
}

// ========== 辅助函数 ==========

func createTestBillingManager(t *testing.T) *BillingManager {
	tmpDir, err := os.MkdirTemp("", "billing-test")
	require.NoError(t, err)
	config := DefaultBillingConfig()
	bm, err := NewBillingManager(config, tmpDir)
	require.NoError(t, err)
	return bm
}

func createTestInvoice(t *testing.T, bm *BillingManager) *Invoice {
	ctx := context.Background()
	invoice, err := bm.CreateInvoice(ctx, &InvoiceInput{
		UserID:      "user1",
		UserName:    "测试用户",
		PeriodStart: time.Now().AddDate(0, -1, 0),
		PeriodEnd:   time.Now(),
		LineItems: []InvoiceLineItemInput{
			{
				Description: "存储费用",
				Quantity:    100,
				Unit:        "GB",
				UnitPrice:   0.1,
				PoolID:      "pool1",
				PoolName:    "存储池1",
			},
		},
	})
	require.NoError(t, err)
	return invoice
}

// ========== 新增测试：提升覆盖率 ==========

func TestGetPoolUsageSummary(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	ctx := context.Background()

	// 创建存储池用量记录
	for i := 0; i < 3; i++ {
		_, err := bm.RecordUsage(ctx, &UsageRecordInput{
			UserID:           "user1",
			PoolID:           "pool1",
			PoolName:         "存储池1",
			PeriodStart:      time.Now().AddDate(0, 0, -30+i),
			PeriodEnd:        time.Now().AddDate(0, 0, -25+i),
			StorageUsedBytes: uint64((i + 1) * 50 * 1024 * 1024 * 1024),
		})
		require.NoError(t, err)
	}

	summary, err := bm.GetPoolUsageSummary("pool1", time.Now().AddDate(0, 0, -60), time.Now())
	require.NoError(t, err)
	require.NotNil(t, summary)
	assert.Equal(t, "pool1", summary.PoolID)
	assert.Equal(t, "存储池1", summary.PoolName)
	assert.Equal(t, 300.0, summary.TotalStorageUsedGB) // 50 + 100 + 150
	assert.Equal(t, 3, summary.Records)
}

func TestGetPoolBillingStats(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	ctx := context.Background()

	// 创建存储池用量记录
	_, err := bm.RecordUsage(ctx, &UsageRecordInput{
		UserID:           "user1",
		PoolID:           "pool1",
		PoolName:         "存储池1",
		PeriodStart:      time.Now().AddDate(0, 0, -30),
		PeriodEnd:        time.Now(),
		StorageUsedBytes: 100 * 1024 * 1024 * 1024,
	})
	require.NoError(t, err)

	stats, err := bm.GetPoolBillingStats("pool1", time.Now().AddDate(0, -1, 0), time.Now())
	require.NoError(t, err)
	assert.Equal(t, "pool1", stats.PoolID)
	assert.Equal(t, "存储池1", stats.PoolName)
	assert.Equal(t, 100.0, stats.StorageUsedGB)
	assert.Equal(t, 1, stats.UserCount)
}

func TestCalculatePoolStorageCost(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	// 测试默认定价
	summary := &PoolUsageSummary{
		PoolID:             "pool1",
		TotalStorageUsedGB: 100,
	}
	cost := bm.calculatePoolStorageCost(summary)
	assert.Greater(t, cost, 0.0)
}

func TestCalculateBandwidthTieredCost(t *testing.T) {
	tiers := []BandwidthTier{
		{MinGB: 0, MaxGB: 100, PricePerGB: 0.5},
		{MinGB: 100, MaxGB: 1000, PricePerGB: 0.4},
		{MinGB: 1000, MaxGB: -1, PricePerGB: 0.3},
	}

	tests := []struct {
		amount   float64
		expected float64
	}{
		{50, 25.0},    // 50 * 0.5 = 25
		{150, 70.0},   // 100 * 0.5 + 50 * 0.4 = 50 + 20 = 70
		{2000, 710.0}, // 100 * 0.5 + 900 * 0.4 + 1000 * 0.3 = 50 + 360 + 300 = 710
	}

	for _, tt := range tests {
		result := calculateBandwidthTieredCost(tt.amount, tiers)
		assert.InDelta(t, tt.expected, result, 0.01)
	}
}

func TestExtractInvoiceNumber(t *testing.T) {
	tests := []struct {
		number   string
		expected int
	}{
		{"INV-20240101-0001", 1},
		{"INV-20240101-1234", 1234},
		{"INV-20240101-9999", 9999},
	}

	for _, tt := range tests {
		result := extractInvoiceNumber(tt.number)
		assert.Equal(t, tt.expected, result)
	}
}

func TestGetBandwidthUnitPrice(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	// 测试默认定价
	price := bm.getBandwidthUnitPrice(50)
	assert.Equal(t, 0.5, price)

	price = bm.getBandwidthUnitPrice(150)
	assert.Equal(t, 0.5, price) // 默认没有阶梯定价，返回基础价格
}

func TestLoadData(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "billing-test")
	require.NoError(t, err)
	defer os.RemoveAll(tmpDir)

	config := DefaultBillingConfig()

	// 创建第一个管理器并添加数据
	bm1, err := NewBillingManager(config, tmpDir)
	require.NoError(t, err)

	ctx := context.Background()
	_, err = bm1.RecordUsage(ctx, &UsageRecordInput{
		UserID:           "user1",
		PeriodStart:      time.Now().AddDate(0, 0, -30),
		PeriodEnd:        time.Now(),
		StorageUsedBytes: 100 * 1024 * 1024 * 1024,
	})
	require.NoError(t, err)

	// 创建第二个管理器，应该加载已有数据
	bm2, err := NewBillingManager(config, tmpDir)
	require.NoError(t, err)

	records, err := bm2.ListUsageRecords("", "", time.Time{}, time.Time{})
	require.NoError(t, err)
	assert.Len(t, records, 1)
}

func TestGetInvoiceByNumber(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	created := createTestInvoice(t, bm)

	retrieved, err := bm.GetInvoiceByNumber(created.InvoiceNumber)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)

	_, err = bm.GetInvoiceByNumber("nonexistent")
	assert.Error(t, err)
	assert.Equal(t, ErrInvoiceNotFound, err)
}

func TestGetPoolUsageSummaryNotFound(t *testing.T) {
	bm := createTestBillingManager(t)
	defer os.RemoveAll(bm.dataDir)

	_, err := bm.GetPoolUsageSummary("nonexistent", time.Time{}, time.Time{})
	assert.Error(t, err)
	assert.Equal(t, ErrUsageRecordNotFound, err)
}
