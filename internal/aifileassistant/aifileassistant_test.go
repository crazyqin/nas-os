package aifileassistant

import (
	"testing"
)

func TestAIFileAssistant_AnalyzeFile(t *testing.T) {
	afa := NewAIFileAssistant(nil)

	content := []byte("This is a test document with some content for analysis.")
	analysis, err := afa.AnalyzeFile("/documents/test.txt", content)
	if err != nil {
		t.Fatalf("Failed to analyze file: %v", err)
	}

	if analysis.Category != CategoryDocument {
		t.Errorf("Expected category 'document', got '%s'", analysis.Category)
	}

	if len(analysis.Tags) == 0 {
		t.Error("Expected at least one tag")
	}

	if analysis.Language != "en" {
		t.Errorf("Expected language 'en', got '%s'", analysis.Language)
	}
}

func TestAIFileAssistant_SearchFiles(t *testing.T) {
	afa := NewAIFileAssistant(nil)

	// 添加一些文件
	afa.AnalyzeFile("/docs/readme.md", []byte("# README\nThis is the readme file"))
	afa.AnalyzeFile("/code/main.go", []byte("package main\n\nfunc main() {}"))
	afa.AnalyzeFile("/images/photo.jpg", []byte("fake image content"))

	// 搜索
	query := &SearchQuery{
		Query:    "readme",
		Category: CategoryDocument,
		Limit:    10,
	}

	result, err := afa.SearchFiles(query)
	if err != nil {
		t.Fatalf("Failed to search: %v", err)
	}

	if result.Total == 0 {
		t.Error("Expected at least one result")
	}
}

func TestAIFileAssistant_DetectDuplicates(t *testing.T) {
	afa := NewAIFileAssistant(nil)

	content := []byte("duplicate content")

	// 添加重复文件
	afa.AnalyzeFile("/file1.txt", content)
	afa.AnalyzeFile("/file2.txt", content)
	afa.AnalyzeFile("/file3.txt", []byte("different content"))

	duplicates := afa.DetectDuplicates()

	if len(duplicates) == 0 {
		t.Error("Expected duplicate group")
	}
}

func TestAIFileAssistant_GetOrganizeSuggestions(t *testing.T) {
	afa := NewAIFileAssistant(nil)

	// 添加大文件
	largeContent := make([]byte, 200*1024*1024) // 200MB
	afa.AnalyzeFile("/large-file.zip", largeContent)

	suggestions := afa.GetOrganizeSuggestions()

	if len(suggestions) == 0 {
		t.Error("Expected organize suggestions")
	}
}

func TestAIFileAssistant_Stats(t *testing.T) {
	afa := NewAIFileAssistant(nil)

	afa.AnalyzeFile("/file1.txt", []byte("content 1"))
	afa.AnalyzeFile("/file2.jpg", []byte("content 2"))
	afa.AnalyzeFile("/file3.go", []byte("content 3"))

	stats := afa.GetStats()

	if stats.TotalFiles != 3 {
		t.Errorf("Expected 3 files, got %d", stats.TotalFiles)
	}

	if stats.CategoryCounts[CategoryDocument] == 0 {
		t.Error("Expected document category count")
	}
}

func TestAIFileAssistant_AddManualTag(t *testing.T) {
	afa := NewAIFileAssistant(nil)

	analysis, _ := afa.AnalyzeFile("/file.txt", []byte("content"))

	err := afa.AddManualTag(analysis.ID, "important")
	if err != nil {
		t.Fatalf("Failed to add tag: %v", err)
	}

	tags, _ := afa.GetFileTags(analysis.ID)
	found := false
	for _, tag := range tags {
		if tag.Name == "important" && tag.Type == TagManual {
			found = true
			break
		}
	}

	if !found {
		t.Error("Expected manual tag 'important'")
	}
}

func TestAIFileAssistant_ClassifyFile(t *testing.T) {
	afa := NewAIFileAssistant(nil)

	tests := []struct {
		path     string
		expected FileCategory
	}{
		{"doc.pdf", CategoryDocument},
		{"photo.jpg", CategoryImage},
		{"video.mp4", CategoryVideo},
		{"song.mp3", CategoryAudio},
		{"code.go", CategoryCode},
		{"archive.zip", CategoryArchive},
		{"data.csv", CategoryData},
		{"unknown.xyz", CategoryOther},
	}

	for _, tt := range tests {
		result := afa.classifyFile(tt.path, []byte{})
		if result != tt.expected {
			t.Errorf("classifyFile(%s): expected %s, got %s", tt.path, tt.expected, result)
		}
	}
}

func TestFileCategory_Constants(t *testing.T) {
	categories := []FileCategory{
		CategoryDocument, CategoryImage, CategoryVideo, CategoryAudio,
		CategoryCode, CategoryArchive, CategoryData, CategoryOther,
	}

	for _, c := range categories {
		if c == "" {
			t.Error("Category constant should not be empty")
		}
	}
}

func TestAIFileAssistant_MarshalJSON(t *testing.T) {
	afa := NewAIFileAssistant(nil)

	data, err := afa.MarshalJSON()
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	if len(data) == 0 {
		t.Error("Expected non-empty JSON")
	}
}
