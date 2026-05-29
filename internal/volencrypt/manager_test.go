package volencrypt

import (
	"context"
	"testing"
	"time"
)

func TestManager_CreateVolume(t *testing.T) {
	config := DefaultConfig()
	mgr := NewManager(config)

	volume, err := mgr.CreateVolume("test-vol", "/mnt/volumes/test", 1024*1024*1024)
	if err != nil {
		t.Fatalf("创建卷失败: %v", err)
	}

	if volume.Name != "test-vol" {
		t.Errorf("卷名不匹配: %s", volume.Name)
	}

	if volume.Status != StatusUnencrypted {
		t.Errorf("初始状态应为 unencrypted，实际 %s", volume.Status)
	}

	if volume.Algorithm != "AES-256-XTS" {
		t.Errorf("算法不匹配: %s", volume.Algorithm)
	}
}

func TestManager_CreateDuplicateVolume(t *testing.T) {
	config := DefaultConfig()
	mgr := NewManager(config)

	_, err := mgr.CreateVolume("test-vol", "/mnt/volumes/test", 1024)
	if err != nil {
		t.Fatalf("创建卷失败: %v", err)
	}

	_, err = mgr.CreateVolume("test-vol", "/mnt/volumes/test2", 2048)
	if err == nil {
		t.Fatal("重复卷名应返回错误")
	}
}

func TestManager_EncryptDecrypt(t *testing.T) {
	config := DefaultConfig()
	mgr := NewManager(config)
	ctx := context.Background()

	mgr.Start(ctx)
	defer mgr.Stop()

	volume, _ := mgr.CreateVolume("enc-vol", "/mnt/volumes/enc", 1024*1024)

	// 加密
	err := mgr.EncryptVolume(ctx, volume.ID)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	// 等待加密完成
	time.Sleep(2 * time.Second)

	v, _ := mgr.GetVolume(volume.ID)
	if v.Status != StatusEncrypted {
		t.Errorf("加密后状态应为 encrypted，实际 %s", v.Status)
	}

	// 解密
	err = mgr.DecryptVolume(ctx, volume.ID)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}

	// 等待解密完成
	time.Sleep(2 * time.Second)

	v, _ = mgr.GetVolume(volume.ID)
	if v.Status != StatusUnencrypted {
		t.Errorf("解密后状态应为 unencrypted，实际 %s", v.Status)
	}
}

func TestManager_LockUnlock(t *testing.T) {
	config := DefaultConfig()
	mgr := NewManager(config)
	ctx := context.Background()
	mgr.Start(ctx)
	defer mgr.Stop()

	volume, _ := mgr.CreateVolume("lock-vol", "/mnt/volumes/lock", 1024*1024)

	// 先加密
	mgr.EncryptVolume(ctx, volume.ID)
	time.Sleep(2 * time.Second)

	// 锁定
	err := mgr.LockVolume(volume.ID)
	if err != nil {
		t.Fatalf("锁定失败: %v", err)
	}

	v, _ := mgr.GetVolume(volume.ID)
	if v.Status != StatusLocked {
		t.Errorf("锁定后状态应为 locked，实际 %s", v.Status)
	}

	// 解锁
	err = mgr.UnlockVolume(volume.ID, "/mnt/unlocked")
	if err != nil {
		t.Fatalf("解锁失败: %v", err)
	}

	v, _ = mgr.GetVolume(volume.ID)
	if v.MountPoint != "/mnt/unlocked" {
		t.Errorf("挂载点不匹配: %s", v.MountPoint)
	}
}

func TestManager_RotateKey(t *testing.T) {
	config := DefaultConfig()
	mgr := NewManager(config)

	volume, _ := mgr.CreateVolume("key-vol", "/mnt/volumes/key", 1024)
	oldKeyID := volume.KeyID

	err := mgr.RotateKey(volume.ID)
	if err != nil {
		t.Fatalf("密钥轮换失败: %v", err)
	}

	v, _ := mgr.GetVolume(volume.ID)
	if v.KeyID == oldKeyID {
		t.Error("密钥轮换后 ID 应该不同")
	}
}

func TestManager_AuditLog(t *testing.T) {
	config := DefaultConfig()
	mgr := NewManager(config)

	// 执行一些操作
	mgr.CreateVolume("audit-vol", "/mnt/volumes/audit", 1024)
	mgr.CreateVolume("audit-vol2", "/mnt/volumes/audit2", 2048)

	auditLog := mgr.GetAuditLog(10)
	if len(auditLog) < 2 {
		t.Errorf("审计日志应至少有 2 条记录，实际 %d", len(auditLog))
	}
}

func TestManager_ListVolumes(t *testing.T) {
	config := DefaultConfig()
	mgr := NewManager(config)

	mgr.CreateVolume("vol1", "/mnt/v1", 1024)
	mgr.CreateVolume("vol2", "/mnt/v2", 2048)
	mgr.CreateVolume("vol3", "/mnt/v3", 4096)

	volumes := mgr.ListVolumes()
	if len(volumes) != 3 {
		t.Errorf("应有 3 个卷，实际 %d", len(volumes))
	}
}

func TestManager_Stats(t *testing.T) {
	config := DefaultConfig()
	mgr := NewManager(config)

	mgr.CreateVolume("stat-vol", "/mnt/volumes/stat", 1024)

	stats := mgr.GetStats()
	if stats["total_volumes"] != 1 {
		t.Errorf("总卷数应为 1，实际 %v", stats["total_volumes"])
	}
}

func TestEncryptDecryptData(t *testing.T) {
	key := make([]byte, 32) // AES-256
	for i := range key {
		key[i] = byte(i)
	}

	plaintext := []byte("Hello, NAS-OS! 这是加密测试数据。")

	// 加密
	ciphertext, err := EncryptData(key, plaintext)
	if err != nil {
		t.Fatalf("加密失败: %v", err)
	}

	// 解密
	decrypted, err := DecryptData(key, ciphertext)
	if err != nil {
		t.Fatalf("解密失败: %v", err)
	}

	if string(decrypted) != string(plaintext) {
		t.Errorf("解密数据不匹配: %s vs %s", decrypted, plaintext)
	}
}

func TestVolumeStatus_String(t *testing.T) {
	tests := []struct {
		status   VolumeStatus
		expected string
	}{
		{StatusUnencrypted, "unencrypted"},
		{StatusEncrypting, "encrypting"},
		{StatusEncrypted, "encrypted"},
		{StatusDecrypting, "decrypting"},
		{StatusError, "error"},
		{StatusLocked, "locked"},
	}

	for _, tt := range tests {
		if string(tt.status) != tt.expected {
			t.Errorf("VolumeStatus %s != %s", tt.status, tt.expected)
		}
	}
}
