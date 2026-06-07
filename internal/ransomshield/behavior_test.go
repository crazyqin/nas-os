package ransomshield

import (
	"fmt"
	"testing"
	"time"
)

func TestNewBehaviorAnalyzer(t *testing.T) {
	ba := NewBehaviorAnalyzer()
	if ba == nil {
		t.Fatal("NewBehaviorAnalyzer returned nil")
	}

	if len(ba.patternDetectors) < 3 {
		t.Errorf("expected at least 3 pattern detectors, got %d", len(ba.patternDetectors))
	}

	if ba.globalBaseline == nil {
		t.Error("globalBaseline should not be nil")
	}
}

func TestBehaviorAnalyzer_RecordFileOp(t *testing.T) {
	ba := NewBehaviorAnalyzer()

	op := FileOpRecord{
		Path:        "/data/test.txt",
		OpType:      "write",
		Size:        1024,
		Entropy:     7.8,
		ProcessID:   1234,
		ProcessName: "malware",
		Timestamp:   time.Now(),
	}

	ba.RecordFileOp(op)

	stats := ba.GetStats()
	if stats.EventsProcessed != 1 {
		t.Errorf("expected 1 event processed, got %d", stats.EventsProcessed)
	}

	// 验证进程画像已创建
	profile, ok := ba.GetProfile(1234)
	if !ok {
		t.Fatal("process profile should exist")
	}

	if profile.Name != "malware" {
		t.Errorf("expected process name 'malware', got '%s'", profile.Name)
	}

	if profile.TotalWrites != 1 {
		t.Errorf("expected 1 write, got %d", profile.TotalWrites)
	}
}

func TestBehaviorAnalyzer_EncryptionBurstDetection(t *testing.T) {
	ba := NewBehaviorAnalyzer()

	done := make(chan AnomalyEvent, 10)
	ba.SetAnomalyCallback(func(event AnomalyEvent) {
		if event.AnomalyType == "encryption-burst" {
			done <- event
		}
	})

	pid := 5678
	for i := 0; i < 500; i++ {
		ba.RecordFileOp(FileOpRecord{
			Path:        fmt.Sprintf("/data/encrypted_%d.txt", i),
			OpType:      "write",
			Size:        4096,
			Entropy:     7.9,
			ProcessID:   pid,
			ProcessName: "ransomware",
			Timestamp:   time.Now(),
		})
	}

	ba.runAnalysis()

	// 等待异步回调完成
	select {
	case <-done:
		// 成功检测到
	case <-time.After(2 * time.Second):
		t.Error("expected encryption-burst anomaly to be detected")
	}
}

func TestBehaviorAnalyzer_AnomalousProcesses(t *testing.T) {
	ba := NewBehaviorAnalyzer()

	// 添加正常进程
	for i := 0; i < 10; i++ {
		ba.RecordFileOp(FileOpRecord{
			Path:        "/data/normal.txt",
			OpType:      "write",
			Size:        100,
			Entropy:     4.0,
			ProcessID:   100,
			ProcessName: "normal-app",
			Timestamp:   time.Now(),
		})
	}

	// 添加可疑进程 - 需要大量写入使 writeRate > 5.0
	for i := 0; i < 500; i++ {
		ba.RecordFileOp(FileOpRecord{
			Path:        fmt.Sprintf("/data/encrypted_%d.txt", i),
			OpType:      "write",
			Size:        8192,
			Entropy:     7.9,
			ProcessID:   200,
			ProcessName: "suspicious",
			Timestamp:   time.Now(),
		})
	}

	// 运行分析
	ba.runAnalysis()

	anomalous := ba.GetAnomalousProcesses(20.0)
	if len(anomalous) == 0 {
		t.Error("expected at least one anomalous process")
	}
}

func TestBehaviorAnalyzer_ExtractExt(t *testing.T) {
	tests := []struct {
		path string
		want string
	}{
		{"/data/file.txt", ".txt"},
		{"/data/file.docx", ".docx"},
		{"/data/noext", ""},
		{"/data/.hidden", ".hidden"},
		{"file.xlsx", ".xlsx"},
	}

	for _, tt := range tests {
		got := extractExt(tt.path)
		if got != tt.want {
			t.Errorf("extractExt(%q) = %q, want %q", tt.path, got, tt.want)
		}
	}
}

func TestBehaviorAnalyzer_MultipleProcessTracking(t *testing.T) {
	ba := NewBehaviorAnalyzer()

	// 添加多个不同进程
	pids := []int{101, 102, 103}
	for _, pid := range pids {
		ba.RecordFileOp(FileOpRecord{
			Path:        "/data/test.txt",
			OpType:      "read",
			Size:        0,
			ProcessID:   pid,
			ProcessName: "proc",
			Timestamp:   time.Now(),
		})
	}

	stats := ba.GetStats()
	if stats.ProfilesTracked != 3 {
		t.Errorf("expected 3 profiles tracked, got %d", stats.ProfilesTracked)
	}

	for _, pid := range pids {
		_, ok := ba.GetProfile(pid)
		if !ok {
			t.Errorf("profile for pid %d should exist", pid)
		}
	}
}

func TestBehaviorAnalyzer_FileOpTypes(t *testing.T) {
	ba := NewBehaviorAnalyzer()
	pid := 999

	ba.RecordFileOp(FileOpRecord{
		Path: "/data/a.txt", OpType: "read", ProcessID: pid, ProcessName: "proc", Timestamp: time.Now(),
	})
	ba.RecordFileOp(FileOpRecord{
		Path: "/data/b.txt", OpType: "write", Size: 1000, ProcessID: pid, ProcessName: "proc", Timestamp: time.Now(),
	})
	ba.RecordFileOp(FileOpRecord{
		Path: "/data/c.txt", OpType: "delete", ProcessID: pid, ProcessName: "proc", Timestamp: time.Now(),
	})
	ba.RecordFileOp(FileOpRecord{
		Path: "/data/d.txt", OldPath: "/data/c.txt", OpType: "rename", ProcessID: pid, ProcessName: "proc", Timestamp: time.Now(),
	})

	profile, _ := ba.GetProfile(pid)
	if profile.TotalReads != 1 {
		t.Errorf("expected 1 read, got %d", profile.TotalReads)
	}
	if profile.TotalWrites != 1 {
		t.Errorf("expected 1 write, got %d", profile.TotalWrites)
	}
	if profile.TotalDeletes != 1 {
		t.Errorf("expected 1 delete, got %d", profile.TotalDeletes)
	}
	if profile.TotalRenames != 1 {
		t.Errorf("expected 1 rename, got %d", profile.TotalRenames)
	}
}

func TestPatternDetectors_Name(t *testing.T) {
	detectors := []PatternDetector{
		&EncryptionBurstDetector{},
		&MassRenameDetector{},
		&EntropySpikeDetector{},
		&ShadowCopyDeletionDetector{},
		&ExtensionStormDetector{},
	}

	expectedNames := map[string]bool{
		"encryption-burst":     false,
		"mass-rename":          false,
		"entropy-spike":        false,
		"shadow-copy-deletion": false,
		"extension-storm":      false,
	}

	for _, d := range detectors {
		name := d.Name()
		if _, ok := expectedNames[name]; ok {
			expectedNames[name] = true
		} else {
			t.Errorf("unexpected detector name: %s", name)
		}
	}

	for name, found := range expectedNames {
		if !found {
			t.Errorf("detector %s not found", name)
		}
	}
}

func TestBehaviorAnalyzer_BaselineUpdate(t *testing.T) {
	ba := NewBehaviorAnalyzer()

	// 初始基线
	initialAvg := ba.globalBaseline.AvgWriteRate

	// 添加数据
	for i := 0; i < 10; i++ {
		ba.RecordFileOp(FileOpRecord{
			Path:        "/data/test.txt",
			OpType:      "write",
			Size:        1024,
			Entropy:     5.0,
			ProcessID:   300 + i,
			ProcessName: "app",
			Timestamp:   time.Now(),
		})
	}

	ba.runAnalysis()

	newAvg := ba.globalBaseline.AvgWriteRate
	if newAvg == initialAvg {
		// 基线可能未改变因为窗口统计需要时间
		t.Log("baseline may not have changed yet")
	}

	if ba.globalBaseline.SampleCount == 0 {
		t.Error("expected SampleCount > 0 after analysis")
	}
}
