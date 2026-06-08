package securityaudit

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AuditLogger 审计日志记录器.
type AuditLogger struct {
	events    []AuditEvent
	retention time.Duration
	mu        sync.RWMutex
}

// NewAuditLogger 创建审计日志记录器.
func NewAuditLogger() *AuditLogger {
	l := &AuditLogger{
		events:    make([]AuditEvent, 0),
		retention: 90 * 24 * time.Hour, // 默认保留 90 天
	}

	// 启动清理例程
	go l.cleanupRoutine()

	return l
}

// cleanupRoutine 清理过期日志.
func (l *AuditLogger) cleanupRoutine() {
	ticker := time.NewTicker(24 * time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		l.cleanup()
	}
}

// cleanup 清理过期日志.
func (l *AuditLogger) cleanup() {
	l.mu.Lock()
	defer l.mu.Unlock()

	cutoff := time.Now().Add(-l.retention)
	valid := make([]AuditEvent, 0)

	for _, event := range l.events {
		if event.Timestamp.After(cutoff) {
			valid = append(valid, event)
		}
	}

	l.events = valid
}

// SetRetention 设置日志保留时间.
func (l *AuditLogger) SetRetention(duration time.Duration) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.retention = duration
}

// Log 记录审计事件.
func (l *AuditLogger) Log(event AuditEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 设置默认值
	if event.ID == "" {
		event.ID = uuid.New().String()
	}
	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}

	l.events = append(l.events, event)
}

// GetLogs 获取审计日志.
func (l *AuditLogger) GetLogs(limit, offset int, filters map[string]string) []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// 应用过滤器
	filtered := make([]AuditEvent, 0)
	for _, event := range l.events {
		if l.matchFilters(event, filters) {
			filtered = append(filtered, event)
		}
	}

	// 按时间倒序排列（最新的在前）
	// 这里简化处理，实际应该使用排序算法
	if offset >= len(filtered) {
		return []AuditEvent{}
	}

	end := offset + limit
	if end > len(filtered) {
		end = len(filtered)
	}

	return filtered[offset:end]
}

// matchFilters 检查事件是否匹配过滤器.
func (l *AuditLogger) matchFilters(event AuditEvent, filters map[string]string) bool {
	if filters == nil {
		return true
	}

	if eventType, ok := filters["event_type"]; ok && string(event.EventType) != eventType {
		return false
	}
	if severity, ok := filters["severity"]; ok && string(event.Severity) != severity {
		return false
	}
	if actor, ok := filters["actor"]; ok && event.Actor != actor {
		return false
	}
	if status, ok := filters["status"]; ok && event.Status != status {
		return false
	}
	if resource, ok := filters["resource"]; ok && event.Resource != resource {
		return false
	}

	return true
}

// GetEvent 获取单个事件.
func (l *AuditLogger) GetEvent(id string) (*AuditEvent, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	for _, event := range l.events {
		if event.ID == id {
			return &event, nil
		}
	}

	return nil, fmt.Errorf("事件 %s 不存在", id)
}

// GenerateReport 生成审计报告.
func (l *AuditLogger) GenerateReport(startTime, endTime time.Time) AuditReport {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// 过滤时间范围内的事件
	events := make([]AuditEvent, 0)
	for _, event := range l.events {
		if event.Timestamp.After(startTime) && event.Timestamp.Before(endTime) {
			events = append(events, event)
		}
	}

	// 统计各类型事件
	byType := make(map[AuditEventType]int)
	bySeverity := make(map[SecurityCheckSeverity]int)
	actorCounts := make(map[string]int)
	actorFailed := make(map[string]int)
	resourceCounts := make(map[string]int)
	resourceLastAccess := make(map[string]time.Time)

	for _, event := range events {
		byType[event.EventType]++
		bySeverity[event.Severity]++

		actorCounts[event.Actor]++
		if event.Status == "failure" {
			actorFailed[event.Actor]++
		}

		if event.Resource != "" {
			resourceCounts[event.Resource]++
			if event.Timestamp.After(resourceLastAccess[event.Resource]) {
				resourceLastAccess[event.Resource] = event.Timestamp
			}
		}
	}

	// 生成 Top Actors
	topActors := make([]ActorStats, 0)
	for actor, count := range actorCounts {
		topActors = append(topActors, ActorStats{
			Actor:       actor,
			EventCount:  count,
			FailedCount: actorFailed[actor],
		})
	}

	// 生成 Top Resources
	topResources := make([]ResourceStats, 0)
	for resource, count := range resourceCounts {
		topResources = append(topResources, ResourceStats{
			Resource:   resource,
			EventCount: count,
			LastAccess: resourceLastAccess[resource],
		})
	}

	// 生成时间线（按小时）
	timeline := l.generateTimeline(events, startTime, endTime)

	return AuditReport{
		ReportID:     uuid.New().String(),
		StartTime:    startTime,
		EndTime:      endTime,
		TotalEvents:  len(events),
		ByType:       byType,
		BySeverity:   bySeverity,
		TopActors:    topActors,
		TopResources: topResources,
		Timeline:     timeline,
	}
}

// generateTimeline 生成时间线.
func (l *AuditLogger) generateTimeline(events []AuditEvent, startTime, endTime time.Time) []TimelineEntry {
	// 按小时分组
	hourlyEvents := make(map[time.Time][]string)
	hourlyCounts := make(map[time.Time]int)

	for _, event := range events {
		hour := event.Timestamp.Truncate(time.Hour)
		hourlyEvents[hour] = append(hourlyEvents[hour], event.ID)
		hourlyCounts[hour]++
	}

	// 生成时间线条目
	timeline := make([]TimelineEntry, 0)
	for hour := startTime.Truncate(time.Hour); hour.Before(endTime); hour = hour.Add(time.Hour) {
		if count, ok := hourlyCounts[hour]; ok {
			timeline = append(timeline, TimelineEntry{
				Timestamp: hour,
				Count:     count,
				Events:    hourlyEvents[hour],
			})
		}
	}

	return timeline
}

// ExportLogs 导出日志.
func (l *AuditLogger) ExportLogs(startTime, endTime time.Time, format string) ([]byte, error) {
	l.mu.RLock()
	defer l.mu.RUnlock()

	// 过滤时间范围内的事件
	events := make([]AuditEvent, 0)
	for _, event := range l.events {
		if event.Timestamp.After(startTime) && event.Timestamp.Before(endTime) {
			events = append(events, event)
		}
	}

	switch format {
	case "json":
		return json.MarshalIndent(events, "", "  ")
	case "csv":
		return l.exportCSV(events)
	default:
		return nil, fmt.Errorf("不支持的导出格式: %s", format)
	}
}

// exportCSV 导出为 CSV 格式.
func (l *AuditLogger) exportCSV(events []AuditEvent) ([]byte, error) {
	buf := &bytes.Buffer{}
	writer := csv.NewWriter(buf)

	// 写入表头
	header := []string{"ID", "Timestamp", "EventType", "Severity", "Actor", "ActorIP", "Resource", "Action", "Status", "Message"}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// 写入数据
	for _, event := range events {
		row := []string{
			event.ID,
			event.Timestamp.Format(time.RFC3339),
			string(event.EventType),
			string(event.Severity),
			event.Actor,
			event.ActorIP,
			event.Resource,
			event.Action,
			event.Status,
			event.Message,
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buf.Bytes(), nil
}

// GetStats 获取日志统计.
func (l *AuditLogger) GetStats() map[string]interface{} {
	l.mu.RLock()
	defer l.mu.RUnlock()

	total := len(l.events)
	byType := make(map[AuditEventType]int)
	bySeverity := make(map[SecurityCheckSeverity]int)
	lastEvent := time.Time{}

	for _, event := range l.events {
		byType[event.EventType]++
		bySeverity[event.Severity]++
		if event.Timestamp.After(lastEvent) {
			lastEvent = event.Timestamp
		}
	}

	return map[string]interface{}{
		"total_events":  total,
		"by_type":       byType,
		"by_severity":   bySeverity,
		"last_event":    lastEvent,
		"retention":     l.retention.String(),
	}
}

// SearchEvents 搜索事件.
func (l *AuditLogger) SearchEvents(query string, limit int) []AuditEvent {
	l.mu.RLock()
	defer l.mu.RUnlock()

	results := make([]AuditEvent, 0)
	query = strings.ToLower(query)

	for _, event := range l.events {
		if l.eventMatchesQuery(event, query) {
			results = append(results, event)
			if len(results) >= limit {
				break
			}
		}
	}

	return results
}

// eventMatchesQuery 检查事件是否匹配查询.
func (l *AuditLogger) eventMatchesQuery(event AuditEvent, query string) bool {
	if strings.Contains(strings.ToLower(event.Actor), query) {
		return true
	}
	if strings.Contains(strings.ToLower(event.Resource), query) {
		return true
	}
	if strings.Contains(strings.ToLower(event.Action), query) {
		return true
	}
	if strings.Contains(strings.ToLower(event.Message), query) {
		return true
	}
	if strings.Contains(strings.ToLower(event.ActorIP), query) {
		return true
	}
	return false
}

// ClearLogs 清除所有日志.
func (l *AuditLogger) ClearLogs() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = make([]AuditEvent, 0)
}

// GetLogCount 获取日志数量.
func (l *AuditLogger) GetLogCount() int {
	l.mu.RLock()
	defer l.mu.RUnlock()
	return len(l.events)
}
