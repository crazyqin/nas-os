// Package smb SMB Multi-Channel 性能基准测试
package smb

import (
	"fmt"
	"math/rand"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// ============================================================================
// SMB Multi-Channel 基准测试套件
// 测试场景：单连接、多通道（2/4通道）、大文件、小文件、并发
// 输出指标：吞吐量(MB/s)、IOPS、延迟(ms)
// ============================================================================

// 缓冲区大小常量（优化值）.
const (
	smallBufferSize  = 64 * 1024   // 64KB - 小文件场景
	mediumBufferSize = 256 * 1024  // 256KB - 中等文件
	largeBufferSize  = 1024 * 1024 // 1MB - 大文件/顺序读写
)

// ==============================================================================
// 1. 模拟 SMB Multi-Channel 传输基准测试
// ==============================================================================

// mockChannel 模拟SMB通道.
type mockChannel struct {
	id            int
	ip            string
	bandwidthMbps int
	connected     bool
	latencyMs     int64
}

// newMockChannels 创建模拟通道.
func newMockChannels(count int) []*mockChannel {
	channels := make([]*mockChannel, count)
	for i := 0; i < count; i++ {
		channels[i] = &mockChannel{
			id:            i + 1,
			ip:            fmt.Sprintf("192.168.1.%d", 10+i),
			bandwidthMbps: 1000,
			connected:     true,
			latencyMs:     int64(1 + rand.Intn(5)), // 1-5ms
		}
	}
	return channels
}

// toMB 转换bytes到MB.
func toMB(bytes int64) float64 {
	return float64(bytes) / float64(1024*1024)
}

// BenchmarkMultiChannel_SingleConnection 单连接基准测试（基线）.
func BenchmarkMultiChannel_SingleConnection(b *testing.B) {
	channels := newMockChannels(1)
	totalBytes := int64(largeBufferSize * 100)
	var transferred int64

	buf := make([]byte, largeBufferSize)
	rand.Read(buf)

	b.ResetTimer()
	b.SetBytes(largeBufferSize)

	for i := 0; i < b.N; i++ {
		transfer := simulateChannelTransfer(channels[0], buf, totalBytes/int64(len(buf)))
		atomic.AddInt64(&transferred, transfer)
	}

	b.StopTimer()
	elapsed := b.Elapsed().Seconds()
	b.ReportMetric(toMB(transferred)/elapsed, "MB/s_single")
	b.ReportMetric(float64(transferred/int64(largeBufferSize))/elapsed, "IOPS_single")
	b.ReportMetric(elapsed*1000/float64(b.N), "ms_single")
}

// BenchmarkMultiChannel_TwoChannels 2通道基准测试.
func BenchmarkMultiChannel_TwoChannels(b *testing.B) {
	channels := newMockChannels(2)
	totalBytes := int64(largeBufferSize * 100)
	var transferred int64

	buf := make([]byte, largeBufferSize)
	rand.Read(buf)

	b.ResetTimer()
	b.SetBytes(largeBufferSize * 2)

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for _, ch := range channels {
			wg.Add(1)
			go func(c *mockChannel) {
				defer wg.Done()
				transfer := simulateChannelTransfer(c, buf, totalBytes/int64(len(buf))/int64(len(channels)))
				atomic.AddInt64(&transferred, transfer)
			}(ch)
		}
		wg.Wait()
	}

	b.StopTimer()
	elapsed := b.Elapsed().Seconds()
	b.ReportMetric(toMB(transferred)/elapsed, "MB/s_2ch")
	b.ReportMetric(float64(transferred/int64(largeBufferSize))/elapsed, "IOPS_2ch")
}

// BenchmarkMultiChannel_FourChannels 4通道基准测试.
func BenchmarkMultiChannel_FourChannels(b *testing.B) {
	channels := newMockChannels(4)
	totalBytes := int64(largeBufferSize * 100)
	var transferred int64

	buf := make([]byte, largeBufferSize)
	rand.Read(buf)

	b.ResetTimer()
	b.SetBytes(largeBufferSize * 4)

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for _, ch := range channels {
			wg.Add(1)
			go func(c *mockChannel) {
				defer wg.Done()
				transfer := simulateChannelTransfer(c, buf, totalBytes/int64(len(buf))/int64(len(channels)))
				atomic.AddInt64(&transferred, transfer)
			}(ch)
		}
		wg.Wait()
	}

	b.StopTimer()
	elapsed := b.Elapsed().Seconds()
	b.ReportMetric(toMB(transferred)/elapsed, "MB/s_4ch")
	b.ReportMetric(float64(transferred/int64(largeBufferSize))/elapsed, "IOPS_4ch")
}

// BenchmarkMultiChannel_SmallFile 小文件(64KB)多通道测试.
func BenchmarkMultiChannel_SmallFile(b *testing.B) {
	channels := newMockChannels(4)
	var transferred int64

	buf := make([]byte, smallBufferSize)
	rand.Read(buf)

	b.ResetTimer()
	b.SetBytes(smallBufferSize)

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for _, ch := range channels {
			wg.Add(1)
			go func(c *mockChannel) {
				defer wg.Done()
				transfer := simulateChannelTransfer(c, buf, 1)
				atomic.AddInt64(&transferred, transfer)
			}(ch)
		}
		wg.Wait()
	}

	b.StopTimer()
	elapsed := b.Elapsed().Seconds()
	b.ReportMetric(toMB(transferred)/elapsed, "MB/s_small")
	b.ReportMetric(elapsed*1000/float64(b.N*4), "ms_latency_small")
}

// BenchmarkMultiChannel_LargeFile 大文件(1MB)多通道测试.
func BenchmarkMultiChannel_LargeFile(b *testing.B) {
	channels := newMockChannels(4)
	totalBytes := int64(largeBufferSize * 100)
	var transferred int64

	buf := make([]byte, largeBufferSize)
	rand.Read(buf)

	b.ResetTimer()
	b.SetBytes(largeBufferSize)

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for _, ch := range channels {
			wg.Add(1)
			go func(c *mockChannel) {
				defer wg.Done()
				transfer := simulateChannelTransfer(c, buf, totalBytes/int64(len(buf))/int64(len(channels)))
				atomic.AddInt64(&transferred, transfer)
			}(ch)
		}
		wg.Wait()
	}

	b.StopTimer()
	elapsed := b.Elapsed().Seconds()
	b.ReportMetric(toMB(transferred)/elapsed, "MB/s_large")
	b.ReportMetric(elapsed*1000/float64(b.N), "ms_latency_large")
}

// BenchmarkMultiChannel_ConcurrentWrites 并发写入测试.
func BenchmarkMultiChannel_ConcurrentWrites(b *testing.B) {
	channels := newMockChannels(4)
	numWorkers := 8
	var transferred int64

	buf := make([]byte, mediumBufferSize)
	rand.Read(buf)

	b.ResetTimer()
	b.SetBytes(mediumBufferSize * int64(numWorkers))

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				ch := channels[workerID%len(channels)]
				transfer := simulateChannelTransfer(ch, buf, 10)
				atomic.AddInt64(&transferred, transfer)
			}(w)
		}
		wg.Wait()
	}

	b.StopTimer()
	elapsed := b.Elapsed().Seconds()
	b.ReportMetric(toMB(transferred)/elapsed, "MB/s_concurrent")
	b.ReportMetric(float64(transferred/int64(mediumBufferSize))/elapsed, "IOPS_concurrent")
}

// BenchmarkMultiChannel_ConcurrentReads 并发读取测试.
func BenchmarkMultiChannel_ConcurrentReads(b *testing.B) {
	channels := newMockChannels(4)
	numWorkers := 16
	var transferred int64

	buf := make([]byte, mediumBufferSize)
	rand.Read(buf)

	b.ResetTimer()
	b.SetBytes(mediumBufferSize * int64(numWorkers))

	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for w := 0; w < numWorkers; w++ {
			wg.Add(1)
			go func(workerID int) {
				defer wg.Done()
				ch := channels[workerID%len(channels)]
				transfer := simulateChannelRead(ch, buf, 10)
				atomic.AddInt64(&transferred, transfer)
			}(w)
		}
		wg.Wait()
	}

	b.StopTimer()
	elapsed := b.Elapsed().Seconds()
	b.ReportMetric(toMB(transferred)/elapsed, "MB/s_read_concurrent")
}

// BenchmarkMultiChannel_RoundRobin 轮询负载均衡测试.
func BenchmarkMultiChannel_RoundRobin(b *testing.B) {
	channels := newMockChannels(4)
	counter := int64(0)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		idx := atomic.AddInt64(&counter, 1) % int64(len(channels))
		_ = channels[idx]
	}

	b.StopTimer()
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "selections/sec")
}

// BenchmarkMultiChannel_HealthCheck 健康检查性能测试.
func BenchmarkMultiChannel_HealthCheck(b *testing.B) {
	channels := newMockChannels(4)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, ch := range channels {
			_ = checkChannelHealth(ch)
		}
	}

	b.StopTimer()
	b.ReportMetric(float64(b.N*len(channels))/b.Elapsed().Seconds(), "checks/sec")
}

// BenchmarkMultiChannel_ChannelSwitch 通道切换测试（故障恢复）.
func BenchmarkMultiChannel_ChannelSwitch(b *testing.B) {
	channels := newMockChannels(4)

	// 标记第一个通道为断开
	channels[0].connected = false

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		found := false
		for _, ch := range channels {
			if ch.connected {
				_ = ch
				found = true
				break
			}
		}
		if !found {
			channels[0].connected = true
		}
	}

	b.StopTimer()
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "switches/sec")
}

// simulateChannelTransfer 模拟通道传输.
func simulateChannelTransfer(ch *mockChannel, buf []byte, iterations int64) int64 {
	var transferred int64
	for i := int64(0); i < iterations; i++ {
		// 模拟网络延迟和带宽限制
		time.Sleep(time.Duration(ch.latencyMs) * time.Millisecond / 100)
		transferred += int64(len(buf))
	}
	return transferred
}

// simulateChannelRead 模拟通道读取.
func simulateChannelRead(ch *mockChannel, buf []byte, iterations int64) int64 {
	var transferred int64
	for i := int64(0); i < iterations; i++ {
		time.Sleep(time.Duration(ch.latencyMs) * time.Millisecond / 200)
		transferred += int64(len(buf))
	}
	return transferred
}

// checkChannelHealth 检查通道健康.
func checkChannelHealth(ch *mockChannel) int {
	if !ch.connected {
		return 0
	}
	score := 100
	if ch.latencyMs > 10 {
		score -= 20
	}
	if ch.bandwidthMbps < 100 {
		score -= 30
	}
	return score
}

// ==============================================================================
// 2. 网络接口发现基准测试
// ==============================================================================

// BenchmarkInterfaceDiscovery 接口发现性能测试.
func BenchmarkInterfaceDiscovery(b *testing.B) {
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ifaces, _ := net.Interfaces()
		_ = len(ifaces)
	}
	b.StopTimer()
	b.ReportMetric(float64(b.N)/b.Elapsed().Seconds(), "discoveries/sec")
}

// BenchmarkInterfaceInfoCollection 接口信息收集测试.
func BenchmarkInterfaceInfoCollection(b *testing.B) {
	ifaces, _ := net.Interfaces()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, iface := range ifaces {
			addrs, _ := iface.Addrs()
			_ = len(addrs)
			_ = iface.MTU
		}
	}
	b.StopTimer()
	b.ReportMetric(float64(b.N*len(ifaces))/b.Elapsed().Seconds(), "info_collections/sec")
}

// BenchmarkInterfaceSpeedDetection 接口速度检测测试.
func BenchmarkInterfaceSpeedDetection(b *testing.B) {
	ifaces, _ := net.Interfaces()
	if len(ifaces) == 0 {
		b.Skip("无可用网络接口")
	}

	ifaceName := ifaces[0].Name

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		speedPath := fmt.Sprintf("/sys/class/net/%s/speed", ifaceName)
		_, _ = os.ReadFile(speedPath)
	}
	b.StopTimer()
}

// ==============================================================================
// 3. MultichannelManager 基准测试
// ==============================================================================

// BenchmarkMultichannelManager_New 创建管理器测试.
func BenchmarkMultichannelManager_New(b *testing.B) {
	config := DefaultMultichannelConfig()
	config.Enabled = false

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr := NewMultichannelManager(config)
		_ = mgr
	}
}

// BenchmarkMultichannelManager_GetStatus 获取状态测试.
func BenchmarkMultichannelManager_GetStatus(b *testing.B) {
	config := DefaultMultichannelConfig()
	config.Enabled = false
	mgr := NewMultichannelManager(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.GetStatus()
	}
}

// BenchmarkMultichannelManager_GetMetrics 获取指标测试.
func BenchmarkMultichannelManager_GetMetrics(b *testing.B) {
	config := DefaultMultichannelConfig()
	config.Enabled = false
	mgr := NewMultichannelManager(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.GetMultichannelMetrics()
	}
}

// BenchmarkMultichannelManager_RoundRobinSelection 轮询选择测试.
func BenchmarkMultichannelManager_RoundRobinSelection(b *testing.B) {
	config := DefaultMultichannelConfig()
	config.Enabled = false
	config.RoundRobin = true
	mgr := NewMultichannelManager(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.GetRoundRobinInterface()
	}
}

// BenchmarkMultichannelManager_ChannelUpdate 通道更新测试.
func BenchmarkMultichannelManager_ChannelUpdate(b *testing.B) {
	config := DefaultMultichannelConfig()
	config.Enabled = false
	mgr := NewMultichannelManager(config)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.EnableChannel(1)
		_ = mgr.DisableChannel(1)
	}
}

// ==============================================================================
// 4. 配置生成基准测试
// ==============================================================================

// BenchmarkGenerateMultichannelConfig 生成多通道配置测试.
func BenchmarkGenerateMultichannelConfig(b *testing.B) {
	config := DefaultMultichannelConfig()
	config.Enabled = true

	interfaces := make([]*NetworkInterface, 4)
	for i := 0; i < 4; i++ {
		interfaces[i] = &NetworkInterface{
			Name:        fmt.Sprintf("eth%d", i),
			IPAddresses: []string{fmt.Sprintf("192.168.1.%d", 10+i)},
			SpeedMbps:   1000,
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = GenerateMultichannelConfig(&config, interfaces)
	}
}

// BenchmarkMultichannelConfigValidation 配置验证测试.
func BenchmarkMultichannelConfigValidation(b *testing.B) {
	config := DefaultMultichannelConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateMultichannelConfig(&config)
	}
}

// ==============================================================================
// 5. 端到端集成基准测试
// ==============================================================================

// BenchmarkEndToEnd_SingleToMultiChannel 单通道到多通道性能对比.
func BenchmarkEndToEnd_SingleToMultiChannel(b *testing.B) {
	// 单通道基准
	b.Run("Single", func(b *testing.B) {
		channels := newMockChannels(1)
		buf := make([]byte, largeBufferSize)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			_ = simulateChannelTransfer(channels[0], buf, 100)
		}
	})

	// 2通道
	b.Run("Dual", func(b *testing.B) {
		mc := newMockChannels(2)
		buf := make([]byte, largeBufferSize)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup
			for _, ch := range mc {
				wg.Add(1)
				go func(c *mockChannel) {
					defer wg.Done()
					_ = simulateChannelTransfer(c, buf, 50)
				}(ch)
			}
			wg.Wait()
		}
	})

	// 4通道
	b.Run("Quad", func(b *testing.B) {
		mc := newMockChannels(4)
		buf := make([]byte, largeBufferSize)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup
			for _, ch := range mc {
				wg.Add(1)
				go func(c *mockChannel) {
					defer wg.Done()
					_ = simulateChannelTransfer(c, buf, 25)
				}(ch)
			}
			wg.Wait()
		}
	})
}

// BenchmarkEndToEnd_RealisticFileTransfer 真实文件传输模拟.
func BenchmarkEndToEnd_RealisticFileTransfer(b *testing.B) {
	channels := newMockChannels(4)

	b.ResetTimer()
	b.SetBytes(64*1024 + 1024*1024 + 10*1024*1024) // 总文件大小

	for i := 0; i < b.N; i++ {
		sizes := []int64{64 * 1024, 1024 * 1024, 10 * 1024 * 1024}
		for _, size := range sizes {
			var wg sync.WaitGroup
			perChannel := size / int64(len(channels))
			for _, ch := range channels {
				wg.Add(1)
				go func(c *mockChannel, chunkSize int64) {
					defer wg.Done()
					iterations := chunkSize / int64(64*1024)
					for j := int64(0); j < iterations; j++ {
						time.Sleep(time.Duration(c.latencyMs) * time.Millisecond / 100)
					}
				}(ch, perChannel)
			}
			wg.Wait()
		}
	}
	b.StopTimer()
}

// ==============================================================================
// 6. 内存和并发压力测试
// ==============================================================================

// BenchmarkMemory_ChannelStateMemory 通道状态内存占用.
func BenchmarkMemory_ChannelStateMemory(b *testing.B) {
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		channels := make([]*SMBChannel, 4)
		for j := 0; j < 4; j++ {
			channels[j] = &SMBChannel{
				ID:            j + 1,
				InterfaceName: fmt.Sprintf("eth%d", j),
				IPAddress:     fmt.Sprintf("192.168.1.%d", 10+j),
				BandwidthMbps: 1000,
				Connected:     true,
				HealthScore:   100,
				ActiveSince:   time.Now(),
			}
		}
		_ = channels
	}
}

// BenchmarkConcurrency_ConnectionPool 连接池性能.
func BenchmarkConcurrency_ConnectionPool(b *testing.B) {
	channels := newMockChannels(4)
	pool := sync.Pool{
		New: func() interface{} {
			buf := make([]byte, mediumBufferSize)
			return &buf
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		var wg sync.WaitGroup
		for _, ch := range channels {
			wg.Add(1)
			go func(c *mockChannel) {
				defer wg.Done()
				bufPtr := pool.Get().(*[]byte)
				defer pool.Put(bufPtr)
				_ = simulateChannelTransfer(c, *bufPtr, 10)
			}(ch)
		}
		wg.Wait()
	}
	b.StopTimer()
}

// ==============================================================================
// 7. Buffer大小优化测试
// ==============================================================================

// BenchmarkBufferOptimization 缓冲区大小优化测试.
func BenchmarkBufferOptimization(b *testing.B) {
	b.Run("64KB", func(b *testing.B) {
		channels := newMockChannels(4)
		buf := make([]byte, 64*1024)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup
			for _, ch := range channels {
				wg.Add(1)
				go func(c *mockChannel) {
					defer wg.Done()
					_ = simulateChannelTransfer(c, buf, 10)
				}(ch)
			}
			wg.Wait()
		}
	})

	b.Run("256KB", func(b *testing.B) {
		channels := newMockChannels(4)
		buf := make([]byte, 256*1024)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup
			for _, ch := range channels {
				wg.Add(1)
				go func(c *mockChannel) {
					defer wg.Done()
					_ = simulateChannelTransfer(c, buf, 10)
				}(ch)
			}
			wg.Wait()
		}
	})

	b.Run("1MB", func(b *testing.B) {
		channels := newMockChannels(4)
		buf := make([]byte, 1024*1024)
		b.ResetTimer()
		for i := 0; i < b.N; i++ {
			var wg sync.WaitGroup
			for _, ch := range channels {
				wg.Add(1)
				go func(c *mockChannel) {
					defer wg.Done()
					_ = simulateChannelTransfer(c, buf, 10)
				}(ch)
			}
			wg.Wait()
		}
	})
}

// ==============================================================================
// 保留原有 Manager 操作基准测试
// ==============================================================================

// BenchmarkNewManager 创建Manager基准测试.
func BenchmarkNewManager(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := fmt.Sprintf("%s/smb.json", tmpDir)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewManager(configPath)
	}
}

// BenchmarkCreateShare 创建共享基准测试.
func BenchmarkCreateShare(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := fmt.Sprintf("%s/smb.json", tmpDir)
	mgr, _ := NewManager(configPath)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		share := &Share{
			Name: fmt.Sprintf("bench-share-%d", i),
			Path: fmt.Sprintf("%s/share-%d", tmpDir, i),
		}
		_ = mgr.CreateShare(share)
	}
}

// BenchmarkGetShare 获取共享基准测试.
func BenchmarkGetShare(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := fmt.Sprintf("%s/smb.json", tmpDir)
	mgr, _ := NewManager(configPath)

	sharePath := fmt.Sprintf("%s/bench-share", tmpDir)
	_ = mgr.CreateShare(&Share{
		Name: "bench-share",
		Path: sharePath,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.GetShare("bench-share")
	}
}

// BenchmarkListShares 列出共享基准测试.
func BenchmarkListShares(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := fmt.Sprintf("%s/smb.json", tmpDir)
	mgr, _ := NewManager(configPath)

	for i := 0; i < 100; i++ {
		_ = mgr.CreateShare(&Share{
			Name: fmt.Sprintf("share-%d", i),
			Path: fmt.Sprintf("%s/share-%d", tmpDir, i),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.ListShares()
	}
}

// BenchmarkUpdateShare 更新共享基准测试.
func BenchmarkUpdateShare(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := fmt.Sprintf("%s/smb.json", tmpDir)
	mgr, _ := NewManager(configPath)

	sharePath := fmt.Sprintf("%s/bench-update", tmpDir)
	_ = mgr.CreateShare(&Share{
		Name: "bench-update",
		Path: sharePath,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.UpdateShare("bench-update", &Share{
			Comment: fmt.Sprintf("Updated %d", i),
		})
	}
}

// BenchmarkSaveConfig 保存配置基准测试.
func BenchmarkSaveConfig(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := fmt.Sprintf("%s/smb.json", tmpDir)
	mgr, _ := NewManager(configPath)

	for i := 0; i < 50; i++ {
		_ = mgr.CreateShare(&Share{
			Name: fmt.Sprintf("save-share-%d", i),
			Path: fmt.Sprintf("%s/share-%d", tmpDir, i),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.saveConfig()
	}
}

// BenchmarkConcurrentRead 并发读取基准测试.
func BenchmarkConcurrentRead(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := fmt.Sprintf("%s/smb.json", tmpDir)
	mgr, _ := NewManager(configPath)

	for i := 0; i < 100; i++ {
		_ = mgr.CreateShare(&Share{
			Name: fmt.Sprintf("concurrent-%d", i),
			Path: fmt.Sprintf("%s/share-%d", tmpDir, i),
		})
	}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _ = mgr.GetShare(fmt.Sprintf("concurrent-%d", i%100))
			i++
		}
	})
}

// BenchmarkConcurrentWrite 并发写入基准测试.
func BenchmarkConcurrentWrite(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := fmt.Sprintf("%s/smb.json", tmpDir)
	mgr, _ := NewManager(configPath)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			name := fmt.Sprintf("par-write-%d", i)
			_ = mgr.CreateShare(&Share{
				Name: name,
				Path: fmt.Sprintf("%s/%s", tmpDir, name),
			})
			_ = mgr.DeleteShare(name)
			i++
		}
	})
}

// BenchmarkValidateShareConfig 验证共享配置基准测试.
func BenchmarkValidateShareConfig(b *testing.B) {
	share := &Share{
		Name:          "bench-validate",
		Path:          "/data/validate",
		CreateMask:    "0644",
		DirectoryMask: "0755",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateShareConfig(share)
	}
}

// BenchmarkValidateConfig 验证配置基准测试.
func BenchmarkValidateConfig(b *testing.B) {
	config := &Config{
		Workgroup:    "WORKGROUP",
		ServerString: "Test Server",
		MinProtocol:  "SMB2",
		MaxProtocol:  "SMB3",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateConfig(config)
	}
}

// BenchmarkListShares_Alloc 列出共享内存分配基准测试.
func BenchmarkListShares_Alloc(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := fmt.Sprintf("%s/smb.json", tmpDir)
	mgr, _ := NewManager(configPath)

	for i := 0; i < 100; i++ {
		_ = mgr.CreateShare(&Share{
			Name: fmt.Sprintf("alloc-%d", i),
			Path: fmt.Sprintf("%s/share-%d", tmpDir, i),
		})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		shares, _ := mgr.ListShares()
		_ = shares
	}
}
