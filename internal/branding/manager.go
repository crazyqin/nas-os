package branding

import (
	"encoding/json"
	"errors"
	"sync"
	"time"
)

var (
	// ErrBrandNotFound 品牌配置未找到.
	ErrBrandNotFound = errors.New("brand config not found")
	// ErrInvalidTheme 无效主题.
	ErrInvalidTheme = errors.New("invalid theme mode")
	// ErrDuplicateBrand 品牌名称重复.
	ErrDuplicateBrand = errors.New("brand name already exists")
)

// Manager 品牌管理器.
type Manager struct {
	mu       sync.RWMutex
	brands   map[string]*BrandConfig
	activeID string
}

// NewManager 创建品牌管理器.
func NewManager() *Manager {
	m := &Manager{
		brands: make(map[string]*BrandConfig),
	}
	// 注册默认品牌
	m.brands["default"] = &BrandConfig{
		ID:   "default",
		Name: "NAS-OS",
		Theme: Theme{
			Mode:         "auto",
			PrimaryColor: "#1890ff",
			AccentColor:  "#52c41a",
		},
		LoginScreen: LoginScreen{
			Title:    "NAS-OS",
			Subtitle: "网络存储操作系统",
			ShowLogo: true,
		},
		Fonts: Fonts{
			Primary:   "Inter",
			Monospace: "JetBrains Mono",
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
	m.activeID = "default"
	return m
}

// Get 获取品牌配置.
func (m *Manager) Get(id string) (*BrandConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.brands[id]
	if !ok {
		return nil, ErrBrandNotFound
	}
	return b, nil
}

// GetActive 获取当前激活的品牌配置.
func (m *Manager) GetActive() *BrandConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.brands[m.activeID]
}

// List 列出所有品牌配置.
func (m *Manager) List() []*BrandConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()
	result := make([]*BrandConfig, 0, len(m.brands))
	for _, b := range m.brands {
		result = append(result, b)
	}
	return result
}

// Create 创建品牌配置.
func (m *Manager) Create(cfg *BrandConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, existing := range m.brands {
		if existing.Name == cfg.Name && existing.ID != cfg.ID {
			return ErrDuplicateBrand
		}
	}
	if cfg.ID == "" {
		cfg.ID = "brand-" + time.Now().Format("20060102150405")
	}
	now := time.Now()
	cfg.CreatedAt = now
	cfg.UpdatedAt = now
	m.brands[cfg.ID] = cfg
	return nil
}

// Update 更新品牌配置.
func (m *Manager) Update(id string, cfg *BrandConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	existing, ok := m.brands[id]
	if !ok {
		return ErrBrandNotFound
	}
	cfg.ID = id
	cfg.CreatedAt = existing.CreatedAt
	cfg.UpdatedAt = time.Now()
	m.brands[id] = cfg
	return nil
}

// Delete 删除品牌配置.
func (m *Manager) Delete(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if id == "default" {
		return errors.New("cannot delete default brand")
	}
	if _, ok := m.brands[id]; !ok {
		return ErrBrandNotFound
	}
	if m.activeID == id {
		m.activeID = "default"
	}
	delete(m.brands, id)
	return nil
}

// SetTheme 切换主题.
func (m *Manager) SetTheme(id, mode string) error {
	if mode != "light" && mode != "dark" && mode != "auto" {
		return ErrInvalidTheme
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.brands[id]
	if !ok {
		return ErrBrandNotFound
	}
	b.Theme.Mode = mode
	b.UpdatedAt = time.Now()
	return nil
}

// SetActive 设置激活品牌.
func (m *Manager) SetActive(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if _, ok := m.brands[id]; !ok {
		return ErrBrandNotFound
	}
	m.activeID = id
	return nil
}

// UpdateLogo 更新Logo.
func (m *Manager) UpdateLogo(id string, logo Logo) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.brands[id]
	if !ok {
		return ErrBrandNotFound
	}
	b.Logo = logo
	b.UpdatedAt = time.Now()
	return nil
}

// UpdateLoginScreen 更新登录页面.
func (m *Manager) UpdateLoginScreen(id string, ls LoginScreen) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.brands[id]
	if !ok {
		return ErrBrandNotFound
	}
	b.LoginScreen = ls
	b.UpdatedAt = time.Now()
	return nil
}

// UpdateCustomCSS 更新自定义CSS.
func (m *Manager) UpdateCustomCSS(id string, css CustomCSS) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.brands[id]
	if !ok {
		return ErrBrandNotFound
	}
	b.CustomCSS = css
	b.UpdatedAt = time.Now()
	return nil
}

// UpdateFonts 更新字体.
func (m *Manager) UpdateFonts(id string, fonts Fonts) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, ok := m.brands[id]
	if !ok {
		return ErrBrandNotFound
	}
	b.Fonts = fonts
	b.UpdatedAt = time.Now()
	return nil
}

// Export 导出品牌配置为JSON.
func (m *Manager) Export(id string) ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.brands[id]
	if !ok {
		return nil, ErrBrandNotFound
	}
	return json.MarshalIndent(b, "", "  ")
}

// Import 从JSON导入品牌配置.
func (m *Manager) Import(data []byte) (*BrandConfig, error) {
	var cfg BrandConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if err := m.Create(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

// ExportAll 导出所有品牌配置.
func (m *Manager) ExportAll() ([]byte, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return json.MarshalIndent(m.brands, "", "  ")
}
