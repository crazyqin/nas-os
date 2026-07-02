// Package ransomware - 多因子威胁评分引擎
// 参考TrueNAS 26的多因子检测设计，综合行为+熵值+签名进行威胁评分
package ransomware

import (
	"math"
	"sync"
	"time"
)

// ThreatScoringEngine 多因子威胁评分引擎.
type ThreatScoringEngine struct {
	config    ThreatScoringConfig
	signature *SignatureMatcher
	entropy   *EntropyAnalyzer
	behavior  *BehaviorAnalyzer
	honeypot  *HoneypotDetector

	// 统计
	stats   ScoringStats
	statsMu sync.RWMutex

	// 事件缓存（用于时间模式分析）
	events   []*FileEvent
	eventsMu sync.RWMutex
}

// ScoringStats 评分统计.
type ScoringStats struct {
	TotalEvaluated     int64                   `json:"total_evaluated"`
	ByThreatLevel      map[ThreatLevel]int64   `json:"by_threat_level"`
	ByDetectionType    map[DetectionType]int64 `json:"by_detection_type"`
	AverageScore       float64                 `json:"average_score"`
	MultiFactorMatches int64                   `json:"multi_factor_matches"`
	HighEntropyEvents  int64                   `json:"high_entropy_events"`
	BehaviorTriggers   int64                   `json:"behavior_triggers"`
	HoneypotTriggers   int64                   `json:"honeypot_triggers"`
}

// NewThreatScoringEngine 创建威胁评分引擎.
func NewThreatScoringEngine(config ThreatScoringConfig) *ThreatScoringEngine {
	engine := &ThreatScoringEngine{
		config:    config,
		signature: NewSignatureMatcher(DefaultSignatureDB()),
		entropy:   NewEntropyAnalyzer(config.EntropyThreshold),
		behavior:  NewBehaviorAnalyzer(DefaultBehaviorPatterns()),
		honeypot:  NewHoneypotDetector(DefaultHoneypotConfig()),
		events:    make([]*FileEvent, 0, 10000),
		stats: ScoringStats{
			ByThreatLevel:   make(map[ThreatLevel]int64),
			ByDetectionType: make(map[DetectionType]int64),
		},
	}
	return engine
}

// Evaluate 综合评估文件事件，返回多因子威胁评分结果.
func (e *ThreatScoringEngine) Evaluate(event *FileEvent) *DetectionResult {
	result := &DetectionResult{
		ID:             generateID(),
		Timestamp:      time.Now(),
		FilePath:       event.Path,
		ThreatLevel:    ThreatLevelNone,
		ThreatScore:    0,
		DetectionType:  DetectionTypeMulti,
		DetectionTypes: make([]DetectionType, 0),
		FactorScores:   FactorScores{},
		Details:        make(map[string]interface{}),
		Confidence:     0,
	}

	// 1. 签名因子评估
	sigResult := e.signature.Match(event)
	if sigResult != nil {
		result.FactorScores.SignatureScore = sigResult.Score
		result.SignatureID = sigResult.SignatureID
		result.SignatureName = sigResult.SignatureName
		result.DetectionTypes = append(result.DetectionTypes, DetectionTypeSignature)
	}

	// 2. 熵值因子评估（检测加密文件）
	if entropy, ok := event.Entropies["file"]; ok {
		result.EntropyValue = entropy
		result.FactorScores.EntropyScore = e.entropy.Evaluate(entropy)
		if result.FactorScores.EntropyScore > 0 {
			result.DetectionTypes = append(result.DetectionTypes, DetectionTypeEntropy)
		}
	} else if event.Operation == FileOpWrite || event.Operation == FileOpModify {
		// 需要计算熵值的情况
		// 实际实现中会读取文件计算
	}

	// 3. 行为因子评估
	behaviorScore := e.behavior.Evaluate(event, e.getRecentEvents(e.config.RapidChangeWindow))
	result.FactorScores.BehaviorScore = behaviorScore
	if behaviorScore > 0 {
		result.DetectionTypes = append(result.DetectionTypes, DetectionTypeBehavior)
		if behaviorScore >= 50 {
			result.BehaviorID = "rapid_change"
			result.BehaviorName = "快速文件变更"
		}
	}

	// 4. 扩展名因子评估
	extScore := e.evaluateExtension(event)
	result.FactorScores.ExtensionScore = extScore
	if extScore > 0 {
		result.DetectionTypes = append(result.DetectionTypes, DetectionTypeExtension)
	}

	// 5. 诱饵文件因子评估（honeypot）
	honeypotScore := e.honeypot.Check(event)
	result.FactorScores.HoneypotScore = honeypotScore
	if honeypotScore > 0 {
		result.DetectionTypes = append(result.DetectionTypes, DetectionTypeHoneypot)
	}

	// 6. 时间模式因子评估
	timestampScore := e.evaluateTimestampPattern(event)
	result.FactorScores.TimestampScore = timestampScore
	if timestampScore > 0 {
		result.DetectionTypes = append(result.DetectionTypes, DetectionTypePattern)
	}

	// 7. 用户行为因子评估
	userScore := e.evaluateUserBehavior(event)
	result.FactorScores.UserScore = userScore

	// 计算综合评分（加权平均）
	totalScore := e.calculateWeightedScore(result.FactorScores)

	// 应用加成系数
	totalScore = e.applyBoosts(totalScore, result)

	// 限制最大值
	result.ThreatScore = min(100, max(0, int(math.Round(totalScore))))

	// 确定威胁等级
	result.ThreatLevel = e.determineThreatLevel(result.ThreatScore)

	// 计算置信度
	result.Confidence = e.calculateConfidence(result)

	// 更新统计
	e.updateStats(result)

	// 缓存事件
	e.addEvent(event)

	// 设置建议行动
	result.SuggestedAction = e.getSuggestedAction(result)

	return result
}

// calculateWeightedScore 计算加权综合评分.
func (e *ThreatScoringEngine) calculateWeightedScore(scores FactorScores) float64 {
	cfg := e.config

	weighted := 0.0
	weightSum := 0.0

	// 签名因子
	if scores.SignatureScore > 0 {
		weighted += float64(scores.SignatureScore) * cfg.SignatureWeight
		weightSum += cfg.SignatureWeight
	}

	// 熵值因子
	if scores.EntropyScore > 0 {
		weighted += float64(scores.EntropyScore) * cfg.EntropyWeight
		weightSum += cfg.EntropyWeight
	}

	// 行为因子
	if scores.BehaviorScore > 0 {
		weighted += float64(scores.BehaviorScore) * cfg.BehaviorWeight
		weightSum += cfg.BehaviorWeight
	}

	// 扩展名因子
	if scores.ExtensionScore > 0 {
		weighted += float64(scores.ExtensionScore) * cfg.ExtensionWeight
		weightSum += cfg.ExtensionWeight
	}

	// 诱饵因子
	if scores.HoneypotScore > 0 {
		weighted += float64(scores.HoneypotScore) * cfg.HoneypotWeight
		weightSum += cfg.HoneypotWeight
	}

	// 时间模式因子
	if scores.TimestampScore > 0 {
		weighted += float64(scores.TimestampScore) * cfg.TimestampPatternWeight
		weightSum += cfg.TimestampPatternWeight
	}

	// 用户行为因子
	if scores.UserScore > 0 {
		weighted += float64(scores.UserScore) * cfg.UserBehaviorWeight
		weightSum += cfg.UserBehaviorWeight
	}

	// 如果没有任何因子命中，返回0
	if weightSum == 0 {
		return 0
	}

	// 归一化到100
	return weighted / weightSum
}

// applyBoosts 应用加成系数.
func (e *ThreatScoringEngine) applyBoosts(score float64, result *DetectionResult) float64 {
	cfg := e.config

	// 多因子匹配加成
	if len(result.DetectionTypes) >= 3 {
		score *= cfg.MultipleFactorBoost
	}

	// KEV漏洞加成（如果签名匹配已知在利用的漏洞）
	if result.SignatureID != "" && e.signature.IsKEV(result.SignatureID) {
		score *= cfg.KEVBoost
	}

	// 勒索软件关联加成
	if result.SignatureName != "" && e.signature.IsRansomwareFamily(result.SignatureName) {
		score *= cfg.RansomwareBoost
	}

	return score
}

// determineThreatLevel 确定威胁等级.
func (e *ThreatScoringEngine) determineThreatLevel(score int) ThreatLevel {
	if score >= e.config.CriticalScoreThreshold {
		return ThreatLevelCritical
	} else if score >= e.config.HighScoreThreshold {
		return ThreatLevelHigh
	} else if score >= 40 {
		return ThreatLevelMedium
	} else if score >= 20 {
		return ThreatLevelLow
	}
	return ThreatLevelNone
}

// calculateConfidence 计算置信度.
func (e *ThreatScoringEngine) calculateConfidence(result *DetectionResult) float64 {
	// 置信度基于匹配因子数量和评分一致性
	factorCount := len(result.DetectionTypes)

	// 因子越多，置信度越高
	confidence := 0.0
	switch factorCount {
	case 0:
		confidence = 0
	case 1:
		confidence = 0.5
	case 2:
		confidence = 0.7
	case 3:
		confidence = 0.85
	default:
		confidence = 0.95
	}

	// 诱饵文件触发时置信度极高
	if result.FactorScores.HoneypotScore > 0 {
		confidence = math.Max(confidence, 0.95)
	}

	// 签名匹配时置信度提升
	if result.FactorScores.SignatureScore > 80 {
		confidence = math.Max(confidence, 0.9)
	}

	return confidence
}

// getSuggestedAction 获取建议行动.
func (e *ThreatScoringEngine) getSuggestedAction(result *DetectionResult) string {
	switch result.ThreatLevel {
	case ThreatLevelCritical:
		if result.FactorScores.HoneypotScore > 0 {
			return "诱饵文件触发，勒索软件攻击确认。立即隔离系统、断开网络、保护剩余文件、从备份恢复。"
		}
		return "高威胁评分，建议立即隔离受影响文件和进程，检查系统是否被加密，准备从备份恢复。"
	case ThreatLevelHigh:
		return "威胁评分较高，建议立即检查受影响文件，隔离可疑进程，启用只读模式保护数据。"
	case ThreatLevelMedium:
		return "中等威胁，建议检查文件状态，追踪可疑活动来源，考虑启用增强监控。"
	case ThreatLevelLow:
		return "低威胁信号，建议持续监控并记录相关事件。"
	default:
		return "无明确威胁，继续正常监控。"
	}
}

// evaluateExtension 评估扩展名风险.
func (e *ThreatScoringEngine) evaluateExtension(event *FileEvent) int {
	ext := event.Extension
	if ext == "" {
		return 0
	}

	// 检查是否为已知勒索软件扩展名
	if e.signature.MatchExtension(ext) {
		return 90
	}

	// 检查扩展名变更（加密典型特征）
	if event.OldExtension != "" && event.Extension != event.OldExtension {
		// 扩展名变为随机字符串模式
		if isRandomExtension(ext) {
			return 80
		}
		// 扩展名添加加密标记
		if containsEncryptionMarker(ext) {
			return 70
		}
	}

	return 0
}

// evaluateTimestampPattern 评估时间模式.
func (e *ThreatScoringEngine) evaluateTimestampPattern(event *FileEvent) int {
	// 分析最近的文件操作时间分布
	events := e.getRecentEvents(e.config.RapidChangeWindow)

	if len(events) < 10 {
		return 0
	}

	// 计算操作频率
	now := time.Now()
	countInLastWindow := 0
	for _, ev := range events {
		if now.Sub(ev.Timestamp).Seconds() <= float64(e.config.RapidChangeWindow) {
			countInLastWindow++
		}
	}

	// 快速变更模式
	if countInLastWindow >= e.config.RapidChangeThreshold {
		return min(100, countInLastWindow)
	}

	// 夜间异常活动（通常勒索软件在夜间执行）
	hour := event.Timestamp.Hour()
	if hour >= 0 && hour <= 5 && countInLastWindow > 10 {
		return 30
	}

	return 0
}

// evaluateUserBehavior 评估用户行为.
func (e *ThreatScoringEngine) evaluateUserBehavior(event *FileEvent) int {
	// 检查用户是否在短时间内操作了大量不同类型的文件
	events := e.getRecentEvents(300) // 5分钟

	if len(events) < 5 {
		return 0
	}

	userEvents := make(map[string]int)
	for _, ev := range events {
		if ev.UserID != "" {
			userEvents[ev.UserID]++
		}
	}

	// 单用户短时间内大量操作
	if event.UserID != "" && userEvents[event.UserID] > 50 {
		return 50
	}

	// 跨多个目录操作（目录遍历加密特征）
	dirs := make(map[string]int)
	for _, ev := range events {
		dir := getDirectory(ev.Path)
		dirs[dir]++
	}

	if len(dirs) > 5 && len(events) > 20 {
		return 40
	}

	return 0
}

// updateStats 更新统计.
func (e *ThreatScoringEngine) updateStats(result *DetectionResult) {
	e.statsMu.Lock()
	defer e.statsMu.Unlock()

	e.stats.TotalEvaluated++
	e.stats.ByThreatLevel[result.ThreatLevel]++

	for _, dt := range result.DetectionTypes {
		e.stats.ByDetectionType[dt]++
	}

	e.stats.AverageScore = (e.stats.AverageScore*float64(e.stats.TotalEvaluated-1) +
		float64(result.ThreatScore)) / float64(e.stats.TotalEvaluated)

	if len(result.DetectionTypes) >= 3 {
		e.stats.MultiFactorMatches++
	}
	if result.FactorScores.EntropyScore > 50 {
		e.stats.HighEntropyEvents++
	}
	if result.FactorScores.BehaviorScore > 50 {
		e.stats.BehaviorTriggers++
	}
	if result.FactorScores.HoneypotScore > 0 {
		e.stats.HoneypotTriggers++
	}
}

// addEvent 添加事件到缓存.
func (e *ThreatScoringEngine) addEvent(event *FileEvent) {
	e.eventsMu.Lock()
	defer e.eventsMu.Unlock()

	e.events = append(e.events, event)

	// 限制缓存大小
	if len(e.events) > 10000 {
		e.events = e.events[len(e.events)-5000:]
	}
}

// getRecentEvents 获取最近的事件.
func (e *ThreatScoringEngine) getRecentEvents(windowSeconds int) []*FileEvent {
	e.eventsMu.RLock()
	defer e.eventsMu.RUnlock()

	now := time.Now()
	cutoff := now.Add(-time.Duration(windowSeconds) * time.Second)

	var recent []*FileEvent
	for _, ev := range e.events {
		if ev.Timestamp.After(cutoff) {
			recent = append(recent, ev)
		}
	}

	return recent
}

// GetStats 获取统计信息.
func (e *ThreatScoringEngine) GetStats() ScoringStats {
	e.statsMu.RLock()
	defer e.statsMu.RUnlock()
	return e.stats
}

// ========== 熵值分析器 ==========

// EntropyAnalyzer 熵值分析器.
type EntropyAnalyzer struct {
	threshold float64
}

// NewEntropyAnalyzer 创建熵值分析器.
func NewEntropyAnalyzer(threshold float64) *EntropyAnalyzer {
	if threshold <= 0 {
		threshold = 7.5
	}
	return &EntropyAnalyzer{threshold: threshold}
}

// Evaluate 评估熵值，返回评分 0-100.
func (e *EntropyAnalyzer) Evaluate(entropy float64) int {
	if entropy < e.threshold {
		return 0
	}

	// 熵值越高评分越高
	// 7.5-8.0 范围映射到 0-100
	if entropy >= 8.0 {
		return 100
	}

	// 线性映射
	score := int((entropy - e.threshold) / (8.0 - e.threshold) * 100)
	return min(100, max(0, score))
}

// CalculateEntropy 计算数据的香农熵.
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
		if count > 0 {
			p := float64(count) / length
			entropy -= p * math.Log2(p)
		}
	}

	return entropy
}

// ========== 签名匹配器 ==========

// SignatureMatcher 签名匹配器.
type SignatureMatcher struct {
	db          *SignatureDB
	extensions  map[string]*RansomwareSignature
	ransomNotes map[string]*RansomwareSignature
}

// NewSignatureMatcher 创建签名匹配器.
func NewSignatureMatcher(db *SignatureDB) *SignatureMatcher {
	m := &SignatureMatcher{
		db:          db,
		extensions:  make(map[string]*RansomwareSignature),
		ransomNotes: make(map[string]*RansomwareSignature),
	}

	// 建立索引
	for _, sig := range db.Signatures {
		for _, ext := range sig.Extensions {
			m.extensions[ext] = sig
		}
		for _, note := range sig.RansomNoteFiles {
			m.ransomNotes[note] = sig
		}
	}

	return m
}

// MatchResult 签名匹配结果.
type MatchResult struct {
	Score         int
	SignatureID   string
	SignatureName string
	Family        string
}

// Match 匹配文件事件.
func (m *SignatureMatcher) Match(event *FileEvent) *MatchResult {
	// 扩展名匹配
	if event.Extension != "" {
		if sig, ok := m.extensions[event.Extension]; ok {
			return &MatchResult{
				Score:         90,
				SignatureID:   sig.ID,
				SignatureName: sig.Name,
				Family:        sig.Family,
			}
		}
	}

	// 勒索信文件名匹配
	filename := getFilename(event.Path)
	if sig, ok := m.ransomNotes[filename]; ok {
		return &MatchResult{
			Score:         95,
			SignatureID:   sig.ID,
			SignatureName: sig.Name,
			Family:        sig.Family,
		}
	}

	return nil
}

// MatchExtension 匹配扩展名.
func (m *SignatureMatcher) MatchExtension(ext string) bool {
	_, ok := m.extensions[ext]
	return ok
}

// MatchRansomNote 匹配勒索信文件名.
func (m *SignatureMatcher) MatchRansomNote(filename string) bool {
	_, ok := m.ransomNotes[filename]
	return ok
}

// IsKEV 检查是否为已知在利用的漏洞关联.
func (m *SignatureMatcher) IsKEV(sigID string) bool {
	for _, sig := range m.db.Signatures {
		if sig.ID == sigID {
			// 检查是否在KEV列表中（实际应查询KEV数据库）
			return sig.Severity == ThreatLevelCritical
		}
	}
	return false
}

// IsRansomwareFamily 检查是否为勒索软件家族.
func (m *SignatureMatcher) IsRansomwareFamily(name string) bool {
	for _, sig := range m.db.Signatures {
		if sig.Name == name || contains(sig.Aliases, name) {
			return true
		}
	}
	return false
}

// ========== 行为分析器 ==========

// BehaviorAnalyzer 行为分析器.
type BehaviorAnalyzer struct {
	patterns []BehaviorPattern
}

// NewBehaviorAnalyzer 创建行为分析器.
func NewBehaviorAnalyzer(patterns []BehaviorPattern) *BehaviorAnalyzer {
	return &BehaviorAnalyzer{patterns: patterns}
}

// Evaluate 评估行为模式.
func (b *BehaviorAnalyzer) Evaluate(event *FileEvent, recentEvents []*FileEvent) int {
	totalScore := 0

	for _, pattern := range b.patterns {
		if !pattern.Enabled {
			continue
		}

		if b.matchPattern(pattern, event, recentEvents) {
			totalScore += pattern.Weight
		}
	}

	return min(100, totalScore)
}

// matchPattern 匹配行为模式.
func (b *BehaviorAnalyzer) matchPattern(pattern BehaviorPattern, event *FileEvent, recentEvents []*FileEvent) bool {
	matchedConditions := 0

	for _, cond := range pattern.Conditions {
		if b.evaluateCondition(cond, event, recentEvents) {
			matchedConditions++
		}
	}

	return matchedConditions >= len(pattern.Conditions)/2
}

// evaluateCondition 评估条件.
func (b *BehaviorAnalyzer) evaluateCondition(cond Condition, event *FileEvent, recentEvents []*FileEvent) bool {
	switch cond.Type {
	case "count":
		return b.evaluateCountCondition(cond, recentEvents)
	case "operation":
		return string(event.Operation) == cond.Value
	case "match":
		return b.evaluateMatchCondition(cond, event)
	default:
		return false
	}
}

// evaluateCountCondition 评估计数条件.
func (b *BehaviorAnalyzer) evaluateCountCondition(cond Condition, events []*FileEvent) bool {
	count := 0
	for _, ev := range events {
		if string(ev.Operation) == cond.Field {
			count++
		}
	}
	return count >= cond.Count
}

// evaluateMatchCondition 评估匹配条件.
func (b *BehaviorAnalyzer) evaluateMatchCondition(cond Condition, event *FileEvent) bool {
	switch cond.Field {
	case "extension":
		return event.Extension == cond.Value
	case "old_extension":
		return event.OldExtension == cond.Value
	default:
		return false
	}
}

// ========== 诱饵检测器 ==========

// HoneypotDetector 诱饵文件检测器.
type HoneypotDetector struct {
	config HoneypotConfig
	files  map[string]*HoneypotFile // path -> file
	mu     sync.RWMutex
}

// HoneypotFile 诱饵文件记录.
type HoneypotFile struct {
	ID          string
	Path        string
	Hash        string
	CreatedAt   time.Time
	Status      string // active, triggered, deleted
	TriggeredAt *time.Time
	TriggerType string
}

// NewHoneypotDetector 创建诱饵检测器.
func NewHoneypotDetector(config HoneypotConfig) *HoneypotDetector {
	return &HoneypotDetector{
		config: config,
		files:  make(map[string]*HoneypotFile),
	}
}

// Check 检查事件是否触及诱饵文件.
func (h *HoneypotDetector) Check(event *FileEvent) int {
	h.mu.RLock()
	defer h.mu.RUnlock()

	_, ok := h.files[event.Path]
	if !ok {
		return 0
	}

	// 诱饵文件被操作，高分威胁
	switch event.Operation {
	case FileOpDelete:
		return 100
	case FileOpModify:
		return 95
	case FileOpRename:
		return 90
	case FileOpRead:
		if h.config.AlertOnAccess {
			return 30
		}
		return 0
	default:
		return 50
	}
}

// RegisterFile 注册诱饵文件.
func (h *HoneypotDetector) RegisterFile(file *HoneypotFile) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.files[file.Path] = file
}

// ========== 签名数据库 ==========

// SignatureDB 签名数据库.
type SignatureDB struct {
	Signatures  []*RansomwareSignature
	LastUpdated time.Time
}

// DefaultSignatureDB 默认签名库.
func DefaultSignatureDB() *SignatureDB {
	return &SignatureDB{
		Signatures: []*RansomwareSignature{
			{
				ID:              "wannacry",
				Name:            "WannaCry",
				Family:          "WannaCry",
				Extensions:      []string{".WNRY", ".wannacry", ".wcry"},
				RansomNoteFiles: []string{"@WanaDecryptor@.exe.lnk", "@WanaDecryptor@.exe", "WannaDecryptor.exe"},
				Severity:        ThreatLevelCritical,
			},
			{
				ID:              "locky",
				Name:            "Locky",
				Family:          "Locky",
				Extensions:      []string{".locky", ".zepto", ".odin", ".shit", ".thor", ".hammer"},
				RansomNoteFiles: []string{"_locky_recover_instructions.txt", "locky_recover_instructions.txt"},
				Severity:        ThreatLevelCritical,
			},
			{
				ID:              "cryptolocker",
				Name:            "CryptoLocker",
				Family:          "CryptoLocker",
				Extensions:      []string{".encrypted", ".crypto"},
				RansomNoteFiles: []string{"DECRYPT_INSTRUCTIONS.txt", "DECRYPT_INSTRUCTIONS.html"},
				Severity:        ThreatLevelCritical,
			},
			{
				ID:              "ryuk",
				Name:            "Ryuk",
				Family:          "Ryuk",
				Extensions:      []string{".RYK"},
				RansomNoteFiles: []string{"RyukReadMe.txt", "RyukReadMe.html"},
				Severity:        ThreatLevelCritical,
			},
			{
				ID:              "conti",
				Name:            "Conti",
				Family:          "Conti",
				Extensions:      []string{".CONTI"},
				RansomNoteFiles: []string{"CONTI_README.txt"},
				Severity:        ThreatLevelCritical,
			},
			{
				ID:              "blackcat",
				Name:            "BlackCat/ALPHV",
				Family:          "BlackCat",
				Extensions:      []string{".alphv", ".blackcat"},
				RansomNoteFiles: []string{"README-ALPHV.txt", "README-BlackCat.txt"},
				Severity:        ThreatLevelCritical,
			},
			{
				ID:         "generic-encrypted",
				Name:       "Generic Encrypted",
				Family:     "Unknown",
				Extensions: []string{".encrypted", ".locked", ".crypto", ".enc", ".ransom"},
				Severity:   ThreatLevelHigh,
			},
		},
		LastUpdated: time.Now(),
	}
}

// ========== 默认行为模式 ==========

// DefaultBehaviorPatterns 默认行为模式.
func DefaultBehaviorPatterns() []BehaviorPattern {
	return []BehaviorPattern{
		{
			ID:          "rapid-file-modification",
			Name:        "快速文件修改",
			Description: "短时间内大量文件被修改",
			Conditions: []Condition{
				{Type: "count", Field: "modify", Count: 10},
				{Type: "count", Field: "write", Count: 10},
			},
			Weight:    30,
			Threshold: 20,
			Severity:  ThreatLevelHigh,
			Enabled:   true,
		},
		{
			ID:          "mass-rename",
			Name:        "批量重命名",
			Description: "大量文件被重命名（加密标记）",
			Conditions: []Condition{
				{Type: "count", Field: "rename", Count: 5},
			},
			Weight:    25,
			Threshold: 20,
			Severity:  ThreatLevelHigh,
			Enabled:   true,
		},
		{
			ID:          "rapid-deletion",
			Name:        "快速删除",
			Description: "短时间内大量文件删除",
			Conditions: []Condition{
				{Type: "count", Field: "delete", Count: 20},
			},
			Weight:    20,
			Threshold: 15,
			Severity:  ThreatLevelHigh,
			Enabled:   true,
		},
		{
			ID:          "extension-change-pattern",
			Name:        "扩展名变更模式",
			Description: "文件扩展名批量变更",
			Conditions: []Condition{
				{Type: "operation", Field: "rename", Value: "rename"},
			},
			Weight:    30,
			Threshold: 25,
			Severity:  ThreatLevelCritical,
			Enabled:   true,
		},
	}
}

// ========== 辅助函数 ==========

func generateID() string {
	return "det_" + time.Now().Format("20060102150405") + "_" + randomHex(8)
}

func randomHex(n int) string {
	// 简化实现
	return time.Now().Format("nanosecond")[:n]
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func getFilename(path string) string {
	// 从路径提取文件名
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return path
}

func getDirectory(path string) string {
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[:i]
		}
	}
	return ""
}

func isRandomExtension(ext string) bool {
	// 检查是否为随机字符串扩展名（勒索软件典型特征）
	// 如 .abc123, .xyz789 等
	if len(ext) < 4 || len(ext) > 12 {
		return false
	}
	// 去掉前导点
	ext = ext[1:]
	// 检查是否为字母数字混合
	hasLetter := false
	hasDigit := false
	for _, c := range ext {
		if c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' {
			hasLetter = true
		}
		if c >= '0' && c <= '9' {
			hasDigit = true
		}
	}
	return hasLetter && hasDigit
}

func containsEncryptionMarker(ext string) bool {
	markers := []string{"encrypted", "locked", "crypto", "enc", "ransom"}
	lowerExt := ext[1:] // 去掉点
	for _, m := range markers {
		if lowerExt == m || containsStr(lowerExt, m) {
			return true
		}
	}
	return false
}

func containsStr(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func contains(list []string, item string) bool {
	for _, s := range list {
		if s == item {
			return true
		}
	}
	return false
}

// DefaultHoneypotConfig 默认诱饵配置.
func DefaultHoneypotConfig() HoneypotConfig {
	return HoneypotConfig{
		Enabled:       true,
		FilesPerPath:  5,
		FileTypes:     []string{".doc", ".docx", ".xls", ".xlsx", ".pdf"},
		AlertOnModify: true,
		AlertOnDelete: true,
		AlertOnRename: true,
	}
}
