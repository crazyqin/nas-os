// Package media provides Dolby Vision/Atmos playback configuration
// Supports Blu-ray, HDR, Dolby Vision, and Dolby Atmos audio configurations
package media

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// DolbyVisionProfile represents Dolby Vision profile
type DolbyVisionProfile string

const (
	// DVProfile5 represents Dolby Vision Profile 5 (HDR10 base layer only)
	DVProfile5 DolbyVisionProfile = "profile5" // HDR10 base layer only
	// DVProfile7 represents Dolby Vision Profile 7 (HDR10 + Dolby Vision dual layer)
	DVProfile7 DolbyVisionProfile = "profile7" // HDR10 + Dolby Vision dual layer
	// DVProfile8 represents Dolby Vision Profile 8 (HDR10 + Dolby Vision single layer)
	DVProfile8 DolbyVisionProfile = "profile8" // HDR10 + Dolby Vision single layer
	// DVProfileMEL represents Dolby Vision MEL profile
	DVProfileMEL DolbyVisionProfile = "profile-mel" // MEL (Minimum Enhanced Layer)
	// DVProfileFEL represents Dolby Vision FEL profile
	DVProfileFEL DolbyVisionProfile = "profile-fel" // FEL (Full Enhanced Layer)
)

// AudioCodec represents supported audio codecs
type AudioCodec string

const (
	// AudioAAC represents AAC audio codec
	AudioAAC AudioCodec = "aac"
	// AudioAC3 represents AC3/Dolby Digital audio codec
	AudioAC3 AudioCodec = "ac3"
	// AudioEAC3 represents Dolby Digital Plus audio codec
	AudioEAC3 AudioCodec = "eac3" // Dolby Digital Plus
	// AudioTrueHD represents Dolby TrueHD audio codec
	AudioTrueHD AudioCodec = "truehd" // Dolby TrueHD
	// AudioAtmos represents Dolby Atmos audio codec
	AudioAtmos AudioCodec = "atmos" // Dolby Atmos
	// AudioDTS represents DTS audio codec
	AudioDTS AudioCodec = "dts"
	// AudioDTSHD represents DTS-HD audio codec
	AudioDTSHD AudioCodec = "dtshd" // DTS-HD Master Audio
	// AudioDTSHDMA represents DTS-HD Master Audio codec
	AudioDTSHDMA AudioCodec = "dts-hd-ma" // DTS-HD Master Audio
	// AudioDTSHDHR represents DTS-HD High Resolution audio codec
	AudioDTSHDHR AudioCodec = "dts-hd-hra" // DTS-HD High Resolution
	// AudioLPCM represents LPCM audio codec
	AudioLPCM AudioCodec = "lpcm"
	// AudioFLAC represents FLAC audio codec
	AudioFLAC AudioCodec = "flac"
	// AudioOpus represents Opus audio codec
	AudioOpus AudioCodec = "opus"
)

// HDRFormat represents HDR format
type HDRFormat string

const (
	// HDRNone indicates no HDR format
	HDRNone HDRFormat = "none"
	// HDR10 represents standard HDR10 format
	HDR10 HDRFormat = "hdr10"
	// HDR10Plus represents HDR10+ format
	HDR10Plus HDRFormat = "hdr10plus"
	// DolbyVision represents Dolby Vision HDR format
	DolbyVision HDRFormat = "dolby-vision"
	// HLG represents Hybrid Log-Gamma HDR format
	HLG HDRFormat = "hlg"
)

// VideoCodec represents supported video codecs
type VideoCodec string

const (
	// VideoH264 represents H.264/AVC video codec
	VideoH264 VideoCodec = "h264"
	// VideoH265 represents H.265/HEVC video codec
	VideoH265 VideoCodec = "h265"
	// VideoHEVC represents HEVC video codec
	VideoHEVC VideoCodec = "hevc"
	// VideoVP9 represents VP9 video codec
	VideoVP9 VideoCodec = "vp9"
	// VideoAV1 represents AV1 video codec
	VideoAV1 VideoCodec = "av1"
)

// BluRayPlaybackConfig represents Blu-ray playback configuration
type BluRayPlaybackConfig struct {
	// Video settings
	VideoCodec         VideoCodec         `json:"videoCodec"`
	HDRFormat          HDRFormat          `json:"hdrFormat"`
	DolbyVisionProfile DolbyVisionProfile `json:"dolbyVisionProfile,omitempty"`
	Resolution         string             `json:"resolution"`        // 1920x1080, 3840x2160
	BitDepth           int                `json:"bitDepth"`          // 8, 10, 12
	ColorSpace         string             `json:"colorSpace"`        // bt709, bt2020, smpte2084
	MaxCLL             int                `json:"maxCLL,omitempty"`  // Max Content Light Level
	MaxFALL            int                `json:"maxFALL,omitempty"` // Max Frame Average Light Level

	// Audio settings
	AudioCodec        AudioCodec `json:"audioCodec"`
	AudioChannels     int        `json:"audioChannels"`   // 2, 6, 8
	AudioSampleRate   int        `json:"audioSampleRate"` // 48000, 96000
	AudioBitDepth     int        `json:"audioBitDepth"`   // 16, 24
	AtmosEnabled      bool       `json:"atmosEnabled"`
	TrueHDPassthrough bool       `json:"truehdPassthrough"`

	// Playback settings
	HWAccel        string `json:"hwAccel"` // none, cuda, qsv, vaapi
	Deinterlace    bool   `json:"deinterlace"`
	ToneMapping    string `json:"toneMapping"`    // none, hable, mobius, reinhard
	TargetPeakNits int    `json:"targetPeakNits"` // Target peak luminance for tone mapping

	// Subtitle settings
	SubtitleEnabled  bool   `json:"subtitleEnabled"`
	SubtitleBurn     bool   `json:"subtitleBurn"` // Burn subtitles into video
	SubtitleLanguage string `json:"subtitleLanguage"`
}

// DefaultBluRayConfig returns default Blu-ray playback configuration
func DefaultBluRayConfig() BluRayPlaybackConfig {
	return BluRayPlaybackConfig{
		VideoCodec:      VideoHEVC,
		HDRFormat:       HDR10,
		Resolution:      "1920x1080",
		BitDepth:        10,
		ColorSpace:      "bt2020",
		AudioCodec:      AudioEAC3,
		AudioChannels:   6,
		AudioSampleRate: 48000,
		AudioBitDepth:   24,
		HWAccel:         "none",
		Deinterlace:     true,
		ToneMapping:     "hable",
		TargetPeakNits:  100,
		SubtitleEnabled: true,
	}
}

// DolbyAtmosConfig represents Dolby Atmos configuration
type DolbyAtmosConfig struct {
	Enabled          bool   `json:"enabled"`
	BedChannels      int    `json:"bedChannels"` // 7.1.4, 5.1.4, etc.
	ObjectBasedAudio bool   `json:"objectBasedAudio"`
	HeightSpeakers   int    `json:"heightSpeakers"` // Number of height speakers
	Passthrough      bool   `json:"passthrough"`    // Passthrough to receiver
	Downmix          string `json:"downmix"`        // stereo, 5.1, 7.1
}

// DefaultDolbyAtmosConfig returns default Atmos configuration
func DefaultDolbyAtmosConfig() DolbyAtmosConfig {
	return DolbyAtmosConfig{
		Enabled:          true,
		BedChannels:      8, // 7.1
		ObjectBasedAudio: true,
		HeightSpeakers:   4,
		Passthrough:      true,
	}
}

// DolbyVisionConfig represents Dolby Vision configuration
type DolbyVisionConfig struct {
	Profile         DolbyVisionProfile `json:"profile"`
	Enabled         bool               `json:"enabled"`
	L1Metadata      bool               `json:"l1Metadata"`     // Level 1 metadata
	L2Metadata      bool               `json:"l2Metadata"`     // Level 2 metadata (per-scene)
	BackwardCompat  bool               `json:"backwardCompat"` // HDR10 backward compatibility
	ICtCpColorSpace bool               `json:"ictpColorSpace"` // Use ICtCp color space
	MaxLuminance    int                `json:"maxLuminance"`   // nits
	MinLuminance    float64            `json:"minLuminance"`   // nits
}

// DefaultDolbyVisionConfig returns default Dolby Vision configuration
func DefaultDolbyVisionConfig() DolbyVisionConfig {
	return DolbyVisionConfig{
		Profile:        DVProfile8,
		Enabled:        true,
		BackwardCompat: true,
		MaxLuminance:   1000,
		MinLuminance:   0.005,
	}
}

// HDRConfig represents HDR configuration
type HDRConfig struct {
	Format          HDRFormat `json:"format"`
	MaxCLL          int       `json:"maxCLL"`
	MaxFALL         int       `json:"maxFALL"`
	ColorVolume     bool      `json:"colorVolume"`
	DynamicMetadata bool      `json:"dynamicMetadata"`
	ST2084Curve     bool      `json:"st2084Curve"`
}

// PlaybackCapabilities represents device playback capabilities
type PlaybackCapabilities struct {
	// Video capabilities
	MaxResolution        string       `json:"maxResolution"`
	SupportedHDR         []HDRFormat  `json:"supportedHDR"`
	SupportedVideoCodecs []VideoCodec `json:"supportedVideoCodecs"`
	BitDepthSupport      int          `json:"bitDepthSupport"`
	DolbyVisionSupport   bool         `json:"dolbyVisionSupport"`

	// Audio capabilities
	SupportedAudioCodecs []AudioCodec `json:"supportedAudioCodecs"`
	MaxAudioChannels     int          `json:"maxAudioChannels"`
	AtmosSupport         bool         `json:"atmosSupport"`
	TrueHDSupport        bool         `json:"truehdSupport"`

	// Streaming capabilities
	MaxBitrate  int64 `json:"maxBitrate"`
	HLSSupport  bool  `json:"hlsSupport"`
	DASHSupport bool  `json:"dashSupport"`
}

// AnalyzeMediaFile analyzes a media file for Dolby/HDR capabilities
func AnalyzeMediaFile(ffprobePath, filePath string) (*MediaAnalysis, error) {
	if ffprobePath == "" {
		ffprobePath = "ffprobe"
	}

	cmd := exec.Command(ffprobePath,
		"-v", "quiet",
		"-print_format", "json",
		"-show_format",
		"-show_streams",
		filePath,
	)

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ffprobe failed: %w", err)
	}

	return parseMediaAnalysis(output)
}

// MediaAnalysis represents the result of media file analysis
type MediaAnalysis struct {
	FilePath string  `json:"filePath"`
	FileSize int64   `json:"fileSize"`
	Duration float64 `json:"duration"`
	Bitrate  int64   `json:"bitrate"`

	Video     VideoAnalysis      `json:"video"`
	Audio     []AudioAnalysis    `json:"audio"`
	Subtitles []SubtitleAnalysis `json:"subtitles"`

	IsBluRay      bool `json:"isBluRay"`
	Is4K          bool `json:"is4K"`
	IsHDR         bool `json:"isHDR"`
	IsDolbyVision bool `json:"isDolbyVision"`
	IsAtmos       bool `json:"isAtmos"`
}

// VideoAnalysis represents video stream analysis
type VideoAnalysis struct {
	Index          int                `json:"index"`
	Codec          VideoCodec         `json:"codec"`
	Width          int                `json:"width"`
	Height         int                `json:"height"`
	BitDepth       int                `json:"bitDepth"`
	ColorSpace     string             `json:"colorSpace"`
	ColorPrimaries string             `json:"colorPrimaries"`
	ColorTRC       string             `json:"colorTRC"`
	HDRFormat      HDRFormat          `json:"hdrFormat"`
	DolbyVision    bool               `json:"dolbyVision"`
	DVProfile      DolbyVisionProfile `json:"dvProfile"`
	Bitrate        int64              `json:"bitrate"`
	Framerate      float64            `json:"framerate"`
}

// AudioAnalysis represents audio stream analysis
type AudioAnalysis struct {
	Index      int        `json:"index"`
	Codec      AudioCodec `json:"codec"`
	Channels   int        `json:"channels"`
	SampleRate int        `json:"sampleRate"`
	BitDepth   int        `json:"bitDepth"`
	Language   string     `json:"language"`
	IsAtmos    bool       `json:"isAtmos"`
	IsTrueHD   bool       `json:"isTrueHD"`
	Bitrate    int64      `json:"bitrate"`
}

// SubtitleAnalysis represents subtitle stream analysis
type SubtitleAnalysis struct {
	Index    int    `json:"index"`
	Codec    string `json:"codec"`
	Language string `json:"language"`
	Forced   bool   `json:"forced"`
	Default  bool   `json:"default"`
}

// parseMediaAnalysis parses ffprobe JSON output
func parseMediaAnalysis(data []byte) (*MediaAnalysis, error) {
	var result struct {
		Format struct {
			Filename string `json:"filename"`
			Size     string `json:"size"`
			Duration string `json:"duration"`
			BitRate  string `json:"bit_rate"`
		} `json:"format"`
		Streams []struct {
			Index          int    `json:"index"`
			CodecType      string `json:"codec_type"`
			CodecName      string `json:"codec_name"`
			Width          int    `json:"width"`
			Height         int    `json:"height"`
			Channels       int    `json:"channels"`
			SampleRate     string `json:"sample_rate"`
			BitRate        string `json:"bit_rate"`
			Language       string `json:"language"`
			ColorSpace     string `json:"color_space"`
			ColorPrimaries string `json:"color_primaries"`
			ColorTransfer  string `json:"color_transfer"`
			Profile        string `json:"profile"`
			RFrameRate     string `json:"r_frame_rate"`
		} `json:"streams"`
	}

	if err := json.Unmarshal(data, &result); err != nil {
		return nil, err
	}

	analysis := &MediaAnalysis{
		FilePath: result.Format.Filename,
		Video:    VideoAnalysis{},
	}

	// Parse format info
	if size, err := parseInt64(result.Format.Size); err == nil {
		analysis.FileSize = size
	}
	if dur, err := parseFloat64(result.Format.Duration); err == nil {
		analysis.Duration = dur
	}
	if br, err := parseInt64(result.Format.BitRate); err == nil {
		analysis.Bitrate = br
	}

	// Parse streams
	for _, s := range result.Streams {
		switch s.CodecType {
		case "video":
			analysis.Video = VideoAnalysis{
				Index:          s.Index,
				Codec:          VideoCodec(s.CodecName),
				Width:          s.Width,
				Height:         s.Height,
				ColorSpace:     s.ColorSpace,
				ColorPrimaries: s.ColorPrimaries,
				ColorTRC:       s.ColorTransfer,
				BitDepth:       parseBitDepth(s.Profile),
			}

			// Detect HDR
			if s.ColorPrimaries == "bt2020" || s.ColorTransfer == "smpte2084" {
				analysis.IsHDR = true
				analysis.Video.HDRFormat = HDR10
			}

			// Detect Dolby Vision
			if strings.Contains(strings.ToLower(s.Profile), "dolby") ||
				strings.Contains(strings.ToLower(s.CodecName), "dovi") {
				analysis.IsDolbyVision = true
				analysis.Video.DolbyVision = true
			}

			// Detect 4K
			if s.Width >= 3840 || s.Height >= 2160 {
				analysis.Is4K = true
			}

			// Parse framerate
			if s.RFrameRate != "" {
				analysis.Video.Framerate = parseFrameRate(s.RFrameRate)
			}

		case "audio":
			audio := AudioAnalysis{
				Index:    s.Index,
				Codec:    AudioCodec(s.CodecName),
				Channels: s.Channels,
				Language: s.Language,
			}

			if sr, err := parseInt(s.SampleRate); err == nil {
				audio.SampleRate = sr
			}
			if br, err := parseInt64(s.BitRate); err == nil {
				audio.Bitrate = br
			}

			// Detect Dolby Atmos
			if s.CodecName == "truehd" || strings.Contains(strings.ToLower(s.Profile), "atmos") {
				audio.IsTrueHD = true
				if strings.Contains(strings.ToLower(s.Profile), "atmos") {
					audio.IsAtmos = true
					analysis.IsAtmos = true
				}
			}

			analysis.Audio = append(analysis.Audio, audio)

		case "subtitle":
			sub := SubtitleAnalysis{
				Index:    s.Index,
				Codec:    s.CodecName,
				Language: s.Language,
			}
			analysis.Subtitles = append(analysis.Subtitles, sub)
		}
	}

	// Detect Blu-ray structure
	if strings.Contains(strings.ToLower(analysis.FilePath), "bdmv") ||
		strings.Contains(strings.ToLower(analysis.FilePath), "bluray") {
		analysis.IsBluRay = true
	}

	return analysis, nil
}

// GenerateFFmpegArgs generates FFmpeg arguments for Dolby/HDR playback
func GenerateFFmpegArgs(config BluRayPlaybackConfig, inputPath, outputPath string) []string {
	args := []string{"-i", inputPath}

	// Hardware acceleration
	switch strings.ToLower(config.HWAccel) {
	case "cuda", "nvenc":
		args = append(args, "-hwaccel", "cuda", "-hwaccel_output_format", "cuda")
	case "qsv":
		args = append(args, "-hwaccel", "qsv")
	case "vaapi":
		args = append(args, "-hwaccel", "vaapi")
	}

	// Video encoding
	args = append(args, "-c:v")
	switch config.HWAccel {
	case "cuda", "nvenc":
		args = append(args, "hevc_nvenc")
	case "qsv":
		args = append(args, "hevc_qsv")
	default:
		args = append(args, "libx265")
	}

	// HDR/Dolby Vision settings
	if config.HDRFormat != HDRNone {
		args = append(args,
			"-pix_fmt", "yuv420p10le",
			"-color_primaries", config.ColorSpace,
			"-color_trc", "smpte2084",
			"-colorspace", "bt2020nc",
		)

		// Dolby Vision metadata
		if config.HDRFormat == DolbyVision && config.DolbyVisionProfile != "" {
			args = append(args, "-dv_profile", string(config.DolbyVisionProfile))
		}
	}

	// Resolution
	if config.Resolution != "" {
		args = append(args, "-s", config.Resolution)
	}

	// Tone mapping for HDR to SDR
	if config.ToneMapping != "none" && config.HDRFormat != HDRNone {
		filter := fmt.Sprintf("tonemap_xxx=tonemap=%s:peak=%d", config.ToneMapping, config.TargetPeakNits)
		args = append(args, "-vf", filter)
	}

	// Audio settings
	if config.TrueHDPassthrough && config.AudioCodec == AudioTrueHD {
		args = append(args, "-c:a", "copy")
	} else if config.AtmosEnabled {
		args = append(args, "-c:a", "eac3")
		if config.AudioChannels > 0 {
			args = append(args, "-ac", fmt.Sprintf("%d", config.AudioChannels))
		}
	} else {
		args = append(args, "-c:a", string(config.AudioCodec))
	}

	// Output format
	args = append(args, "-f", "matroska", "-y", outputPath)

	return args
}

// GetOptimalPlaybackConfig returns optimal playback config based on file analysis and device capabilities
func GetOptimalPlaybackConfig(analysis *MediaAnalysis, caps *PlaybackCapabilities) BluRayPlaybackConfig {
	config := DefaultBluRayConfig()

	// Match video resolution
	if caps.MaxResolution != "" {
		config.Resolution = caps.MaxResolution
	}
	if analysis.Is4K && containsHDR(caps.SupportedHDR, HDR10) {
		config.Resolution = "3840x2160"
		config.HDRFormat = HDR10
	}

	// Match HDR format
	if analysis.IsHDR {
		for _, hdr := range caps.SupportedHDR {
			if hdr == analysis.Video.HDRFormat {
				config.HDRFormat = analysis.Video.HDRFormat
				break
			}
		}
	}

	// Match audio codec
	if analysis.IsAtmos && caps.AtmosSupport {
		config.AudioCodec = AudioAtmos
		config.AtmosEnabled = true
	} else if analysis.Video.BitDepth > 8 {
		config.BitDepth = analysis.Video.BitDepth
	}

	// Set color space
	if analysis.Video.ColorSpace != "" {
		config.ColorSpace = analysis.Video.ColorSpace
	}

	return config
}

// CreatePlaybackManifest creates a DASH/HLS manifest for adaptive streaming
func CreatePlaybackManifest(analysis *MediaAnalysis, outputDir string, qualities []AdaptiveStream) error {
	// Create master playlist
	masterPath := filepath.Join(outputDir, "master.m3u8")

	var content strings.Builder
	content.WriteString("#EXTM3U\n")
	content.WriteString("#EXT-X-VERSION:6\n")

	for _, q := range qualities {
		bandwidth := parseBitrateToInt(q.Bitrate)
		fmt.Fprintf(&content, "#EXT-X-STREAM-INF:BANDWIDTH=%d,RESOLUTION=%s,CODECS=\"hvc1.2.4.L150.90\",AUDIO=\"audio\"\n",
			bandwidth, q.Resolution)
		fmt.Fprintf(&content, "%s/stream.m3u8\n", q.Quality)
	}

	// Audio group
	content.WriteString("#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID=\"audio\",NAME=\"Audio\",AUTOSELECT=YES,DEFAULT=YES\n")

	return os.WriteFile(masterPath, []byte(content.String()), 0644)
}

// Helper functions

func parseBitDepth(profile string) int {
	profile = strings.ToLower(profile)
	if strings.Contains(profile, "main10") || strings.Contains(profile, "10") {
		return 10
	}
	if strings.Contains(profile, "main12") || strings.Contains(profile, "12") {
		return 12
	}
	return 8
}

func parseFrameRate(framerate string) float64 {
	parts := strings.Split(framerate, "/")
	if len(parts) == 2 {
		num, _ := parseFloat64(parts[0])
		den, _ := parseFloat64(parts[1])
		if den != 0 {
			return num / den
		}
	}
	rate, _ := parseFloat64(framerate)
	return rate
}

func parseInt(s string) (int, error) {
	var result int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}
	return result, nil
}

func parseInt64(s string) (int64, error) {
	var result int64
	for _, c := range s {
		if c >= '0' && c <= '9' {
			result = result*10 + int64(c-'0')
		}
	}
	return result, nil
}

func parseFloat64(s string) (float64, error) {
	var result float64
	var decimal float64 = 1
	var afterDecimal bool

	for _, c := range s {
		if c >= '0' && c <= '9' {
			if afterDecimal {
				decimal *= 10
				result = result + float64(c-'0')/decimal
			} else {
				result = result*10 + float64(c-'0')
			}
		} else if c == '.' {
			afterDecimal = true
		}
	}
	return result, nil
}

func parseBitrateToInt(bitrate string) int {
	bitrate = strings.TrimSpace(bitrate)
	multiplier := 1

	if strings.HasSuffix(bitrate, "M") {
		multiplier = 1000000
		bitrate = strings.TrimSuffix(bitrate, "M")
	} else if strings.HasSuffix(bitrate, "K") || strings.HasSuffix(bitrate, "k") {
		multiplier = 1000
		bitrate = strings.TrimSuffix(bitrate, "K")
		bitrate = strings.TrimSuffix(bitrate, "k")
	}

	var result int
	for _, c := range bitrate {
		if c >= '0' && c <= '9' {
			result = result*10 + int(c-'0')
		}
	}

	return result * multiplier
}

func containsHDR(formats []HDRFormat, target HDRFormat) bool {
	for _, f := range formats {
		if f == target {
			return true
		}
	}
	return false
}
