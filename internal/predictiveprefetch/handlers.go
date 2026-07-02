// Package predictiveprefetch provides HTTP handlers for predictive prefetching.
package predictiveprefetch

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Handler predictive prefetch HTTP handler.
type Handler struct {
	prefetch *PredictivePrefetch
}

// NewHandler creates a new handler.
func NewHandler(prefetch *PredictivePrefetch) *Handler {
	return &Handler{prefetch: prefetch}
}

// RegisterRoutes registers routes.
func (h *Handler) RegisterRoutes(r *gin.RouterGroup) {
	prefetchGroup := r.Group("/predictive-prefetch")
	{
		// Access recording
		prefetchGroup.POST("/access", h.HandleRecordAccess)

		// Prediction
		prefetchGroup.POST("/predict", h.HandlePredict)

		// Prefetch
		prefetchGroup.POST("/prefetch", h.HandlePrefetch)

		// Cache management
		prefetchGroup.GET("/cache", h.HandleGetCached)
		prefetchGroup.DELETE("/cache", h.HandleClearCache)

		// Stats
		prefetchGroup.GET("/stats", h.HandleGetStats)

		// Config
		prefetchGroup.GET("/config", h.HandleGetConfig)
		prefetchGroup.PUT("/config", h.HandleUpdateConfig)

		// Enable/Disable
		prefetchGroup.POST("/enable", h.HandleEnable)
		prefetchGroup.POST("/disable", h.HandleDisable)
	}
}

// HandleRecordAccess records a file access.
func (h *Handler) HandleRecordAccess(c *gin.Context) {
	var req struct {
		UserID   string  `json:"user_id" binding:"required"`
		FilePath string  `json:"file_path" binding:"required"`
		Duration float64 `json:"duration"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.prefetch.RecordAccess(c.Request.Context(), req.UserID, req.FilePath, req.Duration); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "record_failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "访问记录成功",
	})
}

// HandlePredict predicts next files.
func (h *Handler) HandlePredict(c *gin.Context) {
	var req struct {
		UserID      string `json:"user_id" binding:"required"`
		CurrentFile string `json:"current_file" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "请求参数无效: " + err.Error(),
		})
		return
	}

	candidates := h.prefetch.Predict(c.Request.Context(), req.UserID, req.CurrentFile)
	c.JSON(http.StatusOK, gin.H{
		"predictions": candidates,
		"count":       len(candidates),
	})
}

// HandlePrefetch prefetches files.
func (h *Handler) HandlePrefetch(c *gin.Context) {
	var req struct {
		Candidates []PrefetchCandidate `json:"candidates" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.prefetch.Prefetch(c.Request.Context(), req.Candidates); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"error":   "prefetch_failed",
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "预取成功",
		"prefetched": len(req.Candidates),
	})
}

// HandleGetCached returns cached files.
func (h *Handler) HandleGetCached(c *gin.Context) {
	entries := h.prefetch.GetCached()
	c.JSON(http.StatusOK, gin.H{
		"cached": entries,
		"count":  len(entries),
		"size":   h.prefetch.GetCacheSize(),
	})
}

// HandleClearCache clears the cache.
func (h *Handler) HandleClearCache(c *gin.Context) {
	h.prefetch.ClearCache()
	c.JSON(http.StatusOK, gin.H{
		"message": "缓存已清除",
	})
}

// HandleGetStats returns prefetch statistics.
func (h *Handler) HandleGetStats(c *gin.Context) {
	stats := h.prefetch.GetStats()
	c.JSON(http.StatusOK, stats)
}

// HandleGetConfig returns prefetch configuration.
func (h *Handler) HandleGetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, h.prefetch.config)
}

// HandleUpdateConfig updates prefetch configuration.
func (h *Handler) HandleUpdateConfig(c *gin.Context) {
	var config PrefetchConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "invalid_request",
			"message": "请求参数无效: " + err.Error(),
		})
		return
	}

	h.prefetch.mu.Lock()
	h.prefetch.config = config
	h.prefetch.mu.Unlock()

	c.JSON(http.StatusOK, gin.H{
		"message": "配置更新成功",
	})
}

// HandleEnable enables prefetching.
func (h *Handler) HandleEnable(c *gin.Context) {
	h.prefetch.SetEnabled(true)
	c.JSON(http.StatusOK, gin.H{
		"message": "预取已启用",
	})
}

// HandleDisable disables prefetching.
func (h *Handler) HandleDisable(c *gin.Context) {
	h.prefetch.SetEnabled(false)
	c.JSON(http.StatusOK, gin.H{
		"message": "预取已禁用",
	})
}
