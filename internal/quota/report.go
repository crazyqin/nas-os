// Package quota 提供存储配额管理功能
package quota

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
)

// ReportGenerator 报告生成器
type ReportGenerator struct {
	manager     *Manager
	monitor     *Monitor
	cleanup     *CleanupManager
	cron        *cron.Cron
	scheduledID cron.EntryID
	mu          sync.Mutex
}

// ScheduledReport 定时报告配置
type ScheduledReport struct {
	Request    ReportRequest
	Schedule   string
	OutputPath string
	LastRun    time.Time
	NextRun    time.Time
	Enabled    bool
}

// NewReportGenerator 创建报告生成器
func NewReportGenerator(manager *Manager, monitor *Monitor, cleanup *CleanupManager) *ReportGenerator {
	cronInstance := cron.New(cron.WithSeconds())
	cronInstance.Start()

	return &ReportGenerator{
		manager: manager,
		monitor: monitor,
		cleanup: cleanup,
		cron:    cronInstance,
	}
}

// Stop 停止定时任务
func (g *ReportGenerator) Stop() {
	g.mu.Lock()
	defer g.mu.Unlock()

	if g.cron != nil {
		g.cron.Stop()
	}
}

// ScheduleReport 定时生成报告
// schedule 格式：秒 分 时 日 月 周 (例如："0 0 8 * * *" 表示每天早上 8 点)
func (g *ReportGenerator) ScheduleReport(req ReportRequest, schedule string, outputPath string) error {
	g.mu.Lock()
	defer g.mu.Unlock()

	// 如果已有定时任务，先取消
	if g.scheduledID != 0 {
		g.cron.Remove(g.scheduledID)
	}

	// 解析 schedule 验证格式（支持秒级）
	parser := cron.NewParser(cron.Second | cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
	_, err := parser.Parse(schedule)
	if err != nil {
		return fmt.Errorf("无效的 cron 表达式：%v", err)
	}

	// 创建定时任务
	g.scheduledID, _ = g.cron.AddFunc(schedule, func() {
		g.generateAndExport(req, outputPath)
	})

	// 获取下次执行时间
	entry := g.cron.Entry(g.scheduledID)
	fmt.Printf("[quota] 定时报告已调度：%s - 下次执行：%s\n", schedule, entry.Next.Format("2006-01-02 15:04:05"))

	return nil
}

// generateAndExport 生成并导出报告
func (g *ReportGenerator) generateAndExport(req ReportRequest, outputPath string) {
	report, err := g.GenerateReport(req)
	if err != nil {
		fmt.Printf("[quota] 生成报告失败：%v\n", err)
		return
	}

	// 创建输出目录
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Printf("[quota] 创建目录失败：%v\n", err)
		return
	}

	// 导出报告
	if err := g.ExportReport(report, outputPath); err != nil {
		fmt.Printf("[quota] 导出报告失败：%v\n", err)
		return
	}

	fmt.Printf("[quota] 定时报告已生成：%s\n", outputPath)
}

// GenerateReport 生成报告
func (g *ReportGenerator) GenerateReport(req ReportRequest) (*Report, error) {
	// 设置默认值
	if req.Format == "" {
		req.Format = ReportFormatJSON
	}

	// 设置时间范围
	period := ReportPeriod{
		StartTime: time.Now().AddDate(0, 0, -7), // 默认最近 7 天
		EndTime:   time.Now(),
	}
	if req.StartTime != nil {
		period.StartTime = *req.StartTime
	}
	if req.EndTime != nil {
		period.EndTime = *req.EndTime
	}

	report := &Report{
		ID:          generateID(),
		Type:        req.Type,
		Format:      req.Format,
		GeneratedAt: time.Now(),
		Period:      period,
	}

	// 生成摘要
	report.Summary = g.generateSummary(period)

	// 根据类型生成详情
	switch req.Type {
	case ReportTypeSummary:
		report.Details = g.generateSummaryDetails(period)

	case ReportTypeUser:
		details, err := g.generateUserReport(req.UserID, period)
		if err != nil {
			return nil, err
		}
		report.Details = details

	case ReportTypeGroup:
		details, err := g.generateGroupReport(req.GroupID, period)
		if err != nil {
			return nil, err
		}
		report.Details = details

	case ReportTypeVolume:
		details, err := g.generateVolumeReport(req.VolumeName, period)
		if err != nil {
			return nil, err
		}
		report.Details = details

	case ReportTypeTrend:
		details, err := g.generateTrendReport(period)
		if err != nil {
			return nil, err
		}
		report.Details = details

	default:
		return nil, fmt.Errorf("不支持的报告类型: %s", req.Type)
	}

	return report, nil
}

// generateSummary 生成报告摘要
func (g *ReportGenerator) generateSummary(period ReportPeriod) ReportSummary {
	usages, _ := g.manager.GetAllUsage()
	alerts := g.manager.GetAlerts()

	summary := ReportSummary{
		TotalQuotas: len(usages),
	}

	for _, usage := range usages {
		summary.TotalLimitBytes += usage.HardLimit
		summary.TotalUsedBytes += usage.UsedBytes
		summary.TotalFreeBytes += usage.Available

		if usage.IsOverSoft {
			summary.OverSoftLimit++
		}
		if usage.IsOverHard {
			summary.OverHardLimit++
		}
	}

	if summary.TotalLimitBytes > 0 {
		summary.AverageUsage = float64(summary.TotalUsedBytes) / float64(summary.TotalLimitBytes) * 100
	}

	summary.ActiveAlerts = len(alerts)

	// 获取清理统计
	if g.cleanup != nil {
		stats := g.cleanup.GetCleanupStats()
		summary.CleanupTasksRun = stats["total_tasks"].(int)
		summary.BytesCleaned = stats["total_bytes_freed"].(uint64)
	}

	return summary
}

// generateSummaryDetails 生成汇总报告详情
func (g *ReportGenerator) generateSummaryDetails(period ReportPeriod) interface{} {
	usages, _ := g.manager.GetAllUsage()
	alerts := g.manager.GetAlerts()

	// 按卷分组
	volumeMap := make(map[string][]QuotaUsage)
	for _, usage := range usages {
		volumeMap[usage.VolumeName] = append(volumeMap[usage.VolumeName], *usage)
	}

	return map[string]interface{}{
		"quotas_by_volume": volumeMap,
		"active_alerts":    alerts,
		"top_users":        g.getTopUsers(usages, 10),
		"top_groups":       g.getTopGroups(usages, 10),
	}
}

// generateUserReport 生成用户配额报告
func (g *ReportGenerator) generateUserReport(userID string, period ReportPeriod) ([]UserQuotaReport, error) {
	var reports []UserQuotaReport

	if userID != "" {
		// 单个用户
		usages, err := g.manager.GetUserUsage(userID)
		if err != nil {
			return nil, err
		}

		report := UserQuotaReport{
			Username: userID,
		}

		for _, usage := range usages {
			report.Quotas = append(report.Quotas, *usage)
			report.TotalLimit += usage.HardLimit
			report.TotalUsed += usage.UsedBytes
			report.TotalAvailable += usage.Available
		}

		if report.TotalLimit > 0 {
			report.UsagePercent = float64(report.TotalUsed) / float64(report.TotalLimit) * 100
		}

		reports = append(reports, report)
	} else {
		// 所有用户
		usages, _ := g.manager.GetAllUsage()
		userMap := make(map[string][]QuotaUsage)

		for _, usage := range usages {
			if usage.Type == QuotaTypeUser {
				userMap[usage.TargetID] = append(userMap[usage.TargetID], *usage)
			}
		}

		for username, quotas := range userMap {
			report := UserQuotaReport{
				Username: username,
				Quotas:   quotas,
			}

			for _, usage := range quotas {
				report.TotalLimit += usage.HardLimit
				report.TotalUsed += usage.UsedBytes
				report.TotalAvailable += usage.Available
			}

			if report.TotalLimit > 0 {
				report.UsagePercent = float64(report.TotalUsed) / float64(report.TotalLimit) * 100
			}

			reports = append(reports, report)
		}
	}

	// 按使用量排序
	sort.Slice(reports, func(i, j int) bool {
		return reports[i].TotalUsed > reports[j].TotalUsed
	})

	return reports, nil
}

// generateGroupReport 生成用户组配额报告
func (g *ReportGenerator) generateGroupReport(groupID string, period ReportPeriod) (interface{}, error) {
	usages, _ := g.manager.GetAllUsage()

	if groupID != "" {
		// 单个用户组
		var groupUsages []QuotaUsage
		for _, usage := range usages {
			if usage.Type == QuotaTypeGroup && usage.TargetID == groupID {
				groupUsages = append(groupUsages, *usage)
			}
		}
		return groupUsages, nil
	}

	// 所有用户组
	groupMap := make(map[string][]QuotaUsage)
	for _, usage := range usages {
		if usage.Type == QuotaTypeGroup {
			groupMap[usage.TargetID] = append(groupMap[usage.TargetID], *usage)
		}
	}

	return groupMap, nil
}

// generateVolumeReport 生成卷配额报告
func (g *ReportGenerator) generateVolumeReport(volumeName string, period ReportPeriod) ([]VolumeQuotaReport, error) {
	usages, _ := g.manager.GetAllUsage()
	alerts := g.manager.GetAlerts()

	if volumeName != "" {
		// 单个卷
		report := VolumeQuotaReport{
			VolumeName: volumeName,
		}

		for _, usage := range usages {
			if usage.VolumeName != volumeName {
				continue
			}

			report.TotalLimit += usage.HardLimit
			report.TotalUsed += usage.UsedBytes
			report.TotalFree += usage.Available

			if usage.Type == QuotaTypeUser {
				report.UserQuotas = append(report.UserQuotas, *usage)
			} else {
				report.GroupQuotas = append(report.GroupQuotas, *usage)
			}
		}

		// 添加相关告警
		for _, alert := range alerts {
			if alert.VolumeName == volumeName {
				report.ActiveAlerts = append(report.ActiveAlerts, *alert)
			}
		}

		return []VolumeQuotaReport{report}, nil
	}

	// 所有卷
	volumeMap := make(map[string]*VolumeQuotaReport)

	for _, usage := range usages {
		if _, exists := volumeMap[usage.VolumeName]; !exists {
			volumeMap[usage.VolumeName] = &VolumeQuotaReport{
				VolumeName: usage.VolumeName,
			}
		}

		report := volumeMap[usage.VolumeName]
		report.TotalLimit += usage.HardLimit
		report.TotalUsed += usage.UsedBytes
		report.TotalFree += usage.Available

		if usage.Type == QuotaTypeUser {
			report.UserQuotas = append(report.UserQuotas, *usage)
		} else {
			report.GroupQuotas = append(report.GroupQuotas, *usage)
		}
	}

	// 添加告警
	for _, alert := range alerts {
		if report, exists := volumeMap[alert.VolumeName]; exists {
			report.ActiveAlerts = append(report.ActiveAlerts, *alert)
		}
	}

	// 转换为切片
	reports := make([]VolumeQuotaReport, 0, len(volumeMap))
	for _, report := range volumeMap {
		reports = append(reports, *report)
	}

	return reports, nil
}

// generateTrendReport 生成趋势报告
func (g *ReportGenerator) generateTrendReport(period ReportPeriod) ([]QuotaTrend, error) {
	usages, _ := g.manager.GetAllUsage()
	trends := make([]QuotaTrend, 0)

	for _, usage := range usages {
		trend := QuotaTrend{
			QuotaID:    usage.QuotaID,
			TargetName: usage.TargetName,
		}

		// 获取趋势数据
		if g.monitor != nil {
			trend.DataPoints = g.monitor.GetTrend(usage.QuotaID, period.EndTime.Sub(period.StartTime))
			trend.GrowthRate = g.monitor.CalculateGrowthRate(usage.QuotaID)
			trend.ProjectedDaysToFull = g.monitor.PredictFullTime(usage.QuotaID, usage.HardLimit)
		}

		trends = append(trends, trend)
	}

	return trends, nil
}

// getTopUsers 获取使用量最高的用户
func (g *ReportGenerator) getTopUsers(usages []*QuotaUsage, limit int) []QuotaUsage {
	userMap := make(map[string]*QuotaUsage)

	for _, usage := range usages {
		if usage.Type != QuotaTypeUser {
			continue
		}

		if existing, exists := userMap[usage.TargetID]; exists {
			existing.UsedBytes += usage.UsedBytes
		} else {
			copy := *usage
			userMap[usage.TargetID] = &copy
		}
	}

	// 转换为切片并排序
	users := make([]QuotaUsage, 0, len(userMap))
	for _, usage := range userMap {
		users = append(users, *usage)
	}

	sort.Slice(users, func(i, j int) bool {
		return users[i].UsedBytes > users[j].UsedBytes
	})

	if limit > 0 && len(users) > limit {
		users = users[:limit]
	}

	return users
}

// getTopGroups 获取使用量最高的用户组
func (g *ReportGenerator) getTopGroups(usages []*QuotaUsage, limit int) []QuotaUsage {
	groupMap := make(map[string]*QuotaUsage)

	for _, usage := range usages {
		if usage.Type != QuotaTypeGroup {
			continue
		}

		if existing, exists := groupMap[usage.TargetID]; exists {
			existing.UsedBytes += usage.UsedBytes
		} else {
			copy := *usage
			groupMap[usage.TargetID] = &copy
		}
	}

	groups := make([]QuotaUsage, 0, len(groupMap))
	for _, usage := range groupMap {
		groups = append(groups, *usage)
	}

	sort.Slice(groups, func(i, j int) bool {
		return groups[i].UsedBytes > groups[j].UsedBytes
	})

	if limit > 0 && len(groups) > limit {
		groups = groups[:limit]
	}

	return groups
}

// ========== 报告导出 ==========

// ExportReport 导出报告
func (g *ReportGenerator) ExportReport(report *Report, outputPath string) error {
	switch report.Format {
	case ReportFormatJSON:
		return g.exportJSON(report, outputPath)
	case ReportFormatCSV:
		return g.exportCSV(report, outputPath)
	case ReportFormatHTML:
		return g.exportHTML(report, outputPath)
	default:
		return fmt.Errorf("不支持的导出格式: %s", report.Format)
	}
}

// exportJSON 导出 JSON 格式
func (g *ReportGenerator) exportJSON(report *Report, outputPath string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outputPath, data, 0644)
}

// exportCSV 导出 CSV 格式
func (g *ReportGenerator) exportCSV(report *Report, outputPath string) error {
	file, err := os.Create(outputPath)
	if err != nil {
		return err
	}
	defer func() { _ = file.Close() }()

	writer := csv.NewWriter(file)
	defer writer.Flush()

	// 写入头部
	headers := []string{"报告ID", "类型", "生成时间", "总配额数", "总限制(字节)", "总使用(字节)", "平均使用率", "超软限制数", "超硬限制数", "活跃告警"}
	_ = writer.Write(headers)

	// 写入数据
	row := []string{
		report.ID,
		string(report.Type),
		report.GeneratedAt.Format("2006-01-02 15:04:05"),
		fmt.Sprintf("%d", report.Summary.TotalQuotas),
		fmt.Sprintf("%d", report.Summary.TotalLimitBytes),
		fmt.Sprintf("%d", report.Summary.TotalUsedBytes),
		fmt.Sprintf("%.2f%%", report.Summary.AverageUsage),
		fmt.Sprintf("%d", report.Summary.OverSoftLimit),
		fmt.Sprintf("%d", report.Summary.OverHardLimit),
		fmt.Sprintf("%d", report.Summary.ActiveAlerts),
	}
	_ = writer.Write(row)

	return nil
}

// exportHTML 导出 HTML 格式
func (g *ReportGenerator) exportHTML(report *Report, outputPath string) error {
	var sb strings.Builder

	sb.WriteString(`<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <title>存储配额报告</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; }
        h1 { color: #333; }
        table { border-collapse: collapse; width: 100%; margin-top: 20px; }
        th, td { border: 1px solid #ddd; padding: 8px; text-align: left; }
        th { background-color: #4CAF50; color: white; }
        tr:nth-child(even) { background-color: #f2f2f2; }
        .summary { background-color: #e7f3ff; padding: 15px; border-radius: 5px; }
        .alert { color: red; }
        .warning { color: orange; }
    </style>
</head>
<body>
    <h1>存储配额报告</h1>
    <div class="summary">
        <p><strong>报告类型:</strong> ` + string(report.Type) + `</p>
        <p><strong>生成时间:</strong> ` + report.GeneratedAt.Format("2006-01-02 15:04:05") + `</p>
        <p><strong>时间范围:</strong> ` + report.Period.StartTime.Format("2006-01-02") + ` 至 ` + report.Period.EndTime.Format("2006-01-02") + `</p>
    </div>
    
    <h2>摘要</h2>
    <table>
        <tr><th>指标</th><th>值</th></tr>
        <tr><td>总配额数</td><td>` + fmt.Sprintf("%d", report.Summary.TotalQuotas) + `</td></tr>
        <tr><td>总限制</td><td>` + formatBytes(report.Summary.TotalLimitBytes) + `</td></tr>
        <tr><td>总使用</td><td>` + formatBytes(report.Summary.TotalUsedBytes) + `</td></tr>
        <tr><td>平均使用率</td><td>` + fmt.Sprintf("%.2f%%", report.Summary.AverageUsage) + `</td></tr>
        <tr><td>超软限制</td><td class="warning">` + fmt.Sprintf("%d", report.Summary.OverSoftLimit) + `</td></tr>
        <tr><td>超硬限制</td><td class="alert">` + fmt.Sprintf("%d", report.Summary.OverHardLimit) + `</td></tr>
        <tr><td>活跃告警</td><td class="alert">` + fmt.Sprintf("%d", report.Summary.ActiveAlerts) + `</td></tr>
    </table>
</body>
</html>`)

	return os.WriteFile(outputPath, []byte(sb.String()), 0644)
}
