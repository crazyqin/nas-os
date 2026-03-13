package auth

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RBACHandlers RBAC HTTP 处理器
type RBACHandlers struct {
	manager *RBACManager
}

// NewRBACHandlers 创建 RBAC 处理器
func NewRBACHandlers(mgr *RBACManager) *RBACHandlers {
	return &RBACHandlers{manager: mgr}
}

// RegisterRoutes 注册路由
func (h *RBACHandlers) RegisterRoutes(api *gin.RouterGroup) {
	rbac := api.Group("/rbac")
	{
		// 角色管理
		rbac.GET("/roles", h.getRoles)
		rbac.POST("/roles", h.createRole)
		rbac.GET("/roles/:name", h.getRole)
		rbac.DELETE("/roles/:name", h.deleteRole)

		// 用户角色分配
		rbac.GET("/users/:id/roles", h.getUserRoles)
		rbac.POST("/users/:id/roles", h.assignUserRole)
		rbac.DELETE("/users/:id/roles/:role", h.removeUserRole)

		// 组角色分配
		rbac.GET("/groups/:id/roles", h.getGroupRoles)
		rbac.POST("/groups/:id/roles", h.assignGroupRole)
		rbac.DELETE("/groups/:id/roles/:role", h.removeGroupRole)

		// 权限检查
		rbac.POST("/check", h.checkPermission)
		rbac.GET("/users/:id/permissions", h.getUserPermissions)

		// 资源 ACL
		rbac.GET("/resources/:id/acl", h.getResourceACL)
		rbac.PUT("/resources/:id/acl", h.setResourceACL)
	}
}

// APIResponse 通用 API 响应
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

func success(data interface{}) APIResponse {
	return APIResponse{Code: 0, Message: "success", Data: data}
}

func apiError(code int, message string) APIResponse {
	return APIResponse{Code: code, Message: message}
}

// ========== 角色管理 ==========

// getRoles 获取所有角色
func (h *RBACHandlers) getRoles(c *gin.Context) {
	roles := h.manager.GetRoles()
	c.JSON(http.StatusOK, success(roles))
}

// CreateRoleRequest 创建角色请求
type CreateRoleRequest struct {
	Name        string       `json:"name" binding:"required"`
	Description string       `json:"description"`
	Permissions []Permission `json:"permissions"`
	Inherits    []string     `json:"inherits"`
}

// createRole 创建自定义角色
func (h *RBACHandlers) createRole(c *gin.Context) {
	var req CreateRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	// 检查是否已存在
	if _, exists := h.manager.roles[Role(req.Name)]; exists {
		c.JSON(http.StatusConflict, apiError(409, "角色已存在"))
		return
	}

	// 转换继承角色
	inherits := make([]Role, len(req.Inherits))
	for i, r := range req.Inherits {
		inherits[i] = Role(r)
	}

	err := h.manager.AddRole(
		Role(req.Name),
		req.Description,
		req.Permissions,
		inherits,
	)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, success(nil))
}

// getRole 获取角色详情
func (h *RBACHandlers) getRole(c *gin.Context) {
	roleName := Role(c.Param("name"))

	h.manager.mu.RLock()
	roleDef, exists := h.manager.roles[roleName]
	h.manager.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, apiError(404, "角色不存在"))
		return
	}

	c.JSON(http.StatusOK, success(roleDef))
}

// deleteRole 删除角色
func (h *RBACHandlers) deleteRole(c *gin.Context) {
	roleName := Role(c.Param("name"))

	// 不允许删除内置角色
	if roleName == RoleAdmin || roleName == RoleUser || roleName == RoleGuest || roleName == RoleSystem {
		c.JSON(http.StatusBadRequest, apiError(400, "不能删除内置角色"))
		return
	}

	h.manager.mu.Lock()
	delete(h.manager.roles, roleName)
	h.manager.mu.Unlock()

	c.JSON(http.StatusOK, success(nil))
}

// ========== 用户角色分配 ==========

// getUserRoles 获取用户的所有角色
func (h *RBACHandlers) getUserRoles(c *gin.Context) {
	userID := c.Param("id")

	roles := h.manager.GetUserRoles(userID)
	if roles == nil {
		roles = []Role{}
	}

	c.JSON(http.StatusOK, success(roles))
}

// AssignRoleRequest 分配角色请求
type AssignRoleRequest struct {
	Role string `json:"role" binding:"required"`
}

// assignUserRole 给用户分配角色
func (h *RBACHandlers) assignUserRole(c *gin.Context) {
	userID := c.Param("id")

	var req AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	err := h.manager.AssignRoleToUser(userID, Role(req.Role))
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// removeUserRole 移除用户角色
func (h *RBACHandlers) removeUserRole(c *gin.Context) {
	userID := c.Param("id")
	roleName := Role(c.Param("role"))

	err := h.manager.RemoveUserRole(userID, roleName)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// ========== 组角色分配 ==========

// getGroupRoles 获取用户组的所有角色
func (h *RBACHandlers) getGroupRoles(c *gin.Context) {
	groupID := c.Param("id")

	h.manager.mu.RLock()
	roles := h.manager.groupRoles[groupID]
	h.manager.mu.RUnlock()

	if roles == nil {
		roles = []Role{}
	}

	c.JSON(http.StatusOK, success(roles))
}

// assignGroupRole 给用户组分配角色
func (h *RBACHandlers) assignGroupRole(c *gin.Context) {
	groupID := c.Param("id")

	var req AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	err := h.manager.AssignRoleToGroup(groupID, Role(req.Role))
	if err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	c.JSON(http.StatusOK, success(nil))
}

// removeGroupRole 移除用户组角色
func (h *RBACHandlers) removeGroupRole(c *gin.Context) {
	groupID := c.Param("id")
	roleName := Role(c.Param("role"))

	h.manager.mu.Lock()
	defer h.manager.mu.Unlock()

	roles := h.manager.groupRoles[groupID]
	for i, r := range roles {
		if r == roleName {
			h.manager.groupRoles[groupID] = append(roles[:i], roles[i+1:]...)
			c.JSON(http.StatusOK, success(nil))
			return
		}
	}

	c.JSON(http.StatusBadRequest, apiError(400, "角色未分配"))
}

// ========== 权限检查 ==========

// CheckPermissionRequest 权限检查请求
type CheckPermissionRequest struct {
	UserID   string   `json:"user_id" binding:"required"`
	Groups   []string `json:"groups"`
	Resource string   `json:"resource" binding:"required"`
	Action   string   `json:"action" binding:"required"`
}

// checkPermission 检查权限
func (h *RBACHandlers) checkPermission(c *gin.Context) {
	var req CheckPermissionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	hasPermission := h.manager.CheckPermission(
		req.UserID,
		req.Groups,
		Resource(req.Resource),
		Action(req.Action),
	)

	c.JSON(http.StatusOK, success(map[string]interface{}{
		"allowed": hasPermission,
	}))
}

// getUserPermissions 获取用户所有权限
func (h *RBACHandlers) getUserPermissions(c *gin.Context) {
	userID := c.Param("id")

	// 从查询参数获取用户组
	groups := c.QueryArray("groups")

	permissions := h.manager.GetPermissions(userID, groups)
	if permissions == nil {
		permissions = []Permission{}
	}

	c.JSON(http.StatusOK, success(permissions))
}

// ========== 资源 ACL ==========

// ResourceACLRequest 资源 ACL 请求
type ResourceACLRequest struct {
	ResourceType string     `json:"resource_type" binding:"required"`
	OwnerID      string     `json:"owner_id" binding:"required"`
	GroupACLs    []GroupACL `json:"group_acls"`
	UserACLs     []UserACL  `json:"user_acls"`
	ParentID     string     `json:"parent_id,omitempty"`
	Inherit      bool       `json:"inherit"`
}

// getResourceACL 获取资源 ACL
func (h *RBACHandlers) getResourceACL(c *gin.Context) {
	resourceID := c.Param("id")

	h.manager.mu.RLock()
	acl, exists := h.manager.resourceACLs[resourceID]
	h.manager.mu.RUnlock()

	if !exists {
		c.JSON(http.StatusNotFound, apiError(404, "ACL 不存在"))
		return
	}

	c.JSON(http.StatusOK, success(acl))
}

// setResourceACL 设置资源 ACL
func (h *RBACHandlers) setResourceACL(c *gin.Context) {
	resourceID := c.Param("id")

	var req ResourceACLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiError(400, err.Error()))
		return
	}

	h.manager.SetResourceACL(
		resourceID,
		Resource(req.ResourceType),
		req.OwnerID,
		req.GroupACLs,
		req.UserACLs,
	)

	c.JSON(http.StatusOK, success(nil))
}
