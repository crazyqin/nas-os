// Package cloudfuse provides cloud storage mounting via FUSE
// 挂载管理器 - 管理多个网盘挂载点
package cloudfuse

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"nas-os/internal/cloudsync"

	"bazil.org/fuse"
	"bazil.org/fuse/fs"
)

// Manager 挂载管理器.
type Manager struct {
	mu         sync.RWMutex
	mounts     map[string]*MountInstance // id -> mount instance
	configPath string
	config     *Config
	providers  map[string]cloudsync.Provider // provider id -> provider
	ctx        context.Context
	cancel     context.CancelFunc
	configDir  string
}

// MountInstance 挂载实例.
type MountInstance struct {
	ID           string
	Config       *MountConfig
	MountPoint   string
	Status       MountStatus
	Provider     cloudsync.Provider
	FuseConn     *fuse.Conn
	FileSystem   *CloudFS
	CacheManager *CacheManager
	Error        string
	MountedAt    time.Time
	Stats        *MountStats
}

// Config cloudfuse 配置.
type Config struct {
	Version    string        `json:"version"`
	Mounts     []MountConfig `json:"mounts"`
	ProviderID string        `json:"providerId,omitempty"`
}

// NewManager 创建挂载管理器.
func NewManager(configPath string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		mounts:     make(map[string]*MountInstance),
		configPath: configPath,
		providers:  make(map[string]cloudsync.Provider),
		ctx:        ctx,
		cancel:     cancel,
		config: &Config{
			Version: "1.0",
			Mounts:  []MountConfig{},
		},
	}

	// 设置默认配置目录
	if configPath == "" {
		m.configDir = "/etc/nas-os/cloudfuse"
		m.configPath = filepath.Join(m.configDir, "config.json")
	} else {
		m.configDir = filepath.Dir(configPath)
	}

	return m
}

// Initialize 初始化管理器.
func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 加载配置文件
	if err := m.loadConfig(); err != nil {
		// 配置文件不存在，创建默认配置
		if os.IsNotExist(err) {
			if err := os.MkdirAll(m.configDir, 0750); err != nil {
				return fmt.Errorf("创建配置目录失败: %w", err)
			}
			if err := m.saveConfig(); err != nil {
				return fmt.Errorf("保存默认配置失败: %w", err)
			}
		} else {
			return fmt.Errorf("加载配置失败: %w", err)
		}
	}

	// 自动挂载配置中标记为 autoMount 的挂载点
	for i := range m.config.Mounts {
		cfg := &m.config.Mounts[i]
		if cfg.AutoMount && cfg.Enabled {
			// 异步挂载，避免阻塞初始化
			go func(c *MountConfig) {
				_, _ = m.Mount(c)
			}(cfg)
		}
	}

	return nil
}

// loadConfig 加载配置文件.
func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, m.config)
}

// saveConfig 保存配置文件.
func (m *Manager) saveConfig() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0600)
}

// RegisterProvider 注册云存储提供商.
func (m *Manager) RegisterProvider(providerID string, provider cloudsync.Provider) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.providers[providerID] = provider
}

// CreateProvider 从配置创建提供商.
func (m *Manager) CreateProvider(mountConfig *MountConfig) (cloudsync.Provider, error) {
	providerConfig := &cloudsync.ProviderConfig{
		Type:         cloudsync.ProviderType(mountConfig.Type),
		AccessToken:  mountConfig.AccessToken,
		RefreshToken: mountConfig.RefreshToken,
		UserID:       mountConfig.UserID,
		DriveID:      mountConfig.DriveID,
		Endpoint:     mountConfig.Endpoint,
		Bucket:       mountConfig.Bucket,
		AccessKey:    mountConfig.AccessKey,
		SecretKey:    mountConfig.SecretKey,
		Region:       mountConfig.Region,
		PathStyle:    mountConfig.PathStyle,
		Insecure:     mountConfig.Insecure,
		ClientID:     mountConfig.ClientID,
		TenantID:     mountConfig.TenantID,
		RootFolderID: mountConfig.RootFolder,
	}

	return cloudsync.NewProvider(m.ctx, providerConfig)
}

// Mount 创建挂载.
func (m *Manager) Mount(cfg *MountConfig) (*MountInfo, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否已存在
	if _, exists := m.mounts[cfg.ID]; exists {
		return nil, fmt.Errorf("挂载点 %s 已存在", cfg.ID)
	}

	// 验证配置
	if err := m.validateMountConfig(cfg); err != nil {
		return nil, fmt.Errorf("配置验证失败: %w", err)
	}

	// 创建挂载实例
	instance := &MountInstance{
		ID:     cfg.ID,
		Config: cfg,
		Status: MountStatusMounting,
		Stats: &MountStats{
			MountID:   cfg.ID,
			StartTime: time.Now(),
		},
	}
	m.mounts[cfg.ID] = instance

	// 创建提供商
	provider, err := m.CreateProvider(cfg)
	if err != nil {
		instance.Status = MountStatusError
		instance.Error = fmt.Sprintf("创建提供商失败: %v", err)
		return m.getInstanceInfo(instance), err
	}
	instance.Provider = provider

	// 测试连接
	testResult, err := provider.TestConnection(m.ctx)
	if err != nil || !testResult.Success {
		instance.Status = MountStatusError
		instance.Error = fmt.Sprintf("连接测试失败: %v", testResult.Message)
		return m.getInstanceInfo(instance), fmt.Errorf("连接测试失败: %s", testResult.Message)
	}

	// 创建缓存管理器
	if cfg.CacheEnabled {
		cacheDir := cfg.CacheDir
		if cacheDir == "" {
			cacheDir = filepath.Join(m.configDir, "cache", cfg.ID)
		}
		cacheSize := cfg.CacheSize
		if cacheSize == 0 {
			cacheSize = 1024 // 默认 1GB
		}

		cacheManager, err := NewCacheManager(cacheDir, cacheSize)
		if err != nil {
			instance.Status = MountStatusError
			instance.Error = fmt.Sprintf("创建缓存管理器失败: %v", err)
			return m.getInstanceInfo(instance), err
		}
		instance.CacheManager = cacheManager
	}

	// 确保挂载点目录存在
	if err := os.MkdirAll(cfg.MountPoint, 0755); err != nil {
		instance.Status = MountStatusError
		instance.Error = fmt.Sprintf("创建挂载点目录失败: %v", err)
		return m.getInstanceInfo(instance), err
	}

	// 创建 FUSE 文件系统
	fileSystem, err := NewCloudFS(cfg, provider, instance.CacheManager)
	if err != nil {
		instance.Status = MountStatusError
		instance.Error = fmt.Sprintf("创建文件系统失败: %v", err)
		return m.getInstanceInfo(instance), err
	}
	instance.FileSystem = fileSystem

	// 挂载 FUSE
	options := []fuse.MountOption{
		fuse.FSName("cloudfuse-" + cfg.ID),
		fuse.Subtype("cloudfuse"),
	}

	if cfg.AllowOther {
		options = append(options, fuse.AllowOther())
	}

	fuseConn, err := fuse.Mount(cfg.MountPoint, options...)
	if err != nil {
		instance.Status = MountStatusError
		instance.Error = fmt.Sprintf("挂载 FUSE 失败: %v", err)
		return m.getInstanceInfo(instance), err
	}
	instance.FuseConn = fuseConn

	// 启动文件系统服务
	go m.serveFuse(instance)

	// 更新状态
	instance.Status = MountStatusMounted
	instance.MountedAt = time.Now()

	// 添加到配置
	m.addMountConfigInternal(cfg)

	return m.getInstanceInfo(instance), nil
}

// serveFuse 服务 FUSE 连接.
func (m *Manager) serveFuse(instance *MountInstance) {
	err := fs.Serve(instance.FuseConn, instance.FileSystem)
	if err != nil {
		m.mu.Lock()
		instance.Status = MountStatusError
		instance.Error = fmt.Sprintf("FUSE 服务错误: %v", err)
		m.mu.Unlock()
	}
}

// Unmount 卸载挂载.
func (m *Manager) Unmount(mountID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.mounts[mountID]
	if !exists {
		return fmt.Errorf("挂载点 %s 不存在", mountID)
	}

	instance.Status = MountStatusUnmounting

	// 卸载 FUSE
	if instance.FuseConn != nil {
		if err := instance.FuseConn.Close(); err != nil {
			instance.Status = MountStatusError
			instance.Error = fmt.Sprintf("卸载 FUSE 失败: %v", err)
			return err
		}

		// 执行系统卸载
		_ = fuse.Unmount(instance.Config.MountPoint)
	}

	// 关闭缓存
	if instance.CacheManager != nil {
		_ = instance.CacheManager.Close()
	}

	// 关闭提供商
	if instance.Provider != nil {
		_ = instance.Provider.Close()
	}

	// 删除实例
	delete(m.mounts, mountID)

	return nil
}

// GetMount 获取挂载信息.
func (m *Manager) GetMount(mountID string) (*MountInfo, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, exists := m.mounts[mountID]
	if !exists {
		return nil, fmt.Errorf("挂载点 %s 不存在", mountID)
	}

	return m.getInstanceInfo(instance), nil
}

// ListMounts 列出所有挂载.
func (m *Manager) ListMounts() []MountInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	list := make([]MountInfo, 0, len(m.mounts))
	for _, instance := range m.mounts {
		list = append(list, *m.getInstanceInfo(instance))
	}

	return list
}

// getInstanceInfo 从实例获取信息.
func (m *Manager) getInstanceInfo(instance *MountInstance) *MountInfo {
	info := &MountInfo{
		ID:         instance.ID,
		Name:       instance.Config.Name,
		Type:       instance.Config.Type,
		MountPoint: instance.Config.MountPoint,
		Status:     instance.Status,
		CreatedAt:  instance.Config.CreatedAt,
		MountedAt:  &instance.MountedAt,
		Error:      instance.Error,
	}

	if instance.Stats != nil {
		info.ReadBytes = instance.Stats.TotalReadBytes
		info.WriteBytes = instance.Stats.TotalWriteBytes
		info.ReadOps = instance.Stats.TotalReadOps
		info.WriteOps = instance.Stats.TotalWriteOps
	}

	if instance.CacheManager != nil {
		info.CacheHitRate = instance.CacheManager.HitRate()
		info.CacheUsedBytes = instance.CacheManager.UsedSize()
	}

	return info
}

// validateMountConfig 验证挂载配置.
func (m *Manager) validateMountConfig(cfg *MountConfig) error {
	if cfg.ID == "" {
		cfg.ID = fmt.Sprintf("mount-%d", time.Now().Unix())
	}
	if cfg.Name == "" {
		return fmt.Errorf("挂载名称不能为空")
	}
	if cfg.MountPoint == "" {
		return fmt.Errorf("挂载点路径不能为空")
	}
	if cfg.Type == "" {
		return fmt.Errorf("挂载类型不能为空")
	}

	// 设置默认值
	if cfg.CreatedAt.IsZero() {
		cfg.CreatedAt = time.Now()
	}
	cfg.UpdatedAt = time.Now()

	return nil
}

// AddMountConfig 添加挂载配置（公开方法）.
func (m *Manager) AddMountConfig(cfg *MountConfig) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.addMountConfigInternal(cfg)
}

// addMountConfigInternal 内部方法：添加挂载配置.
func (m *Manager) addMountConfigInternal(cfg *MountConfig) {
	// 检查是否已存在
	for i, c := range m.config.Mounts {
		if c.ID == cfg.ID {
			m.config.Mounts[i] = *cfg
			_ = m.saveConfig()
			return
		}
	}

	m.config.Mounts = append(m.config.Mounts, *cfg)
	_ = m.saveConfig()
}

// RemoveMountConfig 删除挂载配置.
func (m *Manager) RemoveMountConfig(mountID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, cfg := range m.config.Mounts {
		if cfg.ID == mountID {
			m.config.Mounts = append(m.config.Mounts[:i], m.config.Mounts[i+1:]...)
			return m.saveConfig()
		}
	}

	return nil
}

// UpdateMountConfig 更新挂载配置.
func (m *Manager) UpdateMountConfig(mountID string, cfg *MountConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	for i, c := range m.config.Mounts {
		if c.ID == mountID {
			cfg.ID = mountID
			cfg.UpdatedAt = time.Now()
			m.config.Mounts[i] = *cfg
			return m.saveConfig()
		}
	}

	return fmt.Errorf("挂载点 %s 不存在", mountID)
}

// GetStats 获取挂载统计.
func (m *Manager) GetStats(mountID string) (*MountStats, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, exists := m.mounts[mountID]
	if !exists {
		return nil, fmt.Errorf("挂载点 %s 不存在", mountID)
	}

	if instance.FileSystem == nil || instance.FileSystem.stats == nil {
		return &MountStats{MountID: mountID}, nil
	}

	stats := instance.FileSystem.stats
	stats.Uptime = int64(time.Since(stats.StartTime).Seconds())
	return stats, nil
}

// TestMountConfig 测试挂载配置.
func (m *Manager) TestMountConfig(cfg *MountConfig) (*cloudsync.ConnectionTestResult, error) {
	provider, err := m.CreateProvider(cfg)
	if err != nil {
		return nil, err
	}
	defer func() { _ = provider.Close() }()

	return provider.TestConnection(m.ctx)
}

// Close 关闭管理器.
func (m *Manager) Close() error {
	m.cancel()

	m.mu.Lock()
	defer m.mu.Unlock()

	// 卸载所有挂载点
	for id := range m.mounts {
		_ = m.Unmount(id)
	}

	return nil
}
