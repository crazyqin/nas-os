package quantumsafevault

import (
	"encoding/json"
	"net/http"
)

// Handler HTTP处理器
type Handler struct {
	service *Service
}

// NewHandler 创建处理器
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/quantum/keys", h.handleKeys)
	mux.HandleFunc("/api/v1/quantum/key", h.handleKey)
	mux.HandleFunc("/api/v1/quantum/encrypt", h.handleEncrypt)
	mux.HandleFunc("/api/v1/quantum/decrypt", h.handleDecrypt)
	mux.HandleFunc("/api/v1/quantum/sign", h.handleSign)
	mux.HandleFunc("/api/v1/quantum/verify", h.handleVerify)
	mux.HandleFunc("/api/v1/quantum/audit", h.handleAudit)
}

// handleKeys 处理密钥列表
func (h *Handler) handleKeys(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		keyType := KeyType(r.URL.Query().Get("type"))
		algorithm := Algorithm(r.URL.Query().Get("algorithm"))
		keys := h.service.ListKeys(keyType, algorithm)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"keys":  keys,
			"total": len(keys),
		})

	case http.MethodPost:
		var req struct {
			Algorithm Algorithm     `json:"algorithm"`
			KeyType   KeyType       `json:"key_type"`
			Security  SecurityLevel `json:"security"`
			Tags      []string      `json:"tags"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}
		key, err := h.service.GenerateKeyPair(req.Algorithm, req.KeyType, req.Security, req.Tags)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		json.NewEncoder(w).Encode(key)

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleKey 处理单个密钥
func (h *Handler) handleKey(w http.ResponseWriter, r *http.Request) {
	id := r.URL.Query().Get("id")
	if id == "" {
		http.Error(w, "Missing key id", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case http.MethodGet:
		key, err := h.service.GetKey(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(key)

	case http.MethodDelete:
		if err := h.service.DeleteKey(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "deleted"})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleEncrypt 处理加密请求
func (h *Handler) handleEncrypt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		KeyID    string            `json:"key_id"`
		Data     []byte            `json:"data"`
		Metadata map[string]string `json:"metadata"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	encrypted, err := h.service.Encrypt(req.KeyID, req.Data, req.Metadata)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(encrypted)
}

// handleDecrypt 处理解密请求
func (h *Handler) handleDecrypt(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		EncryptedID string `json:"encrypted_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	plaintext, err := h.service.Decrypt(req.EncryptedID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/octet-stream")
	w.Write(plaintext)
}

// handleSign 处理签名请求
func (h *Handler) handleSign(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		KeyID string `json:"key_id"`
		Data  []byte `json:"data"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	signature, err := h.service.Sign(req.KeyID, req.Data)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(signature)
}

// handleVerify 处理验证请求
func (h *Handler) handleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		SignatureID string `json:"signature_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	verified, err := h.service.Verify(req.SignatureID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]bool{"verified": verified})
}

// handleAudit 处理审计日志请求
func (h *Handler) handleAudit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	action := r.URL.Query().Get("action")
	limit := 100

	logs := h.service.GetAuditLog(action, limit)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"logs":  logs,
		"total": len(logs),
	})
}
