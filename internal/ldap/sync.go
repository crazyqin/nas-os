package ldap

import (
	"context"
	"fmt"
	"log"
	"strings"
	"sync"
	"time"

	"github.com/go-ldap/ldap/v3"
)

// Synchronizer LDAP 同步器
type Synchronizer struct {
	client     *Client
	config     Config
	userSync   UserSyncHandler
	groupSync  GroupSyncHandler
	mu         sync.Mutex
	running    bool
	cancel     context.CancelFunc
	lastSync   time.Time
	lastResult *SyncResult
}

// UserSyncHandler 用户同步处理器接口
type UserSyncHandler interface {
	CreateUser(user *User) error
	UpdateUser(user *User) error
	DeactivateUser(username string) error
	GetUser(username string) (*User, error)
	ListUsers() ([]*User, error)
}

// GroupSyncHandler 组同步处理器接口
type GroupSyncHandler interface {
	CreateGroup(group *Group) error
	UpdateGroup(group *Group) error
	DeleteGroup(name string) error
	GetGroup(name string) (*Group, error)
	ListGroups() ([]*Group, error)
	AddUserToGroup(username, groupName string) error
	RemoveUserFromGroup(username, groupName string) error
}

// NewSynchronizer 创建同步器
func NewSynchronizer(config Config, userHandler UserSyncHandler, groupHandler GroupSyncHandler) (*Synchronizer, error) {
	client, err := NewClient(config)
	if err != nil {
		return nil, err
	}

	return &Synchronizer{
		client:    client,
		config:    config,
		userSync:  userHandler,
		groupSync: groupHandler,
	}, nil
}

// Start 启动同步
func (s *Synchronizer) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.running {
		return fmt.Errorf("同步器已在运行")
	}

	ctx, cancel := context.WithCancel(ctx)
	s.cancel = cancel
	s.running = true

	go s.syncLoop(ctx)

	return nil
}

// Stop 停止同步
func (s *Synchronizer) Stop() {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.running {
		return
	}

	if s.cancel != nil {
		s.cancel()
	}
	s.running = false
}

// syncLoop 同步循环
func (s *Synchronizer) syncLoop(ctx context.Context) {
	interval := s.config.SyncConfig.Interval
	if interval <= 0 {
		interval = time.Hour
	}

	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			result, err := s.SyncAll()
			if err != nil {
				log.Printf("LDAP 同步失败: %v", err)
			}
			s.lastResult = result
			s.lastSync = time.Now()
		}
	}
}

// SyncAll 执行全量同步
func (s *Synchronizer) SyncAll() (*SyncResult, error) {
	result := &SyncResult{
		StartTime: time.Now(),
	}

	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// 同步组（先同步组，因为用户可能依赖组）
	if s.config.SyncConfig.SyncGroups {
		if err := s.syncGroups(result); err != nil {
			result.Message = fmt.Sprintf("组同步失败: %v", err)
			return result, err
		}
	}

	// 同步用户
	if s.config.SyncConfig.SyncUsers {
		if err := s.syncUsers(result); err != nil {
			result.Message = fmt.Sprintf("用户同步失败: %v", err)
			return result, err
		}
	}

	result.Success = true
	result.Message = "同步完成"

	return result, nil
}

// syncUsers 同步用户
func (s *Synchronizer) syncUsers(result *SyncResult) error {
	// 获取 LDAP 用户
	ldapUsers, err := s.fetchAllUsers()
	if err != nil {
		return fmt.Errorf("获取 LDAP 用户失败: %w", err)
	}

	// 获取本地用户
	localUsers, err := s.userSync.ListUsers()
	if err != nil {
		return fmt.Errorf("获取本地用户失败: %w", err)
	}

	// 建立映射
	localUserMap := make(map[string]*User)
	for _, u := range localUsers {
		localUserMap[u.Username] = u
	}

	ldapUserMap := make(map[string]*User)
	for _, u := range ldapUsers {
		ldapUserMap[u.Username] = u
	}

	// 处理 LDAP 用户
	for _, ldapUser := range ldapUsers {
		_, exists := localUserMap[ldapUser.Username]

		if !exists {
			// 创建新用户
			if s.config.SyncConfig.CreateUsers {
				if err := s.userSync.CreateUser(ldapUser); err != nil {
					result.UsersFailed++
					result.UserErrors = append(result.UserErrors, SyncError{
						Name:  ldapUser.Username,
						Type:  "user",
						Error: err.Error(),
					})
					continue
				}
				result.UsersCreated++
			}
		} else {
			// 更新现有用户
			if s.config.SyncConfig.UpdateUsers {
				if err := s.userSync.UpdateUser(ldapUser); err != nil {
					result.UsersFailed++
					result.UserErrors = append(result.UserErrors, SyncError{
						Name:  ldapUser.Username,
						Type:  "user",
						Error: err.Error(),
					})
					continue
				}
				result.UsersUpdated++
			} else {
				result.UsersSkipped++
			}
		}
	}

	// 处理本地不存在于 LDAP 的用户
	if s.config.SyncConfig.DeactivateUsers {
		for _, localUser := range localUsers {
			if _, exists := ldapUserMap[localUser.Username]; !exists {
				// 用户在 LDAP 中不存在，停用
				if err := s.userSync.DeactivateUser(localUser.Username); err != nil {
					result.UserErrors = append(result.UserErrors, SyncError{
						Name:  localUser.Username,
						Type:  "user",
						Error: err.Error(),
					})
					continue
				}
				result.UsersDeactivated++
			}
		}
	}

	return nil
}

// syncGroups 同步组
func (s *Synchronizer) syncGroups(result *SyncResult) error {
	// 获取 LDAP 组
	ldapGroups, err := s.fetchAllGroups()
	if err != nil {
		return fmt.Errorf("获取 LDAP 组失败: %w", err)
	}

	// 获取本地组
	localGroups, err := s.groupSync.ListGroups()
	if err != nil {
		return fmt.Errorf("获取本地组失败: %w", err)
	}

	// 建立映射
	localGroupMap := make(map[string]*Group)
	for _, g := range localGroups {
		localGroupMap[g.Name] = g
	}

	ldapGroupMap := make(map[string]*Group)
	for _, g := range ldapGroups {
		ldapGroupMap[g.Name] = g
	}

	// 处理 LDAP 组
	for _, ldapGroup := range ldapGroups {
		localGroup, exists := localGroupMap[ldapGroup.Name]

		if !exists {
			// 创建新组
			if s.config.SyncConfig.CreateGroups {
				if err := s.groupSync.CreateGroup(ldapGroup); err != nil {
					result.GroupsFailed++
					result.GroupErrors = append(result.GroupErrors, SyncError{
						Name:  ldapGroup.Name,
						Type:  "group",
						Error: err.Error(),
					})
					continue
				}
				result.GroupsCreated++
			}
		} else {
			// 更新现有组
			if s.config.SyncConfig.UpdateGroups {
				// 合并成员
				if s.config.SyncConfig.ConflictResolution == "merge" {
					ldapGroup.Members = s.mergeMembers(localGroup.Members, ldapGroup.Members)
				}

				if err := s.groupSync.UpdateGroup(ldapGroup); err != nil {
					result.GroupsFailed++
					result.GroupErrors = append(result.GroupErrors, SyncError{
						Name:  ldapGroup.Name,
						Type:  "group",
						Error: err.Error(),
					})
					continue
				}
				result.GroupsUpdated++
			} else {
				result.GroupsSkipped++
			}
		}
	}

	// 处理本地不存在于 LDAP 的组
	if s.config.SyncConfig.DeleteGroups {
		for _, localGroup := range localGroups {
			if _, exists := ldapGroupMap[localGroup.Name]; !exists {
				if err := s.groupSync.DeleteGroup(localGroup.Name); err != nil {
					result.GroupErrors = append(result.GroupErrors, SyncError{
						Name:  localGroup.Name,
						Type:  "group",
						Error: err.Error(),
					})
					continue
				}
				result.GroupsDeleted++
			}
		}
	}

	return nil
}

// mergeMembers 合并成员列表
func (s *Synchronizer) mergeMembers(local, ldap []string) []string {
	members := make(map[string]bool)
	for _, m := range local {
		members[m] = true
	}
	for _, m := range ldap {
		members[m] = true
	}

	result := make([]string, 0, len(members))
	for m := range members {
		result = append(result, m)
	}
	return result
}

// fetchAllUsers 获取所有 LDAP 用户
func (s *Synchronizer) fetchAllUsers() ([]*User, error) {
	if err := s.client.Bind(); err != nil {
		return nil, err
	}

	filter := fmt.Sprintf("(objectClass=%s)", s.config.Attributes.UserObjectClass)
	if s.config.SyncConfig.UserFilter != "" {
		filter = fmt.Sprintf("(&%s%s)", filter, s.config.SyncConfig.UserFilter)
	}

	searchDN := s.config.BaseDN
	if s.config.UserSearchDN != "" {
		searchDN = s.config.UserSearchDN
	}

	attributes := []string{
		"dn",
		s.config.Attributes.Username,
		s.config.Attributes.Email,
		s.config.Attributes.FirstName,
		s.config.Attributes.LastName,
		s.config.Attributes.FullName,
		s.config.Attributes.DisplayName,
		s.config.Attributes.MemberOfAttribute,
	}

	searchRequest := ldap.NewSearchRequest(
		searchDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		filter,
		attributes,
		nil,
	)

	result, err := s.client.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	users := make([]*User, 0, len(result.Entries))
	for _, entry := range result.Entries {
		user := &User{
			DN:          entry.DN,
			Username:    entry.GetAttributeValue(s.config.Attributes.Username),
			Email:       entry.GetAttributeValue(s.config.Attributes.Email),
			FirstName:   entry.GetAttributeValue(s.config.Attributes.FirstName),
			LastName:    entry.GetAttributeValue(s.config.Attributes.LastName),
			FullName:    entry.GetAttributeValue(s.config.Attributes.FullName),
			DisplayName: entry.GetAttributeValue(s.config.Attributes.DisplayName),
		}

		// 处理组
		memberOf := entry.GetAttributeValues(s.config.Attributes.MemberOfAttribute)
		user.Groups = make([]string, 0, len(memberOf))
		for _, dn := range memberOf {
			if cn := s.extractCN(dn); cn != "" {
				user.Groups = append(user.Groups, cn)
			}
		}

		// 应用排除过滤
		if s.config.SyncConfig.UserExcludeFilter != "" {
			if s.matchesExcludeFilter(user, s.config.SyncConfig.UserExcludeFilter) {
				continue
			}
		}

		users = append(users, user)
	}

	return users, nil
}

// fetchAllGroups 获取所有 LDAP 组
func (s *Synchronizer) fetchAllGroups() ([]*Group, error) {
	if err := s.client.Bind(); err != nil {
		return nil, err
	}

	filter := fmt.Sprintf("(objectClass=%s)", s.config.Attributes.GroupObjectClass)
	if s.config.SyncConfig.GroupFilter != "" {
		filter = fmt.Sprintf("(&%s%s)", filter, s.config.SyncConfig.GroupFilter)
	}

	searchDN := s.config.BaseDN
	if s.config.GroupSearchDN != "" {
		searchDN = s.config.GroupSearchDN
	}

	attributes := []string{
		"dn",
		s.config.Attributes.GroupName,
		s.config.Attributes.GroupDescription,
		s.config.Attributes.GroupMember,
	}

	searchRequest := ldap.NewSearchRequest(
		searchDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		0, 0, false,
		filter,
		attributes,
		nil,
	)

	result, err := s.client.Search(searchRequest)
	if err != nil {
		return nil, err
	}

	groups := make([]*Group, 0, len(result.Entries))
	for _, entry := range result.Entries {
		group := &Group{
			DN:          entry.DN,
			Name:        entry.GetAttributeValue(s.config.Attributes.GroupName),
			Description: entry.GetAttributeValue(s.config.Attributes.GroupDescription),
			Members:     entry.GetAttributeValues(s.config.Attributes.GroupMember),
		}

		// 将 DN 转换为用户名
		group.MemberUsers = make([]string, 0, len(group.Members))
		for _, dn := range group.Members {
			if cn := s.extractCN(dn); cn != "" {
				group.MemberUsers = append(group.MemberUsers, cn)
			}
		}

		groups = append(groups, group)
	}

	return groups, nil
}

// extractCN 从 DN 中提取 CN
func (s *Synchronizer) extractCN(dn string) string {
	parsed, err := ldap.ParseDN(dn)
	if err != nil {
		return ""
	}

	for _, rdn := range parsed.RDNs {
		for _, attr := range rdn.Attributes {
			if strings.EqualFold(attr.Type, "cn") {
				return attr.Value
			}
		}
	}

	return ""
}

// matchesExcludeFilter 检查是否匹配排除过滤
func (s *Synchronizer) matchesExcludeFilter(user *User, filter string) bool {
	// 简单实现：检查用户名或邮箱是否包含过滤字符串
	return strings.Contains(user.Username, filter) ||
		strings.Contains(user.Email, filter)
}

// SyncSingleUser 同步单个用户
func (s *Synchronizer) SyncSingleUser(username string) (*SyncResult, error) {
	result := &SyncResult{
		StartTime: time.Now(),
	}
	defer func() {
		result.EndTime = time.Now()
		result.Duration = result.EndTime.Sub(result.StartTime)
	}()

	// 从 LDAP 获取用户
	if err := s.client.Bind(); err != nil {
		return result, err
	}

	filter := fmt.Sprintf("(&(objectClass=%s)(%s=%s))",
		s.config.Attributes.UserObjectClass,
		s.config.Attributes.Username,
		ldap.EscapeFilter(username))

	searchDN := s.config.BaseDN
	if s.config.UserSearchDN != "" {
		searchDN = s.config.UserSearchDN
	}

	searchRequest := ldap.NewSearchRequest(
		searchDN,
		ldap.ScopeWholeSubtree,
		ldap.NeverDerefAliases,
		1, 0, false,
		filter,
		[]string{
			"dn",
			s.config.Attributes.Username,
			s.config.Attributes.Email,
			s.config.Attributes.FirstName,
			s.config.Attributes.LastName,
			s.config.Attributes.FullName,
			s.config.Attributes.MemberOfAttribute,
		},
		nil,
	)

	searchResult, err := s.client.Search(searchRequest)
	if err != nil {
		return result, fmt.Errorf("搜索用户失败: %w", err)
	}

	if len(searchResult.Entries) == 0 {
		return result, ErrUserNotFound
	}

	entry := searchResult.Entries[0]
	user := &User{
		DN:          entry.DN,
		Username:    entry.GetAttributeValue(s.config.Attributes.Username),
		Email:       entry.GetAttributeValue(s.config.Attributes.Email),
		FirstName:   entry.GetAttributeValue(s.config.Attributes.FirstName),
		LastName:    entry.GetAttributeValue(s.config.Attributes.LastName),
		FullName:    entry.GetAttributeValue(s.config.Attributes.FullName),
	}

	// 处理组
	memberOf := entry.GetAttributeValues(s.config.Attributes.MemberOfAttribute)
	user.Groups = make([]string, 0, len(memberOf))
	for _, dn := range memberOf {
		if cn := s.extractCN(dn); cn != "" {
			user.Groups = append(user.Groups, cn)
		}
	}

	// 检查本地是否存在
	localUser, err := s.userSync.GetUser(username)
	if err != nil || localUser == nil {
		// 创建用户
		if err := s.userSync.CreateUser(user); err != nil {
			result.UsersFailed++
			result.UserErrors = append(result.UserErrors, SyncError{
				Name:  username,
				Type:  "user",
				Error: err.Error(),
			})
			return result, err
		}
		result.UsersCreated++
	} else {
		// 更新用户
		if err := s.userSync.UpdateUser(user); err != nil {
			result.UsersFailed++
			result.UserErrors = append(result.UserErrors, SyncError{
				Name:  username,
				Type:  "user",
				Error: err.Error(),
			})
			return result, err
		}
		result.UsersUpdated++
	}

	result.Success = true
	result.Message = "用户同步完成"

	return result, nil
}

// GetLastSync 获取上次同步时间和结果
func (s *Synchronizer) GetLastSync() (time.Time, *SyncResult) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.lastSync, s.lastResult
}

// IsRunning 检查是否正在运行
func (s *Synchronizer) IsRunning() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.running
}

// Close 关闭同步器
func (s *Synchronizer) Close() error {
	s.Stop()
	return s.client.Close()
}

// GetStatus 获取同步状态
func (s *Synchronizer) GetStatus() map[string]interface{} {
	s.mu.Lock()
	defer s.mu.Unlock()

	status := map[string]interface{}{
		"running":    s.running,
		"last_sync":  s.lastSync,
		"config":     s.config.Name,
		"sync_mode":  s.config.SyncConfig.Mode,
	}

	if s.lastResult != nil {
		status["last_result"] = map[string]interface{}{
			"success":         s.lastResult.Success,
			"duration":        s.lastResult.Duration.String(),
			"users_created":   s.lastResult.UsersCreated,
			"users_updated":   s.lastResult.UsersUpdated,
			"groups_created":  s.lastResult.GroupsCreated,
			"groups_updated":  s.lastResult.GroupsUpdated,
			"message":         s.lastResult.Message,
		}
	}

	return status
}