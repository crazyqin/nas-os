package budgetmgr

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Handlers 预算管理 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建预算管理 HTTP 处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// ServeHTTP 实现 http.Handler 接口，路由分发.
func (h *Handlers) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/budgetmgr")
	path = strings.TrimPrefix(path, "/")

	switch {
	// 预算 CRUD
	case path == "budgets" && r.Method == http.MethodGet:
		h.listBudgets(w, r)
	case path == "budgets" && r.Method == http.MethodPost:
		h.createBudget(w, r)
	case path == "budgets/department" && r.Method == http.MethodGet:
		h.listBudgetsByDepartment(w, r)
	case strings.HasPrefix(path, "budgets/") && !strings.Contains(path, "/utilization") &&
		!strings.Contains(path, "/usage") && r.Method == http.MethodGet:
		h.getBudget(w, r)
	case strings.HasPrefix(path, "budgets/") && !strings.Contains(path, "/utilization") &&
		!strings.Contains(path, "/usage") && r.Method == http.MethodPut:
		h.updateBudget(w, r)
	case strings.HasPrefix(path, "budgets/") && !strings.Contains(path, "/utilization") &&
		!strings.Contains(path, "/usage") && r.Method == http.MethodDelete:
		h.deleteBudget(w, r)

	// 使用率追踪
	case strings.HasSuffix(path, "/usage") && r.Method == http.MethodPost:
		h.recordUsage(w, r)
	case strings.HasSuffix(path, "/utilization") && r.Method == http.MethodGet:
		h.getUtilization(w, r)
	case path == "utilization-reports" && r.Method == http.MethodGet:
		h.getAllUtilizationReports(w, r)

	// 审批流程
	case path == "requests" && r.Method == http.MethodGet:
		h.listRequests(w, r)
	case path == "requests" && r.Method == http.MethodPost:
		h.createRequest(w, r)
	case path == "requests/pending" && r.Method == http.MethodGet:
		h.listPendingRequests(w, r)
	case strings.HasSuffix(path, "/approve") && r.Method == http.MethodPost:
		h.approveRequest(w, r)
	case strings.HasSuffix(path, "/reject") && r.Method == http.MethodPost:
		h.rejectRequest(w, r)
	case strings.HasSuffix(path, "/allocate") && r.Method == http.MethodPost:
		h.allocateRequest(w, r)
	case strings.HasPrefix(path, "requests/") && r.Method == http.MethodGet:
		h.getRequest(w, r)

	// 超支告警
	case path == "overbudget-alerts" && r.Method == http.MethodGet:
		h.checkOverBudget(w, r)
	case path == "degradation-actions" && r.Method == http.MethodGet:
		h.getDegradationActions(w, r)

	// 对比分析
	case path == "comparison" && r.Method == http.MethodGet:
		h.compareBudgets(w, r)

	// 模板管理
	case path == "templates" && r.Method == http.MethodGet:
		h.listTemplates(w, r)
	case path == "templates" && r.Method == http.MethodPost:
		h.createTemplate(w, r)
	case strings.HasPrefix(path, "templates/") && !strings.HasSuffix(path, "/create-budget") && r.Method == http.MethodGet:
		h.getTemplate(w, r)
	case strings.HasPrefix(path, "templates/") && !strings.HasSuffix(path, "/create-budget") && r.Method == http.MethodDelete:
		h.deleteTemplate(w, r)
	case strings.HasSuffix(path, "/create-budget") && r.Method == http.MethodPost:
		h.createBudgetFromTemplate(w, r)

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
	// 去掉可能的后缀
	if idx := strings.Index(trimmed, "/"); idx >= 0 {
		return trimmed[:idx]
	}
	return trimmed
}

// ========== 预算 CRUD handlers ==========

func (h *Handlers) createBudget(w http.ResponseWriter, r *http.Request) {
	var b Budget
	if err := json.NewDecoder(r.Body).Decode(&b); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	if err := h.manager.CreateBudget(&b); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "预算创建成功",
		"budget":  b,
	})
}

func (h *Handlers) getBudget(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/budgetmgr/budgets/")
	id := strings.TrimPrefix(path, "/")
	b, err := h.manager.GetBudget(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, b)
}

func (h *Handlers) listBudgets(w http.ResponseWriter, r *http.Request) {
	budgets := h.manager.ListBudgets()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"budgets": budgets,
		"total":   len(budgets),
	})
}

func (h *Handlers) listBudgetsByDepartment(w http.ResponseWriter, r *http.Request) {
	dept := r.URL.Query().Get("department")
	if dept == "" {
		writeError(w, http.StatusBadRequest, "缺少department参数")
		return
	}
	budgets := h.manager.ListBudgetsByDepartment(dept)
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"department": dept,
		"budgets":    budgets,
		"total":      len(budgets),
	})
}

func (h *Handlers) updateBudget(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/budgetmgr/budgets/")
	id := strings.TrimPrefix(path, "/")
	var updates Budget
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	b, err := h.manager.UpdateBudget(id, &updates)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"message": "预算更新成功",
		"budget":  b,
	})
}

func (h *Handlers) deleteBudget(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/budgetmgr/budgets/")
	id := strings.TrimPrefix(path, "/")
	if err := h.manager.DeleteBudget(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "预算删除成功"})
}

// ========== 使用率 handlers ==========

func (h *Handlers) recordUsage(w http.ResponseWriter, r *http.Request) {
	// 提取 budget ID: /api/v1/budgetmgr/budgets/{id}/usage
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/budgetmgr/budgets/")
	fullPath = strings.TrimSuffix(fullPath, "/usage")
	id := strings.TrimPrefix(fullPath, "/")

	var req struct {
		Amount float64 `json:"amount"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	if err := h.manager.RecordUsage(id, req.Amount); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "使用记录成功"})
}

func (h *Handlers) getUtilization(w http.ResponseWriter, r *http.Request) {
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/budgetmgr/budgets/")
	fullPath = strings.TrimSuffix(fullPath, "/utilization")
	id := strings.TrimPrefix(fullPath, "/")

	report, err := h.manager.GetUtilizationReport(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (h *Handlers) getAllUtilizationReports(w http.ResponseWriter, r *http.Request) {
	reports := h.manager.GetAllUtilizationReports()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"reports": reports,
		"total":   len(reports),
	})
}

// ========== 审批流程 handlers ==========

func (h *Handlers) createRequest(w http.ResponseWriter, r *http.Request) {
	var req BudgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	if err := h.manager.CreateRequest(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "申请创建成功",
		"request": req,
	})
}

func (h *Handlers) getRequest(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/budgetmgr/requests/")
	id := strings.TrimPrefix(path, "/")
	req, err := h.manager.GetRequest(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, req)
}

func (h *Handlers) listRequests(w http.ResponseWriter, r *http.Request) {
	requests := h.manager.ListRequests()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"requests": requests,
		"total":    len(requests),
	})
}

func (h *Handlers) listPendingRequests(w http.ResponseWriter, r *http.Request) {
	requests := h.manager.ListPendingRequests()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"requests": requests,
		"total":    len(requests),
	})
}

func (h *Handlers) approveRequest(w http.ResponseWriter, r *http.Request) {
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/budgetmgr/requests/")
	fullPath = strings.TrimSuffix(fullPath, "/approve")
	id := strings.TrimPrefix(fullPath, "/")

	var req struct {
		Approver string `json:"approver"`
		Note     string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	if err := h.manager.ApproveRequest(id, req.Approver, req.Note); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "申请已批准"})
}

func (h *Handlers) rejectRequest(w http.ResponseWriter, r *http.Request) {
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/budgetmgr/requests/")
	fullPath = strings.TrimSuffix(fullPath, "/reject")
	id := strings.TrimPrefix(fullPath, "/")

	var req struct {
		Approver string `json:"approver"`
		Note     string `json:"note"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	if err := h.manager.RejectRequest(id, req.Approver, req.Note); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "申请已拒绝"})
}

func (h *Handlers) allocateRequest(w http.ResponseWriter, r *http.Request) {
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/budgetmgr/requests/")
	fullPath = strings.TrimSuffix(fullPath, "/allocate")
	id := strings.TrimPrefix(fullPath, "/")

	if err := h.manager.AllocateRequest(id); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "预算已分配"})
}

// ========== 超支告警 handlers ==========

func (h *Handlers) checkOverBudget(w http.ResponseWriter, r *http.Request) {
	alerts := h.manager.CheckOverBudget()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

func (h *Handlers) getDegradationActions(w http.ResponseWriter, r *http.Request) {
	actions := h.manager.GetDegradationActions()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"actions": actions,
		"total":   len(actions),
	})
}

// ========== 对比分析 handlers ==========

func (h *Handlers) compareBudgets(w http.ResponseWriter, r *http.Request) {
	dept := r.URL.Query().Get("department")
	if dept == "" {
		writeError(w, http.StatusBadRequest, "缺少department参数")
		return
	}
	comp := h.manager.CompareBudgets(dept)
	writeJSON(w, http.StatusOK, comp)
}

// ========== 模板管理 handlers ==========

func (h *Handlers) createTemplate(w http.ResponseWriter, r *http.Request) {
	var t BudgetTemplate
	if err := json.NewDecoder(r.Body).Decode(&t); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	if err := h.manager.CreateTemplate(&t); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message":  "模板创建成功",
		"template": t,
	})
}

func (h *Handlers) getTemplate(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/budgetmgr/templates/")
	id := strings.TrimPrefix(path, "/")
	// 确保不取到 create-budget 后缀
	if idx := strings.Index(id, "/"); idx >= 0 {
		id = id[:idx]
	}
	t, err := h.manager.GetTemplate(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (h *Handlers) listTemplates(w http.ResponseWriter, r *http.Request) {
	templates := h.manager.ListTemplates()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"templates": templates,
		"total":     len(templates),
	})
}

func (h *Handlers) deleteTemplate(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/budgetmgr/templates/")
	id := strings.TrimPrefix(path, "/")
	if err := h.manager.DeleteTemplate(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": "模板删除成功"})
}

func (h *Handlers) createBudgetFromTemplate(w http.ResponseWriter, r *http.Request) {
	// 提取 template ID: /api/v1/budgetmgr/templates/{id}/create-budget
	fullPath := strings.TrimPrefix(r.URL.Path, "/api/v1/budgetmgr/templates/")
	fullPath = strings.TrimSuffix(fullPath, "/create-budget")
	templateID := strings.TrimPrefix(fullPath, "/")

	var req struct {
		BudgetID   string    `json:"budget_id"`
		Name       string    `json:"name"`
		Department string    `json:"department"`
		StartDate  time.Time `json:"start_date"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	if req.StartDate.IsZero() {
		req.StartDate = time.Now()
	}

	b, err := h.manager.CreateBudgetFromTemplate(templateID, req.BudgetID, req.Name, req.Department, req.StartDate)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, map[string]interface{}{
		"message": "从模板创建预算成功",
		"budget":  b,
	})
}
