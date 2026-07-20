// Package config 提供 NAS-OS 强类型运行时配置加载。
//
// 目标：
//   - 通过 --config 指定 YAML 文件，环境变量可覆盖敏感字段。
//   - 所有硬编码的路径、地址、目录，最终迁移到本包表达。
//   - 加载失败/校验失败在启动前明确报错，而不是运行时才发现。
package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config 是 NAS-OS 运行时根配置。
type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Paths    PathsConfig    `yaml:"paths"`
	Storage  StorageConfig  `yaml:"storage"`
	Auth     AuthConfig     `yaml:"auth"`
	Modules  ModulesConfig  `yaml:"modules"`
	// Packages is the ADR-0001 Stage-1 surface (dual-read with modules.*).
	// Prefer ResolvePackages() / OptionalProductsEnabled() in production code.
	Packages PackagesConfig `yaml:"packages"`
}

// ServerConfig 描述 HTTP/Web 服务参数。
type ServerConfig struct {
	Host string `yaml:"host"`
	Port int    `yaml:"port"`
}

// Addr 返回监听地址 host:port。
func (s ServerConfig) Addr() string {
	if s.Host == "" {
		return fmt.Sprintf(":%d", s.Port)
	}
	return fmt.Sprintf("%s:%d", s.Host, s.Port)
}

// PathsConfig 集中所有关键路径。
// 保留旧的默认值以最小化迁移风险，后续按模块把散落的字面量收敛到这里。
type PathsConfig struct {
	// MountBase 挂载根目录（用户主目录、存储卷、共享的默认宿主）。
	MountBase string `yaml:"mount_base"`
	// ConfigDir 存放配置和状态描述文件。
	ConfigDir string `yaml:"config_dir"`
	// DataDir 存放运行时数据（数据库、索引、缓存等）。
	DataDir string `yaml:"data_dir"`
	// SambaConfig Samba 主配置文件路径。
	SambaConfig string `yaml:"samba_config"`
	// NFSExports NFS exports 文件路径。
	NFSExports string `yaml:"nfs_exports"`
}

// StorageConfig 描述存储子系统参数。
type StorageConfig struct {
	DefaultProfile string `yaml:"default_profile"`
	AutoScrub      bool   `yaml:"auto_scrub"`
	ScrubSchedule  string `yaml:"scrub_schedule"`
}

// AuthConfig 描述认证/会话参数（当前仅占位，供后续扩展）。
type AuthConfig struct {
	SessionTTLHours int    `yaml:"session_ttl_hours"`
	InitialPassword string `yaml:"initial_password"`
}

// ModulesConfig holds legacy module switches and docker path settings.
//
// Deprecated (ADR-0001 Stage 3): Optional and Extensions are compatibility aliases
// for packages.recommended_system and packages.enabled. Prefer packages.*;
// modules.optional / modules.extensions still work via dual-read but emit warnings.
type ModulesConfig struct {
	Docker DockerConfig `yaml:"docker"`
	// Optional is deprecated; use packages.recommended_system.
	// Still dual-read by ResolvePackages for compatibility.
	Optional bool `yaml:"optional"`
	// Extensions is deprecated; use packages.enabled.
	// Still dual-read by ResolvePackages for compatibility.
	// Lab packages are never loaded via this list.
	Extensions []string `yaml:"extensions"`
}

// DockerConfig 描述 Docker 模块设置。
type DockerConfig struct {
	Enabled  bool   `yaml:"enabled"`
	DataRoot string `yaml:"data_root"`
}

// Default 返回一份代表现有生产默认值的配置。
func Default() *Config {
	return &Config{
		Server: ServerConfig{
			Host: "127.0.0.1",
			Port: 8080,
		},
		Paths: PathsConfig{
			MountBase:   "/mnt",
			ConfigDir:   "/etc/nas-os",
			DataDir:     "/var/lib/nas-os",
			SambaConfig: "/etc/samba/smb.conf",
			NFSExports:  "/etc/exports",
		},
		Storage: StorageConfig{
			DefaultProfile: "single",
			AutoScrub:      true,
			ScrubSchedule:  "0 2 * * 0",
		},
		Auth: AuthConfig{
			SessionTTLHours: 24,
		},
		Modules: ModulesConfig{
			Docker: DockerConfig{
				Enabled:  false,
				DataRoot: "/mnt/docker",
			},
			Optional:   false, // default: no non-Core product managers
			Extensions: nil,
		},
		Packages: PackagesConfig{
			RecommendedSystem: false, // default Core-only (ADR-0001 Stage 1)
			Enabled:           nil,
			CommunityDir:      "",    // default: no third-party discovery
			LegacySOPlugins:   false, // deprecated .so host off by default
		},
	}
}

// BootProductIDs returns recommended_product IDs that should construct product
// managers at process start: full set when recommended_system, otherwise
// individually named IDs from packages.enabled plus app-center-enabled.json.
func (c *Config) BootProductIDs() []string {
	if c == nil {
		return nil
	}
	if c.OptionalProductsEnabled() {
		return RecommendedSystemPackageIDs()
	}
	seen := make(map[string]struct{})
	var out []string
	add := func(id string) {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" {
			return
		}
		if e, ok := LookupSystemPackage(id); !ok || e.Kind != KindRecommendedProduct {
			return
		}
		if _, dup := seen[id]; dup {
			return
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	for _, id := range c.EnabledPackageNames() {
		add(id)
	}
	for _, id := range LoadAppCenterEnabledIDs(c.DataPath("app-center-enabled.json")) {
		add(id)
	}
	return out
}

// WantsProduct reports whether a recommended product should be active at boot.
func (c *Config) WantsProduct(id string) bool {
	id = strings.ToLower(strings.TrimSpace(id))
	if c == nil || id == "" {
		return false
	}
	if c.OptionalProductsEnabled() {
		return true // bulk recommended surface
	}
	for _, p := range c.BootProductIDs() {
		if p == id {
			return true
		}
	}
	return false
}

// LoadAppCenterEnabledIDs reads data_dir/app-center-enabled.json (UI click persistence).
func LoadAppCenterEnabledIDs(path string) []string {
	if path == "" {
		return nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	var doc struct {
		Enabled []string `json:"enabled"`
	}
	if err := json.Unmarshal(data, &doc); err != nil {
		return nil
	}
	var out []string
	seen := make(map[string]struct{})
	for _, id := range doc.Enabled {
		id = strings.ToLower(strings.TrimSpace(id))
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

// LegacySOPluginHostEnabled reports whether the deprecated .so plugin host may start.
func (c *Config) LegacySOPluginHostEnabled() bool {
	if c == nil {
		return false
	}
	return c.Packages.LegacySOPlugins && (c.OptionalProductsEnabled() || len(c.BootProductIDs()) > 0)
}

// Load 从磁盘加载 YAML 配置。
// path 为空时返回 Default() 副本；文件不存在也返回 Default()（保持向后兼容）。
// 加载后立即应用环境变量覆盖并校验。
func Load(path string) (*Config, error) {
	cfg := Default()

	if path != "" {
		data, err := os.ReadFile(path)
		switch {
		case err == nil:
			if len(data) > 0 {
				if err := yaml.Unmarshal(data, cfg); err != nil {
					return nil, fmt.Errorf("解析配置 %s 失败：%w", path, err)
				}
			}
		case errors.Is(err, os.ErrNotExist):
			// 保持向后兼容：允许无配置启动，使用默认值。
		default:
			return nil, fmt.Errorf("读取配置 %s 失败：%w", path, err)
		}
	}

	if err := cfg.applyEnv(); err != nil {
		return nil, err
	}

	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	return cfg, nil
}

// applyEnv 使用 NAS_OS_* 环境变量覆盖可选字段。
// 仅覆盖运维/部署时最常调整的参数；密钥类字段禁止落 YAML。
func (c *Config) applyEnv() error {
	if v := os.Getenv("NAS_OS_LISTEN_HOST"); v != "" {
		c.Server.Host = v
	}
	if v := os.Getenv("NAS_OS_LISTEN_PORT"); v != "" {
		port, err := strconv.Atoi(v)
		if err != nil || port <= 0 || port > 65535 {
			return fmt.Errorf("NAS_OS_LISTEN_PORT 非法：%q", v)
		}
		c.Server.Port = port
	}
	if v := os.Getenv("NAS_OS_MOUNT_BASE"); v != "" {
		c.Paths.MountBase = v
	}
	if v := os.Getenv("NAS_OS_CONFIG_DIR"); v != "" {
		c.Paths.ConfigDir = v
	}
	if v := os.Getenv("NAS_OS_DATA_DIR"); v != "" {
		c.Paths.DataDir = v
	}
	if v := os.Getenv("NAS_OS_SAMBA_CONFIG"); v != "" {
		c.Paths.SambaConfig = v
	}
	if v := os.Getenv("NAS_OS_NFS_EXPORTS"); v != "" {
		c.Paths.NFSExports = v
	}
	return nil
}

// Validate 校验根配置。
func (c *Config) Validate() error {
	if c.Server.Port <= 0 || c.Server.Port > 65535 {
		return fmt.Errorf("server.port 非法：%d", c.Server.Port)
	}
	if strings.TrimSpace(c.Paths.MountBase) == "" {
		return errors.New("paths.mount_base 不能为空")
	}
	if strings.TrimSpace(c.Paths.ConfigDir) == "" {
		return errors.New("paths.config_dir 不能为空")
	}
	if strings.TrimSpace(c.Paths.DataDir) == "" {
		return errors.New("paths.data_dir 不能为空")
	}
	// 路径必须是绝对路径，避免 CWD 依赖。
	for name, p := range map[string]string{
		"mount_base":   c.Paths.MountBase,
		"config_dir":   c.Paths.ConfigDir,
		"data_dir":     c.Paths.DataDir,
		"samba_config": c.Paths.SambaConfig,
		"nfs_exports":  c.Paths.NFSExports,
	} {
		if p != "" && !filepath.IsAbs(p) {
			return fmt.Errorf("paths.%s 必须是绝对路径：%s", name, p)
		}
	}
	return nil
}

// ConfigPath 返回 ConfigDir 下的子路径。
func (c *Config) ConfigPath(elem ...string) string {
	return filepath.Join(append([]string{c.Paths.ConfigDir}, elem...)...)
}

// DataPath 返回 DataDir 下的子路径。
func (c *Config) DataPath(elem ...string) string {
	return filepath.Join(append([]string{c.Paths.DataDir}, elem...)...)
}

// MountPath 返回 MountBase 下的子路径。
func (c *Config) MountPath(elem ...string) string {
	return filepath.Join(append([]string{c.Paths.MountBase}, elem...)...)
}
