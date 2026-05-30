package websharepro

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler WebShare HTTP处理器
type Handler struct {
	manager *WebShareManager
}

// NewHandler 创建处理器
func NewHandler(manager *WebShareManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/webshare")
	{
		group.GET("/links", h.ListLinks)
		group.GET("/links/:id", h.GetLink)
		group.POST("/links", h.CreateLink)
		group.DELETE("/links/:id", h.DeleteLink)
		group.GET("/links/:id/access-log", h.GetAccessLog)
		group.GET("/stats", h.GetStats)
		group.POST("/cleanup", h.CleanupExpired)
	}
}

// CreateLinkRequest 创建链接请求
type CreateLinkRequest struct {
	Path         string          `json:"path" binding:"required"`
	Name         string          `json:"name" binding:"required"`
	CreatedBy    string          `json:"createdBy" binding:"required"`
	Permission   SharePermission `json:"permission"`
	ExpiryHours  int             `json:"expiryHours"`
	Password     string          `json:"password"`
}

// ListLinks 列出链接
func (h *Handler) ListLinks(c *gin.Context) {
	createdBy := c.Query("createdBy")
	activeOnly := c.Query("active") == "true"

	links := h.manager.ListShareLinks(createdBy, activeOnly)
	c.JSON(http.StatusOK, gin.H{"links": links})
}

// GetLink 获取链接详情
func (h *Handler) GetLink(c *gin.Context) {
	id := c.Param("id")
	link, ok := h.manager.GetShareLink(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	c.JSON(http.StatusOK, link)
}

// CreateLink 创建链接
func (h *Handler) CreateLink(c *gin.Context) {
	var req CreateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	perm := req.Permission
	if perm == "" {
		perm = PermReadOnly
	}

	link, err := h.manager.CreateShareLink(req.Path, req.Name, req.CreatedBy, perm, req.ExpiryHours, req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, link)
}

// DeleteLink 删除链接
func (h *Handler) DeleteLink(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteShareLink(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "link deleted"})
}

// GetAccessLog 获取访问日志
func (h *Handler) GetAccessLog(c *gin.Context) {
	id := c.Param("id")
	link, ok := h.manager.GetShareLink(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"accessLog": link.AccessLog})
}

// GetStats 获取统计
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}

// CleanupExpired 清理过期链接
func (h *Handler) CleanupExpired(c *gin.Context) {
	count := h.manager.CleanupExpired()
	c.JSON(http.StatusOK, gin.H{"cleaned": count})
}
