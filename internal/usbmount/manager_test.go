package usbmount

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ========== Manager 测试 ==========

func TestNewManager(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	assert.NotNil(t, m)
	assert.NotNil(t, m.config)
	assert.NotNil(t, m.devices)
	assert.NotNil(t, m.rules)
	assert.True(t, m.config.AutoMount)
	assert.Equal(t, "/media", m.config.DefaultMountPoint)
}

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.True(t, config.AutoMount)
	assert.Equal(t, "/media", config.DefaultMountPoint)
	assert.Contains(t, config.AllowedFileSystems, "vfat")
	assert.Contains(t, config.AllowedFileSystems, "ntfs")
	assert.Contains(t, config.AllowedFileSystems, "ext4")
	assert.Equal(t, 5, config.ScanInterval)
}

func TestManager_StartStop(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	// 启动
	err := m.Start()
	assert.NoError(t, err)
	assert.True(t, m.IsRunning())

	// 重复启动
	err = m.Start()
	assert.NoError(t, err)

	// 停止
	m.Stop()
	assert.False(t, m.IsRunning())

	// 重复停止
	m.Stop()
	assert.False(t, m.IsRunning())
}

func TestManager_ListDevices(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	// 空列表
	devices := m.ListDevices()
	assert.Empty(t, devices)

	// 添加设备
	m.mu.Lock()
	m.devices["test-1"] = &Device{
		ID:         "test-1",
		DevicePath: "/dev/sdb1",
		Label:      "TestDrive",
		Type:       "vfat",
	}
	m.mu.Unlock()

	devices = m.ListDevices()
	assert.Len(t, devices, 1)
	assert.Equal(t, "test-1", devices[0].ID)
}

func TestManager_GetDevice(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	// 设备不存在
	_, err := m.GetDevice("nonexistent")
	assert.ErrorIs(t, err, ErrDeviceNotFound)

	// 添加设备
	m.mu.Lock()
	m.devices["test-1"] = &Device{
		ID:         "test-1",
		DevicePath: "/dev/sdb1",
		Label:      "TestDrive",
		Type:       "vfat",
	}
	m.mu.Unlock()

	// 获取设备
	device, err := m.GetDevice("test-1")
	assert.NoError(t, err)
	assert.Equal(t, "test-1", device.ID)
	assert.Equal(t, "TestDrive", device.Label)
}

// ========== 规则测试 ==========

func TestManager_AddRule(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	rule := &MountRule{
		Name:      "Test Rule",
		Priority:  10,
		MatchUUID: "test-uuid-*",
		AutoMount: true,
		Enabled:   true,
	}

	err := m.AddRule(rule)
	assert.NoError(t, err)
	assert.NotEmpty(t, rule.ID)
	assert.False(t, rule.CreatedAt.IsZero())

	rules := m.ListRules()
	assert.Len(t, rules, 1)
}

func TestManager_UpdateRule(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	// 先添加规则
	rule := &MountRule{
		Name:      "Test Rule",
		Priority:  10,
		AutoMount: true,
		Enabled:   true,
	}
	m.AddRule(rule)

	// 更新规则
	updatedRule := &MountRule{
		Name:      "Updated Rule",
		Priority:  20,
		AutoMount: false,
		Enabled:   true,
	}

	err := m.UpdateRule(rule.ID, updatedRule)
	assert.NoError(t, err)

	// 验证更新
	rules := m.ListRules()
	assert.Equal(t, "Updated Rule", rules[0].Name)
	assert.Equal(t, 20, rules[0].Priority)

	// 更新不存在的规则
	err = m.UpdateRule("nonexistent", updatedRule)
	assert.ErrorIs(t, err, ErrRuleNotFound)
}

func TestManager_DeleteRule(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	// 添加规则
	rule := &MountRule{
		Name:     "Test Rule",
		Enabled:  true,
	}
	m.AddRule(rule)

	// 删除规则
	err := m.DeleteRule(rule.ID)
	assert.NoError(t, err)

	rules := m.ListRules()
	assert.Empty(t, rules)

	// 删除不存在的规则
	err = m.DeleteRule("nonexistent")
	assert.ErrorIs(t, err, ErrRuleNotFound)
}

func TestManager_GetRule(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	// 不存在
	_, err := m.GetRule("nonexistent")
	assert.ErrorIs(t, err, ErrRuleNotFound)

	// 添加规则
	rule := &MountRule{
		Name:    "Test Rule",
		Enabled: true,
	}
	m.AddRule(rule)

	// 获取规则
	found, err := m.GetRule(rule.ID)
	assert.NoError(t, err)
	assert.Equal(t, rule.ID, found.ID)
}

// ========== 规则匹配测试 ==========

func TestManager_MatchRule(t *testing.T) {
	m := &Manager{}

	tests := []struct {
		name     string
		rule     *MountRule
		device   *Device
		expected bool
	}{
		{
			name: "匹配 UUID",
			rule: &MountRule{
				MatchUUID: "test-uuid-123",
			},
			device: &Device{
				UUID: "test-uuid-123",
			},
			expected: true,
		},
		{
			name: "UUID 通配符匹配",
			rule: &MountRule{
				MatchUUID: "test-*",
			},
			device: &Device{
				UUID: "test-uuid-123",
			},
			expected: true,
		},
		{
			name: "UUID 不匹配",
			rule: &MountRule{
				MatchUUID: "other-*",
			},
			device: &Device{
				UUID: "test-uuid-123",
			},
			expected: false,
		},
		{
			name: "匹配 Label",
			rule: &MountRule{
				MatchLabel: "MyDrive",
			},
			device: &Device{
				Label: "MyDrive",
			},
			expected: true,
		},
		{
			name: "匹配文件系统类型",
			rule: &MountRule{
				MatchType: "vfat",
			},
			device: &Device{
				Type: "vfat",
			},
			expected: true,
		},
		{
			name: "匹配厂商",
			rule: &MountRule{
				MatchVendor: "SanDisk",
			},
			device: &Device{
				Vendor: "SanDisk",
			},
			expected: true,
		},
		{
			name: "多条件匹配",
			rule: &MountRule{
				MatchUUID:   "test-*",
				MatchType:   "vfat",
				MatchVendor: "SanDisk",
			},
			device: &Device{
				UUID:   "test-123",
				Type:   "vfat",
				Vendor: "SanDisk",
			},
			expected: true,
		},
		{
			name: "部分匹配失败",
			rule: &MountRule{
				MatchUUID: "test-*",
				MatchType: "ext4",
			},
			device: &Device{
				UUID: "test-123",
				Type: "vfat",
			},
			expected: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.matchRule(tt.rule, tt.device)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestMatchPattern(t *testing.T) {
	tests := []struct {
		pattern string
		value   string
		expected bool
	}{
		{"*", "anything", true},
		{"", "", true},
		{"exact", "exact", true},
		{"exact", "different", false},
		{"prefix*", "prefix-suffix", true},
		{"*suffix", "prefix-suffix", true},
		{"test-*", "test-123", true},
		{"test-*", "other-123", false},
	}

	for _, tt := range tests {
		t.Run(tt.pattern+"_"+tt.value, func(t *testing.T) {
			result := matchPattern(tt.pattern, tt.value)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== 挂载点生成测试 ==========

func TestManager_GenerateMountPoint(t *testing.T) {
	m := &Manager{
		config: DefaultConfig(),
	}

	tests := []struct {
		name     string
		device   *Device
		expected string
	}{
		{
			name: "使用 Label",
			device: &Device{
				Label:      "MyDrive",
				DevicePath: "/dev/sdb1",
			},
			expected: "/media/MyDrive",
		},
		{
			name: "使用 UUID",
			device: &Device{
				UUID:       "12345678-1234-1234-1234-123456789abc",
				DevicePath: "/dev/sdb1",
			},
			expected: "/media/12345678",
		},
		{
			name: "使用设备路径",
			device: &Device{
				DevicePath: "/dev/sdb1",
			},
			expected: "/media/sdb1",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.generateMountPoint(tt.device)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestSanitizeMountName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"SimpleName", "SimpleName"},
		{"Name With Spaces", "Name_With_Spaces"},
		{"Name/With/Slashes", "Name_With_Slashes"},
		{"Name:With:Colons", "Name_With_Colons"},
		{"Name-With-Dashes", "Name-With-Dashes"},
		{"Name_With_Underscores", "Name_With_Underscores"},
		{"Name@#$%^&*()", "Name_________"},
		{"", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := sanitizeMountName(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== 文件系统检查测试 ==========

func TestManager_IsFileSystemAllowed(t *testing.T) {
	m := &Manager{
		config: DefaultConfig(),
	}

	tests := []struct {
		fsType   string
		expected bool
	}{
		{"vfat", true},
		{"ntfs", true},
		{"exfat", true},
		{"ext4", true},
		{"btrfs", true},
		{"xfs", true},
		{"swap", false},
		{"unknown", false},
	}

	for _, tt := range tests {
		t.Run(tt.fsType, func(t *testing.T) {
			result := m.isFileSystemAllowed(tt.fsType)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== 配置测试 ==========

func TestManager_GetConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	config := m.GetConfig()
	assert.NotNil(t, config)
	assert.True(t, config.AutoMount)
}

func TestManager_UpdateConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	newConfig := &Config{
		AutoMount:         false,
		DefaultMountPoint: "/mnt/usb",
		ScanInterval:      10,
	}

	err := m.UpdateConfig(newConfig)
	assert.NoError(t, err)

	config := m.GetConfig()
	assert.False(t, config.AutoMount)
	assert.Equal(t, "/mnt/usb", config.DefaultMountPoint)
	assert.Equal(t, 10, config.ScanInterval)
}

// ========== 事件处理测试 ==========

func TestManager_AddEventHandler(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	eventReceived := false
	m.AddEventHandler(func(event DeviceEvent) {
		eventReceived = true
	})

	// 启动管理器处理事件
	m.Start()
	defer m.Stop()

	// 发送事件
	m.emitEvent(DeviceEvent{
		Type:      EventDeviceAdded,
		Timestamp: time.Now(),
	})

	// 等待事件处理
	time.Sleep(100 * time.Millisecond)

	assert.True(t, eventReceived)
}

// ========== 挂载选项构建测试 ==========

func TestManager_BuildMountOptions(t *testing.T) {
	m := &Manager{
		config: DefaultConfig(),
	}

	tests := []struct {
		name     string
		fsType   string
		opts     map[string]string
		expected []string
	}{
		{
			name:     "vfat 默认选项",
			fsType:   "vfat",
			opts:     nil,
			expected: []string{"utf8,uid=1000,gid=1000,umask=000"},
		},
		{
			name:   "自定义选项",
			fsType: "vfat",
			opts:   map[string]string{"noexec": "", "sync": ""},
			expected: []string{
				"utf8,uid=1000,gid=1000,umask=000",
				"noexec",
				"sync",
			},
		},
		{
			name:     "无默认选项的文件系统",
			fsType:   "ext4",
			opts:     nil,
			expected: []string{},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := m.buildMountOptions(tt.fsType, tt.opts)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== JSON 解析测试 ==========

func TestExtractJSONString(t *testing.T) {
	tests := []struct {
		line     string
		expected string
	}{
		{`"path": "/dev/sdb1"`, "/dev/sdb1"},
		{`"label": "MyDrive"`, "MyDrive"},
		{`"mountpoint": null`, ""},
		{`"fstype": "vfat"`, "vfat"},
		{`invalid`, ""},
	}

	for _, tt := range tests {
		t.Run(tt.line, func(t *testing.T) {
			result := extractJSONString(tt.line)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// ========== 边界条件测试 ==========

func TestManager_EmptyRules(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	rules := m.ListRules()
	assert.Empty(t, rules)

	rule := m.findMatchingRule(&Device{
		UUID: "test-123",
	})
	assert.Nil(t, rule)
}

func TestManager_DisabledRule(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	// 添加禁用的规则
	rule := &MountRule{
		Name:      "Disabled Rule",
		MatchUUID: "test-*",
		Enabled:   false,
	}
	m.AddRule(rule)

	// 查找匹配规则
	found := m.findMatchingRule(&Device{
		UUID: "test-123",
	})
	assert.Nil(t, found)
}

func TestManager_RulePriority(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	// 添加多个规则
	lowPriority := &MountRule{
		Name:      "Low Priority",
		Priority:  1,
		MatchUUID: "test-*",
		Enabled:   true,
	}
	m.AddRule(lowPriority)

	highPriority := &MountRule{
		Name:      "High Priority",
		Priority:  10,
		MatchUUID: "test-*",
		Enabled:   true,
	}
	m.AddRule(highPriority)

	// 查找匹配规则（应返回高优先级的）
	found := m.findMatchingRule(&Device{
		UUID: "test-123",
	})
	require.NotNil(t, found)
	assert.Equal(t, "High Priority", found.Name)
}

// ========== 并发安全测试 ==========

func TestManager_ConcurrentAccess(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)
	// 不启动后台监控，只测试并发访问

	// 并发添加设备
	var wg sync.WaitGroup
	wg.Add(10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			defer wg.Done()
			m.mu.Lock()
			m.devices[fmt.Sprintf("device-%d", idx)] = &Device{
				ID:         fmt.Sprintf("device-%d", idx),
				DevicePath: fmt.Sprintf("/dev/sd%c", 'a'+idx),
			}
			m.mu.Unlock()
		}(i)
	}

	wg.Wait()

	// 验证设备数量
	m.mu.RLock()
	count := len(m.devices)
	m.mu.RUnlock()

	assert.Equal(t, 10, count)
}

// ========== 上下文取消测试 ==========

func TestManager_ContextCancellation(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)

	ctx, cancel := context.WithCancel(context.Background())
	m.ctx = ctx

	// 启动监控
	go m.monitorDevices()

	// 取消上下文
	cancel()

	// 等待 goroutine 结束
	time.Sleep(100 * time.Millisecond)

	// 不应该 panic
}

// ========== 忽略设备测试 ==========

func TestManager_IgnoreDevice(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "usb-config.json")

	m := NewManager(configPath)
	m.config.IgnoreDevices = []string{"ignore-uuid", "IgnoreLabel"}

	// 测试忽略的设备
	assert.True(t, m.isDeviceIgnored(&Device{UUID: "ignore-uuid"}))
	assert.True(t, m.isDeviceIgnored(&Device{Label: "IgnoreLabel"}))
	assert.False(t, m.isDeviceIgnored(&Device{UUID: "other-uuid"}))
}

// 辅助方法：检查设备是否被忽略
func (m *Manager) isDeviceIgnored(device *Device) bool {
	for _, ignore := range m.config.IgnoreDevices {
		if device.UUID == ignore || device.Label == ignore {
			return true
		}
	}
	return false
}