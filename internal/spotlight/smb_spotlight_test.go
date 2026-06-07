package spotlight

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func TestNewSMBSpotlightProtocol(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEngineConfig()
	// 使用临时目录避免权限问题
	tmpDir := t.TempDir()
	config.IndexPath = tmpDir + "/index.bleve"
	engine, err := NewEngine(config, logger)
	require.NoError(t, err)
	defer engine.Stop()

	protocol := NewSMBSpotlightProtocol(engine)
	assert.NotNil(t, protocol)
	assert.NotNil(t, protocol.engine)
}

func TestParseKMDQueryAttributes(t *testing.T) {
	tests := []struct {
		name     string
		query    string
		expected map[string]string
	}{
		{
			name:  "单个属性",
			query: `kMDItemDisplayName == "test"`,
			expected: map[string]string{
				kMDItemDisplayName: `"test"`,
			},
		},
		{
			name:  "多个属性",
			query: `kMDItemDisplayName == "test*" && kMDItemContentType == "public.text"`,
			expected: map[string]string{
				kMDItemDisplayName: `"test*"`,
				kMDItemContentType: `"public.text"`,
			},
		},
		{
			name:     "空查询",
			query:    "",
			expected: map[string]string{},
		},
		{
			name:  "带空格的查询",
			query: ` kMDItemDisplayName == "test file" `,
			expected: map[string]string{
				kMDItemDisplayName: `"test file"`,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseKMDQueryAttributes(tt.query)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestIsValidKMDAttribute(t *testing.T) {
	validAttrs := []string{
		kMDItemDisplayName,
		kMDItemPath,
		kMDItemFSSize,
		kMDItemFSCreationDate,
		kMDItemFSContentChangeDate,
		kMDItemContentType,
		kMDItemKind,
		kMDItemTextContent,
		kMDItemKeywords,
		kMDItemFSName,
	}

	for _, attr := range validAttrs {
		assert.True(t, isValidKMDAttribute(attr), "应该是有效属性: %s", attr)
	}

	invalidAttrs := []string{
		"invalid",
		"kMDItemInvalid",
		"",
	}

	for _, attr := range invalidAttrs {
		assert.False(t, isValidKMDAttribute(attr), "应该是无效属性: %s", attr)
	}
}

func TestParseSizeValue(t *testing.T) {
	tests := []struct {
		name     string
		value    string
		expected int64
	}{
		{name: "字节", value: "1024", expected: 1024},
		{name: "KB", value: "1KB", expected: 1024},
		{name: "MB", value: "10MB", expected: 10 * 1024 * 1024},
		{name: "GB", value: "1GB", expected: 1024 * 1024 * 1024},
		{name: "大于", value: ">1MB", expected: 1024 * 1024},
		{name: "小于", value: "<100KB", expected: 100 * 1024},
		{name: "空值", value: "", expected: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseSizeValue(tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetFileKind(t *testing.T) {
	tests := []struct {
		ext      string
		expected string
	}{
		{".txt", "纯文本文档"},
		{".pdf", "PDF 文档"},
		{".doc", "Word 文档"},
		{".docx", "Word 文档"},
		{".jpg", "JPEG 图像"},
		{".png", "PNG 图像"},
		{".mp4", "MP4 视频"},
		{".mp3", "MP3 音频"},
		{".zip", "ZIP 压缩包"},
		{".go", "Go 源代码"},
		{".unknown", "文件"},
	}

	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			result := getFileKind(tt.ext)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestEncodeDecodeResponse(t *testing.T) {
	resp := &SpotlightResponse{
		Command:    CmdQuery,
		Status:     0,
		QueryID:    12345,
		TotalCount: 2,
		HasMore:    false,
		Results: []SpotlightResult{
			{
				Path: "/test/file1.txt",
				Attributes: map[string]interface{}{
					kMDItemDisplayName: "file1.txt",
					kMDItemFSSize:      int64(1024),
				},
			},
			{
				Path: "/test/file2.txt",
				Attributes: map[string]interface{}{
					kMDItemDisplayName: "file2.txt",
					kMDItemFSSize:      int64(2048),
				},
			},
		},
	}

	// 编码
	data, err := EncodeResponse(resp)
	require.NoError(t, err)
	assert.NotEmpty(t, data)

	// 验证头部
	assert.Equal(t, byte(0), data[0]) // CmdQuery = 1, 但高位在前
	assert.Equal(t, byte(1), data[3]) // CmdQuery = 0x01
}

func TestConvertQuery(t *testing.T) {
	logger := zap.NewNop()
	config := DefaultEngineConfig()
	// 使用临时目录避免权限问题
	tmpDir := t.TempDir()
	config.IndexPath = tmpDir + "/index.bleve"
	engine, err := NewEngine(config, logger)
	require.NoError(t, err)
	defer engine.Stop()

	protocol := NewSMBSpotlightProtocol(engine)

	tests := []struct {
		name       string
		query      string
		maxResults uint32
		check      func(EngineSearchRequest) bool
	}{
		{
			name:       "基本查询",
			query:      `kMDItemDisplayName == "test"`,
			maxResults: 50,
			check: func(req EngineSearchRequest) bool {
				return req.Query == "test" && req.Limit == 50
			},
		},
		{
			name:       "带通配符",
			query:      `kMDItemDisplayName == "test*"`,
			maxResults: 100,
			check: func(req EngineSearchRequest) bool {
				return req.Query == "test" && req.Limit == 100
			},
		},
		{
			name:       "简单查询",
			query:      "test query",
			maxResults: 50,
			check: func(req EngineSearchRequest) bool {
				return req.Query == "test query" && req.Limit == 50
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := protocol.convertQuery(tt.query, tt.maxResults)
			assert.True(t, tt.check(result))
		})
	}
}

func TestSpotlightCommandConstants(t *testing.T) {
	assert.Equal(t, SpotlightCommand(0x01), CmdQuery)
	assert.Equal(t, SpotlightCommand(0x02), CmdFetch)
	assert.Equal(t, SpotlightCommand(0x03), CmdClose)
	assert.Equal(t, SpotlightCommand(0x04), CmdPing)
	assert.Equal(t, SpotlightCommand(0x05), CmdRegister)
	assert.Equal(t, SpotlightCommand(0x06), CmdUnregister)
}

func TestKMDAttributeConstants(t *testing.T) {
	assert.Equal(t, "kMDItemDisplayName", kMDItemDisplayName)
	assert.Equal(t, "kMDItemPath", kMDItemPath)
	assert.Equal(t, "kMDItemFSSize", kMDItemFSSize)
	assert.Equal(t, "kMDItemFSCreationDate", kMDItemFSCreationDate)
	assert.Equal(t, "kMDItemFSContentChangeDate", kMDItemFSContentChangeDate)
	assert.Equal(t, "kMDItemContentType", kMDItemContentType)
	assert.Equal(t, "kMDItemKind", kMDItemKind)
	assert.Equal(t, "kMDItemTextContent", kMDItemTextContent)
	assert.Equal(t, "kMDItemKeywords", kMDItemKeywords)
	assert.Equal(t, "kMDItemFSName", kMDItemFSName)
	assert.Equal(t, "kMDItemContentTypeTree", kMDItemContentTypeTree)
}
