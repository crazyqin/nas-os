package search

import (
	"html"
	"regexp"
	"strings"
)

const (
	// HighlightStart is the start tag for highlighting
	HighlightStart = "<mark>"
	// HighlightEnd is the end tag for highlighting
	HighlightEnd = "</mark>"
)

// Highlighter 高亮处理器
type Highlighter struct {
	startTag string
	endTag   string
}

// NewHighlighter 创建高亮处理器
func NewHighlighter() *Highlighter {
	return &Highlighter{
		startTag: HighlightStart,
		endTag:   HighlightEnd,
	}
}

// HighlightText 高亮文本中的关键词
func (h *Highlighter) HighlightText(text, query string) string {
	if text == "" || query == "" {
		return text
	}

	// 转义HTML
	text = html.EscapeString(text)

	// 分词
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return text
	}

	// 构建正则
	patterns := make([]string, len(terms))
	for i, term := range terms {
		patterns[i] = regexp.QuoteMeta(term)
	}

	// 不区分大小写匹配
	pattern := "(?i)(" + strings.Join(patterns, "|") + ")"
	re := regexp.MustCompile(pattern)

	// 替换为高亮
	result := re.ReplaceAllStringFunc(text, func(match string) string {
		return h.startTag + match + h.endTag
	})

	return result
}

// HighlightWithContext 高亮并显示上下文
func (h *Highlighter) HighlightWithContext(text, query string, contextLen int) []string {
	if text == "" || query == "" {
		return nil
	}

	// 分词
	terms := strings.Fields(query)
	if len(terms) == 0 {
		return nil
	}

	// 构建正则
	patterns := make([]string, len(terms))
	for i, term := range terms {
		patterns[i] = regexp.QuoteMeta(term)
	}
	pattern := "(?i)(" + strings.Join(patterns, "|") + ")"
	re := regexp.MustCompile(pattern)

	// 查找所有匹配位置
	matches := re.FindAllStringIndex(text, -1)
	if len(matches) == 0 {
		return nil
	}

	// 提取带上下文的片段
	fragments := make([]string, 0, len(matches))
	seen := make(map[int]bool)

	for _, match := range matches {
		start := match[0]
		end := match[1]

		// 计算上下文范围
		contextStart := start - contextLen
		if contextStart < 0 {
			contextStart = 0
		}
		contextEnd := end + contextLen
		if contextEnd > len(text) {
			contextEnd = len(text)
		}

		// 避免重复
		key := contextStart*10000 + contextEnd
		if seen[key] {
			continue
		}
		seen[key] = true

		// 提取片段
		fragment := text[contextStart:contextEnd]

		// 添加省略号
		if contextStart > 0 {
			fragment = "..." + fragment
		}
		if contextEnd < len(text) {
			fragment = fragment + "..."
		}

		// 转义并高亮
		fragment = html.EscapeString(fragment)
		fragment = re.ReplaceAllStringFunc(fragment, func(match string) string {
			return h.startTag + match + h.endTag
		})

		fragments = append(fragments, fragment)
	}

	// 限制返回数量
	if len(fragments) > 5 {
		fragments = fragments[:5]
	}

	return fragments
}

// ExtractSnippet 提取摘要
func ExtractSnippet(text string, maxLen int) string {
	if len(text) <= maxLen {
		return text
	}

	// 截取前 maxLen 个字符
	snippet := text[:maxLen]

	// 尝试在词边界截断
	lastSpace := strings.LastIndex(snippet, " ")
	if lastSpace > maxLen/2 {
		snippet = snippet[:lastSpace]
	}

	return snippet + "..."
}

// CleanHighlight 清理高亮标签
func CleanHighlight(text string) string {
	text = strings.ReplaceAll(text, HighlightStart, "")
	text = strings.ReplaceAll(text, HighlightEnd, "")
	return text
}

// FormatHighlight 格式化高亮结果为纯文本
func FormatHighlight(text string) string {
	text = strings.ReplaceAll(text, HighlightStart, "**")
	text = strings.ReplaceAll(text, HighlightEnd, "**")
	return text
}
