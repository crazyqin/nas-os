package plugin

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	neturl "net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager 插件管理器.
type Manager struct {
	loader     *Loader
	pluginDir  string
	configDir  string
	dataDir    string
	states     map[string]*State
	hooks      map[HookType][]Hook
	extensions map[string]*ExtensionPoint
	mu         sync.RWMutex
	stateFile  string
}

// ManagerConfig 管理器配置.
type ManagerConfig struct {
	PluginDir string // 插件目录
	ConfigDir string // 配置目录
	DataDir   string // 数据目录
}

// NewManager 创建插件管理器.
func NewManager(cfg ManagerConfig) (*Manager, error) {
	if cfg.PluginDir == "" {
		cfg.PluginDir = "/opt/nas/plugins"
	}
	if cfg.ConfigDir == "" {
		cfg.ConfigDir = "/etc/nas-os/plugins"
	}
	if cfg.DataDir == "" {
		cfg.DataDir = "/var/lib/nas-os/plugins"
	}

	// 创建必要目录
	dirs := []string{cfg.PluginDir, cfg.ConfigDir, cfg.DataDir}
	for _, dir := range dirs {
		if err := os.MkdirAll(dir, 0750); err != nil {
			return nil, fmt.Errorf("创建目录 %s 失败: %w", dir, err)
		}
	}

	m := &Manager{
		loader:     NewLoader(cfg.PluginDir),
		pluginDir:  cfg.PluginDir,
		configDir:  cfg.ConfigDir,
		dataDir:    cfg.DataDir,
		states:     make(map[string]*State),
		hooks:      make(map[HookType][]Hook),
		extensions: make(map[string]*ExtensionPoint),
		stateFile:  filepath.Join(cfg.ConfigDir, "states.json"),
	}

	// 加载已保存的状态
	if err := m.loadStates(); err != nil {
		// 状态文件不存在不是错误
		if !os.IsNotExist(err) {
			return nil, fmt.Errorf("加载插件状态失败: %w", err)
		}
	}

	// 初始化已安装的插件
	if err := m.initInstalledPlugins(); err != nil {
		return nil, fmt.Errorf("初始化已安装插件失败: %w", err)
	}

	return m, nil
}

// initInstalledPlugins 初始化已安装的插件.
func (m *Manager) initInstalledPlugins() error {
	entries, err := os.ReadDir(m.pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	for _, entry := range entries {
		var pluginPath string
		if entry.IsDir() {
			pluginPath = filepath.Join(m.pluginDir, entry.Name())
		} else if strings.HasSuffix(entry.Name(), ".so") {
			pluginPath = filepath.Join(m.pluginDir, entry.Name())
		} else {
			continue
		}

		// 尝试加载插件
		inst, err := m.loader.Load(pluginPath)
		if err != nil {
			continue // 忽略加载失败的插件
		}

		// 恢复状态
		if state, exists := m.states[inst.Info.ID]; exists {
			inst.Enabled = state.Enabled
			inst.State = *state

			// 如果之前是启用的，现在也启动
			if state.Enabled {
				if err := m.StartPlugin(inst.Info.ID); err != nil {
					inst.State.Error = err.Error()
				}
			}
		}
	}

	return nil
}

// Discover 发现可用插件.
func (m *Manager) Discover() ([]Info, error) {
	return m.loader.Discover()
}

// List 列出所有插件状态.
func (m *Manager) List() []State {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]State, 0, len(m.states))
	for _, state := range m.states {
		result = append(result, *state)
	}
	return result
}

// Get 获取插件详情.
func (m *Manager) Get(pluginID string) (*State, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	state, exists := m.states[pluginID]
	if !exists {
		return nil, fmt.Errorf("插件 %s 不存在", pluginID)
	}
	return state, nil
}

// Install 安装插件.
func (m *Manager) Install(source string) (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var pluginPath string
	var err error

	// 判断来源类型
	if strings.HasPrefix(source, "http://") || strings.HasPrefix(source, "https://") {
		// 从 URL 下载
		pluginPath, err = m.downloadPlugin(source)
	} else if strings.HasSuffix(source, ".so") || isDir(source) {
		// 本地文件或目录
		pluginPath, err = m.copyPlugin(source)
	} else {
		// 假设是插件 ID，从市场下载
		pluginPath, err = m.installFromMarket(source)
	}

	if err != nil {
		return nil, err
	}

	// 加载插件
	inst, err := m.loader.Load(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("加载插件失败: %w", err)
	}

	// 检查依赖
	if err := m.checkDependencies(inst.Info.Dependencies); err != nil {
		_ = m.loader.Unload(inst.Info.ID)
		return nil, fmt.Errorf("依赖检查失败: %w", err)
	}

	// 创建状态
	state := &State{
		ID:          inst.Info.ID,
		Version:     inst.Info.Version,
		Enabled:     false,
		Running:     false,
		Installed:   true,
		InstalledAt: time.Now(),
		UpdatedAt:   time.Now(),
	}
	m.states[inst.Info.ID] = state

	// 保存状态
	if err := m.saveStates(); err != nil {
		log.Printf("保存状态失败: %v", err)
	}

	return state, nil
}

// Uninstall 卸载插件.
func (m *Manager) Uninstall(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[pluginID]
	if !exists {
		return fmt.Errorf("插件 %s 不存在", pluginID)
	}

	// 检查是否有其他插件依赖
	for _, s := range m.states {
		deps := m.getDependenciesFromState(s)
		for _, dep := range deps {
			if dep.ID == pluginID && !dep.Optional {
				return fmt.Errorf("插件 %s 依赖 %s，无法卸载", s.ID, pluginID)
			}
		}
	}

	// 停止并卸载
	if state.Running {
		if err := m.stopPlugin(pluginID); err != nil {
			return err
		}
	}

	if err := m.loader.Unload(pluginID); err != nil {
		return err
	}

	// 删除插件文件
	inst, _ := m.loader.GetInstance(pluginID)
	if inst != nil {
		_ = os.Remove(inst.Path) // 显式忽略错误，插件文件可能已被删除
	}

	// 删除配置文件
	configPath := filepath.Join(m.configDir, pluginID+".json")
	_ = os.Remove(configPath) // 显式忽略错误，配置文件可能不存在

	// 删除状态
	delete(m.states, pluginID)

	return m.saveStates()
}

// Enable 启用插件.
func (m *Manager) Enable(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[pluginID]
	if !exists {
		return fmt.Errorf("插件 %s 不存在", pluginID)
	}

	if state.Enabled {
		return nil // 已经启用
	}

	inst, exists := m.loader.GetInstance(pluginID)
	if !exists {
		// 重新加载插件
		pluginPath := filepath.Join(m.pluginDir, pluginID)
		var err error
		inst, err = m.loader.Load(pluginPath)
		if err != nil {
			return fmt.Errorf("加载插件失败: %w", err)
		}
	}

	// 加载配置
	config, err := m.loadConfig(pluginID)
	if err != nil && !os.IsNotExist(err) {
		return fmt.Errorf("加载配置失败: %w", err)
	}

	// 初始化插件
	if err := inst.Plugin.Init(config); err != nil {
		return fmt.Errorf("初始化插件失败: %w", err)
	}

	// 启动插件
	if err := inst.Plugin.Start(); err != nil {
		return fmt.Errorf("启动插件失败: %w", err)
	}

	state.Enabled = true
	state.Running = true
	state.UpdatedAt = time.Now()
	inst.Enabled = true
	inst.Running = true

	if err := m.saveStates(); err != nil {
		log.Printf("保存状态失败: %v", err)
	}
	return nil
}

// Disable 禁用插件.
func (m *Manager) Disable(pluginID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[pluginID]
	if !exists {
		return fmt.Errorf("插件 %s 不存在", pluginID)
	}

	if !state.Enabled {
		return nil // 已经禁用
	}

	// 停止插件
	if err := m.stopPlugin(pluginID); err != nil {
		return err
	}

	state.Enabled = false
	state.UpdatedAt = time.Now()

	if err := m.saveStates(); err != nil {
		log.Printf("保存状态失败: %v", err)
	}
	return nil
}

// StartPlugin 启动插件.
func (m *Manager) StartPlugin(pluginID string) error {
	return m.Enable(pluginID)
}

// StopPlugin 停止插件.
func (m *Manager) StopPlugin(pluginID string) error {
	return m.Disable(pluginID)
}

// stopPlugin 内部停止方法.
func (m *Manager) stopPlugin(pluginID string) error {
	inst, exists := m.loader.GetInstance(pluginID)
	if !exists {
		return nil
	}

	if !inst.Running {
		return nil
	}

	if err := inst.Plugin.Stop(); err != nil {
		return fmt.Errorf("停止插件失败: %w", err)
	}

	inst.Running = false
	if state, ok := m.states[pluginID]; ok {
		state.Running = false
	}

	return nil
}

// Update 更新插件.
func (m *Manager) Update(pluginID string) (*State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	state, exists := m.states[pluginID]
	if !exists {
		return nil, fmt.Errorf("插件 %s 不存在", pluginID)
	}

	// 保存当前状态
	wasEnabled := state.Enabled

	// 禁用插件
	if wasEnabled {
		if err := m.stopPlugin(pluginID); err != nil {
			return nil, err
		}
	}

	// 从市场下载更新版本
	pluginPath, err := m.installFromMarket(pluginID)
	if err != nil {
		return nil, err
	}

	// 重新加载
	if err := m.loader.Unload(pluginID); err != nil {
		// 忽略卸载错误（插件可能未加载）
		_ = err // 明确忽略错误，避免 staticcheck 警告
	}

	inst, err := m.loader.Load(pluginPath)
	if err != nil {
		return nil, fmt.Errorf("重新加载插件失败: %w", err)
	}

	state.Version = inst.Info.Version
	state.UpdatedAt = time.Now()

	// 如果之前是启用的，重新启用
	if wasEnabled {
		if err := m.Enable(pluginID); err != nil {
			state.Error = err.Error()
		}
	}

	if err := m.saveStates(); err != nil {
		log.Printf("保存状态失败: %v", err)
	}
	return state, nil
}

// Configure 配置插件.
func (m *Manager) Configure(pluginID string, config map[string]interface{}) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	_, exists := m.states[pluginID]
	if !exists {
		return fmt.Errorf("插件 %s 不存在", pluginID)
	}

	// 保存配置
	configPath := filepath.Join(m.configDir, pluginID+".json")
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	if err := os.WriteFile(configPath, data, 0640); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}

	// 如果插件正在运行，重新加载配置
	inst, exists := m.loader.GetInstance(pluginID)
	if exists && inst.Running {
		_ = inst.Plugin.Stop()
		_ = inst.Plugin.Init(config)
		_ = inst.Plugin.Start()
	}

	return nil
}

// RegisterHook 注册钩子.
func (m *Manager) RegisterHook(hookType HookType, hook Hook) {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.hooks[hookType] = append(m.hooks[hookType], hook)
}

// ExecuteHooks 执行钩子.
func (m *Manager) ExecuteHooks(hookType HookType, ctx HookContext) error {
	m.mu.RLock()
	hooks := m.hooks[hookType]
	m.mu.RUnlock()

	for _, hook := range hooks {
		if err := hook(ctx); err != nil {
			return err
		}
	}
	return nil
}

// RegisterExtension 注册扩展.
func (m *Manager) RegisterExtension(ext *Extension) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	point, exists := m.extensions[ext.PointID]
	if !exists {
		point = &ExtensionPoint{
			ID:         ext.PointID,
			Name:       ext.PointID,
			Extensions: []*Extension{},
		}
		m.extensions[ext.PointID] = point
	}

	point.Extensions = append(point.Extensions, ext)
	return nil
}

// GetExtensions 获取扩展点的所有扩展.
func (m *Manager) GetExtensions(pointID string) []*Extension {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if point, exists := m.extensions[pointID]; exists {
		return point.Extensions
	}
	return nil
}

// downloadPlugin 从 URL 下载插件.
func (m *Manager) downloadPlugin(url string) (string, error) {
	// 安全检查：只允许 HTTPS URL（除非在测试环境）
	if !strings.HasPrefix(url, "https://") {
		if os.Getenv("ENV") != "test" && os.Getenv("ALLOW_HTTP_PLUGIN") != "true" {
			return "", fmt.Errorf("安全限制：只允许从 HTTPS URL 下载插件")
		}
	}

	// 解析并验证 URL
	parsedURL, err := neturl.Parse(url)
	if err != nil {
		return "", fmt.Errorf("无效的 URL: %w", err)
	}

	// 防止访问内部网络地址（SSRF 防护）
	host := parsedURL.Hostname()
	if host == "localhost" || host == "127.0.0.1" || strings.HasPrefix(host, "192.168.") ||
		strings.HasPrefix(host, "10.") || strings.HasPrefix(host, "172.") {
		if os.Getenv("ENV") != "test" && os.Getenv("ALLOW_PRIVATE_NETWORK_PLUGIN") != "true" {
			return "", fmt.Errorf("安全限制：不允许从私有网络地址下载插件")
		}
	}

	// #nosec G107 -- URL is validated above with SSRF protection
	resp, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("下载失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("下载失败: HTTP %d", resp.StatusCode)
	}

	// 生成临时文件名
	filename := filepath.Base(url)
	if !strings.HasSuffix(filename, ".so") {
		filename += ".so"
	}

	tmpPath := filepath.Join(m.pluginDir, filename+".tmp")
	outPath := filepath.Join(m.pluginDir, filename)

	// 创建临时文件
	out, err := os.Create(tmpPath)
	if err != nil {
		return "", err
	}
	defer func() { _ = out.Close() }()

	// 复制内容
	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}

	// 重命名为最终文件
	if err := os.Rename(tmpPath, outPath); err != nil {
		_ = os.Remove(tmpPath)
		return "", err
	}

	return outPath, nil
}

// copyPlugin 复制本地插件.
func (m *Manager) copyPlugin(source string) (string, error) {
	info, err := os.Stat(source)
	if err != nil {
		return "", err
	}

	if info.IsDir() {
		// 复制整个目录
		targetDir := filepath.Join(m.pluginDir, info.Name())
		if err := copyDir(source, targetDir); err != nil {
			return "", err
		}
		return targetDir, nil
	}

	// 复制 .so 文件
	targetPath := filepath.Join(m.pluginDir, info.Name())
	if err := copyFile(source, targetPath); err != nil {
		return "", err
	}
	return targetPath, nil
}

// installFromMarket 从市场安装插件.
func (m *Manager) installFromMarket(pluginID string) (string, error) {
	// 这里应该调用市场 API 下载插件
	// 暂时返回错误，后续实现
	return "", fmt.Errorf("插件市场暂不可用")
}

// checkDependencies 检查依赖.
func (m *Manager) checkDependencies(deps []Dependency) error {
	for _, dep := range deps {
		if dep.Optional {
			continue
		}

		state, exists := m.states[dep.ID]
		if !exists || !state.Installed {
			return fmt.Errorf("缺少依赖: %s", dep.ID)
		}
	}
	return nil
}

// loadConfig 加载插件配置.
func (m *Manager) loadConfig(pluginID string) (map[string]interface{}, error) {
	configPath := filepath.Join(m.configDir, pluginID+".json")
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}

	var config map[string]interface{}
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, err
	}

	return config, nil
}

// loadStates 加载状态.
func (m *Manager) loadStates() error {
	data, err := os.ReadFile(m.stateFile)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.states)
}

// saveStates 保存状态.
func (m *Manager) saveStates() error {
	data, err := json.MarshalIndent(m.states, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.stateFile, data, 0640)
}

// getDependenciesFromState 从状态获取依赖.
func (m *Manager) getDependenciesFromState(state *State) []Dependency {
	if state == nil {
		return nil
	}

	// 从已加载的插件实例中获取依赖信息
	inst, exists := m.loader.GetInstance(state.ID)
	if !exists {
		return nil
	}

	return inst.Info.Dependencies
}

// isDir 检查是否是目录.
func isDir(path string) bool {
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return info.IsDir()
}

// copyFile 复制文件.
func copyFile(src, dst string) error {
	source, err := os.Open(src)
	if err != nil {
		return err
	}
	defer func() { _ = source.Close() }()

	destination, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer func() { _ = destination.Close() }()

	_, err = io.Copy(destination, source)
	return err
}

// copyDir 复制目录.
func copyDir(src, dst string) error {
	if err := os.MkdirAll(dst, 0750); err != nil {
		return err
	}

	entries, err := os.ReadDir(src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		srcPath := filepath.Join(src, entry.Name())
		dstPath := filepath.Join(dst, entry.Name())

		if entry.IsDir() {
			if err := copyDir(srcPath, dstPath); err != nil {
				return err
			}
		} else {
			if err := copyFile(srcPath, dstPath); err != nil {
				return err
			}
		}
	}

	return nil
}

// hashFile 计算文件哈希 - 保留用于未来需要校验插件完整性的场景
// func hashFile(path string) (string, error) {
// 	file, err := os.Open(path)
// 	if err != nil {
// 		return "", err
// 	}
// 	defer func() { _ = file.Close() }()
//
// 	hash := sha256.New()
// 	if _, err := io.Copy(hash, file); err != nil {
// 		return "", err
// 	}
//
// 	return hex.EncodeToString(hash.Sum(nil)), nil
// }
