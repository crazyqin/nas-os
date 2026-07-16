// Package knowledgebase 提供个人知识库管理功能
package knowledgebase

import (
	"regexp"
	"strings"
	"sync"
)

// Editor 富文本编辑器.
type Editor struct {
	mu       sync.RWMutex
	content  string
	metadata map[string]string
}

// NewEditor 创建编辑器.
func NewEditor() *Editor {
	return &Editor{
		metadata: make(map[string]string),
	}
}

// RenderResult 渲染结果.
type RenderResult struct {
	HTML    string `json:"html"`
	Plain   string `json:"plain"`
	Summary string `json:"summary"`
}

// SetContent 设置内容.
func (e *Editor) SetContent(content string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.content = content
}

// GetContent 获取内容.
func (e *Editor) GetContent() string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return e.content
}

// SetMetadata 设置元数据.
func (e *Editor) SetMetadata(key, value string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.metadata[key] = value
}

// GetMetadata 获取元数据.
func (e *Editor) GetMetadata(key string) (string, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	val, ok := e.metadata[key]
	return val, ok
}

// RenderMarkdown 渲染Markdown.
func (e *Editor) RenderMarkdown(content string) RenderResult {
	if content == "" {
		e.mu.RLock()
		content = e.content
		e.mu.RUnlock()
	}

	html := content

	// 标题
	html = regexp.MustCompile(`(?m)^### (.+)$`).ReplaceAllString(html, "<h3>$1</h3>")
	html = regexp.MustCompile(`(?m)^## (.+)$`).ReplaceAllString(html, "<h2>$1</h2>")
	html = regexp.MustCompile(`(?m)^# (.+)$`).ReplaceAllString(html, "<h1>$1</h1>")

	// 粗体和斜体
	html = regexp.MustCompile(`\*\*(.+?)\*\*`).ReplaceAllString(html, "<strong>$1</strong>")
	html = regexp.MustCompile(`\*(.+?)\*`).ReplaceAllString(html, "<em>$1</em>")

	// 代码块
	html = regexp.MustCompile("(?s)```(\\w*)\\n(.+?)```").ReplaceAllString(html, "<pre><code class=\"language-$1\">$2</code></pre>")

	// 行内代码
	html = regexp.MustCompile("`([^`]+)`").ReplaceAllString(html, "<code>$1</code>")

	// 链接
	html = regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`).ReplaceAllString(html, "<a href=\"$2\">$1</a>")

	// 图片
	html = regexp.MustCompile(`!\[([^\]]*)\]\(([^)]+)\)`).ReplaceAllString(html, "<img src=\"$2\" alt=\"$1\">")

	// 列表
	html = regexp.MustCompile(`(?m)^- (.+)$`).ReplaceAllString(html, "<li>$1</li>")

	// 段落
	lines := strings.Split(html, "\n")
	processed := make([]string, 0)
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		if trimmed != "" && !strings.HasPrefix(trimmed, "<") {
			processed = append(processed, "<p>"+trimmed+"</p>")
		} else {
			processed = append(processed, line)
		}
	}
	html = strings.Join(processed, "\n")

	// 生成纯文本
	plain := regexp.MustCompile(`<[^>]+>`).ReplaceAllString(html, "")
	plain = strings.TrimSpace(plain)

	// 生成摘要
	summary := plain
	if len(summary) > 200 {
		summary = summary[:200] + "..."
	}

	return RenderResult{
		HTML:    html,
		Plain:   plain,
		Summary: summary,
	}
}

// HighlightCode 代码高亮.
func (e *Editor) HighlightCode(code, language string) string {
	// 简单的语法高亮包装
	highlighted := code

	// Go 关键词
	if language == "go" || language == "golang" {
		keywords := []string{"func", "var", "const", "type", "struct", "interface", "map", "chan", "go", "select", "case", "switch", "if", "else", "for", "range", "return", "break", "continue", "package", "import"}
		for _, kw := range keywords {
			pattern := regexp.MustCompile(`\b` + kw + `\b`)
			highlighted = pattern.ReplaceAllString(highlighted, `<span class="keyword">`+kw+`</span>`)
		}
	}

	// Python 关键词
	if language == "python" || language == "py" {
		keywords := []string{"def", "class", "import", "from", "if", "elif", "else", "for", "while", "return", "break", "continue", "pass", "yield", "lambda", "with", "as", "try", "except", "finally", "raise"}
		for _, kw := range keywords {
			pattern := regexp.MustCompile(`\b` + kw + `\b`)
			highlighted = pattern.ReplaceAllString(highlighted, `<span class="keyword">`+kw+`</span>`)
		}
	}

	// JavaScript 关键词
	if language == "javascript" || language == "js" {
		keywords := []string{"function", "var", "let", "const", "if", "else", "for", "while", "return", "break", "continue", "class", "extends", "import", "export", "default", "new", "this", "super", "try", "catch", "finally", "throw", "async", "await"}
		for _, kw := range keywords {
			pattern := regexp.MustCompile(`\b` + kw + `\b`)
			highlighted = pattern.ReplaceAllString(highlighted, `<span class="keyword">`+kw+`</span>`)
		}
	}

	return `<pre><code class="language-` + language + `">` + highlighted + `</code></pre>`
}

// RenderMath 渲染数学公式.
func (e *Editor) RenderMath(content string) string {
	// 行内公式 $...$
	content = regexp.MustCompile(`\$([^$]+)\$`).ReplaceAllString(content, `<span class="math-inline">$1</span>`)

	// 块级公式 $$...$$
	content = regexp.MustCompile(`(?s)\$\$(.+?)\$\$`).ReplaceAllString(content, `<div class="math-block">$1</div>`)

	return content
}

// ExtractLinks 提取链接.
func (e *Editor) ExtractLinks(content string) []string {
	pattern := regexp.MustCompile(`\[([^\]]+)\]\(([^)]+)\)`)
	matches := pattern.FindAllStringSubmatch(content, -1)

	links := make([]string, 0)
	for _, match := range matches {
		if len(match) > 2 {
			links = append(links, match[2])
		}
	}
	return links
}

// ExtractWikiLinks 提取Wiki风格链接 [[...]].
func (e *Editor) ExtractWikiLinks(content string) []string {
	pattern := regexp.MustCompile(`\[\[([^\]]+)\]\]`)
	matches := pattern.FindAllStringSubmatch(content, -1)

	links := make([]string, 0)
	for _, match := range matches {
		if len(match) > 1 {
			links = append(links, match[1])
		}
	}
	return links
}

// ExtractTags 提取标签 #tag.
func (e *Editor) ExtractTags(content string) []string {
	pattern := regexp.MustCompile(`#(\w+)`)
	matches := pattern.FindAllStringSubmatch(content, -1)

	tags := make([]string, 0)
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			tag := match[1]
			if !seen[tag] {
				seen[tag] = true
				tags = append(tags, tag)
			}
		}
	}
	return tags
}

// ExtractMentions 提取@提及.
func (e *Editor) ExtractMentions(content string) []string {
	pattern := regexp.MustCompile(`@(\w+)`)
	matches := pattern.FindAllStringSubmatch(content, -1)

	mentions := make([]string, 0)
	seen := make(map[string]bool)
	for _, match := range matches {
		if len(match) > 1 {
			mention := match[1]
			if !seen[mention] {
				seen[mention] = true
				mentions = append(mentions, mention)
			}
		}
	}
	return mentions
}

// TOCItem 目录项.
type TOCItem struct {
	Level int    `json:"level"`
	Title string `json:"title"`
	ID    string `json:"id"`
}

// GenerateTOC 生成目录.
func (e *Editor) GenerateTOC(content string) []TOCItem {
	pattern := regexp.MustCompile(`(?m)^(#{1,6})\s+(.+)$`)
	matches := pattern.FindAllStringSubmatch(content, -1)

	toc := make([]TOCItem, 0)
	for _, match := range matches {
		if len(match) > 2 {
			level := len(match[1])
			title := match[2]
			id := strings.ToLower(strings.ReplaceAll(title, " ", "-"))
			toc = append(toc, TOCItem{
				Level: level,
				Title: title,
				ID:    id,
			})
		}
	}
	return toc
}

// WordCount 统计字数.
func (e *Editor) WordCount(content string) int {
	if content == "" {
		return 0
	}
	words := strings.Fields(content)
	return len(words)
}

// CharCount 统计字符数.
func (e *Editor) CharCount(content string) int {
	return len(content)
}

// ReadingTime 估算阅读时间（分钟）.
func (e *Editor) ReadingTime(content string) int {
	wordCount := e.WordCount(content)
	if wordCount == 0 {
		return 0
	}
	minutes := wordCount / 200
	if minutes == 0 {
		minutes = 1
	}
	return minutes
}

// ReplaceVariables 替换变量 {{var}}.
func (e *Editor) ReplaceVariables(content string, vars map[string]string) string {
	result := content
	for key, value := range vars {
		result = strings.ReplaceAll(result, "{{"+key+"}}", value)
	}
	return result
}

// SanitizeHTML 清理HTML.
func (e *Editor) SanitizeHTML(html string) string {
	// 移除危险标签
	dangerousTags := []string{"script", "iframe", "object", "embed", "form"}
	result := html
	for _, tag := range dangerousTags {
		pattern := regexp.MustCompile(`(?i)<` + tag + `[^>]*>.*?</` + tag + `>`)
		result = pattern.ReplaceAllString(result, "")
	}

	// 移除事件属性
	result = regexp.MustCompile(`(?i)\s+on\w+\s*=\s*"[^"]*"`).ReplaceAllString(result, "")
	result = regexp.MustCompile(`(?i)\s+on\w+\s*=\s*'[^']*'`).ReplaceAllString(result, "")

	return result
}
