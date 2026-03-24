// Package disk 提供NVMe健康监控API处理器
// Version: v1.0.0 - NVMe S.M.A.R.T测试接口
package disk

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// NVMeHandlers NVMe监控处理器
type NVMeHandlers struct {
	monitor *NVMeMonitor
}

// NewNVMeHandlers 创建NVMe监控处理器
func NewNVMeHandlers(monitor *NVMeMonitor) *NVMeHandlers {
	return &NVMeHandlers{
		monitor: monitor,
	}
}

// RegisterRoutes 注册NVMe路由
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
// @Security BearerAuth
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
// @Security BearerAuth
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
// @Security BearerAuth
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
// @Security BearerAuth
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
// @Security BearerAuth
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
// @Security BearerAuth
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

// NVMeTestRequest NVMe测试请求
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
// @Security BearerAuth
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
// @Security BearerAuth
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
// @Security BearerAuth
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
// @Security BearerAuth
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
// @Security BearerAuth
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
// @Security BearerAuth
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
		TotalDevices     int     `json:"totalDevices"`
		HealthyCount     int     `json:"healthyCount"`
		WarningCount     int     `json:"warningCount"`
		CriticalCount    int     `json:"criticalCount"`
		AvgTemperature   float64 `json:"avgTemperature"`
		AvgHealthPercent float64 `json:"avgHealthPercent"`
		TotalTBW         float64 `json:"totalTBW"`
		TotalCapacity    uint64  `json:"totalCapacity"`
	}{}

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
			"summary":  summary,
			"devices":  devices,
			"lastUpdate": time.Now(),
		},
	})
}