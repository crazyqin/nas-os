// Package ransomware_defense 提供勒索软件防护模块
// detector.go - 检测引擎（行为分析 + 加密签名识别 + 快照对比 + 威胁评分）
package ransomware_defense

import (
	"fmt"
	"log"
	"math"
	"strings"
	"sync"
	"time"
)

// =============================================================================
// DetectionEngine 检测引擎
// =============================================================================

// DetectionEngine 检测引擎
type DetectionEngine struct {
	mu           sync.RWMutex
	config       DefenseConfig
	patterns     []BehaviorPattern
	signatures   []EncryptionSignature
	honeypotMgr  *HoneypotManager
	snapshotMgr  SnapshotManager
}

// NewDetectionEngine 创建新的检测引擎
func NewDetectionEngine(config DefenseConfig, honeypotMgr *HoneypotManager, snapshotMgr SnapshotManager) *DetectionEngine {
	engine := &DetectionEngine{
		config:      config,
		honeypotMgr: honeypotMgr,
		snapshotMgr: snapshotMgr,
		patterns:    getDefaultBehaviorPatterns(),
		signatures:  getDefaultEncryptionSignatures(),
	}
	return engine
}

// getDefaultBehaviorPatterns 获取默认行为模式
func getDefaultBehaviorPatterns() []BehaviorPattern {
	return []BehaviorPattern{
		{
			ID:          "rapid-encryption",
			Name:        "快速加密行为",
			Description: "短时间内大量文件被修改且熵值显著增加，典型勒索软件加密特征",
			Severity:    ThreatLevelCritical,
			Weight:      90,
			Indicators: []PatternIndicator{
				{Type: "file_modify_rate", Threshold: 50, TimeWindowSec: 60},
				{Type: "entropy_spike", Threshold: 7.5, TimeWindowSec: 60},
			},
		},
		{
			ID:          "mass-rename-extension",
			Name:        "批量重命名扩展名",
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
			Name:        "快速批量删除",
			Description: "短时间内大量文件被删除，可能为勒索软件清除原文件",
			Severity:    ThreatLevelHigh,
			Weight:      70,
			Indicators: []PatternIndicator{
				{Type: "file_delete_rate", Threshold: 100, TimeWindowSec: 60},
			},
		},
		{
			ID:          "ransom-note-drop",
			Name:        "勒索信投放",
			Description: "检测到勒索信文件特征",
			Severity:    ThreatLevelCritical,
			Weight:      95,
			Indicators: []PatternIndicator{
				{Type: "ransom_note_detected", Threshold: 1, TimeWindowSec: 120},
			},
		},
		{
			ID:          "single-source-burst",
			Name:        "单源突发操作",
			Description: "单一用户/IP大量文件操作",
			Severity:    ThreatLevelMedium,
			Weight:      60,
			Indicators: []PatternIndicator{
				{Type: "single_source_rate", Threshold: 80, TimeWindowSec: 60},
			},
		},
		{
			ID:          "cross-share-scan",
			Name:        "跨共享扫描",
			Description: "短时间内访问多个共享目录",
			Severity:    ThreatLevelMedium,
			Weight:      55,
			Indicators: []PatternIndicator{
				{Type: "unique_share_access", Threshold: 3, TimeWindowSec: 120},
			},
		},
	}
}

// getDefaultEncryptionSignatures 获取默认加密签名特征
func getDefaultEncryptionSignatures() []EncryptionSignature {
	return []EncryptionSignature{
		{
			ID:          "known-ransom-extensions",
			Name:        "已知勒索扩展名",
			Description: "已知勒索软件使用的文件扩展名",
			KnownExtensions: []string{
				".encrypted", ".locked", ".crypto", ".crypted",
				".cry", ".wnry", ".wncry", ".wcry",
				".locky", ".cerber", ".zepto", ".odin",
				".thor", ".aesir", ".zzzzz", ".micro",
				".aaa", ".abc", ".ccc", ".xyz",
				".crypt", ".ezz", ".ecc", ".exx",
			},
			EntropyRange: [2]float64{7.0, 8.0},
			Severity:     ThreatLevelCritical,
		},
		{
			ID:          "high-entropy-modification",
			Name:        "高熵值修改",
			Description: "文件修改后熵值显著增加，疑似加密操作",
			EntropyRange: [2]float64{7.5, 8.0},
			Severity:     ThreatLevelHigh,
		},
		{
			ID:          "ransom-note-patterns",
			Name:        "勒索信特征",
			Description: "文件内容匹配勒索信特征字符串",
			RansomNotePatterns: []string{
				"your files have been encrypted",
				"your files are encrypted",
				"pay the ransom",
				"bitcoin wallet",
				"decrypt your files",
				"your important files",
				"all your files have been",
				"what happened to your computer",
				"restore your files",
				"private key",
				"decryption key",
				"ransom",
				"YOUR FILES ARE ENCRYPTED",
				"DECRYPT YOUR FILES",
				"SEND BITCOIN",
			},
			Severity: ThreatLevelCritical,
		},
	}
}

// =============================================================================
// 综合检测方法
// =============================================================================

// AnalyzeActivities 综合分析文件活动，返回威胁事件列表
func (de *DetectionEngine) AnalyzeActivities(activities []FileActivity) []ThreatEvent {
	if len(activities) == 0 {
		return nil
	}

	var allThreats []ThreatEvent

	// 1. 行为模式检测
	behaviorThreats := de.detectBehaviorPatterns(activities)
	allThreats = append(allThreats, behaviorThreats...)

	// 2. 加密签名检测
	encryptionThreats := de.detectEncryptionSignatures(activities)
	allThreats = append(allThreats, encryptionThreats...)

	// 3. 蜜罐检测
	honeypotThreats := de.detectHoneypotTriggers(activities)
	allThreats = append(allThreats, honeypotThreats...)

	// 4. 综合威胁评分
	for i := range allThreats {
		score := de.CalculateThreatScore(allThreats[i], activities)
		allThreats[i].Score = &score
		allThreats[i].ThreatLevel = score.Level
	}

	return allThreats
}

// =============================================================================
// 行为模式检测
// =============================================================================

// detectBehaviorPatterns 检测行为模式
func (de *DetectionEngine) detectBehaviorPatterns(activities []FileActivity) []ThreatEvent {
	de.mu.RLock()
	patterns := de.patterns
	de.mu.RUnlock()

	stats := de.calculateActivityStats(activities)
	var threats []ThreatEvent

	for _, pattern := range patterns {
		threat := de.evaluatePattern(pattern, activities, stats)
		if threat != nil {
			threats = append(threats, *threat)
		}
	}

	return threats
}

// activityStats 活动统计
type activityStats struct {
	totalCount        int
	modifyCount       int
	deleteCount       int
	renameCount       int
	createCount       int
	readCount         int
	suspExtCount      int
	extChangeCount    int
	highEntropyCount  int
	avgEntropy        float64
	maxEntropy        float64
	minEntropy        float64
	processCounts     map[string]int
	userCounts        map[string]int
	ipCounts          map[string]int
	shareCounts       map[string]int
	uniqueShares      map[string]bool
	affectedFiles     []string
}

// calculateActivityStats 计算活动统计
func (de *DetectionEngine) calculateActivityStats(activities []FileActivity) activityStats {
	stats := activityStats{
		processCounts: make(map[string]int),
		userCounts:    make(map[string]int),
		ipCounts:      make(map[string]int),
		shareCounts:   make(map[string]int),
		uniqueShares:  make(map[string]bool),
		minEntropy:    8.0,
	}

	fileSet := make(map[string]bool)
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
		case FileOpRead:
			stats.readCount++
		}

		// 检查可疑扩展名
		if de.isSuspiciousExtension(a.Path) {
			stats.suspExtCount++
		}

		// 检查扩展名变更
		if a.Operation == FileOpRename && a.OldPath != "" {
			oldExt := getFileExtension(a.OldPath)
			newExt := getFileExtension(a.Path)
			if oldExt != newExt {
				stats.extChangeCount++
			}
		}

		// 熵值统计
		if a.Entropy >= de.config.EntropyThreshold {
			stats.highEntropyCount++
		}
		if a.Entropy > stats.maxEntropy {
			stats.maxEntropy = a.Entropy
		}
		if a.Entropy < stats.minEntropy && a.Entropy > 0 {
			stats.minEntropy = a.Entropy
		}
		totalEntropy += a.Entropy

		// 进程/用户/IP统计
		if a.ProcessName != "" {
			stats.processCounts[a.ProcessName]++
		}
		if a.SourceUser != "" {
			stats.userCounts[a.SourceUser]++
		}
		if a.SourceIP != "" {
			stats.ipCounts[a.SourceIP]++
		}
		if a.ShareName != "" {
			stats.shareCounts[a.ShareName]++
			stats.uniqueShares[a.ShareName] = true
		}

		fileSet[a.Path] = true
	}

	if stats.totalCount > 0 {
		stats.avgEntropy = totalEntropy / float64(stats.totalCount)
	}

	stats.affectedFiles = make([]string, 0, len(fileSet))
	for f := range fileSet {
		stats.affectedFiles = append(stats.affectedFiles, f)
	}

	return stats
}

// evaluatePattern 评估行为模式
func (de *DetectionEngine) evaluatePattern(pattern BehaviorPattern, activities []FileActivity, stats activityStats) *ThreatEvent {
	score := 0
	matchedIndicators := 0

	for _, indicator := range pattern.Indicators {
		matched := false
		switch indicator.Type {
		case "file_modify_rate":
			matched = float64(stats.modifyCount) >= indicator.Threshold
		case "file_delete_rate":
			matched = float64(stats.deleteCount) >= indicator.Threshold
		case "file_rename_rate":
			matched = float64(stats.renameCount) >= indicator.Threshold
		case "file_create_rate":
			matched = float64(stats.createCount) >= indicator.Threshold
		case "entropy_spike":
			matched = stats.maxEntropy >= indicator.Threshold || stats.highEntropyCount >= 10
		case "suspicious_extension":
			matched = float64(stats.suspExtCount) >= indicator.Threshold
		case "extension_change_rate":
			matched = float64(stats.extChangeCount) >= indicator.Threshold
		case "single_source_rate":
			for _, count := range stats.ipCounts {
				if float64(count) >= indicator.Threshold {
					matched = true
					break
				}
			}
		case "unique_share_access":
			matched = float64(len(stats.uniqueShares)) >= indicator.Threshold
		case "ransom_note_detected":
			for _, a := range activities {
				if de.containsRansomNotePattern(a.Path) {
					matched = true
					break
				}
			}
		}

		if matched {
			matchedIndicators++
			score += pattern.Weight / len(pattern.Indicators)
		}
	}

	// 所有指标都匹配才触发
	if matchedIndicators == 0 {
		return nil
	}

	level := de.scoreToThreatLevel(score)
	action := de.decideAction(level)
	processName, processID, userID, sourceIP := de.identifyPrimarySource(stats)

	return &ThreatEvent{
		ID:            fmt.Sprintf("threat-%d", time.Now().UnixNano()),
		ThreatLevel:   level,
		DetectionMethod: "behavior_pattern",
		Description:   fmt.Sprintf("[%s] %s", pattern.ID, pattern.Description),
		SourceIP:      sourceIP,
		SourceUser:    userID,
		ProcessName:   processName,
		ProcessID:     processID,
		AffectedFiles: stats.affectedFiles,
		Action:        action,
		Timestamp:     time.Now(),
		Details: map[string]interface{}{
			"pattern_id":       pattern.ID,
			"pattern_name":     pattern.Name,
			"matched":          matchedIndicators,
			"total_indicators": len(pattern.Indicators),
			"score":            score,
			"total_activities": stats.totalCount,
			"modify_count":     stats.modifyCount,
			"delete_count":     stats.deleteCount,
			"rename_count":     stats.renameCount,
			"avg_entropy":      stats.avgEntropy,
			"max_entropy":      stats.maxEntropy,
		},
	}
}

// =============================================================================
// 加密签名检测
// =============================================================================

// detectEncryptionSignatures 检测加密签名
func (de *DetectionEngine) detectEncryptionSignatures(activities []FileActivity) []ThreatEvent {
	de.mu.RLock()
	signatures := de.signatures
	de.mu.RUnlock()

	var threats []ThreatEvent

	// 按来源分组
	sourceGroups := groupActivitiesBySource(activities)

	for sourceKey, group := range sourceGroups {
		matchedSignatures := de.matchSignatures(group, signatures)
		if len(matchedSignatures) == 0 {
			continue
		}

		// 计算得分
		maxSeverity := ThreatLevelNone
		sigNames := make([]string, 0, len(matchedSignatures))
		for _, sig := range matchedSignatures {
			if sig.Severity > maxSeverity {
				maxSeverity = sig.Severity
			}
			sigNames = append(sigNames, sig.Name)
		}

		affectedFiles := make([]string, 0, len(group))
		for _, a := range group {
			affectedFiles = append(affectedFiles, a.Path)
		}

		// 从sourceKey解析IP和用户
		parts := strings.SplitN(sourceKey, "|", 2)
		sourceIP := parts[0]
		sourceUser := ""
		if len(parts) > 1 {
			sourceUser = parts[1]
		}

		threat := ThreatEvent{
			ID:              fmt.Sprintf("threat-enc-%d", time.Now().UnixNano()),
			ThreatLevel:     maxSeverity,
			DetectionMethod: "encryption_signature",
			Description:     fmt.Sprintf("检测到加密签名特征: %s", strings.Join(sigNames, ", ")),
			SourceIP:        sourceIP,
			SourceUser:      sourceUser,
			AffectedFiles:   affectedFiles,
			Action:          de.decideAction(maxSeverity),
			Timestamp:       time.Now(),
			Details: map[string]interface{}{
				"matched_signatures": sigNames,
				"file_count":         len(group),
			},
		}

		threats = append(threats, threat)
	}

	return threats
}

// matchSignatures 匹配加密签名
func (de *DetectionEngine) matchSignatures(activities []FileActivity, signatures []EncryptionSignature) []EncryptionSignature {
	var matched []EncryptionSignature

	for _, sig := range signatures {
		matchedCount := 0
		for _, a := range activities {
			if de.matchesSignature(a, sig) {
				matchedCount++
			}
		}
		// 至少匹配一定比例的文件才认为有效
		if matchedCount > 0 && float64(matchedCount) >= float64(len(activities))*0.3 {
			matched = append(matched, sig)
		}
	}

	return matched
}

// matchesSignature 检查活动是否匹配签名
func (de *DetectionEngine) matchesSignature(activity FileActivity, sig EncryptionSignature) bool {
	// 检查扩展名
	if len(sig.KnownExtensions) > 0 {
		ext := getFileExtension(activity.Path)
		for _, knownExt := range sig.KnownExtensions {
			if strings.EqualFold(ext, knownExt) {
				return true
			}
		}
	}

	// 检查熵值范围
	if sig.EntropyRange[0] > 0 && activity.Entropy >= sig.EntropyRange[0] && activity.Entropy <= sig.EntropyRange[1] {
		return true
	}

	return false
}

// containsRansomNotePattern 检查文件是否包含勒索信特征
func (de *DetectionEngine) containsRansomNotePattern(path string) bool {
	ext := strings.ToLower(getFileExtension(path))
	// 勒索信通常是 txt/html/htm/url 文件
	if ext != ".txt" && ext != ".html" && ext != ".htm" && ext != ".url" {
		return false
	}

	// 检查文件名特征
	lowerName := strings.ToLower(path)
	ransomNames := []string{
		"readme", "how_to", "decrypt", "recover", "restore",
		"how_to_decrypt", "how_to_recover", "your_files",
		"_readme", "!readme", "read_me", "instructions",
	}
	for _, name := range ransomNames {
		if strings.Contains(lowerName, name) {
			return true
		}
	}

	return false
}

// =============================================================================
// 蜜罐检测
// =============================================================================

// detectHoneypotTriggers 检测蜜罐触发
func (de *DetectionEngine) detectHoneypotTriggers(activities []FileActivity) []ThreatEvent {
	if de.honeypotMgr == nil {
		return nil
	}

	var threats []ThreatEvent

	for _, a := range activities {
		if hp := de.honeypotMgr.CheckActivity(a); hp != nil {
			threat := ThreatEvent{
				ID:                fmt.Sprintf("threat-hp-%d", time.Now().UnixNano()),
				ThreatLevel:       ThreatLevelCritical,
				DetectionMethod:   "honeypot",
				Description:       fmt.Sprintf("蜜罐文件被触发: %s (类型: %s)", hp.FileName, hp.FileType),
				SourceIP:          a.SourceIP,
				SourceUser:        a.SourceUser,
				ProcessName:       a.ProcessName,
				ProcessID:         a.ProcessID,
				AffectedShare:     hp.ShareName,
				Protocol:          a.Protocol,
				TriggeredHoneypot: hp.ID,
				AffectedFiles:     []string{hp.Path},
				Action:            ActionDisableShare,
				Timestamp:         time.Now(),
				Details: map[string]interface{}{
					"honeypot_id":   hp.ID,
					"honeypot_type": hp.FileType,
					"file_name":     hp.FileName,
					"trigger_count": hp.TriggerCount,
				},
			}
			threats = append(threats, threat)
		}
	}

	return threats
}

// =============================================================================
// 快照对比检测
// =============================================================================

// DetectSnapshotAnomalies 通过快照对比检测异常
func (de *DetectionEngine) DetectSnapshotAnomalies(dataset string) (*ThreatEvent, *SnapshotDelta, error) {
	if de.snapshotMgr == nil {
		return nil, nil, fmt.Errorf("快照管理器未配置")
	}

	snapshots, err := de.snapshotMgr.ListSnapshots(dataset)
	if err != nil {
		return nil, nil, fmt.Errorf("获取快照列表失败: %w", err)
	}

	if len(snapshots) < 2 {
		return nil, nil, nil
	}

	// 取最近两个快照对比
	latest := snapshots[len(snapshots)-1]
	previous := snapshots[len(snapshots)-2]

	delta, err := de.snapshotMgr.CompareSnapshots(previous.ID, latest.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("快照对比失败: %w", err)
	}

	// 判断是否存在异常
	anomalyScore := de.evaluateSnapshotDelta(delta)

	if anomalyScore < 30 {
		return nil, delta, nil
	}

	level := de.scoreToThreatLevel(anomalyScore)
	threat := &ThreatEvent{
		ID:                fmt.Sprintf("threat-snap-%d", time.Now().UnixNano()),
		ThreatLevel:       level,
		DetectionMethod:   "snapshot_comparison",
		Description:       fmt.Sprintf("快照对比发现异常: 删除%d文件, 修改%d文件, %d文件熵值增加, %d扩展名变更",
			delta.DeletedFiles, delta.ModifiedFiles, delta.EntropyIncrease, delta.SuspiciousExtensions),
		SnapshotDelta:     delta,
		AffectedShare:     dataset,
		Action:            de.decideAction(level),
		Timestamp:         time.Now(),
		Details: map[string]interface{}{
			"snapshot_from": previous.ID,
			"snapshot_to":   latest.ID,
			"anomaly_score": anomalyScore,
		},
	}

	return threat, delta, nil
}

// evaluateSnapshotDelta 评估快照差异的异常程度
func (de *DetectionEngine) evaluateSnapshotDelta(delta *SnapshotDelta) int {
	score := 0

	// 删除大量文件
	if delta.DeletedFiles > 100 {
		score += 30
	} else if delta.DeletedFiles > 50 {
		score += 20
	} else if delta.DeletedFiles > 10 {
		score += 10
	}

	// 大量修改
	if delta.ModifiedFiles > 200 {
		score += 25
	} else if delta.ModifiedFiles > 100 {
		score += 15
	} else if delta.ModifiedFiles > 30 {
		score += 10
	}

	// 熵值增加
	if delta.EntropyIncrease > 100 {
		score += 25
	} else if delta.EntropyIncrease > 50 {
		score += 15
	} else if delta.EntropyIncrease > 10 {
		score += 10
	}

	// 可疑扩展名变更
	if delta.SuspiciousExtensions > 50 {
		score += 20
	} else if delta.SuspiciousExtensions > 20 {
		score += 15
	} else if delta.SuspiciousExtensions > 5 {
		score += 10
	}

	// 大量重命名
	if delta.RenamedFiles > 50 {
		score += 15
	} else if delta.RenamedFiles > 20 {
		score += 10
	}

	return min(score, 100)
}

// =============================================================================
// 威胁评分系统
// =============================================================================

// CalculateThreatScore 计算综合威胁评分
func (de *DetectionEngine) CalculateThreatScore(threat ThreatEvent, activities []FileActivity) ThreatScore {
	factors := make([]ScoreFactor, 0, 5)

	// 因子1: 蜜罐触发 (权重: 30)
	honeypotScore := 0
	if threat.TriggeredHoneypot != "" {
		honeypotScore = 30
	}
	factors = append(factors, ScoreFactor{
		Name:        "honeypot",
		Score:       honeypotScore,
		MaxScore:    30,
		Weight:      0.30,
		Description: "蜜罐文件触发检测",
	})

	// 因子2: 行为分析 (权重: 25)
	behaviorScore := de.calculateBehaviorScore(activities)
	factors = append(factors, ScoreFactor{
		Name:        "behavior",
		Score:       behaviorScore,
		MaxScore:    25,
		Weight:      0.25,
		Description: "文件行为模式分析",
	})

	// 因子3: 加密签名 (权重: 25)
	encryptionScore := de.calculateEncryptionScore(threat)
	factors = append(factors, ScoreFactor{
		Name:        "encryption",
		Score:       encryptionScore,
		MaxScore:    25,
		Weight:      0.25,
		Description: "加密签名特征匹配",
	})

	// 因子4: 快照对比 (权重: 10)
	snapshotScore := 0
	if threat.SnapshotDelta != nil {
		snapshotScore = de.evaluateSnapshotDelta(threat.SnapshotDelta) / 10
	}
	factors = append(factors, ScoreFactor{
		Name:        "snapshot_delta",
		Score:       snapshotScore,
		MaxScore:    10,
		Weight:      0.10,
		Description: "快照对比异常检测",
	})

	// 因子5: 操作速率 (权重: 10)
	rateScore := de.calculateRateScore(activities)
	factors = append(factors, ScoreFactor{
		Name:        "rate",
		Score:       rateScore,
		MaxScore:    10,
		Weight:      0.10,
		Description: "文件操作速率分析",
	})

	// 计算综合得分
	overall := honeypotScore + behaviorScore + encryptionScore + snapshotScore + rateScore
	overall = min(overall, 100)

	return ThreatScore{
		OverallScore:       overall,
		HoneypotScore:      honeypotScore,
		BehaviorScore:      behaviorScore,
		EncryptionScore:    encryptionScore,
		SnapshotDeltaScore: snapshotScore,
		RateScore:          rateScore,
		Level:              de.scoreToThreatLevel(overall),
		Factors:            factors,
	}
}

// calculateBehaviorScore 计算行为分析得分
func (de *DetectionEngine) calculateBehaviorScore(activities []FileActivity) int {
	if len(activities) == 0 {
		return 0
	}

	score := 0
	stats := de.calculateActivityStats(activities)

	// 高熵值文件占比
	highEntropyRatio := float64(stats.highEntropyCount) / float64(stats.totalCount)
	if highEntropyRatio > 0.7 {
		score += 15
	} else if highEntropyRatio > 0.4 {
		score += 10
	} else if highEntropyRatio > 0.2 {
		score += 5
	}

	// 修改+删除操作占比
	modifyDeleteRatio := float64(stats.modifyCount+stats.deleteCount) / float64(stats.totalCount)
	if modifyDeleteRatio > 0.8 {
		score += 10
	} else if modifyDeleteRatio > 0.5 {
		score += 5
	}

	return min(score, 25)
}

// calculateEncryptionScore 计算加密签名得分
func (de *DetectionEngine) calculateEncryptionScore(threat ThreatEvent) int {
	if threat.DetectionMethod == "encryption_signature" {
		return 25
	}

	// 检查可疑扩展名
	if threat.Details != nil {
		if sigs, ok := threat.Details["matched_signatures"]; ok {
			if sigList, ok := sigs.([]string); ok && len(sigList) > 0 {
				return 20
			}
		}
	}

	return 0
}

// calculateRateScore 计算操作速率得分
func (de *DetectionEngine) calculateRateScore(activities []FileActivity) int {
	if len(activities) == 0 || de.config.WindowSizeSec == 0 {
		return 0
	}

	rate := float64(len(activities)) / float64(de.config.WindowSizeSec)
	threshold := float64(de.config.FileRateThreshold) / float64(de.config.WindowSizeSec)

	if rate > threshold*3 {
		return 10
	} else if rate > threshold*2 {
		return 7
	} else if rate > threshold {
		return 5
	}
	return 0
}

// =============================================================================
// 辅助方法
// =============================================================================

// scoreToThreatLevel 分数转威胁级别
func (de *DetectionEngine) scoreToThreatLevel(score int) ThreatLevel {
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
func (de *DetectionEngine) decideAction(level ThreatLevel) ResponseAction {
	switch level {
	case ThreatLevelCritical:
		if de.config.AutoDisableShare {
			return ActionDisableShare
		}
		return ActionBlockIP
	case ThreatLevelHigh:
		if de.config.AutoReadOnly {
			return ActionReadOnly
		}
		return ActionBlockIP
	case ThreatLevelMedium:
		if de.config.AutoBlockIP {
			return ActionBlockIP
		}
		return ActionRestrictAccess
	default:
		return ActionAlert
	}
}

// isSuspiciousExtension 检查是否为可疑扩展名
func (de *DetectionEngine) isSuspiciousExtension(path string) bool {
	ext := getFileExtension(path)
	de.mu.RLock()
	defer de.mu.RUnlock()

	for _, se := range de.config.SuspiciousExtensions {
		if strings.EqualFold(ext, se) {
			return true
		}
	}
	return false
}

// identifyPrimarySource 识别主要来源
func (de *DetectionEngine) identifyPrimarySource(stats activityStats) (processName string, processID int, userID string, sourceIP string) {
	// 找出操作最多的进程
	maxCount := 0
	for proc, count := range stats.processCounts {
		if count > maxCount {
			maxCount = count
			processName = proc
		}
	}

	// 找出操作最多的用户
	maxCount = 0
	for uid, count := range stats.userCounts {
		if count > maxCount {
			maxCount = count
			userID = uid
		}
	}

	// 找出操作最多的IP
	maxCount = 0
	for ip, count := range stats.ipCounts {
		if count > maxCount {
			maxCount = count
			sourceIP = ip
		}
	}

	return
}

// groupActivitiesBySource 按来源分组活动
func groupActivitiesBySource(activities []FileActivity) map[string][]FileActivity {
	groups := make(map[string][]FileActivity)
	for _, a := range activities {
		key := fmt.Sprintf("%s|%s", a.SourceIP, a.SourceUser)
		groups[key] = append(groups[key], a)
	}
	return groups
}

// getFileExtension 获取文件扩展名
func getFileExtension(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '.' {
			return path[i:]
		}
		if path[i] == '/' {
			return ""
		}
	}
	return ""
}

// GetPatterns 获取所有行为模式
func (de *DetectionEngine) GetPatterns() []BehaviorPattern {
	de.mu.RLock()
	defer de.mu.RUnlock()
	result := make([]BehaviorPattern, len(de.patterns))
	copy(result, de.patterns)
	return result
}

// GetSignatures 获取所有加密签名
func (de *DetectionEngine) GetSignatures() []EncryptionSignature {
	de.mu.RLock()
	defer de.mu.RUnlock()
	result := make([]EncryptionSignature, len(de.signatures))
	copy(result, de.signatures)
	return result
}

// AddPattern 添加自定义行为模式
func (de *DetectionEngine) AddPattern(pattern BehaviorPattern) {
	de.mu.Lock()
	defer de.mu.Unlock()
	de.patterns = append(de.patterns, pattern)
	log.Printf("添加行为模式: %s (%s)", pattern.ID, pattern.Name)
}

// AddSignature 添加自定义加密签名
func (de *DetectionEngine) AddSignature(sig EncryptionSignature) {
	de.mu.Lock()
	defer de.mu.Unlock()
	de.signatures = append(de.signatures, sig)
	log.Printf("添加加密签名: %s (%s)", sig.ID, sig.Name)
}

// UpdateConfig 更新检测配置
func (de *DetectionEngine) UpdateConfig(config DefenseConfig) {
	de.mu.Lock()
	de.config = config
	de.mu.Unlock()
	log.Println("检测引擎配置已更新")
}

// min 返回两个整数中的较小值
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// max 返回两个整数中的较大值
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// abs 返回浮点数绝对值
func absFloat64(x float64) float64 {
	return math.Abs(x)
}
