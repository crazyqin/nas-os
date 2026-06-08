package storageanalytics

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Reporter 报告生成器.
type Reporter struct {
	costAnalyzer *CostAnalyzer
}

// NewReporter 创建报告生成器.
func NewReporter() *Reporter {
	return &Reporter{
		costAnalyzer: NewCostAnalyzer(nil),
	}
}

// ToJSON 生成JSON格式报告.
func (r *Reporter) ToJSON(report *StorageReport) ([]byte, error) {
	return json.MarshalIndent(report, "", "  ")
}

// ToMarkdown 生成Markdown格式报告.
func (r *Reporter) ToMarkdown(report *StorageReport) string {
	var sb strings.Builder

	sb.WriteString("# 存储分析报告\n\n")
	sb.WriteString(fmt.Sprintf("**扫描路径:** `%s`  \n", report.ScanPath))
	sb.WriteString(fmt.Sprintf("**生成时间:** %s  \n", report.GeneratedAt.Format("2006-01-02 15:04:05")))
	sb.WriteString("\n---\n\n")

	// 存储概览
	sb.WriteString("## 📊 存储概览\n\n")
	sb.WriteString(fmt.Sprintf("- **总大小:** %s  \n", formatBytes(report.Summary.TotalSize)))
	sb.WriteString(fmt.Sprintf("- **文件数量:** %d  \n", report.Summary.TotalFiles))
	sb.WriteString(fmt.Sprintf("- **目录数量:** %d  \n", report.Summary.TotalDirs))
	sb.WriteString(fmt.Sprintf("- **平均文件大小:** %s  \n", formatBytes(report.Summary.AvgFileSize)))
	sb.WriteString(fmt.Sprintf("- **中位数文件大小:** %s  \n", formatBytes(report.Summary.MedianFileSize)))
	if report.Summary.LargestFile != "" {
		sb.WriteString(fmt.Sprintf("- **最大文件:** `%s` (%s)  \n", report.Summary.LargestFile, formatBytes(report.Summary.LargestSize)))
	}
	if report.Summary.OldestFile != "" {
		sb.WriteString(fmt.Sprintf("- **最老文件:** `%s` (%s前)  \n", report.Summary.OldestFile, report.Summary.OldestAge))
	}
	sb.WriteString("\n")

	// 文件类型分布
	sb.WriteString("## 📁 文件类型分布\n\n")
	sb.WriteString("| 类型 | 文件数 | 大小 | 占比 |\n")
	sb.WriteString("|------|--------|------|------|\n")
	for _, s := range report.FileTypeStats {
		sb.WriteString(fmt.Sprintf("| %s | %d | %s | %.1f%% |\n",
			s.Category, s.FileCount, formatBytes(s.TotalSize), s.Percentage))
	}
	sb.WriteString("\n")

	// Top 目录
	if len(report.TopDirectories) > 0 {
		sb.WriteString("## 📂 Top 目录\n\n")
		sb.WriteString("| 排名 | 目录 | 大小 | 文件数 |\n")
		sb.WriteString("|------|------|------|--------|\n")
		for i, d := range report.TopDirectories {
			sb.WriteString(fmt.Sprintf("| %d | `%s` | %s | %d |\n",
				i+1, d.Path, formatBytes(d.TotalSize), d.FileCount))
		}
		sb.WriteString("\n")
	}

	// 文件大小分布
	sb.WriteString("## 📏 文件大小分布\n\n")
	sb.WriteString("| 区间 | 文件数 | 大小 | 占比 |\n")
	sb.WriteString("|------|--------|------|------|\n")
	for _, d := range report.SizeDist {
		sb.WriteString(fmt.Sprintf("| %s | %d | %s | %.1f%% |\n",
			d.Bracket, d.FileCount, formatBytes(d.TotalSize), d.Percentage))
	}
	sb.WriteString("\n")

	// 文件年龄分布
	sb.WriteString("📅 文件年龄分布\n\n")
	sb.WriteString("| 年龄 | 文件数 | 大小 | 占比 |\n")
	sb.WriteString("|------|--------|------|------|\n")
	for _, d := range report.AgeDist {
		sb.WriteString(fmt.Sprintf("| %s | %d | %s | %.1f%% |\n",
			d.Bracket, d.FileCount, formatBytes(d.TotalSize), d.Percentage))
	}
	sb.WriteString("\n")

	// 访问频率
	sb.WriteString("## 🔍 访问频率分析\n\n")
	sb.WriteString("| 频率 | 文件数 | 大小 | 占比 |\n")
	sb.WriteString("|------|--------|------|------|\n")
	for _, d := range report.AccessDist {
		freqName := accessFrequencyName(d.Frequency)
		sb.WriteString(fmt.Sprintf("| %s | %d | %s | %.1f%% |\n",
			freqName, d.FileCount, formatBytes(d.TotalSize), d.Percentage))
	}
	sb.WriteString("\n")

	// 存储健康
	sb.WriteString("## 💚 存储健康指标\n\n")
	sb.WriteString(fmt.Sprintf("- **综合评分:** %.0f/100  \n", report.Health.OverallScore))
	sb.WriteString(fmt.Sprintf("- **碎片化评分:** %.0f/100  \n", report.Health.FragmentationScore))
	sb.WriteString(fmt.Sprintf("- **效率评分:** %.0f/100  \n", report.Health.EfficiencyScore))
	sb.WriteString(fmt.Sprintf("- **数据冗余率:** %.1f%%  \n", report.Health.RedundancyRate*100))
	sb.WriteString(fmt.Sprintf("- **备份覆盖率:** %.1f%%  \n", report.Health.BackupCoverage*100))
	sb.WriteString("\n")

	// 智能洞察
	if len(report.Insights.Insights) > 0 {
		sb.WriteString("## 💡 智能洞察\n\n")
		for _, in := range report.Insights.Insights {
			icon := "💡"
			switch in.Type {
			case "anomaly":
				icon = "⚠️"
			case "waste":
				icon = "🗑️"
			case "optimization":
				icon = "🔧"
			}
			sb.WriteString(fmt.Sprintf("### %s %s\n\n", icon, in.Title))
			sb.WriteString(fmt.Sprintf("- **严重程度:** %s  \n", in.Severity))
			sb.WriteString(fmt.Sprintf("- **详情:** %s  \n", in.Detail))
			if in.Saving > 0 {
				sb.WriteString(fmt.Sprintf("- **可节省:** %s  \n", formatBytes(in.Saving)))
			}
			sb.WriteString(fmt.Sprintf("- **建议:** %s  \n\n", in.Action))
		}
	}

	// 总结
	if report.Insights.TotalPotentialSaving > 0 {
		sb.WriteString("---\n\n")
		sb.WriteString(fmt.Sprintf("💡 **总计可节省空间: %s**  \n", formatBytes(report.Insights.TotalPotentialSaving)))
	}

	return sb.String()
}

// ToMarkdownWithCost 生成包含成本分析的Markdown报告.
func (r *Reporter) ToMarkdownWithCost(report *StorageReport, costReport *StorageCostReport) string {
	// 先生成基础报告
	md := r.ToMarkdown(report)

	if costReport == nil {
		return md
	}

	var sb strings.Builder
	sb.WriteString(md)
	sb.WriteString("\n\n---\n\n")

	// 成本分析
	sb.WriteString("## 💰 存储成本分析\n\n")
	sb.WriteString(fmt.Sprintf("- **月度总成本:** ¥%.2f  \n", costReport.TotalMonthlyCost))
	sb.WriteString(fmt.Sprintf("- **年度总成本:** ¥%.2f  \n", costReport.TotalYearlyCost))
	sb.WriteString(fmt.Sprintf("- **平均每TB成本:** ¥%.2f/月  \n", costReport.CostPerTBAvg))
	sb.WriteString("\n")

	// 层级成本分解
	if len(costReport.TierBreakdown) > 0 {
		sb.WriteString("### 📊 层级成本分解\n\n")
		sb.WriteString("| 层级 | 使用量 | 月成本 | 年成本 | 使用率 |\n")
		sb.WriteString("|------|--------|--------|--------|--------|\n")
		for _, bd := range costReport.TierBreakdown {
			sb.WriteString(fmt.Sprintf("| %s | %.2f TB | ¥%.2f | ¥%.2f | %.1f%% |\n",
				bd.TierName, bd.UsedTB, bd.MonthlyCost, bd.YearlyCost, bd.Utilization*100))
		}
		sb.WriteString("\n")
	}

	// 成本预测
	if costReport.Forecast != nil {
		sb.WriteString("### 📈 成本预测\n\n")
		if costReport.Forecast.GrowthRateTB > 0 {
			sb.WriteString(fmt.Sprintf("- **月增长率:** %.3f TB/月  \n", costReport.Forecast.GrowthRateTB))
		}
		if costReport.Forecast.Breakpoint != nil {
			sb.WriteString(fmt.Sprintf("- **容量瓶颈预计:** %s（%d天后）  \n",
				costReport.Forecast.Breakpoint.EstimatedDate.Format("2006-01-02"),
				costReport.Forecast.Breakpoint.DaysRemaining))
		}
		sb.WriteString("\n")

		// 未来预测
		if len(costReport.Forecast.Predictions) > 0 {
			sb.WriteString("#### 未来成本预测\n\n")
			sb.WriteString("| 时间 | 预测容量 | 预测成本 | 置信度 |\n")
			sb.WriteString("|------|----------|----------|--------|\n")
			for _, pred := range costReport.Forecast.Predictions {
				sb.WriteString(fmt.Sprintf("| %s | %.2f TB | ¥%.2f | %.0f%% |\n",
					pred.PredictedDate.Format("2006-01"),
					pred.PredictedSizeTB,
					pred.PredictedCost,
					pred.Confidence*100))
			}
			sb.WriteString("\n")
		}
	}

	// 优化建议
	if len(costReport.Recommendations) > 0 {
		sb.WriteString("## 🔧 存储优化建议\n\n")
		for _, rec := range costReport.Recommendations {
			sb.WriteString(fmt.Sprintf("### %s [%s]\n\n", rec.Title, rec.Priority))
			sb.WriteString(fmt.Sprintf("- **描述:** %s  \n", rec.Description))
			sb.WriteString(fmt.Sprintf("- **预期影响:** %s  \n", rec.Impact))
			if rec.SavingCost > 0 {
				sb.WriteString(fmt.Sprintf("- **可节省:** ¥%.2f/月  \n", rec.SavingCost))
			}
			if rec.SavingBytes > 0 {
				sb.WriteString(fmt.Sprintf("- **可释放空间:** %s  \n", formatBytes(rec.SavingBytes)))
			}
			sb.WriteString(fmt.Sprintf("- **实施难度:** %s  \n", rec.Effort))
			if len(rec.Steps) > 0 {
				sb.WriteString("- **实施步骤:**  \n")
				for _, step := range rec.Steps {
					sb.WriteString(fmt.Sprintf("  %s  \n", step))
				}
			}
			sb.WriteString("\n")
		}
	}

	// 云存储对比
	if costReport.ComparisonWithCloud != nil {
		sb.WriteString("## ☁️ 云存储成本对比\n\n")
		sb.WriteString(fmt.Sprintf("- **本地存储每TB成本:** ¥%.2f/月  \n", costReport.ComparisonWithCloud.LocalCostPerTB))
		sb.WriteString(fmt.Sprintf("- **最优云方案:** %s  \n", costReport.ComparisonWithCloud.BestOption))
		if costReport.ComparisonWithCloud.SavingsVsCloud > 0 {
			sb.WriteString(fmt.Sprintf("- **本地存储节省:** ¥%.2f/月  \n", costReport.ComparisonWithCloud.SavingsVsCloud))
		} else {
			sb.WriteString(fmt.Sprintf("- **云存储更便宜:** ¥%.2f/月  \n", -costReport.ComparisonWithCloud.SavingsVsCloud))
		}
		sb.WriteString("\n")

		if len(costReport.ComparisonWithCloud.CloudProviders) > 0 {
			sb.WriteString("| 云服务商 | 方案 | 每TB成本 | 月成本 | 延迟 |\n")
			sb.WriteString("|----------|------|----------|--------|------|\n")
			for _, cp := range costReport.ComparisonWithCloud.CloudProviders {
				sb.WriteString(fmt.Sprintf("| %s | %s | ¥%.2f | ¥%.2f | %.0fms |\n",
					cp.Provider, cp.Tier, cp.CostPerTBMonth, cp.MonthlyCost, cp.LatencyMs))
			}
			sb.WriteString("\n")
		}
	}

	return sb.String()
}

// accessFrequencyName 访问频率中文名.
func accessFrequencyName(freq AccessFrequency) string {
	switch freq {
	case AccessFrequent:
		return "频繁"
	case AccessOccasional:
		return "偶尔"
	case AccessRare:
		return "很少"
	case AccessNever:
		return "从未访问"
	default:
		return string(freq)
	}
}
