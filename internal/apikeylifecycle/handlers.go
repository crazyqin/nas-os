// Package apikeylifecycle 提供API密钥生命周期的HTTP处理器
package apikeylifecycle

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers API密钥生命周期HTTP处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	keyGroup := api.Group("/api-keys")
	{
		// 密钥管理
		keyGroup.POST("", h.createKey)
		keyGroup.GET("", h.listKeys)
		keyGroup.GET("/:id", h.getKey)
		keyGroup.DELETE("/:id", h.revokeKey)
		keyGroup.POST("/:id/rotate", h.rotateKey)

		// 权限管理
		keyGroup.PUT("/:id/permissions", h.updatePermissions)
		keyGroup.PUT("/:id/expiration", h.setExpiration)

		// 轮换策略
		keyGroup.GET("/rotation-policy", h.getRotationPolicy)
		keyGroup.PUT("/rotation-policy", h.updateRotationPolicy)
		keyGroup.GET("/rotation-check", h.checkRotation)

		// 过期管理
		keyGroup.POST("/expire-check", h.expireKeys)
		keyGroup.GET("/expiring", h.getExpiringKeys)

		// 审计
		keyGroup.GET("/audit", h.getAuditLog)

		// 统计
		keyGroup.GET("/stats", h.getStats)
	}
}

// createKey 创建密钥
func (h *Handlers) createKey(c *gin.Context) {
	var key APIKey
	if err := c.ShouldBindJSON(&key); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	fullKey, err := h.manager.CreateKey(&key)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "API密钥创建成功",
		"api_key": gin.H{
			"id":         key.ID,
			"name":       key.Name,
			"key":        fullKey, // 只在创建时返回完整密钥
			"key_prefix": key.KeyPrefix,
			"user_id":    key.UserID,
			"expires_at": key.ExpiresAt,
		},
		"warning": "请妥善保管密钥，此密钥不会再次显示",
	})
}

// listKeys 列出密钥
func (h *Handlers) listKeys(c *gin.Context) {
	userID := c.Query("user_id")
	keys := h.manager.ListKeys(userID)
	c.JSON(http.StatusOK, gin.H{
		"api_keys": keys,
		"total":    len(keys),
	})
}

// getKey 获取密钥
func (h *Handlers) getKey(c *gin.Context) {
	id := c.Param("id")
	key, err := h.manager.GetKey(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, key)
}

// revokeKey 撤销密钥
func (h *Handlers) revokeKey(c *gin.Context) {
	id := c.Param("id")
	userID := c.Query("user_id")

	if err := h.manager.RevokeKey(id, userID); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "API密钥已撤销"})
}

// rotateKey 轮换密钥
func (h *Handlers) rotateKey(c *gin.Context) {
	id := c.Param("id")

	fullKey, err := h.manager.RotateKey(id)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "API密钥轮换成功",
		"api_key": fullKey,
		"warning": "请妥善保管新密钥，此密钥不会再次显示",
	})
}

// updatePermissions 更新权限
func (h *Handlers) updatePermissions(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		Permissions []Permission `json:"permissions" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.UpdateKeyPermissions(id, req.Permissions); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "权限更新成功"})
}

// setExpiration 设置过期时间
func (h *Handlers) setExpiration(c *gin.Context) {
	id := c.Param("id")
	var req struct {
		ExpiresAt time.Time `json:"expires_at" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.SetKeyExpiration(id, req.ExpiresAt); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "过期时间设置成功"})
}

// getRotationPolicy 获取轮换策略
func (h *Handlers) getRotationPolicy(c *gin.Context) {
	policy := h.manager.GetRotationPolicy()
	c.JSON(http.StatusOK, policy)
}

// updateRotationPolicy 更新轮换策略
func (h *Handlers) updateRotationPolicy(c *gin.Context) {
	var policy RotationPolicy
	if err := c.ShouldBindJSON(&policy); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	h.manager.UpdateRotationPolicy(&policy)
	c.JSON(http.StatusOK, gin.H{"message": "轮换策略更新成功"})
}

// checkRotation 检查轮换
func (h *Handlers) checkRotation(c *gin.Context) {
	keys := h.manager.CheckRotation()
	c.JSON(http.StatusOK, gin.H{
		"need_rotation": keys,
		"total":         len(keys),
	})
}

// expireKeys 过期密钥
func (h *Handlers) expireKeys(c *gin.Context) {
	expired := h.manager.ExpireKeys()
	c.JSON(http.StatusOK, gin.H{
		"message":       "过期检查完成",
		"expired_count": expired,
	})
}

// getExpiringKeys 获取即将过期的密钥
func (h *Handlers) getExpiringKeys(c *gin.Context) {
	days := 7
	keys := h.manager.GetExpiringKeys(days)
	c.JSON(http.StatusOK, gin.H{
		"expiring_keys": keys,
		"total":         len(keys),
		"days":          days,
	})
}

// getAuditLog 获取审计日志
func (h *Handlers) getAuditLog(c *gin.Context) {
	keyID := c.Query("key_id")
	logs := h.manager.GetAuditLog(keyID, 50)
	c.JSON(http.StatusOK, gin.H{
		"audit_log": logs,
		"total":     len(logs),
	})
}

// getStats 获取统计
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, stats)
}
