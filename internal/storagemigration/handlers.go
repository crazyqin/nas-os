package storagemigration

import (
	"encoding/json"
	"net/http"
)

// Handler 存储迁移 HTTP 处理器。
type Handler struct {
	engine *Engine
}

// NewHandler 创建处理器。
func NewHandler(engine *Engine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册路由。
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/storage-migration/start", h.Start)
	mux.HandleFunc("GET /api/storage-migration/{taskId}", h.GetTask)
	mux.HandleFunc("GET /api/storage-migration/{taskId}/report", h.GetReport)
	mux.HandleFunc("POST /api/storage-migration/{taskId}/cancel", h.Cancel)
	mux.HandleFunc("GET /api/storage-migration/tasks", h.ListTasks)
	mux.HandleFunc("GET /api/storage-migration/sources", h.GetSources)
}

// Start 启动迁移。
func (h *Handler) Start(w http.ResponseWriter, r *http.Request) {
	var cfg MigrationConfig
	if err := json.NewDecoder(r.Body).Decode(&cfg); err != nil {
		http.Error(w, "无效的请求体: "+err.Error(), http.StatusBadRequest)
		return
	}

	task, err := h.engine.Start(cfg)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	writeJSON(w, http.StatusCreated, task)
}

// GetTask 获取任务状态。
func (h *Handler) GetTask(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskId")
	task, err := h.engine.GetTask(taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotFound)
		return
	}
	writeJSON(w, http.StatusOK, task)
}

// GetReport 获取迁移报告。
func (h *Handler) GetReport(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskId")
	report, err := h.engine.GetReport(taskID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

// Cancel 取消迁移。
func (h *Handler) Cancel(w http.ResponseWriter, r *http.Request) {
	taskID := r.PathValue("taskId")
	if err := h.engine.Cancel(taskID); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
}

// ListTasks 列出所有任务。
func (h *Handler) ListTasks(w http.ResponseWriter, r *http.Request) {
	tasks := h.engine.ListTasks()
	writeJSON(w, http.StatusOK, tasks)
}

// GetSources 获取支持的源系统。
func (h *Handler) GetSources(w http.ResponseWriter, r *http.Request) {
	sources := make([]map[string]string, 0, len(AllSources()))
	for _, s := range AllSources() {
		sources = append(sources, map[string]string{
			"id":   string(s),
			"name": sourceName(s),
		})
	}
	writeJSON(w, http.StatusOK, sources)
}

func sourceName(s SourceSystem) string {
	switch s {
	case SourceSynology:
		return "群晖 DSM"
	case SourceTrueNAS:
		return "TrueNAS"
	case SourceQNAP:
		return "威联通 QTS"
	case SourceUnraid:
		return "Unraid"
	case SourceGeneric:
		return "通用 (rsync/SCP)"
	default:
		return string(s)
	}
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}
