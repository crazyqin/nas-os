package vault

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler 保险库 HTTP 处理器。
type Handler struct {
	manager *Manager
	logger  *zap.Logger
}

// NewHandler 创建保险库 HTTP 处理器实例。
func NewHandler(manager *Manager, logger *zap.Logger) *Handler {
	return &Handler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterRoutes 注册保险库相关的 HTTP 路由。
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	vaults := rg.Group("/vaults")
	{
		vaults.POST("", h.CreateVault)
		vaults.GET("", h.ListVaults)
		vaults.GET("/stats", h.GetStats)
		vaults.GET("/:id", h.GetVault)
		vaults.POST("/:id/unlock", h.UnlockVault)
		vaults.POST("/:id/lock", h.LockVault)
		vaults.DELETE("/:id", h.DeleteVault)
	}
}

// createVaultRequest 创建保险库请求体。
type createVaultRequest struct {
	Name        string `json:"name" binding:"required"`
	Description string `json:"description"`
	MountPath   string `json:"mount_path" binding:"required"`
	Algorithm   string `json:"algorithm"`
	Passphrase  string `json:"passphrase"`
}

// unlockVaultRequest 解锁保险库请求体。
type unlockVaultRequest struct {
	Passphrase string `json:"passphrase" binding:"required"`
}

// apiResponse 统一 API 响应结构。
type apiResponse struct {
	Success bool        `json:"success"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// CreateVault 创建保险库。
// POST /api/v1/vaults
func (h *Handler) CreateVault(c *gin.Context) {
	var req createVaultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			Success: false,
			Error:   "请求参数错误: " + err.Error(),
		})
		return
	}

	vault, err := h.manager.CreateVault(req.Name, req.Description, req.MountPath, req.Algorithm, req.Passphrase)
	if err != nil {
		status := http.StatusInternalServerError
		if ve, ok := err.(*VaultError); ok {
			switch ve.Code {
			case "VAULT_ALREADY_EXISTS":
				status = http.StatusConflict
			case "INVALID_ALGORITHM", "INVALID_NAME", "INVALID_PATH":
				status = http.StatusBadRequest
			}
		}
		c.JSON(status, apiResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	h.logger.Info("HTTP: 创建保险库", zap.String("id", vault.ID), zap.String("name", vault.Name))
	c.JSON(http.StatusCreated, apiResponse{
		Success: true,
		Data:    vault,
	})
}

// ListVaults 列出所有保险库。
// GET /api/v1/vaults
func (h *Handler) ListVaults(c *gin.Context) {
	vaults := h.manager.ListVaults()
	c.JSON(http.StatusOK, apiResponse{
		Success: true,
		Data:    vaults,
	})
}

// GetVault 获取保险库详情。
// GET /api/v1/vaults/:id
func (h *Handler) GetVault(c *gin.Context) {
	id := c.Param("id")

	vault, err := h.manager.GetVault(id)
	if err != nil {
		status := http.StatusInternalServerError
		if ve, ok := err.(*VaultError); ok && ve.Code == "VAULT_NOT_FOUND" {
			status = http.StatusNotFound
		}
		c.JSON(status, apiResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, apiResponse{
		Success: true,
		Data:    vault,
	})
}

// UnlockVault 解锁保险库。
// POST /api/v1/vaults/:id/unlock
func (h *Handler) UnlockVault(c *gin.Context) {
	id := c.Param("id")

	var req unlockVaultRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, apiResponse{
			Success: false,
			Error:   "请求参数错误: " + err.Error(),
		})
		return
	}

	err := h.manager.UnlockVault(id, req.Passphrase)
	if err != nil {
		status := http.StatusInternalServerError
		if ve, ok := err.(*VaultError); ok {
			switch ve.Code {
			case "VAULT_NOT_FOUND":
				status = http.StatusNotFound
			case "INVALID_PASSPHRASE":
				status = http.StatusUnauthorized
			case "MAX_ATTEMPTS_EXCEEDED":
				status = http.StatusTooManyRequests
			case "VAULT_ALREADY_UNLOCKED":
				status = http.StatusConflict
			}
		}
		c.JSON(status, apiResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	h.logger.Info("HTTP: 解锁保险库", zap.String("id", id))
	c.JSON(http.StatusOK, apiResponse{
		Success: true,
		Data:    gin.H{"message": "保险库已解锁"},
	})
}

// LockVault 锁定保险库。
// POST /api/v1/vaults/:id/lock
func (h *Handler) LockVault(c *gin.Context) {
	id := c.Param("id")

	err := h.manager.LockVault(id)
	if err != nil {
		status := http.StatusInternalServerError
		if ve, ok := err.(*VaultError); ok {
			switch ve.Code {
			case "VAULT_NOT_FOUND":
				status = http.StatusNotFound
			case "VAULT_LOCKED":
				status = http.StatusConflict
			}
		}
		c.JSON(status, apiResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	h.logger.Info("HTTP: 锁定保险库", zap.String("id", id))
	c.JSON(http.StatusOK, apiResponse{
		Success: true,
		Data:    gin.H{"message": "保险库已锁定"},
	})
}

// DeleteVault 删除保险库。
// DELETE /api/v1/vaults/:id
func (h *Handler) DeleteVault(c *gin.Context) {
	id := c.Param("id")

	err := h.manager.DeleteVault(id)
	if err != nil {
		status := http.StatusInternalServerError
		if ve, ok := err.(*VaultError); ok {
			switch ve.Code {
			case "VAULT_NOT_FOUND":
				status = http.StatusNotFound
			case "VAULT_UNLOCKED":
				status = http.StatusConflict
			}
		}
		c.JSON(status, apiResponse{
			Success: false,
			Error:   err.Error(),
		})
		return
	}

	h.logger.Info("HTTP: 删除保险库", zap.String("id", id))
	c.JSON(http.StatusOK, apiResponse{
		Success: true,
		Data:    gin.H{"message": "保险库已删除"},
	})
}

// GetStats 获取保险库统计信息。
// GET /api/v1/vaults/stats
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats()
	c.JSON(http.StatusOK, apiResponse{
		Success: true,
		Data:    stats,
	})
}
