// Package containerautoupdate 提供 REST API 处理器
package containerautoupdate

import (
	"context"
	"net/http"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 容器自动更新 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	cu := r.Group("/containerautoupdate")
	{
		// 策略管理
		cu.POST("/policies", h.SetPolicy)
		cu.GET("/policies/:id", h.GetPolicy)

		// 更新检查
		cu.GET("/check/:id", h.CheckUpdates)
		cu.GET("/check-all", h.CheckAllUpdates)

		// 更新执行
		cu.POST("/apply/:id", h.ApplyUpdate)
		cu.POST("/rollback/:recordId", h.Rollback)

		// 健康状态
		cu.GET("/health/:id", h.GetHealth)

		// 历史和统计
		cu.GET("/history/:id", h.GetUpdateHistory)
		cu.GET("/stats", h.GetStats)
	}
}

// ========== 策略管理 ==========

// SetPolicy 设置更新策略.
func (h *Handlers) SetPolicy(c *gin.Context) {
	var req SetPolicyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	policy := UpdatePolicy{
		ContainerID:       req.ContainerID,
		ContainerName:     req.ContainerName,
		Enabled:           req.Enabled,
		Schedule:          req.Schedule,
		MaxRetries:        req.MaxRetries,
		RollbackOnFailure: req.RollbackOnFailure,
		HealthCheckURL:    req.HealthCheckURL,
		HealthCheckTimeout: req.HealthCheckTimeout,
		PreUpdateHook:     req.PreUpdateHook,
		PostUpdateHook:    req.PostUpdateHook,
		NotifyOnUpdate:    req.NotifyOnUpdate,
		NotifyOnFailure:   req.NotifyOnFailure,
	}

	result, err := h.manager.SetPolicy(context.Background(), policy)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: result})
}

// GetPolicy 获取更新策略.
func (h *Handlers) GetPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: policy})
}

// ========== 更新检查 ==========

// CheckUpdates 检查更新.
func (h *Handlers) CheckUpdates(c *gin.Context) {
	id := c.Param("id")
	check, err := h.manager.CheckUpdates(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: check})
}

// CheckAllUpdates 批量检查更新.
func (h *Handlers) CheckAllUpdates(c *gin.Context) {
	checks := h.manager.CheckAllUpdates(context.Background())

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(checks),
			"checks": checks,
		},
	})
}

// ========== 更新执行 ==========

// ApplyUpdate 执行更新.
func (h *Handlers) ApplyUpdate(c *gin.Context) {
	id := c.Param("id")
	record, err := h.manager.ApplyUpdate(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "update started", Data: record})
}

// Rollback 回滚.
func (h *Handlers) Rollback(c *gin.Context) {
	recordId := c.Param("recordId")
	err := h.manager.Rollback(context.Background(), recordId)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "rollback completed"})
}

// ========== 健康状态 ==========

// GetHealth 获取健康状态.
func (h *Handlers) GetHealth(c *gin.Context) {
	id := c.Param("id")
	health, err := h.manager.GetHealth(context.Background(), id)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: health})
}

// ========== 历史和统计 ==========

// GetUpdateHistory 获取更新历史.
func (h *Handlers) GetUpdateHistory(c *gin.Context) {
	id := c.Param("id")
	history := h.manager.GetUpdateHistory(context.Background(), id)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(history),
			"history": history,
		},
	})
}

// GetStats 获取统计.
func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.manager.GetStats(context.Background())
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}
