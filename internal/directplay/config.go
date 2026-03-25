package directplay

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
)

// ConfigManager 配置管理器
type ConfigManager struct {
	configPath string
	config     *DirectPlayConfig
	mu         sync.RWMutex
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configPath string) (*ConfigManager, error) {
	if configPath == "" {
		// 默认配置路径
		homeDir, _ := os.UserHomeDir()
		configPath = filepath.Join(homeDir, ".nas-os", "directplay.json")
	}

	cm := &ConfigManager{
		configPath: configPath,
		config:     DefaultDirectPlayConfig(),
	}

	// 加载配置
	if err := cm.Load(); err != nil {
		// 配置文件不存在，使用默认配置
		_ = cm.Save()
	}

	return cm, nil
}

// Load 加载配置
func (cm *ConfigManager) Load() error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return err
	}

	var config DirectPlayConfig
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	cm.config = &config
	return nil
}

// Save 保存配置
func (cm *ConfigManager) Save() error {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 确保目录存在
	if err := os.MkdirAll(filepath.Dir(cm.configPath), 0750); err != nil {
		return err
	}

	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cm.configPath, data, 0600)
}

// Get 获取配置
func (cm *ConfigManager) Get() *DirectPlayConfig {
	cm.mu.RLock()
	defer cm.mu.RUnlock()

	// 返回副本
	config := *cm.config
	return &config
}

// Update 更新配置
func (cm *ConfigManager) Update(config *DirectPlayConfig) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.config = config
	return cm.Save()
}

// SetEnabled 设置总开关
func (cm *ConfigManager) SetEnabled(enabled bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.config.Enabled = enabled
	return cm.Save()
}

// SetProviderEnabled 设置网盘开关
func (cm *ConfigManager) SetProviderEnabled(provider ProviderType, enabled bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	switch provider {
	case ProviderBaiduPan:
		cm.config.BaiduPanEnabled = enabled
	case Provider123Pan:
		cm.config.Pan123Enabled = enabled
	case ProviderAliyunPan:
		cm.config.AliyunPanEnabled = enabled
	}

	return cm.Save()
}

// SetCacheEnabled 设置缓存开关
func (cm *ConfigManager) SetCacheEnabled(enabled bool) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.config.CacheEnabled = enabled
	return cm.Save()
}

// SetCacheTTL 设置缓存有效期
func (cm *ConfigManager) SetCacheTTL(ttl int) error {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	cm.config.CacheTTL = cm.config.CacheTTL.Abs()
	return cm.Save()
}