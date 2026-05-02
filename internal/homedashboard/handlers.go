// Package homedashboard 提供 HTTP API 处理器
package homedashboard

import (
	"encoding/json"
	"fmt"
	"net/http"
)

// Handlers 仪表盘 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/v1/homedashboard.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/homedashboard/dashboards", h.handleDashboards)
	mux.HandleFunc("/api/v1/homedashboard/dashboards/", h.handleDashboardByID)
	mux.HandleFunc("/api/v1/homedashboard/templates", h.handleTemplates)
	mux.HandleFunc("/api/v1/homedashboard/templates/", h.handleTemplateByID)
	mux.HandleFunc("/api/v1/homedashboard/subscribe/", h.handleSubscribe)
}

// apiResponse 标准 API 响应.
type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// handleDashboards 处理 /api/v1/homedashboard/dashboards.
func (h *Handlers) handleDashboards(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		userID := r.URL.Query().Get("user_id")
		dashboards := h.manager.ListDashboards(userID)
		writeJSON(w, http.StatusOK, apiResponse{
			Code:    0,
			Message: "success",
			Data: map[string]interface{}{
				"total":      len(dashboards),
				"dashboards": dashboards,
			},
		})
	case http.MethodPost:
		var req CreateDashboardRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
			return
		}
		if req.UserID == "" {
			writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: "user_id is required"})
			return
		}
		if req.Name == "" {
			req.Name = "我的仪表盘"
		}
		d := h.manager.CreateDashboard(req)
		writeJSON(w, http.StatusCreated, apiResponse{Code: 0, Message: "created", Data: d})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleDashboardByID 处理 /api/v1/homedashboard/dashboards/{id} 及子路径.
func (h *Handlers) handleDashboardByID(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/api/v1/homedashboard/dashboards/"):]
	if path == "" {
		http.NotFound(w, r)
		return
	}

	// 解析 id/layouts/{layoutId}/widgets/{widgetId} 等路径
	segments := splitPath(path)
	id := segments[0]

	if len(segments) == 1 {
		// /dashboards/{id}
		h.handleDashboard(w, r, id)
		return
	}

	if segments[1] == "layouts" {
		if len(segments) == 2 {
			// /dashboards/{id}/layouts
			h.handleLayouts(w, r, id)
			return
		}
		layoutID := segments[2]
		if len(segments) == 3 {
			// /dashboards/{id}/layouts/{layoutId}
			h.handleLayout(w, r, id, layoutID)
			return
		}
		if segments[3] == "widgets" {
			if len(segments) == 4 {
				// /dashboards/{id}/layouts/{layoutId}/widgets
				h.handleWidgets(w, r, id, layoutID)
				return
			}
			widgetID := segments[4]
			// /dashboards/{id}/layouts/{layoutId}/widgets/{widgetId}
			h.handleWidget(w, r, id, layoutID, widgetID)
			return
		}
	}

	http.NotFound(w, r)
}

func (h *Handlers) handleDashboard(w http.ResponseWriter, r *http.Request, id string) {
	switch r.Method {
	case http.MethodGet:
		d, err := h.manager.GetDashboard(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: d})
	case http.MethodPut:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
			return
		}
		d, err := h.manager.UpdateDashboard(id, req.Name)
		if err != nil {
			writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "updated", Data: d})
	case http.MethodDelete:
		if err := h.manager.DeleteDashboard(id); err != nil {
			writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "deleted"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) handleLayouts(w http.ResponseWriter, r *http.Request, dashboardID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateLayoutRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	layout, err := h.manager.AddLayout(dashboardID, req)
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, apiResponse{Code: 0, Message: "created", Data: layout})
}

func (h *Handlers) handleLayout(w http.ResponseWriter, r *http.Request, dashboardID, layoutID string) {
	switch r.Method {
	case http.MethodDelete:
		if err := h.manager.DeleteLayout(dashboardID, layoutID); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "deleted"})
	case http.MethodPut:
		var req struct {
			IsActive bool `json:"is_active"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		if req.IsActive {
			if err := h.manager.SetActiveLayout(dashboardID, layoutID); err != nil {
				writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
				return
			}
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "updated"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) handleWidgets(w http.ResponseWriter, r *http.Request, dashboardID, layoutID string) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AddWidgetRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	widget, err := h.manager.AddWidget(dashboardID, layoutID, req)
	if err != nil {
		writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusCreated, apiResponse{Code: 0, Message: "created", Data: widget})
}

func (h *Handlers) handleWidget(w http.ResponseWriter, r *http.Request, dashboardID, layoutID, widgetID string) {
	switch r.Method {
	case http.MethodGet:
		widget, err := h.manager.GetWidget(dashboardID, layoutID, widgetID)
		if err != nil {
			writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: widget})
	case http.MethodPut:
		var req UpdateWidgetRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request: " + err.Error()})
			return
		}
		widget, err := h.manager.UpdateWidget(dashboardID, layoutID, widgetID, req)
		if err != nil {
			writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "updated", Data: widget})
	case http.MethodDelete:
		if err := h.manager.DeleteWidget(dashboardID, layoutID, widgetID); err != nil {
			writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "deleted"})
	default:
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleTemplates 处理 /api/v1/homedashboard/templates.
func (h *Handlers) handleTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	widgetType := WidgetType(r.URL.Query().Get("type"))
	templates := h.manager.ListTemplates(widgetType)
	writeJSON(w, http.StatusOK, apiResponse{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"total":     len(templates),
			"templates": templates,
		},
	})
}

// handleTemplateByID 处理 /api/v1/homedashboard/templates/{id} 及子路径.
func (h *Handlers) handleTemplateByID(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Path[len("/api/v1/homedashboard/templates/"):]
	if path == "" {
		http.NotFound(w, r)
		return
	}

	segments := splitPath(path)
	id := segments[0]

	if len(segments) == 1 {
		// GET /templates/{id}
		if r.Method != http.MethodGet {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		t, err := h.manager.GetTemplate(id)
		if err != nil {
			writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: t})
		return
	}

	if segments[1] == "download" {
		// POST /templates/{id}/download
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if err := h.manager.DownloadTemplate(id); err != nil {
			writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "downloaded"})
		return
	}

	if segments[1] == "rate" {
		// POST /templates/{id}/rate
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req struct {
			Rating float64 `json:"rating"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: "invalid request"})
			return
		}
		if err := h.manager.RateTemplate(id, req.Rating); err != nil {
			writeJSON(w, http.StatusBadRequest, apiResponse{Code: 1, Message: err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "rated"})
		return
	}

	http.NotFound(w, r)
}

// handleSubscribe 处理 WebSocket 订阅.
func (h *Handlers) handleSubscribe(w http.ResponseWriter, r *http.Request) {
	dashboardID := r.URL.Path[len("/api/v1/homedashboard/subscribe/"):]
	if dashboardID == "" {
		http.NotFound(w, r)
		return
	}

	// 验证 dashboard 存在
	if _, err := h.manager.GetDashboard(dashboardID); err != nil {
		writeJSON(w, http.StatusNotFound, apiResponse{Code: 1, Message: err.Error()})
		return
	}

	// 简化的 SSE 方式提供实时更新（WebSocket 升级由反向代理处理）
	flusher, ok := w.(http.Flusher)
	if !ok {
		writeJSON(w, http.StatusInternalServerError, apiResponse{Code: 1, Message: "streaming not supported"})
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	ch := h.manager.Subscribe(dashboardID)
	defer h.manager.Unsubscribe(dashboardID, ch)

	for {
		select {
		case msg, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(msg)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

// splitPath 分割 URL 路径.
func splitPath(path string) []string {
	var segments []string
	start := 0
	for i := 0; i <= len(path); i++ {
		if i == len(path) || path[i] == '/' {
			if i > start {
				segments = append(segments, path[start:i])
			}
			start = i + 1
		}
	}
	return segments
}
