package optimizer

import (
	"sync"
	"testing"
	"time"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg == nil {
		t.Fatal("DefaultConfig() returned nil")
	}

	if !cfg.CacheEnabled {
		t.Error("Expected CacheEnabled to be true")
	}
	if cfg.CacheCapacity <= 0 {
		t.Error("Expected positive CacheCapacity")
	}
	if cfg.CacheTTL <= 0 {
		t.Error("Expected positive CacheTTL")
	}
	if !cfg.GCEnabled {
		t.Error("Expected GCEnabled to be true")
	}
	if !cfg.PoolEnabled {
		t.Error("Expected PoolEnabled to be true")
	}
}

func TestNewOptimizer(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GCInterval = time.Minute // 确保有效的 GCInterval

	opt := NewOptimizer(cfg, nil)

	if opt == nil {
		t.Fatal("NewOptimizer() returned nil")
	}

	if !opt.initialized {
		t.Error("Optimizer should be initialized")
	}

	// 等待 goroutine 启动
	time.Sleep(10 * time.Millisecond)
	opt.Stop()
}

func TestNewOptimizerWithNilConfig(t *testing.T) {
	opt := NewOptimizer(nil, nil)

	if opt == nil {
		t.Fatal("NewOptimizer(nil) returned nil")
	}

	// 应该使用默认配置
	if opt.config == nil {
		t.Error("Config should not be nil")
	}

	// 等待 goroutine 启动
	time.Sleep(10 * time.Millisecond)
	opt.Stop()
}

func TestCacheGetSet(t *testing.T) {
	cfg := &Config{
		CacheEnabled:  true,
		CacheCapacity: 100,
		CacheTTL:      time.Minute,
		GCEnabled:     false,
		GCInterval:    time.Minute, // 添加有效的 GCInterval
	}

	opt := NewOptimizer(cfg, nil)
	defer opt.Stop()

	// 测试 Set 和 Get
	opt.CacheSet("key1", "value1")

	val, ok := opt.CacheGet("key1")
	if !ok {
		t.Error("CacheGet() should find key1")
	}
	if val != "value1" {
		t.Errorf("CacheGet() = %v, expected 'value1'", val)
	}

	// 测试不存在的键
	_, ok = opt.CacheGet("nonexistent")
	if ok {
		t.Error("CacheGet() should not find nonexistent key")
	}
}

func TestCacheDelete(t *testing.T) {
	cfg := &Config{
		CacheEnabled:  true,
		CacheCapacity: 100,
		CacheTTL:      time.Minute,
		GCEnabled:     false,
		GCInterval:    time.Minute, // 添加有效的 GCInterval
	}

	opt := NewOptimizer(cfg, nil)
	defer opt.Stop()

	opt.CacheSet("key1", "value1")
	opt.CacheDelete("key1")

	_, ok := opt.CacheGet("key1")
	if ok {
		t.Error("CacheGet() should not find deleted key")
	}
}

func TestCacheDisabled(t *testing.T) {
	cfg := &Config{
		CacheEnabled:  false,
		CacheCapacity: 100,
		CacheTTL:      time.Minute,
		GCEnabled:     false,
		GCInterval:    time.Minute, // 添加有效的 GCInterval
	}

	opt := NewOptimizer(cfg, nil)
	defer opt.Stop()

	// 缓存未启用，操作应该无效果
	opt.CacheSet("key1", "value1")

	_, ok := opt.CacheGet("key1")
	if ok {
		t.Error("CacheGet() should not find key when cache is disabled")
	}
}

func TestGetStats(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GCInterval = time.Minute // 确保有效的 GCInterval

	opt := NewOptimizer(cfg, nil)

	// 等待 goroutine 启动
	time.Sleep(10 * time.Millisecond)
	defer opt.Stop()

	stats := opt.GetStats()

	if stats == nil {
		t.Fatal("GetStats() returned nil")
	}

	// 初始状态下，缓存命中和未命中应该是 0
	// (注意：GetStats 可能会触发内部更新)
	if stats.Goroutines < 0 {
		t.Error("Goroutines should not be negative")
	}
}

func TestOptimizeQuery(t *testing.T) {
	cfg := &Config{
		CacheEnabled:  true,
		CacheCapacity: 100,
		CacheTTL:      time.Minute,
		GCEnabled:     false,
		GCInterval:    time.Minute, // 添加有效的 GCInterval
	}

	opt := NewOptimizer(cfg, nil)
	defer opt.Stop()

	callCount := 0
	queryFn := func() (interface{}, error) {
		callCount++
		return "result", nil
	}

	// 第一次调用，应该执行查询函数
	result, err := opt.OptimizeQuery("query1", queryFn)
	if err != nil {
		t.Errorf("OptimizeQuery() returned error: %v", err)
	}
	if result != "result" {
		t.Errorf("OptimizeQuery() = %v, expected 'result'", result)
	}
	if callCount != 1 {
		t.Errorf("queryFn called %d times, expected 1", callCount)
	}

	// 第二次调用，应该从缓存获取
	_, err = opt.OptimizeQuery("query1", queryFn)
	if err != nil {
		t.Errorf("OptimizeQuery() returned error: %v", err)
	}
	if callCount != 1 {
		t.Errorf("queryFn should not be called again, but was called %d times", callCount)
	}

	// 检查优化统计
	stats := opt.GetStats()
	if stats.Optimizations != 1 {
		t.Errorf("Optimizations = %d, expected 1", stats.Optimizations)
	}
}

func TestOptimizeQueryWithCacheDisabled(t *testing.T) {
	cfg := &Config{
		CacheEnabled:  false,
		CacheCapacity: 100,
		CacheTTL:      time.Minute,
		GCEnabled:     false,
		GCInterval:    time.Minute, // 添加有效的 GCInterval
	}

	opt := NewOptimizer(cfg, nil)
	defer opt.Stop()

	callCount := 0
	queryFn := func() (interface{}, error) {
		callCount++
		return "result", nil
	}

	// 缓存禁用时，每次都应该调用查询函数
	opt.OptimizeQuery("query1", queryFn)
	opt.OptimizeQuery("query1", queryFn)

	if callCount != 2 {
		t.Errorf("queryFn called %d times, expected 2 (cache disabled)", callCount)
	}
}

func TestBatchProcess(t *testing.T) {
	cfg := &Config{
		BatchEnabled: true,
		BatchSize:    3,
		BatchTimeout: 10 * time.Millisecond,
		CacheEnabled: false,
		GCEnabled:    false,
		GCInterval:   time.Minute, // 添加有效的 GCInterval
	}

	opt := NewOptimizer(cfg, nil)
	defer opt.Stop()

	var processed []int
	var mu sync.Mutex

	processFn := func(item interface{}) error {
		mu.Lock()
		processed = append(processed, item.(int))
		mu.Unlock()
		return nil
	}

	items := []interface{}{1, 2, 3, 4, 5}
	err := opt.BatchProcess(items, processFn)

	if err != nil {
		t.Errorf("BatchProcess() returned error: %v", err)
	}

	if len(processed) != 5 {
		t.Errorf("Processed %d items, expected 5", len(processed))
	}
}

func TestBatchProcessDisabled(t *testing.T) {
	cfg := &Config{
		BatchEnabled: false,
		BatchSize:    3,
		CacheEnabled: false,
		GCEnabled:    false,
		GCInterval:   time.Minute, // 添加有效的 GCInterval
	}

	opt := NewOptimizer(cfg, nil)
	defer opt.Stop()

	var count int
	processFn := func(item interface{}) error {
		count++
		return nil
	}

	items := []interface{}{1, 2, 3, 4, 5}
	err := opt.BatchProcess(items, processFn)

	if err != nil {
		t.Errorf("BatchProcess() returned error: %v", err)
	}

	if count != 5 {
		t.Errorf("Processed %d items, expected 5", count)
	}
}

func TestBatchProcessWithError(t *testing.T) {
	cfg := &Config{
		BatchEnabled: true,
		BatchSize:    3,
		CacheEnabled: false,
		GCEnabled:    false,
		GCInterval:   time.Minute, // 添加有效的 GCInterval
	}

	opt := NewOptimizer(cfg, nil)
	defer opt.Stop()

	processFn := func(item interface{}) error {
		if item.(int) == 3 {
			return ErrTestError
		}
		return nil
	}

	items := []interface{}{1, 2, 3, 4, 5}
	err := opt.BatchProcess(items, processFn)

	if err == nil {
		t.Error("Expected error when processing fails")
	}
}

func TestGetConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GCInterval = time.Minute // 确保有效的 GCInterval

	opt := NewOptimizer(cfg, nil)

	// 等待 goroutine 启动
	time.Sleep(10 * time.Millisecond)
	defer opt.Stop()

	result := opt.GetConfig()

	if result == nil {
		t.Fatal("GetConfig() returned nil")
	}

	if result.CacheCapacity != cfg.CacheCapacity {
		t.Errorf("CacheCapacity = %d, expected %d", result.CacheCapacity, cfg.CacheCapacity)
	}
}

func TestUpdateConfig(t *testing.T) {
	cfg := DefaultConfig()
	cfg.GCInterval = time.Minute // 确保有效的 GCInterval

	opt := NewOptimizer(cfg, nil)

	// 等待 goroutine 启动
	time.Sleep(10 * time.Millisecond)
	defer opt.Stop()

	newCfg := &Config{
		CacheEnabled:  false,
		CacheCapacity: 500,
		CacheTTL:      10 * time.Minute,
		GCEnabled:     false,
		GCInterval:    time.Minute, // 确保有效的 GCInterval
	}

	opt.UpdateConfig(newCfg)

	result := opt.GetConfig()
	if result.CacheCapacity != 500 {
		t.Errorf("CacheCapacity = %d, expected 500", result.CacheCapacity)
	}
}

func TestForceGC(t *testing.T) {
	cfg := &Config{
		CacheEnabled: false,
		GCEnabled:    false,
		GCInterval:   time.Minute, // 添加有效的 GCInterval
	}

	opt := NewOptimizer(cfg, nil)
	defer opt.Stop()

	// ForceGC 不应该 panic
	opt.ForceGC()

	stats := opt.GetStats()
	if stats.LastGCTime.IsZero() {
		t.Error("LastGCTime should be set after ForceGC")
	}
}

func TestGetCache(t *testing.T) {
	cfg := &Config{
		CacheEnabled:  true,
		CacheCapacity: 100,
		CacheTTL:      time.Minute,
		GCEnabled:     false,
		GCInterval:    time.Minute, // 添加有效的 GCInterval
	}

	opt := NewOptimizer(cfg, nil)
	defer opt.Stop()

	cache := opt.GetCache()
	if cache == nil {
		t.Error("GetCache() returned nil")
	}
}

// 测试错误.
var ErrTestError = &testError{}

type testError struct{}

func (e *testError) Error() string {
	return "test error"
}
