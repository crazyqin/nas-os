// Package aifilesum 提供AI智能文件摘要生成功能
package aifilesum

import (
	"encoding/json"
	"log"
	"net/http"
)

// Handlers HTTP处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// SummarizeFile 单文件摘要
func (h *Handlers) SummarizeFile(w http.ResponseWriter, r *http.Request) {
	var req struct {
		FilePath string            `json:"file_path"`
		Options  *SummarizeOptions `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误: "+err.Error(), http.StatusBadRequest)
		return
	}

	if req.FilePath == "" {
		http.Error(w, "文件路径必填", http.StatusBadRequest)
		return
	}

	summary, err := h.manager.SummarizeFile(r.Context(), req.FilePath, req.Options)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err == ErrFileNotFound {
			statusCode = http.StatusNotFound
		} else if err == ErrUnsupportedFormat {
			statusCode = http.StatusBadRequest
		}
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// CreateBatchTask 创建批量任务
func (h *Handlers) CreateBatchTask(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Files   []string          `json:"files"`
		Options *SummarizeOptions `json:"options"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "请求参数错误: "+err.Error(), http.StatusBadRequest)
		return
	}

	if len(req.Files) == 0 {
		http.Error(w, "文件列表不能为空", http.StatusBadRequest)
		return
	}

	task, err := h.manager.CreateBatchTask(req.Files, req.Options)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if err == ErrQueueFull {
			statusCode = http.StatusTooManyRequests
		}
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusCreated)
	json.NewEncoder(w).Encode(task)
}

// RunBatchTask 执行批量任务
func (h *Handlers) RunBatchTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, "任务ID必填", http.StatusBadRequest)
		return
	}

	if err := h.manager.RunBatchTask(r.Context(), taskID); err != nil {
		statusCode := http.StatusInternalServerError
		if err == ErrTaskNotFound {
			statusCode = http.StatusNotFound
		} else if err == ErrTaskAlreadyRunning {
			statusCode = http.StatusConflict
		}
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "started"})
}

// GetTask 获取任务
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

// ListTasks 列出任务
func (h *Handlers) ListTasks(w http.ResponseWriter, r *http.Request) {
	tasks := h.manager.ListTasks()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tasks)
}

// CancelTask 取消任务
func (h *Handlers) CancelTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.URL.Query().Get("id")
	if taskID == "" {
		http.Error(w, "任务ID必填", http.StatusBadRequest)
		return
	}

	if err := h.manager.CancelTask(taskID); err != nil {
		statusCode := http.StatusInternalServerError
		if err == ErrTaskNotFound {
			statusCode = http.StatusNotFound
		} else if err == ErrTaskNotRunning {
			statusCode = http.StatusConflict
		}
		http.Error(w, err.Error(), statusCode)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cancelled"})
}

// GetSummary 获取摘要
func (h *Handlers) GetSummary(w http.ResponseWriter, r *http.Request) {
	summaryID := r.URL.Query().Get("id")
	if summaryID == "" {
		http.Error(w, "摘要ID必填", http.StatusBadRequest)
		return
	}

	summary, err := h.manager.GetSummary(summaryID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// GetSummaryByFile 根据文件获取摘要
func (h *Handlers) GetSummaryByFile(w http.ResponseWriter, r *http.Request) {
	filePath := r.URL.Query().Get("path")
	if filePath == "" {
		http.Error(w, "文件路径必填", http.StatusBadRequest)
		return
	}

	summary, err := h.manager.GetSummaryByFile(filePath)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(summary)
}

// SearchByTag 按标签搜索
func (h *Handlers) SearchByTag(w http.ResponseWriter, r *http.Request) {
	tag := r.URL.Query().Get("tag")
	if tag == "" {
		http.Error(w, "标签必填", http.StatusBadRequest)
		return
	}

	results := h.manager.SearchByTag(tag)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// SearchByKeyword 按关键词搜索
func (h *Handlers) SearchByKeyword(w http.ResponseWriter, r *http.Request) {
	keyword := r.URL.Query().Get("keyword")
	if keyword == "" {
		http.Error(w, "关键词必填", http.StatusBadRequest)
		return
	}

	results := h.manager.SearchByKeyword(keyword)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(results)
}

// GetStats 获取统计信息
func (h *Handlers) GetStats(w http.ResponseWriter, r *http.Request) {
	stats := h.manager.GetStats()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(stats)
}

// ClearCache 清除缓存
func (h *Handlers) ClearCache(w http.ResponseWriter, r *http.Request) {
	h.manager.ClearCache()
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{"status": "cleared"})
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	log.Println("📝 注册 AI 文件摘要路由...")

	// 单文件摘要
	mux.HandleFunc("POST /api/aifilesum/summarize", h.SummarizeFile)

	// 批量任务
	mux.HandleFunc("POST /api/aifilesum/batch", h.CreateBatchTask)
	mux.HandleFunc("POST /api/aifilesum/batch/run", h.RunBatchTask)
	mux.HandleFunc("GET /api/aifilesum/batch/status", h.GetTask)
	mux.HandleFunc("GET /api/aifilesum/batch/list", h.ListTasks)
	mux.HandleFunc("POST /api/aifilesum/batch/cancel", h.CancelTask)

	// 摘要查询
	mux.HandleFunc("GET /api/aifilesum/summary", h.GetSummary)
	mux.HandleFunc("GET /api/aifilesum/summary/file", h.GetSummaryByFile)

	// 搜索
	mux.HandleFunc("GET /api/aifilesum/search/tag", h.SearchByTag)
	mux.HandleFunc("GET /api/aifilesum/search/keyword", h.SearchByKeyword)

	// 统计
	mux.HandleFunc("GET /api/aifilesum/stats", h.GetStats)

	// 缓存管理
	mux.HandleFunc("POST /api/aifilesum/cache/clear", h.ClearCache)

	log.Println("✅ AI 文件摘要路由注册完成")
}
