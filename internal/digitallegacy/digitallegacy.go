// Package digitallegacy 提供数字遗产管理模块入口
//
// 本模块实现了完整的数字遗产管理功能，包括：
// - 遗产计划的创建、激活、触发和管理
// - 数字资产分类管理（文件、密码、账户、加密货币钱包等）
// - 多级紧急联系人验证机制
// - 死亡验证机制（心跳检测 + 人工确认）
// - 时间锁和安全解锁流程
// - 访问授权和审计日志
//
// 使用标准库实现，无外部依赖。
package digitallegacy

import (
	"fmt"
	"time"
)

// ModuleVersion 模块版本.
const ModuleVersion = "1.0.0"

// ModuleName 模块名称.
const ModuleName = "digitallegacy"

// NewLegacyService 创建数字遗产服务（便捷入口）.
func NewLegacyService(encryptionKey []byte) *Manager {
	config := GetDefaultConfig()
	return NewManager(config, encryptionKey)
}

// NewLegacyServiceWithConfig 使用自定义配置创建数字遗产服务.
func NewLegacyServiceWithConfig(config *DefaultLegacyConfig, encryptionKey []byte) *Manager {
	return NewManager(config, encryptionKey)
}

// SetWillDocument 设置遗嘱文档.
func (m *Manager) SetWillDocument(planID string, req *WillDocumentRequest) (*WillDocument, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Title == "" {
		return nil, fmt.Errorf("will document title is required")
	}

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
	if plan.IsEncrypted && req.Content != "" {
		encryptedContent, err := m.encryptData(req.Content)
		if err != nil {
			return nil, fmt.Errorf("failed to encrypt will document: %w", err)
		}
		doc.EncryptedContent = encryptedContent
		doc.Content = ""
		doc.IsEncrypted = true
	}

	if req.Content != "" {
		doc.FileHash = hashData(req.Content)
	}

	plan.WillDocument = doc
	plan.UpdatedAt = time.Now()

	m.addAuditLog(&AuditLog{
		ID:        generateID(),
		PlanID:    planID,
		UserID:    plan.OwnerID,
		Action:    "will_document_set",
		Resource:  "will_document",
		Details:   fmt.Sprintf("Will document '%s' set for plan %s", req.Title, planID),
		CreatedAt: time.Now(),
	})

	return doc, nil
}

// GetWillDocument 获取遗嘱文档.
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

	if decrypt && doc.IsEncrypted && doc.EncryptedContent != "" {
		content, err := m.decryptData(doc.EncryptedContent)
		if err != nil {
			return nil, fmt.Errorf("failed to decrypt will document: %w", err)
		}
		doc.Content = content
	}

	return &doc, nil
}

// GetModuleInfo 获取模块信息.
func GetModuleInfo() map[string]interface{} {
	return map[string]interface{}{
		"name":        ModuleName,
		"version":     ModuleVersion,
		"description": "数字遗产管理模块",
		"features": []string{
			"遗产计划管理",
			"数字资产分类（文件、密码、账户、加密货币钱包）",
			"多级紧急联系人验证",
			"死亡验证（心跳检测 + 人工确认）",
			"时间锁安全解锁",
			"访问授权管理",
			"审计日志",
		},
	}
}
