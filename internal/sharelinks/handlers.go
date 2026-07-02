package sharelinks

import (
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

// Handler HTTP处理器.
type Handler struct {
	manager *LinkManager
}

// NewHandler 创建处理器.
func NewHandler(manager *LinkManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	group := rg.Group("/sharelinks")
	{
		// 链接管理
		group.GET("", h.ListLinks)
		group.GET("/:id", h.GetLink)
		group.POST("", h.CreateLink)
		group.PUT("/:id", h.UpdateLink)
		group.DELETE("/:id", h.DeleteLink)

		// 链接状态
		group.POST("/:id/disable", h.DisableLink)
		group.POST("/:id/enable", h.EnableLink)

		// 访问控制
		group.GET("/:id/access", h.AccessLink)
		group.POST("/:id/download", h.DownloadFile)
		group.GET("/:id/preview", h.PreviewFile)

		// 统计
		group.GET("/:id/stats", h.GetLinkStats)
		group.GET("/stats", h.GetGlobalStats)

		// 二维码
		group.GET("/:id/qrcode", h.GenerateQRCode)

		// 短链接访问
		group.GET("/s/:shortcode", h.AccessByShortCode)

		// 维护
		group.POST("/cleanup", h.CleanupExpired)
	}
}

// CreateLinkRequest 创建链接请求.
type CreateLinkRequest struct {
	Path             string   `json:"path" binding:"required"`
	Name             string   `json:"name" binding:"required"`
	CreatedBy        string   `json:"createdBy" binding:"required"`
	Type             LinkType `json:"type"`
	Password         string   `json:"password"`
	ExpiryHours      int      `json:"expiryHours"`
	MaxDownloads     int      `json:"maxDownloads"`
	Description      string   `json:"description"`
	Tags             []string `json:"tags"`
	RefererWhitelist []string `json:"refererWhitelist"`
	BatchPaths       []string `json:"batchPaths"`
}

// UpdateLinkRequest 更新链接请求.
type UpdateLinkRequest struct {
	Name             *string  `json:"name"`
	Description      *string  `json:"description"`
	Tags             []string `json:"tags"`
	RefererWhitelist []string `json:"refererWhitelist"`
	ExpiryHours      *int     `json:"expiryHours"`
	MaxDownloads     *int     `json:"maxDownloads"`
}

// AccessLinkRequest 访问链接请求.
type AccessLinkRequest struct {
	Password string `json:"password"`
}

// ListLinks 列出链接.
func (h *Handler) ListLinks(c *gin.Context) {
	createdBy := c.Query("createdBy")
	activeOnly := c.Query("active") == "true"

	links := h.manager.ListLinks(createdBy, activeOnly)
	c.JSON(http.StatusOK, gin.H{"links": links})
}

// GetLink 获取链接详情.
func (h *Handler) GetLink(c *gin.Context) {
	id := c.Param("id")
	link, ok := h.manager.GetLink(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "link not found"})
		return
	}
	c.JSON(http.StatusOK, link)
}

// CreateLink 创建链接.
func (h *Handler) CreateLink(c *gin.Context) {
	var req CreateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	linkType := req.Type
	if linkType == "" {
		linkType = LinkTypePublic
	}

	// 构建选项
	opts := []LinkOption{}
	if req.Password != "" {
		opts = append(opts, WithPassword(req.Password))
	}
	if req.ExpiryHours > 0 {
		opts = append(opts, WithExpiry(req.ExpiryHours))
	}
	if req.MaxDownloads > 0 {
		opts = append(opts, WithMaxDownloads(req.MaxDownloads))
	}
	if req.Description != "" {
		opts = append(opts, WithDescription(req.Description))
	}
	if len(req.Tags) > 0 {
		opts = append(opts, WithTags(req.Tags))
	}
	if len(req.RefererWhitelist) > 0 {
		opts = append(opts, WithRefererWhitelist(req.RefererWhitelist))
	}
	if len(req.BatchPaths) > 0 {
		opts = append(opts, WithBatchPaths(req.BatchPaths))
	}

	link, err := h.manager.CreateLink(req.Path, req.Name, req.CreatedBy, linkType, opts...)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, link)
}

// UpdateLink 更新链接.
func (h *Handler) UpdateLink(c *gin.Context) {
	id := c.Param("id")
	var req UpdateLinkRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	opts := []LinkOption{}
	if req.Name != nil {
		opts = append(opts, func(l *ShareLink) { l.Name = *req.Name })
	}
	if req.Description != nil {
		opts = append(opts, WithDescription(*req.Description))
	}
	if len(req.Tags) > 0 {
		opts = append(opts, WithTags(req.Tags))
	}
	if len(req.RefererWhitelist) > 0 {
		opts = append(opts, WithRefererWhitelist(req.RefererWhitelist))
	}
	if req.ExpiryHours != nil {
		opts = append(opts, WithExpiry(*req.ExpiryHours))
	}
	if req.MaxDownloads != nil {
		opts = append(opts, WithMaxDownloads(*req.MaxDownloads))
	}

	link, err := h.manager.UpdateLink(id, opts...)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, link)
}

// DeleteLink 删除链接.
func (h *Handler) DeleteLink(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DeleteLink(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "link deleted"})
}

// DisableLink 禁用链接.
func (h *Handler) DisableLink(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.DisableLink(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "link disabled"})
}

// EnableLink 启用链接.
func (h *Handler) EnableLink(c *gin.Context) {
	id := c.Param("id")
	if err := h.manager.EnableLink(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"message": "link enabled"})
}

// AccessLink 访问链接.
func (h *Handler) AccessLink(c *gin.Context) {
	id := c.Param("id")

	var req AccessLinkRequest
	c.ShouldBindJSON(&req)

	referer := c.GetHeader("Referer")

	link, err := h.manager.ValidateAccess(id, req.Password, referer)
	if err != nil {
		switch err {
		case ErrLinkNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case ErrLinkExpired:
			c.JSON(http.StatusGone, gin.H{"error": err.Error()})
		case ErrLinkDisabled:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case ErrDownloadLimit:
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		case ErrInvalidPassword:
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		case ErrRefererDenied:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// 记录访问
	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	h.manager.RecordAccess(id, ip, userAgent, referer, "view")

	c.JSON(http.StatusOK, gin.H{
		"link":    link,
		"message": "access granted",
	})
}

// DownloadFile 下载文件.
func (h *Handler) DownloadFile(c *gin.Context) {
	id := c.Param("id")

	var req AccessLinkRequest
	c.ShouldBindJSON(&req)

	referer := c.GetHeader("Referer")

	link, err := h.manager.ValidateAccess(id, req.Password, referer)
	if err != nil {
		switch err {
		case ErrLinkNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case ErrLinkExpired:
			c.JSON(http.StatusGone, gin.H{"error": err.Error()})
		case ErrLinkDisabled:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case ErrDownloadLimit:
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		case ErrInvalidPassword:
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		case ErrRefererDenied:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// 记录下载
	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	h.manager.RecordAccess(id, ip, userAgent, referer, "download")

	c.JSON(http.StatusOK, gin.H{
		"link":    link,
		"message": "download initiated",
	})
}

// PreviewFile 预览文件.
func (h *Handler) PreviewFile(c *gin.Context) {
	id := c.Param("id")

	var req AccessLinkRequest
	c.ShouldBindJSON(&req)

	referer := c.GetHeader("Referer")

	link, err := h.manager.ValidateAccess(id, req.Password, referer)
	if err != nil {
		switch err {
		case ErrLinkNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case ErrLinkExpired:
			c.JSON(http.StatusGone, gin.H{"error": err.Error()})
		case ErrLinkDisabled:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case ErrInvalidPassword:
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		case ErrRefererDenied:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// 记录预览
	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	h.manager.RecordAccess(id, ip, userAgent, referer, "preview")

	c.JSON(http.StatusOK, gin.H{
		"link":        link,
		"previewType": link.PreviewType,
		"message":     "preview granted",
	})
}

// GetLinkStats 获取链接统计.
func (h *Handler) GetLinkStats(c *gin.Context) {
	id := c.Param("id")
	stats, err := h.manager.GetLinkStats(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, stats)
}

// GetGlobalStats 获取全局统计.
func (h *Handler) GetGlobalStats(c *gin.Context) {
	stats := h.manager.GetGlobalStats()
	c.JSON(http.StatusOK, stats)
}

// GenerateQRCode 生成二维码.
func (h *Handler) GenerateQRCode(c *gin.Context) {
	id := c.Param("id")
	data, err := h.manager.GenerateQRCodeData(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{
		"url":    data,
		"qrData": data,
	})
}

// AccessByShortCode 通过短码访问.
func (h *Handler) AccessByShortCode(c *gin.Context) {
	shortCode := c.Param("shortcode")

	// 从查询参数获取密码
	password := c.Query("password")
	if password == "" {
		var req AccessLinkRequest
		c.ShouldBindJSON(&req)
		password = req.Password
	}

	referer := c.GetHeader("Referer")

	link, err := h.manager.ValidateAccessByShortCode(shortCode, password, referer)
	if err != nil {
		switch err {
		case ErrInvalidShortCode:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case ErrLinkExpired:
			c.JSON(http.StatusGone, gin.H{"error": err.Error()})
		case ErrLinkDisabled:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		case ErrDownloadLimit:
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		case ErrInvalidPassword:
			c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		case ErrRefererDenied:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	// 记录访问
	ip := c.ClientIP()
	userAgent := c.GetHeader("User-Agent")
	h.manager.RecordAccess(link.ID, ip, userAgent, referer, "view")

	c.JSON(http.StatusOK, gin.H{
		"link":    link,
		"message": "access granted",
	})
}

// CleanupExpired 清理过期链接.
func (h *Handler) CleanupExpired(c *gin.Context) {
	count := h.manager.CleanupExpired()
	c.JSON(http.StatusOK, gin.H{"cleaned": count})
}

// getClientIP 获取客户端IP（支持代理）.
func getClientIP(c *gin.Context) string {
	ip := c.GetHeader("X-Forwarded-For")
	if ip != "" {
		return strings.Split(ip, ",")[0]
	}
	ip = c.GetHeader("X-Real-IP")
	if ip != "" {
		return ip
	}
	return c.ClientIP()
}
