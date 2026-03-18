// Package billing 提供发票管理器功能
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/xuri/excelize/v2"
)

// ========== 额外错误定义 ==========

var (
	// ErrInvoiceNumberExists 发票号码已存在错误
	ErrInvoiceNumberExists = errors.New("发票号码已存在")
	// ErrInvoiceExportFailed 发票导出失败错误
	ErrInvoiceExportFailed = errors.New("发票导出失败")
)

// ========== 扩展类型 ==========

// InvoiceManagerInvoice 发票管理器使用的发票结构（扩展版）
type InvoiceManagerInvoice struct {
	ID          string        `json:"id"`
	Number      string        `json:"number"`      // 发票号码
	Type        string        `json:"type"`        // 发票类型
	Status      InvoiceStatus `json:"status"`      // 发票状态
	Title       string        `json:"title"`       // 发票抬头
	Description string        `json:"description"` // 描述

	// 开票方信息
	Issuer      InvoiceParty `json:"issuer"`
	Beneficiary InvoiceParty `json:"beneficiary"`

	// 收票方信息
	Recipient InvoiceParty `json:"recipient"`
	Payer     InvoiceParty `json:"payer"`

	// 金额信息
	Items       []InvoiceManagerItem `json:"items"`
	Subtotal    float64              `json:"subtotal"`
	TaxAmount   float64              `json:"tax_amount"`
	TotalAmount float64              `json:"total_amount"`
	Currency    string               `json:"currency"`

	// 税务信息
	TaxRate   float64 `json:"tax_rate"`
	TaxType   string  `json:"tax_type"`
	TaxNumber string  `json:"tax_number"`

	// 时间信息
	IssueDate time.Time  `json:"issue_date"`
	DueDate   *time.Time `json:"due_date"`
	PaidDate  *time.Time `json:"paid_date"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`

	// 支付信息
	PaymentMethod string `json:"payment_method"`
	PaymentRef    string `json:"payment_ref"`
	BankAccount   string `json:"bank_account"`
	BankName      string `json:"bank_name"`

	// 关联信息
	OrderID    string `json:"order_id"`
	ContractID string `json:"contract_id"`
	ProjectID  string `json:"project_id"`
	UserID     string `json:"user_id"`

	// 附加信息
	Remarks     string                 `json:"remarks"`
	Attachments []string               `json:"attachments"`
	Tags        []string               `json:"tags"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// InvoiceParty 发票相关方
type InvoiceParty struct {
	Name        string `json:"name"`
	TaxNumber   string `json:"tax_number"`
	Address     string `json:"address"`
	Phone       string `json:"phone"`
	BankName    string `json:"bank_name"`
	BankAccount string `json:"bank_account"`
	Email       string `json:"email"`
	Contact     string `json:"contact"`
}

// InvoiceManagerItem 发票明细项
type InvoiceManagerItem struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	Description string  `json:"description"`
	Quantity    float64 `json:"quantity"`
	Unit        string  `json:"unit"`
	UnitPrice   float64 `json:"unit_price"`
	Amount      float64 `json:"amount"`
	TaxRate     float64 `json:"tax_rate"`
	TaxAmount   float64 `json:"tax_amount"`
	TotalAmount float64 `json:"total_amount"`
	Discount    float64 `json:"discount"`
	Category    string  `json:"category"`
	Code        string  `json:"code"`
}

// InvoiceManagerInput 发票输入
type InvoiceManagerInput struct {
	Number        string                 `json:"number" binding:"required"`
	Type          string                 `json:"type"`
	Title         string                 `json:"title"`
	Description   string                 `json:"description"`
	Issuer        InvoiceParty           `json:"issuer"`
	Beneficiary   InvoiceParty           `json:"beneficiary"`
	Recipient     InvoiceParty           `json:"recipient"`
	Payer         InvoiceParty           `json:"payer"`
	Items         []InvoiceManagerItem   `json:"items" binding:"required"`
	TaxRate       float64                `json:"tax_rate"`
	TaxType       string                 `json:"tax_type"`
	TaxNumber     string                 `json:"tax_number"`
	IssueDate     *time.Time             `json:"issue_date"`
	DueDate       *time.Time             `json:"due_date"`
	PaymentMethod string                 `json:"payment_method"`
	BankAccount   string                 `json:"bank_account"`
	BankName      string                 `json:"bank_name"`
	OrderID       string                 `json:"order_id"`
	ContractID    string                 `json:"contract_id"`
	ProjectID     string                 `json:"project_id"`
	UserID        string                 `json:"user_id"`
	Remarks       string                 `json:"remarks"`
	Tags          []string               `json:"tags"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// InvoiceManagerQuery 发票查询参数
type InvoiceManagerQuery struct {
	IDs        []string        `json:"ids,omitempty"`
	Numbers    []string        `json:"numbers,omitempty"`
	Statuses   []InvoiceStatus `json:"statuses,omitempty"`
	Types      []string        `json:"types,omitempty"`
	UserIDs    []string        `json:"user_ids,omitempty"`
	ProjectIDs []string        `json:"project_ids,omitempty"`
	OrderIDs   []string        `json:"order_ids,omitempty"`
	StartDate  *time.Time      `json:"start_date,omitempty"`
	EndDate    *time.Time      `json:"end_date,omitempty"`
	MinAmount  *float64        `json:"min_amount,omitempty"`
	MaxAmount  *float64        `json:"max_amount,omitempty"`
	Keyword    string          `json:"keyword,omitempty"`
	Tags       []string        `json:"tags,omitempty"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	SortBy     string          `json:"sort_by"`
	SortOrder  string          `json:"sort_order"`
}

// InvoiceExportOptions 发票导出选项
type InvoiceExportOptions struct {
	Format       string `json:"format"`
	Filename     string `json:"filename"`
	OutputPath   string `json:"output_path"`
	IncludeItems bool   `json:"include_items"`
	DateRange    bool   `json:"date_range"`
	Summary      bool   `json:"summary"`
}

// InvoiceManagerStats 发票统计
type InvoiceManagerStats struct {
	TotalCount     int                           `json:"total_count"`
	TotalAmount    float64                       `json:"total_amount"`
	TotalTaxAmount float64                       `json:"total_tax_amount"`
	ByStatus       map[InvoiceStatus]StatusStats `json:"by_status"`
	ByType         map[string]TypeStats          `json:"by_type"`
	ByMonth        []MonthlyStats                `json:"by_month"`
	PendingAmount  float64                       `json:"pending_amount"`
	OverdueCount   int                           `json:"overdue_count"`
	OverdueAmount  float64                       `json:"overdue_amount"`
}

// StatusStats 状态统计
type StatusStats struct {
	Count  int     `json:"count"`
	Amount float64 `json:"amount"`
}

// TypeStats 类型统计
type TypeStats struct {
	Count  int     `json:"count"`
	Amount float64 `json:"amount"`
}

// MonthlyStats 月度统计
type MonthlyStats struct {
	Month     string  `json:"month"`
	Count     int     `json:"count"`
	Amount    float64 `json:"amount"`
	TaxAmount float64 `json:"tax_amount"`
}

// ========== 发票管理器 ==========

// InvoiceManager 发票管理器
type InvoiceManager struct {
	mu          sync.RWMutex
	invoices    map[string]*InvoiceManagerInvoice
	numberIndex map[string]string
	storagePath string
	config      InvoiceManagerConfig
}

// InvoiceManagerConfig 发票管理器配置
type InvoiceManagerConfig struct {
	StoragePath     string  `json:"storage_path"`
	AutoNumber      bool    `json:"auto_number"`
	NumberPrefix    string  `json:"number_prefix"`
	NumberDigits    int     `json:"number_digits"`
	DefaultTaxRate  float64 `json:"default_tax_rate"`
	DefaultCurrency string  `json:"default_currency"`
}

// NewInvoiceManager 创建发票管理器
func NewInvoiceManager(config InvoiceManagerConfig) (*InvoiceManager, error) {
	if config.StoragePath == "" {
		config.StoragePath = "./data/invoices"
	}
	if config.NumberDigits == 0 {
		config.NumberDigits = 8
	}
	if config.DefaultCurrency == "" {
		config.DefaultCurrency = "CNY"
	}

	im := &InvoiceManager{
		invoices:    make(map[string]*InvoiceManagerInvoice),
		numberIndex: make(map[string]string),
		storagePath: config.StoragePath,
		config:      config,
	}

	if err := os.MkdirAll(config.StoragePath, 0755); err != nil {
		return nil, fmt.Errorf("创建存储目录失败: %w", err)
	}

	if err := im.loadInvoices(); err != nil {
		return nil, fmt.Errorf("加载发票失败: %w", err)
	}

	return im, nil
}

func (im *InvoiceManager) loadInvoices() error {
	files, err := filepath.Glob(filepath.Join(im.storagePath, "*.json"))
	if err != nil {
		return err
	}

	for _, file := range files {
		data, err := os.ReadFile(file)
		if err != nil {
			continue
		}

		var invoice InvoiceManagerInvoice
		if err := json.Unmarshal(data, &invoice); err != nil {
			continue
		}

		im.invoices[invoice.ID] = &invoice
		if invoice.Number != "" {
			im.numberIndex[invoice.Number] = invoice.ID
		}
	}

	return nil
}

// CreateInvoice 创建发票
func (im *InvoiceManager) CreateInvoice(ctx context.Context, input InvoiceManagerInput) (*InvoiceManagerInvoice, error) {
	im.mu.Lock()
	defer im.mu.Unlock()

	if input.Number != "" {
		if _, exists := im.numberIndex[input.Number]; exists {
			return nil, ErrInvoiceNumberExists
		}
	}

	id := uuid.New().String()
	number := input.Number
	if number == "" && im.config.AutoNumber {
		number = im.generateInvoiceNumber()
	}

	items := input.Items
	if len(items) == 0 {
		return nil, errors.New("无效的发票数据")
	}

	subtotal := 0.0
	taxAmount := 0.0
	for i := range items {
		if items[i].ID == "" {
			items[i].ID = uuid.New().String()
		}
		items[i].Amount = items[i].Quantity * items[i].UnitPrice * (1 - items[i].Discount/100)
		if items[i].TaxRate == 0 {
			items[i].TaxRate = input.TaxRate
		}
		items[i].TaxAmount = items[i].Amount * items[i].TaxRate / 100
		items[i].TotalAmount = items[i].Amount + items[i].TaxAmount
		subtotal += items[i].Amount
		taxAmount += items[i].TaxAmount
	}

	taxRate := input.TaxRate
	if taxRate == 0 {
		taxRate = im.config.DefaultTaxRate
	}

	now := time.Now()
	issueDate := now
	if input.IssueDate != nil {
		issueDate = *input.IssueDate
	}

	invoice := &InvoiceManagerInvoice{
		ID:            id,
		Number:        number,
		Type:          input.Type,
		Status:        InvoiceStatusDraft,
		Title:         input.Title,
		Description:   input.Description,
		Issuer:        input.Issuer,
		Beneficiary:   input.Beneficiary,
		Recipient:     input.Recipient,
		Payer:         input.Payer,
		Items:         items,
		Subtotal:      subtotal,
		TaxAmount:     taxAmount,
		TotalAmount:   subtotal + taxAmount,
		Currency:      im.config.DefaultCurrency,
		TaxRate:       taxRate,
		TaxType:       input.TaxType,
		TaxNumber:     input.TaxNumber,
		IssueDate:     issueDate,
		DueDate:       input.DueDate,
		CreatedAt:     now,
		UpdatedAt:     now,
		PaymentMethod: input.PaymentMethod,
		BankAccount:   input.BankAccount,
		BankName:      input.BankName,
		OrderID:       input.OrderID,
		ContractID:    input.ContractID,
		ProjectID:     input.ProjectID,
		UserID:        input.UserID,
		Remarks:       input.Remarks,
		Tags:          input.Tags,
		Metadata:      input.Metadata,
	}

	if err := im.saveInvoice(invoice); err != nil {
		return nil, err
	}

	im.invoices[id] = invoice
	if number != "" {
		im.numberIndex[number] = id
	}

	return invoice, nil
}

func (im *InvoiceManager) generateInvoiceNumber() string {
	now := time.Now()
	seq := len(im.invoices) + 1
	return fmt.Sprintf("%s%s%0*d", im.config.NumberPrefix, now.Format("20060102"), im.config.NumberDigits, seq)
}

// GetInvoice 获取发票
func (im *InvoiceManager) GetInvoice(ctx context.Context, id string) (*InvoiceManagerInvoice, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	invoice, exists := im.invoices[id]
	if !exists {
		return nil, ErrInvoiceNotFound
	}
	return invoice, nil
}

// GetInvoiceByNumber 按发票号获取
func (im *InvoiceManager) GetInvoiceByNumber(ctx context.Context, number string) (*InvoiceManagerInvoice, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	id, exists := im.numberIndex[number]
	if !exists {
		return nil, ErrInvoiceNotFound
	}
	return im.invoices[id], nil
}

// UpdateInvoice 更新发票
func (im *InvoiceManager) UpdateInvoice(ctx context.Context, id string, input InvoiceManagerInput) (*InvoiceManagerInvoice, error) {
	im.mu.Lock()
	defer im.mu.Unlock()

	invoice, exists := im.invoices[id]
	if !exists {
		return nil, ErrInvoiceNotFound
	}

	if invoice.Status != InvoiceStatusDraft {
		return nil, fmt.Errorf("只有草稿状态的发票可以修改")
	}

	if input.Title != "" {
		invoice.Title = input.Title
	}
	if input.Description != "" {
		invoice.Description = input.Description
	}
	if len(input.Items) > 0 {
		invoice.Items = input.Items
		subtotal := 0.0
		taxAmount := 0.0
		for i := range invoice.Items {
			if invoice.Items[i].ID == "" {
				invoice.Items[i].ID = uuid.New().String()
			}
			invoice.Items[i].Amount = invoice.Items[i].Quantity * invoice.Items[i].UnitPrice * (1 - invoice.Items[i].Discount/100)
			if invoice.Items[i].TaxRate == 0 {
				invoice.Items[i].TaxRate = input.TaxRate
			}
			invoice.Items[i].TaxAmount = invoice.Items[i].Amount * invoice.Items[i].TaxRate / 100
			invoice.Items[i].TotalAmount = invoice.Items[i].Amount + invoice.Items[i].TaxAmount
			subtotal += invoice.Items[i].Amount
			taxAmount += invoice.Items[i].TaxAmount
		}
		invoice.Subtotal = subtotal
		invoice.TaxAmount = taxAmount
		invoice.TotalAmount = subtotal + taxAmount
	}
	if input.TaxRate > 0 {
		invoice.TaxRate = input.TaxRate
	}
	if input.Remarks != "" {
		invoice.Remarks = input.Remarks
	}
	if len(input.Tags) > 0 {
		invoice.Tags = input.Tags
	}

	invoice.UpdatedAt = time.Now()
	if err := im.saveInvoice(invoice); err != nil {
		return nil, err
	}
	return invoice, nil
}

// UpdateInvoiceStatus 更新发票状态
func (im *InvoiceManager) UpdateInvoiceStatus(ctx context.Context, id string, status InvoiceStatus) (*InvoiceManagerInvoice, error) {
	im.mu.Lock()
	defer im.mu.Unlock()

	invoice, exists := im.invoices[id]
	if !exists {
		return nil, ErrInvoiceNotFound
	}

	if !isValidStatusTransition(invoice.Status, status) {
		return nil, fmt.Errorf("无效的状态转换: %s -> %s", invoice.Status, status)
	}

	invoice.Status = status
	invoice.UpdatedAt = time.Now()

	if status == InvoiceStatusPaid {
		now := time.Now()
		invoice.PaidDate = &now
	}

	if err := im.saveInvoice(invoice); err != nil {
		return nil, err
	}
	return invoice, nil
}

func isValidStatusTransition(from, to InvoiceStatus) bool {
	validTransitions := map[InvoiceStatus][]InvoiceStatus{
		InvoiceStatusDraft:    {InvoiceStatusIssued, InvoiceStatusVoid},
		InvoiceStatusIssued:   {InvoiceStatusSent, InvoiceStatusVoid},
		InvoiceStatusSent:     {InvoiceStatusPaid, InvoiceStatusOverdue, InvoiceStatusVoid},
		InvoiceStatusPaid:     {InvoiceStatusRefunded},
		InvoiceStatusOverdue:  {InvoiceStatusPaid, InvoiceStatusVoid},
		InvoiceStatusVoid:     {},
		InvoiceStatusRefunded: {},
	}

	allowed, exists := validTransitions[from]
	if !exists {
		return false
	}

	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// DeleteInvoice 删除发票
func (im *InvoiceManager) DeleteInvoice(ctx context.Context, id string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	invoice, exists := im.invoices[id]
	if !exists {
		return ErrInvoiceNotFound
	}

	if invoice.Status != InvoiceStatusDraft && invoice.Status != InvoiceStatusVoid {
		return fmt.Errorf("只有草稿或已作废的发票可以删除")
	}

	filePath := filepath.Join(im.storagePath, id+".json")
	_ = os.Remove(filePath) // 清理操作，忽略错误

	delete(im.invoices, id)
	if invoice.Number != "" {
		delete(im.numberIndex, invoice.Number)
	}
	return nil
}

// QueryInvoices 查询发票
func (im *InvoiceManager) QueryInvoices(ctx context.Context, query InvoiceManagerQuery) ([]*InvoiceManagerInvoice, int, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	var results []*InvoiceManagerInvoice

	for _, invoice := range im.invoices {
		if !im.matchQuery(invoice, query) {
			continue
		}
		results = append(results, invoice)
	}

	im.sortInvoices(results, query.SortBy, query.SortOrder)

	total := len(results)
	page := query.Page
	if page < 1 {
		page = 1
	}
	pageSize := query.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize
	if start > total {
		return []*InvoiceManagerInvoice{}, total, nil
	}
	if end > total {
		end = total
	}

	return results[start:end], total, nil
}

func (im *InvoiceManager) matchQuery(invoice *InvoiceManagerInvoice, query InvoiceManagerQuery) bool {
	if len(query.IDs) > 0 {
		found := false
		for _, id := range query.IDs {
			if invoice.ID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(query.Numbers) > 0 {
		found := false
		for _, num := range query.Numbers {
			if invoice.Number == num {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(query.Statuses) > 0 {
		found := false
		for _, status := range query.Statuses {
			if invoice.Status == status {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if len(query.UserIDs) > 0 {
		found := false
		for _, uid := range query.UserIDs {
			if invoice.UserID == uid {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	if query.StartDate != nil && invoice.IssueDate.Before(*query.StartDate) {
		return false
	}
	if query.EndDate != nil && invoice.IssueDate.After(*query.EndDate) {
		return false
	}

	if query.MinAmount != nil && invoice.TotalAmount < *query.MinAmount {
		return false
	}
	if query.MaxAmount != nil && invoice.TotalAmount > *query.MaxAmount {
		return false
	}

	if query.Keyword != "" {
		keyword := query.Keyword
		if !containsIgnoreCase(invoice.Title, keyword) &&
			!containsIgnoreCase(invoice.Number, keyword) &&
			!containsIgnoreCase(invoice.Description, keyword) &&
			!containsIgnoreCase(invoice.Recipient.Name, keyword) {
			return false
		}
	}

	return true
}

func (im *InvoiceManager) sortInvoices(invoices []*InvoiceManagerInvoice, sortBy, sortOrder string) {
	if sortBy == "" {
		sortBy = "created_at"
	}
	if sortOrder == "" {
		sortOrder = "desc"
	}

	for i := 0; i < len(invoices)-1; i++ {
		for j := i + 1; j < len(invoices); j++ {
			var swap bool
			switch sortBy {
			case "number":
				swap = invoices[i].Number > invoices[j].Number
			case "issue_date":
				swap = invoices[i].IssueDate.After(invoices[j].IssueDate)
			case "total_amount":
				swap = invoices[i].TotalAmount > invoices[j].TotalAmount
			case "status":
				swap = invoices[i].Status > invoices[j].Status
			default:
				swap = invoices[i].CreatedAt.After(invoices[j].CreatedAt)
			}

			if sortOrder == "desc" {
				swap = !swap
			}

			if swap {
				invoices[i], invoices[j] = invoices[j], invoices[i]
			}
		}
	}
}

// ExportInvoices 导出发票
func (im *InvoiceManager) ExportInvoices(ctx context.Context, query InvoiceManagerQuery, options InvoiceExportOptions) (string, error) {
	invoices, _, err := im.QueryInvoices(ctx, query)
	if err != nil {
		return "", err
	}

	if len(invoices) == 0 {
		return "", ErrInvoiceNotFound
	}

	outputPath := options.OutputPath
	if outputPath == "" {
		outputPath = im.storagePath
	}

	filename := options.Filename
	if filename == "" {
		filename = fmt.Sprintf("invoices_%s", time.Now().Format("20060102_150405"))
	}

	switch options.Format {
	case "json":
		return im.exportToJSON(invoices, outputPath, filename)
	case "csv":
		return im.exportToCSV(invoices, outputPath, filename)
	case "xlsx":
		return im.exportToExcel(invoices, outputPath, filename)
	default:
		return im.exportToJSON(invoices, outputPath, filename)
	}
}

func (im *InvoiceManager) exportToJSON(invoices []*InvoiceManagerInvoice, outputPath, filename string) (string, error) {
	filePath := filepath.Join(outputPath, filename+".json")
	data, err := json.MarshalIndent(invoices, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化失败: %w", err)
	}
	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}
	return filePath, nil
}

func (im *InvoiceManager) exportToCSV(invoices []*InvoiceManagerInvoice, outputPath, filename string) (string, error) {
	filePath := filepath.Join(outputPath, filename+".csv")
	var lines []string
	lines = append(lines, "ID,发票号,类型,状态,抬头,金额,税额,总金额,开票日期,收款方,备注")

	for _, inv := range invoices {
		line := fmt.Sprintf("%s,%s,%s,%s,%s,%.2f,%.2f,%.2f,%s,%s,%s",
			inv.ID, inv.Number, inv.Type, inv.Status, inv.Title,
			inv.Subtotal, inv.TaxAmount, inv.TotalAmount,
			inv.IssueDate.Format("2006-01-02"), inv.Recipient.Name, inv.Remarks)
		lines = append(lines, line)
	}

	data := []byte{}
	for _, line := range lines {
		data = append(data, []byte(line+"\n")...)
	}

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return "", fmt.Errorf("写入文件失败: %w", err)
	}
	return filePath, nil
}

func (im *InvoiceManager) exportToExcel(invoices []*InvoiceManagerInvoice, outputPath, filename string) (string, error) {
	filePath := filepath.Join(outputPath, filename+".xlsx")
	f := excelize.NewFile()
	defer func() {
		if err := f.Close(); err != nil {
			// 记录关闭错误，但不影响主流程
			log.Printf("关闭 Excel 文件失败: %v", err)
		}
	}()

	sheet := "发票列表"
	if err := f.SetSheetName("Sheet1", sheet); err != nil {
		return "", fmt.Errorf("设置工作表名称失败: %w", err)
	}

	headers := []string{"ID", "发票号", "类型", "状态", "抬头", "金额", "税额", "总金额", "开票日期", "收款方", "备注"}
	for i, h := range headers {
		cell, err := excelize.CoordinatesToCellName(i+1, 1)
		if err != nil {
			return "", fmt.Errorf("获取单元格坐标失败: %w", err)
		}
		if err := f.SetCellValue(sheet, cell, h); err != nil {
			return "", fmt.Errorf("设置表头失败: %w", err)
		}
	}

	for i, inv := range invoices {
		row := i + 2
		if err := f.SetCellValue(sheet, fmt.Sprintf("A%d", row), inv.ID); err != nil {
			return "", fmt.Errorf("设置单元格失败: %w", err)
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("B%d", row), inv.Number); err != nil {
			return "", fmt.Errorf("设置单元格失败: %w", err)
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("C%d", row), inv.Type); err != nil {
			return "", fmt.Errorf("设置单元格失败: %w", err)
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("D%d", row), inv.Status); err != nil {
			return "", fmt.Errorf("设置单元格失败: %w", err)
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("E%d", row), inv.Title); err != nil {
			return "", fmt.Errorf("设置单元格失败: %w", err)
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("F%d", row), inv.Subtotal); err != nil {
			return "", fmt.Errorf("设置单元格失败: %w", err)
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("G%d", row), inv.TaxAmount); err != nil {
			return "", fmt.Errorf("设置单元格失败: %w", err)
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("H%d", row), inv.TotalAmount); err != nil {
			return "", fmt.Errorf("设置单元格失败: %w", err)
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("I%d", row), inv.IssueDate.Format("2006-01-02")); err != nil {
			return "", fmt.Errorf("设置单元格失败: %w", err)
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("J%d", row), inv.Recipient.Name); err != nil {
			return "", fmt.Errorf("设置单元格失败: %w", err)
		}
		if err := f.SetCellValue(sheet, fmt.Sprintf("K%d", row), inv.Remarks); err != nil {
			return "", fmt.Errorf("设置单元格失败: %w", err)
		}
	}

	if err := f.SaveAs(filePath); err != nil {
		return "", fmt.Errorf("保存Excel失败: %w", err)
	}
	return filePath, nil
}

// GetInvoiceStats 获取发票统计
func (im *InvoiceManager) GetInvoiceStats(ctx context.Context, query InvoiceManagerQuery) (*InvoiceManagerStats, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	stats := &InvoiceManagerStats{
		ByStatus: make(map[InvoiceStatus]StatusStats),
		ByType:   make(map[string]TypeStats),
	}

	now := time.Now()

	for _, invoice := range im.invoices {
		if !im.matchQuery(invoice, query) {
			continue
		}

		stats.TotalCount++
		stats.TotalAmount += invoice.TotalAmount
		stats.TotalTaxAmount += invoice.TaxAmount

		ss := stats.ByStatus[invoice.Status]
		ss.Count++
		ss.Amount += invoice.TotalAmount
		stats.ByStatus[invoice.Status] = ss

		ts := stats.ByType[invoice.Type]
		ts.Count++
		ts.Amount += invoice.TotalAmount
		stats.ByType[invoice.Type] = ts

		if invoice.Status == InvoiceStatusSent || invoice.Status == InvoiceStatusOverdue {
			stats.PendingAmount += invoice.TotalAmount
		}

		if invoice.DueDate != nil && invoice.DueDate.Before(now) &&
			invoice.Status != InvoiceStatusPaid && invoice.Status != InvoiceStatusVoid {
			stats.OverdueCount++
			stats.OverdueAmount += invoice.TotalAmount
		}
	}

	return stats, nil
}

func (im *InvoiceManager) saveInvoice(invoice *InvoiceManagerInvoice) error {
	data, err := json.MarshalIndent(invoice, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化发票失败: %w", err)
	}

	filePath := filepath.Join(im.storagePath, invoice.ID+".json")
	return os.WriteFile(filePath, data, 0644)
}

func containsIgnoreCase(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr ||
		(len(s) > len(substr) && containsLower(s, substr)))
}

func containsLower(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			sc := s[i+j]
			if sc >= 'A' && sc <= 'Z' {
				sc += 32
			}
			subc := substr[j]
			if subc >= 'A' && subc <= 'Z' {
				subc += 32
			}
			if sc != subc {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

// ========== 增强功能 ==========

// BatchCreateInvoices 批量创建发票
func (im *InvoiceManager) BatchCreateInvoices(ctx context.Context, inputs []InvoiceManagerInput) ([]*InvoiceManagerInvoice, []error) {
	var invoices []*InvoiceManagerInvoice
	var errors []error

	for i, input := range inputs {
		invoice, err := im.CreateInvoice(ctx, input)
		if err != nil {
			errors = append(errors, fmt.Errorf("发票 %d: %w", i, err))
			continue
		}
		invoices = append(invoices, invoice)
	}

	return invoices, errors
}

// BulkUpdateStatus 批量更新发票状态
func (im *InvoiceManager) BulkUpdateStatus(ctx context.Context, ids []string, status InvoiceStatus) ([]*InvoiceManagerInvoice, []error) {
	var updated []*InvoiceManagerInvoice
	var errors []error

	for _, id := range ids {
		invoice, err := im.UpdateInvoiceStatus(ctx, id, status)
		if err != nil {
			errors = append(errors, fmt.Errorf("发票 %s: %w", id, err))
			continue
		}
		updated = append(updated, invoice)
	}

	return updated, errors
}

// GetInvoicesByUser 获取用户的发票列表
func (im *InvoiceManager) GetInvoicesByUser(ctx context.Context, userID string, limit int) ([]*InvoiceManagerInvoice, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	var invoices []*InvoiceManagerInvoice
	for _, invoice := range im.invoices {
		if invoice.UserID == userID {
			invoices = append(invoices, invoice)
		}
	}

	// 按创建时间排序（最新的在前）
	for i := 0; i < len(invoices)-1; i++ {
		for j := i + 1; j < len(invoices); j++ {
			if invoices[i].CreatedAt.Before(invoices[j].CreatedAt) {
				invoices[i], invoices[j] = invoices[j], invoices[i]
			}
		}
	}

	if limit > 0 && len(invoices) > limit {
		invoices = invoices[:limit]
	}

	return invoices, nil
}

// GetInvoicesByProject 获取项目的发票列表
func (im *InvoiceManager) GetInvoicesByProject(ctx context.Context, projectID string) ([]*InvoiceManagerInvoice, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	var invoices []*InvoiceManagerInvoice
	for _, invoice := range im.invoices {
		if invoice.ProjectID == projectID {
			invoices = append(invoices, invoice)
		}
	}

	return invoices, nil
}

// GetInvoicesByOrder 获取订单相关的发票
func (im *InvoiceManager) GetInvoicesByOrder(ctx context.Context, orderID string) ([]*InvoiceManagerInvoice, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	var invoices []*InvoiceManagerInvoice
	for _, invoice := range im.invoices {
		if invoice.OrderID == orderID {
			invoices = append(invoices, invoice)
		}
	}

	return invoices, nil
}

// GetOverdueInvoices 获取逾期发票
func (im *InvoiceManager) GetOverdueInvoices(ctx context.Context) ([]*InvoiceManagerInvoice, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	now := time.Now()
	var invoices []*InvoiceManagerInvoice

	for _, invoice := range im.invoices {
		if invoice.DueDate != nil && invoice.DueDate.Before(now) {
			if invoice.Status != InvoiceStatusPaid && invoice.Status != InvoiceStatusVoid && invoice.Status != InvoiceStatusRefunded {
				invoices = append(invoices, invoice)
			}
		}
	}

	return invoices, nil
}

// GetPendingInvoices 获取待处理发票
func (im *InvoiceManager) GetPendingInvoices(ctx context.Context) ([]*InvoiceManagerInvoice, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	var invoices []*InvoiceManagerInvoice
	for _, invoice := range im.invoices {
		if invoice.Status == InvoiceStatusDraft || invoice.Status == InvoiceStatusIssued || invoice.Status == InvoiceStatusSent {
			invoices = append(invoices, invoice)
		}
	}

	return invoices, nil
}

// CalculateTotals 计算发票总额
func (im *InvoiceManager) CalculateTotals(ctx context.Context, query InvoiceManagerQuery) (map[string]float64, error) {
	invoices, _, err := im.QueryInvoices(ctx, query)
	if err != nil {
		return nil, err
	}

	totals := map[string]float64{
		"subtotal":     0,
		"tax_amount":   0,
		"total_amount": 0,
		"paid":         0,
		"pending":      0,
		"overdue":      0,
	}

	now := time.Now()
	for _, inv := range invoices {
		totals["subtotal"] += inv.Subtotal
		totals["tax_amount"] += inv.TaxAmount
		totals["total_amount"] += inv.TotalAmount

		switch inv.Status {
		case InvoiceStatusPaid:
			totals["paid"] += inv.TotalAmount
		case InvoiceStatusOverdue:
			totals["overdue"] += inv.TotalAmount
		default:
			if inv.DueDate != nil && inv.DueDate.Before(now) {
				totals["overdue"] += inv.TotalAmount
			} else {
				totals["pending"] += inv.TotalAmount
			}
		}
	}

	return totals, nil
}

// DuplicateInvoice 复制发票
func (im *InvoiceManager) DuplicateInvoice(ctx context.Context, id string) (*InvoiceManagerInvoice, error) {
	original, err := im.GetInvoice(ctx, id)
	if err != nil {
		return nil, err
	}

	input := InvoiceManagerInput{
		Type:          original.Type,
		Title:         original.Title + " (副本)",
		Description:   original.Description,
		Issuer:        original.Issuer,
		Beneficiary:   original.Beneficiary,
		Recipient:     original.Recipient,
		Payer:         original.Payer,
		Items:         original.Items,
		TaxRate:       original.TaxRate,
		TaxType:       original.TaxType,
		TaxNumber:     original.TaxNumber,
		PaymentMethod: original.PaymentMethod,
		BankAccount:   original.BankAccount,
		BankName:      original.BankName,
		OrderID:       original.OrderID,
		ContractID:    original.ContractID,
		ProjectID:     original.ProjectID,
		UserID:        original.UserID,
		Remarks:       original.Remarks,
		Tags:          original.Tags,
	}

	return im.CreateInvoice(ctx, input)
}

// AddInvoiceItem 添加发票明细项
func (im *InvoiceManager) AddInvoiceItem(ctx context.Context, invoiceID string, item InvoiceManagerItem) (*InvoiceManagerInvoice, error) {
	im.mu.Lock()
	defer im.mu.Unlock()

	invoice, exists := im.invoices[invoiceID]
	if !exists {
		return nil, ErrInvoiceNotFound
	}

	if invoice.Status != InvoiceStatusDraft {
		return nil, fmt.Errorf("只有草稿状态的发票可以添加明细")
	}

	// 生成明细ID
	if item.ID == "" {
		item.ID = uuid.New().String()
	}

	// 计算明细金额
	item.Amount = item.Quantity * item.UnitPrice * (1 - item.Discount/100)
	if item.TaxRate == 0 {
		item.TaxRate = invoice.TaxRate
	}
	item.TaxAmount = item.Amount * item.TaxRate / 100
	item.TotalAmount = item.Amount + item.TaxAmount

	invoice.Items = append(invoice.Items, item)

	// 重新计算总额
	im.recalculateInvoiceTotals(invoice)
	invoice.UpdatedAt = time.Now()

	if err := im.saveInvoice(invoice); err != nil {
		return nil, err
	}

	return invoice, nil
}

// RemoveInvoiceItem 移除发票明细项
func (im *InvoiceManager) RemoveInvoiceItem(ctx context.Context, invoiceID, itemID string) (*InvoiceManagerInvoice, error) {
	im.mu.Lock()
	defer im.mu.Unlock()

	invoice, exists := im.invoices[invoiceID]
	if !exists {
		return nil, ErrInvoiceNotFound
	}

	if invoice.Status != InvoiceStatusDraft {
		return nil, fmt.Errorf("只有草稿状态的发票可以移除明细")
	}

	// 查找并移除明细
	found := false
	newItems := make([]InvoiceManagerItem, 0, len(invoice.Items)-1)
	for _, item := range invoice.Items {
		if item.ID == itemID {
			found = true
			continue
		}
		newItems = append(newItems, item)
	}

	if !found {
		return nil, fmt.Errorf("明细项不存在: %s", itemID)
	}

	invoice.Items = newItems

	// 重新计算总额
	im.recalculateInvoiceTotals(invoice)
	invoice.UpdatedAt = time.Now()

	if err := im.saveInvoice(invoice); err != nil {
		return nil, err
	}

	return invoice, nil
}

// UpdateInvoiceItem 更新发票明细项
func (im *InvoiceManager) UpdateInvoiceItem(ctx context.Context, invoiceID string, item InvoiceManagerItem) (*InvoiceManagerInvoice, error) {
	im.mu.Lock()
	defer im.mu.Unlock()

	invoice, exists := im.invoices[invoiceID]
	if !exists {
		return nil, ErrInvoiceNotFound
	}

	if invoice.Status != InvoiceStatusDraft {
		return nil, fmt.Errorf("只有草稿状态的发票可以更新明细")
	}

	// 查找并更新明细
	found := false
	for i := range invoice.Items {
		if invoice.Items[i].ID == item.ID {
			// 计算明细金额
			item.Amount = item.Quantity * item.UnitPrice * (1 - item.Discount/100)
			if item.TaxRate == 0 {
				item.TaxRate = invoice.TaxRate
			}
			item.TaxAmount = item.Amount * item.TaxRate / 100
			item.TotalAmount = item.Amount + item.TaxAmount

			invoice.Items[i] = item
			found = true
			break
		}
	}

	if !found {
		return nil, fmt.Errorf("明细项不存在: %s", item.ID)
	}

	// 重新计算总额
	im.recalculateInvoiceTotals(invoice)
	invoice.UpdatedAt = time.Now()

	if err := im.saveInvoice(invoice); err != nil {
		return nil, err
	}

	return invoice, nil
}

// recalculateInvoiceTotals 重新计算发票总额
func (im *InvoiceManager) recalculateInvoiceTotals(invoice *InvoiceManagerInvoice) {
	var subtotal, taxAmount float64
	for _, item := range invoice.Items {
		subtotal += item.Amount
		taxAmount += item.TaxAmount
	}

	invoice.Subtotal = subtotal
	invoice.TaxAmount = taxAmount
	invoice.TotalAmount = subtotal + taxAmount
}

// SendInvoice 发送发票
func (im *InvoiceManager) SendInvoice(ctx context.Context, id string, recipient string) (*InvoiceManagerInvoice, error) {
	invoice, err := im.UpdateInvoiceStatus(ctx, id, InvoiceStatusSent)
	if err != nil {
		return nil, err
	}

	// 记录发送信息
	im.mu.Lock()
	invoice.Metadata = map[string]interface{}{
		"sent_at": time.Now(),
		"sent_to": recipient,
		"sent_by": "system",
	}
	im.mu.Unlock()

	if err := im.saveInvoice(invoice); err != nil {
		return nil, err
	}

	return invoice, nil
}

// MarkOverdue 标记发票为逾期
func (im *InvoiceManager) MarkOverdue(ctx context.Context, id string) (*InvoiceManagerInvoice, error) {
	invoice, err := im.GetInvoice(ctx, id)
	if err != nil {
		return nil, err
	}

	if invoice.Status == InvoiceStatusSent {
		return im.UpdateInvoiceStatus(ctx, id, InvoiceStatusOverdue)
	}

	return invoice, nil
}

// ApplyDiscount 应用折扣
func (im *InvoiceManager) ApplyDiscount(ctx context.Context, id string, discountPercent float64, reason string) (*InvoiceManagerInvoice, error) {
	im.mu.Lock()
	defer im.mu.Unlock()

	invoice, exists := im.invoices[id]
	if !exists {
		return nil, ErrInvoiceNotFound
	}

	if invoice.Status != InvoiceStatusDraft {
		return nil, fmt.Errorf("只有草稿状态的发票可以应用折扣")
	}

	// 应用折扣到所有明细
	for i := range invoice.Items {
		invoice.Items[i].Discount = discountPercent
		invoice.Items[i].Amount = invoice.Items[i].Quantity * invoice.Items[i].UnitPrice * (1 - discountPercent/100)
		invoice.Items[i].TaxAmount = invoice.Items[i].Amount * invoice.Items[i].TaxRate / 100
		invoice.Items[i].TotalAmount = invoice.Items[i].Amount + invoice.Items[i].TaxAmount
	}

	// 重新计算总额
	im.recalculateInvoiceTotals(invoice)
	invoice.UpdatedAt = time.Now()

	// 记录折扣原因
	if invoice.Metadata == nil {
		invoice.Metadata = make(map[string]interface{})
	}
	invoice.Metadata["discount_percent"] = discountPercent
	invoice.Metadata["discount_reason"] = reason

	if err := im.saveInvoice(invoice); err != nil {
		return nil, err
	}

	return invoice, nil
}

// GetInvoiceHistory 获取发票历史记录
func (im *InvoiceManager) GetInvoiceHistory(ctx context.Context, id string) ([]map[string]interface{}, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	invoice, exists := im.invoices[id]
	if !exists {
		return nil, ErrInvoiceNotFound
	}

	history := []map[string]interface{}{
		{
			"action":    "created",
			"timestamp": invoice.CreatedAt,
			"status":    InvoiceStatusDraft,
			"by":        "system",
		},
	}

	// 根据当前状态推断历史
	if invoice.Status != InvoiceStatusDraft {
		history = append(history, map[string]interface{}{
			"action":    "status_change",
			"timestamp": invoice.UpdatedAt,
			"from":      InvoiceStatusDraft,
			"to":        invoice.Status,
		})
	}

	if invoice.PaidDate != nil {
		history = append(history, map[string]interface{}{
			"action":    "paid",
			"timestamp": *invoice.PaidDate,
			"method":    invoice.PaymentMethod,
			"reference": invoice.PaymentRef,
		})
	}

	return history, nil
}

// ValidateInvoice 验证发票数据
func (im *InvoiceManager) ValidateInvoice(invoice *InvoiceManagerInvoice) []string {
	var errors []string

	if len(invoice.Items) == 0 {
		errors = append(errors, "发票必须包含至少一个明细项")
	}

	if invoice.TotalAmount <= 0 {
		errors = append(errors, "发票总金额必须大于0")
	}

	if invoice.Recipient.Name == "" {
		errors = append(errors, "收票方名称不能为空")
	}

	if invoice.Issuer.Name == "" {
		errors = append(errors, "开票方名称不能为空")
	}

	// 验证明细
	for i, item := range invoice.Items {
		if item.Name == "" {
			errors = append(errors, fmt.Sprintf("明细项 %d: 名称不能为空", i+1))
		}
		if item.Quantity <= 0 {
			errors = append(errors, fmt.Sprintf("明细项 %d: 数量必须大于0", i+1))
		}
		if item.UnitPrice < 0 {
			errors = append(errors, fmt.Sprintf("明细项 %d: 单价不能为负数", i+1))
		}
	}

	return errors
}
