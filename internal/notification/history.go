package notification

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// HistoryManager 历史记录管理器
type HistoryManager struct {
	records    []*Record
	recordMap  map[string]*Record
	mu         sync.RWMutex
	storePath  string
	maxRecords int
	maxDays    int
}

// NewHistoryManager 创建历史记录管理器
func NewHistoryManager(storePath string, maxRecords, maxDays int) (*HistoryManager, error) {
	if maxRecords <= 0 {
		maxRecords = 10000
	}
	if maxDays <= 0 {
		maxDays = 30
	}

	hm := &HistoryManager{
		records:    make([]*Record, 0),
		recordMap:  make(map[string]*Record),
		storePath:  storePath,
		maxRecords: maxRecords,
		maxDays:    maxDays,
	}

	if err := hm.load(); err != nil {
		return nil, fmt.Errorf("加载历史记录失败: %w", err)
	}

	// 启动定期清理
	go hm.periodicCleanup()

	return hm, nil
}

// load 加载历史记录
func (hm *HistoryManager) load() error {
	if hm.storePath == "" {
		return nil
	}

	data, err := os.ReadFile(hm.storePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var records []*Record
	if err := json.Unmarshal(data, &records); err != nil {
		return err
	}

	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.records = records
	hm.recordMap = make(map[string]*Record)
	for _, r := range records {
		hm.recordMap[r.ID] = r
	}

	return nil
}

// save 保存历史记录
func (hm *HistoryManager) save() error {
	if hm.storePath == "" {
		return nil
	}

	hm.mu.RLock()
	records := make([]*Record, len(hm.records))
	copy(records, hm.records)
	hm.mu.RUnlock()

	data, err := json.MarshalIndent(records, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(hm.storePath)
	if err := os.MkdirAll(dir, 0750); err != nil {
		return err
	}

	return os.WriteFile(hm.storePath, data, 0640)
}

// AddRecord 添加记录
func (hm *HistoryManager) AddRecord(record *Record) error {
	if record.ID == "" {
		record.ID = GenerateID()
	}

	record.CreatedAt = time.Now()
	record.UpdatedAt = time.Now()

	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.records = append(hm.records, record)
	hm.recordMap[record.ID] = record

	// 检查是否需要清理
	if len(hm.records) > hm.maxRecords {
		hm.cleanup()
	}

	return hm.save()
}

// UpdateRecord 更新记录
func (hm *HistoryManager) UpdateRecord(record *Record) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if _, exists := hm.recordMap[record.ID]; !exists {
		return fmt.Errorf("记录不存在: %s", record.ID)
	}

	record.UpdatedAt = time.Now()
	hm.recordMap[record.ID] = record

	for i, r := range hm.records {
		if r.ID == record.ID {
			hm.records[i] = record
			break
		}
	}

	return hm.save()
}

// GetRecord 获取记录
func (hm *HistoryManager) GetRecord(id string) (*Record, error) {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	record, exists := hm.recordMap[id]
	if !exists {
		return nil, fmt.Errorf("记录不存在: %s", id)
	}

	return record, nil
}

// DeleteRecord 删除记录
func (hm *HistoryManager) DeleteRecord(id string) error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	if _, exists := hm.recordMap[id]; !exists {
		return fmt.Errorf("记录不存在: %s", id)
	}

	delete(hm.recordMap, id)

	for i, r := range hm.records {
		if r.ID == id {
			hm.records = append(hm.records[:i], hm.records[i+1:]...)
			break
		}
	}

	return hm.save()
}

// Query 查询记录
func (hm *HistoryManager) Query(filter *HistoryFilter) []*Record {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result := make([]*Record, 0)

	for _, r := range hm.records {
		if !hm.matchFilter(r, filter) {
			continue
		}
		result = append(result, r)
	}

	// 按时间倒序排序
	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	// 分页
	page := filter.Page
	if page < 1 {
		page = 1
	}
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}

	start := (page - 1) * pageSize
	end := start + pageSize

	if start >= len(result) {
		return []*Record{}
	}

	if end > len(result) {
		end = len(result)
	}

	return result[start:end]
}

// matchFilter 匹配过滤条件
func (hm *HistoryManager) matchFilter(record *Record, filter *HistoryFilter) bool {
	if filter == nil {
		return true
	}

	// 时间范围
	if filter.StartTime != nil && record.CreatedAt.Before(*filter.StartTime) {
		return false
	}
	if filter.EndTime != nil && record.CreatedAt.After(*filter.EndTime) {
		return false
	}

	// 状态
	if filter.Status != "" && record.Status != filter.Status {
		return false
	}

	// 渠道
	if filter.Channel != "" && record.Channel != filter.Channel {
		return false
	}

	// 级别
	if filter.Level != "" && record.Notification != nil && record.Notification.Level != filter.Level {
		return false
	}

	// 类别
	if filter.Category != "" && record.Notification != nil && record.Notification.Category != filter.Category {
		return false
	}

	// 来源
	if filter.Source != "" && record.Notification != nil && record.Notification.Source != filter.Source {
		return false
	}

	// 搜索
	if filter.Search != "" && record.Notification != nil {
		searchLower := filter.Search
		titleMatch := containsIgnoreCase(record.Notification.Title, searchLower)
		messageMatch := containsIgnoreCase(record.Notification.Message, searchLower)
		if !titleMatch && !messageMatch {
			return false
		}
	}

	return true
}

// GetStats 获取统计
func (hm *HistoryManager) GetStats(startTime, endTime *time.Time) *HistoryStats {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	stats := &HistoryStats{
		ChannelStats: make(map[ChannelType]int),
		LevelStats:   make(map[Level]int),
		DailyStats:   make([]DailyStat, 0),
	}

	for _, r := range hm.records {
		// 时间范围过滤
		if startTime != nil && r.CreatedAt.Before(*startTime) {
			continue
		}
		if endTime != nil && r.CreatedAt.After(*endTime) {
			continue
		}

		stats.TotalCount++

		switch r.Status {
		case StatusSent:
			stats.SuccessCount++
		case StatusFailed:
			stats.FailedCount++
		case StatusPending, StatusRetrying:
			stats.PendingCount++
		}

		stats.ChannelStats[r.Channel]++

		if r.Notification != nil {
			stats.LevelStats[r.Notification.Level]++
		}
	}

	// 计算每日统计
	stats.DailyStats = hm.calculateDailyStats(startTime, endTime)

	return stats
}

// calculateDailyStats 计算每日统计
func (hm *HistoryManager) calculateDailyStats(startTime, endTime *time.Time) []DailyStat {
	dailyMap := make(map[string]*DailyStat)

	for _, r := range hm.records {
		if startTime != nil && r.CreatedAt.Before(*startTime) {
			continue
		}
		if endTime != nil && r.CreatedAt.After(*endTime) {
			continue
		}

		date := r.CreatedAt.Format("2006-01-02")
		if _, exists := dailyMap[date]; !exists {
			dailyMap[date] = &DailyStat{Date: date}
		}

		dailyMap[date].Count++
		switch r.Status {
		case StatusSent:
			dailyMap[date].Success++
		case StatusFailed:
			dailyMap[date].Failed++
		}
	}

	// 转换为切片并排序
	result := make([]DailyStat, 0, len(dailyMap))
	for _, stat := range dailyMap {
		result = append(result, *stat)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].Date < result[j].Date
	})

	return result
}

// cleanup 清理过期记录
func (hm *HistoryManager) cleanup() {
	cutoff := time.Now().AddDate(0, 0, -hm.maxDays)

	newRecords := make([]*Record, 0)
	for _, r := range hm.records {
		if r.CreatedAt.After(cutoff) {
			newRecords = append(newRecords, r)
		} else {
			delete(hm.recordMap, r.ID)
		}
	}

	// 如果记录数仍然超限，删除最旧的记录
	if len(newRecords) > hm.maxRecords {
		// 按时间排序
		sort.Slice(newRecords, func(i, j int) bool {
			return newRecords[i].CreatedAt.After(newRecords[j].CreatedAt)
		})

		// 删除多余的记录
		for i := hm.maxRecords; i < len(newRecords); i++ {
			delete(hm.recordMap, newRecords[i].ID)
		}
		newRecords = newRecords[:hm.maxRecords]
	}

	hm.records = newRecords
}

// periodicCleanup 定期清理
func (hm *HistoryManager) periodicCleanup() {
	ticker := time.NewTicker(time.Hour)
	defer ticker.Stop()

	for range ticker.C {
		hm.mu.Lock()
		hm.cleanup()
		hm.mu.Unlock()
		_ = hm.save()
	}
}

// Clear 清空历史记录
func (hm *HistoryManager) Clear() error {
	hm.mu.Lock()
	defer hm.mu.Unlock()

	hm.records = make([]*Record, 0)
	hm.recordMap = make(map[string]*Record)

	return hm.save()
}

// Count 获取记录总数
func (hm *HistoryManager) Count() int {
	hm.mu.RLock()
	defer hm.mu.RUnlock()
	return len(hm.records)
}

// GetRecordsByNotificationID 根据通知ID获取记录
func (hm *HistoryManager) GetRecordsByNotificationID(notificationID string) []*Record {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result := make([]*Record, 0)
	for _, r := range hm.records {
		if r.NotificationID == notificationID {
			result = append(result, r)
		}
	}

	return result
}

// GetPendingRecords 获取待发送记录
func (hm *HistoryManager) GetPendingRecords() []*Record {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result := make([]*Record, 0)
	for _, r := range hm.records {
		if r.Status == StatusPending || r.Status == StatusRetrying {
			result = append(result, r)
		}
	}

	return result
}

// GetFailedRecords 获取失败记录
func (hm *HistoryManager) GetFailedRecords(limit int) []*Record {
	hm.mu.RLock()
	defer hm.mu.RUnlock()

	result := make([]*Record, 0)
	for _, r := range hm.records {
		if r.Status == StatusFailed {
			result = append(result, r)
			if limit > 0 && len(result) >= limit {
				break
			}
		}
	}

	return result
}

// 辅助函数

func containsIgnoreCase(s, substr string) bool {
	sLower := s
	substrLower := substr
	return containsSubstring(sLower, substrLower)
}

func containsSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
