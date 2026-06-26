package snapshotmgr

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// TeamSnapshotPolicy 团队文件夹的快照策略
// 参考群晖 Snapshot Replication 的共享文件夹快照策略，
// 为团队协作场景提供统一的快照保护和可见性管理。
type TeamSnapshotPolicy struct {
	ID         string `json:"id"`
	TeamID     string `json:"team_id"`     // 团队 ID
	FolderPath string `json:"folder_path"` // 团队文件夹路径
	PolicyName string `json:"policy_name"` // 策略名称
	Enabled    bool   `json:"enabled"`     // 是否启用
	Recursive  bool   `json:"recursive"`   // 递归包含子目录
	AutoCreate bool   `json:"auto_create"` // 自动创建快照
	CronExpr   string `json:"cron_expr"`   // 定时表达式（如 "0 */4 * * *" 每4小时）

	// 保留策略（简化版，复用 RetentionPolicy 的粒度概念）
	RetainHourly  int `json:"retain_hourly"`
	RetainDaily   int `json:"retain_daily"`
	RetainWeekly  int `json:"retain_weekly"`
	RetainMonthly int `json:"retain_monthly"`

	// 可见性设置
	Visibility TeamSnapshotVisibility `json:"visibility"`

	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// TeamSnapshotVisibility 跨用户快照可见性管理
type TeamSnapshotVisibility struct {
	// OwnerVisible 团队所有者是否可见
	OwnerVisible bool `json:"owner_visible"`
	// MemberVisible 团队成员是否可见
	MemberVisible bool `json:"member_visible"`
	// GuestVisible 访客是否可见
	GuestVisible bool `json:"guest_visible"`
	// AdminVisible 管理员是否始终可见（不受以上限制）
	AdminVisible bool `json:"admin_visible"`
	// RestoreOwnerOnly 仅所有者可恢复
	RestoreOwnerOnly bool `json:"restore_owner_only"`
	// DeleteAdminOnly 仅管理员可删除
	DeleteAdminOnly bool `json:"delete_admin_only"`
}

// TeamSnapshot 团队快照记录
type TeamSnapshot struct {
	SnapshotID string    `json:"snapshot_id"` // 关联的系统快照 ID
	TeamID     string    `json:"team_id"`     // 团队 ID
	FolderPath string    `json:"folder_path"` // 团队文件夹路径
	CreatedBy  string    `json:"created_by"`  // 创建者用户 ID
	Source     string    `json:"source"`      // manual / scheduled / auto
	SizeBytes  int64     `json:"size_bytes"`  // 快照大小
	Locked     bool      `json:"locked"`      // 是否锁定（防止删除）
	CreatedAt  time.Time `json:"created_at"`
}

// TeamSnapshotManager 团队快照管理器
type TeamSnapshotManager struct {
	mu        sync.RWMutex
	logger    *zap.Logger
	manager   *Manager
	policies  map[string]*TeamSnapshotPolicy // key: policy ID
	snapshots map[string]*TeamSnapshot       // key: snapshot ID
	// visibilityCache 用户->可见快照缓存
	visibilityCache map[string][]string // key: userID, value: snapshot IDs
}

// NewTeamSnapshotManager 创建团队快照管理器
func NewTeamSnapshotManager(logger *zap.Logger, manager *Manager) *TeamSnapshotManager {
	if logger == nil {
		logger = zap.NewNop()
	}
	return &TeamSnapshotManager{
		logger:          logger,
		manager:         manager,
		policies:        make(map[string]*TeamSnapshotPolicy),
		snapshots:       make(map[string]*TeamSnapshot),
		visibilityCache: make(map[string][]string),
	}
}

// CreatePolicy 创建团队快照策略
func (t *TeamSnapshotManager) CreatePolicy(policy *TeamSnapshotPolicy) (*TeamSnapshotPolicy, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate id: %w", err)
	}

	policy.ID = id
	policy.CreatedAt = time.Now()
	policy.UpdatedAt = time.Now()

	// 默认可见性
	if !policy.Visibility.OwnerVisible && !policy.Visibility.MemberVisible {
		policy.Visibility.OwnerVisible = true
		policy.Visibility.MemberVisible = true
	}
	policy.Visibility.AdminVisible = true // 管理员始终可见

	t.policies[id] = policy

	t.logger.Info("team snapshot policy created",
		zap.String("policy_id", id),
		zap.String("team_id", policy.TeamID),
		zap.String("folder_path", policy.FolderPath),
	)

	return policy, nil
}

// GetPolicy 获取团队快照策略
func (t *TeamSnapshotManager) GetPolicy(id string) (*TeamSnapshotPolicy, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	p, ok := t.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}
	return p, nil
}

// ListPoliciesByTeam 列出团队的所有策略
func (t *TeamSnapshotManager) ListPoliciesByTeam(teamID string) []TeamSnapshotPolicy {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []TeamSnapshotPolicy
	for _, p := range t.policies {
		if p.TeamID == teamID {
			result = append(result, *p)
		}
	}
	return result
}

// UpdatePolicy 更新团队快照策略
func (t *TeamSnapshotManager) UpdatePolicy(id string, updates *TeamSnapshotPolicy) (*TeamSnapshotPolicy, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	p, ok := t.policies[id]
	if !ok {
		return nil, fmt.Errorf("policy %s not found", id)
	}

	// 选择性更新
	if updates.PolicyName != "" {
		p.PolicyName = updates.PolicyName
	}
	if updates.CronExpr != "" {
		p.CronExpr = updates.CronExpr
	}
	p.Enabled = updates.Enabled
	p.Recursive = updates.Recursive
	p.AutoCreate = updates.AutoCreate
	p.RetainHourly = updates.RetainHourly
	p.RetainDaily = updates.RetainDaily
	p.RetainWeekly = updates.RetainWeekly
	p.RetainMonthly = updates.RetainMonthly
	p.Visibility = updates.Visibility
	p.UpdatedAt = time.Now()

	t.policies[id] = p

	t.logger.Info("team snapshot policy updated", zap.String("policy_id", id))
	return p, nil
}

// DeletePolicy 删除团队快照策略
func (t *TeamSnapshotManager) DeletePolicy(id string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	if _, ok := t.policies[id]; !ok {
		return fmt.Errorf("policy %s not found", id)
	}

	delete(t.policies, id)
	t.logger.Info("team snapshot policy deleted", zap.String("policy_id", id))
	return nil
}

// CreateTeamSnapshot 创建团队快照
func (t *TeamSnapshotManager) CreateTeamSnapshot(teamID, folderPath, createdBy, source string) (*TeamSnapshot, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 通过底层 Manager 创建系统快照
	items := []SnapshotItem{
		{
			Category: "team",
			Key:      folderPath,
			FilePath: folderPath,
		},
	}

	snap, err := t.manager.CreateSnapshot(
		fmt.Sprintf("team-%s-%s", teamID, folderPath),
		fmt.Sprintf("Team snapshot by %s", createdBy),
		source,
		items,
	)
	if err != nil {
		return nil, fmt.Errorf("create system snapshot: %w", err)
	}

	teamSnap := &TeamSnapshot{
		SnapshotID: snap.ID,
		TeamID:     teamID,
		FolderPath: folderPath,
		CreatedBy:  createdBy,
		Source:     source,
		SizeBytes:  snap.SizeBytes,
		CreatedAt:  snap.CreatedAt,
	}

	t.snapshots[snap.ID] = teamSnap

	// 刷新可见性缓存
	t.invalidateVisibilityCache(teamID)

	t.logger.Info("team snapshot created",
		zap.String("snapshot_id", snap.ID),
		zap.String("team_id", teamID),
		zap.String("created_by", createdBy),
	)

	return teamSnap, nil
}

// ListTeamSnapshots 列出团队可见快照
func (t *TeamSnapshotManager) ListTeamSnapshots(teamID, userID, userRole string) []TeamSnapshot {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var result []TeamSnapshot
	for _, ts := range t.snapshots {
		if ts.TeamID != teamID {
			continue
		}
		if t.canView(ts, userID, userRole) {
			result = append(result, *ts)
		}
	}
	return result
}

// RestoreTeamSnapshot 恢复团队快照
func (t *TeamSnapshotManager) RestoreTeamSnapshot(snapshotID, userID, userRole string) error {
	t.mu.RLock()
	ts, ok := t.snapshots[snapshotID]
	if !ok {
		t.mu.RUnlock()
		return fmt.Errorf("team snapshot %s not found", snapshotID)
	}

	// 检查恢复权限
	policies := t.getPoliciesForFolder(ts.TeamID, ts.FolderPath)
	var canRestore bool
	for _, p := range policies {
		if p.Visibility.RestoreOwnerOnly {
			canRestore = ts.CreatedBy == userID || userRole == "admin"
		} else {
			canRestore = userRole == "admin" || userRole == "owner" || userRole == "member"
		}
		if canRestore {
			break
		}
	}
	if !canRestore && len(policies) > 0 {
		t.mu.RUnlock()
		return fmt.Errorf("user %s not authorized to restore snapshot %s", userID, snapshotID)
	}

	t.mu.RUnlock()

	_, err := t.manager.RestoreSnapshot(snapshotID)
	if err != nil {
		return fmt.Errorf("restore snapshot: %w", err)
	}

	t.logger.Info("team snapshot restored",
		zap.String("snapshot_id", snapshotID),
		zap.String("user_id", userID),
	)
	return nil
}

// DeleteTeamSnapshot 删除团队快照
func (t *TeamSnapshotManager) DeleteTeamSnapshot(snapshotID, userID, userRole string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	ts, ok := t.snapshots[snapshotID]
	if !ok {
		return fmt.Errorf("team snapshot %s not found", snapshotID)
	}

	if ts.Locked {
		return fmt.Errorf("team snapshot %s is locked and cannot be deleted", snapshotID)
	}

	// 检查删除权限
	policies := t.getPoliciesForFolderLocked(ts.TeamID, ts.FolderPath)
	var canDelete bool
	for _, p := range policies {
		if p.Visibility.DeleteAdminOnly {
			canDelete = userRole == "admin"
		} else {
			canDelete = userRole == "admin" || userRole == "owner"
		}
		if canDelete {
			break
		}
	}
	if !canDelete && len(policies) > 0 {
		return fmt.Errorf("user %s not authorized to delete snapshot %s", userID, snapshotID)
	}

	if err := t.manager.DeleteSnapshot(snapshotID); err != nil {
		return fmt.Errorf("delete snapshot: %w", err)
	}

	delete(t.snapshots, snapshotID)
	t.invalidateVisibilityCache(ts.TeamID)

	t.logger.Info("team snapshot deleted",
		zap.String("snapshot_id", snapshotID),
		zap.String("user_id", userID),
	)
	return nil
}

// canView 检查用户是否可以查看快照
func (t *TeamSnapshotManager) canView(ts *TeamSnapshot, userID, userRole string) bool {
	policies := t.getPoliciesForFolder(ts.TeamID, ts.FolderPath)

	// 无策略时默认所有团队成员可见
	if len(policies) == 0 {
		return true
	}

	for _, p := range policies {
		vis := p.Visibility
		if userRole == "admin" && vis.AdminVisible {
			return true
		}
		if userRole == "owner" && vis.OwnerVisible {
			return true
		}
		if userRole == "member" && vis.MemberVisible {
			return true
		}
		if userRole == "guest" && vis.GuestVisible {
			return true
		}
	}

	return false
}

// getPoliciesForFolder 获取文件夹的策略（RLock 已持有）
func (t *TeamSnapshotManager) getPoliciesForFolder(teamID, folderPath string) []*TeamSnapshotPolicy {
	t.mu.RLock()
	defer t.mu.RUnlock()

	return t.getPoliciesForFolderLocked(teamID, folderPath)
}

// getPoliciesForFolderLocked 获取文件夹的策略（调用方已持有锁）
func (t *TeamSnapshotManager) getPoliciesForFolderLocked(teamID, folderPath string) []*TeamSnapshotPolicy {
	var result []*TeamSnapshotPolicy
	for _, p := range t.policies {
		if p.TeamID == teamID && p.FolderPath == folderPath {
			result = append(result, p)
		}
	}
	return result
}

// invalidateVisibilityCache 使可见性缓存失效
func (t *TeamSnapshotManager) invalidateVisibilityCache(teamID string) {
	// 简单实现：清空整个缓存
	t.visibilityCache = make(map[string][]string)
}

// LockSnapshot 锁定团队快照
func (t *TeamSnapshotManager) LockSnapshot(snapshotID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	ts, ok := t.snapshots[snapshotID]
	if !ok {
		return fmt.Errorf("team snapshot %s not found", snapshotID)
	}
	ts.Locked = true
	t.logger.Info("team snapshot locked", zap.String("snapshot_id", snapshotID))
	return nil
}

// UnlockSnapshot 解锁团队快照
func (t *TeamSnapshotManager) UnlockSnapshot(snapshotID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	ts, ok := t.snapshots[snapshotID]
	if !ok {
		return fmt.Errorf("team snapshot %s not found", snapshotID)
	}
	ts.Locked = false
	t.logger.Info("team snapshot unlocked", zap.String("snapshot_id", snapshotID))
	return nil
}
