package ztaccess

import (
	"encoding/json"
	"net/http"
)

// Handlers provides HTTP handlers for the ztaccess API
type Handlers struct {
	zt      *ZTAccess
	gateway *Gateway
	auth    *AuthManager
	session *SessionManager
	audit   *AuditManager
}

// NewHandlers creates new HTTP handlers
func NewHandlers(zt *ZTAccess) *Handlers {
	return &Handlers{
		zt:      zt,
		gateway: NewGateway(zt),
		auth:    NewAuthManager(zt),
		session: NewSessionManager(zt),
		audit:   NewAuditManager(zt),
	}
}

// RegisterRoutes registers the HTTP routes
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/ztaccess/login", h.handleLogin)
	mux.HandleFunc("/api/v1/ztaccess/logout", h.handleLogout)
	mux.HandleFunc("/api/v1/ztaccess/validate", h.handleValidate)
	mux.HandleFunc("/api/v1/ztaccess/authorize", h.handleAuthorize)
	mux.HandleFunc("/api/v1/ztaccess/sessions", h.handleSessions)
	mux.HandleFunc("/api/v1/ztaccess/audit", h.handleAudit)
	mux.HandleFunc("/api/v1/ztaccess/anomalies", h.handleAnomalies)
	mux.HandleFunc("/api/v1/ztaccess/users", h.handleUsers)
	mux.HandleFunc("/api/v1/ztaccess/policies", h.handlePolicies)
}

// handleLogin handles login requests
func (h *Handlers) handleLogin(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		Username string     `json:"username"`
		Password string     `json:"password"`
		Device   DeviceInfo `json:"device"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	session, err := h.gateway.Authenticate(request.Username, request.Password, request.Device)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	// Generate token
	token, err := h.auth.GenerateToken(session)
	if err != nil {
		http.Error(w, "Failed to generate token", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"session": session,
		"token":   token,
	})
}

// handleLogout handles logout requests
func (h *Handlers) handleLogout(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	err := h.gateway.RevokeSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "logged out"})
}

// handleValidate handles session validation requests
func (h *Handlers) handleValidate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	sessionID := r.Header.Get("X-Session-ID")
	if sessionID == "" {
		http.Error(w, "Missing session ID", http.StatusBadRequest)
		return
	}

	session, err := h.gateway.ValidateSession(sessionID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(session)
}

// handleAuthorize handles authorization requests
func (h *Handlers) handleAuthorize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var request struct {
		SessionID string `json:"session_id"`
		Resource  string `json:"resource"`
		Action    string `json:"action"`
	}

	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	allowed, err := h.gateway.Authorize(request.SessionID, request.Resource, request.Action)
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"allowed": allowed})
}

// handleSessions handles session management requests
func (h *Handlers) handleSessions(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		sessions := h.session.GetActiveSessions()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(sessions)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleAudit handles audit log requests
func (h *Handlers) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	limit := 100
	filters := make(map[string]string)

	if userID := r.URL.Query().Get("user_id"); userID != "" {
		filters["user_id"] = userID
	}
	if action := r.URL.Query().Get("action"); action != "" {
		filters["action"] = action
	}

	log := h.audit.GetAuditLog(limit, filters)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(log)
}

// handleAnomalies handles anomaly detection requests
func (h *Handlers) handleAnomalies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	anomalies := h.audit.GetAnomalies(100)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(anomalies)
}

// handleUsers handles user management requests
func (h *Handlers) handleUsers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.zt.mu.RLock()
		users := make([]*UserIdentity, 0, len(h.zt.users))
		for _, user := range h.zt.users {
			users = append(users, user)
		}
		h.zt.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(users)

	case http.MethodPost:
		var user UserIdentity
		if err := json.NewDecoder(r.Body).Decode(&user); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		h.zt.mu.Lock()
		h.zt.users[user.UserID] = &user
		h.zt.mu.Unlock()

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(user)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handlePolicies handles policy management requests
func (h *Handlers) handlePolicies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.zt.mu.RLock()
		policies := make([]*AccessPolicy, 0, len(h.zt.policies))
		for _, policy := range h.zt.policies {
			policies = append(policies, policy)
		}
		h.zt.mu.RUnlock()

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(policies)

	case http.MethodPost:
		var policy AccessPolicy
		if err := json.NewDecoder(r.Body).Decode(&policy); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		h.zt.mu.Lock()
		h.zt.policies[policy.PolicyID] = &policy
		h.zt.mu.Unlock()

		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(policy)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}
