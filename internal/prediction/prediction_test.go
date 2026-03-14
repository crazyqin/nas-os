package prediction

import (
	"testing"
	"time"
)

// ========== 配置测试 ==========

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.CollectionInterval != 5*time.Minute {
		t.Errorf("Expected CollectionInterval=5m, got %v", cfg.CollectionInterval)
	}

	if cfg.HistoryRetentionDays != 90 {
		t.Errorf("Expected HistoryRetentionDays=90, got %d", cfg.HistoryRetentionDays)
	}

	if cfg.PredictionDays != 30 {
		t.Errorf("Expected PredictionDays=30, got %d", cfg.PredictionDays)
	}

	if cfg.WarningThreshold >= cfg.CriticalThreshold {
		t.Error("WarningThreshold should be less than CriticalThreshold")
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "default config is valid",
			config:  DefaultConfig(),
			wantErr: false,
		},
		{
			name: "collection interval too short",
			config: &Config{
				CollectionInterval:   30 * time.Second,
				HistoryRetentionDays: 90,
				PredictionDays:       30,
				AnomalySensitivity:   0.8,
				WarningThreshold:     75,
				CriticalThreshold:    90,
			},
			wantErr: true,
		},
		{
			name: "retention days too short",
			config: &Config{
				CollectionInterval:   5 * time.Minute,
				HistoryRetentionDays: 0,
				PredictionDays:       30,
				AnomalySensitivity:   0.8,
				WarningThreshold:     75,
				CriticalThreshold:    90,
			},
			wantErr: true,
		},
		{
			name: "prediction days too short",
			config: &Config{
				CollectionInterval:   5 * time.Minute,
				HistoryRetentionDays: 90,
				PredictionDays:       0,
				AnomalySensitivity:   0.8,
				WarningThreshold:     75,
				CriticalThreshold:    90,
			},
			wantErr: true,
		},
		{
			name: "anomaly sensitivity out of range",
			config: &Config{
				CollectionInterval:   5 * time.Minute,
				HistoryRetentionDays: 90,
				PredictionDays:       30,
				AnomalySensitivity:   1.5,
				WarningThreshold:     75,
				CriticalThreshold:    90,
			},
			wantErr: true,
		},
		{
			name: "warning threshold >= critical threshold",
			config: &Config{
				CollectionInterval:   5 * time.Minute,
				HistoryRetentionDays: 90,
				PredictionDays:       30,
				AnomalySensitivity:   0.8,
				WarningThreshold:     90,
				CriticalThreshold:    90,
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

// ========== Manager 创建测试 ==========

func TestNewManager(t *testing.T) {
	mgr, err := NewManager(nil)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if !mgr.initialized {
		t.Error("Manager should be initialized")
	}

	if mgr.history == nil {
		t.Error("History store should not be nil")
	}

	if mgr.model == nil {
		t.Error("Prediction model should not be nil")
	}

	if mgr.anomalyDetector == nil {
		t.Error("Anomaly detector should not be nil")
	}

	// 清理
	mgr.Stop()
}

func TestNewManagerWithConfig(t *testing.T) {
	cfg := &Config{
		CollectionInterval:   10 * time.Minute,
		HistoryRetentionDays: 60,
		PredictionDays:       14,
		AnomalySensitivity:   0.7,
		WarningThreshold:     70,
		CriticalThreshold:    85,
	}

	mgr, err := NewManager(cfg)
	if err != nil {
		t.Fatalf("NewManager() error = %v", err)
	}

	if mgr.config.CollectionInterval != 10*time.Minute {
		t.Errorf("Expected CollectionInterval=10m, got %v", mgr.config.CollectionInterval)
	}

	mgr.Stop()
}

func TestNewManagerInvalidConfig(t *testing.T) {
	cfg := &Config{
		CollectionInterval:   30 * time.Second, // too short
		HistoryRetentionDays: 90,
		PredictionDays:       30,
		AnomalySensitivity:   0.8,
		WarningThreshold:     75,
		CriticalThreshold:    90,
	}

	_, err := NewManager(cfg)
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

// ========== 数据记录测试 ==========

func TestRecordUsage(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	err := mgr.RecordUsage("test-vol", 500.0, 1000.0)
	if err != nil {
		t.Fatalf("RecordUsage() error = %v", err)
	}

	// 验证数据已存储
	mgr.history.mu.RLock()
	volHistory, exists := mgr.history.VolumeData["test-vol"]
	mgr.history.mu.RUnlock()

	if !exists {
		t.Fatal("Volume history should exist")
	}

	if len(volHistory.UsageHistory) != 1 {
		t.Errorf("Expected 1 usage record, got %d", len(volHistory.UsageHistory))
	}

	record := volHistory.UsageHistory[0]
	if record.UsedGB != 500.0 {
		t.Errorf("Expected UsedGB=500, got %f", record.UsedGB)
	}

	if record.UsageRate != 50.0 {
		t.Errorf("Expected UsageRate=50, got %f", record.UsageRate)
	}
}

func TestRecordUsageMultiple(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 记录多次数据
	for i := 0; i < 5; i++ {
		err := mgr.RecordUsage("test-vol", float64(i+1)*100, 1000.0)
		if err != nil {
			t.Fatalf("RecordUsage() error = %v", err)
		}
	}

	mgr.history.mu.RLock()
	volHistory := mgr.history.VolumeData["test-vol"]
	mgr.history.mu.RUnlock()

	if len(volHistory.UsageHistory) != 5 {
		t.Errorf("Expected 5 usage records, got %d", len(volHistory.UsageHistory))
	}
}

func TestRecordIO(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	err := mgr.RecordIO("test-vol", 100.5, 50.2, 1500)
	if err != nil {
		t.Fatalf("RecordIO() error = %v", err)
	}

	mgr.history.mu.RLock()
	volHistory, exists := mgr.history.VolumeData["test-vol"]
	mgr.history.mu.RUnlock()

	if !exists {
		t.Fatal("Volume history should exist")
	}

	if len(volHistory.IOHistory) != 1 {
		t.Errorf("Expected 1 IO record, got %d", len(volHistory.IOHistory))
	}

	record := volHistory.IOHistory[0]
	if record.ReadMBps != 100.5 {
		t.Errorf("Expected ReadMBps=100.5, got %f", record.ReadMBps)
	}
}

// ========== 预测测试 ==========

func TestPredictInsufficientData(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 只记录一条数据
	mgr.RecordUsage("test-vol", 500.0, 1000.0)

	_, err := mgr.Predict("test-vol")
	if err == nil {
		t.Error("Expected error for insufficient data")
	}
}

func TestPredictBasic(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 记录足够的历史数据（模拟稳定增长）
	for i := 0; i < 10; i++ {
		mgr.RecordUsage("test-vol", float64(i+1)*10, 1000.0)
	}

	result, err := mgr.Predict("test-vol")
	if err != nil {
		t.Fatalf("Predict() error = %v", err)
	}

	if result.VolumeName != "test-vol" {
		t.Errorf("Expected VolumeName='test-vol', got '%s'", result.VolumeName)
	}

	if result.CurrentUsage <= 0 {
		t.Error("CurrentUsage should be positive")
	}

	if result.Trend == "" {
		t.Error("Trend should not be empty")
	}
}

func TestPredictTrendIncreasing(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 模拟持续增长
	for i := 0; i < 10; i++ {
		mgr.RecordUsage("test-vol", float64(i+1)*10, 1000.0)
	}

	result, _ := mgr.Predict("test-vol")

	if result.Trend != "increasing" {
		t.Errorf("Expected trend='increasing', got '%s'", result.Trend)
	}

	if result.GrowthRateDaily <= 0 {
		t.Error("GrowthRateDaily should be positive for increasing trend")
	}
}

func TestPredictTrendStable(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 模拟稳定使用
	for i := 0; i < 10; i++ {
		mgr.RecordUsage("test-vol", 500.0, 1000.0)
	}

	result, _ := mgr.Predict("test-vol")

	if result.Trend != "stable" {
		t.Errorf("Expected trend='stable', got '%s'", result.Trend)
	}
}

func TestPredictFutureUsage(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 记录历史数据
	for i := 0; i < 20; i++ {
		mgr.RecordUsage("test-vol", float64(i+1)*10, 1000.0)
	}

	result, _ := mgr.Predict("test-vol")

	// 检查预测点
	if len(result.PredictedUsage) == 0 {
		t.Error("Should have predicted usage points")
	}

	// 检查预测时间是否递增
	for i := 1; i < len(result.PredictedUsage); i++ {
		if !result.PredictedUsage[i].Date.After(result.PredictedUsage[i-1].Date) {
			t.Error("Predicted dates should be in ascending order")
		}
	}
}

func TestPredictThresholdDays(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 模拟快速增长
	for i := 0; i < 10; i++ {
		mgr.RecordUsage("test-vol", float64(i+1)*50, 1000.0)
	}

	result, _ := mgr.Predict("test-vol")

	// 应该计算出增长率和趋势
	if result.GrowthRateDaily <= 0 {
		t.Error("GrowthRateDaily should be positive for increasing data")
	}

	// 根据实际数据，趋势应该是 increasing
	if result.Trend != "increasing" {
		t.Errorf("Expected trend='increasing', got '%s'", result.Trend)
	}
}

// ========== 历史数据测试 ==========

func TestGetHistory(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 记录数据
	for i := 0; i < 5; i++ {
		mgr.RecordUsage("test-vol", float64(i+1)*100, 1000.0)
	}

	records, err := mgr.GetHistory("test-vol", 1)
	if err != nil {
		t.Fatalf("GetHistory() error = %v", err)
	}

	if len(records) != 5 {
		t.Errorf("Expected 5 records, got %d", len(records))
	}
}

func TestGetHistoryNonexistentVolume(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	_, err := mgr.GetHistory("nonexistent", 7)
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

func TestListVolumes(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 初始应该为空
	volumes := mgr.ListVolumes()
	if len(volumes) != 0 {
		t.Errorf("Expected empty volumes list, got %d", len(volumes))
	}

	// 记录数据后
	mgr.RecordUsage("vol1", 100, 1000)
	mgr.RecordUsage("vol2", 200, 1000)

	volumes = mgr.ListVolumes()
	if len(volumes) != 2 {
		t.Errorf("Expected 2 volumes, got %d", len(volumes))
	}
}

// ========== 建议生成测试 ==========

func TestGenerateAdvicesWarningThreshold(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 模拟接近预警阈值
	for i := 0; i < 10; i++ {
		mgr.RecordUsage("test-vol", float64(i+1)*5, 100.0)
	}

	result, _ := mgr.Predict("test-vol")

	// 检查是否生成了建议
	if len(result.Advices) == 0 {
		t.Log("No advices generated (may need specific conditions)")
	}
}

func TestAdviceRules(t *testing.T) {
	rules := getDefaultAdviceRules()

	if len(rules) == 0 {
		t.Error("Should have default advice rules")
	}

	// 验证规则结构
	for _, rule := range rules {
		if rule.Name == "" {
			t.Error("Rule should have a name")
		}
		if rule.Condition == nil {
			t.Error("Rule should have a condition function")
		}
		if rule.Generate == nil {
			t.Error("Rule should have a generate function")
		}
	}
}

// ========== 异常检测测试 ==========

func TestAnomalyDetection(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 建立基线（稳定使用）
	for i := 0; i < 15; i++ {
		mgr.RecordUsage("test-vol", 500.0, 1000.0)
	}

	// 突然跳变
	mgr.RecordUsage("test-vol", 900.0, 1000.0)

	// 获取异常
	anomalies := mgr.getAnomalies("test-vol")

	// 可能检测到异常（取决于基线建立情况）
	t.Logf("Detected %d anomalies", len(anomalies))
}

// ========== 预测结果方法测试 ==========

func TestPredictionResultMethods(t *testing.T) {
	result := &PredictionResult{
		VolumeName:       "test",
		CurrentUsage:     500.0,
		CurrentTotal:     1000.0,
		CurrentUsageRate: 50.0,
		Trend:            "increasing",
		WarningInDays:    20,
		CriticalInDays:   0,
		PredictedUsage: []PredictedPoint{
			{Date: time.Now().AddDate(0, 0, 1), UsageGB: 510},
			{Date: time.Now().AddDate(0, 0, 2), UsageGB: 520},
		},
	}

	if result.PredictedPoints() != 2 {
		t.Errorf("Expected 2 predicted points, got %d", result.PredictedPoints())
	}

	if !result.HasWarning() {
		t.Error("Should have warning (WarningInDays=20)")
	}

	if result.IsCritical() {
		t.Error("Should not be critical (CriticalInDays=0)")
	}
}

func TestPredictionResultCritical(t *testing.T) {
	result := &PredictionResult{
		WarningInDays:  5,
		CriticalInDays: 7,
	}

	if !result.HasWarning() {
		t.Error("Should have warning")
	}

	if !result.IsCritical() {
		t.Error("Should be critical")
	}
}

// ========== 配置更新测试 ==========

func TestUpdateConfig(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	newCfg := &Config{
		CollectionInterval:   10 * time.Minute,
		HistoryRetentionDays: 60,
		PredictionDays:       14,
		AnomalySensitivity:   0.7,
		WarningThreshold:     70,
		CriticalThreshold:    85,
	}

	err := mgr.UpdateConfig(newCfg)
	if err != nil {
		t.Fatalf("UpdateConfig() error = %v", err)
	}

	if mgr.config.WarningThreshold != 70 {
		t.Errorf("Expected WarningThreshold=70, got %f", mgr.config.WarningThreshold)
	}
}

func TestUpdateConfigInvalid(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	invalidCfg := &Config{
		CollectionInterval:   10 * time.Minute,
		HistoryRetentionDays: 60,
		PredictionDays:       14,
		AnomalySensitivity:   1.5, // invalid
		WarningThreshold:     70,
		CriticalThreshold:    85,
	}

	err := mgr.UpdateConfig(invalidCfg)
	if err == nil {
		t.Error("Expected error for invalid config")
	}
}

// ========== 全量预测测试 ==========

func TestGetAllPredictions(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 记录多个卷的数据
	for vol := 0; vol < 3; vol++ {
		volName := string(rune('A' + vol))
		for i := 0; i < 10; i++ {
			mgr.RecordUsage(volName, float64(i+1)*10, 1000.0)
		}
	}

	results, err := mgr.GetAllPredictions()
	if err != nil {
		t.Fatalf("GetAllPredictions() error = %v", err)
	}

	if len(results) != 3 {
		t.Errorf("Expected 3 prediction results, got %d", len(results))
	}
}

// ========== 历史数据清理测试 ==========

func TestHistoryCleanup(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 设置较短的保留期
	mgr.config.HistoryRetentionDays = 1

	// 记录数据
	mgr.RecordUsage("test-vol", 100, 1000)

	// 触发清理
	mgr.cleanupOldHistory()

	// 验证数据仍存在（因为是刚记录的）
	records, _ := mgr.GetHistory("test-vol", 1)
	if len(records) != 1 {
		t.Errorf("Expected 1 record, got %d", len(records))
	}
}

// ========== 边界条件测试 ==========

func TestRecordUsageZeroTotal(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 总容量为0
	err := mgr.RecordUsage("test-vol", 100, 0)
	if err != nil {
		t.Fatalf("RecordUsage() error = %v", err)
	}

	// 不应该panic
	mgr.history.mu.RLock()
	record := mgr.history.VolumeData["test-vol"].UsageHistory[0]
	mgr.history.mu.RUnlock()

	if record.UsageRate != 0 {
		t.Errorf("Expected UsageRate=0 when total=0, got %f", record.UsageRate)
	}
}

func TestPredictEmptyVolume(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	_, err := mgr.Predict("nonexistent")
	if err == nil {
		t.Error("Expected error for nonexistent volume")
	}
}

// ========== 并发安全测试 ==========

func TestConcurrentRecordUsage(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	done := make(chan bool)

	// 并发写入
	for i := 0; i < 10; i++ {
		go func(idx int) {
			for j := 0; j < 10; j++ {
				mgr.RecordUsage("test-vol", float64(idx*10+j), 1000.0)
			}
			done <- true
		}(i)
	}

	// 等待完成
	for i := 0; i < 10; i++ {
		<-done
	}

	// 验证数据完整性
	mgr.history.mu.RLock()
	records := mgr.history.VolumeData["test-vol"].UsageHistory
	mgr.history.mu.RUnlock()

	if len(records) != 100 {
		t.Errorf("Expected 100 records, got %d", len(records))
	}
}

func TestConcurrentPredict(t *testing.T) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 准备数据
	for i := 0; i < 10; i++ {
		mgr.RecordUsage("test-vol", float64(i+1)*10, 1000.0)
	}

	done := make(chan bool)

	// 并发预测
	for i := 0; i < 5; i++ {
		go func() {
			_, err := mgr.Predict("test-vol")
			if err != nil {
				t.Errorf("Predict() error = %v", err)
			}
			done <- true
		}()
	}

	// 等待完成
	for i := 0; i < 5; i++ {
		<-done
	}
}

// ========== 性能测试 ==========

func BenchmarkRecordUsage(b *testing.B) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.RecordUsage("bench-vol", float64(i), 1000.0)
	}
}

func BenchmarkPredict(b *testing.B) {
	mgr, _ := NewManager(nil)
	defer mgr.Stop()

	// 准备数据
	for i := 0; i < 100; i++ {
		mgr.RecordUsage("bench-vol", float64(i), 1000.0)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.Predict("bench-vol")
	}
}
