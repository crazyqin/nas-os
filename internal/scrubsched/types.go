// Package scrubsched 提供智能Scrub调度功能，对标TrueNAS 26的定时Scrub避峰功能
package scrubsched

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

// Scrub调度相关错误.
var (
	// ErrPolicyNotFound 策略不存在.
	ErrPolicyNotFound = errors.New("调度策略不存在")
	// ErrPolicyExists 策略已存在（同名或同池重复策略）.
	ErrPolicyExists = errors.New("调度策略已存在")
	// ErrPoolNotFound 存储池不存在.
	ErrPoolNotFound = errors.New("存储池不存在")
	// ErrScrubNotRunning 当前池没有正在运行的Scrub.
	ErrScrubNotRunning = errors.New("当前存储池没有正在运行的Scrub")
	// ErrScrubAlreadyRunning 当前池已有Scrub在运行.
	ErrScrubAlreadyRunning = errors.New("当前存储池已有Scrub在运行")
	// ErrInvalidCronExpr 无效的cron表达式.
	ErrInvalidCronExpr = errors.New("无效的cron表达式")
	// ErrInvalidThreshold 无效的阈值配置.
	ErrInvalidThreshold = errors.New("无效的阈值配置")
	// ErrMaxDurationExceeded 超过最大执行时间限制.
	ErrMaxDurationExceeded = errors.New("超过最大执行时间限制")
	// ErrIOOverload IO负载过高，Scrub被暂停.
	ErrIOOverload = errors.New("IO负载过高，Scrub被暂停")
)

// ========== 调度策略类型 ==========

// ScheduleType 调度类型.
type ScheduleType string

// 调度类型常量.
const (
	// ScheduleWeekly 按周调度.
	ScheduleWeekly ScheduleType = "weekly"
	// ScheduleMonthly 按月调度.
	ScheduleMonthly ScheduleType = "monthly"
	// ScheduleCustom 自定义cron表达式.
	ScheduleCustom ScheduleType = "custom"
)

// ScrubState Scrub执行状态.
type ScrubState string

// Scrub状态常量.
const (
	// StateIdle 空闲.
	StateIdle ScrubState = "idle"
	// StateRunning 运行中.
	StateRunning ScrubState = "running"
	// StatePaused 暂停（避峰暂停）.
	StatePaused ScrubState = "paused"
	// StateCompleted 已完成.
	StateCompleted ScrubState = "completed"
	// StateFailed 失败.
	StateFailed ScrubState = "failed"
	// StateCancelled 已取消.
	StateCancelled ScrubState = "cancelled"
)

// ScrubPriority Scrub优先级.
type ScrubPriority int

// 优先级常量.
const (
	// PriorityLow 低优先级.
	PriorityLow ScrubPriority = 1
	// PriorityNormal 普通优先级.
	PriorityNormal ScrubPriority = 5
	// PriorityHigh 高优先级.
	PriorityHigh ScrubPriority = 10
)

// ========== 核心数据结构 ==========

// Policy Scrub调度策略.
type Policy struct {
	ID          string       `json:"id"`           // 策略唯一标识
	Name        string       `json:"name"`         // 策略名称
	PoolID      string       `json:"pool_id"`      // 关联的存储池ID
	Schedule    ScheduleType `json:"schedule"`      // 调度类型
	CronExpr    string       `json:"cron_expr"`    // cron表达式（custom类型时使用）
	WeekDay     int          `json:"week_day"`     // 星期几执行（weekly类型，0=周日）
	MonthDay    int          `json:"month_day"`    // 每月几号执行（monthly类型）
	Hour        int          `json:"hour"`         // 执行小时（0-23）
	Minute      int          `json:"minute"`       // 执行分钟（0-59）
	Priority    ScrubPriority `json:"priority"`    // 优先级
	Enabled     bool         `json:"enabled"`      // 是否启用
	// 避峰配置
	AvoidPeak   bool         `json:"avoid_peak"`   // 是否启用避峰调度
	PeakWindows []PeakWindow `json:"peak_windows"` // 高峰时段配置
	IOThreshold IOThreshold  `json:"io_threshold"` // IO负载阈值
	// 执行限制
	MaxDuration  time.Duration `json:"max_duration"`  // 最大执行时间
	ThrottleRate float64       `json:"throttle_rate"` // IO节流比例（0-1）
	// 健康整合
	HealthAdjust bool `json:"health_adjust"` // 是否根据磁盘健康调整频率
	// 元数据
	LastRun    *time.Time `json:"last_run"`    // 上次执行时间
	NextRun    *time.Time `json:"next_run"`    // 下次计划执行时间
	RunCount   int        `json:"run_count"`   // 累计执行次数
	CreatedAt  time.Time  `json:"created_at"`  // 创建时间
	UpdatedAt  time.Time  `json:"updated_at"`  // 更新时间
}

// PeakWindow 高峰时段窗口.
type PeakWindow struct {
	DayOfWeek int `json:"day_of_week"` // 星期几（0=周日，-1=每天）
	StartHour int `json:"start_hour"`  // 开始小时
	StartMin  int `json:"start_min"`   // 开始分钟
	EndHour   int `json:"end_hour"`    // 结束小时
	EndMin    int `json:"end_min"`     // 结束分钟
}

// IOThreshold IO负载阈值配置.
type IOThreshold struct {
	IOPSMax     int     `json:"iops_max"`     // IOPS上限
	BandwidthMax float64 `json:"bandwidth_max"` // 带宽上限（MB/s）
	LatencyMax  float64 `json:"latency_max"`  // 延迟上限（ms）
	ResumeRatio float64 `json:"resume_ratio"` // 恢复阈值比例（0-1，相对上限）
}

// ========== Scrub执行相关 ==========

// ScrubStatus Scrub当前状态.
type ScrubStatus struct {
	PoolID      string     `json:"pool_id"`      // 存储池ID
	State       ScrubState `json:"state"`         // 当前状态
	Progress    float64    `json:"progress"`      // 进度百分比（0-100）
	StartTime   *time.Time `json:"start_time"`   // 开始时间
	PauseTime   *time.Time `json:"pause_time"`   // 暂停时间
	ResumeTime  *time.Time `json:"resume_time"`  // 恢复时间
	ElapsedTime int64      `json:"elapsed_time"`  // 已用时间（秒）
	PauseCount  int        `json:"pause_count"`   // 暂停次数
	PauseReason string     `json:"pause_reason"`  // 暂停原因
	PolicyID    string     `json:"policy_id"`     // 关联策略ID
	IsManual    bool       `json:"is_manual"`     // 是否手动触发
}

// ScrubRecord Scrub执行历史记录.
type ScrubRecord struct {
	ID           string      `json:"id"`            // 记录唯一标识
	PoolID       string      `json:"pool_id"`       // 存储池ID
	PolicyID     string      `json:"policy_id"`     // 关联策略ID
	State        ScrubState  `json:"state"`         // 最终状态
	StartTime    time.Time   `json:"start_time"`    // 开始时间
	EndTime      time.Time   `json:"end_time"`      // 结束时间
	Duration     int64       `json:"duration"`      // 总耗时（秒）
	PauseCount   int         `json:"pause_count"`   // 暂停次数
	PauseTotalMs int64       `json:"pause_total_ms"` // 暂停总时长（毫秒）
	// 错误统计
	ErrorsFound  int         `json:"errors_found"`  // 发现的错误数
	ErrorsFixed  int         `json:"errors_fixed"`  // 修复的错误数
	// 性能指标
	SpeedMBps    float64     `json:"speed_mbps"`    // 平均速度（MB/s）
	IOPSImpact   float64     `json:"iops_impact"`   // IO影响（IOPS）
	IsManual     bool        `json:"is_manual"`     // 是否手动触发
	ErrorMessage string      `json:"error_message"` // 错误信息
}

// ========== IO负载相关 ==========

// IOLoad IO负载数据.
type IOLoad struct {
	Timestamp time.Time `json:"timestamp"`  // 采集时间
	PoolID    string    `json:"pool_id"`    // 存储池ID
	IOPS      int       `json:"iops"`       // 当前IOPS
	Bandwidth float64   `json:"bandwidth"`  // 带宽（MB/s）
	Latency   float64   `json:"latency"`    // 延迟（ms）
	ReadIOPS  int       `json:"read_iops"`  // 读IOPS
	WriteIOPS int       `json:"write_iops"` // 写IOPS
}

// IOLoadHistory IO负载历史.
type IOLoadHistory struct {
	PoolID  string   `json:"pool_id"`  // 存储池ID
	Records []IOLoad `json:"records"`   // 历史记录
}

// ========== 健康整合相关 ==========

// DiskHealth 磁盘健康状态.
type DiskHealth struct {
	DiskID      string  `json:"disk_id"`      // 磁盘标识
	PoolID      string  `json:"pool_id"`      // 所属池
	Health      string  `json:"health"`       // 健康状态：good/warning/critical
	Temperature int     `json:"temperature"`  // 温度（℃）
	RealloCount int     `json:"realloc_count"` // 重新分配扇区数
	PendingCount int    `json:"pending_count"` // 待处理扇区数
	UDMAErrors  int     `json:"udma_errors"`  // UDMA错误数
	SMARTErrors int     `json:"smart_errors"` // SMART错误数
}

// PoolHealthSummary 存储池健康汇总.
type PoolHealthSummary struct {
	PoolID       string       `json:"pool_id"`       // 存储池ID
	OverallHealth string      `json:"overall_health"` // 整体健康状态
	DiskCount    int          `json:"disk_count"`    // 磁盘数量
	HealthyDisks int          `json:"healthy_disks"` // 健康磁盘数
	WarningDisks int          `json:"warning_disks"` // 告警磁盘数
	CriticalDisks int         `json:"critical_disks"` // 严重磁盘数
	Disks        []DiskHealth `json:"disks"`         // 磁盘详情
}

// ========== 建议相关 ==========

// ScrubRecommendation Scrub调度建议.
type ScrubRecommendation struct {
	PoolID      string    `json:"pool_id"`      // 存储池ID
	PoolName    string    `json:"pool_name"`    // 存储池名称
	Reason      string    `json:"reason"`       // 建议原因
	SuggestedAt time.Time `json:"suggested_at"` // 建议执行时间
	Priority    string    `json:"priority"`     // 建议优先级：low/normal/urgent
	HealthNote  string    `json:"health_note"`  // 健康备注
}

// ========== 请求/响应结构 ==========

// CreatePolicyRequest 创建策略请求.
type CreatePolicyRequest struct {
	Name        string       `json:"name" binding:"required"`        // 策略名称
	PoolID      string       `json:"pool_id" binding:"required"`     // 存储池ID
	Schedule    ScheduleType `json:"schedule" binding:"required"`    // 调度类型
	CronExpr    string       `json:"cron_expr"`                      // cron表达式
	WeekDay     int          `json:"week_day"`                       // 星期几
	MonthDay    int          `json:"month_day"`                      // 每月几号
	Hour        int          `json:"hour"`                           // 执行小时
	Minute      int          `json:"minute"`                         // 执行分钟
	Priority    ScrubPriority `json:"priority"`                      // 优先级
	Enabled     bool         `json:"enabled"`                        // 是否启用
	AvoidPeak   bool         `json:"avoid_peak"`                     // 避峰调度
	PeakWindows []PeakWindow `json:"peak_windows"`                   // 高峰时段
	IOThreshold IOThreshold  `json:"io_threshold"`                   // IO阈值
	MaxDuration int          `json:"max_duration_hours"`             // 最大执行时间（小时）
	ThrottleRate float64     `json:"throttle_rate"`                  // 节流比例
	HealthAdjust bool        `json:"health_adjust"`                  // 健康调整
}

// UpdatePolicyRequest 更新策略请求.
type UpdatePolicyRequest struct {
	Name        *string       `json:"name"`           // 策略名称
	Schedule    *ScheduleType `json:"schedule"`       // 调度类型
	CronExpr    *string       `json:"cron_expr"`      // cron表达式
	WeekDay     *int          `json:"week_day"`       // 星期几
	MonthDay    *int          `json:"month_day"`      // 每月几号
	Hour        *int          `json:"hour"`           // 执行小时
	Minute      *int          `json:"minute"`         // 执行分钟
	Priority    *ScrubPriority `json:"priority"`      // 优先级
	Enabled     *bool         `json:"enabled"`        // 是否启用
	AvoidPeak   *bool         `json:"avoid_peak"`     // 避峰调度
	PeakWindows []PeakWindow  `json:"peak_windows"`   // 高峰时段
	IOThreshold *IOThreshold  `json:"io_threshold"`   // IO阈值
	MaxDuration *int          `json:"max_duration_hours"` // 最大执行时间（小时）
	ThrottleRate *float64     `json:"throttle_rate"`  // 节流比例
	HealthAdjust *bool        `json:"health_adjust"`  // 健康调整
}

// ScrubStatusResponse Scrub状态响应.
type ScrubStatusResponse struct {
	Status map[string]*ScrubStatus `json:"status"` // 各池状态
}

// IOCurrentResponse 当前IO负载响应.
type IOCurrentResponse struct {
	Loads map[string]*IOLoad `json:"loads"` // 各池IO负载
}

// RecommendationsResponse 建议响应.
type RecommendationsResponse struct {
	Recommendations []ScrubRecommendation `json:"recommendations"` // 建议列表
}

// HistoryResponse 历史记录响应.
type HistoryResponse struct {
	Records []ScrubRecord `json:"records"` // 历史记录
	Total   int           `json:"total"`   // 总数
}
