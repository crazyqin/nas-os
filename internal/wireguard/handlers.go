package wireguard

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler handles WireGuard HTTP requests
type Handler struct {
	manager *Manager
}

// NewHandler creates a new WireGuard handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// RegisterRoutes registers WireGuard API routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/wireguard/interface", h.handleInterface)
	mux.HandleFunc("/api/v1/wireguard/peers", h.handlePeers)
	mux.HandleFunc("/api/v1/wireguard/peers/", h.handlePeerByID)
	mux.HandleFunc("/api/v1/wireguard/stats", h.handleStats)
	mux.HandleFunc("/api/v1/wireguard/generate-keypair", h.handleGenerateKeyPair)
}

func (h *Handler) handleInterface(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getInterface(w, r)
	case http.MethodPut:
		h.updateInterface(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) getInterface(w http.ResponseWriter, r *http.Request) {
	iface, err := h.manager.GetInterface()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, iface)
}

func (h *Handler) updateInterface(w http.ResponseWriter, r *http.Request) {
	var req InterfaceConfigRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if err := h.manager.ConfigureInterface(req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	
	iface, err := h.manager.GetInterface()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, iface)
}

func (h *Handler) handlePeers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listPeers(w, r)
	case http.MethodPost:
		h.createPeer(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) listPeers(w http.ResponseWriter, r *http.Request) {
	peers, err := h.manager.ListPeers()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, peers)
}

func (h *Handler) createPeer(w http.ResponseWriter, r *http.Request) {
	var req CreatePeerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if req.PublicKey == "" {
		writeError(w, http.StatusBadRequest, "public_key is required")
		return
	}
	if req.AllowedIPs == "" {
		writeError(w, http.StatusBadRequest, "allowed_ips is required")
		return
	}
	
	peer, err := h.manager.CreatePeer(req)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, peer)
}

func (h *Handler) handlePeerByID(w http.ResponseWriter, r *http.Request) {
	// Extract peer ID from path: /api/v1/wireguard/peers/{id}
	path := r.URL.Path
	prefix := "/api/v1/wireguard/peers/"
	if !strings.HasPrefix(path, prefix) {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}
	
	remaining := strings.TrimPrefix(path, prefix)
	parts := strings.Split(remaining, "/")
	id := parts[0]
	
	if id == "" {
		writeError(w, http.StatusBadRequest, "peer ID is required")
		return
	}
	
	// Check for sub-resource: /config
	if len(parts) > 1 && parts[1] == "config" {
		h.getPeerConfig(w, r, id)
		return
	}
	
	switch r.Method {
	case http.MethodGet:
		h.getPeer(w, r, id)
	case http.MethodPut:
		h.updatePeer(w, r, id)
	case http.MethodDelete:
		h.deletePeer(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) getPeer(w http.ResponseWriter, r *http.Request, id string) {
	peer, err := h.manager.GetPeer(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, peer)
}

func (h *Handler) updatePeer(w http.ResponseWriter, r *http.Request, id string) {
	var req UpdatePeerRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	
	if err := h.manager.UpdatePeer(id, req); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	
	peer, err := h.manager.GetPeer(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, peer)
}

func (h *Handler) deletePeer(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.manager.DeletePeer(id); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *Handler) getPeerConfig(w http.ResponseWriter, r *http.Request, id string) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	peer, err := h.manager.GetPeer(id)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}
	
	config, err := h.manager.GeneratePeerConfig(peer)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	writeJSON(w, http.StatusOK, PeerConfigResponse{Config: config})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	stats, err := h.manager.GetStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleGenerateKeyPair(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	
	publicKey, privateKey, err := h.manager.GenerateKeyPair()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	
	writeJSON(w, http.StatusOK, KeyPairResponse{
		PublicKey:  publicKey,
		PrivateKey: privateKey,
	})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, ErrorResponse{
		Error:   http.StatusText(status),
		Message: message,
	})
}
