package snapshotmgr

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"

	"go.uber.org/zap"
)

// Manager 快照管理器.
type Manager struct {
	mu          sync.RWMutex
	logger      *zap.Logger
	config      *SnapshotConfig
	snapshotDir string
	snapshots   map[string]*Snapshot
}

// NewManager 创建快照管理器.
func NewManager(logger *zap.Logger, config *SnapshotConfig, snapshotDir string) *Manager {
	if logger == nil {
		logger = zap.NewNop()
	}
	if config == nil {
		config = &SnapshotConfig{
			MaxSnapshots:  20,
			RetentionDays: 90,
		}
	}
	if config.MaxSnapshots <= 0 {
		config.MaxSnapshots = 20
	}
	if config.RetentionDays <= 0 {
		config.RetentionDays = 90
	}

	m := &Manager{
		logger:      logger,
		config:      config,
		snapshotDir: snapshotDir,
		snapshots:   make(map[string]*Snapshot),
	}

	// 加载已有快照
	m.loadFromDisk()

	return m
}

// generateID 生成 16 字节 hex ID.
func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// CreateSnapshot 创建快照.
func (m *Manager) CreateSnapshot(name, description, source string, items []SnapshotItem) (*Snapshot, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	// 检查是否超过最大数量（不含 deleted）
	activeCount := 0
	for _, s := range m.snapshots {
		if s.Status != "deleted" {
			activeCount++
		}
	}
	if activeCount >= m.config.MaxSnapshots {
		return nil, fmt.Errorf("maximum number of snapshots (%d) reached", m.config.MaxSnapshots)
	}

	id, err := generateID()
	if err != nil {
		return nil, fmt.Errorf("generate id: %w", err)
	}

	snap := &Snapshot{
		ID:          id,
		Name:        name,
		Description: description,
		Source:      source,
		CreatedAt:   time.Now(),
		Status:      "active",
		Items:       items,
	}

	// 计算总大小
	var totalSize int64
	for i := range snap.Items {
		totalSize += snap.Items[i].Size
	}
	snap.SizeBytes = totalSize

	// 保存到磁盘
	if err := m.saveSnapshot(snap); err != nil {
		return nil, fmt.Errorf("save snapshot: %w", err)
	}

	m.snapshots[id] = snap

	m.logger.Info("snapshot created",
		zap.String("id", id),
		zap.String("name", name),
		zap.String("source", source),
		zap.Int("items", len(items)),
	)

	return snap, nil
}

// ListSnapshots 列出所有快照.
func (m *Manager) ListSnapshots() []Snapshot {
	m.mu.RLock()
	defer m.mu.RUnlock()

	result := make([]Snapshot, 0, len(m.snapshots))
	for _, s := range m.snapshots {
		if s.Status != "deleted" {
			result = append(result, *s)
		}
	}

	sort.Slice(result, func(i, j int) bool {
		return result[i].CreatedAt.After(result[j].CreatedAt)
	})

	return result
}

// GetSnapshot 获取快照详情.
func (m *Manager) GetSnapshot(id string) (*Snapshot, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	snap, ok := m.snapshots[id]
	if !ok || snap.Status == "deleted" {
		return nil, fmt.Errorf("snapshot %s not found", id)
	}

	result := *snap
	result.Items = make([]SnapshotItem, len(snap.Items))
	copy(result.Items, snap.Items)

	return &result, nil
}

// DeleteSnapshot 删除快照.
func (m *Manager) DeleteSnapshot(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[id]
	if !ok || snap.Status == "deleted" {
		return fmt.Errorf("snapshot %s not found", id)
	}

	snap.Status = "deleted"

	// 更新磁盘元数据
	if err := m.saveSnapshotMeta(snap); err != nil {
		return fmt.Errorf("save snapshot meta: %w", err)
	}

	// 删除快照文件
	snapDir := filepath.Join(m.snapshotDir, id)
	os.RemoveAll(snapDir)

	m.logger.Info("snapshot deleted", zap.String("id", id))
	return nil
}

// RestoreSnapshot 回滚快照，返回需要恢复的配置项.
func (m *Manager) RestoreSnapshot(id string) ([]SnapshotItem, error) {
	m.mu.Lock()
	defer m.mu.Unlock()

	snap, ok := m.snapshots[id]
	if !ok || snap.Status == "deleted" {
		return nil, fmt.Errorf("snapshot %s not found", id)
	}

	if snap.Status == "restoring" {
		return nil, fmt.Errorf("snapshot %s is already being restored", id)
	}

	snap.Status = "restoring"
	if err := m.saveSnapshotMeta(snap); err != nil {
		return nil, fmt.Errorf("save snapshot meta: %w", err)
	}

	// 构建需要恢复的项目列表（包含快照目录中的实际文件路径）
	items := make([]SnapshotItem, len(snap.Items))
	copy(items, snap.Items)

	m.logger.Info("snapshot restore initiated",
		zap.String("id", id),
		zap.Int("items", len(items)),
	)

	// 恢复完成后状态回到 active
	snap.Status = "active"
	if err := m.saveSnapshotMeta(snap); err != nil {
		m.logger.Error("failed to reset snapshot status", zap.String("id", id), zap.Error(err))
	}

	return items, nil
}

// CleanupOld 清理过期快照.
func (m *Manager) CleanupOld() {
	m.mu.Lock()
	defer m.mu.Unlock()

	cutoff := time.Now().AddDate(0, 0, -m.config.RetentionDays)

	for _, snap := range m.snapshots {
		if snap.Status == "deleted" {
			continue
		}
		if snap.CreatedAt.Before(cutoff) {
			snap.Status = "deleted"
			m.saveSnapshotMeta(snap)
			snapDir := filepath.Join(m.snapshotDir, snap.ID)
			os.RemoveAll(snapDir)
			m.logger.Info("snapshot expired", zap.String("id", snap.ID))
		}
	}
}

// GetStats 获取快照统计信息.
func (m *Manager) GetStats() map[string]interface{} {
	m.mu.RLock()
	defer m.mu.RUnlock()

	active := 0
	deleted := 0
	var totalSize int64

	for _, s := range m.snapshots {
		switch s.Status {
		case "deleted":
			deleted++
		default:
			active++
			totalSize += s.SizeBytes
		}
	}

	return map[string]interface{}{
		"total_snapshots":   active + deleted,
		"active_snapshots":  active,
		"deleted_snapshots": deleted,
		"total_size_bytes":  totalSize,
		"max_snapshots":     m.config.MaxSnapshots,
		"retention_days":    m.config.RetentionDays,
	}
}

// saveSnapshot 保存快照到磁盘（元数据 + 配置文件副本）.
func (m *Manager) saveSnapshot(snap *Snapshot) error {
	snapDir := filepath.Join(m.snapshotDir, snap.ID)
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return err
	}

	return m.saveSnapshotMeta(snap)
}

// saveSnapshotMeta 保存快照元数据.
func (m *Manager) saveSnapshotMeta(snap *Snapshot) error {
	snapDir := filepath.Join(m.snapshotDir, snap.ID)
	if err := os.MkdirAll(snapDir, 0755); err != nil {
		return err
	}

	metaPath := filepath.Join(snapDir, "meta.json")
	data, err := json.MarshalIndent(snap, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(metaPath, data, 0644)
}

// loadFromDisk 从磁盘加载快照.
func (m *Manager) loadFromDisk() {
	entries, err := os.ReadDir(m.snapshotDir)
	if err != nil {
		if os.IsNotExist(err) {
			return
		}
		m.logger.Error("failed to read snapshot dir", zap.Error(err))
		return
	}

	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}

		metaPath := filepath.Join(m.snapshotDir, entry.Name(), "meta.json")
		data, err := os.ReadFile(metaPath)
		if err != nil {
			m.logger.Warn("failed to read snapshot meta", zap.String("dir", entry.Name()), zap.Error(err))
			continue
		}

		var snap Snapshot
		if err := json.Unmarshal(data, &snap); err != nil {
			m.logger.Warn("failed to parse snapshot meta", zap.String("dir", entry.Name()), zap.Error(err))
			continue
		}

		m.snapshots[snap.ID] = &snap
	}
}

// calculateChecksum 计算文件 SHA256 校验和.
func calculateChecksum(data []byte) string {
	h := sha256.Sum256(data)
	return hex.EncodeToString(h[:])
}
