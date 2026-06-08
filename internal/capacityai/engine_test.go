package capacityai

import (
	"testing"
)

func TestNewCapacityAI(t *testing.T) {
	ai := NewCapacityAI()
	if ai == nil {
		t.Fatal("NewCapacityAI returned nil")
	}
}

func TestRegisterPool(t *testing.T) {
	ai := NewCapacityAI()
	pool := StoragePool{
		ID:         "pool-1",
		Name:       "主存储池",
		TotalBytes: 1024 * 1024 * 1024 * 1024, // 1TB
		UsedBytes:  500 * 1024 * 1024 * 1024,   // 500GB
		PoolType:   "raidz1",
		Tier:       "hdd",
	}
	ai.RegisterPool(pool)
	
	pools := ai.GetPools()
	if len(pools) != 1 {
		t.Fatalf("expected 1 pool, got %d", len(pools))
	}
	if pools[0].Name != "主存储池" {
		t.Errorf("unexpected name: %s", pools[0].Name)
	}
	expectedFree := int64(524 * 1024 * 1024 * 1024) // ~524GB
	if pools[0].FreeBytes < expectedFree-1024*1024*1024 || pools[0].FreeBytes > expectedFree+1024*1024*1024 {
		t.Errorf("unexpected free bytes: %d (expected ~%d)", pools[0].FreeBytes, expectedFree)
	}
}

func TestRecordUsage(t *testing.T) {
	ai := NewCapacityAI()
	ai.RegisterPool(StoragePool{
		ID:         "pool-1",
		Name:       "测试池",
		TotalBytes: 1024 * 1024 * 1024 * 1024,
		UsedBytes:  100 * 1024 * 1024 * 1024,
		PoolType:   "single",
		Tier:       "ssd",
	})
	
	ai.RecordUsage("pool-1", 200*1024*1024*1024)
	pools := ai.GetPools()
	if pools[0].UsedBytes != 200*1024*1024*1024 {
		t.Errorf("expected 200GB used, got %d", pools[0].UsedBytes)
	}
}

func TestGetForecasts(t *testing.T) {
	ai := NewCapacityAI()
	forecasts := ai.GetForecasts()
	if forecasts == nil {
		t.Fatal("expected non-nil forecasts")
	}
}

func TestGetOptimizations(t *testing.T) {
	ai := NewCapacityAI()
	opts := ai.GetOptimizations()
	if opts == nil {
		t.Fatal("expected non-nil optimizations")
	}
}
