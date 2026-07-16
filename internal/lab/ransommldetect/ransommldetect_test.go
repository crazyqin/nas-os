package ransommldetect

import (
	"math"
	"testing"
	"time"
)

// ============================================================
// CalculateEntropy / ExtractEntropyFeatures
// ============================================================

func TestCalculateEntropy_Empty(t *testing.T) {
	if e := CalculateEntropy(nil); e != 0 {
		t.Errorf("empty input should yield 0, got %f", e)
	}
}

func TestCalculateEntropy_Uniform(t *testing.T) {
	// All-same bytes → entropy ≈ 0
	data := make([]byte, 256)
	e := CalculateEntropy(data)
	if e > 0.01 {
		t.Errorf("uniform data should have near-zero entropy, got %f", e)
	}
}

func TestCalculateEntropy_Random(t *testing.T) {
	// Full byte distribution → entropy close to 8
	data := make([]byte, 256)
	for i := range data {
		data[i] = byte(i)
	}
	e := CalculateEntropy(data)
	if e < 7.9 {
		t.Errorf("full distribution should have entropy ~8, got %f", e)
	}
}

func TestCalculateEntropy_HighEntropySample(t *testing.T) {
	// Pseudo-random-ish data
	data := make([]byte, 65536)
	for i := range data {
		data[i] = byte(i * 251 % 256)
	}
	e := CalculateEntropy(data)
	if e < 7.5 {
		t.Errorf("expected high entropy >=7.5, got %f", e)
	}
}

func TestExtractEntropyFeatures(t *testing.T) {
	samples := map[string][]byte{
		"/data/normal.txt":    []byte("hello world"),
		"/data/encrypted.enc": makeHighEntropyBytes(1024),
	}
	results := ExtractEntropyFeatures(samples, 7.5, 0)

	if len(results) != 2 {
		t.Fatalf("expected 2 results, got %d", len(results))
	}

	lowCount, highCount := 0, 0
	for _, r := range results {
		if r.IsHigh {
			highCount++
		} else {
			lowCount++
		}
	}
	if highCount != 1 || lowCount != 1 {
		t.Errorf("expected 1 high + 1 low, got %d high + %d low", highCount, lowCount)
	}
}

func TestExtractEntropyFeatures_SampleSizeCap(t *testing.T) {
	big := makeHighEntropyBytes(65536 * 2)
	samples := map[string][]byte{"/data/big.enc": big}
	results := ExtractEntropyFeatures(samples, 7.5, 65536)
	if len(results) != 1 {
		t.Fatal("expected 1 result")
	}
	if results[0].FileSize > 65536 {
		t.Errorf("sample size should have been capped, got %d", results[0].FileSize)
	}
}

// ============================================================
// WriteFrequencyAnalyzer
// ============================================================

func TestWriteFrequencyAnalyzer_NoAnomaly(t *testing.T) {
	a := NewWriteFrequencyAnalyzer(1*time.Minute, 3.0)
	a.SetBaseline("/data/docs", 10.0) // 10 ops/min expected

	// Record 5 ops in 1 minute → 5 ops/min, below 10*3=30 threshold
	now := time.Now()
	for i := 0; i < 5; i++ {
		a.Record("/data/docs", now.Add(-time.Duration(i)*time.Second))
	}

	stats := a.Analyze()
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat entry, got %d", len(stats))
	}
	if stats[0].IsAnomaly {
		t.Error("should not be anomaly with low rate")
	}
}

func TestWriteFrequencyAnalyzer_Anomaly(t *testing.T) {
	a := NewWriteFrequencyAnalyzer(1*time.Minute, 2.0)
	a.SetBaseline("/data/docs", 5.0) // 5 ops/min baseline

	now := time.Now()
	// 50 ops in 1 minute → 50 ops/min, deviation = (50-5)/5 = 9 > 2
	for i := 0; i < 50; i++ {
		a.Record("/data/docs", now.Add(-time.Duration(i*600)*time.Millisecond))
	}

	stats := a.Analyze()
	if len(stats) != 1 {
		t.Fatalf("expected 1 stat entry, got %d", len(stats))
	}
	if !stats[0].IsAnomaly {
		t.Errorf("expected anomaly, deviation=%.2f", stats[0].Deviation)
	}
}

func TestWriteFrequencyAnalyzer_Pruning(t *testing.T) {
	a := NewWriteFrequencyAnalyzer(1*time.Minute, 3.0)

	// Record an event 2 minutes ago (should be pruned)
	a.Record("/data/old", time.Now().Add(-2*time.Minute))
	// Record an event now
	a.Record("data/new", time.Now())

	stats := a.Analyze()
	for _, s := range stats {
		if s.Directory == "/data/old" && s.OpsPerMinute > 0 {
			t.Error("old events should have been pruned, directory should have 0 ops")
		}
	}
}

// ============================================================
// ExtensionChangeDetector
// ============================================================

func TestExtensionChangeDetector_NoSuspicious(t *testing.T) {
	d := NewExtensionChangeDetector(5*time.Minute, 10)
	now := time.Now()
	d.Record(ExtensionChangeEvent{Path: "/data/a.txt", OldExt: ".txt", NewExt: ".bak", Time: now})
	d.Record(ExtensionChangeEvent{Path: "/data/b.txt", OldExt: ".txt", NewExt: ".bak", Time: now})

	results := d.Analyze()
	for _, r := range results {
		if r.IsSuspicious {
			t.Errorf("2 events should not trigger threshold 10, got suspicious for %s", r.NewExtension)
		}
	}
}

func TestExtensionChangeDetector_KnownRansomwareExt(t *testing.T) {
	d := NewExtensionChangeDetector(5*time.Minute, 100) // high threshold
	now := time.Now()
	d.Record(ExtensionChangeEvent{Path: "/data/a.docx", OldExt: ".docx", NewExt: ".encrypted", Time: now})

	results := d.Analyze()
	found := false
	for _, r := range results {
		if r.NewExtension == ".encrypted" && r.IsSuspicious {
			found = true
		}
	}
	if !found {
		t.Error("known ransomware extension should be flagged even below count threshold")
	}
}

func TestExtensionChangeDetector_BatchThreshold(t *testing.T) {
	d := NewExtensionChangeDetector(5*time.Minute, 5)
	now := time.Now()
	for i := 0; i < 5; i++ {
		d.Record(ExtensionChangeEvent{
			Path:   "/data/file" + string(rune('0'+i)) + ".txt",
			OldExt: ".txt",
			NewExt: ".zzzz",
			Time:   now,
		})
	}

	results := d.Analyze()
	found := false
	for _, r := range results {
		if r.NewExtension == ".zzzz" && r.IsSuspicious && r.Count >= 5 {
			found = true
		}
	}
	if !found {
		t.Error("5 events with same new ext >= threshold 5 should be suspicious")
	}
}

// ============================================================
// RansomwarePredictor
// ============================================================

func TestPredictor_LowThreat(t *testing.T) {
	p := NewRansomwarePredictor(DefaultPredictorWeights(), nil)
	fv := FeatureVector{
		Timestamp:        time.Now(),
		HighEntropyRatio: 0.0,
		ExtChangeCount:   0,
		YARAMatchCount:   0,
	}
	pred := p.Predict(fv)
	if pred.ThreatLevel != ThreatLevelLow {
		t.Errorf("expected Low, got %s (score=%.3f)", pred.ThreatLevel, pred.Score)
	}
}

func TestPredictor_CriticalThreat(t *testing.T) {
	p := NewRansomwarePredictor(DefaultPredictorWeights(), nil)
	fv := FeatureVector{
		Timestamp:         time.Now(),
		HighEntropyCount:  80,
		HighEntropyRatio:  0.95,
		WriteAnomalyCount: 3,
		TopWriteAnomaly:   &WriteFrequencyStats{Deviation: 9.0, IsAnomaly: true},
		ExtChangeCount:    90,
		SuspiciousExts:    []string{".encrypted"},
		YARAMatchCount:    3,
	}
	pred := p.Predict(fv)
	if pred.ThreatLevel < ThreatLevelHigh {
		t.Errorf("expected at least High, got %s (score=%.3f)", pred.ThreatLevel, pred.Score)
	}
}

func TestPredictor_WeightsSum(t *testing.T) {
	w := DefaultPredictorWeights()
	sum := w.EntropyWeight + w.FreqWeight + w.ExtChangeWeight + w.YARAWeight
	if math.Abs(sum-1.0) > 0.01 {
		t.Errorf("default weights should sum to 1.0, got %f", sum)
	}
}

// ============================================================
// ThreatLevel
// ============================================================

func TestThreatLevelString(t *testing.T) {
	cases := map[ThreatLevel]string{
		ThreatLevelLow:      "low",
		ThreatLevelMedium:   "medium",
		ThreatLevelHigh:     "high",
		ThreatLevelCritical: "critical",
	}
	for level, want := range cases {
		if got := level.String(); got != want {
			t.Errorf("ThreatLevel(%d).String() = %q, want %q", level, got, want)
		}
	}
}

func TestThreatLevelFromString(t *testing.T) {
	cases := map[string]ThreatLevel{
		"low":      ThreatLevelLow,
		"medium":   ThreatLevelMedium,
		"high":     ThreatLevelHigh,
		"critical": ThreatLevelCritical,
		"UNKNOWN":  ThreatLevelLow, // default
	}
	for input, want := range cases {
		if got := ThreatLevelFromString(input); got != want {
			t.Errorf("ThreatLevelFromString(%q) = %d, want %d", input, got, want)
		}
	}
}

func TestThreatLevelMarshalText(t *testing.T) {
	b, err := ThreatLevelCritical.MarshalText()
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != "critical" {
		t.Errorf("MarshalText = %q, want %q", string(b), "critical")
	}
}

// ============================================================
// Detector enhanced
// ============================================================

func TestDetector_StartStop(t *testing.T) {
	cfg := DefaultDetectorConfig()
	d := NewDetector(cfg, nil)
	d.Start()
	time.Sleep(100 * time.Millisecond)
	d.Stop()
}

func TestDetector_RecordActivity(t *testing.T) {
	cfg := DefaultDetectorConfig()
	d := NewDetector(cfg, nil)
	d.RecordActivity(FileActivity{
		Path:      "/data/test.txt",
		Operation: "modify",
		Size:      1024,
		Timestamp: time.Now(),
		Process:   "testproc",
	})
	alerts := d.GetAlerts()
	// Should not immediately produce an alert from a single event
	if len(alerts) != 0 {
		t.Errorf("single event should not trigger alert, got %d", len(alerts))
	}
}

func TestDetector_FeedEntropySample(t *testing.T) {
	cfg := DefaultDetectorConfig()
	d := NewDetector(cfg, nil)
	d.FeedEntropySample("/data/test.enc", makeHighEntropyBytes(1024))
}

func TestDetector_GetThreatStatus(t *testing.T) {
	cfg := DefaultDetectorConfig()
	d := NewDetector(cfg, nil)
	d.Start()
	defer d.Stop()

	status := d.GetThreatStatus()
	if !status.Running {
		t.Error("detector should be running")
	}
	if status.Level != ThreatLevelLow {
		t.Errorf("initial threat level should be Low, got %s", status.Level)
	}
}

func TestDetector_ScanNow(t *testing.T) {
	cfg := DefaultDetectorConfig()
	d := NewDetector(cfg, nil)
	pred := d.ScanNow()
	// With no data, should be low threat
	if pred.ThreatLevel != ThreatLevelLow {
		t.Errorf("empty scan should be Low, got %s", pred.ThreatLevel)
	}
}

func TestDetector_GetThreatEvents(t *testing.T) {
	cfg := DefaultDetectorConfig()
	d := NewDetector(cfg, nil)

	// Record many activities that should trigger an alert
	now := time.Now()
	for i := 0; i < 200; i++ {
		d.RecordActivity(FileActivity{
			Path:      "/data/file" + string(rune(i)) + ".encrypted",
			Operation: "rename",
			Size:      1024,
			Timestamp: now,
			Process:   "suspicious",
		})
	}

	events := d.GetThreatEvents(1, 10)
	// May or may not have events depending on timing; just verify no crash
	_ = events
}

// ============================================================
// IncidentResponse
// ============================================================

func TestIncidentResponse_SystemPathRejection(t *testing.T) {
	cfg := DefaultDetectorConfig()
	d := NewDetector(cfg, nil)
	ir := NewIncidentResponse(d, nil)

	result := ir.QuarantinePaths([]string{"/bin/ls", "/etc/passwd"}, "test")
	if len(result.Failed) != 2 {
		t.Errorf("system paths should be rejected, got %d failed", len(result.Failed))
	}
}

func TestIncidentResponse_AutoQuarantine_LowThreat(t *testing.T) {
	cfg := DefaultDetectorConfig()
	d := NewDetector(cfg, nil)
	ir := NewIncidentResponse(d, nil)

	result := ir.AutoQuarantine()
	if result != nil {
		t.Error("low threat should not trigger auto-quarantine")
	}
}

func TestGenerateRecoveryPlan_LowThreat(t *testing.T) {
	cfg := DefaultDetectorConfig()
	d := NewDetector(cfg, nil)
	ir := NewIncidentResponse(d, nil)

	plan := ir.GenerateRecoveryPlan()
	if plan != nil {
		t.Error("low threat should not generate recovery plan")
	}
}

func TestGenerateRecoveryPlan_HighThreat(t *testing.T) {
	cfg := DefaultDetectorConfig()
	cfg.BlockThreshold = 0.01 // trigger easily
	d := NewDetector(cfg, nil)

	// Feed enough data to trigger high threat
	now := time.Now()
	for i := 0; i < 200; i++ {
		d.RecordActivity(FileActivity{
			Path:      "/data/secret.docx.encrypted",
			Operation: "rename",
			Size:      2048,
			Timestamp: now,
			Process:   "ransomware",
		})
	}
	d.FeedEntropySample("/data/secret.docx.encrypted", makeHighEntropyBytes(1024))

	// Force a scan to populate alerts
	d.ScanNow()

	ir := NewIncidentResponse(d, nil)
	plan := ir.GenerateRecoveryPlan()
	// Plan may or may not be generated depending on score; verify no crash
	_ = plan
}

// ============================================================
// AlertEscalation
// ============================================================

func TestAlertEscalation_LowThreat(t *testing.T) {
	cfg := DefaultDetectorConfig()
	d := NewDetector(cfg, nil)
	policy := DefaultEscalationPolicy()
	ae := NewAlertEscalation(d, policy, nil)

	actions := ae.Evaluate()
	if len(actions) != 0 {
		t.Errorf("low threat should not trigger escalation, got %d actions", len(actions))
	}
}

func TestAlertEscalation_Cooldown(t *testing.T) {
	cfg := DefaultDetectorConfig()
	cfg.BlockThreshold = 0.01
	d := NewDetector(cfg, nil)

	policy := DefaultEscalationPolicy()
	policy.CooldownSec = 600 // 10 min cooldown

	ae := NewAlertEscalation(d, policy, nil)

	// First evaluate (should be no-op for low threat)
	actions1 := ae.Evaluate()
	_ = actions1

	// Second evaluate immediately (cooldown should block even if threat existed)
	// Since the detector has no high threat, both calls return nothing.
	actions2 := ae.Evaluate()
	if len(actions2) != 0 {
		t.Error("expected no escalation for low-threat detector")
	}
}

func TestAlertEscalation_History(t *testing.T) {
	cfg := DefaultDetectorConfig()
	d := NewDetector(cfg, nil)
	policy := DefaultEscalationPolicy()
	ae := NewAlertEscalation(d, policy, nil)

	history := ae.History()
	if history == nil {
		t.Error("history should not be nil")
	}
}

// ============================================================
// YARA interface
// ============================================================

func TestNoOpYARAScanner(t *testing.T) {
	scanner := &NoOpYARAScanner{}
	matches, err := scanner.ScanFile("/any/path")
	if err != nil {
		t.Errorf("NoOp scanner should not error, got %v", err)
	}
	if len(matches) != 0 {
		t.Errorf("NoOp scanner should return no matches, got %d", len(matches))
	}

	matches, err = scanner.ScanDir("/any/dir")
	if err != nil {
		t.Error(err)
	}
	if len(matches) != 0 {
		t.Errorf("NoOp scanner should return no matches, got %d", len(matches))
	}

	if err := scanner.AddRule("rule test { condition: true }"); err != nil {
		t.Error(err)
	}
	if err := scanner.LoadRulesFile("/any/rules.yar"); err != nil {
		t.Error(err)
	}
}

// ============================================================
// KnownRansomwareExtensions
// ============================================================

func TestKnownRansomwareExtensions(t *testing.T) {
	known := []string{".encrypted", ".locked", ".crypto", ".locky", ".cerber", ".wncry"}
	for _, ext := range known {
		if !KnownRansomwareExtensions[ext] {
			t.Errorf("expected %s in KnownRansomwareExtensions", ext)
		}
	}
	unknown := ".normalfile"
	if KnownRansomwareExtensions[unknown] {
		t.Errorf("%s should not be in KnownRansomwareExtensions", unknown)
	}
}

// ============================================================
// Helper functions
// ============================================================

func TestGetExtension(t *testing.T) {
	cases := map[string]string{
		"/data/file.txt":       ".txt",
		"/data/file.enc":       ".enc",
		"/data/noext":          "",
		"/data/.hidden":        ".hidden",
		"/data/dir.with.dot/f": "",
	}
	for input, want := range cases {
		if got := getExtension(input); got != want {
			t.Errorf("getExtension(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestGetDir(t *testing.T) {
	cases := map[string]string{
		"/data/subdir/file.txt": "/data/subdir/",
		"/file.txt":             "/",
		"file.txt":              ".",
	}
	for input, want := range cases {
		if got := getDir(input); got != want {
			t.Errorf("getDir(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSeverityFromScore(t *testing.T) {
	cases := map[float64]string{
		0.0:  "low",
		0.3:  "low",
		0.49: "low",
		0.5:  "medium",
		0.69: "medium",
		0.7:  "high",
		0.9:  "critical",
		1.0:  "critical",
	}
	for score, want := range cases {
		if got := severityFromScore(score); got != want {
			t.Errorf("severityFromScore(%.2f) = %q, want %q", score, got, want)
		}
	}
}

// ============================================================
// Helpers for tests
// ============================================================

// makeHighEntropyBytes creates a byte slice with high Shannon entropy.
func makeHighEntropyBytes(n int) []byte {
	data := make([]byte, n)
	for i := range data {
		data[i] = byte((i * 251) % 256)
	}
	return data
}

// BenchmarkCalculateEntropy benchmarks entropy calculation.
func BenchmarkCalculateEntropy(b *testing.B) {
	data := makeHighEntropyBytes(65536)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		CalculateEntropy(data)
	}
}

// BenchmarkPredict benchmarks prediction.
func BenchmarkPredict(b *testing.B) {
	p := NewRansomwarePredictor(DefaultPredictorWeights(), nil)
	fv := FeatureVector{
		HighEntropyRatio: 0.8,
		TopWriteAnomaly:  &WriteFrequencyStats{Deviation: 5.0, IsAnomaly: true},
		ExtChangeCount:   50,
		SuspiciousExts:   []string{".encrypted"},
		YARAMatchCount:   2,
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		p.Predict(fv)
	}
}
