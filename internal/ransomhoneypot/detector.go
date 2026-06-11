// Package ransomhoneypot - AI 行为检测器
// 通过熵值分析、批量操作检测、模式匹配等 AI 算法识别勒索软件活动
package ransomhoneypot

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// ============================================================
// AI 行为检测器
// ============================================================

// AIBehaviorDetector AI 驱动的行为分析检测器
type AIBehaviorDetector struct {
	mu       sync.RWMutex
	config   *HoneypotConfig
	patterns []*ThreatPattern // 威胁检测模式库
}

// NewAIBehaviorDetector 创建 AI 行为检测器
func NewAIBehaviorDetector(config *HoneypotConfig) *AIBehaviorDetector {
	d := &AIBehaviorDetector{
		config:   config,
		patterns: make([]*ThreatPattern, 0),
	}
	d.initDefaultPatterns()
	return d
}

// initDefaultPatterns 初始化默认威胁检测模式
// 参考 TrueNAS Ransomware Detection 的模式库
func (d *AIBehaviorDetector) initDefaultPatterns() {
	d.patterns = []*ThreatPattern{
		{
			ID:             "encryption_behavior",
			Name:           "加密行为检测",
			Description:    "检测文件熵值急剧升高至加密数据特征范围（>7.0），常见于 AES/ChaCha20 加密",
			Enabled:        true,
			MinEntropy:     7.0,
			BatchThreshold: 5,
			TimeWindowSec:  120,
			SuspiciousExts: []string{".encrypted", ".locked", ".crypto", ".crypt", ".enc"},
			Weight:         0.35,
			Severity:       ThreatLevelCritical,
		},
		{
			ID:             "batch_rename_ext",
			Name:           "批量扩展名篡改",
			Description:    "检测短时间内大量文件扩展名被修改为统一可疑后缀",
			Enabled:        true,
			MinEntropy:     0,
			BatchThreshold: 10,
			TimeWindowSec:  60,
			SuspiciousExts: []string{
				".encrypted", ".locked", ".crypto", ".crypt", ".enc", ".crypted",
				".WNCRY", ".wncry", ".locky", ".zepto", ".thor", ".aaa", ".abc",
				".xyz", ".zzz", ".micro", ".xxx", ".ttt", ".ecc", ".ezz",
			},
			Weight:   0.25,
			Severity: ThreatLevelCritical,
		},
		{
			ID:             "rapid_mass_modify",
			Name:           "快速批量修改",
			Description:    "检测极短时间内大量文件被修改的异常行为模式",
			Enabled:        true,
			MinEntropy:     6.0,
			BatchThreshold: 30,
			TimeWindowSec:  60,
			SuspiciousExts: []string{},
			Weight:         0.20,
			Severity:       ThreatLevelHigh,
		},
		{
			ID:             "ransom_note_creation",
			Name:           "勒索信创建检测",
			Description:    "检测目录中出现已知勒索信文件名模式",
			Enabled:        true,
			MinEntropy:     0,
			BatchThreshold: 1,
			TimeWindowSec:  300,
			SuspiciousExts: []string{},
			Weight:         0.10,
			Severity:       ThreatLevelCritical,
		},
		{
			ID:             "shadow_copy_deletion",
			Name:           "卷影副本删除检测",
			Description:    "检测对系统备份/快照的可疑删除行为",
			Enabled:        true,
			MinEntropy:     0,
			BatchThreshold: 3,
			TimeWindowSec:  120,
			SuspiciousExts: []string{},
			Weight:         0.10,
			Severity:       ThreatLevelHigh,
		},
	}
}

// Analyze 对文件变更事件进行 AI 行为分析
// 返回综合分析结果，包含各维度得分和最终威胁判定
func (d *AIBehaviorDetector) Analyze(events []*FileChangeEvent, stats *DetectionStats) *AIAnalysisResult {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := &AIAnalysisResult{
		PatternMatches: make([]PatternMatch, 0),
		Indicators:     make([]BehaviorIndicator, 0),
	}

	if len(events) == 0 {
		return result
	}

	// ============================================================
	// 1. 熵值异常分析
	// ============================================================
	result.EntropyScore = d.analyzeEntropy(events)

	// ============================================================
	// 2. 批量重命名分析
	// ============================================================
	result.BatchRenameScore = d.analyzeBatchRename(events)

	// ============================================================
	// 3. 文件变更速率分析
	// ============================================================
	result.FileChangeScore = d.analyzeFileChangeRate(events)

	// ============================================================
	// 4. 蜜罐触发分析
	// ============================================================
	result.DecoyTriggerScore = d.analyzeDecoyTriggers(events)

	// ============================================================
	// 5. 扩展名篡改分析
	// ============================================================
	result.ExtChangeScore = d.analyzeExtensionChanges(events)

	// ============================================================
	// 6. 模式匹配
	// ============================================================
	result.PatternMatches = d.matchPatterns(events)

	// ============================================================
	// 7. 综合评分
	// ============================================================
	result.OverallScore = result.EntropyScore*WeightEntropyChange +
		result.BatchRenameScore*WeightBatchRename +
		result.FileChangeScore*WeightFileChangeRate +
		result.DecoyTriggerScore*WeightDecoyTrigger +
		result.ExtChangeScore*WeightExtensionChange

	// 如果有高严重性模式匹配，提升分数
	for _, pm := range result.PatternMatches {
		if pm.Matched {
			result.OverallScore = math.Min(1.0, result.OverallScore+0.1)
		}
	}

	// 确定威胁级别
	result.ThreatLevel = d.determineThreatLevel(result.OverallScore)

	// 构建行为指标
	result.Indicators = d.buildIndicators(events, result)

	return result
}

// analyzeEntropy 分析事件序列中的熵值变化
// 检测文件内容从正常文本（低熵）变为加密数据（高熵）的异常模式
func (d *AIBehaviorDetector) analyzeEntropy(events []*FileChangeEvent) float64 {
	if len(events) == 0 {
		return 0
	}

	highEntropyCount := 0
	entropyJumps := 0 // 熵值急剧升高的次数

	for _, e := range events {
		// 熵值超过阈值
		if e.EntropyAfter > d.config.EntropyThreshold {
			highEntropyCount++
		}
		// 熵值突然升高超过 2.0（从正常文本到加密数据）
		if e.EntropyAfter-e.EntropyBefore > 2.0 {
			entropyJumps++
		}
	}

	// 计算得分：基于高熵文件比例和熵值跳跃次数
	ratio := float64(highEntropyCount) / float64(len(events))
	jumpRatio := float64(entropyJumps) / float64(len(events))

	score := ratio*0.6 + jumpRatio*0.4
	return math.Min(1.0, score)
}

// analyzeBatchRename 分析批量重命名行为
// 检测短时间内大量文件被重命名为相同或相似模式
func (d *AIBehaviorDetector) analyzeBatchRename(events []*FileChangeEvent) float64 {
	renameEvents := make([]*FileChangeEvent, 0)
	for _, e := range events {
		if e.EventType == "rename" {
			renameEvents = append(renameEvents, e)
		}
	}

	if len(renameEvents) == 0 {
		return 0
	}

	// 统计目标扩展名分布
	extCount := make(map[string]int)
	for _, e := range renameEvents {
		ext := strings.ToLower(filepath.Ext(e.FilePath))
		if ext != "" {
			extCount[ext]++
		}
	}

	// 检查是否有可疑的单一扩展名占主导
	maxCount := 0
	for _, count := range extCount {
		if count > maxCount {
			maxCount = count
		}
	}

	// 检查是否使用了已知勒索扩展名
	suspiciousExtHits := 0
	for ext := range extCount {
		if d.isSuspiciousExtension(ext) {
			suspiciousExtHits += extCount[ext]
		}
	}

	// 综合得分
	batchScore := math.Min(1.0, float64(len(renameEvents))/float64(d.config.BatchThreshold))
	extScore := 0.0
	if len(renameEvents) > 0 {
		extScore = float64(suspiciousExtHits) / float64(len(renameEvents))
	}

	return batchScore*0.6 + extScore*0.4
}

// analyzeFileChangeRate 分析文件变更速率
// 检测远超正常水平的文件修改频率
func (d *AIBehaviorDetector) analyzeFileChangeRate(events []*FileChangeEvent) float64 {
	if len(events) < 2 {
		return 0
	}

	// 计算事件时间跨度
	first := events[0].Timestamp
	last := events[len(events)-1].Timestamp
	duration := last.Sub(first).Seconds()
	if duration <= 0 {
		duration = 1
	}

	// 每分钟文件变更数
	changeRate := float64(len(events)) / (duration / 60.0)

	// 得分：超过阈值的部分按比例计算
	threshold := float64(d.config.BatchThreshold)
	if changeRate <= threshold*0.5 {
		return 0
	}

	score := (changeRate - threshold*0.5) / (threshold * 2)
	return math.Min(1.0, score)
}

// analyzeDecoyTriggers 分析蜜罐触发情况
// 诱饵文件被访问/修改是最直接的勒索软件活动指标
func (d *AIBehaviorDetector) analyzeDecoyTriggers(events []*FileChangeEvent) float64 {
	decoyHits := 0
	for _, e := range events {
		if e.IsDecoy {
			decoyHits++
		}
	}

	if decoyHits == 0 {
		return 0
	}

	// 任何蜜罐触发都值得高度关注
	// 1 个蜜罐触发 = 0.5 基础分，每多一个 +0.15，上限 1.0
	score := 0.5 + float64(decoyHits-1)*0.15
	return math.Min(1.0, score)
}

// analyzeExtensionChanges 分析文件扩展名篡改
func (d *AIBehaviorDetector) analyzeExtensionChanges(events []*FileChangeEvent) float64 {
	extChanges := 0
	for _, e := range events {
		if e.EventType == "rename" && e.OldPath != "" {
			oldExt := strings.ToLower(filepath.Ext(e.OldPath))
			newExt := strings.ToLower(filepath.Ext(e.FilePath))
			if oldExt != newExt {
				extChanges++
			}
		}
	}

	if extChanges == 0 {
		return 0
	}

	return math.Min(1.0, float64(extChanges)/float64(d.config.BatchThreshold))
}

// matchPatterns 匹配威胁检测模式库
func (d *AIBehaviorDetector) matchPatterns(events []*FileChangeEvent) []PatternMatch {
	matches := make([]PatternMatch, 0, len(d.patterns))

	for _, pattern := range d.patterns {
		if !pattern.Enabled {
			continue
		}

		match := PatternMatch{
			PatternID:   pattern.ID,
			PatternName: pattern.Name,
		}

		// 计算模式匹配得分
		matchScore := 0.0

		// 检查批量操作阈值
		eventsInWindow := d.countEventsInWindow(events, pattern.TimeWindowSec)
		if eventsInWindow >= pattern.BatchThreshold {
			matchScore += 0.4
		}

		// 检查可疑扩展名
		if len(pattern.SuspiciousExts) > 0 {
			suspiciousHits := d.countSuspiciousExtensions(events, pattern.SuspiciousExts)
			if suspiciousHits > 0 {
				matchScore += 0.3 * math.Min(1.0, float64(suspiciousHits)/float64(pattern.BatchThreshold))
			}
		}

		// 检查熵值
		if pattern.MinEntropy > 0 {
			highEntropyCount := 0
			for _, e := range events {
				if e.EntropyAfter >= pattern.MinEntropy {
					highEntropyCount++
				}
			}
			if highEntropyCount > 0 {
				matchScore += 0.3 * math.Min(1.0, float64(highEntropyCount)/float64(len(events)))
			}
		}

		// 检查勒索信模式（特殊处理）
		if pattern.ID == "ransom_note_creation" {
			ransomNoteHits := d.detectRansomNotes(events)
			if ransomNoteHits > 0 {
				matchScore = 0.9
			}
		}

		match.MatchScore = matchScore
		match.Matched = matchScore >= 0.5
		matches = append(matches, match)
	}

	return matches
}

// determineThreatLevel 根据综合得分确定威胁级别
func (d *AIBehaviorDetector) determineThreatLevel(score float64) int {
	switch {
	case score >= 0.85:
		return ThreatLevelCritical
	case score >= 0.65:
		return ThreatLevelHigh
	case score >= 0.40:
		return ThreatLevelMedium
	case score >= 0.20:
		return ThreatLevelLow
	default:
		return ThreatLevelNone
	}
}

// buildIndicators 构建行为指标列表
func (d *AIBehaviorDetector) buildIndicators(events []*FileChangeEvent, result *AIAnalysisResult) []BehaviorIndicator {
	indicators := make([]BehaviorIndicator, 0)

	// 熵值指标
	indicators = append(indicators, BehaviorIndicator{
		Name:        "文件熵值异常",
		Description: "检测到文件内容熵值异常升高，可能表示加密操作",
		Value:       result.EntropyScore,
		Threshold:   0.3,
		Exceeded:    result.EntropyScore > 0.3,
		Weight:      WeightEntropyChange,
	})

	// 批量重命名指标
	indicators = append(indicators, BehaviorIndicator{
		Name:        "批量重命名活动",
		Description: "检测到大量文件被重命名为可疑扩展名",
		Value:       result.BatchRenameScore,
		Threshold:   0.3,
		Exceeded:    result.BatchRenameScore > 0.3,
		Weight:      WeightBatchRename,
	})

	// 文件变更速率指标
	indicators = append(indicators, BehaviorIndicator{
		Name:        "文件变更速率",
		Description: "文件修改频率超过正常范围",
		Value:       result.FileChangeScore,
		Threshold:   0.3,
		Exceeded:    result.FileChangeScore > 0.3,
		Weight:      WeightFileChangeRate,
	})

	// 蜜罐触发指标
	indicators = append(indicators, BehaviorIndicator{
		Name:        "蜜罐触发",
		Description: "检测到诱饵文件被访问或修改",
		Value:       result.DecoyTriggerScore,
		Threshold:   0.0, // 任何蜜罐触发都是异常
		Exceeded:    result.DecoyTriggerScore > 0,
		Weight:      WeightDecoyTrigger,
	})

	// 扩展名篡改指标
	indicators = append(indicators, BehaviorIndicator{
		Name:        "扩展名篡改",
		Description: "检测到文件扩展名被批量修改",
		Value:       result.ExtChangeScore,
		Threshold:   0.2,
		Exceeded:    result.ExtChangeScore > 0.2,
		Weight:      WeightExtensionChange,
	})

	// 事件计数指标
	indicators = append(indicators, BehaviorIndicator{
		Name:        "异常事件总量",
		Description: "时间窗口内的文件变更事件总数",
		Value:       len(events),
		Threshold:   d.config.BatchThreshold,
		Exceeded:    len(events) > d.config.BatchThreshold,
		Weight:      0.05,
	})

	return indicators
}

// ============================================================
// 辅助分析方法
// ============================================================

// countEventsInWindow 统计指定时间窗口内的事件数
func (d *AIBehaviorDetector) countEventsInWindow(events []*FileChangeEvent, windowSec int) int {
	if len(events) == 0 {
		return 0
	}

	cutoff := time.Now().Add(-time.Duration(windowSec) * time.Second)
	count := 0
	for _, e := range events {
		if e.Timestamp.After(cutoff) {
			count++
		}
	}
	return count
}

// countSuspiciousExtensions 统计可疑扩展名出现次数
func (d *AIBehaviorDetector) countSuspiciousExtensions(events []*FileChangeEvent, suspiciousExts []string) int {
	count := 0
	for _, e := range events {
		ext := strings.ToLower(filepath.Ext(e.FilePath))
		for _, s := range suspiciousExts {
			if strings.EqualFold(ext, s) {
				count++
				break
			}
		}
	}
	return count
}

// isSuspiciousExtension 检查扩展名是否属于已知勒索软件后缀
func (d *AIBehaviorDetector) isSuspiciousExtension(ext string) bool {
	// 综合已知勒索软件扩展名列表
	knownRansomExts := map[string]bool{
		".encrypted": true, ".locked": true, ".crypto": true,
		".crypt": true, ".enc": true, ".crypted": true,
		".wncry": true, ".locky": true, ".zepto": true,
		".thor": true, ".aaa": true, ".abc": true,
		".xyz": true, ".zzz": true, ".micro": true,
		".xxx": true, ".ttt": true, ".ecc": true,
		".ezz": true, ".cerber": true, ".odin": true,
		".aesir": true, ".osiris": true, ".shino": true,
	}
	return knownRansomExts[strings.ToLower(ext)]
}

// detectRansomNotes 检测勒索信文件名模式
func (d *AIBehaviorDetector) detectRansomNotes(events []*FileChangeEvent) int {
	// 常见勒索信文件名关键词
	ransomNotePatterns := []string{
		"readme", "decrypt", "how_to", "recover", "restore",
		"ransom", "payment", "bitcoin", "wallet", "tor",
		"your_files", "important", "help_restore", "fix_your",
		"how_to_decrypt", "how_to_recover", "read_me",
	}

	count := 0
	for _, e := range events {
		baseName := strings.ToLower(filepath.Base(e.FilePath))
		for _, pattern := range ransomNotePatterns {
			if strings.Contains(baseName, pattern) {
				count++
				break
			}
		}
	}
	return count
}

// AddPattern 添加自定义威胁检测模式
func (d *AIBehaviorDetector) AddPattern(pattern *ThreatPattern) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if pattern.ID == "" {
		return fmt.Errorf("模式ID不能为空")
	}

	// 检查是否已存在
	for _, p := range d.patterns {
		if p.ID == pattern.ID {
			return fmt.Errorf("模式已存在: %s", pattern.ID)
		}
	}

	d.patterns = append(d.patterns, pattern)
	return nil
}

// RemovePattern 移除威胁检测模式
func (d *AIBehaviorDetector) RemovePattern(patternID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	for i, p := range d.patterns {
		if p.ID == patternID {
			d.patterns = append(d.patterns[:i], d.patterns[i+1:]...)
			return nil
		}
	}
	return ErrPatternNotFound
}

// ListPatterns 列出所有威胁检测模式
func (d *AIBehaviorDetector) ListPatterns() []*ThreatPattern {
	d.mu.RLock()
	defer d.mu.RUnlock()

	result := make([]*ThreatPattern, len(d.patterns))
	copy(result, d.patterns)
	return result
}
