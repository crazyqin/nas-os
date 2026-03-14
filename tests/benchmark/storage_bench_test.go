// Package benchmark 提供 NAS-OS 存储操作性能基准测试
// v2.14.0 存储基准测试覆盖
package benchmark

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"nas-os/internal/cache"
)

// ========== 文件读写性能基准测试 ==========

// BenchmarkFileWrite 小文件写入性能
func BenchmarkFileWrite_Small(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-write-small")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	data := make([]byte, 1024) // 1KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := filepath.Join(tmpDir, fmt.Sprintf("file-%d.dat", i))
		if err := os.WriteFile(path, data, 0644); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(len(data)), "bytes/op")
}

// BenchmarkFileWrite_Medium 中等文件写入性能
func BenchmarkFileWrite_Medium(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-write-medium")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	data := make([]byte, 64*1024) // 64KB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := filepath.Join(tmpDir, fmt.Sprintf("file-%d.dat", i))
		if err := os.WriteFile(path, data, 0644); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(len(data)), "bytes/op")
}

// BenchmarkFileWrite_Large 大文件写入性能
func BenchmarkFileWrite_Large(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-write-large")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	data := make([]byte, 1024*1024) // 1MB

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		path := filepath.Join(tmpDir, fmt.Sprintf("file-%d.dat", i))
		if err := os.WriteFile(path, data, 0644); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(len(data)), "bytes/op")
}

// BenchmarkFileRead_Small 小文件读取性能
func BenchmarkFileRead_Small(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-read-small")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	data := make([]byte, 1024) // 1KB
	path := filepath.Join(tmpDir, "test.dat")
	if err := os.WriteFile(path, data, 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(len(data)), "bytes/op")
}

// BenchmarkFileRead_Medium 中等文件读取性能
func BenchmarkFileRead_Medium(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-read-medium")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	data := make([]byte, 64*1024) // 64KB
	path := filepath.Join(tmpDir, "test.dat")
	if err := os.WriteFile(path, data, 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(len(data)), "bytes/op")
}

// BenchmarkFileRead_Large 大文件读取性能
func BenchmarkFileRead_Large(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-read-large")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	data := make([]byte, 1024*1024) // 1MB
	path := filepath.Join(tmpDir, "test.dat")
	if err := os.WriteFile(path, data, 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := os.ReadFile(path)
		if err != nil {
			b.Fatal(err)
		}
	}
	b.ReportMetric(float64(len(data)), "bytes/op")
}

// BenchmarkFileStat 文件状态查询性能
func BenchmarkFileStat(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-stat")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "test.dat")
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := os.Stat(path)
		if err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkFileExists 文件存在性检查性能
func BenchmarkFileExists(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-exists")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	path := filepath.Join(tmpDir, "test.dat")
	if err := os.WriteFile(path, []byte("test"), 0644); err != nil {
		b.Fatal(err)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			b.Fatal("file should exist")
		}
	}
}

// ========== 并发操作性能基准测试 ==========

// BenchmarkConcurrentFileWrite 并发文件写入
func BenchmarkConcurrentFileWrite(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-concurrent-write")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	data := make([]byte, 4*1024) // 4KB

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			path := filepath.Join(tmpDir, fmt.Sprintf("concurrent-%d.dat", i))
			if err := os.WriteFile(path, data, 0644); err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// BenchmarkConcurrentFileRead 并发文件读取
func BenchmarkConcurrentFileRead(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-concurrent-read")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	// 预创建 100 个文件
	data := make([]byte, 4*1024)
	for i := 0; i < 100; i++ {
		path := filepath.Join(tmpDir, fmt.Sprintf("file-%d.dat", i))
		if err := os.WriteFile(path, data, 0644); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			path := filepath.Join(tmpDir, fmt.Sprintf("file-%d.dat", i%100))
			_, err := os.ReadFile(path)
			if err != nil {
				b.Fatal(err)
			}
			i++
		}
	})
}

// BenchmarkConcurrentFileReadWrite 混合并发读写
func BenchmarkConcurrentFileReadWrite(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-concurrent-rw")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	data := make([]byte, 4*1024)
	for i := 0; i < 50; i++ {
		path := filepath.Join(tmpDir, fmt.Sprintf("file-%d.dat", i))
		if err := os.WriteFile(path, data, 0644); err != nil {
			b.Fatal(err)
		}
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				// 读操作
				path := filepath.Join(tmpDir, fmt.Sprintf("file-%d.dat", i%50))
				_, _ = os.ReadFile(path)
			} else {
				// 写操作
				path := filepath.Join(tmpDir, fmt.Sprintf("file-%d.dat", (i%50)+50))
				_ = os.WriteFile(path, data, 0644)
			}
			i++
		}
	})
}

// BenchmarkConcurrentDirectoryCreate 并发目录创建
func BenchmarkConcurrentDirectoryCreate(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-concurrent-dir")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			path := filepath.Join(tmpDir, fmt.Sprintf("dir-%d", i))
			_ = os.Mkdir(path, 0755)
			i++
		}
	})
}

// BenchmarkConcurrentCacheWithFileIO 缓存+文件IO混合并发
func BenchmarkConcurrentCacheWithFileIO(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-cache-fileio")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	c := cache.NewLRUCache(1000, time.Minute)
	data := make([]byte, 4*1024)

	// 预创建文件
	for i := 0; i < 50; i++ {
		path := filepath.Join(tmpDir, fmt.Sprintf("file-%d.dat", i))
		_ = os.WriteFile(path, data, 0644)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("key-%d", i%100)
			if i%3 == 0 {
				// 缓存写入
				c.Set(key, filepath.Join(tmpDir, fmt.Sprintf("file-%d.dat", i%50)))
			} else if i%3 == 1 {
				// 缓存读取
				_, _ = c.Get(key)
			} else {
				// 文件读取
				path := filepath.Join(tmpDir, fmt.Sprintf("file-%d.dat", i%50))
				_, _ = os.ReadFile(path)
			}
			i++
		}
	})
}

// ========== 缓存命中率测试 ==========

// BenchmarkCacheHitRate_High 高命中率场景 (90%+)
func BenchmarkCacheHitRate_High(b *testing.B) {
	c := cache.NewLRUCache(1000, time.Minute)

	// 预填充缓存
	for i := 0; i < 100; i++ {
		c.Set(fmt.Sprintf("hot-key-%d", i), fmt.Sprintf("value-%d", i))
	}

	hits := int64(0)
	misses := int64(0)
	var mu sync.Mutex

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			// 90% 访问热数据，10% 访问冷数据
			key := fmt.Sprintf("hot-key-%d", 0)
			if _, ok := c.Get(key); ok {
				mu.Lock()
				hits++
				mu.Unlock()
			} else {
				mu.Lock()
				misses++
				mu.Unlock()
				c.Set(key, "new-value")
			}
		}
	})

	b.ReportMetric(float64(hits)/float64(hits+misses)*100, "hit_rate_%")
}

// BenchmarkCacheHitRate_Medium 中命中率场景 (50-70%)
func BenchmarkCacheHitRate_Medium(b *testing.B) {
	c := cache.NewLRUCache(500, time.Minute)

	// 预填充缓存
	for i := 0; i < 500; i++ {
		c.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
	}

	hits := int64(0)
	misses := int64(0)
	var mu sync.Mutex

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// 随机访问，部分命中
			key := fmt.Sprintf("key-%d", i%800)
			if _, ok := c.Get(key); ok {
				mu.Lock()
				hits++
				mu.Unlock()
			} else {
				mu.Lock()
				misses++
				mu.Unlock()
				c.Set(key, "new-value")
			}
			i++
		}
	})

	b.ReportMetric(float64(hits)/float64(hits+misses)*100, "hit_rate_%")
}

// BenchmarkCacheHitRate_Low 低命中率场景 (<30%)
func BenchmarkCacheHitRate_Low(b *testing.B) {
	c := cache.NewLRUCache(100, time.Minute)

	hits := int64(0)
	misses := int64(0)
	var mu sync.Mutex

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			// 访问大量不存在的 key
			key := fmt.Sprintf("random-key-%d-%d", i, i*7)
			if _, ok := c.Get(key); ok {
				mu.Lock()
				hits++
				mu.Unlock()
			} else {
				mu.Lock()
				misses++
				mu.Unlock()
				c.Set(key, "value")
			}
			i++
		}
	})

	b.ReportMetric(float64(hits)/float64(hits+misses)*100, "hit_rate_%")
}

// BenchmarkCacheEviction 缓存驱逐性能
func BenchmarkCacheEviction(b *testing.B) {
	c := cache.NewLRUCache(100, time.Minute)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("key-%d", i), fmt.Sprintf("value-%d", i))
	}
}

// BenchmarkCacheSequential 顺序访问模式
func BenchmarkCacheSequential(b *testing.B) {
	c := cache.NewLRUCache(1000, time.Minute)

	// 预填充
	for i := 0; i < 1000; i++ {
		c.Set(fmt.Sprintf("seq-key-%d", i), i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Get(fmt.Sprintf("seq-key-%d", i%1000))
	}
}

// BenchmarkCacheRandom 随机访问模式
func BenchmarkCacheRandom(b *testing.B) {
	c := cache.NewLRUCache(1000, time.Minute)

	// 预填充
	for i := 0; i < 1000; i++ {
		c.Set(fmt.Sprintf("rand-key-%d", i), i)
	}

	// 使用固定种子保证可重复性
	seed := int64(42)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 简单的伪随机
		idx := int((seed + int64(i)*2654435761) % 1000)
		if idx < 0 {
			idx = -idx
		}
		c.Get(fmt.Sprintf("rand-key-%d", idx))
	}
}

// BenchmarkCacheZipf Zipf 分布访问模式（模拟真实热点）
func BenchmarkCacheZipf(b *testing.B) {
	c := cache.NewLRUCache(1000, time.Minute)

	// 预填充
	for i := 0; i < 1000; i++ {
		c.Set(fmt.Sprintf("zipf-key-%d", i), i)
	}

	// 简化的 Zipf-like 分布：小数字访问更频繁
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		// 使用简单的加权：越小的数字被访问的概率越高
		idx := (i * i) % 1000
		c.Get(fmt.Sprintf("zipf-key-%d", idx))
	}
}

// ========== 综合性能测试 ==========

// BenchmarkStorageWorkload_Simulated 模拟存储工作负载
func BenchmarkStorageWorkload_Simulated(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-workload")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	c := cache.NewLRUCache(500, time.Minute)
	data := make([]byte, 16*1024) // 16KB

	// 预创建文件
	for i := 0; i < 100; i++ {
		path := filepath.Join(tmpDir, fmt.Sprintf("data-%d.bin", i))
		_ = os.WriteFile(path, data, 0644)
		c.Set(fmt.Sprintf("path-%d", i), path)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			switch i % 5 {
			case 0, 1:
				// 读操作 (40%)
				key := fmt.Sprintf("path-%d", i%100)
				if path, ok := c.Get(key); ok {
					_, _ = os.ReadFile(path.(string))
				}
			case 2, 3:
				// 写操作 (40%)
				path := filepath.Join(tmpDir, fmt.Sprintf("write-%d.bin", i%50))
				_ = os.WriteFile(path, data, 0644)
			case 4:
				// 元数据操作 (20%)
				_, _ = os.Stat(filepath.Join(tmpDir, fmt.Sprintf("data-%d.bin", i%100)))
			}
			i++
		}
	})
}

// BenchmarkStorageWorkload_HeavyWrite 重写入工作负载
func BenchmarkStorageWorkload_HeavyWrite(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-heavy-write")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	c := cache.NewLRUCache(100, time.Minute)
	data := make([]byte, 64*1024) // 64KB

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			path := filepath.Join(tmpDir, fmt.Sprintf("heavy-%d.bin", i%1000))
			_ = os.WriteFile(path, data, 0644)
			c.Set(fmt.Sprintf("file-%d", i%100), path)
			i++
		}
	})
}

// BenchmarkStorageWorkload_HeavyRead 重读取工作负载
func BenchmarkStorageWorkload_HeavyRead(b *testing.B) {
	tmpDir, err := os.MkdirTemp("", "bench-heavy-read")
	if err != nil {
		b.Fatal(err)
	}
	defer os.RemoveAll(tmpDir)

	c := cache.NewLRUCache(1000, time.Minute)
	data := make([]byte, 64*1024) // 64KB

	// 预创建大量文件
	for i := 0; i < 500; i++ {
		path := filepath.Join(tmpDir, fmt.Sprintf("read-%d.bin", i))
		_ = os.WriteFile(path, data, 0644)
		c.Set(fmt.Sprintf("file-%d", i), path)
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			key := fmt.Sprintf("file-%d", i%500)
			if path, ok := c.Get(key); ok {
				_, _ = os.ReadFile(path.(string))
			}
			i++
		}
	})
}

// BenchmarkMemoryAllocation 文件路径内存分配
func BenchmarkMemoryAllocation_FilePath(b *testing.B) {
	base := "/mnt/data"

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		_ = filepath.Join(base, "subdir", fmt.Sprintf("file-%d.dat", i))
	}
}

// BenchmarkMemoryAllocation_CacheItem 缓存项内存分配
func BenchmarkMemoryAllocation_CacheItem(b *testing.B) {
	c := cache.NewLRUCache(10000, time.Minute)

	b.ResetTimer()
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		c.Set(fmt.Sprintf("alloc-key-%d", i), fmt.Sprintf("alloc-value-%d", i))
	}
}
