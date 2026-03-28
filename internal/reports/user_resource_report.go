// Package reports 提供报表生成和管理功能
package reports

import (
	"time"
)

// ========== 用户/部门资源使用报表 v2.60.0 ==========

// UserResourceReport 用户资源使用报表.
type UserResourceReport struct {
	// 报表ID
	ID string `json:"id"`

	// 报表名称
	Name string `json:"name"`

	// 生成时间
	GeneratedAt time.Time `json:"generated_at"`

	// 报表周期
	Period ReportPeriod `json:"period"`

	// 用户资源使用汇总
	Summary UserResourceSummary `json:"summary"`

	// 用户详情列表
	Users []UserResourceDetail `json:"users"`

	// 部门汇总
	Departments []DepartmentResourceSummary `json:"departments"`

	// 资源使用排名
	Rankings UserResourceRankings `json:"rankings"`

	// 趋势数据
	Trend []UserResourceTrendPoint `json:"trend"`

	// 异常用户
	Anomalies []UserResourceAnomaly `json:"anomalies"`

	// 图表数据
	Charts []UserResourceChart `json:"charts"`
}

// UserResourceSummary 用户资源汇总.
type UserResourceSummary struct {
	// 用户总数
	TotalUsers int `json:"total_users"`

	// 活跃用户数
	ActiveUsers int `json:"active_users"`

	// 非活跃用户数
	InactiveUsers int `json:"inactive_users"`

	// 超配额用户数
	OverQuotaUsers int `json:"over_quota_users"`

	// 总存储使用量（字节）
	TotalStorageUsed uint64 `json:"total_storage_used"`

	// 总配额（字节）
	TotalQuota uint64 `json:"total_quota"`

	// 平均使用量（字节）
	AvgUsagePerUser uint64 `json:"avg_usage_per_user"`

	// 平均配额使用率（%）
	AvgQuotaUsagePercent float64 `json:"avg_quota_usage_percent"`

	// 文件总数
	TotalFiles uint64 `json:"total_files"`

	// 目录总数
	TotalDirectories uint64 `json:"total_directories"`

	// 本期新增用户数
	NewUsers int `json:"new_users"`

	// 本期删除用户数
	DeletedUsers int `json:"deleted_users"`
}

// UserResourceDetail 用户资源详情.
type UserResourceDetail struct {
	// 用户名
	Username string `json:"username"`

	// 显示名称
	DisplayName string `json:"display_name"`

	// 用户ID
	UserID string `json:"user_id"`

	// 部门
	Department string `json:"department"`

	// 邮箱
	Email string `json:"email"`

	// 配额限制（字节）
	QuotaLimit uint64 `json:"quota_limit"`

	// 软限制（字节）
	SoftLimit uint64 `json:"soft_limit"`

	// 硬限制（字节）
	HardLimit uint64 `json:"hard_limit"`

	// 已使用（字节）
	UsedBytes uint64 `json:"used_bytes"`

	// 可用（字节）
	AvailableBytes uint64 `json:"available_bytes"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 配额使用状态
	QuotaStatus string `json:"quota_status"` // normal, warning, exceeded

	// 文件数量
	FileCount uint64 `json:"file_count"`

	// 目录数量
	DirectoryCount uint64 `json:"directory_count"`

	// 平均文件大小（字节）
	AvgFileSize uint64 `json:"avg_file_size"`

	// 最大文件大小（字节）
	MaxFileSize uint64 `json:"max_file_size"`

	// 文件类型分布
	FileTypes []FileTypeUsage `json:"file_types"`

	// 最近访问时间
	LastAccessTime *time.Time `json:"last_access_time,omitempty"`

	// 最近修改时间
	LastModifyTime *time.Time `json:"last_modify_time,omitempty"`

	// 创建时间
	CreatedTime *time.Time `json:"created_time,omitempty"`

	// 活跃状态
	IsActive bool `json:"is_active"`

	// 使用趋势（最近7天）
	RecentTrend []DailyUsagePoint `json:"recent_trend"`

	// 增长率（%）
	GrowthRate float64 `json:"growth_rate"`
}

// FileTypeUsage 文件类型使用情况.
type FileTypeUsage struct {
	// 文件类型
	Type string `json:"type"` // document, image, video, audio, archive, other

	// 文件数量
	Count uint64 `json:"count"`

	// 总大小（字节）
	Size uint64 `json:"size"`

	// 占比（%）
	Percent float64 `json:"percent"`
}

// DailyUsagePoint 每日使用数据点.
type DailyUsagePoint struct {
	// 日期
	Date string `json:"date"`

	// 使用量（字节）
	UsedBytes uint64 `json:"used_bytes"`

	// 文件数
	FileCount uint64 `json:"file_count"`

	// 增量（字节）
	DeltaBytes int64 `json:"delta_bytes"`
}

// DepartmentResourceSummary 部门资源汇总.
type DepartmentResourceSummary struct {
	// 部门名称
	Name string `json:"name"`

	// 部门ID
	ID string `json:"id"`

	// 用户数量
	UserCount int `json:"user_count"`

	// 活跃用户数
	ActiveUserCount int `json:"active_user_count"`

	// 总配额（字节）
	TotalQuota uint64 `json:"total_quota"`

	// 总使用量（字节）
	TotalUsed uint64 `json:"total_used"`

	// 使用率（%）
	UsagePercent float64 `json:"usage_percent"`

	// 平均用户使用量
	AvgUserUsage uint64 `json:"avg_user_usage"`

	// 文件总数
	TotalFiles uint64 `json:"total_files"`

	// 超配额用户数
	OverQuotaCount int `json:"over_quota_count"`

	// 部门管理员
	Admins []string `json:"admins"`

	// 部门成员
	Members []string `json:"members,omitempty"`
}

// UserResourceRankings 用户资源排名.
type UserResourceRankings struct {
	// 存储使用量排名
	TopStorageUsers []UserRankingItem `json:"top_storage_users"`

	// 文件数量排名
	TopFileUsers []UserRankingItem `json:"top_file_users"`

	// 增长最快用户
	TopGrowthUsers []UserRankingItem `json:"top_growth_users"`

	// 配额使用率最高
	TopQuotaUsageUsers []UserRankingItem `json:"top_quota_usage_users"`

	// 最不活跃用户
	TopInactiveUsers []UserRankingItem `json:"top_inactive_users"`
}

// UserRankingItem 用户排名项.
type UserRankingItem struct {
	// 用户名
	Username string `json:"username"`

	// 显示名称
	DisplayName string `json:"display_name"`

	// 部门
	Department string `json:"department"`

	// 值
	Value float64 `json:"value"`

	// 单位
	Unit string `json:"unit"`

	// 排名
	Rank int `json:"rank"`
}

// UserResourceTrendPoint 用户资源趋势数据点.
type UserResourceTrendPoint struct {
	// 时间
	Timestamp time.Time `json:"timestamp"`

	// 总用户数
	TotalUsers int `json:"total_users"`

	// 活跃用户数
	ActiveUsers int `json:"active_users"`

	// 总使用量（字节）
	TotalUsed uint64 `json:"total_used"`

	// 平均使用量
	AvgUsage uint64 `json:"avg_usage"`

	// 平均使用率（%）
	AvgUsagePercent float64 `json:"avg_usage_percent"`
}

// UserResourceAnomaly 用户资源异常.
type UserResourceAnomaly struct {
	// 用户名
	Username string `json:"username"`

	// 异常类型
	Type string `json:"type"` // quota_exceeded, unusual_growth, inactive, large_files

	// 严重级别
	Severity string `json:"severity"` // info, warning, critical

	// 描述
	Description string `json:"description"`

	// 当前值
	CurrentValue float64 `json:"current_value"`

	// 阈值
	Threshold float64 `json:"threshold"`

	// 建议操作
	SuggestedAction string `json:"suggested_action"`
}

// UserResourceChart 用户资源图表.
type UserResourceChart struct {
	// 图表ID
	ID string `json:"id"`

	// 类型
	Type string `json:"type"`

	// 标题
	Title string `json:"title"`

	// 数据
	Data interface{} `json:"data"`
}

// UserResourceReportGenerator 用户资源报表生成器.
type UserResourceReportGenerator struct {
	users       []UserResourceDetail
	departments []DepartmentResourceSummary
	trendData   []UserResourceTrendPoint
}

// NewUserResourceReportGenerator 创建用户资源报表生成器.
func NewUserResourceReportGenerator() *UserResourceReportGenerator {
	return &UserResourceReportGenerator{}
}

// SetUsers 设置用户数据.
func (g *UserResourceReportGenerator) SetUsers(users []UserResourceDetail) {
	g.users = users
}

// SetDepartments 设置部门数据.
func (g *UserResourceReportGenerator) SetDepartments(departments []DepartmentResourceSummary) {
	g.departments = departments
}

// SetTrendData 设置趋势数据.
func (g *UserResourceReportGenerator) SetTrendData(data []UserResourceTrendPoint) {
	g.trendData = data
}

// Generate 生成报表.
func (g *UserResourceReportGenerator) Generate(period ReportPeriod) *UserResourceReport {
	now := time.Now()

	summary := g.generateSummary()
	rankings := g.generateRankings()
	anomalies := g.detectAnomalies()
	charts := g.generateCharts()

	return &UserResourceReport{
		ID:          "user_resource_" + now.Format("20060102150405"),
		Name:        "用户资源使用报表",
		GeneratedAt: now,
		Period:      period,
		Summary:     summary,
		Users:       g.users,
		Departments: g.departments,
		Rankings:    rankings,
		Trend:       g.trendData,
		Anomalies:   anomalies,
		Charts:      charts,
	}
}

// generateSummary 生成汇总.
func (g *UserResourceReportGenerator) generateSummary() UserResourceSummary {
	summary := UserResourceSummary{
		TotalUsers:     len(g.users),
		ActiveUsers:    0,
		InactiveUsers:  0,
		OverQuotaUsers: 0,
	}

	var totalUsed uint64
	var totalQuota uint64
	var totalFiles uint64
	var totalDirs uint64

	for _, user := range g.users {
		totalUsed += user.UsedBytes
		totalQuota += user.QuotaLimit
		totalFiles += user.FileCount
		totalDirs += user.DirectoryCount

		if user.IsActive {
			summary.ActiveUsers++
		} else {
			summary.InactiveUsers++
		}

		if user.QuotaStatus == "exceeded" {
			summary.OverQuotaUsers++
		}
	}

	summary.TotalStorageUsed = totalUsed
	summary.TotalQuota = totalQuota
	summary.TotalFiles = totalFiles
	summary.TotalDirectories = totalDirs

	if summary.TotalUsers > 0 {
		summary.AvgUsagePerUser = totalUsed / uint64(summary.TotalUsers)
	}

	if totalQuota > 0 {
		summary.AvgQuotaUsagePercent = round(float64(totalUsed)/float64(totalQuota)*100, 1)
	}

	return summary
}

// generateRankings 生成排名.
func (g *UserResourceReportGenerator) generateRankings() UserResourceRankings {
	rankings := UserResourceRankings{}

	// 存储使用量排名
	topStorage := make([]UserRankingItem, 0)
	for _, u := range g.users {
		topStorage = append(topStorage, UserRankingItem{
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Department:  u.Department,
			Value:       float64(u.UsedBytes),
			Unit:        "bytes",
		})
	}
	// 排序并取前10
	topStorage = sortAndLimit(topStorage, 10)
	for i := range topStorage {
		topStorage[i].Rank = i + 1
	}
	rankings.TopStorageUsers = topStorage

	// 文件数量排名
	topFiles := make([]UserRankingItem, 0)
	for _, u := range g.users {
		topFiles = append(topFiles, UserRankingItem{
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Department:  u.Department,
			Value:       float64(u.FileCount),
			Unit:        "files",
		})
	}
	topFiles = sortAndLimit(topFiles, 10)
	for i := range topFiles {
		topFiles[i].Rank = i + 1
	}
	rankings.TopFileUsers = topFiles

	// 增长最快用户
	topGrowth := make([]UserRankingItem, 0)
	for _, u := range g.users {
		if u.GrowthRate > 0 {
			topGrowth = append(topGrowth, UserRankingItem{
				Username:    u.Username,
				DisplayName: u.DisplayName,
				Department:  u.Department,
				Value:       u.GrowthRate,
				Unit:        "percent",
			})
		}
	}
	topGrowth = sortAndLimit(topGrowth, 10)
	for i := range topGrowth {
		topGrowth[i].Rank = i + 1
	}
	rankings.TopGrowthUsers = topGrowth

	// 配额使用率最高
	topQuota := make([]UserRankingItem, 0)
	for _, u := range g.users {
		topQuota = append(topQuota, UserRankingItem{
			Username:    u.Username,
			DisplayName: u.DisplayName,
			Department:  u.Department,
			Value:       u.UsagePercent,
			Unit:        "percent",
		})
	}
	topQuota = sortAndLimit(topQuota, 10)
	for i := range topQuota {
		topQuota[i].Rank = i + 1
	}
	rankings.TopQuotaUsageUsers = topQuota

	return rankings
}

// detectAnomalies 检测异常.
func (g *UserResourceReportGenerator) detectAnomalies() []UserResourceAnomaly {
	anomalies := make([]UserResourceAnomaly, 0)

	for _, user := range g.users {
		// 配额超限
		if user.UsagePercent > 100 {
			anomalies = append(anomalies, UserResourceAnomaly{
				Username:        user.Username,
				Type:            "quota_exceeded",
				Severity:        "critical",
				Description:     "用户已超出配额限制",
				CurrentValue:    user.UsagePercent,
				Threshold:       100,
				SuggestedAction: "请联系用户清理文件或增加配额",
			})
		} else if user.UsagePercent > 90 {
			anomalies = append(anomalies, UserResourceAnomaly{
				Username:        user.Username,
				Type:            "quota_warning",
				Severity:        "warning",
				Description:     "用户配额使用率超过90%",
				CurrentValue:    user.UsagePercent,
				Threshold:       90,
				SuggestedAction: "建议通知用户清理不必要的文件",
			})
		}

		// 异常增长
		if user.GrowthRate > 50 {
			anomalies = append(anomalies, UserResourceAnomaly{
				Username:        user.Username,
				Type:            "unusual_growth",
				Severity:        "warning",
				Description:     "用户存储使用量异常增长",
				CurrentValue:    user.GrowthRate,
				Threshold:       50,
				SuggestedAction: "建议检查用户是否上传了大量文件",
			})
		}

		// 不活跃用户
		if !user.IsActive {
			anomalies = append(anomalies, UserResourceAnomaly{
				Username:        user.Username,
				Type:            "inactive",
				Severity:        "info",
				Description:     "用户长期未活跃",
				CurrentValue:    0,
				Threshold:       0,
				SuggestedAction: "可考虑归档或清理用户数据",
			})
		}
	}

	return anomalies
}

// generateCharts 生成图表.
func (g *UserResourceReportGenerator) generateCharts() []UserResourceChart {
	charts := make([]UserResourceChart, 0)

	// 部门分布图
	deptData := make([]map[string]interface{}, 0)
	for _, dept := range g.departments {
		deptData = append(deptData, map[string]interface{}{
			"name":  dept.Name,
			"value": dept.TotalUsed,
			"users": dept.UserCount,
		})
	}
	charts = append(charts, UserResourceChart{
		ID:    "chart_dept_distribution",
		Type:  "bar",
		Title: "部门存储分布",
		Data:  deptData,
	})

	// 用户使用量分布
	userDistData := make([]map[string]interface{}, 0)
	ranges := []struct {
		label string
		min   float64
		max   float64
	}{
		{"0-20%", 0, 20},
		{"20-40%", 20, 40},
		{"40-60%", 40, 60},
		{"60-80%", 60, 80},
		{"80-100%", 80, 100},
		{">100%", 100, 999},
	}

	for _, r := range ranges {
		count := 0
		for _, u := range g.users {
			if u.UsagePercent >= r.min && u.UsagePercent < r.max {
				count++
			}
		}
		if r.max == 999 {
			count = 0
			for _, u := range g.users {
				if u.UsagePercent >= 100 {
					count++
				}
			}
		}
		userDistData = append(userDistData, map[string]interface{}{
			"range": r.label,
			"count": count,
		})
	}
	charts = append(charts, UserResourceChart{
		ID:    "chart_usage_distribution",
		Type:  "histogram",
		Title: "配额使用率分布",
		Data:  userDistData,
	})

	return charts
}

// 排序并限制数量
func sortAndLimit(items []UserRankingItem, limit int) []UserRankingItem {
	if len(items) == 0 {
		return items
	}

	// 简单冒泡排序（按值降序）
	for i := 0; i < len(items)-1; i++ {
		for j := i + 1; j < len(items); j++ {
			if items[j].Value > items[i].Value {
				items[i], items[j] = items[j], items[i]
			}
		}
	}

	if len(items) > limit {
		items = items[:limit]
	}

	return items
}
