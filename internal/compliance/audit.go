// Package compliance 提供安全审计日志功能
//
// 本模块实现了操作审计日志（JSONL格式）、用户行为追踪、异常访问检测和审计报告生成。
// 遵循 TrueNAS 25.10 的安全审计能力设计。
//
// 刑部（法务合规）注: 本模块已于 2026-06-24 完成法务合规检查。
// 检查报告详见 /xingbu/NAS-OS_COMPLIANCE_REPORT.md
package compliance

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// AuditEvent 审计事件
type AuditEvent struct {
	ID         string                 `json:"id"`
	Timestamp  time.Time              `json:"timestamp"`
	EventType  AuditEventType         `json:"event_type"`
	Severity   AuditSeverity          `json:"severity"`
	Actor      AuditActor             `json:"actor"`
	Resource   AuditResource          `json:"resource"`
	Action     string                 `json:"action"`
	Result     AuditResult            `json:"result"`
	Details    map[string]interface{} `json:"details,omitempty"`
	IPAddress  string                 `json:"ip_address,omitempty"`
	UserAgent  string                 `json:"user_agent,omitempty"`
	SessionID  string                 `json:"session_id,omitempty"`
	RequestID  string                 `json:"request_id,omitempty"`
}

// AuditEventType 审计事件类型
type AuditEventType string

const (
	EventLogin            AuditEventType = "login"
	EventLogout           AuditEventType = "logout"
	EventAccess           AuditEventType = "access"
	EventModify           AuditEventType = "modify"
	EventDelete           AuditEventType = "delete"
	EventCreate           AuditEventType = "create"
	EventConfig           AuditEventType = "config"
	EventSecurity         AuditEventType = "security"
	EventSystem           AuditEventType = "system"
	EventNetwork          AuditEventType = "network"
	EventPermission       AuditEventType = "permission"
	EventEncryption       AuditEventType = "encryption"
	EventBackup           AuditEventType = "backup"
	EventExport           AuditEventType = "export"
	EventImport           AuditEventType = "import"
	EventCompliance       AuditEventType = "compliance"
)

// AuditSeverity 审计事件严重级别
type AuditSeverity string

const (
	SeverityCritical AuditSeverity = "critical"
	SeverityHigh     AuditSeverity = "high"
	SeverityMedium   AuditSeverity = "medium"
	SeverityLow      AuditSeverity = "low"
	SeverityInfo     AuditSeverity = "info"
)

// AuditActor 审计事件执行者
type AuditActor struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Type     string `json:"type"` // user, system, service, api-key
	Role     string `json:"role,omitempty"`
	TenantID string `json:"tenant_id,omitempty"`
}

// AuditResource 审计事件资源
type AuditResource struct {
	ID       string `json:"id"`
	Type     string `json:"type"` // file, directory, share, user, config, volume, etc.
	Name     string `json:"name,omitempty"`
	Path     string `json:"path,omitempty"`
	ParentID string `json:"parent_id,omitempty"`
}

// AuditResult 审计事件结果
type AuditResult struct {
	Status  string `json:"status"` // success, failure, denied, error
	Code    int    `json:"code,omitempty"`
	Message string `json:"message,omitempty"`
}

// AuditQuery 审计日志查询参数
type AuditQuery struct {
	StartTime    *time.Time       `json:"start_time,omitempty"`
	EndTime      *time.Time       `json:"end_time,omitempty"`
	EventTypes   []AuditEventType `json:"event_types,omitempty"`
	Severities   []AuditSeverity  `json:"severities,omitempty"`
	ActorID      string           `json:"actor_id,omitempty"`
	ResourceType string           `json:"resource_type,omitempty"`
	ResourceID   string           `json:"resource_id,omitempty"`
	Status       string           `json:"status,omitempty"`
	IPAddress    string           `json:"ip_address,omitempty"`
	Limit        int              `json:"limit,omitempty"`
	Offset       int              `json:"offset,omitempty"`
}

// AuditQueryResult 审计日志查询结果
type AuditQueryResult struct {
	Events     []AuditEvent `json:"events"`
	Total      int          `json:"total"`
	Limit      int          `json:"limit"`
	Offset     int          `json:"offset"`
	HasMore    bool         `json:"has_more"`
	QueriedAt  time.Time    `json:"queried_at"`
}

// AnomalyDetectionResult 异常检测结果
type AnomalyDetectionResult struct {
	ID          string        `json:"id"`
	DetectedAt  time.Time     `json:"detected_at"`
	AnomalyType string        `json:"anomaly_type"`
	Severity    AuditSeverity `json:"severity"`
	Description string        `json:"description"`
	Events      []AuditEvent  `json:"events"`
	RiskScore   float64       `json:"risk_score"`
	Suggestions []string      `json:"suggestions"`
}

// UserBehaviorProfile 用户行为画像
type UserBehaviorProfile struct {
	UserID           string         `json:"user_id"`
	UserName         string         `json:"user_name"`
	TotalEvents      int            `json:"total_events"`
	EventCounts      map[string]int `json:"event_counts"`
	LastActivity     time.Time      `json:"last_activity"`
	TypicalHours     []int          `json:"typical_hours"`
	TypicalResources []string       `json:"typical_resources"`
	IPAddressHistory []string       `json:"ip_address_history"`
	RiskScore        float64        `json:"risk_score"`
	Alerts           []string       `json:"alerts,omitempty"`
}

// AuditReport 审计报告
type AuditReport struct {
	ID              string               `json:"id"`
	Title           string               `json:"title"`
	Period          ReportPeriod         `json:"period"`
	Summary         AuditReportSummary   `json:"summary"`
	TopEvents       []EventTypeCount     `json:"top_events"`
	TopActors       []ActorCount         `json:"top_actors"`
	Anomalies       []AnomalyDetectionResult `json:"anomalies"`
	SecurityEvents  []AuditEvent         `json:"security_events"`
	Recommendations []string             `json:"recommendations"`
	GeneratedAt     time.Time            `json:"generated_at"`
}

// AuditReportSummary 审计报告摘要
type AuditReportSummary struct {
	TotalEvents     int            `json:"total_events"`
	EventsByType    map[string]int `json:"events_by_type"`
	EventsBySeverity map[string]int `json:"events_by_severity"`
	SuccessRate     float64        `json:"success_rate"`
	FailureRate     float64        `json:"failure_rate"`
	UniqueActors    int            `json:"unique_actors"`
	UniqueResources int            `json:"unique_resources"`
}

// EventTypeCount 事件类型计数
type EventTypeCount struct {
	Type  AuditEventType `json:"type"`
	Count int            `json:"count"`
}

// ActorCount 执行者计数
type ActorCount struct {
	ActorID   string `json:"actor_id"`
	ActorName string `json:"actor_name"`
	Count     int    `json:"count"`
}

// AuditLogger 审计日志记录器
type AuditLogger struct {
	mu            sync.RWMutex
	logDir        string
	currentFile   *os.File
	writer        *bufio.Writer
	events        []AuditEvent
	maxEvents     int
	retentionDays int
	profiles      map[string]*UserBehaviorProfile
}

// NewAuditLogger 创建审计日志记录器
func NewAuditLogger(logDir string, maxEvents int, retentionDays int) (*AuditLogger, error) {
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, fmt.Errorf("创建审计日志目录失败: %w", err)
	}

	logger := &AuditLogger{
		logDir:        logDir,
		maxEvents:     maxEvents,
		retentionDays: retentionDays,
		events:        make([]AuditEvent, 0),
		profiles:      make(map[string]*UserBehaviorProfile),
	}

	// 打开当前日志文件
	if err := logger.rotateLogFile(); err != nil {
		return nil, err
	}

	return logger, nil
}

// rotateLogFile 轮转日志文件
func (l *AuditLogger) rotateLogFile() error {
	if l.currentFile != nil {
		l.writer.Flush()
		l.currentFile.Close()
	}

	filename := filepath.Join(l.logDir, fmt.Sprintf("audit_%s.jsonl", time.Now().Format("2006-01-02")))
	file, err := os.OpenFile(filename, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0640)
	if err != nil {
		return fmt.Errorf("打开审计日志文件失败: %w", err)
	}

	l.currentFile = file
	l.writer = bufio.NewWriter(file)

	return nil
}

// LogEvent 记录审计事件
func (l *AuditLogger) LogEvent(event *AuditEvent) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 设置默认值
	if event.ID == "" {
		event.ID = fmt.Sprintf("audit-%d", time.Now().UnixNano())
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	// 序列化为 JSON
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("序列化审计事件失败: %w", err)
	}

	// 写入 JSONL 文件
	if _, err := l.writer.Write(data); err != nil {
		return fmt.Errorf("写入审计日志失败: %w", err)
	}
	if _, err := l.writer.WriteString("\n"); err != nil {
		return fmt.Errorf("写入换行符失败: %w", err)
	}

	// 刷新缓冲区
	if err := l.writer.Flush(); err != nil {
		return fmt.Errorf("刷新缓冲区失败: %w", err)
	}

	// 添加到内存
	l.events = append(l.events, *event)

	// 更新用户行为画像
	l.updateUserProfile(event)

	// 检查是否需要轮转
	if len(l.events) >= l.maxEvents {
		if err := l.rotateLogFile(); err != nil {
			return err
		}
		l.events = l.events[:0]
	}

	return nil
}

// updateUserProfile 更新用户行为画像
func (l *AuditLogger) updateUserProfile(event *AuditEvent) {
	actorID := event.Actor.ID
	if actorID == "" {
		return
	}

	profile, exists := l.profiles[actorID]
	if !exists {
		profile = &UserBehaviorProfile{
			UserID:       actorID,
			UserName:     event.Actor.Name,
			EventCounts:  make(map[string]int),
			IPAddressHistory: make([]string, 0),
		}
		l.profiles[actorID] = profile
	}

	profile.TotalEvents++
	profile.EventCounts[string(event.EventType)]++
	profile.LastActivity = event.Timestamp

	// 记录 IP 地址历史
	if event.IPAddress != "" {
		found := false
		for _, ip := range profile.IPAddressHistory {
			if ip == event.IPAddress {
				found = true
				break
			}
		}
		if !found {
			profile.IPAddressHistory = append(profile.IPAddressHistory, event.IPAddress)
		}
	}

	// 记录典型活动时间
	hour := event.Timestamp.Hour()
	if len(profile.TypicalHours) < 24 {
		profile.TypicalHours = append(profile.TypicalHours, hour)
	}

	// 记录典型资源
	if event.Resource.Type != "" {
		found := false
		for _, r := range profile.TypicalResources {
			if r == event.Resource.Type {
				found = true
				break
			}
		}
		if !found && len(profile.TypicalResources) < 10 {
			profile.TypicalResources = append(profile.TypicalResources, event.Resource.Type)
		}
	}
}

// QueryEvents 查询审计事件
func (l *AuditLogger) QueryEvents(query *AuditQuery) (*AuditQueryResult, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var filtered []AuditEvent

	for _, event := range l.events {
		if l.matchesQuery(&event, query) {
			filtered = append(filtered, event)
		}
	}

	// 排序（按时间倒序）
	sort.Slice(filtered, func(i, j int) bool {
		return filtered[i].Timestamp.After(filtered[j].Timestamp)
	})

	// 分页
	total := len(filtered)
	offset := query.Offset
	if offset > total {
		offset = total
	}

	limit := query.Limit
	if limit <= 0 {
		limit = 50
	}

	end := offset + limit
	if end > total {
		end = total
	}

	result := &AuditQueryResult{
		Events:    filtered[offset:end],
		Total:     total,
		Limit:     limit,
		Offset:    offset,
		HasMore:   end < total,
		QueriedAt: time.Now(),
	}

	return result, nil
}

// matchesQuery 检查事件是否匹配查询条件
func (l *AuditLogger) matchesQuery(event *AuditEvent, query *AuditQuery) bool {
	// 时间范围
	if query.StartTime != nil && event.Timestamp.Before(*query.StartTime) {
		return false
	}
	if query.EndTime != nil && event.Timestamp.After(*query.EndTime) {
		return false
	}

	// 事件类型
	if len(query.EventTypes) > 0 {
		found := false
		for _, t := range query.EventTypes {
			if event.EventType == t {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 严重级别
	if len(query.Severities) > 0 {
		found := false
		for _, s := range query.Severities {
			if event.Severity == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 执行者
	if query.ActorID != "" && event.Actor.ID != query.ActorID {
		return false
	}

	// 资源类型
	if query.ResourceType != "" && event.Resource.Type != query.ResourceType {
		return false
	}

	// 资源 ID
	if query.ResourceID != "" && event.Resource.ID != query.ResourceID {
		return false
	}

	// 状态
	if query.Status != "" && event.Result.Status != query.Status {
		return false
	}

	// IP 地址
	if query.IPAddress != "" && event.IPAddress != query.IPAddress {
		return false
	}

	return true
}

// DetectAnomalies 检测异常行为
func (l *AuditLogger) DetectAnomalies() []AnomalyDetectionResult {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var anomalies []AnomalyDetectionResult

	// 检测异常模式
	anomalies = append(anomalies, l.detectBruteForce()...)
	anomalies = append(anomalies, l.detectUnusualAccess()...)
	anomalies = append(anomalies, l.detectPrivilegeEscalation()...)
	anomalies = append(anomalies, l.detectDataExfiltration()...)

	return anomalies
}

// detectBruteForce 检测暴力破解
func (l *AuditLogger) detectBruteForce() []AnomalyDetectionResult {
	var anomalies []AnomalyDetectionResult

	// 按 IP 地址统计登录失败
	ipFailures := make(map[string][]AuditEvent)
	for _, event := range l.events {
		if event.EventType == EventLogin && event.Result.Status == "failure" {
			ipFailures[event.IPAddress] = append(ipFailures[event.IPAddress], event)
		}
	}

	for ip, events := range ipFailures {
		if len(events) >= 5 { // 5次失败登录
			anomalies = append(anomalies, AnomalyDetectionResult{
				ID:          fmt.Sprintf("anomaly-brute-%d", time.Now().UnixNano()),
				DetectedAt:  time.Now(),
				AnomalyType: "brute_force",
				Severity:    SeverityHigh,
				Description: fmt.Sprintf("检测到来自 IP %s 的 %d 次登录失败，疑似暴力破解", ip, len(events)),
				Events:      events,
				RiskScore:   0.8,
				Suggestions: []string{
					"临时封禁该 IP 地址",
					"启用账户锁定策略",
					"实施多因素认证",
				},
			})
		}
	}

	return anomalies
}

// detectUnusualAccess 检测异常访问
func (l *AuditLogger) detectUnusualAccess() []AnomalyDetectionResult {
	var anomalies []AnomalyDetectionResult

	// 检测非工作时间的访问
	for _, event := range l.events {
		hour := event.Timestamp.Hour()
		if hour >= 0 && hour < 6 { // 凌晨 0-6 点
			if event.EventType == EventAccess || event.EventType == EventModify {
				anomalies = append(anomalies, AnomalyDetectionResult{
					ID:          fmt.Sprintf("anomaly-unusual-%d", time.Now().UnixNano()),
					DetectedAt:  time.Now(),
					AnomalyType: "unusual_access_time",
					Severity:    SeverityMedium,
					Description: fmt.Sprintf("检测到非工作时间（%d:00）的 %s 操作", hour, event.EventType),
					Events:      []AuditEvent{event},
					RiskScore:   0.5,
					Suggestions: []string{
						"确认是否为授权操作",
						"检查账户是否被盗用",
					},
				})
			}
		}
	}

	return anomalies
}

// detectPrivilegeEscalation 检测权限提升
func (l *AuditLogger) detectPrivilegeEscalation() []AnomalyDetectionResult {
	var anomalies []AnomalyDetectionResult

	// 检测权限变更
	for _, event := range l.events {
		if event.EventType == EventPermission && event.Action == "grant" {
			if role, ok := event.Details["new_role"].(string); ok {
				if role == "admin" || role == "root" {
					anomalies = append(anomalies, AnomalyDetectionResult{
						ID:          fmt.Sprintf("anomaly-priv-%d", time.Now().UnixNano()),
						DetectedAt:  time.Now(),
						AnomalyType: "privilege_escalation",
						Severity:    SeverityCritical,
						Description: fmt.Sprintf("检测到用户 %s 被授予 %s 权限", event.Actor.Name, role),
						Events:      []AuditEvent{event},
						RiskScore:   0.9,
						Suggestions: []string{
							"确认权限授予是否经过审批",
							"检查是否有异常的权限变更链",
						},
					})
				}
			}
		}
	}

	return anomalies
}

// detectDataExfiltration 检测数据泄露
func (l *AuditLogger) detectDataExfiltration() []AnomalyDetectionResult {
	var anomalies []AnomalyDetectionResult

	// 检测大量导出操作
	exportCounts := make(map[string]int)
	for _, event := range l.events {
		if event.EventType == EventExport {
			exportCounts[event.Actor.ID]++
		}
	}

	for actorID, count := range exportCounts {
		if count >= 10 { // 10次以上导出
			anomalies = append(anomalies, AnomalyDetectionResult{
				ID:          fmt.Sprintf("anomaly-exfil-%d", time.Now().UnixNano()),
				DetectedAt:  time.Now(),
				AnomalyType: "data_exfiltration",
				Severity:    SeverityHigh,
				Description: fmt.Sprintf("检测到用户 %s 进行了 %d 次数据导出", actorID, count),
				RiskScore:   0.7,
				Suggestions: []string{
					"审查导出内容",
					"检查是否为正常业务操作",
					"考虑启用 DLP 策略",
				},
			})
		}
	}

	return anomalies
}

// GetUserProfile 获取用户行为画像
func (l *AuditLogger) GetUserProfile(userID string) (*UserBehaviorProfile, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	profile, exists := l.profiles[userID]
	if !exists {
		return nil, fmt.Errorf("用户 %s 的行为画像不存在", userID)
	}

	return profile, nil
}

// ListUserProfiles 列出所有用户行为画像
func (l *AuditLogger) ListUserProfiles() []*UserBehaviorProfile {
	l.mu.RLock()
	defer l.mu.RUnlock()

	var profiles []*UserBehaviorProfile
	for _, profile := range l.profiles {
		profiles = append(profiles, profile)
	}

	return profiles
}

// GenerateAuditReport 生成审计报告
func (l *AuditLogger) GenerateAuditReport(period ReportPeriod) *AuditReport {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// 筛选时间范围内的事件
	var periodEvents []AuditEvent
	for _, event := range l.events {
		if event.Timestamp.After(period.Start) && event.Timestamp.Before(period.End) {
			periodEvents = append(periodEvents, event)
		}
	}

	// 统计事件类型
	eventsByType := make(map[string]int)
	eventsBySeverity := make(map[string]int)
	actorCounts := make(map[string]int)
	actorNames := make(map[string]string)
	successCount := 0
	failureCount := 0
	actors := make(map[string]bool)
	resources := make(map[string]bool)

	for _, event := range periodEvents {
		eventsByType[string(event.EventType)]++
		eventsBySeverity[string(event.Severity)]++

		if event.Result.Status == "success" {
			successCount++
		} else if event.Result.Status == "failure" || event.Result.Status == "denied" {
			failureCount++
		}

		actorCounts[event.Actor.ID]++
		actorNames[event.Actor.ID] = event.Actor.Name
		actors[event.Actor.ID] = true
		resources[event.Resource.ID] = true
	}

	// 构建 Top 事件
	var topEvents []EventTypeCount
	for t, count := range eventsByType {
		topEvents = append(topEvents, EventTypeCount{Type: AuditEventType(t), Count: count})
	}
	sort.Slice(topEvents, func(i, j int) bool {
		return topEvents[i].Count > topEvents[j].Count
	})

	// 构建 Top 执行者
	var topActors []ActorCount
	for id, count := range actorCounts {
		topActors = append(topActors, ActorCount{ActorID: id, ActorName: actorNames[id], Count: count})
	}
	sort.Slice(topActors, func(i, j int) bool {
		return topActors[i].Count > topActors[j].Count
	})

	// 检测异常
	anomalies := l.detectAnomaliesInPeriod(periodEvents)

	// 筛选安全事件
	var securityEvents []AuditEvent
	for _, event := range periodEvents {
		if event.EventType == EventSecurity || event.Severity == SeverityCritical || event.Severity == SeverityHigh {
			securityEvents = append(securityEvents, event)
		}
	}

	totalEvents := len(periodEvents)
	successRate := 0.0
	failureRate := 0.0
	if totalEvents > 0 {
		successRate = float64(successCount) / float64(totalEvents) * 100
		failureRate = float64(failureCount) / float64(totalEvents) * 100
	}

	report := &AuditReport{
		ID:    fmt.Sprintf("audit-report-%d", time.Now().UnixNano()),
		Title: fmt.Sprintf("安全审计报告 (%s - %s)", period.Start.Format("2006-01-02"), period.End.Format("2006-01-02")),
		Period: period,
		Summary: AuditReportSummary{
			TotalEvents:      totalEvents,
			EventsByType:     eventsByType,
			EventsBySeverity: eventsBySeverity,
			SuccessRate:      successRate,
			FailureRate:      failureRate,
			UniqueActors:     len(actors),
			UniqueResources:  len(resources),
		},
		TopEvents:       topEvents,
		TopActors:       topActors,
		Anomalies:       anomalies,
		SecurityEvents:  securityEvents,
		Recommendations: l.generateRecommendations(anomalies),
		GeneratedAt:     time.Now(),
	}

	return report
}

// detectAnomaliesInPeriod 检测指定时间段内的异常
func (l *AuditLogger) detectAnomaliesInPeriod(events []AuditEvent) []AnomalyDetectionResult {
	var anomalies []AnomalyDetectionResult

	// 按 IP 统计失败登录
	ipFailures := make(map[string][]AuditEvent)
	for _, event := range events {
		if event.EventType == EventLogin && event.Result.Status == "failure" {
			ipFailures[event.IPAddress] = append(ipFailures[event.IPAddress], event)
		}
	}

	for ip, failEvents := range ipFailures {
		if len(failEvents) >= 5 {
			anomalies = append(anomalies, AnomalyDetectionResult{
				ID:          fmt.Sprintf("anomaly-%d", time.Now().UnixNano()),
				DetectedAt:  time.Now(),
				AnomalyType: "brute_force",
				Severity:    SeverityHigh,
				Description: fmt.Sprintf("IP %s 登录失败 %d 次", ip, len(failEvents)),
				Events:      failEvents,
				RiskScore:   0.8,
			})
		}
	}

	return anomalies
}

// generateRecommendations 生成审计建议
func (l *AuditLogger) generateRecommendations(anomalies []AnomalyDetectionResult) []string {
	var recommendations []string

	for _, anomaly := range anomalies {
		recommendations = append(recommendations, anomaly.Suggestions...)
	}

	if len(recommendations) == 0 {
		recommendations = append(recommendations, "系统运行正常，建议定期审查审计日志")
	}

	return recommendations
}

// Close 关闭审计日志记录器
func (l *AuditLogger) Close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.writer != nil {
		l.writer.Flush()
	}
	if l.currentFile != nil {
		return l.currentFile.Close()
	}

	return nil
}

// ExportAuditLogs 导出审计日志
func (l *AuditLogger) ExportAuditLogs(query *AuditQuery, format string) ([]byte, error) {
	result, err := l.QueryEvents(query)
	if err != nil {
		return nil, err
	}

	switch strings.ToLower(format) {
	case "jsonl":
		return l.exportJSONL(result.Events)
	case "json":
		return json.MarshalIndent(result.Events, "", "  ")
	default:
		return nil, fmt.Errorf("不支持的导出格式: %s", format)
	}
}

// exportJSONL 导出为 JSONL 格式
func (l *AuditLogger) exportJSONL(events []AuditEvent) ([]byte, error) {
	var buf strings.Builder
	for _, event := range events {
		data, err := json.Marshal(event)
		if err != nil {
			return nil, err
		}
		buf.Write(data)
		buf.WriteString("\n")
	}
	return []byte(buf.String()), nil
}
