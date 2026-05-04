package vpnserver

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler provides HTTP handlers for VPN server management.
type Handler struct {
	service *Service
}

// NewHandler creates a new VPN server HTTP handler.
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes registers all VPN server routes on the given mux.
// Routes are registered under /api/v1/vpnserver/.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/vpnserver/status", h.handleStatus)
	mux.HandleFunc("/api/v1/vpnserver/wireguard", h.handleWGInterfaces)
	mux.HandleFunc("/api/v1/vpnserver/wireguard/", h.handleWGInterface)
	mux.HandleFunc("/api/v1/vpnserver/openvpn", h.handleOpenVPN)
	mux.HandleFunc("/api/v1/vpnserver/openvpn/", h.handleOpenVPNPath)
	mux.HandleFunc("/api/v1/vpnserver/users", h.handleUsers)
	mux.HandleFunc("/api/v1/vpnserver/users/", h.handleUserPath)
	mux.HandleFunc("/api/v1/vpnserver/sessions", h.handleSessions)
	mux.HandleFunc("/api/v1/vpnserver/dns", h.handleDNS)
	mux.HandleFunc("/api/v1/vpnserver/nat", h.handleNAT)
	mux.HandleFunc("/api/v1/vpnserver/config/", h.handleConfig)
	mux.HandleFunc("/api/v1/vpnserver/fail2ban/status", h.handleFail2BanStatus)
	mux.HandleFunc("/api/v1/vpnserver/fail2ban/unblock", h.handleFail2BanUnblock)
	mux.HandleFunc("/api/v1/vpnserver/fail2ban/whitelist", h.handleFail2BanWhiteList)
}

// writeJSON writes a JSON response with the given status code.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError writes an error JSON response.
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, APIResponse{Code: status, Message: message})
}

// writeSuccess writes a success JSON response.
func writeSuccess(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, APIResponse{Code: 0, Message: "ok", Data: data})
}

// ==================== Server Status ====================

// handleStatus handles GET /api/v1/vpnserver/status.
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	status := h.service.GetServerStatus()
	writeSuccess(w, status)
}

// ==================== WireGuard Handlers ====================

// handleWGInterfaces handles GET/POST /api/v1/vpnserver/wireguard.
func (h *Handler) handleWGInterfaces(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		ifaces := h.service.ListWGInterfaces()
		writeSuccess(w, ifaces)
	case http.MethodPost:
		var req CreateWGInterfaceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		iface, err := h.service.CreateWGInterface(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, APIResponse{Code: 0, Message: "created", Data: iface})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleWGInterface handles operations on a specific WireGuard interface.
// Routes: /api/v1/vpnserver/wireguard/{name} and sub-paths like peers, start, stop.
func (h *Handler) handleWGInterface(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/vpnserver/wireguard/")
	parts := strings.SplitN(path, "/", 2)
	if parts[0] == "" {
		writeError(w, http.StatusBadRequest, "interface name required")
		return
	}

	ifaceName := parts[0]
	subPath := ""
	if len(parts) > 1 {
		subPath = parts[1]
	}

	if subPath == "" {
		// /api/v1/vpnserver/wireguard/{name}
		switch r.Method {
		case http.MethodGet:
			iface, err := h.service.GetWGInterface(ifaceName)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeSuccess(w, iface)
		case http.MethodDelete:
			if err := h.service.DeleteWGInterface(ifaceName); err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeSuccess(w, nil)
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	// Sub-paths
	switch subPath {
	case "start":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := h.service.StartWGInterface(ifaceName); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeSuccess(w, "started")

	case "stop":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := h.service.StopWGInterface(ifaceName); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeSuccess(w, "stopped")

	case "peers":
		h.handleWGPeers(w, r, ifaceName)
	default:
		writeError(w, http.StatusNotFound, "not found")
	}
}

// handleWGPeers handles WireGuard peer operations.
func (h *Handler) handleWGPeers(w http.ResponseWriter, r *http.Request, ifaceName string) {
	switch r.Method {
	case http.MethodGet:
		peers, err := h.service.ListWGPeers(ifaceName)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeSuccess(w, peers)
	case http.MethodPost:
		var req AddWGPeerRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		peer, err := h.service.AddWGPeer(ifaceName, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, APIResponse{Code: 0, Message: "peer added", Data: peer})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ==================== OpenVPN Handlers ====================

// handleOpenVPN handles GET/PUT /api/v1/vpnserver/openvpn.
func (h *Handler) handleOpenVPN(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := h.service.GetOpenVPNConfig()
		if cfg == nil {
			writeSuccess(w, nil)
			return
		}
		writeSuccess(w, cfg)
	case http.MethodPut:
		var req UpdateOpenVPNRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		cfg := h.service.UpdateOpenVPNConfig(req)
		writeSuccess(w, cfg)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleOpenVPNPath handles sub-paths under /api/v1/vpnserver/openvpn/.
func (h *Handler) handleOpenVPNPath(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/vpnserver/openvpn/")
	switch path {
	case "start":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := h.service.StartOpenVPN(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeSuccess(w, "started")
	case "stop":
		if r.Method != http.MethodPost {
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
		if err := h.service.StopOpenVPN(); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeSuccess(w, "stopped")
	case "clients":
		h.handleOpenVPNClients(w, r)
	default:
		// Handle /openvpn/clients/{id}
		if strings.HasPrefix(path, "clients/") {
			clientID := strings.TrimPrefix(path, "clients/")
			h.handleOpenVPNClient(w, r, clientID)
			return
		}
		writeError(w, http.StatusNotFound, "not found")
	}
}

// handleOpenVPNClients handles GET/POST /api/v1/vpnserver/openvpn/clients.
func (h *Handler) handleOpenVPNClients(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		clients := h.service.ListOpenVPNClients()
		writeSuccess(w, clients)
	case http.MethodPost:
		var req struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		client, err := h.service.CreateOpenVPNClient(req.Name)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, APIResponse{Code: 0, Message: "client created", Data: client})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleOpenVPNClient handles GET/DELETE /api/v1/vpnserver/openvpn/clients/{id}.
func (h *Handler) handleOpenVPNClient(w http.ResponseWriter, r *http.Request, clientID string) {
	switch r.Method {
	case http.MethodGet:
		client, err := h.service.GetOpenVPNClient(clientID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeSuccess(w, client)
	case http.MethodDelete:
		if err := h.service.DeleteOpenVPNClient(clientID); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeSuccess(w, "deleted")
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ==================== User Handlers ====================

// handleUsers handles GET/POST /api/v1/vpnserver/users.
func (h *Handler) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		users := h.service.ListVPNUsers()
		writeSuccess(w, users)
	case http.MethodPost:
		var req CreateVPNUserRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		user, err := h.service.CreateVPNUser(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, APIResponse{Code: 0, Message: "user created", Data: user})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleUserPath handles sub-paths under /api/v1/vpnserver/users/{id}.
func (h *Handler) handleUserPath(w http.ResponseWriter, r *http.Request) {
	path := strings.TrimPrefix(r.URL.Path, "/api/v1/vpnserver/users/")
	parts := strings.SplitN(path, "/", 2)
	if parts[0] == "" {
		writeError(w, http.StatusBadRequest, "user ID required")
		return
	}

	userID := parts[0]
	subPath := ""
	if len(parts) > 1 {
		subPath = parts[1]
	}

	if subPath == "" {
		switch r.Method {
		case http.MethodGet:
			user, err := h.service.GetVPNUser(userID)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeSuccess(w, user)
		case http.MethodPut:
			var req CreateVPNUserRequest
			if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
				writeError(w, http.StatusBadRequest, "invalid request body")
				return
			}
			user, err := h.service.UpdateVPNUser(userID, req)
			if err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeSuccess(w, user)
		case http.MethodDelete:
			if err := h.service.DeleteVPNUser(userID); err != nil {
				writeError(w, http.StatusNotFound, err.Error())
				return
			}
			writeSuccess(w, "deleted")
		default:
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		}
		return
	}

	if subPath == "devices" {
		h.handleDevices(w, r, userID)
		return
	}

	// Handle /users/{id}/devices/{deviceId}
	if strings.HasPrefix(subPath, "devices/") {
		deviceID := strings.TrimPrefix(subPath, "devices/")
		h.handleDevice(w, r, userID, deviceID)
		return
	}

	writeError(w, http.StatusNotFound, "not found")
}

// handleDevices handles GET/POST /api/v1/vpnserver/users/{id}/devices.
func (h *Handler) handleDevices(w http.ResponseWriter, r *http.Request, userID string) {
	switch r.Method {
	case http.MethodGet:
		devices, err := h.service.ListDevices(userID)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeSuccess(w, devices)
	case http.MethodPost:
		var req AddDeviceRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		device, err := h.service.AddDevice(userID, req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, APIResponse{Code: 0, Message: "device added", Data: device})
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleDevice handles DELETE /api/v1/vpnserver/users/{id}/devices/{deviceId}.
func (h *Handler) handleDevice(w http.ResponseWriter, r *http.Request, userID, deviceID string) {
	if r.Method != http.MethodDelete {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	if err := h.service.DeleteDevice(userID, deviceID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeSuccess(w, "device deleted")
}

// ==================== Session Handlers ====================

// handleSessions handles GET /api/v1/vpnserver/sessions.
func (h *Handler) handleSessions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	sessions := h.service.GetActiveSessions()
	writeSuccess(w, sessions)
}

// ==================== DNS Handlers ====================

// handleDNS handles GET/PUT /api/v1/vpnserver/dns.
func (h *Handler) handleDNS(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := h.service.GetDNSConfig()
		writeSuccess(w, cfg)
	case http.MethodPut:
		var req UpdateDNSRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		cfg := h.service.UpdateDNSConfig(req)
		writeSuccess(w, cfg)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ==================== NAT Handlers ====================

// handleNAT handles GET/PUT /api/v1/vpnserver/nat.
func (h *Handler) handleNAT(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cfg := h.service.GetNATConfig()
		writeSuccess(w, cfg)
	case http.MethodPut:
		var req UpdateNATRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		cfg := h.service.UpdateNATConfig(req)
		writeSuccess(w, cfg)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// ==================== Config Generation Handlers ====================

// handleConfig handles GET /api/v1/vpnserver/config/wireguard/{ifaceName}/{publicKey}
// and GET /api/v1/vpnserver/config/openvpn/{clientId}.
func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	path := strings.TrimPrefix(r.URL.Path, "/api/v1/vpnserver/config/")
	parts := strings.SplitN(path, "/", 3)

	if len(parts) < 2 {
		writeError(w, http.StatusBadRequest, "invalid config path")
		return
	}

	protocol := parts[0]
	switch protocol {
	case "wireguard":
		if len(parts) < 3 {
			writeError(w, http.StatusBadRequest, "wireguard config requires interface name and public key")
			return
		}
		config, err := h.service.GenerateWGConfig(parts[1], parts[2])
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Disposition", "attachment; filename=\"wg0.conf\"")
		w.Write([]byte(config))
	case "openvpn":
		if len(parts) < 2 {
			writeError(w, http.StatusBadRequest, "openvpn config requires client ID")
			return
		}
		config, err := h.service.GenerateOpenVPNConfig(parts[1])
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Disposition", "attachment; filename=\"client.ovpn\"")
		w.Write([]byte(config))
	default:
		writeError(w, http.StatusBadRequest, "unknown protocol: "+protocol)
	}
}

// ==================== Fail2Ban Handlers ====================

// handleFail2BanStatus handles GET /api/v1/vpnserver/fail2ban/status.
func (h *Handler) handleFail2BanStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	status := h.service.fail2ban.GetStatus()
	writeSuccess(w, status)
}

// handleFail2BanUnblock handles POST /api/v1/vpnserver/fail2ban/unblock.
func (h *Handler) handleFail2BanUnblock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IP == "" {
		writeError(w, http.StatusBadRequest, "invalid request: ip required")
		return
	}
	if err := h.service.fail2ban.Unblock(req.IP); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeSuccess(w, nil)
}

// handleFail2BanWhiteList handles POST /api/v1/vpnserver/fail2ban/whitelist.
func (h *Handler) handleFail2BanWhiteList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	var req struct {
		IP     string `json:"ip"`
		Action string `json:"action"` // "add" or "remove"
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.IP == "" {
		writeError(w, http.StatusBadRequest, "invalid request: ip required")
		return
	}
	switch req.Action {
	case "add":
		h.service.fail2ban.AddToWhiteList(req.IP)
		writeSuccess(w, nil)
	case "remove":
		if err := h.service.fail2ban.RemoveFromWhiteList(req.IP); err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeSuccess(w, nil)
	default:
		writeError(w, http.StatusBadRequest, "action must be 'add' or 'remove'")
	}
}
