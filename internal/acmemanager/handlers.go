package acmemanager

import (
	"encoding/json"
	"net/http"
)

// ACMEHandler ACME证书HTTP处理器
type ACMEHandler struct {
	manager *ACMEManager
}

// NewACMEHandler 创建处理器
func NewACMEHandler(manager *ACMEManager) *ACMEHandler {
	return &ACMEHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *ACMEHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/acme/account/create", h.handleCreateAccount)
	mux.HandleFunc("/api/acme/account/get", h.handleGetAccount)
	mux.HandleFunc("/api/acme/accounts", h.handleListAccounts)
	mux.HandleFunc("/api/acme/certificate/request", h.handleRequestCertificate)
	mux.HandleFunc("/api/acme/certificate/get", h.handleGetCertificate)
	mux.HandleFunc("/api/acme/certificates", h.handleListCertificates)
	mux.HandleFunc("/api/acme/certificate/revoke", h.handleRevokeCertificate)
	mux.HandleFunc("/api/acme/certificate/renew", h.handleRenewCertificate)
	mux.HandleFunc("/api/acme/certificate/expiring", h.handleCheckExpiring)
	mux.HandleFunc("/api/acme/stats", h.handleGetStats)
}

// handleCreateAccount 处理创建账户请求
func (h *ACMEHandler) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	account, err := h.manager.CreateAccount(req.Email)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, account)
}

// handleGetAccount 处理获取账户请求
func (h *ACMEHandler) handleGetAccount(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	account, err := h.manager.GetAccount(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, account)
}

// handleListAccounts 处理列出账户请求
func (h *ACMEHandler) handleListAccounts(w http.ResponseWriter, r *http.Request) {
	accounts := h.manager.ListAccounts()
	respondJSON(w, accounts)
}

// handleRequestCertificate 处理请求证书请求
func (h *ACMEHandler) handleRequestCertificate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Domain string `json:"domain"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	cert, err := h.manager.RequestCertificate(req.Domain)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, cert)
}

// handleGetCertificate 处理获取证书请求
func (h *ACMEHandler) handleGetCertificate(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "ID is required", http.StatusBadRequest)
		return
	}

	cert, err := h.manager.GetCertificate(id)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, cert)
}

// handleListCertificates 处理列出证书请求
func (h *ACMEHandler) handleListCertificates(w http.ResponseWriter, r *http.Request) {
	certs := h.manager.ListCertificates()
	respondJSON(w, certs)
}

// handleRevokeCertificate 处理吊销证书请求
func (h *ACMEHandler) handleRevokeCertificate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.RevokeCertificate(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, map[string]string{"status": "revoked"})
}

// handleRenewCertificate 处理续期证书请求
func (h *ACMEHandler) handleRenewCertificate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := h.manager.RenewCertificate(req.ID); err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, map[string]string{"status": "renewing"})
}

// handleCheckExpiring 处理检查即将过期证书请求
func (h *ACMEHandler) handleCheckExpiring(w http.ResponseWriter, r *http.Request) {
	days := 30
	certs := h.manager.CheckExpiring(days)
	respondJSON(w, certs)
}

// handleGetStats 处理获取统计请求
func (h *ACMEHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()
	respondJSON(w, stats)
}

// respondJSON 响应JSON
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
