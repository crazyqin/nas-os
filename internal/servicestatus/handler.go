// Package servicestatus HTTP API handler
// 使用标准库 net/http 提供服务仪表盘 REST API
package servicestatus

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// ============================================================
// Handler 服务仪表盘 HTTP 处理器
// ============================================================

// Handler 服务仪表盘 HTTP 处理器.
type Handler struct {
	dashboard *ServiceDashboard
}

// NewHandler 创建处理器.
func NewHandler(dashboard *ServiceDashboard) *Handler {
	return &Handler{dashboard: dashboard}
}

// RegisterRoutes 注册路由到给定的 ServeMux.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/dashboard/services", h.handleServices)
	mux.HandleFunc("/api/dashboard/services/", h.handleServiceByID)
	mux.HandleFunc("/api/dashboard/topology", h.handleTopology)
	mux.HandleFunc("/api/dashboard/overview", h.handleOverview)
	mux.HandleFunc("/api/dashboard/groups/", h.handleGroupAction)
}

// apiResponse 标准 API 响应.
type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// writeJSON 输出 JSON 响应.
func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// writeError 输出错误响应.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{Code: 1, Message: msg})
}

// writeSuccess 输出成功响应.
func writeSuccess(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: data})
}

// ============================================================
// GET /api/dashboard/services - 服务列表
// POST /api/dashboard/services - 注册服务
// ============================================================

func (h *Handler) handleServices(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listServices(w, r)
	case http.MethodPost:
		h.registerService(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// listServices GET /api/dashboard/services?status=&type=&group=.
func (h *Handler) listServices(w http.ResponseWriter, r *http.Request) {
	status := ServiceStatus(r.URL.Query().Get("status"))
	svcType := ServiceType(r.URL.Query().Get("type"))
	group := r.URL.Query().Get("group")

	services := h.dashboard.ListServices(status, svcType, group)
	writeSuccess(w, services)
}

// registerService POST /api/dashboard/services.
func (h *Handler) registerService(w http.ResponseWriter, r *http.Request) {
	var svc Service
	if err := json.NewDecoder(r.Body).Decode(&svc); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if err := h.dashboard.RegisterService(&svc); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, apiResponse{Code: 0, Message: "service registered", Data: svc})
}

// ============================================================
// /api/dashboard/services/:id/* - 服务详情 / 操作
// ============================================================

func (h *Handler) handleServiceByID(w http.ResponseWriter, r *http.Request) {
	// 解析路径: /api/dashboard/services/{id} 或 /api/dashboard/services/{id}/{action}
	path := strings.TrimPrefix(r.URL.Path, "/api/dashboard/services/")
	parts := strings.SplitN(path, "/", 2)
	svcID := ServiceID(parts[0])

	if svcID == "" {
		writeError(w, http.StatusBadRequest, "缺少服务 ID")
		return
	}

	action := ""
	if len(parts) > 1 {
		action = parts[1]
	}

	switch action {
	case "":
		h.handleServiceDetail(w, r, svcID)
	case "start":
		h.handleStartService(w, r, svcID)
	case "stop":
		h.handleStopService(w, r, svcID)
	case "restart":
		h.handleRestartService(w, r, svcID)
	case "health":
		h.handleServiceHealth(w, r, svcID)
	case "metrics":
		h.handleServiceMetrics(w, r, svcID)
	default:
		writeError(w, http.StatusNotFound, "未知路径")
	}
}

// handleServiceDetail GET /api/dashboard/services/:id.
func (h *Handler) handleServiceDetail(w http.ResponseWriter, r *http.Request, svcID ServiceID) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	svc, err := h.dashboard.GetService(svcID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeSuccess(w, svc)
}

// handleStartService POST /api/dashboard/services/:id/start.
func (h *Handler) handleStartService(w http.ResponseWriter, r *http.Request, svcID ServiceID) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := h.dashboard.StartService(svcID); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeSuccess(w, map[string]string{"message": fmt.Sprintf("服务 %s 启动中", svcID)})
}

// handleStopService POST /api/dashboard/services/:id/stop.
func (h *Handler) handleStopService(w http.ResponseWriter, r *http.Request, svcID ServiceID) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := h.dashboard.StopService(svcID); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeSuccess(w, map[string]string{"message": fmt.Sprintf("服务 %s 已停止", svcID)})
}

// handleRestartService POST /api/dashboard/services/:id/restart.
func (h *Handler) handleRestartService(w http.ResponseWriter, r *http.Request, svcID ServiceID) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := h.dashboard.RestartService(svcID); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeSuccess(w, map[string]string{"message": fmt.Sprintf("服务 %s 重启中", svcID)})
}

// handleServiceHealth GET /api/dashboard/services/:id/health.
func (h *Handler) handleServiceHealth(w http.ResponseWriter, r *http.Request, svcID ServiceID) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	hc, err := h.dashboard.RunHealthCheck(context.Background(), svcID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeSuccess(w, hc)
}

// handleServiceMetrics GET /api/dashboard/services/:id/metrics.
func (h *Handler) handleServiceMetrics(w http.ResponseWriter, r *http.Request, svcID ServiceID) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	metrics, err := h.dashboard.GetMetrics(svcID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeSuccess(w, metrics)
}

// ============================================================
// GET /api/dashboard/topology - 服务依赖拓扑
// ============================================================

func (h *Handler) handleTopology(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 支持 ?sort=topological 查询参数
	if r.URL.Query().Get("sort") == "topological" {
		sorted, err := h.dashboard.TopologicalSort()
		if err != nil {
			writeError(w, http.StatusUnprocessableEntity, err.Error())
			return
		}
		writeSuccess(w, map[string]interface{}{
			"topology": h.dashboard.GetTopology(),
			"sorted":   sorted,
		})
		return
	}

	writeSuccess(w, h.dashboard.GetTopology())
}

// ============================================================
// GET /api/dashboard/overview - 系统总览
// ============================================================

func (h *Handler) handleOverview(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	writeSuccess(w, h.dashboard.GetOverview())
}

// ============================================================
// POST /api/dashboard/groups/:group/action - 批量操作服务组
// ============================================================

func (h *Handler) handleGroupAction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 解析路径: /api/dashboard/groups/{group}/action
	path := strings.TrimPrefix(r.URL.Path, "/api/dashboard/groups/")
	parts := strings.SplitN(path, "/", 2)
	if len(parts) < 2 || parts[1] != "action" {
		writeError(w, http.StatusBadRequest, "路径格式: /api/dashboard/groups/{group}/action")
		return
	}

	group := parts[0]
	if group == "" {
		writeError(w, http.StatusBadRequest, "缺少组名")
		return
	}

	var req struct {
		Action GroupAction `json:"action"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if req.Action == "" {
		writeError(w, http.StatusBadRequest, "缺少 action 字段（start/stop/restart）")
		return
	}

	results := h.dashboard.ExecuteGroupAction(group, req.Action)
	writeSuccess(w, results)
}
