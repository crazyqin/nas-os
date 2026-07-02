// Package spotlightfull - HTTP API handlers
// 提供 RESTful API 接口：搜索、索引管理、统计
package spotlightfull

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Handlers HTTP API 处理器.
type Handlers struct {
	engine  *SearchEngine
	indexer *FileIndexer
	mu      sync.RWMutex
	taskSeq int                     // 异步任务序号
	tasks   map[string]*RebuildTask // 异步任务表
}

// RebuildTask 异步重建任务.
type RebuildTask struct {
	ID        string    `json:"id"`
	Status    string    `json:"status"` // pending, running, completed, failed
	Message   string    `json:"message"`
	StartedAt time.Time `json:"started_at"`
	EndedAt   time.Time `json:"ended_at,omitempty"`
}

// NewHandlers 创建 API 处理器.
func NewHandlers(engine *SearchEngine, indexer *FileIndexer) *Handlers {
	return &Handlers{
		engine:  engine,
		indexer: indexer,
		tasks:   make(map[string]*RebuildTask),
	}
}

// RegisterRoutes 注册 HTTP 路由到标准库 ServeMux.
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/search", h.handleSearch)
	mux.HandleFunc("/index/rebuild", h.handleRebuild)
	mux.HandleFunc("/index/stats", h.handleStats)
	mux.HandleFunc("/index/task/", h.handleTaskStatus)
	mux.HandleFunc("/api/v1/suggest", h.handleSuggest)
	mux.HandleFunc("/api/v1/document/", h.handleDocument)
}

// handleSearch GET /api/v1/search?q=xxx&type=xxx
// 统一搜索接口，支持文件名、内容、元数据搜索.
func (h *Handlers) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 和 POST 方法")
		return
	}

	var filter SearchFilter

	if r.Method == http.MethodGet {
		// 从查询参数解析
		filter.Query = strings.TrimSpace(r.URL.Query().Get("q"))
		if filter.Query == "" {
			writeError(w, http.StatusBadRequest, "缺少必要参数 q")
			return
		}

		filter.FileTypes = parseFileTypes(r.URL.Query().Get("type"))
		filter.MinSize = parseInt64Ptr(r.URL.Query().Get("min_size"))
		filter.MaxSize = parseInt64Ptr(r.URL.Query().Get("max_size"))
		filter.After = parseTimePtr(r.URL.Query().Get("after"))
		filter.Before = parseTimePtr(r.URL.Query().Get("before"))
		filter.PathScope = r.URL.Query().Get("path")
		filter.Page = parseIntDefault(r.URL.Query().Get("page"), 1)
		filter.PageSize = parseIntDefault(r.URL.Query().Get("page_size"), 20)
		filter.SortBy = r.URL.Query().Get("sort_by")
		filter.SortOrder = r.URL.Query().Get("sort_order")
	} else {
		// POST: 从 JSON body 解析
		if err := json.NewDecoder(r.Body).Decode(&filter); err != nil {
			writeError(w, http.StatusBadRequest, "请求体解析失败: "+err.Error())
			return
		}
		if filter.Query == "" {
			writeError(w, http.StatusBadRequest, "缺少必要字段 query")
			return
		}
	}

	// 参数校验
	if filter.Page < 1 {
		filter.Page = 1
	}
	if filter.PageSize < 1 {
		filter.PageSize = 20
	}
	if filter.PageSize > 200 {
		filter.PageSize = 200
	}

	resp, err := h.engine.Search(&filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "搜索失败: "+err.Error())
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Code:    200,
		Message: "ok",
		Data:    resp,
	})
}

// handleRebuild POST /index/rebuild
// 重建索引，支持同步和异步模式.
func (h *Handlers) handleRebuild(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req RebuildRequest
	if r.Body != nil {
		json.NewDecoder(r.Body).Decode(&req)
	}

	if req.Async {
		// 异步模式
		h.mu.Lock()
		h.taskSeq++
		taskID := fmt.Sprintf("rebuild-%d-%d", time.Now().Unix(), h.taskSeq)
		task := &RebuildTask{
			ID:        taskID,
			Status:    "pending",
			Message:   "任务已排队",
			StartedAt: time.Now(),
		}
		h.tasks[taskID] = task
		h.mu.Unlock()

		go h.runRebuildTask(task)

		writeJSON(w, http.StatusAccepted, APIResponse{
			Code:    202,
			Message: "异步重建任务已启动",
			Data:    task,
		})
		return
	}

	// 同步模式
	start := time.Now()
	if err := h.engine.RebuildIndex(); err != nil {
		writeError(w, http.StatusInternalServerError, "重建索引失败: "+err.Error())
		return
	}

	// 从磁盘重新加载索引（如有）
	if err := h.engine.loadIndex(); err != nil {
		fmt.Printf("[spotlightfull] 重新加载索引失败: %v\n", err)
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Code:    200,
		Message: "索引重建完成",
		Data: &RebuildResponse{
			Status:  "completed",
			Message: fmt.Sprintf("重建完成，耗时 %s", time.Since(start)),
		},
	})
}

// handleStats GET /index/stats
// 获取索引统计信息.
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	stats := h.engine.GetStats()

	writeJSON(w, http.StatusOK, APIResponse{
		Code:    200,
		Message: "ok",
		Data:    stats,
	})
}

// handleTaskStatus GET /index/task/{taskId}
// 查询异步任务状态.
func (h *Handlers) handleTaskStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	taskID := strings.TrimPrefix(r.URL.Path, "/index/task/")
	if taskID == "" {
		writeError(w, http.StatusBadRequest, "缺少任务ID")
		return
	}

	h.mu.RLock()
	task, ok := h.tasks[taskID]
	h.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "任务不存在")
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Code:    200,
		Message: "ok",
		Data:    task,
	})
}

// handleSuggest GET /api/v1/suggest?q=xxx
// 搜索建议/自动补全.
func (h *Handlers) handleSuggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if query == "" {
		writeError(w, http.StatusBadRequest, "缺少参数 q")
		return
	}

	filter := &SearchFilter{
		Query:    query,
		Page:     1,
		PageSize: 5,
	}

	resp, err := h.engine.Search(filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "搜索失败: "+err.Error())
		return
	}

	suggestions := resp.Suggests
	if suggestions == nil {
		suggestions = []string{}
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Code:    200,
		Message: "ok",
		Data: map[string]interface{}{
			"suggestions": suggestions,
			"query":       query,
		},
	})
}

// handleDocument GET /api/v1/document?id=xxx
// 获取单个文档详情.
func (h *Handlers) handleDocument(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	docID := strings.TrimPrefix(r.URL.Path, "/api/v1/document/")
	if docID == "" {
		docID = r.URL.Query().Get("id")
	}
	if docID == "" {
		writeError(w, http.StatusBadRequest, "缺少文档ID")
		return
	}

	h.engine.index.mu.RLock()
	entry, ok := h.engine.index.docs[docID]
	h.engine.index.mu.RUnlock()

	if !ok {
		writeError(w, http.StatusNotFound, "文档不存在")
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Code:    200,
		Message: "ok",
		Data:    entry,
	})
}

// runRebuildTask 执行异步重建任务.
func (h *Handlers) runRebuildTask(task *RebuildTask) {
	h.mu.Lock()
	task.Status = "running"
	task.Message = "正在重建索引..."
	h.mu.Unlock()

	err := h.engine.RebuildIndex()

	h.mu.Lock()
	if err != nil {
		task.Status = "failed"
		task.Message = "重建失败: " + err.Error()
	} else {
		task.Status = "completed"
		task.Message = "索引重建完成"
	}
	task.EndedAt = time.Now()
	h.mu.Unlock()
}

// ---- 解析辅助函数 ----

// parseFileTypes 解析文件类型参数（逗号分隔）.
func parseFileTypes(s string) []FileType {
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	types := make([]FileType, 0, len(parts))
	validTypes := map[FileType]bool{
		FileTypeDocument: true,
		FileTypeImage:    true,
		FileTypeVideo:    true,
		FileTypeAudio:    true,
		FileTypeArchive:  true,
		FileTypeCode:     true,
		FileTypeOther:    true,
	}
	for _, p := range parts {
		ft := FileType(strings.TrimSpace(strings.ToLower(p)))
		if validTypes[ft] {
			types = append(types, ft)
		}
	}
	if len(types) == 0 {
		return nil
	}
	return types
}

// parseInt64Ptr 解析 int64 指针参数.
func parseInt64Ptr(s string) *int64 {
	if s == "" {
		return nil
	}
	v, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return nil
	}
	return &v
}

// parseTimePtr 解析时间指针参数（RFC3339 或日期格式）.
func parseTimePtr(s string) *time.Time {
	if s == "" {
		return nil
	}
	// 尝试 RFC3339
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return &t
	}
	// 尝试日期格式
	if t, err := time.Parse("2006-01-02", s); err == nil {
		return &t
	}
	return nil
}

// parseIntDefault 解析整数，失败时返回默认值.
func parseIntDefault(s string, def int) int {
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return def
	}
	return v
}

// writeJSON 写入 JSON 响应.
func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应.
func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, APIResponse{
		Code:    code,
		Message: message,
	})
}
