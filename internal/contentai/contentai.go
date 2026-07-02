package contentai

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"
)

// ContentType 内容类型.
type ContentType string

const (
	ContentTypeArticle   ContentType = "article"
	ContentTypeBlog      ContentType = "blog"
	ContentTypeSocial    ContentType = "social"
	ContentTypeEmail     ContentType = "email"
	ContentTypeProduct   ContentType = "product"
	ContentTypeSEO       ContentType = "seo"
	ContentTypeTechnical ContentType = "technical"
	ContentTypeCreative  ContentType = "creative"
)

// ContentStatus 内容状态.
type ContentStatus string

const (
	ContentStatusDraft     ContentStatus = "draft"
	ContentStatusPending   ContentStatus = "pending"
	ContentStatusCompleted ContentStatus = "completed"
	ContentStatusFailed    ContentStatus = "failed"
)

// Language 语言.
type Language string

const (
	LangChinese  Language = "zh"
	LangEnglish  Language = "en"
	LangJapanese Language = "ja"
	LangKorean   Language = "ko"
	LangFrench   Language = "fr"
	LangGerman   Language = "de"
	LangSpanish  Language = "es"
)

// WritingStyle 写作风格.
type WritingStyle string

const (
	StyleFormal    WritingStyle = "formal"
	StyleCasual    WritingStyle = "casual"
	StyleTechnical WritingStyle = "technical"
	StyleCreative  WritingStyle = "creative"
	StyleAcademic  WritingStyle = "academic"
	StyleBusiness  WritingStyle = "business"
)

// Config 配置.
type Config struct {
	ModelPath     string        `json:"model_path"`
	Language      Language      `json:"language"`
	MaxTokens     int           `json:"max_tokens"`
	Temperature   float64       `json:"temperature"`
	TopP          float64       `json:"top_p"`
	MaxConcurrent int           `json:"max_concurrent"`
	Timeout       time.Duration `json:"timeout"`
	TemplateDir   string        `json:"template_dir"`
}

// DefaultConfig 返回默认配置.
func DefaultConfig() *Config {
	return &Config{
		ModelPath:     "/models/contentai",
		Language:      LangChinese,
		MaxTokens:     4096,
		Temperature:   0.7,
		TopP:          0.9,
		MaxConcurrent: 5,
		Timeout:       60 * time.Second,
		TemplateDir:   "/templates/contentai",
	}
}

// ContentTemplate 内容模板.
type ContentTemplate struct {
	ID          string       `json:"id"`
	Name        string       `json:"name"`
	Description string       `json:"description"`
	Type        ContentType  `json:"type"`
	Language    Language     `json:"language"`
	Style       WritingStyle `json:"style"`
	Template    string       `json:"template"`
	Variables   []string     `json:"variables"`
	Tags        []string     `json:"tags"`
	IsDefault   bool         `json:"is_default"`
	CreatedAt   time.Time    `json:"created_at"`
	UpdatedAt   time.Time    `json:"updated_at"`
}

// ContentTask 内容任务.
type ContentTask struct {
	ID          string                 `json:"id"`
	UserID      string                 `json:"user_id"`
	Type        ContentType            `json:"type"`
	Prompt      string                 `json:"prompt"`
	TemplateID  string                 `json:"template_id,omitempty"`
	Language    Language               `json:"language"`
	Style       WritingStyle           `json:"style"`
	MaxLength   int                    `json:"max_length"`
	Keywords    []string               `json:"keywords,omitempty"`
	Params      map[string]interface{} `json:"params,omitempty"`
	Status      ContentStatus          `json:"status"`
	Result      *ContentResult         `json:"result,omitempty"`
	Error       string                 `json:"error,omitempty"`
	CreatedAt   time.Time              `json:"created_at"`
	CompletedAt *time.Time             `json:"completed_at,omitempty"`
}

// ContentResult 内容结果.
type ContentResult struct {
	TaskID    string                 `json:"task_id"`
	Content   string                 `json:"content"`
	Title     string                 `json:"title,omitempty"`
	Summary   string                 `json:"summary,omitempty"`
	Keywords  []string               `json:"keywords,omitempty"`
	WordCount int                    `json:"word_count"`
	CharCount int                    `json:"char_count"`
	Language  Language               `json:"language"`
	Style     WritingStyle           `json:"style"`
	Metadata  map[string]interface{} `json:"metadata,omitempty"`
	CreatedAt time.Time              `json:"created_at"`
}

// SEOSuggestion SEO建议.
type SEOSuggestion struct {
	Type        string `json:"type"`
	Title       string `json:"title"`
	Description string `json:"description"`
	Priority    int    `json:"priority"`
	Impact      string `json:"impact"`
	Suggestion  string `json:"suggestion"`
}

// SEOAnalysis SEO分析结果.
type SEOAnalysis struct {
	ContentID   string           `json:"content_id"`
	Score       int              `json:"score"`
	Keywords    []KeywordDensity `json:"keywords"`
	Suggestions []SEOSuggestion  `json:"suggestions"`
	MetaTitle   string           `json:"meta_title"`
	MetaDesc    string           `json:"meta_desc"`
	Readability int              `json:"readability"`
	CreatedAt   time.Time        `json:"created_at"`
}

// KeywordDensity 关键词密度.
type KeywordDensity struct {
	Keyword string  `json:"keyword"`
	Count   int     `json:"count"`
	Density float64 `json:"density"`
}

// GrammarError 语法错误.
type GrammarError struct {
	Start      int    `json:"start"`
	End        int    `json:"end"`
	ErrorText  string `json:"error_text"`
	Message    string `json:"message"`
	Suggestion string `json:"suggestion"`
	Severity   string `json:"severity"`
}

// GrammarCheckResult 语法检查结果.
type GrammarCheckResult struct {
	Text          string         `json:"text"`
	Errors        []GrammarError `json:"errors"`
	Score         int            `json:"score"`
	CorrectedText string         `json:"corrected_text"`
	CreatedAt     time.Time      `json:"created_at"`
}

// TranslationResult 翻译结果.
type TranslationResult struct {
	SourceText string   `json:"source_text"`
	TargetText string   `json:"target_text"`
	SourceLang Language `json:"source_lang"`
	TargetLang Language `json:"target_lang"`
	WordCount  int      `json:"word_count"`
	CharCount  int      `json:"char_count"`
}

// SummaryResult 摘要结果.
type SummaryResult struct {
	OriginalText     string  `json:"original_text"`
	Summary          string  `json:"summary"`
	MaxLength        int     `json:"max_length"`
	WordCount        int     `json:"word_count"`
	CompressionRatio float64 `json:"compression_ratio"`
}

// OutlineSection 大纲章节.
type OutlineSection struct {
	Level    int               `json:"level"`
	Title    string            `json:"title"`
	Content  string            `json:"content,omitempty"`
	Children []*OutlineSection `json:"children,omitempty"`
}

// OutlineResult 大纲结果.
type OutlineResult struct {
	Topic    string            `json:"topic"`
	Sections []*OutlineSection `json:"sections"`
	Language Language          `json:"language"`
}

// WritingImprovement 写作改进.
type WritingImprovement struct {
	Original    string   `json:"original"`
	Improved    string   `json:"improved"`
	Changes     []string `json:"changes"`
	ScoreBefore int      `json:"score_before"`
	ScoreAfter  int      `json:"score_after"`
}

// ContentStats 内容统计.
type ContentStats struct {
	TotalTasks     int            `json:"total_tasks"`
	CompletedTasks int            `json:"completed_tasks"`
	FailedTasks    int            `json:"failed_tasks"`
	TotalTemplates int            `json:"total_templates"`
	TasksByType    map[string]int `json:"tasks_by_type"`
	TasksByLang    map[string]int `json:"tasks_by_lang"`
}

// ContentAI AI内容创作助手.
type ContentAI struct {
	mu          sync.RWMutex
	config      *Config
	tasks       map[string]*ContentTask
	templates   map[string]*ContentTemplate
	isRunning   bool
	stopCh      chan struct{}
	taskCounter int64
}

// New 创建ContentAI实例.
func New(config *Config) *ContentAI {
	if config == nil {
		config = DefaultConfig()
	}

	ca := &ContentAI{
		config:    config,
		tasks:     make(map[string]*ContentTask),
		templates: make(map[string]*ContentTemplate),
		stopCh:    make(chan struct{}),
	}

	ca.initDefaultTemplates()
	return ca
}

// initDefaultTemplates 初始化默认模板.
func (ca *ContentAI) initDefaultTemplates() {
	defaultTemplates := []*ContentTemplate{
		{
			ID:          "tpl_article_zh",
			Name:        "中文文章模板",
			Description: "标准中文文章写作模板",
			Type:        ContentTypeArticle,
			Language:    LangChinese,
			Style:       StyleFormal,
			Template:    "# {{title}}\n\n## 引言\n{{introduction}}\n\n## 正文\n{{body}}\n\n## 结论\n{{conclusion}}",
			Variables:   []string{"title", "introduction", "body", "conclusion"},
			Tags:        []string{"article", "chinese", "formal"},
			IsDefault:   true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "tpl_blog_en",
			Name:        "English Blog Template",
			Description: "Standard English blog writing template",
			Type:        ContentTypeBlog,
			Language:    LangEnglish,
			Style:       StyleCasual,
			Template:    "# {{title}}\n\n*By {{author}} | {{date}}*\n\n{{content}}\n\n---\n*Tags: {{tags}}*",
			Variables:   []string{"title", "author", "date", "content", "tags"},
			Tags:        []string{"blog", "english", "casual"},
			IsDefault:   true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "tpl_seo_meta",
			Name:        "SEO Meta Template",
			Description: "SEO meta description and title template",
			Type:        ContentTypeSEO,
			Language:    LangEnglish,
			Style:       StyleBusiness,
			Template:    "Title: {{meta_title}}\nDescription: {{meta_description}}\nKeywords: {{keywords}}",
			Variables:   []string{"meta_title", "meta_description", "keywords"},
			Tags:        []string{"seo", "meta", "business"},
			IsDefault:   true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
		{
			ID:          "tpl_product_zh",
			Name:        "产品描述模板",
			Description: "中文产品描述写作模板",
			Type:        ContentTypeProduct,
			Language:    LangChinese,
			Style:       StyleBusiness,
			Template:    "## {{product_name}}\n\n### 产品特点\n{{features}}\n\n### 技术参数\n{{specifications}}\n\n### 使用场景\n{{use_cases}}",
			Variables:   []string{"product_name", "features", "specifications", "use_cases"},
			Tags:        []string{"product", "chinese", "business"},
			IsDefault:   true,
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		},
	}

	for _, tpl := range defaultTemplates {
		ca.templates[tpl.ID] = tpl
	}
}

// Start 启动ContentAI.
func (ca *ContentAI) Start() error {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	if ca.isRunning {
		return fmt.Errorf("ContentAI已经在运行")
	}

	ca.isRunning = true
	ca.stopCh = make(chan struct{})
	return nil
}

// Stop 停止ContentAI.
func (ca *ContentAI) Stop() error {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	if !ca.isRunning {
		return fmt.Errorf("ContentAI未在运行")
	}

	close(ca.stopCh)
	ca.isRunning = false
	return nil
}

// IsRunning 检查是否运行中.
func (ca *ContentAI) IsRunning() bool {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	return ca.isRunning
}

// GetConfig 获取配置.
func (ca *ContentAI) GetConfig() *Config {
	ca.mu.RLock()
	defer ca.mu.RUnlock()
	return ca.config
}

// generateID 生成唯一ID.
func (ca *ContentAI) generateID(prefix string) string {
	ca.taskCounter++
	return fmt.Sprintf("%s_%d_%d", prefix, time.Now().UnixNano(), ca.taskCounter)
}

// GenerateContent 生成内容.
func (ca *ContentAI) GenerateContent(ctx context.Context, prompt string, templateID string) (*ContentTask, error) {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	if !ca.isRunning {
		return nil, fmt.Errorf("ContentAI未在运行")
	}

	// 验证模板（如果提供）
	var tpl *ContentTemplate
	if templateID != "" {
		var exists bool
		tpl, exists = ca.templates[templateID]
		if !exists {
			return nil, fmt.Errorf("模板 %s 不存在", templateID)
		}
	}

	// 创建任务
	task := &ContentTask{
		ID:         ca.generateID("task"),
		Type:       ContentTypeArticle,
		Prompt:     prompt,
		TemplateID: templateID,
		Language:   ca.config.Language,
		MaxLength:  ca.config.MaxTokens,
		Status:     ContentStatusPending,
		CreatedAt:  time.Now(),
	}

	if tpl != nil {
		task.Type = tpl.Type
		task.Style = tpl.Style
		task.Language = tpl.Language
	}

	ca.tasks[task.ID] = task

	// 异步处理任务
	go ca.processTask(task)

	return task, nil
}

// processTask 处理内容生成任务.
func (ca *ContentAI) processTask(task *ContentTask) {
	ca.mu.Lock()
	task.Status = ContentStatusPending
	ca.mu.Unlock()

	// 模拟AI生成（实际应调用模型）
	time.Sleep(100 * time.Millisecond)

	ca.mu.Lock()
	defer ca.mu.Unlock()

	// 生成模拟内容
	content := ca.generateMockContent(task)

	now := time.Now()
	task.Status = ContentStatusCompleted
	task.CompletedAt = &now
	task.Result = &ContentResult{
		TaskID:    task.ID,
		Content:   content,
		Title:     ca.extractTitle(content),
		WordCount: len(strings.Fields(content)),
		CharCount: len(content),
		Language:  task.Language,
		Style:     task.Style,
		CreatedAt: now,
	}
}

// generateMockContent 生成模拟内容.
func (ca *ContentAI) generateMockContent(task *ContentTask) string {
	// 简单的模拟生成
	return fmt.Sprintf("根据提示「%s」生成的内容。\n\n这是一段由AI生成的示例内容，用于演示ContentAI模块的功能。在实际应用中，这里会调用本地AI模型来生成高质量的内容。", task.Prompt)
}

// extractTitle 提取标题.
func (ca *ContentAI) extractTitle(content string) string {
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) > 0 {
		title := strings.TrimLeft(lines[0], "# ")
		if title != "" {
			return title
		}
	}
	return "Untitled"
}

// GetTask 获取任务.
func (ca *ContentAI) GetTask(taskID string) (*ContentTask, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	task, exists := ca.tasks[taskID]
	if !exists {
		return nil, fmt.Errorf("任务 %s 不存在", taskID)
	}
	return task, nil
}

// ListTasks 列出任务.
func (ca *ContentAI) ListTasks(userID string, status ContentStatus) []*ContentTask {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	tasks := make([]*ContentTask, 0)
	for _, task := range ca.tasks {
		if userID != "" && task.UserID != userID {
			continue
		}
		if status != "" && task.Status != status {
			continue
		}
		tasks = append(tasks, task)
	}
	return tasks
}

// TranslateContent 翻译内容.
func (ca *ContentAI) TranslateContent(ctx context.Context, text string, targetLang Language) (*TranslationResult, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	if !ca.isRunning {
		return nil, fmt.Errorf("ContentAI未在运行")
	}

	if text == "" {
		return nil, fmt.Errorf("翻译文本不能为空")
	}

	// 模拟翻译（实际应调用翻译模型）
	translated := ca.mockTranslate(text, targetLang)

	return &TranslationResult{
		SourceText: text,
		TargetText: translated,
		SourceLang: ca.config.Language,
		TargetLang: targetLang,
		WordCount:  len(strings.Fields(translated)),
		CharCount:  len(translated),
	}, nil
}

// mockTranslate 模拟翻译.
func (ca *ContentAI) mockTranslate(text string, targetLang Language) string {
	// 简单的模拟翻译
	switch targetLang {
	case LangEnglish:
		return "[Translated to English] " + text
	case LangChinese:
		return "[翻译为中文] " + text
	case LangJapanese:
		return "[日本語に翻訳] " + text
	default:
		return "[Translated] " + text
	}
}

// SummarizeContent 摘要内容.
func (ca *ContentAI) SummarizeContent(ctx context.Context, text string, maxLength int) (*SummaryResult, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	if !ca.isRunning {
		return nil, fmt.Errorf("ContentAI未在运行")
	}

	if text == "" {
		return nil, fmt.Errorf("摘要文本不能为空")
	}

	if maxLength <= 0 {
		maxLength = 200
	}

	// 模拟摘要生成
	summary := ca.mockSummarize(text, maxLength)

	return &SummaryResult{
		OriginalText:     text,
		Summary:          summary,
		MaxLength:        maxLength,
		WordCount:        len(strings.Fields(summary)),
		CompressionRatio: float64(len(summary)) / float64(len(text)),
	}, nil
}

// mockSummarize 模拟摘要.
func (ca *ContentAI) mockSummarize(text string, maxLength int) string {
	if len(text) <= maxLength {
		return text
	}
	// 简单截断
	runes := []rune(text)
	if len(runes) > maxLength {
		return string(runes[:maxLength]) + "..."
	}
	return text
}

// AnalyzeSEO SEO分析.
func (ca *ContentAI) AnalyzeSEO(ctx context.Context, content string, keywords []string) (*SEOAnalysis, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	if !ca.isRunning {
		return nil, fmt.Errorf("ContentAI未在运行")
	}

	if content == "" {
		return nil, fmt.Errorf("分析内容不能为空")
	}

	// 计算关键词密度
	keywordDensities := ca.calculateKeywordDensity(content, keywords)

	// 生成SEO建议
	suggestions := ca.generateSEOSuggestions(content, keywords)

	// 计算SEO分数
	score := ca.calculateSEOScore(content, keywordDensities, suggestions)

	return &SEOAnalysis{
		ContentID:   ca.generateID("seo"),
		Score:       score,
		Keywords:    keywordDensities,
		Suggestions: suggestions,
		MetaTitle:   ca.extractTitle(content),
		MetaDesc:    ca.generateMetaDescription(content),
		Readability: ca.calculateReadability(content),
		CreatedAt:   time.Now(),
	}, nil
}

// calculateKeywordDensity 计算关键词密度.
func (ca *ContentAI) calculateKeywordDensity(content string, keywords []string) []KeywordDensity {
	contentLower := strings.ToLower(content)
	totalWords := len(strings.Fields(content))

	densities := make([]KeywordDensity, 0, len(keywords))
	for _, kw := range keywords {
		count := strings.Count(contentLower, strings.ToLower(kw))
		density := 0.0
		if totalWords > 0 {
			density = float64(count) / float64(totalWords) * 100
		}
		densities = append(densities, KeywordDensity{
			Keyword: kw,
			Count:   count,
			Density: density,
		})
	}
	return densities
}

// generateSEOSuggestions 生成SEO建议.
func (ca *ContentAI) generateSEOSuggestions(content string, keywords []string) []SEOSuggestion {
	suggestions := make([]SEOSuggestion, 0)

	contentLower := strings.ToLower(content)
	wordCount := len(strings.Fields(content))

	// 检查内容长度
	if wordCount < 300 {
		suggestions = append(suggestions, SEOSuggestion{
			Type:        "content_length",
			Title:       "内容长度不足",
			Description: "建议内容至少300字以获得更好的SEO效果",
			Priority:    1,
			Impact:      "high",
			Suggestion:  "增加更多有价值的内容，目标字数建议在1000-2000字之间",
		})
	}

	// 检查关键词使用
	for _, kw := range keywords {
		count := strings.Count(contentLower, strings.ToLower(kw))
		if count == 0 {
			suggestions = append(suggestions, SEOSuggestion{
				Type:        "keyword_missing",
				Title:       fmt.Sprintf("缺少关键词: %s", kw),
				Description: fmt.Sprintf("内容中未找到关键词「%s」", kw),
				Priority:    2,
				Impact:      "high",
				Suggestion:  fmt.Sprintf("在标题、首段和正文中合理使用关键词「%s」", kw),
			})
		} else if count > 10 {
			suggestions = append(suggestions, SEOSuggestion{
				Type:        "keyword_stuffing",
				Title:       fmt.Sprintf("关键词堆砌: %s", kw),
				Description: fmt.Sprintf("关键词「%s」出现次数过多(%d次)，可能被视为关键词堆砌", kw, count),
				Priority:    2,
				Impact:      "medium",
				Suggestion:  "适当减少关键词使用频率，保持自然的语言表达",
			})
		}
	}

	// 检查标题
	if !strings.Contains(content, "#") {
		suggestions = append(suggestions, SEOSuggestion{
			Type:        "heading_missing",
			Title:       "缺少标题结构",
			Description: "内容没有使用标题标签",
			Priority:    3,
			Impact:      "medium",
			Suggestion:  "使用H1-H6标签构建清晰的内容层次结构",
		})
	}

	return suggestions
}

// calculateSEOScore 计算SEO分数.
func (ca *ContentAI) calculateSEOScore(content string, densities []KeywordDensity, suggestions []SEOSuggestion) int {
	score := 100

	// 根据建议扣分
	for _, s := range suggestions {
		switch s.Impact {
		case "high":
			score -= 15
		case "medium":
			score -= 10
		case "low":
			score -= 5
		}
	}

	// 检查关键词密度
	for _, d := range densities {
		if d.Density < 1.0 {
			score -= 5
		} else if d.Density > 3.0 {
			score -= 10
		}
	}

	if score < 0 {
		score = 0
	}
	return score
}

// generateMetaDescription 生成meta描述.
func (ca *ContentAI) generateMetaDescription(content string) string {
	// 取前160个字符作为描述
	runes := []rune(content)
	if len(runes) > 160 {
		return string(runes[:160]) + "..."
	}
	return content
}

// calculateReadability 计算可读性分数.
func (ca *ContentAI) calculateReadability(content string) int {
	// 简单的可读性计算
	sentences := strings.Split(content, "。")
	words := strings.Fields(content)

	if len(sentences) == 0 || len(words) == 0 {
		return 50
	}

	avgWordsPerSentence := float64(len(words)) / float64(len(sentences))

	// 简单评分：句子越短，可读性越高
	if avgWordsPerSentence < 15 {
		return 90
	} else if avgWordsPerSentence < 25 {
		return 70
	} else {
		return 50
	}
}

// ImproveWriting 改进写作.
func (ca *ContentAI) ImproveWriting(ctx context.Context, text string) (*WritingImprovement, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	if !ca.isRunning {
		return nil, fmt.Errorf("ContentAI未在运行")
	}

	if text == "" {
		return nil, fmt.Errorf("改进文本不能为空")
	}

	// 模拟改进写作
	improved, changes := ca.mockImproveWriting(text)

	return &WritingImprovement{
		Original:    text,
		Improved:    improved,
		Changes:     changes,
		ScoreBefore: ca.calculateTextScore(text),
		ScoreAfter:  ca.calculateTextScore(improved),
	}, nil
}

// mockImproveWriting 模拟改进写作.
func (ca *ContentAI) mockImproveWriting(text string) (string, []string) {
	changes := make([]string, 0)
	improved := text

	// 简单的改进模拟
	if strings.Contains(improved, "。  ") {
		improved = strings.ReplaceAll(improved, "。  ", "。")
		changes = append(changes, "修复多余空格")
	}

	if !strings.HasSuffix(improved, "。") && !strings.HasSuffix(improved, "！") && !strings.HasSuffix(improved, "？") {
		improved += "。"
		changes = append(changes, "添加结尾标点")
	}

	return improved, changes
}

// calculateTextScore 计算文本质量分数.
func (ca *ContentAI) calculateTextScore(text string) int {
	score := 70

	// 检查标点
	if strings.HasSuffix(text, "。") || strings.HasSuffix(text, "！") || strings.HasSuffix(text, "？") {
		score += 10
	}

	// 检查长度
	wordCount := len(strings.Fields(text))
	if wordCount > 50 {
		score += 10
	}

	if score > 100 {
		score = 100
	}
	return score
}

// AddTemplate 添加模板.
func (ca *ContentAI) AddTemplate(template *ContentTemplate) error {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	if template.ID == "" {
		template.ID = ca.generateID("tpl")
	}

	if _, exists := ca.templates[template.ID]; exists {
		return fmt.Errorf("模板 %s 已存在", template.ID)
	}

	now := time.Now()
	template.CreatedAt = now
	template.UpdatedAt = now

	ca.templates[template.ID] = template
	return nil
}

// GetTemplate 获取模板.
func (ca *ContentAI) GetTemplate(templateID string) (*ContentTemplate, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	tpl, exists := ca.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("模板 %s 不存在", templateID)
	}
	return tpl, nil
}

// GetTemplates 获取模板列表.
func (ca *ContentAI) GetTemplates() []*ContentTemplate {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	templates := make([]*ContentTemplate, 0, len(ca.templates))
	for _, tpl := range ca.templates {
		templates = append(templates, tpl)
	}
	return templates
}

// UpdateTemplate 更新模板.
func (ca *ContentAI) UpdateTemplate(template *ContentTemplate) error {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	if _, exists := ca.templates[template.ID]; !exists {
		return fmt.Errorf("模板 %s 不存在", template.ID)
	}

	template.UpdatedAt = time.Now()
	ca.templates[template.ID] = template
	return nil
}

// DeleteTemplate 删除模板.
func (ca *ContentAI) DeleteTemplate(templateID string) error {
	ca.mu.Lock()
	defer ca.mu.Unlock()

	if _, exists := ca.templates[templateID]; !exists {
		return fmt.Errorf("模板 %s 不存在", templateID)
	}

	delete(ca.templates, templateID)
	return nil
}

// GenerateOutline 生成大纲.
func (ca *ContentAI) GenerateOutline(ctx context.Context, topic string) (*OutlineResult, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	if !ca.isRunning {
		return nil, fmt.Errorf("ContentAI未在运行")
	}

	if topic == "" {
		return nil, fmt.Errorf("主题不能为空")
	}

	// 模拟大纲生成
	outline := ca.mockGenerateOutline(topic)

	return outline, nil
}

// mockGenerateOutline 模拟大纲生成.
func (ca *ContentAI) mockGenerateOutline(topic string) *OutlineResult {
	return &OutlineResult{
		Topic: topic,
		Sections: []*OutlineSection{
			{
				Level:   1,
				Title:   fmt.Sprintf("引言 - %s", topic),
				Content: "介绍主题背景和重要性",
			},
			{
				Level:   1,
				Title:   "背景分析",
				Content: "分析相关背景和现状",
				Children: []*OutlineSection{
					{
						Level:   2,
						Title:   "历史发展",
						Content: "发展历程和演变",
					},
					{
						Level:   2,
						Title:   "当前状态",
						Content: "现状分析和数据",
					},
				},
			},
			{
				Level:   1,
				Title:   "核心内容",
				Content: "详细论述",
				Children: []*OutlineSection{
					{
						Level:   2,
						Title:   "要点一",
						Content: "第一个核心要点",
					},
					{
						Level:   2,
						Title:   "要点二",
						Content: "第二个核心要点",
					},
					{
						Level:   2,
						Title:   "要点三",
						Content: "第三个核心要点",
					},
				},
			},
			{
				Level:   1,
				Title:   "案例分析",
				Content: "相关案例和实践经验",
			},
			{
				Level:   1,
				Title:   "总结与展望",
				Content: "总结主要观点，展望未来发展",
			},
		},
		Language: ca.config.Language,
	}
}

// CheckGrammar 语法检查.
func (ca *ContentAI) CheckGrammar(ctx context.Context, text string) (*GrammarCheckResult, error) {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	if !ca.isRunning {
		return nil, fmt.Errorf("ContentAI未在运行")
	}

	if text == "" {
		return nil, fmt.Errorf("检查文本不能为空")
	}

	// 模拟语法检查
	errors := ca.mockGrammarCheck(text)
	corrected := ca.applyCorrections(text, errors)

	score := 100 - len(errors)*10
	if score < 0 {
		score = 0
	}

	return &GrammarCheckResult{
		Text:          text,
		Errors:        errors,
		Score:         score,
		CorrectedText: corrected,
		CreatedAt:     time.Now(),
	}, nil
}

// mockGrammarCheck 模拟语法检查.
func (ca *ContentAI) mockGrammarCheck(text string) []GrammarError {
	errors := make([]GrammarError, 0)

	// 检查常见问题
	if strings.Contains(text, "。  ") {
		idx := strings.Index(text, "。  ")
		errors = append(errors, GrammarError{
			Start:      idx,
			End:        idx + 3,
			ErrorText:  "。  ",
			Message:    "多余空格",
			Suggestion: "。",
			Severity:   "low",
		})
	}

	if strings.Contains(text, "，，") {
		idx := strings.Index(text, "，，")
		errors = append(errors, GrammarError{
			Start:      idx,
			End:        idx + 2,
			ErrorText:  "，，",
			Message:    "重复标点",
			Suggestion: "，",
			Severity:   "medium",
		})
	}

	return errors
}

// applyCorrections 应用修正.
func (ca *ContentAI) applyCorrections(text string, errors []GrammarError) string {
	corrected := text
	for _, err := range errors {
		if err.Suggestion != "" {
			corrected = strings.Replace(corrected, err.ErrorText, err.Suggestion, 1)
		}
	}
	return corrected
}

// GetStats 获取统计信息.
func (ca *ContentAI) GetStats() *ContentStats {
	ca.mu.RLock()
	defer ca.mu.RUnlock()

	stats := &ContentStats{
		TasksByType: make(map[string]int),
		TasksByLang: make(map[string]int),
	}

	stats.TotalTasks = len(ca.tasks)
	stats.TotalTemplates = len(ca.templates)

	for _, task := range ca.tasks {
		switch task.Status {
		case ContentStatusCompleted:
			stats.CompletedTasks++
		case ContentStatusFailed:
			stats.FailedTasks++
		}

		stats.TasksByType[string(task.Type)]++
		stats.TasksByLang[string(task.Language)]++
	}

	return stats
}
