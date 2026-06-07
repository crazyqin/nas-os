// Package ransomware - 快照异常检测模块
// 对标TrueNAS 26 Ransomware Defense的快照空间异常检测
package ransomware

import (
	"context"
	"fmt"
	"sync"
	"time"
)

// SnapshotAnomalyDetector 快照异常检测器
// 监控快照空间增长速率，检测突然大量数据变化（勒索加密典型特征）
type SnapshotAnomalyDetector struct {
	config       SnapshotAnomalyConfig
	zfsAdapter   ZFSAdapterInterface
	historyStore *SnapshotHistoryStore
	thresholds   SnapshotThresholds
	running      bool
	mu           sync.RWMutex
	ctx          context.Context
	cancel       context.CancelFunc
	stats        AnomalyStats
	statsMu      sync.RWMutex
	alertChan    chan SnapshotAnomaly
}

// SnapshotAnomalyConfig 快照异常检测配置
type SnapshotAnomalyConfig struct {
	Enabled             bool          `json:"enabled"`
	CheckInterval       time.Duration `json:"check_interval"`        // 检查间隔
	HistoryWindow       time.Duration `json:"history_window"`        // 历史记录窗口
	SizeIncreasePercent float64       `json:"size_increase_percent"` // 大小突增阈值（百分比）
	CountIncreaseRate   int           `json:"count_increase_rate"`   // 快照数量突增阈值
	SpaceUsedPercent    float64       `json:"space_used_percent"`    // 空间占用率阈值
	DeletionAlert       bool          `json:"deletion_alert"`        // 快照删除告警
	MonitorDatasets     []string      `json:"monitor_datasets"`      // 监控的数据集
	ExcludeDatasets     []string      `json:"exclude_datasets"`      // 排除的数据集
}

// DefaultSnapshotAnomalyConfig 默认配置
func DefaultSnapshotAnomalyConfig() SnapshotAnomalyConfig {
	return SnapshotAnomalyConfig{
		Enabled:             true,
		CheckInterval:       5 * time.Minute,
		HistoryWindow:       24 * time.Hour,
		SizeIncreasePercent: 500.0, // 500%突增告警（勒索典型：大量文件被加密）
		CountIncreaseRate:   10,    // 每分钟新增10个快照告警
		SpaceUsedPercent:    80.0,  // 80%空间占用告警
		DeletionAlert:       true,  // 启用快照删除检测
		MonitorDatasets:     []string{"tank/data", "tank/backup", "tank/shares"},
		ExcludeDatasets:     []string{},
	}
}

// SnapshotThresholds 快照阈值
type SnapshotThresholds struct {
	// 快照大小突增阈值（相对上次快照）
	SizeIncreasePercent float64
	// 快照数量突增阈值
	CountIncreaseRate int
	// 空间占用率阈值
	SpaceUsedPercent float64
	// 快照删除检测
	DeletionAlert bool
	// 连续异常阈值（连续N次异常触发告警）
	ConsecutiveAnomalyThreshold int
}

// SnapshotAnomaly 快照异常类型
type SnapshotAnomaly struct {
	ID               string            `json:"id"`
	Type             AnomalyType       `json:"type"`
	Dataset          string            `json:"dataset"`
	Snapshot         string            `json:"snapshot,omitempty"`
	Rate             float64           `json:"rate,omitempty"`
	Threshold        float64           `json:"threshold"`
	Details          map[string]string `json:"details,omitempty"`
	Timestamp        time.Time         `json:"timestamp"`
	ThreatLevel      ThreatLevel       `json:"threat_level"`
	SuggestedAction  string            `json:"suggested_action"`
	DeletedSnapshots []string          `json:"deleted_snapshots,omitempty"`
}

// AnomalyType 异常类型
type AnomalyType string

const (
	AnomalySizeIncrease     AnomalyType = "size_increase"     // 快照大小突增
	AnomalyCountIncrease    AnomalyType = "count_increase"    // 快照数量突增
	AnomalySpaceUsed        AnomalyType = "space_used"        // 空间占用过高
	AnomalySnapshotDeletion AnomalyType = "snapshot_deletion" // 快照被删除
	AnomalyRapidChange      AnomalyType = "rapid_change"      // 快速数据变化
)

// AnomalyStats 异常统计
type AnomalyStats struct {
	TotalChecks       int64                 `json:"total_checks"`
	AnomaliesDetected int64                 `json:"anomalies_detected"`
	ByType            map[AnomalyType]int64 `json:"by_type"`
	ByDataset         map[string]int64      `json:"by_dataset"`
	LastCheckTime     *time.Time            `json:"last_check_time,omitempty"`
	LastAnomalyTime   *time.Time            `json:"last_anomaly_time,omitempty"`
	AlertsSent        int64                 `json:"alerts_sent"`
}

// ZFSAdapterInterface ZFS适配器接口
type ZFSAdapterInterface interface {
	ListSnapshots(ctx context.Context, dataset string) ([]SnapshotInfo, error)
	GetDatasetUsage(ctx context.Context, dataset string) (*DatasetUsage, error)
	CreateSnapshot(ctx context.Context, dataset, name string) (string, error)
	DestroySnapshot(ctx context.Context, snapshot string) error
	HoldSnapshot(ctx context.Context, snapshot, tag string) error
	ReleaseSnapshot(ctx context.Context, snapshot, tag string) error
	GetSnapshotUsed(ctx context.Context, snapshot string) (int64, error)
}

// SnapshotInfo 快照信息
type SnapshotInfo struct {
	Name       string    `json:"name"`
	Dataset    string    `json:"dataset"`
	Created    time.Time `json:"created"`
	Used       int64     `json:"used"`               // 已用空间（字节）
	Referenced int64     `json:"referenced"`         // 引用空间（字节）
	Written    int64     `json:"written"`            // 写入数据量
	HoldTag    string    `json:"hold_tag,omitempty"` // Hold标签
}

// DatasetUsage 数据集使用情况
type DatasetUsage struct {
	Dataset         string  `json:"dataset"`
	Used            int64   `json:"used"`
	Available       int64   `json:"available"`
	TotalReferenced int64   `json:"total_referenced"`
	UsedPercent     float64 `json:"used_percent"`
	SnapshotCount   int     `json:"snapshot_count"`
}

// SnapshotHistoryStore 快照历史存储
type SnapshotHistoryStore struct {
	records map[string][]SnapshotRecord // dataset -> records
	mu      sync.RWMutex
	maxAge  time.Duration
}

// SnapshotRecord 快照记录
type SnapshotRecord struct {
	SnapshotName string    `json:"snapshot_name"`
	Dataset      string    `json:"dataset"`
	Used         int64     `json:"used"`
	Created      time.Time `json:"created"`
	RecordedAt   time.Time `json:"recorded_at"`
}

// NewSnapshotAnomalyDetector 创建快照异常检测器
func NewSnapshotAnomalyDetector(config SnapshotAnomalyConfig) *SnapshotAnomalyDetector {
	detector := &SnapshotAnomalyDetector{
		config:       config,
		historyStore: NewSnapshotHistoryStore(config.HistoryWindow),
		thresholds: SnapshotThresholds{
			SizeIncreasePercent:         config.SizeIncreasePercent,
			CountIncreaseRate:           config.CountIncreaseRate,
			SpaceUsedPercent:            config.SpaceUsedPercent,
			DeletionAlert:               config.DeletionAlert,
			ConsecutiveAnomalyThreshold: 3, // 连续3次异常触发告警
		},
		alertChan: make(chan SnapshotAnomaly, 100),
		stats: AnomalyStats{
			ByType:    make(map[AnomalyType]int64),
			ByDataset: make(map[string]int64),
		},
	}
	return detector
}

// SetZFSAdapter 设置ZFS适配器
func (d *SnapshotAnomalyDetector) SetZFSAdapter(adapter ZFSAdapterInterface) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.zfsAdapter = adapter
}

// Start 启动检测
func (d *SnapshotAnomalyDetector) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return nil
	}
	d.ctx, d.cancel = context.WithCancel(ctx)
	d.running = true
	d.mu.Unlock()

	// 启动检测循环
	go d.detectLoop()

	// 启动清理循环
	go d.cleanupLoop()

	return nil
}

// Stop 停止检测
func (d *SnapshotAnomalyDetector) Stop() {
	d.mu.Lock()
	defer d.mu.Unlock()

	if !d.running {
		return
	}

	d.running = false
	if d.cancel != nil {
		d.cancel()
	}
	close(d.alertChan)
}

// detectLoop 检测循环
func (d *SnapshotAnomalyDetector) detectLoop() {
	ticker := time.NewTicker(d.config.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			anomalies := d.Detect(d.ctx)
			for _, anomaly := range anomalies {
				select {
				case d.alertChan <- anomaly:
					d.statsMu.Lock()
					d.stats.AlertsSent++
					d.statsMu.Unlock()
				default:
					// 通道满，丢弃
				}
			}
		}
	}
}

// cleanupLoop 清理循环
func (d *SnapshotAnomalyDetector) cleanupLoop() {
	ticker := time.NewTicker(1 * time.Hour)
	defer ticker.Stop()

	for {
		select {
		case <-d.ctx.Done():
			return
		case <-ticker.C:
			d.historyStore.Cleanup()
		}
	}
}

// Detect 执行检测
func (d *SnapshotAnomalyDetector) Detect(ctx context.Context) []SnapshotAnomaly {
	d.mu.RLock()
	zfsAdapter := d.zfsAdapter
	running := d.running
	d.mu.RUnlock()

	if !running || zfsAdapter == nil {
		return nil
	}

	anomalies := []SnapshotAnomaly{}
	now := time.Now()

	// 更新统计
	d.statsMu.Lock()
	d.stats.TotalChecks++
	d.stats.LastCheckTime = &now
	d.statsMu.Unlock()

	for _, dataset := range d.config.MonitorDatasets {
		// 检查是否排除
		if d.isExcluded(dataset) {
			continue
		}

		// 1. 获取当前快照列表
		snapshots, err := zfsAdapter.ListSnapshots(ctx, dataset)
		if err != nil {
			continue
		}

		// 2. 获取数据集使用情况
		usage, err := zfsAdapter.GetDatasetUsage(ctx, dataset)
		if err != nil {
			continue
		}

		// 3. 检测空间占用异常
		if usage.UsedPercent >= d.thresholds.SpaceUsedPercent {
			anomaly := SnapshotAnomaly{
				ID:              generateAnomalyID(),
				Type:            AnomalySpaceUsed,
				Dataset:         dataset,
				Rate:            usage.UsedPercent,
				Threshold:       d.thresholds.SpaceUsedPercent,
				Timestamp:       now,
				ThreatLevel:     ThreatLevelHigh,
				SuggestedAction: "数据集空间占用过高，勒索攻击可能导致空间耗尽。建议立即检查数据状态。",
				Details: map[string]string{
					"used_bytes":      fmt.Sprintf("%d", usage.Used),
					"available_bytes": fmt.Sprintf("%d", usage.Available),
					"snapshot_count":  fmt.Sprintf("%d", usage.SnapshotCount),
				},
			}
			anomalies = append(anomalies, anomaly)
			d.updateStats(anomaly)
		}

		// 4. 检测快照大小突增（勒索典型：大量文件被加密）
		anomalies = append(anomalies, d.detectSizeIncrease(dataset, snapshots, now)...)

		// 5. 检测快照数量突增
		anomalies = append(anomalies, d.detectCountIncrease(dataset, snapshots, now)...)

		// 6. 检测快照删除（勒索可能删除快照阻止恢复）
		if d.thresholds.DeletionAlert {
			anomalies = append(anomalies, d.detectSnapshotDeletion(ctx, dataset, snapshots, now)...)
		}

		// 7. 记录历史
		d.recordHistory(dataset, snapshots, now)
	}

	return anomalies
}

// detectSizeIncrease 检测快照大小突增
func (d *SnapshotAnomalyDetector) detectSizeIncrease(dataset string, snapshots []SnapshotInfo, now time.Time) []SnapshotAnomaly {
	anomalies := []SnapshotAnomaly{}

	// 获取历史记录
	history := d.historyStore.Get(dataset)
	if len(history) < 2 {
		return anomalies
	}

	// 计算大小增长率
	for i := 1; i < len(snapshots); i++ {
		current := snapshots[i]
		prev := snapshots[i-1]

		if prev.Used == 0 {
			continue
		}

		// 计算时间差（分钟）
		timeDiff := current.Created.Sub(prev.Created).Minutes()

		// 计算增长百分比
		increaseRate := float64(current.Used-prev.Used) / float64(prev.Used) * 100

		// 检测异常突增
		if increaseRate >= d.thresholds.SizeIncreasePercent {
			// 勒索攻击特征：短时间内大量数据变化

			threatLevel := ThreatLevelCritical
			if timeDiff > 60 { // 超过1小时的变化可能是正常操作
				threatLevel = ThreatLevelHigh
			}

			anomaly := SnapshotAnomaly{
				ID:              generateAnomalyID(),
				Type:            AnomalySizeIncrease,
				Dataset:         dataset,
				Snapshot:        current.Name,
				Rate:            increaseRate,
				Threshold:       d.thresholds.SizeIncreasePercent,
				Timestamp:       now,
				ThreatLevel:     threatLevel,
				SuggestedAction: "快照大小突增异常，疑似勒索攻击正在加密文件。立即检查数据状态，锁定快照，准备恢复。",
				Details: map[string]string{
					"current_used":  fmt.Sprintf("%d", current.Used),
					"previous_used": fmt.Sprintf("%d", prev.Used),
					"time_diff_min": fmt.Sprintf("%.2f", timeDiff),
					"increase_rate": fmt.Sprintf("%.2f%%", increaseRate),
				},
			}
			anomalies = append(anomalies, anomaly)
			d.updateStats(anomaly)
		}

		// 检测快速数据变化（短时间内大量写入）
		if current.Written > 0 && timeDiff < 10 && current.Written > 100*1024*1024 { // 10分钟内写入超过100MB
			anomaly := SnapshotAnomaly{
				ID:              generateAnomalyID(),
				Type:            AnomalyRapidChange,
				Dataset:         dataset,
				Snapshot:        current.Name,
				Rate:            float64(current.Written),
				Threshold:       100 * 1024 * 1024,
				Timestamp:       now,
				ThreatLevel:     ThreatLevelHigh,
				SuggestedAction: "短时间内大量数据写入，疑似勒索攻击。建议加强监控。",
				Details: map[string]string{
					"written_bytes": fmt.Sprintf("%d", current.Written),
					"time_diff_min": fmt.Sprintf("%.2f", timeDiff),
				},
			}
			anomalies = append(anomalies, anomaly)
			d.updateStats(anomaly)
		}
	}

	return anomalies
}

// detectCountIncrease 检测快照数量突增
func (d *SnapshotAnomalyDetector) detectCountIncrease(dataset string, snapshots []SnapshotInfo, now time.Time) []SnapshotAnomaly {
	anomalies := []SnapshotAnomaly{}

	// 统计最近时间窗口内的新增快照
	windowStart := now.Add(-d.config.CheckInterval)
	newCount := 0

	for _, snap := range snapshots {
		if snap.Created.After(windowStart) {
			newCount++
		}
	}

	// 检测异常
	if newCount >= d.thresholds.CountIncreaseRate {
		anomaly := SnapshotAnomaly{
			ID:              generateAnomalyID(),
			Type:            AnomalyCountIncrease,
			Dataset:         dataset,
			Rate:            float64(newCount),
			Threshold:       float64(d.thresholds.CountIncreaseRate),
			Timestamp:       now,
			ThreatLevel:     ThreatLevelMedium,
			SuggestedAction: "短时间内创建大量快照，可能存在异常操作。建议检查快照创建原因。",
			Details: map[string]string{
				"new_snapshots":   fmt.Sprintf("%d", newCount),
				"total_snapshots": fmt.Sprintf("%d", len(snapshots)),
				"window_minutes":  fmt.Sprintf("%.0f", d.config.CheckInterval.Minutes()),
			},
		}
		anomalies = append(anomalies, anomaly)
		d.updateStats(anomaly)
	}

	return anomalies
}

// detectSnapshotDeletion 检测快照删除
func (d *SnapshotAnomalyDetector) detectSnapshotDeletion(ctx context.Context, dataset string, currentSnapshots []SnapshotInfo, now time.Time) []SnapshotAnomaly {
	anomalies := []SnapshotAnomaly{}

	// 获取历史记录
	history := d.historyStore.Get(dataset)
	if len(history) == 0 {
		return anomalies
	}

	// 找出被删除的快照
	deletedSnapshots := []string{}
	currentSet := make(map[string]bool)
	for _, snap := range currentSnapshots {
		currentSet[snap.Name] = true
	}

	for _, record := range history {
		if !currentSet[record.SnapshotName] {
			// 检查是否是最近删除的（时间窗口内）
			if now.Sub(record.RecordedAt) <= 2*d.config.CheckInterval {
				deletedSnapshots = append(deletedSnapshots, record.SnapshotName)
			}
		}
	}

	// 如果有快照被删除，触发告警
	if len(deletedSnapshots) > 0 {
		anomaly := SnapshotAnomaly{
			ID:               generateAnomalyID(),
			Type:             AnomalySnapshotDeletion,
			Dataset:          dataset,
			Timestamp:        now,
			ThreatLevel:      ThreatLevelCritical, // 删除快照是勒索攻击的高级指标
			SuggestedAction:  "快照被删除，勒索攻击可能正在尝试阻止数据恢复。立即锁定剩余快照，检查数据状态。",
			DeletedSnapshots: deletedSnapshots,
			Details: map[string]string{
				"deleted_count":   fmt.Sprintf("%d", len(deletedSnapshots)),
				"remaining_count": fmt.Sprintf("%d", len(currentSnapshots)),
			},
		}
		anomalies = append(anomalies, anomaly)
		d.updateStats(anomaly)
	}

	return anomalies
}

// isExcluded 检查是否排除
func (d *SnapshotAnomalyDetector) isExcluded(dataset string) bool {
	for _, excluded := range d.config.ExcludeDatasets {
		if dataset == excluded {
			return true
		}
	}
	return false
}

// recordHistory 记录历史
func (d *SnapshotAnomalyDetector) recordHistory(dataset string, snapshots []SnapshotInfo, now time.Time) {
	for _, snap := range snapshots {
		record := SnapshotRecord{
			SnapshotName: snap.Name,
			Dataset:      snap.Dataset,
			Used:         snap.Used,
			Created:      snap.Created,
			RecordedAt:   now,
		}
		d.historyStore.Add(record)
	}
}

// updateStats 更新统计
func (d *SnapshotAnomalyDetector) updateStats(anomaly SnapshotAnomaly) {
	d.statsMu.Lock()
	defer d.statsMu.Unlock()

	d.stats.AnomaliesDetected++
	d.stats.ByType[anomaly.Type]++
	d.stats.ByDataset[anomaly.Dataset]++
	d.stats.LastAnomalyTime = &anomaly.Timestamp
}

// Alerts 返回告警通道
func (d *SnapshotAnomalyDetector) Alerts() <-chan SnapshotAnomaly {
	return d.alertChan
}

// GetStats 获取统计信息
func (d *SnapshotAnomalyDetector) GetStats() AnomalyStats {
	d.statsMu.RLock()
	defer d.statsMu.RUnlock()
	return d.stats
}

// NewSnapshotHistoryStore 创建快照历史存储
func NewSnapshotHistoryStore(maxAge time.Duration) *SnapshotHistoryStore {
	return &SnapshotHistoryStore{
		records: make(map[string][]SnapshotRecord),
		maxAge:  maxAge,
	}
}

// Add 添加记录
func (s *SnapshotHistoryStore) Add(record SnapshotRecord) {
	s.mu.Lock()
	defer s.mu.Unlock()

	dataset := record.Dataset
	s.records[dataset] = append(s.records[dataset], record)

	// 限制大小
	if len(s.records[dataset]) > 1000 {
		s.records[dataset] = s.records[dataset][len(s.records[dataset])-500:]
	}
}

// Get 获取记录
func (s *SnapshotHistoryStore) Get(dataset string) []SnapshotRecord {
	s.mu.RLock()
	defer s.mu.RUnlock()

	records := s.records[dataset]
	if len(records) == 0 {
		return nil
	}

	// 过滤过期记录
	now := time.Now()
	cutoff := now.Add(-s.maxAge)
	var validRecords []SnapshotRecord
	for _, r := range records {
		if r.RecordedAt.After(cutoff) {
			validRecords = append(validRecords, r)
		}
	}

	return validRecords
}

// Cleanup 清理过期记录
func (s *SnapshotHistoryStore) Cleanup() int {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	cutoff := now.Add(-s.maxAge)
	var cleaned int

	for dataset, records := range s.records {
		var validRecords []SnapshotRecord
		for _, r := range records {
			if r.RecordedAt.After(cutoff) {
				validRecords = append(validRecords, r)
			} else {
				cleaned++
			}
		}
		s.records[dataset] = validRecords
	}

	return cleaned
}

// Helper functions

func generateAnomalyID() string {
	return "anomaly_" + time.Now().Format("20060102150405") + "_" + randomHex(8)
}

func randomHex(n int) string {
	return fmt.Sprintf("%x", time.Now().UnixNano())[:n]
}
