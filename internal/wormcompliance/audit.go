package wormcompliance

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sync"
	"time"

	"github.com/google/uuid"
)

// AuditManager 审计管理器
type AuditManager struct {
	mu           sync.RWMutex
	entries      []*AuditEntry
	hashChain    string // 审计哈希链头
	maxRetention int    // 最大保留天数
}

// NewAuditManager 创建审计管理器
func NewAuditManager(maxRetentionDays int) *AuditManager {
	return &AuditManager{
		entries:      make([]*AuditEntry, 0),
		hashChain:    "audit-seed",
		maxRetention: maxRetentionDays,
	}
}

// LogAction 记录操作
func (am *AuditManager) LogAction(objectID, action, actor, details, ipAddress string, success bool, reason string) *AuditEntry {
	am.mu.Lock()
	defer am.mu.Unlock()

	// 计算哈希
	hash := am.calculateHash(objectID, action, actor, details)

	entry := &AuditEntry{
		ID:        uuid.New().String(),
		Timestamp: time.Now(),
		ObjectID:  objectID,
		Action:    action,
		Actor:     actor,
		Details:   details,
		PrevHash:  am.hashChain,
		Hash:      hash,
		IPAddress: ipAddress,
		Success:   success,
		Reason:    reason,
	}

	// 更新哈希链
	am.hashChain = hash

	am.entries = append(am.entries, entry)
	return entry
}

// GetEntries 获取审计记录
func (am *AuditManager) GetEntries(limit int) []*AuditEntry {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if limit <= 0 || limit > len(am.entries) {
		limit = len(am.entries)
	}

	// 返回最新的记录
	start := len(am.entries) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*AuditEntry, limit)
	copy(result, am.entries[start:])
	return result
}

// GetEntriesForObject 获取指定对象的审计记录
func (am *AuditManager) GetEntriesForObject(objectID string) []*AuditEntry {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var result []*AuditEntry
	for _, entry := range am.entries {
		if entry.ObjectID == objectID {
			result = append(result, entry)
		}
	}
	return result
}

// GetEntriesByActor 获取指定操作者的审计记录
func (am *AuditManager) GetEntriesByActor(actor string) []*AuditEntry {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var result []*AuditEntry
	for _, entry := range am.entries {
		if entry.Actor == actor {
			result = append(result, entry)
		}
	}
	return result
}

// GetEntriesByTimeRange 获取时间范围内的审计记录
func (am *AuditManager) GetEntriesByTimeRange(start, end time.Time) []*AuditEntry {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var result []*AuditEntry
	for _, entry := range am.entries {
		if entry.Timestamp.After(start) && entry.Timestamp.Before(end) {
			result = append(result, entry)
		}
	}
	return result
}

// GetFailedAttempts 获取失败的操作尝试
func (am *AuditManager) GetFailedAttempts() []*AuditEntry {
	am.mu.RLock()
	defer am.mu.RUnlock()

	var result []*AuditEntry
	for _, entry := range am.entries {
		if !entry.Success {
			result = append(result, entry)
		}
	}
	return result
}

// VerifyAuditChain 验证审计链完整性
func (am *AuditManager) VerifyAuditChain() (bool, error) {
	am.mu.RLock()
	defer am.mu.RUnlock()

	if len(am.entries) == 0 {
		return true, nil
	}

	prevHash := "audit-seed"
	for _, entry := range am.entries {
		// 验证前向哈希
		if entry.PrevHash != prevHash {
			return false, fmt.Errorf("审计链断裂于记录 %s", entry.ID)
		}

		// 使用记录的 PrevHash 重新计算哈希
		savedHashChain := am.hashChain
		am.hashChain = entry.PrevHash
		expectedHash := am.calculateHash(entry.ObjectID, entry.Action, entry.Actor, entry.Details)
		am.hashChain = savedHashChain
		if entry.Hash != expectedHash {
			return false, fmt.Errorf("审计记录 %s 哈希不匹配", entry.ID)
		}

		prevHash = entry.Hash
	}

	return true, nil
}

// PurgeOldEntries 清理过期的审计记录
func (am *AuditManager) PurgeOldEntries() int {
	am.mu.Lock()
	defer am.mu.Unlock()

	if am.maxRetention <= 0 {
		return 0
	}

	cutoff := time.Now().AddDate(0, 0, -am.maxRetention)
	purged := 0

	// 找到需要保留的记录
	newEntries := make([]*AuditEntry, 0, len(am.entries))
	for _, entry := range am.entries {
		if entry.Timestamp.After(cutoff) {
			newEntries = append(newEntries, entry)
		} else {
			purged++
		}
	}

	am.entries = newEntries
	return purged
}

// GetStats 获取审计统计
func (am *AuditManager) GetStats() map[string]interface{} {
	am.mu.RLock()
	defer am.mu.RUnlock()

	total := len(am.entries)
	successful := 0
	failed := 0
	actors := make(map[string]int)
	actions := make(map[string]int)

	for _, entry := range am.entries {
		if entry.Success {
			successful++
		} else {
			failed++
		}
		actors[entry.Actor]++
		actions[entry.Action]++
	}

	return map[string]interface{}{
		"total_records":    total,
		"successful":       successful,
		"failed":           failed,
		"unique_actors":    len(actors),
		"unique_actions":   len(actions),
		"actor_breakdown":  actors,
		"action_breakdown": actions,
	}
}

// GetEntryCount 获取记录总数
func (am *AuditManager) GetEntryCount() int {
	am.mu.RLock()
	defer am.mu.RUnlock()
	return len(am.entries)
}

// calculateHash 计算审计记录哈希
func (am *AuditManager) calculateHash(objectID, action, actor, details string) string {
	data := fmt.Sprintf("%s|%s|%s|%s|%s", objectID, action, actor, details, am.hashChain)
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])
}
