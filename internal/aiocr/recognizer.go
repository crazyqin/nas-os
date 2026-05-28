// recognizer.go - 文字识别器（多语言支持）
package aiocr

import (
	"fmt"
	"log"
	"strings"
)

// Recognizer 文字识别器.
type Recognizer struct {
	config    *Config
	languages map[string]*Language
}

// NewRecognizer 创建识别器.
func NewRecognizer(cfg *Config) *Recognizer {
	r := &Recognizer{
		config:    cfg,
		languages: make(map[string]*Language),
	}

	// 初始化支持的语言
	r.initLanguages()

	return r
}

// initLanguages 初始化支持的语言.
func (r *Recognizer) initLanguages() {
	defaultLangs := []*Language{
		{Code: "chi_sim", Name: "简体中文", Enabled: true, Installed: true},
		{Code: "chi_tra", Name: "繁体中文", Enabled: true, Installed: true},
		{Code: "eng", Name: "英文", Enabled: true, Installed: true},
		{Code: "jpn", Name: "日文", Enabled: true, Installed: true},
		{Code: "kor", Name: "韩文", Enabled: true, Installed: true},
		{Code: "fra", Name: "法文", Enabled: false, Installed: false},
		{Code: "deu", Name: "德文", Enabled: false, Installed: false},
		{Code: "spa", Name: "西班牙文", Enabled: false, Installed: false},
		{Code: "rus", Name: "俄文", Enabled: false, Installed: false},
		{Code: "ara", Name: "阿拉伯文", Enabled: false, Installed: false},
	}

	for _, lang := range defaultLangs {
		r.languages[lang.Code] = lang
	}
}

// Recognize 文字识别.
func (r *Recognizer) Recognize(img *ProcessedImage, language string, options *OCROptions) ([]*PageResult, error) {
	if img == nil {
		return nil, fmt.Errorf("图像不能为空")
	}

	log.Printf("📝 开始文字识别，语言: %s", language)

	// 解析语言列表
	langs := r.parseLanguages(language)

	// 创建页面结果
	page := &PageResult{
		PageNumber: 1,
		Width:      img.Width,
		Height:     img.Height,
		Blocks:     make([]*TextBlock, 0),
		Tables:     make([]*Table, 0),
	}

	// 识别文本块
	blocks, err := r.recognizeBlocks(img, langs)
	if err != nil {
		return nil, fmt.Errorf("识别文本块失败: %w", err)
	}
	page.Blocks = blocks

	// 提取表格
	if options != nil && options.ExtractTables {
		tables := r.extractTables(img, blocks)
		page.Tables = tables
	}

	// 组装全文
	page.Text = r.assembleText(blocks)

	// 计算置信度
	page.Confidence = r.calculateConfidence(blocks)

	log.Printf("✅ 文字识别完成，文本块: %d, 置信度: %.2f",
		len(blocks), page.Confidence)

	return []*PageResult{page}, nil
}

// parseLanguages 解析语言列表.
func (r *Recognizer) parseLanguages(language string) []string {
	if language == "" {
		language = r.config.DefaultLanguage
	}

	// 支持 "chi_sim+eng" 格式
	langs := strings.Split(language, "+")
	result := make([]string, 0, len(langs))

	for _, lang := range langs {
		lang = strings.TrimSpace(lang)
		if lang != "" {
			if l, exists := r.languages[lang]; exists && l.Enabled {
				result = append(result, lang)
			}
		}
	}

	if len(result) == 0 {
		result = []string{"eng"}
	}

	return result
}

// recognizeBlocks 识别文本块.
func (r *Recognizer) recognizeBlocks(img *ProcessedImage, languages []string) ([]*TextBlock, error) {
	// 这里简化实现，实际应该调用 OCR 引擎
	// 返回模拟的文本块
	blocks := []*TextBlock{
		{
			ID:         "block_1",
			Text:       "示例文档标题",
			X:          100,
			Y:          50,
			Width:      300,
			Height:     40,
			Confidence: 0.95,
			Type:       "text",
			FontSize:   24,
			IsBold:     true,
		},
		{
			ID:         "block_2",
			Text:       "这是文档正文内容，用于测试 OCR 识别功能。",
			X:          100,
			Y:          100,
			Width:      500,
			Height:     60,
			Confidence: 0.92,
			Type:       "text",
			FontSize:   14,
		},
	}

	return blocks, nil
}

// extractTables 提取表格.
func (r *Recognizer) extractTables(img *ProcessedImage, blocks []*TextBlock) []*Table {
	// 简化实现：检测可能的表格区域
	tables := make([]*Table, 0)

	// 按位置排序文本块
	// 检测表格边界
	// 提取单元格内容

	return tables
}

// assembleText 组装文本.
func (r *Recognizer) assembleText(blocks []*TextBlock) string {
	if len(blocks) == 0 {
		return ""
	}

	var result strings.Builder
	for i, block := range blocks {
		result.WriteString(block.Text)
		if i < len(blocks)-1 {
			// 根据块类型决定分隔符
			if block.Type == "text" {
				result.WriteString("\n")
			}
		}
	}

	return result.String()
}

// calculateConfidence 计算置信度.
func (r *Recognizer) calculateConfidence(blocks []*TextBlock) float64 {
	if len(blocks) == 0 {
		return 0
	}

	total := 0.0
	for _, block := range blocks {
		total += block.Confidence
	}

	return total / float64(len(blocks))
}

// GetSupportedLanguages 获取支持的语言列表.
func (r *Recognizer) GetSupportedLanguages() []*Language {
	result := make([]*Language, 0, len(r.languages))
	for _, lang := range r.languages {
		result = append(result, lang)
	}
	return result
}

// InstallLanguage 安装语言包.
func (r *Recognizer) InstallLanguage(code string) error {
	lang, exists := r.languages[code]
	if !exists {
		return fmt.Errorf("不支持的语言: %s", code)
	}

	// 模拟安装过程
	lang.Installed = true
	log.Printf("✅ 语言包已安装: %s (%s)", lang.Name, code)
	return nil
}

// EnableLanguage 启用语言.
func (r *Recognizer) EnableLanguage(code string) error {
	lang, exists := r.languages[code]
	if !exists {
		return fmt.Errorf("不支持的语言: %s", code)
	}

	if !lang.Installed {
		return fmt.Errorf("语言包未安装: %s", code)
	}

	lang.Enabled = true
	log.Printf("✅ 语言已启用: %s (%s)", lang.Name, code)
	return nil
}

// DisableLanguage 禁用语言.
func (r *Recognizer) DisableLanguage(code string) error {
	lang, exists := r.languages[code]
	if !exists {
		return fmt.Errorf("不支持的语言: %s", code)
	}

	lang.Enabled = false
	log.Printf("✅ 语言已禁用: %s (%s)", lang.Name, code)
	return nil
}
