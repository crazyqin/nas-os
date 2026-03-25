package tiering

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// AccessTracker 访问频率追踪器.
type AccessTracker struct {
	mu sync.RWMutex

	config PolicyEngineConfig

	// 文件访问记录
	records map[string]*FileAccessRecord

	// 按存储层索引
	byTier map[TierType]map[string]*FileAccessRecord

	// 统计
	stats *AccessStats

	// 存储
	dataPath string

	// 运行状态
	running bool
	stopCh  chan struct{}
}

// NewAccessTracker 创建访问追踪器.
func NewAccessTracker(config PolicyEngineConfig) *AccessTracker {
	return &AccessTracker{
		config:   config,
		records:  make(map[string]*FileAccessRecord),
		byTier:   make(map[TierType]map[string]*FileAccessRecord),
		dataPath: "/var/lib/nas-os/tiering/access_records.json",
		stats: &AccessStats{
			ByTier: make(map[TierType]*TierStats),
		},
		stopCh: make(chan struct{}),
	}
}

// Start 启动追踪器.
func (t *AccessTracker) Start() error {
	t.mu.Lock()
	defer t.mu.Unlock()

	// 加载历史记录
	if err := t.loadRecords(); err != nil {
		// 文件不存在是正常的
		t.records = make(map[string]*FileAccessRecord)
		t.byTier = make(map[TierType]map[string]*FileAccessRecord)
	}

	// 初始化存储层索引
	for _, tier := range []TierType{TierTypeSSD, TierTypeHDD, TierTypeCloud} {
		if t.byTier[tier] == nil {
			t.byTier[tier] = make(map[string]*FileAccessRecord)
		}
	}

	t.running = true

	// 启动定时保存和统计更新
	go t.runBackgroundTasks()

	return nil
}

// Stop 停止追踪器.
func (t *AccessTracker) Stop() {
	t.mu.Lock()
	t.running = false
	t.mu.Unlock()

	close(t.stopCh)

	// 保存记录
	_ = t.saveRecords()
}

// ==================== 访问记录操作 ====================

// RecordAccess 记录文件访问.
func (t *AccessTracker) RecordAccess(path string, tier TierType, readBytes, writeBytes int64) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	record, ok := t.records[path]
	if !ok {
		// 新文件
		record = &FileAccessRecord{
			Path:        path,
			CurrentTier: tier,
			AccessTime:  time.Now(),
		}

		// 获取文件信息
		if info, err := os.Stat(path); err == nil {
			record.Size = info.Size()
			record.ModTime = info.ModTime()
		}

		t.records[path] = record

		// 更新存储层索引
		if t.byTier[tier] == nil {
			t.byTier[tier] = make(map[string]*FileAccessRecord)
		}
		t.byTier[tier][path] = record
	}

	// 更新访问统计
	record.AccessCount++
	record.ReadBytes += readBytes
	record.WriteBytes += writeBytes
	record.AccessTime = time.Now()
	record.LastModified = time.Now()

	// 更新访问频率
	record.Frequency = t.calculateFrequency(record)

	return nil
}

// RecordFileRead 记录文件读取.
func (t *AccessTracker) RecordFileRead(path string, tier TierType, bytesRead int64) error {
	return t.RecordAccess(path, tier, bytesRead, 0)
}

// RecordFileWrite 记录文件写入.
func (t *AccessTracker) RecordFileWrite(path string, tier TierType, bytesWritten int64) error {
	return t.RecordAccess(path, tier, 0, bytesWritten)
}

// GetRecord 获取文件访问记录.
func (t *AccessTracker) GetRecord(path string) (*FileAccessRecord, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	record, ok := t.records[path]
	if !ok {
		return nil, fmt.Errorf("文件访问记录不存在: %s", path)
	}

	return record, nil
}

// GetRecordsByTier 获取指定存储层的所有记录.
func (t *AccessTracker) GetRecordsByTier(tier TierType) []*FileAccessRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var records []*FileAccessRecord
	if tierRecords, ok := t.byTier[tier]; ok {
		for _, record := range tierRecords {
			records = append(records, record)
		}
	}

	return records
}

// GetHotFiles 获取热数据文件.
func (t *AccessTracker) GetHotFiles(tier TierType, limit int) []*FileAccessRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var files []*FileAccessRecord
	if tierRecords, ok := t.byTier[tier]; ok {
		for _, record := range tierRecords {
			if record.Frequency == AccessFrequencyHot {
				files = append(files, record)
			}
			if limit > 0 && len(files) >= limit {
				break
			}
		}
	}

	return files
}

// GetColdFiles 获取冷数据文件.
func (t *AccessTracker) GetColdFiles(tier TierType, limit int) []*FileAccessRecord {
	t.mu.RLock()
	defer t.mu.RUnlock()

	var files []*FileAccessRecord
	if tierRecords, ok := t.byTier[tier]; ok {
		for _, record := range tierRecords {
			if record.Frequency == AccessFrequencyCold {
				files = append(files, record)
			}
			if limit > 0 && len(files) >= limit {
				break
			}
		}
	}

	return files
}

// UpdateFileTier 更新文件存储层.
func (t *AccessTracker) UpdateFileTier(path string, newTier TierType) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	record, ok := t.records[path]
	if !ok {
		return fmt.Errorf("文件访问记录不存在: %s", path)
	}

	// 从旧存储层索引移除
	if oldRecords, ok := t.byTier[record.CurrentTier]; ok {
		delete(oldRecords, path)
	}

	// 更新存储层
	record.CurrentTier = newTier

	// 添加到新存储层索引
	if t.byTier[newTier] == nil {
		t.byTier[newTier] = make(map[string]*FileAccessRecord)
	}
	t.byTier[newTier][path] = record

	return nil
}

// RemoveRecord 移除访问记录.
func (t *AccessTracker) RemoveRecord(path string) error {
	t.mu.Lock()
	defer t.mu.Unlock()

	record, ok := t.records[path]
	if !ok {
		return nil
	}

	// 从存储层索引移除
	if tierRecords, ok := t.byTier[record.CurrentTier]; ok {
		delete(tierRecords, path)
	}

	// 删除记录
	delete(t.records, path)

	return nil
}

// ==================== 统计 ====================

// GetStats 获取访问统计.
func (t *AccessTracker) GetStats() *AccessStats {
	t.mu.RLock()
	defer t.mu.RUnlock()

	// 更新统计
	t.updateStatsLocked()

	return t.stats
}

// GetTierStats 获取存储层统计.
func (t *AccessTracker) GetTierStats(tier TierType, config *TierConfig) (*TierStats, error) {
	t.mu.RLock()
	defer t.mu.RUnlock()

	stats := &TierStats{
		Type:     tier,
		Name:     config.Name,
		Capacity: config.Capacity,
	}

	tierRecords, ok := t.byTier[tier]
	if !ok {
		return stats, nil
	}

	for _, record := range tierRecords {
		stats.TotalFiles++
		stats.TotalBytes += record.Size

		switch record.Frequency {
		case AccessFrequencyHot:
			stats.HotFiles++
			stats.HotBytes += record.Size
		case AccessFrequencyWarm:
			stats.WarmFiles++
			stats.WarmBytes += record.Size
		case AccessFrequencyCold:
			stats.ColdFiles++
			stats.ColdBytes += record.Size
		}
	}

	stats.Used = stats.TotalBytes
	if stats.Capacity > 0 {
		stats.Available = stats.Capacity - stats.Used
		stats.UsagePercent = float64(stats.Used) / float64(stats.Capacity) * 100
	}

	return stats, nil
}

// updateStatsLocked 更新统计（需要锁）.
func (t *AccessTracker) updateStatsLocked() {
	stats := &AccessStats{
		LastUpdated: time.Now(),
		ByTier:      make(map[TierType]*TierStats),
	}

	for _, record := range t.records {
		stats.TotalFiles++
		stats.TotalAccesses += record.AccessCount
		stats.TotalReadBytes += record.ReadBytes
		stats.TotalWriteBytes += record.WriteBytes

		switch record.Frequency {
		case AccessFrequencyHot:
			stats.HotFiles++
		case AccessFrequencyWarm:
			stats.WarmFiles++
		case AccessFrequencyCold:
			stats.ColdFiles++
		}
	}

	// 按 Tier 统计
	for tier, records := range t.byTier {
		tierStats := &TierStats{Type: tier}
		for _, record := range records {
			tierStats.TotalFiles++
			tierStats.TotalBytes += record.Size
		}
		stats.ByTier[tier] = tierStats
	}

	t.stats = stats
}

// calculateFrequency 计算访问频率.
func (t *AccessTracker) calculateFrequency(record *FileAccessRecord) AccessFrequency {
	// 基于访问次数和时间判断
	accessCount := record.AccessCount
	age := time.Since(record.AccessTime)

	// 热数据：访问次数高且近期有访问
	if accessCount >= t.config.HotThreshold && age < 24*time.Hour {
		return AccessFrequencyHot
	}

	// 冷数据：长时间未访问
	coldDuration := time.Duration(t.config.ColdAgeHours) * time.Hour
	if age > coldDuration {
		return AccessFrequencyCold
	}

	// 温数据
	return AccessFrequencyWarm
}

// ==================== 后台任务 ====================

func (t *AccessTracker) runBackgroundTasks() {
	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-t.stopCh:
			return
		case <-ticker.C:
			t.updateFrequencies()
			_ = t.saveRecords()
		}
	}
}

// updateFrequencies 更新访问频率.
func (t *AccessTracker) updateFrequencies() {
	t.mu.Lock()
	defer t.mu.Unlock()

	for _, record := range t.records {
		record.Frequency = t.calculateFrequency(record)
	}
}

// ==================== 持久化 ====================

func (t *AccessTracker) loadRecords() error {
	data, err := os.ReadFile(t.dataPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &t.records)
}

func (t *AccessTracker) saveRecords() error {
	t.mu.RLock()
	defer t.mu.RUnlock()

	data, err := json.MarshalIndent(t.records, "", "  ")
	if err != nil {
		return err
	}

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(t.dataPath), 0750); err != nil {
		return err
	}

	return os.WriteFile(t.dataPath, data, 0640)
}
