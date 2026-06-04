// Package ransomshield - 行为分析引擎
// 多维度行为建模、滑动窗口统计、加密行为模式识别、进程画像
package ransomshield

import (
	"log"
	"math"
	"strings"
	"sync"
	"time"
)

// ============================================================
// 行为分析引擎
// ============================================================

// BehaviorAnalyzer 行为分析引擎
type BehaviorAnalyzer struct {
	mu sync.RWMutex

	// profiles 进程行为画像 (pid -> ProcessProfile)
	profiles map[int]*ProcessProfile

	// fileOps 滑动窗口内的文件操作 (path -> []FileOpRecord)
	fileOps map[string][]FileOpRecord

	// patternDetectors 行为模式检测器
	patternDetectors []PatternDetector

	// globalBaseline 全局基线
	globalBaseline *BehaviorBaseline

	// stats 统计
	stats BehaviorStats

	// onAnomaly 异常检测回调
	onAnomaly func(event AnomalyEvent)

	// running 运行状态
	running bool
	stopChan chan struct{}
}

// ProcessProfile 进程行为画像
type ProcessProfile struct {
	PID          int                `json:"pid"`
	Name         string             `json:"name"`
	StartTime    time.Time          `json:"start_time"`
	TotalReads   int64              `json:"total_reads"`
	TotalWrites  int64              `json:"total_writes"`
	TotalDeletes int64              `json:"total_deletes"`
	TotalRenames int64              `json:"total_renames"`
	UniqueFiles  map[string]bool    `json:"-"`
	EntropyAvg   float64            `json:"entropy_avg"`
	EntropyMax   float64            `json:"entropy_max"`
	BytesWritten int64              `json:"bytes_written"`
	Extensions   map[string]int     `json:"extensions"`
	AnomalyScore float64            `json:"anomaly_score"`
	BehaviorTags []string           `json:"behavior_tags"`
	FirstSeen    time.Time          `json:"first_seen"`
	LastSeen     time.Time          `json:"last_seen"`
	WindowStats  *WindowStats       `json:"window_stats,omitempty"`
}

// WindowStats 滑动窗口统计
type WindowStats struct {
	WindowDuration time.Duration `json:"window_duration"`
	WriteRate      float64       `json:"write_rate"`      // 写入/秒
	DeleteRate     float64       `json:"delete_rate"`     // 删除/秒
	RenameRate     float64       `json:"rename_rate"`     // 重命名/秒
	ExtChangeRate  float64       `json:"ext_change_rate"` // 扩展名变更/秒
	EntropySpike   float64       `json:"entropy_spike"`   // 熵值突变
	FileDiversity  float64       `json:"file_diversity"`  // 文件类型多样性
}

// FileOpRecord 文件操作记录
type FileOpRecord struct {
	Path       string    `json:"path"`
	OldPath    string    `json:"old_path,omitempty"`
	OpType     string    `json:"op_type"` // read, write, delete, rename
	Size       int64     `json:"size"`
	Entropy    float64   `json:"entropy"`
	ProcessID  int       `json:"process_id"`
	ProcessName string   `json:"process_name"`
	Timestamp  time.Time `json:"timestamp"`
}

// AnomalyEvent 异常事件
type AnomalyEvent struct {
	ID          string      `json:"id"`
	ProcessID   int         `json:"process_id"`
	ProcessName string      `json:"process_name"`
	AnomalyType string      `json:"anomaly_type"`
	Severity    ThreatLevel `json:"severity"`
	Score       float64     `json:"score"`
	Details     string      `json:"details"`
	Indicators  []string    `json:"indicators"`
	Timestamp   time.Time   `json:"timestamp"`
}

// PatternDetector 行为模式检测器接口
type PatternDetector interface {
	Name() string
	Detect(profile *ProcessProfile, recentOps []FileOpRecord) *AnomalyEvent
}

// BehaviorBaseline 行为基线
type BehaviorBaseline struct {
	AvgWriteRate    float64 `json:"avg_write_rate"`
	AvgDeleteRate   float64 `json:"avg_delete_rate"`
	AvgEntropy      float64 `json:"avg_entropy"`
	StdWriteRate    float64 `json:"std_write_rate"`
	StdDeleteRate   float64 `json:"std_delete_rate"`
	StdEntropy      float64 `json:"std_entropy"`
	SampleCount     int     `json:"sample_count"`
	LastUpdated     time.Time `json:"last_updated"`
}

// BehaviorStats 行为分析统计
type BehaviorStats struct {
	EventsProcessed  int64     `json:"events_processed"`
	AnomaliesFound   int64     `json:"anomalies_found"`
	ProfilesTracked  int       `json:"profiles_tracked"`
	BaselineUpdates  int       `json:"baseline_updates"`
	LastAnalysisTime time.Time `json:"last_analysis_time"`
}

// ============================================================
// 构造与生命周期
// ============================================================

// NewBehaviorAnalyzer 创建行为分析引擎
func NewBehaviorAnalyzer() *BehaviorAnalyzer {
	ba := &BehaviorAnalyzer{
		profiles:     make(map[int]*ProcessProfile),
		fileOps:      make(map[string][]FileOpRecord),
		globalBaseline: &BehaviorBaseline{
			AvgWriteRate:  10.0,  // 初始基线：每分钟10次写入
			AvgDeleteRate: 2.0,
			AvgEntropy:    5.0,
			StdWriteRate:  5.0,
			StdDeleteRate:  2.0,
			StdEntropy:     1.5,
			SampleCount:    0,
			LastUpdated:    time.Now(),
		},
		stopChan: make(chan struct{}),
	}

	// 注册内置检测器
	ba.patternDetectors = []PatternDetector{
		&EncryptionBurstDetector{},
		&MassRenameDetector{},
		&EntropySpikeDetector{},
		&ShadowCopyDeletionDetector{},
		&ExtensionStormDetector{},
	}

	return ba
}

// SetAnomalyCallback 设置异常回调
func (ba *BehaviorAnalyzer) SetAnomalyCallback(fn func(event AnomalyEvent)) {
	ba.mu.Lock()
	ba.onAnomaly = fn
	ba.mu.Unlock()
}

// Start 启动行为分析引擎
func (ba *BehaviorAnalyzer) Start() {
	ba.mu.Lock()
	if ba.running {
		ba.mu.Unlock()
		return
	}
	ba.running = true
	ba.mu.Unlock()

	go ba.analysisLoop()
	go ba.cleanupLoop()

	log.Println("[BehaviorAnalyzer] 行为分析引擎已启动")
}

// Stop 停止行为分析引擎
func (ba *BehaviorAnalyzer) Stop() {
	ba.mu.Lock()
	defer ba.mu.Unlock()
	if !ba.running {
		return
	}
	close(ba.stopChan)
	ba.running = false
	log.Println("[BehaviorAnalyzer] 行为分析引擎已停止")
}

// ============================================================
// 事件摄入
// ============================================================

// RecordFileOp 记录文件操作
func (ba *BehaviorAnalyzer) RecordFileOp(op FileOpRecord) {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	// 更新进程画像
	ba.updateProfile(op)

	// 添加到滑动窗口
	key := op.Path
	ba.fileOps[key] = append(ba.fileOps[key], op)

	// 保持窗口在合理大小
	if len(ba.fileOps[key]) > 1000 {
		ba.fileOps[key] = ba.fileOps[key][1:]
	}

	ba.stats.EventsProcessed++
}

// updateProfile 更新进程画像
func (ba *BehaviorAnalyzer) updateProfile(op FileOpRecord) {
	profile, exists := ba.profiles[op.ProcessID]
	if !exists {
		profile = &ProcessProfile{
			PID:         op.ProcessID,
			Name:        op.ProcessName,
			StartTime:   op.Timestamp,
			UniqueFiles: make(map[string]bool),
			Extensions:  make(map[string]int),
			FirstSeen:   op.Timestamp,
			LastSeen:    op.Timestamp,
		}
		ba.profiles[op.ProcessID] = profile
		ba.stats.ProfilesTracked++
	}

	profile.LastSeen = op.Timestamp
	profile.UniqueFiles[op.Path] = true

	// 统计扩展名
	if ext := extractExt(op.Path); ext != "" {
		profile.Extensions[ext]++
	}

	switch op.OpType {
	case "write":
		profile.TotalWrites++
		profile.BytesWritten += op.Size
		if op.Entropy > profile.EntropyMax {
			profile.EntropyMax = op.Entropy
		}
		// 滚动平均熵
		if profile.TotalWrites == 1 {
			profile.EntropyAvg = op.Entropy
		} else {
			profile.EntropyAvg = (profile.EntropyAvg*float64(profile.TotalWrites-1) + op.Entropy) / float64(profile.TotalWrites)
		}
	case "read":
		profile.TotalReads++
	case "delete":
		profile.TotalDeletes++
	case "rename":
		profile.TotalRenames++
	}
}

// ============================================================
// 分析循环
// ============================================================

// analysisLoop 定时分析
func (ba *BehaviorAnalyzer) analysisLoop() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ba.stopChan:
			return
		case <-ticker.C:
			ba.runAnalysis()
		}
	}
}

// runAnalysis 执行行为分析
func (ba *BehaviorAnalyzer) runAnalysis() {
	ba.mu.Lock()
	profiles := make(map[int]*ProcessProfile)
	for k, v := range ba.profiles {
		profiles[k] = v
	}

	// 计算窗口统计
	for _, profile := range profiles {
		ba.computeWindowStats(profile)
	}
	ba.mu.Unlock()

	// 对每个进程运行检测器
	for _, profile := range profiles {
		recentOps := ba.getRecentOps(profile.PID, 5*time.Minute)

		for _, detector := range ba.patternDetectors {
			anomaly := detector.Detect(profile, recentOps)
			if anomaly != nil {
				ba.handleAnomaly(anomaly)
			}
		}
	}

	// 更新基线
	ba.updateBaseline(profiles)

	ba.mu.Lock()
	ba.stats.LastAnalysisTime = time.Now()
	ba.mu.Unlock()
}

// computeWindowStats 计算滑动窗口统计
func (ba *BehaviorAnalyzer) computeWindowStats(profile *ProcessProfile) {
	window := 60 * time.Second // 1分钟窗口
	now := time.Now()

	var writes, deletes, renames, extChanges int
	var entropySum float64
	var entropyCount int

	for _, ops := range ba.fileOps {
		for _, op := range ops {
			if op.ProcessID != profile.PID {
				continue
			}
			if now.Sub(op.Timestamp) > window {
				continue
			}

			switch op.OpType {
			case "write":
				writes++
				if op.Entropy > 0 {
					entropySum += op.Entropy
					entropyCount++
				}
			case "delete":
				deletes++
			case "rename":
				renames++
				if extractExt(op.OldPath) != extractExt(op.Path) {
					extChanges++
				}
			}
		}
	}

	windowSec := window.Seconds()
	profile.WindowStats = &WindowStats{
		WindowDuration: window,
		WriteRate:      float64(writes) / windowSec,
		DeleteRate:     float64(deletes) / windowSec,
		RenameRate:     float64(renames) / windowSec,
		ExtChangeRate:  float64(extChanges) / windowSec,
	}

	if entropyCount > 0 {
		avgEntropy := entropySum / float64(entropyCount)
		profile.WindowStats.EntropySpike = math.Abs(avgEntropy - ba.globalBaseline.AvgEntropy)
	}

	// 计算异常分数
	profile.AnomalyScore = ba.calculateAnomalyScore(profile)
}

// calculateAnomalyScore 计算进程异常分数 (0-100)
func (ba *BehaviorAnalyzer) calculateAnomalyScore(profile *ProcessProfile) float64 {
	score := 0.0
	ws := profile.WindowStats
	if ws == nil {
		return 0
	}
	baseline := ba.globalBaseline

	// 写入率异常
	if baseline.StdWriteRate > 0 {
		zScore := (ws.WriteRate - baseline.AvgWriteRate) / baseline.StdWriteRate
		if zScore > 0 {
			score += math.Min(zScore*15, 30)
		}
	}

	// 删除率异常
	if baseline.StdDeleteRate > 0 {
		zScore := (ws.DeleteRate - baseline.AvgDeleteRate) / baseline.StdDeleteRate
		if zScore > 0 {
			score += math.Min(zScore*15, 25)
		}
	}

	// 重命名率
	if ws.RenameRate > 0.5 {
		score += math.Min(ws.RenameRate*20, 25)
	}

	// 扩展名变更率
	if ws.ExtChangeRate > 0.1 {
		score += math.Min(ws.ExtChangeRate*50, 30)
	}

	// 熵值突变
	if ws.EntropySpike > 2.0 {
		score += math.Min(ws.EntropySpike*5, 20)
	}

	// 高熵写入
	if profile.EntropyAvg > 7.5 {
		score += 15
	}

	return math.Min(score, 100)
}

// handleAnomaly 处理异常事件
func (ba *BehaviorAnalyzer) handleAnomaly(event *AnomalyEvent) {
	ba.mu.Lock()
	ba.stats.AnomaliesFound++
	ba.mu.Unlock()

	log.Printf("[BehaviorAnalyzer] 检测到异常: 进程=%s(%d), 类型=%s, 评分=%.1f",
		event.ProcessName, event.ProcessID, event.AnomalyType, event.Score)

	if ba.onAnomaly != nil {
		go ba.onAnomaly(*event)
	}
}

// getRecentOps 获取进程近期操作
func (ba *BehaviorAnalyzer) getRecentOps(pid int, window time.Duration) []FileOpRecord {
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	var ops []FileOpRecord
	cutoff := time.Now().Add(-window)

	for _, opList := range ba.fileOps {
		for _, op := range opList {
			if op.ProcessID == pid && op.Timestamp.After(cutoff) {
				ops = append(ops, op)
			}
		}
	}
	return ops
}

// ============================================================
// 基线更新
// ============================================================

// updateBaseline 更新全局基线
func (ba *BehaviorAnalyzer) updateBaseline(profiles map[int]*ProcessProfile) {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	if len(profiles) == 0 {
		return
	}

	var totalWriteRate, totalDeleteRate, totalEntropy float64
	count := 0

	for _, p := range profiles {
		if p.WindowStats == nil {
			continue
		}
		totalWriteRate += p.WindowStats.WriteRate
		totalDeleteRate += p.WindowStats.DeleteRate
		totalEntropy += p.EntropyAvg
		count++
	}

	if count == 0 {
		return
	}

	n := float64(count)
	baseline := ba.globalBaseline
	oldN := float64(baseline.SampleCount)
	newN := oldN + n

	// 增量更新均值
	baseline.AvgWriteRate = (baseline.AvgWriteRate*oldN + totalWriteRate) / newN
	baseline.AvgDeleteRate = (baseline.AvgDeleteRate*oldN + totalDeleteRate) / newN
	baseline.AvgEntropy = (baseline.AvgEntropy*oldN + totalEntropy) / newN

	baseline.SampleCount += count
	baseline.LastUpdated = time.Now()
	ba.stats.BaselineUpdates++
}

// ============================================================
// 清理循环
// ============================================================

// cleanupLoop 清理过期数据
func (ba *BehaviorAnalyzer) cleanupLoop() {
	ticker := time.NewTicker(10 * time.Minute)
	defer ticker.Stop()

	for {
		select {
		case <-ba.stopChan:
			return
		case <-ticker.C:
			ba.cleanup()
		}
	}
}

// cleanup 清理过期的进程画像和操作记录
func (ba *BehaviorAnalyzer) cleanup() {
	ba.mu.Lock()
	defer ba.mu.Unlock()

	cutoff := time.Now().Add(-30 * time.Minute)

	// 清理不活跃的进程画像
	for pid, profile := range ba.profiles {
		if profile.LastSeen.Before(cutoff) {
			delete(ba.profiles, pid)
			ba.stats.ProfilesTracked--
		}
	}

	// 清理旧的文件操作记录
	for path, ops := range ba.fileOps {
		var recent []FileOpRecord
		for _, op := range ops {
			if op.Timestamp.After(cutoff) {
				recent = append(recent, op)
			}
		}
		if len(recent) == 0 {
			delete(ba.fileOps, path)
		} else {
			ba.fileOps[path] = recent
		}
	}
}

// ============================================================
// 查询接口
// ============================================================

// GetProfile 获取进程画像
func (ba *BehaviorAnalyzer) GetProfile(pid int) (*ProcessProfile, bool) {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	p, ok := ba.profiles[pid]
	if !ok {
		return nil, false
	}
	result := *p
	return &result, true
}

// GetAnomalousProcesses 获取异常进程列表
func (ba *BehaviorAnalyzer) GetAnomalousProcesses(threshold float64) []ProcessProfile {
	ba.mu.RLock()
	defer ba.mu.RUnlock()

	var result []ProcessProfile
	for _, p := range ba.profiles {
		if p.AnomalyScore >= threshold {
			result = append(result, *p)
		}
	}
	return result
}

// GetStats 获取统计信息
func (ba *BehaviorAnalyzer) GetStats() BehaviorStats {
	ba.mu.RLock()
	defer ba.mu.RUnlock()
	stats := ba.stats
	stats.ProfilesTracked = len(ba.profiles)
	return stats
}

// ============================================================
// 内置模式检测器
// ============================================================

// EncryptionBurstDetector 加密突发检测器
type EncryptionBurstDetector struct{}

func (d *EncryptionBurstDetector) Name() string { return "encryption-burst" }

func (d *EncryptionBurstDetector) Detect(profile *ProcessProfile, recentOps []FileOpRecord) *AnomalyEvent {
	if profile.WindowStats == nil {
		return nil
	}

	ws := profile.WindowStats
	// 高写入率 + 高熵 = 加密行为
	if ws.WriteRate > 5.0 && profile.EntropyAvg > 7.0 {
		return &AnomalyEvent{
			ProcessID:   profile.PID,
			ProcessName: profile.Name,
			AnomalyType: "encryption-burst",
			Severity:    ThreatLevelCritical,
			Score:       profile.AnomalyScore,
			Details:     "检测到加密突发行为：高写入率 + 高熵值",
			Indicators: []string{
				"write_rate_high",
				"entropy_high",
			},
			Timestamp: time.Now(),
		}
	}
	return nil
}

// MassRenameDetector 批量重命名检测器
type MassRenameDetector struct{}

func (d *MassRenameDetector) Name() string { return "mass-rename" }

func (d *MassRenameDetector) Detect(profile *ProcessProfile, recentOps []FileOpRecord) *AnomalyEvent {
	if profile.WindowStats == nil {
		return nil
	}

	// 大量重命名操作
	if profile.WindowStats.RenameRate > 2.0 {
		return &AnomalyEvent{
			ProcessID:   profile.PID,
			ProcessName: profile.Name,
			AnomalyType: "mass-rename",
			Severity:    ThreatLevelHigh,
			Score:       profile.AnomalyScore,
			Details:     "检测到批量重命名行为",
			Indicators:  []string{"rename_rate_high"},
			Timestamp:   time.Now(),
		}
	}
	return nil
}

// EntropySpikeDetector 熵值突变检测器
type EntropySpikeDetector struct{}

func (d *EntropySpikeDetector) Name() string { return "entropy-spike" }

func (d *EntropySpikeDetector) Detect(profile *ProcessProfile, recentOps []FileOpRecord) *AnomalyEvent {
	if profile.WindowStats == nil {
		return nil
	}

	if profile.WindowStats.EntropySpike > 3.0 {
		return &AnomalyEvent{
			ProcessID:   profile.PID,
			ProcessName: profile.Name,
			AnomalyType: "entropy-spike",
			Severity:    ThreatLevelHigh,
			Score:       profile.AnomalyScore,
			Details:     "检测到熵值突变，疑似加密行为",
			Indicators:  []string{"entropy_spike"},
			Timestamp:   time.Now(),
		}
	}
	return nil
}

// ShadowCopyDeletionDetector 卷影副本删除检测器
type ShadowCopyDeletionDetector struct{}

func (d *ShadowCopyDeletionDetector) Name() string { return "shadow-copy-deletion" }

func (d *ShadowCopyDeletionDetector) Detect(profile *ProcessProfile, recentOps []FileOpRecord) *AnomalyEvent {
	suspiciousCmds := []string{
		"vssadmin delete shadows",
		"wmic shadowcopy delete",
		"bcdedit /set {default} recoveryenabled no",
	}

	// 检查进程名或近期操作是否包含可疑命令
	procLower := strings.ToLower(profile.Name)
	for _, cmd := range suspiciousCmds {
		if strings.Contains(procLower, cmd) {
			return &AnomalyEvent{
				ProcessID:   profile.PID,
				ProcessName: profile.Name,
				AnomalyType: "shadow-copy-deletion",
				Severity:    ThreatLevelCritical,
				Score:       95,
				Details:     "检测到卷影副本删除行为，典型勒索软件特征",
				Indicators:  []string{"shadow_copy_deletion", "recovery_disable"},
				Timestamp:   time.Now(),
			}
		}
	}
	return nil
}

// ExtensionStormDetector 扩展名风暴检测器
type ExtensionStormDetector struct{}

func (d *ExtensionStormDetector) Name() string { return "extension-storm" }

func (d *ExtensionStormDetector) Detect(profile *ProcessProfile, recentOps []FileOpRecord) *AnomalyEvent {
	if profile.WindowStats == nil {
		return nil
	}

	if profile.WindowStats.ExtChangeRate > 1.0 {
		// 统计新扩展名
		knownRansomExts := []string{
			".encrypted", ".locked", ".crypto", ".crypt", ".locky",
			".cerber", ".wncry", ".wncryt", ".ryk", ".ryuk",
			".maze", ".revil", ".sodinokibi", ".conti",
		}

		for _, op := range recentOps {
			if op.OpType == "rename" {
				newExt := strings.ToLower(extractExt(op.Path))
				for _, ransomExt := range knownRansomExts {
					if newExt == ransomExt {
						return &AnomalyEvent{
							ProcessID:   profile.PID,
							ProcessName: profile.Name,
							AnomalyType: "extension-storm",
							Severity:    ThreatLevelCritical,
							Score:       98,
							Details:     "检测到勒索软件扩展名变更: " + newExt,
							Indicators:  []string{"ransom_extension", "mass_rename"},
							Timestamp:   time.Now(),
						}
					}
				}
			}
		}

		// 即使不是已知勒索扩展名，大量扩展名变更也是异常
		if profile.WindowStats.ExtChangeRate > 3.0 {
			return &AnomalyEvent{
				ProcessID:   profile.PID,
				ProcessName: profile.Name,
				AnomalyType: "extension-storm",
				Severity:    ThreatLevelHigh,
				Score:       profile.AnomalyScore,
				Details:     "检测到大量扩展名变更",
				Indicators:  []string{"mass_ext_change"},
				Timestamp:   time.Now(),
			}
		}
	}
	return nil
}

// ============================================================
// 辅助函数
// ============================================================

// extractExt 提取文件扩展名
func extractExt(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return strings.ToLower(path[i:])
		}
		if path[i] == '/' || path[i] == '\\' {
			break
		}
	}
	return ""
}
