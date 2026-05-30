// Package quotaalert 提供配额预警 REST API 处理器
package quotaalert

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 配额预警 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	quota := r.Group("/quotaalert")
	{
		// 配额规则管理
		quota.POST("/rules", h.setQuotaRule)
		quota.GET("/rules/:path", h.getQuotaRule)

		// 使用量查询
		quota.GET("/usage/:userid", h.getUsage)

		// 配额检查
		quota.GET("/check/:userid", h.checkQuota)

		// 趋势预测
		quota.GET("/predict/:userid", h.predictFullDate)

		// 清理建议
		quota.GET("/suggestions/:userid", h.getSuggestions)

		// 告警管理
		quota.GET("/alerts", h.getAlerts)
		quota.POST("/alerts/:id/ack", h.acknowledgeAlert)

		// 全局报告
		quota.GET("/report", h.generateReport)

		// 自动清理
		quota.POST("/cleanup/:userid", h.autoCleanup)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// setQuotaRuleRequest 设置配额规则请求
type setQuotaRuleRequest struct {
	Path              string  `json:"path" binding:"required"`
	UserID            string  `json:"user_id" binding:"required"`
	MaxBytes          int64   `json:"max_bytes" binding:"required"`
	MaxFiles          int64   `json:"max_files"`
	WarnThreshold     float64 `json:"warn_threshold" binding:"required"`
	CriticalThreshold float64 `json:"critical_threshold" binding:"required"`
	Enabled           bool    `json:"enabled"`
}

// setQuotaRule 设置配额规则
func (h *Handlers) setQuotaRule(c *gin.Context) {
	var req setQuotaRuleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "请求参数无效: " + err.Error(),
		})
		return
	}

	rule := QuotaRule{
		Path:              req.Path,
		UserID:            req.UserID,
		MaxBytes:          req.MaxBytes,
		MaxFiles:          req.MaxFiles,
		WarnThreshold:     req.WarnThreshold,
		CriticalThreshold: req.CriticalThreshold,
		Enabled:           req.Enabled,
	}

	if err := h.manager.SetQuota(c.Request.Context(), rule); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "配额规则已设置",
		Data:    rule,
	})
}

// getQuotaRule 获取配额规则
func (h *Handlers) getQuotaRule(c *gin.Context) {
	path := c.Param("path")
	userID := c.Query("user_id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "缺少 user_id 参数",
		})
		return
	}

	rule, err := h.manager.GetQuota(c.Request.Context(), userID, path)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    rule,
	})
}

// getUsage 获取使用量
func (h *Handlers) getUsage(c *gin.Context) {
	userID := c.Param("userid")
	path := c.Query("path")

	if path == "" {
		// 返回用户所有路径的使用量
		h.manager.mu.RLock()
		usages := make([]*QuotaUsage, 0)
		for key, usage := range h.manager.usages {
			if len(key) > len(userID)+1 && key[:len(userID)+1] == userID+":" {
				usages = append(usages, usage)
			}
		}
		h.manager.mu.RUnlock()

		c.JSON(http.StatusOK, response{
			Code:    0,
			Message: "success",
			Data:    usages,
		})
		return
	}

	// 返回指定路径的使用量
	key := ruleKey(userID, path)
	h.manager.mu.RLock()
	usage, ok := h.manager.usages[key]
	h.manager.mu.RUnlock()

	if !ok {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: "暂无使用量数据",
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    usage,
	})
}

// checkQuota 检查配额
func (h *Handlers) checkQuota(c *gin.Context) {
	userID := c.Param("userid")
	path := c.Query("path")

	if path == "" {
		// 检查用户所有路径
		h.manager.mu.RLock()
		rules := make([]*QuotaRule, 0)
		for key, rule := range h.manager.rules {
			if len(key) > len(userID)+1 && key[:len(userID)+1] == userID+":" {
				rules = append(rules, rule)
			}
		}
		h.manager.mu.RUnlock()

		alerts := make([]*QuotaAlert, 0)
		for _, rule := range rules {
			alert, err := h.manager.CheckQuota(c.Request.Context(), rule.UserID, rule.Path)
			if err == nil && alert != nil {
				alerts = append(alerts, alert)
			}
		}

		c.JSON(http.StatusOK, response{
			Code:    0,
			Message: "success",
			Data:    alerts,
		})
		return
	}

	// 检查指定路径
	alert, err := h.manager.CheckQuota(c.Request.Context(), userID, path)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    alert,
	})
}

// predictFullDate 预测用满日期
func (h *Handlers) predictFullDate(c *gin.Context) {
	userID := c.Param("userid")
	path := c.Query("path")

	if path == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "缺少 path 参数",
		})
		return
	}

	trend, err := h.manager.PredictFullDate(c.Request.Context(), userID, path)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    trend,
	})
}

// getSuggestions 获取清理建议
func (h *Handlers) getSuggestions(c *gin.Context) {
	userID := c.Param("userid")
	path := c.Query("path")

	if path == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "缺少 path 参数",
		})
		return
	}

	suggestions, err := h.manager.GenerateCleanupSuggestions(c.Request.Context(), userID, path)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    suggestions,
	})
}

// getAlerts 获取告警列表
func (h *Handlers) getAlerts(c *gin.Context) {
	userID := c.Query("user_id")
	unacknowledgedOnly := c.Query("unacknowledged") == "true"

	alerts := h.manager.GetAlerts(c.Request.Context(), userID, unacknowledgedOnly)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    alerts,
	})
}

// acknowledgeAlert 确认告警
func (h *Handlers) acknowledgeAlert(c *gin.Context) {
	alertID := c.Param("id")

	if err := h.manager.AcknowledgeAlert(c.Request.Context(), alertID); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "告警已确认",
	})
}

// generateReport 生成全局报告
func (h *Handlers) generateReport(c *gin.Context) {
	report, err := h.manager.GenerateReport(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    report,
	})
}

// autoCleanup 自动清理
func (h *Handlers) autoCleanup(c *gin.Context) {
	userID := c.Param("userid")

	cleaned, err := h.manager.AutoCleanup(c.Request.Context(), userID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "清理完成",
		Data: map[string]int64{
			"cleaned_bytes": cleaned,
		},
	})
}
