// Package ransomshield - 取证分析
// 事件时间线重建、攻击链分析、证据收集、取证报告生成
package ransomshield

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// ============================================================
// 取证分析引擎
// ============================================================

// ForensicsEngine 取证分析引擎
type ForensicsEngine struct {
	mu sync.RWMutex

	incidents     map[string]*SecurityIncident
	evidenceStore map[string]*Evidence
	timelines     map[string]*Timeline
	attackChains  map[string]*AttackChain
	reportDir     string
	stats         ForensicsStats
	running       bool
	stopChan      chan struct{}
}

// SecurityIncident 安全事件
type SecurityIncident struct {
	ID            string      `json:"id"`
	Title         string      `json:"title"`
	Description   string      `json:"description"`
	Severity      ThreatLevel `json:"severity"`
	Status        string      `json:"status"`
	ThreatIDs     []string    `json:"threat_ids"`
	EvidenceIDs   []string    `json:"evidence_ids"`
	TimelineID    string      `json:"timeline_id"`
	AttackChainID string      `json:"attack_chain_id"`
	Tags          []string    `json:"tags"`
	Assignee      string      `json:"assignee"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
	ResolvedAt    *time.Time  `json:"resolved_at,omitempty"`
}

// Evidence 证据
type Evidence struct {
	ID             string         `json:"id"`
	IncidentID     string         `json:"incident_id"`
	Type           string         `json:"type"`
	Name           string         `json:"name"`
	Path           string         `json:"path"`
	Hash           string         `json:"hash"`
	SizeBytes      int64          `json:"size_bytes"`
	Description    string         `json:"description"`
	CollectedBy    string         `json:"collected_by"`
	ChainOfCustody []CustodyEntry `json:"chain_of_custody"`
	Tags           []string       `json:"tags"`
	CreatedAt      time.Time      `json:"created_at"`
}

// CustodyEntry 证据保管链
type CustodyEntry struct {
	Action    string    `json:"action"`
	Officer   string    `json:"officer"`
	Timestamp time.Time `json:"timestamp"`
	Notes     string    `json:"notes"`
}

// Timeline 事件时间线
type Timeline struct {
	ID         string          `json:"id"`
	IncidentID string          `json:"incident_id"`
	Events     []TimelineEvent `json:"events"`
	StartTime  time.Time       `json:"start_time"`
	EndTime    time.Time       `json:"end_time"`
	CreatedAt  time.Time       `json:"created_at"`
}

// TimelineEvent 时间线事件
type TimelineEvent struct {
	ID          string                 `json:"id"`
	Timestamp   time.Time              `json:"timestamp"`
	Phase       AttackPhase            `json:"phase"`
	Category    string                 `json:"category"`
	Description string                 `json:"description"`
	Source      string                 `json:"source"`
	Details     map[string]interface{} `json:"details,omitempty"`
	Severity    ThreatLevel            `json:"severity"`
	Confidence  float64                `json:"confidence"`
}

// AttackChain 攻击链
type AttackChain struct {
	ID         string       `json:"id"`
	IncidentID string       `json:"incident_id"`
	Name       string       `json:"name"`
	Family     string       `json:"family"`
	Phases     []ChainPhase `json:"phases"`
	IOCs       []string     `json:"iocs"`
	TTPs       []string     `json:"ttps"`
	Confidence float64      `json:"confidence"`
	StartTime  time.Time    `json:"start_time"`
	EndTime    time.Time    `json:"end_time"`
	CreatedAt  time.Time    `json:"created_at"`
}

// ChainPhase 攻击链阶段
type ChainPhase struct {
	Phase       AttackPhase `json:"phase"`
	Description string      `json:"description"`
	Indicators  []string    `json:"indicators"`
	StartTime   time.Time   `json:"start_time"`
	EndTime     time.Time   `json:"end_time"`
	Confidence  float64     `json:"confidence"`
}

// ForensicsReport 取证报告
type ForensicsReport struct {
	ID              string       `json:"id"`
	IncidentID      string       `json:"incident_id"`
	Title           string       `json:"title"`
	Summary         string       `json:"summary"`
	Timeline        *Timeline    `json:"timeline,omitempty"`
	AttackChain     *AttackChain `json:"attack_chain,omitempty"`
	Evidence        []Evidence   `json:"evidence"`
	IOCs            []IOC        `json:"iocs"`
	Recommendations []string     `json:"recommendations"`
	GeneratedAt     time.Time    `json:"generated_at"`
	GeneratedBy     string       `json:"generated_by"`
	Format          string       `json:"format"`
}

// ForensicsStats 取证统计
type ForensicsStats struct {
	TotalIncidents    int       `json:"total_incidents"`
	OpenIncidents     int       `json:"open_incidents"`
	ResolvedIncidents int       `json:"resolved_incidents"`
	TotalEvidence     int       `json:"total_evidence"`
	TotalTimelines    int       `json:"total_timelines"`
	TotalReports      int64     `json:"total_reports"`
	LastReportTime    time.Time `json:"last_report_time"`
}

// ============================================================
// 构造与生命周期
// ============================================================

// NewForensicsEngine 创建取证分析引擎
func NewForensicsEngine(reportDir string) *ForensicsEngine {
	fe := &ForensicsEngine{
		incidents:     make(map[string]*SecurityIncident),
		evidenceStore: make(map[string]*Evidence),
		timelines:     make(map[string]*Timeline),
		attackChains:  make(map[string]*AttackChain),
		reportDir:     reportDir,
		stopChan:      make(chan struct{}),
	}
	os.MkdirAll(reportDir, 0755)
	return fe
}

// Start 启动取证引擎
func (fe *ForensicsEngine) Start() {
	fe.mu.Lock()
	if fe.running {
		fe.mu.Unlock()
		return
	}
	fe.running = true
	fe.mu.Unlock()
	log.Println("[Forensics] 取证分析引擎已启动")
}

// Stop 停止取证引擎
func (fe *ForensicsEngine) Stop() {
	fe.mu.Lock()
	defer fe.mu.Unlock()
	if !fe.running {
		return
	}
	close(fe.stopChan)
	fe.running = false
	log.Println("[Forensics] 取证分析引擎已停止")
}

// ============================================================
// 事件管理
// ============================================================

// CreateIncident 创建安全事件
func (fe *ForensicsEngine) CreateIncident(title, description string, severity ThreatLevel, threatIDs []string) *SecurityIncident {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	incident := &SecurityIncident{
		ID:          uuid.New().String(),
		Title:       title,
		Description: description,
		Severity:    severity,
		Status:      "open",
		ThreatIDs:   threatIDs,
		Tags:        []string{"ransomware", "auto-detected"},
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}
	fe.incidents[incident.ID] = incident
	fe.stats.TotalIncidents++
	fe.stats.OpenIncidents++

	log.Printf("[Forensics] 安全事件已创建: ID=%s, Title=%s, Severity=%s",
		incident.ID, title, severity.String())
	return incident
}

// UpdateIncidentStatus 更新事件状态
func (fe *ForensicsEngine) UpdateIncidentStatus(id, status string) error {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	incident, ok := fe.incidents[id]
	if !ok {
		return fmt.Errorf("事件不存在: %s", id)
	}

	oldStatus := incident.Status
	incident.Status = status
	incident.UpdatedAt = time.Now()

	if status == "resolved" && oldStatus != "resolved" {
		now := time.Now()
		incident.ResolvedAt = &now
		fe.stats.ResolvedIncidents++
		fe.stats.OpenIncidents--
	}
	return nil
}

// GetIncident 获取事件
func (fe *ForensicsEngine) GetIncident(id string) (*SecurityIncident, bool) {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	inc, ok := fe.incidents[id]
	if !ok {
		return nil, false
	}
	result := *inc
	return &result, true
}

// ListIncidents 列出事件
func (fe *ForensicsEngine) ListIncidents(status string, limit int) []SecurityIncident {
	fe.mu.RLock()
	defer fe.mu.RUnlock()

	var result []SecurityIncident
	for _, inc := range fe.incidents {
		if status != "" && inc.Status != status {
			continue
		}
		result = append(result, *inc)
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	if limit > 0 && len(result) > limit {
		result = result[:limit]
	}
	return result
}

// ============================================================
// 证据收集
// ============================================================

// CollectEvidence 收集证据
func (fe *ForensicsEngine) CollectEvidence(incidentID, evidenceType, name, path, description string) (*Evidence, error) {
	var hash string
	var size int64

	if info, err := os.Stat(path); err == nil {
		size = info.Size()
		data, err := os.ReadFile(path)
		if err == nil {
			h := sha256.Sum256(data)
			hash = hex.EncodeToString(h[:])
		}
	}

	evidence := &Evidence{
		ID:          uuid.New().String(),
		IncidentID:  incidentID,
		Type:        evidenceType,
		Name:        name,
		Path:        path,
		Hash:        hash,
		SizeBytes:   size,
		Description: description,
		CollectedBy: "RansomShield-Auto",
		ChainOfCustody: []CustodyEntry{
			{Action: "collected", Officer: "RansomShield-Auto", Timestamp: time.Now(), Notes: "自动收集"},
		},
		CreatedAt: time.Now(),
	}

	fe.mu.Lock()
	fe.evidenceStore[evidence.ID] = evidence
	if inc, ok := fe.incidents[incidentID]; ok {
		inc.EvidenceIDs = append(inc.EvidenceIDs, evidence.ID)
		inc.UpdatedAt = time.Now()
	}
	fe.stats.TotalEvidence++
	fe.mu.Unlock()

	log.Printf("[Forensics] 证据已收集: ID=%s, Type=%s, Path=%s", evidence.ID, evidenceType, path)
	return evidence, nil
}

// CollectProcessEvidence 收集进程证据
func (fe *ForensicsEngine) CollectProcessEvidence(incidentID string, event ThreatEvent) *Evidence {
	evidence := &Evidence{
		ID:          uuid.New().String(),
		IncidentID:  incidentID,
		Type:        "process",
		Name:        fmt.Sprintf("process-%s-%d", event.ProcessName, event.ProcessID),
		Description: fmt.Sprintf("可疑进程信息: %s (PID: %d)", event.ProcessName, event.ProcessID),
		CollectedBy: "RansomShield-Auto",
		ChainOfCustody: []CustodyEntry{
			{Action: "collected", Officer: "RansomShield-Auto", Timestamp: time.Now()},
		},
		CreatedAt: time.Now(),
	}

	detailPath := filepath.Join(fe.reportDir, fmt.Sprintf("process_%s_%d.json", evidence.ID[:8], event.ProcessID))
	details := map[string]interface{}{
		"process_name": event.ProcessName,
		"process_id":   event.ProcessID,
		"source_path":  event.SourcePath,
		"threat_level": event.Level.String(),
		"score":        event.Score,
		"indicators":   event.Indicators,
		"timestamp":    event.CreatedAt,
	}
	data, _ := json.MarshalIndent(details, "", "  ")
	os.WriteFile(detailPath, data, 0644)

	evidence.Path = detailPath
	h := sha256.Sum256(data)
	evidence.Hash = hex.EncodeToString(h[:])
	evidence.SizeBytes = int64(len(data))

	fe.mu.Lock()
	fe.evidenceStore[evidence.ID] = evidence
	if inc, ok := fe.incidents[incidentID]; ok {
		inc.EvidenceIDs = append(inc.EvidenceIDs, evidence.ID)
	}
	fe.stats.TotalEvidence++
	fe.mu.Unlock()

	return evidence
}

// CollectFileEvidence 收集文件证据（隔离副本）
func (fe *ForensicsEngine) CollectFileEvidence(incidentID, filePath string) *Evidence {
	data, err := os.ReadFile(filePath)
	if err != nil {
		log.Printf("[Forensics] 读取文件失败: %s, %v", filePath, err)
		return nil
	}

	evidenceName := fmt.Sprintf("file_%s_%s", uuid.New().String()[:8], filepath.Base(filePath))
	evidencePath := filepath.Join(fe.reportDir, "evidence", evidenceName)
	os.MkdirAll(filepath.Dir(evidencePath), 0700)
	os.WriteFile(evidencePath, data, 0400)

	h := sha256.Sum256(data)

	evidence := &Evidence{
		ID:          uuid.New().String(),
		IncidentID:  incidentID,
		Type:        "file",
		Name:        filepath.Base(filePath),
		Path:        evidencePath,
		Hash:        hex.EncodeToString(h[:]),
		SizeBytes:   int64(len(data)),
		Description: fmt.Sprintf("可疑文件副本: %s", filePath),
		CollectedBy: "RansomShield-Auto",
		ChainOfCustody: []CustodyEntry{
			{Action: "collected", Officer: "RansomShield-Auto", Timestamp: time.Now(), Notes: fmt.Sprintf("原始路径: %s", filePath)},
		},
		CreatedAt: time.Now(),
	}

	fe.mu.Lock()
	fe.evidenceStore[evidence.ID] = evidence
	if inc, ok := fe.incidents[incidentID]; ok {
		inc.EvidenceIDs = append(inc.EvidenceIDs, evidence.ID)
	}
	fe.stats.TotalEvidence++
	fe.mu.Unlock()

	return evidence
}

// ============================================================
// 时间线构建
// ============================================================

// BuildTimeline 构建事件时间线
func (fe *ForensicsEngine) BuildTimeline(incidentID string, events []FileOpRecord, threats []ThreatEvent) *Timeline {
	var timelineEvents []TimelineEvent

	for _, op := range events {
		evt := TimelineEvent{
			ID:          uuid.New().String(),
			Timestamp:   op.Timestamp,
			Category:    "file",
			Description: fmt.Sprintf("文件%s: %s", op.OpType, op.Path),
			Source:      fmt.Sprintf("%s(%d)", op.ProcessName, op.ProcessID),
			Severity:    ThreatLevelLow,
			Confidence:  0.9,
			Details: map[string]interface{}{
				"operation": op.OpType,
				"path":      op.Path,
				"size":      op.Size,
				"entropy":   op.Entropy,
			},
		}

		switch op.OpType {
		case "read":
			evt.Phase = AttackPhaseRecon
		case "write":
			if op.Entropy > 7.0 {
				evt.Phase = AttackPhaseEncrypt
				evt.Severity = ThreatLevelHigh
			} else {
				evt.Phase = AttackPhaseExecute
			}
		case "delete":
			evt.Phase = AttackPhaseExecute
			evt.Severity = ThreatLevelMedium
		case "rename":
			if extractExt(op.OldPath) != extractExt(op.Path) {
				evt.Phase = AttackPhaseEncrypt
				evt.Severity = ThreatLevelHigh
				evt.Description = fmt.Sprintf("扩展名变更: %s -> %s", op.OldPath, op.Path)
			} else {
				evt.Phase = AttackPhaseExecute
			}
		}
		timelineEvents = append(timelineEvents, evt)
	}

	for _, threat := range threats {
		evt := TimelineEvent{
			ID:          uuid.New().String(),
			Timestamp:   threat.CreatedAt,
			Phase:       threat.Phase,
			Category:    "threat",
			Description: fmt.Sprintf("威胁检测: %v (评分: %d)", threat.Indicators, threat.Score),
			Source:      fmt.Sprintf("Rule:%s", threat.RuleID),
			Severity:    threat.Level,
			Confidence:  threat.Confidence,
		}
		timelineEvents = append(timelineEvents, evt)
	}

	sort.Slice(timelineEvents, func(i, j int) bool {
		return timelineEvents[i].Timestamp.Before(timelineEvents[j].Timestamp)
	})

	var startTime, endTime time.Time
	if len(timelineEvents) > 0 {
		startTime = timelineEvents[0].Timestamp
		endTime = timelineEvents[len(timelineEvents)-1].Timestamp
	}

	timeline := &Timeline{
		ID:         uuid.New().String(),
		IncidentID: incidentID,
		Events:     timelineEvents,
		StartTime:  startTime,
		EndTime:    endTime,
		CreatedAt:  time.Now(),
	}

	fe.mu.Lock()
	fe.timelines[timeline.ID] = timeline
	if inc, ok := fe.incidents[incidentID]; ok {
		inc.TimelineID = timeline.ID
	}
	fe.stats.TotalTimelines++
	fe.mu.Unlock()

	log.Printf("[Forensics] 时间线已构建: ID=%s, 事件数=%d", timeline.ID, len(timelineEvents))
	return timeline
}

// ============================================================
// 攻击链分析
// ============================================================

// AnalyzeAttackChain 分析攻击链
func (fe *ForensicsEngine) AnalyzeAttackChain(incidentID string, timeline *Timeline) *AttackChain {
	if timeline == nil || len(timeline.Events) == 0 {
		return nil
	}

	phaseEvents := make(map[AttackPhase][]TimelineEvent)
	for _, evt := range timeline.Events {
		phaseEvents[evt.Phase] = append(phaseEvents[evt.Phase], evt)
	}

	var phases []ChainPhase
	phaseOrder := []AttackPhase{
		AttackPhaseRecon, AttackPhaseDelivery, AttackPhaseExecute,
		AttackPhaseEncrypt, AttackPhaseExfil, AttackPhaseRansom,
	}

	for _, phase := range phaseOrder {
		events, ok := phaseEvents[phase]
		if !ok || len(events) == 0 {
			continue
		}

		seen := make(map[string]bool)
		var indicators []string
		for _, evt := range events {
			if !seen[evt.Description] {
				indicators = append(indicators, evt.Description)
				seen[evt.Description] = true
			}
		}

		phases = append(phases, ChainPhase{
			Phase:       phase,
			Description: fmt.Sprintf("检测到 %d 个%s阶段事件", len(events), phase),
			Indicators:  indicators,
			StartTime:   events[0].Timestamp,
			EndTime:     events[len(events)-1].Timestamp,
			Confidence:  calculatePhaseConfidence(events),
		})
	}

	// 识别勒索软件家族
	family := identifyRansomwareFamily(timeline)

	// MITRE ATT&CK TTPs
	ttps := identifyTTPs(phases)

	var startTime, endTime time.Time
	if len(phases) > 0 {
		startTime = phases[0].StartTime
		endTime = phases[len(phases)-1].EndTime
	}

	chain := &AttackChain{
		ID:         uuid.New().String(),
		IncidentID: incidentID,
		Name:       fmt.Sprintf("攻击链分析 - %s", family),
		Family:     family,
		Phases:     phases,
		TTPs:       ttps,
		Confidence: calculateChainConfidence(phases),
		StartTime:  startTime,
		EndTime:    endTime,
		CreatedAt:  time.Now(),
	}

	fe.mu.Lock()
	fe.attackChains[chain.ID] = chain
	if inc, ok := fe.incidents[incidentID]; ok {
		inc.AttackChainID = chain.ID
	}
	fe.mu.Unlock()

	log.Printf("[Forensics] 攻击链已分析: ID=%s, 家族=%s, 阶段=%d, TTPs=%v",
		chain.ID, family, len(phases), ttps)
	return chain
}

// calculatePhaseConfidence 计算阶段置信度
func calculatePhaseConfidence(events []TimelineEvent) float64 {
	if len(events) == 0 {
		return 0
	}
	var total float64
	for _, evt := range events {
		total += evt.Confidence
	}
	avg := total / float64(len(events))
	// 事件越多，置信度越高
	boost := float64(len(events)) * 0.02
	if boost > 0.1 {
		boost = 0.1
	}
	confidence := avg + boost
	if confidence > 1.0 {
		confidence = 1.0
	}
	return confidence
}

// calculateChainConfidence 计算攻击链总体置信度
func calculateChainConfidence(phases []ChainPhase) float64 {
	if len(phases) == 0 {
		return 0
	}
	var total float64
	for _, p := range phases {
		total += p.Confidence
	}
	avg := total / float64(len(phases))
	// 检测到的阶段越多，置信度越高
	phaseBoost := float64(len(phases)) * 0.05
	if phaseBoost > 0.2 {
		phaseBoost = 0.2
	}
	confidence := avg + phaseBoost
	if confidence > 1.0 {
		confidence = 1.0
	}
	return confidence
}

// identifyRansomwareFamily 识别勒索软件家族
func identifyRansomwareFamily(timeline *Timeline) string {
	familyKeywords := map[string][]string{
		"WannaCry":         {"wncry", "wannacry", "wanacry", "wcry"},
		"Ryuk":             {"ryuk", "ryk"},
		"Conti":            {"conti"},
		"LockBit":          {"lockbit", "lock bit"},
		"REvil/Sodinokibi": {"revil", "sodinokibi"},
		"Maze":             {"maze"},
		"BlackCat/ALPHV":   {"blackcat", "alphv"},
		"Cerber":           {"cerber"},
		"Petya/NotPetya":   {"petya", "notpetya", "goldeneye"},
	}

	// 收集所有文本描述
	var texts []string
	for _, evt := range timeline.Events {
		texts = append(texts, strings.ToLower(evt.Description))
		texts = append(texts, strings.ToLower(evt.Source))
	}
	allText := strings.Join(texts, " ")

	for family, keywords := range familyKeywords {
		for _, kw := range keywords {
			if strings.Contains(allText, kw) {
				return family
			}
		}
	}

	// 根据攻击模式推断
	for _, evt := range timeline.Events {
		if evt.Phase == AttackPhaseEncrypt && evt.Severity >= ThreatLevelHigh {
			return "未知勒索软件"
		}
	}

	return "未知"
}

// identifyTTPs 识别 MITRE ATT&CK TTPs
func identifyTTPs(phases []ChainPhase) []string {
	var ttps []string
	seen := make(map[string]bool)

	for _, phase := range phases {
		var ttp string
		switch phase.Phase {
		case AttackPhaseRecon:
			ttp = "T1595 - Active Scanning"
		case AttackPhaseDelivery:
			ttp = "T1566 - Phishing"
		case AttackPhaseExecute:
			ttp = "T1059 - Command and Scripting Interpreter"
		case AttackPhaseEncrypt:
			ttp = "T1486 - Data Encrypted for Impact"
		case AttackPhaseExfil:
			ttp = "T1041 - Exfiltration Over C2 Channel"
		case AttackPhaseRansom:
			ttp = "T1657 - Financial Theft"
		}

		if ttp != "" && !seen[ttp] {
			ttps = append(ttps, ttp)
			seen[ttp] = true
		}

		// 根据指标细化 TTP
		for _, ind := range phase.Indicators {
			indLower := strings.ToLower(ind)
			switch {
			case strings.Contains(indLower, "shadow") || strings.Contains(indLower, "vssadmin"):
				if !seen["T1490"] {
					ttps = append(ttps, "T1490 - Inhibit System Recovery")
					seen["T1490"] = true
				}
			case strings.Contains(indLower, "network") || strings.Contains(indLower, "smb"):
				if !seen["T1021"] {
					ttps = append(ttps, "T1021 - Remote Services")
					seen["T1021"] = true
				}
			}
		}
	}
	return ttps
}

// ============================================================
// 取证报告生成
// ============================================================

// GenerateReport 生成取证报告
func (fe *ForensicsEngine) GenerateReport(incidentID string) (*ForensicsReport, error) {
	fe.mu.RLock()
	incident, ok := fe.incidents[incidentID]
	if !ok {
		fe.mu.RUnlock()
		return nil, fmt.Errorf("事件不存在: %s", incidentID)
	}

	// 收集关联的证据
	var evidenceList []Evidence
	for _, eid := range incident.EvidenceIDs {
		if ev, ok := fe.evidenceStore[eid]; ok {
			evidenceList = append(evidenceList, *ev)
		}
	}

	// 获取时间线
	var timeline *Timeline
	if incident.TimelineID != "" {
		timeline = fe.timelines[incident.TimelineID]
	}

	// 获取攻击链
	var attackChain *AttackChain
	if incident.AttackChainID != "" {
		attackChain = fe.attackChains[incident.AttackChainID]
	}
	fe.mu.RUnlock()

	// 生成建议
	recommendations := generateRecommendations(incident, attackChain)

	report := &ForensicsReport{
		ID:              uuid.New().String(),
		IncidentID:      incidentID,
		Title:           fmt.Sprintf("取证报告 - %s", incident.Title),
		Summary:         generateSummary(incident, attackChain),
		Timeline:        timeline,
		AttackChain:     attackChain,
		Evidence:        evidenceList,
		Recommendations: recommendations,
		GeneratedAt:     time.Now(),
		GeneratedBy:     "RansomShield-Forensics",
		Format:          "json",
	}

	// 保存报告
	reportPath := filepath.Join(fe.reportDir, fmt.Sprintf("report_%s_%s.json",
		incidentID[:8], time.Now().Format("20060102_150405")))
	data, _ := json.MarshalIndent(report, "", "  ")
	os.WriteFile(reportPath, data, 0644)

	fe.mu.Lock()
	fe.stats.TotalReports++
	fe.stats.LastReportTime = time.Now()
	fe.mu.Unlock()

	log.Printf("[Forensics] 取证报告已生成: %s (事件: %s)", report.ID, incidentID)
	return report, nil
}

// generateSummary 生成报告摘要
func generateSummary(incident *SecurityIncident, chain *AttackChain) string {
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("安全事件: %s\n", incident.Title))
	sb.WriteString(fmt.Sprintf("严重级别: %s\n", incident.Severity.String()))
	sb.WriteString(fmt.Sprintf("事件状态: %s\n", incident.Status))

	if chain != nil {
		sb.WriteString(fmt.Sprintf("攻击家族: %s\n", chain.Family))
		sb.WriteString(fmt.Sprintf("攻击阶段: %d\n", len(chain.Phases)))
		sb.WriteString(fmt.Sprintf("分析置信度: %.0f%%\n", chain.Confidence*100))
	}

	sb.WriteString(fmt.Sprintf("关联证据: %d 份\n", len(incident.EvidenceIDs)))
	sb.WriteString(fmt.Sprintf("描述: %s", incident.Description))
	return sb.String()
}

// generateRecommendations 生成响应建议
func generateRecommendations(incident *SecurityIncident, chain *AttackChain) []string {
	var recs []string

	// 通用建议
	recs = append(recs, "立即隔离受影响系统，防止威胁横向扩散")
	recs = append(recs, "验证最近的恢复点，确保备份数据未被加密")

	if incident.Severity >= ThreatLevelHigh {
		recs = append(recs, "评估数据泄露风险，检查是否有数据外传迹象")
		recs = append(recs, "通知安全团队和管理层")
	}

	if chain != nil {
		for _, phase := range chain.Phases {
			switch phase.Phase {
			case AttackPhaseRecon:
				recs = append(recs, "加固网络边界，检查防火墙和访问控制策略")
			case AttackPhaseEncrypt:
				recs = append(recs, "使用恢复点回滚受影响文件系统")
				recs = append(recs, "扫描所有系统检测残留恶意软件")
			case AttackPhaseExfil:
				recs = append(recs, "评估数据泄露影响范围，准备合规报告")
			case AttackPhaseRansom:
				recs = append(recs, "不要支付赎金，联系执法部门和安全顾问")
			}
		}
	}

	// 后续措施
	recs = append(recs, "事后分析攻击路径，修补安全漏洞")
	recs = append(recs, "更新威胁情报库，增强检测能力")
	recs = append(recs, "进行安全培训，提高用户安全意识")

	return recs
}

// ============================================================
// 查询接口
// ============================================================

// GetStats 获取统计信息
func (fe *ForensicsEngine) GetStats() ForensicsStats {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	stats := fe.stats
	stats.TotalEvidence = len(fe.evidenceStore)
	stats.TotalTimelines = len(fe.timelines)
	return stats
}

// GetEvidence 获取证据
func (fe *ForensicsEngine) GetEvidence(id string) (*Evidence, bool) {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	ev, ok := fe.evidenceStore[id]
	if !ok {
		return nil, false
	}
	result := *ev
	return &result, true
}

// GetTimeline 获取时间线
func (fe *ForensicsEngine) GetTimeline(id string) (*Timeline, bool) {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	tl, ok := fe.timelines[id]
	if !ok {
		return nil, false
	}
	result := *tl
	return &result, true
}

// GetAttackChain 获取攻击链
func (fe *ForensicsEngine) GetAttackChain(id string) (*AttackChain, bool) {
	fe.mu.RLock()
	defer fe.mu.RUnlock()
	ac, ok := fe.attackChains[id]
	if !ok {
		return nil, false
	}
	result := *ac
	return &result, true
}

// AddCustodyEntry 添加证据保管链记录
func (fe *ForensicsEngine) AddCustodyEntry(evidenceID, action, officer, notes string) error {
	fe.mu.Lock()
	defer fe.mu.Unlock()

	ev, ok := fe.evidenceStore[evidenceID]
	if !ok {
		return fmt.Errorf("证据不存在: %s", evidenceID)
	}

	entry := CustodyEntry{
		Action:    action,
		Officer:   officer,
		Timestamp: time.Now(),
		Notes:     notes,
	}
	ev.ChainOfCustody = append(ev.ChainOfCustody, entry)
	return nil
}
