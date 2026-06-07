package permaudit

import (
	"fmt"
	"time"

	"go.uber.org/zap"
)

// Auditor 权限审计引擎
type Auditor struct {
	logger *zap.Logger
}

// NewAuditor 创建审计器
func NewAuditor(logger *zap.Logger) *Auditor {
	return &Auditor{logger: logger}
}

// ScanUsers 扫描用户权限并生成完整报告
func (a *Auditor) ScanUsers(users []UserPerm) AuditReport {
	existingGroups := extractGroups(users)
	issues := make([]PermIssue, 0)

	issues = append(issues, a.CheckOverPrivileged(users)...)
	issues = append(issues, a.CheckOrphans(users, existingGroups)...)
	issues = append(issues, a.CheckStaleUsers(users, 30)...)
	issues = append(issues, a.CheckWeakPasswords(users, 8)...)

	now := time.Now()
	activeDays := 30
	activeUsers := 0
	inactiveUsers := 0
	adminCount := 0

	for _, u := range users {
		if u.IsAdmin {
			adminCount++
		}
		if u.LastLogin.IsZero() || now.Sub(u.LastLogin) > time.Duration(activeDays)*24*time.Hour {
			inactiveUsers++
		} else {
			activeUsers++
		}
	}

	report := AuditReport{
		GeneratedAt:     now,
		TotalUsers:      len(users),
		AdminCount:      adminCount,
		ActiveUsers:     activeUsers,
		InactiveUsers:   inactiveUsers,
		Issues:          issues,
		IssueSummary:    sumByType(issues),
		SeveritySummary: sumBySeverity(issues),
		Score:           CalculateScore(issues),
		Recommendations: generateRecommendations(issues, adminCount, len(users)),
	}

	a.logger.Info("权限审计完成",
		zap.Int("total_users", len(users)),
		zap.Int("issues", len(issues)),
		zap.Int("score", report.Score),
	)

	return report
}

// CheckOverPrivileged 检测过度授权（普通用户具有管理员权限标记）
func (a *Auditor) CheckOverPrivileged(users []UserPerm) []PermIssue {
	var issues []PermIssue
	for _, u := range users {
		if u.IsAdmin && contains(u.Groups, "admin") {
			// 真正的管理员，不算过度授权
			continue
		}
		if u.IsAdmin && !contains(u.Groups, "admin") {
			issues = append(issues, PermIssue{
				Type:           "over-privileged",
				Severity:       "high",
				UserID:         u.UserID,
				UserName:       u.UserName,
				Resource:       "admin",
				Description:    fmt.Sprintf("用户 %s 标记为管理员但不在 admin 组中", u.UserName),
				Recommendation: "检查该用户的管理员权限是否正确，如非必要请移除管理员标志",
			})
		}
		// 也检查：普通用户却拥有过多共享目录访问权限
		if !u.IsAdmin && len(u.Shares) > 5 {
			issues = append(issues, PermIssue{
				Type:           "over-privileged",
				Severity:       "medium",
				UserID:         u.UserID,
				UserName:       u.UserName,
				Resource:       "shares",
				Description:    fmt.Sprintf("普通用户 %s 可访问 %d 个共享目录，数量偏多", u.UserName, len(u.Shares)),
				Recommendation: "审查该用户的共享目录权限，遵循最小权限原则",
			})
		}
	}
	return issues
}

// CheckOrphans 检测孤儿权限（用户所属的组不存在于系统中）
func (a *Auditor) CheckOrphans(users []UserPerm, existingGroups []string) []PermIssue {
	var issues []PermIssue
	groupSet := toSet(existingGroups)
	for _, u := range users {
		for _, g := range u.Groups {
			if !groupSet[g] {
				issues = append(issues, PermIssue{
					Type:           "orphan",
					Severity:       "medium",
					UserID:         u.UserID,
					UserName:       u.UserName,
					Resource:       g,
					Description:    fmt.Sprintf("用户 %s 所属的组 %q 不存在", u.UserName, g),
					Recommendation: fmt.Sprintf("删除该用户的 %q 组关联，或重新创建该组", g),
				})
			}
		}
	}
	return issues
}

// CheckStaleUsers 检测不活跃用户
func (a *Auditor) CheckStaleUsers(users []UserPerm, inactiveDays int) []PermIssue {
	var issues []PermIssue
	threshold := time.Duration(inactiveDays) * 24 * time.Hour
	now := time.Now()
	for _, u := range users {
		if u.LastLogin.IsZero() || now.Sub(u.LastLogin) > threshold {
			days := 0
			if !u.LastLogin.IsZero() {
				days = int(now.Sub(u.LastLogin).Hours() / 24)
			}
			issues = append(issues, PermIssue{
				Type:           "stale",
				Severity:       severityForStale(days),
				UserID:         u.UserID,
				UserName:       u.UserName,
				Resource:       "account",
				Description:    fmt.Sprintf("用户 %s 已 %d 天未登录", u.UserName, days),
				Recommendation: "确认该用户是否仍需此账号，如不需要建议禁用或删除",
			})
		}
	}
	return issues
}

// CheckWeakPasswords 检测弱密码
func (a *Auditor) CheckWeakPasswords(users []UserPerm, minLength int) []PermIssue {
	var issues []PermIssue
	for _, u := range users {
		if u.PasswordLen > 0 && u.PasswordLen < minLength {
			issues = append(issues, PermIssue{
				Type:           "weak-password",
				Severity:       "high",
				UserID:         u.UserID,
				UserName:       u.UserName,
				Resource:       "password",
				Description:    fmt.Sprintf("用户 %s 密码长度 %d，低于最低要求 %d", u.UserName, u.PasswordLen, minLength),
				Recommendation: "强制用户修改密码，要求至少 8 位并包含大小写字母和数字",
			})
		}
	}
	return issues
}

// CalculateScore 计算安全评分
// 基础分 100，按问题严重级别扣分：
//
//	critical -15, high -10, medium -5, low -2
func CalculateScore(issues []PermIssue) int {
	score := 100
	for _, iss := range issues {
		switch iss.Severity {
		case "critical":
			score -= 15
		case "high":
			score -= 10
		case "medium":
			score -= 5
		case "low":
			score -= 2
		}
	}
	if score < 0 {
		score = 0
	}
	return score
}

// --- 辅助函数 ---

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

func toSet(ss []string) map[string]bool {
	m := make(map[string]bool, len(ss))
	for _, s := range ss {
		m[s] = true
	}
	return m
}

func extractGroups(users []UserPerm) []string {
	seen := make(map[string]bool)
	var result []string
	for _, u := range users {
		for _, g := range u.Groups {
			if !seen[g] {
				seen[g] = true
				result = append(result, g)
			}
		}
	}
	return result
}

func severityForStale(days int) string {
	if days > 90 {
		return "high"
	}
	if days > 60 {
		return "medium"
	}
	return "low"
}

func sumByType(issues []PermIssue) map[string]int {
	m := make(map[string]int)
	for _, iss := range issues {
		m[iss.Type]++
	}
	return m
}

func sumBySeverity(issues []PermIssue) map[string]int {
	m := make(map[string]int)
	for _, iss := range issues {
		m[iss.Severity]++
	}
	return m
}

func generateRecommendations(issues []PermIssue, adminCount, totalUsers int) []string {
	var recs []string
	issueTypes := sumByType(issues)
	_ = adminCount
	_ = totalUsers

	if issueTypes["over-privileged"] > 0 {
		recs = append(recs, "存在过度授权用户，建议定期审查权限配置，遵循最小权限原则")
	}
	if issueTypes["orphan"] > 0 {
		recs = append(recs, "发现孤儿权限（组已不存在），建议清理无效的组关联")
	}
	if issueTypes["stale"] > 0 {
		recs = append(recs, "存在长期不活跃用户，建议禁用或删除不再使用的账号")
	}
	if issueTypes["weak-password"] > 0 {
		recs = append(recs, "检测到弱密码用户，建议强制密码策略并启用定期密码更换")
	}
	if totalUsers > 0 && adminCount > 0 && float64(adminCount)/float64(totalUsers) > 0.3 {
		recs = append(recs, "管理员占比超过30%，建议减少管理员数量，使用精细化权限组替代")
	}
	if len(issues) == 0 {
		recs = append(recs, "当前权限配置良好，无明显问题")
	}
	return recs
}
