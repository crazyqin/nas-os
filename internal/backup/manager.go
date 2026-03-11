package backup

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// Manager 备份管理器
type Manager struct {
	mu sync.RWMutex

	// 备份配置
	configs map[string]*BackupConfig

	// 备份任务状态
	tasks map[string]*BackupTask

	// 配置文件路径
	configPath string

	// 存储挂载点
	storagePath string
}

// BackupConfig 备份配置
type BackupConfig struct {
	ID          string        `json:"id"`
	Name        string        `json:"name"`
	Source      string        `json:"source"`      // 源路径
	Destination string        `json:"destination"` // 目标路径
	Type        BackupType    `json:"type"`        // local, remote, rsync
	Schedule    string        `json:"schedule"`    // cron 表达式
	Retention   int           `json:"retention"`   // 保留数量
	Compression bool          `json:"compression"` // 是否压缩
	Encrypt     bool          `json:"encrypt"`     // 是否加密
	Enabled     bool          `json:"enabled"`
	LastRun     string        `json:"lastRun"`
	NextRun     string        `json:"nextRun"`

	// 远程配置
	RemoteHost     string `json:"remoteHost,omitempty"`
	RemoteUser     string `json:"remoteUser,omitempty"`
	RemotePort     int    `json:"remotePort,omitempty"`
	RemotePassword string `json:"remotePassword,omitempty"` // 可选，SSH 密钥认证

	// rsync 特定配置
	RsyncOptions []string `json:"rsyncOptions,omitempty"`
	Exclude      []string `json:"exclude,omitempty"`
}

// BackupType 备份类型
type BackupType string

const (
	BackupTypeLocal  BackupType = "local"
	BackupTypeRemote BackupType = "remote"
	BackupTypeRsync  BackupType = "rsync"
)

// BackupTask 备份任务状态
type BackupTask struct {
	ID         string     `json:"id"`
	ConfigID   string     `json:"configId"`
	Status     TaskStatus `json:"status"`
	StartTime  time.Time  `json:"startTime"`
	EndTime    time.Time  `json:"endTime,omitempty"`
	Progress   int        `json:"progress"`   // 0-100
	TotalSize  int64      `json:"totalSize"`  // 字节
	TotalFiles int64      `json:"totalFiles"` // 文件数
	Speed      int64      `json:"speed"`      // 字节/秒
	Error      string     `json:"error,omitempty"`
}

// TaskStatus 任务状态
type TaskStatus string

const (
	TaskStatusPending   TaskStatus = "pending"
	TaskStatusRunning   TaskStatus = "running"
	TaskStatusCompleted TaskStatus = "completed"
	TaskStatusFailed    TaskStatus = "failed"
	TaskStatusCancelled TaskStatus = "cancelled"
)

// BackupHistory 备份历史记录
type BackupHistory struct {
	ID           string    `json:"id"`
	ConfigID     string    `json:"configId"`
	Name         string    `json:"name"`
	Type         BackupType `json:"type"`
	Size         int64     `json:"size"`
	FileCount    int64     `json:"fileCount"`
	Duration     int64     `json:"duration"` // 秒
	CreatedAt    time.Time `json:"createdAt"`
	Path         string    `json:"path"`
	Verified     bool      `json:"verified"`
	Checksum     string    `json:"checksum,omitempty"`
}

// RestoreOptions 恢复选项
type RestoreOptions struct {
	BackupID   string `json:"backupId"`
	TargetPath string `json:"targetPath"`
	Overwrite  bool   `json:"overwrite"`
	Decrypt    bool   `json:"decrypt"`
	Password   string `json:"password,omitempty"`
}

// NewManager 创建备份管理器
func NewManager(configPath, storagePath string) *Manager {
	return &Manager{
		configs:     make(map[string]*BackupConfig),
		tasks:       make(map[string]*BackupTask),
		configPath:  configPath,
		storagePath: storagePath,
	}
}

// Initialize 初始化
func (m *Manager) Initialize() error {
	if err := m.loadConfig(); err != nil {
		// 配置文件不存在是正常的
	}
	return nil
}

// ========== 备份配置管理 ==========

// ListConfigs 列出所有备份配置
func (m *Manager) ListConfigs() []*BackupConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var configs []*BackupConfig
	for _, cfg := range m.configs {
		configs = append(configs, cfg)
	}
	return configs
}

// GetConfig 获取单个配置
func (m *Manager) GetConfig(id string) (*BackupConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg, ok := m.configs[id]
	if !ok {
		return nil, fmt.Errorf("备份配置不存在: %s", id)
	}
	return cfg, nil
}

// CreateConfig 创建备份配置
func (m *Manager) CreateConfig(config BackupConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if config.ID == "" {
		config.ID = generateID()
	}

	if config.Name == "" {
		return fmt.Errorf("备份名称不能为空")
	}

	if config.Source == "" {
		return fmt.Errorf("源路径不能为空")
	}

	// 设置默认值
	if config.Type == "" {
		config.Type = BackupTypeLocal
	}
	if config.Retention == 0 {
		config.Retention = 7
	}
	if config.RemotePort == 0 {
		config.RemotePort = 22
	}

	m.configs[config.ID] = &config
	m.saveConfig()

	return nil
}

// UpdateConfig 更新备份配置
func (m *Manager) UpdateConfig(id string, config BackupConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.configs[id]; !ok {
		return fmt.Errorf("备份配置不存在: %s", id)
	}

	config.ID = id
	m.configs[id] = &config
	m.saveConfig()

	return nil
}

// DeleteConfig 删除备份配置
func (m *Manager) DeleteConfig(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.configs[id]; !ok {
		return fmt.Errorf("备份配置不存在: %s", id)
	}

	delete(m.configs, id)
	m.saveConfig()

	return nil
}

// EnableConfig 启用/禁用备份
func (m *Manager) EnableConfig(id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg, ok := m.configs[id]
	if !ok {
		return fmt.Errorf("备份配置不存在: %s", id)
	}

	cfg.Enabled = enabled
	m.saveConfig()

	return nil
}

// ========== 备份执行 ==========

// RunBackup 执行备份
func (m *Manager) RunBackup(configID string) (*BackupTask, error) {
	m.mu.Lock()
	cfg, ok := m.configs[configID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("备份配置不存在: %s", configID)
	}

	// 创建任务
	task := &BackupTask{
		ID:        generateID(),
		ConfigID:  configID,
		Status:    TaskStatusRunning,
		StartTime: time.Now(),
	}
	m.tasks[task.ID] = task
	m.mu.Unlock()

	// 异步执行备份
	go m.executeBackup(cfg, task)

	return task, nil
}

// executeBackup 执行备份逻辑
func (m *Manager) executeBackup(cfg *BackupConfig, task *BackupTask) {
	defer func() {
		task.EndTime = time.Now()
		cfg.LastRun = task.StartTime.Format("2006-01-02 15:04:05")
		m.mu.Lock()
		m.saveConfig()
		m.mu.Unlock()
	}()

	var err error

	switch cfg.Type {
	case BackupTypeLocal:
		_, err = m.runLocalBackup(cfg, task)
	case BackupTypeRemote:
		_, err = m.runRemoteBackup(cfg, task)
	case BackupTypeRsync:
		_, err = m.runRsyncBackup(cfg, task)
	default:
		err = fmt.Errorf("不支持的备份类型: %s", cfg.Type)
	}

	m.mu.Lock()
	if err != nil {
		task.Status = TaskStatusFailed
		task.Error = err.Error()
	} else {
		task.Status = TaskStatusCompleted
		task.Progress = 100
	}
	m.mu.Unlock()
}

// runLocalBackup 本地备份
func (m *Manager) runLocalBackup(cfg *BackupConfig, task *BackupTask) (string, error) {
	// 创建目标目录
	destDir := cfg.Destination
	if destDir == "" {
		destDir = filepath.Join(m.storagePath, "backups", cfg.Name)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("创建目标目录失败: %w", err)
	}

	// 生成备份文件名
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("%s_%s", cfg.Name, timestamp)
	var backupPath string

	if cfg.Compression {
		backupPath = filepath.Join(destDir, backupName+".tar.gz")

		// 使用 tar 压缩
		cmd := exec.Command("tar", "czf", backupPath, "-C", cfg.Source, ".")
		if output, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("压缩失败: %w, output: %s", err, string(output))
		}
	} else {
		backupPath = filepath.Join(destDir, backupName)
		if err := copyDirectory(cfg.Source, backupPath); err != nil {
			return "", fmt.Errorf("复制失败: %w", err)
		}
	}

	// 清理旧备份
	if err := m.cleanupOldBackups(destDir, cfg.Retention); err != nil {
		// 记录警告但不失败
		fmt.Printf("清理旧备份失败: %v\n", err)
	}

	return backupPath, nil
}

// runRemoteBackup 远程备份 (通过 SSH)
func (m *Manager) runRemoteBackup(cfg *BackupConfig, task *BackupTask) (string, error) {
	if cfg.RemoteHost == "" {
		return "", fmt.Errorf("远程主机地址不能为空")
	}

	// 构建远程路径
	remotePath := cfg.Destination
	if remotePath == "" {
		remotePath = fmt.Sprintf("/backup/%s", cfg.Name)
	}

	// 生成备份文件名
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("%s_%s.tar.gz", cfg.Name, timestamp)

	// 本地临时文件
	localTemp := filepath.Join(os.TempDir(), backupName)

	// 先本地压缩
	cmd := exec.Command("tar", "czf", localTemp, "-C", cfg.Source, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("压缩失败: %w, output: %s", err, string(output))
	}
	defer os.Remove(localTemp)

	// 使用 scp 传输
	remoteTarget := fmt.Sprintf("%s@%s:%s/%s",
		cfg.RemoteUser, cfg.RemoteHost, remotePath, backupName)

	scpArgs := []string{"-P", fmt.Sprintf("%d", cfg.RemotePort), localTemp, remoteTarget}
	cmd = exec.Command("scp", scpArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("传输失败: %w, output: %s", err, string(output))
	}

	return remoteTarget, nil
}

// runRsyncBackup rsync 同步备份
func (m *Manager) runRsyncBackup(cfg *BackupConfig, task *BackupTask) (string, error) {
	destination := cfg.Destination

	// 如果是远程 rsync
	if cfg.RemoteHost != "" {
		destination = fmt.Sprintf("%s@%s:%s",
			cfg.RemoteUser, cfg.RemoteHost, cfg.Destination)
	}

	// 构建 rsync 命令
	args := []string{"-avz", "--progress"}

	// 添加排除规则
	for _, exclude := range cfg.Exclude {
		args = append(args, "--exclude", exclude)
	}

	// 添加自定义选项
	args = append(args, cfg.RsyncOptions...)

	// SSH 端口
	if cfg.RemotePort != 0 && cfg.RemotePort != 22 {
		args = append(args, "-e", fmt.Sprintf("ssh -p %d", cfg.RemotePort))
	}

	args = append(args, cfg.Source+"/", destination)

	cmd := exec.Command("rsync", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("rsync 失败: %w, output: %s", err, string(output))
	}

	return destination, nil
}

// cleanupOldBackups 清理旧备份
func (m *Manager) cleanupOldBackups(dir string, retention int) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.tar.gz"))
	if err != nil {
		return err
	}

	if len(files) <= retention {
		return nil
	}

	// 按修改时间排序
	type fileInfo struct {
		path    string
		modTime time.Time
	}
	var fileInfos []fileInfo
	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}
		fileInfos = append(fileInfos, fileInfo{path: f, modTime: info.ModTime()})
	}

	// 删除最旧的文件
	for i := 0; i < len(fileInfos)-retention; i++ {
		os.Remove(fileInfos[i].path)
	}

	return nil
}

// ========== 恢复操作 ==========

// Restore 恢复备份
func (m *Manager) Restore(options RestoreOptions) (*BackupTask, error) {
	task := &BackupTask{
		ID:        generateID(),
		ConfigID:  options.BackupID,
		Status:    TaskStatusRunning,
		StartTime: time.Now(),
	}

	m.mu.Lock()
	m.tasks[task.ID] = task
	m.mu.Unlock()

	go m.executeRestore(options, task)

	return task, nil
}

// executeRestore 执行恢复
func (m *Manager) executeRestore(options RestoreOptions, task *BackupTask) {
	defer func() {
		task.EndTime = time.Now()
		m.mu.Lock()
		task.Status = TaskStatusCompleted
		m.mu.Unlock()
	}()

	// 查找备份文件
	backupPath := options.BackupID
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		// 可能是备份历史 ID，需要查找
		task.Status = TaskStatusFailed
		task.Error = "备份文件不存在"
		return
	}

	// 创建目标目录
	if err := os.MkdirAll(options.TargetPath, 0755); err != nil {
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("创建目标目录失败: %v", err)
		return
	}

	// 解压恢复
	if strings.HasSuffix(backupPath, ".tar.gz") {
		cmd := exec.Command("tar", "xzf", backupPath, "-C", options.TargetPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			task.Status = TaskStatusFailed
			task.Error = fmt.Sprintf("解压失败: %v, output: %s", err, string(output))
			return
		}
	} else {
		// 直接复制
		if err := copyDirectory(backupPath, options.TargetPath); err != nil {
			task.Status = TaskStatusFailed
			task.Error = fmt.Sprintf("复制失败: %v", err)
			return
		}
	}

	task.Progress = 100
}

// ========== 任务管理 ==========

// GetTask 获取任务状态
func (m *Manager) GetTask(taskID string) (*BackupTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("任务不存在: %s", taskID)
	}
	return task, nil
}

// ListTasks 列出所有任务
func (m *Manager) ListTasks() []*BackupTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tasks []*BackupTask
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

// CancelTask 取消任务
func (m *Manager) CancelTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("任务不存在: %s", taskID)
	}

	if task.Status != TaskStatusRunning {
		return fmt.Errorf("任务不在运行中")
	}

	task.Status = TaskStatusCancelled
	return nil
}

// ========== 备份历史 ==========

// GetHistory 获取备份历史
func (m *Manager) GetHistory(configID string) ([]*BackupHistory, error) {
	m.mu.RLock()
	cfg, ok := m.configs[configID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("备份配置不存在: %s", configID)
	}

	var history []*BackupHistory

	// 查找备份文件
	destDir := cfg.Destination
	if destDir == "" {
		destDir = filepath.Join(m.storagePath, "backups", cfg.Name)
	}

	files, err := filepath.Glob(filepath.Join(destDir, "*.tar.gz"))
	if err != nil {
		return history, nil
	}

	for _, f := range files {
		info, err := os.Stat(f)
		if err != nil {
			continue
		}

		history = append(history, &BackupHistory{
			ID:        filepath.Base(f),
			ConfigID:  configID,
			Name:      filepath.Base(f),
			Type:      cfg.Type,
			Size:      info.Size(),
			CreatedAt: info.ModTime(),
			Path:      f,
		})
	}

	return history, nil
}

// ========== 辅助函数 ==========

func (m *Manager) loadConfig() error {
	data, err := os.ReadFile(m.configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &m.configs)
}

func (m *Manager) saveConfig() error {
	data, err := json.MarshalIndent(m.configs, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0644)
}

func generateID() string {
	return fmt.Sprintf("backup_%d", time.Now().UnixNano())
}

func copyDirectory(src, dst string) error {
	return filepath.Walk(src, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		dstPath := filepath.Join(dst, relPath)

		if info.IsDir() {
			return os.MkdirAll(dstPath, info.Mode())
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(dstPath, data, info.Mode())
	})
}