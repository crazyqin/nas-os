// Package billing 提供支付记录管理功能
package billing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ========== 错误定义 ==========

var (
	ErrPaymentNotFound      = errors.New("支付记录不存在")
	ErrPaymentAlreadyRefund = errors.New("支付已退款")
	ErrInvalidPaymentMethod = errors.New("无效的支付方式")
	ErrPaymentAmountInvalid = errors.New("支付金额无效")
)

// ========== 支付方式 ==========

// PaymentMethod 支付方式
type PaymentMethod string

const (
	PaymentMethodBankTransfer PaymentMethod = "bank_transfer" // 银行转账
	PaymentMethodAlipay      PaymentMethod = "alipay"        // 支付宝
	PaymentMethodWeChatPay   PaymentMethod = "wechat_pay"    // 微信支付
	PaymentMethodCreditCard  PaymentMethod = "credit_card"   // 信用卡
	PaymentMethodPayPal      PaymentMethod = "paypal"        // PayPal
	PaymentMethodCash        PaymentMethod = "cash"          // 现金
	PaymentMethodCheck       PaymentMethod = "check"         // 支票
	PaymentMethodOther       PaymentMethod = "other"         // 其他
)

// PaymentMethodInfo 支付方式信息
type PaymentMethodInfo struct {
	Method      PaymentMethod `json:"method"`
	Name        string        `json:"name"`
	IsActive    bool          `json:"is_active"`
	Icon        string        `json:"icon"`
	Description string        `json:"description"`
	FeePercent  float64       `json:"fee_percent"` // 手续费百分比
	FeeFixed    float64       `json:"fee_fixed"`   // 固定手续费
	MinAmount   float64       `json:"min_amount"`  // 最低金额
	MaxAmount   float64       `json:"max_amount"`  // 最高金额（0 表示无限制）
	Config      map[string]interface{} `json:"config,omitempty"`
}

// GetDefaultPaymentMethods 获取默认支付方式
func GetDefaultPaymentMethods() []PaymentMethodInfo {
	return []PaymentMethodInfo{
		{
			Method:      PaymentMethodBankTransfer,
			Name:        "银行转账",
			IsActive:    true,
			Icon:        "bank",
			Description: "通过银行转账付款",
			FeePercent:  0,
			FeeFixed:    0,
		},
		{
			Method:      PaymentMethodAlipay,
			Name:        "支付宝",
			IsActive:    true,
			Icon:        "alipay",
			Description: "使用支付宝扫码付款",
			FeePercent:  0.006,
			FeeFixed:    0,
			MinAmount:   0.01,
		},
		{
			Method:      PaymentMethodWeChatPay,
			Name:        "微信支付",
			IsActive:    true,
			Icon:        "wechat",
			Description: "使用微信扫码付款",
			FeePercent:  0.006,
			FeeFixed:    0,
			MinAmount:   0.01,
		},
		{
			Method:      PaymentMethodCreditCard,
			Name:        "信用卡",
			IsActive:    true,
			Icon:        "credit-card",
			Description: "使用信用卡付款",
			FeePercent:  0.028,
			FeeFixed:    0.3,
			MinAmount:   1,
		},
		{
			Method:      PaymentMethodPayPal,
			Name:        "PayPal",
			IsActive:    true,
			Icon:        "paypal",
			Description: "使用 PayPal 付款",
			FeePercent:  0.034,
			FeeFixed:    2.9,
		},
		{
			Method:      PaymentMethodCash,
			Name:        "现金",
			IsActive:    true,
			Icon:        "cash",
			Description: "现金付款",
			FeePercent:  0,
			FeeFixed:    0,
		},
		{
			Method:      PaymentMethodCheck,
			Name:        "支票",
			IsActive:    true,
			Icon:        "check",
			Description: "支票付款",
			FeePercent:  0,
			FeeFixed:    0,
		},
		{
			Method:      PaymentMethodOther,
			Name:        "其他",
			IsActive:    true,
			Icon:        "other",
			Description: "其他支付方式",
			FeePercent:  0,
			FeeFixed:    0,
		},
	}
}

// ========== 支付状态 ==========

// PaymentStatus 支付状态
type PaymentStatus string

const (
	PaymentStatusPending   PaymentStatus = "pending"   // 待支付
	PaymentStatusProcessing PaymentStatus = "processing" // 处理中
	PaymentStatusCompleted PaymentStatus = "completed" // 已完成
	PaymentStatusFailed    PaymentStatus = "failed"    // 失败
	PaymentStatusCancelled PaymentStatus = "cancelled" // 已取消
	PaymentStatusRefunded  PaymentStatus = "refunded"  // 已退款
	PaymentStatusPartial   PaymentStatus = "partial"   // 部分退款
)

// ========== 支付记录 ==========

// Payment 支付记录
type Payment struct {
	ID              string                 `json:"id"`
	PaymentNumber   string                 `json:"payment_number"`   // 支付编号
	InvoiceID       string                 `json:"invoice_id"`       // 关联账单 ID
	InvoiceNumber   string                 `json:"invoice_number"`   // 关联账单编号
	UserID          string                 `json:"user_id"`
	UserName        string                 `json:"user_name"`

	// 支付方式
	Method          PaymentMethod          `json:"method"`
	MethodName      string                 `json:"method_name"`

	// 支付金额
	Currency        string                 `json:"currency"`
	Amount          float64                `json:"amount"`           // 支付金额
	FeeAmount       float64                `json:"fee_amount"`       // 手续费
	NetAmount       float64                `json:"net_amount"`       // 实际到账
	RefundAmount    float64                `json:"refund_amount"`    // 已退款金额

	// 状态
	Status          PaymentStatus          `json:"status"`
	StatusMessage   string                 `json:"status_message,omitempty"`

	// 时间信息
	CreatedAt       time.Time              `json:"created_at"`
	UpdatedAt       time.Time              `json:"updated_at"`
	CompletedAt     *time.Time             `json:"completed_at,omitempty"`
	CancelledAt     *time.Time             `json:"cancelled_at,omitempty"`
	RefundedAt      *time.Time             `json:"refunded_at,omitempty"`

	// 支付详情
	TransactionID   string                 `json:"transaction_id,omitempty"`   // 第三方交易号
	Reference       string                 `json:"reference,omitempty"`        // 支付参考号
	Description     string                 `json:"description,omitempty"`

	// 支付渠道信息
	ChannelInfo     map[string]interface{} `json:"channel_info,omitempty"`

	// 退款信息
	RefundReason    string                 `json:"refund_reason,omitempty"`
	RefundRecords   []RefundRecord         `json:"refund_records,omitempty"`

	// 审核信息
	ApprovedBy      string                 `json:"approved_by,omitempty"`
	ApprovedAt      *time.Time             `json:"approved_at,omitempty"`

	// 备注
	Notes           string                 `json:"notes,omitempty"`

	// 元数据
	Metadata        map[string]interface{} `json:"metadata,omitempty"`
}

// RefundRecord 退款记录
type RefundRecord struct {
	ID              string       `json:"id"`
	PaymentID       string       `json:"payment_id"`
	Amount          float64      `json:"amount"`
	Reason          string       `json:"reason"`
	Status          RefundStatus `json:"status"`
	TransactionID   string       `json:"transaction_id,omitempty"`
	ProcessedAt     *time.Time   `json:"processed_at,omitempty"`
	CreatedAt       time.Time    `json:"created_at"`
}

// RefundStatus 退款状态
type RefundStatus string

const (
	RefundStatusPending   RefundStatus = "pending"
	RefundStatusProcessed RefundStatus = "processed"
	RefundStatusFailed    RefundStatus = "failed"
)

// PaymentInput 支付输入
type PaymentInput struct {
	InvoiceID     string                 `json:"invoice_id" binding:"required"`
	UserID        string                 `json:"user_id"`
	UserName      string                 `json:"user_name"`
	Method        PaymentMethod          `json:"method" binding:"required"`
	Amount        float64                `json:"amount" binding:"required"`
	Currency      string                 `json:"currency"`
	TransactionID string                 `json:"transaction_id"`
	Reference     string                 `json:"reference"`
	Description   string                 `json:"description"`
	ChannelInfo   map[string]interface{} `json:"channel_info"`
	Notes         string                 `json:"notes"`
	Metadata      map[string]interface{} `json:"metadata"`
}

// RefundInput 退款输入
type RefundInput struct {
	PaymentID   string  `json:"payment_id" binding:"required"`
	Amount      float64 `json:"amount" binding:"required"`
	Reason      string  `json:"reason" binding:"required"`
	ProcessedBy string  `json:"processed_by"`
}

// ========== 支付统计 ==========

// PaymentStats 支付统计
type PaymentStats struct {
	PeriodStart time.Time `json:"period_start"`
	PeriodEnd   time.Time `json:"period_end"`
	GeneratedAt time.Time `json:"generated_at"`

	// 总体统计
	TotalPayments   int     `json:"total_payments"`
	CompletedCount  int     `json:"completed_count"`
	PendingCount    int     `json:"pending_count"`
	FailedCount     int     `json:"failed_count"`
	CancelledCount  int     `json:"cancelled_count"`
	RefundedCount   int     `json:"refunded_count"`

	// 金额统计
	TotalAmount     float64 `json:"total_amount"`
	CompletedAmount float64 `json:"completed_amount"`
	PendingAmount   float64 `json:"pending_amount"`
	FailedAmount    float64 `json:"failed_amount"`
	RefundedAmount  float64 `json:"refunded_amount"`
	TotalFeeAmount  float64 `json:"total_fee_amount"`
	NetAmount       float64 `json:"net_amount"`

	// 按支付方式统计
	MethodStats []PaymentMethodStats `json:"method_stats"`

	// 按日期统计
	DailyStats []DailyPaymentStats `json:"daily_stats"`
}

// PaymentMethodStats 支付方式统计
type PaymentMethodStats struct {
	Method         PaymentMethod `json:"method"`
	MethodName     string        `json:"method_name"`
	Count          int           `json:"count"`
	TotalAmount    float64       `json:"total_amount"`
	FeeAmount      float64       `json:"fee_amount"`
	NetAmount      float64       `json:"net_amount"`
	AverageAmount  float64       `json:"average_amount"`
	SuccessRate    float64       `json:"success_rate"`
}

// DailyPaymentStats 每日支付统计
type DailyPaymentStats struct {
	Date          time.Time `json:"date"`
	Count         int       `json:"count"`
	TotalAmount   float64   `json:"total_amount"`
	CompletedCount int      `json:"completed_count"`
	FailedCount   int       `json:"failed_count"`
}

// ========== 支付管理器 ==========

// PaymentManager 支付管理器
type PaymentManager struct {
	billingManager  *BillingManager
	dataDir         string
	paymentMethods  map[PaymentMethod]*PaymentMethodInfo
	payments        map[string]*Payment
	paymentCounter  int
	mu              sync.RWMutex
}

// NewPaymentManager 创建支付管理器
func NewPaymentManager(billingManager *BillingManager, dataDir string) (*PaymentManager, error) {
	if dataDir == "" {
		dataDir = "./data/payments"
	}

	// 确保数据目录存在
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据目录失败: %w", err)
	}

	pm := &PaymentManager{
		billingManager: billingManager,
		dataDir:        dataDir,
		paymentMethods: make(map[PaymentMethod]*PaymentMethodInfo),
		payments:       make(map[string]*Payment),
	}

	// 初始化默认支付方式
	for _, method := range GetDefaultPaymentMethods() {
		pm.paymentMethods[method.Method] = &method
	}

	// 加载已有数据
	if err := pm.load(); err != nil {
		return nil, fmt.Errorf("加载数据失败: %w", err)
	}

	return pm, nil
}

// load 加载数据
func (pm *PaymentManager) load() error {
	// 加载支付记录
	paymentsPath := filepath.Join(pm.dataDir, "payments.json")
	if data, err := os.ReadFile(paymentsPath); err == nil {
		var payments []*Payment
		if err := json.Unmarshal(data, &payments); err != nil {
			return fmt.Errorf("解析支付记录失败: %w", err)
		}
		for _, p := range payments {
			pm.payments[p.ID] = p
			// 更新计数器
			if num := extractPaymentNumber(p.PaymentNumber); num > pm.paymentCounter {
				pm.paymentCounter = num
			}
		}
	}

	return nil
}

// save 保存数据
func (pm *PaymentManager) save() error {
	payments := make([]*Payment, 0, len(pm.payments))
	for _, p := range pm.payments {
		payments = append(payments, p)
	}

	data, err := json.MarshalIndent(payments, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化支付记录失败: %w", err)
	}

	return os.WriteFile(filepath.Join(pm.dataDir, "payments.json"), data, 0644)
}

// ========== 支付方式管理 ==========

// GetPaymentMethods 获取支付方式列表
func (pm *PaymentManager) GetPaymentMethods() []*PaymentMethodInfo {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	result := make([]*PaymentMethodInfo, 0, len(pm.paymentMethods))
	for _, method := range pm.paymentMethods {
		result = append(result, method)
	}
	return result
}

// GetPaymentMethod 获取支付方式
func (pm *PaymentManager) GetPaymentMethod(method PaymentMethod) (*PaymentMethodInfo, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	info, ok := pm.paymentMethods[method]
	if !ok {
		return nil, ErrInvalidPaymentMethod
	}
	return info, nil
}

// UpdatePaymentMethod 更新支付方式
func (pm *PaymentManager) UpdatePaymentMethod(method PaymentMethod, info *PaymentMethodInfo) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	info.Method = method
	pm.paymentMethods[method] = info
	return nil
}

// EnablePaymentMethod 启用支付方式
func (pm *PaymentManager) EnablePaymentMethod(method PaymentMethod) error {
	return pm.UpdatePaymentMethodStatus(method, true)
}

// DisablePaymentMethod 禁用支付方式
func (pm *PaymentManager) DisablePaymentMethod(method PaymentMethod) error {
	return pm.UpdatePaymentMethodStatus(method, false)
}

// UpdatePaymentMethodStatus 更新支付方式状态
func (pm *PaymentManager) UpdatePaymentMethodStatus(method PaymentMethod, active bool) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	info, ok := pm.paymentMethods[method]
	if !ok {
		return ErrInvalidPaymentMethod
	}
	info.IsActive = active
	return nil
}

// CalculateFee 计算手续费
func (pm *PaymentManager) CalculateFee(method PaymentMethod, amount float64) (float64, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	info, ok := pm.paymentMethods[method]
	if !ok {
		return 0, ErrInvalidPaymentMethod
	}

	// 检查金额限制
	if info.MinAmount > 0 && amount < info.MinAmount {
		return 0, fmt.Errorf("支付金额不能低于 %.2f", info.MinAmount)
	}
	if info.MaxAmount > 0 && amount > info.MaxAmount {
		return 0, fmt.Errorf("支付金额不能超过 %.2f", info.MaxAmount)
	}

	// 计算手续费
	fee := amount*info.FeePercent + info.FeeFixed
	return fee, nil
}

// ========== 支付记录管理 ==========

// CreatePayment 创建支付记录
func (pm *PaymentManager) CreatePayment(ctx context.Context, input *PaymentInput) (*Payment, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	// 验证支付方式
	methodInfo, ok := pm.paymentMethods[input.Method]
	if !ok || !methodInfo.IsActive {
		return nil, ErrInvalidPaymentMethod
	}

	// 验证金额
	if input.Amount <= 0 {
		return nil, ErrPaymentAmountInvalid
	}

	// 获取账单信息
	var invoice *Invoice
	var invoiceNumber string
	if input.InvoiceID != "" {
		inv, err := pm.billingManager.GetInvoice(input.InvoiceID)
		if err == nil {
			invoice = inv
			invoiceNumber = inv.InvoiceNumber
		}
	}

	// 计算手续费
	feeAmount, _ := pm.CalculateFee(input.Method, input.Amount)

	// 生成支付编号
	pm.paymentCounter++
	paymentNumber := fmt.Sprintf("PAY-%s-%04d", time.Now().Format("20060102"), pm.paymentCounter)

	// 获取用户信息
	userID := input.UserID
	userName := input.UserName
	if invoice != nil && userID == "" {
		userID = invoice.UserID
		userName = invoice.UserName
	}

	// 获取货币
	currency := input.Currency
	if currency == "" {
		currency = pm.billingManager.GetConfig().DefaultCurrency
	}

	payment := &Payment{
		ID:            generateID("pay"),
		PaymentNumber: paymentNumber,
		InvoiceID:     input.InvoiceID,
		InvoiceNumber: invoiceNumber,
		UserID:        userID,
		UserName:      userName,
		Method:        input.Method,
		MethodName:    methodInfo.Name,
		Currency:      currency,
		Amount:        input.Amount,
		FeeAmount:     feeAmount,
		NetAmount:     input.Amount - feeAmount,
		RefundAmount:  0,
		Status:        PaymentStatusPending,
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		TransactionID: input.TransactionID,
		Reference:     input.Reference,
		Description:   input.Description,
		ChannelInfo:   input.ChannelInfo,
		Notes:         input.Notes,
		Metadata:      input.Metadata,
	}

	pm.payments[payment.ID] = payment

	if err := pm.save(); err != nil {
		return nil, err
	}

	return payment, nil
}

// GetPayment 获取支付记录
func (pm *PaymentManager) GetPayment(id string) (*Payment, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	payment, ok := pm.payments[id]
	if !ok {
		return nil, ErrPaymentNotFound
	}
	return payment, nil
}

// GetPaymentByNumber 按支付编号获取
func (pm *PaymentManager) GetPaymentByNumber(number string) (*Payment, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, p := range pm.payments {
		if p.PaymentNumber == number {
			return p, nil
		}
	}
	return nil, ErrPaymentNotFound
}

// GetPaymentByTransactionID 按交易号获取
func (pm *PaymentManager) GetPaymentByTransactionID(transactionID string) (*Payment, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	for _, p := range pm.payments {
		if p.TransactionID == transactionID {
			return p, nil
		}
	}
	return nil, ErrPaymentNotFound
}

// ListPayments 列出支付记录
func (pm *PaymentManager) ListPayments(userID string, invoiceID string, status PaymentStatus, start, end time.Time) ([]*Payment, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []*Payment
	for _, p := range pm.payments {
		// 过滤条件
		if userID != "" && p.UserID != userID {
			continue
		}
		if invoiceID != "" && p.InvoiceID != invoiceID {
			continue
		}
		if status != "" && p.Status != status {
			continue
		}
		if !start.IsZero() && p.CreatedAt.Before(start) {
			continue
		}
		if !end.IsZero() && p.CreatedAt.After(end) {
			continue
		}
		result = append(result, p)
	}
	return result, nil
}

// ========== 支付状态更新 ==========

// UpdatePaymentStatus 更新支付状态
func (pm *PaymentManager) UpdatePaymentStatus(id string, status PaymentStatus, message string) (*Payment, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	payment, ok := pm.payments[id]
	if !ok {
		return nil, ErrPaymentNotFound
	}

	oldStatus := payment.Status

	// 状态转换验证
	if !isValidStatusTransition(oldStatus, status) {
		return nil, fmt.Errorf("不能从 %s 状态转换到 %s", oldStatus, status)
	}

	payment.Status = status
	payment.StatusMessage = message
	payment.UpdatedAt = time.Now()

	// 更新相关时间戳
	switch status {
	case PaymentStatusCompleted:
		now := time.Now()
		payment.CompletedAt = &now
		// 自动更新账单状态
		if payment.InvoiceID != "" {
			_, _ = pm.billingManager.MarkInvoicePaid(payment.InvoiceID, string(payment.Method), payment.PaymentNumber)
		}
	case PaymentStatusCancelled:
		now := time.Now()
		payment.CancelledAt = &now
	case PaymentStatusRefunded:
		now := time.Now()
		payment.RefundedAt = &now
	}

	if err := pm.save(); err != nil {
		return nil, err
	}

	return payment, nil
}

// MarkPaymentProcessing 标记为处理中
func (pm *PaymentManager) MarkPaymentProcessing(id string) (*Payment, error) {
	return pm.UpdatePaymentStatus(id, PaymentStatusProcessing, "")
}

// MarkPaymentCompleted 标记为已完成
func (pm *PaymentManager) MarkPaymentCompleted(id string, transactionID string) (*Payment, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	payment, ok := pm.payments[id]
	if !ok {
		return nil, ErrPaymentNotFound
	}

	if transactionID != "" {
		payment.TransactionID = transactionID
	}

	now := time.Now()
	payment.Status = PaymentStatusCompleted
	payment.CompletedAt = &now
	payment.UpdatedAt = now

	// 自动更新账单状态
	if payment.InvoiceID != "" {
		_, _ = pm.billingManager.MarkInvoicePaid(payment.InvoiceID, string(payment.Method), payment.PaymentNumber)
	}

	if err := pm.save(); err != nil {
		return nil, err
	}

	return payment, nil
}

// MarkPaymentFailed 标记为失败
func (pm *PaymentManager) MarkPaymentFailed(id string, reason string) (*Payment, error) {
	return pm.UpdatePaymentStatus(id, PaymentStatusFailed, reason)
}

// MarkPaymentCancelled 标记为取消
func (pm *PaymentManager) MarkPaymentCancelled(id string, reason string) (*Payment, error) {
	return pm.UpdatePaymentStatus(id, PaymentStatusCancelled, reason)
}

// ========== 退款管理 ==========

// ProcessRefund 处理退款
func (pm *PaymentManager) ProcessRefund(ctx context.Context, input *RefundInput) (*Payment, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	payment, ok := pm.payments[input.PaymentID]
	if !ok {
		return nil, ErrPaymentNotFound
	}

	// 检查支付状态
	if payment.Status != PaymentStatusCompleted && payment.Status != PaymentStatusPartial {
		return nil, fmt.Errorf("只能对已完成的支付进行退款")
	}

	// 检查退款金额
	availableRefund := payment.Amount - payment.RefundAmount
	if input.Amount > availableRefund {
		return nil, fmt.Errorf("退款金额不能超过 %.2f", availableRefund)
	}

	// 创建退款记录
	refundRecord := RefundRecord{
		ID:        generateID("refund"),
		PaymentID: payment.ID,
		Amount:    input.Amount,
		Reason:    input.Reason,
		Status:    RefundStatusProcessed,
		CreatedAt: time.Now(),
	}
	now := time.Now()
	refundRecord.ProcessedAt = &now

	// 更新支付记录
	payment.RefundAmount += input.Amount
	payment.RefundRecords = append(payment.RefundRecords, refundRecord)
	payment.UpdatedAt = time.Now()

	// 更新状态
	if payment.RefundAmount >= payment.Amount {
		payment.Status = PaymentStatusRefunded
		payment.RefundedAt = &now
	} else {
		payment.Status = PaymentStatusPartial
	}

	if err := pm.save(); err != nil {
		return nil, err
	}

	return payment, nil
}

// GetRefundRecords 获取退款记录
func (pm *PaymentManager) GetRefundRecords(paymentID string) ([]RefundRecord, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	payment, ok := pm.payments[paymentID]
	if !ok {
		return nil, ErrPaymentNotFound
	}

	return payment.RefundRecords, nil
}

// ========== 支付统计 ==========

// GetPaymentStats 获取支付统计
func (pm *PaymentManager) GetPaymentStats(start, end time.Time) (*PaymentStats, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stats := &PaymentStats{
		PeriodStart: start,
		PeriodEnd:   end,
		GeneratedAt: time.Now(),
	}

	// 按支付方式统计
	methodStatsMap := make(map[PaymentMethod]*PaymentMethodStats)
	for method, info := range pm.paymentMethods {
		methodStatsMap[method] = &PaymentMethodStats{
			Method:     method,
			MethodName: info.Name,
		}
	}

	// 按日期统计
	dailyStatsMap := make(map[string]*DailyPaymentStats)

	// 遍历支付记录
	for _, p := range pm.payments {
		if !start.IsZero() && p.CreatedAt.Before(start) {
			continue
		}
		if !end.IsZero() && p.CreatedAt.After(end) {
			continue
		}

		stats.TotalPayments++
		stats.TotalAmount += p.Amount
		stats.TotalFeeAmount += p.FeeAmount

		switch p.Status {
		case PaymentStatusCompleted:
			stats.CompletedCount++
			stats.CompletedAmount += p.Amount
			stats.NetAmount += p.NetAmount
		case PaymentStatusPending, PaymentStatusProcessing:
			stats.PendingCount++
			stats.PendingAmount += p.Amount
		case PaymentStatusFailed:
			stats.FailedCount++
			stats.FailedAmount += p.Amount
		case PaymentStatusCancelled:
			stats.CancelledCount++
		case PaymentStatusRefunded:
			stats.RefundedCount++
			stats.RefundedAmount += p.RefundAmount
		case PaymentStatusPartial:
			stats.CompletedCount++
			stats.CompletedAmount += p.Amount
			stats.RefundedAmount += p.RefundAmount
		}

		// 按支付方式统计
		if ms, ok := methodStatsMap[p.Method]; ok {
			ms.Count++
			ms.TotalAmount += p.Amount
			ms.FeeAmount += p.FeeAmount
			if p.Status == PaymentStatusCompleted || p.Status == PaymentStatusPartial {
				ms.NetAmount += p.NetAmount
			}
		}

		// 按日期统计
		dateKey := p.CreatedAt.Format("2006-01-02")
		if ds, ok := dailyStatsMap[dateKey]; ok {
			ds.Count++
			ds.TotalAmount += p.Amount
			if p.Status == PaymentStatusCompleted {
				ds.CompletedCount++
			} else if p.Status == PaymentStatusFailed {
				ds.FailedCount++
			}
		} else {
			ds := &DailyPaymentStats{
				Date:        p.CreatedAt.Truncate(24 * time.Hour),
				Count:       1,
				TotalAmount: p.Amount,
			}
			if p.Status == PaymentStatusCompleted {
				ds.CompletedCount = 1
			} else if p.Status == PaymentStatusFailed {
				ds.FailedCount = 1
			}
			dailyStatsMap[dateKey] = ds
		}
	}

	// 计算平均值和成功率
	for _, ms := range methodStatsMap {
		if ms.Count > 0 {
			ms.AverageAmount = ms.TotalAmount / float64(ms.Count)
		}
		completedCount := 0
		for _, p := range pm.payments {
			if p.Method == ms.Method && (p.Status == PaymentStatusCompleted || p.Status == PaymentStatusPartial) {
				completedCount++
			}
		}
		if ms.Count > 0 {
			ms.SuccessRate = float64(completedCount) / float64(ms.Count) * 100
		}
		stats.MethodStats = append(stats.MethodStats, *ms)
	}

	// 转换每日统计
	for _, ds := range dailyStatsMap {
		stats.DailyStats = append(stats.DailyStats, *ds)
	}

	return stats, nil
}

// GetUserPaymentStats 获取用户支付统计
func (pm *PaymentManager) GetUserPaymentStats(userID string, start, end time.Time) (*PaymentStats, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	stats := &PaymentStats{
		PeriodStart: start,
		PeriodEnd:   end,
		GeneratedAt: time.Now(),
	}

	for _, p := range pm.payments {
		if p.UserID != userID {
			continue
		}
		if !start.IsZero() && p.CreatedAt.Before(start) {
			continue
		}
		if !end.IsZero() && p.CreatedAt.After(end) {
			continue
		}

		stats.TotalPayments++
		stats.TotalAmount += p.Amount
		stats.TotalFeeAmount += p.FeeAmount

		switch p.Status {
		case PaymentStatusCompleted:
			stats.CompletedCount++
			stats.CompletedAmount += p.Amount
			stats.NetAmount += p.NetAmount
		case PaymentStatusPending, PaymentStatusProcessing:
			stats.PendingCount++
			stats.PendingAmount += p.Amount
		case PaymentStatusFailed:
			stats.FailedCount++
		case PaymentStatusCancelled:
			stats.CancelledCount++
		case PaymentStatusRefunded:
			stats.RefundedCount++
			stats.RefundedAmount += p.RefundAmount
		}
	}

	return stats, nil
}

// ========== 支付查询 ==========

// GetPaymentsByInvoiceID 按账单 ID 获取支付记录
func (pm *PaymentManager) GetPaymentsByInvoiceID(invoiceID string) ([]*Payment, error) {
	return pm.ListPayments("", invoiceID, "", time.Time{}, time.Time{})
}

// GetPendingPayments 获取待处理支付
func (pm *PaymentManager) GetPendingPayments() ([]*Payment, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	var result []*Payment
	for _, p := range pm.payments {
		if p.Status == PaymentStatusPending || p.Status == PaymentStatusProcessing {
			result = append(result, p)
		}
	}
	return result, nil
}

// GetOverduePayments 获取逾期支付（待支付超过指定天数）
func (pm *PaymentManager) GetOverduePayments(days int) ([]*Payment, error) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()

	cutoff := time.Now().AddDate(0, 0, -days)
	var result []*Payment

	for _, p := range pm.payments {
		if p.Status == PaymentStatusPending && p.CreatedAt.Before(cutoff) {
			result = append(result, p)
		}
	}
	return result, nil
}

// ========== 辅助函数 ==========

// isValidStatusTransition 验证状态转换是否有效
func isValidStatusTransition(from, to PaymentStatus) bool {
	validTransitions := map[PaymentStatus][]PaymentStatus{
		PaymentStatusPending: {
			PaymentStatusProcessing,
			PaymentStatusCompleted,
			PaymentStatusFailed,
			PaymentStatusCancelled,
		},
		PaymentStatusProcessing: {
			PaymentStatusCompleted,
			PaymentStatusFailed,
			PaymentStatusCancelled,
		},
		PaymentStatusCompleted: {
			PaymentStatusRefunded,
			PaymentStatusPartial,
		},
		PaymentStatusPartial: {
			PaymentStatusRefunded,
			PaymentStatusPartial,
		},
		PaymentStatusFailed:    {},
		PaymentStatusCancelled: {},
		PaymentStatusRefunded:  {},
	}

	allowed, ok := validTransitions[from]
	if !ok {
		return false
	}

	for _, s := range allowed {
		if s == to {
			return true
		}
	}
	return false
}

// extractPaymentNumber 从支付编号提取编号
func extractPaymentNumber(number string) int {
	var num int
	fmt.Sscanf(number, "%*[^0-9]%d", &num)
	return num
}