package storagemap

import (
	"encoding/json"
	"net/http"
)

// StorageMapHandler 存储地图HTTP处理器
type StorageMapHandler struct {
	manager *StorageMapManager
}

// NewStorageMapHandler 创建处理器
func NewStorageMapHandler(manager *StorageMapManager) *StorageMapHandler {
	return &StorageMapHandler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *StorageMapHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/storagemap/scan", h.handleScan)
	mux.HandleFunc("/api/storagemap/tree", h.handleGetTree)
	mux.HandleFunc("/api/storagemap/treemap", h.handleGetTreemap)
	mux.HandleFunc("/api/storagemap/large-files", h.handleGetLargeFiles)
	mux.HandleFunc("/api/storagemap/type-distribution", h.handleGetTypeDistribution)
	mux.HandleFunc("/api/storagemap/duplicates", h.handleGetDuplicates)
	mux.HandleFunc("/api/storagemap/summary", h.handleGetSummary)
	mux.HandleFunc("/api/storagemap/compare", h.handleCompare)
}

// handleScan 处理扫描请求
func (h *StorageMapHandler) handleScan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		Path string `json:"path"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	job, err := h.manager.StartScan(req.Path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	respondJSON(w, job)
}

// handleGetTree 处理获取树请求
func (h *StorageMapHandler) handleGetTree(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	tree, err := h.manager.GetStorageTree(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, tree)
}

// handleGetTreemap 处理获取树图请求
func (h *StorageMapHandler) handleGetTreemap(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	treemap, err := h.manager.GenerateTreemap(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, treemap)
}

// handleGetLargeFiles 处理获取大文件请求
func (h *StorageMapHandler) handleGetLargeFiles(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	limit := 10
	files, err := h.manager.GetLargeFiles(path, limit)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, files)
}

// handleGetTypeDistribution 处理获取类型分布请求
func (h *StorageMapHandler) handleGetTypeDistribution(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	dist, err := h.manager.GetTypeDistribution(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, dist)
}

// handleGetDuplicates 处理获取重复文件请求
func (h *StorageMapHandler) handleGetDuplicates(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	duplicates, err := h.manager.GetDuplicateFiles(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, duplicates)
}

// handleGetSummary 处理获取摘要请求
func (h *StorageMapHandler) handleGetSummary(w http.ResponseWriter, r *http.Request) {
	path := r.URL.Query().Get("path")
	if path == "" {
		http.Error(w, "Path is required", http.StatusBadRequest)
		return
	}

	summary, err := h.manager.GetUsageSummary(path)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, summary)
}

// handleCompare 处理比较请求
func (h *StorageMapHandler) handleCompare(w http.ResponseWriter, r *http.Request) {
	path1 := r.URL.Query().Get("path1")
	path2 := r.URL.Query().Get("path2")

	if path1 == "" || path2 == "" {
		http.Error(w, "Both paths are required", http.StatusBadRequest)
		return
	}

	diff, err := h.manager.CompareSnapshots(path1, path2)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	respondJSON(w, diff)
}

// respondJSON 响应JSON
func respondJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}
