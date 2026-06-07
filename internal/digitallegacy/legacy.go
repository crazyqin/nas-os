// Package digitallegacy 提供数字遗产核心管理逻辑
package digitallegacy

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"sync"
	"time"
)

// Manager 数字遗产管理器
type Manager struct {
	mu               sync.RWMutex
	config           *DefaultLegacyConfig
	plans            map[string]*LegacyPlan
	contacts         map[string]*TrustContact
	auditLogs        []*AuditLog
	inactivityChecks []*InactivityCheck
	accessGrants     []*AccessGrant
	encryptionKey    []byte
	stopChan         chan struct{}
	running          bool
}

// NewManager 创建数字遗产管理器
func NewManager(config *DefaultLegacyConfig, encryptionKey []byte) *Manager {
	if config == nil {
		config = GetDefaultConfig()
	}

	return &Manager{
		config:           config,
		plans:            make(map[string]*LegacyPlan),
		contacts:         make(map[string]*TrustContact),
		auditLogs:        make([]*AuditLog, 0),
		inactivityChecks: make([]*InactivityCheck, 0),
		accessGrants:     make([]*AccessGrant, 0),
		encryptionKey:    encryptionKey,
		stopChan:         make(chan struct{}),
	}
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

	if req.Name == "" {
		return nil, fmt.Errorf("plan name is required")
	}

	if !IsValidTriggerType(req.TriggerType) {
		return nil, fmt.Errorf("invalid trigger type: %s", req.TriggerType)
	}

	plan := &LegacyPlan{
		ID:                generateID(),
		Name:              req.Name,
		Description:       req.Description,
		OwnerID:           ownerID,
		Status:            LegacyStatusDraft,
		TriggerType:       req.TriggerType,
		TriggerConditions: req.TriggerConditions,
		IsEncrypted:       req.IsEncrypted,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// 设置默认触发条件
	if plan.TriggerConditions == nil {
		plan.TriggerConditions = &TriggerConditions{
			InactivityDays:    m.config.InactivityDays,
			GracePeriodDays:   m.config.GracePeriodDays,
			RequiredWitnesses: m.config.RequiredWitnesses,
		}
	}

	m.plans[plan.ID] = plan

	m.addAuditLog(&AuditLog{
		ID:        generateID(),
		PlanID:    plan.ID,
		UserID:    ownerID,
		Action:    "plan_created",
		Resource:  "legacy_plan",
		Details:   fmt.Sprintf("Plan %s created", plan.Name),
		CreatedAt: time.Now(),
	})

	log.Printf("[digitallegacy] plan created: id=%s name=%s owner=%s", plan.ID, plan.Name, ownerID)
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

	if req.Name != "" {
		plan.Name = req.Name
	}
	plan.Description = req.Description
	plan.TriggerType = req.TriggerType
	plan.TriggerConditions = req.TriggerConditions
	plan.IsEncrypted = req.IsEncrypted
	plan.UpdatedAt = time.Now()

	m.addAuditLog(&AuditLog{
		ID:        generateID(),
		PlanID:    id,
		UserID:    plan.OwnerID,
		Action:    "plan_updated",
		Resource:  "legacy_plan",
		Details:   fmt.Sprintf("Plan %s updated", id),
		CreatedAt: time.Now(),
	})

	log.Printf("[digitallegacy] plan updated: id=%s", id)
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

	m.addAuditLog(&AuditLog{
		ID:        generateID(),
		PlanID:    id,
		UserID:    plan.OwnerID,
		Action:    "plan_deleted",
		Resource:  "legacy_plan",
		Details:   fmt.Sprintf("Plan %s deleted", id),
		CreatedAt: time.Now(),
	})

	log.Printf("[digitallegacy] plan deleted: id=%s", id)
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

	m.addAuditLog(&AuditLog{
		ID:        generateID(),
		PlanID:    id,
		UserID:    plan.OwnerID,
		Action:    "plan_activated",
		Resource:  "legacy_plan",
		Details:   fmt.Sprintf("Plan %s activated", id),
		CreatedAt: time.Now(),
	})

	log.Printf("[digitallegacy] plan activated: id=%s", id)
	return nil
}

// TriggerPlan 触发遗产计划
func (m *Manager) TriggerPlan(planID string, req *TriggerRequest) error {
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
			}
		}
	}

	m.addAuditLog(&AuditLog{
		ID:        generateID(),
		PlanID:    planID,
		UserID:    "system",
		Action:    "plan_triggered",
		Resource:  "legacy_plan",
		Details:   fmt.Sprintf("Plan %s triggered via %s", planID, plan.TriggerType),
		CreatedAt: now,
	})

	log.Printf("[digitallegacy] plan triggered: id=%s type=%s", planID, plan.TriggerType)
	return nil
}

// CheckInactivity 检查不活跃状态
func (m *Manager) CheckInactivity() []*InactivityCheck {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	checks := make([]*InactivityCheck, 0)

	for _, plan := range m.plans {
		if plan.Status != LegacyStatusActive || plan.TriggerType != TriggerInactivity {
			continue
		}

		if plan.TriggerConditions == nil || plan.TriggerConditions.LastActiveAt == nil {
			continue
		}

		daysInactive := int(now.Sub(*plan.TriggerConditions.LastActiveAt).Hours() / 24)

		check := &InactivityCheck{
			ID:           generateID(),
			PlanID:       plan.ID,
			OwnerID:      plan.OwnerID,
			LastActive:   *plan.TriggerConditions.LastActiveAt,
			DaysInactive: daysInactive,
			CheckedAt:    now,
		}

		if daysInactive >= plan.TriggerConditions.InactivityDays {
			check.IsTriggered = true
			plan.Status = LegacyStatusTriggered
			plan.TriggeredAt = &now
			plan.UpdatedAt = now

			log.Printf("[digitallegacy] plan triggered by inactivity: id=%s days=%d", plan.ID, daysInactive)
		}

		checks = append(checks, check)
		m.inactivityChecks = append(m.inactivityChecks, check)
	}

	return checks
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

// GetAllAccessGrants 获取计划的所有访问授权
func (m *Manager) GetAllAccessGrants(planID string) []*AccessGrant {
	m.mu.RLock()
	defer m.mu.RUnlock()

	grants := make([]*AccessGrant, 0)
	for _, g := range m.accessGrants {
		if g.PlanID == planID && g.IsActive {
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
			log.Printf("[digitallegacy] access grant revoked: id=%s", grantID)
			return nil
		}
	}

	return fmt.Errorf("access grant not found: %s", grantID)
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
func (m *Manager) addAuditLog(entry *AuditLog) {
	m.auditLogs = append(m.auditLogs, entry)
	if len(m.auditLogs) > 10000 {
		m.auditLogs = m.auditLogs[len(m.auditLogs)-10000:]
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
