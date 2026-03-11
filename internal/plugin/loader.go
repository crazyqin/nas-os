package plugin

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"plugin"
	"strings"
	"sync"
	"time"
)

// Loader 插件加载器
type Loader struct {
	pluginDir string
	instances map[string]*PluginInstance
	mu        sync.RWMutex
}

// NewLoader 创建插件加载器
func NewLoader(pluginDir string) *Loader {
	return &Loader{
		pluginDir: pluginDir,
		instances: make(map[string]*PluginInstance),
	}
}

// Discover 发现所有可用插件
func (l *Loader) Discover() ([]PluginInfo, error) {
	var plugins []PluginInfo

	// 遍历插件目录
	entries, err := os.ReadDir(l.pluginDir)
	if err != nil {
		if os.IsNotExist(err) {
			return plugins, nil
		}
		return nil, fmt.Errorf("读取插件目录失败: %w", err)
	}

	for _, entry := range entries {
		if entry.IsDir() {
			// 检查目录中是否有 manifest.json
			manifestPath := filepath.Join(l.pluginDir, entry.Name(), "manifest.json")
			if info, err := l.loadManifest(manifestPath); err == nil {
				plugins = append(plugins, info)
			}
		} else if strings.HasSuffix(entry.Name(), ".so") {
			// 尝试直接加载 .so 文件
			soPath := filepath.Join(l.pluginDir, entry.Name())
			if info, err := l.loadSOInfo(soPath); err == nil {
				plugins = append(plugins, info)
			}
		}
	}

	return plugins, nil
}

// Load 加载插件
func (l *Loader) Load(pluginPath string) (*PluginInstance, error) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// 检查是否已加载
	for _, inst := range l.instances {
		if inst.Path == pluginPath {
			return inst, nil
		}
	}

	// 确定是 .so 文件还是插件目录
	var info PluginInfo
	var pluginPathActual string

	if strings.HasSuffix(pluginPath, ".so") {
		// 直接加载 .so 文件
		pluginPathActual = pluginPath
		loadedInfo, err := l.loadSOInfo(pluginPath)
		if err != nil {
			return nil, err
		}
		info = loadedInfo
	} else {
		// 从目录加载
		manifestPath := filepath.Join(pluginPath, "manifest.json")
		loadedInfo, err := l.loadManifest(manifestPath)
		if err != nil {
			return nil, fmt.Errorf("加载插件清单失败: %w", err)
		}
		info = loadedInfo

		// 查找 .so 文件
		soPath := filepath.Join(pluginPath, info.MainFile)
		if _, err := os.Stat(soPath); os.IsNotExist(err) {
			// 尝试查找目录中的 .so 文件
			entries, _ := os.ReadDir(pluginPath)
			for _, e := range entries {
				if strings.HasSuffix(e.Name(), ".so") {
					soPath = filepath.Join(pluginPath, e.Name())
					break
				}
			}
		}
		pluginPathActual = soPath
	}

	// 打开 .so 文件
	plug, err := plugin.Open(pluginPathActual)
	if err != nil {
		return nil, fmt.Errorf("打开插件失败: %w", err)
	}

	// 查找入口函数
	entrypoint := info.Entrypoint
	if entrypoint == "" {
		entrypoint = "New" // 默认入口函数
	}

	sym, err := plug.Lookup(entrypoint)
	if err != nil {
		return nil, fmt.Errorf("查找入口函数 %s 失败: %w", entrypoint, err)
	}

	// 类型断言
	newPluginFunc, ok := sym.(func() Plugin)
	if !ok {
		return nil, fmt.Errorf("入口函数 %s 类型不正确，期望 func() Plugin", entrypoint)
	}

	// 创建插件实例
	pluginInstance := newPluginFunc()

	instance := &PluginInstance{
		Info:    info,
		Plugin:  pluginInstance,
		Path:    pluginPathActual,
		Enabled: false,
		Running: false,
		State: PluginState{
			ID:        info.ID,
			Version:   info.Version,
			Installed: true,
			InstalledAt: timeNow(),
		},
	}

	l.instances[info.ID] = instance
	return instance, nil
}

// Unload 卸载插件
func (l *Loader) Unload(pluginID string) error {
	l.mu.Lock()
	defer l.mu.Unlock()

	instance, exists := l.instances[pluginID]
	if !exists {
		return fmt.Errorf("插件 %s 未加载", pluginID)
	}

	// 如果正在运行，先停止
	if instance.Running {
		if err := instance.Plugin.Stop(); err != nil {
			return fmt.Errorf("停止插件失败: %w", err)
		}
		instance.Running = false
	}

	// 销毁插件
	if err := instance.Plugin.Destroy(); err != nil {
		return fmt.Errorf("销毁插件失败: %w", err)
	}

	delete(l.instances, pluginID)
	return nil
}

// GetInstance 获取插件实例
func (l *Loader) GetInstance(pluginID string) (*PluginInstance, bool) {
	l.mu.RLock()
	defer l.mu.RUnlock()
	inst, ok := l.instances[pluginID]
	return inst, ok
}

// ListInstances 列出所有已加载的插件实例
func (l *Loader) ListInstances() []*PluginInstance {
	l.mu.RLock()
	defer l.mu.RUnlock()

	instances := make([]*PluginInstance, 0, len(l.instances))
	for _, inst := range l.instances {
		instances = append(instances, inst)
	}
	return instances
}

// loadManifest 加载插件清单
func (l *Loader) loadManifest(manifestPath string) (PluginInfo, error) {
	var info PluginInfo

	data, err := os.ReadFile(manifestPath)
	if err != nil {
		return info, fmt.Errorf("读取清单文件失败: %w", err)
	}

	if err := json.Unmarshal(data, &info); err != nil {
		return info, fmt.Errorf("解析清单文件失败: %w", err)
	}

	return info, nil
}

// loadSOInfo 从 .so 文件加载插件信息
func (l *Loader) loadSOInfo(soPath string) (PluginInfo, error) {
	// 尝试打开 .so 文件
	plug, err := plugin.Open(soPath)
	if err != nil {
		return PluginInfo{}, fmt.Errorf("打开 .so 文件失败: %w", err)
	}

	// 查找 Info 符号
	sym, err := plug.Lookup("Info")
	if err == nil {
		if infoFunc, ok := sym.(func() PluginInfo); ok {
			return infoFunc(), nil
		}
	}

	// 查找 PluginInfo 变量
	sym, err = plug.Lookup("PluginInfo")
	if err == nil {
		if info, ok := sym.(*PluginInfo); ok {
			return *info, nil
		}
	}

	// 使用文件名作为插件 ID
	filename := filepath.Base(soPath)
	id := strings.TrimSuffix(filename, ".so")
	id = strings.ReplaceAll(id, "_", "-")

	return PluginInfo{
		ID:   "plugin." + id,
		Name: id,
		Version: "1.0.0",
		MainFile: soPath,
	}, nil
}

// timeNow 返回当前时间（便于测试）
var timeNow = func() time.Time {
	return time.Now()
}