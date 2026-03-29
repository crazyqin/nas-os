// Package cloudmount - 挂载管理器
// 管理多个网盘挂载点，使用 rclone 作为后端
package cloudmount

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager 挂载管理器.
type Manager struct {
	mu          sync.RWMutex
	mounts      map[string]*MountInstance // id -> mount instance
	config      *GlobalConfig
	configPath  string
	configDir   string
	rclonePath  string
	rcloneConf  string
	ctx         context.Context
	cancel      context.CancelFunc
	eventChan   chan MountEvent
	running     bool
}

// NewManager 创建挂载管理器.
func NewManager(configPath string) *Manager {
	ctx, cancel := context.WithCancel(context.Background())

	m := &Manager{
		mounts:    make(map[string]*MountInstance),
		ctx:       ctx,
		cancel:    cancel,
		eventChan: make(chan MountEvent, 100),
	}

	// 设置配置路径
	if configPath == "" {
		m.configDir = "/var/lib/nas-os/cloudmount"
		m.configPath = filepath.Join(m.configDir, "config.json")
		m.rcloneConf = filepath.Join(m.configDir, "rclone.conf")
	} else {
		m.configDir = filepath.Dir(configPath)
		m.configPath = configPath
		m.rcloneConf = filepath.Join(m.configDir, "rclone.conf")
	}

	// 默认配置
	m.config = &GlobalConfig{
		Version:        "1.0",
		DefaultConfig:  GetDefaultSettings(),
		Mounts:         []CloudMountConfig{},
		RclonePath:     "/usr/bin/rclone",
		RcloneConfPath: m.rcloneConf,
		LogLevel:       "INFO",
	}

	// 检测 rclone 路径
	m.detectRclone()

	return m
}

// detectRclone 检测 rclone 安装路径.
func (m *Manager) detectRclone() {
	// 检查常用路径
	paths := []string{
		"/usr/bin/rclone",
		"/usr/local/bin/rclone",
		"/opt/rclone/rclone",
		"rclone", // PATH 中的 rclone
	}

	for _, path := range paths {
		if _, err := exec.LookPath(path); err == nil {
			m.rclonePath = path
			m.config.RclonePath = path
			return
		}
	}

	// 未找到 rclone
	m.rclonePath = ""
	m.config.RclonePath = ""
}

// Initialize 初始化管理器.
func (m *Manager) Initialize() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 创建配置目录
	if err := os.MkdirAll(m.configDir, 0750); err != nil {
		return fmt.Errorf("创建配置目录失败: %w", err)
	}

	// 创建缓存目录
	cacheDir := m.config.DefaultConfig.CacheDir
	if err := os.MkdirAll(cacheDir, 0750); err != nil {
		return fmt.Errorf("创建缓存目录失败: %w", err)
	}

	// 加载配置
	if err := m.loadConfig(); err != nil {
		if !os.IsNotExist(err) {
			return fmt.Errorf("加载配置失败: %w", err)
		}
		// 配置不存在，保存默认配置
		if err := m.saveConfig(); err != nil {
			return fmt.Errorf("保存默认配置失败: %w", err)
		}
	}

	// 生成 rclone.conf
	if err := m.generateRcloneConf(); err != nil {
		return fmt.Errorf("生成 rclone 配置失败: %w", err)
	}

	// 检查 rclone 可用性
	if m.rclonePath == "" {
		return fmt.Errorf("rclone 未安装，请先安装 rclone: apt install rclone 或 brew install rclone")
	}

	// 验证 rclone 版本
	if err := m.checkRcloneVersion(); err != nil {
		return fmt.Errorf("rclone 版本检查失败: %w", err)
	}

	m.running = true
	return nil
}

// loadConfig 加载配置文件.
func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	if err := json.Unmarshal(data, m.config); err != nil {
		return fmt.Errorf("解析配置失败: %w", err)
	}

	// 初始化挂载实例
	for _, cfg := range m.config.Mounts {
		instance := &MountInstance{
			ID:     cfg.ID,
			Config: &cfg,
			Status: MountStatusIdle,
		}
		m.mounts[cfg.ID] = instance
	}

	return nil
}

// saveConfig 保存配置文件.
func (m *Manager) saveConfig() error {
	data, err := json.MarshalIndent(m.config, "", "  ")
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}

	return os.WriteFile(m.configPath, data, 0600)
}

// checkRcloneVersion 检查 rclone 版本.
func (m *Manager) checkRcloneVersion() error {
	cmd := exec.CommandContext(m.ctx, m.rclonePath, "version")
	output, err := cmd.Output()
	if err != nil {
		return err
	}

	// 解析版本
	lines := strings.Split(string(output), "\n")
	if len(lines) > 0 {
		versionLine := lines[0]
		if strings.Contains(versionLine, "rclone") {
			// 版本信息已获取，可以记录
		}
	}

	return nil
}

// ==================== 挂载操作 ====================

// Mount 挂载云盘.
func (m *Manager) Mount(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.mounts[id]
	if !exists {
		return fmt.Errorf("挂载配置不存在: %s", id)
	}

	if instance.Status == MountStatusMounted {
		return fmt.Errorf("已挂载: %s", id)
	}

	if !instance.Config.Enabled {
		return fmt.Errorf("挂载已禁用: %s", id)
	}

	// 发送事件
	m.sendEvent(MountEvent{
		MountID: id,
		Type:    EventMountStart,
		Status:  MountStatusMounting,
		Message: "开始挂载",
	})

	instance.Status = MountStatusMounting

	// 检查挂载点
	mountPoint := instance.Config.MountPoint
	if mountPoint == "" {
		mountPoint = filepath.Join("/mnt", instance.Config.Name)
		instance.Config.MountPoint = mountPoint
	}

	// 创建挂载点目录
	if err := os.MkdirAll(mountPoint, 0755); err != nil {
		instance.Status = MountStatusError
		instance.Error = err.Error()
		m.sendEvent(MountEvent{
			MountID: id,
			Type:    EventMountFailed,
			Status:  MountStatusError,
			Error:   err.Error(),
		})
		return fmt.Errorf("创建挂载点失败: %w", err)
	}

	// 检查挂载点是否已挂载
	if m.isMounted(mountPoint) {
		instance.Status = MountStatusError
		instance.Error = "挂载点已被占用"
		m.sendEvent(MountEvent{
			MountID: id,
			Type:    EventMountFailed,
			Status:  MountStatusError,
			Error:   "挂载点已被占用",
		})
		return fmt.Errorf("挂载点已被占用: %s", mountPoint)
	}

	// 构建 rclone 命令
	args := m.buildRcloneArgs(instance)
	cmd := exec.CommandContext(m.ctx, m.rclonePath, args...)
	instance.CmdArgs = args

	// 设置环境变量
	cmd.Env = append(os.Environ(),
		fmt.Sprintf("RCLONE_CONFIG=%s", m.rcloneConf),
	)

	// 启动 rclone（后台运行）
	if err := cmd.Start(); err != nil {
		instance.Status = MountStatusError
		instance.Error = err.Error()
		m.sendEvent(MountEvent{
			MountID: id,
			Type:    EventMountFailed,
			Status:  MountStatusError,
			Error:   err.Error(),
		})
		return fmt.Errorf("启动 rclone 失败: %w", err)
	}

	instance.PID = cmd.Process.Pid
	instance.Status = MountStatusMounted
	instance.MountedAt = time.Now()
	instance.MountPoint = mountPoint

	// 等待挂载完成
	go m.waitForMount(cmd, instance)

	m.sendEvent(MountEvent{
		MountID: id,
		Type:    EventMountSuccess,
		Status:  MountStatusMounted,
		Message: fmt.Sprintf("挂载成功: %s -> %s", instance.Config.Name, mountPoint),
	})

	return nil
}

// Unmount 卸载云盘.
func (m *Manager) Unmount(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.mounts[id]
	if !exists {
		return fmt.Errorf("挂载配置不存在: %s", id)
	}

	if instance.Status != MountStatusMounted {
		return fmt.Errorf("未挂载: %s", id)
	}

	m.sendEvent(MountEvent{
		MountID: id,
		Type:    EventUnmountStart,
		Status:  MountStatusUnmounting,
		Message: "开始卸载",
	})

	instance.Status = MountStatusUnmounting

	// 使用 fusermount 卸载（Linux）
	unmountCmd := exec.CommandContext(m.ctx, "fusermount", "-u", instance.MountPoint)
	if err := unmountCmd.Run(); err != nil {
		// 尙试使用 umount
		unmountCmd = exec.CommandContext(m.ctx, "umount", instance.MountPoint)
		if err := unmountCmd.Run(); err != nil {
			instance.Status = MountStatusError
			instance.Error = err.Error()
			m.sendEvent(MountEvent{
				MountID: id,
				Type:    EventUnmountFailed,
				Status:  MountStatusError,
				Error:   err.Error(),
			})
			return fmt.Errorf("卸载失败: %w", err)
		}
	}

	instance.Status = MountStatusIdle
	instance.PID = 0
	instance.Error = ""

	m.sendEvent(MountEvent{
		MountID: id,
		Type:    EventUnmountSuccess,
		Status:  MountStatusIdle,
		Message: "卸载成功",
	})

	return nil
}

// MountAll 挂载所有已启用的云盘.
func (m *Manager) MountAll() error {
	m.mu.RLock()
	ids := make([]string, 0, len(m.mounts))
	for id, instance := range m.mounts {
		if instance.Config.Enabled && instance.Status != MountStatusMounted {
			ids = append(ids, id)
		}
	}
	m.mu.RUnlock()

	errors := make([]error, 0)
	for _, id := range ids {
		if err := m.Mount(id); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("部分挂载失败: %v", errors)
	}
	return nil
}

// UnmountAll 卸载所有挂载.
func (m *Manager) UnmountAll() error {
	m.mu.RLock()
	ids := make([]string, 0, len(m.mounts))
	for id, instance := range m.mounts {
		if instance.Status == MountStatusMounted {
			ids = append(ids, id)
		}
	}
	m.mu.RUnlock()

	errors := make([]error, 0)
	for _, id := range ids {
		if err := m.Unmount(id); err != nil {
			errors = append(errors, err)
		}
	}

	if len(errors) > 0 {
		return fmt.Errorf("部分卸载失败: %v", errors)
	}
	return nil
}

// ==================== 配置管理 ====================

// AddMount 添加挂载配置.
func (m *Manager) AddMount(config *CloudMountConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.ID == "" {
		config.ID = fmt.Sprintf("%s-%d", config.ProviderType, time.Now().Unix())
	}

	if _, exists := m.mounts[config.ID]; exists {
		return fmt.Errorf("挂载配置已存在: %s", config.ID)
	}

	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()
	config.Status = MountStatusIdle

	instance := &MountInstance{
		ID:     config.ID,
		Config: config,
		Status: MountStatusIdle,
	}

	m.mounts[config.ID] = instance
	m.config.Mounts = append(m.config.Mounts, *config)

	// 保存配置
	if err := m.saveConfig(); err != nil {
		return err
	}

	// 更新 rclone.conf
	return m.generateRcloneConf()
}

// UpdateMount 更新挂载配置.
func (m *Manager) UpdateMount(id string, config *CloudMountConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.mounts[id]
	if !exists {
		return fmt.Errorf("挂载配置不存在: %s", id)
	}

	// 如果已挂载，需要先卸载
	if instance.Status == MountStatusMounted {
		m.mu.Unlock()
		if err := m.Unmount(id); err != nil {
			m.mu.Lock()
			return fmt.Errorf("卸载失败，无法更新配置: %w", err)
		}
		m.mu.Lock()
	}

	config.ID = id
	config.UpdatedAt = time.Now()
	instance.Config = config

	// 更新配置列表
	for i, c := range m.config.Mounts {
		if c.ID == id {
			m.config.Mounts[i] = *config
			break
		}
	}

	if err := m.saveConfig(); err != nil {
		return err
	}

	return m.generateRcloneConf()
}

// RemoveMount 删除挂载配置.
func (m *Manager) RemoveMount(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	instance, exists := m.mounts[id]
	if !exists {
		return fmt.Errorf("挂载配置不存在: %s", id)
	}

	// 如果已挂载，需要先卸载
	if instance.Status == MountStatusMounted {
		m.mu.Unlock()
		if err := m.Unmount(id); err != nil {
			m.mu.Lock()
			return fmt.Errorf("卸载失败，无法删除配置: %w", err)
		}
		m.mu.Lock()
	}

	delete(m.mounts, id)

	// 从配置列表移除
	for i, c := range m.config.Mounts {
		if c.ID == id {
			m.config.Mounts = append(m.config.Mounts[:i], m.config.Mounts[i+1:]...)
			break
		}
	}

	if err := m.saveConfig(); err != nil {
		return err
	}

	return m.generateRcloneConf()
}

// GetMount 获取挂载配置.
func (m *Manager) GetMount(id string) (*MountInstance, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	instance, exists := m.mounts[id]
	if !exists {
		return nil, fmt.Errorf("挂载配置不存在: %s", id)
	}
	return instance, nil
}

// ListMounts 列出所有挂载.
func (m *Manager) ListMounts() []*MountInstance {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]*MountInstance, 0, len(m.mounts))
	for _, instance := range m.mounts {
		result = append(result, instance)
	}
	return result
}

// ==================== 状态查询 ====================

// GetStats 获取挂载统计.
func (m *Manager) GetStats(id string) (*MountStats, error) {
	m.mu.RLock()
	instance, exists := m.mounts[id]
	m.mu.RUnlock()

	if !exists {
		return nil, fmt.Errorf("挂载配置不存在: %s", id)
	}

	if instance.Status != MountStatusMounted {
		return nil, fmt.Errorf("未挂载: %s", id)
	}

	// 使用 rclone about 获取存储信息
	stats, err := m.fetchRcloneAbout(instance)
	if err != nil {
		return nil, err
	}

	instance.Stats = stats
	return stats, nil
}

// isMounted 检查挂载点是否已挂载.
func (m *Manager) isMounted(mountPoint string) bool {
	// 使用 mountpoint 命令检查
	cmd := exec.Command("mountpoint", "-q", mountPoint)
	return cmd.Run() == nil
}

// ==================== 内部方法 ====================

// buildRcloneArgs 构建 rclone mount 命令参数.
func (m *Manager) buildRcloneArgs(instance *MountInstance) []string {
	cfg := instance.Config
	defaults := m.config.DefaultConfig

	args := []string{
		"mount",
		fmt.Sprintf("%s:%s", cfg.Name, cfg.RemotePath),
		cfg.MountPoint,
	}

	// VFS 缓存模式
	cacheMode := cfg.CacheMode
	if cacheMode == "" {
		cacheMode = defaults.CacheMode
	}
	args = append(args, "--vfs-cache-mode", string(cacheMode))

	// 缓存目录
	cacheDir := cfg.CacheDir
	if cacheDir == "" {
		cacheDir = defaults.CacheDir
	}
	args = append(args, "--cache-dir", cacheDir)

	// 缓存最大年龄
	cacheMaxAge := cfg.CacheMaxAge
	if cacheMaxAge == 0 {
		cacheMaxAge = defaults.CacheMaxAge
	}
	args = append(args, "--vfs-cache-max-age", fmt.Sprintf("%ds", cacheMaxAge))

	// 缓存最大大小
	cacheMaxSize := cfg.CacheMaxSize
	if cacheMaxSize == 0 {
		cacheMaxSize = defaults.CacheMaxSize
	}
	args = append(args, "--vfs-cache-max-size", fmt.Sprintf("%d", cacheMaxSize))

	// 分块大小
	chunkSize := cfg.ChunkSize
	if chunkSize == 0 {
		chunkSize = defaults.ChunkSize
	}
	args = append(args, "--vfs-read-chunk-size", fmt.Sprintf("%d", chunkSize))

	// 只读模式
	if cfg.ReadOnly {
		args = append(args, "--read-only")
	}

	// 允许其他用户访问
	if cfg.AllowOther {
		args = append(args, "--allow-other")
	}

	// 允许 root 访问
	if cfg.AllowRoot {
		args = append(args, "--allow-root")
	}

	// 带宽限制
	if cfg.BandwidthLimit > 0 {
		args = append(args, "--bwlimit", fmt.Sprintf("%dK", cfg.BandwidthLimit))
	}

	// 超时
	timeout := cfg.Timeout
	if timeout == 0 {
		timeout = defaults.Timeout
	}
	args = append(args, "--contimeout", fmt.Sprintf("%ds", timeout))
	args = append(args, "--timeout", fmt.Sprintf("%ds", timeout))

	// 重试次数
	retryCount := cfg.RetryCount
	if retryCount == 0 {
		retryCount = defaults.RetryCount
	}
	args = append(args, "--retries", fmt.Sprintf("%d", retryCount))

	// 日志级别
	args = append(args, "--log-level", m.config.LogLevel)

	// 使用 poller 模式（更好的文件变更检测）
	args = append(args, "--poll-interval", "15s")

	// daemon 模式（后台运行）
	args = append(args, "--daemon")

	return args
}

// waitForMount 等待挂载进程.
func (m *Manager) waitForMount(cmd *exec.Cmd, instance *MountInstance) {
	err := cmd.Wait()
	if err != nil {
		m.mu.Lock()
		instance.Status = MountStatusError
		instance.Error = err.Error()
		instance.PID = 0
		m.mu.Unlock()

		m.sendEvent(MountEvent{
			MountID: id,
			Type:    EventError,
			Status:  MountStatusError,
			Error:   err.Error(),
		})
	}
}

// sendEvent 发送事件.
func (m *Manager) sendEvent(event MountEvent) {
	event.ID = fmt.Sprintf("event-%d", time.Now().UnixNano())
	event.Timestamp = time.Now()

	select {
	case m.eventChan <- event:
	default:
		// 事件通道满，丢弃旧事件
	}
}

// GetEvents 获取事件通道.
func (m *Manager) GetEvents() <-chan MountEvent {
	return m.eventChan
}

// Shutdown 关闭管理器.
func (m *Manager) Shutdown() {
	m.mu.Lock()
	m.running = false
	m.mu.Unlock()

	// 卸载所有挂载
	_ = m.UnmountAll()

	m.cancel()
	close(m.eventChan)
}