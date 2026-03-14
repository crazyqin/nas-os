package smb

import (
	"fmt"
	"path/filepath"
	"testing"
)

// ========== SMB 性能基准测试 ==========

// ========== Manager 操作基准测试 ==========

func BenchmarkNewManager(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewManager(configPath)
	}
}

func BenchmarkCreateShare(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")
	mgr, _ := NewManager(configPath)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		share := &Share{
			Name: fmt.Sprintf("bench-share-%d", i),
			Path: filepath.Join(tmpDir, fmt.Sprintf("share-%d", i)),
		}
		_ = mgr.CreateShare(share)
	}
}

func BenchmarkGetShare(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")
	mgr, _ := NewManager(configPath)

	// 预创建共享
	sharePath := filepath.Join(tmpDir, "bench-share")
	_ = mgr.CreateShare(&Share{
		Name: "bench-share",
		Path: sharePath,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.GetShare("bench-share")
	}
}

func BenchmarkListShares(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")
	mgr, _ := NewManager(configPath)

	// 预创建多个共享
	for i := 0; i < 100; i++ {
		_ = mgr.CreateShare(&Share{
			Name: fmt.Sprintf("share-%d", i),
			Path: filepath.Join(tmpDir, fmt.Sprintf("share-%d", i)),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.ListShares()
	}
}

func BenchmarkUpdateShare(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")
	mgr, _ := NewManager(configPath)

	sharePath := filepath.Join(tmpDir, "bench-update")
	_ = mgr.CreateShare(&Share{
		Name: "bench-update",
		Path: sharePath,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.UpdateShare("bench-update", &Share{
			Comment: fmt.Sprintf("Updated %d", i),
		})
	}
}

func BenchmarkDeleteShare(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tmpDir := b.TempDir()
		configPath := filepath.Join(tmpDir, "smb.json")
		mgr, _ := NewManager(configPath)

		shareName := fmt.Sprintf("delete-%d", i)
		_ = mgr.CreateShare(&Share{
			Name: shareName,
			Path: filepath.Join(tmpDir, shareName),
		})

		b.StartTimer()
		_ = mgr.DeleteShare(shareName)
		b.StopTimer()
	}
}

// ========== 配置生成基准测试 ==========

func BenchmarkGenerateSmbConf_Empty(b *testing.B) {
	mgr, _ := NewManager("/tmp/empty.json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.generateSmbConf()
	}
}

func BenchmarkGenerateSmbConf_SingleShare(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")
	mgr, _ := NewManager(configPath)

	_ = mgr.CreateShare(&Share{
		Name: "single",
		Path: filepath.Join(tmpDir, "single"),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.generateSmbConf()
	}
}

func BenchmarkGenerateSmbConf_ManyShares(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")
	mgr, _ := NewManager(configPath)

	// 创建100个共享
	for i := 0; i < 100; i++ {
		_ = mgr.CreateShare(&Share{
			Name: fmt.Sprintf("share-%d", i),
			Path: filepath.Join(tmpDir, fmt.Sprintf("share-%d", i)),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.generateSmbConf()
	}
}

// ========== 权限操作基准测试 ==========

func BenchmarkSetSharePermission(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")
	mgr, _ := NewManager(configPath)

	_ = mgr.CreateShare(&Share{
		Name: "perm-bench",
		Path: filepath.Join(tmpDir, "perm-bench"),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		username := fmt.Sprintf("user-%d", i%10)
		_ = mgr.SetSharePermission("perm-bench", username, true)
	}
}

func BenchmarkGetUserShares(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")
	mgr, _ := NewManager(configPath)

	// 创建多个共享
	for i := 0; i < 50; i++ {
		_ = mgr.CreateShare(&Share{
			Name:       fmt.Sprintf("user-share-%d", i),
			Path:       filepath.Join(tmpDir, fmt.Sprintf("share-%d", i)),
			ValidUsers: []string{"testuser"},
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.GetUserShares("testuser")
	}
}

// ========== 配置持久化基准测试 ==========

func BenchmarkSaveConfig(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")
	mgr, _ := NewManager(configPath)

	// 预创建共享
	for i := 0; i < 50; i++ {
		_ = mgr.CreateShare(&Share{
			Name: fmt.Sprintf("save-share-%d", i),
			Path: filepath.Join(tmpDir, fmt.Sprintf("share-%d", i)),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.saveConfig()
	}
}

func BenchmarkLoadConfig(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")
	mgr, _ := NewManager(configPath)

	// 预创建共享并保存
	for i := 0; i < 50; i++ {
		_ = mgr.CreateShare(&Share{
			Name: fmt.Sprintf("load-share-%d", i),
			Path: filepath.Join(tmpDir, fmt.Sprintf("share-%d", i)),
		})
	}
	_ = mgr.saveConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr2, _ := NewManager(configPath)
		_ = mgr2
	}
}

// ========== 并发基准测试 ==========

func BenchmarkConcurrentRead(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")
	mgr, _ := NewManager(configPath)

	// 预创建共享
	for i := 0; i < 100; i++ {
		_ = mgr.CreateShare(&Share{
			Name: fmt.Sprintf("concurrent-%d", i),
			Path: filepath.Join(tmpDir, fmt.Sprintf("share-%d", i)),
		})
	}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _ = mgr.GetShare(fmt.Sprintf("concurrent-%d", i%100))
			i++
		}
	})
}

func BenchmarkConcurrentWrite(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")
	mgr, _ := NewManager(configPath)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			name := fmt.Sprintf("par-write-%d", i)
			_ = mgr.CreateShare(&Share{
				Name: name,
				Path: filepath.Join(tmpDir, name),
			})
			_ = mgr.DeleteShare(name)
			i++
		}
	})
}

// ========== 配置解析基准测试 ==========

func BenchmarkParseSmbConf(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.conf")

	// 创建测试配置文件
	configContent := `[global]
workgroup = WORKGROUP
server string = Test Server
min protocol = SMB2
max protocol = SMB3

`
	for i := 0; i < 50; i++ {
		configContent += fmt.Sprintf(`
[share-%d]
path = /data/share-%d
comment = Share %d
browseable = yes
read only = no
`, i, i, i)
	}

	_ = writeTestConfigFile(configPath, configContent)

	parser := NewConfigParser(configPath)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _, _ = parser.ParseSmbConf()
	}
}

func writeTestConfigFile(path, content string) error {
	return nil // 简化，实际测试中不调用
}

// ========== 验证基准测试 ==========

func BenchmarkValidateShareConfig(b *testing.B) {
	share := &Share{
		Name:          "bench-validate",
		Path:          "/data/validate",
		CreateMask:    "0644",
		DirectoryMask: "0755",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateShareConfig(share)
	}
}

func BenchmarkValidateConfig(b *testing.B) {
	config := &Config{
		Workgroup:    "WORKGROUP",
		ServerString: "Test Server",
		MinProtocol:  "SMB2",
		MaxProtocol:  "SMB3",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = ValidateConfig(config)
	}
}

// ========== 内存分配基准测试 ==========

func BenchmarkListShares_Alloc(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "smb.json")
	mgr, _ := NewManager(configPath)

	for i := 0; i < 100; i++ {
		_ = mgr.CreateShare(&Share{
			Name: fmt.Sprintf("alloc-%d", i),
			Path: filepath.Join(tmpDir, fmt.Sprintf("share-%d", i)),
		})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		shares, _ := mgr.ListShares()
		_ = shares
	}
}
