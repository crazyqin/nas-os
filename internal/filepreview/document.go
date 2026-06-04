package filepreview

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	stdhtml "html"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/extension"
	gmhtml "github.com/yuin/goldmark/renderer/html"
)

// DocumentPreviewer 文档预览器.
type DocumentPreviewer struct {
	config    *PreviewConfig
	semaphore chan struct{}
	mu        sync.RWMutex
}

// NewDocumentPreviewer 创建文档预览器.
func NewDocumentPreviewer(config *PreviewConfig) *DocumentPreviewer {
	if config == nil {
		config = DefaultPreviewConfig()
	}
	return &DocumentPreviewer{
		config:    config,
		semaphore: make(chan struct{}, config.MaxConcurrent),
	}
}

// Generate 生成文档预览.
func (p *DocumentPreviewer) Generate(ctx context.Context, req *PreviewRequest) (*PreviewResult, error) {
	// 检查文件是否存在.
	info, err := os.Stat(req.FilePath)
	if err != nil {
		return nil, ErrFileNotFound
	}

	// 检查文件大小.
	if info.Size() > p.config.MaxFileSize {
		return nil, fmt.Errorf("%w: 文件大小 %d 超过限制 %d", ErrInvalidSize, info.Size(), p.config.MaxFileSize)
	}

	// 检测文档格式.
	format := DetectDocumentFormat(req.FilePath)
	if format == "" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, filepath.Ext(req.FilePath))
	}

	// 限制并发.
	select {
	case p.semaphore <- struct{}{}:
		defer func() { <-p.semaphore }()
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	// 根据格式选择生成方式.
	switch format {
	case DocPDF:
		return p.generatePDFPreview(ctx, req)
	case DocDOCX, DocXLSX, DocPPTX:
		return p.generateOfficePreview(ctx, req, format)
	case DocMarkdown:
		return p.generateMarkdownPreview(ctx, req)
	case DocHTML:
		return p.generateHTMLPreview(ctx, req)
	case DocTXT, DocCSV:
		return p.generateTextPreview(ctx, req, format)
	default:
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, format)
	}
}

// GetDocumentInfo 获取文档信息.
func (p *DocumentPreviewer) GetDocumentInfo(ctx context.Context, filePath string) (*DocumentInfo, error) {
	format := DetectDocumentFormat(filePath)
	if format == "" {
		return nil, fmt.Errorf("%w: %s", ErrUnsupportedFormat, filepath.Ext(filePath))
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return nil, ErrFileNotFound
	}

	docInfo := &DocumentInfo{
		FilePath: filePath,
		Format:   format,
		FileSize: info.Size(),
	}

	// 根据格式获取额外信息.
	switch format {
	case DocPDF:
		pdfInfo, err := p.getPDFInfo(ctx, filePath)
		if err == nil {
			docInfo.PageCount = pdfInfo.PageCount
			docInfo.Title = pdfInfo.Title
			docInfo.Author = pdfInfo.Author
		}
	case DocDOCX:
		docxInfo, err := p.getDOCXInfo(ctx, filePath)
		if err == nil {
			docInfo.Title = docxInfo.Title
			docInfo.Author = docxInfo.Author
		}
	}

	return docInfo, nil
}

// generatePDFPreview 生成 PDF 预览.
func (p *DocumentPreviewer) generatePDFPreview(ctx context.Context, req *PreviewRequest) (*PreviewResult, error) {
	pageNum := req.PageNumber
	if pageNum <= 0 {
		pageNum = 1
	}

	// 生成输出路径.
	outputPath := fmt.Sprintf("%s_p%d.jpg", req.FilePath, pageNum)

	// 使用 pdftoppm 生成预览图.
	cmd := exec.CommandContext(ctx, "pdftoppm",
		"-f", fmt.Sprintf("%d", pageNum),
		"-l", fmt.Sprintf("%d", pageNum),
		"-jpeg",
		"-r", "150", // 150 DPI
		"-scale-to", "1200",
		req.FilePath,
		strings.TrimSuffix(outputPath, filepath.Ext(outputPath)),
	)
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		// 回退到 ImageMagick.
		return p.pdfFallbackPreview(ctx, req, pageNum)
	}

	// 检查生成的文件.
	actualPath := fmt.Sprintf("%s_p%d-1.jpg", req.FilePath, pageNum)
	if _, err := os.Stat(actualPath); err != nil {
		return nil, fmt.Errorf("%w: PDF 预览生成失败", ErrGenerationFailed)
	}

	// 重命名到目标路径.
	if err := os.Rename(actualPath, outputPath); err != nil {
		return nil, err
	}

	stat, _ := os.Stat(outputPath)
	return &PreviewResult{
		FilePath:    req.FilePath,
		FileType:    FileTypeDocument,
		PreviewPath: outputPath,
		ContentType: "image/jpeg",
		Width:       1200,
		FileSize:    stat.Size(),
		GeneratedAt: stat.ModTime(),
		PageNumber:  pageNum,
	}, nil
}

// pdfFallbackPreview PDF 预览回退方案.
func (p *DocumentPreviewer) pdfFallbackPreview(ctx context.Context, req *PreviewRequest, pageNum int) (*PreviewResult, error) {
	outputPath := fmt.Sprintf("%s_p%d.jpg", req.FilePath, pageNum)

	cmd := exec.CommandContext(ctx, "convert",
		"-density", "150",
		fmt.Sprintf("%s[%d]", req.FilePath, pageNum-1),
		"-quality", "85",
		"-resize", "1200x",
		outputPath,
	)
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: PDF 转换失败", ErrGenerationFailed)
	}

	stat, _ := os.Stat(outputPath)
	return &PreviewResult{
		FilePath:    req.FilePath,
		FileType:    FileTypeDocument,
		PreviewPath: outputPath,
		ContentType: "image/jpeg",
		Width:       1200,
		FileSize:    stat.Size(),
		GeneratedAt: stat.ModTime(),
		PageNumber:  pageNum,
	}, nil
}

// generateOfficePreview 生成 Office 文档预览.
func (p *DocumentPreviewer) generateOfficePreview(ctx context.Context, req *PreviewRequest, format DocumentFormat) (*PreviewResult, error) {
	// 创建临时目录.
	tmpDir, err := os.MkdirTemp("", "office-preview-*")
	if err != nil {
		return nil, fmt.Errorf("创建临时目录失败: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	// 使用 LibreOffice 转换为 PDF.
	cmd := exec.CommandContext(ctx, p.config.LibreOfficePath,
		"--headless",
		"--convert-to", "pdf",
		"--outdir", tmpDir,
		req.FilePath,
	)
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: Office 文档转换失败: %s", ErrGenerationFailed, err)
	}

	// 找到生成的 PDF 文件.
	baseName := strings.TrimSuffix(filepath.Base(req.FilePath), filepath.Ext(req.FilePath))
	pdfPath := filepath.Join(tmpDir, baseName+".pdf")

	if _, err := os.Stat(pdfPath); err != nil {
		return nil, fmt.Errorf("%w: 转换后的 PDF 未找到", ErrGenerationFailed)
	}

	// 生成 PDF 预览.
	pdfReq := &PreviewRequest{
		FilePath:   pdfPath,
		PageNumber: req.PageNumber,
		Width:      req.Width,
		Height:     req.Height,
	}

	result, err := p.generatePDFPreview(ctx, pdfReq)
	if err != nil {
		return nil, err
	}

	// 更新结果.
	result.FilePath = req.FilePath
	result.Metadata = map[string]string{
		"original_format": string(format),
		"converted_from":  "office",
	}

	return result, nil
}

// generateMarkdownPreview 生成 Markdown 预览.
func (p *DocumentPreviewer) generateMarkdownPreview(ctx context.Context, req *PreviewRequest) (*PreviewResult, error) {
	// 读取 Markdown 内容.
	content, err := os.ReadFile(req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 转换为 HTML.
	md := goldmark.New(
		goldmark.WithExtensions(
			extension.GFM,
			extension.Typographer,
		),
		goldmark.WithRendererOptions(
			gmhtml.WithUnsafe(),
		),
	)

	var buf bytes.Buffer
	if err := md.Convert(content, &buf); err != nil {
		return nil, fmt.Errorf("Markdown 转换失败: %w", err)
	}

	// 生成完整 HTML.
	htmlContent := p.wrapMarkdownHTML(buf.String(), filepath.Base(req.FilePath))

	// 保存 HTML 文件.
	outputPath := strings.TrimSuffix(req.FilePath, filepath.Ext(req.FilePath)) + "_preview.html"
	if err := os.WriteFile(outputPath, []byte(htmlContent), 0o644); err != nil {
		return nil, fmt.Errorf("保存预览文件失败: %w", err)
	}

	stat, _ := os.Stat(outputPath)
	return &PreviewResult{
		FilePath:    req.FilePath,
		FileType:    FileTypeDocument,
		PreviewPath: outputPath,
		ContentType: "text/html",
		FileSize:    stat.Size(),
		GeneratedAt: stat.ModTime(),
		Metadata: map[string]string{
			"original_format": "markdown",
			"rendered":        "true",
		},
	}, nil
}

// generateHTMLPreview 生成 HTML 预览.
func (p *DocumentPreviewer) generateHTMLPreview(ctx context.Context, req *PreviewRequest) (*PreviewResult, error) {
	content, err := os.ReadFile(req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 包装 HTML 内容（安全处理）.
	safeHTML := p.wrapHTMLContent(string(content), filepath.Base(req.FilePath))

	outputPath := strings.TrimSuffix(req.FilePath, filepath.Ext(req.FilePath)) + "_preview.html"
	if err := os.WriteFile(outputPath, []byte(safeHTML), 0o644); err != nil {
		return nil, fmt.Errorf("保存预览文件失败: %w", err)
	}

	stat, _ := os.Stat(outputPath)
	return &PreviewResult{
		FilePath:    req.FilePath,
		FileType:    FileTypeDocument,
		PreviewPath: outputPath,
		ContentType: "text/html",
		FileSize:    stat.Size(),
		GeneratedAt: stat.ModTime(),
	}, nil
}

// generateTextPreview 生成纯文本预览.
func (p *DocumentPreviewer) generateTextPreview(ctx context.Context, req *PreviewRequest, format DocumentFormat) (*PreviewResult, error) {
	content, err := os.ReadFile(req.FilePath)
	if err != nil {
		return nil, fmt.Errorf("读取文件失败: %w", err)
	}

	// 处理 CSV.
	var htmlContent string
	if format == DocCSV {
		htmlContent = p.csvToHTML(string(content))
	} else {
		// 纯文本转换为 HTML.
		escaped := stdhtml.EscapeString(string(content))
		htmlContent = fmt.Sprintf("<pre class=\"text-preview\">%s</pre>", escaped)
	}

	// 包装完整 HTML.
	fullHTML := p.wrapMarkdownHTML(htmlContent, filepath.Base(req.FilePath))

	outputPath := strings.TrimSuffix(req.FilePath, filepath.Ext(req.FilePath)) + "_preview.html"
	if err := os.WriteFile(outputPath, []byte(fullHTML), 0o644); err != nil {
		return nil, fmt.Errorf("保存预览文件失败: %w", err)
	}

	stat, _ := os.Stat(outputPath)
	return &PreviewResult{
		FilePath:    req.FilePath,
		FileType:    FileTypeDocument,
		PreviewPath: outputPath,
		ContentType: "text/html",
		FileSize:    stat.Size(),
		GeneratedAt: stat.ModTime(),
	}, nil
}

// wrapMarkdownHTML 包装 Markdown HTML.
func (p *DocumentPreviewer) wrapMarkdownHTML(body, title string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html>
<head>
<meta charset="utf-8">
<title>%s</title>
<style>
body {
	font-family: -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
	line-height: 1.6;
	max-width: 900px;
	margin: 0 auto;
	padding: 20px;
	color: #333;
	background: #fff;
}
pre {
	background: #f5f5f5;
	padding: 12px;
	border-radius: 4px;
	overflow-x: auto;
}
code {
	background: #f5f5f5;
	padding: 2px 6px;
	border-radius: 3px;
	font-size: 90%%;
}
pre code {
	background: none;
	padding: 0;
}
img {
	max-width: 100%%;
	height: auto;
}
table {
	border-collapse: collapse;
	width: 100%%;
	margin: 1em 0;
}
th, td {
	border: 1px solid #ddd;
	padding: 8px;
	text-align: left;
}
th {
	background: #f5f5f5;
}
blockquote {
	border-left: 4px solid #ddd;
	margin: 0;
	padding-left: 16px;
	color: #666;
}
.text-preview {
	white-space: pre-wrap;
	word-wrap: break-word;
}
</style>
</head>
<body>
%s
</body>
</html>`, stdhtml.EscapeString(title), body)
}

// wrapHTMLContent 包装 HTML 内容（安全处理）.
func (p *DocumentPreviewer) wrapHTMLContent(content, title string) string {
	// 移除危险标签和属性.
	sanitized := sanitizeHTML(content)
	return p.wrapMarkdownHTML(sanitized, title)
}

// csvToHTML 将 CSV 转换为 HTML 表格.
func (p *DocumentPreviewer) csvToHTML(csvContent string) string {
	if csvContent == "" {
		return "<p>空文件</p>"
	}

	lines := strings.Split(csvContent, "\n")
	if len(lines) == 0 {
		return "<p>空文件</p>"
	}

	var buf strings.Builder
	buf.WriteString("<table>\n")

	for i, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, ",")
		if i == 0 {
			buf.WriteString("<thead><tr>")
			for _, field := range fields {
				buf.WriteString(fmt.Sprintf("<th>%s</th>", stdhtml.EscapeString(strings.TrimSpace(field))))
			}
			buf.WriteString("</tr></thead>\n<tbody>\n")
		} else {
			buf.WriteString("<tr>")
			for _, field := range fields {
				buf.WriteString(fmt.Sprintf("<td>%s</td>", stdhtml.EscapeString(strings.TrimSpace(field))))
			}
			buf.WriteString("</tr>\n")
		}
	}

	buf.WriteString("</tbody></table>")
	return buf.String()
}

// getPDFInfo 获取 PDF 信息.
func (p *DocumentPreviewer) getPDFInfo(ctx context.Context, filePath string) (*DocumentInfo, error) {
	cmd := exec.CommandContext(ctx, "pdfinfo", filePath)
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{}

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	info := &DocumentInfo{
		FilePath: filePath,
		Format:   DocPDF,
	}

	// 解析 pdfinfo 输出.
	lines := strings.Split(stdout.String(), "\n")
	for _, line := range lines {
		parts := strings.SplitN(line, ":", 2)
		if len(parts) != 2 {
			continue
		}
		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch key {
		case "Pages":
			fmt.Sscanf(value, "%d", &info.PageCount)
		case "Title":
			info.Title = value
		case "Author":
			info.Author = value
		case "CreationDate":
			if t, err := time.Parse("Mon Jan 2 15:04:05 2006", value); err == nil {
				info.CreatedAt = &t
			}
		}
	}

	return info, nil
}

// getDOCXInfo 获取 DOCX 信息.
func (p *DocumentPreviewer) getDOCXInfo(ctx context.Context, filePath string) (*DocumentInfo, error) {
	// 使用 python-docx 或 unzip 提取元数据.
	cmd := exec.CommandContext(ctx, "python3", "-c", fmt.Sprintf(`
import zipfile
import xml.etree.ElementTree as ET

with zipfile.ZipFile('%s', 'r') as z:
    if 'docProps/core.xml' in z.namelist():
        with z.open('docProps/core.xml') as f:
            tree = ET.parse(f)
            root = tree.getroot()
            ns = {'cp': 'http://schemas.openxmlformats.org/package/2006/metadata/core-properties'}
            title = root.find('.//cp:title', ns)
            creator = root.find('.//cp:creator', ns)
            result = {}
            if title is not None:
                result['title'] = title.text
            if creator is not None:
                result['author'] = creator.text
            print(json.dumps(result))
`, filePath))
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &bytes.Buffer{}

	info := &DocumentInfo{
		FilePath: filePath,
		Format:   DocDOCX,
	}

	if err := cmd.Run(); err != nil {
		return info, nil // 非致命错误
	}

	var metadata map[string]string
	if err := json.Unmarshal(stdout.Bytes(), &metadata); err != nil {
		return info, nil
	}

	info.Title = metadata["title"]
	info.Author = metadata["author"]

	return info, nil
}

// sanitizeHTML 清理 HTML 内容.
func sanitizeHTML(content string) string {
	// 移除 script 标签.
	content = removeTag(content, "script")
	content = removeTag(content, "style")
	content = removeTag(content, "iframe")
	content = removeTag(content, "object")
	content = removeTag(content, "embed")

	// 移除事件属性.
	content = removeEventAttributes(content)

	return content
}

// removeTag 移除 HTML 标签.
func removeTag(content, tag string) string {
	start := strings.ToLower(fmt.Sprintf("<%s", tag))
	end := fmt.Sprintf("</%s>", tag)

	for {
		startIdx := strings.Index(strings.ToLower(content), start)
		if startIdx == -1 {
			break
		}

		endIdx := strings.Index(strings.ToLower(content[startIdx:]), end)
		if endIdx == -1 {
			break
		}

		content = content[:startIdx] + content[startIdx+endIdx+len(end):]
	}

	return content
}

// removeEventAttributes 移除事件属性.
func removeEventAttributes(content string) string {
	// 简单实现：移除 onclick, onload 等属性.
	events := []string{"onclick", "onload", "onerror", "onmouseover", "onfocus", "onblur"}
	for _, event := range events {
		for {
			idx := strings.Index(strings.ToLower(content), event+"=")
			if idx == -1 {
				break
			}
			// 找到属性值结束位置.
			start := idx + len(event) + 1
			if start >= len(content) {
				break
			}
			quote := content[start]
			if quote == '"' || quote == '\'' {
				endIdx := strings.IndexByte(content[start+1:], quote)
				if endIdx != -1 {
					content = content[:idx] + content[start+endIdx+2:]
				}
			}
		}
	}
	return content
}
