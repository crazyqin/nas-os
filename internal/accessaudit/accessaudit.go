// Package accessaudit 提供零信任访问审计功能
package accessaudit

import (
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// Auditor 访问审计器.
type Auditor struct {
	mu              sync.RWMutex
	records         []*AccessRecord
	anomalies       []*AnomalyDetection
	riskConfig      *RiskScoreConfig
	userHistory     map[string][]*AccessRecord
	ipHistory       map[string][]*AccessRecord
	resourceHistory map[string][]*AccessRecord
}

// NewAuditor 创建审计器.
func NewAuditor(config *RiskScoreConfig) *Auditor {
	if config == nil {
		config = DefaultRiskScoreConfig()
	}
	return &Auditor{
		records:         make([]*AccessRecord, 0),
		anomalies:       make([]*AnomalyDetection, 0),
		riskConfig:      config,
		userHistory:     make(map[string][]*AccessRecord),
		ipHistory:       make(map[string][]*AccessRecord),
		resourceHistory: make(map[string][]*AccessRecord),
	}
}

// RecordAccess 记录访问并计算风险评分.
func (a *Auditor) RecordAccess(record *AccessRecord) {
	a.mu.Lock()
	defer a.mu.Unlock()

	// 设置时间戳
	if record.Timestamp.IsZero() {
		record.Timestamp = time.Now()
	}

	// 计算风险评分
	record.RiskScore = a.calculateRiskScore(record)
	record.RiskLevel = a.getRiskLevel(record.RiskScore)

	// 存储记录
	a.records = append(a.records, record)

	// 更新历史索引
	a.userHistory[record.UserID] = append(a.userHistory[record.UserID], record)
	a.ipHistory[record.SourceIP] = append(a.ipHistory[record.SourceIP], record)
	a.resourceHistory[record.Resource] = append(a.resourceHistory[record.Resource], record)

	// 检测异常
	a.detectAnomalies(record)
}

// calculateRiskScore 计算风险评分.
func (a *Auditor) calculateRiskScore(record *AccessRecord) float64 {
	score := 0.0
	config := a.riskConfig

	// 1. 失败访问
	if record.Status == StatusFailed || record.Status == StatusDenied {
		score += config.FailedAttemptWeight * 100
	}

	// 2. 非常规时间
	hour := record.Timestamp.Hour()
	if hour >= config.UnusualTimeStart && hour < config.UnusualTimeEnd {
		score += config.UnusualTimeWeight * 100
	}

	// 3. 异常IP（首次出现的IP）
	if _, exists := a.ipHistory[record.SourceIP]; !exists {
		score += config.UnusualIPWeight * 100
	}

	// 4. 高频访问
	recentCount := a.countRecentAccess(record.UserID, 1*time.Minute)
	if recentCount > config.HighFreqThreshold {
		freqFactor := math.Min(float64(recentCount)/float64(config.HighFreqThreshold), 3.0)
		score += config.HighFreqWeight * 100 * (freqFactor / 3.0)
	}

	// 5. 敏感资源
	for _, sensitive := range config.SensitiveResources {
		if strings.Contains(record.Resource, sensitive) {
			score += config.SensitiveResourceWeight * 100
			break
		}
	}

	return math.Min(score, 100.0)
}

// getRiskLevel 根据风险评分获取风险等级.
func (a *Auditor) getRiskLevel(score float64) RiskLevel {
	switch {
	case score >= 80:
		return RiskCritical
	case score >= 60:
		return RiskHigh
	case score >= 40:
		return RiskMedium
	default:
		return RiskLow
	}
}

// countRecentAccess 统计最近时间段内的访问次数.
func (a *Auditor) countRecentAccess(userID string, duration time.Duration) int {
	now := time.Now()
	count := 0
	records := a.userHistory[userID]
	for i := len(records) - 1; i >= 0; i-- {
		if now.Sub(records[i].Timestamp) <= duration {
			count++
		} else {
			break
		}
	}
	return count
}

// detectAnomalies 检测异常访问.
func (a *Auditor) detectAnomalies(record *AccessRecord) {
	// 频繁失败检测
	if record.Status == StatusFailed || record.Status == StatusDenied {
		failCount := a.countRecentFailures(record.UserID, 5*time.Minute)
		if failCount >= 5 {
			a.anomalies = append(a.anomalies, &AnomalyDetection{
				ID:          generateID(),
				Timestamp:   time.Now(),
				AnomalyType: "频繁失败访问",
				Description: "用户 " + record.UserID + " 在5分钟内失败" + itoa(failCount) + "次",
				Severity:    RiskHigh,
				UserID:      record.UserID,
				SourceIP:    record.SourceIP,
				RiskScore:   80.0,
			})
		}
	}

	// 异常时间访问检测
	hour := record.Timestamp.Hour()
	if hour >= 0 && hour < 6 {
		a.anomalies = append(a.anomalies, &AnomalyDetection{
			ID:          generateID(),
			Timestamp:   time.Now(),
			AnomalyType: "非常规时间访问",
			Description: "用户 " + record.UserID + " 在凌晨" + itoa(hour) + "点访问",
			Severity:    RiskMedium,
			UserID:      record.UserID,
			SourceIP:    record.SourceIP,
			RiskScore:   60.0,
		})
	}

	// 高频访问检测
	recentCount := a.countRecentAccess(record.UserID, 1*time.Minute)
	if recentCount > a.riskConfig.HighFreqThreshold {
		a.anomalies = append(a.anomalies, &AnomalyDetection{
			ID:          generateID(),
			Timestamp:   time.Now(),
			AnomalyType: "高频访问",
			Description: "用户 " + record.UserID + " 1分钟内访问" + itoa(recentCount) + "次",
			Severity:    RiskHigh,
			UserID:      record.UserID,
			SourceIP:    record.SourceIP,
			RiskScore:   75.0,
		})
	}
}

// countRecentFailures 统计最近失败次数.
func (a *Auditor) countRecentFailures(userID string, duration time.Duration) int {
	now := time.Now()
	count := 0
	records := a.userHistory[userID]
	for i := len(records) - 1; i >= 0; i-- {
		if now.Sub(records[i].Timestamp) <= duration {
			if records[i].Status == StatusFailed || records[i].Status == StatusDenied {
				count++
			}
		} else {
			break
		}
	}
	return count
}

// Query 查询访问记录.
func (a *Auditor) Query(query AccessQuery) []*AccessRecord {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var results []*AccessRecord

	for _, record := range a.records {
		if !a.matchesQuery(record, query) {
			continue
		}
		results = append(results, record)
	}

	// 排序（按时间降序）
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	// 分页
	if query.Offset > 0 && query.Offset < len(results) {
		results = results[query.Offset:]
	}
	if query.Limit > 0 && query.Limit < len(results) {
		results = results[:query.Limit]
	}

	return results
}

// matchesQuery 检查记录是否匹配查询条件.
func (a *Auditor) matchesQuery(record *AccessRecord, query AccessQuery) bool {
	if query.StartTime != nil && record.Timestamp.Before(*query.StartTime) {
		return false
	}
	if query.EndTime != nil && record.Timestamp.After(*query.EndTime) {
		return false
	}
	if query.UserID != "" && record.UserID != query.UserID {
		return false
	}
	if query.SourceIP != "" && record.SourceIP != query.SourceIP {
		return false
	}
	if query.Resource != "" && !strings.Contains(record.Resource, query.Resource) {
		return false
	}
	if query.ResourceType != "" && record.ResourceType != query.ResourceType {
		return false
	}
	if query.Action != "" && record.Action != query.Action {
		return false
	}
	if query.Status != "" && record.Status != query.Status {
		return false
	}
	if query.RiskLevel != "" && record.RiskLevel != query.RiskLevel {
		return false
	}
	if query.MinRiskScore != nil && record.RiskScore < *query.MinRiskScore {
		return false
	}
	return true
}

// GenerateReport 生成审计报告.
func (a *Auditor) GenerateReport(startTime, endTime time.Time) (*AuditReport, error) {
	if startTime.After(endTime) {
		return nil, ErrInvalidTimeRange
	}

	a.mu.RLock()
	defer a.mu.RUnlock()

	report := &AuditReport{
		ID:               generateID(),
		GeneratedAt:      time.Now(),
		StartTime:        startTime,
		EndTime:          endTime,
		RiskDistribution: make(map[RiskLevel]int),
	}

	// 统计数据
	userStats := make(map[string]*UserStats)
	resourceStats := make(map[string]*ResourceStats)
	ipStats := make(map[string]*IPStats)
	hourlyStats := make(map[int]*HourlyActivity)

	for _, record := range a.records {
		if record.Timestamp.Before(startTime) || record.Timestamp.After(endTime) {
			continue
		}

		report.TotalRecords++
		report.AvgRiskScore += record.RiskScore

		// 状态统计
		switch record.Status {
		case StatusSuccess:
			report.SuccessCount++
		case StatusDenied:
			report.DeniedCount++
		case StatusFailed:
			report.FailedCount++
		case StatusError:
			report.ErrorCount++
		}

		// 风险等级分布
		report.RiskDistribution[record.RiskLevel]++
		if record.RiskLevel == RiskHigh || record.RiskLevel == RiskCritical {
			report.HighRiskCount++
		}

		// 用户统计
		if _, exists := userStats[record.UserID]; !exists {
			userStats[record.UserID] = &UserStats{UserID: record.UserID, UserName: record.UserName}
		}
		us := userStats[record.UserID]
		us.AccessCount++
		us.AvgRiskScore += record.RiskScore
		if record.Status == StatusSuccess {
			us.SuccessCount++
		} else if record.Status == StatusDenied {
			us.DeniedCount++
		}

		// 资源统计
		if _, exists := resourceStats[record.Resource]; !exists {
			resourceStats[record.Resource] = &ResourceStats{Resource: record.Resource}
		}
		rs := resourceStats[record.Resource]
		rs.AccessCount++
		rs.AvgRiskScore += record.RiskScore
		if record.Status == StatusSuccess {
			rs.SuccessCount++
		} else if record.Status == StatusDenied {
			rs.DeniedCount++
		}

		// IP统计
		if _, exists := ipStats[record.SourceIP]; !exists {
			ipStats[record.SourceIP] = &IPStats{SourceIP: record.SourceIP}
		}
		is := ipStats[record.SourceIP]
		is.AccessCount++
		is.AvgRiskScore += record.RiskScore
		if record.Status == StatusSuccess {
			is.SuccessCount++
		} else if record.Status == StatusDenied {
			is.DeniedCount++
		}

		// 每小时统计
		hour := record.Timestamp.Hour()
		if _, exists := hourlyStats[hour]; !exists {
			hourlyStats[hour] = &HourlyActivity{Hour: hour}
		}
		hs := hourlyStats[hour]
		hs.AccessCount++
		if record.Status == StatusSuccess {
			hs.SuccessCount++
		} else if record.Status == StatusDenied {
			hs.DeniedCount++
		}
	}

	// 计算平均值
	if report.TotalRecords > 0 {
		report.AvgRiskScore /= float64(report.TotalRecords)
	}

	// 转换并排序Top用户
	for _, us := range userStats {
		if us.AccessCount > 0 {
			us.AvgRiskScore /= float64(us.AccessCount)
		}
		report.TopUsers = append(report.TopUsers, *us)
	}
	sort.Slice(report.TopUsers, func(i, j int) bool {
		return report.TopUsers[i].AccessCount > report.TopUsers[j].AccessCount
	})
	if len(report.TopUsers) > 10 {
		report.TopUsers = report.TopUsers[:10]
	}

	// 转换并排序Top资源
	for _, rs := range resourceStats {
		if rs.AccessCount > 0 {
			rs.AvgRiskScore /= float64(rs.AccessCount)
		}
		report.TopResources = append(report.TopResources, *rs)
	}
	sort.Slice(report.TopResources, func(i, j int) bool {
		return report.TopResources[i].AccessCount > report.TopResources[j].AccessCount
	})
	if len(report.TopResources) > 10 {
		report.TopResources = report.TopResources[:10]
	}

	// 转换并排序Top IP
	for _, is := range ipStats {
		if is.AccessCount > 0 {
			is.AvgRiskScore /= float64(is.AccessCount)
		}
		report.TopSourceIPs = append(report.TopSourceIPs, *is)
	}
	sort.Slice(report.TopSourceIPs, func(i, j int) bool {
		return report.TopSourceIPs[i].AccessCount > report.TopSourceIPs[j].AccessCount
	})
	if len(report.TopSourceIPs) > 10 {
		report.TopSourceIPs = report.TopSourceIPs[:10]
	}

	// 转换每小时统计
	for hour := 0; hour < 24; hour++ {
		if hs, exists := hourlyStats[hour]; exists {
			report.HourlyActivity = append(report.HourlyActivity, *hs)
		} else {
			report.HourlyActivity = append(report.HourlyActivity, HourlyActivity{Hour: hour})
		}
	}

	// 获取时间范围内的异常
	for _, anomaly := range a.anomalies {
		if !anomaly.Timestamp.Before(startTime) && !anomaly.Timestamp.After(endTime) {
			report.Anomalies = append(report.Anomalies, *anomaly)
		}
	}

	return report, nil
}

// GetAnomalies 获取异常列表.
func (a *Auditor) GetAnomalies() []AnomalyDetection {
	a.mu.RLock()
	defer a.mu.RUnlock()

	result := make([]AnomalyDetection, len(a.anomalies))
	for i, anomaly := range a.anomalies {
		result[i] = *anomaly
	}
	return result
}

// ResolveAnomaly 标记异常为已解决.
func (a *Auditor) ResolveAnomaly(id string) bool {
	a.mu.Lock()
	defer a.mu.Unlock()

	for _, anomaly := range a.anomalies {
		if anomaly.ID == id {
			anomaly.IsResolved = true
			return true
		}
	}
	return false
}

// 辅助函数
func generateID() string {
	return time.Now().Format("20060102150405") + "-" + randomString(6)
}

func randomString(n int) string {
	const letters = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, n)
	for i := range b {
		b[i] = letters[time.Now().UnixNano()%int64(len(letters))]
	}
	return string(b)
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	result := ""
	for i > 0 {
		result = string(rune('0'+i%10)) + result
		i /= 10
	}
	return result
}
