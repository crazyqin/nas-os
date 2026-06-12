package vlan

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handlers 提供 VLAN HTTP API
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建 VLAN API 处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/vlans", h.handleVLANs)
	mux.HandleFunc("/api/v1/vlans/", h.handleVLANByID)
	mux.HandleFunc("/api/v1/vlans/stats/", h.handleVLANStats)
}

func (h *Handlers) handleVLANs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listVLANs(w, r)
	case http.MethodPost:
		h.createVLAN(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) handleVLANByID(w http.ResponseWriter, r *http.Request) {
	idStr := r.URL.Path[len("/api/v1/vlans/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "无效的 VLAN ID", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getVLAN(w, id)
	case http.MethodPut:
		h.updateVLAN(w, r, id)
	case http.MethodDelete:
		h.deleteVLAN(w, id)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handlers) handleVLANStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	idStr := r.URL.Path[len("/api/v1/vlans/stats/"):]
	id, err := strconv.Atoi(idStr)
	if err != nil {
		http.Error(w, "无效的 VLAN ID", http.StatusBadRequest)
		return
	}

	stats, err := h.manager.GetStats(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	writeJSON(w, stats)
}

func (h *Handlers) listVLANs(w http.ResponseWriter, _ *http.Request) {
	vlans := h.manager.List()
	writeJSON(w, vlans)
}

func (h *Handlers) getVLAN(w http.ResponseWriter, id int) {
	vlan, err := h.manager.Get(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, vlan)
}

type createVLANRequest struct {
	ParentIface string `json:"parent_iface"`
	VLANID      int    `json:"vlan_id"`
	IPAddr      string `json:"ip_addr"`
	Netmask     string `json:"netmask"`
	Gateway     string `json:"gateway"`
	MTU         int    `json:"mtu"`
}

func (h *Handlers) createVLAN(w http.ResponseWriter, r *http.Request) {
	var req createVLANRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	vlan, err := h.manager.Create(req.ParentIface, req.VLANID, req.IPAddr, req.Netmask, req.Gateway, req.MTU)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusCreated)
	writeJSON(w, vlan)
}

type updateVLANRequest struct {
	IPAddr  string   `json:"ip_addr"`
	Netmask string   `json:"netmask"`
	Gateway string   `json:"gateway"`
	MTU     int      `json:"mtu"`
	Tags    []string `json:"tags"`
}

func (h *Handlers) updateVLAN(w http.ResponseWriter, r *http.Request, id int) {
	var req updateVLANRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "无效的请求体", http.StatusBadRequest)
		return
	}

	vlan, err := h.manager.Update(id, req.IPAddr, req.Netmask, req.Gateway, req.MTU, req.Tags)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, vlan)
}

func (h *Handlers) deleteVLAN(w http.ResponseWriter, id int) {
	if err := h.manager.Delete(id); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
