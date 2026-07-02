package aifilecleaner

import (
	"encoding/json"
	"net/http"
)

// Handlers HTTP处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// Scan 扫描文件.
func (h *Handlers) Scan(w http.ResponseWriter, r *http.Request) {
	var config ScanConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		// 使用默认配置
		config = *h.manager.config
	}

	if config.RootPath != "" {
		h.manager.config.RootPath = config.RootPath
	}
	if config.LargeFileThresholdMB > 0 {
		h.manager.config.LargeFileThresholdMB = config.LargeFileThresholdMB
	}
	if config.StaleDays > 0 {
		h.manager.config.StaleDays = config.StaleDays
	}

	result, err := h.manager.Scan()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// FindDuplicates 查找重复文件.
func (h *Handlers) FindDuplicates(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Paths []string `json:"paths"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	duplicates, err := h.manager.FindDuplicates(req.Paths)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(duplicates)
}

// GetScanResult 获取扫描结果.
func (h *Handlers) GetScanResult(w http.ResponseWriter, r *http.Request) {
	result, err := h.manager.GetScanResult()
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(result)
}

// CreateCleanTask 创建清理任务.
func (h *Handlers) CreateCleanTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Files []string   `json:"files"`
		Mode  DeleteMode `json:"mode"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	task, err := h.manager.CreateCleanTask(req.Files, req.Mode)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

// RunCleanTask 执行清理任务.
func (h *Handlers) RunCleanTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, "任务ID必填", http.StatusBadRequest)
		return
	}

	if err := h.manager.RunCleanTask(taskID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// GetTask 获取任务.
func (h *Handlers) GetTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, "任务ID必填", http.StatusBadRequest)
		return
	}

	task, err := h.manager.GetTask(taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(task)
}

// ListTasks 列出任务.
func (h *Handlers) ListTasks(w http.ResponseWriter, r *http.Request) {
	tasks := h.manager.ListTasks()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// CancelTask 取消任务.
func (h *Handlers) CancelTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, "任务ID必填", http.StatusBadRequest)
		return
	}

	if err := h.manager.CancelTask(taskID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
}
