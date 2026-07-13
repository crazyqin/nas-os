// Package teamworkspace 团队工作区管理器
// 对标 Synology Drive 团队空间和飞牛共享空间，支持团队空间创建/配置、
// 成员权限管理、空间配额、协作文件夹、外部用户邀请和工作区健康度评估。
package teamworkspace

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"
)

// ========== 权限常量 ==========

const (
	PermissionViewer  = "viewer"   // 只读
	PermissionEditor  = "editor"   // 读写
	PermissionManager = "manager"  // 管理（不含删除工作区）
	PermissionAdmin   = "admin"    // 完全管理
	PermissionOwner   = "owner"    // 所有者

	// 成员操作
	ActionAdd    = "add"
	ActionRemove = "remove"
	ActionUpdate = "update"

	// 健康状态
	HealthExcellent = "excellent"
	HealthGood      = "good"
	HealthFair      = "fair"
	HealthWarning   = "warning"
	HealthCritical  = "critical"

	// 邀请状态
	InviteStatusPending  = "pending"
	InviteStatusAccepted = "accepted"
	InviteStatusExpired  = "expired"
)

var validPermissions = map[string]bool{
	PermissionViewer:  true,
	PermissionEditor:  true,
	PermissionManager: true,
	PermissionAdmin:   true,
	PermissionOwner:   true,
}

var validActions = map[string]bool{
	ActionAdd:    true,
	ActionRemove: true,
	ActionUpdate: true,
}

// ========== 核心结构体 ==========

// CreateWorkspaceOptions 创建工作区选项.
type CreateWorkspaceOptions struct {
	Name              string   // 工作区名称
	Description       string   // 描述
	OwnerID           string   // 所有者用户ID
	DefaultPermission string   // 默认成员权限
	QuotaGB           float64  // 配额(GB)
	Tags              []string // 标签
}

// Workspace 工作区信息.
type Workspace struct {
	ID              string   // 工作区唯一ID
	Name            string   // 名称
	Description     string   // 描述
	OwnerID         string   // 所有者ID
	MemberCount     int      // 成员数量
	QuotaGB         float64  // 配额(GB)
	UsedGB          float64  // 已用(GB)
	CreatedAt       int64    // 创建时间(Unix)
	Health          string   // 健康状态
	Tags            []string // 标签
	DefaultPermission string // 默认权限
}

// MemberChange 成员变更.
type MemberChange struct {
	UserID     string // 用户ID
	Action     string // add/remove/update
	Permission string // 权限
}

// MemberUpdateResult 成员更新结果.
type MemberUpdateResult struct {
	WorkspaceID string   // 工作区ID
	Added       []string // 成功添加的
	Removed     []string // 成功移除的
	Updated     []string // 成功更新的
	Errors      []string // 错误信息
}

// QuotaResult 配额设置结果.
type QuotaResult struct {
	WorkspaceID      string  // 工作区ID
	QuotaGB          float64 // 新配额
	PreviousQuotaGB  float64 // 旧配额
	Effective        bool    // 是否生效
	Warning         string  // 警告信息
}

// ExternalInvite 外部用户邀请.
type ExternalInvite struct {
	Email      string // 邀请邮箱
	Permission string // 权限
	ExpiresIn  int    // 过期小时数(0=默认72h)
	WorkspaceID string // 工作区ID
}

// InviteResult 邀请结果.
type InviteResult struct {
	InviteID    string // 邀请ID
	Email       string // 邮箱
	Status      string // 状态
	ExpiresAt   int64  // 过期时间(Unix)
	InviteLink  string // 邀请链接
}

// WorkspaceHealth 工作区健康度.
type WorkspaceHealth struct {
	WorkspaceID      string   // 工作区ID
	Score            float64  // 健康分(0-100)
	Issues           []string // 问题列表
	Recommendations  []string // 建议
	ActiveMembers    int      // 活跃成员数
	StorageTrend     string   // 存储趋势
}

// WorkspaceFilter 工作区过滤.
type WorkspaceFilter struct {
	OwnerID    string // 按所有者
	NameLike   string // 名称模糊匹配
	MinMembers int    // 最小成员数
}

// ========== 内部结构体 ==========

// memberInfo 内部成员信息.
type memberInfo struct {
	UserID     string
	Permission string
	JoinedAt   int64
	LastActive int64 // 最后活跃时间(Unix)
}

// inviteInfo 内部邀请信息.
type inviteInfo struct {
	InviteID    string
	Email       string
	Permission  string
	WorkspaceID string
	Status      string
	ExpiresAt   int64
}

// ========== WorkspaceManager ==========

// WorkspaceManager 团队工作区管理器.
type WorkspaceManager struct {
	mu         sync.RWMutex
	workspaces map[string]*Workspace          // workspaceID -> Workspace
	members    map[string]map[string]*memberInfo // workspaceID -> userID -> memberInfo
	invites    map[string]*inviteInfo            // inviteID -> inviteInfo
	tags       map[string]map[string]bool        // workspaceID -> tag set
	usedGB     map[string]float64                // workspaceID -> used GB (独立追踪)
}

// NewManager 创建工作区管理器.
func NewManager() *WorkspaceManager {
	return &WorkspaceManager{
		workspaces: make(map[string]*Workspace),
		members:    make(map[string]map[string]*memberInfo),
		invites:    make(map[string]*inviteInfo),
		tags:       make(map[string]map[string]bool),
		usedGB:     make(map[string]float64),
	}
}

// ========== 辅助函数 ==========

// generateID 生成随机ID.
func generateID(prefix string) string {
	b := make([]byte, 12)
	_, _ = rand.Read(b)
	return prefix + "_" + hex.EncodeToString(b)
}

// sanitizePermission 规范化权限.
func sanitizePermission(perm string) string {
	perm = strings.ToLower(strings.TrimSpace(perm))
	if !validPermissions[perm] {
		return PermissionEditor // 默认editor
	}
	return perm
}

// sanitizeAction 规范化操作.
func sanitizeAction(action string) string {
	action = strings.ToLower(strings.TrimSpace(action))
	if !validActions[action] {
		return ""
	}
	return action
}

// nowUnix 当前Unix时间戳.
func nowUnix() int64 {
	return time.Now().Unix()
}

// ========== CreateWorkspace ==========

// CreateWorkspace 创建团队工作区.
func (m *WorkspaceManager) CreateWorkspace(opts CreateWorkspaceOptions) (*Workspace, error) {
	if strings.TrimSpace(opts.Name) == "" {
		return nil, fmt.Errorf("workspace name is required")
	}
	if strings.TrimSpace(opts.OwnerID) == "" {
		return nil, fmt.Errorf("owner ID is required")
	}

	perm := sanitizePermission(opts.DefaultPermission)
	if opts.DefaultPermission == "" {
		perm = PermissionEditor
	}

	if opts.QuotaGB < 0 {
		opts.QuotaGB = 0
	}

	wsID := generateID("ws")
	now := nowUnix()

	// 去重标签
	tagSet := make(map[string]bool)
	for _, t := range opts.Tags {
		t = strings.TrimSpace(t)
		if t != "" {
			tagSet[t] = true
		}
	}
	tags := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tags = append(tags, t)
	}
	sort.Strings(tags)

	ws := &Workspace{
		ID:                wsID,
		Name:              strings.TrimSpace(opts.Name),
		Description:       strings.TrimSpace(opts.Description),
		OwnerID:           opts.OwnerID,
		MemberCount:       1, // 所有者自动加入
		QuotaGB:           opts.QuotaGB,
		UsedGB:            0,
		CreatedAt:         now,
		Health:            HealthGood,
		Tags:              tags,
		DefaultPermission: perm,
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.workspaces[wsID] = ws
	m.members[wsID] = make(map[string]*memberInfo)
	m.members[wsID][opts.OwnerID] = &memberInfo{
		UserID:     opts.OwnerID,
		Permission: PermissionOwner,
		JoinedAt:   now,
		LastActive: now,
	}
	m.usedGB[wsID] = 0
	m.tags[wsID] = tagSet

	return ws, nil
}

// ========== ManageMembers ==========

// ManageMembers 管理工作区成员.
func (m *WorkspaceManager) ManageMembers(workspaceID string, changes []MemberChange) (*MemberUpdateResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ws, ok := m.workspaces[workspaceID]
	if !ok {
		return nil, fmt.Errorf("workspace %s not found", workspaceID)
	}

	result := &MemberUpdateResult{
		WorkspaceID: workspaceID,
		Added:       []string{},
		Removed:     []string{},
		Updated:     []string{},
		Errors:      []string{},
	}

	memberMap, ok := m.members[workspaceID]
	if !ok {
		memberMap = make(map[string]*memberInfo)
		m.members[workspaceID] = memberMap
	}

	for _, ch := range changes {
		userID := strings.TrimSpace(ch.UserID)
		if userID == "" {
			result.Errors = append(result.Errors, "empty user ID in member change")
			continue
		}

		action := sanitizeAction(ch.Action)
		if action == "" {
			result.Errors = append(result.Errors, fmt.Sprintf("invalid action %q for user %s", ch.Action, userID))
			continue
		}

		switch action {
		case ActionAdd:
			if _, exists := memberMap[userID]; exists {
				result.Errors = append(result.Errors, fmt.Sprintf("user %s already a member", userID))
				continue
			}
			perm := sanitizePermission(ch.Permission)
			if perm == PermissionOwner {
				perm = PermissionAdmin // 不能通过add授予owner
			}
			now := nowUnix()
			memberMap[userID] = &memberInfo{
				UserID:     userID,
				Permission: perm,
				JoinedAt:   now,
				LastActive: now,
			}
			result.Added = append(result.Added, userID)

		case ActionRemove:
			if _, exists := memberMap[userID]; !exists {
				result.Errors = append(result.Errors, fmt.Sprintf("user %s is not a member", userID))
				continue
			}
			if memberMap[userID].Permission == PermissionOwner {
				result.Errors = append(result.Errors, fmt.Sprintf("cannot remove owner %s", userID))
				continue
			}
			delete(memberMap, userID)
			result.Removed = append(result.Removed, userID)

		case ActionUpdate:
			if _, exists := memberMap[userID]; !exists {
				result.Errors = append(result.Errors, fmt.Sprintf("user %s is not a member", userID))
				continue
			}
			if memberMap[userID].Permission == PermissionOwner {
				result.Errors = append(result.Errors, fmt.Sprintf("cannot change owner %s permission", userID))
				continue
			}
			perm := sanitizePermission(ch.Permission)
			if perm == PermissionOwner {
				perm = PermissionAdmin // 不能通过update设为owner
			}
			memberMap[userID].Permission = perm
			memberMap[userID].LastActive = nowUnix()
			result.Updated = append(result.Updated, userID)
		}
	}

	// 更新成员计数
	ws.MemberCount = len(memberMap)
	// 重新评估健康
	ws.Health = m.assessHealthLocked(workspaceID)

	return result, nil
}

// ========== SetQuota ==========

// SetQuota 设置工作区配额.
func (m *WorkspaceManager) SetQuota(workspaceID string, quotaGB float64) (*QuotaResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	ws, ok := m.workspaces[workspaceID]
	if !ok {
		return nil, fmt.Errorf("workspace %s not found", workspaceID)
	}

	if quotaGB < 0 {
		return nil, fmt.Errorf("quota must be non-negative")
	}

	prev := ws.QuotaGB
	ws.QuotaGB = quotaGB

	result := &QuotaResult{
		WorkspaceID:     workspaceID,
		QuotaGB:        quotaGB,
		PreviousQuotaGB: prev,
		Effective:       true,
	}

	// 检查当前使用量是否超限
	used := m.usedGB[workspaceID]
	if quotaGB > 0 && used > quotaGB {
		result.Warning = fmt.Sprintf("current usage %.2f GB exceeds new quota %.2f GB", used, quotaGB)
	} else if quotaGB == 0 {
		result.Warning = "quota set to 0 (unlimited)"
	}

	// 重新评估健康
	ws.Health = m.assessHealthLocked(workspaceID)

	return result, nil
}

// ========== InviteExternal ==========

// InviteExternal 邀请外部用户.
func (m *WorkspaceManager) InviteExternal(workspaceID string, invite ExternalInvite) (*InviteResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.workspaces[workspaceID]; !ok {
		return nil, fmt.Errorf("workspace %s not found", workspaceID)
	}

	if strings.TrimSpace(invite.Email) == "" {
		return nil, fmt.Errorf("email is required")
	}

	perm := sanitizePermission(invite.Permission)
	if perm == PermissionOwner {
		perm = PermissionViewer // 外部用户不能是owner，降到viewer
	}

	expHours := invite.ExpiresIn
	if expHours <= 0 {
		expHours = 72 // 默认72小时
	}
	expiresAt := nowUnix() + int64(expHours)*3600

	inviteID := generateID("inv")
	inviteLink := fmt.Sprintf("https://nas.local/invite/%s", inviteID)

	info := &inviteInfo{
		InviteID:    inviteID,
		Email:       strings.TrimSpace(invite.Email),
		Permission:  perm,
		WorkspaceID: workspaceID,
		Status:      InviteStatusPending,
		ExpiresAt:   expiresAt,
	}
	m.invites[inviteID] = info

	result := &InviteResult{
		InviteID:   inviteID,
		Email:      info.Email,
		Status:     InviteStatusPending,
		ExpiresAt:  expiresAt,
		InviteLink: inviteLink,
	}

	return result, nil
}

// ========== AssessHealth ==========

// AssessHealth 评估工作区健康度.
func (m *WorkspaceManager) AssessHealth(workspaceID string) (*WorkspaceHealth, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.workspaces[workspaceID]; !ok {
		return nil, fmt.Errorf("workspace %s not found", workspaceID)
	}

	return m.assessHealthFull(workspaceID), nil
}

// assessHealthLocked 在已持锁的情况下评估健康(简化版，返回状态字符串).
func (m *WorkspaceManager) assessHealthLocked(workspaceID string) string {
	health := m.computeHealth(workspaceID)
	return health.toStatus(health.Score)
}

// assessHealthFull 构建完整的 WorkspaceHealth.
func (m *WorkspaceManager) assessHealthFull(workspaceID string) *WorkspaceHealth {
	h := m.computeHealth(workspaceID)
	return &WorkspaceHealth{
		WorkspaceID:     workspaceID,
		Score:           h.Score,
		Issues:          h.Issues,
		Recommendations: h.Recommendations,
		ActiveMembers:   h.ActiveMembers,
		StorageTrend:    h.StorageTrend,
	}
}

// healthInfo 健康计算中间结果.
type healthInfo struct {
	Score           float64
	Issues          []string
	Recommendations []string
	ActiveMembers   int
	StorageTrend   string
}

// computeHealth 计算工作区健康度.
func (m *WorkspaceManager) computeHealth(workspaceID string) healthInfo {
	ws := m.workspaces[workspaceID]
	memberMap := m.members[workspaceID]
	used := m.usedGB[workspaceID]

	score := 100.0
	var issues []string
	var recommendations []string

	// 1. 存储配额检查
	if ws.QuotaGB > 0 {
		usagePct := (used / ws.QuotaGB) * 100
		if usagePct >= 95 {
			score -= 30
			issues = append(issues, fmt.Sprintf("storage usage %.1f%% exceeds 95%%", usagePct))
			recommendations = append(recommendations, "increase quota or clean up unnecessary files")
		} else if usagePct >= 80 {
			score -= 15
			issues = append(issues, fmt.Sprintf("storage usage %.1f%% exceeds 80%%", usagePct))
			recommendations = append(recommendations, "monitor storage growth and plan capacity expansion")
		}

		// 存储趋势
		if usagePct >= 90 {
			StorageTrend := "critical"
			_ = StorageTrend
		}
	}

	// 2. 成员活跃度检查
	now := nowUnix()
	activeCount := 0
	activeThreshold := int64(7 * 24 * 3600) // 7天内活跃

	for _, mem := range memberMap {
		if now-mem.LastActive < activeThreshold {
			activeCount++
		}
	}

	if activeCount == 0 {
		score -= 25
		issues = append(issues, "no active members in the last 7 days")
		recommendations = append(recommendations, "review workspace relevance or notify members")
	} else if activeCount < len(memberMap)/2 && len(memberMap) > 2 {
		score -= 10
		issues = append(issues, fmt.Sprintf("only %d of %d members active in last 7 days", activeCount, len(memberMap)))
		recommendations = append(recommendations, "engage inactive members")
	}

	// 3. 成员数量检查
	if len(memberMap) == 1 {
		score -= 10
		issues = append(issues, "workspace has only 1 member (owner only)")
		recommendations = append(recommendations, "invite more members for collaboration")
	}

	// 4. 配额为0（无限制）检查
	if ws.QuotaGB == 0 {
		score -= 5
		issues = append(issues, "no quota limit set")
		recommendations = append(recommendations, "set a quota to prevent uncontrolled storage growth")
	}

	// 5. 活跃邀请检查
	pendingInvites := 0
	expiredInvites := 0
	for _, inv := range m.invites {
		if inv.WorkspaceID != workspaceID {
			continue
		}
		if inv.Status == InviteStatusPending {
			if now > inv.ExpiresAt {
				expiredInvites++
			} else {
				pendingInvites++
			}
		}
	}
	if expiredInvites > 0 {
		score -= 5
		issues = append(issues, fmt.Sprintf("%d expired invite(s) need cleanup", expiredInvites))
		recommendations = append(recommendations, "clean up expired invites")
	}
	if pendingInvites > 5 {
		score -= 5
		issues = append(issues, fmt.Sprintf("%d pending invites, more than 5", pendingInvites))
		recommendations = append(recommendations, "review pending invitations")
	}

	// 存储趋势
	storageTrend := "stable"
	if ws.QuotaGB > 0 {
		usagePct := (used / ws.QuotaGB) * 100
		if usagePct >= 90 {
			storageTrend = "critical"
		} else if usagePct >= 70 {
			storageTrend = "growing"
		}
	} else {
		if used > 100 {
			storageTrend = "growing"
		}
	}

	// 确保分数在合理范围
	score = math.Max(0, math.Min(100, score))

	if len(issues) == 0 {
		recommendations = append(recommendations, "workspace is in good condition")
	}

	return healthInfo{
		Score:           math.Round(score*100) / 100,
		Issues:          issues,
		Recommendations: recommendations,
		ActiveMembers:   activeCount,
		StorageTrend:    storageTrend,
	}
}

// toStatus 将分数转为状态字符串.
func (h healthInfo) toStatus(score float64) string {
	switch {
	case score >= 90:
		return HealthExcellent
	case score >= 75:
		return HealthGood
	case score >= 60:
		return HealthFair
	case score >= 40:
		return HealthWarning
	default:
		return HealthCritical
	}
}

// ========== ListWorkspaces ==========

// ListWorkspaces 列出工作区.
func (m *WorkspaceManager) ListWorkspaces(filter WorkspaceFilter) ([]Workspace, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result []Workspace

	for _, ws := range m.workspaces {
		// 过滤：所有者
		if filter.OwnerID != "" && ws.OwnerID != filter.OwnerID {
			continue
		}
		// 过滤：名称模糊匹配
		if filter.NameLike != "" {
			if !strings.Contains(strings.ToLower(ws.Name), strings.ToLower(filter.NameLike)) {
				continue
			}
		}
		// 过滤：最小成员数
		if filter.MinMembers > 0 && ws.MemberCount < filter.MinMembers {
			continue
		}
		// 返回副本
		result = append(result, *ws)
	}

	// 按创建时间降序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt > result[j].CreatedAt
	})

	return result, nil
}

// ========== 辅助方法（用于测试和内部调用） ==========

// SetUsedGB 设置工作区已用量（供测试使用）.
func (m *WorkspaceManager) SetUsedGB(workspaceID string, usedGB float64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.workspaces[workspaceID]; !ok {
		return fmt.Errorf("workspace %s not found", workspaceID)
	}
	m.usedGB[workspaceID] = usedGB
	m.workspaces[workspaceID].UsedGB = usedGB
	m.workspaces[workspaceID].Health = m.assessHealthLocked(workspaceID)
	return nil
}

// TouchMemberActivity 更新成员活跃时间（供测试使用）.
func (m *WorkspaceManager) TouchMemberActivity(workspaceID, userID string, activeAt int64) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	memberMap, ok := m.members[workspaceID]
	if !ok {
		return fmt.Errorf("workspace %s not found", workspaceID)
	}
	mem, ok := memberMap[userID]
	if !ok {
		return fmt.Errorf("user %s not found in workspace %s", userID, workspaceID)
	}
	mem.LastActive = activeAt
	m.workspaces[workspaceID].Health = m.assessHealthLocked(workspaceID)
	return nil
}

// GetInvite 获取邀请信息（供测试使用）.
func (m *WorkspaceManager) GetInvite(inviteID string) (*inviteInfo, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	inv, ok := m.invites[inviteID]
	return inv, ok
}