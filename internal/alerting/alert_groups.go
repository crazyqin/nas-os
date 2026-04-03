// Package alerting 提供增强告警系统
// 对标群晖 Active Insight 告警分组功能
package alerting

import (
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"
)

// ========== 告警分组定义 ==========

// AlertGroup 告警分组
type AlertGroup struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description"`
	Category    AlertCategory     `json:"category"`    // 存储/网络/系统/安全
	Priority    int               `json:"priority"`    // 显示优先级（1-10）
	Enabled     bool              `json:"enabled"`
	Rules       []string          `json:"rules"`       // 关联的规则 ID
	Labels      map[string]string `json:"labels"`      // 分组标签
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// AlertCategory 告警分类（对标群晖 Active Insight）
type AlertCategory string

const (
	// CategoryStorage 存储类告警：磁盘空间、磁盘健康、RAID状态、IOPS异常
	CategoryStorage AlertCategory = "storage"
	// CategoryNetwork 网络类告警：网络连接、带宽异常、丢包、延迟
	CategoryNetwork AlertCategory = "network"
	// CategorySystem 系统类告警：CPU、内存、进程、服务状态
	CategorySystem AlertCategory = "system"
	// CategorySecurity 安全类告警：入侵检测、异常登录、权限变更
	CategorySecurity AlertCategory = "security"
)

// AlertGroupManager 告警分组管理器
type AlertGroupManager struct {
	mu     sync.RWMutex
	groups map[string]*AlertGroup
	stats  map[AlertCategory]*GroupStats
}

// GroupStats 分组统计
type GroupStats struct {
	Category        AlertCategory `json:"category"`
	TotalAlerts     int           `json:"total_alerts"`
	ActiveAlerts    int           `json:"active_alerts"`
	CriticalCount   int           `json:"critical_count"`
	WarningCount    int           `json:"warning_count"`
	InfoCount       int           `json:"info_count"`
	LastAlertTime   *time.Time    `json:"last_alert_time,omitempty"`
	AcknowledgedPct float64       `json:"acknowledged_pct"` // 确认率
}

// NewAlertGroupManager 创建告警分组管理器
func NewAlertGroupManager() *AlertGroupManager {
	mgr := &AlertGroupManager{
		groups: make(map[string]*AlertGroup),
		stats:  make(map[AlertCategory]*GroupStats),
	}

	// 初始化分组统计
	for _, cat := range []AlertCategory{CategoryStorage, CategoryNetwork, CategorySystem, CategorySecurity} {
		mgr.stats[cat] = &GroupStats{Category: cat}
	}

	// 添加默认分组（对标群晖 Active Insight）
	mgr.initDefaultGroups()

	return mgr
}

// initDefaultGroups 初始化默认告警分组
func (m *AlertGroupManager) initDefaultGroups() {
	defaultGroups := []*AlertGroup{
		// 存储类分组
		{
			ID:          "storage-disk-space",
			Name:        "磁盘空间",
			Description: "存储卷空间使用率告警",
			Category:    CategoryStorage,
			Priority:    1,
			Enabled:     true,
			Rules:       []string{"disk-space-warning", "disk-space-critical"},
			Labels:      map[string]string{"impact": "high", "auto_resolve": "true"},
			CreatedAt:   time.Now(),
		},
		{
			ID:          "storage-disk-health",
			Name:        "磁盘健康",
			Description: "磁盘SMART状态、健康评分告警",
			Category:    CategoryStorage,
			Priority:    2,
			Enabled:     true,
			Rules:       []string{"disk-health-warning", "disk-health-critical"},
			Labels:      map[string]string{"impact": "critical", "auto_resolve": "false"},
			CreatedAt:   time.Now(),
		},
		{
			ID:          "storage-raid",
			Name:        "RAID状态",
			Description: "RAID阵列状态、重建进度告警",
			Category:    CategoryStorage,
			Priority:    3,
			Enabled:     true,
			Rules:       []string{},
			Labels:      map[string]string{"impact": "critical", "auto_resolve": "false"},
			CreatedAt:   time.Now(),
		},
		{
			ID:          "storage-io",
			Name:        "存储IO",
			Description: "IOPS异常、读写延迟告警",
			Category:    CategoryStorage,
			Priority:    4,
			Enabled:     true,
			Rules:       []string{},
			Labels:      map[string]string{"impact": "medium", "auto_resolve": "true"},
			CreatedAt:   time.Now(),
		},

		// 网络类分组
		{
			ID:          "network-connection",
			Name:        "网络连接",
			Description: "网络断连、接口状态告警",
			Category:    CategoryNetwork,
			Priority:    1,
			Enabled:     true,
			Rules:       []string{},
			Labels:      map[string]string{"impact": "high", "auto_resolve": "true"},
			CreatedAt:   time.Now(),
		},
		{
			ID:          "network-bandwidth",
			Name:        "带宽异常",
			Description: "带宽超限、流量异常告警",
			Category:    CategoryNetwork,
			Priority:    2,
			Enabled:     true,
			Rules:       []string{},
			Labels:      map[string]string{"impact": "medium", "auto_resolve": "true"},
			CreatedAt:   time.Now(),
		},
		{
			ID:          "network-latency",
			Name:        "网络延迟",
			Description: "丢包率、延迟异常告警",
			Category:    CategoryNetwork,
			Priority:    3,
			Enabled:     true,
			Rules:       []string{},
			Labels:      map[string]string{"impact": "medium", "auto_resolve": "true"},
			CreatedAt:   time.Now(),
		},

		// 系统类分组
		{
			ID:          "system-cpu",
			Name:        "CPU资源",
			Description: "CPU使用率、负载异常告警",
			Category:    CategorySystem,
			Priority:    1,
			Enabled:     true,
			Rules:       []string{"cpu-warning", "cpu-critical"},
			Labels:      map[string]string{"impact": "medium", "auto_resolve": "true"},
			CreatedAt:   time.Now(),
		},
		{
			ID:          "system-memory",
			Name:        "内存资源",
			Description: "内存使用率、OOM告警",
			Category:    CategorySystem,
			Priority:    2,
			Enabled:     true,
			Rules:       []string{"memory-warning", "memory-critical"},
			Labels:      map[string]string{"impact": "high", "auto_resolve": "true"},
			CreatedAt:   time.Now(),
		},
		{
			ID:          "system-temperature",
			Name:        "温度异常",
			Description: "设备温度、风扇状态告警",
			Category:    CategorySystem,
			Priority:    3,
			Enabled:     true,
			Rules:       []string{"temperature-warning", "temperature-critical"},
			Labels:      map[string]string{"impact": "high", "auto_resolve": "true"},
			CreatedAt:   time.Now(),
		},
		{
			ID:          "system-service",
			Name:        "服务状态",
			Description: "核心服务运行状态告警",
			Category:    CategorySystem,
			Priority:    4,
			Enabled:     true,
			Rules:       []string{},
			Labels:      map[string]string{"impact": "critical", "auto_resolve": "false"},
			CreatedAt:   time.Now(),
		},

		// 安全类分组
		{
			ID:          "security-intrusion",
			Name:        "入侵检测",
			Description: "异常访问、攻击行为告警",
			Category:    CategorySecurity,
			Priority:    1,
			Enabled:     true,
			Rules:       []string{},
			Labels:      map[string]string{"impact": "critical", "auto_resolve": "false"},
			CreatedAt:   time.Now(),
		},
		{
			ID:          "security-auth",
			Name:        "认证异常",
			Description: "异常登录、暴力破解告警",
			Category:    CategorySecurity,
			Priority:    2,
			Enabled:     true,
			Rules:       []string{},
			Labels:      map[string]string{"impact": "high", "auto_resolve": "false"},
			CreatedAt:   time.Now(),
		},
		{
			ID:          "security-permission",
			Name:        "权限变更",
			Description: "权限修改、配置变更告警",
			Category:    CategorySecurity,
			Priority:    3,
			Enabled:     true,
			Rules:       []string{},
			Labels:      map[string]string{"impact": "medium", "auto_resolve": "false"},
			CreatedAt:   time.Now(),
		},
		{
			ID:          "security-malware",
			Name:        "恶意软件",
			Description: "病毒检测、可疑文件告警",
			Category:    CategorySecurity,
			Priority:    4,
			Enabled:     true,
			Rules:       []string{},
			Labels:      map[string]string{"impact": "critical", "auto_resolve": "false"},
			CreatedAt:   time.Now(),
		},
	}

	for _, g := range defaultGroups {
		g.UpdatedAt = g.CreatedAt
		m.groups[g.ID] = g
	}
}

// ========== 分组管理方法 ==========

// CreateGroup 创建告警分组
func (m *AlertGroupManager) CreateGroup(group AlertGroup) (*AlertGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if group.ID == "" {
		group.ID = generateGroupID(group.Category)
	}

	if _, exists := m.groups[group.ID]; exists {
		return nil, fmt.Errorf("分组已存在: %s", group.ID)
	}

	group.CreatedAt = time.Now()
	group.UpdatedAt = time.Now()
	m.groups[group.ID] = &group

	return &group, nil
}

// GetGroup 获取告警分组
func (m *AlertGroupManager) GetGroup(id string) (*AlertGroup, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	group, exists := m.groups[id]
	if !exists {
		return nil, fmt.Errorf("分组不存在: %s", id)
	}

	return group, nil
}

// UpdateGroup 更新告警分组
func (m *AlertGroupManager) UpdateGroup(id string, group AlertGroup) (*AlertGroup, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	existing, exists := m.groups[id]
	if !exists {
		return nil, fmt.Errorf("分组不存在: %s", id)
	}

	group.ID = id
	group.CreatedAt = existing.CreatedAt
	group.UpdatedAt = time.Now()
	m.groups[id] = &group

	return &group, nil
}

// DeleteGroup 删除告警分组
func (m *AlertGroupManager) DeleteGroup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.groups[id]; !exists {
		return fmt.Errorf("分组不存在: %s", id)
	}

	delete(m.groups, id)
	return nil
}

// ListGroups 列出所有告警分组
func (m *AlertGroupManager) ListGroups() []*AlertGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AlertGroup, 0, len(m.groups))
	for _, g := range m.groups {
		result = append(result, g)
	}

	// 按优先级排序
	sortGroupsByPriority(result)

	return result
}

// ListGroupsByCategory 按分类列出分组
func (m *AlertGroupManager) ListGroupsByCategory(category AlertCategory) []*AlertGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AlertGroup, 0)
	for _, g := range m.groups {
		if g.Category == category {
			result = append(result, g)
		}
	}

	sortGroupsByPriority(result)
	return result
}

// EnableGroup 启用分组
func (m *AlertGroupManager) EnableGroup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, exists := m.groups[id]
	if !exists {
		return fmt.Errorf("分组不存在: %s", id)
	}

	group.Enabled = true
	group.UpdatedAt = time.Now()

	return nil
}

// DisableGroup 禁用分组
func (m *AlertGroupManager) DisableGroup(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, exists := m.groups[id]
	if !exists {
		return fmt.Errorf("分组不存在: %s", id)
	}

	group.Enabled = false
	group.UpdatedAt = time.Now()

	return nil
}

// AddRuleToGroup 向分组添加规则
func (m *AlertGroupManager) AddRuleToGroup(groupID, ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, exists := m.groups[groupID]
	if !exists {
		return fmt.Errorf("分组不存在: %s", groupID)
	}

	// 检查规则是否已存在
	for _, r := range group.Rules {
		if r == ruleID {
			return nil // 已存在，不重复添加
		}
	}

	group.Rules = append(group.Rules, ruleID)
	group.UpdatedAt = time.Now()

	return nil
}

// RemoveRuleFromGroup 从分组移除规则
func (m *AlertGroupManager) RemoveRuleFromGroup(groupID, ruleID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	group, exists := m.groups[groupID]
	if !exists {
		return fmt.Errorf("分组不存在: %s", groupID)
	}

	for i, r := range group.Rules {
		if r == ruleID {
			group.Rules = append(group.Rules[:i], group.Rules[i+1:]...)
			group.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("规则不在分组中: %s", ruleID)
}

// GetGroupsByRuleID 获取包含指定规则的分组
func (m *AlertGroupManager) GetGroupsByRuleID(ruleID string) []*AlertGroup {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*AlertGroup, 0)
	for _, g := range m.groups {
		for _, r := range g.Rules {
			if r == ruleID {
				result = append(result, g)
				break
			}
		}
	}

	return result
}

// ========== 统计方法 ==========

// UpdateStats 更新分组统计
func (m *AlertGroupManager) UpdateStats(category AlertCategory, alerts []GroupAlertInfo) {
	m.mu.Lock()
	defer m.mu.Unlock()

	stats := m.stats[category]
	if stats == nil {
		stats = &GroupStats{Category: category}
		m.stats[category] = stats
	}

	stats.TotalAlerts = len(alerts)
	stats.ActiveAlerts = 0
	stats.CriticalCount = 0
	stats.WarningCount = 0
	stats.InfoCount = 0
	acknowledged := 0

	for _, a := range alerts {
		if !a.Resolved {
			stats.ActiveAlerts++
		}
		switch a.Level {
		case AlertLevelCritical:
			stats.CriticalCount++
		case AlertLevelWarning:
			stats.WarningCount++
		case AlertLevelInfo:
			stats.InfoCount++
		}
		if a.Acknowledged {
			acknowledged++
		}
		if a.Timestamp != nil && (stats.LastAlertTime == nil || a.Timestamp.After(*stats.LastAlertTime)) {
			stats.LastAlertTime = a.Timestamp
		}
	}

	if stats.TotalAlerts > 0 {
		stats.AcknowledgedPct = float64(acknowledged) / float64(stats.TotalAlerts) * 100
	} else {
		stats.AcknowledgedPct = 0
	}
}

// GetStats 获取分组统计
func (m *AlertGroupManager) GetStats(category AlertCategory) *GroupStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return m.stats[category]
}

// GetAllStats 获取所有分组统计
func (m *AlertGroupManager) GetAllStats() map[AlertCategory]*GroupStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[AlertCategory]*GroupStats)
	for cat, stats := range m.stats {
		result[cat] = stats
	}

	return result
}

// GetCategorySummary 获取分类概览（对标群晖 Active Insight Dashboard）
func (m *AlertGroupManager) GetCategorySummary() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := map[string]interface{}{
		"categories": make([]map[string]interface{}, 0),
		"total_active":    0,
		"total_critical":  0,
		"health_score":    100, // 健康评分（100为完美）
	}

	categories := summary["categories"].([]map[string]interface{})

	totalActive := 0
	totalCritical := 0
	healthPenalty := 0

	// 分类优先级顺序（展示顺序）
	order := []AlertCategory{CategoryStorage, CategorySystem, CategoryNetwork, CategorySecurity}

	for _, cat := range order {
		stats := m.stats[cat]
		if stats == nil {
			continue
		}

		catInfo := map[string]interface{}{
			"category":        cat,
			"name":            getCategoryName(cat),
			"icon":            getCategoryIcon(cat),
			"active_alerts":   stats.ActiveAlerts,
			"critical_count":  stats.CriticalCount,
			"warning_count":   stats.WarningCount,
			"info_count":      stats.InfoCount,
			"health_status":   getHealthStatus(stats),
			"last_alert_time": stats.LastAlertTime,
		}

		categories = append(categories, catInfo)
		totalActive += stats.ActiveAlerts
		totalCritical += stats.CriticalCount

		// 计算健康扣分
		healthPenalty += stats.CriticalCount * 15 // 每个Critical扣15分
		healthPenalty += stats.WarningCount * 5   // 每个Warning扣5分
	}

	summary["categories"] = categories
	summary["total_active"] = totalActive
	summary["total_critical"] = totalCritical

	// 计算健康评分（最低0分）
	healthScore := 100 - healthPenalty
	if healthScore < 0 {
		healthScore = 0
	}
	summary["health_score"] = healthScore

	return summary
}

// ========== 持久化 ==========

// SaveGroups 保存分组配置
func (m *AlertGroupManager) SaveGroups(path string) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	data := struct {
		Groups []*AlertGroup `json:"groups"`
	}{
		Groups: make([]*AlertGroup, 0, len(m.groups)),
	}

	for _, g := range m.groups {
		data.Groups = append(data.Groups, g)
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(path, jsonData, 0640)
}

// LoadGroups 加载分组配置
func (m *AlertGroupManager) LoadGroups(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	var loaded struct {
		Groups []*AlertGroup `json:"groups"`
	}

	if err := json.Unmarshal(data, &loaded); err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	for _, g := range loaded.Groups {
		m.groups[g.ID] = g
	}

	return nil
}

// ========== 辅助类型和方法 ==========

// GroupAlertInfo 分组告警信息（用于统计）
type GroupAlertInfo struct {
	ID           string     `json:"id"`
	Level        AlertLevel `json:"level"`
	Resolved     bool       `json:"resolved"`
	Acknowledged bool       `json:"acknowledged"`
	Timestamp    *time.Time `json:"timestamp,omitempty"`
}

// AlertLevel 告警级别
type AlertLevel string

const (
	// AlertLevelCritical 严重级别：需要立即处理
	AlertLevelCritical AlertLevel = "critical"
	// AlertLevelWarning 警告级别：需要关注
	AlertLevelWarning AlertLevel = "warning"
	// AlertLevelInfo 信息级别：仅供参考
	AlertLevelInfo AlertLevel = "info"
	// AlertLevelEmergency 紧急级别：最高优先级
	AlertLevelEmergency AlertLevel = "emergency"
)

// AlertLevelPriority 告警级别优先级映射
var AlertLevelPriority = map[AlertLevel]int{
	AlertLevelEmergency: 4,
	AlertLevelCritical:  3,
	AlertLevelWarning:   2,
	AlertLevelInfo:      1,
}

// generateGroupID 生成分组ID
func generateGroupID(category AlertCategory) string {
	return fmt.Sprintf("group-%s-%d", category, time.Now().UnixNano())
}

// sortGroupsByPriority 按优先级排序分组
func sortGroupsByPriority(groups []*AlertGroup) {
	for i := 0; i < len(groups)-1; i++ {
		for j := i + 1; j < len(groups); j++ {
			if groups[j].Priority < groups[i].Priority {
				groups[i], groups[j] = groups[j], groups[i]
			}
		}
	}
}

// getCategoryName 获取分类名称
func getCategoryName(category AlertCategory) string {
	names := map[AlertCategory]string{
		CategoryStorage: "存储",
		CategoryNetwork: "网络",
		CategorySystem:  "系统",
		CategorySecurity: "安全",
	}
	return names[category]
}

// getCategoryIcon 获取分类图标
func getCategoryIcon(category AlertCategory) string {
	icons := map[AlertCategory]string{
		CategoryStorage:  "storage",
		CategoryNetwork:  "network",
		CategorySystem:   "system",
		CategorySecurity: "security",
	}
	return icons[category]
}

// getHealthStatus 获取健康状态
func getHealthStatus(stats *GroupStats) string {
	if stats.CriticalCount > 0 {
		return "critical"
	}
	if stats.WarningCount > 0 {
		return "warning"
	}
	if stats.InfoCount > 0 {
		return "info"
	}
	return "healthy"
}