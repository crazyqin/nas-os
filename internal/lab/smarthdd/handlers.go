package smarthdd

import (
	"encoding/json"
	"net/http"
)

// SmartHDDHandler HTTP处理器.
type SmartHDDHandler struct {
	manager *SmartHDDManager
}

// NewSmartHDDHandler 创建处理器.
func NewSmartHDDHandler(manager *SmartHDDManager) *SmartHDDHandler {
	return &SmartHDDHandler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *SmartHDDHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/smarthdd/disks", h.handleListDisks)
	mux.HandleFunc("/api/smarthdd/disk/get", h.handleGetDisk)
	mux.HandleFunc("/api/smarthdd/disk/register", h.handleRegisterDisk)
	mux.HandleFunc("/api/smarthdd/disk/unregister", h.handleUnregisterDisk)
	mux.HandleFunc("/api/smarthdd/disk/scan", h.handleScanDisk)
	mux.HandleFunc("/api/smarthdd/scan/all", h.handleScanAll)
	mux.HandleFunc("/api/smarthdd/stats", h.handleGetStats)
	mux.HandleFunc("/api/smarthdd/alerts", h.handleGetAlerts)
	mux.HandleFunc("/api/smarthdd/alert/resolve", h.handleResolveAlert)
}

func (h *SmartHDDHandler) handleListDisks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	disks := h.manager.ListDisks()
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": disks})
}

func (h *SmartHDDHandler) handleGetDisk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "缺少id参数"})
		return
	}
	disk, err := h.manager.GetDisk(id)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": disk})
}

func (h *SmartHDDHandler) handleRegisterDisk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var disk DiskInfo
	if err := json.NewDecoder(r.Body).Decode(&disk); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.RegisterDisk(&disk); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": disk})
}

func (h *SmartHDDHandler) handleUnregisterDisk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.UnregisterDisk(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *SmartHDDHandler) handleScanDisk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	disk, err := h.manager.ScanDisk(req.ID)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": disk})
}

func (h *SmartHDDHandler) handleScanAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	h.manager.ScanAll()
	writeJSON(w, map[string]interface{}{"code": 0, "message": "扫描完成"})
}

func (h *SmartHDDHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := h.manager.GetStats()
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": stats})
}

func (h *SmartHDDHandler) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	resolved := r.URL.Query().Get("resolved") == "true"
	alerts := h.manager.GetAlerts(resolved)
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": alerts})
}

func (h *SmartHDDHandler) handleResolveAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.ResolveAlert(req.ID); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
