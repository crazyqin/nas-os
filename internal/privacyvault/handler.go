// Package privacyvault - handler.go 提供隐私保险箱的 HTTP API 接口。
package privacyvault

import (
	"encoding/json"
	"net/http"
)

// Handler 处理隐私保险箱的 HTTP 请求.
type Handler struct {
	engine *Engine
}

// NewHandler 创建隐私保险箱处理器.
func NewHandler(engine *Engine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册 HTTP 路由.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/vaults", h.handleVaults)
	mux.HandleFunc("/api/v1/vault", h.handleVault)
	mux.HandleFunc("/api/v1/vault/lock", h.handleLock)
	mux.HandleFunc("/api/v1/vault/unlock", h.handleUnlock)
	mux.HandleFunc("/api/v1/vault/stats", h.handleStats)
	mux.HandleFunc("/api/v1/vault/audit", h.handleAudit)
	mux.HandleFunc("/api/v1/vault/secret", h.handleSecret)
	mux.HandleFunc("/api/v1/vault/share", h.handleShare)
}

// handleVaults 处理保险库列表请求.
func (h *Handler) handleVaults(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	vaults := h.engine.ListVaults()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(vaults)
}

// handleVault 处理保险库 CRUD 操作.
func (h *Handler) handleVault(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id == "" {
			http.Error(w, "Missing id parameter", http.StatusBadRequest)
			return
		}
		vault, err := h.engine.GetVault(id)
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
		if err := h.engine.CreateVault(&req.Vault, req.Passphrase); err != nil {
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
		if err := h.engine.Destroy(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(map[string]string{"status": "destroyed"})
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleLock 锁定保险库.
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
	if err := h.engine.Lock(req.VaultID); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "locked"})
}

// handleUnlock 解锁保险库.
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
	if err := h.engine.Unlock(req.VaultID, req.Passphrase); err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	json.NewEncoder(w).Encode(map[string]string{"status": "unlocked"})
}

// handleStats 返回统计信息.
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	stats := h.engine.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleAudit 返回审计日志.
func (h *Handler) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	vaultID := r.URL.Query().Get("vault_id")
	entries := h.engine.GetAuditLog(vaultID, 100)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(entries)
}

// handleSecret 处理加密条目操作.
func (h *Handler) handleSecret(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		vaultID := r.URL.Query().Get("vault_id")
		secretID := r.URL.Query().Get("secret_id")
		if vaultID == "" {
			http.Error(w, "Missing vault_id", http.StatusBadRequest)
			return
		}
		if secretID != "" {
			secret, err := h.engine.GetSecret(vaultID, secretID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusNotFound)
				return
			}
			json.NewEncoder(w).Encode(secret)
		} else {
			secrets, err := h.engine.ListSecrets(vaultID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			json.NewEncoder(w).Encode(secrets)
		}
	case http.MethodPost:
		var req struct {
			VaultID string  `json:"vault_id"`
			Secret  *Secret `json:"secret"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		if err := h.engine.AddSecret(req.VaultID, req.Secret); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(req.Secret)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleShare 处理分享链接操作.
func (h *Handler) handleShare(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	token := r.URL.Query().Get("token")
	if token == "" {
		http.Error(w, "Missing token", http.StatusBadRequest)
		return
	}
	link, err := h.engine.AccessShareLink(token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	json.NewEncoder(w).Encode(link)
}
