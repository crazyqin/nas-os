// smartthermal2.go - 智能温控系统 v2
// AI自适应散热、噪音优化、温度预测、多区域温控
package smartthermal2

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ==================== ThermalEngine 温控引擎 ====================

// ThermalEngine 温控引擎核心.
type ThermalEngine struct {
	logger     *zap.Logger
	mu         sync.RWMutex
	sensors    map[string]*Sensor
	zones      map[string]*ThermalZone
	sensorHist map[string][]SensorHistory
	maxHistory int
	settings   GlobalSettings
}

// NewThermalEngine 创建温控引擎.
func NewThermalEngine(logger *zap.Logger) *ThermalEngine {
	e := &ThermalEngine{
		logger:     logger,
		sensors:    make(map[string]*Sensor),
		zones:      make(map[string]*ThermalZone),
		sensorHist: make(map[string][]SensorHistory),
		maxHistory: 1440,
		settings: GlobalSettings{
			PollIntervalSec: 10,
			WindowSize:      10,
			AdaptiveEnabled: true,
			NoiseSettings: NoiseSettings{
				MaxDBA:          40,
				ScheduleEnabled: true,
				DayStartHour:    8,
				DayEndHour:      22,
				DayMaxDBA:       45,
				NightMaxDBA:     30,
			},
			AlertSettings: AlertSettings{
				WarningTemp:   70,
				CriticalTemp:  80,
				EmergencyTemp: 90,
				AutoProtect:   true,
			},
		},
	}
	e.initMockData()
	return e
}

// initMockData 初始化模拟数据.
func (e *ThermalEngine) initMockData() {
	now := time.Now()
	sensorDefs := []struct {
		id, name string
		stype    SensorType
		zone     string
		temp     float64
	}{
		{"cpu-0", "CPU Package", SensorCPU, "cpu-zone", 42.5},
		{"cpu-core0", "CPU Core 0", SensorCPU, "cpu-zone", 40.0},
		{"cpu-core1", "CPU Core 1", SensorCPU, "cpu-zone", 41.0},
		{"gpu-0", "GPU", SensorGPU, "cpu-zone", 38.0},
		{"hdd-0", "HDD Bay 1", SensorHDD, "hdd-zone", 35.0},
		{"hdd-1", "HDD Bay 2", SensorHDD, "hdd-zone", 36.5},
		{"hdd-2", "HDD Bay 3", SensorHDD, "hdd-zone", 34.0},
		{"ssd-0", "SSD", SensorSSD, "hdd-zone", 32.0},
		{"nvme-0", "NVMe", SensorNVMe, "hdd-zone", 39.0},
		{"mb-0", "主板芯片", SensorMother, "rear-zone", 37.0},
		{"ambient-0", "环境温度", SensorAmbient, "front-zone", 26.0},
	}
	for _, s := range sensorDefs {
		e.sensors[s.id] = &Sensor{
			ID: s.id, Name: s.name, Type: s.stype, Temp: s.temp,
			MaxTemp: s.temp, MinTemp: s.temp, AvgTemp: s.temp,
			Zone: s.zone, Status: SensorNormal, UpdatedAt: now,
		}
	}
	zoneDefs := []struct {
		id, name, desc string
		sensorIDs      []string
	}{
		{"front-zone", "机箱前部", "硬盘笼和进风区域", []string{"ambient-0"}},
		{"rear-zone", "机箱后部", "IO挡板和电源区域", []string{"mb-0"}},
		{"cpu-zone", "CPU区域", "处理器和显卡散热区", []string{"cpu-0", "cpu-core0", "cpu-core1", "gpu-0"}},
		{"hdd-zone", "硬盘区域", "硬盘笼散热区域", []string{"hdd-0", "hdd-1", "hdd-2", "ssd-0", "nvme-0"}},
		{"psu-zone", "电源区域", "电源散热区域", []string{"mb-0"}},
	}
	for _, z := range zoneDefs {
		maxT, avgT := e.calcZoneTemps(z.sensorIDs)
		e.zones[z.id] = &ThermalZone{
			ID: z.id, Name: z.name, Description: z.desc,
			SensorIDs: z.sensorIDs, MaxTemp: maxT, AvgTemp: avgT,
			Status: SensorNormal, UpdatedAt: now,
		}
	}
}

// calcZoneTemps 计算区域温度.
func (e *ThermalEngine) calcZoneTemps(sensorIDs []string) (maxT, avgT float64) {
	var total float64
	var count int
	for _, sid := range sensorIDs {
		if s, ok := e.sensors[sid]; ok {
			total += s.Temp
			count++
			if s.Temp > maxT {
				maxT = s.Temp
			}
		}
	}
	if count > 0 {
		avgT = total / float64(count)
	}
	return
}

// Sample 采样温度.
func (e *ThermalEngine) Sample() {
	e.mu.Lock()
	defer e.mu.Unlock()
	now := time.Now()
	for _, s := range e.sensors {
		e.sensorHist[s.ID] = append(e.sensorHist[s.ID], SensorHistory{Timestamp: now, Temp: s.Temp})
		if len(e.sensorHist[s.ID]) > e.maxHistory {
			e.sensorHist[s.ID] = e.sensorHist[s.ID][len(e.sensorHist[s.ID])-e.maxHistory:]
		}
		window := e.settings.WindowSize
		hist := e.sensorHist[s.ID]
		if len(hist) > 0 {
			start := len(hist) - window
			if start < 0 {
				start = 0
			}
			var sum float64
			for _, h := range hist[start:] {
				sum += h.Temp
			}
			s.AvgTemp = sum / float64(len(hist[start:]))
		}
		if s.Temp > s.MaxTemp {
			s.MaxTemp = s.Temp
		}
		if s.Temp < s.MinTemp || s.MinTemp == 0 {
			s.MinTemp = s.Temp
		}
		s.Status = e.classifySensorStatus(s.Temp)
		s.UpdatedAt = now
	}
	for _, z := range e.zones {
		maxT, avgT := e.calcZoneTemps(z.SensorIDs)
		z.MaxTemp, z.AvgTemp = maxT, avgT
		z.Status = e.classifySensorStatus(maxT)
		z.UpdatedAt = now
	}
}

// classifySensorStatus 分类传感器状态.
func (e *ThermalEngine) classifySensorStatus(temp float64) SensorStatus {
	switch {
	case temp >= e.settings.AlertSettings.EmergencyTemp:
		return SensorEmergency
	case temp >= e.settings.AlertSettings.CriticalTemp:
		return SensorCritical
	case temp >= e.settings.AlertSettings.WarningTemp:
		return SensorWarning
	default:
		return SensorNormal
	}
}

// GetSensors 获取所有传感器.
func (e *ThermalEngine) GetSensors() []Sensor {
	e.mu.RLock()
	defer e.mu.RUnlock()
	sensors := make([]Sensor, 0, len(e.sensors))
	for _, s := range e.sensors {
		sensors = append(sensors, *s)
	}
	sort.Slice(sensors, func(i, j int) bool { return sensors[i].ID < sensors[j].ID })
	return sensors
}

// GetSensor 获取单个传感器.
func (e *ThermalEngine) GetSensor(id string) (*Sensor, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	s, ok := e.sensors[id]
	if !ok {
		return nil, false
	}
	result := *s
	return &result, true
}

// GetSensorHistory 获取传感器历史.
func (e *ThermalEngine) GetSensorHistory(id string, minutes int) []SensorHistory {
	e.mu.RLock()
	defer e.mu.RUnlock()
	if minutes <= 0 {
		minutes = 60
	}
	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)
	var result []SensorHistory
	for _, h := range e.sensorHist[id] {
		if h.Timestamp.After(cutoff) {
			result = append(result, h)
		}
	}
	return result
}

// GetZones 获取所有温控区域.
func (e *ThermalEngine) GetZones() []ThermalZone {
	e.mu.RLock()
	defer e.mu.RUnlock()
	zones := make([]ThermalZone, 0, len(e.zones))
	for _, z := range e.zones {
		zones = append(zones, *z)
	}
	sort.Slice(zones, func(i, j int) bool { return zones[i].ID < zones[j].ID })
	return zones
}

// UpdateSensorTemp 更新传感器温度.
func (e *ThermalEngine) UpdateSensorTemp(id string, temp float64) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if s, ok := e.sensors[id]; ok {
		s.Temp = temp
		s.Status = e.classifySensorStatus(temp)
	}
}

// GetSettings 获取全局设置.
func (e *ThermalEngine) GetSettings() GlobalSettings {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.settings
}

// UpdateSettings 更新全局设置.
func (e *ThermalEngine) UpdateSettings(s GlobalSettings) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.settings = s
	e.logger.Info("更新全局设置", zap.Int("pollSec", s.PollIntervalSec))
}

// ==================== FanController 风扇控制器 ====================

// FanController 风扇控制器.
type FanController struct {
	logger         *zap.Logger
	mu             sync.RWMutex
	engine         *ThermalEngine
	fans           map[string]*FanInfo
	curves         map[FanProfileType]*FanCurve
	adaptiveEWMA   map[string]*EWMAData
	transitionRate float64
}

// NewFanController 创建风扇控制器.
func NewFanController(logger *zap.Logger, engine *ThermalEngine) *FanController {
	fc := &FanController{
		logger:         logger,
		engine:         engine,
		fans:           make(map[string]*FanInfo),
		curves:         make(map[FanProfileType]*FanCurve),
		adaptiveEWMA:   make(map[string]*EWMAData),
		transitionRate: 5.0,
	}
	fc.initDefaultCurves()
	fc.initMockFans()
	return fc
}

// initDefaultCurves 初始化默认风扇曲线.
func (fc *FanController) initDefaultCurves() {
	fc.curves[FanProfileSilent] = &FanCurve{
		Type: FanProfileSilent,
		Points: []FanCurvePoint{
			{Temp: 25, PWM: 10}, {Temp: 40, PWM: 20}, {Temp: 55, PWM: 35},
			{Temp: 70, PWM: 55}, {Temp: 80, PWM: 80}, {Temp: 90, PWM: 100},
		},
	}
	fc.curves[FanProfileStandard] = &FanCurve{
		Type: FanProfileStandard,
		Points: []FanCurvePoint{
			{Temp: 25, PWM: 20}, {Temp: 40, PWM: 35}, {Temp: 55, PWM: 50},
			{Temp: 65, PWM: 65}, {Temp: 75, PWM: 80}, {Temp: 85, PWM: 100},
		},
	}
	fc.curves[FanProfilePerformance] = &FanCurve{
		Type: FanProfilePerformance,
		Points: []FanCurvePoint{
			{Temp: 25, PWM: 30}, {Temp: 40, PWM: 50}, {Temp: 55, PWM: 70},
			{Temp: 65, PWM: 85}, {Temp: 75, PWM: 95}, {Temp: 85, PWM: 100},
		},
	}
	fc.curves[FanProfileFullSpeed] = &FanCurve{
		Type:   FanProfileFullSpeed,
		Points: []FanCurvePoint{{Temp: 0, PWM: 100}, {Temp: 100, PWM: 100}},
	}
}

// initMockFans 初始化模拟风扇.
func (fc *FanController) initMockFans() {
	fanDefs := []struct {
		id, name, zone string
		maxRPM, minRPM int
		profile        FanProfileType
	}{
		{"fan-cpu", "CPU散热风扇", "cpu-zone", 3000, 300, FanProfileStandard},
		{"fan-front", "前置进风扇", "front-zone", 2000, 200, FanProfileStandard},
		{"fan-rear", "后排风扇", "rear-zone", 2000, 200, FanProfileStandard},
		{"fan-hdd", "硬盘散热风扇", "hdd-zone", 2500, 250, FanProfileStandard},
	}
	for _, f := range fanDefs {
		fc.fans[f.id] = &FanInfo{
			ID: f.id, Name: f.name, Zone: f.zone, PWM: 30, TargetPWM: 30,
			RPM: int(float64(f.maxRPM) * 0.3), MaxRPM: f.maxRPM, MinRPM: f.minRPM,
			Profile: f.profile, Status: FanStatusNormal, UpdatedAt: time.Now(),
		}
	}
}

// InterpolateCurve 分段线性插值计算风扇转速.
func (fc *FanController) InterpolateCurve(temp float64, curve *FanCurve) float64 {
	if curve == nil || len(curve.Points) == 0 {
		return 30
	}
	points := curve.Points
	if temp <= points[0].Temp {
		return points[0].PWM
	}
	if temp >= points[len(points)-1].Temp {
		return points[len(points)-1].PWM
	}
	for i := 1; i < len(points); i++ {
		if temp <= points[i].Temp {
			t1, t2 := points[i-1].Temp, points[i].Temp
			p1, p2 := points[i-1].PWM, points[i].PWM
			return p1 + (temp-t1)/(t2-t1)*(p2-p1)
		}
	}
	return points[len(points)-1].PWM
}

// UpdateFans 更新所有风扇转速.
func (fc *FanController) UpdateFans() {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	for _, fan := range fc.fans {
		if fan.Status == FanStatusFailed || fan.Status == FanStatusDisabled {
			continue
		}
		maxTemp := fc.getZoneMaxTemp(fan.Zone)
		var targetPWM float64
		if fan.Profile == FanProfileAdaptive {
			targetPWM = fc.adaptiveTarget(fan.ID, maxTemp)
		} else {
			curve := fc.curves[fan.Profile]
			if curve == nil {
				curve = fc.curves[FanProfileStandard]
			}
			targetPWM = fc.InterpolateCurve(maxTemp, curve)
		}
		fan.TargetPWM = targetPWM
		fan.PWM = fc.smoothTransition(fan.PWM, targetPWM)
		fan.RPM = int(float64(fan.MaxRPM) * fan.PWM / 100)
		if fan.RPM < fan.MinRPM && fan.PWM > 0 {
			fan.RPM = fan.MinRPM
		}
		fan.NoiseLevel = fc.estimateFanNoise(fan.RPM, fan.MaxRPM)
		fan.UpdatedAt = time.Now()
	}
}

// smoothTransition 平滑过渡.
func (fc *FanController) smoothTransition(current, target float64) float64 {
	diff := target - current
	if math.Abs(diff) <= fc.transitionRate {
		return target
	}
	if diff > 0 {
		return current + fc.transitionRate
	}
	return current - fc.transitionRate
}

// getZoneMaxTemp 获取区域最高温度.
func (fc *FanController) getZoneMaxTemp(zoneID string) float64 {
	fc.engine.mu.RLock()
	defer fc.engine.mu.RUnlock()
	if z, ok := fc.engine.zones[zoneID]; ok {
		return z.MaxTemp
	}
	if s, ok := fc.engine.sensors["cpu-0"]; ok {
		return s.Temp
	}
	return 40
}

// adaptiveTarget AI自适应目标（EWMA）.
func (fc *FanController) adaptiveTarget(fanID string, temp float64) float64 {
	standard := fc.curves[FanProfileStandard]
	idealPWM := fc.InterpolateCurve(temp, standard)
	ewma, ok := fc.adaptiveEWMA[fanID]
	if !ok {
		fc.adaptiveEWMA[fanID] = &EWMAData{Value: idealPWM, Alpha: 0.3}
		return idealPWM
	}
	ewma.Value = ewma.Alpha*idealPWM + (1-ewma.Alpha)*ewma.Value
	return ewma.Value
}

// estimateFanNoise 估算风扇噪音（基于常见风扇噪音曲线）.
func (fc *FanController) estimateFanNoise(rpm, maxRPM int) float64 {
	if rpm <= 0 {
		return 0
	}
	ratio := float64(rpm) / float64(maxRPM)
	noise := 18.0 + 30.0*math.Log10(ratio+0.1)
	if noise < 0 {
		noise = 0
	}
	return math.Round(noise*10) / 10
}

// GetFans 获取所有风扇.
func (fc *FanController) GetFans() []FanInfo {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	fans := make([]FanInfo, 0, len(fc.fans))
	for _, f := range fc.fans {
		fans = append(fans, *f)
	}
	sort.Slice(fans, func(i, j int) bool { return fans[i].ID < fans[j].ID })
	return fans
}

// GetFan 获取单个风扇.
func (fc *FanController) GetFan(id string) (*FanInfo, bool) {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	f, ok := fc.fans[id]
	if !ok {
		return nil, false
	}
	result := *f
	return &result, true
}

// UpdateFan 更新风扇设置.
func (fc *FanController) UpdateFan(id string, req FanUpdateRequest) error {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	fan, ok := fc.fans[id]
	if !ok {
		return fmt.Errorf("风扇 %s 未找到", id)
	}
	if req.PWM != nil {
		if *req.PWM < 0 || *req.PWM > 100 {
			return fmt.Errorf("PWM 必须在 0-100 之间")
		}
		fan.PWM = *req.PWM
		fan.RPM = int(float64(fan.MaxRPM) * *req.PWM / 100)
	}
	if req.Profile != nil {
		fan.Profile = *req.Profile
	}
	fan.NoiseLevel = fc.estimateFanNoise(fan.RPM, fan.MaxRPM)
	fan.UpdatedAt = time.Now()
	return nil
}

// CheckFanHealth 风扇故障检测.
func (fc *FanController) CheckFanHealth() []string {
	fc.mu.Lock()
	defer fc.mu.Unlock()
	var issues []string
	for _, fan := range fc.fans {
		if fan.Status == FanStatusDisabled {
			continue
		}
		if fan.PWM > 10 && fan.RPM == 0 {
			fan.Status = FanStatusFailed
			issues = append(issues, fmt.Sprintf("风扇 %s 故障：PWM=%.0f%% 但转速为0", fan.Name, fan.PWM))
		}
		expectedRPM := int(float64(fan.MaxRPM) * fan.PWM / 100)
		if fan.PWM > 20 && fan.RPM < expectedRPM/2 && fan.RPM > 0 {
			fan.Status = FanStatusWarning
			issues = append(issues, fmt.Sprintf("风扇 %s 转速异常：期望%dRPM，实际%dRPM", fan.Name, expectedRPM, fan.RPM))
		}
	}
	return issues
}

// GetCurves 获取所有风扇曲线.
func (fc *FanController) GetCurves() map[FanProfileType]*FanCurve {
	fc.mu.RLock()
	defer fc.mu.RUnlock()
	result := make(map[FanProfileType]*FanCurve)
	for k, v := range fc.curves {
		result[k] = v
	}
	return result
}

// ==================== NoiseOptimizer 噪音优化器 ====================

// NoiseOptimizer 噪音优化器.
type NoiseOptimizer struct {
	logger   *zap.Logger
	mu       sync.RWMutex
	fc       *FanController
	settings NoiseSettings
}

// NewNoiseOptimizer 创建噪音优化器.
func NewNoiseOptimizer(logger *zap.Logger, fc *FanController) *NoiseOptimizer {
	return &NoiseOptimizer{
		logger: logger, fc: fc,
		settings: NoiseSettings{
			MaxDBA: 40, ScheduleEnabled: true,
			DayStartHour: 8, DayEndHour: 22, DayMaxDBA: 45, NightMaxDBA: 30,
		},
	}
}

// Assess 评估噪音.
func (no *NoiseOptimizer) Assess() NoiseAssessment {
	no.mu.RLock()
	defer no.mu.RUnlock()
	fans := no.fc.GetFans()
	var totalPower float64
	var perFan []FanNoise
	for _, f := range fans {
		if f.Status == FanStatusDisabled {
			continue
		}
		perFan = append(perFan, FanNoise{FanID: f.ID, Name: f.Name, DBA: f.NoiseLevel, RPM: f.RPM})
		totalPower += math.Pow(10, f.NoiseLevel/10.0)
	}
	totalDBA := 0.0
	if totalPower > 0 {
		totalDBA = 10.0 * math.Log10(totalPower)
	}
	totalDBA = math.Round(totalDBA*10) / 10
	level := no.classifyNoise(totalDBA)
	budget := no.getCurrentBudget()
	budgetUsed := 0.0
	if budget > 0 {
		budgetUsed = math.Round(totalDBA/budget*10000) / 100
	}
	recommendation := no.generateRecommendation(totalDBA, budget, fans)
	return NoiseAssessment{
		TotalDBA: totalDBA, Level: level, NoiseBudget: budget,
		BudgetUsed: budgetUsed, PerFanNoise: perFan, Recommendation: recommendation,
	}
}

// classifyNoise 分类噪音级别.
func (no *NoiseOptimizer) classifyNoise(dba float64) NoiseLevel {
	switch {
	case dba < 25:
		return NoiseSilent
	case dba < 35:
		return NoiseQuiet
	case dba < 45:
		return NoiseModerate
	case dba < 55:
		return NoiseLoud
	default:
		return NoiseVeryLoud
	}
}

// getCurrentBudget 获取当前噪音预算.
func (no *NoiseOptimizer) getCurrentBudget() float64 {
	if !no.settings.ScheduleEnabled {
		return no.settings.MaxDBA
	}
	hour := time.Now().Hour()
	if hour >= no.settings.DayStartHour && hour < no.settings.DayEndHour {
		return no.settings.DayMaxDBA
	}
	return no.settings.NightMaxDBA
}

// generateRecommendation 生成建议.
func (no *NoiseOptimizer) generateRecommendation(current, budget float64, fans []FanInfo) string {
	if current <= budget {
		return "当前噪音在预算范围内，散热状态良好"
	}
	overBudget := current - budget
	if overBudget < 5 {
		return fmt.Sprintf("噪音超预算 %.1fdBA，建议将风扇曲线切换为静音模式", overBudget)
	}
	var loudest FanInfo
	for _, f := range fans {
		if f.NoiseLevel > loudest.NoiseLevel {
			loudest = f
		}
	}
	return fmt.Sprintf("噪音超预算 %.1fdBA，最大噪音源 %s，建议降低其转速或切换静音方案", overBudget, loudest.Name)
}

// UpdateSettings 更新噪音设置.
func (no *NoiseOptimizer) UpdateSettings(s NoiseSettings) {
	no.mu.Lock()
	defer no.mu.Unlock()
	no.settings = s
}

// GetSettings 获取噪音设置.
func (no *NoiseOptimizer) GetSettings() NoiseSettings {
	no.mu.RLock()
	defer no.mu.RUnlock()
	return no.settings
}

// ==================== ThermalPredictor 温度预测器 ====================

// ThermalPredictor 温度预测器.
type ThermalPredictor struct {
	logger       *zap.Logger
	mu           sync.RWMutex
	engine       *ThermalEngine
	safeMargin   float64
	seasonalComp []SeasonalCompensation
}

// NewThermalPredictor 创建温度预测器.
func NewThermalPredictor(logger *zap.Logger, engine *ThermalEngine) *ThermalPredictor {
	return &ThermalPredictor{
		logger: logger, engine: engine, safeMargin: 1.1,
		seasonalComp: []SeasonalCompensation{
			{Month: 1, Compensation: -2.0}, {Month: 2, Compensation: -1.5},
			{Month: 3, Compensation: 0}, {Month: 4, Compensation: 1.0},
			{Month: 5, Compensation: 2.0}, {Month: 6, Compensation: 3.0},
			{Month: 7, Compensation: 4.0}, {Month: 8, Compensation: 4.0},
			{Month: 9, Compensation: 2.5}, {Month: 10, Compensation: 1.0},
			{Month: 11, Compensation: 0}, {Month: 12, Compensation: -1.0},
		},
	}
}

// Predict 预测温度（线性外推 + 安全系数 + 季节补偿）.
func (tp *ThermalPredictor) Predict(sensorID string, futureMinutes int) (*PredictionResult, error) {
	tp.mu.RLock()
	defer tp.mu.RUnlock()
	engine := tp.engine
	engine.mu.RLock()
	defer engine.mu.RUnlock()
	sensor, ok := engine.sensors[sensorID]
	if !ok {
		return nil, fmt.Errorf("传感器 %s 未找到", sensorID)
	}
	hist := engine.sensorHist[sensorID]
	if len(hist) < 2 {
		return &PredictionResult{
			SensorID: sensorID, CurrentTemp: sensor.Temp, PredictedTemp: sensor.Temp,
			PredictionMins: futureMinutes, Trend: "stable", RatePerMin: 0,
			Confidence: 0.5, SafeMargin: tp.safeMargin, WillOverheat: false,
		}, nil
	}
	n := 10
	if len(hist) < n {
		n = len(hist)
	}
	recent := hist[len(hist)-n:]
	var sumX, sumY, sumXY, sumX2 float64
	for i, h := range recent {
		x, y := float64(i), h.Temp
		sumX += x
		sumY += y
		sumXY += x * y
		sumX2 += x * x
	}
	nn := float64(n)
	denom := nn*sumX2 - sumX*sumX
	ratePerMin := 0.0
	if denom != 0 {
		ratePerMin = (nn*sumXY - sumX*sumY) / denom
	}
	predicted := sensor.Temp + ratePerMin*float64(futureMinutes)
	month := int(time.Now().Month())
	for _, s := range tp.seasonalComp {
		if s.Month == month {
			predicted += s.Compensation
			break
		}
	}
	predictedWithMargin := predicted * tp.safeMargin
	trend := "stable"
	if ratePerMin > 0.5 {
		trend = "rising"
	} else if ratePerMin < -0.5 {
		trend = "falling"
	}
	confidence := math.Min(nn/10.0, 1.0) * 0.8
	if n > 3 {
		var variance float64
		mean := sumY / nn
		for _, h := range recent {
			d := h.Temp - mean
			variance += d * d
		}
		variance /= nn
		if variance < 4 {
			confidence += 0.2
		}
	}
	confidence = math.Min(confidence, 1.0)
	warningTemp := engine.settings.AlertSettings.WarningTemp
	willOverheat := predictedWithMargin > warningTemp
	minutesToOverheat := 0
	if willOverheat && ratePerMin > 0 {
		minutesToOverheat = int((warningTemp - sensor.Temp) / ratePerMin)
		if minutesToOverheat < 0 {
			minutesToOverheat = 0
		}
	}
	return &PredictionResult{
		SensorID: sensorID, CurrentTemp: sensor.Temp, PredictedTemp: predictedWithMargin,
		PredictionMins: futureMinutes, Trend: trend, RatePerMin: ratePerMin,
		Confidence: confidence, SafeMargin: tp.safeMargin,
		WillOverheat: willOverheat, MinutesToOverheat: minutesToOverheat,
	}, nil
}

// PredictAll 预测所有传感器.
func (tp *ThermalPredictor) PredictAll(futureMinutes int) []PredictionResult {
	tp.mu.RLock()
	sensorIDs := make([]string, 0, len(tp.engine.sensors))
	for id := range tp.engine.sensors {
		sensorIDs = append(sensorIDs, id)
	}
	tp.mu.RUnlock()
	var results []PredictionResult
	for _, id := range sensorIDs {
		r, err := tp.Predict(id, futureMinutes)
		if err == nil {
			results = append(results, *r)
		}
	}
	return results
}

// ==================== CoolingProfile 散热方案管理 ====================

// ProfileManager 散热方案管理器.
type ProfileManager struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	profiles map[string]*CoolingProfile
	activeID string
}

// NewProfileManager 创建散热方案管理器.
func NewProfileManager(logger *zap.Logger) *ProfileManager {
	pm := &ProfileManager{
		logger:   logger,
		profiles: make(map[string]*CoolingProfile),
	}
	pm.initPresets()
	return pm
}

// initPresets 初始化预设方案.
func (pm *ProfileManager) initPresets() {
	now := time.Now()
	presets := []CoolingProfile{
		{
			ID: "bedroom", Name: "卧室NAS", Description: "极致静音，适合卧室环境",
			Scenario: "bedroom", IsDefault: true, IsActive: false, IsCustom: false, CreatedAt: now,
			NoiseLimit: 25, MaxTemp: 80,
			FanCurve: FanCurve{Type: FanProfileSilent, Points: []FanCurvePoint{
				{Temp: 20, PWM: 8}, {Temp: 35, PWM: 15}, {Temp: 50, PWM: 25},
				{Temp: 65, PWM: 45}, {Temp: 80, PWM: 75}, {Temp: 90, PWM: 100},
			}},
		},
		{
			ID: "study", Name: "书房NAS", Description: "平衡静音和散热，适合书房使用",
			Scenario: "study", IsDefault: false, IsActive: true, IsCustom: false, CreatedAt: now,
			NoiseLimit: 35, MaxTemp: 75,
			FanCurve: FanCurve{Type: FanProfileStandard, Points: []FanCurvePoint{
				{Temp: 25, PWM: 20}, {Temp: 40, PWM: 35}, {Temp: 55, PWM: 50},
				{Temp: 65, PWM: 65}, {Temp: 75, PWM: 80}, {Temp: 85, PWM: 100},
			}},
		},
		{
			ID: "serverroom", Name: "机房NAS", Description: "高性能散热，噪音无限制",
			Scenario: "serverroom", IsDefault: false, IsActive: false, IsCustom: false, CreatedAt: now,
			NoiseLimit: 60, MaxTemp: 70,
			FanCurve: FanCurve{Type: FanProfilePerformance, Points: []FanCurvePoint{
				{Temp: 25, PWM: 40}, {Temp: 40, PWM: 55}, {Temp: 55, PWM: 75},
				{Temp: 65, PWM: 90}, {Temp: 75, PWM: 100},
			}},
		},
		{
			ID: "livingroom", Name: "客厅NAS", Description: "兼顾静音和性能，适合客厅",
			Scenario: "livingroom", IsDefault: false, IsActive: false, IsCustom: false, CreatedAt: now,
			NoiseLimit: 38, MaxTemp: 75,
			FanCurve: FanCurve{Type: FanProfileStandard, Points: []FanCurvePoint{
				{Temp: 25, PWM: 18}, {Temp: 40, PWM: 30}, {Temp: 55, PWM: 48},
				{Temp: 65, PWM: 62}, {Temp: 75, PWM: 80}, {Temp: 85, PWM: 100},
			}},
		},
	}
	for i := range presets {
		pm.profiles[presets[i].ID] = &presets[i]
		if presets[i].IsActive {
			pm.activeID = presets[i].ID
		}
	}
}

// List 列出所有方案.
func (pm *ProfileManager) List() []CoolingProfile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	list := make([]CoolingProfile, 0, len(pm.profiles))
	for _, p := range pm.profiles {
		list = append(list, *p)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].ID < list[j].ID })
	return list
}

// Get 获取方案.
func (pm *ProfileManager) Get(id string) (*CoolingProfile, bool) {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.profiles[id]
	if !ok {
		return nil, false
	}
	result := *p
	return &result, true
}

// GetActive 获取当前激活方案.
func (pm *ProfileManager) GetActive() *CoolingProfile {
	pm.mu.RLock()
	defer pm.mu.RUnlock()
	p, ok := pm.profiles[pm.activeID]
	if !ok {
		return nil
	}
	result := *p
	return &result
}

// Create 创建自定义方案.
func (pm *ProfileManager) Create(req ProfileCreateRequest) (*CoolingProfile, error) {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	id := fmt.Sprintf("custom-%d", time.Now().UnixNano())
	profile := &CoolingProfile{
		ID: id, Name: req.Name, Description: req.Description,
		Scenario: req.Scenario, FanCurve: req.FanCurve,
		NoiseLimit: req.NoiseLimit, MaxTemp: req.MaxTemp,
		IsDefault: false, IsActive: false, IsCustom: true, CreatedAt: time.Now(),
	}
	if req.Schedule != nil {
		profile.Schedule = req.Schedule
	}
	pm.profiles[id] = profile
	pm.logger.Info("创建散热方案", zap.String("id", id), zap.String("name", req.Name))
	return profile, nil
}

// SetActive 切换活跃方案.
func (pm *ProfileManager) SetActive(id string) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	if _, ok := pm.profiles[id]; !ok {
		return fmt.Errorf("方案 %s 未找到", id)
	}
	if old, ok := pm.profiles[pm.activeID]; ok {
		old.IsActive = false
	}
	pm.profiles[id].IsActive = true
	pm.activeID = id
	pm.logger.Info("切换散热方案", zap.String("id", id))
	return nil
}

// ==================== AlertManager 告警管理 ====================

// AlertManager 告警管理器.
type AlertManager struct {
	mu       sync.RWMutex
	logger   *zap.Logger
	engine   *ThermalEngine
	fc       *FanController
	alerts   map[string]*ThermalAlert
	settings AlertSettings
	alertSeq int64
}

// NewAlertManager 创建告警管理器.
func NewAlertManager(logger *zap.Logger, engine *ThermalEngine, fc *FanController) *AlertManager {
	return &AlertManager{
		logger: logger, engine: engine, fc: fc,
		alerts: make(map[string]*ThermalAlert),
		settings: AlertSettings{
			WarningTemp: 70, CriticalTemp: 80, EmergencyTemp: 90, AutoProtect: true,
		},
	}
}

// Check 检查并生成告警.
func (am *AlertManager) Check() {
	am.mu.Lock()
	defer am.mu.Unlock()
	sensors := am.engine.GetSensors()
	for _, s := range sensors {
		var level AlertLevel
		var threshold float64
		switch {
		case s.Temp >= am.settings.EmergencyTemp:
			level = AlertEmergency
			threshold = am.settings.EmergencyTemp
		case s.Temp >= am.settings.CriticalTemp:
			level = AlertCritical
			threshold = am.settings.CriticalTemp
		case s.Temp >= am.settings.WarningTemp:
			level = AlertWarning
			threshold = am.settings.WarningTemp
		default:
			continue
		}
		am.alertSeq++
		alertID := fmt.Sprintf("alert-%d", am.alertSeq)
		msg := fmt.Sprintf("%s 温度 %.1f°C 超过 %s 阈值 %.0f°C", s.Name, s.Temp, level, threshold)
		var actions []string
		if am.settings.AutoProtect {
			actions = am.executeProtection(level, s.ID)
		}
		am.alerts[alertID] = &ThermalAlert{
			ID: alertID, Level: level, Source: s.ID, Message: msg,
			Temp: s.Temp, Threshold: threshold, Actions: actions,
			Active: true, CreatedAt: time.Now(),
		}
		am.logger.Warn("温度告警", zap.String("id", alertID), zap.String("level", string(level)), zap.Float64("temp", s.Temp))
	}
}

// executeProtection 执行保护动作.
func (am *AlertManager) executeProtection(level AlertLevel, sensorID string) []string {
	var actions []string
	switch level {
	case AlertWarning:
		actions = append(actions, "提升风扇转速")
	case AlertCritical:
		actions = append(actions, "切换全速散热")
		actions = append(actions, "发送告警通知")
	case AlertEmergency:
		actions = append(actions, "紧急全速散热")
		actions = append(actions, "发送紧急告警")
		actions = append(actions, "准备降频保护")
	}
	return actions
}

// GetActive 获取活跃告警.
func (am *AlertManager) GetActive() []ThermalAlert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	var active []ThermalAlert
	for _, a := range am.alerts {
		if a.Active {
			active = append(active, *a)
		}
	}
	return active
}

// GetAll 获取所有告警.
func (am *AlertManager) GetAll(limit int) []ThermalAlert {
	am.mu.RLock()
	defer am.mu.RUnlock()
	all := make([]ThermalAlert, 0, len(am.alerts))
	for _, a := range am.alerts {
		all = append(all, *a)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].CreatedAt.After(all[j].CreatedAt) })
	if limit > 0 && limit < len(all) {
		all = all[:limit]
	}
	return all
}

// Resolve 解决告警.
func (am *AlertManager) Resolve(id string) error {
	am.mu.Lock()
	defer am.mu.Unlock()
	a, ok := am.alerts[id]
	if !ok {
		return fmt.Errorf("告警 %s 未找到", id)
	}
	now := time.Now()
	a.Active = false
	a.ResolvedAt = &now
	return nil
}

// EmergencyCooling 紧急降温.
func (am *AlertManager) EmergencyCooling() {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.logger.Warn("执行紧急降温")
	// 设置所有风扇为全速
	for _, fan := range am.fc.fans {
		fan.Profile = FanProfileFullSpeed
		fan.TargetPWM = 100
	}
	am.fc.UpdateFans()
}

// GetSettings 获取告警设置.
func (am *AlertManager) GetSettings() AlertSettings {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return am.settings
}

// UpdateSettings 更新告警设置.
func (am *AlertManager) UpdateSettings(s AlertSettings) {
	am.mu.Lock()
	defer am.mu.Unlock()
	am.settings = s
}
