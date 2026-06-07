// Package thermal 提供系统散热与温控管理功能
// Version: v1.0.0 - 温控管理
package thermal

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// ThermalZone 温度区域
type ThermalZone struct {
	ID        string     `json:"id"`
	Name      string     `json:"name"`
	Type      string     `json:"type"`
	Temp      float64    `json:"temp"`     // 摄氏度
	MaxTemp   float64    `json:"maxTemp"`  // 最高温度阈值
	CritTemp  float64    `json:"critTemp"` // 临界温度阈值
	Status    ZoneStatus `json:"status"`
	UpdatedAt time.Time  `json:"updatedAt"`
}

// ZoneStatus 温度区域状态
type ZoneStatus string

const (
	StatusNormal   ZoneStatus = "normal"
	StatusWarm     ZoneStatus = "warm"
	StatusHot      ZoneStatus = "hot"
	StatusCritical ZoneStatus = "critical"
)

// FanInfo 风扇信息
type FanInfo struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Speed    int     `json:"speed"` // RPM
	MaxSpeed int     `json:"maxSpeed"`
	Percent  float64 `json:"percent"` // 0-100
	Mode     FanMode `json:"mode"`
}

// FanMode 风扇模式
type FanMode string

const (
	FanAuto   FanMode = "auto"
	FanManual FanMode = "manual"
	FanSilent FanMode = "silent"
	FanTurbo  FanMode = "turbo"
)

// ThermalPolicy 温控策略
type ThermalPolicy struct {
	Name          string        `json:"name"`
	WarmThresh    float64       `json:"warmThresh"`  // 升温阈值
	HotThresh     float64       `json:"hotThresh"`   // 高温阈值
	CritThresh    float64       `json:"critThresh"`  // 临界阈值
	FanCurve      []FanPoint    `json:"fanCurve"`    // 风扇曲线
	ThrottleCPU   bool          `json:"throttleCPU"` // 高温时降频
	CheckInterval time.Duration `json:"checkInterval"`
}

// FanPoint 风扇曲线控制点
type FanPoint struct {
	Temp   float64 `json:"temp"`
	Target float64 `json:"target"` // 风扇百分比
}

// ThermalOverview 散热总览
type ThermalOverview struct {
	CPUTemp       float64        `json:"cpuTemp"`
	GPUTemp       float64        `json:"gpuTemp,omitempty"`
	AmbientTemp   float64        `json:"ambientTemp"`
	HottestZone   string         `json:"hottestZone"`
	HottestTemp   float64        `json:"hottestTemp"`
	OverallStatus ZoneStatus     `json:"overallStatus"`
	Zones         []ThermalZone  `json:"zones"`
	Fans          []FanInfo      `json:"fans"`
	Alerts        []ThermalAlert `json:"alerts,omitempty"`
	Trend         TempTrend      `json:"trend"`
	UpdatedAt     time.Time      `json:"updatedAt"`
}

// TempTrend 温度趋势
type TempTrend struct {
	Direction string  `json:"direction"` // rising, falling, stable
	Rate      float64 `json:"rate"`      // 每分钟变化率
}

// ThermalAlert 温度告警
type ThermalAlert struct {
	Zone      string    `json:"zone"`
	Temp      float64   `json:"temp"`
	Threshold float64   `json:"threshold"`
	Level     string    `json:"level"` // warm, hot, critical
	Message   string    `json:"message"`
	Time      time.Time `json:"time"`
}

// Manager 温控管理器
type Manager struct {
	logger   *zap.Logger
	mu       sync.RWMutex
	zones    []ThermalZone
	fans     []FanInfo
	policy   ThermalPolicy
	alerts   []ThermalAlert
	history  []TempHistory
	maxAlert int
	maxHist  int
}

// TempHistory 温度历史记录
type TempHistory struct {
	Timestamp time.Time          `json:"timestamp"`
	Temps     map[string]float64 `json:"temps"`
}

var (
	ErrZoneNotFound = fmt.Errorf("温度区域未找到")
	ErrFanNotFound  = fmt.Errorf("风扇未找到")
)

// NewManager 创建温控管理器
func NewManager(logger *zap.Logger) *Manager {
	m := &Manager{
		logger:   logger,
		maxAlert: 100,
		maxHist:  1440, // 24h @ 1min intervals
		policy: ThermalPolicy{
			Name:       "default",
			WarmThresh: 60,
			HotThresh:  75,
			CritThresh: 90,
			FanCurve: []FanPoint{
				{Temp: 30, Target: 20},
				{Temp: 50, Target: 40},
				{Temp: 65, Target: 60},
				{Temp: 75, Target: 80},
				{Temp: 85, Target: 100},
			},
			ThrottleCPU:   true,
			CheckInterval: time.Minute,
		},
	}
	return m
}

// LoadZones 从 sysfs 加载温度区域
func (m *Manager) LoadZones() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	basePath := "/sys/class/thermal"
	entries, err := os.ReadDir(basePath)
	if err != nil {
		m.logger.Warn("无法读取thermal目录，使用模拟数据", zap.Error(err))
		m.loadMockZones()
		return nil
	}

	var zones []ThermalZone
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "thermal_zone") {
			continue
		}
		zonePath := filepath.Join(basePath, entry.Name())
		zone := m.readZone(zonePath, entry.Name())
		if zone != nil {
			zones = append(zones, *zone)
		}
	}

	if len(zones) == 0 {
		m.loadMockZones()
		return nil
	}

	m.zones = zones
	m.logger.Info("加载温度区域", zap.Int("count", len(zones)))
	return nil
}

// readZone 从 sysfs 读取单个温度区域
func (m *Manager) readZone(path, id string) *ThermalZone {
	tempFile := filepath.Join(path, "temp")
	typeFile := filepath.Join(path, "type")

	data, err := os.ReadFile(tempFile)
	if err != nil {
		return nil
	}
	tempVal, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64)
	if err != nil {
		return nil
	}
	// sysfs 温度值通常是 millidegree
	temp := tempVal / 1000.0
	if temp > 200 {
		temp = tempVal // 已经是摄氏度
	}

	name := id
	if typeData, err := os.ReadFile(typeFile); err == nil {
		name = strings.TrimSpace(string(typeData))
	}

	return &ThermalZone{
		ID:        id,
		Name:      name,
		Type:      name,
		Temp:      temp,
		MaxTemp:   m.policy.HotThresh,
		CritTemp:  m.policy.CritThresh,
		Status:    m.classifyTemp(temp),
		UpdatedAt: time.Now(),
	}
}

// loadMockZones 加载模拟温度数据
func (m *Manager) loadMockZones() {
	m.zones = []ThermalZone{
		{ID: "zone0", Name: "CPU", Type: "x86_pkg_temp", Temp: 45, MaxTemp: 75, CritTemp: 90, Status: StatusNormal, UpdatedAt: time.Now()},
		{ID: "zone1", Name: "GPU", Type: "gpu_thermal", Temp: 38, MaxTemp: 80, CritTemp: 95, Status: StatusNormal, UpdatedAt: time.Now()},
		{ID: "zone2", Name: "Ambient", Type: "ambient", Temp: 28, MaxTemp: 50, CritTemp: 60, Status: StatusNormal, UpdatedAt: time.Now()},
		{ID: "zone3", Name: "HDD Bay", Type: "hdd_thermal", Temp: 35, MaxTemp: 55, CritTemp: 65, Status: StatusNormal, UpdatedAt: time.Now()},
	}
	m.fans = []FanInfo{
		{ID: "fan0", Name: "CPU Fan", Speed: 1200, MaxSpeed: 3000, Percent: 40, Mode: FanAuto},
		{ID: "fan1", Name: "System Fan", Speed: 800, MaxSpeed: 2500, Percent: 32, Mode: FanAuto},
	}
}

// classifyTemp 根据温度分类状态
func (m *Manager) classifyTemp(temp float64) ZoneStatus {
	switch {
	case temp >= m.policy.CritThresh:
		return StatusCritical
	case temp >= m.policy.HotThresh:
		return StatusHot
	case temp >= m.policy.WarmThresh:
		return StatusWarm
	default:
		return StatusNormal
	}
}

// GetOverview 获取散热总览
func (m *Manager) GetOverview() ThermalOverview {
	m.mu.RLock()
	defer m.mu.RUnlock()

	overview := ThermalOverview{
		Zones:     m.zones,
		Fans:      m.fans,
		UpdatedAt: time.Now(),
	}

	if len(m.alerts) > 0 {
		overview.Alerts = m.alerts
	}

	// 找最热区域
	overall := StatusNormal
	for _, z := range m.zones {
		if z.Temp > overview.HottestTemp {
			overview.HottestTemp = z.Temp
			overview.HottestZone = z.Name
		}
		if z.Status == StatusCritical {
			overall = StatusCritical
		} else if z.Status == StatusHot && overall != StatusCritical {
			overall = StatusHot
		} else if z.Status == StatusWarm && overall == StatusNormal {
			overall = StatusWarm
		}
		if z.Name == "CPU" {
			overview.CPUTemp = z.Temp
		} else if z.Name == "GPU" {
			overview.GPUTemp = z.Temp
		} else if z.Name == "Ambient" {
			overview.AmbientTemp = z.Temp
		}
	}
	overview.OverallStatus = overall

	// 计算趋势
	overview.Trend = m.calcTrend()

	return overview
}

// calcTrend 计算温度趋势
func (m *Manager) calcTrend() TempTrend {
	if len(m.history) < 2 {
		return TempTrend{Direction: "stable", Rate: 0}
	}

	last := m.history[len(m.history)-1]
	prev := m.history[len(m.history)-2]

	// 取 CPU 温度计算趋势
	cpuNow, ok1 := last.Temps["CPU"]
	cpuPrev, ok2 := prev.Temps["CPU"]
	if !ok1 || !ok2 {
		return TempTrend{Direction: "stable", Rate: 0}
	}

	diff := cpuNow - cpuPrev
	rate := diff // 每 checkInterval 变化

	switch {
	case diff > 2:
		return TempTrend{Direction: "rising", Rate: rate}
	case diff < -2:
		return TempTrend{Direction: "falling", Rate: rate}
	default:
		return TempTrend{Direction: "stable", Rate: rate}
	}
}

// Refresh 刷新温度数据
func (m *Manager) Refresh() {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 记录历史
	temps := make(map[string]float64)
	for _, z := range m.zones {
		temps[z.Name] = z.Temp
	}
	m.history = append(m.history, TempHistory{
		Timestamp: time.Now(),
		Temps:     temps,
	})
	if len(m.history) > m.maxHist {
		m.history = m.history[len(m.history)-m.maxHist:]
	}

	// 检查告警
	m.checkAlerts()

	// 应用风扇策略
	m.applyFanPolicy()

	m.logger.Debug("温控刷新完成",
		zap.Float64("cpu", m.zones[0].Temp),
		zap.Int("alerts", len(m.alerts)))
}

// checkAlerts 检查温度告警
func (m *Manager) checkAlerts() {
	for _, z := range m.zones {
		var level, msg string
		var thresh float64

		switch {
		case z.Temp >= m.policy.CritThresh:
			level = "critical"
			thresh = m.policy.CritThresh
			msg = fmt.Sprintf("%s 温度 %.1f°C 达到临界值（阈值 %.0f°C）", z.Name, z.Temp, thresh)
		case z.Temp >= m.policy.HotThresh:
			level = "hot"
			thresh = m.policy.HotThresh
			msg = fmt.Sprintf("%s 温度 %.1f°C 过高（阈值 %.0f°C）", z.Name, z.Temp, thresh)
		case z.Temp >= m.policy.WarmThresh:
			level = "warm"
			thresh = m.policy.WarmThresh
			msg = fmt.Sprintf("%s 温度 %.1f°C 偏高（阈值 %.0f°C）", z.Name, z.Temp, thresh)
		default:
			continue
		}

		alert := ThermalAlert{
			Zone:      z.Name,
			Temp:      z.Temp,
			Threshold: thresh,
			Level:     level,
			Message:   msg,
			Time:      time.Now(),
		}
		m.alerts = append(m.alerts, alert)
		if len(m.alerts) > m.maxAlert {
			m.alerts = m.alerts[len(m.alerts)-m.maxAlert:]
		}

		m.logger.Warn("温度告警", zap.String("zone", z.Name), zap.String("level", level))
	}
}

// applyFanPolicy 根据温度策略调整风扇
func (m *Manager) applyFanPolicy() {
	for i, fan := range m.fans {
		if fan.Mode != FanAuto {
			continue
		}
		// 根据 CPU 温度查找风扇曲线
		cpuTemp := float64(30)
		for _, z := range m.zones {
			if z.Name == "CPU" {
				cpuTemp = z.Temp
				break
			}
		}

		target := m.interpolateFanCurve(cpuTemp)
		m.fans[i].Percent = target
		m.fans[i].Speed = int(float64(fan.MaxSpeed) * target / 100)
	}
}

// interpolateFanCurve 根据温度插值计算风扇转速
func (m *Manager) interpolateFanCurve(temp float64) float64 {
	curve := m.policy.FanCurve
	if len(curve) == 0 {
		return 50
	}

	// 低于最低点
	if temp <= curve[0].Temp {
		return curve[0].Target
	}
	// 高于最高点
	if temp >= curve[len(curve)-1].Temp {
		return curve[len(curve)-1].Target
	}
	// 线性插值
	for i := 1; i < len(curve); i++ {
		if temp <= curve[i].Temp {
			ratio := (temp - curve[i-1].Temp) / (curve[i].Temp - curve[i-1].Temp)
			return curve[i-1].Target + ratio*(curve[i].Target-curve[i-1].Target)
		}
	}
	return curve[len(curve)-1].Target
}

// SetFanMode 设置风扇模式
func (m *Manager) SetFanMode(fanID string, mode FanMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, f := range m.fans {
		if f.ID == fanID {
			m.fans[i].Mode = mode
			m.logger.Info("设置风扇模式", zap.String("fan", fanID), zap.String("mode", string(mode)))
			return nil
		}
	}
	return ErrFanNotFound
}

// SetFanSpeed 手动设置风扇转速
func (m *Manager) SetFanSpeed(fanID string, percent float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if percent < 0 || percent > 100 {
		return fmt.Errorf("风扇转速百分比必须在 0-100 之间")
	}

	for i, f := range m.fans {
		if f.ID == fanID {
			if f.Mode != FanManual {
				return fmt.Errorf("风扇 %s 不在手动模式", fanID)
			}
			m.fans[i].Percent = percent
			m.fans[i].Speed = int(float64(f.MaxSpeed) * percent / 100)
			m.logger.Info("设置风扇转速", zap.String("fan", fanID), zap.Float64("percent", percent))
			return nil
		}
	}
	return ErrFanNotFound
}

// GetHistory 获取温度历史
func (m *Manager) GetHistory(minutes int) []TempHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if minutes <= 0 {
		minutes = 60
	}

	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)
	var result []TempHistory
	for _, h := range m.history {
		if h.Timestamp.After(cutoff) {
			result = append(result, h)
		}
	}
	return result
}

// UpdatePolicy 更新温控策略
func (m *Manager) UpdatePolicy(policy ThermalPolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = policy
	m.logger.Info("更新温控策略", zap.String("name", policy.Name))
}

// GetPolicy 获取当前温控策略
func (m *Manager) GetPolicy() ThermalPolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policy
}

// GetAlerts 获取最近告警
func (m *Manager) GetAlerts(limit int) []ThermalAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.alerts) {
		limit = len(m.alerts)
	}
	return m.alerts[len(m.alerts)-limit:]
}

// ClearAlerts 清空告警
func (m *Manager) ClearAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = nil
}

// readFanFromSysfs 从 sysfs 读取风扇信息
func readFanFromSysfs(hwmonPath string) *FanInfo {
	nameFile := filepath.Join(hwmonPath, "fan1_label")
	if _, err := os.Stat(nameFile); err != nil {
		nameFile = filepath.Join(hwmonPath, "name")
	}

	name := "Fan"
	if data, err := os.ReadFile(nameFile); err == nil {
		name = strings.TrimSpace(string(data))
	}

	fan := &FanInfo{
		ID:   filepath.Base(hwmonPath) + "-fan1",
		Name: name,
		Mode: FanAuto,
	}

	// 读取转速
	if data, err := os.ReadFile(filepath.Join(hwmonPath, "fan1_input")); err == nil {
		if val, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			fan.Speed = val
		}
	}

	// 读取最大转速
	if data, err := os.ReadFile(filepath.Join(hwmonPath, "fan1_max")); err == nil {
		if val, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			fan.MaxSpeed = val
		}
	}
	if fan.MaxSpeed > 0 {
		fan.Percent = float64(fan.Speed) / float64(fan.MaxSpeed) * 100
	}

	return fan
}

// ParseSensorsOutput 解析 lm-sensors 输出
func ParseSensorsOutput(output string) []ThermalZone {
	var zones []ThermalZone
	scanner := bufio.NewScanner(strings.NewReader(output))

	var currentName string
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" {
			currentName = ""
			continue
		}

		// 设备标题行
		if !strings.Contains(line, ":") && !strings.HasPrefix(line, " ") {
			currentName = line
			continue
		}

		// 温度行
		if strings.Contains(line, "°C") && strings.Contains(line, ":") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) != 2 {
				continue
			}
			label := strings.TrimSpace(parts[0])
			tempStr := strings.TrimSpace(parts[1])

			// 提取温度值
			idx := strings.Index(tempStr, "°C")
			if idx < 0 {
				continue
			}
			temp, err := strconv.ParseFloat(strings.TrimSpace(tempStr[:idx]), 64)
			if err != nil {
				continue
			}

			name := label
			if currentName != "" {
				name = currentName + "/" + label
			}

			zones = append(zones, ThermalZone{
				ID:        fmt.Sprintf("sensors-%d", len(zones)),
				Name:      name,
				Type:      label,
				Temp:      temp,
				Status:    StatusNormal,
				UpdatedAt: time.Now(),
			})
		}
	}

	return zones
}

// SortZonesByTemp 按温度降序排列区域
func SortZonesByTemp(zones []ThermalZone) []ThermalZone {
	sorted := make([]ThermalZone, len(zones))
	copy(sorted, zones)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Temp > sorted[j].Temp
	})
	return sorted
}
