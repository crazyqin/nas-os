package sysresmon

import (
	"context"
	"testing"
	"time"
)

func TestRingBuffer_PushAndLatest(t *testing.T) {
	rb := NewRingBuffer(10)

	// 测试空缓冲区
	if latest := rb.Latest(); latest != nil {
		t.Error("expected nil for empty buffer")
	}

	// 推入数据
	snap := ResourceSnapshot{
		Timestamp: time.Now(),
		CPU: CPUInfo{
			UsagePercent: 50.0,
			Cores:        4,
		},
	}
	rb.Push(snap)

	latest := rb.Latest()
	if latest == nil {
		t.Fatal("expected snapshot, got nil")
	}
	if latest.CPU.UsagePercent != 50.0 {
		t.Errorf("expected 50.0, got %f", latest.CPU.UsagePercent)
	}
}

func TestRingBuffer_CircularOverwrite(t *testing.T) {
	rb := NewRingBuffer(3)

	// 推入 4 条数据，应该覆盖最旧的
	for i := 0; i < 4; i++ {
		snap := ResourceSnapshot{
			Timestamp: time.Now().Add(time.Duration(i) * time.Second),
			CPU:       CPUInfo{UsagePercent: float64(i * 10)},
		}
		rb.Push(snap)
	}

	if rb.Count() != 3 {
		t.Errorf("expected count 3, got %d", rb.Count())
	}

	// 验证最后 3 条数据
	history := rb.LastN(3)
	if len(history) != 3 {
		t.Fatal("expected 3 items")
	}

	// 第一条应该是 10（0 被覆盖了）
	if history[0].CPU.UsagePercent != 10.0 {
		t.Errorf("expected 10.0, got %f", history[0].CPU.UsagePercent)
	}

	// 最后一条应该是 30
	if history[2].CPU.UsagePercent != 30.0 {
		t.Errorf("expected 30.0, got %f", history[2].CPU.UsagePercent)
	}
}

func TestRingBuffer_Since(t *testing.T) {
	rb := NewRingBuffer(10)

	base := time.Now()

	// 推入 5 条数据，时间间隔 1 分钟
	for i := 0; i < 5; i++ {
		snap := ResourceSnapshot{
			Timestamp: base.Add(time.Duration(-5+i) * time.Minute),
		}
		rb.Push(snap)
	}

	// 查询最近 3 分钟
	since := base.Add(-3 * time.Minute)
	result := rb.Since(since)

	if len(result) != 3 {
		t.Errorf("expected 3 items, got %d", len(result))
	}
}

func TestDashboard_AnalyzeBottleneck(t *testing.T) {
	d := &Dashboard{}
	history := make([]ResourceSnapshot, 20)

	// 模拟 CPU 高负载
	for i := range history {
		history[i] = ResourceSnapshot{
			Timestamp: time.Now(),
			CPU: CPUInfo{
				UsagePercent: 85.0,
			},
			Memory: MemoryInfo{
				UsagePercent: 50.0,
			},
		}
	}

	analysis := d.analyzeBottleneck(history)

	if analysis.Type != "cpu" {
		t.Errorf("expected bottleneck type 'cpu', got '%s'", analysis.Type)
	}
	if analysis.Severity < 80 {
		t.Errorf("expected severity >= 80, got %d", analysis.Severity)
	}
}

func TestDashboard_AnalyzeBottleneck_Memory(t *testing.T) {
	d := &Dashboard{}
	history := make([]ResourceSnapshot, 20)

	// 模拟内存高负载
	for i := range history {
		history[i] = ResourceSnapshot{
			Timestamp: time.Now(),
			CPU: CPUInfo{
				UsagePercent: 30.0,
			},
			Memory: MemoryInfo{
				UsagePercent: 90.0,
			},
		}
	}

	analysis := d.analyzeBottleneck(history)

	if analysis.Type != "memory" {
		t.Errorf("expected bottleneck type 'memory', got '%s'", analysis.Type)
	}
}

func TestDashboard_AnalyzeBottleneck_None(t *testing.T) {
	d := &Dashboard{}
	history := make([]ResourceSnapshot, 20)

	// 模拟正常负载
	for i := range history {
		history[i] = ResourceSnapshot{
			Timestamp: time.Now(),
			CPU: CPUInfo{
				UsagePercent: 30.0,
			},
			Memory: MemoryInfo{
				UsagePercent: 40.0,
			},
			DiskIO: DiskIOInfo{
				ReadBytesPerSec:  1000000,
				WriteBytesPerSec: 1000000,
			},
			Network: NetworkInfo{
				UploadPerSec:   100000,
				DownloadPerSec: 100000,
			},
		}
	}

	analysis := d.analyzeBottleneck(history)

	if analysis.Type != "none" {
		t.Errorf("expected bottleneck type 'none', got '%s'", analysis.Type)
	}
	if analysis.Severity != 0 {
		t.Errorf("expected severity 0, got %d", analysis.Severity)
	}
}

func TestCalcMean(t *testing.T) {
	tests := []struct {
		data     []float64
		expected float64
	}{
		{[]float64{1, 2, 3, 4, 5}, 3.0},
		{[]float64{10, 20, 30}, 20.0},
		{[]float64{}, 0},
		{[]float64{5}, 5.0},
	}

	for _, test := range tests {
		result := calcMean(test.data)
		if result != test.expected {
			t.Errorf("calcMean(%v) = %f, expected %f", test.data, result, test.expected)
		}
	}
}

func TestCalcMax(t *testing.T) {
	tests := []struct {
		data     []float64
		expected float64
	}{
		{[]float64{1, 5, 3, 2, 4}, 5.0},
		{[]float64{-1, -5, -3}, -1.0},
		{[]float64{}, 0},
	}

	for _, test := range tests {
		result := calcMax(test.data)
		if result != test.expected {
			t.Errorf("calcMax(%v) = %f, expected %f", test.data, result, test.expected)
		}
	}
}

func TestCalcMin(t *testing.T) {
	tests := []struct {
		data     []float64
		expected float64
	}{
		{[]float64{1, 5, 3, 2, 4}, 1.0},
		{[]float64{-1, -5, -3}, -5.0},
		{[]float64{}, 0},
	}

	for _, test := range tests {
		result := calcMin(test.data)
		if result != test.expected {
			t.Errorf("calcMin(%v) = %f, expected %f", test.data, result, test.expected)
		}
	}
}

func TestCalcStdDev(t *testing.T) {
	data := []float64{2, 4, 4, 4, 5, 5, 7, 9}
	stddev := calcStdDev(data)

	// 标准差约为 2.0
	if stddev < 1.9 || stddev > 2.1 {
		t.Errorf("expected stddev around 2.0, got %f", stddev)
	}
}

func TestCalcMovingAverage(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	ma := calcMovingAverage(data, 3)

	// 应该有 8 个点
	if len(ma) != 8 {
		t.Fatalf("expected 8 points, got %d", len(ma))
	}

	// 第一个点应该是 (1+2+3)/3 = 2
	if ma[0] != 2.0 {
		t.Errorf("expected 2.0, got %f", ma[0])
	}

	// 最后一个点应该是 (8+9+10)/3 = 9
	if ma[7] != 9.0 {
		t.Errorf("expected 9.0, got %f", ma[7])
	}
}

func TestCalcSlope(t *testing.T) {
	// 测试上升趋势
	up := []float64{1, 2, 3, 4, 5}
	slope := calcSlope(up)
	if slope <= 0 {
		t.Errorf("expected positive slope, got %f", slope)
	}

	// 测试下降趋势
	down := []float64{5, 4, 3, 2, 1}
	slope = calcSlope(down)
	if slope >= 0 {
		t.Errorf("expected negative slope, got %f", slope)
	}

	// 测试平稳
	stable := []float64{5, 5, 5, 5, 5}
	slope = calcSlope(stable)
	if slope != 0 {
		t.Errorf("expected zero slope, got %f", slope)
	}
}

func TestPercentile(t *testing.T) {
	data := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	// 50th percentile (median)
	p50 := percentile(data, 50)
	if p50 < 5.0 || p50 > 6.0 {
		t.Errorf("expected p50 around 5.5, got %f", p50)
	}

	// 90th percentile
	p90 := percentile(data, 90)
	if p90 < 9.0 || p90 > 10.0 {
		t.Errorf("expected p90 around 9.1, got %f", p90)
	}

	// 0th percentile
	p0 := percentile(data, 0)
	if p0 != 1.0 {
		t.Errorf("expected p0 = 1.0, got %f", p0)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Interval != 30 {
		t.Errorf("expected interval 30, got %d", cfg.Interval)
	}
	if cfg.BufferSize != 2880 {
		t.Errorf("expected buffer size 2880, got %d", cfg.BufferSize)
	}
}

func TestNewResourceMonitor(t *testing.T) {
	// 测试默认配置
	rm := NewResourceMonitor(nil)
	if rm == nil {
		t.Fatal("expected monitor, got nil")
	}

	cfg := rm.GetConfig()
	if cfg.Interval != 30 {
		t.Errorf("expected default interval 30, got %d", cfg.Interval)
	}

	// 测试自定义配置
	customCfg := &Config{Interval: 10, BufferSize: 100}
	rm = NewResourceMonitor(customCfg)
	cfg = rm.GetConfig()
	if cfg.Interval != 10 {
		t.Errorf("expected interval 10, got %d", cfg.Interval)
	}
}

func TestResourceMonitor_StartStop(t *testing.T) {
	rm := NewResourceMonitor(&Config{
		Interval:   1, // 1 秒间隔便于测试
		BufferSize: 100,
	})

	ctx := context.Background()

	// 启动监控
	err := rm.Start(ctx)
	if err != nil {
		t.Fatalf("failed to start: %v", err)
	}

	// 等待采集
	time.Sleep(1500 * time.Millisecond)

	// 检查是否有数据
	latest := rm.GetLatest()
	if latest == nil {
		t.Error("expected snapshot, got nil")
	}

	// 停止监控
	rm.Stop()
}

func TestDashboard_GetDashboardData(t *testing.T) {
	rm := NewResourceMonitor(&Config{
		Interval:   1,
		BufferSize: 100,
	})

	ctx := context.Background()
	rm.Start(ctx)
	defer rm.Stop()

	// 等待一些数据
	time.Sleep(2 * time.Second)

	dashboard := NewDashboard(rm)

	// 测试各时间范围
	for _, r := range []TimeRange{Range1H, Range6H, Range24H, Range7D} {
		data := dashboard.GetDashboardData(r)
		if data == nil {
			t.Errorf("expected data for range %s, got nil", r)
		}
		if data.Current == nil {
			t.Errorf("expected current snapshot for range %s", r)
		}
	}
}

func TestDashboard_GetTrendData(t *testing.T) {
	rm := NewResourceMonitor(&Config{
		Interval:   1,
		BufferSize: 100,
	})

	ctx := context.Background()
	rm.Start(ctx)
	defer rm.Stop()

	time.Sleep(3 * time.Second)

	dashboard := NewDashboard(rm)
	trends := dashboard.GetTrendData(Range1H)

	if trends == nil {
		t.Fatal("expected trends, got nil")
	}

	// 应该有一些数据点
	if len(trends) == 0 {
		t.Error("expected trend data points")
	}

	// 检查数据点结构
	point := trends[0]
	if point.Timestamp.IsZero() {
		t.Error("expected non-zero timestamp")
	}
}

func TestTimeRange_ParseRange(t *testing.T) {
	d := &Dashboard{}

	tests := []struct {
		input    TimeRange
		expected time.Duration
	}{
		{Range1H, 1 * time.Hour},
		{Range6H, 6 * time.Hour},
		{Range24H, 24 * time.Hour},
		{Range7D, 7 * 24 * time.Hour},
		{"invalid", 1 * time.Hour},
	}

	for _, test := range tests {
		result := d.parseRange(test.input)
		if result != test.expected {
			t.Errorf("parseRange(%s) = %v, expected %v", test.input, result, test.expected)
		}
	}
}

func BenchmarkRingBuffer_Push(b *testing.B) {
	rb := NewRingBuffer(1000)
	snap := ResourceSnapshot{
		Timestamp: time.Now(),
		CPU:       CPUInfo{UsagePercent: 50},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rb.Push(snap)
	}
}

func BenchmarkRingBuffer_Since(b *testing.B) {
	rb := NewRingBuffer(2880)

	// 填充数据
	base := time.Now()
	for i := 0; i < 2880; i++ {
		rb.Push(ResourceSnapshot{
			Timestamp: base.Add(time.Duration(-i) * 30 * time.Second),
		})
	}

	since := base.Add(-1 * time.Hour)
	b.ResetTimer()

	for i := 0; i < b.N; i++ {
		rb.Since(since)
	}
}

func BenchmarkCalcMean(b *testing.B) {
	data := make([]float64, 1000)
	for i := range data {
		data[i] = float64(i)
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		calcMean(data)
	}
}
