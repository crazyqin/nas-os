// Package reports 提供报表生成和管理功能
package reports

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// ========== 电费计算 API 处理器 ==========

// ElectricityAPI 电费计算API
type ElectricityAPI struct {
	calculator *ElectricityCostCalculator
	profiles   map[string]PowerProfile
	tariffs    map[string]ElectricityTariff
}

// NewElectricityAPI 创建电费计算API
func NewElectricityAPI(config ElectricityCostConfig) *ElectricityAPI {
	// 使用默认北京电价
	tariff := DefaultTariffs["beijing_general"]
	if config.TariffID != "" {
		if t, ok := DefaultTariffs[config.TariffID]; ok {
			tariff = t
		}
	}

	return &ElectricityAPI{
		calculator: NewElectricityCostCalculator(config, tariff),
		profiles:   make(map[string]PowerProfile),
		tariffs:    DefaultTariffs,
	}
}

// RegisterRoutes 注册路由
func (api *ElectricityAPI) RegisterRoutes(r *gin.RouterGroup) {
	elec := r.Group("/electricity")
	{
		// 电价配置
		elec.GET("/tariffs", api.ListTariffs)
		elec.GET("/tariffs/:id", api.GetTariff)
		elec.POST("/tariffs", api.CreateTariff)
		elec.PUT("/tariffs/:id", api.UpdateTariff)
		elec.DELETE("/tariffs/:id", api.DeleteTariff)

		// 设备功耗配置
		elec.GET("/profiles", api.ListProfiles)
		elec.GET("/profiles/:id", api.GetProfile)
		elec.POST("/profiles", api.CreateProfile)
		elec.PUT("/profiles/:id", api.UpdateProfile)
		elec.DELETE("/profiles/:id", api.DeleteProfile)

		// 电费计算
		elec.POST("/calculate", api.CalculateCost)
		elec.POST("/calculate/all", api.CalculateAllCosts)
		elec.POST("/estimate", api.EstimateCost)

		// 账单生成
		elec.POST("/bill", api.GenerateBill)

		// 电费预测
		elec.POST("/forecast", api.ForecastCost)

		// 用电记录
		elec.POST("/readings", api.AddReading)
		elec.GET("/readings", api.ListReadings)
	}
}

// ========== 电价配置 API ==========

// ListTariffsRequest 列出电价配置请求
type ListTariffsRequest struct {
	Region string `form:"region"` // 按地区筛选
}

// ListTariffs 列出电价配置
func (api *ElectricityAPI) ListTariffs(c *gin.Context) {
	var req ListTariffsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tariffs := make([]ElectricityTariff, 0)
	for _, t := range api.tariffs {
		if req.Region == "" || t.Region == req.Region {
			tariffs = append(tariffs, t)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"tariffs": tariffs,
		"total":   len(tariffs),
	})
}

// GetTariff 获取单个电价配置
func (api *ElectricityAPI) GetTariff(c *gin.Context) {
	id := c.Param("id")
	tariff, ok := api.tariffs[id]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "tariff not found"})
		return
	}

	c.JSON(http.StatusOK, tariff)
}

// CreateTariffRequest 创建电价配置请求
type CreateTariffRequest struct {
	Name           string          `json:"name" binding:"required"`
	Region         string          `json:"region" binding:"required"`
	VoltageLevel   string          `json:"voltage_level"`
	TimeSlots      []TimeSlot      `json:"time_slots" binding:"required"`
	SeasonalPrices []SeasonalPrice `json:"seasonal_prices" binding:"required"`
	EffectiveFrom  *time.Time      `json:"effective_from"`
}

// CreateTariff 创建电价配置
func (api *ElectricityAPI) CreateTariff(c *gin.Context) {
	var req CreateTariffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	effectiveFrom := now
	if req.EffectiveFrom != nil {
		effectiveFrom = *req.EffectiveFrom
	}

	tariff := ElectricityTariff{
		ID:             "tariff_" + now.Format("20060102150405"),
		Name:           req.Name,
		Region:         req.Region,
		VoltageLevel:   req.VoltageLevel,
		TimeSlots:      req.TimeSlots,
		SeasonalPrices: req.SeasonalPrices,
		EffectiveFrom:  effectiveFrom,
		CreatedAt:      now,
		UpdatedAt:      now,
	}

	api.tariffs[tariff.ID] = tariff

	c.JSON(http.StatusCreated, tariff)
}

// UpdateTariff 更新电价配置
func (api *ElectricityAPI) UpdateTariff(c *gin.Context) {
	id := c.Param("id")
	tariff, ok := api.tariffs[id]
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "tariff not found"})
		return
	}

	var req CreateTariffRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	tariff.Name = req.Name
	tariff.Region = req.Region
	tariff.VoltageLevel = req.VoltageLevel
	tariff.TimeSlots = req.TimeSlots
	tariff.SeasonalPrices = req.SeasonalPrices
	tariff.UpdatedAt = time.Now()

	api.tariffs[id] = tariff

	c.JSON(http.StatusOK, tariff)
}

// DeleteTariff 删除电价配置
func (api *ElectricityAPI) DeleteTariff(c *gin.Context) {
	id := c.Param("id")
	if _, ok := api.tariffs[id]; !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "tariff not found"})
		return
	}

	delete(api.tariffs, id)

	c.JSON(http.StatusOK, gin.H{"message": "tariff deleted"})
}

// ========== 设备功耗配置 API ==========

// ListProfiles 列出设备功耗配置
func (api *ElectricityAPI) ListProfiles(c *gin.Context) {
	profiles := api.calculator.ListPowerProfiles()

	c.JSON(http.StatusOK, gin.H{
		"profiles": profiles,
		"total":    len(profiles),
	})
}

// GetProfile 获取单个设备功耗配置
func (api *ElectricityAPI) GetProfile(c *gin.Context) {
	id := c.Param("id")
	profile, ok := api.calculator.GetPowerProfile(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}

	c.JSON(http.StatusOK, profile)
}

// CreateProfileRequest 创建设备功耗配置请求
type CreateProfileRequest struct {
	DeviceName        string     `json:"device_name" binding:"required"`
	DeviceType        DeviceType `json:"device_type"`
	IdlePowerWatts    float64    `json:"idle_power_watts"`
	MaxPowerWatts     float64    `json:"max_power_watts"`
	TypicalPowerWatts float64    `json:"typical_power_watts" binding:"required"`
	StandbyPowerWatts float64    `json:"standby_power_watts"`
	DailyOnHours      float64    `json:"daily_on_hours"`
	WeeklyOnDays      int        `json:"weekly_on_days"`
	Location          string     `json:"location"`
	Description       string     `json:"description"`
}

// CreateProfile 创建设备功耗配置
func (api *ElectricityAPI) CreateProfile(c *gin.Context) {
	var req CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	now := time.Now()
	profile := PowerProfile{
		ID:                "profile_" + now.Format("20060102150405"),
		DeviceName:        req.DeviceName,
		DeviceType:        req.DeviceType,
		IdlePowerWatts:    req.IdlePowerWatts,
		MaxPowerWatts:     req.MaxPowerWatts,
		TypicalPowerWatts: req.TypicalPowerWatts,
		StandbyPowerWatts: req.StandbyPowerWatts,
		DailyOnHours:      req.DailyOnHours,
		WeeklyOnDays:      req.WeeklyOnDays,
		Location:          req.Location,
		Description:       req.Description,
		CreatedAt:         now,
		UpdatedAt:         now,
	}

	if profile.DailyOnHours == 0 {
		profile.DailyOnHours = 24
	}
	if profile.WeeklyOnDays == 0 {
		profile.WeeklyOnDays = 7
	}

	api.calculator.AddPowerProfile(profile)

	c.JSON(http.StatusCreated, profile)
}

// UpdateProfile 更新设备功耗配置
func (api *ElectricityAPI) UpdateProfile(c *gin.Context) {
	id := c.Param("id")
	profile, ok := api.calculator.GetPowerProfile(id)
	if !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}

	var req CreateProfileRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	profile.DeviceName = req.DeviceName
	profile.DeviceType = req.DeviceType
	profile.IdlePowerWatts = req.IdlePowerWatts
	profile.MaxPowerWatts = req.MaxPowerWatts
	profile.TypicalPowerWatts = req.TypicalPowerWatts
	profile.StandbyPowerWatts = req.StandbyPowerWatts
	profile.DailyOnHours = req.DailyOnHours
	profile.WeeklyOnDays = req.WeeklyOnDays
	profile.Location = req.Location
	profile.Description = req.Description
	profile.UpdatedAt = time.Now()

	api.calculator.AddPowerProfile(profile)

	c.JSON(http.StatusOK, profile)
}

// DeleteProfile 删除设备功耗配置
func (api *ElectricityAPI) DeleteProfile(c *gin.Context) {
	id := c.Param("id")
	if _, ok := api.calculator.GetPowerProfile(id); !ok {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}

	api.calculator.RemovePowerProfile(id)

	c.JSON(http.StatusOK, gin.H{"message": "profile deleted"})
}

// ========== 电费计算 API ==========

// CalculateCostRequest 计算电费请求
type CalculateCostRequest struct {
	DeviceID string         `json:"device_id" binding:"required"`
	Readings []PowerReading `json:"readings"`
	Period   ReportPeriod   `json:"period"`
}

// CalculateCost 计算指定设备电费
func (api *ElectricityAPI) CalculateCost(c *gin.Context) {
	var req CalculateCostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Period.StartTime.IsZero() || req.Period.EndTime.IsZero() {
		req.Period = ReportPeriod{
			StartTime: time.Now().AddDate(0, 0, -30),
			EndTime:   time.Now(),
		}
	}

	result := api.calculator.Calculate(req.DeviceID, req.Readings, req.Period)

	c.JSON(http.StatusOK, result)
}

// CalculateAllCostsRequest 计算所有设备电费请求
type CalculateAllCostsRequest struct {
	Readings []PowerReading `json:"readings" binding:"required"`
	Period   ReportPeriod   `json:"period"`
}

// CalculateAllCosts 计算所有设备电费
func (api *ElectricityAPI) CalculateAllCosts(c *gin.Context) {
	var req CalculateAllCostsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Period.StartTime.IsZero() || req.Period.EndTime.IsZero() {
		req.Period = ReportPeriod{
			StartTime: time.Now().AddDate(0, 0, -30),
			EndTime:   time.Now(),
		}
	}

	results := api.calculator.CalculateAll(req.Readings, req.Period)

	c.JSON(http.StatusOK, gin.H{
		"results": results,
		"total":   len(results),
	})
}

// EstimateCostRequest 估算电费请求
type EstimateCostRequest struct {
	ProfileID string `json:"profile_id" binding:"required"`
	Days      int    `json:"days"`
}

// EstimateCost 从配置估算电费
func (api *ElectricityAPI) EstimateCost(c *gin.Context) {
	var req EstimateCostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Days == 0 {
		req.Days = 30
	}

	result := api.calculator.EstimateFromProfile(req.ProfileID, req.Days)
	if result == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "profile not found"})
		return
	}

	c.JSON(http.StatusOK, result)
}

// ========== 账单生成 API ==========

// GenerateBillRequest 生成账单请求
type GenerateBillRequest struct {
	CustomerName string                            `json:"customer_name" binding:"required"`
	Period       ReportPeriod                      `json:"period"`
	Results      map[string]*ElectricityCostResult `json:"results"`
}

// GenerateBill 生成电费账单
func (api *ElectricityAPI) GenerateBill(c *gin.Context) {
	var req GenerateBillRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Period.StartTime.IsZero() || req.Period.EndTime.IsZero() {
		req.Period = ReportPeriod{
			StartTime: time.Now().AddDate(0, -1, 0),
			EndTime:   time.Now(),
		}
	}

	bill := api.calculator.GenerateBill(req.Results, req.Period, req.CustomerName)

	c.JSON(http.StatusOK, bill)
}

// ========== 电费预测 API ==========

// ForecastCostRequest 预测电费请求
type ForecastCostRequest struct {
	Historical []ElectricityTrendPoint `json:"historical" binding:"required"`
	Months     int                     `json:"months"`
}

// ForecastCost 预测未来电费
func (api *ElectricityAPI) ForecastCost(c *gin.Context) {
	var req ForecastCostRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Months == 0 {
		req.Months = 3
	}

	if len(req.Historical) < 2 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "at least 2 historical data points required"})
		return
	}

	forecast := api.calculator.Forecast(req.Historical, req.Months)
	if forecast == nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "forecast failed"})
		return
	}

	c.JSON(http.StatusOK, forecast)
}

// ========== 用电记录 API ==========

// readingStore 用电记录存储 (简化实现)
var readingStore = make(map[string][]PowerReading)

// AddReading 添加用电读数
func (api *ElectricityAPI) AddReading(c *gin.Context) {
	var reading PowerReading
	if err := c.ShouldBindJSON(&reading); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if reading.Timestamp.IsZero() {
		reading.Timestamp = time.Now()
	}

	readingStore[reading.DeviceID] = append(readingStore[reading.DeviceID], reading)

	c.JSON(http.StatusCreated, reading)
}

// ListReadingsRequest 列出用电记录请求
type ListReadingsRequest struct {
	DeviceID  string    `form:"device_id"`
	StartTime time.Time `form:"start_time" time_format:"2006-01-02T15:04:05Z07:00"`
	EndTime   time.Time `form:"end_time" time_format:"2006-01-02T15:04:05Z07:00"`
	Limit     int       `form:"limit"`
}

// ListReadings 列出用电记录
func (api *ElectricityAPI) ListReadings(c *gin.Context) {
	var req ListReadingsRequest
	if err := c.ShouldBindQuery(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if req.Limit == 0 {
		req.Limit = 100
	}

	readings := make([]PowerReading, 0)

	if req.DeviceID != "" {
		// 获取指定设备的记录
		deviceReadings := readingStore[req.DeviceID]
		for _, r := range deviceReadings {
			if !req.StartTime.IsZero() && r.Timestamp.Before(req.StartTime) {
				continue
			}
			if !req.EndTime.IsZero() && r.Timestamp.After(req.EndTime) {
				continue
			}
			readings = append(readings, r)
			if len(readings) >= req.Limit {
				break
			}
		}
	} else {
		// 获取所有设备的记录
		for _, deviceReadings := range readingStore {
			for _, r := range deviceReadings {
				if !req.StartTime.IsZero() && r.Timestamp.Before(req.StartTime) {
					continue
				}
				if !req.EndTime.IsZero() && r.Timestamp.After(req.EndTime) {
					continue
				}
				readings = append(readings, r)
				if len(readings) >= req.Limit {
					break
				}
			}
			if len(readings) >= req.Limit {
				break
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"readings": readings,
		"total":    len(readings),
	})
}

// ========== API 工厂函数 ==========

// DefaultElectricityConfig 默认电费计算配置
func DefaultElectricityConfig() ElectricityCostConfig {
	return ElectricityCostConfig{
		Region:             "北京",
		TariffID:           "beijing_general",
		Currency:           "CNY",
		TaxRate:            0.13,
		AddOnCharges:       0.05,
		DiscountRate:       0.0,
		PowerFactorPenalty: 0.0,
	}
}
