package aivideostudio

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// ========== 错误常量测试 ==========

func TestErrorSentinels(t *testing.T) {
	assert.EqualError(t, ErrVideoNotFound, "视频文件不存在")
	assert.EqualError(t, ErrTaskNotFound, "任务不存在")
	assert.EqualError(t, ErrTranscodeFailed, "转码失败")
	assert.EqualError(t, ErrUnsupportedCodec, "不支持的编码格式")
	assert.EqualError(t, ErrSceneDetectionFailed, "场景检测失败")
	assert.EqualError(t, ErrThumbnailFailed, "缩略图生成失败")
	assert.EqualError(t, ErrSummaryFailed, "摘要生成失败")
	assert.EqualError(t, ErrEnhancementFailed, "视频增强失败")
	assert.EqualError(t, ErrSubtitleFailed, "字幕生成失败")
	assert.EqualError(t, ErrInvalidPath, "无效路径")
	assert.EqualError(t, ErrBatchFailed, "批量任务失败")
	assert.EqualError(t, ErrUnsupportedFormat, "不支持的视频格式")
	assert.EqualError(t, ErrTaskAlreadyRunning, "任务已在运行中")
	assert.EqualError(t, ErrInsufficientSpace, "磁盘空间不足")
}

// ========== 编码格式测试 ==========

func TestVideoCodecConstants(t *testing.T) {
	assert.Equal(t, VideoCodec("h264"), CodecH264)
	assert.Equal(t, VideoCodec("h265"), CodecH265)
	assert.Equal(t, VideoCodec("av1"), CodecAV1)
	assert.Equal(t, VideoCodec("vp9"), CodecVP9)
}

// ========== 任务状态测试 ==========

func TestTaskStatusConstants(t *testing.T) {
	assert.Equal(t, TaskStatus("pending"), TaskStatusPending)
	assert.Equal(t, TaskStatus("running"), TaskStatusRunning)
	assert.Equal(t, TaskStatus("completed"), TaskStatusCompleted)
	assert.Equal(t, TaskStatus("failed"), TaskStatusFailed)
	assert.Equal(t, TaskStatus("cancelled"), TaskStatusCancelled)
	assert.Equal(t, TaskStatus("paused"), TaskStatusPaused)
}

// ========== 任务类型测试 ==========

func TestTaskTypeConstants(t *testing.T) {
	assert.Equal(t, TaskType("transcode"), TaskTypeTranscode)
	assert.Equal(t, TaskType("scene_detection"), TaskTypeSceneDetection)
	assert.Equal(t, TaskType("thumbnail"), TaskTypeThumbnail)
	assert.Equal(t, TaskType("preview"), TaskTypePreview)
	assert.Equal(t, TaskType("summary"), TaskTypeSummary)
	assert.Equal(t, TaskType("enhancement"), TaskTypeEnhancement)
	assert.Equal(t, TaskType("subtitle"), TaskTypeSubtitle)
	assert.Equal(t, TaskType("metadata"), TaskTypeMetadata)
	assert.Equal(t, TaskType("batch_convert"), TaskTypeBatchConvert)
}

// ========== 结构体初始化测试 ==========

func TestVideoInfoCreation(t *testing.T) {
	now := time.Now()
	vi := VideoInfo{
		ID:           "vid-001",
		FileName:     "test.mp4",
		FilePath:     "/videos/test.mp4",
		FileSize:     1024000,
		Duration:     120.5,
		Width:        1920,
		Height:       1080,
		Codec:        CodecH264,
		Bitrate:      5000000,
		FrameRate:    30.0,
		Format:       "mp4",
		HasAudio:     true,
		AudioCodec:   "aac",
		AudioBitrate: 128000,
		CreatedAt:    now,
		UpdatedAt:    now,
	}

	assert.Equal(t, "vid-001", vi.ID)
	assert.Equal(t, "test.mp4", vi.FileName)
	assert.Equal(t, int64(1024000), vi.FileSize)
	assert.Equal(t, 1920, vi.Width)
	assert.Equal(t, 1080, vi.Height)
	assert.Equal(t, CodecH264, vi.Codec)
	assert.True(t, vi.HasAudio)
	assert.Equal(t, "aac", vi.AudioCodec)
	assert.Empty(t, vi.Subtitles)
}

func TestVideoTaskCreation(t *testing.T) {
	now := time.Now()
	task := VideoTask{
		ID:         "task-001",
		Type:       TaskTypeTranscode,
		Status:     TaskStatusPending,
		VideoID:    "vid-001",
		InputPath:  "/videos/input.mp4",
		OutputPath: "/videos/output.mkv",
		Progress:   0,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	assert.Equal(t, "task-001", task.ID)
	assert.Equal(t, TaskTypeTranscode, task.Type)
	assert.Equal(t, TaskStatusPending, task.Status)
	assert.Equal(t, float64(0), task.Progress)
	assert.Empty(t, task.ErrorMessage)
	assert.Nil(t, task.StartedAt)
	assert.Nil(t, task.CompletedAt)
}

func TestTranscodeProfileCreation(t *testing.T) {
	profile := TranscodeProfile{
		Name:             "h265-1080p",
		TargetCodec:      CodecH265,
		TargetResolution: Resolution1080p,
		TargetBitrate:    4000000,
		Quality:          80,
		TwoPass:          true,
		HardwareAccel:    true,
		PreserveAudio:    true,
	}

	assert.Equal(t, "h265-1080p", profile.Name)
	assert.Equal(t, CodecH265, profile.TargetCodec)
	assert.Equal(t, Resolution1080p, profile.TargetResolution)
	assert.True(t, profile.TwoPass)
	assert.True(t, profile.HardwareAccel)
	assert.Equal(t, 80, profile.Quality)
}

func TestSceneInfoCreation(t *testing.T) {
	scene := SceneInfo{
		SceneIndex:    1,
		StartTime:     0.0,
		EndTime:       30.5,
		Duration:      30.5,
		Description:   "开场白",
		Tags:          []string{"intro", "outdoor"},
		KeyFrameTime:  15.0,
		ThumbnailPath: "/thumbs/scene1.jpg",
	}

	assert.Equal(t, 1, scene.SceneIndex)
	assert.Equal(t, 30.5, scene.Duration)
	assert.Len(t, scene.Tags, 2)
	assert.Contains(t, scene.Tags, "intro")
	assert.Equal(t, 15.0, scene.KeyFrameTime)
}

func TestSubtitleTrackCreation(t *testing.T) {
	sub := SubtitleTrack{
		ID:       "sub-001",
		VideoID:  "vid-001",
		Language: "zh",
		Format:   "srt",
		FilePath: "/subs/sub-001.srt",
		Entries: []SubtitleEntry{
			{
				Index:      1,
				StartTime:  0,
				EndTime:    3000,
				Text:       "你好世界",
				Confidence: 0.95,
			},
			{
				Index:     2,
				StartTime: 3000,
				EndTime:   6000,
				Text:      "欢迎来到AI视频工作室",
				Speaker:   "narrator",
			},
		},
		GeneratedBy: "ai",
		Accuracy:    92.5,
		CreatedAt:   time.Now(),
	}

	assert.Equal(t, "sub-001", sub.ID)
	assert.Equal(t, "ai", sub.GeneratedBy)
	assert.Len(t, sub.Entries, 2)
	assert.Equal(t, "你好世界", sub.Entries[0].Text)
	assert.Equal(t, "narrator", sub.Entries[1].Speaker)
	assert.Equal(t, 92.5, sub.Accuracy)
}

func TestVideoSummaryCreation(t *testing.T) {
	summary := VideoSummary{
		VideoID: "vid-001",
		Title:   "风景视频",
		Summary: "一段美丽的自然风景视频",
		KeyMoments: []KeyMoment{
			{Time: 10.0, Description: "日出场景", Importance: 0.9},
			{Time: 60.0, Description: "山峰全景", Importance: 0.8},
		},
		Topics:      []string{"自然", "风景", "旅游"},
		Language:    "zh",
		GeneratedAt: time.Now(),
	}

	assert.Equal(t, "vid-001", summary.VideoID)
	assert.Len(t, summary.KeyMoments, 2)
	assert.Equal(t, 0.9, summary.KeyMoments[0].Importance)
	assert.Contains(t, summary.Topics, "自然")
}

func TestBatchConvertRequestCreation(t *testing.T) {
	req := BatchConvertRequest{
		InputPaths: []string{
			"/videos/a.avi",
			"/videos/b.avi",
			"/videos/c.avi",
		},
		OutputDir:     "/videos/output/",
		NamingPattern: "{name}.{ext}",
		Overwrite:     false,
		Profile: TranscodeProfile{
			TargetCodec:      CodecH265,
			TargetResolution: Resolution720p,
			Quality:          70,
		},
	}

	assert.Len(t, req.InputPaths, 3)
	assert.Equal(t, "/videos/output/", req.OutputDir)
	assert.False(t, req.Overwrite)
	assert.Equal(t, CodecH265, req.Profile.TargetCodec)
}

func TestMetadataAnalysisCreation(t *testing.T) {
	analysis := MetadataAnalysis{
		VideoID:          "vid-001",
		QualityScore:     85.0,
		CompressionRatio: 0.65,
		BitrateAnalysis: BitrateAnalysis{
			AverageBitrate: 5000000,
			PeakBitrate:    8000000,
			MinBitrate:     2000000,
			IsVBR:          true,
		},
		ResolutionAnalysis: ResolutionAnalysis{
			NativeResolution:    "1920x1080",
			EffectiveResolution: "1920x1080",
			IsUpscaled:          false,
			PixelFormat:         "yuv420p",
			ColorSpace:          "bt709",
		},
		AudioAnalysis: AudioAnalysis{
			Channels:     2,
			SampleRate:   48000,
			BitDepth:     16,
			Loudness:     -14.0,
			DynamicRange: 20.0,
		},
		Recommendations: []string{"建议降低码率以节省空间"},
		AnalyzedAt:      time.Now(),
	}

	assert.Equal(t, "vid-001", analysis.VideoID)
	assert.Equal(t, 85.0, analysis.QualityScore)
	assert.True(t, analysis.BitrateAnalysis.IsVBR)
	assert.False(t, analysis.ResolutionAnalysis.IsUpscaled)
	assert.Len(t, analysis.Recommendations, 1)
}

func TestThumbnailConfigCreation(t *testing.T) {
	cfg := ThumbnailConfig{
		Width:       320,
		Height:      180,
		Quality:     80,
		Count:       10,
		Interval:    0,
		Format:      "jpg",
		SpriteSheet: true,
	}

	assert.Equal(t, 320, cfg.Width)
	assert.Equal(t, 180, cfg.Height)
	assert.Equal(t, 10, cfg.Count)
	assert.True(t, cfg.SpriteSheet)
}

func TestResolutionPresets(t *testing.T) {
	assert.Equal(t, ResolutionPreset("4k"), Resolution4K)
	assert.Equal(t, ResolutionPreset("1080p"), Resolution1080p)
	assert.Equal(t, ResolutionPreset("720p"), Resolution720p)
	assert.Equal(t, ResolutionPreset("480p"), Resolution480p)
}

func TestEnhancementTypes(t *testing.T) {
	assert.Equal(t, EnhancementType("super_resolution"), EnhancementSuperResolution)
	assert.Equal(t, EnhancementType("denoise"), EnhancementDenoise)
	assert.Equal(t, EnhancementType("sharpen"), EnhancementSharpen)
	assert.Equal(t, EnhancementType("color_correct"), EnhancementColorCorrect)
	assert.Equal(t, EnhancementType("stabilize"), EnhancementStabilize)
	assert.Equal(t, EnhancementType("hdr"), EnhancementHDR)
}
