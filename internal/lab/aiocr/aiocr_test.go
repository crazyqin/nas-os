// aiocr_test.go - 完整单元测试
package aiocr

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	assert.True(t, cfg.Enabled)
	assert.Equal(t, "builtin", cfg.Engine)
	assert.Equal(t, "chi_sim+eng", cfg.DefaultLanguage)
	assert.Equal(t, 4, cfg.Workers)
	assert.Equal(t, 100, cfg.QueueSize)
	assert.True(t, cfg.IndexEnabled)
	assert.Equal(t, 90, cfg.RetentionDays)
	assert.NotNil(t, cfg.Desensitize)
}

func TestNewEngine(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workers = 2
	cfg.QueueSize = 10

	engine, err := NewEngine(cfg)
	require.NoError(t, err)
	require.NotNil(t, engine)

	defer engine.Close()

	stats := engine.GetStats()
	assert.Equal(t, int64(0), stats.TotalRequests)
	assert.Equal(t, 0, stats.QueueLength)
}

func TestEngineSubmit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.Workers = 1
	cfg.QueueSize = 5
	cfg.IndexEnabled = false

	engine, err := NewEngine(cfg)
	require.NoError(t, err)
	defer engine.Close()

	req := &OCRRequest{
		FileID:   "file_1",
		FilePath: "/test/document.pdf",
		Language: "chi_sim+eng",
		Options: &OCROptions{
			RemoveNoise: true,
			Deskew:      true,
			Binarize:    true,
		},
		Priority: 5,
	}

	id, err := engine.Submit(req)
	require.NoError(t, err)
	assert.NotEmpty(t, id)

	// 等待处理
	time.Sleep(100 * time.Millisecond)

	stats := engine.GetStats()
	assert.True(t, stats.TotalRequests > 0)
}

func TestEngineRecognize(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IndexEnabled = false

	engine, err := NewEngine(cfg)
	require.NoError(t, err)
	defer engine.Close()

	options := &OCROptions{
		RemoveNoise:   true,
		Deskew:        true,
		Binarize:      true,
		ExtractTables: true,
		Desensitize:   true,
	}

	result, err := engine.Recognize("/test/document.pdf", options)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.NotEmpty(t, result.ID)
	assert.NotEmpty(t, result.Pages)
	assert.True(t, result.Confidence > 0)
}

func TestEngineBatchSubmit(t *testing.T) {
	cfg := DefaultConfig()
	cfg.IndexEnabled = false

	engine, err := NewEngine(cfg)
	require.NoError(t, err)
	defer engine.Close()

	files := []string{
		"/test/doc1.pdf",
		"/test/doc2.pdf",
		"/test/doc3.pdf",
	}

	options := &OCROptions{
		RemoveNoise: true,
		Binarize:    true,
	}

	batchID, err := engine.SubmitBatch(files, options)
	require.NoError(t, err)
	assert.NotEmpty(t, batchID)

	// 等待处理
	time.Sleep(500 * time.Millisecond)

	task, err := engine.GetBatchTask(batchID)
	require.NoError(t, err)
	assert.Equal(t, BatchStatusCompleted, task.Status)
	assert.Equal(t, 3, task.TotalFiles)
}

func TestPreprocessorProcess(t *testing.T) {
	cfg := DefaultConfig()
	p := NewPreprocessor(cfg)

	options := &OCROptions{
		RemoveNoise:  true,
		Deskew:       true,
		Binarize:     true,
		EnhanceImage: true,
	}

	result, err := p.Process("/test/image.png", options)
	require.NoError(t, err)
	require.NotNil(t, result)

	assert.True(t, result.Width > 0)
	assert.True(t, result.Height > 0)
	assert.Equal(t, 1, result.Pages)
}

func TestRecognizerRecognize(t *testing.T) {
	cfg := DefaultConfig()
	r := NewRecognizer(cfg)

	img := &ProcessedImage{
		Path:   "/test/image.png",
		Width:  800,
		Height: 600,
		Format: "png",
		Pages:  1,
	}

	options := &OCROptions{
		ExtractTables: true,
	}

	pages, err := r.Recognize(img, "chi_sim+eng", options)
	require.NoError(t, err)
	require.NotNil(t, pages)
	assert.True(t, len(pages) > 0)

	page := pages[0]
	assert.Equal(t, 1, page.PageNumber)
	assert.NotEmpty(t, page.Text)
	assert.True(t, page.Confidence > 0)
}

func TestRecognizerLanguages(t *testing.T) {
	cfg := DefaultConfig()
	r := NewRecognizer(cfg)

	langs := r.GetSupportedLanguages()
	assert.True(t, len(langs) > 0)

	// 测试安装语言
	err := r.InstallLanguage("fra")
	assert.NoError(t, err)

	// 测试启用语言
	err = r.EnableLanguage("fra")
	assert.NoError(t, err)

	// 测试禁用语言
	err = r.DisableLanguage("fra")
	assert.NoError(t, err)
}

func TestExtractorExtract(t *testing.T) {
	cfg := DefaultConfig()
	e := NewExtractor(cfg)

	text := `
		增值税普通发票
		发票代码：011001900411
		发票号码：07654321
		开票日期：2024年01月15日
		金额：¥1000.00
		税额：¥130.00
		价税合计：¥1130.00
		购买方：北京科技有限公司
		销售方：上海贸易有限公司
		纳税人识别号：91110108MA01XXXXX
	`

	pages := []*PageResult{
		{PageNumber: 1, Text: text, Confidence: 0.95},
	}

	structured := e.Extract(text, pages, CategoryInvoice)
	require.NotNil(t, structured)

	assert.Equal(t, "invoice", structured.DocumentType)
	assert.NotEmpty(t, structured.Template)
	assert.True(t, structured.Confidence > 0)
}

func TestExtractorTemplates(t *testing.T) {
	cfg := DefaultConfig()
	e := NewExtractor(cfg)

	templates := e.GetTemplates()
	assert.True(t, len(templates) > 0)
	assert.NotNil(t, templates[CategoryInvoice])
	assert.NotNil(t, templates[CategoryContract])
	assert.NotNil(t, templates[CategoryIDCard])
}

func TestClassifierClassify(t *testing.T) {
	cfg := DefaultConfig()
	c := NewClassifier(cfg)

	tests := []struct {
		text     string
		expected DocumentCategory
	}{
		{
			text:     "增值税普通发票  发票代码  发票号码  金额  税额",
			expected: CategoryInvoice,
		},
		{
			text:     "合同  甲方  乙方  签订日期  违约责任",
			expected: CategoryContract,
		},
		{
			text:     "居民身份证  姓名  性别  民族  住址  公民身份号码",
			expected: CategoryIDCard,
		},
		{
			text:     "营业执照  统一社会信用代码  法定代表人  注册资本",
			expected: CategoryBusiness,
		},
	}

	for _, test := range tests {
		t.Run(string(test.expected), func(t *testing.T) {
			pages := []*PageResult{
				{PageNumber: 1, Text: test.text},
			}

			result := c.Classify(test.text, pages)
			require.NotNil(t, result)
			assert.Equal(t, test.expected, result.Category)
			assert.True(t, result.Confidence > 0)
		})
	}
}

func TestClassifierRules(t *testing.T) {
	cfg := DefaultConfig()
	c := NewClassifier(cfg)

	rules := c.GetRules()
	assert.True(t, len(rules) > 0)

	// 添加规则
	c.AddRule(&ClassificationRule{
		Name:     "测试规则",
		Category: CategoryOther,
		Keywords: []string{"测试"},
		Priority: 1,
		Weight:   1.0,
		Enabled:  true,
	})

	rules = c.GetRules()
	assert.Equal(t, len(rules), len(c.rules))

	// 移除规则
	c.RemoveRule("测试规则")
}

func TestValidatorValidate(t *testing.T) {
	cfg := DefaultConfig()
	v := NewValidator(cfg)

	result := &OCRResult{
		ID: "test_1",
		Structured: &StructuredData{
			Fields: map[string]interface{}{
				"id_number":   "110101199001011234",
				"card_number": "6222021234567890123",
				"phone":       "13812345678",
			},
		},
	}

	v.Validate(result)
	assert.NotNil(t, result.Structured)
}

func TestValidatorIDChecksum(t *testing.T) {
	cfg := DefaultConfig()
	v := NewValidator(cfg)

	tests := []struct {
		id      string
		isValid bool
	}{
		{"110101199001011234", false}, // 测试用例
		{"11010119900101123X", false}, // 测试用例
	}

	for _, test := range tests {
		result := v.validateIDChecksum(test.id)
		assert.Equal(t, test.isValid, result, "ID: %s", test.id)
	}
}

func TestValidatorLuhn(t *testing.T) {
	cfg := DefaultConfig()
	v := NewValidator(cfg)

	tests := []struct {
		card    string
		isValid bool
	}{
		{"6222021234567890123", false},
		{"4111111111111111", true}, // 测试卡号
	}

	for _, test := range tests {
		result := v.validateLuhn(test.card)
		assert.Equal(t, test.isValid, result, "Card: %s", test.card)
	}
}

func TestArchiverArchive(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ArchivePath = "/tmp/test_ocr_archive"
	cfg.RetentionDays = 1

	a := NewArchiver(cfg)

	result := &OCRResult{
		ID:         "test_archive_1",
		FileID:     "file_1",
		FileName:   "test.pdf",
		FullText:   "测试文档内容",
		Template:   "invoice",
		Language:   "chi_sim+eng",
		Confidence: 0.95,
		CreatedAt:  time.Now(),
	}

	err := a.Archive(result)
	require.NoError(t, err)

	// 获取文档
	doc, err := a.GetDocument("test_archive_1")
	require.NoError(t, err)
	assert.Equal(t, "test_archive_1", doc.ID)
}

func TestArchiverSearch(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ArchivePath = "/tmp/test_ocr_archive"

	a := NewArchiver(cfg)

	// 先归档一个文档
	result := &OCRResult{
		ID:        "test_search_1",
		FileID:    "file_1",
		FileName:  "test.pdf",
		FullText:  "测试发票内容",
		Template:  "invoice",
		Language:  "chi_sim+eng",
		CreatedAt: time.Now(),
	}

	a.Archive(result)

	// 搜索
	query := &SearchQuery{
		Keyword:  "发票",
		Category: "invoice",
		Limit:    10,
	}

	results, err := a.Search(query)
	require.NoError(t, err)
	assert.True(t, len(results) > 0)
}

func TestArchiverCleanup(t *testing.T) {
	cfg := DefaultConfig()
	cfg.ArchivePath = "/tmp/test_ocr_archive_cleanup"
	cfg.RetentionDays = 0 // 立即过期

	a := NewArchiver(cfg)

	// 归档一个文档
	result := &OCRResult{
		ID:        "test_cleanup_1",
		FileID:    "file_1",
		FullText:  "测试",
		Template:  "other",
		CreatedAt: time.Now(),
	}

	a.Archive(result)

	// 清理
	deleted, err := a.Cleanup()
	require.NoError(t, err)
	assert.Equal(t, 1, deleted)
}

func TestComplianceDesensitizeText(t *testing.T) {
	cfg := DefaultDesensitizeConfig()
	c := NewCompliance(cfg)

	tests := []struct {
		name     string
		input    string
		expected bool
	}{
		{
			name:     "身份证号",
			input:    "公民身份号码：110101199001011234",
			expected: true,
		},
		{
			name:     "手机号",
			input:    "联系电话：13812345678",
			expected: true,
		},
		{
			name:     "邮箱",
			input:    "邮箱：test@example.com",
			expected: true,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			result := c.DesensitizeText(test.input)
			assert.NotEqual(t, test.input, result)
			assert.Contains(t, result, "*")
		})
	}
}

func TestComplianceCheckCompliance(t *testing.T) {
	cfg := DefaultDesensitizeConfig()
	c := NewCompliance(cfg)

	text := "姓名：张三，身份证号：110101199001011234，手机：13812345678"

	check := c.CheckCompliance(text)
	require.NotNil(t, check)
	assert.False(t, check.IsValid)
	assert.True(t, len(check.Violations) > 0)
}

func TestComplianceMaskIDCard(t *testing.T) {
	cfg := DefaultDesensitizeConfig()
	c := NewCompliance(cfg)

	masked := c.MaskIDCard("110101199001011234")
	assert.Equal(t, 18, len(masked))
	assert.Contains(t, masked, "*")
	assert.Equal(t, "1101", masked[:4])
	assert.Equal(t, "1234", masked[14:])
}

func TestComplianceMaskBankCard(t *testing.T) {
	cfg := DefaultDesensitizeConfig()
	c := NewCompliance(cfg)

	masked := c.MaskBankCard("6222021234567890123")
	assert.Contains(t, masked, "*")
	assert.Equal(t, "6222", masked[:4])
}

func TestComplianceMaskPhone(t *testing.T) {
	cfg := DefaultDesensitizeConfig()
	c := NewCompliance(cfg)

	masked := c.MaskPhone("13812345678")
	assert.Equal(t, 11, len(masked))
	assert.Contains(t, masked, "*")
	assert.Equal(t, "138", masked[:3])
	assert.Equal(t, "5678", masked[7:])
}

func TestComplianceMaskEmail(t *testing.T) {
	cfg := DefaultDesensitizeConfig()
	c := NewCompliance(cfg)

	masked := c.MaskEmail("test@example.com")
	assert.Contains(t, masked, "*")
	assert.Contains(t, masked, "@example.com")
	assert.Equal(t, "te", masked[:2])
}

func TestComplianceAddCustomPattern(t *testing.T) {
	cfg := DefaultDesensitizeConfig()
	c := NewCompliance(cfg)

	err := c.AddCustomPattern("passport", `[A-Z]\d{8}`)
	assert.NoError(t, err)

	patterns := c.GetPatterns()
	assert.Contains(t, patterns, "passport")
}

func TestComplianceDesensitizeResult(t *testing.T) {
	cfg := DefaultDesensitizeConfig()
	c := NewCompliance(cfg)

	result := &OCRResult{
		ID: "test_desensitize",
		Pages: []*PageResult{
			{
				Text: "身份证号：110101199001011234",
				Blocks: []*TextBlock{
					{Text: "手机：13812345678"},
				},
			},
		},
		FullText: "邮箱：test@example.com",
		Structured: &StructuredData{
			Fields: map[string]interface{}{
				"id_number": "110101199001011234",
				"phone":     "13812345678",
			},
		},
	}

	c.DesensitizeResult(result, nil)

	assert.True(t, result.Desensitized)
	assert.Contains(t, result.FullText, "*")
	assert.Contains(t, result.Pages[0].Text, "*")
}
