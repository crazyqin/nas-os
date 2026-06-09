// Package ransomware 勒索软件检测模块 - HTTP 处理器
package ransomware

import (
	"encoding/json"
	"net/http"
)

// HTTPHandler HTTP 处理器
type HTTPHandler struct {
	detector *RansomwareDetector
}

// NewHTTPHandler 创建 HTTP 处理器
func NewHTTPHandler(detector *RansomwareDetector) *HTTPHandler {
	return &HTTPHandler{detector: detector}
}

// RegisterRoutes 注册路由
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/ransomware/status", h.handleStatus)
	mux.HandleFunc("/api/ransomware/start", h.handleStart)
	mux.HandleFunc("/api/ransomware/stop", h.handleStop)
	mux.HandleFunc("/api/ransomware/rules", h.handleRules)
	mux.HandleFunc("/api/ransomware/rules/add", h.handleAddRule)
	mux.HandleFunc("/api/ransomware/activities", h.handleActivities)
	mux.HandleFunc("/api/ransomware/stats", h.handleStats)
}

func (h *HTTPHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.detector.GetStats()
	json.NewEncoder(w).Encode(map[string]interface{}{
		"is_monitoring": stats["is_monitoring"],
		"status":        "active",
	})
}

func (h *HTTPHandler) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.detector.StartMonitoring()
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

func (h *HTTPHandler) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	h.detector.StopMonitoring()
	json.NewEncoder(w).Encode(map[string]string{"status": "stopped"})
}

func (h *HTTPHandler) handleRules(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.detector.ListRules())
}

func (h *HTTPHandler) handleAddRule(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var rule DetectionRule
	if err := json.NewDecoder(r.Body).Decode(&rule); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if err := h.detector.AddRule(&rule); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	json.NewEncoder(w).Encode(rule)
}

func (h *HTTPHandler) handleActivities(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.detector.GetActivities(100))
}

func (h *HTTPHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	json.NewEncoder(w).Encode(h.detector.GetStats())
}
