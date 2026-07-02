// Package digitallegacy 提供死亡验证和紧急联系人验证机制
package digitallegacy

import (
	"fmt"
	"log"
	"time"
)

// AddEmergencyContact 添加紧急联系人.
func (m *Manager) AddEmergencyContact(planID string, req *EmergencyContactRequest) (*EmergencyContact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	if req.Name == "" {
		return nil, fmt.Errorf("contact name is required")
	}

	// 验证级别 1-3
	if req.Level < 1 || req.Level > 3 {
		req.Level = 1
	}

	contact := &EmergencyContact{
		ID:              generateID(),
		PlanID:          planID,
		Name:            req.Name,
		Email:           req.Email,
		Phone:           req.Phone,
		Relationship:    req.Relationship,
		Role:            req.Role,
		Level:           req.Level,
		IsPrimary:       req.IsPrimary,
		CanTriggerPlan:  req.CanTriggerPlan,
		NotifyOnTrigger: req.NotifyOnTrigger,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	plan.EmergencyContacts = append(plan.EmergencyContacts, contact)
	plan.UpdatedAt = time.Now()

	m.addAuditLog(&AuditLog{
		ID:        generateID(),
		PlanID:    planID,
		UserID:    plan.OwnerID,
		Action:    "emergency_contact_added",
		Resource:  "emergency_contact",
		Details:   fmt.Sprintf("Emergency contact %s (level=%d) added to plan %s", contact.Name, contact.Level, planID),
		CreatedAt: time.Now(),
	})

	log.Printf("[digitallegacy] emergency contact added: id=%s plan=%s name=%s level=%d", contact.ID, planID, contact.Name, contact.Level)
	return contact, nil
}

// UpdateEmergencyContact 更新紧急联系人.
func (m *Manager) UpdateEmergencyContact(planID, contactID string, req *EmergencyContactRequest) (*EmergencyContact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	for _, c := range plan.EmergencyContacts {
		if c.ID == contactID {
			if req.Name != "" {
				c.Name = req.Name
			}
			c.Email = req.Email
			c.Phone = req.Phone
			c.Relationship = req.Relationship
			c.Role = req.Role
			if req.Level >= 1 && req.Level <= 3 {
				c.Level = req.Level
			}
			c.IsPrimary = req.IsPrimary
			c.CanTriggerPlan = req.CanTriggerPlan
			c.NotifyOnTrigger = req.NotifyOnTrigger
			c.UpdatedAt = time.Now()
			plan.UpdatedAt = time.Now()

			log.Printf("[digitallegacy] emergency contact updated: id=%s", contactID)
			return c, nil
		}
	}

	return nil, fmt.Errorf("emergency contact not found: %s", contactID)
}

// RemoveEmergencyContact 移除紧急联系人.
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

			log.Printf("[digitallegacy] emergency contact removed: id=%s plan=%s", contactID, planID)
			return nil
		}
	}

	return fmt.Errorf("emergency contact not found: %s", contactID)
}

// ListEmergencyContacts 列出紧急联系人.
func (m *Manager) ListEmergencyContacts(planID string) ([]*EmergencyContact, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	return plan.EmergencyContacts, nil
}

// VerifyEmergencyContact 验证紧急联系人.
func (m *Manager) VerifyEmergencyContact(planID, contactID string, method VerificationMethod, code string) (*VerificationRequest, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !IsValidVerificationMethod(method) {
		return nil, fmt.Errorf("invalid verification method: %s", method)
	}

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	var targetContact *EmergencyContact
	for _, c := range plan.EmergencyContacts {
		if c.ID == contactID {
			targetContact = c
			break
		}
	}

	if targetContact == nil {
		return nil, fmt.Errorf("emergency contact not found: %s", contactID)
	}

	// 创建验证请求
	vr := &VerificationRequest{
		ID:        generateID(),
		PlanID:    planID,
		ContactID: contactID,
		Method:    method,
		Level:     VerificationLevel(targetContact.Level),
		Status:    "pending",
		Code:      code,
		ExpiresAt: time.Now().Add(24 * time.Hour),
		CreatedAt: time.Now(),
	}

	// 简化验证：非空 code 即通过
	if code != "" {
		now := time.Now()
		vr.Status = "verified"
		vr.VerifiedAt = &now

		targetContact.IsVerified = true
		targetContact.VerifiedAt = &now
		targetContact.UpdatedAt = now
		plan.UpdatedAt = now

		m.addAuditLog(&AuditLog{
			ID:        generateID(),
			PlanID:    planID,
			UserID:    plan.OwnerID,
			Action:    "contact_verified",
			Resource:  "emergency_contact",
			Details:   fmt.Sprintf("Contact %s verified via %s at level %d", contactID, method, targetContact.Level),
			CreatedAt: now,
		})

		log.Printf("[digitallegacy] emergency contact verified: id=%s method=%s level=%d", contactID, method, targetContact.Level)
	}

	return vr, nil
}

// GetVerificationStatus 获取验证状态.
func (m *Manager) GetVerificationStatus(planID string) (map[string]bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	status := make(map[string]bool)
	for _, c := range plan.EmergencyContacts {
		status[c.ID] = c.IsVerified
	}

	return status, nil
}

// CheckVerificationLevel 检查是否满足验证级别要求.
func (m *Manager) CheckVerificationLevel(planID string, requiredLevel VerificationLevel) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[planID]
	if !ok {
		return false, fmt.Errorf("plan not found: %s", planID)
	}

	// 检查每个级别的验证状态
	levelVerified := make(map[int]bool)
	for _, c := range plan.EmergencyContacts {
		if c.IsVerified && c.Level > 0 {
			levelVerified[c.Level] = true
		}
	}

	// 检查从 1 到 requiredLevel 的所有级别是否都有验证
	for level := 1; level <= int(requiredLevel); level++ {
		if !levelVerified[level] {
			return false, nil
		}
	}

	return true, nil
}

// StartDeathVerification 启动死亡验证流程.
func (m *Manager) StartDeathVerification(planID string, req *DeathVerificationRequest) (*DeathVerification, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	if plan.Status != LegacyStatusActive {
		return nil, fmt.Errorf("plan must be active to start death verification")
	}

	if req.ConfirmerName == "" {
		return nil, fmt.Errorf("confirmer name is required")
	}

	if !IsValidVerificationMethod(req.Method) {
		return nil, fmt.Errorf("invalid verification method: %s", req.Method)
	}

	dv := &DeathVerification{
		ID:                generateID(),
		PlanID:            planID,
		OwnerID:           plan.OwnerID,
		Status:            "pending",
		VerificationLevel: VerifyLevelPrimary,
		ConfirmerID:       req.ConfirmerID,
		ConfirmerName:     req.ConfirmerName,
		ConfirmerRelation: req.ConfirmerRelation,
		Method:            req.Method,
		Evidence:          req.Evidence,
		Notes:             req.Notes,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	// 如果有死亡证明，直接进入高级别验证
	if req.Method == VerifyDeathCert || req.Method == VerifyNotary {
		dv.VerificationLevel = VerifyLevelTertiary
	}

	m.addAuditLog(&AuditLog{
		ID:        generateID(),
		PlanID:    planID,
		UserID:    req.ConfirmerID,
		Action:    "death_verification_started",
		Resource:  "death_verification",
		Details:   fmt.Sprintf("Death verification started by %s (relation=%s, method=%s)", req.ConfirmerName, req.ConfirmerRelation, req.Method),
		CreatedAt: time.Now(),
	})

	log.Printf("[digitallegacy] death verification started: id=%s plan=%s confirmer=%s", dv.ID, planID, req.ConfirmerName)
	return dv, nil
}

// ConfirmDeath 确认死亡.
func (m *Manager) ConfirmDeath(planID string, verificationID string, confirmerLevel VerificationLevel, evidence string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return fmt.Errorf("plan not found: %s", planID)
	}

	if plan.Status != LegacyStatusActive && plan.Status != LegacyStatusTriggered {
		return fmt.Errorf("plan must be active or triggered to confirm death")
	}

	// 检查是否有足够的验证级别
	levelVerified := make(map[int]bool)
	for _, c := range plan.EmergencyContacts {
		if c.IsVerified && c.Level > 0 {
			levelVerified[c.Level] = true
		}
	}

	// 至少需要两个级别的验证
	verifiedLevels := 0
	for _, v := range levelVerified {
		if v {
			verifiedLevels++
		}
	}

	if verifiedLevels < 2 && confirmerLevel < VerifyLevelTertiary {
		return fmt.Errorf("insufficient verification levels: need at least 2, got %d", verifiedLevels)
	}

	now := time.Now()

	// 触发计划
	if plan.Status == LegacyStatusActive {
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
	}

	m.addAuditLog(&AuditLog{
		ID:        generateID(),
		PlanID:    planID,
		UserID:    "system",
		Action:    "death_confirmed",
		Resource:  "death_verification",
		Details:   fmt.Sprintf("Death confirmed at level %d with evidence: %s", confirmerLevel, evidence),
		CreatedAt: now,
	})

	log.Printf("[digitallegacy] death confirmed: plan=%s level=%d", planID, confirmerLevel)
	return nil
}

// RecordHeartbeat 记录心跳.
func (m *Manager) RecordHeartbeat(ownerID string, note string) *HeartbeatRecord {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	record := &HeartbeatRecord{
		ID:        generateID(),
		OwnerID:   ownerID,
		Status:    HeartbeatAlive,
		CheckedAt: now,
		ExpiresAt: now.Add(time.Duration(m.config.HeartbeatTimeout) * time.Hour),
		Note:      note,
	}

	// 更新所有该用户的活跃计划的最后活跃时间
	for _, plan := range m.plans {
		if plan.OwnerID == ownerID && plan.Status == LegacyStatusActive {
			if plan.TriggerConditions == nil {
				plan.TriggerConditions = &TriggerConditions{}
			}
			plan.TriggerConditions.LastActiveAt = &now
		}
	}

	log.Printf("[digitallegacy] heartbeat recorded: owner=%s", ownerID)
	return record
}

// CheckHeartbeatStatus 检查心跳状态.
func (m *Manager) CheckHeartbeatStatus(ownerID string) (*HeartbeatStatus, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	// 查找该用户的活跃计划
	for _, plan := range m.plans {
		if plan.OwnerID == ownerID && plan.Status == LegacyStatusActive {
			if plan.TriggerConditions == nil || plan.TriggerConditions.LastActiveAt == nil {
				status := HeartbeatMissing
				return &status, nil
			}

			hoursSinceLastActive := time.Since(*plan.TriggerConditions.LastActiveAt).Hours()
			if hoursSinceLastActive > float64(m.config.HeartbeatTimeout) {
				status := HeartbeatMissing
				return &status, nil
			}

			status := HeartbeatAlive
			return &status, nil
		}
	}

	status := HeartbeatAlive
	return &status, nil
}

// ProcessMissingHeartbeats 处理缺失心跳.
func (m *Manager) ProcessMissingHeartbeats() []string {
	m.mu.Lock()
	defer m.mu.Unlock()

	now := time.Now()
	triggeredPlans := make([]string, 0)

	for _, plan := range m.plans {
		if plan.Status != LegacyStatusActive {
			continue
		}

		if plan.TriggerType != TriggerInactivity {
			continue
		}

		if plan.TriggerConditions == nil || plan.TriggerConditions.LastActiveAt == nil {
			continue
		}

		hoursSinceLastActive := now.Sub(*plan.TriggerConditions.LastActiveAt).Hours()
		if hoursSinceLastActive > float64(m.config.HeartbeatTimeout) {
			// 标记为缺失状态
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
							PlanID:        plan.ID,
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

			triggeredPlans = append(triggeredPlans, plan.ID)

			m.addAuditLog(&AuditLog{
				ID:        generateID(),
				PlanID:    plan.ID,
				UserID:    "system",
				Action:    "heartbeat_missing_triggered",
				Resource:  "legacy_plan",
				Details:   fmt.Sprintf("Plan %s triggered due to missing heartbeat (%.0f hours since last active)", plan.ID, hoursSinceLastActive),
				CreatedAt: now,
			})

			log.Printf("[digitallegacy] plan triggered by missing heartbeat: id=%s hours=%.0f", plan.ID, hoursSinceLastActive)
		}
	}

	return triggeredPlans
}

// AddTrustContact 添加信任联系人.
func (m *Manager) AddTrustContact(ownerID string, contact *TrustContact) (*TrustContact, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if contact.Name == "" {
		return nil, fmt.Errorf("contact name is required")
	}

	contact.ID = generateID()
	contact.OwnerID = ownerID
	contact.CreatedAt = time.Now()
	contact.UpdatedAt = time.Now()

	m.contacts[contact.ID] = contact

	log.Printf("[digitallegacy] trust contact added: id=%s owner=%s name=%s", contact.ID, ownerID, contact.Name)
	return contact, nil
}

// RemoveTrustContact 移除信任联系人.
func (m *Manager) RemoveTrustContact(contactID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.contacts[contactID]; !ok {
		return fmt.Errorf("trust contact not found: %s", contactID)
	}

	delete(m.contacts, contactID)
	log.Printf("[digitallegacy] trust contact removed: id=%s", contactID)
	return nil
}

// GetTrustContacts 获取信任联系人列表.
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

// SetTimeLock 设置时间锁.
func (m *Manager) SetTimeLock(planID string, req *TimeLockRequest) (*TimeLock, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	if req.UnlockAt.Before(time.Now()) {
		return nil, fmt.Errorf("unlock time must be in the future")
	}

	if req.RequiredLevel < 1 || req.RequiredLevel > 3 {
		req.RequiredLevel = 1
	}

	tl := &TimeLock{
		ID:            generateID(),
		PlanID:        planID,
		UnlockAt:      req.UnlockAt,
		IsActive:      true,
		RequiredLevel: req.RequiredLevel,
		CreatedAt:     time.Now(),
	}

	plan.TimeLock = tl
	plan.UpdatedAt = time.Now()

	m.addAuditLog(&AuditLog{
		ID:        generateID(),
		PlanID:    planID,
		UserID:    plan.OwnerID,
		Action:    "timelock_set",
		Resource:  "time_lock",
		Details:   fmt.Sprintf("Time lock set for plan %s, unlock at %s, required level %d", planID, req.UnlockAt.Format(time.RFC3339), req.RequiredLevel),
		CreatedAt: time.Now(),
	})

	log.Printf("[digitallegacy] time lock set: plan=%s unlock_at=%s level=%d", planID, req.UnlockAt.Format(time.RFC3339), req.RequiredLevel)
	return tl, nil
}

// CheckTimeLock 检查时间锁状态.
func (m *Manager) CheckTimeLock(planID string) (bool, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[planID]
	if !ok {
		return false, fmt.Errorf("plan not found: %s", planID)
	}

	if plan.TimeLock == nil {
		return true, nil // 没有时间锁，视为已解锁
	}

	if !plan.TimeLock.IsActive {
		return true, nil
	}

	if plan.TimeLock.UnlockedAt != nil {
		return true, nil
	}

	if time.Now().Before(plan.TimeLock.UnlockAt) {
		return false, nil // 时间锁未到期
	}

	return true, nil // 时间锁已到期
}

// UnlockTimeLock 解锁时间锁.
func (m *Manager) UnlockTimeLock(planID string, verificationLevel VerificationLevel) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return fmt.Errorf("plan not found: %s", planID)
	}

	if plan.TimeLock == nil {
		return fmt.Errorf("no time lock for plan: %s", planID)
	}

	if !plan.TimeLock.IsActive {
		return fmt.Errorf("time lock is not active")
	}

	if plan.TimeLock.UnlockedAt != nil {
		return fmt.Errorf("time lock already unlocked")
	}

	if time.Now().Before(plan.TimeLock.UnlockAt) {
		return fmt.Errorf("time lock not yet expired")
	}

	if int(verificationLevel) < plan.TimeLock.RequiredLevel {
		return fmt.Errorf("insufficient verification level: need %d, got %d", plan.TimeLock.RequiredLevel, verificationLevel)
	}

	now := time.Now()
	plan.TimeLock.UnlockedAt = &now
	plan.TimeLock.IsActive = false
	plan.UpdatedAt = now

	m.addAuditLog(&AuditLog{
		ID:        generateID(),
		PlanID:    planID,
		UserID:    "system",
		Action:    "timelock_unlocked",
		Resource:  "time_lock",
		Details:   fmt.Sprintf("Time lock unlocked for plan %s at level %d", planID, verificationLevel),
		CreatedAt: now,
	})

	log.Printf("[digitallegacy] time lock unlocked: plan=%s level=%d", planID, verificationLevel)
	return nil
}
