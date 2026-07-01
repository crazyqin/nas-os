// Package sharedlabels 提供 REST API 处理器
package sharedlabels

import (
	"nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// Handler HTTP 处理器
type Handler struct {
	svc *Service
}

// NewHandler 创建处理器
func NewHandler(svc *Service) *Handler {
	return &Handler{svc: svc}
}

// RegisterRoutes 注册路由到 gin 路由组
// 路由前缀: /api/v1/sharedlabels
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/sharedlabels")
	{
		g.POST("/assign", h.assignLabels)   // 为文件分配标签
		g.GET("/search", h.searchByLabels)  // 按标签搜索文件
		g.GET("/list", h.listLabels)        // 列出所有标签
		g.DELETE("/remove", h.removeLabels) // 移除文件标签
		g.POST("/create", h.createLabel)    // 创建新标签
		g.GET("/stats", h.getStats)         // 标签统计
	}
}

// assignLabels 为文件分配标签
// POST /api/v1/sharedlabels/assign
func (h *Handler) assignLabels(c *gin.Context) {
	var req AssignLabelRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	result, err := h.svc.AssignLabels(c.Request.Context(), req)
	if err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.Created(c, gin.H{"assigned": result, "count": len(result)})
}

// searchByLabels 按标签搜索文件
// GET /api/v1/sharedlabels/search?label_ids=xxx&label_ids=yyy
func (h *Handler) searchByLabels(c *gin.Context) {
	labelIDs := c.QueryArray("label_ids")
	if len(labelIDs) == 0 {
		api.BadRequest(c, "至少提供一个 label_ids 参数")
		return
	}

	result, err := h.svc.SearchByLabels(c.Request.Context(), labelIDs)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, gin.H{"files": result, "count": len(result)})
}

// listLabels 列出所有标签
// GET /api/v1/sharedlabels/list?type=team&tenant_id=xxx
func (h *Handler) listLabels(c *gin.Context) {
	labelType := LabelType(c.Query("type"))
	tenantID := c.Query("tenant_id")

	labels, err := h.svc.ListLabels(c.Request.Context(), labelType, tenantID)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, gin.H{"labels": labels, "total": len(labels)})
}

// removeLabels 移除文件标签
// DELETE /api/v1/sharedlabels/remove
func (h *Handler) removeLabels(c *gin.Context) {
	var req RemoveLabelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		api.BadRequest(c, "请求参数错误: "+err.Error())
		return
	}

	if err := h.svc.RemoveLabels(c.Request.Context(), req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	api.OKWithMessage(c, "标签移除成功", nil)
}

// createLabel 创建新标签
// POST /api/v1/sharedlabels/create
func (h *Handler) createLabel(c *gin.Context) {
	var req CreateLabelRequest
	if err := api.BindAndValidate(c, &req); err != nil {
		api.BadRequest(c, err.Error())
		return
	}

	label, err := h.svc.CreateLabel(c.Request.Context(), req)
	if err != nil {
		api.Conflict(c, err.Error())
		return
	}

	api.Created(c, label)
}

// getStats 获取标签统计
// GET /api/v1/sharedlabels/stats?tenant_id=xxx
func (h *Handler) getStats(c *gin.Context) {
	tenantID := c.Query("tenant_id")

	stats, err := h.svc.GetStats(c.Request.Context(), tenantID)
	if err != nil {
		api.InternalError(c, err.Error())
		return
	}

	api.OK(c, stats)
}
