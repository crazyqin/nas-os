package ftp

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// TransferHandlers 传输日志 API 处理器.
type TransferHandlers struct {
	logger *TransferLogger
	server *Server
}

// NewTransferHandlers 创建传输日志处理器.
func NewTransferHandlers(logger *TransferLogger, server *Server) *TransferHandlers {
	return &TransferHandlers{
		logger: logger,
		server: server,
	}
}

// RegisterRoutes 注册路由.
func (h *TransferHandlers) RegisterRoutes(api *gin.RouterGroup) {
	transfers := api.Group("/ftp/transfers")
	{
		transfers.GET("", h.ListTransfers)
		transfers.GET("/stats", h.GetStats)
		transfers.DELETE("", h.ClearLogs)
		transfers.GET("/config", h.GetConfig)
		transfers.PUT("/config", h.UpdateConfig)
	}
}

// ListTransfersRequest 列表请求.
type ListTransfersRequest struct {
	Username  string `form:"username"`
	Direction string `form:"direction"` // upload, download
	Success   string `form:"success"`   // true, false, all
	StartTime string `form:"start_time"`
	EndTime   string `form:"end_time"`
	Limit     int    `form:"limit"`
	Offset    int    `form:"offset"`
}

// ListTransfers 获取传输日志列表
// @Summary 获取传输日志列表
// @Description 获取 FTP 传输日志列表，支持过滤
// @Tags ftp
// @Accept json
// @Produce json
// @Param username query string false "用户名过滤"
// @Param direction query string false "传输方向 (upload/download)"
// @Param success query string false "是否成功 (true/false/all)"
// @Param start_time query string false "开始时间 (RFC3339)"
// @Param end_time query string false "结束时间 (RFC3339)"
// @Param limit query int false "返回数量限制"
// @Param offset query int false "偏移量"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ftp/transfers [get].
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

	// 构建过滤条件
	filter := &TransferLogFilter{
		Username:  req.Username,
		Direction: req.Direction,
	}

	switch req.Success {
	case "true":
		success := true
		filter.Success = &success
	case "false":
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

	// 设置默认限制
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

// GetStatsRequest 统计请求.
type GetStatsRequest struct {
	Period string `form:"period"` // 1h, 24h, 7d, 30d
}

// GetStats 获取传输统计
// @Summary 获取传输统计
// @Description 获取 FTP 传输统计信息
// @Tags ftp
// @Accept json
// @Produce json
// @Param period query string false "统计周期 (1h/24h/7d/30d)"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ftp/transfers/stats [get].
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
// @Summary 清除传输日志
// @Description 清除所有 FTP 传输日志
// @Tags ftp
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ftp/transfers [delete].
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
// @Summary 获取日志配置
// @Description 获取传输日志配置
// @Tags ftp
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ftp/transfers/config [get].
func (h *TransferHandlers) GetConfig(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"enabled": h.logger.IsEnabled(),
		},
	})
}

// UpdateConfigRequest 更新配置请求.
type UpdateConfigRequest struct {
	Enabled bool `json:"enabled"`
}

// UpdateConfig 更新日志配置
// @Summary 更新日志配置
// @Description 更新传输日志配置
// @Tags ftp
// @Accept json
// @Produce json
// @Param config body UpdateConfigRequest true "配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /ftp/transfers/config [put].
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

// parsePeriod 解析周期字符串.
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
