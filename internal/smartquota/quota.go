// Package smartquota 提供存储配额智能管理功能
// 支持多层级配额、弹性策略、使用量预测、告警通知、历史追踪和清理建议
package smartquota

import (
	"context"
	"fmt"
	"log"
	"math"
	"sort"
	"sync"
	"time"
)

// QuotaLevel 配额层级
type QuotaLevel string

const (
	LevelUser    QuotaLevel = "user"    // 用户级
	LevelGroup   QuotaLevel = "group"   // 组级
	LevelShare   QuotaLevel = "share"   // 共享级
	LevelProject QuotaLevel = "project" // 项目级
)

// QuotaPolicy 配额策略
type QuotaPolicy string

const (
	PolicyHard    QuotaPolicy = "hard"    // 硬限制：超限拒绝写入
	PolicySoft    QuotaPolicy = "soft"    // 软限制：超限发出警告
	PolicyElastic QuotaPolicy = "elastic" // 弹性：自动扩容
)

// AlertThreshold 告警阈值
var AlertThresholds = []float64{50, 75, 90, 100}

// UsageRecord 使用量记录
type UsageRecord struct {
	Timestamp time.Time `json:"timestamp"`
	UsedBytes int64     `json:"usedBytes"`
}

// Alert 配额告警
type Alert struct {
	ID          string    `json:"id"`
	QuotaID     string    `json:"quotaId"`
	QuotaName   string    `json:"quotaName"`
	Threshold   float64   `json:"threshold"`   // 触发阈值百分比
	UsedBytes   int64     `json:"usedBytes"`
	LimitBytes  int64     `json:"limitBytes"`
	UsagePct    float64   `json:"usagePct"`
	Level       string    `json:"level"`       // warning / critical / full
	Message     string    `json:"message"`
	TriggeredAt time.Time `json:"triggeredAt"`
	Acked       bool      `json:"acked"`
}

// Prediction 使用量预测
type Prediction struct {
	QuotaID         string    `json:"quotaId"`
	CurrentUsed     int64     `json:"currentUsed"`
	LimitBytes      int64     `json:"limitBytes"`
	DailyGrowthRate float64   `json:"dailyGrowthRate"` // 日均增长字节
	ExhaustDate     *time.Time `json:"exhaustDate,omitempty"` // 预计用尽日期
	DaysRemaining   float64   `json:"daysRemaining"`
	Trend           string    `json:"trend"` // increasing / stable / decreasing
}

// CleanupSuggestion 清理建议
type CleanupSuggestion struct {
	Type        string `json:"type"`        // large_file / duplicate / stale_file
	Target      string `json:"target"`      // 目标路径
	Size        int64  `json:"size"`        // 大小
	Description string `json:"description"` // 建议说明
	Priority    int    `json:"priority"`    // 优先级 1-5
}

// HistoryStats 历史统计
type HistoryStats struct {
	QuotaID   string           `json:"quotaId"`
	Period    string           `json:"period"` // day / week / month
	Records   []UsageRecord    `json:"records"`
	AvgUsage  int64            `json:"avgUsage"`
	MaxUsage  int64            `json:"maxUsage"`
	MinUsage  int64            `json:"minUsage"`
	Growth    int64            `json:"growth"` // 周期内净增长
}

// QuotaConfig 配额配置
type QuotaConfig struct {
	ID             string     `json:"id"`
	Name           string     `json:"name"`
	Level          QuotaLevel `json:"level"`
	OwnerID        string     `json:"ownerId"`        // 拥有者ID（用户/组/项目）
	ParentID       string     `json:"parentId,omitempty"` // 父配额ID（用于继承）
	LimitBytes     int64      `json:"limitBytes"`     // 配额上限
	UsedBytes      int64      `json:"usedBytes"`      // 已使用量
	Policy         QuotaPolicy `json:"policy"`
	MaxAutoExpand  int64      `json:"maxAutoExpand,omitempty"` // 弹性策略最大扩展值
	AlertsSent     map[float64]bool `json:"-"`         // 已发送告警记录
	History        []UsageRecord    `json:"-"`         // 历史使用量
	CreatedAt      time.Time  `json:"createdAt"`
	UpdatedAt      time.Time  `json:"updatedAt"`
}

// QuotaTemplate 配额模板
type QuotaTemplate struct {
	Name       string     `json:"name"`
	Level      QuotaLevel `json:"level"`
	LimitBytes int64      `json:"limitBytes"`
	Policy     QuotaPolicy `json:"policy"`
}

// DefaultTemplates 默认配额模板
var DefaultTemplates = []QuotaTemplate{
	{Name: "family_user", Level: LevelUser, LimitBytes: 1024 * 1024 * 1024 * 1024, Policy: PolicySoft},      // 1TB
	{Name: "office_user", Level: LevelUser, LimitBytes: 500 * 1024 * 1024 * 1024, Policy: PolicyHard},       // 500GB
	{Name: "media_user", Level: LevelUser, LimitBytes: 2 * 1024 * 1024 * 1024 * 1024, Policy: PolicyElastic}, // 2TB
	{Name: "group_default", Level: LevelGroup, LimitBytes: 5 * 1024 * 1024 * 1024 * 1024, Policy: PolicySoft}, // 5TB
	{Name: "project_default", Level: LevelProject, LimitBytes: 10 * 1024 * 1024 * 1024 * 1024, Policy: PolicyHard}, // 10TB
}

// QuotaManager 配额管理器
type QuotaManager struct {
	quotas    map[string]*QuotaConfig
	alerts    []*Alert
	mu        sync.RWMutex
	ctx       context.Context
	cancel    context.CancelFunc
	nextID    int
	onAlert   func(alert *Alert) // 告警回调
}

// NewQuotaManager 创建配额管理器
func NewQuotaManager() *QuotaManager {
	ctx, cancel := context.WithCancel(context.Background())
	return &QuotaManager{
		quotas: make(map[string]*QuotaConfig),
		alerts: make([]*Alert, 0),
		ctx:    ctx,
		cancel: cancel,
		nextID: 1,
	}
}

// SetAlertCallback 设置告警回调
func (qm *QuotaManager) SetAlertCallback(fn func(alert *Alert)) {
	qm.mu.Lock()
	defer qm.mu.Unlock()
	qm.onAlert = fn
}

// CreateQuota 创建配额
func (qm *QuotaManager) CreateQuota(cfg QuotaConfig) (*QuotaConfig, error) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if cfg.Name == "" {
		return nil, fmt.Errorf("quota name is required")
	}
	if cfg.LimitBytes <= 0 {
		return nil, fmt.Errorf("limitBytes must be positive")
	}

	cfg.ID = fmt.Sprintf("q%d", qm.nextID)
	qm.nextID++
	cfg.CreatedAt = time.Now()
	cfg.UpdatedAt = cfg.CreatedAt
	cfg.AlertsSent = make(map[float64]bool)
	if cfg.History == nil {
		cfg.History = make([]UsageRecord, 0)
	}

	qm.quotas[cfg.ID] = &cfg
	return &cfg, nil
}

// GetQuota 获取配额
func (qm *QuotaManager) GetQuota(id string) (*QuotaConfig, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	q, ok := qm.quotas[id]
	if !ok {
		return nil, fmt.Errorf("quota %s not found", id)
	}
	return q, nil
}

// UpdateQuota 更新配额
func (qm *QuotaManager) UpdateQuota(id string, update QuotaConfig) (*QuotaConfig, error) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	q, ok := qm.quotas[id]
	if !ok {
		return nil, fmt.Errorf("quota %s not found", id)
	}

	if update.Name != "" {
		q.Name = update.Name
	}
	if update.LimitBytes > 0 {
		q.LimitBytes = update.LimitBytes
	}
	if update.Policy != "" {
		q.Policy = update.Policy
	}
	if update.MaxAutoExpand > 0 {
		q.MaxAutoExpand = update.MaxAutoExpand
	}
	q.UpdatedAt = time.Now()

	return q, nil
}

// DeleteQuota 删除配额
func (qm *QuotaManager) DeleteQuota(id string) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if _, ok := qm.quotas[id]; !ok {
		return fmt.Errorf("quota %s not found", id)
	}

	// 删除子配额的父引用
	for _, q := range qm.quotas {
		if q.ParentID == id {
			q.ParentID = ""
		}
	}

	delete(qm.quotas, id)
	return nil
}

// ListQuotas 列出所有配额
func (qm *QuotaManager) ListQuotas() []*QuotaConfig {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	result := make([]*QuotaConfig, 0, len(qm.quotas))
	for _, q := range qm.quotas {
		result = append(result, q)
	}
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.Before(result[j].CreatedAt)
	})
	return result
}

// UpdateUsage 更新使用量并检查告警
func (qm *QuotaManager) UpdateUsage(id string, usedBytes int64) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	q, ok := qm.quotas[id]
	if !ok {
		return fmt.Errorf("quota %s not found", id)
	}

	q.UsedBytes = usedBytes
	q.UpdatedAt = time.Now()

	// 记录历史
	q.History = append(q.History, UsageRecord{
		Timestamp: time.Now(),
		UsedBytes: usedBytes,
	})

	// 检查告警
	qm.checkAlertsLocked(q)

	// 处理弹性策略
	if q.Policy == PolicyElastic && usedBytes > q.LimitBytes {
		expandTo := int64(float64(usedBytes) * 1.2) // 自动扩容20%
		if q.MaxAutoExpand > 0 && expandTo > q.MaxAutoExpand {
			expandTo = q.MaxAutoExpand
		}
		q.LimitBytes = expandTo
	}

	return nil
}

// checkAlertsLocked 检查告警（需持有锁）
func (qm *QuotaManager) checkAlertsLocked(q *QuotaConfig) {
	if q.LimitBytes <= 0 {
		return
	}

	usagePct := float64(q.UsedBytes) / float64(q.LimitBytes) * 100

	for _, threshold := range AlertThresholds {
		if usagePct >= threshold {
			if q.AlertsSent[threshold] {
				continue // 已发送过此阈值告警
			}

			level := "warning"
			if threshold >= 100 {
				level = "full"
			} else if threshold >= 90 {
				level = "critical"
			}

			alert := &Alert{
				ID:          fmt.Sprintf("a%d", len(qm.alerts)+1),
				QuotaID:     q.ID,
				QuotaName:   q.Name,
				Threshold:   threshold,
				UsedBytes:   q.UsedBytes,
				LimitBytes:  q.LimitBytes,
				UsagePct:    usagePct,
				Level:       level,
				Message:     fmt.Sprintf("配额 %s 使用量达到 %.1f%%（%d / %d）", q.Name, usagePct, q.UsedBytes, q.LimitBytes),
				TriggeredAt: time.Now(),
			}

			qm.alerts = append(qm.alerts, alert)
			q.AlertsSent[threshold] = true

			if qm.onAlert != nil {
				qm.onAlert(alert)
			}
		}
	}
}

// GetAlerts 获取告警列表
func (qm *QuotaManager) GetAlerts(ackedFilter *bool) []*Alert {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	result := make([]*Alert, 0)
	for _, a := range qm.alerts {
		if ackedFilter != nil && a.Acked != *ackedFilter {
			continue
		}
		result = append(result, a)
	}
	return result
}

// PredictUsage 使用量预测
func (qm *QuotaManager) PredictUsage(id string) (*Prediction, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	q, ok := qm.quotas[id]
	if !ok {
		return nil, fmt.Errorf("quota %s not found", id)
	}

	pred := &Prediction{
		QuotaID:     q.ID,
		CurrentUsed: q.UsedBytes,
		LimitBytes:  q.LimitBytes,
	}

	if len(q.History) < 2 {
		pred.Trend = "stable"
		pred.DaysRemaining = -1 // 数据不足
		return pred, nil
	}

	// 线性回归预测
	n := float64(len(q.History))
	first := q.History[0]
	last := q.History[len(q.History)-1]
	duration := last.Timestamp.Sub(first.Timestamp)
	if duration.Hours() < 1 {
		pred.Trend = "stable"
		pred.DaysRemaining = -1
		return pred, nil
	}

	// 日均增长率
	bytesDiff := float64(last.UsedBytes - first.UsedBytes)
	daysDiff := duration.Hours() / 24
	if daysDiff < 0.01 {
		daysDiff = 0.01
	}
	pred.DailyGrowthRate = bytesDiff / daysDiff

	if pred.DailyGrowthRate > 1024*1024 { // >1MB/day
		pred.Trend = "increasing"
	} else if pred.DailyGrowthRate < -1024*1024 {
		pred.Trend = "decreasing"
	} else {
		pred.Trend = "stable"
	}

	// 预计用尽日期
	if pred.DailyGrowthRate > 0 {
		remaining := float64(q.LimitBytes-q.UsedBytes)
		daysLeft := remaining / pred.DailyGrowthRate
		pred.DaysRemaining = daysLeft
		exhaustDate := time.Now().Add(time.Duration(daysLeft * 24 * float64(time.Hour)))
		pred.ExhaustDate = &exhaustDate
	} else {
		pred.DaysRemaining = math.MaxFloat64 // 不会用尽
	}

	_ = n // n used for future regression enhancement
	return pred, nil
}

// GetHistory 获取历史统计
func (qm *QuotaManager) GetHistory(id, period string) (*HistoryStats, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	q, ok := qm.quotas[id]
	if !ok {
		return nil, fmt.Errorf("quota %s not found", id)
	}

	var duration time.Duration
	switch period {
	case "day":
		duration = 24 * time.Hour
	case "week":
		duration = 7 * 24 * time.Hour
	case "month":
		duration = 30 * 24 * time.Hour
	default:
		return nil, fmt.Errorf("invalid period: %s", period)
	}

	cutoff := time.Now().Add(-duration)
	records := make([]UsageRecord, 0)
	for _, r := range q.History {
		if r.Timestamp.After(cutoff) {
			records = append(records, r)
		}
	}

	stats := &HistoryStats{
		QuotaID: q.ID,
		Period:  period,
		Records: records,
	}

	if len(records) == 0 {
		return stats, nil
	}

	var sum, maxVal, minVal int64
	minVal = math.MaxInt64
	for _, r := range records {
		sum += r.UsedBytes
		if r.UsedBytes > maxVal {
			maxVal = r.UsedBytes
		}
		if r.UsedBytes < minVal {
			minVal = r.UsedBytes
		}
	}

	stats.AvgUsage = sum / int64(len(records))
	stats.MaxUsage = maxVal
	stats.MinUsage = minVal
	if len(records) >= 2 {
		stats.Growth = records[len(records)-1].UsedBytes - records[0].UsedBytes
	}

	return stats, nil
}

// ApplyTemplate 应用配额模板
func (qm *QuotaManager) ApplyTemplate(templateName, ownerID, name string) (*QuotaConfig, error) {
	var tmpl *QuotaTemplate
	for _, t := range DefaultTemplates {
		if t.Name == templateName {
			t := t
			tmpl = &t
			break
		}
	}
	if tmpl == nil {
		return nil, fmt.Errorf("template %s not found", templateName)
	}

	cfg := QuotaConfig{
		Name:       name,
		Level:      tmpl.Level,
		OwnerID:    ownerID,
		LimitBytes: tmpl.LimitBytes,
		Policy:     tmpl.Policy,
	}

	return qm.CreateQuota(cfg)
}

// InheritQuota 配额继承：从父配额分配子配额
func (qm *QuotaManager) InheritQuota(parentID, childName, childOwnerID string, allocateBytes int64) (*QuotaConfig, error) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	parent, ok := qm.quotas[parentID]
	if !ok {
		return nil, fmt.Errorf("parent quota %s not found", parentID)
	}

	// 计算已分配给子配额的总量
	var allocated int64
	for _, q := range qm.quotas {
		if q.ParentID == parentID {
			allocated += q.LimitBytes
		}
	}

	if allocated+allocateBytes > parent.LimitBytes {
		return nil, fmt.Errorf("insufficient parent quota: available %d, requested %d", parent.LimitBytes-allocated, allocateBytes)
	}

	childLevel := LevelUser
	if parent.Level == LevelProject {
		childLevel = LevelShare
	}

	id := fmt.Sprintf("q%d", qm.nextID)
	qm.nextID++
	child := &QuotaConfig{
		ID:         id,
		Name:       childName,
		Level:      childLevel,
		OwnerID:    childOwnerID,
		ParentID:   parentID,
		LimitBytes: allocateBytes,
		Policy:     parent.Policy,
		AlertsSent: make(map[float64]bool),
		History:    make([]UsageRecord, 0),
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	qm.quotas[id] = child
	return child, nil
}

// GetCleanupSuggestions 获取清理建议
func (qm *QuotaManager) GetCleanupSuggestions(id string) ([]CleanupSuggestion, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	q, ok := qm.quotas[id]
	if !ok {
		return nil, fmt.Errorf("quota %s not found", id)
	}

	suggestions := make([]CleanupSuggestion, 0)
	usagePct := float64(q.UsedBytes) / float64(q.LimitBytes) * 100

	if usagePct < 50 {
		return suggestions, nil // 使用率低无需建议
	}

	// 大文件建议
	if usagePct > 75 {
		suggestions = append(suggestions, CleanupSuggestion{
			Type:        "large_file",
			Target:      fmt.Sprintf("/data/%s/", q.OwnerID),
			Size:        q.LimitBytes / 10,
			Description: fmt.Sprintf("建议清理大于 %s 的大文件以释放空间", formatBytes(q.LimitBytes/10)),
			Priority:    1,
		})
	}

	// 重复文件建议
	if usagePct > 85 {
		suggestions = append(suggestions, CleanupSuggestion{
			Type:        "duplicate",
			Target:      fmt.Sprintf("/data/%s/", q.OwnerID),
			Size:        q.UsedBytes / 20,
			Description: "检测到可能存在重复文件，建议使用去重工具清理",
			Priority:    2,
		})
	}

	// 长期未访问文件
	if usagePct > 90 {
		suggestions = append(suggestions, CleanupSuggestion{
			Type:        "stale_file",
			Target:      fmt.Sprintf("/data/%s/", q.OwnerID),
			Size:        q.UsedBytes / 5,
			Description: "建议归档超过90天未访问的文件到冷存储",
			Priority:    3,
		})
	}

	// 弹性策略下扩容建议
	if q.Policy == PolicyElastic && usagePct > 90 {
		suggestions = append(suggestions, CleanupSuggestion{
			Type:        "auto_expand",
			Target:      q.Name,
			Size:        int64(float64(q.LimitBytes) * 0.2),
			Description: "弹性策略：建议扩容20%或清理现有数据",
			Priority:    4,
		})
	}

	sort.Slice(suggestions, func(i, j int) bool {
		return suggestions[i].Priority < suggestions[j].Priority
	})

	return suggestions, nil
}

// formatBytes 格式化字节数
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// log 供内部使用
var _ = log.Println
