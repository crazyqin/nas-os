// Package app 应用配置管理
// 提供应用配置的持久化、验证、版本管理等功能
package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// ========== 配置文件类型 ==========

// AppConfigFile 应用配置文件结构
type AppConfigFile struct {
	Version     string                 `json:"version"`     // 配置文件版本
	AppID       string                 `json:"appId"`       // 应用ID
	TemplateID  string                 `json:"templateId"`  // 模板ID
	CreatedAt   time.Time              `json:"createdAt"`   // 创建时间
	UpdatedAt   time.Time              `json:"updatedAt"`   // 更新时间
	Settings    map[string]interface{} `json:"settings"`    // 用户设置
	Secrets     map[string]string      `json:"secrets"`     // 密钥（不持久化，运行时注入）
	Metadata    ConfigMetadata         `json:"metadata"`    // 元数据
}

// ConfigMetadata 配置元数据
type ConfigMetadata struct {
	InstallOptions InstallOptions `json:"installOptions"` // 安装选项
	UpgradeHistory []UpgradeRecord `json:"upgradeHistory"` // 升级历史
	BackupPaths    []string       `json:"backupPaths"`    // 备份路径
	CustomFields   map[string]interface{} `json:"customFields"` // 自定义字段
}

// UpgradeRecord 升级记录
type UpgradeRecord struct {
	FromVersion string    `json:"fromVersion"` // 原版本
	ToVersion   string    `json:"toVersion"`   // 新版本
	At          time.Time `json:"at"`          // 升级时间
	By          string    `json:"by"`          // 操作者
	Result      string    `json:"result"`      // 结果(success/failed)
}

// ========== 配置管理器 ==========

// ConfigManager 配置管理器
type ConfigManager struct {
	configDir string // 配置文件目录
}

// NewConfigManager 创建配置管理器
func NewConfigManager(configDir string) (*ConfigManager, error) {
	if err := os.MkdirAll(configDir, 0750); err != nil {
		return nil, fmt.Errorf("创建配置目录失败: %w", err)
	}

	return &ConfigManager{
		configDir: configDir,
	}, nil
}

// Load 加载应用配置
func (m *ConfigManager) Load(appID string) (*AppConfigFile, error) {
	configPath := filepath.Join(m.configDir, appID+".json")

	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil // 配置不存在，返回空
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}

	config := &AppConfigFile{}
	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("解析配置文件失败: %w", err)
	}

	return config, nil
}

// Save 保存应用配置
func (m *ConfigManager) Save(config *AppConfigFile) error {
	config.UpdatedAt = time.Now()

	configPath := filepath.Join(m.configDir, config.AppID+".json")

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	return os.WriteFile(configPath, data, 0640) // 配置文件权限更严格
}

// Delete 删除应用配置
func (m *ConfigManager) Delete(appID string) error {
	configPath := filepath.Join(m.configDir, appID+".json")

	return os.Remove(configPath)
}

// Exists 检查配置是否存在
func (m *ConfigManager) Exists(appID string) bool {
	configPath := filepath.Join(m.configDir, appID+".json")
	_, err := os.Stat(configPath)
	return err == nil
}

// Backup 备份配置
func (m *ConfigManager) Backup(appID string) (string, error) {
	config, err := m.Load(appID)
	if err != nil {
		return "", err
	}

	if config == nil {
		return "", fmt.Errorf("配置不存在")
	}

	// 创建备份文件名
	backupName := fmt.Sprintf("%s.%s.bak", appID, time.Now().Format("20060102150405"))
	backupPath := filepath.Join(m.configDir, "backups", backupName)

	// 创建备份目录
	if err := os.MkdirAll(filepath.Dir(backupPath), 0750); err != nil {
		return "", fmt.Errorf("创建备份目录失败: %w", err)
	}

	// 保存备份
	configCopy := *config // 复制一份
	data, err := json.MarshalIndent(configCopy, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(backupPath, data, 0640); err != nil {
		return "", fmt.Errorf("保存备份失败: %w", err)
	}

	return backupPath, nil
}

// Restore 从备份恢复配置
func (m *ConfigManager) Restore(appID, backupPath string) error {
	data, err := os.ReadFile(backupPath)
	if err != nil {
		return fmt.Errorf("读取备份文件失败: %w", err)
	}

	config := &AppConfigFile{}
	if err := json.Unmarshal(data, config); err != nil {
		return fmt.Errorf("解析备份文件失败: %w", err)
	}

	return m.Save(config)
}

// ListBackups 列出备份文件
func (m *ConfigManager) ListBackups(appID string) ([]string, error) {
	backupDir := filepath.Join(m.configDir, "backups")

	files, err := os.ReadDir(backupDir)
	if err != nil {
		if os.IsNotExist(err) {
			return []string{}, nil
		}
		return nil, fmt.Errorf("读取备份目录失败: %w", err)
	}

	backups := []string{}
	for _, file := range files {
		if strings.HasPrefix(file.Name(), appID+".") && strings.HasSuffix(file.Name(), ".bak") {
			backups = append(backups, filepath.Join(backupDir, file.Name()))
		}
	}

	return backups, nil
}

// ========== 配置验证 ==========

// ConfigValidator 配置验证器
type ConfigValidator struct{}

// NewConfigValidator 创建配置验证器
func NewConfigValidator() *ConfigValidator {
	return &ConfigValidator{}
}

// ValidateSettings 验证设置值
func (v *ConfigValidator) ValidateSettings(settings map[string]interface{}, schema ConfigSchema) []error {
	errors := []error{}

	for key, spec := range schema.Fields {
		value, exists := settings[key]

		// 检查必填字段
		if !exists && spec.Required {
			errors = append(errors, fmt.Errorf("字段 %s 不能为空", key))
			continue
		}

		// 如果值存在，验证类型和格式
		if exists {
			if err := v.validateField(key, value, spec); err != nil {
				errors = append(errors, err)
			}
		}
	}

	return errors
}

// validateField 验证单个字段
func (v *ConfigValidator) validateField(key string, value interface{}, spec ConfigFieldSpec) error {
	// 类型验证
	if spec.Type != "" {
		switch spec.Type {
		case "string":
			if _, ok := value.(string); !ok {
				return fmt.Errorf("字段 %s 类型应为 string", key)
			}
		case "int":
			switch value.(type) {
			case int, int64, float64:
				// JSON数字默认为float64，允许转换
			default:
				return fmt.Errorf("字段 %s 类型应为 int", key)
			}
		case "bool":
			if _, ok := value.(bool); !ok {
				return fmt.Errorf("字段 %s 类型应为 bool", key)
			}
		case "array":
			if _, ok := value.([]interface{}); !ok {
				return fmt.Errorf("字段 %s 类型应为 array", key)
			}
		case "object":
			if _, ok := value.(map[string]interface{}); !ok {
				return fmt.Errorf("字段 %s 类型应为 object", key)
			}
		}
	}

	// 字符串验证
	if str, ok := value.(string); ok {
		// 最小长度
		if spec.MinLength > 0 && len(str) < spec.MinLength {
			return fmt.Errorf("字段 %s 长度不能小于 %d", key, spec.MinLength)
		}

		// 最大长度
		if spec.MaxLength > 0 && len(str) > spec.MaxLength {
			return fmt.Errorf("字段 %s 长度不能大于 %d", key, spec.MaxLength)
		}

		// 正则匹配
		if spec.Pattern != "" && !strings.Contains(str, spec.Pattern) {
			return fmt.Errorf("字段 %s 格式不正确", key)
		}
	}

	// 数值验证
	if num, ok := value.(float64); ok {
		// 最小值
		if spec.Min > 0 && num < spec.Min {
			return fmt.Errorf("字段 %s 不能小于 %v", key, spec.Min)
		}

		// 最大值
		if spec.Max > 0 && num > spec.Max {
			return fmt.Errorf("字段 %s 不能大于 %v", key, spec.Max)
		}
	}

	// 枚举值验证
	if len(spec.Enum) > 0 {
		found := false
		for _, enumVal := range spec.Enum {
			if fmt.Sprintf("%v", value) == fmt.Sprintf("%v", enumVal) {
				found = true
				break
			}
		}
		if !found {
			return fmt.Errorf("字段 %s 值不在允许范围内", key)
		}
	}

	return nil
}

// ========== 配置模板 ==========

// ConfigSchema 配置模板定义
type ConfigSchema struct {
	Version string                `json:"version"` // 配置版本
	Fields  map[string]ConfigFieldSpec `json:"fields"` // 字段定义
	Groups  []ConfigGroup         `json:"groups"`  // 字段分组（用于UI显示）
}

// ConfigFieldSpec 配置字段规格
type ConfigFieldSpec struct {
	Type        string        `json:"type"`        // 类型: string/int/bool/array/object
	Required    bool          `json:"required"`    // 必填
	Default     interface{}   `json:"default"`     // 默认值
	Label       string        `json:"label"`       // 显示标签
	Description string        `json:"description"` // 字段说明
	MinLength   int           `json:"minLength"`   // 最小长度（字符串）
	MaxLength   int           `json:"maxLength"`   // 最大长度（字符串）
	Min         float64       `json:"min"`         // 最小值（数值）
	Max         float64       `json:"max"`         // 最大值（数值）
	Pattern     string        `json:"pattern"`     // 格式模式
	Enum        []interface{} `json:"enum"`        // 枚举值列表
	Secret      bool          `json:"secret"`      // 是否为密钥字段
	ReadOnly    bool          `json:"readOnly"`    // 只读（安装后不可更改）
}

// ConfigGroup 配置字段分组
type ConfigGroup struct {
	Name   string   `json:"name"`   // 分组名称
	Label  string   `json:"label"`  // 分组标签
	Fields []string `json:"fields"` // 分组包含的字段
}

// ========== 配置迁移 ==========

// ConfigMigrator 配置迁移器
type ConfigMigrator struct {
	managers map[string]MigrationHandler // 版本迁移处理器
}

// MigrationHandler 迁移处理器
type MigrationHandler func(old *AppConfigFile) (*AppConfigFile, error)

// NewConfigMigrator 创建配置迁移器
func NewConfigMigrator() *ConfigMigrator {
	return &ConfigMigrator{
		managers: make(map[string]MigrationHandler),
	}
}

// RegisterMigration 注册迁移处理器
func (m *ConfigMigrator) RegisterMigration(fromVersion, toVersion string, handler MigrationHandler) {
	key := fmt.Sprintf("%s->%s", fromVersion, toVersion)
	m.managers[key] = handler
}

// Migrate 执行配置迁移
func (m *ConfigMigrator) Migrate(config *AppConfigFile, targetVersion string) (*AppConfigFile, error) {
	currentVersion := config.Version

	// 如果版本相同，无需迁移
	if currentVersion == targetVersion {
		return config, nil
	}

	// 查找迁移处理器
	key := fmt.Sprintf("%s->%s", currentVersion, targetVersion)
	handler, exists := m.managers[key]
	if !exists {
		return nil, fmt.Errorf("不支持从版本 %s 到 %s 的迁移", currentVersion, targetVersion)
	}

	// 执行迁移
	return handler(config)
}

// ========== 默认配置模板 ==========

// DefaultConfigSchema 默认配置模板
var DefaultConfigSchema = ConfigSchema{
	Version: "1.0",
	Fields: map[string]ConfigFieldSpec{
		// 端口配置
		"webPort": {
			Type:        "int",
			Label:       "Web端口",
			Description: "Web界面访问端口",
			Default:     8080,
			Min:         1,
			Max:         65535,
			Required:    false,
		},

		// 数据目录
		"dataDir": {
			Type:        "string",
			Label:       "数据目录",
			Description: "应用数据存储目录",
			Default:     "/opt/nas/apps/{{.AppID}}/data",
			Required:    true,
			MinLength:   1,
		},

		// 时区
		"timezone": {
			Type:        "string",
			Label:       "时区",
			Description: "应用时区设置",
			Default:     "Asia/Shanghai",
			Enum:        []interface{}{"Asia/Shanghai", "UTC", "America/New_York", "Europe/London"},
		},

		// PUID/PGID
		"puid": {
			Type:        "int",
			Label:       "用户ID",
			Description: "运行用户ID",
			Default:     1000,
			Min:         0,
		},
		"pgid": {
			Type:        "int",
			Label:       "组ID",
			Description: "运行组ID",
			Default:     1000,
			Min:         0,
		},

		// 资源限制
		"cpuLimit": {
			Type:        "string",
			Label:       "CPU限制",
			Description: "CPU使用限制",
			Default:     "",
			Pattern:     "\\d+(\\.\\d+)?",
		},
		"memoryLimit": {
			Type:        "string",
			Label:       "内存限制",
			Description: "内存使用限制",
			Default:     "",
			Pattern:     "\\d+[MG]",
		},
	},
	Groups: []ConfigGroup{
		{
			Name:   "basic",
			Label:  "基本设置",
			Fields: []string{"webPort", "dataDir", "timezone"},
		},
		{
			Name:   "permission",
			Label:  "权限设置",
			Fields: []string{"puid", "pgid"},
		},
		{
			Name:   "resource",
			Label:  "资源限制",
			Fields: []string{"cpuLimit", "memoryLimit"},
		},
	},
}

// GetConfigSchema 获取默认配置模板
func GetConfigSchema() ConfigSchema {
	return DefaultConfigSchema
}

// GetConfigSchemaForTemplate 根据应用模板生成配置模板
func GetConfigSchemaForTemplate(tmpl *Template) ConfigSchema {
	schema := ConfigSchema{
		Version: "1.0",
		Fields:  make(map[string]ConfigFieldSpec),
		Groups:  []ConfigGroup{},
	}

	// 添加端口配置
	portFields := []string{}
	for _, container := range tmpl.Containers {
		for _, port := range container.Ports {
			fieldName := fmt.Sprintf("port_%s", port.Name)
			schema.Fields[fieldName] = ConfigFieldSpec{
				Type:        "int",
				Label:       port.Description,
				Description: fmt.Sprintf("%s端口映射", port.Description),
				Default:     port.DefaultHostPort,
				Min:         1,
				Max:         65535,
				Required:    port.Required,
			}
			portFields = append(portFields, fieldName)
		}
	}

	if len(portFields) > 0 {
		schema.Groups = append(schema.Groups, ConfigGroup{
			Name:   "ports",
			Label:  "端口配置",
			Fields: portFields,
		})
	}

	// 添加卷配置
	volumeFields := []string{}
	for _, container := range tmpl.Containers {
		for _, vol := range container.Volumes {
			fieldName := fmt.Sprintf("volume_%s", vol.Name)
			schema.Fields[fieldName] = ConfigFieldSpec{
				Type:        "string",
				Label:       vol.Description,
				Description: fmt.Sprintf("%s目录路径", vol.Description),
				Default:     vol.DefaultHostPath,
				Required:    vol.Required,
				MinLength:   1,
			}
			volumeFields = append(volumeFields, fieldName)
		}
	}

	if len(volumeFields) > 0 {
		schema.Groups = append(schema.Groups, ConfigGroup{
			Name:   "volumes",
			Label:  "目录配置",
			Fields: volumeFields,
		})
	}

	// 添加环境变量配置
	envFields := []string{}
	for _, container := range tmpl.Containers {
		for key, value := range container.Environment {
			fieldName := fmt.Sprintf("env_%s", key)
			schema.Fields[fieldName] = ConfigFieldSpec{
				Type:        "string",
				Label:       key,
				Description: fmt.Sprintf("环境变量 %s", key),
				Default:     value,
				Required:    false,
			}
			envFields = append(envFields, fieldName)
		}
	}

	if len(envFields) > 0 {
		schema.Groups = append(schema.Groups, ConfigGroup{
			Name:   "environment",
			Label:  "环境变量",
			Fields: envFields,
		})
	}

	return schema
}