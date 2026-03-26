package ransomware

import (
	"container/list"
	"crypto/sha256"
	"encoding/hex"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// BehaviorMonitor 行为监控器.
type BehaviorMonitor struct {
	config       MonitorConfig
	signatureDB  *SignatureDB
	events       *list.List
	eventMu      sync.RWMutex
	patterns     []BehaviorPattern
	alertChan    chan<- Alert
	startTime    time.Time
	stats        Statistics
	statsMu      sync.RWMutex
}

// NewBehaviorMonitor 创建行为监控器.
func NewBehaviorMonitor(config MonitorConfig, sigDB *SignatureDB) *BehaviorMonitor {
	m := &BehaviorMonitor{
		config:      config,
		signatureDB: sigDB,
		events:      list.New(),
		startTime:   time.Now(),
	}

	// 加载内置行为模式
	m.loadBehaviorPatterns()

	return m
}

// SetAlertChannel 设置告警通道.
func (m *BehaviorMonitor) SetAlertChannel(ch chan<- Alert) {
	m.alertChan = ch
}

// loadBehaviorPatterns 加载内置行为模式.
func (m *BehaviorMonitor) loadBehaviorPatterns() {
	m.patterns = []BehaviorPattern{
		// 模式1: 快速文件加密行为
		{
			ID:          "rapid-encryption",
			Name:        "快速文件加密",
			Description: "短时间内大量文件被修改且熵值显著增加",
			Conditions: []Condition{
				{Type: "count", Field: "modify", Operator: "gt", Value: 10, TimeWindow: 60},
				{Type: "average", Field: "entropy", Operator: "gt", Value: 7.0},
			},
			Weight:    30,
			Threshold: 25,
			Severity:  ThreatLevelCritical,
			Enabled:   true,
		},
		// 模式2: 批量扩展名变更
		{
			ID:          "mass-extension-change",
			Name:        "批量扩展名变更",
			Description: "大量文件被重命名为相同或相似的扩展名",
			Conditions: []Condition{
				{Type: "count", Field: "rename", Operator: "gt", Value: 5, TimeWindow: 60},
				{Type: "match", Field: "new_extension", Operator: "regex", Value: "^[a-z0-9]{4,10}$"},
			},
			Weight:    25,
			Threshold: 20,
			Severity:  ThreatLevelHigh,
			Enabled:   true,
		},
		// 模式3: 创建勒索信
		{
			ID:          "ransom-note-creation",
			Name:        "勒索信创建",
			Description: "检测到勒索信文件被创建",
			Conditions: []Condition{
				{Type: "match", Field: "filename", Operator: "in_ransom_notes", Value: true},
			},
			Weight:    40,
			Threshold: 30,
			Severity:  ThreatLevelCritical,
			Enabled:   true,
		},
		// 模式4: 快速文件删除
		{
			ID:          "rapid-deletion",
			Name:        "快速文件删除",
			Description: "短时间内大量文件被删除",
			Conditions: []Condition{
				{Type: "count", Field: "delete", Operator: "gt", Value: 20, TimeWindow: 60},
			},
			Weight:    20,
			Threshold: 15,
			Severity:  ThreatLevelHigh,
			Enabled:   true,
		},
		// 模式5: 影子副本删除（Windows特有，此处作为标记）
		{
			ID:          "shadow-copy-deletion",
			Name:        "影子副本删除",
			Description: "尝试删除备份或影子副本",
			Conditions: []Condition{
				{Type: "match", Field: "command", Operator: "contains", Value: "vssadmin"},
			},
			Weight:    35,
			Threshold: 30,
			Severity:  ThreatLevelCritical,
			Enabled:   true,
		},
		// 模式6: 高熵值文件创建
		{
			ID:          "high-entropy-files",
			Name:        "高熵值文件创建",
			Description: "创建的高熵值文件数量异常",
			Conditions: []Condition{
				{Type: "count", Field: "high_entropy_create", Operator: "gt", Value: 5, TimeWindow: 120},
				{Type: "average", Field: "entropy", Operator: "gt", Value: 7.5},
			},
			Weight:    25,
			Threshold: 20,
			Severity:  ThreatLevelHigh,
			Enabled:   true,
		},
		// 模式7: 目录遍历加密
		{
			ID:          "directory-traversal-encryption",
			Name:        "目录遍历加密",
			Description: "按目录结构系统性加密文件",
			Conditions: []Condition{
				{Type: "count", Field: "unique_directories", Operator: "gt", Value: 3, TimeWindow: 120},
				{Type: "count", Field: "modify", Operator: "gt", Value: 10, TimeWindow: 120},
			},
			Weight:    20,
			Threshold: 15,
			Severity:  ThreatLevelHigh,
			Enabled:   true,
		},
		// 模式8: 已知勒索软件扩展名
		{
			ID:          "known-ransomware-extension",
			Name:        "已知勒索软件扩展名",
			Description: "文件被重命名为已知的勒索软件扩展名",
			Conditions: []Condition{
				{Type: "match", Field: "extension", Operator: "in_signature_db", Value: true},
			},
			Weight:    45,
			Threshold: 35,
			Severity:  ThreatLevelCritical,
			Enabled:   true,
		},
	}
}

// ProcessEvent 处理文件事件.
func (m *BehaviorMonitor) ProcessEvent(event FileEvent) *DetectionResult {
	// 检查是否在排除路径中
	if m.isExcluded(event.Path) {
		return nil
	}

	// 添加事件到缓存
	m.addEvent(event)

	// 更新统计
	m.updateStats(event)

	// 1. 扩展名检测
	if result := m.detectByExtension(event); result != nil {
		return result
	}

	// 2. 勒索信检测
	if result := m.detectByRansomNote(event); result != nil {
		return result
	}

	// 3. 行为模式分析
	if result := m.analyzeBehavior(event); result != nil {
		return result
	}

	return nil
}

// addEvent 添加事件到缓存.
func (m *BehaviorMonitor) addEvent(event FileEvent) {
	m.eventMu.Lock()
	defer m.eventMu.Unlock()

	// 限制事件数量
	for m.events.Len() >= m.config.MaxEvents {
		m.events.Remove(m.events.Front())
	}

	m.events.PushBack(event)
}

// isExcluded 检查路径是否被排除.
func (m *BehaviorMonitor) isExcluded(path string) bool {
	for _, excluded := range m.config.ExcludePaths {
		if strings.HasPrefix(path, excluded) {
			return true
		}
	}
	return false
}

// detectByExtension 通过扩展名检测.
func (m *BehaviorMonitor) detectByExtension(event FileEvent) *DetectionResult {
	ext := strings.ToLower(filepath.Ext(event.Path))
	if ext == "" {
		return nil
	}

	sigs := m.signatureDB.MatchExtension(event.Path)
	if len(sigs) == 0 {
		return nil
	}

	// 取最高严重级别
	var maxSeverity ThreatLevel
	var matchedSig *RansomwareSignature
	for _, sig := range sigs {
		if sig.Severity == ThreatLevelCritical || maxSeverity == "" {
			maxSeverity = sig.Severity
			matchedSig = sig
		}
	}

	return &DetectionResult{
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		ThreatLevel:   maxSeverity,
		DetectionType: DetectionTypeExtension,
		SignatureID:   matchedSig.ID,
		SignatureName: matchedSig.Name,
		FilePath:      event.Path,
		Confidence:    0.9,
		Details: map[string]interface{}{
			"matched_extension": ext,
			"family":            matchedSig.Family,
		},
		SuggestedAction: "立即隔离文件并检查系统",
	}
}

// detectByRansomNote 通过勒索信检测.
func (m *BehaviorMonitor) detectByRansomNote(event FileEvent) *DetectionResult {
	if event.Operation != FileOpCreate {
		return nil
	}

	filename := filepath.Base(event.Path)
	sigs := m.signatureDB.MatchRansomNote(filename)
	if len(sigs) == 0 {
		// 检查是否匹配通用勒索信模式
		if m.isRansomNotePattern(filename) {
			return &DetectionResult{
				ID:            uuid.New().String(),
				Timestamp:     time.Now(),
				ThreatLevel:   ThreatLevelHigh,
				DetectionType: DetectionTypeSignature,
				SignatureID:   "generic-ransom-note",
				SignatureName: "Generic Ransom Note",
				FilePath:      event.Path,
				Confidence:    0.8,
				Details: map[string]interface{}{
					"filename": filename,
					"pattern":  "generic ransom note",
				},
				SuggestedAction: "检查文件内容，确认是否为勒索信",
			}
		}
		return nil
	}

	// 取最高严重级别
	var matchedSig *RansomwareSignature
	for _, sig := range sigs {
		if matchedSig == nil || sig.Severity == ThreatLevelCritical {
			matchedSig = sig
		}
	}

	return &DetectionResult{
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		ThreatLevel:   matchedSig.Severity,
		DetectionType: DetectionTypeSignature,
		SignatureID:   matchedSig.ID,
		SignatureName: matchedSig.Name,
		FilePath:      event.Path,
		Confidence:    0.95,
		Details: map[string]interface{}{
			"ransom_note": filename,
			"family":      matchedSig.Family,
		},
		SuggestedAction: "立即隔离系统，防止进一步感染",
	}
}

// isRansomNotePattern 检查是否匹配勒索信模式.
func (m *BehaviorMonitor) isRansomNotePattern(filename string) bool {
	lowerName := strings.ToLower(filename)
	patterns := []string{
		"readme", "decrypt", "restore", "ransom", "recovery",
		"how_to", "restore_files", "decrypt_files", "!!!",
	}

	for _, pattern := range patterns {
		if strings.Contains(lowerName, pattern) {
			return true
		}
	}
	return false
}

// analyzeBehavior 行为分析.
func (m *BehaviorMonitor) analyzeBehavior(event FileEvent) *DetectionResult {
	m.eventMu.RLock()
	defer m.eventMu.RUnlock()

	var totalScore int
	var matchedPatterns []BehaviorPattern

	for _, pattern := range m.patterns {
		if !pattern.Enabled {
			continue
		}

		score := m.evaluatePattern(pattern, event)
		if score >= pattern.Threshold {
			totalScore += score
			matchedPatterns = append(matchedPatterns, pattern)
		}
	}

	if totalScore < 20 {
		return nil
	}

	// 确定威胁级别
	severity := ThreatLevelMedium
	if totalScore >= 50 {
		severity = ThreatLevelCritical
	} else if totalScore >= 35 {
		severity = ThreatLevelHigh
	}

	// 获取相关文件
	affectedFiles := m.getAffectedFiles(event.Path)

	return &DetectionResult{
		ID:            uuid.New().String(),
		Timestamp:     time.Now(),
		ThreatLevel:   severity,
		DetectionType: DetectionTypeBehavior,
		BehaviorID:    matchedPatterns[0].ID,
		BehaviorName:  matchedPatterns[0].Name,
		FilePath:      event.Path,
		FileCount:     len(affectedFiles),
		Confidence:    float64(totalScore) / 100.0,
		AffectedFiles: affectedFiles,
		Details: map[string]interface{}{
			"score":            totalScore,
			"matched_patterns": len(matchedPatterns),
			"patterns":         getPatternNames(matchedPatterns),
		},
		SuggestedAction: "建议隔离相关文件并进行安全检查",
	}
}

// evaluatePattern 评估行为模式.
func (m *BehaviorMonitor) evaluatePattern(pattern BehaviorPattern, currentEvent FileEvent) int {
	score := 0
	now := time.Now()

	for _, cond := range pattern.Conditions {
		switch cond.Type {
		case "count":
			if m.evaluateCountCondition(cond, now) {
				score += pattern.Weight
			}
		case "match":
			if m.evaluateMatchCondition(cond, currentEvent) {
				score += pattern.Weight
			}
		case "average":
			if m.evaluateAverageCondition(cond, now) {
				score += pattern.Weight
			}
		}
	}

	return score
}

// evaluateCountCondition 评估计数条件.
func (m *BehaviorMonitor) evaluateCountCondition(cond Condition, now time.Time) bool {
	timeWindow := cond.TimeWindow
	if timeWindow == 0 {
		timeWindow = 300 // 默认5分钟
	}

	cutoff := now.Add(-time.Duration(timeWindow) * time.Second)
	var count int

	for e := m.events.Back(); e != nil; e = e.Prev() {
		event := e.Value.(FileEvent)
		if event.Timestamp.Before(cutoff) {
			break
		}

		switch cond.Field {
		case "modify":
			if event.Operation == FileOpModify || event.Operation == FileOpWrite {
				count++
			}
		case "delete":
			if event.Operation == FileOpDelete {
				count++
			}
		case "rename":
			if event.Operation == FileOpRename {
				count++
			}
		case "create":
			if event.Operation == FileOpCreate {
				count++
			}
		case "high_entropy_create":
			if event.Operation == FileOpCreate {
				if entropy, ok := event.Entropies["file"]; ok && entropy > 7.5 {
					count++
				}
			}
		case "unique_directories":
			// 简化实现，实际应统计唯一目录数
			count++
		}
	}

	return m.compareValue(count, cond.Operator, cond.Value)
}

// evaluateMatchCondition 评估匹配条件.
func (m *BehaviorMonitor) evaluateMatchCondition(cond Condition, event FileEvent) bool {
	var value string

	switch cond.Field {
	case "filename":
		value = filepath.Base(event.Path)
	case "extension":
		value = filepath.Ext(event.Path)
	case "new_extension":
		if event.OldExtension != "" {
			value = filepath.Ext(event.Path)
		}
	case "command":
		if event.Metadata != nil {
			if cmd, ok := event.Metadata["command"].(string); ok {
				value = cmd
			}
		}
	}

	switch cond.Operator {
	case "contains":
		return strings.Contains(value, cond.Value.(string))
	case "regex":
		// 简化实现，实际应使用正则
		return len(value) >= 4 && len(value) <= 10
	case "in_ransom_notes":
		return len(m.signatureDB.MatchRansomNote(value)) > 0
	case "in_signature_db":
		return len(m.signatureDB.MatchExtension(value)) > 0
	}

	return false
}

// evaluateAverageCondition 评估平均值条件.
func (m *BehaviorMonitor) evaluateAverageCondition(cond Condition, now time.Time) bool {
	timeWindow := cond.TimeWindow
	if timeWindow == 0 {
		timeWindow = 300
	}

	cutoff := now.Add(-time.Duration(timeWindow) * time.Second)
	var total float64
	var count int

	for e := m.events.Back(); e != nil; e = e.Prev() {
		event := e.Value.(FileEvent)
		if event.Timestamp.Before(cutoff) {
			break
		}

		if event.Entropies != nil {
			if entropy, ok := event.Entropies[cond.Field]; ok {
				total += entropy
				count++
			}
		}
	}

	if count == 0 {
		return false
	}

	avg := total / float64(count)
	return m.compareValue(avg, cond.Operator, cond.Value)
}

// compareValue 比较值.
func (m *BehaviorMonitor) compareValue(actual interface{}, operator string, expected interface{}) bool {
	switch operator {
	case "gt":
		return toFloat(actual) > toFloat(expected)
	case "gte":
		return toFloat(actual) >= toFloat(expected)
	case "lt":
		return toFloat(actual) < toFloat(expected)
	case "lte":
		return toFloat(actual) <= toFloat(expected)
	case "eq":
		return actual == expected
	case "ne":
		return actual != expected
	}
	return false
}

// getAffectedFiles 获取受影响的文件列表.
func (m *BehaviorMonitor) getAffectedFiles(basePath string) []string {
	var files []string
	dir := filepath.Dir(basePath)

	for e := m.events.Back(); e != nil; e = e.Prev() {
		event := e.Value.(FileEvent)
		if filepath.Dir(event.Path) == dir {
			files = append(files, event.Path)
		}
	}

	if len(files) > 100 {
		files = files[:100]
	}

	return files
}

// updateStats 更新统计信息.
func (m *BehaviorMonitor) updateStats(event FileEvent) {
	m.statsMu.Lock()
	defer m.statsMu.Unlock()

	m.stats.TotalEvents++
}

// GetStats 获取统计信息.
func (m *BehaviorMonitor) GetStats() Statistics {
	m.statsMu.RLock()
	defer m.statsMu.RUnlock()

	stats := m.stats
	stats.Uptime = time.Since(m.startTime)
	return stats
}

// ClearEvents 清除事件缓存.
func (m *BehaviorMonitor) ClearEvents() {
	m.eventMu.Lock()
	defer m.eventMu.Unlock()

	m.events = list.New()
}

// CalculateEntropy 计算文件熵值.
func CalculateEntropy(data []byte) float64 {
	if len(data) == 0 {
		return 0
	}

	freq := make(map[byte]int)
	for _, b := range data {
		freq[b]++
	}

	var entropy float64
	length := float64(len(data))

	for _, count := range freq {
		p := float64(count) / length
		entropy -= p * math.Log2(p)
	}

	return entropy
}

// CalculateFileEntropy 计算文件熵值.
func CalculateFileEntropy(filePath string) (float64, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return 0, err
	}
	return CalculateEntropy(data), nil
}

// CalculateFileHash 计算文件哈希.
func CalculateFileHash(filePath string) (string, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return "", err
	}

	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:]), nil
}

// Helper functions

func toFloat(v interface{}) float64 {
	switch val := v.(type) {
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case float64:
		return val
	case float32:
		return float64(val)
	}
	return 0
}

func getPatternNames(patterns []BehaviorPattern) []string {
	var names []string
	for _, p := range patterns {
		names = append(names, p.Name)
	}
	return names
}