package predictivemaintenance

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	engine *Engine
}

// NewHandler 创建处理器
func NewHandler(engine *Engine) *Handler {
	return &Handler{engine: engine}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	group := r.Group("/predictive-maintenance")
	{
		group.GET("/components", h.ListComponents)
		group.GET("/components/:id", h.GetHealth)
		group.GET("/components/:id/predict", h.Predict)
		group.POST("/check", h.CheckAll)
		group.GET("/schedules", h.ListSchedules)
		group.POST("/schedules", h.CreateSchedule)
	}
}

// ListComponents 列出组件
func (h *Handler) ListComponents(c *gin.Context) {
	comps := h.engine.ListComponents()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": comps})
}

// GetHealth 获取组件健康
func (h *Handler) GetHealth(c *gin.Context) {
	id := c.Param("id")
	comp, err := h.engine.GetHealth(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": comp})
}

// Predict 预测
func (h *Handler) Predict(c *gin.Context) {
	id := c.Param("id")
	pred, err := h.engine.Predict(c.Request.Context(), id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"success": true, "data": pred})
}

// CheckAll 检查所有
func (h *Handler) CheckAll(c *gin.Context) {
	results := h.engine.CheckAll(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"success": true, "data": results})
}

// ListSchedules 列出维护计划
func (h *Handler) ListSchedules(c *gin.Context) {
	schedules := h.engine.ListSchedules()
	c.JSON(http.StatusOK, gin.H{"success": true, "data": schedules})
}

// CreateScheduleReq 创建维护计划请求
type CreateScheduleReq struct {
	ComponentID string `json:"componentId" binding:"required"`
	Type        string `json:"type" binding:"required"`
	Title       string `json:"title" binding:"required"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
}

// CreateSchedule 创建维护计划
func (h *Handler) CreateSchedule(c *gin.Context) {
	var req CreateScheduleReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	sched, err := h.engine.CreateSchedule(req.ComponentID, req.Type, req.Title, req.Description, req.Priority)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"success": false, "error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, gin.H{"success": true, "data": sched})
}
