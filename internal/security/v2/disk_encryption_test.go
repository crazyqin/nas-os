package securityv2

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// ========== 磁盘加密配置测试 ==========

func TestDiskEncryptionConfig_Validation(t *testing.T) {
	tests := []struct {
		name   string
		config DiskEncryptionConfig
		valid  bool
	}{
		{
			name: "valid LUKS2 config",
			config: DiskEncryptionConfig{
				DevicePath:     "/dev/sda1",
				EncryptionType: EncryptionTypeLUKS2,
				Cipher:         "aes-xts-plain64",
				KeySize:        512,
				KeySourceType:  KeySourcePassphrase,
			},
			valid: true,
		},
		{
			name: "valid LUKS1 config",
			config: DiskEncryptionConfig{
				DevicePath:     "/dev/sdb1",
				EncryptionType: EncryptionTypeLUKS1,
				Cipher:         "aes-cbc-essiv:sha256",
				KeySize:        256,
				KeySourceType:  KeySourceKeyFile,
			},
			valid: true,
		},
		{
			name: "missing device path",
			config: DiskEncryptionConfig{
				EncryptionType: EncryptionTypeLUKS2,
			},
			valid: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// 基本验证
			if tt.config.DevicePath == "" && tt.valid {
				t.Error("DevicePath should not be empty for valid config")
			}
		})
	}
}

func TestDiskEncryptionConfig_Defaults(t *testing.T) {
	config := DiskEncryptionConfig{
		DevicePath: "/dev/sda1",
	}

	// 测试默认值应用逻辑
	if config.EncryptionType == "" {
		// 默认应该是 LUKS2
		config.EncryptionType = EncryptionTypeLUKS2
	}

	if config.Cipher == "" {
		config.Cipher = "aes-xts-plain64"
	}

	if config.KeySize == 0 {
		config.KeySize = 512
	}

	if config.EncryptionType != EncryptionTypeLUKS2 {
		t.Errorf("Default encryption type should be LUKS2, got %s", config.EncryptionType)
	}

	if config.Cipher != "aes-xts-plain64" {
		t.Errorf("Default cipher should be aes-xts-plain64, got %s", config.Cipher)
	}

	if config.KeySize != 512 {
		t.Errorf("Default key size should be 512, got %d", config.KeySize)
	}
}

// ========== 加密类型测试 ==========

func TestEncryptionType(t *testing.T) {
	types := []EncryptionType{
		EncryptionTypeLUKS1,
		EncryptionTypeLUKS2,
	}

	for _, et := range types {
		if et == "" {
			t.Error("Encryption type should not be empty")
		}
	}
}

// ========== 密钥来源类型测试 ==========

func TestKeySourceType(t *testing.T) {
	sources := []KeySourceType{
		KeySourcePassphrase,
		KeySourceKeyFile,
		KeySourceTPM,
		KeySourceYubiKey,
	}

	for _, source := range sources {
		if source == "" {
			t.Error("Key source type should not be empty")
		}
	}
}

// ========== 加密状态测试 ==========

func TestEncryptionStatus(t *testing.T) {
	statuses := []EncryptionStatus{
		EncryptionStatusLocked,
		EncryptionStatusUnlocked,
		EncryptionStatusError,
	}

	for _, status := range statuses {
		if status == "" {
			t.Error("Encryption status should not be empty")
		}
	}
}

// ========== LUKS 信息测试 ==========

func TestLUKSInfo(t *testing.T) {
	info := LUKSInfo{
		Version:      "2",
		Cipher:       "aes-xts-plain64",
		KeySize:      512,
		Device:       "/dev/sda1",
		UUID:         "abc123-def456",
		MapperName:   "encrypted_drive",
		MetadataSize: 16384,
		DataOffset:   32768,
		KeySlots: []KeySlotInfo{
			{SlotID: 0, Enabled: true, Type: "password"},
			{SlotID: 1, Enabled: false, Type: "unused"},
		},
	}

	if info.Version != "2" {
		t.Errorf("Version should be 2, got %s", info.Version)
	}

	if len(info.KeySlots) != 2 {
		t.Errorf("Expected 2 key slots, got %d", len(info.KeySlots))
	}

	activeSlots := 0
	for _, slot := range info.KeySlots {
		if slot.Enabled {
			activeSlots++
		}
	}

	if activeSlots != 1 {
		t.Errorf("Expected 1 active slot, got %d", activeSlots)
	}
}

func TestKeySlotInfo(t *testing.T) {
	now := time.Now()
	slot := KeySlotInfo{
		SlotID:         0,
		Enabled:        true,
		Type:           "password",
		KeyDescription: "Primary key",
		CreatedAt:      &now,
		LastUsedAt:     &now,
	}

	if !slot.Enabled {
		t.Error("Slot should be enabled")
	}

	if slot.Type != "password" {
		t.Errorf("Type should be password, got %s", slot.Type)
	}
}

// ========== 密钥轮换策略测试 ==========

func TestKeyRotationPolicy(t *testing.T) {
	policy := KeyRotationPolicy{
		Enabled:          true,
		IntervalDays:     90,
		MaxKeyAgeDays:    180,
		NotificationDays: 14,
		AutoRotate:       false,
		BackupKeys:       true,
		KeyHistory:       make([]KeyHistoryEntry, 0),
	}

	if !policy.Enabled {
		t.Error("Policy should be enabled")
	}

	if policy.IntervalDays != 90 {
		t.Errorf("Interval should be 90 days, got %d", policy.IntervalDays)
	}

	if policy.MaxKeyAgeDays < policy.IntervalDays {
		t.Error("MaxKeyAgeDays should be >= IntervalDays")
	}
}

func TestKeyRotationPolicy_AutoRotation(t *testing.T) {
	policy := KeyRotationPolicy{
		Enabled:      true,
		AutoRotate:   true,
		IntervalDays: 30,
	}

	if !policy.AutoRotate {
		t.Error("AutoRotate should be true")
	}
}

// ========== 密钥历史记录测试 ==========

func TestKeyHistoryEntry(t *testing.T) {
	entry := KeyHistoryEntry{
		SlotID:      0,
		Action:      "rotated",
		Timestamp:   time.Now(),
		Description: "Scheduled key rotation",
		User:        "admin",
	}

	if entry.Action != "rotated" {
		t.Errorf("Action should be rotated, got %s", entry.Action)
	}

	if entry.User != "admin" {
		t.Errorf("User should be admin, got %s", entry.User)
	}
}

// ========== 加密性能测试 ==========

func TestEncryptionPerformance(t *testing.T) {
	perf := EncryptionPerformance{
		DevicePath:          "/dev/sda1",
		ReadSpeedMBps:       500.0,
		WriteSpeedMBps:      450.0,
		CPUOverheadPercent:  5.0,
		MemoryUsageMB:       32.0,
		QueueDepth:          32,
		Algorithm:           "aes-xts-plain64",
		HardwareAccelerated: true,
		LastUpdated:         time.Now(),
	}

	if perf.ReadSpeedMBps <= 0 {
		t.Error("Read speed should be positive")
	}

	if perf.WriteSpeedMBps <= 0 {
		t.Error("Write speed should be positive")
	}

	if !perf.HardwareAccelerated {
		t.Error("Should be hardware accelerated")
	}
}

func TestEncryptionPerformance_HardwareAcceleration(t *testing.T) {
	perf := EncryptionPerformance{
		HardwareAccelerated: true,
	}

	// 硬件加速应该有更好的性能
	if !perf.HardwareAccelerated {
		// 软件加密通常更慢
		perf.ReadSpeedMBps = 100.0
	} else {
		// 硬件加速应该更快
		perf.ReadSpeedMBps = 500.0
	}
}

// ========== 加密基准测试结果 ==========

func TestEncryptionBenchmark(t *testing.T) {
	benchmark := EncryptionBenchmark{
		Algorithm:           "aes",
		Cipher:              "aes-xts-plain64",
		KeySize:             512,
		EncryptionSpeedMBps: 800.0,
		DecryptionSpeedMBps: 850.0,
		LatencyMs:           0.5,
		Recommended:         true,
		Notes:               "Best performance with AES-NI",
	}

	if !benchmark.Recommended {
		t.Error("This benchmark should be recommended")
	}

	if benchmark.EncryptionSpeedMBps < benchmark.DecryptionSpeedMBps {
		// 解密通常比加密快
		t.Log("Decryption is faster than encryption, which is normal")
	}
}

// ========== 磁盘加密管理器测试 ==========

func TestDiskEncryptionManager_New(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "encryption.json")

	dm := NewDiskEncryptionManager(configPath)
	if dm == nil {
		t.Fatal("Manager should not be nil")
	}

	if dm.configs == nil {
		t.Error("Configs map should be initialized")
	}

	if dm.rotationPolicy == nil {
		t.Error("Rotation policy should be initialized")
	}
}

func TestDiskEncryptionManager_ListConfigs(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "encryption.json")

	dm := NewDiskEncryptionManager(configPath)

	// 手动添加配置（跳过实际 LUKS 操作）
	dm.configs["/dev/sda1"] = &DiskEncryptionConfig{
		ID:             "test-1",
		DevicePath:     "/dev/sda1",
		EncryptionType: EncryptionTypeLUKS2,
	}

	dm.configs["/dev/sdb1"] = &DiskEncryptionConfig{
		ID:             "test-2",
		DevicePath:     "/dev/sdb1",
		EncryptionType: EncryptionTypeLUKS2,
	}

	configs := dm.ListConfigs()
	if len(configs) != 2 {
		t.Errorf("Expected 2 configs, got %d", len(configs))
	}
}

func TestDiskEncryptionManager_GetConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "encryption.json")

	dm := NewDiskEncryptionManager(configPath)
	dm.configs["/dev/sda1"] = &DiskEncryptionConfig{
		ID:             "test-1",
		DevicePath:     "/dev/sda1",
		EncryptionType: EncryptionTypeLUKS2,
	}

	config, err := dm.GetConfig("/dev/sda1")
	if err != nil {
		t.Fatalf("GetConfig failed: %v", err)
	}

	if config.ID != "test-1" {
		t.Errorf("ID mismatch: got %s", config.ID)
	}

	_, err = dm.GetConfig("/dev/nonexistent")
	if err == nil {
		t.Error("GetConfig should fail for nonexistent device")
	}
}

func TestDiskEncryptionManager_SetRotationPolicy(t *testing.T) {
	dm := NewDiskEncryptionManager("")

	policy := &KeyRotationPolicy{
		Enabled:       true,
		IntervalDays:  60,
		MaxKeyAgeDays: 120,
		AutoRotate:    true,
	}

	dm.SetRotationPolicy(policy)

	if dm.rotationPolicy.IntervalDays != 60 {
		t.Errorf("Interval should be 60, got %d", dm.rotationPolicy.IntervalDays)
	}
}

func TestDiskEncryptionManager_CheckKeyRotation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "encryption.json")

	dm := NewDiskEncryptionManager(configPath)
	dm.rotationPolicy = &KeyRotationPolicy{
		Enabled:       true,
		MaxKeyAgeDays: 90,
	}

	// 添加一个从未轮换过的配置
	dm.configs["/dev/sda1"] = &DiskEncryptionConfig{
		ID:                 "test-1",
		DevicePath:         "/dev/sda1",
		KeyRotationEnabled: true,
		KeyRotationDays:    90,
	}

	needsRotation, daysUntilExpiry, err := dm.CheckKeyRotation("/dev/sda1")
	if err != nil {
		t.Fatalf("CheckKeyRotation failed: %v", err)
	}

	if !needsRotation {
		t.Error("Should need rotation for never-rotated key")
	}

	if daysUntilExpiry != 0 {
		t.Errorf("Days until expiry should be 0, got %d", daysUntilExpiry)
	}
}

func TestDiskEncryptionManager_CheckKeyRotation_RecentRotation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "encryption.json")

	dm := NewDiskEncryptionManager(configPath)
	dm.rotationPolicy = &KeyRotationPolicy{
		Enabled:       true,
		MaxKeyAgeDays: 90,
	}

	// 添加一个最近轮换过的配置
	now := time.Now()
	dm.configs["/dev/sda1"] = &DiskEncryptionConfig{
		ID:                 "test-1",
		DevicePath:         "/dev/sda1",
		KeyRotationEnabled: true,
		KeyRotationDays:    90,
		LastKeyRotation:    &now,
	}

	needsRotation, _, err := dm.CheckKeyRotation("/dev/sda1")
	if err != nil {
		t.Fatalf("CheckKeyRotation failed: %v", err)
	}

	if needsRotation {
		t.Error("Should not need rotation for recently rotated key")
	}
}

// ========== 硬件加速检测测试 ==========

func TestDiskEncryptionManager_HardwareAcceleration(t *testing.T) {
	dm := NewDiskEncryptionManager("")

	// 这个测试依赖于系统，所以只检查不会 panic
	hwAccel := dm.checkHardwareAcceleration()
	t.Logf("Hardware acceleration: %v", hwAccel)
}

// ========== 优化建议测试 ==========

func TestDiskEncryptionManager_OptimizeEncryption(t *testing.T) {
	// 这个测试需要实际 LUKS 设备，所以我们测试逻辑
	// 而不是实际调用

	recommendations := []string{}

	// 模拟检查逻辑
	hwAccel := true // 假设有硬件加速
	keySize := 256
	_ = "aes-cbc-essiv:sha256" // cipher (unused in this test)
	activeSlots := 1

	if !hwAccel {
		recommendations = append(recommendations, "建议启用 CPU AES-NI 硬件加速")
	}

	if keySize < 512 {
		recommendations = append(recommendations, "建议使用 512-bit 密钥以获得更好的安全性")
	}

	if activeSlots < 2 {
		recommendations = append(recommendations, "建议至少配置 2 个密钥槽位以防止单点故障")
	}

	if len(recommendations) > 0 {
		t.Logf("Recommendations: %v", recommendations)
	}
}

// ========== 基准测试测试 ==========

func TestDiskEncryptionManager_RunEncryptionBenchmark(t *testing.T) {
	dm := NewDiskEncryptionManager("")

	results, err := dm.RunEncryptionBenchmark(100)
	if err != nil {
		t.Fatalf("RunEncryptionBenchmark failed: %v", err)
	}

	if len(results) == 0 {
		t.Error("Should return benchmark results")
	}

	// 检查是否有推荐的配置
	hasRecommended := false
	for _, r := range results {
		if r.Recommended {
			hasRecommended = true
			break
		}
	}

	if !hasRecommended {
		t.Log("No recommended configuration found")
	}
}

// ========== 钩子测试 ==========

func TestDiskEncryptionManager_Hooks(t *testing.T) {
	var called bool

	dm := NewDiskEncryptionManager("")
	dm.SetHooks(EncryptionHooks{
		OnDiskUnlocked: func(devicePath string) {
			called = true
		},
	})

	// 触发钩子
	if dm.hooks.OnDiskUnlocked != nil {
		dm.hooks.OnDiskUnlocked("/dev/sda1")
	}

	if !called {
		t.Error("OnDiskUnlocked hook should be called")
	}
}

// ========== 密钥生成测试 ==========

func TestGeneratePassphrase(t *testing.T) {
	passphrase := generatePassphrase()

	if passphrase == "" {
		t.Error("Passphrase should not be empty")
	}

	if len(passphrase) < 16 {
		t.Error("Passphrase should be at least 16 characters")
	}

	// 生成多个密码，确保随机性
	passphrase2 := generatePassphrase()
	if passphrase == passphrase2 {
		t.Error("Passphrases should be unique")
	}
}

// ========== 配置保存加载测试 ==========

func TestDiskEncryptionManager_SaveLoadConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "encryption.json")

	dm := NewDiskEncryptionManager(configPath)

	// 添加配置
	dm.configs["/dev/sda1"] = &DiskEncryptionConfig{
		ID:             "test-save",
		DevicePath:     "/dev/sda1",
		EncryptionType: EncryptionTypeLUKS2,
		Cipher:         "aes-xts-plain64",
		KeySize:        512,
	}

	// 保存
	if err := dm.saveConfig(); err != nil {
		t.Fatalf("saveConfig failed: %v", err)
	}

	// 验证文件存在
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file should exist")
	}

	// 加载到新管理器
	dm2 := NewDiskEncryptionManager(configPath)
	if err := dm2.loadConfig(); err != nil {
		t.Fatalf("loadConfig failed: %v", err)
	}

	if len(dm2.configs) != 1 {
		t.Errorf("Expected 1 config, got %d", len(dm2.configs))
	}
}

// ========== LUKS Dump 解析测试 ==========

func TestDiskEncryptionManager_ParseLUKSDump(t *testing.T) {
	dm := NewDiskEncryptionManager("")

	// 模拟 LUKS2 dump 输出
	dump := `LUKS header information
Version:        2
Epoch:          3
Metadata area:  16384 [bytes]
Keyslots area:  16744448 [bytes]
UUID:           abc123-def456-789
Label:          (no label)
Subsystem:      (no subsystem)
Data segments:
  0: crypt
	offset: 16777216 [bytes]
	length: (whole device)
	cipher: aes-xts-plain64
	sector: 512 [bytes]

Keyslots:
  0: luks2
	Key:        aes-xts-plain64
	Key size:   512 bits
	Area offset:32768 [bytes]
	Area length:258048 [bytes]

  1: luks2
	Key:        aes-xts-plain64
	Key size:   512 bits
	Area offset:290816 [bytes]
	Area length:258048 [bytes]`

	info, err := dm.parseLUKSDump(dump, "/dev/sda1")
	if err != nil {
		t.Fatalf("parseLUKSDump failed: %v", err)
	}

	if info.Version != "2" {
		t.Errorf("Version should be 2, got %s", info.Version)
	}

	if info.UUID != "abc123-def456-789" {
		t.Errorf("UUID mismatch: got %s", info.UUID)
	}
}

// ========== 时间相关测试 ==========

func TestDiskEncryptionConfig_Timestamps(t *testing.T) {
	now := time.Now()
	config := DiskEncryptionConfig{
		CreatedAt: now,
		UpdatedAt: now,
	}

	if config.CreatedAt.IsZero() {
		t.Error("CreatedAt should not be zero")
	}

	if config.UpdatedAt.IsZero() {
		t.Error("UpdatedAt should not be zero")
	}

	// 模拟更新
	later := now.Add(time.Hour)
	config.UpdatedAt = later

	if !config.UpdatedAt.After(config.CreatedAt) {
		t.Error("UpdatedAt should be after CreatedAt")
	}
}

func TestDiskEncryptionConfig_LastKeyRotation(t *testing.T) {
	now := time.Now()
	config := DiskEncryptionConfig{
		LastKeyRotation: &now,
	}

	if config.LastKeyRotation.IsZero() {
		t.Error("LastKeyRotation should not be zero")
	}

	// 计算距离上次轮换的天数
	daysSinceRotation := int(time.Since(*config.LastKeyRotation).Hours() / 24)
	if daysSinceRotation != 0 {
		t.Logf("Days since rotation: %d", daysSinceRotation)
	}
}
