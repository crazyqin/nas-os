package privacyvault

import (
	"encoding/json"
	"net/http"
)

// Handler handles HTTP requests for privacy vault
type Handler struct {
	manager *Manager
}

// NewHandler creates a new privacy vault handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes registers the HTTP routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/vaults", h.handleVaults)
	mux.HandleFunc("/api/v1/vault", h.handleVault)
	mux.HandleFunc("/api/v1/vault/lock", h.handleLock)
	mux.HandleFunc("/api/v1/vault/unlock", h.handleUnlock)
	mux.HandleFunc("/api/v1/vault/stats", h.handleStats)
	mux.HandleFunc("/api/v1/vault/audit", h.handleAudit)
}

// handleVaults handles vault listing
func (h *Handler) handleVaults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	vaults := h.manager.ListVaults()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vaults)
}

// handleVault handles vault CRUD operations
func (h *Handler) handleVault(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}
		vault, err := h.manager.GetVault(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		json.NewEncoder(w).Encode(vault)
	case http.MethodPost:
		var req struct {
			Vault      Vault  `json:"vault"`
			Passphrase string `json:"passphrase"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.manager.CreateVault(&req.Vault, req.Passphrase); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(req.Vault)
	case http.MethodDelete:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}
		if err := h.manager.Destroy(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "destroyed"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleLock locks a vault
func (h *Handler) handleLock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		VaultID string `json:"vault_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.manager.Lock(req.VaultID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "locked"})
}

// handleUnlock unlocks a vault
func (h *Handler) handleUnlock(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		VaultID    string `json:"vault_id"`
		Passphrase string `json:"passphrase"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	if err := h.manager.Unlock(req.VaultID, req.Passphrase); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "unlocked"})
}

// handleStats returns vault statistics
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := h.manager.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleAudit returns audit log
func (h *Handler) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	vaultID := r.URL.Query().Get("vault_id")
	entries := h.manager.GetAuditLog(vaultID, 100)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}
