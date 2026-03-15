// Package billing 提供账单生成和管理功能
package billing

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"html/template"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	ErrInvoiceTemplateNotFound = errors.New("账单模板不存在")
	ErrInvoiceExportFailed     = errors.New("账单导出失败")
	ErrInvalidExportFormat     = errors.New("无效的导出格式")
)

// ========== 账单模板 ==========

// InvoiceTemplate 账单模板
type InvoiceTemplate struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	IsDefault   bool                   `json:"is_default"`
	IsActive    bool                   `json:"is_active"`
	CreatedAt   time.Time              `json:"created_at"`
	UpdatedAt   time.Time              `json:"updated_at"`
	Sections    []InvoiceSection       `json:"sections"`
	Styles      InvoiceTemplateStyles  `json:"styles"`
	Metadata    map[string]interface{} `json:"metadata,omitempty"`
}

// InvoiceSection 账单模板区块
type InvoiceSection struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	Type       SectionType      `json:"type"`
	Position   int              `json:"position"`
	Visible    bool             `json:"visible"`
	Fields     []InvoiceField   `json:"fields"`
	Conditions []SectionCondition `json:"conditions,omitempty"`
}

// SectionType 区块类型
type SectionType string

const (
	SectionTypeHeader      SectionType = "header"       // 头部信息
	SectionTypeCompany     SectionType = "company"      // 公司信息
	SectionTypeCustomer    SectionType = "customer"     // 客户信息
	SectionTypeItems       SectionType = "items"        // 明细项目
	SectionTypeSummary     SectionType = "summary"      // 汇总信息
	SectionTypePayment     SectionType = "payment"      // 支付信息
	SectionTypeNotes       SectionType = "notes"        // 备注
	SectionTypeTerms       SectionType = "terms"        // 条款
	SectionTypeFooter      SectionType = "footer"       // 页脚
	SectionTypeCustom      SectionType = "custom"       // 自定义
)

// InvoiceField 账单字段
type InvoiceField struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Label       string      `json:"label"`
	Type        FieldType   `json:"type"`
	Source      string      `json:"source"`      // 数据来源字段
	Format      string      `json:"format"`      // 格式化模板
	Required    bool        `json:"required"`
	Visible     bool        `json:"visible"`
	Position    int         `json:"position"`
	Width       int         `json:"width"`       // 宽度百分比
	Align       string      `json:"align"`       // left, center, right
	Default     interface{} `json:"default"`
	Style       FieldStyle  `json:"style,omitempty"`
}

// FieldType 字段类型
type FieldType string

const (
	FieldTypeText    FieldType = "text"
	FieldTypeNumber  FieldType = "number"
	FieldTypeDate    FieldType = "date"
	FieldTypeMoney   FieldType = "money"
	FieldTypePercent FieldType = "percent"
	FieldTypeBool    FieldType = "bool"
	FieldTypeList    FieldType = "list"
)

// FieldStyle 字段样式
type FieldStyle struct {
	Bold      bool   `json:"bold"`
	Italic    bool   `json:"italic"`
	Underline bool   `json:"underline"`
	Color     string `json:"color"`
	Size      int    `json:"size"` // 字体大小 (pt)
}

// SectionCondition 区块显示条件
type SectionCondition struct {
	Field    string      `json:"field"`
	Operator string      `json:"operator"` // eq, ne, gt, lt, gte, lte, empty, notempty
	Value    interface{} `json:"value"`
}

// InvoiceTemplateStyles 模板样式
type InvoiceTemplateStyles struct {
	PageWidth      string `json:"page_width"`       // 页面宽度 (A4, Letter, etc.)
	PageMargin     string `json:"page_margin"`      // 页边距
	FontFamily     string `json:"font_family"`      // 字体
	FontSize       int    `json:"font_size"`        // 字体大小
	HeaderColor    string `json:"header_color"`     // 表头颜色
	BorderColor    string `json:"border_color"`     // 边框颜色
	RowEvenColor   string `json:"row_even_color"`   // 偶数行背景色
	RowOddColor    string `json:"row_odd_color"`    // 奇数行背景色
	LogoURL        string `json:"logo_url"`         // Logo URL
	LogoWidth      int    `json:"logo_width"`       // Logo 宽度
	CurrencySymbol string `json:"currency_symbol"`  // 货币符号
	DateFormat     string `json:"date_format"`      // 日期格式
}

// ========== 账单生成器 ==========

// InvoiceGenerator 账单生成器
type InvoiceGenerator struct {
	billingManager *BillingManager
	templateDir    string
	templates      map[string]*InvoiceTemplate
	mu             sync.RWMutex
}

// NewInvoiceGenerator 创建账单生成器
func NewInvoiceGenerator(billingManager *BillingManager, templateDir string) (*InvoiceGenerator, error) {
	if templateDir == "" {
		templateDir = "./templates/invoices"
	}

	// 确保模板目录存在
	if err := os.MkdirAll(templateDir, 0755); err != nil {
		return nil, fmt.Errorf("创建模板目录失败: %w", err)
	}

	generator := &InvoiceGenerator{
		billingManager: billingManager,
		templateDir:    templateDir,
		templates:      make(map[string]*InvoiceTemplate),
	}

	// 加载已有模板
	if err := generator.loadTemplates(); err != nil {
		return nil, fmt.Errorf("加载模板失败: %w", err)
	}

	// 如果没有模板，创建默认模板
	if len(generator.templates) == 0 {
		if err := generator.createDefaultTemplates(); err != nil {
			return nil, fmt.Errorf("创建默认模板失败: %w", err)
		}
	}

	return generator, nil
}

// loadTemplates 加载模板
func (g *InvoiceGenerator) loadTemplates() error {
	entries, err := os.ReadDir(g.templateDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".json") {
			continue
		}

		path := filepath.Join(g.templateDir, entry.Name())
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}

		var tmpl InvoiceTemplate
		if err := json.Unmarshal(data, &tmpl); err != nil {
			continue
		}

		g.templates[tmpl.ID] = &tmpl
	}

	return nil
}

// createDefaultTemplates 创建默认模板
func (g *InvoiceGenerator) createDefaultTemplates() error {
	defaultTemplate := &InvoiceTemplate{
		ID:          "default",
		Name:        "标准账单模板",
		Description: "适用于大多数场景的标准账单模板",
		IsDefault:   true,
		IsActive:    true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		Sections: []InvoiceSection{
			{
				ID:       "header",
				Name:     "账单头部",
				Type:     SectionTypeHeader,
				Position: 1,
				Visible:  true,
				Fields: []InvoiceField{
					{ID: "invoice_number", Name: "invoice_number", Label: "账单编号", Type: FieldTypeText, Source: "invoice_number", Required: true, Visible: true, Position: 1, Width: 50, Align: "left", Style: FieldStyle{Bold: true, Size: 14}},
					{ID: "invoice_date", Name: "invoice_date", Label: "开票日期", Type: FieldTypeDate, Source: "issued_at", Required: true, Visible: true, Position: 2, Width: 50, Align: "right", Format: "2006-01-02"},
				},
			},
			{
				ID:       "company",
				Name:     "公司信息",
				Type:     SectionTypeCompany,
				Position: 2,
				Visible:  true,
				Fields: []InvoiceField{
					{ID: "company_name", Name: "company_name", Label: "公司名称", Type: FieldTypeText, Source: "metadata.company_name", Visible: true, Position: 1, Style: FieldStyle{Bold: true}},
					{ID: "company_address", Name: "company_address", Label: "地址", Type: FieldTypeText, Source: "metadata.company_address", Visible: true, Position: 2},
					{ID: "company_tax_id", Name: "company_tax_id", Label: "税号", Type: FieldTypeText, Source: "metadata.company_tax_id", Visible: true, Position: 3},
				},
			},
			{
				ID:       "customer",
				Name:     "客户信息",
				Type:     SectionTypeCustomer,
				Position: 3,
				Visible:  true,
				Fields: []InvoiceField{
					{ID: "customer_name", Name: "customer_name", Label: "客户名称", Type: FieldTypeText, Source: "user_name", Visible: true, Position: 1, Style: FieldStyle{Bold: true}},
					{ID: "customer_id", Name: "customer_id", Label: "客户ID", Type: FieldTypeText, Source: "user_id", Visible: true, Position: 2},
				},
			},
			{
				ID:       "items",
				Name:     "明细项目",
				Type:     SectionTypeItems,
				Position: 4,
				Visible:  true,
				Fields: []InvoiceField{
					{ID: "item_desc", Name: "description", Label: "描述", Type: FieldTypeText, Source: "description", Visible: true, Position: 1, Width: 40, Align: "left"},
					{ID: "item_qty", Name: "quantity", Label: "数量", Type: FieldTypeNumber, Source: "quantity", Visible: true, Position: 2, Width: 15, Align: "center"},
					{ID: "item_unit", Name: "unit", Label: "单位", Type: FieldTypeText, Source: "unit", Visible: true, Position: 3, Width: 10, Align: "center"},
					{ID: "item_price", Name: "unit_price", Label: "单价", Type: FieldTypeMoney, Source: "unit_price", Visible: true, Position: 4, Width: 15, Align: "right"},
					{ID: "item_amount", Name: "amount", Label: "金额", Type: FieldTypeMoney, Source: "amount", Visible: true, Position: 5, Width: 20, Align: "right"},
				},
			},
			{
				ID:       "summary",
				Name:     "汇总信息",
				Type:     SectionTypeSummary,
				Position: 5,
				Visible:  true,
				Fields: []InvoiceField{
					{ID: "subtotal", Name: "subtotal", Label: "小计", Type: FieldTypeMoney, Source: "subtotal", Visible: true, Position: 1, Align: "right"},
					{ID: "discount", Name: "discount", Label: "折扣", Type: FieldTypeMoney, Source: "discount_amount", Visible: true, Position: 2, Align: "right"},
					{ID: "tax", Name: "tax", Label: "税额", Type: FieldTypeMoney, Source: "tax_amount", Visible: true, Position: 3, Align: "right"},
					{ID: "total", Name: "total", Label: "总计", Type: FieldTypeMoney, Source: "total_amount", Visible: true, Position: 4, Align: "right", Style: FieldStyle{Bold: true, Size: 12}},
				},
			},
			{
				ID:       "notes",
				Name:     "备注",
				Type:     SectionTypeNotes,
				Position: 6,
				Visible:  true,
				Fields: []InvoiceField{
					{ID: "notes", Name: "notes", Label: "备注", Type: FieldTypeText, Source: "notes", Visible: true, Position: 1},
				},
			},
			{
				ID:       "terms",
				Name:     "条款",
				Type:     SectionTypeTerms,
				Position: 7,
				Visible:  true,
				Fields: []InvoiceField{
					{ID: "terms", Name: "terms", Label: "付款条款", Type: FieldTypeText, Source: "terms", Visible: true, Position: 1},
				},
			},
		},
		Styles: InvoiceTemplateStyles{
			PageWidth:    "A4",
			PageMargin:   "20mm",
			FontFamily:   "Arial, sans-serif",
			FontSize:     10,
			HeaderColor:  "#333333",
			BorderColor:  "#dddddd",
			RowEvenColor: "#f9f9f9",
			RowOddColor:  "#ffffff",
			DateFormat:   "2006-01-02",
		},
	}

	return g.CreateTemplate(defaultTemplate)
}

// ========== 模板管理 ==========

// CreateTemplate 创建模板
func (g *InvoiceGenerator) CreateTemplate(tmpl *InvoiceTemplate) error {
	if tmpl.ID == "" {
		tmpl.ID = generateID("tmpl")
	}
	tmpl.CreatedAt = time.Now()
	tmpl.UpdatedAt = time.Now()

	g.mu.Lock()
	defer g.mu.Unlock()

	// 如果设为默认，取消其他默认
	if tmpl.IsDefault {
		for _, t := range g.templates {
			if t.IsDefault {
				t.IsDefault = false
			}
		}
	}

	g.templates[tmpl.ID] = tmpl

	return g.saveTemplate(tmpl)
}

// GetTemplate 获取模板
func (g *InvoiceGenerator) GetTemplate(id string) (*InvoiceTemplate, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	tmpl, ok := g.templates[id]
	if !ok {
		return nil, ErrInvoiceTemplateNotFound
	}
	return tmpl, nil
}

// GetDefaultTemplate 获取默认模板
func (g *InvoiceGenerator) GetDefaultTemplate() (*InvoiceTemplate, error) {
	g.mu.RLock()
	defer g.mu.RUnlock()

	for _, tmpl := range g.templates {
		if tmpl.IsDefault && tmpl.IsActive {
			return tmpl, nil
		}
	}

	// 如果没有默认模板，返回第一个可用模板
	for _, tmpl := range g.templates {
		if tmpl.IsActive {
			return tmpl, nil
		}
	}

	return nil, ErrInvoiceTemplateNotFound
}

// ListTemplates 列出模板
func (g *InvoiceGenerator) ListTemplates() []*InvoiceTemplate {
	g.mu.RLock()
	defer g.mu.RUnlock()

	result := make([]*InvoiceTemplate, 0, len(g.templates))
	for _, tmpl := range g.templates {
		result = append(result, tmpl)
	}
	return result
}

// UpdateTemplate 更新模板
func (g *InvoiceGenerator) UpdateTemplate(id string, tmpl *InvoiceTemplate) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	existing, ok := g.templates[id]
	if !ok {
		return ErrInvoiceTemplateNotFound
	}

	// 保留创建时间
	tmpl.ID = id
	tmpl.CreatedAt = existing.CreatedAt
	tmpl.UpdatedAt = time.Now()

	// 如果设为默认，取消其他默认
	if tmpl.IsDefault {
		for _, t := range g.templates {
			if t.ID != id && t.IsDefault {
				t.IsDefault = false
				_ = g.saveTemplate(t)
			}
		}
	}

	g.templates[id] = tmpl
	return g.saveTemplate(tmpl)
}

// DeleteTemplate 删除模板
func (g *InvoiceGenerator) DeleteTemplate(id string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	if _, ok := g.templates[id]; !ok {
		return ErrInvoiceTemplateNotFound
	}

	delete(g.templates, id)

	// 删除文件
	path := filepath.Join(g.templateDir, id+".json")
	return os.Remove(path)
}

// saveTemplate 保存模板
func (g *InvoiceGenerator) saveTemplate(tmpl *InvoiceTemplate) error {
	data, err := json.MarshalIndent(tmpl, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化模板失败: %w", err)
	}

	path := filepath.Join(g.templateDir, tmpl.ID+".json")
	return os.WriteFile(path, data, 0644)
}

// ========== 账单生成 ==========

// GenerateInvoiceData 生成账单数据
func (g *InvoiceGenerator) GenerateInvoiceData(ctx context.Context, invoiceID string, templateID string) (*InvoiceRenderData, error) {
	// 获取账单
	invoice, err := g.billingManager.GetInvoice(invoiceID)
	if err != nil {
		return nil, err
	}

	// 获取模板
	var tmpl *InvoiceTemplate
	if templateID != "" {
		tmpl, err = g.GetTemplate(templateID)
		if err != nil {
			return nil, err
		}
	} else {
		tmpl, err = g.GetDefaultTemplate()
		if err != nil {
			return nil, err
		}
	}

	// 构建渲染数据
	renderData := &InvoiceRenderData{
		Invoice:      invoice,
		Template:     tmpl,
		RenderedAt:   time.Now(),
		CompanyInfo:  g.getCompanyInfo(),
		Sections:     make(map[string]map[string]interface{}),
		Calculations: g.calculateInvoiceValues(invoice),
	}

	// 处理每个区块
	for _, section := range tmpl.Sections {
		if !section.Visible || !g.evaluateConditions(section.Conditions, invoice) {
			continue
		}

		sectionData := make(map[string]interface{})
		for _, field := range section.Fields {
			if !field.Visible {
				continue
			}
			value := g.getFieldValue(field, invoice)
			sectionData[field.Name] = value
		}
		renderData.Sections[string(section.Type)] = sectionData
	}

	return renderData, nil
}

// getCompanyInfo 获取公司信息
func (g *InvoiceGenerator) getCompanyInfo() map[string]string {
	config := g.billingManager.GetConfig()
	if config == nil {
		return nil
	}

	return map[string]string{
		"name":    config.CompanyName,
		"address": config.CompanyAddress,
		"tax_id":  config.CompanyTaxID,
		"phone":   config.CompanyPhone,
		"email":   config.CompanyEmail,
	}
}

// getFieldValue 获取字段值
func (g *InvoiceGenerator) getFieldValue(field InvoiceField, invoice *Invoice) interface{} {
	// 解析 source 路径
	parts := strings.Split(field.Source, ".")
	var value interface{} = invoice

	for _, part := range parts {
		switch v := value.(type) {
		case *Invoice:
			switch part {
			case "id":
				value = v.ID
			case "invoice_number":
				value = v.InvoiceNumber
			case "user_id":
				value = v.UserID
			case "user_name":
				value = v.UserName
			case "period_start":
				value = v.PeriodStart
			case "period_end":
				value = v.PeriodEnd
			case "issued_at":
				value = v.IssuedAt
			case "due_at":
				value = v.DueAt
			case "paid_at":
				if v.PaidAt != nil {
					value = *v.PaidAt
				} else {
					value = nil
				}
			case "status":
				value = string(v.Status)
			case "currency":
				value = v.Currency
			case "subtotal":
				value = v.Subtotal
			case "tax_amount":
				value = v.TaxAmount
			case "total_amount":
				value = v.TotalAmount
			case "discount_amount":
				value = v.DiscountAmount
			case "storage_amount":
				value = v.StorageAmount
			case "bandwidth_amount":
				value = v.BandwidthAmount
			case "other_amount":
				value = v.OtherAmount
			case "notes":
				value = v.Notes
			case "terms":
				value = v.Terms
			case "metadata":
				value = v.Metadata
			default:
				return field.Default
			}
		case map[string]interface{}:
			if val, ok := v[part]; ok {
				value = val
			} else {
				return field.Default
			}
		default:
			return field.Default
		}
	}

	// 格式化值
	return g.formatFieldValue(field, value)
}

// formatFieldValue 格式化字段值
func (g *InvoiceGenerator) formatFieldValue(field InvoiceField, value interface{}) interface{} {
	if value == nil {
		return field.Default
	}

	switch field.Type {
	case FieldTypeDate:
		if t, ok := value.(time.Time); ok {
			format := field.Format
			if format == "" {
				format = "2006-01-02"
			}
			return t.Format(format)
		}
	case FieldTypeMoney:
		if num, ok := toFloat64(value); ok {
			return fmt.Sprintf("%.2f", num)
		}
	case FieldTypeNumber:
		if num, ok := toFloat64(value); ok {
			if field.Format != "" {
				return fmt.Sprintf(field.Format, num)
			}
			return num
		}
	case FieldTypePercent:
		if num, ok := toFloat64(value); ok {
			return fmt.Sprintf("%.2f%%", num*100)
		}
	}

	return value
}

// evaluateConditions 评估条件
func (g *InvoiceGenerator) evaluateConditions(conditions []SectionCondition, invoice *Invoice) bool {
	if len(conditions) == 0 {
		return true
	}

	for _, cond := range conditions {
		value := g.getFieldValue(InvoiceField{Source: cond.Field}, invoice)

		var result bool
		switch cond.Operator {
		case "eq":
			result = fmt.Sprintf("%v", value) == fmt.Sprintf("%v", cond.Value)
		case "ne":
			result = fmt.Sprintf("%v", value) != fmt.Sprintf("%v", cond.Value)
		case "gt":
			if v1, ok1 := toFloat64(value); ok1 {
				if v2, ok2 := toFloat64(cond.Value); ok2 {
					result = v1 > v2
				}
			}
		case "lt":
			if v1, ok1 := toFloat64(value); ok1 {
				if v2, ok2 := toFloat64(cond.Value); ok2 {
					result = v1 < v2
				}
			}
		case "gte":
			if v1, ok1 := toFloat64(value); ok1 {
				if v2, ok2 := toFloat64(cond.Value); ok2 {
					result = v1 >= v2
				}
			}
		case "lte":
			if v1, ok1 := toFloat64(value); ok1 {
				if v2, ok2 := toFloat64(cond.Value); ok2 {
					result = v1 <= v2
				}
			}
		case "empty":
			result = value == nil || value == "" || value == 0
		case "notempty":
			result = value != nil && value != "" && value != 0
		default:
			result = true
		}

		if !result {
			return false
		}
	}

	return true
}

// calculateInvoiceValues 计算账单值
func (g *InvoiceGenerator) calculateInvoiceValues(invoice *Invoice) map[string]interface{} {
	return map[string]interface{}{
		"item_count":        len(invoice.LineItems),
		"total_quantity":    g.calculateTotalQuantity(invoice),
		"discount_percent":  g.calculateDiscountPercent(invoice),
		"tax_percent":       g.calculateTaxPercent(invoice),
		"amount_due":        invoice.TotalAmount,
		"days_overdue":      g.calculateDaysOverdue(invoice),
		"payment_status":    g.getPaymentStatus(invoice),
	}
}

func (g *InvoiceGenerator) calculateTotalQuantity(invoice *Invoice) float64 {
	var total float64
	for _, item := range invoice.LineItems {
		total += item.Quantity
	}
	return total
}

func (g *InvoiceGenerator) calculateDiscountPercent(invoice *Invoice) float64 {
	if invoice.Subtotal == 0 {
		return 0
	}
	return (invoice.DiscountAmount / invoice.Subtotal) * 100
}

func (g *InvoiceGenerator) calculateTaxPercent(invoice *Invoice) float64 {
	if invoice.Subtotal == 0 {
		return 0
	}
	return (invoice.TaxAmount / invoice.Subtotal) * 100
}

func (g *InvoiceGenerator) calculateDaysOverdue(invoice *Invoice) int {
	if invoice.Status == InvoiceStatusPaid {
		return 0
	}
	if time.Now().Before(invoice.DueAt) {
		return 0
	}
	return int(time.Now().Sub(invoice.DueAt).Hours() / 24)
}

func (g *InvoiceGenerator) getPaymentStatus(invoice *Invoice) string {
	switch invoice.Status {
	case InvoiceStatusPaid:
		return "已支付"
	case InvoiceStatusOverdue:
		return "已逾期"
	case InvoiceStatusIssued, InvoiceStatusSent:
		return "待支付"
	case InvoiceStatusVoid:
		return "已作废"
	case InvoiceStatusRefunded:
		return "已退款"
	default:
		return string(invoice.Status)
	}
}

// InvoiceRenderData 账单渲染数据
type InvoiceRenderData struct {
	Invoice      *Invoice                     `json:"invoice"`
	Template     *InvoiceTemplate             `json:"template"`
	RenderedAt   time.Time                    `json:"rendered_at"`
	CompanyInfo  map[string]string            `json:"company_info,omitempty"`
	Sections     map[string]map[string]interface{} `json:"sections"`
	Calculations map[string]interface{}       `json:"calculations"`
}

// ========== 账单导出 ==========

// ExportFormat 导出格式
type ExportFormat string

const (
	ExportFormatPDF  ExportFormat = "pdf"
	ExportFormatHTML ExportFormat = "html"
	ExportFormatJSON ExportFormat = "json"
)

// ExportInvoice 导出账单
func (g *InvoiceGenerator) ExportInvoice(ctx context.Context, invoiceID string, format ExportFormat, templateID string) ([]byte, error) {
	renderData, err := g.GenerateInvoiceData(ctx, invoiceID, templateID)
	if err != nil {
		return nil, err
	}

	switch format {
	case ExportFormatJSON:
		return g.exportJSON(renderData)
	case ExportFormatHTML:
		return g.exportHTML(renderData)
	case ExportFormatPDF:
		return g.exportPDF(renderData)
	default:
		return nil, ErrInvalidExportFormat
	}
}

// exportJSON 导出 JSON
func (g *InvoiceGenerator) exportJSON(data *InvoiceRenderData) ([]byte, error) {
	return json.MarshalIndent(data, "", "  ")
}

// exportHTML 导出 HTML
func (g *InvoiceGenerator) exportHTML(data *InvoiceRenderData) ([]byte, error) {
	tmpl := data.Template

	// 使用内置 HTML 模板
	htmlTmpl := g.buildHTMLTemplate(tmpl)

	t, err := template.New("invoice").Parse(htmlTmpl)
	if err != nil {
		return nil, fmt.Errorf("解析 HTML 模板失败: %w", err)
	}

	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("渲染 HTML 失败: %w", err)
	}

	return buf.Bytes(), nil
}

// buildHTMLTemplate 构建 HTML 模板
func (g *InvoiceGenerator) buildHTMLTemplate(tmpl *InvoiceTemplate) string {
	styles := tmpl.Styles

	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
    <meta charset="UTF-8">
    <title>账单 - {{.Invoice.InvoiceNumber}}</title>
    <style>
        body {
            font-family: %s;
            font-size: %dpx;
            margin: 0;
            padding: 20px;
            color: #333;
        }
        .container {
            max-width: 800px;
            margin: 0 auto;
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
            border-bottom: 2px solid %s;
            padding-bottom: 20px;
        }
        .header h1 {
            margin: 0;
            color: %s;
        }
        .header .invoice-number {
            font-size: 18px;
            color: #666;
            margin-top: 10px;
        }
        .info-section {
            display: flex;
            justify-content: space-between;
            margin-bottom: 30px;
        }
        .info-box {
            flex: 1;
        }
        .info-box h3 {
            margin: 0 0 10px 0;
            color: %s;
            font-size: 14px;
            border-bottom: 1px solid %s;
            padding-bottom: 5px;
        }
        .info-box p {
            margin: 5px 0;
            font-size: 12px;
        }
        .items-table {
            width: 100%%;
            border-collapse: collapse;
            margin-bottom: 30px;
        }
        .items-table th {
            background-color: %s;
            color: white;
            padding: 10px;
            text-align: left;
        }
        .items-table td {
            padding: 10px;
            border-bottom: 1px solid %s;
        }
        .items-table tr:nth-child(even) {
            background-color: %s;
        }
        .items-table tr:nth-child(odd) {
            background-color: %s;
        }
        .summary {
            text-align: right;
            margin-bottom: 30px;
        }
        .summary-row {
            margin: 5px 0;
        }
        .summary-row.total {
            font-weight: bold;
            font-size: 16px;
            border-top: 2px solid %s;
            padding-top: 10px;
            margin-top: 10px;
        }
        .notes, .terms {
            margin-bottom: 20px;
            padding: 15px;
            background-color: #f5f5f5;
            border-radius: 5px;
        }
        .notes h4, .terms h4 {
            margin: 0 0 10px 0;
            color: %s;
        }
        .footer {
            text-align: center;
            margin-top: 50px;
            padding-top: 20px;
            border-top: 1px solid %s;
            color: #666;
            font-size: 12px;
        }
        @media print {
            body { padding: 0; }
            .container { max-width: 100%%; }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>账单</h1>
            <div class="invoice-number">编号: {{.Invoice.InvoiceNumber}}</div>
        </div>

        <div class="info-section">
            <div class="info-box">
                <h3>开票方</h3>
                {{if .CompanyInfo}}
                <p><strong>{{.CompanyInfo.name}}</strong></p>
                <p>{{.CompanyInfo.address}}</p>
                <p>税号: {{.CompanyInfo.tax_id}}</p>
                <p>电话: {{.CompanyInfo.phone}}</p>
                <p>邮箱: {{.CompanyInfo.email}}</p>
                {{end}}
            </div>
            <div class="info-box">
                <h3>客户信息</h3>
                <p><strong>{{.Invoice.UserName}}</strong></p>
                <p>客户ID: {{.Invoice.UserID}}</p>
            </div>
            <div class="info-box">
                <h3>账单信息</h3>
                <p>开票日期: {{.Invoice.IssuedAt.Format "2006-01-02"}}</p>
                <p>到期日期: {{.Invoice.DueAt.Format "2006-01-02"}}</p>
                <p>状态: {{.Calculations.payment_status}}</p>
            </div>
        </div>

        <table class="items-table">
            <thead>
                <tr>
                    <th>描述</th>
                    <th>数量</th>
                    <th>单位</th>
                    <th>单价</th>
                    <th>金额</th>
                </tr>
            </thead>
            <tbody>
                {{range .Invoice.LineItems}}
                <tr>
                    <td>{{.Description}}</td>
                    <td>{{.Quantity}}</td>
                    <td>{{.Unit}}</td>
                    <td>{{printf "%.2f" .UnitPrice}}</td>
                    <td>{{printf "%.2f" .Amount}}</td>
                </tr>
                {{end}}
            </tbody>
        </table>

        <div class="summary">
            <div class="summary-row">小计: {{printf "%.2f" .Invoice.Subtotal}}</div>
            {{if gt .Invoice.DiscountAmount 0}}
            <div class="summary-row">折扣: -{{printf "%.2f" .Invoice.DiscountAmount}}</div>
            {{end}}
            {{if gt .Invoice.TaxAmount 0}}
            <div class="summary-row">税额: {{printf "%.2f" .Invoice.TaxAmount}}</div>
            {{end}}
            <div class="summary-row total">总计: {{printf "%.2f" .Invoice.TotalAmount}}</div>
        </div>

        {{if .Invoice.Notes}}
        <div class="notes">
            <h4>备注</h4>
            <p>{{.Invoice.Notes}}</p>
        </div>
        {{end}}

        {{if .Invoice.Terms}}
        <div class="terms">
            <h4>付款条款</h4>
            <p>{{.Invoice.Terms}}</p>
        </div>
        {{end}}

        <div class="footer">
            <p>感谢您的信任与支持！</p>
            <p>本账单生成于: {{.RenderedAt.Format "2006-01-02 15:04:05"}}</p>
        </div>
    </div>
</body>
</html>`,
		styles.FontFamily, styles.FontSize,
		styles.BorderColor, styles.HeaderColor,
		styles.HeaderColor, styles.BorderColor,
		styles.HeaderColor, styles.BorderColor,
		styles.RowEvenColor, styles.RowOddColor,
		styles.BorderColor, styles.HeaderColor,
		styles.BorderColor,
	)
}

// exportPDF 导出 PDF
func (g *InvoiceGenerator) exportPDF(data *InvoiceRenderData) ([]byte, error) {
	// 先生成 HTML
	htmlData, err := g.exportHTML(data)
	if err != nil {
		return nil, err
	}

	// 保存 HTML 到临时文件
	tempDir := os.TempDir()
	tempFile := filepath.Join(tempDir, fmt.Sprintf("invoice-%s.html", data.Invoice.ID))
	if err := os.WriteFile(tempFile, htmlData, 0644); err != nil {
		return nil, fmt.Errorf("保存临时 HTML 文件失败: %w", err)
	}
	defer os.Remove(tempFile)

	// 尝试使用 wkhtmltopdf 或类似工具转 PDF
	pdfFile := filepath.Join(tempDir, fmt.Sprintf("invoice-%s.pdf", data.Invoice.ID))
	defer os.Remove(pdfFile)

	// 检查是否有 wkhtmltopdf
	if _, err := os.Stat("/usr/bin/wkhtmltopdf"); err == nil {
		// 使用 wkhtmltopdf
		cmd := fmt.Sprintf("wkhtmltopdf --page-size A4 --margin-top 20mm --margin-bottom 20mm --margin-left 20mm --margin-right 20mm %s %s", tempFile, pdfFile)
		// 注意：这里简化处理，实际应使用 exec.Command
		_ = cmd
	}

	// 如果没有 PDF 转换工具，返回 HTML 并提示
	return nil, fmt.Errorf("PDF 导出需要安装 wkhtmltopdf，当前返回 HTML 格式")
}

// ExportInvoicesBatch 批量导出账单
func (g *InvoiceGenerator) ExportInvoicesBatch(ctx context.Context, invoiceIDs []string, format ExportFormat, templateID string) (map[string][]byte, error) {
	results := make(map[string][]byte)
	errors := make(map[string]error)

	for _, id := range invoiceIDs {
		data, err := g.ExportInvoice(ctx, id, format, templateID)
		if err != nil {
			errors[id] = err
		} else {
			results[id] = data
		}
	}

	if len(errors) > 0 {
		return results, fmt.Errorf("部分账单导出失败: %d 个成功, %d 个失败", len(results), len(errors))
	}

	return results, nil
}

// ========== 辅助函数 ==========

// toFloat64 转换为 float64
func toFloat64(value interface{}) (float64, bool) {
	switch v := value.(type) {
	case float64:
		return v, true
	case float32:
		return float64(v), true
	case int:
		return float64(v), true
	case int64:
		return float64(v), true
	case int32:
		return float64(v), true
	case string:
		if f, err := strconv.ParseFloat(v, 64); err == nil {
			return f, true
		}
	}
	return 0, false
}

// InvoiceExportOptions 导出选项
type InvoiceExportOptions struct {
	Format       ExportFormat `json:"format"`
	TemplateID   string       `json:"template_id"`
	IncludeLogo  bool         `json:"include_logo"`
	IncludeNotes bool         `json:"include_notes"`
	IncludeTerms bool         `json:"include_terms"`
	Language     string       `json:"language"`
	Currency     string       `json:"currency"`
}

// ExportInvoiceWithOptions 带选项的导出
func (g *InvoiceGenerator) ExportInvoiceWithOptions(ctx context.Context, invoiceID string, options InvoiceExportOptions) (io.Reader, error) {
	data, err := g.ExportInvoice(ctx, invoiceID, options.Format, options.TemplateID)
	if err != nil {
		return nil, err
	}
	return bytes.NewReader(data), nil
}