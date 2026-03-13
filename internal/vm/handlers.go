package vm

import (
	"encoding/json"
	"net/http"

	"go.uber.org/zap"
)

// Handler VM HTTP 处理器
type Handler struct {
	manager         *Manager
	isoManager      *ISOManager
	snapshotManager *SnapshotManager
	logger          *zap.Logger
}

// NewHandler 创建 VM 处理器
func NewHandler(manager *Manager, isoManager *ISOManager, snapshotManager *SnapshotManager, logger *zap.Logger) *Handler {
	return &Handler{
		manager:         manager,
		isoManager:      isoManager,
		snapshotManager: snapshotManager,
		logger:          logger,
	}
}

// RegisterRoutes 注册路由（保留用于标准 HTTP ServeMux）
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/vms", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			h.HandleCreateVM(w, r)
			return
		}
		h.HandleListVMs(w, r)
	})
	mux.HandleFunc("/api/v1/vms/", h.HandleVM)
	mux.HandleFunc("/api/v1/vm-isos", h.HandleListISOs)
	mux.HandleFunc("/api/v1/vm-isos/", h.HandleISO)
	mux.HandleFunc("/api/v1/vm-snapshots", h.HandleListSnapshots)
	mux.HandleFunc("/api/v1/vm-snapshots/", h.HandleSnapshot)
	mux.HandleFunc("/api/v1/vm-templates", h.HandleListTemplates)
	mux.HandleFunc("/api/v1/vm-usb-devices", h.HandleUSBDevices)
	mux.HandleFunc("/api/v1/vm-pci-devices", h.HandlePCIDevices)
}

// HandleListVMs 导出方法供 Gin 使用
func (h *Handler) HandleListVMs(w http.ResponseWriter, r *http.Request) {
	h.handleListVMs(w, r)
}

// HandleCreateVM 导出方法供 Gin 使用
func (h *Handler) HandleCreateVM(w http.ResponseWriter, r *http.Request) {
	h.handleCreateVM(w, r)
}

// HandleVM 导出方法供 Gin 使用
func (h *Handler) HandleVM(w http.ResponseWriter, r *http.Request) {
	h.handleVM(w, r)
}

// HandleListISOs 导出方法供 Gin 使用
func (h *Handler) HandleListISOs(w http.ResponseWriter, r *http.Request) {
	h.handleListISOs(w, r)
}

// HandleISO 导出方法供 Gin 使用
func (h *Handler) HandleISO(w http.ResponseWriter, r *http.Request) {
	h.handleISO(w, r)
}

// HandleListSnapshots 导出方法供 Gin 使用
func (h *Handler) HandleListSnapshots(w http.ResponseWriter, r *http.Request) {
	h.handleListSnapshots(w, r)
}

// HandleSnapshot 导出方法供 Gin 使用
func (h *Handler) HandleSnapshot(w http.ResponseWriter, r *http.Request) {
	h.handleSnapshot(w, r)
}

// HandleListTemplates 导出方法供 Gin 使用
func (h *Handler) HandleListTemplates(w http.ResponseWriter, r *http.Request) {
	h.handleListTemplates(w, r)
}

// HandleUSBDevices 导出方法供 Gin 使用
func (h *Handler) HandleUSBDevices(w http.ResponseWriter, r *http.Request) {
	h.handleUSBDevices(w, r)
}

// HandlePCIDevices 导出方法供 Gin 使用
func (h *Handler) HandlePCIDevices(w http.ResponseWriter, r *http.Request) {
	h.handlePCIDevices(w, r)
}

// VM 管理

func (h *Handler) handleListVMs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vms := h.manager.ListVMs()
	h.jsonResponse(w, vms)
}

func (h *Handler) handleVM(w http.ResponseWriter, r *http.Request) {
	vmID := r.URL.Path[len("/api/v1/vms/"):]
	if vmID == "" {
		http.Error(w, "VM ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getVM(w, r, vmID)
	case http.MethodPut:
		h.updateVM(w, r, vmID)
	case http.MethodPost:
		h.vmAction(w, r, vmID)
	case http.MethodDelete:
		h.deleteVM(w, r, vmID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) getVM(w http.ResponseWriter, r *http.Request, vmID string) {
	vm, err := h.manager.GetVM(vmID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.jsonResponse(w, vm)
}

func (h *Handler) updateVM(w http.ResponseWriter, r *http.Request, vmID string) {
	var config VMConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	vm, err := h.manager.UpdateVM(r.Context(), vmID, config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	h.jsonResponse(w, vm)
}

func (h *Handler) vmAction(w http.ResponseWriter, r *http.Request, vmID string) {
	var action struct {
		Action string `json:"action"`
		Force  bool   `json:"force"`
	}

	if err := json.NewDecoder(r.Body).Decode(&action); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	var err error
	switch action.Action {
	case "start":
		err = h.manager.StartVM(r.Context(), vmID)
	case "stop":
		err = h.manager.StopVM(r.Context(), vmID, action.Force)
	case "restart":
		err = h.manager.StopVM(r.Context(), vmID, action.Force)
		if err == nil {
			err = h.manager.StartVM(r.Context(), vmID)
		}
	case "delete":
		err = h.manager.DeleteVM(r.Context(), vmID, action.Force)
	case "vnc":
		conn, e := h.manager.GetVNCConnection(vmID)
		if e != nil {
			err = e
		} else {
			h.jsonResponse(w, conn)
			return
		}
	default:
		http.Error(w, "Unknown action: "+action.Action, http.StatusBadRequest)
		return
	}

	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, map[string]string{"status": "success"})
}

func (h *Handler) deleteVM(w http.ResponseWriter, r *http.Request, vmID string) {
	force := r.URL.Query().Get("force") == "true"

	err := h.manager.DeleteVM(r.Context(), vmID, force)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, map[string]string{"status": "success"})
}

// ISO 管理

func (h *Handler) handleListISOs(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	isos := h.isoManager.ListISOs()
	h.jsonResponse(w, isos)
}

func (h *Handler) handleISO(w http.ResponseWriter, r *http.Request) {
	isoID := r.URL.Path[len("/api/v1/vm-isos/"):]
	if isoID == "" {
		http.Error(w, "ISO ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getISO(w, r, isoID)
	case http.MethodDelete:
		h.deleteISO(w, r, isoID)
	case http.MethodPost:
		h.isoAction(w, r, isoID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) getISO(w http.ResponseWriter, r *http.Request, isoID string) {
	iso, err := h.isoManager.GetISO(isoID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.jsonResponse(w, iso)
}

func (h *Handler) deleteISO(w http.ResponseWriter, r *http.Request, isoID string) {
	err := h.isoManager.DeleteISO(isoID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, map[string]string{"status": "success"})
}

func (h *Handler) isoAction(w http.ResponseWriter, r *http.Request, isoID string) {
	var action struct {
		Action string `json:"action"`
	}

	if err := json.NewDecoder(r.Body).Decode(&action); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	switch action.Action {
	case "download":
		// 异步下载
		go func() {
			_, err := h.isoManager.DownloadISO(r.Context(), isoID, nil)
			if err != nil {
				h.logger.Error("ISO 下载失败", zap.Error(err))
			}
		}()
		h.jsonResponse(w, map[string]string{"status": "downloading"})
	default:
		http.Error(w, "Unknown action: "+action.Action, http.StatusBadRequest)
	}
}

// 快照管理

func (h *Handler) handleListSnapshots(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	vmID := r.URL.Query().Get("vmId")
	var snapshots []*VMSnapshot

	if vmID != "" {
		snapshots = h.snapshotManager.ListSnapshots(vmID)
	} else {
		// 返回所有快照
		h.jsonResponse(w, map[string]interface{}{"snapshots": "query vmId required"})
		return
	}

	h.jsonResponse(w, snapshots)
}

func (h *Handler) handleSnapshot(w http.ResponseWriter, r *http.Request) {
	snapshotID := r.URL.Path[len("/api/v1/vm-snapshots/"):]
	if snapshotID == "" {
		http.Error(w, "Snapshot ID required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getSnapshot(w, r, snapshotID)
	case http.MethodPost:
		h.snapshotAction(w, r, snapshotID)
	case http.MethodDelete:
		h.deleteSnapshot(w, r, snapshotID)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) getSnapshot(w http.ResponseWriter, r *http.Request, snapshotID string) {
	snapshot, err := h.snapshotManager.GetSnapshot(snapshotID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	h.jsonResponse(w, snapshot)
}

func (h *Handler) snapshotAction(w http.ResponseWriter, r *http.Request, snapshotID string) {
	var action struct {
		Action string `json:"action"`
	}

	if err := json.NewDecoder(r.Body).Decode(&action); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	switch action.Action {
	case "restore":
		err := h.snapshotManager.RestoreSnapshot(r.Context(), snapshotID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		h.jsonResponse(w, map[string]string{"status": "restoring"})
	default:
		http.Error(w, "Unknown action: "+action.Action, http.StatusBadRequest)
	}
}

func (h *Handler) deleteSnapshot(w http.ResponseWriter, r *http.Request, snapshotID string) {
	err := h.snapshotManager.DeleteSnapshot(r.Context(), snapshotID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, map[string]string{"status": "success"})
}

// 模板管理

func (h *Handler) handleListTemplates(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	templates := h.manager.ListTemplates()
	h.jsonResponse(w, templates)
}

// 硬件设备

func (h *Handler) handleUSBDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	devices, err := h.manager.ListUSBDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, devices)
}

func (h *Handler) handlePCIDevices(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	devices, err := h.manager.ListPCIDevices()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	h.jsonResponse(w, devices)
}

// 辅助函数

func (h *Handler) jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(data); err != nil {
		http.Error(w, "encode error", http.StatusInternalServerError)
		return
	}
}

// CreateVMRequest 创建 VM 请求
type CreateVMRequest struct {
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Type        string            `json:"type"`
	CPU         int               `json:"cpu"`
	Memory      int               `json:"memory"`
	DiskSize    int               `json:"diskSize"`
	Network     string            `json:"network"`
	ISOPath     string            `json:"isoPath"`
	VNCEnabled  bool              `json:"vncEnabled"`
	USBDevices  []string          `json:"usbDevices"`
	PCIDevices  []string          `json:"pciDevices"`
	Tags        map[string]string `json:"tags"`
}

// handleCreateVM 创建 VM（单独的处理函数）
func (h *Handler) handleCreateVM(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateVMRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	vmType := VMTypeLinux
	switch req.Type {
	case "windows":
		vmType = VMTypeWindows
	case "other":
		vmType = VMTypeOther
	}

	config := VMConfig{
		Name:        req.Name,
		Description: req.Description,
		Type:        vmType,
		CPU:         req.CPU,
		Memory:      uint64(req.Memory),
		DiskSize:    uint64(req.DiskSize),
		Network:     req.Network,
		ISOPath:     req.ISOPath,
		VNCEnabled:  req.VNCEnabled,
		USBDevices:  req.USBDevices,
		PCIDevices:  req.PCIDevices,
		Tags:        req.Tags,
	}

	vm, err := h.manager.CreateVM(r.Context(), config)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	h.jsonResponse(w, vm)
}
