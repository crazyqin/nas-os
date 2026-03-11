// Package quota 提供存储配额管理功能
package quota

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// Manager 配额管理器
type Manager struct {
	mu            sync.RWMutex
	quotas        map[string]*Quota         // quotaID -> Quota
	groupQuotas   map[string]*Quota         // groupID -> Quota (用户组配额)
	userQuotas    map[string]map[string]*Quota // username -> volumeName -> Quota
	policies      map[string]*CleanupPolicy // policyID -> CleanupPolicy
	alerts        map[string]*Alert         // alertID -> Alert
	alertHistory  []*Alert                  // 历史告警
	configPath    string
	storageMgr    StorageProvider
	userProvider  UserProvider
	alertConfig   AlertConfig
	monitor       *Monitor
}

// StorageProvider 存储接口（用于获取卷使用情况）
type StorageProvider interface {
	GetVolume(name string) *VolumeInfo
	GetUsage(volumeName string) (total, used, free uint64, err error)
}

// VolumeInfo 卷信息
type VolumeInfo struct {
	Name       string
	MountPoint string
	Size       uint64
	Used       uint64
	Free       uint64
}

// UserProvider 用户接口（用于验证用户/组存在性）
type UserProvider interface {
	UserExists(username string) bool
	GroupExists(groupName string) bool
	GetUserHomeDir(username string) string
}

// NewManager 创建配额管理器
func NewManager(configPath string, storage StorageProvider, userProv UserProvider) (*Manager, error) {
	m := &Manager{
		quotas:       make(map[string]*Quota),
		groupQuotas:  make(map[string]*Quota),
		userQuotas:   make(map[string]map[string]*Quota),
		policies:     make(map[string]*CleanupPolicy),
		alerts:       make(map[string]*Alert),
		alertHistory: make([]*Alert, 0),
		configPath:   configPath,
		storageMgr:   storage,
		userProvider: userProv,
		alertConfig: AlertConfig{
			Enabled:            true,
			SoftLimitThreshold: 80,
			HardLimitThreshold: 95,
			CheckInterval:      5 * time.Minute,
			SilenceDuration:    1 * time.Hour,
		},
	}

	// 加载配置
	if configPath != "" {
		if err := m.loadConfig(); err != nil {
			return nil, fmt.Errorf("加载配额配置失败: %w", err)
		}
	}

	// 初始化监控器
	m.monitor = NewMonitor(m, m.alertConfig)

	return m, nil
}

// Start 启动配额管理（监控、清理等）
func (m *Manager) Start() {
	if m.monitor != nil {
		m.monitor.Start()
	}
}

// Stop 停止配额管理
func (m *Manager) Stop() {
	if m.monitor != nil {
		m.monitor.Stop()
	}
}

// ========== 配额管理 ==========

// CreateQuota 创建配额
func (m *Manager) CreateQuota(input QuotaInput) (*Quota, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 验证目标存在
	if input.Type == QuotaTypeUser {
		if m.userProvider != nil && !m.userProvider.UserExists(input.TargetID) {
			return nil, ErrUserNotFound
		}
	} else if input.Type == QuotaTypeGroup {
		if m.userProvider != nil && !m.userProvider.GroupExists(input.TargetID) {
			return nil, ErrGroupNotFound
		}
	}

	// 验证限制值
	if input.HardLimit == 0 {
		return nil, ErrInvalidLimit
	}
	if input.SoftLimit > input.HardLimit {
		input.SoftLimit = input.HardLimit
	}

	// 检查是否已存在
	for _, q := range m.quotas {
		if q.Type == input.Type && q.TargetID == input.TargetID && 
		   q.VolumeName == input.VolumeName && q.Path == input.Path {
			return nil, ErrQuotaExists
		}
	}

	quota := &Quota{
		ID:         generateID(),
		Type:       input.Type,
		TargetID:   input.TargetID,
		TargetName: input.TargetID,
		VolumeName: input.VolumeName,
		Path:       input.Path,
		HardLimit:  input.HardLimit,
		SoftLimit:  input.SoftLimit,
		CreatedAt:  time.Now(),
		UpdatedAt:  time.Now(),
	}

	m.quotas[quota.ID] = quota

	// 更新索引
	if input.Type == QuotaTypeUser {
		if m.userQuotas[input.TargetID] == nil {
			m.userQuotas[input.TargetID] = make(map[string]*Quota)
		}
		m.userQuotas[input.TargetID][input.VolumeName] = quota
	} else {
		m.groupQuotas[input.TargetID] = quota
	}

	m.saveConfig()
	return quota, nil
}

// GetQuota 获取配额
func (m *Manager) GetQuota(id string) (*Quota, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	quota, exists := m.quotas[id]
	if !exists {
		return nil, ErrQuotaNotFound
	}
	return quota, nil
}

// ListQuotas 列出所有配额
func (m *Manager) ListQuotas() []*Quota {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Quota, 0, len(m.quotas))
	for _, q := range m.quotas {
		result = append(result, q)
	}
	return result
}

// ListUserQuotas 列出用户的配额
func (m *Manager) ListUserQuotas(username string) []*Quota {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Quota, 0)
	for _, q := range m.quotas {
		if q.Type == QuotaTypeUser && q.TargetID == username {
			result = append(result, q)
		}
	}
	return result
}

// ListGroupQuotas 列出用户组的配额
func (m *Manager) ListGroupQuotas(groupName string) []*Quota {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Quota, 0)
	for _, q := range m.quotas {
		if q.Type == QuotaTypeGroup && q.TargetID == groupName {
			result = append(result, q)
		}
	}
	return result
}

// UpdateQuota 更新配额
func (m *Manager) UpdateQuota(id string, input QuotaInput) (*Quota, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	quota, exists := m.quotas[id]
	if !exists {
		return nil, ErrQuotaNotFound
	}

	// 验证限制值
	if input.HardLimit == 0 {
		return nil, ErrInvalidLimit
	}
	if input.SoftLimit > input.HardLimit {
		input.SoftLimit = input.HardLimit
	}

	quota.HardLimit = input.HardLimit
	quota.SoftLimit = input.SoftLimit
	quota.UpdatedAt = time.Now()

	m.saveConfig()
	return quota, nil
}

// DeleteQuota 删除配额
func (m *Manager) DeleteQuota(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	quota, exists := m.quotas[id]
	if !exists {
		return ErrQuotaNotFound
	}

	// 清理索引
	if quota.Type == QuotaTypeUser {
		if userQuotas, ok := m.userQuotas[quota.TargetID]; ok {
			delete(userQuotas, quota.VolumeName)
		}
	} else {
		delete(m.groupQuotas, quota.TargetID)
	}

	delete(m.quotas, id)
	m.saveConfig()
	return nil
}

// ========== 配额使用查询 ==========

// GetUsage 获取配额使用情况
func (m *Manager) GetUsage(quotaID string) (*QuotaUsage, error) {
	m.mu.RLock()
	quota, exists := m.quotas[quotaID]
	m.mu.RUnlock()

	if !exists {
		return nil, ErrQuotaNotFound
	}

	return m.calculateUsage(quota)
}

// GetAllUsage 获取所有配额使用情况
func (m *Manager) GetAllUsage() ([]*QuotaUsage, error) {
	m.mu.RLock()
	quotas := make([]*Quota, 0, len(m.quotas))
	for _, q := range m.quotas {
		quotas = append(quotas, q)
	}
	m.mu.RUnlock()

	result := make([]*QuotaUsage, 0, len(quotas))
	for _, q := range quotas {
		usage, err := m.calculateUsage(q)
		if err != nil {
			continue
		}
		result = append(result, usage)
	}
	return result, nil
}

// GetUserUsage 获取用户所有配额使用情况
func (m *Manager) GetUserUsage(username string) ([]*QuotaUsage, error) {
	m.mu.RLock()
	quotas := m.ListUserQuotas(username)
	m.mu.RUnlock()

	result := make([]*QuotaUsage, 0, len(quotas))
	for _, q := range quotas {
		usage, err := m.calculateUsage(q)
		if err != nil {
			continue
		}
		result = append(result, usage)
	}
	return result, nil
}

// calculateUsage 计算配额使用情况
func (m *Manager) calculateUsage(quota *Quota) (*QuotaUsage, error) {
	usage := &QuotaUsage{
		QuotaID:     quota.ID,
		Type:        quota.Type,
		TargetID:    quota.TargetID,
		TargetName:  quota.TargetName,
		VolumeName:  quota.VolumeName,
		Path:        quota.Path,
		HardLimit:   quota.HardLimit,
		SoftLimit:   quota.SoftLimit,
		LastChecked: time.Now(),
	}

	// 计算实际使用量
	usedBytes, err := m.calculatePathUsage(quota)
	if err != nil {
		usedBytes = 0
	}

	usage.UsedBytes = usedBytes

	// 计算可用空间和百分比
	if quota.HardLimit > usedBytes {
		usage.Available = quota.HardLimit - usedBytes
	} else {
		usage.Available = 0
	}

	if quota.HardLimit > 0 {
		usage.UsagePercent = float64(usedBytes) / float64(quota.HardLimit) * 100
	}

	// 检查是否超限
	usage.IsOverSoft = quota.SoftLimit > 0 && usedBytes > quota.SoftLimit
	usage.IsOverHard = usedBytes > quota.HardLimit

	return usage, nil
}

// calculatePathUsage 计算路径使用量
func (m *Manager) calculatePathUsage(quota *Quota) (uint64, error) {
	// 确定要检查的路径
	var targetPath string
	if quota.Path != "" {
		targetPath = quota.Path
	} else if m.storageMgr != nil {
		vol := m.storageMgr.GetVolume(quota.VolumeName)
		if vol == nil {
			return 0, ErrVolumeNotFound
		}
		// 使用用户主目录
		if m.userProvider != nil {
			targetPath = m.userProvider.GetUserHomeDir(quota.TargetID)
		}
	}

	if targetPath == "" {
		return 0, errors.New("无法确定配额路径")
	}

	// 使用 du 命令计算目录大小
	return m.getDirSize(targetPath)
}

// getDirSize 获取目录大小
func (m *Manager) getDirSize(path string) (uint64, error) {
	// 检查路径是否存在
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return 0, nil
	}

	// 使用 du 命令
	cmd := exec.Command("du", "-sb", path)
	output, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("计算目录大小失败: %w", err)
	}

	// 解析输出
	var size uint64
	fmt.Sscanf(string(output), "%d", &size)
	return size, nil
}

// ========== 告警管理 ==========

// GetAlerts 获取活跃告警
func (m *Manager) GetAlerts() []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*Alert, 0, len(m.alerts))
	for _, a := range m.alerts {
		if a.Status == AlertStatusActive {
			result = append(result, a)
		}
	}
	return result
}

// GetAlertHistory 获取告警历史
func (m *Manager) GetAlertHistory(limit int) []*Alert {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.alertHistory) {
		limit = len(m.alertHistory)
	}

	result := make([]*Alert, limit)
	copy(result, m.alertHistory[len(m.alertHistory)-limit:])
	return result
}

// SilenceAlert 静默告警
func (m *Manager) SilenceAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[alertID]
	if !exists {
		return errors.New("告警不存在")
	}

	alert.Status = AlertStatusSilenced
	m.saveConfig()
	return nil
}

// ResolveAlert 解决告警
func (m *Manager) ResolveAlert(alertID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	alert, exists := m.alerts[alertID]
	if !exists {
		return errors.New("告警不存在")
	}

	now := time.Now()
	alert.Status = AlertStatusResolved
	alert.ResolvedAt = &now

	// 移到历史记录
	m.alertHistory = append(m.alertHistory, alert)
	delete(m.alerts, alertID)

	m.saveConfig()
	return nil
}

// CreateAlert 创建告警（内部使用）
func (m *Manager) createAlert(quota *Quota, usage *QuotaUsage, alertType AlertType) *Alert {
	alert := &Alert{
		ID:           generateID(),
		QuotaID:      quota.ID,
		Type:         alertType,
		Status:       AlertStatusActive,
		TargetID:     quota.TargetID,
		TargetName:   quota.TargetName,
		VolumeName:   quota.VolumeName,
		Path:         quota.Path,
		UsedBytes:    usage.UsedBytes,
		LimitBytes:   usage.HardLimit,
		UsagePercent: usage.UsagePercent,
		CreatedAt:    time.Now(),
	}

	switch alertType {
	case AlertTypeSoftLimit:
		alert.Message = fmt.Sprintf("用户 %s 存储使用已达 %.1f%%，超过软限制", 
			quota.TargetName, usage.UsagePercent)
	case AlertTypeHardLimit:
		alert.Message = fmt.Sprintf("用户 %s 存储使用已达 %.1f%%，超过硬限制，写入可能被拒绝", 
			quota.TargetName, usage.UsagePercent)
	}

	m.alerts[alert.ID] = alert
	return alert
}

// SetAlertConfig 设置告警配置
func (m *Manager) SetAlertConfig(config AlertConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertConfig = config
	if m.monitor != nil {
		m.monitor.UpdateConfig(config)
	}
}

// GetAlertConfig 获取告警配置
func (m *Manager) GetAlertConfig() AlertConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.alertConfig
}

// ========== 持久化 ==========

type persistentQuotaConfig struct {
	Quotas       []*Quota         `json:"quotas"`
	Policies     []*CleanupPolicy `json:"policies"`
	Alerts       []*Alert         `json:"alerts"`
	AlertHistory []*Alert         `json:"alert_history"`
	AlertConfig  AlertConfig      `json:"alert_config"`
}

func (m *Manager) loadConfig() error {
	if m.configPath == "" {
		return nil
	}

	if _, err := os.Stat(m.configPath); os.IsNotExist(err) {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}

	var pc persistentQuotaConfig
	if err := json.Unmarshal(data, &pc); err != nil {
		return fmt.Errorf("解析配置文件失败: %w", err)
	}

	// 加载配额
	for _, q := range pc.Quotas {
		m.quotas[q.ID] = q
		if q.Type == QuotaTypeUser {
			if m.userQuotas[q.TargetID] == nil {
				m.userQuotas[q.TargetID] = make(map[string]*Quota)
			}
			m.userQuotas[q.TargetID][q.VolumeName] = q
		} else {
			m.groupQuotas[q.TargetID] = q
		}
	}

	// 加载清理策略
	for _, p := range pc.Policies {
		m.policies[p.ID] = p
	}

	// 加载告警
	for _, a := range pc.Alerts {
		m.alerts[a.ID] = a
	}
	m.alertHistory = pc.AlertHistory

	// 加载告警配置
	if pc.AlertConfig.CheckInterval > 0 {
		m.alertConfig = pc.AlertConfig
	}

	return nil
}

func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	pc := persistentQuotaConfig{
		Quotas:       make([]*Quota, 0, len(m.quotas)),
		Policies:     make([]*CleanupPolicy, 0, len(m.policies)),
		Alerts:       make([]*Alert, 0, len(m.alerts)),
		AlertHistory: m.alertHistory,
		AlertConfig:  m.alertConfig,
	}

	for _, q := range m.quotas {
		pc.Quotas = append(pc.Quotas, q)
	}
	for _, p := range m.policies {
		pc.Policies = append(pc.Policies, p)
	}
	for _, a := range m.alerts {
		pc.Alerts = append(pc.Alerts, a)
	}

	data, err := json.MarshalIndent(pc, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0755); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0600)
}

func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// CheckQuota 检查配额（用于写入前验证）
func (m *Manager) CheckQuota(username, volumeName string, additionalBytes uint64) error {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 查找用户配额
	var quota *Quota
	if userQuotas, ok := m.userQuotas[username]; ok {
		quota = userQuotas[volumeName]
	}

	if quota == nil {
		// 没有配额限制，允许写入
		return nil
	}

	// 计算当前使用量
	usage, err := m.calculateUsage(quota)
	if err != nil {
		return err
	}

	// 检查是否会超过硬限制
	if usage.UsedBytes+additionalBytes > quota.HardLimit {
		return ErrQuotaExceeded
	}

	return nil
}