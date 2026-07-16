// Package audioquality 提供智能音频流质量控制适配器功能。
// 根据网络带宽、设备能力、存储空间自动推荐最佳音质，
// 支持 Hi-Res 音频检测、蓝牙编解码匹配、音质降级策略、
// 网络抖动缓冲和设备能力发现。
package audioquality

import (
	"errors"
	"fmt"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// 音质等级常量
const (
	TierLossless  = "lossless"  // 无损 FLAC/ALAC
	TierHigh      = "high"      // 高品 320k AAC
	TierStandard  = "standard"  // 标准 192k AAC
	TierCompressed = "compressed" // 压缩 128k MP3
)

// 默认比特率 (kbps)
const (
	defaultLosslessBitrate   = 1411
	defaultHighBitrate       = 320
	defaultStandardBitrate   = 192
	defaultCompressedBitrate = 128
)

// 蓝牙编解码器信息
var bluetoothCodecTable = map[string]struct {
	MaxBitrate int
	Lossless   bool
}{
	"LDAC":    {MaxBitrate: 990, Lossless: false},
	"LDHC":    {MaxBitrate: 1024, Lossless: false},
	"aptX":    {MaxBitrate: 384, Lossless: false},
	"aptX_HD": {MaxBitrate: 576, Lossless: false},
	"aptX_Adaptive": {MaxBitrate: 420, Lossless: false},
	"AAC":     {MaxBitrate: 256, Lossless: false},
	"SBC":     {MaxBitrate: 328, Lossless: false},
	"LC3":     {MaxBitrate: 345, Lossless: false},
	"LC3plus": {MaxBitrate: 512, Lossless: false},
}

// AudioQualityAdvisor 智能音频流质量控制适配器。
type AudioQualityAdvisor struct{}

// RecommendOptions 推荐选项。
type RecommendOptions struct {
	NetworkBandwidth float64 // 网络带宽 Mbps
	DeviceName       string  // 设备名称
	StorageFreeGB    float64 // 剩余存储空间 GB
	PreferLossless   bool    // 用户偏好无损
	BluetoothMode    bool    // 是否蓝牙模式
}

// QualityRecommendation 音质推荐结果。
type QualityRecommendation struct {
	Tier            string  // 音质等级
	Bitrate         int     // 推荐比特率 kbps
	Codec           string  // 推荐编解码器
	Reason          string  // 推荐理由
	EstimatedSizeMB float64 // 预估文件大小 MB (每分钟)
}

// HiResInfo Hi-Res 音频检测结果。
type HiResInfo struct {
	IsHiRes    bool // 是否 Hi-Res 音频
	SampleRate int  // 采样率 Hz
	BitDepth   int  // 位深
	Channels   int  // 声道数
}

// BluetoothDeviceInfo 蓝牙设备信息。
type BluetoothDeviceInfo struct {
	Name       string // 设备名称
	Codec      string // 当前编解码器
	MaxBitrate int    // 最大支持比特率 kbps
}

// CodecMatch 编解码器匹配结果。
type CodecMatch struct {
	RecommendedCodec     string // 推荐编解码器
	MaxSupportedBitrate  int    // 最大支持比特率 kbps
	Compatible           bool   // 是否兼容
	Notes                string // 备注
}

// QualityProfile 音质配置。
type QualityProfile struct {
	Format     string // 格式 (FLAC, AAC, MP3, etc.)
	Bitrate    int    // 比特率 kbps
	Channels   int    // 声道数
	SampleRate int    // 采样率 Hz
}

// DowngradeResult 降级结果。
type DowngradeResult struct {
	NewTier         string  // 新音质等级
	NewBitrate      int     // 新比特率 kbps
	SavingsPercent  float64 // 节省百分比
	QualityLoss     string  // 音质损失描述
	Recommended     bool    // 是否推荐降级
}

// NewAdvisor 创建音频质量推荐器。
func NewAdvisor() *AudioQualityAdvisor {
	return &AudioQualityAdvisor{}
}

// Recommend 根据选项推荐最佳音质。
func (a *AudioQualityAdvisor) Recommend(opts RecommendOptions) (*QualityRecommendation, error) {
	if opts.NetworkBandwidth < 0 {
		return nil, errors.New("网络带宽不能为负数")
	}
	if opts.StorageFreeGB < 0 {
		return nil, errors.New("存储空间不能为负数")
	}

	// 将网络带宽 Mbps 转为 kbps
	bandwidthKbps := opts.NetworkBandwidth * 1000

	// 存储不足时强制压缩
	if opts.StorageFreeGB < 1.0 {
		return &QualityRecommendation{
			Tier:            TierCompressed,
			Bitrate:         defaultCompressedBitrate,
			Codec:           "MP3",
			Reason:          "存储空间不足，使用压缩格式节省空间",
			EstimatedSizeMB: calcSizePerMin(defaultCompressedBitrate, "MP3"),
		}, nil
	}

	// 蓝牙模式优先匹配蓝牙编解码器限制
	if opts.BluetoothMode {
		return a.recommendBluetooth(opts, bandwidthKbps), nil
	}

	// 用户偏好无损且条件允许
	if opts.PreferLossless && bandwidthKbps >= defaultLosslessBitrate && opts.StorageFreeGB > 5 {
		return &QualityRecommendation{
			Tier:            TierLossless,
			Bitrate:         defaultLosslessBitrate,
			Codec:           "FLAC",
			Reason:          "网络带宽和存储空间充足，满足无损偏好",
			EstimatedSizeMB: calcSizePerMin(defaultLosslessBitrate, "FLAC"),
		}, nil
	}

	// 根据带宽分级推荐
	switch {
	case bandwidthKbps >= defaultLosslessBitrate && opts.StorageFreeGB > 5:
		return &QualityRecommendation{
			Tier:            TierLossless,
			Bitrate:         defaultLosslessBitrate,
			Codec:           "FLAC",
			Reason:          "网络带宽充足，推荐无损音质",
			EstimatedSizeMB: calcSizePerMin(defaultLosslessBitrate, "FLAC"),
		}, nil

	case bandwidthKbps >= defaultHighBitrate:
		return &QualityRecommendation{
			Tier:            TierHigh,
			Bitrate:         defaultHighBitrate,
			Codec:           "AAC",
			Reason:          "网络带宽支持高品质音质",
			EstimatedSizeMB: calcSizePerMin(defaultHighBitrate, "AAC"),
		}, nil

	case bandwidthKbps >= defaultStandardBitrate:
		return &QualityRecommendation{
			Tier:            TierStandard,
			Bitrate:         defaultStandardBitrate,
			Codec:           "AAC",
			Reason:          "网络带宽适中，使用标准音质",
			EstimatedSizeMB: calcSizePerMin(defaultStandardBitrate, "AAC"),
		}, nil

	default:
		return &QualityRecommendation{
			Tier:            TierCompressed,
			Bitrate:         defaultCompressedBitrate,
			Codec:           "MP3",
			Reason:          "网络带宽有限，使用压缩音质保证流畅播放",
			EstimatedSizeMB: calcSizePerMin(defaultCompressedBitrate, "MP3"),
		}, nil
	}
}

// recommendBluetooth 蓝牙模式下的音质推荐。
func (a *AudioQualityAdvisor) recommendBluetooth(opts RecommendOptions, bandwidthKbps float64) *QualityRecommendation {
	// 蓝牙模式下受编解码器限制，无法达到真正无损
	maxBtBitrate := 990 // LDAC 最高

	if opts.NetworkBandwidth > 0 && bandwidthKbps < float64(maxBtBitrate) {
		maxBtBitrate = int(bandwidthKbps)
	}

	switch {
	case maxBtBitrate >= 990:
		return &QualityRecommendation{
			Tier:            TierHigh,
			Bitrate:         990,
			Codec:           "LDAC",
			Reason:          "蓝牙 LDAC 编解码器支持高品质传输",
			EstimatedSizeMB: calcSizePerMin(990, "LDAC"),
		}
	case maxBtBitrate >= 576:
		return &QualityRecommendation{
			Tier:            TierHigh,
			Bitrate:         576,
			Codec:           "aptX_HD",
			Reason:          "蓝牙 aptX HD 编解码器支持高品质传输",
			EstimatedSizeMB: calcSizePerMin(576, "aptX_HD"),
		}
	case maxBtBitrate >= 384:
		return &QualityRecommendation{
			Tier:            TierStandard,
			Bitrate:         384,
			Codec:           "aptX",
			Reason:          "蓝牙 aptX 编解码器支持标准音质传输",
			EstimatedSizeMB: calcSizePerMin(384, "aptX"),
		}
	case maxBtBitrate >= 256:
		return &QualityRecommendation{
			Tier:            TierStandard,
			Bitrate:         256,
			Codec:           "AAC",
			Reason:          "蓝牙 AAC 编解码器提供标准音质",
			EstimatedSizeMB: calcSizePerMin(256, "AAC"),
		}
	default:
		return &QualityRecommendation{
			Tier:            TierCompressed,
			Bitrate:         128,
			Codec:           "SBC",
			Reason:          "蓝牙带宽有限，使用 SBC 基础编解码器",
			EstimatedSizeMB: calcSizePerMin(128, "SBC"),
		}
	}
}

// DetectHiRes 检测音频文件是否为 Hi-Res 音频。
// 通过文件扩展名和采样率/位深信息进行判断。
func (a *AudioQualityAdvisor) DetectHiRes(path string) (*HiResInfo, error) {
	if path == "" {
		return nil, errors.New("文件路径不能为空")
	}

	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("无法访问文件: %w", err)
	}

	info := &HiResInfo{}

	ext := strings.ToLower(filepath.Ext(path))

	switch ext {
	case ".flac", ".wav", ".aiff", ".dsd", ".dff", ".dsf":
		// 这些格式可能是 Hi-Res
		// 尝试从文件名解析采样率和位深（格式: 96kHz_24bit.flac）
		base := filepath.Base(path)
		info = parseHiResFilename(base)

		// 如果文件名没有包含信息，使用默认 Hi-Res 参数
		if info.SampleRate == 0 {
			info.SampleRate = 96000
			info.BitDepth = 24
			info.Channels = 2
			info.IsHiRes = true
		}
	case ".alac", ".m4a":
		// ALAC 可能是 Hi-Res，也可能不是
		info.SampleRate = 44100
		info.BitDepth = 16
		info.Channels = 2
		info.IsHiRes = false

	case ".mp3":
		info.SampleRate = 44100
		info.BitDepth = 16
		info.Channels = 2
		info.IsHiRes = false

	case ".aac":
		info.SampleRate = 44100
		info.BitDepth = 16
		info.Channels = 2
		info.IsHiRes = false

	case ".ogg":
		info.SampleRate = 44100
		info.BitDepth = 16
		info.Channels = 2
		info.IsHiRes = false

	default:
		return nil, fmt.Errorf("不支持的音频格式: %s", ext)
	}

	// Hi-Res 标准：采样率 >= 48kHz 且位深 >= 24bit
	if info.SampleRate >= 48000 && info.BitDepth >= 24 {
		info.IsHiRes = true
	}

	return info, nil
}

// parseHiResFilename 从文件名解析 Hi-Res 信息。
// 支持格式如 "96kHz_24bit.flac" 或 "192-24.flac"
func parseHiResFilename(filename string) *HiResInfo {
	info := &HiResInfo{}

	// 尝试匹配 "96kHz_24bit" 或 "96khz_24bit" 模式
	lower := strings.ToLower(filename)

	// 查找采样率
	for _, sr := range []int{44100, 48000, 88200, 96000, 176400, 192000, 352800, 384000} {
		srStr := strconv.Itoa(sr / 1000)
		if strings.Contains(lower, srStr+"khz") || strings.Contains(lower, srStr+"kh") || strings.Contains(lower, srStr+"-") {
			info.SampleRate = sr
			break
		}
	}

	// 查找位深
	for _, bd := range []int{16, 24, 32} {
		bdStr := strconv.Itoa(bd)
		if strings.Contains(lower, bdStr+"bit") || strings.Contains(lower, "_"+bdStr+".") || strings.Contains(lower, "-"+bdStr+".") || strings.Contains(lower, "_"+bdStr+"_") {
			info.BitDepth = bd
			break
		}
	}

	// 默认声道数
	info.Channels = 2

	return info
}

// MatchBluetoothCodec 匹配蓝牙编解码器。
func (a *AudioQualityAdvisor) MatchBluetoothCodec(deviceInfo BluetoothDeviceInfo) (*CodecMatch, error) {
	if deviceInfo.Name == "" {
		return nil, errors.New("设备名称不能为空")
	}

	codec := strings.TrimSpace(deviceInfo.Codec)
	if codec == "" {
		return &CodecMatch{
			RecommendedCodec:    "SBC",
			MaxSupportedBitrate: 328,
			Compatible:          true,
			Notes:               "未指定编解码器，使用通用 SBC 编解码器",
		}, nil
	}

	// 标准化编解码器名称（去除空格、横线等）
	normalizedCodec := strings.ReplaceAll(codec, "-", "_")
	normalizedCodec = strings.ReplaceAll(normalizedCodec, " ", "_")

	if codecInfo, ok := bluetoothCodecTable[normalizedCodec]; ok {
		notes := fmt.Sprintf("%s 支持最高 %d kbps", codec, codecInfo.MaxBitrate)
		if codecInfo.Lossless {
			notes += "，支持无损传输"
		} else {
			notes += "，有损压缩"
		}

		return &CodecMatch{
			RecommendedCodec:    codec,
			MaxSupportedBitrate: codecInfo.MaxBitrate,
			Compatible:          true,
			Notes:               notes,
		}, nil
	}

	// 未知编解码器，回退到 SBC
	return &CodecMatch{
		RecommendedCodec:    "SBC",
		MaxSupportedBitrate: 328,
		Compatible:          false,
		Notes:               fmt.Sprintf("未知编解码器 %s，回退到通用 SBC", codec),
	}, nil
}

// CalculateBitrate 根据音质配置计算实际所需比特率。
func (a *AudioQualityAdvisor) CalculateBitrate(profile QualityProfile) (int, error) {
	if profile.SampleRate <= 0 {
		return 0, errors.New("采样率必须大于 0")
	}
	if profile.Channels <= 0 {
		return 0, errors.New("声道数必须大于 0")
	}
	if profile.Bitrate < 0 {
		return 0, errors.New("比特率不能为负数")
	}

	format := strings.ToUpper(profile.Format)

	switch format {
	case "FLAC", "WAV", "AIFF":
		// 无损格式：比特率 = 采样率 × 位深 × 声道数 / 1000
		// 假设位深为 16（如果未提供其他信息）
		bitDepth := 16
		if profile.Bitrate > 0 {
			return profile.Bitrate, nil
		}
		return profile.SampleRate * bitDepth * profile.Channels / 1000, nil

	case "ALAC":
		// ALAC 无损，但压缩率约 50-60%
		bitDepth := 16
		if profile.Bitrate > 0 {
			return profile.Bitrate, nil
		}
		rawBitrate := profile.SampleRate * bitDepth * profile.Channels / 1000
		return int(math.Round(float64(rawBitrate) * 0.55)), nil

	case "AAC", "MP3", "OGG", "OPUS":
		// 有损压缩格式直接使用指定比特率
		if profile.Bitrate > 0 {
			return profile.Bitrate, nil
		}
		// 根据采样率推算合理默认值
		if profile.SampleRate >= 48000 {
			return defaultHighBitrate, nil
		}
		return defaultStandardBitrate, nil

	case "DSD", "DFF", "DSF":
		// DSD 格式：DSD64 = 2.8MHz = ~2900 kbps per channel
		if profile.Bitrate > 0 {
			return profile.Bitrate, nil
		}
		return 2822 * profile.Channels, nil

	default:
		if profile.Bitrate > 0 {
			return profile.Bitrate, nil
		}
		return 0, fmt.Errorf("未知格式: %s，无法计算比特率", profile.Format)
	}
}

// RecommendDowngrade 根据当前比特率和降级原因推荐降级策略。
func (a *AudioQualityAdvisor) RecommendDowngrade(currentBitrate int, reason string) (*DowngradeResult, error) {
	if currentBitrate <= 0 {
		return nil, errors.New("当前比特率必须大于 0")
	}

	reasonLower := strings.ToLower(reason)

	switch {
	case strings.Contains(reasonLower, "网络") || strings.Contains(reasonLower, "network"):
		// 网络原因降级
		if currentBitrate <= defaultCompressedBitrate {
			return &DowngradeResult{
				NewTier:        TierCompressed,
				NewBitrate:     defaultCompressedBitrate,
				SavingsPercent: 0,
				QualityLoss:    "已经是最低音质等级，无法继续降级",
				Recommended:    false,
			}, nil
		}
		switch {
		case currentBitrate > defaultHighBitrate:
			return &DowngradeResult{
				NewTier:        TierHigh,
				NewBitrate:     defaultHighBitrate,
				SavingsPercent: calcSavings(currentBitrate, defaultHighBitrate),
				QualityLoss:    "从无损降级到高品质 320k AAC，高频细节有一定损失",
				Recommended:    true,
			}, nil
		case currentBitrate > defaultStandardBitrate:
			return &DowngradeResult{
				NewTier:        TierStandard,
				NewBitrate:     defaultStandardBitrate,
				SavingsPercent: calcSavings(currentBitrate, defaultStandardBitrate),
				QualityLoss:    "从高品质降级到标准音质，高频和中频细节有损失",
				Recommended:    true,
			}, nil
		default:
			return &DowngradeResult{
				NewTier:        TierCompressed,
				NewBitrate:     defaultCompressedBitrate,
				SavingsPercent: calcSavings(currentBitrate, defaultCompressedBitrate),
				QualityLoss:    "降级到压缩音质，音质明显下降但保证播放流畅",
				Recommended:    true,
			}, nil
		}

	case strings.Contains(reasonLower, "存储") || strings.Contains(reasonLower, "storage"):
		// 存储原因降级——更激进
		switch {
		case currentBitrate > defaultHighBitrate:
			return &DowngradeResult{
				NewTier:        TierStandard,
				NewBitrate:     defaultStandardBitrate,
				SavingsPercent: calcSavings(currentBitrate, defaultStandardBitrate),
				QualityLoss:    "从无损/高品质直接降级到标准音质以节省存储空间",
				Recommended:    true,
			}, nil
		case currentBitrate > defaultCompressedBitrate:
			return &DowngradeResult{
				NewTier:        TierCompressed,
				NewBitrate:     defaultCompressedBitrate,
				SavingsPercent: calcSavings(currentBitrate, defaultCompressedBitrate),
				QualityLoss:    "降级到压缩音质，大幅节省存储空间",
				Recommended:    true,
			}, nil
		default:
			return &DowngradeResult{
				NewTier:        TierCompressed,
				NewBitrate:     defaultCompressedBitrate,
				SavingsPercent: 0,
				QualityLoss:    "已经是最低音质等级，无法继续降级",
				Recommended:    false,
			}, nil
		}

	case strings.Contains(reasonLower, "蓝牙") || strings.Contains(reasonLower, "bluetooth"):
		// 蓝牙限制降级
		maxBtBitrate := 990 // LDAC 最高
		if currentBitrate <= maxBtBitrate {
			return &DowngradeResult{
				NewTier:        TierHigh,
				NewBitrate:     currentBitrate,
				SavingsPercent: 0,
				QualityLoss:    "当前比特率在蓝牙可接受范围内，无需降级",
				Recommended:    false,
			}, nil
		}
		return &DowngradeResult{
			NewTier:        TierHigh,
			NewBitrate:     maxBtBitrate,
			SavingsPercent: calcSavings(currentBitrate, maxBtBitrate),
			QualityLoss:    "蓝牙传输限制，从无损降级到高品质 LDAC",
			Recommended:    true,
		}, nil

	default:
		// 通用降级
		target := defaultHighBitrate
		if currentBitrate <= defaultHighBitrate {
			target = defaultStandardBitrate
		}
		if currentBitrate <= defaultStandardBitrate {
			target = defaultCompressedBitrate
		}
		if currentBitrate <= defaultCompressedBitrate {
			return &DowngradeResult{
				NewTier:        TierCompressed,
				NewBitrate:     defaultCompressedBitrate,
				SavingsPercent: 0,
				QualityLoss:    "已经是最低音质等级",
				Recommended:    false,
			}, nil
		}

		tier := TierHigh
		if target == defaultStandardBitrate {
			tier = TierStandard
		} else if target == defaultCompressedBitrate {
			tier = TierCompressed
		}

		return &DowngradeResult{
			NewTier:        tier,
			NewBitrate:     target,
			SavingsPercent: calcSavings(currentBitrate, target),
			QualityLoss:    "根据条件限制降级音质",
			Recommended:    true,
		}, nil
	}
}

// calcSizePerMin 计算每分钟音频的预估文件大小（MB）。
func calcSizePerMin(bitrateKbps int, codec string) float64 {
	// 文件大小 (MB) = 比特率 (kbps) × 60 (秒) / 8 / 1024
	// 对于无损格式如 FLAC 有约 50-60% 压缩率
	codecUpper := strings.ToUpper(codec)
	compressionRatio := 1.0

	switch codecUpper {
	case "FLAC", "ALAC":
		compressionRatio = 0.55 // 无损压缩约 55% 原始大小
	case "WAV", "AIFF":
		compressionRatio = 1.0 // 未压缩
	case "DSD", "DFF", "DSF":
		compressionRatio = 1.0
	}

	size := float64(bitrateKbps) * 60 / 8 / 1024 * compressionRatio
	return math.Round(size*100) / 100
}

// calcSavings 计算节省百分比。
func calcSavings(oldBitrate, newBitrate int) float64 {
	if oldBitrate <= 0 {
		return 0
	}
	saved := math.Round((1.0-float64(newBitrate)/float64(oldBitrate))*10000) / 100
	if saved < 0 {
		return 0
	}
	return saved
}