package contentai

import (
	"context"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	ca := New(nil)
	if ca == nil {
		t.Fatal("New returned nil")
	}
	if ca.tasks == nil {
		t.Error("tasks map not initialized")
	}
	if ca.templates == nil {
		t.Error("templates map not initialized")
	}
	if ca.config == nil {
		t.Error("config not initialized")
	}

	// 测试使用自定义配置
	config := &Config{
		ModelPath:   "/custom/path",
		Language:    LangEnglish,
		MaxTokens:   2048,
		Temperature: 0.8,
	}
	ca2 := New(config)
	if ca2.config.ModelPath != "/custom/path" {
		t.Errorf("expected /custom/path, got %s", ca2.config.ModelPath)
	}
	if ca2.config.Language != LangEnglish {
		t.Errorf("expected en, got %s", ca2.config.Language)
	}
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()
	if config == nil {
		t.Fatal("DefaultConfig returned nil")
	}
	if config.MaxTokens != 4096 {
		t.Errorf("expected 4096, got %d", config.MaxTokens)
	}
	if config.Language != LangChinese {
		t.Errorf("expected zh, got %s", config.Language)
	}
	if config.Temperature != 0.7 {
		t.Errorf("expected 0.7, got %f", config.Temperature)
	}
}

func TestStartStop(t *testing.T) {
	ca := New(nil)

	// 测试启动
	err := ca.Start()
	if err != nil {
		t.Fatalf("Start failed: %v", err)
	}
	if !ca.IsRunning() {
		t.Error("expected running state")
	}

	// 测试重复启动
	err = ca.Start()
	if err == nil {
		t.Error("expected error for duplicate start")
	}

	// 测试停止
	err = ca.Stop()
	if err != nil {
		t.Fatalf("Stop failed: %v", err)
	}
	if ca.IsRunning() {
		t.Error("expected stopped state")
	}

	// 测试重复停止
	err = ca.Stop()
	if err == nil {
		t.Error("expected error for duplicate stop")
	}
}

func TestDefaultTemplates(t *testing.T) {
	ca := New(nil)

	templates := ca.GetTemplates()
	if len(templates) < 4 {
		t.Errorf("expected at least 4 default templates, got %d", len(templates))
	}

	// 检查中文文章模板
	tpl, err := ca.GetTemplate("tpl_article_zh")
	if err != nil {
		t.Fatalf("GetTemplate failed: %v", err)
	}
	if tpl.Name != "中文文章模板" {
		t.Errorf("expected 中文文章模板, got %s", tpl.Name)
	}
	if tpl.Type != ContentTypeArticle {
		t.Errorf("expected article, got %s", tpl.Type)
	}
	if tpl.Language != LangChinese {
		t.Errorf("expected zh, got %s", tpl.Language)
	}
}

func TestAddTemplate(t *testing.T) {
	ca := New(nil)

	// 添加新模板
	tpl := &ContentTemplate{
		Name:        "自定义模板",
		Description: "测试模板",
		Type:        ContentTypeBlog,
		Language:    LangChinese,
		Style:       StyleCasual,
		Template:    "自定义内容：{{content}}",
		Variables:   []string{"content"},
		Tags:        []string{"custom", "test"},
	}

	err := ca.AddTemplate(tpl)
	if err != nil {
		t.Fatalf("AddTemplate failed: %v", err)
	}

	if tpl.ID == "" {
		t.Error("expected template ID to be generated")
	}

	// 验证模板已添加
	templates := ca.GetTemplates()
	found := false
	for _, tmpl := range templates {
		if tmpl.ID == tpl.ID {
			found = true
			break
		}
	}
	if !found {
		t.Error("template not found in list")
	}

	// 测试重复添加
	err = ca.AddTemplate(tpl)
	if err == nil {
		t.Error("expected error for duplicate template")
	}
}

func TestUpdateDeleteTemplate(t *testing.T) {
	ca := New(nil)

	// 更新现有模板
	tpl, _ := ca.GetTemplate("tpl_article_zh")
	tpl.Name = "更新后的模板名称"

	err := ca.UpdateTemplate(tpl)
	if err != nil {
		t.Fatalf("UpdateTemplate failed: %v", err)
	}

	updatedTpl, _ := ca.GetTemplate("tpl_article_zh")
	if updatedTpl.Name != "更新后的模板名称" {
		t.Errorf("expected 更新后的模板名称, got %s", updatedTpl.Name)
	}

	// 测试更新不存在的模板
	fakeTpl := &ContentTemplate{ID: "nonexistent"}
	err = ca.UpdateTemplate(fakeTpl)
	if err == nil {
		t.Error("expected error for nonexistent template")
	}

	// 删除模板
	err = ca.DeleteTemplate("tpl_article_zh")
	if err != nil {
		t.Fatalf("DeleteTemplate failed: %v", err)
	}

	// 验证已删除
	_, err = ca.GetTemplate("tpl_article_zh")
	if err == nil {
		t.Error("expected error for deleted template")
	}

	// 测试删除不存在的模板
	err = ca.DeleteTemplate("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}
}

func TestGenerateContent(t *testing.T) {
	ca := New(nil)
	ca.Start()
	defer ca.Stop()

	ctx := context.Background()

	// 生成内容
	task, err := ca.GenerateContent(ctx, "写一篇关于人工智能的文章", "tpl_article_zh")
	if err != nil {
		t.Fatalf("GenerateContent failed: %v", err)
	}

	if task.ID == "" {
		t.Error("expected task ID")
	}
	if task.Status != ContentStatusPending && task.Status != ContentStatusCompleted {
		t.Errorf("unexpected status: %s", task.Status)
	}

	// 等待任务完成
	time.Sleep(200 * time.Millisecond)

	// 获取任务结果
	completedTask, err := ca.GetTask(task.ID)
	if err != nil {
		t.Fatalf("GetTask failed: %v", err)
	}

	if completedTask.Status != ContentStatusCompleted {
		t.Errorf("expected completed, got %s", completedTask.Status)
	}
	if completedTask.Result == nil {
		t.Error("expected result")
	}
	if completedTask.Result.Content == "" {
		t.Error("expected content")
	}

	// 测试不存在的模板
	_, err = ca.GenerateContent(ctx, "test", "nonexistent_tpl")
	if err == nil {
		t.Error("expected error for nonexistent template")
	}

	// 测试未运行状态
	ca2 := New(nil)
	_, err = ca2.GenerateContent(ctx, "test", "")
	if err == nil {
		t.Error("expected error when not running")
	}
}

func TestTranslateContent(t *testing.T) {
	ca := New(nil)
	ca.Start()
	defer ca.Stop()

	ctx := context.Background()

	// 翻译到英文
	result, err := ca.TranslateContent(ctx, "你好世界", LangEnglish)
	if err != nil {
		t.Fatalf("TranslateContent failed: %v", err)
	}

	if result.SourceText != "你好世界" {
		t.Errorf("expected 你好世界, got %s", result.SourceText)
	}
	if result.TargetLang != LangEnglish {
		t.Errorf("expected en, got %s", result.TargetLang)
	}
	if result.TargetText == "" {
		t.Error("expected translated text")
	}

	// 翻译到日文
	result, err = ca.TranslateContent(ctx, "测试", LangJapanese)
	if err != nil {
		t.Fatalf("TranslateContent failed: %v", err)
	}
	if result.TargetLang != LangJapanese {
		t.Errorf("expected ja, got %s", result.TargetLang)
	}

	// 测试空文本
	_, err = ca.TranslateContent(ctx, "", LangEnglish)
	if err == nil {
		t.Error("expected error for empty text")
	}

	// 测试未运行状态
	ca2 := New(nil)
	_, err = ca2.TranslateContent(ctx, "test", LangEnglish)
	if err == nil {
		t.Error("expected error when not running")
	}
}

func TestSummarizeContent(t *testing.T) {
	ca := New(nil)
	ca.Start()
	defer ca.Stop()

	ctx := context.Background()

	text := "这是一段很长的文本，用于测试摘要功能。摘要功能可以将长文本压缩成简短的摘要，方便用户快速了解内容。在实际应用中，AI会分析文本的关键信息并生成高质量的摘要。"

	// 生成摘要
	result, err := ca.SummarizeContent(ctx, text, 50)
	if err != nil {
		t.Fatalf("SummarizeContent failed: %v", err)
	}

	if result.Summary == "" {
		t.Error("expected summary")
	}
	if result.MaxLength != 50 {
		t.Errorf("expected 50, got %d", result.MaxLength)
	}
	if result.CompressionRatio <= 0 {
		t.Error("expected positive compression ratio")
	}

	// 测试短文本（不需要摘要）
	shortText := "短文本"
	result, err = ca.SummarizeContent(ctx, shortText, 100)
	if err != nil {
		t.Fatalf("SummarizeContent failed: %v", err)
	}
	if result.Summary != shortText {
		t.Errorf("expected %s, got %s", shortText, result.Summary)
	}

	// 测试空文本
	_, err = ca.SummarizeContent(ctx, "", 100)
	if err == nil {
		t.Error("expected error for empty text")
	}

	// 测试未运行状态
	ca2 := New(nil)
	_, err = ca2.SummarizeContent(ctx, "test", 100)
	if err == nil {
		t.Error("expected error when not running")
	}
}

func TestAnalyzeSEO(t *testing.T) {
	ca := New(nil)
	ca.Start()
	defer ca.Stop()

	ctx := context.Background()

	content := "# SEO优化指南\n\nSEO优化是提高网站排名的重要手段。通过合理的SEO优化，可以让网站在搜索引擎中获得更好的排名。\n\n## 关键词优化\n\n关键词是SEO优化的核心。选择合适的关键词并合理使用，可以有效提高网站的可见性。"

	keywords := []string{"SEO优化", "关键词", "搜索引擎"}

	// 分析SEO
	result, err := ca.AnalyzeSEO(ctx, content, keywords)
	if err != nil {
		t.Fatalf("AnalyzeSEO failed: %v", err)
	}

	if result.Score < 0 || result.Score > 100 {
		t.Errorf("expected score 0-100, got %d", result.Score)
	}
	if len(result.Keywords) != len(keywords) {
		t.Errorf("expected %d keyword densities, got %d", len(keywords), len(result.Keywords))
	}
	if result.MetaTitle == "" {
		t.Error("expected meta title")
	}
	if result.Readability <= 0 {
		t.Error("expected positive readability score")
	}

	// 测试空内容
	_, err = ca.AnalyzeSEO(ctx, "", keywords)
	if err == nil {
		t.Error("expected error for empty content")
	}

	// 测试未运行状态
	ca2 := New(nil)
	_, err = ca2.AnalyzeSEO(ctx, "test", keywords)
	if err == nil {
		t.Error("expected error when not running")
	}
}

func TestImproveWriting(t *testing.T) {
	ca := New(nil)
	ca.Start()
	defer ca.Stop()

	ctx := context.Background()

	// 改进写作
	text := "这是一段测试文本。  包含一些问题，"
	result, err := ca.ImproveWriting(ctx, text)
	if err != nil {
		t.Fatalf("ImproveWriting failed: %v", err)
	}

	if result.Original != text {
		t.Error("original text mismatch")
	}
	if result.Improved == "" {
		t.Error("expected improved text")
	}
	if result.ScoreBefore <= 0 {
		t.Error("expected positive score before")
	}
	if result.ScoreAfter <= 0 {
		t.Error("expected positive score after")
	}

	// 测试空文本
	_, err = ca.ImproveWriting(ctx, "")
	if err == nil {
		t.Error("expected error for empty text")
	}

	// 测试未运行状态
	ca2 := New(nil)
	_, err = ca2.ImproveWriting(ctx, "test")
	if err == nil {
		t.Error("expected error when not running")
	}
}

func TestGenerateOutline(t *testing.T) {
	ca := New(nil)
	ca.Start()
	defer ca.Stop()

	ctx := context.Background()

	// 生成大纲
	result, err := ca.GenerateOutline(ctx, "人工智能技术")
	if err != nil {
		t.Fatalf("GenerateOutline failed: %v", err)
	}

	if result.Topic != "人工智能技术" {
		t.Errorf("expected 人工智能技术, got %s", result.Topic)
	}
	if len(result.Sections) == 0 {
		t.Error("expected sections")
	}
	if result.Language == "" {
		t.Error("expected language")
	}

	// 检查章节结构
	for _, section := range result.Sections {
		if section.Title == "" {
			t.Error("expected section title")
		}
		if section.Level < 1 {
			t.Error("expected positive level")
		}
	}

	// 测试空主题
	_, err = ca.GenerateOutline(ctx, "")
	if err == nil {
		t.Error("expected error for empty topic")
	}

	// 测试未运行状态
	ca2 := New(nil)
	_, err = ca2.GenerateOutline(ctx, "test")
	if err == nil {
		t.Error("expected error when not running")
	}
}

func TestCheckGrammar(t *testing.T) {
	ca := New(nil)
	ca.Start()
	defer ca.Stop()

	ctx := context.Background()

	// 检查语法
	text := "这是一段测试文本。  包含一些问题，，需要检查。"
	result, err := ca.CheckGrammar(ctx, text)
	if err != nil {
		t.Fatalf("CheckGrammar failed: %v", err)
	}

	if result.Text != text {
		t.Error("original text mismatch")
	}
	if result.Score < 0 || result.Score > 100 {
		t.Errorf("expected score 0-100, got %d", result.Score)
	}
	if result.CorrectedText == "" {
		t.Error("expected corrected text")
	}
	if result.CreatedAt.IsZero() {
		t.Error("expected created at time")
	}

	// 测试正确文本
	correctText := "这是一段正确的文本。"
	result, err = ca.CheckGrammar(ctx, correctText)
	if err != nil {
		t.Fatalf("CheckGrammar failed: %v", err)
	}
	if len(result.Errors) > 0 {
		t.Errorf("expected no errors, got %d", len(result.Errors))
	}

	// 测试空文本
	_, err = ca.CheckGrammar(ctx, "")
	if err == nil {
		t.Error("expected error for empty text")
	}

	// 测试未运行状态
	ca2 := New(nil)
	_, err = ca2.CheckGrammar(ctx, "test")
	if err == nil {
		t.Error("expected error when not running")
	}
}

func TestListTasks(t *testing.T) {
	ca := New(nil)
	ca.Start()
	defer ca.Stop()

	ctx := context.Background()

	// 创建多个任务
	ca.GenerateContent(ctx, "任务1", "")
	ca.GenerateContent(ctx, "任务2", "")
	ca.GenerateContent(ctx, "任务3", "")

	// 等待任务完成
	time.Sleep(300 * time.Millisecond)

	// 列出所有任务
	tasks := ca.ListTasks("", "")
	if len(tasks) < 3 {
		t.Errorf("expected at least 3 tasks, got %d", len(tasks))
	}

	// 按状态筛选
	completedTasks := ca.ListTasks("", ContentStatusCompleted)
	if len(completedTasks) == 0 {
		t.Error("expected completed tasks")
	}
}

func TestGetStats(t *testing.T) {
	ca := New(nil)
	ca.Start()
	defer ca.Stop()

	ctx := context.Background()

	// 创建一些任务
	ca.GenerateContent(ctx, "任务1", "")
	ca.GenerateContent(ctx, "任务2", "")

	// 等待任务完成
	time.Sleep(300 * time.Millisecond)

	// 获取统计
	stats := ca.GetStats()
	if stats == nil {
		t.Fatal("GetStats returned nil")
	}
	if stats.TotalTasks < 2 {
		t.Errorf("expected at least 2 tasks, got %d", stats.TotalTasks)
	}
	if stats.TotalTemplates < 4 {
		t.Errorf("expected at least 4 templates, got %d", stats.TotalTemplates)
	}
	if stats.CompletedTasks == 0 {
		t.Error("expected completed tasks")
	}
}

func TestGetConfig(t *testing.T) {
	config := &Config{
		ModelPath:   "/test/path",
		Language:    LangEnglish,
		MaxTokens:   1024,
		Temperature: 0.5,
	}

	ca := New(config)
	got := ca.GetConfig()

	if got.ModelPath != "/test/path" {
		t.Errorf("expected /test/path, got %s", got.ModelPath)
	}
	if got.Language != LangEnglish {
		t.Errorf("expected en, got %s", got.Language)
	}
	if got.MaxTokens != 1024 {
		t.Errorf("expected 1024, got %d", got.MaxTokens)
	}
}
