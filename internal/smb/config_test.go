package smb

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// ========== Config Parser 测试 ==========

func TestParseSmbConf(t *testing.T) {
	// 创建测试配置文件
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "smb.conf")

	configContent := `[global]
workgroup = TESTGROUP
server string = Test Server
security = user
min protocol = SMB2
max protocol = SMB3

[share1]
path = /data/share1
comment = Test Share 1
browseable = yes
read only = no
guest ok = no
valid users = admin, user1
create mask = 0644

[share2]
path = /data/share2
comment = Test Share 2
browseable = no
read only = yes
guest ok = yes
veto files = /.DS_Store/Thumbs.db/
`
	if err := os.WriteFile(configPath, []byte(configContent), 0644); err != nil {
		t.Fatal(err)
	}

	parser := NewConfigParser(configPath)
	shares, config, err := parser.ParseSmbConf()
	if err != nil {
		t.Fatalf("解析失败: %v", err)
	}

	// 验证全局配置
	if config.Workgroup != "TESTGROUP" {
		t.Errorf("Workgroup 错误: %s", config.Workgroup)
	}
	if config.ServerString != "Test Server" {
		t.Errorf("ServerString 错误: %s", config.ServerString)
	}

	// 验证共享
	if len(shares) != 2 {
		t.Errorf("共享数量错误: %d", len(shares))
	}

	share1, exists := shares["share1"]
	if !exists {
		t.Fatal("share1 不存在")
	}
	if share1.Path != "/data/share1" {
		t.Errorf("share1 Path 错误: %s", share1.Path)
	}
	if share1.Comment != "Test Share 1" {
		t.Errorf("share1 Comment 错误: %s", share1.Comment)
	}
	if !share1.Browseable {
		t.Error("share1 应该是 browseable")
	}
	if share1.ReadOnly {
		t.Error("share1 不应该是 read only")
	}
	if len(share1.ValidUsers) != 2 {
		t.Errorf("share1 ValidUsers 错误: %v", share1.ValidUsers)
	}
}

func TestParseSmbConfFileNotExist(t *testing.T) {
	parser := NewConfigParser("/nonexistent/smb.conf")
	_, _, err := parser.ParseSmbConf()
	if err == nil {
		t.Error("文件不存在应该返回错误")
	}
}

func TestParseUserList(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"admin, user1, user2", 3},
		{"admin", 1},
		{"", 0},
		{"@group, admin", 1}, // @group 被过滤
		{"  admin  ,  user1  ", 2},
	}

	for _, tt := range tests {
		result := parseUserList(tt.input)
		if len(result) != tt.expected {
			t.Errorf("parseUserList(%q) = %d, expected %d", tt.input, len(result), tt.expected)
		}
	}
}

func TestParseVetoFiles(t *testing.T) {
	tests := []struct {
		input    string
		expected int
	}{
		{"/.DS_Store/Thumbs.db/", 2},
		{"/file1/", 1},
		{"", 0},
		{"/", 0},
	}

	for _, tt := range tests {
		result := parseVetoFiles(tt.input)
		if len(result) != tt.expected {
			t.Errorf("parseVetoFiles(%q) = %d, expected %d", tt.input, len(result), tt.expected)
		}
	}
}

func TestGenerateSmbConf(t *testing.T) {
	config := &Config{
		Workgroup:    "WORKGROUP",
		ServerString: "Test Server",
		MinProtocol:  "SMB2",
		MaxProtocol:  "SMB3",
	}

	shares := map[string]*Share{
		"test": {
			Name:          "test",
			Path:          "/data/test",
			Comment:       "Test Share",
			Browseable:    true,
			ReadOnly:      false,
			GuestOK:       false,
			ValidUsers:    []string{"admin"},
			CreateMask:    "0644",
			DirectoryMask: "0755",
		},
	}

	result := GenerateSmbConf(config, shares)

	// 验证全局配置
	if !strings.Contains(result, "workgroup = WORKGROUP") {
		t.Error("应该包含 workgroup")
	}
	if !strings.Contains(result, "[test]") {
		t.Error("应该包含共享定义")
	}
	if !strings.Contains(result, "path = /data/test") {
		t.Error("应该包含共享路径")
	}
}

func TestBoolToYesNo(t *testing.T) {
	if boolToYesNo(true) != "yes" {
		t.Error("true 应该返回 yes")
	}
	if boolToYesNo(false) != "no" {
		t.Error("false 应该返回 no")
	}
}

func TestValidateShareConfig(t *testing.T) {
	tests := []struct {
		name    string
		share   *Share
		wantErr bool
	}{
		{
			name: "有效配置",
			share: &Share{
				Name:       "test",
				Path:       "/data/test",
				CreateMask: "0644",
			},
			wantErr: false,
		},
		{
			name:    "空名称",
			share:   &Share{Path: "/data/test"},
			wantErr: true,
		},
		{
			name:    "空路径",
			share:   &Share{Name: "test"},
			wantErr: true,
		},
		{
			name: "相对路径",
			share: &Share{
				Name: "test",
				Path: "relative/path",
			},
			wantErr: true,
		},
		{
			name: "无效 create mask",
			share: &Share{
				Name:       "test",
				Path:       "/data/test",
				CreateMask: "9999",
			},
			wantErr: true,
		},
		{
			name: "mask 长度错误",
			share: &Share{
				Name:       "test",
				Path:       "/data/test",
				CreateMask: "644",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateShareConfig(tt.share)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateShareConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestValidateConfig(t *testing.T) {
	tests := []struct {
		name    string
		config  *Config
		wantErr bool
	}{
		{
			name:    "默认配置",
			config:  &Config{},
			wantErr: false,
		},
		{
			name: "有效配置",
			config: &Config{
				Workgroup:   "WORKGROUP",
				MinProtocol: "SMB2",
				MaxProtocol: "SMB3",
			},
			wantErr: false,
		},
		{
			name: "workgroup 太长",
			config: &Config{
				Workgroup: "VERYLONGWORKGROUPNAME",
			},
			wantErr: true,
		},
		{
			name: "无效协议",
			config: &Config{
				MinProtocol: "INVALID",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateConfig(tt.config)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateConfig() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestWriteSmbConf(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "smb.conf")

	config := &Config{Workgroup: "TEST"}
	shares := map[string]*Share{
		"test": {Name: "test", Path: "/data/test"},
	}

	err := WriteSmbConf(configPath, config, shares)
	if err != nil {
		t.Fatalf("WriteSmbConf 失败: %v", err)
	}

	// 验证文件已创建
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Error("配置文件未创建")
	}

	// 验证内容
	data, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "workgroup = TEST") {
		t.Error("配置文件内容不正确")
	}
}

func TestBackupSmbConf(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "smb.conf")

	// 文件不存在时应该成功
	err := BackupSmbConf(configPath)
	if err != nil {
		t.Errorf("文件不存在时应该成功: %v", err)
	}

	// 创建文件
	if err := os.WriteFile(configPath, []byte("test"), 0644); err != nil {
		t.Fatal(err)
	}

	// 备份
	err = BackupSmbConf(configPath)
	if err != nil {
		t.Fatalf("BackupSmbConf 失败: %v", err)
	}

	// 验证备份文件
	backupPath := configPath + ".bak"
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		t.Error("备份文件未创建")
	}
}

func TestValidateOctalMask(t *testing.T) {
	tests := []struct {
		mask    string
		wantErr bool
	}{
		{"0644", false},
		{"0755", false},
		{"0777", false},
		{"0000", false},
		{"644", true},   // 长度错误
		{"0844", true},  // 非八进制
		{"abcd", true},  // 非数字
		{"", true},      // 空
	}

	for _, tt := range tests {
		err := validateOctalMask(tt.mask)
		if (err != nil) != tt.wantErr {
			t.Errorf("validateOctalMask(%q) error = %v, wantErr %v", tt.mask, err, tt.wantErr)
		}
	}
}