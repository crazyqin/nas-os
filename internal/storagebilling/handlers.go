package storagebilling

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Handlers 存储计费 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建存储计费 HTTP 处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// ServeHTTP 实现 http.Handler 接口，路由分发.
func (h *Handlers) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling")
	path = strings.TrimPrefix(path, "/")

	switch {
	// 费率管理
	case path == "rates" && r.Method == http.MethodGet:
		h.getTierRates(w, r)
	case strings.HasPrefix(path, "rates/") && r.Method == http.MethodPut:
		h.setTierRate(w, r)

	// 租户管理
	case path == "tenants" && r.Method == http.MethodGet:
		h.listTenants(w, r)
	case path == "tenants" && r.Method == http.MethodPost:
		h.createTenant(w, r)
	case path == "tenants/department" && r.Method == http.MethodGet:
		h.listTenantsByDepartment(w, r)
	case path == "tenants/summaries" && r.Method == http.MethodGet:
		h.getAllTenantSummaries(w, r)
	case strings.HasPrefix(path, "tenants/") && !strings.Contains(path, "/usage") &&
		!strings.Contains(path, "/quotas") && !strings.Contains(path, "/bills") &&
		!strings.Contains(path, "/optimize") && r.Method == http.MethodGet:
		h.getTenant(w, r)
	case strings.HasPrefix(path, "tenants/") && !strings.Contains(path, "/usage") &&
		!strings.Contains(path, "/quotas") && !strings.Contains(path, "/bills") &&
		!strings.Contains(path, "/optimize") && r.Method == http.MethodPut:
		h.updateTenant(w, r)
	case strings.HasPrefix(path, "tenants/") && !strings.Contains(path, "/usage") &&
		!strings.Contains(path, "/quotas") && !strings.Contains(path, "/bills") &&
		!strings.Contains(path, "/optimize") && r.Method == http.MethodDelete:
		h.deleteTenant(w, r)

	// 用量管理
	case strings.HasSuffix(path, "/usage") && !strings.HasSuffix(path, "/history") && r.Method == http.MethodPost:
		h.recordUsage(w, r)
	case strings.HasSuffix(path, "/usage/summary") && r.Method == http.MethodGet:
		h.getUsageSummary(w, r)
	case strings.HasSuffix(path, "/usage/history") && r.Method == http.MethodGet:
		h.getUsageHistory(w, r)

	// 配额管理
	case strings.HasSuffix(path, "/quotas") && r.Method == http.MethodGet:
		h.getQuotas(w, r)
	case strings.HasSuffix(path, "/quotas") && r.Method == http.MethodPost:
		h.createQuota(w, r)
	case strings.HasPrefix(path, "quotas/") && r.Method == http.MethodPut:
		h.updateQuota(w, r)
	case strings.HasPrefix(path, "quotas/") && r.Method == http.MethodDelete:
		h.deleteQuota(w, r)
	case path == "quotas/exceeded" && r.Method == http.MethodGet:
		h.checkQuotaExceeded(w, r)
	case path == "quotas/alerts" && r.Method == http.MethodGet:
		h.checkQuotaAlerts(w, r)

	// 账单管理
	case path == "bills" && r.Method == http.MethodGet:
		h.listBills(w, r)
	case path == "bills/monthly" && r.Method == http.MethodPost:
		h.generateMonthlyBills(w, r)
	case path == "bills/quarterly" && r.Method == http.MethodPost:
		h.generateQuarterlyBills(w, r)
	case strings.HasSuffix(path, "/bills") && r.Method == http.MethodGet:
		h.listTenantBills(w, r)
	case strings.HasSuffix(path, "/bills") && r.Method == http.MethodPost:
		h.generateBill(w, r)
	case strings.HasPrefix(path, "bills/") && !strings.Contains(path, "/status") && r.Method == http.MethodGet:
		h.getBill(w, r)
	case strings.HasSuffix(path, "/status") && r.Method == http.MethodPut:
		h.updateBillStatus(w, r)

	// 成本优化
	case strings.HasSuffix(path, "/optimize") && r.Method == http.MethodGet:
		h.analyzeCostOptimization(w, r)

	default:
		http.NotFound(w, r)
	}
}

// writeJSON 写入JSON响应.
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

// writeError 写入错误响应.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

// extractID 从路径中提取最后一段ID.
func extractID(path, prefix string) string {
	trimmed := strings.TrimPrefix(path, prefix)
	trimmed = strings.TrimPrefix(trimmed, "/")
	if idx := strings.Index(trimmed, "/"); idx >= 0 {
		return trimmed[:idx]
	}
	return trimmed
}

// ========== 费率管理 handlers ==========

func (h *Handlers) getTierRates(w http.ResponseWriter, r *http.Request) {
	rates := h.manager.GetTierRates()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"rates": rates,
		"total": len(rates),
	})
}

func (h *Handlers) setTierRate(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/rates/")
	tier := StorageTier(strings.TrimPrefix(path, "/"))

	var req struct {
		RatePerGB float64 `json:"rate_per_gb"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	if err := h.manager.SetTierRate(tier, req.RatePerGB); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "费率更新成功"})
}

// ========== 租户管理 handlers ==========

func (h *Handlers) createTenant(w http.ResponseWriter, r *http.Request) {
	var t Tenant
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	if err := h.manager.CreateTenant(&t); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "租户创建成功",
		"tenant":  t,
	})
}

func (h *Handlers) getTenant(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/tenants/")
	id := strings.TrimPrefix(path, "/")

	t, err := h.manager.GetTenant(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, t)
}

func (h *Handlers) listTenants(w http.ResponseWriter, r *http.Request) {
	tenants := h.manager.ListTenants()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenants": tenants,
		"total":   len(tenants),
	})
}

func (h *Handlers) listTenantsByDepartment(w http.ResponseWriter, r *http.Request) {
	dept := r.URL.Query().Get("department")
	if dept == "" {
		writeError(w, http.StatusBadRequest, "缺少department参数")
		return
	}

	tenants := h.manager.ListTenantsByDepartment(dept)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"department": dept,
		"tenants":    tenants,
		"total":      len(tenants),
	})
}

func (h *Handlers) updateTenant(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/tenants/")
	id := strings.TrimPrefix(path, "/")

	var updates Tenant
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	t, err := h.manager.UpdateTenant(id, &updates)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "租户更新成功",
		"tenant":  t,
	})
}

func (h *Handlers) deleteTenant(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/tenants/")
	id := strings.TrimPrefix(path, "/")

	if err := h.manager.DeleteTenant(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "租户删除成功"})
}

// ========== 用量管理 handlers ==========

func (h *Handlers) recordUsage(w http.ResponseWriter, r *http.Request) {
	// 提取 tenant ID: /api/v1/storagebilling/tenants/{id}/usage
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/tenants/")
	fullPath = strings.TrimSuffix(fullPath, "/usage")
	tenantID := strings.TrimPrefix(fullPath, "/")

	var req struct {
		Tier   StorageTier `json:"tier"`
		UsedGB float64     `json:"used_gb"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	if err := h.manager.RecordUsage(tenantID, req.Tier, req.UsedGB); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "用量记录成功"})
}

func (h *Handlers) getUsageSummary(w http.ResponseWriter, r *http.Request) {
	// 提取 tenant ID: /api/v1/storagebilling/tenants/{id}/usage/summary
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/tenants/")
	fullPath = strings.TrimSuffix(fullPath, "/usage/summary")
	tenantID := strings.TrimPrefix(fullPath, "/")

	summary, err := h.manager.GetUsageSummary(tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

func (h *Handlers) getUsageHistory(w http.ResponseWriter, r *http.Request) {
	// 提取 tenant ID: /api/v1/storagebilling/tenants/{id}/usage/history
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/tenants/")
	fullPath = strings.TrimSuffix(fullPath, "/usage/history")
	tenantID := strings.TrimPrefix(fullPath, "/")

	tier := StorageTier(r.URL.Query().Get("tier"))
	sinceStr := r.URL.Query().Get("since")

	var since time.Time
	if sinceStr != "" {
		var err error
		since, err = time.Parse(time.RFC3339, sinceStr)
		if err != nil {
			writeError(w, http.StatusBadRequest, "since参数格式无效，请使用RFC3339格式")
			return
		}
	} else {
		since = time.Now().AddDate(0, -1, 0) // 默认查询最近一个月
	}

	history := h.manager.GetUsageHistory(tenantID, tier, since)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_id": tenantID,
		"history":   history,
		"total":     len(history),
	})
}

// ========== 配额管理 handlers ==========

func (h *Handlers) createQuota(w http.ResponseWriter, r *http.Request) {
	// 提取 tenant ID: /api/v1/storagebilling/tenants/{id}/quotas
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/tenants/")
	fullPath = strings.TrimSuffix(fullPath, "/quotas")
	tenantID := strings.TrimPrefix(fullPath, "/")

	var q StorageQuota
	if err := json.NewDecoder(r.Body).Decode(&q); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	q.TenantID = tenantID

	if err := h.manager.CreateQuota(&q); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "配额创建成功",
		"quota":   q,
	})
}

func (h *Handlers) getQuotas(w http.ResponseWriter, r *http.Request) {
	// 提取 tenant ID: /api/v1/storagebilling/tenants/{id}/quotas
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/tenants/")
	fullPath = strings.TrimSuffix(fullPath, "/quotas")
	tenantID := strings.TrimPrefix(fullPath, "/")

	quotas, err := h.manager.GetQuotas(tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_id": tenantID,
		"quotas":    quotas,
		"total":     len(quotas),
	})
}

func (h *Handlers) updateQuota(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/quotas/")
	quotaID := strings.TrimPrefix(path, "/")

	var updates StorageQuota
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	q, err := h.manager.UpdateQuota(quotaID, &updates)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "配额更新成功",
		"quota":   q,
	})
}

func (h *Handlers) deleteQuota(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/quotas/")
	quotaID := strings.TrimPrefix(path, "/")

	if err := h.manager.DeleteQuota(quotaID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "配额删除成功"})
}

func (h *Handlers) checkQuotaExceeded(w http.ResponseWriter, r *http.Request) {
	exceeded := h.manager.CheckQuotaExceeded()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"exceeded": exceeded,
		"total":    len(exceeded),
	})
}

func (h *Handlers) checkQuotaAlerts(w http.ResponseWriter, r *http.Request) {
	alerts := h.manager.CheckQuotaAlerts()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

// ========== 账单管理 handlers ==========

func (h *Handlers) generateBill(w http.ResponseWriter, r *http.Request) {
	// 提取 tenant ID: /api/v1/storagebilling/tenants/{id}/bills
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/tenants/")
	fullPath = strings.TrimSuffix(fullPath, "/bills")
	tenantID := strings.TrimPrefix(fullPath, "/")

	var req struct {
		BillingCycle BillingCycle `json:"billing_cycle"`
		PeriodStart  time.Time    `json:"period_start"`
		PeriodEnd    time.Time    `json:"period_end"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	if req.PeriodStart.IsZero() || req.PeriodEnd.IsZero() {
		writeError(w, http.StatusBadRequest, "缺少周期开始/结束时间")
		return
	}

	bill, err := h.manager.GenerateBill(tenantID, req.BillingCycle, req.PeriodStart, req.PeriodEnd)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "账单生成成功",
		"bill":    bill,
	})
}

func (h *Handlers) getBill(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/bills/")
	billID := strings.TrimPrefix(path, "/")

	bill, err := h.manager.GetBill(billID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, bill)
}

func (h *Handlers) listBills(w http.ResponseWriter, r *http.Request) {
	tenantID := r.URL.Query().Get("tenant_id")
	bills := h.manager.ListBills(tenantID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"bills": bills,
		"total": len(bills),
	})
}

func (h *Handlers) listTenantBills(w http.ResponseWriter, r *http.Request) {
	// 提取 tenant ID: /api/v1/storagebilling/tenants/{id}/bills
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/tenants/")
	fullPath = strings.TrimSuffix(fullPath, "/bills")
	tenantID := strings.TrimPrefix(fullPath, "/")

	bills := h.manager.ListBills(tenantID)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"tenant_id": tenantID,
		"bills":     bills,
		"total":     len(bills),
	})
}

func (h *Handlers) updateBillStatus(w http.ResponseWriter, r *http.Request) {
	// 提取 bill ID: /api/v1/storagebilling/bills/{id}/status
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/bills/")
	fullPath = strings.TrimSuffix(fullPath, "/status")
	billID := strings.TrimPrefix(fullPath, "/")

	var req struct {
		Status BillStatus `json:"status"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	if err := h.manager.UpdateBillStatus(billID, req.Status); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, map[string]string{"message": "账单状态更新成功"})
}

func (h *Handlers) generateMonthlyBills(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Year  int        `json:"year"`
		Month time.Month `json:"month"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	if req.Year == 0 {
		req.Year = time.Now().Year()
	}
	if req.Month == 0 {
		req.Month = time.Now().Month()
	}

	bills := h.manager.GenerateMonthlyBills(req.Year, req.Month)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("已生成%d张月度账单", len(bills)),
		"bills":   bills,
		"total":   len(bills),
	})
}

func (h *Handlers) generateQuarterlyBills(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Year    int `json:"year"`
		Quarter int `json:"quarter"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	if req.Year == 0 {
		req.Year = time.Now().Year()
	}
	if req.Quarter == 0 {
		// 计算当前季度
		req.Quarter = int((time.Now().Month()-1)/3) + 1
	}

	bills := h.manager.GenerateQuarterlyBills(req.Year, req.Quarter)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": fmt.Sprintf("已生成%d张季度账单", len(bills)),
		"bills":   bills,
		"total":   len(bills),
	})
}

// ========== 成本优化 handlers ==========

func (h *Handlers) analyzeCostOptimization(w http.ResponseWriter, r *http.Request) {
	// 提取 tenant ID: /api/v1/storagebilling/tenants/{id}/optimize
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/storagebilling/tenants/")
	fullPath = strings.TrimSuffix(fullPath, "/optimize")
	tenantID := strings.TrimPrefix(fullPath, "/")

	optimization, err := h.manager.AnalyzeCostOptimization(tenantID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, optimization)
}

func (h *Handlers) getAllTenantSummaries(w http.ResponseWriter, r *http.Request) {
	summaries := h.manager.GetAllTenantSummaries()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"summaries": summaries,
		"total":     len(summaries),
	})
}

// ========== 辅助函数 ==========

// parseQueryParamInt 解析整数查询参数.
func parseQueryParamInt(r *http.Request, key string, defaultVal int) int {
	valStr := r.URL.Query().Get(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.Atoi(valStr)
	if err != nil {
		return defaultVal
	}
	return val
}

// parseQueryParamFloat 解析浮点数查询参数.
func parseQueryParamFloat(r *http.Request, key string, defaultVal float64) float64 {
	valStr := r.URL.Query().Get(key)
	if valStr == "" {
		return defaultVal
	}
	val, err := strconv.ParseFloat(valStr, 64)
	if err != nil {
		return defaultVal
	}
	return val
}
