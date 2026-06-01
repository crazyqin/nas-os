package unifiedsearch

import (
	"testing"
	"time"
)

func TestNewEngine(t *testing.T) {
	engine := NewEngine()
	if engine == nil {
		t.Fatal("NewEngine returned nil")
	}
}

func TestIndexAndSearch(t *testing.T) {
	engine := NewEngine()

	item := &EngineSearchItem{
		Name:    "项目报告.docx",
		Path:    "/documents/项目报告.docx",
		Type:    SourceDocument,
		Size:    1024 * 1024,
		Content: "这是一份重要的项目报告",
		Tags:    []string{"工作", "报告"},
		Owner:   "admin",
	}

	if err := engine.IndexItem(item); err != nil {
		t.Fatalf("IndexItem failed: %v", err)
	}

	if item.ID == "" {
		t.Error("Item ID not generated")
	}

	// 搜索
	results, total := engine.Search(EngineSearchQuery{Keyword: "项目报告"})
	if total != 1 {
		t.Errorf("Expected 1 result, got %d", total)
	}

	if len(results) == 0 {
		t.Fatal("No results returned")
	}

	if results[0].Item.Name != "项目报告.docx" {
		t.Errorf("Expected '项目报告.docx', got '%s'", results[0].Item.Name)
	}
}

func TestSearchByType(t *testing.T) {
	engine := NewEngine()

	engine.IndexItem(&EngineSearchItem{Name: "photo.jpg", Type: SourcePhoto, Content: "风景照片"})
	engine.IndexItem(&EngineSearchItem{Name: "doc.pdf", Type: SourceDocument, Content: "风景文档"})
	engine.IndexItem(&EngineSearchItem{Name: "video.mp4", Type: SourceVideo, Content: "风景视频"})

	results, total := engine.Search(EngineSearchQuery{
		Keyword: "风景",
		Types:   []EngineSearchType{SourcePhoto},
	})

	if total != 1 {
		t.Errorf("Expected 1 result, got %d", total)
	}

	if len(results) > 0 && results[0].Item.Type != SourcePhoto {
		t.Errorf("Expected photo type, got %s", results[0].Item.Type)
	}
}

func TestSearchByTags(t *testing.T) {
	engine := NewEngine()

	engine.IndexItem(&EngineSearchItem{Name: "file1.txt", Tags: []string{"重要", "工作"}})
	engine.IndexItem(&EngineSearchItem{Name: "file2.txt", Tags: []string{"个人"}})
	engine.IndexItem(&EngineSearchItem{Name: "file3.txt", Tags: []string{"重要", "备份"}})

	_, total := engine.Search(EngineSearchQuery{Tags: []string{"重要"}})

	if total != 2 {
		t.Errorf("Expected 2 results, got %d", total)
	}
}

func TestSearchByOwner(t *testing.T) {
	engine := NewEngine()

	engine.IndexItem(&EngineSearchItem{Name: "a.txt", Owner: "alice"})
	engine.IndexItem(&EngineSearchItem{Name: "b.txt", Owner: "bob"})

	_, total := engine.Search(EngineSearchQuery{Owner: "alice"})

	if total != 1 {
		t.Errorf("Expected 1 result, got %d", total)
	}
}

func TestSearchByDateRange(t *testing.T) {
	engine := NewEngine()

	// 先索引，然后手动设置更新时间
	oldItem := &EngineSearchItem{Name: "old.txt"}
	engine.IndexItem(oldItem)
	oldItem.UpdatedAt = time.Now().Add(-90 * 24 * time.Hour)

	newItem := &EngineSearchItem{Name: "new.txt"}
	engine.IndexItem(newItem)
	newItem.UpdatedAt = time.Now().Add(-1 * 24 * time.Hour)

	// 搜索最近7天的
	from := time.Now().Add(-7 * 24 * time.Hour)
	results, total := engine.Search(EngineSearchQuery{DateFrom: &from})

	if total != 1 {
		t.Errorf("Expected 1 result, got %d", total)
	}
	if len(results) > 0 && results[0].Item.Name != "new.txt" {
		t.Errorf("Expected 'new.txt', got '%s'", results[0].Item.Name)
	}
}

func TestSearchBySize(t *testing.T) {
	engine := NewEngine()

	engine.IndexItem(&EngineSearchItem{Name: "small.txt", Size: 100})
	engine.IndexItem(&EngineSearchItem{Name: "large.bin", Size: 1024 * 1024 * 100})

	_, total := engine.Search(EngineSearchQuery{SizeMin: 1024 * 1024})

	if total != 1 {
		t.Errorf("Expected 1 result, got %d", total)
	}
}

func TestSearchPagination(t *testing.T) {
	engine := NewEngine()

	for i := 0; i < 10; i++ {
		engine.IndexItem(&EngineSearchItem{
			Name:    "test.txt",
			Content: "测试内容",
		})
	}

	results, total := engine.Search(EngineSearchQuery{
		Keyword: "测试",
		Limit:   5,
		Offset:  0,
	})

	if total != 10 {
		t.Errorf("Expected 10 total, got %d", total)
	}

	if len(results) != 5 {
		t.Errorf("Expected 5 results, got %d", len(results))
	}
}

func TestSearchSort(t *testing.T) {
	engine := NewEngine()

	engine.IndexItem(&EngineSearchItem{Name: "b.txt", Content: "排序测试", Size: 200})
	engine.IndexItem(&EngineSearchItem{Name: "a.txt", Content: "排序测试", Size: 100})
	engine.IndexItem(&EngineSearchItem{Name: "c.txt", Content: "排序测试", Size: 300})

	results, _ := engine.Search(EngineSearchQuery{
		Keyword: "排序测试",
		SortBy:  "name",
	})

	if len(results) >= 2 {
		if results[0].Item.Name > results[1].Item.Name {
			t.Error("Results not sorted by name")
		}
	}
}

func TestRemoveItem(t *testing.T) {
	engine := NewEngine()

	engine.IndexItem(&EngineSearchItem{Name: "test.txt"})
	engine.IndexItem(&EngineSearchItem{Name: "test2.txt"})

	_, total := engine.Search(EngineSearchQuery{})
	if total != 2 {
		t.Errorf("Expected 2 items, got %d", total)
	}

	// 获取第一个item的ID
	items := engine.ListItems(0)
	if len(items) < 2 {
		t.Fatal("Expected at least 2 items")
	}

	engine.RemoveItem(items[0].ID)

	_, total = engine.Search(EngineSearchQuery{})
	if total != 1 {
		t.Errorf("Expected 1 item after removal, got %d", total)
	}
}

func TestGetStats(t *testing.T) {
	engine := NewEngine()

	engine.IndexItem(&EngineSearchItem{Name: "a.txt", Type: SourceFile})
	engine.IndexItem(&EngineSearchItem{Name: "b.jpg", Type: SourcePhoto})
	engine.IndexItem(&EngineSearchItem{Name: "c.pdf", Type: SourceDocument})
	engine.IndexItem(&EngineSearchItem{Name: "d.txt", Type: SourceFile})

	stats := engine.GetStats()

	if stats.TotalItems != 4 {
		t.Errorf("Expected 4 total items, got %d", stats.TotalItems)
	}

	if stats.ItemsByType["file"] != 2 {
		t.Errorf("Expected 2 file items, got %d", stats.ItemsByType["file"])
	}
}

func TestRemoveItemNotFound(t *testing.T) {
	engine := NewEngine()

	err := engine.RemoveItem("non-existent")
	if err == nil {
		t.Error("Expected error for non-existent item")
	}
}
