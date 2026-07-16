// Package threathunter 提供威胁猎手 HTTP API 处理器
package threathunter

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// Handlers 威胁猎手 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册 HTTP 路由.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/threathunter/scan", h.handleScan)
	mux.HandleFunc("/api/v1/threathunter/threats", h.handleThreats)
	mux.HandleFunc("/api/v1/threathunter/score", h.handleScore)
	mux.HandleFunc("/api/v1/threathunter/trends", h.handleTrends)
	mux.HandleFunc("/api/v1/threathunter/incidents", h.handleIncidents)
	mux.HandleFunc("/api/v1/threathunter/intel", h.handleIntel)
}

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

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{Code: 1, Message: msg})
}

// handleScan 处理扫描请求.
func (h *Handlers) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ScanRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	result, err := h.manager.RunScan(&req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "scan completed", Data: result})
}

// handleThreats 处理威胁请求.
func (h *Handlers) handleThreats(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		// 检查是否查询单个威胁
		id := r.URL.Query().Get("id")
		if id != "" {
			// /api/v1/threathunter/threats?id=xxx
			threat, err := h.manager.GetThreat(id)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: threat})
			return
		}

		// 支持路径解析: /api/v1/threathunter/threats/:id
		path := r.URL.Path
		parts := strings.Split(strings.TrimPrefix(path, "/api/v1/threathunter/threats/"), "/")
		if len(parts) > 0 && parts[0] != "" {
			threat, err := h.manager.GetThreat(parts[0])
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: threat})
			return
		}

		// 列出威胁，支持过滤
		level := ThreatLevel(r.URL.Query().Get("level"))
		category := ThreatCategory(r.URL.Query().Get("category"))
		status := ThreatStatus(r.URL.Query().Get("status"))
		threats := h.manager.GetThreats(level, category, status)
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: threats})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleScore 处理安全评分请求.
func (h *Handlers) handleScore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	score := h.manager.GetScore()
	writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: score})
}

// handleTrends 处理趋势请求.
func (h *Handlers) handleTrends(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	daysStr := r.URL.Query().Get("days")
	days := 7
	if daysStr != "" {
		if d, err := strconv.Atoi(daysStr); err == nil && d > 0 {
			days = d
		}
	}

	trends := h.manager.GetTrends(days)
	writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: trends})
}

// handleIncidents 处理事件请求.
func (h *Handlers) handleIncidents(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id != "" {
			inc, err := h.manager.GetIncident(id)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: inc})
			return
		}

		status := IncidentStatus(r.URL.Query().Get("status"))
		severity := IncidentSeverity(r.URL.Query().Get("severity"))
		incidents := h.manager.ListIncidents(status, severity)
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: incidents})

	case http.MethodPost:
		var req IncidentRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		if req.Title == "" {
			writeError(w, http.StatusBadRequest, "title is required")
			return
		}
		if req.Severity == "" {
			writeError(w, http.StatusBadRequest, "severity is required")
			return
		}
		inc := h.manager.CreateIncident(&req)
		writeJSON(w, http.StatusCreated, apiResponse{Code: 0, Message: "incident created", Data: inc})

	case http.MethodPut:
		// 更新事件状态
		var req struct {
			ID     string         `json:"id"`
			Status IncidentStatus `json:"status"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
			return
		}
		if err := h.manager.UpdateIncidentStatus(req.ID, req.Status); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "incident updated"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleIntel 处理威胁情报请求.
func (h *Handlers) handleIntel(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// 区分 /intel 和 /intel/feeds
	path := r.URL.Path
	if strings.HasSuffix(path, "/feeds") || strings.Contains(path, "/feeds") {
		feeds := h.manager.ListFeeds()
		writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: feeds})
		return
	}

	activeOnly := r.URL.Query().Get("active") == "true"
	intel := h.manager.ListIntel(activeOnly)
	writeJSON(w, http.StatusOK, apiResponse{Code: 0, Message: "success", Data: intel})
}
