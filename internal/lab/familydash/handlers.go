// Package familydash 提供 REST API 处理器
package familydash

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 家庭仪表板 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	family := r.Group("/family")
	{
		// 成员管理
		family.GET("/members", h.listMembers)
		family.POST("/members", h.createMember)
		family.GET("/members/:id", h.getMember)
		family.PUT("/members/:id", h.updateMember)
		family.DELETE("/members/:id", h.deleteMember)
		family.GET("/members/online", h.getOnlineMembers)
		family.GET("/members/children", h.getChildMembers)
		family.PUT("/members/:id/status", h.updateStatus)

		// 个人资料
		family.GET("/members/:id/profile", h.getProfile)
		family.PUT("/members/:id/profile", h.updateProfile)

		// 收藏
		family.POST("/members/:id/favorites", h.addFavorite)
		family.DELETE("/members/:id/favorites/:fav_id", h.removeFavorite)

		// 权限管理
		family.GET("/members/:id/permissions", h.getPermissions)
		family.PUT("/members/:id/permissions", h.setPermissions)
		family.GET("/members/:id/permissions/check", h.checkPermission)

		// 活动记录
		family.GET("/activity", h.getActivity)
		family.GET("/members/:id/activity/summary", h.getActivitySummary)

		// 统计
		family.GET("/stats", h.getStats)
	}
}

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listMembers 列出成员.
func (h *Handlers) listMembers(c *gin.Context) {
	members := h.manager.ListMembers()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    members,
	})
}

// createMember 创建成员.
func (h *Handlers) createMember(c *gin.Context) {
	var req CreateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	member, err := h.manager.CreateMember(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "member created",
		Data:    member,
	})
}

// getMember 获取成员.
func (h *Handlers) getMember(c *gin.Context) {
	id := c.Param("id")
	member, err := h.manager.GetMember(id)
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
		Data:    member,
	})
}

// updateMember 更新成员.
func (h *Handlers) updateMember(c *gin.Context) {
	id := c.Param("id")
	var req UpdateMemberRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	member, err := h.manager.UpdateMember(id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "member updated",
		Data:    member,
	})
}

// deleteMember 删除成员.
func (h *Handlers) deleteMember(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteMember(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "member deleted",
	})
}

// getOnlineMembers 获取在线成员.
func (h *Handlers) getOnlineMembers(c *gin.Context) {
	members := h.manager.GetOnlineMembers()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    members,
	})
}

// getChildMembers 获取子成员.
func (h *Handlers) getChildMembers(c *gin.Context) {
	members := h.manager.GetChildMembers()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    members,
	})
}

// updateStatus 更新状态.
func (h *Handlers) updateStatus(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Status MemberStatus `json:"status" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.UpdateMemberStatus(id, req.Status); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "status updated",
	})
}

// getProfile 获取个人资料.
func (h *Handlers) getProfile(c *gin.Context) {
	id := c.Param("id")
	profile, err := h.manager.GetProfile(id)
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
		Data:    profile,
	})
}

// updateProfile 更新个人资料.
func (h *Handlers) updateProfile(c *gin.Context) {
	id := c.Param("id")
	var req UpdateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	profile, err := h.manager.UpdateProfile(id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "profile updated",
		Data:    profile,
	})
}

// addFavorite 添加收藏.
func (h *Handlers) addFavorite(c *gin.Context) {
	id := c.Param("id")
	var req AddFavoriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.AddFavorite(id, &req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "favorite added",
	})
}

// removeFavorite 移除收藏.
func (h *Handlers) removeFavorite(c *gin.Context) {
	id := c.Param("id")
	favID := c.Param("fav_id")

	if err := h.manager.RemoveFavorite(id, favID); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "favorite removed",
	})
}

// getPermissions 获取权限.
func (h *Handlers) getPermissions(c *gin.Context) {
	id := c.Param("id")
	perms, err := h.manager.GetPermissions(id)
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
		Data:    perms,
	})
}

// setPermissions 设置权限.
func (h *Handlers) setPermissions(c *gin.Context) {
	id := c.Param("id")
	var req UpdatePermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	perms, err := h.manager.SetPermissions(id, &req)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "permissions updated",
		Data:    perms,
	})
}

// checkPermission 检查权限.
func (h *Handlers) checkPermission(c *gin.Context) {
	id := c.Param("id")
	action := c.Query("action")

	if action == "" {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "action parameter is required",
		})
		return
	}

	allowed := h.manager.CheckPermission(id, action)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: map[string]interface{}{
			"member_id": id,
			"action":    action,
			"allowed":   allowed,
		},
	})
}

// getActivity 获取活动记录.
func (h *Handlers) getActivity(c *gin.Context) {
	query := &ActivityQuery{
		MemberID: c.Query("member_id"),
		Type:     ActivityType(c.Query("type")),
		FromDate: c.Query("from_date"),
		ToDate:   c.Query("to_date"),
		Limit:    50,
	}

	activities := h.manager.GetActivity(query)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    activities,
	})
}

// getActivitySummary 获取活动摘要.
func (h *Handlers) getActivitySummary(c *gin.Context) {
	id := c.Param("id")
	period := c.DefaultQuery("period", "weekly")

	summary := h.manager.GetActivitySummary(id, period)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    summary,
	})
}

// getStats 获取统计.
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GenerateStats()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    stats,
	})
}
