// handlers.go - SmartThermal2 HTTP 接口
package smartthermal2

import (
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
)

// Handlers 温控接口处理器.
type Handlers struct {
	logger    *zap.Logger
	engine    *ThermalEngine
	fc        *FanController
	noise     *NoiseOptimizer
	predictor *ThermalPredictor
	profiles  *ProfileManager
	alerts    *AlertManager
}

// NewHandlers 创建温控接口处理器.
func NewHandlers(
	logger *zap.Logger,
	engine *ThermalEngine,
	fc *FanController,
	noise *NoiseOptimizer,
	predictor *ThermalPredictor,
	profiles *ProfileManager,
	alerts *AlertManager,
) *Handlers {
	return &Handlers{
		logger: logger, engine: engine, fc: fc,
		noise: noise, predictor: predictor, profiles: profiles, alerts: alerts,
	}
}

// RegisterRoutes 注册路由.
func (h *Handlers) RegisterRoutes(rg *gin.RouterGroup) {
	st := rg.Group("/smartthermal2")
	{
		st.GET("/sensors", h.getSensors)
		st.GET("/sensors/:id/history", h.getSensorHistory)
		st.GET("/fans", h.getFans)
		st.PUT("/fans/:id", h.updateFan)
		st.GET("/profiles", h.getProfiles)
		st.POST("/profiles", h.createProfile)
		st.PUT("/active-profile", h.setActiveProfile)
		st.GET("/dashboard", h.getDashboard)
		st.GET("/predict", h.getPredict)
		st.GET("/noise", h.getNoise)
		st.PUT("/settings", h.updateSettings)
		st.GET("/alerts", h.getAlerts)
		st.POST("/emergency", h.emergencyCooling)
	}
}

// getSensors 获取传感器列表和实时温度.
func (h *Handlers) getSensors(c *gin.Context) {
	sensors := h.engine.GetSensors()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Data: sensors})
}

// getSensorHistory 获取传感器历史.
func (h *Handlers) getSensorHistory(c *gin.Context) {
	id := c.Param("id")
	minutes := 60
	if m, err := strconv.Atoi(c.Query("minutes")); err == nil && m > 0 {
		minutes = m
	}
	history := h.engine.GetSensorHistory(id, minutes)
	if history == nil {
		history = []SensorHistory{}
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Data: history})
}

// getFans 获取风扇列表和状态.
func (h *Handlers) getFans(c *gin.Context) {
	fans := h.fc.GetFans()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Data: fans})
}

// updateFan 更新风扇设置.
func (h *Handlers) updateFan(c *gin.Context) {
	id := c.Param("id")
	var req FanUpdateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: "无效的请求参数"})
		return
	}
	if err := h.fc.UpdateFan(id, req); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	fan, _ := h.fc.GetFan(id)
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "风扇设置已更新", Data: fan})
}

// getProfiles 获取散热方案列表.
func (h *Handlers) getProfiles(c *gin.Context) {
	profiles := h.profiles.List()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Data: profiles})
}

// createProfile 创建散热方案.
func (h *Handlers) createProfile(c *gin.Context) {
	var req ProfileCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: "无效的请求参数"})
		return
	}
	profile, err := h.profiles.Create(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusCreated, APIResponse{Code: 0, Message: "方案已创建", Data: profile})
}

// setActiveProfile 切换活跃方案.
func (h *Handlers) setActiveProfile(c *gin.Context) {
	var req ProfileSwitchRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: "无效的请求参数"})
		return
	}
	if err := h.profiles.SetActive(req.ProfileID); err != nil {
		c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
		return
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "方案已切换"})
}

// getDashboard 获取温控仪表板.
func (h *Handlers) getDashboard(c *gin.Context) {
	sensors := h.engine.GetSensors()
	zones := h.engine.GetZones()
	fans := h.fc.GetFans()
	noise := h.noise.Assess()
	activeAlerts := h.alerts.GetActive()
	predictions := h.predictor.PredictAll(30)

	// 确定整体状态
	overall := SensorNormal
	for _, s := range sensors {
		if s.Status == SensorEmergency {
			overall = SensorEmergency
		} else if s.Status == SensorCritical && overall != SensorEmergency {
			overall = SensorCritical
		} else if s.Status == SensorWarning && overall == SensorNormal {
			overall = SensorWarning
		}
	}

	activeProfile := h.profiles.GetActive()
	profileName := ""
	if activeProfile != nil {
		profileName = activeProfile.Name
	}

	dashboard := Dashboard{
		OverallStatus:  overall,
		CurrentProfile: profileName,
		Sensors:        sensors,
		Zones:          zones,
		Fans:           fans,
		Noise:          noise,
		ActiveAlerts:   activeAlerts,
		Predictions:    predictions,
		UpdatedAt:      time.Now(),
	}
	c.JSON(http.StatusOK, APIResponse{Code: 0, Data: dashboard})
}

// getPredict 获取温度预测.
func (h *Handlers) getPredict(c *gin.Context) {
	minutes := 30
	if m, err := strconv.Atoi(c.Query("minutes")); err == nil && m > 0 {
		minutes = m
	}
	sensorID := c.Query("sensor")
	if sensorID != "" {
		result, err := h.predictor.Predict(sensorID, minutes)
		if err != nil {
			c.JSON(http.StatusNotFound, APIResponse{Code: -1, Message: err.Error()})
			return
		}
		c.JSON(http.StatusOK, APIResponse{Code: 0, Data: result})
		return
	}
	results := h.predictor.PredictAll(minutes)
	c.JSON(http.StatusOK, APIResponse{Code: 0, Data: results})
}

// getNoise 获取噪音评估.
func (h *Handlers) getNoise(c *gin.Context) {
	assessment := h.noise.Assess()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Data: assessment})
}

// updateSettings 更新全局设置.
func (h *Handlers) updateSettings(c *gin.Context) {
	var settings GlobalSettings
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, APIResponse{Code: -1, Message: "无效的设置参数"})
		return
	}
	h.engine.UpdateSettings(settings)
	h.noise.UpdateSettings(settings.NoiseSettings)
	h.alerts.UpdateSettings(settings.AlertSettings)
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "设置已更新", Data: settings})
}

// getAlerts 获取告警列表.
func (h *Handlers) getAlerts(c *gin.Context) {
	limit := 50
	if l, err := strconv.Atoi(c.Query("limit")); err == nil && l > 0 {
		limit = l
	}
	alerts := h.alerts.GetAll(limit)
	c.JSON(http.StatusOK, APIResponse{Code: 0, Data: alerts})
}

// emergencyCooling 紧急降温.
func (h *Handlers) emergencyCooling(c *gin.Context) {
	h.alerts.EmergencyCooling()
	c.JSON(http.StatusOK, APIResponse{Code: 0, Message: "紧急降温已启动"})
}
