// types.go - SmartThermal2 类型定义
// 智能温控系统 v2：AI自适应散热、噪音优化、温度预测
package smartthermal2

import (
	"time"
)

// ==================== 传感器相关 ====================

// SensorType 传感器类型.
type SensorType string

const (
	SensorCPU     SensorType = "cpu"
	SensorGPU     SensorType = "gpu"
	SensorHDD     SensorType = "hdd"
	SensorSSD     SensorType = "ssd"
	SensorNVMe    SensorType = "nvme"
	SensorMother  SensorType = "motherboard"
	SensorAmbient SensorType = "ambient"
)

// Sensor 传感器.
type Sensor struct {
	ID        string       `json:"id"`
	Name      string       `json:"name"`
	Type      SensorType   `json:"type"`
	Temp      float64      `json:"temp"`    // 当前温度（摄氏度）
	MaxTemp   float64      `json:"maxTemp"` // 最高记录温度
	MinTemp   float64      `json:"minTemp"` // 最低记录温度
	AvgTemp   float64      `json:"avgTemp"` // 平均温度
	Zone      string       `json:"zone"`    // 所属温控区域
	Status    SensorStatus `json:"status"`
	UpdatedAt time.Time    `json:"updatedAt"`
}

// SensorStatus 传感器状态.
type SensorStatus string

const (
	SensorNormal    SensorStatus = "normal"
	SensorWarning   SensorStatus = "warning"
	SensorCritical  SensorStatus = "critical"
	SensorEmergency SensorStatus = "emergency"
)

// SensorHistory 传感器历史记录.
type SensorHistory struct {
	Timestamp time.Time `json:"timestamp"`
	Temp      float64   `json:"temp"`
}

// ==================== 温控区域相关 ====================

// ThermalZone 温控区域.
type ThermalZone struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	SensorIDs   []string     `json:"sensorIds"` // 关联的传感器
	MaxTemp     float64      `json:"maxTemp"`   // 区域最高温度
	AvgTemp     float64      `json:"avgTemp"`   // 区域平均温度
	Status      SensorStatus `json:"status"`
	UpdatedAt   time.Time    `json:"updatedAt"`
}

// ==================== 风扇相关 ====================

// FanInfo 风扇信息.
type FanInfo struct {
	ID         string         `json:"id"`
	Name       string         `json:"name"`
	Zone       string         `json:"zone"`      // 所属区域
	PWM        float64        `json:"pwm"`       // 当前 PWM (0-100%)
	TargetPWM  float64        `json:"targetPWM"` // 目标 PWM
	RPM        int            `json:"rpm"`       // 当前转速
	MaxRPM     int            `json:"maxRpm"`    // 最大转速
	MinRPM     int            `json:"minRpm"`    // 最小转速
	Profile    FanProfileType `json:"profile"`   // 当前风扇曲线
	Status     FanStatus      `json:"status"`
	NoiseLevel float64        `json:"noiseLevel"` // 估算噪音 (dBA)
	UpdatedAt  time.Time      `json:"updatedAt"`
}

// FanStatus 风扇状态.
type FanStatus string

const (
	FanStatusNormal   FanStatus = "normal"
	FanStatusWarning  FanStatus = "warning"
	FanStatusFailed   FanStatus = "failed"
	FanStatusDisabled FanStatus = "disabled"
)

// FanProfileType 风扇曲线类型.
type FanProfileType string

const (
	FanProfileSilent      FanProfileType = "silent"      // 静音
	FanProfileStandard    FanProfileType = "standard"    // 标准
	FanProfilePerformance FanProfileType = "performance" // 性能
	FanProfileFullSpeed   FanProfileType = "fullspeed"   // 全速
	FanProfileAdaptive    FanProfileType = "adaptive"    // AI自适应
)

// FanCurvePoint 风扇曲线控制点.
type FanCurvePoint struct {
	Temp float64 `json:"temp"` // 温度（摄氏度）
	PWM  float64 `json:"pwm"`  // PWM 百分比 (0-100)
}

// FanCurve 风扇曲线.
type FanCurve struct {
	Type   FanProfileType  `json:"type"`
	Points []FanCurvePoint `json:"points"`
}

// FanUpdateRequest 风扇更新请求.
type FanUpdateRequest struct {
	PWM     *float64        `json:"pwm,omitempty"`
	Profile *FanProfileType `json:"profile,omitempty"`
}

// ==================== 噪音优化相关 ====================

// NoiseLevel 噪音级别.
type NoiseLevel string

const (
	NoiseSilent   NoiseLevel = "silent"    // < 25 dBA
	NoiseQuiet    NoiseLevel = "quiet"     // 25-35 dBA
	NoiseModerate NoiseLevel = "moderate"  // 35-45 dBA
	NoiseLoud     NoiseLevel = "loud"      // 45-55 dBA
	NoiseVeryLoud NoiseLevel = "very_loud" // > 55 dBA
)

// NoiseAssessment 噪音评估.
type NoiseAssessment struct {
	TotalDBA       float64    `json:"totalDba"`       // 总噪音 (dBA)
	Level          NoiseLevel `json:"level"`          // 噪音级别
	NoiseBudget    float64    `json:"noiseBudget"`    // 噪音预算 (dBA)
	BudgetUsed     float64    `json:"budgetUsed"`     // 已使用预算百分比
	PerFanNoise    []FanNoise `json:"perFanNoise"`    // 每个风扇噪音
	Recommendation string     `json:"recommendation"` // 建议
}

// FanNoise 单个风扇噪音.
type FanNoise struct {
	FanID string  `json:"fanId"`
	Name  string  `json:"name"`
	DBA   float64 `json:"dba"`
	RPM   int     `json:"rpm"`
}

// NoiseSettings 噪音设置.
type NoiseSettings struct {
	MaxDBA          float64 `json:"maxDba"`          // 最大允许噪音
	ScheduleEnabled bool    `json:"scheduleEnabled"` // 启用时间调度
	DayStartHour    int     `json:"dayStartHour"`    // 白天开始时间
	DayEndHour      int     `json:"dayEndHour"`      // 白天结束时间
	DayMaxDBA       float64 `json:"dayMaxDba"`       // 白天噪音限制
	NightMaxDBA     float64 `json:"nightMaxDba"`     // 夜间噪音限制
}

// ==================== 温度预测相关 ====================

// PredictionResult 温度预测结果.
type PredictionResult struct {
	SensorID          string  `json:"sensorId"`
	CurrentTemp       float64 `json:"currentTemp"`
	PredictedTemp     float64 `json:"predictedTemp"`     // 预测温度
	PredictionMins    int     `json:"predictionMins"`    // 预测未来分钟数
	Trend             string  `json:"trend"`             // rising/falling/stable
	RatePerMin        float64 `json:"ratePerMin"`        // 每分钟变化率
	Confidence        float64 `json:"confidence"`        // 置信度 (0-1)
	SafeMargin        float64 `json:"safeMargin"`        // 安全系数
	WillOverheat      bool    `json:"willOverheat"`      // 是否会过热
	MinutesToOverheat int     `json:"minutesToOverheat"` // 距过热分钟数
}

// SeasonalCompensation 季节性温度补偿.
type SeasonalCompensation struct {
	Month        int     `json:"month"`        // 月份
	Compensation float64 `json:"compensation"` // 补偿值（摄氏度）
}

// ==================== 散热方案相关 ====================

// CoolingProfile 散热方案.
type CoolingProfile struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description"`
	Scenario    string          `json:"scenario"` // 卧室NAS/书房NAS/机房NAS/客厅NAS
	FanCurve    FanCurve        `json:"fanCurve"`
	NoiseLimit  float64         `json:"noiseLimit"` // 噪音限制 (dBA)
	MaxTemp     float64         `json:"maxTemp"`    // 最高温度限制
	IsDefault   bool            `json:"isDefault"`
	IsActive    bool            `json:"isActive"`
	IsCustom    bool            `json:"isCustom"`
	Schedule    *ScheduleConfig `json:"schedule,omitempty"` // 定时切换
	CreatedAt   time.Time       `json:"createdAt"`
}

// ScheduleConfig 定时切换配置.
type ScheduleConfig struct {
	Enabled      bool   `json:"enabled"`
	DayProfile   string `json:"dayProfile"`   // 白天方案ID
	NightProfile string `json:"nightProfile"` // 夜间方案ID
	DayStart     int    `json:"dayStart"`     // 白天开始小时
	DayEnd       int    `json:"dayEnd"`       // 白天结束小时
}

// ProfileSwitchRequest 方案切换请求.
type ProfileSwitchRequest struct {
	ProfileID string `json:"profileId" binding:"required"`
}

// ProfileCreateRequest 方案创建请求.
type ProfileCreateRequest struct {
	Name        string          `json:"name" binding:"required"`
	Description string          `json:"description"`
	Scenario    string          `json:"scenario"`
	FanCurve    FanCurve        `json:"fanCurve"`
	NoiseLimit  float64         `json:"noiseLimit"`
	MaxTemp     float64         `json:"maxTemp"`
	Schedule    *ScheduleConfig `json:"schedule,omitempty"`
}

// ==================== 告警相关 ====================

// AlertLevel 告警级别.
type AlertLevel string

const (
	AlertWarning   AlertLevel = "warning"   // 警告
	AlertCritical  AlertLevel = "critical"  // 危险
	AlertEmergency AlertLevel = "emergency" // 紧急
)

// ThermalAlert 温控告警.
type ThermalAlert struct {
	ID         string     `json:"id"`
	Level      AlertLevel `json:"level"`
	Source     string     `json:"source"` // 来源（传感器/风扇/区域）
	Message    string     `json:"message"`
	Temp       float64    `json:"temp"`
	Threshold  float64    `json:"threshold"`
	Actions    []string   `json:"actions"` // 已执行的保护动作
	Active     bool       `json:"active"`
	CreatedAt  time.Time  `json:"createdAt"`
	ResolvedAt *time.Time `json:"resolvedAt,omitempty"`
}

// AlertSettings 告警设置.
type AlertSettings struct {
	WarningTemp   float64 `json:"warningTemp"`   // 警告温度
	CriticalTemp  float64 `json:"criticalTemp"`  // 危险温度
	EmergencyTemp float64 `json:"emergencyTemp"` // 紧急温度
	WebhookURL    string  `json:"webhookUrl"`    // Webhook 地址
	EmailTo       string  `json:"emailTo"`       // 告警邮箱
	SMSTo         string  `json:"smsTo"`         // 短信号码
	AutoProtect   bool    `json:"autoProtect"`   // 自动保护
}

// ==================== 全局设置 ====================

// GlobalSettings 全局设置.
type GlobalSettings struct {
	PollIntervalSec int           `json:"pollIntervalSec"` // 采样间隔（秒）
	WindowSize      int           `json:"windowSize"`      // 滑动窗口大小
	AdaptiveEnabled bool          `json:"adaptiveEnabled"` // AI自适应开关
	NoiseSettings   NoiseSettings `json:"noiseSettings"`
	AlertSettings   AlertSettings `json:"alertSettings"`
}

// ==================== Dashboard ====================

// Dashboard 温控仪表板.
type Dashboard struct {
	OverallStatus  SensorStatus       `json:"overallStatus"`
	CurrentProfile string             `json:"currentProfile"`
	Sensors        []Sensor           `json:"sensors"`
	Zones          []ThermalZone      `json:"zones"`
	Fans           []FanInfo          `json:"fans"`
	Noise          NoiseAssessment    `json:"noise"`
	ActiveAlerts   []ThermalAlert     `json:"activeAlerts"`
	Predictions    []PredictionResult `json:"predictions"`
	UpdatedAt      time.Time          `json:"updatedAt"`
}

// ==================== EWMA 相关 ====================

// EWMAData 指数加权移动平均数据.
type EWMAData struct {
	Value float64
	Alpha float64 // 平滑因子
}

// ==================== 请求/响应通用 ====================

// APIResponse 通用 API 响应.
type APIResponse struct {
	Code    int         `json:"code"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
}

// HistoryQuery 历史查询参数.
type HistoryQuery struct {
	Minutes int `form:"minutes"`
}

// LimitQuery 分页查询.
type LimitQuery struct {
	Limit int `form:"limit"`
}

// PredictQuery 预测查询参数.
type PredictQuery struct {
	Minutes int `form:"minutes"` // 预测未来几分钟
}
