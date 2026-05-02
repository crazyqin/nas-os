package licensescan

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handlers 许可证合规扫描 HTTP 处理器.
type Handlers struct {
	manager    *Manager
	scheduler  *Scheduler
	reportGen  *ReportGenerator
}

// NewHandlers 创建HTTP处理器.
func NewHandlers(manager *Manager, scheduler *Scheduler) *Handlers {
	return &Handlers{
		manager:   manager,
		scheduler: scheduler,
		reportGen: NewReportGenerator(),
	}
}

// RegisterRoutes 注册路由到http.ServeMux.
// API路由前缀: /api/v1/licensescan/
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	// 扫描管理
	mux.HandleFunc("/api/v1/licensescan/scan/docker", h.handleDockerScan)
	mux.HandleFunc("/api/v1/licensescan/scan/gomod", h.handleGoModScan)
	mux.HandleFunc("/api/v1/licensescan/scan/results", h.handleListScans)
	mux.HandleFunc("/api/v1/licensescan/scan/result/", h.handleGetScanResult)

	// 策略管理
	mux.HandleFunc("/api/v1/licensescan/policies", h.handlePolicies)
	mux.HandleFunc("/api/v1/licensescan/policy/", h.handlePolicy)

	// 报告管理
	mux.HandleFunc("/api/v1/licensescan/reports", h.handleListReports)
	mux.HandleFunc("/api/v1/licensescan/report/generate", h.handleGenerateReport)
	mux.HandleFunc("/api/v1/licensescan/report/", h.handleGetReport)

	// 仪表盘
	mux.HandleFunc("/api/v1/licensescan/dashboard", h.handleDashboard)

	// 告警
	mux.HandleFunc("/api/v1/licensescan/alerts", h.handleAlerts)

	// 调度器
	mux.HandleFunc("/api/v1/licensescan/scheduler/tasks", h.handleSchedulerTasks)
	mux.HandleFunc("/api/v1/licensescan/scheduler/task/", h.handleSchedulerTask)
}

// ========== 扫描Handler ==========

// handleDockerScan 处理Docker镜像扫描请求.
// POST /api/v1/licensescan/scan/docker
func (h *Handlers) handleDockerScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持POST方法")
		return
	}

	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	if req.Target == "" {
		writeError(w, http.StatusBadRequest, "扫描目标不能为空")
		return
	}

	result, err := h.manager.RunDockerScan(req.Target, req.PolicyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "扫描失败: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleGoModScan 处理Go模块扫描请求.
// POST /api/v1/licensescan/scan/gomod
func (h *Handlers) handleGoModScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持POST方法")
		return
	}

	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}
	if req.Target == "" {
		writeError(w, http.StatusBadRequest, "扫描目标不能为空")
		return
	}

	result, err := h.manager.RunGoModScan(req.Target, req.PolicyID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "扫描失败: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// handleListScans 列出所有扫描结果.
// GET /api/v1/licensescan/scan/results
func (h *Handlers) handleListScans(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持GET方法")
		return
	}

	scans := h.manager.ListScans()
	writeJSON(w, http.StatusOK, ScanListResponse{
		Scans: scans,
		Total: len(scans),
	})
}

// handleGetScanResult 获取单个扫描结果.
// GET /api/v1/licensescan/scan/result/{id}
func (h *Handlers) handleGetScanResult(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持GET方法")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/licensescan/scan/result/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "扫描ID不能为空")
		return
	}

	result, err := h.manager.GetScanResult(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// ========== 策略Handler ==========

// handlePolicies 处理策略列表和创建.
// GET /api/v1/licensescan/policies - 列出策略
// POST /api/v1/licensescan/policies - 创建策略
func (h *Handlers) handlePolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		policies := h.manager.ListPolicies()
		writeJSON(w, http.StatusOK, PolicyListResponse{
			Policies: policies,
			Total:    len(policies),
		})
	case http.MethodPost:
		var policy Policy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
			return
		}
		if err := h.manager.CreatePolicy(&policy); err != nil {
			writeError(w, http.StatusConflict, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, policy)
	default:
		writeError(w, http.StatusMethodNotAllowed, "仅支持GET/POST方法")
	}
}

// handlePolicy 处理单个策略的查询、更新、删除.
// GET /api/v1/licensescan/policy/{id}
// PUT /api/v1/licensescan/policy/{id}
// DELETE /api/v1/licensescan/policy/{id}
func (h *Handlers) handlePolicy(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/v1/licensescan/policy/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "策略ID不能为空")
		return
	}

	switch r.Method {
	case http.MethodGet:
		policy, err := h.manager.GetPolicy(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, policy)
	case http.MethodPut:
		var policy Policy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
			return
		}
		policy.ID = id
		if err := h.manager.UpdatePolicy(&policy); err != nil {
			writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, policy)
	case http.MethodDelete:
		if err := h.manager.DeletePolicy(id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"message": "策略已删除"})
	default:
		writeError(w, http.StatusMethodNotAllowed, "仅支持GET/PUT/DELETE方法")
	}
}

// ========== 报告Handler ==========

// handleListReports 列出所有报告.
// GET /api/v1/licensescan/reports
func (h *Handlers) handleListReports(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持GET方法")
		return
	}

	reports := h.manager.ListReports()
	writeJSON(w, http.StatusOK, ReportListResponse{
		Reports: reports,
		Total:   len(reports),
	})
}

// handleGenerateReport 生成扫描报告.
// POST /api/v1/licensescan/report/generate
func (h *Handlers) handleGenerateReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持POST方法")
		return
	}

	var req struct {
		Title    string   `json:"title"`
		Format   string   `json:"format"`
		ScanIDs  []string `json:"scan_ids"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
		return
	}

	if req.Title == "" {
		req.Title = "许可证合规扫描报告"
	}
	if req.Format == "" {
		req.Format = "json"
	}

	format := ReportFormat(req.Format)
	report, err := h.manager.GenerateReport(req.Title, format, req.ScanIDs)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if format == FormatHTML {
		htmlBytes, err := h.reportGen.GenerateHTML(report)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "生成HTML报告失败: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(htmlBytes)
		return
	}

	writeJSON(w, http.StatusCreated, report)
}

// handleGetReport 获取报告.
// GET /api/v1/licensescan/report/{id}
func (h *Handlers) handleGetReport(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持GET方法")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/licensescan/report/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "报告ID不能为空")
		return
	}

	report, err := h.manager.GetReport(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": err.Error()})
		return
	}

	// 根据Accept头决定返回格式
	accept := r.Header.Get("Accept")
	if strings.Contains(accept, "text/html") {
		htmlBytes, err := h.reportGen.GenerateHTML(report)
		if err != nil {
			writeError(w, http.StatusInternalServerError, "生成HTML报告失败: "+err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		w.Write(htmlBytes)
		return
	}

	writeJSON(w, http.StatusOK, report)
}

// ========== 仪表盘Handler ==========

// handleDashboard 获取合规仪表盘数据.
// GET /api/v1/licensescan/dashboard
func (h *Handlers) handleDashboard(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持GET方法")
		return
	}

	data := h.manager.GetDashboardData()
	writeJSON(w, http.StatusOK, data)
}

// ========== 告警Handler ==========

// handleAlerts 获取告警列表.
// GET /api/v1/licensescan/alerts
func (h *Handlers) handleAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持GET方法")
		return
	}

	alerts := h.manager.GetAlerts()
	writeJSON(w, http.StatusOK, AlertListResponse{
		Alerts: alerts,
		Total:  len(alerts),
	})
}

// ========== 调度器Handler ==========

// handleSchedulerTasks 处理调度器任务列表和创建.
// GET /api/v1/licensescan/scheduler/tasks
// POST /api/v1/licensescan/scheduler/tasks
func (h *Handlers) handleSchedulerTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tasks := h.scheduler.ListTasks()
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"tasks": tasks,
			"total": len(tasks),
		})
	case http.MethodPost:
		var task ScheduledTask
		if err := json.NewDecoder(r.Body).Decode(&task); err != nil {
			writeError(w, http.StatusBadRequest, "请求参数无效: "+err.Error())
			return
		}
		h.scheduler.AddTask(task)
		writeJSON(w, http.StatusCreated, task)
	default:
		writeError(w, http.StatusMethodNotAllowed, "仅支持GET/POST方法")
	}
}

// handleSchedulerTask 处理单个调度器任务的删除.
// DELETE /api/v1/licensescan/scheduler/task/{id}
func (h *Handlers) handleSchedulerTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "仅支持DELETE方法")
		return
	}

	id := strings.TrimPrefix(r.URL.Path, "/api/v1/licensescan/scheduler/task/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "任务ID不能为空")
		return
	}

	if h.scheduler.RemoveTask(id) {
		writeJSON(w, http.StatusOK, map[string]string{"message": "任务已删除"})
	} else {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "任务不存在"})
	}
}

// ========== 工具函数 ==========

// writeJSON 写入JSON响应.
func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应.
func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, map[string]string{"error": message})
}
