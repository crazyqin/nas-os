package storageanalytics

import (
	"encoding/json"
	"fmt"
	"strings"
)

// Reporter 报告生成器.
type Reporter struct{}

// NewReporter 创建报告生成器.
func NewReporter() *Reporter {
	return &Reporter{}
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
