// Package knowledgebase 提供个人知识库管理功能
package knowledgebase

import (
	"encoding/json"
	"errors"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Importer 导入器.
type Importer struct {
	mu sync.RWMutex
}

// NewImporter 创建导入器.
func NewImporter() *Importer {
	return &Importer{}
}

// ImportResult 导入结果.
type ImportResult struct {
	Docs    []*Document `json:"docs"`
	Tags    []string    `json:"tags"`
	Links   []Link      `json:"links"`
	Errors  []string    `json:"errors,omitempty"`
}

// NotionExport Notion导出数据结构.
type NotionExport struct {
	Pages []NotionPage `json:"pages"`
}

// NotionPage Notion页面.
type NotionPage struct {
	ID       string         `json:"id"`
	Title    string         `json:"title"`
	Content  string         `json:"content"`
	Tags     []string       `json:"tags"`
	Parent   string         `json:"parent,omitempty"`
	Children []NotionPage   `json:"children,omitempty"`
}

// ObsidianVault Obsidian仓库结构.
type ObsidianVault struct {
	Files []ObsidianFile `json:"files"`
}

// ObsidianFile Obsidian文件.
type ObsidianFile struct {
	Path     string `json:"path"`
	Content  string `json:"content"`
	FrontMatter map[string]interface{} `json:"front_matter,omitempty"`
}

// ConfluenceExport Confluence导出结构.
type ConfluenceExport struct {
	Pages []ConfluencePage `json:"pages"`
}

// ConfluencePage Confluence页面.
type ConfluencePage struct {
	ID      string `json:"id"`
	Title   string `json:"title"`
	Content string `json:"content"`
	Space   string `json:"space"`
	Parent  string `json:"parent,omitempty"`
}

// EvernoteExport Evernote导出结构.
type EvernoteExport struct {
	Notes []EvernoteNote `json:"notes"`
}

// EvernoteNote Evernote笔记.
type EvernoteNote struct {
	GUID     string   `json:"guid"`
	Title    string   `json:"title"`
	Content  string   `json:"content"`
	Tags     []string `json:"tags"`
	Notebook string   `json:"notebook"`
	Created  string   `json:"created"`
	Updated  string   `json:"updated"`
}

// ImportFromNotion 从Notion导入.
func (imp *Importer) ImportFromNotion(data []byte, workspaceID, author string) (*ImportResult, error) {
	imp.mu.Lock()
	defer imp.mu.Unlock()

	var export NotionExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, errors.New("无效的Notion导出格式")
	}

	result := &ImportResult{
		Docs:   make([]*Document, 0),
		Tags:   make([]string, 0),
		Links:  make([]Link, 0),
		Errors: make([]string, 0),
	}

	tagSet := make(map[string]bool)

	var processPage func(page NotionPage, parentID string)
	processPage = func(page NotionPage, parentID string) {
		doc := &Document{
			ID:          "doc_notion_" + page.ID,
			Title:       page.Title,
			Content:     page.Content,
			Author:      author,
			WorkspaceID: workspaceID,
			Tags:        page.Tags,
			Links:       make([]Link, 0),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		// 收集标签
		for _, tag := range page.Tags {
			tagSet[tag] = true
		}

		// 创建父链接
		if parentID != "" {
			link := Link{
				SourceID: doc.ID,
				TargetID: parentID,
				Type:     "parent",
			}
			doc.Links = append(doc.Links, link)
			result.Links = append(result.Links, link)
		}

		// 提取内容中的链接
		extractedLinks := extractMarkdownLinks(doc.Content)
		for _, linkID := range extractedLinks {
			link := Link{
				SourceID: doc.ID,
				TargetID: linkID,
				Type:     "reference",
			}
			doc.Links = append(doc.Links, link)
			result.Links = append(result.Links, link)
		}

		result.Docs = append(result.Docs, doc)

		// 递归处理子页面
		for _, child := range page.Children {
			processPage(child, doc.ID)
		}
	}

	for _, page := range export.Pages {
		processPage(page, "")
	}

	// 收集所有标签
	for tag := range tagSet {
		result.Tags = append(result.Tags, tag)
	}

	return result, nil
}

// ImportFromObsidian 从Obsidian导入.
func (imp *Importer) ImportFromObsidian(data []byte, workspaceID, author string) (*ImportResult, error) {
	imp.mu.Lock()
	defer imp.mu.Unlock()

	var vault ObsidianVault
	if err := json.Unmarshal(data, &vault); err != nil {
		return nil, errors.New("无效的Obsidian导出格式")
	}

	result := &ImportResult{
		Docs:   make([]*Document, 0),
		Tags:   make([]string, 0),
		Links:  make([]Link, 0),
		Errors: make([]string, 0),
	}

	tagSet := make(map[string]bool)

	for _, file := range vault.Files {
		// 只处理markdown文件
		if !strings.HasSuffix(file.Path, ".md") {
			continue
		}

		doc := &Document{
			ID:          "doc_obsidian_" + sanitizeID(file.Path),
			Title:       extractTitle(file.Path, file.Content),
			Content:     file.Content,
			Author:      author,
			WorkspaceID: workspaceID,
			Links:       make([]Link, 0),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		// 从frontmatter提取标签
		if tags, ok := file.FrontMatter["tags"]; ok {
			if tagList, ok := tags.([]interface{}); ok {
				for _, t := range tagList {
					if tag, ok := t.(string); ok {
						doc.Tags = append(doc.Tags, tag)
						tagSet[tag] = true
					}
				}
			}
		}

		// 提取wiki链接
		wikiLinks := extractWikiLinks(file.Content)
		for _, linkTarget := range wikiLinks {
			link := Link{
				SourceID: doc.ID,
				TargetID: "doc_obsidian_" + sanitizeID(linkTarget),
				Type:     "reference",
			}
			doc.Links = append(doc.Links, link)
			result.Links = append(result.Links, link)
		}

		// 提取标签
		extractedTags := extractHashtags(file.Content)
		for _, tag := range extractedTags {
			doc.Tags = append(doc.Tags, tag)
			tagSet[tag] = true
		}

		result.Docs = append(result.Docs, doc)
	}

	// 收集所有标签
	for tag := range tagSet {
		result.Tags = append(result.Tags, tag)
	}

	return result, nil
}

// ImportFromConfluence 从Confluence导入.
func (imp *Importer) ImportFromConfluence(data []byte, workspaceID, author string) (*ImportResult, error) {
	imp.mu.Lock()
	defer imp.mu.Unlock()

	var export ConfluenceExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, errors.New("无效的Confluence导出格式")
	}

	result := &ImportResult{
		Docs:   make([]*Document, 0),
		Tags:   make([]string, 0),
		Links:  make([]Link, 0),
		Errors: make([]string, 0),
	}

	for _, page := range export.Pages {
		doc := &Document{
			ID:          "doc_confluence_" + page.ID,
			Title:       page.Title,
			Content:     convertConfluenceToMarkdown(page.Content),
			Author:      author,
			WorkspaceID: workspaceID,
			Tags:        []string{page.Space},
			Links:       make([]Link, 0),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		// 创建父链接
		if page.Parent != "" {
			link := Link{
				SourceID: doc.ID,
				TargetID: "doc_confluence_" + page.Parent,
				Type:     "parent",
			}
			doc.Links = append(doc.Links, link)
			result.Links = append(result.Links, link)
		}

		result.Docs = append(result.Docs, doc)
	}

	return result, nil
}

// ImportFromEvernote 从Evernote导入.
func (imp *Importer) ImportFromEvernote(data []byte, workspaceID, author string) (*ImportResult, error) {
	imp.mu.Lock()
	defer imp.mu.Unlock()

	var export EvernoteExport
	if err := json.Unmarshal(data, &export); err != nil {
		return nil, errors.New("无效的Evernote导出格式")
	}

	result := &ImportResult{
		Docs:   make([]*Document, 0),
		Tags:   make([]string, 0),
		Links:  make([]Link, 0),
		Errors: make([]string, 0),
	}

	tagSet := make(map[string]bool)

	for _, note := range export.Notes {
		doc := &Document{
			ID:          "doc_evernote_" + note.GUID,
			Title:       note.Title,
			Content:     convertEvernoteToMarkdown(note.Content),
			Author:      author,
			WorkspaceID: workspaceID,
			Tags:        note.Tags,
			Links:       make([]Link, 0),
		}

		// 解析时间
		if note.Created != "" {
			if t, err := time.Parse("20060102T150405Z", note.Created); err == nil {
				doc.CreatedAt = t
			} else {
				doc.CreatedAt = time.Now()
			}
		} else {
			doc.CreatedAt = time.Now()
		}

		if note.Updated != "" {
			if t, err := time.Parse("20060102T150405Z", note.Updated); err == nil {
				doc.UpdatedAt = t
			} else {
				doc.UpdatedAt = time.Now()
			}
		} else {
			doc.UpdatedAt = time.Now()
		}

		// 收集标签
		for _, tag := range note.Tags {
			tagSet[tag] = true
		}

		result.Docs = append(result.Docs, doc)
	}

	// 收集所有标签
	for tag := range tagSet {
		result.Tags = append(result.Tags, tag)
	}

	return result, nil
}

// ImportFromMarkdown 从Markdown导入.
func (imp *Importer) ImportFromMarkdown(content, title, workspaceID, author string) (*Document, error) {
	imp.mu.Lock()
	defer imp.mu.Unlock()

	if title == "" {
		title = extractTitleFromMarkdown(content)
	}

	doc := &Document{
		ID:          "doc_md_" + sanitizeID(title),
		Title:       title,
		Content:     content,
		Author:      author,
		WorkspaceID: workspaceID,
		Tags:        extractHashtags(content),
		Links:       make([]Link, 0),
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	// 提取wiki链接
	wikiLinks := extractWikiLinks(content)
	for _, linkTarget := range wikiLinks {
		link := Link{
			SourceID: doc.ID,
			TargetID: "doc_md_" + sanitizeID(linkTarget),
			Type:     "reference",
		}
		doc.Links = append(doc.Links, link)
	}

	return doc, nil
}

// ExportToMarkdown 导出为Markdown.
func (imp *Importer) ExportToMarkdown(doc *Document) string {
	var sb strings.Builder

	sb.WriteString("# " + doc.Title + "\n\n")

	// 元数据
	sb.WriteString("---\n")
	sb.WriteString("author: " + doc.Author + "\n")
	sb.WriteString("created: " + doc.CreatedAt.Format("2006-01-02 15:04:05") + "\n")
	sb.WriteString("updated: " + doc.UpdatedAt.Format("2006-01-02 15:04:05") + "\n")
	if len(doc.Tags) > 0 {
		sb.WriteString("tags: [" + strings.Join(doc.Tags, ", ") + "]\n")
	}
	sb.WriteString("---\n\n")

	sb.WriteString(doc.Content)

	return sb.String()
}

// ExportToJSON 导出为JSON.
func (imp *Importer) ExportToJSON(doc *Document) ([]byte, error) {
	return json.MarshalIndent(doc, "", "  ")
}

// 辅助函数

func extractMarkdownLinks(content string) []string {
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

func extractWikiLinks(content string) []string {
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

func extractHashtags(content string) []string {
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

func extractTitle(path, content string) string {
	// 从文件名提取
	parts := strings.Split(path, "/")
	if len(parts) > 0 {
		filename := parts[len(parts)-1]
		filename = strings.TrimSuffix(filename, ".md")
		return filename
	}

	// 从内容提取
	return extractTitleFromMarkdown(content)
}

func extractTitleFromMarkdown(content string) string {
	lines := strings.SplitN(content, "\n", 2)
	if len(lines) > 0 {
		line := strings.TrimSpace(lines[0])
		if strings.HasPrefix(line, "# ") {
			return strings.TrimPrefix(line, "# ")
		}
	}
	return "Untitled"
}

func sanitizeID(s string) string {
	reg := regexp.MustCompile(`[^a-zA-Z0-9_-]`)
	return reg.ReplaceAllString(strings.ToLower(s), "_")
}

func convertConfluenceToMarkdown(html string) string {
	// 简单的HTML到Markdown转换
	result := html

	// 标题
	result = regexp.MustCompile(`(?i)<h1[^>]*>(.+?)</h1>`).ReplaceAllString(result, "# $1\n")
	result = regexp.MustCompile(`(?i)<h2[^>]*>(.+?)</h2>`).ReplaceAllString(result, "## $1\n")
	result = regexp.MustCompile(`(?i)<h3[^>]*>(.+?)</h3>`).ReplaceAllString(result, "### $1\n")

	// 粗体和斜体
	result = regexp.MustCompile(`(?i)<strong>(.+?)</strong>`).ReplaceAllString(result, "**$1**")
	result = regexp.MustCompile(`(?i)<em>(.+?)</em>`).ReplaceAllString(result, "*$1*")

	// 链接
	result = regexp.MustCompile(`(?i)<a[^>]*href="([^"]*)"[^>]*>(.+?)</a>`).ReplaceAllString(result, "[$2]($1)")

	// 列表
	result = regexp.MustCompile(`(?i)<li>(.+?)</li>`).ReplaceAllString(result, "- $1\n")

	// 移除其他HTML标签
	result = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(result, "")

	// 清理多余空行
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")

	return strings.TrimSpace(result)
}

func convertEvernoteToMarkdown(enml string) string {
	// ENML到Markdown转换
	result := enml

	// 移除ENML包装
	result = regexp.MustCompile(`(?i)<!DOCTYPE[^>]*>`).ReplaceAllString(result, "")
	result = regexp.MustCompile(`(?i)<en-note[^>]*>`).ReplaceAllString(result, "")
	result = regexp.MustCompile(`(?i)</en-note>`).ReplaceAllString(result, "")

	// 转换标题
	result = regexp.MustCompile(`(?i)<h1[^>]*>(.+?)</h1>`).ReplaceAllString(result, "# $1\n")
	result = regexp.MustCompile(`(?i)<h2[^>]*>(.+?)</h2>`).ReplaceAllString(result, "## $1\n")

	// 转换格式
	result = regexp.MustCompile(`(?i)<b>(.+?)</b>`).ReplaceAllString(result, "**$1**")
	result = regexp.MustCompile(`(?i)<i>(.+?)</i>`).ReplaceAllString(result, "*$1*")

	// 移除其他标签
	result = regexp.MustCompile(`<[^>]+>`).ReplaceAllString(result, "")

	// 清理
	result = regexp.MustCompile(`\n{3,}`).ReplaceAllString(result, "\n\n")

	return strings.TrimSpace(result)
}
