package nfs

import (
	"fmt"
	"path/filepath"
	"testing"
)

// ========== NFS 性能基准测试 ==========

// ========== Manager 操作基准测试 ==========

func BenchmarkNFSNewManager(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = NewManager(configPath)
	}
}

func BenchmarkNFSCreateExport(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		export := &Export{
			Path: filepath.Join(tmpDir, fmt.Sprintf("export-%d", i)),
		}
		_ = mgr.CreateExport(export)
	}
}

func BenchmarkNFSGetExport(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	exportPath := filepath.Join(tmpDir, "bench-export")
	_ = mgr.CreateExport(&Export{
		Path: exportPath,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.GetExport(exportPath)
	}
}

func BenchmarkNFSListExports(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	// 预创建多个导出
	for i := 0; i < 100; i++ {
		_ = mgr.CreateExport(&Export{
			Path: filepath.Join(tmpDir, fmt.Sprintf("export-%d", i)),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.ListExports()
	}
}

func BenchmarkNFSUpdateExport(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	exportPath := filepath.Join(tmpDir, "bench-update")
	_ = mgr.CreateExport(&Export{
		Path: exportPath,
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.UpdateExport(exportPath, &Export{
			Comment: fmt.Sprintf("Updated %d", i),
		})
	}
}

func BenchmarkNFSDeleteExport(b *testing.B) {
	for i := 0; i < b.N; i++ {
		tmpDir := b.TempDir()
		configPath := filepath.Join(tmpDir, "nfs.json")
		mgr, _ := NewManager(configPath)

		exportPath := filepath.Join(tmpDir, fmt.Sprintf("delete-%d", i))
		_ = mgr.CreateExport(&Export{
			Path: exportPath,
		})

		b.StartTimer()
		_ = mgr.DeleteExport(exportPath)
		b.StopTimer()
	}
}

// ========== 配置生成基准测试 ==========

func BenchmarkGenerateExportsFile_Empty(b *testing.B) {
	mgr, _ := NewManager("/tmp/empty-nfs.json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.GenerateExportsFile()
	}
}

func BenchmarkGenerateExportsFile_SingleExport(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	_ = mgr.CreateExport(&Export{
		Path: filepath.Join(tmpDir, "single"),
	})

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.GenerateExportsFile()
	}
}

func BenchmarkGenerateExportsFile_ManyExports(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	// 创建100个导出
	for i := 0; i < 100; i++ {
		_ = mgr.CreateExport(&Export{
			Path: filepath.Join(tmpDir, fmt.Sprintf("export-%d", i)),
			Clients: []Client{
				{Host: "192.168.1.0/24"},
			},
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.GenerateExportsFile()
	}
}

func BenchmarkGenerateExportsFile_ComplexExports(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	// 创建复杂配置的导出
	for i := 0; i < 50; i++ {
		_ = mgr.CreateExport(&Export{
			Path: filepath.Join(tmpDir, fmt.Sprintf("complex-%d", i)),
			Options: ExportOptions{
				Rw:           true,
				NoRootSquash: true,
				Async:        true,
			},
			Clients: []Client{
				{Host: "192.168.1.0/24", Options: []string{"rw"}},
				{Host: "10.0.0.0/8", Options: []string{"ro"}},
				{Host: "172.16.0.1"},
			},
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = mgr.GenerateExportsFile()
	}
}

// ========== 选项转换基准测试 ==========

func BenchmarkOptionsToString(b *testing.B) {
	mgr, _ := NewManager("/tmp/opts.json")
	opts := &ExportOptions{
		Rw:           true,
		NoRootSquash: true,
		Async:        false,
		Secure:       true,
		SubtreeCheck: false,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.optionsToString(opts)
	}
}

func BenchmarkOptionsToString_AllEnabled(b *testing.B) {
	mgr, _ := NewManager("/tmp/opts-all.json")
	opts := &ExportOptions{
		Ro:           false,
		Rw:           true,
		NoRootSquash: true,
		Async:        true,
		Secure:       true,
		SubtreeCheck: true,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.optionsToString(opts)
	}
}

// ========== 配置持久化基准测试 ==========

func BenchmarkNFSSaveConfig(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	// 预创建导出
	for i := 0; i < 50; i++ {
		_ = mgr.CreateExport(&Export{
			Path: filepath.Join(tmpDir, fmt.Sprintf("save-export-%d", i)),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.saveConfig()
	}
}

func BenchmarkNFSSaveConfigLocked(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	// 预创建导出
	for i := 0; i < 50; i++ {
		_ = mgr.CreateExport(&Export{
			Path: filepath.Join(tmpDir, fmt.Sprintf("locked-export-%d", i)),
		})
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr.mu.Lock()
		_ = mgr.saveConfigLocked()
		mgr.mu.Unlock()
	}
}

func BenchmarkNFSLoadConfig(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	// 预创建导出并保存
	for i := 0; i < 50; i++ {
		_ = mgr.CreateExport(&Export{
			Path: filepath.Join(tmpDir, fmt.Sprintf("load-export-%d", i)),
		})
	}
	_ = mgr.saveConfig()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		mgr2, _ := NewManager(configPath)
		_ = mgr2
	}
}

// ========== 验证基准测试 ==========

func BenchmarkValidateExport(b *testing.B) {
	mgr, _ := NewManager("/tmp/validate.json")
	export := &Export{
		Path: "/data/validate",
		Options: ExportOptions{
			Rw: true,
		},
		Clients: []Client{
			{Host: "192.168.1.0/24"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.ValidateExport(export)
	}
}

func BenchmarkValidateExport_Complex(b *testing.B) {
	mgr, _ := NewManager("/tmp/validate-complex.json")
	export := &Export{
		Path: "/data/complex",
		Options: ExportOptions{
			Rw:           true,
			NoRootSquash: true,
			Async:        true,
		},
		Clients: []Client{
			{Host: "192.168.1.0/24", Options: []string{"rw", "no_root_squash"}},
			{Host: "10.0.0.0/8", Options: []string{"ro"}},
			{Host: "172.16.0.1"},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = mgr.ValidateExport(export)
	}
}

// ========== 并发基准测试 ==========

func BenchmarkNFSConcurrentRead(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	// 预创建导出
	for i := 0; i < 100; i++ {
		_ = mgr.CreateExport(&Export{
			Path: filepath.Join(tmpDir, fmt.Sprintf("concurrent-%d", i)),
		})
	}

	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			_, _ = mgr.GetExport(filepath.Join(tmpDir, fmt.Sprintf("concurrent-%d", i%100)))
			i++
		}
	})
}

func BenchmarkNFSConcurrentWrite(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			name := filepath.Join(tmpDir, fmt.Sprintf("par-write-%d", i))
			_ = mgr.CreateExport(&Export{Path: name})
			_ = mgr.DeleteExport(name)
			i++
		}
	})
}

func BenchmarkNFSConcurrentReadWrite(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	// 预创建导出
	for i := 0; i < 50; i++ {
		_ = mgr.CreateExport(&Export{
			Path: filepath.Join(tmpDir, fmt.Sprintf("rw-%d", i)),
		})
	}

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				_, _ = mgr.GetExport(filepath.Join(tmpDir, fmt.Sprintf("rw-%d", i%50)))
			} else {
				_ = mgr.UpdateExport(filepath.Join(tmpDir, fmt.Sprintf("rw-%d", i%50)), &Export{
					Comment: fmt.Sprintf("Updated %d", i),
				})
			}
			i++
		}
	})
}

// ========== 内存分配基准测试 ==========

func BenchmarkNFSListExports_Alloc(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	for i := 0; i < 100; i++ {
		_ = mgr.CreateExport(&Export{
			Path: filepath.Join(tmpDir, fmt.Sprintf("alloc-%d", i)),
		})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		exports, _ := mgr.ListExports()
		_ = exports
	}
}

func BenchmarkGenerateExportsFile_Alloc(b *testing.B) {
	tmpDir := b.TempDir()
	configPath := filepath.Join(tmpDir, "nfs.json")
	mgr, _ := NewManager(configPath)

	for i := 0; i < 100; i++ {
		_ = mgr.CreateExport(&Export{
			Path: filepath.Join(tmpDir, fmt.Sprintf("gen-alloc-%d", i)),
			Clients: []Client{
				{Host: "192.168.1.0/24"},
			},
		})
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		content, _ := mgr.GenerateExportsFile()
		_ = content
	}
}

// ========== 配置解析器基准测试 ==========

func BenchmarkParseExportsFile(b *testing.B) {
	tmpDir := b.TempDir()
	exportsPath := filepath.Join(tmpDir, "exports")

	// 创建测试配置文件
	exportsContent := ""
	for i := 0; i < 50; i++ {
		exportsContent += fmt.Sprintf("/data/export%d 192.168.1.0/24(rw,no_root_squash) 10.0.0.0/8(ro)\n", i)
	}
	_ = writeTestNFSConfig(exportsPath, exportsContent)

	parser := NewConfigParser()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.ParseExportsFile(exportsPath)
	}
}

func BenchmarkParseExportLine(b *testing.B) {
	parser := NewConfigParser()
	line := "/data/share 192.168.1.0/24(rw,no_root_squash,async) 10.0.0.0/8(ro) 172.16.0.1(rw)"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.parseExportLine(line)
	}
}

func BenchmarkParseClientSpec(b *testing.B) {
	parser := NewConfigParser()
	spec := "192.168.1.0/24(rw,no_root_squash,async,secure)"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = parser.parseClientSpec(spec)
	}
}

func writeTestNFSConfig(path, content string) error {
	return nil // 简化
}

// ========== 应用默认选项基准测试 ==========

func BenchmarkApplyDefaultOptions(b *testing.B) {
	mgr, _ := NewManager("/tmp/defaults.json")

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		export := &Export{Path: "/test/defaults"}
		mgr.applyDefaultOptions(export)
	}
}

// ========== 客户端操作基准测试 ==========

func BenchmarkMergeExports(b *testing.B) {
	parser := NewConfigParser()

	existing := make([]*Export, 50)
	for i := 0; i < 50; i++ {
		existing[i] = &Export{
			Path:    fmt.Sprintf("/existing/%d", i),
			Comment: "old",
		}
	}

	newExports := make([]*Export, 30)
	for i := 0; i < 30; i++ {
		newExports[i] = &Export{
			Path:    fmt.Sprintf("/existing/%d", i), // 部分覆盖
			Comment: "new",
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parser.MergeExports(existing, newExports)
	}
}
