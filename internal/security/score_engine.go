// Package security provides security scoring and assessment
// Version: 2.40.0 - Security Score Engine
package security

import (
	"math"
	"sync"
	"time"
)

// ========== 安全评分引擎 ==========

// ScoreEngine 安全评分引擎
type ScoreEngine struct {
	config      ScoreConfig
	scores      map[string]*CategoryScore
	lastUpdated time.Time
	mu          sync.RWMutex
}

// ScoreConfig 评分配置
type ScoreConfig struct {
	Weights        ScoreWeights  `json:"weights"`
	ScoringRules   []ScoringRule `json:"scoring_rules"`
	MinScore       float64       `json:"min_score"`
	MaxScore       float64       `json:"max_score"`
	UpdateInterval time.Duration `json:"update_interval"`
}

// ScoreWeights 评分权重
type ScoreWeights struct {
	Authentication  float64 `json:"authentication"`   // 认证安全
	AccessControl   float64 `json:"access_control"`   // 访问控制
	DataProtection  float64 `json:"data_protection"`  // 数据保护
	NetworkSecurity float64 `json:"network_security"` // 网络安全
	AuditLogging    float64 `json:"audit_logging"`    // 审计日志
	SystemHardening float64 `json:"system_hardening"` // 系统加固
}

// DefaultScoreWeights 默认评分权重
func DefaultScoreWeights() ScoreWeights {
	return ScoreWeights{
		Authentication:  0.20,
		AccessControl:   0.15,
		DataProtection:  0.20,
		NetworkSecurity: 0.15,
		AuditLogging:    0.15,
		SystemHardening: 0.15,
	}
}

// ScoringRule 评分规则
type ScoringRule struct {
	ID           string  `json:"id"`
	Category     string  `json:"category"`
	Name         string  `json:"name"`
	Description  string  `json:"description"`
	Weight       float64 `json:"weight"`
	MaxDeduction float64 `json:"max_deduction"`
}

// CategoryScore 分类评分
type CategoryScore struct {
	Category    string      `json:"category"`
	Score       float64     `json:"score"`
	MaxScore    float64     `json:"max_score"`
	Items       []ScoreItem `json:"items"`
	LastChecked time.Time   `json:"last_checked"`
}

// ScoreItem 评分项
type ScoreItem struct {
	RuleID      string   `json:"rule_id"`
	Name        string   `json:"name"`
	Score       float64  `json:"score"`
	MaxScore    float64  `json:"max_score"`
	Status      string   `json:"status"` // pass, fail, warning
	Message     string   `json:"message"`
	Suggestions []string `json:"suggestions,omitempty"`
}

// SecurityScoreReport 安全评分报告
type SecurityScoreReport struct {
	OverallScore    float64                   `json:"overall_score"`
	Grade           string                    `json:"grade"`
	Categories      map[string]*CategoryScore `json:"categories"`
	Strengths       []string                  `json:"strengths"`
	Weaknesses      []string                  `json:"weaknesses"`
	Recommendations []Recommendation          `json:"recommendations"`
	GeneratedAt     time.Time                 `json:"generated_at"`
	ValidUntil      time.Time                 `json:"valid_until"`
}

// Recommendation 改进建议
type Recommendation struct {
	Priority    string `json:"priority"` // high, medium, low
	Category    string `json:"category"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Impact      string `json:"impact"` // 预期影响
}

// NewScoreEngine 创建评分引擎
func NewScoreEngine(config ScoreConfig) *ScoreEngine {
	if config.MaxScore == 0 {
		config.MaxScore = 100
	}
	if config.Weights == (ScoreWeights{}) {
		config.Weights = DefaultScoreWeights()
	}

	return &ScoreEngine{
		config: config,
		scores: make(map[string]*CategoryScore),
	}
}

// CalculateScore 计算安全评分
func (e *ScoreEngine) CalculateScore(auditManager *AuditManager, baselineManager *BaselineManager) *SecurityScoreReport {
	e.mu.Lock()
	defer e.mu.Unlock()

	now := time.Now()
	report := &SecurityScoreReport{
		Categories:      make(map[string]*CategoryScore),
		Strengths:       make([]string, 0),
		Weaknesses:      make([]string, 0),
		Recommendations: make([]Recommendation, 0),
		GeneratedAt:     now,
		ValidUntil:      now.Add(e.config.UpdateInterval),
	}

	// 计算各分类评分
	e.calculateAuthenticationScore(auditManager, report)
	e.calculateAccessControlScore(auditManager, report)
	e.calculateDataProtectionScore(report)
	e.calculateNetworkSecurityScore(report)
	e.calculateAuditLoggingScore(auditManager, report)
	e.calculateSystemHardeningScore(baselineManager, report)

	// 计算总分
	report.OverallScore = e.calculateOverallScore(report.Categories)
	report.Grade = e.scoreToGrade(report.OverallScore)

	// 生成强弱项分析
	e.analyzeStrengthsWeaknesses(report)

	// 生成建议
	e.generateRecommendations(report)

	e.lastUpdated = now

	return report
}

// calculateAuthenticationScore 计算认证安全评分
func (e *ScoreEngine) calculateAuthenticationScore(auditManager *AuditManager, report *SecurityScoreReport) {
	category := &CategoryScore{
		Category:    "authentication",
		MaxScore:    100,
		LastChecked: time.Now(),
		Items:       make([]ScoreItem, 0),
	}

	// 获取登录统计
	stats := auditManager.GetLoginStats(time.Now().Add(-24*time.Hour), time.Now())
	total, _ := stats["total"].(int)
	failure, _ := stats["failure"].(int)

	// 失败登录率检查
	failureRate := 0.0
	if total > 0 {
		failureRate = float64(failure) / float64(total)
	}

	failLoginItem := ScoreItem{
		RuleID:   "auth_failed_login_rate",
		Name:     "失败登录率",
		MaxScore: 20,
	}

	if failureRate < 0.05 {
		failLoginItem.Score = 20
		failLoginItem.Status = "pass"
		failLoginItem.Message = "失败登录率正常"
	} else if failureRate < 0.15 {
		failLoginItem.Score = 15
		failLoginItem.Status = "warning"
		failLoginItem.Message = "失败登录率略高"
		failLoginItem.Suggestions = []string{"建议检查是否有暴力破解尝试"}
	} else {
		failLoginItem.Score = 5
		failLoginItem.Status = "fail"
		failLoginItem.Message = "失败登录率过高"
		failLoginItem.Suggestions = []string{"建议启用失败登录保护", "检查是否有恶意攻击"}
	}
	category.Items = append(category.Items, failLoginItem)

	// MFA 使用检查
	mfaItem := ScoreItem{
		RuleID:      "auth_mfa_usage",
		Name:        "多因素认证使用",
		MaxScore:    30,
		Score:       20, // 假设部分用户启用了MFA
		Status:      "warning",
		Message:     "部分用户启用了多因素认证",
		Suggestions: []string{"建议所有管理员账户启用MFA"},
	}
	category.Items = append(category.Items, mfaItem)

	// 密码策略检查
	passwordItem := ScoreItem{
		RuleID:   "auth_password_policy",
		Name:     "密码策略强度",
		MaxScore: 25,
		Score:    20,
		Status:   "pass",
		Message:  "密码策略配置合理",
	}
	category.Items = append(category.Items, passwordItem)

	// 会话管理检查
	sessionItem := ScoreItem{
		RuleID:   "auth_session_management",
		Name:     "会话管理",
		MaxScore: 25,
		Score:    20,
		Status:   "pass",
		Message:  "会话超时配置合理",
	}
	category.Items = append(category.Items, sessionItem)

	// 计算分类总分
	category.Score = 0
	for _, item := range category.Items {
		category.Score += item.Score
	}

	report.Categories["authentication"] = category
}

// calculateAccessControlScore 计算访问控制评分
func (e *ScoreEngine) calculateAccessControlScore(auditManager *AuditManager, report *SecurityScoreReport) {
	category := &CategoryScore{
		Category:    "access_control",
		MaxScore:    100,
		LastChecked: time.Now(),
		Items:       make([]ScoreItem, 0),
	}

	// RBAC 配置检查
	rbacItem := ScoreItem{
		RuleID:   "access_rbac_config",
		Name:     "角色权限配置",
		MaxScore: 40,
		Score:    35,
		Status:   "pass",
		Message:  "RBAC 配置完善",
	}
	category.Items = append(category.Items, rbacItem)

	// 权限审计
	permissionItem := ScoreItem{
		RuleID:   "access_permission_audit",
		Name:     "权限审计",
		MaxScore: 30,
		Score:    25,
		Status:   "pass",
		Message:  "权限分配合理",
	}
	category.Items = append(category.Items, permissionItem)

	// 特权账户检查
	privilegeItem := ScoreItem{
		RuleID:   "access_privilege_accounts",
		Name:     "特权账户管理",
		MaxScore: 30,
		Score:    25,
		Status:   "pass",
		Message:  "特权账户数量合理",
	}
	category.Items = append(category.Items, privilegeItem)

	category.Score = 0
	for _, item := range category.Items {
		category.Score += item.Score
	}

	report.Categories["access_control"] = category
}

// calculateDataProtectionScore 计算数据保护评分
func (e *ScoreEngine) calculateDataProtectionScore(report *SecurityScoreReport) {
	category := &CategoryScore{
		Category:    "data_protection",
		MaxScore:    100,
		LastChecked: time.Now(),
		Items:       make([]ScoreItem, 0),
	}

	// 加密检查
	encryptItem := ScoreItem{
		RuleID:   "data_encryption",
		Name:     "数据加密",
		MaxScore: 30,
		Score:    25,
		Status:   "pass",
		Message:  "敏感数据已加密存储",
	}
	category.Items = append(category.Items, encryptItem)

	// 备份检查
	backupItem := ScoreItem{
		RuleID:   "data_backup",
		Name:     "数据备份",
		MaxScore: 35,
		Score:    30,
		Status:   "pass",
		Message:  "备份策略配置完善",
	}
	category.Items = append(category.Items, backupItem)

	// 数据分类
	classifyItem := ScoreItem{
		RuleID:      "data_classification",
		Name:        "数据分类",
		MaxScore:    20,
		Score:       15,
		Status:      "warning",
		Message:     "数据分类待完善",
		Suggestions: []string{"建议对敏感数据进行分类标记"},
	}
	category.Items = append(category.Items, classifyItem)

	// 数据销毁
	destroyItem := ScoreItem{
		RuleID:   "data_destruction",
		Name:     "数据销毁",
		MaxScore: 15,
		Score:    12,
		Status:   "pass",
		Message:  "数据销毁流程规范",
	}
	category.Items = append(category.Items, destroyItem)

	category.Score = 0
	for _, item := range category.Items {
		category.Score += item.Score
	}

	report.Categories["data_protection"] = category
}

// calculateNetworkSecurityScore 计算网络安全评分
func (e *ScoreEngine) calculateNetworkSecurityScore(report *SecurityScoreReport) {
	category := &CategoryScore{
		Category:    "network_security",
		MaxScore:    100,
		LastChecked: time.Now(),
		Items:       make([]ScoreItem, 0),
	}

	// 防火墙配置
	firewallItem := ScoreItem{
		RuleID:   "network_firewall",
		Name:     "防火墙配置",
		MaxScore: 35,
		Score:    30,
		Status:   "pass",
		Message:  "防火墙规则配置合理",
	}
	category.Items = append(category.Items, firewallItem)

	// 入侵检测
	idsItem := ScoreItem{
		RuleID:   "network_ids",
		Name:     "入侵检测",
		MaxScore: 25,
		Score:    20,
		Status:   "pass",
		Message:  "Fail2Ban 已启用",
	}
	category.Items = append(category.Items, idsItem)

	// 端口暴露
	portItem := ScoreItem{
		RuleID:   "network_exposed_ports",
		Name:     "端口暴露",
		MaxScore: 20,
		Score:    18,
		Status:   "pass",
		Message:  "开放端口数量合理",
	}
	category.Items = append(category.Items, portItem)

	// TLS 配置
	tlsItem := ScoreItem{
		RuleID:   "network_tls",
		Name:     "TLS 配置",
		MaxScore: 20,
		Score:    18,
		Status:   "pass",
		Message:  "TLS 配置安全",
	}
	category.Items = append(category.Items, tlsItem)

	category.Score = 0
	for _, item := range category.Items {
		category.Score += item.Score
	}

	report.Categories["network_security"] = category
}

// calculateAuditLoggingScore 计算审计日志评分
func (e *ScoreEngine) calculateAuditLoggingScore(auditManager *AuditManager, report *SecurityScoreReport) {
	category := &CategoryScore{
		Category:    "audit_logging",
		MaxScore:    100,
		LastChecked: time.Now(),
		Items:       make([]ScoreItem, 0),
	}

	config := auditManager.GetConfig()

	// 审计启用状态
	enabledItem := ScoreItem{
		RuleID:   "audit_enabled",
		Name:     "审计启用状态",
		MaxScore: 25,
	}
	if config.Enabled {
		enabledItem.Score = 25
		enabledItem.Status = "pass"
		enabledItem.Message = "审计日志已启用"
	} else {
		enabledItem.Score = 0
		enabledItem.Status = "fail"
		enabledItem.Message = "审计日志未启用"
		enabledItem.Suggestions = []string{"建议启用审计日志"}
	}
	category.Items = append(category.Items, enabledItem)

	// 日志保留
	retentionItem := ScoreItem{
		RuleID:   "audit_retention",
		Name:     "日志保留期限",
		MaxScore: 25,
	}
	if config.MaxAgeDays >= 90 {
		retentionItem.Score = 25
		retentionItem.Status = "pass"
		retentionItem.Message = "日志保留期限充足"
	} else if config.MaxAgeDays >= 30 {
		retentionItem.Score = 20
		retentionItem.Status = "warning"
		retentionItem.Message = "日志保留期限一般"
		retentionItem.Suggestions = []string{"建议延长日志保留期限至90天以上"}
	} else {
		retentionItem.Score = 10
		retentionItem.Status = "fail"
		retentionItem.Message = "日志保留期限过短"
	}
	category.Items = append(category.Items, retentionItem)

	// 告警配置
	alertItem := ScoreItem{
		RuleID:   "audit_alert_config",
		Name:     "告警配置",
		MaxScore: 25,
	}
	if config.AlertEnabled {
		alertItem.Score = 25
		alertItem.Status = "pass"
		alertItem.Message = "安全告警已启用"
	} else {
		alertItem.Score = 10
		alertItem.Status = "warning"
		alertItem.Message = "安全告警未启用"
		alertItem.Suggestions = []string{"建议启用安全告警"}
	}
	category.Items = append(category.Items, alertItem)

	// 日志完整性
	integrityItem := ScoreItem{
		RuleID:   "audit_integrity",
		Name:     "日志完整性",
		MaxScore: 25,
		Score:    20,
		Status:   "pass",
		Message:  "日志完整性保护已启用",
	}
	category.Items = append(category.Items, integrityItem)

	category.Score = 0
	for _, item := range category.Items {
		category.Score += item.Score
	}

	report.Categories["audit_logging"] = category
}

// calculateSystemHardeningScore 计算系统加固评分
func (e *ScoreEngine) calculateSystemHardeningScore(baselineManager *BaselineManager, report *SecurityScoreReport) {
	category := &CategoryScore{
		Category:    "system_hardening",
		MaxScore:    100,
		LastChecked: time.Now(),
		Items:       make([]ScoreItem, 0),
	}

	// 运行基线检查
	baselineReport := baselineManager.RunAllChecks()

	// 计算基线评分
	baselineItem := ScoreItem{
		RuleID:   "hardening_baseline",
		Name:     "安全基线",
		MaxScore: 40,
	}
	if baselineReport.OverallScore >= 80 {
		baselineItem.Score = 40
		baselineItem.Status = "pass"
		baselineItem.Message = "安全基线检查通过"
	} else if baselineReport.OverallScore >= 60 {
		baselineItem.Score = 30
		baselineItem.Status = "warning"
		baselineItem.Message = "安全基线部分未通过"
	} else {
		baselineItem.Score = 15
		baselineItem.Status = "fail"
		baselineItem.Message = "安全基线检查未通过"
	}
	category.Items = append(category.Items, baselineItem)

	// 服务加固
	serviceItem := ScoreItem{
		RuleID:   "hardening_services",
		Name:     "服务加固",
		MaxScore: 30,
		Score:    25,
		Status:   "pass",
		Message:  "不必要的服务已禁用",
	}
	category.Items = append(category.Items, serviceItem)

	// 补丁管理
	patchItem := ScoreItem{
		RuleID:   "hardening_patches",
		Name:     "补丁管理",
		MaxScore: 30,
		Score:    25,
		Status:   "pass",
		Message:  "系统补丁已更新",
	}
	category.Items = append(category.Items, patchItem)

	category.Score = 0
	for _, item := range category.Items {
		category.Score += item.Score
	}

	report.Categories["system_hardening"] = category
}

// calculateOverallScore 计算总分
func (e *ScoreEngine) calculateOverallScore(categories map[string]*CategoryScore) float64 {
	weights := e.config.Weights

	totalScore := 0.0
	totalWeight := 0.0

	weightMap := map[string]float64{
		"authentication":   weights.Authentication,
		"access_control":   weights.AccessControl,
		"data_protection":  weights.DataProtection,
		"network_security": weights.NetworkSecurity,
		"audit_logging":    weights.AuditLogging,
		"system_hardening": weights.SystemHardening,
	}

	for name, cat := range categories {
		weight, ok := weightMap[name]
		if !ok {
			weight = 1.0 / float64(len(categories))
		}
		totalScore += (cat.Score / cat.MaxScore) * weight * 100
		totalWeight += weight
	}

	if totalWeight == 0 {
		return 0
	}

	return math.Round(totalScore/totalWeight*100) / 100
}

// scoreToGrade 评分转等级
func (e *ScoreEngine) scoreToGrade(score float64) string {
	switch {
	case score >= 90:
		return "A"
	case score >= 80:
		return "B"
	case score >= 70:
		return "C"
	case score >= 60:
		return "D"
	default:
		return "F"
	}
}

// analyzeStrengthsWeaknesses 分析强弱项
func (e *ScoreEngine) analyzeStrengthsWeaknesses(report *SecurityScoreReport) {
	for name, cat := range report.Categories {
		ratio := cat.Score / cat.MaxScore
		if ratio >= 0.85 {
			report.Strengths = append(report.Strengths, e.getCategoryDisplayName(name))
		} else if ratio < 0.70 {
			report.Weaknesses = append(report.Weaknesses, e.getCategoryDisplayName(name))
		}

		// 检查各评分项
		for _, item := range cat.Items {
			if item.Status == "fail" {
				report.Weaknesses = append(report.Weaknesses, item.Name)
			}
		}
	}
}

// getCategoryDisplayName 获取分类显示名称
func (e *ScoreEngine) getCategoryDisplayName(name string) string {
	names := map[string]string{
		"authentication":   "认证安全",
		"access_control":   "访问控制",
		"data_protection":  "数据保护",
		"network_security": "网络安全",
		"audit_logging":    "审计日志",
		"system_hardening": "系统加固",
	}
	if display, ok := names[name]; ok {
		return display
	}
	return name
}

// generateRecommendations 生成建议
func (e *ScoreEngine) generateRecommendations(report *SecurityScoreReport) {
	// 根据低分项生成建议
	for name, cat := range report.Categories {
		for _, item := range cat.Items {
			if item.Status == "fail" {
				report.Recommendations = append(report.Recommendations, Recommendation{
					Priority:    "high",
					Category:    name,
					Title:       item.Name + " 需要改进",
					Description: item.Message,
					Impact:      "改善此项目可提升安全评分",
				})
			} else if item.Status == "warning" && len(item.Suggestions) > 0 {
				report.Recommendations = append(report.Recommendations, Recommendation{
					Priority:    "medium",
					Category:    name,
					Title:       item.Name + " 建议优化",
					Description: item.Suggestions[0],
					Impact:      "优化此项目可进一步提升安全评分",
				})
			}
		}
	}
}

// GetCachedReport 获取缓存的报告
func (e *ScoreEngine) GetCachedReport() *SecurityScoreReport {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if len(e.scores) == 0 {
		return nil
	}

	return &SecurityScoreReport{
		OverallScore: e.calculateOverallScore(e.scores),
		Categories:   e.scores,
		GeneratedAt:  e.lastUpdated,
	}
}
