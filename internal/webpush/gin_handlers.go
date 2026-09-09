// gin_handlers.go - Web Push 管理端点（Full 构建，挂 admin 组）.
package webpush

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// GinHandler Web Push 管理端点.
type GinHandler struct {
	mgr *Manager
}

// NewGinHandler 创建端点处理器.
func NewGinHandler(m *Manager) *GinHandler {
	return &GinHandler{mgr: m}
}

// RegisterRoutes 注册路由.
//
//	GET  /webpush/status      公钥与订阅数
//	POST /webpush/subscribe   浏览器订阅（PushSubscription.toJSON 原样）
//	POST /webpush/unsubscribe 退订
//	POST /webpush/test        发送测试推送
func (h *GinHandler) RegisterRoutes(rg *gin.RouterGroup) {
	if h == nil || h.mgr == nil || rg == nil {
		return
	}
	g := rg.Group("/webpush")
	{
		g.GET("/status", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"code": 0,
				"data": gin.H{"publicKey": h.mgr.PublicKey(), "subscriptions": h.mgr.Count()},
			})
		})

		g.POST("/subscribe", func(c *gin.Context) {
			var body struct {
				Endpoint string `json:"endpoint" binding:"required"`
				Keys     struct {
					P256dh string `json:"p256dh" binding:"required"`
					Auth   string `json:"auth" binding:"required"`
				} `json:"keys"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "无效的订阅数据"})
				return
			}
			sub := &Subscription{
				Endpoint:  body.Endpoint,
				P256dh:    body.Keys.P256dh,
				Auth:      body.Keys.Auth,
				UserAgent: c.Request.UserAgent(),
			}
			if err := h.mgr.Subscribe(sub); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "订阅成功", "data": gin.H{"subscriptions": h.mgr.Count()}})
		})

		g.POST("/unsubscribe", func(c *gin.Context) {
			var body struct {
				Endpoint string `json:"endpoint" binding:"required"`
			}
			if err := c.ShouldBindJSON(&body); err != nil {
				c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": "无效的请求体"})
				return
			}
			if err := h.mgr.Unsubscribe(body.Endpoint); err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{"code": -1, "message": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "已退订", "data": gin.H{"subscriptions": h.mgr.Count()}})
		})

		g.POST("/test", func(c *gin.Context) {
			var body struct {
				Title string `json:"title"`
				Body  string `json:"body"`
			}
			_ = c.ShouldBindJSON(&body) // 允许空 body，走默认文案
			if body.Title == "" {
				body.Title = "NAS-OS 测试推送"
			}
			if body.Body == "" {
				body.Body = "如果你看到这条通知，浏览器推送通道工作正常。"
			}
			sent, err := h.mgr.Send(c.Request.Context(), body.Title, body.Body, "info", "/")
			if err != nil && sent == 0 {
				c.JSON(http.StatusBadRequest, gin.H{"code": -1, "message": err.Error()})
				return
			}
			c.JSON(http.StatusOK, gin.H{"code": 0, "message": "测试推送已发送", "data": gin.H{"sent": sent}})
		})
	}
}
