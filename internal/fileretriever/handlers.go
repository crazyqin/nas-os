package fileretriever

import (
	"encoding/json"
	"fmt"
	"net/http"
)

type FileVersionHandler struct {
	manager *FileVersionManager
}

func NewFileVersionHandler(manager *FileVersionManager) *FileVersionHandler {
	return &FileVersionHandler{manager: manager}
}

func (h *FileVersionHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/fileversions/save", h.handleSaveVersion)
	mux.HandleFunc("/api/fileversions/list", h.handleGetVersions)
	mux.HandleFunc("/api/fileversion/get", h.handleGetVersion)
	mux.HandleFunc("/api/fileversion/current", h.handleGetCurrentVersion)
	mux.HandleFunc("/api/fileversion/restore", h.handleRestoreVersion)
	mux.HandleFunc("/api/fileversion/delete", h.handleDeleteVersion)
	mux.HandleFunc("/api/recycle/add", h.handleAddToRecycle)
	mux.HandleFunc("/api/recycle/list", h.handleListRecycle)
	mux.HandleFunc("/api/recycle/get", h.handleGetRecycleEntry)
	mux.HandleFunc("/api/recycle/restore", h.handleRestoreFromRecycle)
	mux.HandleFunc("/api/recycle/empty", h.handleEmptyRecycle)
	mux.HandleFunc("/api/recycle/cleanup", h.handleCleanupExpired)
	mux.HandleFunc("/api/fileversions/stats", h.handleGetStats)
}

func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func (h *FileVersionHandler) handleSaveVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var ver FileVersion
	if err := json.NewDecoder(r.Body).Decode(&ver); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	saved, err := h.manager.SaveVersion(&ver)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": saved})
}

func (h *FileVersionHandler) handleGetVersions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "缺少path参数"})
		return
	}
	versions, err := h.manager.GetVersions(filePath)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": versions})
}

func (h *FileVersionHandler) handleGetVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	filePath := r.URL.Query().Get("path")
	versionStr := r.URL.Query().Get("version")
	if filePath == "" || versionStr == "" {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "缺少参数"})
		return
	}
	var versionNum int
	if _, err := fmt.Sscanf(versionStr, "%d", &versionNum); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的版本号"})
		return
	}
	ver, err := h.manager.GetVersion(filePath, versionNum)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": ver})
}

func (h *FileVersionHandler) handleGetCurrentVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "缺少path参数"})
		return
	}
	ver, err := h.manager.GetCurrentVersion(filePath)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": ver})
}

func (h *FileVersionHandler) handleRestoreVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		FilePath string `json:"file_path"`
		Version  int    `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	restored, err := h.manager.RestoreVersion(req.FilePath, req.Version)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": restored})
}

func (h *FileVersionHandler) handleDeleteVersion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		FilePath string `json:"file_path"`
		Version  int    `json:"version"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	if err := h.manager.DeleteVersion(req.FilePath, req.Version); err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success"})
}

func (h *FileVersionHandler) handleAddToRecycle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var entry RecycleEntry
	if err := json.NewDecoder(r.Body).Decode(&entry); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	added, err := h.manager.AddToRecycle(&entry)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": added})
}

func (h *FileVersionHandler) handleListRecycle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.ListRecycle()})
}

func (h *FileVersionHandler) handleGetRecycleEntry(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "缺少id参数"})
		return
	}
	entry, err := h.manager.GetRecycleEntry(id)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 404, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": entry})
}

func (h *FileVersionHandler) handleRestoreFromRecycle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, map[string]interface{}{"code": 400, "message": "无效的请求体"})
		return
	}
	restored, err := h.manager.RestoreFromRecycle(req.ID)
	if err != nil {
		writeJSON(w, map[string]interface{}{"code": 500, "message": err.Error()})
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": restored})
}

func (h *FileVersionHandler) handleEmptyRecycle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	count := h.manager.EmptyRecycle()
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": map[string]int{"deleted": count}})
}

func (h *FileVersionHandler) handleCleanupExpired(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	count := h.manager.CleanupExpired()
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": map[string]int{"cleaned": count}})
}

func (h *FileVersionHandler) handleGetStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	writeJSON(w, map[string]interface{}{"code": 0, "message": "success", "data": h.manager.GetStats()})
}
