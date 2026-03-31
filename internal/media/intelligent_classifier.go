// Package media provides intelligent media classification
package media

import (
	"path/filepath"
	"regexp"
	"strings"
)

// IntelligentClassifier 智能分类器 - 自动识别媒体类型和质量
type IntelligentClassifier struct {
	categoryPatterns []*ClassificationPattern
	qualityPatterns  []*ClassificationPattern
	hdrPatterns      []*ClassificationPattern
	audioPatterns    []*ClassificationPattern
}

// ClassificationPattern 分类模式
type ClassificationPattern struct {
	Pattern     *regexp.Regexp
	Category    interface{}
	Confidence  float64
	Tags        []string
}

// NewIntelligentClassifier 创建智能分类器
func NewIntelligentClassifier() *IntelligentClassifier {
	return &IntelligentClassifier{
		categoryPatterns: initCategoryPatterns(),
		qualityPatterns:  initQualityPatterns(),
		hdrPatterns:      initHDRPatterns(),
		audioPatterns:    initAudioPatterns(),
	}
}

// Classify 分类媒体文件
func (c *IntelligentClassifier) Classify(filename string, metadata *MediaMetadata, videoInfo *VideoInfo) *IntelligentClassification {
	result := &IntelligentClassification{
		Category:     CategoryOther,
		Quality:      QualityHD,
		HDR:          HDRNone,
		Audio:        AudioNone,
		Confidence:   0.5,
		DetectedTags: []string{},
	}

	// Step 1: 根据元数据分类（最高优先级）
	if metadata != nil {
		c.classifyFromMetadata(metadata, result)
	}

	// Step 2: 根据文件名分类
	c.classifyFromFilename(filename, result)

	// Step 3: 根据视频信息分类
	if videoInfo != nil {
		c.classifyFromVideoInfo(videoInfo, result)
	}

	// Step 4: 综合判断置信度
	c.calculateConfidence(result)

	return result
}

// classifyFromMetadata 从元数据分类
func (c *IntelligentClassifier) classifyFromMetadata(meta *MediaMetadata, result *IntelligentClassification) {
	// 类型分类
	switch meta.Type {
	case MediaTypeMovie:
		// 检查是否为动画
		for _, genre := range meta.Genres {
			if genre == "动画" || genre == "Animation" || genre == "Anime" {
				result.Category = CategoryAnimation
				result.DetectedTags = append(result.DetectedTags, "动画")
				result.Confidence = 0.9
				return
			}
			if genre == "纪录片" || genre == "Documentary" {
				result.Category = CategoryDocumentary
				result.DetectedTags = append(result.DetectedTags, "纪录片")
				result.Confidence = 0.9
				return
			}
			if genre == "音乐" || genre == "Music" || genre == "Musical" {
				result.Category = CategoryMusic
				result.DetectedTags = append(result.DetectedTags, "音乐")
				result.Confidence = 0.9
				return
			}
		}
		result.Category = CategoryMovie

	case MediaTypeTVShow:
		result.Category = CategoryTVShow
	}

	// 标语/关键词分类
	if meta.Tagline != "" || meta.Overview != "" {
		text := meta.Tagline + " " + meta.Overview
		if containsAny(text, []string{"综艺", "Variety", "真人秀", "Reality", "选秀"}) {
			result.Category = CategoryVariety
			result.DetectedTags = append(result.DetectedTags, "综艺")
		}
		if containsAny(text, []string{"体育", "Sport", "足球", "篮球", "奥运会", "世界杯"}) {
			result.Category = CategorySport
			result.DetectedTags = append(result.DetectedTags, "体育")
		}
	}
}

// classifyFromFilename 从文件名分类
func (c *IntelligentClassifier) classifyFromFilename(filename string, result *IntelligentClassification) {
	name := strings.ToLower(filename)

	// 应用分类模式
	for _, pattern := range c.categoryPatterns {
		if pattern.Pattern.MatchString(name) {
			if cat, ok := pattern.Category.(MediaCategory); ok {
				result.Category = cat
				result.Confidence = pattern.Confidence
				result.DetectedTags = append(result.DetectedTags, pattern.Tags...)
			}
		}
	}

	// 质量分类
	for _, pattern := range c.qualityPatterns {
		if pattern.Pattern.MatchString(name) {
			if qual, ok := pattern.Category.(VideoQuality); ok {
				result.Quality = qual
				result.DetectedTags = append(result.DetectedTags, pattern.Tags...)
			}
		}
	}

	// HDR分类
	for _, pattern := range c.hdrPatterns {
		if pattern.Pattern.MatchString(name) {
			if hdr, ok := pattern.Category.(HDRType); ok {
				result.HDR = hdr
				result.DetectedTags = append(result.DetectedTags, pattern.Tags...)
			}
		}
	}

	// 音频分类
	for _, pattern := range c.audioPatterns {
		if pattern.Pattern.MatchString(name) {
			if audio, ok := pattern.Category.(AudioType); ok {
				result.Audio = audio
				result.DetectedTags = append(result.DetectedTags, pattern.Tags...)
			}
		}
	}

	// 特殊识别
	// 动漫/动画
	if containsAny(name, []string{"动漫", "anime", "动画", "ova", "oad", "番剧", "日漫", "国漫"}) {
		result.Category = CategoryAnimation
		result.DetectedTags = append(result.DetectedTags, "动画")
	}

	// 纪录片
	if containsAny(name, []string{"纪录片", "documentary", "探索频道", "national geographic", "bbc earth", "discovery"}) {
		result.Category = CategoryDocumentary
		result.DetectedTags = append(result.DetectedTags, "纪录片")
	}

	// 综艺
	if containsAny(name, []string{"综艺", "真人秀", "选秀", "脱口秀", "talk show", "综艺大观", "快乐大本营", "奔跑吧"}) {
		result.Category = CategoryVariety
		result.DetectedTags = append(result.DetectedTags, "综艺")
	}

	// 体育
	if containsAny(name, []string{"体育", "sports", "足球", "football", "篮球", "basketball", "nba", "cba", "世界杯", "奥运会", "欧冠", "英超", "中超"}) {
		result.Category = CategorySport
		result.DetectedTags = append(result.DetectedTags, "体育")
	}

	// 音乐/MV
	if containsAny(name, []string{"mv", "music video", "演唱会", "concert", "mv.", "music.", "歌曲"}) {
		result.Category = CategoryMusic
		result.DetectedTags = append(result.DetectedTags, "音乐")
	}

	// 教育/课程
	if containsAny(name, []string{"教程", "tutorial", "课程", "course", "lecture", "讲座", "教学", "educational", "learn", "学习"}) {
		result.Category = CategoryEducational
		result.DetectedTags = append(result.DetectedTags, "教育")
	}
}

// classifyFromVideoInfo 从视频信息分类
func (c *IntelligentClassifier) classifyFromVideoInfo(info *VideoInfo, result *IntelligentClassification) {
	// 分辨率判断质量
	if info.Height >= 4320 {
		result.Quality = Quality8K
		result.Resolution = "7680x4320"
	} else if info.Height >= 2160 {
		result.Quality = QualityUHD
		result.Resolution = "3840x2160"
	} else if info.Height >= 1080 {
		result.Quality = QualityFHD
		result.Resolution = "1920x1080"
	} else if info.Height >= 720 {
		result.Quality = QualityHD
		result.Resolution = "1280x720"
	} else {
		result.Quality = QualitySD
		result.Resolution = "480x360"
	}

	// 比特率辅助判断
	if info.Bitrate > 20000000 && result.Quality == QualityFHD {
		// 高比特率1080p可能是蓝光源
		result.Quality = QualityBluRay
	}

	// 帧率
	result.FrameRate = info.Framerate

	// 编码判断HDR
	// HEVC/H.265常见于HDR内容
	if info.VideoCodec == "hevc" || info.VideoCodec == "h265" {
		// 需要进一步分析是否为HDR
		// 这里简化处理，实际需要读取视频流信息
	}

	// 音频编码
	switch strings.ToLower(info.AudioCodec) {
	case "eac3":
		result.Audio = AudioEAC3
	case "ac3":
		result.Audio = AudioAC3
	case "dts":
		result.Audio = AudioDTS
	case "truehd":
		result.Audio = AudioTrueHD
	case "flac":
		result.Audio = AudioFLAC
	case "aac":
		result.Audio = AudioAAC
	}
}

// calculateConfidence 计算最终置信度
func (c *IntelligentClassifier) calculateConfidence(result *IntelligentClassification) {
	// 多特征匹配则置信度更高
	tagCount := len(result.DetectedTags)
	if tagCount >= 3 {
		result.Confidence = 0.95
	} else if tagCount >= 2 {
		result.Confidence = 0.85
	} else if tagCount >= 1 {
		result.Confidence = 0.7
	}
}

// ====== 模式初始化 ======

func initCategoryPatterns() []*ClassificationPattern {
	return []*ClassificationPattern{
		// 动画
		{regexp.MustCompile(`(?i)\b(anime|动漫|动画|ova|oad|番剧)\b`), CategoryAnimation, 0.9, []string{"动画"}},
		{regexp.MustCompile(`(?i)\b(s\d+e\d+|ep\d+|episode)\b`), CategoryTVShow, 0.8, []string{"电视剧"}},
		{regexp.MustCompile(`(?i)\b(完整版|全集|连载|更新至)\b`), CategoryTVShow, 0.75, []string{"电视剧"}},
		// 纪录片
		{regexp.MustCompile(`(?i)\b(documentary|纪录片|探索|discovery)\b`), CategoryDocumentary, 0.9, []string{"纪录片"}},
		// 综艺
		{regexp.MustCompile(`(?i)\b(综艺|真人秀|脱口秀|variety)\b`), CategoryVariety, 0.9, []string{"综艺"}},
		// 音乐
		{regexp.MustCompile(`(?i)\b(mv|演唱会|concert|music)\b`), CategoryMusic, 0.9, []string{"音乐"}},
		// 体育
		{regexp.MustCompile(`(?i)\b(体育|sport|足球|篮球|nba|世界杯|奥运会)\b`), CategorySport, 0.9, []string{"体育"}},
	}
}

func initQualityPatterns() []*ClassificationPattern {
	return []*ClassificationPattern{
		// 8K
		{regexp.MustCompile(`(?i)\b(8k|4320p|uhd8k)\b`), Quality8K, 0.95, []string{"8K"}},
		// 4K
		{regexp.MustCompile(`(?i)\b(4k|2160p|uhd)\b`), QualityUHD, 0.95, []string{"4K"}},
		// 1080p
		{regexp.MustCompile(`(?i)\b(1080p|fhd|fullhd)\b`), QualityFHD, 0.95, []string{"1080p"}},
		// 720p
		{regexp.MustCompile(`(?i)\b(720p|hd)\b`), QualityHD, 0.9, []string{"720p"}},
		// 480p
		{regexp.MustCompile(`(?i)\b(480p|sd)\b`), QualitySD, 0.9, []string{"480p"}},
		// 蓝光
		{regexp.MustCompile(`(?i)\b(bluray|blu-ray|bd|bdrip)\b`), QualityBluRay, 0.9, []string{"蓝光"}},
		// Remux
		{regexp.MustCompile(`(?i)\bremux\b`), QualityRemux, 0.95, []string{"Remux"}},
		// WEB-DL
		{regexp.MustCompile(`(?i)\bweb-dl|webdl\b`), QualityWEBDL, 0.9, []string{"WEB-DL"}},
		// HDTV
		{regexp.MustCompile(`(?i)\bhdtv\b`), QualityHDTV, 0.85, []string{"HDTV"}},
	}
}

func initHDRPatterns() []*ClassificationPattern {
	return []*ClassificationPattern{
		// Dolby Vision
		{regexp.MustCompile(`(?i)\b(dolbyvision|dv|dolby.vision)\b`), DolbyVision, 0.95, []string{"Dolby Vision"}},
		// HDR10+
		{regexp.MustCompile(`(?i)\b(hdr10plus|hdr10+|hdr.plus)\b`), HDR10Plus, 0.95, []string{"HDR10+"}},
		// HDR10
		{regexp.MustCompile(`(?i)\b(hdr10|hdr)\b`), HDR10, 0.9, []string{"HDR10"}},
		// HLG
		{regexp.MustCompile(`(?i)\b(hlg|hybrid.log.gamma)\b`), HLG, 0.9, []string{"HLG"}},
	}
}

func initAudioPatterns() []*ClassificationPattern {
	return []*ClassificationPattern{
		// Atmos
		{regexp.MustCompile(`(?i)\b(atmos|dolby.atmos)\b`), AudioAtmos, 0.95, []string{"Dolby Atmos"}},
		// TrueHD
		{regexp.MustCompile(`(?i)\b(truehd|dolby.truehd)\b`), AudioTrueHD, 0.9, []string{"TrueHD"}},
		// DTS-HD MA
		{regexp.MustCompile(`(?i)\b(dts-hd.ma|dtshdma|dts.hd.ma)\b`), AudioDTSHDMA, 0.95, []string{"DTS-HD MA"}},
		// DTS-HD
		{regexp.MustCompile(`(?i)\b(dts-hd|dtshd)\b`), AudioDTSHD, 0.9, []string{"DTS-HD"}},
		// DTS
		{regexp.MustCompile(`(?i)\b(dts|dts:x)\b`), AudioDTS, 0.85, []string{"DTS"}},
		// EAC3/DDP
		{regexp.MustCompile(`(?i)\b(eac3|ddp|dd.plus|dolby.digital.plus)\b`), AudioEAC3, 0.9, []string{"EAC3"}},
		// AC3/DD
		{regexp.MustCompile(`(?i)\b(ac3|dd|dolby.digital)\b`), AudioAC3, 0.85, []string{"AC3"}},
		// FLAC
		{regexp.MustCompile(`(?i)\bflac\b`), AudioFLAC, 0.9, []string{"FLAC"}},
		// AAC
		{regexp.MustCompile(`(?i)\baac\b`), AudioAAC, 0.8, []string{"AAC"}},
	}
}

// ====== 辅助函数 ======

func containsAny(s string, keywords []string) bool {
	for _, kw := range keywords {
		if strings.Contains(s, strings.ToLower(kw)) {
			return true
		}
	}
	return false
}

// DetectCategoryFromFile 从文件检测分类
func (c *IntelligentClassifier) DetectCategoryFromFile(filePath string) MediaCategory {
	filename := filepath.Base(filePath)
	result := c.Classify(filename, nil, nil)
	return result.Category
}

// DetectQualityFromFile 从文件检测质量
func (c *IntelligentClassifier) DetectQualityFromFile(filePath string, videoInfo *VideoInfo) VideoQuality {
	filename := filepath.Base(filePath)
	result := c.Classify(filename, nil, videoInfo)
	return result.Quality
}

// IsHDRFile 检测是否为HDR文件
func (c *IntelligentClassifier) IsHDRFile(filePath string) bool {
	filename := filepath.Base(filePath)
	for _, pattern := range c.hdrPatterns {
		if pattern.Pattern.MatchString(strings.ToLower(filename)) {
			return true
		}
	}
	return false
}

// IsHighQualityAudio 检测是否为高质量音频
func (c *IntelligentClassifier) IsHighQualityAudio(filePath string) bool {
	filename := filepath.Base(filePath)
	name := strings.ToLower(filename)
	highQualityAudio := []AudioType{AudioAtmos, AudioTrueHD, AudioDTSHDMA, AudioDTSHD, AudioFLAC}
	for _, pattern := range c.audioPatterns {
		if pattern.Pattern.MatchString(name) {
			if audio, ok := pattern.Category.(AudioType); ok {
				for _, hqa := range highQualityAudio {
					if audio == hqa {
						return true
					}
				}
			}
		}
	}
	return false
}