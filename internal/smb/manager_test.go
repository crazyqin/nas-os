package smb

import (
	"os"
	"path/filepath"
	"testing"

	"nas-os/internal/users"
)

// 测试辅助函数
func setupTestManager(t *testing.T) (*Manager, string) {
	// 创建临时目录
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")

	// 创建用户管理器
	userMgr, err := users.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("创建用户管理器失败：%v", err)
	}

	// 创建 SMB 管理器
	mgr, err := NewManager(userMgr, configPath)
	if err != nil {
		t.Fatalf("创建 SMB 管理器失败：%v", err)
	}

	return mgr, tmpDir
}

// ========== 配置测试 ==========

func TestDefaultConfig(t *testing.T) {
	mgr, _ := setupTestManager(t)

	if !mgr.config.Enabled {
		t.Error("默认配置应该启用 SMB")
	}
	if mgr.config.Workgroup != "WORKGROUP" {
		t.Errorf("默认工作组错误：%s", mgr.config.Workgroup)
	}
	if mgr.config.MinProtocol != "SMB2" {
		t.Errorf("默认最小协议错误：%s", mgr.config.MinProtocol)
	}
}

func TestConfigPersistence(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享
	share, err := mgr.CreateShare(ShareInput{
		Name:    "test-share",
		Path:    filepath.Join(tmpDir, "share"),
		Comment: "Test share",
	})
	if err != nil {
		t.Fatalf("创建共享失败：%v", err)
	}
	share.ReadOnly = true
	share.GuestAccess = true

	// 保存配置
	if err := mgr.saveConfig(); err != nil {
		t.Fatalf("保存配置失败：%v", err)
	}

	// 重新加载配置
	userMgr, _ := users.NewManager(tmpDir)
	mgr2, err := NewManager(userMgr, mgr.configPath)
	if err != nil {
		t.Fatalf("重新创建管理器失败：%v", err)
	}

	// 验证共享已加载
	loaded, err := mgr2.GetShare("test-share")
	if err != nil {
		t.Fatalf("加载共享失败：%v", err)
	}
	if loaded.Comment != "Test share" {
		t.Errorf("共享注释错误：%s", loaded.Comment)
	}
	if !loaded.ReadOnly {
		t.Error("共享应该是只读的")
	}
	if !loaded.GuestAccess {
		t.Error("共享应该允许访客访问")
	}
}

// ========== 共享 CRUD 测试 ==========

func TestCreateShare(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	tests := []struct {
		name    string
		input   ShareInput
		wantErr bool
	}{
		{
			name: "正常创建",
			input: ShareInput{
				Name:    "share1",
				Path:    filepath.Join(tmpDir, "share1"),
				Comment: "Share 1",
			},
			wantErr: false,
		},
		{
			name: "路径自动创建",
			input: ShareInput{
				Name:    "share2",
				Path:    filepath.Join(tmpDir, "newdir", "share2"),
				Comment: "Auto created path",
			},
			wantErr: false,
		},
		{
			name: "重复创建",
			input: ShareInput{
				Name:    "share1",
				Path:    filepath.Join(tmpDir, "share1"),
				Comment: "Duplicate",
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			share, err := mgr.CreateShare(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateShare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr {
				if share.Name != tt.input.Name {
					t.Errorf("共享名称错误：%s", share.Name)
				}
				if share.Path != tt.input.Path {
					t.Errorf("共享路径错误：%s", share.Path)
				}
				// 验证目录已创建
				if _, err := os.Stat(share.Path); os.IsNotExist(err) {
					t.Error("共享目录未创建")
				}
			}
		})
	}
}

func TestGetShare(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建测试共享
	mgr.CreateShare(ShareInput{
		Name: "existing",
		Path: filepath.Join(tmpDir, "existing"),
	})

	tests := []struct {
		name      string
		shareName string
		wantErr   bool
	}{
		{"获取存在的共享", "existing", false},
		{"获取不存在的共享", "nonexistent", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			share, err := mgr.GetShare(tt.shareName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetShare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && share.Name != tt.shareName {
				t.Errorf("共享名称错误：%s", share.Name)
			}
		})
	}
}

func TestListShares(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建多个共享
	for i := 1; i <= 3; i++ {
		mgr.CreateShare(ShareInput{
			Name:    "share" + string(rune('0'+i)),
			Path:    filepath.Join(tmpDir, "share"+string(rune('0'+i))),
			Comment: "Share",
		})
	}

	shares := mgr.ListShares()
	if len(shares) != 3 {
		t.Errorf("共享数量错误：%d", len(shares))
	}
}

func TestUpdateShare(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享
	mgr.CreateShare(ShareInput{
		Name:    "update-test",
		Path:    filepath.Join(tmpDir, "update-test"),
		Comment: "Original",
	})

	// 更新共享
	share, err := mgr.UpdateShare("update-test", ShareInput{
		Comment:     "Updated",
		ReadOnly:    true,
		GuestAccess: true,
	})
	if err != nil {
		t.Fatalf("更新共享失败：%v", err)
	}

	if share.Comment != "Updated" {
		t.Errorf("注释未更新：%s", share.Comment)
	}
	if !share.ReadOnly {
		t.Error("应该设置为只读")
	}
	if !share.GuestAccess {
		t.Error("应该允许访客访问")
	}
}

func TestDeleteShare(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享
	mgr.CreateShare(ShareInput{
		Name: "delete-test",
		Path: filepath.Join(tmpDir, "delete-test"),
	})

	// 删除共享
	if err := mgr.DeleteShare("delete-test"); err != nil {
		t.Fatalf("删除共享失败：%v", err)
	}

	// 验证已删除
	_, err := mgr.GetShare("delete-test")
	if err == nil {
		t.Error("共享应该已被删除")
	}

	// 删除不存在的共享
	err = mgr.DeleteShare("nonexistent")
	if err == nil {
		t.Error("删除不存在的共享应该报错")
	}
}

// ========== 权限管理测试 ==========

func TestSetSharePermission(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享
	mgr.CreateShare(ShareInput{
		Name: "perm-test",
		Path: filepath.Join(tmpDir, "perm-test"),
	})

	// 设置权限（用户 admin 存在于默认用户管理器中）
	err := mgr.SetSharePermission("perm-test", "admin", true)
	if err != nil {
		t.Fatalf("设置权限失败：%v", err)
	}

	share, _ := mgr.GetShare("perm-test")
	found := false
	for _, u := range share.AllowedUsers {
		if u == "admin" {
			found = true
			break
		}
	}
	if !found {
		t.Error("用户应该被添加到允许列表")
	}
}

func TestRemoveSharePermission(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享并添加权限
	mgr.CreateShare(ShareInput{
		Name:         "remove-perm-test",
		Path:         filepath.Join(tmpDir, "remove-perm-test"),
		AllowedUsers: []string{"admin"},
	})

	// 移除权限
	if err := mgr.RemoveSharePermission("remove-perm-test", "admin"); err != nil {
		t.Fatalf("移除权限失败：%v", err)
	}

	share, _ := mgr.GetShare("remove-perm-test")
	if len(share.AllowedUsers) != 0 {
		t.Errorf("允许列表应该为空：%v", share.AllowedUsers)
	}
}

func TestGetUserShares(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建多个共享
	mgr.CreateShare(ShareInput{
		Name:        "public",
		Path:        filepath.Join(tmpDir, "public"),
		GuestAccess: true,
	})
	mgr.CreateShare(ShareInput{
		Name:         "private",
		Path:         filepath.Join(tmpDir, "private"),
		AllowedUsers: []string{"admin"},
	})

	// 获取用户可见的共享
	shares := mgr.GetUserShares("admin")
	if len(shares) != 2 {
		t.Errorf("管理员应该能看到所有共享：%d", len(shares))
	}
}

// ========== 配置生成测试 ==========

func TestGenerateSmbConf(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享
	mgr.CreateShare(ShareInput{
		Name:        "test-share",
		Path:        filepath.Join(tmpDir, "test-share"),
		Comment:     "Test Share",
		ReadOnly:    false,
		GuestAccess: true,
		Browseable:  true,
	})

	config := mgr.generateSmbConf()

	// 验证全局配置
	if !contains(config, "workgroup = WORKGROUP") {
		t.Error("配置应该包含工作组设置")
	}
	if !contains(config, "[test-share]") {
		t.Error("配置应该包含共享定义")
	}
	if !contains(config, "guest ok = true") {
		t.Error("配置应该包含访客设置")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// ========== 并发安全测试 ==========

func TestConcurrentAccess(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 并发创建共享
	done := make(chan bool)
	for i := 0; i < 10; i++ {
		go func(idx int) {
			mgr.CreateShare(ShareInput{
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

	shares := mgr.ListShares()
	if len(shares) != 10 {
		t.Errorf("并发创建后共享数量错误：%d", len(shares))
	}
}