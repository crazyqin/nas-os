// Package smartfan 提供智能风扇控制功能
// 温度监控、PID 控制、多区域联动、静音模式、故障检测
package smartfan

import (
	"time"
)

// ========== 风扇设备 ==========

// FanDevice 风扇设备
type FanDevice struct {
	ID          string    `json:"id"`          // 唯一标识 (如 fan0, fan1)
	Name        string    `json:"name"`        // 显示名称 (如 CPU风扇, 机箱风扇)
	PWMPath     string    `json:"pwmPath"`     // PWM 控制路径 (/sys/class/hwmon/hwmonX/pwmY)
	RPMPath     string    `json:"rpmPath"`     // 转速读取路径 (/sys/class/hwmon/hwmonX/fanY_input)
	CurrentRPM  int       `json:"currentRpm"`  // 当前转速
	MaxRPM      int       `json:"maxRpm"`      // 最大转速
	MinRPM      int       `json:"minRpm"`      // 最小转速 (低于此值可能停转)
	CurrentPWM  int       `json:"currentPwm"`  // 当前 PWM 值 (0-255)
	TempSource  string    `json:"tempSource"`  // 关联温度源 ID
	IsRunning   bool      `json:"isRunning"`   // 是否在运行
	LastUpdated time.Time `json:"lastUpdated"` // 最后更新时间
}

// ========== 风扇配置 ==========

// FanProfile 风扇配置方案
type FanProfile struct {
	ID        string           `json:"id"`        // 唯一标识
	Name      string           `json:"name"`      // 配置名称
	Mode      FanMode          `json:"mode"`      // 运行模式
	Curve     []CurvePoint     `json:"curve"`     // 温度-转速曲线点
	IsDefault bool             `json:"isDefault"` // 是否默认配置
	CreatedAt time.Time        `json:"createdAt"` // 创建时间
	UpdatedAt time.Time        `json:"updatedAt"` // 更新时间
}

// CurvePoint 温度-转速曲线点
type CurvePoint struct {
	Temp    float64 `json:"temp"`    // 温度阈值 (°C)
	Percent int     `json:"percent"` // 风扇转速百分比 (0-100)
}

// FanMode 风扇运行模式
type FanMode string

const (
	FanModeSilent      FanMode = "silent"      // 静音模式
	FanModeBalanced    FanMode = "balanced"    // 均衡模式
	FanModePerformance FanMode = "performance" // 性能模式
	FanModeCustom      FanMode = "custom"      // 自定义模式
)

// ========== 温度区域 ==========

// ThermalZone 温度监控区域
type ThermalZone struct {
	ID           string    `json:"id"`           // 唯一标识
	Name         string    `json:"name"`         // 区域名称
	Type         ZoneType  `json:"type"`         // 区域类型
	SensorPath   string    `json:"sensorPath"`   // 温度传感器路径 (/sys/class/thermal/thermal_zoneX/temp)
	CurrentTemp  float64   `json:"currentTemp"`  // 当前温度 (°C)
	MinTemp      float64   `json:"minTemp"`      // 历史最低温度
	MaxTemp      float64   `json:"maxTemp"`      // 历史最高温度
	AvgTemp      float64   `json:"avgTemp"`      // 平均温度
	CriticalTemp float64   `json:"criticalTemp"` // 临界温度 (超过将触发告警)
	WarningTemp  float64   `json:"warningTemp"`  // 警告温度
	FanIDs       []string  `json:"fanIds"`       // 关联的风扇 ID 列表
	LastUpdated  time.Time `json:"lastUpdated"`  // 最后更新时间
}

// ZoneType 温度区域类型
type ZoneType string

const (
	ZoneTypeCPU     ZoneType = "cpu"     // CPU 温度
	ZoneTypeGPU     ZoneType = "gpu"     // GPU 温度
	ZoneTypeHDD     ZoneType = "hdd"     // 机械硬盘温度
	ZoneTypeNVMe    ZoneType = "nvme"    // NVMe SSD 温度
	ZoneTypeAmbient ZoneType = "ambient" // 环境温度
)

// ========== 风扇策略 ==========

// FanPolicy 风扇控制策略
type FanPolicy struct {
	ID               string    `json:"id"`               // 唯一标识
	Name             string    `json:"name"`             // 策略名称
	Description      string    `json:"description"`      // 策略描述
	ProfileID        string    `json:"profileId"`        // 关联配置 ID
	MinFanSpeed      int       `json:"minFanSpeed"`      // 最小风扇转速百分比
	MaxFanSpeed      int       `json:"maxFanSpeed"`      // 最大风扇转速百分比
	NightModeEnabled bool      `json:"nightModeEnabled"` // 是否启用夜间模式
	NightStart       string    `json:"nightStart"`       // 夜间模式开始时间 (HH:MM)
	NightEnd         string    `json:"nightEnd"`         // 夜间模式结束时间 (HH:MM)
	NightSpeedLimit  int       `json:"nightSpeedLimit"`  // 夜间最大转速百分比
	IsActive         bool      `json:"isActive"`         // 是否为当前活跃策略
	CreatedAt        time.Time `json:"createdAt"`        // 创建时间
}

// ========== 告警 ==========

// FanAlert 风扇告警
type FanAlert struct {
	ID        string        `json:"id"`        // 唯一标识
	Type      AlertType     `json:"type"`      // 告警类型
	Severity  AlertSeverity `json:"severity"`  // 严重程度
	Source    string        `json:"source"`    // 告警来源 (风扇ID或温度区域ID)
	Message   string        `json:"message"`   // 告警详情
	Value     float64       `json:"value"`     // 当前值
	Threshold float64       `json:"threshold"` // 阈值
	Timestamp time.Time     `json:"timestamp"` // 告警时间
	Acked     bool          `json:"acked"`     // 是否已确认
	AckedAt   *time.Time    `json:"ackedAt"`   // 确认时间
}

// AlertType 告警类型
type AlertType string

const (
	AlertTypeOverheat      AlertType = "overheat"      // 过热
	AlertTypeFanFailure    AlertType = "fan_failure"    // 风扇故障
	AlertTypeAbnormalRPM   AlertType = "abnormal_rpm"  // 转速异常
	AlertTypeSensorFailure AlertType = "sensor_failure" // 传感器故障
)

// AlertSeverity 告警严重程度
type AlertSeverity string

const (
	AlertSeverityInfo     AlertSeverity = "info"     // 信息
	AlertSeverityWarning  AlertSeverity = "warning"  // 警告
	AlertSeverityCritical AlertSeverity = "critical" // 严重
)

// ========== PID 控制器 ==========

// PIDConfig PID 控制器配置
type PIDConfig struct {
	Kp        float64 `json:"kp"`        // 比例系数: 控制当前误差的影响
	Ki        float64 `json:"ki"`        // 积分系数: 消除稳态误差
	Kd        float64 `json:"kd"`        // 微分系数: 预测误差变化趋势
	SetPoint  float64 `json:"setPoint"`  // 目标温度 (°C)
	MinOutput float64 `json:"minOutput"` // 最小输出 (风扇最低转速%)
	MaxOutput float64 `json:"maxOutput"` // 最大输出 (风扇最高转速%)

	// 高级特性配置
	DeadZone              float64 `json:"deadZone,omitempty"`              // 死区范围 (°C)，误差在此范围内不调整
	IntegralSepThreshold  float64 `json:"integralSepThreshold,omitempty"`  // 积分分离阈值，误差超过此值暂停积分
	IntegralLimit         float64 `json:"integralLimit,omitempty"`         // 积分限幅值，防止积分饱和
	IntegralDecay         float64 `json:"integralDecay,omitempty"`         // 积分衰减系数 (0-1)，稳定时衰减积分
	DerivativeFilterAlpha float64 `json:"derivativeFilterAlpha,omitempty"` // 微分滤波系数 (0-1)，越小滤波越强
}

// DefaultPIDConfig 默认 PID 配置
// 适用于一般散热场景，可根据实际硬件调整
func DefaultPIDConfig() PIDConfig {
	return PIDConfig{
		Kp:                    2.0,
		Ki:                    0.5,
		Kd:                    1.0,
		SetPoint:              60.0, // 目标温度 60°C
		MinOutput:             0.0,
		MaxOutput:             100.0,
		DeadZone:              1.0,  // 1°C 死区
		IntegralSepThreshold:  10.0, // 误差超过 10°C 时暂停积分
		IntegralLimit:         80.0, // 积分项限幅
		IntegralDecay:         0.95, // 每 10 个周期衰减 5%
		DerivativeFilterAlpha: 0.3,  // 微分滤波系数
	}
}

// ========== 统计数据 ==========

// FanStats 风扇统计数据
type FanStats struct {
	Timestamp     time.Time     `json:"timestamp"`
	Zones         []ThermalZone `json:"zones"`
	Fans          []FanDevice   `json:"fans"`
	ActiveProfile string        `json:"activeProfile"`
	ActivePolicy  string        `json:"activePolicy"`
	AvgTemp       float64       `json:"avgTemp"`      // 全局平均温度
	MaxTemp       float64       `json:"maxTemp"`      // 全局最高温度
	AvgFanSpeed   float64       `json:"avgFanSpeed"`  // 平均风扇转速百分比
}

// HistoryRecord 历史记录
type HistoryRecord struct {
	Timestamp time.Time          `json:"timestamp"`
	Temps     map[string]float64 `json:"temps"` // zone_id -> temperature
	RPMs      map[string]int     `json:"rpms"`  // fan_id -> rpm
}
