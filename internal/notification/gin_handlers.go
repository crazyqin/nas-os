package notification

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// GinHandler 通知中心 Gin HTTP 处理器.
type GinHandler struct {
	service *Service
}

// NewGinHandler 创建通知中心 Gin 处理器.
func NewGinHandler(s *Service) *GinHandler {
	return &GinHandler{service: s}
}

// RegisterRoutes 注册路由到 Gin 路由组.
func (h *GinHandler) RegisterRoutes(rg *gin.RouterGroup) {
	notif := rg.Group("/notifications")
	{
		// 发送通知
		notif.POST("", h.SendNotification)
		notif.POST("/send", h.SendNotification)

		// 历史记录
		notif.GET("/history", h.ListHistory)
		notif.GET("/history/:id", h.GetHistoryRecord)
		notif.DELETE("/history/:id", h.DeleteHistoryRecord)
		notif.POST("/history/:id/retry", h.RetryRecord)
		notif.POST("/history/clear", h.ClearHistory)

		// 统计
		notif.GET("/stats", h.GetStats)

		// 渠道管理
		notif.GET("/channels", h.ListChannels)
		notif.POST("/channels", h.AddChannel)
		notif.PUT("/channels/:id", h.UpdateChannel)
		notif.DELETE("/channels/:id", h.DeleteChannel)
		notif.POST("/channels/:id/test", h.TestChannel)

		// 规则管理
		notif.GET("/rules", h.ListRules)
		notif.POST("/rules", h.CreateRule)
		notif.PUT("/rules/:id", h.UpdateRule)
		notif.DELETE("/rules/:id", h.DeleteRule)
		notif.POST("/rules/:id/toggle", h.ToggleRule)
		notif.POST("/rules/test", h.TestRule)

		// 模板管理
		notif.GET("/templates", h.ListTemplates)
		notif.POST("/templates", h.CreateTemplate)
		notif.GET("/templates/:id", h.GetTemplate)
		notif.PUT("/templates/:id", h.UpdateTemplate)
		notif.DELETE("/templates/:id", h.DeleteTemplate)
		notif.POST("/templates/:id/render", h.RenderTemplate)
	}
}

// SendNotification 发送通知
// @Summary 发送通知
// @Description 发送一条新的系统通知，支持多渠道推送
// @Tags notifications
// @Accept json
// @Produce json
// @Param request body SendRequest true "通知内容"
// @Success 200 {object} SendResponse
// @Failure 400 {object} map[string]string
// @Router /api/v1/notifications [post].
func (h *GinHandler) SendNotification(c *gin.Context) {
	var req SendRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body: " + err.Error()})
		return
	}

	if req.Notification == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "notification is required"})
		return
	}
	if req.Notification.Title == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "title is required"})
		return
	}
	if req.Notification.Level == "" {
		req.Notification.Level = LevelInfo
	}

	resp, err := h.service.Send(c.Request.Context(), &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}

// ListHistory 获取历史记录列表.
func (h *GinHandler) ListHistory(c *gin.Context) {
	filter := &HistoryFilter{
		Level:    Level(c.Query("level")),
		Category: c.Query("category"),
		Source:   c.Query("source"),
		Search:   c.Query("search"),
	}

	if status := c.Query("status"); status != "" {
		filter.Status = Status(status)
	}

	if limit := c.Query("limit"); limit != "" {
		if l, err := strconv.Atoi(limit); err == nil && l > 0 {
			filter.PageSize = l
		}
	}

	offset := 0
	if o := c.Query("offset"); o != "" {
		if v, err := strconv.Atoi(o); err == nil && v > 0 {
			offset = v
		}
	}
	if filter.PageSize > 0 {
		filter.Page = offset/filter.PageSize + 1
	}

	histMgr := h.service.GetHistoryManager()
	records := histMgr.Query(filter)

	c.JSON(http.StatusOK, gin.H{
		"records": records,
		"total":   len(records),
	})
}

// GetHistoryRecord 获取单条历史记录.
func (h *GinHandler) GetHistoryRecord(c *gin.Context) {
	id := c.Param("id")
	histMgr := h.service.GetHistoryManager()
	record, err := histMgr.GetRecord(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, record)
}

// DeleteHistoryRecord 删除历史记录.
func (h *GinHandler) DeleteHistoryRecord(c *gin.Context) {
	id := c.Param("id")
	histMgr := h.service.GetHistoryManager()
	if err := histMgr.DeleteRecord(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// RetryRecord 重试失败的记录.
func (h *GinHandler) RetryRecord(c *gin.Context) {
	id := c.Param("id")
	if err := h.service.RetryFailed(id); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "retrying"})
}

// ClearHistory 清除历史记录.
func (h *GinHandler) ClearHistory(c *gin.Context) {
	histMgr := h.service.GetHistoryManager()
	before := histMgr.Count()
	if err := histMgr.Clear(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"status":  "cleared",
		"removed": before,
	})
}

// GetStats 获取统计信息.
func (h *GinHandler) GetStats(c *gin.Context) {
	var startTime, endTime *time.Time

	if start := c.Query("start"); start != "" {
		if t, err := time.Parse(time.RFC3339, start); err == nil {
			startTime = &t
		}
	}
	if end := c.Query("end"); end != "" {
		if t, err := time.Parse(time.RFC3339, end); err == nil {
			endTime = &t
		}
	}

	stats := h.service.GetStats(startTime, endTime)
	c.JSON(http.StatusOK, stats)
}

// ListChannels 获取渠道列表.
func (h *GinHandler) ListChannels(c *gin.Context) {
	cm := h.service.GetChannelManager()
	channelType := ChannelType(c.Query("type"))

	var channels []*ChannelConfig
	if channelType != "" {
		channels = cm.ListChannels(channelType)
	} else {
		channels = cm.ListChannels("")
	}

	c.JSON(http.StatusOK, gin.H{"channels": channels})
}

// AddChannel 添加渠道.
func (h *GinHandler) AddChannel(c *gin.Context) {
	var config ChannelConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if err := h.service.AddChannel(&config); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, config)
}

// UpdateChannel 更新渠道.
func (h *GinHandler) UpdateChannel(c *gin.Context) {
	id := c.Param("id")
	var config ChannelConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	config.ID = id

	if err := h.service.UpdateChannel(&config); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, config)
}

// DeleteChannel 删除渠道.
func (h *GinHandler) DeleteChannel(c *gin.Context) {
	id := c.Param("id")
	cm := h.service.GetChannelManager()
	if err := cm.RemoveChannel(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// TestChannel 测试渠道.
func (h *GinHandler) TestChannel(c *gin.Context) {
	id := c.Param("id")
	cm := h.service.GetChannelManager()
	channel, err := cm.GetChannel(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	testNotif := &Notification{
		ID:       "test-" + strconv.FormatInt(time.Now().UnixMilli(), 10),
		Title:    "渠道测试",
		Message:  "这是一条测试通知，用于验证渠道配置是否正确。",
		Level:    LevelInfo,
		Source:   "notification-center",
		Category: "test",
	}

	registry := NewSenderRegistry()
	sender, ok := registry.Get(channel.Type)
	if !ok || sender == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "unsupported channel type: " + string(channel.Type)})
		return
	}

	if err := sender.Send(channel, testNotif); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "test failed: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"status": "test_sent"})
}

// ListRules 获取规则列表.
func (h *GinHandler) ListRules(c *gin.Context) {
	re := h.service.GetRuleEngine()
	rules := re.ListRules()
	c.JSON(http.StatusOK, gin.H{"rules": rules})
}

// CreateRule 创建规则.
func (h *GinHandler) CreateRule(c *gin.Context) {
	var rule Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	re := h.service.GetRuleEngine()
	if err := re.CreateRule(&rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, rule)
}

// UpdateRule 更新规则.
func (h *GinHandler) UpdateRule(c *gin.Context) {
	id := c.Param("id")
	var rule Rule
	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	rule.ID = id

	re := h.service.GetRuleEngine()
	if err := re.UpdateRule(&rule); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rule)
}

// DeleteRule 删除规则.
func (h *GinHandler) DeleteRule(c *gin.Context) {
	id := c.Param("id")
	re := h.service.GetRuleEngine()
	if err := re.DeleteRule(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// ToggleRule 切换规则启用状态.
func (h *GinHandler) ToggleRule(c *gin.Context) {
	id := c.Param("id")
	re := h.service.GetRuleEngine()
	rule, err := re.GetRule(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	rule.Enabled = !rule.Enabled
	if err := re.UpdateRule(rule); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rule)
}

// TestRule 测试规则.
func (h *GinHandler) TestRule(c *gin.Context) {
	var req struct {
		Rule         *Rule         `json:"rule"`
		Notification *Notification `json:"notification"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	if req.Rule == nil || req.Notification == nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "rule and notification are required"})
		return
	}

	re := h.service.GetRuleEngine()
	result := re.TestRule(req.Rule, req.Notification)
	c.JSON(http.StatusOK, result)
}

// ListTemplates 获取模板列表.
func (h *GinHandler) ListTemplates(c *gin.Context) {
	tm := h.service.GetTemplateManager()
	category := c.Query("category")

	var templates []*Template
	if category != "" {
		templates = tm.List(category)
	} else {
		templates = tm.List("")
	}

	c.JSON(http.StatusOK, gin.H{"templates": templates})
}

// CreateTemplate 创建模板.
func (h *GinHandler) CreateTemplate(c *gin.Context) {
	var template Template
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	tm := h.service.GetTemplateManager()
	if err := tm.Create(&template); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, template)
}

// GetTemplate 获取模板详情.
func (h *GinHandler) GetTemplate(c *gin.Context) {
	id := c.Param("id")
	tm := h.service.GetTemplateManager()
	template, err := tm.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, template)
}

// UpdateTemplate 更新模板.
func (h *GinHandler) UpdateTemplate(c *gin.Context) {
	id := c.Param("id")
	var template Template
	if err := c.ShouldBindJSON(&template); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}
	template.ID = id

	tm := h.service.GetTemplateManager()
	if err := tm.Update(&template); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, template)
}

// DeleteTemplate 删除模板.
func (h *GinHandler) DeleteTemplate(c *gin.Context) {
	id := c.Param("id")
	tm := h.service.GetTemplateManager()
	if err := tm.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "deleted"})
}

// RenderTemplate 渲染模板.
func (h *GinHandler) RenderTemplate(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Variables map[string]interface{} `json:"variables"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	tm := h.service.GetTemplateManager()
	rendered, err := tm.Render(id, req.Variables)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, rendered)
}
