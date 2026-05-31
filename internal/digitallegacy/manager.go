// Package digitallegacy 提供数字遗产核心管理逻辑
package digitallegacy

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 数字遗产管理器
type Manager struct {
	mu              sync.RWMutex
	logger          *zap.Logger
	config          *DefaultLegacyConfig
	plans           map[string]*LegacyPlan
	contacts        map[string]*TrustContact
	auditLogs       []*AuditLog
	inactivityChecks []*InactivityCheck
	accessGrants    []*AccessGrant
	encryptionKey   []byte
	stopChan        chan struct{}
	running         bool
}

// NewManager 创建数字遗产管理器
func NewManager(logger *zap.Logger, config *DefaultLegacyConfig, encryptionKey []byte) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = GetDefaultConfig()
	}

	m := &Manager{
		logger:           logger,
		config:           config,
		plans:            make(map[string]*LegacyPlan),
		contacts:         make(map[string]*TrustContact),
		auditLogs:        make([]*AuditLog, 0),
		inactivityChecks: make([]*InactivityCheck, 0),
		accessGrants:     make([]*AccessGrant, 0),
		encryptionKey:    encryptionKey,
		stopChan:         make(chan struct{}),
	}

	return m
}

// generateID 生成唯一 ID
func generateID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return hex.EncodeToString(b)
}

// encryptData 加密数据
func (m *Manager) encryptData(data string) (string, error) {
	if m.encryptionKey == nil {
		return "", fmt.Errorf("encryption key not set")
	}

	block, err := aes.NewCipher(m.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nonce, nonce, []byte(data), nil)
	return hex.EncodeToString(ciphertext), nil
}

// decryptData 解密数据
func (m *Manager) decryptData(encryptedData string) (string, error) {
	if m.encryptionKey == nil {
		return "", fmt.Errorf("encryption key not set")
	}

	data, err := hex.DecodeString(encryptedData)
	if err != nil {
		return "", err
	}

	block, err := aes.NewCipher(m.encryptionKey)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return "", fmt.Errorf("ciphertext too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}

	return string(plaintext), nil
}

// hashData 计算数据哈希
func hashData(data string) string {
	h := sha256.Sum256([]byte(data))
	return hex.EncodeToString(h[:])
}

// CreatePlan 创建遗产计划
func (m *Manager) CreatePlan(req *LegacyPlanRequest, ownerID string) (*LegacyPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !IsValidTriggerType(req.TriggerType) {
		return nil, fmt.Errorf("invalid trigger type: %s", req.TriggerType)
	}

	plan := &LegacyPlan{
		ID:          generateID(),
		Name:        req.Name,
		Description: req.Description,
		OwnerID:     ownerID,
		Status:      LegacyStatusDraft,
		TriggerType: req.TriggerType,
		TriggerConditions: req.TriggerConditions,
		IsEncrypted: req.IsEncrypted,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 设置默认触发条件
	if plan.TriggerConditions == nil {
		plan.TriggerConditions = &TriggerConditions{
			InactivityDays: m.config.InactivityDays,
			GracePeriodDays: m.config.GracePeriodDays,
			RequiredWitnesses: m.config.RequiredWitnesses,
		}
	}

	m.plans[plan.ID] = plan

	m.logger.Info("legacy plan created",
		zap.String("plan_id", plan.ID),
		zap.String("name", plan.Name),
		zap.String("owner_id", ownerID))

	return plan, nil
}

// GetPlan 获取遗产计划
func (m *Manager) GetPlan(id string) (*LegacyPlan, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[id]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", id)
	}

	return plan, nil
}

// ListPlans 列出所有遗产计划
func (m *Manager) ListPlans(ownerID string) []*LegacyPlan {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plans := make([]*LegacyPlan, 0)
	for _, p := range m.plans {
		if p.OwnerID == ownerID {
			plans = append(plans, p)
		}
	}
	return plans
}

// UpdatePlan 更新遗产计划
func (m *Manager) UpdatePlan(id string, req *LegacyPlanRequest) (*LegacyPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[id]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", id)
	}

	if plan.Status == LegacyStatusTriggered || plan.Status == LegacyStatusCompleted {
		return nil, fmt.Errorf("cannot update plan in %s status", plan.Status)
	}

	plan.Name = req.Name
	plan.Description = req.Description
	plan.TriggerType = req.TriggerType
	plan.TriggerConditions = req.TriggerConditions
	plan.IsEncrypted = req.IsEncrypted
	plan.UpdatedAt = time.Now()

	m.logger.Info("legacy plan updated", zap.String("plan_id", id))
	return plan, nil
}

// DeletePlan 删除遗产计划
func (m *Manager) DeletePlan(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[id]
	if !ok {
		return fmt.Errorf("plan not found: %s", id)
	}

	if plan.Status == LegacyStatusTriggered || plan.Status == LegacyStatusCompleted {
		return fmt.Errorf("cannot delete plan in %s status", plan.Status)
	}

	delete(m.plans, id)

	m.logger.Info("legacy plan deleted", zap.String("plan_id", id))
	return nil
}

// ActivatePlan 激活遗产计划
func (m *Manager) ActivatePlan(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[id]
	if !ok {
		return fmt.Errorf("plan not found: %s", id)
	}

	if plan.Status != LegacyStatusDraft {
		return fmt.Errorf("plan must be in draft status to activate")
	}

	if len(plan.Beneficiaries) == 0 {
		return fmt.Errorf("plan must have at least one beneficiary")
	}

	if len(plan.Assets) == 0 {
		return fmt.Errorf("plan must have at least one asset")
	}

	// 验证分配比例总和
	totalPercent := 0
	for _, b := range plan.Beneficiaries {
		totalPercent += b.AllocationPercent
	}
	if totalPercent != 100 {
		return fmt.Errorf("allocation percentages must sum to 100, got %d", totalPercent)
	}

	plan.Status = LegacyStatusActive
	plan.UpdatedAt = time.Now()

	m.logger.Info("legacy plan activated", zap.String("plan_id", id))
	return nil
}

// AddBeneficiary 添加受益人
func (m *Manager) AddBeneficiary(planID string, req *BeneficiaryRequest) (*Beneficiary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	if !IsValidContactRole(req.Role) {
		return nil, fmt.Errorf("invalid contact role: %s", req.Role)
	}

	if !IsValidAccessLevel(req.AccessLevel) {
		return nil, fmt.Errorf("invalid access level: %s", req.AccessLevel)
	}

	if len(plan.Beneficiaries) >= m.config.MaxBeneficiaries {
		return nil, fmt.Errorf("maximum number of beneficiaries reached")
	}

	beneficiary := &Beneficiary{
		ID:               generateID(),
		PlanID:           planID,
		Name:             req.Name,
		Email:            req.Email,
		Phone:            req.Phone,
		Relationship:     req.Relationship,
		Role:             req.Role,
		AllocationPercent: req.AllocationPercent,
		AccessLevel:      req.AccessLevel,
		CreatedAt:        time.Now(),
		UpdatedAt:        time.Now(),
	}

	plan.Beneficiaries = append(plan.Beneficiaries, beneficiary)
	plan.UpdatedAt = time.Now()

	m.logger.Info("beneficiary added",
		zap.String("plan_id", planID),
		zap.String("beneficiary_id", beneficiary.ID))

	return beneficiary, nil
}

// RemoveBeneficiary 移除受益人
func (m *Manager) RemoveBeneficiary(planID, beneficiaryID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return fmt.Errorf("plan not found: %s", planID)
	}

	for i, b := range plan.Beneficiaries {
		if b.ID == beneficiaryID {
			plan.Beneficiaries = append(plan.Beneficiaries[:i], plan.Beneficiaries[i+1:]...)
			plan.UpdatedAt = time.Now()
			m.logger.Info("beneficiary removed",
				zap.String("plan_id", planID),
				zap.String("beneficiary_id", beneficiaryID))
			return nil
		}
	}

	return fmt.Errorf("beneficiary not found: %s", beneficiaryID)
}

// UpdateBeneficiary 更新受益人
func (m *Manager) UpdateBeneficiary(planID, beneficiaryID string, req *BeneficiaryRequest) (*Beneficiary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	for _, b := range plan.Beneficiaries {
		if b.ID == beneficiaryID {
			b.Name = req.Name
			b.Email = req.Email
			b.Phone = req.Phone
			b.Relationship = req.Relationship
			b.Role = req.Role
			b.AllocationPercent = req.AllocationPercent
			b.AccessLevel = req.AccessLevel
			b.UpdatedAt = time.Now()
			plan.UpdatedAt = time.Now()

			m.logger.Info("beneficiary updated",
				zap.String("plan_id", planID),
				zap.String("beneficiary_id", beneficiaryID))
			return b, nil
		}
	}

	return nil, fmt.Errorf("beneficiary not found: %s", beneficiaryID)
}

// AddAsset 添加数字资产
func (m *Manager) AddAsset(planID string, req *AssetRequest) (*DigitalAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	if !IsValidAssetType(req.Type) {
		return nil, fmt.Errorf("invalid asset type: %s", req.Type)
	}

	if len(plan.Assets) >= m.config.MaxAssets {
		return nil, fmt.Errorf("maximum number of assets reached")
	}

	asset := &DigitalAsset{
		ID:          generateID(),
		PlanID:      planID,
		Name:        req.Name,
		Type:        req.Type,
		Description: req.Description,
		Value:       req.Value,
		Notes:       req.Notes,
		AssignedTo:  req.AssignedTo,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 加密敏感数据
	if req.Data != "" && plan.IsEncrypted {
		encryptedData, err := m.encryptData(req.Data)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt data: %w", err)
		}
		asset.EncryptedData = encryptedData
		asset.DataHash = hashData(req.Data)
		asset.IsEncrypted = true
	} else {
		asset.Value = req.Data
	}

	plan.Assets = append(plan.Assets, asset)
	plan.UpdatedAt = time.Now()

	m.logger.Info("asset added",
		zap.String("plan_id", planID),
		zap.String("asset_id", asset.ID),
		zap.String("type", string(asset.Type)))

	return asset, nil
}

// RemoveAsset 移除数字资产
func (m *Manager) RemoveAsset(planID, assetID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return fmt.Errorf("plan not found: %s", planID)
	}

	for i, a := range plan.Assets {
		if a.ID == assetID {
			plan.Assets = append(plan.Assets[:i], plan.Assets[i+1:]...)
			plan.UpdatedAt = time.Now()
			m.logger.Info("asset removed",
				zap.String("plan_id", planID),
				zap.String("asset_id", assetID))
			return nil
		}
	}

	return fmt.Errorf("asset not found: %s", assetID)
}

// UpdateAsset 更新数字资产
func (m *Manager) UpdateAsset(planID, assetID string, req *AssetRequest) (*DigitalAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	for _, a := range plan.Assets {
		if a.ID == assetID {
			a.Name = req.Name
			a.Type = req.Type
			a.Description = req.Description
			a.Notes = req.Notes
			a.AssignedTo = req.AssignedTo
			a.UpdatedAt = time.Now()

			// 加密敏感数据
			if req.Data != "" && plan.IsEncrypted {
				encryptedData, err := m.encryptData(req.Data)
				if err != nil {
					return nil, fmt.Errorf("failed to encrypt data: %w", err)
				}
				a.EncryptedData = encryptedData
				a.DataHash = hashData(req.Data)
				a.IsEncrypted = true
			} else if req.Data != "" {
				a.Value = req.Data
			}

			plan.UpdatedAt = time.Now()

			m.logger.Info("asset updated",
				zap.String("plan_id", planID),
				zap.String("asset_id", assetID))
			return a, nil
		}
	}

	return nil, fmt.Errorf("asset not found: %s", assetID)
}

// AddEmergencyContact 添加紧急联系人
func (m *Manager) AddEmergencyContact(planID string, req *EmergencyContactRequest) (*EmergencyContact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	contact := &EmergencyContact{
		ID:              generateID(),
		PlanID:          planID,
		Name:            req.Name,
		Email:           req.Email,
		Phone:           req.Phone,
		Relationship:    req.Relationship,
		Role:            req.Role,
		IsPrimary:       req.IsPrimary,
		CanTriggerPlan:  req.CanTriggerPlan,
		NotifyOnTrigger: req.NotifyOnTrigger,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	plan.EmergencyContacts = append(plan.EmergencyContacts, contact)
	plan.UpdatedAt = time.Now()

	m.logger.Info("emergency contact added",
		zap.String("plan_id", planID),
		zap.String("contact_id", contact.ID))

	return contact, nil
}

// RemoveEmergencyContact 移除紧急联系人
func (m *Manager) RemoveEmergencyContact(planID, contactID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return fmt.Errorf("plan not found: %s", planID)
	}

	for i, c := range plan.EmergencyContacts {
		if c.ID == contactID {
			plan.EmergencyContacts = append(plan.EmergencyContacts[:i], plan.EmergencyContacts[i+1:]...)
			plan.UpdatedAt = time.Now()
			m.logger.Info("emergency contact removed",
				zap.String("plan_id", planID),
				zap.String("contact_id", contactID))
			return nil
		}
	}

	return fmt.Errorf("emergency contact not found: %s", contactID)
}

// SetWillDocument 设置遗嘱文档
func (m *Manager) SetWillDocument(planID string, req *WillDocumentRequest) (*WillDocument, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	doc := &WillDocument{
		ID:        generateID(),
		PlanID:    planID,
		Title:     req.Title,
		Content:   req.Content,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	// 加密遗嘱内容
	if plan.IsEncrypted {
		encryptedContent, err := m.encryptData(req.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt will document: %w", err)
		}
		doc.EncryptedContent = encryptedContent
		doc.Content = ""
		doc.IsEncrypted = true
	}

	doc.FileHash = hashData(req.Content)

	plan.WillDocument = doc
	plan.UpdatedAt = time.Now()

	m.logger.Info("will document set",
		zap.String("plan_id", planID),
		zap.String("doc_id", doc.ID))

	return doc, nil
}

// GetWillDocument 获取遗嘱文档
func (m *Manager) GetWillDocument(planID string, decrypt bool) (*WillDocument, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	if plan.WillDocument == nil {
		return nil, fmt.Errorf("no will document for plan: %s", planID)
	}

	doc := *plan.WillDocument

	if decrypt && doc.IsEncrypted {
		content, err := m.decryptData(doc.EncryptedContent)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt will document: %w", err)
		}
		doc.Content = content
	}

	return &doc, nil
}

// TriggerPlan 触发遗产计划
func (m *Manager) TriggerPlan(ctx context.Context, planID string, req *TriggerRequest) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return fmt.Errorf("plan not found: %s", planID)
	}

	if plan.Status != LegacyStatusActive {
		return fmt.Errorf("plan must be active to trigger")
	}

	// 验证紧急代码
	if plan.TriggerConditions.EmergencyCode != "" && req.EmergencyCode != plan.TriggerConditions.EmergencyCode {
		return fmt.Errorf("invalid emergency code")
	}

	now := time.Now()
	plan.Status = LegacyStatusTriggered
	plan.TriggeredAt = &now
	plan.UpdatedAt = now

	// 生成访问授权
	for _, beneficiary := range plan.Beneficiaries {
		for _, asset := range plan.Assets {
			// 检查受益人是否被分配给该资产
			assigned := false
			for _, assignedID := range asset.AssignedTo {
				if assignedID == beneficiary.ID {
					assigned = true
					break
				}
			}

			if assigned || len(asset.AssignedTo) == 0 {
				grant := &AccessGrant{
					ID:            generateID(),
					PlanID:        planID,
					BeneficiaryID: beneficiary.ID,
					AssetID:       asset.ID,
					AccessLevel:   beneficiary.AccessLevel,
					GrantedAt:     now,
					IsActive:      true,
				}

				m.accessGrants = append(m.accessGrants, grant)

				m.logger.Info("access granted",
					zap.String("plan_id", planID),
					zap.String("beneficiary_id", beneficiary.ID),
					zap.String("asset_id", asset.ID))
			}
		}
	}

	// 记录审计日志
	m.addAuditLog(&AuditLog{
		ID:        generateID(),
		PlanID:    planID,
		UserID:    "system",
		Action:    "plan_triggered",
		Resource:  "legacy_plan",
		Details:   fmt.Sprintf("Plan %s triggered", planID),
		CreatedAt: now,
	})

	m.logger.Info("legacy plan triggered",
		zap.String("plan_id", planID),
		zap.String("trigger_type", string(plan.TriggerType)))

	return nil
}

// CheckInactivity 检查不活跃状态
func (m *Manager) CheckInactivity(ctx context.Context) []*InactivityCheck {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	checks := make([]*InactivityCheck, 0)

	for _, plan := range m.plans {
		if plan.Status != LegacyStatusActive {
			continue
		}

		if plan.TriggerType != TriggerInactivity {
			continue
		}

		// 计算不活跃天数
		lastActive := plan.TriggerConditions.LastActiveAt
		if lastActive == nil {
			continue
		}

		daysInactive := int(now.Sub(*lastActive).Hours() / 24)

		check := &InactivityCheck{
			ID:           generateID(),
			PlanID:       plan.ID,
			OwnerID:      plan.OwnerID,
			LastActive:   *lastActive,
			DaysInactive: daysInactive,
			CheckedAt:    now,
		}

		if daysInactive >= plan.TriggerConditions.InactivityDays {
			check.IsTriggered = true

			// 自动触发计划
			plan.Status = LegacyStatusTriggered
			plan.TriggeredAt = &now
			plan.UpdatedAt = now

			m.logger.Warn("plan triggered due to inactivity",
				zap.String("plan_id", plan.ID),
				zap.Int("days_inactive", daysInactive))
		}

		checks = append(checks, check)
		m.inactivityChecks = append(m.inactivityChecks, check)
	}

	return checks
}

// GetAccessGrants 获取访问授权
func (m *Manager) GetAccessGrants(planID, beneficiaryID string) []*AccessGrant {
	m.mu.RLock()
	defer m.mu.RUnlock()

	grants := make([]*AccessGrant, 0)
	for _, g := range m.accessGrants {
		if g.PlanID == planID && g.BeneficiaryID == beneficiaryID && g.IsActive {
			grants = append(grants, g)
		}
	}
	return grants
}

// RevokeAccessGrant 撤销访问授权
func (m *Manager) RevokeAccessGrant(grantID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for _, g := range m.accessGrants {
		if g.ID == grantID {
			now := time.Now()
			g.IsActive = false
			g.RevokedAt = &now
			m.logger.Info("access grant revoked", zap.String("grant_id", grantID))
			return nil
		}
	}

	return fmt.Errorf("access grant not found: %s", grantID)
}

// DecryptAssetData 解密资产数据
func (m *Manager) DecryptAssetData(planID, assetID string) (string, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[planID]
	if !ok {
		return "", fmt.Errorf("plan not found: %s", planID)
	}

	for _, asset := range plan.Assets {
		if asset.ID == assetID {
			if !asset.IsEncrypted {
				return asset.Value, nil
			}

			data, err := m.decryptData(asset.EncryptedData)
			if err != nil {
				return "", fmt.Errorf("failed to decrypt asset data: %w", err)
			}

			// 验证哈希
			if hashData(data) != asset.DataHash {
				return "", fmt.Errorf("data integrity check failed")
			}

			return data, nil
		}
	}

	return "", fmt.Errorf("asset not found: %s", assetID)
}

// GetAuditLogs 获取审计日志
func (m *Manager) GetAuditLogs(planID string, limit int) []*AuditLog {
	m.mu.RLock()
	defer m.mu.RUnlock()

	logs := make([]*AuditLog, 0)
	for _, l := range m.auditLogs {
		if l.PlanID == planID || planID == "" {
			logs = append(logs, l)
		}
	}

	if limit > 0 && limit < len(logs) {
		logs = logs[len(logs)-limit:]
	}

	return logs
}

// addAuditLog 添加审计日志
func (m *Manager) addAuditLog(log *AuditLog) {
	m.auditLogs = append(m.auditLogs, log)

	// 限制日志大小
	if len(m.auditLogs) > 10000 {
		m.auditLogs = m.auditLogs[len(m.auditLogs)-10000:]
	}
}

// GetTrustContacts 获取信任联系人列表
func (m *Manager) GetTrustContacts(ownerID string) []*TrustContact {
	m.mu.RLock()
	defer m.mu.RUnlock()

	contacts := make([]*TrustContact, 0)
	for _, c := range m.contacts {
		if c.OwnerID == ownerID {
			contacts = append(contacts, c)
		}
	}
	return contacts
}

// AddTrustContact 添加信任联系人
func (m *Manager) AddTrustContact(ownerID string, contact *TrustContact) (*TrustContact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	contact.ID = generateID()
	contact.OwnerID = ownerID
	contact.CreatedAt = time.Now()
	contact.UpdatedAt = time.Now()

	m.contacts[contact.ID] = contact

	m.logger.Info("trust contact added",
		zap.String("contact_id", contact.ID),
		zap.String("owner_id", ownerID))

	return contact, nil
}

// RemoveTrustContact 移除信任联系人
func (m *Manager) RemoveTrustContact(contactID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.contacts[contactID]; !ok {
		return fmt.Errorf("trust contact not found: %s", contactID)
	}

	delete(m.contacts, contactID)
	m.logger.Info("trust contact removed", zap.String("contact_id", contactID))
	return nil
}

// UpdateLastActive 更新最后活跃时间
func (m *Manager) UpdateLastActive(ownerID string) {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	for _, plan := range m.plans {
		if plan.OwnerID == ownerID && plan.Status == LegacyStatusActive {
			if plan.TriggerConditions == nil {
				plan.TriggerConditions = &TriggerConditions{}
			}
			plan.TriggerConditions.LastActiveAt = &now
		}
	}
}

// GetConfig 获取配置
func (m *Manager) GetConfig() *DefaultLegacyConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *DefaultLegacyConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if cfg != nil {
		m.config = cfg
	}
}
