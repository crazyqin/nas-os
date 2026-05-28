package smartalerttriage

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 智能告警分类 API 处理器.
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建处理器.
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes 注册路由到 gin.RouterGroup.
//
// 注册的路由：
//   POST   /smartalerttriage/ingest              - 接收告警
//   GET    /smartalerttriage/list                - 告警列表
//   GET    /smartalerttriage/:id                 - 获取告警详情
//   POST   /smartalerttriage/:id/acknowledge     - 确认告警
//   POST   /smartalerttriage/:id/resolve         - 解决告警
//   GET    /smartalerttriage/stats               - 告警统计
//   GET    /smartalerttriage/trend               - 告警趋势
//   GET    /smartalerttriage/groups              - 聚合组列表
//   GET    /smartalerttriage/rootcause/:id       - 根因详情
//   POST   /smartalerttriage/suppression         - 创建抑制规则
//   GET    /smartalerttriage/suppression          - 列出抑制规则
//   DELETE /smartalerttriage/suppression/:id      - 删除抑制规则
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	triage := r.Group("/smartalerttriage")
	{
		triage.POST("/ingest", h.ingestAlert)
		triage.GET("/list", h.listAlerts)
		triage.GET("/:id", h.getAlert)
		triage.POST("/:id/acknowledge", h.acknowledge)
		triage.POST("/:id/resolve", h.resolveAlert)
		triage.GET("/stats", h.getStats)
		triage.GET("/trend", h.getTrend)
		triage.GET("/groups", h.listGroups)
		triage.GET("/rootcause/:id", h.getRootCause)
		triage.POST("/suppression", h.createSuppression)
		triage.GET("/suppression", h.listSuppressions)
		triage.DELETE("/suppression/:id", h.removeSuppression)
	}
}

// apiResp 标准响应.
type apiResp struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// ingestAlert 接收告警.
// POST /api/v1/smartalerttriage/ingest
func (h *Handler) ingestAlert(c *gin.Context) {
	var req ClassifyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResp{Code: 400, Message: "参数错误: " + err.Error()})
		return
	}

	alert := h.manager.Ingest(req.Title, req.Description, req.Source, req.Resource, req.Labels)

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "告警已接收",
		Data:    alert,
	})
}

// listAlerts 获取告警列表.
// GET /api/v1/smartalerttriage/list?category=storage&priority=critical&state=active
func (h *Handler) listAlerts(c *gin.Context) {
	var q ListQuery
	_ = c.ShouldBindQuery(&q)

	alerts := h.manager.List(q)

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(alerts),
			"alerts": alerts,
		},
	})
}

// getAlert 获取告警详情.
// GET /api/v1/smartalerttriage/:id
func (h *Handler) getAlert(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, apiResp{Code: 400, Message: "缺少告警ID"})
		return
	}

	alert, err := h.manager.Get(id)
	if err != nil {
		c.JSON(http.StatusNotFound, apiResp{Code: 404, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "success",
		Data:    alert,
	})
}

// acknowledge 确认告警.
// POST /api/v1/smartalerttriage/:id/acknowledge
func (h *Handler) acknowledge(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, apiResp{Code: 400, Message: "缺少告警ID"})
		return
	}

	var req AcknowledgeRequest
	_ = c.ShouldBindJSON(&req)
	if req.Operator == "" {
		req.Operator = "admin"
	}

	if err := h.manager.Acknowledge(id, req.Operator); err != nil {
		c.JSON(http.StatusNotFound, apiResp{Code: 404, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "告警已确认",
	})
}

// resolveAlert 解决告警.
// POST /api/v1/smartalerttriage/:id/resolve
func (h *Handler) resolveAlert(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, apiResp{Code: 400, Message: "缺少告警ID"})
		return
	}

	if err := h.manager.Resolve(id); err != nil {
		c.JSON(http.StatusNotFound, apiResp{Code: 404, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "告警已解决",
	})
}

// getStats 获取告警统计.
// GET /api/v1/smartalerttriage/stats?hours=24
func (h *Handler) getStats(c *gin.Context) {
	hoursStr := c.DefaultQuery("hours", "24")
	hours, err := strconv.Atoi(hoursStr)
	if err != nil || hours <= 0 {
		hours = 24
	}

	stats := h.manager.GetStats(hours)

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}

// getTrend 获取告警趋势.
// GET /api/v1/smartalerttriage/trend?hours=24&points=24
func (h *Handler) getTrend(c *gin.Context) {
	hoursStr := c.DefaultQuery("hours", "24")
	hours, err := strconv.Atoi(hoursStr)
	if err != nil || hours <= 0 {
		hours = 24
	}

	pointsStr := c.DefaultQuery("points", "24")
	points, err := strconv.Atoi(pointsStr)
	if err != nil || points <= 0 {
		points = 24
	}

	trend := h.manager.GetTrend(hours, points)

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"hours":  hours,
			"points": trend,
		},
	})
}

// listGroups 列出聚合组.
// GET /api/v1/smartalerttriage/groups
func (h *Handler) listGroups(c *gin.Context) {
	groups := h.manager.ListGroups()

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(groups),
			"groups": groups,
		},
	})
}

// getRootCause 获取根因详情.
// GET /api/v1/smartalerttriage/rootcause/:id
func (h *Handler) getRootCause(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, apiResp{Code: 400, Message: "缺少根因ID"})
		return
	}

	rc, err := h.manager.GetRootCause(id)
	if err != nil {
		c.JSON(http.StatusNotFound, apiResp{Code: 404, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "success",
		Data:    rc,
	})
}

// createSuppression 创建抑制规则.
// POST /api/v1/smartalerttriage/suppression
func (h *Handler) createSuppression(c *gin.Context) {
	var req SuppressRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResp{Code: 400, Message: "参数错误: " + err.Error()})
		return
	}

	rule := h.manager.AddSuppression(req)

	c.JSON(http.StatusCreated, apiResp{
		Code:    0,
		Message: "抑制规则已创建",
		Data:    rule,
	})
}

// listSuppressions 列出抑制规则.
// GET /api/v1/smartalerttriage/suppression
func (h *Handler) listSuppressions(c *gin.Context) {
	rules := h.manager.ListSuppressions()

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(rules),
			"rules": rules,
		},
	})
}

// removeSuppression 删除抑制规则.
// DELETE /api/v1/smartalerttriage/suppression/:id
func (h *Handler) removeSuppression(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, apiResp{Code: 400, Message: "缺少规则ID"})
		return
	}

	if err := h.manager.RemoveSuppression(id); err != nil {
		c.JSON(http.StatusNotFound, apiResp{Code: 404, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "抑制规则已删除",
	})
}
