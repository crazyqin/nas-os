// Package smartfan 智能风扇控制
// 对标飞牛fnOS硬件监控
// 基于温度自动调节风扇转速，支持自定义曲线、告警
package smartfan

import (
	"fmt"
	"sync"
	"time"
)

// FanMode 风扇模式.
type FanMode string

const (
	FanModeAuto   FanMode = "auto"
	FanModeManual FanMode = "manual"
	FanModeSilent FanMode = "silent"
	FanModePerf   FanMode = "performance"
	FanModeCustom FanMode = "custom"
)

// FanProfile 风扇配置曲线.
type FanProfile struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Mode      FanMode      `json:"mode"`
	Points    []CurvePoint `json:"points"`
	MinRPM    int          `json:"min_rpm"`
	MaxRPM    int          `json:"max_rpm"`
	CreatedAt time.Time    `json:"created_at"`
}

// CurvePoint 温度-RPM曲线点.
type CurvePoint struct {
	Temperature float64 `json:"temperature"`
	DutyPercent int     `json:"duty_percent"`
}

// FanStatus 风扇状态.
type FanStatus struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	RPM         int       `json:"rpm"`
	DutyPercent int       `json:"duty_percent"`
	Mode        FanMode   `json:"mode"`
	Temperature float64   `json:"temperature"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// TemperatureSensor 温度传感器.
type TemperatureSensor struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Temp      float64   `json:"temp"`
	High      float64   `json:"high"`
	Critical  float64   `json:"critical"`
	UpdatedAt time.Time `json:"updated_at"`
}

// FanAlert 风扇告警.
type FanAlert struct {
	ID        string    `json:"id"`
	Type      string    `json:"type"`
	Message   string    `json:"message"`
	Temp      float64   `json:"temp"`
	Threshold float64   `json:"threshold"`
	CreatedAt time.Time `json:"created_at"`
	Resolved  bool      `json:"resolved"`
}

// Manager 风扇管理器.
type Manager struct {
	mu       sync.RWMutex
	fans     map[string]*FanStatus
	sensors  map[string]*TemperatureSensor
	profiles map[string]*FanProfile
	activeID string
	alerts   []FanAlert
	stopCh   chan struct{}
}

// NewManager 创建风扇管理器.
func NewManager() *Manager {
	m := &Manager{
		fans:     make(map[string]*FanStatus),
		sensors:  make(map[string]*TemperatureSensor),
		profiles: make(map[string]*FanProfile),
		alerts:   make([]FanAlert, 0),
		stopCh:   make(chan struct{}),
	}
	m.initDefaults()
	return m
}

func (m *Manager) initDefaults() {
	// 默认风扇
	m.fans["fan0"] = &FanStatus{ID: "fan0", Name: "CPU风扇", Mode: FanModeAuto, RPM: 1200, DutyPercent: 40}
	m.fans["fan1"] = &FanStatus{ID: "fan1", Name: "系统风扇", Mode: FanModeAuto, RPM: 800, DutyPercent: 30}

	// 默认温度传感器
	m.sensors["cpu"] = &TemperatureSensor{ID: "cpu", Name: "CPU温度", Temp: 45, High: 75, Critical: 90}
	m.sensors["hdd0"] = &TemperatureSensor{ID: "hdd0", Name: "硬盘1温度", Temp: 38, High: 55, Critical: 65}
	m.sensors["hdd1"] = &TemperatureSensor{ID: "hdd1", Name: "硬盘2温度", Temp: 36, High: 55, Critical: 65}

	// 默认配置曲线
	m.profiles["silent"] = &FanProfile{
		ID: "silent", Name: "静音模式", Mode: FanModeSilent,
		Points: []CurvePoint{{30, 20}, {50, 30}, {65, 50}, {75, 70}, {85, 100}},
		MinRPM: 400, MaxRPM: 2000,
	}
	m.profiles["performance"] = &FanProfile{
		ID: "performance", Name: "性能模式", Mode: FanModePerf,
		Points: []CurvePoint{{30, 40}, {45, 50}, {55, 65}, {65, 80}, {75, 100}},
		MinRPM: 800, MaxRPM: 3000,
	}
	m.profiles["balanced"] = &FanProfile{
		ID: "balanced", Name: "均衡模式", Mode: FanModeAuto,
		Points: []CurvePoint{{30, 25}, {50, 40}, {60, 55}, {70, 70}, {80, 90}},
		MinRPM: 600, MaxRPM: 2500,
	}
	m.activeID = "balanced"
}

// Start 启动监控.
func (m *Manager) Start() {
	go m.monitorLoop()
}

// Stop 停止监控.
func (m *Manager) Stop() {
	close(m.stopCh)
}

func (m *Manager) monitorLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			m.adjustFans()
		case <-m.stopCh:
			return
		}
	}
}

func (m *Manager) adjustFans() {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile := m.profiles[m.activeID]
	if profile == nil {
		return
	}

	// 获取最高温度
	maxTemp := 0.0
	for _, s := range m.sensors {
		if s.Temp > maxTemp {
			maxTemp = s.Temp
		}
	}

	// 根据曲线计算目标转速
	duty := m.calculateDuty(maxTemp, profile)
	for _, fan := range m.fans {
		if fan.Mode == FanModeAuto || fan.Mode == FanModeSilent || fan.Mode == FanModePerf {
			fan.DutyPercent = duty
			fan.RPM = profile.MinRPM + (profile.MaxRPM-profile.MinRPM)*duty/100
			fan.UpdatedAt = time.Now()
		}
	}

	// 检查告警
	for _, s := range m.sensors {
		if s.Temp >= s.Critical {
			m.alerts = append(m.alerts, FanAlert{
				ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
				Type:      "critical",
				Message:   fmt.Sprintf("%s 温度 %.1f°C 超过临界值 %.1f°C", s.Name, s.Temp, s.Critical),
				Temp:      s.Temp,
				Threshold: s.Critical,
				CreatedAt: time.Now(),
			})
		} else if s.Temp >= s.High {
			m.alerts = append(m.alerts, FanAlert{
				ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
				Type:      "warning",
				Message:   fmt.Sprintf("%s 温度 %.1f°C 超过高温阈值 %.1f°C", s.Name, s.Temp, s.High),
				Temp:      s.Temp,
				Threshold: s.High,
				CreatedAt: time.Now(),
			})
		}
	}
}

func (m *Manager) calculateDuty(temp float64, profile *FanProfile) int {
	if len(profile.Points) == 0 {
		return 50
	}
	if temp <= profile.Points[0].Temperature {
		return profile.Points[0].DutyPercent
	}
	for i := 1; i < len(profile.Points); i++ {
		if temp <= profile.Points[i].Temperature {
			p1 := profile.Points[i-1]
			p2 := profile.Points[i]
			ratio := (temp - p1.Temperature) / (p2.Temperature - p1.Temperature)
			return int(float64(p1.DutyPercent) + ratio*float64(p2.DutyPercent-p1.DutyPercent))
		}
	}
	return profile.Points[len(profile.Points)-1].DutyPercent
}

// GetFans 获取所有风扇状态.
func (m *Manager) GetFans() []*FanStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()
	fans := make([]*FanStatus, 0, len(m.fans))
	for _, f := range m.fans {
		fans = append(fans, f)
	}
	return fans
}

// GetSensors 获取所有温度传感器.
func (m *Manager) GetSensors() []*TemperatureSensor {
	m.mu.RLock()
	defer m.mu.RUnlock()
	sensors := make([]*TemperatureSensor, 0, len(m.sensors))
	for _, s := range m.sensors {
		sensors = append(sensors, s)
	}
	return sensors
}

// GetProfiles 获取所有配置曲线.
func (m *Manager) GetProfiles() []*FanProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	profiles := make([]*FanProfile, 0, len(m.profiles))
	for _, p := range m.profiles {
		profiles = append(profiles, p)
	}
	return profiles
}

// SetProfile 设置活跃配置.
func (m *Manager) SetProfile(profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.profiles[profileID]; !ok {
		return fmt.Errorf("配置不存在: %s", profileID)
	}
	m.activeID = profileID
	return nil
}

// SetFanMode 设置风扇模式.
func (m *Manager) SetFanMode(fanID string, mode FanMode, dutyPercent int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	fan, ok := m.fans[fanID]
	if !ok {
		return fmt.Errorf("风扇不存在: %s", fanID)
	}
	fan.Mode = mode
	if mode == FanModeManual {
		fan.DutyPercent = dutyPercent
	}
	return nil
}

// GetAlerts 获取告警.
func (m *Manager) GetAlerts(resolved bool) []FanAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	alerts := make([]FanAlert, 0)
	for _, a := range m.alerts {
		if a.Resolved == resolved {
			alerts = append(alerts, a)
		}
	}
	return alerts
}
