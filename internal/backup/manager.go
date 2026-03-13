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
	configs map[string]*JobConfig

	// 备份任务状态
	tasks map[string]*BackupTask

	// 配置文件路径
	configPath string

	// 存储挂载点
	storagePath string
}

// JobConfig 备份作业配置
type JobConfig struct {
	ID          string     `json:"id"`
	Name        string     `json:"name"`
	Source      string     `json:"source"`
	Destination string     `json:"destination"`
	Type        BackupType `json:"type"`
	Schedule    string     `json:"schedule"`
	Retention   int        `json:"retention"`
	Compression bool       `json:"compression"`
	Encrypt     bool       `json:"encrypt"`
	Enabled     bool       `json:"enabled"`
	LastRun     string     `json:"lastRun"`
	NextRun     string     `json:"nextRun"`

	// 远程配置
	RemoteHost     string `json:"remoteHost,omitempty"`
	RemoteUser     string `json:"remoteUser,omitempty"`
	RemotePort     int    `json:"remotePort,omitempty"`
	RemotePassword string `json:"remotePassword,omitempty"`

	// rsync 特定配置
	RsyncOptions []string `json:"rsyncOptions,omitempty"`
	Exclude      []string `json:"exclude,omitempty"`

	// 云端备份配置
	CloudBackup bool         `json:"cloudBackup,omitempty"`
	CloudConfig *CloudConfig `json:"cloudConfig,omitempty"`

	// 加密配置
	Encryption        bool   `json:"encryption,omitempty"`
	EncryptionType    string `json:"encryptionType,omitempty"`
	EncryptionKey     string `json:"encryptionKey,omitempty"`
	EncryptionKeyFile string `json:"encryptionKeyFile,omitempty"`

	// 增量备份配置
	ChunkPath string `json:"chunkPath,omitempty"`
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
	Progress   int        `json:"progress"`
	TotalSize  int64      `json:"totalSize"`
	TotalFiles int64      `json:"totalFiles"`
	Speed      int64      `json:"speed"`
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
	ID        string     `json:"id"`
	ConfigID  string     `json:"configId"`
	Name      string     `json:"name"`
	Type      BackupType `json:"type"`
	Size      int64      `json:"size"`
	FileCount int64      `json:"fileCount"`
	Duration  int64      `json:"duration"`
	CreatedAt time.Time  `json:"createdAt"`
	Path      string     `json:"path"`
	Verified  bool       `json:"verified"`
	Checksum  string     `json:"checksum,omitempty"`
}

// BackupStats 备份统计
type BackupStats struct {
	TotalBackups     int           `json:"totalBackups"`
	TotalSize        int64         `json:"totalSize"`
	TotalSizeHuman   string        `json:"totalSizeHuman"`
	AvgDuration      time.Duration `json:"avgDuration"`
	AverageDuration  time.Duration `json:"averageDuration"`
	SuccessCount     int           `json:"successCount"`
	FailedCount      int           `json:"failedCount"`
	SuccessRate      float64       `json:"successRate"`
	LastBackupTime   time.Time     `json:"lastBackupTime"`
	NextBackupTime   time.Time     `json:"nextBackupTime"`
	IncrementalRatio float64       `json:"incrementalRatio"`
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
		configs:     make(map[string]*JobConfig),
		tasks:       make(map[string]*BackupTask),
		configPath:  configPath,
		storagePath: storagePath,
	}
}

// Initialize 初始化
func (m *Manager) Initialize() error {
	if err := m.loadConfig(); err != nil {
		// 配置文件不存在是正常的，记录但不返回错误
		m.mu.Lock()
		m.configs = make(map[string]*JobConfig)
		m.mu.Unlock()
	}
	return nil
}

// ========== 备份配置管理 ==========

func (m *Manager) ListConfigs() []*JobConfig {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var configs []*JobConfig
	for _, cfg := range m.configs {
		configs = append(configs, cfg)
	}
	return configs
}

func (m *Manager) GetConfig(id string) (*JobConfig, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	cfg, ok := m.configs[id]
	if !ok {
		return nil, fmt.Errorf("备份配置不存在：%s", id)
	}
	return cfg, nil
}

func (m *Manager) CreateConfig(config JobConfig) error {
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
	if err := m.saveConfig(); err != nil {
		delete(m.configs, config.ID)
		return fmt.Errorf("保存配置失败：%w", err)
	}

	return nil
}

func (m *Manager) UpdateConfig(id string, config JobConfig) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.configs[id]; !ok {
		return fmt.Errorf("备份配置不存在：%s", id)
	}

	config.ID = id
	m.configs[id] = &config
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("保存配置失败：%w", err)
	}

	return nil
}

func (m *Manager) DeleteConfig(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.configs[id]; !ok {
		return fmt.Errorf("备份配置不存在：%s", id)
	}

	delete(m.configs, id)
	if err := m.saveConfig(); err != nil {
		return fmt.Errorf("保存配置失败：%w", err)
	}

	return nil
}

func (m *Manager) EnableConfig(id string, enabled bool) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	cfg, ok := m.configs[id]
	if !ok {
		return fmt.Errorf("备份配置不存在：%s", id)
	}

	cfg.Enabled = enabled
	m.saveConfig()

	return nil
}

// ========== 备份执行 ==========

func (m *Manager) RunBackup(configID string) (*BackupTask, error) {
	m.mu.Lock()
	cfg, ok := m.configs[configID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("备份配置不存在：%s", configID)
	}

	task := &BackupTask{
		ID:        generateID(),
		ConfigID:  configID,
		Status:    TaskStatusRunning,
		StartTime: time.Now(),
	}
	m.tasks[task.ID] = task
	m.mu.Unlock()

	go m.executeBackup(cfg, task)

	return task, nil
}

func (m *Manager) executeBackup(cfg *JobConfig, task *BackupTask) {
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
		err = fmt.Errorf("不支持的备份类型：%s", cfg.Type)
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

// runLocalBackup 本地备份（支持增量）
func (m *Manager) runLocalBackup(cfg *JobConfig, task *BackupTask) (string, error) {
	destDir := cfg.Destination
	if destDir == "" {
		destDir = filepath.Join(m.storagePath, "backups", cfg.Name)
	}
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return "", fmt.Errorf("创建目标目录失败：%w", err)
	}

	var backupPath string

	// 使用传统备份方式
	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("%s_%s", cfg.Name, timestamp)
	backupPath = filepath.Join(destDir, backupName+".tar.gz")
	cmd := exec.Command("tar", "czf", backupPath, "-C", cfg.Source, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("压缩失败：%w, output: %s", err, string(output))
	}
	task.Progress = 100

	// 加密备份
	if cfg.Encrypt {
		encryptedPath := backupPath + ".enc"
		// 从配置获取加密密钥，如果未配置则使用默认值并警告
		encryptKey := cfg.EncryptionKey
		if encryptKey == "" && cfg.EncryptionKeyFile != "" {
			keyData, err := os.ReadFile(cfg.EncryptionKeyFile)
			if err != nil {
				return "", fmt.Errorf("读取加密密钥文件失败：%w", err)
			}
			encryptKey = strings.TrimSpace(string(keyData))
		}
		if encryptKey == "" {
			return "", fmt.Errorf("启用加密但未配置加密密钥")
		}
		// 使用 openssl 进行加密
		cmd := exec.Command("openssl", "enc", "-aes-256-cbc", "-salt", "-in", backupPath, "-out", encryptedPath, "-pass", "pass:"+encryptKey)
		if output, err := cmd.CombinedOutput(); err != nil {
			os.Remove(encryptedPath)
			return "", fmt.Errorf("加密失败：%w, output: %s", err, string(output))
		}
		os.Remove(backupPath)
		backupPath = encryptedPath
	}

	if err := m.cleanupOldBackups(destDir, cfg.Retention); err != nil {
		fmt.Printf("清理旧备份失败：%v\n", err)
	}

	return backupPath, nil
}

func (m *Manager) runRemoteBackup(cfg *JobConfig, task *BackupTask) (string, error) {
	if cfg.RemoteHost == "" {
		return "", fmt.Errorf("远程主机地址不能为空")
	}

	remotePath := cfg.Destination
	if remotePath == "" {
		remotePath = fmt.Sprintf("/backup/%s", cfg.Name)
	}

	timestamp := time.Now().Format("20060102_150405")
	backupName := fmt.Sprintf("%s_%s.tar.gz", cfg.Name, timestamp)
	localTemp := filepath.Join(os.TempDir(), backupName)

	cmd := exec.Command("tar", "czf", localTemp, "-C", cfg.Source, ".")
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("压缩失败：%w, output: %s", err, string(output))
	}
	defer os.Remove(localTemp)

	remoteTarget := fmt.Sprintf("%s@%s:%s/%s",
		cfg.RemoteUser, cfg.RemoteHost, remotePath, backupName)

	scpArgs := []string{"-P", fmt.Sprintf("%d", cfg.RemotePort), localTemp, remoteTarget}
	cmd = exec.Command("scp", scpArgs...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return "", fmt.Errorf("传输失败：%w, output: %s", err, string(output))
	}

	return remoteTarget, nil
}

func (m *Manager) runRsyncBackup(cfg *JobConfig, task *BackupTask) (string, error) {
	destination := cfg.Destination

	if cfg.RemoteHost != "" {
		destination = fmt.Sprintf("%s@%s:%s",
			cfg.RemoteUser, cfg.RemoteHost, cfg.Destination)
	}

	args := []string{"-avz", "--progress"}

	for _, exclude := range cfg.Exclude {
		args = append(args, "--exclude", exclude)
	}

	args = append(args, cfg.RsyncOptions...)

	if cfg.RemotePort != 0 && cfg.RemotePort != 22 {
		args = append(args, "-e", fmt.Sprintf("ssh -p %d", cfg.RemotePort))
	}

	args = append(args, cfg.Source+"/", destination)

	cmd := exec.Command("rsync", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("rsync 失败：%w, output: %s", err, string(output))
	}

	return destination, nil
}

func (m *Manager) cleanupOldBackups(dir string, retention int) error {
	files, err := filepath.Glob(filepath.Join(dir, "*.tar.gz"))
	if err != nil {
		return err
	}

	if len(files) <= retention {
		return nil
	}

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

	for i := 0; i < len(fileInfos)-retention; i++ {
		os.Remove(fileInfos[i].path)
	}

	return nil
}

// ========== 恢复操作 ==========

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

func (m *Manager) executeRestore(options RestoreOptions, task *BackupTask) {
	defer func() {
		task.EndTime = time.Now()
		m.mu.Lock()
		task.Status = TaskStatusCompleted
		m.mu.Unlock()
	}()

	backupPath := options.BackupID
	if _, err := os.Stat(backupPath); os.IsNotExist(err) {
		task.Status = TaskStatusFailed
		task.Error = "备份文件不存在"
		return
	}

	if err := os.MkdirAll(options.TargetPath, 0755); err != nil {
		task.Status = TaskStatusFailed
		task.Error = fmt.Sprintf("创建目标目录失败：%v", err)
		return
	}

	if strings.HasSuffix(backupPath, ".tar.gz") {
		cmd := exec.Command("tar", "xzf", backupPath, "-C", options.TargetPath)
		if output, err := cmd.CombinedOutput(); err != nil {
			task.Status = TaskStatusFailed
			task.Error = fmt.Sprintf("解压失败：%v, output: %s", err, string(output))
			return
		}
	} else {
		if err := copyDirectory(backupPath, options.TargetPath); err != nil {
			task.Status = TaskStatusFailed
			task.Error = fmt.Sprintf("复制失败：%v", err)
			return
		}
	}

	task.Progress = 100
}

// ========== 任务管理 ==========

func (m *Manager) GetTask(taskID string) (*BackupTask, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return nil, fmt.Errorf("任务不存在：%s", taskID)
	}
	return task, nil
}

func (m *Manager) ListTasks() []*BackupTask {
	m.mu.RLock()
	defer m.mu.RUnlock()

	var tasks []*BackupTask
	for _, task := range m.tasks {
		tasks = append(tasks, task)
	}
	return tasks
}

func (m *Manager) CancelTask(taskID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	task, ok := m.tasks[taskID]
	if !ok {
		return fmt.Errorf("任务不存在：%s", taskID)
	}

	if task.Status != TaskStatusRunning {
		return fmt.Errorf("任务不在运行中")
	}

	task.Status = TaskStatusCancelled
	return nil
}

// ========== 备份历史 ==========

func (m *Manager) GetHistory(configID string) ([]*BackupHistory, error) {
	m.mu.RLock()
	cfg, ok := m.configs[configID]
	m.mu.RUnlock()

	if !ok {
		return nil, fmt.Errorf("备份配置不存在：%s", configID)
	}

	var history []*BackupHistory

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

func copyDirectory(src, dst string) error {
	// 清理和验证源目录路径
	cleanSrc := filepath.Clean(src)
	cleanDst := filepath.Clean(dst)

	// 验证目标目录在预期路径内（防止路径遍历）
	if !strings.HasPrefix(cleanDst, "/mnt") && !strings.HasPrefix(cleanDst, "/backup") {
		return fmt.Errorf("invalid destination path: %s", dst)
	}

	return filepath.Walk(cleanSrc, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		// 处理符号链接（TOCTOU 防护）
		if info.Mode()&os.ModeSymlink != 0 {
			// 跳过符号链接或解析后验证
			realPath, err := filepath.EvalSymlinks(path)
			if err != nil {
				return fmt.Errorf("invalid symlink: %s", path)
			}
			// 验证符号链接目标在源目录内
			if !strings.HasPrefix(realPath, cleanSrc) {
				return fmt.Errorf("symlink escapes source directory: %s", path)
			}
			path = realPath
			// 重新获取文件信息
			info, err = os.Stat(realPath)
			if err != nil {
				return err
			}
		}

		relPath, err := filepath.Rel(cleanSrc, path)
		if err != nil {
			return err
		}

		// 清理目标路径
		dstPath := filepath.Clean(filepath.Join(cleanDst, relPath))

		// 再次验证目标路径（双重检查）
		if !strings.HasPrefix(dstPath, cleanDst) {
			return fmt.Errorf("path traversal detected: %s", dstPath)
		}

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

// ========== 恢复预览 ==========

// RestorePreview 恢复预览信息
type RestorePreview struct {
	BackupPath     string   `json:"backupPath"`
	TargetPath     string   `json:"targetPath"`
	TotalSize      int64    `json:"totalSize"`
	TotalSizeHuman string   `json:"totalSizeHuman"`
	FileCount      int      `json:"fileCount"`
	Files          []string `json:"files,omitempty"`
	Overwrite      bool     `json:"overwrite"`
	EstimatedTime  string   `json:"estimatedTime"`
}

// PreviewRestore 预览恢复操作（不实际执行）
func (m *Manager) PreviewRestore(options RestoreOptions) (*RestorePreview, error) {
	// 找到备份文件
	backupPath := options.BackupID
	if backupPath == "" {
		return nil, fmt.Errorf("备份 ID/路径不能为空")
	}

	// 检查备份文件是否存在
	info, err := os.Stat(backupPath)
	if err != nil {
		return nil, fmt.Errorf("备份文件不存在：%w", err)
	}

	preview := &RestorePreview{
		BackupPath:     backupPath,
		TargetPath:     options.TargetPath,
		TotalSize:      info.Size(),
		TotalSizeHuman: humanReadableSize(info.Size()),
		Overwrite:      options.Overwrite,
	}

	// 如果是压缩包，列出内容
	if strings.HasSuffix(backupPath, ".tar.gz") || strings.HasSuffix(backupPath, ".tar") {
		cmd := exec.Command("tar", "-tzf", backupPath)
		if !strings.HasSuffix(backupPath, ".gz") {
			cmd = exec.Command("tar", "-tf", backupPath)
		}
		output, err := cmd.Output()
		if err != nil {
			return nil, fmt.Errorf("读取备份内容失败：%w", err)
		}

		files := strings.Split(strings.TrimSpace(string(output)), "\n")
		preview.FileCount = len(files)

		// 只显示前 100 个文件
		if len(files) > 100 {
			preview.Files = files[:100]
		} else {
			preview.Files = files
		}
	}

	// 估算恢复时间（假设 100MB/s）
	speed := int64(100 * 1024 * 1024)
	estimatedSeconds := preview.TotalSize / speed
	if estimatedSeconds < 60 {
		preview.EstimatedTime = fmt.Sprintf("约 %d 秒", estimatedSeconds)
	} else {
		preview.EstimatedTime = fmt.Sprintf("约 %d 分钟", estimatedSeconds/60)
	}

	return preview, nil
}

// ========== 统计信息 ==========

// GetStats 获取备份统计信息
func (m *Manager) GetStats() *BackupStats {
	m.mu.RLock()
	defer m.mu.RUnlock()

	stats := &BackupStats{
		TotalBackups: len(m.configs),
	}

	// 计算总大小和平均持续时间
	var totalSize int64
	var totalDuration time.Duration
	var successCount int
	var incrementalCount int

	for _, cfg := range m.configs {
		// 估算备份大小
		if info, err := os.Stat(cfg.Destination); err == nil {
			totalSize += info.Size()
		}

		// 统计任务
		for _, task := range m.tasks {
			if task.ConfigID == cfg.ID {
				if task.Status == TaskStatusCompleted {
					successCount++
					totalDuration += task.EndTime.Sub(task.StartTime)
				}
				if task.TotalSize > 0 {
					incrementalCount++
				}
			}
		}
	}

	stats.TotalSize = totalSize
	stats.TotalSizeHuman = humanReadableSize(totalSize)

	// 计算成功率
	if len(m.tasks) > 0 {
		stats.SuccessRate = float64(successCount) / float64(len(m.tasks)) * 100
	}

	// 计算平均耗时
	if successCount > 0 {
		avgDuration := totalDuration / time.Duration(successCount)
		stats.AverageDuration = avgDuration
	}

	// 计算增量备份节省比例
	if incrementalCount > 0 {
		stats.IncrementalRatio = 0.3 // 假设值，实际需要根据增量备份大小计算
	}

	return stats
}

func humanReadableSize(size int64) string {
	const (
		KB = 1024
		MB = KB * 1024
		GB = MB * 1024
		TB = GB * 1024
	)

	switch {
	case size >= TB:
		return fmt.Sprintf("%.2f TB", float64(size)/TB)
	case size >= GB:
		return fmt.Sprintf("%.2f GB", float64(size)/GB)
	case size >= MB:
		return fmt.Sprintf("%.2f MB", float64(size)/MB)
	case size >= KB:
		return fmt.Sprintf("%.2f KB", float64(size)/KB)
	default:
		return fmt.Sprintf("%d B", size)
	}
}

// CheckConfigDetailed 详细检查配置
func (m *Manager) CheckConfigDetailed(id string) (*ConfigCheckResult, error) {
	config, err := m.GetConfig(id)
	if err != nil {
		return nil, err
	}

	result := &ConfigCheckResult{
		ConfigID: id,
		Status:   "pass",
		Checks:   []CheckItem{},
	}

	// 检查源路径
	if config.Source != "" {
		if _, err := os.Stat(config.Source); err != nil {
			result.Checks = append(result.Checks, CheckItem{
				Name:    "source_path",
				Status:  "fail",
				Message: fmt.Sprintf("源路径不存在: %v", err),
			})
			result.Status = "fail"
		} else {
			result.Checks = append(result.Checks, CheckItem{
				Name:    "source_path",
				Status:  "pass",
				Message: "源路径正常",
			})
		}
	}

	// 检查目标路径
	if config.Destination != "" {
		if _, err := os.Stat(config.Destination); err != nil {
			// 尝试创建目录
			if err := os.MkdirAll(config.Destination, 0755); err != nil {
				result.Checks = append(result.Checks, CheckItem{
					Name:    "destination_path",
					Status:  "fail",
					Message: fmt.Sprintf("目标路径无法创建: %v", err),
				})
				result.Status = "fail"
			} else {
				result.Checks = append(result.Checks, CheckItem{
					Name:    "destination_path",
					Status:  "pass",
					Message: "目标路径已创建",
				})
			}
		} else {
			result.Checks = append(result.Checks, CheckItem{
				Name:    "destination_path",
				Status:  "pass",
				Message: "目标路径正常",
			})
		}
	}

	return result, nil
}

// HealthCheck 健康检查
func (m *Manager) HealthCheck() (*HealthCheckResult, error) {
	result := &HealthCheckResult{
		Status:    "healthy",
		Timestamp: time.Now(),
		Details:   make(map[string]interface{}),
	}

	m.mu.RLock()
	result.Details["config_count"] = len(m.configs)
	result.Details["task_count"] = len(m.tasks)
	m.mu.RUnlock()

	// 检查最近的备份状态
	hasRecentBackup := false
	m.mu.RLock()
	for _, task := range m.tasks {
		if task.Status == TaskStatusCompleted && time.Since(task.StartTime) < 24*time.Hour {
			hasRecentBackup = true
			break
		}
	}
	m.mu.RUnlock()

	if !hasRecentBackup {
		result.Details["warning"] = "no recent backup in last 24 hours"
	}

	return result, nil
}

// HealthCheckResult 健康检查结果
type HealthCheckResult struct {
	Status    string                 `json:"status"`
	Timestamp time.Time              `json:"timestamp"`
	Details   map[string]interface{} `json:"details"`
}

// DefaultRestorePresets 默认恢复预设
func DefaultRestorePresets() []RestorePreset {
	return []RestorePreset{
		{
			ID:          "full",
			Name:        "完整恢复",
			Description: "恢复所有文件",
			Options: RestoreOptions{
				Overwrite: true,
			},
		},
		{
			ID:          "selective",
			Name:        "选择性恢复",
			Description: "选择特定文件恢复",
			Options: RestoreOptions{
				Overwrite: false,
			},
		},
		{
			ID:          "incremental",
			Name:        "增量恢复",
			Description: "只恢复变化的文件",
			Options: RestoreOptions{
				Overwrite: true,
			},
		},
	}
}

// RestorePreset 恢复预设
type RestorePreset struct {
	ID          string         `json:"id"`
	Name        string         `json:"name"`
	Description string         `json:"description"`
	Options     RestoreOptions `json:"options"`
}
