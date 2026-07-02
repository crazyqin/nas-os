// Package upssmart 提供 UPS 智能管理功能
package upssmart

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"time"
)

// APIResponse 统一 API 响应结构.
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
	Message string      `json:"message,omitempty"`
}

// UPSStatusResponse UPS 状态响应.
type UPSStatusResponse struct {
	Devices     []*UPSDevice `json:"devices"`
	TotalCount  int          `json:"total_count"`
	PrimaryID   string       `json:"primary_id,omitempty"`
	LastUpdated time.Time    `json:"last_updated"`
}

// BatteryResponse 电池详情响应.
type BatteryResponse struct {
	Health        *BatteryHealth `json:"health"`
	UPSID         string         `json:"ups_id"`
	UPSName       string         `json:"ups_name"`
	CurrentStatus UPSStatus      `json:"current_status"`
}

// EventsResponse 事件历史响应.
type EventsResponse struct {
	Events  []PowerEventRecord `json:"events"`
	Total   int                `json:"total"`
	HasMore bool               `json:"has_more"`
}

// TestRequest 测试请求.
type TestRequest struct {
	UPSID string `json:"ups_id"`
}

// Handler UPS 管理 API 处理器.
type Handler struct {
	upsManager      *UPSManager
	batteryManager  *BatteryManager
	shutdownManager *ShutdownManager
}

// NewHandler 创建 API 处理器.
func NewHandler(
	upsManager *UPSManager,
	batteryManager *BatteryManager,
	shutdownManager *ShutdownManager,
) *Handler {
	return &Handler{
		upsManager:      upsManager,
		batteryManager:  batteryManager,
		shutdownManager: shutdownManager,
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux, basePath string) {
	mux.HandleFunc(basePath+"/ups/status", h.handleUPSStatus)
	mux.HandleFunc(basePath+"/ups/battery", h.handleBattery)
	mux.HandleFunc(basePath+"/ups/test", h.handleTest)
	mux.HandleFunc(basePath+"/ups/shutdown-policy", h.handleShutdownPolicy)
	mux.HandleFunc(basePath+"/ups/events", h.handleEvents)

	log.Printf("✅ UPS 管理 API 已注册: %s", basePath)
}

// handleUPSStatus 处理 UPS 状态请求
// GET /api/v1/ups/status.
func (h *Handler) handleUPSStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	devices := h.upsManager.GetAllDevices()

	// 获取主 UPS ID
	var primaryID string
	primary, err := h.upsManager.GetPrimaryUPS()
	if err == nil {
		primaryID = primary.ID
	}

	response := UPSStatusResponse{
		Devices:     devices,
		TotalCount:  len(devices),
		PrimaryID:   primaryID,
		LastUpdated: time.Now(),
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    response,
	})
}

// handleBattery 处理电池详情请求
// GET /api/v1/ups/battery?ups_id=xxx.
func (h *Handler) handleBattery(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	upsID := r.URL.Query().Get("ups_id")
	if upsID == "" {
		// 默认返回所有电池信息
		allHealth := h.batteryManager.GetAllBatteryHealth()
		writeJSON(w, http.StatusOK, APIResponse{
			Success: true,
			Data:    allHealth,
		})
		return
	}

	// 获取指定 UPS 的电池信息
	health, err := h.batteryManager.GetBatteryHealth(upsID)
	if err != nil {
		writeError(w, http.StatusNotFound, fmt.Sprintf("未找到 UPS %s 的电池信息", upsID))
		return
	}

	// 获取 UPS 设备信息
	device, err := h.upsManager.GetDevice(upsID)
	var upsName string
	var currentStatus UPSStatus
	if err == nil {
		upsName = device.Name
		currentStatus = device.Status
	}

	response := BatteryResponse{
		Health:        health,
		UPSID:         upsID,
		UPSName:       upsName,
		CurrentStatus: currentStatus,
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    response,
	})
}

// handleTest 处理电池测试请求
// POST /api/v1/ups/test.
func (h *Handler) handleTest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req TestRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	if req.UPSID == "" {
		writeError(w, http.StatusBadRequest, "ups_id 不能为空")
		return
	}

	// 触发电池测试
	if err := h.batteryManager.TriggerBatteryTest(req.UPSID); err != nil {
		writeError(w, http.StatusInternalServerError, fmt.Sprintf("触发测试失败: %v", err))
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: fmt.Sprintf("UPS %s 的电池测试已加入队列", req.UPSID),
	})
}

// handleShutdownPolicy 处理关机策略请求
// GET/PUT /api/v1/ups/shutdown-policy.
func (h *Handler) handleShutdownPolicy(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getShutdownPolicy(w, r)
	case http.MethodPut:
		h.updateShutdownPolicy(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET/PUT 方法")
	}
}

// getShutdownPolicy 获取关机策略.
func (h *Handler) getShutdownPolicy(w http.ResponseWriter, r *http.Request) {
	policy := h.shutdownManager.GetPolicy()
	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    policy,
	})
}

// updateShutdownPolicy 更新关机策略.
func (h *Handler) updateShutdownPolicy(w http.ResponseWriter, r *http.Request) {
	var policy ShutdownPolicy
	if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
		writeError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	// 验证参数
	if policy.BatteryThreshold < 0 || policy.BatteryThreshold > 100 {
		writeError(w, http.StatusBadRequest, "电量阈值必须在 0-100 之间")
		return
	}

	if policy.LoadThreshold < 0 || policy.LoadThreshold > 100 {
		writeError(w, http.StatusBadRequest, "负载阈值必须在 0-100 之间")
		return
	}

	h.shutdownManager.UpdatePolicy(policy)

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Message: "关机策略已更新",
		Data:    policy,
	})
}

// handleEvents 处理电源事件请求
// GET /api/v1/ups/events?limit=50&type=power_out.
func (h *Handler) handleEvents(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	// 解析查询参数
	query := r.URL.Query()

	// 获取限制数量
	limit := 50
	if limitStr := query.Get("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	// 获取事件类型过滤
	var eventFilter *PowerEvent
	if eventType := query.Get("type"); eventType != "" {
		evt := PowerEvent(eventType)
		eventFilter = &evt
	}

	// 获取事件列表
	events := h.upsManager.GetEvents(limit, eventFilter)

	response := EventsResponse{
		Events:  events,
		Total:   len(events),
		HasMore: len(events) >= limit,
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data:    response,
	})
}

// writeJSON 写入 JSON 响应.
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	if err := json.NewEncoder(w).Encode(data); err != nil {
		log.Printf("写入 JSON 响应失败: %v", err)
	}
}

// writeError 写入错误响应.
func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, APIResponse{
		Success: false,
		Error:   message,
	})
}
