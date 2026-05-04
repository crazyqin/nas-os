package aiadvisor

import (
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers AI存储优化顾问HTTP处理器.
type Handlers struct {
	advisor *Advisor
	logger  *zap.Logger
}

// NewHandlers 创建处理器.
func NewHandlers(advisor *Advisor, logger *zap.Logger) *Handlers {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &Handlers{
		advisor: advisor,
		logger:  logger,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	advisorGroup := api.Group("/ai-advisor")
	{
		advisorGroup.POST("/scan", h.startScan)
		advisorGroup.GET("/recommendations", h.getRecommendations)
		advisorGroup.GET("/report", h.getReport)
		advisorGroup.GET("/capacity-forecast", h.getCapacityForecast)
		advisorGroup.POST("/apply/:id", h.applyRecommendation)
	}
}

// startScan 启动存储扫描.
func (h *Handlers) startScan(c *gin.Context) {
	var cfg ScanConfig
	// 使用默认配置，允许部分覆盖
	defaultCfg := DefaultScanConfig()

	if err := c.ShouldBindJSON(&cfg); err != nil {
		// 如果没有body或解析失败，使用默认配置
		cfg = *defaultCfg
	} else {
		// 填充零值字段
		if cfg.RootPath == "" {
			cfg.RootPath = defaultCfg.RootPath
		}
		if cfg.LargeFileThresholdMB == 0 {
			cfg.LargeFileThresholdMB = defaultCfg.LargeFileThresholdMB
		}
		if cfg.StaleDays == 0 {
			cfg.StaleDays = defaultCfg.StaleDays
		}
		if cfg.MaxDepth == 0 {
			cfg.MaxDepth = defaultCfg.MaxDepth
		}
	}

	result, err := h.advisor.Scan(&cfg)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrInvalidPath {
			code = http.StatusBadRequest
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":       "存储扫描完成",
		"scan_result":   result,
	})
}

// getRecommendations 获取优化建议列表.
func (h *Handlers) getRecommendations(c *gin.Context) {
	recs, err := h.advisor.GetRecommendations()
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrNoScanData {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"recommendations": recs,
		"total":           len(recs),
	})
}

// getReport 获取优化报告.
func (h *Handlers) getReport(c *gin.Context) {
	report, err := h.advisor.GetReport()
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrNoScanData {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, report)
}

// getCapacityForecast 获取容量预测.
func (h *Handlers) getCapacityForecast(c *gin.Context) {
	months := 12
	if m, ok := c.GetQuery("months"); ok {
		if parsed, err := strconv.Atoi(m); err == nil && parsed > 0 {
			months = parsed
		}
	}

	forecast, err := h.advisor.GetCapacityForecast(months)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrInsufficientHistory {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, forecast)
}

// applyRecommendation 应用某个建议.
func (h *Handlers) applyRecommendation(c *gin.Context) {
	id := c.Param("id")
	rec, err := h.advisor.ApplyRecommendation(id)
	if err != nil {
		code := http.StatusInternalServerError
		if err == ErrRecommendationNotFound {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":        "建议已应用",
		"recommendation": rec,
	})
}

