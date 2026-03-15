// Package billing 提供发票管理功能测试
package billing

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewInvoiceManager(t *testing.T) {
	// 创建临时目录
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

func TestCreateInvoice(t *testing.T) {
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
	input := InvoiceInput{
		Title:       "测试发票",
		Description: "这是一个测试发票",
		Type:        InvoiceTypeStandard,
		Items: []InvoiceItem{
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

func TestCreateInvoiceWithItems(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceInput{
		Title: "多项发票",
		Items: []InvoiceItem{
			{Name: "项目A", Quantity: 2, UnitPrice: 100.0, TaxRate: 13.0},
			{Name: "项目B", Quantity: 1, UnitPrice: 200.0, TaxRate: 13.0},
		},
	}

	invoice, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)

	// 验证金额计算
	// 项目A: 2 * 100 = 200, 税额: 26
	// 项目B: 1 * 200 = 200, 税额: 26
	// 小计: 400, 税额: 52, 总计: 452
	assert.Equal(t, 400.0, invoice.Subtotal)
	assert.Equal(t, 52.0, invoice.TaxAmount)
	assert.Equal(t, 452.0, invoice.TotalAmount)
}

func TestGetInvoice(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceInput{
		Title: "测试获取",
		Items: []InvoiceItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	}

	created, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)

	// 通过ID获取
	retrieved, err := im.GetInvoice(ctx, created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, retrieved.ID)
	assert.Equal(t, created.Number, retrieved.Number)

	// 通过发票号获取
	byNumber, err := im.GetInvoiceByNumber(ctx, created.Number)
	require.NoError(t, err)
	assert.Equal(t, created.ID, byNumber.ID)
}

func TestUpdateInvoice(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceInput{
		Title: "原始标题",
		Items: []InvoiceItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	}

	created, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)

	// 更新发票
	updateInput := InvoiceInput{
		Title: "更新标题",
		Items: []InvoiceItem{{Name: "更新项目", Quantity: 2, UnitPrice: 200, TaxRate: 13}},
	}

	updated, err := im.UpdateInvoice(ctx, created.ID, updateInput)
	require.NoError(t, err)
	assert.Equal(t, "更新标题", updated.Title)
	assert.Equal(t, 400.0, updated.Subtotal) // 2 * 200
}

func TestUpdateInvoiceStatus(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceInput{
		Title: "状态测试",
		Items: []InvoiceItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	}

	invoice, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)
	assert.Equal(t, InvoiceStatusDraft, invoice.Status)

	// 更新为待审核
	updated, err := im.UpdateInvoiceStatus(ctx, invoice.ID, InvoiceStatusPending)
	require.NoError(t, err)
	assert.Equal(t, InvoiceStatusPending, updated.Status)

	// 更新为已审核
	updated, err = im.UpdateInvoiceStatus(ctx, invoice.ID, InvoiceStatusApproved)
	require.NoError(t, err)
	assert.Equal(t, InvoiceStatusApproved, updated.Status)

	// 无效的状态转换
	_, err = im.UpdateInvoiceStatus(ctx, invoice.ID, InvoiceStatusDraft)
	assert.Error(t, err) // 已审核不能回到草稿
}

func TestDeleteInvoice(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceInput{
		Title: "删除测试",
		Items: []InvoiceItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	}

	invoice, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)

	// 删除草稿发票
	err = im.DeleteInvoice(ctx, invoice.ID)
	require.NoError(t, err)

	// 确认已删除
	_, err = im.GetInvoice(ctx, invoice.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrInvoiceNotFound, err)
}

func TestQueryInvoices(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建多个发票
	for i := 0; i < 5; i++ {
		input := InvoiceInput{
			Title:     fmt.Sprintf("发票%d", i),
			Type:      InvoiceTypeStandard,
			UserID:    "user1",
			ProjectID: "project1",
			Items:     []InvoiceItem{{Name: "测试", Quantity: 1, UnitPrice: float64((i + 1) * 100), TaxRate: 13}},
		}
		_, err := im.CreateInvoice(ctx, input)
		require.NoError(t, err)
	}

	// 查询所有
	results, total, err := im.QueryInvoices(ctx, InvoiceQuery{})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, results, 5)

	// 按用户查询
	results, total, err = im.QueryInvoices(ctx, InvoiceQuery{UserIDs: []string{"user1"}})
	require.NoError(t, err)
	assert.Equal(t, 5, total)

	// 分页查询
	results, total, err = im.QueryInvoices(ctx, InvoiceQuery{Page: 1, PageSize: 2})
	require.NoError(t, err)
	assert.Equal(t, 5, total)
	assert.Len(t, results, 2)
}

func TestExportInvoices(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceInput{
		Title: "导出测试",
		Items: []InvoiceItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	}

	_, err = im.CreateInvoice(ctx, input)
	require.NoError(t, err)

	// 导出JSON
	jsonPath, err := im.ExportInvoices(ctx, InvoiceQuery{}, InvoiceExportOptions{Format: "json"})
	require.NoError(t, err)
	assert.FileExists(t, jsonPath)

	// 导出CSV
	csvPath, err := im.ExportInvoices(ctx, InvoiceQuery{}, InvoiceExportOptions{Format: "csv"})
	require.NoError(t, err)
	assert.FileExists(t, csvPath)

	// 导出Excel
	xlsxPath, err := im.ExportInvoices(ctx, InvoiceQuery{}, InvoiceExportOptions{Format: "xlsx"})
	require.NoError(t, err)
	assert.FileExists(t, xlsxPath)
}

func TestGetInvoiceStats(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建不同状态的发票
	for i := 0; i < 3; i++ {
		input := InvoiceInput{
			Title: fmt.Sprintf("统计发票%d", i),
			Items: []InvoiceItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
		}
		inv, err := im.CreateInvoice(ctx, input)
		require.NoError(t, err)

		if i > 0 {
			_, err = im.UpdateInvoiceStatus(ctx, inv.ID, InvoiceStatusApproved)
			require.NoError(t, err)
		}
	}

	stats, err := im.GetInvoiceStats(ctx, InvoiceQuery{})
	require.NoError(t, err)
	assert.Equal(t, 3, stats.TotalCount)
	assert.Contains(t, stats.ByStatus, InvoiceStatusDraft)
	assert.Contains(t, stats.ByStatus, InvoiceStatusApproved)
}

func TestInvoicePersistence(t *testing.T) {
	tmpDir := t.TempDir()

	// 创建第一个管理器并添加发票
	im1, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceInput{
		Title: "持久化测试",
		Items: []InvoiceItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	}

	invoice, err := im1.CreateInvoice(ctx, input)
	require.NoError(t, err)
	invoiceID := invoice.ID

	// 创建新的管理器，应该能加载之前的数据
	im2, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	// 验证数据持久化
	retrieved, err := im2.GetInvoice(ctx, invoiceID)
	require.NoError(t, err)
	assert.Equal(t, "持久化测试", retrieved.Title)
}

func TestInvoiceNumberGeneration(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{
		StoragePath:  tmpDir,
		AutoNumber:   true,
		NumberPrefix: "INV",
		NumberDigits: 6,
	})
	require.NoError(t, err)

	ctx := context.Background()

	// 创建多个发票，验证号码递增
	input := InvoiceInput{
		Title: "自动编号",
		Items: []InvoiceItem{{Name: "测试", Quantity: 1, UnitPrice: 100, TaxRate: 13}},
	}

	inv1, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)
	assert.Contains(t, inv1.Number, "INV")

	inv2, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)
	assert.NotEqual(t, inv1.Number, inv2.Number)
}

func TestDiscountCalculation(t *testing.T) {
	tmpDir := t.TempDir()
	im, err := NewInvoiceManager(InvoiceManagerConfig{StoragePath: tmpDir})
	require.NoError(t, err)

	ctx := context.Background()
	input := InvoiceInput{
		Title: "折扣测试",
		Items: []InvoiceItem{
			{Name: "原价100", Quantity: 1, UnitPrice: 100, TaxRate: 13, Discount: 10}, // 10%折扣
		},
	}

	invoice, err := im.CreateInvoice(ctx, input)
	require.NoError(t, err)

	// 100 * (1 - 0.1) = 90
	// 税额: 90 * 0.13 = 11.7
	// 总计: 90 + 11.7 = 101.7
	assert.Equal(t, 90.0, invoice.Subtotal)
	assert.InDelta(t, 11.7, invoice.TaxAmount, 0.01)
	assert.InDelta(t, 101.7, invoice.TotalAmount, 0.01)
}