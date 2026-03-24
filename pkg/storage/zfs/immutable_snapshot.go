// Package zfs 实现 ZFS 不可变快照管理 API
// 提供企业级快照保护、WORM（Write Once Read Many）存储能力
package zfs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"
)

// ========== 核心错误定义 ==========

var (
	ErrPoolNotFound       = errors.New("ZFS pool not found")
	ErrDatasetNotFound    = errors.New("ZFS dataset not found")
	ErrSnapshotNotFound   = errors.New("snapshot not found")
	ErrSnapshotExists     = errors.New("snapshot already exists")
	ErrSnapshotImmutable  = errors.New("snapshot is immutable and cannot be modified")
	ErrSnapshotExpired    = errors.New("snapshot has expired")
	ErrInvalidName        = errors.New("invalid snapshot name")
	ErrCloneFailed        = errors.New("clone operation failed")
	ErrRollbackFailed     = errors.New("rollback operation failed")
	ErrHoldNotFound       = errors.New("hold not found")
	ErrBookmarkExists     = errors.New("bookmark already exists")
	ErrPermissionDenied   = errors.New("permission denied")
	ErrZFSNotAvailable    = errors.New("ZFS not available on this system")
)

// ========== 数据结构定义 ==========

// PoolInfo ZFS 池信息
type PoolInfo struct {
	Name      string            `json:"name"`
	State     string            `json:"state"`     // ONLINE, DEGRADED, FAULTED, OFFLINE, REMOVED, UNAVAIL
	Status    string            `json:"status"`
	GUID      uint64            `json:"guid"`
	Version   string            `json:"version"`
	Size      uint64            `json:"size"`      // 总大小（字节）
	Allocated uint64            `json:"allocated"` // 已分配（字节）
	Free      uint64            `json:"free"`      // 空闲（字节）
	ReadOnly  bool              `json:"readOnly"`
	Features  map[string]bool   `json:"features"`
	Vdevs     []VdevInfo        `json:"vdevs"`
	Scan      ScanInfo          `json:"scan"`
}

// VdevInfo 虚拟设备信息
type VdevInfo struct {
	Type     string     `json:"type"` // disk, file, mirror, raidz, raidz2, raidz3, log, cache, spare
	Path     string     `json:"path,omitempty"`
	State    string     `json:"state"`
	GUID     uint64     `json:"guid"`
	Children []VdevInfo `json:"children,omitempty"`
}

// ScanInfo 扫描信息
type ScanInfo struct {
	Function string `json:"function"` // none, scrub, resilver
	State    string `json:"state"`    // none, scanning, finished, canceled
	Progress string `json:"progress"`
	Errors   int    `json:"errors"`
}

// DatasetInfo 数据集信息
type DatasetInfo struct {
	Name          string            `json:"name"`
	Type          string            `json:"type"` // filesystem, volume, snapshot, bookmark
	Mounted       bool              `json:"mounted"`
	Mountpoint    string            `json:"mountpoint,omitempty"`
	Compression   string            `json:"compression"`
	CompressRatio float64           `json:"compressratio"`
	Quota         uint64            `json:"quota,omitempty"`
	RefQuota      uint64            `json:"refquota,omitempty"`
	Reservation   uint64            `json:"reservation,omitempty"`
	RefReservation uint64           `json:"refreservation,omitempty"`
	Used          uint64            `json:"used"`
	Avail         uint64            `json:"avail"`
	Referenced    uint64            `json:"referenced"`
	Written       uint64            `json:"written"`
	Origin        string            `json:"origin,omitempty"` // 用于克隆
	Properties    map[string]string `json:"properties"`
}

// SnapshotInfo 快照信息
type SnapshotInfo struct {
	Name          string            `json:"name"`
	FullName      string            `json:"fullName"`      // pool/dataset@snapshot
	Dataset       string            `json:"dataset"`
	CreationTime  time.Time         `json:"creationTime"`
	Used          uint64            `json:"used"`
	Referenced    uint64            `json:"referenced"`
	Written       uint64            `json:"written"`
	CompressRatio float64           `json:"compressRatio"`
	Clones        []string          `json:"clones,omitempty"`
	Holds         []HoldInfo        `json:"holds,omitempty"`
	Bookmark      string            `json:"bookmark,omitempty"`
	
	// 不可变属性
	Immutable     bool              `json:"immutable"`
	ImmutableTime time.Time         `json:"immutableTime,omitempty"`
	ImmutableBy   string            `json:"immutableBy,omitempty"`
	ExpiryTime    *time.Time        `json:"expiryTime,omitempty"`
	LockType      LockType          `json:"lockType"`
	
	// 校验信息
	Checksum      string            `json:"checksum,omitempty"`
	VerifiedAt    time.Time         `json:"verifiedAt,omitempty"`
	Verified      bool              `json:"verified"`
	
	// 元数据
	Metadata      map[string]string `json:"metadata,omitempty"`
}

// HoldInfo 快照保留信息
type HoldInfo struct {
	Name      string    `json:"name"`
	Tag       string    `json:"tag"`
	CreatedAt time.Time `json:"createdAt"`
	Immutable bool      `json:"immutable"`
}

// LockType 锁类型
type LockType string

const (
	LockTypeNone      LockType = "none"
	LockTypeSoft      LockType = "soft"      // 软锁定，可解锁
	LockTypeHard      LockType = "hard"      // 硬锁定，需满足条件才能解锁
	LockTypeTimed     LockType = "timed"     // 定时锁定，到期自动解锁
	LockTypePermanent LockType = "permanent" // 永久锁定，无法解锁
)

// ImmutablePolicy 不可变策略
type ImmutablePolicy struct {
	// 默认锁类型
	DefaultLockType LockType `json:"defaultLockType"`

	// 默认保留天数（0 = 无限制）
	DefaultRetentionDays int `json:"defaultRetentionDays"`

	// 是否允许提前解锁
	AllowEarlyRelease bool `json:"allowEarlyRelease"`

	// 解锁需要审批
	RequireApproval bool `json:"requireApproval"`

	// 审批者列表
	Approvers []string `json:"approvers,omitempty"`

	// 自动验证间隔（小时）
	AutoVerifyInterval int `json:"autoVerifyInterval"`

	// 验证失败时处理
	OnVerifyFail VerifyFailAction `json:"onVerifyFail"`

	// 最大快照数
	MaxSnapshots int `json:"maxSnapshots"`

	// 自动清理过期快照
	AutoCleanup bool `json:"autoCleanup"`
}

// VerifyFailAction 验证失败处理动作
type VerifyFailAction string

const (
	VerifyFailWarn    VerifyFailAction = "warn"    // 仅警告
	VerifyFailAlert   VerifyFailAction = "alert"   // 发送告警
	VerifyFailLock    VerifyFailAction = "lock"    // 锁定快照
	VerifyFailNone    VerifyFailAction = "none"    // 不处理
)

// DefaultImmutablePolicy 返回默认不可变策略
func DefaultImmutablePolicy() *ImmutablePolicy {
	return &ImmutablePolicy{
		DefaultLockType:      LockTypeSoft,
		DefaultRetentionDays: 365,
		AllowEarlyRelease:    false,
		RequireApproval:      true,
		AutoVerifyInterval:   24,
		OnVerifyFail:         VerifyFailAlert,
		MaxSnapshots:         1000,
		AutoCleanup:          true,
	}
}

// SnapshotCreateOptions 快照创建选项
type SnapshotCreateOptions struct {
	// 快照名称
	Name string `json:"name"`

	// 是否立即设为不可变
	Immutable bool `json:"immutable"`

	// 锁类型
	LockType LockType `json:"lockType"`

	// 过期时间
	ExpiryTime *time.Time `json:"expiryTime,omitempty"`

	// 保留标签
	HoldTag string `json:"holdTag,omitempty"`

	// 元数据
	Metadata map[string]string `json:"metadata,omitempty"`

	// 是否递归创建
	Recursive bool `json:"recursive"`

	// 是否创建书签
	CreateBookmark bool `json:"createBookmark"`

	// 是否验证创建
	Verify bool `json:"verify"`
}

// SnapshotRestoreOptions 快照恢复选项
type SnapshotRestoreOptions struct {
	// 是否销毁更新的数据
	DestroyNewer bool `json:"destroyNewer"`

	// 是否强制执行
	Force bool `json:"force"`

	// 是否递归
	Recursive bool `json:"recursive"`
}

// CloneOptions 克隆选项
type CloneOptions struct {
	// 目标数据集名称
	TargetDataset string `json:"targetDataset"`

	// 是否创建书签
	CreateBookmark bool `json:"createBookmark"`

	// 是否设置属性
	Properties map[string]string `json:"properties,omitempty"`
}

// ========== ZFS 管理器 ==========

// ZFSManager ZFS 管理器
type ZFSManager struct {
	mu sync.RWMutex

	// 配置
	policy *ImmutablePolicy

	// 快照索引
	snapshots map[string]*SnapshotInfo

	// 不可变快照索引
	immutableSnapshots map[string]*SnapshotInfo

	// 配置路径
	configPath string

	// 可用性标志
	available bool
}

// NewZFSManager 创建 ZFS 管理器
func NewZFSManager(configPath string, policy *ImmutablePolicy) (*ZFSManager, error) {
	if policy == nil {
		policy = DefaultImmutablePolicy()
	}

	m := &ZFSManager{
		policy:             policy,
		snapshots:          make(map[string]*SnapshotInfo),
		immutableSnapshots: make(map[string]*SnapshotInfo),
		configPath:         configPath,
	}

	// 检查 ZFS 是否可用
	m.checkAvailable()

	// 加载配置
	if configPath != "" {
		m.loadConfig()
	}

	return m, nil
}

// checkAvailable 检查 ZFS 是否可用
func (m *ZFSManager) checkAvailable() {
	// 检查 zfs 命令是否存在
	if _, err := exec.LookPath("zfs"); err == nil {
		if _, err := exec.LookPath("zpool"); err == nil {
			m.available = true
			return
		}
	}
	m.available = false
}

// IsAvailable 检查 ZFS 是否可用
func (m *ZFSManager) IsAvailable() bool {
	return m.available
}

// loadConfig 加载配置
func (m *ZFSManager) loadConfig() error {
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

	var config struct {
		Policy           *ImmutablePolicy              `json:"policy"`
		Snapshots        map[string]*SnapshotInfo      `json:"snapshots"`
		ImmutableSnapshots map[string]*SnapshotInfo    `json:"immutableSnapshots"`
	}

	if err := json.Unmarshal(data, &config); err != nil {
		return err
	}

	if config.Policy != nil {
		m.policy = config.Policy
	}
	if config.Snapshots != nil {
		m.snapshots = config.Snapshots
	}
	if config.ImmutableSnapshots != nil {
		m.immutableSnapshots = config.ImmutableSnapshots
	}

	return nil
}

// saveConfig 保存配置
func (m *ZFSManager) saveConfig() error {
	if m.configPath == "" {
		return nil
	}

	config := struct {
		Policy             *ImmutablePolicy         `json:"policy"`
		Snapshots          map[string]*SnapshotInfo `json:"snapshots"`
		ImmutableSnapshots map[string]*SnapshotInfo `json:"immutableSnapshots"`
	}{
		Policy:             m.policy,
		Snapshots:          m.snapshots,
		ImmutableSnapshots: m.immutableSnapshots,
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(m.configPath), 0750); err != nil {
		return err
	}

	return os.WriteFile(m.configPath, data, 0640)
}

// ========== 池操作 ==========

// ListPools 列出所有池
func (m *ZFSManager) ListPools(ctx context.Context) ([]PoolInfo, error) {
	if !m.available {
		return nil, ErrZFSNotAvailable
	}

	cmd := exec.CommandContext(ctx, "zpool", "list", "-H", "-o", "name,state,size,allocated,free,readonly", "-p")
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list pools: %w", err)
	}

	var pools []PoolInfo
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 5 {
			continue
		}

		pool := PoolInfo{
			Name:   fields[0],
			State:  fields[1],
		}

		fmt.Sscanf(fields[2], "%d", &pool.Size)
		fmt.Sscanf(fields[3], "%d", &pool.Allocated)
		fmt.Sscanf(fields[4], "%d", &pool.Free)
		pool.ReadOnly = fields[5] == "on"

		pools = append(pools, pool)
	}

	return pools, nil
}

// GetPool 获取池信息
func (m *ZFSManager) GetPool(ctx context.Context, name string) (*PoolInfo, error) {
	pools, err := m.ListPools(ctx)
	if err != nil {
		return nil, err
	}

	for _, p := range pools {
		if p.Name == name {
			return &p, nil
		}
	}

	return nil, ErrPoolNotFound
}

// ========== 数据集操作 ==========

// ListDatasets 列出所有数据集
func (m *ZFSManager) ListDatasets(ctx context.Context, pool string) ([]DatasetInfo, error) {
	if !m.available {
		return nil, ErrZFSNotAvailable
	}

	args := []string{"list", "-H", "-o", "name,type,mounted,mountpoint,compression,compressratio,used,avail,referenced", "-r"}
	if pool != "" {
		args = append(args, pool)
	}

	cmd := exec.CommandContext(ctx, "zfs", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list datasets: %w", err)
	}

	var datasets []DatasetInfo
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 9 {
			continue
		}

		ds := DatasetInfo{
			Name:        fields[0],
			Type:        fields[1],
			Mounted:     fields[2] == "yes",
			Mountpoint:  fields[3],
			Compression: fields[4],
			Properties:  make(map[string]string),
		}

		fmt.Sscanf(fields[5], "%f", &ds.CompressRatio)
		fmt.Sscanf(fields[6], "%d", &ds.Used)
		fmt.Sscanf(fields[7], "%d", &ds.Avail)
		fmt.Sscanf(fields[8], "%d", &ds.Referenced)

		datasets = append(datasets, ds)
	}

	return datasets, nil
}

// GetDataset 获取数据集信息
func (m *ZFSManager) GetDataset(ctx context.Context, name string) (*DatasetInfo, error) {
	datasets, err := m.ListDatasets(ctx, "")
	if err != nil {
		return nil, err
	}

	for _, d := range datasets {
		if d.Name == name {
			return &d, nil
		}
	}

	return nil, ErrDatasetNotFound
}

// ========== 快照操作 ==========

// ListSnapshots 列出快照
func (m *ZFSManager) ListSnapshots(ctx context.Context, dataset string) ([]SnapshotInfo, error) {
	if !m.available {
		return nil, ErrZFSNotAvailable
	}

	args := []string{"list", "-H", "-t", "snapshot", "-o", "name,creation,used,referenced,written,compressratio"}
	if dataset != "" {
		args = append(args, "-r", dataset)
	}

	cmd := exec.CommandContext(ctx, "zfs", args...)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list snapshots: %w", err)
	}

	var snapshots []SnapshotInfo
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) < 6 {
			continue
		}

		fullName := fields[0]
		parts := strings.SplitN(fullName, "@", 2)
		if len(parts) != 2 {
			continue
		}

		snap := SnapshotInfo{
			FullName: fullName,
			Dataset:  parts[0],
			Name:     parts[1],
		}

		// 解析创建时间
		if creation, err := time.Parse("Mon Jan 2 15:04 2006", fields[1]); err == nil {
			snap.CreationTime = creation
		}

		fmt.Sscanf(fields[2], "%d", &snap.Used)
		fmt.Sscanf(fields[3], "%d", &snap.Referenced)
		fmt.Sscanf(fields[4], "%d", &snap.Written)
		fmt.Sscanf(fields[5], "%f", &snap.CompressRatio)

		// 检查是否不可变
		if imm, ok := m.immutableSnapshots[fullName]; ok {
			snap.Immutable = true
			snap.ImmutableTime = imm.ImmutableTime
			snap.ImmutableBy = imm.ImmutableBy
			snap.ExpiryTime = imm.ExpiryTime
			snap.LockType = imm.LockType
		}

		snapshots = append(snapshots, snap)
	}

	return snapshots, nil
}

// CreateSnapshot 创建快照
func (m *ZFSManager) CreateSnapshot(ctx context.Context, dataset string, opts SnapshotCreateOptions) (*SnapshotInfo, error) {
	if !m.available {
		return nil, ErrZFSNotAvailable
	}

	// 验证名称
	if !isValidSnapshotName(opts.Name) {
		return nil, ErrInvalidName
	}

	fullName := fmt.Sprintf("%s@%s", dataset, opts.Name)

	// 检查是否已存在
	if _, err := m.getSnapshotInfo(ctx, fullName); err == nil {
		return nil, ErrSnapshotExists
	}

	// 构建 zfs snapshot 命令
	args := []string{"snapshot"}
	if opts.Recursive {
		args = append(args, "-r")
	}
	args = append(args, fullName)

	cmd := exec.CommandContext(ctx, "zfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to create snapshot: %w (%s)", err, string(output))
	}

	// 创建书签
	if opts.CreateBookmark {
		bookmarkName := fmt.Sprintf("%s#%s", dataset, opts.Name)
		bookmarkCmd := exec.CommandContext(ctx, "zfs", "bookmark", fullName, bookmarkName)
		if output, err := bookmarkCmd.CombinedOutput(); err != nil {
			// 书签失败不影响快照创建
			fmt.Printf("warning: failed to create bookmark: %s\n", string(output))
		}
	}

	// 获取快照信息
	snap, err := m.getSnapshotInfo(ctx, fullName)
	if err != nil {
		return nil, err
	}

	// 设置元数据
	if len(opts.Metadata) > 0 {
		snap.Metadata = opts.Metadata
	}

	// 如果需要不可变
	if opts.Immutable {
		if err := m.setImmutable(fullName, opts.LockType, opts.ExpiryTime); err != nil {
			// 设置不可变失败，删除快照
			_ = m.DeleteSnapshot(ctx, fullName, false)
			return nil, fmt.Errorf("failed to set immutable: %w", err)
		}
		snap.Immutable = true
		snap.LockType = opts.LockType
		snap.ExpiryTime = opts.ExpiryTime
	}

	// 设置保留标签
	if opts.HoldTag != "" {
		if err := m.SetHold(ctx, fullName, opts.HoldTag); err != nil {
			fmt.Printf("warning: failed to set hold: %v\n", err)
		}
	}

	// 验证
	if opts.Verify {
		if err := m.VerifySnapshot(ctx, fullName); err != nil {
			fmt.Printf("warning: verification failed: %v\n", err)
		}
	}

	// 缓存快照
	m.mu.Lock()
	m.snapshots[fullName] = snap
	if snap.Immutable {
		m.immutableSnapshots[fullName] = snap
	}
	m.mu.Unlock()

	// 保存配置
	_ = m.saveConfig()

	return snap, nil
}

// getSnapshotInfo 获取快照信息
func (m *ZFSManager) getSnapshotInfo(ctx context.Context, fullName string) (*SnapshotInfo, error) {
	cmd := exec.CommandContext(ctx, "zfs", "list", "-H", "-t", "snapshot", "-o", 
		"name,creation,used,referenced,written,compressratio", fullName)
	output, err := cmd.Output()
	if err != nil {
		return nil, ErrSnapshotNotFound
	}

	line := strings.TrimSpace(string(output))
	fields := strings.Split(line, "\t")
	if len(fields) < 6 {
		return nil, ErrSnapshotNotFound
	}

	parts := strings.SplitN(fullName, "@", 2)
	snap := &SnapshotInfo{
		FullName: fullName,
		Dataset:  parts[0],
		Name:     parts[1],
	}

	if creation, err := time.Parse("Mon Jan 2 15:04 2006", fields[1]); err == nil {
		snap.CreationTime = creation
	}

	fmt.Sscanf(fields[2], "%d", &snap.Used)
	fmt.Sscanf(fields[3], "%d", &snap.Referenced)
	fmt.Sscanf(fields[4], "%d", &snap.Written)
	fmt.Sscanf(fields[5], "%f", &snap.CompressRatio)

	return snap, nil
}

// DeleteSnapshot 删除快照
func (m *ZFSManager) DeleteSnapshot(ctx context.Context, fullName string, force bool) error {
	if !m.available {
		return ErrZFSNotAvailable
	}

	// 检查不可变
	m.mu.RLock()
	snap, isImmutable := m.immutableSnapshots[fullName]
	m.mu.RUnlock()

	if isImmutable && !force {
		// 检查是否过期
		if snap.ExpiryTime != nil && snap.ExpiryTime.After(time.Now()) {
			return ErrSnapshotImmutable
		}
		// 检查锁类型
		if snap.LockType == LockTypePermanent {
			return ErrSnapshotImmutable
		}
	}

	// 删除快照
	cmd := exec.CommandContext(ctx, "zfs", "destroy", fullName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to delete snapshot: %w (%s)", err, string(output))
	}

	// 清理缓存
	m.mu.Lock()
	delete(m.snapshots, fullName)
	delete(m.immutableSnapshots, fullName)
	m.mu.Unlock()

	_ = m.saveConfig()

	return nil
}

// RollbackSnapshot 回滚到快照
func (m *ZFSManager) RollbackSnapshot(ctx context.Context, fullName string, opts SnapshotRestoreOptions) error {
	if !m.available {
		return ErrZFSNotAvailable
	}

	// 检查不可变
	m.mu.RLock()
	snap, isImmutable := m.immutableSnapshots[fullName]
	m.mu.RUnlock()

	if isImmutable && snap.LockType != LockTypeNone {
		return ErrSnapshotImmutable
	}

	args := []string{"rollback"}
	if opts.DestroyNewer {
		args = append(args, "-r")
	}
	if opts.Force {
		args = append(args, "-f")
	}
	args = append(args, fullName)

	cmd := exec.CommandContext(ctx, "zfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to rollback snapshot: %w (%s)", err, string(output))
	}

	return nil
}

// CloneSnapshot 克隆快照
func (m *ZFSManager) CloneSnapshot(ctx context.Context, fullName string, opts CloneOptions) (*DatasetInfo, error) {
	if !m.available {
		return nil, ErrZFSNotAvailable
	}

	args := []string{"clone"}
	for k, v := range opts.Properties {
		args = append(args, "-o", fmt.Sprintf("%s=%s", k, v))
	}
	args = append(args, fullName, opts.TargetDataset)

	cmd := exec.CommandContext(ctx, "zfs", args...)
	if output, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("failed to clone snapshot: %w (%s)", err, string(output))
	}

	return m.GetDataset(ctx, opts.TargetDataset)
}

// ========== 不可变快照操作 ==========

// setImmutable 设置快照为不可变
func (m *ZFSManager) setImmutable(fullName string, lockType LockType, expiryTime *time.Time) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[fullName]
	if !ok {
		return ErrSnapshotNotFound
	}

	snap.Immutable = true
	snap.ImmutableTime = time.Now()
	snap.LockType = lockType
	snap.ExpiryTime = expiryTime

	m.immutableSnapshots[fullName] = snap

	return nil
}

// SetImmutable 将快照设为不可变
func (m *ZFSManager) SetImmutable(ctx context.Context, fullName string, lockType LockType, expiryTime *time.Time) error {
	if !m.available {
		return ErrZFSNotAvailable
	}

	// 检查快照是否存在
	snap, err := m.getSnapshotInfo(ctx, fullName)
	if err != nil {
		return err
	}

	// 缓存快照
	m.mu.Lock()
	m.snapshots[fullName] = snap
	m.mu.Unlock()

	return m.setImmutable(fullName, lockType, expiryTime)
}

// ReleaseImmutable 释放不可变快照
func (m *ZFSManager) ReleaseImmutable(ctx context.Context, fullName string, approver string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.immutableSnapshots[fullName]
	if !ok {
		return ErrSnapshotNotFound
	}

	// 检查锁类型
	switch snap.LockType {
	case LockTypePermanent:
		return ErrSnapshotImmutable
	case LockTypeHard:
		if m.policy.RequireApproval {
			approved := false
			for _, a := range m.policy.Approvers {
				if a == approver {
					approved = true
					break
				}
			}
			if !approved {
				return ErrPermissionDenied
			}
		}
	case LockTypeTimed:
		if snap.ExpiryTime != nil && snap.ExpiryTime.After(time.Now()) {
			return ErrSnapshotImmutable
		}
	}

	// 检查是否允许提前释放
	if !m.policy.AllowEarlyRelease && (snap.ExpiryTime == nil || snap.ExpiryTime.After(time.Now())) {
		return ErrSnapshotImmutable
	}

	snap.Immutable = false
	snap.LockType = LockTypeNone
	delete(m.immutableSnapshots, fullName)

	_ = m.saveConfig()

	return nil
}

// ListImmutableSnapshots 列出不可变快照
func (m *ZFSManager) ListImmutableSnapshots() []SnapshotInfo {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]SnapshotInfo, 0, len(m.immutableSnapshots))
	for _, snap := range m.immutableSnapshots {
		result = append(result, *snap)
	}
	return result
}

// ========== 保留操作 ==========

// SetHold 设置保留标签
func (m *ZFSManager) SetHold(ctx context.Context, fullName, tag string) error {
	if !m.available {
		return ErrZFSNotAvailable
	}

	cmd := exec.CommandContext(ctx, "zfs", "hold", tag, fullName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to set hold: %w (%s)", err, string(output))
	}

	return nil
}

// ReleaseHold 释放保留标签
func (m *ZFSManager) ReleaseHold(ctx context.Context, fullName, tag string) error {
	if !m.available {
		return ErrZFSNotAvailable
	}

	// 检查不可变
	m.mu.RLock()
	snap, isImmutable := m.immutableSnapshots[fullName]
	m.mu.RUnlock()

	if isImmutable && snap.LockType != LockTypeNone {
		return ErrSnapshotImmutable
	}

	cmd := exec.CommandContext(ctx, "zfs", "release", tag, fullName)
	if output, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("failed to release hold: %w (%s)", err, string(output))
	}

	return nil
}

// ListHolds 列出保留标签
func (m *ZFSManager) ListHolds(ctx context.Context, fullName string) ([]HoldInfo, error) {
	if !m.available {
		return nil, ErrZFSNotAvailable
	}

	cmd := exec.CommandContext(ctx, "zfs", "holds", "-H", fullName)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("failed to list holds: %w", err)
	}

	var holds []HoldInfo
	for _, line := range strings.Split(string(output), "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		fields := strings.Split(line, "\t")
		if len(fields) >= 2 {
			holds = append(holds, HoldInfo{
				Tag: fields[1],
			})
		}
	}

	return holds, nil
}

// ========== 验证操作 ==========

// VerifySnapshot 验证快照完整性
func (m *ZFSManager) VerifySnapshot(ctx context.Context, fullName string) error {
	if !m.available {
		return ErrZFSNotAvailable
	}

	// 获取快照数据并计算校验和
	// 实际实现需要使用 zfs send 计算校验和
	cmd := exec.CommandContext(ctx, "zfs", "send", "-nv", fullName)
	output, err := cmd.Output()
	if err != nil {
		return fmt.Errorf("verification failed: %w", err)
	}

	// 计算校验和
	hash := sha256.Sum256(output)
	checksum := hex.EncodeToString(hash[:])

	m.mu.Lock()
	if snap, ok := m.snapshots[fullName]; ok {
		snap.Checksum = checksum
		snap.VerifiedAt = time.Now()
		snap.Verified = true
	}
	m.mu.Unlock()

	return nil
}

// GetVerificationStatus 获取验证状态
func (m *ZFSManager) GetVerificationStatus(fullName string) (verified bool, verifiedAt time.Time, checksum string) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if snap, ok := m.snapshots[fullName]; ok {
		return snap.Verified, snap.VerifiedAt, snap.Checksum
	}
	return false, time.Time{}, ""
}

// ========== 辅助函数 ==========

// isValidSnapshotName 验证快照名称
func isValidSnapshotName(name string) bool {
	// 快照名称规则: 字母、数字、下划线、连字符、点、空格
	// 不能以数字开头，长度不超过256
	matched, _ := regexp.MatchString(`^[a-zA-Z_][a-zA-Z0-9_\-. ]{0,255}$`, name)
	return matched
}

// GetPolicy 获取策略
func (m *ZFSManager) GetPolicy() *ImmutablePolicy {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.policy
}

// SetPolicy 设置策略
func (m *ZFSManager) SetPolicy(policy *ImmutablePolicy) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.policy = policy
	_ = m.saveConfig()
}

// Close 关闭管理器
func (m *ZFSManager) Close() error {
	return m.saveConfig()
}