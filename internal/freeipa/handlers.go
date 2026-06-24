package freeipa

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"time"
)

// Handler FreeIPA HTTP API 处理器
type Handler struct {
	client *Client
	logger *slog.Logger
}

// NewHandler 创建处理器
func NewHandler(client *Client, logger *slog.Logger) *Handler {
	if logger == nil {
		logger = slog.Default()
	}
	return &Handler{
		client: client,
		logger: logger,
	}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("/api/v1/freeipa/status", h.handleStatus)
	mux.HandleFunc("/api/v1/freeipa/config", h.handleConfig)
	mux.HandleFunc("/api/v1/freeipa/connect", h.handleConnect)
	mux.HandleFunc("/api/v1/freeipa/disconnect", h.handleDisconnect)
	mux.HandleFunc("/api/v1/freeipa/auth", h.handleAuth)
	mux.HandleFunc("/api/v1/freeipa/users", h.handleUsers)
	mux.HandleFunc("/api/v1/freeipa/groups", h.handleGroups)
	mux.HandleFunc("/api/v1/freeipa/sync", h.handleSync)
	mux.HandleFunc("/api/v1/freeipa/stats", h.handleStats)
}

// APIResponse 通用 API 响应
type APIResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// handleStatus 获取目录服务状态
func (h *Handler) handleStatus(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	stats := h.client.GetStats()
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: stats})
}

// handleConfig 获取/更新配置
func (h *Handler) handleConfig(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		config := h.client.GetConfig()
		// 隐藏密码
		config.BindPassword = "***"
		writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: config})

	case http.MethodPut, http.MethodPost:
		var config DirectoryConfig
		if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
			writeError(w, http.StatusBadRequest, "无效的请求体")
			return
		}

		// 验证必填字段
		if config.Host == "" {
			writeError(w, http.StatusBadRequest, "主机地址不能为空")
			return
		}

		if config.Port == 0 {
			config.Port = 389
		}

		if config.BaseDN == "" {
			config.BaseDN = "dc=example,dc=com"
		}

		config.UpdatedAt = time.Now()
		h.client.UpdateConfig(config)

		h.logger.Info("FreeIPA 配置已更新", "host", config.Host, "port", config.Port)
		writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: "配置已更新"})

	default:
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET/PUT/POST 方法")
	}
}

// handleConnect 连接到 FreeIPA
func (h *Handler) handleConnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	if h.client.IsConnected() {
		writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: "已连接"})
		return
	}

	ctx := r.Context()
	if err := h.client.Connect(ctx); err != nil {
		h.logger.Error("FreeIPA 连接失败", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: "连接成功"})
}

// handleDisconnect 断开连接
func (h *Handler) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	if err := h.client.Disconnect(); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: "已断开连接"})
}

// handleAuth 用户认证
func (h *Handler) handleAuth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "无效的请求体")
		return
	}

	if req.Username == "" {
		writeError(w, http.StatusBadRequest, "用户名不能为空")
		return
	}

	result, err := h.client.Authenticate(r.Context(), req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{Success: result.Success, Data: result})
}

// handleUsers 用户管理
func (h *Handler) handleUsers(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	filter := UserSearchFilter{
		Username: r.URL.Query().Get("username"),
		Email:    r.URL.Query().Get("email"),
		Group:    r.URL.Query().Get("group"),
	}

	users, total, err := h.client.SearchUsers(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"users": users,
			"total": total,
		},
	})
}

// handleGroups 组管理
func (h *Handler) handleGroups(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	filter := GroupSearchFilter{
		Name: r.URL.Query().Get("name"),
	}

	groups, total, err := h.client.SearchGroups(r.Context(), filter)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]interface{}{
			"groups": groups,
			"total":  total,
		},
	})
}

// handleSync 触发同步
func (h *Handler) handleSync(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 POST 方法")
		return
	}

	if !h.client.IsConnected() {
		writeError(w, http.StatusBadRequest, "目录服务未连接")
		return
	}

	result, err := h.client.FullSync(r.Context())
	if err != nil {
		h.logger.Error("FreeIPA 同步失败", "error", err)
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.logger.Info("FreeIPA 同步完成",
		"users", result.UsersSynced,
		"groups", result.GroupsSynced)

	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: result})
}

// handleStats 获取统计信息
func (h *Handler) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "仅支持 GET 方法")
		return
	}

	stats := h.client.GetStats()
	writeJSON(w, http.StatusOK, APIResponse{Success: true, Data: stats})
}

// writeJSON 写入 JSON 响应
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// writeError 写入错误响应
func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, APIResponse{Success: false, Error: message})
}
