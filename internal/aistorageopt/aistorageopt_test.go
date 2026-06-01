package aistorageopt

import (
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	cfg := Config{
		Level:        LevelBalanced,
		MaxMetrics:   100,
		LearningRate: 0.01,
		Enabled:      true,
	}
	opt := New(cfg)
	if opt == nil {
		t.Fatal("New 返回 nil")
	}
	if opt.level != LevelBalanced {
		t.Fatalf("期望 level=%s, got %s", LevelBalanced, opt.level)
	}
}

func TestRecordAccess(t *testing.T) {
	opt := New(Config{Enabled: true})
	opt.RecordAccess("file1", 1024, 512)

	opt.mu.RLock()
	pattern, exists := opt.accessPatterns["file1"]
	opt.mu.RUnlock()

	if !exists {
		t.Fatal("访问模式未记录")
	}
	if pattern.AccessCount != 1 {
		t.Fatalf("期望 AccessCount=1, got %d", pattern.AccessCount)
	}
	if pattern.ReadRatio != 1024.0/1536.0 {
		t.Fatalf("ReadRatio 不正确: %f", pattern.ReadRatio)
	}
}

func TestRecordMetrics(t *testing.T) {
	opt := New(Config{MaxMetrics: 10, Enabled: true})
	metrics := &StorageMetrics{
		TotalSpace: 1024 * 1024 * 1024,
		UsedSpace:  512 * 1024 * 1024,
		FreeSpace:  512 * 1024 * 1024,
	}
	opt.RecordMetrics(metrics)

	if len(opt.metrics) != 1 {
		t.Fatalf("期望 1 条指标, got %d", len(opt.metrics))
	}
}

func TestPredictStorage(t *testing.T) {
	opt := New(Config{Enabled: true})

	// 添加测试数据
	for i := 0; i < 10; i++ {
		opt.metrics = append(opt.metrics, &StorageMetrics{
			Timestamp:  time.Now().Add(time.Duration(i) * time.Hour),
			UsedSpace:  int64(100+i*10) * 1024 * 1024,
			FreeSpace:  int64(900-i*10) * 1024 * 1024,
			TotalSpace: 1024 * 1024 * 1024,
		})
	}

	prediction := opt.PredictStorage(7)
	if prediction == nil {
		t.Fatal("预测结果为 nil")
	}
	if prediction.Confidence < 0 || prediction.Confidence > 1 {
		t.Fatalf("置信度超出范围: %f", prediction.Confidence)
	}
}

func TestAddTierPolicy(t *testing.T) {
	opt := New(Config{Enabled: true})
	policy := &TierPolicy{
		ID:      "policy1",
		Name:    "冷数据迁移",
		Enabled: true,
		Tier:    TierCold,
	}

	err := opt.AddTierPolicy(policy)
	if err != nil {
		t.Fatalf("添加策略失败: %v", err)
	}

	opt.mu.RLock()
	_, exists := opt.policies["policy1"]
	opt.mu.RUnlock()

	if !exists {
		t.Fatal("策略未添加")
	}
}

func TestAddTierPolicyEmptyID(t *testing.T) {
	opt := New(Config{Enabled: true})
	policy := &TierPolicy{
		Name: "test",
	}

	err := opt.AddTierPolicy(policy)
	if err == nil {
		t.Fatal("期望返回错误")
	}
}

func TestDeduplicateFiles(t *testing.T) {
	opt := New(Config{Enabled: true})
	files := map[string]string{
		"file1": "hash1",
		"file2": "hash1",
		"file3": "hash2",
		"file4": "hash2",
		"file5": "hash2",
	}

	results := opt.DeduplicateFiles(files)
	if len(results) != 2 {
		t.Fatalf("期望 2 组重复, got %d", len(results))
	}

	hash1Info, exists := results["hash1"]
	if !exists {
		t.Fatal("hash1 未找到")
	}
	if hash1Info.DuplicateCount != 2 {
		t.Fatalf("期望 hash1 重复数=2, got %d", hash1Info.DuplicateCount)
	}
}

func TestGetOptimizationReport(t *testing.T) {
	opt := New(Config{
		Level:   LevelAggressive,
		Enabled: true,
	})

	report := opt.GetOptimizationReport()
	if report["level"] != LevelAggressive {
		t.Fatalf("期望 level=%s, got %v", LevelAggressive, report["level"])
	}
	if report["enabled"] != true {
		t.Fatal("期望 enabled=true")
	}
}

func TestLinearRegression(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5}
	slope, intercept := linearRegression(data)

	// x=[0,1,2,3,4], y=[1,2,3,4,5] → y = 1*x + 1
	if slope < 0.9 || slope > 1.1 {
		t.Fatalf("斜率异常: %f", slope)
	}
	if intercept < 0.9 || intercept > 1.1 {
		t.Fatalf("截距异常: %f", intercept)
	}
}

func TestCalculateConfidence(t *testing.T) {
	// 完美线性数据
	data := []float64{1, 2, 3, 4, 5}
	slope, intercept := linearRegression(data)
	confidence := calculateConfidence(data, slope, intercept)

	if confidence < 0.99 {
		t.Fatalf("线性数据置信度应接近1, got %f", confidence)
	}
}

func TestEvaluateTierPolicies(t *testing.T) {
	opt := New(Config{Enabled: true})
	threshold := 0.5
	policy := &TierPolicy{
		ID:      "test",
		Enabled: true,
		Tier:    TierCold,
		Condition: TierCondition{
			AccessThreshold: &threshold,
		},
	}
	opt.AddTierPolicy(policy)

	opt.accessPatterns["file1"] = &AccessPattern{
		FileID:     "file1",
		AccessFreq: 0.1,
	}

	moved := opt.EvaluateTierPolicies()
	if len(moved) != 1 {
		t.Fatalf("期望移动 1 个文件, got %d", len(moved))
	}
}
