package filetagger

import (
	"encoding/hex"
	"encoding/json"
	"testing"
)

// ========== 标签管理测试 ==========

func TestCreateTag(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	tag, err := engine.CreateTag("照片", CategoryImage, "", "#FF5733", "📷")
	if err != nil {
		t.Fatalf("CreateTag failed: %v", err)
	}

	if tag.Name != "照片" {
		t.Errorf("expected name '照片', got %q", tag.Name)
	}
	if tag.Category != CategoryImage {
		t.Errorf("expected category %q, got %q", CategoryImage, tag.Category)
	}
	if tag.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestCreateDuplicateTag(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	_, err := engine.CreateTag("测试", CategoryDocument, "", "", "")
	if err != nil {
		t.Fatalf("first CreateTag failed: %v", err)
	}

	_, err = engine.CreateTag("测试", CategoryDocument, "", "", "")
	if err != ErrTagExists {
		t.Errorf("expected ErrTagExists, got %v", err)
	}
}

func TestCreateTagWithParent(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	parent, _ := engine.CreateTag("媒体", CategoryOther, "", "", "")
	child, err := engine.CreateTag("照片", CategoryImage, parent.ID, "", "")
	if err != nil {
		t.Fatalf("CreateTag with parent failed: %v", err)
	}

	if child.ParentID != parent.ID {
		t.Errorf("expected parentId %q, got %q", parent.ID, child.ParentID)
	}

	tree := engine.GetTagTree()
	if len(tree) != 1 {
		t.Fatalf("expected 1 root, got %d", len(tree))
	}
	if len(tree[0].Children) != 1 {
		t.Errorf("expected 1 child, got %d", len(tree[0].Children))
	}
}

func TestCreateTagWithInvalidParent(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	_, err := engine.CreateTag("测试", CategoryDocument, "nonexistent", "", "")
	if err != ErrTagNotFound {
		t.Errorf("expected ErrTagNotFound, got %v", err)
	}
}

func TestGetTag(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	created, _ := engine.CreateTag("文档", CategoryDocument, "", "", "")
	got, err := engine.GetTag(created.ID)
	if err != nil {
		t.Fatalf("GetTag failed: %v", err)
	}
	if got.ID != created.ID {
		t.Errorf("ID mismatch: expected %q, got %q", created.ID, got.ID)
	}
}

func TestGetTagNotFound(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	_, err := engine.GetTag("nonexistent")
	if err != ErrTagNotFound {
		t.Errorf("expected ErrTagNotFound, got %v", err)
	}
}

func TestUpdateTag(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	tag, _ := engine.CreateTag("旧名", CategoryDocument, "", "", "")
	updated, err := engine.UpdateTag(tag.ID, "新名", "", "#00FF00", "📄")
	if err != nil {
		t.Fatalf("UpdateTag failed: %v", err)
	}

	if updated.Name != "新名" {
		t.Errorf("expected name '新名', got %q", updated.Name)
	}
	if updated.Color != "#00FF00" {
		t.Errorf("expected color '#00FF00', got %q", updated.Color)
	}
}

func TestDeleteTag(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	tag, _ := engine.CreateTag("临时", CategoryOther, "", "", "")
	if err := engine.DeleteTag(tag.ID); err != nil {
		t.Fatalf("DeleteTag failed: %v", err)
	}

	_, err := engine.GetTag(tag.ID)
	if err != ErrTagNotFound {
		t.Errorf("expected ErrTagNotFound after delete, got %v", err)
	}
}

func TestDeleteTagWithChildren(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	parent, _ := engine.CreateTag("父标签", CategoryOther, "", "", "")
	child, _ := engine.CreateTag("子标签", CategoryOther, parent.ID, "", "")

	if err := engine.DeleteTag(parent.ID); err != nil {
		t.Fatalf("DeleteTag failed: %v", err)
	}

	_, err := engine.GetTag(child.ID)
	if err != ErrTagNotFound {
		t.Error("child tag should be deleted with parent")
	}
}

func TestCircularParentReference(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	a, _ := engine.CreateTag("A", CategoryOther, "", "", "")
	b, _ := engine.CreateTag("B", CategoryOther, a.ID, "", "")

	// 尝试让 A 的父标签设为 B -> 循环引用
	_, err := engine.UpdateTag(a.ID, "", b.ID, "", "")
	if err != ErrCircularParent {
		t.Errorf("expected ErrCircularParent, got %v", err)
	}
}

func TestListTags(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	engine.CreateTag("B", CategoryOther, "", "", "")
	engine.CreateTag("A", CategoryDocument, "", "", "")
	engine.CreateTag("C", CategoryImage, "", "", "")

	tags := engine.ListTags()
	if len(tags) != 3 {
		t.Fatalf("expected 3 tags, got %d", len(tags))
	}

	// 应该按名称排序
	if tags[0].Name != "A" || tags[1].Name != "B" || tags[2].Name != "C" {
		t.Error("tags not sorted by name")
	}
}

func TestGetTagAncestors(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	root, _ := engine.CreateTag("根", CategoryOther, "", "", "")
	mid, _ := engine.CreateTag("中间", CategoryOther, root.ID, "", "")
	leaf, _ := engine.CreateTag("叶子", CategoryOther, mid.ID, "", "")

	ancestors := engine.GetTagAncestors(leaf.ID)
	if len(ancestors) != 2 {
		t.Fatalf("expected 2 ancestors, got %d", len(ancestors))
	}
	if ancestors[0] != mid.ID || ancestors[1] != root.ID {
		t.Errorf("unexpected ancestors: %v", ancestors)
	}
}

// ========== 文件标签测试 ==========

func TestAddFileTag(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("重要", CategoryOther, "", "", "")

	err := engine.AddFileTag("/docs/readme.md", tag.ID, false, "")
	if err != nil {
		t.Fatalf("AddFileTag failed: %v", err)
	}

	ft := engine.GetFileTags("/docs/readme.md")
	if len(ft.Tags) != 1 {
		t.Fatalf("expected 1 tag, got %d", len(ft.Tags))
	}
	if ft.Tags[0].IsAuto {
		t.Error("expected manual tag, got auto")
	}
}

func TestAddDuplicateFileTag(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("标签", CategoryOther, "", "", "")

	engine.AddFileTag("/file.txt", tag.ID, false, "")
	engine.AddFileTag("/file.txt", tag.ID, false, "") // 重复

	ft := engine.GetFileTags("/file.txt")
	if len(ft.Tags) != 1 {
		t.Errorf("expected 1 tag after duplicate add, got %d", len(ft.Tags))
	}
}

func TestRemoveFileTag(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("临时", CategoryOther, "", "", "")

	engine.AddFileTag("/file.txt", tag.ID, false, "")
	err := engine.RemoveFileTag("/file.txt", tag.ID)
	if err != nil {
		t.Fatalf("RemoveFileTag failed: %v", err)
	}

	ft := engine.GetFileTags("/file.txt")
	if len(ft.Tags) != 0 {
		t.Errorf("expected 0 tags after removal, got %d", len(ft.Tags))
	}
}

func TestGetFileTagsAutoManual(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	autoTag, _ := engine.CreateTag("自动", CategoryDocument, "", "", "")
	manualTag, _ := engine.CreateTag("手动", CategoryOther, "", "", "")

	engine.AddFileTag("/doc.pdf", autoTag.ID, true, "rule1")
	engine.AddFileTag("/doc.pdf", manualTag.ID, false, "")

	ft := engine.GetFileTags("/doc.pdf")
	if len(ft.AutoTags) != 1 {
		t.Errorf("expected 1 auto tag, got %d", len(ft.AutoTags))
	}
	if len(ft.ManualTags) != 1 {
		t.Errorf("expected 1 manual tag, got %d", len(ft.ManualTags))
	}
}

func TestDeleteTagRemovesFileAssociations(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("要删", CategoryOther, "", "", "")

	engine.AddFileTag("/file1.txt", tag.ID, false, "")
	engine.AddFileTag("/file2.txt", tag.ID, false, "")

	engine.DeleteTag(tag.ID)

	ft1 := engine.GetFileTags("/file1.txt")
	ft2 := engine.GetFileTags("/file2.txt")
	if len(ft1.Tags) != 0 || len(ft2.Tags) != 0 {
		t.Error("file associations should be cleared after tag deletion")
	}
}

// ========== 规则引擎测试 ==========

func TestCreateRule(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	rule := AutoRule{
		Name:     "图片规则",
		Enabled:  true,
		Priority: 10,
		Type:     RuleTypeExtension,
		TagIDs:   []string{"tag1"},
		Conditions: Condition{
			Extensions: []string{".jpg", ".png", ".gif"},
		},
	}

	created, err := engine.CreateRule(rule)
	if err != nil {
		t.Fatalf("CreateRule failed: %v", err)
	}
	if created.ID == "" {
		t.Error("expected non-empty rule ID")
	}
}

func TestCreateRuleInvalidRegex(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	rule := AutoRule{
		Name:    "坏规则",
		Enabled: true,
		Conditions: Condition{
			PathRegex: "[invalid",
		},
	}

	_, err := engine.CreateRule(rule)
	if err == nil {
		t.Error("expected error for invalid regex")
	}
}

func TestMatchExtension(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("图片", CategoryImage, "", "", "")

	engine.CreateRule(AutoRule{
		Name:     "图片扩展名",
		Enabled:  true,
		Priority: 10,
		Type:     RuleTypeExtension,
		TagIDs:   []string{tag.ID},
		Conditions: Condition{
			Extensions: []string{".jpg", ".png", ".gif"},
		},
	})

	// 匹配
	matched := engine.MatchFile("/photos/vacation.jpg", 1024, "image/jpeg", nil)
	if len(matched) != 1 || matched[0] != tag.ID {
		t.Errorf("expected match for .jpg, got %v", matched)
	}

	// 不匹配
	matched = engine.MatchFile("/docs/report.pdf", 1024, "application/pdf", nil)
	if len(matched) != 0 {
		t.Errorf("expected no match for .pdf, got %v", matched)
	}
}

func TestMatchMIME(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("视频", CategoryVideo, "", "", "")

	engine.CreateRule(AutoRule{
		Name:     "视频MIME",
		Enabled:  true,
		Priority: 10,
		Type:     RuleTypeMIME,
		TagIDs:   []string{tag.ID},
		Conditions: Condition{
			MIMETypes: []string{"video/*"},
		},
	})

	matched := engine.MatchFile("/movies/film.mp4", 1024, "video/mp4", nil)
	if len(matched) != 1 {
		t.Errorf("expected 1 match for video MIME, got %d", len(matched))
	}
}

func TestMatchPathGlob(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("备份", CategoryArchive, "", "", "")

	engine.CreateRule(AutoRule{
		Name:     "备份目录",
		Enabled:  true,
		Priority: 10,
		Type:     RuleTypeGlob,
		TagIDs:   []string{tag.ID},
		Conditions: Condition{
			PathPatterns: []string{"/backup/**"},
		},
	})

	matched := engine.MatchFile("/backup/2024/data.tar.gz", 1024, "", nil)
	if len(matched) != 1 {
		t.Errorf("expected 1 match for backup path, got %d", len(matched))
	}

	matched = engine.MatchFile("/home/user/file.txt", 1024, "", nil)
	if len(matched) != 0 {
		t.Errorf("expected 0 match for non-backup path, got %d", len(matched))
	}
}

func TestMatchSizeOperator(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("大文件", CategoryOther, "", "", "")

	engine.CreateRule(AutoRule{
		Name:     "大文件规则",
		Enabled:  true,
		Priority: 10,
		Type:     RuleTypeSize,
		TagIDs:   []string{tag.ID},
		Conditions: Condition{
			SizeOp:    OpGreater,
			SizeValue: 1024 * 1024, // > 1MB
		},
	})

	matched := engine.MatchFile("/big.iso", 10*1024*1024, "", nil)
	if len(matched) != 1 {
		t.Errorf("expected 1 match for large file, got %d", len(matched))
	}

	matched = engine.MatchFile("/small.txt", 512, "", nil)
	if len(matched) != 0 {
		t.Errorf("expected 0 match for small file, got %d", len(matched))
	}
}

func TestMatchSizeBetween(t *testing.T) {
	ok := matchSize(500, OpBetween, 100, 1000)
	if !ok {
		t.Error("expected match for size 500 between 100-1000")
	}
	ok = matchSize(50, OpBetween, 100, 1000)
	if ok {
		t.Error("expected no match for size 50 between 100-1000")
	}
}

func TestMatchContentMagic(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("PNG文件", CategoryImage, "", "", "")

	engine.CreateRule(AutoRule{
		Name:     "PNG魔术字节",
		Enabled:  true,
		Priority: 10,
		Type:     RuleTypeContent,
		TagIDs:   []string{tag.ID},
		Conditions: Condition{
			ContentMagic: []string{"89504e47"}, // PNG 文件头
		},
	})

	// PNG header
	pngHeader, _ := hex.DecodeString("89504e470d0a1a0a")
	matched := engine.MatchFile("/image.dat", 1024, "", pngHeader)
	if len(matched) != 1 {
		t.Errorf("expected 1 match for PNG content, got %d", len(matched))
	}

	// 非PNG
	matched = engine.MatchFile("/file.dat", 1024, "", []byte{0x00, 0x01, 0x02})
	if len(matched) != 0 {
		t.Errorf("expected 0 match for non-PNG content, got %d", len(matched))
	}
}

func TestMatchAndCondition(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("大图片", CategoryImage, "", "", "")

	engine.CreateRule(AutoRule{
		Name:     "大图片规则",
		Enabled:  true,
		Priority: 10,
		TagIDs:   []string{tag.ID},
		Conditions: Condition{
			And: []Condition{
				{Extensions: []string{".jpg", ".png"}},
				{SizeOp: OpGreater, SizeValue: 1024 * 100}, // > 100KB
			},
		},
	})

	// 大图片 - 匹配
	matched := engine.MatchFile("/photo.jpg", 1024*500, "image/jpeg", nil)
	if len(matched) != 1 {
		t.Errorf("expected 1 match, got %d", len(matched))
	}

	// 小图片 - 不匹配
	matched = engine.MatchFile("/thumb.jpg", 1024, "image/jpeg", nil)
	if len(matched) != 0 {
		t.Errorf("expected 0 match, got %d", len(matched))
	}
}

func TestMatchOrCondition(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("媒体", CategoryOther, "", "", "")

	engine.CreateRule(AutoRule{
		Name:     "媒体文件",
		Enabled:  true,
		Priority: 10,
		TagIDs:   []string{tag.ID},
		Conditions: Condition{
			Or: []Condition{
				{MIMETypes: []string{"image/*"}},
				{MIMETypes: []string{"video/*"}},
			},
		},
	})

	matched := engine.MatchFile("/pic.jpg", 1024, "image/jpeg", nil)
	if len(matched) != 1 {
		t.Errorf("expected match for image, got %d", len(matched))
	}

	matched = engine.MatchFile("/vid.mp4", 1024, "video/mp4", nil)
	if len(matched) != 1 {
		t.Errorf("expected match for video, got %d", len(matched))
	}

	matched = engine.MatchFile("/doc.pdf", 1024, "application/pdf", nil)
	if len(matched) != 0 {
		t.Errorf("expected 0 match for document, got %d", len(matched))
	}
}

func TestMatchNotCondition(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("非图片", CategoryOther, "", "", "")

	engine.CreateRule(AutoRule{
		Name:     "非图片文件",
		Enabled:  true,
		Priority: 10,
		TagIDs:   []string{tag.ID},
		Conditions: Condition{
			Not: &Condition{
				Extensions: []string{".jpg", ".png", ".gif"},
			},
		},
	})

	matched := engine.MatchFile("/readme.txt", 1024, "text/plain", nil)
	if len(matched) != 1 {
		t.Errorf("expected 1 match for non-image, got %d", len(matched))
	}

	matched = engine.MatchFile("/photo.jpg", 1024, "image/jpeg", nil)
	if len(matched) != 0 {
		t.Errorf("expected 0 match for image, got %d", len(matched))
	}
}

func TestDisabledRule(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("标签", CategoryOther, "", "", "")

	engine.CreateRule(AutoRule{
		Name:     "已禁用",
		Enabled:  false,
		Priority: 10,
		TagIDs:   []string{tag.ID},
		Conditions: Condition{
			Extensions: []string{".txt"},
		},
	})

	matched := engine.MatchFile("/test.txt", 1024, "text/plain", nil)
	if len(matched) != 0 {
		t.Errorf("disabled rule should not match, got %d matches", len(matched))
	}
}

func TestRulePriority(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tagA, _ := engine.CreateTag("A", CategoryOther, "", "", "")
	tagB, _ := engine.CreateTag("B", CategoryOther, "", "", "")

	// 低优先级
	engine.CreateRule(AutoRule{
		Name:     "低优先级",
		Enabled:  true,
		Priority: 1,
		TagIDs:   []string{tagA.ID},
		Conditions: Condition{
			Extensions: []string{".txt"},
		},
	})

	// 高优先级
	engine.CreateRule(AutoRule{
		Name:     "高优先级",
		Enabled:  true,
		Priority: 10,
		TagIDs:   []string{tagB.ID},
		Conditions: Condition{
			Extensions: []string{".txt"},
		},
	})

	matched := engine.MatchFile("/test.txt", 1024, "text/plain", nil)
	if len(matched) != 2 {
		t.Fatalf("expected 2 matches, got %d", len(matched))
	}
	// 高优先级应该在前面
	if matched[0] != tagB.ID {
		t.Errorf("expected high priority tag first, got %v", matched)
	}
}

func TestDeleteRule(t *testing.T) {
	engine := NewEngine(DefaultConfig())

	rule, _ := engine.CreateRule(AutoRule{
		Name:       "要删",
		Enabled:    true,
		TagIDs:     []string{"tag1"},
		Conditions: Condition{Extensions: []string{".txt"}},
	})

	if err := engine.DeleteRule(rule.ID); err != nil {
		t.Fatalf("DeleteRule failed: %v", err)
	}

	_, err := engine.GetRule(rule.ID)
	if err != ErrRuleNotFound {
		t.Errorf("expected ErrRuleNotFound after delete, got %v", err)
	}
}

// ========== 智能分类测试 ==========

func TestClassifyFile(t *testing.T) {
	tests := []struct {
		ext      string
		mime     string
		expected FileCategory
	}{
		{".jpg", "image/jpeg", CategoryImage},
		{".png", "image/png", CategoryImage},
		{".mp4", "video/mp4", CategoryVideo},
		{".mp3", "audio/mpeg", CategoryAudio},
		{".flac", "audio/flac", CategoryAudio},
		{".pdf", "application/pdf", CategoryDocument},
		{".docx", "", CategoryDocument},
		{".go", "", CategoryCode},
		{".py", "", CategoryCode},
		{".js", "", CategoryCode},
		{".zip", "application/zip", CategoryArchive},
		{".tar.gz", "", CategoryArchive},
		{".7z", "", CategoryArchive},
		{".ttf", "", CategoryFont},
		{".woff2", "", CategoryFont},
		{".sqlite", "", CategoryData},
		{".xyz", "", CategoryOther},
		{"", "video/webm", CategoryVideo},
		{"", "audio/ogg", CategoryAudio},
		{"", "font/otf", CategoryFont},
	}

	for _, tt := range tests {
		result := ClassifyFile(tt.ext, tt.mime)
		if result != tt.expected {
			t.Errorf("ClassifyFile(%q, %q) = %q, want %q", tt.ext, tt.mime, result, tt.expected)
		}
	}
}

// ========== 搜索测试 ==========

func TestSearchByTags(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tagA, _ := engine.CreateTag("A", CategoryOther, "", "", "")
	tagB, _ := engine.CreateTag("B", CategoryOther, "", "", "")

	engine.AddFileTag("/file1.txt", tagA.ID, false, "")
	engine.AddFileTag("/file1.txt", tagB.ID, false, "")
	engine.AddFileTag("/file2.txt", tagA.ID, false, "")
	engine.AddFileTag("/file3.txt", tagB.ID, false, "")

	// AND 搜索：同时有 A 和 B
	result := engine.Search(SearchQuery{
		Tags:     []string{tagA.ID, tagB.ID},
		PageSize: 10,
	})
	if result.Total != 1 {
		t.Errorf("expected 1 result for AND search, got %d", result.Total)
	}

	// OR 搜索：有 A 或 B
	result = engine.Search(SearchQuery{
		AnyTags:  []string{tagA.ID, tagB.ID},
		PageSize: 10,
	})
	if result.Total != 3 {
		t.Errorf("expected 3 results for OR search, got %d", result.Total)
	}

	// 排除搜索
	result = engine.Search(SearchQuery{
		AnyTags:     []string{tagA.ID, tagB.ID},
		ExcludeTags: []string{tagA.ID},
		PageSize:    10,
	})
	if result.Total != 1 {
		t.Errorf("expected 1 result for exclude search, got %d", result.Total)
	}
}

func TestSearchPagination(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("通用", CategoryOther, "", "", "")

	for i := 0; i < 25; i++ {
		engine.AddFileTag("/file"+itoa(i)+".txt", tag.ID, false, "")
	}

	page1 := engine.Search(SearchQuery{
		Tags:     []string{tag.ID},
		Page:     1,
		PageSize: 10,
	})
	if page1.Total != 25 {
		t.Errorf("expected total 25, got %d", page1.Total)
	}
	if len(page1.Files) != 10 {
		t.Errorf("expected 10 files on page 1, got %d", len(page1.Files))
	}

	page3 := engine.Search(SearchQuery{
		Tags:     []string{tag.ID},
		Page:     3,
		PageSize: 10,
	})
	if len(page3.Files) != 5 {
		t.Errorf("expected 5 files on page 3, got %d", len(page3.Files))
	}
}

func itoa(i int) string {
	return json.Number(intToStr(i)).String()
}

func intToStr(i int) string {
	if i == 0 {
		return "0"
	}
	s := ""
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	for i > 0 {
		s = string(rune('0'+i%10)) + s
		i /= 10
	}
	if neg {
		s = "-" + s
	}
	return s
}

// ========== 统计测试 ==========

func TestGetStats(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag1, _ := engine.CreateTag("图片", CategoryImage, "", "", "")
	tag2, _ := engine.CreateTag("文档", CategoryDocument, "", "", "")

	engine.AddFileTag("/a.jpg", tag1.ID, false, "")
	engine.AddFileTag("/b.jpg", tag1.ID, false, "")
	engine.AddFileTag("/c.pdf", tag2.ID, false, "")

	engine.CreateRule(AutoRule{
		Name: "r1", Enabled: true, TagIDs: []string{tag1.ID},
		Conditions: Condition{Extensions: []string{".jpg"}},
	})

	stats := engine.GetStats()
	if stats.TotalFiles != 3 {
		t.Errorf("expected 3 total files, got %d", stats.TotalFiles)
	}
	if stats.TotalTags != 2 {
		t.Errorf("expected 2 total tags, got %d", stats.TotalTags)
	}
	if stats.TotalRules != 1 {
		t.Errorf("expected 1 total rules, got %d", stats.TotalRules)
	}
}

func TestGetTagStats(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("图片", CategoryImage, "", "", "")

	engine.AddFileTag("/a.jpg", tag.ID, false, "")
	engine.AddFileTag("/b.jpg", tag.ID, false, "")

	stat, err := engine.GetTagStats(tag.ID)
	if err != nil {
		t.Fatalf("GetTagStats failed: %v", err)
	}
	if stat.FileCount != 2 {
		t.Errorf("expected 2 files, got %d", stat.FileCount)
	}
}

// ========== 批量操作测试 ==========

func TestBatchApplyTags(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("批量", CategoryOther, "", "", "")

	files := []string{"/a.txt", "/b.txt", "/c.txt"}
	count, err := engine.BatchApplyTags(files, tag.ID, false, "")
	if err != nil {
		t.Fatalf("BatchApplyTags failed: %v", err)
	}
	if count != 3 {
		t.Errorf("expected 3 applied, got %d", count)
	}

	// 重复应用
	count, _ = engine.BatchApplyTags(files, tag.ID, false, "")
	if count != 0 {
		t.Errorf("expected 0 on duplicate apply, got %d", count)
	}
}

// ========== 导入导出测试 ==========

func TestExportImport(t *testing.T) {
	engine1 := NewEngine(DefaultConfig())
	tag, _ := engine1.CreateTag("导出", CategoryDocument, "", "", "")
	engine1.AddFileTag("/doc.pdf", tag.ID, false, "")
	engine1.CreateRule(AutoRule{
		Name: "r1", Enabled: true, TagIDs: []string{tag.ID},
		Conditions: Condition{Extensions: []string{".pdf"}},
	})

	data, err := engine1.ExportJSON()
	if err != nil {
		t.Fatalf("ExportJSON failed: %v", err)
	}

	// 导入到新引擎
	engine2 := NewEngine(DefaultConfig())
	tags, rules, fileTags, err := engine2.ImportJSON(data, false)
	if err != nil {
		t.Fatalf("ImportJSON failed: %v", err)
	}

	if tags != 1 {
		t.Errorf("expected 1 tag imported, got %d", tags)
	}
	if rules != 1 {
		t.Errorf("expected 1 rule imported, got %d", rules)
	}
	if fileTags != 1 {
		t.Errorf("expected 1 fileTag imported, got %d", fileTags)
	}

	// 验证导入的数据
	importedTags := engine2.ListTags()
	if len(importedTags) != 1 || importedTags[0].Name != "导出" {
		t.Error("imported tag data mismatch")
	}
}

func TestExportImportOverwrite(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	tag, _ := engine.CreateTag("原始", CategoryOther, "", "", "")

	exportData := ExportData{
		Version: "1.0",
		Tags: []Tag{
			{ID: tag.ID, Name: "覆盖", Category: CategoryDocument},
		},
	}

	jsonData, _ := json.Marshal(exportData)

	// 不覆盖
	engine.ImportJSON(jsonData, false)
	got, _ := engine.GetTag(tag.ID)
	if got.Name != "原始" {
		t.Errorf("expected name '原始' without overwrite, got %q", got.Name)
	}

	// 覆盖
	engine.ImportJSON(jsonData, true)
	got, _ = engine.GetTag(tag.ID)
	if got.Name != "覆盖" {
		t.Errorf("expected name '覆盖' with overwrite, got %q", got.Name)
	}
}

func TestImportInvalidJSON(t *testing.T) {
	engine := NewEngine(DefaultConfig())
	_, _, _, err := engine.ImportJSON([]byte("not json"), false)
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

// ========== Glob工具函数测试 ==========

func TestGlobToRegex(t *testing.T) {
	tests := []struct {
		pattern string
		path    string
		match   bool
	}{
		{"/backup/**", "/backup/2024/data.tar.gz", true},
		{"/backup/**", "/home/user/file.txt", false},
		{"*.txt", "readme.txt", true}, // * 匹配任意不含/的字符串
		{"/docs/*.txt", "/docs/readme.txt", true},
		{"/docs/*.txt", "/docs/sub/readme.txt", false},
		{"/photos/**/*.jpg", "/photos/2024/summer/beach.jpg", true},
	}

	for _, tt := range tests {
		re, err := globToRegex(tt.pattern)
		if err != nil {
			t.Errorf("globToRegex(%q) error: %v", tt.pattern, err)
			continue
		}
		result := re.MatchString(tt.path)
		if result != tt.match {
			t.Errorf("glob %q match(%q) = %v, want %v", tt.pattern, tt.path, result, tt.match)
		}
	}
}

// ========== DetectMIME 测试 ==========

func TestDetectMIME(t *testing.T) {
	tests := []struct {
		path     string
		expected string
		altMIME  string // 不同环境mime包可能返回不同值
	}{
		{"photo.jpg", "image/jpeg", ""},
		{"doc.pdf", "application/pdf", ""},
		{"song.mp3", "audio/mpeg", ""},
		{"unknown.xyz", "chemical/x-xyz", "application/octet-stream"},
	}

	for _, tt := range tests {
		result := DetectMIME(tt.path)
		if result != tt.expected && (tt.altMIME == "" || result != tt.altMIME) {
			t.Errorf("DetectMIME(%q) = %q, want %q", tt.path, result, tt.expected)
		}
	}
}

// ========== MatchSize 测试 ==========

func TestMatchSizeOps(t *testing.T) {
	tests := []struct {
		size int64
		op   Operator
		val  int64
		val2 int64
		want bool
	}{
		{100, OpEquals, 100, 0, true},
		{100, OpEquals, 200, 0, false},
		{100, OpNotEqual, 200, 0, true},
		{100, OpNotEqual, 100, 0, false},
		{200, OpGreater, 100, 0, true},
		{50, OpGreater, 100, 0, false},
		{50, OpLess, 100, 0, true},
		{200, OpLess, 100, 0, false},
		{500, OpBetween, 100, 1000, true},
		{50, OpBetween, 100, 1000, false},
		{1500, OpBetween, 100, 1000, false},
	}

	for _, tt := range tests {
		got := matchSize(tt.size, tt.op, tt.val, tt.val2)
		if got != tt.want {
			t.Errorf("matchSize(%d, %s, %d, %d) = %v, want %v",
				tt.size, tt.op, tt.val, tt.val2, got, tt.want)
		}
	}
}

// ========== MatchContentMagic 测试 ==========

func TestMatchContentMagicFunc(t *testing.T) {
	// JPEG magic: FF D8 FF
	jpegHeader := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	ok := matchContentMagic(jpegHeader, "ffd8ff")
	if !ok {
		t.Error("expected match for JPEG magic")
	}

	// 太短的内容
	short := []byte{0xFF, 0xD8}
	ok = matchContentMagic(short, "ffd8ff")
	if ok {
		t.Error("expected no match for short content")
	}

	// 不匹配
	ok = matchContentMagic([]byte{0x00, 0x01, 0x02, 0x03}, "ffd8ff")
	if ok {
		t.Error("expected no match for non-JPEG content")
	}

	// 无效的hex
	ok = matchContentMagic(jpegHeader, "zzzz")
	if ok {
		t.Error("expected no match for invalid hex")
	}
}
