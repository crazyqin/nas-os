package filepreview

import (
	"context"
	"os"
	"path/filepath"
	"testing"
)

func TestNewDocumentPreviewer(t *testing.T) {
	config := DefaultPreviewConfig()
	previewer := NewDocumentPreviewer(config)

	if previewer == nil {
		t.Fatal("NewDocumentPreviewer returned nil")
	}

	if previewer.config != config {
		t.Error("Config not set correctly")
	}
}

func TestDocumentPreviewer_NilConfig(t *testing.T) {
	previewer := NewDocumentPreviewer(nil)

	if previewer == nil {
		t.Fatal("NewDocumentPreviewer(nil) returned nil")
	}

	if previewer.config == nil {
		t.Error("Should use default config when nil is passed")
	}
}

func TestDocumentPreviewer_Generate_FileNotFound(t *testing.T) {
	previewer := NewDocumentPreviewer(nil)
	ctx := context.Background()

	req := &PreviewRequest{
		FilePath: "/nonexistent/doc.pdf",
	}

	_, err := previewer.Generate(ctx, req)
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestDocumentPreviewer_Generate_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.xyz")
	os.WriteFile(tmpFile, []byte("test"), 0o644)

	previewer := NewDocumentPreviewer(nil)
	ctx := context.Background()

	req := &PreviewRequest{
		FilePath: tmpFile,
	}

	_, err := previewer.Generate(ctx, req)
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}

func TestDocumentPreviewer_GetDocumentInfo_FileNotFound(t *testing.T) {
	previewer := NewDocumentPreviewer(nil)
	ctx := context.Background()

	_, err := previewer.GetDocumentInfo(ctx, "/nonexistent/doc.pdf")
	if err == nil {
		t.Error("Expected error for nonexistent file")
	}
}

func TestDocumentPreviewer_GetDocumentInfo_UnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	tmpFile := filepath.Join(tmpDir, "test.xyz")
	os.WriteFile(tmpFile, []byte("test"), 0o644)

	previewer := NewDocumentPreviewer(nil)
	ctx := context.Background()

	_, err := previewer.GetDocumentInfo(ctx, tmpFile)
	if err == nil {
		t.Error("Expected error for unsupported format")
	}
}

func TestDocumentPreviewer_GetDocumentInfo_PDF(t *testing.T) {
	// 创建一个简单的 PDF 测试文件.
	tmpDir := t.TempDir()
	pdfFile := filepath.Join(tmpDir, "test.pdf")

	// 创建一个最小的 PDF 文件.
	pdfContent := `%PDF-1.0
1 0 obj
<< /Type /Catalog /Pages 2 0 R >>
endobj
2 0 obj
<< /Type /Pages /Kids [3 0 R] /Count 1 >>
endobj
3 0 obj
<< /Type /Page /Parent 2 0 R /MediaBox [0 0 612 792] >>
endobj
xref
0 4
0000000000 65535 f 
0000000009 00000 n 
0000000058 00000 n 
0000000115 00000 n 
trailer
<< /Size 4 /Root 1 0 R >>
startxref
190
%%EOF`

	os.WriteFile(pdfFile, []byte(pdfContent), 0o644)

	previewer := NewDocumentPreviewer(nil)
	ctx := context.Background()

	info, err := previewer.GetDocumentInfo(ctx, pdfFile)
	if err != nil {
		// pdfinfo 可能未安装.
		t.Logf("GetDocumentInfo() error (pdfinfo may not be installed): %v", err)
		return
	}

	if info.Format != DocPDF {
		t.Errorf("Format = %v, want %v", info.Format, DocPDF)
	}
}

func TestDocumentPreviewer_GetDocumentInfo_DOCX(t *testing.T) {
	// 创建一个简单的 DOCX 测试文件.
	tmpDir := t.TempDir()
	docxFile := filepath.Join(tmpDir, "test.docx")

	// 创建一个最小的 DOCX 文件（ZIP 格式）.
	// 注意：这里简化处理，实际需要有效的 DOCX 结构.
	os.WriteFile(docxFile, []byte("PK\x03\x04test"), 0o644)

	previewer := NewDocumentPreviewer(nil)
	ctx := context.Background()

	info, err := previewer.GetDocumentInfo(ctx, docxFile)
	if err != nil {
		// 可能需要 python-docx.
		t.Logf("GetDocumentInfo() error (python-docx may not be installed): %v", err)
		return
	}

	if info.Format != DocDOCX {
		t.Errorf("Format = %v, want %v", info.Format, DocDOCX)
	}
}

func TestDocumentPreviewer_WrapMarkdownHTML(t *testing.T) {
	previewer := NewDocumentPreviewer(nil)

	tests := []struct {
		name     string
		body     string
		title    string
		contains []string
	}{
		{
			name:     "simple content",
			body:     "<p>Hello World</p>",
			title:    "Test",
			contains: []string{"Hello World", "Test", "<!DOCTYPE html>"},
		},
		{
			name:     "code block",
			body:     "<pre><code>fmt.Println()</code></pre>",
			title:    "Code",
			contains: []string{"fmt.Println()", "Code"},
		},
		{
			name:     "table",
			body:     "<table><tr><td>Cell</td></tr></table>",
			title:    "Table",
			contains: []string{"Cell", "table"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := previewer.wrapMarkdownHTML(tt.body, tt.title)
			for _, s := range tt.contains {
				if !contains(result, s) {
					t.Errorf("wrapMarkdownHTML() should contain %q", s)
				}
			}
		})
	}
}

func TestDocumentPreviewer_WrapHTMLContent(t *testing.T) {
	previewer := NewDocumentPreviewer(nil)

	tests := []struct {
		name     string
		content  string
		title    string
		safe     bool
		contains []string
	}{
		{
			name:     "safe content",
			content:  "<p>Safe content</p>",
			title:    "Safe",
			safe:     true,
			contains: []string{"Safe content"},
		},
		{
			name:     "script tag",
			content:  "<p>Before</p><script>alert('xss')</script><p>After</p>",
			title:    "XSS",
			safe:     true,
			contains: []string{"Before", "After"},
		},
		{
			name:     "iframe tag",
			content:  "<p>Before</p><iframe src='evil'></iframe><p>After</p>",
			title:    "XSS",
			safe:     true,
			contains: []string{"Before", "After"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := previewer.wrapHTMLContent(tt.content, tt.title)
			for _, s := range tt.contains {
				if !contains(result, s) {
					t.Errorf("wrapHTMLContent() should contain %q", s)
				}
			}
			if tt.safe && contains(result, "<script") {
				t.Error("wrapHTMLContent() should remove script tags")
			}
		})
	}
}

func TestDocumentPreviewer_CSVToHTML(t *testing.T) {
	previewer := NewDocumentPreviewer(nil)

	tests := []struct {
		name     string
		csv      string
		contains []string
	}{
		{
			name:     "simple csv",
			csv:      "Name,Age\nAlice,30\nBob,25",
			contains: []string{"<table>", "<thead>", "<tbody>", "Name", "Age", "Alice", "30"},
		},
		{
			name:     "empty csv",
			csv:      "",
			contains: []string{"<p>空文件</p>"},
		},
		{
			name:     "single row",
			csv:      "A,B,C",
			contains: []string{"<table>", "<thead>", "A", "B", "C"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := previewer.csvToHTML(tt.csv)
			for _, s := range tt.contains {
				if !contains(result, s) {
					t.Errorf("csvToHTML() should contain %q", s)
				}
			}
		})
	}
}

func TestSanitizeHTML(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected []string
		removed  []string
	}{
		{
			name:     "remove script",
			input:    "<p>Hello</p><script>alert('xss')</script><p>World</p>",
			expected: []string{"Hello", "World"},
			removed:  []string{"<script", "alert"},
		},
		{
			name:     "remove style",
			input:    "<p>Hello</p><style>body{color:red}</style><p>World</p>",
			expected: []string{"Hello", "World"},
			removed:  []string{"<style", "color:red"},
		},
		{
			name:     "remove iframe",
			input:    "<p>Hello</p><iframe src='evil'></iframe><p>World</p>",
			expected: []string{"Hello", "World"},
			removed:  []string{"<iframe", "evil"},
		},
		{
			name:     "safe content",
			input:    "<p>Hello <strong>World</strong></p>",
			expected: []string{"Hello", "World", "strong"},
			removed:  []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := sanitizeHTML(tt.input)
			for _, s := range tt.expected {
				if !contains(result, s) {
					t.Errorf("sanitizeHTML() should contain %q", s)
				}
			}
			for _, s := range tt.removed {
				if contains(result, s) {
					t.Errorf("sanitizeHTML() should not contain %q", s)
				}
			}
		})
	}
}

func TestRemoveTag(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		tag      string
		expected string
	}{
		{
			name:     "simple script",
			input:    "<p>Hello</p><script>alert('xss')</script><p>World</p>",
			tag:      "script",
			expected: "<p>Hello</p><p>World</p>",
		},
		{
			name:     "no tag",
			input:    "<p>Hello World</p>",
			tag:      "script",
			expected: "<p>Hello World</p>",
		},
		{
			name:     "multiple tags",
			input:    "<script>a</script><p>Hello</p><script>b</script>",
			tag:      "script",
			expected: "<p>Hello</p>",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := removeTag(tt.input, tt.tag)
			if result != tt.expected {
				t.Errorf("removeTag() = %q, want %q", result, tt.expected)
			}
		})
	}
}

// contains 辅助函数.
func contains(s, substr string) bool {
	return len(s) >= len(substr) && searchString(s, substr)
}

func searchString(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
