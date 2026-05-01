// Package contacts 提供联系人 HTTP API 处理器
package contacts

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 联系人 HTTP 处理器.
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	contacts := api.Group("/contacts")
	{
		contacts.POST("", h.CreateContact)
		contacts.GET("", h.ListContacts)
		contacts.GET("/search", h.Search)
		contacts.GET("/:id", h.GetContact)
		contacts.PUT("/:id", h.UpdateContact)
		contacts.DELETE("/:id", h.DeleteContact)
		contacts.POST("/batch-delete", h.BatchDelete)
		contacts.GET("/:id/vcard", h.ExportVCard)
		contacts.POST("/import-vcard", h.ImportVCard)
		contacts.GET("/duplicates", h.DetectDuplicates)
		// 分组
		contacts.POST("/groups", h.CreateGroup)
		contacts.GET("/groups", h.ListGroups)
		contacts.DELETE("/groups/:id", h.DeleteGroup)
		contacts.POST("/groups/:gid/contacts/:cid", h.AddToGroup)
		contacts.DELETE("/groups/:gid/contacts/:cid", h.RemoveFromGroup)
	}
}

func (h *Handlers) CreateContact(c *gin.Context) {
	var req CreateContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	contact, err := h.mgr.CreateContact(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, contact)
}

func (h *Handlers) GetContact(c *gin.Context) {
	id := c.Param("id")
	contact, err := h.mgr.GetContact(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, contact)
}

func (h *Handlers) UpdateContact(c *gin.Context) {
	id := c.Param("id")
	var req UpdateContactRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	contact, err := h.mgr.UpdateContact(id, req)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, contact)
}

func (h *Handlers) DeleteContact(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.DeleteContact(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handlers) ListContacts(c *gin.Context) {
	groupID := c.Query("group_id")
	contacts := h.mgr.ListContacts(groupID)
	c.JSON(http.StatusOK, contacts)
}

func (h *Handlers) Search(c *gin.Context) {
	q := c.Query("q")
	if q == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "搜索关键词不能为空"})
		return
	}
	results := h.mgr.Search(q)
	c.JSON(http.StatusOK, results)
}

func (h *Handlers) ExportVCard(c *gin.Context) {
	id := c.Param("id")
	vcard, err := h.mgr.ExportVCard(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.Header("Content-Type", "text/vcard")
	c.String(http.StatusOK, vcard)
}

func (h *Handlers) ImportVCard(c *gin.Context) {
	data, err := c.GetRawData()
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	contacts, err := h.mgr.ImportVCard(string(data))
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, contacts)
}

func (h *Handlers) DetectDuplicates(c *gin.Context) {
	duplicates := h.mgr.DetectDuplicates()
	c.JSON(http.StatusOK, duplicates)
}

func (h *Handlers) BatchDelete(c *gin.Context) {
	var req struct {
		IDs []string `json:"ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	count := h.mgr.BatchDelete(req.IDs)
	c.JSON(http.StatusOK, gin.H{"deleted": count})
}

func (h *Handlers) CreateGroup(c *gin.Context) {
	var req CreateGroupRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	group, err := h.mgr.CreateGroup(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, group)
}

func (h *Handlers) ListGroups(c *gin.Context) {
	groups := h.mgr.ListGroups()
	c.JSON(http.StatusOK, groups)
}

func (h *Handlers) DeleteGroup(c *gin.Context) {
	id := c.Param("id")
	if err := h.mgr.DeleteGroup(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已删除"})
}

func (h *Handlers) AddToGroup(c *gin.Context) {
	gid := c.Param("gid")
	cid := c.Param("cid")
	if err := h.mgr.AddToGroup(cid, gid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已添加"})
}

func (h *Handlers) RemoveFromGroup(c *gin.Context) {
	gid := c.Param("gid")
	cid := c.Param("cid")
	if err := h.mgr.RemoveFromGroup(cid, gid); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "已移除"})
}
