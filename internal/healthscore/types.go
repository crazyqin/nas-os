// Package healthscore 提供存储健康评分仪表盘功能。
package healthscore

import (
	"errors"
	"time"
)

// ========== 错误定义 ==========

var (
	// ErrNoScoreData 尚未执行过评分.
	ErrNoScoreData = errors.New("尚未执行健康评分，请先调用 POST /calculate")
	// ErrScoringInProgress 评分任务正在进行中.
	ErrScoringInProgress = errors.New("评分任务正在执行中，请稍后再试")
)

// ========== 评分等级 ==========

// HealthLevel 健康等级.
type HealthLevel string

const (
	LevelExcellent HealthLevel = "excellent" // 优秀 90-100
	LevelGood      HealthLevel = "good"      // 良好 70-89
	LevelWarning   HealthLevel = "warning"   // 警告 50-69
	LevelCritical  HealthLevel = "critical"  // 严重 30-49
	LevelDanger    HealthLevel = "danger"    // 危险 0-29
)

// ClassifyLevel 根据分数返回等级.
func ClassifyLevel(score float64) HealthLevel {
	switch {
	case score >= 90:
		return LevelExcellent
	case score >= 70:
		return LevelGood
	case score >= 50:
		return LevelWarning
	case score >= 30:
		return LevelCritical
	default:
		return LevelDanger
	}
}

// ========== 评分请求 ==========

// CalculateRequest 评分计算请求.
type CalculateRequest struct {
	// Threshold 告警阈值，低于此值触发告警，默认50.
	Threshold float64 `json:"threshold"`
}

// ========== 评分子项 ==========

// CategoryScore 分项评分.
type CategoryScore struct {
	Name   string  `json:"name"`   // 类别名称
	Score  float64 `json:"score"`  // 评分 0-100
	Weight float64 `json:"weight"` // 权重
	Level  string  `json:"level"`  // 健康等级
	Detail string  `json:"detail"` // 评分说明
}

// ========== 健康建议 ==========

// Recommendation 健康建议.
type Recommendation struct {
	Category    string `json:"category"`    // 所属类别
	Severity    string `json:"severity"`    // 严重程度: high, medium, low
	Title       string `json:"title"`       // 建议标题
	Description string `json:"description"` // 详细描述
	Action      string `json:"action"`      // 建议操作
}

// ========== 告警记录 ==========

// Alert 告警记录.
type Alert struct {
	Timestamp time.Time `json:"timestamp"`
	Score     float64   `json:"score"`
	Threshold float64   `json:"threshold"`
	Message   string    `json:"message"`
}

// ========== 历史记录 ==========

// ScoreRecord 评分历史记录.
type ScoreRecord struct {
	Timestamp   time.Time        `json:"timestamp"`
	Overall     float64          `json:"overall"`
	Level       HealthLevel      `json:"level"`
	Categories  []CategoryScore  `json:"categories"`
}

// ========== 综合结果 ==========

// OverallScore 综合评分结果.
type OverallScore struct {
	Score          float64         `json:"score"`           // 综合评分 0-100
	Level          HealthLevel     `json:"level"`           // 健康等级
	Categories     []CategoryScore `json:"categories"`      // 分项评分
	Alerts         []Alert         `json:"alerts,omitempty"` // 未处理的告警
	EvaluatedAt    time.Time       `json:"evaluated_at"`
}

// ScoreDetails 评分详情.
type ScoreDetails struct {
	Overall     OverallScore    `json:"overall"`
	// SMART 磁盘状态.
	SmartStatus SmartStatusInfo `json:"smart_status"`
	// RAID 状态.
	RaidStatus  RaidStatusInfo  `json:"raid_status"`
	// 存储碎片化.
	Fragmentation FragmentationInfo `json:"fragmentation"`
	// 备份信息.
	BackupInfo  BackupInfo      `json:"backup_info"`
	// 数据老化信息.
	DataAgeInfo DataAgeInfo     `json:"data_age_info"`
}

// SmartStatusInfo 磁盘SMART状态.
type SmartStatusInfo struct {
	TotalDisks    int              `json:"total_disks"`
	HealthyDisks  int              `json:"healthy_disks"`
	DegradedDisks int              `json:"degraded_disks"`
	FailedDisks   int              `json:"failed_disks"`
	Disks         []DiskSmartInfo  `json:"disks"`
}

// DiskSmartInfo 单个磁盘SMART信息.
type DiskSmartInfo struct {
	Device    string  `json:"device"`
	Status    string  `json:"status"`    // passed, degraded, failed
	TempC     int     `json:"temp_c"`    // 温度℃
	PowerOnH  int     `json:"power_on_h"` // 通电小时
	Realloct  int     `json:"realloc_sectors"` // 重分配扇区
	Score     float64 `json:"score"`     // 该磁盘健康分
}

// RaidStatusInfo RAID状态.
type RaidStatusInfo struct {
	Level          string  `json:"level"`           // RAID 级别
	State          string  `json:"state"`           // clean, degraded, failed
	ActiveDisks    int     `json:"active_disks"`
	TotalDisks     int     `json:"total_disks"`
	Redundancy     float64 `json:"redundancy"`      // 冗余度 0-1
	Score          float64 `json:"score"`
}

// FragmentationInfo 存储碎片化信息.
type FragmentationInfo struct {
	TotalFragments int     `json:"total_fragments"`
	FragmentRatio  float64 `json:"fragment_ratio"` // 碎片率 0-1
	Score          float64 `json:"score"`
}

// BackupInfo 备份信息.
type BackupInfo struct {
	LastBackupTime  *time.Time `json:"last_backup_time"`
	BackupCount     int        `json:"backup_count"`
	Coverage        float64    `json:"coverage"`     // 覆盖率 0-1
	DaysSinceBackup int        `json:"days_since_backup"`
	Score           float64    `json:"score"`
}

// DataAgeInfo 数据老化信息.
type DataAgeInfo struct {
	OldestFileAge   string  `json:"oldest_file_age"`
	StaleDataRatio  float64 `json:"stale_data_ratio"` // 超过1年未访问的数据占比
	TotalFiles      int     `json:"total_files"`
	StaleFiles      int     `json:"stale_files"`
	Score           float64 `json:"score"`
}

// ========== 历史趋势 ==========

// HistoryQuery 历史查询参数.
type HistoryQuery struct {
	Days  int `form:"days"`  // 查询最近N天，默认30
	Limit int `form:"limit"` // 最大条数，默认100
}

// HistoryResponse 历史趋势响应.
type HistoryResponse struct {
	Records     []ScoreRecord `json:"records"`
	TotalCount  int           `json:"total_count"`
	AvgScore    float64       `json:"avg_score"`
	MinScore    float64       `json:"min_score"`
	MaxScore    float64       `json:"max_score"`
	Trend       string        `json:"trend"` // rising, falling, stable
}

// ========== 健康建议响应 ==========

// RecommendationsResponse 建议响应.
type RecommendationsResponse struct {
	OverallScore    float64          `json:"overall_score"`
	Level           HealthLevel      `json:"level"`
	Recommendations []Recommendation `json:"recommendations"`
	GeneratedAt     time.Time        `json:"generated_at"`
}
