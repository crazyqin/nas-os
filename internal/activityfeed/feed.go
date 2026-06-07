package activityfeed

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"
)

// Feed 是活动流的核心管理器，负责活动的记录、查询和订阅。
type Feed struct {
	mu            sync.RWMutex
	activities    []Activity
	config        FeedConfig
	subscriptions map[string]*WebhookConfig
	channels      map[string]chan FeedEvent
	idCounter     int64
}

// NewFeed 创建并返回一个新的活动流实例。
// config 参数指定活动流的配置，如果为 nil 则使用默认配置。
func NewFeed(config *FeedConfig) *Feed {
	cfg := FeedConfig{
		MaxActivities:          10000,
		RetentionDays:          30,
		EnableWebhook:          true,
		DefaultSummarySchedule: "daily",
		ExportFormats:          []string{"json", "csv"},
	}
	if config != nil {
		cfg = *config
	}

	return &Feed{
		activities:    make([]Activity, 0, cfg.MaxActivities),
		config:        cfg,
		subscriptions: make(map[string]*WebhookConfig),
		channels:      make(map[string]chan FeedEvent),
	}
}

// RecordActivity 记录一条新的活动到活动流中。
// 它会自动生成 ID 和时间戳，并通知所有匹配的订阅者。
// 返回记录的活动和可能的错误。
func (f *Feed) RecordActivity(activity Activity) (Activity, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	// 验证必填字段
	if activity.Service == "" {
		return Activity{}, fmt.Errorf("service is required")
	}
	if activity.Action == "" {
		return Activity{}, fmt.Errorf("action is required")
	}
	if activity.Actor.ID == "" {
		return Activity{}, fmt.Errorf("actor ID is required")
	}

	// 生成ID和时间戳
	f.idCounter++
	activity.ID = fmt.Sprintf("act_%d_%d", time.Now().UnixNano(), f.idCounter)
	activity.CreatedAt = time.Now()
	if activity.Timestamp.IsZero() {
		activity.Timestamp = activity.CreatedAt
	}

	// 设置默认严重级别
	if activity.Severity == "" {
		activity.Severity = SeverityInfo
	}

	// 执行活动关联分析
	activity = f.correlateActivities(activity)

	// 添加到活动列表
	f.activities = append(f.activities, activity)

	// 超出容量时清理旧数据
	if len(f.activities) > f.config.MaxActivities {
		excess := len(f.activities) - f.config.MaxActivities
		f.activities = f.activities[excess:]
	}

	// 异步通知订阅者
	go f.notifySubscribers(activity)

	return activity, nil
}

// QueryActivities 根据过滤条件查询活动。
// 返回匹配的活动列表和可能的错误。
func (f *Feed) QueryActivities(filter ActivityFilter) ([]Activity, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	var results []Activity

	for _, act := range f.activities {
		if f.matchesFilter(act, filter) {
			results = append(results, act)
		}
	}

	// 按时间倒序排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Timestamp.After(results[j].Timestamp)
	})

	// 应用分页
	offset := filter.Offset
	limit := filter.Limit
	if limit <= 0 {
		limit = 100
	}

	if offset >= len(results) {
		return []Activity{}, nil
	}

	end := offset + limit
	if end > len(results) {
		end = len(results)
	}

	return results[offset:end], nil
}

// GetSummary 生成指定时间范围内的活动摘要。
// period 参数指定摘要类型（"daily" 或 "weekly"）。
// 返回活动摘要和可能的错误。
func (f *Feed) GetSummary(period string, startTime, endTime time.Time) (ActivitySummary, error) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	if startTime.After(endTime) {
		return ActivitySummary{}, fmt.Errorf("start time must be before end time")
	}

	// 过滤时间范围内的活动
	var filtered []Activity
	for _, act := range f.activities {
		if !act.Timestamp.Before(startTime) && !act.Timestamp.After(endTime) {
			filtered = append(filtered, act)
		}
	}

	// 统计各维度数据
	byService := make(map[ServiceType]int)
	bySeverity := make(map[Severity]int)
	actorCounts := make(map[string]*ActorStat)
	actionCounts := make(map[string]int)

	for _, act := range filtered {
		byService[act.Service]++
		bySeverity[act.Severity]++
		actionCounts[act.Action]++

		key := act.Actor.ID
		if stat, ok := actorCounts[key]; ok {
			stat.Count++
		} else {
			actorCounts[key] = &ActorStat{
				Actor: act.Actor,
				Count: 1,
			}
		}
	}

	// 排序 TopActors
	topActors := make([]ActorStat, 0, len(actorCounts))
	for _, stat := range actorCounts {
		topActors = append(topActors, *stat)
	}
	sort.Slice(topActors, func(i, j int) bool {
		return topActors[i].Count > topActors[j].Count
	})
	if len(topActors) > 10 {
		topActors = topActors[:10]
	}

	// 排序 TopActions
	topActions := make([]ActionStat, 0, len(actionCounts))
	for action, count := range actionCounts {
		topActions = append(topActions, ActionStat{
			Action: action,
			Count:  count,
		})
	}
	sort.Slice(topActions, func(i, j int) bool {
		return topActions[i].Count > topActions[j].Count
	})
	if len(topActions) > 10 {
		topActions = topActions[:10]
	}

	// 生成错误摘要
	var errorSummary string
	errorCount := bySeverity[SeverityError] + bySeverity[SeverityCritical]
	if errorCount > 0 {
		errorSummary = fmt.Sprintf("共 %d 个错误和 %d 个关键事件",
			bySeverity[SeverityError], bySeverity[SeverityCritical])
	}

	return ActivitySummary{
		Period:          period,
		StartTime:       startTime,
		EndTime:         endTime,
		TotalActivities: len(filtered),
		ByService:       byService,
		BySeverity:      bySeverity,
		TopActors:       topActors,
		TopActions:      topActions,
		ErrorSummary:    errorSummary,
		GeneratedAt:     time.Now(),
	}, nil
}

// Subscribe 创建一个新的活动订阅，返回订阅 ID 和事件通道。
// filter 参数指定订阅关注的活动条件。
// 返回的通道会接收匹配的活动事件，调用方负责消费。
// 使用完后应调用 Unsubscribe 关闭通道。
func (f *Feed) Subscribe(url string, filter ActivityFilter) (string, <-chan FeedEvent, error) {
	f.mu.Lock()
	defer f.mu.Unlock()

	if url == "" {
		return "", nil, fmt.Errorf("webhook URL is required")
	}

	f.idCounter++
	subID := fmt.Sprintf("sub_%d_%d", time.Now().UnixNano(), f.idCounter)

	config := &WebhookConfig{
		ID:        subID,
		URL:       url,
		Filter:    filter,
		Enabled:   true,
		CreatedAt: time.Now(),
	}

	ch := make(chan FeedEvent, 100)
	f.subscriptions[subID] = config
	f.channels[subID] = ch

	return subID, ch, nil
}

// Unsubscribe 取消指定的活动订阅并关闭对应的事件通道。
func (f *Feed) Unsubscribe(subscriptionID string) error {
	f.mu.Lock()
	defer f.mu.Unlock()

	if _, ok := f.subscriptions[subscriptionID]; !ok {
		return fmt.Errorf("subscription %s not found", subscriptionID)
	}

	delete(f.subscriptions, subscriptionID)
	if ch, ok := f.channels[subscriptionID]; ok {
		close(ch)
		delete(f.channels, subscriptionID)
	}

	return nil
}

// ExportFeed 将匹配过滤条件的活动导出为指定格式。
// 支持 "json" 和 "csv" 两种格式。
// 返回导出数据和可能的错误。
func (f *Feed) ExportFeed(filter ActivityFilter, format ExportFormat) (ExportData, error) {
	// 查询活动
	activities, err := f.QueryActivities(filter)
	if err != nil {
		return ExportData{}, fmt.Errorf("query failed: %w", err)
	}

	var content []byte
	var filename string

	switch format {
	case FormatJSON:
		content, err = json.MarshalIndent(activities, "", "  ")
		if err != nil {
			return ExportData{}, fmt.Errorf("JSON marshal failed: %w", err)
		}
		filename = fmt.Sprintf("activities_%s.json", time.Now().Format("20060102_150405"))

	case FormatCSV:
		var buf strings.Builder
		writer := csv.NewWriter(&buf)

		// 写入表头
		header := []string{"ID", "Timestamp", "Service", "Action", "Description", "Severity", "Actor", "Resource"}
		if err := writer.Write(header); err != nil {
			return ExportData{}, fmt.Errorf("CSV header write failed: %w", err)
		}

		// 写入数据行
		for _, act := range activities {
			record := []string{
				act.ID,
				act.Timestamp.Format(time.RFC3339),
				string(act.Service),
				act.Action,
				act.Description,
				string(act.Severity),
				act.Actor.Name,
				act.Resource,
			}
			if err := writer.Write(record); err != nil {
				return ExportData{}, fmt.Errorf("CSV record write failed: %w", err)
			}
		}
		writer.Flush()
		if err := writer.Error(); err != nil {
			return ExportData{}, fmt.Errorf("CSV flush failed: %w", err)
		}
		content = []byte(buf.String())
		filename = fmt.Sprintf("activities_%s.csv", time.Now().Format("20060102_150405"))

	default:
		return ExportData{}, fmt.Errorf("unsupported format: %s", format)
	}

	return ExportData{
		Format:     format,
		Filename:   filename,
		Content:    content,
		Count:      len(activities),
		ExportedAt: time.Now(),
	}, nil
}

// correlateActivities 执行活动关联分析，发现相关的事件。
func (f *Feed) correlateActivities(newActivity Activity) Activity {
	// 查找最近的相关活动
	var relatedIDs []string
	window := 5 * time.Minute

	for i := len(f.activities) - 1; i >= 0; i-- {
		act := f.activities[i]

		// 超出时间窗口则停止
		if newActivity.Timestamp.Sub(act.Timestamp) > window {
			break
		}

		// 检查关联条件
		if act.Actor.ID == newActivity.Actor.ID && act.Service == newActivity.Service {
			relatedIDs = append(relatedIDs, act.ID)
		}
		if act.Resource != "" && act.Resource == newActivity.Resource {
			relatedIDs = append(relatedIDs, act.ID)
		}
	}

	if len(relatedIDs) > 0 {
		newActivity.RelatedIDs = relatedIDs
	}

	return newActivity
}

// matchesFilter 检查活动是否匹配过滤条件。
func (f *Feed) matchesFilter(act Activity, filter ActivityFilter) bool {
	// 服务过滤
	if len(filter.Services) > 0 {
		found := false
		for _, s := range filter.Services {
			if act.Service == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 执行者过滤
	if len(filter.ActorIDs) > 0 {
		found := false
		for _, id := range filter.ActorIDs {
			if act.Actor.ID == id {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 严重级别过滤
	if len(filter.Severities) > 0 {
		found := false
		for _, s := range filter.Severities {
			if act.Severity == s {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}

	// 时间过滤
	if filter.StartTime != nil && act.Timestamp.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && act.Timestamp.After(*filter.EndTime) {
		return false
	}

	// 资源过滤
	if filter.Resource != "" && !strings.Contains(act.Resource, filter.Resource) {
		return false
	}

	// 关键词过滤
	if filter.Keyword != "" {
		keyword := strings.ToLower(filter.Keyword)
		if !strings.Contains(strings.ToLower(act.Description), keyword) &&
			!strings.Contains(strings.ToLower(act.Action), keyword) {
			return false
		}
	}

	return true
}

// notifySubscribers 通知所有匹配的订阅者。
func (f *Feed) notifySubscribers(activity Activity) {
	f.mu.RLock()
	defer f.mu.RUnlock()

	event := FeedEvent{
		Activity:  activity,
		Timestamp: time.Now(),
	}

	for subID, config := range f.subscriptions {
		if !config.Enabled {
			continue
		}

		if f.matchesFilter(activity, config.Filter) {
			event.SubscriptionID = subID
			if ch, ok := f.channels[subID]; ok {
				// 非阻塞发送
				select {
				case ch <- event:
				default:
					// 通道满，丢弃事件
				}
			}
		}
	}
}
