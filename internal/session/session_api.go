// Package session 提供SMB/NFS会话监控和管理功能
package session

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"nas-os/internal/api"
)

// API 会话API处理器
type API struct {
	manager *Manager
	monitor *Monitor
}

// NewAPI 创建会话API处理器
func NewAPI(manager *Manager, monitor *Monitor) *API {
	return &API{
		manager: manager,
		monitor: monitor,
	}
}

// RegisterRoutes 注册路由
func (a *API) RegisterRoutes(r *gin.RouterGroup) {
	sessions := r.Group("/sessions")
	{
		// 会话列表和统计
		sessions.GET("", a.ListSessions)
		sessions.GET("/stats", a.GetStats)
		sessions.GET("/events", a.GetEvents)

		// 按类型/用户/客户端过滤
		sessions.GET("/type/:type", a.ListSessionsByType)
		sessions.GET("/user/:user", a.ListSessionsByUser)
		sessions.GET("/client/:ip", a.ListSessionsByClient)

		// 单个会话操作
		sessions.GET("/:id", a.GetSession)
		sessions.DELETE("/:id", a.KickSession)

		// 批量操作
		sessions.POST("/kick/user/:user", a.KickSessionsByUser)
		sessions.POST("/kick/client/:ip", a.KickSessionsByClient)

		// 监控器状态
		sessions.GET("/monitor/status", a.GetMonitorStatus)
		sessions.POST("/monitor/refresh", a.ForceRefresh)
	}

	// 配置路由
	config := r.Group("/session-config")
	{
		config.GET("", a.GetConfig)
		config.PUT("", a.UpdateConfig)
	}
}

// ListSessionsRequest 列表请求参数
type ListSessionsRequest struct {
	Type   string `form:"type"`   // smb, nfs
	Status string `form:"status"` // active, idle, stale
	User   string `form:"user"`
	Sort   string `form:"sort"`   // connected_at, last_active, bytes
	Order  string `form:"order"`  // asc, desc
	Limit  int    `form:"limit"`
	Offset int    `form:"offset"`
}

// SessionListResponse 会话列表响应
type SessionListResponse struct {
	Total   int       `json:"total"`
	Sessions []*Session `json:"sessions"`
}

// ListSessions 列出所有会话
// @Summary 列出所有会话
// @Description 获取所有SMB和NFS会话列表
// @Tags sessions
// @Accept json
// @Produce json
// @Param type query string false "会话类型 (smb, nfs)"
// @Param status query string false "会话状态 (active, idle, stale)"
// @Param user query string false "用户名过滤"
// @Param sort query string false "排序字段"
// @Param order query string false "排序方向 (asc, desc)"
// @Param limit query int false "返回数量限制"
// @Param offset query int false "偏移量"
// @Success 200 {object} SessionListResponse
// @Failure 500 {object} api.ErrorResponse
// @Router /sessions [get]
func (a *API) ListSessions(c *gin.Context) {
	var req ListSessionsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		api.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 获取会话列表
	sessions := a.manager.ListSessions()

	// 过滤
	if req.Type != "" {
		sessions = filterByType(sessions, SessionType(req.Type))
	}
	if req.Status != "" {
		sessions = filterByStatus(sessions, SessionStatus(req.Status))
	}
	if req.User != "" {
		sessions = filterByUser(sessions, req.User)
	}

	// 排序
	sessions = sortSessions(sessions, req.Sort, req.Order)

	// 分页
	total := len(sessions)
	if req.Offset > 0 && req.Offset < len(sessions) {
		sessions = sessions[req.Offset:]
	}
	if req.Limit > 0 && req.Limit < len(sessions) {
		sessions = sessions[:req.Limit]
	}

	c.JSON(http.StatusOK, SessionListResponse{
		Total:    total,
		Sessions: sessions,
	})
}

// GetStats 获取会话统计
// @Summary 获取会话统计
// @Description 获取会话统计信息，包括总数、活跃数、传输量等
// @Tags sessions
// @Accept json
// @Produce json
// @Success 200 {object} SessionStats
// @Router /sessions/stats [get]
func (a *API) GetStats(c *gin.Context) {
	stats := a.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// GetEvents 获取事件历史
// @Summary 获取事件历史
// @Description 获取会话事件历史记录
// @Tags sessions
// @Accept json
// @Produce json
// @Param limit query int false "返回数量限制"
// @Success 200 {array} SessionEvent
// @Router /sessions/events [get]
func (a *API) GetEvents(c *gin.Context) {
	limit := 100
	if l := c.Query("limit"); l != "" {
		if v, err := strconv.Atoi(l); err == nil && v > 0 {
			limit = v
		}
	}

	events := a.manager.GetEvents(limit)
	c.JSON(http.StatusOK, events)
}

// ListSessionsByType 按类型列出会话
// @Summary 按类型列出会话
// @Description 按会话类型(SMB/NFS)列出会话
// @Tags sessions
// @Accept json
// @Produce json
// @Param type path string true "会话类型 (smb, nfs)"
// @Success 200 {array} Session
// @Failure 400 {object} api.ErrorResponse
// @Router /sessions/type/{type} [get]
func (a *API) ListSessionsByType(c *gin.Context) {
	sessionType := c.Param("type")
	if sessionType != "smb" && sessionType != "nfs" {
		api.BadRequest(c, "无效的会话类型: "+sessionType)
		return
	}

	sessions := a.manager.ListSessionsByType(SessionType(sessionType))
	c.JSON(http.StatusOK, sessions)
}

// ListSessionsByUser 按用户列出会话
// @Summary 按用户列出会话
// @Description 列出指定用户的所有会话
// @Tags sessions
// @Accept json
// @Produce json
// @Param user path string true "用户名"
// @Success 200 {array} Session
// @Router /sessions/user/{user} [get]
func (a *API) ListSessionsByUser(c *gin.Context) {
	user := c.Param("user")
	sessions := a.manager.ListSessionsByUser(user)
	c.JSON(http.StatusOK, sessions)
}

// ListSessionsByClient 按客户端列出会话
// @Summary 按客户端列出会话
// @Description 列出指定客户端IP的所有会话
// @Tags sessions
// @Accept json
// @Produce json
// @Param ip path string true "客户端IP"
// @Success 200 {array} Session
// @Router /sessions/client/{ip} [get]
func (a *API) ListSessionsByClient(c *gin.Context) {
	ip := c.Param("ip")
	sessions := a.manager.ListSessionsByClient(ip)
	c.JSON(http.StatusOK, sessions)
}

// GetSession 获取单个会话
// @Summary 获取单个会话
// @Description 获取指定会话的详细信息
// @Tags sessions
// @Accept json
// @Produce json
// @Param id path string true "会话ID"
// @Success 200 {object} Session
// @Failure 404 {object} api.ErrorResponse
// @Router /sessions/{id} [get]
func (a *API) GetSession(c *gin.Context) {
	id := c.Param("id")
	session, err := a.manager.GetSession(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, session)
}

// KickSessionRequest 断开会话请求
type KickSessionRequest struct {
	Reason string `json:"reason"`
}

// KickSessionResponse 断开会话响应
type KickSessionResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	SessionID string `json:"session_id"`
	User      string `json:"user,omitempty"`
	ClientIP  string `json:"client_ip,omitempty"`
}

// KickSession 强制断开会话
// @Summary 强制断开会话
// @Description 强制断开指定的会话
// @Tags sessions
// @Accept json
// @Produce json
// @Param id path string true "会话ID"
// @Param body body KickSessionRequest false "断开原因"
// @Success 200 {object} KickSessionResponse
// @Failure 404 {object} api.ErrorResponse
// @Router /sessions/{id} [delete]
func (a *API) KickSession(c *gin.Context) {
	id := c.Param("id")

	// 获取会话信息用于响应
	session, err := a.manager.GetSession(id)
	if err != nil {
		api.NotFound(c, err.Error())
		return
	}

	var req KickSessionRequest
	_ = c.ShouldBindJSON(&req)

	reason := req.Reason
	if reason == "" {
		reason = "管理员强制断开"
	}

	if err := a.manager.KickSession(id, reason); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, KickSessionResponse{
		Success:   true,
		Message:   "会话已断开: " + reason,
		SessionID: id,
		User:      session.User,
		ClientIP:  session.ClientIP,
	})
}

// KickSessionsByUserRequest 批量断开请求
type KickSessionsByUserRequest struct {
	Reason string `json:"reason"`
}

// KickSessionsByUserResponse 批量断开响应
type KickSessionsByUserResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
	Count   int    `json:"count"`
	User    string `json:"user"`
}

// KickSessionsByUser 强制断开用户所有会话
// @Summary 强制断开用户所有会话
// @Description 强制断开指定用户的所有会话
// @Tags sessions
// @Accept json
// @Produce json
// @Param user path string true "用户名"
// @Param body body KickSessionsByUserRequest false "断开原因"
// @Success 200 {object} KickSessionsByUserResponse
// @Router /sessions/kick/user/{user} [post]
func (a *API) KickSessionsByUser(c *gin.Context) {
	user := c.Param("user")

	var req KickSessionsByUserRequest
	_ = c.ShouldBindJSON(&req)

	reason := req.Reason
	if reason == "" {
		reason = "管理员强制断开"
	}

	count, err := a.manager.KickSessionsByUser(user, reason)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, KickSessionsByUserResponse{
		Success: true,
		Message: "已断开用户所有会话: " + reason,
		Count:   count,
		User:    user,
	})
}

// KickSessionsByClient 强制断开客户端所有会话
// @Summary 强制断开客户端所有会话
// @Description 强制断开指定客户端IP的所有会话
// @Tags sessions
// @Accept json
// @Produce json
// @Param ip path string true "客户端IP"
// @Param body body KickSessionsByUserRequest false "断开原因"
// @Success 200 {object} KickSessionsByUserResponse
// @Router /sessions/kick/client/{ip} [post]
func (a *API) KickSessionsByClient(c *gin.Context) {
	ip := c.Param("ip")

	var req KickSessionsByUserRequest
	_ = c.ShouldBindJSON(&req)

	reason := req.Reason
	if reason == "" {
		reason = "管理员强制断开"
	}

	count, err := a.manager.KickSessionsByClient(ip, reason)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, KickSessionsByUserResponse{
		Success: true,
		Message: "已断开客户端所有会话: " + reason,
		Count:   count,
		User:    ip,
	})
}

// GetMonitorStatus 获取监控器状态
// @Summary 获取监控器状态
// @Description 获取会话监控器的运行状态
// @Tags sessions
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{}}
// @Router /sessions/monitor/status [get]
func (a *API) GetMonitorStatus(c *gin.Context) {
	if a.monitor == nil {
		c.JSON(http.StatusOK, gin.H{
			"running": false,
			"message": "监控器未初始化",
		})
		return
	}

	status := a.monitor.GetStatus()
	c.JSON(http.StatusOK, status)
}

// ForceRefresh 强制刷新会话数据
// @Summary 强制刷新会话数据
// @Description 立即刷新会话数据（不从缓存）
// @Tags sessions
// @Accept json
// @Produce json
// @Success 200 {object} map[string]string}
// @Failure 500 {object} api.ErrorResponse
// @Router /sessions/monitor/refresh [post]
func (a *API) ForceRefresh(c *gin.Context) {
	if a.monitor == nil {
		api.InternalError(c, "监控器未初始化")
		return
	}

	if err := a.monitor.ForceRefresh(); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "刷新成功",
		"time":    time.Now().Format(time.RFC3339),
	})
}

// GetConfig 获取配置
// @Summary 获取会话管理配置
// @Description 获取会话管理器的配置
// @Tags session-config
// @Accept json
// @Produce json
// @Success 200 {object} Config
// @Router /session-config [get]
func (a *API) GetConfig(c *gin.Context) {
	config := a.manager.GetConfig()
	c.JSON(http.StatusOK, config)
}

// UpdateConfig 更新配置
// @Summary 更新会话管理配置
// @Description 更新会话管理器的配置
// @Tags session-config
// @Accept json
// @Produce json
// @Param body body Config true "配置"
// @Success 200 {object} Config
// @Failure 400 {object} api.ErrorResponse
// @Router /session-config [put]
func (a *API) UpdateConfig(c *gin.Context) {
	var config Config
	if err := c.ShouldBindJSON(&config); err != nil {
		api.BadRequest(c, "参数错误: "+err.Error())
		return
	}

	// 验证配置
	if config.RefreshInterval < 5*time.Second {
		api.BadRequest(c, "刷新间隔不能小于5秒")
		return
	}
	if config.IdleTimeout < time.Minute {
		api.BadRequest(c, "空闲超时不能小于1分钟")
		return
	}

	if err := a.manager.UpdateConfig(&config); err != nil {
		api.InternalError(c, err.Error())
		return
	}

	c.JSON(http.StatusOK, config)
}

// ========== 辅助函数 ==========

func filterByType(sessions []*Session, sessionType SessionType) []*Session {
	var result []*Session
	for _, s := range sessions {
		if s.Type == sessionType {
			result = append(result, s)
		}
	}
	return result
}

func filterByStatus(sessions []*Session, status SessionStatus) []*Session {
	var result []*Session
	for _, s := range sessions {
		if s.Status == status {
			result = append(result, s)
		}
	}
	return result
}

func filterByUser(sessions []*Session, user string) []*Session {
	var result []*Session
	for _, s := range sessions {
		if s.User == user {
			result = append(result, s)
		}
	}
	return result
}

func sortSessions(sessions []*Session, sortField, order string) []*Session {
	if sortField == "" {
		sortField = "connected_at"
	}
	if order == "" {
		order = "desc"
	}

	// 简单排序实现
	result := make([]*Session, len(sessions))
	copy(result, sessions)

	// 根据字段排序
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			var swap bool
			switch sortField {
			case "connected_at":
				swap = result[i].ConnectedAt.After(result[j].ConnectedAt)
			case "last_active":
				swap = result[i].LastActiveAt.After(result[j].LastActiveAt)
			case "bytes_read":
				swap = result[i].BytesRead > result[j].BytesRead
			case "bytes_written":
				swap = result[i].BytesWritten > result[j].BytesWritten
			case "user":
				swap = result[i].User > result[j].User
			default:
				swap = result[i].ConnectedAt.After(result[j].ConnectedAt)
			}

			if order == "asc" {
				swap = !swap
			}

			if swap {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	return result
}

// RegisterRoutesWithGroup 注册路由到指定的路由组
func RegisterRoutesWithGroup(r *gin.RouterGroup, manager *Manager, monitor *Monitor) {
	api := NewAPI(manager, monitor)
	api.RegisterRoutes(r)
}