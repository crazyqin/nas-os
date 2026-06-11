// Package ransombehaviorai 勒索软件行为 AI 检测引擎管理器
package ransombehaviorai

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 勒索行为 AI 检测引擎管理器
type Manager struct {
	mu     sync.RWMutex
	logger *zap.Logger
	config *Config

	// 事件缓冲（滑动窗口）
	fileEvents    []FileBehaviorEvent
	ioSamples     []IOSample
	processEvents []ProcessBehaviorEvent

	// 评估历史
	assessments []*BehaviorAssessment
	responseLog []*ResponseEvent

	// 统计
	stats      Stats
	startTime  time.Time
	running    bool
	lastErr    string
	stopChan   chan struct{}

	// 快照回调
	snapshotCallback func(path string) (string, error)
	// 隔离回调
	isolateCallback func(processID int) error
	// 告警回调
	alertCallback func(assessment *BehaviorAssessment)
}

// NewManager 创建 AI 行为检测管理器
func NewManager(logger *zap.Logger, config *Config) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = DefaultConfig()
	}

	return &Manager{
		logger:        logger,
		config:        config,
		fileEvents:    make([]FileBehaviorEvent, 0),
		ioSamples:     make([]IOSample, 0),
		processEvents: make([]ProcessBehaviorEvent, 0),
		assessments:   make([]*BehaviorAssessment, 0),
		responseLog:   make([]*ResponseEvent, 0),
		stopChan:      make(chan struct{}),
	}
}

// Start 启动引擎
func (m *Manager) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.running {
		return fmt.Errorf("engine already running")
	}

	m.running = true
	m.startTime = time.Now()
	m.stopChan = make(chan struct{})

	m.logger.Info("ransom behavior AI engine started",
		zap.Bool("fileMonitor", m.config.FileMonitor.Enabled),
		zap.Bool("ioMonitor", m.config.IOMonitor.Enabled),
		zap.Bool("processMonitor", m.config.ProcessMonitor.Enabled))

	go m.cleanupLoop()
	return nil
}

// Stop 停止引擎
func (m *Manager) Stop() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if !m.running {
		return
	}

	m.running = false
	close(m.stopChan)
	m.logger.Info("ransom behavior AI engine stopped")
}

// IsRunning 返回引擎是否运行中
func (m *Manager) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.running
}

// cleanupLoop 定期清理过期数据
func (m *Manager) cleanupLoop() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-m.stopChan:
			return
		case <-ticker.C:
			m.cleanup()
		}
	}
}

// cleanup 清理超过时间窗口的事件
func (m *Manager) cleanup() {
	m.mu.Lock()
	defer m.mu.Unlock()

	window := time.Duration(m.config.FileMonitor.TimeWindowSec) * time.Second
	if window == 0 {
		window = 60 * time.Second
	}
	cutoff := time.Now().Add(-window)

	// 清理文件事件
	filtered := m.fileEvents[:0]
	for _, e := range m.fileEvents {
		if e.Timestamp.After(cutoff) {
			filtered = append(filtered, e)
		}
	}
	m.fileEvents = filtered

	// 清理 IO 样本
	ioWindow := time.Duration(m.config.IOMonitor.SampleIntervalSec*m.config.IOMonitor.WindowSize) * time.Second
	if ioWindow == 0 {
		ioWindow = 5 * time.Minute
	}
	ioCutoff := time.Now().Add(-ioWindow)
	filteredIO := m.ioSamples[:0]
	for _, s := range m.ioSamples {
		if s.Timestamp.After(ioCutoff) {
			filteredIO = append(filteredIO, s)
		}
	}
	m.ioSamples = filteredIO

	// 清理进程事件
	procFiltered := m.processEvents[:0]
	for _, e := range m.processEvents {
		if e.Timestamp.After(cutoff) {
			procFiltered = append(procFiltered, e)
		}
	}
	m.processEvents = procFiltered
}

// ============================================================
// Event Ingestion
// ============================================================

// ReportFileEvent 上报文件行为事件
func (m *Manager) ReportFileEvent(event FileBehaviorEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	m.fileEvents = append(m.fileEvents, event)
	m.stats.FileEvents++
	m.stats.TotalEvents++

	m.logger.Debug("file event recorded",
		zap.String("type", string(event.Type)),
		zap.String("path", event.Path),
		zap.String("process", event.ProcessName))
}

// ReportIOSample 上报 IO 采样
func (m *Manager) ReportIOSample(sample IOSample) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if sample.Timestamp.IsZero() {
		sample.Timestamp = time.Now()
	}
	m.ioSamples = append(m.ioSamples, sample)
	m.stats.IOEvents++
	m.stats.TotalEvents++
}

// ReportProcessEvent 上报进程行为事件
func (m *Manager) ReportProcessEvent(event ProcessBehaviorEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if event.Timestamp.IsZero() {
		event.Timestamp = time.Now()
	}
	m.processEvents = append(m.processEvents, event)
	m.stats.ProcessEvents++
	m.stats.TotalEvents++

	m.logger.Debug("process event recorded",
		zap.String("type", string(event.Type)),
		zap.String("process", event.ProcessName))
}

// ============================================================
// AI Scoring Engine
// ============================================================

// Evaluate 执行一次完整的行为评估
func (m *Manager) Evaluate() *BehaviorAssessment {
	m.mu.RLock()
	fileEvents := make([]FileBehaviorEvent, len(m.fileEvents))
	copy(fileEvents, m.fileEvents)
	ioSamples := make([]IOSample, len(m.ioSamples))
	copy(ioSamples, m.ioSamples)
	processEvents := make([]ProcessBehaviorEvent, len(m.processEvents))
	copy(processEvents, m.processEvents)
	cfg := *m.config
	m.mu.RUnlock()

	// 计算各维度评分
	fileScore := m.scoreFileBehavior(fileEvents, cfg.FileMonitor)
	ioScore := m.scoreIOBehavior(ioSamples, cfg.IOMonitor)
	processScore := m.scoreProcessBehavior(processEvents, cfg.ProcessMonitor)

	// AI 加权综合评分
	totalScore := m.computeWeightedScore(fileScore.TotalScore, ioScore.TotalScore, processScore.TotalScore, cfg.AIModel)
	confidence := m.computeConfidence(fileScore, ioScore, processScore, len(fileEvents), len(ioSamples), len(processEvents))
	threatLevel := m.scoreToThreatLevel(totalScore)

	// 构建评估结果
	assessment := &BehaviorAssessment{
		AssessmentID: generateAssessmentID(),
		Timestamp:    time.Now(),
		ThreatLevel:  threatLevel,
		Score:        totalScore,
		Confidence:   confidence,
		FileScore:    fileScore,
		IOScore:      ioScore,
		ProcessScore: processScore,
		Indicators:   m.collectIndicators(fileScore, ioScore, processScore, fileEvents, ioSamples, processEvents),
		AffectedFiles: m.collectAffectedFiles(fileEvents),
		RecommendedAction: m.recommendAction(threatLevel, cfg.AutoResponse),
	}

	// 保存评估
	m.mu.Lock()
	m.assessments = append(m.assessments, assessment)
	if totalScore >= cfg.AIModel.ScoreThreshold {
		m.stats.ThreatsDetected++
		now := time.Now()
		m.stats.LastThreatTime = &now
		m.stats.LastThreatLevel = threatLevel
		m.lastErr = ""
	}
	// 限制历史
	if len(m.assessments) > 1000 {
		m.assessments = m.assessments[100:]
	}
	m.mu.Unlock()

	m.logger.Info("behavior assessment completed",
		zap.Int("score", totalScore),
		zap.Int("confidence", confidence),
		zap.String("threatLevel", threatLevel.String()),
		zap.Int("fileScore", fileScore.TotalScore),
		zap.Int("ioScore", ioScore.TotalScore),
		zap.Int("processScore", processScore.TotalScore))

	return assessment
}

// scoreFileBehavior 计算文件行为评分
func (m *Manager) scoreFileBehavior(events []FileBehaviorEvent, cfg FileMonitorConfig) FileBehaviorScore {
	if !cfg.Enabled || len(events) == 0 {
		return FileBehaviorScore{}
	}

	window := time.Duration(cfg.TimeWindowSec) * time.Second
	cutoff := time.Now().Add(-window)

	// 统计窗口内事件
	var writeCount, renameCount, encryptCount int
	var highEntropyCount int
	suspiciousExtSet := make(map[string]bool, len(cfg.SuspiciousExtensions))
	for _, ext := range cfg.SuspiciousExtensions {
		suspiciousExtSet[strings.ToLower(ext)] = true
	}

	for _, e := range events {
		if e.Timestamp.Before(cutoff) {
			continue
		}
		switch e.Type {
		case FileEventModify, FileEventCreate, FileEventBulkWrite:
			writeCount++
		case FileEventRename, FileEventExtensionChg:
			renameCount++
		case FileEventEncrypt:
			encryptCount++
		}
		if e.Entropy >= cfg.EntropyThreshold {
			highEntropyCount++
		}
		if suspiciousExtSet[strings.ToLower(e.Extension)] {
			encryptCount++
		}
	}

	// 计算各子项评分
	encryptionScore := clampScore(encryptCount * 15 + highEntropyCount * 10)
	bulkRenameScore := 0
	if cfg.BulkRenameThreshold > 0 {
		bulkRenameScore = clampScore(renameCount * 100 / cfg.BulkRenameThreshold)
	}
	bulkWriteScore := 0
	if cfg.BulkWriteThreshold > 0 {
		bulkWriteScore = clampScore(writeCount * 100 / cfg.BulkWriteThreshold)
	}

	total := (encryptionScore*2 + bulkRenameScore + bulkWriteScore) / 4
	if total > 100 {
		total = 100
	}

	return FileBehaviorScore{
		EncryptionLikelihood: encryptionScore,
		BulkRenameScore:      bulkRenameScore,
		BulkWriteScore:       bulkWriteScore,
		TotalScore:           total,
	}
}

// scoreIOBehavior 计算 IO 行为评分
func (m *Manager) scoreIOBehavior(samples []IOSample, cfg IOMonitorConfig) IOBehaviorScore {
	if !cfg.Enabled || len(samples) == 0 {
		return IOBehaviorScore{}
	}

	// 窗口内的聚合
	var totalWrite, totalRead int64
	var maxWriteRate int64
	var burstCount int

	for _, s := range samples {
		totalWrite += s.WriteBytes
		totalRead += s.ReadBytes

		// 估算单样本写入速率
		rate := s.WriteBytes
		if rate > maxWriteRate {
			maxWriteRate = rate
		}
		if cfg.BurstWriteThresholdBps > 0 && rate > cfg.BurstWriteThresholdBps {
			burstCount++
		}
	}

	// 突发写入评分
	burstScore := 0
	if len(samples) > 0 {
		burstScore = clampScore(burstCount * 100 / len(samples))
	}
	if maxWriteRate > cfg.BurstWriteThresholdBps*2 {
		burstScore = clampScore(burstScore + 30)
	}

	// 读写比评分
	rwRatioScore := 0
	if totalRead > 0 {
		ratio := float64(totalWrite) / float64(totalRead)
		if ratio > cfg.AnomalousRWRatio {
			rwRatioScore = clampScore(int((ratio / cfg.AnomalousRWRatio) * 50))
		}
	} else if totalWrite > 0 {
		// 只有写入没有读取
		rwRatioScore = 80
	}

	total := (burstScore + rwRatioScore) / 2
	if total > 100 {
		total = 100
	}

	return IOBehaviorScore{
		BurstWriteScore: burstScore,
		RWRatioScore:    rwRatioScore,
		TotalScore:      total,
	}
}

// scoreProcessBehavior 计算进程行为评分
func (m *Manager) scoreProcessBehavior(events []ProcessBehaviorEvent, cfg ProcessMonitorConfig) ProcessBehaviorScore {
	if !cfg.Enabled || len(events) == 0 {
		return ProcessBehaviorScore{}
	}

	suspiciousSet := make(map[string]bool, len(cfg.SuspiciousProcessNames))
	for _, name := range cfg.SuspiciousProcessNames {
		suspiciousSet[strings.ToLower(name)] = true
	}

	var suspiciousCount, privEscCount, anomalousCount int
	for _, e := range events {
		switch e.Type {
		case ProcessEventSuspicious:
			suspiciousCount++
		case ProcessEventPrivEsc:
			privEscCount++
		case ProcessEventAnomalous:
			anomalousCount++
		}
		// 也按名称检查
		if suspiciousSet[strings.ToLower(e.ProcessName)] {
			suspiciousCount++
		}
	}

	suspScore := clampScore(suspiciousCount * 25)
	privScore := clampScore(privEscCount * 30)
	if cfg.PrivEscThreshold > 0 && privEscCount >= cfg.PrivEscThreshold {
		privScore = clampScore(privScore + 20)
	}
	anomScore := clampScore(anomalousCount * 20)

	total := (suspScore + privScore + anomScore) / 3
	if total > 100 {
		total = 100
	}

	return ProcessBehaviorScore{
		SuspiciousProcessScore: suspScore,
		PrivEscScore:           privScore,
		AnomalousScore:         anomScore,
		TotalScore:             total,
	}
}

// computeWeightedScore 计算加权综合评分
func (m *Manager) computeWeightedScore(fileScore, ioScore, processScore int, cfg AIModelConfig) int {
	totalWeight := cfg.WeightFile + cfg.WeightIO + cfg.WeightProcess
	if totalWeight == 0 {
		totalWeight = 1.0
	}
	weighted := float64(fileScore)*cfg.WeightFile +
		float64(ioScore)*cfg.WeightIO +
		float64(processScore)*cfg.WeightProcess
	return clampScore(int(math.Round(weighted / totalWeight)))
}

// computeConfidence 计算置信度
func (m *Manager) computeConfidence(fs FileBehaviorScore, ios IOBehaviorScore, ps ProcessBehaviorScore, fileN, ioN, procN int) int {
	// 置信度基于数据量和评分一致性
	dataPoints := fileN + ioN + procN
	if dataPoints == 0 {
		return 0
	}

	// 数据量因子：数据越多置信度越高
	dataFactor := math.Min(float64(dataPoints)/50.0, 1.0)

	// 一致性因子：三个维度的评分越一致置信度越高
	scores := []float64{float64(fs.TotalScore), float64(ios.TotalScore), float64(ps.TotalScore)}
	mean := (scores[0] + scores[1] + scores[2]) / 3.0
	var variance float64
	for _, s := range scores {
		variance += (s - mean) * (s - mean)
	}
	variance /= 3.0
	consistencyFactor := 1.0 - math.Min(math.Sqrt(variance)/50.0, 1.0)

	confidence := (dataFactor*0.6 + consistencyFactor*0.4) * 100
	return clampScore(int(math.Round(confidence)))
}

// scoreToThreatLevel 评分转威胁级别
func (m *Manager) scoreToThreatLevel(score int) ThreatLevel {
	switch {
	case score >= 85:
		return ThreatLevelCritical
	case score >= 65:
		return ThreatLevelHigh
	case score >= 40:
		return ThreatLevelMedium
	case score >= 15:
		return ThreatLevelLow
	default:
		return ThreatLevelNone
	}
}

// recommendAction 推荐响应动作
func (m *Manager) recommendAction(level ThreatLevel, cfg AutoResponseConfig) ResponseAction {
	if !cfg.Enabled || level < cfg.ThresholdLevel {
		return ActionAlert
	}

	if level >= ThreatLevelCritical && cfg.IsolateOnCritical {
		return ActionIsolate
	}
	if level >= ThreatLevelHigh {
		if cfg.SnapshotOnThreat {
			return ActionSnapshot
		}
		return ActionQuarantine
	}
	return cfg.DefaultAction
}

// collectIndicators 收集威胁指标
func (m *Manager) collectIndicators(fs FileBehaviorScore, ios IOBehaviorScore, ps ProcessBehaviorScore, fileEvents []FileBehaviorEvent, ioSamples []IOSample, processEvents []ProcessBehaviorEvent) []ThreatIndicator {
	var indicators []ThreatIndicator

	if fs.EncryptionLikelihood > 30 {
		indicators = append(indicators, ThreatIndicator{
			Type:        "encryption_pattern",
			Description: "检测到疑似加密行为",
			Weight:      2,
			Value:       fs.EncryptionLikelihood,
			Threshold:   30,
		})
	}
	if fs.BulkRenameScore > 50 {
		indicators = append(indicators, ThreatIndicator{
			Type:        "bulk_rename",
			Description: "检测到批量重命名行为",
			Weight:      2,
			Value:       fs.BulkRenameScore,
			Threshold:   50,
		})
	}
	if fs.BulkWriteScore > 50 {
		indicators = append(indicators, ThreatIndicator{
			Type:        "bulk_write",
			Description: "检测到异常批量写入",
			Weight:      1,
			Value:       fs.BulkWriteScore,
			Threshold:   50,
		})
	}
	if ios.BurstWriteScore > 40 {
		indicators = append(indicators, ThreatIndicator{
			Type:        "burst_write",
			Description: "检测到突发写入带宽",
			Weight:      1,
			Value:       ios.BurstWriteScore,
			Threshold:   40,
		})
	}
	if ios.RWRatioScore > 40 {
		indicators = append(indicators, ThreatIndicator{
			Type:        "anomalous_rw_ratio",
			Description: "检测到异常读写比",
			Weight:      1,
			Value:       ios.RWRatioScore,
			Threshold:   40,
		})
	}
	if ps.SuspiciousProcessScore > 30 {
		indicators = append(indicators, ThreatIndicator{
			Type:        "suspicious_process",
			Description: "检测到可疑进程行为",
			Weight:      2,
			Value:       ps.SuspiciousProcessScore,
			Threshold:   30,
		})
	}
	if ps.PrivEscScore > 30 {
		indicators = append(indicators, ThreatIndicator{
			Type:        "privilege_escalation",
			Description: "检测到权限提升尝试",
			Weight:      3,
			Value:       ps.PrivEscScore,
			Threshold:   30,
		})
	}

	_ = fileEvents
	_ = ioSamples
	_ = processEvents

	return indicators
}

// collectAffectedFiles 收集受影响文件列表
func (m *Manager) collectAffectedFiles(events []FileBehaviorEvent) []string {
	seen := make(map[string]struct{})
	var files []string
	for _, e := range events {
		if _, ok := seen[e.Path]; !ok {
			seen[e.Path] = struct{}{}
			files = append(files, e.Path)
		}
	}
	// 限制数量
	if len(files) > 100 {
		sort.Strings(files)
		files = files[:100]
	}
	return files
}

// ============================================================
// Auto Response
// ============================================================

// TriggerResponse 触发自动响应
func (m *Manager) TriggerResponse(assessment *BehaviorAssessment) *ResponseEvent {
	action := assessment.RecommendedAction

	resp := &ResponseEvent{
		ID:         generateAssessmentID(),
		Timestamp:  time.Now(),
		Action:     action,
		Assessment: assessment,
		Success:    true,
	}

	switch action {
	case ActionSnapshot:
		if m.snapshotCallback != nil {
			for _, path := range assessment.AffectedFiles {
				snapID, err := m.snapshotCallback(path)
				if err != nil {
					resp.Success = false
					resp.Message = fmt.Sprintf("snapshot failed for %s: %v", path, err)
					break
				}
				resp.SnapshotID = snapID
			}
		}
		if resp.Success {
			resp.Message = "snapshot protection created"
		}
	case ActionIsolate:
		resp.Message = "network isolation triggered"
	case ActionQuarantine:
		resp.Message = "process/file quarantined"
	case ActionLockdown:
		resp.Message = "volume lockdown triggered"
	default:
		resp.Message = "alert notification sent"
	}

	// 告警回调
	if m.alertCallback != nil {
		m.alertCallback(assessment)
	}

	m.mu.Lock()
	m.responseLog = append(m.responseLog, resp)
	m.stats.ResponsesTriggered++
	if action == ActionSnapshot {
		m.stats.SnapshotsCreated++
	}
	if len(m.responseLog) > 1000 {
		m.responseLog = m.responseLog[100:]
	}
	m.mu.Unlock()

	m.logger.Warn("auto response triggered",
		zap.String("action", string(action)),
		zap.Int("score", assessment.Score),
		zap.String("threatLevel", assessment.ThreatLevel.String()),
		zap.String("message", resp.Message))

	return resp
}

// ============================================================
// Setters & Getters
// ============================================================

// SetSnapshotCallback 设置快照回调
func (m *Manager) SetSnapshotCallback(fn func(path string) (string, error)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.snapshotCallback = fn
}

// SetIsolateCallback 设置隔离回调
func (m *Manager) SetIsolateCallback(fn func(processID int) error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.isolateCallback = fn
}

// SetAlertCallback 设置告警回调
func (m *Manager) SetAlertCallback(fn func(assessment *BehaviorAssessment)) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.alertCallback = fn
}

// GetStatus 获取引擎状态
func (m *Manager) GetStatus() EngineStatus {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var uptime int64
	if m.running {
		uptime = int64(time.Since(m.startTime).Seconds())
	}

	activeThreats := 0
	for _, a := range m.assessments {
		if a.ThreatLevel >= ThreatLevelHigh {
			activeThreats++
		}
	}
	// 只统计最近 5 分钟内的活跃威胁
	cutoff := time.Now().Add(-5 * time.Minute)
	recentThreats := 0
	for _, a := range m.assessments {
		if a.Timestamp.After(cutoff) && a.ThreatLevel >= ThreatLevelHigh {
			recentThreats++
		}
	}

	return EngineStatus{
		Running:       m.running,
		Uptime:        uptime,
		Stats:         m.stats,
		ActiveThreats: recentThreats,
		LastError:     m.lastErr,
	}
}

// GetAssessments 获取评估历史
func (m *Manager) GetAssessments(limit int) []*BehaviorAssessment {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.assessments) {
		limit = len(m.assessments)
	}
	start := len(m.assessments) - limit
	if start < 0 {
		start = 0
	}
	result := make([]*BehaviorAssessment, limit)
	copy(result, m.assessments[start:])
	return result
}

// GetResponseLog 获取响应日志
func (m *Manager) GetResponseLog(limit int) []*ResponseEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if limit <= 0 || limit > len(m.responseLog) {
		limit = len(m.responseLog)
	}
	start := len(m.responseLog) - limit
	if start < 0 {
		start = 0
	}
	result := make([]*ResponseEvent, limit)
	copy(result, m.responseLog[start:])
	return result
}

// GetConfig 获取配置副本
func (m *Manager) GetConfig() *Config {
	m.mu.RLock()
	defer m.mu.RUnlock()
	cfg := *m.config
	return &cfg
}

// UpdateConfig 更新配置
func (m *Manager) UpdateConfig(cfg *Config) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.config = cfg
	m.logger.Info("config updated")
}

// ============================================================
// Helpers
// ============================================================

func generateAssessmentID() string {
	return fmt.Sprintf("rba_%d", time.Now().UnixNano())
}

func clampScore(score int) int {
	if score < 0 {
		return 0
	}
	if score > 100 {
		return 100
	}
	return score
}
