// Package disk 提供NVMe健康监控API处理器
// Version: v1.0.0 - NVMe S.M.A.R.T测试接口
package disk

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// NVMeHandlers NVMe监控处理器.
type NVMeHandlers struct {
	monitor *NVMeMonitor
}

// NewNVMeHandlers 创建NVMe监控处理器.
func NewNVMeHandlers(monitor *NVMeMonitor) *NVMeHandlers {
	return &NVMeHandlers{
		monitor: monitor,
	}
}

// RegisterRoutes 注册NVMe路由.
func (h *NVMeHandlers) RegisterRoutes(r *gin.RouterGroup) {
	nvme := r.Group("/nvme")
	{
		// 设备列表和健康状态
		nvme.GET("", h.listNVMeDevices)
		nvme.GET("/:device", h.getNVMeHealth)

		// SMART数据
		nvme.GET("/:device/smart", h.getNVMeSmart)
		nvme.GET("/:device/temperature", h.getNVMeTemperature)
		nvme.GET("/:device/usage", h.getNVMeUsage)

		// v2.388.0: 三级预警和寿命预测
		nvme.GET("/:device/alert", h.getNVMeAlertStatus)
		nvme.GET("/:device/life-prediction", h.getNVMeLifePrediction)
		nvme.GET("/:device/alert-history", h.getNVMeAlertHistory)
		nvme.GET("/alerts/summary", h.getNVMeAlertSummary)

		// 预警阈值配置
		nvme.GET("/alert-thresholds", h.getAlertThresholds)
		nvme.PUT("/alert-thresholds", h.updateAlertThresholds)

		// 温度历史
		nvme.GET("/:device/temperature-history", h.getNVMeTemperatureHistory)

		// 测试接口
		nvme.POST("/:device/test", h.runNVMeTest)
		nvme.GET("/:device/test", h.getTestStatus)
		nvme.DELETE("/:device/test", h.abortTest)

		// 扫描和刷新
		nvme.POST("/scan", h.scanNVMeDevices)
		nvme.POST("/:device/refresh", h.refreshNVMeDevice)

		// 批量操作
		nvme.POST("/test-all", h.runAllNVMeTest)
		nvme.GET("/summary", h.getNVMeSummary)
	}
}

// listNVMeDevices 获取NVMe设备列表
// @Summary 获取NVMe设备列表
// @Description 获取所有NVMe设备的健康状态
// @Tags nvme
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme [get]
// @Security BearerAuth.
func (h *NVMeHandlers) listNVMeDevices(c *gin.Context) {
	devices, err := h.monitor.GetAllNVMeDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	// 生成摘要
	summary := struct {
		Total    int `json:"total"`
		Healthy  int `json:"healthy"`
		Warning  int `json:"warning"`
		Critical int `json:"critical"`
		Unknown  int `json:"unknown"`
	}{}

	for _, dev := range devices {
		summary.Total++
		switch dev.Status {
		case StatusHealthy:
			summary.Healthy++
		case StatusWarning:
			summary.Warning++
		case StatusCritical:
			summary.Critical++
		default:
			summary.Unknown++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"summary": summary,
			"devices": devices,
		},
	})
}

// getNVMeHealth 获取NVMe设备健康详情
// @Summary 获取NVMe设备健康详情
// @Description 获取指定NVMe设备的详细健康信息
// @Tags nvme
// @Accept json
// @Produce json
// @Param device path string true "设备路径 (如 /dev/nvme0)"
// @Success 200 {object} map[string]interface{} "成功"
// @Failure 404 {object} map[string]interface{} "设备不存在"
// @Router /nvme/{device} [get]
// @Security BearerAuth.
func (h *NVMeHandlers) getNVMeHealth(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	// 确保设备路径以/dev/开头
	if len(device) < 5 || device[:5] != "/dev/" {
		device = "/dev/" + device
	}

	info, err := h.monitor.GetNVMeHealth(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    info,
	})
}

// getNVMeSmart 获取NVMe SMART数据
// @Summary 获取NVMe SMART数据
// @Description 获取指定NVMe设备的原始SMART数据
// @Tags nvme
// @Accept json
// @Produce json
// @Param device path string true "设备路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/{device}/smart [get]
// @Security BearerAuth.
func (h *NVMeHandlers) getNVMeSmart(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	if len(device) < 5 || device[:5] != "/dev/" {
		device = "/dev/" + device
	}

	info, err := h.monitor.GetNVMeHealth(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"device":            info.Device,
			"model":             info.Model,
			"serial":            info.Serial,
			"firmware":          info.Firmware,
			"smartStatus":       info.SmartStatus,
			"healthPercentage":  info.HealthPercentage,
			"temperature":       info.Temperature,
			"usage":             info.Usage,
			"availableSpare":    info.AvailableSpare,
			"mediaErrors":       info.MediaErrors,
			"criticalWarnings":  info.CriticalWarnings,
			"powerOnHours":      info.PowerOnHours,
			"powerCycles":       info.PowerCycles,
			"unsafeShutdowns":   info.UnsafeShutdowns,
			"dataUnitsRead":     info.DataUnitsRead,
			"dataUnitsWritten":  info.DataUnitsWritten,
			"hostReadCommands":  info.HostReadCommands,
			"hostWriteCommands": info.HostWriteCommands,
		},
	})
}

// getNVMeTemperature 获取NVMe温度信息
// @Summary 获取NVMe温度信息
// @Description 获取指定NVMe设备的温度数据
// @Tags nvme
// @Accept json
// @Produce json
// @Param device path string true "设备路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/{device}/temperature [get]
// @Security BearerAuth.
func (h *NVMeHandlers) getNVMeTemperature(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	if len(device) < 5 || device[:5] != "/dev/" {
		device = "/dev/" + device
	}

	info, err := h.monitor.GetNVMeHealth(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	if info.Temperature == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "无温度数据",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"device":       info.Device,
			"temperature":  info.Temperature,
			"model":        info.Model,
			"healthStatus": info.Status,
		},
	})
}

// getNVMeUsage 获取NVMe使用情况
// @Summary 获取NVMe使用情况
// @Description 获取指定NVMe设备的使用情况（写入量、寿命等）
// @Tags nvme
// @Accept json
// @Produce json
// @Param device path string true "设备路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/{device}/usage [get]
// @Security BearerAuth.
func (h *NVMeHandlers) getNVMeUsage(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	if len(device) < 5 || device[:5] != "/dev/" {
		device = "/dev/" + device
	}

	info, err := h.monitor.GetNVMeHealth(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	if info.Usage == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "无使用数据",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"device":       info.Device,
			"model":        info.Model,
			"usage":        info.Usage,
			"powerOnHours": info.PowerOnHours,
			"powerCycles":  info.PowerCycles,
			"healthStatus": info.Status,
		},
	})
}

// runNVMeTest 运行NVMe测试
// @Summary 运行NVMe测试
// @Description 对指定NVMe设备运行自检测试
// @Tags nvme
// @Accept json
// @Produce json
// @Param device path string true "设备路径"
// @Param request body NVMeTestRequest true "测试参数"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/{device}/test [post]
// @Security BearerAuth.
func (h *NVMeHandlers) runNVMeTest(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	if len(device) < 5 || device[:5] != "/dev/" {
		device = "/dev/" + device
	}

	var req NVMeTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		// 默认短测试
		req.TestType = "short"
	}

	testType := NVMeTestShort
	switch req.TestType {
	case "long":
		testType = NVMeTestLong
	case "vendor":
		testType = NVMeTestVendor
	case "verify":
		testType = NVMeTestVerify
	}

	result, err := h.monitor.RunNVMeTest(device, testType)
	if err != nil {
		c.JSON(http.StatusConflict, gin.H{
			"code":    409,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusAccepted, gin.H{
		"code":    0,
		"message": "测试已启动",
		"data":    result,
	})
}

// NVMeTestRequest NVMe测试请求.
type NVMeTestRequest struct {
	TestType string `json:"testType"` // short/long/vendor/verify
}

// getTestStatus 获取测试状态
// @Summary 获取测试状态
// @Description 获取指定NVMe设备的测试状态和结果
// @Tags nvme
// @Accept json
// @Produce json
// @Param device path string true "设备路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/{device}/test [get]
// @Security BearerAuth.
func (h *NVMeHandlers) getTestStatus(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	if len(device) < 5 || device[:5] != "/dev/" {
		device = "/dev/" + device
	}

	result, err := h.monitor.GetTestStatus(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    result,
	})
}

// abortTest 中止测试
// @Summary 中止测试
// @Description 中止正在运行的NVMe测试
// @Tags nvme
// @Accept json
// @Produce json
// @Param device path string true "设备路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/{device}/test [delete]
// @Security BearerAuth.
func (h *NVMeHandlers) abortTest(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	if len(device) < 5 || device[:5] != "/dev/" {
		device = "/dev/" + device
	}

	if err := h.monitor.AbortTest(device); err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "测试已中止",
	})
}

// scanNVMeDevices 扫描NVMe设备
// @Summary 扫描NVMe设备
// @Description 重新扫描系统中的NVMe设备
// @Tags nvme
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/scan [post]
// @Security BearerAuth.
func (h *NVMeHandlers) scanNVMeDevices(c *gin.Context) {
	devices, err := h.monitor.ScanNVMeDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "扫描完成",
		"data": gin.H{
			"count":   len(devices),
			"devices": devices,
		},
	})
}

// refreshNVMeDevice 刷新NVMe设备数据
// @Summary 刷新NVMe设备数据
// @Description 强制刷新指定NVMe设备的健康数据
// @Tags nvme
// @Accept json
// @Produce json
// @Param device path string true "设备路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/{device}/refresh [post]
// @Security BearerAuth.
func (h *NVMeHandlers) refreshNVMeDevice(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	if len(device) < 5 || device[:5] != "/dev/" {
		device = "/dev/" + device
	}

	// 清除缓存
	h.monitor.ClearCache(device)

	// 重新获取
	info, err := h.monitor.GetNVMeHealth(device)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "刷新成功",
		"data":    info,
	})
}

// runAllNVMeTest 对所有NVMe设备运行测试
// @Summary 对所有NVMe设备运行测试
// @Description 对所有NVMe设备运行自检测试
// @Tags nvme
// @Accept json
// @Produce json
// @Param request body NVMeTestRequest true "测试参数"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/test-all [post]
// @Security BearerAuth.
func (h *NVMeHandlers) runAllNVMeTest(c *gin.Context) {
	var req NVMeTestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		req.TestType = "short"
	}

	testType := NVMeTestShort
	if req.TestType == "long" {
		testType = NVMeTestLong
	}

	devices, err := h.monitor.ScanNVMeDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	results := make(map[string]interface{})
	for _, device := range devices {
		result, err := h.monitor.RunNVMeTest(device, testType)
		if err != nil {
			results[device] = gin.H{
				"status":  "failed",
				"message": err.Error(),
			}
		} else {
			results[device] = result
		}
	}

	c.JSON(http.StatusAccepted, gin.H{
		"code":    0,
		"message": "测试已启动",
		"data": gin.H{
			"total":   len(devices),
			"results": results,
		},
	})
}

// getNVMeSummary 获取NVMe设备摘要
// @Summary 获取NVMe设备摘要
// @Description 获取所有NVMe设备的健康状态摘要
// @Tags nvme
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/summary [get]
// @Security BearerAuth.
func (h *NVMeHandlers) getNVMeSummary(c *gin.Context) {
	devices, err := h.monitor.GetAllNVMeDevices()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"code":    500,
			"message": err.Error(),
		})
		return
	}

	// 计算汇总数据
	summary := struct {
		TotalDevices     int                `json:"totalDevices"`
		HealthyCount     int                `json:"healthyCount"`
		WarningCount     int                `json:"warningCount"`
		CriticalCount    int                `json:"criticalCount"`
		EmergencyCount   int                `json:"emergencyCount"`
		AvgTemperature   float64            `json:"avgTemperature"`
		AvgHealthPercent float64            `json:"avgHealthPercent"`
		TotalTBW         float64            `json:"totalTBW"`
		TotalCapacity    uint64             `json:"totalCapacity"`
		AlertsByLevel    map[AlertLevel]int `json:"alertsByLevel"`
	}{}

	summary.AlertsByLevel = make(map[AlertLevel]int)
	var totalTemp float64
	var tempCount int
	var totalHealth float64

	for _, dev := range devices {
		summary.TotalDevices++
		summary.TotalCapacity += dev.Size

		switch dev.Status {
		case StatusHealthy:
			summary.HealthyCount++
		case StatusWarning:
			summary.WarningCount++
		case StatusCritical:
			summary.CriticalCount++
		}

		// 三级预警统计
		summary.AlertsByLevel[dev.AlertLevel]++

		if dev.Temperature != nil {
			totalTemp += float64(dev.Temperature.Current)
			tempCount++
		}

		totalHealth += float64(dev.HealthPercentage)

		if dev.Usage != nil {
			summary.TotalTBW += dev.Usage.TBW
		}
	}

	if tempCount > 0 {
		summary.AvgTemperature = totalTemp / float64(tempCount)
	}

	if summary.TotalDevices > 0 {
		summary.AvgHealthPercent = totalHealth / float64(summary.TotalDevices)
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"summary":    summary,
			"devices":    devices,
			"lastUpdate": time.Now(),
		},
	})
}

// ==================== v2.388.0 三级预警 + 寿命预测 API ====================

// getNVMeAlertStatus 获取NVMe预警状态
// @Summary 获取NVMe预警状态
// @Description 获取指定NVMe设备的三级预警状态详情
// @Tags nvme
// @Accept json
// @Produce json
// @Param device path string true "设备路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/{device}/alert [get]
// @Security BearerAuth.
func (h *NVMeHandlers) getNVMeAlertStatus(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	if len(device) < 5 || device[:5] != "/dev/" {
		device = "/dev/" + device
	}

	info, err := h.monitor.GetNVMeHealth(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	// 获取推荐建议
	recommendations := []string{}
	if info.HealthScore != nil {
		recommendations = info.HealthScore.Recommendations
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"device":          info.Device,
			"alertLevel":      info.AlertLevel,
			"alertReasons":    info.AlertReasons,
			"healthPct":       info.HealthPercentage,
			"thresholds":      info.AlertThresholds,
			"recommendations": recommendations,
			"timestamp":       info.LastCheck,
		},
	})
}

// getNVMeLifePrediction 获取NVMe寿命预测
// @Summary 获取NVMe寿命预测
// @Description 获取指定NVMe设备的剩余寿命预测数据
// @Tags nvme
// @Accept json
// @Produce json
// @Param device path string true "设备路径"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/{device}/life-prediction [get]
// @Security BearerAuth.
func (h *NVMeHandlers) getNVMeLifePrediction(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	if len(device) < 5 || device[:5] != "/dev/" {
		device = "/dev/" + device
	}

	info, err := h.monitor.GetNVMeHealth(device)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": err.Error(),
		})
		return
	}

	prediction := h.monitor.GetLifePrediction(device)
	if prediction == nil {
		c.JSON(http.StatusNotFound, gin.H{
			"code":    404,
			"message": "无寿命预测数据",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"device":           info.Device,
			"model":            info.Model,
			"healthPercentage": info.HealthPercentage,
			"prediction":       prediction,
			"usage":            info.Usage,
			"powerOnHours":     info.PowerOnHours,
			"lastCheck":        info.LastCheck,
		},
	})
}

// getNVMeAlertHistory 获取NVMe预警历史
// @Summary 获取NVMe预警历史
// @Description 获取指定NVMe设备的预警事件历史记录
// @Tags nvme
// @Accept json
// @Produce json
// @Param device path string true "设备路径"
// @Param limit query int false "返回数量限制" default(100)
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/{device}/alert-history [get]
// @Security BearerAuth.
func (h *NVMeHandlers) getNVMeAlertHistory(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	if len(device) < 5 || device[:5] != "/dev/" {
		device = "/dev/" + device
	}

	// 获取温度历史作为预警参考
	tempHistory := h.monitor.GetTemperatureHistory(device)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"device":             device,
			"temperatureHistory": tempHistory,
			"lastPrediction":     h.monitor.GetLifePrediction(device),
		},
	})
}

// getNVMeAlertSummary 获取所有NVMe预警摘要
// @Summary 获取所有NVMe预警摘要
// @Description 获取所有NVMe设备的预警状态汇总
// @Tags nvme
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/alerts/summary [get]
// @Security BearerAuth.
func (h *NVMeHandlers) getNVMeAlertSummary(c *gin.Context) {
	summary := h.monitor.GetAlertSummary()

	// 计算各级别数量
	levelCount := map[AlertLevel]int{
		AlertLevelNormal:    0,
		AlertLevelWarning:   0,
		AlertLevelCritical:  0,
		AlertLevelEmergency: 0,
	}

	for _, data := range summary {
		levelCount[data.Level]++
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"summary":      summary,
			"levelCounts":  levelCount,
			"totalDevices": len(summary),
			"timestamp":    time.Now(),
		},
	})
}

// getAlertThresholds 获取预警阈值配置
// @Summary 获取预警阈值配置
// @Description 获取当前的三级预警阈值配置
// @Tags nvme
// @Accept json
// @Produce json
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/alert-thresholds [get]
// @Security BearerAuth.
func (h *NVMeHandlers) getAlertThresholds(c *gin.Context) {
	thresholds := h.monitor.GetAlertThresholds()

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data":    thresholds,
	})
}

// alertThresholdsRequest 预警阈值请求
type alertThresholdsRequest struct {
	TempWarning     uint8 `json:"tempWarning"`
	TempCritical    uint8 `json:"tempCritical"`
	HealthWarning   uint8 `json:"healthWarning"`
	HealthCritical  uint8 `json:"healthCritical"`
	HealthEmergency uint8 `json:"healthEmergency"`
	TBWWarning      uint8 `json:"tbwWarning"`
	TBWCritical     uint8 `json:"tbwCritical"`
}

// updateAlertThresholds 更新预警阈值配置
// @Summary 更新预警阈值配置
// @Description 更新三级预警阈值配置
// @Tags nvme
// @Accept json
// @Produce json
// @Param request body alertThresholdsRequest true "阈值配置"
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/alert-thresholds [put]
// @Security BearerAuth.
func (h *NVMeHandlers) updateAlertThresholds(c *gin.Context) {
	var req alertThresholdsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": err.Error(),
		})
		return
	}

	thresholds := &AlertThresholds{
		TempWarning:     req.TempWarning,
		TempCritical:    req.TempCritical,
		HealthWarning:   req.HealthWarning,
		HealthCritical:  req.HealthCritical,
		HealthEmergency: req.HealthEmergency,
		TBWWarning:      req.TBWWarning,
		TBWCritical:     req.TBWCritical,
	}

	h.monitor.SetAlertThresholds(thresholds)

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "预警阈值已更新",
		"data":    thresholds,
	})
}

// getNVMeTemperatureHistory 获取NVMe温度历史
// @Summary 获取NVMe温度历史
// @Description 获取指定NVMe设备的温度历史记录
// @Tags nvme
// @Accept json
// @Produce json
// @Param device path string true "设备路径"
// @Param limit query int false "返回数量限制" default(100)
// @Success 200 {object} map[string]interface{} "成功"
// @Router /nvme/{device}/temperature-history [get]
// @Security BearerAuth.
func (h *NVMeHandlers) getNVMeTemperatureHistory(c *gin.Context) {
	device := c.Param("device")
	if device == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"code":    400,
			"message": "设备路径不能为空",
		})
		return
	}

	if len(device) < 5 || device[:5] != "/dev/" {
		device = "/dev/" + device
	}

	history := h.monitor.GetTemperatureHistory(device)

	// 计算温度统计
	var avgTemp, minTemp, maxTemp float64
	var totalDuration float64
	if len(history) > 0 {
		minTemp = float64(history[0].Temp)
		maxTemp = float64(history[0].Temp)
		for _, record := range history {
			avgTemp += float64(record.Temp) * record.Duration
			totalDuration += record.Duration
			if float64(record.Temp) < minTemp {
				minTemp = float64(record.Temp)
			}
			if float64(record.Temp) > maxTemp {
				maxTemp = float64(record.Temp)
			}
		}
		if totalDuration > 0 {
			avgTemp /= totalDuration
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"code":    0,
		"message": "success",
		"data": gin.H{
			"device":  device,
			"history": history,
			"stats": gin.H{
				"avgTemp":     avgTemp,
				"minTemp":     minTemp,
				"maxTemp":     maxTemp,
				"recordCount": len(history),
			},
		},
	})
}
