// Package quota 提供存储配额管理和告警功能
package quota

import (
	"net/http"
	"strconv"

	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 配额管理 HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{
		manager: mgr,
	}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	quotaGroup := r.Group("/quota")
	{
		// 配额规则管理
		quotaGroup.GET("/rules", h.listRules)
		quotaGroup.POST("/rules", h.createRule)
		quotaGroup.GET("/rules/:id", h.getRule)
		quotaGroup.PUT("/rules/:id", h.updateRule)
		quotaGroup.DELETE("/rules/:id", h.deleteRule)

		// 配额使用情况
		quotaGroup.GET("/usage", h.getAllUsage)
		quotaGroup.GET("/usage/:id", h.getUsage)

		// 告警管理
		quotaGroup.GET("/alerts", h.getAlerts)
		quotaGroup.GET("/alerts/history", h.getAlertHistory)
		quotaGroup.POST("/alerts/:id/resolve", h.resolveAlert)

		// 通知配置
		quotaGroup.GET("/notify/config", h.getNotifyConfig)
		quotaGroup.PUT("/notify/config", h.setNotifyConfig)

		// 配额检查
		quotaGroup.POST("/check", h.checkQuota)
	}
}

// ========== 配额规则 ==========

// listRules 列出配额规则
// @Summary 列出配额规则
// @Description 列出所有配额规则
// @Tags quota
// @Success 200 {object} api.Response{data=[]QuotaRule}
// @Router /api/v1/quota/rules [get]
func (h *Handlers) listRules(c *gin.Context) {
	rules := h.manager.ListRules()
	api.OK(c, rules)
}

// createRule 创建配额规则
// @Summary 创建配额规则
// @Description 创建新的配额规则
// @Tags quota
// @Accept json
// @Param request body QuotaRuleInput true "创建请求"
// @Success 201 {object} api.Response{data=QuotaRule}
// @Failure 400 {object} api.Response
// @Router /api/v1/quota/rules [post]
func (h *Handlers) createRule(c *gin.Context) {
	var input QuotaRuleInput
	if err := api.BindAndValidate(c, &input); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 设置默认启用
	if !input.Enabled {
		input.Enabled = true
	}

	rule, err := h.manager.CreateRule(input)
	if err != nil {
		switch err {
		case ErrRuleExists:
			api.BadRequest(c, "配额规则已存在")
		case ErrInvalidTarget:
			api.BadRequest(c, "无效的配额目标")
		case ErrInvalidMaxBytes:
			api.BadRequest(c, "无效的容量限制")
		default:
			api.InternalError(c, "创建配额规则失败: "+err.Error())
		}
		return
	}

	api.Created(c, rule)
}

// getRule 获取配额规则
// @Summary 获取配额规则
// @Description 获取指定配额规则详情
// @Tags quota
// @Param id path string true "规则ID"
// @Success 200 {object} api.Response{data=QuotaRule}
// @Failure 404 {object} api.Response
// @Router /api/v1/quota/rules/{id} [get]
func (h *Handlers) getRule(c *gin.Context) {
	id := c.Param("id")

	rule, err := h.manager.GetRule(id)
	if err != nil {
		api.NotFound(c, "配额规则不存在")
		return
	}

	api.OK(c, rule)
}

// updateRule 更新配额规则
// @Summary 更新配额规则
// @Description 更新配额规则
// @Tags quota
// @Accept json
// @Param id path string true "规则ID"
// @Param request body QuotaRuleInput true "更新请求"
// @Success 200 {object} api.Response{data=QuotaRule}
// @Failure 400,404 {object} api.Response
// @Router /api/v1/quota/rules/{id} [put]
func (h *Handlers) updateRule(c *gin.Context) {
	id := c.Param("id")

	var input QuotaRuleInput
	if err := api.BindAndValidate(c, &input); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	rule, err := h.manager.UpdateRule(id, input)
	if err != nil {
		switch err {
		case ErrRuleNotFound:
			api.NotFound(c, "配额规则不存在")
		case ErrInvalidMaxBytes:
			api.BadRequest(c, "无效的容量限制")
		default:
			api.InternalError(c, "更新配额规则失败: "+err.Error())
		}
		return
	}

	api.OK(c, rule)
}

// deleteRule 删除配额规则
// @Summary 删除配额规则
// @Description 删除配额规则
// @Tags quota
// @Param id path string true "规则ID"
// @Success 204 "删除成功"
// @Failure 404 {object} api.Response
// @Router /api/v1/quota/rules/{id} [delete]
func (h *Handlers) deleteRule(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.DeleteRule(id); err != nil {
		api.NotFound(c, "配额规则不存在")
		return
	}

	c.Status(http.StatusNoContent)
}

// ========== 配额使用 ==========

// getAllUsage 获取所有配额使用情况
// @Summary 获取配额使用情况
// @Description 获取所有配额规则的使用情况
// @Tags quota
// @Success 200 {object} api.Response{data=[]QuotaUsage}
// @Router /api/v1/quota/usage [get]
func (h *Handlers) getAllUsage(c *gin.Context) {
	usage := h.manager.GetAllUsage()
	api.OK(c, usage)
}

// getUsage 获取指定配额使用情况
// @Summary 获取指定配额使用情况
// @Description 获取指定配额规则的使用情况
// @Tags quota
// @Param id path string true "规则ID"
// @Success 200 {object} api.Response{data=QuotaUsage}
// @Failure 404 {object} api.Response
// @Router /api/v1/quota/usage/{id} [get]
func (h *Handlers) getUsage(c *gin.Context) {
	id := c.Param("id")

	usage, err := h.manager.GetUsage(id)
	if err != nil {
		api.NotFound(c, "配额规则不存在")
		return
	}

	api.OK(c, usage)
}

// ========== 告警管理 ==========

// getAlerts 获取活跃告警
// @Summary 获取活跃告警
// @Description 获取当前活跃的配额告警
// @Tags quota
// @Param limit query int false "限制数量"
// @Success 200 {object} api.Response{data=[]Alert}
// @Router /api/v1/quota/alerts [get]
func (h *Handlers) getAlerts(c *gin.Context) {
	alerts := h.manager.GetAlerts()
	api.OK(c, alerts)
}

// getAlertHistory 获取告警历史
// @Summary 获取告警历史
// @Description 获取配额告警历史记录
// @Tags quota
// @Param limit query int false "限制数量"
// @Success 200 {object} api.Response{data=[]Alert}
// @Router /api/v1/quota/alerts/history [get]
func (h *Handlers) getAlertHistory(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)

	history := h.manager.GetAlertHistory(limit)
	api.OK(c, history)
}

// resolveAlert 解决告警
// @Summary 解决告警
// @Description 将告警标记为已解决
// @Tags quota
// @Param id path string true "告警ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /api/v1/quota/alerts/{id}/resolve [post]
func (h *Handlers) resolveAlert(c *gin.Context) {
	id := c.Param("id")

	if err := h.manager.ResolveAlert(id); err != nil {
		api.NotFound(c, "告警不存在")
		return
	}

	api.OK(c, gin.H{"message": "告警已解决"})
}

// ========== 通知配置 ==========

// getNotifyConfig 获取通知配置
// @Summary 获取通知配置
// @Description 获取配额告警通知配置
// @Tags quota
// @Success 200 {object} api.Response{data=NotificationConfig}
// @Router /api/v1/quota/notify/config [get]
func (h *Handlers) getNotifyConfig(c *gin.Context) {
	// 直接返回默认配置（实际应从manager获取）
	config := DefaultNotificationConfig()
	api.OK(c, config)
}

// setNotifyConfig 设置通知配置
// @Summary 设置通知配置
// @Description 设置配额告警通知配置
// @Tags quota
// @Accept json
// @Param request body NotificationConfig true "通知配置"
// @Success 200 {object} api.Response
// @Router /api/v1/quota/notify/config [put]
func (h *Handlers) setNotifyConfig(c *gin.Context) {
	var config NotificationConfig
	if err := api.BindAndValidate(c, &config); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	h.manager.SetNotifyConfig(config)
	api.OK(c, gin.H{"message": "通知配置已更新"})
}

// ========== 配额检查 ==========

// checkQuotaRequest 检查配额请求
type checkQuotaRequest struct {
	TargetType      string `json:"target_type" binding:"required"`
	TargetID        string `json:"target_id" binding:"required"`
	AdditionalBytes int64  `json:"additional_bytes" binding:"required"`
}

// checkQuota 检查配额
// @Summary 检查配额
// @Description 检查是否允许写入指定大小的数据
// @Tags quota
// @Accept json
// @Param request body checkQuotaRequest true "检查请求"
// @Success 200 {object} api.Response{data=map[string]bool}
// @Failure 400 {object} api.Response
// @Router /api/v1/quota/check [post]
func (h *Handlers) checkQuota(c *gin.Context) {
	var req checkQuotaRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	err := h.manager.CheckQuota(req.TargetType, req.TargetID, req.AdditionalBytes)

	api.OK(c, gin.H{
		"allowed": err == nil,
		"message": func() string {
			if err == nil {
				return "配额允许"
			}
			return err.Error()
		}(),
	})
}