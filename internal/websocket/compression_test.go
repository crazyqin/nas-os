// Package websocket 提供消息压缩测试
package websocket

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestCompressionAlgorithm_String(t *testing.T) {
	tests := []struct {
		algorithm CompressionAlgorithm
		expected  string
	}{
		{CompressionNone, "none"},
		{CompressionGzip, "gzip"},
		{CompressionZlib, "zlib"},
		{CompressionFlate, "flate"},
		{CompressionSnappy, "snappy"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			if got := tt.algorithm.String(); got != tt.expected {
				t.Errorf("CompressionAlgorithm.String() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestGzipCompressor(t *testing.T) {
	compressor := NewGzipCompressor(6)

	testData := []byte(strings.Repeat("Hello, World! ", 100))

	// 测试压缩
	compressed, err := compressor.Compress(testData)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	if len(compressed) >= len(testData) {
		t.Errorf("Compressed data should be smaller than original, got %d >= %d", len(compressed), len(testData))
	}

	// 测试解压
	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if string(decompressed) != string(testData) {
		t.Errorf("Decompressed data mismatch")
	}

	// 测试算法
	if compressor.Algorithm() != CompressionGzip {
		t.Errorf("Algorithm should be gzip, got %v", compressor.Algorithm())
	}
}

func TestZlibCompressor(t *testing.T) {
	compressor := NewZlibCompressor(6, nil)

	testData := []byte(strings.Repeat("Hello, World! ", 100))

	// 测试压缩
	compressed, err := compressor.Compress(testData)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	if len(compressed) >= len(testData) {
		t.Errorf("Compressed data should be smaller than original, got %d >= %d", len(compressed), len(testData))
	}

	// 测试解压
	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if string(decompressed) != string(testData) {
		t.Errorf("Decompressed data mismatch")
	}

	// 测试算法
	if compressor.Algorithm() != CompressionZlib {
		t.Errorf("Algorithm should be zlib, got %v", compressor.Algorithm())
	}
}

func TestZlibCompressor_WithDict(t *testing.T) {
	dict := []byte("common prefix data that appears frequently")
	compressor := NewZlibCompressor(6, dict)

	testData := []byte("common prefix data that appears frequently and more content")

	// 测试压缩
	compressed, err := compressor.Compress(testData)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	// 测试解压
	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if string(decompressed) != string(testData) {
		t.Errorf("Decompressed data mismatch")
	}
}

func TestFlateCompressor(t *testing.T) {
	compressor := NewFlateCompressor(6, nil)

	testData := []byte(strings.Repeat("Hello, World! ", 100))

	// 测试压缩
	compressed, err := compressor.Compress(testData)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	if len(compressed) >= len(testData) {
		t.Errorf("Compressed data should be smaller than original, got %d >= %d", len(compressed), len(testData))
	}

	// 测试解压
	decompressed, err := compressor.Decompress(compressed)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if string(decompressed) != string(testData) {
		t.Errorf("Decompressed data mismatch")
	}

	// 测试算法
	if compressor.Algorithm() != CompressionFlate {
		t.Errorf("Algorithm should be flate, got %v", compressor.Algorithm())
	}
}

func TestNoneCompressor(t *testing.T) {
	compressor := NewNoneCompressor()

	testData := []byte("Hello, World!")

	// 测试压缩（应该返回原数据）
	compressed, err := compressor.Compress(testData)
	if err != nil {
		t.Fatalf("Compress failed: %v", err)
	}

	if string(compressed) != string(testData) {
		t.Errorf("NoneCompressor should return original data")
	}

	// 测试解压（应该返回原数据）
	decompressed, err := compressor.Decompress(testData)
	if err != nil {
		t.Fatalf("Decompress failed: %v", err)
	}

	if string(decompressed) != string(testData) {
		t.Errorf("NoneCompressor should return original data")
	}

	// 测试算法
	if compressor.Algorithm() != CompressionNone {
		t.Errorf("Algorithm should be none, got %v", compressor.Algorithm())
	}
}

func TestMessageCompressor_CompressMessage(t *testing.T) {
	config := &CompressionConfig{
		Enabled:   true,
		Algorithm: CompressionGzip,
		Level:     6,
		MinSize:   50,
		MaxSize:   10 * 1024 * 1024,
		Threshold: 0.9,
		PoolSize:  10,
	}

	mc := NewMessageCompressor(config)

	// 创建大消息（可压缩）
	largeData := strings.Repeat("x", 1000)
	msg := &Message{
		ID:        "test-1",
		Type:      "test",
		Priority:  PriorityNormal,
		Data:      json.RawMessage(`"` + largeData + `"`),
		Timestamp: time.Now(),
	}

	// 测试压缩
	cm, err := mc.CompressMessage(msg)
	if err != nil {
		t.Fatalf("CompressMessage failed: %v", err)
	}

	if cm.OriginalType != msg.Type {
		t.Errorf("OriginalType mismatch: got %v, want %v", cm.OriginalType, msg.Type)
	}

	if cm.Algorithm != CompressionGzip {
		t.Errorf("Algorithm mismatch: got %v, want %v", cm.Algorithm, CompressionGzip)
	}

	if cm.CompressedSize >= cm.OriginalSize {
		t.Errorf("CompressedSize should be less than OriginalSize")
	}

	// 测试解压
	decompressed, err := mc.DecompressMessage(cm)
	if err != nil {
		t.Fatalf("DecompressMessage failed: %v", err)
	}

	if decompressed.ID != msg.ID {
		t.Errorf("Decompressed message ID mismatch: got %v, want %v", decompressed.ID, msg.ID)
	}
}

func TestMessageCompressor_CompressMessage_TooSmall(t *testing.T) {
	config := &CompressionConfig{
		Enabled:   true,
		Algorithm: CompressionGzip,
		Level:     6,
		MinSize:   1000, // 较大的最小值
		Threshold: 0.9,
	}

	mc := NewMessageCompressor(config)

	// 创建小消息
	msg := &Message{
		ID:        "test-1",
		Type:      "test",
		Priority:  PriorityNormal,
		Data:      json.RawMessage(`"small"`),
		Timestamp: time.Now(),
	}

	// 应该返回 ErrMessageTooSmall
	_, err := mc.CompressMessage(msg)
	if err != ErrMessageTooSmall {
		t.Errorf("Expected ErrMessageTooSmall, got %v", err)
	}
}

func TestMessageCompressor_CompressMessage_TooLarge(t *testing.T) {
	config := &CompressionConfig{
		Enabled:   true,
		Algorithm: CompressionGzip,
		Level:     6,
		MinSize:   10,
		MaxSize:   100, // 很小的最大值
		Threshold: 0.9,
	}

	mc := NewMessageCompressor(config)

	// 创建大消息
	largeData := strings.Repeat("x", 200)
	msg := &Message{
		ID:        "test-1",
		Type:      "test",
		Priority:  PriorityNormal,
		Data:      json.RawMessage(`"` + largeData + `"`),
		Timestamp: time.Now(),
	}

	// 应该返回 ErrMessageTooLarge
	_, err := mc.CompressMessage(msg)
	if err != ErrMessageTooLarge {
		t.Errorf("Expected ErrMessageTooLarge, got %v", err)
	}
}

func TestMessageCompressor_CompressData(t *testing.T) {
	config := &CompressionConfig{
		Enabled:   true,
		Algorithm: CompressionGzip,
		Level:     6,
		MinSize:   50,
		MaxSize:   10 * 1024 * 1024,
		Threshold: 0.9,
	}

	mc := NewMessageCompressor(config)

	// 创建可压缩数据
	testData := []byte(strings.Repeat("Hello, World! ", 100))

	compressed, result, err := mc.CompressData(testData)
	if err != nil {
		t.Fatalf("CompressData failed: %v", err)
	}

	if result.OriginalSize != len(testData) {
		t.Errorf("OriginalSize mismatch: got %v, want %v", result.OriginalSize, len(testData))
	}

	if result.CompressedSize != len(compressed) {
		t.Errorf("CompressedSize mismatch: got %v, want %v", result.CompressedSize, len(compressed))
	}

	if result.Algorithm != CompressionGzip {
		t.Errorf("Algorithm mismatch: got %v, want %v", result.Algorithm, CompressionGzip)
	}

	if result.UsedFallback {
		t.Errorf("UsedFallback should be false for compressible data")
	}

	// 测试解压
	decompressed, err := mc.DecompressData(compressed, CompressionGzip)
	if err != nil {
		t.Fatalf("DecompressData failed: %v", err)
	}

	if string(decompressed) != string(testData) {
		t.Errorf("Decompressed data mismatch")
	}
}

func TestMessageCompressor_CompressData_TooSmall(t *testing.T) {
	config := &CompressionConfig{
		Enabled:   true,
		Algorithm: CompressionGzip,
		Level:     6,
		MinSize:   1000,
		Threshold: 0.9,
	}

	mc := NewMessageCompressor(config)

	// 创建小数据
	testData := []byte("small")

	compressed, result, err := mc.CompressData(testData)
	if err != nil {
		t.Fatalf("CompressData failed: %v", err)
	}

	if !result.UsedFallback {
		t.Errorf("UsedFallback should be true for small data")
	}

	if string(compressed) != string(testData) {
		t.Errorf("Should return original data for small input")
	}
}

func TestMessageCompressor_Disabled(t *testing.T) {
	config := &CompressionConfig{
		Enabled:   false, // 禁用压缩
		Algorithm: CompressionGzip,
		Level:     6,
		MinSize:   10,
		Threshold: 0.9,
	}

	mc := NewMessageCompressor(config)

	msg := &Message{
		ID:        "test-1",
		Type:      "test",
		Priority:  PriorityNormal,
		Data:      json.RawMessage(`"test data"`),
		Timestamp: time.Now(),
	}

	_, err := mc.CompressMessage(msg)
	if err != ErrCompressionDisabled {
		t.Errorf("Expected ErrCompressionDisabled, got %v", err)
	}
}

func TestMessageCompressor_SetAlgorithm(t *testing.T) {
	config := &CompressionConfig{
		Enabled:   true,
		Algorithm: CompressionGzip,
		Level:     6,
		MinSize:   10,
		Threshold: 0.9,
	}

	mc := NewMessageCompressor(config)

	// 设置新算法
	mc.SetAlgorithm(CompressionZlib)

	if mc.compressor.Algorithm() != CompressionZlib {
		t.Errorf("Algorithm should be zlib, got %v", mc.compressor.Algorithm())
	}
}

func TestMessageCompressor_SetLevel(t *testing.T) {
	config := &CompressionConfig{
		Enabled:   true,
		Algorithm: CompressionGzip,
		Level:     6,
		MinSize:   10,
		Threshold: 0.9,
	}

	mc := NewMessageCompressor(config)

	// 设置有效级别
	err := mc.SetLevel(9)
	if err != nil {
		t.Errorf("SetLevel(9) should succeed, got error: %v", err)
	}

	// 设置无效级别
	err = mc.SetLevel(0)
	if err == nil {
		t.Errorf("SetLevel(0) should fail")
	}

	err = mc.SetLevel(10)
	if err == nil {
		t.Errorf("SetLevel(10) should fail")
	}
}

func TestMessageCompressor_Stats(t *testing.T) {
	config := &CompressionConfig{
		Enabled:   true,
		Algorithm: CompressionGzip,
		Level:     6,
		MinSize:   10,
		MaxSize:   10 * 1024 * 1024,
		Threshold: 0.9,
	}

	mc := NewMessageCompressor(config)

	// 执行多次压缩
	for i := 0; i < 5; i++ {
		largeData := strings.Repeat("x", 1000)
		msg := &Message{
			ID:        "test-" + string(rune('0'+i)),
			Type:      "test",
			Priority:  PriorityNormal,
			Data:      json.RawMessage(`"` + largeData + `"`),
			Timestamp: time.Now(),
		}
		mc.CompressMessage(msg)
	}

	stats := mc.GetStats()

	if stats.TotalCompressed != 5 {
		t.Errorf("TotalCompressed should be 5, got %v", stats.TotalCompressed)
	}

	if stats.TotalOriginalBytes <= 0 {
		t.Errorf("TotalOriginalBytes should be positive, got %v", stats.TotalOriginalBytes)
	}

	if stats.TotalBytesSaved <= 0 {
		t.Errorf("TotalBytesSaved should be positive, got %v", stats.TotalBytesSaved)
	}

	// 测试重置
	mc.ResetStats()
	stats = mc.GetStats()

	if stats.TotalCompressed != 0 {
		t.Errorf("TotalCompressed should be 0 after reset, got %v", stats.TotalCompressed)
	}
}

func TestMessageCompressor_BatchCompress(t *testing.T) {
	config := &CompressionConfig{
		Enabled:   true,
		Algorithm: CompressionGzip,
		Level:     6,
		MinSize:   10,
		MaxSize:   10 * 1024 * 1024,
		Threshold: 0.9,
	}

	mc := NewMessageCompressor(config)

	// 创建多个消息
	messages := make([]*Message, 5)
	for i := 0; i < 5; i++ {
		largeData := strings.Repeat("x", 1000)
		messages[i] = &Message{
			ID:        "test-" + string(rune('0'+i)),
			Type:      "test",
			Priority:  PriorityNormal,
			Data:      json.RawMessage(`"` + largeData + `"`),
			Timestamp: time.Now(),
		}
	}

	// 批量压缩
	compressed, errors := mc.BatchCompress(messages)

	if len(compressed) != 5 {
		t.Errorf("Should have 5 compressed messages, got %v", len(compressed))
	}

	if len(errors) != 0 {
		t.Errorf("Should have no errors, got %v", len(errors))
	}
}

func TestMessageCompressor_BatchDecompress(t *testing.T) {
	config := &CompressionConfig{
		Enabled:   true,
		Algorithm: CompressionGzip,
		Level:     6,
		MinSize:   10,
		MaxSize:   10 * 1024 * 1024,
		Threshold: 0.9,
	}

	mc := NewMessageCompressor(config)

	// 创建并压缩消息
	messages := make([]*Message, 5)
	for i := 0; i < 5; i++ {
		largeData := strings.Repeat("x", 1000)
		messages[i] = &Message{
			ID:        "test-" + string(rune('0'+i)),
			Type:      "test",
			Priority:  PriorityNormal,
			Data:      json.RawMessage(`"` + largeData + `"`),
			Timestamp: time.Now(),
		}
	}

	compressed, _ := mc.BatchCompress(messages)

	// 批量解压
	decompressed, errors := mc.BatchDecompress(compressed)

	if len(decompressed) != 5 {
		t.Errorf("Should have 5 decompressed messages, got %v", len(decompressed))
	}

	if len(errors) != 0 {
		t.Errorf("Should have no errors, got %v", len(errors))
	}

	// 验证解压后的消息
	for i, msg := range decompressed {
		if msg.ID != messages[i].ID {
			t.Errorf("Message %d ID mismatch: got %v, want %v", i, msg.ID, messages[i].ID)
		}
	}
}

func TestCompressionMiddleware(t *testing.T) {
	config := &CompressionConfig{
		Enabled:   true,
		Algorithm: CompressionGzip,
		Level:     6,
		MinSize:   50,
		MaxSize:   10 * 1024 * 1024,
		Threshold: 0.9,
	}

	middleware := NewCompressionMiddleware(config)

	// 测试出站处理
	testData := []byte(strings.Repeat("Hello, World! ", 100))
	processed, result, err := middleware.ProcessOutgoing(testData)
	if err != nil {
		t.Fatalf("ProcessOutgoing failed: %v", err)
	}

	if result.UsedFallback {
		t.Errorf("Should compress for large data")
	}

	// 测试入站处理
	decompressed, err := middleware.ProcessIncoming(processed, CompressionGzip)
	if err != nil {
		t.Fatalf("ProcessIncoming failed: %v", err)
	}

	if string(decompressed) != string(testData) {
		t.Errorf("Decompressed data mismatch")
	}

	// 测试统计
	stats := middleware.GetStats()
	if stats.TotalCompressed != 1 {
		t.Errorf("TotalCompressed should be 1, got %v", stats.TotalCompressed)
	}
}

func TestMessageCompressor_DifferentAlgorithms(t *testing.T) {
	testData := []byte(strings.Repeat("Hello, World! ", 100))

	algorithms := []CompressionAlgorithm{
		CompressionGzip,
		CompressionZlib,
		CompressionFlate,
	}

	for _, algo := range algorithms {
		t.Run(string(algo), func(t *testing.T) {
			config := &CompressionConfig{
				Enabled:   true,
				Algorithm: algo,
				Level:     6,
				MinSize:   10,
				MaxSize:   10 * 1024 * 1024,
				Threshold: 0.9,
			}

			mc := NewMessageCompressor(config)

			compressed, result, err := mc.CompressData(testData)
			if err != nil {
				t.Fatalf("CompressData failed: %v", err)
			}

			if result.Algorithm != algo {
				t.Errorf("Algorithm mismatch: got %v, want %v", result.Algorithm, algo)
			}

			decompressed, err := mc.DecompressData(compressed, algo)
			if err != nil {
				t.Fatalf("DecompressData failed: %v", err)
			}

			if string(decompressed) != string(testData) {
				t.Errorf("Decompressed data mismatch")
			}
		})
	}
}

func TestMessageCompressor_CompressionLevels(t *testing.T) {
	testData := []byte(strings.Repeat("Hello, World! ", 100))

	levels := []int{1, 3, 6, 9}

	for _, level := range levels {
		t.Run(string(rune('0'+level)), func(t *testing.T) {
			config := &CompressionConfig{
				Enabled:   true,
				Algorithm: CompressionGzip,
				Level:     level,
				MinSize:   10,
				MaxSize:   10 * 1024 * 1024,
				Threshold: 0.9,
			}

			mc := NewMessageCompressor(config)

			compressed, _, err := mc.CompressData(testData)
			if err != nil {
				t.Fatalf("CompressData failed: %v", err)
			}

			decompressed, err := mc.DecompressData(compressed, CompressionGzip)
			if err != nil {
				t.Fatalf("DecompressData failed: %v", err)
			}

			if string(decompressed) != string(testData) {
				t.Errorf("Decompressed data mismatch for level %d", level)
			}
		})
	}
}

func TestCompressionResult_Ratio(t *testing.T) {
	tests := []struct {
		original   int
		compressed int
		expected   float64
	}{
		{100, 50, 0.5},
		{1000, 200, 0.2},
		{100, 90, 0.9},
		{100, 100, 1.0},
	}

	for _, tt := range tests {
		result := CompressionResult{
			OriginalSize:   tt.original,
			CompressedSize: tt.compressed,
			Ratio:          float64(tt.compressed) / float64(tt.original),
		}

		if result.Ratio != tt.expected {
			t.Errorf("Ratio = %v, want %v", result.Ratio, tt.expected)
		}
	}
}

func TestDefaultCompressionConfig(t *testing.T) {
	config := DefaultCompressionConfig

	if !config.Enabled {
		t.Error("Default config should have compression enabled")
	}

	if config.Algorithm != CompressionGzip {
		t.Errorf("Default algorithm should be gzip, got %v", config.Algorithm)
	}

	if config.Level < 1 || config.Level > 9 {
		t.Errorf("Default level should be between 1-9, got %v", config.Level)
	}

	if config.MinSize <= 0 {
		t.Error("Default MinSize should be positive")
	}

	if config.MaxSize <= 0 {
		t.Error("Default MaxSize should be positive")
	}

	if config.Threshold <= 0 || config.Threshold > 1 {
		t.Errorf("Default Threshold should be between 0-1, got %v", config.Threshold)
	}
}

// 基准测试
func BenchmarkGzipCompressor_Compress(b *testing.B) {
	compressor := NewGzipCompressor(6)
	testData := []byte(strings.Repeat("Hello, World! ", 1000))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressor.Compress(testData)
	}
}

func BenchmarkGzipCompressor_Decompress(b *testing.B) {
	compressor := NewGzipCompressor(6)
	testData := []byte(strings.Repeat("Hello, World! ", 1000))
	compressed, _ := compressor.Compress(testData)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressor.Decompress(compressed)
	}
}

func BenchmarkZlibCompressor_Compress(b *testing.B) {
	compressor := NewZlibCompressor(6, nil)
	testData := []byte(strings.Repeat("Hello, World! ", 1000))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressor.Compress(testData)
	}
}

func BenchmarkFlateCompressor_Compress(b *testing.B) {
	compressor := NewFlateCompressor(6, nil)
	testData := []byte(strings.Repeat("Hello, World! ", 1000))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		compressor.Compress(testData)
	}
}

func BenchmarkMessageCompressor_CompressMessage(b *testing.B) {
	config := &CompressionConfig{
		Enabled:   true,
		Algorithm: CompressionGzip,
		Level:     6,
		MinSize:   10,
		MaxSize:   10 * 1024 * 1024,
		Threshold: 0.9,
	}

	mc := NewMessageCompressor(config)

	largeData := strings.Repeat("x", 1000)
	msg := &Message{
		ID:        "test-1",
		Type:      "test",
		Priority:  PriorityNormal,
		Data:      json.RawMessage(`"` + largeData + `"`),
		Timestamp: time.Now(),
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mc.CompressMessage(msg)
	}
}

func BenchmarkMessageCompressor_DecompressMessage(b *testing.B) {
	config := &CompressionConfig{
		Enabled:   true,
		Algorithm: CompressionGzip,
		Level:     6,
		MinSize:   10,
		MaxSize:   10 * 1024 * 1024,
		Threshold: 0.9,
	}

	mc := NewMessageCompressor(config)

	largeData := strings.Repeat("x", 1000)
	msg := &Message{
		ID:        "test-1",
		Type:      "test",
		Priority:  PriorityNormal,
		Data:      json.RawMessage(`"` + largeData + `"`),
		Timestamp: time.Now(),
	}

	cm, _ := mc.CompressMessage(msg)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mc.DecompressMessage(cm)
	}
}
