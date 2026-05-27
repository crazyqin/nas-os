package smartnotify

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"
)

// Router is the core notification routing engine
type Router struct {
	mu               sync.RWMutex
	rules            map[string]*RoutingRule
	preferences      map[string]*UserPreference
	aggregation      map[string]*PendingAggregation
	history          []*DeliveryResult
	retryQueue       chan *retryTask
	notifQueue       chan *Notification
	deliveryFunc     DeliveryFunc
	aggregationCfg   *AggregationConfig
	retryCfg         *RetryConfig
	stopCh           chan struct{}
	historyMaxSize   int
}

// NewRouter creates a new Router with the given options
func NewRouter(opts ...Option) *Router {
	r := &Router{
		rules:          make(map[string]*RoutingRule),
		preferences:    make(map[string]*UserPreference),
		aggregation:    make(map[string]*PendingAggregation),
		history:        make([]*DeliveryResult, 0, 1000),
		retryQueue:     make(chan *retryTask, 1000),
		notifQueue:     make(chan *Notification, 1000),
		deliveryFunc:   defaultDeliveryFunc,
		aggregationCfg: &AggregationConfig{Enabled: false, Window: 30 * time.Second, MaxCount: 10},
		retryCfg:       &RetryConfig{MaxRetries: 3, InitialWait: time.Second, MaxWait: 30 * time.Second, Multiplier: 2.0},
		stopCh:         make(chan struct{}),
		historyMaxSize: 10000,
	}
	for _, opt := range opts {
		opt(r)
	}
	return r
}

// Option configures the Router
type Option func(*Router)

// WithDeliveryFunc sets the delivery function
func WithDeliveryFunc(fn DeliveryFunc) Option {
	return func(r *Router) {
		r.deliveryFunc = fn
	}
}

// WithAggregationConfig sets aggregation configuration
func WithAggregationConfig(cfg *AggregationConfig) Option {
	return func(r *Router) {
		r.aggregationCfg = cfg
	}
}

// WithRetryConfig sets retry configuration
func WithRetryConfig(cfg *RetryConfig) Option {
	return func(r *Router) {
		r.retryCfg = cfg
	}
}

// WithHistoryMaxSize sets max history size
func WithHistoryMaxSize(size int) Option {
	return func(r *Router) {
		r.historyMaxSize = size
	}
}

// Start starts the router's background workers
func (r *Router) Start(ctx context.Context) {
	go r.processNotifications(ctx)
	go r.processRetries(ctx)
}

// Stop stops the router
func (r *Router) Stop() {
	close(r.stopCh)
}

// AddRule adds a routing rule
func (r *Router) AddRule(rule *RoutingRule) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.rules[rule.ID] = rule
}

// RemoveRule removes a routing rule by ID
func (r *Router) RemoveRule(id string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.rules, id)
}

// GetRule returns a routing rule by ID
func (r *Router) GetRule(id string) (*RoutingRule, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rule, ok := r.rules[id]
	return rule, ok
}

// ListRules returns all routing rules
func (r *Router) ListRules() []*RoutingRule {
	r.mu.RLock()
	defer r.mu.RUnlock()
	rules := make([]*RoutingRule, 0, len(r.rules))
	for _, rule := range r.rules {
		rules = append(rules, rule)
	}
	return rules
}

// SetUserPreference sets notification preferences for a user
func (r *Router) SetUserPreference(pref *UserPreference) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.preferences[pref.UserID] = pref
}

// GetUserPreference returns user preferences
func (r *Router) GetUserPreference(userID string) (*UserPreference, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	pref, ok := r.preferences[userID]
	return pref, ok
}

// DeleteUserPreference deletes user preferences
func (r *Router) DeleteUserPreference(userID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.preferences, userID)
}

// Send submits a notification for routing
func (r *Router) Send(notif *Notification) error {
	if notif.ID == "" {
		return fmt.Errorf("notification ID is required")
	}
	if notif.CreatedAt.IsZero() {
		notif.CreatedAt = time.Now()
	}
	notif.UpdatedAt = notif.CreatedAt
	if notif.Status == "" {
		notif.Status = StatusPending
	}
	r.notifQueue <- notif
	return nil
}

// History returns delivery history, optionally filtered
func (r *Router) History(limit int) []*DeliveryResult {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if limit <= 0 || limit > len(r.history) {
		limit = len(r.history)
	}
	start := len(r.history) - limit
	result := make([]*DeliveryResult, limit)
	copy(result, r.history[start:])
	return result
}

// processNotifications is the main notification processing loop
func (r *Router) processNotifications(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case notif := <-r.notifQueue:
			r.routeNotification(ctx, notif)
		}
	}
}

// processRetries handles retry queue
func (r *Router) processRetries(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-r.stopCh:
			return
		case task := <-r.retryQueue:
			r.executeRetry(ctx, task)
		}
	}
}

// routeNotification routes a single notification
func (r *Router) routeNotification(ctx context.Context, notif *Notification) {
	r.mu.RLock()
	rules := make([]*RoutingRule, 0)
	for _, rule := range r.rules {
		if rule.Enabled && r.matchRule(rule, notif) {
			rules = append(rules, rule)
		}
	}
	r.mu.RUnlock()

	if len(rules) == 0 {
		log.Printf("smartnotify: no matching rule for notification %s (source=%s, priority=%s)",
			notif.ID, notif.Source, notif.Priority)
		return
	}

	// Check quiet hours
	if r.isInQuietHours(notif.Priority) {
		notif.Status = StatusAggregated
		r.addToHistory(&DeliveryResult{
			NotificationID: notif.ID,
			Status:         StatusAggregated,
			SentAt:         time.Now(),
			Error:          "quiet hours active",
		})
		return
	}

	// Check aggregation (skip for already-aggregated notifications)
	if r.aggregationCfg.Enabled && notif.Priority < PriorityCritical && !notif.IsAggregated {
		if r.tryAggregate(notif) {
			return
		}
	}

	// Deliver to matched channels
	for _, rule := range rules {
		for _, ch := range rule.Channels {
			for _, recipient := range rule.Recipients {
				r.deliver(ctx, notif, ch, recipient)
			}
		}
	}
}

// matchRule checks if a notification matches a routing rule
func (r *Router) matchRule(rule *RoutingRule, notif *Notification) bool {
	// Check priority
	if len(rule.Priority) > 0 {
		matched := false
		for _, p := range rule.Priority {
			if p == notif.Priority {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check source
	if len(rule.SourceMatch) > 0 {
		matched := false
		for _, pattern := range rule.SourceMatch {
			if matchSource(notif.Source, pattern) {
				matched = true
				break
			}
		}
		if !matched {
			return false
		}
	}

	// Check time window
	if rule.TimeWindow != nil {
		if !r.inTimeWindow(rule.TimeWindow) {
			return false
		}
	}

	return true
}

// matchSource checks if source matches pattern (supports glob-style * prefix/suffix)
func matchSource(source, pattern string) bool {
	if pattern == "*" {
		return true
	}
	if strings.HasPrefix(pattern, "*") && strings.HasSuffix(pattern, "*") {
		contains := strings.Trim(pattern, "*")
		return strings.Contains(source, contains)
	}
	if strings.HasPrefix(pattern, "*") {
		suffix := strings.TrimPrefix(pattern, "*")
		return strings.HasSuffix(source, suffix)
	}
	if strings.HasSuffix(pattern, "*") {
		prefix := strings.TrimSuffix(pattern, "*")
		return strings.HasPrefix(source, prefix)
	}
	return source == pattern
}

// inTimeWindow checks if current time is within a time window
func (r *Router) inTimeWindow(tw *TimeWindow) bool {
	loc := time.UTC
	if tw.Timezone != "" {
		if l, err := time.LoadLocation(tw.Timezone); err == nil {
			loc = l
		}
	}
	now := time.Now().In(loc)

	// Check day of week
	if len(tw.Days) > 0 {
		today := int(now.Weekday())
		dayMatch := false
		for _, d := range tw.Days {
			if d == today {
				dayMatch = true
				break
			}
		}
		if !dayMatch {
			return false
		}
	}

	// Parse start/end times
	startTime, err := parseHHMM(tw.Start)
	if err != nil {
		return false
	}
	endTime, err := parseHHMM(tw.End)
	if err != nil {
		return false
	}

	currentMinutes := now.Hour()*60 + now.Minute()
	if startTime <= endTime {
		return currentMinutes >= startTime && currentMinutes <= endTime
	}
	// Wraps midnight
	return currentMinutes >= startTime || currentMinutes <= endTime
}

// isInQuietHours checks if current time is in quiet hours
func (r *Router) isInQuietHours(p Priority) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()

	for _, pref := range r.preferences {
		if pref.QuietHours == nil || !pref.QuietHours.Enabled {
			continue
		}
		// Check if priority overrides quiet hours
		for _, op := range pref.QuietHours.Override {
			if op == p {
				return false
			}
		}
		// Check time
		tw := &TimeWindow{
			Start:    pref.QuietHours.Start,
			End:      pref.QuietHours.End,
			Timezone: pref.QuietHours.Timezone,
		}
		if r.inTimeWindow(tw) {
			return true
		}
	}
	return false
}

// tryAggregate attempts to aggregate a notification
func (r *Router) tryAggregate(notif *Notification) bool {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := r.buildAggregationKey(notif)
	keyStr := fmt.Sprintf("%s:%d:%s", key.Source, key.Priority, key.LabelKey)

	if pending, ok := r.aggregation[keyStr]; ok {
		pending.mu.Lock()
		pending.Notifications = append(pending.Notifications, notif)
		count := len(pending.Notifications)
		pending.mu.Unlock()

		if count >= r.aggregationCfg.MaxCount {
			// Flush aggregated notifications
			go r.flushAggregation(keyStr)
		}
		return true
	}

	// Create new aggregation
	pending := &PendingAggregation{
		Key:           key,
		Notifications: []*Notification{notif},
		CreatedAt:     time.Now(),
	}
	r.aggregation[keyStr] = pending

	// Set timer to flush
	pending.Timer = time.AfterFunc(r.aggregationCfg.Window, func() {
		r.flushAggregation(keyStr)
	})

	return true
}

// flushAggregation sends aggregated notifications
func (r *Router) flushAggregation(keyStr string) {
	r.mu.Lock()
	pending, ok := r.aggregation[keyStr]
	if !ok {
		r.mu.Unlock()
		return
	}
	delete(r.aggregation, keyStr)
	r.mu.Unlock()

	pending.mu.Lock()
	notifs := pending.Notifications
	pending.mu.Unlock()

	if pending.Timer != nil {
		pending.Timer.Stop()
	}

	if len(notifs) == 0 {
		return
	}

	// Build aggregated notification
	aggregated := &Notification{
		ID:           fmt.Sprintf("agg-%s-%d", keyStr, time.Now().UnixNano()),
		Title:        fmt.Sprintf("Aggregated: %d notifications", len(notifs)),
		Content:      r.buildAggregatedContent(notifs),
		Priority:     notifs[0].Priority,
		Source:       notifs[0].Source,
		Labels:       notifs[0].Labels,
		Status:       StatusPending,
		IsAggregated: true,
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	// Route aggregated notification (bypass aggregation to avoid recursion)
	r.routeNotification(context.Background(), aggregated)
}

// buildAggregationKey builds an aggregation key from a notification
func (r *Router) buildAggregationKey(notif *Notification) AggregationKey {
	key := AggregationKey{
		Source:   notif.Source,
		Priority: notif.Priority,
	}
	for _, groupBy := range r.aggregationCfg.GroupBy {
		if v, ok := notif.Labels[groupBy]; ok {
			key.LabelKey += groupBy + "=" + v + ";"
		}
	}
	return key
}

// buildAggregatedContent combines multiple notifications into one
func (r *Router) buildAggregatedContent(notifs []*Notification) string {
	var sb strings.Builder
	for i, n := range notifs {
		if i > 0 {
			sb.WriteString("\n---\n")
		}
		sb.WriteString(fmt.Sprintf("[%s] %s: %s", n.Priority, n.Title, n.Content))
	}
	return sb.String()
}

// deliver sends a notification to a specific channel and recipient
func (r *Router) deliver(ctx context.Context, notif *Notification, ch Channel, recipient string) {
	err := r.deliveryFunc(ch, recipient, notif)
	result := &DeliveryResult{
		NotificationID: notif.ID,
		Channel:        ch,
		Recipient:      recipient,
		SentAt:         time.Now(),
	}

	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		// Queue retry
		r.queueRetry(notif, ch, recipient, 0)
	} else {
		result.Status = StatusDelivered
		notif.Status = StatusDelivered
		notif.UpdatedAt = time.Now()
	}

	r.addToHistory(result)
}

// addToHistory adds a delivery result to history
func (r *Router) addToHistory(result *DeliveryResult) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.history = append(r.history, result)
	if len(r.history) > r.historyMaxSize {
		r.history = r.history[len(r.history)-r.historyMaxSize:]
	}
}

type retryTask struct {
	notif     *Notification
	channel   Channel
	recipient string
	retryCount int
}

// queueRetry queues a delivery for retry
func (r *Router) queueRetry(notif *Notification, ch Channel, recipient string, count int) {
	task := &retryTask{
		notif:      notif,
		channel:    ch,
		recipient:  recipient,
		retryCount: count,
	}
	select {
	case r.retryQueue <- task:
	default:
		log.Printf("smartnotify: retry queue full, dropping retry for %s", notif.ID)
	}
}

// executeRetry executes a retry task with exponential backoff
func (r *Router) executeRetry(ctx context.Context, task *retryTask) {
	if task.retryCount >= r.retryCfg.MaxRetries {
		r.addToHistory(&DeliveryResult{
			NotificationID: task.notif.ID,
			Channel:        task.channel,
			Recipient:      task.recipient,
			Status:         StatusFailed,
			Error:          fmt.Sprintf("max retries (%d) exceeded", r.retryCfg.MaxRetries),
			SentAt:         time.Now(),
			RetryCount:     task.retryCount,
		})
		return
	}

	// Exponential backoff
	wait := r.retryCfg.InitialWait
	for i := 0; i < task.retryCount; i++ {
		wait = time.Duration(float64(wait) * r.retryCfg.Multiplier)
		if wait > r.retryCfg.MaxWait {
			wait = r.retryCfg.MaxWait
			break
		}
	}

	select {
	case <-ctx.Done():
		return
	case <-r.stopCh:
		return
	case <-time.After(wait):
	}

	err := r.deliveryFunc(task.channel, task.recipient, task.notif)
	result := &DeliveryResult{
		NotificationID: task.notif.ID,
		Channel:        task.channel,
		Recipient:      task.recipient,
		SentAt:         time.Now(),
		RetryCount:     task.retryCount,
	}

	if err != nil {
		result.Status = StatusFailed
		result.Error = err.Error()
		r.addToHistory(result)
		// Re-queue
		r.queueRetry(task.notif, task.channel, task.recipient, task.retryCount+1)
	} else {
		result.Status = StatusDelivered
		r.addToHistory(result)
	}
}

// defaultDeliveryFunc is a no-op delivery function for testing
func defaultDeliveryFunc(channel Channel, recipient string, notif *Notification) error {
	log.Printf("smartnotify: delivering %s to %s via %s", notif.ID, recipient, channel)
	return nil
}

// parseHHMM parses HH:MM format to minutes since midnight
func parseHHMM(s string) (int, error) {
	var h, m int
	_, err := fmt.Sscanf(s, "%d:%d", &h, &m)
	if err != nil {
		return 0, fmt.Errorf("invalid time format %q: %w", s, err)
	}
	if h < 0 || h > 23 || m < 0 || m > 59 {
		return 0, fmt.Errorf("invalid time %q", s)
	}
	return h*60 + m, nil
}
