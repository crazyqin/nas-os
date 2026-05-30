package vaultencryption

import (
	"testing"
	"time"
)

func TestNewVaultEncryptionManager(t *testing.T) {
	manager := NewVaultEncryptionManager()
	if manager == nil {
		t.Fatal("NewVaultEncryptionManager 返回 nil")
	}

	if manager.keys == nil {
		t.Fatal("keys map 未初始化")
	}

	if manager.volumes == nil {
		t.Fatal("volumes map 未初始化")
	}

	if manager.auditLogs == nil {
		t.Fatal("auditLogs 未初始化")
	}

	if manager.lockTimers == nil {
		t.Fatal("lockTimers 未初始化")
	}

	if manager.retryCounts == nil {
		t.Fatal("retryCounts 未初始化")
	}
}

func TestCreateKey(t *testing.T) {
	manager := NewVaultEncryptionManager()

	req := CreateKeyRequest{
		Name:        "Test Key",
		Description: "测试密钥",
		Password:    "testpassword123",
		ExpiresIn:   30,
	}

	key, err := manager.CreateKey(req)
	if err != nil {
		t.Fatalf("创建密钥失败: %v", err)
	}

	if key.ID == "" {
		t.Error("密钥ID不能为空")
	}

	if key.Name != "Test Key" {
		t.Errorf("期望密钥名称 'Test Key', 实际 '%s'", key.Name)
	}

	if key.KeyHash == "" {
		t.Error("密钥哈希不能为空")
	}

	if key.Salt == "" {
		t.Error("盐值不能为空")
	}

	if !key.IsActive {
		t.Error("密钥应该激活")
	}

	if key.ExpiresAt == nil {
		t.Error("过期时间应该设置")
	}

	// 测试创建无名称密钥
	invalidReq := CreateKeyRequest{
		Password: "test123",
	}
	_, err = manager.CreateKey(invalidReq)
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}

	// 测试创建空密码密钥
	invalidReq = CreateKeyRequest{
		Name:     "Test",
		Password: "",
	}
	_, err = manager.CreateKey(invalidReq)
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}

	// 测试创建短密码密钥
	invalidReq = CreateKeyRequest{
		Name:     "Test",
		Password: "short",
	}
	_, err = manager.CreateKey(invalidReq)
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestDeleteKey(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 先创建密钥
	req := CreateKeyRequest{
		Name:     "To Delete",
		Password: "testpassword123",
	}
	key, _ := manager.CreateKey(req)

	// 删除密钥
	err := manager.DeleteKey(key.ID)
	if err != nil {
		t.Fatalf("删除密钥失败: %v", err)
	}

	// 验证密钥已删除（应该是非激活状态）
	deletedKey, _ := manager.GetKey(key.ID)
	if deletedKey.IsActive {
		t.Error("密钥应该已被删除（非激活）")
	}

	// 测试删除不存在的密钥
	err = manager.DeleteKey("non-existent")
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestGetKey(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 创建密钥
	req := CreateKeyRequest{
		Name:     "Test Key",
		Password: "testpassword123",
	}
	created, _ := manager.CreateKey(req)

	// 获取密钥
	key, err := manager.GetKey(created.ID)
	if err != nil {
		t.Fatalf("获取密钥失败: %v", err)
	}

	if key.ID != created.ID {
		t.Errorf("期望密钥ID '%s', 实际 '%s'", created.ID, key.ID)
	}

	// 测试获取不存在的密钥
	_, err = manager.GetKey("non-existent")
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestListKeys(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 创建多个密钥
	for i := 0; i < 3; i++ {
		req := CreateKeyRequest{
			Name:     "Key " + string(rune('A'+i)),
			Password: "testpassword123",
		}
		manager.CreateKey(req)
	}

	// 列出密钥
	keys := manager.ListKeys()

	if len(keys) != 3 {
		t.Errorf("期望3个密钥, 实际 %d", len(keys))
	}
}

func TestChangePassword(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 创建密钥
	req := CreateKeyRequest{
		Name:     "Test Key",
		Password: "oldpassword123",
	}
	key, _ := manager.CreateKey(req)

	// 保存原始哈希
	originalHash := key.KeyHash

	// 修改密码
	changeReq := ChangePasswordRequest{
		KeyID:       key.ID,
		OldPassword: "oldpassword123",
		NewPassword: "newpassword456",
	}

	err := manager.ChangePassword(changeReq)
	if err != nil {
		t.Fatalf("修改密码失败: %v", err)
	}

	// 验证密码已修改（检查哈希是否不同）
	updatedKey, _ := manager.GetKey(key.ID)
	if updatedKey.KeyHash == originalHash {
		t.Error("密码哈希应该已更新")
	}

	// 测试旧密码错误
	changeReq = ChangePasswordRequest{
		KeyID:       key.ID,
		OldPassword: "wrongpassword",
		NewPassword: "newpassword789",
	}

	err = manager.ChangePassword(changeReq)
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}

	// 测试新密码太短
	changeReq = ChangePasswordRequest{
		KeyID:       key.ID,
		OldPassword: "newpassword456",
		NewPassword: "short",
	}

	err = manager.ChangePassword(changeReq)
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestRegisterVolume(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 创建密钥
	keyReq := CreateKeyRequest{
		Name:     "Test Key",
		Password: "testpassword123",
	}
	key, _ := manager.CreateKey(keyReq)

	// 注册卷
	vol := &EncryptedVolume{
		Name:           "Test Volume",
		Device:         "/dev/sda1",
		MountPoint:     "/mnt/volume1",
		FileSystem:     "ext4",
		TotalSize:      1024 * 1024 * 1024 * 100, // 100GB
		EncryptionAlgo: "aes-256-xts",
		KeyID:          key.ID,
	}

	err := manager.RegisterVolume(vol)
	if err != nil {
		t.Fatalf("注册卷失败: %v", err)
	}

	if vol.ID == "" {
		t.Error("卷ID不能为空")
	}

	if !vol.IsLocked {
		t.Error("新注册的卷应该是锁定状态")
	}

	// 测试注册无名称卷
	invalidVol := &EncryptedVolume{
		Device: "/dev/sdb1",
	}
	err = manager.RegisterVolume(invalidVol)
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestUnregisterVolume(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 创建密钥并注册卷
	keyReq := CreateKeyRequest{
		Name:     "Test Key",
		Password: "testpassword123",
	}
	key, _ := manager.CreateKey(keyReq)

	vol := &EncryptedVolume{
		Name:   "To Delete",
		KeyID:  key.ID,
	}
	manager.RegisterVolume(vol)

	// 注销卷
	err := manager.UnregisterVolume(vol.ID)
	if err != nil {
		t.Fatalf("注销卷失败: %v", err)
	}

	// 验证卷已注销
	_, err = manager.GetVolume(vol.ID)
	if err == nil {
		t.Fatal("卷应该已被注销")
	}

	// 测试注销不存在的卷
	err = manager.UnregisterVolume("non-existent")
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestGetVolume(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 创建密钥并注册卷
	keyReq := CreateKeyRequest{
		Name:     "Test Key",
		Password: "testpassword123",
	}
	key, _ := manager.CreateKey(keyReq)

	vol := &EncryptedVolume{
		Name:   "Test Volume",
		KeyID:  key.ID,
	}
	manager.RegisterVolume(vol)

	// 获取卷
	volume, err := manager.GetVolume(vol.ID)
	if err != nil {
		t.Fatalf("获取卷失败: %v", err)
	}

	if volume.Name != "Test Volume" {
		t.Errorf("期望卷名称 'Test Volume', 实际 '%s'", volume.Name)
	}

	// 测试获取不存在的卷
	_, err = manager.GetVolume("non-existent")
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestListVolumes(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 创建密钥
	keyReq := CreateKeyRequest{
		Name:     "Test Key",
		Password: "testpassword123",
	}
	key, _ := manager.CreateKey(keyReq)

	// 注册多个卷
	for i := 0; i < 3; i++ {
		vol := &EncryptedVolume{
			Name:  "Volume " + string(rune('A'+i)),
			KeyID: key.ID,
		}
		manager.RegisterVolume(vol)
	}

	// 列出卷
	volumes := manager.ListVolumes()

	if len(volumes) != 3 {
		t.Errorf("期望3个卷, 实际 %d", len(volumes))
	}
}

func TestUnlockVolume(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 创建密钥
	keyReq := CreateKeyRequest{
		Name:     "Test Key",
		Password: "testpassword123",
	}
	key, _ := manager.CreateKey(keyReq)

	// 注册卷
	vol := &EncryptedVolume{
		Name:   "Test Volume",
		KeyID:  key.ID,
		MountPoint: "/mnt/test",
	}
	manager.RegisterVolume(vol)

	// 测试解锁
	unlockReq := UnlockRequest{
		VolumeID: vol.ID,
		Password: "testpassword123",
	}

	result, err := manager.UnlockVolume(unlockReq)
	if err != nil {
		t.Fatalf("解锁失败: %v", err)
	}

	if !result.Success {
		t.Error("解锁应该成功")
	}

	if result.MountPoint != "/mnt/test" {
		t.Errorf("期望挂载点 '/mnt/test', 实际 '%s'", result.MountPoint)
	}

	// 验证卷已解锁
	updatedVol, _ := manager.GetVolume(vol.ID)
	if updatedVol.IsLocked {
		t.Error("卷应该已解锁")
	}

	// 测试再次解锁（应该返回已解锁）
	result, err = manager.UnlockVolume(unlockReq)
	if err != nil {
		t.Fatalf("再次解锁应该成功: %v", err)
	}

	if !result.Success {
		t.Error("再次解锁应该成功")
	}
}

func TestUnlockVolumeWrongPassword(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 创建密钥
	keyReq := CreateKeyRequest{
		Name:     "Test Key",
		Password: "correctpassword",
	}
	key, _ := manager.CreateKey(keyReq)

	// 注册卷
	vol := &EncryptedVolume{
		Name:  "Test Volume",
		KeyID: key.ID,
	}
	manager.RegisterVolume(vol)

	// 测试错误密码
	unlockReq := UnlockRequest{
		VolumeID: vol.ID,
		Password: "wrongpassword",
	}

	result, err := manager.UnlockVolume(unlockReq)
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}

	if result.Success {
		t.Error("解锁应该失败")
	}
}

func TestUnlockVolumeMaxRetries(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 创建密钥
	keyReq := CreateKeyRequest{
		Name:     "Test Key",
		Password: "correctpassword",
	}
	key, _ := manager.CreateKey(keyReq)

	// 注册卷
	vol := &EncryptedVolume{
		Name:  "Test Volume",
		KeyID: key.ID,
	}
	manager.RegisterVolume(vol)

	// 多次尝试错误密码
	for i := 0; i < 5; i++ {
		unlockReq := UnlockRequest{
			VolumeID: vol.ID,
			Password: "wrongpassword",
		}
		manager.UnlockVolume(unlockReq)
	}

	// 再次尝试应该被锁定
	unlockReq := UnlockRequest{
		VolumeID: vol.ID,
		Password: "wrongpassword",
	}

	_, err := manager.UnlockVolume(unlockReq)
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestLockVolume(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 创建密钥
	keyReq := CreateKeyRequest{
		Name:     "Test Key",
		Password: "testpassword123",
	}
	key, _ := manager.CreateKey(keyReq)

	// 注册卷
	vol := &EncryptedVolume{
		Name:   "Test Volume",
		KeyID:  key.ID,
	}
	manager.RegisterVolume(vol)

	// 先解锁
	manager.UnlockVolume(UnlockRequest{
		VolumeID: vol.ID,
		Password: "testpassword123",
	})

	// 锁定卷
	err := manager.LockVolume(LockRequest{
		VolumeID: vol.ID,
	})
	if err != nil {
		t.Fatalf("锁定失败: %v", err)
	}

	// 验证卷已锁定
	updatedVol, _ := manager.GetVolume(vol.ID)
	if !updatedVol.IsLocked {
		t.Error("卷应该已锁定")
	}

	// 测试再次锁定
	err = manager.LockVolume(LockRequest{
		VolumeID: vol.ID,
	})
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}

	// 测试锁定不存在的卷
	err = manager.LockVolume(LockRequest{
		VolumeID: "non-existent",
	})
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestGetAuditLogs(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 创建密钥并执行一些操作
	keyReq := CreateKeyRequest{
		Name:     "Test Key",
		Password: "testpassword123",
	}
	key, _ := manager.CreateKey(keyReq)

	vol := &EncryptedVolume{
		Name:  "Test Volume",
		KeyID: key.ID,
	}
	manager.RegisterVolume(vol)

	manager.UnlockVolume(UnlockRequest{
		VolumeID: vol.ID,
		Password: "testpassword123",
	})

	// 获取审计日志
	logs := manager.GetAuditLogs(10)

	if len(logs) == 0 {
		t.Error("应该有审计日志")
	}

	// 验证日志内容
	foundUnlock := false
	for _, log := range logs {
		if log.Action == ActionUnlock && log.Success {
			foundUnlock = true
			break
		}
	}

	if !foundUnlock {
		t.Error("应该找到解锁成功的审计日志")
	}
}

func TestGetStats(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 创建密钥
	keyReq := CreateKeyRequest{
		Name:     "Test Key",
		Password: "testpassword123",
	}
	key, _ := manager.CreateKey(keyReq)

	// 注册卷
	vol := &EncryptedVolume{
		Name:  "Test Volume",
		KeyID: key.ID,
	}
	manager.RegisterVolume(vol)

	// 获取统计
	stats := manager.GetStats()

	if stats.TotalKeys != 1 {
		t.Errorf("期望1个密钥, 实际 %d", stats.TotalKeys)
	}

	if stats.ActiveKeys != 1 {
		t.Errorf("期望1个激活密钥, 实际 %d", stats.ActiveKeys)
	}

	if stats.TotalVolumes != 1 {
		t.Errorf("期望1个卷, 实际 %d", stats.TotalVolumes)
	}

	if stats.LockedVolumes != 1 {
		t.Errorf("期望1个锁定卷, 实际 %d", stats.LockedVolumes)
	}

	// 解锁卷后检查统计
	manager.UnlockVolume(UnlockRequest{
		VolumeID: vol.ID,
		Password: "testpassword123",
	})

	stats = manager.GetStats()

	if stats.UnlockedVolumes != 1 {
		t.Errorf("期望1个解锁卷, 实际 %d", stats.UnlockedVolumes)
	}

	if stats.LockedVolumes != 0 {
		t.Errorf("期望0个锁定卷, 实际 %d", stats.LockedVolumes)
	}
}

func TestConfig(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 获取默认配置
	config := manager.GetConfig()
	if config.AutoLockTimeout == 0 {
		t.Error("默认自动锁定超时应该大于0")
	}

	if config.MaxRetryAttempts == 0 {
		t.Error("默认最大重试次数应该大于0")
	}

	// 设置新配置
	newConfig := VaultConfig{
		AutoLockTimeout:  60 * time.Minute,
		MaxRetryAttempts: 10,
		RetryLockout:     30 * time.Minute,
		KeyDerivation:    "scrypt",
	}

	manager.SetConfig(newConfig)

	// 验证配置
	config = manager.GetConfig()
	if config.AutoLockTimeout != 60*time.Minute {
		t.Errorf("期望自动锁定超时 60分钟, 实际 %v", config.AutoLockTimeout)
	}

	if config.MaxRetryAttempts != 10 {
		t.Errorf("期望最大重试次数 10, 实际 %d", config.MaxRetryAttempts)
	}

	if config.KeyDerivation != "scrypt" {
		t.Errorf("期望密钥派生算法 'scrypt', 实际 '%s'", config.KeyDerivation)
	}
}

func TestNewModule(t *testing.T) {
	module := NewModule()
	if module == nil {
		t.Fatal("NewModule 返回 nil")
	}

	if module.Name() != "vaultencryption" {
		t.Errorf("期望模块名称 'vaultencryption', 实际 '%s'", module.Name())
	}

	if !module.IsEnabled() {
		t.Error("模块应该默认启用")
	}

	// 测试禁用
	module.Disable()
	if module.IsEnabled() {
		t.Error("模块应该被禁用")
	}

	// 测试启用
	module.Enable()
	if !module.IsEnabled() {
		t.Error("模块应该被启用")
	}
}

func TestDeleteKeyWithActiveVolume(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 创建密钥
	keyReq := CreateKeyRequest{
		Name:     "Test Key",
		Password: "testpassword123",
	}
	key, _ := manager.CreateKey(keyReq)

	// 注册卷
	vol := &EncryptedVolume{
		Name:  "Test Volume",
		KeyID: key.ID,
	}
	manager.RegisterVolume(vol)

	// 解锁卷
	manager.UnlockVolume(UnlockRequest{
		VolumeID: vol.ID,
		Password: "testpassword123",
	})

	// 尝试删除密钥（应该失败，因为卷在使用）
	err := manager.DeleteKey(key.ID)
	if err == nil {
		t.Fatal("应该返回错误但没有")
	}
}

func TestAutoLockTimeout(t *testing.T) {
	manager := NewVaultEncryptionManager()

	// 设置短超时用于测试
	config := DefaultVaultConfig()
	config.AutoLockTimeout = 100 * time.Millisecond
	manager.SetConfig(config)

	// 创建密钥
	keyReq := CreateKeyRequest{
		Name:     "Test Key",
		Password: "testpassword123",
	}
	key, _ := manager.CreateKey(keyReq)

	// 注册卷
	vol := &EncryptedVolume{
		Name:   "Test Volume",
		KeyID:  key.ID,
	}
	manager.RegisterVolume(vol)

	// 解锁卷
	manager.UnlockVolume(UnlockRequest{
		VolumeID: vol.ID,
		Password: "testpassword123",
	})

	// 验证卷已解锁
	updatedVol, _ := manager.GetVolume(vol.ID)
	if updatedVol.IsLocked {
		t.Error("卷应该已解锁")
	}

	// 等待自动锁定
	time.Sleep(200 * time.Millisecond)

	// 验证卷已自动锁定
	updatedVol, _ = manager.GetVolume(vol.ID)
	if !updatedVol.IsLocked {
		t.Error("卷应该已自动锁定")
	}
}
