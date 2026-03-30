// Package ransomware 提供勒索软件检测与防护功能
// detector.go - 文件行为监控和异常写入检测
package ransomware

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// FileBehaviorMonitor 文件行为监控接口
type FileBehaviorMonitor interface {
	// Start 启动监控
	Start(ctx context.Context) error

	// Stop 停止监控
	Stop() error

	// AddWatchPath 添加监控路径
	AddWatchPath(path string) error

	// RemoveWatchPath 移除监控路径
	RemoveWatchPath(path string) error

	// GetRecentEvents 获取最近事件
	GetRecentEvents(limit int) []FileEvent
}

// AnomalyDetector 异常写入检测接口
type AnomalyDetector interface {
	// Analyze 分析文件事件
	Analyze(events []FileEvent) *ThreatAssessment

	// DetectEncryption 检测加密行为
	DetectEncryption(path string) (bool, float64, error)

	// GetPatterns 获取已知威胁模式
	GetPatterns() []BehaviorPattern
}

// ProtectionStrategy 防护策略接口
type ProtectionStrategy interface {
	// Execute 执行防护动作
 Execute(assessment *ThreatAssessment) (*ProtectionEvent, error)

	// CreateSnapshot 创建保护快照
	CreateSnapshot(volume string) (string, error)

	// LockVolume 锁定卷
	LockVolume(volume string) error

	// UnlockVolume 解锁卷
	UnlockVolume(volume string) error
}

// Detector 勒索软件检测器
type Detector struct {
	mu sync.RWMutex

	// config 检测配置
	config DetectionConfig

	// monitor 文件行为监控器
	monitor FileBehaviorMonitor

	// anomalyDetector 异常检测器
	anomalyDetector AnomalyDetector

	// protection 防护策略
	protection ProtectionStrategy

	// writeOncePolicy WriteOnce策略
	writeOncePolicy WriteOncePolicy

	// snapshotConfig 快照保护配置
	snapshotConfig SnapshotProtectionConfig

	// eventBuffer 事件缓冲
	eventBuffer []FileEvent

	// threatQueue 威胁队列
	threatQueue []*ThreatAssessment

	// stats 统计
	stats DetectionStats

	// running 运行状态
	running bool

	// startTime 启动时间
	startTime time.Time

	// stopChan 停止通道
	stopChan chan struct{}

	// eventChan 事件通道
	eventChan chan FileEvent

	// alertChan 告警通道
	alertChan chan *ThreatAssessment
}

// NewDetector 创建检测器
func NewDetector(config DetectionConfig) *Detector {
	d := &Detector{
		config:      config,
		eventBuffer: make([]FileEvent, 0, 1000),
		threatQueue: make([]*ThreatAssessment, 0),
		stopChan:    make(chan struct{}),
		eventChan:   make(chan FileEvent, 100),
		alertChan:   make(chan *ThreatAssessment, 10),
	}

	// 初始化内置异常检测器
	d.anomalyDetector = NewBuiltInAnomalyDetector(config.Level)

	return d
}

// SetProtectionStrategy 设置防护策略
func (d *Detector) SetProtectionStrategy(strategy ProtectionStrategy) {
	d.mu.Lock()
	d.protection = strategy
	d.mu.Unlock()
}

// SetWriteOncePolicy 设置WriteOnce策略
func (d *Detector) SetWriteOncePolicy(policy WriteOncePolicy) {
	d.mu.Lock()
	d.writeOncePolicy = policy
	d.mu.Unlock()
}

// SetSnapshotConfig 设置快照保护配置
func (d *Detector) SetSnapshotConfig(config SnapshotProtectionConfig) {
	d.mu.Lock()
	d.snapshotConfig = config
	d.mu.Unlock()
}

// Start 启动检测器
func (d *Detector) Start(ctx context.Context) error {
	d.mu.Lock()
	if d.running {
		d.mu.Unlock()
		return nil
	}
	d.running = true
	d.startTime = time.Now()
	d.mu.Unlock()

	// 启动事件处理循环
	go d.eventLoop(ctx)

	// 启动定时分析
	go d.analysisLoop(ctx)

	// 启动防护检查
	go d.protectionLoop(ctx)

	log.Println("勒索软件检测器已启动")
	return nil
}

// Stop 停止检测器
func (d *Detector) Stop() {
	d.mu.Lock()
	if !d.running {
		d.mu.Unlock()
		return
	}
	d.running = false
	close(d.stopChan)
	d.mu.Unlock()

	log.Println("勒索软件检测器已停止")
}

// GetStatus 获取状态
func (d *Detector) GetStatus() DetectorStatus {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var uptime int64
	if d.running {
		uptime = int64(time.Since(d.startTime).Seconds())
	}

	return DetectorStatus{
		Running:       d.running,
		Uptime:        uptime,
		Stats:         d.stats,
		Config:        d.config,
		ActiveThreats: len(d.threatQueue),
	}
}

// RecordEvent 记录文件事件
func (d *Detector) RecordEvent(event FileEvent) {
	d.mu.Lock()
	d.eventBuffer = append(d.eventBuffer, event)
	d.stats.TotalEvents++

	// 保持缓冲区大小
	if len(d.eventBuffer) > 1000 {
		d.eventBuffer = d.eventBuffer[1:]
	}
	d.mu.Unlock()

	// 发送到事件通道
	select {
	case d.eventChan <- event:
	default:
		// 通道满，丢弃
	}
}

// eventLoop 事件处理循环
func (d *Detector) eventLoop(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopChan:
			return
		case event := <-d.eventChan:
			d.processEvent(event)
		}
	}
}

// processEvent 处理单个事件
func (d *Detector) processEvent(event FileEvent) {
	// 检查是否在监控路径内
	if !d.isWatchedPath(event.Path) {
		return
	}

	// 检查是否被排除
	if d.isExcludedPath(event.Path) {
		return
	}

	// WriteOnce 保护检查
	if d.writeOncePolicy.Enabled && d.isWriteOnceProtected(event.Path) {
		d.handleWriteOnceViolation(event)
		return
	}

	// 检查可疑扩展名
	if d.hasSuspiciousExtension(event.Extension) {
		d.markSuspicious(event, "suspicious_extension")
	}
}

// analysisLoop 定时分析循环
func (d *Detector) analysisLoop(ctx context.Context) {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopChan:
			return
		case <-ticker.C:
			d.runAnalysis()
		}
	}
}

// runAnalysis 执行分析
func (d *Detector) runAnalysis() {
	d.mu.RLock()
	events := make([]FileEvent, len(d.eventBuffer))
	copy(events, d.eventBuffer)
	d.mu.RUnlock()

	if len(events) == 0 {
		return
	}

	// 分析最近的事件窗口
	window := d.getRecentWindow(events, 60) // 60秒窗口
	if len(window) < 3 {
		return
	}

	assessment := d.anomalyDetector.Analyze(window)
	if assessment != nil && assessment.Level >= ThreatLevelLow {
		d.handleThreat(assessment)
	}
}

// protectionLoop 防护检查循环
func (d *Detector) protectionLoop(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-d.stopChan:
			return
		case <-ticker.C:
			d.checkProtectionQueue()
		case assessment := <-d.alertChan:
			d.executeProtection(assessment)
		}
	}
}

// handleThreat 处理威胁
func (d *Detector) handleThreat(assessment *ThreatAssessment) {
	d.mu.Lock()
	d.threatQueue = append(d.threatQueue, assessment)
	d.stats.ThreatsDetected++
	now := time.Now()
	d.stats.LastThreatTime = &now
	d.stats.LastThreatLevel = assessment.Level
	d.mu.Unlock()

	// 发送告警
	select {
	case d.alertChan <- assessment:
	default:
		log.Printf("威胁告警通道满，威胁ID: %s", assessment.AssessmentID)
	}
}

// executeProtection 执行防护
func (d *Detector) executeProtection(assessment *ThreatAssessment) {
	if d.protection == nil {
		log.Printf("防护策略未配置，威胁ID: %s", assessment.AssessmentID)
		return
	}

	// 根据威胁级别决定动作
	action := d.decideAction(assessment)

	if action == ActionAlert {
		log.Printf("威胁告警: %s, 级别: %d, 分数: %d",
			assessment.AssessmentID, assessment.Level, assessment.Score)
		return
	}

	// 执行防护
	pe, err := d.protection.Execute(assessment)
	if err != nil {
		log.Printf("防护执行失败: %v", err)
		return
	}

	d.mu.Lock()
	d.stats.ProtectionsTriggered++
	if pe.SnapshotID != "" {
		d.stats.SnapshotsCreated++
	}
	d.mu.Unlock()

	log.Printf("防护已执行: 动作=%s, 成功=%v", pe.Action, pe.Success)
}

// checkProtectionQueue 检查防护队列
func (d *Detector) checkProtectionQueue() {
	d.mu.Lock()
	if len(d.threatQueue) == 0 {
		d.mu.Unlock()
		return
	}

	// 取出最严重的威胁
	maxLevel := ThreatLevelNone
	maxIdx := -1
	for i, t := range d.threatQueue {
		if t.Level > maxLevel {
			maxLevel = t.Level
			maxIdx = i
		}
	}

	if maxIdx >= 0 && maxLevel >= ThreatLevelMedium {
		assessment := d.threatQueue[maxIdx]
		d.threatQueue = append(d.threatQueue[:maxIdx], d.threatQueue[maxIdx+1:]...)
		d.mu.Unlock()

		d.executeProtection(assessment)
		return
	}

	d.mu.Unlock()
}

// decideAction 决定防护动作
func (d *Detector) decideAction(assessment *ThreatAssessment) ProtectionAction {
	if !d.config.AutoProtectionEnabled {
		return ActionAlert
	}

	// 根据威胁级别决定
	switch assessment.Level {
	case ThreatLevelCritical:
		return ActionLockdown
	case ThreatLevelHigh:
		if d.config.SnapshotOnThreat {
			return ActionSnapshot
		}
		return ActionBlock
	case ThreatLevelMedium:
		return ActionQuarantine
	case ThreatLevelLow:
		return ActionAlert
	default:
		return ActionAlert
	}
}

// BuiltInAnomalyDetector 内置异常检测器
type BuiltInAnomalyDetector struct {
	level   DetectionLevel
	patterns []BehaviorPattern
}

// NewBuiltInAnomalyDetector 创建内置异常检测器
func NewBuiltInAnomalyDetector(level DetectionLevel) *BuiltInAnomalyDetector {
	d := &BuiltInAnomalyDetector{
		level: level,
		patterns: d.getDefaultPatterns(),
	}
	return d
}

// getDefaultPatterns 获取默认威胁模式
func (d *BuiltInAnomalyDetector) getDefaultPatterns() []BehaviorPattern {
	return []BehaviorPattern{
		{
			PatternID: "rapid-encryption",
			Name:      "快速加密行为",
			Description: "短时间内大量文件被加密",
			Severity: ThreatLevelCritical,
			ConfidenceWeight: 90,
			Indicators: []PatternIndicator{
				{EventType: FileEventEncrypt, MinCount: 10, TimeWindowSec: 30, EntropyMin: 7.5},
			},
		},
		{
			PatternID: "bulk-extension-change",
			Name:      "批量扩展名修改",
			Description: "大量文件扩展名被修改为可疑类型",
			Severity: ThreatLevelHigh,
			ConfidenceWeight: 80,
			Indicators: []PatternIndicator{
				{EventType: FileEventRename, MinCount: 20, TimeWindowSec: 60},
			},
		},
		{
			PatternID: "rapid-delete",
			Name:      "快速删除",
			Description: "短时间内大量文件被删除",
			Severity: ThreatLevelHigh,
			ConfidenceWeight: 70,
			Indicators: []PatternIndicator{
				{EventType: FileEventDelete, MinCount: 50, TimeWindowSec: 30},
			},
		},
		{
			PatternID: "suspicious-write-pattern",
			Name:      "可疑写入模式",
			Description: "非正常用户行为的文件写入",
			Severity: ThreatLevelMedium,
			ConfidenceWeight: 60,
			Indicators: []PatternIndicator{
				{EventType: FileEventModify, MinCount: 100, TimeWindowSec: 60},
			},
		},
	}
}

// Analyze 分析事件
func (d *BuiltInAnomalyDetector) Analyze(events []FileEvent) *ThreatAssessment {
	assessment := &ThreatAssessment{
		AssessmentID: uuid.New().String(),
		Timestamp:    time.Now(),
		Indicators:   make([]ThreatIndicator, 0),
		AffectedFiles: make([]string, 0),
		Details:      make(map[string]interface{}),
	}

	// 统计事件类型
	counts := make(map[FileEventType]int)
	encryptedCount := 0
	highEntropyFiles := 0

	for _, e := range events {
		counts[e.Type]++
		if e.IsEncrypted {
			encryptedCount++
		}
		if e.Entropy > 7.5 {
			highEntropyFiles++
		}
	}

	// 检查模式匹配
	maxScore := 0
	for _, pattern := range d.patterns {
		score := d.matchPattern(events, pattern)
		if score > maxScore {
			maxScore = score
			assessment.Details["matched_pattern"] = pattern.PatternID
			assessment.Details["pattern_name"] = pattern.Name
		}
	}

	// 添加指标
	if encryptedCount > 5 {
		assessment.Indicators = append(assessment.Indicators, ThreatIndicator{
			Type:        "encrypted_files",
			Description: "检测到加密文件",
			Weight:      30,
			Value:       encryptedCount,
			Threshold:   5,
		})
	}

	if highEntropyFiles > 10 {
		assessment.Indicators = append(assessment.Indicators, ThreatIndicator{
			Type:        "high_entropy",
			Description: "高熵值文件（可能被加密）",
			Weight:      25,
			Value:       highEntropyFiles,
			Threshold:   10,
		})
	}

	// 计算总分和级别
	assessment.Score = maxScore + encryptedCount*3 + highEntropyFiles*2
	assessment.Level = d.scoreToLevel(assessment.Score)
	assessment.Confidence = min(100, assessment.Score)

	// 收集受影响文件
	for _, e := range events {
		if e.IsEncrypted || e.Entropy > 7.5 {
			assessment.AffectedFiles = append(assessment.AffectedFiles, e.Path)
		}
	}

	// 低威胁不返回
	if assessment.Level < ThreatLevelLow {
		return nil
	}

	return assessment
}

// matchPattern 匹配威胁模式
func (d *BuiltInAnomalyDetector) matchPattern(events []FileEvent, pattern BehaviorPattern) int {
	score := 0
	for _, indicator := range pattern.Indicators {
		count := 0
		windowStart := time.Now().Add(-time.Duration(indicator.TimeWindowSec) * time.Second)

		for _, e := range events {
			if e.Timestamp.After(windowStart) && e.Type == indicator.EventType {
				count++
				if indicator.EntropyMin > 0 && e.Entropy >= indicator.EntropyMin {
					score += 10
				}
			}
		}

		if count >= indicator.MinCount {
			score += pattern.ConfidenceWeight
		}
	}
	return score
}

// scoreToLevel 分数转级别
func (d *BuiltInAnomalyDetector) scoreToLevel(score int) ThreatLevel {
 thresholds := map[DetectionLevel]map[int]ThreatLevel{
		DetectionLevelLow:    {50: ThreatLevelLow, 70: ThreatLevelMedium, 85: ThreatLevelHigh, 95: ThreatLevelCritical},
		DetectionLevelMedium: {30: ThreatLevelLow, 50: ThreatLevelMedium, 70: ThreatLevelHigh, 85: ThreatLevelCritical},
		DetectionLevelHigh:   {20: ThreatLevelLow, 35: ThreatLevelMedium, 50: ThreatLevelHigh, 70: ThreatLevelCritical},
	}

	t := thresholds[d.level]
	for threshold, level := range t {
		if score >= threshold {
			return level
		}
	}
	return ThreatLevelNone
}

// DetectEncryption 检测文件是否被加密
func (d *BuiltInAnomalyDetector) DetectEncryption(path string) (bool, float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return false, 0, err
	}

	if len(data) == 0 {
		return false, 0, nil
	}

	// 计算熵值
	entropy := d.calculateEntropy(data)

	// 高熵值通常表示加密（>7.5）
	isEncrypted := entropy > 7.5

	return isEncrypted, entropy, nil
}

// calculateEntropy 计算数据熵值
func (d *BuiltInAnomalyDetector) calculateEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}

	frequency := make(map[byte]int)
	for _, b := range data {
		frequency[b]++
	}

	var entropy float64
	total := float64(len(data))

	for _, count := range frequency {
		p := float64(count) / total
		entropy -= p * math.Log2(p)
	}

	return entropy
}

// GetPatterns 获取模式列表
func (d *BuiltInAnomalyDetector) GetPatterns() []BehaviorPattern {
	return d.patterns
}

// 辅助方法

func (d *Detector) isWatchedPath(path string) bool {
	for _, wp := range d.config.WatchPaths {
		if strings.HasPrefix(path, wp) {
			return true
		}
	}
	return len(d.config.WatchPaths) == 0 // 未配置则监控所有
}

func (d *Detector) isExcludedPath(path string) bool {
	for _, ep := range d.config.ExcludePaths {
		if strings.HasPrefix(path, ep) {
			return true
		}
	}
	return false
}

func (d *Detector) isWriteOnceProtected(path string) bool {
	for _, pp := range d.writeOncePolicy.ProtectedPaths {
		if strings.HasPrefix(path, pp) {
			return true
		}
	}
	return false
}

func (d *Detector) hasSuspiciousExtension(ext string) bool {
	ext = strings.ToLower(ext)
	for _, se := range d.config.SuspiciousExtensions {
		if ext == strings.ToLower(se) {
			return true
		}
	}
	return false
}

func (d *Detector) handleWriteOnceViolation(event FileEvent) {
	log.Printf("WriteOnce保护违规: path=%s, type=%s, user=%s",
		event.Path, event.Type, event.UserID)
	// TODO: 实际阻止操作需要与存储层集成
}

func (d *Detector) markSuspicious(event FileEvent, reason string) {
	log.Printf("可疑事件: path=%s, reason=%s", event.Path, reason)
}

func (d *Detector) getRecentWindow(events []FileEvent, windowSec int) []FileEvent {
	windowStart := time.Now().Add(-time.Duration(windowSec) * time.Second)
	result := make([]FileEvent, 0)
	for _, e := range events {
		if e.Timestamp.After(windowStart) {
			result = append(result, e)
		}
	}
	return result
}

// HashFile 计算文件哈希
func HashFile(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// GenerateEventID 生成事件ID
func GenerateEventID() string {
	return uuid.New().String()
}

// ScanDirectoryForEncryption 扫描目录检测加密文件
func ScanDirectoryForEncryption(rootPath string, maxFiles int) ([]string, error) {
	detector := NewBuiltInAnomalyDetector(DetectionLevelMedium)
	encryptedFiles := make([]string, 0)
	count := 0

	err := filepath.Walk(rootPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误继续
		}

		if info.IsDir() {
			return nil
		}

		if count >= maxFiles {
			return fmt.Errorf("达到最大扫描数")
		}

		isEncrypted, _, err := detector.DetectEncryption(path)
		if err != nil {
			return nil
		}

		if isEncrypted {
			encryptedFiles = append(encryptedFiles, path)
		}

		count++
		return nil
	})

	if err != nil && err.Error() != "达到最大扫描数" {
		return nil, err
	}

	return encryptedFiles, nil
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}