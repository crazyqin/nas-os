// Package smartfan 提供智能风扇控制功能
// 温度曲线调速、静音模式、自适应调速、多区域控制
package smartfan

import (
	"fmt"
	"log"
	"math"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// FanMode 风扇模式
type FanMode string

const (
	FanModeSilent  FanMode = "silent"  // 静音模式
	FanModeNormal  FanMode = "normal"  // 正常模式
	FanModePerformance FanMode = "performance" // 性能模式
	FanModeCustom  FanMode = "custom"  // 自定义模式
	FanModeAuto    FanMode = "auto"    // 自适应模式
)

// FanZone 风扇区域
type FanZone struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	FanIDs      []string    `json:"fanIds"`
	TempSource  string      `json:"tempSource"` // 温度传感器ID
	MinRPM      int         `json:"minRpm"`
	MaxRPM      int         `json:"maxRpm"`
	CurrentRPM  int         `json:"currentRpm"`
	CurrentTemp float64     `json:"currentTemp"` // 摄氏度
	Mode        FanMode     `json:"mode"`
	Curve       []CurvePoint `json:"curve,omitempty"`
	Enabled     bool        `json:"enabled"`
}

// CurvePoint 温度曲线点
type CurvePoint struct {
	Temp   float64 `json:"temp"`   // 温度 (°C)
	Duty   int     `json:"duty"`   // 占空比 (0-100%)
}

// FanInfo 风扇信息
type FanInfo struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	ZoneID      string `json:"zoneId"`
	CurrentRPM  int    `json:"currentRpm"`
	MaxRPM      int    `json:"maxRpm"`
	Duty        int    `json:"duty"` // 0-100%
	IsHealthy   bool   `json:"isHealthy"`
	Warning     string `json:"warning,omitempty"`
}

// TempSensor 温度传感器
type TempSensor struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Temp      float64   `json:"temp"` // 摄氏度
	High      float64   `json:"high"` // 高温阈值
	Critical  float64   `json:"critical"` // 临界温度
	UpdatedAt time.Time `json:"updatedAt"`
}

// FanProfile 风扇配置方案
type FanProfile struct {
	ID          string              `json:"id"`
	Name        string              `json:"name"`
	Description string              `json:"description"`
	Mode        FanMode             `json:"mode"`
	Zones       map[string][]CurvePoint `json:"zones"` // zoneID -> curve
	IsDefault   bool                `json:"isDefault"`
	CreatedAt   time.Time           `json:"createdAt"`
}

// AlertEvent 告警事件
type AlertEvent struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"` // high_temp, fan_failure, low_rpm
	ZoneID    string    `json:"zoneId"`
	FanID     string    `json:"fanId,omitempty"`
	Message   string    `json:"message"`
	Severity  string    `json:"severity"` // warning, critical
	Temp      float64   `json:"temp,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Resolved  bool      `json:"resolved"`
}

// FanStats 风扇统计
type FanStats struct {
	AvgTemp     float64 `json:"avgTemp"`
	MaxTemp     float64 `json:"maxTemp"`
	AvgRPM      int     `json:"avgRpm"`
	TotalAlerts int     `json:"totalAlerts"`
	Uptime      time.Duration `json:"uptime"`
}

// ========== Manager ==========

// Manager 智能风扇管理器
type Manager struct {
	mu        sync.RWMutex
	zones     map[string]*FanZone
	fans      map[string]*FanInfo
	sensors   map[string]*TempSensor
	profiles  map[string]*FanProfile
	alerts    []AlertEvent
	currentMode FanMode
	startTime time.Time
}

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		zones:       make(map[string]*FanZone),
		fans:        make(map[string]*FanInfo),
		sensors:     make(map[string]*TempSensor),
		profiles:    make(map[string]*FanProfile),
		currentMode: FanModeAuto,
		startTime:   time.Now(),
	}
	m.initDefaults()
	return m
}

// initDefaults 初始化默认配置
func (m *Manager) initDefaults() {
	// 默认温度传感器
	m.sensors["cpu"] = &TempSensor{
		ID: "cpu", Name: "CPU 温度", Temp: 45.0, High: 80.0, Critical: 95.0, UpdatedAt: time.Now(),
	}
	m.sensors["hdd"] = &TempSensor{
		ID: "hdd", Name: "硬盘温度", Temp: 38.0, High: 55.0, Critical: 65.0, UpdatedAt: time.Now(),
	}
	m.sensors["system"] = &TempSensor{
		ID: "system", Name: "系统温度", Temp: 40.0, High: 70.0, Critical: 85.0, UpdatedAt: time.Now(),
	}

	// 默认风扇区域
	m.zones["cpu"] = &FanZone{
		ID: "cpu", Name: "CPU 散热", TempSource: "cpu",
		MinRPM: 300, MaxRPM: 2000, CurrentRPM: 800, CurrentTemp: 45.0,
		Mode: FanModeAuto, Enabled: true,
		FanIDs: []string{"cpu-fan-1"},
		Curve: []CurvePoint{
			{Temp: 30, Duty: 20}, {Temp: 45, Duty: 35}, {Temp: 60, Duty: 55},
			{Temp: 75, Duty: 80}, {Temp: 85, Duty: 100},
		},
	}
	m.zones["hdd"] = &FanZone{
		ID: "hdd", Name: "硬盘散热", TempSource: "hdd",
		MinRPM: 200, MaxRPM: 1500, CurrentRPM: 500, CurrentTemp: 38.0,
		Mode: FanModeAuto, Enabled: true,
		FanIDs: []string{"hdd-fan-1"},
		Curve: []CurvePoint{
			{Temp: 25, Duty: 15}, {Temp: 35, Duty: 30}, {Temp: 45, Duty: 50},
			{Temp: 55, Duty: 80}, {Temp: 60, Duty: 100},
		},
	}
	m.zones["system"] = &FanZone{
		ID: "system", Name: "系统散热", TempSource: "system",
		MinRPM: 200, MaxRPM: 1200, CurrentRPM: 400, CurrentTemp: 40.0,
		Mode: FanModeAuto, Enabled: true,
		FanIDs: []string{"sys-fan-1"},
		Curve: []CurvePoint{
			{Temp: 30, Duty: 15}, {Temp: 45, Duty: 30}, {Temp: 60, Duty: 60},
			{Temp: 70, Duty: 90}, {Temp: 80, Duty: 100},
		},
	}

	// 默认风扇
	m.fans["cpu-fan-1"] = &FanInfo{
		ID: "cpu-fan-1", Name: "CPU 风扇", ZoneID: "cpu", CurrentRPM: 800, MaxRPM: 2000, Duty: 35, IsHealthy: true,
	}
	m.fans["hdd-fan-1"] = &FanInfo{
		ID: "hdd-fan-1", Name: "硬盘风扇", ZoneID: "hdd", CurrentRPM: 500, MaxRPM: 1500, Duty: 30, IsHealthy: true,
	}
	m.fans["sys-fan-1"] = &FanInfo{
		ID: "sys-fan-1", Name: "系统风扇", ZoneID: "system", CurrentRPM: 400, MaxRPM: 1200, Duty: 25, IsHealthy: true,
	}

	// 默认配置方案
	m.profiles["silent"] = &FanProfile{
		ID: "silent", Name: "静音模式", Description: "低噪音运行，适合夜间",
		Mode: FanModeSilent, IsDefault: false, CreatedAt: time.Now(),
	}
	m.profiles["performance"] = &FanProfile{
		ID: "performance", Name: "性能模式", Description: "最大散热，适合高负载",
		Mode: FanModePerformance, IsDefault: false, CreatedAt: time.Now(),
	}
	m.profiles["auto"] = &FanProfile{
		ID: "auto", Name: "自适应模式", Description: "根据温度自动调节",
		Mode: FanModeAuto, IsDefault: true, CreatedAt: time.Now(),
	}
}

// ========== 温度管理 ==========

// UpdateTemperature 更新温度
func (m *Manager) UpdateTemperature(sensorID string, temp float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	sensor, ok := m.sensors[sensorID]
	if !ok {
		return fmt.Errorf("sensor %s not found", sensorID)
	}

	sensor.Temp = temp
	sensor.UpdatedAt = time.Now()

	// 更新关联的风扇区域
	for _, zone := range m.zones {
		if zone.TempSource == sensorID {
			zone.CurrentTemp = temp
			if zone.Enabled && zone.Mode == FanModeAuto {
				m.adjustZone(zone, temp)
			}
		}
	}

	// 检查告警
	if temp >= sensor.Critical {
		m.addAlert("high_temp", sensorID, "", fmt.Sprintf("%s 温度 %.1f°C 达到临界值", sensor.Name, temp), "critical", temp)
	} else if temp >= sensor.High {
		m.addAlert("high_temp", sensorID, "", fmt.Sprintf("%s 温度 %.1f°C 偏高", sensor.Name, temp), "warning", temp)
	}

	return nil
}

// adjustZone 根据温度调节区域
func (m *Manager) adjustZone(zone *FanZone, temp float64) {
	if len(zone.Curve) == 0 {
		return
	}

	// 在曲线上插值
	duty := m.interpolateCurve(zone.Curve, temp)
	zone.CurrentRPM = zone.MinRPM + int(float64(zone.MaxRPM-zone.MinRPM)*float64(duty)/100.0)

	// 更新关联的风扇
	for _, fanID := range zone.FanIDs {
		if fan, ok := m.fans[fanID]; ok {
			fan.CurrentRPM = zone.CurrentRPM
			fan.Duty = duty
		}
	}
}

// interpolateCurve 曲线插值
func (m *Manager) interpolateCurve(curve []CurvePoint, temp float64) int {
	if len(curve) == 0 {
		return 50
	}

	// 低于最低点
	if temp <= curve[0].Temp {
		return curve[0].Duty
	}

	// 高于最高点
	if temp >= curve[len(curve)-1].Temp {
		return curve[len(curve)-1].Duty
	}

	// 线性插值
	for i := 0; i < len(curve)-1; i++ {
		if temp >= curve[i].Temp && temp <= curve[i+1].Temp {
			ratio := (temp - curve[i].Temp) / (curve[i+1].Temp - curve[i].Temp)
			duty := float64(curve[i].Duty) + ratio*float64(curve[i+1].Duty-curve[i].Duty)
			return int(math.Round(duty))
		}
	}

	return 50
}

// addAlert 添加告警
func (m *Manager) addAlert(alertType, zoneID, fanID, message, severity string, temp float64) {
	alert := AlertEvent{
		ID:        fmt.Sprintf("alert-%d", len(m.alerts)+1),
		Type:      alertType,
		ZoneID:    zoneID,
		FanID:     fanID,
		Message:   message,
		Severity:  severity,
		Temp:      temp,
		Timestamp: time.Now(),
	}
	m.alerts = append(m.alerts, alert)
	log.Printf("[智能风扇] 告警: %s", message)
}

// ========== 模式管理 ==========

// SetMode 设置风扇模式
func (m *Manager) SetMode(mode FanMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.currentMode = mode

	for _, zone := range m.zones {
		if !zone.Enabled {
			continue
		}
		zone.Mode = mode

		switch mode {
		case FanModeSilent:
			// 静音模式：限制最大转速
			maxDuty := 50
			duty := m.interpolateCurve(zone.Curve, zone.CurrentTemp)
			if duty > maxDuty {
				duty = maxDuty
			}
			zone.CurrentRPM = zone.MinRPM + int(float64(zone.MaxRPM-zone.MinRPM)*float64(duty)/100.0)
		case FanModePerformance:
			// 性能模式：全速
			zone.CurrentRPM = zone.MaxRPM
		case FanModeAuto:
			// 自适应模式
			m.adjustZone(zone, zone.CurrentTemp)
		case FanModeNormal:
			// 正常模式：固定 50%
			zone.CurrentRPM = zone.MinRPM + (zone.MaxRPM-zone.MinRPM)/2
		}

		for _, fanID := range zone.FanIDs {
			if fan, ok := m.fans[fanID]; ok {
				fan.CurrentRPM = zone.CurrentRPM
				fan.Duty = int(float64(zone.CurrentRPM-zone.MinRPM) / float64(zone.MaxRPM-zone.MinRPM) * 100)
			}
		}
	}

	log.Printf("[智能风扇] 切换模式: %s", mode)
	return nil
}

// GetMode 获取当前模式
func (m *Manager) GetMode() FanMode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentMode
}

// ========== 区域管理 ==========

// GetZone 获取区域
func (m *Manager) GetZone(id string) (*FanZone, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	zone, ok := m.zones[id]
	if !ok {
		return nil, fmt.Errorf("zone %s not found", id)
	}
	return zone, nil
}

// ListZones 列出所有区域
func (m *Manager) ListZones() []*FanZone {
	m.mu.RLock()
	defer m.mu.RUnlock()

	zones := make([]*FanZone, 0, len(m.zones))
	for _, z := range m.zones {
		zones = append(zones, z)
	}
	return zones
}

// UpdateZoneCurve 更新区域温度曲线
func (m *Manager) UpdateZoneCurve(zoneID string, curve []CurvePoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	zone, ok := m.zones[zoneID]
	if !ok {
		return fmt.Errorf("zone %s not found", zoneID)
	}

	zone.Curve = curve
	zone.Mode = FanModeCustom
	log.Printf("[智能风扇] 更新区域 %s 温度曲线", zoneID)
	return nil
}

// ========== 风扇管理 ==========

// GetFan 获取风扇
func (m *Manager) GetFan(id string) (*FanInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fan, ok := m.fans[id]
	if !ok {
		return nil, fmt.Errorf("fan %s not found", id)
	}
	return fan, nil
}

// ListFans 列出所有风扇
func (m *Manager) ListFans() []*FanInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fans := make([]*FanInfo, 0, len(m.fans))
	for _, f := range m.fans {
		fans = append(fans, f)
	}
	return fans
}

// ========== 传感器 ==========

// GetSensor 获取传感器
func (m *Manager) GetSensor(id string) (*TempSensor, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sensor, ok := m.sensors[id]
	if !ok {
		return nil, fmt.Errorf("sensor %s not found", id)
	}
	return sensor, nil
}

// ListSensors 列出所有传感器
func (m *Manager) ListSensors() []*TempSensor {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sensors := make([]*TempSensor, 0, len(m.sensors))
	for _, s := range m.sensors {
		sensors = append(sensors, s)
	}
	return sensors
}

// ========== 告警 ==========

// GetAlerts 获取告警
func (m *Manager) GetAlerts(resolved bool) []AlertEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var alerts []AlertEvent
	for _, a := range m.alerts {
		if a.Resolved == resolved {
			alerts = append(alerts, a)
		}
	}
	return alerts
}

// ResolveAlert 解决告警
func (m *Manager) ResolveAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i := range m.alerts {
		if m.alerts[i].ID == id {
			m.alerts[i].Resolved = true
			return nil
		}
	}
	return fmt.Errorf("alert %s not found", id)
}

// ========== 统计 ==========

// GetStats 获取统计
func (m *Manager) GetStats() FanStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	totalTemp := 0.0
	maxTemp := 0.0
	totalRPM := 0
	count := 0

	for _, zone := range m.zones {
		totalTemp += zone.CurrentTemp
		totalRPM += zone.CurrentRPM
		count++
		if zone.CurrentTemp > maxTemp {
			maxTemp = zone.CurrentTemp
		}
	}

	avgTemp := 0.0
	avgRPM := 0
	if count > 0 {
		avgTemp = totalTemp / float64(count)
		avgRPM = totalRPM / count
	}

	unresolved := 0
	for _, a := range m.alerts {
		if !a.Resolved {
			unresolved++
		}
	}

	return FanStats{
		AvgTemp:     avgTemp,
		MaxTemp:     maxTemp,
		AvgRPM:      avgRPM,
		TotalAlerts: unresolved,
		Uptime:      time.Since(m.startTime),
	}
}
