package smbguard

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// Handlers HTTP 处理器
type Handlers struct {
	guard *Guard
}

// NewHandlers 创建 HTTP 处理器
func NewHandlers(guard *Guard) *Handlers {
	return &Handlers{guard: guard}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(mux *http.ServeMux, prefix string) {
	mux.HandleFunc(prefix+"/status", h.handleStatus)
	mux.HandleFunc(prefix+"/banned", h.handleBanned)
	mux.HandleFunc(prefix+"/ban/release", h.handleReleaseBan)
	mux.HandleFunc(prefix+"/whitelist", h.handleWhitelist)
	mux.HandleFunc(prefix+"/config", h.handleConfig)
	mux.HandleFunc(prefix+"/stats", h.handleStats)
}

// response 通用响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func writeJSON(w http.ResponseWriter, code int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(data)
}

// handleStatus 获取守卫状态
func (h *Handlers) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	stats := h.guard.GetStats()
	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    stats,
	})
}

// handleBanned 获取封禁列表
func (h *Handlers) handleBanned(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	banned := h.guard.GetBannedIPs()
	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    banned,
	})
}

// handleReleaseBan 解除封禁
func (h *Handlers) handleReleaseBan(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	var req struct {
		IP string `json:"ip"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
		return
	}

	if req.IP == "" {
		writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "IP 地址不能为空"})
		return
	}

	if err := h.guard.ReleaseBan(req.IP); err != nil {
		writeJSON(w, http.StatusNotFound, response{Code: 404, Message: err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: fmt.Sprintf("已解除 %s 的封禁", req.IP),
	})
}

// handleWhitelist 管理白名单
func (h *Handlers) handleWhitelist(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.guard.mu.RLock()
		whitelist := h.guard.config.WhitelistCIDRs
		h.guard.mu.RUnlock()

		writeJSON(w, http.StatusOK, response{
			Code:    200,
			Message: "ok",
			Data:    whitelist,
		})

	case http.MethodPost:
		var req struct {
			Action string `json:"action"` // add / remove
			CIDR   string `json:"cidr"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的请求"})
			return
		}

		var err error
		switch strings.ToLower(req.Action) {
		case "add":
			err = h.guard.AddWhitelist(req.CIDR)
		case "remove":
			err = h.guard.RemoveWhitelist(req.CIDR)
		default:
			writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "操作类型必须是 add 或 remove"})
			return
		}

		if err != nil {
			writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: err.Error()})
			return
		}

		writeJSON(w, http.StatusOK, response{
			Code:    200,
			Message: fmt.Sprintf("白名单操作成功: %s %s", req.Action, req.CIDR),
		})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
	}
}

// handleConfig 管理配置
func (h *Handlers) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		h.guard.mu.RLock()
		config := h.guard.config
		h.guard.mu.RUnlock()

		writeJSON(w, http.StatusOK, response{
			Code:    200,
			Message: "ok",
			Data:    config,
		})

	case http.MethodPut:
		var config GuardConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeJSON(w, http.StatusBadRequest, response{Code: 400, Message: "无效的配置"})
			return
		}

		h.guard.UpdateConfig(config)
		writeJSON(w, http.StatusOK, response{
			Code:    200,
			Message: "配置已更新",
		})

	default:
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
	}
}

// handleStats 获取详细统计
func (h *Handlers) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeJSON(w, http.StatusMethodNotAllowed, response{Code: 405, Message: "方法不允许"})
		return
	}

	stats := h.guard.GetStats()
	writeJSON(w, http.StatusOK, response{
		Code:    200,
		Message: "ok",
		Data:    stats,
	})
}
