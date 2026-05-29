// Package energytracker 提供 REST API 处理器
package energytracker

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// response 标准响应.
type response struct {
	Code    int         `json:"code"`
	Message string      `json:"message"`
	Data    interface{} `json:"data,omitempty"`
}

// Handlers 能耗 API 处理器.
type Handlers struct {
	manager *EnergyManager
}

// NewHandlers 创建处理器.
func NewHandlers(manager *EnergyManager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(r *gin.RouterGroup) {
	et := r.Group("/energytracker")
	{
		et.POST("/track", h.TrackUsage)
		et.GET("/readings", h.GetReadings)
		et.GET("/carbon/:device_id", h.CalculateCarbon)
		et.POST("/report", h.GenerateReport)
		et.GET("/optimize/:device_id", h.SuggestOptimization)
		et.GET("/config", h.GetConfig)
		et.PUT("/config", h.UpdateConfig)
		et.DELETE("/readings", h.ClearReadings)
	}
}

// ========== 追踪接口 ==========

// TrackUsage 记录能耗数据.
func (h *Handlers) TrackUsage(c *gin.Context) {
	var req TrackRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	reading, err := h.manager.TrackUsage(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusCreated, response{Code: 0, Message: "recorded", Data: reading})
}

// GetReadings 获取能耗读数.
func (h *Handlers) GetReadings(c *gin.Context) {
	deviceID := c.Query("device_id")
	limitStr := c.DefaultQuery("limit", "100")
	limit, err := strconv.Atoi(limitStr)
	if err != nil || limit <= 0 {
		limit = 100
	}

	readings := h.manager.GetReadings(deviceID, limit)
	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total":    len(readings),
			"readings": readings,
		},
	})
}

// ========== 碳排放接口 ==========

// CalculateCarbon 计算碳排放.
func (h *Handlers) CalculateCarbon(c *gin.Context) {
	deviceID := c.Param("device_id")
	if deviceID == "" {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "device_id is required"})
		return
	}

	// 解析时间范围
	startStr := c.Query("start")
	endStr := c.Query("end")

	var start, end time.Time
	if startStr != "" {
		t, err := time.Parse(time.RFC3339, startStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid start time format"})
			return
		}
		start = t
	} else {
		start = time.Now().Add(-24 * time.Hour)
	}

	if endStr != "" {
		t, err := time.Parse(time.RFC3339, endStr)
		if err != nil {
			c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid end time format"})
			return
		}
		end = t
	} else {
		end = time.Now()
	}

	footprint, err := h.manager.CalculateCarbon(deviceID, start, end)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: footprint})
}

// ========== 报告接口 ==========

// GenerateReport 生成能源报告.
func (h *Handlers) GenerateReport(c *gin.Context) {
	var req ReportRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	report, err := h.manager.GenerateReport(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: report})
}

// ========== 优化接口 ==========

// SuggestOptimization 生成节能建议.
func (h *Handlers) SuggestOptimization(c *gin.Context) {
	deviceID := c.Param("device_id")

	tips, err := h.manager.SuggestOptimization(deviceID)
	if err != nil {
		c.JSON(http.StatusNotFound, response{Code: 1, Message: err.Error()})
		return
	}

	c.JSON(http.StatusOK, response{
		Code:    0,
		Message: "success",
		Data: gin.H{
			"total": len(tips),
			"tips":  tips,
		},
	})
}

// ========== 配置接口 ==========

// GetConfig 获取配置.
func (h *Handlers) GetConfig(c *gin.Context) {
	config := h.manager.GetConfig()
	c.JSON(http.StatusOK, response{Code: 0, Message: "success", Data: config})
}

// UpdateConfig 更新配置.
func (h *Handlers) UpdateConfig(c *gin.Context) {
	var config PowerConfig
	if err := c.ShouldBindJSON(&config); err != nil {
		c.JSON(http.StatusBadRequest, response{Code: 1, Message: "invalid request: " + err.Error()})
		return
	}

	h.manager.UpdateConfig(config)
	c.JSON(http.StatusOK, response{Code: 0, Message: "config updated"})
}

// ClearReadings 清除读数.
func (h *Handlers) ClearReadings(c *gin.Context) {
	h.manager.ClearReadings()
	c.JSON(http.StatusOK, response{Code: 0, Message: "readings cleared"})
}
