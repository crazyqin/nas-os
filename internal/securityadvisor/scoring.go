// Package securityadvisor provides security scoring functionality.
package securityadvisor

import (
	"fmt"
	"time"
)

// CalculateOverallScore 计算总体安全评分
func CalculateOverallScore(checks []SecurityCheck) int {
	if len(checks) == 0 {
		return 0
	}

	weight := DefaultScoreWeight()
	categoryScores := make(map[string][]int)

	// 按类别分组
	for _, check := range checks {
		categoryScores[check.Category] = append(categoryScores[check.Category], check.Score)
	}

	// 计算各类别平均分
	totalWeight := 0.0
	weightedSum := 0.0

	categoryWeights := map[string]float64{
		"password":   weight.Password,
		"port":       weight.Port,
		"permission": weight.Permission,
		"ssl":        weight.SSL,
		"update":     weight.Update,
		"malware":    weight.Malware,
		"firewall":   weight.Firewall,
	}

	for category, scores := range categoryScores {
		if len(scores) == 0 {
			continue
		}

		sum := 0
		for _, score := range scores {
			sum += score
		}
		avg := float64(sum) / float64(len(scores))

		w, ok := categoryWeights[category]
		if !ok {
			w = 0.1 // 默认权重
		}

		weightedSum += avg * w
		totalWeight += w
	}

	if totalWeight == 0 {
		return 0
	}

	return int(weightedSum / totalWeight)
}

// GetSecurityLevel 获取安全等级
func GetSecurityLevel(score int) string {
	switch {
	case score >= 80:
		return "good"
	case score >= 60:
		return "warning"
	default:
		return "critical"
	}
}

// GenerateRecommendations 生成安全建议
func GenerateRecommendations(checks []SecurityCheck) []Recommendation {
	recommendations := make([]Recommendation, 0)
	recMap := make(map[string]bool)

	for _, check := range checks {
		if check.Status == "pass" {
			continue
		}

		// 避免重复建议
		key := fmt.Sprintf("%s-%s", check.Category, check.Status)
		if recMap[key] {
			continue
		}
		recMap[key] = true

		rec := Recommendation{
			ID:          fmt.Sprintf("rec-%s-%d", check.ID, time.Now().Unix()),
			Category:    check.Category,
			Description: check.Message,
		}

		switch check.Category {
		case "password":
			rec.Title = "Improve Password Security"
			rec.Priority = "high"
			rec.Action = "Enforce strong password policies and require regular password changes"

		case "port":
			rec.Title = "Review Open Ports"
			rec.Priority = getPriority(check.Status)
			rec.Action = "Close unnecessary ports and restrict access to essential services only"

		case "permission":
			rec.Title = "Fix File Permissions"
			rec.Priority = getPriority(check.Status)
			rec.Action = check.Remediation

		case "ssl":
			rec.Title = "Update SSL Certificates"
			rec.Priority = getPriority(check.Status)
			rec.Action = "Renew expiring SSL certificates before they expire"

		case "update":
			rec.Title = "Apply System Updates"
			rec.Priority = "medium"
			rec.Action = "Install available security updates immediately"

		case "malware":
			rec.Title = "Address Malware Threats"
			rec.Priority = "critical"
			rec.Action = "Remove detected malware and install antivirus protection"

		case "firewall":
			rec.Title = "Configure Firewall"
			rec.Priority = "high"
			rec.Action = "Configure firewall rules to restrict unauthorized access"

		default:
			rec.Title = "Security Issue Detected"
			rec.Priority = "medium"
			rec.Action = check.Remediation
		}

		recommendations = append(recommendations, rec)
	}

	return recommendations
}

// getPriority 根据状态获取优先级
func getPriority(status string) string {
	switch status {
	case "critical":
		return "high"
	case "warning":
		return "medium"
	default:
		return "low"
	}
}

// FormatSecurityLevel 格式化安全等级显示
func FormatSecurityLevel(level string) string {
	switch level {
	case "good":
		return "Good"
	case "warning":
		return "Warning"
	case "critical":
		return "Critical"
	default:
		return "Unknown"
	}
}

// GetScoreColor 获取评分对应的颜色
func GetScoreColor(score int) string {
	switch {
	case score >= 80:
		return "green"
	case score >= 60:
		return "yellow"
	default:
		return "red"
	}
}

// CalculateCategoryScore 计算单个类别的评分
func CalculateCategoryScore(checks []SecurityCheck, category string) int {
	total := 0
	count := 0

	for _, check := range checks {
		if check.Category == category {
			total += check.Score
			count++
		}
	}

	if count == 0 {
		return 100 // 默认满分
	}

	return total / count
}

// GetCategoryStatus 获取类别状态
func GetCategoryStatus(checks []SecurityCheck, category string) string {
	score := CalculateCategoryScore(checks, category)
	return GetSecurityLevel(score)
}
