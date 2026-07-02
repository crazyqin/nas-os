// Package smartquota 实现智能配额管理功能
// 对标群晖 DSM 7.3 Storage Manager 配额管理
// 支持用户/团队配额、动态调整、告警通知
package smartquota

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// QuotaType 配额类型.
type QuotaType string

const (
	QuotaTypeUser  QuotaType = "user"  // 用户配额
	QuotaTypeTeam  QuotaType = "team"  // 团队配额
	QuotaTypeShare QuotaType = "share" // 共享文件夹配额
)

// QuotaStatus 配额状态.
type QuotaStatus string

const (
	StatusNormal   QuotaStatus = "normal"   // 正常
	StatusWarning  QuotaStatus = "warning"  // 警告
	StatusCritical QuotaStatus = "critical" // 严重
	StatusExceeded QuotaStatus = "exceeded" // 超出
)

// AlertLevel 告警级别.
type AlertLevel string

const (
	AlertLevelInfo     AlertLevel = "info"
	AlertLevelWarning  AlertLevel = "warning"
	AlertLevelCritical AlertLevel = "critical"
)

// Quota 配额定义.
type Quota struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Type        QuotaType   `json:"type"`
	TargetID    string      `json:"target_id"` // 用户ID/团队ID/共享文件夹ID
	TargetName  string      `json:"target_name"`
	MaxBytes    int64       `json:"max_bytes"`  // 最大容量（字节）
	UsedBytes   int64       `json:"used_bytes"` // 已使用（字节）
	MaxFiles    int64       `json:"max_files"`  // 最大文件数
	UsedFiles   int64       `json:"used_files"` // 已使用文件数
	Status      QuotaStatus `json:"status"`
	WarnPercent int         `json:"warn_percent"` // 告警阈值百分比
	CritPercent int         `json:"crit_percent"` // 严重阈值百分比
	Enabled     bool        `json:"enabled"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
	TenantID    string      `json:"tenant_id"`
}

// QuotaAlert 配额告警.
type QuotaAlert struct {
	ID        string     `json:"id"`
	QuotaID   string     `json:"quota_id"`
	QuotaName string     `json:"quota_name"`
	Level     AlertLevel `json:"level"`
	Message   string     `json:"message"`
	Percent   int        `json:"percent"`
	CreatedAt time.Time  `json:"created_at"`
	Acked     bool       `json:"acked"`
	AckedBy   string     `json:"acked_by,omitempty"`
	AckedAt   *time.Time `json:"acked_at,omitempty"`
}

// QuotaStats 配额统计.
type QuotaStats struct {
	TotalQuotas   int            `json:"total_quotas"`
	ActiveQuotas  int            `json:"active_quotas"`
	TotalCapacity int64          `json:"total_capacity"`
	TotalUsed     int64          `json:"total_used"`
	UsagePercent  float64        `json:"usage_percent"`
	AlertsCount   int            `json:"alerts_count"`
	ByType        map[string]int `json:"by_type"`
	ByStatus      map[string]int `json:"by_status"`
}

// Manager 智能配额管理器.
type Manager struct {
	mu          sync.RWMutex
	quotas      map[string]*Quota
	alerts      []*QuotaAlert
	storagePath string
}

// NewManager 创建配额管理器.
func NewManager(storagePath string) *Manager {
	return &Manager{
		quotas:      make(map[string]*Quota),
		alerts:      make([]*QuotaAlert, 0),
		storagePath: storagePath,
	}
}

// CreateQuota 创建配额.
func (m *Manager) CreateQuota(ctx context.Context, quota Quota) (*Quota, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if quota.ID == "" {
		quota.ID = fmt.Sprintf("quota_%d", time.Now().UnixNano())
	}
	if quota.WarnPercent == 0 {
		quota.WarnPercent = 80
	}
	if quota.CritPercent == 0 {
		quota.CritPercent = 95
	}
	quota.Status = StatusNormal
	quota.Enabled = true
	quota.CreatedAt = time.Now()
	quota.UpdatedAt = time.Now()

	m.quotas[quota.ID] = &quota
	return &quota, nil
}

// GetQuota 获取配额.
func (m *Manager) GetQuota(ctx context.Context, quotaID string) (*Quota, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quota, exists := m.quotas[quotaID]
	if !exists {
		return nil, fmt.Errorf("配额不存在: %s", quotaID)
	}
	return quota, nil
}

// ListQuotas 列出配额.
func (m *Manager) ListQuotas(ctx context.Context, quotaType QuotaType, tenantID string) ([]*Quota, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*Quota
	for _, quota := range m.quotas {
		if quotaType != "" && quota.Type != quotaType {
			continue
		}
		if tenantID != "" && quota.TenantID != tenantID {
			continue
		}
		result = append(result, quota)
	}
	return result, nil
}

// UpdateUsage 更新使用量.
func (m *Manager) UpdateUsage(ctx context.Context, quotaID string, usedBytes, usedFiles int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	quota, exists := m.quotas[quotaID]
	if !exists {
		return fmt.Errorf("配额不存在: %s", quotaID)
	}

	quota.UsedBytes = usedBytes
	quota.UsedFiles = usedFiles
	quota.UpdatedAt = time.Now()

	// 计算状态
	percent := int(float64(usedBytes) / float64(quota.MaxBytes) * 100)
	if percent >= 100 {
		quota.Status = StatusExceeded
		m.addAlert(quota, AlertLevelCritical, "配额已超出", percent)
	} else if percent >= quota.CritPercent {
		quota.Status = StatusCritical
		m.addAlert(quota, AlertLevelCritical, "配额即将用尽", percent)
	} else if percent >= quota.WarnPercent {
		quota.Status = StatusWarning
		m.addAlert(quota, AlertLevelWarning, "配额使用率较高", percent)
	} else {
		quota.Status = StatusNormal
	}

	return nil
}

func (m *Manager) addAlert(quota *Quota, level AlertLevel, message string, percent int) {
	alert := &QuotaAlert{
		ID:        fmt.Sprintf("alert_%d", time.Now().UnixNano()),
		QuotaID:   quota.ID,
		QuotaName: quota.Name,
		Level:     level,
		Message:   message,
		Percent:   percent,
		CreatedAt: time.Now(),
	}
	m.alerts = append(m.alerts, alert)
}

// GetAlerts 获取告警列表.
func (m *Manager) GetAlerts(ctx context.Context, unackedOnly bool) []*QuotaAlert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []*QuotaAlert
	for _, alert := range m.alerts {
		if unackedOnly && alert.Acked {
			continue
		}
		result = append(result, alert)
	}
	return result
}

// AckAlert 确认告警.
func (m *Manager) AckAlert(ctx context.Context, alertID, ackedBy string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, alert := range m.alerts {
		if alert.ID == alertID {
			alert.Acked = true
			alert.AckedBy = ackedBy
			now := time.Now()
			alert.AckedAt = &now
			return nil
		}
	}
	return fmt.Errorf("告警不存在: %s", alertID)
}

// DeleteQuota 删除配额.
func (m *Manager) DeleteQuota(ctx context.Context, quotaID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, exists := m.quotas[quotaID]; !exists {
		return fmt.Errorf("配额不存在: %s", quotaID)
	}

	delete(m.quotas, quotaID)
	return nil
}

// GetStats 获取配额统计.
func (m *Manager) GetStats(ctx context.Context, tenantID string) (*QuotaStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &QuotaStats{
		ByType:   make(map[string]int),
		ByStatus: make(map[string]int),
	}

	for _, quota := range m.quotas {
		if tenantID != "" && quota.TenantID != tenantID {
			continue
		}
		stats.TotalQuotas++
		if quota.Enabled {
			stats.ActiveQuotas++
		}
		stats.TotalCapacity += quota.MaxBytes
		stats.TotalUsed += quota.UsedBytes
		stats.ByType[string(quota.Type)]++
		stats.ByStatus[string(quota.Status)]++
	}

	if stats.TotalCapacity > 0 {
		stats.UsagePercent = float64(stats.TotalUsed) / float64(stats.TotalCapacity) * 100
	}

	// 统计未确认告警
	for _, alert := range m.alerts {
		if !alert.Acked {
			stats.AlertsCount++
		}
	}

	return stats, nil
}
