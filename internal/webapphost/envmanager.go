package webapphost

import (
	"fmt"
	"log"
	"sync"
)

// EnvManager 环境变量管理器.
type EnvManager struct {
	mu        sync.RWMutex
	appEnv    map[string]map[string]string // appID -> env vars
	secrets   map[string]map[string]string // appID -> secrets
	templates map[string][]EnvVarDef       // templateID -> env var definitions
}

// NewEnvManager 创建环境变量管理器.
func NewEnvManager() *EnvManager {
	return &EnvManager{
		appEnv:    make(map[string]map[string]string),
		secrets:   make(map[string]map[string]string),
		templates: make(map[string][]EnvVarDef),
	}
}

// SetEnv 设置应用环境变量.
func (em *EnvManager) SetEnv(appID, key, value string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if appID == "" {
		return fmt.Errorf("app ID is required")
	}
	if key == "" {
		return fmt.Errorf("key is required")
	}

	if _, exists := em.appEnv[appID]; !exists {
		em.appEnv[appID] = make(map[string]string)
	}

	em.appEnv[appID][key] = value
	log.Printf("Environment variable set: %s.%s", appID, key)
	return nil
}

// GetEnv 获取应用环境变量.
func (em *EnvManager) GetEnv(appID, key string) (string, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	env, exists := em.appEnv[appID]
	if !exists {
		return "", fmt.Errorf("app not found: %s", appID)
	}

	value, exists := env[key]
	if !exists {
		return "", fmt.Errorf("environment variable not found: %s", key)
	}

	return value, nil
}

// GetAllEnv 获取应用所有环境变量.
func (em *EnvManager) GetAllEnv(appID string) map[string]string {
	em.mu.RLock()
	defer em.mu.RUnlock()

	env, exists := em.appEnv[appID]
	if !exists {
		return make(map[string]string)
	}

	// 复制一份返回
	result := make(map[string]string, len(env))
	for k, v := range env {
		result[k] = v
	}
	return result
}

// DeleteEnv 删除应用环境变量.
func (em *EnvManager) DeleteEnv(appID, key string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	env, exists := em.appEnv[appID]
	if !exists {
		return fmt.Errorf("app not found: %s", appID)
	}

	if _, exists := env[key]; !exists {
		return fmt.Errorf("environment variable not found: %s", key)
	}

	delete(env, key)
	log.Printf("Environment variable deleted: %s.%s", appID, key)
	return nil
}

// BatchSetEnv 批量设置环境变量.
func (em *EnvManager) BatchSetEnv(appID string, vars map[string]string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if appID == "" {
		return fmt.Errorf("app ID is required")
	}

	if _, exists := em.appEnv[appID]; !exists {
		em.appEnv[appID] = make(map[string]string)
	}

	for key, value := range vars {
		em.appEnv[appID][key] = value
	}

	log.Printf("Batch environment variables set for app: %s (%d vars)", appID, len(vars))
	return nil
}

// SetSecret 设置密钥.
func (em *EnvManager) SetSecret(appID, key, value string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if appID == "" {
		return fmt.Errorf("app ID is required")
	}
	if key == "" {
		return fmt.Errorf("key is required")
	}

	if _, exists := em.secrets[appID]; !exists {
		em.secrets[appID] = make(map[string]string)
	}

	// 实际实现：应该加密存储
	em.secrets[appID][key] = value
	log.Printf("Secret set: %s.%s", appID, key)
	return nil
}

// GetSecret 获取密钥.
func (em *EnvManager) GetSecret(appID, key string) (string, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	secrets, exists := em.secrets[appID]
	if !exists {
		return "", fmt.Errorf("app not found: %s", appID)
	}

	value, exists := secrets[key]
	if !exists {
		return "", fmt.Errorf("secret not found: %s", key)
	}

	return value, nil
}

// DeleteSecret 删除密钥.
func (em *EnvManager) DeleteSecret(appID, key string) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	secrets, exists := em.secrets[appID]
	if !exists {
		return fmt.Errorf("app not found: %s", appID)
	}

	if _, exists := secrets[key]; !exists {
		return fmt.Errorf("secret not found: %s", key)
	}

	delete(secrets, key)
	log.Printf("Secret deleted: %s.%s", appID, key)
	return nil
}

// DeleteAllEnv 删除应用所有环境变量.
func (em *EnvManager) DeleteAllEnv(appID string) {
	em.mu.Lock()
	defer em.mu.Unlock()

	delete(em.appEnv, appID)
	delete(em.secrets, appID)
	log.Printf("All environment variables deleted for app: %s", appID)
}

// RegisterTemplate 注册模板环境变量定义.
func (em *EnvManager) RegisterTemplate(templateID string, vars []EnvVarDef) error {
	em.mu.Lock()
	defer em.mu.Unlock()

	if templateID == "" {
		return fmt.Errorf("template ID is required")
	}

	em.templates[templateID] = vars
	log.Printf("Template environment variables registered: %s (%d vars)", templateID, len(vars))
	return nil
}

// GetTemplateVars 获取模板环境变量定义.
func (em *EnvManager) GetTemplateVars(templateID string) ([]EnvVarDef, error) {
	em.mu.RLock()
	defer em.mu.RUnlock()

	vars, exists := em.templates[templateID]
	if !exists {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	return vars, nil
}

// ValidateEnv 验证环境变量是否符合模板定义.
func (em *EnvManager) ValidateEnv(templateID string, vars map[string]string) error {
	em.mu.RLock()
	defer em.mu.RUnlock()

	tmplVars, exists := em.templates[templateID]
	if !exists {
		return nil // 没有模板定义，不验证
	}

	// 检查必需变量
	for _, tmplVar := range tmplVars {
		if tmplVar.Required {
			value, exists := vars[tmplVar.Name]
			if !exists || value == "" {
				return fmt.Errorf("required environment variable missing: %s", tmplVar.Name)
			}
		}
	}

	// 检查选项类型
	for _, tmplVar := range tmplVars {
		if tmplVar.Type == "select" && len(tmplVar.Options) > 0 {
			value, exists := vars[tmplVar.Name]
			if exists && value != "" {
				valid := false
				for _, option := range tmplVar.Options {
					if value == option {
						valid = true
						break
					}
				}
				if !valid {
					return fmt.Errorf("invalid value for %s: %s (valid options: %v)", tmplVar.Name, value, tmplVar.Options)
				}
			}
		}
	}

	return nil
}

// ApplyDefaults 应用默认值.
func (em *EnvManager) ApplyDefaults(templateID string, vars map[string]string) map[string]string {
	em.mu.RLock()
	defer em.mu.RUnlock()

	tmplVars, exists := em.templates[templateID]
	if !exists {
		return vars
	}

	result := make(map[string]string)
	for k, v := range vars {
		result[k] = v
	}

	for _, tmplVar := range tmplVars {
		if _, exists := result[tmplVar.Name]; !exists && tmplVar.Default != "" {
			result[tmplVar.Name] = tmplVar.Default
		}
	}

	return result
}

// MergeEnv 合并环境变量（覆盖）.
func MergeEnv(base, overlay map[string]string) map[string]string {
	result := make(map[string]string)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range overlay {
		result[k] = v
	}
	return result
}

// FilterSecrets 过滤掉密钥（返回脱敏版本）.
func FilterSecrets(vars map[string]string, secretKeys []string) map[string]string {
	secretSet := make(map[string]bool)
	for _, key := range secretKeys {
		secretSet[key] = true
	}

	result := make(map[string]string, len(vars))
	for k, v := range vars {
		if secretSet[k] {
			result[k] = "********"
		} else {
			result[k] = v
		}
	}
	return result
}

// GetEnvCount 获取应用环境变量数量.
func (em *EnvManager) GetEnvCount(appID string) int {
	em.mu.RLock()
	defer em.mu.RUnlock()

	env, exists := em.appEnv[appID]
	if !exists {
		return 0
	}
	return len(env)
}

// GetSecretCount 获取应用密钥数量.
func (em *EnvManager) GetSecretCount(appID string) int {
	em.mu.RLock()
	defer em.mu.RUnlock()

	secrets, exists := em.secrets[appID]
	if !exists {
		return 0
	}
	return len(secrets)
}
