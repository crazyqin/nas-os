package smartstoragepredict

import (
	"testing"
	"time"
)

func TestNewStoragePredictor(t *testing.T) {
	config := DefaultPredictionConfig()
	predictor := NewStoragePredictor(config)

	if predictor == nil {
		t.Fatal("expected predictor to be created")
	}

	if predictor.config.DefaultModel != ModelEnsemble {
		t.Errorf("expected ensemble model, got %s", predictor.config.DefaultModel)
	}
}

func TestRegisterPool(t *testing.T) {
	predictor := NewStoragePredictor(nil)

	pool := &StoragePool{
		ID:         "pool-1",
		Name:       "Test Pool",
		TotalBytes: 1024 * 1024 * 1024 * 100, // 100GB
	}

	err := predictor.RegisterPool(pool)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 重复注册应成功（更新）
	err = predictor.RegisterPool(pool)
	if err != nil {
		t.Fatalf("unexpected error on re-register: %v", err)
	}
}

func TestRecordUsage(t *testing.T) {
	predictor := NewStoragePredictor(nil)

	pool := &StoragePool{
		ID:         "pool-1",
		Name:       "Test Pool",
		TotalBytes: 1024 * 1024 * 1024 * 100, // 100GB
	}
	predictor.RegisterPool(pool)

	// 记录使用量
	err := predictor.RecordUsage("pool-1", 1024*1024*1024*50) // 50GB
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// 验证更新
	updatedPool, _ := predictor.GetPool("pool-1")
	if updatedPool.UsedBytes != 1024*1024*1024*50 {
		t.Errorf("expected 50GB used, got %d", updatedPool.UsedBytes)
	}

	if updatedPool.FreeBytes != 1024*1024*1024*50 {
		t.Errorf("expected 50GB free, got %d", updatedPool.FreeBytes)
	}
}

func TestPredictInsufficientData(t *testing.T) {
	predictor := NewStoragePredictor(nil)

	pool := &StoragePool{
		ID:         "pool-1",
		Name:       "Test Pool",
		TotalBytes: 1024 * 1024 * 1024 * 100,
	}
	predictor.RegisterPool(pool)

	// 数据点不足
	_, err := predictor.Predict("pool-1", 30*24*time.Hour)
	if err != ErrInsufficientData {
		t.Errorf("expected ErrInsufficientData, got %v", err)
	}
}

func TestPredictWithData(t *testing.T) {
	predictor := NewStoragePredictor(nil)

	pool := &StoragePool{
		ID:         "pool-1",
		Name:       "Test Pool",
		TotalBytes: 1024 * 1024 * 1024 * 100, // 100GB
	}
	predictor.RegisterPool(pool)

	// 添加足够的数据点
	for i := 0; i < 15; i++ {
		usage := int64(1024 * 1024 * 1024 * (30 + i)) // 30GB + i GB
		predictor.RecordUsage("pool-1", usage)
		time.Sleep(10 * time.Millisecond) // 模拟时间间隔
	}

	// 预测
	result, err := predictor.Predict("pool-1", 30*24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result == nil {
		t.Fatal("expected prediction result")
	}

	if result.Predicted <= 0 {
		t.Errorf("expected positive prediction, got %f", result.Predicted)
	}

	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("confidence should be between 0 and 1, got %f", result.Confidence)
	}
}

func TestGetStats(t *testing.T) {
	predictor := NewStoragePredictor(nil)

	pool1 := &StoragePool{
		ID:         "pool-1",
		Name:       "Pool 1",
		TotalBytes: 1024 * 1024 * 1024 * 100,
	}
	pool2 := &StoragePool{
		ID:         "pool-2",
		Name:       "Pool 2",
		TotalBytes: 1024 * 1024 * 1024 * 200,
	}

	predictor.RegisterPool(pool1)
	predictor.RegisterPool(pool2)

	predictor.RecordUsage("pool-1", 1024*1024*1024*50)
	predictor.RecordUsage("pool-2", 1024*1024*1024*100)

	stats := predictor.GetStats()

	if stats["total_pools"] != 2 {
		t.Errorf("expected 2 pools, got %v", stats["total_pools"])
	}

	totalCapacity := stats["total_capacity"].(int64)
	expectedCapacity := int64(1024*1024*1024*100 + 1024*1024*1024*200)
	if totalCapacity != expectedCapacity {
		t.Errorf("expected capacity %d, got %d", expectedCapacity, totalCapacity)
	}
}

func TestListPools(t *testing.T) {
	predictor := NewStoragePredictor(nil)

	pool1 := &StoragePool{ID: "pool-1", Name: "Pool 1", TotalBytes: 100}
	pool2 := &StoragePool{ID: "pool-2", Name: "Pool 2", TotalBytes: 200}

	predictor.RegisterPool(pool1)
	predictor.RegisterPool(pool2)

	pools := predictor.ListPools()
	if len(pools) != 2 {
		t.Errorf("expected 2 pools, got %d", len(pools))
	}
}

func TestGetHistory(t *testing.T) {
	predictor := NewStoragePredictor(nil)

	pool := &StoragePool{
		ID:         "pool-1",
		Name:       "Test Pool",
		TotalBytes: 1024 * 1024 * 1024 * 100,
	}
	predictor.RegisterPool(pool)

	// 添加数据
	for i := 0; i < 5; i++ {
		predictor.RecordUsage("pool-1", int64(1024*1024*1024*(30+i)))
		time.Sleep(10 * time.Millisecond)
	}

	// 获取历史
	history := predictor.GetHistory("pool-1", 1*time.Hour)
	if len(history) != 5 {
		t.Errorf("expected 5 history points, got %d", len(history))
	}
}

func TestLinearRegressionModel(t *testing.T) {
	model := &LinearRegressionModel{}

	if model.Name() != ModelLinear {
		t.Errorf("expected linear model, got %s", model.Name())
	}

	if model.MinDataPoints() != 2 {
		t.Errorf("expected 2 min data points, got %d", model.MinDataPoints())
	}

	// 测试预测
	data := []DataPoint{
		{Timestamp: time.Now().Add(-2 * 24 * time.Hour), Value: 100},
		{Timestamp: time.Now().Add(-1 * 24 * time.Hour), Value: 110},
		{Timestamp: time.Now(), Value: 120},
	}

	result, err := model.Predict(data, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Predicted <= 0 {
		t.Errorf("expected positive prediction, got %f", result.Predicted)
	}
}

func TestExponentialGrowthModel(t *testing.T) {
	model := &ExponentialGrowthModel{}

	if model.Name() != ModelExponential {
		t.Errorf("expected exponential model, got %s", model.Name())
	}

	// 测试预测
	data := []DataPoint{
		{Timestamp: time.Now().Add(-3 * 24 * time.Hour), Value: 100},
		{Timestamp: time.Now().Add(-2 * 24 * time.Hour), Value: 110},
		{Timestamp: time.Now().Add(-1 * 24 * time.Hour), Value: 121},
		{Timestamp: time.Now(), Value: 133},
	}

	result, err := model.Predict(data, 7*24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Predicted <= 0 {
		t.Errorf("expected positive prediction, got %f", result.Predicted)
	}
}

func TestEnsembleModel(t *testing.T) {
	model := &EnsembleModel{}

	if model.Name() != ModelEnsemble {
		t.Errorf("expected ensemble model, got %s", model.Name())
	}

	// 测试预测
	data := make([]DataPoint, 15)
	for i := 0; i < 15; i++ {
		data[i] = DataPoint{
			Timestamp: time.Now().Add(-time.Duration(15-i) * 24 * time.Hour),
			Value:     float64(100 + i*10),
		}
	}

	result, err := model.Predict(data, 30*24*time.Hour)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.Predicted <= 0 {
		t.Errorf("expected positive prediction, got %f", result.Predicted)
	}

	if result.Confidence < 0 || result.Confidence > 1 {
		t.Errorf("confidence should be between 0 and 1, got %f", result.Confidence)
	}
}

func TestAlertLevelCalculation(t *testing.T) {
	predictor := NewStoragePredictor(nil)

	pool := &StoragePool{
		ID:         "pool-1",
		Name:       "Test Pool",
		TotalBytes: 1024 * 1024 * 1024 * 100, // 100GB
	}
	predictor.RegisterPool(pool)

	// 测试正常情况
	predictor.RecordUsage("pool-1", 1024*1024*1024*50) // 50GB = 50%
	pool, _ = predictor.GetPool("pool-1")

	result := &PredictionResult{DaysToFull: 365}
	level := predictor.calculateAlertLevel(pool, result)
	if level != AlertNormal {
		t.Errorf("expected normal alert, got %s", level)
	}

	// 测试警告情况
	predictor.RecordUsage("pool-1", 1024*1024*1024*85) // 85GB = 85%
	pool, _ = predictor.GetPool("pool-1")

	level = predictor.calculateAlertLevel(pool, result)
	if level != AlertWarning {
		t.Errorf("expected warning alert, got %s", level)
	}

	// 测试严重情况
	predictor.RecordUsage("pool-1", 1024*1024*1024*92) // 92GB = 92%
	pool, _ = predictor.GetPool("pool-1")

	level = predictor.calculateAlertLevel(pool, result)
	if level != AlertCritical {
		t.Errorf("expected critical alert, got %s", level)
	}

	// 测试满载情况
	predictor.RecordUsage("pool-1", 1024*1024*1024*96) // 96GB = 96%
	pool, _ = predictor.GetPool("pool-1")

	level = predictor.calculateAlertLevel(pool, result)
	if level != AlertFull {
		t.Errorf("expected full alert, got %s", level)
	}
}
