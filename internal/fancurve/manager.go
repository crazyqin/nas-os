// Package fancurve 提供风扇曲线控制功能
// 温度-转速曲线定义、预设方案、平滑算法、滞后控制、多传感器加权
package fancurve

import (
	"encoding/json"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"
)

// ========== 类型定义 ==========

// CurveProfile 曲线方案
type CurveProfile struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Points      []CurvePoint `json:"points"`
	IsDefault   bool         `json:"isDefault"`
	CreatedAt   time.Time    `json:"createdAt"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// CurvePoint 曲线点：温度 → 占空比
type CurvePoint struct {
	Temp float64 `json:"temp"` // 温度 (°C)
	Duty float64 `json:"duty"` // 占空比 (0-100%)
}

// FanChannel 风扇通道
type FanChannel struct {
	ID         string  `json:"id"`
	Name       string  `json:"name"`
	CurrentRPM int     `json:"currentRpm"`
	CurrentDuty float64 `json:"currentDuty"`
	MaxRPM     int     `json:"maxRpm"`
}

// TempSource 温度源
type TempSource struct {
	ID          string  `json:"id"`
	Name        string  `json:"name"`
	CurrentTemp float64 `json:"currentTemp"`
	Type        string  `json:"type"` // CPU/主板/HDD/SSD
}

// HysteresisConfig 滞后配置
type HysteresisConfig struct {
	TempDelta    float64 `json:"tempDelta"`    // 温度滞后值 (°C)
	ResponseDelay float64 `json:"responseDelay"` // 响应延迟 (秒)
}

// SmoothingConfig 平滑配置
type SmoothingConfig struct {
	WindowSize int    `json:"windowSize"` // 平滑窗口大小
	Algorithm  string `json:"algorithm"`  // 移动平均/指数平滑: moving_avg, exponential
}

// WeightedSensor 加权传感器
type WeightedSensor struct {
	SensorID string  `json:"sensorId"`
	Weight   float64 `json:"weight"` // 权重 (0-1)
}

// CurveRecord 历史记录
type CurveRecord struct {
	Timestamp time.Time `json:"timestamp"`
	Temp      float64   `json:"temp"`
	Duty      float64   `json:"duty"`
	RPM       int       `json:"rpm"`
}

// ========== Manager ==========

// Manager 风扇曲线管理器
type Manager struct {
	mu              sync.RWMutex
	profiles        map[string]*CurveProfile
	channels        map[string]*FanChannel
	tempSources     map[string]*TempSource
	activeProfiles  map[string]string       // channelID -> profileID
	hysteresis      HysteresisConfig
	smoothing       SmoothingConfig
	weightedSensors map[string][]WeightedSensor // channelID -> sensors
	history         map[string][]CurveRecord    // channelID -> records
	tempBuffer      map[string][]float64        // channelID -> temp buffer for smoothing
	lastDuty        map[string]float64          // channelID -> last duty
	lastChangeTime  map[string]time.Time        // channelID -> last change time
}

// NewManager 创建管理器
func NewManager() *Manager {
	m := &Manager{
		profiles:        make(map[string]*CurveProfile),
		channels:        make(map[string]*FanChannel),
		tempSources:     make(map[string]*TempSource),
		activeProfiles:  make(map[string]string),
		weightedSensors: make(map[string][]WeightedSensor),
		history:         make(map[string][]CurveRecord),
		tempBuffer:      make(map[string][]float64),
		lastDuty:        make(map[string]float64),
		lastChangeTime:  make(map[string]time.Time),
		hysteresis: HysteresisConfig{
			TempDelta:     2.0,
			ResponseDelay: 5.0,
		},
		smoothing: SmoothingConfig{
			WindowSize: 5,
			Algorithm:  "moving_avg",
		},
	}
	m.initDefaults()
	return m
}

// initDefaults 初始化默认配置
func (m *Manager) initDefaults() {
	// 默认温度源
	m.tempSources["cpu"] = &TempSource{
		ID: "cpu", Name: "CPU 温度", CurrentTemp: 45.0, Type: "CPU",
	}
	m.tempSources["mb"] = &TempSource{
		ID: "mb", Name: "主板温度", CurrentTemp: 38.0, Type: "主板",
	}
	m.tempSources["hdd"] = &TempSource{
		ID: "hdd", Name: "硬盘温度", CurrentTemp: 35.0, Type: "HDD",
	}
	m.tempSources["ssd"] = &TempSource{
		ID: "ssd", Name: "SSD 温度", CurrentTemp: 40.0, Type: "SSD",
	}

	// 默认风扇通道
	m.channels["cpu-fan"] = &FanChannel{
		ID: "cpu-fan", Name: "CPU 风扇", CurrentRPM: 800, CurrentDuty: 35, MaxRPM: 2000,
	}
	m.channels["sys-fan"] = &FanChannel{
		ID: "sys-fan", Name: "系统风扇", CurrentRPM: 500, CurrentDuty: 25, MaxRPM: 1500,
	}
	m.channels["hdd-fan"] = &FanChannel{
		ID: "hdd-fan", Name: "硬盘风扇", CurrentRPM: 400, CurrentDuty: 20, MaxRPM: 1200,
	}

	// 预设曲线方案
	m.profiles["silent"] = &CurveProfile{
		ID: "silent", Name: "静音", Description: "低噪音运行，适合夜间",
		IsDefault: false, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Points: []CurvePoint{
			{Temp: 30, Duty: 10}, {Temp: 40, Duty: 15}, {Temp: 50, Duty: 25},
			{Temp: 60, Duty: 40}, {Temp: 70, Duty: 55}, {Temp: 80, Duty: 70},
		},
	}
	m.profiles["balanced"] = &CurveProfile{
		ID: "balanced", Name: "平衡", Description: "均衡散热与噪音",
		IsDefault: true, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Points: []CurvePoint{
			{Temp: 30, Duty: 15}, {Temp: 40, Duty: 25}, {Temp: 50, Duty: 40},
			{Temp: 60, Duty: 60}, {Temp: 70, Duty: 80}, {Temp: 80, Duty: 100},
		},
	}
	m.profiles["performance"] = &CurveProfile{
		ID: "performance", Name: "性能", Description: "最大散热，适合高负载",
		IsDefault: false, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Points: []CurvePoint{
			{Temp: 30, Duty: 25}, {Temp: 40, Duty: 40}, {Temp: 50, Duty: 60},
			{Temp: 60, Duty: 80}, {Temp: 70, Duty: 95}, {Temp: 80, Duty: 100},
		},
	}
	m.profiles["fullspeed"] = &CurveProfile{
		ID: "fullspeed", Name: "全速", Description: "全速运行，最大散热",
		IsDefault: false, CreatedAt: time.Now(), UpdatedAt: time.Now(),
		Points: []CurvePoint{
			{Temp: 20, Duty: 100}, {Temp: 100, Duty: 100},
		},
	}

	// 默认应用平衡方案
	m.activeProfiles["cpu-fan"] = "balanced"
	m.activeProfiles["sys-fan"] = "balanced"
	m.activeProfiles["hdd-fan"] = "balanced"
}

// ========== 方案管理 ==========

// CreateProfile 创建曲线方案
func (m *Manager) CreateProfile(profile *CurveProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if profile.ID == "" {
		return fmt.Errorf("profile ID cannot be empty")
	}
	if _, exists := m.profiles[profile.ID]; exists {
		return fmt.Errorf("profile %s already exists", profile.ID)
	}

	now := time.Now()
	profile.CreatedAt = now
	profile.UpdatedAt = now
	m.profiles[profile.ID] = profile

	log.Printf("[风扇曲线] 创建方案: %s (%s)", profile.Name, profile.ID)
	return nil
}

// UpdateProfile 更新曲线方案
func (m *Manager) UpdateProfile(profile *CurveProfile) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, ok := m.profiles[profile.ID]
	if !ok {
		return fmt.Errorf("profile %s not found", profile.ID)
	}

	profile.CreatedAt = existing.CreatedAt
	profile.UpdatedAt = time.Now()
	m.profiles[profile.ID] = profile

	log.Printf("[风扇曲线] 更新方案: %s", profile.ID)
	return nil
}

// DeleteProfile 删除曲线方案
func (m *Manager) DeleteProfile(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, ok := m.profiles[id]
	if !ok {
		return fmt.Errorf("profile %s not found", id)
	}
	if profile.IsDefault {
		return fmt.Errorf("cannot delete default profile")
	}

	// 检查是否有通道正在使用
	for chID, pID := range m.activeProfiles {
		if pID == id {
			return fmt.Errorf("profile %s is in use by channel %s", id, chID)
		}
	}

	delete(m.profiles, id)
	log.Printf("[风扇曲线] 删除方案: %s", id)
	return nil
}

// GetProfile 获取曲线方案
func (m *Manager) GetProfile(id string) *CurveProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.profiles[id]
}

// ListProfiles 列出所有方案
func (m *Manager) ListProfiles() []CurveProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profiles := make([]CurveProfile, 0, len(m.profiles))
	for _, p := range m.profiles {
		profiles = append(profiles, *p)
	}
	sort.Slice(profiles, func(i, j int) bool {
		return profiles[i].ID < profiles[j].ID
	})
	return profiles
}

// ========== 通道与方案关联 ==========

// ApplyProfile 将方案应用到通道
func (m *Manager) ApplyProfile(channelID, profileID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.channels[channelID]; !ok {
		return fmt.Errorf("channel %s not found", channelID)
	}
	if _, ok := m.profiles[profileID]; !ok {
		return fmt.Errorf("profile %s not found", profileID)
	}

	m.activeProfiles[channelID] = profileID
	log.Printf("[风扇曲线] 通道 %s 应用方案 %s", channelID, profileID)
	return nil
}

// GetActiveProfile 获取通道的当前方案
func (m *Manager) GetActiveProfile(channelID string) *CurveProfile {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profileID, ok := m.activeProfiles[channelID]
	if !ok {
		return nil
	}
	return m.profiles[profileID]
}

// ========== 通道与传感器 ==========

// ListChannels 列出所有通道
func (m *Manager) ListChannels() []FanChannel {
	m.mu.RLock()
	defer m.mu.RUnlock()

	channels := make([]FanChannel, 0, len(m.channels))
	for _, ch := range m.channels {
		channels = append(channels, *ch)
	}
	return channels
}

// ListTempSources 列出所有温度源
func (m *Manager) ListTempSources() []TempSource {
	m.mu.RLock()
	defer m.mu.RUnlock()

	sources := make([]TempSource, 0, len(m.tempSources))
	for _, s := range m.tempSources {
		sources = append(sources, *s)
	}
	return sources
}

// ========== 配置 ==========

// SetHysteresis 设置滞后配置
func (m *Manager) SetHysteresis(config *HysteresisConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.TempDelta < 0 {
		return fmt.Errorf("temp delta must be non-negative")
	}
	if config.ResponseDelay < 0 {
		return fmt.Errorf("response delay must be non-negative")
	}

	m.hysteresis = *config
	log.Printf("[风扇曲线] 更新滞后配置: delta=%.1f°C, delay=%.1fs", config.TempDelta, config.ResponseDelay)
	return nil
}

// SetSmoothing 设置平滑配置
func (m *Manager) SetSmoothing(config *SmoothingConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.WindowSize < 1 {
		return fmt.Errorf("window size must be at least 1")
	}
	if config.Algorithm != "moving_avg" && config.Algorithm != "exponential" {
		return fmt.Errorf("algorithm must be 'moving_avg' or 'exponential'")
	}

	m.smoothing = *config
	log.Printf("[风扇曲线] 更新平滑配置: window=%d, algo=%s", config.WindowSize, config.Algorithm)
	return nil
}

// SetWeightedSensor 设置通道的加权传感器
func (m *Manager) SetWeightedSensor(channelID string, sensors []WeightedSensor) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.channels[channelID]; !ok {
		return fmt.Errorf("channel %s not found", channelID)
	}

	// 验证传感器存在
	for _, ws := range sensors {
		if _, ok := m.tempSources[ws.SensorID]; !ok {
			return fmt.Errorf("sensor %s not found", ws.SensorID)
		}
	}

	m.weightedSensors[channelID] = sensors
	log.Printf("[风扇曲线] 通道 %s 设置 %d 个加权传感器", channelID, len(sensors))
	return nil
}

// ========== 温度计算 ==========

// UpdateTemperature 更新温度源
func (m *Manager) UpdateTemperature(sensorID string, temp float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	source, ok := m.tempSources[sensorID]
	if !ok {
		return fmt.Errorf("sensor %s not found", sensorID)
	}

	source.CurrentTemp = temp

	// 更新所有关联通道
	for chID, ch := range m.channels {
		weightedTemp := m.calculateWeightedTemp(chID)
		smoothedTemp := m.smoothTemp(chID, weightedTemp)

		// 滞后检查
		if !m.checkHysteresis(chID, smoothedTemp) {
			continue
		}

		// 获取当前方案
		profileID, ok := m.activeProfiles[chID]
		if !ok {
			continue
		}
		profile, ok := m.profiles[profileID]
		if !ok {
			continue
		}

		// 计算目标占空比
		duty := m.interpolate(profile.Points, smoothedTemp)

		// 应用
		ch.CurrentDuty = duty
		ch.CurrentRPM = int(float64(ch.MaxRPM) * duty / 100.0)

		// 记录历史
		m.history[chID] = append(m.history[chID], CurveRecord{
			Timestamp: time.Now(),
			Temp:      smoothedTemp,
			Duty:      duty,
			RPM:       ch.CurrentRPM,
		})

		// 限制历史记录数量
		if len(m.history[chID]) > 1000 {
			m.history[chID] = m.history[chID][len(m.history[chID])-1000:]
		}

		m.lastDuty[chID] = duty
	}

	return nil
}

// calculateWeightedTemp 计算加权温度
func (m *Manager) calculateWeightedTemp(channelID string) float64 {
	sensors, ok := m.weightedSensors[channelID]
	if !ok || len(sensors) == 0 {
		// 默认使用第一个温度源
		for _, src := range m.tempSources {
			return src.CurrentTemp
		}
		return 0
	}

	totalWeight := 0.0
	weightedSum := 0.0
	for _, ws := range sensors {
		if src, ok := m.tempSources[ws.SensorID]; ok {
			weightedSum += src.CurrentTemp * ws.Weight
			totalWeight += ws.Weight
		}
	}

	if totalWeight == 0 {
		return 0
	}
	return weightedSum / totalWeight
}

// smoothTemp 温度平滑
func (m *Manager) smoothTemp(channelID string, newTemp float64) float64 {
	buffer, ok := m.tempBuffer[channelID]
	if !ok {
		buffer = make([]float64, 0, m.smoothing.WindowSize)
	}

	buffer = append(buffer, newTemp)
	if len(buffer) > m.smoothing.WindowSize {
		buffer = buffer[len(buffer)-m.smoothing.WindowSize:]
	}
	m.tempBuffer[channelID] = buffer

	switch m.smoothing.Algorithm {
	case "exponential":
		return m.exponentialSmoothing(buffer)
	default: // moving_avg
		return m.movingAverage(buffer)
	}
}

// movingAverage 移动平均
func (m *Manager) movingAverage(buffer []float64) float64 {
	if len(buffer) == 0 {
		return 0
	}
	sum := 0.0
	for _, v := range buffer {
		sum += v
	}
	return sum / float64(len(buffer))
}

// exponentialSmoothing 指数平滑
func (m *Manager) exponentialSmoothing(buffer []float64) float64 {
	if len(buffer) == 0 {
		return 0
	}
	alpha := 2.0 / float64(len(buffer)+1)
	result := buffer[0]
	for i := 1; i < len(buffer); i++ {
		result = alpha*buffer[i] + (1-alpha)*result
	}
	return result
}

// checkHysteresis 滞后检查
func (m *Manager) checkHysteresis(channelID string, newTemp float64) bool {
	// 响应延迟检查
	if lastChange, ok := m.lastChangeTime[channelID]; ok {
		elapsed := time.Since(lastChange).Seconds()
		if elapsed < m.hysteresis.ResponseDelay {
			return false
		}
	}

	// 温度滞后检查：需要温度变化超过滞后阈值才更新
	// 简化实现：允许更新，滞后逻辑由调用方根据实际需求扩展
	m.lastChangeTime[channelID] = time.Now()
	return true
}

// interpolate 曲线插值
func (m *Manager) interpolate(points []CurvePoint, temp float64) float64 {
	if len(points) == 0 {
		return 50
	}

	// 排序（确保点按温度排序）
	sorted := make([]CurvePoint, len(points))
	copy(sorted, points)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Temp < sorted[j].Temp
	})

	// 低于最低点
	if temp <= sorted[0].Temp {
		return sorted[0].Duty
	}

	// 高于最高点
	if temp >= sorted[len(sorted)-1].Temp {
		return sorted[len(sorted)-1].Duty
	}

	// 线性插值
	for i := 0; i < len(sorted)-1; i++ {
		if temp >= sorted[i].Temp && temp <= sorted[i+1].Temp {
			ratio := (temp - sorted[i].Temp) / (sorted[i+1].Temp - sorted[i].Temp)
			duty := sorted[i].Duty + ratio*(sorted[i+1].Duty-sorted[i].Duty)
			return math.Round(duty*10) / 10 // 保留一位小数
		}
	}

	return 50
}

// ========== 历史记录 ==========

// GetHistory 获取通道历史记录
func (m *Manager) GetHistory(channelID string, duration time.Duration) []CurveRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()

	records, ok := m.history[channelID]
	if !ok {
		return nil
	}

	cutoff := time.Now().Add(-duration)
	result := make([]CurveRecord, 0)
	for _, r := range records {
		if r.Timestamp.After(cutoff) {
			result = append(result, r)
		}
	}
	return result
}

// ========== 导入导出 ==========

// ExportProfile 导出方案为 JSON
func (m *Manager) ExportProfile(id string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profile, ok := m.profiles[id]
	if !ok {
		return nil, fmt.Errorf("profile %s not found", id)
	}

	return json.MarshalIndent(profile, "", "  ")
}

// ImportProfile 从 JSON 导入方案
func (m *Manager) ImportProfile(data []byte) (*CurveProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var profile CurveProfile
	if err := json.Unmarshal(data, &profile); err != nil {
		return nil, fmt.Errorf("invalid profile data: %v", err)
	}

	if profile.ID == "" {
		return nil, fmt.Errorf("profile ID cannot be empty")
	}

	// 检查 ID 冲突
	if _, exists := m.profiles[profile.ID]; exists {
		return nil, fmt.Errorf("profile %s already exists", profile.ID)
	}

	now := time.Now()
	profile.CreatedAt = now
	profile.UpdatedAt = now
	m.profiles[profile.ID] = &profile

	log.Printf("[风扇曲线] 导入方案: %s (%s)", profile.Name, profile.ID)
	return &profile, nil
}

// ========== 预览 ==========

// GetCurvePreviewData 获取曲线预览数据
func (m *Manager) GetCurvePreviewData(profileID string) ([]CurvePoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profile, ok := m.profiles[profileID]
	if !ok {
		return nil, fmt.Errorf("profile %s not found", profileID)
	}

	// 生成详细的预览点（每5度一个点）
	preview := make([]CurvePoint, 0)
	for temp := 20.0; temp <= 100.0; temp += 5 {
		duty := m.interpolate(profile.Points, temp)
		preview = append(preview, CurvePoint{Temp: temp, Duty: duty})
	}
	return preview, nil
}
