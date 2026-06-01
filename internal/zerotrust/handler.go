// Package zerotrust 提供零信任安全 REST API 处理器
package zerotrust

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 零信任安全 API 处理器
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
		// 信任评估
		zt.GET("/trust/:device_id", h.evaluateTrust)
		zt.GET("/trust/score/:device_id", h.getTrustScore)

		// 设备管理
		zt.POST("/devices", h.registerDevice)
		zt.GET("/devices", h.listDevices)
		zt.GET("/devices/:id", h.getDevice)

		// 访问策略
		zt.POST("/policies", h.setPolicy)
		zt.GET("/policies", h.listPolicies)
		zt.GET("/policies/:id", h.getPolicy)
		zt.DELETE("/policies/:id", h.deletePolicy)
		zt.POST("/policies/check", h.checkAccess)

		// 认证会话
		zt.POST("/sessions", h.createSession)
		zt.GET("/sessions", h.listSessions)
		zt.GET("/sessions/:id", h.getSession)
		zt.DELETE("/sessions/:id", h.revokeSession)

		// 威胁管理
		zt.POST("/threats", h.blockThreat)
		zt.GET("/threats", h.listThreats)
		zt.GET("/threats/:id", h.getThreat)
		zt.PUT("/threats/:id", h.resolveThreat)

		// 统计
		zt.GET("/stats", h.getStats)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// evaluateTrust 评估信任
func (h *Handlers) evaluateTrust(c *gin.Context) {
	deviceID := c.Param("device_id")
	score, err := h.manager.EvaluateTrust(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Trust evaluated",
		Data:    score,
	})
}

// getTrustScore 获取信任评分
func (h *Handlers) getTrustScore(c *gin.Context) {
	deviceID := c.Param("device_id")
	score, ok := h.manager.trustScores[deviceID]
	if !ok {
		// 尝试评估
		evalScore, err := h.manager.EvaluateTrust(deviceID)
		if err != nil {
			c.JSON(http.StatusNotFound, response{
				Code:    http.StatusNotFound,
				Message: err.Error(),
			})
			return
		}
		score = evalScore
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    score,
	})
}

// registerDevice 注册设备
func (h *Handlers) registerDevice(c *gin.Context) {
	var device DeviceTrust
	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if err := h.manager.RegisterDevice(&device); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    http.StatusCreated,
		Message: "Device registered",
		Data:    device,
	})
}

// listDevices 获取设备列表
func (h *Handlers) listDevices(c *gin.Context) {
	filter := &DeviceFilter{
		Status:     c.Query("status"),
		DeviceType: c.Query("device_type"),
		Owner:      c.Query("owner"),
	}

	devices := h.manager.ListDevices(filter)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    devices,
	})
}

// getDevice 获取设备信息
func (h *Handlers) getDevice(c *gin.Context) {
	id := c.Param("id")
	device, err := h.manager.GetDeviceTrust(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    device,
	})
}

// setPolicy 设置策略
func (h *Handlers) setPolicy(c *gin.Context) {
	var policy AccessPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if err := h.manager.SetPolicy(&policy); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    http.StatusCreated,
		Message: "Policy set",
		Data:    policy,
	})
}

// listPolicies 获取策略列表
func (h *Handlers) listPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    policies,
	})
}

// getPolicy 获取策略详情
func (h *Handlers) getPolicy(c *gin.Context) {
	id := c.Param("id")
	policy, err := h.manager.GetPolicy(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    policy,
	})
}

// deletePolicy 删除策略
func (h *Handlers) deletePolicy(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeletePolicy(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Policy deleted",
	})
}

// checkAccess 检查访问权限
func (h *Handlers) checkAccess(c *gin.Context) {
	var req struct {
		SubjectType  string `json:"subject_type"`
		SubjectID    string `json:"subject_id"`
		ResourceType string `json:"resource_type"`
		ResourceID   string `json:"resource_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	allowed, action := h.manager.CheckAccess(req.SubjectType, req.SubjectID, req.ResourceType, req.ResourceID)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Access check completed",
		Data: map[string]interface{}{
			"allowed": allowed,
			"action":  action,
		},
	})
}

// createSession 创建会话
func (h *Handlers) createSession(c *gin.Context) {
	var session AuthSession
	if err := c.ShouldBindJSON(&session); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if err := h.manager.CreateSession(&session); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    http.StatusCreated,
		Message: "Session created",
		Data:    session,
	})
}

// listSessions 获取会话列表
func (h *Handlers) listSessions(c *gin.Context) {
	filter := &SessionFilter{
		UserID:   c.Query("user_id"),
		DeviceID: c.Query("device_id"),
		Status:   c.Query("status"),
	}

	sessions := h.manager.ListSessions(filter)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    sessions,
	})
}

// getSession 获取会话详情
func (h *Handlers) getSession(c *gin.Context) {
	id := c.Param("id")
	session, err := h.manager.GetSession(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    session,
	})
}

// revokeSession 撤销会话
func (h *Handlers) revokeSession(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RevokeSession(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Session revoked",
	})
}

// blockThreat 阻断威胁
func (h *Handlers) blockThreat(c *gin.Context) {
	var threat ThreatEvent
	if err := c.ShouldBindJSON(&threat); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if err := h.manager.BlockThreat(&threat); err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    http.StatusInternalServerError,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    http.StatusCreated,
		Message: "Threat blocked",
		Data:    threat,
	})
}

// listThreats 获取威胁列表
func (h *Handlers) listThreats(c *gin.Context) {
	filter := &ThreatFilter{}
	if types := c.QueryArray("type"); len(types) > 0 {
		filter.Types = types
	}
	if severities := c.QueryArray("severity"); len(severities) > 0 {
		filter.Severities = severities
	}
	if statuses := c.QueryArray("status"); len(statuses) > 0 {
		filter.Statuses = statuses
	}

	threats := h.manager.ListThreats(filter)
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    threats,
	})
}

// getThreat 获取威胁详情
func (h *Handlers) getThreat(c *gin.Context) {
	id := c.Param("id")
	threat, err := h.manager.GetThreat(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    threat,
	})
}

// resolveThreat 解决威胁
func (h *Handlers) resolveThreat(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Notes string `json:"notes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    http.StatusBadRequest,
			Message: "Invalid request body",
		})
		return
	}

	if err := h.manager.ResolveThreat(id, req.Notes); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    http.StatusNotFound,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "Threat resolved",
	})
}

// getStats 获取统计信息
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{
		Code:    http.StatusOK,
		Message: "success",
		Data:    stats,
	})
}
