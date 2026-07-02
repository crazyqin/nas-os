// Package aicodeassist 提供 REST API 处理器
package aicodeassist

import (
	"encoding/json"
	"net/http"
)

// Handlers AI 代码助手 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, resp response) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(resp)
}

func decodeJSON(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// HandleCodeCompletion 代码补全.
func (h *Handlers) HandleCodeCompletion(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "method not allowed"})
		return
	}
	var req CompletionRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	resp, err := h.manager.CodeCompletion(r.Context(), &req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response{Code: 0, Message: "success", Data: resp})
}

// HandleReviewCode 代码审查.
func (h *Handlers) HandleReviewCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "method not allowed"})
		return
	}
	var req ReviewRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	resp, err := h.manager.ReviewCode(r.Context(), &req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response{Code: 0, Message: "success", Data: resp})
}

// HandleRefactorCode 代码重构建议.
func (h *Handlers) HandleRefactorCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "method not allowed"})
		return
	}
	var req RefactorRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	resp, err := h.manager.RefactorCode(r.Context(), &req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response{Code: 0, Message: "success", Data: resp})
}

// HandleGenerateTests 测试用例生成.
func (h *Handlers) HandleGenerateTests(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "method not allowed"})
		return
	}
	var req TestGenRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	resp, err := h.manager.GenerateTests(r.Context(), &req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response{Code: 0, Message: "success", Data: resp})
}

// HandleExplainCode 代码解释.
func (h *Handlers) HandleExplainCode(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "method not allowed"})
		return
	}
	var req ExplainRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	resp, err := h.manager.ExplainCode(r.Context(), &req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response{Code: 0, Message: "success", Data: resp})
}

// HandleAnalyzeGitDiff Git diff 分析.
func (h *Handlers) HandleAnalyzeGitDiff(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "method not allowed"})
		return
	}
	var req GitDiffRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	resp, err := h.manager.AnalyzeGitDiff(r.Context(), &req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response{Code: 0, Message: "success", Data: resp})
}

// HandleGenerateCommitMessage commit message 生成.
func (h *Handlers) HandleGenerateCommitMessage(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "method not allowed"})
		return
	}
	var req CommitMsgRequest
	if err := decodeJSON(r, &req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}
	resp, err := h.manager.GenerateCommitMessage(r.Context(), &req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, response{Code: 0, Message: "success", Data: resp})
}

// HandleGetConfig 获取配置.
func (h *Handlers) HandleGetConfig(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "method not allowed"})
		return
	}
	h.manager.mu.RLock()
	cfg := *h.manager.config
	h.manager.mu.RUnlock()
	writeJSON(w, http.StatusOK, response{Code: 0, Message: "success", Data: cfg})
}

// HandleGetSupportedLanguages 获取支持的编程语言.
func (h *Handlers) HandleGetSupportedLanguages(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 1, Message: "method not allowed"})
		return
	}
	writeJSON(w, http.StatusOK, response{Code: 0, Message: "success", Data: SupportedLanguages()})
}
