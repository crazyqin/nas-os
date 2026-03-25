package directplay

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/gorilla/mux"
)

// Handlers 直链播放API处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *mux.Router) {
	// 直链播放相关路由
	r.HandleFunc("/api/directplay/status", h.GetStatus).Methods("GET")
	r.HandleFunc("/api/directplay/config", h.GetConfig).Methods("GET")
	r.HandleFunc("/api/directplay/config", h.UpdateConfig).Methods("PUT")

	// 网盘管理
	r.HandleFunc("/api/directplay/providers", h.ListProviders).Methods("GET")
	r.HandleFunc("/api/directplay/providers/{provider}/connect", h.ConnectProvider).Methods("POST")
	r.HandleFunc("/api/directplay/providers/{provider}/disconnect", h.DisconnectProvider).Methods("POST")
	r.HandleFunc("/api/directplay/providers/{provider}/test", h.TestProviderConnection).Methods("POST")

	// 文件操作
	r.HandleFunc("/api/directplay/providers/{provider}/files", h.ListFiles).Methods("GET")
	r.HandleFunc("/api/directplay/providers/{provider}/files/{fileId}", h.GetFileInfo).Methods("GET")

	// 直链获取
	r.HandleFunc("/api/directplay/providers/{provider}/files/{fileId}/link", h.GetDirectLink).Methods("GET")
	r.HandleFunc("/api/directplay/providers/{provider}/files/{fileId}/refresh", h.RefreshDirectLink).Methods("POST")

	// 流媒体会话
	r.HandleFunc("/api/directplay/stream", h.CreateStreamSession).Methods("POST")
	r.HandleFunc("/api/directplay/stream/{sessionId}", h.GetStreamSession).Methods("GET")
	r.HandleFunc("/api/directplay/stream/{sessionId}", h.CloseStreamSession).Methods("DELETE")

	// 缓存管理
	r.HandleFunc("/api/directplay/cache/clear", h.ClearCache).Methods("POST")
}

// GetStatus 获取直链播放状态
func (h *Handlers) GetStatus(w http.ResponseWriter, r *http.Request) {
	status := h.manager.GetStatus()
	h.respondJSON(w, http.StatusOK, status)
}

// GetConfig 获取配置
func (h *Handlers) GetConfig(w http.ResponseWriter, r *http.Request) {
	h.respondJSON(w, http.StatusOK, h.manager.config)
}

// UpdateConfig 更新配置
func (h *Handlers) UpdateConfig(w http.ResponseWriter, r *http.Request) {
	var config DirectPlayConfig
	if err := json.NewDecoder(r.Body).Decode(&config); err != nil {
		h.respondError(w, http.StatusBadRequest, "无效的配置数据")
		return
	}

	// 更新配置
	h.manager.config = &config
	h.respondJSON(w, http.StatusOK, map[string]string{"message": "配置已更新"})
}

// ListProviders 列出支持的网盘
func (h *Handlers) ListProviders(w http.ResponseWriter, r *http.Request) {
	status := h.manager.GetStatus()
	h.respondJSON(w, http.StatusOK, status.Providers)
}

// ConnectProvider 连接网盘
func (h *Handlers) ConnectProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerType := ProviderType(vars["provider"])

	var req struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
		DriveID      string `json:"driveId"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "无效的请求数据")
		return
	}

	// 设置令牌
	if err := h.manager.SetProviderTokens(providerType, req.AccessToken, req.RefreshToken, req.DriveID); err != nil {
		h.respondError(w, http.StatusBadRequest, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "连接成功"})
}

// DisconnectProvider 断开网盘连接
func (h *Handlers) DisconnectProvider(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerType := ProviderType(vars["provider"])

	// 清除令牌
	_ = h.manager.SetProviderTokens(providerType, "", "", "")

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "已断开连接"})
}

// TestProviderConnection 测试网盘连接
func (h *Handlers) TestProviderConnection(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerType := ProviderType(vars["provider"])

	var req struct {
		AccessToken  string `json:"accessToken"`
		RefreshToken string `json:"refreshToken"`
	}

	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "无效的请求数据")
		return
	}

	info, err := h.manager.TestConnection(r.Context(), providerType, req.AccessToken, req.RefreshToken)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, info)
}

// ListFiles 列出文件
func (h *Handlers) ListFiles(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerType := ProviderType(vars["provider"])

	// 获取认证信息
	accessToken := r.Header.Get("X-Access-Token")
	refreshToken := r.Header.Get("X-Refresh-Token")
	driveID := r.Header.Get("X-Drive-ID")

	// 解析请求参数
	req := ListFilesRequest{
		Provider:     providerType,
		DirPath:      r.URL.Query().Get("path"),
		Recursive:    r.URL.Query().Get("recursive") == "true",
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		DriveID:      driveID,
	}

	if pageStr := r.URL.Query().Get("page"); pageStr != "" {
		req.Page, _ = strconv.Atoi(pageStr)
	}
	if sizeStr := r.URL.Query().Get("pageSize"); sizeStr != "" {
		req.PageSize, _ = strconv.Atoi(sizeStr)
	}

	resp, err := h.manager.ListFiles(r.Context(), &req)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, resp)
}

// GetFileInfo 获取文件信息
func (h *Handlers) GetFileInfo(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerType := ProviderType(vars["provider"])
	fileID := vars["fileId"]

	accessToken := r.Header.Get("X-Access-Token")

	info, err := h.manager.GetFileInfo(r.Context(), providerType, fileID, accessToken)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, info)
}

// GetDirectLink 获取直链
func (h *Handlers) GetDirectLink(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerType := ProviderType(vars["provider"])
	fileID := vars["fileId"]

	accessToken := r.Header.Get("X-Access-Token")
	refreshToken := r.Header.Get("X-Refresh-Token")
	driveID := r.Header.Get("X-Drive-ID")

	req := DirectPlayRequest{
		Provider:     providerType,
		FileID:       fileID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		DriveID:      driveID,
		ForceRefresh: r.URL.Query().Get("force") == "true",
		NeedStream:   r.URL.Query().Get("stream") == "true",
	}

	link, err := h.manager.GetDirectLink(r.Context(), &req)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, link)
}

// RefreshDirectLink 刷新直链
func (h *Handlers) RefreshDirectLink(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	providerType := ProviderType(vars["provider"])
	fileID := vars["fileId"]

	accessToken := r.Header.Get("X-Access-Token")
	refreshToken := r.Header.Get("X-Refresh-Token")
	driveID := r.Header.Get("X-Drive-ID")

	req := DirectPlayRequest{
		Provider:     providerType,
		FileID:       fileID,
		AccessToken:  accessToken,
		RefreshToken: refreshToken,
		DriveID:      driveID,
		ForceRefresh: true,
	}

	link, err := h.manager.RefreshLink(r.Context(), &req)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, link)
}

// CreateStreamSession 创建流媒体会话
func (h *Handlers) CreateStreamSession(w http.ResponseWriter, r *http.Request) {
	var req DirectPlayRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.respondError(w, http.StatusBadRequest, "无效的请求数据")
		return
	}

	// 从header获取认证信息
	if req.AccessToken == "" {
		req.AccessToken = r.Header.Get("X-Access-Token")
	}
	if req.RefreshToken == "" {
		req.RefreshToken = r.Header.Get("X-Refresh-Token")
	}
	if req.DriveID == "" {
		req.DriveID = r.Header.Get("X-Drive-ID")
	}

	session, err := h.manager.CreateStreamSession(r.Context(), &req)
	if err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, session)
}

// GetStreamSession 获取流媒体会话
func (h *Handlers) GetStreamSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	session, err := h.manager.GetStreamSession(sessionID)
	if err != nil {
		h.respondError(w, http.StatusNotFound, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, session)
}

// CloseStreamSession 关闭流媒体会话
func (h *Handlers) CloseStreamSession(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	sessionID := vars["sessionId"]

	if err := h.manager.CloseStreamSession(sessionID); err != nil {
		h.respondError(w, http.StatusInternalServerError, err.Error())
		return
	}

	h.respondJSON(w, http.StatusOK, map[string]string{"message": "会话已关闭"})
}

// ClearCache 清除缓存
func (h *Handlers) ClearCache(w http.ResponseWriter, r *http.Request) {
	h.manager.ClearCache()
	h.respondJSON(w, http.StatusOK, map[string]string{"message": "缓存已清除"})
}

// respondJSON 返回JSON响应
func (h *Handlers) respondJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// respondError 返回错误响应
func (h *Handlers) respondError(w http.ResponseWriter, status int, message string) {
	h.respondJSON(w, status, map[string]string{"error": message})
}
