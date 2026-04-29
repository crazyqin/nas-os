package truesearch

import (
	"archive/zip"
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"go.uber.org/zap"
)

// Extractor 文件内容提取器。
// 支持 txt/md/pdf/docx 格式的文本内容提取。
type Extractor struct {
	maxFileSize int64
	logger      *zap.Logger
}

// NewExtractor 创建内容提取器。
func NewExtractor(maxFileSize int64, logger *zap.Logger) *Extractor {
	return &Extractor{
		maxFileSize: maxFileSize,
		logger:      logger,
	}
}

// Extract 提取文件内容。
func (e *Extractor) Extract(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", fmt.Errorf("stat file: %w", err)
	}
	if info.Size() > e.maxFileSize {
		return "", fmt.Errorf("file too large: %d > %d", info.Size(), e.maxFileSize)
	}
	if info.IsDir() {
		return "", nil
	}

	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".txt", ".md", ".markdown", ".json", ".yaml", ".yml",
		".xml", ".csv", ".log", ".conf", ".cfg", ".ini", ".env",
		".go", ".py", ".js", ".ts", ".rs", ".java", ".c", ".cpp", ".h", ".sh":
		return e.extractText(path)
	case ".pdf":
		return e.extractPDF(path)
	case ".docx":
		return e.extractDOCX(path)
	default:
		return "", nil
	}
}

// extractText 提取纯文本文件内容。
func (e *Extractor) extractText(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read file: %w", err)
	}
	// 只取前 500KB 做索引，避免巨大文件拖慢搜索
	const maxIndexSize = 500 * 1024
	if len(data) > maxIndexSize {
		data = data[:maxIndexSize]
	}
	return string(data), nil
}

// extractPDF 提取 PDF 文件内容。
// 使用纯 Go 实现，解析 PDF 流对象提取文本。
func (e *Extractor) extractPDF(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read pdf: %w", err)
	}
	text := extractPDFText(data)
	if text == "" {
		return "", nil
	}
	const maxIndexSize = 500 * 1024
	if len(text) > maxIndexSize {
		text = text[:maxIndexSize]
	}
	return text, nil
}

// extractDOCX 提取 DOCX 文件内容。
// DOCX 是 ZIP 格式，包含 word/document.xml。
func (e *Extractor) extractDOCX(path string) (string, error) {
	r, err := zip.OpenReader(path)
	if err != nil {
		return "", fmt.Errorf("open docx: %w", err)
	}
	defer func() { _ = r.Close() }()

	for _, f := range r.File {
		if f.Name == "word/document.xml" {
			rc, err := f.Open()
			if err != nil {
				return "", fmt.Errorf("open document.xml: %w", err)
			}
			defer func() { _ = rc.Close() }()

			var buf bytes.Buffer
			if _, err := io.Copy(&buf, rc); err != nil {
				return "", fmt.Errorf("read document.xml: %w", err)
			}
			text := extractXMLText(buf.String())
			const maxIndexSize = 500 * 1024
			if len(text) > maxIndexSize {
				text = text[:maxIndexSize]
			}
			return text, nil
		}
	}
	return "", nil
}

// extractPDFText 从 PDF 二进制数据中提取文本。
// 使用正则匹配 PDF 文本操作符 (Tj, TJ, ')。
func extractPDFText(data []byte) string {
	var result strings.Builder

	// 匹配括号内的文本: (text) Tj 或 (text) '
	tjPattern := regexp.MustCompile(`\(([^)]*)\)\s*(?:Tj|')`)
	for _, match := range tjPattern.FindAllSubmatch(data, -1) {
		if len(match) > 1 {
			result.Write(match[1])
			result.WriteByte(' ')
		}
	}

	// 匹配数组文本: [(text) ...] TJ
	tjArrayPattern := regexp.MustCompile(`\[([^\]]*)\]\s*TJ`)
	for _, match := range tjArrayPattern.FindAllSubmatch(data, -1) {
		if len(match) > 1 {
			inner := match[1]
			parenPattern := regexp.MustCompile(`\(([^)]*)\)`)
			for _, pm := range parenPattern.FindAllSubmatch(inner, -1) {
				if len(pm) > 1 {
					result.Write(pm[1])
				}
			}
			result.WriteByte(' ')
		}
	}

	// 匹配 BT ... ET 文本块
	btPattern := regexp.MustCompile(`BT\s*(.*?)\s*ET`)
	for _, match := range btPattern.FindAllSubmatch(data, -1) {
		if len(match) > 1 {
			textOps := match[1]
			parenPattern := regexp.MustCompile(`\(([^)]*)\)`)
			for _, pm := range parenPattern.FindAllSubmatch(textOps, -1) {
				if len(pm) > 1 {
					result.Write(pm[1])
					result.WriteByte(' ')
				}
			}
		}
	}

	return strings.TrimSpace(result.String())
}

// extractXMLText 从 XML 中提取纯文本内容。
// 去除所有 XML 标签，保留文本节点。
func extractXMLText(xmlContent string) string {
	// 去除 XML 标签
	tagPattern := regexp.MustCompile(`<[^>]*>`)
	text := tagPattern.ReplaceAllString(xmlContent, " ")

	// 处理 XML 实体
	text = strings.ReplaceAll(text, "&amp;", "&")
	text = strings.ReplaceAll(text, "&lt;", "<")
	text = strings.ReplaceAll(text, "&gt;", ">")
	text = strings.ReplaceAll(text, "&quot;", "\"")
	text = strings.ReplaceAll(text, "&apos;", "'")
	text = strings.ReplaceAll(text, "&#xA;", "\n")
	text = strings.ReplaceAll(text, "&#xD;", "\r")
	text = strings.ReplaceAll(text, "&#x9;", "\t")

	// 清理多余空白
	spacePattern := regexp.MustCompile(`\s+`)
	text = spacePattern.ReplaceAllString(text, " ")

	return strings.TrimSpace(text)
}
