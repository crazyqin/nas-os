// Package notificationcenter 通知中心核心逻辑
package notificationcenter

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"text/template"
	"time"

	"go.uber.org/zap"
)

// ========== 通知中心 ==========

// Center 通知中心
type Center struct {
	mu            sync.RWMutex
	logger        *zap.Logger
	notifications map[string]*Notification
	templates     map[string]*NotificationTemplate
	rules         map[string]*NotificationRule
	silentPeriods map[string]*SilentPeriod
	preferences   map[string]*UserPreference
	aggregations  map[string]*AggregationEntry
	aggWindow     time.Duration // 聚合窗口，默认 5 分钟
	handlers      map[Channel][]NotificationHandler
}

// NotificationHandler 通知投递处理函数
type NotificationHandler func(ctx context.Context, notif *Notification, address string) error

// CenterOption 通知中心配置选项
type CenterOption func(*Center)

// WithAggWindow 设置聚合窗口
func WithAggWindow(d time.Duration) CenterOption {
	return func(c *Center) {
		c.aggWindow = d
	}
}

// WithLogger 设置日志器
func WithLogger(l *zap.Logger) CenterOption {
	return func(c *Center) {
		c.logger = l
	}
}

// NewCenter 创建通知中心
func NewCenter(opts ...CenterOption) *Center {
	c := &Center{
		logger:        zap.NewNop(),
		notifications: make(map[string]*Notification),
		templates:     make(map[string]*NotificationTemplate),
		rules:         make(map[string]*NotificationRule),
		silentPeriods: make(map[string]*SilentPeriod),
		preferences:   make(map[string]*UserPreference),
		aggregations:  make(map[string]*AggregationEntry),
		aggWindow:     5 * time.Minute,
		handlers:      make(map[Channel][]NotificationHandler),
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

// RegisterHandler 注册渠道投递处理函数
func (c *Center) RegisterHandler(ch Channel, h NotificationHandler) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handlers[ch] = append(c.handlers[ch], h)
}

// ========== 通知发送 ==========

// SendRequest 发送通知请求
type SendRequest struct {
	Title      string                 `json:"title"`
	Content    string                 `json:"content"`
	Priority   Priority               `json:"priority"`
	Category   string                 `json:"category"`
	Channels   []Channel              `json:"channels"`
	Source     string                 `json:"source,omitempty"`
	Labels     map[string]string      `json:"labels,omitempty"`
	Metadata   map[string]interface{} `json:"metadata,omitempty"`
	AggKey     string                 `json:"agg_key,omitempty"`
	TemplateID string                 `json:"template_id,omitempty"`
	TmplVars   map[string]interface{} `json:"template_vars,omitempty"`
}

// Send 发送通知
func (c *Center) Send(ctx context.Context, req *SendRequest) (*Notification, error) {
	// 使用模板
	if req.TemplateID != "" {
		if err := c.applyTemplate(req); err != nil {
			return nil, err
		}
	}

	// 检查聚合
	if req.AggKey != "" {
		if merged := c.checkAggregation(req); merged != nil {
			return merged, nil
		}
	}

	// 检查静默时段
	if c.isSilenced(req.Priority, req.Labels) {
		c.logger.Debug("notification silenced",
			zap.String("title", req.Title),
			zap.String("priority", string(req.Priority)),
		)
		// 紧急通知仍然入库，只是不推送
	}

	notif := &Notification{
		ID:        generateID(),
		Title:     req.Title,
		Content:   req.Content,
		Priority:  req.Priority,
		Category:  req.Category,
		Status:    StatusUnread,
		Channels:  req.Channels,
		Source:    req.Source,
		Labels:    req.Labels,
		Metadata:  req.Metadata,
		AggKey:    req.AggKey,
		AggCount:  1,
		CreatedAt: time.Now(),
	}

	c.mu.Lock()
	c.notifications[notif.ID] = notif
	// 更新聚合
	if req.AggKey != "" {
		c.aggregations[req.AggKey] = &AggregationEntry{
			AggKey:    req.AggKey,
			Count:     1,
			FirstAt:   notif.CreatedAt,
			LastAt:    notif.CreatedAt,
			LastNotif: notif,
			Window:    c.aggWindow,
		}
	}
	c.mu.Unlock()

	// 投递到各渠道
	if !c.isSilenced(req.Priority, req.Labels) {
		go c.deliver(ctx, notif)
	}

	c.logger.Info("notification created",
		zap.String("id", notif.ID),
		zap.String("title", notif.Title),
		zap.String("priority", string(notif.Priority)),
	)

	return notif, nil
}

// applyTemplate 应用通知模板
func (c *Center) applyTemplate(req *SendRequest) error {
	c.mu.RLock()
	tmpl, ok := c.templates[req.TemplateID]
	c.mu.RUnlock()

	if !ok {
		return ErrTemplateNotFound
	}
	if !tmpl.Enabled {
		return fmt.Errorf("template %s is disabled", req.TemplateID)
	}

	vars := req.TmplVars
	if vars == nil {
		vars = make(map[string]interface{})
	}

	subject, err := renderTemplate(tmpl.Subject, vars)
	if err != nil {
		return fmt.Errorf("render subject: %w", err)
	}
	body, err := renderTemplate(tmpl.Body, vars)
	if err != nil {
		return fmt.Errorf("render body: %w", err)
	}

	if req.Title == "" {
		req.Title = subject
	}
	if req.Content == "" {
		req.Content = body
	}
	if req.Priority == "" {
		req.Priority = tmpl.Priority
	}
	if len(req.Channels) == 0 {
		req.Channels = []Channel{tmpl.Channel}
	}
	return nil
}

func renderTemplate(tmplStr string, vars map[string]interface{}) (string, error) {
	t, err := template.New("").Parse(tmplStr)
	if err != nil {
		return "", err
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, vars); err != nil {
		return "", err
	}
	return buf.String(), nil
}

// checkAggregation 检查通知聚合，如果在窗口内已有相同 key 的通知，返回合并后的通知
func (c *Center) checkAggregation(req *SendRequest) *Notification {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.aggregations[req.AggKey]
	if !ok {
		return nil
	}

	if time.Since(entry.LastAt) > c.aggWindow {
		return nil // 窗口已过期
	}

	// 合并：更新计数和时间
	entry.Count++
	entry.LastAt = time.Now()
	if entry.LastNotif != nil {
		entry.LastNotif.AggCount = entry.Count
		entry.LastNotif.CreatedAt = entry.LastAt
	}
	return entry.LastNotif
}

// deliver 投递通知到各渠道
func (c *Center) deliver(ctx context.Context, notif *Notification) {
	c.mu.RLock()
	handlers := c.handlers
	preferences := c.preferences
	c.mu.RUnlock()

	for _, ch := range notif.Channels {
		hs, ok := handlers[ch]
		if !ok || len(hs) == 0 {
			c.logger.Debug("no handler for channel", zap.String("channel", string(ch)))
			continue
		}

		// 查找需要投递的用户地址
		addresses := c.resolveAddresses(ch, preferences)

		for _, h := range hs {
			for _, addr := range addresses {
				if err := h(ctx, notif, addr); err != nil {
					c.logger.Error("deliver failed",
						zap.String("channel", string(ch)),
						zap.String("address", addr),
						zap.Error(err),
					)
				}
			}
		}
	}
}

// resolveAddresses 解析渠道投递地址
func (c *Center) resolveAddresses(ch Channel, prefs map[string]*UserPreference) []string {
	var addrs []string
	for _, pref := range prefs {
		cp, ok := pref.Channels[ch]
		if !ok || !cp.Enabled {
			continue
		}
		if cp.Address != "" {
			addrs = append(addrs, cp.Address)
		} else {
			addrs = append(addrs, pref.UserID)
		}
	}
	return addrs
}

// isSilenced 检查是否在静默时段
func (c *Center) isSilenced(prio Priority, labels map[string]string) bool {
	if prio == PriorityCritical {
		return false
	}

	c.mu.RLock()
	defer c.mu.RUnlock()

	now := time.Now()
	for _, sp := range c.silentPeriods {
		if !sp.IsActive(now) {
			continue
		}
		if sp.IsPrioritySilent(prio) {
			return true
		}
	}
	return false
}

// ========== 通知读取与管理 ==========

// Get 获取通知
func (c *Center) Get(id string) (*Notification, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	notif, ok := c.notifications[id]
	if !ok {
		return nil, ErrNotificationNotFound
	}
	return notif, nil
}

// List 列出通知
func (c *Center) List(filter *ListFilter) []*Notification {
	c.mu.RLock()
	defer c.mu.RUnlock()

	var result []*Notification
	for _, n := range c.notifications {
		if filter != nil && !filter.Match(n) {
			continue
		}
		result = append(result, n)
	}

	// 按创建时间倒序
	sortNotifications(result)
	return result
}

// MarkAsRead 标记为已读
func (c *Center) MarkAsRead(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	notif, ok := c.notifications[id]
	if !ok {
		return ErrNotificationNotFound
	}
	now := time.Now()
	notif.Status = StatusRead
	notif.ReadAt = &now
	return nil
}

// MarkAllAsRead 全部标记已读
func (c *Center) MarkAllAsRead() int {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := time.Now()
	count := 0
	for _, n := range c.notifications {
		if n.Status == StatusUnread {
			n.Status = StatusRead
			n.ReadAt = &now
			count++
		}
	}
	return count
}

// Archive 归档通知
func (c *Center) Archive(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	notif, ok := c.notifications[id]
	if !ok {
		return ErrNotificationNotFound
	}
	notif.Status = StatusArchived
	return nil
}

// Delete 删除通知
func (c *Center) Delete(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.notifications[id]; !ok {
		return ErrNotificationNotFound
	}
	delete(c.notifications, id)
	return nil
}

// Summary 获取通知摘要
func (c *Center) Summary() *NotificationSummary {
	c.mu.RLock()
	defer c.mu.RUnlock()

	summary := &NotificationSummary{
		ByPriority: make(map[Priority]int),
		ByCategory: make(map[string]int),
	}
	for _, n := range c.notifications {
		switch n.Status {
		case StatusUnread:
			summary.TotalUnread++
		case StatusRead:
			summary.TotalRead++
		}
		summary.ByPriority[n.Priority]++
		if n.Category != "" {
			summary.ByCategory[n.Category]++
		}
	}
	return summary
}

// ========== 模板管理 ==========

// AddTemplate 添加模板
func (c *Center) AddTemplate(tmpl *NotificationTemplate) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if tmpl.CreatedAt.IsZero() {
		tmpl.CreatedAt = time.Now()
	}
	tmpl.UpdatedAt = time.Now()
	c.templates[tmpl.ID] = tmpl
}

// GetTemplate 获取模板
func (c *Center) GetTemplate(id string) (*NotificationTemplate, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	t, ok := c.templates[id]
	if !ok {
		return nil, ErrTemplateNotFound
	}
	return t, nil
}

// ListTemplates 列出模板
func (c *Center) ListTemplates() []*NotificationTemplate {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []*NotificationTemplate
	for _, t := range c.templates {
		result = append(result, t)
	}
	return result
}

// DeleteTemplate 删除模板
func (c *Center) DeleteTemplate(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.templates[id]; !ok {
		return ErrTemplateNotFound
	}
	delete(c.templates, id)
	return nil
}

// ========== 规则管理 ==========

// AddRule 添加规则
func (c *Center) AddRule(rule *NotificationRule) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	// 检查名称重复
	for _, r := range c.rules {
		if r.Name == rule.Name && r.ID != rule.ID {
			return ErrDuplicateRuleName
		}
	}
	if rule.CreatedAt.IsZero() {
		rule.CreatedAt = time.Now()
	}
	rule.UpdatedAt = time.Now()
	c.rules[rule.ID] = rule
	return nil
}

// GetRule 获取规则
func (c *Center) GetRule(id string) (*NotificationRule, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	r, ok := c.rules[id]
	if !ok {
		return nil, ErrRuleNotFound
	}
	return r, nil
}

// ListRules 列出规则
func (c *Center) ListRules() []*NotificationRule {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []*NotificationRule
	for _, r := range c.rules {
		result = append(result, r)
	}
	return result
}

// UpdateRule 更新规则
func (c *Center) UpdateRule(rule *NotificationRule) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.rules[rule.ID]; !ok {
		return ErrRuleNotFound
	}
	// 检查名称重复
	for _, r := range c.rules {
		if r.Name == rule.Name && r.ID != rule.ID {
			return ErrDuplicateRuleName
		}
	}
	rule.UpdatedAt = time.Now()
	c.rules[rule.ID] = rule
	return nil
}

// DeleteRule 删除规则
func (c *Center) DeleteRule(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.rules[id]; !ok {
		return ErrRuleNotFound
	}
	delete(c.rules, id)
	return nil
}

// EvaluateRules 评估所有规则，对输入事件进行匹配
func (c *Center) EvaluateRules(ctx context.Context, event map[string]interface{}) ([]*Notification, error) {
	c.mu.RLock()
	var rules []*NotificationRule
	for _, r := range c.rules {
		if r.Enabled {
			rules = append(rules, r)
		}
	}
	c.mu.RUnlock()

	var fired []*Notification
	for _, rule := range rules {
		if c.matchRule(rule, event) {
			// 检查节流
			if rule.Throttle > 0 {
				if time.Since(rule.lastFiredAt) < time.Duration(rule.Throttle)*time.Second {
					continue
				}
			}
			rule.lastFiredAt = time.Now()

			req := &SendRequest{
				Title:    fmt.Sprintf("[Rule] %s", rule.Name),
				Content:  fmt.Sprintf("Rule %s triggered", rule.Name),
				Priority: rule.Priority,
				Category: rule.Category,
				Channels: rule.Channels,
				Labels:   rule.Labels,
			}

			notif, err := c.Send(ctx, req)
			if err != nil {
				c.logger.Error("rule send failed", zap.String("rule", rule.Name), zap.Error(err))
				continue
			}
			fired = append(fired, notif)
		}
	}
	return fired, nil
}

// matchRule 检查事件是否匹配规则条件
func (c *Center) matchRule(rule *NotificationRule, event map[string]interface{}) bool {
	if len(rule.Conditions) == 0 {
		return false
	}

	logic := rule.Logic
	if logic == "" {
		logic = "and"
	}

	matchCount := 0
	for _, cond := range rule.Conditions {
		val, ok := event[cond.Field]
		if !ok {
			if logic == "and" {
				return false
			}
			continue
		}
		if matchCondition(cond, val) {
			matchCount++
		} else if logic == "and" {
			return false
		}
	}

	if logic == "or" {
		return matchCount > 0
	}
	return matchCount == len(rule.Conditions)
}

// matchCondition 匹配单个条件
func matchCondition(cond RuleCondition, actual interface{}) bool {
	switch cond.Operator {
	case "==":
		return fmt.Sprintf("%v", actual) == fmt.Sprintf("%v", cond.Value)
	case "!=":
		return fmt.Sprintf("%v", actual) != fmt.Sprintf("%v", cond.Value)
	case "contains":
		return strings.Contains(fmt.Sprintf("%v", actual), fmt.Sprintf("%v", cond.Value))
	case "regex":
		re, err := regexp.Compile(fmt.Sprintf("%v", cond.Value))
		if err != nil {
			return false
		}
		return re.MatchString(fmt.Sprintf("%v", actual))
	case ">", "<", ">=", "<=":
		return compareNumeric(actual, cond.Value, cond.Operator)
	default:
		return false
	}
}

// compareNumeric 数值比较
func compareNumeric(a, b interface{}, op string) bool {
	af, okA := toFloat(a)
	bf, okB := toFloat(b)
	if !okA || !okB {
		return false
	}
	switch op {
	case ">":
		return af > bf
	case "<":
		return af < bf
	case ">=":
		return af >= bf
	case "<=":
		return af <= bf
	}
	return false
}

func toFloat(v interface{}) (float64, bool) {
	switch val := v.(type) {
	case float64:
		return val, true
	case float32:
		return float64(val), true
	case int:
		return float64(val), true
	case int64:
		return float64(val), true
	case int32:
		return float64(val), true
	case uint:
		return float64(val), true
	case uint64:
		return float64(val), true
	default:
		return 0, false
	}
}

// ========== 静默时段管理 ==========

// AddSilentPeriod 添加静默时段
func (c *Center) AddSilentPeriod(sp *SilentPeriod) error {
	if sp.StartHour < 0 || sp.StartHour > 23 || sp.EndHour < 0 || sp.EndHour > 23 {
		return ErrInvalidSilentPeriod
	}
	if sp.StartMinute < 0 || sp.StartMinute > 59 || sp.EndMinute < 0 || sp.EndMinute > 59 {
		return ErrInvalidSilentPeriod
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if sp.CreatedAt.IsZero() {
		sp.CreatedAt = time.Now()
	}
	sp.UpdatedAt = time.Now()
	c.silentPeriods[sp.ID] = sp
	return nil
}

// ListSilentPeriods 列出静默时段
func (c *Center) ListSilentPeriods() []*SilentPeriod {
	c.mu.RLock()
	defer c.mu.RUnlock()
	var result []*SilentPeriod
	for _, sp := range c.silentPeriods {
		result = append(result, sp)
	}
	return result
}

// DeleteSilentPeriod 删除静默时段
func (c *Center) DeleteSilentPeriod(id string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if _, ok := c.silentPeriods[id]; !ok {
		return ErrRuleNotFound
	}
	delete(c.silentPeriods, id)
	return nil
}

// ========== 用户偏好管理 ==========

// GetPreference 获取用户偏好
func (c *Center) GetPreference(userID string) *UserPreference {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.preferences[userID]
	if !ok {
		return DefaultUserPreference(userID)
	}
	return p
}

// SetPreference 设置用户偏好
func (c *Center) SetPreference(pref *UserPreference) {
	c.mu.Lock()
	defer c.mu.Unlock()
	pref.UpdatedAt = time.Now()
	c.preferences[pref.UserID] = pref
}

// ========== 列表过滤 ==========

// ListFilter 通知列表过滤器
type ListFilter struct {
	Status   *NotificationStatus `json:"status,omitempty"`
	Priority *Priority           `json:"priority,omitempty"`
	Category string              `json:"category,omitempty"`
	Keyword  string              `json:"keyword,omitempty"`
	Limit    int                 `json:"limit,omitempty"`
	Offset   int                 `json:"offset,omitempty"`
}

// Match 检查通知是否匹配过滤器
func (f *ListFilter) Match(n *Notification) bool {
	if f == nil {
		return true
	}
	if f.Status != nil && n.Status != *f.Status {
		return false
	}
	if f.Priority != nil && n.Priority != *f.Priority {
		return false
	}
	if f.Category != "" && n.Category != f.Category {
		return false
	}
	if f.Keyword != "" {
		keyword := strings.ToLower(f.Keyword)
		if !strings.Contains(strings.ToLower(n.Title), keyword) &&
			!strings.Contains(strings.ToLower(n.Content), keyword) {
			return false
		}
	}
	return true
}

// ========== 工具函数 ==========

func generateID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// sortNotifications 按创建时间倒序排列
func sortNotifications(list []*Notification) {
	for i := 1; i < len(list); i++ {
		key := list[i]
		j := i - 1
		for j >= 0 && list[j].CreatedAt.Before(key.CreatedAt) {
			list[j+1] = list[j]
			j--
		}
		list[j+1] = key
	}
	// 反转为倒序
	for i, j := 0, len(list)-1; i < j; i, j = i+1, j-1 {
		list[i], list[j] = list[j], list[i]
	}
}
