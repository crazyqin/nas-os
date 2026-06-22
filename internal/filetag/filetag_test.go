package filetag

import (
	"testing"
)

func TestCreateTag(t *testing.T) {
	manager := NewManager()
	
	tag, err := manager.CreateTag("重要", "#FF0000", "重要文件", "工作", "admin")
	if err != nil {
		t.Fatalf("创建标签失败: %v", err)
	}
	
	if tag.Name != "重要" {
		t.Errorf("期望标签名 '重要'，实际 '%s'", tag.Name)
	}
	if tag.Color != "#FF0000" {
		t.Errorf("期望颜色 '#FF0000'，实际 '%s'", tag.Color)
	}
	if tag.Category != "工作" {
		t.Errorf("期望分类 '工作'，实际 '%s'", tag.Category)
	}
}

func TestCreateDuplicateTag(t *testing.T) {
	manager := NewManager()
	
	_, err := manager.CreateTag("测试", "#000000", "", "", "admin")
	if err != nil {
		t.Fatalf("第一次创建标签失败: %v", err)
	}
	
	_, err = manager.CreateTag("测试", "#FFFFFF", "", "", "admin")
	if err == nil {
		t.Error("期望返回重复错误，但没有")
	}
}

func TestTagFile(t *testing.T) {
	manager := NewManager()
	
	tag, _ := manager.CreateTag("照片", "#00FF00", "照片文件", "媒体", "admin")
	
	ft, err := manager.TagFile("/photos/vacation.jpg", tag.ID, "user1", "度假照片")
	if err != nil {
		t.Fatalf("为文件添加标签失败: %v", err)
	}
	
	if ft.FilePath != "/photos/vacation.jpg" {
		t.Errorf("期望文件路径 '/photos/vacation.jpg'，实际 '%s'", ft.FilePath)
	}
	if ft.TagID != tag.ID {
		t.Errorf("期望标签ID '%s'，实际 '%s'", tag.ID, ft.TagID)
	}
}

func TestTagFileDuplicate(t *testing.T) {
	manager := NewManager()
	
	tag, _ := manager.CreateTag("测试", "#000000", "", "", "admin")
	manager.TagFile("/test.txt", tag.ID, "user1", "")
	
	_, err := manager.TagFile("/test.txt", tag.ID, "user1", "")
	if err == nil {
		t.Error("期望返回重复错误，但没有")
	}
}

func TestUntagFile(t *testing.T) {
	manager := NewManager()
	
	tag, _ := manager.CreateTag("临时", "#FFFF00", "", "", "admin")
	manager.TagFile("/temp.txt", tag.ID, "user1", "")
	
	err := manager.UntagFile("/temp.txt", tag.ID)
	if err != nil {
		t.Fatalf("移除标签失败: %v", err)
	}
	
	tags := manager.GetFileTags("/temp.txt")
	if len(tags) != 0 {
		t.Errorf("期望没有标签，实际有 %d 个", len(tags))
	}
}

func TestSearchFiles(t *testing.T) {
	manager := NewManager()
	
	tag1, _ := manager.CreateTag("工作", "#0000FF", "", "分类", "admin")
	tag2, _ := manager.CreateTag("重要", "#FF0000", "", "分类", "admin")
	
	manager.TagFile("/work/report.docx", tag1.ID, "user1", "")
	manager.TagFile("/work/report.docx", tag2.ID, "user1", "")
	manager.TagFile("/personal/photo.jpg", tag2.ID, "user1", "")
	
	// 搜索包含"工作"标签的文件
	result := manager.SearchFiles(&SearchRequest{
		Tags: []string{tag1.ID},
	})
	if result.Total != 2 { // 该文件有两个标签记录
		t.Errorf("期望2个结果，实际 %d", result.Total)
	}
	
	// 搜索包含"重要"标签的文件
	result = manager.SearchFiles(&SearchRequest{
		Tags: []string{tag2.ID},
	})
	if result.Total != 3 { // 两个文件，但一个文件有两个标签
		t.Errorf("期望3个结果，实际 %d", result.Total)
	}
	
	// AND搜索：同时包含两个标签
	result = manager.SearchFiles(&SearchRequest{
		Tags:     []string{tag1.ID, tag2.ID},
		Operator: "and",
	})
	if result.Total != 2 { // 一个文件，两个标签记录
		t.Errorf("AND搜索期望2个结果，实际 %d", result.Total)
	}
	
	// OR搜索：包含任一标签
	result = manager.SearchFiles(&SearchRequest{
		Tags:     []string{tag1.ID, tag2.ID},
		Operator: "or",
	})
	if result.Total != 3 { // 两个文件，三个标签记录
		t.Errorf("OR搜索期望3个结果，实际 %d", result.Total)
	}
}

func TestBatchTag(t *testing.T) {
	manager := NewManager()
	
	tag, _ := manager.CreateTag("批量", "#00FFFF", "", "", "admin")
	
	req := &BatchTagRequest{
		FilePaths: []string{"/a.txt", "/b.txt", "/c.txt"},
		TagIDs:    []string{tag.ID},
		TaggedBy:  "user1",
	}
	
	results, err := manager.BatchTag(req)
	if err != nil {
		t.Fatalf("批量打标签失败: %v", err)
	}
	
	if len(results) != 3 {
		t.Errorf("期望3个结果，实际 %d", len(results))
	}
}

func TestDeleteTag(t *testing.T) {
	manager := NewManager()
	
	tag, _ := manager.CreateTag("待删除", "#000000", "", "", "admin")
	manager.TagFile("/test.txt", tag.ID, "user1", "")
	
	err := manager.DeleteTag(tag.ID)
	if err != nil {
		t.Fatalf("删除标签失败: %v", err)
	}
	
	// 验证标签已删除
	_, err = manager.GetTag(tag.ID)
	if err == nil {
		t.Error("期望标签不存在，但找到了")
	}
	
	// 验证文件标签已移除
	tags := manager.GetFileTags("/test.txt")
	if len(tags) != 0 {
		t.Errorf("期望没有标签，实际有 %d 个", len(tags))
	}
}

func TestGetCategories(t *testing.T) {
	manager := NewManager()
	
	manager.CreateTag("标签1", "#000", "", "分类A", "admin")
	manager.CreateTag("标签2", "#000", "", "分类A", "admin")
	manager.CreateTag("标签3", "#000", "", "分类B", "admin")
	
	categories := manager.GetCategories()
	if len(categories) != 2 {
		t.Errorf("期望2个分类，实际 %d", len(categories))
	}
}