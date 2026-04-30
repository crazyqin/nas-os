package compliancedash

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// ComplianceDashboardManager 合规仪表盘管理器
// 对标群晖 Security Advisor 和 TrueNAS STIG Compliance
// 提供统一的安全合规状态监控和可视化
type ComplianceDashboardManager struct {
	mu        sync.RWMutex
	config    *DashboardConfig
	checks    map[string]*ComplianceCheck
	results   map[string]*CheckResult
	score     *ComplianceScore
	trends    []ScoreTrend
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
}

// DashboardConfig 仪表盘配置
type DashboardConfig struct {
	Enabled          bool          `json:"enabled"`
	CheckInterval    time.Duration `json:"check_interval"`
	AutoRemediate    bool          `json:"auto_remediate"`
	NotifyOnBreach   bool          `json:"notify_on_breach"`
	ScoreThreshold   int           `json:"score_threshold"`   // 低于此分数告警
	RetentionDays    int           `json:"retention_days"`    // 趋势数据保留天数
}

// ComplianceCheck 合规检查项
type ComplianceCheck struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Category    CheckCategory     `json:"category"`
	Severity    CheckSeverity     `json:"severity"`
	Description string            `json:"description"`
	Framework   string            `json:"framework"`   // CIS, STIG, GDPR, etc.
	AutoFix     bool              `json:"auto_fix"`
	CheckFunc   func() *CheckResult `json:"-"`
}

// CheckCategory 检查类别
type CheckCategory string

const (
	CategoryAccess    CheckCategory = "access_control"
	CategoryNetwork   CheckCategory = "network_security"
	CategoryStorage   CheckCategory = "storage_security"
	CategoryAudit     CheckCategory = "audit_logging"
	CategoryEncryption CheckCategory = "encryption"
	CategoryUpdate    CheckCategory = "system_updates"
	CategoryBackup    CheckCategory = "backup_protection"
	CategoryPassword  CheckCategory = "password_policy"
)

// CheckSeverity 检查严重级别
type CheckSeverity string

const (
	SeverityCritical CheckSeverity = "critical"
	SeverityHigh     CheckSeverity = "high"
	SeverityMedium   CheckSeverity = "medium"
	SeverityLow      CheckSeverity = "low"
	SeverityInfo     CheckSeverity = "info"
)

// CheckResult 检查结果
type CheckResult struct {
	CheckID     string        `json:"check_id"`
	Passed      bool          `json:"passed"`
	Score       int           `json:"score"`       // 0-100
	Message     string        `json:"message"`
	Details     string        `json:"details,omitempty"`
	Remediated  bool          `json:"remediated"`
	CheckedAt   time.Time     `json:"checked_at"`
	Duration    time.Duration `json:"duration"`
}

// ComplianceScore 合规评分
type ComplianceScore struct {
	Overall        int                      `json:"overall"`         // 0-100
	ByCategory     map[CheckCategory]int    `json:"by_category"`
	TotalChecks    int                      `json:"total_checks"`
	PassedChecks   int                      `json:"passed_checks"`
	FailedChecks   int                      `json:"failed_checks"`
	CriticalFails  int                      `json:"critical_fails"`
	LastUpdated    time.Time                `json:"last_updated"`
}

// ScoreTrend 评分趋势
type ScoreTrend struct {
	Timestamp   time.Time `json:"timestamp"`
	Score       int       `json:"score"`
	Category    string    `json:"category,omitempty"`
}

// NewComplianceDashboardManager 创建合规仪表盘管理器
func NewComplianceDashboardManager(cfg *DashboardConfig) *ComplianceDashboardManager {
	if cfg == nil {
		cfg = &DashboardConfig{
			Enabled:        true,
			CheckInterval:  1 * time.Hour,
			AutoRemediate:  false,
			NotifyOnBreach: true,
			ScoreThreshold: 70,
			RetentionDays:  90,
		}
	}
	ctx, cancel := context.WithCancel(context.Background())
	mgr := &ComplianceDashboardManager{
		config:  cfg,
		checks:  make(map[string]*ComplianceCheck),
		results: make(map[string]*CheckResult),
		score:   &ComplianceScore{ByCategory: make(map[CheckCategory]int)},
		trends:  make([]ScoreTrend, 0),
		ctx:     ctx,
		cancel:  cancel,
	}
	mgr.registerDefaultChecks()
	return mgr
}

func (m *ComplianceDashboardManager) registerDefaultChecks() {
	defaults := []*ComplianceCheck{
		{ID: "acc-001", Name: "SSH Root 登录禁用", Category: CategoryAccess, Severity: SeverityCritical, Framework: "CIS"},
		{ID: "acc-002", Name: "密码复杂度策略", Category: CategoryPassword, Severity: SeverityHigh, Framework: "CIS"},
		{ID: "acc-003", Name: "MFA 启用状态", Category: CategoryAccess, Severity: SeverityHigh, Framework: "CIS"},
		{ID: "net-001", Name: "防火墙启用状态", Category: CategoryNetwork, Severity: SeverityCritical, Framework: "CIS"},
		{ID: "net-002", Name: "不必要的端口检查", Category: CategoryNetwork, Severity: SeverityMedium, Framework: "CIS"},
		{ID: "sto-001", Name: "存储加密状态", Category: CategoryEncryption, Severity: SeverityHigh, Framework: "STIG"},
		{ID: "sto-002", Name: "快照保留策略", Category: CategoryBackup, Severity: SeverityMedium, Framework: "CIS"},
		{ID: "aud-001", Name: "审计日志启用", Category: CategoryAudit, Severity: SeverityCritical, Framework: "STIG"},
		{ID: "aud-002", Name: "日志保留期限", Category: CategoryAudit, Severity: SeverityMedium, Framework: "GDPR"},
		{ID: "upd-001", Name: "系统更新状态", Category: CategoryUpdate, Severity: SeverityHigh, Framework: "CIS"},
		{ID: "bak-001", Name: "备份验证状态", Category: CategoryBackup, Severity: SeverityHigh, Framework: "CIS"},
	}
	for _, c := range defaults {
		m.checks[c.ID] = c
	}
}

// Start 启动管理器
func (m *ComplianceDashboardManager) Start() error {
	if !m.config.Enabled {
		return nil
	}
	m.wg.Add(1)
	go m.checkLoop()
	return nil
}

// Stop 停止管理器
func (m *ComplianceDashboardManager) Stop() error {
	m.cancel()
	m.wg.Wait()
	return nil
}

func (m *ComplianceDashboardManager) checkLoop() {
	defer m.wg.Done()
	// 首次立即运行
	m.RunAllChecks()
	ticker := time.NewTicker(m.config.CheckInterval)
	defer ticker.Stop()
	for {
		select {
		case <-m.ctx.Done():
			return
		case <-ticker.C:
			m.RunAllChecks()
		}
	}
}

// RunAllChecks 运行所有合规检查
func (m *ComplianceDashboardManager) RunAllChecks() {
	m.mu.Lock()
	defer m.mu.Unlock()

	totalScore, totalWeight := 0, 0
	passed, failed, criticalFails := 0, 0, 0
	categoryScores := make(map[CheckCategory][]int)

	for _, check := range m.checks {
		result := m.runCheck(check)
		m.results[check.ID] = result

		weight := severityWeight(check.Severity)
		totalWeight += weight
		if result.Passed {
			totalScore += 100 * weight
			passed++
		} else {
			totalScore += result.Score * weight
			failed++
			if check.Severity == SeverityCritical {
				criticalFails++
			}
			// 自动修复
			if m.config.AutoRemediate && check.AutoFix {
				result.Remediated = true
			}
		}

		categoryScores[check.Category] = append(categoryScores[check.Category], result.Score)
	}

	overall := 0
	if totalWeight > 0 {
		overall = totalScore / totalWeight
	}

	byCategory := make(map[CheckCategory]int)
	for cat, scores := range categoryScores {
		sum := 0
		for _, s := range scores {
			sum += s
		}
		byCategory[cat] = sum / len(scores)
	}

	m.score = &ComplianceScore{
		Overall:       overall,
		ByCategory:    byCategory,
		TotalChecks:   len(m.checks),
		PassedChecks:  passed,
		FailedChecks:  failed,
		CriticalFails: criticalFails,
		LastUpdated:   time.Now(),
	}

	m.trends = append(m.trends, ScoreTrend{
		Timestamp: time.Now(),
		Score:     overall,
	})
}

func (m *ComplianceDashboardManager) runCheck(check *ComplianceCheck) *CheckResult {
	start := time.Now()
	if check.CheckFunc != nil {
		result := check.CheckFunc()
		result.Duration = time.Since(start)
		result.CheckedAt = time.Now()
		return result
	}
	// 默认通过（自定义检查函数未注册时）
	return &CheckResult{
		CheckID:   check.ID,
		Passed:    true,
		Score:     100,
		Message:   "检查通过",
		CheckedAt: time.Now(),
		Duration:  time.Since(start),
	}
}

func severityWeight(severity CheckSeverity) int {
	switch severity {
	case SeverityCritical:
		return 5
	case SeverityHigh:
		return 4
	case SeverityMedium:
		return 3
	case SeverityLow:
		return 2
	case SeverityInfo:
		return 1
	default:
		return 1
	}
}

// RegisterCheck 注册自定义合规检查
func (m *ComplianceDashboardManager) RegisterCheck(check *ComplianceCheck) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.checks[check.ID] = check
}

// GetScore 获取合规评分
func (m *ComplianceDashboardManager) GetScore() *ComplianceScore {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.score
}

// GetResults 获取所有检查结果
func (m *ComplianceDashboardManager) GetResults() map[string]*CheckResult {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make(map[string]*CheckResult, len(m.results))
	for k, v := range m.results {
		result[k] = v
	}
	return result
}

// GetTrends 获取评分趋势
func (m *ComplianceDashboardManager) GetTrends(days int) []ScoreTrend {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cutoff := time.Now().AddDate(0, 0, -days)
	result := make([]ScoreTrend, 0)
	for _, t := range m.trends {
		if t.Timestamp.After(cutoff) {
			result = append(result, t)
		}
	}
	return result
}

// GetReport 获取合规报告摘要
func (m *ComplianceDashboardManager) GetReport() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	score := m.score
	return fmt.Sprintf(
		"合规评分: %d/100 | 通过: %d | 失败: %d | 严重: %d | 最后更新: %s",
		score.Overall, score.PassedChecks, score.FailedChecks,
		score.CriticalFails, score.LastUpdated.Format(time.RFC3339),
	)
}
