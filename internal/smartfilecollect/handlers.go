// Package smartfilecollect - HTTP 处理器
// 处理文件收集相关的 HTTP 请求
package smartfilecollect

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// CollectHandler HTTP 处理器.
type CollectHandler struct {
	manager *CollectManager
}

// NewCollectHandler 创建处理器.
func NewCollectHandler(manager *CollectManager) *CollectHandler {
	return &CollectHandler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *CollectHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/collect/create", h.handleCreate)
	mux.HandleFunc("/api/v1/collect/list", h.handleList)
	mux.HandleFunc("/api/v1/collect/get", h.handleGet)
	mux.HandleFunc("/api/v1/collect/update", h.handleUpdate)
	mux.HandleFunc("/api/v1/collect/delete", h.handleDelete)
	mux.HandleFunc("/api/v1/collect/pause", h.handlePause)
	mux.HandleFunc("/api/v1/collect/resume", h.handleResume)
	mux.HandleFunc("/api/v1/collect/close", h.handleClose)
	mux.HandleFunc("/api/v1/collect/stats", h.handleStats)
	mux.HandleFunc("/api/v1/collect/submit", h.handleSubmit)
	mux.HandleFunc("/api/v1/collect/submission/get", h.handleGetSubmission)
	mux.HandleFunc("/api/v1/collect/submission/list", h.handleListSubmissions)
	mux.HandleFunc("/api/v1/collect/submission/status", h.handleUpdateSubmissionStatus)
}

// handleCreate 处理创建收集请求.
func (h *CollectHandler) handleCreate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CreateCollectRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	// 从请求头或参数获取创建者信息
	creatorID := r.Header.Get("X-User-ID")
	creatorName := r.Header.Get("X-User-Name")

	result, err := h.manager.CreateCollectRequest(&req, creatorID, creatorName)
	if err != nil {
		writeJSON(w, CollectResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, CollectResponse{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// handleList 处理列出收集请求.
func (h *CollectHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	creatorID := r.URL.Query().Get("creator_id")
	results := h.manager.ListCollectRequests(creatorID)

	writeJSON(w, CollectListResponse{
		Code:    0,
		Message: "success",
		Data:    results,
	})
}

// handleGet 处理获取收集请求.
func (h *CollectHandler) handleGet(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "缺少id参数",
		})
		return
	}

	result, err := h.manager.GetCollectRequest(id)
	if err != nil {
		writeJSON(w, CollectResponse{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, CollectResponse{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// handleUpdate 处理更新收集请求.
func (h *CollectHandler) handleUpdate(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID     string                 `json:"id"`
		Fields map[string]interface{} `json:"fields"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if req.ID == "" {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "缺少id参数",
		})
		return
	}

	if err := h.manager.UpdateCollectRequest(req.ID, req.Fields); err != nil {
		writeJSON(w, CollectResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, CollectResponse{
		Code:    0,
		Message: "success",
	})
}

// handleDelete 处理删除收集请求.
func (h *CollectHandler) handleDelete(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost && r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var id string
	if r.Method == http.MethodPost {
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, CollectResponse{
				Code:    400,
				Message: "无效的请求体",
			})
			return
		}
		id = req.ID
	} else {
		id = r.URL.Query().Get("id")
	}

	if id == "" {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "缺少id参数",
		})
		return
	}

	if err := h.manager.DeleteCollectRequest(id); err != nil {
		writeJSON(w, CollectResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, CollectResponse{
		Code:    0,
		Message: "success",
	})
}

// handlePause 处理暂停收集请求.
func (h *CollectHandler) handlePause(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if err := h.manager.PauseCollectRequest(req.ID); err != nil {
		writeJSON(w, CollectResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, CollectResponse{
		Code:    0,
		Message: "success",
	})
}

// handleResume 处理恢复收集请求.
func (h *CollectHandler) handleResume(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if err := h.manager.ResumeCollectRequest(req.ID); err != nil {
		writeJSON(w, CollectResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, CollectResponse{
		Code:    0,
		Message: "success",
	})
}

// handleClose 处理关闭收集请求.
func (h *CollectHandler) handleClose(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		ID string `json:"id"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if err := h.manager.CloseCollectRequest(req.ID); err != nil {
		writeJSON(w, CollectResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, CollectResponse{
		Code:    0,
		Message: "success",
	})
}

// handleStats 处理获取统计信息.
func (h *CollectHandler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	creatorID := r.URL.Query().Get("creator_id")
	stats := h.manager.GetStats(creatorID)

	writeJSON(w, StatsResponse{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// handleSubmit 处理文件提交.
func (h *CollectHandler) handleSubmit(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// 解析 multipart 表单
	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "解析表单失败",
		})
		return
	}

	collectID := r.FormValue("collect_id")
	if collectID == "" {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "缺少collect_id参数",
		})
		return
	}

	// 获取文件
	file, header, err := r.FormFile("file")
	if err != nil {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "获取文件失败",
		})
		return
	}
	defer file.Close()

	// 获取提交者信息
	submitterReq := &SubmitFileRequest{
		SubmitterName:  r.FormValue("submitter_name"),
		SubmitterEmail: r.FormValue("submitter_email"),
	}

	// 获取客户端IP
	clientIP := r.Header.Get("X-Forwarded-For")
	if clientIP == "" {
		clientIP = r.RemoteAddr
	} else {
		clientIP = strings.Split(clientIP, ",")[0]
	}

	// 提交文件
	result, err := h.manager.SubmitFile(collectID, file, header.Filename, submitterReq, clientIP)
	if err != nil {
		writeJSON(w, CollectResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, CollectResponse{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// handleGetSubmission 处理获取提交.
func (h *CollectHandler) handleGetSubmission(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	collectID := r.URL.Query().Get("collect_id")
	submissionID := r.URL.Query().Get("id")

	if collectID == "" || submissionID == "" {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "缺少参数",
		})
		return
	}

	result, err := h.manager.GetSubmission(collectID, submissionID)
	if err != nil {
		writeJSON(w, CollectResponse{
			Code:    404,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, CollectResponse{
		Code:    0,
		Message: "success",
		Data:    result,
	})
}

// handleListSubmissions 处理列出提交.
func (h *CollectHandler) handleListSubmissions(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	collectID := r.URL.Query().Get("collect_id")
	if collectID == "" {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "缺少collect_id参数",
		})
		return
	}

	results, err := h.manager.ListSubmissions(collectID)
	if err != nil {
		writeJSON(w, CollectResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, SubmissionListResponse{
		Code:    0,
		Message: "success",
		Data:    results,
	})
}

// handleUpdateSubmissionStatus 处理更新提交状态.
func (h *CollectHandler) handleUpdateSubmissionStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req struct {
		CollectID    string           `json:"collect_id"`
		SubmissionID string           `json:"submission_id"`
		Status       SubmissionStatus `json:"status"`
		Reason       string           `json:"reason"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "无效的请求体",
		})
		return
	}

	if req.CollectID == "" || req.SubmissionID == "" {
		writeJSON(w, CollectResponse{
			Code:    400,
			Message: "缺少参数",
		})
		return
	}

	if err := h.manager.UpdateSubmissionStatus(req.CollectID, req.SubmissionID, req.Status, req.Reason); err != nil {
		writeJSON(w, CollectResponse{
			Code:    500,
			Message: err.Error(),
		})
		return
	}

	writeJSON(w, CollectResponse{
		Code:    0,
		Message: "success",
	})
}

// writeJSON 写入JSON响应.
func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		fmt.Printf("JSON encode error: %v\n", err)
	}
}
