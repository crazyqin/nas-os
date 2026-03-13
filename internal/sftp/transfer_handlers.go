package sftp

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// TransferHandlers 传输日志 API 处理器
type TransferHandlers struct {
	logger *TransferLogger
	server *Server
}

// NewTransferHandlers 创建传输日志处理器
func NewTransferHandlers(logger *TransferLogger, server *Server) *TransferHandlers {
	return &TransferHandlers{
		logger: logger,
		server: server,
	}
}

// RegisterRoutes 注册路由
func (h *TransferHandlers) RegisterRoutes(api *gin.RouterGroup) {
	transfers := api.Group("/sftp/transfers")
	{
		transfers.GET("", h.ListTransfers)
		transfers.GET("/stats", h.GetStats)
		transfers.DELETE("", h.ClearLogs)
		transfers.GET("/config", h.GetConfig)
		transfers.PUT("/config", h.UpdateConfig)
	}
}

// ListTransfersRequest 列表请求
type ListTransfersRequest struct {
	Username  string `form:"username"`
	Direction string `form:"direction"`
	Success   string `form:"success"`
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
	Limit     int    `form:"limit"`
	Offset    int    `form:"offset"`
}

// ListTransfers 获取传输日志列表
func (h *TransferHandlers) ListTransfers(c *gin.Context) {
	if h.logger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "传输日志服务未初始化",
		})
		return
	}

	var req ListTransfersRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	filter := &TransferLogFilter{
		Username:  req.Username,
		Direction: req.Direction,
	}

	if req.Success == "true" {
		success := true
		filter.Success = &success
	} else if req.Success == "false" {
		success := false
		filter.Success = &success
	}

	if req.StartTime != "" {
		if t, err := time.Parse(time.RFC3339, req.StartTime); err == nil {
			filter.StartTime = t
		}
	}

	if req.EndTime != "" {
		if t, err := time.Parse(time.RFC3339, req.EndTime); err == nil {
			filter.EndTime = t
		}
	}

	limit := req.Limit
	if limit <= 0 {
		limit = 100
	}
	if limit > 1000 {
		limit = 1000
	}

	logs := h.logger.GetLogs(limit, req.Offset, filter)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"logs":  logs,
			"count": len(logs),
			"limit": limit,
		},
	})
}

// GetStatsRequest 统计请求
type GetStatsRequest struct {
	Period string `form:"period"`
}

// GetStats 获取传输统计
func (h *TransferHandlers) GetStats(c *gin.Context) {
	if h.logger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "传输日志服务未初始化",
		})
		return
	}

	period := c.Query("period")
	duration := parsePeriod(period)

	stats := h.logger.GetStats(duration)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    stats,
	})
}

// ClearLogs 清除传输日志
func (h *TransferHandlers) ClearLogs(c *gin.Context) {
	if h.logger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "传输日志服务未初始化",
		})
		return
	}

	if err := h.logger.Clear(); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "传输日志已清除",
	})
}

// GetConfig 获取日志配置
func (h *TransferHandlers) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"enabled": h.logger.IsEnabled(),
		},
	})
}

// UpdateConfigRequest 更新配置请求
type UpdateConfigRequest struct {
	Enabled bool `json:"enabled"`
}

// UpdateConfig 更新日志配置
func (h *TransferHandlers) UpdateConfig(c *gin.Context) {
	if h.logger == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{
			"code":    503,
			"message": "传输日志服务未初始化",
		})
		return
	}

	var req UpdateConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	h.logger.SetEnabled(req.Enabled)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"enabled": h.logger.IsEnabled(),
		},
	})
}

// parsePeriod 解析周期字符串
func parsePeriod(period string) time.Duration {
	switch period {
	case "1h":
		return time.Hour
	case "24h":
		return 24 * time.Hour
	case "7d":
		return 7 * 24 * time.Hour
	case "30d":
		return 30 * 24 * time.Hour
	default:
		return 24 * time.Hour
	}
}