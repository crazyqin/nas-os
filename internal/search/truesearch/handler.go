package truesearch

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// APIHandler TrueSearch REST API handler。
type APIHandler struct {
	engine *Engine
	logger *zap.Logger
}

// NewAPIHandler 创建 API handler。
func NewAPIHandler(engine *Engine, logger *zap.Logger) *APIHandler {
	return &APIHandler{
		engine: engine,
		logger: logger,
	}
}

// RegisterRoutes 注册路由。
func (h *APIHandler) RegisterRoutes(r *gin.RouterGroup) {
	ts := r.Group("/truesearch")
	{
		ts.POST("", h.search)
		ts.GET("/status", h.status)
		ts.POST("/reindex", h.reindex)
		ts.POST("/index", h.indexFiles)
		ts.POST("/index/dir", h.indexDirectory)
	}
}

// search 全文搜索。
func (h *APIHandler) search(c *gin.Context) {
	var req SearchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	if req.Query == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "query 不能为空",
		})
		return
	}

	resp, err := h.engine.Search(req)
	if err != nil {
		h.logger.Error("search failed", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": "搜索失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    resp,
	})
}

// status 获取索引状态。
func (h *APIHandler) status(c *gin.Context) {
	status := h.engine.Status()
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    status,
	})
}

// reindexRequest 重建索引请求。
type reindexRequest struct {
	Path  string `json:"path"`
	Force bool   `json:"force"`
}

// reindex 重建索引。
func (h *APIHandler) reindex(c *gin.Context) {
	var req reindexRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	go func() {
		if err := h.engine.Reindex(req.Path, req.Force); err != nil {
			h.logger.Error("reindex failed", zap.Error(err))
		}
	}()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "重建索引任务已启动",
	})
}

// indexFilesRequest 索引文件请求。
type indexFilesRequest struct {
	Paths []string `json:"paths" binding:"required"`
}

// indexFiles 索引指定文件。
func (h *APIHandler) indexFiles(c *gin.Context) {
	var req indexFilesRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	success := 0
	failed := 0
	var errors []string

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

// indexDirRequest 索引目录请求。
type indexDirRequest struct {
	Path string `json:"path" binding:"required"`
}

// indexDirectory 递归索引目录。
func (h *APIHandler) indexDirectory(c *gin.Context) {
	var req indexDirRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "无效的请求参数: " + err.Error(),
		})
		return
	}

	go func() {
		if err := h.engine.IndexDirectory(req.Path); err != nil {
			h.logger.Error("index directory failed", zap.Error(err))
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
