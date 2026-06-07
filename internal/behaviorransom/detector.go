// Package behaviorransom 提供基于行为分析的勒索软件检测功能
// detector.go - 行为模式检测器
package behaviorransom

import (
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"
)

// BehaviorDetector 行为模式检测器
type BehaviorDetector struct {
	mu       sync.RWMutex
	config   DetectorConfig
	patterns []BehaviorPattern
	entropy  *EntropyAnalyzer
}

// NewBehaviorDetector 创建新的行为检测器
func NewBehaviorDetector(config DetectorConfig) *BehaviorDetector {
	return &BehaviorDetector{
		config:   config,
		patterns: getDefaultPatterns(),
		entropy:  NewEntropyAnalyzer(config.EntropyThreshold),
	}
}

// getDefaultPatterns 获取默认行为模式
func getDefaultPatterns() []BehaviorPattern {
	return []BehaviorPattern{
		{
			ID:          "rapid-encryption",
			Name:        "快速加密行为",
			Description: "短时间内大量文件被修改且熵值显著增加",
			Severity:    ThreatLevelCritical,
			Weight:      90,
			Indicators: []PatternIndicator{
				{Type: "file_modify_rate", Threshold: 50, TimeWindowSec: 60},
				{Type: "entropy_spike", Threshold: 7.5, TimeWindowSec: 60},
			},
		},
		{
			ID:          "mass-rename",
			Name:        "批量重命名",
			Description: "大量文件被重命名为可疑扩展名",
			Severity:    ThreatLevelHigh,
			Weight:      80,
			Indicators: []PatternIndicator{
				{Type: "file_rename_rate", Threshold: 30, TimeWindowSec: 60},
				{Type: "suspicious_extension", Threshold: 10, TimeWindowSec: 60},
			},
		},
		{
			ID:          "rapid-delete",
			Name:        "快速删除",
			Description: "短时间内大量文件被删除",
			Severity:    ThreatLevelHigh,
			Weight:      70,
			Indicators: []PatternIndicator{
				{Type: "file_delete_rate", Threshold: 100, TimeWindowSec: 60},
			},
		},
		{
			ID:          "suspicious-write",
			Name:        "可疑写入模式",
			Description: "非正常用户行为的文件写入模式",
			Severity:    ThreatLevelMedium,
			Weight:      60,
			Indicators: []PatternIndicator{
				{Type: "file_create_rate", Threshold: 80, TimeWindowSec: 60},
			},
		},
		{
			ID:          "extension-change",
			Name:        "扩展名变更",
			Description: "文件扩展名被批量修改",
			Severity:    ThreatLevelHigh,
			Weight:      75,
			Indicators: []PatternIndicator{
				{Type: "extension_change_rate", Threshold: 20, TimeWindowSec: 60},
			},
		},
	}
}

// DetectPatterns 检测行为模式
func (bd *BehaviorDetector) DetectPatterns(activities []FileActivity) []ThreatEvent {
	bd.mu.RLock()
	defer bd.mu.RUnlock()

	if len(activities) == 0 {
		return nil
	}

	var threats []ThreatEvent

	// 统计操作
	stats := bd.calculateStats(activities)

	// 检查每个模式
	for _, pattern := range bd.patterns {
		threat := bd.checkPattern(pattern, activities, stats)
		if threat != nil {
			threats = append(threats, *threat)
		}
	}

	return threats
}

// activityStats 活动统计
type activityStats struct {
	totalCount       int
	modifyCount      int
	deleteCount      int
	renameCount      int
	createCount      int
	suspExtCount     int
	extChangeCount   int
	highEntropyCount int
	avgEntropy       float64
	maxEntropy       float64
	processCounts    map[string]int
	userCounts       map[string]int
}

// calculateStats 计算活动统计
func (bd *BehaviorDetector) calculateStats(activities []FileActivity) activityStats {
	stats := activityStats{
		processCounts: make(map[string]int),
		userCounts:    make(map[string]int),
	}

	var totalEntropy float64

	for _, a := range activities {
		stats.totalCount++

		switch a.Operation {
		case FileOpModify:
			stats.modifyCount++
		case FileOpDelete:
			stats.deleteCount++
		case FileOpRename:
			stats.renameCount++
		case FileOpCreate:
			stats.createCount++
		}

		// 检查可疑扩展名
		if bd.isSuspiciousExtension(a.Path) {
			stats.suspExtCount++
		}

		// 检查扩展名变更（重命名操作）
		if a.Operation == FileOpRename && a.OldPath != "" {
			oldExt := GetFileExtension(a.OldPath)
			newExt := GetFileExtension(a.Path)
			if oldExt != newExt {
				stats.extChangeCount++
			}
		}

		// 检查熵值
		if a.Entropy >= bd.config.EntropyThreshold {
			stats.highEntropyCount++
		}
		if a.Entropy > stats.maxEntropy {
			stats.maxEntropy = a.Entropy
		}
		totalEntropy += a.Entropy

		stats.processCounts[a.ProcessName]++
		stats.userCounts[a.UserID]++
	}

	if stats.totalCount > 0 {
		stats.avgEntropy = totalEntropy / float64(stats.totalCount)
	}

	return stats
}

// checkPattern 检查单个模式
func (bd *BehaviorDetector) checkPattern(pattern BehaviorPattern, activities []FileActivity, stats activityStats) *ThreatEvent {
	score := 0
	matched := false

	for _, indicator := range pattern.Indicators {
		switch indicator.Type {
		case "file_modify_rate":
			if float64(stats.modifyCount) >= indicator.Threshold {
				score += pattern.Weight / len(pattern.Indicators)
				matched = true
			}
		case "file_delete_rate":
			if float64(stats.deleteCount) >= indicator.Threshold {
				score += pattern.Weight / len(pattern.Indicators)
				matched = true
			}
		case "file_rename_rate":
			if float64(stats.renameCount) >= indicator.Threshold {
				score += pattern.Weight / len(pattern.Indicators)
				matched = true
			}
		case "file_create_rate":
			if float64(stats.createCount) >= indicator.Threshold {
				score += pattern.Weight / len(pattern.Indicators)
				matched = true
			}
		case "entropy_spike":
			if stats.maxEntropy >= indicator.Threshold || stats.highEntropyCount >= 10 {
				score += pattern.Weight / len(pattern.Indicators)
				matched = true
			}
		case "suspicious_extension":
			if float64(stats.suspExtCount) >= indicator.Threshold {
				score += pattern.Weight / len(pattern.Indicators)
				matched = true
			}
		case "extension_change_rate":
			if float64(stats.extChangeCount) >= indicator.Threshold {
				score += pattern.Weight / len(pattern.Indicators)
				matched = true
			}
		}
	}

	if !matched {
		return nil
	}

	// 确定威胁级别
	level := bd.scoreToLevel(score)

	// 确定响应动作
	action := bd.decideAction(level)

	// 收集受影响文件
	affectedFiles := bd.collectAffectedFiles(activities)

	// 确定主要进程和用户
	processName, processID, userID := bd.identifySource(stats)

	// 确定源路径
	sourcePath := bd.identifySourcePath(activities)

	return &ThreatEvent{
		ID:            fmt.Sprintf("threat-%d", time.Now().UnixNano()),
		ThreatLevel:   level,
		Score:         score,
		Pattern:       pattern.ID,
		Description:   pattern.Description,
		SourcePath:    sourcePath,
		AffectedFiles: affectedFiles,
		Action:        action,
		Timestamp:     time.Now(),
		ProcessName:   processName,
		ProcessID:     processID,
		UserID:        userID,
		EntropyDelta:  stats.maxEntropy - stats.avgEntropy,
		Details: map[string]interface{}{
			"pattern_name":     pattern.Name,
			"total_activities": stats.totalCount,
			"modify_count":     stats.modifyCount,
			"delete_count":     stats.deleteCount,
			"rename_count":     stats.renameCount,
			"create_count":     stats.createCount,
			"avg_entropy":      stats.avgEntropy,
			"max_entropy":      stats.maxEntropy,
			"high_entropy":     stats.highEntropyCount,
		},
	}
}

// scoreToLevel 分数转威胁级别
func (bd *BehaviorDetector) scoreToLevel(score int) ThreatLevel {
	switch {
	case score >= 80:
		return ThreatLevelCritical
	case score >= 60:
		return ThreatLevelHigh
	case score >= 40:
		return ThreatLevelMedium
	case score >= 20:
		return ThreatLevelLow
	default:
		return ThreatLevelNone
	}
}

// decideAction 根据威胁级别决定响应动作
func (bd *BehaviorDetector) decideAction(level ThreatLevel) ResponseAction {
	switch level {
	case ThreatLevelCritical:
		if bd.config.AutoQuarantine {
			return ActionQuarantine
		}
		return ActionBlock
	case ThreatLevelHigh:
		return ActionBlock
	case ThreatLevelMedium:
		return ActionAlert
	default:
		return ActionAlert
	}
}

// isSuspiciousExtension 检查是否是可疑扩展名
func (bd *BehaviorDetector) isSuspiciousExtension(path string) bool {
	ext := GetFileExtension(path)
	for _, se := range bd.config.SuspiciousExtensions {
		if strings.EqualFold(ext, se) {
			return true
		}
	}
	return false
}

// collectAffectedFiles 收集受影响文件
func (bd *BehaviorDetector) collectAffectedFiles(activities []FileActivity) []string {
	fileSet := make(map[string]bool)
	for _, a := range activities {
		fileSet[a.Path] = true
		if a.OldPath != "" {
			fileSet[a.OldPath] = true
		}
	}

	files := make([]string, 0, len(fileSet))
	for f := range fileSet {
		files = append(files, f)
	}
	return files
}

// identifySource 识别主要来源进程和用户
func (bd *BehaviorDetector) identifySource(stats activityStats) (string, int, string) {
	// 找出操作最多的进程
	maxProcCount := 0
	maxProcName := ""
	for proc, count := range stats.processCounts {
		if count > maxProcCount {
			maxProcCount = count
			maxProcName = proc
		}
	}

	// 找出操作最多的用户
	maxUserCount := 0
	maxUserID := ""
	for uid, count := range stats.userCounts {
		if count > maxUserCount {
			maxUserCount = count
			maxUserID = uid
		}
	}

	return maxProcName, 0, maxUserID
}

// identifySourcePath 识别源路径
func (bd *BehaviorDetector) identifySourcePath(activities []FileActivity) string {
	if len(activities) == 0 {
		return ""
	}

	// 找出最活跃的目录
	dirCounts := make(map[string]int)
	for _, a := range activities {
		dir := getDirectory(a.Path)
		dirCounts[dir]++
	}

	maxCount := 0
	maxDir := ""
	for dir, count := range dirCounts {
		if count > maxCount {
			maxCount = count
			maxDir = dir
		}
	}

	return maxDir
}

// getDirectory 获取文件所在目录
func getDirectory(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i+1]
		}
	}
	return "."
}

// AnalyzeEntropyChange 分析熵值变化
func (bd *BehaviorDetector) AnalyzeEntropyChange(activities []FileActivity) (bool, float64) {
	bd.mu.RLock()
	defer bd.mu.RUnlock()

	if len(activities) < 2 {
		return false, 0
	}

	var initialEntropy, finalEntropy float64
	var count int

	// 计算前半段和后半段的平均熵值
	half := len(activities) / 2
	for i, a := range activities {
		if i < half {
			initialEntropy += a.Entropy
		} else {
			finalEntropy += a.Entropy
		}
		count++
	}

	if half == 0 {
		return false, 0
	}

	initialAvg := initialEntropy / float64(half)
	finalAvg := finalEntropy / float64(len(activities)-half)
	delta := finalAvg - initialAvg

	// 检测熵值突增
	if delta >= bd.config.EntropyDeltaThreshold {
		return true, delta
	}

	return false, delta
}

// GetPatterns 获取所有行为模式
func (bd *BehaviorDetector) GetPatterns() []BehaviorPattern {
	bd.mu.RLock()
	defer bd.mu.RUnlock()
	return bd.patterns
}

// AddPattern 添加自定义行为模式
func (bd *BehaviorDetector) AddPattern(pattern BehaviorPattern) {
	bd.mu.Lock()
	defer bd.mu.Unlock()
	bd.patterns = append(bd.patterns, pattern)
	log.Printf("添加自定义行为模式: %s (%s)", pattern.ID, pattern.Name)
}

// RemovePattern 移除行为模式
func (bd *BehaviorDetector) RemovePattern(patternID string) bool {
	bd.mu.Lock()
	defer bd.mu.Unlock()

	for i, p := range bd.patterns {
		if p.ID == patternID {
			bd.patterns = append(bd.patterns[:i], bd.patterns[i+1:]...)
			log.Printf("移除行为模式: %s", patternID)
			return true
		}
	}
	return false
}

// GetEntropyAnalyzer 获取熵值分析器
func (bd *BehaviorDetector) GetEntropyAnalyzer() *EntropyAnalyzer {
	return bd.entropy
}

// CalculateRiskScore 计算风险评分
func (bd *BehaviorDetector) CalculateRiskScore(activities []FileActivity) float64 {
	if len(activities) == 0 {
		return 0
	}

	score := 0.0

	// 因子1: 操作速率
	rate := float64(len(activities)) / float64(bd.config.WindowSizeSec)
	expectedRate := float64(bd.config.FileRateThreshold) / float64(bd.config.WindowSizeSec)
	if rate > expectedRate {
		score += math.Min(30, (rate/expectedRate)*10)
	}

	// 因子2: 熵值异常
	var totalEntropy float64
	highEntropyCount := 0
	for _, a := range activities {
		totalEntropy += a.Entropy
		if a.Entropy >= bd.config.EntropyThreshold {
			highEntropyCount++
		}
	}
	avgEntropy := totalEntropy / float64(len(activities))
	if avgEntropy > 5.0 {
		score += 20
	}
	if highEntropyCount > len(activities)/2 {
		score += 20
	}

	// 因子3: 可疑扩展名
	suspCount := 0
	for _, a := range activities {
		if bd.isSuspiciousExtension(a.Path) {
			suspCount++
		}
	}
	if suspCount > 0 {
		score += float64(suspCount) / float64(len(activities)) * 30
	}

	return math.Min(100, score)
}
