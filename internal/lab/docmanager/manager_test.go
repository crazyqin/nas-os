package docmanager

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	assert.NotNil(t, m)
}

func TestCreateDocument(t *testing.T) {
	m := NewManager()

	req := CreateDocumentRequest{
		Title:    "测试文档",
		Content:  "这是一个测试文档",
		Tags:     []string{"tag1", "tag2"},
		Category: "通用",
		MimeType: "text/plain",
		Size:     100,
	}

	doc, err := m.CreateDocument(req)
	require.NoError(t, err)
	assert.NotEmpty(t, doc.ID)
	assert.Equal(t, "测试文档", doc.Title)
	assert.Equal(t, "这是一个测试文档", doc.Content)
	assert.Equal(t, []string{"tag1", "tag2"}, doc.Tags)
	assert.Equal(t, "通用", doc.Category)
	assert.Equal(t, "text/plain", doc.MimeType)
	assert.Equal(t, int64(100), doc.Size)
}

func TestCreateDocumentEmptyTitle(t *testing.T) {
	m := NewManager()

	req := CreateDocumentRequest{
		Title: "",
	}

	_, err := m.CreateDocument(req)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "文档标题不能为空")
}

func TestGetDocument(t *testing.T) {
	m := NewManager()

	req := CreateDocumentRequest{
		Title:   "获取测试",
		Content: "内容",
	}
	created, err := m.CreateDocument(req)
	require.NoError(t, err)

	doc, err := m.GetDocument(created.ID)
	require.NoError(t, err)
	assert.Equal(t, created.ID, doc.ID)
	assert.Equal(t, "获取测试", doc.Title)
}

func TestGetDocumentNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.GetDocument("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "文档不存在")
}

func TestUpdateDocument(t *testing.T) {
	m := NewManager()

	req := CreateDocumentRequest{
		Title:   "原始标题",
		Content: "原始内容",
	}
	created, err := m.CreateDocument(req)
	require.NoError(t, err)

	updateReq := UpdateDocumentRequest{
		Title:   "新标题",
		Content: "新内容",
	}
	updated, err := m.UpdateDocument(created.ID, updateReq)
	require.NoError(t, err)
	assert.Equal(t, "新标题", updated.Title)
	assert.Equal(t, "新内容", updated.Content)
}

func TestUpdateDocumentNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.UpdateDocument("nonexistent", UpdateDocumentRequest{Title: "新"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "文档不存在")
}

func TestDeleteDocument(t *testing.T) {
	m := NewManager()

	req := CreateDocumentRequest{Title: "删除测试"}
	created, err := m.CreateDocument(req)
	require.NoError(t, err)

	err = m.DeleteDocument(created.ID)
	require.NoError(t, err)

	_, err = m.GetDocument(created.ID)
	assert.Error(t, err)
}

func TestDeleteDocumentNotFound(t *testing.T) {
	m := NewManager()

	err := m.DeleteDocument("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "文档不存在")
}

func TestListDocuments(t *testing.T) {
	m := NewManager()

	// 创建多个文档
	for i := 0; i < 15; i++ {
		_, err := m.CreateDocument(CreateDocumentRequest{Title: "文档"})
		require.NoError(t, err)
	}

	// 第一页
	docs, total, err := m.ListDocuments(1, 10)
	require.NoError(t, err)
	assert.Equal(t, 15, total)
	assert.Len(t, docs, 10)

	// 第二页
	docs, total, err = m.ListDocuments(2, 10)
	require.NoError(t, err)
	assert.Equal(t, 15, total)
	assert.Len(t, docs, 5)
}

func TestListDocumentsEmpty(t *testing.T) {
	m := NewManager()

	docs, total, err := m.ListDocuments(1, 10)
	require.NoError(t, err)
	assert.Equal(t, 0, total)
	assert.Empty(t, docs)
}

func TestSearchDocuments(t *testing.T) {
	m := NewManager()

	_, _ = m.CreateDocument(CreateDocumentRequest{
		Title:   "合同文件",
		Content: "甲方乙方签订合同",
	})
	_, _ = m.CreateDocument(CreateDocumentRequest{
		Title:   "报告文件",
		Content: "年度总结报告",
	})
	_, _ = m.CreateDocument(CreateDocumentRequest{
		Title:   "发票",
		Content: "金额1000元",
	})

	// 搜索关键词
	result, err := m.SearchDocuments(SearchQuery{Query: "合同"})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, "合同文件", result.Documents[0].Title)

	// 搜索另一个关键词
	result, err = m.SearchDocuments(SearchQuery{Query: "报告"})
	require.NoError(t, err)
	assert.Equal(t, 1, result.Total)
	assert.Equal(t, "报告文件", result.Documents[0].Title)
}

func TestSearchDocumentsNoMatch(t *testing.T) {
	m := NewManager()

	_, _ = m.CreateDocument(CreateDocumentRequest{Title: "测试"})

	result, err := m.SearchDocuments(SearchQuery{Query: "不存在"})
	require.NoError(t, err)
	assert.Equal(t, 0, result.Total)
	assert.Empty(t, result.Documents)
}

func TestCreateAndGetTags(t *testing.T) {
	m := NewManager()

	tag, err := m.CreateTag(CreateTagRequest{Name: "重要", Color: "#ff0000"})
	require.NoError(t, err)
	assert.NotEmpty(t, tag.ID)
	assert.Equal(t, "重要", tag.Name)
	assert.Equal(t, "#ff0000", tag.Color)

	tags, err := m.GetTags()
	require.NoError(t, err)
	assert.Len(t, tags, 1)
	assert.Equal(t, "重要", tags[0].Name)
}

func TestCreateTagEmptyName(t *testing.T) {
	m := NewManager()

	_, err := m.CreateTag(CreateTagRequest{Name: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "标签名称不能为空")
}

func TestCreateTagDuplicate(t *testing.T) {
	m := NewManager()

	_, err := m.CreateTag(CreateTagRequest{Name: "重复"})
	require.NoError(t, err)

	_, err = m.CreateTag(CreateTagRequest{Name: "重复"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "标签已存在")
}

func TestCreateAndGetCategories(t *testing.T) {
	m := NewManager()

	cat, err := m.CreateCategory(CreateCategoryRequest{
		Name:        "合同",
		Description: "合同类文档",
	})
	require.NoError(t, err)
	assert.NotEmpty(t, cat.ID)
	assert.Equal(t, "合同", cat.Name)

	cats, err := m.GetCategories()
	require.NoError(t, err)
	assert.Len(t, cats, 1)
	assert.Equal(t, "合同", cats[0].Name)
}

func TestCreateCategoryEmptyName(t *testing.T) {
	m := NewManager()

	_, err := m.CreateCategory(CreateCategoryRequest{Name: ""})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "分类名称不能为空")
}

func TestCreateCategoryDuplicate(t *testing.T) {
	m := NewManager()

	_, err := m.CreateCategory(CreateCategoryRequest{Name: "重复"})
	require.NoError(t, err)

	_, err = m.CreateCategory(CreateCategoryRequest{Name: "重复"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "分类已存在")
}

func TestCreateCategoryWithParent(t *testing.T) {
	m := NewManager()

	parent, err := m.CreateCategory(CreateCategoryRequest{Name: "父分类"})
	require.NoError(t, err)

	child, err := m.CreateCategory(CreateCategoryRequest{
		Name:     "子分类",
		ParentID: parent.ID,
	})
	require.NoError(t, err)
	assert.Equal(t, parent.ID, child.ParentID)
}

func TestCreateCategoryInvalidParent(t *testing.T) {
	m := NewManager()

	_, err := m.CreateCategory(CreateCategoryRequest{
		Name:     "子分类",
		ParentID: "nonexistent",
	})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "父分类不存在")
}

func TestAddAndRemoveTag(t *testing.T) {
	m := NewManager()

	tag, _ := m.CreateTag(CreateTagRequest{Name: "测试标签", Color: "#00ff00"})
	doc, _ := m.CreateDocument(CreateDocumentRequest{Title: "标签测试"})

	// 添加标签
	err := m.AddTag(doc.ID, tag.ID)
	require.NoError(t, err)

	updated, _ := m.GetDocument(doc.ID)
	assert.Contains(t, updated.Tags, "测试标签")

	// 重复添加不报错
	err = m.AddTag(doc.ID, tag.ID)
	require.NoError(t, err)

	// 移除标签
	err = m.RemoveTag(doc.ID, tag.ID)
	require.NoError(t, err)

	updated, _ = m.GetDocument(doc.ID)
	assert.NotContains(t, updated.Tags, "测试标签")
}

func TestAddTagDocumentNotFound(t *testing.T) {
	m := NewManager()

	tag, _ := m.CreateTag(CreateTagRequest{Name: "test"})

	err := m.AddTag("nonexistent", tag.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "文档不存在")
}

func TestAddTagTagNotFound(t *testing.T) {
	m := NewManager()

	doc, _ := m.CreateDocument(CreateDocumentRequest{Title: "test"})

	err := m.AddTag(doc.ID, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "标签不存在")
}

func TestRemoveTagNotAssigned(t *testing.T) {
	m := NewManager()

	tag, _ := m.CreateTag(CreateTagRequest{Name: "test"})
	doc, _ := m.CreateDocument(CreateDocumentRequest{Title: "test"})

	err := m.RemoveTag(doc.ID, tag.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "未包含标签")
}

func TestSetCategory(t *testing.T) {
	m := NewManager()

	cat, _ := m.CreateCategory(CreateCategoryRequest{Name: "新分类"})
	doc, _ := m.CreateDocument(CreateDocumentRequest{Title: "分类测试"})

	err := m.SetCategory(doc.ID, cat.ID)
	require.NoError(t, err)

	updated, _ := m.GetDocument(doc.ID)
	assert.Equal(t, "新分类", updated.Category)
}

func TestSetCategoryDocNotFound(t *testing.T) {
	m := NewManager()

	cat, _ := m.CreateCategory(CreateCategoryRequest{Name: "test"})

	err := m.SetCategory("nonexistent", cat.ID)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "文档不存在")
}

func TestSetCategoryCatNotFound(t *testing.T) {
	m := NewManager()

	doc, _ := m.CreateDocument(CreateDocumentRequest{Title: "test"})

	err := m.SetCategory(doc.ID, "nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "分类不存在")
}

func TestProcessOCR(t *testing.T) {
	m := NewManager()

	doc, _ := m.CreateDocument(CreateDocumentRequest{
		Title:   "OCR测试文档",
		Content: "测试内容",
	})

	result, err := m.ProcessOCR(doc.ID)
	require.NoError(t, err)
	assert.NotEmpty(t, result.Text)
	assert.Greater(t, result.Confidence, 0.0)
	assert.Equal(t, "zh-CN", result.Language)
	assert.Equal(t, 1, result.Pages)

	// 验证文档OCR文本已更新
	updated, _ := m.GetDocument(doc.ID)
	assert.NotEmpty(t, updated.OCRText)
}

func TestProcessOCRDocNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.ProcessOCR("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "文档不存在")
}

func TestAutoClassify(t *testing.T) {
	m := NewManager()

	// 合同类
	doc1, _ := m.CreateDocument(CreateDocumentRequest{
		Title:   "采购合同",
		Content: "甲方与乙方签订合同",
	})
	cat1, err := m.AutoClassify(doc1.ID)
	require.NoError(t, err)
	assert.Equal(t, "合同", cat1)

	// 报告类
	doc2, _ := m.CreateDocument(CreateDocumentRequest{
		Title:   "年度报告",
		Content: "总结结论如下",
	})
	cat2, err := m.AutoClassify(doc2.ID)
	require.NoError(t, err)
	assert.Equal(t, "报告", cat2)

	// PDF类
	doc3, _ := m.CreateDocument(CreateDocumentRequest{
		Title:    "文档.pdf",
		MimeType: "application/pdf",
	})
	cat3, err := m.AutoClassify(doc3.ID)
	require.NoError(t, err)
	assert.Equal(t, "PDF文档", cat3)

	// 通用类
	doc4, _ := m.CreateDocument(CreateDocumentRequest{
		Title: "其他文档",
	})
	cat4, err := m.AutoClassify(doc4.ID)
	require.NoError(t, err)
	assert.Equal(t, "通用", cat4)
}

func TestAutoClassifyDocNotFound(t *testing.T) {
	m := NewManager()

	_, err := m.AutoClassify("nonexistent")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "文档不存在")
}

func TestGetStats(t *testing.T) {
	m := NewManager()

	// 创建一些数据
	m.CreateDocument(CreateDocumentRequest{Title: "doc1", Category: "合同", MimeType: "text/plain", Size: 100})
	m.CreateDocument(CreateDocumentRequest{Title: "doc2", Category: "合同", MimeType: "application/pdf", Size: 200})
	m.CreateDocument(CreateDocumentRequest{Title: "doc3", Category: "报告", MimeType: "text/plain", Size: 300})
	m.CreateCategory(CreateCategoryRequest{Name: "合同"})
	m.CreateCategory(CreateCategoryRequest{Name: "报告"})
	m.CreateTag(CreateTagRequest{Name: "重要"})

	stats, err := m.GetStats()
	require.NoError(t, err)

	assert.Equal(t, 3, stats["total_documents"])
	assert.Equal(t, 2, stats["total_categories"])
	assert.Equal(t, 1, stats["total_tags"])
	assert.Equal(t, int64(600), stats["total_size"])

	catCount := stats["documents_by_category"].(map[string]int)
	assert.Equal(t, 2, catCount["合同"])
	assert.Equal(t, 1, catCount["报告"])

	mimeCount := stats["documents_by_mime_type"].(map[string]int)
	assert.Equal(t, 2, mimeCount["text/plain"])
	assert.Equal(t, 1, mimeCount["application/pdf"])
}
