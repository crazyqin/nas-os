package capacitypredictor

import (
	"testing"
	"time"
)

func TestRecordAndPredict(t *testing.T) {
	p := NewPredictor()

	// 记录历史数据
	baseTime := time.Now().AddDate(0, 0, -30)
	for i := 0; i < 30; i++ {
		p.RecordSnapshot("dataset1", &UsageSnapshot{
			Timestamp:  baseTime.AddDate(0, 0, i),
			TotalBytes: 1024 * 1024 * 1024 * 100, // 100GB
			UsedBytes:  int64(1024*1024*1024) * (50 + int64(i)), // 每天增长1GB
			FreeBytes:  int64(1024*1024*1024) * (50 - int64(i)),
		})
	}

	// 预测
	result, err := p.Predict("dataset1")
	if err != nil {
		t.Fatalf("Predict failed: %v", err)
	}

	if result.CurrentUsage < 50 || result.CurrentUsage > 100 {
		t.Errorf("unexpected current usage: %.1f%%", result.CurrentUsage)
	}

	if result.GrowthRateDaily <= 0 {
		t.Error("expected positive growth rate")
	}

	if result.Confidence < 50 {
		t.Errorf("confidence too low: %.1f%%", result.Confidence)
	}

	if result.Trend == "" {
		t.Error("expected non-empty trend")
	}
}

func TestPredictInsufficientData(t *testing.T) {
	p := NewPredictor()

	// 只有1个样本
	p.RecordSnapshot("small", &UsageSnapshot{
		Timestamp:  time.Now(),
		TotalBytes: 1000,
		UsedBytes:  500,
		FreeBytes:  500,
	})

	_, err := p.Predict("small")
	if err == nil {
		t.Error("expected error for insufficient data")
	}
}

func TestCheckAlerts(t *testing.T) {
	p := NewPredictor()

	p.RecordSnapshot("alert-test", &UsageSnapshot{
		Timestamp:  time.Now(),
		TotalBytes: 1000,
		UsedBytes:  950, // 95%
		FreeBytes:  50,
	})

	alerts := p.CheckAlerts("alert-test")
	if len(alerts) == 0 {
		t.Error("expected alerts for 95% usage")
	}

	foundCritical := false
	for _, a := range alerts {
		if a.Level == AlertCritical {
			foundCritical = true
		}
	}
	if !foundCritical {
		t.Error("expected critical alert")
	}
}

func TestGenerateReport(t *testing.T) {
	p := NewPredictor()

	baseTime := time.Now().AddDate(0, 0, -10)
	for i := 0; i < 10; i++ {
		p.RecordSnapshot("ds1", &UsageSnapshot{
			Timestamp:  baseTime.AddDate(0, 0, i),
			TotalBytes: 1000,
			UsedBytes:  500 + int64(i*10),
			FreeBytes:  500 - int64(i*10),
		})
	}

	report := p.GenerateReport()
	if report == nil {
		t.Fatal("expected non-nil report")
	}

	if len(report.Datasets) != 1 {
		t.Errorf("expected 1 dataset, got %d", len(report.Datasets))
	}
}

func TestEstimateDaysUntilFull(t *testing.T) {
	days := EstimateDaysUntilFull(1024*1024*1024, 1024*1024) // 1GB free, 1MB/day
	if days != 1024 {
		t.Errorf("expected 1024 days, got %d", days)
	}

	// 零增长
	days = EstimateDaysUntilFull(1024, 0)
	if days != 2147483647 { // math.MaxInt32
		t.Errorf("expected MaxInt32 for zero growth, got %d", days)
	}
}

func TestGetDatasets(t *testing.T) {
	p := NewPredictor()

	p.RecordSnapshot("ds1", &UsageSnapshot{TotalBytes: 100, UsedBytes: 50, FreeBytes: 50})
	p.RecordSnapshot("ds2", &UsageSnapshot{TotalBytes: 200, UsedBytes: 100, FreeBytes: 100})

	datasets := p.GetDatasets()
	if len(datasets) != 2 {
		t.Errorf("expected 2 datasets, got %d", len(datasets))
	}
}

func TestCalculateLinearForecast(t *testing.T) {
	p := NewPredictor()

	baseTime := time.Now().AddDate(0, 0, -10)
	for i := 0; i < 10; i++ {
		p.RecordSnapshot("fc-test", &UsageSnapshot{
			Timestamp:  baseTime.AddDate(0, 0, i),
			TotalBytes: 10000,
			UsedBytes:  5000 + int64(i*100),
			FreeBytes:  5000 - int64(i*100),
		})
	}

	usage, err := p.CalculateLinearForecast("fc-test", 30)
	if err != nil {
		t.Fatalf("CalculateLinearForecast failed: %v", err)
	}

	if usage < 50 || usage > 100 {
		t.Errorf("unexpected forecast usage: %.1f%%", usage)
	}
}
