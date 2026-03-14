package nfs

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// ========== 测试辅助函数 ==========

func setupTestManager(t *testing.T) (*Manager, string) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")

	mgr, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("创建 NFS 管理器失败：%v", err)
	}

	return mgr, tmpDir
}

// ========== 配置测试 ==========

func TestNewManager(t *testing.T) {
	mgr, _ := setupTestManager(t)

	if mgr == nil {
		t.Fatal("Manager 不应为 nil")
	}
	if mgr.exports == nil {
		t.Error("exports map 不应为 nil")
	}
}

func TestConfigPersistence(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建导出
	export := &Export{
		Path:    filepath.Join(tmpDir, "export"),
		Comment: "Test export",
		Options: ExportOptions{
			Rw:           true,
			NoRootSquash: true,
		},
		Clients: []Client{
			{Host: "192.168.1.0/24"},
		},
	}
	_ = mgr.CreateExport(export)

	// 保存
	if err := mgr.saveConfig(); err != nil {
		t.Fatalf("保存配置失败：%v", err)
	}

	// 重新加载
	mgr2, err := NewManager(mgr.configPath)
	if err != nil {
		t.Fatalf("重新创建管理器失败：%v", err)
	}

	// 验证导出已加载
	loaded, err := mgr2.GetExport(export.Path)
	if err != nil {
		t.Fatalf("加载导出失败：%v", err)
	}
	if loaded.Comment != "Test export" {
		t.Errorf("导出注释错误：%s", loaded.Comment)
	}
}

func TestConfigLoadError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")

	// 创建无效的配置文件
	if err := os.WriteFile(configPath, []byte("invalid json"), 0644); err != nil {
		t.Fatal(err)
	}

	_, err := NewManager(configPath)
	if err == nil {
		t.Error("应该返回解析错误")
	}
}

func TestWriteConfigFile(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "nfs.json")

	// 创建一个完整的管理器来测试写入
	mgr, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("创建管理器失败: %v", err)
	}

	pc := persistentConfig{
		Exports: map[string]*Export{
			"/test": {
				Path:    "/test",
				Comment: "Test Export",
				Options: ExportOptions{Rw: true},
			},
		},
	}

	// 写入配置（目录不存在，应自动创建）
	if err := mgr.writeConfigFile(pc); err != nil {
		t.Fatalf("写入配置失败：%v", err)
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

	var loaded persistentConfig
	if err := json.Unmarshal(data, &loaded); err != nil {
		t.Fatalf("解析配置失败：%v", err)
	}

	if len(loaded.Exports) != 1 {
		t.Error("配置内容不正确")
	}
}

// ========== 导出 CRUD 测试 ==========

func TestCreateExport(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	tests := []struct {
		name    string
		export  *Export
		wantErr bool
		errMsg  string
	}{
		{
			name: "正常创建",
			export: &Export{
				Path:    filepath.Join(tmpDir, "export1"),
				Comment: "Export 1",
			},
			wantErr: false,
		},
		{
			name: "带客户端配置创建",
			export: &Export{
				Path: filepath.Join(tmpDir, "export2"),
				Clients: []Client{
					{Host: "192.168.1.0/24"},
					{Host: "10.0.0.0/8"},
				},
			},
			wantErr: false,
		},
		{
			name: "带选项创建",
			export: &Export{
				Path: filepath.Join(tmpDir, "export3"),
				Options: ExportOptions{
					Ro:           true,
					NoRootSquash: true,
					Async:        true,
				},
			},
			wantErr: false,
		},
		{
			name: "路径自动创建",
			export: &Export{
				Path:    filepath.Join(tmpDir, "newdir", "export4"),
				Comment: "Auto created path",
			},
			wantErr: false,
		},
		{
			name: "重复创建",
			export: &Export{
				Path: filepath.Join(tmpDir, "export1"),
			},
			wantErr: true,
			errMsg:  "导出已存在",
		},
		{
			name:    "nil导出",
			export:  nil,
			wantErr: true,
			errMsg:  "导出配置不能为空",
		},
		{
			name: "空路径",
			export: &Export{
				Comment: "No path",
			},
			wantErr: true,
			errMsg:  "导出路径不能为空",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.CreateExport(tt.export)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateExport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("错误消息不包含 '%s': %v", tt.errMsg, err)
				}
				return
			}

			// 验证导出已创建
			created, err := mgr.GetExport(tt.export.Path)
			if err != nil {
				t.Fatalf("获取创建的导出失败: %v", err)
			}
			if created.Path != tt.export.Path {
				t.Errorf("导出路径错误：%s", created.Path)
			}

			// 验证目录已创建
			if _, err := os.Stat(created.Path); os.IsNotExist(err) {
				t.Error("导出目录未创建")
			}

			// 验证默认值
			if len(created.Clients) == 0 {
				t.Error("应该有默认的客户端配置")
			}
		})
	}
}

func TestGetExport(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建测试导出
	testPath := filepath.Join(tmpDir, "existing")
	_ = mgr.CreateExport(&Export{
		Path:     testPath,
		Comment:  "Existing export",
		Options:  ExportOptions{Ro: true},
	})

	tests := []struct {
		name     string
		path     string
		wantErr  bool
		errMsg   string
	}{
		{name: "获取存在的导出", path: testPath, wantErr: false},
		{name: "获取不存在的导出", path: "/nonexistent", wantErr: true, errMsg: "导出不存在"},
		{name: "空路径", path: "", wantErr: true, errMsg: "导出路径不能为空"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, err := mgr.GetExport(tt.path)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetExport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("错误消息不包含 '%s': %v", tt.errMsg, err)
				}
				return
			}
			if exp.Path != tt.path {
				t.Errorf("导出路径错误：%s", exp.Path)
			}
		})
	}
}

func TestListExports(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 初始应该为空
	exports, err := mgr.ListExports()
	if err != nil {
		t.Fatalf("ListExports 失败: %v", err)
	}
	if len(exports) != 0 {
		t.Errorf("初始导出列表应该为空: %d", len(exports))
	}

	// 创建多个导出
	paths := []string{
		filepath.Join(tmpDir, "export1"),
		filepath.Join(tmpDir, "export2"),
		filepath.Join(tmpDir, "export3"),
	}
	for _, path := range paths {
		_ = mgr.CreateExport(&Export{Path: path})
	}

	exports, err = mgr.ListExports()
	if err != nil {
		t.Fatalf("ListExports 失败: %v", err)
	}
	if len(exports) != 3 {
		t.Errorf("导出数量错误：%d", len(exports))
	}

	// 验证所有导出都在列表中
	exportMap := make(map[string]bool)
	for _, e := range exports {
		exportMap[e.Path] = true
	}
	for _, path := range paths {
		if !exportMap[path] {
			t.Errorf("导出 %s 不在列表中", path)
		}
	}
}

func TestUpdateExport(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建导出
	testPath := filepath.Join(tmpDir, "update-test")
	_ = mgr.CreateExport(&Export{
		Path:    testPath,
		Comment: "Original",
	})

	tests := []struct {
		name    string
		path    string
		export  *Export
		wantErr bool
	}{
		{
			name: "更新存在的导出",
			path: testPath,
			export: &Export{
				Comment: "Updated",
				Options: ExportOptions{
					Ro:           true,
					NoRootSquash: true,
				},
				Clients: []Client{
					{Host: "10.0.0.0/8"},
				},
			},
			wantErr: false,
		},
		{
			name:    "更新不存在的导出",
			path:    "/nonexistent",
			export:  &Export{Comment: "Test"},
			wantErr: true,
		},
		{
			name:    "nil导出",
			path:    testPath,
			export:  nil,
			wantErr: true,
		},
		{
			name:    "空路径",
			path:    "",
			export:  &Export{},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.UpdateExport(tt.path, tt.export)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateExport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			exp, _ := mgr.GetExport(tt.path)
			if exp.Comment != tt.export.Comment {
				t.Errorf("注释未更新：%s", exp.Comment)
			}
		})
	}
}

func TestDeleteExport(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建导出
	testPath := filepath.Join(tmpDir, "delete-test")
	_ = mgr.CreateExport(&Export{Path: testPath})

	// 删除存在的导出
	if err := mgr.DeleteExport(testPath); err != nil {
		t.Fatalf("删除导出失败：%v", err)
	}

	// 验证已删除
	_, err := mgr.GetExport(testPath)
	if err == nil {
		t.Error("导出应该已被删除")
	}

	// 删除不存在的导出
	err = mgr.DeleteExport("/nonexistent")
	if err == nil {
		t.Error("删除不存在的导出应该报错")
	}
	if !strings.Contains(err.Error(), "导出不存在") {
		t.Errorf("错误消息不正确: %v", err)
	}

	// 删除空路径
	err = mgr.DeleteExport("")
	if err == nil {
		t.Error("删除空路径应该报错")
	}
}

// ========== 配置生成测试 ==========

func TestGenerateExportsFile(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建导出
	_ = mgr.CreateExport(&Export{
		Path: filepath.Join(tmpDir, "test-export"),
		Options: ExportOptions{
			Rw:           true,
			NoRootSquash: true,
		},
		Clients: []Client{
			{Host: "192.168.1.0/24"},
		},
	})

	exports, err := mgr.GenerateExportsFile()
	if err != nil {
		t.Fatalf("GenerateExportsFile 失败: %v", err)
	}

	// 验证导出格式
	tests := []struct {
		name    string
		contain string
	}{
		{"读写选项", "rw"},
		{"root_squash选项", "no_root_squash"},
		{"同步选项", "sync"},
		{"子树检查", "no_subtree_check"},
		{"客户端", "192.168.1.0/24"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(exports, tt.contain) {
				t.Errorf("配置应该包含 '%s'", tt.contain)
			}
		})
	}
}

func TestGenerateExportsFileReadOnly(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建只读导出
	_ = mgr.CreateExport(&Export{
		Path:    filepath.Join(tmpDir, "readonly-export"),
		Options: ExportOptions{Ro: true},
	})

	exports, err := mgr.GenerateExportsFile()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(exports, "ro") {
		t.Error("只读导出应该包含 ro 选项")
	}
}

func TestGenerateExportsFileMultipleClients(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建多客户端导出
	_ = mgr.CreateExport(&Export{
		Path: filepath.Join(tmpDir, "multi-export"),
		Clients: []Client{
			{Host: "192.168.1.0/24"},
			{Host: "10.0.0.0/8"},
			{Host: "172.16.0.1"},
		},
	})

	exports, err := mgr.GenerateExportsFile()
	if err != nil {
		t.Fatal(err)
	}

	// 验证包含所有客户端
	for _, host := range []string{"192.168.1.0/24", "10.0.0.0/8", "172.16.0.1"} {
		if !strings.Contains(exports, host) {
			t.Errorf("配置应该包含客户端 '%s'", host)
		}
	}
}

func TestGenerateExportsFileEmptyClients(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建无客户端限制的导出
	_ = mgr.CreateExport(&Export{
		Path: filepath.Join(tmpDir, "open-export"),
	})

	exports, err := mgr.GenerateExportsFile()
	if err != nil {
		t.Fatal(err)
	}

	// 无客户端限制时应该允许所有
	if !strings.Contains(exports, "*") {
		t.Error("无客户端限制时应该允许所有")
	}
}

func TestGenerateExportsFileAsync(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建异步导出
	_ = mgr.CreateExport(&Export{
		Path:    filepath.Join(tmpDir, "async-export"),
		Options: ExportOptions{Async: true},
	})

	exports, err := mgr.GenerateExportsFile()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(exports, "async") {
		t.Error("应该包含异步选项")
	}
}

// ========== 选项转换测试 ==========

func TestOptionsToString(t *testing.T) {
	mgr, _ := setupTestManager(t)

	tests := []struct {
		name        string
		options     ExportOptions
		shouldHave  []string
		shouldNot   []string
	}{
		{
			name:       "读写模式",
			options:    ExportOptions{Rw: true},
			shouldHave: []string{"rw"},
			shouldNot:  []string{}, // 其他选项根据默认值生成，只验证 rw 存在且 ro 不存在
		},
		{
			name:       "只读模式",
			options:    ExportOptions{Ro: true},
			shouldHave: []string{"ro"},
			shouldNot:  []string{"rw"},
		},
		{
			name:       "NoRootSquash",
			options:    ExportOptions{NoRootSquash: true},
			shouldHave: []string{"no_root_squash"},
			shouldNot:  []string{}, // 注意：root_squash 是 no_root_squash 的子串，不能检查
		},
		{
			name:       "RootSquash默认",
			options:    ExportOptions{},
			shouldHave: []string{"root_squash"},
			shouldNot:  []string{"no_root_squash"},
		},
		{
			name:       "异步模式",
			options:    ExportOptions{Async: true},
			shouldHave: []string{"async"},
			shouldNot:  []string{}, // 注意：sync 是 async 的子串，不能检查
		},
		{
			name:       "同步模式默认",
			options:    ExportOptions{},
			shouldHave: []string{"sync"},
			shouldNot:  []string{"async"},
		},
		{
			name:       "安全选项",
			options:    ExportOptions{Secure: true},
			shouldHave: []string{"secure"},
		},
		{
			name:       "子树检查",
			options:    ExportOptions{SubtreeCheck: true},
			shouldHave: []string{"subtree_check"},
			shouldNot:  []string{"no_subtree_check"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := mgr.optionsToString(&tt.options)

			for _, s := range tt.shouldHave {
				if !strings.Contains(result, s) {
					t.Errorf("结果应包含 '%s'，实际为 %s", s, result)
				}
			}

			for _, s := range tt.shouldNot {
				if strings.Contains(result, s) {
					t.Errorf("结果不应包含 '%s'，实际为 %s", s, result)
				}
			}
		})
	}
}

// ========== 并发安全测试 ==========

func TestConcurrentCreateExport(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	var wg sync.WaitGroup
	errCount := int32(0)
	successCount := int32(0)

	testPath := filepath.Join(tmpDir, "concurrent")

	// 并发创建 50 个同名导出
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			err := mgr.CreateExport(&Export{Path: testPath})
			if err != nil {
				errCount++
			} else {
				successCount++
			}
		}()
	}

	wg.Wait()

	// 只有一个应该成功
	if successCount != 1 {
		t.Errorf("应该只有一个创建成功，实际: %d", successCount)
	}
	if errCount != 49 {
		t.Errorf("应该有 49 个创建失败，实际: %d", errCount)
	}
}

func TestConcurrentReadWrite(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建导出
	testPath := filepath.Join(tmpDir, "rw-test")
	_ = mgr.CreateExport(&Export{Path: testPath})

	var wg sync.WaitGroup

	// 并发读取
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.GetExport(testPath)
			mgr.ListExports()
		}()
	}

	// 并发更新
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.UpdateExport(testPath, &Export{Comment: "updated"})
		}()
	}

	wg.Wait()

	// 验证最终状态
	exp, err := mgr.GetExport(testPath)
	if err != nil {
		t.Fatal(err)
	}
	if exp.Comment != "updated" {
		t.Error("最终状态应该是最新的更新")
	}
}

func TestConcurrentDeleteCreate(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	var wg sync.WaitGroup

	testPath := filepath.Join(tmpDir, "cycle")

	// 循环删除和创建
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.DeleteExport(testPath)
			mgr.CreateExport(&Export{Path: testPath})
		}()
	}

	wg.Wait()

	// 最终应该存在一个导出
	_, err := mgr.GetExport(testPath)
	if err != nil {
		t.Error("最终应该存在一个导出")
	}
}

// ========== 验证测试 ==========

func TestValidateExport(t *testing.T) {
	mgr, _ := setupTestManager(t)

	tests := []struct {
		name    string
		export  *Export
		wantErr bool
		errMsg  string
	}{
		{
			name: "有效导出",
			export: &Export{
				Path: "/valid/path",
			},
			wantErr: false,
		},
		{
			name:    "nil导出",
			export:  nil,
			wantErr: true,
			errMsg:  "导出配置不能为空",
		},
		{
			name:    "空路径",
			export:  &Export{},
			wantErr: true,
			errMsg:  "导出路径不能为空",
		},
		{
			name: "相对路径",
			export: &Export{
				Path: "relative/path",
			},
			wantErr: true,
			errMsg:  "必须是绝对路径",
		},
		{
			name: "选项冲突",
			export: &Export{
				Path: "/conflict",
				Options: ExportOptions{
					Ro: true,
					Rw: true,
				},
			},
			wantErr: true,
			errMsg:  "不能同时设置只读和读写",
		},
		{
			name: "空客户端主机",
			export: &Export{
				Path: "/empty-client",
				Clients: []Client{
					{Host: ""},
				},
			},
			wantErr: true,
			errMsg:  "客户端主机不能为空",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.ValidateExport(tt.export)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateExport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("错误消息不包含 '%s': %v", tt.errMsg, err)
				}
			}
		})
	}
}

// ========== 边界情况测试 ==========

func TestEmptyExports(t *testing.T) {
	mgr, _ := setupTestManager(t)

	// 空导出列表
	exports, err := mgr.ListExports()
	if err != nil {
		t.Fatal(err)
	}
	if len(exports) != 0 {
		t.Errorf("应该为空: %d", len(exports))
	}

	// 空配置生成
	exportsFile, err := mgr.GenerateExportsFile()
	if err != nil {
		t.Fatal(err)
	}
	if exportsFile != "" {
		t.Errorf("空配置应该生成空字符串: %s", exportsFile)
	}
}

// ========== 配置不存在的情况 ==========

func TestNewManagerNoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent", "nfs.json")

	mgr, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("应该成功创建: %v", err)
	}

	// 应该有空的 exports
	if len(mgr.exports) != 0 {
		t.Error("应该为空配置")
	}
}

// ========== 客户端选项测试 ==========

func TestClientOptions(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建带客户端选项的导出
	_ = mgr.CreateExport(&Export{
		Path: filepath.Join(tmpDir, "client-opts"),
		Clients: []Client{
			{
				Host:    "192.168.1.100",
				Options: []string{"rw", "no_root_squash"},
			},
		},
	})

	exports, err := mgr.GenerateExportsFile()
	if err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(exports, "192.168.1.100") {
		t.Error("应该包含客户端主机")
	}
}

// ========== 默认选项测试 ==========

func TestApplyDefaultOptions(t *testing.T) {
	mgr, _ := setupTestManager(t)

	export := &Export{
		Path: "/test/defaults",
	}

	mgr.applyDefaultOptions(export)

	// 应该有默认客户端
	if len(export.Clients) != 1 {
		t.Error("应该有默认客户端")
	}
	if export.Clients[0].Host != "*" {
		t.Error("默认客户端应该是 *")
	}

	// 应该有默认读写权限
	if !export.Options.Rw {
		t.Error("默认应该是读写模式")
	}
}

// ========== 导出路径变更测试 ==========

func TestUpdateExportChangePath(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建导出
	oldPath := filepath.Join(tmpDir, "old-path")
	_ = mgr.CreateExport(&Export{
		Path:    oldPath,
		Comment: "Old",
	})

	// 更新路径
	newPath := filepath.Join(tmpDir, "new-path")
	err := mgr.UpdateExport(oldPath, &Export{
		Path:    newPath,
		Comment: "New",
	})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	// 旧路径应该不存在
	_, err = mgr.GetExport(oldPath)
	if err == nil {
		t.Error("旧路径应该已删除")
	}

	// 新路径应该存在
	exp, err := mgr.GetExport(newPath)
	if err != nil {
		t.Fatalf("新路径应该存在: %v", err)
	}
	if exp.Comment != "New" {
		t.Error("注释应该已更新")
	}
}