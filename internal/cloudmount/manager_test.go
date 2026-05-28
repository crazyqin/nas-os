package cloudmount

import (
	"testing"
)

func TestNewManager(t *testing.T) {
	m := NewManager()
	if m == nil {
		t.Fatal("expected manager")
	}
}

func TestRegisterAccount(t *testing.T) {
	m := NewManager()

	account := &CloudAccount{
		Provider: ProviderS3,
		Name:     "测试S3",
		Endpoint: "https://s3.amazonaws.com",
		Region:   "us-east-1",
		Auth: AuthConfig{
			Type:      "key",
			AccessKey: "AKIAIOSFODNN7EXAMPLE",
			SecretKey: "wJalrXUtnFEMI/K7MDENG/bPxRfiCYEXAMPLEKEY",
		},
	}

	err := m.RegisterAccount(account)
	if err != nil {
		t.Fatalf("register account failed: %v", err)
	}

	if account.ID == "" {
		t.Error("expected account ID to be set")
	}
	if !account.IsValid {
		t.Error("expected account to be valid")
	}

	// 测试空名称
	err = m.RegisterAccount(&CloudAccount{Provider: ProviderS3})
	if err == nil {
		t.Error("expected error for empty name")
	}

	// 测试空 provider
	err = m.RegisterAccount(&CloudAccount{Name: "test"})
	if err == nil {
		t.Error("expected error for empty provider")
	}
}

func TestListAccounts(t *testing.T) {
	m := NewManager()

	m.RegisterAccount(&CloudAccount{Provider: ProviderS3, Name: "S3账号"})
	m.RegisterAccount(&CloudAccount{Provider: ProviderOSS, Name: "OSS账号"})

	accounts := m.ListAccounts()
	if len(accounts) != 2 {
		t.Errorf("expected 2 accounts, got %d", len(accounts))
	}
}

func TestRemoveAccount(t *testing.T) {
	m := NewManager()

	account := &CloudAccount{Provider: ProviderS3, Name: "待删除"}
	m.RegisterAccount(account)

	err := m.RemoveAccount(account.ID)
	if err != nil {
		t.Fatalf("remove account failed: %v", err)
	}

	accounts := m.ListAccounts()
	if len(accounts) != 0 {
		t.Errorf("expected 0 accounts, got %d", len(accounts))
	}

	// 测试移除不存在的账号
	err = m.RemoveAccount("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent account")
	}

	// 测试移除正在使用的账号
	account2 := &CloudAccount{Provider: ProviderS3, Name: "S3账号"}
	m.RegisterAccount(account2)

	m.Mount(&MountPoint{
		Name:      "test",
		Provider:  ProviderS3,
		Bucket:    "mybucket",
		LocalPath: "/mnt/test",
		AccountID: account2.ID,
	}, nil)

	err = m.RemoveAccount(account2.ID)
	if err == nil {
		t.Error("expected error when removing account in use")
	}
}

func TestMount(t *testing.T) {
	m := NewManager()

	account := &CloudAccount{Provider: ProviderS3, Name: "S3账号"}
	m.RegisterAccount(account)

	point := &MountPoint{
		Name:      "测试挂载",
		Provider:  ProviderS3,
		Bucket:    "mybucket",
		LocalPath: "/mnt/test",
		AccountID: account.ID,
	}

	opts := &MountOptions{
		ReadOnly:      false,
		CacheSize:     2048,
		Prefetch:      true,
		CacheStrategy: CacheAll,
	}

	err := m.Mount(point, opts)
	if err != nil {
		t.Fatalf("mount failed: %v", err)
	}

	if point.ID == "" {
		t.Error("expected mount ID to be set")
	}
	if point.Status != StatusMounted {
		t.Errorf("expected status mounted, got %s", point.Status)
	}
	if point.Options.CacheSize != 2048 {
		t.Errorf("expected cache size 2048, got %d", point.Options.CacheSize)
	}

	// 测试必需字段
	err = m.Mount(&MountPoint{Provider: ProviderS3, Bucket: "b", LocalPath: "/mnt"}, nil)
	if err == nil {
		t.Error("expected error for empty name")
	}

	err = m.Mount(&MountPoint{Name: "test", Bucket: "b", LocalPath: "/mnt"}, nil)
	if err == nil {
		t.Error("expected error for empty provider")
	}
}

func TestUnmount(t *testing.T) {
	m := NewManager()

	account := &CloudAccount{Provider: ProviderS3, Name: "S3账号"}
	m.RegisterAccount(account)

	point := &MountPoint{
		Name:      "测试挂载",
		Provider:  ProviderS3,
		Bucket:    "mybucket",
		LocalPath: "/mnt/test",
		AccountID: account.ID,
	}
	m.Mount(point, nil)

	err := m.Unmount(point.ID)
	if err != nil {
		t.Fatalf("unmount failed: %v", err)
	}

	status := m.GetMountStatus(point.ID)
	if status != StatusUnmounted {
		t.Errorf("expected unmounted, got %s", status)
	}

	// 测试重复卸载
	err = m.Unmount(point.ID)
	if err == nil {
		t.Error("expected error for already unmounted")
	}

	// 测试不存在的挂载点
	err = m.Unmount("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent mount")
	}
}

func TestListMounts(t *testing.T) {
	m := NewManager()

	account := &CloudAccount{Provider: ProviderS3, Name: "S3账号"}
	m.RegisterAccount(account)

	m.Mount(&MountPoint{
		Name: "挂载1", Provider: ProviderS3, Bucket: "bucket1",
		LocalPath: "/mnt/1", AccountID: account.ID,
	}, nil)

	m.Mount(&MountPoint{
		Name: "挂载2", Provider: ProviderS3, Bucket: "bucket2",
		LocalPath: "/mnt/2", AccountID: account.ID,
	}, nil)

	mounts := m.ListMounts()
	if len(mounts) != 2 {
		t.Errorf("expected 2 mounts, got %d", len(mounts))
	}
}

func TestSyncStatus(t *testing.T) {
	m := NewManager()

	account := &CloudAccount{Provider: ProviderS3, Name: "S3账号"}
	m.RegisterAccount(account)

	point := &MountPoint{
		Name: "测试挂载", Provider: ProviderS3, Bucket: "mybucket",
		LocalPath: "/mnt/test", AccountID: account.ID,
	}
	m.Mount(point, nil)

	status, err := m.GetSyncStatus(point.ID)
	if err != nil {
		t.Fatalf("get sync status failed: %v", err)
	}
	if status == nil {
		t.Fatal("expected sync status")
	}

	// 测试不存在的挂载点
	_, err = m.GetSyncStatus("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent mount")
	}
}

func TestTransferStats(t *testing.T) {
	m := NewManager()

	stats := m.GetTransferStats()
	if stats == nil {
		t.Fatal("expected transfer stats")
	}
	if stats.UpdatedAt.IsZero() {
		t.Error("expected UpdatedAt to be set")
	}
}

func TestSetSpeedLimit(t *testing.T) {
	m := NewManager()

	err := m.SetSpeedLimit(1024, 2048)
	if err != nil {
		t.Fatalf("set speed limit failed: %v", err)
	}

	// 测试负值
	err = m.SetSpeedLimit(-1, 1024)
	if err == nil {
		t.Error("expected error for negative upload")
	}

	err = m.SetSpeedLimit(1024, -1)
	if err == nil {
		t.Error("expected error for negative download")
	}
}

func TestFlushCache(t *testing.T) {
	m := NewManager()

	account := &CloudAccount{Provider: ProviderS3, Name: "S3账号"}
	m.RegisterAccount(account)

	point := &MountPoint{
		Name: "测试挂载", Provider: ProviderS3, Bucket: "mybucket",
		LocalPath: "/mnt/test", AccountID: account.ID,
	}
	m.Mount(point, nil)

	err := m.FlushCache(point.ID)
	if err != nil {
		t.Fatalf("flush cache failed: %v", err)
	}

	// 测试未挂载
	m.Unmount(point.ID)
	err = m.FlushCache(point.ID)
	if err == nil {
		t.Error("expected error for unmounted")
	}

	// 测试不存在的挂载点
	err = m.FlushCache("nonexistent")
	if err == nil {
		t.Error("expected error for nonexistent mount")
	}
}
