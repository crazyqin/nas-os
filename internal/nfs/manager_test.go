package nfs

import (
	"os"
	"path/filepath"
	"testing"
)

// 测试辅助函数
func setupTestManager(t *testing.T) (*Manager, string) {
	// 创建临时目录
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")

	// 创建 NFS 管理器
	mgr, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("创建 NFS 管理器失败：%v", err)
	}

	return mgr, tmpDir
}

// ========== 配置测试 ==========

func TestDefaultConfig(t *testing.T) {
	mgr, _ := setupTestManager(t)

	if !mgr.config.Enabled {
		t.Error("默认配置应该启用 NFS")
	}
	if mgr.config.Threads != 8 {
		t.Errorf("默认线程数错误：%d", mgr.config.Threads)
	}
	if mgr.config.GracePeriod != 90 {
		t.Errorf("默认宽限期错误：%d", mgr.config.GracePeriod)
	}
}

func TestConfigPersistence(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建导出
	exp, err := mgr.CreateExport(ExportInput{
		Name:    "test-export",
		Path:    filepath.Join(tmpDir, "export"),
		Comment: "Test export",
	})
	if err != nil {
		t.Fatalf("创建导出失败：%v", err)
	}
	exp.ReadOnly = true
	exp.AllowedNetworks = []string{"192.168.1.0/24"}

	// 保存配置
	if err := mgr.saveConfig(); err != nil {
		t.Fatalf("保存配置失败：%v", err)
	}

	// 重新加载配置
	mgr2, err := NewManager(mgr.configPath)
	if err != nil {
		t.Fatalf("重新创建管理器失败：%v", err)
	}

	// 验证导出已加载
	loaded, err := mgr2.GetExport("test-export")
	if err != nil {
		t.Fatalf("加载导出失败：%v", err)
	}
	if loaded.Comment != "Test export" {
		t.Errorf("导出注释错误：%s", loaded.Comment)
	}
	if !loaded.ReadOnly {
		t.Error("导出应该是只读的")
	}
	if len(loaded.AllowedNetworks) != 1 {
		t.Errorf("网络列表错误：%v", loaded.AllowedNetworks)
	}
}

// ========== 导出 CRUD 测试 ==========

func TestCreateExport(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	tests := []struct {
		name    string
		input   ExportInput
		wantErr bool
	}{
		{
			name: "正常创建",
			input: ExportInput{
				Name:    "export1",
				Path:    filepath.Join(tmpDir, "export1"),
				Comment: "Export 1",
			},
			wantErr: false,
		},
		{
			name: "带网络限制创建",
			input: ExportInput{
				Name:            "export2",
				Path:            filepath.Join(tmpDir, "export2"),
				AllowedNetworks: []string{"192.168.1.0/24", "10.0.0.0/8"},
			},
			wantErr: false,
		},
		{
			name: "带主机限制创建",
			input: ExportInput{
				Name:         "export3",
				Path:         filepath.Join(tmpDir, "export3"),
				AllowedHosts: []string{"192.168.1.100", "192.168.1.101"},
			},
			wantErr: false,
		},
		{
			name: "路径自动创建",
			input: ExportInput{
				Name:    "export4",
				Path:    filepath.Join(tmpDir, "newdir", "export4"),
				Comment: "Auto created path",
			},
			wantErr: false,
		},
		{
			name: "重复创建",
			input: ExportInput{
				Name:    "export1",
				Path:    filepath.Join(tmpDir, "export1"),
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, err := mgr.CreateExport(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateExport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if exp.Name != tt.input.Name {
					t.Errorf("导出名称错误：%s", exp.Name)
				}
				if exp.Path != tt.input.Path {
					t.Errorf("导出路径错误：%s", exp.Path)
				}
				// 验证目录已创建
				if _, err := os.Stat(exp.Path); os.IsNotExist(err) {
					t.Error("导出目录未创建")
				}
				// 验证默认值
				if !exp.NoSubtreeCheck {
					t.Error("默认应该禁用子树检查")
				}
			}
		})
	}
}

func TestGetExport(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建测试导出
	_, _ = mgr.CreateExport(ExportInput{
		Name: "existing",
		Path: filepath.Join(tmpDir, "existing"),
	})

	tests := []struct {
		name       string
		exportName string
		wantErr    bool
	}{
		{"获取存在的导出", "existing", false},
		{"获取不存在的导出", "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			exp, err := mgr.GetExport(tt.exportName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetExport() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && exp.Name != tt.exportName {
				t.Errorf("导出名称错误：%s", exp.Name)
			}
		})
	}
}

func TestListExports(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建多个导出
	for i := 1; i <= 3; i++ {
		_, _ = mgr.CreateExport(ExportInput{
			Name: "export" + string(rune('0'+i)),
			Path: filepath.Join(tmpDir, "export"+string(rune('0'+i))),
		})
	}

	exports := mgr.ListExports()
	if len(exports) != 3 {
		t.Errorf("导出数量错误：%d", len(exports))
	}
}

func TestUpdateExport(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建导出
	_, _ = mgr.CreateExport(ExportInput{
		Name:    "update-test",
		Path:    filepath.Join(tmpDir, "update-test"),
		Comment: "Original",
	})

	// 更新导出
	exp, err := mgr.UpdateExport("update-test", ExportInput{
		Comment:         "Updated",
		ReadOnly:        true,
		AllowedNetworks: []string{"10.0.0.0/8"},
	})
	if err != nil {
		t.Fatalf("更新导出失败：%v", err)
	}

	if exp.Comment != "Updated" {
		t.Errorf("注释未更新：%s", exp.Comment)
	}
	if !exp.ReadOnly {
		t.Error("应该设置为只读")
	}
	if len(exp.AllowedNetworks) != 1 {
		t.Errorf("网络列表错误：%v", exp.AllowedNetworks)
	}
}

func TestDeleteExport(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建导出
	mgr.CreateExport(ExportInput{
		Name: "delete-test",
		Path: filepath.Join(tmpDir, "delete-test"),
	})

	// 删除导出
	if err := mgr.DeleteExport("delete-test"); err != nil {
		t.Fatalf("删除导出失败：%v", err)
	}

	// 验证已删除
	_, err := mgr.GetExport("delete-test")
	if err == nil {
		t.Error("导出应该已被删除")
	}

	// 删除不存在的导出
	err = mgr.DeleteExport("nonexistent")
	if err == nil {
		t.Error("删除不存在的导出应该报错")
	}
}

// ========== 配置生成测试 ==========

func TestGenerateExports(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建导出
	mgr.CreateExport(ExportInput{
		Name:            "test-export",
		Path:            filepath.Join(tmpDir, "test-export"),
		ReadOnly:        false,
		AllowedNetworks: []string{"192.168.1.0/24"},
	})

	exports := mgr.generateExports()

	// 验证导出格式
	if !containsStr(exports, "rw") {
		t.Error("配置应该包含读写选项")
	}
	if !containsStr(exports, "no_subtree_check") {
		t.Error("配置应该包含 no_subtree_check")
	}
	if !containsStr(exports, "no_root_squash") {
		t.Error("配置应该包含 no_root_squash")
	}
}

func TestGenerateExportsReadOnly(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建只读导出
	mgr.CreateExport(ExportInput{
		Name:     "readonly-export",
		Path:     filepath.Join(tmpDir, "readonly-export"),
		ReadOnly: true,
	})

	exports := mgr.generateExports()

	if !containsStr(exports, "ro") {
		t.Error("只读导出应该包含 ro 选项")
	}
}

func TestGenerateExportsMultipleClients(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建多客户端导出
	mgr.CreateExport(ExportInput{
		Name:            "multi-export",
		Path:            filepath.Join(tmpDir, "multi-export"),
		AllowedNetworks: []string{"192.168.1.0/24", "10.0.0.0/8"},
		AllowedHosts:    []string{"172.16.0.1"},
	})

	exports := mgr.generateExports()

	// 验证包含所有客户端
	if !containsStr(exports, "192.168.1.0/24") {
		t.Error("配置应该包含网络 192.168.1.0/24")
	}
	if !containsStr(exports, "10.0.0.0/8") {
		t.Error("配置应该包含网络 10.0.0.0/8")
	}
	if !containsStr(exports, "172.16.0.1") {
		t.Error("配置应该包含主机 172.16.0.1")
	}
}

func containsStr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ========== 导出路径测试 ==========

func TestExportPath(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	testPath := filepath.Join(tmpDir, "path-test")
	mgr.CreateExport(ExportInput{
		Name: "path-test",
		Path: testPath,
	})

	path := mgr.ExportPath("path-test")
	if path != testPath {
		t.Errorf("导出路径错误：%s", path)
	}

	// 不存在的导出
	emptyPath := mgr.ExportPath("nonexistent")
	if emptyPath != "" {
		t.Errorf("不存在的导出应该返回空字符串：%s", emptyPath)
	}
}

// ========== 并发安全测试 ==========

func TestConcurrentAccess(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 并发创建导出
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			mgr.CreateExport(ExportInput{
				Name: "concurrent" + string(rune('0'+idx)),
				Path: filepath.Join(tmpDir, "concurrent"+string(rune('0'+idx))),
			})
			done <- true
		}(i)
	}

	// 等待所有 goroutine 完成
	for i := 0; i < 10; i++ {
		<-done
	}

	exports := mgr.ListExports()
	if len(exports) != 10 {
		t.Errorf("并发创建后导出数量错误：%d", len(exports))
	}
}

// ========== 同步/异步选项测试 ==========

func TestSyncOption(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建异步导出（默认）
	mgr.CreateExport(ExportInput{
		Name: "async-export",
		Path: filepath.Join(tmpDir, "async-export"),
	})

	exports := mgr.generateExports()
	if !containsStr(exports, "async") {
		t.Error("默认应该是异步模式")
	}

	// 修改为同步模式
	exp, _ := mgr.GetExport("async-export")
	exp.Sync = true

	exports = mgr.generateExports()
	if !containsStr(exports, "sync") {
		t.Error("应该包含同步选项")
	}
}