package collaborativenotes

import (
	"strings"
	"testing"
)

func TestNewCollaborativeNotes(t *testing.T) {
	cn := NewCollaborativeNotes()
	if cn == nil {
		t.Fatal("NewCollaborativeNotes returned nil")
	}
}

func TestCreateNotebook(t *testing.T) {
	cn := NewCollaborativeNotes()

	notebook := &Notebook{
		ID:      "nb1",
		Name:    "工作笔记",
		OwnerID: "user1",
	}

	err := cn.CreateNotebook(notebook)
	if err != nil {
		t.Fatalf("CreateNotebook failed: %v", err)
	}

	if notebook.CreatedAt.IsZero() {
		t.Error("expected CreatedAt to be set")
	}

	// 测试重复创建
	err = cn.CreateNotebook(notebook)
	if err == nil {
		t.Error("expected error for duplicate notebook")
	}
}

func TestCreateNestedNotebook(t *testing.T) {
	cn := NewCollaborativeNotes()

	parent := &Notebook{
		ID:      "parent",
		Name:    "父笔记本",
		OwnerID: "user1",
	}
	cn.CreateNotebook(parent)

	child := &Notebook{
		ID:       "child",
		Name:     "子笔记本",
		OwnerID:  "user1",
		ParentID: "parent",
	}

	err := cn.CreateNotebook(child)
	if err != nil {
		t.Fatalf("CreateNotebook failed for child: %v", err)
	}

	// 测试父笔记本不存在
	invalid := &Notebook{
		ID:       "invalid",
		Name:     "无效笔记本",
		OwnerID:  "user1",
		ParentID: "nonexistent",
	}

	err = cn.CreateNotebook(invalid)
	if err == nil {
		t.Error("expected error for non-existent parent")
	}
}

func TestGetNotebook(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNotebook(&Notebook{ID: "nb1", Name: "Test", OwnerID: "user1"})

	notebook, err := cn.GetNotebook("nb1")
	if err != nil {
		t.Fatalf("GetNotebook failed: %v", err)
	}

	if notebook.Name != "Test" {
		t.Errorf("expected Test, got %s", notebook.Name)
	}

	_, err = cn.GetNotebook("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent notebook")
	}
}

func TestUpdateNotebook(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNotebook(&Notebook{ID: "nb1", Name: "Original", OwnerID: "user1"})

	notebook := &Notebook{ID: "nb1", Name: "Updated", OwnerID: "user1"}
	err := cn.UpdateNotebook(notebook)
	if err != nil {
		t.Fatalf("UpdateNotebook failed: %v", err)
	}

	updated, _ := cn.GetNotebook("nb1")
	if updated.Name != "Updated" {
		t.Errorf("expected Updated, got %s", updated.Name)
	}
}

func TestDeleteNotebook(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNotebook(&Notebook{ID: "nb1", Name: "Test", OwnerID: "user1"})

	err := cn.DeleteNotebook("nb1")
	if err != nil {
		t.Fatalf("DeleteNotebook failed: %v", err)
	}

	_, err = cn.GetNotebook("nb1")
	if err == nil {
		t.Error("expected error for deleted notebook")
	}
}

func TestDeleteNotebookWithChildren(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNotebook(&Notebook{ID: "parent", Name: "Parent", OwnerID: "user1"})
	cn.CreateNotebook(&Notebook{ID: "child", Name: "Child", OwnerID: "user1", ParentID: "parent"})

	err := cn.DeleteNotebook("parent")
	if err == nil {
		t.Error("expected error when deleting notebook with children")
	}
}

func TestDeleteNotebookWithNotes(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNotebook(&Notebook{ID: "nb1", Name: "Test", OwnerID: "user1"})
	cn.CreateNote(&Note{ID: "note1", Title: "Test", NotebookID: "nb1", Author: "user1"})

	err := cn.DeleteNotebook("nb1")
	if err == nil {
		t.Error("expected error when deleting notebook with notes")
	}
}

func TestListNotebooks(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNotebook(&Notebook{ID: "nb1", Name: "工作", OwnerID: "user1"})
	cn.CreateNotebook(&Notebook{ID: "nb2", Name: "个人", OwnerID: "user1"})
	cn.CreateNotebook(&Notebook{ID: "nb3", Name: "共享", OwnerID: "user2", IsShared: true, SharedWith: []string{"user3"}})

	notebooks := cn.ListNotebooks("user1", "")
	if len(notebooks) != 2 {
		t.Errorf("expected 2 notebooks for user1, got %d", len(notebooks))
	}

	notebooks = cn.ListNotebooks("user3", "")
	if len(notebooks) != 1 {
		t.Errorf("expected 1 shared notebook for user3, got %d", len(notebooks))
	}
}

func TestShareNotebook(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNotebook(&Notebook{ID: "nb1", Name: "Test", OwnerID: "user1"})

	err := cn.ShareNotebook("nb1", "user2")
	if err != nil {
		t.Fatalf("ShareNotebook failed: %v", err)
	}

	notebook, _ := cn.GetNotebook("nb1")
	if !notebook.IsShared {
		t.Error("expected notebook to be shared")
	}

	// 测试重复分享
	err = cn.ShareNotebook("nb1", "user2")
	if err == nil {
		t.Error("expected error for duplicate share")
	}
}

func TestCreateNote(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNotebook(&Notebook{ID: "nb1", Name: "Test", OwnerID: "user1"})

	note := &Note{
		ID:         "note1",
		Title:      "我的第一篇笔记",
		Content:    "Hello World",
		Markdown:   "# Hello World",
		NotebookID: "nb1",
		Author:     "user1",
	}

	err := cn.CreateNote(note)
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	if note.Status != StatusDraft {
		t.Errorf("expected draft status, got %v", note.Status)
	}

	if note.Version != 1 {
		t.Errorf("expected version 1, got %d", note.Version)
	}

	// 测试重复创建
	err = cn.CreateNote(note)
	if err == nil {
		t.Error("expected error for duplicate note")
	}
}

func TestGetNote(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "Test", Author: "user1"})

	note, err := cn.GetNote("note1")
	if err != nil {
		t.Fatalf("GetNote failed: %v", err)
	}

	if note.Title != "Test" {
		t.Errorf("expected Test, got %s", note.Title)
	}

	_, err = cn.GetNote("nonexistent")
	if err == nil {
		t.Error("expected error for non-existent note")
	}
}

func TestUpdateNote(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "Original", Content: "v1", Author: "user1"})

	note := &Note{ID: "note1", Title: "Updated", Content: "v2", Version: 1}
	err := cn.UpdateNote(note, "user1", "更新内容")
	if err != nil {
		t.Fatalf("UpdateNote failed: %v", err)
	}

	if note.Version != 2 {
		t.Errorf("expected version 2, got %d", note.Version)
	}

	// 测试版本冲突（last_write_wins策略）
	note2 := &Note{ID: "note1", Title: "Conflict", Content: "v3", Version: 1}
	err = cn.UpdateNote(note2, "user2", "冲突更新")
	if err != nil {
		t.Fatalf("UpdateNote should succeed with last_write_wins: %v", err)
	}
}

func TestUpdateNoteConflictFirstWrite(t *testing.T) {
	cn := NewCollaborativeNotes()
	cn.SetConflictResolution(ConflictFirstWrite)

	cn.CreateNote(&Note{ID: "note1", Title: "Original", Content: "v1", Author: "user1"})

	note := &Note{ID: "note1", Title: "Updated", Content: "v2", Version: 1}
	err := cn.UpdateNote(note, "user1", "第一次更新")
	if err != nil {
		t.Fatalf("UpdateNote failed: %v", err)
	}

	note2 := &Note{ID: "note1", Title: "Conflict", Content: "v3", Version: 1}
	err = cn.UpdateNote(note2, "user2", "冲突更新")
	if err == nil {
		t.Error("expected conflict error with first_write_wins")
	}
}

func TestDeleteNote(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "Test", Author: "user1"})

	err := cn.DeleteNote("note1")
	if err != nil {
		t.Fatalf("DeleteNote failed: %v", err)
	}

	_, err = cn.GetNote("note1")
	if err == nil {
		t.Error("expected error for deleted note")
	}
}

func TestListNotes(t *testing.T) {
	cn := NewCollaborativeNotes()

	// 先创建笔记本
	cn.CreateNotebook(&Notebook{ID: "nb1", Name: "NB1", OwnerID: "user1"})
	cn.CreateNotebook(&Notebook{ID: "nb2", Name: "NB2", OwnerID: "user1"})

	err := cn.CreateNote(&Note{ID: "note1", Title: "Note 1", NotebookID: "nb1", Author: "user1"})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}
	err = cn.CreateNote(&Note{ID: "note2", Title: "Note 2", NotebookID: "nb1", Author: "user2"})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}
	err = cn.CreateNote(&Note{ID: "note3", Title: "Note 3", NotebookID: "nb2", Author: "user1"})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	notes := cn.ListNotes("nb1", "", "")
	if len(notes) != 2 {
		t.Errorf("expected 2 notes in nb1, got %d", len(notes))
	}

	notes = cn.ListNotes("", "", "user1")
	if len(notes) != 2 {
		t.Errorf("expected 2 notes by user1, got %d", len(notes))
	}
}

func TestMoveNote(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNotebook(&Notebook{ID: "nb1", Name: "NB1", OwnerID: "user1"})
	cn.CreateNotebook(&Notebook{ID: "nb2", Name: "NB2", OwnerID: "user1"})
	cn.CreateNote(&Note{ID: "note1", Title: "Test", NotebookID: "nb1", Author: "user1"})

	err := cn.MoveNote("note1", "nb2")
	if err != nil {
		t.Fatalf("MoveNote failed: %v", err)
	}

	note, _ := cn.GetNote("note1")
	if note.NotebookID != "nb2" {
		t.Errorf("expected nb2, got %s", note.NotebookID)
	}
}

func TestPinNote(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "Test", Author: "user1"})

	err := cn.PinNote("note1")
	if err != nil {
		t.Fatalf("PinNote failed: %v", err)
	}

	note, _ := cn.GetNote("note1")
	if !note.IsPinned {
		t.Error("expected note to be pinned")
	}

	// 取消置顶
	cn.PinNote("note1")
	note, _ = cn.GetNote("note1")
	if note.IsPinned {
		t.Error("expected note to be unpinned")
	}
}

func TestFavoriteNote(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "Test", Author: "user1"})

	err := cn.FavoriteNote("note1")
	if err != nil {
		t.Fatalf("FavoriteNote failed: %v", err)
	}

	note, _ := cn.GetNote("note1")
	if !note.IsFavorite {
		t.Error("expected note to be favorited")
	}
}

func TestCreateTag(t *testing.T) {
	cn := NewCollaborativeNotes()

	tag := &Tag{ID: "tag1", Name: "重要", Color: "#ff0000"}

	err := cn.CreateTag(tag)
	if err != nil {
		t.Fatalf("CreateTag failed: %v", err)
	}

	// 测试重复创建
	err = cn.CreateTag(tag)
	if err == nil {
		t.Error("expected error for duplicate tag")
	}
}

func TestGetTag(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateTag(&Tag{ID: "tag1", Name: "重要"})

	tag, err := cn.GetTag("tag1")
	if err != nil {
		t.Fatalf("GetTag failed: %v", err)
	}

	if tag.Name != "重要" {
		t.Errorf("expected 重要, got %s", tag.Name)
	}
}

func TestUpdateTag(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateTag(&Tag{ID: "tag1", Name: "Original"})

	tag := &Tag{ID: "tag1", Name: "Updated"}
	err := cn.UpdateTag(tag)
	if err != nil {
		t.Fatalf("UpdateTag failed: %v", err)
	}

	updated, _ := cn.GetTag("tag1")
	if updated.Name != "Updated" {
		t.Errorf("expected Updated, got %s", updated.Name)
	}
}

func TestDeleteTag(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateTag(&Tag{ID: "tag1", Name: "Test"})
	cn.CreateNote(&Note{ID: "note1", Title: "Test", Author: "user1", Tags: []string{"tag1"}})
	cn.AddNoteTag("note1", "tag1")

	err := cn.DeleteTag("tag1")
	if err != nil {
		t.Fatalf("DeleteTag failed: %v", err)
	}

	_, err = cn.GetTag("tag1")
	if err == nil {
		t.Error("expected error for deleted tag")
	}

	tags := cn.GetNoteTags("note1")
	if len(tags) != 0 {
		t.Errorf("expected 0 tags for note after deletion, got %d", len(tags))
	}
}

func TestListTags(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateTag(&Tag{ID: "tag1", Name: "B标签"})
	cn.CreateTag(&Tag{ID: "tag2", Name: "A标签"})

	tags := cn.ListTags()
	if len(tags) != 2 {
		t.Fatalf("expected 2 tags, got %d", len(tags))
	}

	if tags[0].Name != "A标签" {
		t.Errorf("expected A标签 first, got %s", tags[0].Name)
	}
}

func TestAddNoteTag(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateTag(&Tag{ID: "tag1", Name: "重要"})
	cn.CreateNote(&Note{ID: "note1", Title: "Test", Author: "user1"})

	err := cn.AddNoteTag("note1", "tag1")
	if err != nil {
		t.Fatalf("AddNoteTag failed: %v", err)
	}

	tags := cn.GetNoteTags("note1")
	if len(tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(tags))
	}

	tag, _ := cn.GetTag("tag1")
	if tag.NoteCount != 1 {
		t.Errorf("expected note count 1, got %d", tag.NoteCount)
	}
}

func TestRemoveNoteTag(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateTag(&Tag{ID: "tag1", Name: "重要"})
	cn.CreateNote(&Note{ID: "note1", Title: "Test", Author: "user1"})
	cn.AddNoteTag("note1", "tag1")

	err := cn.RemoveNoteTag("note1", "tag1")
	if err != nil {
		t.Fatalf("RemoveNoteTag failed: %v", err)
	}

	tags := cn.GetNoteTags("note1")
	if len(tags) != 0 {
		t.Errorf("expected 0 tags, got %d", len(tags))
	}

	tag, _ := cn.GetTag("tag1")
	if tag.NoteCount != 0 {
		t.Errorf("expected note count 0, got %d", tag.NoteCount)
	}
}

func TestGetVersions(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "v1", Content: "content1", Author: "user1"})

	note := &Note{ID: "note1", Title: "v2", Content: "content2", Version: 1}
	cn.UpdateNote(note, "user1", "更新到v2")

	versions, err := cn.GetVersions("note1")
	if err != nil {
		t.Fatalf("GetVersions failed: %v", err)
	}

	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}

	// 版本应该按降序排列
	if versions[0].Version != 2 {
		t.Errorf("expected version 2 first, got %d", versions[0].Version)
	}
}

func TestGetVersion(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "v1", Content: "content1", Author: "user1"})

	version, err := cn.GetVersion("note1", 1)
	if err != nil {
		t.Fatalf("GetVersion failed: %v", err)
	}

	if version.Title != "v1" {
		t.Errorf("expected v1, got %s", version.Title)
	}

	_, err = cn.GetVersion("note1", 999)
	if err == nil {
		t.Error("expected error for non-existent version")
	}
}

func TestRevertToVersion(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "v1", Content: "content1", Author: "user1"})

	note := &Note{ID: "note1", Title: "v2", Content: "content2", Version: 1}
	cn.UpdateNote(note, "user1", "更新到v2")

	err := cn.RevertToVersion("note1", 1, "user1")
	if err != nil {
		t.Fatalf("RevertToVersion failed: %v", err)
	}

	reverted, _ := cn.GetNote("note1")
	if reverted.Title != "v1" {
		t.Errorf("expected v1, got %s", reverted.Title)
	}
	if reverted.Version != 3 {
		t.Errorf("expected version 3, got %d", reverted.Version)
	}
}

func TestStartCollaboration(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "Test", Author: "user1"})

	session, err := cn.StartCollaboration("note1", "user2")
	if err != nil {
		t.Fatalf("StartCollaboration failed: %v", err)
	}

	if !session.IsActive {
		t.Error("expected session to be active")
	}

	note, _ := cn.GetNote("note1")
	if !contains(note.Collaborators, "user2") {
		t.Error("expected user2 to be in collaborators")
	}
}

func TestEndCollaboration(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "Test", Author: "user1"})
	session, _ := cn.StartCollaboration("note1", "user2")

	err := cn.EndCollaboration(session.ID)
	if err != nil {
		t.Fatalf("EndCollaboration failed: %v", err)
	}

	note, _ := cn.GetNote("note1")
	if contains(note.Collaborators, "user2") {
		t.Error("expected user2 to be removed from collaborators")
	}
}

func TestPingCollaboration(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "Test", Author: "user1"})
	session, _ := cn.StartCollaboration("note1", "user2")

	err := cn.PingCollaboration(session.ID)
	if err != nil {
		t.Fatalf("PingCollaboration failed: %v", err)
	}
}

func TestGetActiveCollaborators(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "Test", Author: "user1"})
	cn.StartCollaboration("note1", "user2")
	cn.StartCollaboration("note1", "user3")

	collabs := cn.GetActiveCollaborators("note1")
	if len(collabs) != 2 {
		t.Errorf("expected 2 active collaborators, got %d", len(collabs))
	}
}

func TestSubmitEdit(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "Test", Content: "Hello World", Author: "user1"})

	op := &EditOperation{
		ID:       "op1",
		NoteID:   "note1",
		UserID:   "user2",
		Type:     "insert",
		Position: 5,
		Content:  " Beautiful",
	}

	err := cn.SubmitEdit(op)
	if err != nil {
		t.Fatalf("SubmitEdit failed: %v", err)
	}

	note, _ := cn.GetNote("note1")
	if note.Content != "Hello Beautiful World" {
		t.Errorf("expected 'Hello Beautiful World', got '%s'", note.Content)
	}
}

func TestSubmitEditDelete(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "Test", Content: "Hello World", Author: "user1"})

	op := &EditOperation{
		ID:       "op1",
		NoteID:   "note1",
		UserID:   "user2",
		Type:     "delete",
		Position: 5,
		Length:   6,
	}

	err := cn.SubmitEdit(op)
	if err != nil {
		t.Fatalf("SubmitEdit failed: %v", err)
	}

	note, _ := cn.GetNote("note1")
	if note.Content != "Hello" {
		t.Errorf("expected 'Hello', got '%s'", note.Content)
	}
}

func TestSubmitEditReplace(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "Test", Content: "Hello World", Author: "user1"})

	op := &EditOperation{
		ID:       "op1",
		NoteID:   "note1",
		UserID:   "user2",
		Type:     "replace",
		Position: 6,
		Length:   5,
		Content:  "Go",
	}

	err := cn.SubmitEdit(op)
	if err != nil {
		t.Fatalf("SubmitEdit failed: %v", err)
	}

	note, _ := cn.GetNote("note1")
	if note.Content != "Hello Go" {
		t.Errorf("expected 'Hello Go', got '%s'", note.Content)
	}
}

func TestSearchNotes(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "Go语言教程", Content: "学习Go语言的基础知识", Author: "user1"})
	cn.CreateNote(&Note{ID: "note2", Title: "Python入门", Content: "Python编程基础", Author: "user1"})
	cn.CreateNote(&Note{ID: "note3", Title: "Go并发编程", Content: "Goroutine和Channel", Author: "user1"})

	results := cn.SearchNotes("Go", "user1")
	if len(results) != 2 {
		t.Errorf("expected 2 results for 'Go', got %d", len(results))
	}

	results = cn.SearchNotes("Python", "user1")
	if len(results) != 1 {
		t.Errorf("expected 1 result for 'Python', got %d", len(results))
	}
}

func TestSearchNotesWithTags(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateTag(&Tag{ID: "tag1", Name: "编程"})
	cn.CreateNote(&Note{ID: "note1", Title: "Test", Author: "user1"})
	cn.AddNoteTag("note1", "tag1")

	results := cn.SearchNotes("编程", "user1")
	if len(results) != 1 {
		t.Errorf("expected 1 result, got %d", len(results))
	}
}

func TestExportNote(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateTag(&Tag{ID: "tag1", Name: "重要"})
	err := cn.CreateNote(&Note{
		ID:       "note1",
		Title:    "测试笔记",
		Content:  "这是内容",
		Markdown: "# 测试\n这是内容",
		Author:   "user1",
		Tags:     []string{"tag1"},
	})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}
	cn.AddNoteTag("note1", "tag1")

	// 导出为Markdown
	md, err := cn.ExportNote("note1", FormatMarkdown)
	if err != nil {
		t.Fatalf("ExportNote failed: %v", err)
	}

	if !strings.Contains(md, "测试笔记") {
		t.Error("expected markdown export to contain title")
	}

	// 导出为HTML
	html, err := cn.ExportNote("note1", FormatHTML)
	if err != nil {
		t.Fatalf("ExportNote failed: %v", err)
	}

	if !strings.Contains(html, "<!DOCTYPE html>") {
		t.Error("expected HTML export to be valid HTML")
	}

	// 不支持的格式
	_, err = cn.ExportNote("note1", "invalid")
	if err == nil {
		t.Error("expected error for unsupported format")
	}
}

func TestImportNote(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNotebook(&Notebook{ID: "nb1", Name: "Test", OwnerID: "user1"})

	mdContent := "# 导入的笔记\n这是从Markdown导入的内容"

	note, err := cn.ImportNote("nb1", mdContent, ImportMarkdown, "user1")
	if err != nil {
		t.Fatalf("ImportNote failed: %v", err)
	}

	if note.Title != "导入的笔记" {
		t.Errorf("expected 导入的笔记, got %s", note.Title)
	}

	if note.Content != "这是从Markdown导入的内容" {
		t.Errorf("expected content to be imported")
	}

	// 测试HTML导入
	htmlContent := "<h1>HTML笔记</h1><p>内容</p>"
	note, err = cn.ImportNote("nb1", htmlContent, ImportHTML, "user1")
	if err != nil {
		t.Fatalf("ImportNote failed: %v", err)
	}

	if note.Title != "导入的HTML笔记" {
		t.Errorf("expected 导入的HTML笔记, got %s", note.Title)
	}
}

func TestImportNoteInvalidNotebook(t *testing.T) {
	cn := NewCollaborativeNotes()

	_, err := cn.ImportNote("nonexistent", "content", ImportMarkdown, "user1")
	if err == nil {
		t.Error("expected error for non-existent notebook")
	}
}

func TestGetStats(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateTag(&Tag{ID: "tag1", Name: "Tag1"})
	cn.CreateTag(&Tag{ID: "tag2", Name: "Tag2"})
	err := cn.CreateNote(&Note{ID: "note1", Title: "Note 1", Author: "user1"})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}
	err = cn.CreateNote(&Note{ID: "note2", Title: "Note 2", Author: "user1"})
	if err != nil {
		t.Fatalf("CreateNote failed: %v", err)
	}

	note := &Note{ID: "note1", Title: "Updated", Version: 1}
	cn.UpdateNote(note, "user1", "update")

	stats := cn.GetStats("user1")
	if stats.TotalNotes != 2 {
		t.Errorf("expected 2 notes, got %d", stats.TotalNotes)
	}
	if stats.TotalVersions != 3 {
		t.Errorf("expected 3 versions, got %d", stats.TotalVersions)
	}
	if stats.TotalTags != 2 {
		t.Errorf("expected 2 tags, got %d", stats.TotalTags)
	}
}

func TestGetEditOperations(t *testing.T) {
	cn := NewCollaborativeNotes()

	cn.CreateNote(&Note{ID: "note1", Title: "Test", Content: "Hello", Author: "user1"})

	cn.SubmitEdit(&EditOperation{
		ID:       "op1",
		NoteID:   "note1",
		UserID:   "user2",
		Type:     "insert",
		Position: 5,
		Content:  " World",
	})

	ops := cn.GetEditOperations("note1")
	if len(ops) != 1 {
		t.Errorf("expected 1 operation, got %d", len(ops))
	}
}
