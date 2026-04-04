// Package vm 提供虚拟机管理 API 接口
package vm

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
	"time"

	"go.uber.org/zap"
)

// Handlers VM API 处理器
type Handlers struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandlers 创建 VM API 处理器
func NewHandlers(manager *Manager, logger *zap.Logger) *Handlers {
	return &Handlers{
		manager: manager,
		logger:  logger,
	}
}

// NewHandler 兼容旧代码的创建函数
func NewHandler(vmMgr *Manager, isoMgr *ISOManager, snapshotMgr *SnapshotManager, logger *zap.Logger) *Handlers {
	return &Handlers{
		manager: vmMgr,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	// VM 管理
	mux.HandleFunc("GET /api/v1/vms", h.ListVMs)
	mux.HandleFunc("POST /api/v1/vms", h.CreateVM)
	mux.HandleFunc("GET /api/v1/vms/{id}", h.GetVM)
	mux.HandleFunc("PUT /api/v1/vms/{id}", h.UpdateVM)
	mux.HandleFunc("DELETE /api/v1/vms/{id}", h.DeleteVM)
	mux.HandleFunc("POST /api/v1/vms/{id}/start", h.StartVM)
	mux.HandleFunc("POST /api/v1/vms/{id}/stop", h.StopVM)
	mux.HandleFunc("POST /api/v1/vms/{id}/restart", h.RestartVM)
	mux.HandleFunc("GET /api/v1/vms/{id}/stats", h.GetVMStats)
	mux.HandleFunc("GET /api/v1/vms/{id}/vnc", h.GetVNCConnection)

	// 模板管理
	mux.HandleFunc("GET /api/v1/vm/templates", h.ListTemplates)
	mux.HandleFunc("GET /api/v1/vm/templates/{id}", h.GetTemplate)
	mux.HandleFunc("POST /api/v1/vm/templates", h.CreateTemplate)
	mux.HandleFunc("DELETE /api/v1/vm/templates/{id}", h.DeleteTemplate)

	// ISO 管理
	mux.HandleFunc("GET /api/v1/vm/isos", h.ListISOs)
	mux.HandleFunc("POST /api/v1/vm/isos", h.UploadISO)
	mux.HandleFunc("DELETE /api/v1/vm/isos/{id}", h.DeleteISO)

	// 硬件设备
	mux.HandleFunc("GET /api/v1/vm/usb-devices", h.ListUSBDevices)
	mux.HandleFunc("GET /api/v1/vm/pci-devices", h.ListPCIDevices)
}

// ListVMs 列出所有虚拟机
func (h *Handlers) ListVMs(w http.ResponseWriter, r *http.Request) {
	vms := h.manager.ListVMs()
	h.jsonResponse(w, vms)
}

// CreateVM 创建虚拟机
func (h *Handlers) CreateVM(w http.ResponseWriter, r *http.Request) {
	var config Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "无效的请求体: "+err.Error())
		return
	}

	vm, err := h.manager.CreateVM(r.Context(), config)
	if err != nil {
		h.logger.Error("创建 VM 失败", zap.Error(err))
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, vm)
}

// GetVM 获取虚拟机详情
func (h *Handlers) GetVM(w http.ResponseWriter, r *http.Request) {
	vmID := r.PathValue("id")
	vm, err := h.manager.GetVM(vmID)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	h.jsonResponse(w, vm)
}

// UpdateVM 更新虚拟机配置
func (h *Handlers) UpdateVM(w http.ResponseWriter, r *http.Request) {
	vmID := r.PathValue("id")

	var config Config
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "无效的请求体: "+err.Error())
		return
	}

	vm, err := h.manager.UpdateVM(r.Context(), vmID, config)
	if err != nil {
		h.logger.Error("更新 VM 失败", zap.Error(err), zap.String("vmId", vmID))
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, vm)
}

// DeleteVM 删除虚拟机
func (h *Handlers) DeleteVM(w http.ResponseWriter, r *http.Request) {
	vmID := r.PathValue("id")
	force := r.URL.Query().Get("force") == "true"

	err := h.manager.DeleteVM(r.Context(), vmID, force)
	if err != nil {
		h.logger.Error("删除 VM 失败", zap.Error(err), zap.String("vmId", vmID))
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, map[string]string{"message": "VM 已删除"})
}

// StartVM 启动虚拟机
func (h *Handlers) StartVM(w http.ResponseWriter, r *http.Request) {
	vmID := r.PathValue("id")

	err := h.manager.StartVM(r.Context(), vmID)
	if err != nil {
		h.logger.Error("启动 VM 失败", zap.Error(err), zap.String("vmId", vmID))
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, map[string]string{"message": "VM 已启动"})
}

// StopVM 停止虚拟机
func (h *Handlers) StopVM(w http.ResponseWriter, r *http.Request) {
	vmID := r.PathValue("id")
	force := r.URL.Query().Get("force") == "true"

	err := h.manager.StopVM(r.Context(), vmID, force)
	if err != nil {
		h.logger.Error("停止 VM 失败", zap.Error(err), zap.String("vmId", vmID))
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, map[string]string{"message": "VM 已停止"})
}

// RestartVM 重启虚拟机
func (h *Handlers) RestartVM(w http.ResponseWriter, r *http.Request) {
	vmID := r.PathValue("id")

	// 先停止
	if err := h.manager.StopVM(r.Context(), vmID, false); err != nil {
		if !strings.Contains(err.Error(), "已停止") {
			h.errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// 再启动
	if err := h.manager.StartVM(r.Context(), vmID); err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, map[string]string{"message": "VM 已重启"})
}

// GetVMStats 获取虚拟机统计信息
func (h *Handlers) GetVMStats(w http.ResponseWriter, r *http.Request) {
	vmID := r.PathValue("id")

	stats, err := h.manager.GetStats(vmID)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	h.jsonResponse(w, stats)
}

// GetVNCConnection 获取 VNC 连接信息
func (h *Handlers) GetVNCConnection(w http.ResponseWriter, r *http.Request) {
	vmID := r.PathValue("id")

	conn, err := h.manager.GetVNCConnection(vmID)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	h.jsonResponse(w, conn)
}

// ListTemplates 列出 VM 模板
func (h *Handlers) ListTemplates(w http.ResponseWriter, r *http.Request) {
	templates := h.manager.ListTemplates()
	h.jsonResponse(w, templates)
}

// GetTemplate 获取模板详情
func (h *Handlers) GetTemplate(w http.ResponseWriter, r *http.Request) {
	templateID := r.PathValue("id")

	tpl, err := h.manager.GetTemplate(templateID)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	h.jsonResponse(w, tpl)
}

// CreateTemplate 创建模板
func (h *Handlers) CreateTemplate(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string            `json:"name"`
		Description string            `json:"description"`
		Type        Type              `json:"type"`
		CPU         int               `json:"cpu"`
		Memory      uint64            `json:"memory"`
		DiskSize    uint64            `json:"diskSize"`
		Network     string            `json:"network"`
		OS          string            `json:"os"`
		Tags        map[string]string `json:"tags"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	tpl, err := h.manager.CreateTemplate(req.Name, req.Description, req.Type, req.CPU, req.Memory, req.DiskSize, req.Network, req.OS, req.Tags)
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, tpl)
}

// DeleteTemplate 删除模板
func (h *Handlers) DeleteTemplate(w http.ResponseWriter, r *http.Request) {
	templateID := r.PathValue("id")

	err := h.manager.DeleteTemplate(templateID)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	h.jsonResponse(w, map[string]string{"message": "模板已删除"})
}

// ListISOs 列出 ISO 镜像
func (h *Handlers) ListISOs(w http.ResponseWriter, r *http.Request) {
	isos := h.manager.ListISOs()
	h.jsonResponse(w, isos)
}

// UploadISO 上传 ISO 镜像（返回上传 URL）
func (h *Handlers) UploadISO(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
		URL  string `json:"url"` // 可选：从 URL 下载
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.errorResponse(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	if req.URL != "" {
		// 从 URL 下载
		iso, err := h.manager.DownloadISO(r.Context(), req.Name, req.URL)
		if err != nil {
			h.errorResponse(w, http.StatusInternalServerError, err.Error())
			return
		}
		h.jsonResponse(w, iso)
		return
	}

	// 返回上传信息
	uploadInfo := h.manager.GetUploadInfo(req.Name)
	h.jsonResponse(w, uploadInfo)
}

// DeleteISO 删除 ISO 镜像
func (h *Handlers) DeleteISO(w http.ResponseWriter, r *http.Request) {
	isoID := r.PathValue("id")

	err := h.manager.DeleteISO(isoID)
	if err != nil {
		h.errorResponse(w, http.StatusNotFound, err.Error())
		return
	}

	h.jsonResponse(w, map[string]string{"message": "ISO 已删除"})
}

// ListUSBDevices 列出可用 USB 设备
func (h *Handlers) ListUSBDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.manager.ListUSBDevices()
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, devices)
}

// ListPCIDevices 列出可用 PCIe 设备
func (h *Handlers) ListPCIDevices(w http.ResponseWriter, r *http.Request) {
	devices, err := h.manager.ListPCIDevices()
	if err != nil {
		h.errorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.jsonResponse(w, devices)
}

// jsonResponse 返回 JSON 响应
func (h *Handlers) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(data)
}

// errorResponse 返回错误响应
func (h *Handlers) errorResponse(w http.ResponseWriter, code int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{
		"error":   http.StatusText(code),
		"message": message,
		"time":    time.Now().Format(time.RFC3339),
	})
}

// parseIntParam 解析整数参数
func parseIntParam(r *http.Request, key string, defaultValue int) int {
	val := r.URL.Query().Get(key)
	if val == "" {
		return defaultValue
	}
	result, err := strconv.Atoi(val)
	if err != nil {
		return defaultValue
	}
	return result
}

// ========== 兼容旧代码的方法别名 ==========

// HandleListVMs 兼容方法
func (h *Handlers) HandleListVMs(w http.ResponseWriter, r *http.Request) {
	h.ListVMs(w, r)
}

// HandleCreateVM 兼容方法
func (h *Handlers) HandleCreateVM(w http.ResponseWriter, r *http.Request) {
	h.CreateVM(w, r)
}

// HandleVM 兼容方法（GET/PUT/DELETE）
func (h *Handlers) HandleVM(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		h.GetVM(w, r)
	case "PUT", "POST":
		h.UpdateVM(w, r)
	case "DELETE":
		h.DeleteVM(w, r)
	}
}

// HandleListISOs 兼容方法
func (h *Handlers) HandleListISOs(w http.ResponseWriter, r *http.Request) {
	h.ListISOs(w, r)
}

// HandleISO 兼容方法
func (h *Handlers) HandleISO(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		h.ListISOs(w, r)
	case "POST":
		h.UploadISO(w, r)
	case "DELETE":
		h.DeleteISO(w, r)
	}
}

// HandleListSnapshots 兼容方法（返回空数组）
func (h *Handlers) HandleListSnapshots(w http.ResponseWriter, r *http.Request) {
	h.jsonResponse(w, []interface{}{})
}

// HandleSnapshot 兼容方法（返回未实现）
func (h *Handlers) HandleSnapshot(w http.ResponseWriter, r *http.Request) {
	h.errorResponse(w, http.StatusNotImplemented, "快照功能待实现")
}

// HandleListTemplates 兼容方法
func (h *Handlers) HandleListTemplates(w http.ResponseWriter, r *http.Request) {
	h.ListTemplates(w, r)
}

// HandleUSBDevices 兼容方法
func (h *Handlers) HandleUSBDevices(w http.ResponseWriter, r *http.Request) {
	h.ListUSBDevices(w, r)
}

// HandlePCIDevices 兼容方法
func (h *Handlers) HandlePCIDevices(w http.ResponseWriter, r *http.Request) {
	h.ListPCIDevices(w, r)
}