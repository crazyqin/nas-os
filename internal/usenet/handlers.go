// Package usenet 提供 Usenet 下载管理 REST API 处理器
package usenet

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Handlers Usenet API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/usenet/servers", h.handleServers)
	mux.HandleFunc("/api/usenet/servers/", h.handleServerByID)
	mux.HandleFunc("/api/usenet/nzbs", h.handleNZBs)
	mux.HandleFunc("/api/usenet/nzbs/", h.handleNZBByID)
	mux.HandleFunc("/api/usenet/downloads", h.handleDownloads)
	mux.HandleFunc("/api/usenet/downloads/", h.handleDownloadByID)
	mux.HandleFunc("/api/usenet/queue", h.handleQueue)
	mux.HandleFunc("/api/usenet/indexers", h.handleIndexers)
	mux.HandleFunc("/api/usenet/indexers/", h.handleIndexerByID)
	mux.HandleFunc("/api/usenet/categories", h.handleCategories)
	mux.HandleFunc("/api/usenet/categories/", h.handleCategoryByID)
	mux.HandleFunc("/api/usenet/stats", h.handleStats)
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, code int, message string) {
	writeJSON(w, code, response{Code: 1, Message: message})
}

// writeSuccess 写入成功响应
func writeSuccess(w http.ResponseWriter, data interface{}) {
	writeJSON(w, http.StatusOK, response{Code: 0, Message: "success", Data: data})
}

// handleServers 处理 /api/usenet/servers
func (h *Handlers) handleServers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		servers := h.manager.ListServers()
		writeSuccess(w, servers)
	case http.MethodPost:
		var server Server
		if err := json.NewDecoder(r.Body).Decode(&server); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求数据: "+err.Error())
			return
		}
		result, err := h.manager.AddServer(&server)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, response{Code: 0, Message: "success", Data: result})
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleServerByID 处理 /api/usenet/servers/{id}
func (h *Handlers) handleServerByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/usenet/servers/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少服务器 ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		server, err := h.manager.GetServer(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeSuccess(w, server)
	case http.MethodPut:
		var server Server
		if err := json.NewDecoder(r.Body).Decode(&server); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求数据: "+err.Error())
			return
		}
		result, err := h.manager.UpdateServer(id, &server)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeSuccess(w, result)
	case http.MethodDelete:
		if err := h.manager.DeleteServer(id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeSuccess(w, nil)
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleNZBs 处理 /api/usenet/nzbs
func (h *Handlers) handleNZBs(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		status := NZBStatus(r.URL.Query().Get("status"))
		nzbs, err := h.manager.ListNZBs(status)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeSuccess(w, nzbs)
	case http.MethodPost:
		var nzb NZB
		if err := json.NewDecoder(r.Body).Decode(&nzb); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求数据: "+err.Error())
			return
		}
		result, err := h.manager.AddNZB(&nzb)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, response{Code: 0, Message: "success", Data: result})
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleNZBByID 处理 /api/usenet/nzbs/{id}
func (h *Handlers) handleNZBByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/usenet/nzbs/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少 NZB ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		nzb, err := h.manager.GetNZB(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeSuccess(w, nzb)
	case http.MethodDelete:
		if err := h.manager.DeleteNZB(id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeSuccess(w, nil)
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleDownloads 处理 /api/usenet/downloads
func (h *Handlers) handleDownloads(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	downloads, err := h.manager.ListDownloads()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeSuccess(w, downloads)
}

// handleDownloadByID 处理 /api/usenet/downloads/{id}
func (h *Handlers) handleDownloadByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/usenet/downloads/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少下载 ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		dl, err := h.manager.GetDownload(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeSuccess(w, dl)
	case http.MethodPost:
		// 处理操作：pause, resume, cancel
		action := r.URL.Query().Get("action")
		switch action {
		case "pause":
			if err := h.manager.PauseDownload(id); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		case "resume":
			if err := h.manager.ResumeDownload(id); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		case "cancel":
			if err := h.manager.CancelDownload(id); err != nil {
				writeError(w, http.StatusBadRequest, err.Error())
				return
			}
		default:
			writeError(w, http.StatusBadRequest, "无效的操作: "+action)
			return
		}
		writeSuccess(w, fmt.Sprintf("操作 %s 已执行", action))
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleQueue 处理 /api/usenet/queue
func (h *Handlers) handleQueue(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		queue, err := h.manager.GetQueue()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeSuccess(w, queue)
	case http.MethodPut:
		var ids []string
		if err := json.NewDecoder(r.Body).Decode(&ids); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求数据: "+err.Error())
			return
		}
		if err := h.manager.ReorderQueue(ids); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeSuccess(w, nil)
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleIndexers 处理 /api/usenet/indexers
func (h *Handlers) handleIndexers(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		indexers := h.manager.ListIndexers()
		writeSuccess(w, indexers)
	case http.MethodPost:
		var indexer Indexer
		if err := json.NewDecoder(r.Body).Decode(&indexer); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求数据: "+err.Error())
			return
		}
		result, err := h.manager.AddIndexer(&indexer)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, response{Code: 0, Message: "success", Data: result})
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleIndexerByID 处理 /api/usenet/indexers/{id}
func (h *Handlers) handleIndexerByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/usenet/indexers/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少索引器 ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		indexer, err := h.manager.GetIndexer(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeSuccess(w, indexer)
	case http.MethodDelete:
		if err := h.manager.DeleteIndexer(id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeSuccess(w, nil)
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleCategories 处理 /api/usenet/categories
func (h *Handlers) handleCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cats, err := h.manager.ListCategories()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeSuccess(w, cats)
	case http.MethodPost:
		var cat Category
		if err := json.NewDecoder(r.Body).Decode(&cat); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求数据: "+err.Error())
			return
		}
		result, err := h.manager.CreateCategory(&cat)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, response{Code: 0, Message: "success", Data: result})
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleCategoryByID 处理 /api/usenet/categories/{id}
func (h *Handlers) handleCategoryByID(w http.ResponseWriter, r *http.Request) {
	id := strings.TrimPrefix(r.URL.Path, "/api/usenet/categories/")
	if id == "" {
		writeError(w, http.StatusBadRequest, "缺少分类 ID")
		return
	}

	switch r.Method {
	case http.MethodGet:
		cat, err := h.manager.GetCategory(id)
		if err != nil {
			writeError(w, http.StatusNotFound, err.Error())
			return
		}
		writeSuccess(w, cat)
	case http.MethodDelete:
		if err := h.manager.DeleteCategory(id); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeSuccess(w, nil)
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleStats 处理 /api/usenet/stats
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	stats, err := h.manager.GetStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeSuccess(w, stats)
}
