// Package shareportal 提供文件分享门户功能
package shareportal

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 分享门户 HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	sp := api.Group("/shareportal")
	{
		// 分享链接管理
		sp.POST("/links", h.CreateShare)
		sp.GET("/links/:id", h.GetShare)
		sp.PUT("/links/:id", h.UpdateShare)
		sp.DELETE("/links/:id", h.DeleteShare)

		// 短链访问
		sp.GET("/s/:shortURL", h.ShortURLAccess)

		// 密码验证
		sp.POST("/links/:id/validate", h.ValidateAccess)

		// 访问统计
		sp.GET("/links/:id/stats", h.GetStats)

		// 品牌配置
		sp.POST("/branding", h.SetBranding)
		sp.GET("/branding/:id", h.GetBranding)

		// 门户管理
		sp.POST("/portals", h.CreatePortal)
	}
}

// CreateShare 创建分享
func (h *Handlers) CreateShare(c *gin.Context) {
	var link ShareLink
	if err := c.ShouldBindJSON(&link); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	result, err := h.manager.CreateShare(c.Request.Context(), link)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// GetShare 获取分享
func (h *Handlers) GetShare(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少分享ID"})
		return
	}

	link, err := h.manager.GetShare(c.Request.Context(), id)
	if err != nil {
		if err == ErrShareNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, link)
}

// UpdateShare 更新分享
func (h *Handlers) UpdateShare(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少分享ID"})
		return
	}

	var updates ShareLink
	if err := c.ShouldBindJSON(&updates); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	result, err := h.manager.UpdateShare(c.Request.Context(), id, updates)
	if err != nil {
		if err == ErrShareNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// DeleteShare 删除分享
func (h *Handlers) DeleteShare(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少分享ID"})
		return
	}

	err := h.manager.DeleteShare(c.Request.Context(), id)
	if err != nil {
		if err == ErrShareNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// ShortURLAccess 短链访问
func (h *Handlers) ShortURLAccess(c *gin.Context) {
	shortURL := c.Param("shortURL")
	if shortURL == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少短链"})
		return
	}

	link, err := h.manager.GetShareByShortURL(c.Request.Context(), shortURL)
	if err != nil {
		if err == ErrShareNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "短链不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	// 记录查看访问
	h.manager.RecordAccess(c.Request.Context(), ShareAccess{
		ShareLinkID: link.ID,
		VisitorIP:   c.ClientIP(),
		VisitorUA:   c.Request.UserAgent(),
		Action:      ActionView,
	})

	c.JSON(http.StatusOK, link)
}

// ValidateAccess 验证密码
func (h *Handlers) ValidateAccess(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少分享ID"})
		return
	}

	var req struct {
		Password string `json:"password"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	valid, err := h.manager.ValidateAccess(c.Request.Context(), id, req.Password)
	if err != nil {
		switch err {
		case ErrShareNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在", "valid": false})
		case ErrShareInactive:
			c.JSON(http.StatusForbidden, gin.H{"error": "分享已停用", "valid": false})
		case ErrShareExpired:
			c.JSON(http.StatusGone, gin.H{"error": "分享已过期", "valid": false})
		case ErrMaxDownloadsExceeded:
			c.JSON(http.StatusForbidden, gin.H{"error": "已达最大下载次数", "valid": false})
		case ErrPasswordRequired:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "需要密码", "valid": false, "require_password": true})
		case ErrPasswordWrong:
			c.JSON(http.StatusUnauthorized, gin.H{"error": "密码错误", "valid": false})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error(), "valid": false})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"valid": valid})
}

// GetStats 获取统计
func (h *Handlers) GetStats(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少分享ID"})
		return
	}

	stats, err := h.manager.GetStats(c.Request.Context(), id)
	if err != nil {
		if err == ErrShareNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "分享不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// SetBranding 设置品牌
func (h *Handlers) SetBranding(c *gin.Context) {
	var branding ShareBranding
	if err := c.ShouldBindJSON(&branding); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	result, err := h.manager.SetBranding(c.Request.Context(), branding)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}

// GetBranding 获取品牌
func (h *Handlers) GetBranding(c *gin.Context) {
	id := c.Param("id")
	if id == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "缺少品牌ID"})
		return
	}

	branding, err := h.manager.GetBranding(c.Request.Context(), id)
	if err != nil {
		if err == ErrBrandingNotFound {
			c.JSON(http.StatusNotFound, gin.H{"error": "品牌配置不存在"})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, branding)
}

// CreatePortal 创建门户
func (h *Handlers) CreatePortal(c *gin.Context) {
	var portal SharePortal
	if err := c.ShouldBindJSON(&portal); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	result, err := h.manager.CreatePortal(c.Request.Context(), portal)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, result)
}
