package integration

import (
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"nas-os/internal/nfs"
	"nas-os/internal/smb"
	"nas-os/internal/users"
)

// ========== 测试辅助函数 ==========

func setupTestEnvironment(t *testing.T) (*smb.Manager, *nfs.Manager, string) {
	tmpDir := t.TempDir()
	smbConfigPath := filepath.Join(tmpDir, "smb.json")
	nfsConfigPath := filepath.Join(tmpDir, "nfs.json")

	userMgr, err := users.NewManager(tmpDir)
	if err != nil {
		t.Fatalf("创建用户管理器失败：%v", err)
	}

	smbMgr, err := smb.NewManagerWithUserMgr(userMgr, smbConfigPath)
	if err != nil {
		t.Fatalf("创建 SMB 管理器失败：%v", err)
	}

	nfsMgr, err := nfs.NewManager(nfsConfigPath)
	if err != nil {
		t.Fatalf("创建 NFS 管理器失败：%v", err)
	}

	return smbMgr, nfsMgr, tmpDir
}

// ========== SMB 共享完整流程测试 ==========

func TestSMBShareFullLifecycle(t *testing.T) {
	smbMgr, _, tmpDir := setupTestEnvironment(t)

	// 1. 创建共享
	share := &smb.Share{
		Name:        "test-share",
		Path:        filepath.Join(tmpDir, "smb-share"),
		Comment:     "Test SMB Share",
		ReadOnly:    false,
		GuestAccess: false,
		ValidUsers:  []string{"admin"},
		Browseable:  true,
	}
	if err := smbMgr.CreateShare(share); err != nil {
		t.Fatalf("创建 SMB 共享失败：%v", err)
	}

	// 验证共享属性
	if share.Name != "test-share" {
		t.Errorf("共享名称错误：%s", share.Name)
	}
	if share.Comment != "Test SMB Share" {
		t.Errorf("共享注释错误：%s", share.Comment)
	}

	// 验证目录已创建
	if _, err := os.Stat(share.Path); os.IsNotExist(err) {
		t.Error("共享目录未创建")
	}

	// 2. 获取共享
	retrieved, err := smbMgr.GetShare("test-share")
	if err != nil {
		t.Fatalf("获取 SMB 共享失败：%v", err)
	}
	if retrieved.Name != share.Name {
		t.Error("获取的共享不匹配")
	}

	// 3. 列出共享
	shares, err := smbMgr.ListShares()
	if err != nil {
		t.Fatalf("列出共享失败：%v", err)
	}
	if len(shares) != 1 {
		t.Errorf("共享列表数量错误：%d", len(shares))
	}

	// 4. 更新共享
	err = smbMgr.UpdateShare("test-share", &smb.Share{
		Comment:     "Updated SMB Share",
		ReadOnly:    true,
		GuestAccess: true,
	})
	if err != nil {
		t.Fatalf("更新 SMB 共享失败：%v", err)
	}
	updated, _ := smbMgr.GetShare("test-share")
	if updated.Comment != "Updated SMB Share" {
		t.Errorf("注释未更新：%s", updated.Comment)
	}
	if !updated.ReadOnly {
		t.Error("应该设置为只读")
	}
	if !updated.GuestAccess {
		t.Error("应该允许访客访问")
	}

	// 5. 设置权限
	err = smbMgr.SetSharePermission("test-share", "admin", true)
	if err != nil {
		t.Fatalf("设置权限失败：%v", err)
	}

	// 6. 获取用户可见共享
	userShares := smbMgr.GetUserShares("admin")
	if len(userShares) != 1 {
		t.Errorf("用户可见共享数量错误：%d", len(userShares))
	}

	// 7. 删除共享
	if err := smbMgr.DeleteShare("test-share"); err != nil {
		t.Fatalf("删除 SMB 共享失败：%v", err)
	}

	// 8. 验证删除
	_, err = smbMgr.GetShare("test-share")
	if err == nil {
		t.Error("共享应该已被删除")
	}
}

func TestSMBShareMultipleShares(t *testing.T) {
	smbMgr, _, tmpDir := setupTestEnvironment(t)

	// 创建多个共享
	shareNames := []string{"share1", "share2", "share3", "share4", "share5"}
	for _, name := range shareNames {
		err := smbMgr.CreateShare(&smb.Share{
			Name:    name,
			Path:    filepath.Join(tmpDir, name),
			Comment: "Share " + name,
		})
		if err != nil {
			t.Fatalf("创建共享 %s 失败：%v", name, err)
		}
	}

	// 验证所有共享
	shares, err := smbMgr.ListShares()
	if err != nil {
		t.Fatal(err)
	}
	if len(shares) != len(shareNames) {
		t.Errorf("共享数量错误：%d != %d", len(shares), len(shareNames))
	}

	// 删除部分共享
	smbMgr.DeleteShare("share2")
	smbMgr.DeleteShare("share4")

	// 验证剩余共享
	shares, _ = smbMgr.ListShares()
	if len(shares) != 3 {
		t.Errorf("删除后共享数量错误：%d", len(shares))
	}
}

func TestSMBSharePermissionFlow(t *testing.T) {
	smbMgr, _, tmpDir := setupTestEnvironment(t)

	// 创建共享
	_ = smbMgr.CreateShare(&smb.Share{
		Name:       "perm-share",
		Path:       filepath.Join(tmpDir, "perm-share"),
		ValidUsers: []string{"admin"},
	})

	// 验证初始权限
	share, _ := smbMgr.GetShare("perm-share")
	if len(share.ValidUsers) != 1 {
		t.Errorf("初始用户数量错误：%d", len(share.ValidUsers))
	}

	// 添加权限（用户已存在）
	err := smbMgr.SetSharePermission("perm-share", "admin", true)
	if err != nil {
		t.Fatalf("设置权限失败：%v", err)
	}

	share, _ = smbMgr.GetShare("perm-share")
	if len(share.ValidUsers) != 1 {
		t.Errorf("重复添加后用户数量错误：%d", len(share.ValidUsers))
	}

	// 移除权限
	err = smbMgr.RemoveSharePermission("perm-share", "admin")
	if err != nil {
		t.Fatalf("移除权限失败：%v", err)
	}

	share, _ = smbMgr.GetShare("perm-share")
	if len(share.ValidUsers) != 0 {
		t.Errorf("移除后用户数量错误：%d", len(share.ValidUsers))
	}
}

// ========== NFS 导出完整流程测试 ==========

func TestNFSExportFullLifecycle(t *testing.T) {
	_, nfsMgr, tmpDir := setupTestEnvironment(t)

	// 1. 创建导出
	export := &nfs.Export{
		Path:    filepath.Join(tmpDir, "nfs-export"),
		Comment: "Test NFS Export",
		Options: nfs.ExportOptions{
			Rw:           true,
			NoRootSquash: true,
		},
		Clients: []nfs.Client{
			{Host: "192.168.1.0/24"},
			{Host: "192.168.1.100"},
		},
	}
	if err := nfsMgr.CreateExport(export); err != nil {
		t.Fatalf("创建 NFS 导出失败：%v", err)
	}

	// 验证导出属性
	if export.Path == "" {
		t.Error("导出路径不应为空")
	}
	if export.Comment != "Test NFS Export" {
		t.Errorf("导出注释错误：%s", export.Comment)
	}
	if len(export.Clients) != 2 {
		t.Errorf("客户端数量错误：%d", len(export.Clients))
	}

	// 验证目录已创建
	if _, err := os.Stat(export.Path); os.IsNotExist(err) {
		t.Error("导出目录未创建")
	}

	// 2. 获取导出
	retrieved, err := nfsMgr.GetExport(export.Path)
	if err != nil {
		t.Fatalf("获取 NFS 导出失败：%v", err)
	}
	if retrieved.Path != export.Path {
		t.Error("获取的导出不匹配")
	}

	// 3. 列出导出
	exports, err := nfsMgr.ListExports()
	if err != nil {
		t.Fatal(err)
	}
	if len(exports) != 1 {
		t.Errorf("导出列表数量错误：%d", len(exports))
	}

	// 4. 更新导出
	err = nfsMgr.UpdateExport(export.Path, &nfs.Export{
		Comment: "Updated NFS Export",
		Options: nfs.ExportOptions{
			Ro: true,
		},
		Clients: []nfs.Client{
			{Host: "10.0.0.0/8"},
			{Host: "172.16.0.0/12"},
		},
	})
	if err != nil {
		t.Fatalf("更新 NFS 导出失败：%v", err)
	}
	updated, _ := nfsMgr.GetExport(export.Path)
	if updated.Comment != "Updated NFS Export" {
		t.Errorf("注释未更新：%s", updated.Comment)
	}
	if !updated.Options.Ro {
		t.Error("应该设置为只读")
	}

	// 5. 删除导出
	if err := nfsMgr.DeleteExport(export.Path); err != nil {
		t.Fatalf("删除 NFS 导出失败：%v", err)
	}

	// 6. 验证删除
	_, err = nfsMgr.GetExport(export.Path)
	if err == nil {
		t.Error("导出应该已被删除")
	}
}

func TestNFSExportMultipleExports(t *testing.T) {
	_, nfsMgr, tmpDir := setupTestEnvironment(t)

	// 创建多个导出
	paths := []string{
		filepath.Join(tmpDir, "export1"),
		filepath.Join(tmpDir, "export2"),
		filepath.Join(tmpDir, "export3"),
		filepath.Join(tmpDir, "export4"),
		filepath.Join(tmpDir, "export5"),
	}
	for _, path := range paths {
		err := nfsMgr.CreateExport(&nfs.Export{
			Path:    path,
			Comment: "Export " + path,
		})
		if err != nil {
			t.Fatalf("创建导出 %s 失败：%v", path, err)
		}
	}

	// 验证所有导出
	exports, err := nfsMgr.ListExports()
	if err != nil {
		t.Fatal(err)
	}
	if len(exports) != len(paths) {
		t.Errorf("导出数量错误：%d != %d", len(exports), len(paths))
	}

	// 删除部分导出
	nfsMgr.DeleteExport(paths[1])
	nfsMgr.DeleteExport(paths[3])

	// 验证剩余导出
	exports, _ = nfsMgr.ListExports()
	if len(exports) != 3 {
		t.Errorf("删除后导出数量错误：%d", len(exports))
	}
}

func TestNFSExportNetworkConfig(t *testing.T) {
	_, nfsMgr, tmpDir := setupTestEnvironment(t)

	// 创建带多种网络配置的导出
	exportPath := filepath.Join(tmpDir, "network-export")
	_ = nfsMgr.CreateExport(&nfs.Export{
		Path: exportPath,
		Clients: []nfs.Client{
			{Host: "192.168.0.0/16"},
			{Host: "10.0.0.0/8"},
			{Host: "172.16.0.1"},
			{Host: "172.16.0.2"},
		},
	})

	// 验证配置
	exp, _ := nfsMgr.GetExport(exportPath)
	if len(exp.Clients) != 4 {
		t.Errorf("客户端数量错误：%d", len(exp.Clients))
	}

	// 生成配置并验证
	exports, err := nfsMgr.GenerateExportsFile()
	if err != nil {
		t.Fatal(err)
	}
	for _, host := range []string{"192.168.0.0/16", "10.0.0.0/8", "172.16.0.1", "172.16.0.2"} {
		if !containsSubstr(exports, host) {
			t.Errorf("配置应该包含客户端 %s", host)
		}
	}
}

// ========== 权限验证测试 ==========

func TestSMBPermissionValidation(t *testing.T) {
	smbMgr, _, tmpDir := setupTestEnvironment(t)

	// 创建共享
	_ = smbMgr.CreateShare(&smb.Share{
		Name:       "validation-share",
		Path:       filepath.Join(tmpDir, "validation-share"),
		ValidUsers: []string{"admin"},
	})

	// 测试用户不存在的情况
	err := smbMgr.SetSharePermission("validation-share", "nonexistent", true)
	if err == nil {
		t.Error("用户不存在应该返回错误")
	}

	// 测试共享不存在的情况
	err = smbMgr.SetSharePermission("nonexistent", "admin", true)
	if err == nil {
		t.Error("共享不存在应该返回错误")
	}
}

func TestSMBGuestAccess(t *testing.T) {
	smbMgr, _, tmpDir := setupTestEnvironment(t)

	// 创建访客共享
	_ = smbMgr.CreateShare(&smb.Share{
		Name:        "guest-share",
		Path:        filepath.Join(tmpDir, "guest-share"),
		GuestOK:     true,
		GuestAccess: true,
	})

	// 验证访客共享对所有人都可见
	shares := smbMgr.GetUserShares("nonexistent-user")
	if len(shares) != 1 {
		t.Errorf("访客共享应该对所有人都可见：%d", len(shares))
	}

	// 创建受限共享
	_ = smbMgr.CreateShare(&smb.Share{
		Name:        "restricted-share",
		Path:        filepath.Join(tmpDir, "restricted-share"),
		GuestOK:     false,
		GuestAccess: false,
		ValidUsers:  []string{"admin"},
	})

	// 管理员应该能看到所有共享
	adminShares := smbMgr.GetUserShares("admin")
	if len(adminShares) != 2 {
		t.Errorf("管理员应该能看到所有共享：%d", len(adminShares))
	}
}

func TestNFSReadOnlyValidation(t *testing.T) {
	_, nfsMgr, tmpDir := setupTestEnvironment(t)

	// 创建读写导出
	rwPath := filepath.Join(tmpDir, "rw-export")
	_ = nfsMgr.CreateExport(&nfs.Export{
		Path:    rwPath,
		Options: nfs.ExportOptions{Rw: true},
	})

	// 创建只读导出
	roPath := filepath.Join(tmpDir, "ro-export")
	_ = nfsMgr.CreateExport(&nfs.Export{
		Path:    roPath,
		Options: nfs.ExportOptions{Ro: true},
	})

	// 验证配置生成
	exports, err := nfsMgr.GenerateExportsFile()
	if err != nil {
		t.Fatal(err)
	}

	// 读写导出应该有 rw
	if !containsSubstr(exports, "rw") {
		t.Error("读写导出应该包含 rw 选项")
	}

	// 只读导出应该有 ro
	if !containsSubstr(exports, "ro") {
		t.Error("只读导出应该包含 ro 选项")
	}
}

// ========== 并发访问测试 ==========

func TestSMBConcurrentOperations(t *testing.T) {
	smbMgr, _, tmpDir := setupTestEnvironment(t)

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// 并发创建 20 个共享
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := smbMgr.CreateShare(&smb.Share{
				Name:    "concurrent-share-" + time.Now().Format("20060102150405.000"),
				Path:    filepath.Join(tmpDir, "share"),
				Comment: "Concurrent test",
			})
			if err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// 验证没有 panic 或竞态条件
	for err := range errChan {
		if err != nil && !containsSubstr(err.Error(), "共享已存在") {
			t.Errorf("意外的错误：%v", err)
		}
	}
}

func TestNFSConcurrentOperations(t *testing.T) {
	_, nfsMgr, tmpDir := setupTestEnvironment(t)

	var wg sync.WaitGroup
	errChan := make(chan error, 100)

	// 并发创建 20 个导出
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func(idx int) {
			defer wg.Done()
			err := nfsMgr.CreateExport(&nfs.Export{
				Path:    filepath.Join(tmpDir, "export") + time.Now().Format(".000"),
				Comment: "Concurrent test",
			})
			if err != nil {
				errChan <- err
			}
		}(i)
	}

	wg.Wait()
	close(errChan)

	// 验证没有 panic 或竞态条件
	for err := range errChan {
		if err != nil && !containsSubstr(err.Error(), "导出已存在") {
			t.Errorf("意外的错误：%v", err)
		}
	}
}

func TestConcurrentReadWriteMix(t *testing.T) {
	smbMgr, nfsMgr, tmpDir := setupTestEnvironment(t)

	// 创建初始共享和导出
	smbPath := filepath.Join(tmpDir, "smb-mix")
	nfsPath := filepath.Join(tmpDir, "nfs-mix")
	_ = smbMgr.CreateShare(&smb.Share{
		Name:    "mix-test",
		Path:    smbPath,
		Comment: "Initial",
	})
	_ = nfsMgr.CreateExport(&nfs.Export{
		Path:    nfsPath,
		Comment: "Initial",
	})

	var wg sync.WaitGroup

	// 并发 SMB 读取
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			smbMgr.GetShare("mix-test")
			smbMgr.ListShares()
			smbMgr.GetUserShares("admin")
		}()
	}

	// 并发 NFS 读取
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nfsMgr.GetExport(nfsPath)
			nfsMgr.ListExports()
		}()
	}

	// 并发 SMB 更新
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			smbMgr.UpdateShare("mix-test", &smb.Share{Comment: "Updated"})
		}()
	}

	// 并发 NFS 更新
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			nfsMgr.UpdateExport(nfsPath, &nfs.Export{Comment: "Updated"})
		}()
	}

	wg.Wait()

	// 验证最终状态一致
	share, _ := smbMgr.GetShare("mix-test")
	if share.Comment != "Updated" {
		t.Error("SMB 共享应该已更新")
	}

	exp, _ := nfsMgr.GetExport(nfsPath)
	if exp.Comment != "Updated" {
		t.Error("NFS 导出应该已更新")
	}
}

// ========== 配置持久化测试 ==========

func TestSMBConfigPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	smbConfigPath := filepath.Join(tmpDir, "smb.json")
	userMgr, _ := users.NewManager(tmpDir)

	// 创建管理器并添加共享
	smbMgr, err := smb.NewManagerWithUserMgr(userMgr, smbConfigPath)
	if err != nil {
		t.Fatal(err)
	}

	_ = smbMgr.CreateShare(&smb.Share{
		Name:        "persistent-share",
		Path:        filepath.Join(tmpDir, "persistent"),
		Comment:     "Persistent Share",
		ReadOnly:    true,
		GuestAccess: true,
		ValidUsers:  []string{"admin"},
	})

	// 创建新管理器重新加载
	smbMgr2, err := smb.NewManagerWithUserMgr(userMgr, smbConfigPath)
	if err != nil {
		t.Fatal(err)
	}

	// 验证共享已加载
	share, err := smbMgr2.GetShare("persistent-share")
	if err != nil {
		t.Fatalf("共享未持久化：%v", err)
	}

	if share.Comment != "Persistent Share" {
		t.Errorf("共享注释错误：%s", share.Comment)
	}
	if !share.ReadOnly {
		t.Error("只读属性未持久化")
	}
	if !share.GuestAccess {
		t.Error("访客访问属性未持久化")
	}
}

func TestNFSConfigPersistence(t *testing.T) {
	tmpDir := t.TempDir()
	nfsConfigPath := filepath.Join(tmpDir, "nfs.json")

	// 创建管理器并添加导出
	nfsMgr, err := nfs.NewManager(nfsConfigPath)
	if err != nil {
		t.Fatal(err)
	}

	exportPath := filepath.Join(tmpDir, "persistent")
	_ = nfsMgr.CreateExport(&nfs.Export{
		Path:    exportPath,
		Comment: "Persistent Export",
		Options: nfs.ExportOptions{
			Ro:           true,
			NoRootSquash: true,
		},
		Clients: []nfs.Client{
			{Host: "192.168.1.0/24"},
			{Host: "192.168.1.100"},
		},
	})

	// 创建新管理器重新加载
	nfsMgr2, err := nfs.NewManager(nfsConfigPath)
	if err != nil {
		t.Fatal(err)
	}

	// 验证导出已加载
	exp, err := nfsMgr2.GetExport(exportPath)
	if err != nil {
		t.Fatalf("导出未持久化：%v", err)
	}

	if exp.Comment != "Persistent Export" {
		t.Errorf("导出注释错误：%s", exp.Comment)
	}
	if !exp.Options.Ro {
		t.Error("只读属性未持久化")
	}
	if len(exp.Clients) != 2 {
		t.Error("客户端列表未持久化")
	}
}

// ========== 边界情况测试 ==========

func TestSMBEmptyOperations(t *testing.T) {
	smbMgr, _, _ := setupTestEnvironment(t)

	// 空列表
	shares, err := smbMgr.ListShares()
	if err != nil {
		t.Fatal(err)
	}
	if len(shares) != 0 {
		t.Error("初始共享列表应该为空")
	}

	// 获取不存在的共享
	_, err = smbMgr.GetShare("nonexistent")
	if err == nil {
		t.Error("获取不存在的共享应该返回错误")
	}

	// 删除不存在的共享
	err = smbMgr.DeleteShare("nonexistent")
	if err == nil {
		t.Error("删除不存在的共享应该返回错误")
	}

	// 更新不存在的共享
	err = smbMgr.UpdateShare("nonexistent", &smb.Share{})
	if err == nil {
		t.Error("更新不存在的共享应该返回错误")
	}
}

func TestNFSEmptyOperations(t *testing.T) {
	_, nfsMgr, _ := setupTestEnvironment(t)

	// 空列表
	exports, err := nfsMgr.ListExports()
	if err != nil {
		t.Fatal(err)
	}
	if len(exports) != 0 {
		t.Error("初始导出列表应该为空")
	}

	// 获取不存在的导出
	_, err = nfsMgr.GetExport("/nonexistent")
	if err == nil {
		t.Error("获取不存在的导出应该返回错误")
	}

	// 删除不存在的导出
	err = nfsMgr.DeleteExport("/nonexistent")
	if err == nil {
		t.Error("删除不存在的导出应该返回错误")
	}

	// 更新不存在的导出
	err = nfsMgr.UpdateExport("/nonexistent", &nfs.Export{})
	if err == nil {
		t.Error("更新不存在的导出应该返回错误")
	}
}

func TestSMBDuplicateOperations(t *testing.T) {
	smbMgr, _, tmpDir := setupTestEnvironment(t)

	// 创建共享
	_ = smbMgr.CreateShare(&smb.Share{
		Name: "duplicate-test",
		Path: filepath.Join(tmpDir, "duplicate"),
	})

	// 重复创建
	err := smbMgr.CreateShare(&smb.Share{
		Name: "duplicate-test",
		Path: filepath.Join(tmpDir, "duplicate2"),
	})
	if err == nil {
		t.Error("重复创建应该返回错误")
	}
}

func TestNFSDuplicateOperations(t *testing.T) {
	_, nfsMgr, tmpDir := setupTestEnvironment(t)

	// 创建导出
	exportPath := filepath.Join(tmpDir, "duplicate")
	_ = nfsMgr.CreateExport(&nfs.Export{
		Path: exportPath,
	})

	// 重复创建
	err := nfsMgr.CreateExport(&nfs.Export{
		Path: exportPath,
	})
	if err == nil {
		t.Error("重复创建应该返回错误")
	}
}

// ========== 辅助函数 ==========

func containsSubstr(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
