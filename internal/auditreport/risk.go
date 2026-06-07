// Package auditreport 提供风险评分功能
package auditreport

import (
	"math"
	"sync"
	"time"
)

// RiskLevel 风险等级.
type RiskLevel string

const (
	RiskLevelCritical RiskLevel = "critical"
	RiskLevelHigh     RiskLevel = "high"
	RiskLevelMedium   RiskLevel = "medium"
	RiskLevelLow      RiskLevel = "low"
	RiskLevelSafe     RiskLevel = "safe"
)

// OperationType 操作类型.
type OperationType string

const (
	OpDelete           OperationType = "delete"
	OpModify           OperationType = "modify"
	OpPermissionChange OperationType = "permission_change"
	OpDataExport       OperationType = "data_export"
	OpDataImport       OperationType = "data_import"
	OpRead             OperationType = "read"
	OpLogin            OperationType = "login"
	OpLogout           OperationType = "logout"
	OpConfigChange     OperationType = "config_change"
	OpUserManagement   OperationType = "user_management"
)

// OperationWeight 操作类型权重.
var OperationWeight = map[OperationType]float64{
	OpDelete:           0.8,
	OpModify:           0.8,
	OpPermissionChange: 0.9,
	OpDataExport:       0.7,
	OpDataImport:       0.6,
	OpRead:             0.1,
	OpLogin:            0.2,
	OpLogout:           0.1,
	OpConfigChange:     0.85,
	OpUserManagement:   0.9,
}

// ResourceSensitivity 资源敏感度.
type ResourceSensitivity string

const (
	SensitivityCritical ResourceSensitivity = "critical"
	SensitivityHigh     ResourceSensitivity = "high"
	SensitivityMedium   ResourceSensitivity = "medium"
	SensitivityLow      ResourceSensitivity = "low"
	SensitivityPublic   ResourceSensitivity = "public"
)

// ResourceSensitivityWeight 资源敏感度权重.
var ResourceSensitivityWeight = map[ResourceSensitivity]float64{
	SensitivityCritical: 1.0,
	SensitivityHigh:     0.8,
	SensitivityMedium:   0.5,
	SensitivityLow:      0.3,
	SensitivityPublic:   0.1,
}

// RiskScoreRequest 风险评分请求.
type RiskScoreRequest struct {
	UserID    string     `form:"user_id"`
	Resource  string     `form:"resource"`
	Action    string     `form:"action"`
	StartTime *time.Time `form:"start_time"`
	EndTime   *time.Time `form:"end_time"`
}

// RiskScoreResult 风险评分结果.
type RiskScoreResult struct {
	UserID          string         `json:"user_id"`
	OverallScore    float64        `json:"overall_score"`
	RiskLevel       RiskLevel      `json:"risk_level"`
	Components      RiskComponents `json:"components"`
	TopRisks        []RiskItem     `json:"top_risks"`
	Recommendations []string       `json:"recommendations"`
	CalculatedAt    time.Time      `json:"calculated_at"`
}

// RiskComponents 风险评分组成.
type RiskComponents struct {
	OperationWeight     float64 `json:"operation_weight"`
	FrequencyAnomaly    float64 `json:"frequency_anomaly"`
	TimeAnomaly         float64 `json:"time_anomaly"`
	ResourceSensitivity float64 `json:"resource_sensitivity"`
	FailureRate         float64 `json:"failure_rate"`
	PrivilegeScore      float64 `json:"privilege_score"`
}

// RiskItem 风险项.
type RiskItem struct {
	Type        string  `json:"type"`
	Score       float64 `json:"score"`
	Description string  `json:"description"`
}

// RiskScorer 风险评分器.
type RiskScorer struct {
	analyzer *Analyzer
	events   []*AuditEvent
	mu       sync.RWMutex
}

// NewRiskScorer 创建风险评分器.
func NewRiskScorer(analyzer *Analyzer) *RiskScorer {
	return &RiskScorer{
		analyzer: analyzer,
		events:   make([]*AuditEvent, 0),
	}
}

// LoadEvents 加载审计事件.
func (rs *RiskScorer) LoadEvents(events []*AuditEvent) {
	rs.mu.Lock()
	defer rs.mu.Unlock()
	rs.events = events
}

// CalculateUserRisk 计算用户风险评分.
func (rs *RiskScorer) CalculateUserRisk(userID string) *RiskScoreResult {
	rs.mu.RLock()
	defer rs.mu.RUnlock()

	// 筛选用户事件
	var userEvents []*AuditEvent
	for _, e := range rs.events {
		if e.UserID == userID {
			userEvents = append(userEvents, e)
		}
	}

	if len(userEvents) == 0 {
		return &RiskScoreResult{
			UserID:       userID,
			OverallScore: 0,
			RiskLevel:    RiskLevelSafe,
			CalculatedAt: time.Now(),
		}
	}

	// 计算各维度分数
	components := RiskComponents{
		OperationWeight:     rs.calculateOperationWeight(userEvents),
		FrequencyAnomaly:    rs.calculateFrequencyAnomaly(userEvents),
		TimeAnomaly:         rs.calculateTimeAnomaly(userEvents),
		ResourceSensitivity: rs.calculateResourceSensitivity(userEvents),
		FailureRate:         rs.calculateFailureRate(userEvents),
		PrivilegeScore:      rs.calculatePrivilegeScore(userEvents),
	}

	// 风险分 = 操作类型权重 * 频率异常度 * 时间异常度 * 资源敏感度
	// 加上失败率和权限分的影响
	overallScore := components.OperationWeight *
		(1 + components.FrequencyAnomaly) *
		(1 + components.TimeAnomaly) *
		components.ResourceSensitivity *
		100

	// 限制在 0-100
	overallScore = math.Min(overallScore, 100)
	overallScore = math.Max(overallScore, 0)

	// 确定风险等级
	riskLevel := determineRiskLevel(overallScore)

	// 识别主要风险项
	topRisks := rs.identifyTopRisks(components)

	// 生成建议
	recommendations := generateRecommendations(riskLevel, components)

	return &RiskScoreResult{
		UserID:          userID,
		OverallScore:    overallScore,
		RiskLevel:       riskLevel,
		Components:      components,
		TopRisks:        topRisks,
		Recommendations: recommendations,
		CalculatedAt:    time.Now(),
	}
}

// CalculateAllUserRisk 计算所有用户风险评分.
func (rs *RiskScorer) CalculateAllUserRisk() []*RiskScoreResult {
	rs.mu.RLock()
	users := make(map[string]bool)
	for _, e := range rs.events {
		users[e.UserID] = true
	}
	rs.mu.RUnlock()

	var results []*RiskScoreResult
	for userID := range users {
		results = append(results, rs.CalculateUserRisk(userID))
	}

	// 按风险分排序
	sortRiskResults(results)

	return results
}

// GetHighRiskUsers 获取高风险用户.
func (rs *RiskScorer) GetHighRiskUsers(threshold float64) []*RiskScoreResult {
	allResults := rs.CalculateAllUserRisk()

	var highRisk []*RiskScoreResult
	for _, r := range allResults {
		if r.OverallScore >= threshold {
			highRisk = append(highRisk, r)
		}
	}

	return highRisk
}

// ========== 内部计算方法 ==========

func (rs *RiskScorer) calculateOperationWeight(events []*AuditEvent) float64 {
	if len(events) == 0 {
		return 0
	}

	totalWeight := 0.0
	for _, e := range events {
		opType := mapActionToOperationType(e.Action)
		if weight, ok := OperationWeight[opType]; ok {
			totalWeight += weight
		} else {
			totalWeight += 0.5 // 默认权重
		}
	}

	return totalWeight / float64(len(events))
}

func (rs *RiskScorer) calculateFrequencyAnomaly(events []*AuditEvent) float64 {
	if len(events) < 2 {
		return 0
	}

	// 按小时统计
	hourlyCount := make(map[int]int)
	for _, e := range events {
		hourlyCount[e.Timestamp.Hour()]++
	}

	// 计算平均值和标准差
	total := 0
	for _, count := range hourlyCount {
		total += count
	}
	avg := float64(total) / float64(len(hourlyCount))

	variance := 0.0
	for _, count := range hourlyCount {
		diff := float64(count) - avg
		variance += diff * diff
	}
	variance /= float64(len(hourlyCount))
	stdDev := math.Sqrt(variance)

	// 异常度 = 标准差 / 平均值（变异系数）
	if avg > 0 {
		return stdDev / avg
	}
	return 0
}

func (rs *RiskScorer) calculateTimeAnomaly(events []*AuditEvent) float64 {
	if len(events) == 0 {
		return 0
	}

	// 非工作时间（0-6点）的事件比例
	unusualHourEvents := 0
	for _, e := range events {
		hour := e.Timestamp.Hour()
		if hour >= 0 && hour < 6 {
			unusualHourEvents++
		}
	}

	return float64(unusualHourEvents) / float64(len(events))
}

func (rs *RiskScorer) calculateResourceSensitivity(events []*AuditEvent) float64 {
	if len(events) == 0 {
		return 0
	}

	// 简化处理：基于资源名称推断敏感度
	// 实际应用中应该有资源敏感度配置
	totalSensitivity := 0.0
	for _, e := range events {
		sensitivity := inferResourceSensitivity(e.Resource)
		totalSensitivity += ResourceSensitivityWeight[sensitivity]
	}

	return totalSensitivity / float64(len(events))
}

func (rs *RiskScorer) calculateFailureRate(events []*AuditEvent) float64 {
	if len(events) == 0 {
		return 0
	}

	failures := 0
	for _, e := range events {
		if e.Result == "failure" || e.Result == "denied" {
			failures++
		}
	}

	return float64(failures) / float64(len(events))
}

func (rs *RiskScorer) calculatePrivilegeScore(events []*AuditEvent) float64 {
	privilegeActions := map[string]bool{
		"permission_change": true,
		"role_change":       true,
		"user_create":       true,
		"user_delete":       true,
		"policy_change":     true,
	}

	count := 0
	for _, e := range events {
		if privilegeActions[e.Action] {
			count++
		}
	}

	// 返回权限操作占比
	if len(events) > 0 {
		return float64(count) / float64(len(events))
	}
	return 0
}

func (rs *RiskScorer) identifyTopRisks(components RiskComponents) []RiskItem {
	var risks []RiskItem

	if components.OperationWeight > 0.6 {
		risks = append(risks, RiskItem{
			Type:        "high_risk_operations",
			Score:       components.OperationWeight * 100,
			Description: "执行了大量高风险操作（删除、修改、权限变更）",
		})
	}

	if components.FrequencyAnomaly > 0.5 {
		risks = append(risks, RiskItem{
			Type:        "frequency_anomaly",
			Score:       components.FrequencyAnomaly * 100,
			Description: "操作频率存在异常波动",
		})
	}

	if components.TimeAnomaly > 0.3 {
		risks = append(risks, RiskItem{
			Type:        "unusual_time_access",
			Score:       components.TimeAnomaly * 100,
			Description: "在非工作时间有较多访问活动",
		})
	}

	if components.FailureRate > 0.3 {
		risks = append(risks, RiskItem{
			Type:        "high_failure_rate",
			Score:       components.FailureRate * 100,
			Description: "操作失败率较高，可能存在越权尝试",
		})
	}

	if components.PrivilegeScore > 0.2 {
		risks = append(risks, RiskItem{
			Type:        "privilege_operations",
			Score:       components.PrivilegeScore * 100,
			Description: "执行了较多权限变更操作",
		})
	}

	return risks
}

// ========== 辅助函数 ==========

func mapActionToOperationType(action string) OperationType {
	actionMap := map[string]OperationType{
		"delete":            OpDelete,
		"remove":            OpDelete,
		"update":            OpModify,
		"edit":              OpModify,
		"modify":            OpModify,
		"permission_change": OpPermissionChange,
		"role_change":       OpPermissionChange,
		"export":            OpDataExport,
		"download":          OpDataExport,
		"import":            OpDataImport,
		"upload":            OpDataImport,
		"read":              OpRead,
		"view":              OpRead,
		"list":              OpRead,
		"login":             OpLogin,
		"logout":            OpLogout,
		"config_change":     OpConfigChange,
		"user_create":       OpUserManagement,
		"user_delete":       OpUserManagement,
	}

	if opType, ok := actionMap[action]; ok {
		return opType
	}
	return OpRead // 默认为读操作
}

func inferResourceSensitivity(resource string) ResourceSensitivity {
	// 简化处理：基于资源路径推断敏感度
	// 实际应用中应该有资源配置
	switch {
	case contains(resource, "admin", "config", "key", "secret", "password"):
		return SensitivityCritical
	case contains(resource, "user", "account", "permission", "role"):
		return SensitivityHigh
	case contains(resource, "file", "document", "data"):
		return SensitivityMedium
	case contains(resource, "public", "static", "health"):
		return SensitivityPublic
	default:
		return SensitivityLow
	}
}

func contains(s string, keywords ...string) bool {
	for _, kw := range keywords {
		if len(s) >= len(kw) {
			for i := 0; i <= len(s)-len(kw); i++ {
				if s[i:i+len(kw)] == kw {
					return true
				}
			}
		}
	}
	return false
}

func determineRiskLevel(score float64) RiskLevel {
	switch {
	case score >= 80:
		return RiskLevelCritical
	case score >= 60:
		return RiskLevelHigh
	case score >= 40:
		return RiskLevelMedium
	case score >= 20:
		return RiskLevelLow
	default:
		return RiskLevelSafe
	}
}

func generateRecommendations(level RiskLevel, components RiskComponents) []string {
	var recommendations []string

	switch level {
	case RiskLevelCritical:
		recommendations = append(recommendations, "立即审查用户活动，可能存在安全威胁")
		recommendations = append(recommendations, "考虑临时限制该用户权限")
		recommendations = append(recommendations, "通知安全团队进行人工审查")
	case RiskLevelHigh:
		recommendations = append(recommendations, "加强监控该用户活动")
		recommendations = append(recommendations, "审查最近的操作日志")
	case RiskLevelMedium:
		recommendations = append(recommendations, "定期审查用户活动模式")
		recommendations = append(recommendations, "确认操作符合业务需求")
	}

	if components.FailureRate > 0.3 {
		recommendations = append(recommendations, "检查是否有未授权的访问尝试")
	}

	if components.TimeAnomaly > 0.3 {
		recommendations = append(recommendations, "确认非工作时间的访问是否合理")
	}

	if components.PrivilegeScore > 0.2 {
		recommendations = append(recommendations, "审查权限变更是否经过授权")
	}

	return recommendations
}

func sortRiskResults(results []*RiskScoreResult) {
	// 简单的冒泡排序，按风险分降序
	for i := 0; i < len(results); i++ {
		for j := i + 1; j < len(results); j++ {
			if results[j].OverallScore > results[i].OverallScore {
				results[i], results[j] = results[j], results[i]
			}
		}
	}
}
