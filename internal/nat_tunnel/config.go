// Package nat_tunnel provides NAT traversal configuration
package nat_tunnel

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// ConfigManager manages tunnel configuration persistence
type ConfigManager struct {
	configPath string
	config     *GlobalConfig
}

// GlobalConfig represents global NAT tunnel configuration
type GlobalConfig struct {
	DefaultType TunnelType              `json:"default_type"`
	ServerAddr  string                  `json:"server_addr"`
	ServerPort  int                     `json:"server_port"`
	Token       string                  `json:"token"`
	EnableHTTPS bool                    `json:"enable_https"`
	FreeQuota   int                     `json:"free_quota"` // Free bandwidth quota (MB/month)
	Tunnels     map[string]TunnelConfig `json:"tunnels"`
}

// NewConfigManager creates a new config manager
func NewConfigManager(configDir string) (*ConfigManager, error) {
	configPath := filepath.Join(configDir, "nat_tunnel.json")

	cm := &ConfigManager{
		configPath: configPath,
		config: &GlobalConfig{
			DefaultType: TunnelTypeFRP,
			FreeQuota:   1024, // 1GB free quota per month (对标fnOS FN Connect)
			Tunnels:     make(map[string]TunnelConfig),
		},
	}

	// Load existing config if exists
	if _, err := os.Stat(configPath); err == nil {
		if err := cm.Load(); err != nil {
			return nil, err
		}
	}

	return cm, nil
}

// Load loads configuration from file
func (cm *ConfigManager) Load() error {
	data, err := os.ReadFile(cm.configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, cm.config)
}

// Save saves configuration to file
func (cm *ConfigManager) Save() error {
	data, err := json.MarshalIndent(cm.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(cm.configPath, data, 0600)
}

// GetConfig gets global configuration
func (cm *ConfigManager) GetConfig() *GlobalConfig {
	return cm.config
}

// SetServerConfig sets server configuration
func (cm *ConfigManager) SetServerConfig(addr string, port int, token string) error {
	cm.config.ServerAddr = addr
	cm.config.ServerPort = port
	cm.config.Token = token
	return cm.Save()
}

// AddTunnel adds a tunnel configuration
func (cm *ConfigManager) AddTunnel(config TunnelConfig) error {
	cm.config.Tunnels[config.ID] = config
	return cm.Save()
}

// RemoveTunnel removes a tunnel configuration
func (cm *ConfigManager) RemoveTunnel(id string) error {
	delete(cm.config.Tunnels, id)
	return cm.Save()
}

// GetTunnel gets a tunnel configuration
func (cm *ConfigManager) GetTunnel(id string) *TunnelConfig {
	if tunnel, ok := cm.config.Tunnels[id]; ok {
		return &tunnel
	}
	return nil
}

// ListTunnels lists all tunnel configurations
func (cm *ConfigManager) ListTunnels() []TunnelConfig {
	tunnels := make([]TunnelConfig, 0, len(cm.config.Tunnels))
	for _, tunnel := range cm.config.Tunnels {
		tunnels = append(tunnels, tunnel)
	}
	return tunnels
}

// DefaultFRPConfig returns default FRP configuration
// 对标飞牛fnOS FN Connect免费服务
func DefaultFRPConfig() GlobalConfig {
	return GlobalConfig{
		DefaultType: TunnelTypeFRP,
		ServerAddr:  "connect.nas-os.io", // Placeholder for free service
		ServerPort:  7000,
		FreeQuota:   1024, // 1GB免费额度
		EnableHTTPS: true,
		Tunnels:     make(map[string]TunnelConfig),
	}
}
