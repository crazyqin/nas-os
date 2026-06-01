package containerimagecache

import (
	"testing"
	"time"
)

func TestDefaultCacheConfig(t *testing.T) {
	tests := []struct {
		name string
		check func(*CacheConfig)
	}{
		{
			name: "默认策略为LRU",
			check: func(c *CacheConfig) {
				if c.Strategy != StrategyLRU {
					t.Errorf("期望策略 %s, 实际 %s", StrategyLRU, c.Strategy)
				}
			},
		},
		{
			name: "默认最大镜像数为1000",
			check: func(c *CacheConfig) {
				if c.MaxImages != 1000 {
					t.Errorf("期望MaxImages 1000, 实际 %d", c.MaxImages)
				}
			},
		},
		{
			name: "默认监听端口为5000",
			check: func(c *CacheConfig) {
				if c.ListenPort != 5000 {
					t.Errorf("期望ListenPort 5000, 实际 %d", c.ListenPort)
				}
			},
		},
		{
			name: "默认预取已启用",
			check: func(c *CacheConfig) {
				if !c.PrefetchEnabled {
					t.Error("期望PrefetchEnabled为true")
				}
			},
		},
		{
			name: "默认TTL为7天",
			check: func(c *CacheConfig) {
				if c.DefaultTTL != 7*24*time.Hour {
					t.Errorf("期望DefaultTTL 7天, 实际 %s", c.DefaultTTL)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg := DefaultCacheConfig()
			tt.check(cfg)
		})
	}
}

func TestNew(t *testing.T) {
	tests := []struct {
		name   string
		config *CacheConfig
	}{
		{name: "nil配置使用默认值", config: nil},
		{name: "自定义配置", config: &CacheConfig{MaxSize: 1024, MaxImages: 10, Strategy: StrategyLFU, ListenPort: 8080}},
		{name: "空配置", config: &CacheConfig{}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr := New(tt.config)
			if mgr == nil {
				t.Fatal("New返回nil")
			}
			if mgr.cache == nil {
				t.Error("cache map未初始化")
			}
			if mgr.registries == nil {
				t.Error("registries map未初始化")
			}
			if mgr.stats == nil {
				t.Error("stats未初始化")
			}
		})
	}
}

func TestNewDefaultRegistries(t *testing.T) {
	mgr := New(nil)
	regs := mgr.GetRegistries()

	expectedTypes := []RegistryType{RegistryDockerHub, RegistryGHCR, RegistryAliyun}
	for _, rt := range expectedTypes {
		if _, exists := regs[rt]; !exists {
			t.Errorf("默认仓库 %s 未注册", rt)
		}
	}
}

func TestAddRemoveRegistry(t *testing.T) {
	mgr := New(nil)

	tests := []struct {
		name      string
		config    *RegistryConfig
		wantError bool
	}{
		{
			name:      "空类型应报错",
			config:    &RegistryConfig{URL: "https://example.com"},
			wantError: true,
		},
		{
			name:      "空URL应报错",
			config:    &RegistryConfig{Type: RegistryCustom},
			wantError: true,
		},
		{
			name:      "有效配置添加成功",
			config:    &RegistryConfig{Type: RegistryCustom, URL: "https://my-registry.com", Enabled: true, Priority: 5},
			wantError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.AddRegistry(tt.config)
			if (err != nil) != tt.wantError {
				t.Errorf("期望错误=%v, 实际错误=%v", tt.wantError, err)
			}
		})
	}

	// 移除不存在的仓库应报错
	if err := mgr.RemoveRegistry("nonexistent"); err == nil {
		t.Error("移除不存在的仓库应返回错误")
	}

	// 移除已添加的自定义仓库
	if err := mgr.RemoveRegistry(RegistryCustom); err != nil {
		t.Errorf("移除仓库失败: %v", err)
	}
}

func TestAddPrefetchRule(t *testing.T) {
	mgr := New(nil)

	rule := &PrefetchRule{
		Name:         "test-rule",
		ImagePattern: "nginx*",
		Enabled:      true,
		Tags:         []string{"latest"},
	}
	mgr.AddPrefetchRule(rule)

	rules := mgr.GetPrefetchRules()
	if len(rules) != 1 {
		t.Errorf("期望1条规则, 实际 %d", len(rules))
	}
	if rules[0].Name != "test-rule" {
		t.Errorf("规则名称不匹配: %s", rules[0].Name)
	}
}

func TestSetBandwidthLimit(t *testing.T) {
	mgr := New(nil)

	tests := []struct {
		name  string
		limit int64
	}{
		{name: "设置10MB/s", limit: 10 * 1024 * 1024},
		{name: "设置0表示不限制", limit: 0},
		{name: "设置100MB/s", limit: 100 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mgr.SetBandwidthLimit(tt.limit)
			cfg := mgr.GetCacheConfig()
			if cfg.BandwidthLimit != tt.limit {
				t.Errorf("期望带宽限制 %d, 实际 %d", tt.limit, cfg.BandwidthLimit)
			}
		})
	}
}

func TestPinUnpin(t *testing.T) {
	mgr := New(nil)

	// 手动往缓存里塞一个镜像
	mgr.mu.Lock()
	mgr.cache["nginx:latest"] = &ImageInfo{
		Name:     "nginx:latest",
		Size:     1024,
		Registry: RegistryDockerHub,
	}
	mgr.mu.Unlock()

	// 固定镜像
	if err := mgr.Pin("nginx:latest"); err != nil {
		t.Fatalf("Pin失败: %v", err)
	}

	info, _ := mgr.GetImageInfo("nginx:latest")
	if !info.IsPinned {
		t.Error("镜像应该被固定")
	}

	// 取消固定
	if err := mgr.Unpin("nginx:latest"); err != nil {
		t.Fatalf("Unpin失败: %v", err)
	}

	info, _ = mgr.GetImageInfo("nginx:latest")
	if info.IsPinned {
		t.Error("镜像应该取消固定")
	}

	// Pin不存在的镜像
	if err := mgr.Pin("nonexistent"); err == nil {
		t.Error("Pin不存在的镜像应返回错误")
	}
}

func TestDelete(t *testing.T) {
	mgr := New(nil)

	mgr.mu.Lock()
	mgr.cache["alpine:3.18"] = &ImageInfo{Name: "alpine:3.18", Size: 500}
	mgr.stats.TotalImages = 1
	mgr.stats.TotalSize = 500
	mgr.mu.Unlock()

	if err := mgr.Delete("alpine:3.18"); err != nil {
		t.Fatalf("Delete失败: %v", err)
	}

	if mgr.IsCached("alpine:3.18") {
		t.Error("镜像应已被删除")
	}

	if mgr.GetCacheImageCount() != 0 {
		t.Errorf("期望镜像数0, 实际 %d", mgr.GetCacheImageCount())
	}

	// 删除不存在的镜像
	if err := mgr.Delete("nonexistent"); err == nil {
		t.Error("删除不存在的镜像应返回错误")
	}
}

func TestClearCache(t *testing.T) {
	mgr := New(nil)

	mgr.mu.Lock()
	mgr.cache["a"] = &ImageInfo{Name: "a", Size: 100}
	mgr.cache["b"] = &ImageInfo{Name: "b", Size: 200}
	mgr.stats.TotalImages = 2
	mgr.stats.TotalSize = 300
	mgr.mu.Unlock()

	mgr.ClearCache()

	if mgr.GetCacheImageCount() != 0 {
		t.Errorf("期望镜像数0, 实际 %d", mgr.GetCacheImageCount())
	}
	if mgr.GetCacheSize() != 0 {
		t.Errorf("期望缓存大小0, 实际 %d", mgr.GetCacheSize())
	}
}

func TestGetStats(t *testing.T) {
	mgr := New(nil)

	stats := mgr.GetStats()
	if stats == nil {
		t.Fatal("GetStats返回nil")
	}
	if stats.TotalImages != 0 {
		t.Errorf("初始镜像数应为0, 实际 %d", stats.TotalImages)
	}
}

func TestGetCacheConfig(t *testing.T) {
	cfg := &CacheConfig{MaxSize: 999, MaxImages: 42, Strategy: StrategyFIFO, ListenPort: 1234}
	mgr := New(cfg)

	got := mgr.GetCacheConfig()
	if got.MaxSize != 999 {
		t.Errorf("MaxSize不匹配: %d", got.MaxSize)
	}
	if got.MaxImages != 42 {
		t.Errorf("MaxImages不匹配: %d", got.MaxImages)
	}
	if got.Strategy != StrategyFIFO {
		t.Errorf("Strategy不匹配: %s", got.Strategy)
	}
}

func TestListCachedImages(t *testing.T) {
	mgr := New(nil)

	mgr.mu.Lock()
	mgr.cache["nginx:latest"] = &ImageInfo{Name: "nginx:latest", Size: 100, LastAccessed: time.Now()}
	mgr.cache["redis:7"] = &ImageInfo{Name: "redis:7", Size: 50, LastAccessed: time.Now().Add(-time.Hour)}
	mgr.mu.Unlock()

	images := mgr.ListCachedImages()
	if len(images) != 2 {
		t.Fatalf("期望2个镜像, 实际 %d", len(images))
	}
	// 应按最后访问时间降序排列
	if images[0].Name != "nginx:latest" {
		t.Errorf("第一个镜像应为 nginx:latest, 实际 %s", images[0].Name)
	}
}

func TestUpdateCacheConfig(t *testing.T) {
	mgr := New(nil)
	newCfg := &CacheConfig{MaxSize: 999, MaxImages: 50, Strategy: StrategyLFU, BandwidthLimit: 1024}
	mgr.UpdateCacheConfig(newCfg)

	got := mgr.GetCacheConfig()
	if got.MaxSize != 999 {
		t.Errorf("MaxSize应为999, 实际 %d", got.MaxSize)
	}
}

func TestIsCached(t *testing.T) {
	mgr := New(nil)

	if mgr.IsCached("any-image") {
		t.Error("空缓存不应包含任何镜像")
	}

	mgr.mu.Lock()
	mgr.cache["my-image"] = &ImageInfo{Name: "my-image", Size: 100}
	mgr.mu.Unlock()

	if !mgr.IsCached("my-image") {
		t.Error("应能查到已缓存的镜像")
	}
}

func TestNewBandwidthLimiter(t *testing.T) {
	tests := []struct {
		name  string
		limit int64
	}{
		{name: "有限制", limit: 1024},
		{name: "无限制", limit: 0},
		{name: "大限制", limit: 100 * 1024 * 1024},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			bl := NewBandwidthLimiter(tt.limit)
			if bl == nil {
				t.Fatal("NewBandwidthLimiter返回nil")
			}
			if bl.limit != tt.limit {
				t.Errorf("期望limit=%d, 实际=%d", tt.limit, bl.limit)
			}
		})
	}
}

func TestBandwidthLimiterAcquireUnlimited(t *testing.T) {
	bl := NewBandwidthLimiter(0)
	// 不限制时不应阻塞
	bl.Acquire(1024 * 1024)
}

func TestBandwidthLimiterSetLimit(t *testing.T) {
	bl := NewBandwidthLimiter(1024)
	bl.SetLimit(2048)
	if bl.limit != 2048 {
		t.Errorf("期望limit=2048, 实际=%d", bl.limit)
	}
}

func TestStartStop(t *testing.T) {
	mgr := New(nil)

	if err := mgr.Start(); err != nil {
		t.Fatalf("Start失败: %v", err)
	}

	// 停止
	mgr.Stop()
	// 等待goroutine退出
	time.Sleep(100 * time.Millisecond)
}
