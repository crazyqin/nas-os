// Package composeinclude 提供 Docker Compose Include 支持。
package composeinclude

import (
	"encoding/json"
	"net/http"
	"strings"
)

// Handler HTTP 处理器
type Handler struct {
	manager *IncludeManager
}

// NewHandler 创建 HTTP 处理器
func NewHandler(manager *IncludeManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/docker/compose-include", h.ComposeInclude)
	mux.HandleFunc("/api/docker/compose-include/", h.ComposeIncludeByID)
}

// ComposeInclude 处理 POST /api/docker/compose-include（解析）
func (h *Handler) ComposeInclude(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method not allowed"))
		return
	}

	var req ParseRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse("invalid request body"))
		return
	}
	if req.Content == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("content is required"))
		return
	}

	result, err := h.manager.Parse(&req)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, errorResponse(err.Error()))
		return
	}

	writeJSON(w, http.StatusCreated, result)
}

// ComposeIncludeByID 处理 GET /api/docker/compose-include/:id（获取解析结果）
func (h *Handler) ComposeIncludeByID(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, errorResponse("method not allowed"))
		return
	}

	// 从路径中提取 ID
	path := r.URL.Path
	prefix := "/api/docker/compose-include/"
	if !strings.HasPrefix(path, prefix) {
		writeJSON(w, http.StatusBadRequest, errorResponse("invalid path"))
		return
	}
	id := strings.TrimPrefix(path, prefix)
	if id == "" {
		writeJSON(w, http.StatusBadRequest, errorResponse("id is required"))
		return
	}

	result, err := h.manager.GetResult(id)
	if err != nil {
		writeJSON(w, http.StatusNotFound, errorResponse(err.Error()))
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func errorResponse(msg string) map[string]string {
	return map[string]string{"error": msg}
}