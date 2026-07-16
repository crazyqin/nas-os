// Package devportal handlers - HTTP API
package devportal

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handlers HTTP处理器.
type Handlers struct {
	mgr *Manager
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *Manager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/devportal")
	{
		// API密钥
		g.GET("/apikeys", h.ListAPIKeys)
		g.POST("/apikeys", h.CreateAPIKey)
		g.GET("/apikeys/:id", h.GetAPIKey)
		g.POST("/apikeys/:id/revoke", h.RevokeAPIKey)

		// Webhook
		g.GET("/webhooks", h.ListWebhooks)
		g.POST("/webhooks", h.RegisterWebhook)
		g.GET("/webhooks/:id", h.GetWebhook)
		g.PUT("/webhooks/:id", h.UpdateWebhook)
		g.DELETE("/webhooks/:id", h.DeleteWebhook)
		g.GET("/webhooks/:id/deliveries", h.ListDeliveries)
		g.POST("/deliveries/:id/retry", h.RetryDelivery)

		// 开发者应用
		g.GET("/apps", h.ListApps)
		g.POST("/apps", h.RegisterApp)
		g.GET("/apps/:id", h.GetApp)
		g.DELETE("/apps/:id", h.DeleteApp)

		// OAuth2
		g.POST("/oauth/token", h.IssueToken)

		// API文档
		g.GET("/openapi.json", h.GetAPISpec)

		// SDK
		g.GET("/sdk/:lang", h.GenerateSDK)

		// 使用量
		g.GET("/usage/:owner_id", h.GetUsageStats)

		// 统计
		g.GET("/stats", h.GetStats)
	}
}

// ==================== API密钥 ====================

func (h *Handlers) ListAPIKeys(c *gin.Context) {
	ownerID := c.Query("owner_id")
	keys := h.mgr.ListAPIKeys(ownerID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": keys, "total": len(keys)})
}

func (h *Handlers) CreateAPIKey(c *gin.Context) {
	var req struct {
		Name       string     `json:"name"`
		OwnerID    string     `json:"owner_id"`
		Scopes     []APIScope `json:"scopes"`
		RateLimit  int        `json:"rate_limit"`
		DailyQuota int        `json:"daily_quota"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	key, err := h.mgr.CreateAPIKey(req.Name, req.OwnerID, req.Scopes, req.RateLimit, req.DailyQuota)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": key})
}

func (h *Handlers) GetAPIKey(c *gin.Context) {
	key, err := h.mgr.GetAPIKey(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": key})
}

func (h *Handlers) RevokeAPIKey(c *gin.Context) {
	if err := h.mgr.RevokeAPIKey(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "revoked"})
}

// ==================== Webhook ====================

func (h *Handlers) ListWebhooks(c *gin.Context) {
	ownerID := c.Query("owner_id")
	webhooks := h.mgr.ListWebhooks(ownerID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": webhooks, "total": len(webhooks)})
}

func (h *Handlers) RegisterWebhook(c *gin.Context) {
	var req struct {
		Name    string         `json:"name"`
		URL     string         `json:"url"`
		OwnerID string         `json:"owner_id"`
		Events  []WebhookEvent `json:"events"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	wh, err := h.mgr.RegisterWebhook(req.Name, req.URL, req.OwnerID, req.Events)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": wh})
}

func (h *Handlers) GetWebhook(c *gin.Context) {
	wh, err := h.mgr.GetWebhook(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": wh})
}

func (h *Handlers) UpdateWebhook(c *gin.Context) {
	var req struct {
		Name   string         `json:"name"`
		URL    string         `json:"url"`
		Events []WebhookEvent `json:"events"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	wh, err := h.mgr.UpdateWebhook(c.Param("id"), req.Name, req.URL, req.Events)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": wh})
}

func (h *Handlers) DeleteWebhook(c *gin.Context) {
	if err := h.mgr.DeleteWebhook(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

func (h *Handlers) ListDeliveries(c *gin.Context) {
	limitStr := c.DefaultQuery("limit", "50")
	limit, _ := strconv.Atoi(limitStr)
	deliveries := h.mgr.ListDeliveries(c.Param("id"), limit)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": deliveries, "total": len(deliveries)})
}

func (h *Handlers) RetryDelivery(c *gin.Context) {
	if err := h.mgr.RetryDelivery(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "retrying"})
}

// ==================== 开发者应用 ====================

func (h *Handlers) ListApps(c *gin.Context) {
	ownerID := c.Query("owner_id")
	apps := h.mgr.ListApps(ownerID)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": apps, "total": len(apps)})
}

func (h *Handlers) RegisterApp(c *gin.Context) {
	var req struct {
		Name         string           `json:"name"`
		OwnerID      string           `json:"owner_id"`
		Description  string           `json:"description"`
		RedirectURIs []string         `json:"redirect_uris"`
		GrantTypes   []OAuthGrantType `json:"grant_types"`
		Scopes       []APIScope       `json:"scopes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	app, err := h.mgr.RegisterApp(req.Name, req.OwnerID, req.Description, req.RedirectURIs, req.GrantTypes, req.Scopes)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": app})
}

func (h *Handlers) GetApp(c *gin.Context) {
	app, err := h.mgr.GetApp(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": app})
}

func (h *Handlers) DeleteApp(c *gin.Context) {
	if err := h.mgr.DeleteApp(c.Param("id")); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "message": "deleted"})
}

// ==================== OAuth2 ====================

func (h *Handlers) IssueToken(c *gin.Context) {
	var req struct {
		ClientID     string         `json:"client_id"`
		ClientSecret string         `json:"client_secret"`
		GrantType    OAuthGrantType `json:"grant_type"`
		Scopes       []APIScope     `json:"scopes"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	token, err := h.mgr.IssueToken(req.ClientID, req.ClientSecret, req.GrantType, req.Scopes)
	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": token})
}

// ==================== API文档 ====================

func (h *Handlers) GetAPISpec(c *gin.Context) {
	spec := h.mgr.GetAPISpec()
	c.JSON(http.StatusOK, spec)
}

// ==================== SDK ====================

func (h *Handlers) GenerateSDK(c *gin.Context) {
	lang := SDKLanguage(c.Param("lang"))
	code, err := h.mgr.GenerateSDK(lang)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": gin.H{"language": lang, "code": code}})
}

// ==================== 使用量 ====================

func (h *Handlers) GetUsageStats(c *gin.Context) {
	daysStr := c.DefaultQuery("days", "30")
	days, _ := strconv.Atoi(daysStr)
	stats := h.mgr.GetUsageStats(c.Param("owner_id"), days)
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats, "total": len(stats)})
}

// ==================== 统计 ====================

func (h *Handlers) GetStats(c *gin.Context) {
	stats := h.mgr.GetStats()
	c.JSON(http.StatusOK, gin.H{"code": 0, "data": stats})
}
