// Package content 提供文档内容提取功能
// 支持从 PDF、Office、Markdown、纯文本等格式中提取文本内容
package content

import (
	"archive/zip"
	"bufio"
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Extractor 文档内容提取器接口
// 所有格式的提取器都实现此接口
type Extractor interface {
	// Extract 从文件中提取文本内容
	Extract(path string) (*ExtractedContent, error)
	// ExtractFromReader 从 io.Reader 提取文本内容
	ExtractFromReader(r io.Reader) (*ExtractedContent, error)
	// Supports 检查是否支持该文件扩展名
	Supports(ext string) bool
	// Name 提取器名称
	Name() string
}

// ExtractedContent 提取的文档内容
type ExtractedContent struct {
	Title    string       `json:"title,omitempty"`
	Content  string       `json:"content"`
	Metadata *DocMetadata `json:"metadata,omitempty"`
	Err      error        `json:"-"`
}

// DocMetadata 文档元数据
type DocMetadata struct {
	Author     string    `json:"author,omitempty"`
	CreatedAt  time.Time `json:"createdAt,omitempty"`
	ModifiedAt time.Time `json:"modifiedAt,omitempty"`
	PageCount  int       `json:"pageCount,omitempty"`
	WordCount  int       `json:"wordCount,omitempty"`
	Language   string    `json:"language,omitempty"`
	Subject    string    `json:"subject,omitempty"`
	Keywords   []string  `json:"keywords,omitempty"`
}

// ContentStats 内容统计
type ContentStats struct {
	TotalChars   int `json:"totalChars"`
	TotalWords   int `json:"totalWords"`
	TotalLines   int `json:"totalLines"`
	ChineseChars int `json:"chineseChars"`
	EnglishWords int `json:"englishWords"`
}

// CalcStats 计算内容统计信息
func (c *ExtractedContent) CalcStats() ContentStats {
	stats := ContentStats{
		TotalChars: len([]rune(c.Content)),
	}
	scanner := bufio.NewScanner(strings.NewReader(c.Content))
	for scanner.Scan() {
		stats.TotalLines++
		line := scanner.Text()
		words := strings.Fields(line)
		stats.TotalWords += len(words)

		for _, r := range line {
			if r >= 0x4E00 && r <= 0x9FFF {
				stats.ChineseChars++
			}
		}
	}
	stats.EnglishWords = stats.TotalWords - stats.ChineseChars
	return stats
}

// ================== 提取器注册表 ==================

// ExtractorRegistry 提取器注册表
// 管理所有已注册的文档内容提取器
type ExtractorRegistry struct {
	extractors map[string]Extractor
	logger     *zap.Logger
	mu         sync.RWMutex
}

// NewExtractorRegistry 创建提取器注册表
func NewExtractorRegistry(logger *zap.Logger) *ExtractorRegistry {
	r := &ExtractorRegistry{
		extractors: make(map[string]Extractor),
		logger:     logger,
	}

	// 注册内置提取器
	r.Register(NewPlainTextExtractor())
	r.Register(NewMarkdownExtractor())
	r.Register(NewOfficeExtractor())
	r.Register(NewPDFExtractor())

	return r
}

// Register 注册提取器
func (r *ExtractorRegistry) Register(extractor Extractor) {
	r.mu.Lock()
	defer r.mu.Unlock()

	name := extractor.Name()
	r.extractors[name] = extractor
	r.logger.Debug("注册文档提取器", zap.String("name", name))
}

// GetExtractor 获取指定文件扩展名对应的提取器
func (r *ExtractorRegistry) GetExtractor(ext string) (Extractor, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	ext = strings.ToLower(ext)
	for _, extractor := range r.extractors {
		if extractor.Supports(ext) {
			return extractor, true
		}
	}
	return nil, false
}

// Extract 提取文件内容
// 根据文件扩展名自动选择合适的提取器
func (r *ExtractorRegistry) Extract(path string) (*ExtractedContent, error) {
	ext := strings.ToLower(filepath.Ext(path))
	extractor, ok := r.GetExtractor(ext)
	if !ok {
		return nil, fmt.Errorf("不支持的文件格式: %s", ext)
	}

	r.logger.Debug("提取文档内容",
		zap.String("path", path),
		zap.String("extractor", extractor.Name()))

	return extractor.Extract(path)
}

// ListExtractors 列出所有已注册的提取器
func (r *ExtractorRegistry) ListExtractors() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	names := make([]string, 0, len(r.extractors))
	for name := range r.extractors {
		names = append(names, name)
	}
	return names
}

// Supports 检查是否支持该文件扩展名
func (r *ExtractorRegistry) Supports(ext string) bool {
	_, ok := r.GetExtractor(ext)
	return ok
}

// ================== 纯文本提取器 ==================

// PlainTextExtractor 纯文本文件内容提取器
// 支持 .txt .log .csv .tsv .conf .cfg .ini .env .sh 等
type PlainTextExtractor struct{}

// NewPlainTextExtractor 创建纯文本提取器
func NewPlainTextExtractor() *PlainTextExtractor {
	return &PlainTextExtractor{}
}

// Name 提取器名称
func (e *PlainTextExtractor) Name() string {
	return "plaintext"
}

// Supports 是否支持该扩展名
func (e *PlainTextExtractor) Supports(ext string) bool {
	supported := map[string]bool{
		".txt":        true,
		".log":        true,
		".csv":        true,
		".tsv":        true,
		".conf":       true,
		".cfg":        true,
		".ini":        true,
		".env":        true,
		".sh":         true,
		".bash":       true,
		".zsh":        true,
		".fish":       true,
		".bat":        true,
		".cmd":        true,
		".ps1":        true,
		".py":         true,
		".rb":         true,
		".pl":         true,
		".r":          true,
		".go":         true,
		".rs":         true,
		".java":       true,
		".kt":         true,
		".scala":      true,
		".c":          true,
		".cpp":        true,
		".h":          true,
		".hpp":        true,
		".cs":         true,
		".swift":      true,
		".js":         true,
		".ts":         true,
		".jsx":        true,
		".tsx":        true,
		".vue":        true,
		".svelte":     true,
		".css":        true,
		".scss":       true,
		".less":       true,
		".html":       true,
		".htm":        true,
		".xml":        true,
		".json":       true,
		".yaml":       true,
		".yml":        true,
		".toml":       true,
		".sql":        true,
		".graphql":    true,
		".proto":      true,
		".mdx":        true,
		".rst":        true,
		".tex":        true,
		".bib":        true,
		".diff":       true,
		".patch":      true,
		".makefile":   true,
		".dockerfile": true,
	}
	return supported[ext]
}

// Extract 从文件提取文本
func (e *PlainTextExtractor) Extract(path string) (*ExtractedContent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	return e.ExtractFromReader(file)
}

// ExtractFromReader 从 reader 提取文本
func (e *PlainTextExtractor) ExtractFromReader(r io.Reader) (*ExtractedContent, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("读取文件内容失败: %w", err)
	}

	// 检测是否为二进制文件
	if isBinaryContent(data) {
		return &ExtractedContent{
			Content: "",
		}, nil
	}

	content := string(data)
	// 清理内容
	content = cleanTextContent(content)

	return &ExtractedContent{
		Content: content,
	}, nil
}

// ================== Markdown 提取器 ==================

// MarkdownExtractor Markdown 文档内容提取器
// 从 Markdown 文件中提取纯文本，去除格式标记
type MarkdownExtractor struct{}

// NewMarkdownExtractor 创建 Markdown 提取器
func NewMarkdownExtractor() *MarkdownExtractor {
	return &MarkdownExtractor{}
}

// Name 提取器名称
func (e *MarkdownExtractor) Name() string {
	return "markdown"
}

// Supports 是否支持该扩展名
func (e *MarkdownExtractor) Supports(ext string) bool {
	return ext == ".md" || ext == ".markdown" || ext == ".mdx"
}

// Extract 从文件提取 Markdown 文本
func (e *MarkdownExtractor) Extract(path string) (*ExtractedContent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	return e.ExtractFromReader(file)
}

// ExtractFromReader 从 reader 提取 Markdown 文本
func (e *MarkdownExtractor) ExtractFromReader(r io.Reader) (*ExtractedContent, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	content := string(data)
	title := extractMarkdownTitle(content)
	content = stripMarkdownFormatting(content)

	return &ExtractedContent{
		Title:   title,
		Content: content,
	}, nil
}

// extractMarkdownTitle 提取 Markdown 标题
func extractMarkdownTitle(content string) string {
	scanner := bufio.NewScanner(strings.NewReader(content))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return ""
}

// stripMarkdownFormatting 去除 Markdown 格式标记
func stripMarkdownFormatting(content string) string {
	var result strings.Builder
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		trimmed := strings.TrimSpace(line)

		// 跳过 YAML front matter
		if trimmed == "---" {
			continue
		}

		// 去除标题标记但保留文本
		if strings.HasPrefix(trimmed, "#") {
			trimmed = strings.TrimLeft(trimmed, "# ")
		}

		// 去除列表标记
		if strings.HasPrefix(trimmed, "- ") || strings.HasPrefix(trimmed, "* ") || strings.HasPrefix(trimmed, "+ ") {
			trimmed = trimmed[2:]
		}

		// 去除代码块标记
		if strings.HasPrefix(trimmed, "```") {
			continue
		}

		// 去除行内格式
		trimmed = removeInlineMarkdown(trimmed)

		if trimmed != "" {
			result.WriteString(trimmed)
			result.WriteString("\n")
		}
	}

	return result.String()
}

// removeInlineMarkdown 去除行内 Markdown 格式
func removeInlineMarkdown(line string) string {
	// 去除粗体
	line = strings.ReplaceAll(line, "**", "")
	line = strings.ReplaceAll(line, "__", "")

	// 去除斜体
	line = strings.ReplaceAll(line, "*", "")
	line = strings.ReplaceAll(line, "_", "")

	// 去除行内代码
	line = strings.ReplaceAll(line, "`", "")

	// 去除链接，保留文本
	for {
		start := strings.Index(line, "[")
		end := strings.Index(line, "]")
		if start == -1 || end == -1 || end <= start {
			break
		}

		linkText := line[start+1 : end]
		// 检查后面是否有 (url)
		if end+1 < len(line) && line[end+1] == '(' {
			urlEnd := strings.Index(line[end:], ")")
			if urlEnd != -1 {
				line = line[:start] + linkText + line[end+urlEnd+2:]
				continue
			}
		}
		line = line[:start] + linkText + line[end+1:]
	}

	// 去除图片标记
	for {
		start := strings.Index(line, "![")
		if start == -1 {
			break
		}
		end := strings.Index(line[start:], ")")
		if end == -1 {
			break
		}
		line = line[:start] + line[start+end+1:]
	}

	return strings.TrimSpace(line)
}

// ================== Office 文档提取器 ==================

// OfficeExtractor Office 文档内容提取器
// 支持 .docx, .xlsx, .pptx 格式
type OfficeExtractor struct{}

// NewOfficeExtractor 创建 Office 提取器
func NewOfficeExtractor() *OfficeExtractor {
	return &OfficeExtractor{}
}

// Name 提取器名称
func (e *OfficeExtractor) Name() string {
	return "office"
}

// Supports 是否支持该扩展名
func (e *OfficeExtractor) Supports(ext string) bool {
	supported := map[string]bool{
		".docx": true,
		".xlsx": true,
		".pptx": true,
	}
	return supported[ext]
}

// Extract 从文件提取 Office 文档内容
func (e *OfficeExtractor) Extract(path string) (*ExtractedContent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开文件失败: %w", err)
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".docx":
		return e.extractDocx(file)
	case ".xlsx":
		return e.extractXlsx(file)
	case ".pptx":
		return e.extractPptx(file)
	default:
		return nil, fmt.Errorf("不支持的 Office 格式: %s", ext)
	}
}

// ExtractFromReader 从 reader 提取 Office 文档内容
func (e *OfficeExtractor) ExtractFromReader(r io.Reader) (*ExtractedContent, error) {
	// 需要先写入临时文件再处理（Office 文档是 zip 格式）
	tmpFile, err := os.CreateTemp("", "office-extract-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时文件失败: %w", err)
	}
	defer os.Remove(tmpFile.Name())
	defer tmpFile.Close()

	if _, err := io.Copy(tmpFile, r); err != nil {
		return nil, fmt.Errorf("写入临时文件失败: %w", err)
	}
	tmpFile.Close()

	return e.Extract(tmpFile.Name())
}

// extractDocx 提取 docx 文档内容
// docx 文件是 ZIP 格式，包含 word/document.xml
func (e *OfficeExtractor) extractDocx(file *os.File) (*ExtractedContent, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("读取 docx 文件失败: %w", err)
	}

	// 解压 ZIP 并读取 word/document.xml
	text, err := extractFromZip(data, "word/document.xml")
	if err != nil {
		return nil, fmt.Errorf("解析 docx 文档失败: %w", err)
	}

	// 解析 XML 提取文本
	content := extractTextFromXML(text)

	return &ExtractedContent{
		Content: content,
	}, nil
}

// extractXlsx 提取 xlsx 文档内容
func (e *OfficeExtractor) extractXlsx(file *os.File) (*ExtractedContent, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("读取 xlsx 文件失败: %w", err)
	}

	// 提取共享字符串
	sharedStringsXML, err := extractFromZip(data, "xl/sharedStrings.xml")
	if err != nil {
		// 某些 xlsx 可能没有共享字符串
		return &ExtractedContent{Content: ""}, nil
	}

	// 解析共享字符串
	sharedStrs := parseSharedStrings(sharedStringsXML)

	// 提取所有工作表内容
	var contents []string
	for i := 1; i <= 100; i++ { // 最多尝试100个工作表
		sheetPath := fmt.Sprintf("xl/worksheets/sheet%d.xml", i)
		sheetXML, err := extractFromZip(data, sheetPath)
		if err != nil {
			break
		}
		sheetText := extractSheetText(sheetXML, sharedStrs)
		if sheetText != "" {
			contents = append(contents, sheetText)
		}
	}

	return &ExtractedContent{
		Content: strings.Join(contents, "\n"),
	}, nil
}

// extractPptx 提取 pptx 文档内容
func (e *OfficeExtractor) extractPptx(file *os.File) (*ExtractedContent, error) {
	data, err := io.ReadAll(file)
	if err != nil {
		return nil, fmt.Errorf("读取 pptx 文件失败: %w", err)
	}

	var contents []string

	// 提取所有幻灯片内容
	for i := 1; i <= 200; i++ { // 最多200页幻灯片
		slidePath := fmt.Sprintf("ppt/slides/slide%d.xml", i)
		slideXML, err := extractFromZip(data, slidePath)
		if err != nil {
			break
		}
		slideText := extractTextFromXML(slideXML)
		if slideText != "" {
			contents = append(contents, slideText)
		}
	}

	// 提取备注
	for i := 1; i <= 200; i++ {
		notesPath := fmt.Sprintf("ppt/notesSlides/notesSlide%d.xml", i)
		notesXML, err := extractFromZip(data, notesPath)
		if err != nil {
			break
		}
		notesText := extractTextFromXML(notesXML)
		if notesText != "" {
			contents = append(contents, notesText)
		}
	}

	return &ExtractedContent{
		Content: strings.Join(contents, "\n"),
	}, nil
}

// extractFromZip 从 ZIP 数据中提取指定文件内容
func extractFromZip(zipData []byte, entryPath string) (string, error) {
	// 使用 Go 标准库的 archive/zip
	r, err := newZipReaderFromBytes(zipData)
	if err != nil {
		return "", err
	}

	for _, f := range r.File {
		if f.Name == entryPath {
			rc, err := f.Open()
			if err != nil {
				return "", err
			}
			defer rc.Close()

			var buf bytes.Buffer
			if _, err := io.Copy(&buf, rc); err != nil {
				return "", err
			}
			return buf.String(), nil
		}
	}

	return "", fmt.Errorf("未找到条目: %s", entryPath)
}

// extractTextFromXML 从 XML 中提取所有文本内容
func extractTextFromXML(xmlContent string) string {
	var texts []string
	decoder := xml.NewDecoder(strings.NewReader(xmlContent))

	// 用于处理 <w:t> 标签（docx/pptx 的文本节点）
	inTextElement := false

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			// 检查是否为文本元素（支持 docx/pptx 的命名空间）
			local := t.Name.Local
			if local == "t" || local == "v" {
				inTextElement = true
			}
			// 检查 xlsx 的 v 元素
			if local == "v" {
				inTextElement = true
			}
		case xml.EndElement:
			local := t.Name.Local
			if local == "t" || local == "v" {
				inTextElement = false
			}
			// 段落结束时添加换行
			if local == "p" || local == "row" {
				texts = append(texts, "\n")
			}
		case xml.CharData:
			if inTextElement {
				text := strings.TrimSpace(string(t))
				if text != "" {
					texts = append(texts, text)
				}
			}
		}
	}

	return strings.Join(texts, " ")
}

// parseSharedStrings 解析 xlsx 共享字符串表
func parseSharedStrings(xmlContent string) []string {
	var strs []string
	decoder := xml.NewDecoder(strings.NewReader(xmlContent))
	inT := false

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "t" {
				inT = true
			}
			// 检测 <si> 开始（shared string item）
			if t.Name.Local == "si" {
				strs = append(strs, "")
			}
		case xml.EndElement:
			if t.Name.Local == "t" {
				inT = false
			}
		case xml.CharData:
			if inT && len(strs) > 0 {
				strs[len(strs)-1] += string(t)
			}
		}
	}

	return strs
}

// extractSheetText 提取工作表文本
func extractSheetText(xmlContent string, sharedStrings []string) string {
	var texts []string
	decoder := xml.NewDecoder(strings.NewReader(xmlContent))
	inV := false
	currentValue := ""

	for {
		token, err := decoder.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			break
		}

		switch t := token.(type) {
		case xml.StartElement:
			if t.Name.Local == "v" || t.Name.Local == "t" {
				inV = true
				currentValue = ""
			}
			// 检查是否引用共享字符串
			if t.Name.Local == "c" {
				for _, attr := range t.Attr {
					if attr.Name.Local == "t" && attr.Value == "s" {
						// 共享字符串引用
					}
				}
			}
		case xml.EndElement:
			if t.Name.Local == "v" || t.Name.Local == "t" {
				inV = false
				texts = append(texts, currentValue)
			}
			if t.Name.Local == "row" {
				texts = append(texts, "\n")
			}
		case xml.CharData:
			if inV {
				currentValue += string(t)
			}
		}
	}

	return strings.Join(texts, " ")
}

// ================== PDF 提取器 ==================

// PDFExtractor PDF 文档内容提取器
// 使用纯 Go 方案解析 PDF 文件结构，提取文本内容
type PDFExtractor struct{}

// NewPDFExtractor 创建 PDF 提取器
func NewPDFExtractor() *PDFExtractor {
	return &PDFExtractor{}
}

// Name 提取器名称
func (e *PDFExtractor) Name() string {
	return "pdf"
}

// Supports 是否支持该扩展名
func (e *PDFExtractor) Supports(ext string) bool {
	return ext == ".pdf"
}

// Extract 从文件提取 PDF 文本
func (e *PDFExtractor) Extract(path string) (*ExtractedContent, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("打开 PDF 文件失败: %w", err)
	}
	defer file.Close()

	return e.ExtractFromReader(file)
}

// ExtractFromReader 从 reader 提取 PDF 文本
func (e *PDFExtractor) ExtractFromReader(r io.Reader) (*ExtractedContent, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("读取 PDF 文件失败: %w", err)
	}

	// PDF 基本验证
	if !isPDFFile(data) {
		return nil, fmt.Errorf("不是有效的 PDF 文件")
	}

	// 提取文本流
	content := extractPDFText(data)
	content = cleanTextContent(content)

	return &ExtractedContent{
		Content: content,
	}, nil
}

// isPDFFile 检查是否为 PDF 文件
func isPDFFile(data []byte) bool {
	return len(data) >= 5 && string(data[:5]) == "%PDF-"
}

// extractPDFText 从 PDF 数据中提取文本
// 使用简化的 PDF 解析：查找文本流并提取可读文本
func extractPDFText(data []byte) string {
	var texts []string

	// 查找文本流 (BT ... ET 块)
	btMarker := []byte("BT")
	etMarker := []byte("ET")

	pos := 0
	for pos < len(data) {
		// 查找 BT 标记
		btIdx := bytes.Index(data[pos:], btMarker)
		if btIdx == -1 {
			break
		}
		btIdx += pos

		// 查找对应的 ET 标记
		etIdx := bytes.Index(data[btIdx:], etMarker)
		if etIdx == -1 {
			break
		}
		etIdx += btIdx

		// 提取文本块
		block := data[btIdx:etIdx]
		blockText := extractTextFromStreamBlock(block)
		if blockText != "" {
			texts = append(texts, blockText)
		}

		pos = etIdx + len(etMarker)
	}

	// 如果没有找到 BT/ET 块，尝试直接提取可读文本
	if len(texts) == 0 {
		texts = extractReadableText(data)
	}

	return strings.Join(texts, " ")
}

// extractTextFromStreamBlock 从文本流块中提取文本
func extractTextFromStreamBlock(block []byte) string {
	var texts []string

	// 提取 (text) Tj 格式的文本
	for {
		start := bytes.Index(block, []byte("("))
		if start == -1 {
			break
		}

		end := findMatchingParen(block, start)
		if end == -1 {
			break
		}

		text := string(block[start+1 : end])
		if isReadableText(text) {
			texts = append(texts, text)
		}

		block = block[end+1:]
	}

	return strings.Join(texts, " ")
}

// findMatchingParen 查找匹配的右括号
func findMatchingParen(data []byte, start int) int {
	depth := 0
	for i := start; i < len(data); i++ {
		switch data[i] {
		case '(':
			depth++
		case ')':
			depth--
			if depth == 0 {
				return i
			}
		case '\\':
			i++ // 跳过转义字符
		}
	}
	return -1
}

// extractReadableText 提取可读文本（用于无结构 PDF）
func extractReadableText(data []byte) []string {
	var texts []string
	var current strings.Builder

	for _, b := range data {
		if b >= 32 && b < 127 {
			current.WriteByte(b)
		} else if b == '\n' || b == '\r' {
			if current.Len() > 0 {
				text := current.String()
				if isReadableText(text) && len(text) > 2 {
					texts = append(texts, text)
				}
				current.Reset()
			}
		}
	}

	if current.Len() > 0 {
		text := current.String()
		if isReadableText(text) && len(text) > 2 {
			texts = append(texts, text)
		}
	}

	return texts
}

// ================== 辅助函数 ==================

// isBinaryContent 检查内容是否为二进制
func isBinaryContent(data []byte) bool {
	// 检查前 8KB
	checkLen := len(data)
	if checkLen > 8192 {
		checkLen = 8192
	}

	nullCount := 0
	for i := 0; i < checkLen; i++ {
		if data[i] == 0 {
			nullCount++
		}
	}

	// 如果超过 1% 的空字节，认为是二进制
	return float64(nullCount)/float64(checkLen) > 0.01
}

// isReadableText 检查文本是否可读
func isReadableText(text string) bool {
	if len(text) < 1 {
		return false
	}

	printable := 0
	for _, r := range text {
		if r >= 32 && r < 127 || r >= 0x4E00 && r <= 0x9FFF {
			printable++
		}
	}

	return float64(printable)/float64(len([]rune(text))) > 0.5
}

// cleanTextContent 清理文本内容
func cleanTextContent(content string) string {
	// 替换多个空白字符为单个空格
	content = strings.Join(strings.Fields(content), " ")

	// 去除控制字符
	var cleaned strings.Builder
	for _, r := range content {
		if r >= 32 || r == '\n' || r == '\t' {
			cleaned.WriteRune(r)
		}
	}

	return strings.TrimSpace(cleaned.String())
}

// newZipReaderFromBytes 从字节数据创建 ZIP reader
func newZipReaderFromBytes(data []byte) (*zip.Reader, error) {
	r := bytes.NewReader(data)
	return zip.NewReader(r, int64(len(data)))
}
