// Package contacts 提供 REST API 处理器
package contacts

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers 联系人 API 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	contacts := r.Group("/contacts")
	{
		// 联系人 CRUD
		contacts.GET("", h.listContacts)
		contacts.POST("", h.createContact)
		contacts.GET("/:id", h.getContact)
		contacts.PUT("/:id", h.updateContact)
		contacts.DELETE("/:id", h.deleteContact)

		// 搜索
		contacts.GET("/search", h.searchContacts)

		// 分组管理
		contacts.GET("/groups", h.listGroups)
		contacts.POST("/groups", h.createGroup)
		contacts.GET("/groups/:id", h.getGroup)
		contacts.PUT("/groups/:id", h.updateGroup)
		contacts.DELETE("/groups/:id", h.deleteGroup)
		contacts.POST("/groups/:id/contacts", h.addContactsToGroup)
		contacts.DELETE("/groups/:id/contacts", h.removeContactsFromGroup)

		// vCard 导入导出
		contacts.GET("/:id/vcard", h.exportVCard)
		contacts.POST("/import/vcard", h.importVCard)
		contacts.POST("/export/vcard", h.exportVCardBatch)

		// CSV 导入
		contacts.POST("/import/csv", h.importCSV)

		// 去重
		contacts.GET("/duplicates", h.findDuplicates)
		contacts.POST("/merge", h.mergeContacts)

		// 分享
		contacts.POST("/groups/:id/share", h.shareGroup)
		contacts.GET("/groups/:id/shares", h.getShares)
		contacts.DELETE("/shares/:id", h.revokeShare)

		// 统计
		contacts.GET("/stats", h.getStats)
	}
}

// response 标准响应
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// listContacts 列出联系人
func (h *Handlers) listContacts(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	offsetStr := c.DefaultQuery("offset", "0")

	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 50
	}
	offset, err := strconv.Atoi(offsetStr)
	if err != nil || offset < 0 {
		offset = 0
	}

	contacts := h.manager.ListContacts(limit, offset)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    contacts,
	})
}

// createContact 创建联系人
func (h *Handlers) createContact(c *gin.Context) {
	var req ContactCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	contact, err := h.manager.CreateContact(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "contact created",
		Data:    contact,
	})
}

// getContact 获取联系人
func (h *Handlers) getContact(c *gin.Context) {
	id := c.Param("id")
	contact, err := h.manager.GetContact(id)
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
		Data:    contact,
	})
}

// updateContact 更新联系人
func (h *Handlers) updateContact(c *gin.Context) {
	id := c.Param("id")
	var req ContactUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	contact, err := h.manager.UpdateContact(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "contact updated",
		Data:    contact,
	})
}

// deleteContact 删除联系人
func (h *Handlers) deleteContact(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteContact(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "contact deleted",
	})
}

// searchContacts 搜索联系人
func (h *Handlers) searchContacts(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	contacts := h.manager.SearchContacts(&req)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    contacts,
	})
}

// listGroups 列出分组
func (h *Handlers) listGroups(c *gin.Context) {
	groups := h.manager.ListGroups()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    groups,
	})
}

// createGroup 创建分组
func (h *Handlers) createGroup(c *gin.Context) {
	var req ContactGroupCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	group, err := h.manager.CreateGroup(&req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "group created",
		Data:    group,
	})
}

// getGroup 获取分组
func (h *Handlers) getGroup(c *gin.Context) {
	id := c.Param("id")
	group, err := h.manager.GetGroup(id)
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
		Data:    group,
	})
}

// updateGroup 更新分组
func (h *Handlers) updateGroup(c *gin.Context) {
	id := c.Param("id")
	var req ContactGroupUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	group, err := h.manager.UpdateGroup(id, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "group updated",
		Data:    group,
	})
}

// deleteGroup 删除分组
func (h *Handlers) deleteGroup(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteGroup(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "group deleted",
	})
}

// addContactsToGroup 添加联系人到分组
func (h *Handlers) addContactsToGroup(c *gin.Context) {
	groupID := c.Param("id")
	var req struct {
		ContactIDs []string `json:"contact_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.AddContactsToGroup(groupID, req.ContactIDs); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "contacts added to group",
	})
}

// removeContactsFromGroup 从分组移除联系人
func (h *Handlers) removeContactsFromGroup(c *gin.Context) {
	groupID := c.Param("id")
	var req struct {
		ContactIDs []string `json:"contact_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	if err := h.manager.RemoveContactsFromGroup(groupID, req.ContactIDs); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "contacts removed from group",
	})
}

// exportVCard 导出单个联系人 vCard
func (h *Handlers) exportVCard(c *gin.Context) {
	id := c.Param("id")
	vcard, err := h.manager.ExportVCard(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.Header("Content-Type", "text/vcard")
	c.Header("Content-Disposition", "attachment; filename=contact.vcf")
	c.String(http.StatusOK, vcard)
}

// importVCard 导入 vCard
func (h *Handlers) importVCard(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
		GroupID string `json:"group_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.ImportVCard(req.Content, req.GroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "vcard imported",
		Data:    result,
	})
}

// exportVCardBatch 批量导出 vCard
func (h *Handlers) exportVCardBatch(c *gin.Context) {
	var req struct {
		ContactIDs []string `json:"contact_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	vcard, err := h.manager.ExportVCardBatch(req.ContactIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.Header("Content-Type", "text/vcard")
	c.Header("Content-Disposition", "attachment; filename=contacts.vcf")
	c.String(http.StatusOK, vcard)
}

// importCSV 导入 CSV
func (h *Handlers) importCSV(c *gin.Context) {
	var req struct {
		Content string `json:"content" binding:"required"`
		GroupID string `json:"group_id"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.ImportCSV(req.Content, req.GroupID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "csv imported",
		Data:    result,
	})
}

// findDuplicates 查找重复联系人
func (h *Handlers) findDuplicates(c *gin.Context) {
	duplicates := h.manager.FindDuplicates()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    duplicates,
	})
}

// mergeContacts 合并联系人
func (h *Handlers) mergeContacts(c *gin.Context) {
	var req struct {
		KeepID   string   `json:"keep_id" binding:"required"`
		MergeIDs []string `json:"merge_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	result, err := h.manager.MergeContacts(req.KeepID, req.MergeIDs)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "contacts merged",
		Data:    result,
	})
}

// shareGroup 分享分组
func (h *Handlers) shareGroup(c *gin.Context) {
	groupID := c.Param("id")
	var req ShareRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{
			Code:    1,
			Message: "invalid request: " + err.Error(),
		})
		return
	}

	req.GroupID = groupID
	share, err := h.manager.ShareGroup(&req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, response{
		Code:    0,
		Message: "group shared",
		Data:    share,
	})
}

// getShares 获取分组分享信息
func (h *Handlers) getShares(c *gin.Context) {
	groupID := c.Param("id")
	shares := h.manager.GetShares(groupID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data:    shares,
	})
}

// revokeShare 撤销分享
func (h *Handlers) revokeShare(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.RevokeShare(id); err != nil {
		c.JSON(http.StatusNotFound, response{
			Code:    1,
			Message: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "share revoked",
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
