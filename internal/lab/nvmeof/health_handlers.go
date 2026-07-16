// Package nvmeof - NVMe Health Monitoring HTTP Handlers
// 温度监控、寿命预测、性能基准测试 REST API
package nvmeof

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// HealthHandler NVMe健康监控HTTP处理器.
type HealthHandler struct {
	healthManager *HealthManager
	logger        *zap.Logger
}

// NewHealthHandler 创建健康监控处理器.
func NewHealthHandler(healthManager *HealthManager, logger *zap.Logger) *HealthHandler {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &HealthHandler{
		healthManager: healthManager,
		logger:        logger,
	}
}

// RegisterRoutes 注册健康监控路由.
func (h *HealthHandler) RegisterRoutes(rg *gin.RouterGroup) {
	health := rg.Group("/nvmeof/health")
	{
		// 温度监控
		health.GET("/temperature", h.getAllDeviceTemperatureStatuses)
		health.GET("/temperature/:device", h.getDeviceTemperatureStatus)
		health.GET("/temperature/:device/history", h.getTemperatureHistory)
		health.POST("/temperature", h.recordTemperature)
		health.GET("/temperature/config", h.getTemperatureConfig)
		health.PUT("/temperature/config", h.updateTemperatureConfig)

		// 温度告警
		health.GET("/temperature/alerts", h.getRecentAlerts)

		// 寿命预测
		health.GET("/life-prediction", h.getAllLifePredictions)
		health.GET("/life-prediction/:device", h.getLifePrediction)
		health.POST("/life-prediction/:device", h.predictDeviceLife)
		health.PUT("/write-pattern/:device", h.updateWritePattern)

		// 性能基准测试
		health.POST("/benchmark", h.startBenchmark)
		health.GET("/benchmark/:id", h.getBenchmarkResult)
		health.GET("/benchmarks", h.listBenchmarkResults)
	}
}

// ============================================================
// 温度监控接口
// ============================================================

// recordTemperatureReq 记录温度请求.
type recordTemperatureReq struct {
	Device       string  `json:"device" binding:"required"`
	SubsystemNQN string  `json:"subsystem_nqn"`
	Temperature  float64 `json:"temperature" binding:"required"`
}

// recordTemperature 记录设备温度.
func (h *HealthHandler) recordTemperature(c *gin.Context) {
	var req recordTemperatureReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if err := h.healthManager.RecordTemperature(c.Request.Context(), req.Device, req.SubsystemNQN, req.Temperature); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "temperature recorded"})
}

// getDeviceTemperatureStatus 获取设备温度状态.
func (h *HealthHandler) getDeviceTemperatureStatus(c *gin.Context) {
	device := c.Param("device")

	status, err := h.healthManager.GetDeviceTemperatureStatus(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, status)
}

// getAllDeviceTemperatureStatuses 获取所有设备温度状态.
func (h *HealthHandler) getAllDeviceTemperatureStatuses(c *gin.Context) {
	statuses := h.healthManager.GetAllDeviceStatuses()
	c.JSON(http.StatusOK, gin.H{
		"devices": statuses,
		"total":   len(statuses),
	})
}

// getTemperatureHistoryReq 温度历史查询参数.
type getTemperatureHistoryReq struct {
	Limit int `form:"limit"`
}

// getTemperatureHistory 获取设备温度历史.
func (h *HealthHandler) getTemperatureHistory(c *gin.Context) {
	device := c.Param("device")

	var req getTemperatureHistoryReq
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Limit <= 0 {
		req.Limit = 100
	}

	history := h.healthManager.GetTemperatureHistory(device, req.Limit)
	c.JSON(http.StatusOK, gin.H{
		"device":  device,
		"history": history,
		"total":   len(history),
	})
}

// getRecentAlerts 获取最近温度告警.
func (h *HealthHandler) getRecentAlerts(c *gin.Context) {
	var req struct {
		Limit int `form:"limit"`
	}
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}
	if req.Limit <= 0 {
		req.Limit = 50
	}

	alerts := h.healthManager.GetRecentAlerts(req.Limit)
	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

// getTemperatureConfig 获取温度监控配置.
func (h *HealthHandler) getTemperatureConfig(c *gin.Context) {
	cfg := h.healthManager.GetTemperatureConfig()
	c.JSON(http.StatusOK, cfg)
}

// updateTemperatureConfig 更新温度监控配置.
func (h *HealthHandler) updateTemperatureConfig(c *gin.Context) {
	var cfg TemperatureConfig
	if err := c.ShouldBindJSON(&cfg); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.healthManager.UpdateTemperatureConfig(cfg)
	c.JSON(http.StatusOK, gin.H{"message": "temperature config updated"})
}

// ============================================================
// 寿命预测接口
// ============================================================

// predictDeviceLifeReq 设备寿命预测请求.
type predictDeviceLifeReq struct {
	SubsystemNQN         string  `json:"subsystem_nqn"`
	Model                string  `json:"model"`
	Serial               string  `json:"serial"`
	TotalWriteCapacityTB float64 `json:"total_write_capacity_tb"`
	TotalWrittenTB       float64 `json:"total_written_tb"`
	PercentageUsed       int     `json:"percentage_used"`
	AvailableSpare       int     `json:"available_spare"`
	PowerOnHours         uint64  `json:"power_on_hours"`
	UnsafeShutdowns      uint64  `json:"unsafe_shutdowns"`
	MediaErrors          uint64  `json:"media_errors"`
}

// predictDeviceLife 预测设备寿命.
func (h *HealthHandler) predictDeviceLife(c *gin.Context) {
	device := c.Param("device")

	var req predictDeviceLifeReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	prediction, err := h.healthManager.PredictDeviceLife(
		c.Request.Context(),
		device,
		req.SubsystemNQN,
		req.Model,
		req.Serial,
		req.TotalWriteCapacityTB,
		req.TotalWrittenTB,
		req.PercentageUsed,
		req.AvailableSpare,
		req.PowerOnHours,
		req.UnsafeShutdowns,
		req.MediaErrors,
	)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, prediction)
}

// getLifePrediction 获取设备寿命预测.
func (h *HealthHandler) getLifePrediction(c *gin.Context) {
	device := c.Param("device")

	prediction, err := h.healthManager.GetLifePrediction(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, prediction)
}

// getAllLifePredictions 获取所有设备寿命预测.
func (h *HealthHandler) getAllLifePredictions(c *gin.Context) {
	predictions := h.healthManager.GetAllLifePredictions()
	c.JSON(http.StatusOK, gin.H{
		"predictions": predictions,
		"total":       len(predictions),
	})
}

// updateWritePatternReq 更新写入模式请求.
type updateWritePatternReq struct {
	SubsystemNQN       string  `json:"subsystem_nqn"`
	TotalWriteTB       float64 `json:"total_write_tb"`
	TotalReadTB        float64 `json:"total_read_tb"`
	DailyWriteAvgGB    float64 `json:"daily_write_avg_gb"`
	WeeklyWriteAvgGB   float64 `json:"weekly_write_avg_gb"`
	PeakWriteRateGBps  float64 `json:"peak_write_rate_gbps"`
	WriteAmplification float64 `json:"write_amplification"`
	SamplePeriodDays   int     `json:"sample_period_days"`
}

// updateWritePattern 更新设备写入模式.
func (h *HealthHandler) updateWritePattern(c *gin.Context) {
	device := c.Param("device")

	var req updateWritePatternReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	h.healthManager.UpdateWritePattern(
		device,
		req.SubsystemNQN,
		req.TotalWriteTB,
		req.TotalReadTB,
		req.DailyWriteAvgGB,
		req.WeeklyWriteAvgGB,
		req.PeakWriteRateGBps,
		req.WriteAmplification,
		req.SamplePeriodDays,
	)

	c.JSON(http.StatusOK, gin.H{"message": "write pattern updated"})
}

// ============================================================
// 性能基准测试接口
// ============================================================

// startBenchmarkReq 启动基准测试请求.
type startBenchmarkReq struct {
	DevicePath   string   `json:"device_path" binding:"required"`
	SubsystemNQN string   `json:"subsystem_nqn"`
	BlockSizeKB  int      `json:"block_size_kb"`
	FileSizeMB   int      `json:"file_size_mb"`
	DurationSec  int      `json:"duration_sec"`
	NumThreads   int      `json:"num_threads"`
	TestTypes    []string `json:"test_types"`
}

// startBenchmark 启动性能基准测试.
func (h *HealthHandler) startBenchmark(c *gin.Context) {
	var req startBenchmarkReq
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	cfg := BenchmarkConfig(req)

	result, err := h.healthManager.StartBenchmark(c.Request.Context(), cfg)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusAccepted, result)
}

// getBenchmarkResult 获取基准测试结果.
func (h *HealthHandler) getBenchmarkResult(c *gin.Context) {
	id := c.Param("id")

	result, err := h.healthManager.GetBenchmarkResult(id)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

// listBenchmarkResults 列出所有基准测试结果.
func (h *HealthHandler) listBenchmarkResults(c *gin.Context) {
	results := h.healthManager.ListBenchmarkResults()
	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(results),
	})
}
