// Package contactsmgr 提供 REST API 处理器
package contactsmgr

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 通讯录模块 API 处理器.
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由到 /api/contacts 路由组.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	contacts := r.Group("/contacts")
	{
		// 联系人 CRUD
		contacts.GET("", h.listContacts)
		contacts.POST("", h.createContact)
		contacts.GET("/:id", h.getContact)
		contacts.PUT("/:id", h.updateContact)
		contacts.DELETE("/:id", h.deleteContact)

		// 联系人组 CRUD
		contacts.GET("/groups", h.listGroups)
		contacts.POST("/groups", h.createGroup)
		contacts.GET("/groups/:id", h.getGroup)
		contacts.PUT("/groups/:id", h.updateGroup)
		contacts.DELETE("/groups/:id", h.deleteGroup)

		// 组成员管理
		contacts.GET("/groups/:id/contacts", h.listContactsByGroup)
		contacts.POST("/groups/:id/contacts", h.addContactsToGroup)
		contacts.DELETE("/groups/:id/contacts", h.removeContactsFromGroup)

		// 搜索
		contacts.GET("/search", h.searchContacts)

		// vCard 导入导出
		contacts.POST("/import", h.importVCard)
		contacts.GET("/export", h.exportVCard)
		contacts.POST("/export", h.exportMultipleVCard)

		// 去重
		contacts.POST("/deduplicate", h.findDuplicates)
		contacts.POST("/merge", h.mergeContacts)

		// 收藏
		contacts.GET("/favorites", h.listFavorites)

		// 统计
		contacts.GET("/stats", h.getStats)
	}
}

// ========== 联系人处理 ==========

func (h *Handlers) listContacts(c *gin.Context) {
	contacts := h.manager.ListContacts()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(contacts),
			"contacts": contacts,
		},
	})
}

func (h *Handlers) createContact(c *gin.Context) {
	var req CreateContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	contact := h.manager.CreateContact(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: contact})
}

func (h *Handlers) getContact(c *gin.Context) {
	id := c.Param("id")
	contact, err := h.manager.GetContact(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: contact})
}

func (h *Handlers) updateContact(c *gin.Context) {
	id := c.Param("id")
	var req UpdateContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	contact, err := h.manager.UpdateContact(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: contact})
}

func (h *Handlers) deleteContact(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteContact(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

// ========== 联系人组处理 ==========

func (h *Handlers) listGroups(c *gin.Context) {
	groups := h.manager.ListGroups()
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":   len(groups),
			"groups":  groups,
		},
	})
}

func (h *Handlers) createGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	group := h.manager.CreateGroup(req)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "created", Data: group})
}

func (h *Handlers) getGroup(c *gin.Context) {
	id := c.Param("id")
	group, err := h.manager.GetGroup(id)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: group})
}

func (h *Handlers) updateGroup(c *gin.Context) {
	id := c.Param("id")
	var req UpdateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	group, err := h.manager.UpdateGroup(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "updated", Data: group})
}

func (h *Handlers) deleteGroup(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteGroup(id); err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "deleted"})
}

func (h *Handlers) listContactsByGroup(c *gin.Context) {
	groupID := c.Param("id")
	contacts := h.manager.ListContactsByGroup(groupID)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(contacts),
			"contacts": contacts,
		},
	})
}

func (h *Handlers) addContactsToGroup(c *gin.Context) {
	groupID := c.Param("id")
	var req AddContactsToGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.AddContactsToGroup(groupID, req.ContactIDs); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "added"})
}

func (h *Handlers) removeContactsFromGroup(c *gin.Context) {
	groupID := c.Param("id")
	var req AddContactsToGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	if err := h.manager.RemoveContactsFromGroup(groupID, req.ContactIDs); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "removed"})
}

// ========== 搜索处理 ==========

func (h *Handlers) searchContacts(c *gin.Context) {
	query := c.Query("q")
	if query == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "query parameter 'q' is required"})
		return
	}

	groupID := c.Query("group_id")

	var contacts []*Contact
	if groupID != "" {
		contacts = h.manager.SearchContactsInGroup(query, groupID)
	} else {
		contacts = h.manager.SearchContacts(query)
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(contacts),
			"query":    query,
			"contacts": contacts,
		},
	})
}

// ========== vCard 导入导出处理 ==========

func (h *Handlers) importVCard(c *gin.Context) {
	var req ImportVCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	// 解析 vCard (简化版，实际应使用 vCard 解析库)
	vcard := VCard{
		Version:   "3.0",
		FirstName: "Imported",
		LastName:  "Contact",
		Notes:     req.Content,
	}

	contact := h.manager.ImportVCard(vcard, req.GroupID)
	c.JSON(http.StatusCreated, response{Code: 0, Message: "imported", Data: contact})
}

func (h *Handlers) exportVCard(c *gin.Context) {
	contactID := c.Query("contact_id")
	if contactID == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "contact_id is required"})
		return
	}

	vcard, err := h.manager.ExportVCard(contactID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: vcard})
}

func (h *Handlers) exportMultipleVCard(c *gin.Context) {
	var req ExportVCardRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	var vcards []*VCard
	if len(req.ContactIDs) == 0 {
		vcards = h.manager.ExportAllVCard()
	} else {
		var err error
		vcards, err = h.manager.ExportMultipleVCard(req.ContactIDs)
		if err != nil {
			c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
			return
		}
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":  len(vcards),
			"vcards": vcards,
		},
	})
}

// ========== 去重处理 ==========

func (h *Handlers) findDuplicates(c *gin.Context) {
	var req DeduplicateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.Field = "auto"
	}

	duplicates := h.manager.FindDuplicates(req.Field)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":      len(duplicates),
			"duplicates": duplicates,
		},
	})
}

func (h *Handlers) mergeContacts(c *gin.Context) {
	var req struct {
		PrimaryID string   `json:"primary_id" binding:"required"`
		MergeIDs  []string `json:"merge_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	contact, err := h.manager.MergeContacts(req.PrimaryID, req.MergeIDs)
	if err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "merged", Data: contact})
}

// ========== 收藏处理 ==========

func (h *Handlers) listFavorites(c *gin.Context) {
	allContacts := h.manager.ListContacts()
	var favorites []*Contact
	for _, c := range allContacts {
		if c.IsFavorite {
			favorites = append(favorites, c)
		}
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(favorites),
			"contacts": favorites,
		},
	})
}

// ========== 统计信息 ==========

func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: stats})
}
