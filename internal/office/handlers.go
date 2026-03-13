package office

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers OnlyOffice HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册路由
// 注意：编辑和配置操作需要认证，调用方应添加认证中间件
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	office := api.Group("/office")
	// 敏感操作需要认证，调用方应添加认证中间件
	{
		// 配置管理 - 需要管理员权限
		office.GET("/config", h.getConfig)
		office.PUT("/config", h.updateConfig)  // 需要管理员权限

		// 服务状态
		office.GET("/status", h.getStatus)

		// 编辑会话 - 需要认证
		office.POST("/edit/:fileId", h.startEdit)        // 需要认证
		office.GET("/sessions", h.listSessions)
		office.GET("/sessions/:sessionId", h.getSession)
		office.DELETE("/sessions/:sessionId", h.closeSession) // 需要认证

		// OnlyOffice 回调（不带 sessionId，通过 body 中的 key 识别）
		// 这是一个内部回调接口，应该验证来源
		office.POST("/callback", h.handleCallback)

		// 文件关联
		office.GET("/associations", h.getAssociations)
		office.GET("/associations/:ext", h.getAssociation)

		// 健康检查
		office.GET("/health", h.healthCheck)
	}
}

// ========== 通用响应 ==========

type Response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func Success(data interface{}) Response {
	return Response{Code: 0, Message: "success", Data: data}
}

func Error(code int, message string) Response {
	return Response{Code: code, Message: message}
}

// ========== 配置管理 ==========

// getConfig 获取配置
// @Summary 获取 OnlyOffice 配置
// @Description 获取 OnlyOffice 集成配置和支持的文件类型
// @Tags office
// @Produce json
// @Success 200 {object} Response{data=ConfigResponse}
// @Router /office/config [get]
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	associations := h.manager.GetAllFileAssociations()

	c.JSON(http.StatusOK, Success(ConfigResponse{
		Config:       *cfg,
		Associations: associations,
	}))
}

// updateConfig 更新配置
// @Summary 更新 OnlyOffice 配置
// @Description 更新 OnlyOffice 集成配置
// @Tags office
// @Accept json
// @Produce json
// @Param request body UpdateConfigRequest true "配置更新请求"
// @Success 200 {object} Response
// @Router /office/config [put]
func (h *Handlers) updateConfig(c *gin.Context) {
	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, Error(400, "无效的请求: "+err.Error()))
		return
	}

	if err := h.manager.UpdateConfig(req); err != nil {
		c.JSON(http.StatusInternalServerError, Error(500, "更新配置失败: "+err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// getStatus 获取服务状态
// @Summary 获取 OnlyOffice 服务状态
// @Description 获取 OnlyOffice 集成服务状态
// @Tags office
// @Produce json
// @Success 200 {object} Response
// @Router /office/status [get]
func (h *Handlers) getStatus(c *gin.Context) {
	status := gin.H{
		"enabled": h.manager.IsEnabled(),
	}

	// 检查服务器连接
	if h.manager.IsEnabled() {
		if err := h.manager.CheckServer(); err != nil {
			status["server_status"] = "unreachable"
			status["server_error"] = err.Error()
		} else {
			status["server_status"] = "ok"
		}
	} else {
		status["server_status"] = "disabled"
	}

	// 统计活跃会话
	sessions, _ := h.manager.ListSessions(SessionStatusActive, 1000, 0)
	status["active_sessions"] = len(sessions)

	// 统计所有会话
	allSessions, _ := h.manager.ListSessions("", 1000, 0)
	status["total_sessions"] = len(allSessions)

	c.JSON(http.StatusOK, Success(status))
}

// ========== 编辑会话 ==========

// startEdit 开始编辑
// @Summary 开始编辑文档
// @Description 创建编辑会话，返回编辑器配置
// @Tags office
// @Accept json
// @Produce json
// @Param fileId path string true "文件 ID"
// @Param request body EditRequest true "编辑请求"
// @Success 200 {object} Response{data=EditResponse}
// @Router /office/edit/{fileId} [post]
func (h *Handlers) startEdit(c *gin.Context) {
	fileID := c.Param("fileId")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, Error(400, "文件 ID 不能为空"))
		return
	}

	// 检查是否启用
	if !h.manager.IsEnabled() {
		c.JSON(http.StatusServiceUnavailable, Error(503, ErrNotEnabled))
		return
	}

	var req EditRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 使用默认值
		req.Mode = "edit"
		req.Language = "zh-CN"
	}

	// 默认值
	if req.Mode == "" {
		req.Mode = "edit"
	}
	if req.Language == "" {
		req.Language = "zh-CN"
	}

	// 获取用户信息
	userID := c.GetString("user_id")
	userName := c.GetString("username")
	if userID == "" {
		userID = "anonymous"
	}
	if userName == "" {
		userName = "匿名用户"
	}

	// 创建会话
	session, editorConfig, err := h.manager.CreateSession(fileID, userID, userName, req.Mode)
	if err != nil {
		c.JSON(http.StatusBadRequest, Error(400, err.Error()))
		return
	}

	// 构建响应
	cfg := h.manager.GetConfig()
	editorURL := cfg.ServerURL + "/web-apps/apps/api/documents/api.js"

	c.JSON(http.StatusOK, Success(EditResponse{
		SessionID:    session.ID,
		EditorConfig: *editorConfig,
		EditorURL:    editorURL,
		ExpiresAt:    session.ExpiresAt,
	}))
}

// listSessions 列出会话
// @Summary 列出编辑会话
// @Description 获取所有或指定状态的编辑会话
// @Tags office
// @Produce json
// @Param status query string false "会话状态过滤"
// @Param limit query int false "返回数量限制" default(20)
// @Param offset query int false "偏移量" default(0)
// @Success 200 {object} Response{data=SessionListResponse}
// @Router /office/sessions [get]
func (h *Handlers) listSessions(c *gin.Context) {
	status := SessionStatus(c.Query("status"))
	limit := 20
	offset := 0

	if l := c.Query("limit"); l != "" {
		if parsed, err := parseInt(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}

	if o := c.Query("offset"); o != "" {
		if parsed, err := parseInt(o); err == nil && parsed >= 0 {
			offset = parsed
		}
	}

	sessions, total := h.manager.ListSessions(status, limit, offset)

	// 转换为响应格式
	result := make([]EditingSession, len(sessions))
	for i, s := range sessions {
		result[i] = *s
	}

	c.JSON(http.StatusOK, Success(SessionListResponse{
		Total:    total,
		Sessions: result,
	}))
}

// getSession 获取会话详情
// @Summary 获取会话详情
// @Description 获取指定编辑会话的详细信息
// @Tags office
// @Produce json
// @Param sessionId path string true "会话 ID"
// @Success 200 {object} Response{data=EditingSession}
// @Router /office/sessions/{sessionId} [get]
func (h *Handlers) getSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, Error(400, "会话 ID 不能为空"))
		return
	}

	session, err := h.manager.GetSession(sessionID)
	if err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(session))
}

// closeSession 关闭会话
// @Summary 关闭编辑会话
// @Description 关闭指定的编辑会话
// @Tags office
// @Produce json
// @Param sessionId path string true "会话 ID"
// @Success 200 {object} Response
// @Router /office/sessions/{sessionId} [delete]
func (h *Handlers) closeSession(c *gin.Context) {
	sessionID := c.Param("sessionId")
	if sessionID == "" {
		c.JSON(http.StatusBadRequest, Error(400, "会话 ID 不能为空"))
		return
	}

	if err := h.manager.CloseSession(sessionID); err != nil {
		c.JSON(http.StatusNotFound, Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, Success(nil))
}

// ========== 回调处理 ==========

// handleCallback 处理 OnlyOffice 回调
// @Summary 处理 OnlyOffice 回调
// @Description 接收并处理 OnlyOffice Document Server 的回调通知
// @Tags office
// @Accept json
// @Produce json
// @Param request body CallbackRequest true "回调请求"
// @Success 200 {object} CallbackResponse
// @Router /office/callback [post]
func (h *Handlers) handleCallback(c *gin.Context) {
	var req CallbackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, CallbackResponse{Error: 1})
		return
	}

	// 验证 Token（如果启用）
	cfg := h.manager.GetConfig()
	if cfg.CallbackAuth && cfg.SecretKey != "" {
		token := c.GetHeader("Authorization")
		if token == "" {
			token = req.Token
		}
		if !h.manager.ValidateCallbackToken(token, req) {
			c.JSON(http.StatusUnauthorized, CallbackResponse{Error: 1})
			return
		}
	}

	// 通过 key 查找会话并处理回调
	if err := h.manager.HandleCallbackByKey(req); err != nil {
		c.JSON(http.StatusOK, CallbackResponse{Error: 1})
		return
	}

	c.JSON(http.StatusOK, CallbackResponse{Error: 0})
}

// ========== 文件关联 ==========

// getAssociations 获取所有文件关联
// @Summary 获取文件关联配置
// @Description 获取所有支持的文件类型及其关联配置
// @Tags office
// @Produce json
// @Success 200 {object} Response
// @Router /office/associations [get]
func (h *Handlers) getAssociations(c *gin.Context) {
	associations := h.manager.GetAllFileAssociations()

	// 转换为列表格式
	result := make([]FileAssociation, 0, len(associations))
	for _, a := range associations {
		result = append(result, a)
	}

	c.JSON(http.StatusOK, Success(gin.H{
		"total": len(result),
		"items": result,
	}))
}

// getAssociation 获取指定扩展名的关联
// @Summary 获取文件关联
// @Description 获取指定文件扩展名的关联配置
// @Tags office
// @Produce json
// @Param ext path string true "文件扩展名"
// @Success 200 {object} Response{data=FileAssociation}
// @Router /office/associations/{ext} [get]
func (h *Handlers) getAssociation(c *gin.Context) {
	ext := c.Param("ext")
	if ext == "" {
		c.JSON(http.StatusBadRequest, Error(400, "扩展名不能为空"))
		return
	}

	assoc, ok := h.manager.GetFileAssociation(ext)
	if !ok {
		c.JSON(http.StatusNotFound, Error(404, "不支持的文件类型"))
		return
	}

	c.JSON(http.StatusOK, Success(assoc))
}

// ========== 健康检查 ==========

// healthCheck 健康检查
// @Summary OnlyOffice 服务健康检查
// @Description 检查 OnlyOffice 服务状态
// @Tags office
// @Produce json
// @Success 200 {object} Response
// @Router /office/health [get]
func (h *Handlers) healthCheck(c *gin.Context) {
	status := gin.H{
		"enabled": h.manager.IsEnabled(),
	}

	// 检查服务器连接
	if h.manager.IsEnabled() {
		if err := h.manager.CheckServer(); err != nil {
			status["server_status"] = "unreachable"
			status["server_error"] = err.Error()
		} else {
			status["server_status"] = "ok"
		}
	}

	// 统计活跃会话
	sessions, _ := h.manager.ListSessions(SessionStatusActive, 1000, 0)
	status["active_sessions"] = len(sessions)

	c.JSON(http.StatusOK, Success(status))
}

// ========== 辅助函数 ==========

func parseInt(s string) (int, error) {
	var result int
	for _, c := range s {
		if c < '0' || c > '9' {
			break
		}
		result = result*10 + int(c-'0')
	}
	return result, nil
}
