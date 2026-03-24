// Package storage 提供高性能存储模块
package storage

import (
	"nas-os/pkg/storage/dedup"
	"nas-os/pkg/storage/rdma"
	"nas-os/pkg/storage/zfs"
)

// StorageModules 存储模块集合
type StorageModules struct {
	Dedup *dedup.FastDeduplicator
	RDMA  *rdma.RDMAManager
	ZFS   *zfs.ZFSManager
}

// Config 存储配置
type Config struct {
	Dedup *dedup.DedupConfig `json:"dedup"`
	RDMA  *rdma.RDMAConfig   `json:"rdma"`
	ZFS   *zfs.ImmutablePolicy `json:"zfs"`
}

// DefaultConfig 返回默认配置
func DefaultConfig() *Config {
	return &Config{
		Dedup: dedup.DefaultDedupConfig(),
		RDMA:  rdma.DefaultRDMAConfig(),
		ZFS:   zfs.DefaultImmutablePolicy(),
	}
}

// Initialize 初始化存储模块
func Initialize(cfg *Config) (*StorageModules, error) {
	if cfg == nil {
		cfg = DefaultConfig()
	}

	modules := &StorageModules{}

	// 初始化去重模块
	deduplicator, err := dedup.NewFastDeduplicator(cfg.Dedup)
	if err != nil {
		return nil, err
	}
	modules.Dedup = deduplicator

	// 初始化 RDMA 模块
	rdmaMgr, err := rdma.NewRDMAManager(cfg.RDMA)
	if err != nil {
		_ = modules.Dedup.Close()
		return nil, err
	}
	modules.RDMA = rdmaMgr

	// 初始化 ZFS 模块
	zfsMgr, err := zfs.NewZFSManager("", cfg.ZFS)
	if err != nil {
		_ = modules.Dedup.Close()
		_ = modules.RDMA.Close()
		return nil, err
	}
	modules.ZFS = zfsMgr

	return modules, nil
}

// Close 关闭所有模块
func (m *StorageModules) Close() error {
	if m.Dedup != nil {
		_ = m.Dedup.Close()
	}
	if m.RDMA != nil {
		_ = m.RDMA.Close()
	}
	if m.ZFS != nil {
		_ = m.ZFS.Close()
	}
	return nil
}

// GetDedupStats 获取去重统计
func (m *StorageModules) GetDedupStats() map[string]interface{} {
	if m.Dedup == nil {
		return nil
	}
	return m.Dedup.GetStats()
}

// GetRDMAStats 获取 RDMA 统计
func (m *StorageModules) GetRDMAStats() map[string]rdma.RDMAStats {
	if m.RDMA == nil {
		return nil
	}
	return m.RDMA.GetStats()
}

// ListImmutableSnapshots 列出不可变快照
func (m *StorageModules) ListImmutableSnapshots() []zfs.SnapshotInfo {
	if m.ZFS == nil {
		return nil
	}
	return m.ZFS.ListImmutableSnapshots()
}

// CheckRDMAAvailable 检查 RDMA 是否可用
func CheckRDMAAvailable() (bool, error) {
	return rdma.CheckRDMAAvailable()
}

// CheckZFSAvailable 检查 ZFS 是否可用
func CheckZFSAvailable() bool {
	mgr, err := zfs.NewZFSManager("", nil)
	if err != nil {
		return false
	}
	available := mgr.IsAvailable()
	_ = mgr.Close()
	return available
}