// Package smartfancontrol 提供智能风扇控制功能
// CPU/硬盘温度实时监控、风扇转速自动调节、自定义温度-转速曲线、
// 噪音模式/性能模式切换、风扇故障检测告警
package smartfancontrol

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// ========== 核心类型 ==========

// FanMode 风扇模式
type FanMode string

const (
	FanModeSilent      FanMode = "silent"      // 静音模式
	FanModeBalanced    FanMode = "balanced"    // 均衡模式
	FanModePerformance FanMode = "performance" // 性能模式
)

// FanStatus 风扇状态
type FanStatus string

const (
	FanStatusOK      FanStatus = "ok"      // 正常
	FanStatusWarning FanStatus = "warning" // 警告
	FanStatusFault   FanStatus = "fault"   // 故障
	FanStatusStopped FanStatus = "stopped" // 停止
)

// FanInfo 风扇信息
type FanInfo struct {
	ID        string    `json:"id"`        // 风扇ID
	Name      string    `json:"name"`      // 风扇名称
	RPM       int       `json:"rpm"`       // 当前转速
	MaxRPM    int       `json:"maxRpm"`    // 最大转速
	DutyCycle float64   `json:"dutyCycle"` // 占空比 (%)
	Status    FanStatus `json:"status"`    // 状态
	UpdatedAt time.Time `json:"updatedAt"`
}

// TemperaturePoint 温度监测点
type TemperaturePoint struct {
	SensorID  string    `json:"sensorId"` // 传感器ID
	Name      string    `json:"name"`     // 传感器名称
	Temp      float64   `json:"temp"`     // 当前温度 (°C)
	MaxTemp   float64   `json:"maxTemp"`  // 最高温度
	UpdatedAt time.Time `json:"updatedAt"`
}

// FanCurvePoint 温度-转速曲线点
type FanCurvePoint struct {
	Temp      float64 `json:"temp"`      // 温度 (°C)
	DutyCycle float64 `json:"dutyCycle"` // 占空比 (%)
}

// FanProfile 风扇配置方案
type FanProfile struct {
	Name      string          `json:"name"`   // 方案名称
	Mode      FanMode         `json:"mode"`   // 模式
	Curve     []FanCurvePoint `json:"curve"`  // 温度-转速曲线
	MinRPM    int             `json:"minRpm"` // 最小转速
	MaxRPM    int             `json:"maxRpm"` // 最大转速
	UpdatedAt time.Time       `json:"updatedAt"`
}

// FanAlert 风扇告警
type FanAlert struct {
	FanID     string    `json:"fanId"`     // 风扇ID
	Type      string    `json:"type"`      // 告警类型 (fault/high_temp/low_rpm)
	Message   string    `json:"message"`   // 告警信息
	Timestamp time.Time `json:"timestamp"` // 发生时间
}

// FanStatusReport 风扇状态报告
type FanStatusReport struct {
	Timestamp     time.Time          `json:"timestamp"`
	Fans          []FanInfo          `json:"fans"`
	Temps         []TemperaturePoint `json:"temps"`
	CurrentMode   FanMode            `json:"currentMode"`
	ActiveProfile *FanProfile        `json:"activeProfile"`
	Alerts        []FanAlert         `json:"alerts"`
}

// ========== Manager ==========

// Manager 智能风扇控制管理器
type Manager struct {
	mu            sync.RWMutex
	fans          map[string]*FanInfo
	temps         map[string]*TemperaturePoint
	profiles      map[string]*FanProfile
	currentMode   FanMode
	activeProfile string
	alerts        []FanAlert
	maxAlerts     int
	stopCh        chan struct{}
	running       bool
}

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		fans:        make(map[string]*FanInfo),
		temps:       make(map[string]*TemperaturePoint),
		profiles:    make(map[string]*FanProfile),
		currentMode: FanModeBalanced,
		maxAlerts:   100,
		stopCh:      make(chan struct{}),
	}

	// 初始化默认配置
	m.initDefaultProfiles()

	return m
}

// initDefaultProfiles 初始化默认风扇配置
func (m *Manager) initDefaultProfiles() {
	m.profiles["silent"] = &FanProfile{
		Name: "静音模式",
		Mode: FanModeSilent,
		Curve: []FanCurvePoint{
			{Temp: 30, DutyCycle: 20},
			{Temp: 40, DutyCycle: 30},
			{Temp: 50, DutyCycle: 40},
			{Temp: 60, DutyCycle: 50},
			{Temp: 70, DutyCycle: 70},
			{Temp: 80, DutyCycle: 100},
		},
		MinRPM: 200,
		MaxRPM: 1500,
	}

	m.profiles["balanced"] = &FanProfile{
		Name: "均衡模式",
		Mode: FanModeBalanced,
		Curve: []FanCurvePoint{
			{Temp: 30, DutyCycle: 30},
			{Temp: 40, DutyCycle: 40},
			{Temp: 50, DutyCycle: 50},
			{Temp: 60, DutyCycle: 70},
			{Temp: 70, DutyCycle: 85},
			{Temp: 80, DutyCycle: 100},
		},
		MinRPM: 300,
		MaxRPM: 2000,
	}

	m.profiles["performance"] = &FanProfile{
		Name: "性能模式",
		Mode: FanModePerformance,
		Curve: []FanCurvePoint{
			{Temp: 30, DutyCycle: 40},
			{Temp: 40, DutyCycle: 50},
			{Temp: 50, DutyCycle: 60},
			{Temp: 60, DutyCycle: 80},
			{Temp: 70, DutyCycle: 100},
			{Temp: 80, DutyCycle: 100},
		},
		MinRPM: 500,
		MaxRPM: 3000,
	}

	m.activeProfile = "balanced"
}

// GetStatusReport 获取状态报告
func (m *Manager) GetStatusReport() *FanStatusReport {
	m.mu.RLock()
	defer m.mu.RUnlock()

	report := &FanStatusReport{
		Timestamp:   time.Now(),
		CurrentMode: m.currentMode,
	}

	// 收集风扇信息
	for _, fan := range m.fans {
		report.Fans = append(report.Fans, *fan)
	}

	// 收集温度信息
	for _, temp := range m.temps {
		report.Temps = append(report.Temps, *temp)
	}

	// 获取当前配置
	if profile, ok := m.profiles[m.activeProfile]; ok {
		report.ActiveProfile = profile
	}

	// 复制告警
	report.Alerts = make([]FanAlert, len(m.alerts))
	copy(report.Alerts, m.alerts)

	return report
}

// GetFan 获取风扇信息
func (m *Manager) GetFan(id string) (*FanInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	fan, ok := m.fans[id]
	if !ok {
		return nil, fmt.Errorf("fan not found: %s", id)
	}
	return fan, nil
}

// GetTemperature 获取温度信息
func (m *Manager) GetTemperature(sensorID string) (*TemperaturePoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	temp, ok := m.temps[sensorID]
	if !ok {
		return nil, fmt.Errorf("temperature sensor not found: %s", sensorID)
	}
	return temp, nil
}

// SetMode 设置风扇模式
func (m *Manager) SetMode(mode FanMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	switch mode {
	case FanModeSilent, FanModeBalanced, FanModePerformance:
		m.currentMode = mode
		m.activeProfile = string(mode)
		log.Printf("[智能风扇] 切换模式: %s", mode)
		return nil
	default:
		return fmt.Errorf("invalid fan mode: %s", mode)
	}
}

// GetMode 获取当前模式
func (m *Manager) GetMode() FanMode {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.currentMode
}

// SetFanCurve 设置自定义温度-转速曲线
func (m *Manager) SetFanCurve(profileName string, curve []FanCurvePoint) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if len(curve) == 0 {
		return fmt.Errorf("fan curve cannot be empty")
	}

	// 验证曲线点
	for i, point := range curve {
		if point.Temp < 0 || point.Temp > 100 {
			return fmt.Errorf("invalid temperature at point %d: %.1f", i, point.Temp)
		}
		if point.DutyCycle < 0 || point.DutyCycle > 100 {
			return fmt.Errorf("invalid duty cycle at point %d: %.1f", i, point.DutyCycle)
		}
	}

	profile := &FanProfile{
		Name:      profileName,
		Mode:      m.currentMode,
		Curve:     curve,
		MinRPM:    200,
		MaxRPM:    2000,
		UpdatedAt: time.Now(),
	}

	m.profiles[profileName] = profile
	log.Printf("[智能风扇] 设置自定义曲线: %s", profileName)
	return nil
}

// GetProfile 获取风扇配置
func (m *Manager) GetProfile(name string) (*FanProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profile, ok := m.profiles[name]
	if !ok {
		return nil, fmt.Errorf("profile not found: %s", name)
	}
	return profile, nil
}

// ListProfiles 列出所有配置
func (m *Manager) ListProfiles() []*FanProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var profiles []*FanProfile
	for _, p := range m.profiles {
		profiles = append(profiles, p)
	}
	return profiles
}

// SetFanSpeed 手动设置风扇转速
func (m *Manager) SetFanSpeed(fanID string, dutyCycle float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	fan, ok := m.fans[fanID]
	if !ok {
		return fmt.Errorf("fan not found: %s", fanID)
	}

	if dutyCycle < 0 || dutyCycle > 100 {
		return fmt.Errorf("invalid duty cycle: %.1f", dutyCycle)
	}

	fan.DutyCycle = dutyCycle
	fan.RPM = int(float64(fan.MaxRPM) * dutyCycle / 100)
	fan.UpdatedAt = time.Now()

	log.Printf("[智能风扇] 手动设置风扇 %s 转速: %.1f%% (%d RPM)", fanID, dutyCycle, fan.RPM)
	return nil
}

// GetAlerts 获取告警列表
func (m *Manager) GetAlerts() []FanAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	alerts := make([]FanAlert, len(m.alerts))
	copy(alerts, m.alerts)
	return alerts
}

// ClearAlerts 清除告警
func (m *Manager) ClearAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.alerts = nil
	log.Println("[智能风扇] 清除所有告警")
}

// collect 采集一次风扇和温度数据
func (m *Manager) collect() {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()

	// 初始化默认风扇
	if len(m.fans) == 0 {
		m.fans["cpu-fan-1"] = &FanInfo{
			ID:     "cpu-fan-1",
			Name:   "CPU风扇1",
			MaxRPM: 2000,
			Status: FanStatusOK,
		}
		m.fans["case-fan-1"] = &FanInfo{
			ID:     "case-fan-1",
			Name:   "机箱风扇1",
			MaxRPM: 1500,
			Status: FanStatusOK,
		}
		m.fans["case-fan-2"] = &FanInfo{
			ID:     "case-fan-2",
			Name:   "机箱风扇2",
			MaxRPM: 1500,
			Status: FanStatusOK,
		}
	}

	// 初始化默认温度传感器
	if len(m.temps) == 0 {
		m.temps["cpu-temp"] = &TemperaturePoint{
			SensorID: "cpu-temp",
			Name:     "CPU温度",
		}
		m.temps["hdd-temp-1"] = &TemperaturePoint{
			SensorID: "hdd-temp-1",
			Name:     "硬盘1温度",
		}
		m.temps["hdd-temp-2"] = &TemperaturePoint{
			SensorID: "hdd-temp-2",
			Name:     "硬盘2温度",
		}
	}

	// 更新温度 (模拟数据)
	for _, temp := range m.temps {
		if temp.SensorID == "cpu-temp" {
			temp.Temp = 45.0 + float64(time.Now().Second()%10)
		} else {
			temp.Temp = 35.0 + float64(time.Now().Second()%5)
		}
		if temp.Temp > temp.MaxTemp {
			temp.MaxTemp = temp.Temp
		}
		temp.UpdatedAt = now
	}

	// 获取当前温度用于计算转速
	var maxTemp float64
	for _, temp := range m.temps {
		if temp.Temp > maxTemp {
			maxTemp = temp.Temp
		}
	}

	// 根据当前配置计算转速
	profile, ok := m.profiles[m.activeProfile]
	if ok {
		dutyCycle := m.calculateDutyCycle(maxTemp, profile)
		for _, fan := range m.fans {
			fan.DutyCycle = dutyCycle
			fan.RPM = int(float64(fan.MaxRPM) * dutyCycle / 100)
			fan.UpdatedAt = now
		}
	}

	// 检查故障和告警
	m.checkAlerts(now)
}

// calculateDutyCycle 根据温度曲线计算占空比
func (m *Manager) calculateDutyCycle(temp float64, profile *FanProfile) float64 {
	if len(profile.Curve) == 0 {
		return 50 // 默认50%
	}

	// 低于最低点
	if temp <= profile.Curve[0].Temp {
		return profile.Curve[0].DutyCycle
	}

	// 高于最高点
	if temp >= profile.Curve[len(profile.Curve)-1].Temp {
		return profile.Curve[len(profile.Curve)-1].DutyCycle
	}

	// 线性插值
	for i := 0; i < len(profile.Curve)-1; i++ {
		if temp >= profile.Curve[i].Temp && temp <= profile.Curve[i+1].Temp {
			ratio := (temp - profile.Curve[i].Temp) / (profile.Curve[i+1].Temp - profile.Curve[i].Temp)
			return profile.Curve[i].DutyCycle + ratio*(profile.Curve[i+1].DutyCycle-profile.Curve[i].DutyCycle)
		}
	}

	return 50
}

// checkAlerts 检查告警
func (m *Manager) checkAlerts(now time.Time) {
	// 检查高温告警
	for _, temp := range m.temps {
		if temp.Temp >= 80 {
			alert := FanAlert{
				Type:      "high_temp",
				Message:   fmt.Sprintf("%s 温度过高: %.1f°C", temp.Name, temp.Temp),
				Timestamp: now,
			}
			m.addAlert(alert)
		}
	}

	// 检查风扇故障
	for _, fan := range m.fans {
		if fan.Status == FanStatusFault {
			alert := FanAlert{
				FanID:     fan.ID,
				Type:      "fault",
				Message:   fmt.Sprintf("风扇 %s 故障", fan.Name),
				Timestamp: now,
			}
			m.addAlert(alert)
		}
	}
}

// addAlert 添加告警
func (m *Manager) addAlert(alert FanAlert) {
	m.alerts = append(m.alerts, alert)
	if len(m.alerts) > m.maxAlerts {
		m.alerts = m.alerts[len(m.alerts)-m.maxAlerts:]
	}
}

// Start 启动定时采集
func (m *Manager) Start(interval time.Duration) {
	m.mu.Lock()
	if m.running {
		m.mu.Unlock()
		return
	}
	m.running = true
	m.stopCh = make(chan struct{})
	m.mu.Unlock()

	go func() {
		// 立即采集一次
		m.collect()

		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				m.collect()
			case <-m.stopCh:
				return
			}
		}
	}()

	log.Printf("[智能风扇] 启动定时采集，间隔 %v", interval)
}

// Stop 停止定时采集
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopCh)
	log.Println("[智能风扇] 停止定时采集")
}
