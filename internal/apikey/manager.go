package apikey

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Manager API密钥管理器.
type Manager struct {
	mu          sync.RWMutex
	keys        map[string]*APIKey   // keyID -> APIKey
	keysByKey   map[string]*APIKey   // hashedKey -> APIKey
	userKeys    map[string][]*APIKey // userID -> []*APIKey
	auditLogs   []*AuditLog
	permissions map[string]*Permission
	scopes      map[string]*Scope
}

// NewManager 创建管理器.
func NewManager() *Manager {
	m := &Manager{
		keys:        make(map[string]*APIKey),
		keysByKey:   make(map[string]*APIKey),
		userKeys:    make(map[string][]*APIKey),
		permissions: make(map[string]*Permission),
		scopes:      make(map[string]*Scope),
	}

	// 初始化默认权限
	m.initDefaultPermissions()
	m.initDefaultScopes()

	return m
}

// initDefaultPermissions 初始化默认权限.
func (m *Manager) initDefaultPermissions() {
	defaultPerms := []*Permission{
		{ID: "read", Name: "读取", Description: "读取数据", Resource: "*", Actions: []string{"GET"}},
		{ID: "write", Name: "写入", Description: "写入数据", Resource: "*", Actions: []string{"POST", "PUT", "PATCH"}},
		{ID: "delete", Name: "删除", Description: "删除数据", Resource: "*", Actions: []string{"DELETE"}},
		{ID: "admin", Name: "管理", Description: "管理权限", Resource: "*", Actions: []string{"*"}},
		{ID: "storage", Name: "存储", Description: "存储管理", Resource: "storage", Actions: []string{"*"}},
		{ID: "network", Name: "网络", Description: "网络管理", Resource: "network", Actions: []string{"*"}},
		{ID: "system", Name: "系统", Description: "系统管理", Resource: "system", Actions: []string{"*"}},
		{ID: "docker", Name: "容器", Description: "容器管理", Resource: "docker", Actions: []string{"*"}},
		{ID: "vm", Name: "虚拟机", Description: "虚拟机管理", Resource: "vm", Actions: []string{"*"}},
	}

	for _, p := range defaultPerms {
		m.permissions[p.ID] = p
	}
}

// initDefaultScopes 初始化默认作用域.
func (m *Manager) initDefaultScopes() {
	defaultScopes := []*Scope{
		{ID: "read_only", Name: "只读", Description: "只读访问", Resources: []string{"*"}},
		{ID: "read_write", Name: "读写", Description: "读写访问", Resources: []string{"*"}},
		{ID: "full_access", Name: "完全访问", Description: "完全访问权限", Resources: []string{"*"}},
		{ID: "storage_only", Name: "仅存储", Description: "仅存储访问", Resources: []string{"storage"}},
		{ID: "network_only", Name: "仅网络", Description: "仅网络访问", Resources: []string{"network"}},
	}

	for _, s := range defaultScopes {
		m.scopes[s.ID] = s
	}
}

// generateKey 生成API密钥.
func generateKey() (string, string, error) {
	// 生成32字节随机数
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", "", err
	}

	// 转换为hex字符串
	key := hex.EncodeToString(bytes)

	// 添加前缀
	key = "nask_" + key

	// 计算哈希用于存储
	hash := sha256.Sum256([]byte(key))
	hashedKey := hex.EncodeToString(hash[:])

	return key, hashedKey, nil
}

// CreateKey 创建API密钥.
func (m *Manager) CreateKey(req *CreateKeyRequest) (*APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查用户密钥数量限制
	userKeys := m.userKeys[req.UserID]
	if len(userKeys) >= 10 { // 限制每用户最多10个密钥
		return nil, fmt.Errorf("用户 %s 已达到密钥数量上限(10)", req.UserID)
	}

	// 检查名称是否重复
	for _, key := range userKeys {
		if key.Name == req.Name && key.Status != StatusRevoked {
			return nil, fmt.Errorf("密钥名称 '%s' 已存在", req.Name)
		}
	}

	// 验证权限
	for _, perm := range req.Permissions {
		if _, ok := m.permissions[perm]; !ok {
			return nil, fmt.Errorf("权限 '%s' 不存在", perm)
		}
	}

	// 验证作用域
	for _, scope := range req.Scopes {
		if _, ok := m.scopes[scope]; !ok {
			return nil, fmt.Errorf("作用域 '%s' 不存在", scope)
		}
	}

	// 生成密钥
	key, hashedKey, err := generateKey()
	if err != nil {
		return nil, fmt.Errorf("生成密钥失败: %v", err)
	}

	// 计算过期时间
	var expiresAt *time.Time
	if req.ExpiresIn > 0 {
		t := time.Now().Add(time.Duration(req.ExpiresIn) * time.Hour)
		expiresAt = &t
	}

	// 如果没有指定权限和作用域，使用默认值
	permissions := req.Permissions
	if len(permissions) == 0 {
		permissions = []string{"read"}
	}
	scopes := req.Scopes
	if len(scopes) == 0 {
		scopes = []string{"read_only"}
	}

	apiKey := &APIKey{
		ID:          uuid.New().String(),
		Name:        req.Name,
		Description: req.Description,
		Key:         key, // 仅创建时返回
		KeyPrefix:   key[:12] + "...",
		UserID:      req.UserID,
		Permissions: permissions,
		Scopes:      scopes,
		ExpiresAt:   expiresAt,
		Status:      StatusActive,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	m.keys[apiKey.ID] = apiKey
	m.keysByKey[hashedKey] = apiKey
	m.userKeys[req.UserID] = append(m.userKeys[req.UserID], apiKey)

	// 记录审计日志
	m.addAuditLog(apiKey.ID, req.UserID, "create", "", "", "", fmt.Sprintf("创建密钥: %s", req.Name))

	return apiKey, nil
}

// GetKey 获取密钥详情.
func (m *Manager) GetKey(keyID string) (*APIKey, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	key, ok := m.keys[keyID]
	if !ok {
		return nil, fmt.Errorf("密钥 %s 不存在", keyID)
	}

	// 返回副本，隐藏密钥值
	result := *key
	result.Key = "" // 不返回完整密钥
	return &result, nil
}

// ListKeys 列出密钥.
func (m *Manager) ListKeys(req *ListKeysRequest) *ListKeysResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if req.Page <= 0 {
		req.Page = 1
	}
	if req.PageSize <= 0 {
		req.PageSize = 20
	}

	var keys []*APIKey

	if req.UserID != "" {
		// 按用户筛选
		keys = m.userKeys[req.UserID]
	} else {
		// 所有密钥
		for _, key := range m.keys {
			keys = append(keys, key)
		}
	}

	// 按状态筛选
	if req.Status != "" {
		var filtered []*APIKey
		for _, key := range keys {
			if key.Status == req.Status {
				filtered = append(filtered, key)
			}
		}
		keys = filtered
	}

	// 分页
	total := len(keys)
	start := (req.Page - 1) * req.PageSize
	end := start + req.PageSize
	if start > total {
		start = total
	}
	if end > total {
		end = total
	}

	// 隐藏密钥值
	result := make([]*APIKey, 0, end-start)
	for _, key := range keys[start:end] {
		k := *key
		k.Key = ""
		result = append(result, &k)
	}

	return &ListKeysResponse{
		Keys:     result,
		Total:    total,
		Page:     req.Page,
		PageSize: req.PageSize,
	}
}

// UpdateKey 更新密钥.
func (m *Manager) UpdateKey(keyID string, req *UpdateKeyRequest) (*APIKey, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, ok := m.keys[keyID]
	if !ok {
		return nil, fmt.Errorf("密钥 %s 不存在", keyID)
	}

	if key.Status == StatusRevoked {
		return nil, fmt.Errorf("密钥 %s 已撤销，无法更新", keyID)
	}

	if req.Name != "" {
		// 检查名称重复
		for _, k := range m.userKeys[key.UserID] {
			if k.ID != keyID && k.Name == req.Name && k.Status != StatusRevoked {
				return nil, fmt.Errorf("密钥名称 '%s' 已存在", req.Name)
			}
		}
		key.Name = req.Name
	}

	if req.Description != "" {
		key.Description = req.Description
	}

	if len(req.Permissions) > 0 {
		// 验证权限
		for _, perm := range req.Permissions {
			if _, ok := m.permissions[perm]; !ok {
				return nil, fmt.Errorf("权限 '%s' 不存在", perm)
			}
		}
		key.Permissions = req.Permissions
	}

	if len(req.Scopes) > 0 {
		// 验证作用域
		for _, scope := range req.Scopes {
			if _, ok := m.scopes[scope]; !ok {
				return nil, fmt.Errorf("作用域 '%s' 不存在", scope)
			}
		}
		key.Scopes = req.Scopes
	}

	if req.Status != "" {
		key.Status = req.Status
	}

	key.UpdatedAt = time.Now()

	// 记录审计日志
	m.addAuditLog(keyID, key.UserID, "update", "", "", "", "更新密钥")

	return key, nil
}

// RevokeKey 撤销密钥.
func (m *Manager) RevokeKey(keyID, revokedBy, reason string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, ok := m.keys[keyID]
	if !ok {
		return fmt.Errorf("密钥 %s 不存在", keyID)
	}

	if key.Status == StatusRevoked {
		return fmt.Errorf("密钥 %s 已经撤销", keyID)
	}

	now := time.Now()
	key.Status = StatusRevoked
	key.RevokedAt = &now
	key.RevokedBy = revokedBy
	key.RevokedReason = reason
	key.UpdatedAt = now

	// 记录审计日志
	m.addAuditLog(keyID, key.UserID, "revoked", "", "", "", fmt.Sprintf("撤销密钥: %s", reason))

	return nil
}

// DeleteKey 删除密钥.
func (m *Manager) DeleteKey(keyID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	key, ok := m.keys[keyID]
	if !ok {
		return fmt.Errorf("密钥 %s 不存在", keyID)
	}

	// 只能删除已撤销的密钥
	if key.Status != StatusRevoked {
		return fmt.Errorf("只能删除已撤销的密钥")
	}

	// 从所有映射中删除
	delete(m.keys, keyID)

	// 从 keysByKey 中删除
	for k, v := range m.keysByKey {
		if v.ID == keyID {
			delete(m.keysByKey, k)
			break
		}
	}

	// 从 userKeys 中删除
	if userKeys, ok := m.userKeys[key.UserID]; ok {
		for i, k := range userKeys {
			if k.ID == keyID {
				m.userKeys[key.UserID] = append(userKeys[:i], userKeys[i+1:]...)
				break
			}
		}
	}

	return nil
}

// ValidateKey 验证API密钥.
func (m *Manager) ValidateKey(req *ValidateKeyRequest) *ValidateKeyResponse {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 计算请求密钥的哈希
	hash := sha256.Sum256([]byte(req.Key))
	hashedKey := hex.EncodeToString(hash[:])

	key, ok := m.keysByKey[hashedKey]
	if !ok {
		return &ValidateKeyResponse{
			Valid: false,
			Error: "无效的API密钥",
		}
	}

	// 检查状态
	if key.Status != StatusActive {
		return &ValidateKeyResponse{
			Valid: false,
			Error: fmt.Sprintf("密钥状态异常: %s", key.Status),
		}
	}

	// 检查是否过期
	if key.ExpiresAt != nil && key.ExpiresAt.Before(time.Now()) {
		// 自动更新状态
		key.Status = StatusExpired
		return &ValidateKeyResponse{
			Valid: false,
			Error: "密钥已过期",
		}
	}

	// 检查权限（如果指定了资源和操作）
	if req.Resource != "" && req.Action != "" {
		hasPermission := false
		for _, perm := range key.Permissions {
			if p, ok := m.permissions[perm]; ok {
				if p.Resource == "*" || p.Resource == req.Resource {
					for _, action := range p.Actions {
						if action == "*" || action == req.Action {
							hasPermission = true
							break
						}
					}
				}
			}
			if hasPermission {
				break
			}
		}
		if !hasPermission {
			return &ValidateKeyResponse{
				Valid: false,
				Error: "权限不足",
			}
		}
	}

	// 更新使用统计
	key.UsageCount++
	now := time.Now()
	key.LastUsedAt = &now

	return &ValidateKeyResponse{
		Valid:       true,
		KeyID:       key.ID,
		UserID:      key.UserID,
		Permissions: key.Permissions,
		Scopes:      key.Scopes,
	}
}

// GetUserKeys 获取用户的密钥.
func (m *Manager) GetUserKeys(userID string) []*APIKey {
	m.mu.RLock()
	defer m.mu.RUnlock()

	keys := m.userKeys[userID]
	result := make([]*APIKey, 0, len(keys))
	for _, key := range keys {
		k := *key
		k.Key = ""
		result = append(result, &k)
	}
	return result
}

// GetStats 获取统计信息.
func (m *Manager) GetStats() *KeyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &KeyStats{}

	for _, key := range m.keys {
		stats.TotalKeys++
		stats.TotalUsage += key.UsageCount

		switch key.Status {
		case StatusActive:
			stats.ActiveKeys++
		case StatusExpired:
			stats.ExpiredKeys++
		case StatusRevoked:
			stats.RevokedKeys++
		case StatusDisabled:
			stats.DisabledKeys++
		}
	}

	return stats
}

// GetUserStats 获取用户统计.
func (m *Manager) GetUserStats(userID string) *UserKeyStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &UserKeyStats{
		UserID: userID,
	}

	keys := m.userKeys[userID]
	stats.TotalKeys = len(keys)

	for _, key := range keys {
		stats.TotalUsage += key.UsageCount
		if key.Status == StatusActive {
			stats.ActiveKeys++
		}
	}

	return stats
}

// CleanupExpiredKeys 清理过期密钥.
func (m *Manager) CleanupExpiredKeys() int {
	m.mu.Lock()
	defer m.mu.Unlock()

	count := 0
	now := time.Now()

	for _, key := range m.keys {
		if key.Status == StatusActive && key.ExpiresAt != nil && key.ExpiresAt.Before(now) {
			key.Status = StatusExpired
			key.UpdatedAt = now
			count++
		}
	}

	return count
}

// GetAuditLogs 获取审计日志.
func (m *Manager) GetAuditLogs(keyID string, limit int) []*AuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var logs []*AuditLog
	for _, log := range m.auditLogs {
		if keyID == "" || log.KeyID == keyID {
			logs = append(logs, log)
		}
	}

	// 返回最新的日志
	if limit > 0 && len(logs) > limit {
		logs = logs[len(logs)-limit:]
	}

	return logs
}

// addAuditLog 添加审计日志.
func (m *Manager) addAuditLog(keyID, userID, action, resource, ip, userAgent, details string) {
	log := &AuditLog{
		ID:        uuid.New().String(),
		KeyID:     keyID,
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		IPAddress: ip,
		UserAgent: userAgent,
		Details:   details,
		CreatedAt: time.Now(),
	}
	m.auditLogs = append(m.auditLogs, log)
}

// GetPermissions 获取所有权限.
func (m *Manager) GetPermissions() []*Permission {
	m.mu.RLock()
	defer m.mu.RUnlock()

	perms := make([]*Permission, 0, len(m.permissions))
	for _, p := range m.permissions {
		perms = append(perms, p)
	}
	return perms
}

// GetScopes 获取所有作用域.
func (m *Manager) GetScopes() []*Scope {
	m.mu.RLock()
	defer m.mu.RUnlock()

	scopes := make([]*Scope, 0, len(m.scopes))
	for _, s := range m.scopes {
		scopes = append(scopes, s)
	}
	return scopes
}
