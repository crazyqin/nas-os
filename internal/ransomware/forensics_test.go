package ransomware

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestNewForensicsEngine(t *testing.T) {
	reportDir := t.TempDir() + "/reports"
	fe := NewForensicsEngine(reportDir)
	if fe == nil {
		t.Fatal("NewForensicsEngine returned nil")
	}

	// 验证报告目录已创建
	if _, err := os.Stat(reportDir); os.IsNotExist(err) {
		t.Error("report directory should be created")
	}
}

func TestForensicsEngine_CreateIncident(t *testing.T) {
	fe := NewForensicsEngine(t.TempDir())

	incident := fe.CreateIncident(
		"勒索软件攻击",
		"检测到快速加密行为",
		ThreatLevelCritical,
		[]string{"threat-1", "threat-2"},
	)

	if incident == nil {
		t.Fatal("CreateIncident returned nil")
	}

	if incident.Title != "勒索软件攻击" {
		t.Errorf("expected title '勒索软件攻击', got '%s'", incident.Title)
	}

	if incident.Status != "open" {
		t.Errorf("expected status 'open', got '%s'", incident.Status)
	}

	if len(incident.ThreatIDs) != 2 {
		t.Errorf("expected 2 threat IDs, got %d", len(incident.ThreatIDs))
	}
}

func TestForensicsEngine_UpdateIncidentStatus(t *testing.T) {
	fe := NewForensicsEngine(t.TempDir())

	incident := fe.CreateIncident("Test", "Desc", ThreatLevelHigh, nil)

	err := fe.UpdateIncidentStatus(incident.ID, "investigating")
	if err != nil {
		t.Fatalf("UpdateIncidentStatus failed: %v", err)
	}

	inc, _ := fe.GetIncident(incident.ID)
	if inc.Status != "investigating" {
		t.Errorf("expected status 'investigating', got '%s'", inc.Status)
	}

	// 解决事件
	fe.UpdateIncidentStatus(incident.ID, "resolved")
	inc, _ = fe.GetIncident(incident.ID)
	if inc.ResolvedAt == nil {
		t.Error("ResolvedAt should be set for resolved incident")
	}

	stats := fe.GetStats()
	if stats.ResolvedIncidents != 1 {
		t.Errorf("expected 1 resolved incident, got %d", stats.ResolvedIncidents)
	}
}

func TestForensicsEngine_UpdateIncidentStatus_NotFound(t *testing.T) {
	fe := NewForensicsEngine(t.TempDir())

	err := fe.UpdateIncidentStatus("nonexistent", "resolved")
	if err == nil {
		t.Error("expected error for non-existent incident")
	}
}

func TestForensicsEngine_ListIncidents(t *testing.T) {
	fe := NewForensicsEngine(t.TempDir())

	fe.CreateIncident("Incident 1", "Desc", ThreatLevelHigh, nil)
	fe.CreateIncident("Incident 2", "Desc", ThreatLevelCritical, nil)
	fe.CreateIncident("Incident 3", "Desc", ThreatLevelMedium, nil)

	all := fe.ListIncidents("", 0)
	if len(all) != 3 {
		t.Errorf("expected 3 incidents, got %d", len(all))
	}

	limited := fe.ListIncidents("", 2)
	if len(limited) != 2 {
		t.Errorf("expected 2 incidents with limit, got %d", len(limited))
	}
}

func TestForensicsEngine_CollectEvidence(t *testing.T) {
	tmpDir := t.TempDir()
	fe := NewForensicsEngine(tmpDir)

	incident := fe.CreateIncident("Test", "Desc", ThreatLevelHigh, nil)

	// 创建测试文件
	testFile := tmpDir + "/evidence.txt"
	os.WriteFile(testFile, []byte("evidence data"), 0644)

	evidence, err := fe.CollectEvidence(incident.ID, "file", "证据文件", testFile, "测试证据")
	if err != nil {
		t.Fatalf("CollectEvidence failed: %v", err)
	}

	if evidence.Hash == "" {
		t.Error("expected hash to be computed")
	}

	if evidence.SizeBytes == 0 {
		t.Error("expected file size > 0")
	}

	// 验证关联到事件
	inc, _ := fe.GetIncident(incident.ID)
	if len(inc.EvidenceIDs) == 0 {
		t.Error("expected evidence to be linked to incident")
	}
}

func TestForensicsEngine_CollectFileEvidence(t *testing.T) {
	tmpDir := t.TempDir()
	fe := NewForensicsEngine(tmpDir)

	incident := fe.CreateIncident("Test", "Desc", ThreatLevelHigh, nil)

	// 创建可疑文件
	suspectFile := tmpDir + "/malware.exe"
	os.WriteFile(suspectFile, []byte("malicious content"), 0644)

	evidence := fe.CollectFileEvidence(incident.ID, suspectFile)
	if evidence == nil {
		t.Fatal("CollectFileEvidence returned nil")
	}

	if evidence.Type != "file" {
		t.Errorf("expected type 'file', got '%s'", evidence.Type)
	}

	// 验证证据副本存在
	if _, err := os.Stat(evidence.Path); os.IsNotExist(err) {
		t.Error("evidence copy should exist")
	}

	// 验证原始文件仍然存在（副本模式）
	if _, err := os.Stat(suspectFile); os.IsNotExist(err) {
		t.Error("original file should still exist (copy mode)")
	}
}

func TestForensicsEngine_CollectProcessEvidence(t *testing.T) {
	tmpDir := t.TempDir()
	fe := NewForensicsEngine(tmpDir)

	incident := fe.CreateIncident("Test", "Desc", ThreatLevelCritical, nil)

	threat := ThreatEvent{
		ID:          "threat-1",
		Level:       ThreatLevelCritical,
		ProcessName: "ransomware",
		ProcessID:   1234,
		SourcePath:  "/data/encrypted.txt",
		Score:       95,
		Indicators:  []string{"high-entropy", "rapid-write"},
		CreatedAt:   time.Now(),
	}

	evidence := fe.CollectProcessEvidence(incident.ID, threat)
	if evidence == nil {
		t.Fatal("CollectProcessEvidence returned nil")
	}

	if evidence.Type != "process" {
		t.Errorf("expected type 'process', got '%s'", evidence.Type)
	}

	// 验证详情文件已创建
	if _, err := os.Stat(evidence.Path); os.IsNotExist(err) {
		t.Error("process detail file should exist")
	}
}

func TestForensicsEngine_BuildTimeline(t *testing.T) {
	fe := NewForensicsEngine(t.TempDir())
	incident := fe.CreateIncident("Test", "Desc", ThreatLevelCritical, nil)

	now := time.Now()
	events := []FileOpRecord{
		{Path: "/data/a.txt", OpType: "read", ProcessName: "proc", ProcessID: 1, Timestamp: now.Add(-10 * time.Minute)},
		{Path: "/data/b.txt", OpType: "write", Size: 4096, Entropy: 7.8, ProcessName: "ransom", ProcessID: 2, Timestamp: now.Add(-5 * time.Minute)},
		{Path: "/data/a.txt.encrypted", OldPath: "/data/a.txt", OpType: "rename", ProcessName: "ransom", ProcessID: 2, Timestamp: now.Add(-3 * time.Minute)},
	}

	threats := []ThreatEvent{
		{
			ID: "t1", RuleID: "rapid-encryption", Level: ThreatLevelCritical,
			Phase: AttackPhaseEncrypt, Score: 95, Confidence: 0.95,
			CreatedAt: now.Add(-2 * time.Minute),
		},
	}

	timeline := fe.BuildTimeline(incident.ID, events, threats)
	if timeline == nil {
		t.Fatal("BuildTimeline returned nil")
	}

	if len(timeline.Events) != 4 {
		t.Errorf("expected 4 timeline events, got %d", len(timeline.Events))
	}

	// 验证时间排序
	for i := 1; i < len(timeline.Events); i++ {
		if timeline.Events[i].Timestamp.Before(timeline.Events[i-1].Timestamp) {
			t.Error("timeline events should be sorted by time")
			break
		}
	}
}

func TestForensicsEngine_AnalyzeAttackChain(t *testing.T) {
	fe := NewForensicsEngine(t.TempDir())
	incident := fe.CreateIncident("Test", "Desc", ThreatLevelCritical, nil)

	now := time.Now()
	timeline := &Timeline{
		ID:         "tl-1",
		IncidentID: incident.ID,
		Events: []TimelineEvent{
			{Phase: AttackPhaseRecon, Description: "文件侦察", Timestamp: now.Add(-30 * time.Minute), Severity: ThreatLevelLow, Confidence: 0.9},
			{Phase: AttackPhaseExecute, Description: "恶意代码执行", Timestamp: now.Add(-20 * time.Minute), Severity: ThreatLevelHigh, Confidence: 0.85},
			{Phase: AttackPhaseEncrypt, Description: "文件加密", Timestamp: now.Add(-10 * time.Minute), Severity: ThreatLevelCritical, Confidence: 0.95},
			{Phase: AttackPhaseEncrypt, Description: "勒索信创建", Timestamp: now.Add(-5 * time.Minute), Severity: ThreatLevelCritical, Confidence: 0.9},
		},
	}

	chain := fe.AnalyzeAttackChain(incident.ID, timeline)
	if chain == nil {
		t.Fatal("AnalyzeAttackChain returned nil")
	}

	if len(chain.Phases) < 2 {
		t.Errorf("expected at least 2 phases, got %d", len(chain.Phases))
	}

	if chain.Confidence <= 0 {
		t.Error("expected confidence > 0")
	}

	if len(chain.TTPs) == 0 {
		t.Error("expected at least one TTP")
	}
}

func TestForensicsEngine_GenerateReport(t *testing.T) {
	tmpDir := t.TempDir()
	fe := NewForensicsEngine(tmpDir)

	incident := fe.CreateIncident("勒索攻击报告", "全面勒索软件攻击", ThreatLevelCritical, []string{"t1"})

	// 收集证据
	testFile := tmpDir + "/evidence.txt"
	os.WriteFile(testFile, []byte("evidence"), 0644)
	fe.CollectEvidence(incident.ID, "file", "证据1", testFile, "测试")

	// 构建时间线
	timeline := fe.BuildTimeline(incident.ID, []FileOpRecord{
		{Path: "/data/a.txt", OpType: "write", Entropy: 7.8, ProcessName: "ransom", ProcessID: 1, Timestamp: time.Now()},
	}, nil)

	// 分析攻击链
	fe.AnalyzeAttackChain(incident.ID, timeline)

	report, err := fe.GenerateReport(incident.ID)
	if err != nil {
		t.Fatalf("GenerateReport failed: %v", err)
	}

	if report.Title == "" {
		t.Error("report title should not be empty")
	}

	if report.Summary == "" {
		t.Error("report summary should not be empty")
	}

	if len(report.Recommendations) == 0 {
		t.Error("expected recommendations")
	}

	if len(report.Evidence) == 0 {
		t.Error("expected evidence in report")
	}

	// 验证报告文件已保存
	reportDir := filepath.Join(tmpDir)
	files, _ := os.ReadDir(reportDir)
	reportFound := false
	for _, f := range files {
		if filepath.Ext(f.Name()) == ".json" && len(f.Name()) > 10 {
			reportFound = true
		}
	}
	if !reportFound {
		t.Error("report file should be saved to disk")
	}
}

func TestForensicsEngine_AddCustodyEntry(t *testing.T) {
	tmpDir := t.TempDir()
	fe := NewForensicsEngine(tmpDir)

	incident := fe.CreateIncident("Test", "Desc", ThreatLevelHigh, nil)

	testFile := tmpDir + "/evidence.txt"
	os.WriteFile(testFile, []byte("data"), 0644)

	evidence, _ := fe.CollectEvidence(incident.ID, "file", "证据", testFile, "")

	err := fe.AddCustodyEntry(evidence.ID, "analyzed", "安全分析师", "已完成分析")
	if err != nil {
		t.Fatalf("AddCustodyEntry failed: %v", err)
	}

	ev, _ := fe.GetEvidence(evidence.ID)
	if len(ev.ChainOfCustody) != 2 {
		t.Errorf("expected 2 custody entries, got %d", len(ev.ChainOfCustody))
	}
}

func TestForensicsEngine_GetStats(t *testing.T) {
	fe := NewForensicsEngine(t.TempDir())

	fe.CreateIncident("Inc1", "Desc", ThreatLevelHigh, nil)
	fe.CreateIncident("Inc2", "Desc", ThreatLevelCritical, nil)

	stats := fe.GetStats()
	if stats.TotalIncidents != 2 {
		t.Errorf("expected 2 total incidents, got %d", stats.TotalIncidents)
	}

	if stats.OpenIncidents != 2 {
		t.Errorf("expected 2 open incidents, got %d", stats.OpenIncidents)
	}
}

func TestForensicsEngine_GetEvidence_NotFound(t *testing.T) {
	fe := NewForensicsEngine(t.TempDir())

	_, found := fe.GetEvidence("nonexistent")
	if found {
		t.Error("expected false for non-existent evidence")
	}
}

func TestForensicsEngine_GetTimeline_NotFound(t *testing.T) {
	fe := NewForensicsEngine(t.TempDir())

	_, found := fe.GetTimeline("nonexistent")
	if found {
		t.Error("expected false for non-existent timeline")
	}
}

func TestForensicsEngine_GetAttackChain_NotFound(t *testing.T) {
	fe := NewForensicsEngine(t.TempDir())

	_, found := fe.GetAttackChain("nonexistent")
	if found {
		t.Error("expected false for non-existent attack chain")
	}
}

func TestForensicsEngine_GenerateReport_NotFound(t *testing.T) {
	fe := NewForensicsEngine(t.TempDir())

	_, err := fe.GenerateReport("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent incident")
	}
}

func TestIdentifyRansomwareFamily(t *testing.T) {
	tests := []struct {
		name     string
		events   []TimelineEvent
		expected string
	}{
		{
			name: "WannaCry indicators",
			events: []TimelineEvent{
				{Description: "检测到 wncry 扩展名"},
			},
			expected: "WannaCry",
		},
		{
			name: "Ryuk indicators",
			events: []TimelineEvent{
				{Description: "发现 RyukReadMe.txt"},
			},
			expected: "Ryuk",
		},
		{
			name: "Unknown ransomware",
			events: []TimelineEvent{
				{Phase: AttackPhaseEncrypt, Description: "encrypted files", Severity: ThreatLevelHigh},
			},
			expected: "未知勒索软件",
		},
		{
			name:     "No indicators",
			events:   []TimelineEvent{},
			expected: "未知",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tl := &Timeline{Events: tt.events}
			got := identifyRansomwareFamily(tl)
			if got != tt.expected {
				t.Errorf("identifyRansomwareFamily() = %q, want %q", got, tt.expected)
			}
		})
	}
}

func TestIdentifyTTPs(t *testing.T) {
	phases := []ChainPhase{
		{Phase: AttackPhaseRecon},
		{Phase: AttackPhaseEncrypt},
		{Phase: AttackPhaseExfil},
	}

	ttps := identifyTTPs(phases)
	if len(ttps) == 0 {
		t.Error("expected TTPs to be identified")
	}

	// 验证包含 Data Encrypted for Impact
	found := false
	for _, ttp := range ttps {
		if ttp == "T1486 - Data Encrypted for Impact" {
			found = true
		}
	}
	if !found {
		t.Error("expected T1486 TTP for encryption phase")
	}
}

func TestCalculatePhaseConfidence(t *testing.T) {
	events := []TimelineEvent{
		{Confidence: 0.9},
		{Confidence: 0.8},
		{Confidence: 0.95},
	}

	conf := calculatePhaseConfidence(events)
	if conf <= 0 || conf > 1.0 {
		t.Errorf("confidence should be between 0 and 1, got %f", conf)
	}

	// 空事件应该返回 0
	emptyConf := calculatePhaseConfidence(nil)
	if emptyConf != 0 {
		t.Errorf("expected 0 for empty events, got %f", emptyConf)
	}
}

func TestGenerateRecommendations(t *testing.T) {
	incident := &SecurityIncident{
		Severity: ThreatLevelCritical,
	}

	chain := &AttackChain{
		Phases: []ChainPhase{
			{Phase: AttackPhaseEncrypt},
			{Phase: AttackPhaseExfil},
		},
	}

	recs := generateRecommendations(incident, chain)
	if len(recs) == 0 {
		t.Error("expected recommendations")
	}

	// 验证包含恢复建议
	found := false
	for _, rec := range recs {
		if containsStr := rec != ""; containsStr {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected non-empty recommendations")
	}
}
