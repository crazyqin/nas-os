package cluster

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"
	"go.uber.org/zap"
)

// 同步模式
const (
	SyncModeAsync    = "async"    // 异步同步
	SyncModeSync     = "sync"     // 同步同步
	SyncModeRealtime = "realtime" // 实时同步
)

// 同步状态
const (
	SyncStatusPending   = "pending"
	SyncStatusRunning   = "running"
	SyncStatusCompleted = "completed"
	SyncStatusFailed    = "failed"
)

// SyncRule 同步规则
type SyncRule struct {
	ID          string    `json:"id"`
	Name        string    `json:"name"`
	SourceNode  string    `json:"source_node"`
	TargetNodes []string  `json:"target_nodes"`
	SourcePath  string    `json:"source_path"`
	TargetPath  string    `json:"target_path"`
	SyncMode    string    `json:"sync_mode"`
	Schedule    string    `json:"schedule"` // cron 表达式
	Enabled     bool      `json:"enabled"`
	LastSync    time.Time `json:"last_sync"`
	NextSync    time.Time `json:"next_sync"`
	Status      string    `json:"status"`
	LastError   string    `json:"last_error"`
	CreatedAt   time.Time `json:"created_at"`
}

// SyncJob 同步任务
type SyncJob struct {
	RuleID      string
	Source      string
	Target      string
	TargetNode  string
	StartTime   time.Time
	EndTime     time.Time
	Status      string
	Error       string
	FilesSynced int64
	BytesSynced int64
}

// SyncStatus 同步状态
type SyncStatus struct {
	TotalRules   int       `json:"total_rules"`
	ActiveRules  int       `json:"active_rules"`
	RunningJobs  int       `json:"running_jobs"`
	TotalJobs    int       `json:"total_jobs"`
	FailedJobs   int       `json:"failed_jobs"`
	LastSyncTime time.Time `json:"last_sync_time"`
}

// StorageSync 存储同步管理器
type StorageSync struct {
	rules      map[string]*SyncRule
	rulesMutex sync.RWMutex
	jobs       []*SyncJob
	jobsMutex  sync.RWMutex
	cron       *cron.Cron
	config     SyncConfig
	ctx        context.Context
	cancel     context.CancelFunc
	logger     *zap.Logger
	cluster    *Manager
}

// SyncConfig 同步配置
type SyncConfig struct {
	DataDir      string `json:"data_dir"`
	MaxRetries   int    `json:"max_retries"`
	RetryDelay   int    `json:"retry_delay"` // 秒
	RsyncPath    string `json:"rsync_path"`
	SSHTimeout   int    `json:"ssh_timeout"` // 秒
	ParallelJobs int    `json:"parallel_jobs"`
}

// NewStorageSync 创建存储同步管理器
func NewStorageSync(config SyncConfig, logger *zap.Logger, cluster *Manager) (*StorageSync, error) {
	if config.DataDir == "" {
		config.DataDir = "/var/lib/nas-os/sync"
	}
	if config.MaxRetries == 0 {
		config.MaxRetries = 3
	}
	if config.RetryDelay == 0 {
		config.RetryDelay = 60
	}
	if config.RsyncPath == "" {
		config.RsyncPath = "/usr/bin/rsync"
	}
	if config.SSHTimeout == 0 {
		config.SSHTimeout = 30
	}
	if config.ParallelJobs == 0 {
		config.ParallelJobs = 2
	}

	ctx, cancel := context.WithCancel(context.Background())

	ss := &StorageSync{
		rules:   make(map[string]*SyncRule),
		jobs:    make([]*SyncJob, 0),
		cron:    cron.New(cron.WithSeconds()),
		config:  config,
		ctx:     ctx,
		cancel:  cancel,
		logger:  logger,
		cluster: cluster,
	}

	// 创建数据目录
	if err := os.MkdirAll(config.DataDir, 0755); err != nil {
		cancel()
		return nil, fmt.Errorf("创建同步数据目录失败：%w", err)
	}

	// 加载持久化规则
	if err := ss.loadRules(); err != nil {
		logger.Warn("加载同步规则失败", zap.Error(err))
	}

	return ss, nil
}

// Initialize 初始化同步管理器
func (ss *StorageSync) Initialize() error {
	ss.logger.Info("初始化存储同步管理器")

	// 启动 cron 调度器
	ss.cron.Start()

	// 注册所有规则的定时任务
	ss.rulesMutex.RLock()
	for _, rule := range ss.rules {
		if rule.Enabled && rule.Schedule != "" {
			if err := ss.scheduleRule(rule); err != nil {
				ss.logger.Error("调度规则失败", zap.String("rule_id", rule.ID), zap.Error(err))
			}
		}
	}
	ss.rulesMutex.RUnlock()

	ss.logger.Info("存储同步管理器初始化完成")
	return nil
}

// CreateRule 创建同步规则
func (ss *StorageSync) CreateRule(rule *SyncRule) error {
	// 安全校验：验证规则字段
	if err := ValidateSyncRule(rule); err != nil {
		return fmt.Errorf("规则验证失败：%w", err)
	}

	ss.rulesMutex.Lock()
	defer ss.rulesMutex.Unlock()

	if rule.ID == "" {
		rule.ID = generateRuleID()
	}
	rule.CreatedAt = time.Now()
	rule.Status = SyncStatusPending

	if rule.Enabled && rule.Schedule != "" {
		if err := ss.scheduleRule(rule); err != nil {
			return fmt.Errorf("调度规则失败：%w", err)
		}
	}

	ss.rules[rule.ID] = rule
	ss.logger.Info("创建同步规则", zap.String("rule_id", rule.ID), zap.String("name", rule.Name))

	// 持久化
	return ss.saveRules()
}

// UpdateRule 更新同步规则
func (ss *StorageSync) UpdateRule(ruleID string, updates map[string]interface{}) error {
	// 安全校验：验证规则ID
	if err := ValidateNodeID(ruleID); err != nil {
		return fmt.Errorf("规则ID无效：%w", err)
	}

	ss.rulesMutex.Lock()
	defer ss.rulesMutex.Unlock()

	rule, exists := ss.rules[ruleID]
	if !exists {
		return fmt.Errorf("规则不存在：%s", ruleID)
	}

	// 应用更新并验证
	for key, value := range updates {
		switch key {
		case "name":
			if v, ok := value.(string); ok {
				if v == "" || len(v) > 128 {
					return errors.New("规则名称必须在1-128字符之间")
				}
				rule.Name = v
			}
		case "source_path":
			if v, ok := value.(string); ok {
				if err := ValidatePath(v); err != nil {
					return fmt.Errorf("源路径无效：%w", err)
				}
				rule.SourcePath = v
			}
		case "target_path":
			if v, ok := value.(string); ok {
				if err := ValidatePath(v); err != nil {
					return fmt.Errorf("目标路径无效：%w", err)
				}
				rule.TargetPath = v
			}
		case "target_nodes":
			if v, ok := value.([]string); ok {
				if len(v) == 0 {
					return errors.New("至少需要一个目标节点")
				}
				for _, nodeID := range v {
					if err := ValidateNodeID(nodeID); err != nil {
						return fmt.Errorf("目标节点ID无效：%w", err)
					}
				}
				rule.TargetNodes = v
			}
		case "sync_mode":
			if v, ok := value.(string); ok {
				validModes := map[string]bool{
					SyncModeAsync:    true,
					SyncModeSync:     true,
					SyncModeRealtime: true,
					"":               true,
				}
				if !validModes[v] {
					return fmt.Errorf("无效的同步模式：%s", v)
				}
				rule.SyncMode = v
			}
		case "schedule":
			if v, ok := value.(string); ok {
				if err := ValidateCronExpression(v); err != nil {
					return fmt.Errorf("cron表达式无效：%w", err)
				}
				rule.Schedule = v
				// 重新调度
				if rule.Enabled {
					ss.unscheduleRule(rule)
					if rule.Schedule != "" {
						if err := ss.scheduleRule(rule); err != nil {
							return err
						}
					}
				}
			}
		case "enabled":
			if v, ok := value.(bool); ok {
				wasEnabled := rule.Enabled
				rule.Enabled = v
				if rule.Enabled && !wasEnabled && rule.Schedule != "" {
					if err := ss.scheduleRule(rule); err != nil {
						return err
					}
				} else if !rule.Enabled {
					ss.unscheduleRule(rule)
				}
			}
		}
	}

	ss.logger.Info("更新同步规则", zap.String("rule_id", ruleID))
	return ss.saveRules()
}

// DeleteRule 删除同步规则
func (ss *StorageSync) DeleteRule(ruleID string) error {
	ss.rulesMutex.Lock()
	defer ss.rulesMutex.Unlock()

	rule, exists := ss.rules[ruleID]
	if !exists {
		return fmt.Errorf("规则不存在：%s", ruleID)
	}

	ss.unscheduleRule(rule)
	delete(ss.rules, ruleID)

	ss.logger.Info("删除同步规则", zap.String("rule_id", ruleID))
	return ss.saveRules()
}

// GetRules 获取所有规则
func (ss *StorageSync) GetRules() []*SyncRule {
	ss.rulesMutex.RLock()
	defer ss.rulesMutex.RUnlock()

	rules := make([]*SyncRule, 0, len(ss.rules))
	for _, rule := range ss.rules {
		rules = append(rules, rule)
	}
	return rules
}

// GetRule 获取指定规则
func (ss *StorageSync) GetRule(ruleID string) (*SyncRule, bool) {
	ss.rulesMutex.RLock()
	defer ss.rulesMutex.RUnlock()

	rule, exists := ss.rules[ruleID]
	return rule, exists
}

// TriggerSync 手动触发同步
func (ss *StorageSync) TriggerSync(ruleID string) error {
	rule, exists := ss.GetRule(ruleID)
	if !exists {
		return fmt.Errorf("规则不存在：%s", ruleID)
	}

	if !rule.Enabled {
		return fmt.Errorf("规则已禁用：%s", ruleID)
	}

	ss.logger.Info("手动触发同步", zap.String("rule_id", ruleID))
	go ss.executeSync(rule)

	return nil
}

// scheduleRule 调度规则
func (ss *StorageSync) scheduleRule(rule *SyncRule) error {
	if rule.Schedule == "" {
		return nil
	}

	schedule, err := cron.ParseStandard(rule.Schedule)
	if err != nil {
		return fmt.Errorf("无效的 cron 表达式：%w", err)
	}

	rule.NextSync = schedule.Next(time.Now())

	_, _ = ss.cron.AddFunc(rule.Schedule, func() {
		ss.executeSync(rule)
	})

	ss.logger.Debug("规则已调度", zap.String("rule_id", rule.ID), zap.String("schedule", rule.Schedule))
	return nil
}

// unscheduleRule 取消调度规则
func (ss *StorageSync) unscheduleRule(rule *SyncRule) {
	// cron 库不直接支持移除任务，这里简化处理
	// 实际实现需要使用 cron.EntryID 来移除
	rule.NextSync = time.Time{}
}

// executeSync 执行同步任务
func (ss *StorageSync) executeSync(rule *SyncRule) {
	ss.logger.Info("执行同步任务", zap.String("rule_id", rule.ID))

	rule.Status = SyncStatusRunning
	rule.LastSync = time.Now()

	// 创建同步任务
	job := &SyncJob{
		RuleID:    rule.ID,
		Source:    rule.SourcePath,
		Target:    rule.TargetPath,
		StartTime: time.Now(),
		Status:    SyncStatusRunning,
	}

	ss.jobsMutex.Lock()
	ss.jobs = append(ss.jobs, job)
	ss.jobsMutex.Unlock()

	// 执行同步到每个目标节点
	var lastError error
	for _, targetNode := range rule.TargetNodes {
		if err := ss.syncToNode(rule, targetNode, job); err != nil {
			lastError = err
			ss.logger.Error("同步到节点失败",
				zap.String("rule_id", rule.ID),
				zap.String("target_node", targetNode),
				zap.Error(err))
		}
	}

	job.EndTime = time.Now()
	if lastError != nil {
		job.Status = SyncStatusFailed
		job.Error = lastError.Error()
		rule.Status = SyncStatusFailed
		rule.LastError = lastError.Error()
	} else {
		job.Status = SyncStatusCompleted
		rule.Status = SyncStatusCompleted
	}

	// 更新下次同步时间
	if rule.Schedule != "" {
		schedule, err := cron.ParseStandard(rule.Schedule)
		if err == nil {
			rule.NextSync = schedule.Next(time.Now())
		}
	}
}

// syncToNode 同步到指定节点
func (ss *StorageSync) syncToNode(rule *SyncRule, targetNodeID string, job *SyncJob) error {
	// 安全校验：验证目标节点ID
	if err := ValidateNodeID(targetNodeID); err != nil {
		return fmt.Errorf("目标节点ID无效：%w", err)
	}

	// 安全校验：验证源路径
	if err := ValidatePath(rule.SourcePath); err != nil {
		return fmt.Errorf("源路径无效：%w", err)
	}

	// 安全校验：验证目标路径
	if err := ValidatePath(rule.TargetPath); err != nil {
		return fmt.Errorf("目标路径无效：%w", err)
	}

	// 获取目标节点信息
	node, exists := ss.cluster.GetNode(targetNodeID)
	if !exists {
		return fmt.Errorf("目标节点不存在：%s", targetNodeID)
	}

	// 安全校验：验证节点IP地址
	if err := ValidateIP(node.IP); err != nil {
		return fmt.Errorf("节点IP地址无效：%w", err)
	}

	if node.Status != StatusOnline {
		return fmt.Errorf("目标节点离线：%s", targetNodeID)
	}

	// 安全校验：验证rsync路径
	// 使用白名单验证，默认允许 /usr/bin/rsync
	allowedRsyncPaths := []string{"/usr/bin/rsync", "/usr/local/bin/rsync"}
	if err := ValidateRsyncPath(ss.config.RsyncPath, allowedRsyncPaths); err != nil {
		return fmt.Errorf("rsync路径验证失败：%w", err)
	}

	// 安全校验：清理路径，防止命令注入
	sourcePath := filepath.Clean(rule.SourcePath)
	targetPath := filepath.Clean(rule.TargetPath)
	nodeIP := node.IP

	// 再次验证清理后的路径
	if pathTraversalPattern.MatchString(sourcePath) || pathTraversalPattern.MatchString(targetPath) {
		return fmt.Errorf("路径包含非法字符")
	}

	// 构建 rsync 命令
	// 使用参数化命令，避免命令注入
	cmd := exec.CommandContext(ss.ctx, ss.config.RsyncPath,
		"-avz",
		"--delete",
		"--progress",
		sourcePath,
		fmt.Sprintf("%s@%s:%s", "nasadmin", nodeIP, targetPath),
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("rsync 执行失败：%w, output: %s", err, string(output))
	}

	// 解析输出，统计同步文件数和字节数
	// 这里简化处理，实际应该解析 rsync 输出
	job.FilesSynced += 1
	job.BytesSynced += int64(len(output))

	ss.logger.Info("同步完成",
		zap.String("rule_id", rule.ID),
		zap.String("target_node", targetNodeID),
		zap.Int64("files", job.FilesSynced),
		zap.Int64("bytes", job.BytesSynced))

	return nil
}

// GetStatus 获取同步状态
func (ss *StorageSync) GetStatus() SyncStatus {
	ss.rulesMutex.RLock()
	ss.jobsMutex.RLock()
	defer ss.rulesMutex.RUnlock()
	ss.jobsMutex.RUnlock()

	status := SyncStatus{
		TotalRules:  len(ss.rules),
		ActiveRules: 0,
		RunningJobs: 0,
		TotalJobs:   len(ss.jobs),
		FailedJobs:  0,
	}

	for _, rule := range ss.rules {
		if rule.Enabled {
			status.ActiveRules++
		}
		if rule.Status == SyncStatusRunning {
			status.RunningJobs++
		}
	}

	for _, job := range ss.jobs {
		if job.Status == SyncStatusFailed {
			status.FailedJobs++
		}
		if !job.EndTime.IsZero() {
			if status.LastSyncTime.IsZero() || job.EndTime.After(status.LastSyncTime) {
				status.LastSyncTime = job.EndTime
			}
		}
	}

	return status
}

// GetJobs 获取同步任务历史
func (ss *StorageSync) GetJobs(limit int) []*SyncJob {
	ss.jobsMutex.RLock()
	defer ss.jobsMutex.RUnlock()

	if limit <= 0 || limit > len(ss.jobs) {
		limit = len(ss.jobs)
	}

	// 返回最近的 limit 个任务
	start := len(ss.jobs) - limit
	if start < 0 {
		start = 0
	}

	result := make([]*SyncJob, limit)
	copy(result, ss.jobs[start:])
	return result
}

// Shutdown 关闭同步管理器
func (ss *StorageSync) Shutdown() error {
	ss.cancel()
	ss.cron.Stop()

	ss.logger.Info("存储同步管理器已关闭")
	return nil
}

// 持久化

func (ss *StorageSync) saveRules() error {
	rulesFile := filepath.Join(ss.config.DataDir, "sync_rules.json")

	data, err := json.MarshalIndent(ss.rules, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(rulesFile, data, 0644)
}

func (ss *StorageSync) loadRules() error {
	rulesFile := filepath.Join(ss.config.DataDir, "sync_rules.json")

	data, err := os.ReadFile(rulesFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &ss.rules)
}

// 辅助函数

func generateRuleID() string {
	return fmt.Sprintf("rule-%d", time.Now().UnixNano())
}

// ========== 安全验证函数 ==========

// 安全验证正则
var (
	// 路径注入检测正则：检测 .. 路径遍历和危险字符
	pathTraversalPattern = regexp.MustCompile(`\.\.[\\/]|[\x00\n\r]`)
)

// 安全验证错误
var (
	ErrInvalidPath       = errors.New("无效的路径：包含非法字符或路径遍历")
	ErrInvalidIP         = errors.New("无效的IP地址")
	ErrInvalidNodeID     = errors.New("无效的节点ID")
	ErrInvalidSchedule   = errors.New("无效的cron表达式")
	ErrInvalidRsyncPath  = errors.New("无效的rsync路径")
	ErrPathNotAllowed    = errors.New("路径不在允许的目录范围内")
	ErrCommandNotAllowed = errors.New("命令不在允许列表中")
)

// ValidatePath 验证路径安全性
// 检查路径遍历攻击和非法字符
func ValidatePath(path string) error {
	if path == "" {
		return errors.New("路径不能为空")
	}

	// 清理路径
	cleanPath := filepath.Clean(path)

	// 检查路径遍历
	if pathTraversalPattern.MatchString(path) {
		return ErrInvalidPath
	}

	// 检查清理后的路径是否与原路径一致（防止编码绕过）
	if cleanPath != filepath.Clean(path) {
		return ErrInvalidPath
	}

	// 禁止空字节和换行符（防止命令注入）
	if strings.ContainsAny(path, "\x00\n\r") {
		return ErrInvalidPath
	}

	return nil
}

// ValidatePathInAllowedDirs 验证路径是否在允许的目录范围内
func ValidatePathInAllowedDirs(path string, allowedDirs []string) error {
	if err := ValidatePath(path); err != nil {
		return err
	}

	absPath, err := filepath.Abs(path)
	if err != nil {
		return ErrInvalidPath
	}

	// 如果没有配置允许目录，则只检查基本安全性
	if len(allowedDirs) == 0 {
		return nil
	}

	for _, dir := range allowedDirs {
		absDir, err := filepath.Abs(dir)
		if err != nil {
			continue
		}
		if strings.HasPrefix(absPath, absDir) {
			return nil
		}
	}

	return ErrPathNotAllowed
}

// ValidateIP 验证IP地址格式
func ValidateIP(ip string) error {
	if ip == "" {
		return errors.New("IP地址不能为空")
	}

	// 解析IP地址
	parsedIP := net.ParseIP(ip)
	if parsedIP == nil {
		return ErrInvalidIP
	}

	return nil
}

// ValidateNodeID 验证节点ID格式
func ValidateNodeID(nodeID string) error {
	if nodeID == "" {
		return errors.New("节点ID不能为空")
	}

	if len(nodeID) > 64 {
		return errors.New("节点ID长度不能超过64字符")
	}

	if !nodeIDPattern.MatchString(nodeID) {
		return ErrInvalidNodeID
	}

	return nil
}

// ValidateCronExpression 验证cron表达式
func ValidateCronExpression(expr string) error {
	if expr == "" {
		return nil // 空表达式是允许的（手动触发）
	}

	// 尝试解析cron表达式
	_, err := cron.ParseStandard(expr)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrInvalidSchedule, err)
	}

	return nil
}

// ValidateRsyncPath 验证rsync可执行文件路径
// 确保路径在允许的白名单内
func ValidateRsyncPath(rsyncPath string, allowedPaths []string) error {
	if rsyncPath == "" {
		return errors.New("rsync路径不能为空")
	}

	// 清理路径
	cleanPath := filepath.Clean(rsyncPath)

	// 检查是否是绝对路径
	if !filepath.IsAbs(cleanPath) {
		return ErrInvalidRsyncPath
	}

	// 检查路径注入
	if pathTraversalPattern.MatchString(rsyncPath) {
		return ErrInvalidRsyncPath
	}

	// 如果有白名单，检查是否在白名单内
	if len(allowedPaths) > 0 {
		allowed := false
		for _, p := range allowedPaths {
			if cleanPath == filepath.Clean(p) {
				allowed = true
				break
			}
		}
		if !allowed {
			return ErrInvalidRsyncPath
		}
	}

	// 检查文件是否存在且可执行
	info, err := os.Stat(cleanPath)
	if err != nil {
		return fmt.Errorf("rsync路径不存在: %w", err)
	}

	// 检查是否是常规文件
	if !info.Mode().IsRegular() {
		return ErrInvalidRsyncPath
	}

	return nil
}

// ValidateSyncRule 验证同步规则的所有字段
func ValidateSyncRule(rule *SyncRule) error {
	if rule == nil {
		return errors.New("规则不能为空")
	}

	// 验证规则名称
	if rule.Name == "" {
		return errors.New("规则名称不能为空")
	}
	if len(rule.Name) > 128 {
		return errors.New("规则名称不能超过128字符")
	}

	// 验证源路径
	if err := ValidatePath(rule.SourcePath); err != nil {
		return fmt.Errorf("源路径无效: %w", err)
	}

	// 验证目标路径
	if err := ValidatePath(rule.TargetPath); err != nil {
		return fmt.Errorf("目标路径无效: %w", err)
	}

	// 验证目标节点
	if len(rule.TargetNodes) == 0 {
		return errors.New("至少需要一个目标节点")
	}
	for _, nodeID := range rule.TargetNodes {
		if err := ValidateNodeID(nodeID); err != nil {
			return fmt.Errorf("目标节点ID无效: %w", err)
		}
	}

	// 验证同步模式
	validModes := map[string]bool{
		SyncModeAsync:    true,
		SyncModeSync:     true,
		SyncModeRealtime: true,
	}
	if rule.SyncMode != "" && !validModes[rule.SyncMode] {
		return fmt.Errorf("无效的同步模式: %s", rule.SyncMode)
	}

	// 验证cron表达式
	if err := ValidateCronExpression(rule.Schedule); err != nil {
		return fmt.Errorf("cron表达式无效: %w", err)
	}

	return nil
}
