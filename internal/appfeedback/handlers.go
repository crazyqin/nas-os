package appfeedback

import (
	"encoding/json"
	"net/http"
	"strconv"
)

// Handler HTTP 处理器
type Handler struct {
	service *Service
}

// NewHandler 创建处理器
func NewHandler(service *Service) *Handler {
	return &Handler{service: service}
}

// CreateFeedback 创建反馈
// POST /feedbacks
func (h *Handler) CreateFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "X-User-ID header is required")
		return
	}

	var req CreateFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	feedback, err := h.service.CreateFeedback(userID, req)
	if err != nil {
		if err == ErrInvalidRating {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to create feedback")
		return
	}

	writeJSON(w, http.StatusCreated, feedback)
}

// GetFeedback 获取反馈
// GET /feedbacks/{id}
func (h *Handler) GetFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	feedback, err := h.service.GetFeedback(id)
	if err != nil {
		if err == ErrFeedbackNotFound {
			writeError(w, http.StatusNotFound, "Feedback not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to get feedback")
		return
	}

	writeJSON(w, http.StatusOK, feedback)
}

// UpdateFeedback 更新反馈
// PUT /feedbacks/{id}
func (h *Handler) UpdateFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPut {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "X-User-ID header is required")
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	var req CreateFeedbackRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	feedback, err := h.service.UpdateFeedback(id, userID, req)
	if err != nil {
		if err == ErrFeedbackNotFound {
			writeError(w, http.StatusNotFound, "Feedback not found")
			return
		}
		if err.Error() == "unauthorized: can only update own feedback" {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		if err == ErrInvalidRating {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to update feedback")
		return
	}

	writeJSON(w, http.StatusOK, feedback)
}

// DeleteFeedback 删除反馈
// DELETE /feedbacks/{id}
func (h *Handler) DeleteFeedback(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "X-User-ID header is required")
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	err := h.service.DeleteFeedback(id, userID)
	if err != nil {
		if err == ErrFeedbackNotFound {
			writeError(w, http.StatusNotFound, "Feedback not found")
			return
		}
		if err.Error() == "unauthorized: can only delete own feedback" {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to delete feedback")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// ListFeedbacks 列出反馈
// GET /feedbacks?app_id=&version=&category=&sort=&page=&page_size=
func (h *Handler) ListFeedbacks(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	query := r.URL.Query()
	params := ListFeedbackParams{
		AppID:    query.Get("app_id"),
		Version:  query.Get("version"),
		Category: FeedbackCategory(query.Get("category")),
		Sort:     SortOrder(query.Get("sort")),
	}

	if pageStr := query.Get("page"); pageStr != "" {
		if page, err := strconv.Atoi(pageStr); err == nil {
			params.Page = page
		}
	}
	if pageSizeStr := query.Get("page_size"); pageSizeStr != "" {
		if pageSize, err := strconv.Atoi(pageSizeStr); err == nil {
			params.PageSize = pageSize
		}
	}

	result, err := h.service.ListFeedbacks(params)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list feedbacks")
		return
	}

	writeJSON(w, http.StatusOK, result)
}

// CreateReply 创建回复
// POST /feedbacks/{id}/replies
func (h *Handler) CreateReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "X-User-ID header is required")
		return
	}

	feedbackID := r.URL.Query().Get("feedback_id")
	if feedbackID == "" {
		writeError(w, http.StatusBadRequest, "feedback_id is required")
		return
	}

	var req CreateReplyRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	reply, err := h.service.CreateReply(feedbackID, userID, req)
	if err != nil {
		if err == ErrFeedbackNotFound {
			writeError(w, http.StatusNotFound, "Feedback not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to create reply")
		return
	}

	writeJSON(w, http.StatusCreated, reply)
}

// ListReplies 列出回复
// GET /feedbacks/{id}/replies
func (h *Handler) ListReplies(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	feedbackID := r.URL.Query().Get("feedback_id")
	if feedbackID == "" {
		writeError(w, http.StatusBadRequest, "feedback_id is required")
		return
	}

	replies, err := h.service.ListReplies(feedbackID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to list replies")
		return
	}

	writeJSON(w, http.StatusOK, replies)
}

// DeleteReply 删除回复
// DELETE /replies/{id}
func (h *Handler) DeleteReply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodDelete {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "X-User-ID header is required")
		return
	}

	id := r.URL.Query().Get("id")
	if id == "" {
		writeError(w, http.StatusBadRequest, "id is required")
		return
	}

	err := h.service.DeleteReply(id, userID)
	if err != nil {
		if err == ErrReplyNotFound {
			writeError(w, http.StatusNotFound, "Reply not found")
			return
		}
		if err.Error() == "unauthorized: can only delete own reply" {
			writeError(w, http.StatusForbidden, err.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to delete reply")
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// Vote 投票
// POST /feedbacks/{id}/vote
func (h *Handler) Vote(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "X-User-ID header is required")
		return
	}

	feedbackID := r.URL.Query().Get("feedback_id")
	if feedbackID == "" {
		writeError(w, http.StatusBadRequest, "feedback_id is required")
		return
	}

	var req CreateVoteRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	vote, err := h.service.Vote(feedbackID, userID, req)
	if err != nil {
		if err == ErrFeedbackNotFound {
			writeError(w, http.StatusNotFound, "Feedback not found")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to vote")
		return
	}

	writeJSON(w, http.StatusCreated, vote)
}

// Report 举报
// POST /feedbacks/{id}/report
func (h *Handler) Report(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	userID := r.Header.Get("X-User-ID")
	if userID == "" {
		writeError(w, http.StatusBadRequest, "X-User-ID header is required")
		return
	}

	feedbackID := r.URL.Query().Get("feedback_id")
	if feedbackID == "" {
		writeError(w, http.StatusBadRequest, "feedback_id is required")
		return
	}

	var req CreateReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Invalid request body")
		return
	}

	report, err := h.service.Report(feedbackID, userID, req)
	if err != nil {
		if err == ErrFeedbackNotFound {
			writeError(w, http.StatusNotFound, "Feedback not found")
			return
		}
		if err == ErrAlreadyReported {
			writeError(w, http.StatusConflict, "Already reported")
			return
		}
		writeError(w, http.StatusInternalServerError, "Failed to report")
		return
	}

	writeJSON(w, http.StatusCreated, report)
}

// GetRatingStats 获取评分统计
// GET /stats?app_id=
func (h *Handler) GetRatingStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		writeError(w, http.StatusBadRequest, "app_id is required")
		return
	}

	stats, err := h.service.GetRatingStats(appID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to get rating stats")
		return
	}

	writeJSON(w, http.StatusOK, stats)
}

// GenerateSummary 生成评论摘要
// GET /summary?app_id=
func (h *Handler) GenerateSummary(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	appID := r.URL.Query().Get("app_id")
	if appID == "" {
		writeError(w, http.StatusBadRequest, "app_id is required")
		return
	}

	summary, err := h.service.GenerateSummary(appID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Failed to generate summary")
		return
	}

	writeJSON(w, http.StatusOK, summary)
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/feedbacks", h.handleFeedbacks)
	mux.HandleFunc("/feedbacks/replies", h.handleReplies)
	mux.HandleFunc("/feedbacks/vote", h.Vote)
	mux.HandleFunc("/feedbacks/report", h.Report)
	mux.HandleFunc("/stats", h.GetRatingStats)
	mux.HandleFunc("/summary", h.GenerateSummary)
}

func (h *Handler) handleFeedbacks(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		id := r.URL.Query().Get("id")
		if id != "" {
			h.GetFeedback(w, r)
		} else {
			h.ListFeedbacks(w, r)
		}
	case http.MethodPost:
		h.CreateFeedback(w, r)
	case http.MethodPut:
		h.UpdateFeedback(w, r)
	case http.MethodDelete:
		h.DeleteFeedback(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

func (h *Handler) handleReplies(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.ListReplies(w, r)
	case http.MethodPost:
		h.CreateReply(w, r)
	case http.MethodDelete:
		h.DeleteReply(w, r)
	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// 辅助函数
func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
