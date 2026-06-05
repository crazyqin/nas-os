// Package smartfancontrol 提供智能风扇控制 HTTP 处理器
package smartfancontrol

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// Handlers 智能风扇控制 HTTP 处理器
type Handlers struct {
	manager *Manager
}

// NewHandlers 创建处理器
func NewHandlers(manager *Manager) *Handlers {
	return &Handlers{manager: manager}
}

// RegisterRoutes 注册路由
func (h *Handlers) RegisterRoutes(api *gin.RouterGroup) {
	fanGroup := api.Group("/smart-fan")
	{
		// 状态查询
		fanGroup.GET("/status", h.getStatusReport)
		fanGroup.GET("/fans/:id", h.getFan)
		fanGroup.GET("/temps/:sensorId", h.getTemperature)

		// 模式控制
		fanGroup.GET("/mode", h.getMode)
		fanGroup.PUT("/mode", h.setMode)

		// 风扇曲线
		fanGroup.GET("/profiles", h.listProfiles)
		fanGroup.GET("/profiles/:name", h.getProfile)
		fanGroup.POST("/profiles", h.setFanCurve)

		// 手动控制
		fanGroup.PUT("/fans/:id/speed", h.setFanSpeed)

		// 告警
		fanGroup.GET("/alerts", h.getAlerts)
		fanGroup.DELETE("/alerts", h.clearAlerts)
	}
}

// getStatusReport 获取状态报告
func (h *Handlers) getStatusReport(c *gin.Context) {
	report := h.manager.GetStatusReport()
	c.JSON(http.StatusOK, report)
}

// getFan 获取风扇信息
func (h *Handlers) getFan(c *gin.Context) {
	fanID := c.Param("id")
	fan, err := h.manager.GetFan(fanID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, fan)
}

// getTemperature 获取温度信息
func (h *Handlers) getTemperature(c *gin.Context) {
	sensorID := c.Param("sensorId")
	temp, err := h.manager.GetTemperature(sensorID)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, temp)
}

// getMode 获取当前模式
func (h *Handlers) getMode(c *gin.Context) {
	mode := h.manager.GetMode()
	c.JSON(http.StatusOK, gin.H{
		"mode": mode,
	})
}

// setModeRequest 设置模式请求
type setModeRequest struct {
	Mode FanMode `json:"mode" binding:"required"`
}

// setMode 设置模式
func (h *Handlers) setMode(c *gin.Context) {
	var req setModeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.SetMode(req.Mode); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "模式设置成功",
		"mode":    req.Mode,
	})
}

// listProfiles 列出所有配置
func (h *Handlers) listProfiles(c *gin.Context) {
	profiles := h.manager.ListProfiles()
	c.JSON(http.StatusOK, gin.H{
		"profiles": profiles,
		"total":    len(profiles),
	})
}

// getProfile 获取配置
func (h *Handlers) getProfile(c *gin.Context) {
	name := c.Param("name")
	profile, err := h.manager.GetProfile(name)
	if err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}
	c.JSON(http.StatusOK, profile)
}

// setFanCurveRequest 设置曲线请求
type setFanCurveRequest struct {
	Name  string          `json:"name" binding:"required"`
	Curve []FanCurvePoint `json:"curve" binding:"required"`
}

// setFanCurve 设置自定义曲线
func (h *Handlers) setFanCurve(c *gin.Context) {
	var req setFanCurveRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.SetFanCurve(req.Name, req.Curve); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"message": "风扇曲线设置成功",
		"name":    req.Name,
	})
}

// setFanSpeedRequest 设置转速请求
type setFanSpeedRequest struct {
	DutyCycle float64 `json:"dutyCycle" binding:"required"`
}

// setFanSpeed 手动设置转速
func (h *Handlers) setFanSpeed(c *gin.Context) {
	fanID := c.Param("id")

	var req setFanSpeedRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请求参数无效: " + err.Error()})
		return
	}

	if err := h.manager.SetFanSpeed(fanID, req.DutyCycle); err != nil {
		code := http.StatusBadRequest
		if err.Error() == "fan not found: "+fanID {
			code = http.StatusNotFound
		}
		c.JSON(code, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "风扇转速设置成功",
		"fanId":   fanID,
	})
}

// getAlerts 获取告警
func (h *Handlers) getAlerts(c *gin.Context) {
	alerts := h.manager.GetAlerts()
	c.JSON(http.StatusOK, gin.H{
		"alerts": alerts,
		"total":  len(alerts),
	})
}

// clearAlerts 清除告警
func (h *Handlers) clearAlerts(c *gin.Context) {
	h.manager.ClearAlerts()
	c.JSON(http.StatusOK, gin.H{
		"message": "告警已清除",
	})
}

// GetReport 获取状态报告 (供外部调用)
func (m *Manager) GetReport() *FanStatusReport {
	return m.GetStatusReport()
}

// AddFan 添加风扇 (供外部调用)
func (m *Manager) AddFan(id, name string, maxRPM int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.fans[id] = &FanInfo{
		ID:        id,
		Name:      name,
		MaxRPM:    maxRPM,
		Status:    FanStatusOK,
		UpdatedAt: time.Now(),
	}
	log.Printf("[智能风扇] 添加风扇: %s (%s)", id, name)
}

// AddTemperatureSensor 添加温度传感器 (供外部调用)
func (m *Manager) AddTemperatureSensor(sensorID, name string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.temps[sensorID] = &TemperaturePoint{
		SensorID:  sensorID,
		Name:      name,
		UpdatedAt: time.Now(),
	}
	log.Printf("[智能风扇] 添加温度传感器: %s (%s)", sensorID, name)
}

// UpdateTemperature 更新温度 (供外部调用)
func (m *Manager) UpdateTemperature(sensorID string, temp float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.temps[sensorID]
	if !ok {
		return fmt.Errorf("temperature sensor not found: %s", sensorID)
	}

	t.Temp = temp
	if temp > t.MaxTemp {
		t.MaxTemp = temp
	}
	t.UpdatedAt = time.Now()
	return nil
}

// UpdateFanStatus 更新风扇状态 (供外部调用)
func (m *Manager) UpdateFanStatus(fanID string, status FanStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fan, ok := m.fans[fanID]
	if !ok {
		return fmt.Errorf("fan not found: %s", fanID)
	}

	fan.Status = status
	fan.UpdatedAt = time.Now()
	return nil
}
