package apikey

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// Handler HTTP API处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建Handler
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	ak := rg.Group("/apikey")
	{
		// 密钥管理
		ak.POST("/keys", h.CreateKey)
		ak.GET("/keys", h.ListKeys)
		ak.GET("/keys/:id", h.GetKey)
		ak.PUT("/keys/:id", h.UpdateKey)
		ak.DELETE("/keys/:id", h.DeleteKey)
		
		// 密钥操作
		ak.POST("/keys/:id/revoke", h.RevokeKey)
		ak.POST("/validate", h.ValidateKey)
		
		// 用户密钥
		ak.GET("/users/:userId/keys", h.GetUserKeys)
		ak.GET("/users/:userId/stats", h.GetUserStats)
		
		// 统计
		ak.GET("/stats", h.GetStats)
		
		// 审计日志
		ak.GET("/audit", h.GetAuditLogs)
		ak.GET("/keys/:id/audit", h.GetKeyAuditLogs)
		
		// 权限和作用域
		ak.GET("/permissions", h.GetPermissions)
		ak.GET("/scopes", h.GetScopes)
		
		// 清理
		ak.POST("/cleanup", h.CleanupExpiredKeys)
	}
}

// CreateKey 创建API密钥
func (h *Handler) CreateKey(c *gin.Context) {
	var req CreateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "参数错误: " + err.Error(),
		})
		return
	}
	
	key, err := h.manager.CreateKey(&req)
	if err != nil {
		c.JSON(http.StatusConflict, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusCreated, APIResponse{
		Success: true,
		Data:    key,
		Message: "API密钥创建成功，请妥善保管密钥",
	})
}

// GetKey 获取密钥详情
func (h *Handler) GetKey(c *gin.Context) {
	keyID := c.Param("id")
	
	key, err := h.manager.GetKey(keyID)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    key,
	})
}

// ListKeys 列出密钥
func (h *Handler) ListKeys(c *gin.Context) {
	var req ListKeysRequest
	
	// 从查询参数解析
	req.UserID = c.Query("user_id")
	req.Status = KeyStatus(c.Query("status"))
	req.Page, _ = strconv.Atoi(c.DefaultQuery("page", "1"))
	req.PageSize, _ = strconv.Atoi(c.DefaultQuery("page_size", "20"))
	
	result := h.manager.ListKeys(&req)
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    result,
	})
}

// UpdateKey 更新密钥
func (h *Handler) UpdateKey(c *gin.Context) {
	keyID := c.Param("id")
	
	var req UpdateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "参数错误: " + err.Error(),
		})
		return
	}
	
	key, err := h.manager.UpdateKey(keyID, &req)
	if err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    key,
	})
}

// DeleteKey 删除密钥
func (h *Handler) DeleteKey(c *gin.Context) {
	keyID := c.Param("id")
	
	if err := h.manager.DeleteKey(keyID); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "密钥已删除",
	})
}

// RevokeKey 撤销密钥
func (h *Handler) RevokeKey(c *gin.Context) {
	keyID := c.Param("id")
	
	var req RevokeKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许没有body的请求
		req.Reason = "手动撤销"
	}
	
	// 从JWT或session获取当前用户
	revokedBy := c.GetString("user_id")
	if revokedBy == "" {
		revokedBy = "system"
	}
	
	if err := h.manager.RevokeKey(keyID, revokedBy, req.Reason); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Message: "密钥已撤销",
	})
}

// ValidateKey 验证API密钥
func (h *Handler) ValidateKey(c *gin.Context) {
	var req ValidateKeyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{
			Success: false,
			Error:   "参数错误: " + err.Error(),
		})
		return
	}
	
	result := h.manager.ValidateKey(&req)
	
	if result.Valid {
		c.JSON(http.StatusOK, APIResponse{
			Success: true,
			Data:    result,
		})
	} else {
		c.JSON(http.StatusUnauthorized, APIResponse{
			Success: false,
			Error:   result.Error,
			Data:    result,
		})
	}
}

// GetUserKeys 获取用户的密钥
func (h *Handler) GetUserKeys(c *gin.Context) {
	userID := c.Param("userId")
	
	keys := h.manager.GetUserKeys(userID)
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    keys,
	})
}

// GetUserStats 获取用户统计
func (h *Handler) GetUserStats(c *gin.Context) {
	userID := c.Param("userId")
	
	stats := h.manager.GetUserStats(userID)
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    stats,
	})
}

// GetStats 获取统计信息
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    stats,
	})
}

// GetAuditLogs 获取审计日志
func (h *Handler) GetAuditLogs(c *gin.Context) {
	keyID := c.Query("key_id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "100"))
	
	logs := h.manager.GetAuditLogs(keyID, limit)
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    logs,
	})
}

// GetKeyAuditLogs 获取密钥审计日志
func (h *Handler) GetKeyAuditLogs(c *gin.Context) {
	keyID := c.Param("id")
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "50"))
	
	logs := h.manager.GetAuditLogs(keyID, limit)
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    logs,
	})
}

// GetPermissions 获取权限列表
func (h *Handler) GetPermissions(c *gin.Context) {
	permissions := h.manager.GetPermissions()
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    permissions,
	})
}

// GetScopes 获取作用域列表
func (h *Handler) GetScopes(c *gin.Context) {
	scopes := h.manager.GetScopes()
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data:    scopes,
	})
}

// CleanupExpiredKeys 清理过期密钥
func (h *Handler) CleanupExpiredKeys(c *gin.Context) {
	count := h.manager.CleanupExpiredKeys()
	
	c.JSON(http.StatusOK, APIResponse{
		Success: true,
		Data: map[string]int{
			"cleaned": count,
		},
		Message: "清理完成",
	})
}