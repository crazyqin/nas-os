package storageforecast

import (
	"context"
	"testing"
	"time"
)

func TestForecast_RegisterPool(t *testing.T) {
	config := DefaultConfig()
	fc := NewForecast(config)

	pool := StoragePool{
		ID:         "pool1",
		Name:       "主存储池",
		TotalBytes: 1024 * 1024 * 1024 * 1024, // 1TB
		UsedBytes:  512 * 1024 * 1024 * 1024,  // 512GB
		FreeBytes:  512 * 1024 * 1024 * 1024,
	}

	fc.RegisterPool(pool)

	pools := fc.ListPools()
	if len(pools) != 1 {
		t.Fatalf("应有 1 个存储池，实际 %d", len(pools))
	}

	if pools[0].Name != "主存储池" {
		t.Errorf("池名不匹配: %s", pools[0].Name)
	}
}

func TestForecast_UpdateUsage(t *testing.T) {
	config := DefaultConfig()
	fc := NewForecast(config)

	pool := StoragePool{
		ID:         "pool1",
		Name:       "测试池",
		TotalBytes: 1000,
		UsedBytes:  500,
		FreeBytes:  500,
	}
	fc.RegisterPool(pool)

	err := fc.UpdatePoolUsage("pool1", 600)
	if err != nil {
		t.Fatalf("更新使用量失败: %v", err)
	}

	p, _ := fc.GetPool("pool1")
	if p.UsedBytes != 600 {
		t.Errorf("使用量不匹配: %d", p.UsedBytes)
	}

	if p.FreeBytes != 400 {
		t.Errorf("剩余空间不匹配: %d", p.FreeBytes)
	}
}

func TestForecast_UpdateNonExistent(t *testing.T) {
	config := DefaultConfig()
	fc := NewForecast(config)

	err := fc.UpdatePoolUsage("nonexistent", 100)
	if err == nil {
		t.Fatal("不存在的池应返回错误")
	}
}

func TestForecast_AlertLevels(t *testing.T) {
	config := DefaultConfig()
	config.WarningThreshold = 80
	config.CriticalThreshold = 90
	config.FullThreshold = 95

	fc := NewForecast(config)

	tests := []struct {
		name     string
		usage    float64
		expected AlertLevel
	}{
		{"正常", 50, AlertInfo},
		{"警告", 85, AlertWarning},
		{"严重", 92, AlertCritical},
		{"满载", 96, AlertFull},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := StoragePool{
				ID:         tt.name,
				Name:       tt.name,
				TotalBytes: 1000,
				UsedBytes:  int64(tt.usage * 10),
				FreeBytes:  int64((100 - tt.usage) * 10),
			}
			fc.RegisterPool(pool)
			fc.UpdatePoolUsage(tt.name, pool.UsedBytes)

			// 检查告警
			alerts := fc.GetAlerts(false)
			found := false
			for _, a := range alerts {
				if a.PoolID == tt.name {
					found = true
					if a.Level != tt.expected {
						t.Errorf("告警级别不匹配: %s vs %s", a.Level, tt.expected)
					}
				}
			}

			if tt.expected != AlertInfo && !found {
				t.Errorf("未找到 %s 的告警", tt.name)
			}
		})
	}
}

func TestForecast_GetForecast(t *testing.T) {
	config := DefaultConfig()
	config.MinDataPoints = 3
	fc := NewForecast(config)

	pool := StoragePool{
		ID:         "pool1",
		Name:       "测试池",
		TotalBytes: 10000,
		UsedBytes:  5000,
		FreeBytes:  5000,
	}
	fc.RegisterPool(pool)

	// 数据点不足
	result, _ := fc.GetForecast("pool1")
	if result.Trend != TrendUnknown {
		t.Error("数据点不足时趋势应为 unknown")
	}

	// 添加足够数据点
	for i := 0; i < 10; i++ {
		fc.UpdatePoolUsage("pool1", int64(5000+i*100))
		time.Sleep(10 * time.Millisecond)
	}

	result, _ = fc.GetForecast("pool1")
	if result.PoolID != "pool1" {
		t.Errorf("池 ID 不匹配: %s", result.PoolID)
	}
}

func TestForecast_AllForecasts(t *testing.T) {
	config := DefaultConfig()
	fc := NewForecast(config)

	fc.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})
	fc.RegisterPool(StoragePool{ID: "p2", Name: "池2", TotalBytes: 2000, UsedBytes: 1000, FreeBytes: 1000})

	results := fc.GetAllForecasts()
	if len(results) != 2 {
		t.Errorf("应有 2 个预测结果，实际 %d", len(results))
	}
}

func TestForecast_Snapshots(t *testing.T) {
	config := DefaultConfig()
	fc := NewForecast(config)

	pool := StoragePool{ID: "pool1", Name: "测试", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500}
	fc.RegisterPool(pool)

	// 添加快照
	for i := 0; i < 5; i++ {
		fc.UpdatePoolUsage("pool1", int64(500+i*10))
	}

	snapshots := fc.GetSnapshots("pool1", 1*time.Hour)
	if len(snapshots) < 5 {
		t.Errorf("应至少有 5 个快照，实际 %d", len(snapshots))
	}
}

func TestForecast_DismissAlert(t *testing.T) {
	config := DefaultConfig()
	config.WarningThreshold = 50
	fc := NewForecast(config)

	pool := StoragePool{ID: "pool1", Name: "测试", TotalBytes: 100, UsedBytes: 60, FreeBytes: 40}
	fc.RegisterPool(pool)
	fc.UpdatePoolUsage("pool1", 60)

	alerts := fc.GetAlerts(false)
	if len(alerts) == 0 {
		t.Fatal("应有告警")
	}

	err := fc.DismissAlert(alerts[0].ID)
	if err != nil {
		t.Fatalf("忽略告警失败: %v", err)
	}

	activeAlerts := fc.GetAlerts(false)
	for _, a := range activeAlerts {
		if a.ID == alerts[0].ID {
			t.Error("被忽略的告警不应出现在活跃列表中")
		}
	}
}

func TestForecast_Stats(t *testing.T) {
	config := DefaultConfig()
	fc := NewForecast(config)

	fc.RegisterPool(StoragePool{ID: "p1", Name: "池1", TotalBytes: 1000, UsedBytes: 500, FreeBytes: 500})

	stats := fc.GetStats()
	if stats["total_pools"] != 1 {
		t.Errorf("总池数应为 1，实际 %v", stats["total_pools"])
	}
}

func TestForecast_ContextCancel(t *testing.T) {
	config := DefaultConfig()
	fc := NewForecast(config)

	ctx, cancel := context.WithCancel(context.Background())
	fc.Start(ctx)

	// 取消 context
	cancel()
	time.Sleep(100 * time.Millisecond)

	// 不应 panic
	fc.Stop()
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1024, "1.0 KB"},
		{1048576, "1.0 MB"},
		{1073741824, "1.0 GB"},
		{1099511627776, "1.0 TB"},
	}

	for _, tt := range tests {
		result := FormatBytes(tt.bytes)
		if result != tt.expected {
			t.Errorf("FormatBytes(%d) = %s, 期望 %s", tt.bytes, result, tt.expected)
		}
	}
}
