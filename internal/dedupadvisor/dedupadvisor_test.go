package dedupadvisor

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanEmptyDir(t *testing.T) {
	advisor := NewAdvisor()

	// 创建临时目录
	tmpDir := t.TempDir()

	result, err := advisor.Scan([]string{tmpDir})
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	if result.TotalFiles != 0 {
		t.Errorf("期望0个文件, 得到 %d", result.TotalFiles)
	}
}

func TestScanWithDuplicates(t *testing.T) {
	advisor := NewAdvisor()

	// 创建临时目录和重复文件
	tmpDir := t.TempDir()

	// 创建相同的文件（大于1KB以满足MinFileSize）
	content := make([]byte, 2048)
	for i := range content {
		content[i] = byte('A' + i%26)
	}
	for i := 0; i < 3; i++ {
		path := filepath.Join(tmpDir, "file"+string(rune('A'+i))+".txt")
		os.WriteFile(path, content, 0644)
	}

	result, err := advisor.Scan([]string{tmpDir})
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	if result.TotalFiles != 3 {
		t.Errorf("期望3个文件, 得到 %d", result.TotalFiles)
	}

	if result.DuplicateFiles != 2 {
		t.Errorf("期望2个重复文件, 得到 %d", result.DuplicateFiles)
	}

	if len(result.Candidates) != 1 {
		t.Errorf("期望1个候选, 得到 %d", len(result.Candidates))
	}
}

func TestScanNoDuplicates(t *testing.T) {
	advisor := NewAdvisor()

	tmpDir := t.TempDir()

	// 创建不同的文件（大于1KB以满足MinFileSize）
	for i := 0; i < 3; i++ {
		path := filepath.Join(tmpDir, "unique"+string(rune('A'+i))+".txt")
		content := make([]byte, 2048)
		for j := range content {
			content[j] = byte('A' + i + j%26)
		}
		os.WriteFile(path, content, 0644)
	}

	result, err := advisor.Scan([]string{tmpDir})
	if err != nil {
		t.Fatalf("扫描失败: %v", err)
	}

	if result.DuplicateFiles != 0 {
		t.Errorf("期望0个重复文件, 得到 %d", result.DuplicateFiles)
	}
}

func TestDetectFileType(t *testing.T) {
	advisor := NewAdvisor()

	tests := []struct {
		path     string
		expected FileType
	}{
		{"doc.pdf", FileTypeDocument},
		{"photo.jpg", FileTypeImage},
		{"video.mp4", FileTypeVideo},
		{"song.mp3", FileTypeAudio},
		{"archive.zip", FileTypeArchive},
		{"other.xyz", FileTypeOther},
	}

	for _, tt := range tests {
		result := advisor.detectFileType(tt.path)
		if result != tt.expected {
			t.Errorf("%s: 期望 %s, 得到 %s", tt.path, tt.expected, result)
		}
	}
}

func TestFormatSize(t *testing.T) {
	tests := []struct {
		bytes    int64
		expected string
	}{
		{1024, "1.00 KB"},
		{1048576, "1.00 MB"},
		{1073741824, "1.00 GB"},
		{500, "500 B"},
	}

	for _, tt := range tests {
		result := formatSize(tt.bytes)
		if result != tt.expected {
			t.Errorf("%d: 期望 %s, 得到 %s", tt.bytes, tt.expected, result)
		}
	}
}

func TestGetLastScan(t *testing.T) {
	advisor := NewAdvisor()

	if advisor.GetLastScan() != nil {
		t.Error("初始应无扫描结果")
	}

	tmpDir := t.TempDir()
	advisor.Scan([]string{tmpDir})

	if advisor.GetLastScan() == nil {
		t.Error("扫描后应有结果")
	}
}
