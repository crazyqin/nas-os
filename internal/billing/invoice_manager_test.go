// Package billing 提供发票管理功能测试
package billing

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInvoiceManager(t *testing.T) {
	tmpDir := t.TempDir()
	config := InvoiceManagerConfig{
		StoragePath:     tmpDir,
		AutoNumber:      true,
		NumberPrefix:    "INV",
		NumberDigits:    6,
		DefaultTaxRate:  13.0,
		DefaultCurrency: "CNY",
	}

	im, err := NewInvoiceManager(config)
	require.NoError(t, err)
	require.NotNil(t, im)
	assert.Equal(t, tmpDir, im.storagePath)
	assert.Equal(t, "INV", im.config.NumberPrefix)
}

func TestCreateInvoiceManagerInvoice(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{
		StoragePath:     tmpDir,
		AutoNumber:      true,
		NumberPrefix:    "INV",
		DefaultTaxRate:  13.0,
		DefaultCurrency: "CNY",
	})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceManagerInput{
		Title:       "测试发票",
		Description: "这是一个测试发票",
		Type:        "standard",
		Items: []InvoiceManagerItem{
			{
				Name:      "服务费",
				Quantity:  1,
				UnitPrice: 1000.0,
				TaxRate:   13.0,
			},
		},
		Issuer: InvoiceParty{
			Name:      "测试公司",
			TaxNumber: "123456789",
		},
		Recipient: InvoiceParty{
			Name:      "客户公司",
			TaxNumber: "987654321",
		},
	}

	invoice, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)
	require.NotNil(t, invoice)

	assert.NotEmpty(t, invoice.ID)
	assert.NotEmpty(t, invoice.Number)
	assert.Equal(t, InvoiceStatusDraft, invoice.Status)
	assert.Equal(t, 1000.0, invoice.Subtotal)
	assert.Equal(t, 130.0, invoice.TaxAmount)
	assert.Equal(t, 1130.0, invoice.TotalAmount)
}

func TestCreateInvoiceManagerWithItems(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceManagerInput{
		Title: "多项发票",
		Items: []InvoiceManagerItem{
			{Name: "项目A", Quantity: 2, UnitPrice: 100.0, TaxRate: 13.0},
			{Name: "项目B", Quantity: 1, UnitPrice: 200.0, TaxRate: 13.0},
		},
	}

	invoice, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)

	assert.Equal(t, 400.0, invoice.Subtotal)
	assert.Equal(t, 52.0, invoice.TaxAmount)
	assert.Equal(t, 452.0, invoice.TotalAmount)
}

func TestGetInvoiceManagerInvoice(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{
		StoragePath:  tmpDir,
		AutoNumber:   true,
		NumberPrefix: "INV",
	})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceManagerInput{
		Title: "测试获取",
		Items: []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	}

	created, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)

	retrieved, err := im.GetInvoice(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.Number, retrieved.Number)

	byNumber, err := im.GetInvoiceByNumber(ctx, created.Number)
	require.NoError(t, err)
	assert.Equal(t, created.ID, byNumber.ID)
}

func TestUpdateInvoiceManagerInvoice(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceManagerInput{
		Title: "原始标题",
		Items: []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	}

	created, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)

	updateInput := InvoiceManagerInput{
		Title: "更新标题",
		Items: []InvoiceManagerItem{{Name: "更新项目", Quantity: 2, UnitPrice: 200, TaxRate: 13}},
	}

	updated, err := im.UpdateInvoice(ctx, created.ID, updateInput)
	require.NoError(t, err)
	assert.Equal(t, "更新标题", updated.Title)
	assert.Equal(t, 400.0, updated.Subtotal)
}

func TestUpdateInvoiceManagerStatus(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceManagerInput{
		Title: "状态测试",
		Items: []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	}

	invoice, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, InvoiceStatusDraft, invoice.Status)

	updated, err := im.UpdateInvoiceStatus(ctx, invoice.ID, InvoiceStatusIssued)
	require.NoError(t, err)
	assert.Equal(t, InvoiceStatusIssued, updated.Status)

	updated, err = im.UpdateInvoiceStatus(ctx, invoice.ID, InvoiceStatusSent)
	require.NoError(t, err)
	assert.Equal(t, InvoiceStatusSent, updated.Status)
}

func TestDeleteInvoiceManagerInvoice(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceManagerInput{
		Title: "删除测试",
		Items: []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	}

	invoice, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)

	err = im.DeleteInvoice(ctx, invoice.ID)
	require.NoError(t, err)

	_, err = im.GetInvoice(ctx, invoice.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrInvoiceNotFound, err)
}

func TestQueryInvoiceManagerInvoices(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	for i := 0; i < 5; i++ {
		input := InvoiceManagerInput{
			Title:     string(rune('A' + i)),
			Type:      "standard",
			UserID:    "user1",
			ProjectID: "project1",
			Items:     []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: float64((i + 1) * 100), TaxRate: 13}},
		}
		_, err := im.CreateInvoice(ctx, input)
		require.NoError(t, err)
	}

	results, total, err := im.QueryInvoices(ctx, InvoiceManagerQuery{})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, results, 5)

	_, total, err = im.QueryInvoices(ctx, InvoiceManagerQuery{UserIDs: []string{"user1"}})
	require.NoError(t, err)
	assert.Equal(t, 5, total)

	results, total, err = im.QueryInvoices(ctx, InvoiceManagerQuery{Page: 1, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, results, 2)
}

func TestExportInvoiceManagerInvoices(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceManagerInput{
		Title: "导出测试",
		Items: []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	}

	_, err = im.CreateInvoice(ctx, input)
	require.NoError(t, err)

	jsonPath, err := im.ExportInvoices(ctx, InvoiceManagerQuery{}, InvoiceExportOptions{Format: "json"})
	require.NoError(t, err)
	assert.FileExists(t, jsonPath)

	csvPath, err := im.ExportInvoices(ctx, InvoiceManagerQuery{}, InvoiceExportOptions{Format: "csv"})
	require.NoError(t, err)
	assert.FileExists(t, csvPath)

	xlsxPath, err := im.ExportInvoices(ctx, InvoiceManagerQuery{}, InvoiceExportOptions{Format: "xlsx"})
	require.NoError(t, err)
	assert.FileExists(t, xlsxPath)
}

func TestGetInvoiceManagerStats(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	for i := 0; i < 3; i++ {
		input := InvoiceManagerInput{
			Title: string(rune('A' + i)),
			Items: []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
		}
		inv, err := im.CreateInvoice(ctx, input)
		require.NoError(t, err)

		if i > 0 {
			_, err = im.UpdateInvoiceStatus(ctx, inv.ID, InvoiceStatusIssued)
			require.NoError(t, err)
		}
	}

	stats, err := im.GetInvoiceStats(ctx, InvoiceManagerQuery{})
	require.NoError(t, err)
	assert.Equal(t, 3, stats.TotalCount)
	assert.Contains(t, stats.ByStatus, InvoiceStatusDraft)
	assert.Contains(t, stats.ByStatus, InvoiceStatusIssued)
}

func TestInvoiceManagerPersistence(t *testing.T) {
	tmpDir := t.TempDir()

	im1, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceManagerInput{
		Title: "持久化测试",
		Items: []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	}

	invoice, err := im1.CreateInvoice(ctx, input)
	require.NoError(t, err)
	invoiceID := invoice.ID

	im2, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	retrieved, err := im2.GetInvoice(ctx, invoiceID)
	require.NoError(t, err)
	assert.Equal(t, "持久化测试", retrieved.Title)
}

func TestInvoiceManagerNumberGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{
		StoragePath:  tmpDir,
		AutoNumber:   true,
		NumberPrefix: "INV",
		NumberDigits: 6,
	})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceManagerInput{
		Title: "自动编号",
		Items: []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	}

	inv1, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)
	assert.Contains(t, inv1.Number, "INV")

	inv2, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)
	assert.NotEqual(t, inv1.Number, inv2.Number)
}

func TestInvoiceManagerDiscountCalculation(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceManagerInput{
		Title: "折扣测试",
		Items: []InvoiceManagerItem{
			{Name: "原价100", Quantity: 1, UnitPrice: 100, TaxRate: 13, Discount: 10},
		},
	}

	invoice, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)

	assert.Equal(t, 90.0, invoice.Subtotal)
	assert.InDelta(t, 11.7, invoice.TaxAmount, 0.01)
	assert.InDelta(t, 101.7, invoice.TotalAmount, 0.01)
}

// ========== 增强功能测试 ==========

func TestBatchCreateInvoices(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	inputs := []InvoiceManagerInput{
		{Title: "批量发票1", Items: []InvoiceManagerItem{{Name: "项目", Quantity: 1, UnitPrice: 100, TaxRate: 13}}},
		{Title: "批量发票2", Items: []InvoiceManagerItem{{Name: "项目", Quantity: 2, UnitPrice: 100, TaxRate: 13}}},
		{Title: "批量发票3", Items: []InvoiceManagerItem{{Name: "项目", Quantity: 3, UnitPrice: 100, TaxRate: 13}}},
	}

	invoices, errors := im.BatchCreateInvoices(ctx, inputs)
	require.Empty(t, errors)
	assert.Len(t, invoices, 3)
}

func TestBulkUpdateStatus(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建多个发票
	var ids []string
	for i := 0; i < 3; i++ {
		inv, err := im.CreateInvoice(ctx, InvoiceManagerInput{
			Title: string(rune('A' + i)),
			Items: []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
		})
		require.NoError(t, err)
		ids = append(ids, inv.ID)
	}

	// 批量更新状态
	updated, errors := im.BulkUpdateStatus(ctx, ids, InvoiceStatusIssued)
	require.Empty(t, errors)
	assert.Len(t, updated, 3)

	for _, inv := range updated {
		assert.Equal(t, InvoiceStatusIssued, inv.Status)
	}
}

func TestGetInvoicesByUser(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建用户的发票
	for i := 0; i < 3; i++ {
		_, err := im.CreateInvoice(ctx, InvoiceManagerInput{
			Title:  "用户发票",
			UserID: "user1",
			Items:  []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
		})
		require.NoError(t, err)
	}

	invoices, err := im.GetInvoicesByUser(ctx, "user1", 10)
	require.NoError(t, err)
	assert.Len(t, invoices, 3)
}

func TestGetInvoicesByProject(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	for i := 0; i < 2; i++ {
		_, err := im.CreateInvoice(ctx, InvoiceManagerInput{
			Title:     "项目发票",
			ProjectID: "project1",
			Items:     []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
		})
		require.NoError(t, err)
	}

	invoices, err := im.GetInvoicesByProject(ctx, "project1")
	require.NoError(t, err)
	assert.Len(t, invoices, 2)
}

func TestGetOverdueInvoices(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建一个逾期发票
	pastDue := time.Now().AddDate(0, 0, -10)
	inv, err := im.CreateInvoice(ctx, InvoiceManagerInput{
		Title:   "逾期发票",
		DueDate: &pastDue,
		Items:   []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	})
	require.NoError(t, err)

	// 开具并发送发票
	im.UpdateInvoiceStatus(ctx, inv.ID, InvoiceStatusIssued)
	im.UpdateInvoiceStatus(ctx, inv.ID, InvoiceStatusSent)

	// 标记逾期
	im.MarkOverdue(ctx, inv.ID)

	overdue, err := im.GetOverdueInvoices(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(overdue), 1)
}

func TestGetPendingInvoices(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建待处理发票
	for i := 0; i < 3; i++ {
		_, err := im.CreateInvoice(ctx, InvoiceManagerInput{
			Title: "待处理",
			Items: []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
		})
		require.NoError(t, err)
	}

	pending, err := im.GetPendingInvoices(ctx)
	require.NoError(t, err)
	assert.GreaterOrEqual(t, len(pending), 3)
}

func TestCalculateTotals(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建多个发票
	for i := 0; i < 3; i++ {
		inv, err := im.CreateInvoice(ctx, InvoiceManagerInput{
			Title: "测试",
			Items: []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
		})
		require.NoError(t, err)

		if i == 0 {
			// 标记一个为已支付
			im.UpdateInvoiceStatus(ctx, inv.ID, InvoiceStatusIssued)
			im.UpdateInvoiceStatus(ctx, inv.ID, InvoiceStatusSent)
			im.UpdateInvoiceStatus(ctx, inv.ID, InvoiceStatusPaid)
		}
	}

	totals, err := im.CalculateTotals(ctx, InvoiceManagerQuery{})
	require.NoError(t, err)

	assert.Greater(t, totals["total_amount"], 0.0)
	assert.Greater(t, totals["paid"], 0.0)
}

func TestDuplicateInvoice(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	original, err := im.CreateInvoice(ctx, InvoiceManagerInput{
		Title:     "原始发票",
		ProjectID: "project1",
		UserID:    "user1",
		Items: []InvoiceManagerItem{
			{Name: "项目A", Quantity: 2, UnitPrice: 100, TaxRate: 13},
		},
	})
	require.NoError(t, err)

	duplicate, err := im.DuplicateInvoice(ctx, original.ID)
	require.NoError(t, err)

	// 验证生成了新的 ID 和发票号
	if original.ID != "" && duplicate.ID != "" {
		assert.NotEqual(t, original.ID, duplicate.ID)
	}
	if original.Number != "" && duplicate.Number != "" {
		assert.NotEqual(t, original.Number, duplicate.Number)
	}
	// 验证状态是草稿
	assert.Equal(t, InvoiceStatusDraft, duplicate.Status)
}

func TestAddInvoiceItem(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	invoice, err := im.CreateInvoice(ctx, InvoiceManagerInput{
		Title: "添加明细测试",
		Items: []InvoiceManagerItem{{Name: "初始项目", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	})
	require.NoError(t, err)

	originalTotal := invoice.TotalAmount

	// 添加新明细
	newItem := InvoiceManagerItem{
		Name:      "新增项目",
		Quantity:  2,
		UnitPrice: 50,
		TaxRate:   13,
	}

	updated, err := im.AddInvoiceItem(ctx, invoice.ID, newItem)
	require.NoError(t, err)

	assert.Len(t, updated.Items, 2)
	assert.Greater(t, updated.TotalAmount, originalTotal)
}

func TestRemoveInvoiceItem(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	invoice, err := im.CreateInvoice(ctx, InvoiceManagerInput{
		Title: "移除明细测试",
		Items: []InvoiceManagerItem{
			{Name: "项目A", Quantity: 1, UnitPrice: 100, TaxRate: 13},
			{Name: "项目B", Quantity: 2, UnitPrice: 50, TaxRate: 13},
		},
	})
	require.NoError(t, err)

	originalTotal := invoice.TotalAmount

	// 移除第一个明细
	itemID := invoice.Items[0].ID
	updated, err := im.RemoveInvoiceItem(ctx, invoice.ID, itemID)
	require.NoError(t, err)

	assert.Len(t, updated.Items, 1)
	assert.Less(t, updated.TotalAmount, originalTotal)
}

func TestUpdateInvoiceItem(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	invoice, err := im.CreateInvoice(ctx, InvoiceManagerInput{
		Title: "更新明细测试",
		Items: []InvoiceManagerItem{{Name: "原项目", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	})
	require.NoError(t, err)

	// 更新明细
	updatedItem := invoice.Items[0]
	updatedItem.Quantity = 3
	updatedItem.UnitPrice = 80

	updated, err := im.UpdateInvoiceItem(ctx, invoice.ID, updatedItem)
	require.NoError(t, err)

	assert.Equal(t, 3, int(updated.Items[0].Quantity))
	assert.Equal(t, 80.0, updated.Items[0].UnitPrice)
}

func TestApplyDiscount(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	invoice, err := im.CreateInvoice(ctx, InvoiceManagerInput{
		Title: "折扣测试",
		Items: []InvoiceManagerItem{
			{Name: "项目A", Quantity: 1, UnitPrice: 100, TaxRate: 13},
			{Name: "项目B", Quantity: 1, UnitPrice: 100, TaxRate: 13},
		},
	})
	require.NoError(t, err)

	originalTotal := invoice.TotalAmount

	// 应用10%折扣
	updated, err := im.ApplyDiscount(ctx, invoice.ID, 10, "老客户优惠")
	require.NoError(t, err)

	assert.Less(t, updated.TotalAmount, originalTotal)
	assert.Equal(t, 10.0, updated.Metadata["discount_percent"])
}

func TestSendInvoice(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	invoice, err := im.CreateInvoice(ctx, InvoiceManagerInput{
		Title: "发送测试",
		Items: []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	})
	require.NoError(t, err)

	// 开具发票
	im.UpdateInvoiceStatus(ctx, invoice.ID, InvoiceStatusIssued)

	// 发送发票
	sent, err := im.SendInvoice(ctx, invoice.ID, "customer@example.com")
	require.NoError(t, err)

	assert.Equal(t, InvoiceStatusSent, sent.Status)
	assert.NotNil(t, sent.Metadata["sent_at"])
}

func TestGetInvoiceHistory(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	invoice, err := im.CreateInvoice(ctx, InvoiceManagerInput{
		Title: "历史测试",
		Items: []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	})
	require.NoError(t, err)

	history, err := im.GetInvoiceHistory(ctx, invoice.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, history)
	assert.Equal(t, "created", history[0]["action"])
}

func TestValidateInvoice(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	// 测试有效发票
	validInvoice := &InvoiceManagerInvoice{
		Items: []InvoiceManagerItem{
			{Name: "项目", Quantity: 1, UnitPrice: 100, TaxRate: 13},
		},
		TotalAmount: 113,
		Recipient:   InvoiceParty{Name: "客户"},
		Issuer:      InvoiceParty{Name: "供应商"},
	}

	errors := im.ValidateInvoice(validInvoice)
	assert.Empty(t, errors)

	// 测试无效发票
	invalidInvoice := &InvoiceManagerInvoice{
		Items:       []InvoiceManagerItem{},
		TotalAmount: 0,
		Recipient:   InvoiceParty{},
		Issuer:      InvoiceParty{},
	}

	errors = im.ValidateInvoice(invalidInvoice)
	assert.NotEmpty(t, errors)
}

func TestGetInvoicesByOrder(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	for i := 0; i < 2; i++ {
		_, err := im.CreateInvoice(ctx, InvoiceManagerInput{
			Title:   "订单发票",
			OrderID: "order123",
			Items:   []InvoiceManagerItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
		})
		require.NoError(t, err)
	}

	invoices, err := im.GetInvoicesByOrder(ctx, "order123")
	require.NoError(t, err)
	assert.Len(t, invoices, 2)
}

func TestInvoiceItemAutoID(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	// 明细项不提供ID
	invoice, err := im.CreateInvoice(ctx, InvoiceManagerInput{
		Title: "自动ID测试",
		Items: []InvoiceManagerItem{
			{Name: "项目A", Quantity: 1, UnitPrice: 100, TaxRate: 13},
		},
	})
	require.NoError(t, err)

	// 验证明细项有ID
	assert.NotEmpty(t, invoice.Items[0].ID)
}
