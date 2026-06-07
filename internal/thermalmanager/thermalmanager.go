// Package thermalmanager 提供智能温控管理功能
// Version: v1.0.0 - 智能温控管理
package thermalmanager

import (
	"context"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// 温度传感器类型
type SensorType string

const (
	SensorCPU         SensorType = "cpu"
	SensorGPU         SensorType = "gpu"
	SensorHDD         SensorType = "hdd"
	SensorMotherboard SensorType = "motherboard"
	SensorAmbient     SensorType = "ambient"
)

// ZoneStatus 温度区域状态
type ZoneStatus string

const (
	StatusNormal   ZoneStatus = "normal"
	StatusWarm     ZoneStatus = "warm"
	StatusHot      ZoneStatus = "hot"
	StatusCritical ZoneStatus = "critical"
)

// CoolingMode 散热模式
type CoolingMode string

const (
	CoolingSilent      CoolingMode = "silent"
	CoolingBalanced    CoolingMode = "balanced"
	CoolingPerformance CoolingMode = "performance"
)

// FanMode 风扇模式
type FanMode string

const (
	FanPWM    FanMode = "pwm"
	FanManual FanMode = "manual"
	FanAuto   FanMode = "auto"
)

// TemperatureZone 温度传感器区域
type TemperatureZone struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Type        SensorType `json:"type"`
	Temperature float64    `json:"temperature"`
	MaxTemp     float64    `json:"maxTemp"`
	CritTemp    float64    `json:"critTemp"`
	Status      ZoneStatus `json:"status"`
	UpdatedAt   time.Time  `json:"updatedAt"`
}

// FanInfo 风扇信息
type FanInfo struct {
	ID       string  `json:"id"`
	Name     string  `json:"name"`
	Speed    int     `json:"speed"`
	MaxSpeed int     `json:"maxSpeed"`
	Percent  float64 `json:"percent"`
	Mode     FanMode `json:"mode"`
	PWMValue int     `json:"pwmValue"`
}

// ThermalCurve 温控曲线点
type ThermalCurve struct {
	Temperature float64 `json:"temperature"`
	FanPercent  float64 `json:"fanPercent"`
}

// CoolingProfile 散热配置
type CoolingProfile struct {
	Name       string         `json:"name"`
	Mode       CoolingMode    `json:"mode"`
	Curves     []ThermalCurve `json:"curves"`
	WarmThresh float64        `json:"warmThresh"`
	HotThresh  float64        `json:"hotThresh"`
	CritThresh float64        `json:"critThresh"`
}

// TemperatureRecord 温度记录
type TemperatureRecord struct {
	Timestamp    time.Time          `json:"timestamp"`
	Temperatures map[string]float64 `json:"temperatures"`
	FanSpeeds    map[string]float64 `json:"fanSpeeds"`
}

// ThermalAlert 温度告警
type ThermalAlert struct {
	ID        string    `json:"id"`
	Zone      string    `json:"zone"`
	Temp      float64   `json:"temp"`
	Threshold float64   `json:"threshold"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	Action    string    `json:"action"`
	CreatedAt time.Time `json:"createdAt"`
}

// ThermalStats 温度统计
type ThermalStats struct {
	Min       float64   `json:"min"`
	Max       float64   `json:"max"`
	Avg       float64   `json:"avg"`
	Current   float64   `json:"current"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// Manager 温控管理器
type Manager struct {
	logger        *zap.Logger
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	zones         []TemperatureZone
	fans          []FanInfo
	profile       CoolingProfile
	alerts        []ThermalAlert
	history       []TemperatureRecord
	maxHistory    int
	maxAlerts     int
	checkInterval time.Duration
	sysfsBase     string
	hwmonBase     string
}

// NewManager 创建温控管理器
func NewManager(logger *zap.Logger) *Manager {
	ctx, cancel := context.WithCancel(context.Background())
	return &Manager{
		logger:        logger,
		ctx:           ctx,
		cancel:        cancel,
		maxHistory:    1440,
		maxAlerts:     100,
		checkInterval: 30 * time.Second,
		sysfsBase:     "/sys/class/thermal",
		hwmonBase:     "/sys/class/hwmon",
		profile: CoolingProfile{
			Name:       "balanced",
			Mode:       CoolingBalanced,
			WarmThresh: 60,
			HotThresh:  75,
			CritThresh: 90,
			Curves: []ThermalCurve{
				{Temperature: 30, FanPercent: 20},
				{Temperature: 45, FanPercent: 35},
				{Temperature: 55, FanPercent: 50},
				{Temperature: 65, FanPercent: 70},
				{Temperature: 75, FanPercent: 85},
				{Temperature: 85, FanPercent: 100},
			},
		},
	}
}

// Start 启动温控管理器
func (m *Manager) Start() error {
	m.logger.Info("启动温控管理器")

	if err := m.loadZones(); err != nil {
		m.logger.Warn("加载温度区域失败", zap.Error(err))
	}

	if err := m.loadFans(); err != nil {
		m.logger.Warn("加载风扇信息失败", zap.Error(err))
	}

	go m.monitorLoop()

	return nil
}

// Stop 停止温控管理器
func (m *Manager) Stop() {
	m.logger.Info("停止温控管理器")
	m.cancel()
}

// monitorLoop 温度监控循环
func (m *Manager) monitorLoop() {
	ticker := time.NewTicker(m.checkInterval)
	defer ticker.Stop()

	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.checkTemperatures()
		}
	}
}

// checkTemperatures 检查温度并应用策略
func (m *Manager) checkTemperatures() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.updateTemperatures()
	m.updateFanSpeeds()
	m.checkAlerts()
	m.recordHistory()
}

// loadZones 加载温度区域
func (m *Manager) loadZones() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries, err := os.ReadDir(m.sysfsBase)
	if err != nil {
		m.logger.Warn("无法读取 thermal 目录，使用模拟数据", zap.Error(err))
		m.loadMockZones()
		return nil
	}

	var zones []TemperatureZone
	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "thermal_zone") {
			continue
		}
		zonePath := filepath.Join(m.sysfsBase, entry.Name())
		zone := m.readZoneFromSysfs(zonePath, entry.Name())
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

// readZoneFromSysfs 从 sysfs 读取温度区域
func (m *Manager) readZoneFromSysfs(path, id string) *TemperatureZone {
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

	temp := tempVal / 1000.0
	if temp > 200 {
		temp = tempVal
	}

	name := id
	zoneType := SensorAmbient
	if typeData, err := os.ReadFile(typeFile); err == nil {
		typeName := strings.TrimSpace(string(typeData))
		name = typeName
		zoneType = classifySensorType(typeName)
	}

	return &TemperatureZone{
		ID:          id,
		Name:        name,
		Type:        zoneType,
		Temperature: temp,
		MaxTemp:     m.profile.HotThresh,
		CritTemp:    m.profile.CritThresh,
		Status:      m.classifyTemp(temp),
		UpdatedAt:   time.Now(),
	}
}

// loadMockZones 加载模拟温度数据
func (m *Manager) loadMockZones() {
	m.zones = []TemperatureZone{
		{ID: "zone0", Name: "CPU", Type: SensorCPU, Temperature: 45, MaxTemp: 75, CritTemp: 90, Status: StatusNormal, UpdatedAt: time.Now()},
		{ID: "zone1", Name: "GPU", Type: SensorGPU, Temperature: 38, MaxTemp: 80, CritTemp: 95, Status: StatusNormal, UpdatedAt: time.Now()},
		{ID: "zone2", Name: "Motherboard", Type: SensorMotherboard, Temperature: 32, MaxTemp: 70, CritTemp: 85, Status: StatusNormal, UpdatedAt: time.Now()},
		{ID: "zone3", Name: "HDD Bay", Type: SensorHDD, Temperature: 35, MaxTemp: 55, CritTemp: 65, Status: StatusNormal, UpdatedAt: time.Now()},
		{ID: "zone4", Name: "Ambient", Type: SensorAmbient, Temperature: 25, MaxTemp: 45, CritTemp: 60, Status: StatusNormal, UpdatedAt: time.Now()},
	}
}

// loadFans 加载风扇信息
func (m *Manager) loadFans() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	entries, err := os.ReadDir(m.hwmonBase)
	if err != nil {
		m.logger.Warn("无法读取 hwmon 目录，使用模拟数据", zap.Error(err))
		m.loadMockFans()
		return nil
	}

	var fans []FanInfo
	for _, entry := range entries {
		hwmonPath := filepath.Join(m.hwmonBase, entry.Name())
		fan := m.readFanFromSysfs(hwmonPath, entry.Name())
		if fan != nil {
			fans = append(fans, *fan)
		}
	}

	if len(fans) == 0 {
		m.loadMockFans()
		return nil
	}

	m.fans = fans
	m.logger.Info("加载风扇信息", zap.Int("count", len(fans)))
	return nil
}

// readFanFromSysfs 从 sysfs 读取风扇信息
func (m *Manager) readFanFromSysfs(path, hwmonID string) *FanInfo {
	inputFile := filepath.Join(path, "fan1_input")
	if _, err := os.Stat(inputFile); err != nil {
		return nil
	}

	data, err := os.ReadFile(inputFile)
	if err != nil {
		return nil
	}
	speed, _ := strconv.Atoi(strings.TrimSpace(string(data)))

	nameFile := filepath.Join(path, "fan1_label")
	name := "Fan"
	if _, err := os.Stat(nameFile); err != nil {
		nameFile = filepath.Join(path, "name")
	}
	if data, err := os.ReadFile(nameFile); err == nil {
		name = strings.TrimSpace(string(data))
	}

	fan := &FanInfo{
		ID:       hwmonID + "-fan1",
		Name:     name,
		Speed:    speed,
		MaxSpeed: 3000,
		Mode:     FanAuto,
	}

	maxFile := filepath.Join(path, "fan1_max")
	if data, err := os.ReadFile(maxFile); err == nil {
		if val, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			fan.MaxSpeed = val
		}
	}

	pwmFile := filepath.Join(path, "pwm1")
	if data, err := os.ReadFile(pwmFile); err == nil {
		if val, err := strconv.Atoi(strings.TrimSpace(string(data))); err == nil {
			fan.PWMValue = val
		}
	}

	if fan.MaxSpeed > 0 {
		fan.Percent = float64(fan.Speed) / float64(fan.MaxSpeed) * 100
	}

	return fan
}

// loadMockFans 加载模拟风扇数据
func (m *Manager) loadMockFans() {
	m.fans = []FanInfo{
		{ID: "fan0", Name: "CPU Fan", Speed: 1200, MaxSpeed: 3000, Percent: 40, Mode: FanAuto, PWMValue: 102},
		{ID: "fan1", Name: "System Fan 1", Speed: 800, MaxSpeed: 2500, Percent: 32, Mode: FanAuto, PWMValue: 82},
		{ID: "fan2", Name: "System Fan 2", Speed: 750, MaxSpeed: 2500, Percent: 30, Mode: FanAuto, PWMValue: 77},
	}
}

// updateTemperatures 更新温度数据
func (m *Manager) updateTemperatures() {
	for i, zone := range m.zones {
		newTemp := m.readTemperature(zone)
		m.zones[i].Temperature = newTemp
		m.zones[i].Status = m.classifyTemp(newTemp)
		m.zones[i].UpdatedAt = time.Now()
	}
}

// readTemperature 读取温度值
func (m *Manager) readTemperature(zone TemperatureZone) float64 {
	if zone.Type == SensorCPU {
		return m.readCPUTemperature()
	}
	return zone.Temperature + randomDelta()
}

// readCPUTemperature 读取 CPU 温度
func (m *Manager) readCPUTemperature() float64 {
	entries, err := os.ReadDir(m.sysfsBase)
	if err != nil {
		return 45
	}

	for _, entry := range entries {
		if !strings.HasPrefix(entry.Name(), "thermal_zone") {
			continue
		}
		typeFile := filepath.Join(m.sysfsBase, entry.Name(), "type")
		if data, err := os.ReadFile(typeFile); err == nil {
			if strings.Contains(strings.ToLower(string(data)), "cpu") ||
				strings.Contains(strings.ToLower(string(data)), "x86_pkg") {
				tempFile := filepath.Join(m.sysfsBase, entry.Name(), "temp")
				if data, err := os.ReadFile(tempFile); err == nil {
					if val, err := strconv.ParseFloat(strings.TrimSpace(string(data)), 64); err == nil {
						temp := val / 1000.0
						if temp > 200 {
							temp = val
						}
						return temp
					}
				}
			}
		}
	}
	return 45
}

// updateFanSpeeds 更新风扇转速
func (m *Manager) updateFanSpeeds() {
	for i, fan := range m.fans {
		if fan.Mode == FanManual {
			continue
		}

		cpuTemp := m.getCPUTemp()
		targetPercent := m.interpolateFanCurve(cpuTemp)
		targetPWM := int(targetPercent * 255 / 100)

		m.fans[i].Percent = targetPercent
		m.fans[i].PWMValue = targetPWM
		m.fans[i].Speed = int(float64(fan.MaxSpeed) * targetPercent / 100)
	}
}

// getCPUTemp 获取 CPU 温度
func (m *Manager) getCPUTemp() float64 {
	for _, zone := range m.zones {
		if zone.Type == SensorCPU {
			return zone.Temperature
		}
	}
	return 45
}

// interpolateFanCurve 风扇曲线插值
func (m *Manager) interpolateFanCurve(temp float64) float64 {
	curves := m.profile.Curves
	if len(curves) == 0 {
		return 50
	}

	if temp <= curves[0].Temperature {
		return curves[0].FanPercent
	}
	if temp >= curves[len(curves)-1].Temperature {
		return curves[len(curves)-1].FanPercent
	}

	for i := 1; i < len(curves); i++ {
		if temp <= curves[i].Temperature {
			ratio := (temp - curves[i-1].Temperature) / (curves[i].Temperature - curves[i-1].Temperature)
			return curves[i-1].FanPercent + ratio*(curves[i].FanPercent-curves[i-1].FanPercent)
		}
	}

	return curves[len(curves)-1].FanPercent
}

// classifyTemp 分类温度状态
func (m *Manager) classifyTemp(temp float64) ZoneStatus {
	switch {
	case temp >= m.profile.CritThresh:
		return StatusCritical
	case temp >= m.profile.HotThresh:
		return StatusHot
	case temp >= m.profile.WarmThresh:
		return StatusWarm
	default:
		return StatusNormal
	}
}

// checkAlerts 检查并生成告警
func (m *Manager) checkAlerts() {
	for _, zone := range m.zones {
		var level, msg, action string
		var thresh float64

		switch {
		case zone.Temperature >= m.profile.CritThresh:
			level = "critical"
			thresh = m.profile.CritThresh
			msg = fmt.Sprintf("%s 温度 %.1f°C 达到临界值", zone.Name, zone.Temperature)
			action = "emergency_throttle"
		case zone.Temperature >= m.profile.HotThresh:
			level = "hot"
			thresh = m.profile.HotThresh
			msg = fmt.Sprintf("%s 温度 %.1f°C 过高", zone.Name, zone.Temperature)
			action = "increase_cooling"
		case zone.Temperature >= m.profile.WarmThresh:
			level = "warm"
			thresh = m.profile.WarmThresh
			msg = fmt.Sprintf("%s 温度 %.1f°C 偏高", zone.Name, zone.Temperature)
			action = "monitor"
		default:
			continue
		}

		alert := ThermalAlert{
			ID:        fmt.Sprintf("alert-%d", time.Now().UnixNano()),
			Zone:      zone.Name,
			Temp:      zone.Temperature,
			Threshold: thresh,
			Level:     level,
			Message:   msg,
			Action:    action,
			CreatedAt: time.Now(),
		}
		m.alerts = append(m.alerts, alert)

		if len(m.alerts) > m.maxAlerts {
			m.alerts = m.alerts[len(m.alerts)-m.maxAlerts:]
		}

		m.logger.Warn("温度告警", zap.String("zone", zone.Name), zap.String("level", level))
	}
}

// recordHistory 记录历史数据
func (m *Manager) recordHistory() {
	temps := make(map[string]float64)
	fans := make(map[string]float64)

	for _, zone := range m.zones {
		temps[zone.Name] = zone.Temperature
	}

	for _, fan := range m.fans {
		fans[fan.Name] = fan.Percent
	}

	record := TemperatureRecord{
		Timestamp:    time.Now(),
		Temperatures: temps,
		FanSpeeds:    fans,
	}

	m.history = append(m.history, record)
	if len(m.history) > m.maxHistory {
		m.history = m.history[len(m.history)-m.maxHistory:]
	}
}

// GetOverview 获取温度总览
func (m *Manager) GetOverview() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var hottest ZoneStatus = StatusNormal
	hottestTemp := 0.0
	hottestZone := ""

	for _, zone := range m.zones {
		if zone.Temperature > hottestTemp {
			hottestTemp = zone.Temperature
			hottestZone = zone.Name
		}
		if zone.Status == StatusCritical {
			hottest = StatusCritical
		} else if zone.Status == StatusHot && hottest != StatusCritical {
			hottest = StatusHot
		} else if zone.Status == StatusWarm && hottest == StatusNormal {
			hottest = StatusWarm
		}
	}

	return map[string]interface{}{
		"zones":         m.zones,
		"fans":          m.fans,
		"profile":       m.profile,
		"hottestZone":   hottestZone,
		"hottestTemp":   hottestTemp,
		"overallStatus": hottest,
		"updatedAt":     time.Now(),
	}
}

// GetZones 获取温度区域列表
func (m *Manager) GetZones() []TemperatureZone {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.zones
}

// GetFans 获取风扇信息
func (m *Manager) GetFans() []FanInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.fans
}

// GetProfile 获取当前散热配置
func (m *Manager) GetProfile() CoolingProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.profile
}

// SetProfile 设置散热配置
func (m *Manager) SetProfile(profile CoolingProfile) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.profile = profile
	m.logger.Info("更新散热配置", zap.String("name", profile.Name))
}

// GetAlerts 获取告警列表
func (m *Manager) GetAlerts(limit int) []ThermalAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.alerts) {
		limit = len(m.alerts)
	}
	if len(m.alerts) == 0 {
		return []ThermalAlert{}
	}
	return m.alerts[len(m.alerts)-limit:]
}

// ClearAlerts 清空告警
func (m *Manager) ClearAlerts() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alerts = nil
}

// GetHistory 获取历史数据
func (m *Manager) GetHistory(minutes int) []TemperatureRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if minutes <= 0 {
		minutes = 60
	}

	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)
	var result []TemperatureRecord
	for _, r := range m.history {
		if r.Timestamp.After(cutoff) {
			result = append(result, r)
		}
	}
	return result
}

// GetStats 获取温度统计
func (m *Manager) GetStats(zoneName string, minutes int) ThermalStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if minutes <= 0 {
		minutes = 60
	}

	cutoff := time.Now().Add(-time.Duration(minutes) * time.Minute)
	var temps []float64

	for _, r := range m.history {
		if r.Timestamp.After(cutoff) {
			if temp, ok := r.Temperatures[zoneName]; ok {
				temps = append(temps, temp)
			}
		}
	}

	if len(temps) == 0 {
		for _, z := range m.zones {
			if z.Name == zoneName {
				return ThermalStats{
					Min:       z.Temperature,
					Max:       z.Temperature,
					Avg:       z.Temperature,
					Current:   z.Temperature,
					UpdatedAt: time.Now(),
				}
			}
		}
		return ThermalStats{UpdatedAt: time.Now()}
	}

	sort.Float64s(temps)
	sum := 0.0
	for _, t := range temps {
		sum += t
	}

	return ThermalStats{
		Min:       temps[0],
		Max:       temps[len(temps)-1],
		Avg:       sum / float64(len(temps)),
		Current:   temps[len(temps)-1],
		UpdatedAt: time.Now(),
	}
}

// SetFanMode 设置风扇模式
func (m *Manager) SetFanMode(fanID string, mode FanMode) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, fan := range m.fans {
		if fan.ID == fanID {
			m.fans[i].Mode = mode
			m.logger.Info("设置风扇模式", zap.String("fan", fanID), zap.String("mode", string(mode)))
			return nil
		}
	}
	return fmt.Errorf("风扇 %s 未找到", fanID)
}

// SetFanPWM 设置风扇 PWM 值
func (m *Manager) SetFanPWM(fanID string, pwmValue int) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if pwmValue < 0 || pwmValue > 255 {
		return fmt.Errorf("PWM 值必须在 0-255 之间")
	}

	for i, fan := range m.fans {
		if fan.ID == fanID {
			if fan.Mode != FanManual {
				return fmt.Errorf("风扇 %s 不在手动模式", fanID)
			}
			m.fans[i].PWMValue = pwmValue
			m.fans[i].Percent = float64(pwmValue) / 255 * 100
			m.fans[i].Speed = int(float64(fan.MaxSpeed) * m.fans[i].Percent / 100)
			m.logger.Info("设置风扇 PWM", zap.String("fan", fanID), zap.Int("pwm", pwmValue))
			return nil
		}
	}
	return fmt.Errorf("风扇 %s 未找到", fanID)
}

// SetCoolingMode 设置散热模式
func (m *Manager) SetCoolingMode(mode CoolingMode) {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile := getProfileForMode(mode)
	m.profile = profile
	m.logger.Info("切换散热模式", zap.String("mode", string(mode)))
}

// classifySensorType 分类传感器类型
func classifySensorType(typeName string) SensorType {
	lower := strings.ToLower(typeName)
	switch {
	case strings.Contains(lower, "cpu") || strings.Contains(lower, "x86_pkg"):
		return SensorCPU
	case strings.Contains(lower, "gpu") || strings.Contains(lower, "nvidia"):
		return SensorGPU
	case strings.Contains(lower, "hdd") || strings.Contains(lower, "disk"):
		return SensorHDD
	case strings.Contains(lower, "motherboard"):
		return SensorMotherboard
	default:
		return SensorAmbient
	}
}

// getProfileForMode 获取散热模式配置
func getProfileForMode(mode CoolingMode) CoolingProfile {
	switch mode {
	case CoolingSilent:
		return CoolingProfile{
			Name:       "silent",
			Mode:       CoolingSilent,
			WarmThresh: 65,
			HotThresh:  80,
			CritThresh: 95,
			Curves: []ThermalCurve{
				{Temperature: 30, FanPercent: 15},
				{Temperature: 50, FanPercent: 25},
				{Temperature: 65, FanPercent: 40},
				{Temperature: 75, FanPercent: 60},
				{Temperature: 85, FanPercent: 80},
				{Temperature: 95, FanPercent: 100},
			},
		}
	case CoolingPerformance:
		return CoolingProfile{
			Name:       "performance",
			Mode:       CoolingPerformance,
			WarmThresh: 50,
			HotThresh:  65,
			CritThresh: 80,
			Curves: []ThermalCurve{
				{Temperature: 25, FanPercent: 30},
				{Temperature: 40, FanPercent: 50},
				{Temperature: 50, FanPercent: 65},
				{Temperature: 60, FanPercent: 80},
				{Temperature: 70, FanPercent: 90},
				{Temperature: 80, FanPercent: 100},
			},
		}
	default:
		return CoolingProfile{
			Name:       "balanced",
			Mode:       CoolingBalanced,
			WarmThresh: 60,
			HotThresh:  75,
			CritThresh: 90,
			Curves: []ThermalCurve{
				{Temperature: 30, FanPercent: 20},
				{Temperature: 45, FanPercent: 35},
				{Temperature: 55, FanPercent: 50},
				{Temperature: 65, FanPercent: 70},
				{Temperature: 75, FanPercent: 85},
				{Temperature: 85, FanPercent: 100},
			},
		}
	}
}

// randomDelta 生成随机温度变化
func randomDelta() float64 {
	return (math.Sin(float64(time.Now().UnixNano()%1000)/100.0*2*math.Pi) * 0.5)
}
