package aiwriter

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

// ========== NewManager 测试 ==========

func TestNewManager(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	assert.NotNil(t, mgr)
	assert.NotNil(t, mgr.config)
	assert.Equal(t, LangChinese, mgr.config.DefaultLanguage)
	assert.Equal(t, StyleFormal, mgr.config.DefaultStyle)
	assert.Equal(t, 2000, mgr.config.MaxTokens)
	assert.Equal(t, 100, mgr.config.HistorySize)
}

// ========== Initialize 测试 ==========

func TestInitialize(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	err := mgr.Initialize()
	assert.NoError(t, err)
	assert.NotEmpty(t, mgr.templates)
}

// ========== GenerateText 测试 ==========

func TestGenerateText_Summary(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	req := &WriteRequest{
		TaskType: TaskSummary,
		Content:  "这是一段很长的测试文本，用于验证摘要生成功能是否正常工作。摘要应该截取前200个字符。",
		Language: LangChinese,
		Style:    StyleFormal,
	}

	result, err := mgr.GenerateText(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, TaskSummary, result.TaskType)
	assert.NotEmpty(t, result.Result)
	assert.NotEmpty(t, result.ID)
	assert.Equal(t, LangChinese, result.Language)
}

func TestGenerateText_Expand(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	req := &WriteRequest{
		TaskType: TaskExpand,
		Content:  "人工智能正在改变世界",
		Language: LangChinese,
		Style:    StyleFormal,
	}

	result, err := mgr.GenerateText(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Result, "人工智能正在改变世界")
	assert.Contains(t, result.Result, "综上所述")
}

func TestGenerateText_ExpandCasual(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	req := &WriteRequest{
		TaskType: TaskExpand,
		Content:  "天气不错",
		Style:    StyleCasual,
	}

	result, err := mgr.GenerateText(req)
	assert.NoError(t, err)
	assert.Contains(t, result.Result, "简单来说")
}

func TestGenerateText_ExpandTechnical(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	req := &WriteRequest{
		TaskType: TaskExpand,
		Content:  "系统架构采用微服务模式",
		Style:    StyleTechnical,
	}

	result, err := mgr.GenerateText(req)
	assert.NoError(t, err)
	assert.Contains(t, result.Result, "基于技术分析")
}

func TestGenerateText_Rewrite(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	req := &WriteRequest{
		TaskType: TaskRewrite,
		Content:  "原始文本内容",
		Language: LangChinese,
		Style:    StyleFormal,
	}

	result, err := mgr.GenerateText(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Result, "原始文本内容")
	assert.Contains(t, result.Result, "正式表述")
}

func TestGenerateText_EmptyContent(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	req := &WriteRequest{
		TaskType: TaskSummary,
		Content:  "",
	}

	result, err := mgr.GenerateText(req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, ErrInvalidInput, err)
}

func TestGenerateText_TemplateTypeReturnsError(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	req := &WriteRequest{
		TaskType: TaskTemplate,
		Content:  "some content",
	}

	result, err := mgr.GenerateText(req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, ErrInvalidInput, err)
}

func TestGenerateText_InvalidTaskType(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	req := &WriteRequest{
		TaskType: "invalid_type",
		Content:  "some content",
	}

	result, err := mgr.GenerateText(req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, ErrInvalidInput, err)
}

func TestGenerateText_EnglishSummary(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	req := &WriteRequest{
		TaskType: TaskSummary,
		Content:  "This is a long English text that should be summarized by extracting the first fifty words from the content to demonstrate the English summary generation capability of the AI writing assistant system.",
		Language: LangEnglish,
	}

	result, err := mgr.GenerateText(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Equal(t, LangEnglish, result.Language)
}

// ========== FillTemplate 测试 ==========

func TestFillTemplate_Email(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	req := &TemplateRequest{
		TemplateID: "email-formal",
		Variables: map[string]string{
			"recipient": "张经理",
			"body":      "关于项目进展的汇报",
			"sender":    "李明",
		},
	}

	result, err := mgr.FillTemplate(req)
	assert.NoError(t, err)
	assert.NotNil(t, result)
	assert.Contains(t, result.Result, "张经理")
	assert.Contains(t, result.Result, "关于项目进展的汇报")
	assert.Contains(t, result.Result, "李明")
	assert.Equal(t, TaskTemplate, result.TaskType)
}

func TestFillTemplate_Report(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	req := &TemplateRequest{
		TemplateID: "report-formal",
		Variables: map[string]string{
			"title":     "2024年第1周工作周报",
			"completed": "完成用户模块开发",
			"planned":   "开始API网关开发",
			"issues":    "无",
		},
	}

	result, err := mgr.FillTemplate(req)
	assert.NoError(t, err)
	assert.Contains(t, result.Result, "2024年第1周工作周报")
	assert.Contains(t, result.Result, "完成用户模块开发")
}

func TestFillTemplate_NotFound(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	req := &TemplateRequest{
		TemplateID: "non-existent",
		Variables:  map[string]string{},
	}

	result, err := mgr.FillTemplate(req)
	assert.Error(t, err)
	assert.Nil(t, result)
	assert.Equal(t, ErrNotFound, err)
}

// ========== ListTemplates 测试 ==========

func TestListTemplates(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	templates := mgr.ListTemplates()
	assert.NotEmpty(t, templates)
	assert.GreaterOrEqual(t, len(templates), 5)

	// 验证包含预期模板
	ids := make(map[string]bool)
	for _, tmpl := range templates {
		ids[tmpl.ID] = true
	}
	assert.True(t, ids["email-formal"])
	assert.True(t, ids["report-formal"])
	assert.True(t, ids["announce-formal"])
	assert.True(t, ids["email-en-formal"])
	assert.True(t, ids["report-ja-formal"])
}

// ========== GetTemplate 测试 ==========

func TestGetTemplate_Exists(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	tmpl, err := mgr.GetTemplate("email-formal")
	assert.NoError(t, err)
	assert.NotNil(t, tmpl)
	assert.Equal(t, "正式邮件", tmpl.Name)
	assert.Equal(t, TmplEmail, tmpl.Type)
	assert.Equal(t, LangChinese, tmpl.Language)
	assert.Contains(t, tmpl.Variables, "recipient")
	assert.Contains(t, tmpl.Variables, "body")
	assert.Contains(t, tmpl.Variables, "sender")
}

func TestGetTemplate_NotExists(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	tmpl, err := mgr.GetTemplate("non-existent-id")
	assert.Error(t, err)
	assert.Nil(t, tmpl)
	assert.Equal(t, ErrNotFound, err)
}

// ========== GetHistory 测试 ==========

func TestGetHistory_Empty(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	history := mgr.GetHistory()
	assert.Empty(t, history)
}

func TestGetHistory_AfterGenerate(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	req := &WriteRequest{
		TaskType: TaskSummary,
		Content:  "测试文本",
	}

	_, err := mgr.GenerateText(req)
	assert.NoError(t, err)

	history := mgr.GetHistory()
	assert.Len(t, history, 1)
	assert.Equal(t, TaskSummary, history[0].TaskType)
}

func TestGetHistory_AfterMultipleOperations(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	// 生成文本
	mgr.GenerateText(&WriteRequest{TaskType: TaskSummary, Content: "文本1"})
	mgr.GenerateText(&WriteRequest{TaskType: TaskExpand, Content: "文本2"})

	// 填充模板
	mgr.FillTemplate(&TemplateRequest{
		TemplateID: "email-formal",
		Variables:  map[string]string{"recipient": "A", "body": "B", "sender": "C"},
	})

	history := mgr.GetHistory()
	assert.Len(t, history, 3)
}

// ========== GetStats 测试 ==========

func TestGetStats_Empty(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	stats := mgr.GetStats()
	assert.Equal(t, 0, stats["total_writes"])
	assert.Equal(t, len(mgr.templates), stats["templates"])
}

func TestGetStats_AfterOperations(t *testing.T) {
	mgr := NewManager("/tmp/test-writer.json")
	mgr.Initialize()

	mgr.GenerateText(&WriteRequest{TaskType: TaskSummary, Content: "文本1", Language: LangChinese})
	mgr.GenerateText(&WriteRequest{TaskType: TaskExpand, Content: "文本2", Language: LangEnglish})

	stats := mgr.GetStats()
	assert.Equal(t, 2, stats["total_writes"])

	taskCounts := stats["task_counts"].(map[TaskType]int)
	assert.Equal(t, 1, taskCounts[TaskSummary])
	assert.Equal(t, 1, taskCounts[TaskExpand])

	langCounts := stats["lang_counts"].(map[Language]int)
	assert.Equal(t, 1, langCounts[LangChinese])
	assert.Equal(t, 1, langCounts[LangEnglish])
}

// ========== 常量测试 ==========

func TestLanguageConstants(t *testing.T) {
	assert.Equal(t, Language("zh"), LangChinese)
	assert.Equal(t, Language("en"), LangEnglish)
	assert.Equal(t, Language("ja"), LangJapanese)
}

func TestWriteStyleConstants(t *testing.T) {
	assert.Equal(t, WriteStyle("formal"), StyleFormal)
	assert.Equal(t, WriteStyle("casual"), StyleCasual)
	assert.Equal(t, WriteStyle("technical"), StyleTechnical)
}

func TestTaskTypeConstants(t *testing.T) {
	assert.Equal(t, TaskType("summary"), TaskSummary)
	assert.Equal(t, TaskType("expand"), TaskExpand)
	assert.Equal(t, TaskType("rewrite"), TaskRewrite)
	assert.Equal(t, TaskType("template"), TaskTemplate)
}

func TestTemplateTypeConstants(t *testing.T) {
	assert.Equal(t, TemplateType("email"), TmplEmail)
	assert.Equal(t, TemplateType("report"), TmplReport)
	assert.Equal(t, TemplateType("announce"), TmplAnnounce)
}
