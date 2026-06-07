package wormcompliance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ImmutabilityManager 不可变性管理器
type ImmutabilityManager struct {
	mu            sync.RWMutex
	objects       map[string]*ProtectedObject
	hashChainHead string // 哈希链头
	config        WORMConfig
}

// NewImmutabilityManager 创建不可变性管理器
func NewImmutabilityManager(config WORMConfig) *ImmutabilityManager {
	return &ImmutabilityManager{
		objects:       make(map[string]*ProtectedObject),
		hashChainHead: config.HashChainSeed,
		config:        config,
	}
}

// ProtectObject 保护对象
func (im *ImmutabilityManager) ProtectObject(path string, size int64, policyID string, createdBy string, metadata map[string]string) (*ProtectedObject, error) {
	im.mu.Lock()
	defer im.mu.Unlock()

	// 计算对象哈希
	hash := im.calculateObjectHash(path, size, createdBy)

	// 创建保护对象
	obj := &ProtectedObject{
		ID:            uuid.New().String(),
		Path:          path,
		Hash:          hash,
		HashChainPrev: im.hashChainHead,
		Size:          size,
		PolicyID:      policyID,
		Locked:        false,
		CreatedAt:     time.Now(),
		CreatedBy:     createdBy,
		Metadata:      metadata,
	}

	// 更新哈希链
	im.hashChainHead = hash

	im.objects[obj.ID] = obj
	return obj, nil
}

// LockObject 锁定对象（进入 WORM 保护）
func (im *ImmutabilityManager) LockObject(objectID string, retentionPeriod RetentionPeriod) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	obj, exists := im.objects[objectID]
	if !exists {
		return fmt.Errorf("对象 %s 不存在", objectID)
	}

	if obj.Locked {
		return fmt.Errorf("对象 %s 已被锁定", objectID)
	}

	now := time.Now()
	obj.Locked = true
	obj.LockedAt = &now

	// 计算过期时间
	if !retentionPeriod.IsForever() {
		expiresAt := now.Add(retentionPeriod.GetDuration())
		obj.ExpiresAt = &expiresAt
	}

	return nil
}

// VerifyObject 验证对象完整性
func (im *ImmutabilityManager) VerifyObject(objectID string) (bool, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	obj, exists := im.objects[objectID]
	if !exists {
		return false, fmt.Errorf("对象 %s 不存在", objectID)
	}

	// 使用对象创建时的前向哈希重新计算
	savedHashChainHead := im.hashChainHead
	im.hashChainHead = obj.HashChainPrev
	expectedHash := im.calculateObjectHash(obj.Path, obj.Size, obj.CreatedBy)
	im.hashChainHead = savedHashChainHead

	return expectedHash == obj.Hash, nil
}

// VerifyHashChain 验证哈希链完整性
func (im *ImmutabilityManager) VerifyHashChain() (bool, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	// 收集所有对象并按创建时间排序
	objects := make([]*ProtectedObject, 0, len(im.objects))
	for _, obj := range im.objects {
		objects = append(objects, obj)
	}

	if len(objects) == 0 {
		return true, nil
	}

	// 按创建时间排序
	sortObjectsByTime(objects)

	// 验证哈希链
	prevHash := im.config.HashChainSeed
	for _, obj := range objects {
		if obj.HashChainPrev != prevHash {
			return false, fmt.Errorf("哈希链断裂于对象 %s", obj.ID)
		}
		prevHash = obj.Hash
	}

	return true, nil
}

// GetObject 获取对象
func (im *ImmutabilityManager) GetObject(objectID string) (*ProtectedObject, error) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	obj, exists := im.objects[objectID]
	if !exists {
		return nil, fmt.Errorf("对象 %s 不存在", objectID)
	}
	return obj, nil
}

// ListObjects 列出所有对象
func (im *ImmutabilityManager) ListObjects() []*ProtectedObject {
	im.mu.RLock()
	defer im.mu.RUnlock()

	objects := make([]*ProtectedObject, 0, len(im.objects))
	for _, obj := range im.objects {
		objects = append(objects, obj)
	}
	return objects
}

// ListExpiredObjects 列出过期对象
func (im *ImmutabilityManager) ListExpiredObjects(now time.Time) []*ProtectedObject {
	im.mu.RLock()
	defer im.mu.RUnlock()

	var expired []*ProtectedObject
	for _, obj := range im.objects {
		if obj.ExpiresAt != nil && now.After(*obj.ExpiresAt) {
			expired = append(expired, obj)
		}
	}
	return expired
}

// CanDelete 检查对象是否可以删除
func (im *ImmutabilityManager) CanDelete(objectID string, mode ComplianceMode, now time.Time) (bool, string) {
	im.mu.RLock()
	defer im.mu.RUnlock()

	obj, exists := im.objects[objectID]
	if !exists {
		return false, "对象不存在"
	}

	// 检查是否过期
	if obj.ExpiresAt != nil && now.After(*obj.ExpiresAt) {
		return true, "保留期已过"
	}

	// 根据合规模式判断
	switch mode {
	case ModeGovernance:
		return true, "治理模式允许特权删除"
	case ModeEnterprise:
		return false, "企业模式不允许删除未过期对象"
	case ModeRegulatory:
		return false, "法规模式禁止删除任何对象"
	default:
		return false, "未知合规模式"
	}
}

// RemoveObject 移除对象（仅在允许时）
func (im *ImmutabilityManager) RemoveObject(objectID string) error {
	im.mu.Lock()
	defer im.mu.Unlock()

	if _, exists := im.objects[objectID]; !exists {
		return fmt.Errorf("对象 %s 不存在", objectID)
	}

	delete(im.objects, objectID)
	return nil
}

// calculateObjectHash 计算对象哈希
func (im *ImmutabilityManager) calculateObjectHash(path string, size int64, createdBy string) string {
	data := fmt.Sprintf("%s|%d|%s|%s", path, size, createdBy, im.hashChainHead)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}

// sortObjectsByTime 按创建时间排序对象
func sortObjectsByTime(objects []*ProtectedObject) {
	// 简单的冒泡排序
	for i := 0; i < len(objects)-1; i++ {
		for j := 0; j < len(objects)-i-1; j++ {
			if objects[j].CreatedAt.After(objects[j+1].CreatedAt) {
				objects[j], objects[j+1] = objects[j+1], objects[j]
			}
		}
	}
}
