package writeonce

import (
	"encoding/json"
	"net/http"
)

// handleCreateFolder handles creating a WriteOnce folder.
func (m *Manager) handleCreateFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	folder, err := m.CreateFolder(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(folder)
}

// handleLockFolder handles locking a folder.
func (m *Manager) handleLockFolder(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req LockFolderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := m.LockFolder(req.FolderID, req.UserID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "locked"})
}

// handleAddFile handles adding a file to a folder.
func (m *Manager) handleAddFile(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req AddFileRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	file, err := m.AddFile(req)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(file)
}

// handleGetFolder handles getting a folder.
func (m *Manager) handleGetFolder(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("id")
	if folderID == "" {
		http.Error(w, "Folder ID required", http.StatusBadRequest)
		return
	}

	folder, err := m.GetFolder(folderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(folder)
}

// handleListFolders handles listing all folders.
func (m *Manager) handleListFolders(w http.ResponseWriter, r *http.Request) {
	folders := m.ListFolders()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"folders": folders,
		"total":   len(folders),
	})
}

// handleGetFiles handles getting files in a folder.
func (m *Manager) handleGetFiles(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder_id")
	if folderID == "" {
		http.Error(w, "Folder ID required", http.StatusBadRequest)
		return
	}

	files, err := m.GetFiles(folderID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"files": files,
		"total": len(files),
	})
}

// handlePreventDelete handles delete prevention check.
func (m *Manager) handlePreventDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FolderID string `json:"folder_id"`
		FileName string `json:"file_name"`
		UserID   string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := m.PreventDelete(req.FolderID, req.FileName, req.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "allowed"})
}

// handlePreventModify handles modify prevention check.
func (m *Manager) handlePreventModify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		FolderID string `json:"folder_id"`
		FileName string `json:"file_name"`
		UserID   string `json:"user_id"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	err := m.PreventModify(req.FolderID, req.FileName, req.UserID)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]string{"error": err.Error()})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "allowed"})
}

// handleGetAuditLog handles getting audit log for a folder.
func (m *Manager) handleGetAuditLog(w http.ResponseWriter, r *http.Request) {
	folderID := r.URL.Query().Get("folder_id")
	if folderID == "" {
		http.Error(w, "Folder ID required", http.StatusBadRequest)
		return
	}

	entries := m.GetAuditLog(folderID)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": entries,
		"total":   len(entries),
	})
}

// handleGetAllAuditLog handles getting all audit log entries.
func (m *Manager) handleGetAllAuditLog(w http.ResponseWriter, r *http.Request) {
	entries := m.GetAllAuditLog()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"entries": entries,
		"total":   len(entries),
	})
}

// handleCheckExpiry handles checking and updating expired folders.
func (m *Manager) handleCheckExpiry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	expired := m.CheckExpiry()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"expired_count": expired,
	})
}

// handleGetStats handles getting statistics.
func (m *Manager) handleGetStats(w http.ResponseWriter, r *http.Request) {
	stats := m.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// handleGetConfig handles getting config.
func (m *Manager) handleGetConfig(w http.ResponseWriter, r *http.Request) {
	config := m.GetConfig()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(config)
}

// handleUpdateConfig handles updating config.
func (m *Manager) handleUpdateConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var config WriteOnceConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if err := m.UpdateConfig(config); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "updated"})
}

// RegisterRoutes registers HTTP routes for WriteOnce.
func (m *Manager) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/writeonce/folder", m.handleGetFolder)
	mux.HandleFunc("/api/writeonce/folders", m.handleListFolders)
	mux.HandleFunc("/api/writeonce/folder/create", m.handleCreateFolder)
	mux.HandleFunc("/api/writeonce/folder/lock", m.handleLockFolder)
	mux.HandleFunc("/api/writeonce/file", m.handleGetFiles)
	mux.HandleFunc("/api/writeonce/file/add", m.handleAddFile)
	mux.HandleFunc("/api/writeonce/file/prevent-delete", m.handlePreventDelete)
	mux.HandleFunc("/api/writeonce/file/prevent-modify", m.handlePreventModify)
	mux.HandleFunc("/api/writeonce/audit", m.handleGetAuditLog)
	mux.HandleFunc("/api/writeonce/audit/all", m.handleGetAllAuditLog)
	mux.HandleFunc("/api/writeonce/expiry/check", m.handleCheckExpiry)
	mux.HandleFunc("/api/writeonce/stats", m.handleGetStats)
	mux.HandleFunc("/api/writeonce/config", m.handleGetConfig)
	mux.HandleFunc("/api/writeonce/config/update", m.handleUpdateConfig)
}
