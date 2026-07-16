package audioquality

import (
	"os"
	"path/filepath"
	"testing"
)

func TestNewAdvisor(t *testing.T) {
	advisor := NewAdvisor()
	if advisor == nil {
		t.Fatal("NewAdvisor 返回 nil")
	}
}

func TestRecommend_Lossless(t *testing.T) {
	advisor := NewAdvisor()
	opts := RecommendOptions{
		NetworkBandwidth: 2.0,    // 2000 kbps
		DeviceName:       "HiFi",
		StorageFreeGB:    100,
		PreferLossless:   true,
	}

	rec, err := advisor.Recommend(opts)
	if err != nil {
		t.Fatalf("Recommend 失败: %v", err)
	}
	if rec.Tier != TierLossless {
		t.Errorf("期望 tier=%s, got %s", TierLossless, rec.Tier)
	}
	if rec.Bitrate != defaultLosslessBitrate {
		t.Errorf("期望 bitrate=%d, got %d", defaultLosslessBitrate, rec.Bitrate)
	}
	if rec.Codec != "FLAC" {
		t.Errorf("期望 codec=FLAC, got %s", rec.Codec)
	}
	if rec.EstimatedSizeMB <= 0 {
		t.Error("EstimatedSizeMB 应大于 0")
	}
}

func TestRecommend_HighQuality(t *testing.T) {
	advisor := NewAdvisor()
	opts := RecommendOptions{
		NetworkBandwidth: 0.5, // 500 kbps
		StorageFreeGB:    10,
	}

	rec, err := advisor.Recommend(opts)
	if err != nil {
		t.Fatalf("Recommend 失败: %v", err)
	}
	if rec.Tier != TierHigh {
		t.Errorf("期望 tier=%s, got %s", TierHigh, rec.Tier)
	}
	if rec.Bitrate != defaultHighBitrate {
		t.Errorf("期望 bitrate=%d, got %d", defaultHighBitrate, rec.Bitrate)
	}
}

func TestRecommend_Standard(t *testing.T) {
	advisor := NewAdvisor()
	opts := RecommendOptions{
		NetworkBandwidth: 0.25, // 250 kbps
		StorageFreeGB:    10,
	}

	rec, err := advisor.Recommend(opts)
	if err != nil {
		t.Fatalf("Recommend 失败: %v", err)
	}
	if rec.Tier != TierStandard {
		t.Errorf("期望 tier=%s, got %s", TierStandard, rec.Tier)
	}
}

func TestRecommend_Compressed(t *testing.T) {
	advisor := NewAdvisor()
	opts := RecommendOptions{
		NetworkBandwidth: 0.1, // 100 kbps
		StorageFreeGB:    10,
	}

	rec, err := advisor.Recommend(opts)
	if err != nil {
		t.Fatalf("Recommend 失败: %v", err)
	}
	if rec.Tier != TierCompressed {
		t.Errorf("期望 tier=%s, got %s", TierCompressed, rec.Tier)
	}
	if rec.Bitrate != defaultCompressedBitrate {
		t.Errorf("期望 bitrate=%d, got %d", defaultCompressedBitrate, rec.Bitrate)
	}
}

func TestRecommend_LowStorage(t *testing.T) {
	advisor := NewAdvisor()
	opts := RecommendOptions{
		NetworkBandwidth: 2.0,
		StorageFreeGB:    0.5, // 存储不足
	}

	rec, err := advisor.Recommend(opts)
	if err != nil {
		t.Fatalf("Recommend 失败: %v", err)
	}
	if rec.Tier != TierCompressed {
		t.Errorf("存储不足时应该推荐压缩格式, got %s", rec.Tier)
	}
	if rec.Reason == "" {
		t.Error("Reason 不能为空")
	}
}

func TestRecommend_BluetoothMode(t *testing.T) {
	advisor := NewAdvisor()
	opts := RecommendOptions{
		NetworkBandwidth: 2.0,
		BluetoothMode:    true,
		StorageFreeGB:    10,
	}

	rec, err := advisor.Recommend(opts)
	if err != nil {
		t.Fatalf("Recommend 失败: %v", err)
	}
	if rec.Tier != TierHigh {
		t.Errorf("蓝牙模式应该推荐高品质, got %s", rec.Tier)
	}
	if rec.Codec != "LDAC" {
		t.Errorf("期望 codec=LDAC, got %s", rec.Codec)
	}
}

func TestRecommend_BluetoothLowBandwidth(t *testing.T) {
	advisor := NewAdvisor()
	opts := RecommendOptions{
		NetworkBandwidth: 0.3, // 300 kbps
		BluetoothMode:    true,
		StorageFreeGB:    10,
	}

	rec, err := advisor.Recommend(opts)
	if err != nil {
		t.Fatalf("Recommend 失败: %v", err)
	}
	// 300 kbps 应该选择 aptX 或 AAC
	if rec.Bitrate > 384 {
		t.Errorf("低带宽蓝牙模式比特率不应超过 384, got %d", rec.Bitrate)
	}
}

func TestRecommend_NegativeBandwidth(t *testing.T) {
	advisor := NewAdvisor()
	opts := RecommendOptions{
		NetworkBandwidth: -1,
	}

	_, err := advisor.Recommend(opts)
	if err == nil {
		t.Error("负带宽应返回错误")
	}
}

func TestRecommend_NegativeStorage(t *testing.T) {
	advisor := NewAdvisor()
	opts := RecommendOptions{
		NetworkBandwidth: 1.0,
		StorageFreeGB:    -1,
	}

	_, err := advisor.Recommend(opts)
	if err == nil {
		t.Error("负存储应返回错误")
	}
}

func TestDetectHiRes_FLAC(t *testing.T) {
	// 创建临时文件
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "track_96kHz_24bit.flac")
	if err := os.WriteFile(path, []byte("fake flac"), 0644); err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}

	advisor := NewAdvisor()
	info, err := advisor.DetectHiRes(path)
	if err != nil {
		t.Fatalf("DetectHiRes 失败: %v", err)
	}
	if !info.IsHiRes {
		t.Error("96kHz/24bit FLAC 应该被检测为 Hi-Res")
	}
	if info.SampleRate != 96000 {
		t.Errorf("期望采样率 96000, got %d", info.SampleRate)
	}
	if info.BitDepth != 24 {
		t.Errorf("期望位深 24, got %d", info.BitDepth)
	}
}

func TestDetectHiRes_MP3(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "song.mp3")
	if err := os.WriteFile(path, []byte("fake mp3"), 0644); err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}

	advisor := NewAdvisor()
	info, err := advisor.DetectHiRes(path)
	if err != nil {
		t.Fatalf("DetectHiRes 失败: %v", err)
	}
	if info.IsHiRes {
		t.Error("MP3 不应该被检测为 Hi-Res")
	}
	if info.SampleRate != 44100 {
		t.Errorf("期望采样率 44100, got %d", info.SampleRate)
	}
}

func TestDetectHiRes_EmptyPath(t *testing.T) {
	advisor := NewAdvisor()
	_, err := advisor.DetectHiRes("")
	if err == nil {
		t.Error("空路径应返回错误")
	}
}

func TestDetectHiRes_NonExistent(t *testing.T) {
	advisor := NewAdvisor()
	_, err := advisor.DetectHiRes("/nonexistent/path/file.flac")
	if err == nil {
		t.Error("不存在的文件应返回错误")
	}
}

func TestDetectHiRes_UnknownFormat(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "audio.xyz")
	if err := os.WriteFile(path, []byte("fake"), 0644); err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}

	advisor := NewAdvisor()
	_, err := advisor.DetectHiRes(path)
	if err == nil {
		t.Error("未知格式应返回错误")
	}
}

func TestDetectHiRes_WAV(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "192kHz_32bit.wav")
	if err := os.WriteFile(path, []byte("fake wav"), 0644); err != nil {
		t.Fatalf("创建临时文件失败: %v", err)
	}

	advisor := NewAdvisor()
	info, err := advisor.DetectHiRes(path)
	if err != nil {
		t.Fatalf("DetectHiRes 失败: %v", err)
	}
	if !info.IsHiRes {
		t.Error("192kHz/32bit WAV 应该被检测为 Hi-Res")
	}
}

func TestMatchBluetoothCodec_LDAC(t *testing.T) {
	advisor := NewAdvisor()
	deviceInfo := BluetoothDeviceInfo{
		Name:       "Sony WH-1000XM4",
		Codec:      "LDAC",
		MaxBitrate: 990,
	}

	match, err := advisor.MatchBluetoothCodec(deviceInfo)
	if err != nil {
		t.Fatalf("MatchBluetoothCodec 失败: %v", err)
	}
	if !match.Compatible {
		t.Error("LDAC 应该是兼容的")
	}
	if match.RecommendedCodec != "LDAC" {
		t.Errorf("期望 LDAC, got %s", match.RecommendedCodec)
	}
	if match.MaxSupportedBitrate != 990 {
		t.Errorf("期望 990 kbps, got %d", match.MaxSupportedBitrate)
	}
}

func TestMatchBluetoothCodec_EmptyCodec(t *testing.T) {
	advisor := NewAdvisor()
	deviceInfo := BluetoothDeviceInfo{
		Name:       "Unknown Device",
		Codec:      "",
		MaxBitrate: 0,
	}

	match, err := advisor.MatchBluetoothCodec(deviceInfo)
	if err != nil {
		t.Fatalf("MatchBluetoothCodec 失败: %v", err)
	}
	if match.RecommendedCodec != "SBC" {
		t.Errorf("空编解码器应回退到 SBC, got %s", match.RecommendedCodec)
	}
	if !match.Compatible {
		t.Error("SBC 应该是兼容的")
	}
}

func TestMatchBluetoothCodec_UnknownCodec(t *testing.T) {
	advisor := NewAdvisor()
	deviceInfo := BluetoothDeviceInfo{
		Name:       "Mystery Device",
		Codec:      "UnknownCodec",
		MaxBitrate: 100,
	}

	match, err := advisor.MatchBluetoothCodec(deviceInfo)
	if err != nil {
		t.Fatalf("MatchBluetoothCodec 失败: %v", err)
	}
	if match.Compatible {
		t.Error("未知编解码器应该不兼容")
	}
}

func TestMatchBluetoothCodec_EmptyName(t *testing.T) {
	advisor := NewAdvisor()
	deviceInfo := BluetoothDeviceInfo{
		Name: "",
	}

	_, err := advisor.MatchBluetoothCodec(deviceInfo)
	if err == nil {
		t.Error("空设备名应返回错误")
	}
}

func TestMatchBluetoothCodec_AptXHD(t *testing.T) {
	advisor := NewAdvisor()
	deviceInfo := BluetoothDeviceInfo{
		Name:       "Audio-Technica",
		Codec:      "aptX-HD",
		MaxBitrate: 576,
	}

	match, err := advisor.MatchBluetoothCodec(deviceInfo)
	if err != nil {
		t.Fatalf("MatchBluetoothCodec 失败: %v", err)
	}
	if !match.Compatible {
		t.Error("aptX-HD 应该是兼容的")
	}
	if match.MaxSupportedBitrate != 576 {
		t.Errorf("期望 576 kbps, got %d", match.MaxSupportedBitrate)
	}
}

func TestCalculateBitrate_FLAC(t *testing.T) {
	advisor := NewAdvisor()
	profile := QualityProfile{
		Format:     "FLAC",
		Bitrate:    0,
		Channels:   2,
		SampleRate: 44100,
	}

	bitrate, err := advisor.CalculateBitrate(profile)
	if err != nil {
		t.Fatalf("CalculateBitrate 失败: %v", err)
	}
	// 44100 × 16 × 2 / 1000 = 1411
	if bitrate != 1411 {
		t.Errorf("期望 1411, got %d", bitrate)
	}
}

func TestCalculateBitrate_AAC(t *testing.T) {
	advisor := NewAdvisor()
	profile := QualityProfile{
		Format:     "AAC",
		Bitrate:    320,
		Channels:   2,
		SampleRate: 48000,
	}

	bitrate, err := advisor.CalculateBitrate(profile)
	if err != nil {
		t.Fatalf("CalculateBitrate 失败: %v", err)
	}
	if bitrate != 320 {
		t.Errorf("期望 320, got %d", bitrate)
	}
}

func TestCalculateBitrate_InvalidSampleRate(t *testing.T) {
	advisor := NewAdvisor()
	profile := QualityProfile{
		Format:     "AAC",
		Bitrate:    128,
		Channels:   2,
		SampleRate: 0,
	}

	_, err := advisor.CalculateBitrate(profile)
	if err == nil {
		t.Error("无效采样率应返回错误")
	}
}

func TestCalculateBitrate_InvalidChannels(t *testing.T) {
	advisor := NewAdvisor()
	profile := QualityProfile{
		Format:     "AAC",
		Bitrate:    128,
		Channels:   0,
		SampleRate: 44100,
	}

	_, err := advisor.CalculateBitrate(profile)
	if err == nil {
		t.Error("无效声道数应返回错误")
	}
}

func TestCalculateBitrate_UnknownFormat(t *testing.T) {
	advisor := NewAdvisor()
	profile := QualityProfile{
		Format:     "UNKNOWN",
		Bitrate:    0,
		Channels:   2,
		SampleRate: 44100,
	}

	_, err := advisor.CalculateBitrate(profile)
	if err == nil {
		t.Error("未知格式应返回错误")
	}
}

func TestCalculateBitrate_DSD(t *testing.T) {
	advisor := NewAdvisor()
	profile := QualityProfile{
		Format:     "DSD",
		Bitrate:    0,
		Channels:   2,
		SampleRate: 2822400,
	}

	bitrate, err := advisor.CalculateBitrate(profile)
	if err != nil {
		t.Fatalf("CalculateBitrate 失败: %v", err)
	}
	// 2822 × 2 = 5644
	if bitrate != 5644 {
		t.Errorf("期望 5644, got %d", bitrate)
	}
}

func TestRecommendDowngrade_NetworkReason(t *testing.T) {
	advisor := NewAdvisor()
	result, err := advisor.RecommendDowngrade(1411, "网络带宽不足")
	if err != nil {
		t.Fatalf("RecommendDowngrade 失败: %v", err)
	}
	if !result.Recommended {
		t.Error("应该推荐降级")
	}
	if result.NewTier != TierHigh {
		t.Errorf("期望 tier=%s, got %s", TierHigh, result.NewTier)
	}
	if result.NewBitrate != defaultHighBitrate {
		t.Errorf("期望 bitrate=%d, got %d", defaultHighBitrate, result.NewBitrate)
	}
	if result.SavingsPercent <= 0 {
		t.Error("节省百分比应大于 0")
	}
}

func TestRecommendDowngrade_StorageReason(t *testing.T) {
	advisor := NewAdvisor()
	result, err := advisor.RecommendDowngrade(1411, "存储空间不足")
	if err != nil {
		t.Fatalf("RecommendDowngrade 失败: %v", err)
	}
	if !result.Recommended {
		t.Error("应该推荐降级")
	}
	// 存储原因更激进，直接降到标准
	if result.NewTier != TierStandard {
		t.Errorf("期望 tier=%s, got %s", TierStandard, result.NewTier)
	}
}

func TestRecommendDowngrade_BluetoothReason(t *testing.T) {
	advisor := NewAdvisor()
	result, err := advisor.RecommendDowngrade(1411, "蓝牙限制")
	if err != nil {
		t.Fatalf("RecommendDowngrade 失败: %v", err)
	}
	if !result.Recommended {
		t.Error("应该推荐降级")
	}
	if result.NewBitrate != 990 {
		t.Errorf("期望 990 kbps (LDAC), got %d", result.NewBitrate)
	}
}

func TestRecommendDowngrade_AlreadyLow(t *testing.T) {
	advisor := NewAdvisor()
	result, err := advisor.RecommendDowngrade(128, "网络带宽不足")
	if err != nil {
		t.Fatalf("RecommendDowngrade 失败: %v", err)
	}
	if result.Recommended {
		t.Error("已经是最低音质，不应该推荐降级")
	}
}

func TestRecommendDowngrade_InvalidBitrate(t *testing.T) {
	advisor := NewAdvisor()
	_, err := advisor.RecommendDowngrade(0, "测试")
	if err == nil {
		t.Error("比特率为 0 应返回错误")
	}
}

func TestRecommendDowngrade_GenericReason(t *testing.T) {
	advisor := NewAdvisor()
	result, err := advisor.RecommendDowngrade(320, "自动调节")
	if err != nil {
		t.Fatalf("RecommendDowngrade 失败: %v", err)
	}
	if !result.Recommended {
		t.Error("应该推荐降级")
	}
	if result.NewTier != TierStandard {
		t.Errorf("期望 tier=%s, got %s", TierStandard, result.NewTier)
	}
}

func TestCalcSizePerMin(t *testing.T) {
	size := calcSizePerMin(1411, "FLAC")
	if size <= 0 {
		t.Error("FLAC 文件大小应大于 0")
	}
	// 1411 kbps × 60 / 8 / 1024 × 0.55 ≈ 5.68 MB
	if size < 5 || size > 7 {
		t.Errorf("FLAC 每分钟大小应在 5-7 MB 之间, got %.2f", size)
	}
}

func TestCalcSavings(t *testing.T) {
	saved := calcSavings(1411, 320)
	// (1 - 320/1411) × 100 ≈ 77.39%
	if saved < 70 || saved > 85 {
		t.Errorf("节省百分比应在 70-85%% 之间, got %.2f", saved)
	}
}

func TestCalcSavings_ZeroBitrate(t *testing.T) {
	saved := calcSavings(0, 100)
	if saved != 0 {
		t.Errorf("原比特率为 0 时应返回 0, got %.2f", saved)
	}
}