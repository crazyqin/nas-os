package docworkspace

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
	tmpls := m.ListTemplates()
	if len(tmpls) < 3 {
		t.Errorf("expected at least 3 default templates, got %d", len(tmpls))
	}
}

func TestCreateAndGetDoc(t *testing.T) {
	m := NewManager()

	err := m.CreateDoc(&Document{
		ID: "doc-1", Title: "测试文档", Content: "Hello World",
		Author: "user1", Category: "通用",
	})
	if err != nil {
		t.Fatalf("create doc failed: %v", err)
	}

	doc := m.GetDoc("doc-1")
	if doc == nil {
		t.Fatal("expected doc")
	}
	if doc.Title != "测试文档" {
		t.Errorf("expected '测试文档', got '%s'", doc.Title)
	}
	if doc.Version != 1 {
		t.Errorf("expected version 1, got %d", doc.Version)
	}

	// 重复创建应报错
	err = m.CreateDoc(&Document{ID: "doc-1", Title: "重复"})
	if err == nil {
		t.Error("expected error for duplicate doc")
	}

	// 空 ID 应报错
	err = m.CreateDoc(&Document{Title: "空ID"})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestUpdateDoc(t *testing.T) {
	m := NewManager()

	m.CreateDoc(&Document{ID: "doc-1", Title: "文档", Content: "v1", Author: "user1"})

	err := m.UpdateDoc("doc-1", "v2 内容", "user2")
	if err != nil {
		t.Fatalf("update doc failed: %v", err)
	}

	doc := m.GetDoc("doc-1")
	if doc.Version != 2 {
		t.Errorf("expected version 2, got %d", doc.Version)
	}
	if doc.Content != "v2 内容" {
		t.Errorf("expected 'v2 内容', got '%s'", doc.Content)
	}

	// 不存在的文档
	err = m.UpdateDoc("nonexistent", "x", "user1")
	if err == nil {
		t.Error("expected error for nonexistent doc")
	}
}

func TestDeleteDoc(t *testing.T) {
	m := NewManager()

	m.CreateDoc(&Document{ID: "doc-1", Title: "文档", Content: "内容", Author: "user1"})

	err := m.DeleteDoc("doc-1")
	if err != nil {
		t.Fatalf("delete doc failed: %v", err)
	}

	if m.GetDoc("doc-1") != nil {
		t.Error("expected nil after delete")
	}

	// 不存在的文档
	err = m.DeleteDoc("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent doc")
	}
}

func TestListDocs(t *testing.T) {
	m := NewManager()

	m.CreateDoc(&Document{ID: "doc-1", Title: "技术文档", Content: "", Author: "u1", Category: "技术", Tags: []string{"go"}})
	m.CreateDoc(&Document{ID: "doc-2", Title: "产品文档", Content: "", Author: "u1", Category: "产品", Tags: []string{"prd"}})
	m.CreateDoc(&Document{ID: "doc-3", Title: "Go 入门", Content: "", Author: "u1", Category: "技术", Tags: []string{"go", "教程"}})

	// 按分类过滤
	docs := m.ListDocs("技术", nil)
	if len(docs) != 2 {
		t.Errorf("expected 2 tech docs, got %d", len(docs))
	}

	// 按标签过滤
	docs = m.ListDocs("", []string{"prd"})
	if len(docs) != 1 {
		t.Errorf("expected 1 prd doc, got %d", len(docs))
	}

	// 全部
	docs = m.ListDocs("", nil)
	if len(docs) != 3 {
		t.Errorf("expected 3 docs, got %d", len(docs))
	}
}

func TestVersionHistory(t *testing.T) {
	m := NewManager()

	m.CreateDoc(&Document{ID: "doc-1", Title: "文档", Content: "v1", Author: "user1"})
	m.UpdateDoc("doc-1", "v2", "user1")
	m.UpdateDoc("doc-1", "v3", "user2")

	versions := m.GetVersions("doc-1")
	if len(versions) != 3 {
		t.Fatalf("expected 3 versions, got %d", len(versions))
	}

	// 回退到版本 1
	err := m.RevertToVersion("doc-1", 1, "user3")
	if err != nil {
		t.Fatalf("revert failed: %v", err)
	}

	doc := m.GetDoc("doc-1")
	if doc.Content != "v1" {
		t.Errorf("expected 'v1', got '%s'", doc.Content)
	}
	if doc.Version != 4 {
		t.Errorf("expected version 4, got %d", doc.Version)
	}

	// 回退到不存在的版本
	err = m.RevertToVersion("doc-1", 99, "user1")
	if err == nil {
		t.Error("expected error for nonexistent version")
	}
}

func TestComments(t *testing.T) {
	m := NewManager()

	m.CreateDoc(&Document{ID: "doc-1", Title: "文档", Content: "内容", Author: "user1"})

	err := m.AddComment(&Comment{DocID: "doc-1", Author: "user2", Content: "写得好", Position: 10})
	if err != nil {
		t.Fatalf("add comment failed: %v", err)
	}

	err = m.AddComment(&Comment{DocID: "doc-1", Author: "user3", Content: "需要修改", Position: 20})
	if err != nil {
		t.Fatalf("add comment failed: %v", err)
	}

	comments := m.GetComments("doc-1")
	if len(comments) != 2 {
		t.Fatalf("expected 2 comments, got %d", len(comments))
	}
	if comments[0].ID == "" {
		t.Error("expected comment ID to be set")
	}

	// 不存在的文档
	err = m.AddComment(&Comment{DocID: "nonexistent", Author: "u1", Content: "x"})
	if err == nil {
		t.Error("expected error for nonexistent doc")
	}
}

func TestSearchDocs(t *testing.T) {
	m := NewManager()

	m.CreateDoc(&Document{ID: "doc-1", Title: "Go 语言教程", Content: "这是一份 Go 语言的学习资料", Author: "u1"})
	m.CreateDoc(&Document{ID: "doc-2", Title: "Python 入门", Content: "Python 编程基础", Author: "u1"})
	m.CreateDoc(&Document{ID: "doc-3", Title: "Go 并发编程", Content: "Goroutine 和 Channel", Author: "u1"})

	results := m.SearchDocs("Go")
	if len(results) < 2 {
		t.Errorf("expected at least 2 results, got %d", len(results))
	}

	// 无匹配
	results = m.SearchDocs("Rust")
	if len(results) != 0 {
		t.Errorf("expected 0 results, got %d", len(results))
	}
}

func TestPermissions(t *testing.T) {
	m := NewManager()

	m.CreateDoc(&Document{ID: "doc-1", Title: "文档", Content: "内容", Author: "user1"})

	err := m.SetPermission("doc-1", "user2", "edit")
	if err != nil {
		t.Fatalf("set permission failed: %v", err)
	}

	// 更新已有权限
	err = m.SetPermission("doc-1", "user2", "admin")
	if err != nil {
		t.Fatalf("update permission failed: %v", err)
	}

	// 无效权限
	err = m.SetPermission("doc-1", "user3", "invalid")
	if err == nil {
		t.Error("expected error for invalid permission")
	}

	// 不存在的文档
	err = m.SetPermission("nonexistent", "user1", "view")
	if err == nil {
		t.Error("expected error for nonexistent doc")
	}
}

func TestTemplates(t *testing.T) {
	m := NewManager()

	tmpls := m.ListTemplates()
	if len(tmpls) < 3 {
		t.Errorf("expected at least 3 templates, got %d", len(tmpls))
	}

	err := m.CreateTemplate(&Template{
		ID: "tpl-custom", Name: "自定义", Content: "# 自定义模板", Category: "通用",
	})
	if err != nil {
		t.Fatalf("create template failed: %v", err)
	}

	tmpls = m.ListTemplates()
	if len(tmpls) < 4 {
		t.Errorf("expected at least 4 templates, got %d", len(tmpls))
	}

	// 空 ID
	err = m.CreateTemplate(&Template{Name: "空"})
	if err == nil {
		t.Error("expected error for empty ID")
	}
}

func TestExportDoc(t *testing.T) {
	m := NewManager()

	m.CreateDoc(&Document{ID: "doc-1", Title: "测试", Content: "Hello", Author: "user1"})

	// Markdown 导出
	data, err := m.ExportDoc("doc-1", "markdown")
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if string(data) != "Hello" {
		t.Errorf("expected 'Hello', got '%s'", string(data))
	}

	// HTML 导出
	data, err = m.ExportDoc("doc-1", "html")
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if len(data) == 0 {
		t.Error("expected non-empty HTML")
	}

	// 不支持的格式
	_, err = m.ExportDoc("doc-1", "xlsx")
	if err == nil {
		t.Error("expected error for unsupported format")
	}

	// 不存在的文档
	_, err = m.ExportDoc("nonexistent", "md")
	if err == nil {
		t.Error("expected error for nonexistent doc")
	}
}
