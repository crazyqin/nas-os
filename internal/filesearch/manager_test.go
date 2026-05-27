package filesearch

import (
	"testing"
	"time"
)

func TestSearchBasic(t *testing.T) {
	mgr := NewManager()

	mgr.IndexFile(&SearchResult{
		Path:      "/docs/readme.md",
		Name:      "readme.md",
		Extension: ".md",
		Size:      1024,
		FileType:  FileTypeDocument,
		ModTime:   time.Now(),
	})

	mgr.IndexFile(&SearchResult{
		Path:      "/photos/vacation.jpg",
		Name:      "vacation.jpg",
		Extension: ".jpg",
		Size:      5120000,
		FileType:  FileTypeImage,
		ModTime:   time.Now(),
	})

	result, err := mgr.Search(SearchRequest{Query: "readme"})
	if err != nil {
		t.Fatalf("Search failed: %v", err)
	}
	if result.Total != 1 {
		t.Errorf("expected 1 result, got %d", result.Total)
	}
	if result.Items[0].Name != "readme.md" {
		t.Errorf("expected readme.md, got %s", result.Items[0].Name)
	}
}

func TestSearchByType(t *testing.T) {
	mgr := NewManager()

	mgr.IndexFile(&SearchResult{
		Path:     "/docs/a.txt",
		Name:     "a.txt",
		FileType: FileTypeDocument,
		ModTime:  time.Now(),
	})
	mgr.IndexFile(&SearchResult{
		Path:     "/photos/b.jpg",
		Name:     "b.jpg",
		FileType: FileTypeImage,
		ModTime:  time.Now(),
	})

	result, _ := mgr.Search(SearchRequest{
		Query: "",
		Type:  FileTypeImage,
	})
	// 空查询+类型过滤应该只返回该类型
	if result.Total != 1 {
		t.Errorf("expected 1 result for type filter, got %d", result.Total)
	}
}

func TestSearchWithTags(t *testing.T) {
	mgr := NewManager()

	mgr.IndexFile(&SearchResult{
		Path:     "/docs/important.md",
		Name:     "important.md",
		FileType: FileTypeDocument,
		Tags:     []string{"work", "important"},
		ModTime:  time.Now(),
	})

	result, _ := mgr.Search(SearchRequest{
		Query: "important",
		Tags:  []string{"work"},
	})
	if result.Total != 1 {
		t.Errorf("expected 1 result, got %d", result.Total)
	}
}

func TestRemoveFromIndex(t *testing.T) {
	mgr := NewManager()

	mgr.IndexFile(&SearchResult{
		Path:     "/temp/file.txt",
		Name:     "file.txt",
		FileType: FileTypeDocument,
		ModTime:  time.Now(),
	})

	mgr.RemoveFromIndex("/temp/file.txt")

	result, _ := mgr.Search(SearchRequest{Query: "file"})
	if result.Total != 0 {
		t.Errorf("expected 0 results after removal, got %d", result.Total)
	}
}

func TestIndexStatus(t *testing.T) {
	mgr := NewManager()

	mgr.IndexFile(&SearchResult{Path: "/a", Name: "a", ModTime: time.Now()})
	mgr.IndexFile(&SearchResult{Path: "/b", Name: "b", ModTime: time.Now()})

	status := mgr.IndexStatus()
	if status.TotalFiles != 2 {
		t.Errorf("expected 2 indexed files, got %d", status.TotalFiles)
	}
}

func TestDetectFileType(t *testing.T) {
	tests := []struct {
		ext  string
		want FileType
	}{
		{".pdf", FileTypeDocument},
		{".jpg", FileTypeImage},
		{".mp4", FileTypeVideo},
		{".mp3", FileTypeAudio},
		{".zip", FileTypeArchive},
		{".go", FileTypeCode},
		{".xyz", FileTypeOther},
	}

	for _, tt := range tests {
		got := DetectFileType(tt.ext)
		if got != tt.want {
			t.Errorf("DetectFileType(%s) = %v, want %v", tt.ext, got, tt.want)
		}
	}
}

func TestSearchPagination(t *testing.T) {
	mgr := NewManager()

	for i := 0; i < 25; i++ {
		mgr.IndexFile(&SearchResult{
			Path:     "/files/file" + string(rune('a'+i)),
			Name:     "file" + string(rune('a'+i)),
			FileType: FileTypeDocument,
			ModTime:  time.Now(),
		})
	}

	result, _ := mgr.Search(SearchRequest{
		Query:    "file",
		Page:     0,
		PageSize: 10,
	})

	if len(result.Items) != 10 {
		t.Errorf("expected 10 items per page, got %d", len(result.Items))
	}
}
