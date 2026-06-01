// Package filesync 提供文件同步 HTTP API 处理器
package filesync

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 文件同步 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册 HTTP 路由
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	// 同步控制
	mux.HandleFunc("/api/v1/filesync/sync/status", h.handleSyncStatus)
	mux.HandleFunc("/api/v1/filesync/sync/start", h.handleSyncStart)
	mux.HandleFunc("/api/v1/filesync/sync/stop", h.handleSyncStop)

	// 冲突管理
	mux.HandleFunc("/api/v1/filesync/sync/conflicts", h.handleConflicts)
	mux.HandleFunc("/api/v1/filesync/sync/conflicts/resolve", h.handleResolveConflict)

	// 同步历史
	mux.HandleFunc("/api/v1/filesync/sync/history", h.handleHistory)
	mux.HandleFunc("/api/v1/filesync/sync/history/restore", h.handleRestoreVersion)

	// 同步任务
	mux.HandleFunc("/api/v1/filesync/sync/tasks", h.handleTasks)
	mux.HandleFunc("/api/v1/filesync/sync/tasks/resume", h.handleResumeTransfer)
	mux.HandleFunc("/api/v1/filesync/sync/tasks/info", h.handleTransferInfo)

	// 设备管理
	mux.HandleFunc("/api/v1/filesync/devices", h.handleDevices)
	mux.HandleFunc("/api/v1/filesync/devices/register", h.handleDeviceRegister)
	mux.HandleFunc("/api/v1/filesync/devices/remove", h.handleDeviceRemove)

	// 文件夹管理
	mux.HandleFunc("/api/v1/filesync/folders", h.handleFolders)
	mux.HandleFunc("/api/v1/filesync/folders/create", h.handleFolderCreate)
	mux.HandleFunc("/api/v1/filesync/folders/delete", h.handleFolderDelete)

	// 带宽管理
	mux.HandleFunc("/api/v1/filesync/bandwidth", h.handleBandwidth)
	mux.HandleFunc("/api/v1/filesync/bandwidth/update", h.handleBandwidthUpdate)
	mux.HandleFunc("/api/v1/filesync/bandwidth/toggle", h.handleBandwidthToggle)

	// 选择性同步
	mux.HandleFunc("/api/v1/filesync/selective", h.handleSelectiveSync)
	mux.HandleFunc("/api/v1/filesync/selective/create", h.handleSelectiveSyncCreate)
	mux.HandleFunc("/api/v1/filesync/selective/delete", h.handleSelectiveSyncDelete)

	// 统计
	mux.HandleFunc("/api/v1/filesync/stats", h.handleStats)
}

// apiResponse 标准 API 响应
type apiResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, resp apiResponse) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, apiResponse{Code: 1, Message: msg})
}

// handleSyncStatus 获取同步状态
func (h *Handlers) handleSyncStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	engine := h.manager.GetSyncStatus()
	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "success", Data: engine,
	})
}

// handleSyncStart 启动同步
func (h *Handlers) handleSyncStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req SyncStartRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	engine, err := h.manager.StartSync(&req)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "sync started", Data: engine,
	})
}

// handleSyncStop 停止同步
func (h *Handlers) handleSyncStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req SyncStopRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	engine, err := h.manager.StopSync(&req)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "sync stopped", Data: engine,
	})
}

// handleConflicts 获取冲突列表
func (h *Handlers) handleConflicts(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	folderID := r.URL.Query().Get("folder_id")
	conflicts := h.manager.GetConflicts(folderID)
	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "success", Data: conflicts,
	})
}

// handleResolveConflict 解决冲突
func (h *Handlers) handleResolveConflict(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req ConflictResolveRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	conflict, err := h.manager.ResolveConflict(&req)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "conflict resolved", Data: conflict,
	})
}

// handleHistory 获取同步历史
func (h *Handlers) handleHistory(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	req := HistoryRequest{
		FolderID: r.URL.Query().Get("folder_id"),
		FilePath: r.URL.Query().Get("file_path"),
	}

	// 解析分页参数
	if limitStr := r.URL.Query().Get("limit"); limitStr != "" {
		if v, err := strconv.Atoi(limitStr); err == nil {
			req.Limit = v
		}
	}
	if offsetStr := r.URL.Query().Get("offset"); offsetStr != "" {
		if v, err := strconv.Atoi(offsetStr); err == nil {
			req.Offset = v
		}
	}

	history := h.manager.GetSyncHistory(&req)
	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "success", Data: history,
	})
}

// handleRestoreVersion 恢复文件版本
func (h *Handlers) handleRestoreVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		HistoryID string `json:"history_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	history, err := h.manager.RestoreVersion(req.HistoryID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "version restored", Data: history,
	})
}

// handleTasks 获取同步任务
func (h *Handlers) handleTasks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	tasks := h.manager.GetSyncTasks(
		r.URL.Query().Get("folder_id"),
		r.URL.Query().Get("device_id"),
	)
	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "success", Data: tasks,
	})
}

// handleTransferInfo 获取断点续传信息
func (h *Handlers) handleTransferInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "task_id is required")
		return
	}

	info, err := h.manager.GetTransferInfo(taskID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "success", Data: info,
	})
}

// handleResumeTransfer 断点续传
func (h *Handlers) handleResumeTransfer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		TaskID string `json:"task_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	task, err := h.manager.ResumeTransfer(req.TaskID)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "transfer resumed", Data: task,
	})
}

// handleDevices 列出设备
func (h *Handlers) handleDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := r.URL.Query().Get("id")
	if id != "" {
		device, err := h.manager.GetDevice(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code: 0, Message: "success", Data: device,
		})
		return
	}

	devices := h.manager.ListDevices()
	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "success", Data: devices,
	})
}

// handleDeviceRegister 注册设备
func (h *Handlers) handleDeviceRegister(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req DeviceRegisterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	device := h.manager.RegisterDevice(&req)
	writeJSON(w, http.StatusCreated, apiResponse{
		Code: 0, Message: "device registered", Data: device,
	})
}

// handleDeviceRemove 移除设备
func (h *Handlers) handleDeviceRemove(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		DeviceID string `json:"device_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if err := h.manager.RemoveDevice(req.DeviceID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "device removed",
	})
}

// handleFolders 列出同步文件夹
func (h *Handlers) handleFolders(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	id := r.URL.Query().Get("id")
	if id != "" {
		folder, err := h.manager.GetFolder(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, apiResponse{
			Code: 0, Message: "success", Data: folder,
		})
		return
	}

	folders := h.manager.ListFolders()
	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "success", Data: folders,
	})
}

// handleFolderCreate 创建同步文件夹
func (h *Handlers) handleFolderCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req FolderCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	folder := h.manager.CreateFolder(&req)
	writeJSON(w, http.StatusCreated, apiResponse{
		Code: 0, Message: "folder created", Data: folder,
	})
}

// handleFolderDelete 删除同步文件夹
func (h *Handlers) handleFolderDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		FolderID string `json:"folder_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if err := h.manager.DeleteFolder(req.FolderID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "folder deleted",
	})
}

// handleBandwidth 获取带宽限制
func (h *Handlers) handleBandwidth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	limits := h.manager.GetBandwidthLimits()
	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "success", Data: limits,
	})
}

// handleBandwidthUpdate 更新带宽限制
func (h *Handlers) handleBandwidthUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		ID            string `json:"id"`
		UploadLimit   int64  `json:"upload_limit"`
		DownloadLimit int64  `json:"download_limit"`
		ScheduleStart string `json:"schedule_start"`
		ScheduleEnd   string `json:"schedule_end"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	bw, err := h.manager.UpdateBandwidthLimit(req.ID, req.UploadLimit, req.DownloadLimit, req.ScheduleStart, req.ScheduleEnd)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "bandwidth limit updated", Data: bw,
	})
}

// handleBandwidthToggle 启用/禁用带宽限制
func (h *Handlers) handleBandwidthToggle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		ID      string `json:"id"`
		Enabled bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	bw, err := h.manager.SetBandwidthEnabled(req.ID, req.Enabled)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "bandwidth limit toggled", Data: bw,
	})
}

// handleSelectiveSync 列出选择性同步规则
func (h *Handlers) handleSelectiveSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	rules := h.manager.ListSelectiveSyncRules(
		r.URL.Query().Get("folder_id"),
		r.URL.Query().Get("device_id"),
	)
	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "success", Data: rules,
	})
}

// handleSelectiveSyncCreate 创建选择性同步规则
func (h *Handlers) handleSelectiveSyncCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		FolderID    string `json:"folder_id"`
		DeviceID    string `json:"device_id"`
		PathPattern string `json:"path_pattern"`
		Type        string `json:"type"`
		Priority    int    `json:"priority"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if strings.TrimSpace(req.PathPattern) == "" {
		writeError(w, http.StatusBadRequest, "path_pattern is required")
		return
	}

	rule := h.manager.CreateSelectiveSyncRule(req.FolderID, req.DeviceID, req.PathPattern, req.Type, req.Priority)
	writeJSON(w, http.StatusCreated, apiResponse{
		Code: 0, Message: "selective sync rule created", Data: rule,
	})
}

// handleSelectiveSyncDelete 删除选择性同步规则
func (h *Handlers) handleSelectiveSyncDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		RuleID string `json:"rule_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request: "+err.Error())
		return
	}

	if err := h.manager.DeleteSelectiveSyncRule(req.RuleID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "selective sync rule deleted",
	})
}

// handleStats 获取同步统计
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	stats := h.manager.GetSyncStats()
	writeJSON(w, http.StatusOK, apiResponse{
		Code: 0, Message: "success", Data: stats,
	})
}

// GinHandler gin 兼容处理器
type GinHandler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建 gin 兼容处理器
func NewHandler(syncMgr *SyncManager, logger *zap.Logger) *GinHandler {
	return &GinHandler{manager: syncMgr.Manager, logger: logger}
}

// RegisterRoutes 注册 gin 路由
func (gh *GinHandler) RegisterRoutes(rg *gin.RouterGroup) {
	sync := rg.Group("/filesync")
	{
		sync.GET("/status", gh.ginSyncStatus)
		sync.POST("/start", gh.ginSyncStart)
		sync.POST("/stop", gh.ginSyncStop)
		sync.GET("/conflicts", gh.ginConflicts)
		sync.GET("/history", gh.ginHistory)
		sync.GET("/tasks", gh.ginTasks)
		sync.GET("/devices", gh.ginDevices)
		sync.GET("/folders", gh.ginFolders)
		sync.GET("/stats", gh.ginStats)
	}
}

func (gh *GinHandler) ginSyncStatus(c *gin.Context) {
	status := gh.manager.GetSyncStatus()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": status})
}

func (gh *GinHandler) ginSyncStart(c *gin.Context) {
	var req SyncStartRequest
	c.ShouldBindJSON(&req)
	result, err := gh.manager.StartSync(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "同步已启动", "data": result})
}

func (gh *GinHandler) ginSyncStop(c *gin.Context) {
	var req SyncStopRequest
	c.ShouldBindJSON(&req)
	result, err := gh.manager.StopSync(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"code": 500, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "同步已停止", "data": result})
}

func (gh *GinHandler) ginConflicts(c *gin.Context) {
	folderID := c.Query("folderId")
	conflicts := gh.manager.GetConflicts(folderID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": conflicts})
}

func (gh *GinHandler) ginHistory(c *gin.Context) {
	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	history := gh.manager.GetSyncHistory(&HistoryRequest{Limit: limit})
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": history})
}

func (gh *GinHandler) ginTasks(c *gin.Context) {
	folderID := c.Query("folderId")
	deviceID := c.Query("deviceId")
	tasks := gh.manager.GetSyncTasks(folderID, deviceID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": tasks})
}

func (gh *GinHandler) ginDevices(c *gin.Context) {
	devices := gh.manager.ListDevices()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": devices})
}

func (gh *GinHandler) ginFolders(c *gin.Context) {
	folders := gh.manager.ListFolders()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": folders})
}

func (gh *GinHandler) ginStats(c *gin.Context) {
	stats := gh.manager.GetSyncStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "success", "data": stats})
}
