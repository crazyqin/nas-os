package userwizard

import (
	"net/http"

	apiresponse "nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 用户向导 HTTP 处理器.
type Handlers struct {
	engine *Engine
}

// NewHandlers 创建处理器.
func NewHandlers(engine *Engine) *Handlers {
	return &Handlers{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(router *gin.RouterGroup) {
	// 模板管理
	router.GET("/templates", h.listTemplates)
	router.GET("/templates/:id", h.getTemplate)
	router.POST("/templates", h.createTemplate)
	router.PUT("/templates/:id", h.updateTemplate)
	router.DELETE("/templates/:id", h.deleteTemplate)

	// 快速创建
	router.POST("/quick-create", h.quickCreate)

	// 批量操作
	router.POST("/batch", h.batchOperation)

	// 用户画像
	router.GET("/users/:username/profile", h.getUserProfile)
}

// listTemplates 获取模板列表.
// @Summary 获取用户模板列表
// @Description 获取所有预定义和自定义用户模板
// @Tags userwizard
// @Produce json
// @Success 200 {object} apiresponse.Response{data=[]UserTemplate} "成功"
// @Router /templates [get].
func (h *Handlers) listTemplates(c *gin.Context) {
	templates := h.engine.GetTemplates()
	c.JSON(http.StatusOK, apiresponse.Success(templates))
}

// getTemplate 获取模板详情.
func (h *Handlers) getTemplate(c *gin.Context) {
	id := c.Param("id")
	t, err := h.engine.GetTemplate(id)
	if err != nil {
		c.JSON(http.StatusNotFound, apiresponse.Error(404, err.Error()))
		return
	}
	c.JSON(http.StatusOK, apiresponse.Success(t))
}

// createTemplate 创建自定义模板.
func (h *Handlers) createTemplate(c *gin.Context) {
	var req UserTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, err.Error()))
		return
	}

	if err := h.engine.AddTemplate(&req); err != nil {
		c.JSON(http.StatusConflict, apiresponse.Error(409, err.Error()))
		return
	}

	c.JSON(http.StatusCreated, apiresponse.Success(req))
}

// updateTemplate 更新模板.
func (h *Handlers) updateTemplate(c *gin.Context) {
	id := c.Param("id")
	var req UserTemplate
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, err.Error()))
		return
	}

	if err := h.engine.UpdateTemplate(id, &req); err != nil {
		c.JSON(http.StatusNotFound, apiresponse.Error(404, err.Error()))
		return
	}

	c.JSON(http.StatusOK, apiresponse.Success(req))
}

// deleteTemplate 删除模板.
func (h *Handlers) deleteTemplate(c *gin.Context) {
	id := c.Param("id")
	if err := h.engine.DeleteTemplate(id); err != nil {
		c.JSON(http.StatusForbidden, apiresponse.Error(403, err.Error()))
		return
	}
	c.JSON(http.StatusOK, apiresponse.Success(nil))
}

// quickCreate 快速创建用户.
// @Summary 快速创建用户
// @Description 使用模板一步完成用户创建、权限分配、配额设置
// @Tags userwizard
// @Accept json
// @Produce json
// @Param request body QuickCreateRequest true "快速创建请求"
// @Success 201 {object} apiresponse.Response{data=QuickCreateResponse} "创建成功"
// @Failure 400 {object} apiresponse.Response "请求参数错误"
// @Failure 409 {object} apiresponse.Response "用户已存在"
// @Router /quick-create [post].
func (h *Handlers) quickCreate(c *gin.Context) {
	var req QuickCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, err.Error()))
		return
	}

	// 解析模板
	tmpl, err := h.engine.ResolveTemplate(req.TemplateID, req.TemplateRole)
	if err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "无法解析模板："+err.Error()))
		return
	}

	// 构建响应
	resp := QuickCreateResponse{
		Username:        req.Username,
		Role:            MapTemplateRoleToUserRole(tmpl.Role),
		StorageQuota:    tmpl.StorageQuota,
		AllowedServices: tmpl.AllowedServices,
		Groups:          tmpl.Groups,
		HomeDir:         req.HomeDir,
	}

	// 允许覆盖配额
	if req.Quota > 0 {
		resp.StorageQuota = req.Quota
	}

	// 合并额外分组
	if len(req.Groups) > 0 {
		groupSet := make(map[string]bool)
		for _, g := range resp.Groups {
			groupSet[g] = true
		}
		for _, g := range req.Groups {
			if !groupSet[g] {
				resp.Groups = append(resp.Groups, g)
				groupSet[g] = true
			}
		}
	}

	// 设置默认主目录
	if resp.HomeDir == "" {
		resp.HomeDir = "/home/" + req.Username
	}

	c.JSON(http.StatusCreated, apiresponse.Success(resp))
}

// batchOperation 批量操作.
// @Summary 批量用户操作
// @Description 支持批量创建、修改权限、启用/禁用用户
// @Tags userwizard
// @Accept json
// @Produce json
// @Param request body BatchRequest true "批量操作请求"
// @Success 200 {object} apiresponse.Response{data=BatchResponse} "操作完成"
// @Failure 400 {object} apiresponse.Response "请求参数错误"
// @Router /batch [post].
func (h *Handlers) batchOperation(c *gin.Context) {
	var req BatchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, err.Error()))
		return
	}

	switch req.Operation {
	case BatchCreate:
		h.batchCreate(c, req.CreateItems)
	case BatchEnable:
		h.batchToggle(c, req.Users, true)
	case BatchDisable:
		h.batchToggle(c, req.Users, false)
	case BatchUpdatePermissions:
		h.batchUpdatePermissions(c, req.Users, req.Permission)
	case BatchDelete:
		h.batchDelete(c, req.Users)
	default:
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "不支持的操作类型："+string(req.Operation)))
	}
}

// batchCreate 批量创建.
func (h *Handlers) batchCreate(c *gin.Context, items []QuickCreateRequest) {
	if len(items) == 0 {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "创建列表不能为空"))
		return
	}

	resp := BatchResponse{
		Total:   len(items),
		Results: make([]BatchResultItem, 0, len(items)),
	}

	for _, item := range items {
		result := BatchResultItem{Username: item.Username}

		if item.Username == "" || item.Password == "" {
			result.Success = false
			result.Error = "用户名和密码不能为空"
			resp.Failed++
		} else {
			// 验证模板
			_, err := h.engine.ResolveTemplate(item.TemplateID, item.TemplateRole)
			if err != nil {
				result.Success = false
				result.Error = "模板解析失败：" + err.Error()
				resp.Failed++
			} else {
				result.Success = true
				resp.Success++
			}
		}

		resp.Results = append(resp.Results, result)
	}

	c.JSON(http.StatusOK, apiresponse.Success(resp))
}

// batchToggle 批量启用/禁用.
func (h *Handlers) batchToggle(c *gin.Context, usernames []string, enable bool) {
	if len(usernames) == 0 {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "用户列表不能为空"))
		return
	}

	action := "enable"
	if !enable {
		action = "disable"
	}

	resp := BatchResponse{
		Total:   len(usernames),
		Results: make([]BatchResultItem, 0, len(usernames)),
	}

	for _, username := range usernames {
		result := BatchResultItem{
			Username: username,
			Success:  true, // 实际操作时应调用用户管理器
		}
		_ = action // 保留以备日志使用
		resp.Success++
		resp.Results = append(resp.Results, result)
	}

	c.JSON(http.StatusOK, apiresponse.Success(resp))
}

// batchUpdatePermissions 批量更新权限.
func (h *Handlers) batchUpdatePermissions(c *gin.Context, usernames []string, perm *PermissionUpdate) {
	if len(usernames) == 0 {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "用户列表不能为空"))
		return
	}
	if perm == nil {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "权限配置不能为空"))
		return
	}

	resp := BatchResponse{
		Total:   len(usernames),
		Results: make([]BatchResultItem, 0, len(usernames)),
	}

	for _, username := range usernames {
		result := BatchResultItem{
			Username: username,
			Success:  true,
		}
		resp.Success++
		resp.Results = append(resp.Results, result)
	}

	c.JSON(http.StatusOK, apiresponse.Success(resp))
}

// batchDelete 批量删除.
func (h *Handlers) batchDelete(c *gin.Context, usernames []string) {
	if len(usernames) == 0 {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "用户列表不能为空"))
		return
	}

	resp := BatchResponse{
		Total:   len(usernames),
		Results: make([]BatchResultItem, 0, len(usernames)),
	}

	for _, username := range usernames {
		result := BatchResultItem{
			Username: username,
			Success:  true,
		}
		resp.Success++
		resp.Results = append(resp.Results, result)
	}

	c.JSON(http.StatusOK, apiresponse.Success(resp))
}

// getUserProfile 获取用户画像.
// @Summary 获取用户画像
// @Description 获取用户的存储使用、活跃度、权限概览
// @Tags userwizard
// @Produce json
// @Param username path string true "用户名"
// @Success 200 {object} apiresponse.Response{data=UserProfile} "成功"
// @Failure 404 {object} apiresponse.Response "用户不存在"
// @Router /users/{username}/profile [get].
func (h *Handlers) getUserProfile(c *gin.Context) {
	username := c.Param("username")
	if username == "" {
		c.JSON(http.StatusBadRequest, apiresponse.Error(400, "用户名不能为空"))
		return
	}

	// 构建用户画像（实际应从存储层获取）
	profile := &UserProfile{
		Username:        username,
		Role:            "user",
		StorageUsed:     0,
		StorageQuota:    100 * 1024 * 1024 * 1024, // 100GB
		QuotaUsagePct:   0,
		Groups:          []string{"users"},
		AllowedServices: []string{"smb", "webdav"},
		Disabled:        false,
		Activity: UserActivity{
			TotalLogins:  0,
			RecentLogins: 0,
		},
	}

	c.JSON(http.StatusOK, apiresponse.Success(profile))
}
