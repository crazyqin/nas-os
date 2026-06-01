// Package docmanager 提供文档管理系统功能
package docmanager

import (
	"encoding/json"
	"net/http"
	"strconv"
	"strings"
)

// APIHandler HTTP API 处理器
type APIHandler struct {
	manager *Manager
}

// NewAPIHandler 创建新的API处理器
func NewAPIHandler(m *Manager) *APIHandler {
	return &APIHandler{manager: m}
}

// RegisterRoutes 注册HTTP路由
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/docmanager/documents", h.handleDocuments)
	mux.HandleFunc("/api/docmanager/documents/", h.handleDocumentByID)
	mux.HandleFunc("/api/docmanager/categories", h.handleCategories)
	mux.HandleFunc("/api/docmanager/tags", h.handleTags)
	mux.HandleFunc("/api/docmanager/search", h.handleSearch)
	mux.HandleFunc("/api/docmanager/stats", h.handleStats)
}

// Response 通用响应结构
type Response struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// handleDocuments 处理 /api/docmanager/documents
func (h *APIHandler) handleDocuments(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.listDocuments(w, r)
	case http.MethodPost:
		h.createDocument(w, r)
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

// handleDocumentByID 处理 /api/docmanager/documents/{id}
func (h *APIHandler) handleDocumentByID(w http.ResponseWriter, r *http.Request) {
	// 提取ID: /api/docmanager/documents/{id} 或 /api/docmanager/documents/{id}/...
	path := strings.TrimPrefix(r.URL.Path, "/api/docmanager/documents/")
	parts := strings.Split(path, "/")
	if len(parts) == 0 || parts[0] == "" {
		writeError(w, http.StatusBadRequest, "缺少文档ID")
		return
	}

	docID := parts[0]
	// 处理子资源: /api/docmanager/documents/{id}/tags, /api/docmanager/documents/{id}/category, /api/docmanager/documents/{id}/ocr
	if len(parts) > 1 {
		switch parts[1] {
		case "tags":
			h.handleDocumentTags(w, r, docID)
			return
		case "category":
			h.handleDocumentCategory(w, r, docID)
			return
		case "ocr":
			h.handleDocumentOCR(w, r, docID)
			return
		case "classify":
			h.handleDocumentClassify(w, r, docID)
			return
		}
	}

	switch r.Method {
	case http.MethodGet:
		h.getDocument(w, r, docID)
	case http.MethodPut:
		h.updateDocument(w, r, docID)
	case http.MethodDelete:
		h.deleteDocument(w, r, docID)
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

func (h *APIHandler) listDocuments(w http.ResponseWriter, r *http.Request) {
	page, _ := strconv.Atoi(r.URL.Query().Get("page"))
	pageSize, _ := strconv.Atoi(r.URL.Query().Get("page_size"))

	docs, total, err := h.manager.ListDocuments(page, pageSize)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{
		Success: true,
		Data: map[string]interface{}{
			"documents": docs,
			"total":     total,
			"page":      page,
			"page_size": pageSize,
		},
	})
}

func (h *APIHandler) createDocument(w http.ResponseWriter, r *http.Request) {
	var req CreateDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	doc, err := h.manager.CreateDocument(req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusCreated, Response{Success: true, Data: doc})
}

func (h *APIHandler) getDocument(w http.ResponseWriter, r *http.Request, docID string) {
	doc, err := h.manager.GetDocument(docID)
	if err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{Success: true, Data: doc})
}

func (h *APIHandler) updateDocument(w http.ResponseWriter, r *http.Request, docID string) {
	var req UpdateDocumentRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	doc, err := h.manager.UpdateDocument(docID, req)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{Success: true, Data: doc})
}

func (h *APIHandler) deleteDocument(w http.ResponseWriter, r *http.Request, docID string) {
	if err := h.manager.DeleteDocument(docID); err != nil {
		writeError(w, http.StatusNotFound, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{Success: true})
}

func (h *APIHandler) handleDocumentTags(w http.ResponseWriter, r *http.Request, docID string) {
	switch r.Method {
	case http.MethodPost:
		var req struct {
			TagID string `json:"tag_id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "请求格式错误")
			return
		}
		if err := h.manager.AddTag(docID, req.TagID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, Response{Success: true})
	case http.MethodDelete:
		tagID := r.URL.Query().Get("tag_id")
		if tagID == "" {
			writeError(w, http.StatusBadRequest, "缺少tag_id参数")
			return
		}
		if err := h.manager.RemoveTag(docID, tagID); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, Response{Success: true})
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

func (h *APIHandler) handleDocumentCategory(w http.ResponseWriter, r *http.Request, docID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	var req struct {
		CategoryID string `json:"category_id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}
	if err := h.manager.SetCategory(docID, req.CategoryID); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, Response{Success: true})
}

func (h *APIHandler) handleDocumentOCR(w http.ResponseWriter, r *http.Request, docID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	result, err := h.manager.ProcessOCR(docID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, Response{Success: true, Data: result})
}

func (h *APIHandler) handleDocumentClassify(w http.ResponseWriter, r *http.Request, docID string) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}
	category, err := h.manager.AutoClassify(docID)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, Response{Success: true, Data: map[string]string{"category": category}})
}

func (h *APIHandler) handleCategories(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		cats, err := h.manager.GetCategories()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, Response{Success: true, Data: cats})
	case http.MethodPost:
		var req CreateCategoryRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "请求格式错误")
			return
		}
		cat, err := h.manager.CreateCategory(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, Response{Success: true, Data: cat})
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

func (h *APIHandler) handleTags(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		tags, err := h.manager.GetTags()
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, Response{Success: true, Data: tags})
	case http.MethodPost:
		var req CreateTagRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeError(w, http.StatusBadRequest, "请求格式错误")
			return
		}
		tag, err := h.manager.CreateTag(req)
		if err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(w, http.StatusCreated, Response{Success: true, Data: tag})
	default:
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
	}
}

func (h *APIHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	var query SearchQuery
	if err := json.NewDecoder(r.Body).Decode(&query); err != nil {
		writeError(w, http.StatusBadRequest, "请求格式错误")
		return
	}

	result, err := h.manager.SearchDocuments(query)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{Success: true, Data: result})
}

func (h *APIHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "不支持的请求方法")
		return
	}

	stats, err := h.manager.GetStats()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, Response{Success: true, Data: stats})
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

func writeError(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, Response{Success: false, Error: msg})
}
