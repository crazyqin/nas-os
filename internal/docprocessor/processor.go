// Package docprocessor 提供智能文档处理功能
package docprocessor

import (
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

// DocType 文档类型枚举
type DocType int

const (
	DocTypeUnknown DocType = iota
	DocTypePDF
	DocTypeOffice
	DocTypeMarkdown
	DocTypeText
	DocTypeJSON
	DocTypeYAML
	DocTypeHTML
	DocTypeCSV
)

// DocTypeNames 文档类型名称映射
var DocTypeNames = map[DocType]string{
	DocTypeUnknown:  "unknown",
	DocTypePDF:      "pdf",
	DocTypeOffice:   "office",
	DocTypeMarkdown: "markdown",
	DocTypeText:     "text",
	DocTypeJSON:     "json",
	DocTypeYAML:     "yaml",
	DocTypeHTML:     "html",
	DocTypeCSV:      "csv",
}

// String 返回文档类型字符串
func (d DocType) String() string {
	if name, ok := DocTypeNames[d]; ok {
		return name
	}
	return "unknown"
}

// Document 表示一个文档
type Document struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Path      string            `json:"path"`
	Content   string            `json:"content"`
	Type      DocType           `json:"type"`
	Size      int64             `json:"size"`
	Metadata  map[string]string `json:"metadata"`
	Tags      []string          `json:"tags"`
	Summary   string            `json:"summary"`
	CreatedAt time.Time         `json:"created_at"`
	UpdatedAt time.Time         `json:"updated_at"`
	Hash      string            `json:"hash"`
}

// AnalysisResult 文档分析结果
type AnalysisResult struct {
	DocumentID   string            `json:"document_id"`
	Type         DocType           `json:"type"`
	Size         int64             `json:"size"`
	WordCount    int               `json:"word_count"`
	LineCount    int               `json:"line_count"`
	CharCount    int               `json:"char_count"`
	Language     string            `json:"language"`
	Encoding     string            `json:"encoding"`
	Keywords     []string          `json:"keywords"`
	Metadata     map[string]string `json:"metadata"`
	Hash         string            `json:"hash"`
	AnalyzedAt   time.Time         `json:"analyzed_at"`
}

// ClassifyResult 文档分类结果
type ClassifyResult struct {
	DocumentID   string   `json:"document_id"`
	Category     string   `json:"category"`
	SubCategory  string   `json:"sub_category"`
	Confidence   float64  `json:"confidence"`
	Tags         []string `json:"tags"`
	Labels       []string `json:"labels"`
}

// SummaryResult 文档摘要结果
type SummaryResult struct {
	DocumentID   string   `json:"document_id"`
	Summary      string   `json:"summary"`
	Keywords     []string `json:"keywords"`
	WordCount    int      `json:"word_count"`
	CompressionRatio float64 `json:"compression_ratio"`
}

// DiffResult 文档对比结果
type DiffResult struct {
	Doc1ID       string   `json:"doc1_id"`
	Doc2ID       string   `json:"doc2_id"`
	Additions    int      `json:"additions"`
	Deletions    int      `json:"deletions"`
	Changes      int      `json:"changes"`
	Similarity   float64  `json:"similarity"`
	DiffLines    []DiffLine `json:"diff_lines"`
}

// DiffLine 对比行
type DiffLine struct {
	Type    string `json:"type"` // "add", "delete", "same"
	LineNum int    `json:"line_num"`
	Content string `json:"content"`
}

// SearchResult 搜索结果
type SearchResult struct {
	DocumentID string   `json:"document_id"`
	Score      float64  `json:"score"`
	Snippets   []string `json:"snippets"`
	Highlights []string `json:"highlights"`
}

// Processor 文档处理器
type Processor struct {
	documents map[string]*Document
	index     map[string][]string // 倒排索引: word -> docIDs
}

// NewProcessor 创建新的处理器实例
func NewProcessor() *Processor {
	return &Processor{
		documents: make(map[string]*Document),
		index:     make(map[string][]string),
	}
}

// DetectType 检测文档类型
func (p *Processor) DetectType(filename string, content []byte) DocType {
	ext := strings.ToLower(filepath.Ext(filename))

	// 基于扩展名判断
	extMap := map[string]DocType{
		".pdf":  DocTypePDF,
		".doc":  DocTypeOffice,
		".docx": DocTypeOffice,
		".xls":  DocTypeOffice,
		".xlsx": DocTypeOffice,
		".ppt":  DocTypeOffice,
		".pptx": DocTypeOffice,
		".md":   DocTypeMarkdown,
		".txt":  DocTypeText,
		".json": DocTypeJSON,
		".yaml": DocTypeYAML,
		".yml":  DocTypeYAML,
		".html": DocTypeHTML,
		".htm":  DocTypeHTML,
		".csv":  DocTypeCSV,
	}

	if dtype, ok := extMap[ext]; ok {
		return dtype
	}

	// 基于内容判断
	return detectByContent(content)
}

// detectByContent 基于内容检测类型
func detectByContent(content []byte) DocType {
	if len(content) == 0 {
		return DocTypeText
	}

	// 检测PDF
	if len(content) >= 4 && string(content[:4]) == "%PDF" {
		return DocTypePDF
	}

	// 检测JSON
	contentStr := strings.TrimSpace(string(content))
	if (strings.HasPrefix(contentStr, "{") && strings.HasSuffix(contentStr, "}")) ||
		(strings.HasPrefix(contentStr, "[") && strings.HasSuffix(contentStr, "]")) {
		return DocTypeJSON
	}

	// 检测HTML
	if strings.Contains(contentStr, "<!DOCTYPE") || strings.Contains(contentStr, "<html") {
		return DocTypeHTML
	}

	// 检测Markdown
	mdPatterns := []string{"# ", "## ", "* ", "- ", "```", "__", "**"}
	for _, pattern := range mdPatterns {
		if strings.Contains(contentStr, pattern) {
			return DocTypeMarkdown
		}
	}

	return DocTypeText
}

// AnalyzeDocument 分析文档
func (p *Processor) AnalyzeDocument(doc *Document) *AnalysisResult {
	result := &AnalysisResult{
		DocumentID: doc.ID,
		Type:       doc.Type,
		Size:       doc.Size,
		AnalyzedAt: time.Now(),
	}

	// 计算统计信息
	result.WordCount = countWords(doc.Content)
	result.LineCount = countLines(doc.Content)
	result.CharCount = utf8.RuneCountInString(doc.Content)

	// 检测语言
	result.Language = detectLanguage(doc.Content)

	// 提取关键词
	result.Keywords = extractKeywords(doc.Content)

	// 计算哈希
	result.Hash = computeHash(doc.Content)

	// 提取元数据
	result.Metadata = extractMetadata(doc)

	return result
}

// ClassifyDocument 分类文档
func (p *Processor) ClassifyDocument(doc *Document) *ClassifyResult {
	result := &ClassifyResult{
		DocumentID: doc.ID,
	}

	// 基于类型分类
	categoryMap := map[DocType]string{
		DocTypePDF:      "文档",
		DocTypeOffice:   "办公文档",
		DocTypeMarkdown: "技术文档",
		DocTypeText:     "文本",
		DocTypeJSON:     "数据",
		DocTypeYAML:     "配置",
		DocTypeHTML:     "网页",
		DocTypeCSV:      "数据",
	}

	if cat, ok := categoryMap[doc.Type]; ok {
		result.Category = cat
	} else {
		result.Category = "其他"
	}

	// 基于内容细分
	content := strings.ToLower(doc.Content)

	// 技术文档检测
	techKeywords := []string{"api", "sdk", "代码", "编程", "开发", "测试", "部署", "docker", "kubernetes", "git"}
	techCount := countKeywordMatches(content, techKeywords)

	// 法律文档检测
	legalKeywords := []string{"合同", "协议", "法律", "条款", "赔偿", "责任", "义务"}
	legalCount := countKeywordMatches(content, legalKeywords)

	// 财务文档检测
	financeKeywords := []string{"财务", "报表", "预算", "收入", "支出", "利润", "发票"}
	financeCount := countKeywordMatches(content, financeKeywords)

	// 确定子类别
	if techCount > 3 {
		result.SubCategory = "技术文档"
	} else if legalCount > 3 {
		result.SubCategory = "法律文档"
	} else if financeCount > 3 {
		result.SubCategory = "财务文档"
	} else {
		result.SubCategory = "通用文档"
	}

	// 置信度计算
	totalKeywords := float64(techCount + legalCount + financeCount)
	if totalKeywords > 0 {
		maxCount := float64(maxInt(techCount, maxInt(legalCount, financeCount)))
		result.Confidence = maxCount / totalKeywords
	} else {
		result.Confidence = 0.5
	}

	// 生成标签
	result.Tags = generateTags(doc)
	result.Labels = generateLabels(doc)

	return result
}

// SummarizeDocument 生成文档摘要
func (p *Processor) SummarizeDocument(doc *Document, maxLength int) *SummaryResult {
	result := &SummaryResult{
		DocumentID: doc.ID,
		Keywords:   extractKeywords(doc.Content),
	}

	// 提取摘要
	result.Summary = extractSummary(doc.Content, maxLength)
	result.WordCount = countWords(result.Summary)

	// 计算压缩比
	origWordCount := countWords(doc.Content)
	if origWordCount > 0 {
		result.CompressionRatio = float64(result.WordCount) / float64(origWordCount)
	}

	return result
}

// DiffDocuments 对比两个文档
func (p *Processor) DiffDocuments(doc1, doc2 *Document) *DiffResult {
	result := &DiffResult{
		Doc1ID: doc1.ID,
		Doc2ID: doc2.ID,
	}

	lines1 := strings.Split(doc1.Content, "\n")
	lines2 := strings.Split(doc2.Content, "\n")

	// 简单的行级对比
	maxLines := len(lines1)
	if len(lines2) > maxLines {
		maxLines = len(lines2)
	}

	var diffLines []DiffLine
	for i := 0; i < maxLines; i++ {
		line1 := ""
		line2 := ""

		if i < len(lines1) {
			line1 = lines1[i]
		}
		if i < len(lines2) {
			line2 = lines2[i]
		}

		if line1 == line2 {
			diffLines = append(diffLines, DiffLine{
				Type:    "same",
				LineNum: i + 1,
				Content: line1,
			})
		} else {
			if line1 != "" {
				result.Deletions++
				diffLines = append(diffLines, DiffLine{
					Type:    "delete",
					LineNum: i + 1,
					Content: line1,
				})
			}
			if line2 != "" {
				result.Additions++
				diffLines = append(diffLines, DiffLine{
					Type:    "add",
					LineNum: i + 1,
					Content: line2,
				})
			}
		}
	}

	result.Changes = result.Additions + result.Deletions
	result.DiffLines = diffLines

	// 计算相似度
	if len(lines1) > 0 || len(lines2) > 0 {
		sameCount := 0
		for _, dl := range diffLines {
			if dl.Type == "same" {
				sameCount++
			}
		}
		result.Similarity = float64(sameCount) / float64(maxLines)
	}

	return result
}

// SearchDocuments 搜索文档
func (p *Processor) SearchDocuments(query string, maxResults int) []SearchResult {
	var results []SearchResult

	query = strings.ToLower(strings.TrimSpace(query))
	queryWords := strings.Fields(query)

	for _, doc := range p.documents {
		score := 0.0
		content := strings.ToLower(doc.Content)

		// 计算匹配分数
		for _, word := range queryWords {
			count := strings.Count(content, word)
			score += float64(count)
		}

		if score > 0 {
			// 提取匹配片段
			snippets := extractSnippets(doc.Content, queryWords, 3)

			result := SearchResult{
				DocumentID: doc.ID,
				Score:      score,
				Snippets:   snippets,
				Highlights: highlightMatches(snippets, queryWords),
			}

			results = append(results, result)
		}
	}

	// 按分数排序
	sort.Slice(results, func(i, j int) bool {
		return results[i].Score > results[j].Score
	})

	// 限制结果数
	if maxResults > 0 && len(results) > maxResults {
		results = results[:maxResults]
	}

	return results
}

// IndexDocument 索引文档
func (p *Processor) IndexDocument(doc *Document) {
	p.documents[doc.ID] = doc

	// 建立倒排索引
	words := tokenize(doc.Content)
	for _, word := range words {
		word = strings.ToLower(word)
		if _, exists := p.index[word]; !exists {
			p.index[word] = []string{doc.ID}
		} else {
			// 避免重复添加
			found := false
			for _, id := range p.index[word] {
				if id == doc.ID {
					found = true
					break
				}
			}
			if !found {
				p.index[word] = append(p.index[word], doc.ID)
			}
		}
	}
}

// GetDocument 获取文档
func (p *Processor) GetDocument(id string) (*Document, bool) {
	doc, exists := p.documents[id]
	return doc, exists
}

// RemoveDocument 移除文档
func (p *Processor) RemoveDocument(id string) {
	if doc, exists := p.documents[id]; exists {
		// 从倒排索引中移除
		words := tokenize(doc.Content)
		for _, word := range words {
			word = strings.ToLower(word)
			if docIDs, ok := p.index[word]; ok {
				newIDs := make([]string, 0, len(docIDs)-1)
				for _, docID := range docIDs {
					if docID != id {
						newIDs = append(newIDs, docID)
					}
				}
				if len(newIDs) == 0 {
					delete(p.index, word)
				} else {
					p.index[word] = newIDs
				}
			}
		}
		delete(p.documents, id)
	}
}

// ListDocuments 列出所有文档
func (p *Processor) ListDocuments() []*Document {
	docs := make([]*Document, 0, len(p.documents))
	for _, doc := range p.documents {
		docs = append(docs, doc)
	}
	return docs
}

// 辅助函数

// countWords 计算单词数
func countWords(text string) int {
	return len(strings.Fields(text))
}

// countLines 计算行数
func countLines(text string) int {
	if text == "" {
		return 0
	}
	return strings.Count(text, "\n") + 1
}

// detectLanguage 检测语言（简单实现）
func detectLanguage(text string) string {
	// 简单的中英文检测
	chinesePattern := regexp.MustCompile(`[\x{4e00}-\x{9fff}]`)
	if chinesePattern.MatchString(text) {
		return "zh"
	}
	return "en"
}

// extractKeywords 提取关键词
func extractKeywords(text string) []string {
	// 移除标点符号
	cleaner := regexp.MustCompile(`[^\w\s\x{4e00}-\x{9fff}]`)
	cleaned := cleaner.ReplaceAllString(text, " ")

	// 分词
	words := tokenize(cleaned)

	// 统计词频
	freq := make(map[string]int)
	for _, word := range words {
		word = strings.ToLower(strings.TrimSpace(word))
		if len(word) > 1 {
			freq[word]++
		}
	}

	// 按频率排序
	type wordFreq struct {
		word string
		freq int
	}
	var sorted []wordFreq
	for w, f := range freq {
		sorted = append(sorted, wordFreq{w, f})
	}
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].freq > sorted[j].freq
	})

	// 返回top10关键词
	keywords := make([]string, 0, 10)
	for i := 0; i < len(sorted) && i < 10; i++ {
		keywords = append(keywords, sorted[i].word)
	}
	return keywords
}

// tokenize 分词
func tokenize(text string) []string {
	// 简单的空格分词
	words := strings.Fields(text)
	var result []string
	for _, w := range words {
		// 对于中文，按字符拆分
		for _, r := range w {
			if r >= 0x4e00 && r <= 0x9fff {
				result = append(result, string(r))
			}
		}
		// 英文单词整体添加
		if regexp.MustCompile(`^[a-zA-Z]+$`).MatchString(w) {
			result = append(result, w)
		}
	}
	return result
}

// computeHash 计算内容哈希
func computeHash(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h[:8])
}

// extractMetadata 提取元数据
func extractMetadata(doc *Document) map[string]string {
	metadata := make(map[string]string)

	metadata["filename"] = doc.Name
	metadata["type"] = doc.Type.String()
	metadata["size"] = fmt.Sprintf("%d", doc.Size)
	metadata["word_count"] = fmt.Sprintf("%d", countWords(doc.Content))
	metadata["line_count"] = fmt.Sprintf("%d", countLines(doc.Content))

	return metadata
}

// extractSummary 提取摘要
func extractSummary(content string, maxLength int) string {
	// 按段落分割
	paragraphs := strings.Split(content, "\n\n")

	if maxLength <= 0 {
		maxLength = 200
	}

	// 收集摘要内容
	var summary strings.Builder
	for _, para := range paragraphs {
		para = strings.TrimSpace(para)
		if para == "" {
			continue
		}

		if summary.Len()+len(para) > maxLength {
			// 截断到指定长度
			remaining := maxLength - summary.Len()
			if remaining > 0 {
				summary.WriteString(para[:remaining])
				summary.WriteString("...")
			}
			break
		}

		if summary.Len() > 0 {
			summary.WriteString("\n")
		}
		summary.WriteString(para)
	}

	return summary.String()
}

// extractSnippets 提取匹配片段
func extractSnippets(content string, keywords []string, maxSnippets int) []string {
	var snippets []string
	lines := strings.Split(content, "\n")

	for _, line := range lines {
		lowerLine := strings.ToLower(line)
		for _, kw := range keywords {
			if strings.Contains(lowerLine, strings.ToLower(kw)) {
				snippets = append(snippets, strings.TrimSpace(line))
				break
			}
		}
		if len(snippets) >= maxSnippets {
			break
		}
	}

	return snippets
}

// highlightMatches 高亮匹配
func highlightMatches(snippets []string, keywords []string) []string {
	var highlighted []string
	for _, snippet := range snippets {
		highlight := snippet
		for _, kw := range keywords {
			pattern := regexp.MustCompile(`(?i)` + regexp.QuoteMeta(kw))
			highlight = pattern.ReplaceAllString(highlight, "**"+kw+"**")
		}
		highlighted = append(highlighted, highlight)
	}
	return highlighted
}

// generateTags 生成标签
func generateTags(doc *Document) []string {
	var tags []string

	// 基于类型
	tags = append(tags, doc.Type.String())

	// 基于内容长度
	if len(doc.Content) > 10000 {
		tags = append(tags, "长文档")
	} else if len(doc.Content) < 1000 {
		tags = append(tags, "短文档")
	}

	// 基于语言
	if detectLanguage(doc.Content) == "zh" {
		tags = append(tags, "中文")
	} else {
		tags = append(tags, "英文")
	}

	return tags
}

// generateLabels 生成标签
func generateLabels(doc *Document) []string {
	var labels []string

	content := strings.ToLower(doc.Content)

	// 技术类
	techKeywords := []string{"代码", "api", "sdk", "docker", "git", "linux"}
	for _, kw := range techKeywords {
		if strings.Contains(content, kw) {
			labels = append(labels, "技术")
			break
		}
	}

	// 商务类
	bizKeywords := []string{"报告", "会议", "计划", "项目", "合同"}
	for _, kw := range bizKeywords {
		if strings.Contains(content, kw) {
			labels = append(labels, "商务")
			break
		}
	}

	// 教育类
	eduKeywords := []string{"教程", "学习", "课程", "培训", "知识"}
	for _, kw := range eduKeywords {
		if strings.Contains(content, kw) {
			labels = append(labels, "教育")
			break
		}
	}

	return labels
}

// countKeywordMatches 计算关键词匹配数
func countKeywordMatches(content string, keywords []string) int {
	count := 0
	for _, kw := range keywords {
		if strings.Contains(content, kw) {
			count++
		}
	}
	return count
}

// maxInt 返回两个整数中较大的一个
func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
