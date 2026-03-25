package advanced

import (
	"encoding/json"
	"net/http"
	"time"
)

// APIHandlers API处理器.
type APIHandlers struct {
	manager *Manager
}

// NewAPIHandlers 创建API处理器.
func NewAPIHandlers(manager *Manager) *APIHandlers {
	return &APIHandlers{
		manager: manager,
	}
}

// RegisterRoutes 注册路由.
func (h *APIHandlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/backup/advanced/config", h.handleConfig)
	mux.HandleFunc("/api/backup/advanced/create", h.handleCreate)
	mux.HandleFunc("/api/backup/advanced/restore", h.handleRestore)
	mux.HandleFunc("/api/backup/advanced/verify", h.handleVerify)
	mux.HandleFunc("/api/backup/advanced/list", h.handleList)
	mux.HandleFunc("/api/backup/advanced/progress", h.handleProgress)
	mux.HandleFunc("/api/backup/advanced/compression", h.handleCompressionInfo)
	mux.HandleFunc("/api/backup/advanced/encryption", h.handleEncryptionInfo)
}

// handleConfig 处理配置请求.
func (h *APIHandlers) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := h.manager.GetConfig()
		jsonResponse(w, http.StatusOK, config)
	case http.MethodPut:
		var config BackupConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			jsonError(w, http.StatusBadRequest, "invalid request body")
			return
		}
		if err := h.manager.UpdateConfig(&config); err != nil {
			jsonError(w, http.StatusInternalServerError, err.Error())
			return
		}
		jsonResponse(w, http.StatusOK, map[string]string{"status": "updated"})
	default:
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
	}
}

// handleCreate 处理创建备份请求.
func (h *APIHandlers) handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		Type string `json:"type"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	backupType := TypeFull
	switch req.Type {
	case "incremental":
		backupType = TypeIncremental
	case "differential":
		backupType = TypeDifferential
	}

	record, err := h.manager.CreateBackup(r.Context(), backupType)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusCreated, record)
}

// handleRestore 处理恢复请求.
func (h *APIHandlers) handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		BackupID   string `json:"backupId"`
		TargetPath string `json:"targetPath"`
		Overwrite  bool   `json:"overwrite"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	record, err := h.manager.RestoreBackup(r.Context(), req.BackupID, req.TargetPath, req.Overwrite)
	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, record)
}

// handleVerify 处理验证请求.
func (h *APIHandlers) handleVerify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	var req struct {
		BackupID string `json:"backupId"`
		Quick    bool   `json:"quick"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	verifier := NewVerifier(h.manager)

	var result *VerificationResult
	var err error

	if req.Quick {
		result, err = verifier.QuickVerify(r.Context(), req.BackupID)
	} else {
		result, err = verifier.VerifyBackup(r.Context(), req.BackupID)
	}

	if err != nil {
		jsonError(w, http.StatusInternalServerError, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, result)
}

// handleList 处理列表请求.
func (h *APIHandlers) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	records := h.manager.ListRecords()
	jsonResponse(w, http.StatusOK, records)
}

// handleProgress 处理进度请求.
func (h *APIHandlers) handleProgress(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	backupID := r.URL.Query().Get("id")
	if backupID == "" {
		jsonError(w, http.StatusBadRequest, "missing backup id")
		return
	}

	progress, err := h.manager.GetProgress(backupID)
	if err != nil {
		jsonError(w, http.StatusNotFound, err.Error())
		return
	}

	jsonResponse(w, http.StatusOK, progress)
}

// handleCompressionInfo 处理压缩信息请求.
func (h *APIHandlers) handleCompressionInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	algorithms := SupportedCompressionAlgorithms()
	jsonResponse(w, http.StatusOK, algorithms)
}

// handleEncryptionInfo 处理加密信息请求.
func (h *APIHandlers) handleEncryptionInfo(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		jsonError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	info := map[string]interface{}{
		"supportedAlgorithms": []string{"AES-256-GCM", "AES-256-CBC"},
		"keySize":             32,
		"recommended":         "AES-256-GCM",
	}
	jsonResponse(w, http.StatusOK, info)
}

// ========== 响应辅助函数 ==========

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, status int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(map[string]string{
		"error":     message,
		"timestamp": time.Now().Format(time.RFC3339),
	})
}
