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
	fmt.Fprintf(&sb, "**扫描路径:** `%s`  \n", report.ScanPath)
	fmt.Fprintf(&sb, "**生成时间:** %s  \n", report.GeneratedAt.Format("2006-01-02 15:04:05"))
	sb.WriteString("\n---\n\n")

	// 存储概览
	sb.WriteString("## 📊 存储概览\n\n")
	fmt.Fprintf(&sb, "- **总大小:** %s  \n", formatBytes(report.Summary.TotalSize))
	fmt.Fprintf(&sb, "- **文件数量:** %d  \n", report.Summary.TotalFiles)
	fmt.Fprintf(&sb, "- **目录数量:** %d  \n", report.Summary.TotalDirs)
	fmt.Fprintf(&sb, "- **平均文件大小:** %s  \n", formatBytes(report.Summary.AvgFileSize))
	fmt.Fprintf(&sb, "- **中位数文件大小:** %s  \n", formatBytes(report.Summary.MedianFileSize))
	if report.Summary.LargestFile != "" {
		fmt.Fprintf(&sb, "- **最大文件:** `%s` (%s)  \n", report.Summary.LargestFile, formatBytes(report.Summary.LargestSize))
	}
	if report.Summary.OldestFile != "" {
		fmt.Fprintf(&sb, "- **最老文件:** `%s` (%s前)  \n", report.Summary.OldestFile, report.Summary.OldestAge)
	}
	sb.WriteString("\n")

	// 文件类型分布
	sb.WriteString("## 📁 文件类型分布\n\n")
	sb.WriteString("| 类型 | 文件数 | 大小 | 占比 |\n")
	sb.WriteString("|------|--------|------|------|\n")
	for _, s := range report.FileTypeStats {
		fmt.Fprintf(&sb, "| %s | %d | %s | %.1f%% |\n",
			s.Category, s.FileCount, formatBytes(s.TotalSize), s.Percentage)
	}
	sb.WriteString("\n")

	// Top 目录
	if len(report.TopDirectories) > 0 {
		sb.WriteString("## 📂 Top 目录\n\n")
		sb.WriteString("| 排名 | 目录 | 大小 | 文件数 |\n")
		sb.WriteString("|------|------|------|--------|\n")
		for i, d := range report.TopDirectories {
			fmt.Fprintf(&sb, "| %d | `%s` | %s | %d |\n",
				i+1, d.Path, formatBytes(d.TotalSize), d.FileCount)
		}
		sb.WriteString("\n")
	}

	// 文件大小分布
	sb.WriteString("## 📏 文件大小分布\n\n")
	sb.WriteString("| 区间 | 文件数 | 大小 | 占比 |\n")
	sb.WriteString("|------|--------|------|------|\n")
	for _, d := range report.SizeDist {
		fmt.Fprintf(&sb, "| %s | %d | %s | %.1f%% |\n",
			d.Bracket, d.FileCount, formatBytes(d.TotalSize), d.Percentage)
	}
	sb.WriteString("\n")

	// 文件年龄分布
	sb.WriteString("📅 文件年龄分布\n\n")
	sb.WriteString("| 年龄 | 文件数 | 大小 | 占比 |\n")
	sb.WriteString("|------|--------|------|------|\n")
	for _, d := range report.AgeDist {
		fmt.Fprintf(&sb, "| %s | %d | %s | %.1f%% |\n",
			d.Bracket, d.FileCount, formatBytes(d.TotalSize), d.Percentage)
	}
	sb.WriteString("\n")

	// 访问频率
	sb.WriteString("## 🔍 访问频率分析\n\n")
	sb.WriteString("| 频率 | 文件数 | 大小 | 占比 |\n")
	sb.WriteString("|------|--------|------|------|\n")
	for _, d := range report.AccessDist {
		freqName := accessFrequencyName(d.Frequency)
		fmt.Fprintf(&sb, "| %s | %d | %s | %.1f%% |\n",
			freqName, d.FileCount, formatBytes(d.TotalSize), d.Percentage)
	}
	sb.WriteString("\n")

	// 存储健康
	sb.WriteString("## 💚 存储健康指标\n\n")
	fmt.Fprintf(&sb, "- **综合评分:** %.0f/100  \n", report.Health.OverallScore)
	fmt.Fprintf(&sb, "- **碎片化评分:** %.0f/100  \n", report.Health.FragmentationScore)
	fmt.Fprintf(&sb, "- **效率评分:** %.0f/100  \n", report.Health.EfficiencyScore)
	fmt.Fprintf(&sb, "- **数据冗余率:** %.1f%%  \n", report.Health.RedundancyRate*100)
	fmt.Fprintf(&sb, "- **备份覆盖率:** %.1f%%  \n", report.Health.BackupCoverage*100)
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
			fmt.Fprintf(&sb, "### %s %s\n\n", icon, in.Title)
			fmt.Fprintf(&sb, "- **严重程度:** %s  \n", in.Severity)
			fmt.Fprintf(&sb, "- **详情:** %s  \n", in.Detail)
			if in.Saving > 0 {
				fmt.Fprintf(&sb, "- **可节省:** %s  \n", formatBytes(in.Saving))
			}
			fmt.Fprintf(&sb, "- **建议:** %s  \n\n", in.Action)
		}
	}

	// 总结
	if report.Insights.TotalPotentialSaving > 0 {
		sb.WriteString("---\n\n")
		fmt.Fprintf(&sb, "💡 **总计可节省空间: %s**  \n", formatBytes(report.Insights.TotalPotentialSaving))
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
	fmt.Fprintf(&sb, "- **月度总成本:** ¥%.2f  \n", costReport.TotalMonthlyCost)
	fmt.Fprintf(&sb, "- **年度总成本:** ¥%.2f  \n", costReport.TotalYearlyCost)
	fmt.Fprintf(&sb, "- **平均每TB成本:** ¥%.2f/月  \n", costReport.CostPerTBAvg)
	sb.WriteString("\n")

	// 层级成本分解
	if len(costReport.TierBreakdown) > 0 {
		sb.WriteString("### 📊 层级成本分解\n\n")
		sb.WriteString("| 层级 | 使用量 | 月成本 | 年成本 | 使用率 |\n")
		sb.WriteString("|------|--------|--------|--------|--------|\n")
		for _, bd := range costReport.TierBreakdown {
			fmt.Fprintf(&sb, "| %s | %.2f TB | ¥%.2f | ¥%.2f | %.1f%% |\n",
				bd.TierName, bd.UsedTB, bd.MonthlyCost, bd.YearlyCost, bd.Utilization*100)
		}
		sb.WriteString("\n")
	}

	// 成本预测
	if costReport.Forecast != nil {
		sb.WriteString("### 📈 成本预测\n\n")
		if costReport.Forecast.GrowthRateTB > 0 {
			fmt.Fprintf(&sb, "- **月增长率:** %.3f TB/月  \n", costReport.Forecast.GrowthRateTB)
		}
		if costReport.Forecast.Breakpoint != nil {
			fmt.Fprintf(&sb, "- **容量瓶颈预计:** %s（%d天后）  \n",
				costReport.Forecast.Breakpoint.EstimatedDate.Format("2006-01-02"),
				costReport.Forecast.Breakpoint.DaysRemaining)
		}
		sb.WriteString("\n")

		// 未来预测
		if len(costReport.Forecast.Predictions) > 0 {
			sb.WriteString("#### 未来成本预测\n\n")
			sb.WriteString("| 时间 | 预测容量 | 预测成本 | 置信度 |\n")
			sb.WriteString("|------|----------|----------|--------|\n")
			for _, pred := range costReport.Forecast.Predictions {
				fmt.Fprintf(&sb, "| %s | %.2f TB | ¥%.2f | %.0f%% |\n",
					pred.PredictedDate.Format("2006-01"),
					pred.PredictedSizeTB,
					pred.PredictedCost,
					pred.Confidence*100)
			}
			sb.WriteString("\n")
		}
	}

	// 优化建议
	if len(costReport.Recommendations) > 0 {
		sb.WriteString("## 🔧 存储优化建议\n\n")
		for _, rec := range costReport.Recommendations {
			fmt.Fprintf(&sb, "### %s [%s]\n\n", rec.Title, rec.Priority)
			fmt.Fprintf(&sb, "- **描述:** %s  \n", rec.Description)
			fmt.Fprintf(&sb, "- **预期影响:** %s  \n", rec.Impact)
			if rec.SavingCost > 0 {
				fmt.Fprintf(&sb, "- **可节省:** ¥%.2f/月  \n", rec.SavingCost)
			}
			if rec.SavingBytes > 0 {
				fmt.Fprintf(&sb, "- **可释放空间:** %s  \n", formatBytes(rec.SavingBytes))
			}
			fmt.Fprintf(&sb, "- **实施难度:** %s  \n", rec.Effort)
			if len(rec.Steps) > 0 {
				sb.WriteString("- **实施步骤:**  \n")
				for _, step := range rec.Steps {
					fmt.Fprintf(&sb, "  %s  \n", step)
				}
			}
			sb.WriteString("\n")
		}
	}

	// 云存储对比
	if costReport.ComparisonWithCloud != nil {
		sb.WriteString("## ☁️ 云存储成本对比\n\n")
		fmt.Fprintf(&sb, "- **本地存储每TB成本:** ¥%.2f/月  \n", costReport.ComparisonWithCloud.LocalCostPerTB)
		fmt.Fprintf(&sb, "- **最优云方案:** %s  \n", costReport.ComparisonWithCloud.BestOption)
		if costReport.ComparisonWithCloud.SavingsVsCloud > 0 {
			fmt.Fprintf(&sb, "- **本地存储节省:** ¥%.2f/月  \n", costReport.ComparisonWithCloud.SavingsVsCloud)
		} else {
			fmt.Fprintf(&sb, "- **云存储更便宜:** ¥%.2f/月  \n", -costReport.ComparisonWithCloud.SavingsVsCloud)
		}
		sb.WriteString("\n")

		if len(costReport.ComparisonWithCloud.CloudProviders) > 0 {
			sb.WriteString("| 云服务商 | 方案 | 每TB成本 | 月成本 | 延迟 |\n")
			sb.WriteString("|----------|------|----------|--------|------|\n")
			for _, cp := range costReport.ComparisonWithCloud.CloudProviders {
				fmt.Fprintf(&sb, "| %s | %s | ¥%.2f | ¥%.2f | %.0fms |\n",
					cp.Provider, cp.Tier, cp.CostPerTBMonth, cp.MonthlyCost, cp.LatencyMs)
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
