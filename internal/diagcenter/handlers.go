// Package diagcenter HTTP API 处理器.
package diagcenter

import (
	"net/http"
	"sync/atomic"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handler HTTP API 处理器.
type Handler struct {
	engine  *Engine
	logger  *zap.Logger
	diaging int32 // 原子标志，防止并发诊断
}

// NewHandler 创建 HTTP 处理器.
func NewHandler(engine *Engine, logger *zap.Logger) *Handler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handler{
		engine: engine,
		logger: logger,
	}
}

// RegisterRoutes 注册路由.
func (h *Handler) RegisterRoutes(api *gin.RouterGroup) {
	diagGroup := api.Group("/diag")
	{
		diagGroup.GET("/run", h.runDiag)
		diagGroup.POST("/run", h.runDiag)
		diagGroup.GET("/latest", h.getLatest)
		diagGroup.GET("/history", h.getHistory)
		diagGroup.GET("/status", h.getStatus)
	}
}

// runDiag 执行诊断.
func (h *Handler) runDiag(c *gin.Context) {
	var req RunDiagRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 允许空 body，使用默认值
		req = RunDiagRequest{}
	}

	// 防止并发诊断
	if !atomic.CompareAndSwapInt32(&h.diaging, 0, 1) {
		c.JSON(http.StatusConflict, gin.H{"error": ErrDiagInProgress.Error()})
		return
	}
	defer atomic.StoreInt32(&h.diaging, 0)

	h.logger.Info("开始执行诊断",
		zap.Any("categories", req.Categories),
	)

	result, err := h.engine.RunDiagnostic(c.Request.Context(), req.Categories)
	if err != nil {
		h.logger.Error("诊断失败", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{"error": "诊断失败: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, RunDiagResponse{
		Result:  result,
		Message: "诊断完成",
	})
}

// getLatest 获取最近一次诊断结果.
func (h *Handler) getLatest(c *gin.Context) {
	result := h.engine.GetLatest()
	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": ErrNoDiagData.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// getHistory 获取诊断历史.
func (h *Handler) getHistory(c *gin.Context) {
	var query HistoryQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		query = HistoryQuery{}
	}

	// 默认值
	if query.Days <= 0 {
		query.Days = 30
	}
	if query.Limit <= 0 {
		query.Limit = 100
	}

	response := h.engine.GetHistory(query)
	c.JSON(http.StatusOK, response)
}

// getStatus 获取诊断状态.
func (h *Handler) getStatus(c *gin.Context) {
	latest := h.engine.GetLatest()
	if latest == nil {
		c.JSON(http.StatusOK, gin.H{
			"status":  "unknown",
			"message": "尚未执行诊断",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":     latest.Status,
		"timestamp":  latest.Timestamp,
		"checks":     len(latest.Checks),
		"alerts":     len(latest.Alerts),
		"summary":    latest.Summary,
		"duration_ms": latest.Duration.Milliseconds(),
	})
}
