// Package benchmark 提供 NAS-OS 性能基准测试
// v2.7.0 性能基准测试覆盖
package benchmark

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"nas-os/internal/cache"
	"nas-os/internal/storage"
	"nas-os/internal/version"

	"github.com/gin-gonic/gin"
)

// ========== 版本信息基准测试 ==========

func BenchmarkVersionInfo(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = version.Info()
	}
}

func BenchmarkVersionString(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = version.String()
	}
}

// ========== 存储操作基准测试 ==========

func BenchmarkRAIDConfigLookup(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = storage.RAIDConfigs["raid1"]
	}
}

func BenchmarkRAIDConfigAll(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for k := range storage.RAIDConfigs {
			_ = storage.RAIDConfigs[k]
		}
	}
}

func BenchmarkVolumeCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &storage.Volume{
			Name:        "test-volume",
			UUID:        "test-uuid",
			Devices:     []string{"/dev/sda1", "/dev/sdb1"},
			Size:        2000000000000,
			Used:        1000000000000,
			Free:        1000000000000,
			DataProfile: "raid1",
			MetaProfile: "raid1",
			MountPoint:  "/mnt/data",
			Status:      storage.VolumeStatus{Healthy: true},
		}
	}
}

func BenchmarkSubVolumeCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &storage.SubVolume{
			ID:       256,
			Name:     "documents",
			Path:     "/mnt/data/documents",
			ParentID: 5,
			ReadOnly: false,
			UUID:     "test-uuid",
		}
	}
}

func BenchmarkSnapshotCreation(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = &storage.Snapshot{
			Name:      "daily-snapshot",
			Path:      "/mnt/data/.snapshots/daily",
			Source:    "documents",
			ReadOnly:  true,
			CreatedAt: time.Now(),
		}
	}
}

// ========== 缓存操作基准测试 ==========

func BenchmarkCacheSet(b *testing.B) {
	c := cache.NewLRUCache(1000, time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set("test-key", "test-value")
	}
}

func BenchmarkCacheGet(b *testing.B) {
	c := cache.NewLRUCache(1000, time.Minute)
	c.Set("test-key", "test-value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = c.Get("test-key")
	}
}

func BenchmarkCacheSetGet(b *testing.B) {
	c := cache.NewLRUCache(1000, time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set("test-key", "test-value")
		_, _ = c.Get("test-key")
	}
}

func BenchmarkCacheDelete(b *testing.B) {
	c := cache.NewLRUCache(1000, time.Minute)
	c.Set("test-key", "test-value")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Delete("test-key")
	}
}

func BenchmarkCacheConcurrentAccess(b *testing.B) {
	c := cache.NewLRUCache(1000, time.Minute)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := string(rune(i % 100))
			if i%2 == 0 {
				c.Set(key, "value")
			} else {
				_, _ = c.Get(key)
			}
			i++
		}
	})
}

// ========== JSON 序列化基准测试 ==========

func BenchmarkJSONEncodeVolume(b *testing.B) {
	vol := &storage.Volume{
		Name:        "data",
		UUID:        "test-uuid",
		Devices:     []string{"/dev/sda1", "/dev/sdb1"},
		Size:        2000000000000,
		Used:        1000000000000,
		Free:        1000000000000,
		DataProfile: "raid1",
		MetaProfile: "raid1",
		MountPoint:  "/mnt/data",
		Status:      storage.VolumeStatus{Healthy: true},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(vol)
	}
}

func BenchmarkJSONDecodeVolume(b *testing.B) {
	data := []byte(`{"name":"data","uuid":"test-uuid","devices":["/dev/sda1","/dev/sdb1"],"size":2000000000000,"used":1000000000000,"free":1000000000000,"data_profile":"raid1","meta_profile":"raid1","mount_point":"/mnt/data","status":{"healthy":true}}`)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var vol storage.Volume
		_ = json.Unmarshal(data, &vol)
	}
}

func BenchmarkJSONEncodeSnapshot(b *testing.B) {
	snap := &storage.Snapshot{
		Name:      "daily-20260314",
		Path:      "/mnt/data/.snapshots/daily-20260314",
		Source:    "documents",
		ReadOnly:  true,
		CreatedAt: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = json.Marshal(snap)
	}
}

// ========== HTTP API 基准测试 ==========

func setupBenchmarkRouter() *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	r.Use(gin.Recovery())

	api := r.Group("/api/v1")
	{
		api.GET("/health", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{"status": "ok", "version": "2.7.0"})
		})

		api.GET("/volumes", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"volumes": []gin.H{
					{"name": "data", "size": 1000000000000},
					{"name": "backup", "size": 2000000000000},
				},
			})
		})

		api.GET("/system/info", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"hostname":  "nas-server",
				"version":   "2.7.0",
				"cpu_usage": 25.5,
				"mem_usage": 45.2,
			})
		})

		api.GET("/performance", func(c *gin.Context) {
			c.JSON(http.StatusOK, gin.H{
				"iops":       gin.H{"read": 1000, "write": 500},
				"throughput": gin.H{"read": 104857600, "write": 52428800},
				"latency":    gin.H{"read": 5.2, "write": 8.5},
			})
		})
	}

	return r
}

func BenchmarkAPIHealthEndpoint(b *testing.B) {
	router := setupBenchmarkRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/health", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkAPIVolumesEndpoint(b *testing.B) {
	router := setupBenchmarkRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/volumes", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkAPISystemInfoEndpoint(b *testing.B) {
	router := setupBenchmarkRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/system/info", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

func BenchmarkAPIPerformanceEndpoint(b *testing.B) {
	router := setupBenchmarkRouter()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/performance", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}

// ========== 并发基准测试 ==========

func BenchmarkConcurrentAPIRequests(b *testing.B) {
	router := setupBenchmarkRouter()

	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			req, _ := http.NewRequest("GET", "/api/v1/health", nil)
			w := httptest.NewRecorder()
			router.ServeHTTP(w, req)
		}
	})
}

func BenchmarkConcurrentCacheOperations(b *testing.B) {
	c := cache.NewLRUCache(1000, time.Minute)

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := string(rune(i % 100))
			if i%3 == 0 {
				c.Set(key, "value")
			} else if i%3 == 1 {
				_, _ = c.Get(key)
			} else {
				c.Delete(key)
			}
			i++
		}
	})
}

// ========== 内存分配基准测试 ==========

func BenchmarkAllocationsVolume(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = &storage.Volume{
			Name:        "test",
			Devices:     []string{"/dev/sda1"},
			DataProfile: "single",
			Status:      storage.VolumeStatus{Healthy: true},
		}
	}
}

func BenchmarkAllocationsJSON(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		data, _ := json.Marshal(gin.H{
			"status": "ok",
			"values": []int{1, 2, 3, 4, 5},
		})
		_ = data
	}
}

func BenchmarkAllocationsHTTPResponse(b *testing.B) {
	router := setupBenchmarkRouter()

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		req, _ := http.NewRequest("GET", "/api/v1/volumes", nil)
		w := httptest.NewRecorder()
		router.ServeHTTP(w, req)
	}
}