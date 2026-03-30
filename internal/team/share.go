// Package team 外链分享功能
package team

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// ShareManager 分享管理器
type ShareManager struct {
	mu         sync.RWMutex
	shares     map[string]*ShareLink // shareID -> ShareLink
	tokenIndex map[string]string     // token -> shareID
	accessLog  map[string][]*ShareAccess // shareID -> accesses
	configPath string
	manager    *Manager
}

// NewShareManager 创建分享管理器
func NewShareManager(configPath string, manager *Manager) *ShareManager {
	sm := &ShareManager{
		shares:     make(map[string]*ShareLink),
		tokenIndex: make(map[string]string),
		accessLog:  make(map[string][]*ShareAccess),
		configPath: configPath,
		manager:    manager,
	}
	
	// 加载配置
	if configPath != "" {
		sm.loadConfig()
	}
	
	return sm
}

// loadConfig 加载配置
func (sm *ShareManager) loadConfig() error {
	if _, err := os.Stat(sm.configPath); os.IsNotExist(err) {
		return nil
	}
	
	data, err := os.ReadFile(sm.configPath)
	if err != nil {
		return err
	}
	
	var config struct {
		Shares    map[string]*ShareLink `json:"shares"`
		AccessLog map[string][]*ShareAccess `json:"access_log"`
	}
	
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}
	
	sm.shares = config.Shares
	sm.accessLog = config.AccessLog
	
	// 重建token索引
	for id, share := range sm.shares {
		sm.tokenIndex[share.Token] = id
	}
	
	return nil
}

// saveConfig 保存配置
func (sm *ShareManager) saveConfig() error {
	if sm.configPath == "" {
		return nil
	}
	
	sm.mu.RLock()
	config := struct {
		Shares    map[string]*ShareLink `json:"shares"`
		AccessLog map[string][]*ShareAccess `json:"access_log"`
	}{
		Shares:    sm.shares,
		AccessLog: sm.accessLog,
	}
	sm.mu.RUnlock()
	
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}
	
	if err := os.MkdirAll(filepath.Dir(sm.configPath), 0750); err != nil {
		return err
	}
	
	return os.WriteFile(sm.configPath, data, 0600)
}

// generateShareToken 生成分享Token
func generateShareToken() string {
	b := make([]byte, 32)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// hashPassword 哈希密码
func hashPassword(password string) string {
	if password == "" {
		return ""
	}
	h := sha256.Sum256([]byte(password + "share_salt"))
	return hex.EncodeToString(h[:])
}

// CreateShare 创建分享链接
func (sm *ShareManager) CreateShare(input ShareInput, userID, username string) (*ShareLink, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	share := &ShareLink{
		ID:           generateID(),
		Token:        generateShareToken(),
		ResourceType: input.ResourceType,
		ResourceID:   input.ResourceID,
		ResourcePath: input.ResourcePath,
		CreatedBy:    userID,
		CreatedAt:    time.Now(),
		Password:     hashPassword(input.Password),
		HasPassword:  input.Password != "",
		MaxAccess:    input.MaxAccess,
		Permission:   input.Permission,
		IsActive:     true,
	}
	
	// 设置过期时间
	if input.ExpiresIn > 0 {
		expiresAt := time.Now().Add(input.ExpiresIn)
		share.ExpiresAt = &expiresAt
	}
	
	// 设置默认权限
	if share.Permission == "" {
		share.Permission = ShareView
	}
	
	sm.shares[share.ID] = share
	sm.tokenIndex[share.Token] = share.ID
	sm.accessLog[share.ID] = make([]*ShareAccess, 0)
	
	// 记录审计日志
	if sm.manager != nil && sm.manager.audit != nil {
		sm.manager.audit.Log(&TeamAuditLog{
			UserID:       userID,
			Username:     username,
			Action:       AuditShareCreate,
			ResourceType: string(input.ResourceType),
			ResourceID:   input.ResourceID,
			ResourcePath: input.ResourcePath,
			Details: map[string]interface{}{
				"token":      share.Token,
				"permission": share.Permission,
				"has_password": share.HasPassword,
			},
		})
	}
	
	sm.saveConfig()
	return share, nil
}

// GetShareByToken 通过Token获取分享
func (sm *ShareManager) GetShareByToken(token string) (*ShareLink, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	shareID, ok := sm.tokenIndex[token]
	if !ok {
		return nil, ErrShareNotFound
	}
	
	share, ok := sm.shares[shareID]
	if !ok {
		return nil, ErrShareNotFound
	}
	
	return share, nil
}

// AccessShare 访问分享
func (sm *ShareManager) AccessShare(token, password, userID, ip, userAgent string) (*ShareLink, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	shareID, ok := sm.tokenIndex[token]
	if !ok {
		return nil, ErrShareNotFound
	}
	
	share, ok := sm.shares[shareID]
	if !ok {
		return nil, ErrShareNotFound
	}
	
	// 检查是否激活
	if !share.IsActive {
		return nil, ErrShareNotFound
	}
	
	// 检查是否过期
	if share.ExpiresAt != nil && time.Now().After(*share.ExpiresAt) {
		return nil, ErrShareExpired
	}
	
	// 检查访问次数
	if share.MaxAccess > 0 && share.AccessCount >= share.MaxAccess {
		return nil, ErrShareLimit
	}
	
	// 验证密码
	if share.HasPassword {
		if hashPassword(password) != share.Password {
			return nil, ErrSharePassword
		}
	}
	
	// 记录访问
	share.AccessCount++
	
	access := &ShareAccess{
		ID:        generateID(),
		ShareID:   share.ID,
		UserID:    userID,
		IP:        ip,
		UserAgent: userAgent,
		AccessAt:  time.Now(),
		Action:    "view",
	}
	
	sm.accessLog[share.ID] = append(sm.accessLog[share.ID], access)
	
	// 记录审计日志
	if sm.manager != nil && sm.manager.audit != nil {
		sm.manager.audit.Log(&TeamAuditLog{
			UserID:   userID,
			Action:   AuditShareAccess,
			Details: map[string]interface{}{
				"share_id":   share.ID,
				"token":      token,
				"ip":         ip,
				"access_count": share.AccessCount,
			},
		})
	}
	
	sm.saveConfig()
	return share, nil
}

// RevokeShare 撤销分享
func (sm *ShareManager) RevokeShare(shareID, userID, username string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	share, ok := sm.shares[shareID]
	if !ok {
		return ErrShareNotFound
	}
	
	// 检查权限（只有创建者或管理员可以撤销）
	if share.CreatedBy != userID {
		// 如果有团队管理器，检查是否是管理员
		if sm.manager == nil || !sm.manager.hasPermissionForUser(userID, RoleAdmin) {
			return ErrNoPermission
		}
	}
	
	share.IsActive = false
	
	// 记录审计日志
	if sm.manager != nil && sm.manager.audit != nil {
		sm.manager.audit.Log(&TeamAuditLog{
			UserID:       userID,
			Username:     username,
			Action:       AuditShareRevoke,
			ResourceID:   shareID,
			ResourcePath: share.ResourcePath,
			Details: map[string]interface{}{
				"token": share.Token,
			},
		})
	}
	
	sm.saveConfig()
	return nil
}

// DeleteShare 删除分享
func (sm *ShareManager) DeleteShare(shareID, userID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	share, ok := sm.shares[shareID]
	if !ok {
		return ErrShareNotFound
	}
	
	// 检查权限
	if share.CreatedBy != userID {
		if sm.manager == nil || !sm.manager.hasPermissionForUser(userID, RoleAdmin) {
			return ErrNoPermission
		}
	}
	
	delete(sm.shares, shareID)
	delete(sm.tokenIndex, share.Token)
	delete(sm.accessLog, shareID)
	
	sm.saveConfig()
	return nil
}

// ListUserShares 列出用户创建的分享
func (sm *ShareManager) ListUserShares(userID string) []*ShareLink {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	shares := make([]*ShareLink, 0)
	for _, share := range sm.shares {
		if share.CreatedBy == userID {
			shares = append(shares, share)
		}
	}
	return shares
}

// ListResourceShares 列出资源的分享
func (sm *ShareManager) ListResourceShares(resourceType ResourceType, resourceID string) []*ShareLink {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	shares := make([]*ShareLink, 0)
	for _, share := range sm.shares {
		if share.ResourceType == resourceType && share.ResourceID == resourceID && share.IsActive {
			shares = append(shares, share)
		}
	}
	return shares
}

// GetShareAccessLog 获取分享访问日志
func (sm *ShareManager) GetShareAccessLog(shareID string) ([]*ShareAccess, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	if _, ok := sm.shares[shareID]; !ok {
		return nil, ErrShareNotFound
	}
	
	accesses, ok := sm.accessLog[shareID]
	if !ok {
		return []*ShareAccess{}, nil
	}
	
	return accesses, nil
}

// UpdateShare 更新分享
func (sm *ShareManager) UpdateShare(shareID string, input ShareInput, userID, username string) (*ShareLink, error) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	share, ok := sm.shares[shareID]
	if !ok {
		return nil, ErrShareNotFound
	}
	
	// 检查权限
	if share.CreatedBy != userID {
		if sm.manager == nil || !sm.manager.hasPermissionForUser(userID, RoleAdmin) {
			return nil, ErrNoPermission
		}
	}
	
	// 更新字段
	if input.Password != "" {
		share.Password = hashPassword(input.Password)
		share.HasPassword = true
	}
	
	if input.ExpiresIn > 0 {
		expiresAt := time.Now().Add(input.ExpiresIn)
		share.ExpiresAt = &expiresAt
	}
	
	if input.MaxAccess > 0 {
		share.MaxAccess = input.MaxAccess
	}
	
	if input.Permission != "" {
		share.Permission = input.Permission
	}
	
	sm.saveConfig()
	return share, nil
}

// CleanExpiredShares 清理过期分享
func (sm *ShareManager) CleanExpiredShares() int {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	now := time.Now()
	count := 0
	
	for id, share := range sm.shares {
		if share.ExpiresAt != nil && now.After(*share.ExpiresAt) {
			delete(sm.shares, id)
			delete(sm.tokenIndex, share.Token)
			count++
		}
	}
	
	if count > 0 {
		sm.saveConfig()
	}
	
	return count
}

// GetShareStats 获取分享统计
func (sm *ShareManager) GetShareStats() map[string]interface{} {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	activeCount := 0
	expiredCount := 0
	now := time.Now()
	
	for _, share := range sm.shares {
		if share.IsActive && (share.ExpiresAt == nil || now.Before(*share.ExpiresAt)) {
			activeCount++
		} else {
			expiredCount++
		}
	}
	
	totalAccess := 0
	for _, accesses := range sm.accessLog {
		totalAccess += len(accesses)
	}
	
	return map[string]interface{}{
		"total_shares":   len(sm.shares),
		"active_shares":  activeCount,
		"expired_shares": expiredCount,
		"total_access":   totalAccess,
	}
}

// 辅助方法：检查用户是否有权限
func (m *Manager) hasPermissionForUser(userID string, requiredRole MemberRole) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	
	// 检查用户所在的所有团队
	for teamID := range m.userTeams[userID] {
		if m.hasPermission(teamID, userID, requiredRole) {
			return true
		}
	}
	return false
}