// Package auditreport 提供审计日志分析功能
package auditreport

import (
	"fmt"
	"math"
	"sort"
	"sync"
	"time"
)

// Analyzer 审计日志分析器.
type Analyzer struct {
	events    []*AuditEvent
	threshold AnomalyThreshold
	mu        sync.RWMutex
}

// AnomalyThreshold 异常检测阈值.
type AnomalyThreshold struct {
	// FrequencyMultiple 频率异常倍数（相对于平均值）
	FrequencyMultiple float64 `json:"frequency_multiple"`
	// TimeWindow 时间窗口（小时）
	TimeWindowHours int `json:"time_window_hours"`
	// MaxFailedAttempts 最大失败尝试次数
	MaxFailedAttempts int `json:"max_failed_attempts"`
	// UnusualHoursStart 非工作时间开始（24小时制）
	UnusualHoursStart int `json:"unusual_hours_start"`
	// UnusualHoursEnd 非工作时间结束
	UnusualHoursEnd int `json:"unusual_hours_end"`
}

// DefaultAnomalyThreshold 默认异常阈值.
func DefaultAnomalyThreshold() AnomalyThreshold {
	return AnomalyThreshold{
		FrequencyMultiple: 3.0,
		TimeWindowHours:   24,
		MaxFailedAttempts: 5,
		UnusualHoursStart: 0,
		UnusualHoursEnd:   6,
	}
}

// Anomaly 异常行为.
type Anomaly struct {
	ID          string    `json:"id"`
	Type        string    `json:"type"`
	Severity    Severity  `json:"severity"`
	UserID      string    `json:"user_id"`
	Description string    `json:"description"`
	Events      []*AuditEvent `json:"events"`
	DetectedAt  time.Time `json:"detected_at"`
	RiskScore   float64   `json:"risk_score"`
}

// AccessPattern 访问模式.
type AccessPattern struct {
	UserID        string            `json:"user_id"`
	TotalEvents   int               `json:"total_events"`
	ByAction      map[string]int    `json:"by_action"`
	ByResource    map[string]int    `json:"by_resource"`
	ByHour        map[int]int       `json:"by_hour"`
	ByResult      map[string]int    `json:"by_result"`
	FirstSeen     time.Time         `json:"first_seen"`
	LastSeen      time.Time         `json:"last_seen"`
	AvgEventsPerDay float64         `json:"avg_events_per_day"`
}

// NewAnalyzer 创建分析器.
func NewAnalyzer() *Analyzer {
	return &Analyzer{
		events:    make([]*AuditEvent, 0),
		threshold: DefaultAnomalyThreshold(),
	}
}

// SetThreshold 设置异常阈值.
func (a *Analyzer) SetThreshold(t AnomalyThreshold) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.threshold = t
}

// LoadEvents 加载审计事件.
func (a *Analyzer) LoadEvents(events []*AuditEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = events
}

// AddEvent 添加单个事件.
func (a *Analyzer) AddEvent(event *AuditEvent) {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.events = append(a.events, event)
}

// DetectAnomalies 检测异常行为.
func (a *Analyzer) DetectAnomalies() []Anomaly {
	a.mu.RLock()
	defer a.mu.RUnlock()

	var anomalies []Anomaly

	// 检测各类异常
	anomalies = append(anomalies, a.detectFrequencyAnomalies()...)
	anomalies = append(anomalies, a.detectFailedAttempts()...)
	anomalies = append(anomalies, a.detectUnusualTimeAccess()...)
	anomalies = append(anomalies, a.detectPrivilegeEscalation()...)

	// 按风险分排序
	sort.Slice(anomalies, func(i, j int) bool {
		return anomalies[i].RiskScore > anomalies[j].RiskScore
	})

	return anomalies
}

// AnalyzeAccessPattern 分析用户访问模式.
func (a *Analyzer) AnalyzeAccessPattern(userID string) *AccessPattern {
	a.mu.RLock()
	defer a.mu.RUnlock()

	pattern := &AccessPattern{
		UserID:     userID,
		ByAction:   make(map[string]int),
		ByResource: make(map[string]int),
		ByHour:     make(map[int]int),
		ByResult:   make(map[string]int),
	}

	var userEvents []*AuditEvent
	for _, e := range a.events {
		if e.UserID == userID {
			userEvents = append(userEvents, e)
		}
	}

	if len(userEvents) == 0 {
		return pattern
	}

	pattern.TotalEvents = len(userEvents)
	pattern.FirstSeen = userEvents[0].Timestamp
	pattern.LastSeen = userEvents[0].Timestamp

	for _, e := range userEvents {
		pattern.ByAction[e.Action]++
		pattern.ByResource[e.Resource]++
		pattern.ByHour[e.Timestamp.Hour()]++
		pattern.ByResult[e.Result]++

		if e.Timestamp.Before(pattern.FirstSeen) {
			pattern.FirstSeen = e.Timestamp
		}
		if e.Timestamp.After(pattern.LastSeen) {
			pattern.LastSeen = e.Timestamp
		}
	}

	// 计算平均每日事件数
	days := pattern.LastSeen.Sub(pattern.FirstSeen).Hours() / 24
	if days > 0 {
		pattern.AvgEventsPerDay = float64(pattern.TotalEvents) / days
	} else {
		pattern.AvgEventsPerDay = float64(pattern.TotalEvents)
	}

	return pattern
}

// GetAllPatterns 获取所有用户的访问模式.
func (a *Analyzer) GetAllPatterns() map[string]*AccessPattern {
	a.mu.RLock()
	defer a.mu.RUnlock()

	users := make(map[string]bool)
	for _, e := range a.events {
		users[e.UserID] = true
	}

	patterns := make(map[string]*AccessPattern)
	for userID := range users {
		// 注意: 这里需要释放读锁再获取，简化处理直接调用内部逻辑
		patterns[userID] = a.analyzePatternInternal(userID)
	}

	return patterns
}

// ========== 内部异常检测方法 ==========

// detectFrequencyAnomalies 检测频率异常.
func (a *Analyzer) detectFrequencyAnomalies() []Anomaly {
	var anomalies []Anomaly

	// 按用户和小时分组统计
	userHourlyCount := make(map[string]map[int]int)
	for _, e := range a.events {
		if _, ok := userHourlyCount[e.UserID]; !ok {
			userHourlyCount[e.UserID] = make(map[int]int)
		}
		hour := e.Timestamp.Hour()
		userHourlyCount[e.UserID][hour]++
	}

	// 计算每用户平均频率
	for userID, hourlyCounts := range userHourlyCount {
		total := 0
		for _, count := range hourlyCounts {
			total += count
		}
		avgPerHour := float64(total) / float64(len(hourlyCounts))

		// 检测超过阈值的时段
		for hour, count := range hourlyCounts {
			if float64(count) > avgPerHour*a.threshold.FrequencyMultiple {
				anomaly := Anomaly{
					ID:       fmt.Sprintf("freq-%s-%d-%d", userID, hour, time.Now().Unix()),
					Type:     "frequency_anomaly",
					Severity: SeverityMedium,
					UserID:   userID,
					Description: fmt.Sprintf("用户 %s 在 %d:00 时段访问频率异常 (%d 次, 平均 %.1f 次/小时)",
						userID, hour, count, avgPerHour),
					DetectedAt: time.Now(),
					RiskScore:  calculateFrequencyRisk(count, avgPerHour),
				}
				anomalies = append(anomalies, anomaly)
			}
		}
	}

	return anomalies
}

// detectFailedAttempts 检测失败尝试.
func (a *Analyzer) detectFailedAttempts() []Anomaly {
	var anomalies []Anomaly

	// 统计每个用户的失败尝试
	userFailures := make(map[string][]*AuditEvent)
	for _, e := range a.events {
		if e.Result == "failure" || e.Result == "denied" {
			userFailures[e.UserID] = append(userFailures[e.UserID], e)
		}
	}

	for userID, failures := range userFailures {
		if len(failures) >= a.threshold.MaxFailedAttempts {
			anomaly := Anomaly{
				ID:       fmt.Sprintf("fail-%s-%d", userID, time.Now().Unix()),
				Type:     "excessive_failures",
				Severity: SeverityHigh,
				UserID:   userID,
				Description: fmt.Sprintf("用户 %s 有 %d 次失败尝试 (阈值: %d)",
					userID, len(failures), a.threshold.MaxFailedAttempts),
				Events:     failures,
				DetectedAt: time.Now(),
				RiskScore:  calculateFailureRisk(len(failures), a.threshold.MaxFailedAttempts),
			}
			anomalies = append(anomalies, anomaly)
		}
	}

	return anomalies
}

// detectUnusualTimeAccess 检测非工作时间访问.
func (a *Analyzer) detectUnusualTimeAccess() []Anomaly {
	var anomalies []Anomaly

	// 按用户分组
	userEvents := make(map[string][]*AuditEvent)
	for _, e := range a.events {
		userEvents[e.UserID] = append(userEvents[e.UserID], e)
	}

	for userID, events := range userEvents {
		var unusualEvents []*AuditEvent
		for _, e := range events {
			hour := e.Timestamp.Hour()
			if hour >= a.threshold.UnusualHoursStart && hour < a.threshold.UnusualHoursEnd {
				unusualEvents = append(unusualEvents, e)
			}
		}

		if len(unusualEvents) > 0 {
			anomaly := Anomaly{
				ID:       fmt.Sprintf("time-%s-%d", userID, time.Now().Unix()),
				Type:     "unusual_time_access",
				Severity: SeverityMedium,
				UserID:   userID,
				Description: fmt.Sprintf("用户 %s 在非工作时间 (%d:00-%d:00) 有 %d 次访问",
					userID, a.threshold.UnusualHoursStart, a.threshold.UnusualHoursEnd, len(unusualEvents)),
				Events:     unusualEvents,
				DetectedAt: time.Now(),
				RiskScore:  float64(len(unusualEvents)) * 5.0,
			}
			anomalies = append(anomalies, anomaly)
		}
	}

	return anomalies
}

// detectPrivilegeEscalation 检测权限提升.
func (a *Analyzer) detectPrivilegeEscalation() []Anomaly {
	var anomalies []Anomaly

	// 检测权限变更操作
	privilegeActions := map[string]bool{
		"permission_change": true,
		"role_change":       true,
		"user_create":       true,
		"user_delete":       true,
		"policy_change":     true,
	}

	// 按用户分组
	userPrivEvents := make(map[string][]*AuditEvent)
	for _, e := range a.events {
		if privilegeActions[e.Action] {
			userPrivEvents[e.UserID] = append(userPrivEvents[e.UserID], e)
		}
	}

	for userID, events := range userPrivEvents {
		if len(events) >= 3 {
			anomaly := Anomaly{
				ID:       fmt.Sprintf("priv-%s-%d", userID, time.Now().Unix()),
				Type:     "privilege_escalation",
				Severity: SeverityCritical,
				UserID:   userID,
				Description: fmt.Sprintf("用户 %s 执行了 %d 次权限变更操作，可能存在权限提升风险",
					userID, len(events)),
				Events:     events,
				DetectedAt: time.Now(),
				RiskScore:  float64(len(events)) * 15.0,
			}
			anomalies = append(anomalies, anomaly)
		}
	}

	return anomalies
}

// analyzePatternInternal 内部分析访问模式（不加锁）.
func (a *Analyzer) analyzePatternInternal(userID string) *AccessPattern {
	pattern := &AccessPattern{
		UserID:     userID,
		ByAction:   make(map[string]int),
		ByResource: make(map[string]int),
		ByHour:     make(map[int]int),
		ByResult:   make(map[string]int),
	}

	for _, e := range a.events {
		if e.UserID != userID {
			continue
		}

		pattern.TotalEvents++
		pattern.ByAction[e.Action]++
		pattern.ByResource[e.Resource]++
		pattern.ByHour[e.Timestamp.Hour()]++
		pattern.ByResult[e.Result]++

		if pattern.FirstSeen.IsZero() || e.Timestamp.Before(pattern.FirstSeen) {
			pattern.FirstSeen = e.Timestamp
		}
		if e.Timestamp.After(pattern.LastSeen) {
			pattern.LastSeen = e.Timestamp
		}
	}

	if pattern.TotalEvents > 0 {
		days := pattern.LastSeen.Sub(pattern.FirstSeen).Hours() / 24
		if days > 0 {
			pattern.AvgEventsPerDay = float64(pattern.TotalEvents) / days
		} else {
			pattern.AvgEventsPerDay = float64(pattern.TotalEvents)
		}
	}

	return pattern
}

// ========== 风险计算辅助函数 ==========

func calculateFrequencyRisk(count int, avg float64) float64 {
	ratio := float64(count) / avg
	return math.Min(ratio*10, 100)
}

func calculateFailureRisk(count, threshold int) float64 {
	ratio := float64(count) / float64(threshold)
	return math.Min(ratio*20, 100)
}
