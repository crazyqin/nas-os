package media

import (
	"testing"
	"time"
)

func TestDefaultTranscodeConfig(t *testing.T) {
	config := DefaultTranscodeConfig()

	if config.VideoCodec != "libx264" {
		t.Errorf("VideoCodec = %s, want libx264", config.VideoCodec)
	}

	if config.AudioCodec != "aac" {
		t.Errorf("AudioCodec = %s, want aac", config.AudioCodec)
	}

	if config.VideoBitrate != "2M" {
		t.Errorf("VideoBitrate = %s, want 2M", config.VideoBitrate)
	}

	if config.OutputFormat != "mp4" {
		t.Errorf("OutputFormat = %s, want mp4", config.OutputFormat)
	}

	if config.CRF != 23 {
		t.Errorf("CRF = %d, want 23", config.CRF)
	}
}

func TestNewTranscoder(t *testing.T) {
	transcoder := NewTranscoder("/usr/bin/ffmpeg", "/usr/bin/ffprobe", 4)
	if transcoder == nil {
		t.Fatal("NewTranscoder returned nil")
	}

	if transcoder.ffmpegPath != "/usr/bin/ffmpeg" {
		t.Errorf("ffmpegPath = %s, want /usr/bin/ffmpeg", transcoder.ffmpegPath)
	}

	if transcoder.maxJobs != 4 {
		t.Errorf("maxJobs = %d, want 4", transcoder.maxJobs)
	}
}

func TestNewTranscoder_Defaults(t *testing.T) {
	transcoder := NewTranscoder("", "", 0)

	if transcoder.ffmpegPath != "ffmpeg" {
		t.Errorf("Default ffmpegPath = %s, want ffmpeg", transcoder.ffmpegPath)
	}

	if transcoder.ffprobePath != "ffprobe" {
		t.Errorf("Default ffprobePath = %s, want ffprobe", transcoder.ffprobePath)
	}

	if transcoder.maxJobs != 2 {
		t.Errorf("Default maxJobs = %d, want 2", transcoder.maxJobs)
	}
}

func TestTranscodeConfig_Fields(t *testing.T) {
	config := TranscodeConfig{
		VideoCodec:   "libx265",
		AudioCodec:   "opus",
		VideoBitrate: "5M",
		AudioBitrate: "192k",
		Resolution:   "1920x1080",
		Framerate:    60,
		OutputFormat: "mkv",
		Preset:       "slow",
		CRF:          18,
		HWAccel:      "cuda",
		CopyVideo:    false,
		CopyAudio:    false,
	}

	if config.VideoCodec != "libx265" {
		t.Errorf("VideoCodec = %s, want libx265", config.VideoCodec)
	}

	if config.Framerate != 60 {
		t.Errorf("Framerate = %d, want 60", config.Framerate)
	}

	if config.HWAccel != "cuda" {
		t.Errorf("HWAccel = %s, want cuda", config.HWAccel)
	}
}

func TestTranscodeJob_Fields(t *testing.T) {
	now := time.Now()
	job := &TranscodeJob{
		ID:           "job_123",
		InputPath:    "/input/video.mp4",
		OutputPath:   "/output/video.mp4",
		Status:       "pending",
		Progress:     0,
		CurrentFrame: 0,
		TotalFrames:  1000,
		Speed:        "1.5x",
		StartTime:    &now,
	}

	if job.ID != "job_123" {
		t.Errorf("ID = %s, want job_123", job.ID)
	}

	if job.Status != "pending" {
		t.Errorf("Status = %s, want pending", job.Status)
	}
}

func TestTranscoder_ListJobs(t *testing.T) {
	transcoder := NewTranscoder("ffmpeg", "ffprobe", 2)

	jobs := transcoder.ListJobs()
	if len(jobs) != 0 {
		t.Errorf("ListJobs should return empty slice initially, got %d", len(jobs))
	}
}

func TestTranscoder_GetJob_NonExistent(t *testing.T) {
	transcoder := NewTranscoder("ffmpeg", "ffprobe", 2)

	job := transcoder.GetJob("nonexistent")
	if job != nil {
		t.Error("GetJob should return nil for non-existent job")
	}
}

func TestTranscoder_DeleteJob_NonExistent(t *testing.T) {
	transcoder := NewTranscoder("ffmpeg", "ffprobe", 2)

	err := transcoder.DeleteJob("nonexistent")
	if err == nil {
		t.Error("DeleteJob should return error for non-existent job")
	}
}

func TestTranscoder_CancelJob_NonExistent(t *testing.T) {
	transcoder := NewTranscoder("ffmpeg", "ffprobe", 2)

	err := transcoder.CancelJob("nonexistent")
	if err == nil {
		t.Error("CancelJob should return error for non-existent job")
	}
}

func TestTranscoder_CreateJob_FileNotFound(t *testing.T) {
	transcoder := NewTranscoder("ffmpeg", "ffprobe", 2)
	config := DefaultTranscodeConfig()

	_, err := transcoder.CreateJob("/nonexistent/input.mp4", "/tmp/output.mp4", config)
	if err == nil {
		t.Error("CreateJob should return error for non-existent input file")
	}
}

func TestTranscoder_BuildFFmpegArgs(t *testing.T) {
	transcoder := NewTranscoder("ffmpeg", "ffprobe", 2)

	job := &TranscodeJob{
		InputPath:  "/input.mp4",
		OutputPath: "/output.mp4",
		Config: TranscodeConfig{
			VideoCodec:   "libx264",
			AudioCodec:   "aac",
			VideoBitrate: "2M",
			AudioBitrate: "128k",
			Resolution:   "1280x720",
			Preset:       "fast",
			CRF:          23,
			OutputFormat: "mp4",
		},
	}

	args := transcoder.buildFFmpegArgs(job)

	// Check essential arguments
	hasInput := false
	hasOutput := false
	for _, arg := range args {
		if arg == "/input.mp4" {
			hasInput = true
		}
		if arg == "/output.mp4" {
			hasOutput = true
		}
	}

	if !hasInput {
		t.Error("FFmpeg args missing input file")
	}

	if !hasOutput {
		t.Error("FFmpeg args missing output file")
	}
}

func TestTranscoder_BuildFFmpegArgs_HWAccel(t *testing.T) {
	transcoder := NewTranscoder("ffmpeg", "ffprobe", 2)

	tests := []struct {
		hwaccel string
		wantArg string
	}{
		{"cuda", "cuda"},
		{"nvenc", "cuda"},
		{"qsv", "qsv"},
		{"vaapi", "vaapi"},
		{"none", ""},
	}

	for _, tt := range tests {
		job := &TranscodeJob{
			InputPath:  "/input.mp4",
			OutputPath: "/output.mp4",
			Config: TranscodeConfig{
				HWAccel: tt.hwaccel,
			},
		}

		args := transcoder.buildFFmpegArgs(job)

		found := false
		for i, arg := range args {
			if arg == "-hwaccel" && i+1 < len(args) && args[i+1] == tt.wantArg {
				found = true
				break
			}
		}

		if tt.wantArg != "" && !found {
			t.Errorf("HWAccel %s: expected -hwaccel %s in args", tt.hwaccel, tt.wantArg)
		}
	}
}

func TestTranscoder_BuildFFmpegArgs_CopyStreams(t *testing.T) {
	transcoder := NewTranscoder("ffmpeg", "ffprobe", 2)

	job := &TranscodeJob{
		InputPath:  "/input.mp4",
		OutputPath: "/output.mp4",
		Config: TranscodeConfig{
			CopyVideo: true,
			CopyAudio: true,
		},
	}

	args := transcoder.buildFFmpegArgs(job)

	// Check for copy codecs
	hasCopyVideo := false
	hasCopyAudio := false
	for i, arg := range args {
		if arg == "-c:v" && i+1 < len(args) && args[i+1] == "copy" {
			hasCopyVideo = true
		}
		if arg == "-c:a" && i+1 < len(args) && args[i+1] == "copy" {
			hasCopyAudio = true
		}
	}

	if !hasCopyVideo {
		t.Error("CopyVideo=true should result in -c:v copy")
	}

	if !hasCopyAudio {
		t.Error("CopyAudio=true should result in -c:a copy")
	}
}

func TestVideoInfo_Fields(t *testing.T) {
	info := &VideoInfo{
		Duration:   120.5,
		Width:      1920,
		Height:     1080,
		Framerate:  30.0,
		VideoCodec: "h264",
		AudioCodec: "aac",
		Bitrate:    5000000,
		Size:       75_000_000,
	}

	if info.Duration != 120.5 {
		t.Errorf("Duration = %f, want 120.5", info.Duration)
	}

	if info.Width != 1920 {
		t.Errorf("Width = %d, want 1920", info.Width)
	}

	if info.VideoCodec != "h264" {
		t.Errorf("VideoCodec = %s, want h264", info.VideoCodec)
	}
}

func TestStreamInfo_Fields(t *testing.T) {
	stream := StreamInfo{
		Index:     0,
		Type:      "video",
		Codec:     "h264",
		Width:     1920,
		Height:    1080,
		Duration:  "120.5",
		BitRate:   "5000000",
		Language:  "eng",
		FrameRate: "30/1",
	}

	if stream.Index != 0 {
		t.Errorf("Index = %d, want 0", stream.Index)
	}

	if stream.Type != "video" {
		t.Errorf("Type = %s, want video", stream.Type)
	}
}

func TestFormatInfo_Fields(t *testing.T) {
	format := FormatInfo{
		Filename:   "test.mp4",
		FormatName: "mov,mp4,m4a,3gp,3g2,mj2",
		Duration:   "120.500000",
		BitRate:    "5000000",
		Size:       "75000000",
	}

	if format.Filename != "test.mp4" {
		t.Errorf("Filename = %s, want test.mp4", format.Filename)
	}
}

func TestParseVideoInfo(t *testing.T) {
	jsonData := `{
		"format": {
			"filename": "test.mp4",
			"format_name": "mov",
			"duration": "120.5",
			"bit_rate": "5000000",
			"size": "75000000"
		},
		"streams": [
			{
				"index": 0,
				"codec_type": "video",
				"codec_name": "h264",
				"width": 1920,
				"height": 1080,
				"r_frame_rate": "30/1"
			},
			{
				"index": 1,
				"codec_type": "audio",
				"codec_name": "aac"
			}
		]
	}`

	info, err := parseVideoInfo([]byte(jsonData))
	if err != nil {
		t.Fatalf("parseVideoInfo returned error: %v", err)
	}

	if info.Duration != 120.5 {
		t.Errorf("Duration = %f, want 120.5", info.Duration)
	}

	if info.Width != 1920 {
		t.Errorf("Width = %d, want 1920", info.Width)
	}

	if info.VideoCodec != "h264" {
		t.Errorf("VideoCodec = %s, want h264", info.VideoCodec)
	}

	if info.AudioCodec != "aac" {
		t.Errorf("AudioCodec = %s, want aac", info.AudioCodec)
	}

	if info.Framerate != 30.0 {
		t.Errorf("Framerate = %f, want 30.0", info.Framerate)
	}
}

func TestTranscoder_QuickConvert(t *testing.T) {
	transcoder := NewTranscoder("ffmpeg", "ffprobe", 2)

	// This will fail without the input file, but we test the function signature
	_, err := transcoder.QuickConvert("/nonexistent.mp4", "/output.webm", "webm")
	if err == nil {
		t.Error("QuickConvert should return error for non-existent file")
	}
}

func TestTranscoder_OptimizeForWeb(t *testing.T) {
	transcoder := NewTranscoder("ffmpeg", "ffprobe", 2)

	_, err := transcoder.OptimizeForWeb("/nonexistent.mp4", "/output.mp4")
	if err == nil {
		t.Error("OptimizeForWeb should return error for non-existent file")
	}
}

func TestParseFFmpegValue(t *testing.T) {
	tests := []struct {
		line     string
		key      string
		expected int
		hasError bool
	}{
		{"frame= 123 fps=30", "frame", 123, false},
		{"frame=456", "frame", 456, false},
		{"fps=60", "fps", 60, false},
		{"no match here", "frame", 0, true},
	}

	for _, tt := range tests {
		result, err := parseFFmpegValue(tt.line, tt.key)
		if tt.hasError {
			if err == nil {
				t.Errorf("parseFFmpegValue(%s, %s) should return error", tt.line, tt.key)
			}
		} else {
			if err != nil {
				t.Errorf("parseFFmpegValue(%s, %s) returned error: %v", tt.line, tt.key, err)
			}
			if result != tt.expected {
				t.Errorf("parseFFmpegValue(%s, %s) = %d, want %d", tt.line, tt.key, result, tt.expected)
			}
		}
	}
}

func TestParseFFmpegFloatValue(t *testing.T) {
	tests := []struct {
		line     string
		key      string
		expected float64
		hasError bool
	}{
		{"fps=30.5", "fps", 30.5, false},
		{"fps=60", "fps", 60, false},
		{"no match", "fps", 0, true},
	}

	for _, tt := range tests {
		result, err := parseFFmpegFloatValue(tt.line, tt.key)
		if tt.hasError {
			if err == nil {
				t.Errorf("parseFFmpegFloatValue(%s, %s) should return error", tt.line, tt.key)
			}
		} else {
			if err != nil {
				t.Errorf("parseFFmpegFloatValue(%s, %s) returned error: %v", tt.line, tt.key, err)
			}
			if result != tt.expected {
				t.Errorf("parseFFmpegFloatValue(%s, %s) = %f, want %f", tt.line, tt.key, result, tt.expected)
			}
		}
	}
}

func TestParseFFmpegSpeed(t *testing.T) {
	tests := []struct {
		line     string
		expected string
	}{
		{"speed=1.5x", "1.5x"},
		{"speed=2.0x", "2.0x"},
		{"no speed here", ""},
	}

	for _, tt := range tests {
		result := parseFFmpegSpeed(tt.line)
		if result != tt.expected {
			t.Errorf("parseFFmpegSpeed(%s) = %s, want %s", tt.line, result, tt.expected)
		}
	}
}
