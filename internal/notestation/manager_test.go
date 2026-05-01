// Package notestation 测试
package notestation

import (
	"testing"
)

func TestCreateNote(t *testing.T) {
	m := NewManager()
	note := m.CreateNote(CreateNoteRequest{
		Title:      "测试笔记",
		Content:    "# Hello\n这是测试内容",
		Author:     "admin",
		Tags:       []string{"test", "demo"},
		NotebookID: "default",
	})
	if note == nil {
		t.Fatal("笔记不应为nil")
	}
	if note.Title != "测试笔记" {
		t.Errorf("标题不匹配: %s", note.Title)
	}
	if note.Author != "admin" {
		t.Errorf("作者不匹配: %s", note.Author)
	}
	if len(note.Tags) != 2 {
		t.Errorf("标签数量不匹配: %d", len(note.Tags))
	}
}

func TestGetNote(t *testing.T) {
	m := NewManager()
	note := m.CreateNote(CreateNoteRequest{Title: "test", Content: "content", Author: "admin"})

	got, err := m.GetNote(note.ID)
	if err != nil {
		t.Fatalf("获取笔记失败: %v", err)
	}
	if got.Title != "test" {
		t.Errorf("标题不匹配")
	}
}

func TestUpdateNote(t *testing.T) {
	m := NewManager()
	note := m.CreateNote(CreateNoteRequest{Title: "old", Content: "old content", Author: "admin"})

	newTitle := "new"
	updated, err := m.UpdateNote(note.ID, UpdateNoteRequest{Title: &newTitle})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}
	if updated.Title != "new" {
		t.Errorf("标题未更新: %s", updated.Title)
	}
}

func TestDeleteNote(t *testing.T) {
	m := NewManager()
	note := m.CreateNote(CreateNoteRequest{Title: "to delete", Author: "admin"})

	err := m.DeleteNote(note.ID)
	if err != nil {
		t.Fatalf("删除失败: %v", err)
	}
	_, err = m.GetNote(note.ID)
	if err == nil {
		t.Error("已删除笔记不应存在")
	}
}

func TestListNotes(t *testing.T) {
	m := NewManager()
	m.CreateNote(CreateNoteRequest{Title: "note1", Author: "admin", NotebookID: "nb1"})
	m.CreateNote(CreateNoteRequest{Title: "note2", Author: "admin", NotebookID: "nb1"})
	m.CreateNote(CreateNoteRequest{Title: "note3", Author: "admin", NotebookID: "nb2"})

	all := m.ListNotesByNotebook("nb1")
	if len(all) != 2 {
		t.Errorf("期望2个笔记，实际 %d", len(all))
	}
}

func TestPinAndFavorite(t *testing.T) {
	m := NewManager()
	note := m.CreateNote(CreateNoteRequest{Title: "test", Author: "admin"})

	pinned := true
	updated, _ := m.UpdateNote(note.ID, UpdateNoteRequest{IsPinned: &pinned})
	if !updated.IsPinned {
		t.Error("笔记应已置顶")
	}

	fav := true
	updated, _ = m.UpdateNote(note.ID, UpdateNoteRequest{IsFavorite: &fav})
	if !updated.IsFavorite {
		t.Error("笔记应已收藏")
	}
}

func TestSearch(t *testing.T) {
	m := NewManager()
	m.CreateNote(CreateNoteRequest{Title: "Go语言入门", Content: "学习Go编程", Author: "admin"})
	m.CreateNote(CreateNoteRequest{Title: "Python教程", Content: "学习Python", Author: "admin"})

	results := m.SearchNotes("Go")
	if len(results) == 0 {
		t.Error("搜索Go应有结果")
	}
}

func TestCreateNotebook(t *testing.T) {
	m := NewManager()
	nb := m.CreateNotebook(CreateNotebookRequest{
		Name:        "工作笔记",
		Description: "工作相关",
		Owner:       "admin",
	})
	if nb == nil {
		t.Fatal("笔记本不应为nil")
	}
	if nb.Name != "工作笔记" {
		t.Errorf("名称不匹配: %s", nb.Name)
	}
}

func TestDeleteNotebook(t *testing.T) {
	m := NewManager()
	nb := m.CreateNotebook(CreateNotebookRequest{Name: "temp"})
	err := m.DeleteNotebook(nb.ID)
	if err != nil {
		t.Fatalf("删除笔记本失败: %v", err)
	}
}

func TestTagStats(t *testing.T) {
	m := NewManager()
	m.CreateNote(CreateNoteRequest{Title: "n1", Tags: []string{"go", "backend"}, Author: "a"})
	m.CreateNote(CreateNoteRequest{Title: "n2", Tags: []string{"go", "frontend"}, Author: "a"})
	m.CreateNote(CreateNoteRequest{Title: "n3", Tags: []string{"python"}, Author: "a"})

	stats := m.GetTagStats()
	goCount := 0
	for _, s := range stats {
		if s.Tag == "go" {
			goCount = s.Count
		}
	}
	if goCount != 2 {
		t.Errorf("go标签应出现2次，实际 %d", goCount)
	}
}

func TestExportNotes(t *testing.T) {
	m := NewManager()
	n1 := m.CreateNote(CreateNoteRequest{Title: "Note 1", Content: "Content 1", Author: "admin"})
	n2 := m.CreateNote(CreateNoteRequest{Title: "Note 2", Content: "Content 2", Author: "admin"})

	exports, err := m.ExportNotes([]string{n1.ID, n2.ID})
	if err != nil {
		t.Fatalf("导出失败: %v", err)
	}
	if len(exports) != 2 {
		t.Errorf("导出应有2个结果，实际 %d", len(exports))
	}
}
