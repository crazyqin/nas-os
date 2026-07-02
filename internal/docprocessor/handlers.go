// Package docprocessor 提供智能文档处理功能
package docprocessor

import (
	"encoding/json"
	"io"
	"net/http"
	"strconv"
	"time"
)

// APIHandler HTTP API 处理器.
type APIHandler struct {
	processor *Processor
}

// NewAPIHandler 创建新的API处理器.
func NewAPIHandler(p *Processor) *APIHandler {
	return &APIHandler{processor: p}
}

// AnalyzeRequest 文档分析请求.
type AnalyzeRequest struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// AnalyzeResponse 文档分析响应.
type AnalyzeResponse struct {
	Success bool            `json:"success"`
	Result  *AnalysisResult `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// ClassifyRequest 文档分类请求.
type ClassifyRequest struct {
	Filename string `json:"filename"`
	Content  string `json:"content"`
}

// ClassifyResponse 文档分类响应.
type ClassifyResponse struct {
	Success bool            `json:"success"`
	Result  *ClassifyResult `json:"result,omitempty"`
	Error   string          `json:"error,omitempty"`
}

// SummarizeRequest 文档摘要请求.
type SummarizeRequest struct {
	Filename  string `json:"filename"`
	Content   string `json:"content"`
	MaxLength int    `json:"max_length,omitempty"`
}

// SummarizeResponse 文档摘要响应.
type SummarizeResponse struct {
	Success bool           `json:"success"`
	Result  *SummaryResult `json:"result,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// DiffRequest 文档对比请求.
type DiffRequest struct {
	Filename1 string `json:"filename1"`
	Content1  string `json:"content1"`
	Filename2 string `json:"filename2"`
	Content2  string `json:"content2"`
}

// DiffResponse 文档对比响应.
type DiffResponse struct {
	Success bool        `json:"success"`
	Result  *DiffResult `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// SearchResponse 搜索响应.
type SearchResponse struct {
	Success bool           `json:"success"`
	Results []SearchResult `json:"results,omitempty"`
	Error   string         `json:"error,omitempty"`
}

// ErrorResponse 错误响应.
type ErrorResponse struct {
	Success bool   `json:"success"`
	Error   string `json:"error"`
}

// RegisterRoutes 注册HTTP路由.
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/docs/analyze", h.handleAnalyze)
	mux.HandleFunc("/api/v1/docs/classify", h.handleClassify)
	mux.HandleFunc("/api/v1/docs/summarize", h.handleSummarize)
	mux.HandleFunc("/api/v1/docs/diff", h.handleDiff)
	mux.HandleFunc("/api/v1/docs/search", h.handleSearch)
}

// handleAnalyze 处理文档分析请求.
func (h *APIHandler) handleAnalyze(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req AnalyzeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "Content is required")
		return
	}

	// 创建文档对象
	doc := &Document{
		ID:        generateID(),
		Name:      req.Filename,
		Content:   req.Content,
		Type:      h.processor.DetectType(req.Filename, []byte(req.Content)),
		Size:      int64(len(req.Content)),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 分析文档
	result := h.processor.AnalyzeDocument(doc)

	writeJSON(w, http.StatusOK, AnalyzeResponse{
		Success: true,
		Result:  result,
	})
}

// handleClassify 处理文档分类请求.
func (h *APIHandler) handleClassify(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req ClassifyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "Content is required")
		return
	}

	// 创建文档对象
	doc := &Document{
		ID:        generateID(),
		Name:      req.Filename,
		Content:   req.Content,
		Type:      h.processor.DetectType(req.Filename, []byte(req.Content)),
		Size:      int64(len(req.Content)),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 分类文档
	result := h.processor.ClassifyDocument(doc)

	writeJSON(w, http.StatusOK, ClassifyResponse{
		Success: true,
		Result:  result,
	})
}

// handleSummarize 处理文档摘要请求.
func (h *APIHandler) handleSummarize(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req SummarizeRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Content == "" {
		writeError(w, http.StatusBadRequest, "Content is required")
		return
	}

	// 设置默认长度
	if req.MaxLength <= 0 {
		req.MaxLength = 200
	}

	// 创建文档对象
	doc := &Document{
		ID:        generateID(),
		Name:      req.Filename,
		Content:   req.Content,
		Type:      h.processor.DetectType(req.Filename, []byte(req.Content)),
		Size:      int64(len(req.Content)),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 生成摘要
	result := h.processor.SummarizeDocument(doc, req.MaxLength)

	writeJSON(w, http.StatusOK, SummarizeResponse{
		Success: true,
		Result:  result,
	})
}

// handleDiff 处理文档对比请求.
func (h *APIHandler) handleDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var req DiffRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	if req.Content1 == "" || req.Content2 == "" {
		writeError(w, http.StatusBadRequest, "Both contents are required")
		return
	}

	// 创建文档对象
	doc1 := &Document{
		ID:        generateID(),
		Name:      req.Filename1,
		Content:   req.Content1,
		Type:      h.processor.DetectType(req.Filename1, []byte(req.Content1)),
		Size:      int64(len(req.Content1)),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	doc2 := &Document{
		ID:        generateID(),
		Name:      req.Filename2,
		Content:   req.Content2,
		Type:      h.processor.DetectType(req.Filename2, []byte(req.Content2)),
		Size:      int64(len(req.Content2)),
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 对比文档
	result := h.processor.DiffDocuments(doc1, doc2)

	writeJSON(w, http.StatusOK, DiffResponse{
		Success: true,
		Result:  result,
	})
}

// handleSearch 处理文档搜索请求.
func (h *APIHandler) handleSearch(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	query := r.URL.Query().Get("q")
	if query == "" {
		writeError(w, http.StatusBadRequest, "Query parameter 'q' is required")
		return
	}

	maxResults := 10
	if maxStr := r.URL.Query().Get("max"); maxStr != "" {
		if max, err := strconv.Atoi(maxStr); err == nil && max > 0 {
			maxResults = max
		}
	}

	// 搜索文档
	results := h.processor.SearchDocuments(query, maxResults)

	writeJSON(w, http.StatusOK, SearchResponse{
		Success: true,
		Results: results,
	})
}

// writeJSON 写入JSON响应.
func writeJSON(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应.
func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, ErrorResponse{
		Success: false,
		Error:   message,
	})
}

// generateID 生成简单的ID.
func generateID() string {
	return strconv.FormatInt(time.Now().UnixNano(), 36)
}

// readRequestBody 读取请求体（备用）.
func readRequestBody(r *http.Request) ([]byte, error) {
	return io.ReadAll(r.Body)
}
