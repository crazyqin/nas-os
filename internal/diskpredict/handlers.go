package diskpredict

import (
	"encoding/json"
	"net/http"
	"time"
)

// DiskPredictHandler HTTP 处理器
type DiskPredictHandler struct {
	manager *DiskPredictManager
}

// NewDiskPredictHandler 创建处理器
func NewDiskPredictHandler(manager *DiskPredictManager) *DiskPredictHandler {
	return &DiskPredictHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *DiskPredictHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/disk/predict", h.handlePredict)
	mux.HandleFunc("/api/v1/disk/predict/all", h.handlePredictAll)
	mux.HandleFunc("/api/v1/disk/predictions", h.handleListPredictions)
	mux.HandleFunc("/api/v1/disk/prediction/get", h.handleGetPrediction)
	mux.HandleFunc("/api/v1/disk/list", h.handleListDisks)
	mux.HandleFunc("/api/v1/disk/get", h.handleGetDisk)
	mux.HandleFunc("/api/v1/disk/register", h.handleRegisterDisk)
	mux.HandleFunc("/api/v1/disk/unregister", h.handleUnregisterDisk)
	mux.HandleFunc("/api/v1/disk/smart/update", h.handleUpdateSMARTData)
	mux.HandleFunc("/api/v1/disk/stats", h.handleGetStats)
	mux.HandleFunc("/api/v1/disk/alerts", h.handleGetAlerts)
	mux.HandleFunc("/api/v1/disk/alert/resolve", h.handleResolveAlert)
}

// handlePredict 处理单个磁盘预测请求
func (h *DiskPredictHandler) handlePredict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var device string

	// GET 请求从查询参数获取
	if r.Method == http.MethodGet {
		device = r.URL.Query().Get("device")
	} else {
		// POST 请求从请求体获取
		var req PredictRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, PredictResponse{
				Code:    400,
				Message: "无效的请求体",
			})
			return
		}
		device = req.Device
	}

	if device == "" {
		writeJSON(w, PredictResponse{
			Code:    400,
			Message: "缺少device参数",
		})
		return
	}

	result, err := h.manager.PredictFailure(device)
	if err != nil {
		writeJSON(w, PredictResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, PredictResponse{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// handlePredictAll 处理预测所有磁盘请求
func (h *DiskPredictHandler) handlePredictAll(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	results := h.manager.PredictAll()

	writeJSON(w, PredictionListResponse{
		Code:    0,
		Message: "success",
		Data:    results,
	})
}

// handleListPredictions 处理列出所有预测结果请求
func (h *DiskPredictHandler) handleListPredictions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	predictions := h.manager.ListPredictions()

	writeJSON(w, PredictionListResponse{
		Code:    0,
		Message: "success",
		Data:    predictions,
	})
}

// handleGetPrediction 处理获取单个预测结果请求
func (h *DiskPredictHandler) handleGetPrediction(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	device := r.URL.Query().Get("device")
	if device == "" {
		writeJSON(w, PredictResponse{
			Code:    400,
			Message: "缺少device参数",
		})
		return
	}

	prediction, err := h.manager.GetPrediction(device)
	if err != nil {
		writeJSON(w, PredictResponse{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, PredictResponse{
		Code:    0,
		Message: "success",
		Data:    prediction,
	})
}

// handleListDisks 处理列出所有磁盘请求
func (h *DiskPredictHandler) handleListDisks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	disks := h.manager.ListDisks()

	writeJSON(w, DiskListResponse{
		Code:    0,
		Message: "success",
		Data:    disks,
	})
}

// handleGetDisk 处理获取单个磁盘信息请求
func (h *DiskPredictHandler) handleGetDisk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	device := r.URL.Query().Get("device")
	if device == "" {
		writeJSON(w, PredictResponse{
			Code:    400,
			Message: "缺少device参数",
		})
		return
	}

	disk, err := h.manager.GetDisk(device)
	if err != nil {
		writeJSON(w, PredictResponse{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, PredictResponse{
		Code:    0,
		Message: "success",
		Data:    disk,
	})
}

// handleRegisterDisk 处理注册磁盘请求
func (h *DiskPredictHandler) handleRegisterDisk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var disk DiskInfo
	if err := json.NewDecoder(r.Body).Decode(&disk); err != nil {
		writeJSON(w, PredictResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if err := h.manager.RegisterDisk(&disk); err != nil {
		writeJSON(w, PredictResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, PredictResponse{
		Code:    0,
		Message: "success",
		Data:    disk,
	})
}

// handleUnregisterDisk 处理注销磁盘请求
func (h *DiskPredictHandler) handleUnregisterDisk(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Device string `json:"device"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, PredictResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if err := h.manager.UnregisterDisk(req.Device); err != nil {
		writeJSON(w, PredictResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, PredictResponse{
		Code:    0,
		Message: "success",
	})
}

// handleUpdateSMARTData 处理更新SMART数据请求
func (h *DiskPredictHandler) handleUpdateSMARTData(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var data SMARTData
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		writeJSON(w, PredictResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if err := h.manager.UpdateSMARTData(&data); err != nil {
		writeJSON(w, PredictResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, PredictResponse{
		Code:    0,
		Message: "success",
	})
}

// handleGetStats 处理获取统计信息请求
func (h *DiskPredictHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	stats := h.manager.GetStats()

	writeJSON(w, StatsResponse{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// handleGetAlerts 处理获取告警信息请求
func (h *DiskPredictHandler) handleGetAlerts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resolved := r.URL.Query().Get("resolved") == "true"
	alerts := h.manager.GetAlerts(resolved)

	writeJSON(w, AlertListResponse{
		Code:    0,
		Message: "success",
		Data:    alerts,
	})
}

// handleResolveAlert 处理解决告警请求
func (h *DiskPredictHandler) handleResolveAlert(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Device    string `json:"device"`
		CreatedAt string `json:"created_at"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, PredictResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	// 解析时间
	createdAt, err := time.Parse(time.RFC3339, req.CreatedAt)
	if err != nil {
		writeJSON(w, PredictResponse{
			Code:    400,
			Message: "无效的时间格式",
		})
		return
	}

	if err := h.manager.ResolveAlert(req.Device, createdAt); err != nil {
		writeJSON(w, PredictResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, PredictResponse{
		Code:    0,
		Message: "success",
	})
}

// writeJSON 写入JSON响应
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
