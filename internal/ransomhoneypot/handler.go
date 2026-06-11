// Package ransomhoneypot - HTTP API 处理器
// 提供蜜罐系统的 REST API 接口
package ransomhoneypot

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// ============================================================
// HTTP 处理器
// ============================================================

// HTTPHandler 蜜罐系统 HTTP API 处理器
type HTTPHandler struct {
	manager *HoneypotManager
}

// NewHTTPHandler 创建 HTTP 处理器
func NewHTTPHandler(manager *HoneypotManager) *HTTPHandler {
	return &HTTPHandler{manager: manager}
}

// RegisterRoutes 注册 API 路由
func (h *HTTPHandler) RegisterRoutes(mux *http.ServeMux) {
	// 系统状态
	mux.HandleFunc("/api/ransomhoneypot/status", h.handleStatus)
	mux.HandleFunc("/api/ransomhoneypot/stats", h.handleStats)

	// 监控控制
	mux.HandleFunc("/api/ransomhoneypot/monitoring/start", h.handleStartMonitoring)
	mux.HandleFunc("/api/ransomhoneypot/monitoring/stop", h.handleStopMonitoring)

	// 诱饵文件管理
	mux.HandleFunc("/api/ransomhoneypot/decoys", h.handleListDecoys)
	mux.HandleFunc("/api/ransomhoneypot/decoys/deploy", h.handleDeployDecoys)
	mux.HandleFunc("/api/ransomhoneypot/decoys/remove", h.handleRemoveDecoy)
	mux.HandleFunc("/api/ransomhoneypot/decoys/check", h.handleCheckDecoys)

	// 监控目标管理
	mux.HandleFunc("/api/ransomhoneypot/targets", h.handleListTargets)
	mux.HandleFunc("/api/ransomhoneypot/targets/remove", h.handleRemoveTarget)

	// 威胁检测
	mux.HandleFunc("/api/ransomhoneypot/detections", h.handleGetDetections)
	mux.HandleFunc("/api/ransomhoneypot/events", h.handleGetEvents)

	// 配置
	mux.HandleFunc("/api/ransomhoneypot/config", h.handleConfig)

	// 威胁模式管理
	mux.HandleFunc("/api/ransomhoneypot/patterns", h.handlePatterns)
}

// ============================================================
// 系统状态 API
// ============================================================

// handleStatus GET /api/ransomhoneypot/status
func (h *HTTPHandler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, map[string]interface{}{
		"enabled":       h.manager.config.Enabled,
		"monitoring":    h.manager.IsMonitoring(),
		"uptime_seconds": h.manager.GetStats().UptimeSeconds,
	})
}

// handleStats GET /api/ransomhoneypot/stats
func (h *HTTPHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, h.manager.GetStats())
}

// ============================================================
// 监控控制 API
// ============================================================

// handleStartMonitoring POST /api/ransomhoneypot/monitoring/start
func (h *HTTPHandler) handleStartMonitoring(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.manager.StartMonitoring(); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	writeJSON(w, map[string]string{"status": "started"})
}

// handleStopMonitoring POST /api/ransomhoneypot/monitoring/stop
func (h *HTTPHandler) handleStopMonitoring(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := h.manager.StopMonitoring(); err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	writeJSON(w, map[string]string{"status": "stopped"})
}

// ============================================================
// 诱饵文件管理 API
// ============================================================

// handleListDecoys GET /api/ransomhoneypot/decoys
func (h *HTTPHandler) handleListDecoys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, h.manager.ListDecoys())
}

// handleDeployDecoys POST /api/ransomhoneypot/decoys/deploy
// Body: MonitorTarget JSON
func (h *HTTPHandler) handleDeployDecoys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var target MonitorTarget
	if err := json.NewDecoder(r.Body).Decode(&target); err != nil {
		http.Error(w, "无效请求体: "+err.Error(), http.StatusBadRequest)
		return
	}

	if target.Path == "" {
		http.Error(w, "路径不能为空", http.StatusBadRequest)
		return
	}

	decoys, err := h.manager.DeployDecoys(&target)
	if err != nil {
		http.Error(w, "部署失败: "+err.Error(), http.StatusInternalServerError)
		return
	}

	writeJSON(w, map[string]interface{}{
		"status":      "deployed",
		"target_id":   target.ID,
		"decoy_count": len(decoys),
		"decoys":      decoys,
	})
}

// handleRemoveDecoy POST /api/ransomhoneypot/decoys/remove
// Body: {"id": "decoy-id"}
func (h *HTTPHandler) handleRemoveDecoy(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效请求体", http.StatusBadRequest)
		return
	}

	if err := h.manager.RemoveDecoy(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]string{"status": "removed"})
}

// handleCheckDecoys GET /api/ransomhoneypot/decoys/check?dir=/path
func (h *HTTPHandler) handleCheckDecoys(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	dir := r.URL.Query().Get("dir")
	corrupted := h.manager.CheckDecoyIntegrity(dir)

	writeJSON(w, map[string]interface{}{
		"corrupted_count": len(corrupted),
		"corrupted_files": corrupted,
	})
}

// ============================================================
// 监控目标管理 API
// ============================================================

// handleListTargets GET /api/ransomhoneypot/targets
func (h *HTTPHandler) handleListTargets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, h.manager.ListTargets())
}

// handleRemoveTarget POST /api/ransomhoneypot/targets/remove
// Body: {"id": "target-id"}
func (h *HTTPHandler) handleRemoveTarget(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效请求体", http.StatusBadRequest)
		return
	}

	if err := h.manager.RemoveTarget(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, map[string]string{"status": "removed"})
}

// ============================================================
// 威胁检测 API
// ============================================================

// handleGetDetections GET /api/ransomhoneypot/detections?limit=50&level=3
func (h *HTTPHandler) handleGetDetections(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := parseIntParam(r, "limit", 50)
	level := parseIntParam(r, "level", 0)

	writeJSON(w, h.manager.GetDetections(limit, level))
}

// handleGetEvents GET /api/ransomhoneypot/events?limit=100&type=modify
func (h *HTTPHandler) handleGetEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := parseIntParam(r, "limit", 100)
	eventType := r.URL.Query().Get("type")

	writeJSON(w, h.manager.GetEvents(limit, eventType))
}

// ============================================================
// 配置 API
// ============================================================

// handleConfig GET/PUT /api/ransomhoneypot/config
func (h *HTTPHandler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, h.manager.GetConfig())

	case http.MethodPut:
		var config HoneypotConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			http.Error(w, "无效配置: "+err.Error(), http.StatusBadRequest)
			return
		}
		h.manager.UpdateConfig(&config)
		writeJSON(w, map[string]string{"status": "updated"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ============================================================
// 威胁模式管理 API
// ============================================================

// handlePatterns GET/POST /api/ransomhoneypot/patterns
func (h *HTTPHandler) handlePatterns(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, h.manager.detector.ListPatterns())

	case http.MethodPost:
		var pattern ThreatPattern
		if err := json.NewDecoder(r.Body).Decode(&pattern); err != nil {
			http.Error(w, "无效请求体: "+err.Error(), http.StatusBadRequest)
			return
		}
		if err := h.manager.detector.AddPattern(&pattern); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		writeJSON(w, pattern)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// ============================================================
// 辅助函数
// ============================================================

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	json.NewEncoder(w).Encode(data)
}

// parseIntParam 解析 URL 查询参数为整数
func parseIntParam(r *http.Request, key string, defaultVal int) int {
	s := r.URL.Query().Get(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return v
}
