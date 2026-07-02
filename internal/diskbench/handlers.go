package diskbench

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 磁盘性能测试 HTTP 处理器.
type Handlers struct {
	mgr *BenchmarkManager
}

// NewHandlers 创建处理器.
func NewHandlers(mgr *BenchmarkManager) *Handlers {
	return &Handlers{mgr: mgr}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	bench := api.Group("/diskbench")
	{
		bench.POST("/run", h.RunBenchmark)
		bench.GET("/results", h.ListResults)
		bench.GET("/results/:id", h.GetResult)
	}
}

func (h *Handlers) RunBenchmark(c *gin.Context) {
	var req struct {
		TargetPath string `json:"target_path" binding:"required"`
		FileSizeMB int    `json:"file_size_mb"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	result, err := h.mgr.RunBenchmark(req.TargetPath, req.FileSizeMB)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusAccepted, result)
}

func (h *Handlers) ListResults(c *gin.Context) {
	c.JSON(http.StatusOK, h.mgr.ListResults())
}

func (h *Handlers) GetResult(c *gin.Context) {
	result, err := h.mgr.GetResult(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, result)
}
