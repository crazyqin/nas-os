package p2prelay

import (
	"encoding/json"
	"net/http"
)

// Handler provides HTTP handlers for P2P relay management.
type Handler struct {
	server *RelayServer
}

// NewHandler creates a new P2P relay HTTP handler.
func NewHandler(s *RelayServer) *Handler {
	return &Handler{server: s}
}

// RegisterRoutes registers P2P relay API routes.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/p2prelay/status", h.handleStatus)
	mux.HandleFunc("/api/v1/p2prelay/peers", h.handlePeers)
	mux.HandleFunc("/api/v1/p2prelay/connect", h.handleConnect)
	mux.HandleFunc("/api/v1/p2prelay/disconnect", h.handleDisconnect)
	mux.HandleFunc("/api/v1/p2prelay/stats", h.handleStats)
	mux.HandleFunc("/api/v1/p2prelay/config", h.handleConfig)
	mux.HandleFunc("/api/v1/p2prelay/start", h.handleStart)
	mux.HandleFunc("/api/v1/p2prelay/stop", h.handleStop)
}

func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"running":  h.server.IsRunning(),
		"nodeId":   h.server.config.NodeID,
		"nodeType": h.server.config.Type,
		"peers":    len(h.server.peers),
	})
}

func (h *Handler) handlePeers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	peers := h.server.GetPeers()
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"peers": peers,
		"total": len(peers),
	})
}

func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		PeerID string `json:"peerId"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.PeerID == "" {
		http.Error(w, "invalid request: peerId required", http.StatusBadRequest)
		return
	}
	peer, err := h.server.Connect(req.PeerID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, peer)
}

func (h *Handler) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	peerID := r.URL.Query().Get("peerId")
	if peerID == "" {
		http.Error(w, "peerId required", http.StatusBadRequest)
		return
	}
	h.server.Disconnect(peerID)
	writeJSON(w, http.StatusOK, map[string]string{"status": "disconnected"})
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, h.server.GetStats())
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, http.StatusOK, h.server.GetConfig())
}

func (h *Handler) handleStart(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if h.server.IsRunning() {
		writeJSON(w, http.StatusOK, map[string]string{"status": "already running"})
		return
	}
	if err := h.server.Start(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (h *Handler) handleStop(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := h.server.Stop(); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}
