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

		// ========== 新增：容量预测 ==========
		quotaGroup.GET("/predict", h.predictAllUsage)
		quotaGroup.GET("/predict/:id", h.predictUsage)
		quotaGroup.GET("/history/:target", h.getUsageHistory)
		quotaGroup.GET("/forecast/config", h.getForecastConfig)
		quotaGroup.PUT("/forecast/config", h.setForecastConfig)

		// ========== 新增：告警规则管理 ==========
		quotaGroup.GET("/alert-rules", h.listAlertRules)
		quotaGroup.POST("/alert-rules", h.createAlertRule)
		quotaGroup.GET("/alert-rules/:id", h.getAlertRule)
		quotaGroup.PUT("/alert-rules/:id", h.updateAlertRule)
		quotaGroup.DELETE("/alert-rules/:id", h.deleteAlertRule)
		quotaGroup.POST("/alert-rules/init-default", h.initDefaultAlertRules)
		quotaGroup.GET("/alert-rules/stats", h.getAlertRuleStats)

		// ========== 新增：增强告警检查 ==========
		quotaGroup.POST("/check-and-alert", h.checkAndAlertWithRules)
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
	config := h.manager.GetForecastConfig()
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

// ========== 容量预测 ==========

// predictAllUsage 预测所有配额使用趋势
// @Summary 预测所有配额使用趋势
// @Description 预测所有配额规则的使用趋势
// @Tags quota
// @Success 200 {object} api.Response{data=[]PredictionResult}
// @Router /api/v1/quota/predict [get]
func (h *Handlers) predictAllUsage(c *gin.Context) {
	results := h.manager.PredictAllUsage()
	api.OK(c, results)
}

// predictUsage 预测指定配额使用趋势
// @Summary 预测指定配额使用趋势
// @Description 预测指定配额规则的使用趋势
// @Tags quota
// @Param id path string true "规则ID"
// @Success 200 {object} api.Response{data=PredictionResult}
// @Failure 404 {object} api.Response
// @Router /api/v1/quota/predict/{id} [get]
func (h *Handlers) predictUsage(c *gin.Context) {
	id := c.Param("id")

	result, err := h.manager.PredictUsage(id)
	if err != nil {
		if err == ErrRuleNotFound {
			api.NotFound(c, "配额规则不存在")
		} else if err == ErrInsufficientData {
			api.BadRequest(c, "历史数据不足以进行预测")
		} else {
			api.InternalError(c, "预测失败: "+err.Error())
		}
		return
	}

	api.OK(c, result)
}

// getUsageHistory 获取使用历史
// @Summary 获取使用历史
// @Description 获取指定目标的使用历史数据
// @Tags quota
// @Param target path string true "目标ID"
// @Success 200 {object} api.Response{data=[]UsageHistory}
// @Router /api/v1/quota/history/{target} [get]
func (h *Handlers) getUsageHistory(c *gin.Context) {
	target := c.Param("target")

	history := h.manager.GetUsageHistory(target)
	api.OK(c, history)
}

// getForecastConfig 获取预测配置
// @Summary 获取预测配置
// @Description 获取容量预测配置
// @Tags quota
// @Success 200 {object} api.Response{data=ForecastConfig}
// @Router /api/v1/quota/forecast/config [get]
func (h *Handlers) getForecastConfig(c *gin.Context) {
	config := h.manager.GetForecastConfig()
	api.OK(c, config)
}

// setForecastConfig 设置预测配置
// @Summary 设置预测配置
// @Description 设置容量预测配置
// @Tags quota
// @Accept json
// @Param request body ForecastConfig true "预测配置"
// @Success 200 {object} api.Response
// @Router /api/v1/quota/forecast/config [put]
func (h *Handlers) setForecastConfig(c *gin.Context) {
	var config ForecastConfig
	if err := api.BindAndValidate(c, &config); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 验证配置
	if config.HistoryDays <= 0 {
		api.BadRequest(c, "历史天数必须大于0")
		return
	}
	if config.MinDataPoints <= 0 {
		api.BadRequest(c, "最小数据点必须大于0")
		return
	}

	h.manager.SetForecastConfig(config)
	api.OK(c, gin.H{"message": "预测配置已更新"})
}

// ========== 告警规则管理 ==========

// listAlertRules 列出告警规则
// @Summary 列出告警规则
// @Description 列出所有告警规则
// @Tags quota
// @Success 200 {object} api.Response{data=[]AlertRule}
// @Router /api/v1/quota/alert-rules [get]
func (h *Handlers) listAlertRules(c *gin.Context) {
	mgr := h.manager.GetAlertRuleManager()
	if mgr == nil {
		api.InternalError(c, "告警规则管理器未初始化")
		return
	}

	rules := mgr.ListRules()
	api.OK(c, rules)
}

// createAlertRule 创建告警规则
// @Summary 创建告警规则
// @Description 创建新的告警规则
// @Tags quota
// @Accept json
// @Param request body AlertRuleInput true "创建请求"
// @Success 201 {object} api.Response{data=AlertRule}
// @Failure 400 {object} api.Response
// @Router /api/v1/quota/alert-rules [post]
func (h *Handlers) createAlertRule(c *gin.Context) {
	mgr := h.manager.GetAlertRuleManager()
	if mgr == nil {
		api.InternalError(c, "告警规则管理器未初始化")
		return
	}

	var input AlertRuleInput
	if err := api.BindAndValidate(c, &input); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	// 设置默认启用
	if !input.Enabled {
		input.Enabled = true
	}

	rule, err := mgr.CreateRule(input)
	if err != nil {
		switch err {
		case ErrInvalidThreshold:
			api.BadRequest(c, "无效的阈值")
		default:
			api.InternalError(c, "创建告警规则失败: "+err.Error())
		}
		return
	}

	api.Created(c, rule)
}

// getAlertRule 获取告警规则
// @Summary 获取告警规则
// @Description 获取指定告警规则详情
// @Tags quota
// @Param id path string true "规则ID"
// @Success 200 {object} api.Response{data=AlertRule}
// @Failure 404 {object} api.Response
// @Router /api/v1/quota/alert-rules/{id} [get]
func (h *Handlers) getAlertRule(c *gin.Context) {
	mgr := h.manager.GetAlertRuleManager()
	if mgr == nil {
		api.InternalError(c, "告警规则管理器未初始化")
		return
	}

	id := c.Param("id")

	rule, err := mgr.GetRule(id)
	if err != nil {
		api.NotFound(c, "告警规则不存在")
		return
	}

	api.OK(c, rule)
}

// updateAlertRule 更新告警规则
// @Summary 更新告警规则
// @Description 更新告警规则
// @Tags quota
// @Accept json
// @Param id path string true "规则ID"
// @Param request body AlertRuleInput true "更新请求"
// @Success 200 {object} api.Response{data=AlertRule}
// @Failure 400,404 {object} api.Response
// @Router /api/v1/quota/alert-rules/{id} [put]
func (h *Handlers) updateAlertRule(c *gin.Context) {
	mgr := h.manager.GetAlertRuleManager()
	if mgr == nil {
		api.InternalError(c, "告警规则管理器未初始化")
		return
	}

	id := c.Param("id")

	var input AlertRuleInput
	if err := api.BindAndValidate(c, &input); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	rule, err := mgr.UpdateRule(id, input)
	if err != nil {
		switch err {
		case ErrAlertRuleNotFound:
			api.NotFound(c, "告警规则不存在")
		case ErrInvalidThreshold:
			api.BadRequest(c, "无效的阈值")
		default:
			api.InternalError(c, "更新告警规则失败: "+err.Error())
		}
		return
	}

	api.OK(c, rule)
}

// deleteAlertRule 删除告警规则
// @Summary 删除告警规则
// @Description 删除告警规则
// @Tags quota
// @Param id path string true "规则ID"
// @Success 204 "删除成功"
// @Failure 404 {object} api.Response
// @Router /api/v1/quota/alert-rules/{id} [delete]
func (h *Handlers) deleteAlertRule(c *gin.Context) {
	mgr := h.manager.GetAlertRuleManager()
	if mgr == nil {
		api.InternalError(c, "告警规则管理器未初始化")
		return
	}

	id := c.Param("id")

	if err := mgr.DeleteRule(id); err != nil {
		api.NotFound(c, "告警规则不存在")
		return
	}

	c.Status(http.StatusNoContent)
}

// initDefaultAlertRules 初始化默认告警规则
// @Summary 初始化默认告警规则
// @Description 初始化默认的告警规则（60%、80%、90%、95%阈值）
// @Tags quota
// @Success 200 {object} api.Response
// @Router /api/v1/quota/alert-rules/init-default [post]
func (h *Handlers) initDefaultAlertRules(c *gin.Context) {
	mgr := h.manager.GetAlertRuleManager()
	if mgr == nil {
		api.InternalError(c, "告警规则管理器未初始化")
		return
	}

	err := mgr.InitDefaultRules()
	if err != nil {
		api.InternalError(c, "初始化默认告警规则失败: "+err.Error())
		return
	}

	api.OK(c, gin.H{"message": "默认告警规则已初始化"})
}

// getAlertRuleStats 获取告警规则统计
// @Summary 获取告警规则统计
// @Description 获取告警规则的统计信息
// @Tags quota
// @Success 200 {object} api.Response{data=map[string]interface{}}
// @Router /api/v1/quota/alert-rules/stats [get]
func (h *Handlers) getAlertRuleStats(c *gin.Context) {
	mgr := h.manager.GetAlertRuleManager()
	if mgr == nil {
		api.InternalError(c, "告警规则管理器未初始化")
		return
	}

	stats := mgr.GetAlertStats()
	api.OK(c, stats)
}

// ========== 增强告警检查 ==========

// checkAndAlertWithRules 使用告警规则进行检查
// @Summary 使用告警规则进行检查
// @Description 检查所有配额规则并使用告警规则生成告警
// @Tags quota
// @Success 200 {object} api.Response{data=[]Alert}
// @Router /api/v1/quota/check-and-alert [post]
func (h *Handlers) checkAndAlertWithRules(c *gin.Context) {
	alerts := h.manager.CheckAndAlertWithRules()
	api.OK(c, alerts)
}
