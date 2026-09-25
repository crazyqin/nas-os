package application

import (
	"errors"
	"path/filepath"
	"reflect"
	"testing"

	"nas-os/internal/config"
)

func TestFriendlyAddr(t *testing.T) {
	tests := []struct {
		name string
		cfg  config.ServerConfig
		want string
	}{
		{name: "all interfaces", cfg: config.ServerConfig{Host: "0.0.0.0", Port: 8080}, want: "localhost:8080"},
		{name: "empty host", cfg: config.ServerConfig{Port: 9090}, want: "localhost:9090"},
		{name: "specific host", cfg: config.ServerConfig{Host: "127.0.0.1", Port: 8081}, want: "127.0.0.1:8081"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := FriendlyAddr(tt.cfg); got != tt.want {
				t.Fatalf("FriendlyAddr() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestCleanupStackRunsInReverseOrder(t *testing.T) {
	stack := &cleanupStack{}
	order := []string{}
	stack.add("first", func() error {
		order = append(order, "first")
		return nil
	})
	stack.add("second", func() error {
		order = append(order, "second")
		return nil
	})
	if err := stack.run(); err != nil {
		t.Fatalf("unexpected cleanup error: %v", err)
	}
	want := []string{"second", "first"}
	if !reflect.DeepEqual(order, want) {
		t.Fatalf("order = %v, want %v", order, want)
	}
	if len(stack.items) != 0 {
		t.Fatal("cleanup stack should be released after run")
	}
}

func TestCleanupStackAggregatesErrors(t *testing.T) {
	stack := &cleanupStack{}
	firstErr := errors.New("first")
	secondErr := errors.New("second")
	stack.add("first", func() error { return firstErr })
	stack.add("second", func() error { return secondErr })
	err := stack.run()
	if !errors.Is(err, firstErr) || !errors.Is(err, secondErr) {
		t.Fatalf("expected joined cleanup errors, got %v", err)
	}
}

func TestCleanupStackReleaseSkipsCleanup(t *testing.T) {
	stack := &cleanupStack{}
	called := false
	stack.add("resource", func() error {
		called = true
		return nil
	})
	stack.release()
	if err := stack.run(); err != nil {
		t.Fatalf("unexpected cleanup error: %v", err)
	}
	if called {
		t.Fatal("released cleanup stack should not run callbacks")
	}
}

// TestSmbStatePathDecoupledFromSambaConf 锁住冒烟第 7 次根因的接线：
// SMB 状态文件必须落在 ConfigDir 下的 smb.json，绝不能指向 Samba 的
// ini 配置路径（stock /etc/samba/smb.conf 会让 nasd 启动即 JSON 解析 fatal）。
func TestSmbStatePathDecoupledFromSambaConf(t *testing.T) {
	cfg := config.Default()
	cfg.Paths.ConfigDir = t.TempDir()
	// 模拟生产：SambaConfig 指向 chroot 阶段 dpkg 带出的 ini 文件
	cfg.Paths.SambaConfig = "/etc/samba/smb.conf"

	got := smbStatePath(cfg)
	if got == cfg.Paths.SambaConfig {
		t.Fatalf("SMB 状态路径不得复用 SambaConfig ini 路径: %s", got)
	}
	want := filepath.Join(cfg.Paths.ConfigDir, "smb.json")
	if got != want {
		t.Fatalf("smbStatePath() = %s, want %s", got, want)
	}
}

// TestNfsStatePathDecoupledFromExportsConf 锁住冒烟第 8 次根因的接线：
// NFS 状态文件必须落在 ConfigDir 下的 nfs.json，绝不能指向 NFS 的
// ini 导出路径（stock /etc/exports 会让 nasd 启动即 JSON 解析 fatal）。
func TestNfsStatePathDecoupledFromExportsConf(t *testing.T) {
	cfg := config.Default()
	cfg.Paths.ConfigDir = t.TempDir()
	// 模拟生产：NFSExports 指向 chroot 阶段 dpkg 带出的 ini 文件
	cfg.Paths.NFSExports = "/etc/exports"

	got := nfsStatePath(cfg)
	if got == cfg.Paths.NFSExports {
		t.Fatalf("NFS 状态路径不得复用 NFSExports ini 路径: %s", got)
	}
	want := filepath.Join(cfg.Paths.ConfigDir, "nfs.json")
	if got != want {
		t.Fatalf("nfsStatePath() = %s, want %s", got, want)
	}
}
