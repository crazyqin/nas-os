// Package storageqos - QoS HTTP处理器
// 注册到 /api/v1/storageqos/ 路由
package storageqos

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler QoS HTTP处理器
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建QoS HTTP处理器
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{manager: manager, logger: logger}
}

// RegisterRoutes 注册QoS路由到 /api/v1/storageqos/
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	qos := rg.Group("/storageqos")
	{
		// 策略管理
		qos.POST("/policies", h.CreatePolicy)
		qos.GET("/policies", h.ListPolicies)
		qos.GET("/policies/:id", h.GetPolicy)
		qos.PUT("/policies/:id", h.UpdatePolicy)
		qos.DELETE("/policies/:id", h.DeletePolicy)

		// 指标监控
		qos.GET("/policies/:id/metrics", h.GetMetrics)
		qos.GET("/policies/:id/metrics/history", h.GetMetricsHistory)

		// 系统统计
		qos.GET("/stats", h.GetStats)
	}
}

// CreatePolicy handles POST /api/v1/storageqos/policies
func (h *Handler) CreatePolicy(c *gin.Context) {
	var req CreatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy, err := h.manager.CreatePolicy(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, policy)
}

// ListPolicies handles GET /api/v1/storageqos/policies
func (h *Handler) ListPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, PolicyListResponse{
		Policies: policies,
		Total:    len(policies),
	})
}

// GetPolicy handles GET /api/v1/storageqos/policies/:id
func (h *Handler) GetPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, policy)
}

// UpdatePolicy handles PUT /api/v1/storageqos/policies/:id
func (h *Handler) UpdatePolicy(c *gin.Context) {
	id := c.Param("id")
	var req UpdatePolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	policy, err := h.manager.UpdatePolicy(c.Request.Context(), id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, policy)
}

// DeletePolicy handles DELETE /api/v1/storageqos/policies/:id
func (h *Handler) DeletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePolicy(c.Request.Context(), id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "QoS策略已删除"})
}

// GetMetrics handles GET /api/v1/storageqos/policies/:id/metrics
func (h *Handler) GetMetrics(c *gin.Context) {
	id := c.Param("id")

	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	metrics, err := h.manager.GetMetrics(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, MetricsResponse{
		PolicyID: id,
		Target:   policy.Target,
		Metrics:  *metrics,
	})
}

// GetMetricsHistory handles GET /api/v1/storageqos/policies/:id/metrics/history
func (h *Handler) GetMetricsHistory(c *gin.Context) {
	id := c.Param("id")

	fromStr := c.DefaultQuery("from", time.Now().Add(-24*time.Hour).Format(time.RFC3339))
	toStr := c.DefaultQuery("to", time.Now().Format(time.RFC3339))

	from, err := time.Parse(time.RFC3339, fromStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的from参数，需RFC3339格式"})
		return
	}
	to, err := time.Parse(time.RFC3339, toStr)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的to参数，需RFC3339格式"})
		return
	}

	history, err := h.manager.GetMetricsHistory(id, from, to)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, history)
}

// GetStats handles GET /api/v1/storageqos/stats
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}
