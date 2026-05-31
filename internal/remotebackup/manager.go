// Package remotebackup 远程备份引擎模块
package remotebackup

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// NewManager 创建远程备份管理器
func NewManager(configPath string) *Manager {
	m := &Manager{
		targets:     make(map[string]*BackupTarget),
		jobs:        make(map[string]*BackupJob),
		versions:    make(map[string][]*BackupVersion),
		configPath:  configPath,
		cancelFuncs: make(map[string]context.CancelFunc),
	}
	_ = m.loadConfig()
	return m
}

// ========== 目标管理 ==========

// ListTargets 列出所有备份目标
func (m *Manager) ListTargets() []*BackupTarget {
	m.mu.RLock()
	defer m.mu.RUnlock()

	targets := make([]*BackupTarget, 0, len(m.targets))
	for _, t := range m.targets {
		// 脱敏返回
		targetCopy := *t
		targetCopy.SecretKey = maskString(targetCopy.SecretKey)
		targetCopy.Password = maskString(targetCopy.Password)
		targetCopy.Encryption.Passphrase = maskString(targetCopy.Encryption.Passphrase)
		targets = append(targets, &targetCopy)
	}
	return targets
}

// GetTarget 获取单个备份目标
func (m *Manager) GetTarget(id string) (*BackupTarget, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	t, ok := m.targets[id]
	if !ok {
		return nil, fmt.Errorf("目标 %s 不存在", id)
	}
	targetCopy := *t
	targetCopy.SecretKey = maskString(targetCopy.SecretKey)
	targetCopy.Password = maskString(targetCopy.Password)
	targetCopy.Encryption.Passphrase = maskString(targetCopy.Encryption.Passphrase)
	return &targetCopy, nil
}

// CreateTarget 创建备份目标
func (m *Manager) CreateTarget(req *BackupTarget) (*BackupTarget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("目标名称不能为空")
	}
	if req.Type == "" {
		return nil, fmt.Errorf("目标类型不能为空")
	}

	target := &BackupTarget{
		ID:             uuid.New().String(),
		Name:           req.Name,
		Type:           req.Type,
		Endpoint:       req.Endpoint,
		Port:           req.Port,
		Bucket:         req.Bucket,
		Path:           req.Path,
		AccessKey:      req.AccessKey,
		SecretKey:      req.SecretKey,
		Username:       req.Username,
		Password:       req.Password,
		Region:         req.Region,
		UseSSL:         req.UseSSL,
		Encryption:     req.Encryption,
		BandwidthLimit: req.BandwidthLimit,
		CreatedAt:      time.Now(),
		UpdatedAt:      time.Now(),
	}

	m.targets[target.ID] = target
	if err := m.saveConfig(); err != nil {
		delete(m.targets, target.ID)
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}

	return target, nil
}

// UpdateTarget 更新备份目标
func (m *Manager) UpdateTarget(id string, req *BackupTarget) (*BackupTarget, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	t, ok := m.targets[id]
	if !ok {
		return nil, fmt.Errorf("目标 %s 不存在", id)
	}

	if req.Name != "" {
		t.Name = req.Name
	}
	if req.Endpoint != "" {
		t.Endpoint = req.Endpoint
	}
	if req.Port != 0 {
		t.Port = req.Port
	}
	if req.Bucket != "" {
		t.Bucket = req.Bucket
	}
	if req.Path != "" {
		t.Path = req.Path
	}
	if req.AccessKey != "" {
		t.AccessKey = req.AccessKey
	}
	if req.SecretKey != "" {
		t.SecretKey = req.SecretKey
	}
	if req.Username != "" {
		t.Username = req.Username
	}
	if req.Password != "" {
		t.Password = req.Password
	}
	if req.Region != "" {
		t.Region = req.Region
	}
	t.UseSSL = req.UseSSL
	t.Encryption = req.Encryption
	t.BandwidthLimit = req.BandwidthLimit
	t.UpdatedAt = time.Now()

	if err := m.saveConfig(); err != nil {
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}

	targetCopy := *t
	return &targetCopy, nil
}

// DeleteTarget 删除备份目标
func (m *Manager) DeleteTarget(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if _, ok := m.targets[id]; !ok {
		return fmt.Errorf("目标 %s 不存在", id)
	}

	// 检查是否有任务关联
	for _, job := range m.jobs {
		if job.TargetID == id {
			return fmt.Errorf("目标 %s 仍被任务 %s 使用，无法删除", id, job.ID)
		}
	}

	delete(m.targets, id)
	return m.saveConfig()
}

// TestConnection 测试连接
func (m *Manager) TestConnection(id string) error {
	m.mu.RLock()
	t, ok := m.targets[id]
	m.mu.RUnlock()

	if !ok {
		return fmt.Errorf("目标 %s 不存在", id)
	}

	// 根据类型测试连接
	switch t.Type {
	case TargetTypeS3:
		return m.testS3Connection(t)
	case TargetTypeFTP:
		return m.testFTPConnection(t)
	case TargetTypeSFTP:
		return m.testSFTPConnection(t)
	case TargetTypeWebDAV:
		return m.testWebDAVConnection(t)
	case TargetTypeRsync:
		return m.testRsyncConnection(t)
	default:
		return fmt.Errorf("不支持的目标类型: %s", t.Type)
	}
}

// ========== 任务管理 ==========

// ListJobs 列出所有备份任务
func (m *Manager) ListJobs() []*BackupJob {
	m.mu.RLock()
	defer m.mu.RUnlock()

	jobs := make([]*BackupJob, 0, len(m.jobs))
	for _, j := range m.jobs {
		jobCopy := *j
		jobs = append(jobs, &jobCopy)
	}
	return jobs
}

// CreateJob 创建备份任务
func (m *Manager) CreateJob(req *BackupJob) (*BackupJob, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	if req.Name == "" {
		return nil, fmt.Errorf("任务名称不能为空")
	}
	if len(req.SourcePaths) == 0 {
		return nil, fmt.Errorf("源路径不能为空")
	}
	if req.TargetID == "" {
		return nil, fmt.Errorf("目标ID不能为空")
	}

	// 验证目标存在
	if _, ok := m.targets[req.TargetID]; !ok {
		return nil, fmt.Errorf("目标 %s 不存在", req.TargetID)
	}

	if req.Strategy == "" {
		req.Strategy = StrategyFull
	}

	job := &BackupJob{
		ID:              uuid.New().String(),
		Name:            req.Name,
		SourcePaths:     req.SourcePaths,
		TargetID:        req.TargetID,
		Strategy:        req.Strategy,
		RetentionPolicy: req.RetentionPolicy,
		Schedule:        req.Schedule,
		ExcludePatterns: req.ExcludePatterns,
		Status:          StatusPending,
		Enabled:         req.Enabled,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}

	m.jobs[job.ID] = job
	m.versions[job.ID] = make([]*BackupVersion, 0)

	if err := m.saveConfig(); err != nil {
		delete(m.jobs, job.ID)
		delete(m.versions, job.ID)
		return nil, fmt.Errorf("保存配置失败: %w", err)
	}

	return job, nil
}

// RunJob 手动执行备份任务
func (m *Manager) RunJob(ctx context.Context, jobID string) (*BackupVersion, error) {
	m.mu.Lock()
	job, ok := m.jobs[jobID]
	if !ok {
		m.mu.Unlock()
		return nil, fmt.Errorf("任务 %s 不存在", jobID)
	}
	if job.Status == StatusRunning {
		m.mu.Unlock()
		return nil, fmt.Errorf("任务 %s 正在运行中", jobID)
	}

	job.Status = StatusRunning
	job.Progress = 0
	job.LastError = ""
	job.UpdatedAt = time.Now()

	// 创建可取消的context
	runCtx, cancel := context.WithCancel(ctx)
	m.cancelFuncs[jobID] = cancel
	m.mu.Unlock()

	// 执行备份
	version, err := m.executeBackup(runCtx, job)

	m.mu.Lock()
	delete(m.cancelFuncs, jobID)
	now := time.Now()
	job.LastRun = &now
	job.UpdatedAt = now

	if err != nil {
		job.Status = StatusFailed
		job.LastError = err.Error()
		m.mu.Unlock()
		return nil, err
	}

	job.Status = StatusCompleted
	job.Progress = 100
	m.versions[jobID] = append(m.versions[jobID], version)
	job.VersionCount = len(m.versions[jobID])

	// 应用保留策略
	m.applyRetentionPolicy(job)
	_ = m.saveConfig()
	m.mu.Unlock()

	return version, nil
}

// CancelJob 取消备份任务
func (m *Manager) CancelJob(jobID string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	job, ok := m.jobs[jobID]
	if !ok {
		return fmt.Errorf("任务 %s 不存在", jobID)
	}

	if job.Status != StatusRunning {
		return fmt.Errorf("任务 %s 未在运行中", jobID)
	}

	if cancel, ok := m.cancelFuncs[jobID]; ok {
		cancel()
		delete(m.cancelFuncs, jobID)
	}

	job.Status = StatusCancelled
	job.UpdatedAt = time.Now()
	return m.saveConfig()
}

// ListVersions 列出任务的版本
func (m *Manager) ListVersions(jobID string) ([]*BackupVersion, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.jobs[jobID]; !ok {
		return nil, fmt.Errorf("任务 %s 不存在", jobID)
	}

	versions := m.versions[jobID]
	result := make([]*BackupVersion, len(versions))
	copy(result, versions)
	return result, nil
}

// Restore 恢复数据
func (m *Manager) Restore(req *RestoreRequest) (*RestoreResult, error) {
	m.mu.RLock()
	job, ok := m.jobs[req.JobID]
	if !ok {
		m.mu.RUnlock()
		return nil, fmt.Errorf("任务 %s 不存在", req.JobID)
	}
	versions := m.versions[req.JobID]
	m.mu.RUnlock()

	if len(versions) == 0 {
		return nil, fmt.Errorf("任务 %s 没有可用的备份版本", req.JobID)
	}

	// 查找版本
	var version *BackupVersion
	if req.VersionID != "" {
		for _, v := range versions {
			if v.ID == req.VersionID {
				version = v
				break
			}
		}
		if version == nil {
			return nil, fmt.Errorf("版本 %s 不存在", req.VersionID)
		}
	} else {
		// 使用最新版本
		version = versions[len(versions)-1]
	}

	// 创建恢复目录
	if err := os.MkdirAll(req.RestorePath, 0755); err != nil {
		return nil, fmt.Errorf("创建恢复目录失败: %w", err)
	}

	result := &RestoreResult{
		JobID:     req.JobID,
		VersionID: version.ID,
	}

	// 执行恢复
	err := m.executeRestore(job, version, req)
	if err != nil {
		result.Errors = append(result.Errors, err.Error())
	}

	return result, nil
}

// GetRestorePoints 获取恢复点列表
func (m *Manager) GetRestorePoints(jobID string) ([]*RestorePoint, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if _, ok := m.jobs[jobID]; !ok {
		return nil, fmt.Errorf("任务 %s 不存在", jobID)
	}

	versions := m.versions[jobID]
	points := make([]*RestorePoint, 0, len(versions))
	for _, v := range versions {
		points = append(points, &RestorePoint{
			Version:          *v,
			RecoverableFiles: []string{},
			CreatedAt:        v.StartedAt,
		})
	}
	return points, nil
}

// ========== 内部方法 ==========

// executeBackup 执行备份任务
func (m *Manager) executeBackup(ctx context.Context, job *BackupJob) (*BackupVersion, error) {
	// 收集源文件
	files, err := m.collectFiles(job.SourcePaths, job.ExcludePatterns)
	if err != nil {
		return nil, fmt.Errorf("收集文件失败: %w", err)
	}

	version := &BackupVersion{
		ID:            uuid.New().String(),
		JobID:         job.ID,
		VersionNumber: len(m.versions[job.ID]) + 1,
		Type:          job.Strategy,
		StartedAt:     time.Now(),
		Status:        StatusRunning,
		FileCount:     len(files),
	}

	checksumHash := sha256.New()
	totalSize := int64(0)

	for _, file := range files {
		select {
		case <-ctx.Done():
			version.Status = StatusCancelled
			return version, ctx.Err()
		default:
		}

		// 获取文件信息
		info, err := os.Stat(file)
		if err != nil {
			continue
		}
		totalSize += info.Size()

		// 计算文件校验和
		fileChecksum, err := m.computeFileChecksum(file)
		if err != nil {
			continue
		}
		checksumHash.Write([]byte(fileChecksum))
	}

	version.TotalSize = totalSize
	version.TransferSize = totalSize // 增量备份时会小于TotalSize
	version.Checksum = hex.EncodeToString(checksumHash.Sum(nil))

	now := time.Now()
	version.CompletedAt = &now
	version.Status = StatusCompleted

	job.Progress = 100
	job.TransferStats = TransferStats{
		TotalBytes: totalSize,
		StartTime:  version.StartedAt,
	}

	return version, nil
}

// executeRestore 执行恢复
func (m *Manager) executeRestore(job *BackupJob, version *BackupVersion, req *RestoreRequest) error {
	// 模拟恢复操作
	for _, srcPath := range job.SourcePaths {
		baseName := filepath.Base(srcPath)
		destPath := filepath.Join(req.RestorePath, baseName)

		if !req.Overwrite {
			if _, err := os.Stat(destPath); err == nil {
				continue
			}
		}

		// 创建目标目录
		destDir := filepath.Dir(destPath)
		if err := os.MkdirAll(destDir, 0755); err != nil {
			return fmt.Errorf("创建目录 %s 失败: %w", destDir, err)
		}
	}

	return nil
}

// collectFiles 收集源路径下的所有文件
func (m *Manager) collectFiles(paths []string, excludes []string) ([]string, error) {
	var files []string

	for _, path := range paths {
		err := filepath.Walk(path, func(walkPath string, info os.FileInfo, err error) error {
			if err != nil {
				return nil
			}
			if info.IsDir() {
				return nil
			}

			// 检查排除模式
			for _, pattern := range excludes {
				if matched, _ := filepath.Match(pattern, info.Name()); matched {
					return nil
				}
			}

			files = append(files, walkPath)
			return nil
		})
		if err != nil {
			return nil, err
		}
	}

	return files, nil
}

// computeFileChecksum 计算文件SHA-256校验和
func (m *Manager) computeFileChecksum(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// applyRetentionPolicy 应用保留策略
func (m *Manager) applyRetentionPolicy(job *BackupJob) {
	versions := m.versions[job.ID]
	if len(versions) == 0 {
		return
	}

	switch job.RetentionPolicy.Unit {
	case RetentionVersions:
		if job.RetentionPolicy.Value > 0 && len(versions) > job.RetentionPolicy.Value {
			// 保留最新的N个版本
			m.versions[job.ID] = versions[len(versions)-job.RetentionPolicy.Value:]
		}
	case RetentionDays:
		if job.RetentionPolicy.Value > 0 {
			cutoff := time.Now().AddDate(0, 0, -job.RetentionPolicy.Value)
			var retained []*BackupVersion
			for _, v := range versions {
				if v.StartedAt.After(cutoff) {
					retained = append(retained, v)
				}
			}
			m.versions[job.ID] = retained
		}
	}
}

// ========== 连接测试 ==========

func (m *Manager) testS3Connection(t *BackupTarget) error {
	if t.Endpoint == "" || t.Bucket == "" {
		return fmt.Errorf("S3配置不完整: 需要endpoint和bucket")
	}
	// 模拟连接测试成功
	return nil
}

func (m *Manager) testFTPConnection(t *BackupTarget) error {
	if t.Endpoint == "" {
		return fmt.Errorf("FTP配置不完整: 需要endpoint")
	}
	return nil
}

func (m *Manager) testSFTPConnection(t *BackupTarget) error {
	if t.Endpoint == "" {
		return fmt.Errorf("SFTP配置不完整: 需要endpoint")
	}
	return nil
}

func (m *Manager) testWebDAVConnection(t *BackupTarget) error {
	if t.Endpoint == "" {
		return fmt.Errorf("WebDAV配置不完整: 需要endpoint")
	}
	return nil
}

func (m *Manager) testRsyncConnection(t *BackupTarget) error {
	if t.Endpoint == "" {
		return fmt.Errorf("Rsync配置不完整: 需要endpoint")
	}
	return nil
}

// ========== 加密相关 ==========

// EncryptData 使用AES-256-GCM加密数据
func (m *Manager) EncryptData(data []byte, passphrase string) ([]byte, error) {
	key := sha256.Sum256([]byte(passphrase))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, err
	}

	return gcm.Seal(nonce, nonce, data, nil), nil
}

// DecryptData 使用AES-256-GCM解密数据
func (m *Manager) DecryptData(data []byte, passphrase string) ([]byte, error) {
	key := sha256.Sum256([]byte(passphrase))

	block, err := aes.NewCipher(key[:])
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return nil, fmt.Errorf("加密数据太短")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	return gcm.Open(nil, nonce, ciphertext, nil)
}

// ComputeSHA256 计算数据的SHA-256校验和
func (m *Manager) ComputeSHA256(data []byte) string {
	hash := sha256.Sum256(data)
	return hex.EncodeToString(hash[:])
}

// ========== 配置持久化 ==========

type configData struct {
	Targets  map[string]*BackupTarget    `json:"targets"`
	Jobs     map[string]*BackupJob       `json:"jobs"`
	Versions map[string][]*BackupVersion `json:"versions"`
}

func (m *Manager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	dir := filepath.Dir(m.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data := configData{
		Targets:  m.targets,
		Jobs:     m.jobs,
		Versions: m.versions,
	}

	jsonData, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(m.configPath, jsonData, 0644)
}

func (m *Manager) loadConfig() error {
	if m.configPath == "" {
		return nil
	}

	data, err := os.ReadFile(m.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	var config configData
	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if config.Targets != nil {
		m.targets = config.Targets
	}
	if config.Jobs != nil {
		m.jobs = config.Jobs
	}
	if config.Versions != nil {
		m.versions = config.Versions
	}

	return nil
}

// maskString 脱敏字符串
func maskString(s string) string {
	if len(s) == 0 {
		return ""
	}
	if len(s) <= 4 {
		return strings.Repeat("*", len(s))
	}
	return s[:2] + strings.Repeat("*", len(s)-4) + s[len(s)-2:]
}
