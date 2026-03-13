package tiering

import (
	"sync"
	"sync/atomic"
	"time"
)

// Metrics 存储分层监控指标
type Metrics struct {
	mu sync.RWMutex

	// 存储层指标
	tierMetrics map[TierType]*TierMetrics

	// 迁移指标
	migrationMetrics *MigrationMetrics

	// 策略指标
	policyMetrics map[string]*PolicyMetrics

	// 访问追踪指标
	accessMetrics *AccessMetrics

	// 启动时间
	startTime time.Time
}

// TierMetrics 存储层指标
type TierMetrics struct {
	// 容量
	TotalBytes     int64   `json:"totalBytes"`
	UsedBytes      int64   `json:"usedBytes"`
	AvailableBytes int64   `json:"availableBytes"`
	UsagePercent   float64 `json:"usagePercent"`

	// 文件统计
	TotalFiles int64 `json:"totalFiles"`
	HotFiles   int64 `json:"hotFiles"`
	WarmFiles  int64 `json:"warmFiles"`
	ColdFiles  int64 `json:"coldFiles"`

	// I/O 统计
	ReadBytes  int64 `json:"readBytes"`
	WriteBytes int64 `json:"writeBytes"`
	ReadOps    int64 `json:"readOps"`
	WriteOps   int64 `json:"writeOps"`

	// 迁移统计
	FilesMigratedIn  int64 `json:"filesMigratedIn"`
	FilesMigratedOut int64 `json:"filesMigratedOut"`
	BytesMigratedIn  int64 `json:"bytesMigratedIn"`
	BytesMigratedOut int64 `json:"bytesMigratedOut"`

	// 最后更新
	LastUpdated time.Time `json:"lastUpdated"`
}

// MigrationMetrics 迁移指标
type MigrationMetrics struct {
	// 任务统计
	TotalTasks     int64 `json:"totalTasks"`
	RunningTasks   int64 `json:"runningTasks"`
	CompletedTasks int64 `json:"completedTasks"`
	FailedTasks    int64 `json:"failedTasks"`
	CancelledTasks int64 `json:"cancelledTasks"`

	// 数据量统计
	TotalBytesMigrated int64 `json:"totalBytesMigrated"`
	TotalFilesMigrated int64 `json:"totalFilesMigrated"`
	TotalBytesFailed   int64 `json:"totalBytesFailed"`
	TotalFilesFailed   int64 `json:"totalFilesFailed"`

	// 性能指标
	AverageMigrationTimeMs int64 `json:"averageMigrationTimeMs"`
	TotalMigrationTimeMs   int64 `json:"totalMigrationTimeMs"`

	// 当前运行任务
	CurrentThroughputBytesPerSec int64 `json:"currentThroughputBytesPerSec"`

	// 按策略统计
	ByPolicy map[string]*PolicyMigrationMetrics `json:"byPolicy"`

	// 最后迁移时间
	LastMigrationTime time.Time `json:"lastMigrationTime"`
}

// PolicyMigrationMetrics 策略迁移指标
type PolicyMigrationMetrics struct {
	PolicyID          string    `json:"policyId"`
	ExecutionCount    int64     `json:"executionCount"`
	SuccessCount      int64     `json:"successCount"`
	FailureCount      int64     `json:"failureCount"`
	TotalBytes        int64     `json:"totalBytes"`
	TotalFiles        int64     `json:"totalFiles"`
	LastRunTime       time.Time `json:"lastRunTime"`
	AverageDurationMs int64     `json:"averageDurationMs"`
}

// PolicyMetrics 策略指标
type PolicyMetrics struct {
	PolicyID     string    `json:"policyId"`
	Enabled      bool      `json:"enabled"`
	LastRunTime  time.Time `json:"lastRunTime"`
	NextRunTime  time.Time `json:"nextRunTime"`
	RunCount     int64     `json:"runCount"`
	SuccessCount int64     `json:"successCount"`
	FailureCount int64     `json:"failureCount"`
}

// AccessMetrics 访问指标
type AccessMetrics struct {
	// 总体统计
	TotalFiles      int64 `json:"totalFiles"`
	TotalAccesses   int64 `json:"totalAccesses"`
	TotalReadBytes  int64 `json:"totalReadBytes"`
	TotalWriteBytes int64 `json:"totalWriteBytes"`

	// 频率分布
	HotFiles  int64 `json:"hotFiles"`
	WarmFiles int64 `json:"warmFiles"`
	ColdFiles int64 `json:"coldFiles"`

	// 命中率
	CacheHitRate float64 `json:"cacheHitRate"`

	// 访问模式
	SequentialAccessRatio float64 `json:"sequentialAccessRatio"`
	RandomAccessRatio     float64 `json:"randomAccessRatio"`

	// 最后更新
	LastUpdated time.Time `json:"lastUpdated"`
}

// NewMetrics 创建监控指标实例
func NewMetrics() *Metrics {
	return &Metrics{
		tierMetrics: make(map[TierType]*TierMetrics),
		migrationMetrics: &MigrationMetrics{
			ByPolicy: make(map[string]*PolicyMigrationMetrics),
		},
		policyMetrics: make(map[string]*PolicyMetrics),
		accessMetrics: &AccessMetrics{},
		startTime:     time.Now(),
	}
}

// ==================== 存储层指标 ====================

// UpdateTierMetrics 更新存储层指标
func (m *Metrics) UpdateTierMetrics(tierType TierType, stats *TierStats) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, ok := m.tierMetrics[tierType]
	if !ok {
		metrics = &TierMetrics{}
		m.tierMetrics[tierType] = metrics
	}

	metrics.TotalBytes = stats.Capacity
	metrics.UsedBytes = stats.Used
	metrics.AvailableBytes = stats.Available
	metrics.UsagePercent = stats.UsagePercent
	metrics.TotalFiles = stats.TotalFiles
	metrics.HotFiles = stats.HotFiles
	metrics.WarmFiles = stats.WarmFiles
	metrics.ColdFiles = stats.ColdFiles
	metrics.LastUpdated = time.Now()
}

// GetTierMetrics 获取存储层指标
func (m *Metrics) GetTierMetrics(tierType TierType) *TierMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if metrics, ok := m.tierMetrics[tierType]; ok {
		return metrics
	}
	return &TierMetrics{}
}

// GetAllTierMetrics 获取所有存储层指标
func (m *Metrics) GetAllTierMetrics() map[TierType]*TierMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[TierType]*TierMetrics)
	for k, v := range m.tierMetrics {
		result[k] = v
	}
	return result
}

// RecordTierIO 记录存储层 I/O
func (m *Metrics) RecordTierIO(tierType TierType, readBytes, writeBytes int64, readOps, writeOps int) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, ok := m.tierMetrics[tierType]
	if !ok {
		metrics = &TierMetrics{}
		m.tierMetrics[tierType] = metrics
	}

	metrics.ReadBytes += readBytes
	metrics.WriteBytes += writeBytes
	metrics.ReadOps += int64(readOps)
	metrics.WriteOps += int64(writeOps)
}

// RecordTierMigration 记录存储层迁移
func (m *Metrics) RecordTierMigration(tierType TierType, filesIn, filesOut, bytesIn, bytesOut int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, ok := m.tierMetrics[tierType]
	if !ok {
		metrics = &TierMetrics{}
		m.tierMetrics[tierType] = metrics
	}

	metrics.FilesMigratedIn += filesIn
	metrics.FilesMigratedOut += filesOut
	metrics.BytesMigratedIn += bytesIn
	metrics.BytesMigratedOut += bytesOut
}

// ==================== 迁移指标 ====================

// RecordMigrationStart 记录迁移开始
func (m *Metrics) RecordMigrationStart() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.migrationMetrics.TotalTasks++
	m.migrationMetrics.RunningTasks++
}

// RecordMigrationComplete 记录迁移完成
func (m *Metrics) RecordMigrationComplete(task *MigrateTask, durationMs int64) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.migrationMetrics.RunningTasks--
	m.migrationMetrics.CompletedTasks++
	m.migrationMetrics.TotalBytesMigrated += task.ProcessedBytes
	m.migrationMetrics.TotalFilesMigrated += task.ProcessedFiles
	m.migrationMetrics.TotalBytesFailed += task.FailedFiles * (task.TotalBytes / max(task.TotalFiles, 1))
	m.migrationMetrics.TotalFilesFailed += task.FailedFiles
	m.migrationMetrics.TotalMigrationTimeMs += durationMs
	m.migrationMetrics.AverageMigrationTimeMs = m.migrationMetrics.TotalMigrationTimeMs / m.migrationMetrics.CompletedTasks
	m.migrationMetrics.LastMigrationTime = time.Now()

	// 按策略统计
	if task.PolicyID != "" {
		policyMetrics, ok := m.migrationMetrics.ByPolicy[task.PolicyID]
		if !ok {
			policyMetrics = &PolicyMigrationMetrics{PolicyID: task.PolicyID}
			m.migrationMetrics.ByPolicy[task.PolicyID] = policyMetrics
		}
		policyMetrics.ExecutionCount++
		policyMetrics.SuccessCount++
		policyMetrics.TotalBytes += task.ProcessedBytes
		policyMetrics.TotalFiles += task.ProcessedFiles
		policyMetrics.LastRunTime = time.Now()
	}
}

// RecordMigrationFailure 记录迁移失败
func (m *Metrics) RecordMigrationFailure(task *MigrateTask) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.migrationMetrics.RunningTasks--
	m.migrationMetrics.FailedTasks++
	m.migrationMetrics.TotalBytesFailed += task.TotalBytes
	m.migrationMetrics.TotalFilesFailed += task.TotalFiles

	// 按策略统计
	if task.PolicyID != "" {
		policyMetrics, ok := m.migrationMetrics.ByPolicy[task.PolicyID]
		if !ok {
			policyMetrics = &PolicyMigrationMetrics{PolicyID: task.PolicyID}
			m.migrationMetrics.ByPolicy[task.PolicyID] = policyMetrics
		}
		policyMetrics.ExecutionCount++
		policyMetrics.FailureCount++
	}
}

// GetMigrationMetrics 获取迁移指标
func (m *Metrics) GetMigrationMetrics() *MigrationMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.migrationMetrics
}

// ==================== 策略指标 ====================

// UpdatePolicyMetrics 更新策略指标
func (m *Metrics) UpdatePolicyMetrics(policyID string, enabled bool, lastRun, nextRun time.Time) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, ok := m.policyMetrics[policyID]
	if !ok {
		metrics = &PolicyMetrics{PolicyID: policyID}
		m.policyMetrics[policyID] = metrics
	}

	metrics.Enabled = enabled
	metrics.LastRunTime = lastRun
	metrics.NextRunTime = nextRun
}

// RecordPolicyExecution 记录策略执行
func (m *Metrics) RecordPolicyExecution(policyID string, success bool) {
	m.mu.Lock()
	defer m.mu.Unlock()

	metrics, ok := m.policyMetrics[policyID]
	if !ok {
		metrics = &PolicyMetrics{PolicyID: policyID}
		m.policyMetrics[policyID] = metrics
	}

	metrics.RunCount++
	if success {
		metrics.SuccessCount++
	} else {
		metrics.FailureCount++
	}
}

// GetPolicyMetrics 获取策略指标
func (m *Metrics) GetPolicyMetrics(policyID string) *PolicyMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if metrics, ok := m.policyMetrics[policyID]; ok {
		return metrics
	}
	return &PolicyMetrics{}
}

// GetAllPolicyMetrics 获取所有策略指标
func (m *Metrics) GetAllPolicyMetrics() map[string]*PolicyMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make(map[string]*PolicyMetrics)
	for k, v := range m.policyMetrics {
		result[k] = v
	}
	return result
}

// ==================== 访问指标 ====================

// UpdateAccessMetrics 更新访问指标
func (m *Metrics) UpdateAccessMetrics(stats *AccessStats) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.accessMetrics.TotalFiles = stats.TotalFiles
	m.accessMetrics.TotalAccesses = stats.TotalAccesses
	m.accessMetrics.TotalReadBytes = stats.TotalReadBytes
	m.accessMetrics.TotalWriteBytes = stats.TotalWriteBytes
	m.accessMetrics.HotFiles = stats.HotFiles
	m.accessMetrics.WarmFiles = stats.WarmFiles
	m.accessMetrics.ColdFiles = stats.ColdFiles
	m.accessMetrics.LastUpdated = time.Now()
}

// GetAccessMetrics 获取访问指标
func (m *Metrics) GetAccessMetrics() *AccessMetrics {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.accessMetrics
}

// ==================== 汇总统计 ====================

// GetSummary 获取汇总统计
func (m *Metrics) GetSummary() *MetricsSummary {
	m.mu.RLock()
	defer m.mu.RUnlock()

	summary := &MetricsSummary{
		Uptime:              time.Since(m.startTime),
		TotalTiers:          len(m.tierMetrics),
		TotalPolicies:       len(m.policyMetrics),
		ActivePolicies:      0,
		RunningMigrations:   m.migrationMetrics.RunningTasks,
		TotalMigrations:     m.migrationMetrics.TotalTasks,
		CompletedMigrations: m.migrationMetrics.CompletedTasks,
		FailedMigrations:    m.migrationMetrics.FailedTasks,
		TotalBytesMigrated:  m.migrationMetrics.TotalBytesMigrated,
		TotalFilesMigrated:  m.migrationMetrics.TotalFilesMigrated,
		TierMetrics:         make(map[TierType]*TierMetrics),
	}

	// 统计活跃策略
	for _, p := range m.policyMetrics {
		if p.Enabled {
			summary.ActivePolicies++
		}
	}

	// 复制存储层指标
	for k, v := range m.tierMetrics {
		summary.TierMetrics[k] = v
	}

	return summary
}

// MetricsSummary 指标汇总
type MetricsSummary struct {
	Uptime              time.Duration             `json:"uptime"`
	TotalTiers          int                       `json:"totalTiers"`
	TotalPolicies       int                       `json:"totalPolicies"`
	ActivePolicies      int                       `json:"activePolicies"`
	RunningMigrations   int64                     `json:"runningMigrations"`
	TotalMigrations     int64                     `json:"totalMigrations"`
	CompletedMigrations int64                     `json:"completedMigrations"`
	FailedMigrations    int64                     `json:"failedMigrations"`
	TotalBytesMigrated  int64                     `json:"totalBytesMigrated"`
	TotalFilesMigrated  int64                     `json:"totalFilesMigrated"`
	TierMetrics         map[TierType]*TierMetrics `json:"tierMetrics"`
}

// ==================== Prometheus 格式导出 ====================

// ExportPrometheus 导出 Prometheus 格式指标
func (m *Metrics) ExportPrometheus() string {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var result string

	// 存储层指标
	for tierType, metrics := range m.tierMetrics {
		result += prometheusGauge("nas_tier_capacity_bytes", float64(metrics.TotalBytes), "tier", string(tierType))
		result += prometheusGauge("nas_tier_used_bytes", float64(metrics.UsedBytes), "tier", string(tierType))
		result += prometheusGauge("nas_tier_available_bytes", float64(metrics.AvailableBytes), "tier", string(tierType))
		result += prometheusGauge("nas_tier_usage_percent", metrics.UsagePercent, "tier", string(tierType))
		result += prometheusGauge("nas_tier_total_files", float64(metrics.TotalFiles), "tier", string(tierType))
		result += prometheusGauge("nas_tier_hot_files", float64(metrics.HotFiles), "tier", string(tierType))
		result += prometheusGauge("nas_tier_warm_files", float64(metrics.WarmFiles), "tier", string(tierType))
		result += prometheusGauge("nas_tier_cold_files", float64(metrics.ColdFiles), "tier", string(tierType))
		result += prometheusCounter("nas_tier_read_bytes_total", float64(metrics.ReadBytes), "tier", string(tierType))
		result += prometheusCounter("nas_tier_write_bytes_total", float64(metrics.WriteBytes), "tier", string(tierType))
		result += prometheusCounter("nas_tier_files_migrated_in_total", float64(metrics.FilesMigratedIn), "tier", string(tierType))
		result += prometheusCounter("nas_tier_files_migrated_out_total", float64(metrics.FilesMigratedOut), "tier", string(tierType))
	}

	// 迁移指标
	result += prometheusGauge("nas_tiering_running_tasks", float64(m.migrationMetrics.RunningTasks))
	result += prometheusCounter("nas_tiering_total_tasks", float64(m.migrationMetrics.TotalTasks))
	result += prometheusCounter("nas_tiering_completed_tasks", float64(m.migrationMetrics.CompletedTasks))
	result += prometheusCounter("nas_tiering_failed_tasks", float64(m.migrationMetrics.FailedTasks))
	result += prometheusCounter("nas_tiering_bytes_migrated_total", float64(m.migrationMetrics.TotalBytesMigrated))
	result += prometheusCounter("nas_tiering_files_migrated_total", float64(m.migrationMetrics.TotalFilesMigrated))
	result += prometheusGauge("nas_tiering_average_migration_time_ms", float64(m.migrationMetrics.AverageMigrationTimeMs))

	// 访问指标
	result += prometheusGauge("nas_tiering_total_tracked_files", float64(m.accessMetrics.TotalFiles))
	result += prometheusCounter("nas_tiering_access_total", float64(m.accessMetrics.TotalAccesses))
	result += prometheusCounter("nas_tiering_read_bytes_total", float64(m.accessMetrics.TotalReadBytes))
	result += prometheusCounter("nas_tiering_write_bytes_total", float64(m.accessMetrics.TotalWriteBytes))
	result += prometheusGauge("nas_tiering_hot_files", float64(m.accessMetrics.HotFiles))
	result += prometheusGauge("nas_tiering_warm_files", float64(m.accessMetrics.WarmFiles))
	result += prometheusGauge("nas_tiering_cold_files", float64(m.accessMetrics.ColdFiles))

	return result
}

func prometheusGauge(name string, value float64, labels ...string) string {
	if len(labels) == 0 {
		return name + " " + floatToString(value) + "\n"
	}
	if len(labels)%2 != 0 {
		return name + " " + floatToString(value) + "\n"
	}

	labelStr := ""
	for i := 0; i < len(labels); i += 2 {
		if i > 0 {
			labelStr += ","
		}
		labelStr += labels[i] + `="` + labels[i+1] + `"`
	}
	return name + "{" + labelStr + "} " + floatToString(value) + "\n"
}

func prometheusCounter(name string, value float64, labels ...string) string {
	return prometheusGauge(name, value, labels...)
}

func floatToString(v float64) string {
	return formatFloat(v)
}

func formatFloat(v float64) string {
	// 简单的浮点数格式化
	if v == float64(int64(v)) {
		return string(append([]byte{}, bytesForInt(int64(v))...))
	}
	s := ""
	for i := 0; i < 6; i++ {
		digit := int64(v * 10)
		s += string(byte('0' + digit%10))
		v = v*10 - float64(digit)
		if v < 0.000001 {
			break
		}
	}
	return s
}

func bytesForInt(n int64) []byte {
	if n == 0 {
		return []byte{'0'}
	}
	var b []byte
	neg := n < 0
	if neg {
		n = -n
	}
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return b
}

func max(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

// MetricsCollector 指标收集器接口
type MetricsCollector interface {
	CollectTierMetrics() map[TierType]*TierMetrics
	CollectMigrationMetrics() *MigrationMetrics
	CollectAccessMetrics() *AccessMetrics
}

// AtomicCounter 原子计数器
type AtomicCounter struct {
	value int64
}

// Increment 增加计数
func (c *AtomicCounter) Increment() {
	atomic.AddInt64(&c.value, 1)
}

// IncrementBy 增加指定值
func (c *AtomicCounter) IncrementBy(n int64) {
	atomic.AddInt64(&c.value, n)
}

// Get 获取当前值
func (c *AtomicCounter) Get() int64 {
	return atomic.LoadInt64(&c.value)
}

// Reset 重置计数器
func (c *AtomicCounter) Reset() {
	atomic.StoreInt64(&c.value, 0)
}
