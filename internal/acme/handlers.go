package acme

import (
	"encoding/json"
	"net/http"
	"strings"
	"time"
)

// Handler handles ACME HTTP requests
type Handler struct {
	manager *Manager
}

// NewHandler creates a new ACME handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{
		manager: manager,
	}
}

// RegisterRoutes registers ACME API routes
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/acme/certificates", h.handleCertificates)
	mux.HandleFunc("/api/v1/acme/certificates/", h.handleCertificateByID)
	mux.HandleFunc("/api/v1/acme/config", h.handleConfig)
	mux.HandleFunc("/api/v1/acme/stats", h.handleStats)
	mux.HandleFunc("/api/v1/acme/check-expiry", h.handleCheckExpiry)
	mux.HandleFunc("/api/v1/acme/auto-renew", h.handleAutoRenew)
	mux.HandleFunc("/api/v1/acme/dns", h.handleDNS)
}

func (h *Handler) handleCertificates(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listCertificates(w, r)
	case http.MethodPost:
		h.requestCertificate(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) listCertificates(w http.ResponseWriter, r *http.Request) {
	certs := h.manager.ListCertificates()
	writeJSON(w, http.StatusOK, certs)
}

func (h *Handler) requestCertificate(w http.ResponseWriter, r *http.Request) {
	var req CreateCertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Domain == "" {
		writeError(w, http.StatusBadRequest, "domain is required")
		return
	}

	cert, err := h.manager.RequestCertificate(req.Domain)
	if err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, cert)
}

func (h *Handler) handleCertificateByID(w http.ResponseWriter, r *http.Request) {
	// Extract certificate ID from path: /api/v1/acme/certificates/{id}
	path := r.URL.Path
	prefix := "/api/v1/acme/certificates/"
	if !strings.HasPrefix(path, prefix) {
		writeError(w, http.StatusBadRequest, "invalid path")
		return
	}

	remaining := strings.TrimPrefix(path, prefix)
	parts := strings.Split(remaining, "/")
	id := parts[0]

	if id == "" {
		writeError(w, http.StatusBadRequest, "certificate ID is required")
		return
	}

	// Check for sub-resources
	if len(parts) > 1 {
		switch parts[1] {
		case "renew":
			if r.Method == http.MethodPost {
				h.renewCertificate(w, r, id)
				return
			}
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		case "revoke":
			if r.Method == http.MethodPost {
				h.revokeCertificate(w, r, id)
				return
			}
			writeError(w, http.StatusMethodNotAllowed, "method not allowed")
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		h.getCertificate(w, r, id)
	case http.MethodDelete:
		h.revokeCertificate(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) getCertificate(w http.ResponseWriter, r *http.Request, id string) {
	// Get by ID - iterate through certs
	certs := h.manager.ListCertificates()
	for _, cert := range certs {
		if cert.ID == id {
			writeJSON(w, http.StatusOK, cert)
			return
		}
	}
	writeError(w, http.StatusNotFound, "certificate not found: "+id)
}

func (h *Handler) renewCertificate(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.manager.RenewCertificate(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}

	// Get updated cert
	certs := h.manager.ListCertificates()
	for _, cert := range certs {
		if cert.ID == id {
			writeJSON(w, http.StatusOK, cert)
			return
		}
	}
	writeError(w, http.StatusInternalServerError, "certificate not found after renewal")
}

func (h *Handler) revokeCertificate(w http.ResponseWriter, r *http.Request, id string) {
	if err := h.manager.RevokeCertificate(id); err != nil {
		if strings.Contains(err.Error(), "not found") {
			writeError(w, http.StatusNotFound, err.Error())
		} else {
			writeError(w, http.StatusBadRequest, err.Error())
		}
		return
	}
	writeJSON(w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.getConfig(w, r)
	case http.MethodPut:
		h.updateConfig(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

func (h *Handler) getConfig(w http.ResponseWriter, r *http.Request) {
	config := h.manager.GetConfig()
	writeJSON(w, http.StatusOK, config)
}

func (h *Handler) updateConfig(w http.ResponseWriter, r *http.Request) {
	var config ACMEConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if err := h.manager.UpdateConfig(config); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, h.manager.GetConfig())
}

func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	certs := h.manager.ListCertificates()
	stats := CertStats{
		TotalCerts: len(certs),
	}

	threshold := time.Now().Add(30 * 24 * time.Hour)
	for _, c := range certs {
		if c.Status == "active" {
			stats.ActiveCerts++
			if c.NotAfter.Before(threshold) {
				stats.ExpiringSoon++
			}
		}
		if c.AutoRenew {
			stats.AutoRenewEnabled++
		}
	}

	writeJSON(w, http.StatusOK, stats)
}

func (h *Handler) handleCheckExpiry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	expiring := h.manager.CheckExpiry()
	writeJSON(w, http.StatusOK, expiring)
}

func (h *Handler) handleAutoRenew(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	if err := h.manager.AutoRenew(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, SuccessResponse{Success: true})
}

func (h *Handler) handleDNS(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Provider    string `json:"provider"`
		Credentials string `json:"credentials"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	if req.Provider == "" {
		writeError(w, http.StatusBadRequest, "provider is required")
		return
	}

	if err := h.manager.ConfigureDNS(req.Provider, req.Credentials); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, SuccessResponse{Success: true})
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
