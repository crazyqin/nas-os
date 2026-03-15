// Package billing 提供发票管理功能测试
package billing

import (
	"context"
	"testing"

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
		StoragePath: tmpDir,
		AutoNumber:  true,
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

	results, total, err = im.QueryInvoices(ctx, InvoiceManagerQuery{UserIDs: []string{"user1"}})
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