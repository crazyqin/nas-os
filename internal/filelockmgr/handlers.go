package filelockmgr

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler HTTP API 处理器
type Handler struct {
	manager *Manager
}

// NewHandler 创建 HTTP 处理器
func NewHandler(manager *Manager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	lockGroup := r.Group("/filelocks")
	{
		lockGroup.POST("", h.AcquireLock)
		lockGroup.DELETE("/:id", h.ReleaseLock)
		lockGroup.POST("/:id/upgrade", h.UpgradeLock)
		lockGroup.POST("/:id/refresh", h.RefreshLock)
		lockGroup.GET("/file/*path", h.GetLocksByFile)
		lockGroup.GET("/user/:id", h.GetLocksByUser)
		lockGroup.GET("/stats", h.GetStats)
		lockGroup.POST("/cleanup", h.CleanupExpired)
	}
}

// AcquireLock 获取锁
// POST /api/v1/filelocks
func (h *Handler) AcquireLock(c *gin.Context) {
	var req LockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	lock, err := h.manager.AcquireLock(c.Request.Context(), req)
	if err != nil {
		switch err {
		case ErrLockConflict:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case ErrMaxLocksExceeded:
			c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		case ErrDuplicateLock:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusCreated, lock)
}

// ReleaseLock 释放锁
// DELETE /api/v1/filelocks/:id
func (h *Handler) ReleaseLock(c *gin.Context) {
	lockID := c.Param("id")

	err := h.manager.ReleaseLock(c.Request.Context(), lockID)
	if err != nil {
		switch err {
		case ErrLockNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "锁已释放"})
}

// UpgradeLock 升级锁
// POST /api/v1/filelocks/:id/upgrade
func (h *Handler) UpgradeLock(c *gin.Context) {
	lockID := c.Param("id")

	err := h.manager.UpgradeLock(c.Request.Context(), lockID)
	if err != nil {
		switch err {
		case ErrLockNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case ErrLockNotShared:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		case ErrLockConflict:
			c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		case ErrUpgradeNotAllowed:
			c.JSON(http.StatusForbidden, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "锁已升级为独占锁"})
}

// RefreshLock 续期锁
// POST /api/v1/filelocks/:id/refresh
func (h *Handler) RefreshLock(c *gin.Context) {
	lockID := c.Param("id")

	var req RefreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数错误: " + err.Error()})
		return
	}

	duration := time.Duration(req.Duration) * time.Second
	err := h.manager.RefreshLock(c.Request.Context(), lockID, duration)
	if err != nil {
		switch err {
		case ErrLockNotFound:
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		case ErrLockExpired:
			c.JSON(http.StatusGone, gin.H{"error": err.Error()})
		case ErrInvalidDuration:
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		default:
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		}
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "锁已续期"})
}

// GetLocksByFile 获取文件的所有锁
// GET /api/v1/filelocks/file/:path
func (h *Handler) GetLocksByFile(c *gin.Context) {
	filePath := c.Param("path")
	if filePath == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "文件路径不能为空"})
		return
	}

	locks := h.manager.GetLocksByFile(c.Request.Context(), filePath)
	if locks == nil {
		locks = []FileLockEntry{}
	}

	c.JSON(http.StatusOK, locks)
}

// GetLocksByUser 获取用户的所有锁
// GET /api/v1/filelocks/user/:id
func (h *Handler) GetLocksByUser(c *gin.Context) {
	userID := c.Param("id")

	locks := h.manager.GetLocksByUser(c.Request.Context(), userID)
	if locks == nil {
		locks = []FileLockEntry{}
	}

	c.JSON(http.StatusOK, locks)
}

// GetStats 获取锁统计
// GET /api/v1/filelocks/stats
func (h *Handler) GetStats(c *gin.Context) {
	stats := h.manager.GetStats(c.Request.Context())
	c.JSON(http.StatusOK, stats)
}

// CleanupExpired 清理过期锁
// POST /api/v1/filelocks/cleanup
func (h *Handler) CleanupExpired(c *gin.Context) {
	count := h.manager.CleanupExpired(c.Request.Context())
	c.JSON(http.StatusOK, gin.H{"cleaned": count})
}
