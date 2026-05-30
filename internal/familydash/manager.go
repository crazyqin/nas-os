// Package familydash 提供家庭仪表板管理器
package familydash

import (
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 家庭仪表板管理器
type Manager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	members     map[string]*FamilyMember
	profiles    map[string]*MemberProfile
	permissions map[string]*Permissions
	activities  []*ActivityEntry
	stopChan    chan struct{}
	running     bool
}

// NewManager 创建家庭仪表板管理器
func NewManager(logger *zap.Logger) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}

	m := &Manager{
		logger:      logger,
		members:     make(map[string]*FamilyMember),
		profiles:    make(map[string]*MemberProfile),
		permissions: make(map[string]*Permissions),
		activities:  make([]*ActivityEntry, 0),
		stopChan:    make(chan struct{}),
	}

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	return fmt.Sprintf("family-%d", time.Now().UnixNano())
}

// CreateMember 创建家庭成员
func (m *Manager) CreateMember(req *CreateMemberRequest) (*FamilyMember, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	member := &FamilyMember{
		ID:        generateID(),
		Name:      req.Name,
		Email:     req.Email,
		Avatar:    req.Avatar,
		Role:      req.Role,
		Status:    StatusOffline,
		IsChild:   req.IsChild,
		BirthYear: req.BirthYear,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if member.Role == "" {
		member.Role = RoleGuest
	}

	m.members[member.ID] = member

	// 创建默认个人资料
	profile := &MemberProfile{
		MemberID:        member.ID,
		DisplayName:     member.Name,
		Theme:           "light",
		Language:        "zh-CN",
		Timezone:        "Asia/Shanghai",
		Notifications:   true,
		DashboardLayout: DefaultDashboardLayout(),
		Favorites:       make([]FavoriteItem, 0),
		RecentFiles:     make([]RecentFile, 0),
		StorageQuota:    10 * 1024 * 1024 * 1024, // 10GB
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	m.profiles[member.ID] = profile

	// 创建默认权限
	perms := DefaultPermissions(member.Role)
	perms.MemberID = member.ID
	m.permissions[member.ID] = perms

	m.logger.Info("family member created",
		zap.String("member_id", member.ID),
		zap.String("name", member.Name),
		zap.String("role", string(member.Role)))

	return member, nil
}

// GetMember 获取家庭成员
func (m *Manager) GetMember(memberID string) (*FamilyMember, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	member, ok := m.members[memberID]
	if !ok {
		return nil, fmt.Errorf("member not found: %s", memberID)
	}
	return member, nil
}

// ListMembers 列出所有家庭成员
func (m *Manager) ListMembers() []*FamilyMember {
	m.mu.RLock()
	defer m.mu.RUnlock()

	members := make([]*FamilyMember, 0, len(m.members))
	for _, member := range m.members {
		members = append(members, member)
	}
	return members
}

// UpdateMember 更新家庭成员
func (m *Manager) UpdateMember(memberID string, req *UpdateMemberRequest) (*FamilyMember, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	member, ok := m.members[memberID]
	if !ok {
		return nil, fmt.Errorf("member not found: %s", memberID)
	}

	if req.Name != "" {
		member.Name = req.Name
	}
	if req.Email != "" {
		member.Email = req.Email
	}
	if req.Avatar != "" {
		member.Avatar = req.Avatar
	}
	if req.Role != "" {
		member.Role = req.Role
	}
	if req.BirthYear > 0 {
		member.BirthYear = req.BirthYear
	}
	member.UpdatedAt = time.Now()

	return member, nil
}

// DeleteMember 删除家庭成员
func (m *Manager) DeleteMember(memberID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.members[memberID]; !ok {
		return fmt.Errorf("member not found: %s", memberID)
	}

	delete(m.members, memberID)
	delete(m.profiles, memberID)
	delete(m.permissions, memberID)

	m.logger.Info("family member deleted", zap.String("member_id", memberID))
	return nil
}

// GetProfile 获取成员个人资料
func (m *Manager) GetProfile(memberID string) (*MemberProfile, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	profile, ok := m.profiles[memberID]
	if !ok {
		return nil, fmt.Errorf("profile not found: %s", memberID)
	}
	return profile, nil
}

// UpdateProfile 更新成员个人资料
func (m *Manager) UpdateProfile(memberID string, req *UpdateProfileRequest) (*MemberProfile, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, ok := m.profiles[memberID]
	if !ok {
		return nil, fmt.Errorf("profile not found: %s", memberID)
	}

	if req.DisplayName != "" {
		profile.DisplayName = req.DisplayName
	}
	if req.Bio != "" {
		profile.Bio = req.Bio
	}
	if req.Theme != "" {
		profile.Theme = req.Theme
	}
	if req.Language != "" {
		profile.Language = req.Language
	}
	if req.Timezone != "" {
		profile.Timezone = req.Timezone
	}
	if req.Notifications != nil {
		profile.Notifications = *req.Notifications
	}
	if req.DashboardLayout != nil {
		profile.DashboardLayout = req.DashboardLayout
	}
	profile.UpdatedAt = time.Now()

	return profile, nil
}

// GetPermissions 获取成员权限
func (m *Manager) GetPermissions(memberID string) (*Permissions, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	perms, ok := m.permissions[memberID]
	if !ok {
		return nil, fmt.Errorf("permissions not found: %s", memberID)
	}
	return perms, nil
}

// SetPermissions 设置成员权限
func (m *Manager) SetPermissions(memberID string, req *UpdatePermissionsRequest) (*Permissions, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	perms, ok := m.permissions[memberID]
	if !ok {
		// 如果不存在则创建
		perms = &Permissions{
			MemberID: memberID,
		}
		m.permissions[memberID] = perms
	}

	if req.Level != "" {
		perms.Level = req.Level
	}
	if req.CanAccessFiles != nil {
		perms.CanAccessFiles = *req.CanAccessFiles
	}
	if req.CanUpload != nil {
		perms.CanUpload = *req.CanUpload
	}
	if req.CanDownload != nil {
		perms.CanDownload = *req.CanDownload
	}
	if req.CanShare != nil {
		perms.CanShare = *req.CanShare
	}
	if req.CanStream != nil {
		perms.CanStream = *req.CanStream
	}
	if req.CanDelete != nil {
		perms.CanDelete = *req.CanDelete
	}
	if req.CanManage != nil {
		perms.CanManage = *req.CanManage
	}
	if req.AllowedApps != nil {
		perms.AllowedApps = req.AllowedApps
	}
	if req.BlockedApps != nil {
		perms.BlockedApps = req.BlockedApps
	}
	if req.StorageQuota > 0 {
		perms.StorageQuota = req.StorageQuota
	}
	if req.TimeLimit > 0 {
		perms.TimeLimit = req.TimeLimit
	}
	if req.AllowedHours != nil {
		perms.AllowedHours = req.AllowedHours
	}

	return perms, nil
}

// AddFavorite 添加收藏
func (m *Manager) AddFavorite(memberID string, req *AddFavoriteRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, ok := m.profiles[memberID]
	if !ok {
		return fmt.Errorf("profile not found: %s", memberID)
	}

	fav := FavoriteItem{
		ID:   generateID(),
		Name: req.Name,
		Type: req.Type,
		Path: req.Path,
		Icon: req.Icon,
	}

	profile.Favorites = append(profile.Favorites, fav)
	profile.UpdatedAt = time.Now()

	return nil
}

// RemoveFavorite 移除收藏
func (m *Manager) RemoveFavorite(memberID, favoriteID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	profile, ok := m.profiles[memberID]
	if !ok {
		return fmt.Errorf("profile not found: %s", memberID)
	}

	for i, fav := range profile.Favorites {
		if fav.ID == favoriteID {
			profile.Favorites = append(profile.Favorites[:i], profile.Favorites[i+1:]...)
			profile.UpdatedAt = time.Now()
			return nil
		}
	}

	return fmt.Errorf("favorite not found: %s", favoriteID)
}

// RecordActivity 记录活动
func (m *Manager) RecordActivity(memberID string, activityType ActivityType, action, resource, details string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	entry := &ActivityEntry{
		ID:        generateID(),
		MemberID:  memberID,
		Type:      activityType,
		Action:    action,
		Resource:  resource,
		Details:   details,
		Timestamp: time.Now(),
	}

	m.activities = append(m.activities, entry)

	// 限制活动记录数量
	if len(m.activities) > 10000 {
		m.activities = m.activities[len(m.activities)-10000:]
	}
}

// GetActivity 获取活动记录
func (m *Manager) GetActivity(query *ActivityQuery) []*ActivityEntry {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*ActivityEntry, 0)

	for _, entry := range m.activities {
		if query.MemberID != "" && entry.MemberID != query.MemberID {
			continue
		}
		if query.Type != "" && entry.Type != query.Type {
			continue
		}
		if query.FromDate != "" {
			fromDate, err := time.Parse("2006-01-02", query.FromDate)
			if err == nil && entry.Timestamp.Before(fromDate) {
				continue
			}
		}
		if query.ToDate != "" {
			toDate, err := time.Parse("2006-01-02", query.ToDate)
			if err == nil && entry.Timestamp.After(toDate.Add(24*time.Hour)) {
				continue
			}
		}

		result = append(result, entry)
	}

	// 按时间倒序
	for i := 0; i < len(result)-1; i++ {
		for j := i + 1; j < len(result); j++ {
			if result[j].Timestamp.After(result[i].Timestamp) {
				result[i], result[j] = result[j], result[i]
			}
		}
	}

	if query.Limit > 0 && query.Limit < len(result) {
		result = result[:query.Limit]
	}

	return result
}

// GetActivitySummary 获取活动摘要
func (m *Manager) GetActivitySummary(memberID, period string) *ActivitySummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 计算时间范围
	endDate := time.Now()
	startDate := endDate

	switch period {
	case "daily":
		startDate = endDate.AddDate(0, 0, -1)
	case "weekly":
		startDate = endDate.AddDate(0, 0, -7)
	case "monthly":
		startDate = endDate.AddDate(0, -1, 0)
	}

	summary := &ActivitySummary{
		MemberID: memberID,
		Period:   period,
	}

	activityMap := make(map[ActivityType]int)
	hourMap := make(map[int]int)

	for _, entry := range m.activities {
		if entry.MemberID == memberID &&
			entry.Timestamp.After(startDate) &&
			entry.Timestamp.Before(endDate) {

			summary.TotalActions++
			activityMap[entry.Type]++

			switch entry.Type {
			case ActivityFileUpload:
				summary.Uploads++
			case ActivityFileDownload:
				summary.Downloads++
			case ActivityStream:
				summary.Streams++
			}

			hourMap[entry.Timestamp.Hour()]++
		}
	}

	// 找出最活跃的小时
	maxCount := 0
	for hour, count := range hourMap {
		if count > maxCount {
			maxCount = count
			summary.MostActiveHour = hour
		}
	}

	// 转换为 TopActivities
	for actType, count := range activityMap {
		summary.TopActivities = append(summary.TopActivities, ActivityCount{
			Type:  actType,
			Count: count,
		})
	}

	return summary
}

// GenerateStats 生成家庭统计
func (m *Manager) GenerateStats() *FamilyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &FamilyStats{
		FamilyID:    "default",
		TotalMembers: len(m.members),
		GeneratedAt:  time.Now(),
	}

	memberStats := make([]MemberStats, 0, len(m.members))

	for _, member := range m.members {
		if member.Status == StatusOnline {
			stats.OnlineMembers++
		}

		profile := m.profiles[member.ID]
		ms := MemberStats{
			MemberID:   member.ID,
			Name:       member.Name,
			LastActive: member.LastActive,
		}

		if profile != nil {
			ms.StorageUsed = profile.StorageUsed
		}

		// 统计活动数
		for _, entry := range m.activities {
			if entry.MemberID == member.ID {
				ms.ActivityCount++
			}
		}

		memberStats = append(memberStats, ms)
		stats.UsedStorage += ms.StorageUsed
	}

	stats.MemberStats = memberStats
	stats.TotalStorage = 1024 * 1024 * 1024 * 1024 // 1TB 默认

	return stats
}

// UpdateMemberStatus 更新成员状态
func (m *Manager) UpdateMemberStatus(memberID string, status MemberStatus) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	member, ok := m.members[memberID]
	if !ok {
		return fmt.Errorf("member not found: %s", memberID)
	}

	member.Status = status
	now := time.Now()
	member.LastActive = &now
	member.UpdatedAt = now

	return nil
}

// GetOnlineMembers 获取在线成员
func (m *Manager) GetOnlineMembers() []*FamilyMember {
	m.mu.RLock()
	defer m.mu.RUnlock()

	online := make([]*FamilyMember, 0)
	for _, member := range m.members {
		if member.Status == StatusOnline {
			online = append(online, member)
		}
	}
	return online
}

// GetChildMembers 获取子成员（儿童）
func (m *Manager) GetChildMembers() []*FamilyMember {
	m.mu.RLock()
	defer m.mu.RUnlock()

	children := make([]*FamilyMember, 0)
	for _, member := range m.members {
		if member.IsChild || member.Role == RoleChild {
			children = append(children, member)
		}
	}
	return children
}

// CheckPermission 检查成员权限
func (m *Manager) CheckPermission(memberID string, action string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()

	perms, ok := m.permissions[memberID]
	if !ok {
		return false
	}

	switch action {
	case "access_files":
		return perms.CanAccessFiles
	case "upload":
		return perms.CanUpload
	case "download":
		return perms.CanDownload
	case "share":
		return perms.CanShare
	case "stream":
		return perms.CanStream
	case "delete":
		return perms.CanDelete
	case "manage":
		return perms.CanManage
	default:
		return false
	}
}
