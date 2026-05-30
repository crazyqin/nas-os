// Package zerotrustgw 提供 REST API 处理器
package zerotrustgw

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 零信任网关 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	zt := r.Group("/zerotrust")
	{
		// 访问控制
		zt.POST("/access/evaluate", h.evaluateAccess)

		// 策略管理
		zt.GET("/policies", h.listPolicies)
		zt.POST("/policies", h.createPolicy)
		zt.GET("/policies/:id", h.getPolicy)
		zt.PUT("/policies/:id", h.updatePolicy)
		zt.DELETE("/policies/:id", h.deletePolicy)

		// 设备管理
		zt.GET("/devices", h.listDevices)
		zt.POST("/devices", h.registerDevice)
		zt.GET("/devices/:id", h.getDevice)

		// 信任分数
		zt.GET("/trust-scores/:userId", h.getTrustScore)

		// 会话管理
		zt.POST("/sessions", h.createSession)
		zt.GET("/sessions/:id", h.getSession)

		// 审计日志
		zt.GET("/audit-log", h.getAuditLog)

		// 配置
		zt.GET("/config", h.getConfig)
		zt.PUT("/config", h.updateConfig)

		// 统计信息
		zt.GET("/stats", h.getStats)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// evaluateAccess 评估访问请求
func (h *Handlers) evaluateAccess(c *gin.Context) {
	var req AccessRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.EvaluateAccess(c.Request.Context(), &req)
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
		Data:    result,
	})
}

// listPolicies 列出策略
func (h *Handlers) listPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    policies,
	})
}

// createPolicy 创建策略
func (h *Handlers) createPolicy(c *gin.Context) {
	var req TrustPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	policy, err := h.manager.CreatePolicy(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "policy created",
		Data:    policy,
	})
}

// getPolicy 获取策略
func (h *Handlers) getPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetPolicy(id)
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
		Data:    policy,
	})
}

// updatePolicy 更新策略
func (h *Handlers) updatePolicy(c *gin.Context) {
	id := c.Param("id")
	var req TrustPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	policy, err := h.manager.UpdatePolicy(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "policy updated",
		Data:    policy,
	})
}

// deletePolicy 删除策略
func (h *Handlers) deletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "policy deleted",
	})
}

// listDevices 列出设备
func (h *Handlers) listDevices(c *gin.Context) {
	devices := h.manager.ListDevices()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    devices,
	})
}

// registerDevice 注册设备
func (h *Handlers) registerDevice(c *gin.Context) {
	var req DeviceProfile
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	device, err := h.manager.RegisterDevice(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "device registered",
		Data:    device,
	})
}

// getDevice 获取设备信息
func (h *Handlers) getDevice(c *gin.Context) {
	id := c.Param("id")
	device, err := h.manager.GetDevice(id)
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
		Data:    device,
	})
}

// getTrustScore 获取信任分数
func (h *Handlers) getTrustScore(c *gin.Context) {
	userID := c.Param("userId")
	score, err := h.manager.GetTrustScore(userID)
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
		Data:    score,
	})
}

// createSession 创建会话
func (h *Handlers) createSession(c *gin.Context) {
	var req struct {
		UserID   string `json:"user_id" binding:"required"`
		DeviceID string `json:"device_id" binding:"required"`
		SourceIP string `json:"source_ip" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	session := h.manager.CreateSession(req.UserID, req.DeviceID, req.SourceIP)
	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "session created",
		Data:    session,
	})
}

// getSession 获取会话
func (h *Handlers) getSession(c *gin.Context) {
	id := c.Param("id")
	session, err := h.manager.GetSession(id)
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
		Data:    session,
	})
}

// getAuditLog 获取审计日志
func (h *Handlers) getAuditLog(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	userID := c.Query("user_id")
	entries := h.manager.GetAuditLog(limit, userID)

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    entries,
	})
}

// getConfig 获取配置
func (h *Handlers) getConfig(c *gin.Context) {
	cfg := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    cfg,
	})
}

// updateConfig 更新配置
func (h *Handlers) updateConfig(c *gin.Context) {
	var cfg ZeroTrustConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	h.manager.UpdateConfig(&cfg)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "config updated",
	})
}

// getStats 获取统计信息
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}
