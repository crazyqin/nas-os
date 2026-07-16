// Package fido2 提供 FIDO2/WebAuthn 凭据存储管理功能
package fido2

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sort"
	"sync"
	"time"
)

// CredentialStore 凭据存储接口.
type CredentialStore interface {
	// SaveCredential 保存凭据
	SaveCredential(cred *Credential) error

	// GetCredential 根据 ID 获取凭据
	GetCredential(id string) (*Credential, error)

	// GetCredentialByWebAuthnID 根据 WebAuthn 凭据 ID 获取凭据
	GetCredentialByWebAuthnID(webauthnID []byte) (*Credential, error)

	// GetUserCredentials 获取用户的所有凭据
	GetUserCredentials(userID string) ([]*Credential, error)

	// DeleteCredential 删除凭据
	DeleteCredential(id string) error

	// UpdateCredential 更新凭据
	UpdateCredential(cred *Credential) error

	// ListCredentials 列出所有凭据
	ListCredentials() ([]*Credential, error)
}

// MemoryCredentialStore 内存凭据存储（用于测试和演示）.
type MemoryCredentialStore struct {
	mu          sync.RWMutex
	credentials map[string]*Credential
}

// NewMemoryCredentialStore 创建内存凭据存储.
func NewMemoryCredentialStore() *MemoryCredentialStore {
	return &MemoryCredentialStore{
		credentials: make(map[string]*Credential),
	}
}

// SaveCredential 保存凭据.
func (s *MemoryCredentialStore) SaveCredential(cred *Credential) error {
	if cred == nil {
		return fmt.Errorf("凭据不能为空")
	}
	if cred.ID == "" {
		return fmt.Errorf("凭据 ID 不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 检查是否已存在
	if _, exists := s.credentials[cred.ID]; exists {
		return fmt.Errorf("凭据已存在: %s", cred.ID)
	}

	s.credentials[cred.ID] = cred
	return nil
}

// GetCredential 根据 ID 获取凭据.
func (s *MemoryCredentialStore) GetCredential(id string) (*Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cred, exists := s.credentials[id]
	if !exists {
		return nil, fmt.Errorf("凭据不存在: %s", id)
	}

	// 返回副本以避免并发修改
	credCopy := *cred
	return &credCopy, nil
}

// GetCredentialByWebAuthnID 根据 WebAuthn 凭据 ID 获取凭据.
func (s *MemoryCredentialStore) GetCredentialByWebAuthnID(webauthnID []byte) (*Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	for _, cred := range s.credentials {
		if bytesEqual(cred.CredentialID, webauthnID) {
			credCopy := *cred
			return &credCopy, nil
		}
	}

	return nil, fmt.Errorf("凭据不存在")
}

// GetUserCredentials 获取用户的所有凭据.
func (s *MemoryCredentialStore) GetUserCredentials(userID string) ([]*Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var creds []*Credential
	for _, cred := range s.credentials {
		if cred.UserID == userID {
			credCopy := *cred
			creds = append(creds, &credCopy)
		}
	}

	// 按创建时间排序
	sort.Slice(creds, func(i, j int) bool {
		return creds[i].CreatedAt.Before(creds[j].CreatedAt)
	})

	return creds, nil
}

// DeleteCredential 删除凭据.
func (s *MemoryCredentialStore) DeleteCredential(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.credentials[id]; !exists {
		return fmt.Errorf("凭据不存在: %s", id)
	}

	delete(s.credentials, id)
	return nil
}

// UpdateCredential 更新凭据.
func (s *MemoryCredentialStore) UpdateCredential(cred *Credential) error {
	if cred == nil {
		return fmt.Errorf("凭据不能为空")
	}
	if cred.ID == "" {
		return fmt.Errorf("凭据 ID 不能为空")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.credentials[cred.ID]; !exists {
		return fmt.Errorf("凭据不存在: %s", cred.ID)
	}

	s.credentials[cred.ID] = cred
	return nil
}

// ListCredentials 列出所有凭据.
func (s *MemoryCredentialStore) ListCredentials() ([]*Credential, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	creds := make([]*Credential, 0, len(s.credentials))
	for _, cred := range s.credentials {
		credCopy := *cred
		creds = append(creds, &credCopy)
	}

	// 按创建时间排序
	sort.Slice(creds, func(i, j int) bool {
		return creds[i].CreatedAt.Before(creds[j].CreatedAt)
	})

	return creds, nil
}

// ==================== 凭据管理器 ====================

// CredentialManager 凭据管理器.
type CredentialManager struct {
	store         CredentialStore
	authenticator *Authenticator
	config        *Config
}

// NewCredentialManager 创建凭据管理器.
func NewCredentialManager(store CredentialStore, authenticator *Authenticator, config *Config) *CredentialManager {
	if store == nil {
		store = NewMemoryCredentialStore()
	}
	if authenticator == nil {
		authenticator = NewAuthenticator(config)
	}
	if config == nil {
		config = DefaultConfig()
	}

	return &CredentialManager{
		store:         store,
		authenticator: authenticator,
		config:        config,
	}
}

// RegisterCredential 注册新凭据.
func (m *CredentialManager) RegisterCredential(
	userID, name string,
	resp *RegistrationResponse,
	challenge string,
) (*Credential, error) {
	// 验证注册响应
	cred, err := m.authenticator.VerifyRegistration(resp, challenge)
	if err != nil {
		return nil, fmt.Errorf("验证注册响应失败: %w", err)
	}

	// 设置凭据属性
	cred.UserID = userID
	cred.Name = name

	// 检查用户凭据数量限制
	existingCreds, err := m.store.GetUserCredentials(userID)
	if err != nil {
		return nil, fmt.Errorf("获取用户凭据失败: %w", err)
	}

	if len(existingCreds) >= m.config.MaxCredentials {
		return nil, fmt.Errorf("已达到最大凭据数量限制: %d", m.config.MaxCredentials)
	}

	// 保存凭据
	if err := m.store.SaveCredential(cred); err != nil {
		return nil, fmt.Errorf("保存凭据失败: %w", err)
	}

	return cred, nil
}

// GetCredential 获取凭据.
func (m *CredentialManager) GetCredential(id string) (*Credential, error) {
	return m.store.GetCredential(id)
}

// GetUserCredentials 获取用户的所有凭据.
func (m *CredentialManager) GetUserCredentials(userID string) ([]*Credential, error) {
	return m.store.GetUserCredentials(userID)
}

// GetUserCredentialInfos 获取用户的凭据简要信息列表.
func (m *CredentialManager) GetUserCredentialInfos(userID string) ([]CredentialInfo, error) {
	creds, err := m.store.GetUserCredentials(userID)
	if err != nil {
		return nil, err
	}

	infos := make([]CredentialInfo, len(creds))
	for i, cred := range creds {
		infos[i] = CredentialInfo{
			ID:            cred.ID,
			Name:          cred.Name,
			Authenticator: cred.Authenticator,
			Transports:    cred.Transports,
			CreatedAt:     cred.CreatedAt,
			LastUsedAt:    cred.LastUsedAt,
			UsageCount:    cred.UsageCount,
			Revoked:       cred.Revoked,
		}
	}

	return infos, nil
}

// DeleteCredential 删除凭据.
func (m *CredentialManager) DeleteCredential(id string) error {
	return m.store.DeleteCredential(id)
}

// RenameCredential 重命名凭据.
func (m *CredentialManager) RenameCredential(id, newName string) error {
	cred, err := m.store.GetCredential(id)
	if err != nil {
		return err
	}

	cred.Name = newName
	return m.store.UpdateCredential(cred)
}

// RevokeCredential 吊销凭据.
func (m *CredentialManager) RevokeCredential(id string) error {
	cred, err := m.store.GetCredential(id)
	if err != nil {
		return err
	}

	cred.Revoked = true
	now := time.Now()
	cred.RevokedAt = &now
	return m.store.UpdateCredential(cred)
}

// UpdateCredentialUsage 更新凭据使用信息.
func (m *CredentialManager) UpdateCredentialUsage(id string, signCount uint32) error {
	cred, err := m.store.GetCredential(id)
	if err != nil {
		return err
	}

	cred.SignCount = signCount
	cred.LastUsedAt = time.Now()
	cred.UsageCount++
	return m.store.UpdateCredential(cred)
}

// FindCredentialByWebAuthnID 根据 WebAuthn 凭据 ID 查找凭据.
func (m *CredentialManager) FindCredentialByWebAuthnID(webauthnID []byte) (*Credential, error) {
	return m.store.GetCredentialByWebAuthnID(webauthnID)
}

// GetActiveUserCredentials 获取用户的活跃（未吊销）凭据.
func (m *CredentialManager) GetActiveUserCredentials(userID string) ([]*Credential, error) {
	allCreds, err := m.store.GetUserCredentials(userID)
	if err != nil {
		return nil, err
	}

	var activeCreds []*Credential
	for _, cred := range allCreds {
		if !cred.Revoked {
			activeCreds = append(activeCreds, cred)
		}
	}

	return activeCreds, nil
}

// ==================== 恢复码管理 ====================

// RecoveryCodeStore 恢复码存储接口.
type RecoveryCodeStore interface {
	// SaveRecoveryCode 保存恢复码
	SaveRecoveryCode(code *RecoveryCode) error

	// GetRecoveryCode 根据 ID 获取恢复码
	GetRecoveryCode(id string) (*RecoveryCode, error)

	// GetUserRecoveryCodes 获取用户的所有恢复码
	GetUserRecoveryCodes(userID string) ([]*RecoveryCode, error)

	// GetUnusedUserRecoveryCodes 获取用户的未使用恢复码
	GetUnusedUserRecoveryCodes(userID string) ([]*RecoveryCode, error)

	// UpdateRecoveryCode 更新恢复码
	UpdateRecoveryCode(code *RecoveryCode) error

	// DeleteUserRecoveryCodes 删除用户的所有恢复码
	DeleteUserRecoveryCodes(userID string) error
}

// MemoryRecoveryCodeStore 内存恢复码存储.
type MemoryRecoveryCodeStore struct {
	mu    sync.RWMutex
	codes map[string]*RecoveryCode
}

// NewMemoryRecoveryCodeStore 创建内存恢复码存储.
func NewMemoryRecoveryCodeStore() *MemoryRecoveryCodeStore {
	return &MemoryRecoveryCodeStore{
		codes: make(map[string]*RecoveryCode),
	}
}

// SaveRecoveryCode 保存恢复码.
func (s *MemoryRecoveryCodeStore) SaveRecoveryCode(code *RecoveryCode) error {
	if code == nil || code.ID == "" {
		return fmt.Errorf("恢复码无效")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.codes[code.ID] = code
	return nil
}

// GetRecoveryCode 根据 ID 获取恢复码.
func (s *MemoryRecoveryCodeStore) GetRecoveryCode(id string) (*RecoveryCode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	code, exists := s.codes[id]
	if !exists {
		return nil, fmt.Errorf("恢复码不存在: %s", id)
	}

	codeCopy := *code
	return &codeCopy, nil
}

// GetUserRecoveryCodes 获取用户的所有恢复码.
func (s *MemoryRecoveryCodeStore) GetUserRecoveryCodes(userID string) ([]*RecoveryCode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var codes []*RecoveryCode
	for _, code := range s.codes {
		if code.UserID == userID {
			codeCopy := *code
			codes = append(codes, &codeCopy)
		}
	}

	return codes, nil
}

// GetUnusedUserRecoveryCodes 获取用户的未使用恢复码.
func (s *MemoryRecoveryCodeStore) GetUnusedUserRecoveryCodes(userID string) ([]*RecoveryCode, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var codes []*RecoveryCode
	for _, code := range s.codes {
		if code.UserID == userID && !code.Used {
			codeCopy := *code
			codes = append(codes, &codeCopy)
		}
	}

	return codes, nil
}

// UpdateRecoveryCode 更新恢复码.
func (s *MemoryRecoveryCodeStore) UpdateRecoveryCode(code *RecoveryCode) error {
	if code == nil || code.ID == "" {
		return fmt.Errorf("恢复码无效")
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if _, exists := s.codes[code.ID]; !exists {
		return fmt.Errorf("恢复码不存在: %s", code.ID)
	}

	s.codes[code.ID] = code
	return nil
}

// DeleteUserRecoveryCodes 删除用户的所有恢复码.
func (s *MemoryRecoveryCodeStore) DeleteUserRecoveryCodes(userID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, code := range s.codes {
		if code.UserID == userID {
			delete(s.codes, id)
		}
	}

	return nil
}

// RecoveryCodeManager 恢复码管理器.
type RecoveryCodeManager struct {
	store         RecoveryCodeStore
	authenticator *Authenticator
	config        *Config
}

// NewRecoveryCodeManager 创建恢复码管理器.
func NewRecoveryCodeManager(store RecoveryCodeStore, authenticator *Authenticator, config *Config) *RecoveryCodeManager {
	if store == nil {
		store = NewMemoryRecoveryCodeStore()
	}
	if authenticator == nil {
		authenticator = NewAuthenticator(config)
	}
	if config == nil {
		config = DefaultConfig()
	}

	return &RecoveryCodeManager{
		store:         store,
		authenticator: authenticator,
		config:        config,
	}
}

// GenerateRecoveryCodes 生成恢复码.
func (m *RecoveryCodeManager) GenerateRecoveryCodes(userID string, count int) ([]string, error) {
	if count <= 0 || count > 10 {
		return nil, fmt.Errorf("恢复码数量必须在 1-10 之间")
	}

	// 删除旧的恢复码
	if err := m.store.DeleteUserRecoveryCodes(userID); err != nil {
		return nil, fmt.Errorf("删除旧恢复码失败: %w", err)
	}

	codes := make([]string, count)
	for i := 0; i < count; i++ {
		code, hash, err := m.authenticator.GenerateRecoveryCode()
		if err != nil {
			return nil, fmt.Errorf("生成恢复码失败: %w", err)
		}

		// 生成恢复码 ID
		codeID := make([]byte, 16)
		if _, err := rand.Read(codeID); err != nil {
			return nil, fmt.Errorf("生成恢复码 ID 失败: %w", err)
		}

		recoveryCode := &RecoveryCode{
			ID:        base64URLEncode(codeID),
			UserID:    userID,
			Code:      hash,
			Used:      false,
			CreatedAt: time.Now(),
			ExpiresAt: time.Now().Add(365 * 24 * time.Hour), // 1年有效期
		}

		if err := m.store.SaveRecoveryCode(recoveryCode); err != nil {
			return nil, fmt.Errorf("保存恢复码失败: %w", err)
		}

		codes[i] = code
	}

	return codes, nil
}

// VerifyRecoveryCode 验证恢复码.
func (m *RecoveryCodeManager) VerifyRecoveryCode(userID, inputCode string) (bool, error) {
	// 获取用户的所有未使用恢复码
	codes, err := m.store.GetUnusedUserRecoveryCodes(userID)
	if err != nil {
		return false, fmt.Errorf("获取恢复码失败: %w", err)
	}

	// 遍历验证
	for _, code := range codes {
		// 检查是否过期
		if time.Now().After(code.ExpiresAt) {
			continue
		}

		// 验证恢复码
		if m.authenticator.VerifyRecoveryCode(inputCode, code.Code) {
			// 标记为已使用
			code.Used = true
			now := time.Now()
			code.UsedAt = &now

			if err := m.store.UpdateRecoveryCode(code); err != nil {
				return false, fmt.Errorf("更新恢复码状态失败: %w", err)
			}

			return true, nil
		}
	}

	return false, nil
}

// GetUserRecoveryCodeInfos 获取用户的恢复码信息.
func (m *RecoveryCodeManager) GetUserRecoveryCodeInfos(userID string) ([]RecoveryCodeInfo, error) {
	codes, err := m.store.GetUserRecoveryCodes(userID)
	if err != nil {
		return nil, err
	}

	infos := make([]RecoveryCodeInfo, len(codes))
	for i, code := range codes {
		infos[i] = RecoveryCodeInfo{
			ID:        code.ID,
			Used:      code.Used,
			UsedAt:    code.UsedAt,
			CreatedAt: code.CreatedAt,
			ExpiresAt: code.ExpiresAt,
		}
	}

	return infos, nil
}

// base64URLEncode Base64 URL 编码.
func base64URLEncode(data []byte) string {
	return base64.URLEncoding.EncodeToString(data)
}
