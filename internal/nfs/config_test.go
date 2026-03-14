package nfs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ========== Config Parser 测试 ==========

func TestNFSConfigParser(t *testing.T) {
	parser := NewConfigParser()
	if parser == nil {
		t.Fatal("NewConfigParser 返回 nil")
	}
}

func TestParseExportsFile(t *testing.T) {
	// 创建测试配置文件
	tmpDir := t.TempDir()
	exportsPath := filepath.Join(tmpDir, "exports")

	exportsContent := `# /etc/exports
/data/share1 192.168.1.0/24(rw,no_root_squash) 10.0.0.0/8(ro)
/data/share2 *(rw,async)
/data/share3 172.16.0.1(sync,no_subtree_check)
`
	if err := os.WriteFile(exportsPath, []byte(exportsContent), 0644); err != nil {
		t.Fatal(err)
	}

	parser := NewConfigParser()
	exports, err := parser.ParseExportsFile(exportsPath)
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	if len(exports) != 3 {
		t.Errorf("导出数量错误: %d", len(exports))
	}

	// 验证第一个导出
	export1 := exports[0]
	if export1.Path != "/data/share1" {
		t.Errorf("导出路径错误: %s", export1.Path)
	}
	if len(export1.Clients) != 2 {
		t.Errorf("客户端数量错误: %d", len(export1.Clients))
	}
}

func TestParseExportsFileNotExist(t *testing.T) {
	parser := NewConfigParser()
	_, err := parser.ParseExportsFile("/nonexistent/exports")
	if err == nil {
		t.Error("文件不存在应该返回错误")
	}
}

func TestParseExportLine(t *testing.T) {
	parser := NewConfigParser()

	tests := []struct {
		line        string
		wantPath    string
		wantClients int
		wantErr     bool
	}{
		{
			line:        "/data/share 192.168.1.0/24(rw,no_root_squash)",
			wantPath:    "/data/share",
			wantClients: 1,
			wantErr:     false,
		},
		{
			line:        "/data/share *(rw)",
			wantPath:    "/data/share",
			wantClients: 1,
			wantErr:     false,
		},
		{
			line:        "/data/share",
			wantPath:    "/data/share",
			wantClients: 0,
			wantErr:     false,
		},
		{
			line:    "relative/path",
			wantErr: true,
		},
		{
			line:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		export, err := parser.parseExportLine(tt.line)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseExportLine(%q) error = %v, wantErr %v", tt.line, err, tt.wantErr)
			continue
		}
		if !tt.wantErr {
			if export.Path != tt.wantPath {
				t.Errorf("Path = %q, want %q", export.Path, tt.wantPath)
			}
			if len(export.Clients) != tt.wantClients {
				t.Errorf("Clients count = %d, want %d", len(export.Clients), tt.wantClients)
			}
		}
	}
}

func TestParseClientSpec(t *testing.T) {
	parser := NewConfigParser()

	tests := []struct {
		spec     string
		wantHost string
		wantOpts int
		wantErr  bool
	}{
		{
			spec:     "192.168.1.0/24(rw,no_root_squash)",
			wantHost: "192.168.1.0/24",
			wantOpts: 2,
			wantErr:  false,
		},
		{
			spec:     "*",
			wantHost: "*",
			wantOpts: 0,
			wantErr:  false,
		},
		{
			spec:     "host.example.com(ro,sync)",
			wantHost: "host.example.com",
			wantOpts: 2,
			wantErr:  false,
		},
	}

	for _, tt := range tests {
		client, err := parser.parseClientSpec(tt.spec)
		if (err != nil) != tt.wantErr {
			t.Errorf("parseClientSpec(%q) error = %v, wantErr %v", tt.spec, err, tt.wantErr)
			continue
		}
		if !tt.wantErr {
			if client.Host != tt.wantHost {
				t.Errorf("Host = %q, want %q", client.Host, tt.wantHost)
			}
			if len(client.Options) != tt.wantOpts {
				t.Errorf("Options count = %d, want %d", len(client.Options), tt.wantOpts)
			}
		}
	}
}

func TestParseClientOptions(t *testing.T) {
	parser := NewConfigParser()

	tests := []struct {
		name    string
		opts    []string
		checkFn func(ExportOptions) bool
	}{
		{
			name: "读写模式",
			opts: []string{"rw", "no_root_squash"},
			checkFn: func(o ExportOptions) bool {
				return o.Rw && o.NoRootSquash
			},
		},
		{
			name: "只读模式",
			opts: []string{"ro"},
			checkFn: func(o ExportOptions) bool {
				return o.Ro && !o.Rw
			},
		},
		{
			name: "异步模式",
			opts: []string{"async"},
			checkFn: func(o ExportOptions) bool {
				return o.Async
			},
		},
		{
			name: "子树检查",
			opts: []string{"subtree_check"},
			checkFn: func(o ExportOptions) bool {
				return o.SubtreeCheck
			},
		},
		{
			name: "安全模式",
			opts: []string{"secure"},
			checkFn: func(o ExportOptions) bool {
				return o.Secure
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.ParseClientOptions(tt.opts)
			if !tt.checkFn(result) {
				t.Errorf("ParseClientOptions(%v) = %+v, check failed", tt.opts, result)
			}
		})
	}
}

func TestWriteExportsFile(t *testing.T) {
	tmpDir := t.TempDir()
	exportsPath := filepath.Join(tmpDir, "exports")

	exports := []*Export{
		{
			Path:    "/data/share1",
			Options: ExportOptions{Rw: true, NoRootSquash: true},
			Clients: []Client{
				{Host: "192.168.1.0/24"},
			},
		},
		{
			Path:    "/data/share2",
			Options: ExportOptions{Ro: true},
			Clients: []Client{
				{Host: "*"},
			},
		},
	}

	parser := NewConfigParser()
	err := parser.WriteExportsFile(exportsPath, exports)
	if err != nil {
		t.Fatalf("WriteExportsFile 失败: %v", err)
	}

	// 验证文件已创建
	if _, err := os.Stat(exportsPath); os.IsNotExist(err) {
		t.Error("文件未创建")
	}

	// 验证内容
	data, err := os.ReadFile(exportsPath)
	if err != nil {
		t.Fatal(err)
	}
	content := string(data)
	if !strings.Contains(content, "/data/share1") {
		t.Error("应该包含导出路径")
	}
}

func TestFormatExportLine(t *testing.T) {
	parser := NewConfigParser()

	tests := []struct {
		name     string
		export   *Export
		contains []string
	}{
		{
			name: "单个客户端",
			export: &Export{
				Path:    "/data/share",
				Options: ExportOptions{Rw: true},
				Clients: []Client{{Host: "192.168.1.0/24"}},
			},
			contains: []string{"/data/share", "192.168.1.0/24", "rw"},
		},
		{
			name: "多个客户端",
			export: &Export{
				Path:    "/data/share",
				Options: ExportOptions{Ro: true},
				Clients: []Client{
					{Host: "192.168.1.0/24"},
					{Host: "10.0.0.0/8"},
				},
			},
			contains: []string{"/data/share", "192.168.1.0/24", "10.0.0.0/8", "ro"},
		},
		{
			name: "无客户端",
			export: &Export{
				Path:    "/data/share",
				Options: ExportOptions{Rw: true},
				Clients: []Client{},
			},
			contains: []string{"/data/share", "*"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.formatExportLine(tt.export)
			for _, c := range tt.contains {
				if !strings.Contains(result, c) {
					t.Errorf("结果应包含 '%s'，实际为 %s", c, result)
				}
			}
		})
	}
}

func TestExportOptionsToString(t *testing.T) {
	parser := NewConfigParser()

	tests := []struct {
		name     string
		opts     ExportOptions
		contains []string
	}{
		{
			name:     "读写模式",
			opts:     ExportOptions{Rw: true},
			contains: []string{"rw"},
		},
		{
			name:     "只读模式",
			opts:     ExportOptions{Ro: true},
			contains: []string{"ro"},
		},
		{
			name:     "NoRootSquash",
			opts:     ExportOptions{NoRootSquash: true},
			contains: []string{"no_root_squash"},
		},
		{
			name:     "异步模式",
			opts:     ExportOptions{Async: true},
			contains: []string{"async"},
		},
		{
			name:     "安全模式",
			opts:     ExportOptions{Secure: true},
			contains: []string{"secure"},
		},
		{
			name:     "子树检查",
			opts:     ExportOptions{SubtreeCheck: true},
			contains: []string{"subtree_check"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parser.exportOptionsToString(&tt.opts)
			for _, c := range tt.contains {
				if !strings.Contains(result, c) {
					t.Errorf("结果应包含 '%s'，实际为 %s", c, result)
				}
			}
		})
	}
}

func TestParseFSID(t *testing.T) {
	parser := NewConfigParser()

	tests := []struct {
		opts     []string
		expected int
		wantErr  bool
	}{
		{[]string{"fsid=100"}, 100, false},
		{[]string{"rw", "fsid=5"}, 5, false},
		{[]string{"rw"}, 0, false},
		{[]string{"fsid=invalid"}, 0, true},
	}

	for _, tt := range tests {
		result, err := parser.ParseFSID(tt.opts)
		if (err != nil) != tt.wantErr {
			t.Errorf("ParseFSID(%v) error = %v, wantErr %v", tt.opts, err, tt.wantErr)
			continue
		}
		if !tt.wantErr && result != tt.expected {
			t.Errorf("ParseFSID(%v) = %d, want %d", tt.opts, result, tt.expected)
		}
	}
}

func TestValidateExportsPath(t *testing.T) {
	parser := NewConfigParser()
	tmpDir := t.TempDir()

	// 创建测试目录
	testDir := filepath.Join(tmpDir, "testdir")
	if err := os.MkdirAll(testDir, 0755); err != nil {
		t.Fatal(err)
	}

	// 创建测试文件
	testFile := filepath.Join(tmpDir, "testfile")
	if err := os.WriteFile(testFile, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		path    string
		wantErr bool
	}{
		{testDir, false},
		{filepath.Join(tmpDir, "nonexistent"), false}, // 不存在也可以
		{"relative/path", true},
		{testFile, true}, // 文件不是目录
	}

	for _, tt := range tests {
		err := parser.ValidateExportsPath(tt.path)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateExportsPath(%q) error = %v, wantErr %v", tt.path, err, tt.wantErr)
		}
	}
}

func TestValidateHost(t *testing.T) {
	parser := NewConfigParser()

	tests := []struct {
		host    string
		wantErr bool
	}{
		{"*", false},
		{"192.168.1.1", false},
		{"192.168.1.0/24", false},
		{"host.example.com", false},
		{"@netgroup", false},
		{"", true},
	}

	for _, tt := range tests {
		err := parser.ValidateHost(tt.host)
		if (err != nil) != tt.wantErr {
			t.Errorf("ValidateHost(%q) error = %v, wantErr %v", tt.host, err, tt.wantErr)
		}
	}
}

func TestMergeExports(t *testing.T) {
	parser := NewConfigParser()

	existing := []*Export{
		{Path: "/path1", Comment: "old1"},
		{Path: "/path2", Comment: "old2"},
	}

	newExports := []*Export{
		{Path: "/path2", Comment: "new2"},
		{Path: "/path3", Comment: "new3"},
	}

	result := parser.MergeExports(existing, newExports)

	if len(result) != 3 {
		t.Errorf("合并后数量错误: %d", len(result))
	}

	// 验证 /path2 被更新
	for _, e := range result {
		if e.Path == "/path2" {
			if e.Comment != "new2" {
				t.Errorf("/path2 应该被更新，Comment = %s", e.Comment)
			}
		}
	}
}
