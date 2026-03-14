// Package snapshot 提供快照复制功能
package snapshot

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"
)

// ========== 类型定义 ==========

// ReplicationConfig 复制配置
type ReplicationConfig struct {
	// ID 配置 ID
	ID string `json:"id"`

	// Name 配置名称
	Name string `json:"name"`

	// SourcePolicyID 源策略 ID
	SourcePolicyID string `json:"sourcePolicyId"`

	// TargetNodes 目标节点列表
	TargetNodes []ReplicationTarget `json:"targetNodes"`

	// Mode 复制模式
	Mode ReplicationMode `json:"mode"`

	// Schedule 复制调度
	Schedule *ReplicationSchedule `json:"schedule,omitempty"`

	// BandwidthLimit 带宽限制 (MB/s, 0 表示不限制)
	BandwidthLimit int `json:"bandwidthLimit,omitempty"`

	// Compress 是否压缩传输
	Compress bool `json:"compress"`

	// Encrypt 是否加密传输
	Encrypt bool `json:"encrypt"`

	// Retention 远程保留策略
	Retention *RemoteRetention `json:"retention,omitempty"`

	// Enabled 是否启用
	Enabled bool `json:"enabled"`

	// CreatedAt 创建时间
	CreatedAt time.Time `json:"createdAt"`

	// UpdatedAt 更新时间
	UpdatedAt time.Time `json:"updatedAt"`
}

// ReplicationMode 复制模式
type ReplicationMode string

const (
	// ReplicationModeFull 全量复制
	ReplicationModeFull ReplicationMode = "full"
	// ReplicationModeIncremental 增量复制
	ReplicationModeIncremental ReplicationMode = "incremental"
	// ReplicationModeDifferential 差异复制
	ReplicationModeDifferential ReplicationMode = "differential"
)

// ReplicationTarget 复制目标
type ReplicationTarget struct {
	// NodeID 节点 ID
	NodeID string `json:"nodeId"`

	// Address 节点地址
	Address string `json:"address"`

	// Port 端口
	Port int `json:"port"`

	// APIKey API 密钥
	APIKey string `json:"apiKey"`

	// TargetVolume 目标卷名
	TargetVolume string `json:"targetVolume"`

	// TargetPath 目标路径
	TargetPath string `json:"targetPath"`

	// Status 节点状态
	Status NodeStatus `json:"status"`

	// LastSync 最后同步时间
	LastSync *time.Time `json:"lastSync,omitempty"`
}

// NodeStatus 节点状态
type NodeStatus string

const (
	NodeStatusOnline  NodeStatus = "online"
	NodeStatusOffline NodeStatus = "offline"
	NodeStatusError   NodeStatus = "error"
)

// ReplicationSchedule 复制调度
type ReplicationSchedule struct {
	// Type 调度类型
	Type string `json:"type"` // immediate, interval, cron

	// IntervalMinutes 间隔分钟
	IntervalMinutes int `json:"intervalMinutes,omitempty"`

	// CronExpression cron 表达式
	CronExpression string `json:"cronExpression,omitempty"`
}

// RemoteRetention 远程保留策略
type RemoteRetention struct {
	// MaxSnapshots 最大快照数
	MaxSnapshots int `json:"maxSnapshots"`

	// MaxAgeDays 最大保留天数
	MaxAgeDays int `json:"maxAgeDays,omitempty"`
}

// ReplicationJob 复制任务
type ReplicationJob struct {
	// ID 任务 ID
	ID string `json:"id"`

	// ConfigID 配置 ID
	ConfigID string `json:"configId"`

	// SnapshotName 快照名称
	SnapshotName string `json:"snapshotName"`

	// SourceVolume 源卷
	SourceVolume string `json:"sourceVolume"`

	// TargetNode 目标节点
	TargetNode string `json:"targetNode"`

	// Status 任务状态
	Status ReplicationJobStatus `json:"status"`

	// Mode 复制模式
	Mode ReplicationMode `json:"mode"`

	// Progress 进度百分比
	Progress int `json:"progress"`

	// BytesTransferred 已传输字节数
	BytesTransferred int64 `json:"bytesTransferred"`

	// TotalBytes 总字节数
	TotalBytes int64 `json:"totalBytes"`

	// StartTime 开始时间
	StartTime *time.Time `json:"startTime,omitempty"`

	// EndTime 结束时间
	EndTime *time.Time `json:"endTime,omitempty"`

	// Speed 传输速度 (MB/s)
	Speed float64 `json:"speed"`

	// Error 错误信息
	Error string `json:"error,omitempty"`

	// RetryCount 重试次数
	RetryCount int `json:"retryCount"`

	// Checksum 数据校验和
	Checksum string `json:"checksum"`
}

// ReplicationJobStatus 任务状态
type ReplicationJobStatus string

const (
	ReplicationJobStatusPending   ReplicationJobStatus = "pending"
	ReplicationJobStatusRunning   ReplicationJobStatus = "running"
	ReplicationJobStatusCompleted ReplicationJobStatus = "completed"
	ReplicationJobStatusFailed    ReplicationJobStatus = "failed"
	ReplicationJobStatusCancelled ReplicationJobStatus = "cancelled"
)

// IncrementalManifest 增量复制清单
type IncrementalManifest struct {
	// BaseSnapshot 基准快照
	BaseSnapshot string `json:"baseSnapshot"`

	// Changes 变更块列表
	Changes []DataBlock `json:"changes"`

	// Checksum 整体校验和
	Checksum string `json:"checksum"`

	// Timestamp 时间戳
	Timestamp time.Time `json:"timestamp"`
}

// DataBlock 数据块
type DataBlock struct {
	// Offset 偏移量
	Offset int64 `json:"offset"`

	// Size 大小
	Size int64 `json:"size"`

	// Checksum 块校验和
	Checksum string `json:"checksum"`

	// Compressed 是否压缩
	Compressed bool `json:"compressed"`
}

// ReplicationStatus 复制状态
type ReplicationStatus struct {
	// ConfigID 配置 ID
	ConfigID string `json:"configId"`

	// TotalJobs 总任务数
	TotalJobs int `json:"totalJobs"`

	// CompletedJobs 完成任务数
	CompletedJobs int `json:"completedJobs"`

	// FailedJobs 失败任务数
	FailedJobs int `json:"failedJobs"`

	// LastSuccess 最后成功时间
	LastSuccess *time.Time `json:"lastSuccess,omitempty"`

	// LastFailure 最后失败时间
	LastFailure *time.Time `json:"lastFailure,omitempty"`

	// TotalBytesTransferred 总传输字节数
	TotalBytesTransferred int64 `json:"totalBytesTransferred"`

	// NodeStatuses 各节点状态
	NodeStatuses map[string]NodeReplicationStatus `json:"nodeStatuses"`
}

// NodeReplicationStatus 节点复制状态
type NodeReplicationStatus struct {
	NodeID     string     `json:"nodeId"`
	Status     NodeStatus `json:"status"`
	LastSync   *time.Time `json:"lastSync,omitempty"`
	LagSeconds int        `json:"lagSeconds"`
}

// ReplicationManager 复制管理器
type ReplicationManager struct {
	mu sync.RWMutex

	// configs 复制配置
	configs map[string]*ReplicationConfig

	// jobs 运行中的任务
	jobs map[string]*ReplicationJob

	// jobHistory 任务历史
	jobHistory []*ReplicationJob

	// policyManager 策略管理器
	policyManager *PolicyManager

	// storageMgr 存储管理器
	storageMgr StorageManager

	// configPath 配置文件路径
	configPath string

	// httpClient HTTP 客户端
	httpClient *http.Client

	// cancelFunc 取消函数
	cancelFunc context.CancelFunc

	// statusChan 状态更新通道
	statusChan chan ReplicationStatus

	// hooks 事件钩子
	hooks ReplicationHooks
}

// ReplicationHooks 复制事件钩子
type ReplicationHooks struct {
	// OnJobStart 任务开始回调
	OnJobStart func(job *ReplicationJob)

	// OnJobComplete 任务完成回调
	OnJobComplete func(job *ReplicationJob)

	// OnJobFailed 任务失败回调
	OnJobFailed func(job *ReplicationJob, err error)

	// OnNodeOnline 节点上线回调
	OnNodeOnline func(nodeID string)

	// OnNodeOffline 节点离线回调
	OnNodeOffline func(nodeID string)
}

// NewReplicationManager 创建复制管理器
func NewReplicationManager(policyMgr *PolicyManager, storageMgr StorageManager, configPath string) *ReplicationManager {
	return &ReplicationManager{
		configs:       make(map[string]*ReplicationConfig),
		jobs:          make(map[string]*ReplicationJob),
		jobHistory:    make([]*ReplicationJob, 0),
		policyManager: policyMgr,
		storageMgr:    storageMgr,
		configPath:    configPath,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
			Transport: &http.Transport{
				DialContext: (&net.Dialer{
					Timeout:   10 * time.Second,
					KeepAlive: 30 * time.Second,
				}).DialContext,
				MaxIdleConns:          10,
				IdleConnTimeout:       90 * time.Second,
				TLSHandshakeTimeout:   10 * time.Second,
				ResponseHeaderTimeout: 10 * time.Second,
			},
		},
		statusChan: make(chan ReplicationStatus, 100),
	}
}

// Initialize 初始化复制管理器
func (rm *ReplicationManager) Initialize() error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 加载配置
	if err := rm.loadConfig(); err != nil {
		// 配置文件不存在是正常的
		rm.configs = make(map[string]*ReplicationConfig)
	}

	// 启动状态监控
	ctx, cancel := context.WithCancel(context.Background())
	rm.cancelFunc = cancel
	go rm.monitorNodes(ctx)

	return nil
}

// Close 关闭复制管理器
func (rm *ReplicationManager) Close() {
	if rm.cancelFunc != nil {
		rm.cancelFunc()
	}
}

// ========== 配置管理 ==========

// CreateConfig 创建复制配置
func (rm *ReplicationManager) CreateConfig(config *ReplicationConfig) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	// 验证
	if err := rm.validateConfig(config); err != nil {
		return err
	}

	// 生成 ID
	if config.ID == "" {
		config.ID = generateID()
	}

	config.CreatedAt = time.Now()
	config.UpdatedAt = time.Now()

	rm.configs[config.ID] = config

	if err := rm.saveConfig(); err != nil {
		delete(rm.configs, config.ID)
		return fmt.Errorf("保存配置失败: %w", err)
	}

	return nil
}

// GetConfig 获取配置
func (rm *ReplicationManager) GetConfig(id string) (*ReplicationConfig, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	config, ok := rm.configs[id]
	if !ok {
		return nil, fmt.Errorf("配置不存在: %s", id)
	}

	return config, nil
}

// ListConfigs 列出所有配置
func (rm *ReplicationManager) ListConfigs() []*ReplicationConfig {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	result := make([]*ReplicationConfig, 0, len(rm.configs))
	for _, c := range rm.configs {
		result = append(result, c)
	}
	return result
}

// UpdateConfig 更新配置
func (rm *ReplicationManager) UpdateConfig(id string, updates *ReplicationConfig) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	existing, ok := rm.configs[id]
	if !ok {
		return fmt.Errorf("配置不存在: %s", id)
	}

	if err := rm.validateConfig(updates); err != nil {
		return err
	}

	updates.ID = id
	updates.CreatedAt = existing.CreatedAt
	updates.UpdatedAt = time.Now()

	rm.configs[id] = updates

	return rm.saveConfig()
}

// DeleteConfig 删除配置
func (rm *ReplicationManager) DeleteConfig(id string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	if _, ok := rm.configs[id]; !ok {
		return fmt.Errorf("配置不存在: %s", id)
	}

	delete(rm.configs, id)

	return rm.saveConfig()
}

// ========== 复制执行 ==========

// StartReplication 启动复制
func (rm *ReplicationManager) StartReplication(configID string) (string, error) {
	rm.mu.RLock()
	config, ok := rm.configs[configID]
	rm.mu.RUnlock()

	if !ok {
		return "", fmt.Errorf("配置不存在: %s", configID)
	}

	// 获取源策略
	policy, err := rm.policyManager.GetPolicy(config.SourcePolicyID)
	if err != nil {
		return "", fmt.Errorf("获取源策略失败: %w", err)
	}

	// 为每个目标节点创建任务
	var jobIDs []string
	for _, target := range config.TargetNodes {
		jobID, err := rm.createJob(config, policy, target)
		if err != nil {
			return "", fmt.Errorf("创建任务失败: %w", err)
		}
		jobIDs = append(jobIDs, jobID)
	}

	// 异步执行任务
	go rm.executeJobs(jobIDs)

	return jobIDs[0], nil
}

// createJob 创建复制任务
func (rm *ReplicationManager) createJob(config *ReplicationConfig, policy *Policy, target ReplicationTarget) (string, error) {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	job := &ReplicationJob{
		ID:           generateID(),
		ConfigID:     config.ID,
		SnapshotName: policy.SnapshotPrefix + "-" + time.Now().Format("20060102-150405"),
		SourceVolume: policy.VolumeName,
		TargetNode:   target.NodeID,
		Status:       ReplicationJobStatusPending,
		Mode:         config.Mode,
		Progress:     0,
	}

	rm.jobs[job.ID] = job

	return job.ID, nil
}

// executeJobs 执行任务
func (rm *ReplicationManager) executeJobs(jobIDs []string) {
	for _, jobID := range jobIDs {
		go rm.executeJob(jobID)
	}
}

// executeJob 执行单个任务
func (rm *ReplicationManager) executeJob(jobID string) {
	rm.mu.Lock()
	job, ok := rm.jobs[jobID]
	if !ok {
		return
	}
	job.Status = ReplicationJobStatusRunning
	now := time.Now()
	job.StartTime = &now
	rm.mu.Unlock()

	// 触发钩子
	if rm.hooks.OnJobStart != nil {
		rm.hooks.OnJobStart(job)
	}

	// 执行复制
	err := rm.doReplication(job)

	rm.mu.Lock()
	defer rm.mu.Unlock()

	if err != nil {
		job.Status = ReplicationJobStatusFailed
		job.Error = err.Error()
		job.RetryCount++

		if rm.hooks.OnJobFailed != nil {
			rm.hooks.OnJobFailed(job, err)
		}
	} else {
		job.Status = ReplicationJobStatusCompleted
		job.Progress = 100
		endTime := time.Now()
		job.EndTime = &endTime

		if rm.hooks.OnJobComplete != nil {
			rm.hooks.OnJobComplete(job)
		}
	}

	// 移动到历史记录
	rm.jobHistory = append(rm.jobHistory, job)
	delete(rm.jobs, jobID)
}

// doReplication 执行实际复制
func (rm *ReplicationManager) doReplication(job *ReplicationJob) error {
	rm.mu.RLock()
	config, ok := rm.configs[job.ConfigID]
	rm.mu.RUnlock()

	if !ok {
		return fmt.Errorf("配置不存在")
	}

	// 找到目标节点
	var target *ReplicationTarget
	for i, t := range config.TargetNodes {
		if t.NodeID == job.TargetNode {
			target = &config.TargetNodes[i]
			break
		}
	}

	if target == nil {
		return fmt.Errorf("目标节点不存在")
	}

	// 获取快照数据
	snapshotPath, err := rm.getSnapshotPath(job.SourceVolume, job.SnapshotName)
	if err != nil {
		return fmt.Errorf("获取快照路径失败: %w", err)
	}

	// 计算快照大小
	totalSize, err := rm.calculateSize(snapshotPath)
	if err != nil {
		return fmt.Errorf("计算大小失败: %w", err)
	}

	rm.mu.Lock()
	job.TotalBytes = totalSize
	rm.mu.Unlock()

	// 根据复制模式执行
	switch job.Mode {
	case ReplicationModeFull:
		return rm.doFullReplication(job, config, target, snapshotPath)
	case ReplicationModeIncremental:
		return rm.doIncrementalReplication(job, config, target, snapshotPath)
	case ReplicationModeDifferential:
		return rm.doDifferentialReplication(job, config, target, snapshotPath)
	default:
		return rm.doFullReplication(job, config, target, snapshotPath)
	}
}

// doFullReplication 全量复制
func (rm *ReplicationManager) doFullReplication(job *ReplicationJob, config *ReplicationConfig, target *ReplicationTarget, snapshotPath string) error {
	// 创建增量清单
	manifest, err := rm.createManifest(snapshotPath, "")
	if err != nil {
		return fmt.Errorf("创建清单失败: %w", err)
	}

	// 传输数据
	return rm.transferData(job, config, target, snapshotPath, manifest)
}

// doIncrementalReplication 增量复制
func (rm *ReplicationManager) doIncrementalReplication(job *ReplicationJob, config *ReplicationConfig, target *ReplicationTarget, snapshotPath string) error {
	// 获取上一次成功复制的快照
	baseSnapshot, err := rm.getLastReplicatedSnapshot(job.TargetNode, job.SourceVolume)
	if err != nil {
		// 没有基准快照，执行全量复制
		return rm.doFullReplication(job, config, target, snapshotPath)
	}

	// 创建增量清单
	manifest, err := rm.createManifest(snapshotPath, baseSnapshot)
	if err != nil {
		return fmt.Errorf("创建增量清单失败: %w", err)
	}

	// 如果变更量超过 50%，执行全量复制
	if len(manifest.Changes) > 0 {
		changeRatio := float64(len(manifest.Changes)) / float64(job.TotalBytes/1024/1024) // 简化计算
		if changeRatio > 0.5 {
			return rm.doFullReplication(job, config, target, snapshotPath)
		}
	}

	// 传输增量数据
	return rm.transferIncremental(job, config, target, snapshotPath, manifest)
}

// doDifferentialReplication 差异复制
func (rm *ReplicationManager) doDifferentialReplication(job *ReplicationJob, config *ReplicationConfig, target *ReplicationTarget, snapshotPath string) error {
	// 差异复制总是与初始基准快照比较
	return rm.doIncrementalReplication(job, config, target, snapshotPath)
}

// transferData 传输数据
func (rm *ReplicationManager) transferData(job *ReplicationJob, config *ReplicationConfig, target *ReplicationTarget, snapshotPath string, manifest *IncrementalManifest) error {
	// 使用 btrfs send/receive 或 rsync 进行传输
	// 这里简化实现，使用 HTTP API

	// 1. 准备传输请求
	_ = TransferRequest{
		SnapshotName: job.SnapshotName,
		Volume:       target.TargetVolume,
		Path:         target.TargetPath,
		Manifest:     manifest,
		Compress:     config.Compress,
		Encrypt:      config.Encrypt,
	}

	// 2. 发送请求到目标节点
	url := fmt.Sprintf("http://%s:%d/api/v1/replication/receive", target.Address, target.Port)

	// 3. 使用 btrfs send 命令发送快照
	ctx := context.Background()
	cmd := exec.CommandContext(ctx, "btrfs", "send", snapshotPath)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建管道失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动发送命令失败: %w", err)
	}

	// 4. 流式传输到目标节点
	// 这里简化实现，实际应该使用流式 HTTP 请求
	buf := make([]byte, 32*1024)
	var transferred int64

	for {
		n, err := stdout.Read(buf)
		if err != nil && err != io.EOF {
			break
		}

		transferred += int64(n)

		rm.mu.Lock()
		job.BytesTransferred = transferred
		if job.TotalBytes > 0 {
			job.Progress = int(float64(transferred) / float64(job.TotalBytes) * 100)
		}
		now := time.Now()
		if job.StartTime != nil {
			elapsed := now.Sub(*job.StartTime).Seconds()
			if elapsed > 0 {
				job.Speed = float64(transferred) / 1024 / 1024 / elapsed
			}
		}
		rm.mu.Unlock()

		if err == io.EOF {
			break
		}
	}

	// 5. 发送完成通知
	httpReq, _ := http.NewRequestWithContext(ctx, "POST", url+"/complete", nil)
	httpReq.Header.Set("X-Api-Key", target.APIKey)
	httpReq.Header.Set("X-Snapshot-Name", job.SnapshotName)
	httpReq.Header.Set("X-Checksum", manifest.Checksum)

	resp, err := rm.httpClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("发送完成通知失败: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("目标节点返回错误: %d", resp.StatusCode)
	}

	return nil
}

// transferIncremental 传输增量数据
func (rm *ReplicationManager) transferIncremental(job *ReplicationJob, config *ReplicationConfig, target *ReplicationTarget, snapshotPath string, manifest *IncrementalManifest) error {
	// 使用 btrfs send -p 进行增量传输
	ctx := context.Background()
	args := []string{"send", "-p", manifest.BaseSnapshot, snapshotPath}
	cmd := exec.CommandContext(ctx, "btrfs", args...)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("创建管道失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动增量发送命令失败: %w", err)
	}

	// 流式传输（简化实现）
	buf := make([]byte, 32*1024)
	var transferred int64

	for {
		n, err := stdout.Read(buf)
		if err != nil && err != io.EOF {
			break
		}

		transferred += int64(n)

		rm.mu.Lock()
		job.BytesTransferred = transferred
		rm.mu.Unlock()

		if err == io.EOF {
			break
		}
	}

	return cmd.Wait()
}

// ========== 增量复制优化 ==========

// createManifest 创建数据清单
func (rm *ReplicationManager) createManifest(snapshotPath, baseSnapshot string) (*IncrementalManifest, error) {
	manifest := &IncrementalManifest{
		BaseSnapshot: baseSnapshot,
		Changes:      make([]DataBlock, 0),
		Timestamp:    time.Now(),
	}

	// 如果有基准快照，使用 btrfs send --dry-run 获取变更
	if baseSnapshot != "" {
		cmd := exec.Command("btrfs", "send", "--dry-run", "-p", baseSnapshot, snapshotPath)
		output, err := cmd.CombinedOutput()
		if err != nil {
			return nil, fmt.Errorf("获取变更列表失败: %w: %s", err, string(output))
		}

		// 解析变更（简化实现）
		// 实际应该解析 btrfs send 的输出
	}

	// 计算校验和
	checksum, err := rm.calculateChecksum(snapshotPath)
	if err != nil {
		return nil, fmt.Errorf("计算校验和失败: %w", err)
	}
	manifest.Checksum = checksum

	return manifest, nil
}

// calculateChecksum 计算校验和
func (rm *ReplicationManager) calculateChecksum(path string) (string, error) {
	hash := sha256.New()

	err := filepath.Walk(path, func(filePath string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() {
			f, err := os.Open(filePath)
			if err != nil {
				return err
			}
			defer func() { _ = f.Close() }()

			if _, err := io.Copy(hash, f); err != nil {
				return err
			}
		}

		return nil
	})

	if err != nil {
		return "", err
	}

	return hex.EncodeToString(hash.Sum(nil)), nil
}

// ========== 状态监控 ==========

// monitorNodes 监控节点状态
func (rm *ReplicationManager) monitorNodes(ctx context.Context) {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			rm.checkNodes()
		}
	}
}

// checkNodes 检查节点状态
func (rm *ReplicationManager) checkNodes() {
	rm.mu.RLock()
	configs := make([]*ReplicationConfig, 0, len(rm.configs))
	for _, c := range rm.configs {
		configs = append(configs, c)
	}
	rm.mu.RUnlock()

	for _, config := range configs {
		for i, target := range config.TargetNodes {
			status := rm.checkNodeStatus(target)

			rm.mu.Lock()
			config.TargetNodes[i].Status = status
			rm.mu.Unlock()

			// 触发钩子
			if status == NodeStatusOnline && rm.hooks.OnNodeOnline != nil {
				rm.hooks.OnNodeOnline(target.NodeID)
			} else if status == NodeStatusOffline && rm.hooks.OnNodeOffline != nil {
				rm.hooks.OnNodeOffline(target.NodeID)
			}
		}
	}
}

// checkNodeStatus 检查单个节点状态
func (rm *ReplicationManager) checkNodeStatus(target ReplicationTarget) NodeStatus {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := fmt.Sprintf("http://%s:%d/api/v1/health", target.Address, target.Port)

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return NodeStatusError
	}

	resp, err := rm.httpClient.Do(req)
	if err != nil {
		return NodeStatusOffline
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		return NodeStatusOnline
	}

	return NodeStatusError
}

// GetStatus 获取复制状态
func (rm *ReplicationManager) GetStatus(configID string) (*ReplicationStatus, error) {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	config, ok := rm.configs[configID]
	if !ok {
		return nil, fmt.Errorf("配置不存在: %s", configID)
	}

	status := &ReplicationStatus{
		ConfigID:     configID,
		NodeStatuses: make(map[string]NodeReplicationStatus),
	}

	// 统计任务
	for _, job := range rm.jobHistory {
		if job.ConfigID == configID {
			status.TotalJobs++
			switch job.Status {
			case ReplicationJobStatusCompleted:
				status.CompletedJobs++
				status.TotalBytesTransferred += job.BytesTransferred
				if status.LastSuccess == nil || job.EndTime.After(*status.LastSuccess) {
					status.LastSuccess = job.EndTime
				}
			case ReplicationJobStatusFailed:
				status.FailedJobs++
				if status.LastFailure == nil || job.EndTime.After(*status.LastFailure) {
					status.LastFailure = job.EndTime
				}
			}
		}
	}

	// 节点状态
	for _, target := range config.TargetNodes {
		status.NodeStatuses[target.NodeID] = NodeReplicationStatus{
			NodeID:   target.NodeID,
			Status:   target.Status,
			LastSync: target.LastSync,
		}
	}

	return status, nil
}

// GetJobs 获取任务列表
func (rm *ReplicationManager) GetJobs(configID string, limit int) []*ReplicationJob {
	rm.mu.RLock()
	defer rm.mu.RUnlock()

	var result []*ReplicationJob

	// 运行中的任务
	for _, job := range rm.jobs {
		if configID == "" || job.ConfigID == configID {
			result = append(result, job)
		}
	}

	// 历史任务（最新的在前）
	for i := len(rm.jobHistory) - 1; i >= 0 && len(result) < limit; i-- {
		job := rm.jobHistory[i]
		if configID == "" || job.ConfigID == configID {
			result = append(result, job)
		}
	}

	return result
}

// CancelJob 取消任务
func (rm *ReplicationManager) CancelJob(jobID string) error {
	rm.mu.Lock()
	defer rm.mu.Unlock()

	job, ok := rm.jobs[jobID]
	if !ok {
		return fmt.Errorf("任务不存在: %s", jobID)
	}

	job.Status = ReplicationJobStatusCancelled
	endTime := time.Now()
	job.EndTime = &endTime

	rm.jobHistory = append(rm.jobHistory, job)
	delete(rm.jobs, jobID)

	return nil
}

// ========== 辅助方法 ==========

func (rm *ReplicationManager) validateConfig(config *ReplicationConfig) error {
	if config.Name == "" {
		return fmt.Errorf("配置名称不能为空")
	}

	if config.SourcePolicyID == "" {
		return fmt.Errorf("源策略 ID 不能为空")
	}

	if len(config.TargetNodes) == 0 {
		return fmt.Errorf("目标节点不能为空")
	}

	for _, target := range config.TargetNodes {
		if target.NodeID == "" {
			return fmt.Errorf("节点 ID 不能为空")
		}
		if target.Address == "" {
			return fmt.Errorf("节点地址不能为空")
		}
	}

	return nil
}

func (rm *ReplicationManager) loadConfig() error {
	data, err := os.ReadFile(rm.configPath)
	if err != nil {
		return err
	}

	return json.Unmarshal(data, &rm.configs)
}

func (rm *ReplicationManager) saveConfig() error {
	data, err := json.MarshalIndent(rm.configs, "", "  ")
	if err != nil {
		return err
	}

	dir := filepath.Dir(rm.configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	return os.WriteFile(rm.configPath, data, 0644)
}

func (rm *ReplicationManager) getSnapshotPath(volume, snapshot string) (string, error) {
	// 简化实现，返回快照路径
	return filepath.Join("/mnt", volume, ".snapshots", snapshot), nil
}

func (rm *ReplicationManager) calculateSize(path string) (int64, error) {
	var size int64
	err := filepath.Walk(path, func(_ string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if !info.IsDir() {
			size += info.Size()
		}
		return nil
	})
	return size, err
}

func (rm *ReplicationManager) getLastReplicatedSnapshot(nodeID, volume string) (string, error) {
	// 查询最后一次成功复制的快照
	// 简化实现
	return "", fmt.Errorf("没有基准快照")
}

// SetHooks 设置事件钩子
func (rm *ReplicationManager) SetHooks(hooks ReplicationHooks) {
	rm.mu.Lock()
	defer rm.mu.Unlock()
	rm.hooks = hooks
}

// ========== API 接收端 ==========

// TransferRequest 传输请求
type TransferRequest struct {
	SnapshotName string               `json:"snapshotName"`
	Volume       string               `json:"volume"`
	Path         string               `json:"path"`
	Manifest     *IncrementalManifest `json:"manifest"`
	Compress     bool                 `json:"compress"`
	Encrypt      bool                 `json:"encrypt"`
}

// ReplicationServer 复制服务端
type ReplicationServer struct {
	manager *ReplicationManager
}

// NewReplicationServer 创建复制服务端
func NewReplicationServer(manager *ReplicationManager) *ReplicationServer {
	return &ReplicationServer{manager: manager}
}

// ReceiveSnapshot 接收快照
func (rs *ReplicationServer) ReceiveSnapshot(req TransferRequest, data io.Reader) error {
	// 使用 btrfs receive 接收快照
	targetPath := filepath.Join("/mnt", req.Volume, req.Path)

	cmd := exec.Command("btrfs", "receive", targetPath)
	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("创建管道失败: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动接收命令失败: %w", err)
	}

	// 写入数据
	if _, err := io.Copy(stdin, data); err != nil {
		return fmt.Errorf("写入数据失败: %w", err)
	}
	stdin.Close()

	return cmd.Wait()
}

// generateID 生成唯一 ID
func generateID() string {
	h := sha256.New()
	h.Write([]byte(time.Now().String()))
	h.Write([]byte(fmt.Sprintf("%d", time.Now().UnixNano())))
	return hex.EncodeToString(h.Sum(nil))[:16]
}
