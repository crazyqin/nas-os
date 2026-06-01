// Package digitallegacy 提供数字资产分类管理功能
package digitallegacy

import (
	"fmt"
	"log"
	"time"
)

// AssetCategory 资产分类
type AssetCategory struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	Type        AssetType `json:"type"`
	Description string    `json:"description,omitempty"`
	Icon        string    `json:"icon,omitempty"`
	Count       int       `json:"count"`
	CreatedAt   time.Time `json:"created_at"`
}

// AddBeneficiary 添加受益人
func (m *Manager) AddBeneficiary(planID string, req *BeneficiaryRequest) (*Beneficiary, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	if req.Name == "" {
		return nil, fmt.Errorf("beneficiary name is required")
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
		ID:                generateID(),
		PlanID:            planID,
		Name:              req.Name,
		Email:             req.Email,
		Phone:             req.Phone,
		Relationship:      req.Relationship,
		Role:              req.Role,
		AllocationPercent: req.AllocationPercent,
		AccessLevel:       req.AccessLevel,
		CreatedAt:         time.Now(),
		UpdatedAt:         time.Now(),
	}

	plan.Beneficiaries = append(plan.Beneficiaries, beneficiary)
	plan.UpdatedAt = time.Now()

	m.addAuditLog(&AuditLog{
		ID:        generateID(),
		PlanID:    planID,
		UserID:    plan.OwnerID,
		Action:    "beneficiary_added",
		Resource:  "beneficiary",
		Details:   fmt.Sprintf("Beneficiary %s added to plan %s", beneficiary.Name, planID),
		CreatedAt: time.Now(),
	})

	log.Printf("[digitallegacy] beneficiary added: id=%s plan=%s name=%s", beneficiary.ID, planID, beneficiary.Name)
	return beneficiary, nil
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
			if req.Name != "" {
				b.Name = req.Name
			}
			b.Email = req.Email
			b.Phone = req.Phone
			b.Relationship = req.Relationship
			b.Role = req.Role
			b.AllocationPercent = req.AllocationPercent
			b.AccessLevel = req.AccessLevel
			b.UpdatedAt = time.Now()
			plan.UpdatedAt = time.Now()

			m.addAuditLog(&AuditLog{
				ID:        generateID(),
				PlanID:    planID,
				UserID:    plan.OwnerID,
				Action:    "beneficiary_updated",
				Resource:  "beneficiary",
				Details:   fmt.Sprintf("Beneficiary %s updated", beneficiaryID),
				CreatedAt: time.Now(),
			})

			log.Printf("[digitallegacy] beneficiary updated: id=%s", beneficiaryID)
			return b, nil
		}
	}

	return nil, fmt.Errorf("beneficiary not found: %s", beneficiaryID)
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

			m.addAuditLog(&AuditLog{
				ID:        generateID(),
				PlanID:    planID,
				UserID:    plan.OwnerID,
				Action:    "beneficiary_removed",
				Resource:  "beneficiary",
				Details:   fmt.Sprintf("Beneficiary %s removed from plan %s", beneficiaryID, planID),
				CreatedAt: time.Now(),
			})

			log.Printf("[digitallegacy] beneficiary removed: id=%s plan=%s", beneficiaryID, planID)
			return nil
		}
	}

	return fmt.Errorf("beneficiary not found: %s", beneficiaryID)
}

// ListBeneficiaries 列出受益人
func (m *Manager) ListBeneficiaries(planID string) ([]*Beneficiary, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	return plan.Beneficiaries, nil
}

// AddAsset 添加数字资产
func (m *Manager) AddAsset(planID string, req *AssetRequest) (*DigitalAsset, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	if req.Name == "" {
		return nil, fmt.Errorf("asset name is required")
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
	} else if req.Data != "" {
		asset.Value = req.Data
	}

	plan.Assets = append(plan.Assets, asset)
	plan.UpdatedAt = time.Now()

	m.addAuditLog(&AuditLog{
		ID:        generateID(),
		PlanID:    planID,
		UserID:    plan.OwnerID,
		Action:    "asset_added",
		Resource:  "digital_asset",
		Details:   fmt.Sprintf("Asset %s (type=%s) added to plan %s", asset.Name, asset.Type, planID),
		CreatedAt: time.Now(),
	})

	log.Printf("[digitallegacy] asset added: id=%s plan=%s type=%s name=%s", asset.ID, planID, asset.Type, asset.Name)
	return asset, nil
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
			if req.Name != "" {
				a.Name = req.Name
			}
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

			m.addAuditLog(&AuditLog{
				ID:        generateID(),
				PlanID:    planID,
				UserID:    plan.OwnerID,
				Action:    "asset_updated",
				Resource:  "digital_asset",
				Details:   fmt.Sprintf("Asset %s updated", assetID),
				CreatedAt: time.Now(),
			})

			log.Printf("[digitallegacy] asset updated: id=%s", assetID)
			return a, nil
		}
	}

	return nil, fmt.Errorf("asset not found: %s", assetID)
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

			m.addAuditLog(&AuditLog{
				ID:        generateID(),
				PlanID:    planID,
				UserID:    plan.OwnerID,
				Action:    "asset_removed",
				Resource:  "digital_asset",
				Details:   fmt.Sprintf("Asset %s removed from plan %s", assetID, planID),
				CreatedAt: time.Now(),
			})

			log.Printf("[digitallegacy] asset removed: id=%s plan=%s", assetID, planID)
			return nil
		}
	}

	return fmt.Errorf("asset not found: %s", assetID)
}

// ListAssets 列出资产
func (m *Manager) ListAssets(planID string) ([]*DigitalAsset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	return plan.Assets, nil
}

// ListAssetsByType 按类型列出资产
func (m *Manager) ListAssetsByType(planID string, assetType AssetType) ([]*DigitalAsset, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	result := make([]*DigitalAsset, 0)
	for _, a := range plan.Assets {
		if a.Type == assetType {
			result = append(result, a)
		}
	}

	return result, nil
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

// GetAssetSummary 获取资产摘要
func (m *Manager) GetAssetSummary(planID string) (map[AssetType]int, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	plan, ok := m.plans[planID]
	if !ok {
		return nil, fmt.Errorf("plan not found: %s", planID)
	}

	summary := make(map[AssetType]int)
	for _, a := range plan.Assets {
		summary[a.Type]++
	}

	return summary, nil
}
