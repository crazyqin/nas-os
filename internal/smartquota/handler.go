package smartquota

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler 配额管理HTTP处理器
type Handler struct {
	mgr *QuotaManager
}

// NewHandler 创建处理器
func NewHandler(mgr *QuotaManager) *Handler {
	return &Handler{mgr: mgr}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	quota := rg.Group("/quota")
	{
		quota.GET("/list", h.List)
		quota.POST("/create", h.Create)
		quota.PUT("/:id", h.Update)
		quota.DELETE("/:id", h.Delete)
		quota.GET("/:id/usage", h.Usage)
		quota.GET("/:id/predict", h.Predict)
		quota.GET("/alerts", h.Alerts)
		quota.POST("/templates/apply", h.ApplyTemplate)
		quota.GET("/history", h.History)
		quota.GET("/cleanup-suggestions", h.CleanupSuggestions)
	}
}

// List 获取配额列表
func (h *Handler) List(c *gin.Context) {
	quotas := h.mgr.ListQuotas()
	c.JSON(http.StatusOK, gin.H{
		"total":  len(quotas),
		"quotas": quotas,
	})
}

// Create 创建配额
func (h *Handler) Create(c *gin.Context) {
	var req QuotaConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	q, err := h.mgr.CreateQuota(req)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, q)
}

// Update 更新配额
func (h *Handler) Update(c *gin.Context) {
	id := c.Param("id")
	var req QuotaConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	q, err := h.mgr.UpdateQuota(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, q)
}

// Delete 删除配额
func (h *Handler) Delete(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.DeleteQuota(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "quota deleted"})
}

// Usage 获取使用量详情
func (h *Handler) Usage(c *gin.Context) {
	id := c.Param("id")
	q, err := h.mgr.GetQuota(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	usagePct := float64(0)
	if q.LimitBytes > 0 {
		usagePct = float64(q.UsedBytes) / float64(q.LimitBytes) * 100
	}

	c.JSON(http.StatusOK, gin.H{
		"quotaId":    q.ID,
		"name":       q.Name,
		"level":      q.Level,
		"limitBytes": q.LimitBytes,
		"usedBytes":  q.UsedBytes,
		"freeBytes":  q.LimitBytes - q.UsedBytes,
		"usagePct":   usagePct,
		"policy":     q.Policy,
		"updatedAt":  q.UpdatedAt,
	})
}

// Predict 使用量预测
func (h *Handler) Predict(c *gin.Context) {
	id := c.Param("id")
	pred, err := h.mgr.PredictUsage(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, pred)
}

// Alerts 获取告警列表
func (h *Handler) Alerts(c *gin.Context) {
	var ackedFilter *bool
	if v := c.Query("acked"); v != "" {
		b, _ := strconv.ParseBool(v)
		ackedFilter = &b
	}

	alerts := h.mgr.GetAlerts(ackedFilter)
	c.JSON(http.StatusOK, gin.H{
		"total":  len(alerts),
		"alerts": alerts,
	})
}

// ApplyTemplateRequest 应用模板请求
type ApplyTemplateRequest struct {
	TemplateName string `json:"templateName" binding:"required"`
	OwnerID      string `json:"ownerId" binding:"required"`
	Name         string `json:"name" binding:"required"`
}

// ApplyTemplate 应用配额模板
func (h *Handler) ApplyTemplate(c *gin.Context) {
	var req ApplyTemplateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	q, err := h.mgr.ApplyTemplate(req.TemplateName, req.OwnerID, req.Name)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, q)
}

// History 获取历史统计
func (h *Handler) History(c *gin.Context) {
	id := c.Query("quotaId")
	period := c.Query("period")
	if id == "" || period == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "quotaId and period are required"})
		return
	}

	stats, err := h.mgr.GetHistory(id, period)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// CleanupSuggestions 获取清理建议
func (h *Handler) CleanupSuggestions(c *gin.Context) {
	id := c.Query("quotaId")
	if id == "" {
		// 返回所有配额的清理建议
		quotas := h.mgr.ListQuotas()
		allSuggestions := make(map[string][]CleanupSuggestion)
		for _, q := range quotas {
			suggestions, err := h.mgr.GetCleanupSuggestions(q.ID)
			if err == nil && len(suggestions) > 0 {
				allSuggestions[q.ID] = suggestions
			}
		}
		c.JSON(http.StatusOK, gin.H{"suggestions": allSuggestions})
		return
	}

	suggestions, err := h.mgr.GetCleanupSuggestions(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"quotaId":     id,
		"suggestions": suggestions,
	})
}
