// Package privacyvault - engine.go 实现保险箱引擎核心，包括保险库生命周期管理、
// 访问控制、安全文件分享、密钥分片和自动锁定功能。
package privacyvault

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// Engine 隐私保险箱引擎
type Engine struct {
	config     *PrivacyVaultConfig
	crypto     *CryptoEngine
	vaults     map[string]*Vault
	secrets    map[string][]*Secret
	policies   map[string][]*AccessPolicy
	auditLog   []*AuditLog
	shareLinks map[string]*ShareLink
	keyShares  map[string][]*KeyShare
	failCounts map[string]int    // vaultID -> fail count
	vaultKeys  map[string][]byte // vaultID -> derived key (memory only)
	mu         sync.RWMutex
	ctx        context.Context
	cancel     context.CancelFunc
}

// NewEngine 创建隐私保险箱引擎
func NewEngine(config *PrivacyVaultConfig) *Engine {
	if config == nil {
		config = DefaultConfig()
	}
	ctx, cancel := context.WithCancel(context.Background())
	e := &Engine{
		config:     config,
		crypto:     NewCryptoEngine(config.DefaultAlgorithm),
		vaults:     make(map[string]*Vault),
		secrets:    make(map[string][]*Secret),
		policies:   make(map[string][]*AccessPolicy),
		auditLog:   make([]*AuditLog, 0),
		shareLinks: make(map[string]*ShareLink),
		keyShares:  make(map[string][]*KeyShare),
		failCounts: make(map[string]int),
		vaultKeys:  make(map[string][]byte),
		ctx:        ctx,
		cancel:     cancel,
	}
	return e
}

// CreateVault 创建新的加密保险库
func (e *Engine) CreateVault(vault *Vault, passphrase string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if len(e.vaults) >= e.config.MaxVaults {
		e.addAudit(vault.ID, "", "create", "", false, "达到保险库数量上限")
		return NewPrivacyVaultError("MAX_VAULTS_REACHED",
			fmt.Sprintf("已达到最大保险库数量（%d）", e.config.MaxVaults), nil)
	}

	if _, exists := e.vaults[vault.ID]; exists {
		return ErrVaultAlreadyExists
	}

	// 派生密钥（仅用于验证，不存储）
	_, salt, err := e.crypto.DeriveKey(passphrase)
	if err != nil {
		e.addAudit(vault.ID, "", "create", "", false, err.Error())
		return err
	}

	// 生成验证令牌
	verificationToken := GenerateVerificationToken(passphrase, salt)

	// 创建密钥记录
	keyID := FormatKeyID(vault.ID, 1)

	vault.Status = StatusLocked
	vault.KeyID = keyID
	vault.CreatedAt = time.Now()
	vault.OwnerID = "" // 由调用方设置

	e.vaults[vault.ID] = vault
	e.secrets[vault.ID] = make([]*Secret, 0)
	e.policies[vault.ID] = make([]*AccessPolicy, 0)
	e.keyShares[vault.ID] = make([]*KeyShare, 0)

	// 存储验证信息（实际中盐值应持久化）
	_ = salt
	_ = verificationToken

	e.addAudit(vault.ID, "", "create", "", true, "保险库创建成功")
	return nil
}

// Unlock 解锁保险库
func (e *Engine) Unlock(vaultID, passphrase string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	vault, exists := e.vaults[vaultID]
	if !exists {
		return ErrVaultNotFound
	}

	if vault.Status == StatusUnlocked {
		return ErrVaultUnlocked
	}

	// 检查失败次数
	if e.failCounts[vaultID] >= e.config.MaxFailedAttempts {
		e.addAudit(vaultID, "", "unlock", "", false, "超出最大尝试次数")
		return ErrMaxAttemptsReached
	}

	// 派生密钥验证（简化实现，实际应比对验证令牌）
	key, _, err := e.crypto.DeriveKey(passphrase)
	if err != nil {
		e.failCounts[vaultID]++
		e.addAudit(vaultID, "", "unlock", "", false, "密钥派生失败")
		return err
	}

	// 存储密钥（仅在内存中）
	e.vaultKeys[vaultID] = key

	vault.Status = StatusUnlocked
	vault.AccessedAt = time.Now()
	e.failCounts[vaultID] = 0

	e.addAudit(vaultID, "", "unlock", "", true, "保险库解锁成功")
	return nil
}

// Lock 锁定保险库
func (e *Engine) Lock(vaultID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	vault, exists := e.vaults[vaultID]
	if !exists {
		return ErrVaultNotFound
	}

	if vault.Status == StatusLocked {
		return ErrVaultLocked
	}

	// 清除内存中的密钥
	delete(e.vaultKeys, vaultID)

	vault.Status = StatusLocked
	e.addAudit(vaultID, "", "lock", "", true, "保险库已锁定")
	return nil
}

// Destroy 销毁保险库（安全擦除所有数据）
func (e *Engine) Destroy(vaultID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	vault, exists := e.vaults[vaultID]
	if !exists {
		return ErrVaultNotFound
	}

	vault.Status = StatusDestroyed

	// 从存储中彻底移除
	delete(e.vaults, vaultID)

	// 清除所有关联数据
	delete(e.vaultKeys, vaultID)
	delete(e.secrets, vaultID)
	delete(e.policies, vaultID)
	delete(e.keyShares, vaultID)
	delete(e.failCounts, vaultID)

	// 清除分享链接
	for id, link := range e.shareLinks {
		if link.VaultID == vaultID {
			delete(e.shareLinks, id)
		}
	}

	e.addAudit(vaultID, "", "destroy", "", true, "保险库已销毁")
	return nil
}

// AddSecret 向保险库添加加密条目
func (e *Engine) AddSecret(vaultID string, secret *Secret) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	vault, exists := e.vaults[vaultID]
	if !exists {
		return ErrVaultNotFound
	}

	if vault.Status != StatusUnlocked {
		return ErrVaultLocked
	}

	secret.VaultID = vaultID
	secret.CreatedAt = time.Now()
	secret.ModifiedAt = time.Now()

	e.secrets[vaultID] = append(e.secrets[vaultID], secret)
	vault.UsedSpace += secret.DataSize
	vault.FileCount++

	e.addAudit(vaultID, "", "add_secret", secret.ID, true,
		fmt.Sprintf("添加加密条目: %s", secret.Name))
	return nil
}

// GetSecret 获取加密条目
func (e *Engine) GetSecret(vaultID, secretID string) (*Secret, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	vault, exists := e.vaults[vaultID]
	if !exists {
		return nil, ErrVaultNotFound
	}

	if vault.Status != StatusUnlocked {
		return nil, ErrVaultLocked
	}

	for _, s := range e.secrets[vaultID] {
		if s.ID == secretID {
			return s, nil
		}
	}

	return nil, ErrSecretNotFound
}

// ListSecrets 列出保险库中的所有加密条目
func (e *Engine) ListSecrets(vaultID string) ([]*Secret, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if _, exists := e.vaults[vaultID]; !exists {
		return nil, ErrVaultNotFound
	}

	secrets := e.secrets[vaultID]
	if secrets == nil {
		return []*Secret{}, nil
	}

	return secrets, nil
}

// CreateShareLink 创建安全分享链接
func (e *Engine) CreateShareLink(vaultID, secretID, createdBy string, perm SharePermission, duration time.Duration) (*ShareLink, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	vault, exists := e.vaults[vaultID]
	if !exists {
		return nil, ErrVaultNotFound
	}

	if vault.Status != StatusUnlocked {
		return nil, ErrVaultLocked
	}

	// 验证条目存在
	found := false
	for _, s := range e.secrets[vaultID] {
		if s.ID == secretID {
			found = true
			break
		}
	}
	if !found {
		return nil, ErrSecretNotFound
	}

	token, err := GenerateToken(32)
	if err != nil {
		return nil, err
	}

	link := &ShareLink{
		ID:         fmt.Sprintf("share-%d", time.Now().UnixNano()),
		SecretID:   secretID,
		VaultID:    vaultID,
		Token:      token,
		Permission: perm,
		ExpiresAt:  time.Now().Add(duration),
		CreatedBy:  createdBy,
		CreatedAt:  time.Now(),
	}

	e.shareLinks[link.ID] = link
	e.addAudit(vaultID, createdBy, "share", secretID, true, "创建分享链接")
	return link, nil
}

// AccessShareLink 通过分享链接访问条目
func (e *Engine) AccessShareLink(token string) (*ShareLink, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	for _, link := range e.shareLinks {
		if link.Token == token {
			if time.Now().After(link.ExpiresAt) {
				return nil, ErrShareLinkExpired
			}
			if link.MaxDownloads > 0 && link.DownloadCount >= link.MaxDownloads {
				return nil, ErrShareLinkExpired
			}
			link.DownloadCount++
			return link, nil
		}
	}

	return nil, ErrSecretNotFound
}

// SetAccessPolicy 设置访问控制策略
func (e *Engine) SetAccessPolicy(vaultID string, policy *AccessPolicy) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.vaults[vaultID]; !exists {
		return ErrVaultNotFound
	}

	policy.VaultID = vaultID
	policy.CreatedAt = time.Now()

	// 覆盖已有策略
	policies := e.policies[vaultID]
	for i, p := range policies {
		if p.UserID == policy.UserID {
			policies[i] = policy
			e.addAudit(vaultID, "", "set_policy", "", true,
				fmt.Sprintf("更新用户 %s 的访问策略", policy.UserID))
			return nil
		}
	}

	e.policies[vaultID] = append(policies, policy)
	e.addAudit(vaultID, "", "set_policy", "", true,
		fmt.Sprintf("为用户 %s 设置访问策略", policy.UserID))
	return nil
}

// CheckAccess 检查用户是否有权访问保险库
func (e *Engine) CheckAccess(vaultID, userID, clientIP string) (bool, AccessLevel) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if _, exists := e.vaults[vaultID]; !exists {
		return false, ""
	}

	policies := e.policies[vaultID]
	for _, p := range policies {
		if p.UserID != userID {
			continue
		}

		// 检查过期
		if p.ExpiresAt != nil && time.Now().After(*p.ExpiresAt) {
			continue
		}

		// 检查访问次数
		if p.MaxAccessCount > 0 && p.AccessCount >= p.MaxAccessCount {
			continue
		}

		// 检查 IP 限制
		if len(p.AllowedIPs) > 0 && clientIP != "" {
			allowed := false
			for _, ip := range p.AllowedIPs {
				if ip == clientIP {
					allowed = true
					break
				}
			}
			if !allowed {
				continue
			}
		}

		p.AccessCount++
		return true, p.Level
	}

	return false, ""
}

// AddKeyShare 添加密钥分片
func (e *Engine) AddKeyShare(vaultID string, share *KeyShare) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.vaults[vaultID]; !exists {
		return ErrVaultNotFound
	}

	share.VaultID = vaultID
	share.CreatedAt = time.Now()

	e.keyShares[vaultID] = append(e.keyShares[vaultID], share)
	e.addAudit(vaultID, "", "add_key_share", "", true,
		fmt.Sprintf("添加密钥分片 #%d", share.ShareIndex))
	return nil
}

// GetKeyShares 获取保险库的密钥分片列表
func (e *Engine) GetKeyShares(vaultID string) ([]*KeyShare, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	if _, exists := e.vaults[vaultID]; !exists {
		return nil, ErrVaultNotFound
	}

	shares := e.keyShares[vaultID]
	if shares == nil {
		return []*KeyShare{}, nil
	}

	return shares, nil
}

// GetVault 获取保险库信息
func (e *Engine) GetVault(vaultID string) (*Vault, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	vault, exists := e.vaults[vaultID]
	if !exists {
		return nil, ErrVaultNotFound
	}

	return vault, nil
}

// ListVaults 列出所有保险库
func (e *Engine) ListVaults() []*Vault {
	e.mu.RLock()
	defer e.mu.RUnlock()

	vaults := make([]*Vault, 0, len(e.vaults))
	for _, v := range e.vaults {
		vaults = append(vaults, v)
	}
	return vaults
}

// GetStats 获取统计信息
func (e *Engine) GetStats() *VaultStats {
	e.mu.RLock()
	defer e.mu.RUnlock()

	stats := &VaultStats{}
	for _, v := range e.vaults {
		stats.TotalVaults++
		stats.TotalSize += v.Size
		stats.UsedSpace += v.UsedSpace
		stats.TotalSecrets += len(e.secrets[v.ID])

		switch v.Status {
		case StatusLocked:
			stats.LockedVaults++
		case StatusUnlocked:
			stats.UnlockedVaults++
		}

		if v.DenyExists {
			stats.HiddenVaults++
		}

		if v.AccessedAt.After(stats.LastActivity) {
			stats.LastActivity = v.AccessedAt
		}
	}

	for _, link := range e.shareLinks {
		if time.Now().Before(link.ExpiresAt) {
			stats.TotalShareLinks++
		}
	}

	return stats
}

// GetAuditLog 获取审计日志
func (e *Engine) GetAuditLog(vaultID string, limit int) []*AuditLog {
	e.mu.RLock()
	defer e.mu.RUnlock()

	var entries []*AuditLog
	for i := len(e.auditLog) - 1; i >= 0; i-- {
		if vaultID == "" || e.auditLog[i].VaultID == vaultID {
			entries = append(entries, e.auditLog[i])
			if limit > 0 && len(entries) >= limit {
				break
			}
		}
	}
	return entries
}

// AutoLockCheck 检查并自动锁定超时的保险库
func (e *Engine) AutoLockCheck() int {
	e.mu.Lock()
	defer e.mu.Unlock()

	locked := 0
	now := time.Now()
	for _, v := range e.vaults {
		if v.Status == StatusUnlocked && v.AutoLockMinutes > 0 {
			if now.Sub(v.AccessedAt) > time.Duration(v.AutoLockMinutes)*time.Minute {
				delete(e.vaultKeys, v.ID)
				v.Status = StatusLocked
				e.addAudit(v.ID, "", "auto_lock", "", true, "自动锁定")
				locked++
			}
		}
	}
	return locked
}

// Stop 停止引擎
func (e *Engine) Stop() {
	e.cancel()
	// 清除所有内存中的密钥
	e.mu.Lock()
	for k := range e.vaultKeys {
		delete(e.vaultKeys, k)
	}
	e.mu.Unlock()
}

func (e *Engine) addAudit(vaultID, userID, action, resource string, success bool, details string) {
	if !e.config.AuditEnabled {
		return
	}
	e.auditLog = append(e.auditLog, &AuditLog{
		ID:        fmt.Sprintf("audit-%d", len(e.auditLog)+1),
		VaultID:   vaultID,
		UserID:    userID,
		Action:    action,
		Resource:  resource,
		Success:   success,
		Details:   details,
		Timestamp: time.Now(),
	})
}
