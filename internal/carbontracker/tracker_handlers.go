// Package carbontracker - 碳足迹追踪 REST API 处理器
package carbontracker

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// TrackerHandler 碳足迹追踪 API 处理器.
type TrackerHandler struct {
	manager *TrackerManager
	logger  *zap.Logger
}

// NewTrackerHandler 创建碳足迹追踪处理器.
func NewTrackerHandler(manager *TrackerManager, logger *zap.Logger) *TrackerHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TrackerHandler{
		manager: manager,
		logger:  logger,
	}
}

// RegisterTrackerRoutes 注册追踪路由.
func (h *TrackerHandler) RegisterTrackerRoutes(rg *gin.RouterGroup) {
	carbon := rg.Group("/carbon")
	{
		// 碳足迹计算
		carbon.POST("/footprint", h.calculateFootprint)

		// 绿色评分
		carbon.GET("/score/:deviceID", h.getGreenScore)

		// 能源来源管理
		carbon.GET("/energy-sources", h.getEnergySources)
		carbon.POST("/energy-source", h.setEnergySource)

		// 历史记录
		carbon.GET("/history/:deviceID", h.getHistory)

		// 排放记录
		carbon.GET("/emissions", h.getEmissions)

		// 设备汇总
		carbon.GET("/summary/:deviceID", h.getDeviceSummary)

		// 仪表盘
		carbon.GET("/dashboard", h.getDashboard)
	}
}

// calculateFootprint 计算碳足迹
// POST /api/v1/carbon/footprint.
func (h *TrackerHandler) calculateFootprint(c *gin.Context) {
	var req struct {
		DeviceID   string  `json:"device_id" binding:"required"`
		DeviceName string  `json:"device_name"`
		CPUWatts   float64 `json:"cpu_watts"`
		DiskWatts  float64 `json:"disk_watts"`
		NetWatts   float64 `json:"net_watts"`
		MemWatts   float64 `json:"mem_watts"`
		GPUWatts   float64 `json:"gpu_watts"`
		OtherWatts float64 `json:"other_watts"`
		Hours      float64 `json:"hours"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	// 默认值
	if req.DeviceName == "" {
		req.DeviceName = req.DeviceID
	}
	if req.Hours <= 0 {
		req.Hours = 1
	}

	consumption := &EnergyConsumption{
		Timestamp:   time.Now(),
		CPUWatts:    req.CPUWatts,
		DiskWatts:   req.DiskWatts,
		NetWatts:    req.NetWatts,
		MemoryWatts: req.MemWatts,
		GPUWatts:    req.GPUWatts,
		OtherWatts:  req.OtherWatts,
		TotalWatts:  req.CPUWatts + req.DiskWatts + req.NetWatts + req.MemWatts + req.GPUWatts + req.OtherWatts,
	}

	footprint, err := h.manager.CalculateFootprint(req.DeviceID, req.DeviceName, consumption, time.Duration(req.Hours*float64(time.Hour)))
	if err != nil {
		h.logger.Error("Failed to calculate footprint", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "计算碳足迹失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    footprint,
	})
}

// getGreenScore 获取绿色评分
// GET /api/v1/carbon/score/:deviceID.
func (h *TrackerHandler) getGreenScore(c *gin.Context) {
	deviceID := c.Param("deviceID")

	score, err := h.manager.GetGreenScore(deviceID)
	if err != nil {
		h.logger.Error("Failed to get green score", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "获取绿色评分失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    score,
	})
}

// getEnergySources 获取能源来源列表
// GET /api/v1/carbon/energy-sources.
func (h *TrackerHandler) getEnergySources(c *gin.Context) {
	sources := h.manager.GetEnergySources()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"sources": sources,
			"total":   len(sources),
		},
	})
}

// setEnergySource 设置能源来源
// POST /api/v1/carbon/energy-source.
func (h *TrackerHandler) setEnergySource(c *gin.Context) {
	var source EnergySource

	if err := c.ShouldBindJSON(&source); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "请求参数无效: " + err.Error(),
		})
		return
	}

	if err := h.manager.SetEnergySource(&source); err != nil {
		h.logger.Error("Failed to set energy source", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "设置能源来源失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "能源来源已更新",
	})
}

// getHistory 获取碳排放历史
// GET /api/v1/carbon/history/:deviceID?start_time=...&end_time=...&limit=100.
func (h *TrackerHandler) getHistory(c *gin.Context) {
	deviceID := c.Param("deviceID")

	var startTime, endTime time.Time
	if v := c.Query("start_time"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			startTime = t
		}
	}
	if v := c.Query("end_time"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			endTime = t
		}
	}

	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	history, err := h.manager.GetHistory(deviceID, startTime, endTime, limit)
	if err != nil {
		h.logger.Error("Failed to get history", zap.Error(err))
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   "获取历史记录失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"history": history,
			"total":   len(history),
		},
	})
}

// getEmissions 获取排放记录
// GET /api/v1/carbon/emissions?start_time=...&end_time=...&limit=100.
func (h *TrackerHandler) getEmissions(c *gin.Context) {
	var startTime, endTime time.Time
	if v := c.Query("start_time"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			startTime = t
		}
	}
	if v := c.Query("end_time"); v != "" {
		if t, err := time.Parse(time.RFC3339, v); err == nil {
			endTime = t
		}
	}

	limit := 100
	if v := c.Query("limit"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			limit = n
		}
	}

	emissions := h.manager.GetEmissions(startTime, endTime, limit)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": gin.H{
			"emissions": emissions,
			"total":     len(emissions),
		},
	})
}

// getDeviceSummary 获取设备碳排放汇总
// GET /api/v1/carbon/summary/:deviceID?days=30.
func (h *TrackerHandler) getDeviceSummary(c *gin.Context) {
	deviceID := c.Param("deviceID")

	days := 30
	if v := c.Query("days"); v != "" {
		if n, err := strconv.Atoi(v); err == nil && n > 0 {
			days = n
		}
	}

	summary, err := h.manager.GetDeviceSummary(deviceID, days)
	if err != nil {
		h.logger.Error("Failed to get device summary", zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "获取设备汇总失败: " + err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    summary,
	})
}

// getDashboard 获取仪表盘数据
// GET /api/v1/carbon/dashboard.
func (h *TrackerHandler) getDashboard(c *gin.Context) {
	dashboard := h.manager.GetDashboardData()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    dashboard,
	})
}
