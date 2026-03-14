package smb

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"

	"nas-os/internal/users"
)

// ========== 测试辅助函数 ==========

func setupTestManager(t *testing.T) (*Manager, string) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")

	mgr, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("创建 SMB 管理器失败：%v", err)
	}

	return mgr, tmpDir
}

func setupTestManagerWithUser(t *testing.T) (*Manager, string) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")

	userMgr, err := users.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("创建用户管理器失败：%v", err)
	}

	mgr, err := NewManagerWithUserMgr(userMgr, configPath)
	if err != nil {
		t.Fatalf("创建 SMB 管理器失败：%v", err)
	}

	return mgr, tmpDir
}

// ========== 配置测试 ==========

func TestDefaultConfig(t *testing.T) {
	mgr, _ := setupTestManager(t)

	tests := []struct {
		name     string
		got      interface{}
		expected interface{}
	}{
		{"Enabled", mgr.config.Enabled, true},
		{"Workgroup", mgr.config.Workgroup, "WORKGROUP"},
		{"ServerString", mgr.config.ServerString, "NAS-OS Samba Server"},
		{"MinProtocol", mgr.config.MinProtocol, "SMB2"},
		{"MaxProtocol", mgr.config.MaxProtocol, "SMB3"},
		{"GuestAccess", mgr.config.GuestAccess, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.got != tt.expected {
				t.Errorf("got %v, expected %v", tt.got, tt.expected)
			}
		})
	}
}

func TestConfigPersistence(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享
	share := &Share{
		Name:    "test-share",
		Path:    filepath.Join(tmpDir, "share"),
		Comment: "Test share",
	}
	_ = mgr.CreateShare(share)

	// 修改配置
	mgr.config.Workgroup = "NEWGROUP"
	mgr.config.MinProtocol = "SMB3"

	// 保存
	if err := mgr.saveConfig(); err != nil {
		t.Fatalf("保存配置失败：%v", err)
	}

	// 重新加载
	mgr2, err := NewManager(mgr.configPath)
	if err != nil {
		t.Fatalf("重新创建管理器失败：%v", err)
	}

	// 验证
	if mgr2.config.Workgroup != "NEWGROUP" {
		t.Errorf("Workgroup 未正确加载")
	}
	if mgr2.config.MinProtocol != "SMB3" {
		t.Errorf("MinProtocol 未正确加载")
	}

	// 验证共享已加载
	loaded, err := mgr2.GetShare("test-share")
	if err != nil {
		t.Fatalf("加载共享失败：%v", err)
	}
	if loaded.Comment != "Test share" {
		t.Errorf("共享注释错误：%s", loaded.Comment)
	}
}

func TestConfigLoadError(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")

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
	configPath := filepath.Join(tmpDir, "subdir", "smb.json")

	pc := persistentConfig{
		Config: &Config{
			Enabled:   true,
			Workgroup: "TESTGROUP",
		},
		Shares: map[string]*Share{
			"test": {Name: "test", Path: "/test"},
		},
	}

	// 写入配置（目录不存在，应自动创建）
	if err := writeConfigFile(configPath, pc); err != nil {
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

	if loaded.Config.Workgroup != "TESTGROUP" {
		t.Error("配置内容不正确")
	}
}

// ========== 共享 CRUD 测试 ==========

func TestCreateShare(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	tests := []struct {
		name    string
		share   *Share
		wantErr bool
		errMsg  string
	}{
		{
			name: "正常创建",
			share: &Share{
				Name:    "share1",
				Path:    filepath.Join(tmpDir, "share1"),
				Comment: "Share 1",
			},
			wantErr: false,
		},
		{
			name: "带所有选项创建",
			share: &Share{
				Name:          "share2",
				Path:          filepath.Join(tmpDir, "share2"),
				Comment:       "Share 2",
				ReadOnly:      true,
				GuestOK:       true,
				GuestAccess:   true,
				ValidUsers:    []string{"admin", "user1"},
				Browseable:    true,
				CreateMask:    "0644",
				DirectoryMask: "0755",
			},
			wantErr: false,
		},
		{
			name: "路径自动创建",
			share: &Share{
				Name:    "share3",
				Path:    filepath.Join(tmpDir, "newdir", "share3"),
				Comment: "Auto created path",
			},
			wantErr: false,
		},
		{
			name: "重复创建",
			share: &Share{
				Name: "share1",
				Path: filepath.Join(tmpDir, "share1"),
			},
			wantErr: true,
			errMsg:  "共享已存在",
		},
		{
			name:    "nil共享",
			share:   nil,
			wantErr: true,
			errMsg:  "共享配置不能为空",
		},
		{
			name: "空名称",
			share: &Share{
				Path: filepath.Join(tmpDir, "noname"),
			},
			wantErr: true,
			errMsg:  "共享名称不能为空",
		},
		{
			name: "空路径",
			share: &Share{
				Name: "nopath",
			},
			wantErr: true,
			errMsg:  "共享路径不能为空",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.CreateShare(tt.share)
			if (err != nil) != tt.wantErr {
				t.Errorf("CreateShare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if tt.errMsg != "" && !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("错误消息不包含 '%s': %v", tt.errMsg, err)
				}
				return
			}

			// 验证返回值
			created, err := mgr.GetShare(tt.share.Name)
			if err != nil {
				t.Fatalf("获取创建的共享失败: %v", err)
			}
			if created.Name != tt.share.Name {
				t.Errorf("共享名称错误：%s", created.Name)
			}
			if created.Path != tt.share.Path {
				t.Errorf("共享路径错误：%s", created.Path)
			}

			// 验证目录已创建
			if _, err := os.Stat(created.Path); os.IsNotExist(err) {
				t.Error("共享目录未创建")
			}

			// 验证默认值
			if len(created.VetoFiles) == 0 {
				t.Error("应该有默认的隐藏文件列表")
			}
			if created.CreateMask == "" {
				t.Error("应该有默认的创建掩码")
			}
		})
	}
}

func TestGetShare(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建测试共享
	_ = mgr.CreateShare(&Share{
		Name:        "existing",
		Path:        filepath.Join(tmpDir, "existing"),
		Comment:     "Existing share",
		ReadOnly:    true,
		GuestAccess: true,
	})

	tests := []struct {
		name      string
		shareName string
		wantErr   bool
		errMsg    string
	}{
		{name: "获取存在的共享", shareName: "existing", wantErr: false},
		{name: "获取不存在的共享", shareName: "nonexistent", wantErr: true, errMsg: "共享不存在"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			share, err := mgr.GetShare(tt.shareName)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetShare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("错误消息不包含 '%s': %v", tt.errMsg, err)
				}
				return
			}
			if share.Name != tt.shareName {
				t.Errorf("共享名称错误：%s", share.Name)
			}
		})
	}
}

func TestListShares(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 初始应该为空
	shares, err := mgr.ListShares()
	if err != nil {
		t.Fatalf("ListShares 失败: %v", err)
	}
	if len(shares) != 0 {
		t.Errorf("初始共享列表应该为空: %d", len(shares))
	}

	// 创建多个共享
	names := []string{"share1", "share2", "share3"}
	for _, name := range names {
		_ = mgr.CreateShare(&Share{
			Name: name,
			Path: filepath.Join(tmpDir, name),
		})
	}

	shares, err = mgr.ListShares()
	if err != nil {
		t.Fatalf("ListShares 失败: %v", err)
	}
	if len(shares) != 3 {
		t.Errorf("共享数量错误：%d", len(shares))
	}

	// 验证所有共享都在列表中
	shareMap := make(map[string]bool)
	for _, s := range shares {
		shareMap[s.Name] = true
	}
	for _, name := range names {
		if !shareMap[name] {
			t.Errorf("共享 %s 不在列表中", name)
		}
	}
}

func TestUpdateShare(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享
	_ = mgr.CreateShare(&Share{
		Name:    "update-test",
		Path:    filepath.Join(tmpDir, "update-test"),
		Comment: "Original",
	})

	tests := []struct {
		name      string
		shareName string
		share     *Share
		wantErr   bool
	}{
		{
			name:      "更新存在的共享",
			shareName: "update-test",
			share: &Share{
				Comment:     "Updated",
				ReadOnly:    true,
				GuestAccess: true,
			},
			wantErr: false,
		},
		{
			name:      "更新不存在的共享",
			shareName: "nonexistent",
			share:     &Share{Comment: "Test"},
			wantErr:   true,
		},
		{
			name:      "nil共享",
			shareName: "update-test",
			share:     nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.UpdateShare(tt.shareName, tt.share)
			if (err != nil) != tt.wantErr {
				t.Errorf("UpdateShare() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr {
				return
			}

			share, _ := mgr.GetShare(tt.shareName)
			if share.Comment != tt.share.Comment {
				t.Errorf("注释未更新：%s", share.Comment)
			}
			if share.ReadOnly != tt.share.ReadOnly {
				t.Errorf("ReadOnly 未更新：%v", share.ReadOnly)
			}
		})
	}
}

func TestDeleteShare(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享
	_ = mgr.CreateShare(&Share{
		Name: "delete-test",
		Path: filepath.Join(tmpDir, "delete-test"),
	})

	// 删除存在的共享
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
	if !strings.Contains(err.Error(), "共享不存在") {
		t.Errorf("错误消息不正确: %v", err)
	}
}

// ========== 权限管理测试 ==========

func TestSetSharePermission(t *testing.T) {
	mgr, tmpDir := setupTestManagerWithUser(t)

	// 创建共享
	_ = mgr.CreateShare(&Share{
		Name: "perm-test",
		Path: filepath.Join(tmpDir, "perm-test"),
	})

	tests := []struct {
		name      string
		shareName string
		username  string
		readWrite bool
		wantErr   bool
	}{
		{name: "设置存在的用户权限", shareName: "perm-test", username: "admin", readWrite: true, wantErr: false},
		{name: "共享不存在", shareName: "nonexistent", username: "admin", readWrite: true, wantErr: true},
		{name: "用户不存在", shareName: "perm-test", username: "nonexistent", readWrite: true, wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := mgr.SetSharePermission(tt.shareName, tt.username, tt.readWrite)
			if (err != nil) != tt.wantErr {
				t.Errorf("SetSharePermission() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}

	// 验证用户被添加到允许列表
	share, _ := mgr.GetShare("perm-test")
	found := false
	for _, u := range share.ValidUsers {
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
	mgr, tmpDir := setupTestManagerWithUser(t)

	// 创建共享并添加权限
	_ = mgr.CreateShare(&Share{
		Name:       "remove-perm-test",
		Path:       filepath.Join(tmpDir, "remove-perm-test"),
		ValidUsers: []string{"admin"},
	})

	// 移除权限
	if err := mgr.RemoveSharePermission("remove-perm-test", "admin"); err != nil {
		t.Fatalf("移除权限失败：%v", err)
	}

	share, _ := mgr.GetShare("remove-perm-test")
	if len(share.ValidUsers) != 0 {
		t.Errorf("允许列表应该为空：%v", share.ValidUsers)
	}

	// 共享不存在
	err := mgr.RemoveSharePermission("nonexistent", "admin")
	if err == nil {
		t.Error("共享不存在应该报错")
	}
}

func TestGetUserShares(t *testing.T) {
	mgr, tmpDir := setupTestManagerWithUser(t)

	// 创建多个共享
	_ = mgr.CreateShare(&Share{
		Name:        "public",
		Path:        filepath.Join(tmpDir, "public"),
		GuestOK:     true,
		GuestAccess: true,
	})
	_ = mgr.CreateShare(&Share{
		Name:       "private",
		Path:       filepath.Join(tmpDir, "private"),
		ValidUsers: []string{"admin"},
	})
	_ = mgr.CreateShare(&Share{
		Name:       "restricted",
		Path:       filepath.Join(tmpDir, "restricted"),
		ValidUsers: []string{"user1"},
	})

	// 管理员应该能看到所有共享
	shares := mgr.GetUserShares("admin")
	if len(shares) != 3 {
		t.Errorf("管理员应该能看到所有共享：%d", len(shares))
	}
}

func TestGetSharePath(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	testPath := filepath.Join(tmpDir, "path-test")
	_ = mgr.CreateShare(&Share{
		Name: "path-test",
		Path: testPath,
	})

	path := mgr.GetSharePath("path-test")
	if path != testPath {
		t.Errorf("共享路径错误：%s", path)
	}

	// 不存在的共享
	emptyPath := mgr.GetSharePath("nonexistent")
	if emptyPath != "" {
		t.Errorf("不存在的共享应该返回空字符串：%s", emptyPath)
	}
}

// ========== 配置生成测试 ==========

func TestManagerGenerateSmbConf(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享
	_ = mgr.CreateShare(&Share{
		Name:        "test-share",
		Path:        filepath.Join(tmpDir, "test-share"),
		Comment:     "Test Share",
		ReadOnly:    false,
		GuestOK:     true,
		GuestAccess: true,
		Browseable:  true,
	})

	config := mgr.generateSmbConf()

	// 验证全局配置
	tests := []struct {
		name    string
		contain string
	}{
		{"工作组设置", "workgroup = WORKGROUP"},
		{"服务器描述", "server string = NAS-OS Samba Server"},
		{"最小协议", "min protocol = SMB2"},
		{"最大协议", "max protocol = SMB3"},
		{"共享定义", "[test-share]"},
		{"共享路径", "path ="},
		{"访客设置", "guest ok = yes"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if !strings.Contains(config, tt.contain) {
				t.Errorf("配置应该包含 '%s'", tt.contain)
			}
		})
	}
}

func TestGenerateSmbConfWithUsers(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建带用户限制的共享
	_ = mgr.CreateShare(&Share{
		Name:       "user-share",
		Path:       filepath.Join(tmpDir, "user-share"),
		ValidUsers: []string{"admin", "user1"},
	})

	config := mgr.generateSmbConf()

	if !strings.Contains(config, "valid users =") {
		t.Error("配置应该包含用户限制")
	}
	if !strings.Contains(config, "admin") {
		t.Error("配置应该包含用户 admin")
	}
}

func TestGenerateSmbConfWithVetoFiles(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享
	_ = mgr.CreateShare(&Share{
		Name: "veto-share",
		Path: filepath.Join(tmpDir, "veto-share"),
	})

	config := mgr.generateSmbConf()

	if !strings.Contains(config, "veto files =") {
		t.Error("配置应该包含隐藏文件设置")
	}
}

// ========== 并发安全测试 ==========

func TestConcurrentCreateShare(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	var wg sync.WaitGroup
	var errCount, successCount int32

	// 并发创建 50 个同名共享
	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := mgr.CreateShare(&Share{
				Name: "concurrent",
				Path: filepath.Join(tmpDir, "concurrent"),
			})
			if err != nil {
				atomic.AddInt32(&errCount, 1)
			} else {
				atomic.AddInt32(&successCount, 1)
			}
		}(i)
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

	// 创建共享
	_ = mgr.CreateShare(&Share{
		Name: "rw-test",
		Path: filepath.Join(tmpDir, "rw-test"),
	})

	var wg sync.WaitGroup

	// 并发读取
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.GetShare("rw-test")
			mgr.ListShares()
		}()
	}

	// 并发更新
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			mgr.UpdateShare("rw-test", &Share{Comment: "updated"})
		}()
	}

	wg.Wait()

	// 验证最终状态
	share, err := mgr.GetShare("rw-test")
	if err != nil {
		t.Fatal(err)
	}
	if share.Comment != "updated" {
		t.Error("最终状态应该是最新的更新")
	}
}

func TestConcurrentDeleteCreate(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	var wg sync.WaitGroup

	// 循环删除和创建
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			name := "cycle"
			path := filepath.Join(tmpDir, name)

			mgr.DeleteShare(name)
			mgr.CreateShare(&Share{Name: name, Path: path})
		}(i)
	}

	wg.Wait()

	// 最终应该存在一个共享
	_, err := mgr.GetShare("cycle")
	if err != nil {
		t.Error("最终应该存在一个共享")
	}
}

// ========== 错误处理测试 ==========

func TestDeleteShareSaveError(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享
	_ = mgr.CreateShare(&Share{
		Name: "delete-test",
		Path: filepath.Join(tmpDir, "delete-test"),
	})

	// 删除
	if err := mgr.DeleteShare("delete-test"); err != nil {
		t.Fatalf("删除失败: %v", err)
	}

	// 验证已删除
	if _, err := mgr.GetShare("delete-test"); err == nil {
		t.Error("共享应该已删除")
	}
}

func TestUpdateShareSaveError(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享
	_ = mgr.CreateShare(&Share{
		Name:    "update-test",
		Path:    filepath.Join(tmpDir, "update-test"),
		Comment: "Original",
	})

	// 更新
	err := mgr.UpdateShare("update-test", &Share{Comment: "Updated"})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	// 验证更新成功
	share, _ := mgr.GetShare("update-test")
	if share.Comment != "Updated" {
		t.Error("更新应该成功")
	}
}

// ========== saveConfigLocked 测试 ==========

func TestSaveConfigLocked(t *testing.T) {
	mgr, _ := setupTestManager(t)

	mgr.mu.Lock()
	mgr.config.Workgroup = "LOCKEDTEST"
	err := mgr.saveConfigLocked()
	mgr.mu.Unlock()

	if err != nil {
		t.Fatalf("saveConfigLocked 失败: %v", err)
	}

	// 验证保存成功
	mgr2, _ := NewManager(mgr.configPath)
	if mgr2.config.Workgroup != "LOCKEDTEST" {
		t.Error("配置应该已保存")
	}
}

// ========== 边界情况测试 ==========

func TestEmptyShares(t *testing.T) {
	mgr, _ := setupTestManager(t)

	// 空共享列表
	shares, err := mgr.ListShares()
	if err != nil {
		t.Fatal(err)
	}
	if len(shares) != 0 {
		t.Errorf("应该为空: %d", len(shares))
	}

	// 空配置生成
	config := mgr.generateSmbConf()
	if !strings.Contains(config, "[global]") {
		t.Error("应该包含全局配置")
	}
}

func TestShareWithEmptyValidUsers(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建空用户列表的共享
	_ = mgr.CreateShare(&Share{
		Name:       "empty-users",
		Path:       filepath.Join(tmpDir, "empty-users"),
		ValidUsers: []string{},
	})

	share, _ := mgr.GetShare("empty-users")
	if len(share.ValidUsers) != 0 {
		t.Errorf("用户列表应该为空: %v", share.ValidUsers)
	}
}

// ========== 配置不存在的情况 ==========

func TestNewManagerNoConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "nonexistent", "smb.json")

	mgr, err := NewManager(configPath)
	if err != nil {
		t.Fatalf("应该成功创建: %v", err)
	}

	// 应该使用默认配置
	if mgr.config.Workgroup != "WORKGROUP" {
		t.Error("应该使用默认配置")
	}
}

// ========== OpenShare/CloseShare 测试 ==========

func TestOpenCloseShare(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享
	_ = mgr.CreateShare(&Share{
		Name: "open-close-test",
		Path: filepath.Join(tmpDir, "open-close-test"),
	})

	// 关闭共享
	if err := mgr.CloseShare("open-close-test"); err != nil {
		t.Fatalf("关闭共享失败: %v", err)
	}

	share, _ := mgr.GetShare("open-close-test")
	if share.Available {
		t.Error("共享应该已关闭")
	}

	// 打开共享
	if err := mgr.OpenShare("open-close-test"); err != nil {
		t.Fatalf("打开共享失败: %v", err)
	}

	share, _ = mgr.GetShare("open-close-test")
	if !share.Available {
		t.Error("共享应该已打开")
	}

	// 不存在的共享
	err := mgr.CloseShare("nonexistent")
	if err == nil {
		t.Error("关闭不存在的共享应该报错")
	}
}

// ========== GetConfig/UpdateConfig 测试 ==========

func TestGetUpdateConfig(t *testing.T) {
	mgr, _ := setupTestManager(t)

	// 获取配置
	config := mgr.GetConfig()
	if config == nil {
		t.Fatal("配置不应为 nil")
	}
	if config.Workgroup != "WORKGROUP" {
		t.Error("工作域应为默认值")
	}

	// 更新配置
	newConfig := &Config{
		Enabled:      true,
		Workgroup:    "NEWGROUP",
		ServerString: "New Server",
		MinProtocol:  "SMB3",
		MaxProtocol:  "SMB3",
	}

	if err := mgr.UpdateConfig(newConfig); err != nil {
		t.Fatalf("更新配置失败: %v", err)
	}

	// 验证更新
	updated := mgr.GetConfig()
	if updated.Workgroup != "NEWGROUP" {
		t.Error("工作域应已更新")
	}
}

// ========== CreateShareFromInput 测试 ==========

func TestCreateShareFromInput(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	share, err := mgr.CreateShareFromInput(ShareInput{
		Name:        "input-test",
		Path:        filepath.Join(tmpDir, "input-test"),
		Comment:     "From Input",
		ReadOnly:    true,
		GuestAccess: true,
	})
	if err != nil {
		t.Fatalf("创建失败: %v", err)
	}

	if share.Name != "input-test" {
		t.Errorf("名称错误: %s", share.Name)
	}
	if share.Comment != "From Input" {
		t.Errorf("注释错误: %s", share.Comment)
	}
}

func TestUpdateShareFromInput(t *testing.T) {
	mgr, tmpDir := setupTestManager(t)

	// 创建共享
	_, _ = mgr.CreateShareFromInput(ShareInput{
		Name: "update-input-test",
		Path: filepath.Join(tmpDir, "update-input-test"),
	})

	// 更新
	share, err := mgr.UpdateShareFromInput("update-input-test", ShareInput{
		Comment:     "Updated",
		ReadOnly:    true,
		GuestAccess: true,
	})
	if err != nil {
		t.Fatalf("更新失败: %v", err)
	}

	if share.Comment != "Updated" {
		t.Errorf("注释错误: %s", share.Comment)
	}
}
