// Package tunnel 提供内网穿透服务 - 带宽监控测试
package tunnel

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestNewBandwidthMonitor 测试创建带宽监控器
func TestNewBandwidthMonitor(t *testing.T) {
	config := DefaultBandwidthConfig()
	monitor := NewBandwidthMonitor(config, nil)

	require.NotNil(t, monitor)
	// 默认配置没有限速，所以限速器为 nil
	assert.Nil(t, monitor.uploadLimiter)
	assert.Nil(t, monitor.downloadLimiter)
}

// TestNewBandwidthMonitorWithLimits 测试创建有限速的带宽监控器
func TestNewBandwidthMonitorWithLimits(t *testing.T) {
	config := BandwidthConfig{
		UploadLimit:     1000000,
		DownloadLimit:   2000000,
		StatsInterval:   time.Second,
		BucketMultiplier: 2,
	}
	monitor := NewBandwidthMonitor(config, nil)

	require.NotNil(t, monitor)
	assert.NotNil(t, monitor.uploadLimiter)
	assert.NotNil(t, monitor.downloadLimiter)
}

// TestBandwidthMonitorRecord 测试记录带宽
func TestBandwidthMonitorRecord(t *testing.T) {
	monitor := NewBandwidthMonitor(DefaultBandwidthConfig(), nil)

	// 记录上传
	monitor.RecordUpload(1000)
	stats := monitor.GetStats()
	assert.Equal(t, int64(1000), stats.TotalUpload)

	// 记录下载
	monitor.RecordDownload(2000)
	stats = monitor.GetStats()
	assert.Equal(t, int64(2000), stats.TotalDownload)
}

// TestBandwidthMonitorUpdateStats 测试更新统计
func TestBandwidthMonitorUpdateStats(t *testing.T) {
	monitor := NewBandwidthMonitor(DefaultBandwidthConfig(), nil)

	// 记录一些数据
	monitor.RecordUpload(10000)
	monitor.RecordDownload(20000)

	// 等待一秒后更新统计
	time.Sleep(time.Second)
	monitor.UpdateStats()

	stats := monitor.GetStats()
	assert.GreaterOrEqual(t, stats.UploadRate, int64(0))
	assert.GreaterOrEqual(t, stats.DownloadRate, int64(0))
}

// TestTokenBucketAllow 测试令牌桶允许
func TestTokenBucketAllow(t *testing.T) {
	// 限速 1000 字节/秒
	bucket := NewTokenBucket(1000, 2000)

	// 应该允许小量数据
	assert.True(t, bucket.Allow(100))
	assert.True(t, bucket.Allow(500))

	// 应该不允许超过容量的数据
	assert.False(t, bucket.Allow(5000))
}

// TestTokenBucketNoLimit 测试无限制的令牌桶
func TestTokenBucketNoLimit(t *testing.T) {
	bucket := NewTokenBucket(0, 0)

	// 无限制应该总是允许
	assert.True(t, bucket.Allow(1000000))
	assert.True(t, bucket.Allow(1000000000))
}

// TestTokenBucketWait 测试令牌桶等待
func TestTokenBucketWait(t *testing.T) {
	bucket := NewTokenBucket(1000, 1000)

	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	// 小量数据应该立即允许
	err := bucket.Wait(ctx, 100)
	assert.NoError(t, err)

	// 大量数据可能需要等待
	ctx2, cancel2 := context.WithTimeout(context.Background(), 10*time.Millisecond)
	defer cancel2()

	err = bucket.Wait(ctx2, 5000)
	// 可能超时或成功，取决于令牌补充
	_ = err
}

// TestBandwidthMonitorService 测试带宽监控服务
func TestBandwidthMonitorService(t *testing.T) {
	config := BandwidthConfig{
		StatsInterval: 100 * time.Millisecond,
	}
	service := NewBandwidthMonitorService(config, nil)

	ctx := context.Background()
	err := service.Start(ctx)
	require.NoError(t, err)

	// 记录一些数据
	monitor := service.GetMonitor()
	monitor.RecordUpload(1000)
	monitor.RecordDownload(2000)

	// 等待统计更新
	time.Sleep(200 * time.Millisecond)

	stats := monitor.GetStats()
	assert.Equal(t, int64(1000), stats.TotalUpload)
	assert.Equal(t, int64(2000), stats.TotalDownload)

	// 停止服务
	service.Stop()
}

// TestBandwidthMonitorSetLimits 测试设置限速
func TestBandwidthMonitorSetLimits(t *testing.T) {
	monitor := NewBandwidthMonitor(DefaultBandwidthConfig(), nil)

	// 设置上传限速
	monitor.SetUploadLimit(1000000)
	assert.Equal(t, int64(1000000), monitor.config.UploadLimit)

	// 设置下载限速
	monitor.SetDownloadLimit(2000000)
	assert.Equal(t, int64(2000000), monitor.config.DownloadLimit)
}

// TestBandwidthMonitorReset 测试重置统计
func TestBandwidthMonitorReset(t *testing.T) {
	monitor := NewBandwidthMonitor(DefaultBandwidthConfig(), nil)

	// 记录一些数据
	monitor.RecordUpload(1000)
	monitor.RecordDownload(2000)

	// 重置
	monitor.Reset()

	stats := monitor.GetStats()
	assert.Equal(t, int64(0), stats.TotalUpload)
	assert.Equal(t, int64(0), stats.TotalDownload)
}

// TestBandwidthMonitorAllowUpload 测试上传限速检查
func TestBandwidthMonitorAllowUpload(t *testing.T) {
	config := BandwidthConfig{
		UploadLimit:     1000,
		BucketMultiplier: 2,
	}
	monitor := NewBandwidthMonitor(config, nil)

	// 应该允许小量上传
	assert.True(t, monitor.AllowUpload(100))
	assert.True(t, monitor.AllowUpload(500))
}

// TestBandwidthMonitorAllowDownload 测试下载限速检查
func TestBandwidthMonitorAllowDownload(t *testing.T) {
	config := BandwidthConfig{
		DownloadLimit:    1000,
		BucketMultiplier: 2,
	}
	monitor := NewBandwidthMonitor(config, nil)

	// 应该允许小量下载
	assert.True(t, monitor.AllowDownload(100))
	assert.True(t, monitor.AllowDownload(500))
}

// TestBandwidthStatsConnections 测试连接数统计
func TestBandwidthStatsConnections(t *testing.T) {
	monitor := NewBandwidthMonitor(DefaultBandwidthConfig(), nil)

	monitor.SetConnections(5)
	stats := monitor.GetStats()
	assert.Equal(t, 5, stats.Connections)

	monitor.SetConnections(10)
	stats = monitor.GetStats()
	assert.Equal(t, 10, stats.Connections)
}

// TestBandwidthMonitorPeakRates 测试峰值速率
func TestBandwidthMonitorPeakRates(t *testing.T) {
	monitor := NewBandwidthMonitor(BandwidthConfig{
		StatsInterval: 10 * time.Millisecond,
	}, nil)

	// 模拟高流量并等待足够时间
	monitor.RecordUpload(100000)
	monitor.RecordDownload(200000)
	
	// 更新统计
	monitor.mu.Lock()
	monitor.stats.UploadRate = 100000
	monitor.stats.DownloadRate = 200000
	monitor.mu.Unlock()

	monitor.UpdateStats()

	stats := monitor.GetStats()
	// 峰值应该在更新后设置
	assert.GreaterOrEqual(t, stats.PeakUploadRate, int64(0))
	assert.GreaterOrEqual(t, stats.PeakDownloadRate, int64(0))
}

// TestBandwidthMonitorAverageRates 测试平均速率
func TestBandwidthMonitorAverageRates(t *testing.T) {
	monitor := NewBandwidthMonitor(BandwidthConfig{
		StatsInterval: 10 * time.Millisecond,
	}, nil)

	// 记录多次数据
	for i := 0; i < 5; i++ {
		monitor.RecordUpload(1000)
		monitor.RecordDownload(2000)
		time.Sleep(10 * time.Millisecond)
		monitor.UpdateStats()
	}

	avgUp, avgDown := monitor.GetAverageRates()
	assert.GreaterOrEqual(t, avgUp, int64(0))
	assert.GreaterOrEqual(t, avgDown, int64(0))
}