package fasttransfer

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 高速传输 HTTP 处理器
type Handlers struct {
	mgr *TransferManager
}

// NewHandlers 创建处理器
func NewHandlers(mgr *TransferManager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	xfer := api.Group("/fasttransfer")
	{
		xfer.POST("/transfers", h.CreateTransfer)
		xfer.GET("/transfers", h.ListTransfers)
		xfer.GET("/transfers/:id", h.GetTransfer)
		xfer.POST("/transfers/:id/cancel", h.CancelTransfer)
		xfer.GET("/stats", h.GetStats)
	}
}

func (h *Handlers) CreateTransfer(c *gin.Context) {
	var req struct {
		Name       string `json:"name" binding:"required"`
		SourcePath string `json:"source_path" binding:"required"`
		DestPath   string `json:"dest_path" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	t, err := h.mgr.CreateTransfer(req.Name, req.SourcePath, req.DestPath)
	if err != nil {
		c.JSON(http.StatusTooManyRequests, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusCreated, t)
}

func (h *Handlers) ListTransfers(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.ListTransfers())
}

func (h *Handlers) GetTransfer(c *gin.Context) {
	t, err := h.mgr.GetTransfer(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, t)
}

func (h *Handlers) CancelTransfer(c *gin.Context) {
	if err := h.mgr.CancelTransfer(c.Param("id")); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, gin.H{"status": "cancelled"})
}

func (h *Handlers) GetStats(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.GetStats())
}
