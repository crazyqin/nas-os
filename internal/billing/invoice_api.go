// Package billing 提供账单管理 RESTful API
package billing

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// InvoiceAPIHandler 账单 API 处理程序
type InvoiceAPIHandler struct {
	billingManager   *BillingManager
	invoiceGenerator *InvoiceGenerator
	paymentManager   *PaymentManager
}

// NewInvoiceAPIHandler 创建账单 API 处理程序
func NewInvoiceAPIHandler(billingManager *BillingManager, invoiceGenerator *InvoiceGenerator, paymentManager *PaymentManager) *InvoiceAPIHandler {
	return &InvoiceAPIHandler{
		billingManager:   billingManager,
		invoiceGenerator: invoiceGenerator,
		paymentManager:   paymentManager,
	}
}

// RegisterRoutes 注册路由
func (h *InvoiceAPIHandler) RegisterRoutes(mux *http.ServeMux) {
	// 账单管理
	mux.HandleFunc("/api/billing/invoices", h.HandleInvoices)
	mux.HandleFunc("/api/billing/invoices/", h.HandleInvoiceByID)
	mux.HandleFunc("/api/billing/invoices/generate", h.HandleGenerateInvoice)
	mux.HandleFunc("/api/billing/invoices/export", h.HandleExportInvoices)

	// 发票操作
	mux.HandleFunc("/api/billing/invoices/issue", h.HandleIssueInvoice)
	mux.HandleFunc("/api/billing/invoices/pay", h.HandlePayInvoice)
	mux.HandleFunc("/api/billing/invoices/void", h.HandleVoidInvoice)

	// 账单导出
	mux.HandleFunc("/api/billing/invoices/export/", h.HandleExportInvoiceByID)

	// 账单模板
	mux.HandleFunc("/api/billing/templates", h.HandleTemplates)
	mux.HandleFunc("/api/billing/templates/", h.HandleTemplateByID)

	// 支付管理
	mux.HandleFunc("/api/billing/payments", h.HandlePayments)
	mux.HandleFunc("/api/billing/payments/", h.HandlePaymentByID)
	mux.HandleFunc("/api/billing/payments/refund", h.HandleRefundPayment)
	mux.HandleFunc("/api/billing/payments/methods", h.HandlePaymentMethods)

	// 统计查询
	mux.HandleFunc("/api/billing/stats", h.HandleBillingStats)
	mux.HandleFunc("/api/billing/stats/invoice", h.HandleInvoiceStats)
	mux.HandleFunc("/api/billing/stats/payment", h.HandlePaymentStats)
	mux.HandleFunc("/api/billing/stats/user/", h.HandleUserBillingStats)
	mux.HandleFunc("/api/billing/summary", h.HandleInvoiceSummary)

	// 用量记录
	mux.HandleFunc("/api/billing/usage", h.HandleUsageRecords)
	mux.HandleFunc("/api/billing/usage/", h.HandleUsageRecordByID)
	mux.HandleFunc("/api/billing/usage/summary", h.HandleUsageSummary)

	// 批量操作
	mux.HandleFunc("/api/billing/batch/generate", h.HandleBatchGenerateInvoices)
	mux.HandleFunc("/api/billing/batch/export", h.HandleBatchExportInvoices)
}

// ========== 账单 CRUD ==========

// HandleInvoices 处理账单列表请求
func (h *InvoiceAPIHandler) HandleInvoices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listInvoices(w, r)
	case http.MethodPost:
		h.createInvoice(w, r)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// HandleInvoiceByID 处理单个账单请求
func (h *InvoiceAPIHandler) HandleInvoiceByID(w http.ResponseWriter, r *http.Request) {
	// 提取账单 ID
	id := h.extractID(r.URL.Path, "/api/billing/invoices/")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "Invoice ID is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getInvoice(w, r, id)
	case http.MethodPut:
		h.updateInvoice(w, r, id)
	case http.MethodDelete:
		h.deleteInvoice(w, r, id)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// listInvoices 列出账单
func (h *InvoiceAPIHandler) listInvoices(w http.ResponseWriter, r *http.Request) {
	// 解析查询参数
	userID := r.URL.Query().Get("user_id")
	statusStr := r.URL.Query().Get("status")
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var status InvoiceStatus
	if statusStr != "" {
		status = InvoiceStatus(statusStr)
	}

	var start, end time.Time
	if startStr != "" {
		start, _ = time.Parse(time.RFC3339, startStr)
	}
	if endStr != "" {
		end, _ = time.Parse(time.RFC3339, endStr)
	}

	// 获取账单列表
	invoices, err := h.billingManager.ListInvoices(userID, status, start, end)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, invoices)
}

// createInvoice 创建账单
func (h *InvoiceAPIHandler) createInvoice(w http.ResponseWriter, r *http.Request) {
	var input InvoiceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	invoice, err := h.billingManager.CreateInvoice(r.Context(), &input)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, invoice)
}

// getInvoice 获取账单
func (h *InvoiceAPIHandler) getInvoice(w http.ResponseWriter, r *http.Request, id string) {
	invoice, err := h.billingManager.GetInvoice(id)
	if err != nil {
		if err == ErrInvoiceNotFound {
			h.writeError(w, http.StatusNotFound, err.Error())
		} else {
			h.writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	h.writeJSON(w, http.StatusOK, invoice)
}

// updateInvoice 更新账单
func (h *InvoiceAPIHandler) updateInvoice(w http.ResponseWriter, r *http.Request, id string) {
	var input InvoiceInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 获取现有账单
	invoice, err := h.billingManager.GetInvoice(id)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// 只允许更新草稿状态的账单
	if invoice.Status != InvoiceStatusDraft {
		h.writeError(w, http.StatusBadRequest, "只能更新草稿状态的账单")
		return
	}

	// 更新字段
	invoice.UserName = input.UserName
	invoice.DiscountAmount = input.DiscountAmount
	invoice.DiscountReason = input.DiscountReason
	invoice.Notes = input.Notes
	invoice.Terms = input.Terms
	invoice.Metadata = input.Metadata

	// 重新计算明细
	if len(input.LineItems) > 0 {
		// 重新处理明细项...
	}

	h.writeJSON(w, http.StatusOK, invoice)
}

// deleteInvoice 删除账单
func (h *InvoiceAPIHandler) deleteInvoice(w http.ResponseWriter, r *http.Request, id string) {
	// 获取账单
	invoice, err := h.billingManager.GetInvoice(id)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	// 只允许删除草稿状态的账单
	if invoice.Status != InvoiceStatusDraft {
		h.writeError(w, http.StatusBadRequest, "只能删除草稿状态的账单")
		return
	}

	// 标记为作废
	_, err = h.billingManager.VoidInvoice(id)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ========== 账单生成 ==========

// HandleGenerateInvoice 处理账单生成请求
func (h *InvoiceAPIHandler) HandleGenerateInvoice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		UserID      string    `json:"user_id"`
		Start       time.Time `json:"start"`
		End         time.Time `json:"end"`
		AutoIssue   bool      `json:"auto_issue"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 根据用量生成账单
	invoice, err := h.billingManager.GenerateInvoiceFromUsage(r.Context(), req.UserID, req.Start, req.End)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 自动开具
	if req.AutoIssue {
		_, err = h.billingManager.IssueInvoice(invoice.ID)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	h.writeJSON(w, http.StatusCreated, invoice)
}

// ========== 账单操作 ==========

// HandleIssueInvoice 开具账单
func (h *InvoiceAPIHandler) HandleIssueInvoice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		InvoiceID string `json:"invoice_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	invoice, err := h.billingManager.IssueInvoice(req.InvoiceID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, invoice)
}

// HandlePayInvoice 标记账单已支付
func (h *InvoiceAPIHandler) HandlePayInvoice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		InvoiceID      string `json:"invoice_id"`
		PaymentMethod  string `json:"payment_method"`
		PaymentRef     string `json:"payment_ref"`
		Amount         float64 `json:"amount"`
		CreatePayment  bool   `json:"create_payment"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 如果需要创建支付记录
	if req.CreatePayment {
		// 创建支付记录
		_, err := h.paymentManager.CreatePayment(r.Context(), &PaymentInput{
			InvoiceID: req.InvoiceID,
			Method:    PaymentMethod(req.PaymentMethod),
			Amount:    req.Amount,
		})
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// 标记账单已支付
	invoice, err := h.billingManager.MarkInvoicePaid(req.InvoiceID, req.PaymentMethod, req.PaymentRef)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, invoice)
}

// HandleVoidInvoice 作废账单
func (h *InvoiceAPIHandler) HandleVoidInvoice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		InvoiceID string `json:"invoice_id"`
		Reason    string `json:"reason"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	invoice, err := h.billingManager.VoidInvoice(req.InvoiceID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, invoice)
}

// ========== 账单导出 ==========

// HandleExportInvoices 批量导出账单
func (h *InvoiceAPIHandler) HandleExportInvoices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		InvoiceIDs []string     `json:"invoice_ids"`
		Format     ExportFormat `json:"format"`
		TemplateID string       `json:"template_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	results, err := h.invoiceGenerator.ExportInvoicesBatch(r.Context(), req.InvoiceIDs, req.Format, req.TemplateID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, results)
}

// HandleExportInvoiceByID 导出单个账单
func (h *InvoiceAPIHandler) HandleExportInvoiceByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 提取账单 ID
	id := h.extractID(r.URL.Path, "/api/billing/invoices/export/")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "Invoice ID is required")
		return
	}

	// 解析参数
	format := ExportFormat(r.URL.Query().Get("format"))
	if format == "" {
		format = ExportFormatPDF
	}
	templateID := r.URL.Query().Get("template_id")

	data, err := h.invoiceGenerator.ExportInvoice(r.Context(), id, format, templateID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 设置响应头
	switch format {
	case ExportFormatPDF:
		w.Header().Set("Content-Type", "application/pdf")
		w.Header().Set("Content-Disposition", "attachment; filename=invoice-"+id+".pdf")
	case ExportFormatHTML:
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
	case ExportFormatJSON:
		w.Header().Set("Content-Type", "application/json")
	}

	w.Write(data)
}

// ========== 模板管理 ==========

// HandleTemplates 处理模板列表请求
func (h *InvoiceAPIHandler) HandleTemplates(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		templates := h.invoiceGenerator.ListTemplates()
		h.writeJSON(w, http.StatusOK, templates)

	case http.MethodPost:
		var tmpl InvoiceTemplate
		if err := json.NewDecoder(r.Body).Decode(&tmpl); err != nil {
			h.writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if err := h.invoiceGenerator.CreateTemplate(&tmpl); err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		h.writeJSON(w, http.StatusCreated, tmpl)

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// HandleTemplateByID 处理单个模板请求
func (h *InvoiceAPIHandler) HandleTemplateByID(w http.ResponseWriter, r *http.Request) {
	// 提取模板 ID
	id := h.extractID(r.URL.Path, "/api/billing/templates/")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "Template ID is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		tmpl, err := h.invoiceGenerator.GetTemplate(id)
		if err != nil {
			h.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, tmpl)

	case http.MethodPut:
		var tmpl InvoiceTemplate
		if err := json.NewDecoder(r.Body).Decode(&tmpl); err != nil {
			h.writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if err := h.invoiceGenerator.UpdateTemplate(id, &tmpl); err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		h.writeJSON(w, http.StatusOK, tmpl)

	case http.MethodDelete:
		if err := h.invoiceGenerator.DeleteTemplate(id); err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		w.WriteHeader(http.StatusNoContent)

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ========== 支付管理 ==========

// HandlePayments 处理支付列表请求
func (h *InvoiceAPIHandler) HandlePayments(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listPayments(w, r)
	case http.MethodPost:
		h.createPayment(w, r)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// HandlePaymentByID 处理单个支付请求
func (h *InvoiceAPIHandler) HandlePaymentByID(w http.ResponseWriter, r *http.Request) {
	// 提取支付 ID
	id := h.extractID(r.URL.Path, "/api/billing/payments/")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "Payment ID is required")
		return
	}

	switch r.Method {
	case http.MethodGet:
		payment, err := h.paymentManager.GetPayment(id)
		if err != nil {
			h.writeError(w, http.StatusNotFound, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, payment)

	case http.MethodPut:
		var req struct {
			Status  PaymentStatus `json:"status"`
			Message string        `json:"message"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		payment, err := h.paymentManager.UpdatePaymentStatus(id, req.Status, req.Message)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		h.writeJSON(w, http.StatusOK, payment)

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// listPayments 列出支付记录
func (h *InvoiceAPIHandler) listPayments(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	invoiceID := r.URL.Query().Get("invoice_id")
	statusStr := r.URL.Query().Get("status")
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")

	var status PaymentStatus
	if statusStr != "" {
		status = PaymentStatus(statusStr)
	}

	var start, end time.Time
	if startStr != "" {
		start, _ = time.Parse(time.RFC3339, startStr)
	}
	if endStr != "" {
		end, _ = time.Parse(time.RFC3339, endStr)
	}

	payments, err := h.paymentManager.ListPayments(userID, invoiceID, status, start, end)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, payments)
}

// createPayment 创建支付记录
func (h *InvoiceAPIHandler) createPayment(w http.ResponseWriter, r *http.Request) {
	var input PaymentInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	payment, err := h.paymentManager.CreatePayment(r.Context(), &input)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, payment)
}

// HandleRefundPayment 处理退款请求
func (h *InvoiceAPIHandler) HandleRefundPayment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var input RefundInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	payment, err := h.paymentManager.ProcessRefund(r.Context(), &input)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, payment)
}

// HandlePaymentMethods 处理支付方式请求
func (h *InvoiceAPIHandler) HandlePaymentMethods(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		methods := h.paymentManager.GetPaymentMethods()
		h.writeJSON(w, http.StatusOK, methods)

	case http.MethodPut:
		var req struct {
			Method PaymentMethod    `json:"method"`
			Info   PaymentMethodInfo `json:"info"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.writeError(w, http.StatusBadRequest, "Invalid request body")
			return
		}

		if err := h.paymentManager.UpdatePaymentMethod(req.Method, &req.Info); err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}

		h.writeJSON(w, http.StatusOK, req.Info)

	default:
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// ========== 统计查询 ==========

// HandleBillingStats 处理计费统计请求
func (h *InvoiceAPIHandler) HandleBillingStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	start, end := h.parseDateRange(r)

	stats, err := h.billingManager.GetBillingStats(start, end)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, stats)
}

// HandleInvoiceStats 处理发票统计请求
func (h *InvoiceAPIHandler) HandleInvoiceStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := r.URL.Query().Get("user_id")
	start, end := h.parseDateRange(r)

	if userID != "" {
		stats, err := h.billingManager.GetUserBillingStats(userID, start, end)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, stats)
		return
	}

	stats, err := h.billingManager.GetBillingStats(start, end)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, stats)
}

// HandlePaymentStats 处理支付统计请求
func (h *InvoiceAPIHandler) HandlePaymentStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := r.URL.Query().Get("user_id")
	start, end := h.parseDateRange(r)

	if userID != "" {
		stats, err := h.paymentManager.GetUserPaymentStats(userID, start, end)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, stats)
		return
	}

	stats, err := h.paymentManager.GetPaymentStats(start, end)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, stats)
}

// HandleUserBillingStats 处理用户计费统计请求
func (h *InvoiceAPIHandler) HandleUserBillingStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// 提取用户 ID
	userID := h.extractID(r.URL.Path, "/api/billing/stats/user/")
	if userID == "" {
		h.writeError(w, http.StatusBadRequest, "User ID is required")
		return
	}

	start, end := h.parseDateRange(r)

	// 获取发票统计
	invoiceStats, err := h.billingManager.GetUserBillingStats(userID, start, end)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 获取支付统计
	paymentStats, err := h.paymentManager.GetUserPaymentStats(userID, start, end)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// 获取用量汇总
	usageSummary, err := h.billingManager.GetUserUsageSummary(userID, start, end)
	if err != nil {
		usageSummary = nil
	}

	result := map[string]interface{}{
		"user_id":        userID,
		"invoice_stats":  invoiceStats,
		"payment_stats":  paymentStats,
		"usage_summary":  usageSummary,
	}

	h.writeJSON(w, http.StatusOK, result)
}

// HandleInvoiceSummary 处理发票汇总请求
func (h *InvoiceAPIHandler) HandleInvoiceSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := r.URL.Query().Get("user_id")
	start, end := h.parseDateRange(r)

	summary, err := h.billingManager.GetInvoiceSummary(userID, start, end)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, summary)
}

// ========== 用量记录 ==========

// HandleUsageRecords 处理用量记录请求
func (h *InvoiceAPIHandler) HandleUsageRecords(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listUsageRecords(w, r)
	case http.MethodPost:
		h.recordUsage(w, r)
	default:
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
	}
}

// HandleUsageRecordByID 处理单个用量记录请求
func (h *InvoiceAPIHandler) HandleUsageRecordByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	id := h.extractID(r.URL.Path, "/api/billing/usage/")
	if id == "" {
		h.writeError(w, http.StatusBadRequest, "Usage record ID is required")
		return
	}

	record, err := h.billingManager.GetUsageRecord(id)
	if err != nil {
		h.writeError(w, http.StatusNotFound, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, record)
}

// HandleUsageSummary 处理用量汇总请求
func (h *InvoiceAPIHandler) HandleUsageSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	userID := r.URL.Query().Get("user_id")
	poolID := r.URL.Query().Get("pool_id")
	start, end := h.parseDateRange(r)

	if userID != "" {
		summary, err := h.billingManager.GetUserUsageSummary(userID, start, end)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, summary)
		return
	}

	if poolID != "" {
		summary, err := h.billingManager.GetPoolUsageSummary(poolID, start, end)
		if err != nil {
			h.writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.writeJSON(w, http.StatusOK, summary)
		return
	}

	h.writeError(w, http.StatusBadRequest, "user_id or pool_id is required")
}

// listUsageRecords 列出用量记录
func (h *InvoiceAPIHandler) listUsageRecords(w http.ResponseWriter, r *http.Request) {
	userID := r.URL.Query().Get("user_id")
	poolID := r.URL.Query().Get("pool_id")
	start, end := h.parseDateRange(r)

	records, err := h.billingManager.ListUsageRecords(userID, poolID, start, end)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, records)
}

// recordUsage 记录用量
func (h *InvoiceAPIHandler) recordUsage(w http.ResponseWriter, r *http.Request) {
	var input UsageRecordInput
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	record, err := h.billingManager.RecordUsage(r.Context(), &input)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusCreated, record)
}

// ========== 批量操作 ==========

// HandleBatchGenerateInvoices 批量生成账单
func (h *InvoiceAPIHandler) HandleBatchGenerateInvoices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		UserIDs    []string  `json:"user_ids"`
		Start      time.Time `json:"start"`
		End        time.Time `json:"end"`
		AutoIssue  bool      `json:"auto_issue"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	results := make([]map[string]interface{}, 0)
	errors := make([]map[string]string, 0)

	for _, userID := range req.UserIDs {
		invoice, err := h.billingManager.GenerateInvoiceFromUsage(r.Context(), userID, req.Start, req.End)
		if err != nil {
			errors = append(errors, map[string]string{
				"user_id": userID,
				"error":   err.Error(),
			})
			continue
		}

		if req.AutoIssue {
			_, err = h.billingManager.IssueInvoice(invoice.ID)
			if err != nil {
				errors = append(errors, map[string]string{
					"user_id":    userID,
					"invoice_id": invoice.ID,
					"error":      err.Error(),
				})
				continue
			}
		}

		results = append(results, map[string]interface{}{
			"user_id":    userID,
			"invoice_id": invoice.ID,
			"invoice":    invoice,
		})
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"success_count": len(results),
		"error_count":   len(errors),
		"results":       results,
		"errors":        errors,
	})
}

// HandleBatchExportInvoices 批量导出账单
func (h *InvoiceAPIHandler) HandleBatchExportInvoices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		h.writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req struct {
		UserIDs    []string     `json:"user_ids"`
		Status     InvoiceStatus `json:"status"`
		Start      time.Time    `json:"start"`
		End        time.Time    `json:"end"`
		Format     ExportFormat `json:"format"`
		TemplateID string       `json:"template_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	// 获取账单列表
	var invoiceIDs []string
	if len(req.UserIDs) > 0 {
		for _, userID := range req.UserIDs {
			invoices, _ := h.billingManager.ListInvoices(userID, req.Status, req.Start, req.End)
			for _, inv := range invoices {
				invoiceIDs = append(invoiceIDs, inv.ID)
			}
		}
	} else {
		invoices, _ := h.billingManager.ListInvoices("", req.Status, req.Start, req.End)
		for _, inv := range invoices {
			invoiceIDs = append(invoiceIDs, inv.ID)
		}
	}

	if len(invoiceIDs) == 0 {
		h.writeError(w, http.StatusBadRequest, "No invoices found")
		return
	}

	results, err := h.invoiceGenerator.ExportInvoicesBatch(r.Context(), invoiceIDs, req.Format, req.TemplateID)
	if err != nil {
		h.writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.writeJSON(w, http.StatusOK, map[string]interface{}{
		"total_count":   len(invoiceIDs),
		"success_count": len(results),
		"results":       results,
	})
}

// ========== 辅助方法 ==========

// extractID 从 URL 路径提取 ID
func (h *InvoiceAPIHandler) extractID(path, prefix string) string {
	id := strings.TrimPrefix(path, prefix)
	// 处理可能的后续路径
	if idx := strings.Index(id, "/"); idx > 0 {
		id = id[:idx]
	}
	return id
}

// parseDateRange 解析日期范围
func (h *InvoiceAPIHandler) parseDateRange(r *http.Request) (start, end time.Time) {
	startStr := r.URL.Query().Get("start")
	endStr := r.URL.Query().Get("end")
	daysStr := r.URL.Query().Get("days")

	if startStr != "" {
		start, _ = time.Parse(time.RFC3339, startStr)
	}
	if endStr != "" {
		end, _ = time.Parse(time.RFC3339, endStr)
	}

	// 如果指定了天数，计算日期范围
	if daysStr != "" && start.IsZero() && end.IsZero() {
		if days, err := strconv.Atoi(daysStr); err == nil {
			end = time.Now()
			start = end.AddDate(0, 0, -days)
		}
	}

	// 默认最近 30 天
	if start.IsZero() && end.IsZero() {
		end = time.Now()
		start = end.AddDate(0, 0, -30)
	}

	return start, end
}

// writeJSON 写入 JSON 响应
func (h *InvoiceAPIHandler) writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应
func (h *InvoiceAPIHandler) writeError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}