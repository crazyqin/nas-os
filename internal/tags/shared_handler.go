package tags

import (
	"net/http"
	"strings"

	apiresponse "nas-os/internal/api"

	"github.com/gin-gonic/gin"
)

// SharedHandlers 共享标签 HTTP 处理器.
type SharedHandlers struct {
	manager *LabelManager
}

// NewSharedHandlers 创建共享标签处理器.
func NewSharedHandlers(mgr *LabelManager) *SharedHandlers {
	return &SharedHandlers{manager: mgr}
}

// RegisterRoutes 注册共享标签路由.
func (h *SharedHandlers) RegisterRoutes(api *gin.RouterGroup) {
	labels := api.Group("/labels")
	{
		labels.POST("", h.createLabel)
		labels.GET("", h.listLabels)
		labels.GET("/search", h.searchLabels)
		labels.GET("/:id", h.getLabel)
		labels.PUT("/:id", h.updateLabel)
		labels.DELETE("/:id", h.deleteLabel)
		labels.POST("/:id/share", h.shareLabel)
		labels.POST("/:id/unshare", h.unshareLabel)
		labels.POST("/:id/assign", h.assignLabel)
		labels.DELETE("/:id/assign/:fileId", h.removeLabel)
		labels.GET("/:id/files", h.getFilesByLabel)
		labels.GET("/:id/stats", h.getLabelStats)
	}
}

// createLabel 创建共享标签
// @Summary 创建共享标签
// @Description 创建一个新的共享标签
// @Tags shared-labels
// @Accept json
// @Produce json
// @Param request body SharedLabelInput true "标签参数"
// @Success 201 {object} api.Response{data=SharedLabel}
// @Failure 400 {object} api.Response
// @Failure 409 {object} api.Response
// @Router /labels [post].
func (h *SharedHandlers) createLabel(c *gin.Context) {
	var req SharedLabelInput
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.BadRequest(c, "无效的请求参数")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Name == "" {
		apiresponse.BadRequest(c, "标签名称不能为空")
		return
	}
	if req.Owner == "" {
		apiresponse.BadRequest(c, "标签所有者不能为空")
		return
	}

	label, err := h.manager.CreateLabel(req)
	if err != nil {
		if err == ErrSharedLabelExists {
			apiresponse.Conflict(c, "标签名称已存在")
			return
		}
		apiresponse.InternalError(c, "创建标签失败")
		return
	}

	apiresponse.CreatedWithMessage(c, "标签创建成功", label)
}

// listLabels 列出共享标签
// @Summary 列出共享标签
// @Description 列出当前用户可见的所有标签（拥有的+被分享的）
// @Tags shared-labels
// @Accept json
// @Produce json
// @Param owner query string true "当前用户ID"
// @Success 200 {object} api.Response{data=[]SharedLabel}
// @Router /labels [get].
func (h *SharedHandlers) listLabels(c *gin.Context) {
	owner := c.Query("owner")
	if owner == "" {
		apiresponse.BadRequest(c, "owner 参数不能为空")
		return
	}

	labels, err := h.manager.ListLabels(owner)
	if err != nil {
		apiresponse.InternalError(c, "获取标签列表失败")
		return
	}

	apiresponse.OK(c, labels)
}

// getLabel 获取共享标签
// @Summary 获取标签详情
// @Description 根据ID获取共享标签详情
// @Tags shared-labels
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Success 200 {object} api.Response{data=SharedLabel}
// @Failure 404 {object} api.Response
// @Router /labels/{id} [get].
func (h *SharedHandlers) getLabel(c *gin.Context) {
	id := c.Param("id")

	label, err := h.manager.GetLabel(id)
	if err != nil {
		if err == ErrSharedLabelNotFound {
			apiresponse.NotFound(c, "标签不存在")
			return
		}
		apiresponse.InternalError(c, "获取标签失败")
		return
	}

	apiresponse.OK(c, label)
}

// updateLabel 更新共享标签
// @Summary 更新共享标签
// @Description 更新标签信息（仅所有者可操作）
// @Tags shared-labels
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Param request body SharedLabelInput true "标签参数"
// @Success 200 {object} api.Response{data=SharedLabel}
// @Failure 400 {object} api.Response
// @Failure 403 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /labels/{id} [put].
func (h *SharedHandlers) updateLabel(c *gin.Context) {
	id := c.Param("id")

	var req SharedLabelInput
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.BadRequest(c, "无效的请求参数")
		return
	}

	req.Name = strings.TrimSpace(req.Name)
	if req.Owner == "" {
		apiresponse.BadRequest(c, "owner 不能为空")
		return
	}

	label, err := h.manager.UpdateLabel(id, req)
	if err != nil {
		switch err {
		case ErrSharedLabelNotFound:
			apiresponse.NotFound(c, "标签不存在")
		case ErrNotSharedOwner:
			apiresponse.Forbidden(c, "仅标签所有者可以修改")
		case ErrSharedLabelExists:
			apiresponse.Conflict(c, "标签名称已存在")
		default:
			apiresponse.InternalError(c, "更新标签失败")
		}
		return
	}

	apiresponse.OKWithMessage(c, "标签更新成功", label)
}

// deleteLabel 删除共享标签
// @Summary 删除共享标签
// @Description 删除标签及所有关联（仅所有者可操作）
// @Tags shared-labels
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Param owner query string true "当前用户ID"
// @Success 200 {object} api.Response
// @Failure 403 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /labels/{id} [delete].
func (h *SharedHandlers) deleteLabel(c *gin.Context) {
	id := c.Param("id")
	owner := c.Query("owner")

	if owner == "" {
		apiresponse.BadRequest(c, "owner 参数不能为空")
		return
	}

	err := h.manager.DeleteLabel(id, owner)
	if err != nil {
		switch err {
		case ErrSharedLabelNotFound:
			apiresponse.NotFound(c, "标签不存在")
		case ErrNotSharedOwner:
			apiresponse.Forbidden(c, "仅标签所有者可以删除")
		default:
			apiresponse.InternalError(c, "删除标签失败")
		}
		return
	}

	apiresponse.OKWithMessage(c, "标签已删除", nil)
}

// shareLabel 分享标签
// @Summary 分享标签给用户
// @Description 将标签分享给指定用户（仅所有者可操作）
// @Tags shared-labels
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Param request body ShareLabelInput true "分享用户列表"
// @Param owner query string true "当前用户ID"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 403 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /labels/{id}/share [post].
func (h *SharedHandlers) shareLabel(c *gin.Context) {
	id := c.Param("id")
	owner := c.Query("owner")

	if owner == "" {
		apiresponse.BadRequest(c, "owner 参数不能为空")
		return
	}

	var req ShareLabelInput
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.BadRequest(c, "无效的请求参数")
		return
	}

	if len(req.Users) == 0 {
		apiresponse.BadRequest(c, "分享用户列表不能为空")
		return
	}

	err := h.manager.ShareLabel(id, req.Users, owner)
	if err != nil {
		switch err {
		case ErrSharedLabelNotFound:
			apiresponse.NotFound(c, "标签不存在")
		case ErrNotSharedOwner:
			apiresponse.Forbidden(c, "仅标签所有者可以分享")
		default:
			apiresponse.InternalError(c, "分享标签失败")
		}
		return
	}

	apiresponse.OKWithMessage(c, "标签已分享", nil)
}

// unshareLabel 取消分享标签
// @Summary 取消分享标签
// @Description 取消标签对指定用户的分享（仅所有者可操作）
// @Tags shared-labels
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Param request body ShareLabelInput true "取消分享的用户列表"
// @Param owner query string true "当前用户ID"
// @Success 200 {object} api.Response
// @Failure 403 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /labels/{id}/unshare [post].
func (h *SharedHandlers) unshareLabel(c *gin.Context) {
	id := c.Param("id")
	owner := c.Query("owner")

	if owner == "" {
		apiresponse.BadRequest(c, "owner 参数不能为空")
		return
	}

	var req ShareLabelInput
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.BadRequest(c, "无效的请求参数")
		return
	}

	err := h.manager.UnshareLabel(id, req.Users, owner)
	if err != nil {
		switch err {
		case ErrSharedLabelNotFound:
			apiresponse.NotFound(c, "标签不存在")
		case ErrNotSharedOwner:
			apiresponse.Forbidden(c, "仅标签所有者可以取消分享")
		default:
			apiresponse.InternalError(c, "取消分享失败")
		}
		return
	}

	apiresponse.OKWithMessage(c, "已取消分享", nil)
}

// assignLabel 分配标签给文件
// @Summary 分配标签给文件
// @Description 将标签分配给指定文件
// @Tags shared-labels
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Param request body AssignLabelInput true "文件ID"
// @Success 200 {object} api.Response
// @Failure 400 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /labels/{id}/assign [post].
func (h *SharedHandlers) assignLabel(c *gin.Context) {
	id := c.Param("id")

	var req AssignLabelInput
	if err := c.ShouldBindJSON(&req); err != nil {
		apiresponse.BadRequest(c, "无效的请求参数")
		return
	}

	if req.FileID == "" {
		apiresponse.BadRequest(c, "fileId 不能为空")
		return
	}

	err := h.manager.AssignLabel(req.FileID, id)
	if err != nil {
		if err == ErrSharedLabelNotFound {
			apiresponse.NotFound(c, "标签不存在")
			return
		}
		apiresponse.InternalError(c, "分配标签失败")
		return
	}

	apiresponse.OKWithMessage(c, "标签已分配", nil)
}

// removeLabel 移除文件上的标签
// @Summary 移除文件上的标签
// @Description 从指定文件移除标签
// @Tags shared-labels
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Param fileId path string true "文件ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /labels/{id}/assign/{fileId} [delete].
func (h *SharedHandlers) removeLabel(c *gin.Context) {
	id := c.Param("id")
	fileID := c.Param("fileId")

	err := h.manager.RemoveLabel(fileID, id)
	if err != nil {
		if err == ErrLabelNotAssigned {
			apiresponse.NotFound(c, "标签未分配给该文件")
			return
		}
		apiresponse.InternalError(c, "移除标签失败")
		return
	}

	apiresponse.OKWithMessage(c, "标签已移除", nil)
}

// getFilesByLabel 获取标签关联的文件
// @Summary 获取标签关联的文件
// @Description 获取指定标签关联的所有文件ID列表
// @Tags shared-labels
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Success 200 {object} api.Response{data=[]string}
// @Failure 404 {object} api.Response
// @Router /labels/{id}/files [get].
func (h *SharedHandlers) getFilesByLabel(c *gin.Context) {
	id := c.Param("id")

	// 验证标签存在
	_, err := h.manager.GetLabel(id)
	if err != nil {
		if err == ErrSharedLabelNotFound {
			apiresponse.NotFound(c, "标签不存在")
			return
		}
		apiresponse.InternalError(c, "获取标签失败")
		return
	}

	files, err := h.manager.GetFilesByLabel(id)
	if err != nil {
		apiresponse.InternalError(c, "获取文件列表失败")
		return
	}

	apiresponse.OK(c, files)
}

// searchLabels 搜索共享标签
// @Summary 搜索共享标签
// @Description 按名称或描述模糊搜索标签
// @Tags shared-labels
// @Accept json
// @Produce json
// @Param q query string true "搜索关键词"
// @Param owner query string true "当前用户ID"
// @Success 200 {object} api.Response{data=[]SharedLabel}
// @Failure 400 {object} api.Response
// @Router /labels/search [get].
func (h *SharedHandlers) searchLabels(c *gin.Context) {
	q := c.Query("q")
	owner := c.Query("owner")

	if q == "" {
		apiresponse.BadRequest(c, "搜索关键词不能为空")
		return
	}
	if owner == "" {
		apiresponse.BadRequest(c, "owner 参数不能为空")
		return
	}

	labels, err := h.manager.SearchLabels(q, owner)
	if err != nil {
		apiresponse.InternalError(c, "搜索标签失败")
		return
	}

	apiresponse.OK(c, labels)
}

// getLabelStats 获取标签统计
// @Summary 获取标签统计
// @Description 获取标签的文件数和分享数
// @Tags shared-labels
// @Accept json
// @Produce json
// @Param id path string true "标签ID"
// @Success 200 {object} api.Response
// @Failure 404 {object} api.Response
// @Router /labels/{id}/stats [get].
func (h *SharedHandlers) getLabelStats(c *gin.Context) {
	id := c.Param("id")

	// 验证标签存在
	_, err := h.manager.GetLabel(id)
	if err != nil {
		if err == ErrSharedLabelNotFound {
			apiresponse.NotFound(c, "标签不存在")
			return
		}
		apiresponse.InternalError(c, "获取标签失败")
		return
	}

	fileCount, shareCount, err := h.manager.GetLabelStats(id)
	if err != nil {
		apiresponse.InternalError(c, "获取统计信息失败")
		return
	}

	apiresponse.OK(c, gin.H{
		"labelId":    id,
		"fileCount":  fileCount,
		"shareCount": shareCount,
	})
}

// ensure compile-time check that SharedHandlers satisfies handler interface.
var _ http.Handler = nil //nolint:unused
