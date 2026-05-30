package ztna

import (
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers ZTNA HTTP 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建 ZTNA 处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{manager: mgr}
}

// RegisterRoutes 注册 ZTNA 路由.
func (h *Handlers) RegisterRoutes(apiGroup *gin.RouterGroup) {
	ztna := apiGroup.Group("/ztna")
	{
		// 策略管理
		ztna.POST("/policies", h.createPolicy)
		ztna.GET("/policies", h.listPolicies)
		ztna.GET("/policies/:id", h.getPolicy)
		ztna.DELETE("/policies/:id", h.deletePolicy)

		// 设备验证
		ztna.POST("/verify", h.verifyDevice)
		ztna.GET("/devices/:id/trust", h.getDeviceTrust)

		// 会话管理
		ztna.GET("/sessions", h.listSessions)
		ztna.GET("/sessions/:id", h.getSession)
		ztna.DELETE("/sessions/:id", h.revokeSession)
	}
}

// ========== 策略处理 ==========

// createPolicy 创建访问策略
// @Summary 创建 ZTNA 访问策略
// @Description 创建新的零信任访问策略
// @Tags ztna
// @Accept json
// @Produce json
// @Param request body CreatePolicyRequest true "策略信息"
// @Success 201 {object} api.Response{data=Policy}
// @Failure 400 {object} api.Response
// @Router /ztna/policies [post].
func (h *Handlers) createPolicy(c *gin.Context) {
	var req CreatePolicyRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	policy, err := h.manager.CreatePolicy(req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.Created(c, policy)
}

// listPolicies 列出所有策略
// @Summary 列出 ZTNA 策略
// @Description 获取所有零信任访问策略
// @Tags ztna
// @Produce json
// @Success 200 {object} api.Response{data=[]Policy}
// @Router /ztna/policies [get].
func (h *Handlers) listPolicies(c *gin.Context) {
	policies := h.manager.ListPolicies()
	api.OK(c, policies)
}

// getPolicy 获取策略详情
// @Summary 获取 ZTNA 策略
// @Description 根据 ID 获取策略详情
// @Tags ztna
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} api.Response{data=Policy}
// @Failure 404 {object} api.Response
// @Router /ztna/policies/{id} [get].
func (h *Handlers) getPolicy(c *gin.Context) {
	policyID := c.Param("id")
	if policyID == "" {
		api.BadRequest(c, "策略 ID 不能为空")
		return
	}

	policy, err := h.manager.GetPolicy(policyID)
	if err != nil {
		if err == ErrPolicyNotFound {
			api.NotFound(c, "策略不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, policy)
}

// deletePolicy 删除策略
// @Summary 删除 ZTNA 策略
// @Description 根据 ID 删除策略
// @Tags ztna
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /ztna/policies/{id} [delete].
func (h *Handlers) deletePolicy(c *gin.Context) {
	policyID := c.Param("id")
	if policyID == "" {
		api.BadRequest(c, "策略 ID 不能为空")
		return
	}

	if err := h.manager.DeletePolicy(policyID); err != nil {
		if err == ErrPolicyNotFound {
			api.NotFound(c, "策略不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "策略已删除", nil)
}

// ========== 设备验证处理 ==========

// verifyDevice 验证设备信任
// @Summary 验证设备信任
// @Description 验证设备并返回信任评分
// @Tags ztna
// @Accept json
// @Produce json
// @Param request body VerifyRequest true "设备信息"
// @Success 200 {object} api.Response{data=DeviceTrust}
// @Failure 400 {object} api.Response
// @Router /ztna/verify [post].
func (h *Handlers) verifyDevice(c *gin.Context) {
	var req VerifyRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	device, err := h.manager.VerifyDevice(req)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, device)
}

// getDeviceTrust 获取设备信任信息
// @Summary 获取设备信任信息
// @Description 根据设备 ID 获取信任评分和状态
// @Tags ztna
// @Produce json
// @Param id path string true "设备 ID"
// @Success 200 {object} api.Response{data=DeviceTrust}
// @Failure 404 {object} api.Response
// @Router /ztna/devices/{id}/trust [get].
func (h *Handlers) getDeviceTrust(c *gin.Context) {
	deviceID := c.Param("id")
	if deviceID == "" {
		api.BadRequest(c, "设备 ID 不能为空")
		return
	}

	device, err := h.manager.GetDeviceTrust(deviceID)
	if err != nil {
		if err == ErrDeviceNotFound {
			api.NotFound(c, "设备未找到")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, device)
}

// ========== 会话管理处理 ==========

// listSessions 列出所有会话
// @Summary 列出 ZTNA 会话
// @Description 获取所有活跃会话
// @Tags ztna
// @Produce json
// @Success 200 {object} api.Response{data=[]Session}
// @Router /ztna/sessions [get].
func (h *Handlers) listSessions(c *gin.Context) {
	sessions := h.manager.ListSessions()
	api.OK(c, sessions)
}

// getSession 获取会话详情
// @Summary 获取 ZTNA 会话
// @Description 根据 ID 获取会话详情
// @Tags ztna
// @Produce json
// @Param id path string true "会话 ID"
// @Success 200 {object} api.Response{data=Session}
// @Failure 404 {object} api.Response
// @Router /ztna/sessions/{id} [get].
func (h *Handlers) getSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		api.BadRequest(c, "会话 ID 不能为空")
		return
	}

	session, err := h.manager.GetSession(sessionID)
	if err != nil {
		if err == ErrSessionNotFound {
			api.NotFound(c, "会话不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, session)
}

// revokeSession 撤销会话
// @Summary 撤销 ZTNA 会话
// @Description 立即撤销会话，终止访问
// @Tags ztna
// @Produce json
// @Param id path string true "会话 ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /ztna/sessions/{id} [delete].
func (h *Handlers) revokeSession(c *gin.Context) {
	sessionID := c.Param("id")
	if sessionID == "" {
		api.BadRequest(c, "会话 ID 不能为空")
		return
	}

	if err := h.manager.RevokeAccess(sessionID); err != nil {
		if err == ErrSessionNotFound {
			api.NotFound(c, "会话不存在")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "会话已撤销", nil)
}
