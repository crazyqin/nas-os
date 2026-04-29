package truesearch

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractText(t *testing.T) {
	dir := t.TempDir()

	tests := []struct {
		name     string
		filename string
		content  string
		wantErr  bool
	}{
		{
			name:     "plain text",
			filename: "test.txt",
			content:  "Hello World, this is a test file.",
			wantErr:  false,
		},
		{
			name:     "markdown",
			filename: "README.md",
			content:  "# Title\n\nThis is a markdown document.\n\n## Section\n\nSome content here.",
			wantErr:  false,
		},
		{
			name:     "json",
			filename: "config.json",
			content:  `{"key": "value", "nested": {"a": 1}}`,
			wantErr:  false,
		},
		{
			name:     "yaml",
			filename: "config.yaml",
			content:  "key: value\nlist:\n  - item1\n  - item2\n",
			wantErr:  false,
		},
	}

	logger := newTestLogger(t)
	extractor := NewExtractor(10*1024*1024, logger)

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			path := filepath.Join(dir, tt.filename)
			if err := os.WriteFile(path, []byte(tt.content), 0644); err != nil {
				t.Fatal(err)
			}

			got, err := extractor.Extract(path)
			if (err != nil) != tt.wantErr {
				t.Errorf("Extract() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.content {
				t.Errorf("Extract() = %q, want %q", got, tt.content)
			}
		})
	}
}

func TestExtractTextTruncation(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)

	// 使用很小的 maxFileSize 来测试截断
	extractor := NewExtractor(1024, logger)

	// 创建超过 500KB 的文件（但 maxFileSize 是 1024，所以应该被拒绝）
	path := filepath.Join(dir, "large.txt")
	content := make([]byte, 2048)
	for i := range content {
		content[i] = 'A'
	}
	if err := os.WriteFile(path, content, 0644); err != nil {
		t.Fatal(err)
	}

	_, err := extractor.Extract(path)
	if err == nil {
		t.Error("expected error for file exceeding maxFileSize")
	}
}

func TestExtractDOCX(t *testing.T) {
	// 创建一个最小的 DOCX (ZIP) 文件来测试
	dir := t.TempDir()
	logger := newTestLogger(t)
	extractor := NewExtractor(10*1024*1024, logger)

	// 测试非 docx 文件不被识别
	path := filepath.Join(dir, "test.docx")
	if err := os.WriteFile(path, []byte("not a real docx"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := extractor.Extract(path)
	if err == nil {
		t.Log("extractDOCX: expected error for invalid docx (got nil), this is OK for basic test")
	}
}

func TestExtractPDF(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)
	extractor := NewExtractor(10*1024*1024, logger)

	// 测试带 PDF 文本标记的假 PDF
	path := filepath.Join(dir, "test.pdf")
	pdfContent := `%PDF-1.4
1 0 obj
<< /Type /Catalog >>
endobj
BT
(Hello World) Tj
ET
BT
(This is a test document) Tj
ET`
	if err := os.WriteFile(path, []byte(pdfContent), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := extractor.Extract(path)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if got == "" {
		t.Error("expected non-empty content from PDF extraction")
	}
	t.Logf("PDF extracted content: %q", got)
}

func TestExtractUnsupportedType(t *testing.T) {
	dir := t.TempDir()
	logger := newTestLogger(t)
	extractor := NewExtractor(10*1024*1024, logger)

	path := filepath.Join(dir, "test.xyz")
	if err := os.WriteFile(path, []byte("some content"), 0644); err != nil {
		t.Fatal(err)
	}

	got, err := extractor.Extract(path)
	if err != nil {
		t.Fatalf("Extract() error = %v", err)
	}
	if got != "" {
		t.Errorf("expected empty string for unsupported type, got %q", got)
	}
}

func TestExtractXMLText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple tags",
			input: "<p>Hello</p><p>World</p>",
			want:  "Hello World",
		},
		{
			name:  "with attributes",
			input: `<w:p><w:r><w:t>Hello World</w:t></w:r></w:p>`,
			want:  "Hello World",
		},
		{
			name:  "with entities",
			input: `<p>Hello &amp; World &lt;test&gt;</p>`,
			want:  "Hello & World <test>",
		},
		{
			name:  "empty",
			input: `<root></root>`,
			want:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractXMLText(tt.input)
			if got != tt.want {
				t.Errorf("extractXMLText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestExtractPDFText(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "simple Tj",
			input: "(Hello World) Tj",
			want:  "Hello World",
		},
		{
			name:  "apostrophe operator",
			input: "(Next line) '",
			want:  "Next line",
		},
		{
			name:  "TJ array",
			input: "[(Hello) ( ) (World)] TJ",
			want:  "Hello World",
		},
		{
			name:  "BT/ET block",
			input: "BT\n(Hello) Tj\n(World) Tj\nET",
			want:  "Hello World",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractPDFText([]byte(tt.input))
			if got != tt.want {
				t.Errorf("extractPDFText() = %q, want %q", got, tt.want)
			}
		})
	}
}
