// Package unifiedbackup 提供统一备份管理功能
package unifiedbackup

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handlers 统一备份 HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{
		manager: mgr,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/unifiedbackup/tasks", h.handleTasks)
	mux.HandleFunc("/api/unifiedbackup/tasks/", h.handleTaskByID)
	mux.HandleFunc("/api/unifiedbackup/tasks/run/", h.handleRunTask)
	mux.HandleFunc("/api/unifiedbackup/tasks/pause/", h.handlePauseTask)
	mux.HandleFunc("/api/unifiedbackup/tasks/resume/", h.handleResumeTask)
	mux.HandleFunc("/api/unifiedbackup/restorepoints", h.handleRestorePoints)
	mux.HandleFunc("/api/unifiedbackup/restore", h.handleRestore)
	mux.HandleFunc("/api/unifiedbackup/restore/jobs/", h.handleRestoreJob)
	mux.HandleFunc("/api/unifiedbackup/stats", h.handleStats)
}

// ========== 辅助方法 ==========

// writeJSON 写入JSON响应.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// writeSuccess 写入成功响应.
func writeSuccess(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: data})
}

// writeCreated 写入创建成功响应.
func writeCreated(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusCreated, APIResponse{Success: true, Data: data})
}

// writeError 写入错误响应.
func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, APIResponse{Success: false, Error: msg})
}

// extractIDFromPath 从路径中提取ID.
func extractIDFromPath(path, prefix string) string {
	id := strings.TrimPrefix(path, prefix)
	id = strings.TrimSuffix(id, "/")
	return id
}

// ========== 任务管理 Handlers ==========

// handleTasks 处理 /api/unifiedbackup/tasks.
func (h *Handlers) handleTasks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listTasks(w, r)
	case http.MethodPost:
		h.createTask(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

func (h *Handlers) listTasks(w http.ResponseWriter, _ *http.Request) {
	tasks := h.manager.ListTasks()
	writeSuccess(w, tasks)
}

func (h *Handlers) createTask(w http.ResponseWriter, r *http.Request) {
	var req CreateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	task, err := h.manager.CreateTask(&req)
	if err != nil {
		if err == ErrInvalidSource || err == ErrEncryptionKeyRequired {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeCreated(w, task)
}

func (h *Handlers) handleTaskByID(w http.ResponseWriter, r *http.Request) {
	id := extractIDFromPath(r.URL.Path, "/api/unifiedbackup/tasks/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少任务ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		h.getTask(w, r, id)
	case http.MethodPut:
		h.updateTask(w, r, id)
	case http.MethodDelete:
		h.deleteTask(w, r, id)
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

func (h *Handlers) getTask(w http.ResponseWriter, _ *http.Request, id string) {
	task, err := h.manager.GetTask(id)
	if err != nil {
		if err == ErrTaskNotFound {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeSuccess(w, task)
}

func (h *Handlers) updateTask(w http.ResponseWriter, r *http.Request, id string) {
	var req UpdateTaskRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	task, err := h.manager.UpdateTask(id, &req)
	if err != nil {
		if err == ErrTaskNotFound {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeSuccess(w, task)
}

func (h *Handlers) deleteTask(w http.ResponseWriter, _ *http.Request, id string) {
	err := h.manager.DeleteTask(id)
	if err != nil {
		switch err {
		case ErrTaskNotFound:
			writeError(w, http.StatusNotFound, err.Error())
		case ErrTaskRunning:
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeSuccess(w, nil)
}

// ========== 任务操作 Handlers ==========

// handleRunTask 处理运行任务.
func (h *Handlers) handleRunTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	id := extractIDFromPath(r.URL.Path, "/api/unifiedbackup/tasks/run/")
	err := h.manager.RunTask(id)
	if err != nil {
		switch err {
		case ErrTaskNotFound:
			writeError(w, http.StatusNotFound, err.Error())
		case ErrTaskRunning:
			writeError(w, http.StatusConflict, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeSuccess(w, map[string]string{"status": "started"})
}

// handlePauseTask 处理暂停任务.
func (h *Handlers) handlePauseTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	id := extractIDFromPath(r.URL.Path, "/api/unifiedbackup/tasks/pause/")
	err := h.manager.PauseTask(id)
	if err != nil {
		switch err {
		case ErrTaskNotFound:
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeSuccess(w, map[string]string{"status": "paused"})
}

// handleResumeTask 处理恢复任务.
func (h *Handlers) handleResumeTask(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	id := extractIDFromPath(r.URL.Path, "/api/unifiedbackup/tasks/resume/")
	err := h.manager.ResumeTask(id)
	if err != nil {
		switch err {
		case ErrTaskNotFound:
			writeError(w, http.StatusNotFound, err.Error())
		default:
			writeError(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	writeSuccess(w, map[string]string{"status": "resumed"})
}

// ========== 恢复点 Handlers ==========

// handleRestorePoints 处理 /api/unifiedbackup/restorepoints.
func (h *Handlers) handleRestorePoints(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	taskID := r.URL.Query().Get("task_id")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "缺少task_id参数")
		return
	}

	points, err := h.manager.GetRestorePoints(taskID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeSuccess(w, points)
}

// ========== 恢复任务 Handlers ==========

// handleRestore 处理 /api/unifiedbackup/restore.
func (h *Handlers) handleRestore(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	var req RestoreRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求参数错误: "+err.Error())
		return
	}

	if req.RestorePointID == "" {
		writeError(w, http.StatusBadRequest, "缺少恢复点ID")
		return
	}

	job, err := h.manager.Restore(&req)
	if err != nil {
		if err == ErrRestorePointNotFound {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeCreated(w, job)
}

// handleRestoreJob 处理 /api/unifiedbackup/restore/jobs/{id}.
func (h *Handlers) handleRestoreJob(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	id := extractIDFromPath(r.URL.Path, "/api/unifiedbackup/restore/jobs/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少任务ID")
		return
	}

	job, err := h.manager.GetRestoreJob(id)
	if err != nil {
		if err == ErrTaskNotFound {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeSuccess(w, job)
}

// ========== 统计 Handlers ==========

// handleStats 处理 /api/unifiedbackup/stats.
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	stats := h.manager.GetStorageStats()
	writeSuccess(w, stats)
}
