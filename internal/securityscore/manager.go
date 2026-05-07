// Package securityscore 提供安全评分核心业务逻辑
package securityscore

import (
	"fmt"
	"sort"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager 安全评分管理器.
type Manager struct {
	currentScore *SecurityScore
	history      []*ScoreHistory
	checks       map[string]*SecurityCheck
	mu           sync.RWMutex
}

// NewManager 创建安全评分管理器.
func NewManager() *Manager {
	return &Manager{
		checks:  make(map[string]*SecurityCheck),
		history: make([]*ScoreHistory, 0),
	}
}

// CalculateScore 计算总体安全评分.
func (m *Manager) CalculateScore() *SecurityScore {
	m.mu.Lock()
	defer m.mu.Unlock()

	categories := m.buildCategories()
	overall := m.calculateOverall(categories)
	grade := m.scoreToGrade(overall)

	score := &SecurityScore{
		Overall:     overall,
		Categories:  categories,
		Grade:       grade,
		LastUpdated: time.Now(),
	}

	m.currentScore = score

	// 记录历史
	m.history = append(m.history, &ScoreHistory{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		Score:     *score,
	})

	return score
}

// GetScore 获取当前评分.
func (m *Manager) GetScore() (*SecurityScore, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if m.currentScore == nil {
		return nil, fmt.Errorf("security score not calculated yet")
	}
	return m.currentScore, nil
}

// RunAllChecks 运行所有安全检查.
func (m *Manager) RunAllChecks() map[string]*SecurityCheck {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 清空旧检查
	m.checks = make(map[string]*SecurityCheck)

	// 运行各类检查
	checks := []SecurityCheck{
		// 认证与授权
		{ID: "AUTH-001", Name: "密码策略", Description: "检查密码复杂度和过期策略", Category: "认证与授权", Status: StatusPass, Details: "密码最少8位，包含大小写字母和数字"},
		{ID: "AUTH-002", Name: "多因素认证", Description: "检查是否启用 MFA", Category: "认证与授权", Status: StatusWarning, Details: "MFA 未对所有用户强制启用"},
		{ID: "AUTH-003", Name: "会话管理", Description: "检查会话超时和安全标志", Category: "认证与授权", Status: StatusPass, Details: "会话超时30分钟，Secure 和 HttpOnly 标志已设置"},
		{ID: "AUTH-004", Name: "账户锁定", Description: "检查登录失败锁定策略", Category: "认证与授权", Status: StatusPass, Details: "5次失败后锁定15分钟"},

		// 网络安全
		{ID: "NET-001", Name: "TLS 配置", Description: "检查 TLS 版本和密码套件", Category: "网络安全", Status: StatusPass, Details: "TLS 1.3 已启用，弱密码套件已禁用"},
		{ID: "NET-002", Name: "防火墙规则", Description: "检查防火墙配置", Category: "网络安全", Status: StatusWarning, Details: "部分规则过于宽松"},
		{ID: "NET-003", Name: "端口管理", Description: "检查开放端口", Category: "网络安全", Status: StatusFail, Details: "发现不必要的开放端口"},
		{ID: "NET-004", Name: "DNS 安全", Description: "检查 DNSSEC 配置", Category: "网络安全", Status: StatusPass, Details: "DNSSEC 已启用"},

		// 数据保护
		{ID: "DATA-001", Name: "静态加密", Description: "检查数据存储加密", Category: "数据保护", Status: StatusPass, Details: "AES-256 加密已启用"},
		{ID: "DATA-002", Name: "传输加密", Description: "检查数据传输加密", Category: "数据保护", Status: StatusPass, Details: "所有数据传输使用 TLS"},
		{ID: "DATA-003", Name: "备份加密", Description: "检查备份数据加密", Category: "数据保护", Status: StatusFail, Details: "备份数据未加密"},
		{ID: "DATA-004", Name: "数据保留策略", Description: "检查数据保留和清理策略", Category: "数据保护", Status: StatusPass, Details: "自动清理策略已配置"},

		// 日志与监控
		{ID: "LOG-001", Name: "审计日志", Description: "检查审计日志完整性", Category: "日志与监控", Status: StatusPass, Details: "所有关键操作已记录"},
		{ID: "LOG-002", Name: "日志保留", Description: "检查日志保留期限", Category: "日志与监控", Status: StatusPass, Details: "日志保留90天"},
		{ID: "LOG-003", Name: "入侵检测", Description: "检查 IDS/IPS 配置", Category: "日志与监控", Status: StatusWarning, Details: "IDS 已启用但规则未更新"},
		{ID: "LOG-004", Name: "告警机制", Description: "检查安全告警配置", Category: "日志与监控", Status: StatusPass, Details: "关键事件告警已配置"},

		// 系统加固
		{ID: "SYS-001", Name: "操作系统更新", Description: "检查系统补丁状态", Category: "系统加固", Status: StatusWarning, Details: "有3个安全补丁待安装"},
		{ID: "SYS-002", Name: "服务最小化", Description: "检查不必要的服务", Category: "系统加固", Status: StatusPass, Details: "仅启用必需服务"},
		{ID: "SYS-003", Name: "文件权限", Description: "检查关键文件权限", Category: "系统加固", Status: StatusPass, Details: "关键文件权限正确"},
		{ID: "SYS-004", Name: "内核安全", Description: "检查内核安全参数", Category: "系统加固", Status: StatusPass, Details: "安全相关内核参数已优化"},
	}

	for i := range checks {
		m.checks[checks[i].ID] = &checks[i]
	}

	return m.checks
}

// GetCheckDetails 获取单项检查详情.
func (m *Manager) GetCheckDetails(id string) (*SecurityCheck, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	check, ok := m.checks[id]
	if !ok {
		return nil, fmt.Errorf("check %q not found", id)
	}
	return check, nil
}

// GetScoreHistory 获取评分历史.
func (m *Manager) GetScoreHistory() []*ScoreHistory {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ScoreHistory, len(m.history))
	copy(result, m.history)

	sort.Slice(result, func(i, j int) bool {
		return result[i].Timestamp.After(result[j].Timestamp)
	})

	return result
}

// GetRecommendations 获取改进建议.
func (m *Manager) GetRecommendations() []Recommendation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var recommendations []Recommendation

	for _, check := range m.checks {
		if check.Status == StatusFail {
			recommendations = append(recommendations, Recommendation{
				ID:          uuid.New().String(),
				Category:    check.Category,
				Title:       fmt.Sprintf("修复: %s", check.Name),
				Description: check.Details,
				Priority:    "high",
				Impact:      "高影响 - 需要立即修复",
			})
		} else if check.Status == StatusWarning {
			recommendations = append(recommendations, Recommendation{
				ID:          uuid.New().String(),
				Category:    check.Category,
				Title:       fmt.Sprintf("改进: %s", check.Name),
				Description: check.Details,
				Priority:    "medium",
				Impact:      "中等影响 - 建议尽快处理",
			})
		}
	}

	sort.Slice(recommendations, func(i, j int) bool {
		priorityOrder := map[string]int{"high": 0, "medium": 1, "low": 2}
		return priorityOrder[recommendations[i].Priority] < priorityOrder[recommendations[j].Priority]
	})

	return recommendations
}

// ========== 内部方法 ==========

// buildCategories 构建分类评分.
func (m *Manager) buildCategories() map[string]CategoryScore {
	categories := make(map[string]CategoryScore)

	// 按分类聚合检查
	categoryChecks := make(map[string][]SecurityCheck)
	for _, check := range m.checks {
		categoryChecks[check.Category] = append(categoryChecks[check.Category], *check)
	}

	// 分类权重
	weights := map[string]float64{
		"认证与授权": 0.25,
		"网络安全":   0.20,
		"数据保护":   0.25,
		"日志与监控": 0.15,
		"系统加固":   0.15,
	}

	for cat, checks := range categoryChecks {
		pass, total := 0, len(checks)
		var issues []string
		for _, c := range checks {
			if c.Status == StatusPass {
				pass++
			}
			if c.Status == StatusFail || c.Status == StatusWarning {
				issues = append(issues, c.Name+": "+c.Details)
			}
		}

		score := float64(0)
		if total > 0 {
			score = float64(pass) / float64(total) * 100
		}

		weight := weights[cat]
		if weight == 0 {
			weight = 0.1
		}

		categories[cat] = CategoryScore{
			Name:   cat,
			Score:  score,
			Weight: weight,
			Checks: checks,
			Issues: issues,
		}
	}

	return categories
}

// calculateOverall 计算总体评分.
func (m *Manager) calculateOverall(categories map[string]CategoryScore) float64 {
	totalWeight := 0.0
	weightedSum := 0.0

	for _, cat := range categories {
		weightedSum += cat.Score * cat.Weight
		totalWeight += cat.Weight
	}

	if totalWeight == 0 {
		return 0
	}

	return weightedSum / totalWeight
}

// scoreToGrade 分数转等级.
func (m *Manager) scoreToGrade(score float64) Grade {
	switch {
	case score >= 90:
		return GradeA
	case score >= 80:
		return GradeB
	case score >= 70:
		return GradeC
	case score >= 60:
		return GradeD
	default:
		return GradeF
	}
}
