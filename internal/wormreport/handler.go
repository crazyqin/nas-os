package wormreport

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handler WORM HTTP处理器
type Handler struct {
	manager *WORMManager
}

// NewHandler 创建处理器
func NewHandler(manager *WORMManager) *Handler {
	return &Handler{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	worm := rg.Group("/worm")
	{
		worm.POST("/lock", h.Lock)
		worm.GET("", h.List)
		worm.GET("/:id", h.Get)
		worm.POST("/:id/verify", h.Verify)
		worm.POST("/verify-all", h.VerifyAll)
		worm.GET("/report", h.Report)
		worm.POST("/expire", h.ExpireExpired)
	}
}

// LockRequest 锁定请求
type LockRequest struct {
	FilePath  string  `json:"filePath" binding:"required"`
	FileSize  int64   `json:"fileSize"`
	Retention string  `json:"retention" binding:"required"`
	LockedBy  string  `json:"lockedBy"`
	DurationDays *int `json:"durationDays,omitempty"`
}

// Lock 锁定文件
func (h *Handler) Lock(c *gin.Context) {
	var req LockRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	var duration *time.Duration
	if req.DurationDays != nil {
		d := time.Duration(*req.DurationDays) * 24 * time.Hour
		duration = &d
	}
	file, err := h.manager.Lock(req.FilePath, req.FileSize, RetentionLevel(req.Retention), req.LockedBy, duration)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, file)
}

// List 列出文件
func (h *Handler) List(c *gin.Context) {
	status := WORMStatus(c.Query("status"))
	retention := RetentionLevel(c.Query("retention"))
	files := h.manager.List(status, retention)
	c.JSON(http.StatusOK, files)
}

// Get 获取文件
func (h *Handler) Get(c *gin.Context) {
	id := c.Param("id")
	file, ok := h.manager.Get(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "file not found"})
		return
	}
	c.JSON(http.StatusOK, file)
}

// Verify 验证文件
func (h *Handler) Verify(c *gin.Context) {
	id := c.Param("id")
	ok, err := h.manager.Verify(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"integrityOk": ok})
}

// VerifyAll 验证所有
func (h *Handler) VerifyAll(c *gin.Context) {
	total, passed, _ := h.manager.VerifyAll()
	c.JSON(http.StatusOK, gin.H{"total": total, "passed": passed, "failed": total - passed})
}

// Report 生成报告
func (h *Handler) Report(c *gin.Context) {
	reportType := c.DefaultQuery("type", "ad-hoc")
	report := h.manager.GenerateReport(reportType)
	c.JSON(http.StatusOK, report)
}

// ExpireExpired 过期到期文件
func (h *Handler) ExpireExpired(c *gin.Context) {
	count := h.manager.ExpireExpired()
	c.JSON(http.StatusOK, gin.H{"expiredCount": count})
}
