// Package smartfan 提供智能风扇控制功能
// 温度监控、PID 控制、多区域联动、静音模式、故障检测
package smartfan

import (
	"fmt"
	"log"
	"math"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Controller 智能风扇控制器
type Controller struct {
	mu          sync.RWMutex
	zones       map[string]*ThermalZone  // 温度区域
	fans        map[string]*FanDevice    // 风扇设备
	profiles    map[string]*FanProfile   // 风扇配置
	policies    map[string]*FanPolicy    // 风扇策略
	alerts      []FanAlert               // 告警列表
	history     []HistoryRecord          // 历史记录
	maxHistory  int                      // 最大历史记录数
	maxAlerts   int                      // 最大告警数
	activeProfileID string              // 当前活跃配置 ID
	activePolicyID  string              // 当前活跃策略 ID
	pidControllers map[string]*PIDController // PID 控制器 (zone_id -> controller)
	logger      *zap.Logger
	stopCh      chan struct{}
	running     bool
	collectInterval time.Duration
}

// PIDController PID 控制器实现
// 支持积分分离、微分滤波、死区控制等高级特性
// 适用于温度-风扇转速的闭环控制场景
//
// 算法原理:
//   output = Kp * error + Ki * ∫error·dt + Kd * d(error)/dt
//   - 积分分离: 当误差超过阈值时暂停积分累积，防止积分饱和
//   - 微分滤波: 对微分项进行低通滤波，减少传感器噪声影响
//   - 死区控制: 误差在死区范围内时不调整，避免频繁抖动
//   - 积分衰减: 长期无误差时逐渐衰减积分项，防止过冲
//
// 典型配置:
//   - CPU散热: Kp=2.0, Ki=0.5, Kd=1.0, 目标温度60°C
//   - GPU散热: Kp=1.5, Ki=0.3, Kd=0.8, 目标温度65°C
//   - NVMe散热: Kp=2.5, Ki=0.6, Kd=1.2, 目标温度55°C
type PIDController struct {
	config       PIDConfig
	integral     float64   // 积分累积值
	prevError    float64   // 上一次误差
	lastTime     time.Time // 上一次计算时间
	filteredD    float64   // 微分项低通滤波后的值
	inDeadZone   bool      // 是否在死区内
	stableCount  int       // 稳定状态计数（用于积分衰减）
}

// NewPIDController 创建 PID 控制器
// config: PID 参数配置，包含 Kp/Ki/Kd 增益系数和目标值
func NewPIDController(config PIDConfig) *PIDController {
	return &PIDController{
		config: config,
	}
}

// Compute 计算 PID 输出
// current: 当前温度值（°C）
// 返回: 风扇转速百分比 (0.0 - 100.0)
//
// 控制逻辑:
//   1. 检查死区 -> 误差足够小时保持当前输出
//   2. 积分分离 -> 误差过大时暂停积分，防止超调
//   3. 微分滤波 -> 使用一阶低通滤波器平滑微分项
//   4. 积分衰减 -> 稳定状态下逐渐衰减积分，防止过冲
//   5. 输出限幅 -> 确保输出在 [MinOutput, MaxOutput] 范围内
func (p *PIDController) Compute(current float64) float64 {
	now := time.Now()
	dt := now.Sub(p.lastTime).Seconds()
	if dt <= 0 {
		dt = 1.0
	}
	p.lastTime = now

	// 计算误差 (温度越高，误差越大，输出越大)
	errorVal := current - p.config.SetPoint

	// 死区控制: 误差在死区范围内时不调整，避免频繁抖动
	// 适用于温度在目标值附近小幅波动的场景
	if p.config.DeadZone > 0 && math.Abs(errorVal) <= p.config.DeadZone {
		if !p.inDeadZone {
			// 进入死区，记录状态
			p.inDeadZone = true
		}
		// 在死区内保持上次输出，不进行积分累积
		return p.clampOutput(p.config.Kp * errorVal + p.config.Ki*p.integral + p.config.Kd*p.filteredD)
	}
	p.inDeadZone = false

	// 积分分离: 当误差超过阈值时暂停积分累积
	// 防止在大误差情况下积分项过大导致超调
	integralSeparation := true
	if p.config.IntegralSepThreshold > 0 {
		integralSeparation = math.Abs(errorVal) <= p.config.IntegralSepThreshold
	}

	if integralSeparation {
		// 正常积分累积
		p.integral += errorVal * dt

		// 积分衰减: 长期稳定时逐渐衰减积分项
		// 防止积分项过大导致的过冲和振荡
		if p.config.IntegralDecay > 0 && p.config.IntegralDecay < 1.0 {
			if math.Abs(errorVal) < p.config.DeadZone*2 {
				p.stableCount++
				// 每10个稳定周期衰减一次积分
				if p.stableCount >= 10 {
					p.integral *= p.config.IntegralDecay
					p.stableCount = 0
				}
			} else {
				p.stableCount = 0
			}
		}
	}

	// 积分限幅 (抗饱和)
	// 限制积分项的范围，防止长时间累积导致的积分饱和
	if p.config.IntegralLimit > 0 {
		p.integral = math.Max(-p.config.IntegralLimit, math.Min(p.config.IntegralLimit, p.integral))
	} else {
		p.integral = math.Max(-100, math.Min(100, p.integral))
	}
	integral := p.config.Ki * p.integral

	// 比例项
	proportional := p.config.Kp * errorVal

	// 微分项 (带低通滤波)
	// 使用一阶指数移动平均滤波器平滑微分项
	// 减少温度传感器噪声导致的风扇抖动
	derivativeRaw := (errorVal - p.prevError) / dt
	if p.config.DerivativeFilterAlpha > 0 && p.config.DerivativeFilterAlpha < 1.0 {
		// 一阶低通滤波: filtered = α * raw + (1-α) * prev_filtered
		p.filteredD = p.config.DerivativeFilterAlpha*derivativeRaw +
			(1-p.config.DerivativeFilterAlpha)*p.filteredD
	} else {
		p.filteredD = derivativeRaw
	}
	derivative := p.config.Kd * p.filteredD
	p.prevError = errorVal

	// 计算输出
	output := proportional + integral + derivative

	// 输出限幅
	return p.clampOutput(output)
}

// clampOutput 将输出限幅到有效范围
func (p *PIDController) clampOutput(output float64) float64 {
	return math.Max(p.config.MinOutput, math.Min(p.config.MaxOutput, output))
}

// Reset 重置 PID 控制器
// 调用后控制器状态完全清空，适用于配置切换或系统重启
func (p *PIDController) Reset() {
	p.integral = 0
	p.prevError = 0
	p.lastTime = time.Time{}
	p.filteredD = 0
	p.inDeadZone = false
	p.stableCount = 0
}

// NewController 创建风扇控制器
func NewController(logger *zap.Logger) *Controller {
	if logger == nil {
		logger = zap.NewNop()
	}

	c := &Controller{
		zones:          make(map[string]*ThermalZone),
		fans:           make(map[string]*FanDevice),
		profiles:       make(map[string]*FanProfile),
		policies:       make(map[string]*FanPolicy),
		pidControllers: make(map[string]*PIDController),
		alerts:         make([]FanAlert, 0),
		history:        make([]HistoryRecord, 0),
		maxHistory:     1440, // 24 小时 (每分钟一次)
		maxAlerts:      1000,
		logger:         logger,
		stopCh:         make(chan struct{}),
		collectInterval: 30 * time.Second,
	}

	// 初始化默认配置
	c.initDefaults()

	return c
}

// initDefaults 初始化默认配置
// 设置静音/均衡/性能三种风扇配置
// 设置默认温度区域 (CPU/GPU/NVMe)
// 设置默认风扇设备
// 初始化各区域的 PID 控制器
func (c *Controller) initDefaults() {
	// 默认风扇配置
	silentProfile := &FanProfile{
		ID:   "silent",
		Name: "静音模式",
		Mode: FanModeSilent,
		Curve: []CurvePoint{
			{Temp: 30, Percent: 20},
			{Temp: 45, Percent: 30},
			{Temp: 55, Percent: 50},
			{Temp: 65, Percent: 70},
			{Temp: 75, Percent: 100},
		},
		IsDefault: true,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	balancedProfile := &FanProfile{
		ID:   "balanced",
		Name: "均衡模式",
		Mode: FanModeBalanced,
		Curve: []CurvePoint{
			{Temp: 30, Percent: 30},
			{Temp: 45, Percent: 45},
			{Temp: 55, Percent: 60},
			{Temp: 65, Percent: 80},
			{Temp: 75, Percent: 100},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	performanceProfile := &FanProfile{
		ID:   "performance",
		Name: "性能模式",
		Mode: FanModePerformance,
		Curve: []CurvePoint{
			{Temp: 30, Percent: 40},
			{Temp: 45, Percent: 55},
			{Temp: 55, Percent: 70},
			{Temp: 65, Percent: 90},
			{Temp: 75, Percent: 100},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	c.profiles["silent"] = silentProfile
	c.profiles["balanced"] = balancedProfile
	c.profiles["performance"] = performanceProfile
	c.activeProfileID = "balanced"

	// 默认策略
	defaultPolicy := &FanPolicy{
		ID:               "default",
		Name:             "默认策略",
		Description:      "均衡散热，夜间自动降速",
		ProfileID:        "balanced",
		MinFanSpeed:      20,
		MaxFanSpeed:      100,
		NightModeEnabled: true,
		NightStart:       "23:00",
		NightEnd:         "07:00",
		NightSpeedLimit:  50,
		IsActive:         true,
		CreatedAt:        time.Now(),
	}
	c.policies["default"] = defaultPolicy
	c.activePolicyID = "default"

	// 默认温度区域
	c.zones["cpu"] = &ThermalZone{
		ID:           "cpu",
		Name:         "CPU",
		Type:         ZoneTypeCPU,
		SensorPath:   "/sys/class/thermal/thermal_zone0/temp",
		CriticalTemp: 85.0,
		WarningTemp:  75.0,
		FanIDs:       []string{"cpu_fan"},
	}

	c.zones["gpu"] = &ThermalZone{
		ID:           "gpu",
		Name:         "GPU",
		Type:         ZoneTypeGPU,
		SensorPath:   "/sys/class/thermal/thermal_zone1/temp",
		CriticalTemp: 90.0,
		WarningTemp:  80.0,
		FanIDs:       []string{"gpu_fan"},
	}

	c.zones["nvme"] = &ThermalZone{
		ID:           "nvme",
		Name:         "NVMe SSD",
		Type:         ZoneTypeNVMe,
		SensorPath:   "/sys/class/thermal/thermal_zone2/temp",
		CriticalTemp: 75.0,
		WarningTemp:  65.0,
		FanIDs:       []string{"case_fan"},
	}

	// 默认风扇
	c.fans["cpu_fan"] = &FanDevice{
		ID:         "cpu_fan",
		Name:       "CPU 风扇",
		PWMPath:    "/sys/class/hwmon/hwmon0/pwm1",
		RPMPath:    "/sys/class/hwmon/hwmon0/fan1_input",
		MaxRPM:     3000,
		MinRPM:     300,
		TempSource: "cpu",
		IsRunning:  true,
	}

	c.fans["gpu_fan"] = &FanDevice{
		ID:         "gpu_fan",
		Name:       "GPU 风扇",
		PWMPath:    "/sys/class/hwmon/hwmon0/pwm2",
		RPMPath:    "/sys/class/hwmon/hwmon0/fan2_input",
		MaxRPM:     2500,
		MinRPM:     300,
		TempSource: "gpu",
		IsRunning:  true,
	}

	c.fans["case_fan"] = &FanDevice{
		ID:         "case_fan",
		Name:       "机箱风扇",
		PWMPath:    "/sys/class/hwmon/hwmon0/pwm3",
		RPMPath:    "/sys/class/hwmon/hwmon0/fan3_input",
		MaxRPM:     2000,
		MinRPM:     200,
		TempSource: "nvme",
		IsRunning:  true,
	}

	// 初始化 PID 控制器
	// 每个温度区域独立的 PID 控制器，目标温度比警告温度低 5°C
	for zoneID, zone := range c.zones {
		pidConfig := DefaultPIDConfig()
		pidConfig.SetPoint = zone.WarningTemp - 5 // 目标温度比警告温度低 5°C

		// 根据区域类型优化 PID 参数
		switch zone.Type {
		case ZoneTypeCPU:
			// CPU 温度变化快，需要较快的响应
			pidConfig.Kp = 2.0
			pidConfig.Ki = 0.5
			pidConfig.Kd = 1.0
			pidConfig.DeadZone = 1.0
			pidConfig.DerivativeFilterAlpha = 0.3
		case ZoneTypeGPU:
			// GPU 负载波动大，需要更强的微分滤波
			pidConfig.Kp = 1.5
			pidConfig.Ki = 0.3
			pidConfig.Kd = 0.8
			pidConfig.DeadZone = 1.5
			pidConfig.DerivativeFilterAlpha = 0.2
		case ZoneTypeNVMe:
			// NVMe 温度变化较慢，可以更温和
			pidConfig.Kp = 2.5
			pidConfig.Ki = 0.6
			pidConfig.Kd = 1.2
			pidConfig.DeadZone = 0.5
			pidConfig.DerivativeFilterAlpha = 0.4
		}

		c.pidControllers[zoneID] = NewPIDController(pidConfig)
	}
}

// Start 启动定时采集和控制
func (c *Controller) Start() {
	c.mu.Lock()
	if c.running {
		c.mu.Unlock()
		return
	}
	c.running = true
	c.stopCh = make(chan struct{})
	c.mu.Unlock()

	go func() {
		// 立即采集一次
		c.collect()

		ticker := time.NewTicker(c.collectInterval)
		defer ticker.Stop()

		for {
			select {
			case <-ticker.C:
				c.collect()
			case <-c.stopCh:
				return
			}
		}
	}()

	c.logger.Info("[智能风扇] 启动定时采集", zap.Duration("interval", c.collectInterval))
}

// Stop 停止定时采集
func (c *Controller) Stop() {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.running {
		return
	}

	c.running = false
	close(c.stopCh)
	c.logger.Info("[智能风扇] 停止定时采集")
}

// collect 采集温度并控制风扇
func (c *Controller) collect() {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()

	// 1. 读取温度传感器
	for zoneID, zone := range c.zones {
		temp, err := c.readTemperature(zone.SensorPath)
		if err != nil {
			c.logger.Warn("[智能风扇] 读取温度失败",
				zap.String("zone", zoneID),
				zap.Error(err))
			continue
		}

		zone.CurrentTemp = temp
		zone.LastUpdated = now

		// 更新统计
		if temp < zone.MinTemp || zone.MinTemp == 0 {
			zone.MinTemp = temp
		}
		if temp > zone.MaxTemp {
			zone.MaxTemp = temp
		}
		// 简单移动平均
		if zone.AvgTemp == 0 {
			zone.AvgTemp = temp
		} else {
			zone.AvgTemp = (zone.AvgTemp*9 + temp) / 10
		}
	}

	// 2. PID 控制计算并设置风扇转速
	for zoneID, zone := range c.zones {
		pid, ok := c.pidControllers[zoneID]
		if !ok {
			continue
		}

		// PID 计算目标转速百分比
		targetPercent := pid.Compute(zone.CurrentTemp)

		// 应用温度-转速曲线修正
		// 当有自定义配置时，使用曲线插值对 PID 输出进行修正
		profile := c.profiles[c.activeProfileID]
		if profile != nil && len(profile.Curve) > 0 {
			curvePercent := c.interpolateCurve(zone.CurrentTemp, profile.Curve)
			// 取 PID 输出和曲线值的较大值，确保安全
			if curvePercent > targetPercent {
				targetPercent = curvePercent
			}
		}

		// 应用夜间模式限制
		if c.isNightMode() {
			policy := c.policies[c.activePolicyID]
			if policy != nil && policy.NightModeEnabled {
				if int(targetPercent) > policy.NightSpeedLimit {
					targetPercent = float64(policy.NightSpeedLimit)
				}
			}
		}

		// 设置关联风扇转速
		for _, fanID := range zone.FanIDs {
			fan, ok := c.fans[fanID]
			if !ok {
				continue
			}

			// 计算 PWM 值 (0-255)
			pwm := int(targetPercent * 255 / 100)
			pwm = max(0, min(255, pwm))

			// 设置 PWM
			if err := c.setFanPWM(fan.PWMPath, pwm); err != nil {
				c.logger.Warn("[智能风扇] 设置 PWM 失败",
					zap.String("fan", fanID),
					zap.Error(err))
			}
			fan.CurrentPWM = pwm

			// 读取实际转速
			rpm, err := c.readFanRPM(fan.RPMPath)
			if err == nil {
				fan.CurrentRPM = rpm
			}

			fan.LastUpdated = now

			// 故障检测
			if fan.IsRunning && pwm > 30 && fan.CurrentRPM < fan.MinRPM/2 {
				c.addAlert(Alert{
					Type:      AlertTypeFanFailure,
					Severity:  AlertSeverityCritical,
					Source:    fanID,
					Message:   fmt.Sprintf("风扇 %s 可能故障: PWM=%d, RPM=%d", fan.Name, pwm, fan.CurrentRPM),
					Value:     float64(fan.CurrentRPM),
					Threshold: float64(fan.MinRPM),
				})
			}
		}

		// 过热检测
		if zone.CurrentTemp >= zone.CriticalTemp {
			c.addAlert(Alert{
				Type:      AlertTypeOverheat,
				Severity:  AlertSeverityCritical,
				Source:    zoneID,
				Message:   fmt.Sprintf("%s 温度过高: %.1f°C (临界: %.1f°C)", zone.Name, zone.CurrentTemp, zone.CriticalTemp),
				Value:     zone.CurrentTemp,
				Threshold: zone.CriticalTemp,
			})
		} else if zone.CurrentTemp >= zone.WarningTemp {
			c.addAlert(Alert{
				Type:      AlertTypeOverheat,
				Severity:  AlertSeverityWarning,
				Source:    zoneID,
				Message:   fmt.Sprintf("%s 温度警告: %.1f°C (警告: %.1f°C)", zone.Name, zone.CurrentTemp, zone.WarningTemp),
				Value:     zone.CurrentTemp,
				Threshold: zone.WarningTemp,
			})
		}
	}

	// 3. 记录历史
	record := HistoryRecord{
		Timestamp: now,
		Temps:     make(map[string]float64),
		RPMs:      make(map[string]int),
	}
	for zoneID, zone := range c.zones {
		record.Temps[zoneID] = zone.CurrentTemp
	}
	for fanID, fan := range c.fans {
		record.RPMs[fanID] = fan.CurrentRPM
	}
	c.history = append(c.history, record)
	if len(c.history) > c.maxHistory {
		c.history = c.history[len(c.history)-c.maxHistory:]
	}
}

// readTemperature 读取温度传感器 (毫摄氏度 -> 摄氏度)
func (c *Controller) readTemperature(path string) (float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// 模拟模式: 返回模拟温度
		return 40.0 + float64(time.Now().Second()%20), nil
	}

	tempStr := strings.TrimSpace(string(data))
	tempMilli, err := strconv.ParseFloat(tempStr, 64)
	if err != nil {
		return 0, fmt.Errorf("parse temperature: %w", err)
	}

	return tempMilli / 1000.0, nil
}

// readFanRPM 读取风扇转速
func (c *Controller) readFanRPM(path string) (int, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		// 模拟模式: 返回模拟转速
		return 1000 + int(time.Now().Second()%500), nil
	}

	rpmStr := strings.TrimSpace(string(data))
	rpm, err := strconv.Atoi(rpmStr)
	if err != nil {
		return 0, fmt.Errorf("parse rpm: %w", err)
	}

	return rpm, nil
}

// setFanPWM 设置风扇 PWM 值
func (c *Controller) setFanPWM(path string, pwm int) error {
	// 模拟模式: 仅记录日志
	c.logger.Debug("[智能风扇] 设置 PWM", zap.String("path", path), zap.Int("pwm", pwm))
	return nil
}

// interpolateCurve 根据温度-转速曲线插值计算目标转速
// 使用线性插值在曲线点之间计算
// 温度低于最低点: 返回最低点转速
// 温度高于最高点: 返回 100%
// 温度在两点之间: 线性插值
func (c *Controller) interpolateCurve(temp float64, curve []CurvePoint) float64 {
	if len(curve) == 0 {
		return 0
	}

	// 温度低于最低阈值
	if temp <= curve[0].Temp {
		return float64(curve[0].Percent)
	}

	// 温度高于最高阈值
	if temp >= curve[len(curve)-1].Temp {
		return 100.0
	}

	// 在曲线点之间线性插值
	for i := 0; i < len(curve)-1; i++ {
		if temp >= curve[i].Temp && temp < curve[i+1].Temp {
			ratio := (temp - curve[i].Temp) / (curve[i+1].Temp - curve[i].Temp)
			return float64(curve[i].Percent) + ratio*float64(curve[i+1].Percent-curve[i].Percent)
		}
	}

	return float64(curve[len(curve)-1].Percent)
}

// isNightMode 判断是否在夜间模式时间段
func (c *Controller) isNightMode() bool {
	policy := c.policies[c.activePolicyID]
	if policy == nil || !policy.NightModeEnabled {
		return false
	}

	now := time.Now()
	currentTime := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())

	// 处理跨日情况 (如 23:00 - 07:00)
	if policy.NightStart > policy.NightEnd {
		return currentTime >= policy.NightStart || currentTime < policy.NightEnd
	}
	return currentTime >= policy.NightStart && currentTime < policy.NightEnd
}

// addAlert 添加告警 (去重: 1 分钟内相同类型不重复)
func (c *Controller) addAlert(alert Alert) {
	// 检查是否重复
	for i := len(c.alerts) - 1; i >= 0 && i >= len(c.alerts)-10; i-- {
		recent := c.alerts[i]
		if recent.Type == alert.Type && recent.Source == alert.Source &&
			time.Since(recent.Timestamp) < 1*time.Minute {
			return // 跳过重复告警
		}
	}

	fanAlert := FanAlert{
		ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		Type:      alert.Type,
		Severity:  alert.Severity,
		Source:    alert.Source,
		Message:   alert.Message,
		Value:     alert.Value,
		Threshold: alert.Threshold,
		Timestamp: time.Now(),
	}

	c.alerts = append(c.alerts, fanAlert)

	if len(c.alerts) > c.maxAlerts {
		c.alerts = c.alerts[len(c.alerts)-c.maxAlerts:]
	}

	log.Printf("[智能风扇] 告警: %s - %s", alert.Type, alert.Message)
}

// Alert 内部告警结构
type Alert struct {
	Type      AlertType
	Severity  AlertSeverity
	Source    string
	Message   string
	Value     float64
	Threshold float64
}

// ========== 公开方法 ==========

// GetFans 获取所有风扇
func (c *Controller) GetFans() []FanDevice {
	c.mu.RLock()
	defer c.mu.RUnlock()

	fans := make([]FanDevice, 0, len(c.fans))
	for _, fan := range c.fans {
		fans = append(fans, *fan)
	}
	return fans
}

// GetZones 获取所有温度区域
func (c *Controller) GetZones() []ThermalZone {
	c.mu.RLock()
	defer c.mu.RUnlock()

	zones := make([]ThermalZone, 0, len(c.zones))
	for _, zone := range c.zones {
		zones = append(zones, *zone)
	}
	return zones
}

// GetProfiles 获取所有配置
func (c *Controller) GetProfiles() []FanProfile {
	c.mu.RLock()
	defer c.mu.RUnlock()

	profiles := make([]FanProfile, 0, len(c.profiles))
	for _, profile := range c.profiles {
		profiles = append(profiles, *profile)
	}
	return profiles
}

// CreateProfile 创建自定义配置
func (c *Controller) CreateProfile(name string, mode FanMode, curve []CurvePoint) (*FanProfile, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	// 校验曲线
	if len(curve) < 2 {
		return nil, fmt.Errorf("曲线至少需要 2 个点")
	}

	// 按温度排序
	for i := 0; i < len(curve)-1; i++ {
		if curve[i].Temp >= curve[i+1].Temp {
			return nil, fmt.Errorf("曲线点温度必须递增")
		}
	}

	id := fmt.Sprintf("custom_%d", time.Now().UnixNano())
	profile := &FanProfile{
		ID:        id,
		Name:      name,
		Mode:      mode,
		Curve:     curve,
		IsDefault: false,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	c.profiles[id] = profile
	c.logger.Info("[智能风扇] 创建配置", zap.String("id", id), zap.String("name", name))

	return profile, nil
}

// SetActiveProfile 切换活跃配置
func (c *Controller) SetActiveProfile(profileID string) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	profile, ok := c.profiles[profileID]
	if !ok {
		return fmt.Errorf("配置不存在: %s", profileID)
	}

	c.activeProfileID = profileID
	c.logger.Info("[智能风扇] 切换配置", zap.String("name", profile.Name))

	return nil
}

// GetStats 获取统计数据
func (c *Controller) GetStats() FanStats {
	c.mu.RLock()
	defer c.mu.RUnlock()

	stats := FanStats{
		Timestamp:     time.Now(),
		ActiveProfile: c.activeProfileID,
		ActivePolicy:  c.activePolicyID,
	}

	// 复制区域数据
	for _, zone := range c.zones {
		stats.Zones = append(stats.Zones, *zone)
		stats.AvgTemp += zone.CurrentTemp
		if zone.CurrentTemp > stats.MaxTemp {
			stats.MaxTemp = zone.CurrentTemp
		}
	}
	if len(c.zones) > 0 {
		stats.AvgTemp /= float64(len(c.zones))
	}

	// 复制风扇数据
	for _, fan := range c.fans {
		stats.Fans = append(stats.Fans, *fan)
		if fan.MaxRPM > 0 {
			stats.AvgFanSpeed += float64(fan.CurrentRPM) / float64(fan.MaxRPM) * 100
		}
	}
	if len(c.fans) > 0 {
		stats.AvgFanSpeed /= float64(len(c.fans))
	}

	return stats
}

// GetAlerts 获取告警列表
func (c *Controller) GetAlerts(limit int) []FanAlert {
	c.mu.RLock()
	defer c.mu.RUnlock()

	if limit <= 0 || limit > len(c.alerts) {
		limit = len(c.alerts)
	}

	// 返回最新的告警
	start := len(c.alerts) - limit
	if start < 0 {
		start = 0
	}
	return c.alerts[start:]
}

// GetHistory 获取历史记录
func (c *Controller) GetHistory(duration time.Duration) []HistoryRecord {
	c.mu.RLock()
	defer c.mu.RUnlock()

	cutoff := time.Now().Add(-duration)
	var result []HistoryRecord
	for _, r := range c.history {
		if r.Timestamp.After(cutoff) {
			result = append(result, r)
		}
	}
	return result
}
