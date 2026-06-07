package zfsquotamgr

import (
	"fmt"
	"sync"
	"time"
)

// Dataset ZFS数据集
type Dataset struct {
	Name         string    `json:"name"`
	MountPoint   string    `json:"mount_point"`
	Quota        int64     `json:"quota_bytes"`
	Used         int64     `json:"used_bytes"`
	Available    int64     `json:"available_bytes"`
	Reservation  int64     `json:"reservation_bytes"`
	Referenced   int64     `json:"referenced_bytes"`
	Compression  string    `json:"compression"`
	ReadOnly     bool      `json:"read_only"`
	CreateTime   time.Time `json:"create_time"`
	LastModified time.Time `json:"last_modified"`
}

// UserQuota 用户配额
type UserQuota struct {
	UserID    string    `json:"user_id"`
	Username  string    `json:"username"`
	Dataset   string    `json:"dataset"`
	Quota     int64     `json:"quota_bytes"`
	Used      int64     `json:"used_bytes"`
	Available int64     `json:"available_bytes"`
	SoftLimit int64     `json:"soft_limit_bytes"`
	HardLimit int64     `json:"hard_limit_bytes"`
	UpdatedAt time.Time `json:"updated_at"`
}

// GroupQuota 组配额
type GroupQuota struct {
	GroupID   string    `json:"group_id"`
	GroupName string    `json:"group_name"`
	Dataset   string    `json:"dataset"`
	Quota     int64     `json:"quota_bytes"`
	Used      int64     `json:"used_bytes"`
	Available int64     `json:"available_bytes"`
	Members   []string  `json:"members"`
	UpdatedAt time.Time `json:"updated_at"`
}

// QuotaRecommendation 配额推荐
type QuotaRecommendation struct {
	Target       string  `json:"target"`
	TargetType   string  `json:"target_type"`
	CurrentQuota int64   `json:"current_quota_bytes"`
	Recommended  int64   `json:"recommended_quota_bytes"`
	UsagePercent float64 `json:"usage_percent"`
	GrowthRate   float64 `json:"growth_rate_per_day"`
	Reason       string  `json:"reason"`
	Priority     string  `json:"priority"`
}

// QuotaAlert 配额告警
type QuotaAlert struct {
	ID           string     `json:"id"`
	Target       string     `json:"target"`
	TargetType   string     `json:"target_type"`
	Dataset      string     `json:"dataset"`
	Level        string     `json:"level"`
	Message      string     `json:"message"`
	UsedBytes    int64      `json:"used_bytes"`
	QuotaBytes   int64      `json:"quota_bytes"`
	UsagePercent float64    `json:"usage_percent"`
	CreatedAt    time.Time  `json:"created_at"`
	Resolved     bool       `json:"resolved"`
	ResolvedAt   *time.Time `json:"resolved_at,omitempty"`
}

// QuotaConfig 配额配置
type QuotaConfig struct {
	WarningThreshold  float64 `json:"warning_threshold"`
	CriticalThreshold float64 `json:"critical_threshold"`
	DefaultUserQuota  int64   `json:"default_user_quota_bytes"`
	DefaultGroupQuota int64   `json:"default_group_quota_bytes"`
	AlertEnabled      bool    `json:"alert_enabled"`
}

// QuotaStats 配额统计
type QuotaStats struct {
	TotalDatasets    int     `json:"total_datasets"`
	TotalUserQuotas  int     `json:"total_user_quotas"`
	TotalGroupQuotas int     `json:"total_group_quotas"`
	TotalQuota       int64   `json:"total_quota_bytes"`
	TotalUsed        int64   `json:"total_used_bytes"`
	AvgUsage         float64 `json:"avg_usage_percent"`
	OverQuota        int     `json:"over_quota_count"`
	NearQuota        int     `json:"near_quota_count"`
	ActiveAlerts     int     `json:"active_alerts"`
}

// ZFSQuotaManager ZFS配额管理器
type ZFSQuotaManager struct {
	mu          sync.RWMutex
	datasets    map[string]*Dataset
	userQuotas  map[string]*UserQuota
	groupQuotas map[string]*GroupQuota
	alerts      []*QuotaAlert
	config      *QuotaConfig
	dataPath    string
}

// NewManager 创建配额管理器
func NewManager(dataPath string) *ZFSQuotaManager {
	m := &ZFSQuotaManager{
		datasets:    make(map[string]*Dataset),
		userQuotas:  make(map[string]*UserQuota),
		groupQuotas: make(map[string]*GroupQuota),
		alerts:      make([]*QuotaAlert, 0),
		config: &QuotaConfig{
			WarningThreshold:  80.0,
			CriticalThreshold: 95.0,
			DefaultUserQuota:  100 * 1024 * 1024 * 1024,  // 100GB
			DefaultGroupQuota: 1024 * 1024 * 1024 * 1024, // 1TB
			AlertEnabled:      true,
		},
		dataPath: dataPath,
	}
	return m
}

// RegisterDataset 注册数据集
func (m *ZFSQuotaManager) RegisterDataset(ds *Dataset) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if ds.Name == "" {
		return fmt.Errorf("数据集名称不能为空")
	}
	ds.LastModified = time.Now()
	m.datasets[ds.Name] = ds
	m.checkDatasetQuota(ds)
	return nil
}

// UnregisterDataset 注销数据集
func (m *ZFSQuotaManager) UnregisterDataset(name string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, exists := m.datasets[name]; !exists {
		return fmt.Errorf("数据集不存在: %s", name)
	}
	delete(m.datasets, name)
	return nil
}

// GetDataset 获取数据集
func (m *ZFSQuotaManager) GetDataset(name string) (*Dataset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ds, exists := m.datasets[name]
	if !exists {
		return nil, fmt.Errorf("数据集不存在: %s", name)
	}
	return ds, nil
}

// ListDatasets 列出所有数据集
func (m *ZFSQuotaManager) ListDatasets() []*Dataset {
	m.mu.RLock()
	defer m.mu.RUnlock()
	datasets := make([]*Dataset, 0, len(m.datasets))
	for _, ds := range m.datasets {
		datasets = append(datasets, ds)
	}
	return datasets
}

// SetDatasetQuota 设置数据集配额
func (m *ZFSQuotaManager) SetDatasetQuota(name string, quota int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ds, exists := m.datasets[name]
	if !exists {
		return fmt.Errorf("数据集不存在: %s", name)
	}
	ds.Quota = quota
	ds.LastModified = time.Now()
	m.checkDatasetQuota(ds)
	return nil
}

// SetUserQuota 设置用户配额
func (m *ZFSQuotaManager) SetUserQuota(userID, username, dataset string, quota int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s", dataset, userID)
	m.userQuotas[key] = &UserQuota{
		UserID:    userID,
		Username:  username,
		Dataset:   dataset,
		Quota:     quota,
		UpdatedAt: time.Now(),
	}
	return nil
}

// GetUserQuota 获取用户配额
func (m *ZFSQuotaManager) GetUserQuota(userID, dataset string) (*UserQuota, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", dataset, userID)
	q, exists := m.userQuotas[key]
	if !exists {
		return nil, fmt.Errorf("用户配额不存在: %s/%s", dataset, userID)
	}
	return q, nil
}

// ListUserQuotas 列出用户配额
func (m *ZFSQuotaManager) ListUserQuotas(dataset string) []*UserQuota {
	m.mu.RLock()
	defer m.mu.RUnlock()
	quotas := make([]*UserQuota, 0)
	for _, q := range m.userQuotas {
		if dataset == "" || q.Dataset == dataset {
			quotas = append(quotas, q)
		}
	}
	return quotas
}

// SetGroupQuota 设置组配额
func (m *ZFSQuotaManager) SetGroupQuota(groupID, groupName, dataset string, quota int64, members []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	key := fmt.Sprintf("%s:%s", dataset, groupID)
	m.groupQuotas[key] = &GroupQuota{
		GroupID:   groupID,
		GroupName: groupName,
		Dataset:   dataset,
		Quota:     quota,
		Members:   members,
		UpdatedAt: time.Now(),
	}
	return nil
}

// GetGroupQuota 获取组配额
func (m *ZFSQuotaManager) GetGroupQuota(groupID, dataset string) (*GroupQuota, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	key := fmt.Sprintf("%s:%s", dataset, groupID)
	q, exists := m.groupQuotas[key]
	if !exists {
		return nil, fmt.Errorf("组配额不存在: %s/%s", dataset, groupID)
	}
	return q, nil
}

// ListGroupQuotas 列出组配额
func (m *ZFSQuotaManager) ListGroupQuotas(dataset string) []*GroupQuota {
	m.mu.RLock()
	defer m.mu.RUnlock()
	quotas := make([]*GroupQuota, 0)
	for _, q := range m.groupQuotas {
		if dataset == "" || q.Dataset == dataset {
			quotas = append(quotas, q)
		}
	}
	return quotas
}

// UpdateUsage 更新使用量
func (m *ZFSQuotaManager) UpdateUsage(dataset string, used int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	ds, exists := m.datasets[dataset]
	if !exists {
		return fmt.Errorf("数据集不存在: %s", dataset)
	}
	ds.Used = used
	if ds.Quota > 0 {
		ds.Available = ds.Quota - used
	}
	ds.LastModified = time.Now()
	m.checkDatasetQuota(ds)
	return nil
}

// GenerateRecommendations 生成配额推荐
func (m *ZFSQuotaManager) GenerateRecommendations() []*QuotaRecommendation {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var recs []*QuotaRecommendation
	for _, ds := range m.datasets {
		if ds.Quota == 0 {
			continue
		}
		usagePercent := float64(ds.Used) / float64(ds.Quota) * 100
		if usagePercent >= m.config.WarningThreshold {
			recommended := int64(float64(ds.Quota) * 1.5)
			priority := "medium"
			if usagePercent >= m.config.CriticalThreshold {
				priority = "high"
				recommended = int64(float64(ds.Quota) * 2.0)
			}
			recs = append(recs, &QuotaRecommendation{
				Target:       ds.Name,
				TargetType:   "dataset",
				CurrentQuota: ds.Quota,
				Recommended:  recommended,
				UsagePercent: usagePercent,
				GrowthRate:   0,
				Reason:       fmt.Sprintf("使用率 %.1f%% 超过阈值 %.1f%%", usagePercent, m.config.WarningThreshold),
				Priority:     priority,
			})
		}
	}
	return recs
}

// GetStats 获取统计
func (m *ZFSQuotaManager) GetStats() *QuotaStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &QuotaStats{
		TotalDatasets:    len(m.datasets),
		TotalUserQuotas:  len(m.userQuotas),
		TotalGroupQuotas: len(m.groupQuotas),
	}

	totalUsage := 0.0
	quotaCount := 0
	for _, ds := range m.datasets {
		stats.TotalQuota += ds.Quota
		stats.TotalUsed += ds.Used
		if ds.Quota > 0 {
			usage := float64(ds.Used) / float64(ds.Quota) * 100
			totalUsage += usage
			quotaCount++
			if usage >= 100 {
				stats.OverQuota++
			} else if usage >= m.config.WarningThreshold {
				stats.NearQuota++
			}
		}
	}
	if quotaCount > 0 {
		stats.AvgUsage = totalUsage / float64(quotaCount)
	}

	for _, a := range m.alerts {
		if !a.Resolved {
			stats.ActiveAlerts++
		}
	}
	return stats
}

// GetAlerts 获取告警
func (m *ZFSQuotaManager) GetAlerts(resolved bool) []*QuotaAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()
	alerts := make([]*QuotaAlert, 0)
	for _, a := range m.alerts {
		if a.Resolved == resolved {
			alerts = append(alerts, a)
		}
	}
	return alerts
}

// ResolveAlert 解决告警
func (m *ZFSQuotaManager) ResolveAlert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, a := range m.alerts {
		if a.ID == id && !a.Resolved {
			a.Resolved = true
			now := time.Now()
			a.ResolvedAt = &now
			return nil
		}
	}
	return fmt.Errorf("告警不存在或已解决: %s", id)
}

// UpdateConfig 更新配置
func (m *ZFSQuotaManager) UpdateConfig(cfg *QuotaConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
}

// GetConfig 获取配置
func (m *ZFSQuotaManager) GetConfig() *QuotaConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.config
}

func (m *ZFSQuotaManager) checkDatasetQuota(ds *Dataset) {
	if ds.Quota <= 0 || !m.config.AlertEnabled {
		return
	}
	usagePercent := float64(ds.Used) / float64(ds.Quota) * 100

	if usagePercent >= m.config.CriticalThreshold {
		m.addAlert(ds.Name, "dataset", ds.Name, "critical",
			fmt.Sprintf("数据集 %s 使用率 %.1f%% 超过临界阈值 %.1f%%", ds.Name, usagePercent, m.config.CriticalThreshold),
			ds.Used, ds.Quota, usagePercent)
	} else if usagePercent >= m.config.WarningThreshold {
		m.addAlert(ds.Name, "dataset", ds.Name, "warning",
			fmt.Sprintf("数据集 %s 使用率 %.1f%% 超过警告阈值 %.1f%%", ds.Name, usagePercent, m.config.WarningThreshold),
			ds.Used, ds.Quota, usagePercent)
	}
}

func (m *ZFSQuotaManager) addAlert(target, targetType, dataset, level, message string, used, quota int64, usagePercent float64) {
	for _, a := range m.alerts {
		if a.Target == target && a.Message == message && !a.Resolved {
			return
		}
	}
	m.alerts = append(m.alerts, &QuotaAlert{
		ID:           fmt.Sprintf("quota-alert-%d", time.Now().UnixNano()),
		Target:       target,
		TargetType:   targetType,
		Dataset:      dataset,
		Level:        level,
		Message:      message,
		UsedBytes:    used,
		QuotaBytes:   quota,
		UsagePercent: usagePercent,
		CreatedAt:    time.Now(),
	})
}
