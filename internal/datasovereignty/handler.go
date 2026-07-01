package datasovereignty

import (
	"fmt"
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handlers 数据主权标签 HTTP 处理器.
type Handlers struct {
	svc *Service
}

// NewHandlers 创建数据主权标签处理器.
func NewHandlers(svc *Service) *Handlers {
	return &Handlers{svc: svc}
}

// RegisterRoutes 注册数据主权标签路由.
func (h *Handlers) RegisterRoutes(apiGroup *gin.RouterGroup) {
	ds := apiGroup.Group("/datasovereignty")
	{
		ds.POST("/tag", h.createTag)
		ds.GET("/check", h.checkTransfer)
		ds.GET("/audit", h.queryAudit)
		ds.DELETE("/tag", h.deleteTag)
	}
}

// createTag 创建数据主权标签
// @Summary 创建数据主权标签
// @Description 为文件/文件夹/存储池打数据主权标签
// @Tags datasovereignty
// @Accept json
// @Produce json
// @Param request body TagRequest true "标签信息"
// @Success 201 {object} api.Response{data=SovereigntyTag}
// @Failure 400 {object} api.Response
// @Router /datasovereignty/tag [post].
func (h *Handlers) createTag(c *gin.Context) {
	var req TagRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	tag, err := h.svc.CreateTag(req)
	if err != nil {
		if err == ErrTagAlreadyExists {
			api.Conflict(c, "该资源已存在数据主权标签")
			return
		}
		api.BadRequest(c, err.Error())
		return
	}

	api.Created(c, tag)
}

// deleteTag 删除数据主权标签
// @Summary 删除数据主权标签
// @Description 根据 ID 删除数据主权标签
// @Tags datasovereignty
// @Produce json
// @Param id query string true "标签 ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /datasovereignty/tag [delete].
func (h *Handlers) deleteTag(c *gin.Context) {
	tagID := c.Query("id")
	if tagID == "" {
		api.BadRequest(c, "标签 ID 不能为空")
		return
	}

	if err := h.svc.DeleteTag(tagID); err != nil {
		if err == ErrTagNotFound {
			api.NotFound(c, "数据主权标签未找到")
			return
		}
		api.InternalError(c, err.Error())
		return
	}

	api.OKWithMessage(c, "标签已删除", nil)
}

// checkTransfer 合规检查
// @Summary 数据传输合规检查
// @Description 检查数据传输到目标区域是否符合合规要求
// @Tags datasovereignty
// @Accept json
// @Produce json
// @Param request body CheckRequest true "检查请求"
// @Success 200 {object} api.Response{data=CheckResponse}
// @Failure 400 {object} api.Response
// @Router /datasovereignty/check [get].
func (h *Handlers) checkTransfer(c *gin.Context) {
	// GET 接口，参数通过 query 或 body 传递
	var req CheckRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 尝试从 query 参数绑定
		req.ResourcePath = c.Query("resource_path")
		req.Action = TransferAction(c.Query("action"))
		req.TargetRegion = DataRegion(c.Query("target_region"))
		req.User = c.Query("user")
		req.ClientIP = c.Query("client_ip")

		if req.ResourcePath == "" || req.Action == "" || req.TargetRegion == "" {
			api.BadRequest(c, "resource_path, action, target_region 不能为空")
			return
		}
	}

	resp, err := h.svc.CheckTransfer(req)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OK(c, resp)
}

// queryAudit 查询审计日志
// @Summary 查询数据跨域审计日志
// @Description 按条件查询数据跨域操作审计日志
// @Tags datasovereignty
// @Produce json
// @Param resource_path query string false "资源路径"
// @Param action query string false "传输动作"
// @Param status query string false "合规状态"
// @Param user query string false "操作用户"
// @Param limit query int false "返回条数上限"
// @Success 200 {object} api.Response{data=[]AuditEntry}
// @Router /datasovereignty/audit [get].
func (h *Handlers) queryAudit(c *gin.Context) {
	query := AuditQuery{
		ResourcePath: c.Query("resource_path"),
		Action:       TransferAction(c.Query("action")),
		Status:       TransferStatus(c.Query("status")),
		User:         c.Query("user"),
	}

	if limitStr := c.Query("limit"); limitStr != "" {
		var limit int
		if _, err := fmt.Sscanf(limitStr, "%d", &limit); err == nil && limit > 0 {
			query.Limit = limit
		}
	}

	entries := h.svc.QueryAudit(query)
	api.OK(c, entries)
}
