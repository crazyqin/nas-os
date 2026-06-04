package filepreview

import (
	"testing"
)

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		filename string
		expected FileType
	}{
		{"photo.jpg", FileTypeImage},
		{"image.png", FileTypeImage},
		{"pic.gif", FileTypeImage},
		{"photo.heic", FileTypeImage},
		{"raw.cr2", FileTypeImage},
		{"video.mp4", FileTypeVideo},
		{"movie.mkv", FileTypeVideo},
		{"clip.avi", FileTypeVideo},
		{"song.mp3", FileTypeAudio},
		{"track.flac", FileTypeAudio},
		{"sound.wav", FileTypeAudio},
		{"doc.pdf", FileTypeDocument},
		{"report.docx", FileTypeDocument},
		{"data.xlsx", FileTypeDocument},
		{"notes.md", FileTypeDocument},
		{"model.gltf", FileType3D},
		{"mesh.obj", FileType3D},
		{"print.stl", FileType3D},
		{"unknown.xyz", FileTypeUnknown},
		{"noext", FileTypeUnknown},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := DetectFileType(tt.filename)
			if result != tt.expected {
				t.Errorf("DetectFileType(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestDetectImageFormat(t *testing.T) {
	tests := []struct {
		filename string
		expected ImageFormat
	}{
		{"photo.jpg", FormatJPEG},
		{"photo.jpeg", FormatJPEG},
		{"image.png", FormatPNG},
		{"anim.gif", FormatGIF},
		{"modern.webp", FormatWebP},
		{"apple.heic", FormatHEIC},
		{"raw.cr2", FormatRAW},
		{"raw.nef", FormatRAW},
		{"raw.arw", FormatRAW},
		{"raw.dng", FormatRAW},
		{"bitmap.bmp", FormatBMP},
		{"scan.tiff", FormatTIFF},
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := DetectImageFormat(tt.filename)
			if result != tt.expected {
				t.Errorf("DetectImageFormat(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestDetectVideoFormat(t *testing.T) {
	tests := []struct {
		filename string
		expected VideoFormat
	}{
		{"video.mp4", VideoMP4},
		{"movie.mkv", VideoMKV},
		{"clip.avi", VideoAVI},
		{"recording.mov", VideoMOV},
		{"old.wmv", VideoWMV},
		{"web.webm", VideoWebM},
		{"flash.flv", VideoFLV},
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := DetectVideoFormat(tt.filename)
			if result != tt.expected {
				t.Errorf("DetectVideoFormat(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestDetectAudioFormat(t *testing.T) {
	tests := []struct {
		filename string
		expected AudioFormat
	}{
		{"song.mp3", AudioMP3},
		{"hires.flac", AudioFLAC},
		{"recording.wav", AudioWAV},
		{"podcast.aac", AudioAAC},
		{"stream.ogg", AudioOGG},
		{"voice.opus", AudioOPUS},
		{"itunes.m4a", AudioM4A},
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := DetectAudioFormat(tt.filename)
			if result != tt.expected {
				t.Errorf("DetectAudioFormat(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestDetectDocumentFormat(t *testing.T) {
	tests := []struct {
		filename string
		expected DocumentFormat
	}{
		{"doc.pdf", DocPDF},
		{"report.docx", DocDOCX},
		{"data.xlsx", DocXLSX},
		{"slides.pptx", DocPPTX},
		{"readme.md", DocMarkdown},
		{"readme.markdown", DocMarkdown},
		{"page.html", DocHTML},
		{"page.htm", DocHTML},
		{"notes.txt", DocTXT},
		{"data.csv", DocCSV},
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := DetectDocumentFormat(tt.filename)
			if result != tt.expected {
				t.Errorf("DetectDocumentFormat(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestDetectModel3DFormat(t *testing.T) {
	tests := []struct {
		filename string
		expected Model3DFormat
	}{
		{"model.gltf", ModelGLTF},
		{"model.glb", ModelGLB},
		{"mesh.obj", ModelOBJ},
		{"print.stl", ModelSTL},
		{"anim.fbx", ModelFBX},
		{"scan.ply", ModelPLY},
		{"unknown.xyz", ""},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			result := DetectModel3DFormat(tt.filename)
			if result != tt.expected {
				t.Errorf("DetectModel3DFormat(%q) = %v, want %v", tt.filename, result, tt.expected)
			}
		})
	}
}

func TestGetThumbnailDimensions(t *testing.T) {
	tests := []struct {
		size          ThumbnailSize
		expectedWidth int
		expectedHeight int
	}{
		{SizeSmall, 150, 150},
		{SizeMedium, 300, 300},
		{SizeLarge, 600, 600},
		{SizeXL, 1200, 1200},
		{"unknown", 300, 300}, // 默认值
	}

	for _, tt := range tests {
		t.Run(string(tt.size), func(t *testing.T) {
			width, height := GetThumbnailDimensions(tt.size)
			if width != tt.expectedWidth || height != tt.expectedHeight {
				t.Errorf("GetThumbnailDimensions(%v) = (%d, %d), want (%d, %d)",
					tt.size, width, height, tt.expectedWidth, tt.expectedHeight)
			}
		})
	}
}

func TestIsSupported(t *testing.T) {
	tests := []struct {
		filename      string
		imageSupport   bool
		videoSupport   bool
		audioSupport   bool
		documentSupport bool
		model3DSupport bool
	}{
		{"photo.jpg", true, false, false, false, false},
		{"video.mp4", false, true, false, false, false},
		{"song.mp3", false, false, true, false, false},
		{"doc.pdf", false, false, false, true, false},
		{"model.obj", false, false, false, false, true},
		{"unknown.xyz", false, false, false, false, false},
	}

	for _, tt := range tests {
		t.Run(tt.filename, func(t *testing.T) {
			if got := IsImageSupported(tt.filename); got != tt.imageSupport {
				t.Errorf("IsImageSupported(%q) = %v, want %v", tt.filename, got, tt.imageSupport)
			}
			if got := IsVideoSupported(tt.filename); got != tt.videoSupport {
				t.Errorf("IsVideoSupported(%q) = %v, want %v", tt.filename, got, tt.videoSupport)
			}
			if got := IsAudioSupported(tt.filename); got != tt.audioSupport {
				t.Errorf("IsAudioSupported(%q) = %v, want %v", tt.filename, got, tt.audioSupport)
			}
			if got := IsDocumentSupported(tt.filename); got != tt.documentSupport {
				t.Errorf("IsDocumentSupported(%q) = %v, want %v", tt.filename, got, tt.documentSupport)
			}
			if got := IsModel3DSupported(tt.filename); got != tt.model3DSupport {
				t.Errorf("IsModel3DSupported(%q) = %v, want %v", tt.filename, got, tt.model3DSupport)
			}
		})
	}
}

func TestDefaultPreviewConfig(t *testing.T) {
	config := DefaultPreviewConfig()

	if config == nil {
		t.Fatal("DefaultPreviewConfig() returned nil")
	}

	if config.Cache.CacheDir == "" {
		t.Error("CacheDir should not be empty")
	}

	if config.Cache.MaxSize <= 0 {
		t.Error("MaxSize should be positive")
	}

	if config.Cache.MaxEntries <= 0 {
		t.Error("MaxEntries should be positive")
	}

	if config.Cache.DefaultTTL <= 0 {
		t.Error("DefaultTTL should be positive")
	}

	if config.FFmpegPath == "" {
		t.Error("FFmpegPath should not be empty")
	}

	if config.MaxConcurrent <= 0 {
		t.Error("MaxConcurrent should be positive")
	}

	if config.DefaultQuality <= 0 || config.DefaultQuality > 100 {
		t.Errorf("DefaultQuality should be 1-100, got %d", config.DefaultQuality)
	}
}

func TestGenerateCacheKey(t *testing.T) {
	tests := []struct {
		filePath string
		width    int
		height   int
		options  []string
		expected string
	}{
		{"/path/to/file.jpg", 300, 300, nil, "/path/to/file.jpg_300x300"},
		{"/path/to/file.jpg", 300, 300, []string{"quality=80"}, "/path/to/file.jpg_300x300_quality=80"},
		{"/path/to/file.jpg", 300, 300, []string{"quality=80", "format=png"}, "/path/to/file.jpg_300x300_quality=80_format=png"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := GenerateCacheKey(tt.filePath, tt.width, tt.height, tt.options...)
			if result != tt.expected {
				t.Errorf("GenerateCacheKey() = %q, want %q", result, tt.expected)
			}
		})
	}
}

func TestGetContentType(t *testing.T) {
	tests := []struct {
		format   ImageFormat
		expected string
	}{
		{FormatJPEG, "image/jpeg"},
		{FormatPNG, "image/png"},
		{FormatGIF, "image/gif"},
		{FormatWebP, "image/webp"},
		{FormatHEIC, "image/heic"},
		{FormatBMP, "image/bmp"},
		{FormatTIFF, "image/tiff"},
		{"unknown", "image/jpeg"},
	}

	for _, tt := range tests {
		t.Run(string(tt.format), func(t *testing.T) {
			result := getContentType(tt.format)
			if result != tt.expected {
				t.Errorf("getContentType(%v) = %q, want %q", tt.format, result, tt.expected)
			}
		})
	}
}
