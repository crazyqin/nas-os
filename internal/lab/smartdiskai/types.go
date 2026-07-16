// Package smartdiskai 提供智能磁盘故障预测增强模块
// 基于 S.M.A.R.T. 数据采集、线性回归故障预测、健康评分、
// 温度趋势分析、磁盘生命周期管理、数据迁移建议
package smartdiskai

import (
	"time"
)

// ============================================================
// 基础枚举类型
// ============================================================

// HealthGrade 健康等级.
type HealthGrade string

const (
	GradeExcellent HealthGrade = "excellent" // 优秀 (90-100)
	GradeGood      HealthGrade = "good"      // 良好 (70-89)
	GradeFair      HealthGrade = "fair"      // 一般 (50-69)
	GradePoor      HealthGrade = "poor"      // 较差 (30-49)
	GradeCritical  HealthGrade = "critical"  // 临界 (0-29)
)

// DiskStatus 磁盘状态.
type DiskStatus string

const (
	StatusHealthy  DiskStatus = "healthy"  // 健康
	StatusWarning  DiskStatus = "warning"  // 警告
	StatusCritical DiskStatus = "critical" // 临界
	StatusFailed   DiskStatus = "failed"   // 已失败
)

// RiskLevel 风险等级.
type RiskLevel string

const (
	RiskLow      RiskLevel = "low"      // 低风险
	RiskMedium   RiskLevel = "medium"   // 中风险
	RiskHigh     RiskLevel = "high"     // 高风险
	RiskCritical RiskLevel = "critical" // 临界风险
)

// TrendDirection 趋势方向.
type TrendDirection string

const (
	TrendImproving TrendDirection = "improving" // 改善中
	TrendStable    TrendDirection = "stable"    // 稳定
	TrendDeclining TrendDirection = "declining" // 恶化中
	TrendCritical  TrendDirection = "critical"  // 临界
)

// AdvicePriority 建议优先级.
type AdvicePriority string

const (
	PriorityUrgent AdvicePriority = "urgent" // 紧急
	PriorityHigh   AdvicePriority = "high"   // 高
	PriorityMedium AdvicePriority = "medium" // 中
	PriorityLow    AdvicePriority = "low"    // 低
	PriorityInfo   AdvicePriority = "info"   // 信息
)

// MigrationUrgency 迁移紧急程度.
type MigrationUrgency string

const (
	MigrationImmediate MigrationUrgency = "immediate" // 立即迁移
	MigrationSoon      MigrationUrgency = "soon"      // 尽快迁移
	MigrationPlanned   MigrationUrgency = "planned"   // 计划迁移
	MigrationOptional  MigrationUrgency = "optional"  // 可选迁移
)

// ============================================================
// SMART 属性 ID 定义
// ============================================================

// SMARTAttributeID SMART 属性 ID.
type SMARTAttributeID int

const (
	SMARTIDReallocatedSectorCt   SMARTAttributeID = 5   // 重映射扇区计数
	SMARTIDSpinRetryCount        SMARTAttributeID = 10  // 主轴重试次数
	SMARTIDCalibrationRetryCount SMARTAttributeID = 11  // 校准重试次数
	SMARTIDPowerCycleCount       SMARTAttributeID = 12  // 通电周期计数
	SMARTIDSoftReadErrorRate     SMARTAttributeID = 13  // 软读错误率
	SMARTIDCurrentPendingSector  SMARTAttributeID = 197 // 当前待映射扇区数
	SMARTIDOfflineUncorrectable  SMARTAttributeID = 198 // 离线不可修复扇区数
	SMARTIDTemperatureCelsius    SMARTAttributeID = 194 // 温度
	SMARTIDPowerOnHours          SMARTAttributeID = 9   // 通电时间
	SMARTIDWearLevelingCount     SMARTAttributeID = 173 // 磨损均衡计数（SSD）
	SMARTIDTotalLBAsWritten      SMARTAttributeID = 241 // 总写入 LBAs
	SMARTIDTotalLBAsRead         SMARTAttributeID = 242 // 总读取 LBAs
	SMARTIDSeekErrorRate         SMARTAttributeID = 7   // 寻道错误率
	SMARTIDSpinUpTime            SMARTAttributeID = 3   // 主轴起旋时间
	SMARTIDStartStopCount        SMARTAttributeID = 4   // 启停计数
	SMARTIDReallocatedEventCount SMARTAttributeID = 196 // 重映射事件计数
	SMARTIDUDMAErrorCount        SMARTAttributeID = 199 // UDMA CRC 错误计数
	SMARTIDMultiZoneErrorRate    SMARTAttributeID = 187 // 多区域错误率
	SMARTIDGSENSEErrorRate       SMARTAttributeID = 191 // 冲击传感器错误率
	SMARTIDLoadUnloadCycleCount  SMARTAttributeID = 193 // 磁头加载/卸载次数
	SMARTIDHeadFlyingHours       SMARTAttributeID = 240 // 磁头飞行时间
	SMARTIDTotalHostWrites       SMARTAttributeID = 48  // 主机总写入量
	SMARTIDTotalHostReads        SMARTAttributeID = 49  // 主机总读取量
	SMARTIDNANDWrites            SMARTAttributeID = 233 // NAND 写入量（SSD）
	SMARTIDSSDLifeLeft           SMARTAttributeID = 231 // SSD 剩余寿命
	SMARTIDUnsafeShutdownCount   SMARTAttributeID = 174 // 不安全关机次数
	SMARTIDTemperature2          SMARTAttributeID = 190 // 温度2（Airflow）
	SMARTIDHardwareECCRecovered  SMARTAttributeID = 195 // 硬件 ECC 恢复次数
	SMARTIDReportedUncorrect     SMARTAttributeID = 188 // 报告的不可修复错误
)

// ============================================================
// SMART 数据结构
// ============================================================

// SMARTAttribute 单个 SMART 属性.
type SMARTAttribute struct {
	ID          SMARTAttributeID `json:"id"`          // 属性 ID
	Name        string           `json:"name"`        // 属性名称
	Value       int              `json:"value"`       // 标准化值（0-253）
	Worst       int              `json:"worst"`       // 历史最差值
	Threshold   int              `json:"threshold"`   // 阈值
	RawValue    uint64           `json:"raw_value"`   // 原始值
	IsCritical  bool             `json:"is_critical"` // 是否关键指标
	Failed      bool             `json:"failed"`      // 是否已失败
	Description string           `json:"description"` // 属性说明
}

// SMARTData 完整 SMART 数据.
type SMARTData struct {
	Device             string           `json:"device"`                // 设备路径
	Model              string           `json:"model"`                 // 型号
	Serial             string           `json:"serial"`                // 序列号
	Firmware           string           `json:"firmware"`              // 固件版本
	Interface          string           `json:"interface"`             // 接口类型
	CapacityBytes      uint64           `json:"capacity_bytes"`        // 容量
	Temperature        int              `json:"temperature"`           // 当前温度（℃）
	MaxTemperature     int              `json:"max_temperature"`       // 历史最高温度
	PowerOnHours       uint64           `json:"power_on_hours"`        // 通电时间（小时）
	PowerCycleCount    uint64           `json:"power_cycle_count"`     // 通电周期
	ReallocatedSects   uint64           `json:"reallocated_sectors"`   // 重映射扇区数
	PendingSects       uint64           `json:"pending_sectors"`       // 待映射扇区数
	UncorrectableSects uint64           `json:"uncorrectable_sectors"` // 不可修复扇区数
	TotalLBAsWritten   uint64           `json:"total_lbas_written"`    // 总写入 LBAs
	TotalLBAsRead      uint64           `json:"total_lbas_read"`       // 总读取 LBAs
	Attributes         []SMARTAttribute `json:"attributes"`            // 属性列表
	CollectedAt        time.Time        `json:"collected_at"`          // 采集时间
	IsSSD              bool             `json:"is_ssd"`                // 是否为 SSD
}

// ============================================================
// 线性回归与预测
// ============================================================

// LinearRegressionResult 线性回归结果.
type LinearRegressionResult struct {
	Slope         float64 `json:"slope"`          // 斜率
	Intercept     float64 `json:"intercept"`      // 截距
	RSquared      float64 `json:"r_squared"`      // R² 决定系数
	Projected90D  float64 `json:"projected_90d"`  // 90天后预测值
	Projected180D float64 `json:"projected_180d"` // 180天后预测值
	Projected365D float64 `json:"projected_365d"` // 365天后预测值
}

// FailurePrediction 故障预测结果.
type FailurePrediction struct {
	Device              string               `json:"device"`
	FailureProbability  float64              `json:"failure_probability"`          // 故障概率 (0-1)
	RiskLevel           RiskLevel            `json:"risk_level"`                   // 风险等级
	EstimatedLifeDays   int                  `json:"estimated_life_days"`          // 预计剩余寿命（天）
	FailDateEstimate    *time.Time           `json:"fail_date_estimate,omitempty"` // 预计故障日期
	Confidence          float64              `json:"confidence"`                   // 置信度 (0-1)
	RiskFactors         []string             `json:"risk_factors"`                 // 风险因素
	ThresholdViolations []ThresholdViolation `json:"threshold_violations"`         // 阈值违规
	PredictedAt         time.Time            `json:"predicted_at"`
}

// ThresholdViolation 阈值违规.
type ThresholdViolation struct {
	AttributeID   SMARTAttributeID `json:"attribute_id"`
	AttributeName string           `json:"attribute_name"`
	CurrentValue  uint64           `json:"current_value"`
	Threshold     uint64           `json:"threshold"`
	Severity      string           `json:"severity"` // warning/critical
	Message       string           `json:"message"`
}

// ============================================================
// SMART 分析结果
// ============================================================

// AttributeTrend 单属性趋势分析.
type AttributeTrend struct {
	AttributeID   SMARTAttributeID        `json:"attribute_id"`
	AttributeName string                  `json:"attribute_name"`
	Current       uint64                  `json:"current"`
	Trend         TrendDirection          `json:"trend"`
	Regression    *LinearRegressionResult `json:"regression,omitempty"`
}

// SMARTAnalysisResult SMART 分析综合结果.
type SMARTAnalysisResult struct {
	Device       string           `json:"device"`
	Attributes   []AttributeTrend `json:"attributes"`
	OverallTrend TrendDirection   `json:"overall_trend"`
	AnalyzedAt   time.Time        `json:"analyzed_at"`
}

// ============================================================
// 健康评分系统
// ============================================================

// AttributeScore 属性评分.
type AttributeScore struct {
	AttributeID   SMARTAttributeID `json:"attribute_id"`
	AttributeName string           `json:"attribute_name"`
	Score         float64          `json:"score"`          // 该属性得分 (0-100)
	Weight        float64          `json:"weight"`         // 权重
	WeightedScore float64          `json:"weighted_score"` // 加权得分
	Status        string           `json:"status"`         // normal/warning/critical
}

// HealthScore 健康评分结果.
type HealthScore struct {
	Device          string           `json:"device"`
	Score           float64          `json:"score"`                    // 综合评分 (0-100)
	Grade           HealthGrade      `json:"grade"`                    // 等级
	Status          DiskStatus       `json:"status"`                   // 状态
	AttributeScores []AttributeScore `json:"attribute_scores"`         // 各属性评分
	PreviousScore   *float64         `json:"previous_score,omitempty"` // 上次评分
	ScoreDelta      float64          `json:"score_delta"`              // 评分变化
	Trend           TrendDirection   `json:"trend"`                    // 评分趋势
	CalculatedAt    time.Time        `json:"calculated_at"`
}

// ============================================================
// 温度趋势分析
// ============================================================

// TemperatureTrend 温度趋势分析结果.
type TemperatureTrend struct {
	Device        string                  `json:"device"`
	CurrentTemp   int                     `json:"current_temp"`         // 当前温度
	AvgTemp       float64                 `json:"avg_temp"`             // 平均温度
	MaxTemp       int                     `json:"max_temp"`             // 最高温度
	MinTemp       int                     `json:"min_temp"`             // 最低温度
	TempStdDev    float64                 `json:"temp_std_dev"`         // 温度标准差
	Trend         TrendDirection          `json:"trend"`                // 温度趋势
	Regression    *LinearRegressionResult `json:"regression,omitempty"` // 回归分析
	Alerts        []TemperatureAlert      `json:"alerts"`               // 温度告警
	PredictedPeak int                     `json:"predicted_peak"`       // 预测峰值温度
	AnalyzedAt    time.Time               `json:"analyzed_at"`
}

// TemperatureAlert 温度告警.
type TemperatureAlert struct {
	Level     string    `json:"level"`     // warning/critical
	Message   string    `json:"message"`   // 告警消息
	Temp      int       `json:"temp"`      // 触发温度
	Threshold int       `json:"threshold"` // 阈值
	CreatedAt time.Time `json:"created_at"`
}

// ============================================================
// 磁盘生命周期管理
// ============================================================

// DiskLifecycle 磁盘生命周期信息.
type DiskLifecycle struct {
	Device           string         `json:"device"`
	Model            string         `json:"model"`
	Serial           string         `json:"serial"`
	ManufactureDate  *time.Time     `json:"manufacture_date,omitempty"` // 制造日期
	WarrantyStart    *time.Time     `json:"warranty_start,omitempty"`   // 保修开始
	WarrantyEnd      *time.Time     `json:"warranty_end,omitempty"`     // 保修结束
	WarrantyYears    int            `json:"warranty_years"`             // 保修年限
	WarrantyStatus   string         `json:"warranty_status"`            // warranty_status: active/expiring_soon/expired/unknown
	WarrantyDaysLeft int            `json:"warranty_days_left"`         // 保修剩余天数
	PowerOnHours     uint64         `json:"power_on_hours"`             // 通电时间
	AgeDays          int            `json:"age_days"`                   // 使用天数
	IsSSD            bool           `json:"is_ssd"`
	WearLevel        *WearLevelInfo `json:"wear_level,omitempty"`   // 磨损均衡信息
	TotalWrites      uint64         `json:"total_writes_bytes"`     // 总写入量
	TotalReads       uint64         `json:"total_reads_bytes"`      // 总读取量
	HealthScore      float64        `json:"health_score"`           // 健康评分
	RemainingLife    float64        `json:"remaining_life_percent"` // 剩余寿命百分比
	UpdatedAt        time.Time      `json:"updated_at"`
}

// WearLevelInfo 磨损均衡信息.
type WearLevelInfo struct {
	WearLevelingCount uint64  `json:"wear_leveling_count"` // 磨损均衡计数
	PercentUsed       float64 `json:"percent_used"`        // 已使用百分比
	PercentRemaining  float64 `json:"percent_remaining"`   // 剩余百分比
	EstimatedTBW      float64 `json:"estimated_tbw"`       // 预估总写入量 (TB)
	CurrentTBW        float64 `json:"current_tbw"`         // 当前写入量 (TB)
	TBWRatio          float64 `json:"tbw_ratio"`           // TBW 使用比例
}

// ============================================================
// 数据迁移建议
// ============================================================

// MigrationRecommendation 数据迁移建议.
type MigrationRecommendation struct {
	ID            string           `json:"id"`
	SourceDevice  string           `json:"source_device"`
	TargetDevice  string           `json:"target_device,omitempty"`
	RiskLevel     RiskLevel        `json:"risk_level"`
	Urgency       MigrationUrgency `json:"urgency"`
	Reason        string           `json:"reason"`
	EstimatedSize uint64           `json:"estimated_size_bytes"`
	EstimatedTime string           `json:"estimated_time"`
	RecommendedFS string           `json:"recommended_fs"`
	Steps         []string         `json:"steps"`
	CreatedAt     time.Time        `json:"created_at"`
}

// ============================================================
// 维护建议
// ============================================================

// MaintenanceAdvice 维护建议.
type MaintenanceAdvice struct {
	ID            string         `json:"id"`
	Device        string         `json:"device"`
	Title         string         `json:"title"`          // 标题
	Description   string         `json:"description"`    // 详细描述
	Priority      AdvicePriority `json:"priority"`       // 优先级
	Category      string         `json:"category"`       // 类别
	EstimatedCost float64        `json:"estimated_cost"` // 预估成本（元）
	Urgency       string         `json:"urgency"`        // 紧急程度说明
	CreatedAt     time.Time      `json:"created_at"`
}

// ============================================================
// API 通用类型
// ============================================================

// DiskListItem 磁盘列表项.
type DiskListItem struct {
	Device       string      `json:"device"`
	Model        string      `json:"model"`
	Serial       string      `json:"serial"`
	Status       DiskStatus  `json:"status"`
	Score        float64     `json:"score"`
	Grade        HealthGrade `json:"grade"`
	IsSSD        bool        `json:"is_ssd"`
	Capacity     uint64      `json:"capacity_bytes"`
	Temperature  int         `json:"temperature"`
	PowerOnHours uint64      `json:"power_on_hours"`
	RiskLevel    RiskLevel   `json:"risk_level"`
}

// DashboardData 仪表板数据.
type DashboardData struct {
	TotalDisks      int       `json:"total_disks"`
	HealthyDisks    int       `json:"healthy_disks"`
	WarningDisks    int       `json:"warning_disks"`
	CriticalDisks   int       `json:"critical_disks"`
	FailedDisks     int       `json:"failed_disks"`
	AverageScore    float64   `json:"average_score"`
	WorstDisk       string    `json:"worst_disk"`
	WorstScore      float64   `json:"worst_score"`
	AdvicesCount    int       `json:"advice_count"`
	MigrationsCount int       `json:"migrations_count"`
	TempAlertsCount int       `json:"temp_alerts_count"`
	GeneratedAt     time.Time `json:"generated_at"`
}

// APIResponse 通用 API 响应.
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}
