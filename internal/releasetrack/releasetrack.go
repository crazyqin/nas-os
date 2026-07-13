// Package releasetrack 提供版本通道管理功能，
// 对标群晖 DSM Beta/Stable 通道和 TrueNAS Stable/Nightlies 通道。
// 支持多通道发布、灰度推送、版本回滚和通道间迁移。
// 吏部开发。
package releasetrack

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// Channel 发布通道.
type Channel string

const (
	ChannelStable   Channel = "stable"
	ChannelBeta     Channel = "beta"
	ChannelCanary   Channel = "canary"
	ChannelNightly  Channel = "nightly"
	ChannelLTS      Channel = "lts"
)

// ChannelConfig 通道配置.
type ChannelConfig struct {
	Name           Channel `json:"name"`
	DisplayName    string  `json:"display_name"`
	Description    string  `json:"description"`
	AutoUpdate     bool    `json:"auto_update"`
	BatchSize      int     `json:"batch_size"`       // 灰度推送批次大小百分比
	BatchDelayMin  int     `json:"batch_delay_min"`   // 批次间隔分钟
	RollbackOnErr  bool    `json:"rollback_on_error"`
	ErrorThreshold float64 `json:"error_threshold"`   // 错误率阈值
}

// Release 发布版本.
type Release struct {
	Version      string    `json:"version"`
	Channel      Channel   `json:"channel"`
	ReleaseNotes string    `json:"release_notes"`
	CreatedAt    time.Time `json:"created_at"`
	PublishedAt  *time.Time `json:"published_at,omitempty"`
	RolledBack   bool      `json:"rolled_back"`
	RollbackNote string    `json:"rollback_note,omitempty"`
	Checksum     string    `json:"checksum"`
	SizeBytes    int64     `json:"size_bytes"`
	Tags         []string  `json:"tags"`
}

// Deployment 部署记录.
type Deployment struct {
	ID           string    `json:"id"`
	ReleaseVer   string    `json:"release_version"`
	Channel      Channel   `json:"channel"`
	BatchPercent int       `json:"batch_percent"`
	StartedAt    time.Time `json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	Status       string    `json:"status"` // pending, deploying, completed, failed, rolled_back
	TargetCount  int       `json:"target_count"`
	DoneCount    int       `json:"done_count"`
	ErrorCount   int       `json:"error_count"`
}

// RollbackPlan 回滚计划.
type RollbackPlan struct {
	FromVersion string  `json:"from_version"`
	ToVersion   string  `json:"to_version"`
	Reason      string  `json:"reason"`
	CreatedAt   time.Time `json:"created_at"`
	Channel     Channel  `json:"channel"`
}

// Manager 发布通道管理器.
type Manager struct {
	mu         sync.RWMutex
	channels   map[Channel]*ChannelConfig
	releases   map[Channel][]*Release
	deployments []Deployment
	rollbacks  []RollbackPlan
	currentRelease map[Channel]string
}

// NewManager 创建管理器.
func NewManager() *Manager {
	m := &Manager{
		channels:       make(map[Channel]*ChannelConfig),
		releases:        make(map[Channel][]*Release),
		currentRelease:  make(map[Channel]string),
	}
	// 默认通道
	m.channels[ChannelStable] = &ChannelConfig{
		Name: ChannelStable, DisplayName: "稳定版",
		Description: "经过充分测试的稳定版本", AutoUpdate: true,
		BatchSize: 100, RollbackOnErr: true, ErrorThreshold: 0.01,
	}
	m.channels[ChannelBeta] = &ChannelConfig{
		Name: ChannelBeta, DisplayName: "公测版",
		Description: "功能完整但可能有边缘问题", AutoUpdate: false,
		BatchSize: 25, RollbackOnErr: true, ErrorThreshold: 0.05,
	}
	m.channels[ChannelCanary] = &ChannelConfig{
		Name: ChannelCanary, DisplayName: "金丝雀",
		Description: "小范围验证版本", AutoUpdate: false,
		BatchSize: 5, RollbackOnErr: true, ErrorThreshold: 0.1,
	}
	m.channels[ChannelNightly] = &ChannelConfig{
		Name: ChannelNightly, DisplayName: "每日构建",
		Description: "开发者构建，不稳定", AutoUpdate: false,
		BatchSize: 100, RollbackOnErr: false, ErrorThreshold: 0.2,
	}
	m.channels[ChannelLTS] = &ChannelConfig{
		Name: ChannelLTS, DisplayName: "长期支持",
		Description: "长期稳定支持版本", AutoUpdate: true,
		BatchSize: 50, RollbackOnErr: true, ErrorThreshold: 0.005,
	}
	return m
}

// PublishRelease 发布版本.
func (m *Manager) PublishRelease(r *Release) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if r.Version == "" {
		return fmt.Errorf("version cannot be empty")
	}
	if _, ok := m.channels[r.Channel]; !ok {
		return fmt.Errorf("unknown channel: %s", r.Channel)
	}
	r.CreatedAt = time.Now()
	m.releases[r.Channel] = append(m.releases[r.Channel], r)
	return nil
}

// GetCurrentRelease 获取当前版本.
func (m *Manager) GetCurrentRelease(ch Channel) (*Release, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	ver, ok := m.currentRelease[ch]
	if !ok {
		return nil, fmt.Errorf("no current release for channel %s", ch)
	}
	releases := m.releases[ch]
	for _, r := range releases {
		if r.Version == ver {
			return r, nil
		}
	}
	return nil, fmt.Errorf("release %s not found", ver)
}

// SetCurrentRelease 设置当前版本.
func (m *Manager) SetCurrentRelease(ch Channel, version string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.currentRelease[ch] = version
	return nil
}

// PromoteRelease 通道间迁移.
func (m *Manager) PromoteRelease(version string, fromCh, toCh Channel) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	releases := m.releases[fromCh]
	var target *Release
	for _, r := range releases {
		if r.Version == version {
			target = r
			break
		}
	}
	if target == nil {
		return fmt.Errorf("release %s not found in channel %s", version, fromCh)
	}
	if _, ok := m.channels[toCh]; !ok {
		return fmt.Errorf("target channel %s not configured", toCh)
	}
	promoted := &Release{
		Version: target.Version, Channel: toCh,
		ReleaseNotes: target.ReleaseNotes, CreatedAt: time.Now(),
		Checksum: target.Checksum, SizeBytes: target.SizeBytes,
		Tags: append(target.Tags, fmt.Sprintf("promoted-from-%s", fromCh)),
	}
	m.releases[toCh] = append(m.releases[toCh], promoted)
	return nil
}

// StartDeployment 开始灰度部署.
func (m *Manager) StartDeployment(version string, ch Channel, targetCount int) (*Deployment, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cfg, ok := m.channels[ch]
	if !ok {
		return nil, fmt.Errorf("channel %s not configured", ch)
	}
	dep := &Deployment{
		ID:           fmt.Sprintf("deploy-%d", time.Now().UnixMilli()),
		ReleaseVer:   version, Channel: ch,
		BatchPercent: cfg.BatchSize,
		StartedAt:    time.Now(), Status: "deploying",
		TargetCount:  targetCount,
	}
	m.deployments = append(m.deployments, *dep)
	return dep, nil
}

// CompleteDeployment 完成部署.
func (m *Manager) CompleteDeployment(id string, doneCount, errorCount int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := range m.deployments {
		if m.deployments[i].ID == id {
			m.deployments[i].DoneCount = doneCount
			m.deployments[i].ErrorCount = errorCount
			m.deployments[i].Status = "completed"
			now := time.Now()
			m.deployments[i].CompletedAt = &now
			return nil
		}
	}
	return fmt.Errorf("deployment %s not found", id)
}

// CreateRollbackPlan 创建回滚计划.
func (m *Manager) CreateRollbackPlan(fromVer, toVer string, ch Channel, reason string) *RollbackPlan {
	m.mu.Lock()
	defer m.mu.Unlock()
	plan := &RollbackPlan{
		FromVersion: fromVer, ToVersion: toVer,
		Reason: reason, Channel: ch, CreatedAt: time.Now(),
	}
	m.rollbacks = append(m.rollbacks, *plan)
	return plan
}

// RollbackRelease 回滚版本.
func (m *Manager) RollbackRelease(version string, ch Channel, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	releases := m.releases[ch]
	for _, r := range releases {
		if r.Version == version {
			r.RolledBack = true
			r.RollbackNote = reason
			m.currentRelease[ch] = ""
			return nil
		}
	}
	return fmt.Errorf("release %s not found in channel %s", version, ch)
}

// ListReleases 列出版本.
func (m *Manager) ListReleases(ch Channel) []*Release {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*Release, 0)
	if releases, ok := m.releases[ch]; ok {
		result = append(result, releases...)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].CreatedAt.After(result[j].CreatedAt) })
	return result
}

// ListChannels 列出通道.
func (m *Manager) ListChannels() []*ChannelConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*ChannelConfig, 0, len(m.channels))
	for _, c := range m.channels {
		result = append(result, c)
	}
	sort.Slice(result, func(i, j int) bool { return result[i].Name < result[j].Name })
	return result
}

// ListDeployments 列出部署.
func (m *Manager) ListDeployments() []Deployment {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]Deployment, len(m.deployments))
	copy(result, m.deployments)
	return result
}