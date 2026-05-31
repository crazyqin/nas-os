package aifilerecovery

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP API处理器
type Handler struct {
	manager *RecoveryManager
}

// NewHandler 创建处理器
func NewHandler(manager *RecoveryManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/scan", h.handleScan)
	mux.HandleFunc(prefix+"/scan/status", h.handleScanStatus)
	mux.HandleFunc(prefix+"/results", h.handleResults)
	mux.HandleFunc(prefix+"/recover", h.handleRecover)
	mux.HandleFunc(prefix+"/files", h.handleFiles)
}

// handleScan 处理扫描请求
func (h *Handler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var config ScanConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	result, err := h.manager.StartScan(r.Context(), config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusConflict)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(result)
}

// handleScanStatus 处理扫描状态查询
func (h *Handler) handleScanStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	scanID := r.URL.Query().Get("id")
	if scanID == "" {
		http.Error(w, "missing scan id", http.StatusBadRequest)
		return
	}

	result, ok := h.manager.GetScanResult(scanID)
	if !ok {
		http.Error(w, "scan not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// handleResults 处理结果列表
func (h *Handler) handleResults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	results := h.manager.ListScanResults()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// handleRecover 处理恢复请求
func (h *Handler) handleRecover(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FileID     string `json:"file_id"`
		OutputPath string `json:"output_path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.RecoverFile(r.Context(), req.FileID, req.OutputPath); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "recovered"})
}

// handleFiles 处理文件列表
func (h *Handler) handleFiles(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	scanID := r.URL.Query().Get("scan_id")
	if scanID == "" {
		http.Error(w, "missing scan id", http.StatusBadRequest)
		return
	}

	result, ok := h.manager.GetScanResult(scanID)
	if !ok {
		http.Error(w, "scan not found", http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result.Files)
}
