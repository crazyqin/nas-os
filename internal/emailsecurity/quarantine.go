// Package emailsecurity 提供隔离邮件管理功能
package emailsecurity

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// QuarantineManager 隔离管理器
type QuarantineManager struct {
	mu           sync.RWMutex
	items        map[string]*QuarantineItem
	retentionDays int           // 保留天数
	maxItems     int           // 最大隔离项数
	autoRelease  bool          // 是否自动释放低威胁项
}

// NewQuarantineManager 创建新的隔离管理器
func NewQuarantineManager(retentionDays, maxItems int, autoRelease bool) *QuarantineManager {
	return &QuarantineManager{
		items:         make(map[string]*QuarantineItem),
		retentionDays: retentionDays,
		maxItems:      maxItems,
		autoRelease:   autoRelease,
	}
}

// QuarantineEmail 隔离邮件
func (qm *QuarantineManager) QuarantineEmail(req ScanEmailRequest, result *ScanResult) (*QuarantineItem, error) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	// 检查隔离队列是否已满
	if qm.maxItems > 0 && len(qm.items) >= qm.maxItems {
		return nil, fmt.Errorf("隔离队列已满 (%d/%d)", len(qm.items), qm.maxItems)
	}

	// 根据扫描结果决定是否隔离
	threatLevel := qm.determineThreatLevel(result.Score)
	reason := qm.generateQuarantineReason(result.Threats)

	item := &QuarantineItem{
		ID:            uuid.New().String(),
		MessageID:     req.MessageID,
		From:          req.From,
		To:            req.To,
		Subject:       req.Subject,
		Reason:        reason,
		ThreatLevel:   threatLevel,
		Status:        QuarantineStatusPending,
		ScanResult:    *result,
		QuarantinedBy: "system",
		CreatedAt:     time.Now(),
		UpdatedAt:     time.Now(),
		ExpiresAt:     time.Now().AddDate(0, 0, qm.retentionDays),
	}

	qm.items[item.ID] = item

	log.Printf("[隔离管理] 邮件已隔离: id=%s, messageID=%s, 威胁等级=%s, 原因=%s",
		item.ID, item.MessageID, item.ThreatLevel, item.Reason)

	// 自动释放低威胁项
	if qm.autoRelease && threatLevel == ThreatLevelLow {
		go qm.autoReleaseItem(item.ID)
	}

	return item, nil
}

// ReviewItem 审批隔离项
func (qm *QuarantineManager) ReviewItem(itemID string, req ReviewQuarantineRequest) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	item, ok := qm.items[itemID]
	if !ok {
		return fmt.Errorf("隔离项不存在: %s", itemID)
	}

	if item.Status != QuarantineStatusPending {
		return fmt.Errorf("隔离项状态不允许审批: %s", item.Status)
	}

	switch req.Action {
	case "approve":
		item.Status = QuarantineStatusApproved
		log.Printf("[隔离管理] 邮件已批准: id=%s, 审批人=%s", itemID, req.ReviewBy)
	case "reject":
		item.Status = QuarantineStatusRejected
		log.Printf("[隔离管理] 邮件已拒绝: id=%s, 审批人=%s", itemID, req.ReviewBy)
	case "release":
		item.Status = QuarantineStatusReleased
		log.Printf("[隔离管理] 邮件已释放: id=%s, 审批人=%s", itemID, req.ReviewBy)
	default:
		return fmt.Errorf("无效的审批操作: %s", req.Action)
	}

	item.ReviewedBy = req.ReviewBy
	item.ReviewNote = req.Note
	item.UpdatedAt = time.Now()

	return nil
}

// GetItem 获取隔离项
func (qm *QuarantineManager) GetItem(itemID string) (*QuarantineItem, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	item, ok := qm.items[itemID]
	if !ok {
		return nil, fmt.Errorf("隔离项不存在: %s", itemID)
	}

	return item, nil
}

// ListItems 列出隔离项
func (qm *QuarantineManager) ListItems(status string, threatLevel string, page, pageSize int) ([]*QuarantineItem, int, error) {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	// 过滤
	filtered := make([]*QuarantineItem, 0)
	for _, item := range qm.items {
		if status != "" && item.Status != status {
			continue
		}
		if threatLevel != "" && item.ThreatLevel != threatLevel {
			continue
		}
		filtered = append(filtered, item)
	}

	// 排序（按创建时间倒序）
	for i := 0; i < len(filtered)-1; i++ {
		for j := i + 1; j < len(filtered); j++ {
			if filtered[i].CreatedAt.Before(filtered[j].CreatedAt) {
				filtered[i], filtered[j] = filtered[j], filtered[i]
			}
		}
	}

	total := len(filtered)

	// 分页
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	if start >= total {
		return []*QuarantineItem{}, total, nil
	}

	end := start + pageSize
	if end > total {
		end = total
	}

	return filtered[start:end], total, nil
}

// DeleteItem 删除隔离项
func (qm *QuarantineManager) DeleteItem(itemID string) error {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	if _, ok := qm.items[itemID]; !ok {
		return fmt.Errorf("隔离项不存在: %s", itemID)
	}

	delete(qm.items, itemID)
	log.Printf("[隔离管理] 隔离项已删除: id=%s", itemID)
	return nil
}

// CleanupExpired 清理过期的隔离项
func (qm *QuarantineManager) CleanupExpired() int {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	now := time.Now()
	expiredCount := 0

	for id, item := range qm.items {
		if now.After(item.ExpiresAt) {
			delete(qm.items, id)
			expiredCount++
		}
	}

	if expiredCount > 0 {
		log.Printf("[隔离管理] 已清理过期隔离项: %d 个", expiredCount)
	}

	return expiredCount
}

// GetStats 获取隔离统计信息
func (qm *QuarantineManager) GetStats() map[string]int {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	stats := map[string]int{
		"total":     len(qm.items),
		"pending":   0,
		"approved":  0,
		"rejected":  0,
		"released":  0,
		"low":       0,
		"medium":    0,
		"high":      0,
		"critical":  0,
	}

	for _, item := range qm.items {
		stats[item.Status]++
		stats[item.ThreatLevel]++
	}

	return stats
}

// GetPendingCount 获取待审批数量
func (qm *QuarantineManager) GetPendingCount() int {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	count := 0
	for _, item := range qm.items {
		if item.Status == QuarantineStatusPending {
			count++
		}
	}
	return count
}

// BatchReview 批量审批
func (qm *QuarantineManager) BatchReview(itemIDs []string, req ReviewQuarantineRequest) (int, error) {
	qm.mu.Lock()
	defer qm.mu.Unlock()

	successCount := 0
	for _, itemID := range itemIDs {
		item, ok := qm.items[itemID]
		if !ok {
			continue
		}

		if item.Status != QuarantineStatusPending {
			continue
		}

		switch req.Action {
		case "approve":
			item.Status = QuarantineStatusApproved
		case "reject":
			item.Status = QuarantineStatusRejected
		case "release":
			item.Status = QuarantineStatusReleased
		}

		item.ReviewedBy = req.ReviewBy
		item.ReviewNote = req.Note
		item.UpdatedAt = time.Now()
		successCount++
	}

	log.Printf("[隔离管理] 批量审批完成: 请求=%d, 成功=%d, 操作=%s",
		len(itemIDs), successCount, req.Action)

	return successCount, nil
}

// autoReleaseItem 自动释放低威胁项
func (qm *QuarantineManager) autoReleaseItem(itemID string) {
	// 等待一段时间后自动释放
	time.Sleep(5 * time.Minute)

	qm.mu.Lock()
	defer qm.mu.Unlock()

	item, ok := qm.items[itemID]
	if !ok || item.Status != QuarantineStatusPending {
		return
	}

	item.Status = QuarantineStatusReleased
	item.ReviewedBy = "system-auto"
	item.ReviewNote = "低威胁自动释放"
	item.UpdatedAt = time.Now()

	log.Printf("[隔离管理] 低威胁邮件自动释放: id=%s", itemID)
}

// determineThreatLevel 根据评分确定威胁等级
func (qm *QuarantineManager) determineThreatLevel(score int) string {
	switch {
	case score >= 80:
		return ThreatLevelCritical
	case score >= 60:
		return ThreatLevelHigh
	case score >= 30:
		return ThreatLevelMedium
	default:
		return ThreatLevelLow
	}
}

// generateQuarantineReason 生成隔离原因
func (qm *QuarantineManager) generateQuarantineReason(threats []ThreatItem) string {
	if len(threats) == 0 {
		return "未知威胁"
	}

	reasons := make([]string, 0)
	for _, t := range threats {
		reasons = append(reasons, t.Name)
	}

	if len(reasons) > 3 {
		return fmt.Sprintf("%s 等 %d 项威胁", reasons[0], len(reasons))
	}

	return fmt.Sprintf("%v", reasons)
}

// ExportQuarantineLog 导出隔离日志
func (qm *QuarantineManager) ExportQuarantineLog(startTime, endTime time.Time) []QuarantineItem {
	qm.mu.RLock()
	defer qm.mu.RUnlock()

	logs := make([]QuarantineItem, 0)
	for _, item := range qm.items {
		if item.CreatedAt.After(startTime) && item.CreatedAt.Before(endTime) {
			logs = append(logs, *item)
		}
	}

	return logs
}
