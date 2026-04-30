package ransomware

import (
	"testing"
	"time"
)

func TestNewEntropyDetector(t *testing.T) {
	ed := NewEntropyDetector(DefaultEntropyDetectorConfig())
	if ed == nil {
		t.Fatal("NewEntropyDetector returned nil")
	}
	if ed.config.WindowSize != 100 {
		t.Errorf("WindowSize = %d, want 100", ed.config.WindowSize)
	}
	if ed.config.HighEntropyThreshold != 7.0 {
		t.Errorf("HighEntropyThreshold = %f, want 7.0", ed.config.HighEntropyThreshold)
	}
}

func TestNewEntropyDetector_ZeroConfig(t *testing.T) {
	// 零值配置应使用默认值
	ed := NewEntropyDetector(EntropyDetectorConfig{})
	if ed == nil {
		t.Fatal("NewEntropyDetector returned nil for zero config")
	}
	cfg := ed.config
	if cfg.WindowSize != DefaultEntropyDetectorConfig().WindowSize {
		t.Error("zero WindowSize not replaced with default")
	}
	if cfg.MinSamples != DefaultEntropyDetectorConfig().MinSamples {
		t.Error("zero MinSamples not replaced with default")
	}
}

func TestEntropyDetector_StartStop(t *testing.T) {
	ed := NewEntropyDetector(DefaultEntropyDetectorConfig())
	ed.Start()

	// 等一下确保 goroutine 启动
	time.Sleep(10 * time.Millisecond)

	ed.Stop()

	// 重复Stop不应panic
	ed.Stop()
}

func TestEntropyDetector_CalculateEntropy(t *testing.T) {
	ed := NewEntropyDetector(DefaultEntropyDetectorConfig())

	tests := []struct {
		name     string
		data     []byte
		minValue float64
		maxValue float64
	}{
		{
			name:     "空数据",
			data:     []byte{},
			minValue: 0,
			maxValue: 0,
		},
		{
			name:     "完全相同字节（零熵）",
			data:     []byte{0xAA, 0xAA, 0xAA, 0xAA},
			minValue: 0,
			maxValue: 0,
		},
		{
			name:     "两种字节（1 bit熵）",
			data:     []byte{0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF, 0x00, 0xFF},
			minValue: 0.9,
			maxValue: 1.1,
		},
		{
			name:     "均匀分布（近似8 bit熵）",
			data:     generateUniformBytes(2560),
			minValue: 7.9,
			maxValue: 8.0,
		},
		{
			name:     "正常文本文件（中等熵）",
			data:     []byte("Hello, World! This is a normal text file with some repeated content. Hello again!"),
			minValue: 3.5,
			maxValue: 5.5,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			entropy := ed.calculateEntropy(tt.data)
			if entropy < tt.minValue || entropy > tt.maxValue {
				t.Errorf("calculateEntropy() = %f, want [%f, %f]", entropy, tt.minValue, tt.maxValue)
			}
		})
	}
}

func TestEntropyDetector_OnFileChange_NormalFile(t *testing.T) {
	ed := NewEntropyDetector(EntropyDetectorConfig{
		WindowSize:           20,
		HighEntropyThreshold: 7.0,
		AnomalyStdDev:        2.5,
		SampleInterval:       0, // 测试中不间隔
		MinSamples:           3,
		BatchThreshold:       5,
		BatchWindow:          time.Minute,
		BaselineAlpha:        0.1,
		MaxPaths:             100,
	})

	base := time.Now()

	// 模拟正常文件的低熵写入
	for i := 0; i < 10; i++ {
		event := FileEvent{
			Path:      "/data/test.txt",
			Operation: FileOpWrite,
			Size:      1024,
			Timestamp: base.Add(time.Duration(i) * time.Second),
			Entropies: map[string]float64{"file": 4.0},
		}
		alert := ed.OnFileChange(event)
		if alert != nil {
			t.Errorf("正常文件不应触发告警，但收到: %+v", alert)
		}
	}
}

func TestEntropyDetector_OnFileChange_HighEntropy(t *testing.T) {
	ed := NewEntropyDetector(EntropyDetectorConfig{
		WindowSize:           20,
		HighEntropyThreshold: 7.0,
		AnomalyStdDev:        2.5,
		SampleInterval:       0,
		MinSamples:           3,
		BatchThreshold:       10,
		BatchWindow:          time.Minute,
		BaselineAlpha:        0.1,
		MaxPaths:             100,
	})

	base := time.Now()

	// 先建立低熵基线
	for i := 0; i < 5; i++ {
		ed.OnFileChange(FileEvent{
			Path:      "/data/doc.pdf",
			Operation: FileOpWrite,
			Size:      1024,
			Timestamp: base.Add(time.Duration(i) * time.Second),
			Entropies: map[string]float64{"file": 4.0},
		})
	}

	// 突然写入高熵数据（模拟加密）
	alert := ed.OnFileChange(FileEvent{
		Path:      "/data/doc.pdf",
		Operation: FileOpWrite,
		Size:      1024,
		Timestamp: base.Add(6 * time.Second),
		Entropies: map[string]float64{"file": 7.8},
	})

	if alert == nil {
		t.Fatal("高熵写入应触发告警")
	}
	if alert.AlertType != "high_entropy" && alert.AlertType != "trend_anomaly" {
		t.Errorf("告警类型 = %s, want high_entropy 或 trend_anomaly", alert.AlertType)
	}
	if alert.Severity != ThreatLevelHigh && alert.Severity != ThreatLevelCritical {
		t.Errorf("威胁级别 = %s, want high 或 critical", alert.Severity)
	}
}

func TestEntropyDetector_BatchDetection(t *testing.T) {
	ed := NewEntropyDetector(EntropyDetectorConfig{
		WindowSize:           50,
		HighEntropyThreshold: 7.0,
		AnomalyStdDev:        2.5,
		SampleInterval:       0,
		MinSamples:           2,
		BatchThreshold:       5,
		BatchWindow:          time.Minute,
		BaselineAlpha:        0.1,
		MaxPaths:             100,
	})

	base := time.Now()
	var gotBatch bool

	// 连续高熵写入，模拟勒索软件批量加密
	for i := 0; i < 6; i++ {
		alert := ed.OnFileChange(FileEvent{
			Path:      "/data/file.dat",
			Operation: FileOpWrite,
			Size:      2048,
			Timestamp: base.Add(time.Duration(i) * 500 * time.Millisecond),
			Entropies: map[string]float64{"file": 7.9},
		})
		if alert != nil && alert.AlertType == "batch_detected" {
			gotBatch = true
			if alert.Severity != ThreatLevelCritical {
				t.Errorf("批量检测严重级别 = %s, want critical", alert.Severity)
			}
		}
	}

	if !gotBatch {
		t.Error("未触发批量高熵检测告警")
	}
}

func TestEntropyDetector_IgnoresNonWriteOps(t *testing.T) {
	ed := NewEntropyDetector(DefaultEntropyDetectorConfig())

	ops := []FileOperation{FileOpDelete, FileOpRename, FileOpMove}
	for _, op := range ops {
		alert := ed.OnFileChange(FileEvent{
			Path:      "/data/test.txt",
			Operation: op,
			Entropies: map[string]float64{"file": 7.9},
		})
		if alert != nil {
			t.Errorf("操作 %s 不应触发告警", op)
		}
	}
}

func TestEntropyDetector_AnalyzeTrend(t *testing.T) {
	ed := NewEntropyDetector(EntropyDetectorConfig{
		WindowSize:           20,
		HighEntropyThreshold: 7.0,
		AnomalyStdDev:        5.0, // 提高阈值避免在测试中提前触发
		SampleInterval:       0,
		MinSamples:           3,
		BatchThreshold:       20,
		BatchWindow:          time.Minute,
		BaselineAlpha:        0.1,
		MaxPaths:             100,
	})

	base := time.Now()

	// 写入上升趋势的数据
	// 熵值：3.0, 4.0, 5.0, 6.0, 7.0
	for i := 0; i < 5; i++ {
		ed.OnFileChange(FileEvent{
			Path:      "/data/trend.dat",
			Operation: FileOpWrite,
			Size:      1024,
			Timestamp: base.Add(time.Duration(i) * time.Second),
			Entropies: map[string]float64{"file": 3.0 + float64(i)},
		})
	}

	trend, current, samples := ed.analyzeTrend("/data/trend.dat")
	if len(samples) != 5 {
		t.Errorf("samples count = %d, want 5", len(samples))
	}
	if current != 7.0 {
		t.Errorf("currentEntropy = %f, want 7.0", current)
	}
	// 趋势应为正（上升）
	if trend <= 0 {
		t.Errorf("trend = %f, want > 0 (ascending)", trend)
	}
}

func TestEntropyDetector_AnalyzeTrend_NonExistentPath(t *testing.T) {
	ed := NewEntropyDetector(DefaultEntropyDetectorConfig())
	trend, entropy, samples := ed.analyzeTrend("/nonexistent")
	if trend != 0 || entropy != 0 || samples != nil {
		t.Error("不存在的路径应返回零值")
	}
}

func TestEntropyDetector_AdaptiveBaseline(t *testing.T) {
	ed := NewEntropyDetector(EntropyDetectorConfig{
		WindowSize:           20,
		HighEntropyThreshold: 7.0,
		AnomalyStdDev:        2.5,
		SampleInterval:       0,
		MinSamples:           3,
		BatchThreshold:       20,
		BatchWindow:          time.Minute,
		BaselineAlpha:        0.2, // 较大学习率，快速收敛
		MaxPaths:             100,
	})

	base := time.Now()

	// 如果一个文件本身就是高熵的（如压缩包），基线应该逐渐升高
	// 这样后续的高熵写入不会每次都告警（降低误报）
	for i := 0; i < 20; i++ {
		ed.OnFileChange(FileEvent{
			Path:      "/data/archive.zip",
			Operation: FileOpWrite,
			Size:      4096,
			Timestamp: base.Add(time.Duration(i) * time.Second),
			Entropies: map[string]float64{"file": 7.8},
		})
	}

	baseline, _, _, ok := ed.GetPathState("/data/archive.zip")
	if !ok {
		t.Fatal("路径应被追踪")
	}
	// 基线应已上升到接近7.8
	if baseline < 7.0 {
		t.Errorf("baseline = %f, want >= 7.0 (应自适应升高)", baseline)
	}
}

func TestEntropyDetector_MaxPaths(t *testing.T) {
	maxPaths := 5
	ed := NewEntropyDetector(EntropyDetectorConfig{
		WindowSize:           10,
		HighEntropyThreshold: 7.0,
		AnomalyStdDev:        2.5,
		SampleInterval:       0,
		MinSamples:           1,
		BatchThreshold:       20,
		BatchWindow:          time.Minute,
		BaselineAlpha:        0.1,
		MaxPaths:             maxPaths,
	})

	// 写入超过maxPaths个不同路径
	for i := 0; i < maxPaths+3; i++ {
		event := FileEvent{
			Path:      "/data/file_" + string(rune('a'+i)) + ".txt",
			Operation: FileOpWrite,
			Size:      512,
			Timestamp: time.Now(),
			Entropies: map[string]float64{"file": 4.0},
		}
		ed.OnFileChange(event)
	}

	stats := ed.GetStats()
	if stats.TrackedPaths > maxPaths {
		t.Errorf("TrackedPaths = %d, want <= %d", stats.TrackedPaths, maxPaths)
	}
}

func TestEntropyDetector_AlertsChannel(t *testing.T) {
	ed := NewEntropyDetector(EntropyDetectorConfig{
		WindowSize:           10,
		HighEntropyThreshold: 7.0,
		AnomalyStdDev:        2.5,
		SampleInterval:       0,
		MinSamples:           2,
		BatchThreshold:       20,
		BatchWindow:          time.Minute,
		BaselineAlpha:        0.1,
		MaxPaths:             100,
	})
	ed.Start()
	defer ed.Stop()

	base := time.Now()

	// 建立基线
	for i := 0; i < 3; i++ {
		ed.OnFileChange(FileEvent{
			Path:      "/data/alert_test.dat",
			Operation: FileOpWrite,
			Size:      1024,
			Timestamp: base.Add(time.Duration(i) * time.Second),
			Entropies: map[string]float64{"file": 3.0},
		})
	}

	// 触发告警
	ed.OnFileChange(FileEvent{
		Path:      "/data/alert_test.dat",
		Operation: FileOpWrite,
		Size:      1024,
		Timestamp: base.Add(4 * time.Second),
		Entropies: map[string]float64{"file": 7.9},
	})

	select {
	case alert := <-ed.Alerts():
		if alert.Entropy < 7.0 {
			t.Errorf("告警熵值 = %f, want >= 7.0", alert.Entropy)
		}
	case <-time.After(time.Second):
		t.Error("未收到告警")
	}
}

func TestEntropyDetector_Stats(t *testing.T) {
	ed := NewEntropyDetector(EntropyDetectorConfig{
		WindowSize:           10,
		HighEntropyThreshold: 7.0,
		AnomalyStdDev:        2.5,
		SampleInterval:       0,
		MinSamples:           1,
		BatchThreshold:       20,
		BatchWindow:          time.Minute,
		BaselineAlpha:        0.1,
		MaxPaths:             100,
	})

	base := time.Now()
	for i := 0; i < 5; i++ {
		ed.OnFileChange(FileEvent{
			Path:      "/data/stats.dat",
			Operation: FileOpWrite,
			Size:      1024,
			Timestamp: base.Add(time.Duration(i) * time.Second),
			Entropies: map[string]float64{"file": 5.0},
		})
	}

	stats := ed.GetStats()
	if stats.TotalSamples != 5 {
		t.Errorf("TotalSamples = %d, want 5", stats.TotalSamples)
	}
	if stats.TrackedPaths != 1 {
		t.Errorf("TrackedPaths = %d, want 1", stats.TrackedPaths)
	}
}

// 辅助函数：生成均匀分布的字节数据.
func generateUniformBytes(n int) []byte {
	data := make([]byte, n)
	for i := 0; i < n; i++ {
		data[i] = byte(i % 256)
	}
	return data
}
