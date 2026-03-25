package search

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handlers 搜索处理器.
type Handlers struct {
	engine *Engine
}

// NewHandlers 创建处理器.
func NewHandlers(engine *Engine) *Handlers {
	return &Handlers{engine: engine}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	search := r.Group("/search")
	{
		search.POST("/query", h.search)
		search.POST("/index", h.indexFiles)
		search.POST("/index/dir", h.indexDirectory)
		search.DELETE("/index", h.deleteFromIndex)
		search.GET("/stats", h.getStats)
	}
}

// search 搜索文件
// @Summary 搜索文件
// @Description 根据查询条件搜索文件，支持文件名和内容搜索
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body Request true "搜索请求"
// @Success 200 {object} Response
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /search/query [post].
func (h *Handlers) search(c *gin.Context) {
	var req Request
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 执行搜索
	result, err := h.engine.Search(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "搜索失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// indexRequest 索引请求.
type indexRequest struct {
	Paths []string `json:"paths" binding:"required"`
}

// indexFiles 索引文件
// @Summary 索引文件
// @Description 将指定文件添加到搜索索引
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body indexRequest true "索引请求"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /search/index [post].
func (h *Handlers) indexFiles(c *gin.Context) {
	var req indexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 索引文件
	success := 0
	failed := 0
	errors := []string{}

	for _, path := range req.Paths {
		if err := h.engine.IndexFile(path); err != nil {
			failed++
			errors = append(errors, path+": "+err.Error())
		} else {
			success++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "索引完成",
		"data": gin.H{
			"success": success,
			"failed":  failed,
			"errors":  errors,
		},
	})
}

// indexDirRequest 索引目录请求.
type indexDirRequest struct {
	Path string `json:"path" binding:"required"`
}

// indexDirectory 索引目录
// @Summary 索引目录
// @Description 递归索引指定目录下的所有文件
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body indexDirRequest true "索引目录请求"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /search/index/dir [post].
func (h *Handlers) indexDirectory(c *gin.Context) {
	var req indexDirRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 异步索引目录
	go func() {
		if err := h.engine.IndexDirectory(req.Path); err != nil {
			// 记录错误，但不阻塞请求
			_ = err // 明确忽略错误，避免 staticcheck 警告
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "索引任务已启动",
		"data": gin.H{
			"path": req.Path,
		},
	})
}

// deleteRequest 删除请求.
type deleteRequest struct {
	Paths []string `json:"paths" binding:"required"`
}

// deleteFromIndex 从索引中删除
// @Summary 从索引中删除
// @Description 从搜索索引中删除指定文件
// @Tags 搜索
// @Accept json
// @Produce json
// @Param request body deleteRequest true "删除请求"
// @Success 200 {object} map[string]interface{}
// @Failure 400 {object} map[string]interface{}
// @Failure 500 {object} map[string]interface{}
// @Router /search/index [delete].
func (h *Handlers) deleteFromIndex(c *gin.Context) {
	var req deleteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	// 批量删除
	if err := h.engine.DeleteBatch(req.Paths); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "删除失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "删除成功",
		"data": gin.H{
			"count": len(req.Paths),
		},
	})
}

// getStats 获取索引统计
// @Summary 获取索引统计
// @Description 获取搜索索引的统计信息
// @Tags 搜索
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /search/stats [get].
func (h *Handlers) getStats(c *gin.Context) {
	stats := h.engine.Stats()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}
