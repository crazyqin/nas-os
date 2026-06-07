package smartalert

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 引导式告警 API 处理器.
type Handler struct {
	engine *Engine
	logger *zap.Logger
}

// NewHandler 创建处理器.
func NewHandler(engine *Engine, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{engine: engine, logger: logger}
}

// RegisterRoutes 注册路由到 gin.RouterGroup.
//
// 注册的路由：
//
//	GET    /smartalert/list                   - 告警列表
//	GET    /smartalert/:id/guide              - 获取告警处置引导
//	POST   /smartalert/:id/acknowledge        - 确认告警
//	POST   /smartalert/silence                - 创建静默规则
//	GET    /smartalert/silence                - 列出静默规则
//	DELETE /smartalert/silence/:id            - 删除静默规则
//	POST   /smartalert/:id/resolve            - 解决告警
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	sa := r.Group("/smartalert")
	{
		sa.GET("/list", h.listAlerts)
		sa.GET("/:id/guide", h.getGuide)
		sa.POST("/:id/acknowledge", h.acknowledge)
		sa.POST("/:id/resolve", h.resolveAlert)
		sa.POST("/silence", h.createSilence)
		sa.GET("/silence", h.listSilences)
		sa.DELETE("/silence/:id", h.removeSilence)
	}
}

// apiResp 标准响应.
type apiResp struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listAlerts 获取告警列表.
// GET /api/v1/smartalert/list?category=disk&severity=critical&state=active
func (h *Handler) listAlerts(c *gin.Context) {
	var q ListQuery
	_ = c.ShouldBindQuery(&q)

	alerts := h.engine.List(q)

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(alerts),
			"alerts": alerts,
		},
	})
}

// getGuide 获取告警处置引导.
// GET /api/v1/smartalert/:id/guide
func (h *Handler) getGuide(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, apiResp{Code: 400, Message: "missing alert id"})
		return
	}

	guide, err := h.engine.GetGuide(id)
	if err != nil {
		c.JSON(http.StatusNotFound, apiResp{Code: 404, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "success",
		Data:    guide,
	})
}

// acknowledge 确认告警.
// POST /api/v1/smartalert/:id/acknowledge
func (h *Handler) acknowledge(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, apiResp{Code: 400, Message: "missing alert id"})
		return
	}

	var req AcknowledgeRequest
	_ = c.ShouldBindJSON(&req)
	if req.Operator == "" {
		req.Operator = "admin"
	}

	if err := h.engine.Acknowledge(id, req.Operator); err != nil {
		c.JSON(http.StatusNotFound, apiResp{Code: 404, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "告警已确认",
	})
}

// resolveAlert 解决告警.
// POST /api/v1/smartalert/:id/resolve
func (h *Handler) resolveAlert(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, apiResp{Code: 400, Message: "missing alert id"})
		return
	}

	if err := h.engine.Resolve(id); err != nil {
		c.JSON(http.StatusNotFound, apiResp{Code: 404, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "告警已解决",
	})
}

// createSilence 创建静默规则.
// POST /api/v1/smartalert/silence
func (h *Handler) createSilence(c *gin.Context) {
	var req SilenceRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResp{Code: 400, Message: "参数错误: " + err.Error()})
		return
	}

	rule := h.engine.AddSilence(req)

	c.JSON(http.StatusCreated, apiResp{
		Code:    0,
		Message: "静默规则已创建",
		Data:    rule,
	})
}

// listSilences 列出静默规则.
// GET /api/v1/smartalert/silence
func (h *Handler) listSilences(c *gin.Context) {
	rules := h.engine.ListSilences()

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(rules),
			"rules": rules,
		},
	})
}

// removeSilence 删除静默规则.
// DELETE /api/v1/smartalert/silence/:id
func (h *Handler) removeSilence(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, apiResp{Code: 400, Message: "missing silence id"})
		return
	}

	if err := h.engine.RemoveSilence(id); err != nil {
		c.JSON(http.StatusNotFound, apiResp{Code: 404, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, apiResp{
		Code:    0,
		Message: "静默规则已删除",
	})
}
