// Package diskhealthai2 提供智能磁盘健康分析与故障预测功能
// 基于 SMART 数据的深度分析、贝叶斯故障预测、健康评分系统
package diskhealth

import (
	"time"
)

// ============================================================
// 基础类型
// ============================================================

// HealthGrade 健康等级
type HealthGrade string

const (
	GradeA HealthGrade = "A" // 优秀 (90-100)
	GradeB HealthGrade = "B" // 良好 (70-89)
	GradeC HealthGrade = "C" // 一般 (50-69)
	GradeD HealthGrade = "D" // 较差 (30-49)
	GradeF HealthGrade = "F" // 临界 (0-29)
)

// DiskStatus 磁盘状态
type DiskStatus string

const (
	StatusHealthy  DiskStatus = "healthy"  // 健康
	StatusWarning  DiskStatus = "warning"  // 警告
	StatusCritical DiskStatus = "critical" // 临界
	StatusFailed   DiskStatus = "failed"   // 已失败
)

// ============================================================
// SMART 属性定义
// ============================================================

// SMARTAttributeID SMART 属性 ID
type SMARTAttributeID int

// 主要 SMART 属性 ID 常量
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
	SMARTIDCurrentPendingECC     SMARTAttributeID = 183 // 当前待 ECC 纠错
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

// SMARTAttribute 单个 SMART 属性
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

// SMARTData 完整 SMART 数据
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
// SMART 分析结果
// ============================================================

// TrendDirection 趋势方向
type TrendDirection string

const (
	TrendImproving TrendDirection = "improving" // 改善中
	TrendStable    TrendDirection = "stable"    // 稳定
	TrendDeclining TrendDirection = "declining" // 恶化中
	TrendCritical  TrendDirection = "critical"  // 临界
)

// LinearRegressionResult 线性回归结果
type LinearRegressionResult struct {
	Slope        float64 `json:"slope"`         // 斜率
	Intercept    float64 `json:"intercept"`     // 截距
	RSquared     float64 `json:"r_squared"`     // R² 决定系数
	Projected90D float64 `json:"projected_90d"` // 90天后预测值
}

// ZScoreAnomaly Z-score 异常检测结果
type ZScoreAnomaly struct {
	AttributeID   SMARTAttributeID `json:"attribute_id"`
	AttributeName string           `json:"attribute_name"`
	CurrentValue  uint64           `json:"current_value"`
	Mean          float64          `json:"mean"`
	StdDev        float64          `json:"std_dev"`
	ZScore        float64          `json:"z_score"`    // Z 分数
	IsAnomaly     bool             `json:"is_anomaly"` // 是否异常（|Z| > 2）
	Severity      string           `json:"severity"`   // low/medium/high
}

// AttributeTrend 单属性趋势分析
type AttributeTrend struct {
	AttributeID   SMARTAttributeID        `json:"attribute_id"`
	AttributeName string                  `json:"attribute_name"`
	Current       uint64                  `json:"current"`
	Trend         TrendDirection          `json:"trend"`
	Regression    *LinearRegressionResult `json:"regression,omitempty"`
	Anomaly       *ZScoreAnomaly          `json:"anomaly,omitempty"`
}

// SMARTAnalysisResult SMART 分析综合结果
type SMARTAnalysisResult struct {
	Device       string           `json:"device"`
	Attributes   []AttributeTrend `json:"attributes"`
	Anomalies    []ZScoreAnomaly  `json:"anomalies"`
	OverallTrend TrendDirection   `json:"overall_trend"`
	AnalyzedAt   time.Time        `json:"analyzed_at"`
}

// ============================================================
// 健康评分系统
// ============================================================

// AttributeScore 属性评分
type AttributeScore struct {
	AttributeID   SMARTAttributeID `json:"attribute_id"`
	AttributeName string           `json:"attribute_name"`
	Score         float64          `json:"score"`          // 该属性得分 (0-100)
	Weight        float64          `json:"weight"`         // 权重
	WeightedScore float64          `json:"weighted_score"` // 加权得分
	Status        string           `json:"status"`         // normal/warning/critical
}

// CorrelationPenalty 属性关联惩罚
type CorrelationPenalty struct {
	Attribute1ID   SMARTAttributeID `json:"attribute1_id"`
	Attribute1Name string           `json:"attribute1_name"`
	Attribute2ID   SMARTAttributeID `json:"attribute2_id"`
	Attribute2Name string           `json:"attribute2_name"`
	Penalty        float64          `json:"penalty"` // 惩罚分
	Reason         string           `json:"reason"`  // 原因
}

// HealthScore 健康评分结果
type HealthScore struct {
	Device             string               `json:"device"`
	Score              float64              `json:"score"`                    // 综合评分 (0-100)
	Grade              HealthGrade          `json:"grade"`                    // 等级
	Status             DiskStatus           `json:"status"`                   // 状态
	AttributeScores    []AttributeScore     `json:"attribute_scores"`         // 各属性评分
	CorrelationPenalty []CorrelationPenalty `json:"correlation_penalty"`      // 关联惩罚
	PreviousScore      *float64             `json:"previous_score,omitempty"` // 上次评分
	ScoreDelta         float64              `json:"score_delta"`              // 评分变化
	Trend              TrendDirection       `json:"trend"`                    // 评分趋势
	CalculatedAt       time.Time            `json:"calculated_at"`
}

// ============================================================
// 故障预测
// ============================================================

// BayesianPrediction 贝叶斯故障预测结果
type BayesianPrediction struct {
	Device               string     `json:"device"`
	FailureProbability   float64    `json:"failure_probability"`          // 故障概率 (0-1)
	PriorProbability     float64    `json:"prior_probability"`            // 先验概率
	Likelihood           float64    `json:"likelihood"`                   // 似然
	PosteriorProbability float64    `json:"posterior_probability"`        // 后验概率
	EstimatedLifeDays    int        `json:"estimated_life_days"`          // 预计剩余寿命（天）
	Confidence           float64    `json:"confidence"`                   // 置信度 (0-1)
	FailDateEstimate     *time.Time `json:"fail_date_estimate,omitempty"` // 预计故障日期
	RiskFactors          []string   `json:"risk_factors"`                 // 风险因素
	PredictedAt          time.Time  `json:"predicted_at"`
}

// ============================================================
// 维护建议
// ============================================================

// AdvicePriority 建议优先级
type AdvicePriority string

const (
	PriorityUrgent AdvicePriority = "urgent" // 紧急
	PriorityHigh   AdvicePriority = "high"   // 高
	PriorityMedium AdvicePriority = "medium" // 中
	PriorityLow    AdvicePriority = "low"    // 低
	PriorityInfo   AdvicePriority = "info"   // 信息
)

// MaintenanceAdvice 维护建议
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
// 磁盘组管理
// ============================================================

// DiskGroup 磁盘组
type DiskGroup struct {
	ID           string      `json:"id"`
	Name         string      `json:"name"`          // 组名
	Type         string      `json:"type"`          // 类型：RAID组/存储池/独立
	Disks        []string    `json:"disks"`         // 磁盘设备列表
	GroupScore   float64     `json:"group_score"`   // 组级健康评分
	GroupGrade   HealthGrade `json:"group_grade"`   // 组级等级
	GroupStatus  DiskStatus  `json:"group_status"`  // 组级状态
	WeakestDisk  string      `json:"weakest_disk"`  // 最弱磁盘
	PriorityDisk string      `json:"priority_disk"` // 优先替换磁盘
	CreatedAt    time.Time   `json:"created_at"`
}

// ============================================================
// 历史数据
// ============================================================

// HealthHistoryPoint 历史健康数据点
type HealthHistoryPoint struct {
	Timestamp time.Time   `json:"timestamp"`
	Score     float64     `json:"score"`
	Grade     HealthGrade `json:"grade"`
	Status    DiskStatus  `json:"status"`
}

// HealthHistory 健康历史
type HealthHistory struct {
	Device string               `json:"device"`
	Points []HealthHistoryPoint `json:"points"`
	Period string               `json:"period"` // 时间范围描述
}

// ============================================================
// API 响应类型
// ============================================================

// DiskListItem 磁盘列表项
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
}

// DashboardData 仪表板数据
type DashboardData struct {
	TotalDisks    int       `json:"total_disks"`
	HealthyDisks  int       `json:"healthy_disks"`
	WarningDisks  int       `json:"warning_disks"`
	CriticalDisks int       `json:"critical_disks"`
	FailedDisks   int       `json:"failed_disks"`
	AverageScore  float64   `json:"average_score"`
	WorstDisk     string    `json:"worst_disk"`
	WorstScore    float64   `json:"worst_score"`
	Groups        int       `json:"groups_count"`
	Advices       int       `json:"advice_count"`
	GeneratedAt   time.Time `json:"generated_at"`
}

// ============================================================
// 请求/响应通用类型
// ============================================================

// APIResponse 通用 API 响应
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// ScanTriggerResponse 扫描触发响应
type ScanTriggerResponse struct {
	ScanID    string    `json:"scan_id"`
	Status    string    `json:"status"`
	StartedAt time.Time `json:"started_at"`
}
